// Package brief: chatbot penyusun Profiler brand (Taut Brief).
//
// Percakapan adalah kulitnya; otaknya adalah state JSON per field. Model hanya
// mengusulkan (field_updates), dan Go yang memutuskan apa yang boleh disimpan —
// key harus ada di skema, lapis 3 tidak boleh diisi LLM, dan status "confirmed"
// tidak boleh turun jadi "filled".
//
// Lapis brief (sesuai desain produk):
//
//	LAPIS 1 Entity Core   → kolom profiler_profiles.entity_core (wajib, inti GEO)
//	LAPIS 2 Brand Context → kolom identity / style / seo yang sudah ada
//	LAPIS 3 Campaign Memory → TIDAK disimpan; dihitung dari data pemakaian
package brief

import (
	"sort"
	"strconv"
	"strings"
)

// Field: satu field brief, berasal dari taut_brief_field_versions.fields.
type Field struct {
	Key             string `json:"key"`
	Layer           int    `json:"layer"`
	Section         string `json:"section"`
	Column          string `json:"column,omitempty"`
	Label           string `json:"label"`
	Widget          string `json:"widget"`
	Required        bool   `json:"required"`
	SortOrder       int    `json:"sort_order"`
	ExampleQuestion string `json:"example_question"`
	Enabled         bool   `json:"enabled"`
}

// WidgetKinds: kosakata widget yang dikenali renderer & validasi. Skema lama
// hanya memakai text/longtext; daftar ini yang jadi kontrak saat admin
// menambah field baru (number/url/select/multiselect/boolean).
var WidgetKinds = map[string]bool{
	"text":        true,
	"longtext":    true,
	"number":      true,
	"url":         true,
	"select":      true,
	"multiselect": true,
	"boolean":     true,
}

// NormalizeWidget: widget kosong/tak dikenal diperlakukan sebagai teks supaya
// skema lama tetap valid.
func NormalizeWidget(w string) string {
	w = strings.ToLower(strings.TrimSpace(w))
	if WidgetKinds[w] {
		return w
	}
	return "text"
}

// ValidateWidgetValue: tipe nilai (string dari UI/LLM) sesuai widget.
// Multiselect memakai pemisah ";" dan boolean menerima true/false/1/0/ya/tidak.
func ValidateWidgetValue(widget, value string) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}
	switch NormalizeWidget(widget) {
	case "number":
		_, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		return err == nil
	case "url":
		return strings.HasPrefix(strings.ToLower(v), "http://") || strings.HasPrefix(strings.ToLower(v), "https://")
	case "boolean":
		switch strings.ToLower(v) {
		case "true", "false", "1", "0", "ya", "tidak", "yes", "no":
			return true
		}
		return false
	case "multiselect":
		parts := strings.Split(v, ";")
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// sectionColumn: pemetaan bawaan seksi → kolom penyimpanan.
// Versi skema boleh menimpa lewat Field.Column bila seksi baru ditambahkan.
var sectionColumn = map[string]string{
	"Entity Core":          "entity_core",
	"Identitas Brand":      "identity",
	"Gaya Penulisan":       "style",
	"Pedoman Konten & SEO": "seo",
}

// StorageColumn: kolom profiler_profiles tempat nilai field ini disimpan.
func StorageColumn(f Field) (string, bool) {
	if c := strings.TrimSpace(f.Column); c != "" {
		return c, true
	}
	c, ok := sectionColumn[strings.TrimSpace(f.Section)]
	return c, ok
}

// FieldState: status + sumber sebuah field. Nilainya sendiri tinggal di
// profiler_profiles; tabel ini hanya menjawab "sudah diisi belum, dan dari mana".
type FieldState struct {
	Status string
	Source string
	Note   string
}

const (
	StatusEmpty     = "empty"
	StatusFilled    = "filled"
	StatusConfirmed = "confirmed"
)

const (
	SourceUser        = "user"
	SourceURL         = "url"
	SourceDocument    = "document"
	SourceAISuggested = "ai_suggested"
)

// FieldUpdate: satu usulan pengisian dari LLM.
type FieldUpdate struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Status string `json:"status"`
	Source string `json:"source"`
	Note   string `json:"note"`
}

// SuggestedValue: draft yang ditawarkan ke user. TIDAK disimpan sampai user
// menyetujuinya — draft tidak boleh masuk field_updates.
type SuggestedValue struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// EngineOutput: kontrak balasan LLM. Urutan field mengikuti urutan JSON yang
// diwajibkan ke model: "reply" PERTAMA, supaya parser inkremental bisa
// memancarkan balasan sebelum dokumen JSON selesai.
type EngineOutput struct {
	Reply                string           `json:"reply"`
	FieldUpdates         []FieldUpdate    `json:"field_updates"`
	SuggestedValues      []SuggestedValue `json:"suggested_values"`
	AskedField           *string          `json:"asked_field"`
	ReadyForConfirmation bool             `json:"ready_for_confirmation"`
}

var validSources = map[string]bool{
	SourceUser: true, SourceURL: true, SourceDocument: true, SourceAISuggested: true,
}

