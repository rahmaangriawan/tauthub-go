package brandflow

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/security"

	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"

	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
	"github.com/rahmaangriawan/tauthub-go/internal/doctext"
)

// taut_extras.go: port endpoint Taut Pintar klien yang belum di-wire —
// agent-status, campaigns/{id}/{articles,files,crawl,gatekeeper,produce,writer-links}.

// ---------- GET /v1/brands/{brandId}/taut/agent-status ----------

// TautAgentStatus: GET /v1/brands/{brandId}/taut/agent-status
func (d *Deps) TautAgentStatus(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	agents := []string{"profiler", "matchmaker", "writer", "writer_mainstream", "writer_blogger", "gatekeeper"}
	statuses := make([]map[string]any, 0, len(agents))
	for _, agent := range agents {
		cfg := d.getAgentConfig(r.Context(), agent)
		var reason any
		if !cfg.Configured {
			switch {
			case !cfg.Enabled:
				reason = "dinonaktifkan oleh admin"
			case cfg.BaseURL == "":
				reason = "base_url kosong"
			case cfg.APIKey == "":
				reason = "api key belum diisi atau kedaluwarsa"
			case cfg.Model == "":
				reason = "model belum dipilih"
			default:
				reason = "konfigurasi belum lengkap"
			}
		}
		statuses = append(statuses, map[string]any{
			"agent": agent, "configured": cfg.Configured, "reason": reason,
		})
	}
	writeNoStore(w, http.StatusOK, map[string]any{"agents": statuses})
}

// ---------- articles/{mediaId} ----------

// TautArticleGet: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/articles/{mediaId}
func (d *Deps) TautArticleGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	mediaID := chiParam(r, "mediaId")
	// Kampanye wajib milik brand pada URL — cegah akses lintas-brand.
	camp, err := d.queryRow(r.Context(),
		"SELECT 1 FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if camp == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	item, err := d.queryRow(r.Context(),
		"SELECT id, article_document_id, featured_image_url, category FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 LIMIT 1",
		campaignID, mediaID, slotParam(r))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if item == nil || !item.has("article_document_id") {
		httpx.Error(w, http.StatusNotFound, "Artikel belum di-generate untuk media ini.")
		return
	}
	documentID := item.strOr("article_document_id", "")
	doc, err := d.queryRow(r.Context(),
		"SELECT title, current_version, meta_description FROM content_documents WHERE id = $1 LIMIT 1", documentID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	ver, err := d.queryRow(r.Context(),
		"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		documentID, docOrVersion(doc))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	title := ""
	metaDescription := any(nil)
	if doc != nil {
		title = doc.strOr("title", "")
		metaDescription = doc.raw("meta_description")
	}
	contentHTML := ""
	if ver != nil {
		contentHTML = ver.strOr("content", "")
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"media_id":           mediaID,
		"title":              title,
		"meta_description":   metaDescription,
		"content_html":       contentHTML,
		"featured_image_url": item.raw("featured_image_url"),
		"category":           item.raw("category"),
	})
}

