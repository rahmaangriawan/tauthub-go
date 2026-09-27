package platform

// media_standards.go: standar konten media admin — GET/POST
// /v1/platform/media-standards dan GET/PUT/DELETE
// /v1/platform/media-standards/{mediaId}.
// Port app/v1/platform/media-standards/route.ts + [mediaId]/route.ts + _standard.ts.

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const mpStandardSelect = `SELECT s.id, s.media_id, m.name AS media_name, s.min_words, s.max_words, s.journalistic_style, s.hard_selling_tolerance, s.quote_required, s.min_quotes, s.image_required, s.category_required, s.required_info, s.forbidden_content, s.status, s.created_at, s.updated_at
FROM media_content_standards s LEFT JOIN media m ON m.id = s.media_id`

var mpJournalisticStyles = []string{"news", "feature", "magazine", "blog"}
var mpHardSellingTolerances = []string{"none", "low", "medium", "high"}

// mpStandardJSON: parity standardJson (required_info/forbidden_content = parseStrings).
func mpStandardJSON(row ptRow) map[string]any {
	return map[string]any{
		"id":                     row.anyOrNil("id"),
		"media_id":               row.anyOrNil("media_id"),
		"media_name":             row.anyOrNil("media_name"),
		"min_words":              row.intOr("min_words", 0),
		"max_words":              row.intOr("max_words", 0),
		"journalistic_style":     row.anyOrNil("journalistic_style"),
		"hard_selling_tolerance": row.anyOrNil("hard_selling_tolerance"),
		"quote_required":         row.boolOr("quote_required", false),
		"min_quotes":             row.intOr("min_quotes", 0),
		"image_required":         row.boolOr("image_required", false),
		"category_required":      row.boolOr("category_required", false),
		"required_info":          mpParseStrings(row.jsonField("required_info")),
		"forbidden_content":      mpParseStrings(row.jsonField("forbidden_content")),
		"status":                 row.anyOrNil("status"),
		"created_at":             row.anyOrNil("created_at"),
		"updated_at":             row.anyOrNil("updated_at"),
	}
}

// mpStandardError: port standardError (FK media → 404, duplikat → 409, else 500).
func mpStandardError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "fk_media") || strings.Contains(msg, "23503") {
		return errNotFound("Media not found")
	}
	if strings.Contains(msg, "duplicate") || strings.Contains(msg, "uq_") || strings.Contains(msg, "23505") {
		return errConflict("Standard untuk media ini sudah ada")
	}
	return err
}

// mpStrArrayField: array string dari body (mis. required_info).
func mpStrArrayField(body map[string]any, key string) ([]string, bool) {
	v, has := body[key]
	if !has {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, isStr := item.(string)
		if !isStr {
			return nil, false
		}
		out = append(out, strings.TrimSpace(s))
	}
	return out, true
}

// mpStandardValidateCreate: parity mediaStandardSchema (nilai + pesan Indonesia).
func mpStandardValidateCreate(body map[string]any) (map[string]any, error) {
	mediaID, hasMedia := strField(body, "media_id")
	if !hasMedia || mediaID == "" {
		return nil, errUnprocessable("Invalid input: expected string, received undefined")
	}
	out := map[string]any{"media_id": mediaID}

	readInt := func(key string, def, min, max int) (int, error) {
		v, has := intField(body, key)
		if !has {
			return def, nil
		}
		if v < min || v > max {
			return 0, errUnprocessable("Invalid " + key)
		}
		return v, nil
	}

	minWords, err := readInt("min_words", 400, 0, 100_000)
	if err != nil {
		return nil, err
	}
	maxWords, err := readInt("max_words", 1500, 0, 100_000)
	if err != nil {
		return nil, err
	}
	if maxWords < minWords {
		return nil, errUnprocessable("max_words must be >= min_words")
	}
	minQuotes, err := readInt("min_quotes", 0, 0, 50)
	if err != nil {
		return nil, err
	}
	out["min_words"], out["max_words"], out["min_quotes"] = minWords, maxWords, minQuotes

	style := "news"
	if v, has := strField(body, "journalistic_style"); has {
		if !mpContains(mpJournalisticStyles, v) {
			return nil, errUnprocessable(mpZEnum(mpJournalisticStyles))
		}
		style = v
	}
	out["journalistic_style"] = style

	tolerance := "low"
	if v, has := strField(body, "hard_selling_tolerance"); has {
		if !mpContains(mpHardSellingTolerances, v) {
			return nil, errUnprocessable(mpZEnum(mpHardSellingTolerances))
		}
		tolerance = v
	}
	out["hard_selling_tolerance"] = tolerance

	for _, key := range []string{"quote_required", "image_required", "category_required"} {
		v, _ := boolField(body, key)
		out[key] = v
	}
	for _, key := range []string{"required_info", "forbidden_content"} {
		arr, has := mpStrArrayField(body, key)
		if !has {
			out[key] = []string{}
			continue
		}
		if len(arr) > 30 {
			return nil, errUnprocessable("Too big: expected array to have <=30 items")
		}
		for _, s := range arr {
			if s == "" || len(s) > 120 {
				return nil, errUnprocessable("Invalid " + key)
			}
		}
		out[key] = arr
	}
	return out, nil
}

