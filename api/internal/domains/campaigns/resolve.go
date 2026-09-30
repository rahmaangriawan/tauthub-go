// Package campaigns: helper bersama untuk domain kampanye (Taut Pintar dan
// wizard kampanye biasa).
package campaigns

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Resolver memetakan public_id kampanye (6 karakter) ke primary key UUID-nya.
//
// URL kampanye boleh memakai public_id, tapi seluruh handler, kolom relasi
// (publications.campaign_id, taut_*, audit_logs.target_id) dan payload job
// bekerja dengan UUID. Karena itu nilai path di-kanonikalisasi SEBELUM handler
// jalan, bukan di tiap query: beberapa handler me-resolve kampanye lewat helper
// tapi masih memakai nilai mentah dari path untuk UPDATE/DELETE, dan nilai yang
// tidak dikenal akan mencocokkan 0 baris tanpa error.
type Resolver struct {
	pool *pgxpool.Pool

	mu    sync.Mutex
	cache map[string]resolvedEntry
}

type resolvedEntry struct {
	id        string
	expiresAt time.Time
}

// TTL pendek: mapping public_id -> id tidak pernah berubah untuk sebuah baris,
// jadi cache hanya membatasi query saat halaman wizard memanggil banyak endpoint
// berurutan dengan id yang sama.
const resolveTTL = 30 * time.Second

func NewResolver(pool *pgxpool.Pool) *Resolver {
	return &Resolver{pool: pool, cache: map[string]resolvedEntry{}}
}

// IsUUID36: bentuk id kampanye kanonik (character(36)). Nilai seperti ini
// diteruskan apa adanya tanpa query tambahan.
func IsUUID36(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i := 0; i < len(v); i++ {
		switch i {
		case 8, 13, 18, 23:
			if v[i] != '-' {
				return false
			}
		default:
			c := v[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// Lookup mengubah public_id menjadi UUID kampanye.
func (r *Resolver) Lookup(ctx context.Context, publicID string) (string, bool) {
	if r == nil || r.pool == nil || publicID == "" {
		return "", false
	}
	now := time.Now()
	r.mu.Lock()
	if e, ok := r.cache[publicID]; ok && e.expiresAt.After(now) {
		r.mu.Unlock()
		return e.id, true
	}
	r.mu.Unlock()

	var id string
	if err := r.pool.QueryRow(ctx,
		"SELECT id FROM campaigns WHERE public_id = $1 LIMIT 1", publicID).Scan(&id); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("[campaigns] resolusi public_id gagal", "public_id", publicID, "err", err)
		}
		return "", false
	}

	r.mu.Lock()
	r.cache[publicID] = resolvedEntry{id: id, expiresAt: now.Add(resolveTTL)}
	r.mu.Unlock()
	return id, true
}

// Wrap membungkus handler rute kampanye. Kalau {campaignId} bukan UUID, nilainya
// diganti UUID kanonik hasil Lookup supaya seluruh handler (termasuk jalur tulis
// dan audit) bekerja dengan id yang benar.
//
// Nilai yang tidak ditemukan dibiarkan apa adanya: handler akan memberi 404 yang
// sudah ada, bukan error baru.
func (r *Resolver) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		raw := chi.URLParam(req, "campaignId")
		if raw != "" && !IsUUID36(raw) {
			if id, ok := r.Lookup(req.Context(), raw); ok {
				rctx := chi.RouteContext(req.Context())
				for i := len(rctx.URLParams.Keys) - 1; i >= 0; i-- {
					if rctx.URLParams.Keys[i] == "campaignId" {
						rctx.URLParams.Values[i] = id
					}
				}
				req.SetPathValue("campaignId", id)
			}
		}
		next(w, req)
	}
}
