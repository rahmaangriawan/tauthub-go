package brandflow

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// media_matchmaker.go: POST /v1/brands/{brandId}/media/matchmaker
//
// Dua agent berversi, satu pipeline:
//  1. media_matchmaker — menerjemahkan brief bebas menjadi kriteria
//     terstruktur (ringkasan kebutuhan).
//  2. media_matchmaker_select — memilih paket dari kandidat katalog nyata
//     berdasarkan ringkasan itu; hanya pada fase "Pilih media" eksplisit.
//
// Penyaringan kandidat, harga, skor, dan total tetap dihitung server dari
// katalog; agent pemilih hanya boleh memilih id dari daftar. Tanpa agent
// kedua, paket disusun skor deterministik. Tidak ada mutasi: kampanye/pesanan
// tidak dibuat di sini.

const (
	mmPoolLimit      = 400 // kandidat maksimum yang diambil dari katalog
	mmMaxCandidates  = 40  // kandidat yang dikirim balik ke UI
	mmMaxPerCategory = 3
	mmDefaultCount   = 8
	mmMaxCount       = 30
	mmMaterialCatMax = 80
)

type mmCriteria struct {
	Objective            string   `json:"objective"`
	Audience             string   `json:"audience"`
	Categories           []string `json:"categories"`
	MediaTypes           []string `json:"media_types"`
	Count                int      `json:"count"`
	BudgetTotal          float64  `json:"budget_total"`
	BudgetMin            float64  `json:"budget_min"`
	BudgetMax            float64  `json:"budget_max"`
	MinDA                int      `json:"min_da"`
	MinAITrust           int      `json:"min_ai_trust"`
	MinTraffic           int64    `json:"min_traffic"`
	RequireHyperlink     bool     `json:"require_hyperlink"`
	RequirePermanentLink bool     `json:"require_permanent_link"`
	// Syarat artikel dari brief. 0 = tidak disebut klien.
	MinWords      int      `json:"min_words"`
	MinBacklinks  int      `json:"min_backlinks"`
	Languages     []string `json:"languages"`
	ExcludeTopics []string `json:"exclude_topics"`
	Notes         string   `json:"notes"`
}

type mmInterpretation struct {
	Summary     string            `json:"summary"`
	Provenance  map[string]string `json:"provenance"`
	Assumptions []string          `json:"assumptions"`
	Unknowns    []string          `json:"unknowns"`
}

type mmCandidate struct {
	ID               string
	Name             string
	WebsiteURL       *string
	Category         *string
	Categories       []string
	MediaType        *string
	PartnerPrice     float64
	ClientPrice      float64
	CreditCost       int
	MediaTier        *string
	EffectiveQuality float64
	HealthScore      *int
	DA               *int
	PA               *int
	AITrust          *int
	SpamScore        *int
	Traffic          int64
	LinkMonths       int
	AllowHyperlink   bool
	MaxLinks         *int
	DewanPers        bool
	DomainVerified   bool
	PublishedCount   int
	PricingType      *string
	PriceStatus      *string
	Bahasa           []string
	TopikTerlarang   []string
	LabelSponsor     *string
	WordCountMode    *string

	Score    float64
	Reasons  []string
	Warnings []string
	// Gaps: kekurangan terhadap syarat klien. Hanya terisi untuk media
	// alternatif (bukan hasil cocok) supaya UI bisa menampilkan alasannya.
	Gaps       []string
	InPackage  bool
	MatchLabel string
}

// MediaMatchmaker: interpretasi brief + seleksi media dari katalog aktif.
func (d *Deps) MediaMatchmaker(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}

	body := decodeJSON(r)
	brief, _ := strField(body, "brief")
	if len(brief) > 4000 {
		brief = brief[:4000]
	}
	// Ringkasan kebutuhan hasil agent pertama — bahan konteks untuk agent
	// pemilih. Teks bebas, tidak dipercaya sebagai data terstruktur.
	briefSummary, _ := strField(body, "summary")
	if len(briefSummary) > 600 {
		briefSummary = briefSummary[:600]
	}
	rawCriteria, hasCriteria := body["criteria"].(map[string]any)

	if brief == "" && !hasCriteria {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tulis brief kampanye atau isi kriteria media terlebih dahulu.")
		return
	}

	catalogCats, err := d.mmCatalogCategories(r.Context())
	if err != nil {
		d.writeErr(w, err)
		return
	}

	var (
		crit   mmCriteria
		interp *mmInterpretation
		// Non-nil supaya JSON selalu `[]`, bukan `null` — UI membaca .length.
		warnings = []string{}
	)
	if hasCriteria {
		// Mode edit: user sudah punya kriteria terstruktur, tidak perlu model.
		raw, _ := json.Marshal(rawCriteria)
		if err := json.Unmarshal(raw, &crit); err != nil {
			httpx.Error(w, http.StatusUnprocessableEntity, "Kriteria media tidak valid. Periksa kembali isian angka.")
			return
		}
	} else {
		var (
			status int
			msg    string
		)
		crit, interp, status, msg = d.mmInterpretBrief(r.Context(), brandRow, brief, catalogCats)
		if status != 0 {
			httpx.Error(w, status, msg)
			return
		}
	}

	crit, catWarnings := mmNormalizeCriteria(crit, catalogCats)
	warnings = append(warnings, catWarnings...)

	// Syarat klien diperlakukan apa adanya. Tidak ada penurunan ambang otomatis:
	// media yang tidak memenuhi backlink/hyperlink/permanen/DA/AI Trust/trafik
	// TIDAK boleh tampil sebagai hasil cocok. Kalau katalog tidak punya cukup
	// media, server menawarkan alternatif terdekat secara terpisah.
	candidates, poolTruncated, err := d.mmCandidates(r.Context(), crit)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	poolSize := len(candidates)
	mmScoreCandidates(candidates, crit)

	pkg, pkgWarnings := mmBuildPackage(candidates, crit)
	warnings = append(warnings, pkgWarnings...)

	// Agent kedua (pemilih media): hanya pada fase "Pilih media" eksplisit
	// (mode criteria) supaya analisis brief tetap cepat. Kalau agent tidak
	// dikonfigurasi, paket deterministik di atas tetap dipakai tanpa noise;
	// kalau agent dipanggil tapi gagal, warning diberikan ke user.
	selector := "auto"
	pkgSummary := ""
	if hasCriteria {
		picks, aiSummary, selStatus := d.mmAgentSelect(r.Context(), brandRow, crit, briefSummary, candidates)
		switch selStatus {
		case "agent":
			if applyWarn, ok := mmApplyAgentPicks(candidates, picks, crit); ok {
				selector = "agent"
				pkgSummary = aiSummary
				warnings = append(warnings, applyWarn...)
				pkg = nil // paket sekarang milik pilihan agent; dihitung ulang di bawah
			} else {
				warnings = append(warnings, "Pilihan agent pemilih tidak valid — paket memakai skor otomatis.")
			}
		case "failed":
			warnings = append(warnings, "Agent pemilih media gagal dijalankan — paket memakai skor otomatis.")
		}
	}
	pkgCount := len(pkg)
	if pkg == nil {
		for i := range candidates {
			if candidates[i].InPackage {
				pkgCount++
			}
		}
	}

	// Kandidat yang ditampilkan: paket dulu, lalu sisanya menurut skor.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].InPackage != candidates[j].InPackage {
			return candidates[i].InPackage
		}
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Name < candidates[j].Name
	})
	shown := candidates
	if len(shown) > mmMaxCandidates {
		shown = shown[:mmMaxCandidates]
	}

	total := 0.0
	byCategory := map[string]int{}
	byType := map[string]int{}
	for _, c := range candidates {
		if !c.InPackage {
			continue
		}
		total += c.ClientPrice
		byCategory[mmCategoryKey(c)]++
		mt := "mainstream"
		if c.MediaType != nil && strings.TrimSpace(*c.MediaType) != "" {
			mt = strings.TrimSpace(*c.MediaType)
		}
		byType[mt]++
	}

	unmet := []string{}
	if pkgCount < crit.Count {
		unmet = append(unmet, "Hanya "+itoa(pkgCount)+" dari "+itoa(crit.Count)+" media yang bisa dipenuhi dengan kriteria dan budget ini.")
	}
	if poolTruncated {
		warnings = append(warnings, "Kandidat dibatasi ke "+itoa(mmPoolLimit)+" media terbaik menurut kualitas & DA; persempit kategori atau budget untuk pencocokan yang lebih tepat.")
	}

	// Syarat tidak terpenuhi: tawarkan media terdekat sebagai alternatif yang
	// tetap bisa dipilih, bukan sekadar pesan "tidak ada hasil".
	alternatives := []mmCandidate{}
	if pkgCount < crit.Count {
		alts, altErr := d.mmAlternatives(r.Context(), crit, candidates)
		if altErr != nil {
			d.writeErr(w, altErr)
			return
		}
		alternatives = alts
	}

	// Komposisi media = sebaran tier pada kandidat yang lolos filter. Dihitung
	// dari data katalog, bukan dari tebakan model, dan dipakai UI sebagai
	// progress bar. Persentase dibulatkan; sisa pembulatan masuk ke item terakhir.
	composition := mmComposition(candidates)

	resp := map[string]any{
		"interpretation": interp,
		"criteria":       crit,
		"candidates":     mmCandidatePayload(shown),
		"composition":    composition,
		"selector":       selector,
		"package": map[string]any{
			"count":       pkgCount,
			"total":       round2(total),
			"budget":      round2(crit.BudgetTotal),
			"remaining":   round2(math.Max(0, crit.BudgetTotal-total)),
			"summary":     pkgSummary,
			"by_category": byCategory,
			"by_type":     byType,
			"unmet":       unmet,
		},
		"pool_size": poolSize,
		"warnings":  warnings,
		// Ringkasan saringan: UI memakai angka ini untuk pemberitahuan
		// "X dari Y media memenuhi syarat" tanpa menebak dari teks warning.
		"strict": map[string]any{
			"requested": crit.Count,
			"matched":   pkgCount,
		},
		// Media terdekat yang TIDAK memenuhi semua syarat. Boleh dipilih klien.
		"alternatives": mmCandidatePayload(alternatives),
	}
	writeJSON(w, http.StatusOK, resp)
}

