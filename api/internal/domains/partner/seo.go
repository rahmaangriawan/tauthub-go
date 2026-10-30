package partner

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/seo"
)

// ---------- SEO: cek domain (port seo/check) ----------

const seoCacheTTL = 24 * time.Hour

// normalizeDomainSeo: parity normalizeDomain — validasi + buang www.
func normalizeDomainSeo(raw string) (string, *PartnerError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", &PartnerError{http.StatusBadRequest, "Masukkan domain untuk diperiksa."}
	}
	parsed, err := urlParse(ensureScheme(value))
	if err != nil || parsed.Host == "" {
		return "", &PartnerError{http.StatusBadRequest, "Domain tidak valid."}
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if host == "" || isUnsafeHost(host) {
		return "", &PartnerError{http.StatusBadRequest, "Domain tidak dapat diperiksa."}
	}
	return host, nil
}

func (d *Deps) SEOCheck(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := d.queryRows(r.Context(),
			"SELECT id, domain, da, pa, spam_score, checked_at FROM partner_seo_check_history WHERE partner_id = $1 ORDER BY checked_at DESC LIMIT 10",
			partner.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		history := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			history = append(history, map[string]any{
				"domain":     row.raw("domain"),
				"da":         row.numPtr("da"),
				"pa":         row.numPtr("pa"),
				"spam_score": row.numPtr("spam_score"),
				"checked_at": row.timePtr("checked_at"),
			})
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"history": history})
		return
	}

	body := decodeJSON(r)
	rawDomain, present := strField(body, "domain")
	if !present || len(rawDomain) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan domain yang valid.")
		return
	}
	if len(rawDomain) > 190 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan domain yang valid.")
		return
	}
	domain, domErr := normalizeDomainSeo(rawDomain)
	if domErr != nil {
		httpx.Error(w, domErr.Status, domErr.Message)
		return
	}
	cached, err := d.queryRow(r.Context(),
		"SELECT id, domain, da, pa, spam_score, checked_at FROM partner_seo_check_history WHERE partner_id = $1 AND domain = $2 ORDER BY checked_at DESC LIMIT 1",
		partner.ID, domain)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if cached != nil {
		if checkedAt, okTime := cached.raw("checked_at").(time.Time); okTime && time.Since(checkedAt) < seoCacheTTL {
			writeJSON(w, http.StatusOK, map[string]any{
				"domain":     cached.raw("domain"),
				"da":         cached.numPtr("da"),
				"pa":         cached.numPtr("pa"),
				"spam_score": cached.numPtr("spam_score"),
				"checked_at": cached.timePtr("checked_at"),
				"source":     "history-cache",
				"cached":     true,
			})
			return
		}
	}
	result, seoErr := d.checkDomainSEO(r, domain)
	if seoErr != nil {
		httpx.Error(w, seoErr.Status, seoErr.Message)
		return
	}
	if err := d.exec(r.Context(),
		"INSERT INTO partner_seo_check_history (id, partner_id, domain, da, pa, spam_score, checked_at) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW())",
		partner.ID, domain, result["da"], result["pa"], result["spam_score"]); err != nil {
		d.writeErr(w, err)
		return
	}
	// Pertahankan riwayat maksimum 50 baris per mitra.
	_ = d.exec(r.Context(), `
		DELETE FROM partner_seo_check_history WHERE id IN (
		  SELECT id FROM partner_seo_check_history WHERE partner_id = $1 ORDER BY checked_at DESC OFFSET 50
		)`, partner.ID)
	result["cached"] = false
	writeJSON(w, http.StatusOK, result)
}

// checkDomainSEO: ambil DA/PA/spam dari provider RapidAPI.
func (d *Deps) checkDomainSEO(r *http.Request, domain string) (map[string]any, *PartnerError) {
	apiKey, err := d.platformSeoAPIKey(r, "sealed_api_key")
	if err != nil {
		return nil, err
	}
	result, err := d.fetchSeoMetrics(r, domain, apiKey)
	if err != nil {
		return nil, err
	}
	result["checked_at"] = isoMilli(nowUTC())
	result["source"] = "rapidapi-domain-metrics"
	if result["spam_score"] == nil {
		// Provider spam checker lama dengan key RapidAPI yang sama (kolom
		// sealed_spam_api_key tidak ada di skema, jalur lama selalu gagal).
		if spam, fetchErr := d.fetchSpamScore(r, domain, apiKey); fetchErr == nil && spam != nil {
			result["spam_score"] = *spam
		}
	}
	return result, nil
}

func (d *Deps) platformSeoAPIKey(r *http.Request, column string) (string, *PartnerError) {
	row, err := d.queryRow(r.Context(),
		"SELECT "+column+" FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		slog.Error("[seo] gagal membaca platform_seo_config", "column", column, "err", err)
		return "", &PartnerError{http.StatusConflict, "Konfigurasi integrasi SEO belum siap. Hubungi admin Tauthub."}
	}
	sealed := row.strOr(column, "")
	if sealed == "" {
		slog.Error("[seo] API key belum dikonfigurasi", "source", "platform_seo_config", "column", column)
		return "", &PartnerError{http.StatusConflict, "Konfigurasi integrasi SEO belum siap. Hubungi admin Tauthub."}
	}
	plain, err := d.unsealT2(sealed)
	if err != nil {
		slog.Error("[seo] API key tersimpan gagal di-unseal (APP_SECRET berubah?)",
			"source", "platform_seo_config", "column", column, "err", err)
		return "", &PartnerError{http.StatusConflict, "Kredensial tersimpan tidak dapat dibuka. Hubungi admin Tauthub untuk menyimpan ulang."}
	}
	return plain, nil
}

