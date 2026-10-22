package authdomain

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// SSRGate: context gate untuk rendering server-side (SvelteKit +layout.server).
//
// Port perilaku gate server-side app lama (admin/layout.tsx, mitra/layout.tsx,
// klien/[brandId]/gate.ts) yang di era Go tidak bisa lagi query DB langsung
// dari frontend. Berbeda dari /v1/auth/refresh, endpoint ini TIDAK merotasi
// sesi — hanya memverifikasi refresh_token lalu membaca role/admin/partner/
// membership supaya layout server bisa redirect sebelum HTML terkirim.
//
// - refresh_token tidak valid / sesi revoked / user non-aktif → 401
// - admin  : baris platform_admins (role + permissions)
// - partner: partners WHERE status='active' (+ status VIP untuk gate klien)
// - brand  : BrandAccess viewer (parity gate.ts — unknown/non-member → null,
//            caller yang memutuskan redirect /klien)

type ssrGateBody struct {
	RefreshToken string `json:"refresh_token"`
	BrandID      string `json:"brand_id"`
}

func (d *Deps) SSRGate(w http.ResponseWriter, r *http.Request) {
	var body ssrGateBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	token := body.RefreshToken
	if token == "" {
		if c, err := r.Cookie("refresh_token"); err == nil {
			token = c.Value
		}
	}
	if token == "" {
		httpx.Error(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	claims, err := auth.VerifyRefreshToken(d.Cfg.Secret, d.Cfg.AppName, token)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	// Rate limit per pengguna, bukan per IP: gate ini dipanggil setiap layout
	// server-side (admin/klien/mitra), jadi kunci per IP membuat semua pengguna
	// di satu jaringan berbagi satu kuota dan pengguna aktif cepat terkunci.
	// Token sudah diverifikasi di atas, jadi `sub` (user ID) tepercaya.
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "ssr:user:"+orUnknown(claims.Sub), d.Cfg.SSRGateRateLimitPerMinute); err != nil {
		writeRateErr(w, err)
		return
	}

	var sessionRevoked bool
	var sessionExpiresAt time.Time
	err = d.Pool.QueryRow(r.Context(),
		"SELECT revoked, expires_at FROM user_sessions WHERE refresh_jti = $1 LIMIT 1",
		claims.JTI,
	).Scan(&sessionRevoked, &sessionExpiresAt)
	if err != nil || sessionRevoked || !sessionExpiresAt.After(time.Now()) {
		httpx.Error(w, http.StatusUnauthorized, "Refresh token revoked")
		return
	}

	var u SessionUser
	var displayName *string
	var createdAt time.Time
	err = d.Pool.QueryRow(r.Context(),
		"SELECT id, email, display_name, role, created_at FROM users WHERE id = $1 AND is_active = TRUE LIMIT 1",
		claims.Sub,
	).Scan(&u.ID, &u.Email, &displayName, &u.Role, &createdAt)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "User no longer active")
		return
	}
	u.DisplayName = displayName
	u.CreatedAt = &createdAt

	resp := map[string]any{
		"user": map[string]any{
			"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "role": u.Role,
		},
	}

	// Context admin (parity admin/layout.tsx → getAdminContext).
	var adminRole string
	var permissionsRaw []byte
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT role, permissions FROM platform_admins WHERE user_id = $1 LIMIT 1", u.ID,
	).Scan(&adminRole, &permissionsRaw); err == nil {
		permissions := map[string]any{}
		_ = json.Unmarshal(permissionsRaw, &permissions)
		resp["admin"] = map[string]any{"role": adminRole, "permissions": permissions}
	}

	// Partner aktif (parity gate.ts & layout mitra).
	var partnerID string
	var partnerPublicID sql.NullString
	var isVip bool
	var vipTier sql.NullString
	var vipExpiresAt *time.Time
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT id, public_id, is_vip, vip_tier, vip_expires_at FROM partners WHERE user_id = $1 AND status = 'active' LIMIT 1", u.ID,
	).Scan(&partnerID, &partnerPublicID, &isVip, &vipTier, &vipExpiresAt); err == nil {
		resp["partner"] = map[string]any{
			"id": partnerID, "public_id": nullStrOr(partnerPublicID, partnerID),
			"is_vip": isVip, "vip_tier": nullStrOr(vipTier, "regular"), "vip_expires_at": vipExpiresAt,
		}
	}

	// Akses brand (parity gate.ts: membership wajib, brand unknown → null).
	if body.BrandID != "" {
		brandRow, brandErr := brands.BrandAccess(r.Context(), d.Pool, body.BrandID, u.ID, "viewer")
		if brandErr == nil {
			resp["brand"] = map[string]any{
				"id": brandRow.ID, "public_id": derefStr(brandRow.PublicID), "role": brandRow.Role,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

func nullStrOr(v sql.NullString, fallback string) string {
	if v.Valid && v.String != "" {
		return v.String
	}
	return fallback
}
