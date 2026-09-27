package partner

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/content"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
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
	if status != "published" && status != "waiting_partner" {
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
		"content_html":       contentHTML,
		"meta_description":   metaDescription,
		"featured_image_url": featured,
		"wp_status":          firstNonNilString(post["status"], "draft"),
		"edited_at":          edits["edited_at"],
		"source":             "wp_draft",
	}, true
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
		"content_html":       firstNonNilString(edits["content_html"], html),
		"meta_description":   firstNonNilString(edits["meta_description"], doc.raw("meta_description")),
		"featured_image_url": firstNonNilString(edits["featured_image_url"], publication.raw("featured_image_url")),
		"wp_status":          "terbit",
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
	var featuredImage any
	hasFeatured := false
	if v, present := body["featured_image_url"]; present {
		hasFeatured = true
		featuredImage = v
	}
	if !hasTitle && !hasContent && !hasMeta && !hasFeatured {
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

var draftImageMIME = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true, "image/bmp": true,
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
		httpx.Error(w, http.StatusUnprocessableEntity, "Format gambar harus PNG, JPG, WebP, GIF, atau BMP.")
		return
	}
	const maxBytes = 10 * 1024 * 1024
	if header.Size > maxBytes {
		httpx.Error(w, http.StatusUnprocessableEntity, "Ukuran gambar maksimal 10 MB.")
		return
	}
	ext := sanitizeExt(strings.ToLower(header.Filename), "png")
	storageKey := uuid.NewString() + "." + ext
	data, err := readAllLimited(file, maxBytes)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	compressed, compErr := compressImageIfNeeded(data, mime)
	if compErr != nil {
		compressed = data
	}
	if err := d.saveUpload(storageKey, compressed); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "publication.partner_draft_image_uploaded", "publication", publicationID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"url":        "/uploads/" + storageKey,
		"size_bytes": len(compressed),
	})
}

// ---------- POST /v1/partner/publications/{publicationId}/rewrite ----------

const rewriteCostIDR = 1000

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

	available := d.partnerAvailableIDR(r.Context(), partner.ID)
	if available < rewriteCostIDR {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{
			"detail": "Saldo tidak cukup untuk Rewrite (" + formatIDR(available) + " tersedia, butuh " + formatIDR(rewriteCostIDR) +
				"). Saldo Mitra bertambah dari penghasilan ketika klien membeli media.",
			"available_idr": available,
			"required_idr":  rewriteCostIDR,
		})
		return
	}

	prompt := buildRewritePrompt(title, contentHTML)
	result, err := d.generateChat(r, prompt, "partner_article_rewrite", publication.strPtr("brand_id"), user.ID)
	if err != nil {
		d.logger().Warn("rewrite AI gagal", "err", err)
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil yang valid. Silakan coba lagi.")
		return
	}
	parsed, ok := parseRewriteOutput(result.Content)
	if !ok {
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil yang valid. Silakan coba lagi.")
		return
	}
	if err := d.exec(r.Context(), `
		INSERT INTO partner_service_charges (id, partner_id, service, amount_idr, reference_id, reference_type, notes)
		VALUES (gen_random_uuid(), $1, 'article_rewrite_ai', $2, $3, 'publication', $4)`,
		partner.ID, rewriteCostIDR, publicationID, "Rewrite artikel oleh mitra"); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "publication.partner_rewrite_ai", "publication", publicationID)
	writeJSON(w, http.StatusOK, map[string]any{
		"title":              truncateRunes(parsed["title"], 200),
		"content_html":       truncateRunes(parsed["content_html"], 200_000),
		"amount_charged_idr": rewriteCostIDR,
	})
}

// buildRewritePrompt: parity prompt rewrite mitra (instruksi bahasa Indonesia).
func buildRewritePrompt(title, contentHTML string) string {
	return strings.Join([]string{
		"Kamu editor artikel profesional berbahasa Indonesia.",
		"Tulis ulang artikel berikut agar lebih segar, mengalir, dan bebas plagiarisme,",
		"tanpa mengubah fakta, sudut pandang, atau pesan utama.",
		"",
		"Judul saat ini: " + title,
		"",
		"Isi artikel (HTML):",
		contentHTML,
		"",
		"Kembalikan JSON valid dengan bentuk: {\"title\": \"...\", \"content_html\": \"...\"}.",
		"Jangan menambahkan penjelasan di luar JSON.",
	}, "\n")
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