// TautArticlePut: PUT /v1/brands/{brandId}/taut/campaigns/{campaignId}/articles/{mediaId}
func (d *Deps) TautArticlePut(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	mediaID := chiParam(r, "mediaId")
	camp, err := d.queryRow(r.Context(),
		"SELECT 1 FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if camp == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	item, err := d.queryRow(r.Context(),
		"SELECT id, article_document_id FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 LIMIT 1",
		campaignID, mediaID, slotParam(r))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if item == nil || !item.has("article_document_id") {
		httpx.Error(w, http.StatusNotFound, "Artikel belum di-generate untuk media ini.")
		return
	}
	documentID := item.strOr("article_document_id", "")
	itemID := item.strOr("id", "")
	body := decodeJSON(r)

	title, hasTitle, err := optionalTrimmedString(body, "title", 300)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	contentHTML, hasContent, err := optionalTrimmedString(body, "content_html", 200000)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	featuredImage, hasFeatured, err := optionalTrimmedString(body, "featured_image_url", 500)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	category, hasCategory, err := optionalTrimmedString(body, "category", 120)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	metaDescription, hasMeta, err := optionalTrimmedString(body, "meta_description", 500)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	// Kategori WAJIB dari kategori media yang ditentukan mitra.
	if hasCategory && category != "" {
		mediaRow, err := d.queryRow(r.Context(), `
			SELECT COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = m.id), m.category) AS categories_csv
			FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id WHERE tcm.id = $1 LIMIT 1`, itemID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		// Pemisahan ganda (internal + koma) supaya media dengan kategori lama
		// bergabung koma tetap mengenali kategori yang dipilih klien.
		allowed := []string{}
		if mediaRow != nil {
			allowed = catutil.Split(mediaRow.strOr("categories_csv", ""))
		}
		if !containsString(allowed, category) {
			httpx.Error(w, http.StatusUnprocessableEntity, "Kategori harus dipilih dari kategori media yang ditentukan mitra.")
			return
		}
	}
	if hasTitle {
		slug := slugifyPlain(title)
		slug = truncateString(slug, 200)
		if err := d.exec(r.Context(),
			"UPDATE content_documents SET title = $1, slug = $2, updated_at = NOW() WHERE id = $3", title, slug, documentID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	if hasMeta {
		if err := d.exec(r.Context(),
			"UPDATE content_documents SET meta_description = $1, updated_at = NOW() WHERE id = $2", metaDescription, documentID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	if hasContent {
		doc, err := d.queryRow(r.Context(), "SELECT current_version FROM content_documents WHERE id = $1", documentID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		next := 1
		if doc != nil {
			next = doc.intOr("current_version", 1) + 1
		}
		safeHTML := sanitizeArticleHTMLForStore(stripDisallowedIframesHTML(contentHTML))
		if err := d.exec(r.Context(),
			"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, $3, $4, 'html', 'manual_edit', 'Edit manual Taut Pintar', $5)",
			uuid.NewString(), documentID, next, safeHTML, user.ID); err != nil {
			d.writeErr(w, err)
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE content_documents SET current_version = $1, updated_at = NOW() WHERE id = $2", next, documentID); err != nil {
			d.writeErr(w, err)
			return
		}
		// Simpan manual juga menimpa artikel: versi lama dihapus (tak ada fitur
		// pemulihan versi), supaya naskah lama tidak menumpuk di database.
		if err := d.pruneContentVersions(r.Context(), documentID, next); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	if hasFeatured || hasCategory {
		sets := []string{}
		args := []any{}
		if hasFeatured {
			args = append(args, featuredImage)
			sets = append(sets, "featured_image_url = $"+itoa(len(args)))
		}
		if hasCategory {
			args = append(args, category)
			sets = append(sets, "category = $"+itoa(len(args)))
		}
		args = append(args, itemID)
		if err := d.exec(r.Context(),
			"UPDATE taut_campaign_media SET "+strings.Join(sets, ", ")+" WHERE id = $"+itoa(len(args)), args...); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautArticleUpload: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/articles/{mediaId}/upload
func (d *Deps) TautArticleUpload(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	mediaID := chiParam(r, "mediaId")
	camp, err := d.queryRow(r.Context(),
		"SELECT 1 FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if camp == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	body := decodeJSON(r)

	title, _ := strField(body, "title")
	contentHTML, ok := strField(body, "content_html")
	if !ok || contentHTML == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi artikel tidak boleh kosong.")
		return
	}
	if utf8.RuneCountInString(contentHTML) > 200000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at most 200000 character(s)")
		return
	}
	// Cover, kategori, dan meta desc ikut dikirim UI pada simpan pertama.
	// Sebelumnya jalur ini hanya menyimpan title+content sehingga tiga nilai itu
	// hilang sampai user menekan Simpan lagi (parity kolom dengan TautArticlePut).
	featuredImage, _, err := optionalTrimmedString(body, "featured_image_url", 500)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	category, _, err := optionalTrimmedString(body, "category", 120)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	metaDescription, _, err := optionalTrimmedString(body, "meta_description", 500)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	item, err := d.queryRow(r.Context(),
		"SELECT id, article_document_id FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 AND status = 'selected' LIMIT 1",
		campaignID, mediaID, slotParam(r))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if item == nil {
		httpx.Error(w, http.StatusNotFound, "Media tidak terpilih di kampanye ini.")
		return
	}
	if item.has("article_document_id") {
		httpx.Error(w, http.StatusConflict, "Media ini sudah punya artikel. Gunakan edit untuk mengubahnya.")
		return
	}
	itemID := item.strOr("id", "")

	// Kategori wajib berasal dari direktori kategori media mitra (sama seperti PUT).
	if category != "" {
		mediaRow, err := d.queryRow(r.Context(), `
			SELECT COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = m.id), m.category) AS categories_csv
			FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id WHERE tcm.id = $1 LIMIT 1`, itemID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		allowed := []string{}
		if mediaRow != nil {
			allowed = catutil.Split(mediaRow.strOr("categories_csv", ""))
		}
		if !containsString(allowed, category) {
			httpx.Error(w, http.StatusUnprocessableEntity, "Kategori harus dipilih dari kategori media yang ditentukan mitra.")
			return
		}
	}

	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		trimmedTitle = "Artikel manual"
	}
	slug := slugifyPlain(trimmedTitle)
	if slug == "" {
		slug = "taut-artikel"
	}
	slug = truncateString(slug, 200)
	docID := uuid.NewString()
	jobID := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO content_documents (id, brand_id, job_id, status, title, slug, meta_title, meta_description, current_version, created_by, approved_by, approved_at) VALUES ($1, $2, $3, 'approved', $4, $5, $6, $7, 1, $8, $9, NOW())",
		docID, brandRow.ID, jobID, trimmedTitle, slug, truncateString(trimmedTitle, 255), metaDescription, user.ID, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, 1, $3, 'html', 'manual_upload', 'Unggah manual Taut Pintar', $4)",
		uuid.NewString(), docID, sanitizeArticleHTMLForStore(stripDisallowedIframesHTML(contentHTML)), user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE taut_campaign_media SET article_document_id = $1, article_status = 'draft', featured_image_url = $2, category = $3 WHERE id = $4",
		docID, featuredImage, category, itemID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "document_id": docID})
}

// ---------- files ----------

// TautFilesList: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/files
func (d *Deps) TautFilesList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	camp, err := d.queryRow(r.Context(),
		"SELECT 1 FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if camp == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	files, err := d.queryRows(r.Context(), `
		SELECT id, campaign_id, submission_id, original_filename, mime_type, size_bytes, checksum,
		       parse_status, parse_error, created_at
		FROM taut_campaign_files
		WHERE campaign_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(files))
	for _, f := range files {
		out = append(out, map[string]any{
			"id": f.raw("id"), "campaign_id": f.raw("campaign_id"), "submission_id": f.raw("submission_id"),
			"original_filename": f.raw("original_filename"), "mime_type": f.raw("mime_type"),
			"size_bytes": f.intOr("size_bytes", 0), "checksum": f.raw("checksum"),
			"parse_status": f.raw("parse_status"), "parse_error": f.raw("parse_error"),
			"created_at": f.timePtr("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": out})
}

var acceptedDocMimes = map[string]string{
	"application/pdf": "pdf",
	"text/plain":      "txt",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
}

// TautFilesUpload: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/files
func (d *Deps) TautFilesUpload(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.queryRow(r.Context(),
		"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	file, filename, size, contentType, err := formFile(r, "file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File tidak ditemukan.")
		return
	}
	defer file.Close()
	mime := strings.ToLower(contentType)
	ext, allowed := acceptedDocMimes[mime]
	if !allowed {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tipe file tidak diizinkan. Izinkan: PDF, DOCX, TXT.")
		return
	}
	const maxSize = 10 * 1024 * 1024
	if size > maxSize {
		httpx.Error(w, http.StatusUnprocessableEntity, "Maksimal 10 MB.")
		return
	}
	buffer, err := readAllLimited(file, maxSize)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if mime == "application/pdf" && !strings.HasPrefix(string(buffer), "%PDF") {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi file tidak cocok dengan tipe PDF.")
		return
	}
	if mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		if len(buffer) < 2 || buffer[0] != 0x50 || buffer[1] != 0x4b {
			httpx.Error(w, http.StatusUnprocessableEntity, "Isi file tidak cocok dengan tipe DOCX.")
			return
		}
	}
	sum := sha256.Sum256(buffer)
	checksum := hex.EncodeToString(sum[:])

	// DOCX dengan gambar: bangun juga versi HTML yang mempertahankan gambarnya.
	// Gambar disimpan ke uploads Tauthub, jadi dokumen pindah ke editor "sama
	// persis" (teks + gambar) dan gambarnya ikut terbawa saat distribusi. Untuk
	// dokumen tanpa gambar (atau format lain) hasilnya kosong → alur lama tetap.
	docxHTML := ""
	if mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		if richHTML, n, rErr := d.extractDocxRich(buffer); rErr == nil && n > 0 {
			docxHTML = richHTML
		}
	}

	existing, err := d.queryRow(r.Context(),
		"SELECT id, original_filename, mime_type, size_bytes, parse_status, extracted_text, created_at FROM taut_campaign_files WHERE campaign_id = $1 AND checksum = $2 AND deleted_at IS NULL LIMIT 1",
		campaignID, checksum)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if existing != nil {
		// extracted_text wajib ikut: klien memakai isi dokumen untuk mengisi
		// editor. Tanpa teks, mengunggah ulang berkas yang sama terlihat gagal
		// ("file gagal dibaca") padahal berkasnya sudah tersimpan.
		writeJSON(w, http.StatusOK, map[string]any{
			"file": map[string]any{
				"id": existing.raw("id"), "original_filename": existing.raw("original_filename"),
				"mime_type": existing.raw("mime_type"), "size_bytes": existing.intOr("size_bytes", 0),
				"parse_status":  existing.raw("parse_status"),
				"extracted_text": existing.raw("extracted_text"),
				"created_at":    existing.timePtr("created_at"),
			},
			"deduplicated": true,
		})
		return
	}

	fileID := uuid.NewString()
	storageKey := fileID + "." + ext
	if err := d.saveUpload(storageKey, buffer); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(), `
		INSERT INTO taut_campaign_files (id, campaign_id, original_filename, storage_key, mime_type, size_bytes, checksum, parse_status, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'parsing', $8)`,
		fileID, campaignID, filename, storageKey, mime, len(buffer), checksum, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}

	// Ekstraksi teks server-side: TXT apa adanya, DOCX dari word/document.xml,
	// PDF lewat pembaca PDF (doctext.FromPDF). Gagal parse bukan 500 — berkas
	// tetap tersimpan, statusnya 'failed' beserta alasannya supaya klien tahu.
	extracted, parseErr := extractDocument(buffer, mime)
	if parseErr != nil {
		msg := truncateString(parseErr.Error(), 500)
		if err := d.exec(r.Context(),
			"UPDATE taut_campaign_files SET parse_status = 'failed', parse_error = $1 WHERE id = $2", msg, fileID); err != nil {
			d.writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"detail": "File tersimpan tetapi gagal di-parse: " + msg,
			"file":   map[string]any{"id": fileID, "parse_status": "failed"},
		})
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE taut_campaign_files SET extracted_text = $1, parse_status = 'ready', parse_error = NULL WHERE id = $2",
		truncateString(extracted, 100000), fileID); err != nil {
		d.writeErr(w, err)
		return
	}
	fileRow, err := d.queryRow(r.Context(),
		"SELECT id, original_filename, mime_type, size_bytes, parse_status, extracted_text, created_at FROM taut_campaign_files WHERE id = $1 LIMIT 1", fileID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	fileOut := map[string]any{
		"id": fileRow.raw("id"), "original_filename": fileRow.raw("original_filename"),
		"mime_type": fileRow.raw("mime_type"), "size_bytes": fileRow.intOr("size_bytes", 0),
		"parse_status": fileRow.raw("parse_status"), "extracted_text": fileRow.raw("extracted_text"),
		"created_at": fileRow.timePtr("created_at"),
	}
	// extracted_html = teks + gambar yang sudah dihosting Tauthub (khusus DOCX
	// bergambar). Klien memakainya apa adanya supaya dokumen sama persis.
	if docxHTML != "" {
		fileOut["extracted_html"] = docxHTML
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"file":         fileOut,
		"deduplicated": false,
	})
}

// extractDocument: ekstraksi teks bahan referensi (port parseDocument).
// TXT native; DOCX dibongkar dengan stdlib (arsip ZIP + XML Office Open XML);
// PDF lewat library pihak ketiga (internal/doctext).
func extractDocument(buffer []byte, mime string) (string, error) {
	if mime == "text/plain" || mime == "text/csv" {
		return truncateString(string(buffer), 100000), nil
	}
	if mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		return extractDocxText(buffer)
	}
	if mime == "application/pdf" {
		return doctext.FromPDF(buffer)
	}
	return "", &bfStatusError{
		Status:  http.StatusUnprocessableEntity,
		Message: "Parser dokumen " + mime + " belum tersedia di backend ini. Unggah DOCX atau TXT, atau tempel isi artikel secara manual.",
	}
}

// extractDocxText: ambil teks dari word/document.xml di dalam arsip DOCX.
// DOCX = ZIP + XML Office Open XML: paragraf = <w:p>, teks = <w:t>, dengan
// <w:br/> ganti baris dan <w:tab/> tab — semuanya cukup ditangani stdlib
// (archive/zip + encoding/xml) tanpa dependensi pihak ketiga.
func extractDocxText(buffer []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(buffer), int64(len(buffer)))
	if err != nil {
		return "", &bfStatusError{
			Status:  http.StatusUnprocessableEntity,
			Message: "DOCX tidak dapat dibuka — arsipnya rusak atau bukan berkas Office yang valid.",
		}
	}
	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", &bfStatusError{
			Status:  http.StatusUnprocessableEntity,
			Message: "DOCX tidak berisi word/document.xml — kemungkinan bukan dokumen Word.",
		}
	}
	rc, err := docFile.Open()
	if err != nil {
		return "", &bfStatusError{
			Status:  http.StatusUnprocessableEntity,
			Message: "Isi DOCX tidak dapat dibaca.",
		}
	}
	defer rc.Close()
	// Batas pengaman terhadap arsip jumbo: XML isi dibaca maksimal 32 MB.
	xmlBytes, err := io.ReadAll(io.LimitReader(rc, 32*1024*1024))
	if err != nil {
		return "", &bfStatusError{
			Status:  http.StatusUnprocessableEntity,
			Message: "Isi DOCX tidak dapat dibaca.",
		}
	}

	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	dec.Strict = false
	var sb strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", &bfStatusError{
				Status:  http.StatusUnprocessableEntity,
				Message: "Struktur XML di dalam DOCX tidak valid.",
			}
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "br":
				sb.WriteString("\n")
			case "tab":
				sb.WriteString("\t")
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				// Akhir paragraf → pemisah baris (tanpa baris kosong ganda).
				if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
			}
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", &bfStatusError{
			Status:  http.StatusUnprocessableEntity,
			Message: "Tidak ada teks yang bisa diekstrak dari DOCX ini.",
		}
	}
	return truncateString(text, 100000), nil
}

