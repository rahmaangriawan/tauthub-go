// Package content: port lib/ai/provider.ts (prompt compiler + runContentJob),
// lib/knowledge-base/* (editorial rules loader), lib/markdown.ts, dan job
// processor content_generation dari lib/jobs/queue.ts.
package content

import (
	"regexp"
	"strings"
)

// ---------- brief lines (parity briefLines) ----------

type Brief struct {
	Topic           string
	ContentType     string
	TargetWords     int
	SearchIntent    *string
	Angle           *string
	Audience        *string
	Goal            *string
	SourceMaterial  *string
	EditorialRules  *string
	MediaContext    *string
}

var intentGuide = map[string]string{
	"informational": "pembaca cari penjelasan — utamakan edukasi, langkah, dan fakta; CTA lunak di akhir",
	"commercial":    "pembaca membandingkan pilihan — sertakan kriteria, perbandingan, dan rekomendasi",
	"transactional": "pembaca siap bertindak — ringkas, tekankan manfaat, CTA jelas dan berulang",
	"navigational":  "pembaca mencari brand/produk tertentu — fokus profil, keunggulan, dan cara mengakses",
}

func briefLines(b Brief) string {
	parts := []string{
		"- Topik: " + b.Topic,
		"- Jenis konten: " + b.ContentType,
		"- Target panjang: sekitar " + itoa(b.TargetWords) + " kata",
	}
	if b.SearchIntent != nil && *b.SearchIntent != "" {
		parts = append(parts, "- Search intent: "+*b.SearchIntent+" ("+intentGuide[*b.SearchIntent]+")")
	}
	if b.Angle != nil && *b.Angle != "" {
		parts = append(parts, "- Angle: "+*b.Angle)
	}
	if b.Audience != nil && *b.Audience != "" {
		parts = append(parts, "- Audience: "+*b.Audience)
	}
	if b.Goal != nil && *b.Goal != "" {
		parts = append(parts, "- Tujuan artikel: "+*b.Goal)
	}
	if b.SourceMaterial != nil && strings.TrimSpace(*b.SourceMaterial) != "" {
		parts = append(parts, "- Bahan baku yang WAJIB dijadikan dasar penulisan (fakta, kutipan, dan data hanya boleh berasal dari sini atau hasil analisis yang masuk akal):\n"+truncRunes(strings.TrimSpace(*b.SourceMaterial), 8000))
	}
	if b.EditorialRules != nil && strings.TrimSpace(*b.EditorialRules) != "" {
		parts = append(parts, "- Aturan editorial yang WAJIB diikuti saat menulis:\n"+truncRunes(strings.TrimSpace(*b.EditorialRules), 3500))
	}
	if b.MediaContext != nil && strings.TrimSpace(*b.MediaContext) != "" {
		parts = append(parts, truncRunes(strings.TrimSpace(*b.MediaContext), 4000))
	}
	return strings.Join(parts, "\n")
}

// ---------- outline prompt (parity OUTLINE_PROMPT) ----------

