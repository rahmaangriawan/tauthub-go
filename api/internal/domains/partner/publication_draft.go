package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/content"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/imageconvert"
)

// ---------- GET/PUT /v1/partner/publications/{publicationId}/draft ----------

func (d *Deps) PublicationDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		d.publicationDraftPut(w, r)
		return
	}
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	status := publication.strOr("status", "")
	// `revision` ikut boleh dipratinjau supaya mitra bisa membaca versi revisi
	// sebelum memutuskan terima/tolak. Jalur PUT tetap tertutup untuk status ini
	// (lihat publicationDraftPut) agar editor mitra tidak menimpa revisi klien.
	if status != "published" && status != "waiting_partner" && status != "revision" {
		d.writeErr(w, errConflict("Preview hanya untuk artikel yang menunggu atau sudah terkirim ke media Anda."))
		return
	}
	isDraftMode := status == "published" && publication.strOr("publish_status", "draft") == "draft"
	if isDraftMode {
		if payload, ok := d.wpDraftPreview(r, publicationID, publication); ok {
			writeJSON(w, http.StatusOK, payload)
			return
		}
	}
	payload, err := d.clientDocumentPreview(r, publicationID, publication)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// wpDraftPreview: parity cabang draf WordPress (baca draf live dari WP).
func (d *Deps) wpDraftPreview(r *http.Request, publicationID string, publication rowMap) (map[string]any, bool) {
	conn, err := d.resolveWPConnection(r, publication.strOr("media_partner_id", ""), publication.strOr("connection_id", ""))
	if err != nil || conn == nil {
		return nil, false
	}
	job, err := d.queryRow(r.Context(),
		"SELECT remote_post_id FROM publication_jobs WHERE publication_id = $1 AND remote_post_id IS NOT NULL ORDER BY created_at DESC LIMIT 1",
		publicationID)
	if err != nil || job == nil {
		return nil, false
	}
	postID := job.strOr("remote_post_id", "")
	if postID == "" {
		return nil, false
	}
	post, ok := d.fetchWPDraft(r, conn, postID)
	if !ok {
		return nil, false
	}
	edits := d.partnerEdits(r.Context(), publicationID)
	docMeta, _ := d.queryRow(r.Context(), "SELECT meta_description FROM content_documents WHERE id = $1 LIMIT 1", publication.raw("document_id"))

	title := firstNonNilString(edits["title"], wpRenderedField(post, "title"), publication.raw("document_title"), "")
	contentHTML := firstNonNilString(edits["content_html"], wpRenderedField(post, "content"), "")
	metaDescription := firstNonNilString(edits["meta_description"], docMeta.raw("meta_description"))
	featured := firstNonNilString(edits["featured_image_url"], publication.raw("featured_image_url"))
	return map[string]any{
		"title":              title,
		"content_html":       relSelfHTMLAny(normalizeDraftHTML(contentHTML)),
		"meta_description":   metaDescription,
		"featured_image_url": relSelfURLAny(featured),
		"wp_status":          firstNonNilString(post["status"], "draft"),
		"category":           draftCategory(publication),
		"edited_at":          edits["edited_at"],
		"source":             "wp_draft",
	}, true
}

// draftCategory: kategori yang dipakai klien untuk publikasi — override mitra
// menang atas kategori media (pilihan klien saat order).
func draftCategory(publication rowMap) string {
	if ov := strings.TrimSpace(publication.strOr("category_override", "")); ov != "" {
		return ov
	}
	return strings.TrimSpace(publication.strOr("media_category", ""))
}

// normalizeDraftHTML: rapikan markup draf sebelum ditampilkan ke mitra, supaya
// sisa editor klien (<div> per baris, <div><br></div>) tidak muncul sebagai tag
// sampah di form edit. Nilai non-string dilewatkan apa adanya.
func normalizeDraftHTML(value any) any {
	s, ok := value.(string)
	if !ok || s == "" {
		return value
	}
	return content.NormalizeArticleHTML(s)
}

