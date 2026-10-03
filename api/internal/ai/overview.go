// overview.go: penilaian kesiapan artikel untuk Google AI Overviews
// (LLM-friendly). Bagian MURNI (prompt, parsing, aturan gate) dipakai bersama
// brandflow & clienttaut supaya skema hasil identik dan bisa diuji; pemanggilan
// LLM tetap di masing-masing domain (config agent gatekeeper masing-masing).
package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AIOverviewMaxScore: total maksimum 4 indikator × skor 10.
const AIOverviewMaxScore = 40

const (
	aiOverviewDefaultTotal   = 32
	aiOverviewDefaultMinItem = 7
	aiOverviewDefaultMaxChar = 8000
)

// AIOverviewSystemPrompt: penilai kesiapan AI Overview — skor 4 indikator 1-10,
// ringkasan maks 2 kalimat per indikator, catatan revisi maks 3 hanya bila ada
// indikator berskor di bawah 7. Dipakai dengan json_mode.
const AIOverviewSystemPrompt = `Anda adalah "Gatekeeper AI Overview QC Agent" untuk platform penerbitan Indonesia (Tauthub). Tugas: menilai apakah draf artikel siap dibaca dan dikutip oleh Google AI Overviews / LLM pencarian.

Nilai 4 indikator, masing-masing skor 1-10 (1 = tidak ada sama sekali, 10 = sempurna):
1. struktur — Struktur & Pemformatan: apakah jawaban/definisi utama sudah ada di 150 kata pertama (BLUF); apakah heading bermakna (untuk media blogger, heading berbasis pertanyaan seperti "Apa itu X?" menaikkan skor); apakah informasi kompleks dipecah dengan list/tabel. PENTING: untuk media mainstream artikel MEMANG tidak memakai heading — jangan turunkan skor karena ketiadaan H2/H3, nilai BLUF dan paragraf pendeknya.
2. entitas — Kepadatan Entitas & Semantik: banyak entitas konkret yang dikenali (nama tokoh/lembaga, lokasi, tanggal, angka, persentase, istilah teknis); rasio sinyal-terhadap-noise tinggi (sedikit kalimat pengisi/fluff); kedalaman sub-topik (LSI) memadai.
3. eeat — Sinyal E-E-A-T & Orisinalitas: ada atribusi/kutipan/rujukan otoritatif atau data yang bisa diverifikasi; klaim objektif dan tidak hiperbolik; ada nilai tambah (information gain), bukan sekadar merangkum informasi umum.
4. niat_pengguna — Penyelarasan Niat Pengguna: menjawab pertanyaan turunan yang lazim (People Also Ask); kesimpulan memberi rekomendasi spesifik/bersyarat, bukan penutup basa-basi.

Aturan keluaran:
- ringkasan: maksimal 2 kalimat bahasa Indonesia per indikator.
- catatan_revisi: maksimal 3 poin perbaikan SANGAT spesifik (sebut bagian/paragrafnya), HANYA bila artikel TIDAK LULUS menurut aturan kelulusan di input pengguna (total di bawah ambang ATAU ada indikator di bawah skor minimum). Bila lulus, kirim array kosong.
- Balas HANYA JSON valid tanpa markdown/penjelasan lain:
{"indikator":[{"kunci":"struktur","skor":1-10,"ringkasan":"..."},{"kunci":"entitas","skor":1-10,"ringkasan":"..."},{"kunci":"eeat","skor":1-10,"ringkasan":"..."},{"kunci":"niat_pengguna","skor":1-10,"ringkasan":"..."}],"catatan_revisi":[]}`

// AIOverviewSettings: toggle + ambang (dari taut_agent_configs.settings.aiOverview).
type AIOverviewSettings struct {
	Enabled          bool
	PassTotal        int
	PassMinIndicator int
	MaxChars         int
}

// DefaultAIOverviewSettings: enabled + ambang standar (32/7/8000).
func DefaultAIOverviewSettings() AIOverviewSettings {
	return AIOverviewSettings{
		Enabled: true, PassTotal: aiOverviewDefaultTotal,
		PassMinIndicator: aiOverviewDefaultMinItem, MaxChars: aiOverviewDefaultMaxChar,
	}
}

