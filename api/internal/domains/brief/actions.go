package brief

// actions.go: aksi yang boleh ditawarkan model sebagai tombol di balasan.
//
// Model tidak mengeksekusi apa pun. Ia hanya mengusulkan aksi lewat kontrak
// JSON, backend menyaringnya ke allowlist di bawah, lalu frontend menampilkan
// tombol. Semua efek samping tetap terjadi lewat klik user ke endpoint yang
// sudah ada.
//
// Kenapa perlu allowlist, bukan meneruskan apa saja: kontrak JSON berasal dari
// LLM. Tanpa penyaringan, model bisa mengarang jenis aksi yang tidak dikenal
// frontend (tombol mati) atau mengirim payload raksasa. Penyaringan di sini
// membuat kesalahan model berakhir sebagai "aksi dibuang", bukan error.

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Batas payload aksi. Nilainya mengikuti batas endpoint tujuan, bukan angka
// baru: caption timeline maksimal 2000 karakter dan hashtag maksimal 10.
const (
	ActionCaptionMax  = 2000
	ActionHashtagsMax = 10
	ActionHashtagLen  = 40
	ActionLabelMax    = 60
	ActionReasonMax   = 200
	ActionPerReplyMax = 3
)

// AksiKind adalah jenis aksi yang dikenal frontend.
const (
	// ActionTimelinePost: tawarkan tombol "Tambahkan ke Timeline".
	ActionTimelinePost = "timeline_post"
)

// allowedActions: satu-satunya sumber jenis aksi yang boleh lolos.
var allowedActions = map[string]bool{
	ActionTimelinePost: true,
}

// TimelineActionPayload: isi aksi yang siap dikirim ke POST .../timeline.
type TimelineActionPayload struct {
	Caption    string   `json:"caption"`
	Hashtags   []string `json:"hashtags,omitempty"`
	OccurredOn string   `json:"occurred_on,omitempty"`
}

// EngineAction: satu aksi yang diusulkan model.
type EngineAction struct {
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Payload struct {
		Caption    string   `json:"caption"`
		Hashtags   []string `json:"hashtags"`
		OccurredOn string   `json:"occurred_on"`
	} `json:"payload"`
	Reason string `json:"reason"`
}

// ActionView: aksi yang sudah lolos validasi dan siap dikirim ke frontend.
// Bentuknya sengaja rata (bukan bersarang) supaya frontend tidak perlu tahu
// struktur internal model.
type ActionView struct {
	Kind       string   `json:"kind"`
	Label      string   `json:"label"`
	Caption    string   `json:"caption,omitempty"`
	Hashtags   []string `json:"hashtags,omitempty"`
	OccurredOn string   `json:"occurred_on,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

// ValidateActions menyaring usulan aksi dari model.
//
// Aksi yang lolos: kind dikenal, payload lengkap dan sesuai batas, label wajar.
// Sisanya dibuang tanpa menggagalkan giliran — satu aksi ngawur tidak boleh
// membuat balasan AI hilang.
func ValidateActions(raw []EngineAction) []ActionView {
	if len(raw) == 0 {
		return nil
	}
	out := make([]ActionView, 0, len(raw))
	seen := map[string]bool{}
	for _, a := range raw {
		if len(out) >= ActionPerReplyMax {
			break
		}
		kind := strings.TrimSpace(a.Kind)
		if !allowedActions[kind] {
			continue
		}
		switch kind {
		case ActionTimelinePost:
			view, ok := validateTimelineAction(a)
			if !ok {
				continue
			}
			// Dedupe per caption: model kadang mengulang usulan yang sama.
			key := kind + "|" + strings.ToLower(view.Caption)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, view)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func validateTimelineAction(a EngineAction) (ActionView, bool) {
	caption := strings.TrimSpace(a.Payload.Caption)
	if caption == "" {
		return ActionView{}, false
	}
	if utf8.RuneCountInString(caption) > ActionCaptionMax {
		return ActionView{}, false
	}
	tags := normalizeActionHashtags(a.Payload.Hashtags)
	occurred := strings.TrimSpace(a.Payload.OccurredOn)
	if occurred != "" {
		if _, err := time.Parse("2006-01-02", occurred); err != nil {
			// Tanggal tidak valid bukan alasan membuang aksi: caption-nya
			// tetap berguna, tanggalnya saja yang dilewati.
			occurred = ""
		}
	}
	label := strings.TrimSpace(a.Label)
	if label == "" {
		label = "Tambahkan ke Timeline"
	}
	if utf8.RuneCountInString(label) > ActionLabelMax {
		label = "Tambahkan ke Timeline"
	}
	reason := strings.TrimSpace(a.Reason)
	if utf8.RuneCountInString(reason) > ActionReasonMax {
		reason = truncateRunes(reason, ActionReasonMax)
	}
	return ActionView{
		Kind: ActionTimelinePost, Label: label, Caption: caption,
		Hashtags: tags, OccurredOn: occurred, Reason: reason,
	}, true
}

// normalizeActionHashtags merapikan tagar dari model: buang "#", spasi, dan
// duplikat; batasi jumlah serta panjang per tagar.
func normalizeActionHashtags(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, h := range raw {
		h = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(h), "#"))
		if h == "" {
			continue
		}
		if utf8.RuneCountInString(h) > ActionHashtagLen {
			continue
		}
		lower := strings.ToLower(h)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		out = append(out, h)
		if len(out) >= ActionHashtagsMax {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
