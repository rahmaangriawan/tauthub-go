package clientread

import (
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// CampaignReportGet: GET /v1/campaign-reports/{token} — data berita acara
// kampanye publik (tanpa auth), sumber yang sama dengan halaman
// /berita-acara/{token}. Port app/v1/campaign-reports/[token]/route.ts +
// lib/campaign/report.ts.
//
// Endpoint publik ini sengaja tidak memerlukan login: tautannya dibagikan ke
// pihak ketiga. Token yang dicabut (revoked_at) → 404.
func (d *Deps) CampaignReportGet(w http.ResponseWriter, r *http.Request) {
	token := chiParam(r, "token")
	if token == "" {
		httpx.Error(w, http.StatusNotFound, "Berita acara tidak ditemukan atau tautannya sudah dicabut.")
		return
	}

	share, err := d.queryRow(r.Context(), `
		SELECT s.id AS share_id, c.id, c.name, c.description, c.status, c.starts_at, c.ends_at, c.created_at,
		       b.name AS brand_name
		FROM campaign_report_shares s
		JOIN campaigns c ON c.id = s.campaign_id
		JOIN brands b ON b.id = s.brand_id
		WHERE s.token = $1 AND s.revoked_at IS NULL
		LIMIT 1`, token)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Gagal memuat berita acara")
		return
	}
	if share == nil {
		httpx.Error(w, http.StatusNotFound, "Berita acara tidak ditemukan atau tautannya sudah dicabut.")
		return
	}

	campaignID := share.strOr("id", "")

	// Agregat ringkasan — parity CAMPAIGN_AGGREGATE_SELECT di lib/campaign/report.ts.
	summary, err := d.queryRow(r.Context(), `
		SELECT
		  (SELECT COUNT(*) FROM campaign_media_items cmi
		     WHERE cmi.campaign_id = c.id
		       AND COALESCE(cmi.variant_document_id, cmi.source_document_id) IS NOT NULL
		       AND cmi.status <> 'cancelled') AS articles_created,
		  (SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status = 'published') AS published_count,
		  (SELECT COUNT(DISTINCT p.media_id) FROM publications p WHERE p.campaign_id = c.id AND p.status = 'published') AS media_published_count,
		  (SELECT STRING_AGG(DISTINCT m.name, ' · ') FROM publications p JOIN media m ON m.id = p.media_id
		     WHERE p.campaign_id = c.id AND p.status = 'published') AS published_media_names,
		  (SELECT COALESCE(SUM(t.used_tc), 0) FROM (
		      SELECT COALESCE(SUM(CASE WHEN l.ref_type = 'capture' THEN l.amount_tc ELSE 0 END), 0)
		             + GREATEST(-COALESCE(SUM(l.amount_tc), 0), 0) AS used_tc
		      FROM credit_ledger_entries l
		      WHERE l.brand_id = c.brand_id AND l.ref_type IN ('reserve', 'release', 'capture')
		        AND EXISTS (SELECT 1 FROM publications p WHERE p.id = l.ref_id AND p.campaign_id = c.id)
		      GROUP BY l.ref_id
		   ) t) AS tc_used
		FROM campaigns c WHERE c.id = $1 LIMIT 1`, campaignID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Gagal memuat berita acara")
		return
	}

	publications, err := d.queryRows(r.Context(), `
		SELECT p.id, p.status, p.published_at, p.published_url,
		       d.title AS document_title, m.name AS media_name, m.category AS media_category
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		WHERE p.campaign_id = $1 AND p.status <> 'cancelled'
		ORDER BY p.published_at DESC NULLS LAST, p.created_at DESC
		LIMIT 200`, campaignID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Gagal memuat berita acara")
		return
	}

	pubItems := make([]map[string]any, 0, len(publications))
	for _, p := range publications {
		pubItems = append(pubItems, map[string]any{
			"id":             p.raw("id"),
			"status":         p.raw("status"),
			"document_title": p.raw("document_title"),
			"media_name":     p.raw("media_name"),
			"media_category": p.raw("media_category"),
			"published_at":   p.timePtr("published_at"),
			"published_url":  p.raw("published_url"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"campaign": map[string]any{
			"id":          campaignID,
			"name":        share.strOr("name", ""),
			"description": share.raw("description"),
			"status":      share.raw("status"),
			"starts_at":   share.timePtr("starts_at"),
			"ends_at":     share.timePtr("ends_at"),
			"created_at":  share.timePtr("created_at"),
		},
		"brand_name": share.strOr("brand_name", ""),
		"summary": map[string]any{
			"articles_created":      summary.intOr("articles_created", 0),
			"published_count":       summary.intOr("published_count", 0),
			"media_published_count": summary.intOr("media_published_count", 0),
			"published_media_names": summary.raw("published_media_names"),
			"tc_used":               summary.numOr("tc_used"),
		},
		"publications": pubItems,
		"generated_at": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}