// AIOverviewSettingsFromJSON: parse settings.aiOverview dari jsonb (map atau string JSON).
func AIOverviewSettingsFromJSON(raw any) AIOverviewSettings {
	st := DefaultAIOverviewSettings()
	m, _ := raw.(map[string]any)
	if m == nil {
		if s, isStr := raw.(string); isStr {
			var parsed map[string]any
			if json.Unmarshal([]byte(s), &parsed) == nil {
				m = parsed
			}
		}
	}
	if m == nil {
		return st
	}
	if v, isBool := m["enabled"].(bool); isBool {
		st.Enabled = v
	}
	if v, ok := overviewNum(m["pass_total"]); ok && v > 0 && v <= AIOverviewMaxScore {
		st.PassTotal = int(v)
	}
	if v, ok := overviewNum(m["pass_min_indicator"]); ok && v > 0 && v <= 10 {
		st.PassMinIndicator = int(v)
	}
	if v, ok := overviewNum(m["max_chars"]); ok && v >= 1000 {
		st.MaxChars = int(v)
	}
	return st
}

// AIOverviewUserPrompt: prompt penilaian (judul + tipe media + jumlah kata + HTML).
// Tipe media menentukan konteks: mainstream tanpa heading tidak dipenalti.
func AIOverviewUserPrompt(title, mediaType, contentHTML string, wordCount int, st AIOverviewSettings) string {
	typeName := "mainstream (berita, tanpa heading)"
	if mediaType == "blogger" {
		typeName = "blogger (artikel evergreen, memakai heading)"
	}
	max := st.MaxChars
	if max <= 0 {
		max = aiOverviewDefaultMaxChar
	}
	html := contentHTML
	if len(html) > max {
		html = html[:max]
	}
	return strings.Join([]string{
		"Judul: " + title,
		"Tipe media: " + typeName,
		"Jumlah kata: " + fmt.Sprintf("%d", wordCount),
		fmt.Sprintf("Aturan kelulusan: total minimal %d dari %d DAN setiap indikator minimal %d. Bila tidak memenuhi, sertakan catatan_revisi (maks 3) yang sangat spesifik.", st.PassTotal, AIOverviewMaxScore, st.PassMinIndicator),
		"Isi (HTML): " + html,
	}, "\n")
}

// AIOverviewIndicator: satu indikator hasil penilaian.
type AIOverviewIndicator struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Score   int    `json:"score"`
	Summary string `json:"summary"`
}

// aiOverviewCanonical: urutan + judul tampilan indikator.
var aiOverviewCanonical = []struct{ Key, Title string }{
	{"struktur", "Struktur & Pemformatan"},
	{"entitas", "Kepadatan Entitas & Semantik"},
	{"eeat", "Sinyal E-E-A-T & Orisinalitas"},
	{"niatPengguna", "Penyelarasan Niat Pengguna"},
}

// AIOverviewResult: hasil penilaian (dipetakan ke checks.aiOverview via JSON()).
type AIOverviewResult struct {
	Status     string
	Verdict    string
	Total      int
	Indicators []AIOverviewIndicator
	Notes      []string
	CheckedAt  string
	Error      string
	Info       string
}

// JSON: bentuk yang disimpan di taut_gatekeeper_audits.checks.aiOverview.
func (r AIOverviewResult) JSON() map[string]any {
	out := map[string]any{
		"status": r.Status, "maxScore": AIOverviewMaxScore,
		"totalScore": r.Total, "indicators": r.Indicators,
		"revisionNotes": r.Notes, "checkedAt": r.CheckedAt,
	}
	if r.Verdict != "" {
		out["verdict"] = r.Verdict
	}
	if r.Error != "" {
		out["error"] = r.Error
	}
	if r.Info != "" {
		out["info"] = r.Info
	}
	return out
}

