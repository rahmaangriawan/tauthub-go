package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/content"
)

// Port app/v1/{bug-reports,internal/jobs,internal/cron/scheduler,publishing/callbacks,
// publishing/feed.json,tracking/events}. Enam endpoint ini bukan bagian
// kontrol-plane admin: sebagian publik (feed/tracking) dan sebagian dijaga
// secret infrastruktur (cron/job runner).

// ---------- POST /v1/bug-reports ----------

func (d *Deps) BugReportSubmit(w http.ResponseWriter, r *http.Request) {
	user := requireUser(w, r)
	if user == nil {
		return
	}
	body := decodeJSON(r)
	message, _ := strField(body, "message")
	if len([]rune(message)) < 10 {
		writeErr(w, d, errUnprocessable("Pesan minimal 10 karakter"))
		return
	}
	if len([]rune(message)) > 5000 {
		writeErr(w, d, errUnprocessable("String must contain at most 5000 character(s)"))
		return
	}
	brandID, _ := strField(body, "brand_id")
	if len(brandID) > 36 {
		writeErr(w, d, errUnprocessable("String must contain at most 36 character(s)"))
		return
	}
	contextInfo, _ := strField(body, "context_info")
	if len(contextInfo) > 500 {
		writeErr(w, d, errUnprocessable("String must contain at most 500 character(s)"))
		return
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(), `
		INSERT INTO bug_reports (id, user_id, brand_id, message, context_info, status)
		VALUES ($1, $2, $3, $4, $5, 'open')`,
		id, user.ID, nullIfEmpty(brandID), message, nullIfEmpty(contextInfo)); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// ---------- POST /v1/internal/jobs ----------

// InternalJobPull: satu putaran worker — ambil 1 job, proses, tandai selesai/gagal.
// Dijaga JOB_RUNNER_KEY (header x-job-runner-key).
func (d *Deps) InternalJobPull(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(os.Getenv("JOB_RUNNER_KEY"))
	if key == "" || r.Header.Get("x-job-runner-key") != key {
		writeErr(w, d, errForbidden("Forbidden"))
		return
	}
	workerID := strings.TrimSpace(r.Header.Get("x-job-worker-id"))
	if workerID == "" {
		workerID = "worker-" + uuid.NewString()[:8]
	}
	res, err := d.claimAndRunJob(r.Context(), workerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if res == nil {
		writeJSON(w, http.StatusOK, map[string]any{"processed": false})
		return
	}
	payload := map[string]any{"processed": true, "id": res.ID, "kind": res.Kind}
	if res.Failed {
		payload["error"] = "failed"
	}
	writeJSON(w, http.StatusOK, payload)
}

// ---------- GET/POST /v1/internal/cron/scheduler ----------

// InternalCronScheduler: trigger scheduler publikasi (guard CRON_SECRET /
// JOB_RUNNER_KEY). Respons hanya memuat hitungan numerik.
func (d *Deps) InternalCronScheduler(w http.ResponseWriter, r *http.Request) {
	expected := strings.TrimSpace(os.Getenv("CRON_SECRET"))
	if expected == "" {
		expected = strings.TrimSpace(os.Getenv("JOB_RUNNER_KEY"))
	}
	provided := r.Header.Get("x-cron-secret")
	if provided == "" {
		provided = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("authorization"), "Bearer "))
	}
	if expected == "" || provided != expected {
		writeErr(w, d, errForbidden("Forbidden"))
		return
	}
	maxJobs := clampInt(r.URL.Query().Get("max_jobs"), 20, 1, 200)
	if d.RunScheduler == nil {
		// Scheduler penuh belum terpasang: jalankan minimal materialisasi
		// publikasi terjadwal supaya cron tetap berguna.
		result, err := d.materializeScheduledPublications(r.Context(), maxJobs)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, d.RunScheduler(r.Context(), maxJobs, d.Runner))
}

// ---------- GET /v1/publishing/feed.json ----------

