package platform

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Modul operasional platform lain yang belum punya berkas sendiri:
// bug-reports, auto-replace, config/template-url, gatekeeper logs/stats.
// Guard mengikuti route lama: isPlatformAdmin → adminGuard("") dan
// isSuperAdmin → adminGuard("") + ctx.IsSuperAdmin().
//
// Helper di berkas ini diberi awalan `mops` (misc ops) supaya tidak bentrok
// dengan helper modul platform lain yang berbagi package.

// ---------- helper body & tipe ----------

// mopsParseJSONBody: baca body JSON apa adanya; ok=false untuk body kosong /
// JSON tidak valid (parity `request.json().catch(() => null)`).
func mopsParseJSONBody(r *http.Request) (any, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, false
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, false
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

// admRawBody: parity `await request.json().catch(() => null)` + zod object —
// skalar tetap diteruskan agar issue tipenya sesuai ("received number" dsb).
func admRawBody(r *http.Request) (map[string]any, string) {
	parsed, ok := mopsParseJSONBody(r)
	if !ok {
		return nil, "Invalid input: expected object, received null"
	}
	if obj, isObj := parsed.(map[string]any); isObj {
		return obj, ""
	}
	return nil, "Invalid input: expected object, received " + mopsJSONType(parsed)
}

// mopsRawBodyObject: parity jsonBody app lama (`data && typeof data === "object"
// ? data : null`) lalu skema object — skalar menjadi null, array diteruskan.
func mopsRawBodyObject(r *http.Request) (map[string]any, string) {
	parsed, ok := mopsParseJSONBody(r)
	if !ok {
		return nil, "Invalid input: expected object, received null"
	}
	switch typed := parsed.(type) {
	case map[string]any:
		return typed, ""
	case []any:
		return nil, "Invalid input: expected object, received array"
	default:
		return nil, "Invalid input: expected object, received null"
	}
}

// mopsJSONType: parity parsedType zod (typeof JS + null/array).
func mopsJSONType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, int, int64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "undefined"
}

// mopsRecordType: zod `record` menerima array (issue "received array"), sedangkan
// skalar/null dilaporkan sesuai tipe aslinya.
func mopsRecordType(v any) string {
	if _, isArr := v.([]any); isArr {
		return "array"
	}
	return mopsJSONType(v)
}

// mopsZodIssue: satu issue zod. Dipakai endpoint yang TIDAK memakai
// errorResponse sehingga ZodError mentah bocor sebagai detail 500
// (auto-replace: `detail: error.message`).
type mopsZodIssue struct {
	Expected  string   `json:"expected,omitempty"`
	Format    string   `json:"format,omitempty"`
	Origin    string   `json:"origin,omitempty"`
	Code      string   `json:"code"`
	Minimum   *float64 `json:"minimum,omitempty"`
	Maximum   *float64 `json:"maximum,omitempty"`
	Inclusive *bool    `json:"inclusive,omitempty"`
	Path      []string `json:"path"`
	Message   string   `json:"message"`
}

// mopsZodMessage: parity ZodError.message = JSON.stringify(issues, null, 2).
func mopsZodMessage(issues []mopsZodIssue) string {
	b, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(b)
}

func mopsFloatPtr(v float64) *float64 { return &v }
func mopsBoolPtr(v bool) *bool        { return &v }

func mopsJSONNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// mopsAtoiOr: parity `Number(x) || fallback` untuk query string.
func mopsAtoiOr(raw string, fallback int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return fallback
	}
	return n
}

