package brandflow

import (
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// audit.go: Audit Publikasi klien, dashboard kepatuhan artikel yang sudah terbit.
//
// Sumbernya sama dengan Link Guardian (publication_link_checks, link_guardian_claims),
// hanya dirangkum: ringkasan, tren bulanan, peringkat media, dan daftar artikel.
// Tidak ada pelacakan pembaca.

// AuditPublications: GET /v1/brands/{brandId}/audit?month=YYYY-MM
// month opsional: memfilter daftar dan ringkasan. Tren 12 bulan selalu tampil.
func (d *Deps) AuditPublications(w http.ResponseWriter, r *http.Request) {
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

	from, until := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	if m := r.URL.Query().Get("month"); m != "" {
		start, perr := time.Parse("2006-01", m)
		if perr != nil {
			httpx.Error(w, http.StatusUnprocessableEntity, "Format bulan harus YYYY-MM")
			return
		}
		from, until = start, start.AddDate(0, 1, 0)
	}

	// Daftar artikel (terfilter periode).
	rows, err := d.queryRows(r.Context(), `
		SELECT p.id, p.published_at, p.published_url,
		       d.title AS document_title, m.name AS media_name,
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
		WHERE p.brand_id = $1 AND p.status = 'published'
		  AND p.published_at >= $2 AND p.published_at < $3
		ORDER BY p.published_at DESC NULLS LAST
		LIMIT 500`, brandRow.ID, from, until)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	totals := map[string]int{
		"published": 0, "healthy": 0, "problem": 0,
		"gone": 0, "unreachable": 0, "unchecked": 0, "claims_open": 0,
	}
	for _, row := range rows {
		linkState := row.strOr("link_state", "")
		kind := db.LinkViolationKind(linkState, row.strOr("in_article_state", ""),
			row.strOr("rel_state", ""), row.strOr("noindex_state", ""))
		totals["published"]++
		switch {
		case linkState == "":
			totals["unchecked"]++
		case linkState == "gone":
			totals["gone"]++
		case linkState == "unreachable":
			totals["unreachable"]++
		}
		if kind != "" {
			totals["problem"]++
		} else if linkState == "live" {
			totals["healthy"]++
		}
		claim := row.strOr("claim_status", "")
		if claim == "submitted" || claim == "checking" || claim == "contacted" {
			totals["claims_open"]++
		}
		items = append(items, map[string]any{
			"id": row.strOr("id", ""), "document_title": row.strOr("document_title", ""),
			"media_name": row.strOr("media_name", ""), "published_at": row.timePtr("published_at"),
			"published_url": bsStrOrNil(row.strOr("published_url", "")),
			"link_state": bsStrOrNil(linkState), "problem_kind": bsStrOrNil(kind),
			"last_checked_at": row.timePtr("last_checked_at"), "warranty_until": row.timePtr("warranty_until"),
			"in_warranty": row.boolOr("in_warranty", false), "claim_status": bsStrOrNil(claim),
		})
	}

	// Tren 12 bulan terakhir (tidak ikut filter periode).
	now := time.Now()
	trendFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -11, 0)
	monthRows, err := d.queryRows(r.Context(), `
		SELECT to_char(date_trunc('month', p.published_at), 'YYYY-MM') AS m,
		       COUNT(p.id)::int AS total,
		       COUNT(p.id) FILTER (WHERE `+problemLinkSQL+`)::int AS problem,
		       COUNT(p.id) FILTER (WHERE NOT `+problemLinkSQL+` AND lc.state = 'live')::int AS healthy
		FROM publications p
		LEFT JOIN publication_link_checks lc ON lc.publication_id = p.id
		WHERE p.brand_id = $1 AND p.status = 'published' AND p.published_at >= $2
		GROUP BY 1 ORDER BY 1`, brandRow.ID, trendFrom)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	monthly := make([]map[string]any, 0, len(monthRows))
	for _, row := range monthRows {
		monthly = append(monthly, map[string]any{
			"month": row.strOr("m", ""), "total": row.intOr("total", 0),
			"healthy": row.intOr("healthy", 0), "problem": row.intOr("problem", 0),
		})
	}

	// Peringkat media (terfilter periode).
	mediaRows, err := d.queryRows(r.Context(), `
		SELECT m.name AS name,
		       COUNT(p.id)::int AS total,
		       COUNT(p.id) FILTER (WHERE `+problemLinkSQL+`)::int AS problem
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN publication_link_checks lc ON lc.publication_id = p.id
		WHERE p.brand_id = $1 AND p.status = 'published'
		  AND p.published_at >= $2 AND p.published_at < $3
		GROUP BY m.name ORDER BY COUNT(p.id) DESC, m.name
		LIMIT 8`, brandRow.ID, from, until)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	media := make([]map[string]any, 0, len(mediaRows))
	for _, row := range mediaRows {
		media = append(media, map[string]any{
			"name": row.strOr("name", ""), "total": row.intOr("total", 0), "problem": row.intOr("problem", 0),
		})
	}

	writeNoStore(w, http.StatusOK, map[string]any{
		"totals": totals, "monthly": monthly, "media": media, "items": items,
	})
}
