package ai

import (
	"strings"
	"testing"
)

func TestGuardSystemPromptComposesBothTasks(t *testing.T) {
	p := GuardSystemPrompt("ATURAN KEAMANAN KHUSUS DOMAIN")
	for _, want := range []string{
		"ATURAN KEAMANAN KHUSUS DOMAIN",
		"Gatekeeper AI Overview QC Agent",
		"VERIFIKASI KE BAHAN",
		`"safety"`,
		`"overview"`,
		"Balas HANYA JSON valid",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt gabungan kehilangan bagian %q", want)
		}
	}
}

func TestGuardUserPromptSections(t *testing.T) {
	st := DefaultAIOverviewSettings()
	p := GuardUserPrompt("Judul X", "blogger", "<p>isi</p>", "isi teks", "", []string{"https://a.test"}, 123, st)
	for _, want := range []string{
		"Judul: Judul X",
		"blogger (artikel evergreen, memakai heading)",
		"Jumlah kata: 123",
		"Aturan kelulusan AI Overview: total minimal 32 dari 40 DAN setiap indikator minimal 7",
		"(KOSONG — tidak ada brief/bahan rujukan untuk generate artikel ini)",
		"https://a.test",
		"Konten (teks polos):",
		"Isi (HTML):",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt gabungan kehilangan bagian %q", want)
		}
	}
	if !strings.Contains(GuardUserPrompt("J", "mainstream", "x", "y", "Bahan ada", nil, 1, st), "Bahan ada") {
		t.Fatal("brief tidak ikut ke prompt")
	}
	// Tanpa tautan → penanda eksplisit (bukan bagian kosong).
	if !strings.Contains(p, "https://a.test") || !strings.Contains(GuardUserPrompt("J", "mainstream", "x", "y", "B", nil, 1, st), "(tidak ada tautan)") {
		t.Fatal("penanda tautan tidak benar")
	}
}

func TestParseGuardResponse(t *testing.T) {
	valid := `{"safety":{"allowed":true,"riskLevel":"low","categories":[],"reasons":["aman"],"confidence":0.9},
		"overview":{"indikator":[{"kunci":"struktur","skor":9,"ringkasan":"BLUF"},{"kunci":"entitas","skor":8,"ringkasan":"padat"},{"kunci":"eeat","skor":8,"ringkasan":"sumber"},{"kunci":"niat_pengguna","skor":8,"ringkasan":"PAA"}],"catatan_revisi":[],"entitas_tanpa_dasar":["PT Karangan"]}}`
	ov, sf, ok := ParseGuardResponse("```json\n" + valid + "\n```")
	if !ok || ov == nil || sf == nil {
		t.Fatalf("respons valid harus diparse: ok=%v", ok)
	}
	if sf["riskLevel"] != "low" {
		t.Fatalf("safety tidak terbaca: %v", sf)
	}
	res := EvaluateAIOverviewFields(ov, DefaultAIOverviewSettings(), "2026-10-03T00:00:00.000Z", true)
	if res.Status != "passed" || res.Total != 33 {
		t.Fatalf("evaluasi dari sub-objek salah: status=%s total=%d", res.Status, res.Total)
	}
	if len(res.UnsourcedEntities) != 1 || res.UnsourcedEntities[0] != "PT Karangan" {
		t.Fatalf("entitas tanpa dasar tidak terbaca: %v", res.UnsourcedEntities)
	}

	// Bagian safety hilang → fallback (ok=false).
	if _, _, ok := ParseGuardResponse(`{"overview":{"indikator":[]}}`); ok {
		t.Fatal("tanpa bagian safety harus ok=false")
	}
	// Bagian overview hilang → fallback.
	if _, _, ok := ParseGuardResponse(`{"safety":{"riskLevel":"low"}}`); ok {
		t.Fatal("tanpa bagian overview harus ok=false")
	}
	// Bukan JSON → fallback.
	if _, _, ok := ParseGuardResponse("tidak ada json"); ok {
		t.Fatal("respons non-JSON harus ok=false")
	}
	// Indikator tidak lengkap → hasil review_required + error (bukan panic).
	ovShort, sfShort, ok := ParseGuardResponse(`{"safety":{"riskLevel":"high"},"overview":{"indikator":[{"kunci":"struktur","skor":5,"ringkasan":"x"}]}}`)
	if !ok || ovShort == nil || sfShort == nil {
		t.Fatal("objek lengkap harus ok=true walau isi indikator kurang")
	}
	resShort := EvaluateAIOverviewFields(ovShort, DefaultAIOverviewSettings(), "", true)
	if resShort.Error == "" || resShort.Status != "review_required" {
		t.Fatalf("indikator kurang dari 4 harus error: %+v", resShort)
	}
}
