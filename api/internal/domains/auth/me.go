package authdomain

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/auth/me/route.ts — GET profil + PATCH display_name (2..120).

func (d *Deps) Me(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"role":         user.Role,
		"display_name": user.DisplayName,
		"created_at":   isoMilli(user.CreatedAt),
	})
}

func (d *Deps) MeUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}

	var body struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "display_name must be 2-120 characters")
		return
	}
	displayName := strings.TrimSpace(body.DisplayName)
	if len(displayName) < 2 || len(displayName) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "display_name must be 2-120 characters")
		return
	}

	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE users SET display_name = $1 WHERE id = $2", displayName, user.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"role":         user.Role,
		"display_name": displayName,
		"created_at":   isoMilli(user.CreatedAt),
	})
}

// TurnstileConfig: publik — {enabled, site_key} no-store (parity route turnstile).
func (d *Deps) TurnstileConfig(w http.ResponseWriter, r *http.Request) {
	enabled, siteKey := d.Turnstile.PublicSiteKey(r.Context())
	resp := map[string]any{"enabled": enabled, "site_key": nil}
	if siteKey != nil {
		resp["site_key"] = *siteKey
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

// TwoFactorVerify: step-2 login 2FA (port v1/auth/2fa/verify/route.ts).
func (d *Deps) TwoFactorVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Challenge dan kode wajib diisi.")
		return
	}
	code := strings.TrimSpace(body.Code)
	if body.Challenge == "" || code == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Challenge dan kode wajib diisi.")
		return
	}

	ip := authRequestIp(r)
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "2fa:ip:"+orUnknown(ip), 10); err != nil {
		writeRateErr(w, err)
		return
	}
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "2fa:code:"+truncate24(body.Challenge), 5); err != nil {
		writeRateErr(w, err)
		return
	}

	payload, ok := auth.VerifyTicket(d.Cfg.Secret, body.Challenge)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Sesi verifikasi kedaluwarsa. Login ulang.")
		return
	}

	var id, email, role string
	var displayName *string
	var isActive bool
	var approvedAt *time.Time
	var twoFactorSecret *string
	var recoveryCodes []byte
	err := d.Pool.QueryRow(r.Context(), `
		SELECT id, email, display_name, role, is_active, approved_at, two_factor_secret, two_factor_recovery_codes
		FROM users WHERE id = $1 LIMIT 1`, payload.UID,
	).Scan(&id, &email, &displayName, &role, &isActive, &approvedAt, &twoFactorSecret, &recoveryCodes)
	if err != nil || !isActive || approvedAt == nil || twoFactorSecret == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid challenge")
		return
	}

	secret, err := crypto.UnsealV3(*twoFactorSecret, d.Cfg.Secret, id)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid challenge")
		return
	}

	if isSixDigits(code) {
		if !auth.VerifyTotp(secret, code) {
			d.auditLogin(r, &id, email, false, ip, "2fa-wrong-code")
			httpx.Error(w, http.StatusUnauthorized, "Kode verifikasi salah.")
			return
		}
	} else {
		// Recovery code: sekali pakai, hash cocok → hapus dari daftar (jsonb array).
		hashed := auth.HashRecoveryCode(code)
		var stored []string
		if len(recoveryCodes) > 0 {
			if err := json.Unmarshal(recoveryCodes, &stored); err != nil {
				stored = nil
			}
		}
		found := false
		for _, h := range stored {
			if h == hashed {
				found = true
				break
			}
		}
		if !found {
			d.auditLogin(r, &id, email, false, ip, "2fa-wrong-recovery")
			httpx.Error(w, http.StatusUnauthorized, "Kode pemulihan salah.")
			return
		}
		remaining := make([]string, 0, len(stored))
		for _, h := range stored {
			if h != hashed {
				remaining = append(remaining, h)
			}
		}
		remainingJSON, _ := json.Marshal(remaining)
		if _, err := d.Pool.Exec(r.Context(),
			"UPDATE users SET two_factor_recovery_codes = $1 WHERE id = $2", remainingJSON, id); err != nil {
			httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
			return
		}
	}

	d.auditLogin(r, &id, email, true, ip, "2fa-ok")
	d.issueSession(w, r, id, SessionUser{ID: id, Email: email, Role: role, DisplayName: displayName}, http.StatusOK)
}
