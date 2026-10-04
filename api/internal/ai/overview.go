// overview.go: penilaian kesiapan artikel untuk Google AI Overviews
// (LLM-friendly). Bagian MURNI (prompt, parsing, aturan gate) dipakai bersama
// brandflow & clienttaut supaya skema hasil identik dan bisa diuji; pemanggilan
// LLM tetap di masing-masing domain (config agent gatekeeper masing-masing).
package ai

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// AIOverviewMaxScore: total maksimum 4 indikator × skor 10.
const AIOverviewMaxScore = 40

const (
	aiOverviewDefaultTotal   = 32
	aiOverviewDefaultMinItem = 7
	aiOverviewDefaultMaxChar = 8000
	// aiOverviewDefaultBriefChars: batas karakter BAHAN/BRIEF yang dikirim ke
	// penilai. Bahan kini menggabungkan brief Profiler + materi klien + topik
	// + kutipan narasumber (lihat tautReferenceMaterial di brandflow), jadi
	// lebih besar dari batas HTML lama supaya tidak ada sumber yang hilang.
	aiOverviewDefaultBriefChars = 12000
)

// AIOverviewIndicatorDef: definisi satu indikator rubrik AI Overview. Ini
// SATU sumber kebenaran yang dipakai penilai (Gatekeeper) dan penulis (The
// Writer), supaya prompt penulisan dan prompt penilaian tidak bisa menyimpang:
// menambah atau mengubah indikator cukup di sini.
type AIOverviewIndicatorDef struct {
	// Key: kunci kanonik internal (dipakai di checks JSON & tampilan UI).
	Key string
	// PromptKey: kunci yang diminta di JSON keluaran LLM (mis. niat_pengguna).
	PromptKey string
	// Title: judul tampilan indikator.
	Title string
	// GraderRule: definisi indikator untuk penilai.
	GraderRule string
	// WriterRule: arahan singkat agar penulis menulis untuk indikator ini.
	WriterRule string
}

// AIOverviewIndicators: 4 indikator rubrik, urutan kanonik.
var AIOverviewIndicators = []AIOverviewIndicatorDef{
	{
		Key: "struktur", PromptKey: "struktur", Title: "Struktur & Pemformatan",
		GraderRule: "apakah jawaban/definisi utama sudah ada di 150 kata pertama (BLUF); apakah heading bermakna (untuk media blogger, heading berbasis pertanyaan seperti \"Apa itu X?\" menaikkan skor); apakah informasi kompleks dipecah dengan list/tabel. PENTING: untuk media mainstream artikel MEMANG tidak memakai heading — jangan turunkan skor karena ketiadaan H2/H3, nilai BLUF dan paragraf pendeknya.",
		WriterRule: "definisi/jawaban inti langsung di 150 kata pertama (BLUF), paragraf pendek, dan kesimpulan yang menegaskan jawaban utama.",
	},
	{
		Key: "entitas", PromptKey: "entitas", Title: "Kepadatan Entitas & Semantik",
		GraderRule: "banyak entitas konkret yang dikenali (nama tokoh/lembaga, lokasi, tanggal, angka, persentase, istilah teknis); rasio sinyal-terhadap-noise tinggi (sedikit kalimat pengisi/fluff); kedalaman sub-topik (LSI) memadai.",
		WriterRule: "sertakan entitas konkret yang ada di bahan (nama orang/lembaga, lokasi, tanggal, angka, persentase, istilah teknis) dan bahas sub-topik terkait, bukan sekadar mengulang kata kunci.",
	},
	{
		Key: "eeat", PromptKey: "eeat", Title: "Sinyal E-E-A-T & Orisinalitas",
		GraderRule: "ada atribusi/kutipan/rujukan otoritatif atau data yang bisa diverifikasi; klaim objektif dan tidak hiperbolik; ada nilai tambah (information gain), bukan sekadar merangkum informasi umum.",
		WriterRule: "kaitkan klaim faktual dengan bahan/brief yang diberikan (mis. \"menurut data pada brief\"), JANGAN mengarang angka/nama/jabatan; hindari bahasa hiperbolik dan kalimat pengisi; beri nilai tambah berupa sudut pandang, sintesis, atau implikasi praktis — bukan rangkuman umum.",
	},
	{
		Key: "niatPengguna", PromptKey: "niat_pengguna", Title: "Penyelarasan Niat Pengguna",
		GraderRule: "menjawab pertanyaan turunan yang lazim (People Also Ask); kesimpulan memberi rekomendasi spesifik/bersyarat, bukan penutup basa-basi.",
		WriterRule: "jawab proaktif 2-3 pertanyaan turunan yang lazim (biaya, cara, perbandingan, risiko) dan tutup dengan rekomendasi bersyarat yang spesifik (contoh: \"Pilih A bila …, pilih B bila …\"), bukan penutup basa-basi.",
	},
}

