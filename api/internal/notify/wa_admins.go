package notify

import (
	"context"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// Notifikasi WhatsApp ke ADMIN lewat Fonnte (antrean wa_deliveries).
//
// Tiga event pertama: pengajuan media/domain mitra, permintaan penarikan dana,
// dan tiket dukungan baru dari mitra. Penerima = semua user admin aktif yang
// sudah mengisi nomor WA + memberi izin (notification_preferences.wa_consent).
//
// Template disimpan di tabel wa_templates (bisa diedit admin di halaman WA
// Gateway); peta di bawah hanya nilai bawaan ketika baris tersimpan belum ada.
// Tipe event sengaja memakai prefix "admin." agar tidak mencampur template
// mitra (mis. withdrawal.requested versi mitra) dan agar gate notify.ShouldSend
// punya policy sendiri (WA immediate).

const (
	AdminEventMediaSubmitted      = "admin.media_submitted"
	AdminEventWithdrawalRequested = "admin.withdrawal_requested"
	AdminEventTicketCreated       = "admin.ticket_created"
)

// AdminWADefaultTemplates: template bawaan event admin (satu sumber; platform
// menggabungkan peta ini ke daftar template di halaman WA Gateway).
var AdminWADefaultTemplates = map[string]string{
	AdminEventMediaSubmitted:      "📥 *Pengajuan media baru*\n{nama} mengajukan \"{judul}\".\n\nReview: {link}",
	AdminEventWithdrawalRequested: "💸 *Penarikan dana baru*\n{nama} mengajukan pencairan {jumlah}.\n\nReview: {link}",
	AdminEventTicketCreated:       "🎫 *Tiket dukungan baru*\n{nama}: {judul}\n\nBuka: {link}",
}

// AdminWAPreviewLinks: contoh tautan untuk pratinjau template di admin.
var AdminWAPreviewLinks = map[string]string{
	AdminEventMediaSubmitted:      "https://app.tauthub.com/admin/media?status=pending_review",
	AdminEventWithdrawalRequested: "https://app.tauthub.com/admin/settlements",
	AdminEventTicketCreated:       "https://app.tauthub.com/admin/dukungan",
}

// AdminWAOptions: satu kejadian yang perlu dikabarkan ke admin via WA.
type AdminWAOptions struct {
	Event       string
	ReferenceID string
	// Data variabel template: nama, judul, jumlah, status.
	Data map[string]string
	// Link halaman admin (relatif atau absolut). Relatif akan diberi BaseURL.
	Link    string
	BaseURL string
}

// EnqueueAdminsWA: antrekan WA ke semua admin (best-effort). Mengembalikan
// jumlah baris antrean yang dibuat. Tanpa config aktif/template nonaktif →
// tidak ada yang dikirim, tanpa error.
func EnqueueAdminsWA(ctx context.Context, pool *pgxpool.Pool, opts AdminWAOptions) int {
	if pool == nil || opts.Event == "" {
		return 0
	}
	var enabled bool
	var country string
	var sealedGroup string
	var plainGroup string
	if err := pool.QueryRow(ctx,
		"SELECT enabled, country_code, COALESCE(sealed_admin_group_target, ''), COALESCE(admin_group_target, '') FROM platform_wa_config WHERE id = 'default' LIMIT 1").
		Scan(&enabled, &country, &sealedGroup, &plainGroup); err != nil || !enabled {
		return 0
	}
	// ID grup disimpan terenkripsi (sealed). Data lama yang masih plaintext
	// dipakai sebagai fallback sampai dienkripsi oleh BackfillWAGroupTarget.
	groupTarget := strings.TrimSpace(plainGroup)
	if strings.TrimSpace(sealedGroup) != "" {
		if plain, uErr := crypto.UnsealCompat(sealedGroup,
			os.Getenv("CMS_ENCRYPTION_SECRET"), os.Getenv("APP_SECRET")); uErr == nil {
			groupTarget = strings.TrimSpace(plain)
		}
	}
	if country == "" {
		country = "62"
	}

	body := AdminWADefaultTemplates[opts.Event]
	if body == "" {
		return 0
	}
	var savedBody string
	var savedEnabled bool
	// Filter audiens wajib: tabel sekarang punya PRIMARY KEY (event, audience),
	// jadi event yang sama bisa punya baris mitra/klien. Tanpa filter ini admin
	// bisa terkirim teks milik audiens lain.
	if err := pool.QueryRow(ctx,
		"SELECT body, enabled FROM wa_templates WHERE event = $1 AND audience = $2 LIMIT 1",
		opts.Event, AudienceAdmin).
		Scan(&savedBody, &savedEnabled); err == nil {
		if !savedEnabled {
			return 0
		}
		if strings.TrimSpace(savedBody) != "" {
			body = savedBody
		}
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

	// Grup WA (bila diisi admin di WA Gateway): satu pesan untuk seluruh tim —
	// tidak butuh nomor pribadi/consent per admin, dan tidak terkena kuota per
	// user karena penerimanya satu tujuan bersama.
	if group := strings.TrimSpace(groupTarget); group != "" {
		dedupe := opts.Event + ":" + opts.ReferenceID + ":group:" + group
		if _, err := pool.Exec(ctx, `
			INSERT INTO wa_deliveries (id, user_id, target, event, rendered_body, dedupe_key, reference_id, status, attempts)
			VALUES (gen_random_uuid(), NULL, $1, $2, $3, $4, $5, 'queued', 0)
			ON CONFLICT DO NOTHING`,
			group, opts.Event, rendered, dedupe, opts.ReferenceID); err == nil {
			return 1
		}
		return 0
	}

	rows, err := pool.Query(ctx, `
		SELECT u.id, COALESCE(np.wa_number, ''), COALESCE(np.wa_consent, false)
		FROM users u
		LEFT JOIN notification_preferences np ON np.user_id = u.id
		WHERE u.role = 'admin' AND u.is_active = TRUE`)
	if err != nil {
		return 0
	}
	type recipient struct {
		id, number string
		consent    bool
	}
	list := []recipient{}
	for rows.Next() {
		var r recipient
		if err := rows.Scan(&r.id, &r.number, &r.consent); err == nil {
			list = append(list, r)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0
	}

	queued := 0
	for _, r := range list {
		if !r.consent {
			continue
		}
		target := normalizeWANumber(r.number, country)
		if target == "" {
			continue
		}
		dedupe := opts.Event + ":" + opts.ReferenceID + ":" + target
		req := Request{UserID: r.id, Type: opts.Event, DedupeKey: dedupe, Channel: "wa"}
		decision := ShouldSend(ctx, pool, req)
		LogDecision(decision, req)
		if !decision.Send {
			continue
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO wa_deliveries (id, user_id, target, event, rendered_body, dedupe_key, reference_id, status, attempts)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'queued', 0)
			ON CONFLICT DO NOTHING`,
			r.id, target, opts.Event, rendered, dedupe, opts.ReferenceID); err == nil {
			queued++
		}
	}
	return queued
}

// normalizeWANumber: salinan aturan platform.igWANormalizeNumber (paket notify
// tidak boleh mengimpor domain platform — arah dependensinya sebaliknya).
func normalizeWANumber(raw, countryCode string) string {
	if raw == "" {
		return ""
	}
	value := strings.NewReplacer(" ", "", "(", "", ")", "", "-", "", ".", "", "+", "").Replace(raw)
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	if countryCode == "" {
		countryCode = "62"
	}
	if strings.HasPrefix(value, "0") {
		value = countryCode + value[1:]
	} else if !strings.HasPrefix(value, countryCode) {
		if len(value) <= 11 {
			value = countryCode + value
		}
	}
	if len(value) < 8 || len(value) > 15 {
		return ""
	}
	return value
}
