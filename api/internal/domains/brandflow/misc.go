package brandflow

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/imageconvert"
)

// misc.go: port analyze/[topic], assets, usage.

// ---------- GET /v1/brands/{brandId}/analyze/{topic} ----------

var intentAnalysisIntents = []string{"informational", "commercial", "transactional", "navigational"}

func analyzeIntentPrompt(topic string) string {
	return `Analisis topik artikel berikut untuk brand Indonesia: "` + topic + `"

Balas HANYA JSON valid, tanpa markdown fence, dengan bentuk:
{"search_intent":"informational|commercial|transactional|navigational","angle":"<satu kalimat sudut pandang spesifik>","suggested_words":<angka 600-2500>}

Panduan intent:
- informational: pembaca cari penjelasan/cara/edukasi
- commercial: pembaca membandingkan pilihan sebelum beli
- transactional: pembaca siap beli/daftar/pesan
- navigational: pembaca mencari brand/produk/halaman tertentu`
}

// TautAnalyzeTopic: GET /v1/brands/{brandId}/analyze/{topic}
func (d *Deps) TautAnalyzeTopic(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	topic := strings.TrimSpace(chiParam(r, "topic"))
	topic = truncateString(topic, 300)
	if utf8.RuneCountInString(topic) < 6 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Topic too short")
		return
	}
	brandID := brandRow.ID
	result, err := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt: analyzeIntentPrompt(topic), Tier: "economy", TaskType: "intent_analysis",
		BrandID: &brandID, UserID: &user.ID,
	})
	if err != nil {
		d.writeErr(w, err)
		return
	}
	analysis := readIntentAnalysis(result.Content)
	writeJSON(w, http.StatusOK, map[string]any{
		"search_intent":   analysis.searchIntent,
		"angle":           analysis.angle,
		"suggested_words": analysis.suggestedWords,
		"mocked":          result.Mocked,
	})
}

type intentAnalysis struct {
	searchIntent   string
	angle          string
	suggestedWords int
}

// readIntentAnalysis: ekstrak objek JSON pertama; fallback intent teraman.
func readIntentAnalysis(text string) intentAnalysis {
	fallback := intentAnalysis{searchIntent: "informational", angle: "", suggestedWords: 1200}
	parsed := extractJSONObject(text)
	if parsed == nil {
		return fallback
	}
	intent := strings.ToLower(strings.TrimSpace(stringValue(parsed["search_intent"])))
	if !containsString(intentAnalysisIntents, intent) {
		intent = "informational"
	}
	angle := truncateString(stringValue(parsed["angle"]), 300)
	words := 1200
	if f, ok := toFloat(parsed["suggested_words"]); ok {
		words = int(f)
	}
	if words < 600 {
		words = 600
	}
	if words > 2500 {
		words = 2500
	}
	return intentAnalysis{searchIntent: intent, angle: angle, suggestedWords: words}
}

// ---------- POST /v1/brands/{brandId}/assets ----------

var allowedAssetMimes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true,
	"application/pdf": true, "text/plain": true,
}

const assetMaxSize = 10 * 1024 * 1024

// AssetsUpload: POST /v1/brands/{brandId}/assets
func (d *Deps) AssetsUpload(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	_, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	file, filename, size, contentType, err := formFile(r, "file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File tidak ditemukan.")
		return
	}
	defer file.Close()
	mime := strings.ToLower(contentType)
	isAllowed := allowedAssetMimes[mime] || strings.HasPrefix(mime, "image/")
	if !isAllowed {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tipe file tidak diizinkan. Izinkan: gambar, PDF, TXT.")
		return
	}
	if size > assetMaxSize {
		httpx.Error(w, http.StatusUnprocessableEntity, "Maksimal 10 MB.")
		return
	}
	buffer, err := readAllLimited(file, assetMaxSize)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	// Magic bytes: MIME klien bisa dipalsukan.
	isImage := false
	if mime != "text/plain" {
		isPng := len(buffer) >= 8 && string(buffer[:8]) == "\x89PNG\r\n\x1a\n"
		isJpeg := len(buffer) >= 3 && buffer[0] == 0xff && buffer[1] == 0xd8 && buffer[2] == 0xff
		isGif := len(buffer) >= 4 && string(buffer[:4]) == "GIF8"
		isWebp := len(buffer) >= 12 && string(buffer[:4]) == "RIFF" && string(buffer[8:12]) == "WEBP"
		isPdf := len(buffer) >= 4 && string(buffer[:4]) == "%PDF"
		isImage = isPng || isJpeg || isGif || isWebp
		if !isImage && !isPdf {
			httpx.Error(w, http.StatusUnprocessableEntity, "Isi file tidak cocok dengan tipe yang diizinkan.")
			return
		}
	}

	// Gambar selalu dikonversi ke WebP (parity app lama). PDF/TXT disimpan
	// apa adanya. Konversi gagal → byte asli tetap disimpan (upload tidak
	// pernah gagal hanya karena encoder), dengan ekstensi aslinya.
	payload := buffer
	ext := assetExtFor(mime, isImage, buffer)
	if ext == "" && isImage {
		if converted, ok := imageconvert.ConvertToWebP(buffer, mime); ok {
			payload = converted
			ext = "webp"
		} else {
			ext = imageExtFallback(mime, buffer)
		}
	}
	if ext == "" {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Tipe berkas tidak didukung. Izinkan: gambar (PNG/JPEG/WebP/GIF/BMP), PDF, atau TXT.")
		return
	}
	filenameOut := newUUID() + "." + ext
	if err := d.saveUpload(filenameOut, payload); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":        "/uploads/" + filenameOut,
		"filename":   filename,
		"size_bytes": len(payload),
	})
}

