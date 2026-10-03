package brandflow

import "testing"

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