func firstNonNilString(vals ...any) any {
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	// Bila hanya nil yang tersedia, kembalikan nil (bukan string kosong) agar
	// parity field opsional (meta_description/featured_image_url) terjaga.
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func wpRenderedField(post map[string]any, field string) any {
	obj, ok := post[field].(map[string]any)
	if !ok {
		return nil
	}
	return obj["rendered"]
}

func (d *Deps) fetchWPDraft(r *http.Request, conn *wpConnection, postID string) (map[string]any, bool) {
	resp, err := wpRequest(r.Context(), wpURL(conn.BaseURL, "wp/v2/posts/"+postID+"?context=edit"), http.MethodGet, wpHeaders(conn.Username, conn.Secret), "", 30*time.Second)
	if err != nil {
		return nil, false
	}
	if resp.status < 200 || resp.status >= 300 {
		return nil, false
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(resp.body), &parsed) != nil {
		return nil, false
	}
	return parsed, true
}

// clientDocumentPreview: parity clientDocumentContent — dokumen klien sebagai fallback.
func (d *Deps) clientDocumentPreview(r *http.Request, publicationID string, publication rowMap) (map[string]any, error) {
	doc, err := d.queryRow(r.Context(),
		"SELECT title, current_version, meta_description FROM content_documents WHERE id = $1 LIMIT 1", publication.raw("document_id"))
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errNotFound("Dokumen klien tidak ditemukan untuk artikel ini.")
	}
	version := doc.intOr("current_version", 0)
	versionRow, err := d.queryRow(r.Context(),
		"SELECT content, content_format FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		publication.raw("document_id"), version)
	if err != nil {
		return nil, err
	}
	html := ""
	if versionRow != nil {
		raw := versionRow.strOr("content", "")
		if versionRow.strOr("content_format", "") == "markdown" {
			html = content.MarkdownToHtml(raw)
		} else {
			html = raw
		}
	}
	edits := d.partnerEdits(r.Context(), publicationID)
	return map[string]any{
		"title":              firstNonNilString(edits["title"], doc.raw("title"), ""),
		"content_html":       relSelfHTMLAny(normalizeDraftHTML(firstNonNilString(edits["content_html"], html))),
		"meta_description":   firstNonNilString(edits["meta_description"], doc.raw("meta_description")),
		"featured_image_url": relSelfURLAny(firstNonNilString(edits["featured_image_url"], publication.raw("featured_image_url"))),
		"wp_status":          "terbit",
		"category":           draftCategory(publication),
		"edited_at":          edits["edited_at"],
		"source":             "client_document",
	}, nil
}

// partnerEdits: baca partner_draft_edits sebagai map (kosong bila null).
func (d *Deps) partnerEdits(ctx context.Context, publicationID string) map[string]any {
	row, _ := d.queryRow(ctx, "SELECT partner_draft_edits FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if row == nil {
		return map[string]any{}
	}
	switch t := row.raw("partner_draft_edits").(type) {
	case map[string]any:
		return t
	case string:
		var m map[string]any
		if json.Unmarshal([]byte(t), &m) == nil && m != nil {
			return m
		}
	}
	return map[string]any{}
}

func (d *Deps) publicationDraftPut(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	status := publication.strOr("status", "")
	if status != "published" && status != "waiting_partner" {
		d.writeErr(w, errConflict("Edit hanya untuk artikel yang menunggu atau sudah terkirim ke media Anda."))
		return
	}
	if status == "published" && publication.strOr("publish_status", "draft") == "publish" {
		d.writeErr(w, errConflict("Artikel yang sudah terbit tidak bisa diedit."))
		return
	}
	body := decodeJSON(r)
	title, hasTitle := strField(body, "title")
	contentHTML, hasContent := strField(body, "content_html")
	metaDescription, hasMeta := strField(body, "meta_description")
	category, hasCategory := strField(body, "category")
	var featuredImage any
	hasFeatured := false
	if v, present := body["featured_image_url"]; present {
		hasFeatured = true
		featuredImage = v
	}
	if !hasTitle && !hasContent && !hasMeta && !hasFeatured && !hasCategory {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Tidak ada perubahan: kirim title, content_html, meta_description, dan/atau featured_image_url.")
		return
	}
	if hasTitle && (title == "" || len(title) > 200) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid title")
		return
	}
	if hasContent && len(contentHTML) > 200_000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid content_html")
		return
	}
	if hasMeta && len(metaDescription) > 500 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid meta_description")
		return
	}
	// Simpan versi yang sudah dirapikan agar editor mitra tidak menambah tag
	// sampah baru dan draf WordPress menerima markup yang bersih.
	if hasContent {
		contentHTML = content.NormalizeArticleHTML(contentHTML)
	}

	// Sinkronisasi draf WordPress (best-effort; snapshot lokal tetap disimpan).
	isDraftMode := status == "published" && publication.strOr("publish_status", "draft") == "draft"
	if isDraftMode {
		d.syncWPDraft(r, publication, title, contentHTML, metaDescription, hasTitle, hasContent, hasMeta)
	}

	edits := d.partnerEdits(r.Context(), publicationID)
	edits["edited_at"] = isoMilli(time.Now())
	if hasTitle {
		edits["title"] = title
	}
	if hasContent {
		edits["content_html"] = contentHTML
	}
	if hasMeta {
		edits["meta_description"] = metaDescription
	}
	if hasFeatured {
		if s, ok := featuredImage.(string); ok {
			edits["featured_image_url"] = strings.TrimSpace(s)
		} else {
			edits["featured_image_url"] = nil
		}
	}
	if err := d.exec(r.Context(),
		"UPDATE publications SET partner_draft_edits = $1::jsonb, updated_at = NOW() WHERE id = $2",
		marshalJSON(edits), publicationID); err != nil {
		d.writeErr(w, err)
		return
	}
	// Kategori bisa diubah mitra di editor (dropdown) — disimpan sebagai override
	// atas kategori media pilihan klien; kosong = kembali ke pilihan klien.
	if hasCategory {
		if err := d.exec(r.Context(),
			"UPDATE publications SET category_override = NULLIF($1, ''), updated_at = NOW() WHERE id = $2",
			strings.TrimSpace(category), publicationID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	d.auditLog(r.Context(), user.ID, "publication.partner_draft_edited", "publication", publicationID)
	updated, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, partnerPublicationJSON(updated))
}

