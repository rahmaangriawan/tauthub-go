package profiler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	briefdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brief"
	briefsummary "github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Pustaka template brief: simpan isi satu Profiler sebagai template, lalu
// terapkan ke brand mana pun yang boleh ditulis pemanggil.
//
// Isi template adalah SNAPSHOT empat kolom jsonb + instructions, bukan tautan
// ke profil sumber. Dengan begitu mengubah profil asal tidak diam-diam mengubah
// brief yang sudah lahir dari template itu.

// profileCopy: isi Profiler yang bisa dipindahkan ke baris baru, baik saat
// duplikasi maupun saat menerapkan template. entity_core ikut karena Instan
// Brief menyimpan jawaban formulir di sana; tanpa kolom ini salinan lahir kosong
// dan progresnya 0%.
type profileCopy struct {
	Name         string
	Description  *string
	EntityCore   []byte
	Identity     []byte
	Style        []byte
	SEO          []byte
	Instructions *string
}

// jsonOrEmpty: kolom jsonb NOT NULL tidak menerima string kosong. Sumber normal
// selalu berisi minimal "{}", tetapi baris lama bisa saja NULL.
func jsonOrEmpty(raw []byte) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

// insertProfileCopy: satu baris profiler_profiles baru di dalam transaksi
// pemanggil. short_code tetap diulang, tetapi lewat savepoint: unique violation
// membatalkan seluruh transaksi Postgres, jadi retry tanpa savepoint tidak akan
// pernah berhasil.
func insertProfileCopy(ctx context.Context, tx pgx.Tx, brandID string, src profileCopy, userID string) (string, string, error) {
	id := uuid.NewString()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		sp, err := tx.Begin(ctx) // savepoint, bukan transaksi baru
		if err != nil {
			return "", "", err
		}
		code := shortCodeFromUUID()
		_, err = sp.Exec(ctx, `
			INSERT INTO profiler_profiles
			  (id, brand_id, name, description, entity_core, identity, style, seo, instructions, short_code, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			id, brandID, src.Name, src.Description,
			jsonOrEmpty(src.EntityCore), jsonOrEmpty(src.Identity), jsonOrEmpty(src.Style), jsonOrEmpty(src.SEO),
			src.Instructions, code, userID)
		if err == nil {
			return id, code, sp.Commit(ctx)
		}
		lastErr = err
		_ = sp.Rollback(ctx)
	}
	return "", "", lastErr
}

// copyProfileAssets: salin blok knowledge dan state field dari Profiler sumber.
//
// chat_summary sengaja TIDAK disalin: percakapan milik brief asal, dan membawa
// ringkasannya ke brief baru membuat model mengira topik lama masih berlaku.
// instructions justru disalin karena itu bagian dari identitas brief, bukan
// percakapan.
func copyProfileAssets(ctx context.Context, tx pgx.Tx, srcProfileID, newProfileID string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO profiler_blocks (id, profile_id, title, kind, content, note, sort_order)
		SELECT gen_random_uuid()::text, $1, title, kind, content, note, sort_order
		FROM profiler_blocks WHERE profile_id = $2`,
		newProfileID, srcProfileID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO taut_brief_field_state (profile_id, field_key, status, source, note, updated_at)
		SELECT $1, field_key, status, source, note, NOW()
		FROM taut_brief_field_state WHERE profile_id = $2
		ON CONFLICT (profile_id, field_key) DO NOTHING`,
		newProfileID, srcProfileID)
	return err
}

// templateFilledKeys: field_key yang nilainya tidak kosong di template.
//
// Key dibatasi ke field skema aktif supaya {{brief_state}} dan progres hanya
// memuat field yang dikenal. Kalau skema belum ada sama sekali, semua key berisi
// tetap disemai apa adanya; lebih baik brief punya state daripada kosong total.
func templateFilledKeys(src profileCopy, fields []briefdomain.Field) []string {
	cols := map[string]map[string]any{
		"entity_core": asMap(src.EntityCore),
		"identity":    asMap(src.Identity),
		"style":       asMap(src.Style),
		"seo":         asMap(src.SEO),
	}
	seen := map[string]bool{}
	out := []string{}
	add := func(key string) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, key)
	}

	if len(fields) == 0 {
		for _, col := range []string{"entity_core", "identity", "style", "seo"} {
			for key, value := range cols[col] {
				if !emptyValue(value) {
					add(key)
				}
			}
		}
		return out
	}

	for _, f := range fields {
		if f.Layer == 3 || !f.Enabled {
			continue
		}
		col, ok := briefdomain.StorageColumn(f)
		if !ok {
			continue
		}
		if value, ok := cols[col][f.Key]; ok && !emptyValue(value) {
			add(f.Key)
		}
	}
	return out
}

// emptyValue: nilai jsonb dianggap kosong bila string kosong, array kosong, atau
// objek kosong. Angka dan boolean apa pun dihitung terisi (false pun jawaban).
func emptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		for _, it := range t {
			if !emptyValue(it) {
				return false
			}
		}
		return true
	case map[string]any:
		for _, it := range t {
			if !emptyValue(it) {
				return false
			}
		}
		return true
	}
	return false
}

// seedFieldStates: tandai field template sebagai terisi oleh user. Template
// memang diisi manusia saat disimpan, bukan hasil AI, jadi source-nya 'user'.
func seedFieldStates(ctx context.Context, tx pgx.Tx, profileID string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO taut_brief_field_state (profile_id, field_key, status, source, updated_at)
		SELECT $1, k, 'filled', 'user', NOW() FROM unnest($2::text[]) AS k
		ON CONFLICT (profile_id, field_key) DO NOTHING`,
		profileID, keys)
	return err
}