// mpStandardValidateUpdate: parity mediaStandardUpdateSchema (semua opsional).
func mpStandardValidateUpdate(body map[string]any) (map[string]any, error) {
	out := map[string]any{}
	if v, has := intField(body, "min_words"); has {
		if v < 0 || v > 100_000 {
			return nil, errUnprocessable("Invalid min_words")
		}
		out["min_words"] = v
	}
	if v, has := intField(body, "max_words"); has {
		if v < 0 || v > 100_000 {
			return nil, errUnprocessable("Invalid max_words")
		}
		out["max_words"] = v
	}
	if minV, ok := out["min_words"].(int); ok {
		if maxV, has := out["max_words"].(int); has && maxV < minV {
			return nil, errUnprocessable("max_words must be >= min_words")
		}
	}
	if v, has := strField(body, "journalistic_style"); has {
		if !mpContains(mpJournalisticStyles, v) {
			return nil, errUnprocessable(mpZEnum(mpJournalisticStyles))
		}
		out["journalistic_style"] = v
	}
	if v, has := strField(body, "hard_selling_tolerance"); has {
		if !mpContains(mpHardSellingTolerances, v) {
			return nil, errUnprocessable(mpZEnum(mpHardSellingTolerances))
		}
		out["hard_selling_tolerance"] = v
	}
	if v, has := boolField(body, "quote_required"); has {
		out["quote_required"] = v
	}
	if v, has := intField(body, "min_quotes"); has {
		if v < 0 || v > 50 {
			return nil, errUnprocessable("Invalid min_quotes")
		}
		out["min_quotes"] = v
	}
	if v, has := boolField(body, "image_required"); has {
		out["image_required"] = v
	}
	if v, has := boolField(body, "category_required"); has {
		out["category_required"] = v
	}
	for _, key := range []string{"required_info", "forbidden_content"} {
		arr, has := mpStrArrayField(body, key)
		if !has {
			continue
		}
		if len(arr) > 30 {
			return nil, errUnprocessable("Too big: expected array to have <=30 items")
		}
		out[key] = arr
	}
	if v, has := strField(body, "status"); has {
		if v != "active" && v != "archived" {
			return nil, errUnprocessable(mpZEnum([]string{"active", "archived"}))
		}
		out["status"] = v
	}
	return out, nil
}

// ---------- GET/POST /v1/platform/media-standards ----------

func (d *Deps) MediaStandardsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	clause := ""
	args := []any{}
	if status == "archived" || status == "active" {
		clause = "WHERE s.status = $1"
		args = append(args, status)
	}
	found, err := d.queryRows(r.Context(), mpStandardSelect+" "+clause+" ORDER BY s.created_at DESC", args...)
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	out := make([]map[string]any, 0, len(found))
	for _, raw := range found {
		out = append(out, mpStandardJSON(ptRow(ptNormalizeMap(raw))))
	}
	writeJSON(w, http.StatusOK, out)
}

func (d *Deps) MediaStandardCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	value, err := mpStandardValidateCreate(body)
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	mediaID, _ := value["media_id"].(string)
	media, err := d.queryRow(r.Context(), "SELECT id FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	if media == nil {
		writeErr(w, d, errNotFound("Media not found"))
		return
	}
	existing, err := d.queryRow(r.Context(), "SELECT id FROM media_content_standards WHERE media_id = $1 LIMIT 1", mediaID)
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	if existing != nil {
		writeErr(w, d, errConflict("Standard untuk media ini sudah ada — gunakan PUT"))
		return
	}

	id := uuid.NewString()
	if err := d.exec(r.Context(), `
		INSERT INTO media_content_standards (id, media_id, min_words, max_words, journalistic_style, hard_selling_tolerance, quote_required, min_quotes, image_required, category_required, required_info, forbidden_content, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'active', $13)`,
		id, mediaID, value["min_words"], value["max_words"], value["journalistic_style"], value["hard_selling_tolerance"],
		value["quote_required"], value["min_quotes"], value["image_required"], value["category_required"],
		mpJSONString(value["required_info"]), mpJSONString(value["forbidden_content"]), user.ID); err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "media.standard_created", "media_content_standard", id)

	row, err := d.queryRow(r.Context(), mpStandardSelect+" WHERE s.id = $1 LIMIT 1", id)
	if err != nil || row == nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	writeJSON(w, http.StatusCreated, mpStandardJSON(ptRow(ptNormalizeMap(row))))
}

