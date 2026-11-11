package platform

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Kode promo top-up (admin dengan izin "topup"). Bonus dicatat per pembayaran;
// kode tidak bisa dihapus agar riwayat pemakaian tetap utuh, hanya dinonaktifkan.

// PromoCodesList: GET /v1/platform/promo-codes
func (d *Deps) PromoCodesList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, "topup"); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT code, bonus_tc, min_topup_tc, max_uses, used_count, valid_until, active, note, created_at
		FROM promo_codes ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"codes": rows})
}

// PromoCodesCreate: POST /v1/platform/promo-codes
func (d *Deps) PromoCodesCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "topup")
	if !ok {
		return
	}
	body := decodeJSON(r)
	code := strings.ToUpper(strings.TrimSpace(promoStr(body, "code")))
	if len(code) < 4 || len(code) > 40 || strings.ContainsAny(code, " \t\n/?&#") {
		writeErr(w, d, errUnprocessable("Kode 4 sampai 40 karakter tanpa spasi atau simbol URL."))
		return
	}
	bonus, okB := finNumInt(body["bonus_tc"])
	if !okB || bonus < 1 || bonus > 1000000 {
		writeErr(w, d, errUnprocessable("Bonus harus 1 sampai 1.000.000 TC."))
		return
	}
	minTC := 1
	if v, has := body["min_topup_tc"]; has && v != nil {
		n, okM := finNumInt(v)
		if !okM || n < 1 {
			writeErr(w, d, errUnprocessable("Minimal top up harus 1 TC atau lebih."))
			return
		}
		minTC = n
	}
	var maxUses any
	if v, has := body["max_uses"]; has && v != nil {
		n, okU := finNumInt(v)
		if !okU || n < 1 {
			writeErr(w, d, errUnprocessable("Batas pakai harus 1 atau lebih, atau kosong."))
			return
		}
		maxUses = n
	}
	var validUntil any
	if s := promoStr(body, "valid_until"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t, err = time.Parse("2006-01-02", s)
			if err == nil {
				t = t.Add(24*time.Hour - time.Second)
			}
		}
		if err != nil || !t.After(time.Now()) {
			writeErr(w, d, errUnprocessable("Masa berlaku harus tanggal di masa depan."))
			return
		}
		validUntil = t
	}
	err := d.exec(r.Context(), `
		INSERT INTO promo_codes (code, bonus_tc, min_topup_tc, max_uses, valid_until, note, created_by)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)`,
		code, bonus, minTC, maxUses, validUntil, strings.TrimSpace(promoStr(body, "note")), user.ID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			writeErr(w, d, errUnprocessable("Kode itu sudah ada."))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "platform.promo.created", "promo_code", code)
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "code": code})
}

// PromoCodesSetActive: PUT /v1/platform/promo-codes/{code} dengan {"active":bool}
func (d *Deps) PromoCodesSetActive(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "topup")
	if !ok {
		return
	}
	body := decodeJSON(r)
	active, isBool := body["active"].(bool)
	if !isBool {
		writeErr(w, d, errUnprocessable("Isi active dengan true atau false."))
		return
	}
	code := strings.ToUpper(chi.URLParam(r, "code"))
	if err := d.exec(r.Context(), "UPDATE promo_codes SET active = $2 WHERE code = $1", code, active); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "platform.promo.updated", "promo_code", code)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active": active})
}

func promoStr(body map[string]any, key string) string {
	s, _ := body[key].(string)
	return s
}
