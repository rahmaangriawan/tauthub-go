package partner

import (
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- GET /v1/partner/link-monitor ----------

var linkMonitorStates = map[string]bool{"gone": true, "unreachable": true, "ok": true, "unknown": true}

func (d *Deps) LinkMonitor(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	view := strings.TrimSpace(q.Get("view"))
	if view == "" {
		view = "publications"
	}
	if view != "summary" && view != "publications" && view != "domains" {
		httpx.Error(w, http.StatusUnprocessableEntity, "view harus summary, publications, atau domains")
		return
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	switch view {
	case "summary":
		watched, err := d.queryRow(r.Context(), `
			SELECT COUNT(*)::int AS watched,
			       COUNT(*) FILTER (WHERE c.state = 'gone')::int AS gone,
			       COUNT(*) FILTER (WHERE c.state = 'unreachable')::int AS unreachable
			FROM publications p
			JOIN media m ON m.id = p.media_id
			LEFT JOIN publication_link_checks c ON c.publication_id = p.id
			WHERE m.partner_id = $1 AND p.status = 'published' AND p.published_url IS NOT NULL
			  AND p.published_at > NOW() - INTERVAL '12 months'
			  AND p.publish_status <> 'draft'`, partner.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		domains, err := d.queryRow(r.Context(), `
			SELECT COUNT(*) FILTER (WHERE domain_health = 'unreachable')::int AS domains_down,
			       COUNT(*) FILTER (WHERE domain_health IN ('unknown', 'token_missing'))::int AS domains_attention
			FROM media WHERE partner_id = $1`, partner.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"watched":           watched.intOr("watched", 0),
			"gone":              watched.intOr("gone", 0),
			"unreachable":       watched.intOr("unreachable", 0),
			"domains_down":      domains.intOr("domains_down", 0),
			"domains_attention": domains.intOr("domains_attention", 0),
		})
	case "domains":
		health := strings.TrimSpace(q.Get("health"))
		if health != "" && health != "unknown" && health != "live" && health != "unreachable" && health != "token_missing" {
			httpx.Error(w, http.StatusUnprocessableEntity, "health harus salah satu dari unknown, live, unreachable, token_missing")
			return
		}
		where := "m.domain_health IN ('unreachable', 'unknown', 'token_missing')"
		args := []any{partner.ID}
		if health != "" {
			where = "m.domain_health = $2"
			args = append(args, health)
		}
		total, err := d.queryRow(r.Context(), "SELECT COUNT(*)::int AS total FROM media m WHERE m.partner_id = $1 AND "+where, args...)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		pageArgs := append(append([]any{}, args...), limit, offset)
		rows, err := d.queryRows(r.Context(),
			`SELECT m.id AS media_id, m.name, m.website_url, m.domain_health, m.domain_last_checked_at, m.domain_check_fail_count, m.domain_verified
			 FROM media m WHERE m.partner_id = $1 AND `+where+`
			 ORDER BY m.domain_last_checked_at ASC NULLS FIRST, m.name ASC LIMIT $`+itoa(len(args)+1)+` OFFSET $`+itoa(len(args)+2),
			pageArgs...)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		domains := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			domains = append(domains, map[string]any{
				"media_id":                row.raw("media_id"),
				"name":                    row.raw("name"),
				"website_url":             row.raw("website_url"),
				"domain_health":           row.raw("domain_health"),
				"domain_last_checked_at":  row.timePtr("domain_last_checked_at"),
				"domain_check_fail_count": row.intOr("domain_check_fail_count", 0),
				"domain_verified":         row.boolOr("domain_verified", false),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"domains": domains, "total": total.intOr("total", 0), "limit": limit, "offset": offset,
		})
	default:
		state := strings.TrimSpace(q.Get("state"))
		mediaName := strings.TrimSpace(q.Get("media"))
		search := strings.TrimSpace(q.Get("q"))
		if len(search) > 120 {
			search = search[:120]
		}
		days := clampInt(q.Get("days"), 0, 0, 3650)

		filters := []string{
			"m.partner_id = $1", "p.status = 'published'", "p.published_url IS NOT NULL",
			"p.published_at > NOW() - INTERVAL '12 months'",
			// Artikel yang terbit sebagai DRAF di website mitra belum tampil publik,
			// jadi tidak ikut dipantau 404 (lihat PublicationLinkCheckJob).
			"p.publish_status <> 'draft'",
		}
		args := []any{partner.ID}
		next := func(v any) string {
			args = append(args, v)
			return "$" + itoa(len(args))
		}
		if state == "" || state == "problematic" {
			filters = append(filters, "c.state IN ('gone', 'unreachable')")
		} else if state != "all" {
			filters = append(filters, "c.state = "+next(state))
		}
		if mediaName != "" {
			filters = append(filters, "m.name = "+next(mediaName))
		}
		if days > 0 {
			// Interval diinterpolasi (nilai sudah dipaksa integer).
			filters = append(filters, "c.last_checked_at >= NOW() - INTERVAL '"+itoa(days)+" days'")
		}
		if search != "" {
			ph := next("%" + search + "%")
			filters = append(filters, "(d.title ILIKE "+ph+" OR m.name ILIKE "+ph+" OR p.published_url ILIKE "+ph+")")
		}
		where := strings.Join(filters, " AND ")

		total, err := d.queryRow(r.Context(), `
			SELECT COUNT(*)::int AS total
			FROM publications p
			JOIN media m ON m.id = p.media_id
			JOIN content_documents d ON d.id = p.document_id
			LEFT JOIN publication_link_checks c ON c.publication_id = p.id
			WHERE `+where, args...)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		pageArgs := append(append([]any{}, args...), limit, offset)
		rows, err := d.queryRows(r.Context(), `
			SELECT p.id, d.title AS document_title, m.name AS media_name,
			       m.category AS media_category, m.media_type,
			       p.published_url, p.published_at, p.featured_image_url,
			       (p.partner_draft_edits->>'featured_image_url') AS draft_featured_image_url,
			       c.state, c.http_status, c.consecutive_failures,
			       c.first_detected_gone_at, c.last_checked_at
			FROM publications p
			JOIN media m ON m.id = p.media_id
			JOIN content_documents d ON d.id = p.document_id
			LEFT JOIN publication_link_checks c ON c.publication_id = p.id
			WHERE `+where+`
			ORDER BY c.first_detected_gone_at DESC NULLS LAST, p.published_at DESC
			LIMIT $`+itoa(len(args)+1)+` OFFSET $`+itoa(len(args)+2), pageArgs...)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		publications := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			publications = append(publications, map[string]any{
				"id":                       row.raw("id"),
				"document_title":           row.raw("document_title"),
				"media_name":               row.raw("media_name"),
				"media_category":           row.raw("media_category"),
				"media_type":               row.raw("media_type"),
				"published_url":            row.raw("published_url"),
				"published_at":             row.timePtr("published_at"),
				"featured_image_url":       row.raw("featured_image_url"),
				"draft_featured_image_url": row.raw("draft_featured_image_url"),
				"state":                    row.raw("state"),
				"http_status":              row.intPtr("http_status"),
				"consecutive_failures":     row.intOr("consecutive_failures", 0),
				"first_detected_gone_at":   row.timePtr("first_detected_gone_at"),
				"last_checked_at":          row.timePtr("last_checked_at"),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"publications": publications, "total": total.intOr("total", 0), "limit": limit, "offset": offset,
		})
	}
}

// ---------- GET/POST /v1/partner/crawler ----------

func (d *Deps) Crawler(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		d.crawlerAction(w, r, partner)
		return
	}
	mediaID := strings.TrimSpace(r.URL.Query().Get("media_id"))
	if mediaID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "media_id wajib diisi")
		return
	}
	if _, err := d.scopedMedia(r, partner.ID, mediaID); err != nil {
		d.writeErr(w, err)
		return
	}
	credential, err := d.queryRow(r.Context(), `
		SELECT id, media_id, partner_id, domain, crawler_id, secret_fingerprint, status, allowed_hosts, allowed_paths,
		       created_at, rotated_at, revoked_at, last_successful_crawl, last_failed_crawl, fail_count
		FROM crawler_credentials WHERE partner_id = $1 AND media_id = $2 ORDER BY created_at DESC LIMIT 1`,
		partner.ID, mediaID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	diagnostics, err := d.queryRows(r.Context(), `
		SELECT d.* FROM crawler_diagnostics d
		JOIN crawler_credentials c ON c.id = d.credential_id OR (c.media_id = d.media_id AND c.partner_id = $1)
		WHERE d.media_id = $2 ORDER BY d.created_at DESC LIMIT 10`, partner.ID, mediaID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"credential":  crawlerCredentialJSON(credential),
		"diagnostics": diagnostics,
	})
}

func crawlerCredentialJSON(row rowMap) any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"id":                    row.raw("id"),
		"media_id":              row.raw("media_id"),
		"partner_id":            row.raw("partner_id"),
		"domain":                row.raw("domain"),
		"crawler_id":            row.raw("crawler_id"),
		"secret_fingerprint":    row.raw("secret_fingerprint"),
		"status":                row.raw("status"),
		"allowed_hosts":         row.raw("allowed_hosts"),
		"allowed_paths":         row.raw("allowed_paths"),
		"created_at":            row.timePtr("created_at"),
		"rotated_at":            row.timePtr("rotated_at"),
		"revoked_at":            row.timePtr("revoked_at"),
		"last_successful_crawl": row.timePtr("last_successful_crawl"),
		"last_failed_crawl":     row.timePtr("last_failed_crawl"),
		"fail_count":            row.intOr("fail_count", 0),
	}
}

func (d *Deps) scopedMedia(r *http.Request, partnerID, mediaID string) (rowMap, error) {
	row, err := d.queryRow(r.Context(),
		"SELECT id, name, website_url, status FROM media WHERE id = $1 AND partner_id = $2 LIMIT 1", mediaID, partnerID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Media tidak ditemukan")
	}
	return row, nil
}
