// Package email: port subsistem email transaksional dari src/lib/email/*.
// Cakupan fase 1+: sender (enqueue + job send_email), process inline, queue
// state machine, resolver smtp_settings, template password-reset/changed,
// transport SMTP. Routing/magic-link fallback menyusul saat domain terkait
// di-port (parity kontrak API tetap terjaga).
package email

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// ---------- resolver (port email/resolver.ts) ----------

type Config struct {
	Enabled   bool
	Host      string
	Port      int
	Secure    bool
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

var ErrNotConfigured = fmt.Errorf("SMTP belum dikonfigurasi lengkap. Isi host, username, password, lalu simpan konfigurasi.")

var ErrDecryptFailed = fmt.Errorf("Gagal mendekripsi kredensial SMTP. Simpan ulang SMTP key dan coba lagi.")

type emailCfgCache struct {
	value *Config
	at    time.Time
}

var (
	cfgMu    sync.Mutex
	cfgCache *emailCfgCache
)

// InvalidateConfigCache: buang cache config (dipanggil setelah admin menyimpan
// smtp_settings) supaya email berikutnya langsung memakai host/kredensial baru
// alih-alih menunggu TTL 60 detik.
func InvalidateConfigCache() {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfgCache = nil
}

// ResolveConfig: smtp_settings id='default' (cache 60s). Tanpa row → ErrNotConfigured.
func ResolveConfig(ctx context.Context, pool *pgxpool.Pool, cmsSecret string) (*Config, error) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if cfgCache != nil && time.Since(cfgCache.at) < 60*time.Second {
		if cfgCache.value == nil {
			return nil, ErrNotConfigured
		}
		return cfgCache.value, nil
	}
	var host, username, passwordSealed, fromEmail, fromName *string
	var port *int
	var secure, enabled *bool
	err := pool.QueryRow(ctx,
		"SELECT host, port, secure, username, password_sealed, from_email, from_name, enabled FROM smtp_settings WHERE id = 'default' LIMIT 1",
	).Scan(&host, &port, &secure, &username, &passwordSealed, &fromEmail, &fromName, &enabled)
	if err != nil || enabled == nil || !*enabled || host == nil || *host == "" || passwordSealed == nil || *passwordSealed == "" {
		cfgCache = &emailCfgCache{value: nil, at: time.Now()}
		return nil, ErrNotConfigured
	}
	password, decErr := crypto.Unseal(*passwordSealed, cmsSecret)
	if decErr != nil {
		cfgCache = &emailCfgCache{value: nil, at: time.Now()}
		return nil, ErrDecryptFailed
	}
	p := 587
	if port != nil && *port > 0 {
		p = *port
	}
	cfg := &Config{
		Enabled: true, Host: *host, Port: p,
		Secure:   secure != nil && *secure,
		Username: derefS(username), Password: password,
		FromEmail: derefS(fromEmail), FromName: derefS(fromName),
	}
	cfgCache = &emailCfgCache{value: cfg, at: time.Now()}
	return cfg, nil
}

