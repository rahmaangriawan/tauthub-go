package clienttaut

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Port lib/plagiarism/smodin.ts + lib/gatekeeper/{content-safety,link-health,
// voice-check}.ts + lib/moderation/normalize.ts (bagian yang dipakai cek).

const smodinHost = "plagiarism-checker-and-auto-citation-generator-multi-lingual.p.rapidapi.com"
const smodinMaxTextChars = 8000

type plagiarismResult struct {
	PercentPlagiarism float64 `json:"percentPlagiarism"`
	Sources           []any   `json:"sources"`
	CheckedAt         string  `json:"checkedAt"`
	Truncated         bool    `json:"truncated,omitempty"`
}

// smodinAPIKey: baca sealed_smodin_api_key dari platform_seo_config + unseal.
func (d *Deps) smodinAPIKey(ctx context.Context) (string, error) {
	row, err := d.queryRow(ctx, "SELECT sealed_smodin_api_key FROM platform_seo_config WHERE id = 'default' LIMIT 1")
	if err != nil || row == nil || !row.has("sealed_smodin_api_key") {
		return "", errStatus(500, "Pengecekan plagiarisme belum dikonfigurasi oleh Admin.")
	}
	sealed := row.strOr("sealed_smodin_api_key", "")
	if sealed == "" {
		return "", errStatus(500, "Pengecekan plagiarisme belum dikonfigurasi oleh Admin.")
	}
	value, err := d.unsealSecret(sealed)
	if err != nil || value == "" {
		return "", errStatus(500, "Kunci plagiarisme tidak dapat dibaca. Perbarui konfigurasi di panel.")
	}
	return value, nil
}

// checkPlagiarism: cek plagiarisme via RapidAPI Smodin (retry 1x untuk transient).
func (d *Deps) checkPlagiarism(ctx context.Context, text, language string) (plagiarismResult, error) {
	key, err := d.smodinAPIKey(ctx)
	if err != nil {
		return plagiarismResult{}, err
	}
	body, _ := json.Marshal(map[string]any{
		"text":     bodyText(text, smodinMaxTextChars),
		"language": language, "includeCitations": false, "scrapeSources": false,
	})
	client := &http.Client{Timeout: 30 * time.Second}
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+smodinHost+"/plagiarism", strings.NewReader(string(body)))
		if reqErr != nil {
			return plagiarismResult{}, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-rapidapi-host", smodinHost)
		req.Header.Set("x-rapidapi-key", key)
		resp, doErr := client.Do(req)
		if doErr != nil {
			lastErr = errStatus(500, "Provider plagiarisme tidak dapat dihubungi sekarang.")
		} else {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var parsed any
				_ = json.Unmarshal(raw, &parsed)
				mapped := mapPlagiarismResponse(parsed)
				if bodyText(text, smodinMaxTextChars) != text {
					mapped.Truncated = true
				}
				return mapped, nil
			}
			switch resp.StatusCode {
			case 401, 403:
				lastErr = errStatus(500, "Autentikasi provider plagiarisme gagal. Perbarui RapidAPI key.")
			case 429:
				lastErr = errStatus(500, "Provider plagiarisme sedang rate-limited. Coba lagi nanti.")
			case 404:
				lastErr = errStatus(500, "Endpoint provider plagiarisme tidak ditemukan.")
			default:
				lastErr = errStatus(500, "Provider plagiarisme sedang tidak tersedia.")
			}
		}
		if attempt < 2 {
			select {
			case <-time.After(time.Duration(700*attempt) * time.Millisecond):
			case <-ctx.Done():
				return plagiarismResult{}, ctx.Err()
			}
		}
	}
	return plagiarismResult{}, lastErr
}

func bodyText(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max]
}

