package authdomain

import (
	"encoding/json"
	"net/http"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/auth/change-password + forgot-password + reset-password.
// Pesan, status code, dan anti-enumerasi dipertahankan persis.

// ChangePassword: wajib Bearer; revoke sesi lain kecuali perangkat ini.
func (d *Deps) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}

	if err := auth.CheckAuthRate(r.Context(), d.Pool, "change-pass:ip:"+orUnknown(authRequestIp(r)), d.Cfg.AuthRateLimitPerMinute); err != nil {
		writeRateErr(w, err)
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid payload")
		return
	}
	// parity passwordChangeSchema: current 1..128, new 10..128
	if len(body.CurrentPassword) < 1 || len(body.CurrentPassword) > 128 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Password baru minimal 10 karakter.")
		return
	}
	if len(body.NewPassword) < 10 || len(body.NewPassword) > 128 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String should have at least 10 characters")
		return
	}

	var passwordHash string
	err := d.Pool.QueryRow(r.Context(),
		"SELECT password_hash FROM users WHERE id = $1 LIMIT 1", user.ID).Scan(&passwordHash)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if !auth.VerifyPassword(body.CurrentPassword, passwordHash) {
		httpx.Error(w, http.StatusBadRequest, "Current password is incorrect")
		return
	}

	nextHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE users SET password_hash = $1 WHERE id = $2", nextHash, user.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	d.Auth.InvalidateAuthCache(user.ID)
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, Action: "auth.password.change", TargetType: strPtr("user"), TargetID: &user.ID,
	})

	// Cabut sesi lain (refresh token) kecuali yang sedang dipakai.
	if c, err := r.Cookie("refresh_token"); err == nil && c.Value != "" {
		if claims, err := auth.VerifyRefreshToken(d.Cfg.Secret, d.Cfg.AppName, c.Value); err == nil {
			_, _ = d.Pool.Exec(r.Context(),
				"UPDATE user_sessions SET revoked = TRUE WHERE user_id = $1 AND refresh_jti <> $2",
				user.ID, claims.JTI)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	_, _ = d.Pool.Exec(r.Context(),
		"UPDATE user_sessions SET revoked = TRUE WHERE user_id = $1", user.ID)
	w.WriteHeader(http.StatusNoContent)
}

// ForgotPassword: anti-enumerasi total — SELALU {ok:true}, bahkan saat 429.
func (d *Deps) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ok := func() {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
	ip := orUnknown(authRequestIp(r))
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "forgot:ip:"+ip, 5); err != nil {
		ok() // parity: rate limit pun dibalas ok:true
		return
	}

	var body struct {
		Email          string `json:"email"`
		TurnstileToken string `json:"turnstile_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	email := lower(trim(body.Email))
	// parity forgotSchema: email max 200 + format
	if email == "" || len(email) > 200 || !emailRe.MatchString(email) {
		ok()
		return
	}
	if err := d.Turnstile.RequireToken(r.Context(), body.TurnstileToken, authRequestIp(r)); err != nil {
		ok() // parity: turnstile gagal → tetap ok:true agar tak membuka enumerasi
		return
	}

	var userID, userEmail string
	err := d.Pool.QueryRow(r.Context(),
		"SELECT id, email FROM users WHERE lower(email) = $1 AND is_active = TRUE LIMIT 1", email).Scan(&userID, &userEmail)
	if err != nil {
		ok()
		return
	}

	rawToken, err := auth.CreateResetToken(r.Context(), d.Pool, userID, 10)
	if err != nil {
		ok()
		return
	}
	// TODO(fase 12): kirim email "password-reset" via email queue (lib/email).
	// Kontrak API tetap {ok:true}; link: {APP_PUBLIC_URL}/reset-password?token=<raw>
	_ = rawToken
	appURL := d.Cfg.PublicAppURL
	_ = appURL
	ok()
}

// ResetPassword: konsumsi token atomik + revoke semua sesi.
func (d *Deps) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "reset:ip:"+orUnknown(authRequestIp(r)), d.Cfg.AuthRateLimitPerMinute); err != nil {
		writeRateErr(w, err)
		return
	}

	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Token reset tidak valid atau sudah kedaluwarsa.")
		return
	}
	token := trim(body.Token)
	if token == "" || len(token) > 200 || len(body.NewPassword) < 10 || len(body.NewPassword) > 200 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String should have at least 10 characters")
		return
	}

	result := auth.VerifyResetToken(r.Context(), d.Pool, token)
	if !result.Valid || result.UserID == "" || result.TokenID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Token reset tidak valid atau sudah kedaluwarsa.")
		return
	}

	passwordHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}

	tx, txErr := d.Pool.Begin(r.Context())
	if txErr != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, txErr.Error()))
		return
	}
	if _, err := tx.Exec(r.Context(), "SELECT id FROM users WHERE id = $1 FOR UPDATE", result.UserID); err != nil {
		_ = tx.Rollback(r.Context())
		httpx.Error(w, http.StatusUnprocessableEntity, "Token reset tidak valid atau sudah kedaluwarsa.")
		return
	}
	if !auth.ClaimResetToken(r.Context(), tx, result.TokenID) {
		_ = tx.Rollback(r.Context())
		httpx.Error(w, http.StatusUnprocessableEntity, "Token reset tidak valid atau sudah kedaluwarsa.")
		return
	}
	if _, err := tx.Exec(r.Context(),
		"UPDATE users SET password_hash = $1 WHERE id = $2", passwordHash, result.UserID); err != nil {
		_ = tx.Rollback(r.Context())
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	if _, err := tx.Exec(r.Context(),
		"UPDATE password_reset_tokens SET consumed_at = NOW() WHERE user_id = $1 AND consumed_at IS NULL",
		result.UserID); err != nil {
		_ = tx.Rollback(r.Context())
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	if _, err := tx.Exec(r.Context(),
		"UPDATE user_sessions SET revoked = TRUE WHERE user_id = $1", result.UserID); err != nil {
		_ = tx.Rollback(r.Context())
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}

	d.Auth.InvalidateAuthCache(result.UserID)
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &result.UserID, Action: "auth.password.reset", TargetType: strPtr("user"), TargetID: &result.UserID,
	})
	// TODO(fase 12): email konfirmasi "password-changed" (best-effort, parity).

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func strPtr(s string) *string { return &s }