// fetchSeoMetrics: cek DA/PA lewat paket internal/seo (provider sama dengan
// metrik media di admin — RapidAPI "Bulk DA PA Checker Semrush", KSDeveloper).
func (d *Deps) fetchSeoMetrics(r *http.Request, domain, apiKey string) (map[string]any, *PartnerError) {
	cfg := seo.LoadConfig(r.Context(), d.Pool)
	metrics, seoErr := seo.Fetch(r.Context(), domain, apiKey, cfg)
	if seoErr != nil {
		switch seoErr.Kind {
		case seo.KindAuth:
			slog.Error("[seo] provider menolak API key", "status", seoErr.Status)
			return nil, &PartnerError{http.StatusServiceUnavailable, "Konfigurasi integrasi SEO bermasalah. Hubungi admin Tauthub."}
		case seo.KindQuota, seo.KindRateLimited:
			return nil, &PartnerError{http.StatusServiceUnavailable, "Provider SEO sedang rate-limited. Coba lagi nanti."}
		case seo.KindNotFound:
			return nil, &PartnerError{http.StatusServiceUnavailable, "Domain media tidak ditemukan provider."}
		case seo.KindMalformed:
			return nil, &PartnerError{http.StatusBadGateway, seo.ErrText(seoErr)}
		default:
			return nil, &PartnerError{http.StatusServiceUnavailable, "Provider SEO sedang tidak tersedia."}
		}
	}
	out := map[string]any{"domain": metrics.Domain, "da": metrics.DA, "pa": metrics.PA, "spam_score": nil}
	if metrics.Spam != nil {
		out["spam_score"] = *metrics.Spam
	}
	return out, nil
}

// fetchSpamScore: provider spam checker opsional (timeout 10s).
func (d *Deps) fetchSpamScore(r *http.Request, domain, apiKey string) (*float64, error) {
	target := "https://website-spam-checker.p.rapidapi.com/api/check?domain=" + urlQueryEscape(domain)
	headers := map[string]string{
		"accept":          "application/json",
		"x-rapidapi-host": "website-spam-checker.p.rapidapi.com",
		"x-rapidapi-key":  apiKey,
	}
	status, body, _ := d.safeGetWithHeader(r, target, headers, 10*time.Second)
	if status < 200 || status >= 300 {
		return nil, nil
	}
	var parsed any
	if jsonUnmarshal(body, &parsed) != nil {
		return nil, nil
	}
	return findMetric(parsed, "spam", 0), nil
}

// findMetric: cari nilai metrik di kedalaman ≤6, cocokkan nama kunci
// (case-insensitive), longgar terhadap string ("DA 42" → 42), clamp 0..100.
func findMetric(node any, key string, depth int) *float64 {
	if depth > 6 {
		return nil
	}
	switch t := node.(type) {
	case map[string]any:
		for k, v := range t {
			if strings.EqualFold(k, key) {
				if f := parseMetricValue(v); f != nil {
					return clamp100(*f)
				}
			}
		}
		for _, v := range t {
			if f := findMetric(v, key, depth+1); f != nil {
				return f
			}
		}
	case []any:
		for _, v := range t {
			if f := findMetric(v, key, depth+1); f != nil {
				return f
			}
		}
	}
	return nil
}

var metricNumberRe = regexpMustCompile(`(\d+(?:\.\d+)?)`)

func parseMetricValue(v any) *float64 {
	switch t := v.(type) {
	case float64:
		return &t
	case string:
		m := metricNumberRe.FindStringSubmatch(t)
		if len(m) < 2 {
			return nil
		}
		f, err := parseFloat(m[1])
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}

func clamp100(v float64) *float64 {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	v = float64(int(v + 0.5))
	return &v
}

// ---------- SEO: speed audit (port seo/speed-audit) ----------

const (
	speedAuditCacheTTL = 60 * time.Minute
	speedAuditCooldown = 60 * time.Second
)

type speedRateKey struct {
	at time.Time
}

var speedRateMu syncMutex
var speedRateMap = map[string]speedRateKey{}

func (d *Deps) SpeedAudit(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		d.speedAuditGet(w, r, partner)
		return
	}
	body := decodeJSON(r)
	rawURL, _ := strField(body, "url")
	strategy := "mobile"
	if s, present := strField(body, "strategy"); present && s == "desktop" {
		strategy = "desktop"
	}
	force, _ := boolField(body, "force")
	normalized, domain, normErr := normalizeSpeedURL(rawURL)
	if normErr != nil {
		httpx.Error(w, normErr.Status, normErr.Message)
		return
	}
	allowed, err := d.partnerMediaHosts(r, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if len(allowed) > 0 && !allowed[domain] {
		httpx.Error(w, http.StatusForbidden,
			"Domain "+domain+" bukan milik media Anda. Pilih dari daftar media Anda.")
		return
	}
	if !force {
		cached, _ := d.queryRow(r.Context(), `
			SELECT * FROM partner_speed_audits WHERE partner_id = $1 AND normalized_url = $2 AND strategy = $3
			ORDER BY created_at DESC LIMIT 1`, partner.ID, normalized, strategy)
		if cached != nil {
			if created, okTime := cached.raw("created_at").(time.Time); okTime && time.Since(created) < speedAuditCacheTTL {
				writeJSON(w, http.StatusOK, speedAuditJSON(cached, true))
				return
			}
		}
	}
	rateKey := partner.ID + "|" + strategy + "|" + normalized
	speedRateMu.Lock()
	entry, seen := speedRateMap[rateKey]
	if seen {
		elapsed := time.Since(entry.at)
		if elapsed < speedAuditCooldown {
			waitSec := int((speedAuditCooldown - elapsed).Seconds()) + 1
			speedRateMu.Unlock()
			httpx.Error(w, http.StatusTooManyRequests,
				"Audit "+strategy+" untuk URL ini baru saja dilakukan. Coba lagi dalam "+itoa(waitSec)+" detik.")
			return
		}
	}
	speedRateMap[rateKey] = speedRateKey{at: time.Now()}
	speedRateMu.Unlock()

	result, psErr := d.fetchPageSpeed(r, normalized, strategy)
	if psErr != nil {
		httpx.Error(w, psErr.Status, psErr.Message)
		return
	}
	stored, err := d.storeSpeedAudit(r, partner.ID, result)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, speedAuditJSON(stored, false))
}

func (d *Deps) speedAuditGet(w http.ResponseWriter, r *http.Request, partner PartnerRow) {
	q := r.URL.Query()
	rawURL := strings.TrimSpace(q.Get("url"))
	strategy := "mobile"
	if q.Get("strategy") == "desktop" {
		strategy = "desktop"
	}
	if rawURL != "" {
		normalized, _, normErr := normalizeSpeedURL(rawURL)
		if normErr != nil {
			httpx.Error(w, normErr.Status, normErr.Message)
			return
		}
		cached, _ := d.queryRow(r.Context(), `
			SELECT * FROM partner_speed_audits WHERE partner_id = $1 AND normalized_url = $2 AND strategy = $3
			ORDER BY created_at DESC LIMIT 1`, partner.ID, normalized, strategy)
		if cached == nil {
			httpx.Error(w, http.StatusNotFound, "Belum ada audit untuk URL ini.")
			return
		}
		writeJSON(w, http.StatusOK, speedAuditJSON(cached, true))
		return
	}
	limit := clampInt(q.Get("limit"), 10, 1, 50)
	rows, _ := d.queryRows(r.Context(),
		"SELECT * FROM partner_speed_audits WHERE partner_id = $1 ORDER BY created_at DESC LIMIT $2", partner.ID, limit)
	writeJSON(w, http.StatusOK, map[string]any{"items": rows})
}

