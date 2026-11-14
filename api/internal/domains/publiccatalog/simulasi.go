package publiccatalog

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Simulasi paket: pengunjung memasukkan anggaran (dan, bila mau, kategori,
// tier, DA minimal, jumlah media), lalu katalog menyusun tiga paket contoh dari
// media publik yang nyata. Harga yang dipakai sama dengan yang tampil di
// katalog (harga mitra × markup), jadi total di simulasi tidak berbeda dari
// total yang akan dilihat klien saat memesan.

const (
	simBudgetMin   = 100_000
	simBudgetMax   = 10_000_000_000
	simCandidates  = 400
	simDefaultCap  = 12
	simMaxCountCap = 30
)

type simMedia struct {
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Domain  string   `json:"domain"`
	Tier    *string  `json:"tier"`
	DA      *float64 `json:"da"`
	Traffic *float64 `json:"traffic"`
	Price   float64  `json:"price"`
}

type simPackage struct {
	Key      string     `json:"key"`
	Label    string     `json:"label"`
	Summary  string     `json:"summary"`
	Items    []simMedia `json:"items"`
	Count    int        `json:"count"`
	Total    float64    `json:"total"`
	Leftover float64    `json:"leftover"`
	AvgDA    *float64   `json:"avg_da"`
	Traffic  float64    `json:"traffic_total"`
}

// GET /v1/public/simulasi?anggaran=…&jumlah=…&kategori=…&tier=…&da_awal=…
func (d *Deps) Simulate(w http.ResponseWriter, r *http.Request) {
	if !d.gate(w, r) {
		return
	}
	q := r.URL.Query()
	budget := atoiOr(q.Get("anggaran"), 0)
	if budget < simBudgetMin || budget > simBudgetMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Anggaran harus antara Rp100.000 dan Rp10.000.000.000.")
		return
	}
	wantCount := clampInt(q.Get("jumlah"), 0, 0, simMaxCountCap)
	capCount := simDefaultCap
	if wantCount > 0 {
		capCount = wantCount
	}
	category := strings.ToLower(strings.TrimSpace(q.Get("kategori")))
	tier := strings.TrimSpace(q.Get("tier"))
	minDA := atoiOr(q.Get("da_awal"), 0)

	markup := d.markupPercent(r)

	where := []string{publicWhere, "m.price > 0"}
	args := []any{}
	addArg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if category != "" {
		p := addArg(category)
		where = append(where, "(EXISTS (SELECT 1 FROM media_category_links l JOIN media_categories c ON c.id = l.category_id "+
			"WHERE l.media_id = m.id AND "+categorySlugExpr("coalesce(c.normalized, c.name)")+" = "+p+") OR "+
			categorySlugExpr("m.category")+" = "+p+")")
	}
	if tier != "" {
		where = append(where, "m.media_tier = "+addArg(tier))
	}
	if minDA > 0 && minDA <= 100 {
		where = append(where, "sm.da >= "+addArg(minDA))
	}
	// Hanya media yang masih muat dalam anggaran yang berguna sebagai kandidat.
	where = append(where, "(m.price * (1 + "+addArg(markup)+" / 100.0)) <= "+addArg(float64(budget)))

	sql := publicSelect + " WHERE " + strings.Join(where, " AND ") +
		" ORDER BY sm.da DESC NULLS LAST, m.name ASC LIMIT " + strconv.Itoa(simCandidates)
	rows, err := d.queryRows(r, sql, args...)
	if err != nil {
		logErr("simulasi paket gagal", err)
		httpx.Error(w, http.StatusInternalServerError, "Gagal menyusun simulasi.")
		return
	}

	pool := make([]simMedia, 0, len(rows))
	for _, row := range rows {
		slug := row.str("public_slug")
		if slug == "" {
			continue
		}
		traffic := simNum(row, "traffic_estimate")
		if v := simNum(row, "sw_visits"); v != nil {
			traffic = v
		}
		var tierPtr *string
		if t := row.str("media_tier"); t != "" {
			tierPtr = &t
		}
		pool = append(pool, simMedia{
			Slug:    slug,
			Name:    row.str("name"),
			URL:     "/media/" + slug,
			Domain:  hostOf(row.str("website_url")),
			Tier:    tierPtr,
			DA:      simNum(row, "da"),
			Traffic: traffic,
			Price:   clientPrice(row.numOr("price"), markup),
		})
	}

	daOf := func(m simMedia) float64 {
		if m.DA == nil {
			return 0
		}
		return *m.DA
	}

	packages := []simPackage{
		buildPackage("hemat", "Hemat", "Sebanyak mungkin media dengan harga termurah.", pool, float64(budget), capCount,
			func(a, b simMedia) bool { return a.Price < b.Price }),
		buildPackage("seimbang", "Seimbang", "Media dengan DA tertinggi untuk tiap rupiah.", pool, float64(budget), capCount,
			func(a, b simMedia) bool { return (daOf(a)+10)/a.Price > (daOf(b)+10)/b.Price }),
		buildPackage("premium", "Premium", "Media dengan DA tertinggi yang muat di anggaran.", pool, float64(budget), capCount,
			func(a, b simMedia) bool {
				if daOf(a) != daOf(b) {
					return daOf(a) > daOf(b)
				}
				return a.Price < b.Price
			}),
	}

	writePublicJSON(w, http.StatusOK, map[string]any{
		"budget":     budget,
		"candidates": len(pool),
		"packages":   packages,
	}, 60)
}

func simNum(r row, key string) *float64 {
	if f, ok := r.num(key); ok {
		return &f
	}
	return nil
}

// buildPackage memilih media berurutan menurut `better` dan mengambil yang
// masih muat di sisa anggaran, sampai jumlahnya mencapai batas.
func buildPackage(key, label, summary string, pool []simMedia, budget float64, capCount int, better func(a, b simMedia) bool) simPackage {
	sorted := append([]simMedia(nil), pool...)
	sort.SliceStable(sorted, func(i, j int) bool { return better(sorted[i], sorted[j]) })

	pkg := simPackage{Key: key, Label: label, Summary: summary, Items: []simMedia{}}
	left := budget
	var daSum float64
	var daN int
	for _, m := range sorted {
		if len(pkg.Items) >= capCount {
			break
		}
		if m.Price > left {
			continue
		}
		pkg.Items = append(pkg.Items, m)
		left -= m.Price
		pkg.Total += m.Price
		if m.DA != nil {
			daSum += *m.DA
			daN++
		}
		if m.Traffic != nil {
			pkg.Traffic += *m.Traffic
		}
	}
	pkg.Count = len(pkg.Items)
	pkg.Leftover = math.Max(left, 0)
	if daN > 0 {
		avg := math.Round(daSum/float64(daN)*10) / 10
		pkg.AvgDA = &avg
	}
	return pkg
}