func mopsClampInt(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// mopsTruncate: parity `str.trim().slice(0, n)`.
func mopsTruncate(raw string, n int) string {
	trimmed := strings.TrimSpace(raw)
	runes := []rune(trimmed)
	if len(runes) > n {
		return string(runes[:n])
	}
	return trimmed
}

// ---------- bug-reports ----------

// BugReportsList: GET /v1/platform/bug-reports
func (d *Deps) BugReportsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	search := mopsTruncate(q.Get("q"), 120)

	offset := mopsAtoiOr(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	limit := mopsClampInt(mopsAtoiOr(q.Get("limit"), 50), 1, 100)

	where := []string{}
	params := []any{}
	if status != "" {
		params = append(params, status)
		where = append(where, "r.status = $"+ptItoa(len(params)))
	}
	if search != "" {
		like := "%" + search + "%"
		params = append(params, like, like, like)
		where = append(where, "(r.message ILIKE $"+ptItoa(len(params)-2)+
			" OR u.email ILIKE $"+ptItoa(len(params)-1)+
			" OR b.name ILIKE $"+ptItoa(len(params))+")")
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}

	countRow, err := d.queryRow(r.Context(),
		`SELECT COUNT(*) AS total FROM bug_reports r
		 LEFT JOIN users u ON u.id = r.user_id
		 LEFT JOIN brands b ON b.id = r.brand_id
		 `+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		`SELECT r.id, r.user_id, r.brand_id, r.message, r.context_info, r.status, r.admin_notes, r.created_at, r.updated_at,
		        u.email AS user_email, u.display_name AS user_name, b.name AS brand_name
		 FROM bug_reports r
		 LEFT JOIN users u ON u.id = r.user_id
		 LEFT JOIN brands b ON b.id = r.brand_id
		 `+clause+`
		 ORDER BY r.created_at DESC LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total": countRow.intOr("total", 0),
		"items": ptRows(items),
	})
}

// BugReportGet: GET /v1/platform/bug-reports/{reportId}
func (d *Deps) BugReportGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	item, err := d.queryRow(r.Context(),
		`SELECT r.id, r.user_id, r.brand_id, r.message, r.context_info, r.status, r.admin_notes, r.created_at, r.updated_at,
		        u.email AS user_email, u.display_name AS user_name, b.name AS brand_name
		 FROM bug_reports r
		 LEFT JOIN users u ON u.id = r.user_id
		 LEFT JOIN brands b ON b.id = r.brand_id
		 WHERE r.id = $1 LIMIT 1`, chiParam(r, "reportId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if item == nil {
		writeErr(w, d, errNotFound("Report not found"))
		return
	}
	writeJSON(w, http.StatusOK, ptNormalizeMap(item))
}

// BugReportUpdate: PATCH /v1/platform/bug-reports/{reportId}
func (d *Deps) BugReportUpdate(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	reportID := chiParam(r, "reportId")

	raw, zodMsg := mopsRawBodyObject(r)
	if zodMsg != "" {
		writeErr(w, d, errUnprocessable(zodMsg))
		return
	}

	status := ""
	hasStatus := false
	if v, present := raw["status"]; present {
		s, isStr := v.(string)
		if !isStr || !ptContains([]string{"open", "triage", "resolved", "closed"}, s) {
			writeErr(w, d, errUnprocessable(`Invalid option: expected one of "open"|"triage"|"resolved"|"closed"`))
			return
		}
		status, hasStatus = s, true
	}

	notes := ""
	hasNotes := false
	if v, present := raw["admin_notes"]; present {
		hasNotes = true
		if v != nil {
			s, isStr := v.(string)
			if !isStr {
				writeErr(w, d, errUnprocessable("Invalid input: expected string, received "+mopsJSONType(v)))
				return
			}
			if len([]rune(s)) > 5000 {
				writeErr(w, d, errUnprocessable("Too big: expected string to have <=5000 characters"))
				return
			}
			notes = s
		}
	}

	if !hasStatus && !hasNotes {
		writeErr(w, d, errUnprocessable("Nothing to update"))
		return
	}

	sets := []string{}
	vals := []any{}
	if hasStatus {
		vals = append(vals, status)
		sets = append(sets, "status = $"+ptItoa(len(vals)))
	}
	if hasNotes {
		// Parity app lama: admin_notes null dikirim sebagai "".
		vals = append(vals, notes)
		sets = append(sets, "admin_notes = $"+ptItoa(len(vals)))
	}
	vals = append(vals, reportID)
	if err := d.exec(r.Context(),
		"UPDATE bug_reports SET "+strings.Join(sets, ", ")+" WHERE id = $"+ptItoa(len(vals)), vals...); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- auto-replace ----------

// AutoReplaceGet: GET /v1/platform/auto-replace
func (d *Deps) AutoReplaceGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	globalRaw, err := d.queryRow(r.Context(),
		"SELECT id, enabled, max_replaces::int AS max_replaces, updated_at FROM auto_replace_configs WHERE brand_id IS NULL LIMIT 1")
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	perBrandRaw, err := d.queryRows(r.Context(),
		`SELECT c.id, c.brand_id, b.name AS brand_name, c.enabled, c.max_replaces::int AS max_replaces, c.updated_at
		 FROM auto_replace_configs c
		 LEFT JOIN brands b ON b.id = c.brand_id
		 WHERE c.brand_id IS NOT NULL
		 ORDER BY b.name ASC`)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}

	var global any
	if globalRaw != nil {
		row := ptRow(ptNormalizeMap(globalRaw))
		global = map[string]any{
			"id":           row.strOr("id", ""),
			"enabled":      row.boolOr("enabled", false),
			"max_replaces": row.intOr("max_replaces", 0),
			"updated_at":   row.anyOrNil("updated_at"),
		}
	}
	out := make([]map[string]any, 0, len(perBrandRaw))
	for _, raw := range perBrandRaw {
		row := ptRow(ptNormalizeMap(raw))
		name := row.strOr("brand_name", "")
		if name == "" {
			name = "Brand"
		}
		out = append(out, map[string]any{
			"id":           row.strOr("id", ""),
			"brand_id":     row.strOr("brand_id", ""),
			"brand_name":   name,
			"enabled":      row.boolOr("enabled", false),
			"max_replaces": row.intOr("max_replaces", 0),
			"updated_at":   row.anyOrNil("updated_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"global": global, "per_brand": out})
}

// AutoReplacePut: PUT /v1/platform/auto-replace?brandId=
func (d *Deps) AutoReplacePut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	brandID := strings.TrimSpace(r.URL.Query().Get("brandId"))

	body, zodMsg := mopsRawBodyObject(r)
	if zodMsg != "" {
		writeErr(w, d, ptErr(http.StatusInternalServerError, mopsZodMessage([]mopsZodIssue{{
			Expected: "object", Code: "invalid_type", Path: []string{}, Message: zodMsg,
		}})))
		return
	}

	// parity configSchema: ZodError.message memuat SEMUA issue sekaligus.
	issues := []mopsZodIssue{}
	enabled := false
	if v, present := body["enabled"]; !present {
		issues = append(issues, mopsZodIssue{
			Expected: "boolean", Code: "invalid_type", Path: []string{"enabled"},
			Message: "Invalid input: expected boolean, received undefined",
		})
	} else if b, isBool := v.(bool); !isBool {
		issues = append(issues, mopsZodIssue{
			Expected: "boolean", Code: "invalid_type", Path: []string{"enabled"},
			Message: "Invalid input: expected boolean, received " + mopsJSONType(v),
		})
	} else {
		enabled = b
	}

	maxReplaces := 0
	if v, present := body["max_replaces"]; !present {
		issues = append(issues, mopsZodIssue{
			Expected: "number", Code: "invalid_type", Path: []string{"max_replaces"},
			Message: "Invalid input: expected number, received undefined",
		})
	} else if n, isNum := mopsJSONNumber(v); !isNum {
		issues = append(issues, mopsZodIssue{
			Expected: "number", Code: "invalid_type", Path: []string{"max_replaces"},
			Message: "Invalid input: expected number, received " + mopsJSONType(v),
		})
	} else if n != float64(int(n)) {
		issues = append(issues, mopsZodIssue{
			Expected: "int", Format: "safeint", Code: "invalid_type", Path: []string{"max_replaces"},
			Message: "Invalid input: expected int, received number",
		})
	} else if int(n) < 1 {
		issues = append(issues, mopsZodIssue{
			Origin: "number", Code: "too_small", Minimum: mopsFloatPtr(1), Inclusive: mopsBoolPtr(true),
			Path: []string{"max_replaces"}, Message: "Too small: expected number to be >=1",
		})
	} else if int(n) > 10 {
		issues = append(issues, mopsZodIssue{
			Origin: "number", Code: "too_big", Maximum: mopsFloatPtr(10), Inclusive: mopsBoolPtr(true),
			Path: []string{"max_replaces"}, Message: "Too big: expected number to be <=10",
		})
	} else {
		maxReplaces = int(n)
	}

	if len(issues) > 0 {
		writeErr(w, d, ptErr(http.StatusInternalServerError, mopsZodMessage(issues)))
		return
	}

	internalErr := func(err error) {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
	}

	if brandID != "" {
		brand, err := d.queryRow(r.Context(), "SELECT id FROM brands WHERE id = $1 LIMIT 1", brandID)
		if err != nil {
			internalErr(err)
			return
		}
		if brand == nil {
			writeErr(w, d, errNotFound("Brand not found"))
			return
		}
		existing, err := d.queryRow(r.Context(),
			"SELECT id FROM auto_replace_configs WHERE brand_id = $1 LIMIT 1", brandID)
		if err != nil {
			internalErr(err)
			return
		}
		if existing != nil {
			if err := d.exec(r.Context(),
				"UPDATE auto_replace_configs SET enabled = $1, max_replaces = $2, updated_by = $3, updated_at = NOW() WHERE id = $4",
				enabled, maxReplaces, user.ID, existing.strOr("id", "")); err != nil {
				internalErr(err)
				return
			}
		} else if err := d.exec(r.Context(),
			"INSERT INTO auto_replace_configs (id, brand_id, enabled, max_replaces, created_by, updated_by) VALUES ($1, $2, $3, $4, $5, $6)",
			uuid.NewString(), brandID, enabled, maxReplaces, user.ID, user.ID); err != nil {
			internalErr(err)
			return
		}
	} else {
		existing, err := d.queryRow(r.Context(),
			"SELECT id FROM auto_replace_configs WHERE brand_id IS NULL LIMIT 1")
		if err != nil {
			internalErr(err)
			return
		}
		if existing != nil {
			if err := d.exec(r.Context(),
				"UPDATE auto_replace_configs SET enabled = $1, max_replaces = $2, updated_by = $3, updated_at = NOW() WHERE id = $4",
				enabled, maxReplaces, user.ID, existing.strOr("id", "")); err != nil {
				internalErr(err)
				return
			}
		} else if err := d.exec(r.Context(),
			"INSERT INTO auto_replace_configs (id, brand_id, enabled, max_replaces, created_by, updated_by) VALUES ($1, NULL, $2, $3, $4, $5)",
			uuid.NewString(), enabled, maxReplaces, user.ID, user.ID); err != nil {
			internalErr(err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- config/template-url ----------

const mopsTemplateMetaKey = "bulk_media_template_url"

// mopsSaveTemplateErr: parity `catch { return apiError(500, "Gagal menyimpan
// template URL") }` — lewat WriteErr, pesan di-scrub di production seperti
// apiError app lama.
var mopsSaveTemplateErr = errors.New("Gagal menyimpan template URL")

// ConfigTemplateURLGet: GET /v1/platform/config/template-url
func (d *Deps) ConfigTemplateURLGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		`SELECT value FROM system_metadata WHERE "key" = $1 LIMIT 1`, mopsTemplateMetaKey)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	var value any
	if row != nil {
		value = row.strOr("value", "")
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": value})
}

// ConfigTemplateURLPut: PUT /v1/platform/config/template-url
func (d *Deps) ConfigTemplateURLPut(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	// Parity: `typeof body?.url === "string" ? body.url.trim() : ""`.
	value, _ := strField(decodeJSON(r), "url")

	if value == "" {
		if err := d.exec(r.Context(),
			`DELETE FROM system_metadata WHERE "key" = $1`, mopsTemplateMetaKey); err != nil {
			writeErr(w, d, mopsSaveTemplateErr)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": nil})
		return
	}
	if !mopsJSURLValid(value) {
		writeErr(w, d, errUnprocessable("URL tidak valid"))
		return
	}
	if err := d.exec(r.Context(),
		`INSERT INTO system_metadata ("key", value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT ("key") DO UPDATE SET value = $3, updated_at = NOW()`,
		mopsTemplateMetaKey, value, value); err != nil {
		writeErr(w, d, mopsSaveTemplateErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": value})
}

// mopsJSURLValid: parity `new URL(value)` (WHATWG) secara praktis — cukup untuk
// menolak input salah (tanpa skema/spasi/port tidak valid) tanpa menolak URL
// sah yang dipakai app lama. Parser URL lengkap tidak ditiru.
func mopsJSURLValid(raw string) bool {
	if raw == "" || strings.ContainsAny(raw, " \t\n\r") {
		return false
	}
	colon := strings.Index(raw, ":")
	if colon <= 0 {
		return false
	}
	scheme := strings.ToLower(raw[:colon])
	for i := 0; i < len(scheme); i++ {
		c := scheme[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9', c == '+', c == '-', c == '.':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	// Skema ber-otoritas (`://`) wajib punya host dan port numerik ≤ 65535.
	if strings.HasPrefix(raw[colon+1:], "//") {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return false
		}
		if u.Port() != "" {
			n, pErr := strconv.Atoi(u.Port())
			if pErr != nil || n > 65535 {
				return false
			}
		}
	}
	// Skema opaque (mailto:, tel:, data:) diterima JS selama tidak ada spasi.
	return true
}

// ---------- gatekeeper ----------

// GatekeeperLogs: GET /v1/platform/gatekeeper/logs
func (d *Deps) GatekeeperLogs(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}

	q := r.URL.Query()
	campaignID := q.Get("campaignId")
	status := q.Get("status")

	// parity `Math.max(1, Number(page ?? 1) || 1)` dan
	// `Math.min(Math.max(Number(pageSize ?? limit ?? 20) || 20, 1), 100)`.
	page := mopsAtoiOr(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	pageSizeRaw := q.Get("pageSize")
	if strings.TrimSpace(pageSizeRaw) == "" {
		pageSizeRaw = q.Get("limit")
	}
	pageSize := mopsAtoiOr(pageSizeRaw, 20)
	if pageSize == 0 {
		pageSize = 20
	}
	pageSize = mopsClampInt(pageSize, 1, 100)

	conds := []string{}
	params := []any{}
	if campaignID != "" {
		params = append(params, campaignID)
		conds = append(conds, "g.campaign_id = $"+ptItoa(len(params)))
	}
	if ptContains([]string{"passed", "failed", "review_required", "pending", "running"}, status) {
		params = append(params, status)
		conds = append(conds, "g.status = $"+ptItoa(len(params)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	totalRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*)::int AS n FROM taut_gatekeeper_audits g "+where, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	args := append([]any{}, params...)
	args = append(args, pageSize, (page-1)*pageSize)
	logs, err := d.queryRows(r.Context(),
		`SELECT g.id, g.campaign_id, c.name AS campaign_name, g.media_id, m.name AS media_name,
		        g.status, g.overall_verdict, g.checks, g.created_at, g.updated_at
		 FROM taut_gatekeeper_audits g
		 LEFT JOIN campaigns c ON c.id = g.campaign_id
		 LEFT JOIN media m ON m.id = g.media_id
		 `+where+`
		 ORDER BY g.created_at DESC
		 LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	out := make([]map[string]any, 0, len(logs))
	for _, raw := range logs {
		row := ptRow(ptNormalizeMap(raw))
		// parity `r.checks && typeof r.checks === "object" ? r.checks : {}`
		// (JS: array juga object, jadi hanya skalar yang diganti {}).
		var checks any = map[string]any{}
		if v := row.jsonField("checks"); v != nil {
			switch v.(type) {
			case map[string]any, []any:
				checks = v
			}
		}
		out = append(out, map[string]any{
			"id":             row.strOr("id", ""),
			"campaignId":     row.strOr("campaign_id", ""),
			"campaignName":   ptNullableStr(row, "campaign_name"),
			"mediaId":        row.strOr("media_id", ""),
			"mediaName":      ptNullableStr(row, "media_name"),
			"status":         row.strOr("status", ""),
			"overallVerdict": row.strOr("overall_verdict", ""),
			"checks":         checks,
			"createdAt":      row.anyOrNil("created_at"),
			"updatedAt":      row.anyOrNil("updated_at"),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"logs":  out,
		"total": totalRow.intOr("n", 0),
	})
}

// GatekeeperStats: GET /v1/platform/gatekeeper/stats
func (d *Deps) GatekeeperStats(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}

	summary, err := d.queryRow(r.Context(),
		`SELECT COUNT(*)::int AS total,
		        COUNT(*) FILTER (WHERE overall_verdict = 'passed')::int AS passed,
		        COUNT(*) FILTER (WHERE overall_verdict = 'failed')::int AS failed,
		        COUNT(*) FILTER (WHERE overall_verdict = 'review_required')::int AS review_required
		 FROM taut_gatekeeper_audits`)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	failedRows, err := d.queryRows(r.Context(),
		"SELECT checks FROM taut_gatekeeper_audits WHERE overall_verdict = 'failed'")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// Kategori content safety dihitung dari jsonb di Go (parity post-process JS
	// app lama; jsonb tidak selalu array).
	blockedByCategory := map[string]int{}
	for _, raw := range failedRows {
		row := ptRow(ptNormalizeMap(raw))
		checks, _ := row.jsonField("checks").(map[string]any)
		safety, _ := checks["contentSafety"].(map[string]any)
		categories, isArr := safety["categories"].([]any)
		if !isArr {
			continue
		}
		for _, cat := range categories {
			if key, isStr := cat.(string); isStr {
				blockedByCategory[key]++
			}
		}
	}

	w.Header().Set("Cache-Control", "no-store")
	var total, passed, failed, reviewRequired int
	if summary != nil {
		total = summary.intOr("total", 0)
		passed = summary.intOr("passed", 0)
		failed = summary.intOr("failed", 0)
		reviewRequired = summary.intOr("review_required", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"totalAudits":       total,
		"passed":            passed,
		"failed":            failed,
		"reviewRequired":    reviewRequired,
		"blockedByCategory": blockedByCategory,
	})
}
