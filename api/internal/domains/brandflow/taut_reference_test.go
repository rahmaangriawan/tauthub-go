package brandflow

import (
	"strings"
	"testing"
)

func TestBuildTautReferenceMaterial(t *testing.T) {
	materials := map[string]any{
		"brief":        "PT Contoh bergerak di bidang epoxy lantai.",
		"key_messages": []any{"harga transparan"},
	}
	spokes := []spokespersonValue{{Name: "Budi Santoso", Role: "Direktur", Quote: "Kualitas nomor satu."}}
	text, has := buildTautReferenceMaterial("Ringkasan brief Profiler.", materials, "Topik dari klien: renovasi gudang.", spokes, "")
	if !has {
		t.Fatal("bahan lengkap harus hasFacts=true")
	}
	for _, want := range []string{
		"[BRIEF PROFILER]",
		"[MATERI DARI KLIEN]",
		"[KONTEKS/TOPIK DARI KLIEN]",
		"[NARASUMBER (kutipan resmi)]",
		"Budi Santoso",
		"PT Contoh",
		"renovasi gudang",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("bahan kehilangan %q:\n%s", want, text)
		}
	}

	// Semua sumber kosong -> tidak ada fakta (Writer wajib generik, Checker
	// memperlakukan bahan kosong).
	empty := map[string]any{"brief": "  ", "key_messages": []any{}, "references": []any{}}
	if _, has := buildTautReferenceMaterial("", empty, "   ", nil, ""); has {
		t.Fatal("bahan kosong harus hasFacts=false")
	}
}

func TestMaterialHasContent(t *testing.T) {
	if materialHasContent(nil) || materialHasContent(map[string]any{}) {
		t.Fatal("map nil/kosong bukan bahan")
	}
	if materialHasContent(map[string]any{"brief": " ", "key_messages": []any{""}}) {
		t.Fatal("nilai kosong bukan bahan")
	}
	if !materialHasContent(map[string]any{"references": []any{"https://contoh.test"}}) {
		t.Fatal("referensi non-kosong harus dianggap bahan")
	}
	if !materialHasContent(map[string]any{"assets": []string{"foto.jpg"}}) {
		t.Fatal("assets []string non-kosong harus dianggap bahan")
	}
}
