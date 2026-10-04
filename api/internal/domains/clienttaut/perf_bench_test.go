package clienttaut

import (
	"strings"
	"testing"
)

// humanizeReason dipanggil per baris media di matchmaker (matchmaker.go:176),
// dan tiap panggilan mengulang seluruh angleLabels — jalur ini di-benchmark
// sekaligus jadi sumber profil PGO.

func BenchmarkHumanizeReason(b *testing.B) {
	text := "konten niche_education dengan angle comparison lalu soft_sell_promo untuk brand"
	b.ReportAllocs()
	var sink string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = humanizeReason(text)
	}
	_ = sink
}

// TestHumanizeReasonKnownOutputs mengunci keluaran untuk masukan satu token,
// yang tidak bergantung urutan iterasi map (deterministik).
func TestHumanizeReasonKnownOutputs(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"comparison", "Perbandingan Produk"},
		{"niche_education", "Edukasi Niche"},
		{"unboxing", "Unboxing"},
		{"thought_leadership", "Opini Pakar"},
	}
	for _, c := range cases {
		if got := humanizeReason(c.in); got != c.want {
			t.Errorf("humanizeReason(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

// TestAngleLabelsDoNotSelfMatch: label manusiawi tidak boleh memuat key angle
// lain sebagai token utuh. Inilah yang menjamin penyisipan sebuah label tidak
// pernah memunculkan kecocokan baru, sehingga precompile regex aman.
func TestAngleLabelsDoNotSelfMatch(t *testing.T) {
	for key, label := range angleLabels {
		if strings.Contains(label, "_") {
			t.Errorf("label %q (dari %q) memuat underscore", label, key)
		}
		for other := range angleLabels {
			if other == key {
				continue
			}
			if wordBoundaryRe(other).MatchString(label) {
				t.Errorf("label %q (dari %q) cocok dengan key lain %q", label, key, other)
			}
		}
	}
}
