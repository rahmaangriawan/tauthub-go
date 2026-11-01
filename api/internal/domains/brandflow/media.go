package brandflow

import (
	"context"
	"math"
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// media.go: port app/v1/brands/[brandId]/{media/recommendations,
// media/[mediaId]/crawl-status, media/[mediaId]/seo, media-packages,
// packages}/route.ts.

// ---------- media/recommendations ----------

// MediaRecommendations: GET /v1/brands/{brandId}/media/recommendations
func (d *Deps) MediaRecommendations(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	q := r.URL.Query()
	category := strings.TrimSpace(q.Get("category"))
	var budget *float64
	// parity `searchParams.get("budget") ? Number(...) : null` — param kosong
	// atau NaN → null (recommendMedia: `budget != null && budget > 0`).
	if raw := q.Get("budget"); raw != "" {
		if v := bsJSNumber(raw); !math.IsNaN(v) {
			budget = &v
		}
	}
	count := bsNumClampInt(bsQueryNumber(q, "count", 10), 1, 50)
	exclude := []string{}
	if raw := q.Get("exclude"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if p := strings.TrimSpace(part); p != "" {
				exclude = append(exclude, p)
			}
		}
	}

	recommendations, err := d.bsRecommendMedia(r.Context(), category, budget, count, exclude)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recommendations": recommendations})
}

// mediaRecommendationRow: parity MediaRecommendation.
type mediaRecommendationRow struct {
	Fields map[string]any
	Score  float64
	Health *float64
}