// TautFileDelete: DELETE /v1/brands/{brandId}/taut/campaigns/{campaignId}/files/{fileId}
func (d *Deps) TautFileDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	fileID := chiParam(r, "fileId")
	campaignID := chiParam(r, "campaignId")
	camp, err := d.queryRow(r.Context(),
		"SELECT 1 FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if camp == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id FROM taut_campaign_files WHERE id = $1 AND campaign_id = $2 AND deleted_at IS NULL LIMIT 1", fileID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "File tidak ditemukan.")
		return
	}
	if err := d.exec(r.Context(), "UPDATE taut_campaign_files SET deleted_at = NOW() WHERE id = $1", fileID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- crawl ----------

// TautCrawl: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/crawl
func (d *Deps) TautCrawl(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	_, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	body := decodeJSON(r)
	rawURL, ok := strField(body, "url")
	if !ok || rawURL == "" || len(rawURL) > 500 || !isHTTPURL(rawURL) {
		httpx.Error(w, http.StatusUnprocessableEntity, "URL tidak valid")
		return
	}

	response, err := securityGet(r, rawURL, map[string]string{
		"user-agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"accept-language": "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7",
	})
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Website belum dapat dibaca otomatis ("+err.Error()+"). Anda tetap dapat melanjutkan dengan mengisi informasi secara manual.")
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		hint := "HTTP " + itoa(response.StatusCode)
		if response.StatusCode == 403 || response.StatusCode == 401 {
			hint = "akses dibatasi/dilindungi"
		} else if response.StatusCode == 404 {
			hint = "halaman tidak ditemukan"
		}
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Website belum dapat dibaca otomatis ("+hint+"). Anda tetap dapat melanjutkan dengan mengisi informasi secara manual.")
		return
	}
	const maxBytes = 4_000_000
	const maxChars = 16_000
	raw, err := readAllLimited(response.Body, maxBytes)
	if err != nil || len(raw) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Website belum dapat dibaca otomatis (konten kosong). Anda tetap dapat melanjutkan dengan mengisi informasi secara manual.")
		return
	}
	text := truncateString(stripHTMLToText(string(raw)), maxChars)
	if utf8.RuneCountInString(text) < 80 {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Website belum dapat dibaca otomatis (konten kosong). Anda tetap dapat melanjutkan dengan mengisi informasi secara manual.")
		return
	}

	angles, err := d.activeTautAngles(r.Context())
	if err != nil {
		d.writeErr(w, err)
		return
	}
	prompt := buildCrawlPrompt(text, angles, rawURL)
	extractSystem := "Kamu adalah ekstraktor fakta dari website. Balas HANYA satu objek JSON persis sesuai format yang diminta user prompt: tanpa penjelasan, tanpa markdown, tanpa membungkusnya dalam key lain."
	result, err := d.callAgentLLM(r.Context(), "profiler", prompt, true, extractSystem, 180000)
	if err != nil {
		var ance *agentNotConfiguredError
		if asAgentNotConfigured(err, &ance) {
			httpx.Error(w, http.StatusConflict, "Agent Profiler belum dikonfigurasi. Hubungi super admin untuk mengatur di Taut Agents.")
			return
		}
		httpx.Error(w, http.StatusBadGateway, "Website sudah terbaca, tetapi ekstraksi AI gagal. Coba lagi atau isi manual.")
		return
	}
	parsed := extractJSONObject(result.Content)
	if parsed == nil {
		httpx.Error(w, http.StatusBadGateway, "Website sudah terbaca, tetapi ekstraksi AI gagal. Coba lagi atau isi manual.")
		return
	}
	extracted := normalizeCrawlExtraction(parsed, rawURL, angles)

	// Simpan ke draft submission (crawl_result) sebagai konteks.
	sub, err := d.queryRow(r.Context(),
		"SELECT id FROM taut_profiler_submissions WHERE campaign_id = $1 AND status = 'draft' ORDER BY revision DESC LIMIT 1", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if sub != nil {
		if err := d.exec(r.Context(),
			"UPDATE taut_profiler_submissions SET crawl_result = $1::jsonb, updated_at = NOW() WHERE id = $2",
			marshalJSON(extracted), sub.strOr("id", "")); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "extracted": extracted})
}

