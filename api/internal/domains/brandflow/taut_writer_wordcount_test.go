package brandflow

import "testing"

// requestedWordCount: ambil target jumlah kata dari instruksi klien, dan
// ABAIKAN angka yang jelas pembatas atas ("maksimal 2500 kata").
func TestRequestedWordCount(t *testing.T) {
	cases := []struct {
		name  string
		topic string
		want  int
		ok    bool
	}{
		{"minimal", "Buat opini. Minimal 2500 kata.", 2500, true},
		{"polos", "Artikel 2500 kata tentang digitalisasi", 2500, true},
		{"kalimat panjang", "Buat opini tentang Tauthub dan perkembangan distribusi konten digital. Minimal 600 kata, tanpa gambar", 600, true},
		{"sekitar", "sekitar 1200 kata", 1200, true},
		{"inggris", "write 2500 words", 2500, true},
		{"batas atas maksimal", "maksimal 2500 kata", 0, false},
		{"batas atas maks", "maks 1500 kata", 0, false},
		{"batas atas tidak lebih dari", "tidak lebih dari 2000 kata", 0, false},
		{"terlalu kecil", "100 kata", 0, false},
		{"tanpa angka", "tanpa batas kata", 0, false},
		{"kosong", "", 0, false},
		{"ambil terbesar", "minimal 800 kata, idealnya 1500 kata", 1500, true},
	}
	for _, tc := range cases {
		got, ok := requestedWordCount(tc.topic)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: requestedWordCount(%q) = (%d,%v), mau (%d,%v)", tc.name, tc.topic, got, ok, tc.want, tc.ok)
		}
	}
}
