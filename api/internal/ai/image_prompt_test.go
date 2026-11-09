package ai

import (
	"strings"
	"testing"
)

// imagePromptWithRoleRules: aturan teks per role harus selalu ikut terkirim ke
// provider gambar, karena provider cenderung menuliskan seluruh prompt ke dalam
// gambar. Kebijakan: satu teks pendek yang penting saja, maksimal 7 kata.
func TestImagePromptWithRoleRules(t *testing.T) {
	base := "Epoxy lantai untuk bangunan komersial"

	cover := imagePromptWithRoleRules(base, "cover")
	if !strings.HasPrefix(cover, base) {
		t.Errorf("prompt cover harus dimulai dari prompt asli, dapat %q", cover)
	}
	for _, want := range []string{"at most ONE short text element", "maximum 7 words", "keyword, title, or subtitle", "no paragraphs"} {
		if !strings.Contains(cover, want) {
			t.Errorf("prompt cover harus memuat %q, dapat %q", want, cover)
		}
	}
	if strings.Contains(cover, "data visualization") {
		t.Errorf("cover tidak perlu pengecualian grafik, dapat %q", cover)
	}

	inline := imagePromptWithRoleRules(base, "inline")
	if !strings.Contains(inline, "maximum 7 words") {
		t.Errorf("prompt inline harus membatasi 7 kata, dapat %q", inline)
	}
	if !strings.Contains(inline, "data visualization") {
		t.Errorf("prompt inline harus mengizinkan angka/label untuk visualisasi data, dapat %q", inline)
	}

	// Role tak dikenal diperlakukan sebagai inline (sama seperti handler).
	if !strings.Contains(imagePromptWithRoleRules(base, ""), "data visualization") {
		t.Error("role kosong harus memakai aturan inline")
	}

	// Prompt kosong tetap punya subjek default yang wajar, bukan kalimat kosong.
	kosong := imagePromptWithRoleRules("   ", "cover")
	if !strings.HasPrefix(kosong, "Editorial illustration") || !strings.Contains(kosong, "maximum 7 words") {
		t.Errorf("prompt kosong harus memakai subjek default + aturan role, dapat %q", kosong)
	}
}
