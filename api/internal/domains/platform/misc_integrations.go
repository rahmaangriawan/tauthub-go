package platform

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
	"github.com/rahmaangriawan/tauthub-go/internal/seo"
)

// Modul integrasi platform: crawlhub, ga4-configs, gsc-config, wa-*, seo,
// rss-feeds. Semua endpoint isPlatformAdmin → adminGuard tanpa permission.

var (
	igGA4MeasurementRe = regexp.MustCompile(`^G-[A-Z0-9]+$`)
	igRSSSlugRe        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	igWAEventRe        = regexp.MustCompile(`^[a-z0-9_.]+$`)
)

// ---------- crawlhub ----------

const igCrawlhubAPIInfoJSON = `{
  "signature": {"headers": ["x-crawlhub-timestamp", "x-crawlhub-signature"], "format": "sha256=hex(HMAC-SHA256(secret, \"<timestamp>.<rawBody>\"))", "maxSkewSeconds": 300},
  "outbound": [
    {"method": "POST", "path": "/v1/targets", "body": "{ mediaId, websiteUrl, intervalHours?, active? }", "purpose": "Daftarkan atau perbarui satu target media."},
    {"method": "POST", "path": "/v1/targets/bulk", "body": "{ targets: [ ... ] }", "purpose": "Rekonsiliasi harian, maksimum 500 target per panggilan."},
    {"method": "POST", "path": "/v1/targets/:mediaId/refresh", "body": "{ mediaId }", "purpose": "Paksa crawl ulang secepatnya."},
    {"method": "GET", "path": "/v1/targets/:mediaId/status", "body": "(kosong)", "purpose": "Status crawl terakhir satu media."},
    {"method": "POST", "path": "/v1/crawl/test", "body": "{ url }", "purpose": "Uji crawl satu URL tanpa menyentuh database."}
  ],
  "inbound": [
    {"method": "POST", "path": "/v1/internal/crawlhub/results", "purpose": "Crawlhub mengirim hasil crawl ke Writer. Belum dibuat di sisi Writer (pekerjaan berikutnya)."}
  ]
}`

type igChConfig struct {
	BaseURL      string
	HMACSecret   string
	IngestSecret string
}

// crawlStaleDays: umur maksimum salinan crawl_targets sebelum dianggap basi.
// Sama dengan default di brandflow/scheduler.go; CRAWLHUB_STALE_MONTHS lama
// tetap dihormati (×30) supaya env produksi yang sudah terpasang tidak berubah
// diam-diam.
func crawlStaleDays() int {
	if v := clampInt(os.Getenv("CRAWLHUB_STALE_DAYS"), 0, 1, 365); v > 0 {
		return v
	}
	if v := clampInt(os.Getenv("CRAWLHUB_STALE_MONTHS"), 0, 1, 12); v > 0 {
		return v * 30
	}
	return 7
}

func (d *Deps) igChLoadConfig(ctx context.Context) (igChConfig, error) {
	cfg := igChConfig{
		BaseURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("CRAWLHUB_BASE_URL")), "/"),
		HMACSecret:   strings.TrimSpace(os.Getenv("CRAWLHUB_HMAC_SECRET")),
		IngestSecret: strings.TrimSpace(os.Getenv("CRAWLHUB_INGEST_SECRET")),
	}
	row, err := d.queryRow(ctx,
		"SELECT base_url, sealed_hmac_secret, sealed_ingest_secret FROM platform_crawlhub_config WHERE id = 'default' LIMIT 1")
	if err != nil || row == nil {
		return cfg, err
	}
	if base := strings.TrimSpace(row.strOr("base_url", "")); base != "" {
		cfg.BaseURL = strings.TrimRight(base, "/")
	}
	open := func(sealed string, fallback string) string {
		if sealed == "" {
			return fallback
		}
		if plain, uErr := d.unsealT2(sealed); uErr == nil {
			return plain
		}
		return ""
	}
	cfg.HMACSecret = open(row.strOr("sealed_hmac_secret", ""), cfg.HMACSecret)
	cfg.IngestSecret = open(row.strOr("sealed_ingest_secret", ""), cfg.IngestSecret)
	return cfg, nil
}

func (c igChConfig) igConfigured() bool { return c.BaseURL != "" && c.HMACSecret != "" }

func (d *Deps) igChConfigJSON(ctx context.Context) (map[string]any, error) {
	cfg, err := d.igChLoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"base_url":                 cfg.BaseURL,
		"hmac_secret_configured":   cfg.HMACSecret != "",
		"ingest_secret_configured": cfg.IngestSecret != "",
		"configured":               cfg.igConfigured(),
		"api":                      json.RawMessage(igCrawlhubAPIInfoJSON),
	}, nil
}

// igChSignedFetch: tanda tangan `sha256=hex(HMAC(secret, "<ts>.<rawBody>"))`.
func (d *Deps) igChSignedFetch(ctx context.Context, cfg igChConfig, path, method string, body any, timeoutMs int) (any, error) {
	rawBody := ""
	if body != nil {
		b, _ := json.Marshal(body)
		rawBody = string(b)
	}
	timestamp := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(cfg.HMACSecret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, rawBody)))
	digest := hex.EncodeToString(mac.Sum(nil))

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	var reader io.Reader
	if rawBody != "" {
		reader = strings.NewReader(rawBody)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, cfg.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if rawBody != "" {
		req.Header.Set("content-type", "application/json")
	}
	req.Header.Set("x-crawlhub-timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("x-crawlhub-signature", "sha256="+digest)

	resp, err := security.SafeClient(time.Duration(timeoutMs) * time.Millisecond).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, ptTruncate(string(text), 300))
	}
	var parsed any
	if err := json.Unmarshal(text, &parsed); err != nil {
		return nil, fmt.Errorf("respons Crawlhub bukan JSON")
	}
	return parsed, nil
}