// ---------- GET/PUT/DELETE /v1/platform/media-standards/{mediaId} ----------

func (d *Deps) mpFindStandard(ctx context.Context, mediaID string) (ptRow, error) {
	row, err := d.queryRow(ctx, mpStandardSelect+" WHERE s.media_id = $1 LIMIT 1", mediaID)
	if err != nil || row == nil {
		return nil, err
	}
	return ptRow(ptNormalizeMap(row)), nil
}

func (d *Deps) MediaStandardGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.mpFindStandard(r.Context(), chiParam(r, "mediaId"))
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Media standard not found"))
		return
	}
	writeJSON(w, http.StatusOK, mpStandardJSON(row))
}

func (d *Deps) MediaStandardUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	existing, err := d.mpFindStandard(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound("Media standard not found"))
		return
	}
	body := decodeJSON(r)
	value, err := mpStandardValidateUpdate(body)
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}

	if err := d.exec(r.Context(), `
		UPDATE media_content_standards SET
		  min_words = COALESCE($1, min_words),
		  max_words = COALESCE($2, max_words),
		  journalistic_style = COALESCE($3, journalistic_style),
		  hard_selling_tolerance = COALESCE($4, hard_selling_tolerance),
		  quote_required = COALESCE($5, quote_required),
		  min_quotes = COALESCE($6, min_quotes),
		  image_required = COALESCE($7, image_required),
		  category_required = COALESCE($8, category_required),
		  required_info = COALESCE($9, required_info),
		  forbidden_content = COALESCE($10, forbidden_content),
		  status = COALESCE($11, status),
		  updated_at = NOW()
		WHERE media_id = $12`,
		ptNullableAnyInt(mpIntFromAny(value, "min_words"), mpHasKey(value, "min_words")),
		ptNullableAnyInt(mpIntFromAny(value, "max_words"), mpHasKey(value, "max_words")),
		mpNullableStrFromMap(value, "journalistic_style"),
		mpNullableStrFromMap(value, "hard_selling_tolerance"),
		mpNullableBoolFromMap(value, "quote_required"),
		ptNullableAnyInt(mpIntFromAny(value, "min_quotes"), mpHasKey(value, "min_quotes")),
		mpNullableBoolFromMap(value, "image_required"),
		mpNullableBoolFromMap(value, "category_required"),
		mpNullableJSONFromMap(value, "required_info"),
		mpNullableJSONFromMap(value, "forbidden_content"),
		mpNullableStrFromMap(value, "status"),
		mediaID); err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	auditID := existing.strOr("id", "")
	writeAuditBestEffort(d, r.Context(), user.ID, "media.standard_updated", "media_content_standard", auditID)

	updated, err := d.mpFindStandard(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	if updated == nil {
		writeErr(w, d, errNotFound("Media standard not found"))
		return
	}
	writeJSON(w, http.StatusOK, mpStandardJSON(updated))
}

func (d *Deps) MediaStandardDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	existing, err := d.mpFindStandard(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound("Media standard not found"))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM media_content_standards WHERE media_id = $1", mediaID); err != nil {
		writeErr(w, d, mpStandardError(err))
		return
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "media.standard_deleted", "media_content_standard", existing.strOr("id", ""))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// mpHasKey: true bila kunci ada di body (nilai boleh apa pun).
func mpHasKey(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

func mpIntFromAny(m map[string]any, key string) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

func mpNullableStrFromMap(m map[string]any, key string) any {
	if s, ok := m[key].(string); ok {
		return s
	}
	return nil
}

func mpNullableBoolFromMap(m map[string]any, key string) any {
	if b, ok := m[key].(bool); ok {
		return b
	}
	return nil
}

func mpNullableJSONFromMap(m map[string]any, key string) any {
	if v, ok := m[key]; ok && v != nil {
		return mpJSONString(v)
	}
	return nil
}