func (d *Deps) syncWPDraft(r *http.Request, publication rowMap, title, contentHTML, metaDescription string, hasTitle, hasContent, hasMeta bool) {
	conn, err := d.resolveWPConnection(r, publication.strOr("media_partner_id", ""), publication.strOr("connection_id", ""))
	if err != nil || conn == nil {
		return
	}
	job, err := d.queryRow(r.Context(),
		"SELECT remote_post_id FROM publication_jobs WHERE publication_id = $1 AND remote_post_id IS NOT NULL ORDER BY created_at DESC LIMIT 1",
		publication.strOr("id", ""))
	if err != nil || job == nil || job.strOr("remote_post_id", "") == "" {
		return
	}
	payload := map[string]any{"status": "draft"}
	if hasTitle {
		payload["title"] = title
	}
	if hasContent {
		payload["content"] = contentHTML
	}
	if hasMeta {
		payload["excerpt"] = metaDescription
	}
	resp, err := wpRequest(r.Context(),
		wpURL(conn.BaseURL, "wp/v2/posts/"+job.strOr("remote_post_id", "")), http.MethodPost,
		wpHeaders(conn.Username, conn.Secret), marshalJSON(payload), 30*time.Second)
	if err != nil {
		d.logger().Warn("WP sync error", "publication", publication.strOr("id", ""), "err", err)
		return
	}
	if resp.status < 200 || resp.status >= 300 {
		d.logger().Warn("WP sync gagal", "publication", publication.strOr("id", ""), "status", resp.status,
			"body", truncateRunes(resp.body, 200))
	}
}

// ---------- POST /v1/partner/publications/{publicationId}/draft/image ----------

