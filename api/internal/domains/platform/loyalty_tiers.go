package platform

import (
	"net/http"
	"strings"
)

// Admin CRUD untuk loyalty_tiers (volume discount). Tabel config kecil, di-keyed
// by name. Dipakai tab "Loyalty" di halaman admin Revenue Shares.

// LoyaltyTiersList: GET /v1/platform/loyalty-tiers
func (d *Deps) LoyaltyTiersList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	rows, err := d.queryRows(r.Context(),
		"SELECT name, min_spend_tc, discount_percent, sort_order FROM loyalty_tiers ORDER BY min_spend_tc ASC")
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal memuat loyalty tiers"))
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"name":             row.strOr("name", ""),
			"min_spend_tc":     row.intOr("min_spend_tc", 0),
			"discount_percent": row.numOr("discount_percent"),
			"sort_order":       row.intOr("sort_order", 0),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"loyalty_tiers": out})
}

// LoyaltyTierUpsert: PUT /v1/platform/loyalty-tiers — tambah/ubah satu tier
// (ON CONFLICT name). name adalah identitasnya.
func (d *Deps) LoyaltyTierUpsert(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	body := decodeJSON(r)
	name, _ := strField(body, "name")
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 40 {
		writeErr(w, d, errUnprocessable("Nama tier wajib diisi (maksimal 40 karakter)"))
		return
	}
	minSpend, hasMin := numField(body, "min_spend_tc")
	if !hasMin || minSpend < 0 {
		writeErr(w, d, errUnprocessable("min_spend_tc tidak valid"))
		return
	}
	disc, hasDisc := numField(body, "discount_percent")
	if !hasDisc || disc < 0 || disc > 100 {
		writeErr(w, d, errUnprocessable("discount_percent harus 0–100"))
		return
	}
	sortOrder, _ := numField(body, "sort_order")
	if err := d.exec(r.Context(), `
		INSERT INTO loyalty_tiers (name, min_spend_tc, discount_percent, sort_order)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE SET
		  min_spend_tc = EXCLUDED.min_spend_tc,
		  discount_percent = EXCLUDED.discount_percent,
		  sort_order = EXCLUDED.sort_order`,
		name, int64(minSpend), disc, int(sortOrder)); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal menyimpan tier"))
		return
	}
	d.audit(r.Context(), user.ID, "finance.loyalty_tier.saved", "loyalty_tier", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// LoyaltyTierDelete: DELETE /v1/platform/loyalty-tiers/{tierName}
func (d *Deps) LoyaltyTierDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	name := chiParam(r, "tierName")
	if strings.TrimSpace(name) == "" {
		writeErr(w, d, errUnprocessable("Nama tier wajib diisi"))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM loyalty_tiers WHERE name = $1", name); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal menghapus tier"))
		return
	}
	d.audit(r.Context(), user.ID, "finance.loyalty_tier.deleted", "loyalty_tier", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
