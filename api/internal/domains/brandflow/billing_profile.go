package brandflow

import (
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// billing_profile.go: profil penagihan brand (nama perusahaan, NPWP, alamat,
// email tagihan). Dipakai top-up berfaktur dan invoice otomatis.
//
// NPWP sensitif: dibaca hanya oleh writer ke atas, diubah hanya oleh admin, dan
// tidak pernah ditulis ke log. Validasi menolak dengan pesan yang jelas; tidak
// ada data yang dibuang diam-diam.

var nonDigitRe = regexp.MustCompile(`\D`)
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// bsNormalizeNPWP: buang titik, tanda hubung, dan spasi; harus 15 atau 16 digit.
// Mengembalikan angka murni dan pesan galat ("" bila valid).
func bsNormalizeNPWP(raw string) (string, string) {
	digits := nonDigitRe.ReplaceAllString(raw, "")
	if len(digits) != 15 && len(digits) != 16 {
		return "", "NPWP harus 15 atau 16 digit (titik dan tanda hubung boleh dipakai)."
	}
	return digits, ""
}

// bsValidateBilling: validasi satu paket data penagihan. Mengembalikan galat
// berbahasa Indonesia atau "" bila valid.
func bsValidateBilling(company, npwp, address string) (string, string, string, string) {
	company = strings.TrimSpace(company)
	address = strings.TrimSpace(address)
	if n := utf8.RuneCountInString(company); n < 2 || n > 200 {
		return "", "", "", "Nama perusahaan wajib diisi (2 sampai 200 karakter)."
	}
	if n := utf8.RuneCountInString(address); n < 5 || n > 500 {
		return "", "", "", "Alamat sesuai NPWP wajib diisi (5 sampai 500 karakter)."
	}
	digits, msg := bsNormalizeNPWP(npwp)
	if msg != "" {
		return "", "", "", msg
	}
	return company, digits, address, ""
}

func (d *Deps) loadBillingProfile(r *http.Request, brandID string) (bsRow, error) {
	return d.queryRow(r.Context(),
		"SELECT company_name, npwp, npwp_address, billing_email, updated_at FROM billing_profiles WHERE brand_id = $1 LIMIT 1", brandID)
}

// BillingProfileGet: GET /v1/brands/{brandId}/billing-profile
func (d *Deps) BillingProfileGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "finance")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.loadBillingProfile(r, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		writeNoStore(w, http.StatusOK, map[string]any{"profile": nil})
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"profile": map[string]any{
		"company_name": row.strOr("company_name", ""), "npwp": row.strOr("npwp", ""),
		"npwp_address": row.strOr("npwp_address", ""), "billing_email": row.strOr("billing_email", ""),
		"updated_at": row.timePtr("updated_at"),
	}})
}

// BillingProfilePut: PUT /v1/brands/{brandId}/billing-profile
func (d *Deps) BillingProfilePut(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	str := func(key string) string {
		s, _ := body[key].(string)
		return s
	}
	company, npwp, address, msg := bsValidateBilling(str("company_name"), str("npwp"), str("npwp_address"))
	if msg != "" {
		httpx.Error(w, http.StatusUnprocessableEntity, msg)
		return
	}
	email := strings.TrimSpace(str("billing_email"))
	if email != "" && (len(email) > 254 || !emailRe.MatchString(email)) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Email tagihan tidak valid.")
		return
	}
	var emailArg any
	if email != "" {
		emailArg = email
	}
	if err := d.exec(r.Context(), `
		INSERT INTO billing_profiles (brand_id, company_name, npwp, npwp_address, billing_email, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (brand_id) DO UPDATE SET
		  company_name = EXCLUDED.company_name, npwp = EXCLUDED.npwp, npwp_address = EXCLUDED.npwp_address,
		  billing_email = EXCLUDED.billing_email, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		brandRow.ID, company, npwp, address, emailArg, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	// Audit mencatat perubahan, bukan isinya (NPWP tidak masuk log).
	d.audit(r.Context(), brandRow.ID, user.ID, "brand.billing_profile_updated", "brand", brandRow.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
