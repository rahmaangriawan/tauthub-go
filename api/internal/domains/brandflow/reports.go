package brandflow

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// reports.go: port app/v1/brands/[brandId]/reports/route.ts +
// reports/link-clicks/route.ts (foundation §17).

// ---------- reports ----------

// ReportsGet: GET /v1/brands/{brandId}/reports
func (d *Deps) ReportsGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	canonicalID := brandRow.ID
	q := r.URL.Query()
	now := time.Now().UTC()
	defaultFrom := now.AddDate(0, 0, -30).Format("2006-01-02")
	defaultTo := now.Format("2006-01-02")
	from := bsNormDate(q.Get("from"), defaultFrom)
	to := bsNormDate(q.Get("to"), defaultTo)
	toExclusive := bsDayAfterISO(to)
	fromISO := from + "T00:00:00.000Z"

	totals, uniqueVisitors := d.bsAnalyticsTotals(r.Context(), canonicalID, from, to)
	brandRates := d.bsLoadBrandPrRates(r.Context(), canonicalID)

	publishedRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*)::int AS published,
		       COALESCE(SUM(client_price), 0)::numeric(14,2) AS estimated_media_value
		FROM publications
		WHERE brand_id = $1 AND status = 'published' AND published_at >= $2 AND published_at < $3`,
		canonicalID, fromISO, toExclusive)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	costRow, err := d.queryRow(r.Context(), `
		SELECT COALESCE(SUM(budget), 0)::numeric(14,2) AS campaign_cost
		FROM campaigns
		WHERE brand_id = $1 AND status IN ('active','completed')
		  AND (starts_at IS NULL OR starts_at <= $2) AND (ends_at IS NULL OR ends_at >= $3)`,
		canonicalID, toExclusive, fromISO)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	pubRows, err := d.queryRows(r.Context(), `
		SELECT campaign_id, COUNT(*)::int AS published, COALESCE(SUM(client_price), 0)::numeric(14,2) AS media_value
		FROM publications
		WHERE brand_id = $1 AND status = 'published' AND published_at >= $2 AND published_at < $3 AND campaign_id IS NOT NULL
		GROUP BY campaign_id`, canonicalID, fromISO, toExclusive)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	eventRows, err := d.queryRows(r.Context(), `
		SELECT campaign_id, event_type, COUNT(*)::int AS count
		FROM tracking_events
		WHERE brand_id = $1 AND created_at >= $2 AND created_at < $3 AND campaign_id IS NOT NULL
		GROUP BY campaign_id, event_type`, canonicalID, fromISO, toExclusive)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignRows, err := d.queryRows(r.Context(), `
		SELECT id, name FROM campaigns
		WHERE brand_id = $1 AND status IN ('active','completed')
		  AND (starts_at IS NULL OR starts_at <= $2) AND (ends_at IS NULL OR ends_at >= $3)`,
		canonicalID, toExclusive, fromISO)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	clickRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*)::int AS clicks
		FROM tracking_events
		WHERE brand_id = $1 AND event_type = 'tracking_link_click'
		  AND created_at >= $2 AND created_at < $3`, canonicalID, fromISO, toExclusive)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	views := totals["article_view"]
	outbound := totals["outbound_click"]
	ctr := 0.0
	if views > 0 {
		ctr = math.Round(float64(outbound)/float64(views)*10000) / 100
	}

	value := d.bsEstimatePRValue(r.Context(), canonicalID, nil, from, to, &brandRates.Rates, nil)

	publishedCount := 0
	estimatedMediaValue := 0.0
	if publishedRow != nil {
		publishedCount = publishedRow.intOr("published", 0)
		estimatedMediaValue = publishedRow.numOr("estimated_media_value")
	}
	campaignCost := 0.0
	if costRow != nil {
		campaignCost = costRow.numOr("campaign_cost")
	}
	clickCount := 0
	if clickRow != nil {
		clickCount = clickRow.intOr("clicks", 0)
	}

	type eventCount struct {
		views  int
		clicks int
	}
	eventCounts := map[string]eventCount{}
	for _, row := range eventRows {
		key := row.strOr("campaign_id", "")
		entry := eventCounts[key]
		switch row.strOr("event_type", "") {
		case "article_view":
			entry.views += row.intOr("count", 0)
		case "outbound_click":
			entry.clicks += row.intOr("count", 0)
		}
		eventCounts[key] = entry
	}

	campaignReport := make([]map[string]any, 0, len(campaignRows))
	for _, campaign := range campaignRows {
		campaignID := campaign.strOr("id", "")
		var pub *bsRow
		for i := range pubRows {
			if pubRows[i].strOr("campaign_id", "") == campaignID {
				pub = &pubRows[i]
				break
			}
		}
		events := eventCounts[campaignID]
		publishedInCampaign := 0
		mediaValue := 0.0
		if pub != nil {
			publishedInCampaign = pub.intOr("published", 0)
			mediaValue = pub.numOr("media_value")
		}
		campaignCTR := 0.0
		if events.views > 0 {
			campaignCTR = math.Round(float64(events.clicks)/float64(events.views)*10000) / 100
		}
		campaignReport = append(campaignReport, map[string]any{
			"campaign_id": campaignID, "campaign_name": campaign.raw("name"),
			"published": publishedInCampaign, "article_views": events.views,
			"outbound_clicks": events.clicks, "ctr": campaignCTR,
			"estimated_media_value": mediaValue,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from": from, "to": to,
		"summary": map[string]any{
			"published": publishedCount, "article_views": views,
			"outbound_clicks": outbound, "cta_clicks": totals["cta_click"],
			"unique_visitors": uniqueVisitors, "ctr": ctr,
			"estimated_media_value": estimatedMediaValue,
			"estimated_reach":       clickCount,
			"estimated_pr_value":    value.EstimatedPRValue,
			"estimated_saving":      value.EstimatedSaving,
			"campaign_cost":         campaignCost,
			"pr_standard":           bsPrStandardJSON(brandRates),
		},
		"campaigns": campaignReport,
	})
}