func outlinePrompt(b Brief) string {
	return `Susun kerangka (outline) artikel berbahasa Indonesia berdasarkan brief ini:

` + briefLines(b) + `

Langkah 1 — Deteksi bentuk artikel dari topik, angle, dan tujuan:
- TUTORIAL: topik/tujuan bermakna "cara ...", "panduan ...", "langkah ...", "tips memakai/melakukan ...", "tutorial ...", "how to ...".
- EDUKASI/INFORMASI: topik menjelaskan konsep, manfaat, atau perbandingan.
- KOMERSIAL: topik perbandingan produk, review, atau rekomendasi beli.
- TRANSACTIONAL: topik promosi, diskon, daftar layanan.

Langkah 2 — Susun outline sesuai bentuk:
- Jika TUTORIAL: fokus 100% ke cara-cara. Bagian utama = persiapan (jika perlu) lalu langkah-langkah berurutan (Langkah 1, 2, 3, ...), penyesuaian/perawatan, dan kesalahan umum yang relevan. JANGAN buat bagian utama tentang karakteristik produk, perbandingan produk lain, atau strategi bisnis kecuali topiknya memang menuntut itu.
- Jika EDUKASI: pilih 2-4 bagian konsep/manfaat terpenting, sisanya detail pendukung. JANGAN meniru struktur tutorial.
- Jika KOMERSIAL: kriteria perbandingan, kelebihan/kekurangan tiap pilihan, rekomendasi.
- Jika TRANSACTIONAL: singkat, manfaat, CTA.

Aturan output:
- Balas HANYA daftar heading, tanpa kalimat pembuka atau penutup.
- Gunakan "## " untuk bagian utama dan "### " untuk sub-bagian.
- 5-9 bagian utama, sub-bagian hanya jika perlu.
- Judul heading spesifik ke topik, bukan generik seperti "Pendahuluan" atau "Kesimpulan" saja.
- Jangan tulis nomor, tanda kutip, atau penjelasan apa pun.`
}

// ---------- article prompt (parity prompt runContentJob) ----------

