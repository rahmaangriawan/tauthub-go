package brands

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/[brandId]/overview/route.ts (READ-ONLY slice dari fase 6/7/8:
// wallet, campaigns, intakes, jobs, publications, dashboardStats, prScoreData,
// trends 7 hari). Tanpa mutasi — parity: request path tidak menjalankan engine
// scoring (prScore dibaca dari snapshot pr_campaign_scores).

func (d *Deps) Overview(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeError(w, err)
		return
	}
	id := brandRow.ID
	now := time.Now()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	sevenDaysAgo := now.AddDate(0, 0, -6)
	sevenStart := time.Date(sevenDaysAgo.Year(), sevenDaysAgo.Month(), sevenDaysAgo.Day(), 0, 0, 0, 0, time.Local)

	// --- wallet summary (port walletSummary: credit_wallets + tc_config.rate_idr) ---
	available, reserved, captured := 0, 0, 0
	rateIDR := 0
	hasWallet := true
	if wErr := d.Pool.QueryRow(r.Context(),
		"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1 LIMIT 1", id).
		Scan(&available, &reserved, &captured); wErr != nil {
		hasWallet = false
	}
	_ = d.Pool.QueryRow(r.Context(), "SELECT rate_idr FROM tc_config LIMIT 1").Scan(&rateIDR)
	wallet := map[string]any{
		"available_tc": available, "reserved_tc": reserved, "captured_tc": captured,
		"total_tc": available + reserved + captured, "rate_idr": rateIDR,
		"available_idr": available * rateIDR, "has_wallet": hasWallet,
	}

	// --- campaigns (+publication_count, parity query overview) ---
	campaigns := []map[string]any{}
	cRows, cErr := d.Pool.Query(r.Context(), `
		SELECT c.id, c.brand_id, c.name, c.description, c.status, c.article_quota, c.budget,
		       c.order_code, c.starts_at, c.ends_at, c.content_intake_id, c.campaign_flow_status,
		       (SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status <> 'cancelled') AS publication_count
		FROM campaigns c WHERE c.brand_id = $1 ORDER BY c.created_at DESC LIMIT 20`, id)
	if cErr == nil {
		defer cRows.Close()
		for cRows.Next() {
			var cid, cbrandID, cname, cstatus string
			var description *string
			var articleQuota int
			var budget float64
			var orderCode, startsAt, endsAt, contentIntakeID, flowStatus *string
			var pubCount int
			if cRows.Scan(&cid, &cbrandID, &cname, &description, &cstatus, &articleQuota, &budget,
				&orderCode, &startsAt, &endsAt, &contentIntakeID, &flowStatus, &pubCount) == nil {
				row := map[string]any{
					"id": cid, "brand_id": cbrandID, "name": cname, "description": description,
					"status": cstatus, "article_quota": articleQuota, "budget": budget,
					"order_code": orderCode, "starts_at": startsAt, "ends_at": endsAt,
					"publication_count": pubCount,
					"articles_created":  0, "published_count": 0, "media_published_count": 0,
					"published_media_names": nil, "tc_used": 0,
					"content_intake_id": contentIntakeID, "campaign_flow_status": flowStatus,
				}
				campaigns = append(campaigns, row)
			}
		}
	}

	// --- intakes (kolom parity) ---
	intakes := []map[string]any{}
	iRows, iErr := d.Pool.Query(r.Context(), `
		SELECT id, brand_id, mode, content_type_code, topic, answers, article_text, article_filename,
		       key_messages, readiness_score, readiness_checks, recommendation, status, job_id, created_by, created_at, updated_at
		FROM content_intakes WHERE brand_id = $1 ORDER BY created_at DESC LIMIT 50`, id)
	if iErr == nil {
		defer iRows.Close()
		for iRows.Next() {
			var iid, bID, mode string
			var contentTypeCode, topic, articleText, articleFilename *string
			var answers, keyMessages, readinessChecks, recommendation []byte
			var readinessScore *int
			var status string
			var jobID, createdBy *string
			var createdAt, updatedAt time.Time
			if iRows.Scan(&iid, &bID, &mode, &contentTypeCode, &topic, &answers, &articleText, &articleFilename,
				&keyMessages, &readinessScore, &readinessChecks, &recommendation, &status, &jobID, &createdBy, &createdAt, &updatedAt) == nil {
				intakes = append(intakes, map[string]any{
					"id": iid, "brand_id": bID, "mode": mode, "content_type_code": contentTypeCode,
					"topic": topic, "answers": jsonRawBytes(answers), "article_text": articleText,
					"article_filename": articleFilename, "key_messages": jsonRawBytes(keyMessages),
					"readiness_score": readinessScore, "readiness_checks": jsonRawBytes(readinessChecks),
					"recommendation": jsonRawBytes(recommendation), "status": status, "job_id": jobID,
					"created_by": createdBy, "created_at": isoMilli(createdAt), "updated_at": isoMilli(updatedAt),
				})
			}
		}
	}

	// --- jobs ---
	jobs := []map[string]any{}
	jRows, jErr := d.Pool.Query(r.Context(), `
		SELECT j.id, j.status, j.current_step, j.progress, j.created_at, j.updated_at,
		       b.topic, b.target_words, d.id
		FROM content_jobs j
		JOIN content_briefs b ON b.id = j.brief_id
		LEFT JOIN content_documents d ON d.job_id = j.id
		WHERE j.brand_id = $1 ORDER BY j.created_at DESC LIMIT 10`, id)
	if jErr == nil {
		defer jRows.Close()
		for jRows.Next() {
			var jid, status, currentStep string
			var progress int
			var createdAt, updatedAt time.Time
			var topic *string
			var targetWords *int
			var documentID *string
			if jRows.Scan(&jid, &status, &currentStep, &progress, &createdAt, &updatedAt, &topic, &targetWords, &documentID) == nil {
				jobs = append(jobs, map[string]any{
					"id": jid, "topic": topic, "status": status, "current_step": currentStep,
					"progress": progress, "target_words": targetWords, "document_id": documentID,
					"created_at": isoMilli(createdAt), "updated_at": isoMilli(updatedAt),
				})
			}
		}
	}

	// --- publications ---
	publications := []map[string]any{}
	pRows, pErr := d.Pool.Query(r.Context(), `
		SELECT p.id, p.document_id, p.media_id, p.status, p.client_price, p.publish_status,
		       p.published_at, p.published_url, p.campaign_id, p.created_at,
		       d.title, m.name, m.category
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		WHERE p.brand_id = $1 ORDER BY p.created_at DESC LIMIT 50`, id)
	if pErr == nil {
		defer pRows.Close()
		for pRows.Next() {
			var pid, documentID, mediaID, status string
			var clientPrice float64
			var publishStatus *string
			var publishedAt *time.Time
			var publishedURL, campaignID *string
			var createdAt time.Time
			var documentTitle, mediaName, mediaCategory *string
			if pRows.Scan(&pid, &documentID, &mediaID, &status, &clientPrice, &publishStatus,
				&publishedAt, &publishedURL, &campaignID, &createdAt,
				&documentTitle, &mediaName, &mediaCategory) == nil {
				var publishedAtISO any
				if publishedAt != nil {
					publishedAtISO = isoMilli(*publishedAt)
				}
				publications = append(publications, map[string]any{
					"id": pid, "document_id": documentID, "media_id": mediaID, "status": status,
					"client_price": clientPrice, "publish_status": publishStatus,
					"published_at": publishedAtISO, "published_url": publishedURL,
					"campaign_id": campaignID, "document_title": documentTitle,
					"media_name": mediaName, "media_category": mediaCategory,
					"created_at": isoMilli(createdAt),
				})
			}
		}
	}

	// --- dashboard stats (parity) ---
	countQ := func(q string, args ...any) int {
		var n int
		_ = d.Pool.QueryRow(r.Context(), q, args...).Scan(&n)
		return n
	}
	inProgress := countQ("SELECT COUNT(*) FROM content_jobs WHERE brand_id = $1 AND status IN ('queued','running','waiting_outline_approval')", id)
	review := countQ("SELECT COUNT(*) FROM content_documents WHERE brand_id = $1 AND status = 'in_review'", id)
	failed := countQ("SELECT COUNT(*) FROM content_jobs WHERE brand_id = $1 AND status = 'failed'", id)
	articles := countQ("SELECT COUNT(*) FROM content_documents WHERE brand_id = $1 AND created_at >= $2", id, month)

	activity := []map[string]any{}
	aRows, aErr := d.Pool.Query(r.Context(),
		"SELECT id, action, target_type, target_id, created_at FROM audit_logs WHERE brand_id = $1 ORDER BY created_at DESC LIMIT 8", id)
	if aErr == nil {
		defer aRows.Close()
		for aRows.Next() {
			var aid, action string
			var targetType, targetID *string
			var createdAt time.Time
			if aRows.Scan(&aid, &action, &targetType, &targetID, &createdAt) == nil {
				activity = append(activity, map[string]any{
					"id": aid, "action": action, "target_type": targetType, "target_id": targetID,
					"created_at": isoMilli(createdAt),
				})
			}
		}
	}

	var monthlyLimit, warningPercent int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT monthly_article_limit, warning_percent FROM brand_ai_quotas WHERE brand_id = $1", id).
		Scan(&monthlyLimit, &warningPercent)
	used := articles
	percent := 0
	if monthlyLimit != 0 {
		percent = int(float64(used) / float64(monthlyLimit) * 1000)
		if percent%10 >= 5 {
			percent = (percent + 5) / 10
		} else {
			percent = percent / 10
		}
	}
	dashboardStats := map[string]any{
		"brand_id": id, "role": brandRow.Role,
		"active_brands": boolToInt(brandRow.Status == "active"), "total_brands": 1,
		"content_in_progress": inProgress, "waiting_review": review, "failed_jobs": failed,
		"monthly_articles": map[string]any{
			"used": used, "limit": monthlyLimit, "percent": percent,
			"warning": percent >= warningPercent,
		},
		"recent_activity": activity,
	}

	// --- prScoreData (read-only snapshot) ---
	var activeCampaign map[string]any
	for _, c := range campaigns {
		if c["status"] == "active" {
			activeCampaign = c
			break
		}
	}
	if activeCampaign == nil && len(campaigns) > 0 {
		activeCampaign = campaigns[0]
	}
	var score any
	if activeCampaign != nil {
		cid, _ := activeCampaign["id"].(string)
		var scoreVal int
		var label *string
		if err := d.Pool.QueryRow(r.Context(),
			"SELECT score, label FROM pr_campaign_scores WHERE campaign_id = $1 ORDER BY calculated_at DESC LIMIT 1",
			cid).Scan(&scoreVal, &label); err == nil {
			score = map[string]any{"score": scoreVal, "label": label}
		}
	}
	prScoreData := map[string]any{
		"campaigns": campaigns, "campaign_info": activeCampaign, "score": score,
	}

	// --- trends 7 hari (generate_series parity) ---
	campaignTrend := d.trendCounts(r, sevenStart, "campaigns", id, "COUNT(c.id)", "")
	contentTrend := d.trendCounts(r, sevenStart, "content_documents", id, "COUNT(cd.id)", "cd")
	publicationTrend := d.trendCountsPub(r, sevenStart, id)

	dailyNet := d.trendNetTC(r, sevenStart, id)
	creditTrend := make([]int, len(dailyNet))
	runningBalance := available
	for i := len(dailyNet) - 1; i >= 0; i-- {
		creditTrend[i] = runningBalance
		runningBalance -= dailyNet[i]
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=5, stale-while-revalidate=30")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"brand_id":  id,
		"public_id": derefStr2(brandRow.PublicID),
		"user": map[string]any{
			"id": user.ID, "email": user.Email, "name": nil, "role": user.Role,
		},
		"wallet": wallet, "campaigns": campaigns, "intakes": intakes,
		"jobs": jobs, "publications": publications,
		"dashboardStats": dashboardStats, "prScoreData": prScoreData,
		"trends": map[string]any{
			"campaigns": campaignTrend, "content": contentTrend,
			"publications": publicationTrend, "credit": creditTrend,
		},
	})
}