// ---------- gatekeeper ----------

// TautGatekeeperAudits: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/audits
func (d *Deps) TautGatekeeperAudits(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.queryRow(r.Context(),
		"SELECT id, name, taut_status, taut_entry_mode FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	items, err := d.queryRows(r.Context(), `
		SELECT tcm.media_id, tcm.slot, m.name, m.media_type, m.website_url,
		       COALESCE(tcm.price, m.price, 0) AS price,
		       m.max_links, sm.da, sm.spam_score, m.traffic_estimate,
		       p.scheduled_at, tcm.article_document_id, d.title AS article_title, d.updated_at AS article_updated_at,
		       p.id AS pub_id, p.status AS pub_status, p.published_url AS pub_url, p.client_approved_at AS pub_client_approved_at,
		       p.featured_image_url AS pub_featured_image_url, tcm.featured_image_url AS media_featured_image_url
		FROM taut_campaign_media tcm
		JOIN media m ON m.id = tcm.media_id
		LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
		LEFT JOIN content_documents d ON d.id = tcm.article_document_id
		LEFT JOIN publications p ON p.document_id = tcm.article_document_id AND p.media_id = tcm.media_id
		WHERE tcm.campaign_id = $1 AND tcm.status = 'selected'
		ORDER BY tcm.match_rank ASC NULLS LAST, m.name ASC, tcm.slot ASC`, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	audits, err := d.listGatekeeperAudits(r.Context(), campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	rate := d.getTcConfig(r.Context()).RateIDR

	// Jumlah analisis QC (semua revisi) per media — dipakai The Writer untuk
	// menampilkan badge "sudah berapa kali dianalisis".
	auditCounts := map[string]int{}
	for i := range audits {
		auditCounts[placementKey(audits[i].MediaID, audits[i].Slot)]++
	}

	docIDs := []string{}
	seenDoc := map[string]bool{}
	for _, item := range items {
		id := item.strOr("article_document_id", "")
		if id != "" && !seenDoc[id] {
			seenDoc[id] = true
			docIDs = append(docIDs, id)
		}
	}
	contentByDoc := map[string]string{}
	if len(docIDs) > 0 {
		versionRows, err := d.queryRows(r.Context(),
			"SELECT v.document_id, v.content FROM content_versions v JOIN content_documents d ON d.id = v.document_id AND d.current_version = v.version WHERE v.document_id = ANY($1)",
			docIDs)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		for _, v := range versionRows {
			contentByDoc[v.strOr("document_id", "")] = v.strOr("content", "")
		}
	}

	mediaOut := make([]map[string]any, 0, len(items))
	canDistribute := len(items) > 0
	totalCostIDR := 0
	totalCostTC := 0
	for _, item := range items {
		price := int(item.numOr("price"))
		if price < 0 {
			price = 0
		}
		totalCostIDR += price
		safeRate := rate
		if safeRate < 1 {
			safeRate = 1
		}
		totalCostTC += (price + safeRate/2) / safeRate

		mediaID := item.strOr("media_id", "")
		slot := item.intOr("slot", 1)
		var latest *gatekeeperAuditRow
		for i := range audits {
			if audits[i].MediaID == mediaID && audits[i].Slot == slot {
				latest = &audits[i]
				break
			}
		}
		checks := map[string]any{}
		if latest != nil {
			checks = latest.Checks
		}
		safety := asCheckStatus(checks["contentSafety"])
		linkStatus := asCheckStatus(checks["linkHealth"])
		if latest == nil || safety != "passed" || linkStatus != "passed" {
			canDistribute = false
		}
		auditStatus := "pending"
		if latest != nil && asCheckStatusString(latest.Status) != "" {
			auditStatus = latest.Status
		}
		unsafeURLs := unsafeURLsFromChecks(checks)
		articleHTML := contentByDoc[item.strOr("article_document_id", "")]
		anchorCount, rels, linkKinds := anchorReport(articleHTML)

		contentSafety := checks["contentSafety"]
		if contentSafety == nil {
			contentSafety = map[string]any{"status": "pending"}
		}
		aiOverview := checks["aiOverview"]
		if aiOverview == nil {
			aiOverview = map[string]any{"status": "pending"}
		}
		linkHealth := checks["linkHealth"]
		if linkHealth == nil {
			linkHealth = map[string]any{"status": "pending"}
		}
		brandVoice := checks["brandVoice"]
		if brandVoice == nil {
			brandVoice = map[string]any{"status": "pending"}
		}

		// Cover yang dilihat klien di modal revisi: yang sudah terpasang di
		// publikasi menang, kalau belum ada pakai cover artikel media itu.
		coverURL := item.strOr("pub_featured_image_url", "")
		if coverURL == "" {
			coverURL = item.strOr("media_featured_image_url", "")
		}

		mediaOut = append(mediaOut, map[string]any{
			"media_id": item.raw("media_id"), "slot": slot, "name": item.raw("name"), "media_type": item.raw("media_type"),
			"website_url":        item.raw("website_url"),
			"featured_image_url": coverURL,
			"price_idr":          price,
			"da":                 numOrNil(item, "da"),
			"ss":                 numOrNil(item, "spam_score"),
			"est_traffic":        numOrNil(item, "traffic_estimate"),
			"link_capacity":      numOrNil(item, "max_links"),
			"scheduled_at":       item.timePtr("scheduled_at"),
			"article": map[string]any{
				"title": item.strOr("article_title", ""), "updated_at": item.timePtr("article_updated_at"),
				"links": map[string]any{"count": anchorCount, "rels": rels, "kinds": linkKinds},
			},
			"publish":            map[string]any{"status": item.raw("pub_status"), "url": item.raw("pub_url")},
			"publication_id":     item.raw("pub_id"),
			"document_id":        item.raw("article_document_id"),
			"client_approved_at": item.timePtr("pub_client_approved_at"),
			"audit": map[string]any{
				"status": auditStatus,
				"count":  auditCounts[placementKey(mediaID, slot)],
				"checks": map[string]any{
					"aiOverview":    aiOverview,
					"contentSafety": contentSafety,
					"linkHealth":    withUnsafeURLs(linkHealth, unsafeURLs),
					"brandVoice":    brandVoice,
				},
			},
		})
	}

	tokensRow, err := d.queryRow(r.Context(),
		"SELECT COALESCE(SUM(token_input + token_output), 0) AS total FROM taut_agent_runs WHERE campaign_id = $1", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	tokensUsed := 0
	if tokensRow != nil {
		tokensUsed = tokensRow.intOr("total", 0)
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"campaign": map[string]any{
			"id": campaignID, "name": campaign.strOr("name", ""), "taut_status": campaign.raw("taut_status"),
			"entry_mode": campaign.raw("taut_entry_mode"),
		},
		"media":         mediaOut,
		"canDistribute": canDistribute,
		"totalCostIdr":  totalCostIDR,
		"totalCostTc":   totalCostTC,
		"tokens":        map[string]any{"used": tokensUsed},
		"rateIdr":       rate,
	})
}

// TautGatekeeperAuditAll: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/audit-all
func (d *Deps) TautGatekeeperAuditAll(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	body := decodeJSON(r)
	requested, _ := stringSliceField(body, "mediaIds")
	if len(requested) > 50 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Array must contain at most 50 element(s)")
		return
	}
	// slots opsional: batasi pemindaian ke nomor artikel tertentu pada media
	// yang diminta. Kosong = semua slot (perilaku lama).
	requestedSlots := []int{}
	if raw, ok := body["slots"].([]any); ok {
		if len(raw) > 50 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Array must contain at most 50 element(s)")
			return
		}
		for _, v := range raw {
			n, isNum := v.(float64)
			if !isNum || n < 1 {
				httpx.Error(w, http.StatusUnprocessableEntity, "Invalid slot")
				return
			}
			requestedSlots = append(requestedSlots, int(n))
		}
	}
	selected, err := d.queryRows(r.Context(),
		"SELECT media_id, slot, article_status FROM taut_campaign_media WHERE campaign_id = $1 AND status = 'selected' ORDER BY created_at", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if len(selected) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Belum ada media terpilih untuk diaudit.")
		return
	}
	scope := selected
	if len(requested) > 0 {
		scope = []bsRow{}
		for _, row := range selected {
			if containsString(requested, row.strOr("media_id", "")) {
				scope = append(scope, row)
			}
		}
	}
	if len(scope) == 0 {
		httpx.Error(w, http.StatusNotFound, "Media yang diminta bukan bagian kampanye ini.")
		return
	}

	// Idempoten bila semua media sudah lulus hard gate DAN ready (tanpa subset).
	if len(requested) == 0 {
		audits, err := d.listGatekeeperAudits(r.Context(), campaignID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		allReady := true
		for _, row := range scope {
			mediaID := row.strOr("media_id", "")
			slot := row.intOr("slot", 1)
			var latest *gatekeeperAuditRow
			for i := range audits {
				// Audit menilai isi satu artikel, jadi pencocokannya harus
				// (media, slot) — bukan media saja.
				if audits[i].MediaID == mediaID && audits[i].Slot == slot {
					latest = &audits[i]
					break
				}
			}
			// Audit lama tanpa kunci aiOverview = skema sebelum AI Overview
			// Checker ada → anggap basi supaya artikel lama ikut dipindai sekali.
			if row.strOr("article_status", "") != "ready" || latest == nil ||
				!hardGatesPassed(latest.Checks) || latest.Checks["aiOverview"] == nil {
				allReady = false
				break
			}
		}
		if allReady {
			writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "alreadyAudited": true, "mediaCount": len(scope)})
			return
		}
	}

	results, err := d.runGatekeeper(r.Context(), campaignID, brandRow.ID, user.ID, requested, requestedSlots)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.advanceTautStatus(r.Context(), campaignID, "review"); err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusAccepted, map[string]any{"ok": true, "mediaCount": len(scope), "items": results})
}

// TautGatekeeperDistribute: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/distribute
func (d *Deps) TautGatekeeperDistribute(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.queryRow(r.Context(),
		"SELECT id, taut_status FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	body := decodeJSON(r)

	gate, err := d.checkCanDistribute(r.Context(), campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !gate.Can {
		httpx.Error(w, http.StatusUnprocessableEntity, "Masih ada artikel yang belum lolos QC.")
		return
	}

	ready, err := d.queryRows(r.Context(),
		"SELECT media_id, slot FROM taut_campaign_media WHERE campaign_id = $1 AND status = 'selected' AND article_status = 'ready' AND article_document_id IS NOT NULL ORDER BY media_id, slot", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	readyRefs := make([]tautPlacementRef, 0, len(ready))
	readyKeys := map[string]bool{}
	for _, row := range ready {
		ref := tautPlacementRef{MediaID: row.strOr("media_id", ""), Slot: row.intOr("slot", 1)}
		readyRefs = append(readyRefs, ref)
		readyKeys[placementKey(ref.MediaID, ref.Slot)] = true
	}
	selectedMedia, err := d.queryRows(r.Context(), `
		SELECT tcm.media_id, tcm.slot, m.name, tcm.article_status
		FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id
		WHERE tcm.campaign_id = $1 AND tcm.status = 'selected'`, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// Hitung berapa slot per media supaya pesan "belum ready" bisa menyebut
	// artikel ke berapa pada domain itu.
	slotsPerMedia := map[string]int{}
	for _, row := range selectedMedia {
		slotsPerMedia[row.strOr("media_id", "")]++
	}
	notReady := []string{}
	for _, row := range selectedMedia {
		mediaID := row.strOr("media_id", "")
		if readyKeys[placementKey(mediaID, row.intOr("slot", 1))] {
			continue
		}
		name := row.strOr("name", "")
		if name == "" {
			if len(mediaID) > 8 {
				name = mediaID[:8]
			} else {
				name = mediaID
			}
		}
		if slotsPerMedia[mediaID] > 1 {
			name = name + " (artikel " + itoa(row.intOr("slot", 1)) + ")"
		}
		notReady = append(notReady, name+" ("+row.strOr("article_status", "")+")")
	}
	if len(notReady) > 0 {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Belum bisa distribusi: "+strings.Join(notReady, ", ")+". Jalankan audit ulang (atau perbaiki artikel) sampai semua media berstatus ready.")
		return
	}

	if err := d.advanceTautStatus(r.Context(), campaignID, "scheduled"); err != nil {
		d.writeErr(w, err)
		return
	}
	timeline, err := timelineFromBody(body, readyRefs)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	created, err := d.createTautPublications(r.Context(), campaignID, brandRow.ID, user.ID, timeline)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publishedCount := 0
	for _, c := range created {
		if c.Status != "failed" {
			publishedCount++
		}
	}
	if len(created) == 0 || publishedCount == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Belum ada artikel berstatus ready untuk diterbitkan.")
		return
	}

	targets := []string{}
	for _, c := range created {
		if c.PublicationID != "" {
			targets = append(targets, c.PublicationID)
		}
	}
	perPublication := splitTcAcrossPublications(gate.TotalCostTC, len(targets))
	reserved := []string{}
	for i, pubID := range targets {
		tc := 0
		if i < len(perPublication) {
			tc = perPublication[i]
		}
		if tc <= 0 {
			continue
		}
		hasReservation, err := d.hasCreditReservation(r.Context(), brandRow.ID, pubID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if hasReservation {
			continue
		}
		applied, err := d.reserveCredit(r.Context(), brandRow.ID, pubID, tc, user.ID)
		if err != nil {
			var ic *insufficientCreditsError
			if asInsufficient(err, &ic) {
				for _, done := range reserved {
					d.releaseCredit(r.Context(), brandRow.ID, done, user.ID)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				_, _ = w.Write([]byte(`{"code":"insufficient_credits","needed":` + itoa(ic.Needed) + `,"available":` + itoa(ic.Available) + `}`))
				return
			}
			d.writeErr(w, err)
			return
		}
		if applied > 0 {
			reserved = append(reserved, pubID)
		}
	}

	if err := d.exec(r.Context(),
		"UPDATE campaigns SET timeline_plan = $1::jsonb, taut_status = 'submitted', updated_at = NOW() WHERE id = $2 AND brand_id = $3",
		marshalJSON(timeline), campaignID, brandRow.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "taut_status": "scheduled", "publications": publishedCount})
}

func hardGatesPassed(checks map[string]any) bool {
	return asCheckStatus(checks["contentSafety"]) == "passed" && asCheckStatus(checks["linkHealth"]) == "passed"
}

// CheckCanDistribute result.
type canDistributeResult struct {
	Can          bool
	TotalCostIDR int
	TotalCostTC  int
	MediaCount   int
}

// checkCanDistribute: server-side gate — setiap slot artikel punya audit hard
// gate passed, dan total biaya dihitung per slot (4 artikel satu domain = 4x).
func (d *Deps) checkCanDistribute(ctx context.Context, campaignID string) (canDistributeResult, error) {
	items, err := d.queryRows(ctx, `
		SELECT tcm.media_id, tcm.slot, COALESCE(tcm.price, m.price, 0) AS price
		FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id
		WHERE tcm.campaign_id = $1 AND tcm.status = 'selected'`, campaignID)
	if err != nil {
		return canDistributeResult{}, err
	}
	if len(items) == 0 {
		return canDistributeResult{Can: false}, nil
	}
	audits, err := d.listGatekeeperAudits(ctx, campaignID)
	if err != nil {
		return canDistributeResult{}, err
	}
	rate := d.getTcConfig(ctx).RateIDR
	if rate < 1 {
		rate = 1
	}
	totalIDR := 0
	totalTC := 0
	for _, item := range items {
		price := int(item.numOr("price"))
		if price < 0 {
			price = 0
		}
		totalIDR += price
		totalTC += (price + rate/2) / rate
	}
	can := true
	for _, item := range items {
		mediaID := item.strOr("media_id", "")
		slot := item.intOr("slot", 1)
		var latest *gatekeeperAuditRow
		for i := range audits {
			if audits[i].MediaID == mediaID && audits[i].Slot == slot {
				latest = &audits[i]
				break
			}
		}
		if latest == nil || asCheckStatus(latest.Checks["contentSafety"]) != "passed" || asCheckStatus(latest.Checks["linkHealth"]) != "passed" {
			can = false
			break
		}
	}
	return canDistributeResult{Can: can, TotalCostIDR: totalIDR, TotalCostTC: totalTC, MediaCount: len(items)}, nil
}

// hasCreditReservation: idempotensi reservasi per publikasi.
func (d *Deps) hasCreditReservation(ctx context.Context, brandID, publicationID string) (bool, error) {
	row, err := d.queryRow(ctx,
		"SELECT 1 AS x FROM credit_ledger_entries WHERE brand_id = $1 AND ref_id = $2 AND ref_type = 'reserve' LIMIT 1",
		brandID, publicationID)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

// ---------- produce ----------

// TautProduce: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/produce
func (d *Deps) TautProduce(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	results, err := d.runTautWriter(r.Context(), campaignID, brandRow.ID, user.ID)
	if err != nil {
		var ic *insufficientCreditsError
		if asInsufficient(err, &ic) {
			httpx.Error(w, http.StatusPaymentRequired,
				"Saldo token tidak cukup untuk produksi (butuh "+itoa(ic.Needed)+", tersedia "+itoa(ic.Available)+").")
			return
		}
		d.writeErr(w, err)
		return
	}
	if err := d.advanceTautStatus(r.Context(), campaignID, "production"); err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"items": results})
}

func decodeTautWriterOptions(r *http.Request) (tautWriterOptions, error) {
	var opts tautWriterOptions
	if r.ContentLength == 0 {
		return opts, nil
	}
	var body struct {
		Topic       string                `json:"topic"`
		Links       []tautWriterLinkInput `json:"links"`
		ArticleType string                `json:"article_type"`
		AngleSlug   json.RawMessage       `json:"angle_slug"`
		Gaya        string                `json:"gaya"`
		Proper      struct {
			Fakta      string            `json:"fakta"`
			Dateline   string            `json:"dateline"`
			Narasumber string            `json:"narasumber"`
			Kutipan    string            `json:"kutipan"`
			Quotes     []tautProperQuote `json:"quotes"`
			Fokus      string            `json:"fokus"`
			BriefID    string            `json:"brief_id"`
		} `json:"proper"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 65536))
	if err := decoder.Decode(&body); err != nil {
		if err == io.EOF {
			return opts, nil
		}
		return opts, errBadRequest("Body JSON tidak valid.")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return opts, errBadRequest("Body JSON tidak valid.")
	}
	opts.Topic = truncateString(strings.TrimSpace(body.Topic), 2000)
	opts.Gaya = strings.TrimSpace(body.Gaya)
	opts.ArticleType = truncateString(strings.TrimSpace(body.ArticleType), 50)
	if len(body.AngleSlug) > 0 {
		var slug string
		if err := json.Unmarshal(body.AngleSlug, &slug); err != nil || strings.TrimSpace(slug) == "" {
			return opts, errUnprocessable("angle_slug wajib berupa string yang tidak kosong.")
		}
		slug = strings.TrimSpace(slug)
		opts.AngleSlug = &slug
	}
	// Gaya "proper": bawa isian tab Yang Proper sebagai konteks strict
	// grounding — fakta terkunci + memori Brief Copilot + kutipan resmi.
	if strings.TrimSpace(body.Gaya) == "proper" {
		p := &tautProperInput{
			Fakta:      truncateString(strings.TrimSpace(body.Proper.Fakta), 20000),
			Dateline:   truncateString(strings.TrimSpace(body.Proper.Dateline), 100),
			Narasumber: truncateString(strings.TrimSpace(body.Proper.Narasumber), 200),
			Kutipan:    truncateString(strings.TrimSpace(body.Proper.Kutipan), 4000),
			Fokus:      truncateString(strings.TrimSpace(body.Proper.Fokus), 4000),
			BriefID:    truncateString(strings.TrimSpace(body.Proper.BriefID), 64),
		}
		for _, q := range body.Proper.Quotes {
			p.Quotes = append(p.Quotes, tautProperQuote{
				Dateline:   truncateString(strings.TrimSpace(q.Dateline), 100),
				Narasumber: truncateString(strings.TrimSpace(q.Narasumber), 200),
				Kutipan:    truncateString(strings.TrimSpace(q.Kutipan), 4000),
			})
		}
		if len(p.Quotes) > 0 {
			p.Dateline = p.Quotes[0].Dateline
			p.Narasumber = p.Quotes[0].Narasumber
			p.Kutipan = p.Quotes[0].Kutipan
		}
		if err := p.validate(); err != nil {
			return opts, err
		}
		opts.Proper = p
	}
	links, err := normalizeTautWriterLinks(body.Links)
	opts.Links = links
	return opts, err
}

