package brief

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

// Orkestrasi satu giliran brief.
//
// Alur: rakit konteks → panggil model (JSON mode, sedapat mungkin sambil
// streaming) → validasi field_updates di Go → tulis nilai ke profiler_profiles
// → simpan pesan + log run. Model hanya mengusulkan; yang menyimpan adalah Go.

const maxHistoryMessages = 20

type turn struct {
	// AskedField: field yang ditanyakan giliran sebelumnya (dari meta pesan
	// assistant terakhir) — diteruskan ke konteks agar jawaban pendek user
	// dikaitkan ke pertanyaan itu.
	AskedField   string
	Config       *briefConfig
	Prompt       string
	Fields       []Field
	FieldVersion int
	States       map[string]FieldState
	Values       map[string]any
	Phase        Phase
	Sources      []SourceItem
	History      []message
	ProfileName  string
	UserName     string
	// ChatSummary: ringkasan bergulir percakapan (kolom profiler_profiles.
	// chat_summary) — jembatan bagi pesan-pesan yang sudah keluar dari
	// jendela riwayat maxHistoryMessages.
	ChatSummary string
	// Memory: fakta relevan dari brief LAIN di brand yang sama (recall
	// embedding lokal atas pesan user terakhir).
	Memory []MemoryFact
	// Timeline: caption post Timeline terbaru. Diberikan ke model supaya ia
	// tidak mengusulkan aksi untuk kabar yang sudah tayang.
	Timeline []string
}

func (d *Deps) loadTurn(ctx context.Context, profileID, brandID, userName string) (*turn, error) {
	cfg, err := d.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	prompt, err := d.loadActivePrompt(ctx)
	if err != nil {
		return nil, err
	}
	fields, fieldVersion, err := d.loadActiveFieldSet(ctx)
	if err != nil {
		return nil, err
	}
	name, _, err := d.loadProfileMeta(ctx, profileID, brandID)
	if err != nil {
		return nil, err
	}
	states, err := d.loadStates(ctx, profileID)
	if err != nil {
		return nil, err
	}
	values, err := d.loadValues(ctx, profileID)
	if err != nil {
		return nil, err
	}
	sources, materialAt := d.loadSources(ctx, profileID)
	history, err := d.loadMessages(ctx, profileID, 60)
	if err != nil {
		return nil, err
	}
	lastUserAt, hasUser := d.lastUserAt(ctx, profileID)
	awaiting, awaitingAt, askedField := d.lastAssistantMeta(ctx, profileID)
	filled, total := Progress(fields, states)

	t := &turn{
		Config: cfg, Prompt: prompt, Fields: fields, FieldVersion: fieldVersion,
		States: states, Values: values, Sources: sources, History: history,
		ProfileName: name, UserName: userName, AskedField: askedField,
		ChatSummary: d.loadChatSummary(ctx, profileID),
	}
	// Caption Timeline terbaru: bahan pembanding untuk aksi usulan. Gagal
	// membaca bukan alasan menggagalkan giliran — model hanya kehilangan
	// konteks anti-duplikat.
	t.Timeline = d.loadRecentTimeline(ctx, profileID, maxTimelineContext)
	// Memori lintas brief: pakai pesan user TERAKHIR sebagai query; field
	// yang sedang ditanyakan diberi boost saat pencocokan.
	t.Memory = d.RecallFacts(ctx, brandID, profileID, lastUserMessage(history), askedField, 5)
	t.Phase = DeterminePhase(PhaseInput{
		HasUserMessage:           hasUser,
		HasMaterial:              materialAt > 0,
		MaterialAt:               materialAt,
		LastUserAt:               lastUserAt,
		AssistantAwaitingConfirm: awaiting,
		AwaitingConfirmAt:        awaitingAt,
		RequiredFilled:           filled,
		RequiredTotal:            total,
	})
	return t, nil
}

// lastUserMessage: isi pesan user terbaru dari riwayat (query recall memori).
func lastUserMessage(history []message) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			return history[i].Content
		}
	}
	return ""
}

func (t *turn) stateView() StateView {
	return StateView{
		Fields: t.Fields, States: t.States, Values: t.Values,
		Phase: t.Phase, Sources: t.Sources, Memory: t.Memory,
		UserName: t.UserName, ProfileName: t.ProfileName,
		AskedField: t.AskedField, Timeline: t.Timeline,
	}
}

