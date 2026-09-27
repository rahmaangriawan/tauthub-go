package brandflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/rand"
	"strings"

	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
)

// analysis.go: port lib/ai/analysis.ts (prompt, parse, persist) + helper kecil
// lain (dedupe, contains, random seed, markdown shim).

// contentHash: parity contentHash (sha256 64 char pertama).
func contentHash(topic, text, mediaID, standardVersion string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{topic, text, mediaID, standardVersion}, "|")))
	return hex.EncodeToString(sum[:])[:64]
}

// resolveStandardBlock: parity resolveEditorialStandard — prioritas standard
// media, lalu fallback media-type, else global.
func (d *Deps) resolveStandardBlock(ctx context.Context, mediaID, mediaType, brandID string) (string, string) {
	if mediaID != "" {
		if media, err := d.queryRow(ctx, "SELECT id, name, media_type FROM media WHERE id = $1 LIMIT 1", mediaID); err == nil && media != nil {
			if std := d.mediaStandardRow(ctx, mediaID); std != nil {
				return "media_specific", buildMediaPromptBlock(*std, media.strOr("name", ""))
			}
			if mediaType == "" {
				mediaType = media.strOr("media_type", "")
			}
		}
	}
	switch mediaType {
	case "mainstream":
		return "mainstream_fallback", mainstreamFallbackBlock
	case "blogger":
		return "blogger_fallback", bloggerFallbackBlock
	}
	return "global", ""
}

// buildMediaPromptBlock: parity buildMediaPromptBlock standard-resolver.
func buildMediaPromptBlock(media mediaStandard, name string) string {
	if name == "" {
		name = "Media"
	}
	lines := []string{
		"### STANDAR EDITORIAL KHUSUS MEDIA: " + name + " (per-media)",
		"Pedoman editorial khusus yang berlaku untuk media \"" + name + "\":",
	}
	if media.MinWords > 0 || media.MaxWords > 0 {
		part := ""
		if media.MinWords > 0 {
			part += "min " + itoa(media.MinWords)
		}
		if media.MaxWords > 0 {
			if part != "" {
				part += " "
			}
			part += "max " + itoa(media.MaxWords)
		}
		lines = append(lines, "- **Panjang Artikel**: "+part+" kata.")
	}
	if media.JournalisticStyle != "" {
		lines = append(lines, "- **Gaya Jurnalistik**: "+media.JournalisticStyle)
	}
	if media.HardSellingTol != "" {
		toleranceMap := map[string]string{
			"none": "Tidak boleh promosi sama sekali", "low": "Promosi minimal",
			"medium": "Promosi moderat", "high": "Promosi diperbolehkan",
		}
		label, ok := toleranceMap[media.HardSellingTol]
		if !ok {
			label = media.HardSellingTol
		}
		lines = append(lines, "- **Toleransi Hard Selling**: "+label)
	}
	if media.QuoteRequired {
		lines = append(lines, "- **Kutipan**: Wajib minimal "+itoa(media.MinQuotes)+" kutipan.")
	}
	if media.ImageRequired {
		lines = append(lines, "- **Gambar**: Wajib.")
	}
	if media.CategoryRequired {
		lines = append(lines, "- **Kategori**: Wajib.")
	}
	if len(media.RequiredInfo) > 0 {
		lines = append(lines, "- **Info Wajib**: "+strings.Join(media.RequiredInfo, ", "))
	}
	if len(media.ForbiddenContent) > 0 {
		lines = append(lines, "- **Konten Terlarang**: "+strings.Join(media.ForbiddenContent, ", "))
	}
	return strings.Join(lines, "\n")
}

const mainstreamFallbackBlock = `### STANDAR EDITORIAL FALLBACK: MEDIA MAINSTREAM (v1)
Pedoman jurnalistik wajib untuk media berita/publisher profesional (aturan HOW TO WRITE):
- **5W+1H**: Pastikan lead menjawab siapa, apa, kapan, di mana, mengapa, dan bagaimana.
- **Lead**: Informasi utama harus jelas dan muncul di awal.
- **Newsworthiness**: Artikel harus memiliki nilai berita; bukan brosur promosi.
- **Fakta dari source**: Fakta harus berasal dari bahan/sumber yang diberikan.
- **Atribusi jelas**: Setiap klaim/kutipan diberi atribusi yang jelas.
- **Kutipan**: Tidak diubah secara menyesatkan.
- **Larangan mengarang**: Tidak mengarang data, narasumber, atau fakta.
- **Struktur**: Berita natural — paragraf mengalir, heading TIDAK wajib.
- **Bahasa**: Jurnalistik formal, tidak berlebihan.
- **Anti-advertorial**: Bukan brosur promosi; promotional claim harus diatribusikan.
- **Headline**: Informatif, tidak clickbait berlebihan.`

