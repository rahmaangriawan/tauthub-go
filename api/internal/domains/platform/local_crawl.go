package platform

// local_crawl.go: Local Retrieval Worker — penarikan halaman media lewat IP
// ISP rumahan untuk domain yang memblokir IP datacenter (HTTP 403/429 dari
// Cloudflare WAF, terverifikasi lewat Worker tes).
//
// Alur:
//   1. Job media_ai_summary gagal menarik halaman (semua ditolak WAF) →
//      lcRequestLocalFetch membuat/upsert request 'pending'.
//   2. Worker Bun (komputer lokal admin) polling GET claim ber-token.
//   3. Worker menarik halaman lewat ISP rumah, mengekstrak teks, POST result.
//   4. Server menjalankan AI (prompt sama dengan job) dan menyimpan
//      media.ai_summary — satu domain satu profil, sama seperti job biasa.
//
// Autentikasi: token khusus LOCAL_WORKER_TOKEN (bukan JWT admin) supaya token
// bisa di-revoke tanpa menyentuh sesi, dan cakupannya hanya dua endpoint ini.

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

const (
	lcStaleClaimMinutes = 15 // claim yang tidak kunjung dikirim hasil dianggap mati
	lcMaxResultChars    = 60_000
)

// lcWorkerToken: token worker dari env. Kosong = fitur worker lokal nonaktif
// (endpoint claim tetap menjawab 503 supaya admin tahu konfigurasinya kurang).
func lcWorkerToken() string {
	return strings.TrimSpace(os.Getenv("LOCAL_WORKER_TOKEN"))
}

func (d *Deps) lcWorkerAuth(w http.ResponseWriter, r *http.Request) bool {
	token := lcWorkerToken()
	if token == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "Worker lokal belum diaktifkan (LOCAL_WORKER_TOKEN kosong di server).")
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
		httpx.Error(w, http.StatusUnauthorized, "Token worker lokal tidak valid.")
		return false
	}
	return true
}

// ---------- GET /v1/platform/local-worker/claim ----------

func (d *Deps) LocalWorkerClaim(w http.ResponseWriter, r *http.Request) {
	if !d.lcWorkerAuth(w, r) {
		return
	}
	// Satu statement atomik: ambil request tertua (pending, atau claimed yang
	// stale) dan tandai claimed — aman untuk beberapa worker sekaligus.
	query := fmt.Sprintf(`
		UPDATE local_crawl_requests
		   SET status = 'claimed', claimed_at = NOW(), updated_at = NOW()
		 WHERE id = (
		   SELECT id FROM local_crawl_requests
		    WHERE status = 'pending'
		       OR (status = 'claimed' AND claimed_at < NOW() - INTERVAL '%d minutes')
		    ORDER BY created_at ASC
		    LIMIT 1
		    FOR UPDATE SKIP LOCKED
		 )
		RETURNING id, media_id, media_name, website_url`,
		lcStaleClaimMinutes)
	row, err := d.queryRow(r.Context(), query)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	p := ptRow(ptNormalizeMap(row))
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          p.strOr("id", ""),
		"media_id":    p.strOr("media_id", ""),
		"media_name":  p.strOr("media_name", ""),
		"website_url": p.strOr("website_url", ""),
	})
}

// ---------- POST /v1/platform/local-worker/requests/{id}/result ----------

