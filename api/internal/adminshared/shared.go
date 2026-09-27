// Package adminshared: fondasi kontrol-plane admin (port src/auth/rbac.ts
// bagian platform + helper respons bersama). Dipakai semua paket admin/platform
// supaya error envelope, permission check, dan serialisasi konsisten.
package adminshared

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// AdminPermissions: parity ADMIN_PERMISSIONS di src/auth/rbac.ts.
var AdminPermissions = []string{
	"media", "publications", "settlements", "payments", "topup",
	"errors", "users", "partners", "brands", "content", "pr", "integrations",
}

type Context struct {
	UserID      string
	Email       string
	Role        string // super_admin | admin
	Permissions map[string]bool
	CreatedAt   time.Time
}

func (c Context) IsSuperAdmin() bool { return c.Role == "super_admin" }

// Can: super_admin selalu boleh; admin biasa mengikuti flag permission.
func (c Context) Can(key string) bool {
	if c.IsSuperAdmin() {
		return true
	}
	return c.Permissions[key]
}

// allowedPermissions: daftar permission yang aktif (untuk payload me/access).
func (c Context) AllowedPermissions() map[string]bool {
	out := make(map[string]bool, len(AdminPermissions))
	for _, key := range AdminPermissions {
		out[key] = c.Can(key)
	}
	return out
}

// ---------- error ----------

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func Err(status int, message string) *Error { return &Error{status, message} }
func ErrNotFound(msg string) *Error         { return &Error{http.StatusNotFound, msg} }
func ErrForbidden(msg string) *Error        { return &Error{http.StatusForbidden, msg} }

// ---------- dependencies ----------

type Deps struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

// ---------- cache konteks admin (parity ADMIN_CACHE 30s) ----------

type cachedAdmin struct {
	ctx       *Context
	expiresAt time.Time
}

var (
	adminCache    = map[string]cachedAdmin{}
	adminCacheTTL = 30 * time.Second
)

// InvalidateAdminCache: dipakai saat admin diubah/dihapus.
func InvalidateAdminCache(userID string) {
	if userID == "" {
		adminCache = map[string]cachedAdmin{}
		return
	}
	delete(adminCache, userID)
}

// AdminContext: port getAdminContext — platform_admins, fallback env
// PLATFORM_ADMIN_EMAILS sebagai super admin.
func (d *Deps) AdminContext(ctx context.Context, userID, email string) *Context {
	if cached, ok := adminCache[userID]; ok && cached.expiresAt.After(time.Now()) {
		return cached.ctx
	}
	var role string
	var permissions []byte
	err := d.Pool.QueryRow(ctx,
		"SELECT role, permissions FROM platform_admins WHERE user_id = $1 LIMIT 1", userID).
		Scan(&role, &permissions)
	if err == nil {
		parsed := map[string]bool{}
		if len(permissions) > 0 {
			_ = json.Unmarshal(permissions, &parsed)
		}
		result := &Context{UserID: userID, Email: email, Role: role, Permissions: parsed, CreatedAt: time.Now()}
		adminCache[userID] = cachedAdmin{result, time.Now().Add(adminCacheTTL)}
		return result
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("[admin] gagal baca platform_admins", "err", err)
	}
	for _, allowed := range d.Cfg.AdminEmails {
		if allowed == "" {
			continue
		}
		if equalFold(allowed, email) {
			result := &Context{UserID: userID, Email: email, Role: "super_admin",
				Permissions: map[string]bool{}, CreatedAt: time.Now()}
			adminCache[userID] = cachedAdmin{result, time.Now().Add(adminCacheTTL)}
			return result
		}
	}
	return nil
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Require: user terautentikasi + platform admin; opsional cek permission.
// Dipakai sebagai guard awal setiap handler admin.
func (d *Deps) Require(w http.ResponseWriter, r *http.Request, permission string) (*httpx.AuthUser, Context, bool) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return nil, Context{}, false
	}
	ctx := d.AdminContext(r.Context(), user.ID, user.Email)
	if ctx == nil {
		// Parity app lama: kontrol-plane mengembalikan 404 (bukan 403) supaya
		// keberadaan resource admin tidak terbaca oleh akun biasa. Layout web
		// yang mengarahkan non-admin ke areanya sendiri.
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return nil, Context{}, false
	}
	if permission != "" && !ctx.Can(permission) {
		httpx.Error(w, http.StatusForbidden, "Anda tidak memiliki akses ke modul ini")
		return nil, Context{}, false
	}
	return user, *ctx, true
}

// WriteErr: pemetaan error seragam (parity errorResponse).
func (d *Deps) WriteErr(w http.ResponseWriter, err error) {
	var ae *Error
	if errors.As(err, &ae) {
		httpx.Error(w, ae.Status, ae.Message)
		return
	}
	slog.Error("[admin]", "err", err)
	httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// ---------- helper umum ----------

func ChiParam(r *http.Request, name string) string {
	return chiParam(r, name)
}

// ClampInt: batas limit/offset query.
func ClampInt(raw string, def, min, max int) int {
	return clampInt(raw, def, min, max)
}

// IsoMilli: serialisasi waktu parity JSON.stringify(Date).
func IsoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// DecodeJSON: body kosong/invalid → map kosong (parity request.json().catch(()=>({}))).
func DecodeJSON(r *http.Request) map[string]any {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		return map[string]any{}
	}
	return body
}

// StrField: ambil string dari body bila ada.
func StrField(body map[string]any, key string) (string, bool) {
	v, ok := body[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return trimSpace(s), true
}

func BoolField(body map[string]any, key string) (bool, bool) {
	v, ok := body[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	if !ok {
		return false, false
	}
	return b, true
}

func NumField(body map[string]any, key string) (float64, bool) {
	v, ok := body[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		f, err := parseFloat(trimSpace(n))
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func IntField(body map[string]any, key string) (int, bool) {
	f, ok := NumField(body, key)
	if !ok {
		return 0, false
	}
	return int(f), true
}

// Page: helper limit/offset standar (limit 1..max).
type Page struct {
	Limit  int
	Offset int
}

func ParsePage(r *http.Request, defLimit, maxLimit int) Page {
	q := r.URL.Query()
	return Page{
		Limit:  clampInt(q.Get("limit"), defLimit, 1, maxLimit),
		Offset: clampInt(q.Get("offset"), 0, 0, 1<<30),
	}
}
