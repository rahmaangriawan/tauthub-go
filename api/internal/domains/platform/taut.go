package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
)

// Modul taut: port app/v1/platform/taut/** + taut-agents/** + taut-agent-logs.
// Semua endpoint di sini super-admin only (parity isSuperAdmin) dan memakai
// guard bersama d.adminGuard lalu cek ctx.IsSuperAdmin().
//
// Helper di berkas ini diberi awalan `pt` (platform taut) supaya tidak bentrok
// dengan helper modul platform lain yang berbagi package.

// Regex modul taut (dibuat sekali di level paket — bukan di dalam handler).
var (
	reTautChannelSlug = regexp.MustCompile(`^[a-z0-9-]+$`)
	reTautAngleSlug   = regexp.MustCompile(`^[a-z0-9_]+$`)
	reTautFence       = regexp.MustCompile("(?i)```(?:json)?")
	reStripTags       = regexp.MustCompile(`<[^>]*>`)
)

// ---------- transaksi ----------

// ptTx: pembungkus pgx.Tx supaya handler menjalankan beberapa statement dalam
// satu transaksi tanpa mengimpor pgx di setiap berkas.
type ptTx struct {
	ctx context.Context
	tx  pgx.Tx
}

func (t *ptTx) exec(sql string, args ...any) error {
	_, err := t.tx.Exec(t.ctx, sql, args...)
	return err
}

// ptWithTx: parity withTransaction app lama.
func (d *Deps) ptWithTx(ctx context.Context, fn func(tx *ptTx) error) error {
	return db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		return fn(&ptTx{ctx: ctx, tx: tx})
	})
}

// ---------- baris dinamis (map kolom → tipe JSON-equivalent) ----------
//
// queryRows di types.go mengembalikan map[string]any apa adanya dari pgx
// (timestamp masih time.Time, jsonb sudah map/[]any). ptRow menormalkan nilai
// agar serialisasi JSON identik dengan RowDataPacket + JSON.stringify(Date).

// ptRow: alias rowMap agar helper pt*/ig* dan rowMap saling kompatibel.
type ptRow = rowMap

func ptRows(rows []rowMap) []rowMap {
	out := make([]rowMap, len(rows))
	for i, m := range rows {
		out[i] = rowMap(ptNormalizeMap(m))
	}
	return out
}

func ptNormalizeMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = ptValue(v)
	}
	return out
}

func ptValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case time.Time:
		return isoMilli(t)
	case *time.Time:
		if t == nil {
			return nil
		}
		return isoMilli(*t)
	case []byte:
		return string(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = ptValue(item)
		}
		return out
	case map[string]any:
		return ptNormalizeMap(t)
	default:
		return v
	}
}

// anyOrNil: nilai kolom apa adanya (sudah ISO untuk timestamp) atau nil.
func (m rowMap) anyOrNil(key string) any {
	v := m.raw(key)
	if v == nil {
		return nil
	}
	return v
}

// ---------- error & pesan validasi ala zod ----------

func ptErr(status int, msg string) error { return adminshared.Err(status, msg) }
func ptErrTooMany(msg string) error      { return adminshared.Err(http.StatusTooManyRequests, msg) }

func ptErrorsAs(err error, target **agentNotConfiguredError) bool { return errors.As(err, target) }

func ptZStringTooBig(max int) string {
	return fmt.Sprintf("String must contain at most %d character(s)", max)
}
func ptZStringTooSmall(min int) string {
	return fmt.Sprintf("String must contain at least %d character(s)", min)
}
func ptZNumberTooBig(max any) string {
	return fmt.Sprintf("Number must be less than or equal to %v", max)
}
func ptZNumberTooSmall(min any) string {
	return fmt.Sprintf("Number must be greater than or equal to %v", min)
}

func ptContains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func ptRoundInt(v float64) int {
	if v < 0 {
		return int(v - 0.5)
	}
	return int(v + 0.5)
}

func ptMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ptItoa(n int) string { return strconv.Itoa(n) }

func ptTruncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func ptNullableStr(m ptRow, key string) any {
	if s, ok := m.str(key); ok {
		return s
	}
	return nil
}

