package platform

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CRUD jenis artikel Taut (taut_article_types) + versi prompt + test.
// Pola sama dengan angle (taut_angles.go): prompt berversi, satu versi aktif.

// TautArticleTypesList: GET /v1/platform/taut/article-types
func (d *Deps) TautArticleTypesList(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT t.id, t.code, t.name, t.description, t.active, t.sort_order,
		        v.id AS active_version_id, v.version AS active_version
		 FROM taut_article_types t
		 LEFT JOIN taut_article_type_prompt_versions v
		   ON v.article_type_id = t.id AND v.status = 'active'
		 ORDER BY t.sort_order, t.name`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": ptRows(rows)})
}

// TautArticleTypeGet: GET /v1/platform/taut/article-types/{typeId}
func (d *Deps) TautArticleTypeGet(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id, code, name, description, active, sort_order, created_by, created_at FROM taut_article_types WHERE id = $1 LIMIT 1",
		chiParam(r, "typeId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Jenis artikel tidak ditemukan."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"article_type": ptRow(ptNormalizeMap(row))})
}

// TautArticleTypeCreate: POST /v1/platform/taut/article-types
func (d *Deps) TautArticleTypeCreate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	body := decodeJSON(r)
	code, _ := strField(body, "code")
	name, _ := strField(body, "name")
	description, _ := strField(body, "description")
	active, hasActive := boolField(body, "active")
	sortOrder, hasSort := intField(body, "sort_order")
	if name == "" || len(name) > 120 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if code == "" || len(code) > 50 || !reTautAngleSlug.MatchString(code) {
		writeErr(w, d, errUnprocessable("Kode jenis artikel harus huruf kecil, angka, atau garis bawah."))
		return
	}
	if len(description) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if !hasActive {
		active = true
	}
	if !hasSort {
		sortOrder = 0
	}
	if err := d.exec(r.Context(),
		"INSERT INTO taut_article_types (id, code, name, description, active, sort_order, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		uuid.NewString(), code, name, description, active, sortOrder, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautArticleTypeUpdate: PATCH /v1/platform/taut/article-types/{typeId}
func (d *Deps) TautArticleTypeUpdate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	typeID := chiParam(r, "typeId")
	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	description, hasDescription := strField(body, "description")
	active, hasActive := boolField(body, "active")
	sortOrder, hasSort := intField(body, "sort_order")
	if hasName && (name == "" || len(name) > 120) {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if hasDescription && len(description) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	row, err := d.queryRow(r.Context(), "SELECT id FROM taut_article_types WHERE id = $1 LIMIT 1", typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Jenis artikel tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(),
		`UPDATE taut_article_types SET name = COALESCE($1, name), description = COALESCE($2, description),
		        active = COALESCE($3, active), sort_order = COALESCE($4, sort_order), updated_by = $5, updated_at = NOW()
		 WHERE id = $6`,
		ptNullableArgStr(name, hasName), ptNullableArgStr(description, hasDescription),
		ptNullableArgBool(active, hasActive), ptNullableArgInt(sortOrder, hasSort), user.ID, typeID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautArticleTypeDelete: DELETE /v1/platform/taut/article-types/{typeId}
func (d *Deps) TautArticleTypeDelete(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	typeID := chiParam(r, "typeId")
	row, err := d.queryRow(r.Context(), "SELECT id FROM taut_article_types WHERE id = $1 LIMIT 1", typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Jenis artikel tidak ditemukan."))
		return
	}
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if err := tx.exec("DELETE FROM taut_article_type_prompt_versions WHERE article_type_id = $1", typeID); err != nil {
			return err
		}
		return tx.exec("DELETE FROM taut_article_types WHERE id = $1", typeID)
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.article_type_deleted", "taut_article_type", typeID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- versi prompt jenis artikel ----------

// TautArticleTypeVersions: GET /v1/platform/taut/article-types/{typeId}/versions
func (d *Deps) TautArticleTypeVersions(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT id, article_type_id, version, status, prompt, change_note, created_by, created_at, published_by, published_at
		 FROM taut_article_type_prompt_versions
		 WHERE article_type_id = $1
		 ORDER BY version DESC`, chiParam(r, "typeId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": ptRows(rows)})
}

