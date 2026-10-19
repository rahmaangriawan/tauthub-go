package brandflow

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"unicode/utf8"
)

// taut_proper.go: mode "Yang Proper" — rilis resmi dengan strict grounding.
//
// Kontraknya: AI hanya boleh menautkan klaim produk (nama, fitur, angka,
// harga, kutipan) ke [DATA FAKTA TERKUNCI] + [MEMORI BRAND COPILOT] +
// [KUTIPAN RESMI]; tidak ada penambahan klaim di luar sumber. Isian Proper
// disimpan per taut_campaign_media supaya (1) Gatekeeper memverifikasi ke
// bahan yang sama dengan Writer, dan (2) regenerasi media ini tetap terkunci
// ke fakta tanpa harus lewat modal lagi.

// tautProperInput: isian lengkap dari tab "Yang Proper" di modal Pengelola
// Naskah — dipakai saat gaya = "proper".
type tautProperInput struct {
	Fakta      string
	Dateline   string
	Narasumber string
	Kutipan    string
	Fokus      string
	BriefID    string
}

// empty: true bila seluruh isian kosong (tidak ada data Proper tersimpan).
func (p *tautProperInput) empty() bool {
	if p == nil {
		return true
	}
	return p.Fakta == "" && p.Dateline == "" && p.Narasumber == "" && p.Kutipan == "" && p.Fokus == "" && p.BriefID == ""
}

// validate: mode Proper boleh jalan dengan fakta manual ATAU Brief Copilot;
// keduanya kosong berarti tidak ada dasar grounding sehingga ditolak. Batas
// panjang mengikuti batas kolom modal, bukan batas AI.
func (p *tautProperInput) validate() error {
	if p == nil || (strings.TrimSpace(p.Fakta) == "" && strings.TrimSpace(p.BriefID) == "") {
		return errUnprocessable("Mode Proper butuh bahan: isi Data/Fakta produk atau pilih Brief Copilot dulu.")
	}
	if utf8.RuneCountInString(p.Fakta) > 20000 {
		return errUnprocessable("Fakta terkunci maksimal 20.000 karakter.")
	}
	if utf8.RuneCountInString(p.Dateline) > 100 {
		return errUnprocessable("Kota rilis maksimal 100 karakter.")
	}
	if utf8.RuneCountInString(p.Narasumber) > 200 {
		return errUnprocessable("Nama & jabatan narasumber maksimal 200 karakter.")
	}
	if utf8.RuneCountInString(p.Kutipan) > 4000 {
		return errUnprocessable("Isi kutipan maksimal 4.000 karakter.")
	}
	if utf8.RuneCountInString(p.Fokus) > 4000 {
		return errUnprocessable("Instruksi tambahan maksimal 4.000 karakter.")
	}
	if utf8.RuneCountInString(p.BriefID) > 64 {
		return errUnprocessable("ID Brief Copilot tidak valid.")
	}
	return nil
}

// saveTautProperFields: simpan isian Proper per media (non-fatal — kolom yang
// belum dimigrasi tidak boleh menggagalkan generate).
func (d *Deps) saveTautProperFields(ctx context.Context, itemID string, p *tautProperInput) {
	if p == nil || itemID == "" {
		return
	}
	err := d.exec(ctx,
		`UPDATE taut_campaign_media SET
		   proper_fakta = NULLIF($1, ''), proper_dateline = NULLIF($2, ''),
		   proper_narasumber = NULLIF($3, ''), proper_kutipan = NULLIF($4, ''),
		   proper_fokus = NULLIF($5, ''), proper_brief_id = NULLIF($6, '')
		 WHERE id = $7`,
		strings.TrimSpace(p.Fakta), strings.TrimSpace(p.Dateline), strings.TrimSpace(p.Narasumber),
		strings.TrimSpace(p.Kutipan), strings.TrimSpace(p.Fokus), strings.TrimSpace(p.BriefID), itemID)
	if err != nil {
		slog.Warn("[taut-proper] gagal menyimpan isian proper", "media_id", itemID, "err", truncateString(err.Error(), 200))
	}
}

