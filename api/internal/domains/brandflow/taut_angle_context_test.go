package brandflow

import (
	"strings"
	"testing"
)

// Uji murni (tanpa DB/LLM): pemilihan subagent writer per tipe media dan
// perangkaian blok "ANGLE BERITA" sebagai sumber informasi.

func TestWriterAgentName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"mainstream", "writer_mainstream"},
		{"blogger", "writer_blogger"},
		{"Blogger", "writer_blogger"},
		{"  blogger  ", "writer_blogger"},
		{"", "writer_mainstream"},
		{"kombinasi", "writer_mainstream"},
	}
	for _, c := range cases {
		if got := writerAgentName(c.in); got != c.want {
			t.Fatalf("writerAgentName(%q) = %q, ingin %q", c.in, got, c.want)
		}
	}
}

func TestTautAngleBriefBlock(t *testing.T) {
	if block, has := tautAngleBriefBlock(nil); block != "" || has {
		t.Fatal("angle nil harus blok kosong tanpa sumber")
	}

	// Tanpa Sumber Informasi: blok tetap memuat nama+deskripsi+prompt gaya, tapi
	// TIDAK dihitung sebagai bahan fakta (model tidak boleh mengarang).
	noSources := &tautAngleContext{
		Channel: "blogger", Slug: "listicle", Name: "Listicle",
		Description: "Artikel daftar.", Prompt: "Tulis daftar berpoin.",
	}
	block, has := tautAngleBriefBlock(noSources)
	if has {
		t.Fatal("angle tanpa source_notes tidak boleh dihitung bahan fakta")
	}
	for _, want := range []string{"[ANGLE BERITA — blogger/Listicle]", "Artikel daftar.", "Tulis daftar berpoin."} {
		if !strings.Contains(block, want) {
			t.Fatalf("blok angle kehilangan %q:\n%s", want, block)
		}
	}

	// Dengan Sumber Informasi: hasSources=true dan isinya masuk blok.
	withSources := &tautAngleContext{
		Channel: "mainstream", Slug: "product_launch", Name: "Peluncuran Produk",
		SourceNotes: "Fakta: PT Contoh meluncurkan produk X pada 2026.",
	}
	block, has = tautAngleBriefBlock(withSources)
	if !has {
		t.Fatal("angle dengan source_notes harus hasSources=true")
	}
	if !strings.Contains(block, "PT Contoh meluncurkan produk X") {
		t.Fatalf("sumber informasi tidak masuk blok:\n%s", block)
	}
}

func TestReferenceMaterialIncludesAngleBlock(t *testing.T) {
	angle := "[ANGLE BERITA — blogger/Listicle]\nSumber informasi angle (bahan fakta dari admin):\nFakta dari angle."
	text, has := buildTautReferenceMaterial("", map[string]any{}, "", nil, angle)
	if !has {
		t.Fatal("blok angle harus dihitung sebagai bahan rujukan")
	}
	if !strings.Contains(text, "[ANGLE BERITA (sumber informasi)]") || !strings.Contains(text, "Fakta dari angle.") {
		t.Fatalf("bahan rujukan tidak memuat section angle:\n%s", text)
	}
}
