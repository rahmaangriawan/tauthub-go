package brandflow

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Loyalty tier (volume discount). Total belanja seumur hidup brand (SUM capture
// TC) membuka tier dengan diskon markup. Diskon memotong markup Tauthub; harga
// partner tidak berubah. Dipakai hanya di checkout + saat charge (lihat
// effectiveMarkup), bukan di katalog.

type loyaltyTier struct {
	Name            string  `json:"name"`
	MinSpendTC      int64   `json:"min_spend_tc"`
	DiscountPercent float64 `json:"discount_percent"`
	SortOrder       int     `json:"sort_order"`
}

var (
	loyaltyMu     sync.Mutex
	loyaltyCached *struct {
		value   []loyaltyTier
		expires time.Time
	}
)

// loyaltyTiers: daftar tier (urut min_spend_tc naik), cache 30s. Tanpa baris
// konfigurasi, satu tier default 0% supaya logika di hilir tetap aman.
func (d *Deps) loyaltyTiers(ctx context.Context) []loyaltyTier {
	now := time.Now()
	loyaltyMu.Lock()
	if loyaltyCached != nil && loyaltyCached.expires.After(now) {
		v := loyaltyCached.value
		loyaltyMu.Unlock()
		return v
	}
	loyaltyMu.Unlock()

	tiers := []loyaltyTier{}
	rows, err := d.Pool.Query(ctx,
		"SELECT name, min_spend_tc, discount_percent, sort_order FROM loyalty_tiers ORDER BY min_spend_tc ASC")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t loyaltyTier
			if rows.Scan(&t.Name, &t.MinSpendTC, &t.DiscountPercent, &t.SortOrder) == nil {
				tiers = append(tiers, t)
			}
		}
	}
	if len(tiers) == 0 {
		tiers = []loyaltyTier{{Name: "Reguler", MinSpendTC: 0, DiscountPercent: 0, SortOrder: 1}}
	}
	sort.SliceStable(tiers, func(i, j int) bool { return tiers[i].MinSpendTC < tiers[j].MinSpendTC })

	loyaltyMu.Lock()
	loyaltyCached = &struct {
		value   []loyaltyTier
		expires time.Time
	}{value: tiers, expires: now.Add(30 * time.Second)}
	loyaltyMu.Unlock()
	return tiers
}

// brandLifetimeSpendTC: total TC yang benar-benar dibelanjakan brand (capture).
func (d *Deps) brandLifetimeSpendTC(ctx context.Context, brandID string) int64 {
	var spent int64
	_ = d.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_tc), 0)
		FROM credit_ledger_entries
		WHERE brand_id = $1 AND ref_type = 'capture' AND amount_tc > 0`, brandID).Scan(&spent)
	if spent < 0 {
		spent = 0
	}
	return spent
}

type loyaltyStatus struct {
	TierName     string       `json:"tier_name"`
	DiscountPct  float64      `json:"discount_percent"`
	LifetimeTC   int64        `json:"lifetime_spend_tc"`
	NextTier     *nextTierVTO `json:"next_tier"`
}

type nextTierVTO struct {
	Name        string `json:"name"`
	MinSpendTC  int64  `json:"min_spend_tc"`
	RemainingTC int64  `json:"remaining_tc"`
}

// brandLoyalty: tier aktif brand + info tier berikutnya (untuk dorongan belanja).
func (d *Deps) brandLoyalty(ctx context.Context, brandID string) loyaltyStatus {
	tiers := d.loyaltyTiers(ctx)
	spent := d.brandLifetimeSpendTC(ctx, brandID)

	cur := tiers[0]
	var next *nextTierVTO
	for i, t := range tiers {
		if spent >= t.MinSpendTC {
			cur = t
			if i+1 < len(tiers) {
				nt := tiers[i+1]
				next = &nextTierVTO{Name: nt.Name, MinSpendTC: nt.MinSpendTC, RemainingTC: nt.MinSpendTC - spent}
			} else {
				next = nil
			}
		}
	}
	return loyaltyStatus{
		TierName:    cur.Name,
		DiscountPct: cur.DiscountPercent,
		LifetimeTC:  spent,
		NextTier:    next,
	}
}

// effectiveMarkup: markup efektif setelah diskon loyalty brand. Diskon = d% off
// dari harga klien (TC) — client_price = partner × (1 + markup/100) × (1 − d/100).
// Partner price tetap utuh; selisihnya memotong margin (markup) Tauthub. Dipakai
// HANYA di jalur charge & snapshot publikasi, bukan di katalog klien.
func (d *Deps) effectiveMarkup(ctx context.Context, brandID string) float64 {
	markup := d.getMarkupPercent(ctx)
	if brandID == "" {
		return markup
	}
	disc := d.brandLoyalty(ctx, brandID).DiscountPct
	return discountedMarkup(markup, disc)
}

// discountedMarkup: markup yang membuat client_price turun d% (partner tetap).
func discountedMarkup(markup, disc float64) float64 {
	if disc <= 0 {
		return markup
	}
	if disc > 100 {
		disc = 100
	}
	return ((1+markup/100)*(1-disc/100) - 1) * 100
}

// Loyalty: GET /v1/brands/{brandId}/loyalty — tier aktif + info tier berikutnya,
// untuk ditampilkan di modal checkout.
func (d *Deps) Loyalty(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d.brandLoyalty(r.Context(), bfBrand.ID))
}
