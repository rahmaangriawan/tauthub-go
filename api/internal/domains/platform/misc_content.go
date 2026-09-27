package platform

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

// Modul content-intake-types + knowledge-bases + moderations. Port
// app/v1/platform/content-intake-types/**, knowledge-bases/**, moderations/**.
// Semua endpoint isPlatformAdmin (bukan super-admin) → adminGuard tanpa
// permission lalu cukup lolos guard (admin biasa boleh).

// Regex & konstanta modul content.
var (
	reIntakeCode  = regexp.MustCompile(`^[a-z0-9_]+$`)
	reKbCode      = regexp.MustCompile(`^[a-z0-9-]+$`)
	reBlankLine   = regexp.MustCompile(`\n\s*\n`)
	mcSectionMeta = []mcSection{
		{Key: "content_purpose", Label: "Content Purpose", Priority: 1},
		{Key: "news_angle", Label: "News Angle", Priority: 2},
		{Key: "journalistic_style", Label: "Journalistic Style", Priority: 1},
		{Key: "required_information", Label: "Required Information", Priority: 3},
		{Key: "optional_information", Label: "Optional Information", Priority: 4},
		{Key: "structure_guidance", Label: "Structure Guidance", Priority: 2},
		{Key: "lead_guidance", Label: "Lead / Opening Guidance", Priority: 1},
		{Key: "fact_attribution_rules", Label: "Fact & Attribution Rules", Priority: 1},
		{Key: "quote_guidance", Label: "Quote Guidance", Priority: 2},
		{Key: "language_tone", Label: "Language & Tone", Priority: 2},
		{Key: "what_to_avoid", Label: "What to Avoid", Priority: 1},
		{Key: "angle_variation_rules", Label: "Angle Variation Rules", Priority: 1},
		{Key: "source_priority", Label: "Source Priority", Priority: 2},
		{Key: "fact_conflict_handling", Label: "Fact Conflict Handling", Priority: 2},
		{Key: "editorial_quality_checklist", Label: "Editorial Quality Checklist", Priority: 3},
	}
)

type mcSection struct {
	Key      string
	Label    string
	Priority int
}

// mcEmptySections: parity getEmptyEditorialSections.
func mcEmptySections() map[string]string {
	out := make(map[string]string, len(mcSectionMeta))
	for _, s := range mcSectionMeta {
		out[s.Key] = ""
	}
	return out
}

// ---------- content intake types ----------

// mcIntakeTypeJSON: parity intakeTypeJson (questions = parseQuestions).
func mcIntakeTypeJSON(row ptRow) map[string]any {
	return map[string]any{
		"id":          row.strOr("id", ""),
		"code":        row.strOr("code", ""),
		"name":        row.strOr("name", ""),
		"description": ptNullableStr(row, "description"),
		"questions":   mcParseQuestions(row.jsonField("questions")),
		"status":      row.strOr("status", ""),
		"created_at":  row.anyOrNil("created_at"),
		"updated_at":  row.anyOrNil("updated_at"),
	}
}

// mcParseQuestions: parity parseQuestions — hanya objek dgn key & label string.
func mcParseQuestions(value any) []any {
	arr, ok := value.([]any)
	if !ok {
		return []any{}
	}
	out := make([]any, 0, len(arr))
	for _, item := range arr {
		obj, isObj := item.(map[string]any)
		if !isObj {
			continue
		}
		if _, hasKey := obj["key"].(string); !hasKey {
			continue
		}
		if _, hasLabel := obj["label"].(string); !hasLabel {
			continue
		}
		out = append(out, obj)
	}
	return out
}