// bsRecommendMedia: port recommendMedia (skor budget/health/kategori/kinerja
// + shuffle pool teratas).
func (d *Deps) bsRecommendMedia(ctx context.Context, category string, budget *float64, count int, excludeIDs []string) ([]map[string]any, error) {
	count = bsNumClampInt(float64(count), 1, 50)
	if budget != nil && *budget <= 0 {
		budget = nil
	}
	cat := strings.TrimSpace(category)
	exclude := map[string]bool{}
	for _, id := range excludeIDs {
		exclude[id] = true
	}

	rows, err := d.queryRows(ctx, `
		SELECT m.id, m.name, m.slug, m.category, m.website_url, m.price, m.credit_cost,
		       m.integration_type, m.health_score, m.traffic_estimate, m.status, m.dewan_pers_verified,
		       COALESCE(stats.pub_count, 0)::int AS pub_count,
		       COALESCE(stats.rej_count, 0)::int AS rej_count
		FROM media m
		LEFT JOIN (
		  SELECT media_id,
		         COUNT(*) FILTER (WHERE status = 'published')::int AS pub_count,
		         COUNT(*) FILTER (WHERE status = 'partner_rejected')::int AS rej_count
		  FROM publications GROUP BY media_id
		) stats ON stats.media_id = m.id
		WHERE m.status IN ('active','draft','pending_review')
		ORDER BY m.name ASC`)
	if err != nil {
		return nil, err
	}

	scored := []mediaRecommendationRow{}
	for _, row := range rows {
		id := row.strOr("id", "")
		if exclude[id] {
			continue
		}
		price := row.numOr("price")
		var health *float64
		if row.has("health_score") {
			v := row.numOr("health_score")
			health = &v
		}
		published := row.intOr("pub_count", 0)
		rejected := row.intOr("rej_count", 0)
		traffic := row.numOr("traffic_estimate")

		nameLower := strings.ToLower(row.strOr("name", ""))
		penalty := 0.0
		if strings.Contains(nameLower, "smoke") || strings.Contains(nameLower, "test") {
			penalty = 35
		} else if row.strOr("integration_type", "") == "manual" && strings.Contains(nameLower, "non partner") {
			penalty = 15
		}

		budgetFit := 40.0
		var fit string
		switch {
		case budget == nil:
			budgetFit = 40
		case price <= *budget:
			cap := 15.0
			if *budget != 0 {
				cap = math.Min(15, (price / *budget)*15)
			}
			budgetFit = 40 - cap
		default:
			budgetFit = 10
		}

		healthFit := 18.0
		if health != nil {
			healthFit = (*health / 100) * 30
		}
		categoryFit := 10.0
		if cat != "" && strings.EqualFold(row.strOr("category", ""), cat) {
			categoryFit = 20
		}
		perf := 0.0
		if traffic > 0 {
			perf += 3
		}
		perf += math.Min(7, float64(published)*2+traffic/100000)
		performanceFit := math.Min(10, perf)
		total := math.Max(0, math.Round(budgetFit+healthFit+categoryFit+performanceFit-penalty))

		reasons := []string{}
		if budget != nil && price <= *budget {
			reasons = append(reasons, "Sesuai budget ("+bsIntlIDR(price)+")")
		}
		if budget != nil && price > *budget {
			reasons = append(reasons, "Di atas budget "+bsIntlIDR(*budget))
		}
		if cat != "" && strings.EqualFold(row.strOr("category", ""), cat) {
			reasons = append(reasons, "Kategori "+cat)
		}
		if health != nil && *health >= 70 {
			reasons = append(reasons, "Kesehatan media "+bsNumberToString(*health)+"/100")
		}
		if published > 0 {
			reasons = append(reasons, itoa(published)+" publikasi sukses")
		}
		if rejected > 0 && published > 0 {
			ratio := math.Round((float64(rejected) / float64(published+rejected)) * 100)
			reasons = append(reasons, "Rasio tolak "+bsNumberToString(ratio)+"%")
		}
		if traffic > 0 {
			reasons = append(reasons, "Estimasi trafik "+bsIntlCompact(traffic)+"/bulan")
		}

		switch {
		case budgetFit < 20:
			fit = "budget"
		case categoryFit == 20:
			fit = "category"
		case health != nil && *health >= 70:
			fit = "health"
		default:
			fit = "performance"
		}

		var availabilityReason any
		if row.strOr("status", "") == "pending_review" {
			availabilityReason = "Menunggu Review Admin"
		}
		scored = append(scored, mediaRecommendationRow{
			Score: total, Health: health,
			Fields: map[string]any{
				"id": row.raw("id"), "name": row.raw("name"), "slug": row.raw("slug"),
				"category": row.raw("category"), "website_url": row.raw("website_url"),
				"price": price, "credit_cost": row.numOr("credit_cost"),
				"integration_type": row.strOr("integration_type", "manual"),
				"health_score":     health, "traffic_estimate": traffic,
				"score": total, "reasons": reasons, "fit": fit,
				"status":              row.strOr("status", "active"),
				"is_available":        row.strOr("status", "") == "active",
				"availability_reason": availabilityReason,
				"dewan_pers_verified": row.boolOr("dewan_pers_verified", false),
			},
		})
	}

	// Urut skor desc, health desc.
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			better := scored[j].Score > scored[i].Score ||
				(scored[j].Score == scored[i].Score && bsHealthVal(scored[j].Health) > bsHealthVal(scored[i].Health))
			if better {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	limit := count
	if len(scored) <= count {
		out := make([]map[string]any, 0, len(scored))
		for _, item := range scored {
			out = append(out, item.Fields)
		}
		return out, nil
	}
	// Randomize/shuffle pool teratas (parity Math.random Fisher-Yates).
	poolSize := len(scored)
	if count*3 > 15 {
		if count*3 < poolSize {
			poolSize = count * 3
		}
	} else if 15 < poolSize {
		poolSize = 15
	}
	topPool := append([]mediaRecommendationRow{}, scored[:poolSize]...)
	for i := len(topPool) - 1; i > 0; i-- {
		j := bsRandInt(i + 1)
		topPool[i], topPool[j] = topPool[j], topPool[i]
	}
	if limit > len(topPool) {
		limit = len(topPool)
	}
	out := make([]map[string]any, 0, limit)
	for _, item := range topPool[:limit] {
		out = append(out, item.Fields)
	}
	return out, nil
}

func bsHealthVal(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// ---------- media/[mediaId]/crawl-status ----------

// MediaCrawlStatus: GET /v1/brands/{brandId}/media/{mediaId}/crawl-status
func (d *Deps) MediaCrawlStatus(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(), "SELECT id FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return
	}

	// Salinan lokal jadi sumber utama — tapi hanya selama masih segar. Sweep
	// hanya menyegarkannya tiap CRAWLHUB_STALE_DAYS, jadi tanpa batas umur ini
	// angka velocity bisa tampil beku berhari-hari padahal Crawlhub sudah punya
	// hasil crawl yang lebih baru.
	ttlDays := schEnvInt("CRAWLHUB_VELOCITY_TTL_DAYS", defaultCrawlStaleDays)
	if ttlDays <= 0 {
		ttlDays = defaultCrawlStaleDays
	}
	local, err := d.queryRow(r.Context(), `
		SELECT articles_7d, confidence, source, last_crawled_at, last_status, last_error,
		       (last_crawled_at IS NOT NULL
		        AND last_crawled_at >= NOW() - make_interval(days => $2::int)) AS fresh
		FROM crawl_targets
		WHERE media_id = $1 AND articles_7d IS NOT NULL
		LIMIT 1`, mediaID, ttlDays)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// Angka lama hanya dianggap segar bila crawl terakhir benar-benar sukses.
	// Baris yang terakhir diblokir/no_source tidak boleh tampil seolah STAGNAN
	// baru — UI memakai `stale` untuk menandai "(belum diperbarui)".
	localFreshOK := local != nil && local.boolOr("fresh", false)
	if localFreshOK {
		status := local.strOr("last_status", "")
		localFreshOK = status == "" || status == "ok"
	}
	if localFreshOK {
		writeJSON(w, http.StatusOK, map[string]any{
			"configured": true, "articles_7d": local.numPtr("articles_7d"),
			"confidence": local.raw("confidence"), "source": local.raw("source"),
			"last_crawled_at": local.timePtr("last_crawled_at"), "stale": false,
			"last_status": local.raw("last_status"), "last_error": local.raw("last_error"),
		})
		return
	}
	// localPayload: angka terakhir yang pernah terukur tetap ditampilkan kalau
	// Crawlhub tidak terjangkau atau run terakhirnya tidak menghasilkan angka
	// (mis. diblokir WAF). stale=true supaya UI tidak menyimpulkan status dari
	// data lama.
	localPayload := func(configured bool, note string) map[string]any {
		payload := map[string]any{
			"configured": configured, "articles_7d": nil, "confidence": nil,
			"source": nil, "last_crawled_at": nil, "stale": true,
			"last_status": nil, "last_error": nil,
		}
		if local != nil {
			payload["articles_7d"] = local.numPtr("articles_7d")
			payload["confidence"] = local.raw("confidence")
			payload["source"] = local.raw("source")
			payload["last_crawled_at"] = local.timePtr("last_crawled_at")
			payload["last_status"] = local.raw("last_status")
			payload["last_error"] = local.raw("last_error")
		}
		if note != "" {
			payload["error"] = note
		}
		return payload
	}

	config := d.bsLoadCrawlhubConfig(r.Context())
	if !bsCrawlhubConfigured(config) {
		writeJSON(w, http.StatusOK, localPayload(false, ""))
		return
	}
	result, fetchErr := d.bsFetchCrawlhubTargetStatus(r.Context(), config, mediaID)
	if fetchErr != nil {
		// Crawlhub tidak terjangkau → pakai angka lama, bukan 500.
		writeJSON(w, http.StatusOK, localPayload(true, fetchErr.Error()))
		return
	}
	var lastRun map[string]any
	if lr, ok := result["last_run"].(map[string]any); ok {
		lastRun = lr
	}
	var articles any
	var confidence any
	var source any
	if lastRun != nil {
		articles = lastRun["articles_7d"]
		confidence = lastRun["confidence"]
		source = lastRun["source"]
	}
	if articles == nil && local != nil {
		// Run terakhir tidak menghasilkan angka: jangan buang angka terakhir
		// yang pernah terukur hanya karena satu crawl diblokir.
		writeJSON(w, http.StatusOK, localPayload(true, ""))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true, "articles_7d": articles, "confidence": confidence,
		"source": source, "last_crawled_at": result["last_crawled_at"], "stale": false,
		"last_status": "ok", "last_error": nil,
	})
}