// loadTautProperStored: isian Proper yang tersimpan untuk media ini (dipakai
// regenerasi & Gatekeeper). Bila kolom belum dimigrasi atau kosong → nil.
func (d *Deps) loadTautProperStored(ctx context.Context, itemID string) *tautProperInput {
	if itemID == "" {
		return nil
	}
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(proper_fakta, '') AS proper_fakta,
		       COALESCE(proper_dateline, '') AS proper_dateline,
		       COALESCE(proper_narasumber, '') AS proper_narasumber,
		       COALESCE(proper_kutipan, '') AS proper_kutipan,
		       COALESCE(proper_fokus, '') AS proper_fokus,
		       COALESCE(proper_brief_id, '') AS proper_brief_id
		FROM taut_campaign_media WHERE id = $1 LIMIT 1`, itemID)
	if err != nil || row == nil {
		if err != nil {
			slog.Warn("[taut-proper] gagal memuat isian tersimpan", "media_id", itemID, "err", truncateString(err.Error(), 200))
		}
		return nil
	}
	p := &tautProperInput{
		Fakta:      row.strOr("proper_fakta", ""),
		Dateline:   row.strOr("proper_dateline", ""),
		Narasumber: row.strOr("proper_narasumber", ""),
		Kutipan:    row.strOr("proper_kutipan", ""),
		Fokus:      row.strOr("proper_fokus", ""),
		BriefID:    row.strOr("proper_brief_id", ""),
	}
	if p.empty() {
		return nil
	}
	return p
}

// loadTautProperBrief: gabungan fakta Brief Copilot (profiler_profiles milik
// brand ini). Lintas-tenant tersaring via brand_id. "" bila brief tidak
// ditemukan atau seluruh kolomnya kosong.
func (d *Deps) loadTautProperBrief(ctx context.Context, brandID, briefID string) string {
	id := strings.TrimSpace(briefID)
	if id == "" {
		return ""
	}
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(entity_core, '{}'::jsonb) AS entity_core,
		       COALESCE(identity, '{}'::jsonb) AS identity,
		       COALESCE(style, '{}'::jsonb) AS style,
		       COALESCE(seo, '{}'::jsonb) AS seo
		FROM profiler_profiles
		WHERE (id = $1 OR short_code = $1) AND brand_id = $2 AND deleted_at IS NULL
		LIMIT 1`, id, brandID)
	if err != nil || row == nil {
		return ""
	}
	merged := map[string]any{}
	for _, key := range []string{"entity_core", "identity", "style", "seo"} {
		var obj map[string]any
		if json.Unmarshal([]byte(row.strOr(key, "")), &obj) == nil {
			for k, v := range obj {
				if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
					continue
				}
				merged[k] = v
			}
		}
	}
	lines := make([]string, 0, len(merged))
	for _, key := range tautProperKnownKeys {
		if v, ok := merged[key]; ok {
			if line := properFactLine(key, v); line != "" {
				lines = append(lines, line)
			}
			delete(merged, key)
		}
	}
	// Sisa kunci tak dikenal (field admin baru) tetap diserahkan ke AI.
	for k, v := range merged {
		if line := properFactLine(k, v); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// properFactLine: satu baris "Label: value" untuk blok memori AI.
func properFactLine(key string, v any) string {
	value := ""
	switch t := v.(type) {
	case string:
		value = strings.TrimSpace(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		value = strings.Join(parts, "; ")
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		if string(raw) == "null" {
			return ""
		}
		value = string(raw)
	}
	if value == "" {
		return ""
	}
	return properLabel(key) + ": " + value
}

// properLabel: label manusiawi untuk kunci copilot yang umum; sisanya dibaca
// snake-case diganti spasi.
func properLabel(key string) string {
	known := map[string]string{
		"canonical_name":        "Nama brand",
		"canonical_description": "Deskripsi brand",
		"entity_category":       "Kategori entitas",
		"canonical_links":       "Tautan resmi",
		"official_facts":        "Fakta resmi",
		"allowed_claims":        "Klaim yang boleh",
		"forbidden_claims":      "Klaim yang dilarang",
	}
	if label, ok := known[key]; ok {
		return label
	}
	return strings.ReplaceAll(key, "_", " ")
}

// tautProperKnownKeys: urutan penyajian fakta dari Brief — Entity Core dulu
// (nama brand, kategori, deskripsi), lalu fakta resmi & klaim.
var tautProperKnownKeys = []string{
	"canonical_name", "entity_category", "canonical_description",
	"official_facts", "allowed_claims", "forbidden_claims", "canonical_links",
}

// buildTautProperBlock: teks aturan strict grounding + semua sumber fakta
// Proper untuk prompt Writer DAN Checker.
func buildTautProperBlock(p *tautProperInput, briefFacts string) string {
	if p == nil || (strings.TrimSpace(p.Fakta) == "" && briefFacts == "") {
		return ""
	}
	var b strings.Builder
	b.WriteString("[MODE PROPER — RILIS RESMI, STRICT GROUNDING (WAJIB)]\n")
	b.WriteString("Klien memilih mode PROPER: ini rilis resmi berbasis fakta terkunci, bukan tulisan bebas. Aturan berikut MENANG atas gaya menulis mana pun:\n")
	b.WriteString("1. SELURUH klaim produk (nama, fitur, keunggulan, angka, harga, tanggal, lokasi, dan kutipan) WAJIB berasal dari [DATA FAKTA TERKUNCI] dan/atau [MEMORI BRAND COPILOT] di bawah. DILARANG menambah, menebak, atau menurunkan klaim di luar sumber itu (contoh dilarang: harga yang tidak tertulis, sertifikat/partnership yang tidak tertulis).\n")
	b.WriteString("2. JANGAN menambahkan fitur, layanan, penghargaan, atau kemitraan baru yang tidak muncul di sumber.\n")
	b.WriteString("3. TANPA menyebut nama kompetitor atau menyandingkan brand ini dengan perusahaan lain.\n")
	b.WriteString("4. DILARANG klaim \"nomor 1\", \"terbaik\", \"paling terpercaya\" dan sejenisnya KECUALI tertulis persis di sumber.\n")
	b.WriteString("5. Kutipan narasumber hanya boleh muncul bila ada [KUTIPAN RESMI]; pakai kata, frasa, dan tanda baca persis seperti yang tertulis dan tulis sebagai <blockquote><p>…</p></blockquote> dengan atribusi nama & jabatan. DILARANG menulis kutipan lain dari narasumber tersebut.\n")
	b.WriteString("6. Gatekeeper memverifikasi klaim ke blok yang sama — klaim tanpa sumber di sini ditandai gagal QC.\n")
	if s := strings.TrimSpace(p.Fakta); s != "" {
		b.WriteString("\n[DATA FAKTA TERKUNCI — dipakai persis seperti ini]\n" + s + "\n")
	}
	if s := strings.TrimSpace(briefFacts); s != "" {
		b.WriteString("\n[MEMORI BRAND COPILOT — fakta kanonik yang boleh dipakai]\n" + s + "\n")
	}
	if strings.TrimSpace(p.Narasumber) != "" && strings.TrimSpace(p.Kutipan) != "" {
		b.WriteString("\n[KUTIPAN RESMI — pakai persis seperti ini, tanpa mengubah kata]\n")
		b.WriteString("Narasumber: " + strings.TrimSpace(p.Narasumber) + "\n")
		b.WriteString("Kutipan: \"" + strings.TrimSpace(p.Kutipan) + "\"\n")
	} else if strings.TrimSpace(p.Narasumber) != "" {
		b.WriteString("\n[NARASUMBER TANPA KUTIPAN]\nKlien mencantumkan narasumber \"" + strings.TrimSpace(p.Narasumber) + "\" tapi tidak memberikan teks kutipan. Jadi JANGAN tulis kutipan langsung dari narasumber ini — cukup pernyataan netral (mis. \"ujar ...\") tanpa tanda kutip, atau tidak usah menyebut narasumber sama sekali.\n")
	}
	if strings.TrimSpace(p.Dateline) != "" {
		city := strings.TrimSpace(p.Dateline)
		b.WriteString("\n[DATELINE KOTA RILIS] " + city + " — jadikan kota dateline lead (format \"" + strings.ToUpper(city) + ", ...\").\n")
	}
	if strings.TrimSpace(p.Fokus) != "" {
		b.WriteString("\n[INSTRUKSI TAMBAHAN DARI KLIEN — hanya arahan tutur & fokus babak]\n" + strings.TrimSpace(p.Fokus) + "\nArahan ini TIDAK boleh menambah fakta baru di luar sumber di atas.\n")
	}
	return strings.TrimSpace(b.String())
}