func ptNullableQuery(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func ptJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ptJSONIndent2: JSON.stringify(x, null, 2) untuk kebutuhan prompt teks.
func ptJSONIndent2(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func ptNullableArgStr(v string, has bool) any {
	if !has {
		return nil
	}
	return v
}

func ptNullableArgBool(v bool, has bool) any {
	if !has {
		return nil
	}
	return v
}

func ptNullableArgInt(v int, has bool) any {
	if !has {
		return nil
	}
	return v
}

func ptNullableAnyInt(v int, has bool) any {
	if !has {
		return nil
	}
	return v
}

func ptJSONStringArg(v any) any {
	if v == nil {
		return nil
	}
	return ptJSONString(v)
}

// ptStrArray: normalisasi kolom array/JSON menjadi []string (parity parseArray).
func ptStrArray(v any) []string {
	switch t := v.(type) {
	case nil:
		return []string{}
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	}
	return []string{}
}

// ---------- base prompt (taut_profiler_base_prompt_versions) ----------

// TautBasePromptGet: GET /v1/platform/taut/base-prompt
func (d *Deps) TautBasePromptGet(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	rows, err := d.queryRows(r.Context(),
		"SELECT id, version, status, prompt, change_note, created_by, created_at, published_by, published_at FROM taut_profiler_base_prompt_versions ORDER BY version DESC")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": ptRows(rows)})
}

// TautBasePromptCreate: POST /v1/platform/taut/base-prompt
func (d *Deps) TautBasePromptCreate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	body := decodeJSON(r)
	prompt := ""
	if s, hasStr := strField(body, "prompt"); hasStr {
		if len(s) > 20000 {
			writeErr(w, d, errUnprocessable(ptZStringTooBig(20000)))
			return
		}
		prompt = s
	}
	changeNote := ""
	if s, hasStr := strField(body, "change_note"); hasStr {
		if len(s) > 255 {
			writeErr(w, d, errUnprocessable(ptZStringTooBig(255)))
			return
		}
		changeNote = s
	}
	maxRow, err := d.queryRow(r.Context(), "SELECT COALESCE(MAX(version), 0) AS m FROM taut_profiler_base_prompt_versions")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	nextVersion := maxRow.intOr("m", 0) + 1
	id := uuid.NewString()
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if err := tx.exec("UPDATE taut_profiler_base_prompt_versions SET status = 'archived' WHERE status = 'active'"); err != nil {
			return err
		}
		return tx.exec(
			"INSERT INTO taut_profiler_base_prompt_versions (id, version, status, prompt, change_note, created_by, published_by, published_at) VALUES ($1, $2, 'active', $3, $4, $5, $6, NOW())",
			id, nextVersion, prompt, changeNote, user.ID, user.ID)
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "version": nextVersion})
}

