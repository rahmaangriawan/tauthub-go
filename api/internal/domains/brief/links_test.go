package brief

import (
	"strings"
	"testing"
)

func TestExtractURLs(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    []string
	}{
		{
			name:    "tautan di akhir kalimat dengan titik",
			message: "Ini website brand kami https://contoh.id.",
			want:    []string{"https://contoh.id"},
		},
		{
			name:    "tautan dalam tanda kurung",
			message: "Coba baca (https://contoh.id/tentang-kami) ya",
			want:    []string{"https://contoh.id/tentang-kami"},
		},
		{
			name:    "duplikat hanya dihitung sekali",
			message: "https://a.id lalu https://a.id lagi",
			want:    []string{"https://a.id"},
		},
		{
			name:    "maksimal dua tautan per pesan",
			message: "https://a.id https://b.id https://c.id",
			want:    []string{"https://a.id", "https://b.id"},
		},
		{
			name:    "tanpa tautan",
			message: "nama brand kami rabiyuk",
			want:    []string{},
		},
		{
			name:    "skema selain http diabaikan",
			message: "ftp://contoh.id bukan http",
			want:    []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractURLs(tc.message)
			if len(got) != len(tc.want) {
				t.Fatalf("dapat %v, mau %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("dapat %v, mau %v", got, tc.want)
				}
			}
		})
	}
}

func TestLinkHTMLToText(t *testing.T) {
	html := `<html><head><title>Contoh Brand — Tentang</title>
		<style>body{color:red}</style><script>alert(1)</script></head>
		<body><h1>Contoh Brand</h1><p>Kami &amp; tim membantu UMKM.</p></body></html>`
	text := linkHTMLToText(html)
	if !strings.Contains(text, "Contoh Brand") {
		t.Fatalf("teks %q tidak memuat nama brand", text)
	}
	if !strings.Contains(text, "Kami & tim membantu UMKM.") {
		t.Fatalf("entitas HTML harus didekode, dapat %q", text)
	}
	if strings.Contains(text, "alert(1)") || strings.Contains(text, "color:red") {
		t.Fatalf("script/style harus dibuang, dapat %q", text)
	}
	if got := linkFirstTitle(html); got != "Contoh Brand — Tentang" {
		t.Fatalf("judul %q", got)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("abcdef", 3); got != "abc" {
		t.Fatalf("dapat %q", got)
	}
	// Karakter multibyte tidak boleh terbelah.
	if got := truncateRunes("ragam—brand", 5); got != "ragam" {
		t.Fatalf("dapat %q", got)
	}
	if got := truncateRunes("pendek", 99); got != "pendek" {
		t.Fatalf("dapat %q", got)
	}
}
