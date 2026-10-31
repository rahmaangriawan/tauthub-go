package seo

import "testing"

// Provider nyata kadang mengirim nilai di luar 0..100. Kolom DB
// (da/pa/spam_score smallint) punya CHECK 0..100, jadi nilai mentah membuat
// seluruh INSERT gagal — persis error "new row for relation media_seo_metrics
// violates check constraint ck_media_seo_metrics_spam".
//
// Test ini mengunci perilaku: nilai negatif → nil (tidak diukur), nilai di atas
// 100 → 100, pecahan → dibulatkan.
func TestClampScore(t *testing.T) {
	cases := []struct {
		name string
		in   *float64
		want *float64
	}{
		{"nil tetap nil", nil, nil},
		{"negatif jadi nil", f(-1), nil},
		{"negatif besar jadi nil", f(-25), nil},
		{"nol tetap nol", f(0), f(0)},
		{"dalam rentang", f(37), f(37)},
		{"batas atas", f(100), f(100)},
		{"lebih dari 100 dipotong", f(250), f(100)},
		{"pecahan dibulatkan", f(12.6), f(13)},
	}
	for _, c := range cases {
		got := clampScore(c.in)
		switch {
		case c.want == nil && got != nil:
			t.Errorf("%s: mau nil, dapat %v", c.name, *got)
		case c.want != nil && got == nil:
			t.Errorf("%s: mau %v, dapat nil", c.name, *c.want)
		case c.want != nil && got != nil && *got != *c.want:
			t.Errorf("%s: mau %v, dapat %v", c.name, *c.want, *got)
		}
	}
}

// findMetrics harus mengembalikan spam nil (bukan -1) untuk tanda
// "tidak diketahui", dan memotong nilai di atas 100.
func TestFindMetricsClampsSpam(t *testing.T) {
	// Bentuk respons dengan spam -1 (tanda tidak diketahui).
	payload := map[string]any{
		"data": []any{
			map[string]any{"domain": "menatamasa.com", "da": float64(12), "pa": float64(9), "spam_score": float64(-1)},
		},
	}
	da, pa, spam := findMetrics(payload, "menatamasa.com")
	if da == nil || pa == nil {
		t.Fatalf("DA/PA harus terisi, dapat da=%v pa=%v", da, pa)
	}
	if spam != nil {
		t.Errorf("spam -1 harus jadi nil (tidak diukur), dapat %v", *spam)
	}

	// Spam di atas 100 dipotong ke 100.
	payload2 := map[string]any{
		"data": []any{
			map[string]any{"domain": "contoh.com", "da": float64(20), "pa": float64(15), "spam_score": float64(180)},
		},
	}
	_, _, spam2 := findMetrics(payload2, "contoh.com")
	if spam2 == nil || *spam2 != 100 {
		t.Errorf("spam 180 harus jadi 100, dapat %v", spam2)
	}
}

// DA/PA negatif membuat domain dianggap tidak lengkap, bukan tersimpan sebagai 0.
func TestFindMetricsRejectsNegativeDA(t *testing.T) {
	payload := map[string]any{
		"data": []any{
			map[string]any{"domain": "rusak.com", "da": float64(-1), "pa": float64(-1)},
		},
	}
	da, pa, _ := findMetrics(payload, "rusak.com")
	if da != nil || pa != nil {
		t.Errorf("DA/PA negatif harus jadi nil, dapat da=%v pa=%v", da, pa)
	}
}

func f(v float64) *float64 { return &v }
