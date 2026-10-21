package platform

// link_monitor.go: Pelacak 404 admin — GET /v1/platform/link-monitor
// (view=summary|publications|domains), POST /v1/platform/link-monitor/check,
// dan POST /v1/platform/link-monitor/replace.
// Port app/v1/platform/link-monitor/**.

import (
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
)

// mpLinkStates: state tautan yang bisa difilter halaman Pelacak 404.
var mpLinkStates = []string{"live", "gone", "unreachable", "no_url"}

// mpDomainHealths: kesehatan domain mitra (media.domain_health, migrasi 148).
var mpDomainHealths = []string{"unknown", "live", "unreachable", "token_missing"}

// mpLinkMonitorSweepCap: batas jumlah baris per kategori saat menyapu semua
// baris eligible (parity SWEEP_CAP).
const mpLinkMonitorSweepCap = 200

// ---------- GET /v1/platform/link-monitor ----------

func (d *Deps) LinkMonitorGlobal(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	view := strings.TrimSpace(q.Get("view"))
	if view == "" {
		view = "publications"
	}
	limit := mpClampInt(q.Get("limit"), 50, 1, 500)
	offset := mpClampInt(q.Get("offset"), 0, 0, 1<<30)

	switch view {
	case "summary":
		pubs, err := d.queryRow(r.Context(), `
			SELECT COUNT(*)::int AS watched,
			       COUNT(*) FILTER (WHERE c.state = 'live')::int AS live,
			       COUNT(*) FILTER (WHERE c.state = 'gone')::int AS gone,
			       COUNT(*) FILTER (WHERE c.state = 'unreachable')::int AS unreachable,
			       COUNT(*) FILTER (WHERE c.state = 'no_url')::int AS no_url
			FROM publications p
			LEFT JOIN publication_link_checks c ON c.publication_id = p.id
			JOIN media m ON m.id = p.media_id
			WHERE p.status = 'published'
			  AND p.published_url IS NOT NULL
			  AND `+db.LinkWarrantySQL("p", "m")+`
			  AND p.publish_status <> 'draft'`)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		domains, err := d.queryRow(r.Context(), `
			SELECT COUNT(*) FILTER (WHERE domain_health = 'unreachable')::int AS domains_down,
			       COUNT(*) FILTER (WHERE domain_health = 'unknown')::int AS domains_unknown
			FROM media WHERE partner_id IS NOT NULL`)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		prow := ptRow(ptNormalizeMap(pubs))
		drow := ptRow(ptNormalizeMap(domains))
		writeJSON(w, http.StatusOK, map[string]any{
			"watched":         prow.intOr("watched", 0),
			"live":            prow.intOr("live", 0),
			"gone":            prow.intOr("gone", 0),
			"unreachable":     prow.intOr("unreachable", 0),
			"no_url":          prow.intOr("no_url", 0),
			"domains_down":    drow.intOr("domains_down", 0),
			"domains_unknown": drow.intOr("domains_unknown", 0),
		})
		return

	case "domains":
		health := strings.TrimSpace(q.Get("health"))
		if health != "" && !mpContains(mpDomainHealths, health) {
			writeErr(w, d, errUnprocessable("health harus salah satu dari unknown, live, unreachable, token_missing"))
			return
		}
		// Default: hanya yang bermasalah atau belum pernah dicek.
		where := "m.domain_health IN ('unreachable', 'unknown')"
		values := []any{}
		if health != "" {
			values = append(values, health)
			where = "m.domain_health = $1"
		}
		countRow, err := d.queryRow(r.Context(),
			"SELECT COUNT(*)::int AS total FROM media m WHERE m.partner_id IS NOT NULL AND "+where, values...)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		pageArgs := append(append([]any{}, values...), limit, offset)
		items, err := d.queryRows(r.Context(), `
			SELECT m.id AS media_id, m.name, m.website_url, prt.name AS partner_name,
			       m.domain_health, m.domain_last_checked_at, m.domain_check_fail_count, m.domain_verified
			FROM media m
			LEFT JOIN partners prt ON prt.id = m.partner_id
			WHERE m.partner_id IS NOT NULL AND `+where+`
			ORDER BY m.domain_last_checked_at ASC NULLS FIRST, m.name ASC
			LIMIT $`+ptItoa(len(values)+1)+` OFFSET $`+ptItoa(len(values)+2), pageArgs...)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, raw := range items {
			row := ptRow(ptNormalizeMap(raw))
			out = append(out, map[string]any{
				"media_id":                row.anyOrNil("media_id"),
				"name":                    row.anyOrNil("name"),
				"website_url":             row.anyOrNil("website_url"),
				"partner_name":            row.anyOrNil("partner_name"),
				"domain_health":           row.anyOrNil("domain_health"),
				"domain_last_checked_at":  row.anyOrNil("domain_last_checked_at"),
				"domain_check_fail_count": row.intOr("domain_check_fail_count", 0),
				"domain_verified":         row.boolOr("domain_verified", false),
			})
		}
		total := 0
		if countRow != nil {
			total = ptRow(ptNormalizeMap(countRow)).intOr("total", 0)
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": total, "limit": limit, "offset": offset})
		return
	}

	// view=publications
	state := strings.TrimSpace(q.Get("state"))
	if state != "" && !mpContains(mpLinkStates, state) {
		writeErr(w, d, errUnprocessable("state harus salah satu dari live, gone, unreachable, no_url"))
		return
	}
	partnerID := strings.TrimSpace(q.Get("partner_id"))
	mediaID := strings.TrimSpace(q.Get("media_id"))
	search := strings.TrimSpace(q.Get("q"))
	if len(search) > 120 {
		search = search[:120]
	}

	filters := []string{
		"p.status = 'published'",
		"p.published_url IS NOT NULL",
		// Masa pemantauan mengikuti kebijakan media (0 = tautan permanen).
		db.LinkWarrantySQL("p", "m"),
		// Publikasi berstatus DRAF di website mitra belum tampil publik → tidak
		// dipantau 404 (sama dengan aturan di scheduler & PublicationLinkCheckJob).
		"p.publish_status <> 'draft'",
	}
	values := []any{}
	next := func(v any) string {
		values = append(values, v)
		return "$" + ptItoa(len(values))
	}
	if state != "" {
		filters = append(filters, "c.state = "+next(state))
	} else {
		// Tanpa filter state, tampilkan HANYA yang bermasalah.
		filters = append(filters, "c.state IN ('gone', 'unreachable')")
	}
	if partnerID != "" {
		filters = append(filters, "m.partner_id = "+next(partnerID))
	}
	if mediaID != "" {
		filters = append(filters, "m.id = "+next(mediaID))
	}
	if search != "" {
		ph := next("%" + search + "%")
		filters = append(filters, "(d.title ILIKE "+ph+" OR m.name ILIKE "+ph+" OR prt.name ILIKE "+ph+" OR p.published_url ILIKE "+ph+")")
	}
	where := strings.Join(filters, " AND ")

	countRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*)::int AS total
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN brands b ON b.id = p.brand_id
		LEFT JOIN partners prt ON prt.id = m.partner_id
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		WHERE `+where, values...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	pageArgs := append(append([]any{}, values...), limit, offset)
	items, err := d.queryRows(r.Context(), `
		SELECT p.id, d.title AS document_title, b.name AS brand_name, m.name AS media_name,
		       prt.name AS partner_name, p.published_url, p.published_at,
		       c.state, c.http_status, c.consecutive_failures,
		       c.first_detected_gone_at, c.last_checked_at, c.error_message
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN brands b ON b.id = p.brand_id
		LEFT JOIN partners prt ON prt.id = m.partner_id
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		WHERE `+where+`
		ORDER BY c.first_detected_gone_at DESC NULLS LAST, p.published_at DESC
		LIMIT $`+ptItoa(len(values)+1)+` OFFSET $`+ptItoa(len(values)+2), pageArgs...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		out = append(out, map[string]any{
			"id":                     row.anyOrNil("id"),
			"document_title":         row.anyOrNil("document_title"),
			"brand_name":             row.anyOrNil("brand_name"),
			"media_name":             row.anyOrNil("media_name"),
			"partner_name":           row.anyOrNil("partner_name"),
			"published_url":          row.anyOrNil("published_url"),
			"published_at":           row.anyOrNil("published_at"),
			"state":                  row.anyOrNil("state"),
			"http_status":            row.numPtr("http_status"),
			"consecutive_failures":   row.intOr("consecutive_failures", 0),
			"first_detected_gone_at": row.anyOrNil("first_detected_gone_at"),
			"last_checked_at":        row.anyOrNil("last_checked_at"),
			"error_message":          row.anyOrNil("error_message"),
		})
	}
	total := 0
	if countRow != nil {
		total = ptRow(ptNormalizeMap(countRow)).intOr("total", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": total, "limit": limit, "offset": offset})
}

// ---------- POST /v1/platform/link-monitor/check ----------

func (d *Deps) LinkMonitorCheck(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	publicationIDs, hasPubs := mpUUIDListField(body, "publicationIds")
	mediaIDs, hasMedia := mpUUIDListField(body, "mediaIds")
	if (!hasPubs && !hasMedia) || (hasPubs && publicationIDs == nil) || (hasMedia && mediaIDs == nil) {
		// Tanpa keduanya = [] (body kosong bukan error); tipe/format salah = 422.
		if !hasPubs && !hasMedia {
			publicationIDs, mediaIDs = []string{}, []string{}
		} else {
			writeErr(w, d, errUnprocessable("publicationIds dan mediaIds harus berupa daftar UUID"))
			return
		}
	}

	capped := false
	if len(publicationIDs) == 0 && len(mediaIDs) == 0 {
		// Eligibility identik dengan scheduler; ambil CAP+1 untuk tahu apakah
		// masih ada sisa (laporkan capped dengan jujur).
		pubRows, err := d.queryRows(r.Context(), `
			SELECT p.id FROM publications p
			LEFT JOIN publication_link_checks c ON c.publication_id = p.id
			JOIN media m ON m.id = p.media_id
			WHERE p.status = 'published'
			  AND p.published_url IS NOT NULL
			  AND `+db.LinkWarrantySQL("p", "m")+`
			  AND p.publish_status <> 'draft'
			  AND (c.last_checked_at IS NULL OR c.last_checked_at < NOW() - INTERVAL '7 days')
			  AND NOT EXISTS (
			    SELECT 1 FROM jobs j
			     WHERE j.kind = 'publication_link_check' AND j.status IN ('pending', 'running')
			       AND j.payload->>'publicationId' = p.id
			  )
			ORDER BY c.last_checked_at NULLS FIRST
			LIMIT $1`, mpLinkMonitorSweepCap+1)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		mediaRows, err := d.queryRows(r.Context(), `
			SELECT id FROM media
			WHERE partner_id IS NOT NULL
			  AND website_url IS NOT NULL AND TRIM(website_url) <> ''
			  AND (domain_last_checked_at IS NULL OR domain_last_checked_at < NOW() - INTERVAL '7 days')
			ORDER BY domain_last_checked_at NULLS FIRST
			LIMIT $1`, mpLinkMonitorSweepCap+1)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		if len(pubRows) > mpLinkMonitorSweepCap || len(mediaRows) > mpLinkMonitorSweepCap {
			capped = true
		}
		for _, raw := range mpHeadRows(pubRows, mpLinkMonitorSweepCap) {
			publicationIDs = append(publicationIDs, ptRow(ptNormalizeMap(raw)).strOr("id", ""))
		}
		for _, raw := range mpHeadRows(mediaRows, mpLinkMonitorSweepCap) {
			mediaIDs = append(mediaIDs, ptRow(ptNormalizeMap(raw)).strOr("id", ""))
		}
	}

	queuedPublications := 0
	for _, publicationID := range publicationIDs {
		if _, err := d.mpEnqueueJob(r.Context(), "publication_link_check", map[string]any{"publicationId": publicationID}, 0); err == nil {
			queuedPublications++
		} else {
			d.logWarn("enqueue link check gagal", publicationID, err)
		}
	}
	queuedDomains := 0
	for _, mediaID := range mediaIDs {
		if _, err := d.mpEnqueueJob(r.Context(), "domain_verify", map[string]any{"mediaId": mediaID}, 0); err == nil {
			queuedDomains++
		} else {
			d.logWarn("enqueue domain verify gagal", mediaID, err)
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":                  true,
		"queued_publications": queuedPublications,
		"queued_domains":      queuedDomains,
		"total":               queuedPublications + queuedDomains,
		"capped":              capped,
	})
}

func mpHeadRows(rows []rowMap, n int) []rowMap {
	if len(rows) <= n {
		return rows
	}
	return rows[:n]
}

// mpUUIDListField: array string dari body; nil (has=true) berarti tipe salah.
func mpUUIDListField(body map[string]any, key string) ([]string, bool) {
	v, has := body[key]
	if !has {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, true
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, isStr := item.(string)
		if !isStr {
			return nil, true
		}
		out = append(out, s)
	}
	return out, true
}

// ---------- POST /v1/platform/link-monitor/replace ----------

// mpLinkSkipMessages: alasan skip dari lib (English) → pesan admin (Indonesia).
var mpLinkSkipMessages = map[string]string{
	"publication not found or not published":     "Publikasi tidak ditemukan atau statusnya bukan published.",
	"link is not marked as gone":                 "Link artikel ini tidak berstatus hilang (404/410), jadi tidak perlu diganti.",
	"auto-replace disabled for this publication": "Penggantian otomatis dimatikan untuk publikasi ini.",
	"auto-replace disabled by configuration":     "Fitur penggantian otomatis sedang dimatikan di konfigurasi.",
	"auto-replace limit reached":                 "Batas penggantian beruntun untuk publikasi ini sudah tercapai.",
	"no alternative media":                       "Tidak ada media alternatif yang tersedia untuk menggantikan.",
	"no candidate passed the editorial check":    "Tidak ada kandidat media yang lolos pemeriksaan standar editorial.",
}

func (d *Deps) LinkMonitorReplace(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	publicationID, hasID := strField(body, "publicationId")
	if !hasID || publicationID == "" || !mpLooksLikeUUID(publicationID) {
		writeErr(w, d, errUnprocessable("publicationId wajib berupa UUID"))
		return
	}

	result := d.mpReplacePublicationAfterLinkLoss(r.Context(), publicationID)
	writeAuditBestEffort(d, r.Context(), user.ID, "publication.link_lost.replace", "publication", publicationID)

	replaced, _ := result["replaced"].(bool)
	if !replaced {
		reason, _ := result["skipped_reason"].(string)
		if reason == "" {
			reason = "Penggantian tidak dapat dijalankan."
		}
		message, known := mpLinkSkipMessages[reason]
		if !known {
			message = reason
		}
		status := http.StatusConflict
		if strings.Contains(reason, "not found") {
			status = http.StatusNotFound
		}
		writeErr(w, d, ptErr(status, message))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                 true,
		"publication_id":     publicationID,
		"replaced":           true,
		"new_publication_id": result["publication_id"],
		"media_id":           result["media_id"],
		"media_name":         result["media_name"],
	})
}

// mpLooksLikeUUID: validasi bentuk UUID (parity z.string().uuid()).
func mpLooksLikeUUID(v string) bool {
	parts := strings.Split(v, "-")
	if len(parts) != 5 {
		return false
	}
	lengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != lengths[i] {
			return false
		}
		for _, ch := range part {
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
				return false
			}
		}
	}
	return true
}