func (d *Deps) LocalWorkerResult(w http.ResponseWriter, r *http.Request) {
	if !d.lcWorkerAuth(w, r) {
		return
	}
	reqID := chiParam(r, "id")
	body := decodeJSON(r)
	ok, _ := boolField(body, "ok")
	text, _ := strField(body, "text")
	pages := body["pages"]
	errorMessage, _ := strField(body, "error")
	text = strings.TrimSpace(text)

	reqRow, err := d.queryRow(r.Context(), `
		SELECT id, media_id, media_name, website_url, status
		FROM local_crawl_requests WHERE id = $1::text LIMIT 1`, reqID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if reqRow == nil {
		writeErr(w, d, errNotFound("Request worker lokal tidak ditemukan"))
		return
	}
	req := ptRow(ptNormalizeMap(reqRow))
	if req.strOr("status", "") != "claimed" {
		writeErr(w, d, errConflict("Request sudah diproses sebelumnya."))
		return
	}
	mediaID := req.strOr("media_id", "")
	mediaName := req.strOr("media_name", "")
	websiteURL := strings.TrimSpace(req.strOr("website_url", ""))

	if !ok {
		reason := ptTruncate(strings.TrimSpace(errorMessage), 800)
		if reason == "" {
			reason = "Worker lokal gagal menarik halaman."
		}
		if err := d.exec(r.Context(), `
			UPDATE local_crawl_requests
			   SET status = 'failed', error_message = $1, done_at = NOW(), updated_at = NOW()
			 WHERE id = $2::text`, reason, reqID); err != nil {
			writeErr(w, d, err)
			return
		}
		d.mpRecordSummaryEvent(r.Context(), mediaID, mediaName, websiteURL, "failed", nil,
			"Worker lokal: "+reason)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "failed"})
		return
	}

	if runes := []rune(text); len(runes) < 80 {
		reason := "Hasil ekstraksi worker lokal terlalu pendek."
		_ = d.exec(r.Context(), `
			UPDATE local_crawl_requests
			   SET status = 'failed', error_message = $1, done_at = NOW(), updated_at = NOW()
			 WHERE id = $2::text`, reason, reqID)
		d.mpRecordSummaryEvent(r.Context(), mediaID, mediaName, websiteURL, "failed", pages,
			"Worker lokal: "+reason)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "failed"})
		return
	}
	if runes := []rune(text); len(runes) > lcMaxResultChars {
		text = string(runes[:lcMaxResultChars])
	}

	if err := d.exec(r.Context(), `
		UPDATE local_crawl_requests
		   SET status = 'done', result_text = $1, result_pages = $2::jsonb, done_at = NOW(), updated_at = NOW()
		 WHERE id = $3::text`, text, mpJSONString(pages), reqID); err != nil {
		writeErr(w, d, err)
		return
	}

	// AI tetap di VPS dengan prompt yang sama seperti job biasa.
	media, err := d.mpManualMediaRow(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	runtime, rErr := ai.LoadAIRuntime(r.Context(), d.Pool)
	if rErr != nil {
		writeErr(w, d, rErr)
		return
	}
	if !ai.IsAIConfigured(runtime) {
		d.mpRecordSummaryEvent(r.Context(), mediaID, mediaName, websiteURL, "failed", pages,
			"Worker lokal sukses, tapi AI belum dikonfigurasi.")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "done", "ai": false, "error": "AI belum dikonfigurasi"})
		return
	}
	result, gErr := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		// Prompt sama dengan job media_ai_summary supaya gaya kesimpulan konsisten.
		Prompt:   mjSummaryPrompt(mjMitraContext(media), text),
		Tier:     "economy",
		TaskType: "media_summary_manual",
		Runtime:  runtime,
	})
	if gErr != nil {
		d.mpRecordSummaryEvent(r.Context(), mediaID, mediaName, websiteURL, "failed", pages,
			"Worker lokal sukses, AI gagal: "+ptTruncate(gErr.Error(), 300))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "done", "ai": false, "error": gErr.Error()})
		return
	}
	if result.Mocked {
		d.mpRecordSummaryEvent(r.Context(), mediaID, mediaName, websiteURL, "failed", pages,
			"Worker lokal sukses, AI belum dikonfigurasi.")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "done", "ai": false, "error": "AI belum dikonfigurasi"})
		return
	}
	summary := mjTrimQuotes(result.Content)
	if utf8.RuneCountInString(summary) < mjMinSummaryChars {
		d.mpRecordSummaryEvent(r.Context(), mediaID, mediaName, websiteURL, "failed", pages,
			"Worker lokal sukses, hasil AI terlalu pendek.")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "done", "ai": false, "error": "Hasil AI terlalu pendek"})
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE media SET ai_summary = $1, ai_summary_generated_at = NOW() WHERE id = $2::text",
		summary, mediaID); err != nil {
		writeErr(w, d, err)
		return
	}
	if domain := mjNormalizedDomain(websiteURL); domain != "" {
		_ = d.exec(r.Context(), `
			UPDATE media SET ai_summary = $1, ai_summary_generated_at = NOW()
			WHERE id <> $2::text AND ai_summary IS NULL
			  AND RTRIM(REGEXP_REPLACE(LOWER(TRIM(website_url)), '^(https?://)?(www\.)?', ''), '/') = $3`,
			summary, mediaID, domain)
	}
	d.mpRecordSummaryEvent(r.Context(), mediaID, mediaName, websiteURL, "success", pages, "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "done", "ai": true})
}

// ---------- trigger dari job media_ai_summary ----------

// lcCrawlFullyBlocked: true bila crawl gagal total — ada catatan halaman tapi
// semuanya ditolak/buram (WAF, DNS), sehingga ringkasan hanya berdiri di atas
// data mitra. Kasus inilah yang dikirim ke worker lokal.
func lcCrawlFullyBlocked(pages []mjCrawledPage) bool {
	if len(pages) == 0 {
		return false
	}
	for _, p := range pages {
		if p.OK {
			return false
		}
	}
	return true
}

// lcRequestLocalFetch: minta worker lokal menarik ulang halaman media.
// Upsert per media (satu request aktif); bila worker belum dikonfigurasi,
// tidak ada yang ditulis — perilaku fallback tetap seperti semula.
func (d *Deps) lcRequestLocalFetch(ctx context.Context, mediaID, mediaName, websiteURL string) {
	if lcWorkerToken() == "" {
		slog.Warn("[media-summary] fallback worker lokal dilewati: LOCAL_WORKER_TOKEN kosong", "media", mediaID)
		return
	}
	// Jangan timpa request yang sedang diproses worker.
	row, err := d.queryRow(ctx,
		"SELECT status FROM local_crawl_requests WHERE media_id = $1::text LIMIT 1", mediaID)
	if err == nil && row != nil {
		status := ptRow(ptNormalizeMap(row)).strOr("status", "")
		if status == "pending" || status == "claimed" {
			slog.Info("[media-summary] fallback worker lokal: request sudah ada (status " + status + ")", "media", mediaID)
			return
		}
	}
	if err := d.exec(ctx, `
		INSERT INTO local_crawl_requests (id, media_id, media_name, website_url, status)
		VALUES ($1, $2, $3, $4, 'pending')
		ON CONFLICT (media_id) DO UPDATE
		   SET status = 'pending', error_message = NULL, updated_at = NOW()
		 WHERE local_crawl_requests.status IN ('done', 'failed')`,
		uuid.NewString(), mediaID, nullIfEmpty(mediaName), nullIfEmpty(websiteURL)); err != nil {
		slog.Error("[media-summary] fallback worker lokal gagal dibuat", "media", mediaID, "err", err)
		return
	}
	slog.Info("[media-summary] fallback worker lokal dibuat", "media", mediaID, "url", websiteURL)
	d.mpRecordSummaryEvent(ctx, mediaID, mediaName, websiteURL, "queued", nil,
		"Crawl VPS gagal (semua halaman ditolak WAF) — menunggu worker lokal.")
}
