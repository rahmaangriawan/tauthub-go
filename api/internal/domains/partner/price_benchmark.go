package partner

import (
	"context"
	"sort"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
)

// price_benchmark.go: pembanding harga sebaya untuk Pusat Penjual.
//
// Harga mitra dibandingkan dengan harga media aktif lain (harga mitra, sebelum
// markup, sama seperti yang diisi di formulir media). Pembanding dipilih
// bertahap dari yang paling mirip:
//
//	1. kategori utama sama + jenis media sama + DA berselisih paling banyak 10
//	2. kategori utama sama + jenis media sama
//	3. jenis media sama saja
//
// Tahap dipakai bila menghasilkan minimal benchmarkMinPeers pembanding, supaya
// median tidak ditentukan satu atau dua media. Kalau tahap terakhir pun kurang,
// tidak ada saran ("Data pembanding belum cukup"). Identitas pembanding tidak
// pernah dikirim ke mitra.

const (
	benchmarkMinPeers = 5
	benchmarkDARange  = 10.0
)

type peerMedia struct {
	ID        string
	Category  string
	MediaType string
	Price     float64
	DA        *float64
}

type priceBenchmark struct {
	Peers    int     `json:"peers"`
	Basis    string  `json:"basis"` // category_da | category | media_type
	Median   float64 `json:"median"`
	P25      float64 `json:"p25"`
	P75      float64 `json:"p75"`
	Position string  `json:"position"` // below | within | above
	Price    float64 `json:"price"`
}

func primaryCategory(raw string) string {
	parts := catutil.Split(raw)
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[0]))
}

// percentile: interpolasi linear pada slice yang sudah terurut naik.
func percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(pos)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := pos - float64(lo)
	return sorted[lo] + (sorted[hi]-sorted[lo])*frac
}

// benchmarkPrice: pembanding harga untuk satu media. nil = data belum cukup
// atau media belum punya harga.
func benchmarkPrice(self peerMedia, pool []peerMedia) *priceBenchmark {
	if self.Price <= 0 {
		return nil
	}
	selfCat := primaryCategory(self.Category)
	byType := make([]peerMedia, 0, len(pool))
	for _, p := range pool {
		if p.ID != self.ID && p.Price > 0 && p.MediaType == self.MediaType {
			byType = append(byType, p)
		}
	}
	byCategory := make([]peerMedia, 0, len(byType))
	if selfCat != "" {
		for _, p := range byType {
			if primaryCategory(p.Category) == selfCat {
				byCategory = append(byCategory, p)
			}
		}
	}
	byDA := make([]peerMedia, 0, len(byCategory))
	if self.DA != nil {
		for _, p := range byCategory {
			if p.DA == nil {
				continue
			}
			diff := *p.DA - *self.DA
			if diff < 0 {
				diff = -diff
			}
			if diff <= benchmarkDARange {
				byDA = append(byDA, p)
			}
		}
	}

	var chosen []peerMedia
	basis := ""
	switch {
	case len(byDA) >= benchmarkMinPeers:
		chosen, basis = byDA, "category_da"
	case len(byCategory) >= benchmarkMinPeers:
		chosen, basis = byCategory, "category"
	case len(byType) >= benchmarkMinPeers:
		chosen, basis = byType, "media_type"
	default:
		return nil
	}

	prices := make([]float64, 0, len(chosen))
	for _, p := range chosen {
		prices = append(prices, p.Price)
	}
	sort.Float64s(prices)
	p25, med, p75 := percentile(prices, 0.25), percentile(prices, 0.5), percentile(prices, 0.75)
	position := "within"
	if self.Price < p25 {
		position = "below"
	} else if self.Price > p75 {
		position = "above"
	}
	return &priceBenchmark{
		Peers: len(chosen), Basis: basis, Median: med, P25: p25, P75: p75, Position: position, Price: self.Price,
	}
}

// loadPeerPool: media aktif berharga untuk pembanding. Dimuat sekali per
// permintaan Pusat Penjual.
func (d *Deps) loadPeerPool(ctx context.Context) []peerMedia {
	rows, err := d.queryRows(ctx, `
		SELECT m.id, m.category, m.media_type, m.price::float8 AS price, sm.da::float8 AS da
		FROM media m
		LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
		WHERE m.status = 'active' AND m.suspended_at IS NULL AND m.price > 0
		LIMIT 5000`)
	if err != nil {
		return nil
	}
	pool := make([]peerMedia, 0, len(rows))
	for _, row := range rows {
		p := peerMedia{
			ID:        row.strOr("id", ""),
			Category:  row.strOr("category", ""),
			MediaType: row.strOr("media_type", "mainstream"),
			Price:     row.numOr("price"),
		}
		if v, ok := row.num("da"); ok {
			p.DA = &v
		}
		pool = append(pool, p)
	}
	return pool
}
