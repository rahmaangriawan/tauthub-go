package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// Port lib/auth/google.ts — Google Sign-In: config DB + env fallback, state
// cookie CSRF, ticket HMAC httpOnly, exchange code, userinfo.

const (
	GoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL    = "https://oauth2.googleapis.com/token"
	GoogleUserinfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

	stateCookieName  = "goauth_state"
	ticketCookieName = "goauth_ticket"
	stateTTL         = 10 * time.Minute
	googleTicketTTL  = 5 * time.Minute
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Source       string // database | environment | none
	ConfigError  string // table_missing | record_empty | secret_decrypt_failed | ""
}

type Google struct {
	Pool            *pgxpool.Pool
	AppSecret       string
	CmsSecret       string
	EnvClientID     string
	EnvClientSecret string
	EnvRedirectURI  string
	IsProduction    bool

	mu  sync.Mutex
	cfg *googleCfgCache
}

type googleCfgCache struct {
	value     GoogleOAuthConfig
	expiresAt time.Time
}

// LoadConfig: DB platform_oauth_configs dulu, env fallback; cache 30s.
func (g *Google) LoadConfig(ctx context.Context) GoogleOAuthConfig {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cfg != nil && g.cfg.expiresAt.After(time.Now()) {
		return g.cfg.value
	}
	cfg := GoogleOAuthConfig{Source: "none", ConfigError: "table_missing"}
	var clientID, sealedSecret, redirectURI *string
	err := g.Pool.QueryRow(ctx,
		"SELECT client_id, sealed_client_secret, redirect_uri FROM platform_oauth_configs WHERE id = 'google' LIMIT 1",
	).Scan(&clientID, &sealedSecret, &redirectURI)
	if err == nil && clientID != nil && *clientID != "" {
		secret := ""
		decryptFailed := false
		if sealedSecret != nil && *sealedSecret != "" {
			// parity: unsealSecret mencoba beberapa kandidat secret.
			for _, candidate := range []string{g.CmsSecret, g.AppSecret} {
				if candidate == "" {
					continue
				}
				if plain, err := crypto.Unseal(*sealedSecret, candidate); err == nil && plain != "" {
					secret = plain
					break
				}
			}
			if secret == "" {
				decryptFailed = true
			}
		}
		cfg = GoogleOAuthConfig{ClientID: *clientID, ClientSecret: secret, RedirectURI: derefStr2(redirectURI), Source: "database"}
		if decryptFailed {
			cfg.ConfigError = "secret_decrypt_failed"
		}
	} else if err == nil {
		cfg.ConfigError = "record_empty"
	}

	if cfg.Source != "database" {
		cfg = GoogleOAuthConfig{
			ClientID: g.EnvClientID, ClientSecret: g.EnvClientSecret, RedirectURI: g.EnvRedirectURI,
			Source: "environment", ConfigError: cfg.ConfigError,
		}
		if cfg.ClientID == "" {
			cfg.Source = "none"
		} else {
			cfg.ConfigError = ""
		}
	}

	g.cfg = &googleCfgCache{value: cfg, expiresAt: time.Now().Add(30 * time.Second)}
	return cfg
}

// UnsealGoogle sudah tidak perlu — crypto.Unseal dipakai langsung.

func (g *Google) Configured(ctx context.Context) bool {
	cfg := g.LoadConfig(ctx)
	return cfg.ClientID != "" && cfg.ClientSecret != "" && cfg.RedirectURI != ""
}

// ---------- state cookie (CSRF) ----------

type OAuthStateInfo struct {
	Mode     string
	RoleHint string
	ReturnTo string
}

// CreateOAuthState: random state + cookie base64url payload (parity).
func (g *Google) CreateOAuthState(w http.ResponseWriter, mode, roleHint, returnTo, origin string) string {
	stateBytes := make([]byte, 24)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)
	payload, _ := json.Marshal(map[string]any{
		"mode":     mode,
		"roleHint": roleHint,
		"returnTo": SafeReturnTo(returnTo, origin),
		"state":    state,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    base64.RawURLEncoding.EncodeToString(payload),
		Path:     "/",
		MaxAge:   int(stateTTL.Seconds()),
		HttpOnly: true,
		Secure:   g.IsProduction,
		SameSite: http.SameSiteLaxMode,
	})
	return state
}

