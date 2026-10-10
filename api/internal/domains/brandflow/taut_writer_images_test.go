package brandflow

import "testing"

// userRequestedInlineImages harus menghormati sanggahan: klien yang menulis
// "tanpa gambar" tidak boleh dikenai biaya gambar, tetapi permintaan campuran
// tetap terdeteksi.
func TestUserRequestedInlineImages(t *testing.T) {
	cases := []struct {
		name  string
		topic string
		want  bool
	}{
		{"negasi tanpa", "tanpa gambar", false},
		{"negasi jangan", "Jangan pakai gambar", false},
		{"negasi tidak perlu", "tidak perlu ilustrasi", false},
		{"negasi tanpa foto", "tanpa foto", false},
		{"negasi hindari", "Hindari gambar", false},
		{"negasi kapital", "Tanpa Gambar", false},
		{"negasi inggris", "no images please", false},
		{"negasi label", "Gambar: tidak ada", false},
		{"kata turunan", "menggambarkan situasi pasar", false},
		{"kosong", "", false},
		{"permintaan positif", "Tambahkan 1 gambar", true},
		{"permintaan jumlah", "Buat artikel 500 kata dan sertakan 2 gambar isi", true},
		{"permintaan infografis", "Sertakan infografis", true},
		{"campuran", "tanpa gambar utama, tapi tambahkan 2 gambar isi", true},
		{"kalimat pengguna", "Buat opini tentang Tauthub dan perkembangan distribusi konten digital. Minimal 600 kata, tanpa gambar", false},
	}
	for _, tc := range cases {
		if got := userRequestedInlineImages(tc.topic); got != tc.want {
			t.Errorf("%s: userRequestedInlineImages(%q) = %v, mau %v", tc.name, tc.topic, got, tc.want)
		}
	}
}
