package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	emailqueue "github.com/rahmaangriawan/tauthub-go/internal/email"
)

// Alamat email mati (hard bounce) dan komplain spam. Diisi dua jalur: penolakan
// langsung saat kirim (internal/email.ProcessEmailInline) dan webhook Kirim.id
// untuk bounce yang datang belakangan. Hanya baris reason='hard_bounce' yang
// menghentikan pengiriman; 'complaint' hanya informasi untuk admin.

// EmailSuppressionsList: GET /v1/platform/operations/email-suppressions
// ?search=&offset=&limit=
func (d *Deps) EmailSuppressionsList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	q := r.URL.Query()
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	limit := clampInt(q.Get("limit"), 20, 1, 100)
	search := strings.ToLower(strings.TrimSpace(q.Get("search")))

	where := ""
	args := []any{}
	if search != "" {
		args = append(args, "%"+search+"%")
		where = "WHERE email LIKE $1"
	}

	total := 0
	if row, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS total FROM email_suppressions "+where, args...); err == nil && row != nil {
		total = row.intOr("total", 0)
	}
	args = append(args, limit, offset)
	rows, err := d.queryRows(r.Context(),
		`SELECT email, reason, detail, created_at FROM email_suppressions `+where+
			` ORDER BY created_at DESC LIMIT $`+ptItoa(len(args)-1)+` OFFSET $`+ptItoa(len(args)), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"email": row.strOr("email", ""), "reason": row.strOr("reason", ""),
			"detail": row.strOr("detail", ""), "created_at": row.timePtr("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": items})
}

// EmailSuppressionRemove: DELETE /v1/platform/operations/email-suppressions
// body {email}. Menghapus baris sehingga alamat boleh dikirimi lagi (mis. setelah
// pemilik memperbaiki kotak masuknya).
func (d *Deps) EmailSuppressionRemove(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	email, _ := strField(body, "email")
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		writeErr(w, d, errUnprocessable("email is required"))
		return
	}
	row, err := d.queryRow(r.Context(),
		"DELETE FROM email_suppressions WHERE email = $1 RETURNING email", email)
	if err != nil || row == nil {
		writeErr(w, d, errNotFound("Alamat tidak ada di daftar."))
		return
	}
	writeAuditBestEffort(d, r.Context(), admin.ID, "email_suppression.removed", "email", email)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// kirimWebhookSecret: secret efektif webhook. Prioritas: yang diisi admin di
// halaman Konfigurasi (smtp_settings, ter-seal), lalu env KIRIM_WEBHOOK_SECRET.
// Mengembalikan secret dan sumbernya ("db", "env", atau "" bila belum ada).
func (d *Deps) kirimWebhookSecret(ctx context.Context) (string, string) {
	row, _ := d.queryRow(ctx,
		"SELECT webhook_secret_sealed FROM smtp_settings WHERE id = 'default' LIMIT 1")
	if row != nil {
		if sealed := row.strOr("webhook_secret_sealed", ""); sealed != "" {
			if plain, err := d.unsealT2(sealed); err == nil && strings.TrimSpace(plain) != "" {
				return strings.TrimSpace(plain), "db"
			}
		}
	}
	if d.Cfg != nil && d.Cfg.KirimWebhookSecret != "" {
		return d.Cfg.KirimWebhookSecret, "env"
	}
	return "", ""
}

// KirimWebhookSettingsGet: GET /v1/platform/settings/email/webhook. Tidak pernah
// mengembalikan secret, hanya status dan URL yang harus didaftarkan di Kirim.id.
func (d *Deps) KirimWebhookSettingsGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	_, source := d.kirimWebhookSecret(r.Context())
	base := ""
	if d.Cfg != nil {
		base = d.Cfg.PublicAppURL
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": source != "",
		"source":     source,
		"url":        base + "/v1/webhooks/email/kirim",
	})
}

// KirimWebhookSettingsPut: PUT /v1/platform/settings/email/webhook body {secret}.
// Secret kosong menghapus secret yang tersimpan di database.
func (d *Deps) KirimWebhookSettingsPut(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	secret, _ := strField(body, "secret")
	secret = strings.TrimSpace(secret)
	if len(secret) > 500 {
		writeErr(w, d, errUnprocessable("Secret terlalu panjang."))
		return
	}
	var sealed any
	if secret != "" {
		s, err := d.sealT2(secret)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		sealed = s
	}
	if err := d.exec(r.Context(), `
		INSERT INTO smtp_settings (id, webhook_secret_sealed, updated_at, updated_by)
		VALUES ('default', $1, NOW(), $2)
		ON CONFLICT (id) DO UPDATE SET
		  webhook_secret_sealed = EXCLUDED.webhook_secret_sealed,
		  updated_at = NOW(), updated_by = EXCLUDED.updated_by`,
		sealed, admin.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeAuditBestEffort(d, r.Context(), admin.ID, "smtp.webhook_secret_updated", "smtp_settings", "default")
	_, source := d.kirimWebhookSecret(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "configured": source != "", "source": source})
}

// kirimWebhookEvent: bidang webhook email Kirim.id yang dipakai. Field lain
// diabaikan (penyedia bisa menambah field kapan saja).
type kirimWebhookEvent struct {
	Event     string `json:"event"`
	To        string `json:"to"`
	Reason    string `json:"reason"`
	MessageID string `json:"message_id"`
}

// KirimEmailWebhook: POST /v1/webhooks/email/kirim (publik, ditandatangani).
//
// Keaslian dibuktikan dengan HMAC-SHA256 hex dari body mentah memakai secret
// webhook, dikirim di header X-Kirim-Signature. Dokumentasi Kirim.id tidak
// membedakan bounce permanen dan sementara, jadi jenisnya ditentukan dari teks
// `reason`: hanya penolakan 5xx yang menyatakan alamat tidak ada yang dicatat.
// Event lain (delivered, open, click, ...) dijawab 200 dan diabaikan, supaya
// penyedia tidak mengulang kirim.
func (d *Deps) KirimEmailWebhook(w http.ResponseWriter, r *http.Request) {
	secret, _ := d.kirimWebhookSecret(r.Context())
	if secret == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Webhook not configured"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Invalid webhook signature"})
		return
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	want := mac.Sum(nil)
	got, decErr := hex.DecodeString(strings.TrimSpace(r.Header.Get("X-Kirim-Signature")))
	if decErr != nil || !hmac.Equal(got, want) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Invalid webhook signature"})
		return
	}

	var ev kirimWebhookEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Invalid payload"})
		return
	}
	to := strings.ToLower(strings.TrimSpace(ev.To))
	switch strings.ToLower(ev.Event) {
	case "bounce":
		if to != "" && emailqueue.IsHardBounceReason(ev.Reason) {
			emailqueue.MarkSuppressed(r.Context(), d.Pool, to, "hard_bounce", "kirim.id: "+ev.Reason)
			slog.Warn("[email] bounce permanen dari webhook", "message_id", ev.MessageID)
		}
	case "complaint":
		if to != "" {
			emailqueue.MarkSuppressed(r.Context(), d.Pool, to, "complaint", "kirim.id: "+ev.Reason)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