// CrawlhubConfig: GET /v1/platform/crawlhub
func (d *Deps) CrawlhubConfig(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	payload, err := d.igChConfigJSON(r.Context())
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// CrawlhubConfigUpdate: PUT /v1/platform/crawlhub
func (d *Deps) CrawlhubConfigUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	baseURL, hasBase := strField(body, "base_url")
	if !hasBase || !ptIsHTTPURL(baseURL) || len(baseURL) > 500 {
		writeErr(w, d, errUnprocessable("Invalid url"))
		return
	}
	hmacSecret, hasHMAC := strField(body, "hmac_secret")
	ingestSecret, hasIngest := strField(body, "ingest_secret")
	if hasHMAC && hmacSecret != "" && (len(hmacSecret) < 8 || len(hmacSecret) > 1000) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasIngest && ingestSecret != "" && (len(ingestSecret) < 8 || len(ingestSecret) > 1000) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	var sealedHMAC, sealedIngest any
	if hasHMAC && hmacSecret != "" {
		sealed, sErr := d.sealT2(hmacSecret)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedHMAC = sealed
	}
	if hasIngest && ingestSecret != "" {
		sealed, sErr := d.sealT2(ingestSecret)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedIngest = sealed
	}
	if err := d.exec(r.Context(),
		`INSERT INTO platform_crawlhub_config (id, base_url, sealed_hmac_secret, sealed_ingest_secret, updated_by, updated_at)
		 VALUES ('default', $1, $2, $3, $4, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   base_url = EXCLUDED.base_url,
		   sealed_hmac_secret = COALESCE(EXCLUDED.sealed_hmac_secret, platform_crawlhub_config.sealed_hmac_secret),
		   sealed_ingest_secret = COALESCE(EXCLUDED.sealed_ingest_secret, platform_crawlhub_config.sealed_ingest_secret),
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		strings.TrimRight(baseURL, "/"), sealedHMAC, sealedIngest, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	payload, err := d.igChConfigJSON(r.Context())
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// CrawlhubAction: POST /v1/platform/crawlhub
func (d *Deps) CrawlhubAction(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	action, _ := strField(body, "action")
	cfg, err := d.igChLoadConfig(r.Context())
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if !cfg.igConfigured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "Crawlhub belum dikonfigurasi"})
		return
	}
	switch action {
	case "test":
		domain, _ := strField(body, "domain")
		if len(domain) < 3 || len(domain) > 500 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		data, fErr := d.igChSignedFetch(r.Context(), cfg, "/v1/crawl/test", http.MethodPost, map[string]any{"url": domain}, 90000)
		if fErr != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": fErr.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": data})
	case "status":
		mediaID, _ := strField(body, "mediaId")
		if mediaID == "" || len(mediaID) > 64 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		data, fErr := d.igChSignedFetch(r.Context(), cfg, "/v1/targets/"+url.PathEscape(mediaID)+"/status", http.MethodGet, nil, 15000)
		if fErr != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": fErr.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "target": data})
	case "sync":
		mediaID, _ := strField(body, "mediaId")
		if mediaID == "" {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		outcome := d.igChSyncCrawlTarget(r.Context(), cfg, mediaID)
		if !outcome.OK {
			writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": outcome.Error})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "articles_7d": outcome.Articles7d})
	case "crawl_now":
		mediaID, _ := strField(body, "mediaId")
		if mediaID == "" || len(mediaID) > 64 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		// Crawl Ulang per baris: coba ukur sendiri dulu (WP API/feed/sitemap/
		// Google News/Wayback). Untuk media yang diblokir WAF, ini sering
		// satu-satunya jalur yang menghasilkan angka; Crawlhub baru dipanggil
		// bila semuanya gagal.
		if website, wErr := d.igChMediaWebsite(r.Context(), mediaID); wErr == nil {
			fb := d.crawlMeasureFallbackWithin(r.Context(), website, crawlFallbackDrainBudget)
			storeErr := d.crawlStoreFallback(r.Context(), mediaID, website, fb)
			if fb.Articles != nil {
				if storeErr != nil {
					// Jangan pernah klaim berhasil kalau angkanya tidak tersimpan:
					// UI akan membaca crawl_targets, bukan respons ini.
					writeJSON(w, http.StatusInternalServerError, map[string]any{
						"ok": false, "error": fmt.Sprintf("terukur %d artikel, tetapi gagal disimpan: %s", *fb.Articles, storeErr.Error()),
					})
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"ok": true, "requested": true, "articles_7d": *fb.Articles, "source": fb.Source,
				})
				return
			}
			if storeErr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"ok": false, "error": "gagal menyimpan status crawl: " + storeErr.Error(),
				})
				return
			}
		}
		if err := d.igChRequestImmediateRecrawl(r.Context(), cfg, mediaID); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "requested": true})
	default:
		mediaID, _ := strField(body, "mediaId")
		if mediaID == "" || len(mediaID) > 64 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		if _, fErr := d.igChSignedFetch(r.Context(), cfg, "/v1/targets/"+url.PathEscape(mediaID)+"/refresh", http.MethodPost, map[string]any{"mediaId": mediaID}, 15000); fErr != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": fErr.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

type igChSyncOutcome struct {
	OK         bool
	Error      string
	Articles7d any
	Registered bool
}

// igChMediaWebsite: ambil website_url media (non-partner atau tanpa website → error).
func (d *Deps) igChMediaWebsite(ctx context.Context, mediaID string) (string, error) {
	row, err := d.queryRow(ctx,
		"SELECT id, website_url FROM media WHERE id = $1 AND partner_id IS NOT NULL AND website_url IS NOT NULL AND TRIM(website_url) <> '' LIMIT 1", mediaID)
	if err != nil {
		return "", err
	}
	if row == nil {
		return "", fmt.Errorf("media tidak ditemukan / tanpa website_url")
	}
	return row.strOr("website_url", ""), nil
}

// igChSyncCrawlTarget: port syncCrawlTarget (register + status + simpan salinan).
func (d *Deps) igChSyncCrawlTarget(ctx context.Context, cfg igChConfig, mediaID string) igChSyncOutcome {
	websiteURL, err := d.igChMediaWebsite(ctx, mediaID)
	if err != nil {
		return igChSyncOutcome{OK: false, Error: err.Error()}
	}
	if _, rErr := d.igChSignedFetch(ctx, cfg, "/v1/targets", http.MethodPost, map[string]any{
		"mediaId": mediaID, "websiteUrl": websiteURL, "active": true,
	}, 15000); rErr != nil {
		d.igChRecordCrawlFailure(ctx, mediaID, websiteURL, rErr.Error())
		return igChSyncOutcome{OK: false, Error: rErr.Error()}
	}
	statusRaw, sErr := d.igChSignedFetch(ctx, cfg, "/v1/targets/"+url.PathEscape(mediaID)+"/status", http.MethodGet, nil, 15000)
	if sErr != nil {
		d.igChRecordCrawlFailure(ctx, mediaID, websiteURL, sErr.Error())
		return igChSyncOutcome{OK: false, Error: sErr.Error()}
	}
	statusObj, _ := statusRaw.(map[string]any)
	lastRun, _ := statusObj["last_run"].(map[string]any)
	var articles7d, confidence, source, lastCrawled, lastStatus any
	if lastRun != nil {
		articles7d = lastRun["articles_7d"]
		confidence = lastRun["confidence"]
		source = lastRun["source"]
	}
	if statusObj != nil {
		lastCrawled = statusObj["last_crawled_at"]
		lastStatus = statusObj["last_status"]
	}
	// Anti-loop: beberapa respons Crawlhub meletakkan waktu crawl di dalam
	// last_run, dan target yang baru terdaftar memang belum pernah di-crawl.
	// Bila tetap kosong, simpan waktu sukses sinkronisasi ini — sweep menambah
	// ulang job pada siklus stale 3 bulan, bukan ulang tiap menit (root cause
	// 41 ribu job crawl_target_sync di production).
	if lastCrawled == nil && lastRun != nil {
		if v, ok := lastRun["last_crawled_at"]; ok && v != nil {
			lastCrawled = v
		}
	}
	if lastCrawled == nil {
		lastCrawled = time.Now()
		if lastStatus == nil || lastStatus == "" {
			lastStatus = "registered"
		}
	}
	if err := d.exec(ctx,
		`INSERT INTO crawl_targets (media_id, website_url, active, articles_7d, confidence, source, last_crawled_at, last_status, consecutive_failures, last_error, updated_at)
		 VALUES ($1, $2, TRUE, $3, $4, $5, $6, $7, 0, NULL, NOW())
		 ON CONFLICT (media_id) DO UPDATE SET
		   website_url = EXCLUDED.website_url,
		   -- Run yang gagal (diblokir WAF, timeout) tidak boleh menghapus angka
		   -- terakhir yang pernah terukur; status & error tetap diperbarui.
		   articles_7d = COALESCE(EXCLUDED.articles_7d, crawl_targets.articles_7d),
		   confidence = COALESCE(EXCLUDED.confidence, crawl_targets.confidence),
		   source = COALESCE(EXCLUDED.source, crawl_targets.source),
		   -- last_crawled_at menandai umur ANGKA, bukan umur percobaan: kalau
		   -- run tidak menghasilkan angka, waktu lama dipertahankan supaya TTL
		   -- di crawl-status tetap tahu angkanya sudah basi.
		   last_crawled_at = CASE
		     WHEN EXCLUDED.articles_7d IS NOT NULL THEN EXCLUDED.last_crawled_at
		     ELSE crawl_targets.last_crawled_at
		   END,
		   last_status = EXCLUDED.last_status,
		   consecutive_failures = 0,
		   last_error = NULL,
		   updated_at = NOW()`,
		mediaID, websiteURL, articles7d, confidence, source, lastCrawled, lastStatus); err != nil {
		return igChSyncOutcome{OK: false, Error: err.Error()}
	}
	// Paksa crawl hanya kalau target belum pernah punya run sama sekali. Kalau
	// run sudah ada tapi angkanya kosong (blocked/no_source), Crawlhub punya
	// retry sendiri — memaksa refresh di setiap sync justru menghajar situs yang
	// sama berulang kali dan memperbesar peluang diblokir WAF-nya.
	if lastRun == nil {
		if _, fErr := d.igChSignedFetch(ctx, cfg, "/v1/targets/"+url.PathEscape(mediaID)+"/refresh", http.MethodPost, map[string]any{"mediaId": mediaID}, 15000); fErr != nil {
			// best-effort: re-crawl gagal akan dicoba lagi nanti
		}
	}
	return igChSyncOutcome{OK: true, Articles7d: articles7d, Registered: true}
}

func (d *Deps) igChRecordCrawlFailure(ctx context.Context, mediaID, websiteURL, message string) {
	_ = d.exec(ctx,
		`INSERT INTO crawl_targets (media_id, website_url, active, consecutive_failures, last_error, updated_at)
		 VALUES ($1, $2, TRUE, 1, $3, NOW())
		 ON CONFLICT (media_id) DO UPDATE SET
		   website_url = EXCLUDED.website_url,
		   consecutive_failures = crawl_targets.consecutive_failures + 1,
		   last_error = EXCLUDED.last_error,
		   updated_at = NOW()`,
		mediaID, websiteURL, ptTruncate(message, 500))
}

// igChRequestImmediateRecrawl: port requestImmediateRecrawl.
func (d *Deps) igChRequestImmediateRecrawl(ctx context.Context, cfg igChConfig, mediaID string) error {
	websiteURL, err := d.igChMediaWebsite(ctx, mediaID)
	if err != nil {
		return err
	}
	if _, rErr := d.igChSignedFetch(ctx, cfg, "/v1/targets", http.MethodPost, map[string]any{
		"mediaId": mediaID, "websiteUrl": websiteURL, "active": true,
	}, 15000); rErr != nil {
		return rErr
	}
	_, refErr := d.igChSignedFetch(ctx, cfg, "/v1/targets/"+url.PathEscape(mediaID)+"/refresh", http.MethodPost, map[string]any{"mediaId": mediaID}, 15000)
	return refErr
}

// CrawlhubTargets: GET /v1/platform/crawlhub/targets
func (d *Deps) CrawlhubTargets(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	filter := strings.TrimSpace(q.Get("filter"))
	if filter == "" {
		filter = "all"
	}
	if !ptContains([]string{"all", "registered", "problem", "stale", "unregistered"}, filter) {
		writeErr(w, d, errUnprocessable("filter harus salah satu dari all, registered, problem, stale, unregistered"))
		return
	}
	limit := clampInt(q.Get("limit"), 20, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	filters := []string{
		"m.partner_id IS NOT NULL",
		"m.status = 'active'",
		"m.website_url IS NOT NULL AND TRIM(m.website_url) <> ''",
	}
	if filter == "registered" {
		filters = append(filters, "ct.media_id IS NOT NULL")
	}
	if filter == "problem" {
		filters = append(filters, "(COALESCE(ct.consecutive_failures, 0) > 0 OR ct.last_status IN ('blocked', 'error'))")
	}
	if filter == "stale" {
		filters = append(filters, fmt.Sprintf("(ct.last_crawled_at IS NULL OR ct.last_crawled_at < NOW() - make_interval(days => %d))", crawlStaleDays()))
	}
	if filter == "unregistered" {
		filters = append(filters, "ct.media_id IS NULL")
	}
	where := strings.Join(filters, " AND ")

	summary, err := d.queryRow(r.Context(),
		`SELECT COUNT(*)::int AS total,
		        COUNT(ct.media_id)::int AS registered,
		        COUNT(*) FILTER (WHERE COALESCE(ct.consecutive_failures, 0) > 0 OR ct.last_status IN ('blocked', 'error'))::int AS failed,
		        COUNT(*) FILTER (WHERE ct.media_id IS NULL)::int AS unregistered
		 FROM media m
		 LEFT JOIN crawl_targets ct ON ct.media_id = m.id
		 WHERE m.partner_id IS NOT NULL AND m.status = 'active'
		   AND m.website_url IS NOT NULL AND TRIM(m.website_url) <> ''`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*)::int AS total FROM media m LEFT JOIN crawl_targets ct ON ct.media_id = m.id WHERE "+where)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// stale: angka dianggap basi bila lewat TTL, atau percobaan terakhir tidak
	// menghasilkan angka (blocked/error/no_source). UI memakainya untuk menandai
	// "(basi)" dan tidak menyimpulkan STAGNAN dari data lama.
	staleExpr := fmt.Sprintf(
		"(ct.last_crawled_at IS NOT NULL AND (ct.last_crawled_at < NOW() - make_interval(days => %d) OR ct.last_status IN ('blocked', 'error', 'no_source'))) AS stale",
		crawlStaleDays(),
	)
	targets, err := d.queryRows(r.Context(),
		`SELECT m.id AS media_id, m.name, m.website_url,
		        ct.articles_7d, ct.confidence, ct.source,
		        ct.last_crawled_at, ct.last_status, ct.consecutive_failures, ct.last_error,
		        (ct.media_id IS NOT NULL) AS registered, `+staleExpr+`
		 FROM media m
		 LEFT JOIN crawl_targets ct ON ct.media_id = m.id
		 WHERE `+where+`
		 ORDER BY ct.last_crawled_at DESC NULLS LAST, m.name ASC
		 LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	pendingRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*)::int AS total FROM jobs WHERE kind = 'crawl_target_sync' AND status IN ('pending', 'running')")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	s := ptRow(ptNormalizeMap(summary))
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"total":        int(s.numOr("total")),
			"registered":   int(s.numOr("registered")),
			"failed":       int(s.numOr("failed")),
			"unregistered": int(s.numOr("unregistered")),
			"pending":      pendingRow.intOr("total", 0),
		},
		"targets": ptRows(targets),
		"total":   countRow.intOr("total", 0),
		"limit":   limit,
		"offset":  offset,
	})
}

// CrawlhubTargetsSync: POST /v1/platform/crawlhub/targets
//
// Sengaja TIDAK memproses job di dalam request HTTP: fallback per media bisa
// memakan 10–15 detik, dan 10 job sekaligus melewati timeout proxy (gejalanya
// "Tidak dapat terhubung ke server API"). Cukup enqueue; worker
// (`crawl_target_sync`) memproses di latar belakang dan UI memantau hasilnya.
func (d *Deps) CrawlhubTargetsSync(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	// Manual sync: ambil media yang belum pernah dicrawl, lama tidak dicrawl,
	// tidak punya angka, atau terakhir blocked/error/no_source. Cooldown 24 jam
	// sengaja dilewati supaya tombol ini benar-benar memaksa percobaan ulang.
	enqueued, err := d.igChEnqueueCrawlTargetSyncs(r.Context(), 50)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enqueued": enqueued})
}

