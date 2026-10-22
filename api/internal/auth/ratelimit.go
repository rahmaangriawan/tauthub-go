package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Port src/lib/auth/rate-limit.ts — DB-backed sliding window pada
// auth_rate_limits (atomic UPSERT) + fallback in-memory saat DB gagal.

type AuthRateLimitError struct{}

func (e *AuthRateLimitError) Error() string {
	return "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi."
}

type memoryBucket struct {
	count       int
	windowStart int64 // ms
}

const memoryMaxBuckets = 5_000

var (
	memoryMu      sync.Mutex
	memoryBuckets = map[string]*memoryBucket{}
)

func checkMemoryRate(keyedBucket string, limit, windowSeconds int) error {
	now := time.Now().UnixMilli()
	windowMs := int64(windowSeconds) * 1000

	memoryMu.Lock()
	defer memoryMu.Unlock()
	entry, ok := memoryBuckets[keyedBucket]
	if !ok || now-entry.windowStart >= windowMs {
		if len(memoryBuckets) >= memoryMaxBuckets {
			for k, v := range memoryBuckets {
				if now-v.windowStart >= windowMs {
					delete(memoryBuckets, k)
				}
			}
			if len(memoryBuckets) >= memoryMaxBuckets {
				memoryBuckets = map[string]*memoryBucket{}
			}
		}
		memoryBuckets[keyedBucket] = &memoryBucket{count: 1, windowStart: now}
		return nil
	}
	entry.count++
	if entry.count > limit {
		return &AuthRateLimitError{}
	}
	return nil
}

// CheckRate: port checkRate() — UPSERT atomik, count > limit → AuthRateLimitError.
// DB error → jangan blokir trafik, jatuh ke in-memory (log saja).
func CheckRate(ctx context.Context, pool *pgxpool.Pool, bucket string, maxPerMinute, windowSeconds int) error {
	limit := maxPerMinute
	if limit <= 0 {
		limit = 10
	}
	window := windowSeconds
	if window <= 0 {
		window = 60
	}
	keyedBucket := fmt.Sprintf("%s:%d", bucket, window)

	id, err := uuid.NewRandom()
	if err != nil {
		return checkMemoryRate(keyedBucket, limit, window)
	}
	var count int
	err = pool.QueryRow(ctx, `
		INSERT INTO auth_rate_limits (id, bucket, count, window_start, updated_at)
		VALUES ($1, $2, 1, NOW(), NOW())
		ON CONFLICT (bucket) DO UPDATE SET
		  count = CASE
		    WHEN auth_rate_limits.window_start < NOW() - make_interval(secs => $3::int) THEN 1
		    ELSE auth_rate_limits.count + 1
		  END,
		  window_start = CASE
		    WHEN auth_rate_limits.window_start < NOW() - make_interval(secs => $3::int) THEN NOW()
		    ELSE auth_rate_limits.window_start
		  END,
		  updated_at = NOW()
		RETURNING count`,
		id.String(), keyedBucket, window,
	).Scan(&count)
	if err != nil {
		slog.Warn("[rate-limit] DB gagal, memakai fallback in-memory", "err", err)
		return checkMemoryRate(keyedBucket, limit, window)
	}
	if count > limit {
		return &AuthRateLimitError{}
	}
	return nil
}

// Kunci bucket: endpoint yang sudah punya identitas memakai user ID
// (`refresh:user:<id>`, `ssr:user:<id>`, `change-pass:user:<id>`,
// `verify-pass:user:<id>`, `2fa:user:<id>`, `google:link:user:<id>`, dan
// `write:*:<userID>`), supaya beberapa pengguna di satu IP tidak saling
// menghabiskan kuota. Bucket per IP hanya dipakai endpoint pra-login yang belum
// punya identitas (login, register, forgot/reset password, referral, callback
// publik) — di sana IP adalah satu-satunya penahan brute-force.
//
// CheckAuthRate: login/register/2fa (default env AUTH_RATE_LIMIT_PER_MINUTE).
func CheckAuthRate(ctx context.Context, pool *pgxpool.Pool, bucket string, defaultPerMinute int) error {
	return CheckRate(ctx, pool, bucket, defaultPerMinute, 60)
}

// CheckWriteRate: AI/uang/publikasi (default env WRITE_RATE_LIMIT_PER_MINUTE).
func CheckWriteRate(ctx context.Context, pool *pgxpool.Pool, bucket string, defaultPerMinute int) error {
	return CheckRate(ctx, pool, bucket, defaultPerMinute, 60)
}

// RequestIp: x-forwarded-for (first) → x-real-ip.
func RequestIp(header http.Header) string {
	if xff := header.Get("x-forwarded-for"); xff != "" {
		for _, part := range strings.Split(xff, ",") {
			if p := strings.TrimSpace(part); p != "" {
				return p
			}
		}
	}
	if xri := header.Get("x-real-ip"); xri != "" {
		return xri
	}
	return ""
}