// ValidateUpdates menyaring usulan LLM. Yang lolos dikembalikan bersama alasan
// penolakan untuk field lainnya (dipakai log/debug, bukan ditampilkan ke user).
func ValidateUpdates(fields []Field, current map[string]FieldState, updates []FieldUpdate) (accepted []FieldUpdate, rejected []string) {
	byKey := make(map[string]Field, len(fields))
	for _, f := range fields {
		byKey[f.Key] = f
	}
	for _, u := range updates {
		key := strings.TrimSpace(u.Key)
		if key == "" {
			rejected = append(rejected, "key kosong")
			continue
		}
		f, ok := byKey[key]
		if !ok {
			rejected = append(rejected, key+": tidak ada di skema")
			continue
		}
		if !f.Enabled {
			rejected = append(rejected, key+": field nonaktif")
			continue
		}
		// Lapis 3 (Campaign Memory) diisi sistem, bukan LLM.
		if f.Layer == 3 {
			rejected = append(rejected, key+": lapis 3 diisi sistem")
			continue
		}
		if _, ok := StorageColumn(f); !ok {
			rejected = append(rejected, key+": seksi tanpa kolom penyimpanan")
			continue
		}
		value := strings.TrimSpace(u.Value)
		if value == "" {
			rejected = append(rejected, key+": nilai kosong")
			continue
		}
		// Tipe nilai harus cocok dengan widget field (number/url/boolean/…).
		// Tanpa ini skema bisa menyimpan "12rb" di field angka dan merusak
		// konteks AI Writer.
		if !ValidateWidgetValue(f.Widget, value) {
			rejected = append(rejected, key+": nilai tidak sesuai tipe "+NormalizeWidget(f.Widget))
			continue
		}
		status := strings.TrimSpace(u.Status)
		if status != StatusFilled && status != StatusConfirmed {
			status = StatusFilled
		}
		source := strings.TrimSpace(u.Source)
		if !validSources[source] {
			source = SourceUser
		}
		// status field yang sudah dikonfirmasi tidak boleh turun tanpa permintaan
		// user. Perubahan yang diminta user datang dengan status "confirmed".
		if prev, ok := current[key]; ok && prev.Status == StatusConfirmed && status == StatusFilled {
			rejected = append(rejected, key+": confirmed tidak boleh turun jadi filled")
			continue
		}
		accepted = append(accepted, FieldUpdate{
			Key: key, Value: value, Status: status, Source: source, Note: strings.TrimSpace(u.Note),
		})
	}
	return accepted, rejected
}

// Candidates: field wajib yang belum terisi, urut prioritas dari skema.
// Ini pengganti "biar LLM memilih mau tanya apa" — urutannya ditentukan config.
func Candidates(fields []Field, states map[string]FieldState, limit int) []string {
	todo := make([]Field, 0, len(fields))
	for _, f := range fields {
		if !f.Enabled || !f.Required || f.Layer == 3 {
			continue
		}
		if st, ok := states[f.Key]; ok && (st.Status == StatusFilled || st.Status == StatusConfirmed) {
			continue
		}
		todo = append(todo, f)
	}
	sort.SliceStable(todo, func(i, j int) bool { return todo[i].SortOrder < todo[j].SortOrder })
	out := make([]string, 0, len(todo))
	for _, f := range todo {
		out = append(out, f.Key)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Progress: berapa field wajib yang sudah terisi.
func Progress(fields []Field, states map[string]FieldState) (filled, total int) {
	for _, f := range fields {
		if !f.Enabled || !f.Required || f.Layer == 3 {
			continue
		}
		total++
		if st, ok := states[f.Key]; ok && (st.Status == StatusFilled || st.Status == StatusConfirmed) {
			filled++
		}
	}
	return filled, total
}

// Phase ditentukan backend dari state, bukan oleh LLM.
type Phase string

const (
	PhaseIntake     Phase = "intake"
	PhaseConfirm    Phase = "confirm"
	PhaseFill       Phase = "fill"
	PhaseReadyCheck Phase = "ready_check"
	PhaseDone       Phase = "done"
)

type PhaseInput struct {
	// Ada pesan user sama sekali belum (percakapan masih benar-benar baru).
	HasUserMessage bool
	// Material (dokumen/URL siap dibaca) sudah ada, dan kapan material terbaru masuk.
	HasMaterial bool
	MaterialAt  int64 // unix milli; 0 bila tidak ada
	// Pesan user terakhir, untuk tahu apakah ringkasan tangkapan sudah dibalas.
	LastUserAt int64
	// Pesan assistant terakhir menyatakan brief siap dikonfirmasi, dan kapan.
	AssistantAwaitingConfirm bool
	AwaitingConfirmAt        int64
	RequiredFilled           int
	RequiredTotal            int
}

// DeterminePhase menetapkan fase dari state. LLM tidak pernah memutuskan fasenya
// sendiri; ia hanya menerima hasilnya.
//
// Catatan: "done" bersifat kosmetik — brief tetap bisa dikoreksi kapan saja lewat
// panel Ringkasan. Kalau model pada giliran berikutnya menyatakan belum siap,
// fase otomatis kembali ke fill.
func DeterminePhase(in PhaseInput) Phase {
	if in.AssistantAwaitingConfirm {
		if in.LastUserAt > in.AwaitingConfirmAt {
			return PhaseDone
		}
		return PhaseReadyCheck
	}
	if in.RequiredTotal > 0 && in.RequiredFilled >= in.RequiredTotal {
		return PhaseReadyCheck
	}
	if !in.HasUserMessage {
		return PhaseIntake
	}
	if in.HasMaterial && in.MaterialAt > 0 && in.LastUserAt < in.MaterialAt {
		return PhaseConfirm
	}
	return PhaseFill
}
