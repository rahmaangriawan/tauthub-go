package brandflow

import (
	"strings"
	"testing"
)

// Regresi: judul artikel hidup di field judul terpisah, jadi isi hasil cleanup
// dokumen TIDAK boleh membawa <h1> — hasil model yang memakai H1 diturunkan
// jadi H2 (atribut ikut dibuang).
func TestNormalizeCleanupHTMLDemotesH1(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "h1 dengan atribut",
			in:   `<h1 class="judul">Panduan Epoxy</h1><p>Isi pertama.</p>`,
			want: `<h2>Panduan Epoxy</h2><p>Isi pertama.</p>`,
		},
		{
			name: "h1 dalam pagar kode markdown",
			in:   "```html\n<h1>Panduan Epoxy</h1><p>Isi pertama.</p>\n```",
			want: `<h2>Panduan Epoxy</h2><p>Isi pertama.</p>`,
		},
		{
			name: "tanpa h1 tidak berubah",
			in:   `<h2>Subjudul Pertama</h2><p>Isi paragraf pembuka yang cukup panjang untuk lolos ambang panjang.</p>`,
			want: `<h2>Subjudul Pertama</h2><p>Isi paragraf pembuka yang cukup panjang untuk lolos ambang panjang.</p>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeCleanupHTML(tc.in)
			if !ok {
				t.Fatalf("normalizeCleanupHTML harus ok untuk input: %s", tc.in)
			}
			if got != tc.want {
				t.Fatalf("H1 tidak diturunkan dengan benar:\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// Prompt cleanup tidak boleh lagi memerintahkan model membuat <h1>.
func TestDocCleanupPromptForbidsH1(t *testing.T) {
	prompt := buildDocCleanupPrompt("Judul Dokumen", "Isi naskah.")
	if strings.Contains(prompt, "H1 untuk judul") {
		t.Fatalf("prompt masih menunjuk H1 sebagai judul:\n%s", prompt)
	}
	if !strings.Contains(prompt, "JANGAN membuat <h1>") {
		t.Fatalf("prompt harus melarang <h1> secara eksplisit:\n%s", prompt)
	}
}
