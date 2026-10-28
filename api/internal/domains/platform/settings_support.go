package platform

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/smtp"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/email"
)

// base64Std: pembantu encoding subjek email.
func base64Std(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// Port lib/email/resolver.ts + transport.ts (bagian verify/classify),
// lib/media/page-speed.ts, dan lib/auth/turnstile.ts (bagian test secret).
//
// Helper di berkas ini diberi awalan `st` supaya tidak bentrok dengan helper
// modul platform lain yang berbagi package.

// ---------- SMTP ----------

// stSMTPRow: baris smtp_settings apa adanya (untuk GET/PUT settings email).
func (d *Deps) stSMTPRow(ctx context.Context) rowMap {
	row, err := d.queryRow(ctx,
		"SELECT id, provider, host, port, secure, username, password_sealed, from_email, from_name, enabled, last_test_at, last_test_ok, last_test_message, updated_at, updated_by FROM smtp_settings WHERE id = 'default' LIMIT 1")
	if err != nil {
		return nil
	}
	return row
}

// stSMTPConfig: config efektif untuk verify/send, parity resolveEmailConfig
// dengan opsi allowDisabled (test-connection tetap boleh saat enabled=false).
type stSMTPConfig struct {
	Host      string
	Port      int
	Secure    bool
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// stResolveSMTP: parity resolveEmailConfig. ErrNotConfigured/ErrDecryptFailed
// memakai sentinel internal/email supaya pesan & status sama.
func (d *Deps) stResolveSMTP(ctx context.Context, allowDisabled bool) (*stSMTPConfig, error) {
	row := d.stSMTPRow(ctx)
	if row == nil {
		return nil, email.ErrNotConfigured
	}
	if !allowDisabled && !row.boolOr("enabled", false) {
		return nil, email.ErrNotConfigured
	}
	host := row.strOr("host", "")
	sealed := row.strOr("password_sealed", "")
	if host == "" || sealed == "" {
		return nil, email.ErrNotConfigured
	}
	plain, err := d.unsealT2(sealed)
	if err != nil {
		return nil, email.ErrDecryptFailed
	}
	port := row.intOr("port", 587)
	if port <= 0 {
		port = 587
	}
	return &stSMTPConfig{
		Host: host, Port: port, Secure: row.boolOr("secure", false),
		Username: row.strOr("username", ""), Password: plain,
		FromEmail: row.strOr("from_email", ""), FromName: row.strOr("from_name", ""),
	}, nil
}

// stUpdateSMTPTest: parity updateSmtpTestResult (pesan dipotong 500 char).
func (d *Deps) stUpdateSMTPTest(ctx context.Context, ok bool, message string) {
	if len(message) > 500 {
		message = message[:500]
	}
	_ = d.exec(ctx,
		"UPDATE smtp_settings SET last_test_at = NOW(), last_test_ok = $1, last_test_message = $2 WHERE id = 'default'",
		ok, message)
}

// stSMTPClassification: parity SmtpErrorCode + SAFE_MESSAGE transport.ts.
type stSMTPClassification struct {
	Code    string
	Status  int
	Message string
}

var stSMTPMessages = map[string]string{
	"SMTP_CONFIG_MISSING":    "SMTP belum dikonfigurasi lengkap. Isi host, username, password, lalu simpan konfigurasi.",
	"SMTP_DECRYPTION_FAILED": "Gagal mendekripsi kredensial SMTP. Simpan ulang SMTP key dan coba lagi.",
	"SMTP_CONNECTION_FAILED": "Gagal terhubung ke SMTP relay. Periksa host/port dan koneksi jaringan server.",
	"SMTP_TLS_FAILED":        "Gagal menegosiasi TLS/STARTTLS dengan SMTP relay. Periksa port 587 + STARTTLS atau coba port 465.",
	// Kirim.id: 535 = login salah, 550 5.7.1 = pengirim bukan domain aktif /
	// saldo koin habis, 452 = rate limit atau antrean penuh, 451 = gangguan.
	"SMTP_AUTH_FAILED":        "Autentikasi SMTP gagal. Periksa username (kir + 6 angka) dan password 32 karakter dari dashboard Kirim.id.",
	"SMTP_SENDER_REJECTED":    "Alamat pengirim ditolak. Pastikan domain alamat From sudah aktif di akun Kirim.id dan saldo koin masih ada.",
	"SMTP_RECIPIENT_REJECTED": "Alamat penerima ditolak oleh provider.",
	"SMTP_SEND_FAILED":        "Gagal mengirim email via SMTP. Cek log server dan coba lagi.",
}

func stClassify(code, status string, rawErr error) stSMTPClassification {
	raw := ""
	if rawErr != nil {
		raw = strings.ToLower(rawErr.Error())
	}
	lower := strings.ToLower(code + " " + status + " " + raw)
	switch {
	case strings.Contains(lower, "eauth") || strings.Contains(lower, "535") ||
		strings.Contains(lower, "authentication failed") || strings.Contains(lower, "invalid login") ||
		strings.Contains(lower, "bad username"):
		return stSMTPClassification{"SMTP_AUTH_FAILED", 502, stSMTPMessages["SMTP_AUTH_FAILED"]}
	case strings.Contains(lower, "starttls") || strings.Contains(lower, "tls") ||
		strings.Contains(lower, "ssl") || strings.Contains(lower, "certificate") || strings.Contains(lower, "handshake"):
		return stSMTPClassification{"SMTP_TLS_FAILED", 502, stSMTPMessages["SMTP_TLS_FAILED"]}
	case strings.Contains(lower, "edns") || strings.Contains(lower, "enotfound") ||
		strings.Contains(lower, "econnrefused") || strings.Contains(lower, "etimedout") ||
		strings.Contains(lower, "econnreset") || strings.Contains(lower, "ehostunreach") ||
		strings.Contains(lower, "getaddrinfo") || strings.Contains(lower, "greeting never received") ||
		strings.Contains(lower, "connection closed") || strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "dial") || strings.Contains(lower, "connect"):
		return stSMTPClassification{"SMTP_CONNECTION_FAILED", 502, stSMTPMessages["SMTP_CONNECTION_FAILED"]}
	case strings.Contains(lower, "recipient") || strings.Contains(lower, "mailbox") || strings.Contains(lower, "user unknown"):
		return stSMTPClassification{"SMTP_RECIPIENT_REJECTED", 400, stSMTPMessages["SMTP_RECIPIENT_REJECTED"]}
	case strings.Contains(lower, "sender") || strings.Contains(lower, "from address") ||
		strings.Contains(lower, "domain") || strings.Contains(lower, "unverified"):
		return stSMTPClassification{"SMTP_SENDER_REJECTED", 400, stSMTPMessages["SMTP_SENDER_REJECTED"]}
	}
	return stSMTPClassification{"SMTP_SEND_FAILED", 502, stSMTPMessages["SMTP_SEND_FAILED"]}
}

// stVerifySMTP: parity verifyTransport — connect + STARTTLS + AUTH
// (net/smtp tidak punya verify(), jadi urutannya eksplisit).
func stVerifySMTP(cfg *stSMTPConfig) stSMTPClassification {
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	dialer := &net.Dialer{Timeout: 12 * time.Second}

	var conn net.Conn
	var err error
	if cfg.Secure {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: cfg.Host})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return stClassify("", "", err)
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return stClassify("", "", err)
	}
	defer client.Close()

	if !cfg.Secure && cfg.Port == 587 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				return stClassify("ESTARTTLS", "", err)
			}
		}
	}
	if cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
				return stClassify("EAUTH", "", err)
			}
		}
	}
	if err := client.Noop(); err != nil {
		return stClassify("", "", err)
	}
	return stSMTPClassification{"SMTP_OK", 0, "SMTP connection successful"}
}

