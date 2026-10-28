package publiccatalog

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// publicSelect: kolom yang dibaca katalog publik. Sengaja tidak menyentuh
// partner_id/notes/markup/adapter/credentials — lihat kontrak paket.
const publicSelect = `
SELECT m.public_slug, m.name, m.category, m.description, m.website_url,
       m.contoh_artikel_url, m.media_type, m.media_tier, m.bahasa_artikel,
       m.traffic_estimate, m.max_links, m.allow_hyperlink, m.link_duration_months,
       m.word_count_mode, m.allow_embed, m.label_sponsor, m.price, m.pricing_type,
       m.dewan_pers_verified, m.domain_verified, m.updated_at,
       -- SLA publik = rata-rata jam terbit nyata media ini (dihitung sistem
       -- dari riwayat publikasi). NULL bila riwayatnya belum cukup.
       (m.health_breakdown->>'avg_publish_hours') AS avg_publish_hours,
       COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name)
                 FROM media_category_links l JOIN media_categories c ON c.id = l.category_id
                 WHERE l.media_id = m.id), m.category) AS categories_csv,
       sm.da, sm.pa, sm.spam_score, sm.ai_trust_score, sm.ai_citation, sm.checked_at,
       mcs.image_required, mcs.min_words, mcs.max_words, mcs.journalistic_style,
       mcs.quote_required, mcs.min_quotes, mcs.category_required,
       (SELECT COUNT(*)::int FROM publications p
          WHERE p.media_id = m.id AND p.status = 'published') AS published_count,
       (SELECT ROUND(AVG(r.rating)::numeric, 1)::float8 FROM media_ratings r
          WHERE r.media_id = m.id) AS rating_avg,
       (SELECT COUNT(*)::int FROM media_ratings r
          WHERE r.media_id = m.id) AS rating_count,
       sw.visits AS sw_visits, sw.created_at AS sw_snapshot_at
FROM media m
LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
LEFT JOIN LATERAL (
    SELECT image_required, min_words, max_words, journalistic_style,
           quote_required, min_quotes, category_required
    FROM media_content_standards mcs
    WHERE mcs.media_id = m.id AND mcs.status = 'active'
    ORDER BY mcs.updated_at DESC
    LIMIT 1
) mcs ON TRUE
LEFT JOIN LATERAL (
    SELECT CASE WHEN s.insights->>'visits' ~ '^-?[0-9]+([.][0-9]+)?$'
                THEN ((s.insights->>'visits')::numeric)::bigint END AS visits,
           s.created_at
    FROM media_similarweb_snapshots s
    WHERE s.media_id = m.id
    ORDER BY s.created_at DESC LIMIT 1
) sw ON TRUE
`

var publicSorts = map[string]string{
	"nama":       "m.name ASC",
	"da":         "sm.da DESC NULLS LAST, m.name ASC",
	"trafik":     "m.traffic_estimate DESC, m.name ASC",
	"harga_asc":  "m.price ASC, m.name ASC",
	"harga_desc": "m.price DESC, m.name ASC",
	"baru":       "m.updated_at DESC NULLS LAST, m.name ASC",
}

// categorySlugExpr: ekspresi SQL slug kategori (kebab-case) dari kolom teks.
// Kategori disimpan sebagai nama bebas ("Media Nasional"), jadi slug untuk URL
// dihitung dengan ekspresi yang sama di listing dan di filter — kalau berbeda,
// tautan kategori akan berujung "tidak ditemukan".
func categorySlugExpr(arg string) string {
	return "trim(both '-' from regexp_replace(lower(trim(" + arg + ")), '[^a-z0-9]+', '-', 'g'))"
}