const bloggerFallbackBlock = `### STANDAR EDITORIAL FALLBACK: MEDIA BLOGGER / EVERGREEN (v1)
Pedoman wajib untuk blog/publisher fleksibel (aturan HOW TO WRITE):
- **Konten dapat berupa**: Informasi, Tutorial, Panduan, Edukasi, Evergreen, Komersial, Review, Opini, Listicle, FAQ, atau konten niche.
- **Heading**: Boleh digunakan untuk struktur yang membantu pembaca.
- **Struktur**: Fleksibel mengikuti intent konten.
- **SEO/readability**: Boleh dipertimbangkan (heading, paragraf pendek, subjudul informatif).
- **Gaya**: Boleh conversational jika sesuai target pembaca.
- **CTA**: Boleh digunakan bila relevan.
- **Tidak wajib 5W+1H**: Struktur berita tidak dipaksakan.
- **Larangan**: Jangan spam/keyword stuffing; jangan mengarang fakta/angka/narasumber.`

// buildAnalysisPrompt: port buildAnalysisPrompt (lib/ai/analysis.ts).
func buildAnalysisPrompt(topic, content, mediaType, mediaID string, intake bsRow, promptBlock, standardSource string) string {
	mediaTypeLabel := "Umum"
	if mediaType == "blogger" {
		mediaTypeLabel = "Media Blogger"
	} else if mediaType == "mainstream" {
		mediaTypeLabel = "Media Mainstream"
	}
	standardLabel := "Standard editorial global"
	switch standardSource {
	case "media_specific":
		standardLabel = "Standard khusus media"
	case "mainstream_fallback":
		standardLabel = "Standard Jurnalistik Mainstream (fallback)"
	case "blogger_fallback":
		standardLabel = "Standard Blog / Evergreen (fallback)"
	}

	answerLines := ""
	if answers, ok := intake.jsonField("answers").(map[string]any); ok {
		parts := []string{}
		for _, key := range sortedKeys(answers) {
			if v, ok := answers[key].(string); ok && strings.TrimSpace(v) != "" {
				parts = append(parts, "- "+key+": "+truncateString(v, 500))
			}
		}
		answerLines = strings.Join(parts, "\n")
	}
	if answerLines == "" {
		answerLines = "(tidak ada)"
	}

	var instruction string
	switch mediaType {
	case "mainstream":
		instruction = `
Output JSON dengan struktur berikut (untuk media mainstream):
{
  "editorial_score": <0-100>,
  "diagnosis": "<1-2 kalimat diagnosis>",
  "strengths": ["<poin positif>", "..."],
  "violations": ["<pelanggaran/bias>", "..."],
  "headline_suggestions": ["<2-5 judul informatif, newsworthy, tidak clickbait>", "..."],
  "revised_draft": "<draf hasil revisi berdasarkan standard media, jangan mengarang fakta baru>"
}`
	case "blogger":
		instruction = `
Output JSON dengan struktur berikut (untuk media blogger):
{
  "content_type_detected": "<Informasi|Tutorial|Panduan|Edukasi|Evergreen|Komersial|Review|Opini|Listicle|FAQ|Niche Content>",
  "editorial_fit_score": <0-100>,
  "readability": <0-100>,
  "clarity": <0-100>,
  "search_intent": "<intent pencarian>",
  "structure_score": <0-100>,
  "potential_value": "<nilai potensial artikel>",
  "headline_suggestions": ["<2-5 judul berbasis search intent/informatif>", "..."],
  "revised_draft": "<draf hasil revisi sesuai gaya blog/evergreen>"
}`
	default:
		instruction = `
Output JSON dengan struktur:
{
  "editorial_score": <0-100>,
  "diagnosis": "...",
  "strengths": ["..."],
  "violations": ["..."],
  "content_type_detected": "...",
  "headline_suggestions": ["..."],
  "revised_draft": "..."
}`
	}

	mediaTarget := "Belum ada media dipilih"
	if mediaID != "" {
		mediaTarget = "Media dipilih"
	}
	block := ""
	if promptBlock != "" {
		block = "\n" + promptBlock
	}
	return "Anda adalah editor profesional di platform Tauthub. Analisis naskah berikut berdasarkan konteks editorial yang diberikan.\n\n" +
		"KONTEKS EDITORIAL:\n" +
		"- Media Target: " + mediaTarget + "\n" +
		"- Media Type: " + mediaTypeLabel + "\n" +
		"- Standard: " + standardLabel + "\n" + block + "\n\n" +
		"Bahan/Answer:\n" + answerLines + "\n\n" +
		"Topik:\n" + orDash(topic) + "\n\n" +
		"NASKAH YANG DIANALISIS:\n" + truncateString(content, 12000) + "\n\n" +
		instruction + "\n\n" +
		"ATURAN:\n" +
		"- Jangan mengarang fakta, angka, narasumber, atau kutipan.\n" +
		"- Jika sumber tidak cukup, tandai sebagai perlu verifikasi, jangan mengisi kekosongan dengan asumsi.\n" +
		"- Jangan membuat heading otomatis hanya untuk struktur.\n" +
		"- Untuk media mainstream: heading OPTIONAL, artikel boleh berupa paragraf natural.\n" +
		"- Untuk media blogger: jangan paksa format berita 5W+1H.\n" +
		"- Hasilkan HANYA JSON valid tanpa teks lain."
}

