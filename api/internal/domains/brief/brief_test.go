package brief

import (
	"strings"
	"testing"
)

func testFields() []Field {
	return []Field{
		{Key: "canonical_name", Layer: 1, Section: "Entity Core", Label: "Nama kanonik", Widget: "text", Required: true, SortOrder: 1, Enabled: true},
		{Key: "canonical_description", Layer: 1, Section: "Entity Core", Label: "Deskripsi kanonik", Widget: "longtext", Required: true, SortOrder: 2, Enabled: true},
		{Key: "forbidden_claims", Layer: 1, Section: "Entity Core", Label: "Klaim dilarang", Widget: "longtext", Required: true, SortOrder: 3, Enabled: true},
		{Key: "tone", Layer: 2, Section: "Gaya Penulisan", Label: "Gaya bahasa", Widget: "text", SortOrder: 201, Enabled: true},
		{Key: "memory_note", Layer: 3, Section: "Campaign Memory", Label: "Catatan sistem", Widget: "text", SortOrder: 901, Enabled: true},
		{Key: "disabled_field", Layer: 2, Section: "Gaya Penulisan", Label: "Nonaktif", Widget: "text", SortOrder: 202},
	}
}

func TestStorageColumn(t *testing.T) {
	fields := testFields()
	cases := map[string]string{
		"canonical_name": "entity_core",
		"tone":           "style",
	}
	for key, want := range cases {
		f, ok := fieldByKey(fields, key)
		if !ok {
			t.Fatalf("field %s tidak ada di fixture", key)
		}
		got, ok := StorageColumn(f)
		if !ok || got != want {
			t.Fatalf("%s: kolom %q (ok=%v), mau %q", key, got, ok, want)
		}
	}
	// Kolom eksplisit menang atas pemetaan seksi.
	explicit := Field{Key: "x", Section: "Seksi Baru", Column: "entity_core"}
	if got, ok := StorageColumn(explicit); !ok || got != "entity_core" {
		t.Fatalf("kolom eksplisit harus dipakai, dapat %q ok=%v", got, ok)
	}
	if _, ok := StorageColumn(Field{Key: "y", Section: "Seksi Tak Dikenal"}); ok {
		t.Fatalf("seksi tanpa kolom harus ditolak")
	}
}

func TestValidateUpdatesRejects(t *testing.T) {
	fields := testFields()
	current := map[string]FieldState{
		"canonical_name": {Status: StatusConfirmed, Source: SourceUser},
	}
	updates := []FieldUpdate{
		{Key: "canonical_name", Value: "Tauthub", Status: StatusFilled},               // turun status → tolak
		{Key: "memory_note", Value: "apa saja", Status: StatusFilled},                 // lapis 3 → tolak
		{Key: "tidak_ada", Value: "x", Status: StatusFilled},                          // bukan skema → tolak
		{Key: "disabled_field", Value: "x", Status: StatusFilled},                     // nonaktif → tolak
		{Key: "tone", Value: "   ", Status: StatusFilled},                             // kosong → tolak
		{Key: "forbidden_claims", Value: "jangan klaim nomor 1", Status: "sembarang"}, // status tak dikenal → filled
		{Key: "canonical_description", Value: "Tauthub adalah platform PR.", Status: StatusFilled, Source: "sumber_ajaib"},
	}
	accepted, rejected := ValidateUpdates(fields, current, updates)

	if len(accepted) != 2 {
		t.Fatalf("harus 2 update diterima, dapat %d: %+v", len(accepted), accepted)
	}
	if len(rejected) != 5 {
		t.Fatalf("harus 5 update ditolak, dapat %d: %+v", len(rejected), rejected)
	}
	if accepted[0].Status != StatusFilled {
		t.Fatalf("status tak dikenal harus jatuh ke filled, dapat %q", accepted[0].Status)
	}
	if accepted[1].Source != SourceUser {
		t.Fatalf("source tak dikenal harus jatuh ke user, dapat %q", accepted[1].Source)
	}
}

func TestValidateUpdatesKeepsConfirmedChange(t *testing.T) {
	fields := testFields()
	current := map[string]FieldState{
		"canonical_name": {Status: StatusConfirmed, Source: SourceUser},
	}
	// User minta ubah: datang dengan status confirmed → harus lolos.
	accepted, rejected := ValidateUpdates(fields, current, []FieldUpdate{
		{Key: "canonical_name", Value: "Tauthub.com", Status: StatusConfirmed, Source: SourceUser},
	})
	if len(accepted) != 1 || len(rejected) != 0 {
		t.Fatalf("perubahan yang diminta user harus lolos: accepted=%d rejected=%v", len(accepted), rejected)
	}
}

func TestCandidatesOrdersByPriorityAndSkipsFilled(t *testing.T) {
	fields := testFields()
	states := map[string]FieldState{
		"canonical_name":   {Status: StatusConfirmed},
		"forbidden_claims": {Status: StatusFilled},
	}
	got := Candidates(fields, states, 0)
	want := []string{"canonical_description"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("kandidat salah: %v, mau %v", got, want)
	}
	// Lapis 3 dan field nonaktif tidak pernah jadi kandidat.
	for _, key := range got {
		if key == "memory_note" || key == "disabled_field" {
			t.Fatalf("field sistem/nonaktif tidak boleh jadi kandidat: %v", got)
		}
	}
}