// trendCounts: sparkline 7 hari via generate_series (hari kosong = 0).
func (d *Deps) trendCounts(r *http.Request, sevenStart time.Time, table, brandID, countExpr, alias string) []int {
	q := `SELECT d.day::date AS day, ` + countExpr + ` AS count
		FROM generate_series($1::date, $2::date, '1 day') AS d(day)
		LEFT JOIN ` + table + ` ` + aliasOrEmpty(alias) + ` ON ` + joinCond(table, alias) + `
		GROUP BY d.day ORDER BY d.day`
	return d.scanTrend(r, q, sevenStart, time.Now(), brandID)
}

func (d *Deps) trendCountsPub(r *http.Request, sevenStart time.Time, brandID string) []int {
	q := `SELECT d.day::date AS day, COUNT(p.id) AS count
		FROM generate_series($1::date, $2::date, '1 day') AS d(day)
		LEFT JOIN publications p ON p.brand_id = $3 AND p.status = 'published' AND date_trunc('day', p.created_at) = d.day
		GROUP BY d.day ORDER BY d.day`
	return d.scanTrend(r, q, sevenStart, time.Now(), brandID)
}

func (d *Deps) trendNetTC(r *http.Request, sevenStart time.Time, brandID string) []int {
	q := `SELECT d.day::date AS day, COALESCE(SUM(e.amount_tc), 0) AS net_tc
		FROM generate_series($1::date, $2::date, '1 day') AS d(day)
		LEFT JOIN credit_ledger_entries e ON e.brand_id = $3 AND date_trunc('day', e.created_at) = d.day
		GROUP BY d.day ORDER BY d.day`
	return d.scanTrend(r, q, sevenStart, time.Now(), brandID)
}

func (d *Deps) scanTrend(r *http.Request, q string, start, end time.Time, brandID string) []int {
	rows, err := d.Pool.Query(r.Context(), q, start, end, brandID)
	if err != nil {
		return []int{}
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var day time.Time
		var count int
		if rows.Scan(&day, &count) == nil {
			out = append(out, count)
		}
	}
	return out
}

func aliasOrEmpty(alias string) string {
	if alias == "" {
		return ""
	}
	return alias
}

func joinCond(table, alias string) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	return prefix + "brand_id = $3 AND date_trunc('day', " + prefix + "created_at) = d.day"
}

func jsonRawBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	return v
}

func derefStr2(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
