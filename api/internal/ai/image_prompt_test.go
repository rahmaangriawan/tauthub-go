package ai

import (
	"strings"
	"testing"
)

// imagePromptWithRoleRules: aturan teks per role harus selalu ikut terkirim ke
// provider gambar, karena provider cenderung menuliskan prompt ke dalam gambar.
func TestImagePromptWithRoleRules(t *testing.T) {
	base := "Epoxy lantai untuk bangunan komersial"

	cover := imagePromptWithRoleRules(base, "cover")
	if !strings.HasPrefix(cover, base) {
		t.Errorf("prompt cover harus dimulai dari prompt asli, dapat %q", cover)
	}
	if !strings.Contains(cover, "NO text") {
		t.Errorf("prompt cover harus melarang teks, dapat %q", cover)
	}

	inline := imagePromptWithRoleRules(base, "inline")
	if !strings.Contains(inline, "MINIMAL text") {
		t.Errorf("prompt inline harus meminta teks minimal, dapat %q", inline)
	}
	if !strings.Contains(inline, "data visualization") {
		t.Errorf("prompt inline harus mengizinkan teks hanya untuk visualisasi data, dapat %q", inline)
	}

	// Role tak dikenal diperlakukan sebagai inline (sama seperti handler).
	if !strings.Contains(imagePromptWithRoleRules(base, ""), "MINIMAL text") {
		t.Error("role kosong harus memakai aturan inline")
	}

	// Prompt kosong tetap punya subjek default yang wajar, bukan kalimat kosong.
	kosong := imagePromptWithRoleRules("   ", "cover")
	if !strings.HasPrefix(kosong, "Editorial illustration") || !strings.Contains(kosong, "NO text") {
		t.Errorf("prompt kosong harus memakai subjek default + aturan role, dapat %q", kosong)
	}
}
