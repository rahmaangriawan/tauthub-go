package brandflow

import "testing"

// Pencocokan domain milik sendiri harus tahan terhadap variasi URL yang lazim
// (www, subdomain, path, port) TAPI tidak boleh tertipu domain palsu yang
// menempelkan nama domain kita sebagai subdomainnya.
func TestHostOwned(t *testing.T) {
	own := map[string]bool{"rabiyuk.com": true, "detik.com": true}
	cases := []struct {
		url  string
		want bool
	}{
		{"https://rabiyuk.com", true},
		{"http://rabiyuk.com/", true},
		{"https://www.rabiyuk.com", true},
		{"https://rabiyuk.com/promo?x=1", true},
		{"http://blog.rabiyuk.com/artikel", true},
		{"https://detik.com/berita/1", true},
		{"https://rabiyuk.com.evil.com", false},
		{"https://notrabiyuk.com", false},
		{"https://rabiyuk-com.id", false},
		{"https://contoh.id", false},
		{"", false},
		{"bukan-url", false},
	}
	for _, tc := range cases {
		if got := hostOwned(tc.url, own); got != tc.want {
			t.Fatalf("hostOwned(%q) = %v, mau %v", tc.url, got, tc.want)
		}
	}

	if hostOwned("https://apa-saja.com", map[string]bool{}) {
		t.Fatal("daftar kosong tidak boleh menganggap apa pun milik sendiri")
	}
}