// stSMTPTestTemplate: parity registerTemplate("smtp-test") di app lama.
func stSMTPTestTemplate() (string, string) {
	subject := "Tauthub SMTP Test"
	html := `<!DOCTYPE html><html lang="id"><body style="margin:0;padding:24px;background:#f1f5f9;font-family:sans-serif;color:#0f172a;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:12px;overflow:hidden;">
<tr><td style="padding:24px;"><h1 style="margin:0 0 16px;font-size:20px;">Tauthub SMTP Test</h1><p style="margin:0;line-height:1.55;font-size:14px;">SMTP Tauthub berhasil dikonfigurasi.</p><p style="margin:16px 0 0;color:#475569;font-size:13px;">Waktu: ` + time.Now().UTC().Format("2006-01-02T15:04:05.000Z") + `</p></td></tr>
</table></body></html>`
	return subject, html
}

// ---------- PageSpeed ----------

// stPageSpeedError: parity PageSpeedError(status, message).
type stPageSpeedError struct {
	Status  int
	Message string
}

func (e *stPageSpeedError) Error() string { return e.Message }

func stPageSpeedErr(status int, msg string) *stPageSpeedError {
	return &stPageSpeedError{Status: status, Message: msg}
}

var stUnsafeHostRe = regexp.MustCompile(`^(127\.|10\.|192\.168\.|172\.(1[6-9]|2[0-9]|3[0-1])\.)`)

