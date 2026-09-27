package platform

// domain_diagnostics.go: diagnostik verifikasi domain admin —
// GET /v1/platform/domain-diagnostics (+ ?view=summary) dan
// POST /v1/platform/domain-diagnostics/recheck.
// Port app/v1/platform/domain-diagnostics/route.ts + recheck/route.ts.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// mpDomainResults: sebab kegagalan verifikasi domain yang bisa difilter admin.
var mpDomainResults = []string{"verified", "token_not_found", "token_mismatch", "unreachable"}

// ---------- GET /v1/platform/domain-diagnostics ----------

func (d *Deps) DomainDiagnosticsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	view := strings.TrimSpace(q.Get("view"))
	if view == "" {
		view = "list"
	}
	limit := mpClampInt(q.Get("limit"), 50, 1, 500)
	offset := mpClampInt(q.Get("offset"), 0, 0, 1<<30)

	if view == "summary" {
		row, err := d.queryRow(r.Context(), `
			SELECT COUNT(*) FILTER (WHERE domain_last_result = 'unreachable')::int AS unreachable,
			       COUNT(*) FILTER (WHERE domain_last_result = 'token_not_found')::int AS token_not_found,
			       COUNT(*) FILTER (WHERE domain_last_result = 'token_mismatch')::int AS token_mismatch,
			       COUNT(*) FILTER (WHERE domain_last_result = 'verified')::int AS verified,
			       COUNT(*) FILTER (WHERE domain_last_result IS NULL)::int AS never_checked
			FROM media
			WHERE website_url IS NOT NULL AND TRIM(website_url) <> ''`)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		out := map[string]int{}
		if row != nil {
			pr := ptRow(ptNormalizeMap(row))
			out["unreachable"] = pr.intOr("unreachable", 0)
			out["token_not_found"] = pr.intOr("token_not_found", 0)
			out["token_mismatch"] = pr.intOr("token_mismatch", 0)
			out["verified"] = pr.intOr("verified", 0)
			out["never_checked"] = pr.intOr("never_checked", 0)
		} else {
			for _, key := range []string{"unreachable", "token_not_found", "token_mismatch", "verified", "never_checked"} {
				out[key] = 0
			}
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	result := strings.TrimSpace(q.Get("result"))
	if result != "" && !mpContains(mpDomainResults, result) {
		writeErr(w, d, errUnprocessable("result harus salah satu dari verified, token_not_found, token_mismatch, unreachable"))
		return
	}
	search := strings.TrimSpace(q.Get("q"))
	if len(search) > 120 {
		search = search[:120]
	}
	partnerID := strings.TrimSpace(q.Get("partner_id"))

	filters := []string{"m.website_url IS NOT NULL", "TRIM(m.website_url) <> ''"}
	values := []any{}
	next := func(v any) string {
		values = append(values, v)
		return "$" + ptItoa(len(values))
	}
	if result != "" {
		filters = append(filters, "m.domain_last_result = "+next(result))
	} else {
		// Tanpa filter, sembunyikan yang sehat: halaman ini untuk mencari masalah.
		filters = append(filters, "m.domain_last_result IS DISTINCT FROM 'verified'")
	}
	if partnerID != "" {
		filters = append(filters, "m.partner_id = "+next(partnerID))
	}
	if search != "" {
		ph := next("%" + search + "%")
		filters = append(filters, "(m.name ILIKE "+ph+" OR m.website_url ILIKE "+ph+" OR prt.name ILIKE "+ph+")")
	}
	where := strings.Join(filters, " AND ")

	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*)::int AS total FROM media m LEFT JOIN partners prt ON prt.id = m.partner_id WHERE "+where, values...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	pageArgs := append(append([]any{}, values...), limit, offset)
	items, err := d.queryRows(r.Context(), `
		SELECT m.id AS media_id, m.name, m.website_url, m.partner_id, prt.name AS partner_name,
		       m.domain_verified, m.domain_health, m.domain_check_fail_count,
		       m.domain_last_checked_at, m.domain_last_result, m.domain_last_error, m.domain_last_error_at,
		       m.domain_last_stage, m.domain_last_trace
		FROM media m
		LEFT JOIN partners prt ON prt.id = m.partner_id
		WHERE `+where+`
		ORDER BY m.domain_last_error_at DESC NULLS LAST, m.name ASC
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
			"partner_id":              row.anyOrNil("partner_id"),
			"partner_name":            row.anyOrNil("partner_name"),
			"domain_verified":         row.boolOr("domain_verified", false),
			"domain_health":           row.anyOrNil("domain_health"),
			"domain_check_fail_count": row.intOr("domain_check_fail_count", 0),
			"domain_last_checked_at":  row.anyOrNil("domain_last_checked_at"),
			"domain_last_result":      row.anyOrNil("domain_last_result"),
			"domain_last_error":       row.anyOrNil("domain_last_error"),
			"domain_last_error_at":    row.anyOrNil("domain_last_error_at"),
			"domain_last_stage":       row.anyOrNil("domain_last_stage"),
			"domain_last_trace":       row.jsonField("domain_last_trace"),
		})
	}
	total := 0
	if countRow != nil {
		total = ptRow(ptNormalizeMap(countRow)).intOr("total", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": total, "limit": limit, "offset": offset})
}

// ---------- POST /v1/platform/domain-diagnostics/recheck ----------

func (d *Deps) DomainDiagnosticsRecheck(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	mediaID, _ := strField(body, "media_id")
	if mediaID == "" {
		writeErr(w, d, errUnprocessable("media_id wajib diisi"))
		return
	}
	if err := d.mpCheckRecheckRate(r, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}

	media, err := d.queryRow(r.Context(), "SELECT id, website_url FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if media == nil {
		writeErr(w, d, errNotFound("Media tidak ditemukan"))
		return
	}
	websiteURL := strings.TrimSpace(ptRow(ptNormalizeMap(media)).strOr("website_url", ""))
	if websiteURL == "" {
		writeErr(w, d, errUnprocessable("Media ini belum punya Website/Domain"))
		return
	}
	writeJSON(w, http.StatusOK, d.mpVerifyDomainOwnership(r, mediaID))
}

// mpCheckRecheckRate: rate limit 20/menit khusus aksi recheck
// (parity checkWriteRate(bucket, 20) — bucket per user).
func (d *Deps) mpCheckRecheckRate(r *http.Request, userID string) error {
	if err := auth.CheckWriteRate(r.Context(), d.Pool, "domain-recheck:"+userID, 20); err != nil {
		return ptErrTooMany("Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi.")
	}
	return nil
}

// mpVerifyDomainOwnership: port verifyDomainOwnership admin — ambil halaman
// media dan cari token, lalu catat seluruh tahap di kolom domain_*.
func (d *Deps) mpVerifyDomainOwnership(r *http.Request, mediaID string) map[string]any {
	ctx := r.Context()
	row, err := d.queryRow(ctx, `
		SELECT id, name, website_url, domain_verify_token, domain_verified, domain_check_fail_count, domain_health
		FROM media WHERE id = $1 LIMIT 1`, mediaID)
	if err != nil || row == nil {
		return map[string]any{"ok": false, "status": "error", "stage": "load", "trace": []string{"Media tidak ditemukan"}}
	}
	media := ptRow(ptNormalizeMap(row))
	website := strings.TrimSpace(media.strOr("website_url", ""))
	if website == "" {
		return d.mpRecordDomainCheckFailure(ctx, mediaID, media,
			"token_not_found", "Website/domain media belum diisi", "no_website", []string{"Website/domain media kosong"})
	}
	token := media.strOr("domain_verify_token", "")
	target := strings.Replace(mpEnsureScheme(website), "http://", "https://", 1)

	trace := []string{"GET " + target}
	resp, reqErr := security.Get(ctx, target, map[string]string{
		"user-agent":      mpBrowserUA,
		"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"accept-language": "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7",
	}, 30*time.Second)
	if reqErr != nil {
		stage := "fetch"
		code := mpErrorCode(reqErr)
		if code == "ENOTFOUND" || code == "EAI_AGAIN" {
			stage = "dns"
		}
		return d.mpRecordDomainCheckFailure(ctx, mediaID, media,
			"unreachable", ptTruncate(reqErr.Error(), 300), stage, append(trace, "error: "+code+": "+ptTruncate(reqErr.Error(), 200)))
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 500_000))
	resp.Body.Close()
	status := resp.StatusCode
	trace = append(trace, "HTTP "+ptItoa(status))
	if status < 200 || status >= 300 {
		return d.mpRecordDomainCheckFailure(ctx, mediaID, media,
			"unreachable", "HTTP "+ptItoa(status)+" "+strings.TrimSpace(resp.Status), "http", trace)
	}
	lowerBody := strings.ToLower(string(body))
	if token != "" && strings.Contains(lowerBody, strings.ToLower(token)) {
		if err := d.exec(ctx, `
			UPDATE media SET domain_verified = TRUE, domain_verified_at = NOW(), domain_last_checked_at = NOW(),
			  domain_check_fail_count = 0, domain_health = 'live', domain_last_result = 'verified',
			  domain_last_error = NULL, domain_last_stage = 'ok', domain_last_trace = $1, domain_last_error_at = NULL, updated_at = NOW()
			WHERE id = $2`, mpJSONString(trace), mediaID); err != nil {
			return map[string]any{"ok": false, "status": "error", "stage": "update", "trace": trace}
		}
		if err := d.exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), NULL, 'media.domain_verified', 'media', $1)",
			mediaID); err != nil {
			d.logWarn("audit domain_verified gagal", mediaID, err)
		}
		// Domain rujukan yang menunggu approve ditandai approved (best-effort).
		_ = d.exec(ctx,
			"UPDATE referred_domains SET status = 'approved', rejection_reason = NULL, updated_at = NOW() WHERE media_id = $1 AND status = 'pending'", mediaID)
		return map[string]any{
			"ok": true, "status": "verified", "checked_at": d.isoNow(),
			"fail_count": 0, "stage": "ok", "trace": trace,
		}
	}
	if token != "" {
		if idx := strings.Index(lowerBody, "domain-verify"); idx >= 0 {
			payload := string(body)
			start := idx
			end := ptMinInt(idx+80, len(payload))
			result := d.mpRecordDomainCheckFailure(ctx, mediaID, media,
				"token_missing", "token_mismatch", "token", append(trace, "token ditemukan tapi tidak cocok"))
			result["found_token"] = ptTruncate(payload[start:end], 80)
			return result
		}
	}
	return d.mpRecordDomainCheckFailure(ctx, mediaID, media,
		"token_not_found", "Token verifikasi tidak ditemukan di halaman.", "token", trace)
}

func ptMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// mpRecordDomainCheckFailure: naikkan fail_count; setelah 3 kegagalan berturut
// pada media yang sebelumnya terverifikasi, status verifikasi dicabut.
func (d *Deps) mpRecordDomainCheckFailure(ctx context.Context, mediaID string, before ptRow, result, message, stage string, trace []string) map[string]any {
	failCount := before.intOr("domain_check_fail_count", 0) + 1
	wasVerified := before.boolOr("domain_verified", false)
	revoked := wasVerified && failCount >= 3
	health := before.strOr("domain_health", "unknown")
	if result == "unreachable" {
		health = "unreachable"
	}
	if revoked {
		_ = d.exec(ctx, `
			UPDATE media SET domain_verified = FALSE, domain_last_checked_at = NOW(), domain_check_fail_count = $1,
			  domain_health = $2, domain_last_result = $3, domain_last_error = $4, domain_last_stage = $5,
			  domain_last_trace = $6, domain_last_error_at = NOW(), updated_at = NOW()
			WHERE id = $7`,
			failCount, health, result, message, stage, mpJSONString(trace), mediaID)
		if err := d.exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), NULL, 'media.domain_revoked', 'media', $1)",
			mediaID); err != nil {
			d.logWarn("audit domain_revoked gagal", mediaID, err)
		}
	} else {
		_ = d.exec(ctx, `
			UPDATE media SET domain_last_checked_at = NOW(), domain_check_fail_count = $1,
			  domain_health = $2, domain_last_result = $3, domain_last_error = $4, domain_last_stage = $5,
			  domain_last_trace = $6, domain_last_error_at = NOW(), updated_at = NOW()
			WHERE id = $7`,
			failCount, health, result, message, stage, mpJSONString(trace), mediaID)
	}
	payload := map[string]any{
		"ok":         false,
		"status":     result,
		"checked_at": d.isoNow(),
		"fail_count": failCount,
		"stage":      stage,
		"trace":      trace,
		"reason":     message,
	}
	if revoked {
		payload["revoked"] = true
	}
	return payload
}

// mpErrorCode: kode errno jaringan dari error (parity cause.code di Node).
func mpErrorCode(err error) string {
	msg := err.Error()
	for _, code := range []string{"ENOTFOUND", "EAI_AGAIN", "ECONNREFUSED", "ECONNRESET", "ETIMEDOUT", "EHOSTUNREACH", "ENETUNREACH"} {
		if strings.Contains(msg, code) {
			return code
		}
	}
	return ""
}
