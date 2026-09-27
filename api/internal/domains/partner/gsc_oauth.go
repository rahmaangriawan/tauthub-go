package partner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// gsc_oauth.go: port lib/media/gsc.ts (exchangeGscCode/listGscSites/refreshGscToken
// /sealGscToken) + app/api/gsc/callback/route.ts — melengkapi GSCConnect yang
// sudah ada agar alur OAuth GSC mitra benar-benar selesai.

const (
	gscTokenURL     = "https://oauth2.googleapis.com/token"
	gscSitesURL     = "https://www.googleapis.com/webmasters/v3/sites"
	gscDefaultScope = "https://www.googleapis.com/auth/webmasters.readonly"
	gscHTTPTimeout  = 10 * time.Second
)

// gscTokenResult: hasil exchange/refresh token GSC.
type gscTokenResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	Scope        string
}

// exchangeGscCode: authorization code → token (parity exchangeGscCode).
func exchangeGscCode(code, clientID, clientSecret, redirectURI, codeVerifier string) (gscTokenResult, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	data, err := bloggerPostForm(gscTokenURL, form)
	if err != nil {
		return gscTokenResult{}, err
	}
	accessToken, _ := data["access_token"].(string)
	if accessToken == "" {
		return gscTokenResult{}, fmt.Errorf("Token exchange gagal: access_token kosong")
	}
	out := gscTokenResult{AccessToken: accessToken, ExpiresIn: 3600}
	if rt, ok := data["refresh_token"].(string); ok {
		out.RefreshToken = rt
	}
	if v, ok := data["expires_in"].(float64); ok && v > 0 {
		out.ExpiresIn = int(v)
	}
	if sc, ok := data["scope"].(string); ok {
		out.Scope = sc
	}
	return out, nil
}

// refreshGscToken: refresh access token GSC (parity refreshGscToken).
func refreshGscToken(refreshToken, clientID, clientSecret string) (string, int, error) {
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "refresh_token")
	data, err := bloggerPostForm(gscTokenURL, form)
	if err != nil {
		return "", 0, err
	}
	accessToken, _ := data["access_token"].(string)
	if accessToken == "" {
		return "", 0, fmt.Errorf("Refresh gagal: access_token kosong")
	}
	expiresIn := 3600
	if v, ok := data["expires_in"].(float64); ok && v > 0 {
		expiresIn = int(v)
	}
	return accessToken, expiresIn, nil
}

type gscSite struct {
	SiteURL         string
	PermissionLevel string
}