// stNormalizeSpeedURL: port normalizeSpeedUrl.
func stNormalizeSpeedURL(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", stPageSpeedErr(400, "Masukkan URL website untuk diperiksa.")
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", "", stPageSpeedErr(400, "URL tidak valid. Contoh: https://example.com")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", stPageSpeedErr(400, "URL harus http atau https.")
	}
	hostname := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	if hostname == "" || hostname == "localhost" || hostname == "::1" ||
		stUnsafeHostRe.MatchString(hostname) ||
		strings.HasSuffix(hostname, ".local") || strings.HasSuffix(hostname, ".internal") ||
		strings.Contains(hostname, "..") || !strings.Contains(hostname, ".") {
		return "", "", stPageSpeedErr(400, "Domain tidak dapat diperiksa.")
	}
	pathname := strings.TrimRight(parsed.Path, "/")
	if pathname == "/" {
		pathname = ""
	}
	return "https://" + hostname + pathname + parsed.RawQuery, hostname, nil
}

// stPageSpeedResult: bentuk NormalizedSpeedResult (field yang dipakai route).
type stPageSpeedResult struct {
	URL                string
	Domain             string
	PerformanceScore   any
	AccessibilityScore any
}

// stFetchPageSpeed: port fetchPageSpeed. Urutan parity: key dulu (503 bila
// belum dikonfigurasi), baru panggil API dengan URL apa adanya; normalisasi URL
// hanya untuk field respons (url/normalized_url/domain).
func (d *Deps) stFetchPageSpeed(ctx context.Context, target string) (*stPageSpeedResult, error) {
	key := d.mpPageSpeedKey(ctx)
	if key == "" {
		return nil, stPageSpeedErr(503, "API Pagespeed belum dikonfigurasi. Hubungi admin untuk mengatur API key di Konfigurasi → API Pagespeed.")
	}
	_, domain, normErr := stNormalizeSpeedURL(target)
	if normErr != nil {
		return nil, normErr
	}
	apiURL := "https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=" + url.QueryEscape(target) +
		"&strategy=mobile&category=performance&category=accessibility&category=best-practices&category=seo&key=" + url.QueryEscape(key)
	status, body := mpSafeGet(ctx, apiURL, map[string]string{"accept": "application/json"}, 30*time.Second)
	if status == 0 {
		return nil, stPageSpeedErr(504, "Gagal menghubungi PageSpeed API: koneksi gagal")
	}
	if status < 200 || status >= 300 {
		detail := ""
		var parsed map[string]any
		if json.Unmarshal([]byte(body), &parsed) == nil {
			if e, ok := parsed["error"].(map[string]any); ok {
				if m, ok := e["message"].(string); ok {
					detail = m
				}
			}
		}
		if detail == "" {
			detail = ptTruncate(body, 500)
		}
		switch {
		case status == 429:
			return nil, stPageSpeedErr(429, "PageSpeed API dibatasi (quota). Coba lagi beberapa menit.")
		case status >= 500:
			return nil, stPageSpeedErr(502, "PageSpeed API sedang gangguan. Coba lagi.")
		case strings.Contains(detail, "Lighthouse returned error") || strings.Contains(detail, "DNS"):
			return nil, stPageSpeedErr(400, "Website tidak dapat dianalisis. Pastikan URL dapat diakses secara publik.")
		}
		if detail == "" {
			detail = "Gagal menganalisis website."
		}
		return nil, stPageSpeedErr(502, detail)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, stPageSpeedErr(502, "Gagal menganalisis website.")
	}
	light, _ := parsed["lighthouseResult"].(map[string]any)
	categories, _ := light["categories"].(map[string]any)
	if len(categories) == 0 {
		return nil, stPageSpeedErr(502, "PageSpeed tidak mengembalikan hasil Lighthouse.")
	}
	round := func(key string) any {
		entry, _ := categories[key].(map[string]any)
		score, ok := entry["score"].(float64)
		if !ok {
			return nil
		}
		return int(math.Round(score * 100))
	}
	return &stPageSpeedResult{
		URL: target, Domain: domain,
		PerformanceScore: round("performance"), AccessibilityScore: round("accessibility"),
	}, nil
}

// ---------- Turnstile ----------

var stTurnstileTestURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// stTurnstileSecret: secret efektif dari DB ter-seal atau env (prefix env:),
// parity resolveSecret di lib/auth/turnstile.ts.
func (d *Deps) stTurnstileSecret(ctx context.Context) string {
	row, _ := d.queryRow(ctx,
		"SELECT site_key, sealed_secret_key, enabled FROM platform_turnstile_config WHERE id = 'default' LIMIT 1")
	if row != nil {
		sealed := row.strOr("sealed_secret_key", "")
		if strings.HasPrefix(sealed, "env:") {
			return strings.TrimSpace(sealed[len("env:"):])
		}
		if sealed != "" {
			if plain, err := d.unsealT2(sealed); err == nil {
				return strings.TrimSpace(plain)
			}
		}
	}
	return strings.TrimSpace(stEnv("TURNSTILE_SECRET_KEY"))
}

