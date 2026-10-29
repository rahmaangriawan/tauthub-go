package brief

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Tes alur chatbot untuk fitur aksi: dari JSON model sampai baris pesan yang
// dikirim ke frontend, dan penandaan aksi terpakai.
//
// Semua di sini murni: tidak ada DB, tidak ada panggilan model. Yang diuji
// adalah sambungan antar-bagian, tempat bug paling mungkin muncul.

// Jalur lengkap: model membalas dengan actions, hasil parse divalidasi, lalu
// baris pesan untuk frontend harus membawa aksinya.
func TestChatbotActionFlow(t *testing.T) {
	raw := `{
	  "reply": "Promo-nya sudah aku rapikan. Tekan tombol di bawah kalau mau tayang.",
	  "field_updates": [],
	  "suggested_values": [],
	  "asked_field": null,
	  "ready_for_confirmation": false,
	  "actions": [
	    {"kind":"timeline_post","label":"Tambahkan ke Timeline",
	     "payload":{"caption":"Promo potong rambut 20% sepanjang Oktober.","hashtags":["promo","barbershop"],"occurred_on":"2026-10-29"},
	     "reason":"user menyebut promo bulan ini"}
	  ]
	}`
	out, err := parseEngineOutput(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	actions := ValidateActions(out.Actions)
	if len(actions) != 1 {
		t.Fatalf("aksi harus lolos validasi, dapat %d", len(actions))
	}

	// Baris pesan seperti yang dikirim GET /chat setelah reload.
	msg := message{
		ID:        "m-1",
		Role:      "assistant",
		Content:   out.Reply,
		CreatedAt: time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC),
		Meta:      map[string]any{"actions": actions, "ready_for_confirmation": false},
	}
	row := assistantMessageRow(msg)

	got, ok := row["actions"].([]ActionView)
	if !ok {
		t.Fatalf("baris pesan tidak membawa actions bertipe ActionView: %T", row["actions"])
	}
	if len(got) != 1 || got[0].Kind != ActionTimelinePost {
		t.Fatalf("aksi hilang di baris pesan: %#v", got)
	}
	if got[0].Caption == "" {
		t.Fatalf("caption kosong di baris pesan")
	}
	// actions_used harus array kosong, bukan nil, supaya frontend tidak perlu
	// menebak bentuknya.
	used, ok := row["actions_used"].([]string)
	if !ok || len(used) != 0 {
		t.Fatalf("actions_used harus []string kosong, dapat %#v", row["actions_used"])
	}
	if row["created_at"] != "2026-10-29T09:00:00.000Z" {
		t.Fatalf("format waktu pesan berubah: %v", row["created_at"])
	}
}

// Setelah aksi dijalankan, baris pesan harus menandainya terpakai sehingga
// frontend menonaktifkan tombolnya. Ini yang mencegah post duplikat setelah
// reload.
func TestChatbotActionMarkedUsed(t *testing.T) {
	meta := map[string]any{
		"actions": []ActionView{{Kind: ActionTimelinePost, Label: "Tambahkan ke Timeline", Caption: "Promo 20%."}},
	}

	updated, already := markActionUsed(meta, ActionTimelinePost)
	if already {
		t.Fatalf("penandaan pertama tidak boleh dianggap sudah ada")
	}
	if !actionUsedIn(updated, ActionTimelinePost) {
		t.Fatalf("kind tidak tercatat setelah penandaan")
	}

	// Klik kedua: idempoten, tidak menambah duplikat.
	again, already2 := markActionUsed(updated, ActionTimelinePost)
	if !already2 {
		t.Fatalf("klik kedua harus melaporkan already=true")
	}
	if n := len(actionUsedKinds(again)); n != 1 {
		t.Fatalf("klik ganda menambah entri: %d", n)
	}

	// Meta lain tidak boleh hilang saat ditandai.
	if _, ok := again["actions"]; !ok {
		t.Fatalf("meta actions ikut terhapus")
	}
}