// parseAnalysisResult: port parseAnalysis (lib/ai/analysis.ts).
func parseAnalysisResult(content, standardSource string) map[string]any {
	out := emptyAnalysisResult(standardSource, nil, nil)
	block := extractJSONBlock(content)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(block), &parsed); err != nil {
		if m := reScoreText.FindStringSubmatch(content); m != nil {
			if v, err := parseIntSafe(m[1]); err == nil {
				out["editorial_score"] = clampIntValue(v, 0, 100)
			}
		}
		return out
	}
	out["editorial_score"] = numOrNullRange(parsed["editorial_score"])
	out["diagnosis"] = stringOrNull(parsed["diagnosis"])
	out["strengths"] = stringArrayOrEmpty(parsed["strengths"])
	out["violations"] = stringArrayOrEmpty(parsed["violations"])
	out["content_type_detected"] = stringOrNull(parsed["content_type_detected"])
	out["editorial_fit_score"] = numOrNullRange(parsed["editorial_fit_score"])
	out["readability"] = numOrNullRange(parsed["readability"])
	out["clarity"] = numOrNullRange(parsed["clarity"])
	out["search_intent"] = stringOrNull(parsed["search_intent"])
	out["structure_score"] = numOrNullRange(parsed["structure_score"])
	out["potential_value"] = stringOrNull(parsed["potential_value"])
	out["headline_suggestions"] = capAnyArray(stringArrayOrEmpty(parsed["headline_suggestions"]), 5)
	out["revised_draft"] = stringOrNull(parsed["revised_draft"])
	return out
}

// emptyAnalysisResult: parity struktur AnalysisResult kosong.
func emptyAnalysisResult(standardSource string, standardVersion, fallbackType any) map[string]any {
	return map[string]any{
		"editorial_score":       nil,
		"diagnosis":             nil,
		"strengths":             []string{},
		"violations":            []string{},
		"content_type_detected": nil,
		"editorial_fit_score":   nil,
		"readability":           nil,
		"clarity":               nil,
		"search_intent":         nil,
		"structure_score":       nil,
		"potential_value":       nil,
		"headline_suggestions":  []string{},
		"revised_draft":         nil,
		"standard_source":       standardSource,
		"standard_version":      standardVersion,
		"fallback_type":         fallbackType,
	}
}

// analysisRowToJSON: baris content_ai_analyses → JSON AnalysisResult.
func analysisRowToJSON(row bsRow) map[string]any {
	return map[string]any{
		"editorial_score":       row.numPtr("editorial_score"),
		"diagnosis":             row.raw("diagnosis"),
		"strengths":             jsonStringList(row.jsonField("strengths")),
		"violations":            jsonStringList(row.jsonField("violations")),
		"content_type_detected": row.raw("content_type_detected"),
		"editorial_fit_score":   row.numPtr("editorial_fit_score"),
		"readability":           row.numPtr("readability"),
		"clarity":               row.numPtr("clarity"),
		"search_intent":         row.raw("search_intent"),
		"structure_score":       row.numPtr("structure_score"),
		"potential_value":       row.raw("potential_value"),
		"headline_suggestions":  jsonStringList(row.jsonField("headline_suggestions")),
		"revised_draft":         row.raw("revised_draft"),
		"standard_source":       row.raw("standard_source"),
		"standard_version":      row.raw("standard_version"),
		"fallback_type":         row.raw("fallback_type"),
	}
}

func extractJSONBlock(text string) string {
	if m := reJSONBlock.FindString(text); m != "" {
		return m
	}
	return text
}

func numOrNullRange(v any) any {
	f, ok := toFloat(v)
	if !ok {
		return nil
	}
	return clampIntValue(int(math.Round(f)), 0, 100)
}

func stringOrNull(v any) any {
	if s, ok := v.(string); ok {
		return s
	}
	return nil
}

func stringArrayOrEmpty(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func capAnyArray(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func clampIntValue(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func orDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

// ---------- helper kecil ----------

func dedupeStrings(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// randomSeed: parity Math.floor(Math.random() * 2**31).
func randomSeed() int {
	return rand.Intn(1 << 31)
}

// randInt: bilangan bulat acak [0, n).
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return rand.Intn(n)
}

// markdownToHTML: shim internal/content.MarkdownToHtml.
func markdownToHTML(md string) string {
	return contentengine.MarkdownToHtml(md)
}

// rowToAnyMap: bsRow → map[string]any (kunci seleksi SELECT * apa adanya).
func rowToAnyMap(row bsRow) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		out[k] = v
	}
	return out
}