// ConsumeOAuthState: baca + hapus cookie, cocokkan state.
func (g *Google) ConsumeOAuthState(r *http.Request, w http.ResponseWriter, expectedState string) *OAuthStateInfo {
	c, err := r.Cookie(stateCookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: g.IsProduction, SameSite: http.SameSiteLaxMode})
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil
	}
	var parsed struct {
		Mode     any `json:"mode"`
		RoleHint any `json:"roleHint"`
		ReturnTo any `json:"returnTo"`
		State    any `json:"state"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	state, _ := parsed.State.(string)
	mode, _ := parsed.Mode.(string)
	if state != expectedState || mode == "" {
		return nil
	}
	roleHint, _ := parsed.RoleHint.(string)
	returnTo, _ := parsed.ReturnTo.(string)
	return &OAuthStateInfo{Mode: mode, RoleHint: roleHint, ReturnTo: returnTo}
}

// ---------- ticket cookie (HMAC, no Google tokens persisted) ----------

type GoogleTicket struct {
	Kind string // register | link | session
	Data map[string]string
}

func (g *Google) signTicketPayload(payload string) string {
	mac := hmac.New(sha256.New, []byte(g.AppSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (g *Google) SetTicket(w http.ResponseWriter, kind string, data map[string]string) {
	body := map[string]any{"kind": kind, "exp": time.Now().Add(googleTicketTTL).UnixMilli()}
	for k, v := range data {
		body[k] = v
	}
	raw, _ := json.Marshal(body)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	http.SetCookie(w, &http.Cookie{
		Name:     ticketCookieName,
		Value:    encoded + "." + g.signTicketPayload(encoded),
		Path:     "/",
		MaxAge:   int(googleTicketTTL.Seconds()),
		HttpOnly: true,
		Secure:   g.IsProduction,
		SameSite: http.SameSiteLaxMode,
	})
}

func (g *Google) GetTicket(r *http.Request) *GoogleTicket {
	c, err := r.Cookie(ticketCookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	encoded, sig, ok := strings.Cut(c.Value, ".")
	if !ok || encoded == "" || sig == "" || sig != g.signTicketPayload(encoded) {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	exp, _ := body["exp"].(float64)
	if exp == 0 || int64(exp) < time.Now().UnixMilli() {
		return nil
	}
	kind, _ := body["kind"].(string)
	if kind != "register" && kind != "link" && kind != "session" {
		return nil
	}
	data := map[string]string{}
	for k, v := range body {
		if k == "kind" || k == "exp" {
			continue
		}
		if s, ok := v.(string); ok {
			data[k] = s
		}
	}
	return &GoogleTicket{Kind: kind, Data: data}
}

func (g *Google) ClearTicket(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: ticketCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: g.IsProduction, SameSite: http.SameSiteLaxMode})
}

// ---------- exchange + userinfo ----------

type GoogleIDClaims struct {
	Sub          string
	Email        string
	HD           string
	EmailVerified bool
}

type GoogleExchange struct {
	AccessToken string
	IDClaims    GoogleIDClaims
}

// ExchangeGoogleCode: code → access token (+ klaim id_token tanpa verifikasi,
// parity: token langsung dari Google via TLS). redirectURI harus sama persis
// dengan yang dipakai saat membangun URL consent (Google menolak bila beda).
func (g *Google) ExchangeGoogleCode(ctx context.Context, code, redirectURI string) *GoogleExchange {
	cfg := g.LoadConfig(ctx)
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURI == "" {
		return nil
	}
	if redirectURI == "" {
		redirectURI = cfg.RedirectURI
	}
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, GoogleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil || res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil
	}
	defer res.Body.Close()
	var data struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil || data.AccessToken == "" {
		return nil
	}
	return &GoogleExchange{AccessToken: data.AccessToken, IDClaims: parseGoogleIDToken(data.IDToken)}
}

func parseGoogleIDToken(idToken string) GoogleIDClaims {
	var claims GoogleIDClaims
	if idToken == "" {
		return claims
	}
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 || parts[1] == "" {
		return claims
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return claims
	}
	claims.Sub, _ = m["sub"].(string)
	claims.Email, _ = m["email"].(string)
	if hd, ok := m["hd"].(string); ok {
		claims.HD = hd
	}
	claims.EmailVerified = m["email_verified"] == true
	return claims
}

type GoogleUserinfo struct {
	Sub          string
	Email        string
	Name         string
	Picture      string
	EmailVerified bool
}

// FetchUserinfo: identitas Google; sub adalah identitas provider (bukan email).
func (g *Google) FetchUserinfo(ctx context.Context, accessToken string) *GoogleUserinfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GoogleUserinfoURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil || res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil
	}
	defer res.Body.Close()
	var data struct {
		Sub           any `json:"sub"`
		Email         any `json:"email"`
		Name          any `json:"name"`
		Picture       any `json:"picture"`
		EmailVerified any `json:"email_verified"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil
	}
	sub, _ := data.Sub.(string)
	if sub == "" {
		return nil
	}
	email, _ := data.Email.(string)
	name, _ := data.Name.(string)
	picture, _ := data.Picture.(string)
	return &GoogleUserinfo{
		Sub: sub, Email: strings.ToLower(email), Name: name, Picture: picture,
		EmailVerified: data.EmailVerified == true,
	}
}

