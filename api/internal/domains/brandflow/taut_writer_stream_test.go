package brandflow

import (
	"strings"
	"testing"
)

func TestPartialContentHTMLPlain(t *testing.T) {
	// Key belum muncul (model masih menulis title).
	if v, closed := partialContentHTML(`{"title": "Epoxy`); v != "" || closed {
		t.Errorf("sebelum key: dapat %q closed=%v", v, closed)
	}
	// Key muncul, isi parsial tanpa escape.
	acc := `{"title": "Epoxy", "content_html": "<p>Halo`
	v, closed := partialContentHTML(acc)
	if v != "<p>Halo" || closed {
		t.Errorf("parsial: dapat %q closed=%v", v, closed)
	}
	// Isi lengkap dengan kutip penutup.
	acc += ` dunia</p>", "meta_description": "x"}`
	v, closed = partialContentHTML(acc)
	if v != "<p>Halo dunia</p>" || !closed {
		t.Errorf("lengkap: dapat %q closed=%v", v, closed)
	}
}

func TestPartialContentHTMLEscapes(t *testing.T) {
	acc := `{"content_html": "<p>Baris satu\nBaris \"dua\" \\ok</p>", "meta_description": "m"}`
	v, closed := partialContentHTML(acc)
	want := "<p>Baris satu\nBaris \"dua\" \\ok</p>"
	if v != want || !closed {
		t.Errorf("escape: dapat %q closed=%v, mau %q", v, closed, want)
	}
	// Escape \n lengkap di ujung tetap di-decode (bukan terpotong).
	partial := `{"content_html": "<p>A\n`
	v, closed = partialContentHTML(partial)
	if v != "<p>A\n" || closed {
		t.Errorf("\\n di ujung: dapat %q closed=%v", v, closed)
	}
	// Escape benar-benar terpotong (backslash jadi karakter terakhir).
	partial = `{"content_html": "<p>A\`
	v, closed = partialContentHTML(partial)
	if v != "<p>A" || closed {
		t.Errorf("escape terpotong: dapat %q closed=%v", v, closed)
	}
	// \uXXXX terpotong.
	partial = `{"content_html": "<p>e\u0`
	v, _ = partialContentHTML(partial)
	if v != "<p>e" {
		t.Errorf("\\u terpotong: dapat %q", v)
	}
	// \uXXXX lengkap (kutip sebagai contoh unicode).
	partial = `{"content_html": "<p>e\u0026amp;</p>"}`
	v, closed = partialContentHTML(partial)
	if v != "<p>e&amp;</p>" || !closed {
		t.Errorf("\\u lengkap: dapat %q closed=%v", v, closed)
	}
}

func TestPartialContentHTMLKeyOrder(t *testing.T) {
	// Urutan key berbeda (content_html di akhir) tetap terdeteksi.
	acc := `{"meta_description": "abc", "content_html": "<p>X`
	v, closed := partialContentHTML(acc)
	if v != "<p>X" || closed {
		t.Errorf("urutan key: dapat %q closed=%v", v, closed)
	}
}

// TestPartialContentHTMLStreamParity: peStreamHTML harus menghasilkan akumulasi
// yang identik dengan partialContentHTML pada SETIAP prefix, apa pun ukuran
// delta-nya — termasuk escape yang terbelah di tengah delta.
func TestPartialContentHTMLStreamParity(t *testing.T) {
	cases := []string{
		`{"title": "Epoxy`,
		`{"title": "Epoxy", "content_html": "<p>Halo`,
		`{"title": "Epoxy", "content_html": "<p>Halo dunia</p>", "meta_description": "x"}`,
		`{"content_html": "<p>escape \n \t \\ \" selesai \u00e9 invalid \uZ</p>"}`,
		`{"a":1,"content_html":"<p>potong \u0`,
		`{"content_html" :   "<p>spasi dan titik dua</p>"}`,
		`tidak ada key sama sekali`,
		`{"content_html" :  <p>tanpa quote`,
		`{"content_html"`,
		`{"content_html":`,
		`{"content_html":"`,
		``,
	}
	for _, acc := range cases {
		for _, chunk := range []int{1, 2, 3, 5, 17, 4096} {
			var got strings.Builder
			stream := newPEStreamHTML()
			start := 0
			for start <= len(acc) {
				stop := start + chunk
				if stop > len(acc) {
					stop = len(acc)
				}
				tail, closed := stream.Write(acc[start:stop])
				prefix := acc[:stop]
				ref, refClosed := partialContentHTML(prefix)
				got.WriteString(tail)
				if got.String() != ref {
					t.Fatalf("acc=%q chunk=%d stop=%d: stream=%q mau %q", acc, chunk, stop, got.String(), ref)
				}
				if closed != refClosed {
					t.Fatalf("acc=%q chunk=%d stop=%d: closed=%v mau %v", acc, chunk, stop, closed, refClosed)
				}
				if stop == start {
					break
				}
				start = stop
			}
		}
	}
}