// stTurnstileSecretConfigured: parity `!sealed && !process.env.TURNSTILE_SECRET_KEY?.trim()`.
func (d *Deps) stTurnstileSecretConfigured(ctx context.Context) bool {
	row, _ := d.queryRow(ctx,
		"SELECT sealed_secret_key FROM platform_turnstile_config WHERE id = 'default' LIMIT 1")
	if row != nil && row.strOr("sealed_secret_key", "") != "" {
		return true
	}
	return strings.TrimSpace(stEnv("TURNSTILE_SECRET_KEY")) != ""
}

// stTestTurnstileSecret: port testTurnstileSecret.
func (d *Deps) stTestTurnstileSecret(ctx context.Context) (bool, string) {
	secret := d.stTurnstileSecret(ctx)
	if secret == "" {
		return false, "Secret Key belum dikonfigurasi."
	}
	form := "secret=" + url.QueryEscape(secret) + "&response=dummy-test-token"
	status, body := mpSafePostJSON(ctx, stTurnstileTestURL, form,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 10*time.Second)
	if status == 0 {
		return false, "Gagal menghubungi Cloudflare Turnstile. Periksa jaringan server."
	}
	var parsed struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return false, "Gagal menghubungi Cloudflare Turnstile. Periksa jaringan server."
	}
	for _, code := range parsed.ErrorCodes {
		if code == "invalid-input-secret" {
			return false, "Secret Key tidak valid. Periksa kembali di dashboard Cloudflare Turnstile."
		}
	}
	for _, code := range parsed.ErrorCodes {
		if code == "invalid-input-response" {
			return true, "Secret Key valid (token dummy ditolak — normal)."
		}
	}
	if parsed.Success {
		return true, "Koneksi Turnstile berhasil."
	}
	return false, "Tidak dapat memverifikasi — cek jaringan/secret."
}

// stEnv: pembacaan env dengan trim (dipisah agar mudah diuji).
var stEnv = func(key string) string { return strings.TrimSpace(getenv(key)) }

// stValidateEmail: parity z.string().email() — pesan "Invalid email address".
var stEmailRe = regexp.MustCompile(`^[A-Za-z0-9_'+\-](?:[A-Za-z0-9_'+\-.]*[A-Za-z0-9_+\-])?@(?:[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.)+[A-Za-z]{2,}$`)

func stInvalidEmailErr() error { return errUnprocessable("Invalid email address") }

// stSMTPSendNow: port transport.sendMail untuk test-send admin — kirim
// langsung tanpa antrean (parity sendTransactionalEmailNow).
func stSMTPSendNow(cfg *stSMTPConfig, to, subject, html string) (string, error) {
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	from := cfg.FromEmail
	if from == "" {
		from = "no-reply@tauthub.com"
	}
	fromName := cfg.FromName
	if fromName == "" {
		fromName = "Tauthub"
	}
	headers := map[string]string{
		"From":         fmt.Sprintf("%q <%s>", fromName, from),
		"To":           to,
		"Subject":      stMimeHeaderEncode(subject),
		"MIME-Version": "1.0",
		"Content-Type": `text/html; charset="utf-8"`,
	}
	var msg strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
	}
	msg.WriteString("\r\n" + html)

	dialer := &net.Dialer{Timeout: 12 * time.Second}
	var conn net.Conn
	var err error
	if cfg.Secure {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: cfg.Host})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return "", fmt.Errorf("smtp connect: %w", err)
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("smtp connect: %w", err)
	}
	defer client.Close()
	if !cfg.Secure && cfg.Port == 587 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				return "", fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	if cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
				return "", fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	if err := client.Mail(from); err != nil {
		return "", fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return "", fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		return "", fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return "", fmt.Errorf("smtp quit: %w", err)
	}
	return fmt.Sprintf("go-smtp-%d", time.Now().UnixNano()), nil
}

// stMimeHeaderEncode: subjek non-ASCII di-encode RFC 2047.
func stMimeHeaderEncode(s string) string {
	needs := false
	for _, r := range s {
		if r > 127 {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	return "=?UTF-8?B?" + base64Std(s) + "?="
}

// stSMTPErrCode: parity err.code untuk EmailNotConfiguredError/SmtpDecryptionError.
func stSMTPErrCode(err error) string {
	if errors.Is(err, email.ErrDecryptFailed) {
		return "SMTP_DECRYPTION_FAILED"
	}
	return "SMTP_CONFIG_MISSING"
}

// errorsIs: pembantu pembanding error (dipakai handler settings).
func errorsIs(err, target error) bool { return errors.Is(err, target) }
