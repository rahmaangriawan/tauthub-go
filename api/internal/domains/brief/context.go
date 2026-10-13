package brief

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Batas ukuran blok yang disuntik ke user message. Variabel per giliran masuk
// ke PESAN USER, bukan system prompt, supaya system prompt tetap statis dan
// bisa diverifikasi/di-versi terpisah.
const (
	maxSchemaChars  = 6000
	maxStateChars   = 4000
	maxSourcesChars = 5000
	maxSourceItem   = 1200
)

// StateView: seluruh yang dibutuhkan untuk merakit satu giliran.
type StateView struct {
	Fields      []Field
	States      map[string]FieldState
	Values      map[string]any // key field → nilai saat ini (dari jsonb)
	Phase       Phase
	Sources     []SourceItem
	UserName    string
	ProfileName string
	// AskedField: field yang AI tanyakan giliran sebelumnya. Dipakai supaya
	// jawaban pendek user (satu kata pun) menempel ke pertanyaan itu — bukan
	// dinilai sebagai pesan yang berdiri sendiri lalu ditolak/tanya ulang.
	AskedField string
}

// SourceItem: potongan dokumen/URL yang sudah dibaca sistem.
type SourceItem struct {
	Kind  string // "dokumen" | "url"
	Label string
	Text  string
}

// BuildUserMessage merakit blok konteks per giliran. Riwayat percakapan dikirim
// terpisah sebagai pesan user/assistant biasa, jadi tidak diulang di sini.
func BuildUserMessage(v StateView) string {
	filled, total := Progress(v.Fields, v.States)
	candidates := Candidates(v.Fields, v.States, 3)

	var b strings.Builder
	b.WriteString("Konteks brief (dari sistem, bukan dari user):\n\n")
	b.WriteString("User : " + orDash(v.UserName) + "\n")
	b.WriteString("Brief: " + orDash(v.ProfileName) + "\n")
	b.WriteString("Fase saat ini : " + string(v.Phase) + "\n")
	b.WriteString(fmt.Sprintf("Progress : %d dari %d field wajib terisi\n", filled, total))

	if v.AskedField != "" {
		b.WriteString("\nPertanyaan TERAKHIR yang kamu ajukan (asked_field): " + v.AskedField + "\n")
		b.WriteString("→ Pesan user pada giliran ini adalah jawaban atas pertanyaan itu. Terima jawaban singkat, informal, bahkan satu kata untuk field tersebut; isi field-nya dan lanjutkan. Jangan menganggapnya typo, jangan minta diulang, jangan minta konfirmasi — kecuali benar-benar ambigu dan berdampak pada hasil.\n")
	} else {
		b.WriteString("\nBelum ada pertanyaan menggantung dari giliran sebelumnya.\n")
	}

	b.WriteString("\n## Schema field\n")
	b.WriteString(clip(schemaBlock(v.Fields), maxSchemaChars))

	b.WriteString("\n\n## State brief saat ini\n")
	b.WriteString(clip(stateBlock(v.Fields, v.States, v.Values), maxStateChars))

	b.WriteString("\n\n## Field yang perlu ditanyakan (urutan prioritas)\n")
	if len(candidates) == 0 {
		b.WriteString("(tidak ada — semua field wajib sudah terisi)\n")
	} else {
		for i, key := range candidates {
			if f, ok := fieldByKey(v.Fields, key); ok {
				b.WriteString(fmt.Sprintf("%d. %s — %s\n", i+1, key, f.Label))
			} else {
				b.WriteString(fmt.Sprintf("%d. %s\n", i+1, key))
			}
		}
	}

	b.WriteString("\n## Dokumen & website yang sudah dibaca sistem\n")
	b.WriteString(clip(sourceBlock(v.Sources), maxSourcesChars))
	return b.String()
}

func schemaBlock(fields []Field) string {
	ordered := enabledFields(fields)
	var b strings.Builder
	section := ""
	for _, f := range ordered {
		if f.Section != section {
			section = f.Section
			b.WriteString("\n[" + section + "]\n")
		}
		req := "opsional"
		if f.Required {
			req = "WAJIB"
		}
		b.WriteString("- " + f.Key + " (" + f.Label + ", " + req)
		if w := strings.TrimSpace(f.Widget); w != "" {
			b.WriteString(", bentuk: " + w)
		}
		b.WriteString(")")
		if q := strings.TrimSpace(f.ExampleQuestion); q != "" {
			b.WriteString("\n  contoh pertanyaan: " + q)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func stateBlock(fields []Field, states map[string]FieldState, values map[string]any) string {
	ordered := enabledFields(fields)
	var b strings.Builder
	for _, f := range ordered {
		st, ok := states[f.Key]
		status := StatusEmpty
		source := "-"
		note := ""
		if ok {
			status = st.Status
			if st.Source != "" {
				source = st.Source
			}
			note = st.Note
		}
		value := renderValue(values[f.Key])
		if value == "" {
			value = "(kosong)"
		}
		line := fmt.Sprintf("%-22s | %-9s | %-12s | %s", f.Key, status, source, value)
		if note != "" {
			line += " (" + note + ")"
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func sourceBlock(items []SourceItem) string {
	if len(items) == 0 {
		return "(belum ada dokumen atau website yang selesai dibaca)\n"
	}
	var b strings.Builder
	for _, s := range items {
		b.WriteString("\n[" + strings.ToUpper(s.Kind) + ": " + s.Label + "]\n")
		b.WriteString(clip(strings.TrimSpace(s.Text), maxSourceItem))
		b.WriteString("\n")
	}
	return b.String()
}

// RenderOutputJSON memaksa bentuk JSON yang diminta dari model. Dipakai saat
// membangun prompt percobaan di halaman admin.
func RenderOutputContract() string {
	return `{
  "reply": "teks balasan untuk user (Markdown ringan). Letakkan key ini PERTAMA.",
  "field_updates": [
    { "key": "nama_field", "value": "nilai", "status": "filled | confirmed",
      "source": "user | url | document | ai_suggested", "note": "alasan singkat (opsional)" }
  ],
  "suggested_values": [
    { "key": "nama_field", "value": "draft yang kamu tawarkan", "reason": "kenapa" }
  ],
  "asked_field": "key field yang kamu tanyakan di reply, atau null",
  "ready_for_confirmation": false
}`
}

// ---------- util ----------

func enabledFields(fields []Field) []Field {
	out := make([]Field, 0, len(fields))
	for _, f := range fields {
		if f.Enabled && f.Layer != 3 {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

func fieldByKey(fields []Field, key string) (Field, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

func renderValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "ya"
		}
		return "tidak"
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := renderValue(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		raw, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(raw)
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n…(dipotong)"
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
