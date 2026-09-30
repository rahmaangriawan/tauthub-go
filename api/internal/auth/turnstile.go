package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// Port src/lib/auth/turnstile.ts — config DB platform_turnstile_config
// (cache 60s) + fallback env. Verifikasi via Cloudflare siteverify.

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
const turnstileCacheTTL = 60 * time.Second

type TurnstileConfig struct {
	Enabled      bool
	SiteKey      string
	SealedSecret string
}

type TurnstileError struct{ Message string }

func (e *TurnstileError) Error() string { return e.Message }

type Turnstile struct {
	Pool                  *pgxpool.Pool
	TurnstileSiteKeyEnv   string
	TurnstileSecretKeyEnv string
	CmsEncryptionSecret   string // fallback APP_SECRET (port serverConfig.cmsEncryptionSecret)

	mu   sync.Mutex
	cache *turnstileCache
}

type turnstileCache struct {
	value TurnstileConfig
	at    time.Time
}

func (t *Turnstile) GetConfig(ctx context.Context) TurnstileConfig {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cache != nil && time.Since(t.cache.at) < turnstileCacheTTL {
		return t.cache.value
	}
	cfg := TurnstileConfig{}
	row := t.Pool.QueryRow(ctx,
		"SELECT site_key, sealed_secret_key, enabled FROM platform_turnstile_config WHERE id = 'default' LIMIT 1")
	var siteKey, sealedSecret *string
	var enabled bool
	if err := row.Scan(&siteKey, &sealedSecret, &enabled); err == nil {
		cfg = TurnstileConfig{Enabled: enabled, SiteKey: deref(siteKey), SealedSecret: deref(sealedSecret)}
	} else {
		// tabel belum ada / error — pakai fallback env (parity catch)
		cfg = TurnstileConfig{}
	}
	if cfg.SiteKey == "" && strings.TrimSpace(t.TurnstileSiteKeyEnv) != "" {
		cfg.SiteKey = strings.TrimSpace(t.TurnstileSiteKeyEnv)
	}
	if cfg.SealedSecret == "" && strings.TrimSpace(t.TurnstileSecretKeyEnv) != "" {
		cfg.SealedSecret = "env:" + strings.TrimSpace(t.TurnstileSecretKeyEnv)
	}
	t.cache = &turnstileCache{value: cfg, at: time.Now()}
	return cfg
}

// resolveSecret: env: prefix → plaintext; selain itu unseal (v1/v2) dengan cms secret.
func (t *Turnstile) resolveSecret(cfg TurnstileConfig) string {
	if cfg.SealedSecret == "" {
		return ""
	}
	if strings.HasPrefix(cfg.SealedSecret, "env:") {
		return cfg.SealedSecret[len("env:"):]
	}
	plain, err := crypto.Unseal(cfg.SealedSecret, t.CmsEncryptionSecret)
	if err != nil {
		return ""
	}
	return plain
}

// PublicSiteKey: untuk widget client (parity publicTurnstileSiteKey).
func (t *Turnstile) PublicSiteKey(ctx context.Context) (bool, *string) {
	cfg := t.GetConfig(ctx)
	if !cfg.Enabled || cfg.SiteKey == "" {
		return false, nil
	}
	return true, &cfg.SiteKey
}

// RequireToken: enabled tapi secret kosong → TOLAK (fail-closed); token kosong
// atau verifikasi gagal → TurnstileError (port requireTurnstileToken).
//
// Sebelumnya misconfig ini dilewati (fail-open) sehingga proteksi bot mati
// diam-diam saat secret lupa diisi.
func (t *Turnstile) RequireToken(ctx context.Context, token string, remoteIp string) error {
	cfg := t.GetConfig(ctx)
	if !cfg.Enabled {
		return nil
	}
	secret := t.resolveSecret(cfg)
	if secret == "" {
		slog.Error("[turnstile] enabled tapi secret belum dikonfigurasi — verifikasi ditolak (fail-closed)")
		return &TurnstileError{Message: "Verifikasi Turnstile belum dikonfigurasi. Hubungi admin."}
	}
	value := strings.TrimSpace(token)
	if value == "" {
		return &TurnstileError{Message: "Verifikasi Turnstile diperlukan. Centang kotak 'Saya bukan robot' lalu coba lagi."}
	}
	if !t.verifyToken(ctx, secret, value, remoteIp) {
		return &TurnstileError{Message: "Verifikasi Turnstile gagal. Silakan coba lagi."}
	}
	return nil
}

func (t *Turnstile) verifyToken(ctx context.Context, secret, token, remoteIp string) bool {
	if secret == "" || token == "" {
		return false
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIp != "" {
		form.Set("remoteip", remoteIp)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return false
	}
	var data struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return false
	}
	return data.Success
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