// draftImageMIME: format yang diizinkan — GIF & BMP sengaja ditiadakan
// (aturan pemilik produk): JPG/PNG otomatis dijadikan WebP oleh klien, jadi
// yang benar-benar dibutuhkan hanya header PNG/JPG/WebP.
var draftImageMIME = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/webp": true,
}

func (d *Deps) PublicationDraftImage(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	status := publication.strOr("status", "")
	if status != "published" && status != "waiting_partner" {
		httpx.Error(w, http.StatusConflict, "Upload gambar hanya untuk artikel yang menunggu atau sudah terkirim.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File tidak ditemukan.")
		return
	}
	defer file.Close()
	mime := header.Header.Get("Content-Type")
	if !draftImageMIME[mime] {
		httpx.Error(w, http.StatusUnprocessableEntity, "Format gambar harus JPG, PNG, atau WebP.")
		return
	}
	const maxBytes = 10 * 1024 * 1024
	if header.Size > maxBytes {
		httpx.Error(w, http.StatusUnprocessableEntity, "Ukuran gambar maksimal 10 MB.")
		return
	}
	data, err := readAllLimited(file, maxBytes)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// Selalu ke WebP (parity app lama); gagal → simpan byte asli.
	payload := data
	ext := sanitizeExt(strings.ToLower(header.Filename), "png")
	if converted, ok := imageconvert.ConvertToWebP(data, mime); ok {
		payload = converted
		ext = "webp"
	}
	storageKey := uuid.NewString() + "." + ext
	if err := d.saveUpload(storageKey, payload); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "publication.partner_draft_image_uploaded", "publication", publicationID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"url":        "/uploads/" + storageKey,
		"size_bytes": len(payload),
	})
}

// ---------- POST /v1/partner/publications/{publicationId}/rewrite ----------

// rewriteMaxPerPublication: batas pemakaian Rewrite AI per artikel — aturan
// pemilik produk: gratis dulu, maksimal 2 kali generate per publikasi.
const rewriteMaxPerPublication = 2

func (d *Deps) PublicationRewrite(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	status := publication.strOr("status", "")
	if status != "published" && status != "waiting_partner" {
		d.writeErr(w, errConflict("Rewrite hanya untuk artikel yang menunggu atau sudah terkirim ke media Anda."))
		return
	}
	if status == "published" && publication.strOr("publish_status", "draft") == "publish" {
		d.writeErr(w, errConflict("Artikel yang sudah terbit tidak bisa di-rewrite."))
		return
	}
	body := decodeJSON(r)
	title, hasTitle := strField(body, "title")
	contentHTML, hasContent := strField(body, "content_html")
	if !hasTitle || !hasContent {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid request")
		return
	}
	if len(title) == 0 || len(title) > 500 || len(contentHTML) > 200_000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid request")
		return
	}

	// Klaim jatah Rewrite secara atomik: hanya 2 pemakaian per publikasi.
	// Kalau generate gagal, jatah dikembalikan di bawah supaya mitra bisa mencoba lagi.
	claimed, err := d.execRows(r.Context(),
		"UPDATE publications SET rewrite_ai_count = rewrite_ai_count + 1 WHERE id = $1 AND COALESCE(rewrite_ai_count, 0) < $2",
		publicationID, rewriteMaxPerPublication)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if claimed == 0 {
		d.writeErr(w, errConflict(fmt.Sprintf("Batas Rewrite AI tercapai — maksimal %d kali per artikel.", rewriteMaxPerPublication)))
		return
	}

	// Mesin tulis The Writer: blok ANGLE pilihan klien + bahan writer dari
	// kampanye (topik tersimpan) dibaca dari basis data kampanye publikasi ini,
	// sehingga rewrite menyesuaikan sudut pandang sesuai angle yang dipilih klien.
	angleBlock := d.rewriteAngleBlock(r.Context(), publication)
	topicBlock := d.rewriteTopicBlock(r.Context(), publication)
	prompt := buildRewritePrompt(title, contentHTML, angleBlock, topicBlock)
	result, err := d.generateChat(r, prompt, "partner_article_rewrite", publication.strPtr("brand_id"), user.ID)
	if err != nil {
		d.logger().Warn("rewrite AI gagal", "err", err)
		d.releaseRewriteClaim(r.Context(), publicationID)
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil yang valid. Silakan coba lagi.")
		return
	}
	parsed, ok := parseRewriteOutput(result.Content)
	if !ok {
		d.releaseRewriteClaim(r.Context(), publicationID)
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil yang valid. Silakan coba lagi.")
		return
	}
	d.auditLog(r.Context(), user.ID, "publication.partner_rewrite_ai", "publication", publicationID)
	writeJSON(w, http.StatusOK, map[string]any{
		"title":              truncateRunes(parsed["title"], 200),
		"content_html":       content.NormalizeArticleHTML(truncateRunes(parsed["content_html"], 200_000)),
		"amount_charged_idr": 0,
		"rewrite_remain":     maxInt(0, rewriteMaxPerPublication-int(claimed)),
	})
}