// mcFindIntakeType: parity findType.
func (d *Deps) mcFindIntakeType(r *http.Request, code string) (ptRow, error) {
	row, err := d.queryRow(r.Context(),
		"SELECT id, code, name, description, questions, status, created_at, updated_at FROM content_intake_types WHERE code = $1 LIMIT 1", code)
	if err != nil || row == nil {
		return nil, err
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// mcIntakeTypeValidation meniru intakeTypeSchema/intakeTypeUpdateSchema.
func mcIntakeTypeValidation(body map[string]any, isCreate bool) error {
	if isCreate {
		code, hasCode := strField(body, "code")
		if !hasCode || code == "" || len(code) > 50 || len(code) < 2 || !reIntakeCode.MatchString(code) {
			return errUnprocessable("Invalid payload")
		}
	}
	if name, hasName := strField(body, "name"); hasName {
		if len(name) < 2 || len(name) > 120 {
			return errUnprocessable("Invalid payload")
		}
	} else if isCreate {
		return errUnprocessable("Invalid payload")
	}
	if desc, hasDesc := rawField(body, "description"); hasDesc && desc != nil {
		s, isString := desc.(string)
		if !isString || len(s) > 2000 {
			return errUnprocessable("Invalid payload")
		}
	}
	if q, hasQ := rawField(body, "questions"); hasQ && q != nil {
		arr, isArr := q.([]any)
		if !isArr || len(arr) > 30 {
			return errUnprocessable("Invalid payload")
		}
	}
	if st, hasSt := strField(body, "status"); hasSt && st != "" {
		if st != "active" && st != "archived" {
			return errUnprocessable("Invalid payload")
		}
	}
	return nil
}

// ContentIntakeTypesList: GET /v1/platform/content-intake-types
func (d *Deps) ContentIntakeTypesList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	search := strings.TrimSpace(q.Get("q"))
	if len(search) > 120 {
		search = search[:120]
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	params := []any{}
	if status == "archived" || status == "active" {
		params = append(params, status)
		where = append(where, "status = $"+ptItoa(len(params)))
	}
	if search != "" {
		like := "%" + search + "%"
		params = append(params, like, like, like)
		where = append(where, "(name ILIKE $"+ptItoa(len(params)-2)+" OR code ILIKE $"+ptItoa(len(params)-1)+" OR description ILIKE $"+ptItoa(len(params))+")")
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM content_intake_types "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	listArgs := append([]any{}, params...)
	listArgs = append(listArgs, limit, offset)
	found, err := d.queryRows(r.Context(),
		`SELECT t.id, t.code, t.name, t.description, t.questions, t.status, t.created_at, t.updated_at,
		        (SELECT v.version FROM content_type_standard_versions v JOIN content_type_standards s ON s.id = v.standard_id WHERE s.content_type_code = t.code AND v.status = 'active' LIMIT 1) AS standard_version,
		        (SELECT s.status FROM content_type_standards s WHERE s.content_type_code = t.code LIMIT 1) AS standard_status,
		        (SELECT COUNT(*) FROM content_type_standard_chunks c JOIN content_type_standard_versions v ON v.id = c.version_id JOIN content_type_standards s ON s.id = v.standard_id WHERE s.content_type_code = t.code AND v.status = 'active') AS standard_chunk_count
		 FROM content_intake_types t `+clause+`
		 ORDER BY t.created_at, t.code LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2),
		listArgs...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(found))
	for _, raw := range found {
		row := ptRow(ptNormalizeMap(raw))
		item := mcIntakeTypeJSON(row)
		item["standard_version"] = row.numPtr("standard_version")
		item["standard_status"] = row.anyOrNil("standard_status")
		item["standard_chunk_count"] = int(row.numOr("standard_chunk_count"))
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// ContentIntakeTypeCreate: POST /v1/platform/content-intake-types
func (d *Deps) ContentIntakeTypeCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	if err := mcIntakeTypeValidation(body, true); err != nil {
		writeErr(w, d, err)
		return
	}
	code, _ := strField(body, "code")
	name, _ := strField(body, "name")
	description, hasDescription := strField(body, "description")
	questions, _ := rawField(body, "questions")
	if questions == nil {
		questions = []any{}
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO content_intake_types (id, code, name, description, questions, status, created_by) VALUES ($1, $2, $3, $4, $5, 'active', $6)",
		id, code, name, ptNullableArgStr(description, hasDescription && description != ""), ptJSONString(questions), user.ID); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errConflict("Kode jenis konten sudah digunakan"))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "content.intake_type_created", "content_intake_type", id)
	row, err := d.mcFindIntakeType(r, code)
	if err != nil || row == nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, mcIntakeTypeJSON(row))
}

// ContentIntakeTypeGet: GET /v1/platform/content-intake-types/{code}
func (d *Deps) ContentIntakeTypeGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.mcFindIntakeType(r, chiParam(r, "code"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Content intake type not found"))
		return
	}
	writeJSON(w, http.StatusOK, mcIntakeTypeJSON(row))
}

// ContentIntakeTypeUpdate: PUT /v1/platform/content-intake-types/{code}
func (d *Deps) ContentIntakeTypeUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	code := chiParam(r, "code")
	row, err := d.mcFindIntakeType(r, code)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Content intake type not found"))
		return
	}
	body := decodeJSON(r)
	if err := mcIntakeTypeValidation(body, false); err != nil {
		writeErr(w, d, err)
		return
	}
	name, hasName := strField(body, "name")
	description, hasDescription := strField(body, "description")
	questions, hasQuestions := rawField(body, "questions")
	status, hasStatus := strField(body, "status")

	descArg := any(nil)
	if hasDescription {
		descArg = description
	}
	questionsArg := any(nil)
	if hasQuestions && questions != nil {
		questionsArg = ptJSONString(questions)
	}
	if err := d.exec(r.Context(),
		"UPDATE content_intake_types SET name = COALESCE($1, name), description = COALESCE($2, description), questions = COALESCE($3, questions), status = COALESCE($4, status), updated_at = NOW() WHERE id = $5",
		ptNullableArgStr(name, hasName), descArg, questionsArg, ptNullableArgStr(status, hasStatus), row.strOr("id", "")); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errConflict("Kode jenis konten sudah digunakan"))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "content.intake_type_updated", "content_intake_type", row.strOr("id", ""))
	updated, err := d.mcFindIntakeType(r, code)
	if err != nil || updated == nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, mcIntakeTypeJSON(updated))
}

// ContentIntakeTypeDelete: DELETE /v1/platform/content-intake-types/{code}
func (d *Deps) ContentIntakeTypeDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	code := chiParam(r, "code")
	row, err := d.mcFindIntakeType(r, code)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Content intake type not found"))
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE content_intake_types SET status = 'archived', updated_at = NOW() WHERE id = $1", row.strOr("id", "")); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "content.intake_type_archived", "content_intake_type", row.strOr("id", ""))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- content type standards ----------

var mcSectionKeys = []string{
	"content_purpose", "news_angle", "journalistic_style", "required_information", "optional_information",
	"structure_guidance", "lead_guidance", "fact_attribution_rules", "quote_guidance", "language_tone",
	"what_to_avoid", "angle_variation_rules", "source_priority", "fact_conflict_handling", "editorial_quality_checklist",
}

// ContentIntakeTypeStandardsGet: GET /v1/platform/content-intake-types/{code}/standards
func (d *Deps) ContentIntakeTypeStandardsGet(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	cleanCode := strings.ToLower(strings.TrimSpace(chiParam(r, "code")))
	intakeType, err := d.queryRow(r.Context(),
		"SELECT id, code, name, description FROM content_intake_types WHERE code = $1 LIMIT 1", cleanCode)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if intakeType == nil {
		writeErr(w, d, errNotFound("Content intake type not found"))
		return
	}
	standard, err := d.mcStandardForCode(r, cleanCode, user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	activeVerRow, err := d.queryRow(r.Context(),
		"SELECT * FROM content_type_standard_versions WHERE standard_id = $1 AND status = 'active' LIMIT 1", standard.strOr("id", ""))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	allVersions, err := d.queryRows(r.Context(),
		"SELECT id, version, status, change_note, created_at FROM content_type_standard_versions WHERE standard_id = $1 ORDER BY version DESC", standard.strOr("id", ""))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	chunkCount := 0
	if activeVerRow != nil {
		chunkRow, cErr := d.queryRow(r.Context(),
			"SELECT COUNT(*) as count FROM content_type_standard_chunks WHERE version_id = $1", activeVerRow.strOr("id", ""))
		if cErr == nil && chunkRow != nil {
			chunkCount = int(chunkRow.numOr("count"))
		}
	}
	activeRow := ptRow(nil)
	if activeVerRow != nil {
		activeRow = ptRow(ptNormalizeMap(activeVerRow))
	}
	var activeStandard any
	if activeRow != nil {
		sections := mcEmptySections()
		for _, key := range mcSectionKeys {
			sections[key] = activeRow.strOr(key, "")
		}
		activeStandard = map[string]any{
			"id":                activeRow.strOr("id", ""),
			"standard_id":       activeRow.strOr("standard_id", ""),
			"content_type_code": activeRow.strOr("content_type_code", ""),
			"version":           int(activeRow.numOr("version")),
			"status":            activeRow.strOr("status", ""),
			"change_note":       ptNullableStr(activeRow, "change_note"),
			"sections":          sections,
			"created_by":        ptNullableStr(activeRow, "created_by"),
			"created_at":        activeRow.strOr("created_at", ""),
		}
	}
	versionsOut := make([]map[string]any, 0, len(allVersions))
	for _, raw := range allVersions {
		v := ptRow(ptNormalizeMap(raw))
		versionsOut = append(versionsOut, map[string]any{
			"id": v.strOr("id", ""), "version": int(v.numOr("version")),
			"status": v.strOr("status", ""), "change_note": ptNullableStr(v, "change_note"),
			"created_at": v.strOr("created_at", ""),
		})
	}
	var activeVersion any
	if activeStandard != nil {
		activeVersion = activeStandard.(map[string]any)["version"]
	}
	writeJSON(w, http.StatusOK, map[string]any{"standard": map[string]any{
		"id":                       standard.strOr("id", ""),
		"content_type_code":        cleanCode,
		"content_type_name":        intakeType.strOr("name", ""),
		"content_type_description": ptNullableStr(ptRow(ptNormalizeMap(intakeType)), "description"),
		"status":                   standard.strOr("status", ""),
		"active_version":           activeVersion,
		"active_standard":          activeStandard,
		"versions":                 versionsOut,
		"chunk_count":              chunkCount,
		"updated_at":               standard.strOr("updated_at", ""),
	}})
}

// mcStandardForCode: get-or-create content_type_standards (efek samping sama).
func (d *Deps) mcStandardForCode(r *http.Request, cleanCode, userID string) (ptRow, error) {
	row, err := d.queryRow(r.Context(),
		"SELECT id, content_type_code, status, created_at, updated_at FROM content_type_standards WHERE content_type_code = $1 LIMIT 1", cleanCode)
	if err != nil {
		return nil, err
	}
	if row != nil {
		return ptRow(ptNormalizeMap(row)), nil
	}
	standardID := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO content_type_standards (id, content_type_code, status, created_by) VALUES ($1, $2, 'active', $3)",
		standardID, cleanCode, userID); err != nil {
		return nil, err
	}
	created, err := d.queryRow(r.Context(), "SELECT * FROM content_type_standards WHERE id = $1 LIMIT 1", standardID)
	if err != nil || created == nil {
		return nil, err
	}
	return ptRow(ptNormalizeMap(created)), nil
}

// mcStandardValidation: parity saveStandardSchema (sections semua string).
func mcStandardValidation(body map[string]any) (map[string]string, string, bool) {
	sectionsRaw, hasSections := rawField(body, "sections")
	if !hasSections {
		return nil, "", false
	}
	sectionsMap, isObj := sectionsRaw.(map[string]any)
	if !isObj {
		return nil, "", false
	}
	sections := mcEmptySections()
	for _, key := range mcSectionKeys {
		if v, ok := sectionsMap[key]; ok && v != nil {
			s, isString := v.(string)
			if !isString {
				return nil, "", false
			}
			sections[key] = s
		}
	}
	status := "active"
	if v, ok := body["status"]; ok && v != nil {
		s, isString := v.(string)
		if !isString || (s != "draft" && s != "active") {
			return nil, "", false
		}
		status = s
	}
	return sections, status, true
}

// ContentIntakeTypeStandardsPut: PUT /v1/platform/content-intake-types/{code}/standards
func (d *Deps) ContentIntakeTypeStandardsPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	cleanCode := strings.ToLower(strings.TrimSpace(chiParam(r, "code")))
	intakeType, err := d.queryRow(r.Context(),
		"SELECT id, code, name, description FROM content_intake_types WHERE code = $1 LIMIT 1", cleanCode)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if intakeType == nil {
		writeErr(w, d, errNotFound("Content intake type not found"))
		return
	}
	body := decodeJSON(r)
	sections, status, valid := mcStandardValidation(body)
	if !valid {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Payload tidak valid"))
		return
	}
	changeNote := ""
	if s, has := strField(body, "change_note"); has {
		if len(s) > 255 {
			writeErr(w, d, ptErr(http.StatusInternalServerError, "Payload tidak valid"))
			return
		}
		changeNote = s
	}
	standard, err := d.mcStandardForCode(r, cleanCode, user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	maxRow, err := d.queryRow(r.Context(),
		"SELECT COALESCE(MAX(version), 0) AS max_v FROM content_type_standard_versions WHERE standard_id = $1", standard.strOr("id", ""))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	nextVersion := maxRow.intOr("max_v", 0) + 1
	newVersionID := uuid.NewString()
	if changeNote == "" {
		changeNote = "Pembaruan standar v" + ptItoa(nextVersion)
	}
	chunks := mcGenerateContentTypeChunks(cleanCode, intakeType.strOr("name", ""), sections)

	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if status == "active" {
			if err := tx.exec("UPDATE content_type_standard_versions SET status = 'archived' WHERE standard_id = $1 AND status = 'active'", standard.strOr("id", "")); err != nil {
				return err
			}
		}
		if err := tx.exec(
			`INSERT INTO content_type_standard_versions (
			   id, standard_id, content_type_code, version, status, change_note,
			   content_purpose, news_angle, journalistic_style, required_information, optional_information,
			   structure_guidance, lead_guidance, fact_attribution_rules, quote_guidance, language_tone,
			   what_to_avoid, angle_variation_rules, source_priority, fact_conflict_handling, editorial_quality_checklist,
			   created_by
			 ) VALUES (
			   $1, $2, $3, $4, $5, $6,
			   $7, $8, $9, $10, $11,
			   $12, $13, $14, $15, $16,
			   $17, $18, $19, $20, $21,
			   $22
			 )`,
			newVersionID, standard.strOr("id", ""), cleanCode, nextVersion, status, changeNote,
			sections["content_purpose"], sections["news_angle"], sections["journalistic_style"],
			sections["required_information"], sections["optional_information"], sections["structure_guidance"],
			sections["lead_guidance"], sections["fact_attribution_rules"], sections["quote_guidance"],
			sections["language_tone"], sections["what_to_avoid"], sections["angle_variation_rules"],
			sections["source_priority"], sections["fact_conflict_handling"], sections["editorial_quality_checklist"],
			user.ID); err != nil {
			return err
		}
		for _, c := range chunks {
			if err := tx.exec(
				`INSERT INTO content_type_standard_chunks (
				   id, version_id, content_type_code, section, chunk_index, content, embedding, priority
				 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				uuid.NewString(), newVersionID, cleanCode, c.Section, c.ChunkIndex, c.Content, ptJSONString(c.Embedding), c.Priority); err != nil {
				return err
			}
		}
		if err := tx.exec("UPDATE content_type_standards SET updated_at = NOW() WHERE id = $1", standard.strOr("id", "")); err != nil {
			return err
		}
		return tx.exec(
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, 'content.type_standard_updated', 'content_type_standard', $3)",
			uuid.NewString(), user.ID, standard.strOr("id", ""))
	}); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true, "version_id": newVersionID, "version": nextVersion, "status": status,
	})
}