// aiOverviewRoleText: deskripsi peran penilai.
const aiOverviewRoleText = `Anda adalah "Gatekeeper AI Overview QC Agent" untuk platform penerbitan Indonesia (Tauthub). Tugas: menilai apakah draf artikel siap dibaca dan dikutip oleh Google AI Overviews / LLM pencarian.`

// aiOverviewVerificationText: aturan verifikasi klaim ke bahan rujukan.
const aiOverviewVerificationText = `VERIFIKASI KE BAHAN (WAJIB): bandingkan setiap nama tokoh, nama perusahaan/lembaga, studi kasus, kutipan, dan angka spesifik di artikel dengan BAHAN/BRIEF yang diberikan. Bila ada yang TIDAK ada di bahan, itu indikasi klaim tanpa dasar: turunkan skor indikator entitas dan E-E-A-T, lalu masukkan temuannya ke catatan_revisi dan ke "entitas_tanpa_dasar". Bila bahan kosong, tandai klaim spesifik apa pun sebagai tanpa dasar.`

// aiOverviewOutputRulesText: aturan keluaran penilai.
const aiOverviewOutputRulesText = `Aturan keluaran:
- ringkasan: maksimal 2 kalimat bahasa Indonesia per indikator.
- catatan_revisi: maksimal 3 poin perbaikan SANGAT spesifik (sebut bagian/paragrafnya), HANYA bila artikel TIDAK LULUS menurut aturan kelulusan di input pengguna (total di bawah ambang ATAU ada indikator di bawah skor minimum). Bila lulus, kirim array kosong.
- entitas_tanpa_dasar: maksimal 3 nama/angka/studi kasus di artikel yang tidak ditemukan di bahan (kosongkan bila semua berdasar bahan).`

// buildAIOverviewRules: rangkai prompt penilai dari definisi indikator bersama.
// aiOverviewRules: deskripsi peran + 4 indikator + verifikasi ke bahan.
// Dipisah dari skema keluaran supaya bisa dikomposisi dengan cek lain
// (lihat GuardSystemPrompt di guard.go) tanpa menggandakan teksnya.
func buildAIOverviewRules() string {
	var b strings.Builder
	b.WriteString(aiOverviewRoleText)
	b.WriteString("\n\nNilai 4 indikator, masing-masing skor 1-10 (1 = tidak ada sama sekali, 10 = sempurna):\n")
	for i, ind := range AIOverviewIndicators {
		b.WriteString(fmt.Sprintf("%d. %s — %s: %s\n", i+1, ind.PromptKey, ind.Title, ind.GraderRule))
	}
	b.WriteString("\n")
	b.WriteString(aiOverviewVerificationText)
	b.WriteString("\n\n")
	b.WriteString(aiOverviewOutputRulesText)
	return b.String()
}

var aiOverviewRules = buildAIOverviewRules()