// publicationChosenAngle: baca kolom `channels` (jsonb) dari taut_profiler_submissions
// lalu cari angle_slug untuk channel bertipe mediaType. Bentuk kolom bisa []any
// hasil decode pgx atau string JSON legacy — keduanya diterima.
func publicationChosenAngle(raw any, mediaType string) string {
	if raw == nil {
		return ""
	}
	entries := []any{}
	switch t := raw.(type) {
	case []any:
		entries = t
	case string:
		var parsed []any
		if json.Unmarshal([]byte(t), &parsed) != nil {
			return ""
		}
		entries = parsed
	default:
		return ""
	}
	for _, entry := range entries {
		ch, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		channel, _ := ch["channel"].(string)
		if channel != mediaType {
			continue
		}
		if slug, _ := ch["angle_slug"].(string); slug != "" {
			return slug
		}
	}
	return ""
}

// rewriteAngleBlock: ambil blok ANGLE pilihan klien untuk kampanye publikasi ini
// (paritas loadTautChosenAnglePrompt di brandflow) — dipakai sebagai sudut
// pandang The Writer saat rewrite. Kosong bila kampanye tanpa pilihan angle.
func (d *Deps) rewriteAngleBlock(ctx context.Context, publication rowMap) string {
	mediaRow, err := d.queryRow(ctx, "SELECT media_type FROM media WHERE id = $1 LIMIT 1", publication.strOr("media_id", ""))
	if err != nil || mediaRow == nil {
		return ""
	}
	mediaType := mediaRow.strOr("media_type", "")
	sub, err := d.queryRow(ctx, `
		SELECT channels FROM taut_profiler_submissions
		WHERE campaign_id = $1 ORDER BY revision DESC LIMIT 1`, publication.strOr("campaign_id", ""))
	if err != nil || sub == nil {
		return ""
	}
	angleSlug := publicationChosenAngle(sub.raw("channels"), mediaType)
	if angleSlug == "" {
		return ""
	}
	row, err := d.queryRow(ctx, `
		SELECT v.prompt
		FROM taut_content_angle_prompt_versions v
		JOIN taut_content_angles a ON a.id = v.content_angle_id
		JOIN taut_media_channels c ON c.id = a.channel_id
		WHERE a.slug = $1 AND c.slug = $2 AND v.status = 'active'
		ORDER BY v.version DESC LIMIT 1`, angleSlug, mediaType)
	if err != nil || row == nil {
		return ""
	}
	prompt := strings.TrimSpace(row.strOr("prompt", ""))
	if prompt == "" {
		return ""
	}
	return "[ANGLE PILIHAN KLIEN — " + mediaType + "/" + angleSlug + "]\n" + truncateRunes(prompt, 16000)
}

