package platform

// media_support.go: helper bersama modul media admin — serialisasi mediaJson,
// pricing engine, status/kategori, dewan pers, diagnostik domain, provider
// eksternal (RapidAPI/Pagespeed/SimilarWeb), health score, standard check, dan
// penggantian artikel (Pelacak 404). Semua simbol berprefiks `mp` agar tidak
// bentrok dengan modul platform lain di paket yang sama.
//
// Port dari app/v1/platform/media/_media.ts + lib/media/* + lib/pricing/engine.ts
// + lib/standard/* + lib/media/auto-replace.ts. Logika partner yang sama
// (mediaJson mitra, submitDewanPersVerification) sengaja DISALIN, bukan diimpor,
// supaya paket platform tidak membentuk siklus dengan paket partner.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
	"github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
	"github.com/rahmaangriawan/tauthub-go/internal/seo"
)

// ---------- konstanta media (parity _media.ts) ----------

var mpMediaStatuses = []string{
	"draft", "active", "inactive", "archived", "hidden", "pending_review", "rejected",
}

var mpActivePublicationStatuses = []string{
	"pending", "waiting_partner", "partner_approved", "queued", "processing", "retrying",
}

// mpArchiveSafeStatuses: satu-satunya status aktif yang aman diarsipkan massal.
var mpArchiveSafeStatuses = map[string]bool{"waiting_partner": true}

// mpArchiveBlockingStatuses: parity ARCHIVE_BLOCKING_STATUSES.
func mpArchiveBlockingStatuses() []string {
	out := []string{}
	for _, s := range mpActivePublicationStatuses {
		if !mpArchiveSafeStatuses[s] {
			out = append(out, s)
		}
	}
	return out
}

var mpMediaIntegrationTypes = []string{
	"manual", "wordpress", "rss", "webhook", "tulisio_api", "custom_web", "blogger", "emdash",
}

var mpMediaTiers = []string{"tier1", "tier2", "tier3"}

func mpContains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// mpZEnum: pesan zod v4 untuk invalid_value → `Invalid option: expected one of
// "a"|"b"` (parity util.joinValues + stringifyPrimitive).
func mpZEnum(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = `"` + v + `"`
	}
	return "Invalid option: expected one of " + strings.Join(quoted, "|")
}

func mpValidIntegrationType(v string) bool { return mpContains(mpMediaIntegrationTypes, v) }

var mpWordCountModes = map[string]bool{
	"sesuai_permintaan": true, "kata_300_700": true, "kata_800_1500": true,
}

const mpDefaultWordCountMode = "sesuai_permintaan"

// mpNormalizeWordCountMode: parity normalizeWordCountMode — nilai tak dikenal
// jatuh ke mode bawaan.
func mpNormalizeWordCountMode(v any) string {
	s, ok := v.(string)
	if !ok {
		return mpDefaultWordCountMode
	}
	trimmed := strings.TrimSpace(s)
	if mpWordCountModes[trimmed] {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "custom_") {
		parts := strings.Split(strings.TrimPrefix(trimmed, "custom_"), "_")
		if len(parts) == 2 {
			min, err1 := strconv.Atoi(parts[0])
			max, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && min > 0 && max >= min {
				return trimmed
			}
		}
	}
	return mpDefaultWordCountMode
}

// mpSlugifyMediaName: parity slugifyMediaName.
func mpSlugifyMediaName(name string) string {
	lower := strings.ToLower(name)
	var b strings.Builder
	lastDash := false
	for _, ch := range lower {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 160 {
		out = out[:160]
	}
	return out
}

// mpCategoriesArray: string_agg dengan pemisah E'\u0001' → daftar kategori,
// sekaligus memisah data lama yang menyimpan beberapa kategori dengan koma.
func mpCategoriesArray(row ptRow) []string {
	raw, _ := row.str("categories_csv")
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	return catutil.Split(raw)
}

// ---------- pricing engine (port lib/pricing/engine.ts) ----------

const mpDefaultMarkupPercent = 15.0

type mpMarkupCache struct {
	mu    sync.Mutex
	value float64
	exp   time.Time
	ok    bool
}

var mpMarkup = &mpMarkupCache{}

// mpGetMarkupPercent: pricing_config.markup_percent (singleton id=1), cache 30s.
func (d *Deps) mpGetMarkupPercent(ctx context.Context) float64 {
	mpMarkup.mu.Lock()
	if mpMarkup.ok && mpMarkup.exp.After(time.Now()) {
		v := mpMarkup.value
		mpMarkup.mu.Unlock()
		return v
	}
	mpMarkup.mu.Unlock()

	value := 0.0
	found := false
	if err := d.Pool.QueryRow(ctx, "SELECT markup_percent FROM pricing_config WHERE id = 1 LIMIT 1").Scan(&value); err == nil {
		found = value >= 0
	}
	if !found {
		value = mpDefaultMarkupPercent
	}
	mpMarkup.mu.Lock()
	mpMarkup.value, mpMarkup.exp, mpMarkup.ok = value, time.Now().Add(30*time.Second), true
	mpMarkup.mu.Unlock()
	return value
}

func mpRound2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// mpComputeClientPrice: parity computeClientPrice (half-up 2 desimal).
func mpComputeClientPrice(partnerPrice, markupPercent float64) float64 {
	base := math.Max(0, partnerPrice)
	markup := math.Max(0, markupPercent)
	return mpRound2(base * (1 + markup/100))
}

func mpMediaPriceFields(price, markupPercent float64) map[string]any {
	base := math.Max(0, price)
	markup := math.Max(0, markupPercent)
	client := mpComputeClientPrice(base, markup)
	return map[string]any{
		"markup_percent": markup,
		"markup_amount":  mpRound2(client - base),
		"client_price":   client,
	}
}

type mpRateCache struct {
	mu    sync.Mutex
	value float64
	exp   time.Time
	ok    bool
}

var mpRate = &mpRateCache{}

// mpGetRateIDR: tc_config.rate_idr (default 1000, minimum 1), cache 30s.
func (d *Deps) mpGetRateIDR(ctx context.Context) float64 {
	mpRate.mu.Lock()
	if mpRate.ok && mpRate.exp.After(time.Now()) {
		v := mpRate.value
		mpRate.mu.Unlock()
		return v
	}
	mpRate.mu.Unlock()

	rate := 0.0
	found := false
	if err := d.Pool.QueryRow(ctx, "SELECT rate_idr FROM tc_config WHERE id = 1 LIMIT 1").Scan(&rate); err == nil {
		found = rate >= 1
	}
	if !found {
		rate = 1000
	}
	mpRate.mu.Lock()
	mpRate.value, mpRate.exp, mpRate.ok = rate, time.Now().Add(30*time.Second), true
	mpRate.mu.Unlock()
	return rate
}

// mpCreditCostFor: credit_cost = ceil(clientPrice / rateIdr), min 1 untuk price>0.
func mpCreditCostFor(clientPrice, rateIDR float64) int {
	if clientPrice <= 0 || rateIDR <= 0 {
		return 0
	}
	cost := int(math.Ceil(clientPrice / rateIDR))
	if cost < 1 {
		cost = 1
	}
	return cost
}

// ---------- mediaJson (port mediaJson di _media.ts) ----------

// mpJSONValue: kolom jsonb datang sebagai string (lihat normalizeValue), jadi
// UI menerima teks JSON mentah dan `Array.isArray` selalu gagal. Parse dulu
// supaya bentuknya benar-benar array/objek di response.
func mpJSONValue(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out any
	if json.Unmarshal([]byte(s), &out) != nil {
		return nil
	}
	return out
}

func (d *Deps) mpMediaJSON(row ptRow, markupPercent float64) map[string]any {
	if row == nil {
		return nil
	}
	partnerPrice := row.numOr("price")
	categories := mpCategoriesArray(row)

	// Catatan: admin memakai kolom legacy `category` apa adanya (bukan kategori
	// pertama dari tabel link) — parity mediaJson di _media.ts.
	payload := map[string]any{
		"id":                     row.anyOrNil("id"),
		"name":                   row.anyOrNil("name"),
		"slug":                   row.anyOrNil("slug"),
		"public_slug":            row.anyOrNil("public_slug"),
		"public_listed":          row.boolOr("public_listed", true),
		"categories":             categories,
		"category":               row.anyOrNil("category"),
		"pending_changes":        mpJSONValue(row.anyOrNil("pending_changes")),
		"has_pending_values":     row.boolOr("has_pending_values", false),
		"pending_requested_at":   row.anyOrNil("pending_requested_at"),
		"description":            row.anyOrNil("description"),
		"website_url":            row.anyOrNil("website_url"),
		"contoh_artikel_url":     row.anyOrNil("contoh_artikel_url"),
		"adapter_url":            row.anyOrNil("adapter_url"),
		"price":                  partnerPrice,
		"credit_cost":            row.intOr("credit_cost", 1),
		"status":                 row.anyOrNil("status"),
		"pricing_type":           row.strOr("pricing_type", "nett"),
		"price_status":           row.anyOrNil("price_status"),
		"negotiated_price":       row.numPtr("negotiated_price"),
		"integration_type":       row.anyOrNil("integration_type"),
		"website_platform":       row.anyOrNil("website_platform"),
		"partner_hidden":         row.boolOr("partner_hidden", false),
		"partner_id":             row.anyOrNil("partner_id"),
		"partner_name":           row.anyOrNil("partner_name"),
		"partner_email":          row.anyOrNil("partner_email"),
		"domain_verified":        row.boolOr("domain_verified", false),
		"domain_verified_at":     row.anyOrNil("domain_verified_at"),
		"partner_wa_set":         mpPartnerWASet(row),
		"partner_wa_required":    row.intOr("partner_media_count", 0) > partnerWAThreshold && !mpPartnerWASet(row),
		"auto_publish":           row.boolOr("auto_publish", false),
		"referred_by_partner_id": row.anyOrNil("referred_by_partner_id"),
		"referrer_name":          row.anyOrNil("referrer_name"),
		"publish_defaults":       mpJSONValue(row.anyOrNil("publish_defaults")),
		"notes":                  row.anyOrNil("notes"),
		"traffic_estimate":       row.numOr("traffic_estimate"),
		"media_tier":             row.strOr("media_tier", "tier3"),
		"media_quality_score":    row.numPtr("media_quality_score"),
		"media_quality_notes":    row.anyOrNil("media_quality_notes"),
		"publication_mode":       row.strOr("publication_mode", "draft"),
		"word_count_mode":        mpNormalizeWordCountMode(row.anyOrNil("word_count_mode")),
		"link_duration_months":   row.intOr("link_duration_months", 12),
		"max_links":              row.intOr("max_links", 1),
		"allow_hyperlink":        row.boolOr("allow_hyperlink", true),
		"link_rel":               mpLinkRel(row),
		"label_sponsor":          row.strOr("label_sponsor", "tidak_ada"),
		"dewan_pers_verified":    row.boolOr("dewan_pers_verified", false),
		"dewan_pers_note":        row.anyOrNil("dewan_pers_note"),
		"dewan_pers_cert_no":     row.anyOrNil("dewan_pers_cert_no"),
		"dewan_pers_doc_key":     row.anyOrNil("dewan_pers_doc_key"),
		"rejection_reason":       row.anyOrNil("rejection_reason"),
		"reviewed_by":            row.anyOrNil("reviewed_by"),
		"reviewed_at":            row.anyOrNil("reviewed_at"),
		"health_score":           row.numPtr("health_score"),
		"health_breakdown":       row.anyOrNil("health_breakdown"),
		"health_computed_at":     row.anyOrNil("health_computed_at"),
		"created_by":             row.anyOrNil("created_by"),
		"created_at":             row.anyOrNil("created_at"),
		"updated_at":             row.anyOrNil("updated_at"),
		// Panel detail media: jumlah artikel yang sudah benar-benar terbit lewat
		// media ini. Dihitung di query daftar (published_count).
		"published_count": row.intOr("published_count", 0),
		// Status penangguhan. Dipisah dari `status` karena suspend adalah kendali
		// admin yang tidak boleh diubah mitra lewat halaman media mereka.
		"suspended_at":     row.anyOrNil("suspended_at"),
		"suspended_reason": row.anyOrNil("suspended_reason"),
		"is_suspended":     row.has("suspended_at"),
		// Tingkat pelanggaran aktif terakhir (none/ringan/berat), diisi subquery.
		"violation_level": row.strOr("violation_level", "none"),
	}
	for k, v := range mpMediaPriceFields(partnerPrice, markupPercent) {
		payload[k] = v
	}
	return payload
}

// ---------- dewan pers (port lib/media/dewan-pers.ts bagian status) ----------

var mpDewanPersCertRegex = regexp.MustCompile(`^[0-9]+/DP-Verifikasi/K/(I|II|III|IV|V|VI|VII|VIII|IX|X|XI|XII)/[0-9]{4}$`)

func mpValidDewanPersCert(cert string) bool {
	return mpDewanPersCertRegex.MatchString(strings.TrimSpace(cert))
}

// mpLatestDewanPersVerification: baris paling relevan (pending > verified > …).
func (d *Deps) mpLatestDewanPersVerification(ctx context.Context, mediaID string) ptRow {
	row, err := d.queryRow(ctx, `
		SELECT * FROM media_dewan_pers_verifications WHERE media_id = $1
		ORDER BY CASE status WHEN 'pending' THEN 0 WHEN 'verified' THEN 1 WHEN 'conflict' THEN 2 WHEN 'rejected' THEN 3 ELSE 4 END, submitted_at DESC
		LIMIT 1`, mediaID)
	if err != nil || row == nil {
		return nil
	}
	return ptRow(ptNormalizeMap(row))
}

// mpSyncDewanPersMediaColumns: sinkron kolom lama media.dewan_pers_*.
func (d *Deps) mpSyncDewanPersMediaColumns(ctx context.Context, mediaID string) {
	v := d.mpLatestDewanPersVerification(ctx, mediaID)
	if v == nil {
		return
	}
	if v.strOr("status", "") == "verified" {
		_ = d.exec(ctx,
			"UPDATE media SET dewan_pers_verified = TRUE, dewan_pers_cert_no = $1, dewan_pers_note = COALESCE($2, dewan_pers_note), updated_at = NOW() WHERE id = $3",
			v.strOr("certificate_no", ""), v.anyOrNil("rejection_reason"), mediaID)
		return
	}
	_ = d.exec(ctx, "UPDATE media SET dewan_pers_verified = FALSE, updated_at = NOW() WHERE id = $1", mediaID)
}

// ---------- domain helpers (port lib/media/seo-metrics.ts) ----------

func mpEnsureScheme(raw string) string {
	if strings.Contains(raw, "://") {
		return raw
	}
	return "https://" + raw
}

// mpIsUnsafeHost: localhost/.local/.internal + rentang IP privat.
func mpIsUnsafeHost(host string) bool {
	host = strings.ToLower(host)
	if host == "localhost" || host == "::1" {
		return true
	}
	if strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return true
	}
	parts := strings.Split(host, ".")
	if len(parts) == 4 {
		octets := make([]int, 4)
		valid := true
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				valid = false
				break
			}
			octets[i] = n
		}
		if valid {
			if octets[0] == 10 || octets[0] == 127 {
				return true
			}
			if octets[0] == 172 && octets[1] >= 16 && octets[1] <= 31 {
				return true
			}
			if octets[0] == 192 && octets[1] == 168 {
				return true
			}
		}
	}
	return false
}