// Meta datang dari jsonb, jadi actions_used terbaca sebagai []any. Fungsi harus
// menormalkannya, bukan menganggapnya []string.
func TestActionUsedKindsFromJSON(t *testing.T) {
	var meta map[string]any
	if err := json.Unmarshal([]byte(`{"actions_used":["timeline_post"]}`), &meta); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := actionUsedKinds(meta)
	if len(got) != 1 || got[0] != ActionTimelinePost {
		t.Fatalf("[]any tidak dinormalkan: %#v", got)
	}
	if !actionUsedIn(meta, ActionTimelinePost) {
		t.Fatalf("actionUsedIn gagal untuk data dari jsonb")
	}
	// Nilai sampah (bukan string) dilewati, bukan panic.
	var dirty map[string]any
	_ = json.Unmarshal([]byte(`{"actions_used":["timeline_post",7,null]}`), &dirty)
	if got := actionUsedKinds(dirty); len(got) != 1 {
		t.Fatalf("nilai non-string harus dilewati: %#v", got)
	}
}

// Pesan tanpa aksi tidak boleh mendapat key actions, dan pesan user tidak
// pernah membawa aksi walau meta-nya berisi.
func TestAssistantMessageRowWithoutActions(t *testing.T) {
	plain := assistantMessageRow(message{ID: "m-2", Role: "assistant", Content: "Halo"})
	if _, ok := plain["actions"]; ok {
		t.Fatalf("pesan tanpa aksi tidak boleh punya key actions")
	}

	userRow := assistantMessageRow(message{
		ID: "m-3", Role: "user", Content: "promo dong",
		Meta: map[string]any{"actions": []ActionView{{Kind: ActionTimelinePost, Caption: "x"}}},
	})
	if _, ok := userRow["actions"]; ok {
		t.Fatalf("pesan user tidak boleh membawa aksi")
	}
	if _, ok := userRow["actions_used"]; ok {
		t.Fatalf("pesan user tidak boleh membawa actions_used")
	}
}

// Meta kosong disimpan sebagai NULL, bukan objek kosong, supaya query yang
// memeriksa "punya meta" tetap benar.
func TestEncodeMetaEmptyIsNil(t *testing.T) {
	v, err := encodeMeta(nil)
	if err != nil || v != nil {
		t.Fatalf("meta nil harus jadi NULL: %v %v", v, err)
	}
	v2, err := encodeMeta(map[string]any{})
	if err != nil || v2 != nil {
		t.Fatalf("meta kosong harus jadi NULL: %v %v", v2, err)
	}
	v3, err := encodeMeta(map[string]any{"actions_used": []string{ActionTimelinePost}})
	if err != nil {
		t.Fatalf("encode gagal: %v", err)
	}
	s, ok := v3.(string)
	if !ok || s == "" {
		t.Fatalf("meta berisi harus jadi string JSON: %#v", v3)
	}
	var back map[string]any
	if err := json.Unmarshal([]byte(s), &back); err != nil {
		t.Fatalf("JSON hasil encode tidak valid: %v", err)
	}
}