// assetExtFor: ekstensi keluaran; "" bila butuh konversi yang belum tersedia.
// WebP sudah WebP (tanpa konversi), PDF/TXT apa adanya. PNG/JPEG/GIF butuh
// konversi ke WebP → belum tersedia, kembalikan "".
func assetExtFor(mime string, isImage bool, buffer []byte) string {
	if mime == "application/pdf" {
		return "pdf"
	}
	if mime == "text/plain" {
		return "txt"
	}
	if isImage && len(buffer) >= 12 && string(buffer[:4]) == "RIFF" && string(buffer[8:12]) == "WEBP" {
		return "webp"
	}
	return ""
}

// imageExtFallback: ekstensi dari magic bytes saat konversi WebP tidak tersedia
// (encoder gagal / gambar korup). Menjaga upload tetap berhasil.
func imageExtFallback(mime string, buffer []byte) string {
	switch {
	case len(buffer) >= 8 && string(buffer[:8]) == "\x89PNG\r\n\x1a\n":
		return "png"
	case len(buffer) >= 3 && buffer[0] == 0xff && buffer[1] == 0xd8 && buffer[2] == 0xff:
		return "jpg"
	case len(buffer) >= 4 && string(buffer[:4]) == "GIF8":
		return "gif"
	case len(buffer) >= 2 && buffer[0] == 'B' && buffer[1] == 'M':
		return "bmp"
	}
	switch mime {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/bmp":
		return "bmp"
	}
	return "webp"
}

// ---------- GET /v1/brands/{brandId}/usage ----------

