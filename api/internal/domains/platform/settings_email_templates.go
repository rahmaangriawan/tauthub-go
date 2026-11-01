package platform

// Admin API untuk panel Template di tab Email/SMTP:
//   GET/PUT  /platform/email/style
//   GET      /platform/email/templates
//   PUT      /platform/email/templates
//   DELETE   /platform/email/templates?audience=&event=
//   PUT      /platform/email/channels
//   POST     /platform/email/preview
//   POST     /platform/email/test-send
//
// Semua admin-only, dicatat di audit.

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
)

var (
	emailHexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	emailURLRe      = regexp.MustCompile(`^https?://[^\s]+$`)
)

func emailStyleJSON(s email.Style) map[string]any {
	return map[string]any{
		"brand_name":    s.BrandName,
		"logo_url":      s.LogoURL,
		"primary_color": s.PrimaryColor,
		"accent_color":  s.AccentColor,
		"page_bg":       s.PageBG,
		"card_bg":       s.CardBG,
		"text_color":    s.TextColor,
		"muted_color":   s.MutedColor,
		"font_stack":    s.FontStack,
		"button_radius": s.ButtonRadius,
		"footer_text":   s.FooterText,
		"support_email": s.SupportEmail,
	}
}

// emailStyleRead membaca langsung dari DB (bukan cache) supaya admin melihat
// nilai terbaru tepat setelah menyimpan.
func (d *Deps) emailStyleRead(ctx context.Context) email.Style {
	style := email.DefaultStyle()
	row, err := d.queryRow(ctx, `
		SELECT brand_name, logo_url, primary_color, accent_color, page_bg, card_bg,
		       text_color, muted_color, font_stack, button_radius, footer_text, support_email
		FROM email_branding WHERE id = 'default' LIMIT 1`)
	if err != nil || row == nil {
		return style
	}
	s := ptRow(ptNormalizeMap(row))
	style.BrandName = s.strOr("brand_name", style.BrandName)
	style.LogoURL = s.strOr("logo_url", style.LogoURL)
	style.PrimaryColor = s.strOr("primary_color", style.PrimaryColor)
	style.AccentColor = s.strOr("accent_color", style.AccentColor)
	style.PageBG = s.strOr("page_bg", style.PageBG)
	style.CardBG = s.strOr("card_bg", style.CardBG)
	style.TextColor = s.strOr("text_color", style.TextColor)
	style.MutedColor = s.strOr("muted_color", style.MutedColor)
	style.FontStack = s.strOr("font_stack", style.FontStack)
	style.ButtonRadius = s.intOr("button_radius", style.ButtonRadius)
	style.FooterText = s.strOr("footer_text", style.FooterText)
	style.SupportEmail = s.strOr("support_email", style.SupportEmail)
	return style
}

// EmailStyleGet: GET /v1/platform/email/style
func (d *Deps) EmailStyleGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	writeJSON(w, http.StatusOK, emailStyleJSON(d.emailStyleRead(r.Context())))
}