// mmInterpretBrief memanggil agent berversi yang sama dengan Racik Agent.
// Status 0 berarti sukses; selain itu pesan sudah siap ditampilkan.
func (d *Deps) mmInterpretBrief(ctx context.Context, brandRow brandsdomain.BrandRow, brief string, cats []string) (mmCriteria, *mmInterpretation, int, string) {
	var zero mmCriteria

	masterPrompt := summary.LoadActivePrompt(ctx, d.Pool, summary.AgentMatchmaker)
	if strings.TrimSpace(masterPrompt) == "" {
		return zero, nil, http.StatusUnprocessableEntity, "Prompt AI Matchmaker belum diatur. Buka admin Taut Brief > Agent Matchmaker."
	}
	secret := ""
	if d.Cfg != nil {
		secret = d.Cfg.CmsEncryptionSecret
	}
	agentCfg := summary.LoadAgentConfig(ctx, d.Pool, secret, summary.AgentMatchmaker)
	if !agentCfg.Configured() {
		return zero, nil, http.StatusUnprocessableEntity, "AI Matchmaker belum dikonfigurasi. Isi base URL, API key, dan model di admin Taut Brief > Agent Matchmaker."
	}

	client := &ai.ChatClient{
		BaseURL:     agentCfg.BaseURL,
		APIKey:      agentCfg.APIKey,
		Model:       agentCfg.Model,
		Temperature: agentCfg.Temperature,
		MaxTokens:   agentCfg.MaxTokens,
		Timeout:     time.Duration(agentCfg.TimeoutMS) * time.Millisecond,
	}
	material := mmBuildMaterial(brandRow, brief, cats)
	startedAt := time.Now()
	result, err := client.CompleteMessages(ctx, []ai.ChatMessage{
		{Role: "system", Content: masterPrompt},
		{Role: "user", Content: material},
	}, true)
	if err != nil {
		// Detail penyebab (kredit habis, API key salah, rate limit, dst.) tetap
		// tercatat di taut_brief_runs untuk admin (folder Log). Pesan ke klien
		// netral — jangan pernah mengekspos instruksi admin di UI klien.
		d.logMatchmakerRun(ctx, summary.AgentMatchmaker, brandRow.ID, agentCfg.Model, material, err.Error(), 0, 0, time.Since(startedAt), err)
		return zero, nil, http.StatusBadGateway, "Layanan AI sedang tidak tersedia. Coba lagi beberapa saat lagi."
	}

	var parsed struct {
		Summary     string            `json:"summary"`
		Criteria    mmCriteria        `json:"criteria"`
		Provenance  map[string]string `json:"provenance"`
		Assumptions []string          `json:"assumptions"`
		Unknowns    []string          `json:"unknowns"`
	}
	raw := mmExtractJSON(result.Content)
	if raw == "" || json.Unmarshal([]byte(raw), &parsed) != nil {
		d.logMatchmakerRun(ctx, summary.AgentMatchmaker, brandRow.ID, agentCfg.Model, material, result.Content, result.InputTokens, result.OutputTokens, time.Since(startedAt), nil)
		return zero, nil, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi."
	}

	d.logMatchmakerRun(ctx, summary.AgentMatchmaker, brandRow.ID, agentCfg.Model, material, result.Content,
		result.InputTokens, result.OutputTokens, time.Since(startedAt), nil)

	crit, ctxNotes := mmApplyBriefContext(parsed.Criteria, brief)
	interp := &mmInterpretation{
		Summary:     mmClip(parsed.Summary, 400),
		Provenance:  mmNormalizeProvenance(parsed.Provenance),
		Assumptions: mmClipList(append(ctxNotes, parsed.Assumptions...), 8, 240),
		Unknowns:    mmClipList(parsed.Unknowns, 8, 240),
	}
	return crit, interp, 0, ""
}

// Frasa yang menandakan angka anggaran milik SATU media, bukan total kampanye.
var mmPerMediaPhrases = []string{
	"per media", "permedia", "/media", "per-media", "setiap media", "tiap media",
	"per artikel", "per placement", "per situs", "per publikasi", "harga media", "harga per",
	"masing-masing media", "masing masing media",
}

// Frasa yang menegaskan angka anggaran adalah total kampanye.
var mmTotalPhrases = []string{"total", "semua media", "seluruh", "keseluruhan", "all in", "all-in"}

// mmApplyBriefContext: failsafe deterministik di atas hasil model. Model
// kadang membaca "butuh media budget 200k" sebagai budget_max=200000 (plafon
// per media) padahal maksud klien adalah anggaran total — hasilnya paket
// sampai 8×200rb. Server membaca ulang kalimat briefnya: bila tidak ada frasa
// per-media, angka itu milik total kampanye. Sebaliknya, bila klien jelas
// menulis "per media" dan model justru mengisi budget_total tanpa count,
// angka itu dipindah ke plafon per media. Catatan dikembalikan sebagai
// assumptions supaya klien melihat cara angka itu dibaca.
func mmApplyBriefContext(crit mmCriteria, brief string) (mmCriteria, []string) {
	notes := []string{}
	text := strings.ToLower(brief)
	hasPerMedia := false
	for _, p := range mmPerMediaPhrases {
		if strings.Contains(text, p) {
			hasPerMedia = true
			break
		}
	}
	hasTotal := false
	for _, p := range mmTotalPhrases {
		if strings.Contains(text, p) {
			hasTotal = true
			break
		}
	}

	switch {
	case crit.BudgetTotal <= 0 && crit.BudgetMax > 0 && !hasPerMedia:
		// "budget 200k" tanpa "per media" → total kampanye.
		crit.BudgetTotal = crit.BudgetMax
		crit.BudgetMax = 0
		crit.BudgetMin = 0
		notes = append(notes, "Anggaran Rp"+bsIntlIDR(crit.BudgetTotal)+" dibaca sebagai total kampanye, bukan harga per media — media dipilih sampai totalnya tidak melebihi angka itu.")
	case crit.BudgetTotal > 0 && crit.BudgetMax <= 0 && crit.Count <= 0 && hasPerMedia && !hasTotal:
		// "maksimal 200k per media" tanpa total/jumlah → plafon per media.
		crit.BudgetMax = crit.BudgetTotal
		crit.BudgetTotal = 0
		notes = append(notes, "Rp"+bsIntlIDR(crit.BudgetMax)+" dibaca sebagai batas harga per media sesuai kata \"per media\" di brief.")
	}
	return crit, notes
}