// ContentIntakeTypeStandardVersionActivate: POST .../standards/versions/{versionId}/activate
func (d *Deps) ContentIntakeTypeStandardVersionActivate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	cleanCode := strings.ToLower(strings.TrimSpace(chiParam(r, "code")))
	versionID := chiParam(r, "versionId")
	verRow, err := d.queryRow(r.Context(),
		`SELECT v.id, v.standard_id, v.version, v.status, s.content_type_code
		 FROM content_type_standard_versions v
		 JOIN content_type_standards s ON s.id = v.standard_id
		 WHERE v.id = $1 AND s.content_type_code = $2 LIMIT 1`, versionID, cleanCode)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if verRow == nil {
		writeErr(w, d, errNotFound("Versi standar tidak ditemukan"))
		return
	}
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if err := tx.exec("UPDATE content_type_standard_versions SET status = 'archived' WHERE standard_id = $1 AND status = 'active'", verRow.strOr("standard_id", "")); err != nil {
			return err
		}
		if err := tx.exec("UPDATE content_type_standard_versions SET status = 'active' WHERE id = $1", versionID); err != nil {
			return err
		}
		if err := tx.exec("UPDATE content_type_standards SET updated_at = NOW() WHERE id = $1", verRow.strOr("standard_id", "")); err != nil {
			return err
		}
		return tx.exec(
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, 'content.type_standard_version_activated', 'content_type_standard_version', $3)",
			uuid.NewString(), user.ID, versionID)
	}); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true, "version_id": versionID,
		"version": int(verRow.numOr("version")), "status": "active",
	})
}

// ---------- chunker content-standards ----------

type mcChunk struct {
	Section    string
	ChunkIndex int
	Content    string
	Embedding  []float64
	Priority   int
}