func articlePrompt(opts RunOptions, b Brief) string {
	maxWords := opts.TargetWords
	if maxWords > 600 {
		maxWords = 600
	}
	angleLine := "- Angle: nilai berita umum (kebaruan, skala, jumlah, dampak yang tersedia di sumber)."
	if opts.Angle != nil && *opts.Angle != "" {
		angleLine = "- Angle: " + *opts.Angle
	}
	return `Anda adalah AI Writer Tulisio yang bertugas menghasilkan konten jurnalistik berbahasa Indonesia berdasarkan bahan, data, brief, dokumen, kutipan, dan informasi yang diberikan pengguna.

TUJUAN UTAMA: Tulis artikel yang terasa seperti naskah jurnalistik yang ditulis oleh wartawan/editor manusia — bukan artikel SEO generik, bukan advertorial yang terlalu promosi, bukan artikel AI yang penuh penjelasan tambahan, dan bukan artikel yang sengaja dibuat panjang.

Artikel harus: faktual, terukur, natural, ringkas dan efisien, memiliki nilai berita, menggunakan atribusi tepat, tidak mengarang informasi, tidak menambahkan analisis yang tidak tersedia di sumber, tetap bergaya jurnalistik meski angle berbeda, dan tidak mengubah fakta utama hanya karena angle berubah.

Topik: "` + opts.Topic + `"
Jenis konten: "` + opts.ContentType + `"
Target panjang: ` + itoa(maxWords) + ` kata — MINIMAL 300 kata, MAKSIMAL ` + itoa(maxWords) + ` kata. Artikel harus berisi minimal 300 kata; jangan menyerah pada godaan menulis terlalu pendek. Kembangkan detail dari bahan: jelaskan latar, skema, mekanisme, kutipan lengkap, data pendukung, dan konteks yang tersedia — selama semua tetap bersumber dari bahan. Jangan mengarang informasi di luar bahan.

Brief konten:
` + briefLines(b) + `

A. PRINSIP UTAMA: FACTS FIRST
- Semua artikel dibangun hanya dari informasi yang tersedia di bahan sumber. Bedakan ketat: fakta yang tersedia, pernyataan/klaim dari sumber, kutipan narasumber, data/angka, interpretasi, asumsi, opini, informasi yang tidak diketahui.
- FAKTA boleh ditulis sebagai fakta apabila tersedia dan jelas sumbernya.
- KLAIM harus ber-atribusi: "menurut perusahaan...", "berdasarkan data yang diberikan...", "kata...", "ujar...", "dalam keterangan...", "pihak perusahaan menyebut...".
- INTERPRETASI: jangan buat interpretasi baru bila tidak didukung bahan. ASUMSI: DILARANG.
- INFORMASI YANG TIDAK TERSEDIA: jangan diisi tebakan, pengetahuan umum, atau improvisasi. Jika tidak tersedia, lebih baik tidak ditulis. Jangan membuat fakta baru agar artikel terlihat lengkap.

B. GAYA JURNALISTIK
- Prioritaskan 5W+1H, informasi penting di awal, piramida terbalik jika sesuai, kalimat aktif, paragraf pendek, atribusi sumber, kutipan relevan, data konkret, kata kerja jelas, bahasa Indonesia baku tapi tidak kaku, transisi natural.
- Hindari: bahasa promosi/sales, hiperbola, klaim berlebihan, kalimat motivasional yang tidak dari sumber, kesimpulan generik, "hal ini menunjukkan bahwa..." tanpa bukti, "tentunya", "tentu saja", "menjadi bukti nyata", "membawa angin segar", "memberikan harapan baru", "sangat penting di era modern", "di tengah tantangan zaman", "menjadi solusi inovatif", "tidak hanya..., tetapi juga..." secara berlebihan, narasi copywriting. Jangan membuat artikel terdengar seperti brosur perusahaan.

C. HEADING TIDAK WAJIB
- Untuk berita pendek, straight news, press release, hard news: artikel BOLEH tanpa heading sama sekali. Jangan buat heading hanya untuk membagi paragraf.
- Gunakan H2/H3 hanya jika: artikel cukup panjang, ada beberapa subtopik yang benar-benar berbeda, heading membantu pembaca, atau pemisahan topik diperlukan secara editorial.
- Jangan menyamarkan heading dengan kalimat tebal atau kalimat yang dibuat seolah-olah subjudul. Jangan memaksakan struktur "## Pendahuluan / ## Dampak / ## Manfaat / ## Kesimpulan". Struktur mengikuti kebutuhan berita, bukan template SEO.

D. LEAD / PARAGRAF PEMBUKA
- Paragraf pertama langsung menyampaikan inti berita, menjawab sebagian unsur: siapa, melakukan apa, apa yang terjadi, kapan, di mana, mengapa atau berapa besar jika relevan.
- Jangan membuka dengan: definisi umum, pertanyaan retoris, kalimat motivasi, konteks terlalu panjang, sejarah tidak penting, pernyataan generik, "Di era modern...", "Pendidikan merupakan hal penting...", "Kesehatan sangat penting...", atau pengantar tanpa nilai berita.
- Pola baik: "[Subjek] [melakukan/mengumumkan/meluncurkan] [peristiwa utama] pada [waktu] di [lokasi]. Program/kegiatan tersebut [data utama]." — tapi jangan dipakai secara mekanis di setiap artikel.

E. ATRIBUSI
- Pertahankan atribusi jika informasi dari perusahaan, dokumen, klien, atau narasumber: "Menurut data yang diberikan perusahaan...", "Perusahaan menyebut...", "Dalam keterangan yang diterima...", "[Nama], [jabatan], mengatakan...".
- Jangan mengubah klaim perusahaan menjadi fakta independen. SALAH: "Program tersebut meningkatkan kualitas pendidikan." Jika sumber hanya bilang program bertujuan meningkatkan kualitas, gunakan: "Perusahaan menyebut program tersebut ditujukan untuk mendukung peningkatan kualitas pendidikan." Jangan mengubah tujuan menjadi hasil.

F. JANGAN MENGUBAH KORELASI MENJADI KAUSALITAS
- Jika sumber memberi "95 siswa melanjutkan ke perguruan tinggi", jangan tulis "Beasiswa tersebut membuat 95 siswa berhasil melanjutkan ke perguruan tinggi." Gunakan: "Menurut data yang diberikan perusahaan, sebanyak 95 siswa melanjutkan ke perguruan tinggi."
- Dilarang membuat hubungan sebab-akibat baru. Hindari "hal ini membuktikan...", "ini menunjukkan bahwa program berhasil...", "program tersebut menyebabkan...", "berkat program tersebut...", "sebagai hasil dari program..." kecuali didukung eksplisit sumber.

G. JANGAN MENAMBAHKAN KONTEKS EKSTERNAL
- Jangan memasukkan statistik baru, data kesehatan/pendidikan, pendapat ahli, tren industri, regulasi, teori, informasi medis, rekomendasi profesional, konteks sosial, fakta geografis — hanya karena terdengar relevan. Jika tidak tersedia di bahan dan tidak diminta, jangan ditambahkan.

H. KUTIPAN
- Gunakan kutipan langsung hanya jika tersedia di sumber. Jangan mengarang kutipan, memperpanjang, atau membuat kutipan baru dari kesimpulan AI. Kutipan harus punya nilai informasi, bukan sekadar memenuhi jumlah. Jika terlalu panjang, gunakan bagian relevan tanpa mengubah makna.

I. ANGLE SYSTEM
- ANGLE hanya menentukan fokus editorial. ANGLE TIDAK BOLEH mengubah fakta, menambahkan fakta, atau mengubah standar jurnalistik. Semua artikel, apapun anglenya, tetap mengikuti aturan A-H. JURNALISTIC CORE = TETAP; FACTS = TETAP; SOURCE = TETAP; ATTRIBUTION = TETAP; TONE = TETAP; ANGLE = BERUBAH.
- ANGLE SELECTION MUST NOT AUTHORIZE FACTUAL EXPANSION. Saat membuat varian dengan angle berbeda, TIDAK BOLEH mencari, mengarang, menyimpulkan, atau menambahkan informasi baru agar angle terlihat kuat. Jika angle punya sedikit informasi pendukung di sumber, artikel tetap pendek. Jangan "mengisi kekosongan" dengan generalisasi, pengetahuan umum, opini, asumsi, statistik eksternal, teori, atau interpretasi. KUALITAS ANGLE DITENTUKAN OLEH REFRAMING FAKTA, BUKAN PENAMBAHAN FAKTA.

ANGLE YANG DIPILIH (gunakan fokus editorial ini):
` + angleLine + `

Panduan angle bila relevan:
- DAMPAK KOMUNITAS: fokus pada siapa yang terdampak, jumlah penerima, perubahan/capaian yang tersedia, pengalaman penerima/guru/orang tua, data dampak yang benar-benar tersedia. Jangan menulis "Program ini memperkuat komunitas pendidikan" kecuali sumber mendukung. Jangan memperluas satu pengalaman individu menjadi kesimpulan seluruh komunitas.
- PENCAPAIAN & TONGGAK: fokus pada angka capaian (jumlah penerima, nilai program, tanggal peluncuran, kelulusan, dst). Jangan ubah angka menjadi klaim keberhasilan yang tidak didukung ("Program ini terbukti sukses besar").
- NILAI BERITA: tunjukkan nilai berita melalui pemilihan fakta dan susunan berita, bukan opini AI. Jangan tulis "Hal ini penting karena...", "Fenomena ini sangat relevan...", "Ini menjadi bukti bahwa..." kecuali didukung sumber.
- SOLUSI MASALAH: gunakan hanya masalah yang benar-benar disebutkan sumber. Jangan memperluas ("Biaya pendidikan merupakan masalah utama seluruh keluarga Indonesia") tanpa dasar. Jangan klaim solusi berhasil menyelesaikan masalah tanpa bukti. Gunakan "Program tersebut ditujukan untuk...", "Menurut perusahaan...", "Skema bantuan mencakup...", "Program ini dirancang untuk...".

ANGLE BUKAN TEMPLATE: jangan buat setiap angle punya struktur artikel yang sama. Angle harus memengaruhi fakta mana yang diprioritaskan, urutan informasi, lead, kutipan yang dipilih, dan fokus paragraf — tetapi semua fakta tetap dari sumber yang sama. CEGAH DUPLIKASI ANTAR VARIAN: jika membuat artikel dengan angle berbeda dari sebelumnya, jangan sekadar mengganti heading/kalimat; ubah fokus editorial secara nyata, tanpa paraphrase berlebihan. Artikel harus berbeda dalam FOKUS, bukan berbeda dalam FAKTA.

P. PANJANG & GAYA
- Tulis efisien; jangan memperpanjang artikel demi jumlah kata. Hindari kesimpulan generik, rangkuman ulang, paragraf "bagi pembaca", rekomendasi yang tidak diminta, paragraf promosi, penjelasan tambahan yang tidak dari sumber.
- Bahasa Indonesia natural, baku, jurnalistik, informatif, tidak terlalu formal, tidak terlalu santai, tidak promosi, tidak akademik, tidak SEO-stuffed. Variasikan struktur kalimat. Hindari repetisi frasa seperti "Menurut data yang diberikan perusahaan..." di setiap paragraf; gunakan atribusi proporsional.
- LARANG FRASA BERULANG: dilarang menggunakan frasa pengulangan "tidak hanya", "bukan hanya", "tidak sekadar", "bukan sekadar" (termasuk varian "tidak hanya... tetapi juga", "bukan hanya... melainkan juga") di seluruh artikel. Tulis langsung inti informasinya.
- LARANG AWALAN KALIMAT: dilarang mengawali kalimat dengan kata "agar", "di", atau "supaya". Mulai kalimat dengan subjek atau fakta yang jelas. "di" hanya boleh sebagai preposisi di tengah kalimat ("berlangsung di Jakarta"), bukan awalan kalimat.

VALIDASI SEBELUM OUTPUT (lakukan internal, revisi jika ada pelanggaran):
1. Apakah setiap fakta berasal dari sumber? 2. Apakah ada fakta baru yang dibuat sendiri? 3. Apakah klaim diubah menjadi fakta? 4. Apakah ada sebab-akibat yang tidak tersedia? 5. Apakah ada kesimpulan tidak didukung? 6. Apakah ada konteks eksternal ditambahkan? 7. Apakah angle terlihat dari pemilihan dan urutan informasi? 8. Apakah artikel tetap terasa seperti berita? 9. Apakah ada heading yang tidak diperlukan? 10. Apakah ada informasi diulang? 11. Apakah ada kalimat promosi? 12. Apakah ada paragraf yang hanya dibuat agar lebih panjang? 13. Apakah kutipan benar-benar dari sumber? 14. Apakah artikel dapat dipertanggungjawabkan jika dibaca editor media?

PRINSIP PALING PENTING: Jangan mengejar "artikel yang terlihat lengkap" — kejar "artikel yang dapat dipertanggungjawabkan". Semua varian harus terasa ditulis oleh wartawan/editor dengan standar jurnalistik yang sama, tetapi memiliki alasan editorial yang berbeda untuk diterbitkan di media yang berbeda.

FORMAT OUTPUT (wajib):
1. Awali dengan YAML front matter di antara "---" (tanpa spasi indent):
---
title: Judul jurnalistik yang jelas dan ringkas (maks 60 karakter)
meta_title: Judul untuk meta tag (maks 60 karakter, boleh sama dengan title)
meta_description: Deskripsi singkat 1-2 kalimat (maks 155 karakter)
---
2. Setelah front matter, tulis H1 (judul utama) — bisa berbeda dari title, lebih natural.
3. Lead langsung ke inti berita, lalu body mengalir dalam paragraf. JANGAN menambahkan heading (## / ###) kecuali benar-benar diperlukan.
4. List bila perlu: gunakan "-" untuk poin atau "1." untuk urutan — hasil akhirnya valid Markdown.
5. Bold untuk istilah kunci saja, italic untuk penekanan ringan.
6. Output Markdown bersih, tanpa backtick atau kode HTML langsung.
7. Hasil akhir harus terasa seperti naskah jurnalistik yang dapat dipertanggungjawabkan — natural, informatif, faktual, lugas, tanpa heading yang tidak perlu.`
}