// mmAgentSelect: agent kedua yang memilih paket dari kandidat katalog nyata
// berdasarkan ringkasan kebutuhan. Status: "agent" (sukses), "unconfigured"
// (agent tidak dipakai — fallback deterministik tanpa warning), "failed"
// (dipanggil tapi gagal — user diberi tahu). Kandidat, harga, dan total tetap
// ditentukan server; agent hanya memilih id dari daftar dan memberi alasan.
func (d *Deps) mmAgentSelect(ctx context.Context, brandRow brandsdomain.BrandRow, crit mmCriteria, briefSummary string, list []mmCandidate) (map[string]string, string, string) {
	masterPrompt := summary.LoadActivePrompt(ctx, d.Pool, summary.AgentMatchmakerSelect)
	if strings.TrimSpace(masterPrompt) == "" {
		return nil, "", "unconfigured"
	}
	secret := ""
	if d.Cfg != nil {
		secret = d.Cfg.CmsEncryptionSecret
	}
	agentCfg := summary.LoadAgentConfig(ctx, d.Pool, secret, summary.AgentMatchmakerSelect)
	if !agentCfg.Configured() || len(list) == 0 {
		return nil, "", "unconfigured"
	}

	// Kandidat terbaik saja yang dikirim ke model (pool bisa sampai 400).
	shown := make([]mmCandidate, len(list))
	copy(shown, list)
	sort.SliceStable(shown, func(i, j int) bool { return shown[i].Score > shown[j].Score })
	if len(shown) > mmMaxCandidates {
		shown = shown[:mmMaxCandidates]
	}

	client := &ai.ChatClient{
		BaseURL:     agentCfg.BaseURL,
		APIKey:      agentCfg.APIKey,
		Model:       agentCfg.Model,
		Temperature: agentCfg.Temperature,
		MaxTokens:   agentCfg.MaxTokens,
		Timeout:     time.Duration(agentCfg.TimeoutMS) * time.Millisecond,
	}
	material := mmSelectMaterial(crit, briefSummary, shown)
	startedAt := time.Now()
	result, err := client.CompleteMessages(ctx, []ai.ChatMessage{
		{Role: "system", Content: masterPrompt},
		{Role: "user", Content: material},
	}, true)
	if err != nil {
		d.logMatchmakerRun(ctx, summary.AgentMatchmakerSelect, brandRow.ID, agentCfg.Model, material, err.Error(), 0, 0, time.Since(startedAt), err)
		return nil, "", "failed"
	}

	var parsed struct {
		PackageSummary string `json:"package_summary"`
		Picks          []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"picks"`
	}
	raw := mmExtractJSON(result.Content)
	if raw == "" || json.Unmarshal([]byte(raw), &parsed) != nil {
		d.logMatchmakerRun(ctx, summary.AgentMatchmakerSelect, brandRow.ID, agentCfg.Model, material, result.Content, result.InputTokens, result.OutputTokens, time.Since(startedAt), nil)
		return nil, "", "failed"
	}
	d.logMatchmakerRun(ctx, summary.AgentMatchmakerSelect, brandRow.ID, agentCfg.Model, material, result.Content,
		result.InputTokens, result.OutputTokens, time.Since(startedAt), nil)

	valid := make(map[string]bool, len(shown))
	for _, c := range shown {
		valid[c.ID] = true
	}
	picks := map[string]string{}
	for _, p := range parsed.Picks {
		id := strings.TrimSpace(p.ID)
		if id == "" || !valid[id] {
			continue // id di luar kandidat = buang, jangan pernah dipercaya
		}
		if _, dup := picks[id]; !dup {
			picks[id] = mmClip(p.Reason, 240)
		}
	}
	if len(picks) == 0 {
		return nil, "", "failed"
	}
	return picks, mmClip(parsed.PackageSummary, 300), "agent"
}

// mmApplyAgentPicks menandai pilihan agent pada kandidat, mengganti alasan
// dengan alasan agent, lalu menegakkan batas count dan budget total. Return
// ok=false bila hasil akhir tidak masuk akal (panggilan boleh mempertahankan
// paket deterministik yang sudah ada).
func mmApplyAgentPicks(list []mmCandidate, picks map[string]string, crit mmCriteria) ([]string, bool) {
	if len(picks) == 0 {
		return nil, false
	}
	byID := make(map[string]int, len(list))
	for i := range list {
		byID[list[i].ID] = i
		list[i].InPackage = false
	}

	// Clamp jumlah pilihan ke count, prioritas skor tertinggi bila lebih.
	idx := make([]int, 0, len(picks))
	for id := range picks {
		if i, ok := byID[id]; ok {
			idx = append(idx, i)
		}
	}
	if len(idx) > crit.Count {
		sort.SliceStable(idx, func(a, b int) bool { return list[idx[a]].Score > list[idx[b]].Score })
		idx = idx[:crit.Count]
	}

	total := 0.0
	picked := make([]int, 0, len(idx))
	for _, i := range idx {
		if crit.BudgetTotal > 0 && total+list[i].ClientPrice > crit.BudgetTotal {
			continue
		}
		picked = append(picked, i)
		total += list[i].ClientPrice
	}
	// Budget sempit tapi masih ada ruang: isi sisa slot dengan skor tertinggi
	// yang terjangkau supaya paket tidak setengah kosong gara-gara urutan AI.
	if len(picked) < crit.Count {
		sort.SliceStable(idx, func(a, b int) bool { return list[idx[a]].Score > list[idx[b]].Score })
		for _, i := range idx {
			if len(picked) >= crit.Count {
				break
			}
			inPicked := false
			for _, p := range picked {
				if p == i {
					inPicked = true
					break
				}
			}
			if inPicked {
				continue
			}
			if crit.BudgetTotal > 0 && total+list[i].ClientPrice > crit.BudgetTotal {
				continue
			}
			picked = append(picked, i)
			total += list[i].ClientPrice
		}
	}
	if len(picked) == 0 {
		return nil, false
	}

	warnings := []string{}
	if crit.BudgetTotal > 0 && total > crit.BudgetTotal { // tidak mungkin, tapi jaga
		warnings = append(warnings, "Total pilihan AI melebihi budget total.")
	}
	if len(picks) > len(picked) {
		warnings = append(warnings, "Sebagian pilihan agent dilepas agar jumlah media dan budget tetap sesuai kriteria.")
	}
	for _, i := range picked {
		list[i].InPackage = true
		if r := strings.TrimSpace(picks[list[i].ID]); r != "" {
			list[i].Reasons = []string{r}
		}
	}
	return warnings, true
}

func mmSelectMaterial(crit mmCriteria, briefSummary string, list []mmCandidate) string {
	var b strings.Builder
	b.WriteString("Ringkasan kebutuhan (dari agent peringkas):\n")
	if s := strings.TrimSpace(briefSummary); s != "" {
		b.WriteString(s)
	} else {
		b.WriteString("(tidak tersedia — gunakan kriteria di bawah)")
	}
	b.WriteString("\n\nKriteria: jumlah media=")
	b.WriteString(itoa(crit.Count))
	if crit.BudgetTotal > 0 {
		b.WriteString(", budget_total=Rp")
		b.WriteString(bsIntlIDR(crit.BudgetTotal))
	}
	if len(crit.Categories) > 0 {
		b.WriteString(", kategori=")
		b.WriteString(strings.Join(crit.Categories, "/"))
	}
	if len(crit.MediaTypes) > 0 {
		b.WriteString(", jenis=")
		b.WriteString(strings.Join(crit.MediaTypes, "/"))
	}
	if crit.MinWords > 0 {
		b.WriteString(", minimal ")
		b.WriteString(itoa(crit.MinWords))
		b.WriteString(" kata")
	}
	if crit.MinBacklinks > 0 {
		b.WriteString(", minimal ")
		b.WriteString(itoa(crit.MinBacklinks))
		b.WriteString(" backlink")
	}
	if crit.RequireHyperlink {
		b.WriteString(", wajib hyperlink")
	}
	if crit.RequirePermanentLink {
		b.WriteString(", wajib tautan permanen")
	}
	if len(crit.ExcludeTopics) > 0 {
		b.WriteString(", hindari topik=")
		b.WriteString(strings.Join(crit.ExcludeTopics, "/"))
	}
	if crit.Objective != "" {
		b.WriteString(", tujuan=")
		b.WriteString(crit.Objective)
	}
	b.WriteString("\n\nDaftar kandidat (format: id | nama | kategori | tier | harga klien | DA | AI Trust | traffic/bln | durasi link | hyperlink | maks link | Dewan Pers | artikel terbit | rentang kata):\n")
	for _, c := range list {
		b.WriteString(c.ID)
		b.WriteString(" | ")
		b.WriteString(c.Name)
		b.WriteString(" | ")
		b.WriteString(mmCategoryKey(c))
		b.WriteString(" | ")
		b.WriteString(derefStr(c.MediaTier))
		b.WriteString(" | Rp")
		b.WriteString(bsIntlIDR(c.ClientPrice))
		b.WriteString(" | DA ")
		b.WriteString(itoa(mmIntOr(c.DA, 0)))
		b.WriteString(" | AI ")
		b.WriteString(itoa(mmIntOr(c.AITrust, 0)))
		b.WriteString(" | ")
		b.WriteString(itoa64(c.Traffic))
		b.WriteString(" | link ")
		if c.LinkMonths == 0 {
			b.WriteString("permanen")
		} else {
			b.WriteString(itoa(c.LinkMonths) + " bln")
		}
		b.WriteString(" | hyperlink ")
		if c.AllowHyperlink {
			b.WriteString("ya")
		} else {
			b.WriteString("tidak")
		}
		b.WriteString(" | maks link ")
		b.WriteString(itoa(mmIntOr(c.MaxLinks, 0)))
		b.WriteString(" | Dewan Pers ")
		if c.DewanPers {
			b.WriteString("ya")
		} else {
			b.WriteString("tidak")
		}
		b.WriteString(" | ")
		b.WriteString(itoa(c.PublishedCount))
		b.WriteString(" artikel")
		if minW, maxW, flexible := mmWordRange(c.WordCountMode); !flexible && maxW > 0 {
			b.WriteString(" | kata ")
			b.WriteString(itoa(minW) + "-" + itoa(maxW))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func mmBuildMaterial(brandRow brandsdomain.BrandRow, brief string, cats []string) string {
	var b strings.Builder
	b.WriteString("Brief klien:\n")
	b.WriteString(brief)
	b.WriteString("\n\nBrand: ")
	b.WriteString(brandRow.Name)
	if brandRow.Industry != nil && strings.TrimSpace(*brandRow.Industry) != "" {
		b.WriteString(" (industri: ")
		b.WriteString(strings.TrimSpace(*brandRow.Industry))
		b.WriteString(")")
	}
	if brandRow.PrimaryLanguage != "" {
		b.WriteString("\nBahasa utama brand: ")
		b.WriteString(brandRow.PrimaryLanguage)
	}
	shown := cats
	if len(shown) > mmMaterialCatMax {
		shown = shown[:mmMaterialCatMax]
	}
	b.WriteString("\n\nKategori yang tersedia di katalog: ")
	if len(shown) == 0 {
		b.WriteString("(belum ada kategori)")
	} else {
		b.WriteString(strings.Join(shown, ", "))
	}
	b.WriteString("\nJenis media yang tersedia: mainstream, blogger")
	return b.String()
}

func mmExtractJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = s[nl+1:]
		}
		if end := strings.LastIndex(s, "```"); end >= 0 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func (d *Deps) logMatchmakerRun(ctx context.Context, agent, brandID, model, input, output string, tokIn, tokOut int, dur time.Duration, runErr error) {
	if strings.TrimSpace(brandID) == "" {
		return
	}
	status := "done"
	errText := ""
	if runErr != nil {
		status = "failed"
		errText = runErr.Error()
	}
	// Context request yang sudah batal (klien menutup/timeout) tetap harus bisa
	// mencatat run-nya — pakai background ctx berbatas waktu, bukan ctx request.
	logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = d.exec(logCtx, `
		INSERT INTO taut_brief_runs
		  (id, brand_id, profile_id, agent, status, model, token_input, token_output,
		   duration_ms, error, input_preview, output_preview, created_at, finished_at)
		VALUES (gen_random_uuid()::text, $1, NULL, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, now(), now())`,
		brandID, agent, status, model,
		nullInt(tokIn), nullInt(tokOut), int(dur.Milliseconds()), errText,
		truncateString(input, 2000), truncateString(output, 2000))
}

// ---------- katalog ----------

func (d *Deps) mmCatalogCategories(ctx context.Context) ([]string, error) {
	rows, err := d.Pool.Query(ctx, "SELECT name FROM media_categories ORDER BY name ASC LIMIT 500")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil && strings.TrimSpace(name) != "" {
			out = append(out, name)
		}
	}
	return out, rows.Err()
}

