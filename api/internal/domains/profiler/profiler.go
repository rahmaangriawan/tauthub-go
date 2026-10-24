package profiler

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/doctext"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	briefsummary "github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Profiler (Structured Knowledge Workspace) tingkat klien.
//
// Satu klien (brand) bisa punya banyak Profiler. Tiap Profiler menyimpan field
// TERSTRUKTUR (identitas bisnis, gaya penulisan, pedoman konten & SEO),
// instruksi khusus bebas, blok knowledge kustom, dokumen referensi, gambar
// referensi, dan URL referensi. Backend menyusun semua itu jadi konteks untuk
// The Writer — bukan menyerahkan system prompt mentah.
//
// Catatan penamaan: ini BUKAN agent "Profiler" Taut Pintar (taut_profiler_*,
// per-kampanye). Ini workspace knowledge milik klien.

type Deps struct {
	Pool *pgxpool.Pool
}

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func (d *Deps) access(r *http.Request, brandID, userID, minRole string) (brandsdomain.BrandRow, error) {
	return brandsdomain.BrandAccess(r.Context(), d.Pool, brandID, userID, minRole)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, err error) {
	var be *brandsdomain.BrandError
	if errors.As(err, &be) {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, err.Error())
}

func decodeJSON(r *http.Request) map[string]any {
	var body map[string]any
	_ = json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&body)
	if body == nil {
		body = map[string]any{}
	}
	return body
}

func boolField(body map[string]any, key string) (bool, bool) {
	v, ok := body[key]
	if !ok || v == nil {
		return false, false
	}
	b, isBool := v.(bool)
	return b, isBool
}

func strField(body map[string]any, key string) (string, bool) {
	v, ok := body[key]
	if !ok || v == nil {
		return "", false
	}
	s, isStr := v.(string)
	if !isStr {
		return "", false
	}
	return strings.TrimSpace(s), true
}

func jsonField(body map[string]any, key string) []byte {
	v, ok := body[key]
	if !ok || v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}

func uploadDir() string {
	if v := strings.TrimSpace(os.Getenv("UPLOAD_DIR")); v != "" {
		return v
	}
	return filepath.Join(".data", "uploads")
}

func saveUpload(key string, data []byte) error {
	dir := uploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, key), data, 0o644)
}

// ---------- profiler CRUD ----------

// ProfilesList: GET /v1/brands/{brandId}/profiler/profiles
func (d *Deps) ProfilesList(w http.ResponseWriter, r *http.Request) {
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
	rows, err := d.Pool.Query(r.Context(), `
		SELECT p.id, p.name, p.description, p.short_code, p.identity, p.style, p.seo, p.instructions,
		       p.pinned_at, p.updated_at,
		       (SELECT COUNT(*) FROM profiler_documents x WHERE x.profile_id = p.id AND x.deleted_at IS NULL) AS docs,
		       (SELECT COUNT(*) FROM profiler_images x WHERE x.profile_id = p.id AND x.deleted_at IS NULL) AS imgs,
		       (SELECT COUNT(*) FROM profiler_urls x WHERE x.profile_id = p.id AND x.deleted_at IS NULL) AS urls,
		       (SELECT COUNT(*) FROM profiler_blocks x WHERE x.profile_id = p.id) AS blocks
		FROM profiler_profiles p
		WHERE p.brand_id = $1 AND p.deleted_at IS NULL
		ORDER BY p.pinned_at DESC NULLS LAST, p.updated_at DESC`, brandID)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var (
			id, name                      string
			description, instructions     *string
			shortCode                     *string
			identity, style, seo          []byte
			pinnedAt, updatedAt           any
			docs, imgs, urls, blocksCount int
		)
		if err := rows.Scan(&id, &name, &description, &shortCode, &identity, &style, &seo, &instructions,
			&pinnedAt, &updatedAt, &docs, &imgs, &urls, &blocksCount); err != nil {
			writeErr(w, err)
			return
		}
		items = append(items, map[string]any{
			"id": id, "name": name, "description": description,
			"short_code":      shortCode,
			"pinned":          pinnedAt != nil,
			"readiness":       readiness(identity, style, seo, instructions, docs, imgs, urls, blocksCount),
			"documents_count": docs, "images_count": imgs, "urls_count": urls, "blocks_count": blocksCount,
			"updated_at": updatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": items})
}

// readiness: status kesiapan data sederhana (belum diisi / siap / perlu diperiksa).
func readiness(identity, style, seo []byte, instructions *string, docs, imgs, urls, blocks int) string {
	filled := 0
	for _, b := range [][]byte{identity, style, seo} {
		if len(bytes.TrimSpace(b)) > 2 && string(bytes.TrimSpace(b)) != "{}" {
			filled++
		}
	}
	has := instructions != nil && strings.TrimSpace(*instructions) != ""
	hasKnowledge := docs > 0 || urls > 0 || blocks > 0
	switch {
	case filled == 0 && !has && !hasKnowledge && imgs == 0:
		return "empty"
	case filled >= 2 && (has || hasKnowledge):
		return "ready"
	default:
		return "review"
	}
}

// shortCodeFromUUID: 5 karakter lowercase dari UUID — kunci URL ringkas.
func shortCodeFromUUID() string { return strings.ToLower(uuid.NewString())[:5] }

// ProfileCreate: POST /v1/brands/{brandId}/profiler/profiles  {name, description?}
func (d *Deps) ProfileCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	name, _ := strField(body, "name")
	description, _ := strField(body, "description")
	if name == "" || len(name) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Nama Profiler wajib diisi (maks 160 karakter).")
		return
	}
	// Kode pendek untuk URL ringkas (/profiler/{kode}). Bentrok dijaga dengan
	// mencoba ulang bila unique index-nya mengeluh.
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		code := shortCodeFromUUID()
		id := uuid.NewString()
		if _, err := d.Pool.Exec(r.Context(), `
			INSERT INTO profiler_profiles (id, brand_id, name, description, short_code, created_by)
			VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)`, id, brandID, name, description, code, user.ID); err != nil {
			lastErr = err
			continue
		}
		d.queueBriefSummary(r.Context(), brandID, id, briefsummary.KindEntity, briefsummary.KindFile)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "short_code": code})
		return
	}
	writeErr(w, lastErr)
}