// TautProduceOne: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/produce/{mediaId}
func (d *Deps) TautProduceOne(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	mediaID := chiParam(r, "mediaId")
	opts, err := decodeTautWriterOptions(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	result, err := d.generateTautWriterArticle(r.Context(), campaignID, brandRow.ID, user.ID, mediaID, slotParam(r), opts)
	if err != nil {
		var ic *insufficientCreditsError
		if asInsufficient(err, &ic) {
			httpx.Error(w, http.StatusPaymentRequired,
				"Saldo kredit tidak cukup untuk generate (butuh "+itoa(ic.Needed)+", tersedia "+itoa(ic.Available)+").")
			return
		}
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, result)
}

// TautProduceOneStream: POST .../produce/{mediaId}/stream — versi SSE dari
// TautProduceOne. Event: delta (tambahan html parsial, escape sudah di-decode),
// done (payload sama dengan produce biasa), error (message + status HTTP).
// Semua cek akses/kredit tetap jalan; hasil akhir tetap lewat finalize yang
// sama, jadi versi stream dan non-stream menghasilkan artikel identik.
func (d *Deps) TautProduceOneStream(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	mediaID := chiParam(r, "mediaId")
	opts, err := decodeTautWriterOptions(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	flusher, canFlush := w.(http.Flusher)
	started := false
	sendEvent := func(event string, payload any) {
		if !canFlush {
			return
		}
		if !started {
			h := w.Header()
			h.Set("Content-Type", "text/event-stream")
			h.Set("Cache-Control", "no-cache")
			h.Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			started = true
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonStringify(payload))
		flusher.Flush()
	}

	opts.StreamHTML = func(partial string) {
		sendEvent("delta", map[string]any{"append": partial})
	}
	result, err := d.generateTautWriterArticle(r.Context(), campaignID, brandRow.ID, user.ID, mediaID, slotParam(r), opts)
	if err != nil {
		var ic *insufficientCreditsError
		if asInsufficient(err, &ic) {
			message := "Saldo kredit tidak cukup untuk generate (butuh " + itoa(ic.Needed) + ", tersedia " + itoa(ic.Available) + ")."
			if !started {
				httpx.Error(w, http.StatusPaymentRequired, message)
			} else {
				sendEvent("error", map[string]any{"status": http.StatusPaymentRequired, "message": message})
			}
			return
		}
		if !started {
			d.writeErr(w, err)
		} else {
			sendEvent("error", map[string]any{"status": http.StatusInternalServerError, "message": err.Error()})
		}
		return
	}
	// Tanpa flusher (pratinjau streaming tetap terkirim bila bisa): tulis hasil
	// sebagai JSON biasa agar klien tetap mendapat artikel.
	if !canFlush {
		writeNoStore(w, http.StatusOK, result)
		return
	}
	sendEvent("done", result)
}

// ---------- writer-links ----------

// TautWriterLinks: GET / PUT /v1/brands/{brandId}/taut/campaigns/{campaignId}/writer-links
func (d *Deps) TautWriterLinks(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.queryRow(r.Context(), `
		SELECT id, tracking_mode, primary_url, primary_anchor, backup_url, backup_anchor
		FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1`, campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	if r.Method == http.MethodGet {
		primaryCode, backupCode := d.getTautWriterShortLinks(r.Context(), campaignID)
		shortDomain := d.getShortLinkDomain(r.Context())
		writeNoStore(w, http.StatusOK, map[string]any{
			"tracking_mode":  campaign.strOr("tracking_mode", "direct"),
			"primary_url":    campaign.raw("primary_url"),
			"primary_anchor": campaign.raw("primary_anchor"),
			"backup_url":     campaign.raw("backup_url"),
			"backup_anchor":  campaign.raw("backup_anchor"),
			"short_domain":   shortDomain,
			"primary_code":   primaryCode,
			"backup_code":    backupCode,
		})
		return
	}

	body := decodeJSON(r)
	trackingMode, ok := strField(body, "tracking_mode")
	if !ok || (trackingMode != "direct" && trackingMode != "tauthub") {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid option: expected one of \"direct\"|\"tauthub\"")
		return
	}
	primaryURL, ok := strField(body, "primary_url")
	if !ok || !isHTTPURL(primaryURL) || len(primaryURL) > 2000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Target URL utama tidak valid.")
		return
	}
	primaryAnchor, ok := strField(body, "primary_anchor")
	if !ok || primaryAnchor == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Anchor teks induk wajib diisi.")
		return
	}
	if utf8.RuneCountInString(primaryAnchor) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at most 120 character(s)")
		return
	}
	backupURL, _ := strField(body, "backup_url")
	if backupURL != "" && (!isHTTPURL(backupURL) || len(backupURL) > 2000) {
		httpx.Error(w, http.StatusUnprocessableEntity, "URL cadangan tidak valid.")
		return
	}
	backupAnchor, _ := strField(body, "backup_anchor")
	if utf8.RuneCountInString(backupAnchor) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at most 120 character(s)")
		return
	}
	if (backupURL != "" && backupAnchor == "") || (backupURL == "" && backupAnchor != "") {
		httpx.Error(w, http.StatusUnprocessableEntity, "URL dan anchor cadangan harus diisi berpasangan.")
		return
	}

	if err := d.exec(r.Context(), `
		UPDATE campaigns SET
		  tracking_mode = $1, primary_url = $2, primary_anchor = $3,
		  backup_url = $4, backup_anchor = $5, updated_at = NOW()
		WHERE id = $6`,
		trackingMode, primaryURL, primaryAnchor, anyOrNil(backupURL), anyOrNil(backupAnchor), campaignID); err != nil {
		d.writeErr(w, err)
		return
	}

	var primaryCode, backupCode any
	if trackingMode == "tauthub" {
		primary, err := d.upsertWriterShortLink(r.Context(), brandRow.ID, campaignID, "writer_primary", primaryURL, user.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		primaryCode = primary
		if backupURL != "" {
			backup, err := d.upsertWriterShortLink(r.Context(), brandRow.ID, campaignID, "writer_backup", backupURL, user.ID)
			if err != nil {
				d.writeErr(w, err)
				return
			}
			backupCode = backup
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "tracking_mode": trackingMode, "primary_url": primaryURL,
		"primary_anchor": primaryAnchor, "backup_url": anyOrNil(backupURL), "backup_anchor": anyOrNil(backupAnchor),
		"primary_code": primaryCode, "backup_code": backupCode,
		"short_domain": d.getShortLinkDomain(r.Context()),
	})
}

