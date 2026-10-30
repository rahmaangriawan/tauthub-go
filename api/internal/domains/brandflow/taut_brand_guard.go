package brandflow

import (
	"context"
	"regexp"
	"strings"

	"unicode"
)

// taut_brand_guard.go: penjagaan agar satu artikel tidak membahas dua brand.
//
// Instruksi topik dari klien kadang menyebut brand lain di luar brand kampanye
// — mis. brief "Salsari Resto" yang menyuruh "integrasikan Giovani Barbershop
// sebagai brand relevan". Model menuruti apa adanya: judulnya jadi
// "Giovani Barbershop dan Salsari Resto: ...", isinya berpindah topik, dan
// pembaca tidak tahu artikel ini tentang apa. Penyebabnya bukan bug penulisan,
// tapi instruksi yang mencampur dua subjek; karena itu penjagaannya berupa
// peringatan keras di prompt, bukan pembuangan instruksi (klien tetap berhak
// menyinggung brand lain sebagai konteks).

// otherBrandMentions: nama brand lain (terdaftar di tabel brands) yang disebut
// di instruksi klien. Brand kampanye dan nama media dikecualikan.
//
// Pencocokan memakai kata utuh dan mengabaikan huruf besar/kecil; nama brand
// minimal 4 karakter supaya singkatan pendek tidak memicu salah tuduh.
func (d *Deps) otherBrandMentions(ctx context.Context, topic, ownBrand, mediaName string) []string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil
	}
	rows, err := d.queryRows(ctx, "SELECT name FROM brands WHERE deleted_at IS NULL")
	if err != nil {
		return nil // best-effort: tanpa daftar brand, jangan blokir penulisan
	}
	normalizedTopic := normalizeForBrandMatch(topic)
	skip := map[string]bool{
		normalizeForBrandMatch(ownBrand): true,
		normalizeForBrandMatch(mediaName): true,
	}
	out := []string{}
	seen := map[string]bool{}
	for _, row := range rows {
		name := strings.TrimSpace(row.strOr("name", ""))
		key := normalizeForBrandMatch(name)
		if key == "" || utf8RuneLen(key) < 4 || skip[key] || seen[key] {
			continue
		}
		// Nama brand berupa kata umum ("Media", "Blogger", "Berita") akan cocok
		// dengan hampir semua instruksi dan memicu peringatan palsu di setiap
		// artikel. Lewati nama yang seluruh katanya kata umum.
		if brandNameTooGeneric(key) {
			continue
		}
		seen[key] = true
		if containsWholePhrase(normalizedTopic, key) {
			out = append(out, name)
		}
	}
	return out
}

// tautGenericBrandWords: kata yang terlalu umum untuk dijadikan penanda brand.
var tautGenericBrandWords = map[string]bool{
	"media": true, "blog": true, "blogger": true, "berita": true, "news": true,
	"online": true, "digital": true, "indonesia": true, "nasional": true,
	"resto": true, "restoran": true, "cafe": true, "toko": true, "shop": true,
	"store": true, "group": true, "official": true, "center": true, "centre": true,
	"barbershop": true, "salon": true, "hotel": true, "travel": true,
}

// brandNameTooGeneric: semua kata pada nama brand termasuk kata umum.
func brandNameTooGeneric(key string) bool {
	words := strings.Fields(key)
	if len(words) == 0 {
		return true
	}
	for _, w := range words {
		if !tautGenericBrandWords[w] {
			return false
		}
	}
	return true
}

// normalizeForBrandMatch: huruf kecil, tanda baca jadi spasi, spasi dirapikan —
// supaya "Giovani Barbershop" cocok dengan "giovani  barbershop," di teks.
func normalizeForBrandMatch(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// containsWholePhrase: true bila phrase muncul di text sebagai rangkaian kata
// utuh (bukan potongan kata lain).
func containsWholePhrase(text, phrase string) bool {
	if phrase == "" {
		return false
	}
	re, err := regexp.Compile(`(^|\s)` + regexp.QuoteMeta(phrase) + `($|\s)`)
	if err != nil {
		return false
	}
	return re.MatchString(text)
}

// utf8RuneLen: jumlah karakter (bukan byte) supaya ambang minimal 4 karakter
// berlaku untuk nama berhuruf non-ASCII juga.
func utf8RuneLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
