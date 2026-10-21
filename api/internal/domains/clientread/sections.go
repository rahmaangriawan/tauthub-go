package clientread

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
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
		var orderCode, contentIntakeID, flowStatus *string
		var startsAt, endsAt *time.Time
		var createdAt time.Time
		var pubCount, articlesCreated, publishedCount, mediaPublishedCount int
		var publishedMediaNames *string
		var tcUsed int
		if scanErr := rowsQ.Scan(&id, &brandID, &name, &description, &cstatus, &articleQuota, &budget, &orderCode,
			&startsAt, &endsAt, &contentIntakeID, &flowStatus, &createdAt,
			&pubCount, &articlesCreated, &publishedCount, &mediaPublishedCount, &publishedMediaNames, &tcUsed); scanErr != nil {
			slog.Error("[clientread] scan kampanye gagal", "err", scanErr)
			continue
		}
		campaigns = append(campaigns, map[string]any{
			"id": id, "brand_id": brandID, "name": name, "description": description, "status": cstatus,
			"article_quota": articleQuota, "budget": budget, "order_code": orderCode,
			"starts_at": nilIfNilTime(startsAt), "ends_at": nilIfNilTime(endsAt), "content_intake_id": contentIntakeID,
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

	// Filter daftar (parity app lama). state/problematic khusus: "problematic"
	// berarti state IS NULL OR state IN ('gone','unreachable').
	stateFilter := strings.TrimSpace(q.Get("state"))
	if stateFilter != "" && stateFilter != "all" && stateFilter != "problematic" &&
		!strIn([]string{"live", "gone", "unreachable", "no_url"}, stateFilter) {
		httpx.Error(w, http.StatusUnprocessableEntity, "state harus salah satu dari live, gone, unreachable, no_url, problematic")
		return
	}
	searchQ := strings.TrimSpace(q.Get("q"))
	if len(searchQ) > 120 {
		searchQ = searchQ[:120]
	}
	mediaFilter := strings.TrimSpace(q.Get("media"))
	daysFilter := clampInt(q.Get("days"), 0, 0, 3650)

	if view == "summary" {
		var watched, live, gone, unreachable, noURL int
		var lastChecked *time.Time
		_ = d.Pool.QueryRow(r.Context(), `
			SELECT COUNT(*)::int,
			       COUNT(*) FILTER (WHERE c.state = 'live')::int,
			       COUNT(*) FILTER (WHERE c.state = 'gone')::int,
			       COUNT(*) FILTER (WHERE c.state = 'unreachable')::int,
			       COUNT(*) FILTER (WHERE c.state = 'no_url')::int,
			       MAX(c.last_checked_at)
			FROM publications p
			LEFT JOIN publication_link_checks c ON c.publication_id = p.id
			JOIN media m ON m.id = p.media_id
			WHERE p.brand_id = $1 AND p.status = 'published' AND p.published_url IS NOT NULL
			  AND `+db.LinkWarrantySQL("p", "m")+`
			  AND p.publish_status <> 'draft'`, brandID).
			Scan(&watched, &live, &gone, &unreachable, &noURL, &lastChecked)

		// Jeda pengecekan manual (1x sehari) dibaca dari audit_logs — sumber yang
		// sama dengan penegakan di server, supaya tombol di UI tidak menipu.
		var lastManual *time.Time
		_ = d.Pool.QueryRow(r.Context(), `
			SELECT MAX(created_at) FROM audit_logs
			 WHERE brand_id = $1 AND action = 'link_monitor.manual_check'`, brandID).Scan(&lastManual)
		var nextManual any
		manualAvailable := true
		if lastManual != nil {
			if next := lastManual.Add(24 * time.Hour); next.After(time.Now()) {
				nextManual = tsOrNil(&next)
				manualAvailable = false
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"watched": watched, "live": live, "gone": gone,
			"unreachable": unreachable, "no_url": noURL,
			"last_checked_at":        tsOrNil(lastChecked),
			"next_manual_check_at":   nextManual,
			"manual_check_available": manualAvailable,
		})
		return
	}

	// view=publications (default): daftar artikel dengan status link.
	// Publikasi yang diterbitkan sebagai DRAF di website mitra tidak pernah masuk
	// pelacak 404: URL-nya belum bisa diakses publik, jadi pengecekan hanya akan
	// melaporkan "hilang" palsu.
	clause := "WHERE p.brand_id = $1 AND p.status = 'published' AND p.published_url IS NOT NULL" +
		" AND " + db.LinkWarrantySQL("p", "m") +
		" AND p.publish_status <> 'draft'"
	args := []any{brandID}
	addArg := func(v any) string {
		args = append(args, v)
		return "$" + itoa(len(args))
	}
	switch stateFilter {
	case "":
		// tanpa filter state
	case "problematic":
		clause += " AND (c.state IS NULL OR c.state IN ('gone','unreachable'))"
	case "all":
		// semua state
	default:
		clause += " AND c.state = " + addArg(stateFilter)
	}
	if searchQ != "" {
		like := "%" + searchQ + "%"
		clause += " AND (d.title ILIKE " + addArg(like) + " OR m.name ILIKE " + addArg(like) +
			" OR p.published_url ILIKE " + addArg(like) + ")"
	}
	if mediaFilter != "" {
		clause += " AND m.name = " + addArg(mediaFilter)
	}
	if daysFilter > 0 {
		clause += " AND p.published_at > NOW() - make_interval(days => " + addArg(daysFilter) + ")"
	}

	var total int
	_ = d.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM publications p
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id `+clause, args...).Scan(&total)

	limitArg := addArg(limit)
	offsetArg := addArg(offset)
	rowsQ, err := d.Pool.Query(r.Context(), `
		SELECT p.id, p.document_id, p.media_id, p.published_url, p.published_at, d.title, m.name,
		       COALESCE(c.state, 'no_url') AS state,
		       c.last_checked_at, c.http_status, c.consecutive_failures, c.error_message,
		       c.final_url, c.first_detected_gone_at,
		       c.in_article_state, c.in_article_anchor, c.in_article_found,
		       c.in_article_checked_at, c.in_article_error,
		       COALESCE(m.domain_verified, FALSE) AS domain_verified,
		       `+db.LinkWarrantySQL("p", "m")+` AS in_warranty,
		       g.id AS claim_id, g.status AS claim_status, g.refund_idr, g.refund_tc
		FROM publications p
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		LEFT JOIN link_guardian_claims g ON g.publication_id = p.id
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id `+clause+`
		ORDER BY p.published_at DESC LIMIT `+limitArg+` OFFSET `+offsetArg, args...)
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
		var inArticleState, inArticleAnchor, inArticleError *string
		var inArticleFound *int
		var inArticleCheckedAt *time.Time
		var domainVerified, inWarranty bool
		var claimID, claimStatus *string
		var refundIDR, refundTC *int
		if rowsQ.Scan(&id, &documentID, &mediaID, &publishedURL, &publishedAt, &title, &mediaName,
			&state, &lastCheckedAt, &httpStatus, &consecutiveFailures, &lastError,
			&finalURL, &firstGoneAt,
			&inArticleState, &inArticleAnchor, &inArticleFound, &inArticleCheckedAt, &inArticleError,
			&domainVerified, &inWarranty,
			&claimID, &claimStatus, &refundIDR, &refundTC) == nil {
			items = append(items, map[string]any{
				"id": id, "document_id": documentID, "media_id": mediaID,
				"published_url": publishedURL, "published_at": tsOrNil(publishedAt),
				"document_title": title, "media_name": mediaName, "state": state,
				"last_checked_at": tsOrNil(lastCheckedAt), "http_status": httpStatus,
				"consecutive_failures": consecutiveFailures, "error_message": lastError,
				"final_url": finalURL, "first_detected_gone_at": tsOrNil(firstGoneAt),
				// Pemeriksaan tautan di dalam artikel (Link Guardian).
				"in_article_state":      inArticleState,
				"in_article_anchor":     inArticleAnchor,
				"in_article_found":      inArticleFound,
				"in_article_checked_at": tsOrNil(inArticleCheckedAt),
				"in_article_error":      inArticleError,
				"domain_verified":       domainVerified,
				"in_warranty":           inWarranty,
				// Klaim garansi (bila sudah pernah diajukan brand ini).
				"claim_id":     claimID,
				"claim_status": claimStatus,
				"refund_idr":   refundIDR,
				"refund_tc":    refundTC,
			})
		}
	}
	// "publications" = nama kunci yang dibaca UI app lama; "items" dipertahankan
	// untuk pemakai lama.
	writeJSON(w, http.StatusOK, map[string]any{
		"publications": items, "items": items, "total": total,
		"limit": limit, "offset": offset, "view": view,
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
		"publication_id":  nilIfEmpty(publicationID),
		"totals":          totals,
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

// nilIfNilTime: *time.Time → ISO-8601 millis atau nil.
func nilIfNilTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return isoMilli(*t)
}

// strIn: cek keanggotaan string dalam slice.
func strIn(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
