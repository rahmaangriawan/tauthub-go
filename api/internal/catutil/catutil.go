// Package catutil menormalkan daftar kategori media.
//
// Mitra sering mengisi beberapa kategori dalam satu kolom dengan pemisah koma
// (mis. "News, Ekonomi, Lifestyle"). Tanpa pemisahan, nilai itu tersimpan
// sebagai SATU kategori panjang sehingga filter kategori tidak pernah cocok
// dan chip kategori di UI menampilkan satu label gabungan.
//
// Helper di sini dipakai baik saat MENULIS (memisah sebelum disimpan ke
// media_categories/media_category_links) maupun saat MEMBACA data lama yang
// masih berisi string gabungan di media.category.
package catutil

import (
	"strings"
	"unicode"
)

// separatorInternal adalah pemisah yang dipakai string_agg di SQL untuk
// menggabungkan beberapa nama kategori dari tabel link.
const separatorInternal = "\u0001"

// Split memisah satu string menjadi daftar kategori: memotong pada koma dan
// pemisah internal, memangkas spasi, membuang nilai kosong, serta
// men-deduplikasi tanpa peduli besar-kecil huruf (kemunculan pertama menang).
func Split(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\u0001'
	})
	return splitParts(parts)
}

// SplitAll menerapkan Split ke setiap nilai lalu menggabungkannya, sehingga
// campuran nilai tunggal dan nilai bergabung koma tetap menghasilkan daftar
// yang bersih dan bebas duplikat.
func SplitAll(values []string) []string {
	var parts []string
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if strings.ContainsAny(v, ",\u0001") {
			parts = append(parts, strings.FieldsFunc(v, func(r rune) bool {
				return r == ',' || r == '\u0001'
			})...)
			continue
		}
		parts = append(parts, v)
	}
	return splitParts(parts)
}

// JoinInternal menyusun ulang daftar menjadi satu string dengan pemisah
// internal, dipakai saat perlu membandingkan dengan hasil string_agg.
func JoinInternal(values []string) string {
	return strings.Join(SplitAll(values), separatorInternal)
}

// Canonical merapikan nama kategori agar konsisten di seluruh platform:
// setiap kata dimulai huruf kapital, sisanya huruf kecil ("berita" → "Berita").
// Singkatan yang sudah ditulis kapital semua ("SEO", "PR") dipertahankan, dan
// kata yang mengandung angka/tanda khusus ("Kpop 2026", "Food & Beverage")
// dibiarkan apa adanya supaya merek tidak rusak. Dipakai saat MENULIS.
func Canonical(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	words := strings.Fields(name)
	for i, w := range words {
		if w == "&" || strings.ToUpper(w) == w {
			continue
		}
		if !hasLetterOrDigit(w) || !isSimpleWord(w) {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " ")
}

func hasLetterOrDigit(w string) bool {
	for _, r := range w {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func isSimpleWord(w string) bool {
	for _, r := range w {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func splitParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		name := Canonical(p)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
