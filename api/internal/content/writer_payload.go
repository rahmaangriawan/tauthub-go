package content

import (
	"encoding/json"
	"strings"
)

// WriterPayload: tiga field yang dijanjikan prompt Writer kepada model.
type WriterPayload struct {
	Title           string
	ContentHTML     string
	MetaDescription string
	Category        string
}

// ParseWriterPayload membaca jawaban Writer yang seharusnya berupa JSON
// {title, content_html, meta_description, category?}.
//
// Model sering menulis atribut HTML dengan kutip ganda mentah di dalam nilai
// JSON, mis. <a href="https://contoh.com">. Kutip itu tidak di-escape, jadi
// JSON-nya tidak valid dan decoder menolak SELURUH payload. Dulu kegagalan itu
// membuat judul jatuh ke nama kampanye dan seluruh JSON mentah ditulis ke isi
// artikel — klien melihat teks {"title": ...} di editor.
//
// Urutan upaya:
//  1. decode JSON biasa (kasus normal, termasuk fence markdown);
//  2. ekstraksi per-field yang toleran: nilai sebuah field dianggap selesai
//     saat kutip penutupnya diikuti koma + nama key berikutnya, atau diikuti
//     penutup objek. Kutip lain di dalam nilai diperlakukan sebagai teks HTML.
//
// ok=false berarti jawaban benar-benar bukan payload Writer.
func ParseWriterPayload(raw string) (WriterPayload, bool) {
	var out WriterPayload
	text := stripJSONFence(strings.TrimSpace(raw))
	if text == "" {
		return out, false
	}

	// 1. Jalur normal.
	var direct struct {
		Title           string `json:"title"`
		ContentHTML     string `json:"content_html"`
		MetaDescription string `json:"meta_description"`
		Category        string `json:"category"`
	}
	if json.Unmarshal([]byte(text), &direct) == nil && strings.TrimSpace(direct.ContentHTML) != "" {
		out.Title = direct.Title
		out.ContentHTML = direct.ContentHTML
		out.MetaDescription = direct.MetaDescription
		out.Category = direct.Category
		return out, true
	}

	// 2. Jalur toleran.
	keys := []string{"title", "content_html", "meta_description", "category"}
	out.Title = extractTolerantString(text, "title", keys)
	out.ContentHTML = extractTolerantString(text, "content_html", keys)
	out.MetaDescription = extractTolerantString(text, "meta_description", keys)
	out.Category = extractTolerantString(text, "category", keys)
	if strings.TrimSpace(out.ContentHTML) == "" {
		return WriterPayload{}, false
	}
	return out, true
}

// stripJSONFence membuang pagar kode markdown yang membungkus JSON.
func stripJSONFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// extractTolerantString mengambil nilai string sebuah key dari teks yang
// menyerupai JSON tapi mungkin memuat kutip mentah di dalam nilainya.
func extractTolerantString(text, key string, allKeys []string) string {
	marker := `"` + key + `"`
	ki := strings.Index(text, marker)
	if ki < 0 {
		return ""
	}
	j := ki + len(marker)
	for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n' || text[j] == '\r') {
		j++
	}
	if j >= len(text) || text[j] != ':' {
		return ""
	}
	j++
	for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n' || text[j] == '\r') {
		j++
	}
	if j >= len(text) || text[j] != '"' {
		return ""
	}
	j++
	var b strings.Builder
	for j < len(text) {
		c := text[j]
		if c == '\\' && j+1 < len(text) {
			decoded, next := decodeEscape(text, j)
			b.WriteString(decoded)
			j = next
			continue
		}
		if c == '"' && isValueEnd(text, j, allKeys) {
			return b.String()
		}
		b.WriteByte(c)
		j++
	}
	// Kutip penutup tidak ditemukan (output terpotong): pakai yang sudah ada.
	return b.String()
}

// WriterValueQuoteIsTerminator: true bila kutip ganda pada posisi quotePos
// adalah penutup nilai string JSON — yaitu diikuti `}` atau `,` lalu nama key
// berikutnya. Dipakai juga oleh pembaca streaming Writer, karena sebagian
// model menulis atribut HTML dengan kutip ganda mentah di dalam nilai.
//
// Beda dengan pemeriksa di jalur parse: di sini ujung teks TIDAK dianggap
// penutup, karena akumulasi streaming bisa terpotong tepat setelah kutip
// pembuka atribut (mis. `href="`). Butuh penanda sintaks yang nyata.
func WriterValueQuoteIsTerminator(text string, quotePos int) bool {
	k := quotePos + 1
	for k < len(text) && isJSONSpace(text[k]) {
		k++
	}
	if k >= len(text) {
		return false
	}
	if text[k] == '}' {
		return true
	}
	if text[k] != ',' {
		return false
	}
	k++
	for k < len(text) && isJSONSpace(text[k]) {
		k++
	}
	if k >= len(text) || text[k] != '"' {
		return false
	}
	k++
	for _, name := range writerKeys {
		if strings.HasPrefix(text[k:], name) {
			after := k + len(name)
			if after < len(text) && text[after] == '"' {
				return true
			}
		}
	}
	return false
}

var writerKeys = []string{"title", "content_html", "meta_description", "category"}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isValueEnd: posisi j adalah kutip penutup nilai bila setelah kutip itu
// (melewati spasi) ada `}` atau `,` yang diikuti nama key berikutnya.
func isValueEnd(text string, j int, allKeys []string) bool {
	k := j + 1
	for k < len(text) && (text[k] == ' ' || text[k] == '\t' || text[k] == '\n' || text[k] == '\r') {
		k++
	}
	if k >= len(text) {
		// Kutip penutup di akhir teks.
		return true
	}
	if text[k] == '}' {
		return true
	}
	if text[k] != ',' {
		return false
	}
	k++
	for k < len(text) && (text[k] == ' ' || text[k] == '\t' || text[k] == '\n' || text[k] == '\r') {
		k++
	}
	if k >= len(text) || text[k] != '"' {
		return false
	}
	k++
	for _, name := range allKeys {
		if strings.HasPrefix(text[k:], name) {
			after := k + len(name)
			if after < len(text) && text[after] == '"' {
				return true
			}
		}
	}
	return false
}

// decodeEscape menerjemahkan satu escape JSON mulai dari posisi i (yang berisi
// backslash) dan mengembalikan hasilnya serta posisi baca berikutnya.
func decodeEscape(s string, i int) (string, int) {
	if i+1 >= len(s) {
		return "\\", i + 1
	}
	switch s[i+1] {
	case '"', '\\', '/':
		return string(s[i+1]), i + 2
	case 'n':
		return "\n", i + 2
	case 't':
		return "\t", i + 2
	case 'r':
		return "\r", i + 2
	case 'b':
		return "\b", i + 2
	case 'f':
		return "\f", i + 2
	case 'u':
		if i+6 > len(s) {
			return string(s[i+1]), i + 2
		}
		var code rune
		ok := true
		for _, c := range s[i+2 : i+6] {
			code <<= 4
			switch {
			case c >= '0' && c <= '9':
				code |= c - '0'
			case c >= 'a' && c <= 'f':
				code |= c - 'a' + 10
			case c >= 'A' && c <= 'F':
				code |= c - 'A' + 10
			default:
				ok = false
			}
		}
		if !ok {
			return string(s[i+1]), i + 2
		}
		return string(code), i + 6
	default:
		return string(s[i+1]), i + 2
	}
}