// GET /v1/public/media
func (d *Deps) MediaList(w http.ResponseWriter, r *http.Request) {
	if !d.gate(w, r) {
		return
	}
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("q"))
	if len(search) > 120 {
		search = search[:120]
	}
	category := strings.TrimSpace(q.Get("kategori"))
	tier := strings.TrimSpace(q.Get("tier"))
	minDA := atoiOr(q.Get("da_awal"), 0)
	maxPrice := atoiOr(q.Get("harga_maks"), 0)
	page := clampInt(q.Get("page"), 1, 1, 10_000)
	limit := clampInt(q.Get("limit"), 24, 1, 60)

	markup := d.markupPercent(r)

	where := []string{publicWhere}
	args := []any{}
	addArg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if search != "" {
		like := "%" + search + "%"
		p := addArg(like)
		where = append(where, "(m.name ILIKE "+p+" OR m.website_url ILIKE "+p+
			" OR m.category ILIKE "+p+" OR m.description ILIKE "+p+")")
	}
	if category != "" {
		norm := strings.ToLower(category)
		p := addArg(norm)
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
	if maxPrice > 0 {
		// Harga publik = harga mitra × (1 + markup). Filter memakai angka yang
		// sama dengan yang ditampilkan, bukan harga mitra.
		where = append(where, "(m.price * (1 + "+addArg(markup)+" / 100.0)) <= "+addArg(float64(maxPrice)))
	}

	clause := " WHERE " + strings.Join(where, " AND ")

	total := 0
	if rows, err := d.queryRows(r, "SELECT COUNT(*)::int AS total FROM media m LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id"+clause, args...); err != nil {
		logErr("hitung katalog gagal", err)
		httpx.Error(w, http.StatusInternalServerError, "Gagal membaca katalog.")
		return
	} else if len(rows) > 0 {
		total = rows[0].intOr("total", 0)
	}

	order := publicSorts[q.Get("urut")]
	if order == "" {
		order = publicSorts["nama"]
	}
	// Sort harga memakai harga publik (harga mitra × markup tetap monoton, jadi
	// urutan m.price setara dengan urutan client_price).
	sql := publicSelect + clause + " ORDER BY " + order +
		" LIMIT " + addArg(limit) + " OFFSET " + addArg((page-1)*limit)

	rows, err := d.queryRows(r, sql, args...)
	if err != nil {
		logErr("baca katalog gagal", err)
		httpx.Error(w, http.StatusInternalServerError, "Gagal membaca katalog.")
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, publicItem(row, markup, false))
	}

	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}
	writePublicJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": pages,
	}, 60)
}

// GET /v1/public/media/{publicSlug}
func (d *Deps) MediaDetail(w http.ResponseWriter, r *http.Request) {
	if !d.gate(w, r) {
		return
	}
	slug := strings.TrimSpace(chi.URLParam(r, "publicSlug"))
	if slug == "" {
		httpx.Error(w, http.StatusNotFound, "Media tidak ditemukan.")
		return
	}
	markup := d.markupPercent(r)
	rows, err := d.queryRows(r,
		publicSelect+" WHERE "+publicWhere+" AND m.public_slug = $1 LIMIT 1", slug)
	if err != nil {
		logErr("baca detail media gagal", err)
		httpx.Error(w, http.StatusInternalServerError, "Gagal membaca media.")
		return
	}
	if len(rows) == 0 {
		httpx.Error(w, http.StatusNotFound, "Media tidak ditemukan.")
		return
	}

	item := publicItem(rows[0], markup, true)

	// Ringkasan AI tentang situs media (bila ada) — konten unik yang berguna
	// untuk halaman detail, bukan data internal.
	var summary pgtype.Text
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT NULLIF(TRIM(ai_summary), '') FROM media WHERE public_slug = $1", slug).Scan(&summary); err == nil && summary.Valid {
		item["about"] = summary.String
	}

	writePublicJSON(w, http.StatusOK, item, 300)
}

// GET /v1/public/kategori
func (d *Deps) Categories(w http.ResponseWriter, r *http.Request) {
	if !d.gate(w, r) {
		return
	}
	rows, err := d.queryRows(r, `
		SELECT name, slug, COUNT(*)::int AS count FROM (
		  SELECT c.name AS name, `+categorySlugExpr("coalesce(c.normalized, c.name)")+` AS slug, l.media_id AS media_id
		  FROM media_category_links l
		  JOIN media_categories c ON c.id = l.category_id
		  JOIN media m ON m.id = l.media_id
		  WHERE `+publicWhere+`
		  UNION ALL
		  SELECT m.category AS name, `+categorySlugExpr("m.category")+` AS slug, m.id AS media_id
		  FROM media m
		  WHERE `+publicWhere+`
		    AND m.category IS NOT NULL AND trim(m.category) <> ''
		    AND NOT EXISTS (SELECT 1 FROM media_category_links l WHERE l.media_id = m.id)
		) s
		GROUP BY name, slug
		ORDER BY count DESC, name ASC
		LIMIT 200`)
	if err != nil {
		logErr("baca kategori gagal", err)
		httpx.Error(w, http.StatusInternalServerError, "Gagal membaca kategori.")
		return
	}
	// Gabungkan per slug: dua nama berbeda bisa menghasilkan slug yang sama
	// (mis. "Bisnis" dan "Bisnis,"), dan satu kategori = satu URL. Baris sudah
	// terurut count DESC, jadi nama yang pertama muncul adalah yang paling umum.
	items := make([]map[string]any, 0, len(rows))
	index := map[string]int{}
	for _, row := range rows {
		slug := row.str("slug")
		if slug == "" {
			continue
		}
		count := row.intOr("count", 0)
		if i, ok := index[slug]; ok {
			items[i]["count"] = items[i]["count"].(int) + count
			continue
		}
		index[slug] = len(items)
		items = append(items, map[string]any{
			"name":  strings.TrimSpace(row.str("name")),
			"slug":  slug,
			"url":   "/kategori/" + slug,
			"count": count,
		})
	}
	writePublicJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)}, 60)
}