// PublishingFeedJSON: JSON pull untuk website custom mitra (token sama dengan
// RSS feed). Rate limit 60/menit per feed via rss_access_logs.
func (d *Deps) PublishingFeedJSON(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		plainNotFound(w)
		return
	}
	feed, err := d.queryRow(ctx, `
		SELECT f.id, f.partner_id, f.media_id, f.name, f.item_status,
		       m.name AS media_name, m.website_url AS media_website_url
		FROM rss_feeds f LEFT JOIN media m ON m.id = f.media_id
		WHERE f.token = $1 AND f.status = 'active' LIMIT 1`, token)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if feed == nil {
		// Token tidak dikenal/dicabut: 404 tanpa membocorkan keberadaan feed.
		plainNotFound(w)
		return
	}
	feedID := feed.strOr("id", "")

	recent, _ := d.queryRow(ctx,
		"SELECT COUNT(*) AS count FROM rss_access_logs WHERE feed_id = $1 AND created_at > NOW() - INTERVAL '60 seconds'",
		feedID)
	if recent.intOr("count", 0) >= 60 {
		d.logFeedAccess(ctx, feedID, r, 429)
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}

	args := []any{feed.strOr("partner_id", "")}
	mediaClause := "TRUE"
	if feed.has("media_id") {
		args = append(args, feed.strOr("media_id", ""))
		mediaClause = "p.media_id = $2"
	}
	statuses := []string{"published"}
	if feed.strOr("item_status", "") == "draft" {
		statuses = []string{"queued", "published"}
	}
	start := len(args)
	ph := make([]string, len(statuses))
	for i, st := range statuses {
		args = append(args, st)
		ph[i] = "$" + itoa(start+i+1)
	}
	args = append(args, 50)

	items, err := d.queryRows(ctx, `
		SELECT p.id AS publication_id, d.title, d.slug, d.meta_title, d.meta_description,
		       v.content, v.content_format,
		       b.primary_language, p.published_at, p.published_url,
		       m.id AS media_id, m.name AS media_name, m.website_url AS media_website_url
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN content_versions v ON v.document_id = d.id AND v.version = d.current_version
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		WHERE m.partner_id = $1 AND `+mediaClause+` AND p.status IN (`+strings.Join(ph, ",")+`)
		ORDER BY p.published_at DESC NULLS LAST LIMIT $`+itoa(len(args)), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		raw := item.strOr("content", "")
		isMarkdown := strings.EqualFold(item.strOr("content_format", "markdown"), "markdown")
		var contentMarkdown, contentHTML any
		if isMarkdown {
			contentMarkdown = raw
			contentHTML = content.MarkdownToHtml(raw)
		} else {
			contentMarkdown = nil
			contentHTML = raw
		}
		out = append(out, map[string]any{
			"publication_id": item.raw("publication_id"),
			"article": map[string]any{
				"title":            item.raw("title"),
				"slug":             item.raw("slug"),
				"content_markdown": contentMarkdown,
				"content_html":     contentHTML,
				"excerpt":          item.raw("meta_description"),
				"meta_title":       item.raw("meta_title"),
				"meta_description": item.raw("meta_description"),
				"language":         item.strOr("primary_language", "id-ID"),
			},
			"media": map[string]any{
				"id":          item.raw("media_id"),
				"name":        item.raw("media_name"),
				"website_url": item.raw("media_website_url"),
			},
			"tracking": map[string]any{
				"publication_id": item.raw("publication_id"),
			},
			"published_url": item.raw("published_url"),
			"published_at":  item.timePtr("published_at"),
		})
	}
	d.logFeedAccess(ctx, feedID, r, 200)
	writeJSON(w, http.StatusOK, map[string]any{
		"feed": map[string]any{
			"id":           feedID,
			"name":         feed.strOr("media_name", feed.strOr("name", "")),
			"media_id":     feed.raw("media_id"),
			"website_url":  feed.raw("media_website_url"),
			"generated_at": isoMilli(time.Now()),
		},
		"items": out,
	})
}