// igChEnqueueCrawlTargetSyncs: port enqueueCrawlTargetSyncs (maks limit).
func (d *Deps) igChEnqueueCrawlTargetSyncs(ctx context.Context, limit int) (int, error) {
	cfg, err := d.igChLoadConfig(ctx)
	if err != nil {
		return 0, err
	}
	if !cfg.igConfigured() {
		return 0, nil
	}
	staleDays := crawlStaleDays()
	candidates, err := d.queryRows(ctx,
		`SELECT m.id FROM media m
		   LEFT JOIN crawl_targets ct ON ct.media_id = m.id
		  WHERE m.partner_id IS NOT NULL
		    AND m.status = 'active'
		    AND m.website_url IS NOT NULL AND TRIM(m.website_url) <> ''
		    AND (
		      ct.media_id IS NULL
		      OR ct.last_crawled_at IS NULL
		      OR ct.articles_7d IS NULL
		      OR COALESCE(ct.consecutive_failures, 0) > 0
		      OR ct.last_status IN ('blocked', 'error', 'no_source')
		      OR ct.last_crawled_at < NOW() - make_interval(days => $1::int)
		    )
		    AND NOT EXISTS (
		      SELECT 1 FROM jobs j
		      WHERE j.kind = 'crawl_target_sync' AND j.status IN ('pending', 'running')
		        AND j.payload->>'mediaId' = m.id
		    )
		  ORDER BY ct.last_crawled_at ASC NULLS FIRST
		  LIMIT $2`, staleDays, limit)
	if err != nil {
		return 0, err
	}
	enqueued := 0
	for _, row := range candidates {
		mediaID := row.strOr("id", "")
		if err := d.exec(ctx,
			"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES (gen_random_uuid(), 'crawl_target_sync', $1::jsonb, 'pending', 0, 3, NOW())",
			ptJSONString(map[string]any{"mediaId": mediaID})); err != nil {
			continue
		}
		enqueued++
	}
	return enqueued, nil
}

// ---------- ga4-configs ----------

func igGA4JSON(row ptRow, brandName any) map[string]any {
	return map[string]any{
		"id":             row.strOr("id", ""),
		"brand_id":       row.strOr("brand_id", ""),
		"brand_name":     brandName,
		"measurement_id": row.strOr("measurement_id", ""),
		"has_api_secret": row.has("api_secret") || row.has("sealed_api_secret"),
		"event_mapping":  row.jsonField("event_mapping"),
		"enabled":        row.boolOr("enabled", false),
		"consent_mode":   ptNullableStr(row, "consent_mode"),
		"created_by":     ptNullableStr(row, "created_by"),
		"created_at":     row.anyOrNil("created_at"),
		"updated_at":     row.anyOrNil("updated_at"),
	}
}

func (d *Deps) igGA4Scoped(r *http.Request, configID string) (ptRow, error) {
	row, err := d.queryRow(r.Context(),
		"SELECT g.*, b.name AS brand_name FROM ga4_configs g LEFT JOIN brands b ON b.id = g.brand_id WHERE g.id = $1 LIMIT 1", configID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("GA4 config not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// GA4ConfigsList: GET /v1/platform/ga4-configs
func (d *Deps) GA4ConfigsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	brandID := strings.TrimSpace(q.Get("brand_id"))
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	clause := ""
	params := []any{}
	if brandID != "" {
		params = append(params, brandID)
		clause = "WHERE g.brand_id = $1"
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM ga4_configs g "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		"SELECT g.*, b.name AS brand_name FROM ga4_configs g LEFT JOIN brands b ON b.id = g.brand_id "+
			clause+" ORDER BY g.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		out = append(out, igGA4JSON(row, ptNullableStr(row, "brand_name")))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ga4_configs": out, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// GA4ConfigCreate: POST /v1/platform/ga4-configs
func (d *Deps) GA4ConfigCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	brandID, _ := strField(body, "brand_id")
	measurementID, _ := strField(body, "measurement_id")
	if brandID == "" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if !igGA4MeasurementRe.MatchString(measurementID) {
		writeErr(w, d, errUnprocessable("measurement_id must look like G-XXXXXXX"))
		return
	}
	apiSecret, hasSecret := strField(body, "api_secret")
	if hasSecret && apiSecret != "" && (len(apiSecret) < 8 || len(apiSecret) > 64) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	eventMapping, hasMapping := rawField(body, "event_mapping")
	enabled, _ := boolField(body, "enabled")
	consentMode := "required"
	if s, hasCM := strField(body, "consent_mode"); hasCM && s != "" {
		if s != "required" && s != "none" {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		consentMode = s
	}
	brand, err := d.queryRow(r.Context(), "SELECT id FROM brands WHERE id = $1 LIMIT 1", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if brand == nil {
		writeErr(w, d, errNotFound("Brand not found"))
		return
	}
	id := uuid.NewString()
	var sealedSecret any
	if hasSecret && apiSecret != "" {
		sealed, sErr := d.sealT2(apiSecret)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedSecret = sealed
	}
	var mappingArg any
	if hasMapping && eventMapping != nil {
		mappingArg = ptJSONString(eventMapping)
	}
	if err := d.exec(r.Context(),
		"INSERT INTO ga4_configs (id, brand_id, measurement_id, api_secret, sealed_api_secret, event_mapping, enabled, consent_mode, created_by) VALUES ($1, $2, $3, NULL, $4, $5, $6, $7, $8)",
		id, brandID, measurementID, sealedSecret, mappingArg, enabled, consentMode, user.ID); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errConflict("Brand already has a GA4 config"))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "ga4.config.created", "ga4_config", id)
	created, err := d.igGA4Scoped(r, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, igGA4JSON(created, ptNullableStr(created, "brand_name")))
}

// GA4ConfigGet: GET /v1/platform/ga4-configs/{configId}
func (d *Deps) GA4ConfigGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.igGA4Scoped(r, chiParam(r, "configId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, igGA4JSON(row, ptNullableStr(row, "brand_name")))
}

// GA4ConfigUpdate: PUT /v1/platform/ga4-configs/{configId}
func (d *Deps) GA4ConfigUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	configID := chiParam(r, "configId")
	if _, err := d.igGA4Scoped(r, configID); err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	measurementID, hasMeasurement := strField(body, "measurement_id")
	if hasMeasurement && !igGA4MeasurementRe.MatchString(measurementID) {
		writeErr(w, d, errUnprocessable("measurement_id must look like G-XXXXXXX"))
		return
	}
	apiSecret, hasSecret := strField(body, "api_secret")
	if hasSecret && apiSecret != "" && (len(apiSecret) < 8 || len(apiSecret) > 64) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	eventMapping, hasMapping := rawField(body, "event_mapping")
	enabled, hasEnabled := boolField(body, "enabled")
	consentMode, hasConsent := strField(body, "consent_mode")
	if hasConsent && consentMode != "required" && consentMode != "none" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	var sealedSecret any
	if hasSecret && apiSecret != "" {
		sealed, sErr := d.sealT2(apiSecret)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedSecret = sealed
	}
	var mappingArg any
	if hasMapping && eventMapping != nil {
		mappingArg = ptJSONString(eventMapping)
	}
	if err := d.exec(r.Context(),
		`UPDATE ga4_configs SET
		   measurement_id = COALESCE($1, measurement_id),
		   sealed_api_secret = COALESCE($2, sealed_api_secret),
		   event_mapping = COALESCE($3, event_mapping),
		   enabled = COALESCE($4, enabled),
		   consent_mode = COALESCE($5, consent_mode),
		   updated_at = NOW()
		 WHERE id = $6`,
		ptNullableArgStr(measurementID, hasMeasurement), sealedSecret, mappingArg,
		ptNullableArgBool(enabled, hasEnabled), ptNullableArgStr(consentMode, hasConsent && consentMode != ""), configID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "ga4.config.updated", "ga4_config", configID)
	row, err := d.igGA4Scoped(r, configID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, igGA4JSON(row, ptNullableStr(row, "brand_name")))
}

// GA4ConfigDelete: DELETE /v1/platform/ga4-configs/{configId}
func (d *Deps) GA4ConfigDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	configID := chiParam(r, "configId")
	if _, err := d.igGA4Scoped(r, configID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM ga4_configs WHERE id = $1", configID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "ga4.config.deleted", "ga4_config", configID)
	writeJSON(w, http.StatusOK, map[string]any{"detail": "GA4 config deleted"})
}

// ---------- gsc-config ----------

// GSCConfigGet: GET /v1/platform/gsc-config
func (d *Deps) GSCConfigGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, _ := d.queryRow(r.Context(),
		"SELECT client_id, redirect_uri, sealed_client_secret FROM platform_gsc_config WHERE id = 'default' LIMIT 1")
	clientID := ""
	if row != nil {
		clientID = row.strOr("client_id", "")
	}
	secretConfigured := false
	if row != nil && row.strOr("sealed_client_secret", "") != "" {
		secretConfigured = true
	}
	if clientID == "" {
		clientID = strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	}
	if !secretConfigured {
		secretConfigured = strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")) != ""
	}
	redirectURI := ""
	if row != nil {
		redirectURI = row.strOr("redirect_uri", "")
	}
	redirectMissing := false
	if redirectURI == "" {
		if base := strings.TrimRight(strings.TrimSpace(d.Cfg.PublicAppURL), "/"); base != "" {
			redirectURI = base + "/api/gsc/callback"
		} else {
			redirectMissing = true
		}
	}
	source := "environment"
	if row != nil && row.strOr("client_id", "") != "" {
		source = "database"
	}
	// Diagnostik admin-only (mirror /v1/platform/oauth/google): alasan rinci
	// kegagalan konfigurasi + field kurang. Endpoint mitra tidak mengirim ini.
	secretDecryptable := false
	configError := ""
	configErrorDetail := ""
	missing := []string{}
	if clientID == "" {
		missing = append(missing, "client_id")
	}
	sealedSecret := ""
	if row != nil {
		sealedSecret = row.strOr("sealed_client_secret", "")
	}
	if sealedSecret != "" {
		if plain, uErr := d.unsealT2(sealedSecret); uErr == nil && plain != "" {
			secretDecryptable = true
		} else {
			configError = "secret_decrypt_failed"
			configErrorDetail = "Client Secret GSC tersimpan tidak dapat di-decrypt (kemungkinan APP_SECRET berubah). Simpan ulang di tab GSC."
		}
	} else if strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")) != "" {
		secretDecryptable = true
	} else {
		missing = append(missing, "client_secret")
	}
	if redirectMissing {
		missing = append(missing, "redirect_uri")
	}
	if configError == "" && len(missing) > 0 {
		configError = "config_incomplete"
		configErrorDetail = "Field kurang: " + strings.Join(missing, ", ") + "."
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"client_id": clientID, "client_secret_configured": secretConfigured,
		"redirect_uri": redirectURI, "source": source,
		"client_secret_decryptable": secretDecryptable,
		"config_error":              configError,
		"config_error_detail":       configErrorDetail,
		"missing_fields":            missing,
	})
}

