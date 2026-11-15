package clientread

import (
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
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
	// Publik tanpa auth — dibatasi longgar per IP supaya tautan sah tetap
	// nyaman dipakai, tapi brute force token tertahan.
	ip := auth.RequestIp(r.Header)
	if ip == "" {
		ip = "unknown"
	}
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "campaign-report:"+ip, 120); err != nil {
		w.Header().Set("Retry-After", "60")
		httpx.Error(w, http.StatusTooManyRequests, "Terlalu banyak permintaan. Coba lagi sebentar lagi.")
		return
	}

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
		  (SELECT COUNT(*) FROM publications p JOIN publication_link_checks lc ON lc.publication_id = p.id
		     WHERE p.campaign_id = c.id AND p.status = 'published' AND lc.state = 'live') AS link_live_count,
		  (SELECT ROUND(AVG(sm.da)::numeric, 1) FROM media_seo_metrics sm
		     WHERE sm.da IS NOT NULL AND sm.media_id IN (
		       SELECT p.media_id FROM publications p WHERE p.campaign_id = c.id AND p.status = 'published')) AS avg_da,
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
		       d.title AS document_title, m.name AS media_name, m.category AS media_category,
		       p.featured_image_url,
		       lc.state AS link_state, lc.last_checked_at, lc.rel_value, lc.rel_state, lc.noindex_state,
		       lc.in_article_state,
		       (SELECT COUNT(*) FROM publication_link_snapshots sn WHERE sn.publication_id = p.id)::int AS snapshot_count,
		       (SELECT sn.checked_at FROM publication_link_snapshots sn
		         WHERE sn.publication_id = p.id AND sn.kind = 'publish' LIMIT 1) AS baseline_at,
		       m.link_rel, m.link_duration_months,
		       CASE WHEN COALESCE(m.link_duration_months, 12) = 0 THEN NULL
		            ELSE p.published_at + make_interval(months => COALESCE(m.link_duration_months, 12)) END AS warranty_until,
		       `+db.LinkWarrantySQL("p", "m")+` AS in_warranty,
		       g.status AS claim_status
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		LEFT JOIN publication_link_checks lc ON lc.publication_id = p.id
		LEFT JOIN link_guardian_claims g ON g.publication_id = p.id
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
			"id":                p.raw("id"),
			"status":            p.raw("status"),
			"document_title":    p.raw("document_title"),
			"media_name":        p.raw("media_name"),
			"media_category":    p.raw("media_category"),
			"featured_image_url": p.raw("featured_image_url"),
			"published_at":      p.timePtr("published_at"),
			"published_url":     p.raw("published_url"),
			// Bukti tayang: kondisi tautan menurut pemantauan Tauthub.
			"link_state":         p.raw("link_state"),
			"in_article_state":   p.raw("in_article_state"),
			"last_checked_at":    p.timePtr("last_checked_at"),
			"baseline_at":        p.timePtr("baseline_at"),
			"snapshot_count":     p.intOr("snapshot_count", 0),
			"rel_value":          p.raw("rel_value"),
			"rel_state":          p.raw("rel_state"),
			"noindex_state":      p.raw("noindex_state"),
			"link_rel":           p.raw("link_rel"),
			"link_duration_months": p.raw("link_duration_months"),
			"warranty_until":     p.timePtr("warranty_until"),
			"in_warranty":        p.raw("in_warranty") == true,
			"claim_status":       p.raw("claim_status"),
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
			// Hasil agregat untuk tautan publik: tanpa rincian biaya.
			"link_live_count": summary.intOr("link_live_count", 0),
			"avg_da":          summary.raw("avg_da"),
			"published_media_names": summary.raw("published_media_names"),
			"tc_used":               summary.numOr("tc_used"),
		},
		"publications": pubItems,
		"generated_at": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}