func speedAuditJSON(row rowMap, cached bool) map[string]any {
	return map[string]any{
		"id":                   row.raw("id"),
		"url":                  row.raw("url"),
		"normalized_url":       row.raw("normalized_url"),
		"domain":               row.raw("domain"),
		"strategy":             row.raw("strategy"),
		"performance_score":    row.numPtr("performance_score"),
		"accessibility_score":  row.numPtr("accessibility_score"),
		"best_practices_score": row.numPtr("best_practices_score"),
		"seo_score":            row.numPtr("seo_score"),
		"lcp_ms":               row.numPtr("lcp_ms"),
		"inp_ms":               row.numPtr("inp_ms"),
		"cls":                  row.numPtr("cls"),
		"audits":               row.raw("audits"),
		"provider":             row.raw("provider"),
		"created_at":           row.timePtr("created_at"),
		"cached":               cached,
	}
}

// normalizeSpeedURL: parity normalizeSpeedUrl.
func normalizeSpeedURL(raw string) (normalized, domain string, err *PartnerError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", &PartnerError{http.StatusBadRequest, "Masukkan URL website untuk diperiksa."}
	}
	parsed, parseErr := urlParse(ensureScheme(value))
	if parseErr != nil || parsed.Host == "" {
		return "", "", &PartnerError{http.StatusBadRequest, "URL tidak valid. Contoh: https://example.com"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", &PartnerError{http.StatusBadRequest, "URL harus http atau https."}
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if host == "" || isUnsafeHost(host) || strings.Contains(host, "..") || !strings.Contains(host, ".") {
		return "", "", &PartnerError{http.StatusBadRequest, "Domain tidak dapat diperiksa."}
	}
	path := strings.TrimRight(parsed.Path, "/")
	normalized = "https://" + host + path + parsed.RawQuery
	if parsed.RawQuery != "" {
		normalized = "https://" + host + path + "?" + parsed.RawQuery
	}
	return normalized, host, nil
}

// partnerMediaHosts: kumpulan host website media mitra (untuk cek kepemilikan).
func (d *Deps) partnerMediaHosts(r *http.Request, partnerID string) (map[string]bool, error) {
	rows, err := d.queryRows(r.Context(),
		"SELECT website_url FROM media WHERE partner_id = $1 AND website_url IS NOT NULL AND status <> 'hidden' LIMIT 50",
		partnerID)
	if err != nil {
		return nil, nil
	}
	allowed := map[string]bool{}
	for _, row := range rows {
		host := extractHost(row.strOr("website_url", ""))
		host = strings.TrimPrefix(host, "www.")
		if host != "" {
			allowed[host] = true
		}
	}
	return allowed, nil
}

// fetchPageSpeed: GET PageSpeed Insights v5 (timeout 30s).
func (d *Deps) fetchPageSpeed(r *http.Request, target, strategy string) (map[string]any, *PartnerError) {
	apiKey, keyErr := d.pageSpeedAPIKey(r)
	if keyErr != nil {
		return nil, keyErr
	}
	url := "https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=" + urlQueryEscape(target) +
		"&strategy=" + urlQueryEscape(strategy) +
		"&category=performance&category=accessibility&category=best-practices&category=seo&key=" + urlQueryEscape(apiKey)
	status, body, _ := d.safeGetWithHeader(r, url, map[string]string{"accept": "application/json"}, 30*time.Second)
	if status == 0 {
		return nil, &PartnerError{http.StatusGatewayTimeout, "Gagal menghubungi PageSpeed API: koneksi gagal"}
	}
	if status < 200 || status >= 300 {
		switch {
		case status == 429:
			return nil, &PartnerError{http.StatusTooManyRequests, "PageSpeed API dibatasi (quota). Coba lagi beberapa menit."}
		case status >= 500:
			return nil, &PartnerError{http.StatusBadGateway, "PageSpeed API sedang gangguan. Coba lagi."}
		}
		detail := extractApiDetail(body)
		if strings.Contains(detail, "Lighthouse returned error") || strings.Contains(detail, "DNS") {
			return nil, &PartnerError{http.StatusBadRequest, "Website tidak dapat dianalisis. Pastikan URL dapat diakses secara publik."}
		}
		if detail == "" {
			detail = "Gagal menganalisis website."
		}
		return nil, &PartnerError{http.StatusBadGateway, truncateRunes(detail, 500)}
	}
	var parsed map[string]any
	if jsonUnmarshal(body, &parsed) != nil {
		return nil, &PartnerError{http.StatusBadGateway, "Gagal menganalisis website."}
	}
	light, ok := parsed["lighthouseResult"].(map[string]any)
	if !ok {
		return nil, &PartnerError{http.StatusBadGateway, "PageSpeed tidak mengembalikan hasil Lighthouse."}
	}
	categories, _ := light["categories"].(map[string]any)
	if len(categories) == 0 {
		return nil, &PartnerError{http.StatusBadGateway, "PageSpeed tidak mengembalikan hasil Lighthouse."}
	}

	result := map[string]any{
		"url":                  target,
		"normalized_url":       target,
		"domain":               extractHost(target),
		"strategy":             strategy,
		"performance_score":    categoryScore(categories, "performance"),
		"accessibility_score":  categoryScore(categories, "accessibility"),
		"best_practices_score": categoryScore(categories, "best-practices"),
		"seo_score":            categoryScore(categories, "seo"),
		"provider":             "pagespeed",
		"created_at":           isoMilli(nowUTC()),
	}

	audits, _ := light["audits"].(map[string]any)
	result["lcp_ms"] = auditNumeric(audits, []string{"largest-contentful-paint"})
	result["inp_ms"] = auditNumeric(audits, []string{
		"interaction-to-next-paint", "experimental-interaction-to-next-paint", "max-potential-fid",
	})
	result["cls"] = auditCLS(audits)
	result["audits"] = failingAudits(audits)
	return result, nil
}

func (d *Deps) pageSpeedAPIKey(r *http.Request) (string, *PartnerError) {
	row, _ := d.queryRow(r.Context(),
		"SELECT sealed_api_key FROM platform_pagespeed_config WHERE id = 'default' LIMIT 1")
	if row != nil {
		sealed := row.strOr("sealed_api_key", "")
		if sealed != "" {
			if plain, err := d.unsealT2(sealed); err == nil && plain != "" {
				return plain, nil
			} else if err != nil {
				slog.Error("[pagespeed] API key tersimpan gagal di-unseal (APP_SECRET berubah?)",
					"source", "platform_pagespeed_config", "err", err)
			}
		}
	}
	if envKey := strings.TrimSpace(envValue("GOOGLE_PAGESPEED_API_KEY")); envKey != "" {
		return envKey, nil
	}
	slog.Error("[pagespeed] API key belum dikonfigurasi", "source", "platform_pagespeed_config")
	return "", &PartnerError{http.StatusServiceUnavailable,
		"Konfigurasi integrasi Pagespeed belum siap. Hubungi admin Tauthub."}
}

func categoryScore(categories map[string]any, key string) any {
	entry, ok := categories[key].(map[string]any)
	if !ok {
		return nil
	}
	score, ok := entry["score"].(float64)
	if !ok {
		return nil
	}
	return int(score*100 + 0.5)
}

func auditNumeric(audits map[string]any, keys []string) any {
	for _, key := range keys {
		entry, ok := audits[key].(map[string]any)
		if !ok {
			continue
		}
		if v, isNum := entry["numericValue"].(float64); isNum {
			return int(v + 0.5)
		}
	}
	return nil
}

func auditCLS(audits map[string]any) any {
	entry, ok := audits["cumulative-layout-shift"].(map[string]any)
	if !ok {
		return nil
	}
	v, isNum := entry["numericValue"].(float64)
	if !isNum {
		return nil
	}
	return round3(v)
}

func round3(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}

// failingAudits: rekomendasi gagal (skor < 0.9), maksimal 20, urut naik.
func failingAudits(audits map[string]any) []map[string]any {
	out := []map[string]any{}
	for id, raw := range audits {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		title, hasTitle := entry["title"].(string)
		if !hasTitle || title == "" {
			continue
		}
		var scoreVal any
		if score, isNum := entry["score"].(float64); isNum {
			if score >= 0.9 {
				continue
			}
			scoreVal = score
		}
		displayValue, hasDisplay := entry["displayValue"].(string)
		if scoreVal == nil && (!hasDisplay || displayValue == "") {
			continue
		}
		item := map[string]any{"id": id, "title": title, "score": scoreVal}
		if hasDisplay {
			item["displayValue"] = displayValue
		}
		if desc, isString := entry["description"].(string); isString {
			item["description"] = truncateRunes(desc, 300)
		}
		out = append(out, item)
		if len(out) >= 20 {
			break
		}
	}
	sortAuditsByScore(out)
	return out
}

func sortAuditsByScore(items []map[string]any) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			if auditScore(items[j]) < auditScore(items[j-1]) {
				items[j], items[j-1] = items[j-1], items[j]
				continue
			}
			break
		}
	}
}

