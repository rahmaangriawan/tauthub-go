package platform

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Port app/v1/platform/{link-clicks,oauth/google,server-config,short-link-config}.
// Empat endpoint ini hanya super admin / platform admin dan tidak masuk modul
// lain, jadi ditempatkan di satu file.

// ---------- GET /v1/platform/link-clicks ----------

// LinkClicksGet: detail klik short link untuk ADMIN — memuat IP mentah
// (versi klien sengaja tidak memuat IP, UU PDP). Super admin saja.
func (d *Deps) LinkClicksGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	now := time.Now()
	q := r.URL.Query()
	normDate := func(value, fallback string) string {
		value = strings.TrimSpace(value)
		if len(value) < 10 {
			return fallback
		}
		candidate := value[:10]
		if _, err := time.Parse("2006-01-02", candidate); err != nil {
			return fallback
		}
		return candidate
	}
	from := normDate(q.Get("from"), now.AddDate(0, 0, -30).Format("2006-01-02"))
	to := normDate(q.Get("to"), now.Format("2006-01-02"))
	toDate, _ := time.Parse("2006-01-02", to)
	toExclusive := toDate.AddDate(0, 0, 1).UTC().Format(time.RFC3339)
	fromISO := from + "T00:00:00.000Z"
	limit := clampInt(q.Get("limit"), 100, 1, 300)
	brandFilter := strings.TrimSpace(q.Get("brandId"))

	args := []any{fromISO, toExclusive}
	clause := "WHERE e.event_type = 'tracking_link_click' AND e.created_at >= $1 AND e.created_at < $2"
	if brandFilter != "" {
		args = append(args, brandFilter)
		clause += " AND e.brand_id = $" + itoa(len(args))
	}
	args = append(args, limit)

	items, err := d.queryRows(ctx, `
		SELECT e.created_at, l.code, l.kind, e.url, e.session_id, e.ip_address, e.user_agent, e.referrer,
		       c.name AS campaign_name, b.name AS brand_name, b.id AS brand_id
		FROM tracking_events e
		LEFT JOIN tracking_links l ON l.id = e.link_id
		LEFT JOIN campaigns c ON c.id = e.campaign_id
		LEFT JOIN brands b ON b.id = e.brand_id
		`+clause+`
		ORDER BY e.created_at DESC LIMIT $`+itoa(len(args)), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, row := range items {
		out = append(out, map[string]any{
			"created_at":    row.timePtr("created_at"),
			"code":          row.raw("code"),
			"kind":          row.raw("kind"),
			"url":           row.raw("url"),
			"session_id":    row.raw("session_id"),
			"ip_address":    row.raw("ip_address"),
			"user_agent":    row.raw("user_agent"),
			"referrer":      row.raw("referrer"),
			"campaign_name": row.raw("campaign_name"),
			"brand_name":    row.raw("brand_name"),
			"brand_id":      row.raw("brand_id"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from":  from,
		"to":    to,
		"limit": limit,
		"items": out,
	})
}

// ---------- GET/PUT /v1/platform/oauth/google ----------

const googleCallbackURL = "https://app.tauthub.com/v1/auth/google/callback"

func (d *Deps) OAuthGoogleGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT client_id, redirect_uri, updated_at FROM platform_oauth_configs WHERE id = 'google' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"provider":     "google",
		"client_id":    row.raw("client_id"),
		"redirect_uri": row.raw("redirect_uri"),
		"callback_url": googleCallbackURL,
		"updated_at":   row.timePtr("updated_at"),
	})
}

func (d *Deps) OAuthGooglePut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	clientID, present := strField(body, "client_id")
	if !present || clientID == "" || len(clientID) > 500 {
		writeErr(w, d, errUnprocessable("Invalid input"))
		return
	}
	clientSecret, _ := strField(body, "client_secret")
	if len(clientSecret) > 1000 {
		writeErr(w, d, errUnprocessable("String must contain at most 1000 character(s)"))
		return
	}
	redirectURI, present := strField(body, "redirect_uri")
	if !present || redirectURI == "" || len(redirectURI) > 500 || !isAbsURL(redirectURI) {
		writeErr(w, d, errUnprocessable("Invalid url"))
		return
	}

	var sealed any
	if clientSecret != "" {
		value, err := d.sealT2Entity(clientSecret, "google")
		if err != nil {
			writeErr(w, d, err)
			return
		}
		sealed = value
	} else {
		existing, _ := d.queryRow(r.Context(),
			"SELECT sealed_client_secret FROM platform_oauth_configs WHERE id = 'google' LIMIT 1")
		if existing == nil || !existing.has("sealed_client_secret") {
			writeErr(w, d, errUnprocessable("client_secret wajib diisi saat pertama kali"))
			return
		}
	}

	if err := d.exec(r.Context(), `
		INSERT INTO platform_oauth_configs (id, provider, client_id, sealed_client_secret, redirect_uri, updated_by, updated_at)
		VALUES ('google', 'google', $1, $2, $3, $4, NOW())
		ON CONFLICT (id) DO UPDATE SET
		  client_id = EXCLUDED.client_id,
		  sealed_client_secret = COALESCE(EXCLUDED.sealed_client_secret, platform_oauth_configs.sealed_client_secret),
		  redirect_uri = EXCLUDED.redirect_uri,
		  updated_by = EXCLUDED.updated_by,
		  updated_at = NOW()`,
		clientID, sealed, redirectURI, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "oauth.google_updated", "platform_oauth_config", "google")
	ai.InvalidateRuntimeCache()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- GET/PUT /v1/platform/server-config ----------

