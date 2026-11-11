package brandflow

import (
	"net/http"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// performance.go: Analisis Kinerja klien, ringkasan hasil per kampanye.
//
// Hanya fakta yang tercatat di Tauthub: artikel terbit, media yang memuatnya,
// kondisi tautan dari Link Guardian (artikel bisa dibuka, tautan di dalamnya
// masih ada, tanpa noindex, atribut sesuai), masa garansi, dan klaim garansi.
// Tidak ada pelacakan klik pembaca: Tauthub tidak memantau perilaku pengunjung.

// problemLinkSQL: tautan bermasalah menurut definisi bersama db.LinkViolationKind.
const problemLinkSQL = `(lc.state = 'gone' OR lc.in_article_state = 'removed' OR lc.noindex_state = 'noindex' OR lc.rel_state = 'changed')`

// PerformanceOverview: GET /v1/brands/{brandId}/performance
func (d *Deps) PerformanceOverview(w http.ResponseWriter, r *http.Request) {
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
	rows, err := d.queryRows(r.Context(), `
		SELECT c.id, c.name, c.status, c.created_at,
		       COUNT(p.id) FILTER (WHERE p.status = 'published')::int AS published,
		       COUNT(p.id) FILTER (WHERE p.status NOT IN ('published', 'cancelled', 'failed'))::int AS in_progress,
		       COUNT(DISTINCT p.media_id) FILTER (WHERE p.status = 'published')::int AS media,
		       COUNT(p.id) FILTER (WHERE p.status = 'published' AND lc.state = 'live' AND NOT `+problemLinkSQL+`)::int AS healthy,
		       COUNT(p.id) FILTER (WHERE p.status = 'published' AND `+problemLinkSQL+`)::int AS problem,
		       COUNT(g.id) FILTER (WHERE g.status IN ('submitted', 'checking', 'contacted'))::int AS claims_open,
		       COUNT(g.id) FILTER (WHERE g.status IN ('refunded', 'replaced'))::int AS claims_used,
		       MAX(p.published_at) AS last_published_at
		FROM campaigns c
		LEFT JOIN publications p ON p.campaign_id = c.id AND p.status <> 'cancelled'
		LEFT JOIN publication_link_checks lc ON lc.publication_id = p.id
		LEFT JOIN link_guardian_claims g ON g.publication_id = p.id
		WHERE c.brand_id = $1 AND c.deleted_at IS NULL
		GROUP BY c.id, c.name, c.status, c.created_at
		ORDER BY MAX(p.published_at) DESC NULLS LAST, c.created_at DESC
		LIMIT 100`, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	totals := map[string]int{"published": 0, "media": 0, "problem": 0, "claims_open": 0}
	for _, row := range rows {
		item := map[string]any{
			"id": row.strOr("id", ""), "name": row.strOr("name", ""), "status": row.strOr("status", ""),
			"created_at": row.timePtr("created_at"), "last_published_at": row.timePtr("last_published_at"),
			"published": row.intOr("published", 0), "in_progress": row.intOr("in_progress", 0),
			"media": row.intOr("media", 0), "healthy": row.intOr("healthy", 0), "problem": row.intOr("problem", 0),
			"claims_open": row.intOr("claims_open", 0), "claims_used": row.intOr("claims_used", 0),
		}
		items = append(items, item)
		totals["published"] += row.intOr("published", 0)
		totals["media"] += row.intOr("media", 0)
		totals["problem"] += row.intOr("problem", 0)
		totals["claims_open"] += row.intOr("claims_open", 0)
	}
	writeNoStore(w, http.StatusOK, map[string]any{"totals": totals, "campaigns": items})
}

// CampaignPerformance: GET /v1/brands/{brandId}/campaigns/{campaignId}/performance
func (d *Deps) CampaignPerformance(w http.ResponseWriter, r *http.Request) {
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
	rows, err := d.queryRows(r.Context(), `
		SELECT p.id, p.status, p.published_at, p.published_url,
		       d.title AS document_title, m.name AS media_name, m.website_url,
		       lc.state AS link_state, lc.in_article_state, lc.rel_state, lc.noindex_state, lc.last_checked_at,
		       CASE WHEN COALESCE(m.link_duration_months, 12) = 0 THEN NULL
		            ELSE p.published_at + make_interval(months => COALESCE(m.link_duration_months, 12)) END AS warranty_until,
		       `+db.LinkWarrantySQL("p", "m")+` AS in_warranty,
		       g.status AS claim_status
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		LEFT JOIN publication_link_checks lc ON lc.publication_id = p.id
		LEFT JOIN link_guardian_claims g ON g.publication_id = p.id
		WHERE p.campaign_id = $1 AND p.brand_id = $2 AND p.status <> 'cancelled'
		ORDER BY p.published_at DESC NULLS LAST, p.created_at DESC
		LIMIT 200`, campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	published, healthy, problem := 0, 0, 0
	mediaSet := map[string]bool{}
	for _, row := range rows {
		kind := db.LinkViolationKind(row.strOr("link_state", ""), row.strOr("in_article_state", ""),
			row.strOr("rel_state", ""), row.strOr("noindex_state", ""))
		if row.strOr("status", "") == "published" {
			published++
			mediaSet[row.strOr("media_name", "")] = true
			if kind != "" {
				problem++
			} else if row.strOr("link_state", "") == "live" {
				healthy++
			}
		}
		items = append(items, map[string]any{
			"id": row.strOr("id", ""), "status": row.strOr("status", ""),
			"document_title": row.strOr("document_title", ""), "media_name": row.strOr("media_name", ""),
			"published_at": row.timePtr("published_at"), "published_url": bsStrOrNil(row.strOr("published_url", "")),
			"link_state": bsStrOrNil(row.strOr("link_state", "")), "problem_kind": bsStrOrNil(kind),
			"last_checked_at": row.timePtr("last_checked_at"), "warranty_until": row.timePtr("warranty_until"),
			"in_warranty": row.boolOr("in_warranty", false), "claim_status": bsStrOrNil(row.strOr("claim_status", "")),
		})
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"campaign": map[string]any{"id": campaignID, "name": campaign.strOr("name", "")},
		"summary":  map[string]any{"published": published, "media": len(mediaSet), "healthy": healthy, "problem": problem},
		"publications": items,
	})
}