// GSCConfigPut: PUT /v1/platform/gsc-config
func (d *Deps) GSCConfigPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	redirectURI, _ := strField(body, "redirect_uri")
	if redirectURI == "" || len(redirectURI) > 500 {
		writeErr(w, d, errUnprocessable("Invalid input"))
		return
	}
	clientID, hasClientID := strField(body, "client_id")
	if hasClientID && len(clientID) > 500 {
		writeErr(w, d, errUnprocessable("Invalid input"))
		return
	}
	clientSecret, hasSecret := strField(body, "client_secret")
	if hasSecret && len(clientSecret) > 2000 {
		writeErr(w, d, errUnprocessable("Invalid input"))
		return
	}
	setClientID := hasClientID && clientID != ""
	hasSecretValue := hasSecret && clientSecret != ""

	columns := []string{"id", "redirect_uri", "updated_by"}
	placeholders := []string{"'default'", "$1", "$2"}
	params := []any{redirectURI, user.ID}
	setClauses := []string{"redirect_uri = EXCLUDED.redirect_uri", "updated_by = EXCLUDED.updated_by", "updated_at = NOW()"}
	if setClientID {
		params = append(params, clientID)
		columns = append(columns, "client_id")
		placeholders = append(placeholders, "$"+ptItoa(len(params)))
		setClauses = append(setClauses, "client_id = EXCLUDED.client_id")
	}
	if hasSecretValue {
		sealed, sErr := d.sealT2(clientSecret)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		params = append(params, sealed)
		columns = append(columns, "sealed_client_secret")
		placeholders = append(placeholders, "$"+ptItoa(len(params)))
		setClauses = append(setClauses, "sealed_client_secret = EXCLUDED.sealed_client_secret")
	}
	sql := "INSERT INTO platform_gsc_config (" + strings.Join(columns, ", ") + ") VALUES (" +
		strings.Join(placeholders, ", ") + ") ON CONFLICT (id) DO UPDATE SET " + strings.Join(setClauses, ", ")
	if err := d.exec(r.Context(), sql, params...); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- wa gateway ----------

const igWAFonnteBase = "https://api.fonnte.com"

// Template WA sekarang dipisah per AUDIENS (mitra, klien, admin). Sumber
// kebenaran teks bawaan, label, dan tautan pratinjau ada di paket notify
// (notify/wa_templates.go) supaya pengirim (notify.EnqueueUserWA) dan panel
// admin memakai teks yang sama persis.
//
// Pemetaan event → audiens menentukan tab mana yang menampilkan event itu.
var igWAAudienceEvents = map[string][]string{
	notify.AudiencePartner: {
		"publication.partner_approved",
		"publication.partner_rejected",
		"publication.published",
		"publication.failed",
		"media.approved",
		"media.rejected",
		"withdrawal.requested",
		"withdrawal.processing",
		"withdrawal.paid",
		"withdrawal.rejected",
		"referral.cpa.credited",
		"ticket.replied",
		"ticket.closed",
	},
	notify.AudienceClient: {
		"publication.partner_approved",
		"publication.partner_rejected",
		"publication.published",
		"publication.failed",
	},
	notify.AudienceAdmin: {
		notify.AdminEventMediaSubmitted,
		notify.AdminEventWithdrawalRequested,
		notify.AdminEventTicketCreated,
	},
}

// igWADefaultTemplates: kompatibilitas untuk pemanggil lama yang memakai peta
// datar (audiens mitra + admin). Disusun dari registry notify.
var igWADefaultTemplates = buildIGWADefaultTemplates()

// igWAPreviewLinks: kompatibilitas, idem.
var igWAPreviewLinks = buildIGWAPreviewLinks()

func buildIGWADefaultTemplates() map[string]string {
	out := map[string]string{}
	for _, audience := range notify.Audiences {
		for event, body := range notify.WADefaultTemplates[audience] {
			if _, exists := out[event]; !exists {
				out[event] = body
			}
		}
	}
	return out
}

func buildIGWAPreviewLinks() map[string]string {
	out := map[string]string{}
	for _, audience := range notify.Audiences {
		for event, link := range notify.WAPreviewLinks[audience] {
			if _, exists := out[event]; !exists {
				out[event] = link
			}
		}
	}
	return out
}

// igWAMaskGroupID: sensor ID grup untuk tampilan — sisakan 6 digit terakhir
// dan domain, mis. 120363430927198167@g.us → ••••198167@g.us.
func igWAMaskGroupID(id string) string {
	value := strings.TrimSpace(id)
	if value == "" {
		return ""
	}
	at := strings.Index(value, "@")
	if at <= 0 {
		return "••••"
	}
	local, domain := value[:at], value[at+1:]
	if len(local) > 6 {
		local = local[len(local)-6:]
	}
	return "••••" + local + "@" + domain
}

// igWAGroupTarget: ID grup admin dalam bentuk asli (untuk pengiriman).
//
// Sumber utamanya sealed_admin_group_target (terenkripsi seperti token).
// admin_group_target (plaintext) hanya fallback data lama yang belum sempat
// dienkripsi oleh BackfillWAGroupTarget. Tidak pernah dikembalikan ke klien.
func (d *Deps) igWAGroupTarget(row ptRow) string {
	if row == nil {
		return ""
	}
	if sealed := strings.TrimSpace(row.strOr("sealed_admin_group_target", "")); sealed != "" {
		if plain, err := d.unsealT2(sealed); err == nil {
			return strings.TrimSpace(plain)
		}
		return ""
	}
	return strings.TrimSpace(row.strOr("admin_group_target", ""))
}

// igWAConfigJSON: token dan ID grup tidak pernah dikembalikan — hanya status
// dan hint tersamar, supaya rahasia tidak pernah ada di respons API.
func (d *Deps) igWAConfigJSON(row ptRow) map[string]any {
	if row == nil {
		return map[string]any{
			"configured": false, "token_hint": nil, "sender_name": nil,
			"country_code": "62", "enabled": false, "updated_at": nil,
			"group_configured": false, "admin_group_target_hint": nil,
		}
	}
	group := d.igWAGroupTarget(row)
	var groupHint any
	if group != "" {
		groupHint = igWAMaskGroupID(group)
	}
	return map[string]any{
		"configured":              row.has("sealed_token"),
		"token_hint":              ptNullableStr(row, "token_hint"),
		"sender_name":             ptNullableStr(row, "sender_name"),
		"country_code":            row.strOr("country_code", "62"),
		"group_configured":        group != "",
		"admin_group_target_hint": groupHint,
		"enabled":                 row.boolOr("enabled", false),
		"updated_at":              row.anyOrNil("updated_at"),
	}
}