// bsAnalyticsTotals: parity analyticsTotals — hitung per event_type + unique
// visitors (distinct session) dari tracking_events dalam satu pass.
func (d *Deps) bsAnalyticsTotals(ctx context.Context, brandID, from, to string) (map[string]int, int) {
	toExclusive := bsDayAfterISO(to)
	rows, err := d.queryRows(ctx, `
		SELECT event_type, COUNT(*)::int AS count,
		       COUNT(DISTINCT session_id) FILTER (WHERE session_id IS NOT NULL AND session_id <> '')::int AS unique_visitors
		FROM tracking_events
		WHERE brand_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY event_type`, brandID, from+"T00:00:00.000Z", toExclusive)
	if err != nil {
		return map[string]int{}, 0
	}
	totals := map[string]int{}
	uniqueVisitors := 0
	for _, row := range rows {
		totals[row.strOr("event_type", "")] = row.intOr("count", 0)
		if v := row.intOr("unique_visitors", 0); v > uniqueVisitors {
			uniqueVisitors = v
		}
	}
	return totals, uniqueVisitors
}

// ---------- reports/link-clicks ----------

// bsClickDetail: parity ClickDetail (lib/tracking/click-detail.ts).
type bsClickDetail struct {
	Device        string
	Browser       string
	OS            string
	ReferrerGroup string
	Geo           any
}

var bsSocialHosts = []string{"facebook", "instagram", "twitter", "x.com", "t.co", "linkedin", "tiktok", "whatsapp", "wa.me", "youtube", "pinterest", "reddit", "telegram"}
var bsSearchHosts = []string{"google", "bing", "yahoo", "duckduckgo", "yandex", "baidu", "ecosia"}

// bsEnrichClick: parity enrichClick.
func bsEnrichClick(ua, referrer string) bsClickDetail {
	s := strings.ToLower(ua)
	device := "desktop"
	switch {
	case strings.Contains(s, "ipad") || strings.Contains(s, "tablet"):
		device = "tablet"
	case strings.Contains(s, "mobi") || strings.Contains(s, "android") || strings.Contains(s, "iphone"):
		device = "mobile"
	}
	browser := "Lainnya"
	switch {
	case strings.Contains(s, "edg/"):
		browser = "Edge"
	case strings.Contains(s, "opr/") || strings.Contains(s, "opera"):
		browser = "Opera"
	case strings.Contains(s, "samsungbrowser"):
		browser = "Samsung Internet"
	case strings.Contains(s, "firefox") || strings.Contains(s, "fxios"):
		browser = "Firefox"
	case strings.Contains(s, "chrome") || strings.Contains(s, "crios"):
		browser = "Chrome"
	case strings.Contains(s, "safari"):
		browser = "Safari"
	case strings.Contains(s, "msie") || strings.Contains(s, "trident"):
		browser = "Internet Explorer"
	}
	osName := "Lainnya"
	switch {
	case strings.Contains(s, "windows"):
		osName = "Windows"
	case strings.Contains(s, "android"):
		osName = "Android"
	case strings.Contains(s, "iphone") || strings.Contains(s, "ipad") || strings.Contains(s, "ipod") || strings.Contains(s, "ios"):
		osName = "iOS"
	case strings.Contains(s, "mac os x") || strings.Contains(s, "macintosh"):
		osName = "macOS"
	case strings.Contains(s, "cros"):
		osName = "Chrome OS"
	case strings.Contains(s, "linux"):
		osName = "Linux"
	}
	ref := strings.ToLower(referrer)
	referrerGroup := "Referral"
	switch {
	case ref == "":
		referrerGroup = "Direct"
	default:
		matchedSearch := false
		for _, host := range bsSearchHosts {
			if strings.Contains(ref, host) {
				matchedSearch = true
				break
			}
		}
		if matchedSearch {
			referrerGroup = "Search"
			break
		}
		for _, host := range bsSocialHosts {
			if strings.Contains(ref, host) {
				referrerGroup = "Social"
				break
			}
		}
	}
	return bsClickDetail{
		Device: device, Browser: browser, OS: osName,
		ReferrerGroup: referrerGroup, Geo: nil,
	}
}