// ---------- media/[mediaId]/seo ----------

// MediaSEO: GET /v1/brands/{brandId}/media/{mediaId}/seo
func (d *Deps) MediaSEO(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT id, website_url FROM media WHERE id = $1 AND status IN ('active', 'pending_review') LIMIT 1", mediaID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		writeJSON(w, http.StatusNotFound, bsEmptySEOResult(mediaID))
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT media_id, da, pa, ai_trust_score, ai_citation, ai_citation_checked_at, checked_at, source FROM media_seo_metrics WHERE media_id = $1 LIMIT 1",
		mediaID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		writeJSON(w, http.StatusOK, bsEmptySEOResult(mediaID))
		return
	}
	websiteURL := media.strOr("website_url", "")
	var domain any
	if websiteURL != "" {
		if dom, domErr := bsDomainFromWebsite(websiteURL); domErr == nil {
			domain = dom
		}
	}
	checkedAt := row.timePtr("checked_at")
	writeJSON(w, http.StatusOK, map[string]any{
		"media_id": row.raw("media_id"), "domain": domain,
		"da": row.numPtr("da"), "pa": row.numPtr("pa"),
		"ai_trust_score": row.numPtr("ai_trust_score"), "ai_citation": row.raw("ai_citation"),
		"ai_citation_checked_at": row.timePtr("ai_citation_checked_at"),
		"checked_at":             checkedAt, "source": row.raw("source"), "cached": true,
	})
}

