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
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
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
		   articles_7d = EXCLUDED.articles_7d,
		   confidence = EXCLUDED.confidence,
		   source = EXCLUDED.source,
		   last_crawled_at = EXCLUDED.last_crawled_at,
		   last_status = EXCLUDED.last_status,
		   consecutive_failures = 0,
		   last_error = NULL,
		   updated_at = NOW()`,
		mediaID, websiteURL, articles7d, confidence, source, lastCrawled, lastStatus); err != nil {
		return igChSyncOutcome{OK: false, Error: err.Error()}
	}
	if articles7d == nil || lastCrawled == nil {
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
		filters = append(filters, "COALESCE(ct.consecutive_failures, 0) > 0")
	}
	if filter == "stale" {
		filters = append(filters, "(ct.last_crawled_at IS NULL OR ct.last_crawled_at < NOW() - make_interval(months => 3))")
	}
	if filter == "unregistered" {
		filters = append(filters, "ct.media_id IS NULL")
	}
	where := strings.Join(filters, " AND ")

	summary, err := d.queryRow(r.Context(),
		`SELECT COUNT(*)::int AS total,
		        COUNT(ct.media_id)::int AS registered,
		        COUNT(*) FILTER (WHERE COALESCE(ct.consecutive_failures, 0) > 0)::int AS failed,
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
	targets, err := d.queryRows(r.Context(),
		`SELECT m.id AS media_id, m.name, m.website_url,
		        ct.articles_7d, ct.confidence, ct.source,
		        ct.last_crawled_at, ct.last_status, ct.consecutive_failures, ct.last_error,
		        (ct.media_id IS NOT NULL) AS registered
		 FROM media m
		 LEFT JOIN crawl_targets ct ON ct.media_id = m.id
		 WHERE `+where+`
		 ORDER BY ct.consecutive_failures DESC NULLS LAST, ct.last_crawled_at ASC NULLS FIRST, m.name ASC
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
func (d *Deps) CrawlhubTargetsSync(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	enqueued, err := d.igChEnqueueCrawlTargetSyncs(r.Context(), 20)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	processed := d.igChDrainCrawlJobs(r.Context(), 10)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enqueued": enqueued, "processed": processed})
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
	staleMonths := 3
	if v := strings.TrimSpace(os.Getenv("CRAWLHUB_STALE_MONTHS")); v != "" {
		staleMonths = clampInt(v, 3, 1, 120)
	}
	candidates, err := d.queryRows(ctx,
		`SELECT m.id FROM media m
		   LEFT JOIN crawl_targets ct ON ct.media_id = m.id
		  WHERE m.partner_id IS NOT NULL
		    AND m.status = 'active'
		    AND m.website_url IS NOT NULL AND TRIM(m.website_url) <> ''
		    AND (
		      ct.media_id IS NULL
		      OR ct.last_crawled_at IS NULL
		      OR (COALESCE(ct.consecutive_failures, 0) BETWEEN 1 AND 2
		          AND ct.updated_at < NOW() - INTERVAL '24 hours')
		      OR ct.last_crawled_at < NOW() - make_interval(months => $1::int)
		    )
		    AND (ct.updated_at IS NULL OR ct.updated_at < NOW() - INTERVAL '24 hours')
		    AND NOT EXISTS (
		      SELECT 1 FROM jobs j
		      WHERE j.kind = 'crawl_target_sync' AND j.status IN ('pending', 'running')
		        AND j.payload->>'mediaId' = m.id
		    )
		  ORDER BY ct.last_crawled_at ASC NULLS FIRST
		  LIMIT $2`, staleMonths, limit)
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

// igChDrainCrawlJobs: proses sampai `limit` job crawl_target_sync pending.
func (d *Deps) igChDrainCrawlJobs(ctx context.Context, limit int) int {
	cfg, err := d.igChLoadConfig(ctx)
	if err != nil || !cfg.igConfigured() {
		return 0
	}
	processed := 0
	for i := 0; i < limit; i++ {
		job, jErr := d.queryRow(ctx,
			`SELECT id, payload FROM jobs WHERE kind = 'crawl_target_sync' AND status = 'pending'
			 ORDER BY run_at ASC NULLS FIRST, created_at ASC LIMIT 1`)
		if jErr != nil || job == nil {
			break
		}
		jobID := job.strOr("id", "")
		if _, cErr := d.execRows(ctx, "UPDATE jobs SET status = 'running', started_at = NOW() WHERE id = $1 AND status = 'pending'", jobID); cErr != nil {
			break
		}
		payload := job.jsonField("payload")
		mediaID := ""
		if m, isMap := payload.(map[string]any); isMap {
			if s, isString := m["mediaId"].(string); isString {
				mediaID = s
			}
		}
		outcome := d.igChSyncCrawlTarget(ctx, cfg, mediaID)
		if outcome.OK {
			_ = d.exec(ctx, "UPDATE jobs SET status = 'completed', finished_at = NOW() WHERE id = $1", jobID)
		} else {
			_ = d.exec(ctx,
				"UPDATE jobs SET status = 'failed', last_error = $1, finished_at = NOW() WHERE id = $2",
				ptTruncate(outcome.Error, 1000), jobID)
		}
		processed++
	}
	return processed
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

var igWAEventLabel = map[string]string{
	"publication.partner_approved": "Publikasi disetujui mitra",
	"publication.partner_rejected": "Publikasi ditolak mitra",
	"publication.published":        "Artikel terbit",
	"publication.failed":           "Publikasi gagal",
	"media.approved":               "Media disetujui",
	"media.rejected":               "Media ditolak",
	"withdrawal.requested":         "Pencairan diajukan",
	"withdrawal.processing":        "Pencairan diproses",
	"withdrawal.paid":              "Pencairan dibayar",
	"withdrawal.rejected":          "Pencairan ditolak",
	"referral.cpa.credited":        "Fee referral (CPA) masuk",
}

var igWADefaultTemplates = map[string]string{
	"publication.partner_approved": "Halo {nama}, publikasi \"{judul}\" telah DISETUJUI mitra dan masuk antrean terbit.\n\nDetail: {link}",
	"publication.partner_rejected": "Halo {nama}, publikasi \"{judul}\" ditolak.\n\nAlasan: {status}\nDetail: {link}",
	"publication.published":        "Halo {nama}, artikel \"{judul}\" sudah TERBIT.\n\nLihat: {link}",
	"publication.failed":           "Halo {nama}, publikasi \"{judul}\" gagal diproses.\n\nKeterangan: {status}\nDetail: {link}",
	"media.approved":               "Halo {nama}, media Anda telah disetujui dan siap menerima order.\n\nDetail: {link}",
	"media.rejected":               "Halo {nama}, pengajuan media Anda ditolak.\n\nCatatan: {status}\nDetail: {link}",
	"withdrawal.requested":         "Halo {nama}, permintaan pencairan {jumlah} sudah masuk dan menunggu diproses.\n\nDetail: {link}",
	"withdrawal.processing":        "Halo {nama}, pencairan {jumlah} sedang diproses.\n\nDetail: {link}",
	"withdrawal.paid":              "Halo {nama}, pencairan {jumlah} sudah DIBAYAR.\n\nDetail: {link}",
	"withdrawal.rejected":          "Halo {nama}, pencairan {jumlah} ditolak.\n\nAlasan: {status}\nDetail: {link}",
	"referral.cpa.credited":        "Halo {nama}, fee referral {jumlah} sudah dikreditkan untuk domain {judul}.\n\nDetail: {link}",
}

var igWAPreviewLinks = map[string]string{
	"publication.partner_approved": "https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh",
	"publication.partner_rejected": "https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh",
	"publication.published":        "https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh",
	"publication.failed":           "https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh",
	"media.approved":               "https://app.tauthub.com/mitra/mitra-contoh/media",
	"media.rejected":               "https://app.tauthub.com/mitra/mitra-contoh/media",
	"withdrawal.requested":         "https://app.tauthub.com/mitra/mitra-contoh/settlements",
	"withdrawal.processing":        "https://app.tauthub.com/mitra/mitra-contoh/settlements",
	"withdrawal.paid":              "https://app.tauthub.com/mitra/mitra-contoh/settlements",
	"withdrawal.rejected":          "https://app.tauthub.com/mitra/mitra-contoh/settlements",
	"referral.cpa.credited":        "https://app.tauthub.com/mitra/mitra-contoh/referral",
}

// igWAConfigJSON: token tidak pernah dikembalikan — hanya status + hint.
func igWAConfigJSON(row ptRow) map[string]any {
	if row == nil {
		return map[string]any{"configured": false, "token_hint": nil, "sender_name": nil, "country_code": "62", "enabled": false, "updated_at": nil}
	}
	return map[string]any{
		"configured":   row.has("sealed_token"),
		"token_hint":   ptNullableStr(row, "token_hint"),
		"sender_name":  ptNullableStr(row, "sender_name"),
		"country_code": row.strOr("country_code", "62"),
		"enabled":      row.boolOr("enabled", false),
		"updated_at":   row.anyOrNil("updated_at"),
	}
}

func (d *Deps) igWALoadConfigRow(ctx context.Context) (ptRow, error) {
	row, err := d.queryRow(ctx,
		"SELECT sealed_token, token_hint, sender_name, country_code, enabled, updated_at FROM platform_wa_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return ptRow(ptNormalizeMap(row)), nil
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
	writeJSON(w, http.StatusOK, igWAConfigJSON(row))
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
		`INSERT INTO platform_wa_config (id, sealed_token, token_hint, sender_name, country_code, enabled, updated_by, updated_at)
		 VALUES ('default', $1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   sealed_token = COALESCE(EXCLUDED.sealed_token, platform_wa_config.sealed_token),
		   token_hint = COALESCE(EXCLUDED.token_hint, platform_wa_config.token_hint),
		   sender_name = COALESCE(EXCLUDED.sender_name, platform_wa_config.sender_name),
		   country_code = COALESCE(EXCLUDED.country_code, platform_wa_config.country_code),
		   enabled = COALESCE(EXCLUDED.enabled, platform_wa_config.enabled),
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		sealed, nullIfEmpty(hint), ptNullableArgStr(senderName, hasSender && senderName != ""),
		effectiveCC, effectiveEnabled, user.ID); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	d.audit(r.Context(), user.ID, "platform.wa.configuration_updated", "platform_wa_config", "default")
	row, err := d.igWALoadConfigRow(r.Context())
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, igWAConfigJSON(row))
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

