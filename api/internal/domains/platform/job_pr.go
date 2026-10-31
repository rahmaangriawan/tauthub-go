package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// job_pr.go: processor job kind `pr_analysis` dan `pr_verify` — port
// lib/pr/analyze.ts analyzePublicationPr + lib/pr/verify.ts verifyPublicationPr
// beserta recalculateAndNotifyPrScore dari lib/jobs/queue.ts.

const (
	prMessageAnalysisVersion   = "message_analysis_v1"
	prSentimentAnalysisVersion = "sentiment_analysis_v1"
	prVerificationVersion      = "verification_v1"
)

// RegisterPRJobs: daftarkan kedua kind PR ke dispatcher bersama.
func (d *Deps) RegisterPRJobs() {
	jobs.Register("pr_analysis", func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["publicationId"].(string)
		if id == "" {
			return fmt.Errorf("payload publicationId kosong")
		}
		return d.PublicationPRAnalysisJob(ctx, id)
	})
	jobs.Register("pr_verify", func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["publicationId"].(string)
		if id == "" {
			return fmt.Errorf("payload publicationId kosong")
		}
		return d.PublicationPRVerifyJob(ctx, id)
	})
}

// ---------- pr_analysis ----------

// prMessageResult: satu hasil pemeriksaan key message.
type prMessageResult struct {
	Message  string `json:"message"`
	Found    bool   `json:"found"`
	Evidence string `json:"evidence"`
}

