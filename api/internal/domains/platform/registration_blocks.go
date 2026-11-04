package platform

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
)

// Daftar email yang diblokir dari pendaftaran (OTP tidak dijawab 48 jam, lihat
// brandflow.schSweepUnansweredEmailCodes). Admin membukanya dari halaman user.

// RegistrationBlocksList: GET /v1/platform/operations/registration-blocks
func (d *Deps) RegistrationBlocksList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT email, reason, blocked_at FROM registration_email_blocks
		WHERE unblocked_at IS NULL ORDER BY blocked_at DESC LIMIT 200`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"email": row.strOr("email", ""), "reason": row.strOr("reason", ""), "blocked_at": row.timePtr("blocked_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// RegistrationBlockRemove: DELETE /v1/platform/operations/registration-blocks
// body {email}. Membuka blokir (unblocked_at), bukan menghapus barisnya,
// supaya riwayatnya tetap ada.
func (d *Deps) RegistrationBlockRemove(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	email, _ := strField(body, "email")
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		writeErr(w, d, errUnprocessable("email is required"))
		return
	}
	row, err := d.queryRow(r.Context(), `
		UPDATE registration_email_blocks SET unblocked_at = NOW(), unblocked_by = $2
		WHERE email = $1 AND unblocked_at IS NULL RETURNING email`, email, admin.ID)
	if err != nil || row == nil {
		writeErr(w, d, errNotFound("Email tidak ada di daftar blokir."))
		return
	}
	writeAuditBestEffort(d, r.Context(), admin.ID, "registration_block.removed", "registration", email)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- domain email yang diblokir (admin) + batas per domain ----------

var blockedDomainRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*\.[a-z]{2,}$`)

func normalizeBlockedDomain(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimPrefix(v, "@")
	v = strings.TrimPrefix(v, "www.")
	return v
}

// BlockedDomainsList: GET /v1/platform/operations/blocked-domains
func (d *Deps) BlockedDomainsList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(),
		"SELECT domain, reason, created_at FROM blocked_email_domains ORDER BY created_at DESC LIMIT 500")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"domain": row.strOr("domain", ""), "reason": row.strOr("reason", ""), "created_at": row.timePtr("created_at"),
		})
	}
	cfg := auth.LoadGuardConfig(r.Context(), d.Pool)
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "builtin_count": auth.BuiltinDisposableCount(), "domain_per_day": cfg.RateLimits.DomainPerDay,
	})
}

// BuiltinDisposableDomains: GET /v1/platform/operations/blocked-domains/builtin
// ?q=&limit=&offset= — daftar temp-mail bawaan (hanya baca), bisa dicari.
func (d *Deps) BuiltinDisposableDomains(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	q := r.URL.Query()
	limit := mpClampInt(q.Get("limit"), 100, 1, 500)
	offset := mpClampInt(q.Get("offset"), 0, 0, 1<<30)
	items, total := auth.BuiltinDisposableList(q.Get("q"), limit, offset)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

// BlockedDomainAdd: POST /v1/platform/operations/blocked-domains {domain, reason?}
func (d *Deps) BlockedDomainAdd(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	domainRaw, _ := strField(body, "domain")
	domain := normalizeBlockedDomain(domainRaw)
	if !blockedDomainRe.MatchString(domain) || len(domain) > 253 {
		writeErr(w, d, errUnprocessable("Domain tidak valid. Contoh: tempmail.io"))
		return
	}
	if domain == "gmail.com" || domain == "googlemail.com" || strings.HasSuffix(domain, ".tauthub.com") || domain == "tauthub.com" {
		writeErr(w, d, errUnprocessable("Domain ini tidak boleh diblokir."))
		return
	}
	reason, _ := strField(body, "reason")
	reason = strings.ToLower(strings.TrimSpace(reason))
	if reason != "disposable" && reason != "abuse" {
		reason = "manual"
	}
	if err := d.exec(r.Context(), `
		INSERT INTO blocked_email_domains (domain, reason, created_by) VALUES ($1, $2, $3)
		ON CONFLICT (domain) DO UPDATE SET reason = EXCLUDED.reason`, domain, reason, admin.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	auth.InvalidateBlockedDomainsCache()
	writeAuditBestEffort(d, r.Context(), admin.ID, "blocked_domain.added", "registration", domain)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "domain": domain, "reason": reason})
}

// BlockedDomainRemove: DELETE /v1/platform/operations/blocked-domains {domain}
func (d *Deps) BlockedDomainRemove(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	domainRaw, _ := strField(body, "domain")
	domain := normalizeBlockedDomain(domainRaw)
	if domain == "" {
		writeErr(w, d, errUnprocessable("domain is required"))
		return
	}
	row, err := d.queryRow(r.Context(), "DELETE FROM blocked_email_domains WHERE domain = $1 RETURNING domain", domain)
	if err != nil || row == nil {
		writeErr(w, d, errNotFound("Domain tidak ada di daftar admin (daftar bawaan tidak bisa dihapus)."))
		return
	}
	auth.InvalidateBlockedDomainsCache()
	writeAuditBestEffort(d, r.Context(), admin.ID, "blocked_domain.removed", "registration", domain)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// RegistrationGuardLimits: GET/PUT /v1/platform/operations/registration-guard
// — saat ini hanya rate_limits.domain_per_day (1–50).
func (d *Deps) RegistrationGuardLimits(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if r.Method == http.MethodPut {
		body := decodeJSON(r)
		n, has := body["domain_per_day"].(float64)
		if !has || n < 1 || n > 50 || n != float64(int(n)) {
			writeErr(w, d, errUnprocessable("domain_per_day harus bilangan bulat 1–50"))
			return
		}
		if err := d.exec(r.Context(), `
			UPDATE registration_guard_config
			SET rate_limits = COALESCE(rate_limits, '{}'::jsonb) || jsonb_build_object('domain_per_day', $1::int), updated_at = NOW()
			WHERE id = 1`, int(n)); err != nil {
			writeErr(w, d, err)
			return
		}
		auth.InvalidateGuardConfigCache()
		writeAuditBestEffort(d, r.Context(), admin.ID, "registration_guard.updated", "registration", "domain_per_day="+ptItoa(int(n)))
	}
	cfg := auth.LoadGuardConfig(r.Context(), d.Pool)
	writeJSON(w, http.StatusOK, map[string]any{"domain_per_day": cfg.RateLimits.DomainPerDay})
}
