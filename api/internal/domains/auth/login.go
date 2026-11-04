package authdomain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/auth/login/route.ts — urutan cek, pesan error, audit, dan lockout
// HARUS identik (parity).

const maxFailedLogins = 5
const lockMinutes = 15

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type loginBody struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	TurnstileToken string `json:"turnstile_token"`
}

type userLoginRow struct {
	ID                     string
	Email                  string
	DisplayName            *string
	Role                   string
	IsActive               bool
	ApprovedAt             *time.Time
	CreatedAt              time.Time
	PasswordHash           string
	FailedLoginCount       int
	LockedUntil            *time.Time
	TwoFactorEnabled       bool
	TwoFactorSecret        *string
	TwoFactorRecoveryCodes []byte
}

func (d *Deps) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	password := body.Password
	// parity loginSchema: email format max 320, password 1..128
	if !emailRe.MatchString(email) || len(email) > 320 || len(password) < 1 || len(password) > 128 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid email or password format")
		return
	}

	ip := auth.RequestIp(r.Header)

	// Rate limit (sebelum cek kredensial): brute-force & credential stuffing.
	if err := auth.CheckAuthRate(r.Context(), d.Pool, fmt.Sprintf("login:ip:%s", orUnknown(ip)), 10); err != nil {
		writeRateErr(w, err)
		return
	}
	if err := auth.CheckAuthRate(r.Context(), d.Pool, fmt.Sprintf("login:email:%s", email), 5); err != nil {
		writeRateErr(w, err)
		return
	}

	// Cloudflare Turnstile (wajib bila enabled di config admin).
	if err := d.Turnstile.RequireToken(r.Context(), body.TurnstileToken, ip); err != nil {
		if te, ok := err.(*auth.TurnstileError); ok {
			httpx.Error(w, http.StatusForbidden, te.Message)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}

	var u userLoginRow
	err := d.Pool.QueryRow(r.Context(), `
		SELECT id, email, display_name, role, is_active, approved_at, created_at, password_hash,
		       failed_login_count, locked_until, two_factor_enabled, two_factor_secret, two_factor_recovery_codes
		FROM users WHERE email = $1 LIMIT 1`, email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.IsActive, &u.ApprovedAt, &u.CreatedAt, &u.PasswordHash,
		&u.FailedLoginCount, &u.LockedUntil, &u.TwoFactorEnabled, &u.TwoFactorSecret, &u.TwoFactorRecoveryCodes)
	if err != nil {
		d.auditLogin(r, nil, email, false, ip, "unknown-email")
		httpx.Error(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	if u.ApprovedAt == nil {
		httpx.Error(w, http.StatusForbidden, "Harap tunggu persetujuan admin")
		return
	}
	if !u.IsActive {
		if d.isSuspendedPartner(r.Context(), u.ID) {
			d.auditLogin(r, &u.ID, email, false, ip, "suspended")
			httpx.Error(w, http.StatusForbidden, suspendedMessage)
			return
		}
		d.auditLogin(r, &u.ID, email, false, ip, "inactive")
		httpx.Error(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Lockout: 5x gagal → kunci 15 menit (counter reset saat sukses).
	if u.LockedUntil != nil && u.LockedUntil.After(time.Now()) {
		d.auditLogin(r, &u.ID, email, false, ip, "locked")
		httpx.Error(w, http.StatusTooManyRequests, "Akun terkunci sementara karena terlalu banyak percobaan gagal. Coba lagi nanti.")
		return
	}

	if !auth.VerifyPassword(password, u.PasswordHash) {
		failed := u.FailedLoginCount + 1
		var lock *time.Time
		if failed >= maxFailedLogins {
			t := time.Now().Add(lockMinutes * time.Minute)
			lock = &t
		}
		_, _ = d.Pool.Exec(r.Context(),
			"UPDATE users SET failed_login_count = $1, locked_until = $2 WHERE id = $3",
			failed, lock, u.ID)
		d.auditLogin(r, &u.ID, email, false, ip, fmt.Sprintf("wrong-password(%d)", failed))
		httpx.Error(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// 2FA aktif → jangan terbitkan token; minta kode TOTP/recovery (one-shot ticket).
	if u.TwoFactorEnabled && u.TwoFactorSecret != nil {
		challenge, err := auth.SignChallenge(d.Cfg.Secret, auth.ChallengePayload{UID: u.ID, Email: u.Email})
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"two_factor_required": true, "challenge": challenge})
		return
	}

	// Login sukses: reset counter gagal + audit.
	if u.FailedLoginCount > 0 || u.LockedUntil != nil {
		_, _ = d.Pool.Exec(r.Context(),
			"UPDATE users SET failed_login_count = 0, locked_until = NULL WHERE id = $1", u.ID)
	}
	d.auditLogin(r, &u.ID, email, true, ip, "password")

	d.issueSession(w, r, u.ID, SessionUser{
		ID: u.ID, Email: u.Email, Role: u.Role, DisplayName: u.DisplayName, CreatedAt: &u.CreatedAt,
	}, http.StatusOK)
}

// suspendedMessage: akun mitra ditangguhkan admin (partners.status = inactive).
const suspendedMessage = "Akun Anda ditangguhkan. Hubungi care@tauthub.com."

// isSuspendedPartner: user nonaktif yang partnernya berstatus inactive →
// pesan "ditangguhkan" (bukan "email/password salah").
func (d *Deps) isSuspendedPartner(ctx context.Context, userID string) bool {
	var one int
	err := d.Pool.QueryRow(ctx,
		"SELECT 1 FROM partners WHERE user_id = $1 AND status = 'inactive' LIMIT 1", userID).Scan(&one)
	return err == nil
}

func writeRateErr(w http.ResponseWriter, err error) {
	if _, ok := err.(*auth.AuthRateLimitError); ok {
		httpx.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}
	slog.Error("rate limit error", "err", err)
	httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500("production", "internal"))
}

func orUnknown(ip string) string {
	if ip == "" {
		return "unknown"
	}
	return ip
}

// auditLogin: best-effort — tidak boleh menggagalkan auth (parity login route).
func (d *Deps) auditLogin(r *http.Request, userID *string, email string, ok bool, ip, detail string) {
	action := "login.failed"
	if ok {
		action = "login.success"
	}
	auditID, err := uuid.NewRandom()
	if err != nil {
		return
	}
	_, _ = d.Pool.Exec(r.Context(),
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'auth', $4)",
		auditID.String(), userID, action, email)
	slog.Info(fmt.Sprintf("[auth] %s email=%s ip=%s %s", action, email, orUnknown(ip), detail))
}
