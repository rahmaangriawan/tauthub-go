package brandflow

import (
	"strings"
	"testing"
)

// Media mainstream yang DIIZINKAN subjudul -> heading dipertahankan.
func TestMainstreamAllowedKeepsHeadings(t *testing.T) {
	in := "<p>Lead berita yang mengalir.</p><h2>Mandat Penyehatan Bisnis</h2><p>Isi bagian.</p>"
	got := finalizeTautArticleHTML(in, "mainstream", false, nil, "", true)
	if !strings.Contains(got, "<h2>Mandat Penyehatan Bisnis</h2>") {
		t.Fatalf("subjudul hilang padahal diizinkan: %s", got)
	}
}

// Media mainstream yang TIDAK diizinkan -> heading jadi paragraf (perilaku lama).
func TestMainstreamDeniedDemotesHeadings(t *testing.T) {
	in := "<p>Lead berita.</p><h2>Subjudul</h2><p>Isi.</p>"
	got := finalizeTautArticleHTML(in, "mainstream", false, nil, "", false)
	if strings.Contains(got, "<h2") {
		t.Fatalf("heading masih ada padahal dilarang: %s", got)
	}
	if !strings.Contains(got, "<p>Subjudul</p>") {
		t.Fatalf("subjudul tidak jadi paragraf: %s", got)
	}
}

// Default (tanpa opsi bool) mengikuti tipe media: mainstream demote.
func TestMainstreamDefaultDemotes(t *testing.T) {
	in := "<p>Lead.</p><h2>Sub</h2><p>Isi.</p>"
	got := finalizeTautArticleHTML(in, "mainstream", false, nil)
	if strings.Contains(got, "<h2") {
		t.Fatalf("default mainstream harus demote: %s", got)
	}
}

// Blogger yang dilarang -> heading jadi paragraf.
func TestBloggerDeniedDemotes(t *testing.T) {
	in := "<p>Lead.</p><h2>Sub</h2><p>Isi.</p>"
	got := finalizeTautArticleHTML(in, "blogger", false, nil, "", false)
	if strings.Contains(got, "<h2") {
		t.Fatalf("blogger yang dilarang masih ber-heading: %s", got)
	}
}

// Pembuka heading tetap ditangani meski subjudul diizinkan.
func TestAllowedStillFixesLeadingHeading(t *testing.T) {
	title := "Judul Artikel"
	in := "<h2>" + title + "</h2><p>Lead.</p><h2>Sub Asli</h2><p>Isi.</p>"
	got := finalizeTautArticleHTML(in, "mainstream", false, nil, title, true)
	if strings.Contains(got, title+"</h2>") {
		t.Fatalf("heading pembuka mengulang judul: %s", got)
	}
	if !strings.Contains(got, "<h2>Sub Asli</h2>") {
		t.Fatalf("subjudul asli hilang: %s", got)
	}
}
