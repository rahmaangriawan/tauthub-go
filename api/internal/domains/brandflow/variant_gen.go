package brandflow

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
)

// Port lib/content/variants.ts + lib/markdown (markdownToHtml) untuk jalur
// varian artikel kampanye. Jalur AI nyata: bila provider belum dikonfigurasi,
// kembalikan pesan konfigurasi jelas (409) — bukan 500.

type variantAngle struct {
	ID          string
	Label       string
	Instruction string
}

// ANGLE_POOL: salinan persis lib/content/variants.ts ANGLE_POOL.
var anglePool = []variantAngle{
	{ID: "news_value", Label: "Nilai berita", Instruction: "Tekankan kebaruan dan dampak bagi pembaca/industri — apa yang berubah dan mengapa penting sekarang."},
	{ID: "benefit", Label: "Manfaat pembaca", Instruction: "Fokus pada manfaat praktis bagi pembaca: apa artinya bagi mereka dan apa yang bisa mereka lakukan."},
	{ID: "story", Label: "Kisah di baliknya", Instruction: "Sajikan sebagai kisah: latar, tokoh, perjalanan, dan momen kunci — naratif namun tetap faktual."},
	{ID: "data", Label: "Data & angka", Instruction: "Jadikan data, statistik, dan fakta terukur sebagai tulang punggung penyajian; angka tampil menonjol."},
	{ID: "expert", Label: "Sudut pakar & kutipan", Instruction: "Bangun artikel di sekitar narasumber/kutipan dan pandangan pakar — kredibilitas lewat suara pihak terkait."},
	{ID: "community", Label: "Dampak komunitas", Instruction: "Fokus pada dampak terhadap masyarakat/komunitas dan pihak yang terbantu atau terpengaruh."},
	{ID: "how_to", Label: "Panduan praktis", Instruction: "Sajikan sebagai panduan: langkah demi langkah, tips, dan hal yang perlu diperhatikan pembaca."},
	{ID: "trend", Label: "Konteks tren", Instruction: "Hubungkan dengan tren, momen, atau isu yang sedang ramai — beri konteks mengapa relevan saat ini."},
	{ID: "solution", Label: "Solusi masalah", Instruction: "Rangkaikan sebagai jawaban atas masalah: masalah → mengapa terjadi → bagaimana diatasi."},
	{ID: "milestone", Label: "Pencapaian & tonggak", Instruction: "Tekankan pencapaian, tonggak sejarah, dan apa yang diraih sejauh ini dengan bukti pendukung."},
}

// mulberry32: PRNG berbenih deterministik (parity lib/content/variants.ts).
func mulberry32(seed uint32) func() float64 {
	state := seed
	return func() float64 {
		state += 0x6d2b79f5
		t := state
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return float64((t^(t>>14))>>0) / 4294967296
	}
}

