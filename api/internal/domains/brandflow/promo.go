package brandflow

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Kode promo top-up: bonus TC tetap yang dicatat sebagai baris ledger terpisah saat
// pembayaran berhasil. Satu brand hanya sekali per kode. Validasi di sini hanya
// memberi pesan lebih awal; klaim sebenarnya terjadi atomik saat pembayaran sukses.

func normalizePromoCode(raw string) string {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if len(code) > 40 {
		code = code[:40]
	}
	return code
}

// checkPromo mengembalikan bonus TC bila kode berlaku untuk brand dan nominal ini,
// atau pesan galat untuk klien.
func (d *Deps) checkPromo(ctx context.Context, brandID, code string, tc int) (int, string) {
	row, err := d.queryRow(ctx, `
		SELECT bonus_tc, min_topup_tc, max_uses, used_count, active,
		       (valid_until IS NOT NULL AND valid_until <= NOW()) AS expired,
		       EXISTS (SELECT 1 FROM promo_redemptions r WHERE r.code = p.code AND r.brand_id = $2) AS used_by_brand
		FROM promo_codes p WHERE p.code = $1`, code, brandID)
	if err != nil {
		return 0, "Kode promo belum bisa diperiksa. Coba lagi."
	}
	if row == nil || !row.boolOr("active", false) {
		return 0, "Kode promo tidak ditemukan."
	}
	if row.boolOr("expired", false) {
		return 0, "Kode promo sudah kedaluwarsa."
	}
	if row.boolOr("used_by_brand", false) {
		return 0, "Brand ini sudah memakai kode promo tersebut."
	}
	if row.raw("max_uses") != nil && row.intOr("used_count", 0) >= row.intOr("max_uses", 0) {
		return 0, "Kuota kode promo sudah habis."
	}
	if tc < row.intOr("min_topup_tc", 1) {
		return 0, "Kode promo ini berlaku untuk top up minimal " + strconv.Itoa(row.intOr("min_topup_tc", 1)) + " TC."
	}
	return row.intOr("bonus_tc", 0), ""
}

// CreditsPromoCheck: GET /v1/brands/{brandId}/credits/promo?code=&tc=
func (d *Deps) CreditsPromoCheck(w http.ResponseWriter, r *http.Request) {
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
	code := normalizePromoCode(r.URL.Query().Get("code"))
	tc, _ := strconv.Atoi(r.URL.Query().Get("tc"))
	if code == "" || tc <= 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi kode promo dan nominal top up.")
		return
	}
	bonus, msg := d.checkPromo(r.Context(), brandRow.ID, code, tc)
	if msg != "" {
		httpx.Error(w, http.StatusUnprocessableEntity, msg)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"code": code, "bonus_tc": bonus})
}
