package platform

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CRUD angle Taut + versi prompt + test (writer generate + gatekeeper analyse).

// TautAngleGet: GET /v1/platform/taut/angles/{angleId}
func (d *Deps) TautAngleGet(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id, channel_id, name, slug, description, source_notes, active, sort_order, created_by, created_at FROM taut_content_angles WHERE id = $1 LIMIT 1",
		chiParam(r, "angleId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Angle tidak ditemukan."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"angle": ptRow(ptNormalizeMap(row))})
}

// TautAngleUpdate: PATCH /v1/platform/taut/angles/{angleId}
func (d *Deps) TautAngleUpdate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	angleID := chiParam(r, "angleId")
	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	description, hasDescription := strField(body, "description")
	sourceNotes, hasSourceNotes := strField(body, "source_notes")
	active, hasActive := boolField(body, "active")
	sortOrder, hasSort := intField(body, "sort_order")
	if hasName && (name == "" || len(name) > 160) {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if hasDescription && len(description) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if hasSourceNotes && len(sourceNotes) > 4000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	row, err := d.queryRow(r.Context(), "SELECT id FROM taut_content_angles WHERE id = $1 LIMIT 1", angleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Angle tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(),
		`UPDATE taut_content_angles SET name = COALESCE($1, name), description = COALESCE($2, description),
		        source_notes = COALESCE($3, source_notes),
		        active = COALESCE($4, active), sort_order = COALESCE($5, sort_order), updated_by = $6, updated_at = NOW()
		 WHERE id = $7`,
		ptNullableArgStr(name, hasName), ptNullableArgStr(description, hasDescription),
		ptNullableArgStr(sourceNotes, hasSourceNotes),
		ptNullableArgBool(active, hasActive), ptNullableArgInt(sortOrder, hasSort), user.ID, angleID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautAngleDelete: DELETE /v1/platform/taut/angles/{angleId}
func (d *Deps) TautAngleDelete(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	angleID := chiParam(r, "angleId")
	row, err := d.queryRow(r.Context(), "SELECT id FROM taut_content_angles WHERE id = $1 LIMIT 1", angleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Angle tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM taut_content_angles WHERE id = $1", angleID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.angle_deleted", "taut_content_angle", angleID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- versi prompt angle ----------

// TautAngleVersions: GET /v1/platform/taut/angles/{angleId}/versions
func (d *Deps) TautAngleVersions(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT id, content_angle_id, version, status, prompt, change_note, created_by, created_at, published_by, published_at
		 FROM taut_content_angle_prompt_versions
		 WHERE content_angle_id = $1
		 ORDER BY version DESC`, chiParam(r, "angleId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": ptRows(rows)})
}

// TautAngleVersionCreate: POST /v1/platform/taut/angles/{angleId}/versions
func (d *Deps) TautAngleVersionCreate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	angleID := chiParam(r, "angleId")
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
	angle, err := d.queryRow(r.Context(), "SELECT id FROM taut_content_angles WHERE id = $1 LIMIT 1", angleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if angle == nil {
		writeErr(w, d, errNotFound("Angle tidak ditemukan."))
		return
	}
	maxRow, err := d.queryRow(r.Context(),
		"SELECT COALESCE(MAX(version), 0) AS m FROM taut_content_angle_prompt_versions WHERE content_angle_id = $1", angleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	nextVersion := maxRow.intOr("m", 0) + 1
	if cloneActive && prompt == "" {
		active, aErr := d.queryRow(r.Context(),
			"SELECT prompt FROM taut_content_angle_prompt_versions WHERE content_angle_id = $1 AND status = 'active' LIMIT 1", angleID)
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
		if err := tx.exec("UPDATE taut_content_angle_prompt_versions SET status = 'archived' WHERE content_angle_id = $1 AND status = 'active'", angleID); err != nil {
			return err
		}
		return tx.exec(
			`INSERT INTO taut_content_angle_prompt_versions
			   (id, content_angle_id, version, status, prompt, change_note, created_by, published_at, published_by)
			 VALUES ($1, $2, $3, 'active', $4, $5, $6, NOW(), $7)`,
			versionID, angleID, nextVersion, prompt, changeNote, user.ID, user.ID)
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": versionID, "version": nextVersion})
}

// TautAngleVersionGet: GET /v1/platform/taut/angles/{angleId}/versions/{versionId}
func (d *Deps) TautAngleVersionGet(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	row, err := d.queryRow(r.Context(),
		`SELECT id, content_angle_id, version, status, prompt, change_note, created_at, published_at
		 FROM taut_content_angle_prompt_versions
		 WHERE id = $1 AND content_angle_id = $2 LIMIT 1`, chiParam(r, "versionId"), chiParam(r, "angleId"))
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

// TautAngleVersionUpdate: PATCH /v1/platform/taut/angles/{angleId}/versions/{versionId}
func (d *Deps) TautAngleVersionUpdate(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	angleID := chiParam(r, "angleId")
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
		"SELECT id FROM taut_content_angle_prompt_versions WHERE id = $1 AND content_angle_id = $2 LIMIT 1", versionID, angleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi prompt tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE taut_content_angle_prompt_versions SET prompt = COALESCE($1, prompt), change_note = COALESCE($2, change_note) WHERE id = $3",
		ptNullableArgStr(prompt, hasPrompt), ptNullableArgStr(changeNote, hasChangeNote), versionID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautAngleVersionDelete: DELETE /v1/platform/taut/angles/{angleId}/versions/{versionId}
func (d *Deps) TautAngleVersionDelete(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	angleID := chiParam(r, "angleId")
	versionID := chiParam(r, "versionId")
	row, err := d.queryRow(r.Context(),
		"SELECT id, status FROM taut_content_angle_prompt_versions WHERE id = $1 AND content_angle_id = $2 LIMIT 1", versionID, angleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi prompt tidak ditemukan."))
		return
	}
	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*)::int AS c FROM taut_content_angle_prompt_versions WHERE content_angle_id = $1", angleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if countRow.intOr("c", 0) <= 1 {
		writeErr(w, d, errConflict("Tidak bisa menghapus satu-satunya versi."))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM taut_content_angle_prompt_versions WHERE id = $1", versionID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.angle_prompt_deleted", "taut_content_angle_prompt_version", versionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautAngleVersionActivate: POST /v1/platform/taut/angles/{angleId}/versions/{versionId}/activate
func (d *Deps) TautAngleVersionActivate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	angleID := chiParam(r, "angleId")
	versionID := chiParam(r, "versionId")
	row, err := d.queryRow(r.Context(),
		"SELECT id, status FROM taut_content_angle_prompt_versions WHERE id = $1 AND content_angle_id = $2 LIMIT 1", versionID, angleID)
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
		if err := tx.exec("UPDATE taut_content_angle_prompt_versions SET status = 'archived' WHERE content_angle_id = $1 AND status = 'active'", angleID); err != nil {
			return err
		}
		return tx.exec("UPDATE taut_content_angle_prompt_versions SET status = 'active', published_by = $1, published_at = NOW() WHERE id = $2", user.ID, versionID)
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.angle_prompt_published", "taut_content_angle_prompt_version", versionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- test angle (writer generate + gatekeeper analyse) ----------

var tautAngleTestRate = newTTLCache(15 * time.Second)

// TautAngleTest: POST /v1/platform/taut/angles/{angleId}/test
func (d *Deps) TautAngleTest(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	angleID := chiParam(r, "angleId")
	body := decodeJSON(r)
	bahan, hasBahan := strField(body, "bahan")
	if !hasBahan || len(bahan) < 1 {
		writeErr(w, d, errUnprocessable("Bahan tidak boleh kosong."))
		return
	}
	if len(bahan) > 8000 {
		writeErr(w, d, errUnprocessable("Bahan maksimal 8000 karakter."))
		return
	}
	promptOverride := ""
	if s, hasStr := strField(body, "promptOverride"); hasStr {
		if len(s) > 20000 {
			writeErr(w, d, errUnprocessable(ptZStringTooBig(20000)))
			return
		}
		promptOverride = s
	}

	rlKey := user.ID + ":" + angleID
	if _, hit := tautAngleTestRate.get(rlKey); hit {
		writeErr(w, d, ptErrTooMany("Terlalu sering. Coba lagi dalam 15 detik."))
		return
	}
	tautAngleTestRate.set(rlKey, true)

	angle, err := d.queryRow(r.Context(),
		`SELECT a.id, a.name, a.slug, a.channel_id, c.slug AS channel_slug, c.name AS channel_name
		 FROM taut_content_angles a JOIN taut_media_channels c ON c.id = a.channel_id WHERE a.id = $1 LIMIT 1`, angleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if angle == nil {
		writeErr(w, d, errNotFound("Angle tidak ditemukan."))
		return
	}

	promptText := strings.TrimSpace(promptOverride)
	var promptVersion map[string]any
	if promptText == "" {
		active, aErr := d.queryRow(r.Context(),
			"SELECT id, version, prompt FROM taut_content_angle_prompt_versions WHERE content_angle_id = $1 AND status = 'active' LIMIT 1", angleID)
		if aErr != nil {
			writeErr(w, d, aErr)
			return
		}
		if active == nil || strings.TrimSpace(active.strOr("prompt", "")) == "" {
			writeErr(w, d, errUnprocessable("Prompt angle belum ada. Isi prompt dulu di textarea atau simpan sebagai versi aktif."))
			return
		}
		promptText = active.strOr("prompt", "")
		promptVersion = map[string]any{"id": active.strOr("id", ""), "version": int(active.numOr("version"))}
	}

	base, _ := d.queryRow(r.Context(), "SELECT prompt FROM taut_profiler_base_prompt_versions WHERE status = 'active' LIMIT 1")
	basePrompt := ""
	if base != nil {
		basePrompt = base.strOr("prompt", "")
	}

	channelSlug := angle.strOr("channel_slug", "")
	angleSlug := angle.strOr("slug", "")
	genUserPrompt := strings.Join([]string{
		"[CHANNEL: " + channelSlug + "][ANGLE: " + angleSlug + "][PROFILER INSTRUCTION]\n" + promptText,
		"[SOURCE MATERIAL — bahan uji]\n" + bahan,
		"[OUTPUT REQUIREMENT]\nBuat SATU objek JSON valid tanpa teks lain dengan struktur:\n" + ptJSONIndent2(map[string]string{
			"title":                 "string — judul artikel",
			"content_html":          "string — isi artikel dalam HTML (boleh pakai <p>, <h2>, <ul>/<li>, <strong>, <em>)",
			"angle_adherence_notes": "string — catatan singkat ketaatan angle (opsional)",
		}),
	}, "\n\n---\n\n")

	genResult, genErr := d.callAgentLLM(r.Context(), "writer", genUserPrompt, true, basePrompt, 0)
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
	analysisPrompt := strings.Join([]string{
		"Kamu adalah auditor kepatuhan angle konten. Tugas: nilai apakah artikel di bawah ini benar-benar sesuai dengan instruksi angle.",
		"[ANGLE INSTRUCTION]\n" + promptText,
		"[ARTIKEL]\nTitle: " + articleTitle + "\n" + ptTruncate(plainArticle, 6000),
		"[BAHAN UJI]\n" + ptTruncate(bahan, 4000),
		"[OUTPUT REQUIREMENT]\nKembalikan SATU objek JSON valid tanpa teks lain:\n" + ptJSONIndent2(map[string]any{
			"compliant":   "boolean — true jika artikel secara keseluruhan sesuai angle",
			"score":       "number 0-100 — skor kepatuhan",
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
		"angle": map[string]any{
			"id": angle.strOr("id", ""), "name": angle.strOr("name", ""),
			"slug": angleSlug, "channel": channelSlug,
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

// ---------- util test angle ----------

func ptStringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, isString := v.(string); isString {
		return s
	}
	return ""
}

func ptBoolOf(m map[string]any, key string) bool {
	if b, ok := m[key].(bool); ok {
		return b
	}
	return false
}

func ptNumOfAny(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	}
	return 0
}

func ptClampScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func ptArrayOfAny(v any) any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{}
}

func ptAnalysisRaw(res *tautLLMResult) any {
	if res == nil {
		return nil
	}
	return ptTruncate(res.Content, 4000)
}