func (d *Deps) mmCandidates(ctx context.Context, crit mmCriteria) ([]mmCandidate, bool, error) {
	return d.mmQueryCandidates(ctx, crit, true)
}

// mmLooseCandidates: pool yang HANYA disaring kategori & jenis media. Dipakai
// untuk menyusun media alternatif — kekurangan syarat angka dihitung di Go
// supaya bisa dijelaskan satu per satu ke klien, bukan sekadar dibuang.
func (d *Deps) mmLooseCandidates(ctx context.Context, crit mmCriteria) ([]mmCandidate, bool, error) {
	return d.mmQueryCandidates(ctx, crit, false)
}

// mmQueryCandidates: `hard=true` menerapkan SEMUA syarat klien apa adanya
// (tidak ada pelonggaran otomatis); `hard=false` hanya memakai lingkup
// kategori & jenis media.
func (d *Deps) mmQueryCandidates(ctx context.Context, crit mmCriteria, hard bool) ([]mmCandidate, bool, error) {
	args := []any{}
	add := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	// Aturan yang sama dengan katalog klien: hanya media aktif.
	where := "WHERE media.status = 'active'"

	if len(crit.Categories) > 0 {
		ors := make([]string, 0, len(crit.Categories))
		for _, c := range crit.Categories {
			norm := strings.ToLower(strings.TrimSpace(c))
			p1 := add(norm)
			p2 := add(norm)
			ors = append(ors, "(EXISTS (SELECT 1 FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = media.id AND c.normalized = "+p1+") OR LOWER(TRIM(media.category)) = "+p2+")")
		}
		where += " AND (" + strings.Join(ors, " OR ") + ")"
	}
	if len(crit.MediaTypes) > 0 {
		where += " AND LOWER(COALESCE(media.media_type, '')) = ANY(" + add(crit.MediaTypes) + ")"
	}
	if hard {
		if crit.MinDA > 0 {
			where += " AND sm.da >= " + add(crit.MinDA)
		}
		if crit.MinAITrust > 0 {
			where += " AND sm.ai_trust_score >= " + add(crit.MinAITrust)
		}
		if crit.MinTraffic > 0 {
			// Angka yang diminta klien harus benar-benar terpenuhi. Media tanpa
			// data trafik TIDAK lagi lolos sebagai "belum terverifikasi": itu
			// pernah membuat hasil tampak memenuhi syarat padahal tidak ada
			// angkanya sama sekali.
			where += " AND GREATEST(COALESCE(media.traffic_estimate, 0), COALESCE(sw.visits, 0)) >= " + add(crit.MinTraffic)
		}
		if crit.RequireHyperlink {
			// max_links ikut dicek: allow_hyperlink saja tidak cukup — penjagaan
			// terbit memperlakukan max_links = 0 sebagai "tidak boleh ada tautan".
			where += " AND media.allow_hyperlink = TRUE AND COALESCE(media.max_links, 0) > 0"
		}
		if crit.MinBacklinks > 0 {
			where += " AND media.allow_hyperlink = TRUE AND COALESCE(media.max_links, 0) >= " + add(crit.MinBacklinks)
		}
		if crit.RequirePermanentLink {
			where += " AND media.link_duration_months = 0"
		}
	}

	// Urutan menentukan media mana yang masuk pool terbatas. Tanpa plafon harga
	// kualitas lebih penting; dengan plafon harga, urutkan termurah dulu supaya
	// media terjangkau tidak terbuang oleh 400 media mahal ber-kualitas tinggi.
	order := `ORDER BY COALESCE(media.media_quality_score,
		                  CASE media.media_tier WHEN 'tier1' THEN 90 WHEN 'tier2' THEN 80 WHEN 'tier3' THEN 70 ELSE 60 END) DESC,
		         sm.da DESC NULLS LAST, media.name`
	if hard && crit.BudgetMax > 0 {
		order = "ORDER BY media.price ASC, sm.da DESC NULLS LAST, media.name"
	}

	q := `
		SELECT media.id, media.name, media.website_url, media.category, media.price, media.credit_cost,
		       media.media_tier, media.media_quality_score, media.health_score, media.traffic_estimate,
		       media.media_type, media.allow_hyperlink, media.max_links, media.link_duration_months,
		       media.dewan_pers_verified, media.domain_verified, media.pricing_type, media.price_status,
		       media.bahasa_artikel, media.topik_terlarang, media.label_sponsor, media.word_count_mode,
		       sm.da, sm.pa, sm.ai_trust_score, sm.spam_score,
		       COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l
		                 JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = media.id), media.category) AS categories_csv,
		       sw.visits AS sw_visits,
		       (SELECT COUNT(*)::int FROM publications p WHERE p.media_id = media.id AND p.status = 'published') AS published_count
		FROM media
		LEFT JOIN media_seo_metrics sm ON sm.media_id = media.id
		LEFT JOIN LATERAL (SELECT CASE WHEN s.insights->>'visits' ~ '^-?[0-9]+([.][0-9]+)?$'
		                          THEN ((s.insights->>'visits')::numeric)::bigint END AS visits
		                  FROM media_similarweb_snapshots s WHERE s.media_id = media.id
		                  ORDER BY s.created_at DESC LIMIT 1) sw ON TRUE
		` + where + `
		` + order + `
		LIMIT ` + itoa(mmPoolLimit)

	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	markup := d.getMarkupPercent(ctx)
	tierDefaults := d.mmTierDefaults(ctx)
	out := []mmCandidate{}
	scanned := 0
	for rows.Next() {
		var c mmCandidate
		var quality *int
		var bahasa, topik, catsCSV *string
		var swVisits *int64
		if err := rows.Scan(&c.ID, &c.Name, &c.WebsiteURL, &c.Category, &c.PartnerPrice, &c.CreditCost,
			&c.MediaTier, &quality, &c.HealthScore, &c.Traffic,
			&c.MediaType, &c.AllowHyperlink, &c.MaxLinks, &c.LinkMonths,
			&c.DewanPers, &c.DomainVerified, &c.PricingType, &c.PriceStatus,
			&bahasa, &topik, &c.LabelSponsor, &c.WordCountMode,
			&c.DA, &c.PA, &c.AITrust, &c.SpamScore,
			&catsCSV, &swVisits, &c.PublishedCount); err != nil {
			continue
		}
		scanned++
		c.ClientPrice = round2(math.Max(0, c.PartnerPrice) * (1 + math.Max(0, markup)/100))
		c.EffectiveQuality = 0
		if quality != nil {
			c.EffectiveQuality = float64(*quality)
		} else if tq, ok := tierDefaults[strings.ToLower(derefStr(c.MediaTier))]; ok {
			c.EffectiveQuality = tq
		}
		if c.Traffic < 0 {
			c.Traffic = 0
		}
		if swVisits != nil && *swVisits > c.Traffic {
			c.Traffic = *swVisits
		}
		c.Bahasa = mmParseStringList(bahasa)
		c.TopikTerlarang = mmParseStringList(topik)
		if catsCSV != nil {
			for _, part := range strings.Split(*catsCSV, "\u0001") {
				if p := strings.TrimSpace(part); p != "" {
					c.Categories = append(c.Categories, p)
				}
			}
		}
		// Filter harga dilakukan di Go karena harga klien = harga mitra + markup,
		// yang tidak tersimpan sebagai kolom. Untuk pool alternatif filter ini
		// sengaja dilewati: media di atas budget justru kandidat yang berguna
		// ditawarkan dengan catatan "lewat budget".
		if hard {
			if crit.BudgetMax > 0 && c.ClientPrice > crit.BudgetMax {
				continue
			}
			if crit.BudgetMin > 0 && c.ClientPrice < crit.BudgetMin {
				continue
			}
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	// Pool dipotong di SQL, jadi hasil bukan seluruh katalog. Ini harus
	// terlihat oleh klien, bukan diam-diam menyempitkan pilihan.
	return out, scanned >= mmPoolLimit, nil
}

func (d *Deps) mmTierDefaults(ctx context.Context) map[string]float64 {
	defaults := map[string]float64{"tier1": 90, "tier2": 80, "tier3": 70}
	var raw []byte
	if err := d.Pool.QueryRow(ctx, "SELECT tier_default_quality FROM pr_media_rules WHERE id = 'default' LIMIT 1").Scan(&raw); err == nil && len(raw) > 0 {
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			for k, v := range m {
				if f, ok := v.(float64); ok {
					defaults[strings.ToLower(k)] = f
				}
			}
		}
	}
	return defaults
}

// ---------- normalisasi kriteria ----------

func mmNormalizeCriteria(crit mmCriteria, catalogCats []string) (mmCriteria, []string) {
	warnings := []string{}

	crit.Objective = mmClip(crit.Objective, 300)
	crit.Audience = mmClip(crit.Audience, 300)
	crit.Notes = mmClip(crit.Notes, 500)

	if crit.Count <= 0 {
		crit.Count = mmDefaultCount
	}
	if crit.Count > mmMaxCount {
		crit.Count = mmMaxCount
	}
	crit.BudgetTotal = mmClampMoney(crit.BudgetTotal)
	crit.BudgetMin = mmClampMoney(crit.BudgetMin)
	crit.BudgetMax = mmClampMoney(crit.BudgetMax)
	if crit.BudgetMin > 0 && crit.BudgetMax > 0 && crit.BudgetMin > crit.BudgetMax {
		crit.BudgetMin, crit.BudgetMax = crit.BudgetMax, crit.BudgetMin
	}
	// Plafon yang tertulis dua kali (min == max) hampir selalu salah tangkap:
	// klien menulis "maksimal 500rb per media", model mengisi keduanya. Itu
	// bukan rentang sempit yang disengaja — filter min akan membuang semua
	// media di bawah 500rb, padahal klien hanya memasang batas atas.
	if crit.BudgetMin > 0 && crit.BudgetMin == crit.BudgetMax {
		crit.BudgetMin = 0
	}
	// Failsafe salah tangkap kedua: rentang harga per media yang sebenarnya
	// anggaran TOTAL (kasus nyata: "Anggaran ideal Rp6-7 juta" terbaca
	// budget_min=6jt & budget_max=7jt PER MEDIA padahal budget_total=7jt).
	// Rentang per media ≥ anggaran total tidak pernah mungkin diminta klien —
	// bila dimaksud serius, 20 media berarti 120jt, jauh dari total di atas.
	if crit.BudgetMin > 0 && crit.BudgetTotal > 0 && crit.BudgetMin >= crit.BudgetTotal {
		crit.BudgetMin = 0
		warnings = append(warnings, "Kriteria harga minimum per media tampak berasal dari anggaran total dan diabaikan.")
	}
	crit.MinDA = clampIntValue(crit.MinDA, 0, 100)
	crit.MinAITrust = clampIntValue(crit.MinAITrust, 0, 100)
	crit.MinWords = clampIntValue(crit.MinWords, 0, 10000)
	crit.MinBacklinks = clampIntValue(crit.MinBacklinks, 0, 20)
	if crit.MinTraffic < 0 {
		crit.MinTraffic = 0
	}

	// Kategori: hanya nama yang benar-benar ada di katalog. AI tidak boleh
	// mengarang kategori, dan nama yang dikembalikan diseragamkan ke ejaan
	// katalog supaya filter SQL-nya cocok.
	known := map[string]string{}
	for _, c := range catalogCats {
		known[strings.ToLower(strings.TrimSpace(c))] = c
	}
	cats := []string{}
	seenCat := map[string]bool{}
	for _, c := range crit.Categories {
		key := strings.ToLower(strings.TrimSpace(c))
		if key == "" || seenCat[key] {
			continue
		}
		seenCat[key] = true
		if canonical, ok := known[key]; ok {
			cats = append(cats, canonical)
		} else {
			warnings = append(warnings, "Kategori \""+mmClip(c, 60)+"\" tidak ada di katalog dan diabaikan.")
		}
	}
	crit.Categories = cats

	types := []string{}
	seenType := map[string]bool{}
	for _, t := range crit.MediaTypes {
		v := strings.ToLower(strings.TrimSpace(t))
		if (v == "mainstream" || v == "blogger") && !seenType[v] {
			seenType[v] = true
			types = append(types, v)
		}
	}
	crit.MediaTypes = types
	crit.Languages = mmClipList(crit.Languages, 6, 40)
	crit.ExcludeTopics = mmClipList(crit.ExcludeTopics, 10, 60)
	return crit, warnings
}

func mmClampMoney(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	if v > 1e13 {
		return 1e13
	}
	return math.Round(v)
}

// ---------- media alternatif ----------

// mmMaxAlternatives: jumlah saran yang ditampilkan di bawah pemberitahuan.
const mmMaxAlternatives = 12

// mmAlternatives: media terdekat yang TIDAK memenuhi syarat ANGKA (DA, trafik,
// backlink, dan sejenisnya). Dipakai UI sebagai daftar "rekomendasi lain"
// bergaya marketplace — media di atas/bawah ANGARAN tidak masuk sini: anggaran
// bukan syarat yang boleh ditawar, itu plafon uang klien. Hanya kandidat
// dengan <= 2 jenis kekurangan yang diambil supaya saran tetap relevan, bukan
// tumpukan media acak.
func (d *Deps) mmAlternatives(ctx context.Context, crit mmCriteria, matched []mmCandidate) ([]mmCandidate, error) {
	loose, _, err := d.mmLooseCandidates(ctx, crit)
	if err != nil {
		return nil, err
	}
	// Media yang sudah cocok tidak perlu diulang sebagai "alternatif".
	seen := make(map[string]bool, len(matched))
	for _, c := range matched {
		seen[c.ID] = true
	}
	mmScoreCandidates(loose, crit)
	out := make([]mmCandidate, 0, mmMaxAlternatives)
	for i := range loose {
		c := loose[i]
		if seen[c.ID] {
			continue
		}
		// Plafon harga per media tidak boleh dilewati walau kualitasnya bagus —
		// media Rp150rb tidak pernah jadi "alternatif" untuk brief "maksimal
		// 100rb". Buang kandidatnya di sini, bukan hanya tanda kekurangan.
		if crit.BudgetMax > 0 && c.ClientPrice > crit.BudgetMax {
			continue
		}
		if crit.BudgetMin > 0 && c.ClientPrice < crit.BudgetMin {
			continue
		}
		gaps := mmCandidateGaps(crit, c)
		if len(gaps) == 0 || len(gaps) > 2 {
			continue
		}
		c.Gaps = gaps
		// Kekurangan yang sama tercermin di badge UI; warning mentah hanya
		// mengulang informasi yang sama dan membuat kartu terlalu ramai.
		c.Warnings = nil
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Gaps) != len(out[j].Gaps) {
			return len(out[i].Gaps) < len(out[j].Gaps)
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > mmMaxAlternatives {
		out = out[:mmMaxAlternatives]
	}
	// Langkah pelebaran ala marketplace ("Hasil dari luar pencarian Anda"):
	// kalau lingkup kategori/jenis media hanya menyediakan sedikit saran
	// (atau kosong — mis. tipe "blogger" + kategori tidak punya irisan di
	// katalog), cari di SELURUH katalog aktif tanpa lingkup itu. Kategorinya
	// tetap disebut sebagai gap, jadi klien tahu media ini di luar pilihannya
	// tapi masih berdekatan secara angka & harga.
	if len(out) < mmMaxAlternatives {
		wide := crit
		wide.Categories = nil
		wide.MediaTypes = nil
		pool, _, wideErr := d.mmQueryCandidates(ctx, wide, false)
		if wideErr == nil && len(pool) > 0 {
			mmScoreCandidates(pool, crit)
			for i := range pool {
				c := pool[i]
				if seen[c.ID] {
					continue
				}
				if crit.BudgetMax > 0 && c.ClientPrice > crit.BudgetMax {
					continue
				}
				if crit.BudgetMin > 0 && c.ClientPrice < crit.BudgetMin {
					continue
				}
				// Plafon gap 3 untuk hasil pelebaran: gap kategori/jenis ikut
				// terhitung di sini (pool sempit tidak pernah membawanya),
				// plus syarat angka — tetap disebut supaya kartunya jujur.
				gaps := mmCandidateGaps(crit, c)
				if len(gaps) == 0 || len(gaps) > 3 {
					continue
				}
				c.Gaps = gaps
				c.Warnings = nil
				seen[c.ID] = true
				out = append(out, c)
				if len(out) >= mmMaxAlternatives {
					break
				}
			}
			sort.SliceStable(out, func(i, j int) bool {
				if len(out[i].Gaps) != len(out[j].Gaps) {
					return len(out[i].Gaps) < len(out[j].Gaps)
				}
				if out[i].Score != out[j].Score {
					return out[i].Score > out[j].Score
				}
				return out[i].Name < out[j].Name
			})
			if len(out) > mmMaxAlternatives {
				out = out[:mmMaxAlternatives]
			}
		}
		// Kegagalan kueri pelebaran dibiarkan: saran dari pool sempit tetap
		// dikembalikan; bila keduanya kosong dengan error, pool utama sudah
		// memutuskan responsnya.
		_ = wideErr
	}
	return out, nil
}

// mmCandidateGaps: daftar kekurangan kandidat terhadap syarat klien. Kalimatnya
// siap tampil di kartu alternatif — angka asli disertakan supaya klien bisa
// menilai sendiri apakah masih layak.
func mmCandidateGaps(crit mmCriteria, c mmCandidate) []string {
	gaps := []string{}
	// Gap kategori & jenis media: pool terlingkup selalu cocok pada dua hal
	// ini sehingga tidak pernah menambah gap; gap ini aktif hanya untuk hasil
	// pelebaran (seluruh katalog) supaya kartunya jujur soal bedanya.
	if len(crit.Categories) > 0 {
		match := false
		for _, cat := range c.Categories {
			for _, want := range crit.Categories {
				if strings.EqualFold(strings.TrimSpace(cat), want) {
					match = true
					break
				}
			}
			if match {
				break
			}
		}
		if !match {
			gaps = append(gaps, "Kategori \""+derefStr(c.Category)+"\" — di luar pilihan ("+strings.Join(crit.Categories, ", ")+").")
		}
	}
	if len(crit.MediaTypes) > 0 {
		mt := strings.ToLower(derefStr(c.MediaType))
		match := false
		for _, want := range crit.MediaTypes {
			if mt == want {
				match = true
				break
			}
		}
		if !match {
			gaps = append(gaps, "Jenis media \""+derefStr(c.MediaType)+"\" — bukan pilihan ("+strings.Join(crit.MediaTypes, ", ")+").")
		}
	}
	if crit.MinDA > 0 {
		if c.DA == nil {
			gaps = append(gaps, "DA belum terdata (butuh ≥ "+itoa(crit.MinDA)+").")
		} else if *c.DA < crit.MinDA {
			gaps = append(gaps, "DA "+itoa(*c.DA)+" — butuh ≥ "+itoa(crit.MinDA)+".")
		}
	}
	if crit.MinAITrust > 0 {
		if c.AITrust == nil {
			gaps = append(gaps, "AI Trust belum terdata (butuh ≥ "+itoa(crit.MinAITrust)+").")
		} else if *c.AITrust < crit.MinAITrust {
			gaps = append(gaps, "AI Trust "+itoa(*c.AITrust)+" — butuh ≥ "+itoa(crit.MinAITrust)+".")
		}
	}
	if crit.MinTraffic > 0 && c.Traffic < crit.MinTraffic {
		if c.Traffic <= 0 {
			gaps = append(gaps, "Trafik belum terdata (butuh ≥ "+bsIntlCompact(float64(crit.MinTraffic))+" /bulan).")
		} else {
			gaps = append(gaps, "Trafik "+bsIntlCompact(float64(c.Traffic))+" /bulan — butuh ≥ "+bsIntlCompact(float64(crit.MinTraffic))+".")
		}
	}
	if crit.MinBacklinks > 0 {
		capacity := mmIntOr(c.MaxLinks, 0)
		if !c.AllowHyperlink || capacity < crit.MinBacklinks {
			gaps = append(gaps, "Maksimal "+itoa(capacity)+" tautan per artikel — butuh "+itoa(crit.MinBacklinks)+".")
		}
	}
	if crit.RequireHyperlink && (!c.AllowHyperlink || mmIntOr(c.MaxLinks, 0) <= 0) {
		gaps = append(gaps, "Tidak menerima tautan aktif.")
	}
	if crit.RequirePermanentLink && c.LinkMonths > 0 {
		gaps = append(gaps, "Tautan hanya "+itoa(c.LinkMonths)+" bulan — butuh permanen.")
	}
	if crit.BudgetMax > 0 && c.ClientPrice > crit.BudgetMax {
		gaps = append(gaps, bsIntlIDR(c.ClientPrice)+" — lewat "+bsIntlIDR(c.ClientPrice-crit.BudgetMax)+" dari batas per media.")
	}
	if crit.BudgetMin > 0 && c.ClientPrice < crit.BudgetMin {
		gaps = append(gaps, bsIntlIDR(c.ClientPrice)+" — di bawah batas minimum per media.")
	}
	return gaps
}

// ---------- skor & paket ----------

func mmScoreCandidates(list []mmCandidate, crit mmCriteria) {
	// Plafon harga per media dipakai sebagai acuan "cocok budget".
	capPerMedia := 0.0
	if crit.BudgetTotal > 0 && crit.Count > 0 {
		capPerMedia = crit.BudgetTotal / float64(crit.Count)
	}
	maxTraffic := int64(0)
	for _, c := range list {
		if c.Traffic > maxTraffic {
			maxTraffic = c.Traffic
		}
	}
	for i := range list {
		c := &list[i]

		quality := 0.5*c.EffectiveQuality + 0.3*float64(mmIntOr(c.DA, 0)) + 0.2*float64(mmIntOr(c.AITrust, 0))
		score := quality / 100 * 35

		perf := 0.0
		if c.HealthScore != nil {
			perf = float64(*c.HealthScore)
		}
		perf = perf/100*12 + math.Min(float64(c.PublishedCount), 50)/50*8
		score += perf

		if maxTraffic > 0 && c.Traffic > 0 {
			score += math.Min(math.Log10(float64(c.Traffic))/7, 1) * 15
		}

		switch {
		case capPerMedia <= 0:
			score += 10 // budget tidak disebut: netral, bukan bonus
		case c.ClientPrice <= capPerMedia:
			score += 15
		default:
			ratio := capPerMedia / c.ClientPrice
			score += 15 * math.Max(0, ratio)
		}

		editorial := 0.0
		if c.AllowHyperlink {
			editorial += 4
		}
		if c.LinkMonths == 0 {
			editorial += 3
		}
		if mmIntOr(c.MaxLinks, 0) >= 1 {
			editorial += 3
		}
		if c.DewanPers {
			editorial += 3
		}
		if c.DomainVerified {
			editorial += 2
		}
		score += editorial

		c.Score = round2(math.Min(100, score))
		c.MatchLabel = mmMatchLabel(c.Score)
		c.Reasons = mmReasons(*c, capPerMedia)
		c.Warnings = mmCandidateWarnings(*c, crit)
	}
}

func mmMatchLabel(score float64) string {
	switch {
	case score >= 80:
		return "Sangat cocok"
	case score >= 65:
		return "Cocok"
	case score >= 50:
		return "Cukup cocok"
	}
	return "Kurang cocok"
}

func mmReasons(c mmCandidate, capPerMedia float64) []string {
	out := []string{}
	if c.DA != nil && *c.DA >= 50 {
		out = append(out, "DA "+itoa(*c.DA)+" — otoritas domain tinggi.")
	} else if c.DA != nil {
		out = append(out, "DA "+itoa(*c.DA)+".")
	}
	if c.AITrust != nil && *c.AITrust >= 60 {
		out = append(out, "AI Trust "+itoa(*c.AITrust)+" — baik untuk visibilitas di mesin AI.")
	}
	if c.Traffic >= 100000 {
		out = append(out, "Estimasi traffic "+bsIntlCompact(float64(c.Traffic))+" kunjungan.")
	}
	if c.HealthScore != nil && *c.HealthScore >= 80 {
		out = append(out, "Kesehatan media "+itoa(*c.HealthScore)+"/100 — jarang menolak atau terlambat.")
	}
	if c.PublishedCount >= 10 {
		out = append(out, itoa(c.PublishedCount)+" artikel sudah terbit lewat media ini.")
	}
	if capPerMedia > 0 && c.ClientPrice <= capPerMedia {
		out = append(out, "Harga "+bsIntlIDR(c.ClientPrice)+" masih di bawah plafon per media.")
	}
	if c.LinkMonths == 0 {
		out = append(out, "Tautan permanen.")
	}
	if c.DewanPers {
		out = append(out, "Terverifikasi Dewan Pers.")
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func mmCandidateWarnings(c mmCandidate, crit mmCriteria) []string {
	out := []string{}
	// Brief sering meminta ambang trafik; media tanpa data tidak ditolak,
	// tapi statusnya diberitahu agar tidak dianggap "terverifikasi".
	if crit.MinTraffic > 0 && c.Traffic <= 0 {
		out = append(out, "Traffic belum terverifikasi — katalog belum punya angkanya.")
	}
	if c.PriceStatus != nil {
		switch strings.ToLower(strings.TrimSpace(*c.PriceStatus)) {
		case "menunggu_review":
			out = append(out, "Harga masih menunggu review admin.")
		case "negosiasi":
			out = append(out, "Harga masih dalam negosiasi.")
		}
	}
	if c.PricingType != nil && strings.EqualFold(strings.TrimSpace(*c.PricingType), "nego") {
		out = append(out, "Harga nego — total bisa berubah.")
	}
	if crit.RequireHyperlink && !c.AllowHyperlink {
		out = append(out, "Tidak menerima tautan aktif.")
	}
	if crit.RequirePermanentLink && c.LinkMonths > 0 {
		out = append(out, "Tautan hanya "+itoa(c.LinkMonths)+" bulan.")
	}
	if crit.MinWords > 0 {
		if minWords, maxWords, flexible := mmWordRange(c.WordCountMode); !flexible && maxWords > 0 && maxWords < crit.MinWords {
			out = append(out, "Panjang artikel di media ini maksimal "+itoa(maxWords)+" kata — kurang dari syarat brief minimal "+itoa(crit.MinWords)+" kata.")
		} else if !flexible && minWords > 0 && minWords < crit.MinWords {
			out = append(out, "Panjang artikel di media ini "+itoa(minWords)+"-"+itoa(maxWords)+" kata — kurang dari syarat brief minimal "+itoa(crit.MinWords)+" kata.")
		}
	}
	if crit.MinBacklinks > 0 && mmIntOr(c.MaxLinks, 0) < crit.MinBacklinks {
		out = append(out, "Media ini hanya mengizinkan maksimal "+itoa(mmIntOr(c.MaxLinks, 0))+" tautan per artikel — kurang dari syarat brief minimal "+itoa(crit.MinBacklinks)+" backlink. Sisanya harus ditaruh di artikel lain.")
	}
	if len(crit.ExcludeTopics) > 0 {
		for _, t := range c.TopikTerlarang {
			for _, ex := range crit.ExcludeTopics {
				if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(ex)) {
					out = append(out, "Topik terlarang media ini bersinggungan dengan \""+mmClip(ex, 40)+"\".")
				}
			}
		}
	}
	return out
}

// mmComposition: sebaran tier media pada kandidat. Dipakai progress bar
// "Komposisi media" — angkanya berasal dari katalog nyata.
func mmComposition(list []mmCandidate) map[string]any {
	type item struct {
		Key     string  `json:"key"`
		Label   string  `json:"label"`
		Count   int     `json:"count"`
		Percent float64 `json:"percent"`
	}
	order := []string{"tier1", "tier2", "tier3", "unknown"}
	labels := map[string]string{"tier1": "Tier 1", "tier2": "Tier 2", "tier3": "Tier 3", "unknown": "Tanpa tier"}
	counts := map[string]int{}
	for _, c := range list {
		key := strings.ToLower(strings.TrimSpace(derefStr(c.MediaTier)))
		if _, ok := labels[key]; !ok {
			key = "unknown"
		}
		counts[key]++
	}
	total := len(list)
	items := []item{}
	used := 0.0
	last := -1
	for _, key := range order {
		n := counts[key]
		if n == 0 {
			continue
		}
		pct := 0.0
		if total > 0 {
			pct = math.Round(float64(n) / float64(total) * 100)
		}
		used += pct
		items = append(items, item{Key: key, Label: labels[key], Count: n, Percent: pct})
		last = len(items) - 1
	}
	// Sisa pembulatan ditambahkan ke item terakhir supaya total tetap 100%.
	if last >= 0 && total > 0 {
		items[last].Percent += 100 - used
	}
	return map[string]any{"total": total, "items": items}
}

func mmBuildPackage(list []mmCandidate, crit mmCriteria) ([]int, []string) {
	idx := make([]int, len(list))
	for i := range list {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return list[idx[a]].Score > list[idx[b]].Score })

	picked := []int{}
	pickedSet := map[int]bool{}
	total := 0.0
	perCat := map[string]int{}

	take := func(ignoreCap bool) {
		for _, i := range idx {
			if len(picked) >= crit.Count {
				return
			}
			if pickedSet[i] {
				continue
			}
			c := list[i]
			cat := strings.ToLower(mmCategoryKey(c))
			if !ignoreCap && perCat[cat] >= mmMaxPerCategory {
				continue
			}
			if crit.BudgetTotal > 0 && total+c.ClientPrice > crit.BudgetTotal {
				continue
			}
			picked = append(picked, i)
			pickedSet[i] = true
			perCat[cat]++
			total += c.ClientPrice
		}
	}
	take(false)
	if len(picked) < crit.Count {
		// Longgarkan batas per kategori dulu sebelum menyerah pada budget.
		take(true)
	}
	for _, i := range picked {
		list[i].InPackage = true
	}

	warnings := []string{}
	if len(picked) < crit.Count {
		warnings = append(warnings, "Paket hanya berisi "+itoa(len(picked))+" media, kurang dari "+itoa(crit.Count)+" yang diminta — kandidat yang memenuhi kriteria tidak cukup.")
	}
	if crit.BudgetTotal > 0 {
		if remaining := crit.BudgetTotal - total; remaining > 0 {
			warnings = append(warnings, "Sisa budget "+bsIntlIDR(remaining)+" belum terpakai.")
		}
	} else {
		warnings = append(warnings, "Budget total tidak disebut — total paket dihitung dari harga katalog, bukan dari anggaran yang disetujui.")
	}
	return picked, warnings
}

// ---------- payload ----------

func mmCandidatePayload(list []mmCandidate) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, c := range list {
		out = append(out, map[string]any{
			"id": c.ID, "name": c.Name, "website_url": c.WebsiteURL,
			"category": c.Category, "categories": c.Categories, "media_type": c.MediaType,
			"price": c.PartnerPrice, "client_price": c.ClientPrice, "credit_cost": c.CreditCost,
			"media_tier": c.MediaTier, "effective_quality": c.EffectiveQuality,
			"health_score": c.HealthScore, "seo_da": c.DA, "seo_pa": c.PA,
			"seo_ai_trust": c.AITrust, "seo_spam_score": c.SpamScore,
			"traffic_estimate": c.Traffic, "link_duration_months": c.LinkMonths,
			"allow_hyperlink": c.AllowHyperlink, "max_links": c.MaxLinks,
			"dewan_pers_verified": c.DewanPers, "domain_verified": c.DomainVerified,
			"published_count": c.PublishedCount, "pricing_type": c.PricingType, "price_status": c.PriceStatus,
			"bahasa_artikel": c.Bahasa, "topik_terlarang": c.TopikTerlarang,
			"label_sponsor": c.LabelSponsor, "word_count_mode": c.WordCountMode,
			"score": c.Score, "match_label": c.MatchLabel,
			"reasons": c.Reasons, "warnings": c.Warnings, "gaps": c.Gaps, "in_package": c.InPackage,
		})
	}
	return out
}

// ---------- util kecil ----------

func mmNormalizeProvenance(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "explicit", "interpreted", "assumed", "unknown":
			out[k] = strings.ToLower(strings.TrimSpace(v))
		default:
			out[k] = "unknown"
		}
	}
	return out
}

