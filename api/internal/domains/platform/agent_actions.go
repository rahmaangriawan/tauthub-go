// Endpoint listing untuk aksi yang diusulkan agent (Hermes / Claude Code via
// MCP). Panel admin /admin/agent-actions memakai ini untuk menampilkan aksi
// yang menunggu/berjalan/selesai dan memberi tombol "Setujui di Telegram"
// bila kartu bot bermasalah.
package platform

import (
	"net/http"
	"strings"
)

// AgentActionsList: GET /v1/platform/agent-actions
// Query:
//   status: filter status (proposed|executing|executed|failed|rejected|expired)
//   tool:   filter nama tool
//   limit:  default 50, max 500
//   offset: default 0
func (d *Deps) AgentActionsList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	tool := strings.TrimSpace(q.Get("tool"))
	limit, offset := pageParams(r)

	where := []string{"TRUE"}
	args := []any{}
	next := func(v any) string {
		args = append(args, v)
		return "$" + ptItoa(len(args))
	}
	if status != "" {
		where = append(where, "aa.status = "+next(status))
	}
	if tool != "" {
		where = append(where, "aa.tool = "+next(tool))
	}

	rows, err := d.queryRows(r.Context(), `
		SELECT aa.id, aa.code, aa.tool, aa.summary, aa.required_permission,
		       aa.status, aa.approved_by::text, aa.telegram_chat_id, aa.telegram_message_id,
		       aa.error, aa.created_at, aa.decided_at, aa.executed_at, aa.expires_at,
		       ac.name AS client_name,
		       (SELECT email FROM users WHERE id = aa.approved_by) AS approver_email
		FROM agent_actions aa
		LEFT JOIN agent_clients ac ON ac.id = aa.client_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY aa.created_at DESC
		LIMIT `+next(limit)+` OFFSET `+next(offset),
		args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	// Hitung total tanpa limit/offset (dua argumen terakhir dibuang).
	countArgs := args[:len(args)-2]
	total := 0
	if row, cerr := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS total FROM agent_actions aa WHERE "+strings.Join(where, " AND "),
		countArgs...); cerr == nil && row != nil {
		total = int(row.numOr("total"))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  ptRows(rows),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// AgentActionGet: GET /v1/platform/agent-actions/{id}
// Mengembalikan baris penuh termasuk params dan result (jsonb).
func (d *Deps) AgentActionGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	id := chiParam(r, "id")
	row, err := d.queryRow(r.Context(), `
		SELECT aa.id, aa.code, aa.tool, aa.summary, aa.required_permission,
		       aa.status, aa.approved_by::text, aa.telegram_chat_id, aa.telegram_message_id,
		       aa.params::text AS params, COALESCE(aa.result::text, '') AS result,
		       COALESCE(aa.error, '') AS error, aa.created_at, aa.decided_at, aa.executed_at, aa.expires_at,
		       ac.name AS client_name,
		       (SELECT email FROM users WHERE id = aa.approved_by) AS approver_email
		FROM agent_actions aa LEFT JOIN agent_clients ac ON ac.id = aa.client_id
		WHERE aa.id = $1 LIMIT 1`, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Aksi tidak ditemukan."))
		return
	}
	writeJSON(w, http.StatusOK, ptRow(ptNormalizeMap(row)))
}
