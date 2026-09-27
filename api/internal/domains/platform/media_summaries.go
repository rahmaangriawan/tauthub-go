package platform

// media_summaries.go: monitoring "Kesimpulan Website" (AI summary) per media —
// GET /v1/platform/media-summaries dan POST /v1/platform/media-summaries/{mediaId}/regenerate.
// Port app/v1/platform/media-summaries/route.ts + [mediaId]/regenerate/route.ts
// dan bagian requestMediaAiSummaryRegeneration di lib/media/ai-summary.ts.

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// mpSummaryEventJSON: satu baris event tracking apa adanya (page_tried sudah jsonb).
func mpSummaryEventJSON(row ptRow) map[string]any {
	return map[string]any{
		"id":            row.anyOrNil("id"),
		"media_id":      row.anyOrNil("media_id"),
		"media_name":    row.anyOrNil("media_name"),
		"website_url":   row.anyOrNil("website_url"),
		"status":        row.anyOrNil("status"),
		"pages_tried":   row.jsonField("pages_tried"),
		"error_message": row.anyOrNil("error_message"),
		"attempt":       row.intOr("attempt", 0),
		"created_at":    row.anyOrNil("created_at"),
	}
}

// ---------- GET /v1/platform/media-summaries ----------

func (d *Deps) MediaSummariesList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	search := strings.TrimSpace(q.Get("search"))
	if len(search) > 120 {
		search = search[:120]
	}
	offset := mpClampInt(q.Get("offset"), 0, 0, 1<<30)
	limit := mpClampInt(q.Get("limit"), 50, 1, 200)

	filters := []string{}
	values := []any{}
	if status != "" {
		values = append(values, status)
		filters = append(filters, "e.status = $"+ptItoa(len(values)))
	}
	if search != "" {
		pattern := "%" + search + "%"
		values = append(values, pattern, pattern)
		filters = append(filters, "(e.media_name ILIKE $"+ptItoa(len(values)-1)+" OR e.website_url ILIKE $"+ptItoa(len(values))+")")
	}
	where := ""
	if len(filters) > 0 {
		where = "WHERE " + strings.Join(filters, " AND ")
	}

	countRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) AS total FROM (
		  SELECT DISTINCT ON (e.media_id) e.media_id FROM media_ai_summary_events e `+where+`
		) latest`, values...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// Satu media = satu baris kondisi terkini (event terbaru) supaya domain yang
	// sama tidak tampil dobel di monitoring.
	items, err := d.queryRows(r.Context(), `
		SELECT * FROM (
		  SELECT DISTINCT ON (e.media_id)
		         e.id, e.media_id, e.media_name, e.website_url, e.status, e.pages_tried, e.error_message, e.attempt, e.created_at
		  FROM media_ai_summary_events e `+where+`
		  ORDER BY e.media_id, e.created_at DESC
		) latest
		ORDER BY latest.created_at DESC LIMIT $`+ptItoa(len(values)+1)+` OFFSET $`+ptItoa(len(values)+2),
		append(append([]any{}, values...), limit, offset)...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	eventMetrics, err := d.queryRows(r.Context(), "SELECT status, COUNT(*)::int AS c FROM media_ai_summary_events GROUP BY status")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	jobMetrics, err := d.queryRows(r.Context(), "SELECT status, COUNT(*)::int AS c FROM jobs WHERE kind = 'media_ai_summary' GROUP BY status")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	pick := func(list []rowMap, key string) int {
		for _, raw := range list {
			row := ptRow(ptNormalizeMap(raw))
			if row.strOr("status", "") == key {
				return row.intOr("c", 0)
			}
		}
		return 0
	}

	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		out = append(out, mpSummaryEventJSON(ptRow(ptNormalizeMap(raw))))
	}
	total := 0
	if countRow != nil {
		total = ptRow(ptNormalizeMap(countRow)).intOr("total", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total": total,
		"items": out,
		"metrics": map[string]any{
			"queued_events": pick(eventMetrics, "queued"),
			"success":       pick(eventMetrics, "success"),
			"failed":        pick(eventMetrics, "failed"),
			"jobs_pending":  pick(jobMetrics, "pending"),
			"jobs_running":  pick(jobMetrics, "running"),
			"jobs_failed":   pick(jobMetrics, "failed"),
		},
	})
}

// ---------- POST /v1/platform/media-summaries/{mediaId}/regenerate ----------

func (d *Deps) MediaSummaryRegenerate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")

	media, err := d.queryRow(r.Context(), "SELECT id, name, website_url FROM media WHERE id = $1::text LIMIT 1", mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if media == nil {
		writeErr(w, d, errNotFound("Media tidak ditemukan"))
		return
	}
	row := ptRow(ptNormalizeMap(media))
	websiteURL := strings.TrimSpace(row.strOr("website_url", ""))
	if websiteURL == "" {
		writeErr(w, d, errConflict("Media tidak memiliki website_url untuk ditarik AI"))
		return
	}

	if err := d.exec(r.Context(), "UPDATE media SET ai_summary = NULL, ai_summary_generated_at = NULL WHERE id = $1::text", mediaID); err != nil {
		writeErr(w, d, err)
		return
	}
	pending, err := d.queryRow(r.Context(),
		"SELECT id FROM jobs WHERE kind = 'media_ai_summary' AND status IN ('pending', 'running') AND payload->>'mediaId' = $1::text LIMIT 1", mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	enqueued := false
	if pending == nil {
		if _, err := d.mpEnqueueJob(r.Context(), "media_ai_summary", map[string]any{"mediaId": mediaID}, 0); err == nil {
			enqueued = true
		}
	}
	d.mpRecordSummaryEvent(r.Context(), mediaID, row.strOr("name", ""), websiteURL, "queued", nil, "")

	// Audit best-effort (detail jsonb).
	if err := d.exec(r.Context(),
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, detail) VALUES (gen_random_uuid(), $1, 'platform.media_summary_regenerated', 'media', $2, $3::jsonb)",
		user.ID, mediaID, mpJSONString(map[string]any{"enqueued": enqueued})); err != nil {
		d.logWarn("audit summary regenerate gagal", mediaID, err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enqueued": enqueued})
}

// mpRecordSummaryEvent: port recordSummaryEvent — kegagalan logging dilewati.
func (d *Deps) mpRecordSummaryEvent(ctx context.Context, mediaID, mediaName, websiteURL, status string, pages any, errorMessage string) {
	if pages == nil {
		pages = []any{}
	}
	if err := d.exec(ctx, `
		INSERT INTO media_ai_summary_events (id, media_id, media_name, website_url, status, pages_tried, error_message, attempt)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)`,
		uuid.NewString(), mediaID, nullIfEmpty(mediaName), nullIfEmpty(websiteURL), status,
		mpJSONString(pages), mpNullableArgStrOrNil(errorMessage, errorMessage != ""), 0); err != nil {
		d.logWarn("catat event summary gagal", mediaID, err)
	}
}