func (d *Deps) igWALoadConfigRow(ctx context.Context) (ptRow, error) {
	// Kolom grup WAJIB ikut dipilih: admin_group_target (data lama) dan
	// sealed_admin_group_target (data terenkripsi). igWAGroupTarget membaca
	// keduanya; tanpa kolom ini GET /wa/config selalu mengembalikan grup kosong.
	row, err := d.queryRow(ctx,
		"SELECT sealed_token, token_hint, sender_name, country_code, enabled, admin_group_target, sealed_admin_group_target, updated_at FROM platform_wa_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// BackfillWAGroupTarget: enkripsi ID grup admin yang masih tersimpan plaintext
// di kolom lama. Dipanggil sekali saat API start; idempoten (melewati baris
// yang sudah punya nilai terenkripsi atau memang kosong).
func (d *Deps) BackfillWAGroupTarget(ctx context.Context) {
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(admin_group_target, '') AS plain,
		       COALESCE(sealed_admin_group_target, '') AS sealed
		FROM platform_wa_config WHERE id = 'default' LIMIT 1`)
	if err != nil || row == nil {
		return
	}
	plain := strings.TrimSpace(row.strOr("plain", ""))
	if plain == "" || strings.TrimSpace(row.strOr("sealed", "")) != "" {
		return
	}
	sealed, sErr := d.sealT2(plain)
	if sErr != nil {
		slog.Warn("[wa] backfill enkripsi ID grup gagal", "err", sErr)
		return
	}
	if err := d.exec(ctx, `
		UPDATE platform_wa_config
		SET sealed_admin_group_target = $1, admin_group_target = NULL, updated_at = NOW()
		WHERE id = 'default'`, sealed); err != nil {
		slog.Warn("[wa] backfill simpan ID grup terenkripsi gagal", "err", err)
	}
}

// igWASanitizeCountryCode: kode negara bersih; nomor lengkap → "62".
func igWASanitizeCountryCode(raw *string) *string {
	if raw == nil {
		return nil
	}
	value := strings.NewReplacer(" ", "", "(", "", ")", "", "+", "", "-", "", ".", "").Replace(*raw)
	if value == "" {
		return nil
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return nil
		}
	}
	if len(value) >= 8 {
		out := "62"
		return &out
	}
	return &value
}

// WAGatewayConfig: GET /v1/platform/wa/config
func (d *Deps) WAGatewayConfig(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.igWALoadConfigRow(r.Context())
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, d.igWAConfigJSON(row))
}

// WAGatewayConfigUpdate: PUT /v1/platform/wa/config
func (d *Deps) WAGatewayConfigUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	token, hasToken := strField(body, "token")
	if hasToken && token != "" && (len(token) < 8 || len(token) > 1000) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	senderName, hasSender := strField(body, "sender_name")
	if hasSender && len(senderName) > 120 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	var countryCode *string
	if cc, hasCC := strField(body, "country_code"); hasCC {
		if len(cc) > 30 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		countryCode = &cc
	}
	enabled, hasEnabled := boolField(body, "enabled")
	// Tujuan grup admin (opsional). Mengikuti pola token: field kosong =
	// TIDAK DIUBAH. Untuk menghapus grup, klien mengirim
	// clear_admin_group_target=true secara eksplisit.
	groupTarget, hasGroup := strField(body, "admin_group_target")
	groupTarget = strings.TrimSpace(groupTarget)
	clearGroup := false
	if v, hasClear := boolField(body, "clear_admin_group_target"); hasClear {
		clearGroup = v
	}
	// groupMode: 0 = tidak diubah, 1 = diganti, 2 = dihapus.
	groupMode := 0
	var sealedGroup any
	if clearGroup {
		groupMode = 2
	} else if hasGroup && groupTarget != "" {
		// Format ID grup Fonnte: deretan angka, boleh dengan sufiks pembuat
		// (-angka, untuk grup lama), dan berakhiran @g.us.
		if len(groupTarget) > 64 || !regexp.MustCompile(`^[0-9]{8,30}(-[0-9]+)?(@g\.us)?$`).MatchString(groupTarget) {
			writeErr(w, d, errUnprocessable("ID grup WA tidak valid (contoh: 120363012345678901@g.us)"))
			return
		}
		s, sErr := d.sealT2(groupTarget)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedGroup = s
		groupMode = 1
	}

	existing, err := d.igWALoadConfigRow(r.Context())
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	var sealed any
	hint := ""
	if hasToken && token != "" {
		s, sErr := d.sealT2(token)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealed = s
		hint = "••••" + token[len(token)-4:]
	} else if existing != nil {
		hint = existing.strOr("token_hint", "")
	}
	sanitizedCC := igWASanitizeCountryCode(countryCode)
	hasCC := countryCode != nil && sanitizedCC != nil
	effectiveCC := "62"
	if hasCC {
		effectiveCC = *sanitizedCC
	} else if existing != nil && existing.strOr("country_code", "") != "" {
		effectiveCC = existing.strOr("country_code", "62")
	}
	effectiveEnabled := false
	if existing != nil {
		effectiveEnabled = existing.boolOr("enabled", false)
	}
	if hasEnabled {
		effectiveEnabled = enabled
	}
	if err := d.exec(r.Context(),
		`INSERT INTO platform_wa_config (id, sealed_token, token_hint, sender_name, country_code, enabled, admin_group_target, sealed_admin_group_target, updated_by, updated_at)
		 VALUES ('default', $1, $2, $3, $4, $5, NULL, $6, $7, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   sealed_token = COALESCE(EXCLUDED.sealed_token, platform_wa_config.sealed_token),
		   token_hint = COALESCE(EXCLUDED.token_hint, platform_wa_config.token_hint),
		   sender_name = COALESCE(EXCLUDED.sender_name, platform_wa_config.sender_name),
		   country_code = COALESCE(EXCLUDED.country_code, platform_wa_config.country_code),
		   enabled = COALESCE(EXCLUDED.enabled, platform_wa_config.enabled),
		   admin_group_target = CASE $8 WHEN 1 THEN NULL WHEN 2 THEN NULL ELSE platform_wa_config.admin_group_target END,
		   sealed_admin_group_target = CASE $8 WHEN 1 THEN $6 WHEN 2 THEN NULL ELSE platform_wa_config.sealed_admin_group_target END,
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		sealed, nullIfEmpty(hint), ptNullableArgStr(senderName, hasSender && senderName != ""),
		effectiveCC, effectiveEnabled, sealedGroup, user.ID, groupMode); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	d.audit(r.Context(), user.ID, "platform.wa.configuration_updated", "platform_wa_config", "default")
	row, err := d.igWALoadConfigRow(r.Context())
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, d.igWAConfigJSON(row))
}

// WADeliveries: GET /v1/platform/wa/deliveries
func (d *Deps) WADeliveries(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	event := strings.TrimSpace(q.Get("event"))
	search := strings.TrimSpace(q.Get("q"))
	days := clampInt(q.Get("days"), 7, 0, 1<<30)
	if !ptContains([]string{"1", "7", "30", "0"}, ptItoa(days)) {
		days = 7
	}
	page := clampInt(q.Get("page"), 1, 1, 1<<30)
	pageSize := clampInt(q.Get("page_size"), 20, 1, 100)

	periodClause := []string{}
	periodParams := []any{}
	if days > 0 {
		periodParams = append(periodParams, time.Now().Add(-time.Duration(days)*24*time.Hour))
		periodClause = append(periodClause, "created_at >= $"+ptItoa(len(periodParams)))
	}
	periodWhere := ""
	if len(periodClause) > 0 {
		periodWhere = " WHERE " + strings.Join(periodClause, " AND ")
	}
	summaryRow, err := d.queryRow(r.Context(),
		`SELECT COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'sent') AS sent,
		        COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		        COUNT(*) FILTER (WHERE status = 'queued') AS queued
		 FROM wa_deliveries`+periodWhere, periodParams...)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	total := summaryRow.intOr("total", 0)
	sent := summaryRow.intOr("sent", 0)
	failed := summaryRow.intOr("failed", 0)
	queued := summaryRow.intOr("queued", 0)
	settled := sent + failed

	byEventRows, err := d.queryRows(r.Context(),
		`SELECT event, COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'sent') AS sent,
		        COUNT(*) FILTER (WHERE status = 'failed') AS failed
		 FROM wa_deliveries`+periodWhere+`
		 GROUP BY event ORDER BY total DESC`, periodParams...)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	topErrorRows, err := d.queryRows(r.Context(),
		`SELECT last_error, COUNT(*) AS total FROM wa_deliveries
		 WHERE status = 'failed' AND last_error IS NOT NULL AND last_error <> ''`+igPeriodAndClause(periodClause)+`
		 GROUP BY last_error ORDER BY total DESC LIMIT 5`, periodParams...)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}

	filters := append([]string{}, periodClause...)
	filterParams := append([]any{}, periodParams...)
	if status == "sent" || status == "failed" || status == "queued" {
		filterParams = append(filterParams, status)
		filters = append(filters, "status = $"+ptItoa(len(filterParams)))
	}
	if event != "" {
		filterParams = append(filterParams, event)
		filters = append(filters, "event = $"+ptItoa(len(filterParams)))
	}
	if search != "" {
		like := "%" + search + "%"
		filterParams = append(filterParams, like, like, like)
		filters = append(filters, "(target LIKE $"+ptItoa(len(filterParams)-2)+" OR last_error LIKE $"+ptItoa(len(filterParams)-1)+" OR rendered_body LIKE $"+ptItoa(len(filterParams))+")")
	}
	where := ""
	if len(filters) > 0 {
		where = " WHERE " + strings.Join(filters, " AND ")
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS total FROM wa_deliveries"+where, filterParams...)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	listArgs := append([]any{}, filterParams...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	items, err := d.queryRows(r.Context(),
		`SELECT id, event, target, status, attempts, last_error, reference_id, rendered_body, created_at, sent_at
		 FROM wa_deliveries`+where+`
		 ORDER BY created_at DESC LIMIT $`+ptItoa(len(filterParams)+1)+` OFFSET $`+ptItoa(len(filterParams)+2), listArgs...)
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}

	var successRate any
	if settled > 0 {
		successRate = ptRoundInt(float64(sent) / float64(settled) * 100)
	}
	byEventOut := make([]map[string]any, 0, len(byEventRows))
	for _, raw := range byEventRows {
		row := ptRow(ptNormalizeMap(raw))
		ev := row.strOr("event", "")
		byEventOut = append(byEventOut, map[string]any{
			"event": ev, "label": igWAEventLabelOr(ev), "total": int(row.numOr("total")),
			"sent": int(row.numOr("sent")), "failed": int(row.numOr("failed")),
		})
	}
	topErrorsOut := make([]map[string]any, 0, len(topErrorRows))
	for _, raw := range topErrorRows {
		row := ptRow(ptNormalizeMap(raw))
		topErrorsOut = append(topErrorsOut, map[string]any{
			"error": row.strOr("last_error", ""), "total": int(row.numOr("total")),
		})
	}
	itemsOut := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		ev := row.strOr("event", "")
		itemsOut = append(itemsOut, map[string]any{
			"id": row.strOr("id", ""), "event": ev, "label": igWAEventLabelOr(ev),
			"target": row.strOr("target", ""), "status": row.strOr("status", ""),
			"attempts": int(row.numOr("attempts")), "last_error": ptNullableStr(row, "last_error"),
			"reference_id": ptNullableStr(row, "reference_id"), "body": row.strOr("rendered_body", ""),
			"created_at": row.anyOrNil("created_at"), "sent_at": row.anyOrNil("sent_at"),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"total": total, "sent": sent, "failed": failed, "queued": queued, "success_rate": successRate,
		},
		"by_event": byEventOut, "top_errors": topErrorsOut, "items": itemsOut,
		"total_filtered": countRow.intOr("total", 0), "page": page, "page_size": pageSize, "days": days,
	})
}

func igPeriodAndClause(periodClause []string) string {
	if len(periodClause) == 0 {
		return ""
	}
	return " AND " + strings.Join(periodClause, " AND ")
}

// igWAEventLabelOr: label event untuk tampilan monitoring. Monitoring tidak
// memisahkan audiens, jadi label diambil dari audiens mana pun yang punya event
// itu (mitra didahulukan, karena sebagian besar event adalah milik mitra).
func igWAEventLabelOr(event string) string {
	for _, audience := range notify.Audiences {
		if notify.WAKnownEvent(audience, event) {
			return notify.WALabel(audience, event)
		}
	}
	return event
}

// igWAEventWired: true bila ada kode pengirim yang benar-benar mengantrekan WA
// untuk event ini. Dipakai panel admin untuk menandai template yang sudah
// tersambung dan mana yang masih menunggu pengirim.
//
// Daftar ini sengaja eksplisit: menambah event baru di registry tanpa
// menyambungkan pengirim akan tampil sebagai "Belum aktif", bukan diam-diam
// terlihat berfungsi.
var igWAEventWired = func() map[string]bool {
	wired := map[string]bool{}
	// Mitra: media.* dan ticket.* dikirim oleh platform (mpWAFollowUp),
	// publication.* oleh brandflow, withdrawal.* oleh platform (settlement),
	// referral.cpa.credited oleh job publikasi.
	for _, event := range []string{
		"media.approved", "media.rejected",
		"ticket.replied", "ticket.closed",
		"publication.partner_approved", "publication.partner_rejected",
		"publication.published", "publication.failed",
		"withdrawal.requested", "withdrawal.processing",
		"withdrawal.paid", "withdrawal.rejected",
		"referral.cpa.credited",
	} {
		wired[event] = true
	}
	// Klien: event publikasi dikirim oleh brandflow ke anggota brand.
	for _, event := range []string{
		"publication.partner_approved", "publication.partner_rejected",
		"publication.published", "publication.failed",
	} {
		wired[event] = true
	}
	// Admin: semua event admin punya pengirim (notify.EnqueueAdminsWA).
	wired[notify.AdminEventMediaSubmitted] = true
	wired[notify.AdminEventWithdrawalRequested] = true
	wired[notify.AdminEventTicketCreated] = true
	return wired
}()

// igWARenderTemplate: port renderWaTemplate.
func igWARenderTemplate(body string, data map[string]string) string {
	out := body
	for key, value := range data {
		out = strings.ReplaceAll(out, "{"+key+"}", value)
	}
	return out
}