// ---------- helper ----------

func docOrVersion(doc bsRow) int {
	if doc == nil {
		return 1
	}
	return doc.intOr("current_version", 1)
}

// optionalTrimmedString: field opsional string ter-trim; hadir=nil→false.
func optionalTrimmedString(body map[string]any, key string, max int) (string, bool, error) {
	raw, present := body[key]
	if !present || raw == nil {
		return "", false, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", false, errUnprocessable("Expected string, received number")
	}
	if utf8.RuneCountInString(s) > max {
		return "", false, errUnprocessable("String must contain at most " + itoa(max) + " character(s)")
	}
	return strings.TrimSpace(s), true, nil
}

func asAgentNotConfigured(err error, target **agentNotConfiguredError) bool {
	if ance, ok := err.(*agentNotConfiguredError); ok {
		*target = ance
		return true
	}
	return false
}

func asCheckStatus(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	s, ok := m["status"].(string)
	if !ok {
		return ""
	}
	switch s {
	case "pending", "running", "passed", "failed", "review_required":
		return s
	}
	return ""
}

func asCheckStatusString(raw string) string {
	switch raw {
	case "pending", "running", "passed", "failed", "review_required":
		return raw
	}
	return ""
}

func unsafeURLsFromChecks(checks map[string]any) []string {
	lh, ok := checks["linkHealth"].(map[string]any)
	if !ok {
		return nil
	}
	arr, ok := lh["links"].([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, entry := range arr {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		status, _ := item["status"].(string)
		if status != "unsafe" && status != "suspicious" {
			continue
		}
		if url, ok := item["url"].(string); ok && url != "" {
			out = append(out, url)
		}
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func withUnsafeURLs(linkHealth any, unsafeURLs []string) any {
	m, ok := linkHealth.(map[string]any)
	if !ok {
		return linkHealth
	}
	if len(unsafeURLs) == 0 {
		return m
	}
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	out["unsafeUrls"] = unsafeURLs
	return out
}

// anchorReport menghitung tautan artikel beserta sifatnya. `kinds` menghitung
// PER TAUTAN (dofollow/nofollow/sponsored) supaya klien tahu komposisinya:
// dua tautan bisa satu dofollow dan satu nofollow, dan itu tidak terlihat dari
// daftar rel unik. `rels` tetap dikirim untuk kompatibilitas nilai mentahnya.
func anchorReport(html string) (int, []string, map[string]int) {
	kinds := map[string]int{}
	if html == "" {
		return 0, []string{}, kinds
	}
	matches := reAnchorOpenAll.FindAllString(html, -1)
	rels := []string{}
	seen := map[string]bool{}
	for _, tag := range matches {
		rel := ""
		if m := reRelAttr.FindStringSubmatch(tag); m != nil {
			rel = strings.ToLower(strings.TrimSpace(m[1]))
		}
		if rel != "" && !seen[rel] {
			seen[rel] = true
			rels = append(rels, rel)
		}
		switch {
		case strings.Contains(rel, "sponsored"):
			kinds["sponsored"]++
		case strings.Contains(rel, "nofollow"):
			kinds["nofollow"]++
		default:
			kinds["dofollow"]++
		}
	}
	return len(matches), rels, kinds
}

func numOrNil(row bsRow, key string) any {
	if !row.has(key) {
		return nil
	}
	return row.numOr(key)
}

// stripDisallowedIframesHTML: buang iframe host tak tepercaya (port embed.ts).
// sanitizeArticleHTMLForStore: filter iframe lama + netralisasi konstruk
// berbahaya sebelum HTML manual disimpan (lihat security.SanitizeArticleHTML).
// Artikel normal tidak berubah; hanya konstruk berbahaya yang dibuang.
// H1 di dalam isi diturunkan jadi H2 (judul artikel ada di field judul), lalu
// markup dirapikan dari sisa editor (<div> per baris, blok kosong).
func sanitizeArticleHTMLForStore(html string) string {
	safe, _ := security.SanitizeArticleHTML(html)
	return contentengine.NormalizeArticleHTML(demoteH1ToH2(safe))
}

func stripDisallowedIframesHTML(html string) string {
	if html == "" {
		return html
	}
	out := reIFramePair.ReplaceAllStringFunc(html, func(tag string) string {
		if isAllowedEmbedSrcHTML(iframeSrcHTML(tag)) {
			return tag
		}
		return ""
	})
	return reIFrameAny.ReplaceAllStringFunc(out, func(tag string) string {
		if isAllowedEmbedSrcHTML(iframeSrcHTML(tag)) {
			return tag
		}
		return ""
	})
}

var allowedEmbedHostsHTML = map[string]bool{
	"www.youtube.com": true, "youtube.com": true,
	"www.youtube-nocookie.com": true, "youtube-nocookie.com": true,
	"player.vimeo.com": true,
	"www.google.com":   true, "google.com": true, "maps.google.com": true,
}

func iframeSrcHTML(tag string) string {
	m := reIFrameSrc.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

func isAllowedEmbedSrcHTML(src string) bool {
	if src == "" || !strings.HasPrefix(strings.ToLower(src), "https://") {
		return false
	}
	rest := src[len("https://"):]
	host := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		host = rest[:i]
	}
	host = strings.ToLower(host)
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if colon := strings.Index(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	return allowedEmbedHostsHTML[host]
}