// PublicationPRAnalysisJob: analisis konsistensi pesan + tone pemberitaan untuk
// satu publikasi. Idempoten: kedua tabel di-upsert per publication_id.
func (d *Deps) PublicationPRAnalysisJob(ctx context.Context, publicationID string) error {
	pub, err := d.queryRow(ctx,
		"SELECT id, document_id, campaign_id FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil || pub == nil {
		return err
	}
	documentID := pub.strOr("document_id", "")
	campaignID := pub.strOr("campaign_id", "")

	doc, err := d.queryRow(ctx,
		"SELECT id, current_version, job_id FROM content_documents WHERE id = $1 LIMIT 1", documentID)
	if err != nil || doc == nil {
		return err
	}
	jobID := doc.strOr("job_id", "")
	version, err := d.queryRow(ctx,
		"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		documentID, doc.intOr("current_version", 1))
	if err != nil || version == nil {
		return err
	}
	text := version.strOr("content", "")

	keyMessages := d.prIntakeKeyMessages(ctx, jobID)

	// --- Konsistensi pesan (hanya bila ada key messages) ---
	if len(keyMessages) > 0 {
		results := []prMessageResult{}
		status := "completed"
		errorMessage := ""

		if outcome, ok := d.prAIAnalyzeMessages(ctx, text, keyMessages); ok {
			results = outcome
		} else if _, configured := d.prAIRuntime(ctx); configured {
			// Provider ada tetapi hasil tidak terpakai — pakai fallback kata kunci.
			results = prFallbackMessageCheck(text, keyMessages)
		} else {
			results = prFallbackMessageCheck(text, keyMessages)
		}
		if len(results) == 0 && status == "completed" {
			results = prFallbackMessageCheck(text, keyMessages)
		}

		var consistency *float64
		if len(results) > 0 {
			found := 0
			for _, r := range results {
				if r.Found {
					found++
				}
			}
			score := float64(int(float64(found)/float64(len(results))*10000+0.5)) / 100
			consistency = &score
		}
		resultsJSON, _ := json.Marshal(results)
		keyJSON, _ := json.Marshal(keyMessages)
		if err := d.exec(ctx, `
			INSERT INTO pr_message_analyses (id, publication_id, campaign_id, key_messages, results, consistency_score, analysis_version, status, error_message, analyzed_at, updated_at)
			VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6, $7, $8, $9, NOW(), NOW())
			ON CONFLICT (publication_id) DO UPDATE SET key_messages = EXCLUDED.key_messages, results = EXCLUDED.results,
			  consistency_score = EXCLUDED.consistency_score, analysis_version = EXCLUDED.analysis_version,
			  status = EXCLUDED.status, error_message = EXCLUDED.error_message, analyzed_at = NOW(), updated_at = NOW()`,
			uuid.NewString(), publicationID, prNullStr(campaignID), string(keyJSON), string(resultsJSON),
			consistency, prMessageAnalysisVersion, status, prNullStr(errorMessage)); err != nil {
			return err
		}
	}

	// --- Tone pemberitaan (selalu dijalankan) ---
	sentiment := "neutral"
	var confidence *float64
	sentStatus := "completed"
	sentError := ""
	if outcome, ok := d.prAIAnalyzeSentiment(ctx, text); ok && outcome != nil {
		sentiment = outcome.Sentiment
		confidence = outcome.Confidence
	} else {
		fallback := prFallbackSentiment(text)
		sentiment = fallback.Sentiment
		confidence = fallback.Confidence
		sentStatus = "completed"
		sentError = ""
	}
	if err := d.exec(ctx, `
		INSERT INTO pr_sentiment_analyses (id, publication_id, campaign_id, sentiment, confidence, analysis_version, status, error_message, analyzed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (publication_id) DO UPDATE SET sentiment = EXCLUDED.sentiment, confidence = EXCLUDED.confidence,
		  analysis_version = EXCLUDED.analysis_version, status = EXCLUDED.status, error_message = EXCLUDED.error_message, analyzed_at = NOW()`,
		uuid.NewString(), publicationID, prNullStr(campaignID), sentiment, confidence,
		prSentimentAnalysisVersion, sentStatus, prNullStr(sentError)); err != nil {
		return err
	}

	if campaignID != "" {
		d.prRecalcAndNotifyScore(ctx, campaignID)
	}
	return nil
}

// ---------- pr_verify ----------

type prVerificationCheck struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// PublicationPRVerifyJob: verifikasi URL terbit terhadap 5 pemeriksaan
// kepatuhan. 404/410 → failed; error jaringan/5xx/403 → unavailable.
func (d *Deps) PublicationPRVerifyJob(ctx context.Context, publicationID string) error {
	pub, err := d.queryRow(ctx, `
		SELECT p.id, p.campaign_id, p.published_url,
		       m.website_url AS media_website,
		       d.title AS document_title, b.name AS brand_name
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || pub == nil {
		return err
	}
	campaignID := pub.strOr("campaign_id", "")

	// Snapshot status verifikasi SEBELUM upsert, supaya notifikasi mitra bisa
	// membandingkan dengan hasil nyata sebelumnya.
	previous, _ := d.queryRow(ctx,
		"SELECT status FROM pr_publication_verifications WHERE publication_id = $1 ORDER BY verified_at DESC LIMIT 1", publicationID)
	previousStatus := ""
	if previous != nil {
		previousStatus = previous.strOr("status", "")
	}

	url := strings.TrimSpace(pub.strOr("published_url", ""))
	if url == "" {
		if err := d.prSaveVerification(ctx, publicationID, campaignID, "", "unavailable", nil, nil, "no published url"); err != nil {
			return err
		}
		d.prAttachVerificationToPublishedNotification(ctx, publicationID, "unavailable", nil, previousStatus)
		return nil
	}

	html, finalURL, fetchStatus, fetchErr := prFetchWithRetry(ctx, url)
	if fetchErr != "" {
		if err := d.prSaveVerification(ctx, publicationID, campaignID, url, "unavailable", nil, nil, fetchErr); err != nil {
			return err
		}
		d.prAttachVerificationToPublishedNotification(ctx, publicationID, "unavailable", nil, previousStatus)
		return nil
	}

	if fetchStatus == http.StatusNotFound || fetchStatus == http.StatusGone {
		message := "HTTP " + itoa(fetchStatus) + " — artikel tidak ditemukan di URL"
		checks := []prVerificationCheck{{Key: "url_accessible", Label: "URL dapat diakses", OK: false, Detail: message}}
		zero := float64(0)
		if err := d.prSaveVerification(ctx, publicationID, campaignID, url, "failed", &zero, checks, message); err != nil {
			return err
		}
		d.prAttachVerificationToPublishedNotification(ctx, publicationID, "failed", &zero, previousStatus)
		return nil
	}
	if fetchStatus < 200 || fetchStatus >= 300 {
		message := "HTTP " + itoa(fetchStatus) + " — URL tidak dapat diverifikasi (robots/access?)"
		if err := d.prSaveVerification(ctx, publicationID, campaignID, url, "unavailable", nil, nil, message); err != nil {
			return err
		}
		d.prAttachVerificationToPublishedNotification(ctx, publicationID, "unavailable", nil, previousStatus)
		return nil
	}

	text := prExtractText(html)
	docTitle := strings.TrimSpace(pub.strOr("document_title", ""))
	brandName := strings.TrimSpace(pub.strOr("brand_name", ""))
	mediaHost := prHostOf(pub.strOr("media_website", ""))
	finalHost := prHostOf(finalURL)

	titleSnippet := ""
	if len(docTitle) > 40 {
		titleSnippet = docTitle[:40]
	} else {
		titleSnippet = docTitle
	}
	titlePresent := false
	if docTitle != "" {
		titlePresent = strings.Contains(text, titleSnippet) ||
			strings.Contains(prTitleFromHTML(html), titleSnippet)
	}
	titleDetail := "judul dokumen kosong"
	if docTitle != "" {
		display := docTitle
		if len(display) > 60 {
			display = display[:60]
		}
		titleDetail = "diharapkan: “" + display + "”"
	}
	brandDetail := "nama brand kosong"
	if brandName != "" {
		brandDetail = "diharapkan: " + brandName
	}
	mediaDetail := "website media kosong — dilewati"
	mediaMatch := true
	if mediaHost != "" {
		mediaMatch = finalHost == mediaHost
		if finalHost == "" {
			mediaDetail = "host: ?"
		} else {
			mediaDetail = "host: " + finalHost
		}
	}

	checks := []prVerificationCheck{
		{Key: "url_accessible", Label: "URL dapat diakses (HTTP 2xx)", OK: true, Detail: "HTTP " + itoa(fetchStatus)},
		{Key: "title_present", Label: "Judul artikel muncul di halaman", OK: titlePresent, Detail: titleDetail},
		{Key: "brand_mention", Label: "Nama brand/klien muncul di halaman",
			OK: brandName != "" && strings.Contains(strings.ToLower(text), strings.ToLower(brandName)), Detail: brandDetail},
		{Key: "content_present", Label: "Konten artikel ter-publish (teks > 200 karakter)",
			OK: len(text) > 200, Detail: itoa(len(text)) + " karakter teks"},
		{Key: "media_match", Label: "URL terbit sesuai domain media", OK: mediaMatch, Detail: mediaDetail},
	}
	okCount := 0
	for _, c := range checks {
		if c.OK {
			okCount++
		}
	}
	score := float64(int(float64(okCount)/float64(len(checks))*10000+0.5)) / 100
	status := "failed"
	if score >= 60 {
		status = "passed"
	}
	if err := d.prSaveVerification(ctx, publicationID, campaignID, url, status, &score, checks, ""); err != nil {
		return err
	}
	d.prAttachVerificationToPublishedNotification(ctx, publicationID, status, &score, previousStatus)

	if campaignID != "" {
		d.prRecalcAndNotifyScore(ctx, campaignID)
	}
	return nil
}

// ---------- skor kampanye ----------

// prRecalcAndNotifyScore: hitung ulang skor PR kampanye setelah aktivitas yang
// relevan. Notifikasi "skor berubah" SENGAJA tidak dikirim lagi — nilainya
// rendah (user sendiri yang memicu analisis) dan hanya mengotori lonceng.
// Skor tetap tersimpan dan terlihat di halaman PR Calculator.
func (d *Deps) prRecalcAndNotifyScore(ctx context.Context, campaignID string) {
	_ = d.prCalculateCampaignScore(ctx, campaignID, true, "")
}

// prAttachVerificationToPublishedNotification: hasil verifikasi URL TIDAK lagi
// dikirim sebagai notifikasi terpisah. Satu artikel sudah mengirim notifikasi
// "publication.published"; baris itu yang ditempeli hasil verifikasi, supaya
// lonceng tidak menampung dua entri untuk kejadian yang sama.
//
// Bila notifikasi terbit tidak ditemukan (mis. publikasi lama yang dibuat
// sebelum perilaku ini ada), hasil verifikasi tetap tersimpan di tabel
// pr_publication_verifications dan terbaca di halaman Status Publikasi.
func (d *Deps) prAttachVerificationToPublishedNotification(ctx context.Context, publicationID, status string, compliance *float64, previousStatus string) {
	if previousStatus != "" && previousStatus == status {
		return
	}
	row, err := d.queryRow(ctx, `
		SELECT pa.user_id AS partner_user_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || row == nil {
		return
	}
	partnerUserID := row.strOr("partner_user_id", "")
	if partnerUserID == "" {
		return
	}

	labels := map[string]string{
		"passed":      "URL terverifikasi",
		"failed":      "URL bermasalah",
		"unavailable": "verifikasi belum bisa dijalankan",
	}
	label := labels[status]
	if label == "" {
		label = status
	}
	suffix := " Verifikasi tautan: " + label
	if compliance != nil {
		suffix += ", skor " + prFormatScore(*compliance) + "/100"
	}
	suffix += "."

	// Hasil verifikasi lama (kalau job dijalankan ulang) dibuang dulu lewat
	// regexp_replace, lalu hasil terbaru ditempel. LEFT(...) menjaga panjang
	// tetap di bawah batas kolom 1000 karakter walau judul artikelnya panjang.
	// Kolom publication_id bertipe character(36) (padding spasi), jadi
	// dibandingkan lewat btrim agar cocok dengan UUID tanpa padding.
	if err := d.exec(ctx, `
		UPDATE notifications
		SET body = LEFT(regexp_replace(COALESCE(body, ''), ' Verifikasi tautan: [^.]*\.', '', 'g'), 860) || $1
		WHERE btrim(publication_id) = $2
		  AND type = 'publication.published'
		  AND user_id = $3`,
		suffix, publicationID, partnerUserID); err != nil {
		slog.Warn("[pr] gagal menempelkan hasil verifikasi ke notifikasi terbit", "pub", publicationID, "err", err)
	}
}

// ---------- penyimpanan & sumber data ----------

func (d *Deps) prSaveVerification(ctx context.Context, publicationID, campaignID, url, status string, score *float64, checks []prVerificationCheck, errorMessage string) error {
	var checksJSON any
	if checks == nil {
		checksJSON = "[]"
	} else {
		b, _ := json.Marshal(checks)
		checksJSON = string(b)
	}
	return d.exec(ctx, `
		INSERT INTO pr_publication_verifications (id, publication_id, campaign_id, url, status, compliance_score, checks, error_message, verified_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, NOW(), NOW())
		ON CONFLICT (publication_id) DO UPDATE SET url = EXCLUDED.url, status = EXCLUDED.status,
		  compliance_score = EXCLUDED.compliance_score, checks = EXCLUDED.checks,
		  error_message = EXCLUDED.error_message, verified_at = NOW(), updated_at = NOW()`,
		uuid.NewString(), publicationID, prNullStr(campaignID), prNullStr(url), status, score,
		checksJSON, prNullStr(errorMessage))
}

// prIntakeKeyMessages: key messages dari Content Intake (via dokumen → job).
func (d *Deps) prIntakeKeyMessages(ctx context.Context, jobID string) []string {
	if jobID == "" {
		return nil
	}
	row, err := d.queryRow(ctx,
		"SELECT key_messages FROM content_intakes WHERE job_id = $1 ORDER BY created_at DESC LIMIT 1", jobID)
	if err != nil || row == nil {
		return nil
	}
	raw := row.jsonField("key_messages")
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		// JSONB kadang datang sebagai string berisi JSON array.
		if s, isStr := raw.(string); isStr {
			var parsed []any
			if json.Unmarshal([]byte(s), &parsed) == nil {
				arr = parsed
			} else {
				arr = []any{s}
			}
		}
	}
	out := []string{}
	for _, m := range arr {
		s, ok := m.(string)
		if !ok {
			continue
		}
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ---------- analisis AI + fallback deterministik ----------

func (d *Deps) prAIRuntime(ctx context.Context) (*ai.AIRuntime, bool) {
	runtime, err := ai.LoadAIRuntime(ctx, d.Pool)
	if err != nil || runtime == nil {
		return nil, false
	}
	return runtime, ai.IsAIConfigured(runtime)
}

func (d *Deps) prAIAnalyzeMessages(ctx context.Context, text string, keyMessages []string) ([]prMessageResult, bool) {
	runtime, ok := d.prAIRuntime(ctx)
	if !ok {
		return nil, false
	}
	var b strings.Builder
	b.WriteString("Analisis konsistensi pesan pada artikel terbit berikut.\n\nKey messages (pesan utama yang wajib tersampaikan):\n")
	for i, m := range keyMessages {
		b.WriteString(itoa(i+1) + ". " + m + "\n")
	}
	b.WriteString("\nArtikel:\n" + truncateBytesStr(text, 6000))
	b.WriteString("\n\nTugas: untuk setiap key message, tentukan apakah pesannya tersampaikan pada artikel. Balas HANYA JSON array tanpa teks lain:\n[{\"message\": \"<teks key message>\", \"found\": true/false, \"evidence\": \"<alasan singkat 1 kalimat>\"}]")

	res, err := ai.GenerateChat(ctx, d.Pool, ai.GenerateChatOptions{
		Prompt: b.String(), Tier: "economy", TaskType: "pr_message_analysis", Runtime: runtime,
	})
	if err != nil || res == nil || res.Mocked {
		return nil, false
	}
	items := prParseMessageJSON(res.Content)
	if len(items) == 0 {
		return nil, false
	}
	return items, true
}

type prSentimentOutcome struct {
	Sentiment  string
	Confidence *float64
}

func (d *Deps) prAIAnalyzeSentiment(ctx context.Context, text string) (*prSentimentOutcome, bool) {
	runtime, ok := d.prAIRuntime(ctx)
	if !ok {
		return nil, false
	}
	prompt := "Tentukan tone/sentiment pemberitaan (coverage tone) artikel berikut. Ini adalah tone ARTIKEL, bukan sentimen audiens.\n\nArtikel:\n" +
		truncateBytesStr(text, 6000) +
		"\n\nBalas HANYA JSON object tanpa teks lain:\n{\"sentiment\": \"positive|neutral|negative\", \"confidence\": 0.0-1.0, \"evidence\": \"<alasan singkat>\"}"

	res, err := ai.GenerateChat(ctx, d.Pool, ai.GenerateChatOptions{
		Prompt: prompt, Tier: "economy", TaskType: "pr_sentiment_analysis", Runtime: runtime,
	})
	if err != nil || res == nil || res.Mocked {
		return nil, false
	}
	parsed := prParseJSONObject(res.Content)
	if parsed == nil {
		return nil, false
	}
	sentiment, _ := parsed["sentiment"].(string)
	if sentiment != "positive" && sentiment != "neutral" && sentiment != "negative" {
		return nil, false
	}
	out := &prSentimentOutcome{Sentiment: sentiment}
	if v, ok := parsed["confidence"].(float64); ok {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		out.Confidence = &v
	}
	return out, true
}

func prParseMessageJSON(content string) []prMessageResult {
	parsed := prParseJSONObject(content)
	if parsed == nil {
		return nil
	}
	// Bisa array langsung; ambil dari bentuk array dalam objek pembungkus.
	var arr []any
	if a, ok := parsed["_array"].([]any); ok {
		arr = a
	} else if a, ok := parsed["results"].([]any); ok {
		arr = a
	} else {
		return nil
	}
	out := []prMessageResult{}
	for _, item := range arr {
		rec, ok := item.(map[string]any)
		if !ok {
			continue
		}
		message, ok := rec["message"].(string)
		if !ok {
			continue
		}
		found, _ := rec["found"].(bool)
		evidence, _ := rec["evidence"].(string)
		out = append(out, prMessageResult{Message: message, Found: found, Evidence: evidence})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// prParseJSONObject: ambil objek/array JSON pertama di dalam teks bebas.
// Array dibungkus kembali ke {"_array": ...} agar pemanggil seragam.
func prParseJSONObject(content string) map[string]any {
	if m := rePRJSONObject.FindString(content); m != "" {
		var parsed map[string]any
		if json.Unmarshal([]byte(m), &parsed) == nil && parsed != nil {
			return parsed
		}
	}
	if m := rePRJSONArray.FindString(content); m != "" {
		var arr []any
		if json.Unmarshal([]byte(m), &arr) == nil {
			return map[string]any{"_array": arr}
		}
	}
	return nil
}

var (
	rePRJSONObject = regexp.MustCompile(`(?s)\{[\s\S]*\}`)
	rePRJSONArray  = regexp.MustCompile(`(?s)\[[\s\S]*\]`)
)

var prStopwords = map[string]bool{
	"yang": true, "dan": true, "di": true, "ke": true, "dari": true, "untuk": true,
	"dengan": true, "pada": true, "ini": true, "itu": true, "adalah": true, "akan": true,
	"tidak": true, "dalam": true, "para": true, "oleh": true, "atau": true, "kepada": true,
	"sebuah": true, "serta": true, "agar": true, "supaya": true, "karena": true, "saat": true,
	"secara": true, "lebih": true, "sudah": true, "belum": true, "juga": true, "telah": true,
	"bisa": true, "dapat": true, "harus": true,
}

var rePRWordSplit = regexp.MustCompile(`[^a-z0-9]+`)

func prSignificantWords(message string) []string {
	parts := rePRWordSplit.Split(strings.ToLower(message), -1)
	out := []string{}
	for _, w := range parts {
		if len(w) > 2 && !prStopwords[w] {
			out = append(out, w)
		}
	}
	return out
}

// prFallbackMessageCheck: pemeriksaan berbasis kata kunci (dipakai saat AI
// tidak tersedia atau hasil AI tidak terpakai).
func prFallbackMessageCheck(text string, keyMessages []string) []prMessageResult {
	normalized := strings.ToLower(text)
	out := make([]prMessageResult, 0, len(keyMessages))
	for _, message := range keyMessages {
		words := prSignificantWords(message)
		hits := []string{}
		misses := []string{}
		for _, w := range words {
			if strings.Contains(normalized, w) {
				hits = append(hits, w)
			} else {
				misses = append(misses, w)
			}
		}
		found := len(words) > 0 && float64(len(hits))/float64(len(words)) >= 0.6
		evidence := "Kata kunci tidak terdeteksi: " + strings.Join(prCap(misses, 5), ", ")
		if found {
			evidence = "Kata kunci terdeteksi pada artikel terbit: " + strings.Join(prCap(hits, 5), ", ")
		} else if len(misses) == 0 {
			evidence = "Kata kunci tidak terdeteksi: tidak ada kata signifikan"
		}
		out = append(out, prMessageResult{Message: message, Found: found, Evidence: evidence})
	}
	return out
}

var prPositiveWords = []string{"sukses", "berhasil", "luncurkan", "resmi", "terbaik", "inovatif", "unggul", "meningkat", "berkembang", "apresiasi", "keberhasilan", "terobosan", "modern", "andal", "tepat", "bermanfaat", "efisien", "cermat", "kebanggaan"}
var prNegativeWords = []string{"gagal", "turun", "menurun", "krisis", "masalah", "kontroversi", "tuduhan", "rugi", "terhenti", "mengeluh", "kecelakaan", "korban", "pelanggaran", "skandal", "ancaman", "tuntutan", "mogok", "larangan"}

func prFallbackSentiment(text string) prSentimentOutcome {
	lower := strings.ToLower(text)
	positive, negative := 0, 0
	for _, w := range prPositiveWords {
		if strings.Contains(lower, w) {
			positive++
		}
	}
	for _, w := range prNegativeWords {
		if strings.Contains(lower, w) {
			negative++
		}
	}
	if positive == 0 && negative == 0 {
		c := 0.5
		return prSentimentOutcome{Sentiment: "neutral", Confidence: &c}
	}
	total := float64(positive + negative)
	if float64(positive)/total >= 0.7 {
		c := 0.6 + 0.3*(float64(positive)/total)
		return prSentimentOutcome{Sentiment: "positive", Confidence: &c}
	}
	if float64(negative)/total >= 0.7 {
		c := 0.6 + 0.3*(float64(negative)/total)
		return prSentimentOutcome{Sentiment: "negative", Confidence: &c}
	}
	c := 0.55
	return prSentimentOutcome{Sentiment: "neutral", Confidence: &c}
}

// ---------- verifikasi URL ----------

var (
	rePRScriptBlock = regexp.MustCompile(`(?is)<script[\s\S]*?</script>`)
	rePRStyleBlock  = regexp.MustCompile(`(?is)<style[\s\S]*?</style>`)
	rePRTag         = regexp.MustCompile(`<[^>]+>`)
	rePROgTitleA    = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']*)["']`)
	rePROgTitleB    = regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']*)["'][^>]+property=["']og:title["']`)
	rePRTitleTag    = regexp.MustCompile(`(?is)<title[^>]*>([\s\S]*?)</title>`)
)

func prExtractText(html string) string {
	s := rePRScriptBlock.ReplaceAllString(html, " ")
	s = rePRStyleBlock.ReplaceAllString(s, " ")
	s = rePRTag.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func prTitleFromHTML(html string) string {
	if m := rePROgTitleA.FindStringSubmatch(html); len(m) > 1 && m[1] != "" {
		return m[1]
	}
	if m := rePROgTitleB.FindStringSubmatch(html); len(m) > 1 && m[1] != "" {
		return m[1]
	}
	if m := rePRTitleTag.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func prHostOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(mpEnsureScheme(raw))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

// prFetchWithRetry: 3 percobaan dengan backoff 1s/5s dan timeout 15s.
// Status >= 500 dicoba ulang; status lain dikembalikan apa adanya.
func prFetchWithRetry(ctx context.Context, url string) (html, finalURL string, status int, errMessage string) {
	backoff := []time.Duration{time.Second, 5 * time.Second}
	lastError := ""
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", "", 0, ctx.Err().Error()
			case <-time.After(backoff[attempt-1]):
			}
		}
		resp, err := security.Get(ctx, url, map[string]string{
			"User-Agent": "Tulisio-PR-Verifier/1.0 (+https://tulisio.example)",
		}, 15*time.Second)
		if err != nil {
			lastError = truncateBytesStr(err.Error(), 500)
			continue
		}
		if resp.StatusCode >= 500 {
			lastError = "HTTP " + itoa(resp.StatusCode) + " (retryable)"
			_ = resp.Body.Close()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastError = truncateBytesStr(readErr.Error(), 500)
			continue
		}
		final := url
		if resp.Request != nil && resp.Request.URL != nil {
			final = resp.Request.URL.String()
		}
		return string(body), final, resp.StatusCode, ""
	}
	if lastError == "" {
		lastError = "fetch failed"
	}
	return "", "", 0, lastError
}

// ---------- util kecil ----------

func prNullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func prAbsDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

// prFormatScore: tampilkan skor tanpa desimal bila bulat (parity template JS).
func prFormatScore(v float64) string {
	if v == float64(int(v)) {
		return itoa(int(v))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

// prCap: batasi panjang slice (helper lokal, parity capStrings).
func prCap(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}