// AIOverviewIndicatorSchemaFragment: potongan skema JSON [{kunci,skor,ringkasan}]
// dari definisi indikator. Dipakai skema tunggal (AI Overview saja) maupun
// skema gabungan safety+overview di guard.go, supaya kunci tidak menyimpang.
func AIOverviewIndicatorSchemaFragment() string {
	parts := make([]string, 0, len(AIOverviewIndicators))
	for _, ind := range AIOverviewIndicators {
		parts = append(parts, fmt.Sprintf(`{"kunci":"%s","skor":1-10,"ringkasan":"..."}`, ind.PromptKey))
	}
	return strings.Join(parts, ",")
}

func buildAIOverviewOutputSchema() string {
	return "Balas HANYA JSON valid tanpa markdown/penjelasan lain:\n" +
		`{"indikator":[` + AIOverviewIndicatorSchemaFragment() + `],"catatan_revisi":[],"entitas_tanpa_dasar":[]}`
}

var aiOverviewOutputSchema = buildAIOverviewOutputSchema()

// AIOverviewSystemPrompt: prompt lengkap (rules + skema) untuk cek AI Overview saja.
var AIOverviewSystemPrompt = aiOverviewRules + "\n\n" + aiOverviewOutputSchema

// AIOverviewPassRule: kalimat aturan kelulusan (ambang dari settings). Dipakai
// bersama oleh prompt penilai dan prompt penulis agar angkanya tidak menyimpang.
func AIOverviewPassRule(st AIOverviewSettings) string {
	return fmt.Sprintf("total minimal %d dari %d DAN setiap indikator minimal %d",
		st.PassTotal, AIOverviewMaxScore, st.PassMinIndicator)
}