// mcGenerateContentTypeChunks: port generateContentTypeChunks.
func mcGenerateContentTypeChunks(contentTypeCode, contentTypeName string, sections map[string]string) []mcChunk {
	all := []mcChunk{}
	for _, meta := range mcSectionMeta {
		rawValue := sections[meta.Key]
		if strings.TrimSpace(rawValue) == "" {
			continue
		}
		all = append(all, mcSplitSectionContent(contentTypeCode, contentTypeName, meta.Key, meta.Label, rawValue, meta.Priority)...)
	}
	return all
}

// mcSplitSectionContent: port splitSectionContent (chunk per ~600 char).
func mcSplitSectionContent(contentTypeCode, contentTypeName, sectionKey, sectionLabel, rawText string, priority int) []mcChunk {
	clean := strings.TrimSpace(rawText)
	if clean == "" {
		return nil
	}
	if len(clean) <= 650 {
		contextual := "[Standar Konten " + contentTypeName + " (" + contentTypeCode + ") — " + sectionLabel + "]:\n" + clean
		return []mcChunk{{
			Section: sectionKey, ChunkIndex: 0, Content: contextual,
			Embedding: ai.LocalEmbedding(contextual), Priority: priority,
		}}
	}
	paragraphs := []string{}
	for _, p := range reBlankLine.Split(clean, -1) {
		if t := strings.TrimSpace(p); t != "" {
			paragraphs = append(paragraphs, t)
		}
	}
	chunks := []mcChunk{}
	currentBlock := ""
	chunkIndex := 0
	for _, para := range paragraphs {
		if currentBlock != "" && len(currentBlock)+len(para) > 600 {
			contextual := "[Standar Konten " + contentTypeName + " (" + contentTypeCode + ") — " + sectionLabel + " (Bagian " + ptItoa(chunkIndex+1) + ")]:\n" + strings.TrimSpace(currentBlock)
			chunks = append(chunks, mcChunk{
				Section: sectionKey, ChunkIndex: chunkIndex, Content: contextual,
				Embedding: ai.LocalEmbedding(contextual), Priority: priority,
			})
			chunkIndex++
			currentBlock = para
		} else {
			if currentBlock != "" {
				currentBlock = currentBlock + "\n\n" + para
			} else {
				currentBlock = para
			}
		}
	}
	if strings.TrimSpace(currentBlock) != "" {
		bagian := ""
		if len(chunks) > 0 {
			bagian = " (Bagian " + ptItoa(chunkIndex+1) + ")"
		}
		contextual := "[Standar Konten " + contentTypeName + " (" + contentTypeCode + ") — " + sectionLabel + bagian + "]:\n" + strings.TrimSpace(currentBlock)
		chunks = append(chunks, mcChunk{
			Section: sectionKey, ChunkIndex: chunkIndex, Content: contextual,
			Embedding: ai.LocalEmbedding(contextual), Priority: priority,
		})
	}
	return chunks
}

// ---------- knowledge bases ----------