// buildMessages: system prompt statis + riwayat sebagai pesan user/assistant
// biasa. Blok state hanya ditempelkan pada pesan user TERAKHIR, karena state
// yang dikirim harus menggambarkan kondisi SEBELUM pesan itu diekstrak.
func (t *turn) buildMessages() []ai.ChatMessage {
	history := t.History
	dropped := 0
	if len(history) > maxHistoryMessages {
		dropped = len(history) - maxHistoryMessages
		history = history[len(history)-maxHistoryMessages:]
	}
	stateBlock := BuildUserMessage(t.stateView())

	msgs := make([]ai.ChatMessage, 0, len(history)+1)
	system := t.Prompt
	if strings.TrimSpace(system) == "" {
		system = "Kamu adalah Brief Assistant Tauthub. Balas dalam JSON sesuai kontrak yang diberikan."
	}
	// Blok aksi disuntikkan ke system prompt, bukan hanya ke kontrak pratinjau
	// admin. Prompt aktif hidup di tabel taut_brief_prompt_versions dan punya
	// blok "FORMAT OUTPUT" sendiri; tanpa suntikan ini model tidak pernah tahu
	// field "actions" ada, sehingga tombol Timeline tidak pernah muncul.
	//
	// Ditempel TEPAT SETELAH blok FORMAT OUTPUT, bukan di akhir prompt: model
	// mengikuti skema JSON pertama yang ia lihat, dan blok di akhir mudah
	// diabaikan setelah belasan aturan perilaku di antaranya.
	system = injectAfterOutputFormat(system, actionPromptBlock())
	// Pesan yang jatuh di luar jendela riwayat digantikan ringkasan bergulir
	// supaya giliran tetap tahu koreksi/preferensi/fakta awal percakapan.
	if dropped > 0 {
		system += "\n\n# PERCAKAPAN SEBELUM JENDELA RIWAYAT\n"
		if s := strings.TrimSpace(t.ChatSummary); s != "" {
			system += s + "\n"
		}
		system += "(" + strconv.Itoa(dropped) + " pesan paling awal tidak disertakan; ringkasan di atas menggantikannya. Jangan menanyakan ulang informasi yang sudah ada di ringkasan atau di state.)\n"
	}
	msgs = append(msgs, ai.ChatMessage{Role: "system", Content: system})

	last := len(history) - 1
	for i, m := range history {
		content := m.Content
		if i == last && m.Role == "user" {
			content = stateBlock + "\n\n---\n\nPesan user:\n" + m.Content
		}
		msgs = append(msgs, ai.ChatMessage{Role: m.Role, Content: content})
	}
	return msgs
}

func (t *turn) client() *ai.ChatClient {
	return &ai.ChatClient{
		BaseURL:     t.Config.BaseURL,
		APIKey:      t.Config.APIKey,
		Model:       t.Config.Model,
		Temperature: t.Config.Temperature,
		MaxTokens:   t.Config.MaxTokens,
		Timeout:     time.Duration(t.Config.TimeoutMS) * time.Millisecond,
	}
}

// engineRun: hasil satu panggilan engine.
type engineRun struct {
	Output   *EngineOutput
	Result   *ai.ChatResult
	Streamed bool
	// Rejected: field_updates yang ditolak validasi (untuk log, bukan user).
	Rejected []string
}

// runEngine menjalankan satu giliran. onReply dipanggil berkala dengan potongan
// teks balasan saat streaming aktif; bila nil, jalur non-streaming dipakai.
func (d *Deps) runEngine(ctx context.Context, t *turn, onReply func(string)) (*engineRun, error) {
	if !t.Config.configured() {
		return nil, errNotConfigured
	}
	msgs := t.buildMessages()
	client := t.client()
	run := &engineRun{}

	// Probe kapabilitas: kalau admin sudah membuktikan provider tidak bisa
	// JSON mode sambil streaming, jangan coba lagi tiap giliran.
	streamJSON, probed := t.Config.Capabilities["stream_json"].(bool)
	canStream := onReply != nil && (!probed || streamJSON)

	if canStream {
		parser := ai.NewJSONStringFieldStream("reply")
		result, err := client.CompleteStreamMessages(ctx, msgs, true, func(chunk string) {
			if delta := parser.Write(chunk); delta != "" {
				onReply(delta)
			}
		})
		if err == nil {
			if out, perr := parseEngineOutput(result.Content); perr == nil {
				run.Output = out
				run.Result = result
				run.Streamed = true
				return run, nil
			}
		}
		// Streaming gagal: sebagian balasan mungkin sudah terkirim ke klien.
		// Jangan ulangi panggilan streaming; jatuh ke jalur non-streaming dan
		// biarkan pemanggil memutuskan apakah potongan tadi perlu diabaikan.
	}

	result, err := client.CompleteMessages(ctx, msgs, true)
	if err != nil {
		return nil, err
	}
	out, err := parseEngineOutput(result.Content)
	if err != nil {
		return nil, err
	}
	run.Output = out
	run.Result = result
	return run, nil
}

// parseEngineOutput membaca kontrak JSON dari model, toleran terhadap pagar
// kode atau teks pembungkus.
func parseEngineOutput(raw string) (*EngineOutput, error) {
	body := extractJSONObjectText(raw)
	if body == "" {
		return nil, errBadOutput
	}
	var out EngineOutput
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, errBadOutput
	}
	return &out, nil
}

func extractJSONObjectText(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = s[nl+1:]
		}
		if end := strings.LastIndex(s, "```"); end >= 0 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}