func TestCandidatesRespectsLimit(t *testing.T) {
	fields := testFields()
	got := Candidates(fields, map[string]FieldState{}, 2)
	if len(got) != 2 {
		t.Fatalf("limit tidak dihormati: %v", got)
	}
	if got[0] != "canonical_name" || got[1] != "canonical_description" {
		t.Fatalf("urutan prioritas salah: %v", got)
	}
}

func TestProgressCountsRequiredOnly(t *testing.T) {
	fields := testFields()
	states := map[string]FieldState{
		"canonical_name":        {Status: StatusConfirmed},
		"tone":                  {Status: StatusConfirmed}, // lapis 2 opsional, tidak dihitung
		"canonical_description": {Status: StatusFilled},
	}
	filled, total := Progress(fields, states)
	if total != 3 {
		t.Fatalf("total field wajib harus 3, dapat %d", total)
	}
	if filled != 2 {
		t.Fatalf("filled harus 2, dapat %d", filled)
	}
}

func TestDeterminePhase(t *testing.T) {
	base := PhaseInput{HasUserMessage: true, RequiredFilled: 2, RequiredTotal: 11}
	cases := []struct {
		name string
		in   PhaseInput
		want Phase
	}{
		{"percakapan baru", PhaseInput{RequiredTotal: 11}, PhaseIntake},
		{"sudah ada pesan user", base, PhaseFill},
		{
			"material baru masuk, belum dibalas",
			PhaseInput{HasUserMessage: true, HasMaterial: true, MaterialAt: 200, LastUserAt: 100, RequiredFilled: 2, RequiredTotal: 11},
			PhaseConfirm,
		},
		{
			"material sudah dibalas",
			PhaseInput{HasUserMessage: true, HasMaterial: true, MaterialAt: 100, LastUserAt: 200, RequiredFilled: 2, RequiredTotal: 11},
			PhaseFill,
		},
		{
			"semua wajib terisi",
			PhaseInput{HasUserMessage: true, RequiredFilled: 11, RequiredTotal: 11},
			PhaseReadyCheck,
		},
		{
			"assistant menawarkan konfirmasi",
			PhaseInput{HasUserMessage: true, AssistantAwaitingConfirm: true, AwaitingConfirmAt: 300, LastUserAt: 200, RequiredFilled: 11, RequiredTotal: 11},
			PhaseReadyCheck,
		},
		{
			"user membalas tawaran konfirmasi",
			PhaseInput{HasUserMessage: true, AssistantAwaitingConfirm: true, AwaitingConfirmAt: 300, LastUserAt: 400, RequiredFilled: 11, RequiredTotal: 11},
			PhaseDone,
		},
	}
	for _, tc := range cases {
		if got := DeterminePhase(tc.in); got != tc.want {
			t.Fatalf("%s: fase %q, mau %q", tc.name, got, tc.want)
		}
	}
}

func TestParseEngineOutput(t *testing.T) {
	cases := []string{
		`{"reply":"halo","field_updates":[],"suggested_values":[],"asked_field":null,"ready_for_confirmation":false}`,
		"```json\n{\"reply\":\"halo\",\"field_updates\":[]}\n```",
		"Baik, ini hasilnya:\n{\"reply\":\"halo\",\"field_updates\":[]}",
	}
	for _, raw := range cases {
		out, err := parseEngineOutput(raw)
		if err != nil {
			t.Fatalf("gagal parse %q: %v", raw, err)
		}
		if out.Reply != "halo" {
			t.Fatalf("reply salah: %q", out.Reply)
		}
	}
	if _, err := parseEngineOutput("bukan json sama sekali"); err == nil {
		t.Fatalf("teks tanpa objek JSON harus error")
	}
}

// Kontrak "reply" harus jadi key pertama supaya parser inkremental bisa
// memancarkannya lebih dulu. Test ini menjaga urutan itu tidak tergeser.
func TestEngineOutputJSONPutsReplyFirst(t *testing.T) {
	out := EngineOutput{Reply: "halo"}
	raw := jsonStringify(out)
	if len(raw) < 10 || raw[:10] != `{"reply":"` {
		t.Fatalf("reply harus jadi key pertama, dapat: %s", raw)
	}
}

// Konteks harus memberi tahu model field apa yang ia tanyakan giliran lalu,
// supaya jawaban pendek (satu kata) tidak dinilai sebagai pesan berdiri sendiri.
func TestBuildUserMessageInjectsAskedField(t *testing.T) {
	view := StateView{
		Fields: testFields(), States: map[string]FieldState{}, Values: map[string]any{},
		Phase: PhaseFill, AskedField: "canonical_name",
	}
	msg := BuildUserMessage(view)
	if !strings.Contains(msg, "asked_field): canonical_name") {
		t.Fatalf("asked_field tidak disuntik ke konteks:\n%s", msg[:400])
	}
	if !strings.Contains(msg, "adalah jawaban atas pertanyaan itu") {
		t.Fatalf("petunjuk jawaban-menempel tidak ada")
	}

	empty := BuildUserMessage(StateView{
		Fields: testFields(), States: map[string]FieldState{}, Values: map[string]any{},
		Phase: PhaseIntake,
	})
	if strings.Contains(empty, "asked_field") {
		t.Fatalf("tanpa pertanyaan menggantung, asked_field tidak boleh muncul")
	}
	if !strings.Contains(empty, "Belum ada pertanyaan menggantung") {
		t.Fatalf("penanda 'belum ada pertanyaan menggantung' tidak ada")
	}
}