// KnowledgeBasesList: GET /v1/platform/knowledge-bases
func (d *Deps) KnowledgeBasesList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("q"))
	if len(search) > 120 {
		search = search[:120]
	}
	scope := strings.TrimSpace(q.Get("scope"))
	status := strings.TrimSpace(q.Get("status"))
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	params := []any{}
	if search != "" {
		like := "%" + search + "%"
		params = append(params, like, like, like)
		where = append(where, "(kb.name ILIKE $"+ptItoa(len(params)-2)+" OR kb.code ILIKE $"+ptItoa(len(params)-1)+" OR kb.description ILIKE $"+ptItoa(len(params))+")")
	}
	if scope == "global" || scope == "media" || scope == "brand" {
		params = append(params, scope)
		where = append(where, "kb.scope = $"+ptItoa(len(params)))
	}
	if status == "active" || status == "inactive" {
		params = append(params, status)
		where = append(where, "kb.status = $"+ptItoa(len(params)))
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM knowledge_bases kb "+clause, params...)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	listArgs := append([]any{}, params...)
	listArgs = append(listArgs, limit, offset)
	found, err := d.queryRows(r.Context(),
		`SELECT kb.id, kb.code, kb.name, kb.description, kb.scope, kb.scope_ref, kb.status,
		        (SELECT MAX(v.version) FROM knowledge_base_versions v WHERE v.knowledge_base_id = kb.id AND v.status = 'active') AS active_version,
		        (SELECT COUNT(*) FROM knowledge_base_rules r JOIN knowledge_base_versions v ON v.id = r.version_id WHERE r.knowledge_base_id = kb.id AND v.status = 'active' AND r.status = 'active') AS rule_count,
		        kb.created_at, kb.updated_at
		 FROM knowledge_bases kb `+clause+`
		 ORDER BY kb.created_at DESC, kb.code
		 LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2), listArgs...)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	items := make([]map[string]any, 0, len(found))
	for _, raw := range found {
		items = append(items, mcKBJSON(ptRow(ptNormalizeMap(raw))))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

func mcKBJSON(row ptRow) map[string]any {
	return map[string]any{
		"id":             row.strOr("id", ""),
		"code":           row.strOr("code", ""),
		"name":           row.strOr("name", ""),
		"description":    ptNullableStr(row, "description"),
		"scope":          row.strOr("scope", ""),
		"scope_ref":      ptNullableStr(row, "scope_ref"),
		"status":         row.strOr("status", ""),
		"active_version": row.numPtr("active_version"),
		"rule_count":     int(row.numOr("rule_count")),
		"created_at":     row.anyOrNil("created_at"),
		"updated_at":     row.anyOrNil("updated_at"),
	}
}

// mcCreateKBValidation: parity createKbSchema.
func mcCreateKBValidation(body map[string]any) error {
	code, hasCode := strField(body, "code")
	if !hasCode || code == "" || len(code) < 2 || len(code) > 60 || !reKbCode.MatchString(code) {
		return ptErr(http.StatusInternalServerError, "Payload tidak valid")
	}
	name, hasName := strField(body, "name")
	if !hasName || len(name) < 3 || len(name) > 200 {
		return ptErr(http.StatusInternalServerError, "Payload tidak valid")
	}
	if desc, hasDesc := body["description"]; hasDesc && desc != nil {
		s, isString := desc.(string)
		if !isString || len(s) > 2000 {
			return ptErr(http.StatusInternalServerError, "Payload tidak valid")
		}
	}
	if scope, hasScope := body["scope"]; hasScope && scope != nil {
		s, isString := scope.(string)
		if !isString || (s != "global" && s != "media" && s != "brand") {
			return ptErr(http.StatusInternalServerError, "Payload tidak valid")
		}
	}
	if ref, hasRef := body["scope_ref"]; hasRef && ref != nil {
		s, isString := ref.(string)
		if !isString || len(s) > 36 {
			return ptErr(http.StatusInternalServerError, "Payload tidak valid")
		}
	}
	return nil
}

// KnowledgeBaseCreate: POST /v1/platform/knowledge-bases
func (d *Deps) KnowledgeBaseCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	if err := mcCreateKBValidation(body); err != nil {
		writeErr(w, d, err)
		return
	}
	code, _ := strField(body, "code")
	name, _ := strField(body, "name")
	description, hasDescription := strField(body, "description")
	scope := "global"
	if s, hasScope := strField(body, "scope"); hasScope && s != "" {
		scope = s
	}
	scopeRef, hasScopeRef := strField(body, "scope_ref")
	kbID := uuid.NewString()
	versionID := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO knowledge_bases (id, code, name, description, scope, scope_ref, status, created_by) VALUES ($1, $2, $3, $4, $5, $6, 'active', $7)",
		kbID, code, name, ptNullableArgStr(description, hasDescription && description != ""), scope,
		ptNullableArgStr(scopeRef, hasScopeRef && scopeRef != ""), user.ID); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errConflict("Kode knowledge base sudah digunakan"))
			return
		}
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if err := d.exec(r.Context(),
		"INSERT INTO knowledge_base_versions (id, knowledge_base_id, version, status, change_note, created_by) VALUES ($1, $2, 1, 'active', 'Versi awal', $3)",
		versionID, kbID, user.ID); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	d.audit(r.Context(), user.ID, "knowledge_base.created", "knowledge_base", kbID)
	row, err := d.queryRow(r.Context(),
		`SELECT kb.id, kb.code, kb.name, kb.description, kb.scope, kb.scope_ref, kb.status,
		        (SELECT MAX(v.version) FROM knowledge_base_versions v WHERE v.knowledge_base_id = kb.id AND v.status = 'active') AS active_version,
		        0 AS rule_count, kb.created_at, kb.updated_at
		 FROM knowledge_bases kb WHERE kb.id = $1 LIMIT 1`, kbID)
	if err != nil || row == nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal membaca knowledge base"))
		return
	}
	writeJSON(w, http.StatusCreated, mcKBJSON(ptRow(ptNormalizeMap(row))))
}

// KnowledgeBaseGet: GET /v1/platform/knowledge-bases/{knowledgeBaseId}
func (d *Deps) KnowledgeBaseGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	kb, err := d.queryRow(r.Context(),
		"SELECT id, code, name, description, scope, scope_ref, status, created_by, created_at, updated_at FROM knowledge_bases WHERE id = $1 LIMIT 1", kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if kb == nil {
		writeErr(w, d, errNotFound("Knowledge base not found"))
		return
	}
	kbRow := ptRow(ptNormalizeMap(kb))
	versions, err := d.queryRows(r.Context(),
		`SELECT v.id, v.knowledge_base_id, v.version, v.status, v.change_note, v.created_by, v.created_at,
		        (SELECT COUNT(*) FROM knowledge_base_rules r WHERE r.version_id = v.id) AS rule_count
		 FROM knowledge_base_versions v WHERE v.knowledge_base_id = $1 ORDER BY v.version DESC`, kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	sources, err := d.queryRows(r.Context(),
		"SELECT id, source_name, source_url, source_type, notes, accessed_at FROM knowledge_base_sources WHERE knowledge_base_id = $1 ORDER BY source_name", kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	versionsOut := make([]map[string]any, 0, len(versions))
	for _, raw := range versions {
		v := ptRow(ptNormalizeMap(raw))
		versionsOut = append(versionsOut, map[string]any{
			"id": v.strOr("id", ""), "knowledge_base_id": v.strOr("knowledge_base_id", ""),
			"version": int(v.numOr("version")), "status": v.strOr("status", ""),
			"change_note": ptNullableStr(v, "change_note"), "created_by": ptNullableStr(v, "created_by"),
			"created_at": v.anyOrNil("created_at"), "rule_count": int(v.numOr("rule_count")),
		})
	}
	sourcesOut := make([]map[string]any, 0, len(sources))
	for _, raw := range sources {
		s := ptRow(ptNormalizeMap(raw))
		sourcesOut = append(sourcesOut, map[string]any{
			"id": s.strOr("id", ""), "source_name": s.strOr("source_name", ""),
			"source_url": ptNullableStr(s, "source_url"), "source_type": s.strOr("source_type", ""),
			"notes": ptNullableStr(s, "notes"), "accessed_at": s.anyOrNil("accessed_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": kbRow.strOr("id", ""), "code": kbRow.strOr("code", ""), "name": kbRow.strOr("name", ""),
		"description": ptNullableStr(kbRow, "description"), "scope": kbRow.strOr("scope", ""),
		"scope_ref": ptNullableStr(kbRow, "scope_ref"), "status": kbRow.strOr("status", ""),
		"created_at": kbRow.anyOrNil("created_at"), "updated_at": kbRow.anyOrNil("updated_at"),
		"versions": versionsOut, "sources": sourcesOut,
	})
}

// KnowledgeBaseUpdate: PATCH /v1/platform/knowledge-bases/{knowledgeBaseId}
func (d *Deps) KnowledgeBaseUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	description, hasDescription := strField(body, "description")
	status, hasStatus := strField(body, "status")
	if hasName && (len(name) < 3 || len(name) > 200) {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Payload tidak valid"))
		return
	}
	if hasDescription && len(description) > 2000 {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Payload tidak valid"))
		return
	}
	if hasStatus && status != "active" && status != "inactive" {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Payload tidak valid"))
		return
	}
	kb, err := d.queryRow(r.Context(), "SELECT id FROM knowledge_bases WHERE id = $1 LIMIT 1", kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if kb == nil {
		writeErr(w, d, errNotFound("Knowledge base not found"))
		return
	}
	sets := []string{}
	params := []any{}
	if hasName {
		params = append(params, name)
		sets = append(sets, "name = $"+ptItoa(len(params)))
	}
	if hasDescription {
		params = append(params, description)
		sets = append(sets, "description = $"+ptItoa(len(params)))
	}
	if hasStatus {
		params = append(params, status)
		sets = append(sets, "status = $"+ptItoa(len(params)))
	}
	if len(sets) > 0 {
		params = append(params, kbID)
		if err := d.exec(r.Context(),
			"UPDATE knowledge_bases SET "+strings.Join(sets, ", ")+", updated_at = NOW() WHERE id = $"+ptItoa(len(params)), params...); err != nil {
			writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
			return
		}
	}
	d.audit(r.Context(), user.ID, "knowledge_base.updated", "knowledge_base", kbID)
	updated, err := d.queryRow(r.Context(),
		"SELECT id, code, name, description, scope, scope_ref, status, created_by, created_at, updated_at FROM knowledge_bases WHERE id = $1 LIMIT 1", kbID)
	if err != nil || updated == nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal membaca knowledge base"))
		return
	}
	writeJSON(w, http.StatusOK, ptRow(ptNormalizeMap(updated)))
}

// KnowledgeBaseVersionsList: GET /v1/platform/knowledge-bases/{knowledgeBaseId}/versions
func (d *Deps) KnowledgeBaseVersionsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	kb, err := d.queryRow(r.Context(), "SELECT id FROM knowledge_bases WHERE id = $1 LIMIT 1", kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if kb == nil {
		writeErr(w, d, errNotFound("Knowledge base not found"))
		return
	}
	versions, err := d.queryRows(r.Context(),
		`SELECT v.id, v.version, v.status, v.change_note, v.created_by, v.created_at,
		        (SELECT COUNT(*) FROM knowledge_base_rules r WHERE r.version_id = v.id) AS rule_count
		 FROM knowledge_base_versions v WHERE v.knowledge_base_id = $1 ORDER BY v.version DESC`, kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	items := make([]map[string]any, 0, len(versions))
	for _, raw := range versions {
		v := ptRow(ptNormalizeMap(raw))
		items = append(items, map[string]any{
			"id": v.strOr("id", ""), "version": int(v.numOr("version")),
			"status": v.strOr("status", ""), "change_note": ptNullableStr(v, "change_note"),
			"created_by": ptNullableStr(v, "created_by"), "created_at": v.anyOrNil("created_at"),
			"rule_count": int(v.numOr("rule_count")),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// KnowledgeBaseVersionCreate: POST /v1/platform/knowledge-bases/{knowledgeBaseId}/versions
func (d *Deps) KnowledgeBaseVersionCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	body := decodeJSON(r)
	changeNote, hasChangeNote := strField(body, "change_note")
	sourceVersionID, hasSource := strField(body, "source_version_id")
	if hasChangeNote && len(changeNote) > 255 {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Payload tidak valid"))
		return
	}
	kb, err := d.queryRow(r.Context(), "SELECT id FROM knowledge_bases WHERE id = $1 LIMIT 1", kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if kb == nil {
		writeErr(w, d, errNotFound("Knowledge base not found"))
		return
	}
	sourceID := ""
	if hasSource && sourceVersionID != "" {
		sourceID = sourceVersionID
	} else {
		active, aErr := d.queryRow(r.Context(),
			"SELECT id FROM knowledge_base_versions WHERE knowledge_base_id = $1 AND status = 'active' ORDER BY version DESC LIMIT 1", kbID)
		if aErr == nil && active != nil {
			sourceID = active.strOr("id", "")
		}
	}
	if sourceID != "" {
		src, sErr := d.queryRow(r.Context(),
			"SELECT id FROM knowledge_base_versions WHERE id = $1 AND knowledge_base_id = $2 LIMIT 1", sourceID, kbID)
		if sErr != nil {
			writeErr(w, d, ptErr(http.StatusInternalServerError, sErr.Error()))
			return
		}
		if src == nil {
			writeErr(w, d, errNotFound("Source version not found"))
			return
		}
	}
	maxRow, err := d.queryRow(r.Context(),
		"SELECT COALESCE(MAX(version), 0) AS v FROM knowledge_base_versions WHERE knowledge_base_id = $1", kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	newVersion := maxRow.intOr("v", 0) + 1
	versionID := uuid.NewString()
	note := changeNote
	if note == "" {
		note = "Draft v" + ptItoa(newVersion)
	}
	if err := d.exec(r.Context(),
		"INSERT INTO knowledge_base_versions (id, knowledge_base_id, version, status, change_note, created_by) VALUES ($1, $2, $3, 'draft', $4, $5)",
		versionID, kbID, newVersion, note, user.ID); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if sourceID != "" {
		if err := d.exec(r.Context(),
			`INSERT INTO knowledge_base_rules (id, knowledge_base_id, version_id, category, title, instruction, examples, forbidden_examples, tags, audience, content_types, priority, status)
			 SELECT gen_random_uuid(), $1, $2, category, title, instruction, examples, forbidden_examples, tags, audience, content_types, priority, status
			 FROM knowledge_base_rules WHERE version_id = $3`,
			kbID, versionID, sourceID); err != nil {
			writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
			return
		}
	}
	d.audit(r.Context(), user.ID, "knowledge_base.version_created", "knowledge_base_version", versionID)
	created, err := d.queryRow(r.Context(),
		"SELECT id, version, status, change_note, created_by, created_at FROM knowledge_base_versions WHERE id = $1 LIMIT 1", versionID)
	if err != nil || created == nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal membaca versi"))
		return
	}
	writeJSON(w, http.StatusCreated, ptRow(ptNormalizeMap(created)))
}

// KnowledgeBaseVersionGet: GET .../versions/{versionId}
func (d *Deps) KnowledgeBaseVersionGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	versionID := chiParam(r, "versionId")
	version, err := d.queryRow(r.Context(),
		"SELECT id, knowledge_base_id, version, status, change_note, created_by, created_at FROM knowledge_base_versions WHERE id = $1 AND knowledge_base_id = $2 LIMIT 1",
		versionID, kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if version == nil {
		writeErr(w, d, errNotFound("Version not found"))
		return
	}
	rules, err := d.queryRows(r.Context(),
		"SELECT id, category, title, instruction, examples, forbidden_examples, tags, audience, content_types, priority, status, created_at, updated_at FROM knowledge_base_rules WHERE version_id = $1 ORDER BY priority, category, title",
		versionID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	rulesOut := make([]map[string]any, 0, len(rules))
	for _, raw := range rules {
		rule := ptRow(ptNormalizeMap(raw))
		rulesOut = append(rulesOut, map[string]any{
			"id": rule.strOr("id", ""), "category": rule.strOr("category", ""),
			"title": rule.strOr("title", ""), "instruction": rule.strOr("instruction", ""),
			"examples":           ptStrArray(rule.jsonField("examples")),
			"forbidden_examples": ptStrArray(rule.jsonField("forbidden_examples")),
			"tags":               ptStrArray(rule.jsonField("tags")),
			"audience":           ptStrArray(rule.jsonField("audience")),
			"content_types":      ptStrArray(rule.jsonField("content_types")),
			"priority":           rule.intOr("priority", 5),
			"status":             rule.strOr("status", ""),
			"created_at":         rule.anyOrNil("created_at"),
			"updated_at":         rule.anyOrNil("updated_at"),
		})
	}
	v := ptRow(ptNormalizeMap(version))
	writeJSON(w, http.StatusOK, map[string]any{
		"id": v.strOr("id", ""), "knowledge_base_id": v.strOr("knowledge_base_id", ""),
		"version": int(v.numOr("version")), "status": v.strOr("status", ""),
		"change_note": ptNullableStr(v, "change_note"), "created_by": ptNullableStr(v, "created_by"),
		"created_at": v.anyOrNil("created_at"), "rules": rulesOut,
	})
}

// KnowledgeBaseVersionUpdate: PATCH .../versions/{versionId}
func (d *Deps) KnowledgeBaseVersionUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	versionID := chiParam(r, "versionId")
	body := decodeJSON(r)
	version, err := d.queryRow(r.Context(),
		"SELECT id, status FROM knowledge_base_versions WHERE id = $1 AND knowledge_base_id = $2 LIMIT 1", versionID, kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if version == nil {
		writeErr(w, d, errNotFound("Version not found"))
		return
	}
	if version.strOr("status", "") != "draft" {
		writeErr(w, d, errConflict("Hanya versi draft yang dapat diedit"))
		return
	}
	if changeNote, hasNote := strField(body, "change_note"); hasNote {
		if len(changeNote) > 255 {
			changeNote = changeNote[:255]
		}
		if err := d.exec(r.Context(), "UPDATE knowledge_base_versions SET change_note = $1 WHERE id = $2", changeNote, versionID); err != nil {
			writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
			return
		}
	}
	d.audit(r.Context(), user.ID, "knowledge_base.version_updated", "knowledge_base_version", versionID)
	updated, err := d.queryRow(r.Context(),
		"SELECT id, version, status, change_note, created_by, created_at FROM knowledge_base_versions WHERE id = $1 LIMIT 1", versionID)
	if err != nil || updated == nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal membaca versi"))
		return
	}
	writeJSON(w, http.StatusOK, ptRow(ptNormalizeMap(updated)))
}

// KnowledgeBaseVersionDelete: DELETE .../versions/{versionId}
func (d *Deps) KnowledgeBaseVersionDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	versionID := chiParam(r, "versionId")
	version, err := d.queryRow(r.Context(),
		"SELECT id, status FROM knowledge_base_versions WHERE id = $1 AND knowledge_base_id = $2 LIMIT 1", versionID, kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if version == nil {
		writeErr(w, d, errNotFound("Version not found"))
		return
	}
	if version.strOr("status", "") != "draft" {
		writeErr(w, d, errConflict("Hanya versi draft yang dapat dihapus"))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM knowledge_base_versions WHERE id = $1", versionID); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	d.audit(r.Context(), user.ID, "knowledge_base.version_deleted", "knowledge_base_version", versionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// KnowledgeBaseVersionActivate: POST .../versions/{versionId}/activate
func (d *Deps) KnowledgeBaseVersionActivate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	versionID := chiParam(r, "versionId")
	version, err := d.queryRow(r.Context(),
		"SELECT id, status FROM knowledge_base_versions WHERE id = $1 AND knowledge_base_id = $2 LIMIT 1", versionID, kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if version == nil {
		writeErr(w, d, errNotFound("Version not found"))
		return
	}
	if version.strOr("status", "") != "draft" {
		writeErr(w, d, errConflict("Hanya versi draft yang dapat dipublikasikan"))
		return
	}
	kb, err := d.queryRow(r.Context(), "SELECT id, status FROM knowledge_bases WHERE id = $1 LIMIT 1", kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if kb == nil {
		writeErr(w, d, errNotFound("Knowledge base not found"))
		return
	}
	if kb.strOr("status", "") != "active" {
		writeErr(w, d, errConflict("Knowledge base nonaktif — aktifkan dulu sebelum publish"))
		return
	}
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if err := tx.exec("UPDATE knowledge_base_versions SET status = 'archived' WHERE knowledge_base_id = $1 AND status = 'active'", kbID); err != nil {
			return err
		}
		return tx.exec("UPDATE knowledge_base_versions SET status = 'active' WHERE id = $1", versionID)
	}); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	d.audit(r.Context(), user.ID, "knowledge_base.version_activated", "knowledge_base_version", versionID)
	v := ptRow(ptNormalizeMap(version))
	writeJSON(w, http.StatusOK, map[string]any{
		"id": v.strOr("id", ""), "status": "active",
	})
}

// KnowledgeBaseVersionRules: POST .../versions/{versionId}/rules (replace semantics)
func (d *Deps) KnowledgeBaseVersionRules(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	kbID := chiParam(r, "knowledgeBaseId")
	versionID := chiParam(r, "versionId")
	body := decodeJSON(r)
	version, err := d.queryRow(r.Context(),
		"SELECT id, status FROM knowledge_base_versions WHERE id = $1 AND knowledge_base_id = $2 LIMIT 1", versionID, kbID)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	if version == nil {
		writeErr(w, d, errNotFound("Version not found"))
		return
	}
	if version.strOr("status", "") != "draft" {
		writeErr(w, d, errConflict("Hanya versi draft yang dapat diedit rules"))
		return
	}
	rulesRaw, hasRules := rawField(body, "rules")
	rulesArr, isArr := rulesRaw.([]any)
	if !hasRules || !isArr || len(rulesArr) > 200 {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Payload tidak valid"))
		return
	}
	parsed := make([]mcRule, 0, len(rulesArr))
	for _, item := range rulesArr {
		obj, isObj := item.(map[string]any)
		if !isObj {
			writeErr(w, d, ptErr(http.StatusInternalServerError, "Payload tidak valid"))
			return
		}
		rule, ruleErr := mcParseRule(obj)
		if ruleErr != nil {
			writeErr(w, d, ruleErr)
			return
		}
		parsed = append(parsed, rule)
	}

	savedIDs := make([]string, 0, len(parsed))
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		oldRules, qErr := tx.queryRows("SELECT id FROM knowledge_base_rules WHERE version_id = $1", versionID)
		if qErr != nil {
			return qErr
		}
		for _, old := range oldRules {
			if err := tx.exec("DELETE FROM knowledge_base_rule_chunks WHERE rule_id = $1", old.strOr("id", "")); err != nil {
				return err
			}
		}
		if err := tx.exec("DELETE FROM knowledge_base_rules WHERE version_id = $1", versionID); err != nil {
			return err
		}
		for _, rule := range parsed {
			id := rule.ID
			if id == "" || !mcRuleExists(tx, id, versionID) {
				id = uuid.NewString()
			}
			if err := tx.exec(
				`INSERT INTO knowledge_base_rules (id, knowledge_base_id, version_id, category, title, instruction, examples, forbidden_examples, tags, audience, content_types, priority, status)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
				id, kbID, versionID, rule.Category, rule.Title, rule.Instruction,
				ptJSONString(rule.Examples), ptJSONString(rule.ForbiddenExamples), ptJSONString(rule.Tags),
				ptJSONString(rule.Audience), ptJSONString(rule.ContentTypes), rule.Priority, rule.Status); err != nil {
				return err
			}
			for _, chunk := range mcBuildRuleChunks(rule) {
				if err := tx.exec(
					"INSERT INTO knowledge_base_rule_chunks (id, rule_id, chunk_index, content, embedding) VALUES ($1, $2, $3, $4, $5)",
					uuid.NewString(), id, chunk.ChunkIndex, chunk.Content, ptJSONString(chunk.Embedding)); err != nil {
					return err
				}
			}
			savedIDs = append(savedIDs, id)
		}
		return nil
	}); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	d.audit(r.Context(), user.ID, "knowledge_base.rules_replaced", "knowledge_base_version", versionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rule_ids": savedIDs, "count": len(savedIDs)})
}