// bsEmptySEOResult: bentuk respons SEO tanpa data (parity route lama).
func bsEmptySEOResult(mediaID string) map[string]any {
	return map[string]any{
		"media_id": mediaID, "domain": nil, "da": nil, "pa": nil,
		"ai_trust_score": nil, "ai_citation": nil, "ai_citation_checked_at": nil,
		"checked_at": nil, "source": nil,
	}
}

// ---------- media-packages ----------

// bsMediaPackageJSON: parity toPackageJson (client media-packages).
func bsMediaPackageJSON(pkg bsRow, items []bsRow, rateIDR float64) map[string]any {
	media := make([]map[string]any, 0, len(items))
	totalCreditCost := 0
	for _, m := range items {
		price := m.numOr("price")
		creditCost := 0
		if price > 0 {
			rate := rateIDR
			if rate < 1 {
				rate = 1
			}
			creditCost = int(math.Ceil(price / rate))
			if creditCost < 1 {
				creditCost = 1
			}
		}
		totalCreditCost += creditCost
		media = append(media, map[string]any{
			"id": m.raw("id"), "name": m.raw("name"), "category": m.raw("category"),
			"price": price, "credit_cost": creditCost,
			"dewan_pers_verified": m.boolOr("dewan_pers_verified", false),
		})
	}
	discountPercent := pkg.numOr("discount_percent")
	if discountPercent < 0 {
		discountPercent = 0
	}
	if discountPercent > 100 {
		discountPercent = 100
	}
	discounted := totalCreditCost
	if discountPercent > 0 {
		discounted = int(math.Round((float64(totalCreditCost) * (100 - discountPercent)) / 100))
		if discounted < 0 {
			discounted = 0
		}
	}
	savings := totalCreditCost - discounted
	if savings < 0 {
		savings = 0
	}
	return map[string]any{
		"id": pkg.raw("id"), "name": pkg.raw("name"), "slug": pkg.raw("slug"),
		"description": pkg.raw("description"), "source": pkg.raw("source"),
		"discount_percent": discountPercent, "media_count": len(media),
		"total_credit_cost": totalCreditCost, "discounted_credit_cost": discounted,
		"savings_credit": savings, "media": media,
	}
}

