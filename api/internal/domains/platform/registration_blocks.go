package platform

import (
	"net/http"
	"strings"
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