// listGscSites: daftar properti Search Console akun terhubung.
func listGscSites(accessToken string) ([]gscSite, error) {
	raw, err := bloggerGet(gscSitesURL, accessToken)
	if err != nil {
		return nil, err
	}
	var data struct {
		SiteEntry []struct {
			SiteURL         string `json:"siteUrl"`
			PermissionLevel string `json:"permissionLevel"`
		} `json:"siteEntry"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &data) != nil {
		return nil, fmt.Errorf("respons GSC tidak valid")
	}
	if data.Error != nil && data.Error.Message != "" {
		return nil, fmt.Errorf("%s", data.Error.Message)
	}
	out := make([]gscSite, 0, len(data.SiteEntry))
	for _, e := range data.SiteEntry {
		level := e.PermissionLevel
		if level == "" {
			level = "siteOwner"
		}
		out = append(out, gscSite{SiteURL: e.SiteURL, PermissionLevel: level})
	}
	return out, nil
}

// sealGscToken: refresh token GSC → t2 (cmsEncryptionSecret/APP_SECRET).
func (d *Deps) sealGscToken(refreshToken string) (string, error) {
	return d.sealT2(refreshToken)
}

// GSCCallback: GET /api/gsc/callback — OAuth GSC mitra. Parity
// app/api/gsc/callback/route.ts. PUBLIK (dipanggil Google, tanpa sesi).
func (d *Deps) GSCCallback(w http.ResponseWriter, r *http.Request) {
	appBase := d.appBase(r)
	q := r.URL.Query()

	redirectErr := func(msg string) {
		http.Redirect(w, r, appBase+"/mitra?gsc_error="+url.QueryEscape(msg), http.StatusFound)
	}

	if errParam := q.Get("error"); errParam != "" {
		msg := "OAuth dibatalkan: " + q.Get("error_description")
		if errParam == "access_denied" {
			msg = "Akses ditolak — Anda membatalkan consent Google."
		}
		redirectErr(msg)
		return
	}
	code := q.Get("code")
	state := q.Get("state")
	if code == "" || state == "" {
		redirectErr("Callback tidak valid (code/state hilang).")
		return
	}

	oauth, err := d.queryRow(r.Context(),
		"SELECT partner_id, code_verifier FROM gsc_oauth_states WHERE state = $1 LIMIT 1", state)
	if err != nil || oauth == nil {
		redirectErr("State OAuth tidak valid atau kedaluwarsa. Silakan coba hubungkan ulang.")
		return
	}
	cfg, cfgErr := d.loadGSCPlatformConfig(r)
	if cfgErr != nil || cfg.ClientID == "" || cfg.ClientSecret == "" {
		redirectErr("Konfigurasi Google OAuth belum diatur.")
		return
	}
	partnerID := oauth.strOr("partner_id", "")
	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = appBase + "/api/gsc/callback"
	}

	token, exErr := exchangeGscCode(code, cfg.ClientID, cfg.ClientSecret, redirectURI, oauth.strOr("code_verifier", ""))
	if exErr != nil {
		_ = d.exec(r.Context(), "DELETE FROM gsc_oauth_states WHERE state = $1", state)
		redirectErr(exErr.Error())
		return
	}

	// Email akun Google (best-effort) untuk pencocokan multi-akun.
	userEmail := ""
	if raw, uErr := bloggerGet(bloggerUserInfo, token.AccessToken); uErr == nil {
		var uj struct {
			Email string `json:"email"`
		}
		if json.Unmarshal(raw, &uj) == nil {
			userEmail = uj.Email
		}
	}

	scope := token.Scope
	if scope == "" {
		scope = gscDefaultScope
	}

	// Multi-akun: cocokkan per (partner_id, email), jangan menimpa akun lain.
	var sealed any
	if token.RefreshToken != "" {
		if s, sErr := d.sealGscToken(token.RefreshToken); sErr == nil {
			sealed = s
		}
	}

	existing, _ := d.queryRow(r.Context(), `
		SELECT id FROM gsc_connections
		WHERE partner_id = $1 AND LOWER(COALESCE(email, '')) = LOWER($2) LIMIT 1`, partnerID, userEmail)

	connectionID := ""
	if existing != nil && userEmail != "" {
		connectionID = existing.strOr("id", "")
		if err := d.exec(r.Context(),
			"UPDATE gsc_connections SET sealed_refresh_token = COALESCE($1, sealed_refresh_token), scope = $2, status = 'active', email = COALESCE($3, email), updated_at = NOW() WHERE id = $4",
			sealed, scope, nullIfEmptyStr(userEmail), connectionID); err != nil {
			d.writeErr(w, err)
			return
		}
	} else {
		connectionID = uuid.NewString()
		if err := d.exec(r.Context(),
			"INSERT INTO gsc_connections (id, partner_id, email, sealed_refresh_token, scope, status) VALUES ($1, $2, $3, $4, $5, 'active')",
			connectionID, partnerID, nullIfEmptyStr(userEmail), sealed, scope); err != nil {
			d.writeErr(w, err)
			return
		}
	}

	// Sites/properties best-effort — user tetap bisa memilih manual bila gagal.
	if sites, sErr := listGscSites(token.AccessToken); sErr == nil {
		for _, site := range sites {
			_ = d.exec(r.Context(), `
				INSERT INTO gsc_properties (id, connection_id, site_url, permission_level)
				VALUES (gen_random_uuid(), $1, $2, $3)
				ON CONFLICT (connection_id, site_url) DO NOTHING`,
				connectionID, site.SiteURL, site.PermissionLevel)
		}
		if len(sites) == 1 {
			if prop, pErr := d.queryRow(r.Context(),
				"SELECT id FROM gsc_properties WHERE connection_id = $1 LIMIT 1", connectionID); pErr == nil && prop != nil {
				_ = d.exec(r.Context(), "UPDATE gsc_connections SET selected_property_id = $1 WHERE id = $2",
					prop.strOr("id", ""), connectionID)
			}
		}
	}

	_ = d.exec(r.Context(), "DELETE FROM gsc_oauth_states WHERE state = $1", state)

	publicID := partnerID
	if prow, _ := d.queryRow(r.Context(), "SELECT public_id, id FROM partners WHERE id = $1 LIMIT 1", partnerID); prow != nil {
		publicID = prow.strOr("public_id", partnerID)
	}
	http.Redirect(w, r, appBase+"/mitra/"+publicID+"/kalkulator-media?gsc_connected=1", http.StatusFound)
}

// appBase: base aplikasi publik (config lalu origin request).
func (d *Deps) appBase(r *http.Request) string {
	base := strings.TrimRight(strings.TrimSpace(d.Cfg.PublicAppURL), "/")
	if base == "" {
		base = strings.TrimRight(appBaseFromRequest(r), "/")
	}
	return base
}

// nullIfEmptyStr: "" → nil agar kolom nullable tetap NULL.
func nullIfEmptyStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ = crypto.Unseal