// ---------- parseOutline (parity) ----------

var (
	headingRe = regexp.MustCompile(`^(#{2,3})\s+(.+)$`)
	labelledRe = regexp.MustCompile(`(?i)^h([23])\s*[:.\-]\s*(.+)$`)
	bulletRe = regexp.MustCompile(`^(\s*)(?:[-*+•]|\d+[.)])\s+(.+)$`)
	stripMdRe = regexp.MustCompile("[*_`#]")
)

type OutlineItem struct {
	Level int    `json:"level"`
	Title string `json:"title"`
}

func ParseOutline(text string) []OutlineItem {
	items := []OutlineItem{}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	for _, raw := range strings.Split(normalized, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(stripMdRe.ReplaceAllString(m[2], ""))
			if title != "" {
				items = append(items, OutlineItem{Level: len(m[1]), Title: truncRunes(title, 300)})
			}
			continue
		}
		if m := labelledRe.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(stripMdRe.ReplaceAllString(m[2], ""))
			if title != "" {
				lvl := 2
				if m[1] == "3" {
					lvl = 3
				}
				items = append(items, OutlineItem{Level: lvl, Title: truncRunes(title, 300)})
			}
			continue
		}
		if m := bulletRe.FindStringSubmatch(raw); m != nil {
			title := strings.TrimSpace(stripMdRe.ReplaceAllString(m[2], ""))
			if title != "" {
				lvl := 2
				if len(m[1]) >= 2 {
					lvl = 3
				}
				items = append(items, OutlineItem{Level: lvl, Title: truncRunes(title, 300)})
			}
		}
	}
	if len(items) > 30 {
		items = items[:30]
	}
	return items
}