// ProfileGet: GET /v1/brands/{brandId}/profiler/profiles/{profileId}
func (d *Deps) ProfileGet(w http.ResponseWriter, r *http.Request) {
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
	row := d.Pool.QueryRow(r.Context(), `
		SELECT id, name, description, identity, style, seo, entity_core, instructions, created_at, updated_at
		FROM profiler_profiles WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL`, profileID, brandID)
	var (
		id, name                  string
		description, instructions *string
		identity, style, seo      []byte
		entityCore                []byte
		createdAt, updatedAt      any
	)
	if err := row.Scan(&id, &name, &description, &identity, &style, &seo, &entityCore, &instructions, &createdAt, &updatedAt); err != nil {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	out := map[string]any{
		"id": id, "name": name, "description": description,
		"identity": asMap(identity), "style": asMap(style), "seo": asMap(seo),
		"entity_core":  asMap(entityCore),
		"instructions": instructions, "created_at": createdAt, "updated_at": updatedAt,
		"blocks":    d.listBlocks(r.Context(), profileID),
		"documents": d.listDocuments(r.Context(), profileID),
		"images":    d.listImages(r.Context(), profileID),
		"urls":      d.listURLs(r.Context(), profileID),
		"timeline":  d.listTimeline(r.Context(), profileID),
	}
	writeJSON(w, http.StatusOK, out)
}

func asMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

// ProfileUpdate: PUT /v1/brands/{brandId}/profiler/profiles/{profileId}
func (d *Deps) ProfileUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")
	body := decodeJSON(r)
	var (
		sets []string
		args []any
		n    = 1
	)
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, n))
		args = append(args, val)
		n++
	}
	if v, ok := strField(body, "name"); ok {
		if v == "" || len(v) > 160 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Nama Profiler tidak valid.")
			return
		}
		add("name", v)
	}
	if v, ok := strField(body, "description"); ok {
		add("description", nullIfEmpty(v))
	}
	if v, ok := strField(body, "instructions"); ok {
		if len(v) > 20000 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Instruksi khusus maksimal 20000 karakter.")
			return
		}
		add("instructions", nullIfEmpty(v))
	}
	if pinned, hasPinned := boolField(body, "pinned"); hasPinned {
		if pinned {
			add("pinned_at", time.Now())
		} else {
			add("pinned_at", nil)
		}
	}
	for _, k := range []string{"identity", "style", "seo", "entity_core"} {
		if raw := jsonField(body, k); raw != nil {
			// Kirim sebagai string supaya pgx mengirim teks JSON (bukan bytea).
			add(k, string(raw))
		}
	}
	if len(sets) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tidak ada perubahan.")
		return
	}
	add("updated_by", user.ID)
	args = append(args, profileID, brandID)
	q := fmt.Sprintf("UPDATE profiler_profiles SET %s, updated_at = NOW() WHERE id = $%d AND brand_id = $%d AND deleted_at IS NULL",
		strings.Join(sets, ", "), n, n+1)
	tag, err := d.Pool.Exec(r.Context(), q, args...)
	if err != nil {
		writeErr(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindEntity)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ProfileDelete: DELETE /v1/brands/{brandId}/profiler/profiles/{profileId} (soft)
func (d *Deps) ProfileDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	tag, err := d.Pool.Exec(r.Context(),
		"UPDATE profiler_profiles SET deleted_at = NOW() WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL",
		chiParam(r, "profileId"), brandID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	// Kampanye yang memakai Profiler ini TIDAK diubah: snapshot-nya sudah disimpan.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ProfileDuplicate: POST /v1/brands/{brandId}/profiler/profiles/{profileId}/duplicate
func (d *Deps) ProfileDuplicate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")
	ctx := r.Context()
	var (
		name                      string
		description, instructions *string
		identity, style, seo      []byte
	)
	if err := d.Pool.QueryRow(ctx,
		"SELECT name, description, identity, style, seo, instructions FROM profiler_profiles WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL",
		profileID, brandID).Scan(&name, &description, &identity, &style, &seo, &instructions); err != nil {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	newID := uuid.NewString()
	var insertErr error
	var shortCode string
	for attempt := 0; attempt < 3; attempt++ {
		shortCode = shortCodeFromUUID()
		_, insertErr = d.Pool.Exec(ctx, `
			INSERT INTO profiler_profiles
			  (id, brand_id, name, description, identity, style, seo, instructions, short_code, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			newID, brandID, truncate(name+" (salinan)", 160), description,
			string(identity), string(style), string(seo), instructions, shortCode, user.ID)
		if insertErr == nil {
			break
		}
	}
	if insertErr != nil {
		writeErr(w, insertErr)
		return
	}
	// Salin blok knowledge kustom.
	if _, err := d.Pool.Exec(ctx, `
		INSERT INTO profiler_blocks (id, profile_id, title, kind, content, note, sort_order)
		SELECT gen_random_uuid()::text, $1, title, kind, content, note, sort_order FROM profiler_blocks WHERE profile_id = $2`,
		newID, profileID); err != nil {
		writeErr(w, err)
		return
	}
	d.queueBriefSummary(ctx, brandID, newID, briefsummary.KindEntity, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"id": newID, "short_code": shortCode})
}

// ---------- blocks ----------

func (d *Deps) listBlocks(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx,
		"SELECT id, title, kind, content, note, sort_order FROM profiler_blocks WHERE profile_id = $1 ORDER BY sort_order, created_at", profileID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, title string
		var kind, content, note *string
		var sortOrder int
		if err := rows.Scan(&id, &title, &kind, &content, &note, &sortOrder); err != nil {
			continue
		}
		out = append(out, map[string]any{"id": id, "title": title, "kind": kind, "content": content, "note": note, "sort_order": sortOrder})
	}
	return out
}

// BlockCreate: POST /v1/brands/{brandId}/profiler/profiles/{profileId}/blocks
func (d *Deps) BlockCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")
	if !d.profileInBrand(r.Context(), profileID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	body := decodeJSON(r)
	title, _ := strField(body, "title")
	kind, _ := strField(body, "kind")
	content, _ := strField(body, "content")
	note, _ := strField(body, "note")
	if title == "" || len(title) > 200 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Judul blok wajib diisi (maks 200 karakter).")
		return
	}
	id := uuid.NewString()
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO profiler_blocks (id, profile_id, title, kind, content, note) VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6,''))`,
		id, profileID, title, kind, content, note); err != nil {
		writeErr(w, err)
		return
	}
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindEntity)
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (d *Deps) profileInBrand(ctx context.Context, profileID, brandID string) bool {
	var exists bool
	_ = d.Pool.QueryRow(ctx,
		"SELECT true FROM profiler_profiles WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL", profileID, brandID).Scan(&exists)
	return exists
}

// queueBriefSummary: tandai ringkasan brief perlu dibuat ulang. Job-nya async
// (diproses scheduler), jadi respons HTTP tidak menunggu model.
func (d *Deps) queueBriefSummary(ctx context.Context, brandID, profileID string, kinds ...briefsummary.Kind) {
	if profileID == "" {
		return
	}
	for _, kind := range kinds {
		_ = briefsummary.Queue(ctx, d.Pool, profileID, brandID, kind)
	}
}

// profileIDOfChild: pemilik baris anak (blocks/documents/images/urls).
func (d *Deps) profileIDOfChild(ctx context.Context, table, id string) string {
	var profileID string
	_ = d.Pool.QueryRow(ctx, fmt.Sprintf("SELECT profile_id FROM %s WHERE id = $1", table), id).Scan(&profileID)
	return profileID
}

// BlockUpdate: PUT /v1/brands/{brandId}/profiler/blocks/{blockId}
func (d *Deps) BlockUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	blockID := chiParam(r, "blockId")
	if !d.childInBrand(r.Context(), "profiler_blocks", blockID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Blok tidak ditemukan.")
		return
	}
	body := decodeJSON(r)
	var (
		sets []string
		args []any
		n    = 1
	)
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, n))
		args = append(args, val)
		n++
	}
	if v, ok := strField(body, "title"); ok {
		add("title", v)
	}
	if v, ok := strField(body, "kind"); ok {
		add("kind", nullIfEmpty(v))
	}
	if v, ok := strField(body, "content"); ok {
		add("content", nullIfEmpty(v))
	}
	if v, ok := strField(body, "note"); ok {
		add("note", nullIfEmpty(v))
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	args = append(args, blockID)
	q := fmt.Sprintf("UPDATE profiler_blocks SET %s, updated_at = NOW() WHERE id = $%d", strings.Join(sets, ", "), n)
	if _, err := d.Pool.Exec(r.Context(), q, args...); err != nil {
		writeErr(w, err)
		return
	}
	d.queueBriefSummary(r.Context(), brandID, d.profileIDOfChild(r.Context(), "profiler_blocks", blockID), briefsummary.KindEntity)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// BlockDelete: DELETE /v1/brands/{brandId}/profiler/blocks/{blockId}
func (d *Deps) BlockDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	blockID := chiParam(r, "blockId")
	if !d.childInBrand(r.Context(), "profiler_blocks", blockID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Blok tidak ditemukan.")
		return
	}
	profileID := d.profileIDOfChild(r.Context(), "profiler_blocks", blockID)
	if _, err := d.Pool.Exec(r.Context(), "DELETE FROM profiler_blocks WHERE id = $1", blockID); err != nil {
		writeErr(w, err)
		return
	}
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindEntity)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// childInBrand: pastikan baris anak (blocks/images/documents/urls) milik brand
// yang sedang login — lewat relasi ke profiler_profiles.
func (d *Deps) childInBrand(ctx context.Context, table, id, brandID string) bool {
	var exists bool
	q := fmt.Sprintf(`SELECT true FROM %s c JOIN profiler_profiles p ON p.id = c.profile_id
		WHERE c.id = $1 AND p.brand_id = $2 AND p.deleted_at IS NULL`, table)
	_ = d.Pool.QueryRow(ctx, q, id, brandID).Scan(&exists)
	return exists
}