// Aksi yang ditolak validasi tidak boleh sampai ke meta, tapi balasan tetap
// dikirim. Ini menjaga satu usulan ngawur tidak menghapus jawaban AI.
func TestChatbotKeepsReplyWhenActionInvalid(t *testing.T) {
	raw := `{
	  "reply": "Baik, aku catat.",
	  "field_updates": [], "suggested_values": [],
	  "asked_field": null, "ready_for_confirmation": false,
	  "actions": [{"kind":"hapus_semua_data","label":"Hapus","payload":{"caption":"x"}}]
	}`
	out, err := parseEngineOutput(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	actions := ValidateActions(out.Actions)
	if len(actions) != 0 {
		t.Fatalf("kind tak dikenal harus dibuang, dapat %#v", actions)
	}

	// Meniru executeTurn: meta hanya diisi kalau ada aksi yang lolos.
	meta := map[string]any{"ready_for_confirmation": false}
	if len(actions) > 0 {
		meta["actions"] = actions
	}
	if _, ok := meta["actions"]; ok {
		t.Fatalf("meta tidak boleh berisi actions kosong")
	}
	if out.Reply == "" {
		t.Fatalf("balasan harus tetap ada")
	}
	row := assistantMessageRow(message{ID: "m-4", Role: "assistant", Content: out.Reply, Meta: meta})
	if row["content"] == "" {
		t.Fatalf("baris pesan kehilangan isi")
	}
}

// Blok aksi HARUS masuk system prompt tiap giliran, bukan hanya ke kontrak
// pratinjau admin. Prompt aktif di database punya blok FORMAT OUTPUT sendiri
// yang tidak memuat "actions"; tanpa suntikan ini model tidak pernah tahu
// field itu ada dan tombol Timeline tidak pernah muncul.
func TestActionBlockInjectedIntoSystemPrompt(t *testing.T) {
	tr := &turn{
		Config: &briefConfig{}, Prompt: "# PROMPT DARI DATABASE\n\n{ \"reply\": \"...\" }",
		Fields: testFields(), States: map[string]FieldState{}, Values: map[string]any{},
		History: []message{{Role: "user", Content: "ada promo 20% minggu ini"}},
	}
	msgs := tr.buildMessages()
	if len(msgs) == 0 {
		t.Fatalf("buildMessages tidak menghasilkan pesan")
	}
	system := msgs[0].Content
	if msgs[0].Role != "system" {
		t.Fatalf("pesan pertama harus system, dapat %q", msgs[0].Role)
	}
	// Prompt asli tetap utuh.
	if !strings.Contains(system, "PROMPT DARI DATABASE") {
		t.Fatalf("prompt asli hilang dari system")
	}
	// Blok aksi ikut.
	if !strings.Contains(system, `"actions"`) {
		t.Fatalf("system prompt tidak memuat key actions")
	}
	if !strings.Contains(system, ActionTimelinePost) {
		t.Fatalf("system prompt tidak menyebut kind %s", ActionTimelinePost)
	}
	if !strings.Contains(system, "TIMELINE TERBARU") {
		t.Fatalf("system prompt tidak merujuk daftar anti-duplikat")
	}
}

// Prompt kosong tetap mendapat blok aksi; tanpa itu instalasi baru tidak akan
// pernah menampilkan tombol.
func TestActionBlockInjectedWhenPromptEmpty(t *testing.T) {
	tr := &turn{
		Config: &briefConfig{}, Prompt: "",
		Fields: testFields(), States: map[string]FieldState{}, Values: map[string]any{},
		History: []message{{Role: "user", Content: "promo baru"}},
	}
	system := tr.buildMessages()[0].Content
	if !strings.Contains(system, `"actions"`) {
		t.Fatalf("prompt kosong tidak mendapat blok aksi")
	}
}

// Timeline terbaru harus sampai ke model lewat pesan user terakhir, karena di
// situlah blok state ditempelkan.
func TestTimelineReachesUserMessage(t *testing.T) {
	tr := &turn{
		Config: &briefConfig{}, Prompt: "p",
		Fields: testFields(), States: map[string]FieldState{}, Values: map[string]any{},
		History:  []message{{Role: "user", Content: "ada promo baru"}},
		Timeline: []string{"Promo Lebaran 15% (sudah tayang)"},
	}
	msgs := tr.buildMessages()
	last := msgs[len(msgs)-1]
	if last.Role != "user" {
		t.Fatalf("pesan terakhir harus user")
	}
	if !strings.Contains(last.Content, "TIMELINE TERBARU") {
		t.Fatalf("blok timeline tidak menempel di pesan user terakhir")
	}
	if !strings.Contains(last.Content, "Promo Lebaran 15%") {
		t.Fatalf("caption timeline tidak ikut ke model")
	}
}

// Blok aksi harus berada DI DALAM bagian format, bukan menumpuk di akhir
// setelah belasan aturan perilaku. Model mengikuti skema JSON pertama yang ia
// baca, jadi posisi menentukan apakah field baru terpakai.
func TestActionBlockSitsInsideFormatSection(t *testing.T) {
	prompt := "# PERAN\nkamu asisten\n\n# FORMAT OUTPUT — WAJIB JSON\n{ \"reply\": \"...\" }\n\n# PRINSIP PERILAKU\n1. ekstrak dulu\n2. tanya satu per giliran\n"
	out := injectAfterOutputFormat(prompt, "# TAMBAHAN\n\"actions\"")
	idxFormat := strings.Index(out, "# FORMAT OUTPUT")
	idxBlock := strings.Index(out, "# TAMBAHAN")
	idxPrinciple := strings.Index(out, "# PRINSIP PERILAKU")
	if idxFormat < 0 || idxBlock < 0 || idxPrinciple < 0 {
		t.Fatalf("penanda hilang: format=%d blok=%d prinsip=%d", idxFormat, idxBlock, idxPrinciple)
	}
	if !(idxFormat < idxBlock && idxBlock < idxPrinciple) {
		t.Fatalf("blok aksi tidak berada di antara FORMAT OUTPUT dan bagian berikutnya")
	}
	// Prompt asli tidak boleh hilang.
	if !strings.Contains(out, "kamu asisten") || !strings.Contains(out, "ekstrak dulu") {
		t.Fatalf("isi prompt asli hilang")
	}
}

// Tanpa penanda FORMAT OUTPUT (prompt kustom), blok tetap disisipkan di akhir
// supaya perilakunya terdefinisi, bukan hilang diam-diam.
func TestActionBlockFallbackWithoutMarker(t *testing.T) {
	out := injectAfterOutputFormat("prompt pendek tanpa penanda", "# TAMBAHAN\n\"actions\"")
	if !strings.Contains(out, "prompt pendek tanpa penanda") {
		t.Fatalf("prompt asli hilang")
	}
	if !strings.Contains(out, "\"actions\"") {
		t.Fatalf("blok aksi tidak disisipkan saat penanda tidak ada")
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "\"actions\"") {
		t.Fatalf("blok fallback harus di akhir")
	}
}

// Tombol aksi hanya bisa diklik kalau respons giliran membawa message_id.
// Sebelumnya insertMessage membuat id di dalam fungsi tanpa mengembalikannya,
// sehingga respons "done" tidak punya id dan tombol mati senyap.
func TestTurnResponseCarriesMessageID(t *testing.T) {
	out := &EngineOutput{Reply: "Kabar baik, siap ditayangkan."}
	tr := &turnResult{
		Output: out,
		Actions: []ActionView{{
			Kind: ActionTimelinePost, Label: "Tambahkan ke Timeline",
			Caption: "Teh Anget buka cabang baru di Dago.",
		}},
		MessageID: "msg-abc",
	}
	resp := tr.response()
	if got, _ := resp["message_id"].(string); got != "msg-abc" {
		t.Fatalf("message_id tidak ikut di respons: %#v", resp["message_id"])
	}
	actions, ok := resp["actions"].([]ActionView)
	if !ok || len(actions) != 1 {
		t.Fatalf("actions tidak ikut di respons: %#v", resp["actions"])
	}
	// Frontend memakai pasangan (message_id, kind) untuk endpoint consume.
	if actions[0].Kind == "" {
		t.Fatalf("kind kosong, endpoint consume akan menolak")
	}
}

// Giliran tanpa aksi tetap mengirim message_id, karena pesan itu juga bisa
// dirujuk nanti (mis. penandaan aksi setelah halaman dimuat ulang).
func TestTurnResponseMessageIDWithoutActions(t *testing.T) {
	tr := &turnResult{Output: &EngineOutput{Reply: "Halo"}, MessageID: "msg-def"}
	resp := tr.response()
	if got, _ := resp["message_id"].(string); got != "msg-def" {
		t.Fatalf("message_id hilang saat tidak ada aksi")
	}
	// Nil slice diserialisasi menjadi null, bukan []. Frontend memeriksa
	// kebenaran nilai, jadi keduanya aman; yang penting tidak ada ActionView
	// sampah dan bentuk JSON-nya tetap bisa dibaca.
	if v, ok := resp["actions"].([]ActionView); ok && len(v) != 0 {
		t.Fatalf("actions harus kosong saat tidak ada aksi: %#v", v)
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("respons tidak bisa diserialisasi: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("JSON respons tidak valid: %v", err)
	}
	if back["message_id"] != "msg-def" {
		t.Fatalf("message_id hilang setelah serialisasi")
	}
	if back["actions"] != nil {
		t.Fatalf("actions harus null di JSON saat tidak ada aksi, dapat %#v", back["actions"])
	}
}
