package ai

import (
	"strings"
	"testing"
)

func TestEvaluateAIOverviewPass(t *testing.T) {
	body := `{"indikator":[
		{"kunci":"struktur","skor":9,"ringkasan":"Lead BLUF kuat."},
		{"kunci":"entitas","skor":8,"ringkasan":"Banyak angka dan nama lembaga."},
		{"kunci":"eeat","skor":9,"ringkasan":"Ada kutipan ahli."},
		{"kunci":"niat_pengguna","skor":8,"ringkasan":"Menjawab PAA."}],
		"catatan_revisi":["harusnya diabaikan saat lulus"]}`
	res := EvaluateAIOverview(body, DefaultAIOverviewSettings(), "2026-10-03T00:00:00.000Z", true)
	if res.Status != "passed" || res.Verdict != "lulus" {
		t.Fatalf("status=%q verdict=%q, mau passed/lulus", res.Status, res.Verdict)
	}
	if res.Total != 34 {
		t.Fatalf("total=%d, mau 34", res.Total)
	}
	if len(res.Notes) != 0 {
		t.Fatalf("catatan revisi harus kosong saat lulus, dapat %v", res.Notes)
	}
	if len(res.Indicators) != 4 || res.Indicators[0].Title == "" {
		t.Fatalf("indikator tidak lengkap: %+v", res.Indicators)
	}
	m := res.JSON()
	if m["maxScore"] != AIOverviewMaxScore || m["status"] != "passed" {
		t.Fatalf("JSON() salah: %v", m)
	}
}

func TestEvaluateAIOverviewRejectAndNotes(t *testing.T) {
	body := `{"indikator":[
		{"kunci":"struktur","skor":6,"ringkasan":"Lead bertele-tele."},
		{"kunci":"entitas","skor":9,"ringkasan":"Padat."},
		{"kunci":"eeat","skor":9,"ringkasan":"Baik."},
		{"kunci":"niat_pengguna","skor":9,"ringkasan":"Baik."}],
		"catatan_revisi":["Pindahkan definisi ke paragraf pertama.","Tambah data harga terbaru.","Sebut sumber statistik.","Poin keempat dipotong."]}`
	res := EvaluateAIOverview(body, DefaultAIOverviewSettings(), "2026-10-03T00:00:00.000Z", true)
	if res.Status != "review_required" || res.Verdict != "ditolak" {
		t.Fatalf("status=%q verdict=%q, mau review_required/ditolak", res.Status, res.Verdict)
	}
	if res.Total != 33 {
		t.Fatalf("total=%d, mau 33 (indikator <7 tetap ditolak walau total tinggi)", res.Total)
	}
	if len(res.Notes) != 3 {
		t.Fatalf("catatan revisi harus dipotong ke 3, dapat %d", len(res.Notes))
	}
	entry := res.JSON()
	if !AIOverviewRejected(entry) {
		t.Fatal("AIOverviewRejected harus true")
	}
	if len(AIOverviewRevisionNotes(entry)) != 3 {
		t.Fatal("AIOverviewRevisionNotes harus mengembalikan 3 catatan")
	}
}

func TestEvaluateAIOverviewScoreClampAndAliases(t *testing.T) {
	body := `{"indikator":[
		{"key":"Struktur","score":15.4,"summary":"clamp atas"},
		{"key":"ENTITIES","skor":-3,"ringkasan":"clamp bawah"},
		{"key":"e-e-a-t","skor":"8","ringkasan":"string angka"},
		{"key":"userIntent","skor":7,"ringkasan":"alias"}],
		"catatan_revisi":[]}`
	res := EvaluateAIOverview(body, DefaultAIOverviewSettings(), "", true)
	if res.Status != "review_required" {
		t.Fatalf("harus review_required (ada skor 0), dapat %q", res.Status)
	}
	if res.Indicators[0].Score != 10 || res.Indicators[1].Score != 0 || res.Indicators[2].Score != 8 {
		t.Fatalf("clamp/parse salah: %+v", res.Indicators)
	}
	if res.Indicators[3].Key != "niatPengguna" {
		t.Fatalf("alias userIntent tidak terpetakan: %+v", res.Indicators[3])
	}
}

func TestEvaluateAIOverviewInvalid(t *testing.T) {
	if res := EvaluateAIOverview("bukan json sama sekali", DefaultAIOverviewSettings(), "", true); res.Error == "" || res.Status != "review_required" {
		t.Fatalf("JSON tidak valid harus error, dapat %+v", res)
	}
	partial := `{"indikator":[{"kunci":"struktur","skor":9,"ringkasan":"x"}],"catatan_revisi":[]}`
	if res := EvaluateAIOverview(partial, DefaultAIOverviewSettings(), "", true); res.Error == "" {
		t.Fatal("indikator kurang dari 4 harus error")
	}
	// Pagar markdown tetap diparse.
	fenced := "```json\n" + `{"indikator":[{"kunci":"struktur","skor":8,"ringkasan":"a"},{"kunci":"entitas","skor":8,"ringkasan":"b"},{"kunci":"eeat","skor":8,"ringkasan":"c"},{"kunci":"niat_pengguna","skor":8,"ringkasan":"d"}],"catatan_revisi":[]}` + "\n```"
	if res := EvaluateAIOverview(fenced, DefaultAIOverviewSettings(), "", true); res.Status != "passed" {
		t.Fatalf("pagar markdown harus tetap diparse, dapat %+v", res)
	}
}