// ---------- documents ----------

func (d *Deps) listDocuments(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, original_filename, storage_key, mime_type, size_bytes, parse_status, parse_error, file_deleted_at, created_at
		FROM profiler_documents WHERE profile_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`, profileID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, filename, status, storageKey string
		var mime, parseErr *string
		var size *int64
		var fileDeletedAt, createdAt any
		if err := rows.Scan(&id, &filename, &storageKey, &mime, &size, &status, &parseErr, &fileDeletedAt, &createdAt); err != nil {
			continue
		}
		// Berkas fisik yang sudah dibersihkan tidak punya URL unduhan; teks
		// ekstraksinya tetap dipakai AI dari DB.
		url := ""
		if fileDeletedAt == nil {
			url = "/uploads/" + storageKey
		}
		out = append(out, map[string]any{
			"id": id, "original_filename": filename, "mime_type": mime, "size_bytes": size,
			"parse_status": status, "parse_error": parseErr, "file_deleted_at": fileDeletedAt, "created_at": createdAt,
			"url": url,
		})
	}
	return out
}

// DocumentUpload: POST /v1/brands/{brandId}/profiler/profiles/{profileId}/documents (multipart)
//
// Urutan pemrosesan (kuota & siklus hidup — lihat uploadquota.go):
//   body cap (MaxBytesReader) → baca berkas → validasi ukuran aktual →
//   allowlist MIME → sniff magic number → dedup checksum → RESERVASI kuota
//   (advisory lock + insert ledger 'processing') → simpan berkas → ekstrak
//   teks → INSERT profiler_documents → finalisasi ledger.
// Teks hasil ekstraksi tetap di DB sebagai memori AI; berkas fisiknya
// sementara dan dibersihkan worker sesuai retensi.
func (d *Deps) DocumentUpload(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")
	if !d.profileInBrand(r.Context(), profileID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	lim := LoadUploadLimits()
	// Batasi body di tingkat HTTP: berkas + overhead multipart. Mencegah
	// request raksasa memakan memori sebelum validasi apa pun jalan.
	r.Body = http.MaxBytesReader(w, r.Body, lim.MaxFileBytes+(2<<20))
	if r.ContentLength > 0 && r.ContentLength > lim.MaxFileBytes+(2<<20) {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "Berkas melebihi batas ukuran unggahan.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "Berkas melebihi batas ukuran unggahan.")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "File tidak ditemukan.")
		return
	}
	defer file.Close()
	buf, err := io.ReadAll(io.LimitReader(file, lim.MaxFileBytes+1))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File tidak dapat dibaca.")
		return
	}
	// Ukuran AKTUAL, bukan klaim header.
	if int64(len(buf)) > lim.MaxFileBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "Berkas melebihi batas ukuran unggahan.")
		return
	}
	filename := filepath.Base(header.Filename)
	mime := docMime(filename, header.Header.Get("Content-Type"))
	if _, allowed := acceptedDocMimes[mime]; !allowed {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tipe file tidak diizinkan. Izinkan: PDF, DOCX, TXT, MD, CSV.")
		return
	}
	if !sniffFormatOK(buf, mime) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi berkas tidak cocok dengan formatnya. Berkas mungkin rusak atau menyamar.")
		return
	}
	source := "chat"
	if r.FormValue("source") == "library" {
		source = "library"
	}
	sum := sha256.Sum256(buf)
	checksum := hex.EncodeToString(sum[:])
	// Idempotensi: file identik tidak digandakan (tidak memakan kuota).
	var existingID string
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT id FROM profiler_documents WHERE profile_id = $1 AND checksum = $2 AND deleted_at IS NULL LIMIT 1",
		profileID, checksum).Scan(&existingID); err == nil && existingID != "" {
		writeJSON(w, http.StatusOK, map[string]any{"id": existingID, "deduplicated": true})
		return
	}
	docID := uuid.NewString()
	ext := sanitizeExt(filename)
	storageKey := docID + "." + ext
	if err := d.reserveAttachment(r.Context(), user.ID, profileID, "document", source, storageKey, filename, mime, int64(len(buf))); err != nil {
		if v := quotaViolationOf(err); v != nil {
			httpx.Error(w, v.Status, v.Message)
			return
		}
		writeErr(w, err)
		return
	}
	if err := saveUpload(storageKey, buf); err != nil {
		d.finalizeAttachment(r.Context(), storageKey, "document", "failed", truncate(err.Error(), 500))
		writeErr(w, err)
		return
	}
	text, parseErr := ExtractText(buf, mime)
	status := "ready"
	var errMsg *string
	if parseErr != nil {
		status = "failed"
		m := truncate(parseErr.Error(), 500)
		errMsg = &m
	}
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO profiler_documents (id, profile_id, original_filename, storage_key, mime_type, size_bytes, checksum, parse_status, extracted_text, parse_error, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		docID, profileID, filename, storageKey, mime, len(buf), checksum, status, nullIfEmpty(truncate(text, 200000)), errMsg, user.ID); err != nil {
		d.finalizeAttachment(r.Context(), storageKey, "document", "failed", truncate(err.Error(), 500))
		writeErr(w, err)
		return
	}
	if parseErr != nil {
		d.finalizeAttachment(r.Context(), storageKey, "document", "failed", truncate(parseErr.Error(), 500))
	} else {
		d.finalizeAttachment(r.Context(), storageKey, "document", "processed", "")
	}
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"id": docID, "parse_status": status, "parse_error": errMsg})
}

// DocumentDelete: DELETE /v1/brands/{brandId}/profiler/documents/{documentId}
func (d *Deps) DocumentDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	docID := chiParam(r, "documentId")
	if !d.childInBrand(r.Context(), "profiler_documents", docID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Dokumen tidak ditemukan.")
		return
	}
	profileID := d.profileIDOfChild(r.Context(), "profiler_documents", docID)
	var storageKey string
	_ = d.Pool.QueryRow(r.Context(), `SELECT storage_key FROM profiler_documents WHERE id = $1`, docID).Scan(&storageKey)
	_, _ = d.Pool.Exec(r.Context(), "UPDATE profiler_documents SET deleted_at = NOW() WHERE id = $1", docID)
	// Berkas fisik menyusul dihapus worker pada siklus berikutnya (delete_pending).
	d.markAttachmentPendingDelete(r.Context(), storageKey)
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- images ----------

func (d *Deps) listImages(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx,
		"SELECT id, url, file_name, mime_type, size_bytes, note, created_at FROM profiler_images WHERE profile_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC", profileID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, url string
		var fileName, mime, note *string
		var size *int64
		var createdAt any
		if err := rows.Scan(&id, &url, &fileName, &mime, &size, &note, &createdAt); err != nil {
			continue
		}
		out = append(out, map[string]any{"id": id, "url": url, "file_name": fileName, "mime_type": mime, "size_bytes": size, "note": note, "created_at": createdAt})
	}
	return out
}

// ImageUpload: POST /v1/brands/{brandId}/profiler/profiles/{profileId}/images (multipart)
//
// Mengikuti pola kuota DocumentUpload. Gambar adalah aset pustaka media —
// retensinya lebih panjang (default 30 hari) dan baris profiler_images
// di-soft-delete saat berkas fisiknya dibersihkan, supaya tab Media tidak
// menampilkan thumbnail mati.
func (d *Deps) ImageUpload(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")
	if !d.profileInBrand(r.Context(), profileID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	lim := LoadUploadLimits()
	r.Body = http.MaxBytesReader(w, r.Body, lim.MaxFileBytes+(2<<20))
	if r.ContentLength > 0 && r.ContentLength > lim.MaxFileBytes+(2<<20) {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "Gambar melebihi batas ukuran unggahan.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "Gambar melebihi batas ukuran unggahan.")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "File gambar tidak ditemukan.")
		return
	}
	defer file.Close()
	buf, err := io.ReadAll(io.LimitReader(file, lim.MaxFileBytes+1))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File tidak dapat dibaca.")
		return
	}
	if int64(len(buf)) > lim.MaxFileBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "Gambar melebihi batas ukuran unggahan.")
		return
	}
	filename := filepath.Base(header.Filename)
	mime := docMime(filename, header.Header.Get("Content-Type"))
	if !acceptedImageMimes[mime] {
		httpx.Error(w, http.StatusUnprocessableEntity, "File harus berupa gambar (JPG, PNG, WebP, GIF).")
		return
	}
	// Sniff isi: ekstensi .png yang sebenarnya skrip/HTML harus tertolak.
	if !sniffFormatOK(buf, mime) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi berkas bukan gambar yang valid.")
		return
	}
	source := "chat"
	if r.FormValue("source") == "library" {
		source = "library"
	}
	note, _ := strField(decodeJSON(r), "note")
	imgID := uuid.NewString()
	ext := sanitizeExt(filename)
	if ext == "" {
		ext = "png"
	}
	storageKey := imgID + "." + ext
	if err := d.reserveAttachment(r.Context(), user.ID, profileID, "image", source, storageKey, filename, mime, int64(len(buf))); err != nil {
		if v := quotaViolationOf(err); v != nil {
			httpx.Error(w, v.Status, v.Message)
			return
		}
		writeErr(w, err)
		return
	}
	if err := saveUpload(storageKey, buf); err != nil {
		d.finalizeAttachment(r.Context(), storageKey, "image", "failed", truncate(err.Error(), 500))
		writeErr(w, err)
		return
	}
	url := "/uploads/" + storageKey
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO profiler_images (id, profile_id, storage_key, url, file_name, mime_type, size_bytes, note, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8,''), $9)`,
		imgID, profileID, storageKey, url, filename, mime, len(buf), note, user.ID); err != nil {
		d.finalizeAttachment(r.Context(), storageKey, "image", "failed", truncate(err.Error(), 500))
		writeErr(w, err)
		return
	}
	d.finalizeAttachment(r.Context(), storageKey, "image", "processed", "")
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"id": imgID, "url": url})
}