// WATemplates: GET /v1/platform/wa/templates
//
// Mengembalikan seluruh template untuk SEMUA audiens sekaligus, masing-masing
// dengan field `audience`, `audience_label`, dan `wired` (true bila ada kode
// pengirim yang benar-benar mengantrekan WA untuk event itu). Panel admin
// memakai `wired` untuk menandai template yang belum aktif.
func (d *Deps) WATemplates(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	saved, err := d.queryRows(r.Context(), "SELECT event, audience, body, enabled FROM wa_templates")
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	type savedTemplate struct {
		body    string
		enabled bool
	}
	savedMap := map[string]savedTemplate{}
	for _, raw := range saved {
		row := ptRow(ptNormalizeMap(raw))
		key := row.strOr("event", "") + "|" + notify.NormalizeAudience(row.strOr("audience", ""))
		savedMap[key] = savedTemplate{
			body:    row.strOr("body", ""),
			enabled: row.boolOr("enabled", true),
		}
	}

	total := 0
	for _, audience := range notify.Audiences {
		total += len(igWAAudienceEvents[audience])
	}
	items := make([]map[string]any, 0, total)
	for _, audience := range notify.Audiences {
		for _, event := range igWAAudienceEvents[audience] {
			defaultBody := notify.WADefaultTemplate(audience, event)
			if defaultBody == "" {
				continue
			}
			body := defaultBody
			enabled := true
			customized := false
			if s, ok := savedMap[event+"|"+audience]; ok {
				if strings.TrimSpace(s.body) != "" {
					body = s.body
					customized = true
				}
				enabled = s.enabled
			}
			previewLink := notify.WAPreviewLink(audience, event)
			if previewLink == "" {
				previewLink = "https://app.tauthub.com/"
			}
			preview := igWARenderTemplate(body, map[string]string{
				"nama": "Budi", "judul": "Contoh Artikel Tauthub", "status": "contoh keterangan",
				"jumlah": "Rp 150.000", "link": previewLink,
			})
			items = append(items, map[string]any{
				"event": event, "audience": audience,
				"audience_label": notify.AudienceLabel[audience],
				"body":           body, "enabled": enabled, "customized": customized,
				"label": notify.WALabel(audience, event), "default_body": defaultBody,
				"preview": preview, "wired": igWAEventWired[event],
			})
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// WATemplateSave: PUT /v1/platform/wa/templates
func (d *Deps) WATemplateSave(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	event, _ := strField(body, "event")
	if event == "" || len(event) > 64 || !igWAEventRe.MatchString(event) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	audienceRaw, _ := strField(body, "audience")
	audience := notify.NormalizeAudience(audienceRaw)
	messageBody, hasBody := strField(body, "body")
	if !hasBody || len(messageBody) > 4000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	enabled, hasEnabled := boolField(body, "enabled")
	if !hasEnabled {
		enabled = true
	}
	if !notify.WAKnownEvent(audience, event) {
		writeErr(w, d, errUnprocessable("Event tidak dikenal untuk audiens ini"))
		return
	}
	if err := d.exec(r.Context(),
		`INSERT INTO wa_templates (event, audience, body, enabled, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (event, audience) DO UPDATE SET
		   body = EXCLUDED.body, enabled = EXCLUDED.enabled,
		   updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		event, audience, messageBody, enabled, user.ID); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	d.audit(r.Context(), user.ID, "platform.wa.template_updated", "wa_template", event+":"+audience)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// WATemplateReset: DELETE /v1/platform/wa/templates
func (d *Deps) WATemplateReset(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	event := strings.TrimSpace(r.URL.Query().Get("event"))
	audience := notify.NormalizeAudience(strings.TrimSpace(r.URL.Query().Get("audience")))
	if event == "" || !notify.WAKnownEvent(audience, event) {
		writeErr(w, d, errUnprocessable("Event tidak dikenal untuk audiens ini"))
		return
	}
	if err := d.exec(r.Context(),
		"DELETE FROM wa_templates WHERE event = $1 AND audience = $2", event, audience); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// igWAToken: token Fonnte mentah (server-side).
func (d *Deps) igWAToken(ctx context.Context) (string, error) {
	row, err := d.queryRow(ctx, "SELECT sealed_token FROM platform_wa_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		return "", err
	}
	if row == nil {
		return "", nil
	}
	sealed := row.strOr("sealed_token", "")
	if sealed == "" {
		return "", nil
	}
	plain, uErr := d.unsealT2(sealed)
	if uErr != nil {
		return "", nil
	}
	return plain, nil
}

// igWANormalizeNumber: port normalizeWaNumber.
// igWAIsGroupTarget: true bila target adalah ID grup WhatsApp Fonnte (…@g.us).
// ID grup tidak boleh dilewatkan igWANormalizeNumber karena mengandung karakter
// non-angka dan akan ditolak sebagai nomor tidak valid.
func igWAIsGroupTarget(target string) bool {
	value := strings.TrimSpace(target)
	return len(value) > len("@g.us") && strings.HasSuffix(strings.ToLower(value), "@g.us")
}

func igWANormalizeNumber(raw, countryCode string) string {
	if raw == "" {
		return ""
	}
	value := strings.NewReplacer(" ", "", "(", "", ")", "", "-", "", ".", "", "+", "").Replace(raw)
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	if countryCode == "" {
		countryCode = "62"
	}
	if strings.HasPrefix(value, "0") {
		value = countryCode + value[1:]
	} else if !strings.HasPrefix(value, countryCode) {
		if len(value) <= 11 {
			value = countryCode + value
		}
	}
	if len(value) < 8 || len(value) > 15 {
		return ""
	}
	return value
}

type igWASendResult struct {
	OK        bool
	Detail    string
	MessageID string
}

// igWASendMessage: port sendWaMessage (Fonnte; sukses dari field status body).
func (d *Deps) igWASendMessage(ctx context.Context, target, message string) (igWASendResult, int, error) {
	token, err := d.igWAToken(ctx)
	if err != nil {
		return igWASendResult{}, http.StatusInternalServerError, err
	}
	if token == "" {
		return igWASendResult{}, http.StatusConflict, fmt.Errorf("WA Gateway belum dikonfigurasi oleh Admin.")
	}
	if target == "" {
		return igWASendResult{}, http.StatusConflict, fmt.Errorf("Nomor tujuan WhatsApp kosong.")
	}
	if strings.TrimSpace(message) == "" {
		return igWASendResult{}, http.StatusConflict, fmt.Errorf("Isi pesan WhatsApp kosong.")
	}
	form := url.Values{}
	form.Set("target", target)
	form.Set("message", message)
	form.Set("delay", "2")

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, rErr := http.NewRequestWithContext(reqCtx, http.MethodPost, igWAFonnteBase+"/send", strings.NewReader(form.Encode()))
	if rErr != nil {
		return igWASendResult{}, http.StatusServiceUnavailable, fmt.Errorf("Gateway WhatsApp tidak dapat dihubungi.")
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, doErr := security.SafeClient(15 * time.Second).Do(req)
	if doErr != nil {
		return igWASendResult{}, http.StatusServiceUnavailable, fmt.Errorf("Gateway WhatsApp tidak dapat dihubungi.")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var payload map[string]any
	if jErr := json.Unmarshal(raw, &payload); jErr != nil || payload == nil {
		return igWASendResult{}, http.StatusBadGateway, fmt.Errorf("Respons gateway WhatsApp kosong (malformed).")
	}
	ok := payload["status"] == true || payload["Status"] == true
	detail := "terkirim"
	if !ok {
		detail = "gagal"
	}
	if d, hasDetail := payload["detail"].(string); hasDetail {
		detail = d
	} else if r, hasReason := payload["reason"].(string); hasReason {
		detail = r
	}
	messageID := ""
	if ids, isArr := payload["id"].([]any); isArr && len(ids) > 0 {
		messageID = fmt.Sprintf("%v", ids[0])
	}
	return igWASendResult{OK: ok, Detail: detail, MessageID: messageID}, 0, nil
}

// igWADeviceStatus: port getWaDeviceStatus.
func (d *Deps) igWADeviceStatus(ctx context.Context) (map[string]any, int, error) {
	token, err := d.igWAToken(ctx)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if token == "" {
		return nil, http.StatusConflict, fmt.Errorf("WA Gateway belum dikonfigurasi oleh Admin.")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, rErr := http.NewRequestWithContext(reqCtx, http.MethodPost, igWAFonnteBase+"/device", nil)
	if rErr != nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("Gateway WhatsApp tidak dapat dihubungi.")
	}
	req.Header.Set("Authorization", token)
	resp, doErr := security.SafeClient(15 * time.Second).Do(req)
	if doErr != nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("Gateway WhatsApp tidak dapat dihubungi.")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var payload map[string]any
	if jErr := json.Unmarshal(raw, &payload); jErr != nil || payload == nil {
		return nil, http.StatusBadGateway, fmt.Errorf("Respons gateway WhatsApp kosong (malformed).")
	}
	if payload["status"] != true && payload["Status"] != true {
		reason := "Token tidak valid."
		if r, hasReason := payload["reason"].(string); hasReason {
			reason = r
		}
		return nil, http.StatusConflict, fmt.Errorf("Gateway WhatsApp menolak token: %s", reason)
	}
	deviceStatus := ""
	if s, isString := payload["device_status"].(string); isString {
		deviceStatus = s
	}
	strOf := func(key string) any {
		if s, isString := payload[key].(string); isString {
			return s
		}
		if payload[key] != nil {
			return fmt.Sprintf("%v", payload[key])
		}
		return nil
	}
	return map[string]any{
		"connected":    deviceStatus == "connect",
		"device":       strOf("device"),
		"deviceStatus": strOf("device_status"),
		"quota":        strOf("quota"),
		"package":      strOf("package"),
		"expired":      strOf("expired"),
		"name":         strOf("name"),
	}, 0, nil
}

// WATest: POST /v1/platform/wa/test
func (d *Deps) WATest(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	target, _ := strField(body, "target")
	message, _ := strField(body, "message")
	target = strings.TrimSpace(target)
	// ID grup Fonnte berbentuk "<angka>@g.us" dan bisa lebih panjang dari nomor
	// telepon (grup lama menyertakan id pembuat). Batas 64 cukup untuk keduanya.
	if len(target) > 64 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if len(message) > 1000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	// Tes kirim adalah satu-satunya jalur WA yang tidak lewat antrean, jadi
	// satu-satunya juga yang tidak tercatat. Dua penjagaan di bawah menutup
	// kebocoran kuota yang tidak terlihat: batas laju per admin, dan baris
	// riwayat di wa_deliveries untuk setiap percobaan.
	if err := auth.CheckWriteRate(r.Context(), d.Pool,
		"write:wa_test:"+admin.ID, d.Cfg.WriteRateLimitPerMinute); err != nil {
		writeErr(w, d, adminshared.Err(http.StatusTooManyRequests,
			"Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi."))
		return
	}
	device, status, err := d.igWADeviceStatus(r.Context())
	if err != nil {
		writeJSON(w, status, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	cfgRow, _ := d.igWALoadConfigRow(r.Context())
	countryCode := "62"
	if cfgRow != nil {
		countryCode = cfgRow.strOr("country_code", "62")
	}
	// Uji grup tersimpan: klien tidak pernah menerima ID grup asli, jadi bila
	// target kosong dan use_saved_group=true, ID dibuka di server.
	if target == "" {
		if useSaved, _ := boolField(body, "use_saved_group"); useSaved {
			target = d.igWAGroupTarget(cfgRow)
			if target == "" {
				writeErr(w, d, errUnprocessable("Grup WA admin belum diatur."))
				return
			}
		}
	}
	if target == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "device": device, "detail": "Koneksi gateway berhasil."})
		return
	}
	// Tujuan grup (…@g.us) dikirim apa adanya: igWANormalizeNumber hanya untuk
	// nomor telepon dan akan menolak ID grup karena mengandung karakter non-angka.
	normalized := ""
	if igWAIsGroupTarget(target) {
		normalized = target
	} else {
		normalized = igWANormalizeNumber(target, countryCode)
		if normalized == "" {
			writeErr(w, d, errUnprocessable("Nomor tujuan tidak valid. Contoh: 08123456789"))
			return
		}
	}
	sendMsg := message
	if sendMsg == "" {
		if igWAIsGroupTarget(normalized) {
			sendMsg = "Tes koneksi grup WhatsApp Tauthub berhasil.\n\nWaktu: " + time.Now().Format("02/01/2006 15:04:05")
		} else {
			sendMsg = "Tes koneksi WA Gateway Tauthub berhasil.\n\nWaktu: " + time.Now().Format("02/01/2006 15:04:05")
		}
	}
	result, sendStatus, sendErr := d.igWASendMessage(r.Context(), normalized, sendMsg)
	if sendErr != nil {
		// Catat percobaan yang gagal di level transport juga: Fonnte tetap
		// menghitung percobaan koneksi pada sebagian paket.
		d.igWALogTestSend(r.Context(), admin.ID, normalized, sendMsg, "failed", sendErr.Error())
		writeJSON(w, sendStatus, map[string]any{"ok": false, "detail": sendErr.Error()})
		return
	}
	detail := "Gateway menolak: " + result.Detail
	rowStatus := "failed"
	if result.OK {
		if igWAIsGroupTarget(normalized) {
			detail = "Pesan uji terkirim ke grup " + normalized + "."
		} else {
			detail = "Pesan uji terkirim ke " + normalized + "."
		}
		rowStatus = "sent"
	}
	d.igWALogTestSend(r.Context(), admin.ID, normalized, sendMsg, rowStatus, detail)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": result.OK, "device": device, "target": normalized, "detail": detail,
		"message_id": nullIfEmpty(result.MessageID),
	})
}

// ---------- seo ----------

// SEOConfigGet: GET /v1/platform/seo/config
func (d *Deps) SEOConfigGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT sealed_api_key, api_key_hint, sealed_similarweb_api_key, sealed_serpens_api_key, updated_at, last_da_error, last_da_error_at FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := igSEOJSON(row, seo.LoadConfig(r.Context(), d.Pool).Host)
	// Diagnostik admin-only: apakah API key tersimpan masih dapat dibuka.
	configError := ""
	configErrorDetail := ""
	decryptable := false
	if row != nil && row.strOr("sealed_api_key", "") != "" {
		if plain, uErr := d.unsealT2(row.strOr("sealed_api_key", "")); uErr == nil && plain != "" {
			decryptable = true
		} else {
			configError = "secret_decrypt_failed"
			configErrorDetail = "API key RapidAPI SEO tersimpan tidak dapat di-decrypt (kemungkinan APP_SECRET berubah). Simpan ulang di tab SEO."
		}
	} else {
		configError = "not_configured"
		configErrorDetail = "API key RapidAPI SEO belum dikonfigurasi."
	}
	out["api_key_decryptable"] = decryptable
	out["last_da_error"] = ptNullableStr(row, "last_da_error")
	out["last_da_error_at"] = row.anyOrNil("last_da_error_at")
	out["config_error"] = configError
	out["config_error_detail"] = configErrorDetail
	writeJSON(w, http.StatusOK, out)
}

func igSEOJSON(row ptRow, host string) map[string]any {
	if host == "" {
		host = seo.DefaultHost
	}
	if row == nil {
		return map[string]any{
			"provider": "rapidapi", "host": host,
			"provider_label": seo.ProviderLabel, "provider_url": seo.ProviderPageURL,
			"api_key_configured": false, "api_key_hint": nil,
			"similarweb_api_key_configured": false,
			"similarweb_provider_url":       mpSimilarWebProviderURL,
			"similarweb_host":               mpSimilarWebHost,
			"serpens_api_key_configured":    false, "updated_at": nil,
		}
	}
	// Kredit provider trafik: similarweb_provider_url menunjuk listing RapidAPI
	// yang benar-benar dipanggil backend, supaya admin tahu ke mana berlangganan.
	return map[string]any{
		"provider":                      "rapidapi",
		"host":                          host,
		"provider_label":                seo.ProviderLabel,
		"provider_url":                  seo.ProviderPageURL,
		"api_key_configured":            row.has("sealed_api_key"),
		"api_key_hint":                  ptNullableStr(row, "api_key_hint"),
		"similarweb_api_key_configured": row.has("sealed_similarweb_api_key"),
		"similarweb_provider_url":       mpSimilarWebProviderURL,
		"similarweb_host":               mpSimilarWebHost,
		"serpens_api_key_configured":    row.has("sealed_serpens_api_key"),
		"updated_at":                    row.anyOrNil("updated_at"),
	}
}

// SEOConfigPut: PUT /v1/platform/seo/config
func (d *Deps) SEOConfigPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	apiKey, hasAPIKey := strField(body, "api_key")
	similarweb, hasSimilarweb := strField(body, "similarweb_api_key")
	serpens, hasSerpens := strField(body, "serpens_api_key")
	for _, pair := range []struct {
		v   string
		has bool
	}{{apiKey, hasAPIKey}, {similarweb, hasSimilarweb}, {serpens, hasSerpens}} {
		if pair.has && pair.v != "" && (len(pair.v) < 8 || len(pair.v) > 1000) {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
	}
	existing, err := d.queryRow(r.Context(),
		"SELECT sealed_api_key, api_key_hint, sealed_similarweb_api_key, sealed_serpens_api_key, updated_at FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	existingRow := ptRow(nil)
	if existing != nil {
		existingRow = ptRow(ptNormalizeMap(existing))
	}
	if (existingRow == nil || !existingRow.has("sealed_api_key")) && !(hasAPIKey && apiKey != "") {
		writeErr(w, d, errUnprocessable("RapidAPI key wajib diisi saat pertama kali"))
		return
	}
	var sealed, hint any
	if hasAPIKey && apiKey != "" {
		s, sErr := d.sealT2(apiKey)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealed = s
		hint = "••••" + apiKey[len(apiKey)-4:]
	} else if existingRow != nil {
		hint = existingRow.strOr("api_key_hint", "")
	}
	var sealedSimilarweb, sealedSerpens any
	if hasSimilarweb && similarweb != "" {
		s, sErr := d.sealT2(similarweb)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedSimilarweb = s
	}
	if hasSerpens && serpens != "" {
		s, sErr := d.sealT2(serpens)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedSerpens = s
	}
	if err := d.exec(r.Context(),
		`INSERT INTO platform_seo_config (id, sealed_api_key, api_key_hint, sealed_similarweb_api_key, sealed_serpens_api_key, updated_by, updated_at)
		 VALUES ('default', $1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   sealed_api_key = COALESCE(EXCLUDED.sealed_api_key, platform_seo_config.sealed_api_key),
		   api_key_hint = COALESCE(EXCLUDED.api_key_hint, platform_seo_config.api_key_hint),
		   sealed_similarweb_api_key = COALESCE(EXCLUDED.sealed_similarweb_api_key, platform_seo_config.sealed_similarweb_api_key),
		   sealed_serpens_api_key = COALESCE(EXCLUDED.sealed_serpens_api_key, platform_seo_config.sealed_serpens_api_key),
		   -- Key baru berarti penyedia mungkin sudah beres: buang catatan kegagalan
		   -- supaya pemeriksaan berikutnya mencoba lagi, bukan menunggu jeda 24 jam.
		   last_da_error = CASE WHEN EXCLUDED.sealed_api_key IS NOT NULL THEN NULL ELSE platform_seo_config.last_da_error END,
		   last_da_error_at = CASE WHEN EXCLUDED.sealed_api_key IS NOT NULL THEN NULL ELSE platform_seo_config.last_da_error_at END,
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		sealed, hint, sealedSimilarweb, sealedSerpens, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "platform.seo.configuration_updated", "platform_seo_config", "default")
	updated, err := d.queryRow(r.Context(),
		"SELECT sealed_api_key, api_key_hint, sealed_similarweb_api_key, sealed_serpens_api_key, updated_at FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, igSEOJSON(ptRow(ptNormalizeMap(updated)), seo.LoadConfig(r.Context(), d.Pool).Host))
}

// SEODiagnose: GET /v1/platform/seo/diagnose
func (d *Deps) SEODiagnose(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	domain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain")))
	if domain == "" {
		domain = "app.tauthub.com"
	}
	config, err := d.queryRow(r.Context(),
		"SELECT sealed_api_key, api_key_hint FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if config == nil || !config.has("sealed_api_key") {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "detail": "RapidAPI SEO key belum dikonfigurasi di Admin Konfigurasi."})
		return
	}
	apiKey, uErr := d.unsealT2(config.strOr("sealed_api_key", ""))
	if uErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "detail": "Gagal membuka kunci tersegel: " + uErr.Error()})
		return
	}
	cfg := seo.LoadConfig(r.Context(), d.Pool)
	batch, seoErr := seo.FetchMany(r.Context(), []string{domain}, apiKey, cfg)
	keyHint := ""
	if config.has("api_key_hint") {
		keyHint = config.strOr("api_key_hint", "")
	}
	keyMasked := apiKey[:igMinInt(6, len(apiKey))] + "…" + apiKey[max(0, len(apiKey)-4):]
	if seoErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":              false,
			"provider_status": seoErr.Status,
			"detail":          seo.ErrText(seoErr),
			"api_key_hint":    nullIfEmpty(keyHint),
			"key_terbaca":     keyMasked,
			"domain":          domain,
			"endpoint":        cfg.URL(),
			"payload_raw":     ptTruncate(seoErr.Body, 2000),
		})
		return
	}
	out := map[string]any{
		"ok":              true,
		"provider_status": http.StatusOK,
		"api_key_hint":    nullIfEmpty(keyHint),
		"key_terbaca":     keyMasked,
		"domain":          domain,
		"endpoint":        cfg.URL(),
		"payload_raw":     ptTruncate(batch.Raw, 2000),
	}
	if len(batch.Missing) > 0 {
		out["missing"] = batch.Missing
	}
	for _, m := range batch.Metrics {
		out["da"] = m.DA
		out["pa"] = m.PA
		if m.Spam != nil {
			out["spam_score"] = *m.Spam
		}
		break
	}
	if _, ok := out["da"]; !ok {
		out["ok"] = false
		out["detail"] = "Provider menjawab, tetapi DA/PA untuk domain uji tidak ditemukan di respons."
	}
	writeJSON(w, http.StatusOK, out)
}