// audit: catat audit_logs dengan brand_id (best-effort; kegagalan tidak
// menggagalkan aksi yang sudah tersimpan, sama seperti helper di brandflow).
func (d *Deps) audit(ctx context.Context, brandID, actorUserID, action, targetType, targetID string) {
	if _, err := d.Pool.Exec(ctx, `
		INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)`,
		nullIfEmpty(brandID), nullIfEmpty(actorUserID), action, targetType, targetID); err != nil {
		slog.Warn("[profiler] audit gagal", "action", action, "err", err)
	}
}

// SaveAsTemplate: POST /v1/brands/{brandId}/profiler/profiles/{profileId}/save-as-template
// Body {name, description, workspace_id?}.
func (d *Deps) SaveAsTemplate(w http.ResponseWriter, r *http.Request) {
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
	name, _ := strField(body, "name")
	if name == "" || len(name) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Nama template wajib diisi (maks 160 karakter).")
		return
	}
	description, _ := strField(body, "description")
	workspaceID, _ := strField(body, "workspace_id")
	ctx := r.Context()

	// Template ber-workspace hanya boleh dibuat anggota workspace itu, dan
	// brand sumbernya harus benar-benar milik workspace tersebut. Tanpa cek
	// kedua, anggota workspace A bisa menempelkan template brand B ke A.
	if workspaceID != "" {
		var ok bool
		_ = d.Pool.QueryRow(ctx, `
			SELECT true FROM agency_workspaces w
			JOIN brands b ON b.workspace_id = w.id
			WHERE w.id = $1 AND w.deleted_at IS NULL AND b.id = $2 AND b.deleted_at IS NULL
			  AND (w.owner_user_id = $3 OR EXISTS (
			        SELECT 1 FROM workspace_members wm
			        WHERE wm.workspace_id = w.id AND wm.user_id = $3))
			LIMIT 1`, workspaceID, brandID, user.ID).Scan(&ok)
		if !ok {
			// 404, bukan 403: workspace yang tidak boleh dilihat tidak boleh
			// membocorkan keberadaannya.
			httpx.Error(w, http.StatusNotFound, "Workspace tidak ditemukan.")
			return
		}
	}

	var src profileCopy
	if err := d.Pool.QueryRow(ctx, `
		SELECT entity_core, identity, style, seo, instructions
		FROM profiler_profiles WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL`,
		profileID, brandID).Scan(&src.EntityCore, &src.Identity, &src.Style, &src.SEO, &src.Instructions); err != nil {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}

	id := uuid.NewString()
	if _, err := d.Pool.Exec(ctx, `
		INSERT INTO brief_templates
		  (id, workspace_id, brand_id, name, description, entity_core, identity, style, seo, instructions, created_by)
		VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''), $6, $7, $8, $9, $10, $11)`,
		id, workspaceID, brandID, name, description,
		jsonOrEmpty(src.EntityCore), jsonOrEmpty(src.Identity), jsonOrEmpty(src.Style), jsonOrEmpty(src.SEO),
		src.Instructions, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	d.audit(ctx, brandID, user.ID, "brief.template_saved", "taut_brief", id)
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

// ListTemplates: GET /v1/templates?brandId=...
//
// Terlihat = brand-nya diikuti pemanggil ATAU workspace-nya diikuti pemanggil.
// brandId hanya filter tampilan, bukan sumber izin: mengirim brandId brand lain
// menghasilkan daftar kosong, bukan data brand itu.
func (d *Deps) ListTemplates(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandFilter := strings.TrimSpace(r.URL.Query().Get("brandId"))
	rows, err := d.Pool.Query(r.Context(), `
		SELECT t.id, t.brand_id, t.workspace_id, t.name, t.description, b.name, t.created_at
		FROM brief_templates t
		LEFT JOIN brands b ON b.id = t.brand_id
		WHERE t.deleted_at IS NULL
		  AND ($2 = '' OR t.brand_id = $2)
		  AND (
		    EXISTS (SELECT 1 FROM brand_memberships bm WHERE bm.brand_id = t.brand_id AND bm.user_id = $1)
		    OR EXISTS (SELECT 1 FROM workspace_members wm WHERE wm.workspace_id = t.workspace_id AND wm.user_id = $1)
		    OR EXISTS (SELECT 1 FROM agency_workspaces w
		               WHERE w.id = t.workspace_id AND w.owner_user_id = $1 AND w.deleted_at IS NULL)
		  )
		ORDER BY t.created_at DESC`, user.ID, brandFilter)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var (
			id, name             string
			brandID, workspaceID *string
			description          *string
			sourceBrand          *string
			createdAt            any
		)
		if err := rows.Scan(&id, &brandID, &workspaceID, &name, &description, &sourceBrand, &createdAt); err != nil {
			continue
		}
		items = append(items, map[string]any{
			"id": id, "brand_id": brandID, "workspace_id": workspaceID,
			"name": name, "description": description,
			"source_brand_name": sourceBrand, "created_at": createdAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": items})
}

// ApplyTemplate: POST /v1/templates/{templateId}/apply  Body {brandId, name}.
//
// Seluruh langkah (profil + state field) satu transaksi. Brief setengah jadi
// lebih berbahaya daripada gagal total: UI akan menampilkan progres sebagian
// yang terlihat sah.
func (d *Deps) ApplyTemplate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	body := decodeJSON(r)
	brandID, _ := strField(body, "brandId")
	if brandID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "brandId wajib diisi.")
		return
	}
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	ctx := r.Context()

	var src profileCopy
	if err := d.Pool.QueryRow(ctx, `
		SELECT t.name, t.description, t.entity_core, t.identity, t.style, t.seo, t.instructions
		FROM brief_templates t
		WHERE t.id = $1 AND t.deleted_at IS NULL
		  AND (
		    EXISTS (SELECT 1 FROM brand_memberships bm WHERE bm.brand_id = t.brand_id AND bm.user_id = $2)
		    OR EXISTS (SELECT 1 FROM workspace_members wm WHERE wm.workspace_id = t.workspace_id AND wm.user_id = $2)
		    OR EXISTS (SELECT 1 FROM agency_workspaces w
		               WHERE w.id = t.workspace_id AND w.owner_user_id = $2 AND w.deleted_at IS NULL)
		  )`, chiParam(r, "templateId"), user.ID).
		Scan(&src.Name, &src.Description, &src.EntityCore, &src.Identity, &src.Style, &src.SEO, &src.Instructions); err != nil {
		httpx.Error(w, http.StatusNotFound, "Template tidak ditemukan.")
		return
	}
	// Nama opsional: tanpa nama, brief mewarisi nama template. Nama di body
	// menang supaya satu template bisa dipakai berkali-kali di brand yang sama.
	if name, ok := strField(body, "name"); ok && name != "" {
		src.Name = name
	}
	if len(src.Name) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Nama Profiler maksimal 160 karakter.")
		return
	}

	fields, _ := d.briefRequiredFields(ctx)
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer tx.Rollback(ctx)

	newID, shortCode, err := insertProfileCopy(ctx, tx, brandID, src, user.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := seedFieldStates(ctx, tx, newID, templateFilledKeys(src, fields)); err != nil {
		writeErr(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeErr(w, err)
		return
	}
	// Audit dan ringkasan setelah commit: keduanya membaca baris yang baru saja
	// ditulis, jadi tidak boleh berjalan sebelum transaksi selesai.
	d.audit(ctx, brandID, user.ID, "brief.template_applied", "taut_brief", newID)
	d.queueBriefSummary(ctx, brandID, newID, briefsummary.KindEntity, briefsummary.KindFile)
	writeJSON(w, http.StatusOK, map[string]any{"id": newID, "short_code": shortCode})
}

// DeleteTemplate: DELETE /v1/templates/{templateId} (soft)
//
// Hanya pembuat atau admin workspace. Template yang tidak terlihat pemanggil
// juga menghasilkan 404, sehingga tidak ada perbedaan sinyal antara "tidak ada"
// dan "tidak boleh".
func (d *Deps) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	templateID := chiParam(r, "templateId")
	var brandID *string
	err := d.Pool.QueryRow(r.Context(), `
		SELECT t.brand_id FROM brief_templates t
		WHERE t.id = $1 AND t.deleted_at IS NULL
		  AND (
		    t.created_by = $2
		    OR EXISTS (SELECT 1 FROM agency_workspaces w
		               WHERE w.id = t.workspace_id AND w.deleted_at IS NULL
		                 AND (w.owner_user_id = $2 OR EXISTS (
		                       SELECT 1 FROM workspace_members wm
		                       WHERE wm.workspace_id = w.id AND wm.user_id = $2
		                         AND wm.role IN ('owner', 'admin'))))
		  )`, templateID, user.ID).Scan(&brandID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Template tidak ditemukan.")
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE brief_templates SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", templateID); err != nil {
		writeErr(w, err)
		return
	}
	auditBrand := ""
	if brandID != nil {
		auditBrand = *brandID
	}
	d.audit(r.Context(), auditBrand, user.ID, "brief.template_deleted", "taut_brief", templateID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
