package brandflow

import "testing"

// Editorial zero-link: URL pada teks dibuat polos tanpa skema supaya tidak
// terasa seperti tautan (instruksi link dari klien tidak boleh mengenaskan).
func TestStripURLSchemeText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"https", "Kunjungi https://contoh.com/produk untuk detail", "Kunjungi contoh.com/produk untuk detail"},
		{"http", "Sumber: http://tauthub.com", "Sumber: tauthub.com"},
		{"dua url", "https://a.id dan http://b.id", "a.id dan b.id"},
		{"dengan query", "https://Contoh.com/Path?a=1&b=2 ya", "Contoh.com/Path?a=1&b=2 ya"},
		{"tanpa url", "Artikel tentang distribusi konten", "Artikel tentang distribusi konten"},
		{"kosong", "", ""},
	}
	for _, tc := range cases {
		if got := stripURLSchemeText(tc.in); got != tc.want {
			t.Errorf("%s: stripURLSchemeText(%q) = %q, mau %q", tc.name, tc.in, got, tc.want)
		}
	}
}