func igWAEventLabelOr(event string) string {
	if label, ok := igWAEventLabel[event]; ok {
		return label
	}
	return event
}

// igWARenderTemplate: port renderWaTemplate.
func igWARenderTemplate(body string, data map[string]string) string {
	out := body
	for key, value := range data {
		out = strings.ReplaceAll(out, "{"+key+"}", value)
	}
	return out
}

// WATemplates: GET /v1/platform/wa/templates
func (d *Deps) WATemplates(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	saved, err := d.queryRows(r.Context(), "SELECT event, body, enabled FROM wa_templates")
	if err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	savedMap := map[string]ptRow{}
	for _, raw := range saved {
		row := ptRow(ptNormalizeMap(raw))
		savedMap[row.strOr("event", "")] = row
	}
	items := make([]map[string]any, 0, len(igWADefaultTemplates))
	for event, defaultBody := range igWADefaultTemplates {
		savedRow := savedMap[event]
		body := defaultBody
		enabled := true
		customized := false
		if savedRow != nil {
			if strings.TrimSpace(savedRow.strOr("body", "")) != "" {
				body = savedRow.strOr("body", "")
				customized = true
			}
			enabled = savedRow.boolOr("enabled", true)
		}
		previewLink := igWAPreviewLinks[event]
		if previewLink == "" {
			previewLink = "https://app.tauthub.com/mitra"
		}
		preview := igWARenderTemplate(body, map[string]string{
			"nama": "Budi", "judul": "Contoh Artikel Tauthub", "status": "contoh keterangan",
			"jumlah": "Rp 150.000", "link": previewLink,
		})
		items = append(items, map[string]any{
			"event": event, "body": body, "enabled": enabled, "customized": customized,
			"label": igWAEventLabelOr(event), "default_body": defaultBody, "preview": preview,
		})
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
	messageBody, hasBody := strField(body, "body")
	if !hasBody || len(messageBody) > 4000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	enabled, hasEnabled := boolField(body, "enabled")
	if !hasEnabled {
		enabled = true
	}
	if _, known := igWADefaultTemplates[event]; !known {
		writeErr(w, d, errUnprocessable("Event tidak dikenal"))
		return
	}
	if err := d.exec(r.Context(),
		`INSERT INTO wa_templates (event, body, enabled, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (event) DO UPDATE SET
		   body = EXCLUDED.body, enabled = EXCLUDED.enabled,
		   updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		event, messageBody, enabled, user.ID); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, err.Error()))
		return
	}
	d.audit(r.Context(), user.ID, "platform.wa.template_updated", "wa_template", event)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// WATemplateReset: DELETE /v1/platform/wa/templates
func (d *Deps) WATemplateReset(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	event := strings.TrimSpace(r.URL.Query().Get("event"))
	if _, known := igWADefaultTemplates[event]; !known || event == "" {
		writeErr(w, d, errUnprocessable("Event tidak dikenal"))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM wa_templates WHERE event = $1", event); err != nil {
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
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	target, _ := strField(body, "target")
	message, _ := strField(body, "message")
	if len(target) > 30 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if len(message) > 1000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
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
	if target == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "device": device, "detail": "Koneksi gateway berhasil."})
		return
	}
	normalized := igWANormalizeNumber(target, countryCode)
	if normalized == "" {
		writeErr(w, d, errUnprocessable("Nomor tujuan tidak valid. Contoh: 08123456789"))
		return
	}
	sendMsg := message
	if sendMsg == "" {
		sendMsg = "Tes koneksi WA Gateway Tauthub berhasil.\n\nWaktu: " + time.Now().Format("02/01/2006 15:04:05")
	}
	result, sendStatus, sendErr := d.igWASendMessage(r.Context(), normalized, sendMsg)
	if sendErr != nil {
		writeJSON(w, sendStatus, map[string]any{"ok": false, "detail": sendErr.Error()})
		return
	}
	detail := "Gateway menolak: " + result.Detail
	if result.OK {
		detail = "Pesan uji terkirim ke " + normalized + "."
	}
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
		"SELECT sealed_api_key, api_key_hint, sealed_similarweb_api_key, sealed_databee_token, sealed_serpens_api_key, updated_at FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := igSEOJSON(row)
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
	out["config_error"] = configError
	out["config_error_detail"] = configErrorDetail
	writeJSON(w, http.StatusOK, out)
}

func igSEOJSON(row ptRow) map[string]any {
	if row == nil {
		return map[string]any{
			"provider": "rapidapi", "host": mpSeoProviderHost,
			"api_key_configured": false, "api_key_hint": nil,
			"similarweb_api_key_configured": false, "databee_token_configured": false,
			"serpens_api_key_configured": false, "updated_at": nil,
		}
	}
	return map[string]any{
		"provider":                      "rapidapi",
		"host":                          mpSeoProviderHost,
		"api_key_configured":            row.has("sealed_api_key"),
		"api_key_hint":                  ptNullableStr(row, "api_key_hint"),
		"similarweb_api_key_configured": row.has("sealed_similarweb_api_key"),
		"databee_token_configured":      row.has("sealed_databee_token"),
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
	databee, hasDatabee := strField(body, "databee_token")
	serpens, hasSerpens := strField(body, "serpens_api_key")
	for _, pair := range []struct {
		v   string
		has bool
	}{{apiKey, hasAPIKey}, {similarweb, hasSimilarweb}, {databee, hasDatabee}, {serpens, hasSerpens}} {
		if pair.has && pair.v != "" && (len(pair.v) < 8 || len(pair.v) > 1000) {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
	}
	existing, err := d.queryRow(r.Context(),
		"SELECT sealed_api_key, api_key_hint, sealed_similarweb_api_key, sealed_databee_token, sealed_serpens_api_key, updated_at FROM platform_seo_config WHERE id = 'default' LIMIT 1")
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
	var sealedSimilarweb, sealedDatabee, sealedSerpens any
	if hasDatabee && databee != "" {
		s, sErr := d.sealT2(databee)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedDatabee = s
	}
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
		`INSERT INTO platform_seo_config (id, sealed_api_key, api_key_hint, sealed_similarweb_api_key, sealed_databee_token, sealed_serpens_api_key, updated_by, updated_at)
		 VALUES ('default', $1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   sealed_api_key = COALESCE(EXCLUDED.sealed_api_key, platform_seo_config.sealed_api_key),
		   api_key_hint = COALESCE(EXCLUDED.api_key_hint, platform_seo_config.api_key_hint),
		   sealed_similarweb_api_key = COALESCE(EXCLUDED.sealed_similarweb_api_key, platform_seo_config.sealed_similarweb_api_key),
		   sealed_databee_token = COALESCE(EXCLUDED.sealed_databee_token, platform_seo_config.sealed_databee_token),
		   sealed_serpens_api_key = COALESCE(EXCLUDED.sealed_serpens_api_key, platform_seo_config.sealed_serpens_api_key),
		   -- Key baru berarti penyedia mungkin sudah beres: buang catatan kegagalan
		   -- supaya pemeriksaan berikutnya mencoba lagi, bukan menunggu jeda 24 jam.
		   last_da_error = CASE WHEN EXCLUDED.sealed_api_key IS NOT NULL THEN NULL ELSE platform_seo_config.last_da_error END,
		   last_da_error_at = CASE WHEN EXCLUDED.sealed_api_key IS NOT NULL THEN NULL ELSE platform_seo_config.last_da_error_at END,
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		sealed, hint, sealedSimilarweb, sealedDatabee, sealedSerpens, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "platform.seo.configuration_updated", "platform_seo_config", "default")
	updated, err := d.queryRow(r.Context(),
		"SELECT sealed_api_key, api_key_hint, sealed_similarweb_api_key, sealed_databee_token, sealed_serpens_api_key, updated_at FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, igSEOJSON(ptRow(ptNormalizeMap(updated))))
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
	reqCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	payload, _ := json.Marshal(map[string]string{"q": domain})
	req, rErr := http.NewRequestWithContext(reqCtx, http.MethodPost, mpSeoProviderURL, strings.NewReader(string(payload)))
	if rErr != nil {
		writeErr(w, d, rErr)
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rapidapi-host", mpSeoProviderHost)
	req.Header.Set("x-rapidapi-key", apiKey)
	resp, doErr := security.SafeClient(15 * time.Second).Do(req)
	if doErr != nil {
		writeErr(w, d, doErr)
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	keyHint := ""
	if config.has("api_key_hint") {
		keyHint = config.strOr("api_key_hint", "")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              resp.StatusCode >= 200 && resp.StatusCode < 300,
		"provider_status": resp.StatusCode,
		"api_key_hint":    nullIfEmpty(keyHint),
		"key_terbaca":     apiKey[:igMinInt(6, len(apiKey))] + "…" + apiKey[max(0, len(apiKey)-4):],
		"domain":          domain,
		"payload_raw":     ptTruncate(string(raw), 2000),
	})
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
		return "Ditolak 401: cek dua-duanya di kartu SimilarWeb — key RapidAPI (di-tempel persis dari Dashboard RapidAPI, tanpa spasi/kutip) dan DataBee Token dari hal listing DataBee. Jawaban mentah provider ada di bawah."
	case status == 403 && strings.Contains(body, "not subscribed"):
		return "Key tersimpan tapi RapidAPI bilang \"You are not subscribed to this API.\" — akun yang punya key ini belum berlangganan listing SimilarWeb (similarweb-api-pro). Buka listing-nya di RapidAPI, klik Subscribe (paket apa pun yang tersedia), lalu tes lagi. Key tidak perlu diganti kalau memang benar."
	case status == 403:
		return "Ditolak 403: biasanya DataBee Token kurang/salah atau langganan belum aktif. Lengkapi DataBee Token di kartu SimilarWeb (dikirim sebagai Authorization: Bearer), lalu tes lagi. Jawaban mentah provider ada di bawah."
	case status == 404:
		return "Kredensial lolos — provider hanya menyebut domain uji (app.tauthub.com) tidak terdata di SimilarWeb. Media lain tetap bisa diperiksa. Anggap tes lolos."
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
	// Validasi dua kredensial dulu; bila ada yang kurang, jangan panggil
	// provider — jelaskan mana yang belum tersimpan. Pesan validasi tidak
	// pernah memuat nilai kredensial.
	apiKey, databee, credsErr := d.mpSimilarWebCredentials(r.Context())
	if credsErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "provider_status": nil, "detail": credsErr.Message,
		})
		return
	}
	// Endpoint country-metadata dipilih untuk diagnose karena ringan; jika
	// kredensial valid, jawaban mentah provider langsung tampil di admin.
	status, body := mpSafeGet(r.Context(), mpSimilarWebDiagnoseURL, mpSimilarWebHeaders(apiKey, databee), 15*time.Second)
	detail := swDiagnoseHint(status, body)
	v := map[string]any{
		"ok":              status >= 200 && status < 300,
		"provider_status": status,
		"ket":             detail,
		"payload_raw":     nullIfEmpty(ptTruncate(body, 800)),
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
