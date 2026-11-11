package httpx

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strings"
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

// Agent menandai request yang masuk lewat jalur service-key MCP. Dipasang di
// context oleh RequireAuth setelah verifikasi, lalu dibaca d.audit untuk
// mencatat jejak agent di audit_logs.detail.
type Agent struct {
	ClientID   string // agent_clients.id
	ActionID   string // agent_actions.id (tulis saja; baca kosong)
	ActionCode string // kode pendek (muncul di Telegram / UI)
}

type ctxAgentKey struct{}

// AgentFromCtx mengembalikan marker agent bila request datang lewat MCP.
func AgentFromCtx(ctx context.Context) *Agent {
	a, _ := ctx.Value(ctxAgentKey{}).(*Agent)
	return a
}

type Authenticator struct {
	Pool   *pgxpool.Pool
	Secret string
	Issuer string

	// ServiceKeyProvider: bila di-set dan mengembalikan string non-kosong,
	// RequireAuth menerima `Bearer <service key>` plus header
	// `X-Act-As-User: <user_id>` sebagai pengganti JWT. Fungsi ini dipanggil
	// per-request supaya rotasi key (lewat admin UI) langsung berlaku tanpa
	// restart. Dipakai cmd/mcp yang dialamatkan ke loopback. Perbandingan
	// constant-time.
	ServiceKeyProvider func() string

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
		if user, agent := a.tryServiceKey(r); user != nil {
			ctx := context.WithValue(r.Context(), ctxKey{}, user)
			if agent != nil {
				ctx = context.WithValue(ctx, ctxAgentKey{}, agent)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
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

// tryServiceKey: jalur loopback untuk MCP admin. Semua syarat berikut WAJIB:
//   - a.ServiceKey di-set (fitur dinyalakan).
//   - Bearer token == ServiceKey (constant-time).
//   - Request tidak membawa X-Forwarded-For / X-Real-IP (nginx selalu memasang
//     salah satunya, agent lokal tidak); r.RemoteAddr berasal dari loopback.
//   - Header X-Act-As-User berisi user_id admin yang aktif dan ter-approve.
//
// Bila salah satu tidak terpenuhi → return nil,nil dan RequireAuth jatuh ke
// jalur JWT biasa supaya tidak membocorkan kehadiran mekanisme ini.
func (a *Authenticator) tryServiceKey(r *http.Request) (*AuthUser, *Agent) {
	if a.ServiceKeyProvider == nil {
		return nil, nil
	}
	key := a.ServiceKeyProvider()
	if key == "" {
		return nil, nil
	}
	token := bearerToken(r)
	if token == "" {
		return nil, nil
	}
	// Panjang berbeda → bukan match. subtle.ConstantTimeCompare mengembalikan
	// 0 untuk panjang berbeda, tapi di sini kita tegaskan supaya tidak
	// mengandalkan perilaku itu.
	if len(token) != len(key) {
		return nil, nil
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(key)) != 1 {
		return nil, nil
	}
	// RealIP middleware menimpa r.RemoteAddr dari XFF/X-Real-IP; kita menolak
	// permintaan yang membawa header itu sama sekali, jadi tidak ada cara
	// client eksternal lewat nginx bisa memakai jalur ini.
	if strings.TrimSpace(r.Header.Get("X-Forwarded-For")) != "" ||
		strings.TrimSpace(r.Header.Get("X-Real-IP")) != "" {
		return nil, nil
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, nil
	}
	actAs := strings.TrimSpace(r.Header.Get("X-Act-As-User"))
	if actAs == "" {
		return nil, nil
	}
	var user AuthUser
	var displayName *string
	err = a.Pool.QueryRow(r.Context(),
		"SELECT id, email, display_name, role, created_at FROM users WHERE id = $1 AND is_active = TRUE AND approved_at IS NOT NULL LIMIT 1",
		actAs,
	).Scan(&user.ID, &user.Email, &displayName, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, nil
	}
	if displayName != nil {
		user.DisplayName = *displayName
	}
	agent := &Agent{
		ClientID:   strings.TrimSpace(r.Header.Get("X-Agent-Client-ID")),
		ActionID:   strings.TrimSpace(r.Header.Get("X-Agent-Action-ID")),
		ActionCode: strings.TrimSpace(r.Header.Get("X-Agent-Action-Code")),
	}
	return &user, agent
}
