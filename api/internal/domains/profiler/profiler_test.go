package profiler

import (
	"testing"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

func TestHTMLToText(t *testing.T) {
	html := `<html><head><title>Judul Halaman</title><style>body{color:red}</style></head>
		<body><script>alert(1)</script><h1>Halo</h1><p>Isi <b>tebal</b> &amp; penting.</p></body></html>`
	got := htmlToText(html)
	for _, want := range []string{"Judul Halaman", "Halo", "Isi", "tebal", "&", "penting."} {
		if !contains(got, want) {
			t.Errorf("htmlToText kehilangan %q; hasil=%q", want, got)
		}
	}
	for _, bad := range []string{"alert(1)", "color:red", "<script"} {
		if contains(got, bad) {
			t.Errorf("htmlToText harus membuang %q; hasil=%q", bad, got)
		}
	}
}

func TestDocMime(t *testing.T) {
	cases := map[string]string{
		"laporan.docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"catatan.md":   "text/markdown",
		"data.csv":     "text/csv",
		"note.txt":     "text/plain",
		"dok.pdf":      "application/pdf",
	}
	for name, want := range cases {
		if got := docMime(name, ""); got != want {
			t.Errorf("docMime(%q)=%q, mau %q", name, got, want)
		}
	}
}

func TestReadiness(t *testing.T) {
	if got := readiness([]byte("{}"), []byte("{}"), []byte("{}"), nil, 0, 0, 0, 0); got != "empty" {
		t.Errorf("profil kosong harus 'empty', dapat %q", got)
	}
	if got := readiness([]byte(`{"brand_name":"A"}`), []byte(`{"tone":"formal"}`), []byte("{}"), strPtr("jangan umum"), 1, 0, 0, 0); got != "ready" {
		t.Errorf("profil terisi harus 'ready', dapat %q", got)
	}
}

func TestExtractTextNativeFormats(t *testing.T) {
	got, err := ExtractText([]byte("baris satu\nbaris dua"), "text/plain")
	if err != nil || got != "baris satu\nbaris dua" {
		t.Fatalf("ExtractText text/plain gagal: %q %v", got, err)
	}
	if _, err := ExtractText([]byte("%PDF-1.7"), "application/pdf"); err == nil {
		t.Error("PDF harus mengembalikan error (belum didukung), bukan sukses")
	}
}

func TestRelevanceRanking(t *testing.T) {
	topic := "harga paket langganan software akuntansi untuk UMKM"
	if got := lexicalOverlap(topic, "Daftar harga paket langganan bulanan dan tahunan"); got < 0.3 {
		t.Errorf("teks relevan harus punya overlap memadai, dapat %v", got)
	}
	if got := lexicalOverlap(topic, "Sejarah perusahaan berdiri tahun 1998 di Surabaya"); got != 0 {
		t.Errorf("teks tak relevan harus 0, dapat %v", got)
	}
	// Kemiripan embedding lokal: topik vs teks relevan harus > teks tak relevan.
	vec := ai.LocalEmbedding(topic)
	rel := relevanceScore(topic, vec, "Paket langganan software akuntansi harganya terjangkau untuk UMKM")
	irr := relevanceScore(topic, vec, "Petunjuk memelihara tanaman hias di pekarangan rumah")
	if rel <= irr {
		t.Errorf("skor relevan (%v) harus > tak relevan (%v)", rel, irr)
	}
}

func strPtr(s string) *string { return &s }

func contains(hay, needle string) bool {
	return len(needle) == 0 || (len(hay) >= len(needle) && indexOf(hay, needle) >= 0)
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