// GET /v1/public/statistik
func (d *Deps) Stats(w http.ResponseWriter, r *http.Request) {
	if !d.gate(w, r) {
		return
	}
	markup := d.markupPercent(r)
	rows, err := d.queryRows(r, `
		SELECT COUNT(*)::int AS total,
		       COUNT(*) FILTER (WHERE m.media_tier = 'tier1')::int AS tier1,
		       COUNT(*) FILTER (WHERE m.media_tier = 'tier2')::int AS tier2,
		       COUNT(*) FILTER (WHERE m.media_tier = 'tier3')::int AS tier3,
		       COALESCE(MIN(m.price), 0) AS min_partner_price,
		       COALESCE(AVG(sm.da), 0)::float8 AS avg_da,
		       COALESCE(MAX(m.updated_at), NOW()) AS updated_at
		FROM media m
		LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
		WHERE `+publicWhere)
	if err != nil {
		logErr("baca statistik gagal", err)
		httpx.Error(w, http.StatusInternalServerError, "Gagal membaca statistik.")
		return
	}
	categoryTotal := 0
	if rowsCat, err := d.queryRows(r,
		"SELECT COUNT(DISTINCT COALESCE(NULLIF("+categorySlugExpr("coalesce(c.normalized, c.name)")+", ''), "+
			categorySlugExpr("m.category")+"))::int AS total "+
			"FROM media m LEFT JOIN media_category_links l ON l.media_id = m.id "+
			"LEFT JOIN media_categories c ON c.id = l.category_id WHERE "+publicWhere); err == nil && len(rowsCat) > 0 {
		categoryTotal = rowsCat[0].intOr("total", 0)
	}

	payload := map[string]any{
		"total":      0,
		"categories": categoryTotal,
		"tier1":      0,
		"tier2":      0,
		"tier3":      0,
		"price_min":  0,
		"avg_da":     0,
		"updated_at": nil,
	}
	if len(rows) > 0 {
		row := rows[0]
		payload["total"] = row.intOr("total", 0)
		payload["tier1"] = row.intOr("tier1", 0)
		payload["tier2"] = row.intOr("tier2", 0)
		payload["tier3"] = row.intOr("tier3", 0)
		payload["price_min"] = clientPrice(row.numOr("min_partner_price"), markup)
		avg := row.numOr("avg_da")
		payload["avg_da"] = float64(int(avg*10+0.5)) / 10
		payload["updated_at"] = row.timePtr("updated_at")
	}
	// Cache pendek: angka total dipakai hero katalog dan harus mengikuti
	// penambahan/penghapusan media, bukan tertahan 5 menit.
	writePublicJSON(w, http.StatusOK, payload, 30)
}

// GET /v1/public/sitemap — daftar URL yang boleh diindeks (dipakai web untuk
// membangun sitemap XML; dipisah dari listing supaya bisa diambil sekali jalan).
func (d *Deps) Sitemap(w http.ResponseWriter, r *http.Request) {
	if !d.gate(w, r) {
		return
	}
	rows, err := d.queryRows(r,
		"SELECT m.public_slug AS slug, m.updated_at FROM media m WHERE "+publicWhere+
			" ORDER BY m.updated_at DESC NULLS LAST LIMIT 50000")
	if err != nil {
		logErr("baca sitemap gagal", err)
		httpx.Error(w, http.StatusInternalServerError, "Gagal membaca sitemap.")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		slug := row.str("slug")
		if slug == "" {
			continue
		}
		items = append(items, map[string]any{
			"slug":       slug,
			"updated_at": row.timePtr("updated_at"),
		})
	}
	writePublicJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)}, 600)
}