// mapPlagiarismResponse: mapping murni respons Smodin → PlagiarismResult.
func mapPlagiarismResponse(raw any) plagiarismResult {
	root, ok := raw.(map[string]any)
	if !ok {
		root = map[string]any{}
	}
	sources := []any{}
	if arr, ok := root["sources"].([]any); ok {
		for _, entry := range arr {
			row, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			url, _ := row["url"].(string)
			if url == "" {
				continue
			}
			item := map[string]any{"url": url}
			if matched, ok := row["matchedText"].(string); ok && matched != "" {
				item["matchedText"] = matched
			}
			sources = append(sources, item)
		}
	}
	return plagiarismResult{
		PercentPlagiarism: percentValue(root["percentPlagiarism"]),
		Sources:           sources,
		CheckedAt:         nowISO(),
	}
}

func percentValue(v any) float64 {
	var parsed float64
	switch t := v.(type) {
	case float64:
		parsed = t
	case string:
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%f", &parsed); err != nil {
			parsed = 0
		}
	default:
		parsed = 0
	}
	if parsed < 0 {
		return 0
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

// ---------- content safety (port lib/gatekeeper/content-safety.ts) ----------

const maxSafetyInputChars = 6000

type contentSafetyResult struct {
	RiskLevel  string   `json:"riskLevel"`
	Categories []string `json:"categories"`
	Reasons    []string `json:"reasons"`
	Confidence float64  `json:"confidence"`
}

var contentSafetyCategories = []string{
	"sexual_content", "child_safety", "gambling", "fraud_scam", "violence", "extortion",
	"hate_speech", "violent_extremism", "harmful_propaganda", "dangerous_activity",
	"illegal_activity_facilitation", "human_exploitation", "self_harm", "other_illegal_content",
}

var contentSafetyFailsafe = contentSafetyResult{
	RiskLevel: "medium", Categories: []string{},
	Reasons: []string{"Analisis gagal — konten perlu review manual."}, Confidence: 0,
}

var preSignals = []struct {
	label    string
	patterns []string
}{
	{"eksploitasi anak", []string{"bokep anak", "video anak", "foto anak", "grooming anak", "eksploitasi anak"}},
	{"ekstremisme kekerasan", []string{"bom bunuh diri", "radikalisasi", "propaganda teror", "rekrutmen teroris"}},
	{"promosi judi", []string{"daftar slot", "link slot", "deposit 100%", "bonus deposit", "klaim bonus", "claim bonus", "maxwin", "free spin", "daftar judi"}},
}

const contentSafetySystemPrompt = `You are the Gatekeeper content safety classifier for an Indonesian publishing platform. Classify by CONTEXT, not keywords. Do not over-block: discussion, education, news/reporting, criticism, prevention and research about a harmful topic are LOW risk. Promotion, facilitation/instruction, glorification, recruitment, or targeted harm are HIGH risk.

Categories (14): sexual_content, child_safety, gambling, fraud_scam, violence, extortion, hate_speech, violent_extremism, harmful_propaganda, dangerous_activity, illegal_activity_facilitation, human_exploitation, self_harm, other_illegal_content.

Top priority: child_safety, violent_extremism, explicit harmful/illegal facilitation.

Return ONLY valid JSON, no markdown:
{"allowed": boolean, "riskLevel": "low"|"medium"|"high", "categories": [string], "reasons": [string], "confidence": number 0-1}

Rules:
- allowed=true ONLY for low risk. medium = suspicious/ambiguous -> allowed=false (manual review). high = block.
- reasons: max 3 short Indonesian strings, user-safe (e.g. "Promosi judi online.").
- "Dampak buruk judi terhadap keluarga", "Sejarah kejahatan siber di Indonesia", "Bahaya kekerasan terhadap anak" => low, allowed=true.

Output JSON only, no explanation.`

// contentSafetyGuard: fail-safe — error apa pun → review manual (medium risk).
func (d *Deps) contentSafetyGuard(ctx context.Context, text string, links []string, briefContext string) contentSafetyResult {
	normalized := normalizeForModeration(text)
	note := preSignalNote(normalized)

	parts := []string{"Konten:\n" + bodyText(text, maxSafetyInputChars)}
	if len(links) > 0 {
		limit := len(links)
		if limit > 20 {
			limit = 20
		}
		parts = append(parts, "Links:\n"+strings.Join(links[:limit], "\n"))
	}
	if briefContext != "" {
		parts = append(parts, "Konteks:\n"+bodyText(briefContext, 500))
	}
	if note != "" {
		parts = append(parts, "Catatan (verifikasi dengan konteks, bukan verdict): "+note)
	}
	userPrompt := strings.Join(parts, "\n\n")

	result, err := d.callAgentLLM(ctx, "gatekeeper", userPrompt, true, contentSafetySystemPrompt, 0)
	if err != nil {
		slog.Warn("[gatekeeper] content-safety gagal -> review_required", "err", truncateString(err.Error(), 200))
		return contentSafetyFailsafe
	}
	parsed := extractJSONObject(result.Content)
	if parsed == nil {
		return contentSafetyFailsafe
	}
	return contentSafetyFromParsed(parsed)
}

// contentSafetyFromParsed: validasi field safety dari JSON yang sudah diparse
// (dipakai jalur gabungan safety+AI Overview; whitelist kategori & clamp tetap
// di sini supaya hasilnya identik dengan jalur terpisah).
func contentSafetyFromParsed(parsed map[string]any) contentSafetyResult {
	riskLevel := parseRiskLevel(parsed["riskLevel"])
	categories := []string{}
	if set := toSet(contentSafetyCategories); set != nil {
		if arr, ok := parsed["categories"].([]any); ok {
			seen := map[string]bool{}
			for _, c := range arr {
				s, ok := c.(string)
				if !ok || seen[s] || !set[s] {
					continue
				}
				seen[s] = true
				categories = append(categories, s)
				if len(categories) >= 6 {
					break
				}
			}
		}
	}
	reasons := []string{}
	if arr, ok := parsed["reasons"].([]any); ok {
		for _, r := range arr {
			s, ok := r.(string)
			if !ok {
				continue
			}
			trimmed := truncateString(strings.TrimSpace(s), 120)
			if trimmed != "" {
				reasons = append(reasons, trimmed)
			}
			if len(reasons) >= 3 {
				break
			}
		}
	}
	confidence := clampConfidence(parsed["confidence"])
	return contentSafetyResult{RiskLevel: riskLevel, Categories: categories, Reasons: reasons, Confidence: confidence}
}

func toSet(items []string) map[string]bool {
	out := map[string]bool{}
	for _, i := range items {
		out[i] = true
	}
	return out
}

func preSignalNote(normalized string) string {
	hits := []string{}
	for _, s := range preSignals {
		for _, p := range s.patterns {
			if strings.Contains(normalized, p) {
				hits = append(hits, s.label)
				break
			}
		}
	}
	if len(hits) == 0 {
		return ""
	}
	return strings.Join(hits, ", ")
}

func parseRiskLevel(v any) string {
	s := strings.ToLower(fmt.Sprint(v))
	if s == "high" || s == "medium" || s == "low" {
		return s
	}
	return "medium"
}

func clampConfidence(v any) float64 {
	var f float64
	switch t := v.(type) {
	case float64:
		f = t
	default:
		f = 0
	}
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// reSeparatorBetweenAlnum: dipakai normalizeForModeration cadangan; modul
// moderation sudah mengekspor NormalizeForModeration (dipakai lebih dulu).
var reSeparatorBetweenAlnum = regexp.MustCompile(`[a-z0-9][^a-z0-9]+[a-z0-9]`)

// normalizeForModeration: pakai port kanonik di internal/moderation bila ada
// (bit-identik dengan app lama); fallback ke salinan lokal.
func normalizeForModeration(raw string) string {
	if raw == "" {
		return ""
	}
	if out := moderation.NormalizeForModeration(raw); out != "" {
		return out
	}
	lower := strings.ToLower(raw)
	lower = mapHomoglyphs(lower)
	lower = mapLeet(lower)
	for i := 0; i < 6; i++ {
		next := reSeparatorBetweenAlnum.ReplaceAllStringFunc(lower, func(m string) string {
			return m[:1] + m[len(m)-1:]
		})
		if next == lower {
			break
		}
		lower = next
	}
	return strings.TrimSpace(reWhitespace.ReplaceAllString(lower, " "))
}

func mapHomoglyphs(s string) string {
	r := strings.NewReplacer(
		"а", "a", "е", "e", "о", "o", "р", "p", "с", "c", "х", "x", "у", "y", "і", "i",
		"α", "a", "ο", "o", "е", "e",
	)
	return r.Replace(s)
}

func mapLeet(s string) string {
	r := strings.NewReplacer(
		"0", "o", "1", "i", "3", "e", "4", "a", "5", "s", "7", "t", "@", "a", "$", "s",
	)
	return r.Replace(s)
}

// ---------- link health (port lib/gatekeeper/link-health.ts) ----------

type linkHealthItem struct {
	URL        string `json:"url"`
	Status     string `json:"status"`
	HTTPStatus any    `json:"httpStatus"`
	Category   any    `json:"category"`
	Reason     string `json:"reason,omitempty"`
	CheckedAt  string `json:"checkedAt"`
}

type linkHealthResult struct {
	Links          []linkHealthItem `json:"links"`
	Verdict        string           `json:"verdict"`
	BlockedReasons []string         `json:"blockedReasons"`
}

var unsafeLinkCategories = map[string]bool{"gambling": true, "pornography": true, "malware": true}
var validLinkCategories = map[string]bool{"gambling": true, "pornography": true, "malware": true, "safe": true, "unknown": true}
var linkReasons = map[string]string{
	"gambling":    "Mengarah ke konten judi",
	"pornography": "Mengarah ke konten pornografi",
	"malware":     "Berpotensi malware",
	"unknown":     "Kategori tidak pasti, perlu review",
}

// linkHealthValidator: probe reachability + klasifikasi AI semua link reachable.
func (d *Deps) linkHealthValidator(ctx context.Context, html string, excludeURLs []string) linkHealthResult {
	checkedAt := nowISO()
	excluded := make(map[string]bool, len(excludeURLs))
	for _, u := range excludeURLs {
		if trimmed := strings.TrimSpace(u); trimmed != "" {
			excluded[trimmed] = true
		}
	}
	extracted := extractLinks(html)
	urls := make([]string, 0, len(extracted))
	for _, u := range extracted {
		if !excluded[u] {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return linkHealthResult{Links: []linkHealthItem{}, Verdict: "passed", BlockedReasons: []string{}}
	}

	ownShort := []linkHealthItem{}
	foreign := make([]string, 0, len(urls))
	for _, u := range urls {
		code := extractTrackingCode(u)
		if code != "" && d.isTrackingCode(ctx, code) {
			ownShort = append(ownShort, linkHealthItem{
				URL: u, Status: "ok", HTTPStatus: nil, Category: "safe",
				Reason: "Short link Tauthub (redirect tercatat)", CheckedAt: checkedAt,
			})
			continue
		}
		foreign = append(foreign, u)
	}

	type probe struct {
		url        string
		httpStatus *int
	}
	// Probe paralel (maks 6 sekaligus): serial 8 dtk/URL terlalu lambat.
	probes := make([]probe, len(foreign))
	var probeWG sync.WaitGroup
	probeSem := make(chan struct{}, 6)
	for i, u := range foreign {
		probeWG.Add(1)
		go func(i int, u string) {
			defer probeWG.Done()
			probeSem <- struct{}{}
			defer func() { <-probeSem }()
			status, ok := d.probeURL(ctx, u)
			if ok {
				s := status
				probes[i] = probe{url: u, httpStatus: &s}
			} else {
				probes[i] = probe{url: u, httpStatus: nil}
			}
		}(i, u)
	}
	probeWG.Wait()

	reachable := []string{}
	for _, p := range probes {
		if p.httpStatus != nil {
			reachable = append(reachable, p.url)
		}
	}
	categories, classificationFailed := d.classifyLinks(ctx, reachable)

	links := append([]linkHealthItem{}, ownShort...)
	for _, p := range probes {
		if p.httpStatus == nil {
			links = append(links, linkHealthItem{URL: p.url, Status: "unreachable", HTTPStatus: nil, Category: nil, Reason: "Tidak dapat dijangkau", CheckedAt: checkedAt})
			continue
		}
		if classificationFailed {
			links = append(links, linkHealthItem{URL: p.url, Status: "suspicious", HTTPStatus: *p.httpStatus, Category: nil, Reason: "Klasifikasi AI gagal", CheckedAt: checkedAt})
			continue
		}
		category := categories[p.url]
		if category == "" {
			category = "unknown"
		}
		if unsafeLinkCategories[category] {
			links = append(links, linkHealthItem{URL: p.url, Status: "unsafe", HTTPStatus: *p.httpStatus, Category: category, Reason: linkReasons[category], CheckedAt: checkedAt})
			continue
		}
		if category == "unknown" {
			links = append(links, linkHealthItem{URL: p.url, Status: "suspicious", HTTPStatus: *p.httpStatus, Category: category, Reason: linkReasons["unknown"], CheckedAt: checkedAt})
			continue
		}
		links = append(links, linkHealthItem{URL: p.url, Status: "ok", HTTPStatus: *p.httpStatus, Category: "safe", CheckedAt: checkedAt})
	}

	unsafeItems := []linkHealthItem{}
	for _, l := range links {
		if l.Status == "unsafe" {
			unsafeItems = append(unsafeItems, l)
		}
	}
	blockedReasons := []string{}
	for _, l := range unsafeItems {
		cat := ""
		if s, ok := l.Category.(string); ok {
			cat = s
		}
		reason := linkReasons[cat]
		if reason == "" {
			reason = "Konten berbahaya"
		}
		blockedReasons = append(blockedReasons, reason+": "+l.URL)
		slog.Warn("[link-health] unsafe", "url", l.URL, "category", cat)
	}

	verdict := "passed"
	hasSuspicious := false
	for _, l := range links {
		if l.Status == "suspicious" {
			hasSuspicious = true
			break
		}
	}
	if len(unsafeItems) > 0 {
		verdict = "failed"
	} else if hasSuspicious {
		verdict = "review_required"
	}
	return linkHealthResult{Links: links, Verdict: verdict, BlockedReasons: blockedReasons}
}

// probeURL: safeFetch HEAD/GET → status code; false bila gagal.
func (d *Deps) probeURL(ctx context.Context, url string) (int, bool) {
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	resp, err := security.Get(reqCtx, url, map[string]string{
		"user-agent": "Mozilla/5.0 (compatible; TautHub/1.0)",
		"accept":     "*/*",
	}, 8*time.Second)
	if err != nil || resp == nil {
		return 0, false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
	return resp.StatusCode, true
}

// classifyLinks: satu panggilan AI untuk semua URL reachable →
// (map url→category, failed). failed=true bila AI gagal total (fail-safe).
func (d *Deps) classifyLinks(ctx context.Context, urls []string) (map[string]string, bool) {
	if len(urls) == 0 {
		return map[string]string{}, false
	}
	lines := make([]string, 0, len(urls))
	for i, u := range urls {
		lines = append(lines, itoa(i+1)+". "+u)
	}
	prompt := strings.Join([]string{
		"Daftar URL berikut muncul di artikel yang akan didistribusikan.",
		"Klasifikasikan tiap URL apakah mengarah ke konten judi (\"gambling\"), pornografi (\"pornography\"), malware, atau aman (\"safe\").",
		"Bila tidak yakin, gunakan \"unknown\" — jangan menebak domain legit (mis. Wikipedia) sebagai berbahaya.",
		"Balas JSON object: {\"links\": [{\"url\": \"<url persis dari daftar>\", \"category\": \"gambling|pornography|malware|safe|unknown\", \"confidence\": 0.0}]}",
		"URL:\n" + strings.Join(lines, "\n"),
	}, "\n")

	result, err := d.callAgentLLM(ctx, "gatekeeper", prompt, true, "", 0)
	if err != nil {
		slog.Error("[link-health] klasifikasi AI gagal", "err", err)
		return nil, true
	}
	parsed := extractJSONObject(result.Content)
	entries := []any{}
	ok := false
	if parsed != nil {
		if arr, isArr := parsed["links"].([]any); isArr {
			entries = arr
			ok = true
		}
	}
	if !ok {
		if direct, isArr := extractJSONAny(result.Content).([]any); isArr {
			entries = direct
			ok = true
		}
	}
	if !ok {
		return nil, true
	}
	m := map[string]string{}
	for _, entry := range entries {
		obj, isObj := entry.(map[string]any)
		if !isObj {
			continue
		}
		url, _ := obj["url"].(string)
		category, _ := obj["category"].(string)
		if url != "" && validLinkCategories[category] {
			m[url] = category
		}
	}
	if len(m) == 0 {
		return nil, true
	}
	return m, false
}

// ---------- brand voice (port lib/gatekeeper/voice-check.ts) ----------

type voiceCheckResult struct {
	Level         string
	Notes         []string
	Deterministic map[string]any
	AIUsed        bool
}

const voiceHint = "Tone brand: netral, informatif, soft-selling; hindari bahasa agresif atau terlalu promotif."

// brandVoiceLimits: cek deterministik dulu; AI hanya sinyal tambahan saat
// level belum off_brand. Hasil informasional (tidak pernah gagal).
func (d *Deps) brandVoiceLimits(ctx context.Context, html, language string, minWords, maxWords int) voiceCheckResult {
	text := extractText(html)
	if text == "" {
		return voiceCheckResult{Level: "skipped", Notes: []string{}, Deterministic: map[string]any{"wordCount": 0, "promoHits": 0, "forbiddenHits": []string{}}}
	}
	wordCount := countWords(text)
	promoHits := countPromotional(text)
	forbiddenHits := []string{}

	minW := minWords
	maxW := maxWords
	if maxW == 0 {
		maxW = -1 // tidak dibatasi
	}
	promoVeryHigh := 4 * 2 // TOLERANCE_THRESHOLDS.low.fail * 2

	notes := []string{}
	level := ""
	switch {
	case minW > 0 && wordCount < int(float64(minW)*0.7):
		level = "off_brand"
		notes = append(notes, "Artikel terlalu pendek: "+itoa(wordCount)+" kata (min "+itoa(minW)+").")
	case len(forbiddenHits) > 0:
		level = "off_brand"
		notes = append(notes, "Frasa terlarang ditemukan: "+strings.Join(capStrings(forbiddenHits, 3), ", ")+".")
	case promoHits > promoVeryHigh:
		level = "off_brand"
		notes = append(notes, "Promosi berlebihan: "+itoa(promoHits)+" istilah promosi.")
	case (minW > 0 && wordCount < minW) || (maxW > 0 && wordCount > maxW):
		level = "minor_drift"
		notes = append(notes, "Jumlah kata "+itoa(wordCount)+" di luar rentang "+itoa(minW)+"-"+itoa(maxW)+".")
	case promoHits > 0:
		level = "minor_drift"
		notes = append(notes, "Ada "+itoa(promoHits)+" istilah promosi — kurangi nada jualan.")
	default:
		level = "consistent"
	}
	_ = language // aiAssist tidak diaktifkan oleh pemanggil Taut (parity)

	return voiceCheckResult{
		Level: level, Notes: notes,
		Deterministic: map[string]any{"wordCount": wordCount, "promoHits": promoHits, "forbiddenHits": forbiddenHits},
	}
}

func nowISO() string {
	return time.Now().UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z07:00")
}