func (d *Deps) logFeedAccess(ctx context.Context, feedID string, r *http.Request, status int) {
	ip := strings.TrimSpace(strings.Split(r.Header.Get("x-forwarded-for"), ",")[0])
	if ip == "" {
		ip = strings.TrimSpace(r.Header.Get("x-real-ip"))
	}
	ua := truncateBytesStr(r.Header.Get("user-agent"), 300)
	if err := d.exec(ctx, `
		INSERT INTO rss_access_logs (id, feed_id, ip_address, user_agent, status)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.NewString(), feedID, nullIfEmpty(ip), ua, status); err != nil {
		d.loggerWarn("log akses feed gagal", err)
	}
}

// ---------- POST /v1/publishing/callbacks ----------

// PublishingCallback: adapter website mitra melaporkan status artikel (two-way
// sync). Auth: Bearer api_key + HMAC atas body (timestamp + signature),
// idempotency key opsional. Ownership: hanya publikasi pada media key itu.
func (d *Deps) PublishingCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, d, errBadRequest("Invalid JSON body"))
		return
	}
	apiKey := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("authorization"), "Bearer "))
	if apiKey == "" {
		writeErr(w, d, errForbidden("Invalid publishing credentials"))
		return
	}
	key, err := d.queryRow(ctx, `
		SELECT id, partner_id, media_id, status, secret_sealed
		FROM publishing_api_keys WHERE api_key_hash = $1 AND status = 'active' LIMIT 1`,
		sha256HexPlatform(apiKey))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if key == nil {
		writeErr(w, d, errForbidden("Invalid publishing credentials"))
		return
	}
	timestamp := strings.TrimSpace(firstNonEmpty(
		r.Header.Get("x-tauthub-timestamp"), r.Header.Get("x-tulisio-timestamp")))
	signature := strings.TrimSpace(firstNonEmpty(
		r.Header.Get("x-tauthub-signature"), r.Header.Get("x-tulisio-signature")))
	if timestamp == "" || signature == "" {
		writeErr(w, d, errForbidden("Missing signature"))
		return
	}
	secret, unsealErr := d.unsealT2(key.strOr("secret_sealed", ""))
	if unsealErr != nil {
		writeErr(w, d, errForbidden("Invalid publishing credentials"))
		return
	}
	ts, parseErr := strconvParseInt(timestamp)
	if parseErr != nil || abs64(time.Now().Unix()-int64(ts)) > 300 {
		writeErr(w, d, errForbidden("Expired signature"))
		return
	}
	expected := hmacSHA256HexPlatform([]byte(secret), timestamp+"."+string(raw))
	if !constantTimeEqualHexPlatform(expected, signature) {
		writeErr(w, d, errForbidden("Invalid signature"))
		return
	}

	var payload struct {
		PublicationID string `json:"publication_id"`
		Status        string `json:"status"`
		ExternalID    string `json:"external_id"`
		URL           string `json:"url"`
		Message       string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		writeErr(w, d, errUnprocessable("Invalid JSON body"))
		return
	}
	if payload.PublicationID == "" {
		writeErr(w, d, errUnprocessable("publication_id wajib diisi"))
		return
	}
	if payload.Status != "published" && payload.Status != "failed" {
		writeErr(w, d, errUnprocessable("Invalid option: expected \"published\"|\"failed\""))
		return
	}

	// Ownership: publikasi harus menargetkan media milik key ini (atau key tanpa media).
	args := []any{payload.PublicationID}
	clause := "p.id = $1"
	if key.has("media_id") {
		args = append(args, key.strOr("media_id", ""))
		clause += " AND p.media_id = $2"
	}
	args = append(args, key.strOr("partner_id", ""))
	publication, err := d.queryRow(ctx, `
		SELECT p.id, p.status, p.media_id FROM publications p
		JOIN media m ON m.id = p.media_id
		WHERE `+clause+` AND m.partner_id = $`+itoa(len(args))+` LIMIT 1`, args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if publication == nil {
		d.recordCallbackLog(ctx, key.strOr("id", ""), payload.PublicationID, "rejected", payload.Status, r, "publication not found")
		writeErr(w, d, errNotFound("Publication not found"))
		return
	}

	if payload.URL != "" {
		if err := d.exec(ctx, "UPDATE publications SET published_url = $1, updated_at = NOW() WHERE id = $2",
			payload.URL, payload.PublicationID); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	d.recordCallbackLog(ctx, key.strOr("id", ""), payload.PublicationID, "accepted", payload.Status, r, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"publication_id": payload.PublicationID,
		"status":         payload.Status,
		"external_id":    nullIfEmpty(payload.ExternalID),
	})
}

func (d *Deps) recordCallbackLog(ctx context.Context, keyID, publicationID, outcome, status string, r *http.Request, errMsg string) {
	ip := strings.TrimSpace(strings.Split(r.Header.Get("x-forwarded-for"), ",")[0])
	if err := d.exec(ctx, `
		INSERT INTO publishing_callback_logs (id, api_key_id, publication_id, outcome, reported_status, ip_address, user_agent, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		uuid.NewString(), nullIfEmpty(keyID), nullIfEmpty(publicationID), outcome, status,
		nullIfEmpty(ip), nullIfEmpty(truncateBytesStr(r.Header.Get("user-agent"), 300)), nullIfEmpty(errMsg)); err != nil {
		d.loggerWarn("log callback gagal", err)
	}
}

// ---------- POST /v1/tracking/events ----------

var trackingEventTypes = []string{
	"impression", "view", "read", "scroll", "click", "conversion",
}

// TrackingEventCreate: catat event tracking artikel (publik, rate limit per IP).
func (d *Deps) TrackingEventCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ip := strings.TrimSpace(strings.Split(r.Header.Get("x-forwarded-for"), ",")[0])
	if ip == "" {
		ip = strings.TrimSpace(r.Header.Get("x-real-ip"))
	}
	if ip != "" {
		recent, _ := d.queryRow(ctx,
			"SELECT COUNT(*)::int AS count FROM tracking_events WHERE ip_address = $1 AND created_at > NOW() - INTERVAL '60 seconds'", ip)
		if recent.intOr("count", 0) >= 120 {
			writeErr(w, d, adminsharedErrTooManyWith("Too many events"))
			return
		}
	}
	body := decodeJSON(r)
	if len(body) == 0 {
		writeErr(w, d, errBadRequest("Invalid JSON body"))
		return
	}
	eventType, present := strField(body, "event_type")
	if !present {
		writeErr(w, d, errUnprocessable("Invalid input: expected \"impression\"|\"view\"|\"read\"|\"scroll\"|\"click\"|\"conversion\", received undefined"))
		return
	}
	if !ptContains(trackingEventTypes, eventType) {
		writeErr(w, d, errUnprocessable(mpZEnum(trackingEventTypes)))
		return
	}
	code, _ := strField(body, "code")
	publicationID, _ := strField(body, "publication_id")
	sessionID, _ := strField(body, "session_id")
	referrer, _ := strField(body, "referrer")
	pageURL, _ := strField(body, "url")
	var metadata any
	if v, ok := body["metadata"]; ok {
		metadata = v
	}
	if code == "" && publicationID == "" {
		writeErr(w, d, errUnprocessable("code or publication_id is required"))
		return
	}
	if len(code) > 24 || len(sessionID) > 80 || len(referrer) > 1000 || len(pageURL) > 1000 {
		writeErr(w, d, errUnprocessable("String too long"))
		return
	}

	var linkID any
	var campaignID, brandID any
	if code != "" {
		link, err := d.queryRow(ctx, `
			SELECT id, publication_id, campaign_id, brand_id FROM tracking_links WHERE code = $1 LIMIT 1`, code)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		if link == nil {
			writeErr(w, d, errNotFound("Unknown tracking link"))
			return
		}
		linkID = link.raw("id")
		publicationID = link.strOr("publication_id", "")
		if link.has("campaign_id") {
			campaignID = link.strOr("campaign_id", "")
		}
		brandID = link.strOr("brand_id", "")
	}
	if brandID == nil && publicationID != "" {
		row, err := d.queryRow(ctx,
			"SELECT brand_id, campaign_id FROM publications WHERE id = $1 LIMIT 1", publicationID)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		if row == nil {
			writeErr(w, d, errNotFound("Unknown publication"))
			return
		}
		brandID = row.strOr("brand_id", "")
		if row.has("campaign_id") {
			campaignID = row.strOr("campaign_id", "")
		}
	}
	if brandID == nil {
		writeErr(w, d, errNotFound("Unknown tracking link"))
		return
	}

	var metadataJSON any
	if metadata != nil {
		metadataJSON = marshalJSONAny(metadata)
	}
	if err := d.exec(ctx, `
		INSERT INTO tracking_events (id, link_id, publication_id, campaign_id, brand_id, event_type,
		                             session_id, ip_address, user_agent, referrer, url, metadata)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)`,
		linkID, nullIfEmpty(publicationID), campaignID, brandID, eventType,
		nullIfEmpty(sessionID), nullIfEmpty(ip), nullIfEmpty(truncateBytesStr(r.Header.Get("user-agent"), 300)),
		nullIfEmpty(referrer), nullIfEmpty(pageURL), metadataJSON); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

// ---------- helper kecil ----------

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncateBytesStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (d *Deps) loggerWarn(msg string, err error) {
	slogWarn("[platform] "+msg, err)
}

func plainNotFound(w http.ResponseWriter) {
	http.Error(w, "Not found", http.StatusNotFound)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---------- worker helper (internal jobs / cron) ----------

type jobRunResult struct {
	ID     string
	Kind   string
	Failed bool
}

// claimAndRunJob: ambil 1 job (FOR UPDATE SKIP LOCKED) lalu proses lewat
// dispatcher bersama internal/jobs. Backoff dan penandaan selesai/gagal
// dikelola Runner supaya semua proses (API, worker, scheduler) seragam.
func (d *Deps) claimAndRunJob(ctx context.Context, workerID string) (*jobRunResult, error) {
	if d.Runner == nil {
		return nil, errInternal("Job runner belum dikonfigurasi")
	}
	job, err := d.Runner.ProcessOnce(ctx, workerID)
	if err != nil || job == nil {
		return nil, err
	}
	// Runner sudah menandai status akhir; baca ulang untuk melaporkan hasil.
	status, sErr := d.queryRow(ctx, "SELECT status FROM jobs WHERE id = $1 LIMIT 1", job.ID)
	failed := sErr == nil && status != nil && status.strOr("status", "") == "failed"
	return &jobRunResult{ID: job.ID, Kind: job.Kind, Failed: failed}, nil
}

// materializeScheduledPublications: scheduler publikasi — publikasi terjadwal
// yang waktunya tiba dipindahkan ke antrean eksekusi.
func (d *Deps) materializeScheduledPublications(ctx context.Context, maxJobs int) (map[string]any, error) {
	due, err := d.queryRows(ctx, `
		SELECT id, brand_id FROM publications
		WHERE status = 'pending' AND scheduled_at IS NOT NULL AND scheduled_at <= NOW()
		ORDER BY scheduled_at LIMIT $1`, maxJobs)
	if err != nil {
		return nil, err
	}
	enqueued := 0
	for _, row := range due {
		id := row.strOr("id", "")
		if err := d.exec(ctx, "UPDATE publications SET status = 'queued', updated_at = NOW() WHERE id = $1 AND status = 'pending'", id); err != nil {
			continue
		}
		if _, err := d.opEnqueueJob(ctx, "publication_execution", map[string]any{
			"publicationId": id, "brandId": row.strOr("brand_id", ""),
		}, 3); err != nil {
			continue
		}
		enqueued++
	}
	return map[string]any{
		"scanned":  len(due),
		"enqueued": enqueued,
	}, nil
}