// swDiagnoseHint: interpretasi jawaban gateway RapidAPI untuk SimilarWeb.
// 403 "not subscribed" dan key yang tidak dikenal sama-sama menghasilkan pesan
// yang identik, jadi arahannya mencakup keduanya.
func swDiagnoseHint(status int, body string) string {
	switch {
	case status == 0:
		return "Provider tidak dapat dihubungi dari server sekarang (jaringan/timeout)."
	case status >= 200 && status < 300:
		return "Key valid dan berlangganan aktif — pemeriksaan SimilarWeb siap dipakai di panel media."
	case status == 401:
		return "Ditolak 401: key RapidAPI salah atau tidak aktif. Tempel ulang key persis dari Dashboard RapidAPI (tanpa spasi/kutip) di kartu SimilarWeb. Jawaban mentah provider ada di bawah."
	case status == 403 && strings.Contains(body, "not subscribed"):
		return "Key tersimpan tapi RapidAPI bilang \"You are not subscribed to this API.\" Akun pemilik key ini belum berlangganan listing Similar Web API (belchiorarkad). Buka listing-nya di RapidAPI, klik Subscribe, lalu tes lagi. Key tidak perlu diganti kalau memang benar."
	case status == 403:
		return "Ditolak 403: biasanya langganan listing Similar Web API (belchiorarkad) belum aktif untuk key ini. Jawaban mentah provider ada di bawah."
	case status == 404:
		return "Kredensial lolos. Provider hanya menyebut domain uji tidak terdata di SimilarWeb. Media lain tetap bisa diperiksa."
	case status == 429:
		return "Kuota/rate limit provider kosong untuk saat ini — tunggu lalu tes lagi."
	case status == 402:
		return "Kuota langganan RapidAPI habis; perlu aktivasi/top-up di dasbor RapidAPI."
	default:
		return "Provider menjawab status " + strconv.Itoa(status) + ". Periksa pesan raw di bawah."
	}
}