// pickAngles: pilih `count` angle berbeda, diacak dengan `seed`.
func pickAngles(count, seed int) []variantAngle {
	shuffled := make([]variantAngle, len(anglePool))
	copy(shuffled, anglePool)
	random := mulberry32(uint32(seed))
	for i := len(shuffled) - 1; i > 0; i-- {
		j := int(math.Floor(random() * float64(i+1)))
		if j < 0 {
			j = 0
		}
		if j > i {
			j = i
		}
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	if count < 0 {
		count = 0
	}
	if count > len(shuffled) {
		count = len(shuffled)
	}
	return shuffled[:count]
}

func angleByID(id string) (variantAngle, bool) {
	for _, a := range anglePool {
		if a.ID == id {
			return a, true
		}
	}
	return variantAngle{}, false
}

// parsedArticle: parity ParsedArticle.
type parsedArticle struct {
	Title           string
	MetaTitle       string
	MetaDescription string
	Body            string
}

// parseFrontMatter: parse YAML-ish front matter + buang H1 pertama di body.
func parseFrontMatter(content string) parsedArticle {
	body := strings.TrimSpace(content)
	values := map[string]string{}
	if match := findFrontMatter(content); match != nil {
		for _, line := range strings.Split(match[1], "\n") {
			sep := strings.Index(line, ":")
			if sep > 0 {
				key := strings.TrimSpace(line[:sep])
				val := strings.TrimSpace(line[sep+1:])
				val = strings.Trim(val, `"'`)
				values[key] = val
			}
		}
		body = strings.TrimSpace(content[len(match[0]):])
	}
	fallbackTitle := values["title"]
	if fallbackTitle == "" {
		fallbackTitle = "Artikel Tulisio"
	}
	fallbackTitle = truncateString(fallbackTitle, 300)
	metaTitle := values["meta_title"]
	if metaTitle == "" {
		metaTitle = fallbackTitle
	}
	// Hapus H1 pertama (markdown "# ..." atau HTML <h1>).
	if md := reMDH1.FindStringIndex(body); md != nil && md[0] == 0 {
		body = strings.TrimSpace(body[md[1]:])
	} else if html := reHTMLH1.FindStringIndex(body); html != nil && html[0] == 0 {
		body = strings.TrimSpace(body[html[1]:])
	}
	return parsedArticle{
		Title:           fallbackTitle,
		MetaTitle:       truncateString(metaTitle, 255),
		MetaDescription: truncateString(values["meta_description"], 500),
		Body:            body,
	}
}

// findFrontMatter: cari blok `---\n...\n---\n?` di awal konten.
// Mengembalikan [full, inner] dalam indeks byte (match[0] = panjang penuh).
func findFrontMatter(content string) []string {
	if !strings.HasPrefix(content, "---") {
		return nil
	}
	rest := content[3:]
	if !strings.HasPrefix(rest, "\n") && !strings.HasPrefix(rest, "\r\n") {
		return nil
	}
	// inner mulai setelah baris pertama.
	start := strings.IndexByte(rest, '\n') + 1
	if start <= 0 {
		return nil
	}
	remaining := rest[start:]
	// cari penutup "\n---"
	idx := strings.Index(remaining, "\n---")
	if idx < 0 {
		return nil
	}
	inner := remaining[:idx]
	// full = 3 (dasbor awal) + start + idx + 4 (\n---) + sisa baris penutup.
	end := idx + 4
	for end < len(remaining) && (remaining[end] == '\n' || remaining[end] == '\r') {
		end++
	}
	total := 3 + start + end
	return []string{content[:total], inner}
}

// buildVariantPrompt: parity buildPrompt lib/content/variants.ts.
func buildVariantPrompt(mode string, sourceMaterial, existingArticle *string, angle variantAngle, mediaName string, editorialRules *string) string {
	var b strings.Builder
	common := []string{
		"Anda adalah AI Writer sekaligus editor jurnalistik Tulisio. Tulis SATU artikel jurnalistik berbahasa Indonesia yang natural, faktual, ringkas, dan layak dibaca sebagai berita/news release — bukan artikel SEO, bukan advertorial, bukan tulisan AI yang sengaja dibuat panjang.",
		"Media tujuan: " + mediaName + ".",
		"Sudut pemberitaan WAJIB untuk artikel ini: " + angle.Label + " — " + angle.Instruction,
	}
	if editorialRules != nil && strings.TrimSpace(*editorialRules) != "" {
		common = append(common, "ATURAN EDITORIAL (wajib diikuti — jangan mengubah fakta dari bahan):\n"+truncateString(strings.TrimSpace(*editorialRules), 3500))
	}
	common = append(common,
		"ATURAN JURNALISTIK (wajib):",
		"1. FACTS FIRST — semua fakta hanya dari bahan; klaim perusahaan/narasumber WAJIB ber-atribusi (\"menurut perusahaan...\", \"pihak perusahaan menyebut...\", \"kata...\"). JANGAN ubah klaim menjadi fakta; jangan mengarang angka, nama, tanggal, kutipan, atau konteks eksternal.",
		"2. LEAD langsung ke inti berita (siapa, apa, kapan, di mana, mengapa penting). Jangan buka dengan definisi umum, kalimat motivasi, atau \"Di era modern...\".",
		"3. HEADING TIDAK WAJIB — untuk berita/press release pendek DEFAULT TANPA HEADING (## / ###). Jangan membuat heading terselubung (kalimat bold sebagai subjudul). Bangun alur dengan paragraf yang saling terhubung.",
		"4. Piramida terbalik: fakta utama di awal, lalu detail, data, kutipan relevan, konteks, penutup natural. Tanpa kesimpulan buatan.",
		"5. Jangan mengubah korelasi menjadi kausalitas; jangan menambahkan analisis/interpretasi yang tidak ada di bahan.",
		"6. Bahasa Indonesia jurnalistik: lugas, aktif, paragraf pendek, tanpa bahasa promosi (\"solusi inovatif\", \"komitmen nyata\", \"membawa angin segar\") dan tanpa frasa AI generik (\"hal ini menunjukkan bahwa...\", \"tentunya\").",
		"7. Panjang: MINIMAL 300 kata, MAKSIMAL 600 kata — pastikan artikel berisi minimal 300 kata dengan mengembangkan detail dari bahan (latar, skema, mekanisme, kutipan lengkap, data pendukung); jangan mengarang informasi di luar bahan.",
		"Format Wajib:",
		"1. Awali dengan YAML front matter:",
		"---",
		"title: Judul jurnalistik jelas dan ringkas (maks 60 karakter)",
		"meta_title: Judul meta (maks 60 karakter)",
		"meta_description: Deskripsi 1-2 kalimat (maks 155 karakter)",
		"---",
		"2. Setelah front matter, tulis H1 (judul utama). Body mengalir dalam paragraf tanpa heading tambahan, kecuali benar-benar diperlukan.",
		"3. List bila perlu: gunakan \"-\" untuk poin atau \"1.\" untuk urutan — valid Markdown.",
		"4. Hanya gunakan fakta, angka, dan kutipan dari bahan — JANGAN mengarang fakta baru.",
		"5. Jangan menyebut 'menurut bahan', 'berdasarkan brief', atau sejenisnya di dalam artikel.",
		"",
	)
	b.WriteString(strings.Join(common, "\n"))
	b.WriteString("\n")
	if mode == "paraphrase" {
		existing := ""
		if existingArticle != nil {
			existing = strings.TrimSpace(*existingArticle)
		}
		b.WriteString(strings.Join([]string{
			"Tugas: PARAFRASE artikel di bawah ini dengan sudut pemberitaan baru di atas.",
			"- Pertahankan SEMUA fakta, angka, kutipan, dan nama yang ada di artikel asli.",
			"- Ubah struktur kalimat, urutan penyajian, dan penekanan — hasil harus terasa sebagai artikel baru, bukan salinan.",
			"- Buat judul baru yang mencerminkan sudut pemberitaan baru.",
			"",
			"ARTIKEL ASLI:",
			truncateString(existing, 40000),
		}, "\n"))
		return b.String()
	}
	source := ""
	if sourceMaterial != nil {
		source = strings.TrimSpace(*sourceMaterial)
	}
	b.WriteString(strings.Join([]string{
		"Tugas: tulis artikel berita/feature dari BAHAN (informasi mentah) di bawah ini.",
		"BAHAN:",
		truncateString(source, 40000),
	}, "\n"))
	return b.String()
}

// generateVariantArticle: jalur AI nyata + validasi minimal 300 kata.
// Bila provider belum dikonfigurasi → bfStatusError 409 pesan konfigurasi.
func (d *Deps) generateVariantArticle(ctx context.Context, mode string, sourceMaterial, existingArticle *string, angle variantAngle, mediaName string, editorialRules *string) (parsedArticle, error) {
	runtime, err := ai.LoadAIRuntime(ctx, d.Pool)
	if err != nil {
		return parsedArticle{}, err
	}
	if !ai.IsAIConfigured(runtime) {
		return parsedArticle{}, &bfStatusError{
			Status:  409,
			Message: "AI generasi artikel belum dikonfigurasi — hubungi admin Tulisio.",
		}
	}
	taskType := "content_generation"
	if mode == "paraphrase" {
		taskType = "content_paraphrase"
	}
	result, err := ai.GenerateChat(ctx, d.Pool, ai.GenerateChatOptions{
		Prompt:   buildVariantPrompt(mode, sourceMaterial, existingArticle, angle, mediaName, editorialRules),
		Tier:     "default",
		TaskType: taskType,
		Runtime:  runtime,
	})
	if err != nil {
		return parsedArticle{}, err
	}
	parsed := parseFrontMatter(result.Content)
	if contentengine.BodyWordCount(parsed.Body) < 300 {
		return parsedArticle{}, errUnprocessable(
			"Hasil generasi artikel hanya " + itoa(contentengine.BodyWordCount(parsed.Body)) + " kata — minimal 300 kata. Coba lagi.")
	}
	return parsed, nil
}

// editorialRulesFor: parity loadEditorialRulesForGeneration (best-effort).
func (d *Deps) editorialRulesFor(ctx context.Context, brandID, contentType string, topic, sourceMaterial *string) *string {
	return contentengine.LoadEditorialRulesForGeneration(ctx, d.Pool, contentType, derefOr(topic, ""), sourceMaterial, nil)
}

// insertVariantDocument: parity insertVariantDocument — dokumen approved + versi 1.
func (d *Deps) insertVariantDocument(ctx context.Context, brandID, userID string, parsed parsedArticle, angle variantAngle, mode string) (string, error) {
	documentID := uuid.NewString()
	// job_id NOT NULL + UNIQUE: id sintetis menjaga kontrak 1:1 tanpa baris content_jobs.
	jobID := uuid.NewString()
	slug := contentengine.Slugify(parsed.Title)
	if slug == "" {
		slug = "artikel"
	}
	slug = truncateString(slug, 200)
	attribution := "ai_generated"
	if mode != "generate" {
		attribution = "ai_paraphrased"
	}
	note := "Variant " + map[bool]string{true: "dibuatkan", false: "parafrase"}[mode == "generate"] + " Tulisio — sudut: " + angle.Label
	if err := d.exec(ctx,
		"INSERT INTO content_documents (id, brand_id, job_id, status, title, slug, meta_title, meta_description, current_version, created_by, approved_by, approved_at) VALUES ($1, $2, $3, 'approved', $4, $5, $6, $7, 1, $8, $9, NOW())",
		documentID, brandID, jobID, truncateString(parsed.Title, 300), slug,
		truncateString(parsed.MetaTitle, 255), truncateString(parsed.MetaDescription, 500), userID, userID); err != nil {
		return "", err
	}
	if err := d.exec(ctx,
		"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, 1, $3, 'html', $4, $5, $6)",
		uuid.NewString(), documentID, contentengine.MarkdownToHtml(parsed.Body), attribution, truncateString(note, 255), userID); err != nil {
		return "", err
	}
	return documentID, nil
}

// jsonStringify: parity JSON.stringify (kompak).
func jsonStringify(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
