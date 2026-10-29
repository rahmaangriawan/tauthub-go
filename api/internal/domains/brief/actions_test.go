package brief

import (
	"strings"
	"testing"
)

// Aksi dari model harus disaring ke allowlist. Kesalahan model berakhir sebagai
// "aksi dibuang", bukan error yang menggagalkan balasan.
func TestValidateActionsRejects(t *testing.T) {
	valid := EngineAction{Kind: ActionTimelinePost, Label: "Tambahkan ke Timeline"}
	valid.Payload.Caption = "Promo potong rambut 20% sampai akhir bulan."
	valid.Payload.Hashtags = []string{"promo", "#barbershop"}
	valid.Payload.OccurredOn = "2026-10-29"
	valid.Reason = "user menyebut promo akhir bulan"

	got := ValidateActions([]EngineAction{valid})
	if len(got) != 1 {
		t.Fatalf("aksi valid harus lolos, dapat %d", len(got))
	}
	if got[0].Kind != ActionTimelinePost {
		t.Fatalf("kind berubah: %q", got[0].Kind)
	}
	if got[0].Caption != valid.Payload.Caption {
		t.Fatalf("caption berubah: %q", got[0].Caption)
	}
	// "#" di depan hashtag dibuang, duplikat tidak ada.
	if len(got[0].Hashtags) != 2 || got[0].Hashtags[0] != "promo" || got[0].Hashtags[1] != "barbershop" {
		t.Fatalf("hashtag tidak dinormalisasi: %#v", got[0].Hashtags)
	}
	if got[0].OccurredOn != "2026-10-29" {
		t.Fatalf("tanggal valid hilang: %q", got[0].OccurredOn)
	}

	unknown := EngineAction{Kind: "delete_brand"}
	unknown.Payload.Caption = "apa saja"
	if out := ValidateActions([]EngineAction{unknown}); len(out) != 0 {
		t.Fatalf("kind tak dikenal harus dibuang, dapat %#v", out)
	}

	empty := EngineAction{Kind: ActionTimelinePost}
	empty.Payload.Caption = "   "
	if out := ValidateActions([]EngineAction{empty}); len(out) != 0 {
		t.Fatalf("caption kosong harus dibuang")
	}

	tooLong := EngineAction{Kind: ActionTimelinePost}
	tooLong.Payload.Caption = strings.Repeat("a", ActionCaptionMax+1)
	if out := ValidateActions([]EngineAction{tooLong}); len(out) != 0 {
		t.Fatalf("caption melebihi batas harus dibuang")
	}
}

// Tanggal ngawur tidak membuang aksi; hanya tanggalnya yang dilewati, karena
// caption tetap berguna bagi user.
func TestValidateActionsKeepsCaptionOnBadDate(t *testing.T) {
	a := EngineAction{Kind: ActionTimelinePost}
	a.Payload.Caption = "Peluncuran menu baru minggu ini."
	a.Payload.OccurredOn = "besok sore"
	out := ValidateActions([]EngineAction{a})
	if len(out) != 1 {
		t.Fatalf("aksi harus tetap lolos, dapat %d", len(out))
	}
	if out[0].OccurredOn != "" {
		t.Fatalf("tanggal tidak valid harus dikosongkan, dapat %q", out[0].OccurredOn)
	}
	if out[0].Caption == "" {
		t.Fatalf("caption tidak boleh ikut hilang")
	}
}

// Model kadang mengulang usulan yang sama dalam satu balasan; hanya satu yang
// boleh tampil, dan jumlah aksi per balasan dibatasi.
func TestValidateActionsDedupesAndCaps(t *testing.T) {
	mk := func(caption string) EngineAction {
		a := EngineAction{Kind: ActionTimelinePost}
		a.Payload.Caption = caption
		return a
	}
	dup := ValidateActions([]EngineAction{mk("Promo 20%"), mk("promo 20%")})
	if len(dup) != 1 {
		t.Fatalf("duplikat caption harus dibuang, dapat %d", len(dup))
	}

	many := make([]EngineAction, 0, 6)
	for _, c := range []string{"satu", "dua", "tiga", "empat", "lima", "enam"} {
		many = append(many, mk(c))
	}
	if got := ValidateActions(many); len(got) != ActionPerReplyMax {
		t.Fatalf("aksi per balasan harus dibatasi %d, dapat %d", ActionPerReplyMax, len(got))
	}
}

// Label kosong diberi teks bawaan supaya tombol tidak pernah tanpa teks.
func TestValidateActionsDefaultLabel(t *testing.T) {
	a := EngineAction{Kind: ActionTimelinePost}
	a.Payload.Caption = "Kabar baru."
	out := ValidateActions([]EngineAction{a})
	if len(out) != 1 || strings.TrimSpace(out[0].Label) == "" {
		t.Fatalf("label bawaan tidak terpasang: %#v", out)
	}
}