// EvaluateAIOverview: parse balasan JSON LLM + terapkan aturan gate di Go.
// LULUS = total >= PassTotal DAN tidak ada indikator < PassMinIndicator.
// Error/parse gagal → review_required + error (non-blocking).
func EvaluateAIOverview(llmContent string, st AIOverviewSettings, checkedAt string) AIOverviewResult {
	parsed := firstJSONObject(llmContent)
	if parsed == nil {
		return AIOverviewResult{Status: "review_required", CheckedAt: checkedAt, Error: "Respons AI Overview tidak valid."}
	}
	scores := map[string]AIOverviewIndicator{}
	if arr, isArr := parsed["indikator"].([]any); isArr {
		for _, entry := range arr {
			obj, isObj := entry.(map[string]any)
			if !isObj {
				continue
			}
			key := aiOverviewKeyOf(obj)
			if key == "" {
				continue
			}
			scores[key] = AIOverviewIndicator{Key: key, Score: aiOverviewScoreOf(obj), Summary: aiOverviewSummaryOf(obj)}
		}
	}
	if len(scores) < len(aiOverviewCanonical) {
		return AIOverviewResult{Status: "review_required", CheckedAt: checkedAt, Error: "AI Overview tidak mengembalikan 4 indikator lengkap."}
	}

	indicators := make([]AIOverviewIndicator, 0, len(aiOverviewCanonical))
	total, minScore := 0, 10
	for _, c := range aiOverviewCanonical {
		e := scores[c.Key]
		e.Title = c.Title
		indicators = append(indicators, e)
		total += e.Score
		if e.Score < minScore {
			minScore = e.Score
		}
	}
	passed := total >= st.PassTotal && minScore >= st.PassMinIndicator
	res := AIOverviewResult{
		Total: total, Indicators: indicators, CheckedAt: checkedAt,
		Status: "review_required", Verdict: "ditolak", Notes: []string{},
	}
	if passed {
		res.Status, res.Verdict = "passed", "lulus"
		return res
	}
	if arr, isArr := parsed["catatan_revisi"].([]any); isArr {
		for _, n := range arr {
			s, isStr := n.(string)
			if !isStr {
				s = fmt.Sprint(n)
			}
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				continue
			}
			if len([]rune(trimmed)) > 300 {
				trimmed = string([]rune(trimmed)[:300])
			}
			res.Notes = append(res.Notes, trimmed)
			if len(res.Notes) >= 3 {
				break
			}
		}
	}
	return res
}

// AIOverviewRejected: penilaian menolak (bukan error teknis/pending).
func AIOverviewRejected(entry map[string]any) bool {
	if entry == nil {
		return false
	}
	status, _ := entry["status"].(string)
	verdict, _ := entry["verdict"].(string)
	return status == "review_required" && verdict == "ditolak"
}

// AIOverviewRevisionNotes: catatan revisi (maks 3) — hanya saat penilaian menolak.
func AIOverviewRevisionNotes(entry map[string]any) []string {
	if !AIOverviewRejected(entry) {
		return nil
	}
	switch v := entry["revisionNotes"].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, isStr := item.(string); isStr && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// firstJSONObject: objek JSON pertama dalam teks (toleran pagar markdown).
func firstJSONObject(raw string) map[string]any {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(raw[start:end+1]), &parsed) != nil {
		return nil
	}
	return parsed
}

// aiOverviewKeyOf: normalisasi kunci indikator (terima variasi ejaan dari LLM).
func aiOverviewKeyOf(obj map[string]any) string {
	for _, field := range []string{"kunci", "key", "indikator", "indicator"} {
		raw, isStr := obj[field].(string)
		if !isStr {
			continue
		}
		norm := ""
		for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				norm += string(r)
			}
		}
		switch norm {
		case "struktur", "structure", "format":
			return "struktur"
		case "entitas", "entity", "entities", "semantik":
			return "entitas"
		case "eeat", "eaat", "otoritas", "orisinalitas":
			return "eeat"
		case "niatpengguna", "niat", "intent", "userintent", "intentpengguna", "pengguna":
			return "niatPengguna"
		}
	}
	return ""
}

// aiOverviewScoreOf: skor 0-10 (dibulatkan, di-clamp).
func aiOverviewScoreOf(obj map[string]any) int {
	for _, field := range []string{"skor", "score"} {
		if raw, exists := obj[field]; exists {
			if v, ok := overviewNum(raw); ok {
				n := int(v + 0.5)
				if n < 0 {
					n = 0
				}
				if n > 10 {
					n = 10
				}
				return n
			}
		}
	}
	return 0
}

// aiOverviewSummaryOf: ringkasan analitis per indikator (maks 300 karakter).
func aiOverviewSummaryOf(obj map[string]any) string {
	for _, field := range []string{"ringkasan", "summary", "analisis", "catatan"} {
		if s, isStr := obj[field].(string); isStr {
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				continue
			}
			if len([]rune(trimmed)) > 300 {
				trimmed = string([]rune(trimmed)[:300])
			}
			return trimmed
		}
	}
	return ""
}

// overviewNum: angka dari json (float64) atau string numerik.
func overviewNum(raw any) (float64, bool) {
	switch n := raw.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(n), "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}
