package platform

import "testing"

// TestOpPostIDFromWPURL: aksi "Sinkron ulang gambar & kategori" hanya boleh
// memakai ID post yang benar-benar tertulis di tautan. Permalink cantik dan
// angka yang tidak wajar harus ditolak supaya aksi itu tidak menebak post mana
// yang akan diperbarui.
func TestOpPostIDFromWPURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"tautan p biasa", "https://asiaberita.com/?p=152", "152"},
		{"p di tengah query", "https://contoh.id/index.php?a=1&p=77&b=2", "77"},
		{"spasi di ujung", "  https://contoh.id/?p=9  ", "9"},
		{"permalink cantik", "https://contoh.id/judul-artikel/", ""},
		{"query lain tanpa p", "https://contoh.id/?page_id=12", ""},
		{"p bukan angka", "https://contoh.id/?p=abc", ""},
		{"p negatif", "https://contoh.id/?p=-3", ""},
		{"p pecahan", "https://contoh.id/?p=1.5", ""},
		{"p kosong", "https://contoh.id/?p=", ""},
		{"string kosong", "", ""},
		{"bukan url", "bukan url sama sekali", ""},
	}
	for _, tc := range cases {
		if got := opPostIDFromWPURL(tc.in); got != tc.want {
			t.Errorf("%s: opPostIDFromWPURL(%q) = %q, mau %q", tc.name, tc.in, got, tc.want)
		}
	}
}
