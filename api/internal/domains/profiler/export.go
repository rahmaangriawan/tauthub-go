package profiler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ProfileExport: GET /v1/brands/{brandId}/profiler/profiles/{profileId}/export
//
// Ekspor seluruh data brief milik klien dalam satu JSON: field terstruktur,
// instruksi, blok knowledge, dokumen (metadata + teks hasil ekstraksi), URL,
// gambar (metadata), timeline, dan ringkasan siap pakai. Dipakai untuk hak
// portabilitas data — tidak ada data tenant lain yang ikut karena profil sudah
// di-scope ke brand lewat akses + query.
//
// Catatan privasi yang JUJUR: teks dokumen yang sudah dihapus (deleted_at
// terisi) tidak diekspor karena extracted_text-nya ikut dikosongkan saat
// penghapusan. Ringkasan yang pernah dibuat sebelum penghapusan tetap bisa
// memuat kutipan — itu keterbatasan yang tidak bisa dihapus surut.
func (d *Deps) ProfileExport(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")

	var (
		name                       string
		description, instructions  *string
		identity, style, seo       []byte
		entityCore                 []byte
		createdAt, updatedAt       any
	)
	row := d.Pool.QueryRow(r.Context(), `
		SELECT name, description, identity, style, seo, entity_core, instructions, created_at, updated_at
		FROM profiler_profiles WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL`, profileID, brandID)
	if err := row.Scan(&name, &description, &identity, &style, &seo, &entityCore, &instructions, &createdAt, &updatedAt); err != nil {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}

	payload := map[string]any{
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"format":      "tauthub.brief.v1",
		"profile": map[string]any{
			"id": profileID, "name": name, "description": description,
			"identity": asMap(identity), "style": asMap(style), "seo": asMap(seo),
			"entity_core": asMap(entityCore), "instructions": instructions,
			"created_at": createdAt, "updated_at": updatedAt,
		},
		"blocks":       d.listBlocks(r.Context(), profileID),
		"documents":    d.listDocumentsForExport(r.Context(), profileID),
		"images":       d.listImages(r.Context(), profileID),
		"urls":         d.listURLsForExport(r.Context(), profileID),
		"timeline":     d.listTimeline(r.Context(), profileID),
		"summaries":    d.listSummariesForExport(r.Context(), profileID),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="brief-`+profileID[:8]+`.json"`)
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

// listDocumentsForExport: metadata + extracted_text (kolom yang sama dipakai
// konteks AI). Hanya baris aktif.
func (d *Deps) listDocumentsForExport(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, original_filename, mime_type, size_bytes, parse_status, created_at,
		       COALESCE(extracted_text, '')
		FROM profiler_documents WHERE profile_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`, profileID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var (
			id, name, status, text string
			mime                   *string
			size                   *int64
			createdAt              any
		)
		if err := rows.Scan(&id, &name, &mime, &size, &status, &createdAt, &text); err != nil {
			continue
		}
		out = append(out, map[string]any{
			"id": id, "original_filename": name, "mime_type": mime, "size_bytes": size,
			"parse_status": status, "created_at": createdAt, "extracted_text": text,
		})
	}
	return out
}

// listURLsForExport: URL referensi + teks hasil fetch.
func (d *Deps) listURLsForExport(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, url, fetch_status, COALESCE(extracted_text, ''), created_at
		FROM profiler_urls WHERE profile_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`, profileID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, url, status, text string
		var createdAt any
		if err := rows.Scan(&id, &url, &status, &text, &createdAt); err != nil {
			continue
		}
		out = append(out, map[string]any{
			"id": id, "url": url, "fetch_status": status,
			"created_at": createdAt, "extracted_text": text,
		})
	}
	return out
}

// listSummariesForExport: ringkasan entity/file yang sudah jadi.
func (d *Deps) listSummariesForExport(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx, `
		SELECT kind, status, COALESCE(content, ''), updated_at
		FROM profiler_brief_summaries WHERE profile_id = $1
		ORDER BY kind`, profileID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var kind, status, content string
		var updatedAt any
		if err := rows.Scan(&kind, &status, &content, &updatedAt); err != nil {
			continue
		}
		out = append(out, map[string]any{
			"kind": kind, "status": status, "content": content, "updated_at": updatedAt,
		})
	}
	return out
}