type mcRule struct {
	ID                string
	Category          string
	Title             string
	Instruction       string
	Examples          []string
	ForbiddenExamples []string
	Tags              []string
	Audience          []string
	ContentTypes      []string
	Priority          int
	Status            string
}

// mcParseRule: parity ruleSchema (pesan error generik 500 seperti route lama).
func mcParseRule(obj map[string]any) (mcRule, error) {
	rule := mcRule{Priority: 5, Status: "active"}
	bad := func() (mcRule, error) {
		return mcRule{}, ptErr(http.StatusInternalServerError, "Payload tidak valid")
	}
	if id, hasID := strField(obj, "id"); hasID {
		if strings.TrimSpace(id) == "" {
			return bad()
		}
		rule.ID = id
	}
	category, _ := strField(obj, "category")
	if category == "" || len(category) > 80 {
		return bad()
	}
	title, _ := strField(obj, "title")
	if title == "" || len(title) > 255 {
		return bad()
	}
	instruction, _ := strField(obj, "instruction")
	if len(instruction) < 10 || len(instruction) > 4000 {
		return bad()
	}
	rule.Category, rule.Title, rule.Instruction = category, title, instruction
	rule.Examples = mcStrArray(obj["examples"])
	rule.ForbiddenExamples = mcStrArray(obj["forbidden_examples"])
	rule.Tags = mcStrArray(obj["tags"])
	rule.Audience = mcStrArray(obj["audience"])
	rule.ContentTypes = mcStrArray(obj["content_types"])
	if len(rule.Examples) > 10 || len(rule.ForbiddenExamples) > 10 || len(rule.Tags) > 20 || len(rule.Audience) > 10 || len(rule.ContentTypes) > 20 {
		return bad()
	}
	if p, hasP := intField(obj, "priority"); hasP {
		if p < 1 || p > 8 {
			return bad()
		}
		rule.Priority = p
	}
	if st, hasSt := strField(obj, "status"); hasSt && st != "" {
		if st != "active" && st != "inactive" {
			return bad()
		}
		rule.Status = st
	}
	return rule, nil
}