// TautBasePromptVersionGet: GET /v1/platform/taut/base-prompt/{versionId}
func (d *Deps) TautBasePromptVersionGet(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id, version, status, prompt, change_note, created_by, created_at, published_by, published_at FROM taut_profiler_base_prompt_versions WHERE id = $1 LIMIT 1",
		chiParam(r, "versionId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi base prompt tidak ditemukan."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": ptRow(ptNormalizeMap(row))})
}

// TautBasePromptVersionUpdate: PATCH /v1/platform/taut/base-prompt/{versionId}
func (d *Deps) TautBasePromptVersionUpdate(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	versionID := chiParam(r, "versionId")
	body := decodeJSON(r)
	var prompt *string
	if s, hasStr := strField(body, "prompt"); hasStr {
		if len(s) > 20000 {
			writeErr(w, d, errUnprocessable(ptZStringTooBig(20000)))
			return
		}
		prompt = &s
	}
	var changeNote *string
	if s, hasStr := strField(body, "change_note"); hasStr {
		if len(s) > 255 {
			writeErr(w, d, errUnprocessable(ptZStringTooBig(255)))
			return
		}
		changeNote = &s
	}
	row, err := d.queryRow(r.Context(), "SELECT id FROM taut_profiler_base_prompt_versions WHERE id = $1 LIMIT 1", versionID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi base prompt tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE taut_profiler_base_prompt_versions SET prompt = COALESCE($1, prompt), change_note = COALESCE($2, change_note) WHERE id = $3",
		ptClampStrPtr(prompt), ptClampStrPtr(changeNote), versionID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func ptClampStrPtr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// TautBasePromptVersionDelete: DELETE /v1/platform/taut/base-prompt/{versionId}
func (d *Deps) TautBasePromptVersionDelete(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	versionID := chiParam(r, "versionId")
	row, err := d.queryRow(r.Context(), "SELECT id FROM taut_profiler_base_prompt_versions WHERE id = $1 LIMIT 1", versionID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi base prompt tidak ditemukan."))
		return
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*)::int AS c FROM taut_profiler_base_prompt_versions")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if countRow.intOr("c", 0) <= 1 {
		writeErr(w, d, errConflict("Tidak bisa menghapus satu-satunya versi."))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM taut_profiler_base_prompt_versions WHERE id = $1", versionID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.base_prompt_deleted", "taut_profiler_base_prompt_version", versionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautBasePromptVersionActivate: POST /v1/platform/taut/base-prompt/{versionId}/activate
func (d *Deps) TautBasePromptVersionActivate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	versionID := chiParam(r, "versionId")
	row, err := d.queryRow(r.Context(), "SELECT id, status FROM taut_profiler_base_prompt_versions WHERE id = $1 LIMIT 1", versionID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Versi base prompt tidak ditemukan."))
		return
	}
	if row.strOr("status", "") != "draft" {
		writeErr(w, d, errConflict("Hanya versi draft yang dapat dipublish."))
		return
	}
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if err := tx.exec("UPDATE taut_profiler_base_prompt_versions SET status = 'archived' WHERE status = 'active'"); err != nil {
			return err
		}
		return tx.exec("UPDATE taut_profiler_base_prompt_versions SET status = 'active', published_by = $1, published_at = NOW() WHERE id = $2", user.ID, versionID)
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.base_prompt_published", "taut_profiler_base_prompt_version", versionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- ai usage ----------

var tautAgentNames = []string{"profiler", "matchmaker", "writer", "gatekeeper"}
var tautRunStatuses = []string{"done", "failed", "running", "pending", "skipped"}

// TautAIUsage: GET /v1/platform/taut/ai-usage
func (d *Deps) TautAIUsage(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	q := r.URL.Query()
	days := clampInt(q.Get("days"), 30, 1, 365)
	agent := strings.TrimSpace(q.Get("agent"))
	status := strings.TrimSpace(q.Get("status"))
	search := strings.TrimSpace(q.Get("search"))
	page := clampInt(q.Get("page"), 1, 1, 1<<30)
	pageSize := clampInt(q.Get("pageSize"), 20, 1, 100)

	baseConds := []string{"r.created_at >= NOW() - ($1 * INTERVAL '1 day')"}
	baseParams := []any{days}
	if status != "" && ptContains(tautRunStatuses, status) {
		baseParams = append(baseParams, status)
		baseConds = append(baseConds, "r.status = $"+ptItoa(len(baseParams)))
	}
	if search != "" {
		baseParams = append(baseParams, "%"+search+"%")
		baseConds = append(baseConds, "c.name ILIKE $"+ptItoa(len(baseParams)))
	}
	baseWhere := "WHERE " + strings.Join(baseConds, " AND ")

	agentConds := append([]string{}, baseConds...)
	agentParams := append([]any{}, baseParams...)
	if agent != "" && ptContains(tautAgentNames, agent) {
		agentParams = append(agentParams, agent)
		agentConds = append(agentConds, "r.agent = $"+ptItoa(len(agentParams)))
	}
	agentWhere := "WHERE " + strings.Join(agentConds, " AND ")

	summary, err := d.queryRow(r.Context(),
		`SELECT COUNT(*) AS calls,
		        COALESCE(SUM(CASE WHEN r.status = 'done' THEN 1 ELSE 0 END), 0) AS done_calls,
		        COALESCE(SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_calls,
		        COALESCE(SUM(r.token_input), 0) AS token_input,
		        COALESCE(SUM(r.token_output), 0) AS token_output,
		        COALESCE(AVG(CASE WHEN r.status = 'done' THEN r.duration_ms END), 0) AS avg_duration_ms
		 FROM taut_agent_runs r
		 JOIN campaigns c ON c.id = r.campaign_id
		 `+agentWhere, agentParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	byAgent, err := d.queryRows(r.Context(),
		`SELECT r.agent,
		        COUNT(*) AS calls,
		        COALESCE(SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_calls,
		        COALESCE(SUM(r.token_input), 0) AS token_input,
		        COALESCE(SUM(r.token_output), 0) AS token_output,
		        COALESCE(AVG(CASE WHEN r.status = 'done' THEN r.duration_ms END), 0) AS avg_duration_ms
		 FROM taut_agent_runs r
		 JOIN campaigns c ON c.id = r.campaign_id
		 `+baseWhere+`
		 GROUP BY r.agent`, baseParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	totalRow, err := d.queryRow(r.Context(),
		`SELECT COUNT(*) AS total FROM taut_agent_runs r JOIN campaigns c ON c.id = r.campaign_id `+agentWhere, agentParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	total := totalRow.intOr("total", 0)
	offset := (page - 1) * pageSize
	runArgs := append([]any{}, agentParams...)
	runArgs = append(runArgs, pageSize, offset)
	runRows, err := d.queryRows(r.Context(),
		`SELECT r.id, r.agent, r.status, r.model, r.token_input, r.token_output, r.duration_ms,
		        r.error, r.created_at, r.finished_at, r.media_id,
		        c.name AS campaign_name, m.name AS media_name
		 FROM taut_agent_runs r
		 JOIN campaigns c ON c.id = r.campaign_id
		 LEFT JOIN media m ON m.id = r.media_id
		 `+agentWhere+`
		 ORDER BY r.created_at DESC LIMIT $`+ptItoa(len(agentParams)+1)+` OFFSET $`+ptItoa(len(agentParams)+2),
		runArgs...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	s := ptRow(ptNormalizeMap(summary))
	summaryOut := map[string]any{
		"calls":           int(s.numOr("calls")),
		"done_calls":      int(s.numOr("done_calls")),
		"failed_calls":    int(s.numOr("failed_calls")),
		"token_input":     int(s.numOr("token_input")),
		"token_output":    int(s.numOr("token_output")),
		"total_tokens":    int(s.numOr("token_input")) + int(s.numOr("token_output")),
		"avg_duration_ms": ptRoundInt(s.numOr("avg_duration_ms")),
	}
	byAgentOut := make([]map[string]any, 0, len(byAgent))
	for _, raw := range byAgent {
		a := ptRow(ptNormalizeMap(raw))
		ti := int(a.numOr("token_input"))
		to := int(a.numOr("token_output"))
		byAgentOut = append(byAgentOut, map[string]any{
			"agent":           a.strOr("agent", ""),
			"calls":           int(a.numOr("calls")),
			"failed_calls":    int(a.numOr("failed_calls")),
			"token_input":     ti,
			"token_output":    to,
			"total_tokens":    ti + to,
			"avg_duration_ms": ptRoundInt(a.numOr("avg_duration_ms")),
		})
	}
	runsOut := make([]map[string]any, 0, len(runRows))
	for _, raw := range runRows {
		x := ptRow(ptNormalizeMap(raw))
		runsOut = append(runsOut, map[string]any{
			"id":            x.strOr("id", ""),
			"agent":         x.strOr("agent", ""),
			"status":        x.strOr("status", ""),
			"model":         ptNullableStr(x, "model"),
			"token_input":   x.numPtr("token_input"),
			"token_output":  x.numPtr("token_output"),
			"duration_ms":   x.numPtr("duration_ms"),
			"error":         ptNullableStr(x, "error"),
			"created_at":    x.anyOrNil("created_at"),
			"finished_at":   x.anyOrNil("finished_at"),
			"campaign_name": ptNullableStr(x, "campaign_name"),
			"media_name":    ptNullableStr(x, "media_name"),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"filters": map[string]any{
			"days": days, "agent": ptNullableQuery(agent), "status": ptNullableQuery(status),
			"search": ptNullableQuery(search), "page": page, "page_size": pageSize,
		},
		"summary":  summaryOut,
		"by_agent": byAgentOut,
		"total":    total,
		"runs":     runsOut,
	})
}
