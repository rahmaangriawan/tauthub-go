package brandflow

import (
	"strings"
	"testing"
)

// requestedWordTarget: ambil JENDELA jumlah kata dari instruksi klien.
// Angka yang jelas pembatas atas ("maksimal 2500 kata") diabaikan; angka
// tunggal ("800 kata") jadi target ±10% supaya tidak cuma jadi batas bawah.
func TestRequestedWordTarget(t *testing.T) {
	cases := []struct {
		name  string
		topic string
		want  wordTarget
		ok    bool
	}{
		{"polos 800 (kasus klien)", "Artikel 800 kata tentang undangan digital", wordTarget{Min: 720, Max: 880, Exact: 800}, true},
		{"minimal", "Buat opini. Minimal 2500 kata.", wordTarget{Min: 2500, Max: 3125}, true},
		{"kalimat panjang", "Buat opini tentang Tauthub dan perkembangan distribusi konten digital. Minimal 600 kata, tanpa gambar", wordTarget{Min: 600, Max: 750}, true},
		{"sekitar", "sekitar 1200 kata", wordTarget{Min: 1080, Max: 1320, Exact: 1200}, true},
		{"inggris", "write 2500 words", wordTarget{Min: 2250, Max: 2750, Exact: 2500}, true},
		{"rentang strip", "tolong tulis 800-900 kata tentang kopi", wordTarget{Min: 800, Max: 900}, true},
		{"rentang en dash", "800–1000 kata tentang kopi", wordTarget{Min: 800, Max: 1000}, true},
		{"rentang sampai", "tulis 800 sampai 900 kata", wordTarget{Min: 800, Max: 900}, true},
		{"angka kecil", "artikel 250 kata", wordTarget{Min: 220, Max: 280, Exact: 250}, true},
		{"batas atas maksimal", "maksimal 2500 kata", wordTarget{}, false},
		{"batas atas maks", "maks 1500 kata", wordTarget{}, false},
		{"batas atas tidak lebih dari", "tidak lebih dari 2000 kata", wordTarget{}, false},
		{"terlalu kecil", "100 kata", wordTarget{}, false},
		{"tanpa angka", "tanpa batas kata", wordTarget{}, false},
		{"kosong", "", wordTarget{}, false},
		{"ambil terbesar", "minimal 800 kata, idealnya 1500 kata", wordTarget{Min: 1350, Max: 1650, Exact: 1500}, true},
	}
	for _, tc := range cases {
		got, ok := requestedWordTarget(tc.topic)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: requestedWordTarget(%q) = (%+v,%v), mau (%+v,%v)", tc.name, tc.topic, got, ok, tc.want, tc.ok)
		}
	}
}

// accepts: draf hanya diterima di dalam jendela; di luar itu ditulis ulang.
func TestWordTargetAccepts(t *testing.T) {
	pola := wordTarget{Min: 720, Max: 880, Exact: 800}
	for _, c := range []struct {
		words int
		want  bool
	}{{700, false}, {720, true}, {800, true}, {880, true}, {1000, false}} {
		if got := pola.accepts(c.words); got != c.want {
			t.Errorf("accepts(%d) = %v, mau %v", c.words, got, c.want)
		}
	}
	minimal := wordTarget{Min: 2500, Max: 3125}
	if !minimal.accepts(2500) || !minimal.accepts(3000) || minimal.accepts(2400) {
		t.Errorf("jendela minimal salah: %+v", minimal)
	}
}

// Prompt aturan & koreksi menyebut KEDUA batas (bukan cuma "jangan kurang dari").
func TestClientWordRuleMentionsWindow(t *testing.T) {
	rule := clientWordRule(wordTarget{Min: 720, Max: 880, Exact: 800})
	for _, want := range []string{"800 kata", "720", "880", "MENANG"} {
		if !strings.Contains(rule, want) {
			t.Errorf("clientWordRule kurang %q: %s", want, rule)
		}
	}
	correction := (wordTarget{Min: 720, Max: 880, Exact: 800}).correction(1000)
	for _, want := range []string{"1000 kata", "720", "880", "Tulis ULANG"} {
		if !strings.Contains(correction, want) {
			t.Errorf("correction kurang %q: %s", want, correction)
		}
	}
}
