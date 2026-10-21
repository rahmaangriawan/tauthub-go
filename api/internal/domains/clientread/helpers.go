// Package clientread: endpoint BACA untuk section klien (katalog media,
// kampanye, pelacak 404, analisis kinerja, kredit, invoice) — port dari
// app/v1/brands/[brandId]/{media,campaigns,link-monitor,analytics,credits,invoices}.
// Semua endpoint di sini read-only; mutasi tetap di paket domain masing-masing.
package clientread

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

type Deps struct {
	Pool *pgxpool.Pool
}

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func (d *Deps) access(r *http.Request, brandID, userID, minRole string) (brandsdomain.BrandRow, error) {
	return brandsdomain.BrandAccess(r.Context(), d.Pool, brandID, userID, minRole)
}

func writeErr(w http.ResponseWriter, err error) {
	if be, ok := err.(*brandsdomain.BrandError); ok {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func clampInt(raw string, def, min, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		n = def
	}
	if n < min {
		n = min
	}
	if n > max {
		n = max
	}
	return n
}

func jsonRaw(b []byte, fallback any) any {
	if len(b) == 0 {
		return fallback
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil || v == nil {
		return fallback
	}
	return v
}

func isoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// ---------- pricing engine (port lib/pricing/engine.ts) ----------

const defaultMarkupPercent = 15

type markupCache struct {
	value   float64
	expires time.Time
}

var markupCached *markupCache

// getMarkupPercent: pricing_config id=1, default 15, cache 30s.
func (d *Deps) getMarkupPercent(r *http.Request) float64 {
	now := time.Now()
	if markupCached != nil && markupCached.expires.After(now) {
		return markupCached.value
	}
	value := 0.0
	found := false
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT markup_percent FROM pricing_config WHERE id = 1 LIMIT 1").Scan(&value); err == nil {
		found = value >= 0
	}
	if !found {
		value = defaultMarkupPercent
	}
	markupCached = &markupCache{value: value, expires: now.Add(30 * time.Second)}
	return value
}

// priceBreakdown: parity priceBreakdown (round 2 desimal).
func priceBreakdown(partnerPrice, markupPercent float64) map[string]float64 {
	partner := partnerPrice
	if partner < 0 {
		partner = 0
	}
	markup := markupPercent
	if markup < 0 {
		markup = 0
	}
	client := round2(partner * (1 + markup/100))
	return map[string]float64{
		"partner_price":  partner,
		"markup_percent": markup,
		"markup_amount":  round2(client - partner),
		"client_price":   client,
	}
}

func round2(v float64) float64 {
	if v < 0 {
		return float64(int(v*100-0.5)) / 100
	}
	return float64(int(v*100+0.5)) / 100
}

// normalizeCategory: parity normalizeCategory.
func normalizeCategory(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
