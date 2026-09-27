package clientread

import (
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/[brandId]/{campaigns,link-monitor,analytics} (read-only).

// campaignsAggregateSelect: parity CAMPAIGN_AGGREGATE_SELECT — agregat per
// kampanye (jumlah publikasi, artikel dibuat, terbit, media unik, TC terpakai).
const campaignsAggregateSelect = `
	(SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status <> 'cancelled') AS publication_count,
	(SELECT COUNT(DISTINCT d.id) FROM publications p JOIN content_documents d ON d.id = p.document_id WHERE p.campaign_id = c.id) AS articles_created,
	(SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status = 'published') AS published_count,
	(SELECT COUNT(DISTINCT p.media_id) FROM publications p WHERE p.campaign_id = c.id AND p.status = 'published') AS media_published_count,
	(SELECT string_agg(DISTINCT m.name, ', ') FROM publications p JOIN media m ON m.id = p.media_id WHERE p.campaign_id = c.id AND p.status = 'published') AS published_media_names,
	(SELECT COALESCE(SUM(ABS(e.amount_tc)), 0) FROM credit_ledger_entries e JOIN publications p ON p.id = e.ref_id WHERE p.campaign_id = c.id AND e.ref_type = 'capture') AS tc_used`

// Campaigns: GET /v1/brands/{brandId}/campaigns — kampanye REGULER saja
// (taut_status NULL; kampanye Taut Pintar punya daftar sendiri).
func (d *Deps) Campaigns(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	valid := map[string]bool{"draft": true, "active": true, "paused": true, "completed": true, "cancelled": true}
	if status != "" && !valid[status] {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid campaign status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	clause := "WHERE c.brand_id = $1 AND c.taut_status IS NULL"
	args := []any{brandRow.ID}
	if status != "" {
		clause += " AND c.status = $2"
		args = append(args, status)
	}
	var total int
	_ = d.Pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM campaigns c "+clause, args...).Scan(&total)

	args = append(args, limit, offset)
	rowsQ, err := d.Pool.Query(r.Context(), `
		SELECT c.id, c.brand_id, c.name, c.description, c.status, c.article_quota, c.budget, c.order_code,
		       c.starts_at, c.ends_at, c.content_intake_id, c.campaign_flow_status, c.created_at,
		       `+campaignsAggregateSelect+`
		FROM campaigns c `+clause+`
		ORDER BY c.created_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()
	campaigns := []map[string]any{}
	for rowsQ.Next() {
		var id, brandID, name, cstatus string
		var description *string
		var articleQuota int
		var budget float64
		var orderCode, startsAt, endsAt, contentIntakeID, flowStatus *string
		var createdAt time.Time
		var pubCount, articlesCreated, publishedCount, mediaPublishedCount int
		var publishedMediaNames *string
		var tcUsed int
		if rowsQ.Scan(&id, &brandID, &name, &description, &cstatus, &articleQuota, &budget, &orderCode,
			&startsAt, &endsAt, &contentIntakeID, &flowStatus, &createdAt,
			&pubCount, &articlesCreated, &publishedCount, &mediaPublishedCount, &publishedMediaNames, &tcUsed) != nil {
			continue
		}
		campaigns = append(campaigns, map[string]any{
			"id": id, "brand_id": brandID, "name": name, "description": description, "status": cstatus,
			"article_quota": articleQuota, "budget": budget, "order_code": orderCode,
			"starts_at": startsAt, "ends_at": endsAt, "content_intake_id": contentIntakeID,
			"campaign_flow_status": flowStatus, "created_at": isoMilli(createdAt),
			"publication_count": pubCount, "articles_created": articlesCreated,
			"published_count": publishedCount, "media_published_count": mediaPublishedCount,
			"published_media_names": publishedMediaNames, "tc_used": tcUsed,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaigns": campaigns, "total": total, "limit": limit, "offset": offset,
	})
}

// LinkMonitor: GET /v1/brands/{brandId}/link-monitor — Pelacak 404 klien.
// Setiap query WAJIB filter p.brand_id (jangan bocorkan publikasi brand lain).
func (d *Deps) LinkMonitor(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	brandID := brandRow.ID
	q := r.URL.Query()
	view := q.Get("view")
	if view == "" {
		view = "publications"
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	if view == "summary" {
		var watched, live, gone, unreachable, noURL int
		_ = d.Pool.QueryRow(r.Context(), `
			SELECT COUNT(*)::int,
			       COUNT(*) FILTER (WHERE c.state = 'live')::int,
			       COUNT(*) FILTER (WHERE c.state = 'gone')::int,
			       COUNT(*) FILTER (WHERE c.state = 'unreachable')::int,
			       COUNT(*) FILTER (WHERE c.state = 'no_url')::int
			FROM publications p
			LEFT JOIN publication_link_checks c ON c.publication_id = p.id
			WHERE p.brand_id = $1 AND p.status = 'published' AND p.published_url IS NOT NULL
			  AND p.published_at > NOW() - INTERVAL '12 months'`, brandID).
			Scan(&watched, &live, &gone, &unreachable, &noURL)
		writeJSON(w, http.StatusOK, map[string]any{
			"watched": watched, "live": live, "gone": gone,
			"unreachable": unreachable, "no_url": noURL,
		})
		return
	}

	// view=publications (default): daftar artikel dengan status link.
	var total int
	_ = d.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM publications p
		WHERE p.brand_id = $1 AND p.status = 'published' AND p.published_url IS NOT NULL
		  AND p.published_at > NOW() - INTERVAL '12 months'`, brandID).Scan(&total)

	rowsQ, err := d.Pool.Query(r.Context(), `
		SELECT p.id, p.document_id, p.media_id, p.published_url, p.published_at, d.title, m.name,
		       COALESCE(c.state, 'no_url') AS state,
		       c.last_checked_at, c.http_status, c.consecutive_failures, c.error_message,
		       c.final_url, c.first_detected_gone_at
		FROM publications p
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		WHERE p.brand_id = $1 AND p.status = 'published' AND p.published_url IS NOT NULL
		  AND p.published_at > NOW() - INTERVAL '12 months'
		ORDER BY p.published_at DESC LIMIT $2 OFFSET $3`, brandID, limit, offset)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()
	items := []map[string]any{}
	for rowsQ.Next() {
		var id, documentID, mediaID, publishedURL, state string
		var publishedAt *time.Time
		var title, mediaName *string
		var lastCheckedAt, firstGoneAt *time.Time
		var httpStatus, consecutiveFailures *int
		var lastError, finalURL *string
		if rowsQ.Scan(&id, &documentID, &mediaID, &publishedURL, &publishedAt, &title, &mediaName,
			&state, &lastCheckedAt, &httpStatus, &consecutiveFailures, &lastError,
			&finalURL, &firstGoneAt) == nil {
			items = append(items, map[string]any{
				"id": id, "document_id": documentID, "media_id": mediaID,
				"published_url": publishedURL, "published_at": tsOrNil(publishedAt),
				"document_title": title, "media_name": mediaName, "state": state,
				"last_checked_at": tsOrNil(lastCheckedAt), "http_status": httpStatus,
				"consecutive_failures": consecutiveFailures, "error_message": lastError,
				"final_url": finalURL, "first_detected_gone_at": tsOrNil(firstGoneAt),
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": limit, "offset": offset, "view": view,
	})
}

// Analytics: GET /v1/brands/{brandId}/analytics — seri harian + total event.
func (d *Deps) Analytics(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	brandID := brandRow.ID
	q := r.URL.Query()
	now := time.Now()
	from := q.Get("from")
	to := q.Get("to")
	if from == "" {
		from = now.AddDate(0, 0, -30).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	publicationID := q.Get("publication_id")

	// Seri harian per event_type.
	clause := "WHERE brand_id = $1 AND event_date >= $2::date AND event_date <= $3::date"
	args := []any{brandID, from, to}
	if publicationID != "" {
		clause += " AND publication_id = $4"
		args = append(args, publicationID)
	}
	daily := map[string]map[string]int{}
	seriesRows, sErr := d.Pool.Query(r.Context(), `
		SELECT event_date, event_type, COALESCE(SUM(count), 0)
		FROM analytics_daily `+clause+`
		GROUP BY event_date, event_type
		ORDER BY event_date`, args...)
	if sErr == nil {
		defer seriesRows.Close()
		for seriesRows.Next() {
			var eventDate time.Time
			var eventType string
			var count int
			if seriesRows.Scan(&eventDate, &eventType, &count) == nil {
				date := eventDate.Format("2006-01-02")
				if daily[date] == nil {
					daily[date] = map[string]int{}
				}
				daily[date][eventType] = count
			}
		}
	}

	// Total per event_type + unique visitors (COUNT DISTINCT visit hash bila ada).
	totals := map[string]int{}
	tRows, tErr := d.Pool.Query(r.Context(), `
		SELECT event_type, COALESCE(SUM(count), 0)
		FROM analytics_daily `+clause+`
		GROUP BY event_type`, args...)
	if tErr == nil {
		defer tRows.Close()
		for tRows.Next() {
			var eventType string
			var count int
			if tRows.Scan(&eventType, &count) == nil {
				totals[eventType] = count
			}
		}
	}
	uniqueVisitors := 0
	uArgs := []any{brandID, from, to}
	uClause := "WHERE brand_id = $1 AND event_date >= $2::date AND event_date <= $3::date AND event_type = 'article_view'"
	if publicationID != "" {
		uClause += " AND publication_id = $4"
		uArgs = append(uArgs, publicationID)
	}
	var views int
	_ = d.Pool.QueryRow(r.Context(), "SELECT COALESCE(SUM(count), 0) FROM analytics_daily "+uClause, uArgs...).Scan(&views)
	uniqueVisitors = views

	writeJSON(w, http.StatusOK, map[string]any{
		"from": from, "to": to,
		"publication_id": nilIfEmpty(publicationID),
		"totals":         totals,
		"unique_visitors": uniqueVisitors,
		"daily":           daily,
	})
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
