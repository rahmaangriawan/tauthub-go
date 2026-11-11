package brandflow

import (
	"net/http"
	"sort"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/impact"
)

// impact.go: laporan dampak kampanye untuk klien (halaman Analisis Kinerja).
// Hitungan klik memakai paket impact, sama dengan berita acara publik. Lihat
// catatan batasnya di paket itu: bot disaring, satu pengunjung sekali per hari,
// tidak ada pemecahan per media atau negara.

const impactDays = 30

// CampaignImpact: GET /v1/brands/{brandId}/campaigns/{campaignId}/impact
func (d *Deps) CampaignImpact(w http.ResponseWriter, r *http.Request) {
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
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, chiParam(r, "campaignId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := campaign.strOr("id", "")
	now := time.Now()
	events, truncated, err := impact.Load(r.Context(), d.Pool, brandRow.ID, campaignID, now.AddDate(0, 0, -impactDays))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	stats := impact.Aggregate(events, impactDays, now)
	stats.Truncated = truncated

	pubs, _ := d.queryRow(r.Context(), `
		SELECT COUNT(*) FILTER (WHERE p.status = 'published')::int AS published,
		       COUNT(*) FILTER (WHERE p.status = 'published' AND lc.state = 'live'
		                          AND COALESCE(lc.in_article_state, '') <> 'removed'
		                          AND COALESCE(lc.noindex_state, '') <> 'noindex'
		                          AND COALESCE(lc.rel_state, '') <> 'changed')::int AS live,
		       COUNT(*) FILTER (WHERE p.status = 'published'
		                          AND (lc.state = 'gone' OR lc.in_article_state = 'removed'
		                               OR lc.noindex_state = 'noindex' OR lc.rel_state = 'changed'))::int AS problem,
		       MAX(lc.last_checked_at) AS last_checked
		FROM publications p
		LEFT JOIN publication_link_checks lc ON lc.publication_id = p.id
		WHERE p.campaign_id = $1`, campaignID)
	published, live, problem := 0, 0, 0
	var lastChecked any
	if pubs != nil {
		published, live, problem = pubs.intOr("published", 0), pubs.intOr("live", 0), pubs.intOr("problem", 0)
		lastChecked = pubs.timePtr("last_checked")
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"campaign": map[string]any{"id": campaignID, "name": campaign.strOr("name", "")},
		"days":     impactDays,
		"stats":    stats,
		"publications": map[string]any{
			"published": published, "live": live, "problem": problem, "last_checked_at": lastChecked,
		},
	})
}

// ImpactOverview: GET /v1/brands/{brandId}/impact
// Ringkasan semua kampanye brand: klik 30 hari, artikel terbit, status tautan.
func (d *Deps) ImpactOverview(w http.ResponseWriter, r *http.Request) {
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
	now := time.Now()
	events, truncated, err := impact.Load(r.Context(), d.Pool, brandRow.ID, "", now.AddDate(0, 0, -impactDays))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	byCampaign := map[string][]impact.Event{}
	for _, e := range events {
		if e.CampaignID != "" {
			byCampaign[e.CampaignID] = append(byCampaign[e.CampaignID], e)
		}
	}

	rows, err := d.queryRows(r.Context(), `
		SELECT c.id, c.name, c.status, c.created_at,
		       COUNT(p.id) FILTER (WHERE p.status = 'published')::int AS published,
		       COUNT(p.id) FILTER (WHERE p.status = 'published'
		                             AND (lc.state = 'gone' OR lc.in_article_state = 'removed'
		                                  OR lc.noindex_state = 'noindex' OR lc.rel_state = 'changed'))::int AS problem
		FROM campaigns c
		LEFT JOIN publications p ON p.campaign_id = c.id
		LEFT JOIN publication_link_checks lc ON lc.publication_id = p.id
		WHERE c.brand_id = $1 AND c.deleted_at IS NULL
		GROUP BY c.id, c.name, c.status, c.created_at
		ORDER BY c.created_at DESC
		LIMIT 100`, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	totalClicks := 0
	for _, row := range rows {
		id := row.strOr("id", "")
		st := impact.Aggregate(byCampaign[id], impactDays, now)
		totalClicks += st.Counted
		items = append(items, map[string]any{
			"id": id, "name": row.strOr("name", ""), "status": row.strOr("status", ""),
			"created_at": row.timePtr("created_at"),
			"published":  row.intOr("published", 0), "problem": row.intOr("problem", 0),
			"clicks": st.Counted, "visitors": st.Visitors, "daily": st.Daily,
		})
	}
	// Kampanye dengan klik terbanyak dulu; sisanya mengikuti urutan terbaru.
	sort.SliceStable(items, func(i, j int) bool { return items[i]["clicks"].(int) > items[j]["clicks"].(int) })
	writeNoStore(w, http.StatusOK, map[string]any{
		"days": impactDays, "total_clicks": totalClicks, "truncated": truncated, "campaigns": items,
	})
}
