package platform

import "strings"

// phoneSearchTerms menyiapkan variasi angka dari kata kunci pencarian supaya
// admin bisa mencari partner/klien lewat sebagian nomor WhatsApp — misalnya
// "2922" untuk nomor yang berakhiran 2922.
//
// Nomor disimpan bebas format ("+62 812-2922", "0812 2922", "62812..."), jadi
// pencocokan dilakukan pada bentuk digitnya saja (regexp_replace di SQL).
// Awalan 0 dan 62 sama-sama dicoba: nomor Indonesia sering ditulis dua cara.
//
// Kurang dari 3 digit diabaikan — "29" akan mencocokkan hampir semua nomor.
func phoneSearchTerms(search string) []string {
	var digits strings.Builder
	for _, r := range search {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if len(d) < 3 {
		return nil
	}
	terms := []string{d}
	switch {
	case strings.HasPrefix(d, "0"):
		terms = append(terms, "62"+d[1:])
	case strings.HasPrefix(d, "62"):
		terms = append(terms, "0"+d[2:])
	}
	return terms
}