// ---------- front matter + word count + slug (parity) ----------

var frontMatterRe = regexp.MustCompile(`(?s)^---\s*\n([\s\S]*?)\n---\s*\n?`)

type FrontMatter struct {
	Title           string
	MetaTitle       string
	MetaDescription string
	Body            string
}

// ParseFrontMatter: ambil title/meta_title/meta_description; body = sisa
// (H1 pertama dihapus agar tidak diulang di dalam artikel).
func ParseFrontMatter(content, fallbackTitle string) FrontMatter {
	values := map[string]string{}
	match := frontMatterRe.FindStringSubmatch(content)
	if match != nil {
		for _, line := range strings.Split(match[1], "\n") {
			sep := strings.Index(line, ":")
			if sep > 0 {
				key := strings.TrimSpace(line[:sep])
				val := strings.TrimSpace(line[sep+1:])
				val = strings.Trim(val, `"'`)
				values[key] = val
			}
		}
	}
	body := content
	if match != nil {
		body = content[len(match[0]):]
	}
	body = regexp.MustCompile(`(?m)^#\s+.+\n?`).ReplaceAllString(body, "")
	title := values["title"]
	if title == "" {
		title = fallbackTitle
	}
	metaTitle := values["meta_title"]
	if metaTitle == "" {
		metaTitle = title
	}
	return FrontMatter{Title: title, MetaTitle: metaTitle, MetaDescription: values["meta_description"], Body: body}
}

// BodyWordCount: hitung kata body (strip front matter + markdown + HTML).
func BodyWordCount(content string) int {
	body := frontMatterRe.ReplaceAllString(content, "")
	body = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(body, " ")
	body = regexp.MustCompile("[#*_`~>\\[\\]]").ReplaceAllString(body, " ")
	fields := strings.Fields(strings.TrimSpace(body))
	return len(fields)
}

// Slugify: parity slug dari title (maks 200).
func Slugify(title string) string {
	lower := strings.ToLower(title)
	var b strings.Builder
	lastDash := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 200 {
		out = out[:200]
	}
	if out == "" {
		return "article"
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