// rewriteTopicBlock: topik/bahan writer yang tersimpan per media kampanye —
// konteks fakta yang dipakai The Writer saat menyusun artikel pertama kali.
func (d *Deps) rewriteTopicBlock(ctx context.Context, publication rowMap) string {
	if publication.strOr("campaign_id", "") == "" || publication.strOr("media_id", "") == "" {
		return ""
	}
	row, err := d.queryRow(ctx, `
		SELECT writer_topic FROM taut_campaign_media
		WHERE campaign_id = $1 AND media_id = $2 LIMIT 1`,
		publication.strOr("campaign_id", ""), publication.strOr("media_id", ""))
	if err != nil || row == nil {
		return ""
	}
	topic := strings.TrimSpace(row.strOr("writer_topic", ""))
	if topic == "" {
		return ""
	}
	return "[KONTEKS TOPIK KLIEN DI THE WRITER]\n" + truncateRunes(topic, 6000)
}

// releaseRewriteClaim: kembalikan jatah yang sempat diklaim but generate gagal.
func (d *Deps) releaseRewriteClaim(ctx context.Context, publicationID string) {
	if _, err := d.execRows(ctx,
		"UPDATE publications SET rewrite_ai_count = GREATEST(COALESCE(rewrite_ai_count, 0) - 1, 0) WHERE id = $1",
		publicationID); err != nil {
		d.logger().Warn("rewrite jatah gagal dikembalikan", "publication", publicationID, "err", err)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// buildRewritePrompt: prompt rewrite mitra memakai panel The Writer —
// angle pilihan klien menuntun sudut pandang, bahan writer menjaga fakta.
func buildRewritePrompt(title, contentHTML, angleBlock, topicBlock string) string {
	parts := []string{
		"Kamu The Writer — penulis AI Tauthub yang profesional berbahasa Indonesia.",
		"Tulis ulang artikel berikut agar lebih segar, mengalir, dan bebas plagiarisme.",
		"FAKTA TETAP DIJAGA: nama, angka, kutipan, tautan, dan urutan kejadian pada isi asli tidak boleh berubah.",
		"",
		"Judul saat ini: " + title,
		"",
		"Isi artikel (HTML):",
		contentHTML,
		"",
	}
	if angleBlock != "" {
		parts = append(parts, angleBlock, "", "SUDUT PANDANG: tulisan hasil rewrite WAJIB mengikuti ANGLE PILIHAN KLIEN di atas — pembasa, struktur paragraf, dan penekanan bahasan diarahkan ke sudut itu, seolah artikel ini yang pertama kali ditulis The Writer dengan angle yang dipilih klien.")
	}
	if topicBlock != "" {
		parts = append(parts, "", topicBlock, "Gunakan konteks topik di atas sebagai bahan fakta tambahan bila relevan.")
	}
	parts = append(parts,
		"",
		"Kembalikan JSON valid dengan bentuk: {\"title\": \"...\", \"content_html\": \"...\"}.",
		"Jangan menambahkan penjelasan di luar JSON.",
	)
	return strings.Join(parts, "\n")
}

// parseRewriteOutput: port parseRewriteOutput — strip fence lalu ambil JSON.
func parseRewriteOutput(raw string) (map[string]string, bool) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil, false
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(text[start:end+1]), &parsed) != nil {
		return nil, false
	}
	title, okTitle := parsed["title"].(string)
	contentHTML, okContent := parsed["content_html"].(string)
	if !okTitle || !okContent {
		return nil, false
	}
	return map[string]string{"title": title, "content_html": contentHTML}, true
}

// partnerAvailableIDR: parity partnerAvailableIdr — settled belum ter-settlement
// dikurangi biaya layanan, minimal 0.
func (d *Deps) partnerAvailableIDR(ctx context.Context, partnerID string) float64 {
	row, _ := d.queryRow(ctx, `
		SELECT
		  (COALESCE((SELECT SUM(amount) FROM partner_earnings WHERE partner_id = $1 AND status = 'settled' AND settlement_id IS NULL), 0)
		   - COALESCE((SELECT SUM(amount_idr) FROM partner_service_charges WHERE partner_id = $1), 0))::numeric(14,2) AS net`,
		partnerID)
	net := row.numOr("net")
	if net < 0 {
		return 0
	}
	return net
}
