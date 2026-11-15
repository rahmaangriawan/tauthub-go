package brandflow

import (
	"context"
	"regexp"
	"strings"
)

// Artikel turunan ("Pakai artikel untuk media lain"): artikel baru untuk media lain
// dalam kampanye yang sama, dengan topik, sudut, gaya, dan fakta yang sama dengan
// artikel asal, tetapi kalimat dan strukturnya berbeda supaya tidak dianggap
// konten duplikat oleh mitra.

const (
	// Biaya default per artikel turunan (kredit). Bisa diubah admin lewat
	// platform_settings.writer_derivative_credit_cost.
	defaultDerivativeCreditCost = 1500
	derivativeCreditCostKey     = "writer_derivative_credit_cost"

	// Batas "terlalu mirip": bagian potongan 5 kata artikel baru yang juga ada
	// di artikel asal. Satu konstanta supaya mudah disetel dari hasil nyata.
	derivedSimilarityMax = 0.35
	derivedShingleSize   = 5

	// Teks artikel asal yang dibawa sebagai rujukan fakta (karakter).
	derivedSourceTextCap = 6000

	derivedChargePrefix = "taut-writer-derived:"
)

// tautDerivedSource: artikel asal yang dijadikan rujukan fakta.
type tautDerivedSource struct {
	MediaID   string
	Slot      int
	MediaName string
	Title     string
	Text      string
}

// derivativeCreditCost: tarif artikel turunan dari platform_settings.
func (d *Deps) derivativeCreditCost(ctx context.Context) int {
	row, err := d.queryRow(ctx, "SELECT value FROM platform_settings WHERE key = $1 LIMIT 1", derivativeCreditCostKey)
	if err != nil || row == nil {
		return defaultDerivativeCreditCost
	}
	n := 0
	for _, r := range strings.TrimSpace(row.strOr("value", "")) {
		if r < '0' || r > '9' {
			return defaultDerivativeCreditCost
		}
		n = n*10 + int(r-'0')
	}
	if n <= 0 {
		return defaultDerivativeCreditCost
	}
	return n
}

// derivedPromptBlock: blok rujukan untuk prompt Writer.
func derivedPromptBlock(src *tautDerivedSource) string {
	if src == nil {
		return ""
	}
	text := src.Text
	if r := []rune(text); len(r) > derivedSourceTextCap {
		text = string(r[:derivedSourceTextCap])
	}
	return strings.Join([]string{
		"ARTIKEL RUJUKAN (JANGAN DISALIN): artikel berikut sudah ditulis untuk media lain (" + src.MediaName + ") dengan topik, sudut, dan fakta yang sama. Tulis artikel BARU untuk media ini.",
		"Aturan artikel turunan (WAJIB): fakta, angka, nama, kutipan, dan tautan harus tetap sama dan hanya boleh diambil dari artikel rujukan ini dan bahan lain di prompt. Judul, paragraf pembuka, urutan pembahasan, struktur paragraf, dan SEMUA kalimat harus berbeda. DILARANG menyalin atau memparafrasekan kalimat demi kalimat. Jangan memakai lima kata berurutan yang sama dengan rujukan, kecuali nama, istilah baku, atau kutipan resmi.",
		"Judul rujukan: " + src.Title,
		"Isi rujukan:\n" + text,
	}, "\n")
}

// derivedRetryInstruction: dipakai sekali bila hasil pertama terlalu mirip.
func derivedRetryInstruction() string {
	return "REVISI WAJIB: draf sebelumnya terlalu mirip dengan artikel rujukan. Tulis ulang dari awal dengan pembuka, urutan, struktur paragraf, dan pilihan kata yang benar-benar berbeda. Fakta tetap sama."
}

var shingleWordRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

func shingleSet(text string, size int) map[string]struct{} {
	words := shingleWordRe.FindAllString(strings.ToLower(text), -1)
	out := map[string]struct{}{}
	if len(words) < size {
		if len(words) > 0 {
			out[strings.Join(words, " ")] = struct{}{}
		}
		return out
	}
	for i := 0; i+size <= len(words); i++ {
		out[strings.Join(words[i:i+size], " ")] = struct{}{}
	}
	return out
}

