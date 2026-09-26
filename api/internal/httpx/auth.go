package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
)

// Port getAuthUser() di src/auth/http.ts: Bearer → verify access token →
// cek user_sessions via sid (revocability) → query user (cache 30s).

type AuthUser struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type cachedUser struct {
	user      AuthUser
	expiresAt time.Time
}

type Authenticator struct {
	Pool   *pgxpool.Pool
	Secret string
	Issuer string

	mu    sync.Mutex
	cache map[string]cachedUser
}

const authCacheTTL = 30 * time.Second

func (a *Authenticator) InvalidateAuthCache(userID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cache == nil {
		return
	}
	if userID == "" {
		a.cache = map[string]cachedUser{}
	} else {
		delete(a.cache, userID)
	}
}

// User memvalidasi request dan mengembalikan user; null jika tidak valid.
func (a *Authenticator) User(r *http.Request) *AuthUser {
	token := bearerToken(r)
	if token == "" {
		return nil
	}
	claims, err := auth.VerifyAccessToken(a.Secret, a.Issuer, token)
	if err != nil {
		return nil
	}

	// Revocability: sid = refresh_jti sesi; revoked/expired → tolak sekarang.
	// Token lama tanpa sid masih diterima (grace period, parity).
	if claims.SID != "" {
		var revoked bool
		var expiresAt time.Time
		err := a.Pool.QueryRow(r.Context(),
			"SELECT revoked, expires_at FROM user_sessions WHERE refresh_jti = $1 LIMIT 1",
			claims.SID,
		).Scan(&revoked, &expiresAt)
		if err != nil || revoked || !expiresAt.After(time.Now()) {
			return nil
		}
	}

	now := time.Now()
	a.mu.Lock()
	if a.cache != nil {
		if cached, ok := a.cache[claims.Sub]; ok && cached.expiresAt.After(now) {
			a.mu.Unlock()
			user := cached.user
			return &user
		}
	}
	a.mu.Unlock()

	var user AuthUser
	var displayName *string
	err = a.Pool.QueryRow(r.Context(),
		"SELECT id, email, display_name, role, created_at FROM users WHERE id = $1 AND is_active = TRUE AND approved_at IS NOT NULL LIMIT 1",
		claims.Sub,
	).Scan(&user.ID, &user.Email, &displayName, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil
	}
	if displayName != nil {
		user.DisplayName = *displayName
	}

	a.mu.Lock()
	if a.cache == nil {
		a.cache = map[string]cachedUser{}
	}
	a.cache[claims.Sub] = cachedUser{user: user, expiresAt: now.Add(authCacheTTL)}
	a.mu.Unlock()
	return &user
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("authorization")
	if header == "" {
		return ""
	}
	const scheme = "bearer "
	if len(header) > len(scheme) && header[:len(scheme)] == scheme {
		return header[len(scheme):]
	}
	const schemeCap = "Bearer "
	if len(header) > len(schemeCap) && header[:len(schemeCap)] == schemeCap {
		return header[len(schemeCap):]
	}
	return ""
}

type ctxKey struct{}

// RequireAuth middleware: 401 dengan pesan parity ("Invalid or expired access token").
func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := a.User(r)
		if user == nil {
			slog.Debug("auth gagal", "path", r.URL.Path)
			Error(w, http.StatusUnauthorized, "Invalid or expired access token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	})
}

// AuthUserFromCtx mengembalikan user yang dipasang RequireAuth.
func AuthUserFromCtx(r *http.Request) *AuthUser {
	user, _ := r.Context().Value(ctxKey{}).(*AuthUser)
	return user
}
