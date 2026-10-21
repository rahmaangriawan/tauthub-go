package brandflow

import "testing"

// matchAllowedCategory: jawaban model harus dipetakan ke kategori media yang
// diizinkan mitra; yang di luar daftar (atau kosong, mis. model ragu / output
// bukan JSON) jatuh ke kategori pertama — persis aturan "kalau AI bingung pakai
// yang paling atas".
func TestMatchAllowedCategory(t *testing.T) {
	allowed := []string{"BISNIS", "Ekonomi", "Teknologi"}

	cases := []struct {
		name   string
		answer string
		list   []string
		want   string
	}{
		{"cocok persis", "Ekonomi", allowed, "Ekonomi"},
		{"beda besar-kecil huruf", "teknologi", allowed, "Teknologi"},
		{"berspasi dan berkutip", `  "BISNIS"  `, allowed, "BISNIS"},
		{"di luar daftar → kategori pertama", "Olahraga", allowed, "BISNIS"},
		{"kosong (model ragu) → kategori pertama", "", allowed, "BISNIS"},
		{"hanya spasi → kategori pertama", "   ", allowed, "BISNIS"},
		{"daftar kosong → tidak mengarang rubrik", "Ekonomi", nil, ""},
		{"daftar satu item → item itu", "apa saja", []string{"Umum"}, "Umum"},
	}

	for _, c := range cases {
		if got := matchAllowedCategory(c.answer, c.list); got != c.want {
			t.Errorf("%s: matchAllowedCategory(%q, %v) = %q, mau %q", c.name, c.answer, c.list, got, c.want)
		}
	}
}