func derefS(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---------- queue (port email/queue.ts) ----------

const maxUserHourly = 20

var (
	rateMu    sync.Mutex
	hourlyLRU = map[string]struct {
		count int
		at    time.Time
	}{}
)

func trackRate(key string, max int) bool {
	now := time.Now()
	rateMu.Lock()
	defer rateMu.Unlock()
	hit, ok := hourlyLRU[key]
	if !ok || now.Sub(hit.at) > time.Hour {
		hourlyLRU[key] = struct {
			count int
			at    time.Time
		}{1, now}
		return true
	}
	if hit.count >= max {
		return false
	}
	hit.count++
	return true
}

type EnqueueOptions struct {
	Template       string
	RecipientEmail string
	UserID         *string
	ReferenceID    *string
	Event          string
	Subject        string
	HTML           string
	DedupeKey      *string
}

// EnqueueEmail: rate limit per user + dedupe (password-reset 10 menit, lainnya
// 1 jam — port INTERVAL window termasuk interpolasinya, lihat R11).
func EnqueueEmail(ctx context.Context, pool *pgxpool.Pool, opts EnqueueOptions) (string, string, error) {
	if opts.UserID != nil && !trackRate("u:"+*opts.UserID, maxUserHourly) {
		id := uuid.NewString()
		_, err := pool.Exec(ctx,
			"INSERT INTO email_deliveries (id, template, recipient_email, user_id, reference_id, event, status, max_attempts, error_message) VALUES ($1, $2, $3, $4, $5, $6, 'rate_limited', 1, $7)",
			id, opts.Template, opts.RecipientEmail, opts.UserID, opts.ReferenceID, opts.Event, "User rate limit exceeded")
		return id, "rate_limited", err
	}

	if opts.DedupeKey != nil && opts.UserID != nil {
		window := "1 hour"
		if opts.Template == "password-reset" {
			window = "10 minutes"
		}
		// Parity R11: window di-interpolasi string (SQL lama begitu) — nilai
		// berasal dari konstanta internal, bukan input user.
		var existingID, existingStatus string
		err := pool.QueryRow(ctx,
			fmt.Sprintf(`SELECT id, status FROM email_deliveries WHERE template = $1 AND user_id = $2 AND dedupe_key = $3 AND created_at > NOW() - INTERVAL '%s' LIMIT 1`, window),
			opts.Template, opts.UserID, opts.DedupeKey).Scan(&existingID, &existingStatus)
		if err == nil {
			return existingID, existingStatus, nil
		}
	}

	id := uuid.NewString()
	_, err := pool.Exec(ctx,
		"INSERT INTO email_deliveries (id, template, recipient_email, user_id, reference_id, event, subject, html, dedupe_key, status, max_attempts, next_attempt_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'queued', 3, NOW())",
		id, opts.Template, opts.RecipientEmail, opts.UserID, opts.ReferenceID, opts.Event, opts.Subject, opts.HTML, opts.DedupeKey)
	if err != nil {
		return "", "", err
	}
	return id, "queued", nil
}

// MarkStalePasswordResetSkipped: link kedaluwarsa sebelum terkirim → skipped.
func MarkStalePasswordResetSkipped(ctx context.Context, pool *pgxpool.Pool, id string) bool {
	ct, err := pool.Exec(ctx, `
		UPDATE email_deliveries
		SET status = 'skipped', error_code = 'RESET_LINK_EXPIRED', error_message = 'Password reset email expired before delivery'
		WHERE id = $1 AND template = 'password-reset' AND status IN ('queued','failed') AND created_at <= NOW() - INTERVAL '10 minutes'`, id)
	return err == nil && ct.RowsAffected() == 1
}

// MarkSending: klaim atomik queued/failed + attempts+1.
func MarkSending(ctx context.Context, pool *pgxpool.Pool, id string) bool {
	ct, err := pool.Exec(ctx, `
		UPDATE email_deliveries SET status = 'sending', attempts = attempts + 1
		WHERE id = $1 AND status IN ('queued','failed') AND next_attempt_at <= NOW()
		  AND (template <> 'password-reset' OR created_at > NOW() - INTERVAL '10 minutes')`, id)
	return err == nil && ct.RowsAffected() == 1
}

func MarkSent(ctx context.Context, pool *pgxpool.Pool, id, providerMessageID string) {
	_, _ = pool.Exec(ctx,
		"UPDATE email_deliveries SET status = 'sent', sent_at = NOW(), provider_message_id = $1 WHERE id = $2",
		providerMessageID, id)
}

func MarkFailed(ctx context.Context, pool *pgxpool.Pool, id string, code, message string) {
	_, _ = pool.Exec(ctx, `
		UPDATE email_deliveries SET status = 'failed', error_code = $1, error_message = $2,
		  next_attempt_at = NOW() + (INTERVAL '1 minute' * POWER(2, attempts))
		WHERE id = $3`,
		code, truncate(message, 500), id)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------- sender (port email/sender.ts) ----------

type SendOptions struct {
	Template        string
	To              string
	RecipientUserID *string
	ReferenceID     *string
	Event           string
	DedupeKey       *string
	Data            map[string]string
	// Audience (client/partner) opsional. Bila kosong, ditentukan dari role
	// penerima. Dipakai untuk memilih template tersimpan per audiens.
	Audience string
}

// SendTransactionalEmail: render template → enqueue → job send_email.
// Return (deliveryID, status).
func SendTransactionalEmail(ctx context.Context, pool *pgxpool.Pool, opts SendOptions) (string, string, error) {
	style := LoadStyle(ctx, pool)
	audience := opts.Audience
	if audience == "" && opts.RecipientUserID != nil && *opts.RecipientUserID != "" {
		var role, name string
		if qErr := pool.QueryRow(ctx, "SELECT role, COALESCE(display_name, '') FROM users WHERE id = $1 LIMIT 1", *opts.RecipientUserID).Scan(&role, &name); qErr == nil {
			audience = AudienceForRole(role)
			if opts.Data == nil {
				opts.Data = map[string]string{}
			}
			if strings.TrimSpace(opts.Data["nama"]) == "" {
				opts.Data["nama"] = name
			}
		}
	}
	var rendered Rendered
	var err error
	usedStored := false
	if audience != "" && strings.TrimSpace(opts.Event) != "" {
		if subject, htmlBody, ok := RenderStored(ctx, pool, style, audience, opts.Event, opts.Data); ok {
			rendered = Rendered{Subject: subject, HTML: htmlBody}
			usedStored = true
		}
	}
	if !usedStored {
		rendered, err = RenderTemplateStyled(style, opts.Template, opts.Data)
		if err != nil {
			return "", "failed", err
		}
	}
	id, status, err := EnqueueEmail(ctx, pool, EnqueueOptions{
		Template: opts.Template, RecipientEmail: opts.To,
		UserID: opts.RecipientUserID, ReferenceID: opts.ReferenceID,
		Event: opts.Event, Subject: rendered.Subject, HTML: rendered.HTML, DedupeKey: opts.DedupeKey,
	})
	if err != nil {
		return "", "", err
	}
	if status == "queued" && id != "" {
		payload, _ := jsonMarshal(map[string]string{"deliveryId": id, "trigger": "email.enqueued"})
		_, _ = pool.Exec(ctx,
			"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES ($1, 'send_email', $2::jsonb, 'pending', 0, 3, NOW())",
			uuid.NewString(), string(payload))
	}
	return id, status, nil
}

// ProcessEmailInline: klaim → kirim via SMTP → mark sent/failed.
func ProcessEmailInline(ctx context.Context, pool *pgxpool.Pool, cmsSecret string, deliveryID string) (bool, string, string) {
	var template string
	var attempts int
	err := pool.QueryRow(ctx,
		"SELECT template, attempts FROM email_deliveries WHERE id = $1 LIMIT 1", deliveryID).Scan(&template, &attempts)
	if err != nil {
		return false, "", "Delivery not found"
	}

	// Parity: password-reset yang sudah >10 menit → skipped (link expired).
	if template == "password-reset" && MarkStalePasswordResetSkipped(ctx, pool, deliveryID) {
		return false, "", "Password reset email expired before delivery"
	}
	if !MarkSending(ctx, pool, deliveryID) {
		return false, "", "Delivery already claimed"
	}

	cfg, err := ResolveConfig(ctx, pool, cmsSecret)
	if err != nil {
		MarkFailed(ctx, pool, deliveryID, "NOT_CONFIGURED", "SMTP not configured")
		return false, "", "SMTP not configured"
	}

	var recipient, subject, html string
	err = pool.QueryRow(ctx,
		"SELECT recipient_email, subject, html FROM email_deliveries WHERE id = $1 LIMIT 1", deliveryID).
		Scan(&recipient, &subject, &html)
	if err != nil {
		return false, "", "Delivery disappeared"
	}

	if IsSuppressed(ctx, pool, recipient) {
		markSkipped(ctx, pool, deliveryID, "SUPPRESSED", "Alamat pernah memantul permanen")
		return false, "", "Recipient suppressed"
	}

	messageID, sendErr := smtpSend(cfg, recipient, subject, html)
	if sendErr != nil && IsHardBounce(sendErr) {
		// Alamat tidak ada: catat dan jangan coba lagi.
		MarkSuppressed(ctx, pool, recipient, "hard_bounce", sendErr.Error())
		markSkipped(ctx, pool, deliveryID, "HARD_BOUNCE", sendErr.Error())
		slog.Warn("[email] alamat mati dicatat", "delivery", deliveryID)
		return false, "", sendErr.Error()
	}
	if sendErr != nil {
		slog.Error("[email] SMTP send gagal", "err", sendErr, "delivery", deliveryID)
		MarkFailed(ctx, pool, deliveryID, "SMTP_ERROR", sendErr.Error())
		return false, "", sendErr.Error()
	}
	MarkSent(ctx, pool, deliveryID, messageID)
	slog.Info("[email] terkirim", "delivery", deliveryID, "to", recipient)
	return true, messageID, ""
}

// RenderTemplate: registry kecil untuk template yang sudah di-port (gaya
// default).
func RenderTemplate(name string, data map[string]string) (Rendered, error) {
	return RenderTemplateStyled(DefaultStyle(), name, data)
}

// RenderTemplateStyled: registry template bawaan dengan gaya admin.
func RenderTemplateStyled(style Style, name string, data map[string]string) (Rendered, error) {
	switch name {
	case "password-reset":
		return renderPasswordReset(style, data), nil
	case "password-changed":
		return renderPasswordChanged(style, data), nil
	case "account-invite":
		// Tanpa entri ini, user yang dibuat admin tanpa password tidak pernah
		// menerima email aktivasi: SendTransactionalEmail mengembalikan error
		// "template not registered" dan tidak ada baris yang diantrekan.
		return renderAccountInvite(style, data), nil
	case "brand-invitation":
		return renderBrandInvitation(style, data), nil
	case "email-verification-code":
		return renderEmailVerificationCode(style, data), nil
	case "account-exists-notice":
		return renderAccountExistsNotice(style, data), nil
	case "login-alert":
		return renderLoginAlert(style, data), nil
	default:
		return Rendered{}, fmt.Errorf("Email template not registered: %s", name)
	}
}