// SEOSimilarWebDiagnose: GET /v1/platform/seo/similarweb-diagnose — ping satu
// kali ke provider SimilarWeb dengan key tersegel supaya admin melihat sebab
// penolakan beneran (berlangganan/kunci salah/kuota) tanpa menyentuh media.
// Tidak menyimpan snapshot apa pun.
func (d *Deps) SEOSimilarWebDiagnose(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	// Validasi key dulu; bila belum ada, jangan panggil provider. Pesan
	// validasi tidak pernah memuat nilai kredensial.
	apiKey, credsErr := d.mpSimilarWebCredentials(r.Context())
	if credsErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "provider_status": nil, "detail": credsErr.Message,
		})
		return
	}
	// Tes memakai endpoint yang sama dengan cek media (/analyticsv1), supaya
	// admin melihat bentuk JSON aslinya dan apakah angka kunjungan terbaca.
	// Satu tes = satu kuota request RapidAPI.
	domain := mpSimilarWebDiagnoseDomain
	status, body := mpSafeGet(r.Context(), mpSimilarWebURL+url.QueryEscape(domain), mpSimilarWebHeaders(apiKey), 15*time.Second)
	detail := swDiagnoseHint(status, body)
	v := map[string]any{
		"ok":              status >= 200 && status < 300,
		"provider_status": status,
		"ket":             detail,
		"domain":          domain,
		"payload_raw":     nullIfEmpty(ptTruncate(body, 3000)),
	}
	if status >= 200 && status < 300 {
		var parsed any
		if json.Unmarshal([]byte(body), &parsed) == nil {
			mapped := mpMapSimilarWebAnalytics(domain, parsed)
			v["visits"] = mapped["visits"]
			v["snapshot_date"] = mapped["snapshot_date"]
			if mapped["visits"] == nil {
				v["ok"] = false
				v["ket"] = "Provider menjawab 200, tetapi angka kunjungan (visits) tidak ditemukan di respons. Kirim jawaban mentah di bawah supaya pemetaannya disesuaikan."
			}
		}
	}
	writeJSON(w, http.StatusOK, v)
}

// ---------- rss feeds ----------

func igRSSJSON(row ptRow) map[string]any {
	installStatus := row.strOr("install_status", "")
	if installStatus == "" {
		installStatus = "pending"
	}
	return map[string]any{
		"id":             row.strOr("id", ""),
		"partner_id":     row.strOr("partner_id", ""),
		"media_id":       ptNullableStr(row, "media_id"),
		"name":           row.strOr("name", ""),
		"slug":           ptNullableStr(row, "slug"),
		"token":          row.strOr("token", ""),
		"status":         row.strOr("status", ""),
		"description":    ptNullableStr(row, "description"),
		"item_status":    row.strOr("item_status", ""),
		"install_status": installStatus,
		"install_error":  ptNullableStr(row, "install_error"),
		"installed_at":   row.anyOrNil("installed_at"),
		"feed_path":      "/feed/partner/" + strings.TrimSpace(row.strOr("token", "")),
		"created_by":     ptNullableStr(row, "created_by"),
		"created_at":     row.anyOrNil("created_at"),
		"updated_at":     row.anyOrNil("updated_at"),
	}
}

func (d *Deps) igRSSScoped(r *http.Request, feedID string) (ptRow, error) {
	row, err := d.queryRow(r.Context(), "SELECT * FROM rss_feeds WHERE id = $1 LIMIT 1", feedID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("RSS feed not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// RSSFeedsList: GET /v1/platform/rss-feeds
func (d *Deps) RSSFeedsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	partnerID := strings.TrimSpace(q.Get("partner_id"))
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "active" && status != "revoked" {
		writeErr(w, d, errUnprocessable("Invalid feed status filter"))
		return
	}
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	params := []any{}
	if partnerID != "" {
		params = append(params, partnerID)
		where = append(where, "f.partner_id = $"+ptItoa(len(params)))
	}
	if status != "" {
		params = append(params, status)
		where = append(where, "f.status = $"+ptItoa(len(params)))
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM rss_feeds f "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		"SELECT f.*, m.name AS media_name FROM rss_feeds f LEFT JOIN media m ON m.id = f.media_id "+
			clause+" ORDER BY f.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	feeds := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		item := igRSSJSON(row)
		item["media_name"] = row.anyOrNil("media_name")
		feeds = append(feeds, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"feeds": feeds, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

func igRSSNewToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return hex.EncodeToString(buf)
}

// igRSSAssertMediaOwnership: parity assertFeedMediaOwnership.
func (d *Deps) igRSSAssertMediaOwnership(r *http.Request, partnerID, mediaID string) error {
	if mediaID == "" {
		return nil
	}
	row, err := d.queryRow(r.Context(), "SELECT id FROM media WHERE id = $1 AND partner_id = $2 LIMIT 1", mediaID, partnerID)
	if err != nil {
		return err
	}
	if row == nil {
		return errConflict("Media does not belong to the feed's partner")
	}
	return nil
}

// RSSFeedCreate: POST /v1/platform/rss-feeds
func (d *Deps) RSSFeedCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	partnerID, _ := strField(body, "partner_id")
	name, _ := strField(body, "name")
	if partnerID == "" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if len(name) < 2 || len(name) > 160 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	mediaID, hasMedia := strField(body, "media_id")
	slug, hasSlug := strField(body, "slug")
	if hasSlug && slug != "" && (len(slug) > 160 || !igRSSSlugRe.MatchString(slug)) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	description, hasDescription := strField(body, "description")
	if hasDescription && len(description) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	itemStatus := "published"
	if s, hasItem := strField(body, "item_status"); hasItem && s != "" {
		if s != "published" && s != "draft" {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		itemStatus = s
	}
	partner, err := d.queryRow(r.Context(), "SELECT id FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if partner == nil {
		writeErr(w, d, errNotFound("Partner not found"))
		return
	}
	if hasMedia && mediaID != "" {
		if err := d.igRSSAssertMediaOwnership(r, partnerID, mediaID); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO rss_feeds (id, partner_id, media_id, name, slug, token, description, item_status, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		id, partnerID, ptNullableArgStr(mediaID, hasMedia && mediaID != ""), name,
		ptNullableArgStr(slug, hasSlug && slug != ""), igRSSNewToken(),
		ptNullableArgStr(description, hasDescription), itemStatus, user.ID); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errConflict("RSS feed name already exists for this partner"))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "rss_feed.created", "rss_feed", id)
	created, err := d.igRSSScoped(r, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, igRSSJSON(created))
}

// RSSFeedGet: GET /v1/platform/rss-feeds/{feedId}
func (d *Deps) RSSFeedGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.igRSSScoped(r, chiParam(r, "feedId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, igRSSJSON(row))
}

// RSSFeedUpdate: PUT /v1/platform/rss-feeds/{feedId}
func (d *Deps) RSSFeedUpdate(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	feedID := chiParam(r, "feedId")
	feed, err := d.igRSSScoped(r, feedID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	if hasName && (len(name) < 2 || len(name) > 160) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	mediaID, hasMedia := strField(body, "media_id")
	description, hasDescription := strField(body, "description")
	if hasDescription && len(description) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	itemStatus, hasItemStatus := strField(body, "item_status")
	if hasItemStatus && itemStatus != "published" && itemStatus != "draft" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	status, hasStatus := strField(body, "status")
	if hasStatus && status != "active" && status != "revoked" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	effectiveMedia := feed.strOr("media_id", "")
	if hasMedia {
		effectiveMedia = mediaID
	}
	if err := d.igRSSAssertMediaOwnership(r, feed.strOr("partner_id", ""), effectiveMedia); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(r.Context(),
		`UPDATE rss_feeds SET
		   name = COALESCE($1, name),
		   media_id = COALESCE($2, media_id),
		   description = COALESCE($3, description),
		   item_status = COALESCE($4, item_status),
		   status = COALESCE($5, status),
		   updated_at = NOW()
		 WHERE id = $6`,
		ptNullableArgStr(name, hasName), ptNullableArgStr(mediaID, hasMedia && mediaID != ""),
		ptNullableArgStr(description, hasDescription), ptNullableArgStr(itemStatus, hasItemStatus && itemStatus != ""),
		ptNullableArgStr(status, hasStatus && status != ""), feedID); err != nil {
		writeErr(w, d, err)
		return
	}
	updated, err := d.igRSSScoped(r, feedID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, igRSSJSON(updated))
}

// RSSFeedDelete: DELETE /v1/platform/rss-feeds/{feedId} (revoke soft).
func (d *Deps) RSSFeedDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	feedID := chiParam(r, "feedId")
	if _, err := d.igRSSScoped(r, feedID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(r.Context(), "UPDATE rss_feeds SET status = 'revoked', updated_at = NOW() WHERE id = $1", feedID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "rss_feed.revoked", "rss_feed", feedID)
	updated, err := d.igRSSScoped(r, feedID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, igRSSJSON(updated))
}

// RSSFeedRotateToken: POST /v1/platform/rss-feeds/{feedId}
func (d *Deps) RSSFeedRotateToken(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	feedID := chiParam(r, "feedId")
	if _, err := d.igRSSScoped(r, feedID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(r.Context(), "UPDATE rss_feeds SET token = $1, updated_at = NOW() WHERE id = $2", igRSSNewToken(), feedID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "rss_feed.token_rotated", "rss_feed", feedID)
	feed, err := d.igRSSScoped(r, feedID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, igRSSJSON(feed))
}

// igMinInt: minimum dua int.
func igMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// igWALogTestSend mencatat tes kirim WA ke wa_deliveries.
//
// Tes kirim tidak lewat antrean, jadi tanpa baris ini tidak ada jejak sama
// sekali bahwa kuota Fonnte terpakai. reference_id diisi user admin yang
// menekan tombol, supaya pemakaian kuota bisa ditelusuri ke orangnya.
func (d *Deps) igWALogTestSend(ctx context.Context, adminUserID, target, message, status, detail string) {
	_, err := d.Pool.Exec(ctx, `
		INSERT INTO wa_deliveries
		  (id, user_id, target, event, rendered_body, status, attempts, last_error, reference_id, sent_at)
		VALUES (gen_random_uuid(), $1, $2, 'wa.test', $3, $4, 1, $5, $1, CASE WHEN $4 = 'sent' THEN now() ELSE NULL END)`,
		nullIfEmpty(adminUserID), target, message, status, igWAClipDetail(detail))
	if err != nil {
		// Best-effort: kegagalan mencatat tidak boleh mengubah hasil tes.
		d.logWarn("catat tes WA gagal", target, err)
	}
}

// igWAClipDetail memotong detail ke panjang kolom last_error.
func igWAClipDetail(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// igWATemplateBody mengambil isi template WA untuk satu event milik MITRA.
//
// Mengembalikan allowed=false kalau admin menonaktifkan template itu, sehingga
// pemanggil berhenti tanpa mengirim apa pun. Kalau belum ada baris di
// wa_templates, dipakai teks bawaan supaya event lama tetap berjalan seperti
// sebelumnya.
func (d *Deps) igWATemplateBody(ctx context.Context, event string) (string, bool) {
	return d.igWATemplateBodyFor(ctx, notify.AudiencePartner, event)
}

// igWATemplateBodyFor: versi sadar-audiens dari igWATemplateBody.
func (d *Deps) igWATemplateBodyFor(ctx context.Context, audience, event string) (string, bool) {
	audience = notify.NormalizeAudience(audience)
	row, err := d.queryRow(ctx,
		"SELECT body, enabled FROM wa_templates WHERE event = $1 AND audience = $2 LIMIT 1", event, audience)
	if err != nil || row == nil {
		// Belum dikustomisasi: pakai bawaan.
		if body := notify.WADefaultTemplate(audience, event); body != "" {
			return body, true
		}
		return "", false
	}
	if !row.boolOr("enabled", true) {
		return "", false
	}
	body := strings.TrimSpace(row.strOr("body", ""))
	if body == "" {
		if def := notify.WADefaultTemplate(audience, event); def != "" {
			return def, true
		}
		return "", false
	}
	return body, true
}