// ImageDelete: DELETE /v1/brands/{brandId}/profiler/images/{imageId}
func (d *Deps) ImageDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	imageID := chiParam(r, "imageId")
	if !d.childInBrand(r.Context(), "profiler_images", imageID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Gambar tidak ditemukan.")
		return
	}
	profileID := d.profileIDOfChild(r.Context(), "profiler_images", imageID)
	var storageKey string
	_ = d.Pool.QueryRow(r.Context(), `SELECT storage_key FROM profiler_images WHERE id = $1`, imageID).Scan(&storageKey)
	_, _ = d.Pool.Exec(r.Context(), "UPDATE profiler_images SET deleted_at = NOW() WHERE id = $1", imageID)
	d.markAttachmentPendingDelete(r.Context(), storageKey)
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ImageUpdate: PUT /v1/brands/{brandId}/profiler/images/{imageId}  {note}
// Catatan/deskripsi opsional per gambar supaya AI tahu konteksnya.
func (d *Deps) ImageUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	imageID := chiParam(r, "imageId")
	if !d.childInBrand(r.Context(), "profiler_images", imageID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Gambar tidak ditemukan.")
		return
	}
	note, ok := strField(decodeJSON(r), "note")
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if _, err := d.Pool.Exec(r.Context(), "UPDATE profiler_images SET note = NULLIF($1, '') WHERE id = $2", note, imageID); err != nil {
		writeErr(w, err)
		return
	}
	d.queueBriefSummary(r.Context(), brandID, d.profileIDOfChild(r.Context(), "profiler_images", imageID), briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- urls ----------

func (d *Deps) listURLs(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx,
		"SELECT id, url, title, note, fetch_status, fetch_error, last_synced_at FROM profiler_urls WHERE profile_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC", profileID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, url, status string
		var title, note, fetchErr *string
		var lastSynced any
		if err := rows.Scan(&id, &url, &title, &note, &status, &fetchErr, &lastSynced); err != nil {
			continue
		}
		out = append(out, map[string]any{"id": id, "url": url, "title": title, "note": note, "fetch_status": status, "fetch_error": fetchErr, "last_synced_at": lastSynced})
	}
	return out
}