func auditScore(item map[string]any) float64 {
	if v, ok := item["score"].(float64); ok {
		return v
	}
	return 1
}

func (d *Deps) storeSpeedAudit(r *http.Request, partnerID string, result map[string]any) (rowMap, error) {
	id := uuid.NewString()
	auditsJSON := marshalJSON(result["audits"])
	if err := d.exec(r.Context(), `
		INSERT INTO partner_speed_audits (id, partner_id, url, normalized_url, domain, strategy, performance_score, accessibility_score, best_practices_score, seo_score, lcp_ms, inp_ms, cls, audits, provider, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15, NOW(), NOW())`,
		id, partnerID, result["url"], result["normalized_url"], result["domain"], result["strategy"],
		result["performance_score"], result["accessibility_score"], result["best_practices_score"], result["seo_score"],
		result["lcp_ms"], result["inp_ms"], result["cls"], auditsJSON, result["provider"]); err != nil {
		return nil, err
	}
	return d.queryRow(r.Context(), "SELECT * FROM partner_speed_audits WHERE id = $1 LIMIT 1", id)
}

// ---------- GSC (port gsc/*) ----------

func (d *Deps) GSC(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "Gunakan GET /v1/partner/gsc/connect untuk memulai OAuth.")
		return
	}
	connections, err := d.queryRows(r.Context(),
		"SELECT * FROM gsc_connections WHERE partner_id = $1 ORDER BY created_at ASC", partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if len(connections) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"connected": false, "connections": []any{}, "properties": []any{}, "snapshots": []any{},
		})
		return
	}
	connPayload := make([]map[string]any, 0, len(connections))
	allProperties := []map[string]any{}
	allSnapshots := []map[string]any{}
	connected := false
	// Properti + snapshot seluruh koneksi diambil sekaligus lalu dikelompokkan
	// (sebelumnya 2 query per koneksi). Error tetap diabaikan seperti semula:
	// data pendukung di halaman ini best-effort.
	connIDs := make([]string, 0, len(connections))
	for _, conn := range connections {
		connIDs = append(connIDs, conn.strOr("id", ""))
	}
	propsByConn := map[string][]rowMap{}
	propRows, _ := d.queryRows(r.Context(),
		"SELECT id, site_url, permission_level, connection_id FROM gsc_properties WHERE connection_id = ANY($1) ORDER BY site_url ASC",
		connIDs)
	for _, p := range propRows {
		key := p.strOr("connection_id", "")
		propsByConn[key] = append(propsByConn[key], p)
	}
	// Batas 10 snapshot per koneksi dipertahankan (dulu LIMIT 10 per query).
	snapsByConn := map[string][]rowMap{}
	snapRows, _ := d.queryRows(r.Context(), `
		SELECT s.*, pr.connection_id AS snap_connection_id
		FROM gsc_snapshots s
		JOIN gsc_properties pr ON pr.id = s.property_id
		WHERE pr.connection_id = ANY($1)
		ORDER BY s.fetched_at DESC`, connIDs)
	for _, s := range snapRows {
		key := s.strOr("snap_connection_id", "")
		if len(snapsByConn[key]) >= 10 {
			continue
		}
		// Kolom bantu pengelompokan dibuang supaya bentuk payload tidak berubah.
		delete(s, "snap_connection_id")
		snapsByConn[key] = append(snapsByConn[key], s)
	}
	for _, conn := range connections {
		props := propsByConn[conn.strOr("id", "")]
		snapshots := snapsByConn[conn.strOr("id", "")]
		propItems := make([]map[string]any, 0, len(props))
		for _, p := range props {
			item := map[string]any{
				"id":               p.raw("id"),
				"site_url":         p.raw("site_url"),
				"permission_level": p.raw("permission_level"),
				"connection_id":    p.raw("connection_id"),
			}
			propItems = append(propItems, item)
			allProperties = append(allProperties, item)
		}
		for _, s := range snapshots {
			allSnapshots = append(allSnapshots, s)
		}
		status := conn.strOr("status", "")
		if status == "active" {
			connected = true
		}
		connPayload = append(connPayload, map[string]any{
			"id":                   conn.raw("id"),
			"email":                conn.raw("email"),
			"status":               conn.raw("status"),
			"selected_property_id": conn.raw("selected_property_id"),
			"properties":           propItems,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connected": connected, "connections": connPayload,
		"properties": allProperties, "snapshots": allSnapshots,
	})
}