// Kontrak JSON harus menyebut "actions" dan aturannya, karena model hanya tahu
// field yang tercantum di sana.
func TestOutputContractMentionsActions(t *testing.T) {
	contract := RenderOutputContract()
	if !strings.Contains(contract, `"actions"`) {
		t.Fatalf("kontrak tidak memuat key actions")
	}
	if !strings.Contains(contract, ActionTimelinePost) {
		t.Fatalf("kontrak tidak menyebut kind %s", ActionTimelinePost)
	}
	rules := RenderActionRules()
	if !strings.Contains(rules, "timeline_post") && !strings.Contains(rules, "Timeline") {
		t.Fatalf("aturan aksi tidak menyebut Timeline")
	}
	if !strings.Contains(rules, "usulan") {
		t.Fatalf("aturan aksi harus menegaskan bahwa ini hanya usulan")
	}
}

// Timeline terbaru harus masuk konteks supaya model tidak mengusulkan kabar
// yang sudah tayang, dan jumlahnya dibatasi.
func TestBuildUserMessageIncludesTimeline(t *testing.T) {
	view := StateView{
		Fields: testFields(), States: map[string]FieldState{}, Values: map[string]any{},
		Phase: PhaseFill,
		Timeline: []string{
			"Promo potong rambut 20% sampai akhir bulan.",
			"Kunjungan Chef Juna minggu lalu.",
		},
	}
	msg := BuildUserMessage(view)
	if !strings.Contains(msg, "TIMELINE TERBARU") {
		t.Fatalf("blok timeline tidak ada di konteks")
	}
	if !strings.Contains(msg, "Promo potong rambut 20%") {
		t.Fatalf("caption timeline tidak ikut")
	}

	// Tanpa timeline, blok itu tidak boleh muncul sama sekali (tidak ada
	// header kosong yang membingungkan model).
	empty := BuildUserMessage(StateView{
		Fields: testFields(), States: map[string]FieldState{}, Values: map[string]any{},
		Phase: PhaseFill,
	})
	if strings.Contains(empty, "TIMELINE TERBARU") {
		t.Fatalf("header timeline muncul walau daftar kosong")
	}

	// Batas jumlah: 30 baris masuk, yang tampil maksimal maxTimelineContext.
	many := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		many = append(many, "kabar nomor "+string(rune('A'+i)))
	}
	bounded := BuildUserMessage(StateView{
		Fields: testFields(), States: map[string]FieldState{}, Values: map[string]any{},
		Phase: PhaseFill, Timeline: many,
	})
	if strings.Count(bounded, "- kabar nomor") != maxTimelineContext {
		t.Fatalf("timeline harus dibatasi %d baris", maxTimelineContext)
	}
}

// ParseEngineOutput harus membaca blok actions dari JSON model, termasuk saat
// dibungkus pagar kode.
func TestParseEngineOutputReadsActions(t *testing.T) {
	raw := "```json\n" + `{
  "reply": "Boleh, aku siapkan usulannya.",
  "field_updates": [],
  "suggested_values": [],
  "asked_field": null,
  "ready_for_confirmation": false,
  "actions": [
    {"kind":"timeline_post","label":"Tambahkan ke Timeline",
     "payload":{"caption":"Promo 20% minggu ini.","hashtags":["promo"],"occurred_on":""},
     "reason":"user menyebut promo"}
  ]
}` + "\n```"
	out, err := parseEngineOutput(raw)
	if err != nil {
		t.Fatalf("parse gagal: %v", err)
	}
	if len(out.Actions) != 1 {
		t.Fatalf("actions tidak terbaca, dapat %d", len(out.Actions))
	}
	if out.Actions[0].Payload.Caption == "" {
		t.Fatalf("caption tidak terbaca")
	}
	got := ValidateActions(out.Actions)
	if len(got) != 1 {
		t.Fatalf("aksi hasil parse harus lolos validasi")
	}
}

// Endpoint consume hanya boleh menerima kind yang ada di allowlist. Ini
// mencegah klien menandai aksi fiktif yang tidak pernah ditawarkan model.
func TestAllowedActionsOnlyTimelineForNow(t *testing.T) {
	if !allowedActions[ActionTimelinePost] {
		t.Fatalf("timeline_post harus diizinkan")
	}
	if allowedActions["delete_brand"] || allowedActions[""] {
		t.Fatalf("kind di luar allowlist tidak boleh lolos")
	}
	if len(allowedActions) != 1 {
		t.Fatalf("allowlist bertambah tanpa tes: %d entri", len(allowedActions))
	}
}

// Hashtag dari model dibersihkan: "#" dibuang, spasi dipangkas, duplikat
// (tanpa beda huruf besar/kecil) dibuang, jumlah dibatasi.
func TestNormalizeActionHashtags(t *testing.T) {
	got := normalizeActionHashtags([]string{"#Promo", " promo ", "", "#PROMO", "barbershop"})
	if len(got) != 2 {
		t.Fatalf("harus 2 hashtag unik, dapat %#v", got)
	}
	if got[0] != "Promo" || got[1] != "barbershop" {
		t.Fatalf("hasil normalisasi salah: %#v", got)
	}

	many := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		many = append(many, "tag"+string(rune('a'+i)))
	}
	if len(normalizeActionHashtags(many)) != ActionHashtagsMax {
		t.Fatalf("jumlah hashtag harus dibatasi %d", ActionHashtagsMax)
	}

	if normalizeActionHashtags([]string{"   ", "#"}) != nil {
		t.Fatalf("hashtag kosong harus menghasilkan nil")
	}
}