func TestAIOverviewSettingsFromJSON(t *testing.T) {
	st := AIOverviewSettingsFromJSON(map[string]any{
		"enabled": false, "pass_total": float64(30), "pass_min_indicator": float64(6), "max_chars": float64(12000),
	})
	if st.Enabled || st.PassTotal != 30 || st.PassMinIndicator != 6 || st.MaxChars != 12000 {
		t.Fatalf("parse settings salah: %+v", st)
	}
	def := AIOverviewSettingsFromJSON(nil)
	if !def.Enabled || def.PassTotal != 32 || def.PassMinIndicator != 7 || def.MaxChars != 8000 {
		t.Fatalf("default salah: %+v", def)
	}
	// Nilai di luar rentang diabaikan (tetap default).
	out := AIOverviewSettingsFromJSON(map[string]any{"pass_total": float64(99), "max_chars": float64(10)})
	if out.PassTotal != 32 || out.MaxChars != 8000 {
		t.Fatalf("nilai tak masuk akal harus diabaikan: %+v", out)
	}
}

func TestAIOverviewUserPromptTruncates(t *testing.T) {
	html := ""
	for i := 0; i < 500; i++ {
		html += "abcdefghij"
	}
	st := DefaultAIOverviewSettings()
	st.MaxChars = 1000
	p := AIOverviewUserPrompt("Judul", "blogger", html, 42, st, "Bahan brief contoh")
	// Isi HTML dipotong ke MaxChars; sisa panjang adalah header tetap prompt.
	if len(p) > st.MaxChars+700 {
		t.Fatalf("prompt harus dipotong sekitar max_chars, panjang %d", len(p))
	}
	if !strings.Contains(p, "Judul: Judul") || !strings.Contains(p, "blogger") || !strings.Contains(p, "Jumlah kata: 42") ||
		!strings.Contains(p, "Aturan kelulusan: total minimal 32 dari 40 DAN setiap indikator minimal 7") ||
		!strings.Contains(p, "BAHAN/BRIEF (rujukan fakta yang boleh dipakai artikel):") ||
		!strings.Contains(p, "Bahan brief contoh") {
		t.Fatalf("prompt kehilangan bagian penting: %s", p[:120])
	}
}

func TestEvaluateAIOverviewBriefEmptyAndUnsourced(t *testing.T) {
	// Bahan kosong + model melaporkan entitas tanpa dasar -> tersimpan apa adanya.
	body := `{"indikator":[
		{"kunci":"struktur","skor":8,"ringkasan":"a"},
		{"kunci":"entitas","skor":5,"ringkasan":"b"},
		{"kunci":"eeat","skor":5,"ringkasan":"c"},
		{"kunci":"niat_pengguna","skor":8,"ringkasan":"d"}],
		"catatan_revisi":["Hapus kutipan tanpa sumber."],
		"entitas_tanpa_dasar":["Budi Santoso","PT Epoxy Solusi Nusantara","RSUD Kota Bandung","entri keempat dipotong"]}`
	res := EvaluateAIOverview(body, DefaultAIOverviewSettings(), "", false)
	if res.BriefAvailable {
		t.Fatal("briefAvailable harus false saat bahan kosong")
	}
	if len(res.UnsourcedEntities) != 3 {
		t.Fatalf("entitas tanpa dasar harus dipotong ke 3, dapat %d", len(res.UnsourcedEntities))
	}
	if res.UnsourcedEntities[0] != "Budi Santoso" || res.UnsourcedEntities[2] != "RSUD Kota Bandung" {
		t.Fatalf("entitas tanpa dasar tidak sesuai: %v", res.UnsourcedEntities)
	}
	m := res.JSON()
	if m["briefAvailable"] != false {
		t.Fatalf("JSON() briefAvailable salah: %v", m["briefAvailable"])
	}
	if arr, ok := m["unsourcedEntities"].([]string); !ok || len(arr) != 3 {
		t.Fatalf("JSON() unsourcedEntities salah: %v", m["unsourcedEntities"])
	}
	// Tanpa field itu (model lama/gagal) -> array kosong, bukan nil.
	res2 := EvaluateAIOverview(`{"indikator":[
		{"kunci":"struktur","skor":8,"ringkasan":"a"},
		{"kunci":"entitas","skor":8,"ringkasan":"b"},
		{"kunci":"eeat","skor":8,"ringkasan":"c"},
		{"kunci":"niat_pengguna","skor":8,"ringkasan":"d"}],"catatan_revisi":[]}`, DefaultAIOverviewSettings(), "", true)
	if res2.UnsourcedEntities == nil || len(res2.UnsourcedEntities) != 0 {
		t.Fatalf("unsourcedEntities harus array kosong, dapat %v", res2.UnsourcedEntities)
	}
}

func TestAIOverviewUserPromptBriefEmptyMarker(t *testing.T) {
	st := DefaultAIOverviewSettings()
	p := AIOverviewUserPrompt("Judul", "mainstream", "<p>x</p>", 10, st, "   ")
	if !strings.Contains(p, "(KOSONG — tidak ada brief/bahan rujukan untuk generate artikel ini)") {
		t.Fatalf("penanda bahan kosong tidak ada di prompt: %s", p)
	}
}
