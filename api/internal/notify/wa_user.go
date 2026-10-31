package notify

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnqueueUserWA: antrekan WhatsApp ke SATU user (mitra atau klien) memakai
// template untuk audiensnya (best-effort).
//
// Mengembalikan true bila satu baris antrean benar-benar dibuat. Penolakan
// (config mati, template nonaktif, tanpa nomor WA, tanpa izin, dedupe, kuota)
// bukan error: pemanggil cukup melanjutkan.
//
// Template diambil dari tabel wa_templates lebih dulu (per event + audiens),
// lalu jatuh ke teks bawaan di WADefaultTemplates. Dengan begitu tombol
// "Nonaktif" dan hasil suntingan admin benar-benar berpengaruh.
type UserWAOptions struct {
	// Audience salah satu AudiencePartner/AudienceClient/AudienceAdmin.
	Audience string
	// Event tipe notifikasi, mis. "publication.published".
	Event string
	// UserID penerima (users.id).
	UserID string
	// ReferenceID identitas entitas (publication id, settlement id, dst) untuk dedupe.
	ReferenceID string
	// Data variabel template: nama, judul, jumlah, status.
	Data map[string]string
	// Link halaman tujuan (relatif atau absolut). Relatif diberi BaseURL.
	Link string
	// BaseURL origin aplikasi, mis. https://app.tauthub.com.
	BaseURL string
}

// EnqueueUserWA mengantrekan satu pesan WA ke satu penerima.
func EnqueueUserWA(ctx context.Context, pool *pgxpool.Pool, opts UserWAOptions) bool {
	if pool == nil || opts.Event == "" || opts.UserID == "" {
		return false
	}
	audience := NormalizeAudience(opts.Audience)
	if !WAKnownEvent(audience, opts.Event) {
		return false
	}

	var enabled bool
	var country string
	if err := pool.QueryRow(ctx,
		"SELECT enabled, country_code FROM platform_wa_config WHERE id = 'default' LIMIT 1").
		Scan(&enabled, &country); err != nil || !enabled {
		return false
	}
	if country == "" {
		country = "62"
	}

	body, allowed := waTemplateBody(ctx, pool, audience, opts.Event)
	if !allowed {
		return false
	}

	var number string
	var consent bool
	if err := pool.QueryRow(ctx,
		"SELECT COALESCE(wa_number, ''), COALESCE(wa_consent, false) FROM notification_preferences WHERE user_id = $1 LIMIT 1",
		opts.UserID).Scan(&number, &consent); err != nil {
		return false
	}
	if !consent {
		return false
	}
	target := normalizeWANumber(number, country)
	if target == "" {
		return false
	}

	link := strings.TrimSpace(opts.Link)
	if link != "" && strings.HasPrefix(link, "/") && opts.BaseURL != "" {
		link = strings.TrimRight(opts.BaseURL, "/") + link
	}
	rendered := body
	for k, v := range opts.Data {
		rendered = strings.ReplaceAll(rendered, "{"+k+"}", v)
	}
	rendered = strings.ReplaceAll(rendered, "{link}", link)

	dedupe := opts.Event + ":" + opts.ReferenceID + ":" + target
	req := Request{UserID: opts.UserID, Type: opts.Event, DedupeKey: dedupe, Channel: "wa"}
	decision := ShouldSend(ctx, pool, req)
	LogDecision(decision, req)
	if !decision.Send {
		return false
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO wa_deliveries (id, user_id, target, event, rendered_body, dedupe_key, reference_id, status, attempts)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'queued', 0)
		ON CONFLICT DO NOTHING`,
		opts.UserID, target, opts.Event, rendered, dedupe, opts.ReferenceID); err != nil {
		slog.Warn("[notify] gagal mengantrekan WA", "event", opts.Event, "err", err)
		return false
	}
	return true
}

// waTemplateBody: isi template tersimpan untuk (audiens, event), atau teks
// bawaan bila belum pernah disunting. allowed=false saat admin menonaktifkan.
func waTemplateBody(ctx context.Context, pool *pgxpool.Pool, audience, event string) (string, bool) {
	var body string
	var enabled bool
	err := pool.QueryRow(ctx,
		"SELECT body, enabled FROM wa_templates WHERE event = $1 AND audience = $2 LIMIT 1",
		event, audience).Scan(&body, &enabled)
	if err != nil {
		def := WADefaultTemplate(audience, event)
		if def == "" {
			return "", false
		}
		return def, true
	}
	if !enabled {
		return "", false
	}
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		return trimmed, true
	}
	def := WADefaultTemplate(audience, event)
	if def == "" {
		return "", false
	}
	return def, true
}

// BaseURL: origin publik aplikasi untuk menautkan WA (WA tidak bisa membuka
// path relatif). PUBLIC_APP_URL diutamakan; fallback ke domain produksi.
func BaseURL() string {
	if v := strings.TrimSpace(os.Getenv("PUBLIC_APP_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://app.tauthub.com"
}