// UsageGet: GET /v1/brands/{brandId}/usage
func (d *Deps) UsageGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	q := r.URL.Query()
	days := clampInt(q.Get("days"), 30, 1, 365)
	limit := clampInt(q.Get("limit"), 20, 1, 100)
	offset := intParamDef(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	since := timeAgo(days * 24 * 3600)

	summary, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) AS calls,
		       COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) AS successful_calls,
		       COALESCE(SUM(CASE WHEN success = FALSE THEN 1 ELSE 0 END), 0) AS failed_calls,
		       COALESCE(SUM(input_tokens), 0) AS input_tokens,
		       COALESCE(SUM(output_tokens), 0) AS output_tokens,
		       COALESCE(SUM(total_tokens), 0) AS total_tokens,
		       COALESCE(ROUND(AVG(latency_ms)), 0) AS avg_latency_ms,
		       COALESCE(SUM(CASE WHEN cache_hit THEN 1 ELSE 0 END), 0) AS cache_hits
		FROM model_usage_events WHERE brand_id = $1 AND created_at >= $2`, brandRow.ID, since)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	quota, err := d.queryRow(r.Context(),
		"SELECT monthly_token_limit, warning_percent, plan_code, daily_call_limit FROM brand_ai_quotas WHERE brand_id = $1", brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	daily, err := d.queryRows(r.Context(), `
		SELECT TO_CHAR(created_at, 'YYYY-MM-DD') AS date, COUNT(*) AS calls,
		       COALESCE(SUM(input_tokens), 0) AS input_tokens,
		       COALESCE(SUM(output_tokens), 0) AS output_tokens,
		       COALESCE(SUM(total_tokens), 0) AS total_tokens
		FROM model_usage_events WHERE brand_id = $1 AND created_at >= $2
		GROUP BY TO_CHAR(created_at, 'YYYY-MM-DD') ORDER BY date DESC`, brandRow.ID, since)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	byModel, err := d.queryRows(r.Context(), `
		SELECT provider, model, COUNT(*) AS calls,
		       COALESCE(SUM(total_tokens), 0) AS total_tokens,
		       COALESCE(ROUND(AVG(latency_ms)), 0) AS avg_latency_ms
		FROM model_usage_events WHERE brand_id = $1 AND created_at >= $2
		GROUP BY provider, model ORDER BY total_tokens DESC`, brandRow.ID, since)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	byTask, err := d.queryRows(r.Context(), `
		SELECT task_type, COUNT(*) AS calls,
		       COALESCE(SUM(total_tokens), 0) AS total_tokens,
		       COALESCE(SUM(CASE WHEN success = FALSE THEN 1 ELSE 0 END), 0) AS failed_calls
		FROM model_usage_events WHERE brand_id = $1 AND created_at >= $2
		GROUP BY task_type ORDER BY total_tokens DESC`, brandRow.ID, since)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	eventCount, err := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM model_usage_events WHERE brand_id = $1 AND created_at >= $2", brandRow.ID, since)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	events, err := d.queryRows(r.Context(), `
		SELECT id, task_type, provider, model, input_tokens, output_tokens, total_tokens, latency_ms, success, error_category, cache_hit, created_at
		FROM model_usage_events WHERE brand_id = $1 AND created_at >= $2
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`, brandRow.ID, since, limit, offset)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	summaryJSON := map[string]any{
		"calls": 0, "successful_calls": 0, "failed_calls": 0,
		"input_tokens": 0, "output_tokens": 0, "total_tokens": 0,
		"avg_latency_ms": 0, "cache_hits": 0,
	}
	if summary != nil {
		summaryJSON = map[string]any{
			"calls":            summary.intOr("calls", 0),
			"successful_calls": summary.intOr("successful_calls", 0),
			"failed_calls":     summary.intOr("failed_calls", 0),
			"input_tokens":     summary.intOr("input_tokens", 0),
			"output_tokens":    summary.intOr("output_tokens", 0),
			"total_tokens":     summary.intOr("total_tokens", 0),
			"avg_latency_ms":   summary.intOr("avg_latency_ms", 0),
			"cache_hits":       summary.intOr("cache_hits", 0),
		}
	}
	var quotaJSON any
	if quota != nil {
		quotaJSON = map[string]any{
			"monthly_token_limit": quota.intOr("monthly_token_limit", 0),
			"warning_percent":     quota.intOr("warning_percent", 0),
			"plan_code":           quota.raw("plan_code"),
			"daily_call_limit":    quota.intOr("daily_call_limit", 0),
		}
	}
	dailyJSON := make([]map[string]any, 0, len(daily))
	for _, row := range daily {
		dailyJSON = append(dailyJSON, map[string]any{
			"date": row.raw("date"), "calls": row.intOr("calls", 0),
			"input_tokens": row.intOr("input_tokens", 0), "output_tokens": row.intOr("output_tokens", 0),
			"total_tokens": row.intOr("total_tokens", 0),
		})
	}
	byModelJSON := make([]map[string]any, 0, len(byModel))
	for _, row := range byModel {
		byModelJSON = append(byModelJSON, map[string]any{
			"provider": row.raw("provider"), "model": row.raw("model"),
			"calls": row.intOr("calls", 0), "total_tokens": row.intOr("total_tokens", 0),
			"avg_latency_ms": row.intOr("avg_latency_ms", 0),
		})
	}
	byTaskJSON := make([]map[string]any, 0, len(byTask))
	for _, row := range byTask {
		byTaskJSON = append(byTaskJSON, map[string]any{
			"task_type": row.raw("task_type"), "calls": row.intOr("calls", 0),
			"total_tokens": row.intOr("total_tokens", 0), "failed_calls": row.intOr("failed_calls", 0),
		})
	}
	eventsJSON := make([]map[string]any, 0, len(events))
	for _, row := range events {
		eventsJSON = append(eventsJSON, map[string]any{
			"id": row.raw("id"), "task_type": row.raw("task_type"),
			"provider": row.raw("provider"), "model": row.raw("model"),
			"input_tokens": row.intOr("input_tokens", 0), "output_tokens": row.intOr("output_tokens", 0),
			"total_tokens": row.intOr("total_tokens", 0), "latency_ms": row.intOr("latency_ms", 0),
			"success": row.boolOr("success", false), "error_category": row.raw("error_category"),
			"cache_hit": row.boolOr("cache_hit", false), "created_at": row.timePtr("created_at"),
		})
	}
	eventsTotal := 0
	if eventCount != nil {
		eventsTotal = eventCount.intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"days": days, "summary": summaryJSON, "quota": quotaJSON,
		"daily": dailyJSON, "by_model": byModelJSON, "by_task": byTaskJSON,
		"events": eventsJSON, "events_total": eventsTotal, "limit": limit, "offset": offset,
	})
}
