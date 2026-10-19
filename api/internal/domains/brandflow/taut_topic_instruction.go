package brandflow

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/produce/topic-instruction ----------

type tautAnchorRow struct {
	Anchor string
	URL    string
}

// TautTopicInstruction: racik instruksi topik otomatis dari anchor yang
// diisi mitra/klien di modal Pengelola Naskah. AI bebas memakai model yang
// sudah dikonfigurasi admin (runtime aktif); yang penting AI memahami
// konteks anchor: judul/arah bahasan WAJIB berbeda dari anchor text —
// memakai kata kunci semantik & kaidah SEO/EEAT — dan bila ada tautan,
// instruksi menuntut penyisipan yang natural.
func (d *Deps) TautTopicInstruction(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	mediaType, _ := strField(body, "media_type")
	rawAnchors, hasAnchors := body["anchors"]
	if !hasAnchors {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid request")
		return
	}
	anchors := make([]tautAnchorRow, 0, 2)
	if list, ok := rawAnchors.([]any); ok {
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			anchor, _ := m["anchor"].(string)
			url, _ := m["url"].(string)
			anchor = strings.TrimSpace(anchor)
			if anchor == "" {
				continue
			}
			anchors = append(anchors, tautAnchorRow{Anchor: anchor, URL: strings.TrimSpace(url)})
		}
	}
	if len(anchors) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi minimal 1 anchor teks di kolom Tautan & Anchor.")
		return
	}
	if len(anchors) > 5 {
		anchors = anchors[:5]
	}

	var anchorLines strings.Builder
	for i, a := range anchors {
		if a.URL != "" {
			anchorLines.WriteString(itoa(i+1) + ". anchor: \"" + a.Anchor + "\" -> " + a.URL + "\n")
		} else {
			anchorLines.WriteString(itoa(i+1) + ". anchor: \"" + a.Anchor + "\"\n")
		}
	}

	mediaLabel := "mainstream"
	if strings.EqualFold(mediaType, "blogger") {
		mediaLabel = "blogger"
	}

	prompt := strings.Join([]string{
		"Kamu perancang instruksi penulisan artikel untuk platform Tauthub (bahasa Indonesia).",
		"Berdasarkan tautan/anchor berikut yang diisi user:",
		anchorLines.String(),
		"Jenis media target: " + mediaLabel + ".",
		"",
		"Tulis SATU instruksi penulisan (1-2 kalimat) berisi:",
		"1. Arah bahasan & tema judul artikel — WAJIB BERBEDA dari anchor text di atas; jangan menyalin anchor persis sebagai judul. Pakai kata kunci semantik yang relevan lalu terhubung ke niat pencarian (search intent), dan cermin kaidah SEO & E-E-A-T (keahlian/pengalaman/otoritas topik).",
		"2. Arahan penyisipan tautan secara natural di bagian yang relevan HANYA bila ada URL di daftar; kalau anchor tidak punya URL, tetap sebut mengikuti konteksnya.",
		"3. Gaya mengikuti jenis media: mainstream = berita/feature objektif kaidah berita; blogger = santai mengalir dengan heading.",
		"",
		"Jangan mengarang fakta, angka, harga, atau nama produk yang tidak ada di daftar anchor.",
		"Kembalikan JSON valid persis bentuk: {\"instruction\": \"...\"}. Tanpa penjelasan di luar JSON.",
	}, "\n")

	runtime, err := ai.LoadAIRuntime(r.Context(), d.Pool)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !ai.IsAIConfigured(runtime) {
		httpx.Error(w, http.StatusBadGateway, "AI belum dikonfigurasi. Atur AI di admin Tauthub.")
		return
	}
	result, err := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt:   prompt,
		Tier:     "economy",
		TaskType: "topic_instruction",
		BrandID:  &brandRow.ID,
		UserID:   &user.ID,
		Runtime:  runtime,
	})
	if err != nil {
		slog.Warn("[taut] racik instruksi gagal", "err", err)
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}

	text := strings.TrimSpace(result.Content)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(text[start:end+1]), &parsed) != nil {
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	instruction, _ := parsed["instruction"].(string)
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	if len(instruction) > 600 {
		instruction = instruction[:600]
	}
	writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "instruction": instruction})
}