// ReportsLinkClicks: GET /v1/brands/{brandId}/reports/link-clicks
func (d *Deps) ReportsLinkClicks(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	canonicalID := brandRow.ID
	q := r.URL.Query()
	now := time.Now().UTC()
	from := bsNormDate(q.Get("from"), now.AddDate(0, 0, -30).Format("2006-01-02"))
	to := bsNormDate(q.Get("to"), now.Format("2006-01-02"))
	toExclusive := bsDayAfterISO(to)
	fromISO := from + "T00:00:00.000Z"
	// parity `Math.min(Math.max(Number(...) || 100, 1), 300)` — "" → 100.
	limit := bsNumClampInt(bsQueryNumber(q, "limit", 100), 1, 300)
	if q.Has("limit") && bsJSNumber(q.Get("limit")) == 0 {
		limit = 100
	}

	summary, err := d.queryRow(r.Context(), `
		SELECT COUNT(*)::int AS clicks,
		       COUNT(DISTINCT COALESCE(session_id, ip_address, id))::int AS unique_sessions,
		       COUNT(DISTINCT link_id)::int AS unique_links
		FROM tracking_events
		WHERE brand_id = $1 AND event_type = 'tracking_link_click'
		  AND created_at >= $2 AND created_at < $3`, canonicalID, fromISO, toExclusive)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// PRIVASI (UU PDP): IP mentah TIDAK dikirim ke klien.
	itemRows, err := d.queryRows(r.Context(), `
		SELECT e.created_at, l.code, l.kind, e.url, e.session_id, e.ip_address, e.user_agent, e.referrer,
		       c.name AS campaign_name
		FROM tracking_events e
		LEFT JOIN tracking_links l ON l.id = e.link_id
		LEFT JOIN campaigns c ON c.id = e.campaign_id
		WHERE e.brand_id = $1 AND e.event_type = 'tracking_link_click'
		  AND e.created_at >= $2 AND e.created_at < $3
		ORDER BY e.created_at DESC
		LIMIT $4`, canonicalID, fromISO, toExclusive, limit)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	clicks, uniqueSessions, uniqueLinks := 0, 0, 0
	if summary != nil {
		clicks = summary.intOr("clicks", 0)
		uniqueSessions = summary.intOr("unique_sessions", 0)
		uniqueLinks = summary.intOr("unique_links", 0)
	}
	items := make([]map[string]any, 0, len(itemRows))
	for _, row := range itemRows {
		detail := bsEnrichClick(row.strOr("user_agent", ""), row.strOr("referrer", ""))
		items = append(items, map[string]any{
			"created_at":    row.timePtr("created_at"),
			"code":          bsStrOrNil(row.strOr("code", "")),
			"kind":          bsStrOrNil(row.strOr("kind", "")),
			"campaign_name": bsStrOrNil(row.strOr("campaign_name", "")),
			"url":           bsStrOrNil(row.strOr("url", "")),
			"session_id":    bsStrOrNil(row.strOr("session_id", "")),
			"referrer":      bsStrOrNil(row.strOr("referrer", "")),
			"device":        detail.Device, "browser": detail.Browser, "os": detail.OS,
			"referrer_group": detail.ReferrerGroup, "geo": detail.Geo,
		})
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"from": from, "to": to,
		"summary": map[string]any{
			"clicks": clicks, "unique_sessions": uniqueSessions, "unique_links": uniqueLinks,
		},
		"items": items,
	})
}

// bsStrOrNil: string kosong → nil (parity `r.x ? String(r.x) : null`).
func bsStrOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