// mpDomainFromWebsite: hostname lowercase tanpa www; nil bila tak layak.
func mpDomainFromWebsite(website any) any {
	raw, _ := website.(string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(mpEnsureScheme(raw))
	if err != nil {
		return nil
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host == "" || mpIsUnsafeHost(host) {
		return nil
	}
	return host
}

// ---------- SEO provider (RapidAPI Domain Metrics / spam / SERPens) ----------

type mpSeoError struct {
	Status  int
	Message string
}

func (e *mpSeoError) Error() string { return e.Message }

func mpSeoErr(status int, msg string) *mpSeoError { return &mpSeoError{status, msg} }

// mpPlatformSeoKey: baca + buka kolom tersegel di platform_seo_config.
func (d *Deps) mpPlatformSeoKey(ctx context.Context, column string) (string, *mpSeoError) {
	row, err := d.queryRow(ctx, "SELECT "+column+" FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil || row == nil {
		return "", mpSeoErr(http.StatusConflict, "RapidAPI SEO belum dikonfigurasi oleh Admin.")
	}
	sealed := row.strOr(column, "")
	if sealed == "" {
		return "", mpSeoErr(http.StatusConflict, "RapidAPI SEO belum dikonfigurasi oleh Admin.")
	}
	plain, uErr := d.unsealT2(sealed)
	if uErr != nil || plain == "" {
		return "", mpSeoErr(http.StatusConflict, "RapidAPI SEO key tidak dapat dibuka. Perbarui konfigurasi Admin.")
	}
	return plain, nil
}

// mpOptionalSeoKey: kolom opsional; kosong/gagal → "" tanpa error.
func (d *Deps) mpOptionalSeoKey(ctx context.Context, column string) string {
	row, err := d.queryRow(ctx, "SELECT "+column+" FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil || row == nil {
		return ""
	}
	sealed := row.strOr(column, "")
	if sealed == "" {
		return ""
	}
	plain, uErr := d.unsealT2(sealed)
	if uErr != nil {
		return ""
	}
	return plain
}

var mpMetricNumberRe = regexp.MustCompile(`(\d+(?:\.\d+)?)`)

// mpFindMetric: cari nilai metrik pada kedalaman ≤6, cocokkan nama kunci
// (case-insensitive), toleran string campuran, clamp 0..100.
func mpFindMetric(node any, key string, depth int) *float64 {
	if depth > 6 {
		return nil
	}
	switch t := node.(type) {
	case map[string]any:
		for k, v := range t {
			if strings.EqualFold(k, key) {
				if f := mpParseMetricValue(v); f != nil {
					return mpClamp100(*f)
				}
			}
		}
		for _, v := range t {
			if f := mpFindMetric(v, key, depth+1); f != nil {
				return f
			}
		}
	case []any:
		for _, v := range t {
			if f := mpFindMetric(v, key, depth+1); f != nil {
				return f
			}
		}
	}
	return nil
}

func mpParseMetricValue(v any) *float64 {
	switch t := v.(type) {
	case float64:
		return &t
	case string:
		m := mpMetricNumberRe.FindStringSubmatch(t)
		if len(m) < 2 {
			return nil
		}
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}

func mpClamp100(v float64) *float64 {
	v = math.Max(0, math.Min(100, v))
	v = math.Round(v)
	return &v
}

// Provider DA/PA sekarang "Bulk DA PA Checker Semrush" (RapidAPI, KSDeveloper).
// Kontrak & parsing dipusatkan di paket internal/seo supaya domain platform dan
// domain partner memakai sumber yang sama; host/path bisa ditimpa lewat
// platform_seo_config (lihat migrasi 279).

// mpRecordDAError: catat kegagalan penyedia DA/PA tingkat platform (langganan
// salah, kuota habis) supaya halaman admin bisa menjelaskan kenapa kolom DA
// kosong. Panggilan dengan msg nil membersihkan catatan setelah penyedia normal
// lagi. Best-effort: kegagalan menulis catatan tidak mengubah hasil pemeriksaan.
func (d *Deps) mpRecordDAError(ctx context.Context, msg *string) {
	var err error
	if msg == nil {
		_, err = d.Pool.Exec(ctx, `
			UPDATE platform_seo_config SET last_da_error = NULL, last_da_error_at = NULL
			WHERE id = 'default' AND last_da_error IS NOT NULL`)
	} else {
		_, err = d.Pool.Exec(ctx, `
			UPDATE platform_seo_config SET last_da_error = $1, last_da_error_at = NOW()
			WHERE id = 'default'`, *msg)
	}
	if err != nil {
		d.logWarn("catat status penyedia DA gagal", "", err)
	}
}

// mpDAProviderPaused: true bila penyedia DA/PA terakhir kali menolak (kuota habis
// atau langganan salah) dalam 24 jam terakhir. Panggilan penyedia dilewati selama
// jeda ini supaya pemeriksaan AI Trust tetap berjalan tanpa menghajar penyedia
// yang sudah menolak; menyimpan ulang key di Konfigurasi membersihkan catatannya
// sehingga percobaan berikutnya langsung jalan.
func (d *Deps) mpDAProviderPaused(ctx context.Context) bool {
	row, err := d.queryRow(ctx, `
		SELECT 1 AS paused FROM platform_seo_config
		WHERE id = 'default' AND last_da_error IS NOT NULL
		  AND last_da_error_at > NOW() - INTERVAL '24 hours'`)
	return err == nil && row != nil
}

// mpFetchSeoMetrics: cek DA/PA satu domain lewat paket internal/seo.
func (d *Deps) mpFetchSeoMetrics(ctx context.Context, domain, apiKey string) (map[string]any, *mpSeoError) {
	cfg := seo.LoadConfig(ctx, d.Pool)
	metrics, seoErr := seo.Fetch(ctx, domain, apiKey, cfg)
	if seoErr != nil {
		msg := seo.ErrText(seoErr)
		switch seoErr.Kind {
		case seo.KindAuth, seo.KindQuota, seo.KindRateLimited:
			d.mpRecordDAError(ctx, &msg)
		}
		status := http.StatusServiceUnavailable
		if seoErr.Kind == seo.KindMalformed {
			status = http.StatusBadGateway
		}
		return nil, mpSeoErr(status, msg)
	}
	// Penyedia normal lagi: bersihkan catatan kegagalan supaya peringatan di
	// halaman admin hilang sendiri.
	d.mpRecordDAError(ctx, nil)
	out := map[string]any{"domain": metrics.Domain, "da": metrics.DA, "pa": metrics.PA, "spam_score": nil}
	if metrics.Spam != nil {
		out["spam_score"] = *metrics.Spam
	}
	return out, nil
}

// mpFetchSpamScore: provider spam checker opsional (timeout 10s).
func (d *Deps) mpFetchSpamScore(ctx context.Context, domain, apiKey string) *float64 {
	target := "https://website-spam-checker.p.rapidapi.com/api/check?domain=" + url.QueryEscape(domain)
	status, body := mpSafeGet(ctx, target, map[string]string{
		"accept":          "application/json",
		"x-rapidapi-host": "website-spam-checker.p.rapidapi.com",
		"x-rapidapi-key":  apiKey,
	}, 10*time.Second)
	if status < 200 || status >= 300 {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil
	}
	return mpFindMetric(parsed, "spam", 0)
}

// mpComputeAiTrustScore: 0.65×performance + 25×schema + 10. Nil bila skor
// performance tidak benar-benar terukur — sebelumnya nilai default 50 dipakai
// saat pengukuran gagal, sehingga puluhan media menampilkan angka jatuhan
// (68 dengan schema, 43 tanpa) yang tampak seperti hasil pemeriksaan.
func (d *Deps) mpComputeAiTrustScore(ctx context.Context, websiteURL string) *float64 {
	target := mpEnsureScheme(websiteURL)
	// Sinyal schema dibaca dari halaman situs itu sendiri. Kalau halaman tidak
	// bisa dibaca dari IP server (banyak host mitra memblokir IP datacenter),
	// skor TETAP dihitung dari kecepatan saja: nilainya jadi batas bawah, bukan
	// dikosongkan. Sebelumnya kegagalan baca halaman membatalkan seluruh skor
	// sehingga 14 media tampak "belum berskor" padahal PageSpeed punya angkanya
	// (mis. jasarelease.co.id: skor kecepatan 79, tertahan hanya karena fetch kami
	// ditolak hostnya).
	hasSchema := false
	status, body := mpSafeGet(ctx, target, map[string]string{"accept": "text/html,application/xhtml+xml"}, 10*time.Second)
	if status >= 200 && status < 300 {
		lower := strings.ToLower(body)
		hasSchema = strings.Contains(lower, "application/ld+json") ||
			strings.Contains(lower, "schema.org") || strings.Contains(lower, "itemscope")
	}
	performance := d.mpPageSpeedPerformance(ctx, target)
	if performance == nil {
		return nil
	}
	score := 0.65*(*performance) + 10.0
	if hasSchema {
		score += 25
	}
	score = math.Max(0, math.Min(100, math.Round(score)))
	return &score
}

// mpPageSpeedPerformance: skor performance PageSpeed (nil bila key tidak ada,
// Lighthouse tidak mengembalikan skor, atau permintaan gagal).
//
// Respons Lighthouse bisa jauh di atas 1 MB karena berisi tangkapan layar.
// Pembacaan dibatasi 16 MB dan langsung di-decode ke struct kecil: pemotongan
// 512 KB (batas mpSafeGet) membuat JSON tidak valid, sehingga skor yang
// sebenarnya ada ikut hilang dan berujung pada angka default.
func (d *Deps) mpPageSpeedPerformance(ctx context.Context, target string) *float64 {
	apiKey := d.mpPageSpeedKey(ctx)
	if apiKey == "" {
		return nil
	}
	u := "https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=" + url.QueryEscape(target) +
		"&strategy=mobile&category=performance&key=" + url.QueryEscape(apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("accept", "application/json")
	// Situs berat bisa butuh lebih dari satu menit sampai PageSpeed membalas
	// (radarcianjur.com tercatat 1m22s). Job ini berjalan di latar, jadi menunggu
	// lebih lama lebih berguna daripada kehilangan skornya.
	resp, err := security.SafeClient(90 * time.Second).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	var parsed struct {
		LighthouseResult struct {
			Categories struct {
				Performance struct {
					Score *float64 `json:"score"`
				} `json:"performance"`
			} `json:"categories"`
		} `json:"lighthouseResult"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&parsed); err != nil {
		return nil
	}
	score := parsed.LighthouseResult.Categories.Performance.Score
	if score == nil {
		return nil
	}
	v := math.Round(*score * 100)
	return &v
}

// mpPageSpeedKey: platform_pagespeed_config lalu env GOOGLE_PAGESPEED_API_KEY.
func (d *Deps) mpPageSpeedKey(ctx context.Context) string {
	row, _ := d.queryRow(ctx, "SELECT sealed_api_key FROM platform_pagespeed_config WHERE id = 'default' LIMIT 1")
	if row != nil {
		if sealed := row.strOr("sealed_api_key", ""); sealed != "" {
			if plain, err := d.unsealT2(sealed); err == nil && plain != "" {
				return plain
			}
		}
	}
	return strings.TrimSpace(mpEnv("GOOGLE_PAGESPEED_API_KEY"))
}

// mpCheckAiCitation: AI Citation via SERPens; nil = key tidak dikonfigurasi.
type mpCitationResult struct {
	Cited     *bool
	CheckedAt string
	// Unavailable: panggilan ke SERPens gagal (bukan "Google tidak menampilkan
	// AI Overview"). Hasil sebelumnya tidak boleh dihapus dalam kasus ini.
	Unavailable bool
}

func (d *Deps) mpCheckAiCitation(ctx context.Context, websiteURL string) *mpCitationResult {
	key := d.mpOptionalSeoKey(ctx, "sealed_serpens_api_key")
	if key == "" {
		return nil
	}
	domain, _ := mpDomainFromWebsite(websiteURL).(string)
	if domain == "" {
		return nil
	}
	checkedAt := d.isoNow()
	target := "https://serpens.p.rapidapi.com/api/v1/search_ai_overview?q=" + url.QueryEscape(domain) + "&gl=id&hl=id"
	status, body := mpSafeGet(ctx, target, map[string]string{
		"accept":          "application/json",
		"x-rapidapi-host": "serpens.p.rapidapi.com",
		"x-rapidapi-key":  key,
	}, 20*time.Second)
	if status < 200 || status >= 300 {
		return &mpCitationResult{CheckedAt: checkedAt, Unavailable: true}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return &mpCitationResult{CheckedAt: checkedAt, Unavailable: true}
	}
	overview, ok := payload["aiOverview"].(map[string]any)
	if !ok {
		// Google tidak menampilkan AI Overview untuk kueri ini, jadi "dikutip
		// atau tidak" belum bisa ditentukan. Dulu di sini langsung ditulis
		// false — itu klaim yang tidak pernah kita periksa.
		return &mpCitationResult{CheckedAt: checkedAt}
	}
	hosts := map[string]bool{}
	mpCollectCitedHosts(overview, 0, hosts)
	cited := false
	for host := range hosts {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			cited = true
			break
		}
	}
	return &mpCitationResult{Cited: &cited, CheckedAt: checkedAt}
}

// mpCitationArgs: nilai yang ditulis untuk ai_citation / ai_citation_checked_at.
// Provider tidak tersedia atau tidak dikonfigurasi → pertahankan nilai lama.
// Tidak ada AI Overview → nil (belum bisa ditentukan), bukan false.
func mpCitationArgs(citation *mpCitationResult, prev rowMap) (any, any) {
	var prevCited, prevChecked any
	if prev != nil {
		prevCited = prev.anyOrNil("ai_citation")
		prevChecked = prev.anyOrNil("ai_citation_checked_at")
	}
	if citation == nil || citation.Unavailable {
		return prevCited, prevChecked
	}
	var cited any
	if citation.Cited != nil {
		cited = *citation.Cited
	}
	return cited, citation.CheckedAt
}

// mpCollectCitedHosts: kumpulkan hostname dari subtree aiOverview.
func mpCollectCitedHosts(node any, depth int, out map[string]bool) {
	if depth > 8 || node == nil {
		return
	}
	switch t := node.(type) {
	case []any:
		for _, item := range t {
			mpCollectCitedHosts(item, depth+1, out)
		}
	case map[string]any:
		for _, value := range t {
			mpCollectCitedHosts(value, depth+1, out)
		}
	case string:
		for _, candidate := range mpURLRe.FindAllString(t, -1) {
			if host := mpHostFromRaw(candidate); host != "" && strings.Contains(host, ".") {
				out[host] = true
			}
		}
	}
}

var mpURLRe = regexp.MustCompile(`(?i)(?:https?://|[a-z0-9-]+(?:\.[a-z0-9-]+)+)[^\s"')>]*`)

func mpHostFromRaw(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || strings.ContainsAny(v, " ") {
		return ""
	}
	parsed, err := url.Parse(mpEnsureScheme(v))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}

// ---------- media.seo endpoints (port lib/media/seo-metrics.ts) ----------

type mpSeoResult struct {
	MediaID           string
	Domain            any
	DA                any
	PA                any
	SpamScore         any
	AITrustScore      any
	AICitation        any
	AICitationChecked any
	CheckedAt         any
	Source            string
	Cached            bool
}

func (r mpSeoResult) JSON() map[string]any {
	return map[string]any{
		"media_id":               r.MediaID,
		"domain":                 r.Domain,
		"da":                     r.DA,
		"pa":                     r.PA,
		"spam_score":             r.SpamScore,
		"ai_trust_score":         r.AITrustScore,
		"ai_citation":            r.AICitation,
		"ai_citation_checked_at": r.AICitationChecked,
		"checked_at":             r.CheckedAt,
		"source":                 r.Source,
		"cached":                 r.Cached,
	}
}

const mpSeoMetricColumns = "media_id, da, pa, spam_score, ai_trust_score, ai_citation, ai_citation_checked_at, checked_at, source"

func mpSeoResultFromRow(mediaID string, row ptRow, domain any, cached bool) mpSeoResult {
	return mpSeoResult{
		MediaID:           mediaID,
		Domain:            domain,
		DA:                row.numPtr("da"),
		PA:                row.numPtr("pa"),
		SpamScore:         row.numPtr("spam_score"),
		AITrustScore:      row.numPtr("ai_trust_score"),
		AICitation:        row.anyOrNil("ai_citation"),
		AICitationChecked: row.anyOrNil("ai_citation_checked_at"),
		CheckedAt:         row.anyOrNil("checked_at"),
		Source:            row.strOr("source", ""),
		Cached:            cached,
	}
}

// mpSeoIsFresh: data dianggap segar bila checked_at lebih baru dari TTL.
// TTL berasal dari platform_seo_config.cache_ttl_hours (default 1440 jam = 60
// hari, setara aturan "2 bulan" sebelumnya) supaya biaya kredit API bisa
// diatur admin tanpa mengubah kode.
func (d *Deps) mpSeoIsFresh(ctx context.Context, checkedAt any) bool {
	t, ok := checkedAt.(string)
	if !ok {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return false
	}
	ttlHours := seo.LoadConfig(ctx, d.Pool).CacheTTLHrs
	if ttlHours <= 0 {
		ttlHours = seo.DefaultCacheTTLHrs
	}
	return parsed.After(time.Now().Add(-time.Duration(ttlHours) * time.Hour))
}

// mpCheckAndStoreSeoMetrics: port checkAndStoreSeoMetrics (DA/PA + sinyal AI).
func (d *Deps) mpCheckAndStoreSeoMetrics(ctx context.Context, mediaID string, websiteURL any, actorID string) (mpSeoResult, error) {
	domain, _ := mpDomainFromWebsite(websiteURL).(string)
	if domain == "" {
		return mpSeoResult{}, mpSeoErr(http.StatusBadRequest, "Media belum memiliki website/domain.")
	}
	cached, err := d.queryRow(ctx, "SELECT "+mpSeoMetricColumns+" FROM media_seo_metrics WHERE media_id = $1 LIMIT 1", mediaID)
	if err != nil {
		return mpSeoResult{}, err
	}

	// Baris "segar" tanpa nilai DA tetap harus diambil ulang: kegagalan penyedia
	// menyimpan checked_at walau DA-nya kosong (lihat `metrics` awal di bawah).
	if cached != nil && d.mpSeoIsFresh(ctx, cached.anyOrNil("checked_at")) && cached.has("da") {
		serpensKey := d.mpOptionalSeoKey(ctx, "sealed_serpens_api_key")
		// ai_citation_checked_at, bukan ai_citation: "Google tidak menampilkan AI
		// Overview" adalah hasil pemeriksaan yang sah, jadi SERPens tidak perlu
		// ditanyai berulang hanya karena nilainya kosong.
		needsAI := !cached.has("ai_trust_score") || (serpensKey != "" && !cached.has("ai_citation_checked_at"))
		if !needsAI {
			return mpSeoResultFromRow(mediaID, ptRow(ptNormalizeMap(cached)), domain, true), nil
		}
		aiTrust := d.mpComputeAiTrustScore(ctx, "https://"+domain)
		var citation *mpCitationResult
		if serpensKey != "" {
			citation = d.mpCheckAiCitation(ctx, "https://"+domain)
		}
		cited, citationChecked := mpCitationArgs(citation, cached)
		if err := d.mpUpdateAiSignals(ctx, mediaID, aiTrust, cited, citationChecked); err != nil {
			return mpSeoResult{}, err
		}
		stored, _ := d.queryRow(ctx, "SELECT "+mpSeoMetricColumns+" FROM media_seo_metrics WHERE media_id = $1 LIMIT 1", mediaID)
		if stored == nil {
			return mpSeoResultFromRow(mediaID, ptRow(ptNormalizeMap(cached)), domain, true), nil
		}
		return mpSeoResultFromRow(mediaID, ptRow(ptNormalizeMap(stored)), domain, true), nil
	}

	metrics := map[string]any{"da": nil, "pa": nil, "spam_score": nil}
	var providerErr *mpSeoError
	if d.mpDAProviderPaused(ctx) {
		// Penyedia DA/PA sedang menolak (mis. kuota habis). Lewati panggilannya:
		// AI Trust tetap dihitung, dan catatan kegagalan lama tidak ditimpa supaya
		// pesannya tetap terbaca admin.
	} else if apiKey, keyErr := d.mpPlatformSeoKey(ctx, "sealed_api_key"); keyErr != nil {
		providerErr = keyErr
	} else if fetched, fetchErr := d.mpFetchSeoMetrics(ctx, domain, apiKey); fetchErr != nil {
		providerErr = fetchErr
	} else {
		metrics = fetched
		// API baru tidak selalu menyertakan spam; provider spam checker lama
		// dipakai sebagai pelengkap dengan key RapidAPI yang sama (kolom
		// sealed_spam_api_key tidak pernah ada di skema, jadi jalur lama mati).
		if metrics["spam_score"] == nil {
			if spam := d.mpFetchSpamScore(ctx, domain, apiKey); spam != nil {
				metrics["spam_score"] = *spam
			}
		}
	}

	aiTrust := d.mpComputeAiTrustScore(ctx, "https://"+domain)
	var citation *mpCitationResult
	if d.mpOptionalSeoKey(ctx, "sealed_serpens_api_key") != "" {
		citation = d.mpCheckAiCitation(ctx, "https://"+domain)
	}
	cited, citationChecked := mpCitationArgs(citation, cached)
	citationOK := citation != nil && !citation.Unavailable

	// Jaring pengaman terakhir sebelum menulis ke DB: kolom da/pa/spam_score
	// bertipe smallint dengan CHECK 0..100. Penyedia bisa mengirim -1 (tanda
	// "tidak diketahui") atau nilai persen di atas 100; tanpa normalisasi di
	// sini seluruh INSERT gagal dengan "violates check constraint" dan media
	// itu tidak pernah mendapat skor sama sekali.
	metrics = mpNormalizeSeoMetrics(metrics)

	if providerErr != nil && metrics["da"] == nil && metrics["pa"] == nil && metrics["spam_score"] == nil &&
		aiTrust == nil && !citationOK {
		return mpSeoResult{}, providerErr
	}

	// Catat domain yang diukur supaya baris ini bisa dibedakan dari domain lain
	// bila website media berganti di kemudian hari.
	metrics["domain"] = domain
	if err := d.mpUpsertSeoMetrics(ctx, mediaID, metrics, aiTrust, cited, citationChecked); err != nil {
		return mpSeoResult{}, err
	}
	d.audit(ctx, actorID, "media.seo_metrics_checked", "media", mediaID)

	stored, _ := d.queryRow(ctx, "SELECT "+mpSeoMetricColumns+" FROM media_seo_metrics WHERE media_id = $1 LIMIT 1", mediaID)
	if stored == nil {
		return mpSeoResult{}, mpSeoErr(http.StatusBadGateway, "Metrik SEO gagal disimpan.")
	}
	return mpSeoResultFromRow(mediaID, ptRow(ptNormalizeMap(stored)), domain, false), nil
}

// mpCheckAndStoreGeoSignals: hitung ulang HANYA sinyal GEO/AI (AI Trust +
// status kutipan) tanpa menyentuh penyedia DA/PA.
//
// Dipisah dari mpCheckAndStoreSeoMetrics karena biayanya berbeda jauh: DA/PA
// memanggil RapidAPI berbayar per media, sementara AI Trust hanya mengukur
// kecepatan (PageSpeed, kuota gratis) + membaca schema.org dari halaman itu
// sendiri. Admin yang ingin menyegarkan kolom GEO tidak perlu membakar kuota
// DA untuk seluruh katalog.
//
// Baris metrik tetap dipertahankan: kalau media belum punya baris sama sekali,
// baris dibuat dengan DA/PA kosong supaya sinyal AI punya tempat menyimpan.
func (d *Deps) mpCheckAndStoreGeoSignals(ctx context.Context, mediaID string, websiteURL any, actorID string) (mpSeoResult, error) {
	domain, _ := mpDomainFromWebsite(websiteURL).(string)
	if domain == "" {
		return mpSeoResult{}, mpSeoErr(http.StatusBadRequest, "Media belum memiliki website/domain.")
	}
	target := "https://" + domain

	aiTrust := d.mpComputeAiTrustScore(ctx, target)
	if aiTrust == nil {
		return mpSeoResult{}, mpSeoErr(http.StatusBadGateway,
			"Skor GEO tidak bisa dihitung: PageSpeed tidak mengembalikan skor. Periksa key PageSpeed di konfigurasi SEO.")
	}

	cached, err := d.queryRow(ctx, "SELECT "+mpSeoMetricColumns+" FROM media_seo_metrics WHERE media_id = $1 LIMIT 1", mediaID)
	if err != nil {
		return mpSeoResult{}, err
	}
	var citation *mpCitationResult
	if d.mpOptionalSeoKey(ctx, "sealed_serpens_api_key") != "" {
		citation = d.mpCheckAiCitation(ctx, target)
	}
	cited, citationChecked := mpCitationArgs(citation, cached)

	if cached == nil {
		// Baris baru: hanya kolom AI yang diisi, DA/PA dibiarkan NULL. `source`
		// ditulis 'geo-only' supaya admin tahu angka DA tidak pernah diambil di
		// baris ini (bukan klaim "sudah dicek RapidAPI").
		if err := d.exec(ctx, `
			INSERT INTO media_seo_metrics (media_id, da, pa, spam_score, ai_trust_score, ai_citation, ai_citation_checked_at, checked_at, source, updated_at)
			VALUES ($1, NULL, NULL, NULL, $2, $3, $4, NOW(), 'geo-only', NOW())`,
			mediaID, mpFloatArg(aiTrust), cited, citationChecked); err != nil {
			return mpSeoResult{}, err
		}
	} else if err := d.mpUpdateAiSignals(ctx, mediaID, aiTrust, cited, citationChecked); err != nil {
		return mpSeoResult{}, err
	}
	d.audit(ctx, actorID, "media.geo_checked", "media", mediaID)

	stored, _ := d.queryRow(ctx, "SELECT "+mpSeoMetricColumns+" FROM media_seo_metrics WHERE media_id = $1 LIMIT 1", mediaID)
	if stored == nil {
		return mpSeoResult{}, mpSeoErr(http.StatusBadGateway, "Sinyal GEO gagal disimpan.")
	}
	return mpSeoResultFromRow(mediaID, ptRow(ptNormalizeMap(stored)), domain, false), nil
}

// mpUpdateAiSignals: tulis sinyal AI pada baris yang sudah ada. Nilai ditulis
// apa adanya (termasuk NULL) supaya angka hasil pemeriksaan lama tidak
// tertinggal saat pengukuran berikutnya tidak menghasilkan apa-apa.
func (d *Deps) mpUpdateAiSignals(ctx context.Context, mediaID string, aiTrust *float64, cited, checked any) error {
	return d.exec(ctx, `
		UPDATE media_seo_metrics
		SET ai_trust_score = $1,
		    ai_citation = $2,
		    ai_citation_checked_at = $3,
		    updated_at = NOW()
		WHERE media_id = $4`, mpFloatArg(aiTrust), cited, checked, mediaID)
}

func (d *Deps) mpUpsertSeoMetrics(ctx context.Context, mediaID string, metrics map[string]any, aiTrust *float64, cited, checked any) error {
	return d.exec(ctx, `
		INSERT INTO media_seo_metrics (media_id, da, pa, spam_score, ai_trust_score, ai_citation, ai_citation_checked_at, checked_at, source, domain, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8, $9, NOW())
		ON CONFLICT (media_id) DO UPDATE SET
		  da = COALESCE(EXCLUDED.da, media_seo_metrics.da),
		  pa = COALESCE(EXCLUDED.pa, media_seo_metrics.pa),
		  spam_score = COALESCE(EXCLUDED.spam_score, media_seo_metrics.spam_score),
		  -- Sinyal AI ditulis apa adanya: DA/PA boleh dipertahankan saat penyedia
		  -- gagal, tapi skor AI Trust dan status kutipan harus mencerminkan
		  -- pemeriksaan terakhir — kalau tidak, angka lama yang keliru ikut
		  -- tersimpan selamanya di balik COALESCE.
		  ai_trust_score = EXCLUDED.ai_trust_score,
		  ai_citation = EXCLUDED.ai_citation,
		  ai_citation_checked_at = EXCLUDED.ai_citation_checked_at,
		  checked_at = EXCLUDED.checked_at,
		  source = EXCLUDED.source,
		  domain = COALESCE(EXCLUDED.domain, media_seo_metrics.domain),
		  updated_at = NOW()`,
		mediaID, metrics["da"], metrics["pa"], metrics["spam_score"], mpFloatArg(aiTrust), cited, checked, "rapidapi-domain-metrics",
		mpDomainArg(metrics["domain"]))
}

// mpDomainArg: nilai kolom domain; string kosong menjadi NULL supaya COALESCE
// tidak menimpa domain yang sudah tercatat dengan string kosong.
func mpDomainArg(v any) any {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}

// mpSetManualDA: admin mengisi Domain Authority sendiri (deteksi otomatis tidak
// menemukannya). Nilai disimpan apa adanya dengan sumber 'manual' dan checked_at
// diperbarui, sehingga jendela "segar" (mpSeoIsFresh) menahan pemeriksaan
// otomatis menimpa angka manual ini. Kolom pa, spam_score, dan sinyal AI tidak
// disentuh.
func (d *Deps) mpSetManualDA(ctx context.Context, mediaID string, da int) error {
	return d.exec(ctx, `
		INSERT INTO media_seo_metrics (media_id, da, checked_at, source, created_at, updated_at)
		VALUES ($1, $2, NOW(), 'manual', NOW(), NOW())
		ON CONFLICT (media_id) DO UPDATE
		SET da = EXCLUDED.da,
		    source = EXCLUDED.source,
		    checked_at = NOW(),
		    updated_at = NOW()`, mediaID, da)
}

// mpClearManualDA: kosongkan DA. Baris yang isinya hanya DA dihapus karena tabel
// menuntut minimal satu sinyal terisi; baris yang masih punya sinyal lain cukup
// dikosongkan nilai DA-nya.
func (d *Deps) mpClearManualDA(ctx context.Context, mediaID string) error {
	if err := d.exec(ctx, `
		DELETE FROM media_seo_metrics
		WHERE media_id = $1
		  AND pa IS NULL AND spam_score IS NULL
		  AND ai_trust_score IS NULL AND ai_citation IS NULL`, mediaID); err != nil {
		return err
	}
	return d.exec(ctx, `
		UPDATE media_seo_metrics SET da = NULL, updated_at = NOW()
		WHERE media_id = $1`, mediaID)
}

func mpFloatArg(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// mpNormalizeSeoMetrics: pastikan da/pa/spam_score selalu berada di 0..100
// (atau nil) sebelum ditulis ke kolom smallint ber-CHECK.
//
// Nilai negatif dianggap "tidak ada data" → nil, bukan 0. Bedanya penting:
// spam 0 berarti "sangat bersih", sedangkan -1 dari penyedia berarti "tidak
// diukur" — menyimpannya sebagai 0 akan menampilkan kabar baik palsu.
func mpNormalizeSeoMetrics(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	for _, key := range []string{"da", "pa", "spam_score"} {
		out[key] = mpNormalizeScoreArg(out[key])
	}
	return out
}

// mpNormalizeScoreArg: terima angka dalam bentuk apa pun dari penyedia
// (float64, int, string angka), bulatkan, lalu clamp ke 0..100.
func mpNormalizeScoreArg(v any) any {
	var f float64
	switch t := v.(type) {
	case nil:
		return nil
	case float64:
		f = t
	case float32:
		f = float64(t)
	case int:
		f = float64(t)
	case int64:
		f = float64(t)
	case int32:
		f = float64(t)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return nil
		}
		f = parsed
	default:
		return nil
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return nil
	}
	if f > 100 {
		f = 100
	}
	return math.Round(f)
}

// ---------- SimilarWeb (port lib/media/similarweb.ts) ----------

type mpSimilarWebError struct {
	Status  int
	Message string
}

func (e *mpSimilarWebError) Error() string { return e.Message }

func mpSWErr(status int, msg string) *mpSimilarWebError { return &mpSimilarWebError{status, msg} }

func mpSWAsNumber(v any) any {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f
		}
		return nil
	case string:
		// SimilarWeb kadang mengirim angka sebagai string berformat:
		// "1.234.567", "12,345", "1.2M", "84.5K". Suffix K/M/B dibaca sebagai
		// pengali supaya kunjungan tidak terbaca sebagai 1.2.
		raw := strings.TrimSpace(t)
		if raw == "" {
			return nil
		}
		mult := 1.0
		switch raw[len(raw)-1] {
		case 'K', 'k':
			mult, raw = 1_000, raw[:len(raw)-1]
		case 'M', 'm':
			mult, raw = 1_000_000, raw[:len(raw)-1]
		case 'B', 'b':
			mult, raw = 1_000_000_000, raw[:len(raw)-1]
		}
		cleaned := strings.NewReplacer("%", "", " ", "", ",", "").Replace(strings.TrimSpace(raw))
		// "1.234.567" (pemisah ribuan titik) bukan angka desimal: hanya
		// dibuang titiknya bila ada lebih dari satu titik.
		if strings.Count(cleaned, ".") > 1 {
			cleaned = strings.ReplaceAll(cleaned, ".", "")
		}
		f, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return nil
		}
		return f * mult
	}
	return nil
}

func mpSWAsString(v any) any {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return nil
}

func mpSWArray(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{}
}

func mpSWObject(v any) map[string]any {
	if obj, ok := v.(map[string]any); ok {
		return obj
	}
	return map[string]any{}
}

// mpMapSimilarWebResponse: port mapSimilarWebResponse.
func mpMapSimilarWebResponse(domain string, payload any) map[string]any {
	root := mpSWObject(payload)
	traffic := mpSWObject(root["traffic"])
	engagement := mpSWObject(traffic["engagement"])
	rankings := mpSWObject(root["rankings"])
	seo := mpSWObject(root["seo"])
	ai := mpSWObject(root["ai_referrals"])
	details := mpSWObject(root["website"])
	images := mpSWObject(details["images"])
	meta := mpSWObject(root["meta"])
	category := mpSWObject(details["category"])
	flags := mpSWObject(details["flags"])
	sourcesShare := mpSWObject(traffic["sources_share"])

	history := mpSWArray(traffic["history"])
	latestMonth := map[string]any{}
	if len(history) > 0 {
		latestMonth = mpSWObject(history[len(history)-1])
	}

	countries := []any{}
	for _, entry := range mpSWArray(traffic["geography_share"]) {
		row := mpSWObject(entry)
		share := mpSWAsNumber(row["share"])
		if share == nil {
			share = 0
		}
		country := mpSWAsString(row["country_code"])
		if country == nil {
			country = "—"
		}
		countries = append(countries, map[string]any{"country": country, "share": share})
		if len(countries) >= 10 {
			break
		}
	}

	// Iterasi map Go tidak berurutan (beda dari urutan kunci objek JS), jadi
	// kunci diurutkan agar bentuk respons stabil antar-request.
	sourceNames := make([]string, 0, len(sourcesShare))
	for name := range sourcesShare {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)
	sources := []any{}
	for _, name := range sourceNames {
		if _, ok := sourcesShare[name].(float64); !ok {
			continue
		}
		sources = append(sources, map[string]any{"name": name, "share": mpSWAsNumber(sourcesShare[name])})
		if len(sources) >= 10 {
			break
		}
	}

	keywords := []any{}
	for _, entry := range mpSWArray(seo["top_keywords"]) {
		row := mpSWObject(entry)
		label := mpSWAsString(row["keyword"])
		if label == nil {
			label = mpSWAsString(row["Name"])
		}
		if label == nil {
			label = "—"
		}
		volume := mpSWAsNumber(row["search_volume"])
		if volume == nil {
			volume = mpSWAsNumber(row["Volume"])
		}
		cpc := mpSWAsNumber(row["cpc_usd"])
		if cpc == nil {
			cpc = mpSWAsNumber(row["CPC"])
		}
		est := mpSWAsNumber(row["estimated_value_usd"])
		if est == nil {
			est = mpSWAsNumber(row["EstimatedValue"])
		}
		keywords = append(keywords, map[string]any{"name": label, "volume": volume, "cpc": cpc, "estimated_value": est})
		if len(keywords) >= 10 {
			break
		}
	}

	prompts := []any{}
	for _, p := range mpSWArray(ai["top_prompts"]) {
		if s, ok := p.(string); ok {
			prompts = append(prompts, s)
		} else if s := mpSWAsString(mpSWObject(p)["Prompt"]); s != nil {
			prompts = append(prompts, s)
		}
		if len(prompts) >= 10 {
			break
		}
	}

	platforms := []any{}
	for _, entry := range mpSWArray(ai["platforms"]) {
		row := mpSWObject(entry)
		name := mpSWAsString(row["name"])
		if name == nil {
			name = "—"
		}
		platforms = append(platforms, map[string]any{"name": name, "rank": mpSWAsNumber(row["rank"]), "share": mpSWAsNumber(row["share"])})
		if len(platforms) >= 10 {
			break
		}
	}

	pagesPerVisit := mpSWAsNumber(engagement["pages_per_visit"])
	timeOnSite := mpSWAsString(engagement["average_visit_duration_seconds"])
	if timeOnSite == nil {
		if n := mpSWAsNumber(engagement["average_visit_duration_seconds"]); n != nil {
			timeOnSite = fmt.Sprintf("%v", n)
		}
	}
	var bounce any
	if n := mpSWAsNumber(engagement["bounce_rate"]); n != nil {
		bounce = fmt.Sprintf("%v", n)
	}

	return map[string]any{
		"domain":        domain,
		"snapshot_date": mpSWAsString(meta["snapshot_date"]),
		"visits":        mpSWAsNumber(latestMonth["visits"]),
		"engagement": map[string]any{
			"time_on_site":    timeOnSite,
			"bounce_rate":     bounce,
			"pages_per_visit": pagesPerVisit,
		},
		"top_countries": countries,
		"sources":       sources,
		"ranks": map[string]any{
			"global":   mpSWAsNumber(mpSWRank(rankings["global"])),
			"country":  mpSWAsNumber(mpSWRank(rankings["country"])),
			"category": mpSWAsNumber(mpSWRank(rankings["category"])),
		},
		"top_keywords": keywords,
		"ai_insights": map[string]any{
			"total_visits":  mpSWAsNumber(ai["total_visits"]),
			"traffic_share": mpSWAsNumber(ai["global_traffic_share"]),
			"top_prompts":   prompts,
			"platforms":     platforms,
		},
		"website": map[string]any{
			"title":         mpSWAsString(details["title"]),
			"description":   mpSWAsString(details["description"]),
			"category":      mpSWAsString(mpSWRankName(category)),
			"favicon":       mpSWAsString(images["favicon_url"]),
			"small_website": mpSWBool(flags["is_small_site"]),
		},
	}
}

// mpSWRank: ambil field "rank" dari objek ranking (index hasil fungsi tidak
// diizinkan langsung di Go).
func mpSWRank(v any) any {
	return mpSWObject(v)["rank"]
}

// mpSWRankName: ambil field "name" dari objek (kategori SimilarWeb).
func mpSWRankName(v any) any {
	return mpSWObject(v)["name"]
}

func mpSWBool(v any) any {
	if b, ok := v.(bool); ok {
		return b
	}
	return nil
}

// Provider trafik: "Similar Web API" oleh belchiorarkad di RapidAPI
// (https://rapidapi.com/belchiorarkad-FqvHs2EDOtP/api/similar-web-api).
// Cukup satu kredensial: key RapidAPI lewat header x-rapidapi-key.
//
// Sebelumnya dipakai "Similarweb API Pro" (DataBee) yang mewajibkan key
// RapidAPI DAN DataBee Token; di produksi token itu tidak pernah terisi dan
// semua job trafik gagal. Kolom sealed_databee_token dibiarkan di skema (tidak
// dibaca lagi) supaya perpindahan ini bisa dibalik tanpa migrasi.
const (
	mpSimilarWebHost        = "similar-web-api.p.rapidapi.com"
	mpSimilarWebProviderURL = "https://rapidapi.com/belchiorarkad-FqvHs2EDOtP/api/similar-web-api"
	mpSimilarWebURL         = "https://" + mpSimilarWebHost + "/analyticsv1?url="
	// Domain uji untuk "Tes koneksi provider". Dipilih domain besar yang
	// pasti terdata, supaya jawaban 200 memperlihatkan bentuk JSON lengkap
	// (dipakai untuk memastikan angka kunjungan terbaca).
	mpSimilarWebDiagnoseDomain = "kompas.com"
)

// mpSimilarWebCredentials: buka key RapidAPI untuk provider SimilarWeb.
func (d *Deps) mpSimilarWebCredentials(ctx context.Context) (string, *mpSimilarWebError) {
	row, err := d.queryRow(ctx, "SELECT sealed_similarweb_api_key FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil || row == nil {
		return "", mpSWErr(http.StatusConflict, "SimilarWeb API belum dikonfigurasi oleh Admin. Isi key RapidAPI di kartu SimilarWeb, panel Konfigurasi.")
	}
	sealed := row.strOr("sealed_similarweb_api_key", "")
	if sealed == "" {
		return "", mpSWErr(http.StatusConflict, "Key RapidAPI SimilarWeb belum diisi. Lengkapi di kartu SimilarWeb, panel Konfigurasi.")
	}
	plain, uErr := d.unsealT2(sealed)
	if uErr != nil || plain == "" {
		return "", mpSWErr(http.StatusConflict, "Key RapidAPI SimilarWeb tidak dapat dibuka (APP_SECRET kemungkinan berubah). Simpan ulang di panel Konfigurasi.")
	}
	return plain, nil
}

// mpSimilarWebHeaders: header wajib RapidAPI. Key hanya lewat header, tidak
// pernah masuk log atau pesan error.
func mpSimilarWebHeaders(apiKey string) map[string]string {
	return map[string]string{
		"accept":          "application/json",
		"x-rapidapi-host": mpSimilarWebHost,
		"x-rapidapi-key":  apiKey,
	}
}

func (d *Deps) mpSimilarWebFetch(ctx context.Context, domain, apiKey string) (map[string]any, *mpSimilarWebError) {
	target := mpSimilarWebURL + url.QueryEscape(domain)
	status, body := mpSafeGet(ctx, target, mpSimilarWebHeaders(apiKey), 15*time.Second)
	if status == 0 {
		return nil, mpSWErr(http.StatusServiceUnavailable, "Provider SimilarWeb tidak dapat dihubungi sekarang.")
	}
	if status < 200 || status >= 300 {
		switch status {
		case 401, 403:
			// RapidAPI memakai 403 "not subscribed" baik untuk key yang tidak
			// dikenal maupun key valid yang belum Subscribe listing ini.
			if status == 403 && strings.Contains(body, "not subscribed") {
				return nil, mpSWErr(http.StatusServiceUnavailable,
					"RapidAPI menolak: \"You are not subscribed to this API.\" Buka listing Similar Web API (belchiorarkad) di RapidAPI lalu Subscribe, lalu coba lagi. Key tersimpan tidak perlu diganti.")
			}
			return nil, mpSWErr(http.StatusServiceUnavailable,
				"Autentikasi provider SimilarWeb ditolak (401/403). Cek key RapidAPI di kartu SimilarWeb, panel Konfigurasi, lalu jalankan \"Tes koneksi provider\" untuk melihat jawaban mentah provider.")
		case 429:
			return nil, mpSWErr(http.StatusServiceUnavailable, "Provider SimilarWeb sedang rate-limited. Coba lagi nanti.")
		case 404:
			return nil, mpSWErr(http.StatusServiceUnavailable, "Domain tidak ditemukan di SimilarWeb.")
		default:
			return nil, mpSWErr(http.StatusServiceUnavailable, "Provider SimilarWeb sedang tidak tersedia.")
		}
	}
	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, mpSWErr(http.StatusBadGateway, "Respons provider SimilarWeb kosong (malformed).")
	}
	mapped := mpMapSimilarWebAnalytics(domain, parsed)
	// Kunjungan 0 adalah jawaban sah: provider menandai domain yang tidak
	// terindeks dengan IsSmall/angka nol. Snapshot tetap disimpan supaya
	// sweep tidak menanyakan domain yang sama berulang kali, tetapi diberi
	// penanda no_data agar UI bisa membedakan "0 kunjungan" dari "0 karena
	// provider tidak punya data".
	//
	// Yang tetap dianggap gagal hanyalah bentuk respons yang tidak dikenali
	// sama sekali (tidak ada field kunjungan). Statusnya 422, bukan 502:
	// gateway tidak bermasalah, provider justru menjawab.
	visits, ok := mapped["visits"].(float64)
	if !ok {
		return nil, mpSWErr(http.StatusUnprocessableEntity,
			"Provider SimilarWeb menjawab, tetapi angka kunjungan tidak ditemukan di respons. Bentuk jawaban provider mungkin berbeda dari yang diperkirakan — jalankan \"Tes koneksi provider\" di Konfigurasi untuk melihat JSON aslinya.")
	}
	mapped["no_data"] = visits <= 0
	return mapped, nil
}

// mpMapSimilarWebAnalytics: petakan jawaban /analyticsv1 ke bentuk insights
// yang disimpan di media_similarweb_snapshots.
//
// Listing belchiorarkad tidak memuat contoh respons; tutorialnya hanya
// menyebut objek "domain_analytics" bertipe bebas. Karena itu pemetaan ini
// membuka pembungkus yang umum (domain_analytics / data / result) lalu mencoba
// beberapa bentuk SimilarWeb yang dikenal. Hanya "visits" dan "snapshot_date"
// yang dibaca kode lain (katalog, matchmaker, sweep scheduler), jadi dua
// field itu yang dicari paling teliti. Bila bentuknya tetap tidak dikenali,
// visits bernilai nil dan media dicoba ulang oleh sweep berikutnya.
func mpMapSimilarWebAnalytics(domain string, payload any) map[string]any {
	root := mpSWUnwrap(payload)
	out := mpMapSimilarWebResponse(domain, root)
	if v, ok := out["visits"].(float64); !ok || v <= 0 {
		if f := mpSWFindVisits(root); f != nil {
			out["visits"] = f
		}
	}
	if out["snapshot_date"] == nil {
		for _, k := range []string{"SnapshotDate", "snapshotDate", "snapshot_date"} {
			if s, ok := root[k].(string); ok && len(s) >= 10 {
				// Kolom snapshot_date bertipe date: cukup YYYY-MM-DD.
				out["snapshot_date"] = s[:10]
				break
			}
		}
	}
	return out
}

// mpSWUnwrap: buka pembungkus respons sampai ketemu objek data.
func mpSWUnwrap(payload any) map[string]any {
	root := mpSWObject(payload)
	for i := 0; i < 3; i++ {
		next := false
		for _, k := range []string{"domain_analytics", "data", "result"} {
			if inner, ok := root[k].(map[string]any); ok && len(inner) > 0 {
				root = inner
				next = true
				break
			}
		}
		if !next {
			break
		}
	}
	return root
}

// mpSWFindVisits: angka kunjungan bulanan dari bentuk-bentuk SimilarWeb yang
// dikenal, urut dari yang paling spesifik.
func mpSWFindVisits(root map[string]any) any {
	// Bentuk SimilarWeb mentah: "Engagments" (ejaan asli SimilarWeb) atau
	// "Engagements" berisi "Visits" (sering berupa string).
	for _, k := range []string{"Engagments", "Engagements", "engagements", "engagement"} {
		eng := mpSWObject(root[k])
		for _, vk := range []string{"Visits", "visits", "total_visits", "totalVisits"} {
			if v := mpSWCountValue(eng[vk]); v != nil {
				return v
			}
		}
	}
	// "EstimatedMonthlyVisits": {"2026-08-01": 123, ...} → ambil bulan terakhir.
	for _, k := range []string{"EstimatedMonthlyVisits", "estimatedMonthlyVisits", "monthly_visits"} {
		months := mpSWObject(root[k])
		if len(months) == 0 {
			continue
		}
		keys := make([]string, 0, len(months))
		for m := range months {
			keys = append(keys, m)
		}
		sort.Strings(keys)
		if v := mpSWCountValue(months[keys[len(keys)-1]]); v != nil {
			return v
		}
	}
	// Field datar di tingkat atas.
	for _, k := range []string{"visits", "Visits", "totalVisits", "total_visits", "monthlyVisits", "monthly_visits"} {
		if v := mpSWCountValue(root[k]); v != nil {
			return v
		}
	}
	// Objek "traffic" berisi total kunjungan atau riwayat bulanan.
	traffic := mpSWObject(root["traffic"])
	for _, k := range []string{"visits", "total_visits", "totalVisits", "monthly_visits"} {
		if v := mpSWCountValue(traffic[k]); v != nil {
			return v
		}
	}
	if history := mpSWArray(traffic["history"]); len(history) > 0 {
		last := mpSWObject(history[len(history)-1])
		for _, k := range []string{"visits", "Visits", "total_visits"} {
			if v := mpSWCountValue(last[k]); v != nil {
				return v
			}
		}
	}
	return nil
}

// mpSWCountValue: angka kunjungan (>= 0) atau nil bila field tidak ada /
// tidak numerik.
//
// NOL DITERIMA. Provider menjawab 0 untuk domain yang tidak terindeks
// (mis. IsSmall: true) — itu jawaban sah, bukan respons rusak. Dulu nol
// dibuang di sini, sehingga domain tanpa data trafik dianggap gagal dan
// admin menerima 502 padahal provider menjawab 200.
func mpSWCountValue(v any) any {
	n, ok := mpSWAsNumber(v).(float64)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return nil
	}
	return n
}

const mpSimilarWebSnapshotColumns = "id, media_id, insights, snapshot_date, created_at"

// mpGetLatestSimilarWebSnapshot: snapshot terakhir (tanpa provider).
func (d *Deps) mpGetLatestSimilarWebSnapshot(ctx context.Context, mediaID string) map[string]any {
	row, err := d.queryRow(ctx,
		"SELECT "+mpSimilarWebSnapshotColumns+" FROM media_similarweb_snapshots WHERE media_id = $1 ORDER BY created_at DESC LIMIT 1", mediaID)
	if err != nil || row == nil {
		return nil
	}
	return d.mpSimilarWebSnapshotJSON(ptRow(ptNormalizeMap(row)), mediaID)
}

func (d *Deps) mpSimilarWebSnapshotJSON(row ptRow, mediaID string) map[string]any {
	insights := row.jsonField("insights")
	out := map[string]any{}
	if obj, ok := insights.(map[string]any); ok {
		for k, v := range obj {
			out[k] = v
		}
	}
	out["media_id"] = mediaID
	out["fetched_at"] = row.anyOrNil("created_at")
	out["snapshot_id"] = row.anyOrNil("id")
	out["cached"] = true
	return out
}

// mpCheckSimilarWebInsights: ambil insights; cooldown 6 bulan, force = fetch ulang.
func (d *Deps) mpCheckSimilarWebInsights(ctx context.Context, mediaID string, websiteURL any, actorID string, force bool) (map[string]any, *mpSimilarWebError) {
	domain, _ := mpDomainFromWebsite(websiteURL).(string)
	if domain == "" {
		return nil, mpSWErr(http.StatusBadRequest, "Masukkan domain untuk diperiksa.")
	}
	latest, err := d.queryRow(ctx,
		"SELECT "+mpSimilarWebSnapshotColumns+" FROM media_similarweb_snapshots WHERE media_id = $1 ORDER BY created_at DESC LIMIT 1", mediaID)
	if err != nil {
		return nil, mpSWErr(http.StatusBadGateway, "Respons provider SimilarWeb kosong (malformed).")
	}

	fresh := false
	hasValidVisits := false
	if latest != nil {
		normalized := ptRow(ptNormalizeMap(latest))
		if t, ok := normalized.anyOrNil("created_at").(string); ok {
			if parsed, pErr := time.Parse(time.RFC3339, t); pErr == nil {
				// Aturan pemilik produk: media yang datanya sudah ada dibiarkan
				// 6 bulan sebelum reqrawl (dulu 2 bulan).
				fresh = parsed.After(time.Now().AddDate(0, -6, 0))
			}
		}
		if insights, ok := normalized.jsonField("insights").(map[string]any); ok {
			// 0 juga hasil sah (provider tidak punya data untuk domain ini),
			// jadi snapshot berisi 0 tetap dipakai sebagai cache 6 bulan.
			if _, ok := insights["visits"].(float64); ok {
				hasValidVisits = true
			}
		}
	}
	if !force && latest != nil && fresh && hasValidVisits {
		return d.mpSimilarWebSnapshotJSON(ptRow(ptNormalizeMap(latest)), mediaID), nil
	}

	apiKey, keyErr := d.mpSimilarWebCredentials(ctx)
	if keyErr != nil {
		return nil, keyErr
	}
	insights, fetchErr := d.mpSimilarWebFetch(ctx, domain, apiKey)
	if fetchErr != nil {
		return nil, fetchErr
	}

	var snapshotDate any
	if s := mpSWAsString(insights["snapshot_date"]); s != nil {
		snapshotDate = s
	}
	if err := d.exec(ctx,
		"INSERT INTO media_similarweb_snapshots (id, media_id, insights, snapshot_date, created_by) VALUES ($1, $2, $3::jsonb, $4, $5)",
		uuid.NewString(), mediaID, mpJSONString(insights), snapshotDate, actorID); err != nil {
		return nil, mpSWErr(http.StatusBadGateway, "Respons provider SimilarWeb kosong (malformed).")
	}

	if visits, ok := insights["visits"].(float64); ok {
		_ = d.exec(ctx, "UPDATE media SET traffic_estimate = $1, updated_at = NOW() WHERE id = $2", int(math.Round(visits)), mediaID)
	}
	d.audit(ctx, actorID, "media.similarweb_checked", "media", mediaID)

	out := map[string]any{}
	for k, v := range insights {
		out[k] = v
	}
	out["media_id"] = mediaID
	out["fetched_at"] = d.isoNow()
	out["cached"] = false
	return out, nil
}

// isoNow: waktu sekarang ISO mili (helper internal).
func (d *Deps) isoNow() string { return isoMilli(time.Now()) }

// mpEnv: pembaca env sederhana.
func mpEnv(key string) string { return strings.TrimSpace(mpGetenv(key)) }

// mpGetenv dipisah agar pembacaan env terpusat di satu titik.
var mpGetenv = func(key string) string { return os.Getenv(key) }

// ---------- health score (port lib/media/intelligence.ts) ----------

// mpComputeMediaHealth: skor 0..100 dari riwayat operasional + kualitas.
func (d *Deps) mpComputeMediaHealth(ctx context.Context, mediaID string) (int, map[string]any, error) {
	media, err := d.queryRow(ctx,
		"SELECT id, status, traffic_estimate, media_quality_score, media_tier, integration_type FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		return 0, nil, err
	}
	if media == nil {
		return 0, nil, errNotFound("Media not found")
	}
	row := ptRow(ptNormalizeMap(media))

	stats, err := d.queryRow(ctx, `
		SELECT
		  COUNT(*)::int AS total,
		  COUNT(*) FILTER (WHERE status = 'published')::int AS published,
		  COUNT(*) FILTER (WHERE status = 'partner_rejected')::int AS rejected,
		  COUNT(*) FILTER (WHERE status = 'failed')::int AS failed,
		  -- SLA hanya dari publikasi yang terbit <= 7 hari; publikasi yang
		  -- menggantung karena mitra lupa ACC dibuang dari data SLA.
		  ROUND(AVG(CASE WHEN published_at IS NOT NULL
		                  AND published_at <= created_at + INTERVAL '7 days'
		                 THEN EXTRACT(EPOCH FROM (published_at - created_at)) / 3600.0 END), 2)::double precision AS avg_publish_hours
		FROM publications
		WHERE media_id = $1 AND status <> 'cancelled'`, mediaID)
	if err != nil {
		return 0, nil, err
	}
	total := 0
	published := 0
	rejected := 0
	failed := 0
	var avgPublishHours *float64
	if stats != nil {
		s := ptRow(ptNormalizeMap(stats))
		total = s.intOr("total", 0)
		published = s.intOr("published", 0)
		rejected = s.intOr("rejected", 0)
		failed = s.intOr("failed", 0)
		if s.has("avg_publish_hours") {
			v := s.numOr("avg_publish_hours")
			avgPublishHours = &v
		}
	}

	status := row.strOr("status", "")
	availability := 0.0
	if status == "active" {
		availability = 15
	} else if status == "draft" {
		availability = 8
	}

	successRate := 10.0
	if total > 0 {
		successRate = float64(published) / float64(total) * 30
	}

	var publishTime float64
	if avgPublishHours == nil {
		publishTime = 21
	} else if *avgPublishHours <= 6 {
		publishTime = 35
	} else if *avgPublishHours <= 24 {
		publishTime = 28
	} else if *avgPublishHours <= 72 {
		publishTime = 18
	} else {
		publishTime = 9
	}

	rejectionRate := 12.0
	if total > 0 {
		rejectionRate = math.Max(0, 20-(float64(rejected)/float64(total))*20)
	}

	operationalTotal := math.Max(0, math.Min(100, math.Round(availability+successRate+publishTime+rejectionRate)))

	tierDefaults := d.mpTierDefaultQuality(ctx)
	var quality *float64
	qualitySource := any(nil)
	if row.has("media_quality_score") {
		v := math.Max(0, math.Min(100, row.numOr("media_quality_score")))
		quality = &v
		qualitySource = "media"
	} else if tier, ok := row.str("media_tier"); ok && tier != "" {
		if def, exists := tierDefaults[tier]; exists {
			v := math.Max(0, math.Min(100, def))
			quality = &v
			qualitySource = "tier_default"
		}
	}

	totalScore := 0.0
	if total > 0 {
		if quality != nil {
			totalScore = math.Round(operationalTotal*0.85 + *quality*0.15)
		} else {
			totalScore = operationalTotal
		}
	}

	noHistory := total == 0
	// Lihat schComputeMediaHealth: batas tampilan 72 jam, kecuali integrasi
	// manual; skor tetap dari nilai mentah.
	displayAvg := avgPublishHours
	if displayAvg != nil && *displayAvg > 72 && row.strOr("integration_type", "") != "manual" {
		v := 72.0
		displayAvg = &v
	}
	breakdown := map[string]any{
		"availability":          mathOrZero(noHistory, availability),
		"success_rate":          mathRound2OrZero(noHistory, successRate),
		"publish_time":          mathOrZero(noHistory, publishTime),
		"rejection_rate":        mathRound2OrZero(noHistory, rejectionRate),
		"operational_total":     mathOrZero(noHistory, operationalTotal),
		"quality":               mpFloatArg(quality),
		"quality_source":        qualitySource,
		"reach":                 nil,
		"total":                 int(totalScore),
		"published":             published,
		"rejected":              rejected,
		"failed":                failed,
		"avg_publish_hours":     mpFloatArg(displayAvg),
		"avg_publish_hours_raw": mpFloatArg(avgPublishHours),
	}
	return int(totalScore), breakdown, nil
}

func mathOrZero(noHistory bool, v float64) any {
	if noHistory {
		return 0
	}
	return v
}

func mathRound2OrZero(noHistory bool, v float64) any {
	if noHistory {
		return 0
	}
	return mpRound2(v)
}

// mpTierDefaultQuality: pr_media_rules.tier_default_quality, fallback default.
func (d *Deps) mpTierDefaultQuality(ctx context.Context) map[string]float64 {
	fallback := map[string]float64{"tier1": 90, "tier2": 80, "tier3": 70}
	row, err := d.queryRow(ctx, "SELECT tier_default_quality FROM pr_media_rules WHERE id = 'default' LIMIT 1")
	if err != nil || row == nil {
		return fallback
	}
	parsed, ok := row.jsonField("tier_default_quality").(map[string]any)
	if !ok || len(parsed) == 0 {
		return fallback
	}
	out := map[string]float64{}
	for k, v := range parsed {
		if f, ok := v.(float64); ok {
			out[k] = f
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

// mpRefreshMediaHealth: hitung + simpan health_score/breakdown.
func (d *Deps) mpRefreshMediaHealth(ctx context.Context, mediaID string) (int, map[string]any, error) {
	score, breakdown, err := d.mpComputeMediaHealth(ctx, mediaID)
	if err != nil {
		return 0, nil, err
	}
	if err := d.exec(ctx,
		"UPDATE media SET health_score = $1, health_breakdown = $2::jsonb, health_computed_at = NOW(), updated_at = NOW() WHERE id = $3",
		score, mpJSONString(breakdown), mediaID); err != nil {
		return 0, nil, err
	}
	return score, breakdown, nil
}

// ---------- standard editorial (port lib/standard/check.ts + service.ts) ----------

var mpPromotionalTerms = []string{
	"diskon", "discount", "promo", "promosi", "gratis", "beli sekarang", "order sekarang",
	"dapatkan sekarang", "harga spesial", "terbatas", "stok terbatas", "klik di sini", "klik disini",
	"murah", "best price", "hubungi kami", "segera daftar", "bursa", "bundling", "cashback",
}

var mpToleranceThresholds = map[string]struct{ warn, fail int }{
	"none":   {1, 1},
	"low":    {1, 4},
	"medium": {3, 7},
	"high":   {6, 11},
}

func mpCountWords(text string) int {
	cleaned := mpTagRe.ReplaceAllString(text, " ")
	cleaned = mpMarkRe.ReplaceAllString(cleaned, " ")
	fields := strings.Fields(cleaned)
	return len(fields)
}

var (
	mpTagRe   = regexp.MustCompile(`<[^>]*>`)
	mpMarkRe  = regexp.MustCompile("[#*_`~>-]")
	mpQuoteRe = regexp.MustCompile(`["“”«»„]`)
)

func mpCountQuotes(text string) int {
	return len(mpQuoteRe.FindAllString(text, -1)) / 2
}

func mpCountPromotional(text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, term := range mpPromotionalTerms {
		count += strings.Count(lower, term)
	}
	return count
}

func mpParseStrings(value any) []string {
	arr, ok := value.([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, item := range arr {
		s := strings.TrimSpace(fmt.Sprint(item))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// mpRunStandardCheck: port runStandardCheck → {status, score, checks, failed, warnings}.
func mpRunStandardCheck(standard ptRow, text string, mediaName any) map[string]any {
	checks := []map[string]any{}
	failed := []string{}
	warnings := []string{}

	minWords := standard.intOr("min_words", 0)
	maxWords := standard.intOr("max_words", 0)
	words := mpCountWords(text)
	if minWords > 0 && words < minWords {
		checks = append(checks, mpCheck("length", "Panjang artikel", "fail",
			fmt.Sprintf("%d kata — kurang dari minimum %d", words, minWords)))
		failed = append(failed, fmt.Sprintf("Panjang artikel (%d kata < %d)", words, minWords))
	} else if maxWords > 0 && words > maxWords {
		checks = append(checks, mpCheck("length", "Panjang artikel", "fail",
			fmt.Sprintf("%d kata — melebihi maksimum %d", words, maxWords)))
		failed = append(failed, fmt.Sprintf("Panjang artikel (%d kata > %d)", words, maxWords))
	} else {
		checks = append(checks, mpCheck("length", "Panjang artikel", "ok",
			fmt.Sprintf("%d kata (batas %d-%d)", words, minWords, maxWords)))
	}

	tolerance := standard.strOr("hard_selling_tolerance", "low")
	thresholds, ok := mpToleranceThresholds[tolerance]
	if !ok {
		thresholds = mpToleranceThresholds["low"]
	}
	promo := mpCountPromotional(text)
	switch {
	case promo >= thresholds.fail:
		checks = append(checks, mpCheck("hard_selling", "Konten promosi berlebihan", "fail",
			fmt.Sprintf("%d istilah promosi ditemukan (toleransi %s)", promo, tolerance)))
		failed = append(failed, fmt.Sprintf("Konten promosi (%d istilah)", promo))
	case promo >= thresholds.warn:
		checks = append(checks, mpCheck("hard_selling", "Konten promosi", "warn",
			fmt.Sprintf("%d istilah promosi — kurangi agar sesuai gaya %s", promo, standard.strOr("journalistic_style", "news"))))
		warnings = append(warnings, fmt.Sprintf("Konten promosi (%d istilah)", promo))
	default:
		checks = append(checks, mpCheck("hard_selling", "Konten promosi", "ok",
			fmt.Sprintf("%d istilah promosi (toleransi %s)", promo, tolerance)))
	}

	quotes := mpCountQuotes(text)
	quoteThreshold := standard.intOr("min_quotes", 0)
	if standard.boolOr("quote_required", false) && quoteThreshold < 1 {
		quoteThreshold = 1
	}
	switch {
	case quoteThreshold > 0 && quotes < quoteThreshold:
		checks = append(checks, mpCheck("quotes", "Kutipan narasumber", "fail",
			fmt.Sprintf("Ditemukan %d kutipan — butuh minimal %d", quotes, quoteThreshold)))
		failed = append(failed, fmt.Sprintf("Kutipan (%d < %d)", quotes, quoteThreshold))
	case quoteThreshold > 0:
		checks = append(checks, mpCheck("quotes", "Kutipan narasumber", "ok", fmt.Sprintf("%d kutipan", quotes)))
	case standard.boolOr("quote_required", false):
		checks = append(checks, mpCheck("quotes", "Kutipan narasumber", "fail", "Belum ada kutipan"))
		failed = append(failed, "Kutipan narasumber")
	default:
		checks = append(checks, mpCheck("quotes", "Kutipan narasumber", "warn", "Tidak wajib, tapi memperkuat artikel"))
		warnings = append(warnings, "Kutipan narasumber belum ada")
	}

	lower := strings.ToLower(text)
	forbidden := mpParseStrings(standard.anyOrNil("forbidden_content"))
	hits := []string{}
	for _, term := range forbidden {
		if term != "" && strings.Contains(lower, strings.ToLower(term)) {
			hits = append(hits, term)
		}
	}
	if len(hits) > 0 {
		checks = append(checks, mpCheck("forbidden", "Konten terlarang", "fail",
			"Ditemukan: "+strings.Join(mpHead(hits, 5), ", ")))
		failed = append(failed, "Konten terlarang ("+strings.Join(hits, ", ")+")")
	} else {
		checks = append(checks, mpCheck("forbidden", "Konten terlarang", "ok", "Bersih"))
	}

	required := mpParseStrings(standard.anyOrNil("required_info"))
	if len(required) > 0 {
		missing := []string{}
		for _, term := range required {
			if term != "" && !strings.Contains(lower, strings.ToLower(term)) {
				missing = append(missing, term)
			}
		}
		if len(missing) == 0 {
			checks = append(checks, mpCheck("required_info", "Informasi wajib", "ok", "Semua tercakup"))
		} else {
			checks = append(checks, mpCheck("required_info", "Informasi wajib", "warn",
				"Belum ada: "+strings.Join(mpHead(missing, 5), ", ")))
			warnings = append(warnings, "Informasi wajib belum ada ("+strings.Join(missing, ", ")+")")
		}
	} else {
		checks = append(checks, mpCheck("required_info", "Informasi wajib", "ok", "Tidak ada ketentuan"))
	}

	hasImage := mpImageRe.MatchString(text)
	if standard.boolOr("image_required", false) && !hasImage {
		checks = append(checks, mpCheck("image", "Gambar pendukung", "warn", "Belum ada gambar — media memintanya"))
		warnings = append(warnings, "Gambar pendukung belum ada")
	} else if standard.boolOr("image_required", false) {
		checks = append(checks, mpCheck("image", "Gambar pendukung", "ok", "Ada"))
	} else {
		checks = append(checks, mpCheck("image", "Gambar pendukung", "ok", "Tidak wajib"))
	}

	if standard.boolOr("category_required", false) {
		checks = append(checks, mpCheck("category", "Kategori media", "warn", "Pastikan kategori dipilih saat mengirim ke media"))
		warnings = append(warnings, "Kategori media perlu dipilih")
	} else {
		checks = append(checks, mpCheck("category", "Kategori media", "ok", "Tidak wajib"))
	}

	weights := map[string]int{
		"length": 20, "hard_selling": 20, "quotes": 15, "forbidden": 25, "required_info": 15, "image": 5, "category": 5,
	}
	achieved := 0
	for _, c := range checks {
		if c["status"] == "ok" {
			key, _ := c["key"].(string)
			achieved += weights[key]
		}
	}
	status := "pass"
	if len(failed) > 0 {
		status = "fail"
	} else if len(warnings) > 0 {
		status = "warning"
	}
	return map[string]any{
		"media_id":   standard.anyOrNil("media_id"),
		"media_name": mediaName,
		"status":     status,
		"score":      achieved,
		"checks":     checks,
		"failed":     failed,
		"warnings":   warnings,
	}
}

var mpImageRe = regexp.MustCompile(`(?i)!\[[^\]]*\]\([^)]+\)|https?:\/\/\S+\.(png|jpe?g|webp|gif)`)

func mpCheck(key, label, status, detail string) map[string]any {
	return map[string]any{"key": key, "label": label, "status": status, "detail": detail}
}

func mpHead(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

// mpFinalStandardCheck: null bila media tidak punya standard aktif.
func (d *Deps) mpFinalStandardCheck(ctx context.Context, brandID, documentID, mediaID string) (map[string]any, error) {
	standard, err := d.mpMediaStandard(ctx, mediaID)
	if err != nil || standard == nil {
		return nil, err
	}
	text, _ := d.mpArticleContent(ctx, brandID, documentID)
	media, _ := d.queryRow(ctx, "SELECT name FROM media WHERE id = $1 LIMIT 1", mediaID)
	var mediaName any
	if media != nil {
		mediaName = ptRow(ptNormalizeMap(media)).anyOrNil("name")
	}
	return mpRunStandardCheck(standard, text, mediaName), nil
}

func (d *Deps) mpMediaStandard(ctx context.Context, mediaID string) (ptRow, error) {
	row, err := d.queryRow(ctx, `
		SELECT id, media_id, min_words, max_words, journalistic_style, hard_selling_tolerance,
		       quote_required, min_quotes, image_required, category_required, required_info,
		       forbidden_content, status
		FROM media_content_standards WHERE media_id = $1 AND status = 'active' LIMIT 1`, mediaID)
	if err != nil || row == nil {
		return nil, err
	}
	return ptRow(ptNormalizeMap(row)), nil
}

func (d *Deps) mpArticleContent(ctx context.Context, brandID, documentID string) (string, string) {
	doc, err := d.queryRow(ctx,
		"SELECT id, title, current_version FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1", documentID, brandID)
	if err != nil || doc == nil {
		return "", ""
	}
	drow := ptRow(ptNormalizeMap(doc))
	version, _ := d.queryRow(ctx,
		"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		documentID, drow.intOr("current_version", 0))
	text := ""
	if version != nil {
		text, _ = ptRow(ptNormalizeMap(version)).str("content")
	}
	return text, drow.strOr("title", "")
}

// ---------- auto-replace (port lib/media/auto-replace.ts + intelligence.ts) ----------

const mpDefaultMaxAutoReplaces = 3

type mpAutoReplaceConfig struct {
	Enabled     bool
	MaxReplaces int
}

func (d *Deps) mpGetAutoReplaceConfig(ctx context.Context, brandID string) mpAutoReplaceConfig {
	brand, err := d.queryRow(ctx, "SELECT enabled, max_replaces FROM auto_replace_configs WHERE brand_id = $1 LIMIT 1", brandID)
	if err == nil && brand != nil {
		row := ptRow(ptNormalizeMap(brand))
		max := row.intOr("max_replaces", 0)
		if max <= 0 {
			max = mpDefaultMaxAutoReplaces
		}
		return mpAutoReplaceConfig{Enabled: row.boolOr("enabled", false), MaxReplaces: max}
	}
	global, err := d.queryRow(ctx, "SELECT enabled, max_replaces FROM auto_replace_configs WHERE brand_id IS NULL LIMIT 1")
	if err == nil && global != nil {
		row := ptRow(ptNormalizeMap(global))
		max := row.intOr("max_replaces", 0)
		if max <= 0 {
			max = mpDefaultMaxAutoReplaces
		}
		return mpAutoReplaceConfig{Enabled: row.boolOr("enabled", false), MaxReplaces: max}
	}
	return mpAutoReplaceConfig{Enabled: true, MaxReplaces: mpDefaultMaxAutoReplaces}
}

type mpMediaRecommendation struct {
	ID     string
	Name   string
	Score  int
	Health float64
}

// mpRecommendMedia: port recommendMedia (skor budget/health/kategori/kinerja).
func (d *Deps) mpRecommendMedia(ctx context.Context, category string, budget *float64, count int, exclude map[string]bool) ([]mpMediaRecommendation, error) {
	if count < 1 {
		count = 10
	}
	if count > 50 {
		count = 50
	}
	rows, err := d.queryRows(ctx, `
		SELECT m.id, m.name, m.category, m.price, m.credit_cost, m.integration_type, m.health_score,
		       m.traffic_estimate, m.status, m.dewan_pers_verified,
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
	scored := make([]mpMediaRecommendation, 0, len(rows))
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		id := row.strOr("id", "")
		if exclude[id] {
			continue
		}
		price := row.numOr("price")
		healthScore := row.numOr("health_score")
		hasHealth := row.has("health_score")
		published := row.intOr("pub_count", 0)
		traffic := row.numOr("traffic_estimate")

		nameLower := strings.ToLower(row.strOr("name", ""))
		penalty := 0.0
		if strings.Contains(nameLower, "smoke") || strings.Contains(nameLower, "test") {
			penalty = 35
		} else if strings.EqualFold(row.strOr("integration_type", ""), "manual") && strings.Contains(nameLower, "non partner") {
			penalty = 15
		}

		budgetFit := 40.0
		if budget != nil {
			if price <= *budget {
				budgetFit = 40 - math.Min(15, (price / *budget)*15)
			} else {
				budgetFit = 10
			}
		}
		healthFit := 18.0
		if hasHealth {
			healthFit = (healthScore / 100) * 30
		}
		categoryFit := 10.0
		if category != "" && strings.EqualFold(row.strOr("category", ""), category) {
			categoryFit = 20
		}
		// Performance (10): riwayat publikasi + traffic (parity app lama).
		perf := math.Min(7, float64(published)*2+traffic/100000)
		if traffic > 0 {
			perf += 3
		}
		performanceFit := math.Min(10, perf)
		total := math.Max(0, math.Round(budgetFit+healthFit+categoryFit+performanceFit-penalty))
		scored = append(scored, mpMediaRecommendation{ID: id, Name: row.strOr("name", ""), Score: int(total), Health: healthScore})
	}
	// Urut skor desc, lalu health desc (disamakan dengan app lama).
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			better := scored[j].Score > scored[i].Score ||
				(scored[j].Score == scored[i].Score && scored[j].Health > scored[i].Health)
			if better {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
	if len(scored) > count {
		return scored[:count], nil
	}
	return scored, nil
}

// mpReplacePublicationAfterLinkLoss: port replacePublicationAfterLinkLoss.
// Mengembalikan map dengan kunci replaced / skipped_reason / publication_id /
// media_id / media_name.
func (d *Deps) mpReplacePublicationAfterLinkLoss(ctx context.Context, publicationID string) map[string]any {
	pub, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, p.document_id, p.campaign_id, p.client_price, p.publish_status, p.created_by,
		       p.status, p.auto_replace_disabled, d.title AS document_title, m.name AS media_name,
		       m.category AS media_category, c.state AS link_state
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || pub == nil {
		return map[string]any{"replaced": false, "skipped_reason": "publication not found or not published"}
	}
	row := ptRow(ptNormalizeMap(pub))
	if row.strOr("status", "") != "published" {
		return map[string]any{"replaced": false, "skipped_reason": "publication not found or not published"}
	}
	if row.strOr("link_state", "") != "gone" {
		return map[string]any{"replaced": false, "skipped_reason": "link is not marked as gone"}
	}
	return d.mpRunAutoReplace(ctx, row, publicationID)
}

func (d *Deps) mpRunAutoReplace(ctx context.Context, pub ptRow, publicationID string) map[string]any {
	brandID := pub.strOr("brand_id", "")
	if pub.boolOr("auto_replace_disabled", false) {
		return map[string]any{"replaced": false, "skipped_reason": "auto-replace disabled for this publication"}
	}
	config := d.mpGetAutoReplaceConfig(ctx, brandID)
	if !config.Enabled {
		return map[string]any{"replaced": false, "skipped_reason": "auto-replace disabled by configuration"}
	}
	counter, err := d.queryRow(ctx,
		"SELECT COUNT(*)::int AS count FROM audit_logs WHERE action = 'publication.auto_replaced' AND target_id = $1", publicationID)
	if err != nil {
		return map[string]any{"replaced": false, "skipped_reason": "no alternative media"}
	}
	if counter != nil && ptRow(ptNormalizeMap(counter)).intOr("count", 0) >= config.MaxReplaces {
		return map[string]any{"replaced": false, "skipped_reason": "auto-replace limit reached"}
	}

	documentID := pub.strOr("document_id", "")
	exclude := map[string]bool{}
	original, _ := d.queryRow(ctx, "SELECT media_id FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if original != nil {
		exclude[ptRow(ptNormalizeMap(original)).strOr("media_id", "")] = true
	}
	tried, err := d.queryRows(ctx, "SELECT DISTINCT media_id FROM publications WHERE document_id = $1 AND media_id IS NOT NULL", documentID)
	if err == nil {
		for _, t := range tried {
			exclude[ptRow(ptNormalizeMap(t)).strOr("media_id", "")] = true
		}
	}
	delete(exclude, "")

	var budget *float64
	if pub.has("client_price") {
		v := pub.numOr("client_price")
		if v > 0 {
			budget = &v
		}
	}
	candidates, err := d.mpRecommendMedia(ctx, pub.strOr("media_category", ""), budget, 10, exclude)
	if err != nil || len(candidates) == 0 {
		return map[string]any{"replaced": false, "skipped_reason": "no alternative media"}
	}
	for _, candidate := range candidates {
		created, createErr := d.mpCreatePublicationRecord(ctx, mpCreatePublicationInput{
			BrandID:       brandID,
			UserID:        strPtrOr(pub.strPtrSafe("created_by"), brandID),
			DocumentID:    documentID,
			MediaID:       candidate.ID,
			PublishStatus: pub.strOr("publish_status", "draft"),
			CampaignID:    pub.strPtrSafe("campaign_id"),
		})
		if createErr != nil {
			continue
		}
		_ = d.exec(ctx, "UPDATE publications SET replaces_id = $1 WHERE id = $2", publicationID, created)
		_ = d.exec(ctx, `
			INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id)
			VALUES (gen_random_uuid(), $1, $2, 'publication.auto_replaced', 'publication', $3)`,
			brandID, mpNullable(pub.strPtrSafe("created_by")), publicationID)
		link := "/klien/" + brandID + "/publications"
		d.mpNotifyBrandMembers(ctx, brandID, "publication.auto_replaced", "Artikel dialihkan ke media pengganti",
			"Setelah artikel di "+pub.strOr("media_name", "media")+" tidak bisa diakses lagi, \""+pub.strOr("document_title", "Artikel")+"\" otomatis dialihkan ke "+candidate.Name+" untuk diproses ulang.", &link)
		return map[string]any{
			"replaced":       true,
			"publication_id": created,
			"media_id":       candidate.ID,
			"media_name":     candidate.Name,
		}
	}
	return map[string]any{"replaced": false, "skipped_reason": "no candidate passed the editorial check"}
}

func (m ptRow) strPtrSafe(key string) *string {
	if s, ok := m.str(key); ok && s != "" {
		return &s
	}
	return nil
}

func strPtrOr(p *string, fallback string) string {
	if p == nil || *p == "" {
		return fallback
	}
	return *p
}

func mpNullable(p *string) any {
	if p == nil || *p == "" {
		return nil
	}
	return *p
}

type mpCreatePublicationInput struct {
	BrandID       string
	UserID        string
	DocumentID    string
	MediaID       string
	ConnectionID  string
	PublishStatus string
	CampaignID    *string
	SkipStandard  bool
}

// mpCreatePublicationRecord: port createPublicationRecord (subset yang dipakai
// penggantian artikel: validasi dokumen/media, standard check, insert, lalu
// waiting_partner atau queued + enqueue). Mengembalikan id publikasi baru.
func (d *Deps) mpCreatePublicationRecord(ctx context.Context, in mpCreatePublicationInput) (string, error) {
	doc, err := d.queryRow(ctx, "SELECT id, status, title FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1", in.DocumentID, in.BrandID)
	if err != nil {
		return "", err
	}
	if doc == nil {
		return "", errConflict("Document not found")
	}
	if ptRow(ptNormalizeMap(doc)).strOr("status", "") != "approved" {
		return "", errConflict("Only approved content can be published")
	}

	media, err := d.queryRow(ctx, `
		SELECT m.id, m.name, m.price, m.status, m.partner_id, m.publication_mode, m.integration_type, m.auto_publish, m.website_url,
		       p.status AS partner_status, p.user_id AS partner_user_id, p.auto_publish_enabled,
		       m.partner_hidden,
		       `+dbpkg.MediaIntegrationReadySQL("m")+` AS integration_ready
		FROM media m LEFT JOIN partners p ON p.id = m.partner_id
		WHERE m.id = $1 LIMIT 1`, in.MediaID)
	if err != nil {
		return "", err
	}
	if media == nil {
		return "", errNotFound("Media not found")
	}
	mrow := ptRow(ptNormalizeMap(media))
	if mrow.strOr("status", "") != "active" {
		return "", errConflict("Media is not active")
	}
	if !mrow.boolOr("integration_ready", true) {
		return "", errConflict(dbpkg.MediaNotReadyMessage)
	}
	if mrow.boolOr("partner_hidden", false) {
		return "", errConflict(dbpkg.MediaHiddenByPartnerMessage)
	}

	existing, err := d.queryRow(ctx, "SELECT id FROM publications WHERE document_id = $1 AND media_id = $2 LIMIT 1", in.DocumentID, in.MediaID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "", errConflict("This article is already published to this media")
	}

	connectionID := any(nil)
	if in.ConnectionID != "" {
		connectionID = in.ConnectionID
	} else if mrow.has("partner_id") {
		provider := dbpkg.CMSProviderForIntegration(mrow.strOr("integration_type", "manual"))
		bound, _ := d.queryRow(ctx, dbpkg.CMSConnectionForMediaSQL,
			mrow.anyOrNil("partner_id"), provider, in.MediaID, mrow.strOr("website_url", ""))
		if bound != nil {
			connectionID = ptRow(ptNormalizeMap(bound)).anyOrNil("id")
		}
	}

	if !in.SkipStandard {
		// Best-effort seperti app lama: hanya Conflict (blocked/review) yang
		// menghentikan pembuatan publikasi.
		if modErr := d.mpModerationPrecheck(ctx, in.BrandID, in.DocumentID, in.UserID); modErr != nil {
			return "", modErr
		}
		result, checkErr := d.mpFinalStandardCheck(ctx, in.BrandID, in.DocumentID, in.MediaID)
		if checkErr == nil && result != nil && result["status"] == "fail" {
			failed, _ := result["failed"].([]string)
			return "", errConflict(fmt.Sprintf("Artikel belum memenuhi standar editorial %s: %s. Perbaiki artikel atau jalankan standard check untuk detail.",
				mrow.strOr("name", "media"), strings.Join(mpHead(failed, 3), "; ")))
		}
	}

	pricing := mpMediaPriceFields(mrow.numOr("price"), d.mpGetMarkupPercent(ctx))
	publicationMode := "draft"
	if mrow.strOr("publication_mode", "draft") == "publish" {
		publicationMode = "publish"
	}
	id := uuid.NewString()
	if err := d.exec(ctx, `
		INSERT INTO publications (id, brand_id, document_id, media_id, status, client_price, partner_price, markup_percent,
		  publish_status, connection_id, notes, campaign_id, created_by, order_code)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9, NULL, $10, $11,
		        'ORD-' || to_char(NOW(), 'YYYYMM') || '-' || lpad(nextval('publication_order_seq')::text, 5, '0'))`,
		id, in.BrandID, in.DocumentID, in.MediaID, pricing["client_price"], mrow.numOr("price"), pricing["markup_percent"],
		publicationMode, connectionID, mpNullable(in.CampaignID), in.UserID); err != nil {
		return "", err
	}

	autoPublish := mrow.boolOr("auto_publish", false) && mrow.boolOr("auto_publish_enabled", false)
	if mrow.has("partner_id") && mrow.strOr("partner_status", "") == "active" && !autoPublish {
		_ = d.exec(ctx, "UPDATE publications SET status = 'waiting_partner', updated_at = NOW() WHERE id = $1 AND status = 'pending'", id)
		return id, nil
	}
	_ = d.exec(ctx, "UPDATE publications SET status = 'queued', updated_at = NOW() WHERE id = $1 AND status = 'pending'", id)
	_, _ = d.mpEnqueueJob(ctx, "publication_execution", map[string]any{"publicationId": id, "brandId": in.BrandID}, 3)
	return id, nil
}

// ---------- notifikasi & job ----------

func (d *Deps) mpNotifyBrandMembers(ctx context.Context, brandID, notifType, title, body string, link *string) {
	if err := d.exec(ctx, `
		INSERT INTO notifications (id, user_id, type, title, body, link, publication_id)
		SELECT gen_random_uuid(), bm.user_id, $1, $2, $3, $4, NULL
		FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		WHERE bm.brand_id = $5 AND u.role <> 'admin'`,
		notifType, title, body, link, brandID); err != nil {
		d.logWarn("notify brand gagal", notifType, err)
	}
}

// mpNotifyPartner: kirim notifikasi in-app ke user mitra (best-effort).
func (d *Deps) mpNotifyPartner(ctx context.Context, partnerID, notifType, title, body string, link *string) bool {
	row, err := d.queryRow(ctx, "SELECT user_id FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil || row == nil {
		return false
	}
	userID := ptRow(ptNormalizeMap(row)).strOr("user_id", "")
	if userID == "" {
		return false
	}
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)",
		userID, notifType, title, body, link); err != nil {
		d.logWarn("notify partner gagal", notifType, err)
		return false
	}
	return true
}

// mpExpireMediaNotifications: tandai notifikasi waiting_partner kedaluwarsa.
func (d *Deps) mpExpireMediaNotifications(ctx context.Context, mediaID string) {
	if err := d.exec(ctx, `
		UPDATE notifications SET expired_at = COALESCE(expired_at, NOW())
		WHERE type = 'publication.waiting_partner' AND expired_at IS NULL
		  AND publication_id IN (SELECT id FROM publications WHERE media_id = $1)`, mediaID); err != nil {
		d.logWarn("expire notifikasi gagal", mediaID, err)
	}
}

// mpExpireMediaNotificationsBulk: satu UPDATE untuk banyak media.
func (d *Deps) mpExpireMediaNotificationsBulk(ctx context.Context, mediaIDs []string) {
	if len(mediaIDs) == 0 {
		return
	}
	ph := mpPlaceholders(len(mediaIDs), 1)
	args := make([]any, 0, len(mediaIDs))
	for _, id := range mediaIDs {
		args = append(args, id)
	}
	if err := d.exec(ctx, `
		UPDATE notifications SET expired_at = COALESCE(expired_at, NOW())
		WHERE type = 'publication.waiting_partner' AND expired_at IS NULL
		  AND publication_id IN (SELECT id FROM publications WHERE media_id IN (`+ph+`))`, args...); err != nil {
		d.logWarn("expire notifikasi bulk gagal", len(mediaIDs), err)
	}
}

// mpEnqueueJob: port enqueueJob — attempts default 3 untuk kind *generation*, else 2.
func (d *Deps) mpEnqueueJob(ctx context.Context, kind string, payload map[string]any, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		if strings.Contains(kind, "generation") {
			maxAttempts = 3
		} else {
			maxAttempts = 2
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var id string
	err = d.Pool.QueryRow(ctx,
		"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES (gen_random_uuid(), $1, $2::jsonb, 'pending', 0, $3, NOW()) RETURNING id",
		kind, string(raw), maxAttempts).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// mpEnqueueJobBestEffort: kegagalan antrean tidak menggagalkan aksi.
func (d *Deps) mpEnqueueJobBestEffort(ctx context.Context, kind string, payload map[string]any, maxAttempts int) {
	if _, err := d.mpEnqueueJob(ctx, kind, payload, maxAttempts); err != nil {
		d.logWarn("enqueue job gagal", kind, err)
	}
}

// mpEnqueueSeoRefreshForMedia: minta pemeriksaan metrik SEO/AI untuk satu media
// begitu media dibuat atau disetujui, tanpa menunggu sweep scheduler (≤1 menit).
// Dilewati bila media belum punya website_url (job-nya pasti gagal), bila
// skornya sudah ada, atau bila sudah ada job pending/running. Media yang sudah
// berskor tidak diperiksa ulang otomatis — perintah pemilik produk; untuk media
// yang masih kosong, percobaan ulang dibatasi sekali per hari.
func (d *Deps) mpEnqueueSeoRefreshForMedia(ctx context.Context, mediaID string) {
	if strings.TrimSpace(mediaID) == "" {
		return
	}
	var websiteURL string
	var skip bool
	err := d.Pool.QueryRow(ctx, `
		SELECT COALESCE(m.website_url, ''),
		       (EXISTS (SELECT 1 FROM jobs j
		                WHERE j.kind = 'media_seo_refresh' AND j.status IN ('pending','running')
		                  AND j.payload->>'mediaId' = m.id)
		        OR EXISTS (SELECT 1 FROM media_seo_metrics s
		                   WHERE s.media_id = m.id
		                     AND ((s.da IS NOT NULL AND s.ai_trust_score IS NOT NULL)
		                          OR s.checked_at > NOW() - INTERVAL '1 day')))
		FROM media m WHERE m.id = $1`, mediaID).Scan(&websiteURL, &skip)
	if err != nil || strings.TrimSpace(websiteURL) == "" || skip {
		return
	}
	d.mpEnqueueJobBestEffort(ctx, "media_seo_refresh", map[string]any{"mediaId": mediaID}, 2)
}

// mpWriteRate: parity checkWriteRate — 429 pesan standar.
func (d *Deps) mpWriteRate(ctx context.Context, bucket string) error {
	if err := auth.CheckWriteRate(ctx, d.Pool, bucket, d.Cfg.WriteRateLimitPerMinute); err != nil {
		return adminshared.Err(http.StatusTooManyRequests, "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi.")
	}
	return nil
}

func (d *Deps) logWarn(msg string, detail any, err error) {
	slog.Warn("[platform] "+msg, "detail", detail, "err", err)
}

// mpEmailEventCategory: port EVENT_RULES lib/email/routing.ts (kategori gerbang
// preferensi user). Kosong = event tidak mengirim email.
var mpEmailCategories = map[string]string{
	"media.approved":            "partner",
	"media.rejected":            "partner",
	"media.dewan_pers.approved": "partner",
	"media.dewan_pers.rejected": "partner",
	"publication.auto_replaced": "campaign",
	"ticket.replied":            "partner",
	"ticket.closed":             "partner",
}

// mpRouteEmailDecision: parity routeEmailDecision — matriks event + preferensi.
func (d *Deps) mpRouteEmailDecision(ctx context.Context, event, userID string) string {
	category, ok := mpEmailCategories[event]
	if !ok {
		return ""
	}
	if userID == "" || category == "security" {
		return category
	}
	pref, err := d.queryRow(ctx,
		"SELECT email_security, email_payments, email_campaign, email_partner FROM notification_preferences WHERE user_id = $1 LIMIT 1", userID)
	if err != nil || pref == nil {
		return category
	}
	row := ptRow(ptNormalizeMap(pref))
	col := "email_partner"
	if category == "campaign" {
		col = "email_campaign"
	} else if category == "payments" {
		col = "email_payments"
	}
	if v, has := row[col].(bool); has && !v {
		return ""
	}
	return category
}

// mpEmailSubjects: judul template email (parity templates/index.ts).
var mpEmailSubjects = map[string]string{
	"media.approved":            "Media Anda telah disetujui",
	"media.rejected":            "Media Anda ditolak",
	"media.dewan_pers.approved": "Verifikasi Dewan Pers disetujui",
	"media.dewan_pers.rejected": "Verifikasi Dewan Pers ditolak",
	"publication.auto_replaced": "Artikel Anda diganti otomatis",
	"ticket.replied":            "Balasan baru tiket dukungan",
	"ticket.closed":             "Tiket dukungan ditandai selesai",
}

// mpEmailFollowUp: kirim email transaksional ke mitra (best-effort). Hanya
// event yang ada di matriks routing app lama yang dikirim.
func (d *Deps) mpEmailFollowUp(ctx context.Context, event, recipientUserID, recipientEmail, referenceID string, data map[string]string) {
	if recipientEmail == "" {
		return
	}
	if d.mpRouteEmailDecision(ctx, event, recipientUserID) == "" {
		return
	}
	subject, hasSubject := mpEmailSubjects[event]
	if !hasSubject {
		return
	}
	style := email.LoadStyle(ctx, d.Pool)
	if data == nil {
		data = map[string]string{}
	}
	if strings.TrimSpace(data["nama"]) == "" && recipientUserID != "" {
		if nameRow, _ := d.queryRow(ctx,
			"SELECT COALESCE(display_name, '') AS n FROM users WHERE id = $1 LIMIT 1", recipientUserID); nameRow != nil {
			data["nama"] = nameRow.strOr("n", "")
		}
	}
	tplData := map[string]string{
		"nama":   data["nama"],
		"judul":  mpFirstNonEmpty(data["media_name"], subject),
		"isi":    mpFirstNonEmpty(data["status"], mpFirstNonEmpty(data["reason"], subject)),
		"status": data["status"],
		"jumlah": data["jumlah"],
		"link":   d.absoluteAppURL(data["link"]),
	}
	html := ""
	if s, h, ok := email.RenderEvent(ctx, d.Pool, style, email.AudiencePartner, event, tplData); ok {
		subject, html = s, h
	} else {
		html = mpRenderDetailEmail(style, subject, mpEmailBodyRows(event, data), d.absoluteAppURL(data["link"]), mpEmailLinkLabel(event))
	}
	// Keputusan kanal diambil dari registry pusat: mode event, kelengkapan
	// kontak, gate baca in-app, dedupe per event, dan kuota per jam. Tanpa
	// pemeriksaan ini setiap review media menjadi satu email berbayar, walau
	// mitra sudah membaca kabarnya di aplikasi.
	dedupeKey := event + ":" + referenceID
	decision := notify.ShouldSend(ctx, d.Pool, notify.Request{
		UserID: recipientUserID, Type: event, DedupeKey: dedupeKey, Channel: "email",
	})
	notify.LogDecision(decision, notify.Request{
		UserID: recipientUserID, Type: event, DedupeKey: dedupeKey, Channel: "email",
	})
	if !decision.Send {
		return
	}
	// EnqueueEmail menulis baris email_deliveries; worker existing yang mengirim.
	if _, _, err := email.EnqueueEmail(ctx, d.Pool, email.EnqueueOptions{
		Template: event, RecipientEmail: recipientEmail, UserID: mpStrPtr(recipientUserID),
		ReferenceID: mpStrPtr(referenceID), Event: event, Subject: subject, HTML: html,
		DedupeKey: mpStrPtr(dedupeKey),
	}); err != nil {
		d.logWarn("email follow-up gagal", event, err)
	}
}

func mpStrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func mpEmailLinkLabel(event string) string {
	switch event {
	case "media.rejected":
		return "Perbaiki Media"
	case "media.dewan_pers.rejected":
		return "Perbaiki"
	}
	return "Lihat Media"
}

func mpEmailBodyRows(event string, data map[string]string) [][2]string {
	switch event {
	case "media.approved":
		return [][2]string{{"Media", mpOrDash(data["media_name"])}, {"Domain", mpOrDash(data["domain"])}}
	case "media.rejected":
		return [][2]string{{"Media", mpOrDash(data["media_name"])}, {"Alasan", mpOrDash(data["reason"])}}
	case "media.dewan_pers.approved":
		return [][2]string{{"Media", mpOrDash(data["media_name"])}, {"Sertifikat", mpOrDash(data["cert_no"])}}
	case "media.dewan_pers.rejected":
		return [][2]string{{"Media", mpOrDash(data["media_name"])}, {"Alasan", mpOrDash(data["reason"])}}
	case "publication.auto_replaced":
		return [][2]string{{"Artikel", mpOrDash(data["document_title"])}, {"Media", mpOrDash(data["media_name"])}}
	case "ticket.replied", "ticket.closed":
		return [][2]string{{"Subjek", mpOrDash(data["judul"])}, {"Keterangan", mpOrDash(data["status"])}}
	}
	return nil
}

func mpOrDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

func mpHTMLEscape(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(v)
}

// mpRenderDetailEmail: kartu email standar (judul + rincian + tombol) dari
// package email, jadi logo dan footer brand tidak lagi berbeda antar-domain.
func mpRenderDetailEmail(style email.Style, title string, rows [][2]string, link, linkLabel string) string {
	return email.DetailStyled(style, title, "", rows, link, linkLabel).HTML
}

// mpFirstNonEmpty: nilai pertama yang tidak kosong.
func mpFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// mpWAFollowUp: antre WhatsApp ke mitra (best-effort). Mirip routeWaNotification:
// butuh config aktif + template event + consent nomor WA.
func (d *Deps) mpWAFollowUp(ctx context.Context, event, recipientUserID, referenceID string, data map[string]string) {
	if recipientUserID == "" {
		return
	}
	if _, known := igWADefaultTemplates[event]; !known {
		return
	}
	cfg, err := d.igWALoadConfigRow(ctx)
	if err != nil || cfg == nil || !cfg.boolOr("enabled", false) || !cfg.has("sealed_token") {
		return
	}
	pref, err := d.queryRow(ctx,
		"SELECT wa_number, wa_consent FROM notification_preferences WHERE user_id = $1 LIMIT 1", recipientUserID)
	if err != nil || pref == nil {
		return
	}
	prow := ptRow(ptNormalizeMap(pref))
	if !prow.boolOr("wa_consent", false) {
		return
	}
	target := igWANormalizeNumber(prow.strOr("wa_number", ""), cfg.strOr("country_code", "62"))
	if target == "" {
		return
	}
	// Template diambil dari wa_templates lebih dulu, supaya admin yang
	// mematikan atau menyunting template di UI benar-benar berpengaruh.
	// Sebelumnya pengirim selalu memakai teks hardcoded dan mengabaikan tabel
	// itu, jadi tombol "Nonaktif" di halaman admin tidak menghentikan apa pun.
	body, allowed := d.igWATemplateBody(ctx, event)
	if !allowed {
		return
	}
	for k, v := range data {
		body = strings.ReplaceAll(body, "{"+k+"}", v)
	}
	body = strings.ReplaceAll(body, "{link}", d.absoluteAppURL(data["link"]))
	dedupe := event + ":" + referenceID + ":" + target
	// Gate yang sama seperti email: event tier 3 (mis. ticket.replied) tidak
	// punya mode WA, jadi langsung ditolak di sini tanpa biaya.
	decision := notify.ShouldSend(ctx, d.Pool, notify.Request{
		UserID: recipientUserID, Type: event, DedupeKey: dedupe, Channel: "wa",
	})
	notify.LogDecision(decision, notify.Request{
		UserID: recipientUserID, Type: event, DedupeKey: dedupe, Channel: "wa",
	})
	if !decision.Send {
		return
	}
	_ = d.exec(ctx,
		"INSERT INTO wa_deliveries (id, user_id, target, event, rendered_body, dedupe_key, reference_id, status, attempts) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'queued', 0) ON CONFLICT DO NOTHING",
		recipientUserID, target, event, body, dedupe, referenceID)
}

// absoluteAppURL: link relatif → absolut (dipakai WA yang tak bisa membuka path).
func (d *Deps) absoluteAppURL(link string) string {
	v := strings.TrimSpace(link)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return v
	}
	if !strings.HasPrefix(v, "/") {
		return v
	}
	return d.Cfg.PublicAppURL + v
}

// ---------- HTTP kecil (safe fetch + body) ----------

const mpBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// mpSafeGet: GET dengan SafeClient + batas body 512 KiB. status 0 = gagal.
func mpSafeGet(ctx context.Context, target string, headers map[string]string, timeout time.Duration) (int, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, ""
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := security.SafeClient(timeout).Do(req)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	return resp.StatusCode, string(body)
}

// mpSafePostJSON: POST body mentah dengan SafeClient.
func mpSafePostJSON(ctx context.Context, target, body string, headers map[string]string, timeout time.Duration) (int, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		return 0, ""
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := security.SafeClient(timeout).Do(req)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	return resp.StatusCode, string(text)
}

func mpJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ---------- placeholder & angka ----------

// mpPlaceholders: "$n, $n+1, …" untuk klausa IN (count > 0).
func mpPlaceholders(count, start int) string {
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = "$" + strconv.Itoa(start+i)
	}
	return strings.Join(parts, ", ")
}

func mpIntPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// mpClampInt: batas limit/offset.
func mpClampInt(raw string, def, min, max int) int { return clampInt(raw, def, min, max) }

// mpContextPool: akses pool untuk helper rate limit lintas paket.

// mpModerationPrecheck: parity pre-check moderasi createPublicationRecord
// (best-effort; hanya Conflict yang dilempar).
func (d *Deps) mpModerationPrecheck(ctx context.Context, brandID, documentID, userID string) error {
	doc, err := d.queryRow(ctx, "SELECT title, slug, meta_description FROM content_documents WHERE id = $1 LIMIT 1", documentID)
	if err != nil || doc == nil {
		return nil
	}
	drow := ptRow(ptNormalizeMap(doc))
	version, _ := d.queryRow(ctx, "SELECT content FROM content_versions WHERE document_id = $1 ORDER BY version DESC LIMIT 1", documentID)
	content := ""
	if version != nil {
		content = ptTruncate(ptRow(ptNormalizeMap(version)).strOr("content", ""), 5000)
	}
	result := moderation.ModerateContent(ctx, d.Pool, moderation.Input{
		Title:           mpStrArg(drow, "title"),
		Slug:            mpStrArg(drow, "slug"),
		MetaDescription: mpStrArg(drow, "meta_description"),
		Content:         &content,
		BrandID:         &brandID,
		UserID:          &userID,
	})
	if result.Status == "blocked" {
		return errConflict(moderation.ClientSafeMessage("blocked"))
	}
	if result.Status == "review_required" {
		audit, _ := d.queryRow(ctx,
			"SELECT review_decision FROM content_moderation_audits WHERE entity_type = 'document' AND entity_id = $1 ORDER BY created_at DESC LIMIT 1", documentID)
		approved := audit != nil && ptRow(ptNormalizeMap(audit)).strOr("review_decision", "") == "approved"
		if !approved {
			return errConflict(moderation.ClientSafeMessage("review_required"))
		}
	}
	moderation.RecordAudit(ctx, d.Pool, "document", documentID, result, &brandID)
	return nil
}

func mpStrArg(row ptRow, key string) *string {
	if s, ok := row.str(key); ok {
		return &s
	}
	return nil
}

// mpLinkRel: media.link_rel, diturunkan dari label_sponsor/allow_hyperlink
// untuk baris lama (sama dengan backfill migrasi 304).
func mpLinkRel(row rowMap) string {
	switch v := row.strOr("link_rel", ""); v {
	case "dofollow", "nofollow", "sponsored":
		return v
	}
	if row.strOr("label_sponsor", "tidak_ada") == "ada_sponsor" {
		return "sponsored"
	}
	if !row.boolOr("allow_hyperlink", true) {
		return "nofollow"
	}
	return "dofollow"
}
