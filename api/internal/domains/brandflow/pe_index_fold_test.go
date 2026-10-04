package brandflow

import (
	"strings"
	"testing"
)

// peIndexFoldLower: referensi perilaku lama peIndexFold (berbasis ToLower).
func peIndexFoldLower(haystack, needle string) int {
	return strings.Index(strings.ToLower(haystack), strings.ToLower(needle))
}

// TestPEIndexFoldParityToLower: versi tanpa-allocasi harus mengembalikan indeks
// yang sama dengan versi ToLower untuk konten nyata. Kasus rune non-ASCII yang
// lowercase-nya satu karakter ASCII (U+0130, U+212A) sengaja tidak dimasukkan —
// di sana versi lama sendiri mengembalikan indeks yang melenceng dari string
// asli karena ToLower mengubah panjang byte (lihat komentar peIndexFold).
func TestPEIndexFoldParityToLower(t *testing.T) {
	haystacks := []string{
		"",
		"<img",
		"x<img",
		"<IMG src=x>",
		"abc <ImG src=y> def",
		"<p>tanpa gambar</p>",
		"<im",
		"<imx> <img>",
		"   <img",
		"emoji 🚀 <img>",
		"aksen é à ü <IMG> ü",
		"<im<im<imG>",
		"gmi<img>",
		"<p>awal</p>" + strings.Repeat("<p>x</p>", 200) + "<IMG src=z>" + strings.Repeat("<p>y</p>", 200),
		strings.Repeat("<img>\n", 50),
	}
	needles := []string{"<img", "IMG", "img", ""}
	for _, h := range haystacks {
		for _, n := range needles {
			want := peIndexFoldLower(h, n)
			got := peIndexFold(h, n)
			if got != want {
				t.Errorf("peIndexFold(%q, %q) = %d, referensi ToLower = %d", h, n, got, want)
			}
		}
	}
}

// TestPEIndexFoldInvalidUTF8: byte UTF-8 tak valid (dan rune seperti U+0130)
// melebar/menyempit saat ToLower, jadi versi lama mengembalikan indeks pada
// string hasil ToLower — bukan string asli — dan peTagEnd mulai memotong di
// tengah tag. Versi baru mengembalikan indeks pada string asli; ini bagian dari
// kelas deviasi yang didokumentasikan di peIndexFold.
func TestPEIndexFoldInvalidUTF8(t *testing.T) {
	h := "campur \xff <img> rusak"
	const want = 9 // posisi '<img' di string asli
	if got := peIndexFold(h, "<img"); got != want {
		t.Fatalf("peIndexFold = %d, mau %d (indeks string asli)", got, want)
	}
	if shifted := peIndexFoldLower(h, "<img"); shifted == want {
		t.Fatalf("asumsi salah: versi ToLower ternyata juga %d", shifted)
	}
}