func (d *Deps) GSCConnect(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	cfg, cfgErr := d.loadGSCPlatformConfig(r)
	if cfgErr != nil {
		httpx.Error(w, cfgErr.Status, cfgErr.Message)
		return
	}
	state := randomBase64URL(16)
	verifier := randomBase64URL(32)
	challenge := base64URLSha256(verifier)
	if err := d.exec(r.Context(),
		"DELETE FROM gsc_oauth_states WHERE partner_id = $1 OR created_at < NOW() - INTERVAL '30 minutes'", partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"INSERT INTO gsc_oauth_states (id, partner_id, state, code_verifier) VALUES (gen_random_uuid(), $1, $2, $3)",
		partner.ID, state, verifier); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": buildGSCAuthURL(cfg.ClientID, cfg.RedirectURI, state, challenge)})
}

type gscConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

func (d *Deps) loadGSCPlatformConfig(r *http.Request) (gscConfig, *PartnerError) {
	row, _ := d.queryRow(r.Context(),
		"SELECT client_id, sealed_client_secret, redirect_uri FROM platform_gsc_config WHERE id = 'default' LIMIT 1")
	cfg := gscConfig{}
	decryptFailed := false
	if row != nil {
		cfg.ClientID = strings.TrimSpace(row.strOr("client_id", ""))
		cfg.RedirectURI = strings.TrimSpace(row.strOr("redirect_uri", ""))
		if sealed := row.strOr("sealed_client_secret", ""); sealed != "" {
			if plain, err := d.unsealT2(sealed); err == nil {
				cfg.ClientSecret = plain
			} else {
				decryptFailed = true
			}
		}
	}
	if cfg.ClientID == "" {
		cfg.ClientID = strings.TrimSpace(envValue("GOOGLE_CLIENT_ID"))
	}
	if cfg.ClientSecret == "" {
		cfg.ClientSecret = strings.TrimSpace(envValue("GOOGLE_CLIENT_SECRET"))
	}
	if cfg.RedirectURI == "" {
		base := strings.TrimRight(strings.TrimSpace(d.Cfg.PublicAppURL), "/")
		if base == "" {
			slog.Error("[gsc] redirect_uri kosong dan PublicAppURL tidak diatur",
				"source", "platform_gsc_config", "missing", "redirect_uri")
			return gscConfig{}, &PartnerError{http.StatusServiceUnavailable,
				"Konfigurasi integrasi Google belum siap. Hubungi admin Tauthub."}
		}
		cfg.RedirectURI = base + "/api/gsc/callback"
	}
	if decryptFailed {
		slog.Error("[gsc] sealed_client_secret gagal di-unseal (APP_SECRET berubah?)",
			"source", "platform_gsc_config", "config_error", "secret_decrypt_failed")
		return gscConfig{}, &PartnerError{http.StatusServiceUnavailable,
			"Kredensial tersimpan tidak dapat dibuka. Hubungi admin Tauthub untuk menyimpan ulang."}
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		missing := []string{}
		if cfg.ClientID == "" {
			missing = append(missing, "client_id")
		}
		if cfg.ClientSecret == "" {
			missing = append(missing, "client_secret")
		}
		slog.Error("[gsc] konfigurasi Google OAuth belum lengkap",
			"source", "platform_gsc_config", "missing", strings.Join(missing, ","))
		return gscConfig{}, &PartnerError{http.StatusServiceUnavailable,
			"Konfigurasi integrasi Google belum siap. Hubungi admin Tauthub."}
	}
	return cfg, nil
}