// ---------- helpers DB ----------

func (g *Google) FindOAuthAccount(ctx context.Context, provider, providerAccountID string) (userID string, ok bool) {
	err := g.Pool.QueryRow(ctx,
		"SELECT user_id FROM user_oauth_accounts WHERE provider = $1 AND provider_account_id = $2 LIMIT 1",
		provider, providerAccountID).Scan(&userID)
	return userID, err == nil
}

type GoogleUserRow struct {
	ID          string
	Email       string
	DisplayName string
	Role        string
	IsActive    bool
	ApprovedAt  *time.Time
}

func (g *Google) FindUserByEmail(ctx context.Context, email string) *GoogleUserRow {
	var row GoogleUserRow
	err := g.Pool.QueryRow(ctx,
		"SELECT id, email, display_name, role, is_active, approved_at FROM users WHERE email = $1 LIMIT 1",
		strings.ToLower(email)).Scan(&row.ID, &row.Email, &row.DisplayName, &row.Role, &row.IsActive, &row.ApprovedAt)
	if err != nil {
		return nil
	}
	return &row
}

// LinkOAuthAccount: unique index menjamin satu provider account satu user
// (TEST 9). Duplikat → error dengan pesan parity.
func (g *Google) LinkOAuthAccount(ctx context.Context, exec interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}, params struct {
	UserID, Provider, ProviderAccountID, Email, DisplayName, AvatarUrl string
}) error {
	_, err := exec.Exec(ctx,
		"INSERT INTO user_oauth_accounts (id, user_id, provider, provider_account_id, email, display_name, avatar_url) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		uuid.NewString(), params.UserID, params.Provider, params.ProviderAccountID,
		nilIfEmptyStr(params.Email), nilIfEmptyStr(params.DisplayName), nilIfEmptyStr(params.AvatarUrl))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "uq_oauth_provider_account") {
			return ErrDuplicateOAuth
		}
		return err
	}
	return nil
}

var ErrDuplicateOAuth = fmtError("Akun Google ini sudah terhubung ke akun lain.")

func fmtError(msg string) error { return &stringError{msg} }

type stringError struct{ msg string }

func (e *stringError) Error() string { return e.msg }

func nilIfEmptyStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefStr2(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
