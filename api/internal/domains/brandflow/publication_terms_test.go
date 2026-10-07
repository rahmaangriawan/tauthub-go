package brandflow

import (
	"reflect"
	"testing"
)

// Regresi: snapshot job masih memakai tipe Go asli (int) selama percobaan
// pertama dan baru menjadi float64 setelah dibaca ulang dari jsonb. Dulu hanya
// float64 yang diterima, sehingga percobaan pertama mengirim artikel ke
// WordPress tanpa kategori (jatuh ke Uncategorized) dan tanpa gambar utama,
// padahal payload_snapshot di DB menampilkan nilai yang sudah ter-resolve.
func TestBsIntIDsMenerimaIntDanFloat(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  []int
	}{
		{"int dari memori", []any{4, 7}, []int{4, 7}},
		{"float64 dari jsonb", []any{float64(4), float64(7)}, []int{4, 7}},
		{"campur int dan float64", []any{4, float64(7)}, []int{4, 7}},
		{"nama kategori dilewati", []any{"Nasional"}, []int{}},
		{"nol dilewati", []any{0, 4}, []int{4}},
		{"negatif dilewati", []any{-3}, []int{}},
		{"pecahan dilewati", []any{4.5}, []int{}},
		{"bukan array", "Nasional", []int{}},
		{"nil", nil, []int{}},
	}
	for _, tc := range cases {
		if got := bsIntIDs(tc.value); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: bsIntIDs(%#v) = %v, mau %v", tc.name, tc.value, got, tc.want)
		}
	}
}

// featured_media ditulis sebagai int saat upload berhasil di percobaan yang
// sama; konversi harus tetap membacanya.
func TestPeNumFromAnyMembacaIntMediaID(t *testing.T) {
	if got := peNumFromAny(151); got != 151 {
		t.Fatalf("peNumFromAny(int 151) = %v, mau 151", got)
	}
	if got := peNumFromAny(float64(151)); got != 151 {
		t.Fatalf("peNumFromAny(float64 151) = %v, mau 151", got)
	}
}

// Payload ke WordPress harus membawa kategori + gambar utama pada percobaan
// PERTAMA (nilai masih int di memori), bukan hanya setelah retry (float64).
func TestBsWordPressPayloadMembawaKategoriDanGambar(t *testing.T) {
	document := bsRow{"title": "Pemerintah Dorong Digitalisasi Usaha Kecil", "meta_description": ""}
	cases := []struct {
		name     string
		resolved map[string]any
	}{
		{"tipe Go asli (percobaan pertama)", map[string]any{"categories": []any{4}, "featured_media": 151}},
		{"tipe jsonb (setelah retry)", map[string]any{"categories": []any{float64(4)}, "featured_media": float64(151)}},
	}
	for _, tc := range cases {
		snapshot := map[string]any{"publish_status": "draft", "resolved": tc.resolved}
		payload := bsWordPressPayload(snapshot, document, "pemerintah-dorong-digitalisasi", "<p>isi</p>")
		if got, _ := payload["categories"].([]int); !reflect.DeepEqual(got, []int{4}) {
			t.Fatalf("%s: categories = %#v, mau [4]", tc.name, payload["categories"])
		}
		if got, _ := payload["featured_media"].(int); got != 151 {
			t.Fatalf("%s: featured_media = %#v, mau 151", tc.name, payload["featured_media"])
		}
		if payload["status"] != "draft" {
			t.Fatalf("%s: status = %v, mau draft", tc.name, payload["status"])
		}
	}
}
