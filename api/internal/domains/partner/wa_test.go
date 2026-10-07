package partner

import "testing"

// TestWAExceededBy: aturan nomor WhatsApp. Media PERTAMA harus tetap bebas
// (itu pintu masuk mitra baru), dan setiap penambahan setelahnya terkena
// kewajiban kontak. Ini menjaga ambang tidak diam-diam kembali ke 2.
func TestWAExceededBy(t *testing.T) {
	if waMediaThreshold != 1 {
		t.Fatalf("waMediaThreshold = %d, mau 1 (media ke-2 dst wajib WA)", waMediaThreshold)
	}
	cases := []struct {
		name            string
		existing, addin int
		want            bool
	}{
		{"media pertama bebas", 0, 1, false},
		{"media kedua wajib WA", 1, 1, true},
		{"media ketiga wajib WA", 2, 1, true},
		{"mitra kosong impor dua baris sekaligus", 0, 2, true},
		{"hanya membaca tanpa menambah", 1, 0, false},
		{"mitra kosong tanpa penambahan", 0, 0, false},
	}
	for _, tc := range cases {
		if got := waExceededBy(tc.existing, tc.addin); got != tc.want {
			t.Errorf("%s: waExceededBy(%d, %d) = %v, mau %v", tc.name, tc.existing, tc.addin, got, tc.want)
		}
	}
}
