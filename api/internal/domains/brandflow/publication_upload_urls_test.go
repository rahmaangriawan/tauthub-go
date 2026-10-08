package brandflow

import (
	"strings"
	"testing"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Gambar artikel dipasang di situs media sebagai tautan ke /uploads. Berkas itu
// wajib login, jadi tanpa tanda tangan publik gambarnya patah di Blogger.
func TestPeAbsolutizeUploadURLsMenandatanganiGambar(t *testing.T) {
	const secret = "rahasia-app"
	const origin = "https://app.tauthub.com"
	sig := security.SignUploadKey(secret, "f1da7a76.webp")

	got := peAbsolutizeUploadURLs(`<p><img src="/uploads/f1da7a76.webp" /></p>`, origin, secret)
	want := `<p><img src="` + origin + `/uploads/f1da7a76.webp?s=` + sig + `" /></p>`
	if got != want {
		t.Fatalf("dapat %q, mau %q", got, want)
	}

	// Tanda tangan harus stabil supaya URL artikel tidak berubah tiap terbit.
	if again := peAbsolutizeUploadURLs(got, origin, secret); strings.Count(again, "?s=") != 1 {
		t.Fatalf("tanda tangan ganda atau hilang: %q", again)
	}
}

func TestPeAbsolutizeUploadURLsTidakMenyentuhURLLuar(t *testing.T) {
	const secret = "rahasia-app"
	html := `<p><img src="https://cdn.lain.com/gambar.png" /><img src="/assets/lokal.png" /></p>`
	if got := peAbsolutizeUploadURLs(html, "https://app.tauthub.com", secret); got != html {
		t.Fatalf("URL non-uploads berubah: %q", got)
	}
	if got := peAbsolutizeUploadURLs(html, "", secret); got != html {
		t.Fatalf("tanpa origin tidak boleh berubah: %q", got)
	}
}

func TestPeAbsolutizeUploadURLsKutipTunggal(t *testing.T) {
	const secret = "rahasia-app"
	got := peAbsolutizeUploadURLs(`<img src='/uploads/a.webp'>`, "https://app.tauthub.com", secret)
	want := `<img src='https://app.tauthub.com/uploads/a.webp?s=` + security.SignUploadKey(secret, "a.webp") + `'>`
	if got != want {
		t.Fatalf("dapat %q, mau %q", got, want)
	}
}