// ServerConfigGet: IP publik server (migrasi 158) — nilai tersegel tidak pernah
// dikirim ke klien, hanya IP + asalnya.
func (d *Deps) ServerConfigGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, d.describeServerPublicIP(r.Context()))
}

func (d *Deps) ServerConfigPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	raw, _ := strField(body, "public_ip")
	if raw != "" && !reIPv4OrIPv6.MatchString(raw) {
		writeErr(w, d, errUnprocessable("Format IP tidak valid. Contoh: 43.156.237.97"))
		return
	}
	if err := d.saveServerPublicIP(r.Context(), raw, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "platform.server_config_updated", "platform_server_config", "default")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, d.describeServerPublicIP(r.Context()))
}

// ---------- GET/PUT /v1/platform/short-link-config ----------

const shortLinkDomainKey = "short_link_domain"

func (d *Deps) ShortLinkConfigGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"domain": d.shortLinkBase(r.Context())})
}

func (d *Deps) ShortLinkConfigPut(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	body := decodeJSON(r)
	raw, _ := strField(body, "domain")
	raw = strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://"), "/")
	if raw != "" && !reDomainOnly.MatchString(raw) {
		writeErr(w, d, errUnprocessable("Format domain tidak valid. Contoh: tautan.link"))
		return
	}
	if err := d.exec(r.Context(), `
		INSERT INTO platform_settings (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		shortLinkDomainKey, raw); err != nil {
		writeErr(w, d, err)
		return
	}
	row, _ := d.queryRow(r.Context(),
		"SELECT value, updated_at FROM platform_settings WHERE key = $1 LIMIT 1", shortLinkDomainKey)
	writeJSON(w, http.StatusOK, map[string]any{
		"domain":     raw,
		"updated_at": row.timePtr("updated_at"),
	})
}

// shortLinkBase: domain short link aktif; kosong = fallback domain utama.
func (d *Deps) shortLinkBase(ctx context.Context) any {
	row, _ := d.queryRow(ctx,
		"SELECT value FROM platform_settings WHERE key = $1 LIMIT 1", shortLinkDomainKey)
	if row == nil {
		return nil
	}
	if v, ok := row.str("value"); ok && v != "" {
		return v
	}
	return nil
}

// describeServerPublicIP: bentuk respons parity describeServerPublicIp —
// menyertakan IP efektif + asal nilai (tersimpan / auto-deteksi).
func (d *Deps) describeServerPublicIP(ctx context.Context) map[string]any {
	stored, _ := d.queryRow(ctx,
		"SELECT value FROM platform_settings WHERE key = 'server_public_ip' LIMIT 1")
	var saved any
	if stored != nil {
		if v, ok := stored.str("value"); ok && v != "" {
			saved = v
		}
	}
	effective := saved
	source := "stored"
	if effective == nil {
		detected := detectPublicIP(ctx)
		if detected != "" {
			effective = detected
			source = "auto"
		} else {
			source = "unavailable"
		}
	}
	return map[string]any{
		"public_ip": effective,
		"source":    source,
		"stored":    saved,
	}
}

func (d *Deps) saveServerPublicIP(ctx context.Context, value, userID string) error {
	return d.exec(ctx, `
		INSERT INTO platform_settings (key, value, updated_at) VALUES ('server_public_ip', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		nullIfEmpty(value))
}

// detectPublicIP: deteksi IP publik server (best-effort, timeout singkat).
func detectPublicIP(ctx context.Context) string {
	resp, err := security.Get(ctx, "https://api.ipify.org", map[string]string{"accept": "text/plain"}, 3*time.Second)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	return strings.TrimSpace(string(buf[:n]))
}

// itoa: pembungkus strconv.Itoa (dipakai untuk menyusun placeholder $n).
func itoa(n int) string { return strconv.Itoa(n) }