// TautArticleTypeVersionCreate: POST /v1/platform/taut/article-types/{typeId}/versions
func (d *Deps) TautArticleTypeVersionCreate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	typeID := chiParam(r, "typeId")
	body := decodeJSON(r)
	prompt, _ := strField(body, "prompt")
	cloneActive, _ := boolField(body, "clone_active")
	changeNote, _ := strField(body, "change_note")
	if len(prompt) > 20000 {
		writeErr(w, d, errUnprocessable(ptZStringTooBig(20000)))
		return
	}
	if len(changeNote) > 255 {
		writeErr(w, d, errUnprocessable(ptZStringTooBig(255)))
		return
	}
	typeRow, err := d.queryRow(r.Context(), "SELECT id FROM taut_article_types WHERE id = $1 LIMIT 1", typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if typeRow == nil {
		writeErr(w, d, errNotFound("Jenis artikel tidak ditemukan."))
		return
	}
	maxRow, err := d.queryRow(r.Context(),
		"SELECT COALESCE(MAX(version), 0) AS m FROM taut_article_type_prompt_versions WHERE article_type_id = $1", typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	nextVersion := maxRow.intOr("m", 0) + 1
	if cloneActive && prompt == "" {
		active, aErr := d.queryRow(r.Context(),
			"SELECT prompt FROM taut_article_type_prompt_versions WHERE article_type_id = $1 AND status = 'active' LIMIT 1", typeID)
		if aErr != nil {
			writeErr(w, d, aErr)
			return
		}
		if active != nil {
			prompt = active.strOr("prompt", "")
		}
	}
	versionID := uuid.NewString()
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if err := tx.exec("UPDATE taut_article_type_prompt_versions SET status = 'archived' WHERE article_type_id = $1 AND status = 'active'", typeID); err != nil {
			return err
		}
		return tx.exec(
			`INSERT INTO taut_article_type_prompt_versions
			   (id, article_type_id, version, status, prompt, change_note, created_by, published_at, published_by)
			 VALUES ($1, $2, $3, 'active', $4, $5, $6, NOW(), $7)`,
			versionID, typeID, nextVersion, prompt, changeNote, user.ID, user.ID)
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": versionID, "version": nextVersion})
}

// TautArticleTypeVersionGet: GET /v1/platform/taut/article-types/{typeId}/versions/{versionId}
func (d *Deps) TautArticleTypeVersionGet(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	row, err := d.queryRow(r.Context(),
		`SELECT id, article_type_id, version, status, prompt, change_note, created_at, published_at
		 FROM taut_article_type_prompt_versions
		 WHERE id = $1 AND article_type_id = $2 LIMIT 1`, chiParam(r, "versionId"), chiParam(r, "typeId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi prompt tidak ditemukan."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": ptRow(ptNormalizeMap(row))})
}

// TautArticleTypeVersionUpdate: PATCH /v1/platform/taut/article-types/{typeId}/versions/{versionId}
func (d *Deps) TautArticleTypeVersionUpdate(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	typeID := chiParam(r, "typeId")
	versionID := chiParam(r, "versionId")
	body := decodeJSON(r)
	prompt, hasPrompt := strField(body, "prompt")
	changeNote, hasChangeNote := strField(body, "change_note")
	if hasPrompt && len(prompt) > 20000 {
		writeErr(w, d, errUnprocessable(ptZStringTooBig(20000)))
		return
	}
	if hasChangeNote && len(changeNote) > 255 {
		writeErr(w, d, errUnprocessable(ptZStringTooBig(255)))
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id FROM taut_article_type_prompt_versions WHERE id = $1 AND article_type_id = $2 LIMIT 1", versionID, typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi prompt tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE taut_article_type_prompt_versions SET prompt = COALESCE($1, prompt), change_note = COALESCE($2, change_note) WHERE id = $3",
		ptNullableArgStr(prompt, hasPrompt), ptNullableArgStr(changeNote, hasChangeNote), versionID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautArticleTypeVersionDelete: DELETE /v1/platform/taut/article-types/{typeId}/versions/{versionId}
func (d *Deps) TautArticleTypeVersionDelete(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	typeID := chiParam(r, "typeId")
	versionID := chiParam(r, "versionId")
	row, err := d.queryRow(r.Context(),
		"SELECT id, status FROM taut_article_type_prompt_versions WHERE id = $1 AND article_type_id = $2 LIMIT 1", versionID, typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi prompt tidak ditemukan."))
		return
	}
	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*)::int AS c FROM taut_article_type_prompt_versions WHERE article_type_id = $1", typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if countRow.intOr("c", 0) <= 1 {
		writeErr(w, d, errConflict("Tidak bisa menghapus satu-satunya versi."))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM taut_article_type_prompt_versions WHERE id = $1", versionID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.article_type_prompt_deleted", "taut_article_type_prompt_version", versionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautArticleTypeVersionActivate: POST /v1/platform/taut/article-types/{typeId}/versions/{versionId}/activate
func (d *Deps) TautArticleTypeVersionActivate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	typeID := chiParam(r, "typeId")
	versionID := chiParam(r, "versionId")
	row, err := d.queryRow(r.Context(),
		"SELECT id, status FROM taut_article_type_prompt_versions WHERE id = $1 AND article_type_id = $2 LIMIT 1", versionID, typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi prompt tidak ditemukan."))
		return
	}
	if row.strOr("status", "") != "draft" {
		writeErr(w, d, errConflict("Hanya versi draft yang dapat dipublish."))
		return
	}
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if err := tx.exec("UPDATE taut_article_type_prompt_versions SET status = 'archived' WHERE article_type_id = $1 AND status = 'active'", typeID); err != nil {
			return err
		}
		return tx.exec("UPDATE taut_article_type_prompt_versions SET status = 'active', published_by = $1, published_at = NOW() WHERE id = $2", user.ID, versionID)
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.article_type_prompt_published", "taut_article_type_prompt_version", versionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- test jenis artikel (generate + analisis genre) ----------

var tautArticleTypeTestRate = newTTLCache(15 * time.Second)

// TautArticleTypeTest: POST /v1/platform/taut/article-types/{typeId}/test
func (d *Deps) TautArticleTypeTest(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	typeID := chiParam(r, "typeId")
	body := decodeJSON(r)
	bahan, hasBahan := strField(body, "bahan")
	if !hasBahan || len(strings.TrimSpace(bahan)) < 1 {
		writeErr(w, d, errUnprocessable("Bahan tidak boleh kosong."))
		return
	}
	if len(bahan) > 8000 {
		writeErr(w, d, errUnprocessable("Bahan maksimal 8000 karakter."))
		return
	}
	promptOverride := ""
	usedOverride := false
	if s, hasStr := strField(body, "promptOverride"); hasStr {
		if len(s) > 20000 {
			writeErr(w, d, errUnprocessable(ptZStringTooBig(20000)))
			return
		}
		promptOverride = s
		usedOverride = true
	}

	rlKey := user.ID + ":" + typeID
	if _, hit := tautArticleTypeTestRate.get(rlKey); hit {
		writeErr(w, d, ptErrTooMany("Terlalu sering. Coba lagi dalam 15 detik."))
		return
	}
	tautArticleTypeTestRate.set(rlKey, true)

	typeRow, err := d.queryRow(r.Context(),
		"SELECT id, code, name, description FROM taut_article_types WHERE id = $1 LIMIT 1", typeID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if typeRow == nil {
		writeErr(w, d, errNotFound("Jenis artikel tidak ditemukan."))
		return
	}

	// Prompt jenis artikel dipakai sebagai system prompt pengganti saat generate.
	// Kosong (mis. Editorial) = pakai prompt agent standar.
	systemPrompt := promptOverride
	var promptVersion map[string]any
	if !usedOverride {
		active, aErr := d.queryRow(r.Context(),
			"SELECT id, version, prompt FROM taut_article_type_prompt_versions WHERE article_type_id = $1 AND status = 'active' LIMIT 1", typeID)
		if aErr != nil {
			writeErr(w, d, aErr)
			return
		}
		if active != nil {
			systemPrompt = active.strOr("prompt", "")
			promptVersion = map[string]any{"id": active.strOr("id", ""), "version": int(active.numOr("version"))}
		}
	}

	typeName := typeRow.strOr("name", "")
	typeCode := typeRow.strOr("code", "")
	genUserPrompt := strings.Join([]string{
		"[JENIS ARTIKEL: " + typeName + " (" + typeCode + ")]",
		"[SOURCE MATERIAL — bahan uji]\n" + bahan,
		"[OUTPUT REQUIREMENT]\nBuat SATU objek JSON valid tanpa teks lain dengan struktur:\n" + ptJSONIndent2(map[string]string{
			"title":            "string — judul artikel",
			"content_html":     "string — isi artikel dalam HTML",
			"meta_description": "string — meta deskripsi 120-160 karakter",
		}),
	}, "\n\n---\n\n")

	genResult, genErr := d.callAgentLLM(r.Context(), "writer", genUserPrompt, true, systemPrompt, 0)
	if genErr != nil {
		var notCfg *agentNotConfiguredError
		if errors.As(genErr, &notCfg) {
			writeErr(w, d, ptErr(http.StatusServiceUnavailable, notCfg.Error()+" — atur di /admin/ai atau /admin/taut-agents."))
			return
		}
		writeErr(w, d, genErr)
		return
	}
	genBody := extractJSONObject(genResult.Content)
	if genBody == nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"detail": "Output generate tidak valid (bukan JSON).",
			"raw":    ptTruncate(genResult.Content, 4000),
		})
		return
	}
	articleTitle := strings.TrimSpace(ptStringField(genBody, "title"))
	articleHTML := strings.TrimSpace(ptStringField(genBody, "content_html"))
	if articleHTML == "" {
		articleHTML = strings.TrimSpace(ptStringField(genBody, "content"))
	}
	if articleTitle == "" || articleHTML == "" {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"detail": "Artikel hasil generate tidak lengkap (title/content_html kosong).",
			"raw":    ptTruncate(genResult.Content, 4000),
		})
		return
	}

	plainArticle := strings.Join(strings.Fields(reStripTags.ReplaceAllString(articleHTML, " ")), " ")
	genreInstruction := "Tulis artikel berita/editorial jurnalistik standar."
	if strings.TrimSpace(systemPrompt) != "" {
		genreInstruction = systemPrompt
	}
	analysisPrompt := strings.Join([]string{
		"Kamu adalah auditor gaya artikel. Tugas: nilai apakah artikel di bawah ini benar-benar sesuai dengan instruksi jenis artikel.",
		"[INSTRUKSI JENIS ARTIKEL]\n" + ptTruncate(genreInstruction, 8000),
		"[ARTIKEL]\nTitle: " + articleTitle + "\n" + ptTruncate(plainArticle, 6000),
		"[BAHAN UJI]\n" + ptTruncate(bahan, 4000),
		"[OUTPUT REQUIREMENT]\nKembalikan SATU objek JSON valid tanpa teks lain:\n" + ptJSONIndent2(map[string]any{
			"compliant":   "boolean — true jika artikel secara keseluruhan sesuai jenis artikel",
			"score":       "number 0-100 — skor kesesuaian",
			"summary":     "string — ringkasan 1-2 kalimat",
			"checks":      []map[string]string{{"criterion": "string", "pass": "boolean", "notes": "string"}},
			"issues":      []string{"string — daftar masalah jika ada"},
			"suggestions": []string{"string — saran perbaikan jika ada"},
		}),
	}, "\n\n")

	var analysisResult *tautLLMResult
	var analysisBody map[string]any
	if res, aErr := d.callAgentLLM(r.Context(), "gatekeeper", analysisPrompt, true, "", 0); aErr != nil {
		var notCfg *agentNotConfiguredError
		if errors.As(aErr, &notCfg) {
			if res2, aErr2 := d.callAgentLLM(r.Context(), "writer", analysisPrompt, true, "Kamu auditor konten yang ketat dan objektif.", 0); aErr2 == nil {
				analysisResult = res2
				analysisBody = extractJSONObject(res2.Content)
			}
		}
	} else {
		analysisResult = res
		analysisBody = extractJSONObject(res.Content)
	}

	var analysis any
	if analysisBody != nil {
		analysis = map[string]any{
			"compliant":   ptBoolOf(analysisBody, "compliant"),
			"score":       ptClampScore(ptNumOfAny(analysisBody["score"])),
			"summary":     ptStringField(analysisBody, "summary"),
			"checks":      ptArrayOfAny(analysisBody["checks"]),
			"issues":      ptArrayOfAny(analysisBody["issues"]),
			"suggestions": ptArrayOfAny(analysisBody["suggestions"]),
			"raw":         ptAnalysisRaw(analysisResult),
		}
	}

	versionOut := map[string]any{"id": nil, "version": nil, "usedOverride": true}
	if promptVersion != nil {
		versionOut = map[string]any{"id": promptVersion["id"], "version": promptVersion["version"], "usedOverride": false}
	}
	usage := map[string]any{
		"generate": map[string]any{
			"model": genResult.Model, "input": genResult.Input, "output": genResult.Output,
			"durationMs": genResult.DurationMs,
		},
		"analysis": nil,
	}
	if analysisResult != nil {
		usage["analysis"] = map[string]any{
			"model": analysisResult.Model, "input": analysisResult.Input, "output": analysisResult.Output,
			"durationMs": analysisResult.DurationMs,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"articleType": map[string]any{
			"id": typeRow.strOr("id", ""), "name": typeName, "code": typeCode,
		},
		"promptVersion": versionOut,
		"article": map[string]any{
			"title": articleTitle, "content_html": articleHTML,
			"raw": ptTruncate(genResult.Content, 6000),
		},
		"analysis":   analysis,
		"usage":      usage,
		"durationMs": time.Since(started).Milliseconds(),
	})
}