// EmailStylePut: PUT /v1/platform/email/style
func (d *Deps) EmailStylePut(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	style := d.emailStyleRead(r.Context())

	pick := func(key, current string) (string, bool) {
		v, has := strField(body, key)
		if !has {
			return current, true
		}
		return strings.TrimSpace(v), true
	}
	style.BrandName, _ = pick("brand_name", style.BrandName)
	style.LogoURL, _ = pick("logo_url", style.LogoURL)
	style.PrimaryColor, _ = pick("primary_color", style.PrimaryColor)
	style.AccentColor, _ = pick("accent_color", style.AccentColor)
	style.PageBG, _ = pick("page_bg", style.PageBG)
	style.CardBG, _ = pick("card_bg", style.CardBG)
	style.TextColor, _ = pick("text_color", style.TextColor)
	style.MutedColor, _ = pick("muted_color", style.MutedColor)
	style.FontStack, _ = pick("font_stack", style.FontStack)
	style.FooterText, _ = pick("footer_text", style.FooterText)
	style.SupportEmail, _ = pick("support_email", style.SupportEmail)
	if v, has := intField(body, "button_radius"); has {
		style.ButtonRadius = v
	}

	if n := len([]rune(style.BrandName)); n == 0 || n > 80 {
		writeErr(w, d, errUnprocessable("Nama brand 1-80 karakter."))
		return
	}
	if style.LogoURL != "" && (len(style.LogoURL) > 500 || !emailURLRe.MatchString(style.LogoURL)) {
		writeErr(w, d, errUnprocessable("URL logo harus http(s) dan maksimal 500 karakter."))
		return
	}
	for _, c := range []struct {
		label, value string
	}{
		{"Warna primer", style.PrimaryColor},
		{"Warna aksen", style.AccentColor},
		{"Latar halaman", style.PageBG},
		{"Latar kartu", style.CardBG},
		{"Warna teks", style.TextColor},
		{"Warna teks sekunder", style.MutedColor},
	} {
		if !emailHexColorRe.MatchString(c.value) {
			writeErr(w, d, errUnprocessable(c.label+" harus format #RRGGBB."))
			return
		}
	}
	if len(style.FontStack) > 200 || strings.ContainsAny(style.FontStack, "<>\n\r\"") {
		writeErr(w, d, errUnprocessable("Font stack tidak valid."))
		return
	}
	if style.ButtonRadius < 0 || style.ButtonRadius > 24 {
		writeErr(w, d, errUnprocessable("Radius tombol 0-24."))
		return
	}
	if len([]rune(style.FooterText)) > 300 {
		writeErr(w, d, errUnprocessable("Teks footer maksimal 300 karakter."))
		return
	}
	if len(style.SupportEmail) > 200 || (style.SupportEmail != "" && !stEmailRe.MatchString(style.SupportEmail)) {
		writeErr(w, d, errUnprocessable("Email dukungan tidak valid."))
		return
	}

	if err := d.exec(r.Context(), `
		INSERT INTO email_branding (id, brand_name, logo_url, primary_color, accent_color, page_bg, card_bg,
		                            text_color, muted_color, font_stack, button_radius, footer_text, support_email,
		                            updated_by, updated_at)
		VALUES ('default', $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, NULLIF($13,'')::uuid, NOW())
		ON CONFLICT (id) DO UPDATE SET
		  brand_name = EXCLUDED.brand_name, logo_url = EXCLUDED.logo_url,
		  primary_color = EXCLUDED.primary_color, accent_color = EXCLUDED.accent_color,
		  page_bg = EXCLUDED.page_bg, card_bg = EXCLUDED.card_bg,
		  text_color = EXCLUDED.text_color, muted_color = EXCLUDED.muted_color,
		  font_stack = EXCLUDED.font_stack, button_radius = EXCLUDED.button_radius,
		  footer_text = EXCLUDED.footer_text, support_email = EXCLUDED.support_email,
		  updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		style.BrandName, style.LogoURL, style.PrimaryColor, style.AccentColor, style.PageBG, style.CardBG,
		style.TextColor, style.MutedColor, style.FontStack, style.ButtonRadius, style.FooterText,
		style.SupportEmail, admin.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	email.InvalidateStyleCache()
	d.audit(r.Context(), admin.ID, "email.style_updated", "email_branding", "default")
	writeJSON(w, http.StatusOK, emailStyleJSON(style))
}

// EmailTemplatesList: GET /v1/platform/email/templates
func (d *Deps) EmailTemplatesList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	storedRows, err := d.queryRows(ctx, `
		SELECT audience, event, subject, body_html, enabled, updated_at FROM email_templates`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	type stored struct {
		subject, body string
		enabled       bool
		updatedAt     any
	}
	storedMap := map[string]stored{}
	for _, raw := range storedRows {
		row := ptRow(ptNormalizeMap(raw))
		storedMap[row.strOr("audience", "")+"|"+row.strOr("event", "")] = stored{
			subject:   row.strOr("subject", ""),
			body:      row.strOr("body_html", ""),
			enabled:   row.boolOr("enabled", true),
			updatedAt: row.anyOrNil("updated_at"),
		}
	}
	channels, _ := email.LoadChannels(ctx, d.Pool)

	items := make([]map[string]any, 0, len(email.Catalog()))
	for _, e := range email.Catalog() {
		item := map[string]any{
			"audience":        e.Audience,
			"event":           e.Event,
			"label":           e.Label,
			"sender":          string(e.Sender),
			"variables":       e.Variables,
			"subject":         e.Subject,
			"body_html":       e.BodyHTML,
			"default_subject": e.Subject,
			"default_body":    e.BodyHTML,
			"enabled":         true,
			"customized":      false,
			"updated_at":      nil,
		}
		if s, found := storedMap[e.Audience+"|"+e.Event]; found {
			item["subject"] = s.subject
			item["body_html"] = s.body
			item["enabled"] = s.enabled
			item["customized"] = true
			item["updated_at"] = s.updatedAt
		}
		ch := channels[e.Event]
		item["channel_enabled"] = ch.Enabled
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"style": emailStyleJSON(d.emailStyleRead(ctx)),
	})
}

// EmailTemplateSave: PUT /v1/platform/email/templates
func (d *Deps) EmailTemplateSave(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	audience, _ := strField(body, "audience")
	event, _ := strField(body, "event")
	subject, _ := strField(body, "subject")
	bodyHTML, _ := strField(body, "body_html")
	enabled, hasEnabled := boolField(body, "enabled")
	entry, found := email.CatalogEvent(strings.TrimSpace(audience), strings.TrimSpace(event))
	if !found {
		writeErr(w, d, errUnprocessable("Template tidak dikenal."))
		return
	}
	subject = strings.TrimSpace(subject)
	if n := len([]rune(subject)); n == 0 || n > 500 {
		writeErr(w, d, errUnprocessable("Subjek 1-500 karakter."))
		return
	}
	if n := len(bodyHTML); n == 0 || n > 20000 {
		writeErr(w, d, errUnprocessable("Isi email 1-20.000 karakter."))
		return
	}
	if !hasEnabled {
		enabled = true
	}
	if err := email.SaveTemplate(r.Context(), d.Pool, email.StoredTemplate{
		Audience: entry.Audience,
		Event:    entry.Event,
		Subject:  subject,
		BodyHTML: bodyHTML,
		Enabled:  enabled,
	}, admin.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), admin.ID, "email.template_updated", "email_template", entry.Audience+":"+entry.Event)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// EmailTemplateReset: DELETE /v1/platform/email/templates?audience=&event=
func (d *Deps) EmailTemplateReset(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	audience := strings.TrimSpace(q.Get("audience"))
	event := strings.TrimSpace(q.Get("event"))
	if _, found := email.CatalogEvent(audience, event); !found {
		writeErr(w, d, errUnprocessable("Template tidak dikenal."))
		return
	}
	if err := email.ResetTemplate(r.Context(), d.Pool, audience, event); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), admin.ID, "email.template_reset", "email_template", audience+":"+event)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// EmailChannelSet: PUT /v1/platform/email/channels
func (d *Deps) EmailChannelSet(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	event, _ := strField(body, "event")
	event = strings.TrimSpace(event)
	enabled, has := boolField(body, "enabled")
	if !has {
		writeErr(w, d, errUnprocessable("Field enabled wajib diisi."))
		return
	}

	// mode massal: {audience, all:true, enabled} menyalakan atau mematikan
	// email untuk seluruh template audiens tersebut. Dipakai tombol
	// "Aktifkan semua" di panel Template.
	all, _ := boolField(body, "all")
	if all {
		audience, _ := strField(body, "audience")
		audience = strings.TrimSpace(audience)
		if audience != email.AudienceClient && audience != email.AudiencePartner {
			writeErr(w, d, errUnprocessable("Audiens tidak dikenal."))
			return
		}
		events := email.CatalogFor(audience)
		for _, e := range events {
			if err := email.SetChannel(r.Context(), d.Pool, e.Event, enabled, admin.ID); err != nil {
				writeErr(w, d, err)
				return
			}
		}
		notify.InvalidateEmailChannelCache()
		d.audit(r.Context(), admin.ID, "email.channels_bulk_toggled", "email_event_channel", audience)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "audience": audience, "enabled": enabled, "count": len(events),
		})
		return
	}

	if !email.IsCatalogEvent(event) {
		writeErr(w, d, errUnprocessable("Event tidak dikenal."))
		return
	}
	if err := email.SetChannel(r.Context(), d.Pool, event, enabled, admin.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	notify.InvalidateEmailChannelCache()
	d.audit(r.Context(), admin.ID, "email.channel_toggled", "email_event_channel", event)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "event": event, "enabled": enabled})
}

// emailSampleData menyiapkan data pratinjau yang mengikuti kategori event,
// supaya admin melihat email yang benar-benar terbaca. Isi di sini adalah
// data contoh operasional (nama penerima, judul artikel, nominal), bukan
// kalimat meta seperti "ini contoh notifikasi".
func emailSampleData(audience, event string) map[string]string {
	link := "/klien/tauthub/publications"
	if audience == email.AudiencePartner {
		link = "/mitra/siar/publications"
	}
	s := map[string]string{
		"nama":   "Rina Kusuma",
		"judul":  "Strategi Konten B2B untuk 2026",
		"isi":    "Pesanan publikasi untuk Siar.co.id sudah diperbarui.",
		"status": "",
		"jumlah": "",
		"link":   link,
	}

	switch {
	case strings.HasPrefix(event, "security."):
		s["judul"] = "Keamanan akun Tauthub"
		s["isi"] = "Permintaan perubahan keamanan akun diterima."
		s["status"] = ""
		s["link"] = "/login"

	case strings.HasPrefix(event, "withdrawal."):
		s["judul"] = "Penarikan dana"
		s["jumlah"] = "Rp 2.750.000"
		s["status"] = "Transfer bank BCA"
		s["isi"] = "Penarikan dana Rp 2.750.000 telah dibayarkan ke rekening terdaftar."
		s["link"] = "/mitra/siar/settlements"
		switch event {
		case "withdrawal.requested":
			s["status"] = "Menunggu review"
			s["isi"] = "Permintaan penarikan Rp 2.750.000 diterima dan menunggu review Admin."
		case "withdrawal.processing":
			s["status"] = "Diproses"
			s["isi"] = "Penarikan dana Rp 2.750.000 sedang diproses Admin."
		case "withdrawal.rejected":
			s["status"] = "Ditolak"
			s["isi"] = "Penarikan dana Rp 2.750.000 ditolak. Alasan: nomor rekening belum aktif. Saldo telah dikembalikan."
		}

	case strings.HasPrefix(event, "ticket."):
		s["judul"] = "Artikel belum tayang setelah 3 hari"
		s["isi"] = "Artikel belum tayang setelah 3 hari"
		s["status"] = "Kami sudah memeriksa tautan dan meminta konfirmasi tim media. Artikel akan tayang paling lambat besok."
		s["link"] = "/mitra/siar/dukungan"

	case strings.HasPrefix(event, "publication."):
		s["judul"] = "Strategi Konten B2B untuk 2026"
		s["jumlah"] = "Rp 750.000"
		s["status"] = "Terbit"
		s["isi"] = "Pesanan \"Strategi Konten B2B untuk 2026\" untuk Siar.co.id sudah terbit."
		switch event {
		case "publication.failed":
			s["status"] = "Gagal"
			s["isi"] = "Publikasi ke Siar.co.id gagal karena koneksi API media ditolak."
		case "publication.partner_approved":
			s["status"] = "Disetujui media"
			s["isi"] = "Pesanan \"Strategi Konten B2B untuk 2026\" disetujui Siar.co.id."
		case "publication.partner_rejected":
			s["status"] = "Ditolak media"
			s["isi"] = "Pesanan \"Strategi Konten B2B untuk 2026\" ditolak Siar.co.id."
		case "publication.revision_requested":
			s["status"] = "Revisi diminta"
			s["isi"] = "Siar.co.id meminta revisi pada bagian pembuka artikel."
		case "publication.revision_accepted":
			s["status"] = "Revisi diterima"
			s["isi"] = "Siar.co.id menerima revisi artikel."
		case "publication.revision_rejected":
			s["status"] = "Revisi ditolak"
			s["isi"] = "Siar.co.id menolak revisi artikel."
		case "publication.sla_cancelled":
			s["status"] = "Dibatalkan otomatis"
			s["isi"] = "Pesanan dibatalkan otomatis karena media tidak menyetujui dalam 3×24 jam."
		case "publication.waiting_partner":
			s["status"] = "Menunggu persetujuan"
			s["isi"] = "Pesanan \"Strategi Konten B2B untuk 2026\" menunggu persetujuan Anda di Siar.co.id."
		case "publication.client_approved":
			s["status"] = "Disetujui klien"
			s["isi"] = "Klien menyetujui pesanan \"Strategi Konten B2B untuk 2026\"."
		case "publication.reminder":
			s["status"] = "Menunggu persetujuan"
			s["isi"] = "Pesanan menunggu persetujuan Anda sebelum tenggat."
		case "publication.late_approval":
			s["status"] = "Terlambat"
			s["isi"] = "Pesanan melewati batas waktu persetujuan."
		case "publication.auto_replaced":
			s["status"] = "Dialihkan"
			s["isi"] = "Pesanan dialihkan ke media pengganti karena tautan sebelumnya bermasalah."
		}

	case strings.HasPrefix(event, "billing."):
		s["judul"] = "Saldo akun Tauthub"
		s["jumlah"] = "Rp 1.250.000"
		s["status"] = "Saldo menipis"
		s["isi"] = "Saldo akun Anda saat ini Rp 120.000."
		s["link"] = "/klien/tauthub/credits"
		switch event {
		case "billing.balance_forfeit_warning":
			s["status"] = "Peringatan"
			s["isi"] = "Saldo akun Anda akan hangus dalam 7 hari."
		case "billing.balance_forfeited":
			s["status"] = "Hangus"
			s["isi"] = "Saldo akun Anda telah dihanguskan karena akun tidak aktif 12 bulan."
		}

	case strings.HasPrefix(event, "media."):
		s["judul"] = "Siar.co.id"
		s["status"] = "Disetujui"
		s["isi"] = "Media \"Siar.co.id\" telah disetujui Admin."
		s["link"] = "/mitra/siar/media"
		switch event {
		case "media.rejected":
			s["status"] = "Ditolak"
			s["isi"] = "Media \"Siar.co.id\" ditolak Admin. Alasan: kategori belum sesuai."
		case "media.dewan_pers.approved":
			s["status"] = "Terverifikasi"
			s["isi"] = "Verifikasi Dewan Pers untuk Siar.co.id disetujui."
		case "media.dewan_pers.rejected":
			s["status"] = "Ditolak"
			s["isi"] = "Verifikasi Dewan Pers untuk Siar.co.id ditolak."
		case "media.submitted":
			s["status"] = "Menunggu peninjauan"
			s["isi"] = "Pengajuan media \"Siar.co.id\" diterima dan menunggu peninjauan admin."
		case "media.resubmitted":
			s["status"] = "Menunggu peninjauan"
			s["isi"] = "Pengajuan ulang media \"Siar.co.id\" diterima dan menunggu peninjauan admin."
		case "media.negotiated":
			s["status"] = "Harga diperbarui"
			s["isi"] = "Harga media \"Siar.co.id\" berubah setelah negosiasi."
		case "media.domain_down":
			s["status"] = "Domain bermasalah"
			s["isi"] = "Domain Siar.co.id tidak dapat diakses saat pemeriksaan."
		case "media.crawl_blocked":
			s["status"] = "Crawl terblokir"
			s["isi"] = "Crawler tidak dapat membaca halaman Siar.co.id karena akses ditolak."
		}

	case strings.HasPrefix(event, "chat."):
		s["judul"] = "Siar.co.id"
		s["status"] = "Pesan baru"
		s["isi"] = "Artikel sudah kami terima. Perkiraan tayang besok pagi."
		if audience == email.AudiencePartner {
			s["link"] = "/mitra/siar/chat"
		} else {
			s["link"] = "/klien/tauthub/chat"
		}

	case strings.HasPrefix(event, "secure_direct."):
		s["judul"] = "Akses data kampanye Q1"
		s["status"] = "Menunggu persetujuan"
		s["isi"] = "Permintaan akses data aman diajukan untuk kampanye Q1."
		s["link"] = "/mitra/siar/connections"

	case strings.HasPrefix(event, "pr."):
		s["judul"] = "Strategi Konten B2B untuk 2026"
		s["status"] = "Selesai"
		s["isi"] = "Verifikasi PR untuk artikel sudah selesai. Hasil lengkap tersedia di detail artikel."
		s["link"] = "/mitra/siar/publications"

	case strings.HasPrefix(event, "vip."):
		s["judul"] = "Program VIP Tauthub"
		s["status"] = "Aktif"
		s["isi"] = "Fitur VIP aktif sesuai masa kontrak yang tercatat di dashboard."
		s["link"] = "/mitra/siar/vip"

	case strings.HasPrefix(event, "partner."):
		s["judul"] = "Profil mitra"
		s["status"] = "Nomor belum lengkap"
		s["isi"] = "Nomor WhatsApp belum terdaftar di akun Anda."
		s["link"] = "/mitra/siar/profile"
	}
	return s
}

// EmailPreview: POST /v1/platform/email/preview
func (d *Deps) EmailPreview(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	body := decodeJSON(r)
	audience, _ := strField(body, "audience")
	event, _ := strField(body, "event")
	subject, _ := strField(body, "subject")
	bodyHTML, _ := strField(body, "body_html")
	style := d.emailStyleRead(r.Context())
	if raw, has := body["style"].(map[string]any); has {
		if v, ok := raw["brand_name"].(string); ok && strings.TrimSpace(v) != "" {
			style.BrandName = strings.TrimSpace(v)
		}
		for key, target := range map[string]*string{
			"logo_url": &style.LogoURL, "primary_color": &style.PrimaryColor,
			"accent_color": &style.AccentColor, "page_bg": &style.PageBG,
			"card_bg": &style.CardBG, "text_color": &style.TextColor,
			"muted_color": &style.MutedColor, "font_stack": &style.FontStack,
			"footer_text": &style.FooterText, "support_email": &style.SupportEmail,
		} {
			if v, ok := raw[key].(string); ok {
				*target = v
			}
		}
		if v, ok := raw["button_radius"].(float64); ok {
			style.ButtonRadius = int(v)
		}
	}
	if strings.TrimSpace(event) != "" {
		if entry, found := email.CatalogEvent(audience, event); found {
			if strings.TrimSpace(subject) == "" {
				subject = entry.Subject
			}
			if strings.TrimSpace(bodyHTML) == "" {
				bodyHTML = entry.BodyHTML
			}
		}
	}
	data := emailSampleData(strings.TrimSpace(audience), strings.TrimSpace(event))
	rendered := email.RenderTemplateHTML(style, subject, bodyHTML, data)
	writeJSON(w, http.StatusOK, map[string]any{"subject": rendered.Subject, "html": rendered.HTML})
}

// EmailTemplateTestSend: POST /v1/platform/email/test-send
func (d *Deps) EmailTemplateTestSend(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	body := decodeJSON(r)
	audience, _ := strField(body, "audience")
	event, _ := strField(body, "event")
	recipient, _ := strField(body, "recipient_email")
	recipient = strings.TrimSpace(recipient)
	if !stEmailRe.MatchString(recipient) {
		writeErr(w, d, errUnprocessable("Email tujuan tidak valid."))
		return
	}
	entry, found := email.CatalogEvent(strings.TrimSpace(audience), strings.TrimSpace(event))
	if !found {
		writeErr(w, d, errUnprocessable("Template tidak dikenal."))
		return
	}
	style := d.emailStyleRead(ctx)
	data := emailSampleData(entry.Audience, entry.Event)
	subject, htmlBody, okStored := email.RenderStored(ctx, d.Pool, style, entry.Audience, entry.Event, data)
	if !okStored {
		subject = strings.TrimSpace(email.SubstituteVariables(entry.Subject, data))
		htmlBody = email.PageStyled(style, subject, subject, email.SubstituteVariables(entry.BodyHTML, data)).HTML
	}
	cfg, cfgErr := d.stResolveSMTP(ctx, true)
	if cfgErr != nil {
		writeErr(w, d, errUnprocessable("Konfigurasi SMTP belum bisa dipakai. Simpan ulang kredensial SMTP."))
		return
	}
	messageID, sendErr := stSMTPSendNow(cfg, recipient, subject, htmlBody)
	if sendErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "detail": sendErr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "id": messageID, "detail": "Email uji terkirim ke " + recipient + ".",
	})
}