// publicItem: bentuk JSON publik satu media. detail=false memotong deskripsi
// dan membuang field yang hanya relevan di halaman detail.
func publicItem(row row, markup float64, detail bool) map[string]any {
	slug := row.str("public_slug")
	description := strings.TrimSpace(row.str("description"))
	if !detail && len([]rune(description)) > 280 {
		description = string([]rune(description)[:280]) + "…"
	}

	websiteURL := row.str("website_url")
	domain := hostOf(websiteURL)

	trafficSource := "estimate"
	var traffic any = row.numPtr("traffic_estimate")
	visits := row.numPtr("sw_visits")
	if visits != nil {
		trafficSource = "similarweb"
		traffic = visits
	}

	priceType := row.str("pricing_type")
	if priceType == "" {
		priceType = "nett"
	}

	item := map[string]any{
		"slug":         slug,
		"name":         row.str("name"),
		"url":          "/media/" + slug,
		"domain":       domain,
		"website_url":  nilIfEmpty(websiteURL),
		"description":  nilIfEmpty(description),
		"categories":   splitCategories(row.str("categories_csv")),
		"media_type":   nilIfEmpty(row.str("media_type")),
		"tier":         nilIfEmpty(row.str("media_tier")),
		"language":     parseLanguageList(row.str("bahasa_artikel")),
		"traffic": map[string]any{
			"monthly": traffic,
			"source":  trafficSource,
			"as_of":   row.timePtr("sw_snapshot_at"),
		},
		"seo": map[string]any{
			"da":             row.numPtr("da"),
			"pa":             row.numPtr("pa"),
			"spam_score":     row.numPtr("spam_score"),
			"ai_trust_score": row.numPtr("ai_trust_score"),
			"ai_citation":    row.boolPtr("ai_citation"),
			"checked_at":     row.timePtr("checked_at"),
		},
		"price": map[string]any{
			"amount":   clientPrice(row.numOr("price"), markup),
			"currency": "IDR",
			"type":     priceType,
		},
		"link_policy": map[string]any{
			"max_links":       row.intPtr("max_links"),
			"dofollow":        row.boolOr("allow_hyperlink", false),
			"duration_months": row.intOr("link_duration_months", 0),
		},
		"content": map[string]any{
			"word_count_mode": row.str("word_count_mode"),
			"image_required":  row.boolElse("image_required", false),
			"allow_embed":     row.boolOr("allow_embed", false),
			"sponsor_label":   row.str("label_sponsor"),
		},
		"trust": map[string]any{
			"dewan_pers_verified": row.boolOr("dewan_pers_verified", false),
			"domain_verified":     row.boolOr("domain_verified", false),
			"published_count":     row.intOr("published_count", 0),
			"rating_avg":          row.numPtr("rating_avg"),
			"rating_count":        row.intOr("rating_count", 0),
		},
		// SLA: jam rata-rata dari kirim artikel sampai tayang. Angkanya dari
		// health_breakdown yang dihitung sistem, bukan taksiran per tier.
		"sla": map[string]any{
			"avg_publish_hours": row.numPtr("avg_publish_hours"),
		},
		"example_article_url": nilIfEmpty(row.str("contoh_artikel_url")),
		"updated_at":          row.timePtr("updated_at"),
	}

	if detail {
		item["standards"] = map[string]any{
			"min_words":          row.intPtr("min_words"),
			"max_words":          row.intPtr("max_words"),
			"journalistic_style": nilIfEmpty(row.str("journalistic_style")),
			"quote_required":     row.boolElse("quote_required", false),
			"min_quotes":         row.intOr("min_quotes", 0),
			"category_required":  row.boolElse("category_required", false),
		}
	}
	return item
}

func (m row) boolPtr(key string) any {
	if b, ok := m.raw(key).(bool); ok {
		return b
	}
	return nil
}

// boolElse: default dipakai bila kolomnya NULL (standar konten belum diisi).
func (m row) boolElse(key string, fallback bool) bool {
	if b, ok := m.raw(key).(bool); ok {
		return b
	}
	return fallback
}

func nilIfEmpty(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