// MediaPackagesList: GET /v1/brands/{brandId}/media-packages
func (d *Deps) MediaPackagesList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}

	packages, err := d.queryRows(r.Context(), `
		SELECT mp.id, mp.name, mp.slug, mp.description, mp.discount_percent, mp.source,
		       COUNT(mpi.media_id) FILTER (WHERE m.status = 'active') AS media_count
		FROM media_packages mp
		LEFT JOIN media_package_items mpi ON mpi.package_id = mp.id
		LEFT JOIN media m ON m.id = mpi.media_id
		WHERE mp.status = 'active' AND mp.source = 'admin'
		GROUP BY mp.id, mp.name, mp.slug, mp.description, mp.discount_percent, mp.source
		ORDER BY mp.name ASC`)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Gagal memuat media packages")
		return
	}
	active := []bsRow{}
	for _, pkg := range packages {
		if pkg.intOr("media_count", 0) > 0 {
			active = append(active, pkg)
		}
	}
	if len(active) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"packages": []any{}})
		return
	}

	ids := make([]string, 0, len(active))
	for _, pkg := range active {
		ids = append(ids, pkg.strOr("id", ""))
	}
	items, err := d.queryRows(r.Context(), `
		SELECT mpi.package_id, m.id, m.name, m.category, m.price, m.credit_cost, mpi.position, m.dewan_pers_verified
		FROM media_package_items mpi
		JOIN media m ON m.id = mpi.media_id
		WHERE mpi.package_id IN (`+placeholders(1, len(ids))+`) AND m.status = 'active'
		ORDER BY mpi.position ASC`, stringArgs(ids)...)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Gagal memuat media packages")
		return
	}
	byPackage := map[string][]bsRow{}
	for _, item := range items {
		key := item.strOr("package_id", "")
		byPackage[key] = append(byPackage[key], item)
	}
	rateIDR := float64(d.getTcConfig(r.Context()).RateIDR)
	out := make([]map[string]any, 0, len(active))
	for _, pkg := range active {
		out = append(out, bsMediaPackageJSON(pkg, byPackage[pkg.strOr("id", "")], rateIDR))
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": out})
}

// ---------- packages (kredit legacy) ----------

// PackagesList: GET /v1/brands/{brandId}/packages
func (d *Deps) PackagesList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	// Parity route lama: authorize(brandId) lalu query memakai param path apa
	// adanya (bukan id kanonik).
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	brandID := bfBrand.ID
	q := r.URL.Query()
	// parity `Math.min(Math.max(Number(limit ?? 100), 1), 500)` dan
	// `Math.max(Number(offset ?? 0), 0)`.
	limit := bsNumClampInt(bsQueryNumber(q, "limit", 100), 1, 500)
	offsetVal := bsQueryNumber(q, "offset", 0)
	offset := 0
	if !math.IsNaN(offsetVal) && offsetVal > 0 {
		offset = int(offsetVal)
	}

	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM packages WHERE brand_id = $1", brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items, err := d.queryRows(r.Context(), `
		SELECT id, name, credit_total, credit_used, price, status, starts_at, ends_at, created_at, updated_at
		FROM packages WHERE brand_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, brandID, limit, offset)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	balance := d.brandCreditBalance(r.Context(), brandID)
	packages := make([]map[string]any, 0, len(items))
	for _, row := range items {
		creditTotal := row.intOr("credit_total", 0)
		creditUsed := row.intOr("credit_used", 0)
		packages = append(packages, map[string]any{
			"id": row.raw("id"), "name": row.raw("name"),
			"credit_total": creditTotal, "credit_used": creditUsed,
			"credit_remaining": creditTotal - creditUsed,
			"price":            row.numOr("price"), "status": row.raw("status"),
			"starts_at": row.timePtr("starts_at"), "ends_at": row.timePtr("ends_at"),
			"created_at": row.timePtr("created_at"), "updated_at": row.timePtr("updated_at"),
		})
	}
	total := 0
	if countRow != nil {
		total = countRow.intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"packages": packages,
		"balance": map[string]any{
			"total": balance.Total, "used": balance.Used,
			"remaining": balance.Remaining, "package_count": balance.PackageCount,
		},
		"total": total, "limit": limit, "offset": offset,
	})
}

// ---------- util ----------

// bsRandInt: angka acak [0,n) memakai math/rand (determinisme tidak dituntut).
func bsRandInt(n int) int {
	if n <= 0 {
		return 0
	}
	return randInt(n)
}