// URLAdd: POST /v1/brands/{brandId}/profiler/profiles/{profileId}/urls  {url, title?, note?}
func (d *Deps) URLAdd(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")
	if !d.profileInBrand(r.Context(), profileID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	body := decodeJSON(r)
	rawURL, _ := strField(body, "url")
	title, _ := strField(body, "title")
	note, _ := strField(body, "note")
	if !isHTTPURL(rawURL) {
		httpx.Error(w, http.StatusUnprocessableEntity, "URL harus diawali http:// atau https://")
		return
	}
	id := uuid.NewString()
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO profiler_urls (id, profile_id, url, title, note, created_by) VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6)`,
		id, profileID, rawURL, title, note, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	// Ambil isi langsung; kegagalan tidak membatalkan penambahan.
	d.syncURL(r.Context(), id, brandID)
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

// URLSync: POST /v1/brands/{brandId}/profiler/urls/{urlId}/sync
func (d *Deps) URLSync(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	urlID := chiParam(r, "urlId")
	if !d.childInBrand(r.Context(), "profiler_urls", urlID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Referensi URL tidak ditemukan.")
		return
	}
	profileID := d.profileIDOfChild(r.Context(), "profiler_urls", urlID)
	d.syncURL(r.Context(), urlID, brandID)
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// syncURL: ambil konten halaman dengan aman (anti-SSRF lewat security.Get),
// bersihkan jadi teks, simpan status. Tandai 'ready' HANYA bila isi berhasil
// diambil — bukan sekadar URL ada.
func (d *Deps) syncURL(ctx context.Context, urlID, brandID string) {
	var rawURL string
	if err := d.Pool.QueryRow(ctx, "SELECT url FROM profiler_urls WHERE id = $1 AND deleted_at IS NULL", urlID).Scan(&rawURL); err != nil {
		return
	}
	_, _ = d.Pool.Exec(ctx, "UPDATE profiler_urls SET fetch_status = 'fetching', fetch_error = NULL WHERE id = $1", urlID)
	resp, err := security.Get(ctx, rawURL, map[string]string{"User-Agent": "TauthubBot/1.0"}, 20*time.Second)
	if err != nil {
		d.markURLFailed(ctx, urlID, "Gagal mengambil halaman: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		d.markURLFailed(ctx, urlID, fmt.Sprintf("Halaman mengembalikan status %d.", resp.StatusCode))
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		d.markURLFailed(ctx, urlID, "Isi halaman tidak dapat dibaca.")
		return
	}
	text := htmlToText(string(body))
	if len([]rune(text)) < 80 {
		d.markURLFailed(ctx, urlID, "Isi halaman terlalu pendek untuk dijadikan referensi (mungkin butuh login atau memblokir crawler).")
		return
	}
	title := firstTitle(string(body))
	_, _ = d.Pool.Exec(ctx, `
		UPDATE profiler_urls SET fetch_status = 'ready', extracted_text = $1, fetch_error = NULL,
		       title = COALESCE(NULLIF(title, ''), NULLIF($2, '')), last_synced_at = NOW() WHERE id = $3`,
		truncate(text, 100000), title, urlID)
}

func (d *Deps) markURLFailed(ctx context.Context, urlID, msg string) {
	_, _ = d.Pool.Exec(ctx, "UPDATE profiler_urls SET fetch_status = 'failed', fetch_error = $1 WHERE id = $2", truncate(msg, 500), urlID)
}

// URLDelete: DELETE /v1/brands/{brandId}/profiler/urls/{urlId}
func (d *Deps) URLDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	urlID := chiParam(r, "urlId")
	if !d.childInBrand(r.Context(), "profiler_urls", urlID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Referensi URL tidak ditemukan.")
		return
	}
	profileID := d.profileIDOfChild(r.Context(), "profiler_urls", urlID)
	_, _ = d.Pool.Exec(r.Context(), "UPDATE profiler_urls SET deleted_at = NOW() WHERE id = $1", urlID)
	d.queueBriefSummary(r.Context(), brandID, profileID, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- util ----------

var (
	// RE2 (Go) tidak mendukung backreference, jadi tiap tag dibersihkan sendiri.
	reScript   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reNoscript = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	reSvg      = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
	reTags     = regexp.MustCompile(`(?s)<[^>]+>`)
	reSpaces   = regexp.MustCompile(`[ \t\r\f\v]+`)
	reBlankRun = regexp.MustCompile(`\n{3,}`)
	reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

func htmlToText(html string) string {
	s := reScript.ReplaceAllString(html, " ")
	s = reStyle.ReplaceAllString(s, " ")
	s = reNoscript.ReplaceAllString(s, " ")
	s = reSvg.ReplaceAllString(s, " ")
	s = reTags.ReplaceAllString(s, "\n")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'").Replace(s)
	s = reSpaces.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			kept = append(kept, t)
		}
	}
	return strings.TrimSpace(reBlankRun.ReplaceAllString(strings.Join(kept, "\n"), "\n\n"))
}

func firstTitle(html string) string {
	m := reTitle.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return truncate(strings.TrimSpace(reTags.ReplaceAllString(m[1], " ")), 255)
}

var acceptedDocMimes = map[string]bool{
	"application/pdf": true,
	"text/plain":      true,
	"text/csv":        true,
	"text/markdown":   true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

// Hanya raster: SVG bisa membawa skrip dan disajikan dari origin aplikasi.
var acceptedImageMimes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

func docMime(filename, contentType string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	switch ext {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "md", "markdown":
		return "text/markdown"
	case "csv":
		return "text/csv"
	case "txt":
		return "text/plain"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	}
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if acceptedDocMimes[ct] || acceptedImageMimes[ct] {
		return ct
	}
	return "text/plain"
}

// ExtractText: ekstraksi teks server-side (TXT/CSV/MD native, DOCX via stdlib,
// PDF via library pihak ketiga).
func ExtractText(buffer []byte, mime string) (string, error) {
	switch mime {
	case "text/plain", "text/csv", "text/markdown":
		return string(buffer), nil
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return extractDocxText(buffer)
	case "application/pdf":
		return doctext.FromPDF(buffer)
	}
	return "", errors.New("Parser dokumen " + mime + " belum tersedia. Gunakan DOCX, PDF, TXT, MD, atau CSV.")
}

func extractDocxText(buffer []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(buffer), int64(len(buffer)))
	if err != nil {
		return "", errors.New("DOCX tidak dapat dibuka — arsipnya rusak atau bukan berkas Office yang valid.")
	}
	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", errors.New("DOCX tidak berisi word/document.xml.")
	}
	rc, err := docFile.Open()
	if err != nil {
		return "", errors.New("Isi DOCX tidak dapat dibaca.")
	}
	defer rc.Close()
	xmlBytes, err := io.ReadAll(io.LimitReader(rc, 32<<20))
	if err != nil {
		return "", errors.New("Isi DOCX tidak dapat dibaca.")
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
			return "", errors.New("Struktur XML di dalam DOCX tidak valid.")
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
				if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
			}
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

func isHTTPURL(raw string) bool {
	u, err := urlParse(raw)
	if err != nil {
		return false
	}
	return u == "http" || u == "https"
}

func urlParse(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	idx := strings.Index(s, "://")
	if idx <= 0 {
		return "", errors.New("no scheme")
	}
	return strings.ToLower(s[:idx]), nil
}

func sanitizeExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 || idx == len(name)-1 {
		return "bin"
	}
	ext := strings.ToLower(strings.TrimSpace(name[idx+1:]))
	if len(ext) == 0 || len(ext) > 5 {
		return "bin"
	}
	for i := 0; i < len(ext); i++ {
		c := ext[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return "bin"
		}
	}
	return ext
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