func mcStrArray(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, isString := item.(string); isString && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func mcRuleExists(tx *ptTx, ruleID, versionID string) bool {
	rows, err := tx.queryRows("SELECT id FROM knowledge_base_rules WHERE id = $1 AND version_id = $2 LIMIT 1", ruleID, versionID)
	return err == nil && len(rows) > 0
}

// mcBuildRuleChunks: port buildRuleChunks (chunkText 600 char + embedding).
func mcBuildRuleChunks(rule mcRule) []mcChunk {
	parts := []string{rule.Instruction}
	for _, e := range rule.Examples {
		parts = append(parts, "Contoh baik: "+e)
	}
	for _, e := range rule.ForbiddenExamples {
		parts = append(parts, "Hindari: "+e)
	}
	filtered := []string{}
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	text := strings.Join(filtered, "\n\n")

	paragraphs := []string{}
	for _, p := range reBlankLine.Split(text, -1) {
		if t := strings.TrimSpace(p); t != "" {
			paragraphs = append(paragraphs, t)
		}
	}
	if len(paragraphs) == 0 {
		paragraphs = []string{strings.TrimSpace(text)}
	}
	chunks := []string{}
	const size = 600
	current := ""
	for _, paragraph := range paragraphs {
		if current != "" && len(current)+len(paragraph)+2 > size {
			chunks = append(chunks, current)
			current = ""
		}
		if len(paragraph) > size {
			for i := 0; i < len(paragraph); i += size {
				end := i + size
				if end > len(paragraph) {
					end = len(paragraph)
				}
				chunks = append(chunks, paragraph[i:end])
			}
			continue
		}
		if current != "" {
			current = current + "\n\n" + paragraph
		} else {
			current = paragraph
		}
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	if len(chunks) == 0 {
		fallback := strings.TrimSpace(text)
		if fallback == "" {
			fallback = " "
		}
		chunks = []string{fallback}
	}
	out := make([]mcChunk, 0, len(chunks))
	for i, content := range chunks {
		out = append(out, mcChunk{ChunkIndex: i, Content: content, Embedding: ai.LocalEmbedding(content)})
	}
	return out
}

// ---------- moderations ----------

// ModerationsList: GET /v1/platform/moderations
func (d *Deps) ModerationsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	limit := clampInt(q.Get("limit"), 20, 1, 100)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := "WHERE 1=1"
	params := []any{}
	if status == "safe" || status == "review_required" || status == "blocked" {
		params = append(params, status)
		where += " AND a.status = $1"
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS c FROM content_moderation_audits a "+where, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	listArgs := append([]any{}, params...)
	listArgs = append(listArgs, limit, offset)
	items, err := d.queryRows(r.Context(),
		`SELECT a.*, b.name AS brand_name, d.title AS document_title
		 FROM content_moderation_audits a
		 LEFT JOIN brands b ON b.id = a.brand_id
		 LEFT JOIN content_documents d ON d.id = a.entity_id
		 `+where+`
		 ORDER BY a.created_at DESC LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2), listArgs...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	stats, err := d.mcModerationStats(r)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"audits": ptRows(items), "total": countRow.intOr("c", 0), "limit": limit, "offset": offset, "stats": stats,
	})
}

// mcModerationStats: port getModerationStats.
func (d *Deps) mcModerationStats(r *http.Request) (map[string]any, error) {
	fallback := map[string]any{
		"total": 0, "safe": 0, "review_required": 0, "blocked": 0, "ai_called": 0,
		"rule_only": 0, "cache_hit_estimate": 0, "token_input": 0, "token_output": 0, "token_total": 0,
	}
	tot, err1 := d.queryRow(r.Context(), "SELECT COUNT(*) AS c FROM content_moderation_results")
	safe, err2 := d.queryRow(r.Context(), "SELECT COUNT(*) AS c FROM content_moderation_results WHERE status='safe'")
	review, err3 := d.queryRow(r.Context(), "SELECT COUNT(*) AS c FROM content_moderation_results WHERE status='review_required'")
	blocked, err4 := d.queryRow(r.Context(), "SELECT COUNT(*) AS c FROM content_moderation_results WHERE status='blocked'")
	ai, err5 := d.queryRow(r.Context(), "SELECT COUNT(*) AS c FROM content_moderation_results WHERE ai_called=true")
	tokens, err6 := d.queryRow(r.Context(), "SELECT COALESCE(SUM(token_input),0) AS ti, COALESCE(SUM(token_output),0) AS to_, COALESCE(SUM(token_total),0) AS tt FROM content_moderation_results")
	audits, err7 := d.queryRow(r.Context(), "SELECT COUNT(*) AS c FROM content_moderation_audits")
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil || err7 != nil {
		return fallback, nil
	}
	total := tot.intOr("c", 0)
	aiCalled := ai.intOr("c", 0)
	cacheEstimate := audits.intOr("c", 0) - total
	if cacheEstimate < 0 {
		cacheEstimate = 0
	}
	return map[string]any{
		"total":              total,
		"safe":               safe.intOr("c", 0),
		"review_required":    review.intOr("c", 0),
		"blocked":            blocked.intOr("c", 0),
		"ai_called":          aiCalled,
		"rule_only":          total - aiCalled,
		"cache_hit_estimate": cacheEstimate,
		"token_input":        int(tokens.numOr("ti")),
		"token_output":       int(tokens.numOr("to_")),
		"token_total":        int(tokens.numOr("tt")),
	}, nil
}

// ModerationReview: POST /v1/platform/moderations/{auditId}/review
func (d *Deps) ModerationReview(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	auditID := chiParam(r, "auditId")
	body := decodeJSON(r)
	decision, _ := strField(body, "decision")
	if decision != "approved" && decision != "rejected" {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	note := ""
	if s, hasNote := strField(body, "note"); hasNote {
		if len(s) > 2000 {
			writeErr(w, d, errUnprocessable("Payload tidak valid."))
			return
		}
		note = s
	}
	row, err := d.queryRow(r.Context(), "SELECT id FROM content_moderation_audits WHERE id = $1 LIMIT 1", auditID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Audit not found"))
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE content_moderation_audits SET reviewed_by = $1, reviewed_at = NOW(), review_decision = $2, review_note = $3 WHERE id = $4",
		user.ID, decision, ptNullableArgStr(note, note != ""), auditID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "moderation."+decision, "moderation_audit", auditID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- util ----------

// mcIsDuplicate: deteksi pelanggaran unique (parity /Duplicate|uq_/).
func mcIsDuplicate(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "uq_") || strings.Contains(msg, "23505")
}

// queryRows untuk ptTx (dipakai aturan knowledge-base).
func (t *ptTx) queryRows(sql string, args ...any) ([]ptRow, error) {
	rows, err := t.tx.Query(t.ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := rows.FieldDescriptions()
	out := []ptRow{}
	for rows.Next() {
		values, vErr := rows.Values()
		if vErr != nil {
			return nil, vErr
		}
		item := make(ptRow, len(names))
		for i, f := range names {
			item[string(f.Name)] = ptValue(values[i])
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