func (d *Deps) GSCProperties(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	conn, err := d.queryRow(r.Context(),
		"SELECT * FROM gsc_connections WHERE partner_id = $1 LIMIT 1", partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if r.Method == http.MethodGet {
		if conn == nil {
			writeJSON(w, http.StatusOK, map[string]any{"properties": []any{}, "selected_property_id": nil})
			return
		}
		props, _ := d.queryRows(r.Context(),
			"SELECT id, site_url, permission_level FROM gsc_properties WHERE connection_id = $1 ORDER BY site_url ASC",
			conn.raw("id"))
		items := make([]map[string]any, 0, len(props))
		for _, p := range props {
			items = append(items, map[string]any{
				"id": p.raw("id"), "site_url": p.raw("site_url"), "permission_level": p.raw("permission_level"),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"properties": items, "selected_property_id": conn.raw("selected_property_id"),
		})
		return
	}
	if conn == nil {
		httpx.Error(w, http.StatusConflict, "Belum terhubung GSC.")
		return
	}
	body := decodeJSON(r)
	propertyID := strings.TrimSpace(stringOrEmpty(body["property_id"]))
	if propertyID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "property_id wajib")
		return
	}
	property, err := d.queryRow(r.Context(),
		"SELECT id FROM gsc_properties WHERE id = $1 AND connection_id = $2 LIMIT 1", propertyID, conn.raw("id"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if property == nil {
		httpx.Error(w, http.StatusNotFound, "Property tidak ditemukan untuk koneksi ini.")
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE gsc_connections SET selected_property_id = $1, updated_at = NOW() WHERE id = $2", propertyID, conn.raw("id")); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "selected_property_id": propertyID})
}

func (d *Deps) GSCSnapshots(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	conn, err := d.queryRow(r.Context(),
		"SELECT * FROM gsc_connections WHERE partner_id = $1 LIMIT 1", partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if conn == nil || conn.strOr("status", "") != "active" {
		httpx.Error(w, http.StatusConflict, "GSC belum terhubung. Silakan Hubungkan GSC (OAuth) terlebih dahulu.")
		return
	}
	if !conn.has("sealed_refresh_token") {
		httpx.Error(w, http.StatusConflict, "Refresh token tidak tersedia. Silakan hubungkan ulang.")
		return
	}
	refreshToken, unsealErr := d.unsealT2(conn.strOr("sealed_refresh_token", ""))
	if unsealErr != nil {
		slog.Error("[gsc] refresh token mitra gagal di-unseal (APP_SECRET berubah?)",
			"partner_id", partner.ID, "err", unsealErr)
		_ = d.exec(r.Context(), "UPDATE gsc_connections SET status = 'error', updated_at = NOW() WHERE id = $1", conn.raw("id"))
		httpx.Error(w, http.StatusInternalServerError,
			"Kredensial tersimpan tidak dapat dibuka. Hubungkan ulang Google Search Console atau hubungi admin Tauthub.")
		return
	}
	cfg, cfgErr := d.loadGSCPlatformConfig(r)
	if cfgErr != nil {
		httpx.Error(w, cfgErr.Status, cfgErr.Message)
		return
	}
	accessToken, refreshErr := d.refreshGSCToken(r, refreshToken, cfg.ClientID, cfg.ClientSecret)
	if refreshErr != nil {
		_ = d.exec(r.Context(), "UPDATE gsc_connections SET status = 'error', updated_at = NOW() WHERE id = $1", conn.raw("id"))
		message := refreshErr.Error()
		if message == "" {
			message = "Refresh token gagal"
		}
		httpx.Error(w, http.StatusUnauthorized, "Gagal refresh token: "+message+". Silakan hubungkan ulang.")
		return
	}
	body := decodeJSON(r)
	periodDays := 30
	if v, present := numField(body, "period_days"); present {
		switch int(v) {
		case 90, 7:
			periodDays = int(v)
		default:
			periodDays = 30
		}
	}
	propertyID, _ := strField(body, "property_id")
	if propertyID == "" {
		propertyID = conn.strOr("selected_property_id", "")
	}
	if propertyID == "" {
		first, _ := d.queryRow(r.Context(),
			"SELECT id FROM gsc_properties WHERE connection_id = $1 LIMIT 1", conn.raw("id"))
		propertyID = first.strOr("id", "")
	}
	if propertyID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pilih property GSC terlebih dahulu.")
		return
	}
	property, err := d.queryRow(r.Context(),
		"SELECT site_url FROM gsc_properties WHERE id = $1 AND connection_id = $2 LIMIT 1", propertyID, conn.raw("id"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if property == nil {
		httpx.Error(w, http.StatusNotFound, "Property tidak ditemukan.")
		return
	}
	siteURL := property.strOr("site_url", "")

	var mediaID any
	if v, present := body["media_id"]; present && v != nil {
		if s, isString := v.(string); isString && s != "" {
			mediaID = s
			media, _ := d.queryRow(r.Context(),
				"SELECT website_url FROM media WHERE id = $1 AND partner_id = $2 LIMIT 1", s, partner.ID)
			if media != nil && !isGSCDomainMatch(siteURL, media.strOr("website_url", "")) {
				httpx.Error(w, http.StatusUnprocessableEntity, "Property GSC tidak cocok dengan domain media tersebut.")
				return
			}
		}
	}
	endDate := time.Now().UTC()
	startDate := endDate.AddDate(0, 0, -periodDays)
	analytics, analyticsErr := d.fetchSearchAnalytics(r, accessToken, siteURL,
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if analyticsErr != nil {
		httpx.Error(w, http.StatusBadGateway, analyticsErr.Error())
		return
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(), `
		INSERT INTO gsc_snapshots (id, property_id, media_id, period_days, clicks, impressions, ctr, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, propertyID, mediaID, periodDays, analytics["clicks"], analytics["impressions"],
		analytics["ctr"], analytics["position"]); err != nil {
		d.writeErr(w, err)
		return
	}
	payload := map[string]any{
		"ok": true, "snapshot_id": id, "site_url": siteURL, "period_days": periodDays,
	}
	for k, v := range analytics {
		payload[k] = v
	}
	writeJSON(w, http.StatusOK, payload)
}

// isGSCDomainMatch: parity isGscDomainMatch — hostname sama atau subdomain.
func isGSCDomainMatch(siteURL, websiteURL string) bool {
	if strings.TrimSpace(websiteURL) == "" {
		return true
	}
	propertyHost := hostnameOf(siteURL)
	mediaHost := hostnameOf(websiteURL)
	if propertyHost == "" || mediaHost == "" {
		return true
	}
	if propertyHost == mediaHost {
		return true
	}
	return strings.HasSuffix(mediaHost, "."+propertyHost) || strings.HasSuffix(propertyHost, "."+mediaHost)
}

func hostnameOf(raw string) string {
	parsed, err := urlParse(ensureScheme(raw))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}

// refreshGSCToken: POST oauth2.googleapis.com/token (grant refresh_token).
func (d *Deps) refreshGSCToken(r *http.Request, refreshToken, clientID, clientSecret string) (string, error) {
	form := "refresh_token=" + urlQueryEscape(refreshToken) +
		"&client_id=" + urlQueryEscape(clientID) +
		"&client_secret=" + urlQueryEscape(clientSecret) +
		"&grant_type=refresh_token"
	status, body, _ := d.postForm(r, "https://oauth2.googleapis.com/token", form, 20*time.Second)
	if status < 200 || status >= 300 {
		return "", errAccessToken
	}
	var parsed map[string]any
	if jsonUnmarshal(body, &parsed) != nil {
		return "", errAccessToken
	}
	token, _ := parsed["access_token"].(string)
	if token == "" {
		return "", errAccessToken
	}
	return token, nil
}

var errAccessToken = &PartnerError{http.StatusUnauthorized, "Refresh token gagal"}

// fetchSearchAnalytics: POST webmasters/v3 searchAnalytics/query.
func (d *Deps) fetchSearchAnalytics(r *http.Request, accessToken, siteURL, startDate, endDate string) (map[string]any, error) {
	target := "https://www.googleapis.com/webmasters/v3/sites/" + urlPathEscape(siteURL) + "/searchAnalytics/query"
	payload := marshalJSON(map[string]any{"startDate": startDate, "endDate": endDate})
	status, body, _ := d.postJSON(r, target, payload, map[string]string{
		"authorization": "Bearer " + accessToken,
	}, 30*time.Second)
	if status < 200 || status >= 300 {
		detail := extractApiDetail(body)
		if detail == "" {
			detail = "Gagal mengambil data Search Console"
		}
		return nil, &PartnerError{http.StatusBadGateway, detail}
	}
	var parsed map[string]any
	if jsonUnmarshal(body, &parsed) != nil {
		return nil, &PartnerError{http.StatusBadGateway, "Gagal mengambil data Search Console"}
	}
	rows, _ := parsed["rows"].([]any)
	if len(rows) == 0 {
		return map[string]any{"clicks": 0, "impressions": 0, "ctr": 0.0, "position": 0.0}, nil
	}
	first, _ := rows[0].(map[string]any)
	clicks, _ := first["clicks"].(float64)
	impressions, _ := first["impressions"].(float64)
	ctr, _ := first["ctr"].(float64)
	position, _ := first["position"].(float64)
	return map[string]any{
		"clicks":      int(clicks + 0.5),
		"impressions": int(impressions + 0.5),
		"ctr":         ctr,
		"position":    position,
	}, nil
}

// ---------- CMS connections (port cms-connections/*) ----------

func (d *Deps) CMSConnections(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		if status != "" && status != "active" && status != "revoked" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid connection status filter")
			return
		}
		sql := cmsConnectionColumns + " WHERE partner_id = $1"
		args := []any{partner.ID}
		if status != "" {
			sql += " AND status = $2"
			args = append(args, status)
		}
		sql += " ORDER BY created_at DESC"
		rows, err := d.queryRows(r.Context(), sql, args...)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		connections := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			connections = append(connections, cmsConnectionJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"connections": connections, "total": len(connections)})
		return
	}

	body := decodeJSON(r)
	name, _ := strField(body, "name")
	provider, hasProvider := strField(body, "provider")
	if !hasProvider {
		provider = "wordpress"
	}
	baseURL, _ := strField(body, "base_url")
	username, hasUsername := strField(body, "username")
	appPassword, hasAppPassword := strField(body, "application_password")
	secret, hasSecret := strField(body, "secret")
	websiteID, hasWebsiteID := strField(body, "website_id")

	if len(name) < 2 || len(name) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid name")
		return
	}
	if provider != "wordpress" && provider != "webhook" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid provider")
		return
	}
	if len(baseURL) < 8 || len(baseURL) > 500 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid base_url")
		return
	}
	if provider == "wordpress" {
		if !hasUsername || username == "" {
			httpx.Error(w, http.StatusUnprocessableEntity, "username is required for WordPress connections")
			return
		}
		if !hasAppPassword || appPassword == "" {
			httpx.Error(w, http.StatusUnprocessableEntity, "application_password is required for WordPress connections")
			return
		}
	} else if !hasSecret || len(secret) < 16 || len(secret) > 500 {
		httpx.Error(w, http.StatusUnprocessableEntity, "secret is required for webhook connections")
		return
	}
	cleanBaseURL := strings.TrimRight(baseURL, "/")

	dupSQL := ""
	dupArgs := []any{partner.ID}
	if hasWebsiteID && websiteID != "" {
		dupSQL = "partner_id = $1 AND website_id = $2 AND provider = 'wordpress' AND status = 'active'"
		dupArgs = append(dupArgs, websiteID)
	} else {
		dupSQL = "partner_id = $1 AND base_url = $2 AND provider = 'wordpress' AND status = 'active'"
		dupArgs = append(dupArgs, cleanBaseURL)
	}
	existing, err := d.queryRow(r.Context(),
		"SELECT id FROM cms_connections WHERE "+dupSQL+" LIMIT 1", dupArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if existing != nil {
		httpx.Error(w, http.StatusConflict, "Koneksi WordPress untuk website atau URL ini sudah aktif")
		return
	}

	connectionID := uuid.NewString()
	plainSecret := secret
	if provider == "wordpress" {
		plainSecret = regexpWhitespace.ReplaceAllString(appPassword, "")
	}
	sealed, err := d.sealV3ForEntity(plainSecret, connectionID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	var usernameVal any
	if hasUsername && username != "" {
		usernameVal = username
	}
	if err := d.exec(r.Context(), `
		INSERT INTO cms_connections (id, brand_id, partner_id, website_id, provider, name, base_url, username, sealed_secret, enc_key_ver, status, created_by)
		VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, 1, 'active', $9)`,
		connectionID, partner.ID, nullIfEmpty(websiteID), provider, name, cleanBaseURL, usernameVal, sealed, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "cms.connection.created", "cms_connection", connectionID)
	created, _ := d.queryRow(r.Context(), cmsConnectionColumns+" WHERE id = $1 LIMIT 1", connectionID)
	writeJSON(w, http.StatusCreated, cmsConnectionJSON(created))
}

const cmsConnectionColumns = `SELECT id, brand_id, partner_id, website_id, provider, name, base_url, username, sealed_secret, enc_key_ver, status, last_test_status, last_test_message, created_at, updated_at FROM cms_connections`

func cmsConnectionJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":                row.raw("id"),
		"brand_id":          row.raw("brand_id"),
		"partner_id":        row.raw("partner_id"),
		"website_id":        row.raw("website_id"),
		"provider":          row.raw("provider"),
		"name":              row.raw("name"),
		"base_url":          row.raw("base_url"),
		"username":          row.raw("username"),
		"status":            row.raw("status"),
		"last_test_status":  row.raw("last_test_status"),
		"last_test_message": row.raw("last_test_message"),
		"created_at":        row.timePtr("created_at"),
		"updated_at":        row.timePtr("updated_at"),
	}
}

func (d *Deps) CMSConnectionUpdate(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	connectionID := chiParam(r, "connectionId")
	existing, err := d.queryRow(r.Context(),
		cmsConnectionColumns+" WHERE id = $1 AND partner_id = $2 LIMIT 1", connectionID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if existing == nil {
		d.writeErr(w, errNotFound("CMS connection not found"))
		return
	}
	if r.Method == http.MethodDelete {
		if r.URL.Query().Get("mode") == "delete" {
			if existing.strOr("status", "") != "revoked" {
				httpx.Error(w, http.StatusConflict, "Hanya koneksi berstatus revoked yang dapat dihapus.")
				return
			}
			if _, err := d.execRows(r.Context(),
				"DELETE FROM cms_connections WHERE id = $1 AND partner_id = $2", connectionID, partner.ID); err != nil {
				d.writeErr(w, err)
				return
			}
			d.auditLog(r.Context(), user.ID, "cms.connection.deleted", "cms_connection", connectionID)
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE cms_connections SET status = 'revoked', name = CONCAT(name, ' (revoked ', LEFT(id, 8), ')') WHERE id = $1 AND partner_id = $2",
			connectionID, partner.ID); err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLog(r.Context(), user.ID, "cms.connection.revoked", "cms_connection", connectionID)
		updated, _ := d.queryRow(r.Context(), cmsConnectionColumns+" WHERE id = $1 AND partner_id = $2 LIMIT 1", connectionID, partner.ID)
		writeJSON(w, http.StatusOK, cmsConnectionJSON(updated))
		return
	}

	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	baseURL, hasBaseURL := strField(body, "base_url")
	username, hasUsername := strField(body, "username")
	secret, hasSecret := strField(body, "secret")
	if hasName && (len(name) < 2 || len(name) > 120) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid name")
		return
	}
	if hasBaseURL && (len(baseURL) < 8 || len(baseURL) > 500) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid base_url")
		return
	}
	if hasUsername && (len(username) < 1 || len(username) > 160) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid username")
		return
	}
	if hasSecret && (len(secret) < 16 || len(secret) > 500) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid secret")
		return
	}
	sets := []string{"name = COALESCE($1, name)", "base_url = COALESCE($2, base_url)", "username = COALESCE($3, username)"}
	args := []any{nullIfUnset(hasName, name), nullIfUnset(hasBaseURL, strings.TrimRight(baseURL, "/")), nullIfUnset(hasUsername, username)}
	if hasSecret && secret != "" {
		sealed, err := d.sealV3ForEntity(secret, connectionID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		args = append(args, sealed)
		sets = append(sets, "sealed_secret = $"+itoa(len(args)), "enc_key_ver = 1")
	}
	args = append(args, connectionID, partner.ID)
	if err := d.exec(r.Context(),
		"UPDATE cms_connections SET "+strings.Join(sets, ", ")+" WHERE id = $"+itoa(len(args)-1)+" AND partner_id = $"+itoa(len(args)),
		args...); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "cms.connection.updated", "cms_connection", connectionID)
	updated, _ := d.queryRow(r.Context(), cmsConnectionColumns+" WHERE id = $1 AND partner_id = $2 LIMIT 1", connectionID, partner.ID)
	writeJSON(w, http.StatusOK, cmsConnectionJSON(updated))
}

func (d *Deps) CMSConnectionTest(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	connectionID := chiParam(r, "connectionId")
	conn, err := d.queryRow(r.Context(),
		cmsConnectionColumns+" WHERE id = $1 AND partner_id = $2 LIMIT 1", connectionID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if conn == nil {
		httpx.Error(w, http.StatusNotFound, "CMS connection not found")
		return
	}
	result := d.testCMSConnection(r, conn)
	if err := d.exec(r.Context(),
		"UPDATE cms_connections SET last_test_status = $1, last_test_message = $2 WHERE id = $3",
		result["status"], result["message"], connectionID); err != nil {
		d.writeErr(w, err)
		return
	}
	if result["status"] == "ok" {
		writeJSON(w, http.StatusOK, result)
		return
	}
	payload := map[string]any{}
	for k, v := range result {
		payload[k] = v
	}
	payload["detail"] = result["message"]
	writeJSON(w, http.StatusUnprocessableEntity, payload)
}

// testCMSConnection: parity testConnection (wordpress/webhook).
func (d *Deps) testCMSConnection(r *http.Request, conn rowMap) map[string]any {
	secret, secretErr := d.connectionSecret(conn)
	if secretErr != nil {
		return map[string]any{"status": "error", "message": truncateRunes(secretErr.Error(), 500)}
	}
	baseURL := conn.strOr("base_url", "")
	if conn.strOr("provider", "wordpress") == "webhook" {
		return d.testWebhook(r, baseURL, secret)
	}
	return d.testWordPress(r, baseURL, conn.strOr("username", ""), secret)
}

func (d *Deps) testWebhook(r *http.Request, baseURL, secret string) map[string]any {
	eventID := uuid.NewString()
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	raw := marshalJSON(map[string]any{"event": "content.ping", "event_id": eventID, "timestamp": timestamp})
	signature := "sha256=" + hmacSHA256Hex([]byte(secret), timestamp+"."+eventID+"."+raw)
	headers := map[string]string{
		"content-type":        "application/json",
		"x-tulisio-event":     "content.ping",
		"x-tulisio-event-id":  eventID,
		"x-tulisio-timestamp": timestamp,
		"x-tulisio-signature": signature,
	}
	status, body, _ := d.postJSON(r, baseURL, raw, headers, 20*time.Second)
	if status == 0 {
		return map[string]any{"status": "error", "message": "Connection failed"}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"status": "error",
			"message": truncateRunes("Webhook returned "+itoa(status)+": "+body, 500)}
	}
	var parsed map[string]any
	if jsonUnmarshal(body, &parsed) != nil || parsed["ok"] != true {
		return map[string]any{"status": "error",
			"message": truncateRunes("Webhook did not respond with ok:true ("+truncateRunes(body, 200)+")", 500)}
	}
	return map[string]any{"status": "ok", "message": "Webhook received ping and responded ok:true"}
}

func (d *Deps) testWordPress(r *http.Request, baseURL, username, secret string) map[string]any {
	target := wpURL(baseURL, "wp/v2/users/me?context=edit")
	resp, err := wpRequest(r.Context(), target, http.MethodGet, wpHeaders(username, secret), "", 20*time.Second)
	if err != nil {
		return map[string]any{"status": "error", "message": truncateRunes(describeWordPressNetErr(err, target), 500)}
	}
	if resp.status < 200 || resp.status >= 300 {
		return map[string]any{"status": "error",
			"message": truncateRunes("WordPress returned "+itoa(resp.status)+": "+resp.body, 500)}
	}
	var profile map[string]any
	if jsonUnmarshal(resp.body, &profile) != nil {
		return map[string]any{"status": "error", "message": "WordPress did not return a valid profile"}
	}
	capabilities, _ := profile["capabilities"].(map[string]any)
	if capabilities == nil || capabilities["edit_posts"] != true {
		label := firstStringField(profile, "slug", "name", "username")
		return map[string]any{"status": "error",
			"message": "WordPress authenticated as " + label + ", but edit_posts is denied"}
	}
	label := firstStringField(profile, "slug", "name", "username")
	return map[string]any{"status": "ok",
		"message": "Connected as " + label + " with post creation permission"}
}

func firstStringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return "unknown"
}
