package authdomain

import (
	"encoding/json"
	"net/http"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/auth/2fa/{setup,enable,disable} — pesan & urutan cek persis.

// TwoFactorSetup: secret baru (belum aktif), seal per-user, kirim otpauth URL.
func (d *Deps) TwoFactorSetup(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	secret, err := auth.GenerateTotpSecret(20)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	sealed, err := crypto.SealV3(secret, d.Cfg.Secret, user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE users SET two_factor_secret = $1, two_factor_enabled = FALSE WHERE id = $2",
		sealed, user.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"secret":       secret,
		"otpauth_url":  auth.TotpAuthUrl(secret, user.Email, ""),
	})
}

// TwoFactorEnable: verifikasi kode pertama → aktif + recovery codes (raw sekali).
func (d *Deps) TwoFactorEnable(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := auth.CheckAuthRate(r.Context(), d.Pool, "2fa-enable:"+user.ID, 5); err != nil {
		httpx.Error(w, http.StatusTooManyRequests, "Terlalu banyak percobaan. Tunggu satu menit.")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	code := trim(body.Code)

	var twoFactorSecret *string
	var twoFactorEnabled bool
	err := d.Pool.QueryRow(r.Context(),
		"SELECT two_factor_secret, two_factor_enabled FROM users WHERE id = $1 LIMIT 1",
		user.ID).Scan(&twoFactorSecret, &twoFactorEnabled)
	if err != nil || twoFactorSecret == nil {
		httpx.Error(w, http.StatusConflict, "Jalankan setup 2FA terlebih dahulu.")
		return
	}
	if twoFactorEnabled {
		httpx.Error(w, http.StatusConflict, "2FA sudah aktif.")
		return
	}

	secret, err := crypto.UnsealV3(*twoFactorSecret, d.Cfg.Secret, user.ID)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Kode verifikasi salah. Coba lagi.")
		return
	}
	if !auth.VerifyTotp(secret, code) {
		httpx.Error(w, http.StatusUnauthorized, "Kode verifikasi salah. Coba lagi.")
		return
	}

	rawCodes, hashedCodes := auth.GenerateRecoveryCodes(10)
	hashedJSON, _ := json.Marshal(hashedCodes)
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE users SET two_factor_enabled = TRUE, two_factor_recovery_codes = $1 WHERE id = $2",
		hashedJSON, user.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, Action: "auth.2fa_enabled", TargetType: strPtr("auth"), TargetID: &user.ID,
	})
	d.Auth.InvalidateAuthCache(user.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "recovery_codes": rawCodes})
}

// TwoFactorDisable: wajib password (re-auth) supaya sesi curian tak bisa mematikan 2FA.
func (d *Deps) TwoFactorDisable(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := auth.CheckAuthRate(r.Context(), d.Pool, "2fa-disable:"+user.ID, 5); err != nil {
		httpx.Error(w, http.StatusTooManyRequests, "Terlalu banyak percobaan. Tunggu satu menit.")
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Password == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Password wajib diisi untuk menonaktifkan 2FA.")
		return
	}

	var passwordHash string
	var twoFactorEnabled bool
	err := d.Pool.QueryRow(r.Context(),
		"SELECT password_hash, two_factor_enabled FROM users WHERE id = $1 LIMIT 1",
		user.ID).Scan(&passwordHash, &twoFactorEnabled)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if !twoFactorEnabled {
		httpx.Error(w, http.StatusConflict, "2FA belum aktif.")
		return
	}
	if !auth.VerifyPassword(body.Password, passwordHash) {
		httpx.Error(w, http.StatusUnauthorized, "Password salah.")
		return
	}

	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE users SET two_factor_enabled = FALSE, two_factor_secret = NULL, two_factor_recovery_codes = NULL WHERE id = $1",
		user.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, Action: "auth.2fa_disabled", TargetType: strPtr("auth"), TargetID: &user.ID,
	})
	d.Auth.InvalidateAuthCache(user.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false})
}

// NotificationPreferences: GET/PUT /v1/me/notification-preferences (parity).
func (d *Deps) NotificationPreferencesGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	var waNumber, emailAlt1, emailAlt2 *string
	var waConsent bool
	err := d.Pool.QueryRow(r.Context(),
		"SELECT wa_number, wa_consent, email_alt1, email_alt2 FROM notification_preferences WHERE user_id = $1 LIMIT 1",
		user.ID).Scan(&waNumber, &waConsent, &emailAlt1, &emailAlt2)
	resp := map[string]any{
		"wa_number":  "",
		"wa_consent": false,
		"email_alt1": "",
		"email_alt2": "",
	}
	if err == nil {
		if waNumber != nil {
			resp["wa_number"] = *waNumber
		}
		resp["wa_consent"] = waConsent
		if emailAlt1 != nil {
			resp["email_alt1"] = *emailAlt1
		}
		if emailAlt2 != nil {
			resp["email_alt2"] = *emailAlt2
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (d *Deps) NotificationPreferencesPut(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	var body struct {
		WaNumber  *string `json:"wa_number"`
		WaConsent *bool   `json:"wa_consent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Payload tidak valid")
		return
	}
	waNumber := ""
	if body.WaNumber != nil {
		waNumber = trim(*body.WaNumber)
	}
	if len(waNumber) > 30 || body.WaConsent == nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Payload tidak valid")
		return
	}
	var waNumberVal *string
	if waNumber != "" {
		waNumberVal = &waNumber
	}
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO notification_preferences (user_id, wa_number, wa_consent, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
		  wa_number = EXCLUDED.wa_number,
		  wa_consent = EXCLUDED.wa_consent,
		  updated_at = NOW()`,
		user.ID, waNumberVal, *body.WaConsent); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