// AIOverviewWriterRubric: arahan rubrik untuk The Writer, dibangun dari definisi
// indikator yang SAMA dengan penilai, ditambah catatan format per tipe media.
// Ini menggantikan salinan manual rubrik di prompt penulis.
func AIOverviewWriterRubric(st AIOverviewSettings, mediaType string) []string {
	lines := []string{
		fmt.Sprintf("Rubrik AI Overview (WAJIB DIPENUHI): artikel dinilai %d indikator skor 1-10 dan dinyatakan LULUS bila %s. Tulis agar keempatnya kuat sejak awal.",
			len(AIOverviewIndicators), AIOverviewPassRule(st)),
	}
	for i, ind := range AIOverviewIndicators {
		lines = append(lines, fmt.Sprintf("Indikator %d — %s: %s", i+1, ind.Title, ind.WriterRule))
	}
	if mediaType == "blogger" {
		lines = append(lines, "Format tambahan (blogger): jawaban pertanyaan turunan memakai subjudul <h2>/<h3> berbasis pertanyaan yang lazim dicari (\"Apa itu X?\", \"Bagaimana cara Y?\") dan data komparatif/langkah ditulis sebagai list/tabel.")
	} else {
		lines = append(lines, "Format tambahan (mainstream): TETAP tanpa heading dan tanpa list — jawab pertanyaan turunan dalam paragraf mengalir, lalu tutup dengan rekomendasi bersyarat.")
	}
	return lines
}

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
func AIOverviewUserPrompt(title, mediaType, contentHTML string, wordCount int, st AIOverviewSettings, brief string) string {
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
	briefText := strings.TrimSpace(brief)
	if briefText == "" {
		briefText = "(KOSONG — tidak ada brief/bahan rujukan untuk generate artikel ini)"
	} else if utf8.RuneCountInString(briefText) > aiOverviewDefaultBriefChars {
		briefText = string([]rune(briefText)[:aiOverviewDefaultBriefChars])
	}
	return strings.Join([]string{
		"Judul: " + title,
		"Tipe media: " + typeName,
		"Jumlah kata: " + strconv.Itoa(wordCount),
		fmt.Sprintf("Aturan kelulusan: %s. Bila tidak memenuhi, sertakan catatan_revisi (maks 3) yang sangat spesifik.", AIOverviewPassRule(st)),
		"BAHAN/BRIEF (rujukan fakta yang boleh dipakai artikel):\n" + briefText,
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

// aiOverviewCanonical: urutan + judul tampilan indikator (turunan dari
// AIOverviewIndicators supaya tidak ada daftar kembar).
var aiOverviewCanonical = func() []struct{ Key, Title string } {
	out := make([]struct{ Key, Title string }, 0, len(AIOverviewIndicators))
	for _, ind := range AIOverviewIndicators {
		out = append(out, struct{ Key, Title string }{ind.Key, ind.Title})
	}
	return out
}()

// AIOverviewResult: hasil penilaian (dipetakan ke checks.aiOverview via JSON()).
type AIOverviewResult struct {
	Status     string
	Verdict    string
	Total      int
	Indicators []AIOverviewIndicator
	Notes      []string
	// UnsourcedEntities: nama/angka/studi kasus di artikel yang tidak ada di bahan
	// (indikasi klaim tanpa dasar). BriefAvailable=false berarti bahan kosong.
	UnsourcedEntities []string
	BriefAvailable    bool
	CheckedAt         string
	Error             string
	Info              string
}

// JSON: bentuk yang disimpan di taut_gatekeeper_audits.checks.aiOverview.
func (r AIOverviewResult) JSON() map[string]any {
	out := map[string]any{
		"status": r.Status, "maxScore": AIOverviewMaxScore,
		"totalScore": r.Total, "indicators": r.Indicators,
		"revisionNotes": r.Notes, "checkedAt": r.CheckedAt,
		"unsourcedEntities": r.UnsourcedEntities, "briefAvailable": r.BriefAvailable,
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
func EvaluateAIOverview(llmContent string, st AIOverviewSettings, checkedAt string, briefAvailable bool) AIOverviewResult {
	parsed := firstJSONObject(llmContent)
	if parsed == nil {
		return AIOverviewResult{Status: "review_required", CheckedAt: checkedAt, Error: "Respons AI Overview tidak valid.", BriefAvailable: briefAvailable, UnsourcedEntities: []string{}}
	}
	return EvaluateAIOverviewFields(parsed, st, checkedAt, briefAvailable)
}

// EvaluateAIOverviewFields: evaluasi dari map yang sudah diparse (dipakai jalur
// gabungan safety+overview, di mana objek "overview" diambil dari respons besar).
func EvaluateAIOverviewFields(parsed map[string]any, st AIOverviewSettings, checkedAt string, briefAvailable bool) AIOverviewResult {
	if parsed == nil {
		return AIOverviewResult{Status: "review_required", CheckedAt: checkedAt, Error: "Respons AI Overview tidak valid.", BriefAvailable: briefAvailable, UnsourcedEntities: []string{}}
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
		return AIOverviewResult{Status: "review_required", CheckedAt: checkedAt, Error: "AI Overview tidak mengembalikan 4 indikator lengkap.", BriefAvailable: briefAvailable, UnsourcedEntities: []string{}}
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
		BriefAvailable: briefAvailable, UnsourcedEntities: []string{},
	}
	if arr, isArr := parsed["entitas_tanpa_dasar"].([]any); isArr {
		for _, item := range arr {
			s, isStr := item.(string)
			if !isStr {
				s = fmt.Sprint(item)
			}
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				continue
			}
			if utf8.RuneCountInString(trimmed) > 160 {
				trimmed = string([]rune(trimmed)[:160])
			}
			res.UnsourcedEntities = append(res.UnsourcedEntities, trimmed)
			if len(res.UnsourcedEntities) >= 3 {
				break
			}
		}
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
			if utf8.RuneCountInString(trimmed) > 300 {
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
		var nb strings.Builder
		for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				nb.WriteRune(r)
			}
		}
		norm := nb.String()
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
			if utf8.RuneCountInString(trimmed) > 300 {
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