func mmParseStringList(raw *string) []string {
	out := []string{}
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return out
	}
	var arr []string
	if json.Unmarshal([]byte(*raw), &arr) == nil {
		for _, v := range arr {
			if t := strings.TrimSpace(v); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	for _, part := range strings.Split(*raw, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func mmClip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n])
}

func mmClipList(list []string, maxItems, maxLen int) []string {
	out := []string{}
	for _, v := range list {
		if t := mmClip(v, maxLen); t != "" {
			out = append(out, t)
		}
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

// mmWordRange membaca batas kata dari media.word_count_mode.
// "sesuai_permintaan"/kosong = fleksibel (media mengikuti permintaan).
func mmWordRange(mode *string) (minWords, maxWords int, flexible bool) {
	raw := strings.ToLower(strings.TrimSpace(derefStr(mode)))
	if raw == "" || raw == "sesuai_permintaan" || raw == "sesuai_permintaan_klien" {
		return 0, 0, true
	}
	parts := strings.Split(raw, "_")
	if len(parts) < 3 {
		return 0, 0, true
	}
	lo, errLo := strconv.Atoi(parts[len(parts)-2])
	hi, errHi := strconv.Atoi(parts[len(parts)-1])
	if errLo != nil || errHi != nil {
		return 0, 0, true
	}
	return lo, hi, false
}

func mmIntOr(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

// mmCategoryKey: kategori tampilan untuk komposisi paket & batas per kategori.
func mmCategoryKey(c mmCandidate) string {
	if c.Category != nil && strings.TrimSpace(*c.Category) != "" {
		return strings.TrimSpace(*c.Category)
	}
	if len(c.Categories) > 0 {
		return c.Categories[0]
	}
	return "Tanpa kategori"
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