// shingleSimilarity: bagian potongan kata berurutan milik `candidate` yang juga
// ada di `source` (0 sampai 1). Hanya perbandingan teks, tanpa model.
func shingleSimilarity(candidate, source string) float64 {
	a := shingleSet(candidate, derivedShingleSize)
	if len(a) == 0 {
		return 0
	}
	b := shingleSet(source, derivedShingleSize)
	if len(b) == 0 {
		return 0
	}
	hit := 0
	for k := range a {
		if _, ok := b[k]; ok {
			hit++
		}
	}
	return float64(hit) / float64(len(a))
}

// loadDerivedSource memuat artikel asal sebagai rujukan fakta: judul, teks, dan
// nama media. Mengembalikan juga baris penempatan asal (id, sudut, jenis media)
// supaya pemanggil bisa mewarisi gaya, brief, dan sudutnya.
func (d *Deps) loadDerivedSource(ctx context.Context, campaignID, srcMediaID string, srcSlot int) (*tautDerivedSource, bsRow, error) {
	if srcSlot < 1 {
		srcSlot = 1
	}
	srcRow, err := d.queryRow(ctx, `
		SELECT tcm.id, tcm.article_document_id, tcm.angle_slug, m.media_type
		FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id
		WHERE tcm.campaign_id = $1 AND tcm.media_id = $2 AND tcm.slot = $3 LIMIT 1`,
		campaignID, srcMediaID, srcSlot)
	if err != nil {
		return nil, nil, err
	}
	if srcRow == nil || !srcRow.has("article_document_id") {
		return nil, nil, errUnprocessable("Artikel sumber tidak ditemukan.")
	}
	docID := srcRow.strOr("article_document_id", "")
	doc, err := d.queryRow(ctx, "SELECT title, current_version FROM content_documents WHERE id = $1 LIMIT 1", docID)
	if err != nil || doc == nil {
		return nil, nil, errUnprocessable("Artikel sumber tidak ditemukan.")
	}
	ver, err := d.queryRow(ctx, "SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		docID, doc.intOr("current_version", 1))
	if err != nil || ver == nil {
		return nil, nil, errUnprocessable("Isi artikel sumber tidak ditemukan.")
	}
	text := strings.TrimSpace(stripHTMLToText(ver.strOr("content", "")))
	if text == "" {
		return nil, nil, errUnprocessable("Isi artikel sumber kosong.")
	}
	return &tautDerivedSource{
		MediaID: srcMediaID, Slot: srcSlot, MediaName: d.mediaNameFor(ctx, srcMediaID),
		Title: doc.strOr("title", ""), Text: text,
	}, srcRow, nil
}

// siblingDerivedTexts: teks artikel turunan lain dari sumber yang sama (selain
// media dan slot yang sedang ditulis). Hanya dipakai untuk pemeriksaan
// kemiripan; tidak masuk prompt.
func (d *Deps) siblingDerivedTexts(ctx context.Context, campaignID string, src *tautDerivedSource, mediaID string, slot int) []string {
	rows, err := d.queryRows(ctx, `
		SELECT cv.content
		FROM taut_campaign_media t
		JOIN content_documents cd ON cd.id = t.article_document_id
		JOIN content_versions cv ON cv.document_id = cd.id AND cv.version = cd.current_version
		WHERE t.campaign_id = $1 AND t.derived_from_media_id = $2 AND t.derived_from_slot = $3
		  AND NOT (t.media_id = $4 AND t.slot = $5)
		LIMIT 12`, campaignID, src.MediaID, src.Slot, mediaID, slot)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if t := strings.TrimSpace(stripHTMLToText(r.strOr("content", ""))); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// derivedScore: skor kemiripan tertinggi terhadap artikel asal dan artikel
// turunan lain dari sumber yang sama.
func derivedScore(candidate string, src *tautDerivedSource, siblings []string) float64 {
	best := shingleSimilarity(candidate, src.Text)
	for _, sib := range siblings {
		if v := shingleSimilarity(candidate, sib); v > best {
			best = v
		}
	}
	return best
}
