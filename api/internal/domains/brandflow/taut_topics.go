package brandflow

import (
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/produce/topics ----------

// tautTopicSuggestion: satu usulan topik untuk satu slot artikel.
type tautTopicSuggestion struct {
	Slot  int    `json:"slot"`
	Topic string `json:"topic"`
}

// tautTopicSystemPrompt: instruksi tetap untuk penyusun topik. Sengaja pendek:
// yang menentukan hasil adalah konteks kampanye di prompt user, bukan aturan
// panjang di sini.
const tautTopicSystemPrompt = `Kamu editor konten PR senior di Indonesia. Tugasmu hanya menyusun daftar topik/judul artikel, bukan menulis artikelnya. Jawab HANYA dengan JSON valid.`

// TautSuggestTopics: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/produce/topics
//
// Menyusun tepat satu topik untuk tiap artikel yang belum dibuat pada satu
// media, dari konteks singkat klien + nama angle masing-masing. Satu panggilan
// LLM untuk semua topik, bukan satu panggilan per artikel.
//
// Dipakai dialog "Buat N artikel": klien mengisi konteks sekali (produk,
// pembaca, pesan utama) lalu menekan satu tombol untuk mengisi seluruh kolom
// topik. Kalau model gagal, topik tetap dikembalikan lewat susunan
// deterministik (konteks + nama angle) supaya dialog tidak buntu.
func (d *Deps) TautSuggestTopics(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	camp, err := d.queryRow(r.Context(),
		"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if camp == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}

	body := decodeJSON(r)
	product, _, _ := optionalTrimmedStringScalar(body, "product", 300)
	audience, _, _ := optionalTrimmedStringScalar(body, "audience", 300)
	message, _, _ := optionalTrimmedStringScalar(body, "message", 600)
	mediaName, _, _ := optionalTrimmedStringScalar(body, "media_name", 200)

	angles := []tautTopicAngle{}
	if raw, ok := body["angles"].([]any); ok {
		if len(raw) > 20 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Array must contain at most 20 element(s)")
			return
		}
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			slot := 0
			if v, ok := m["slot"].(float64); ok {
				slot = int(v)
			}
			if slot < 1 {
				continue
			}
			name, _ := m["name"].(string)
			slug, _ := m["slug"].(string)
			angles = append(angles, tautTopicAngle{
				Slot: slot,
				Name: truncateString(strings.TrimSpace(name), 160),
				Slug: truncateString(strings.TrimSpace(slug), 160),
			})
		}
	}
	if len(angles) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Minimal satu artikel dibutuhkan.")
		return
	}
	// Konteks kosong total: topik tidak mungkin disusun dari apa pun.
	if product == "" && audience == "" && message == "" {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Isi minimal produk/jasa atau pesan utama dulu supaya topik bisa disusun.")
		return
	}

	prompt := buildTautTopicPrompt(mediaName, product, audience, message, angles)
	result, llmErr := d.runTautAgentSystem(r.Context(), "writer", campaignID, prompt, nil, true, tautTopicSystemPrompt, nil)

	fallback := false
	suggestions := []tautTopicSuggestion{}
	if llmErr == nil {
		suggestions = parseTautTopicSuggestions(result.Content, angles)
	}
	if len(suggestions) == 0 {
		// Model gagal atau jawabannya tidak bisa dipakai: jangan buntu, susun
		// topik dasar dari konteks + angle.
		fallback = true
		suggestions = deterministicTautTopics(product, message, angles)
	}

	writeNoStore(w, http.StatusOK, map[string]any{
		"topics":   suggestions,
		"fallback": fallback,
	})
}

// tautTopicAngle: satu artikel yang butuh topik.
type tautTopicAngle struct {
	Slot int
	Name string
	Slug string
}

func buildTautTopicPrompt(mediaName, product, audience, message string, angles []tautTopicAngle) string {
	var b strings.Builder
	b.WriteString("Susun topik artikel untuk beberapa artikel yang akan terbit di MEDIA YANG SAMA.\n")
	b.WriteString("Setiap artikel harus benar-benar berbeda sudut bahasannya — bukan topik yang sama dengan kata lain.\n\n")
	if mediaName != "" {
		b.WriteString("Media tujuan: " + mediaName + "\n")
	}
	b.WriteString("Konteks dari klien:\n")
	if product != "" {
		b.WriteString("- Produk/jasa: " + product + "\n")
	}
	if audience != "" {
		b.WriteString("- Target pembaca: " + audience + "\n")
	}
	if message != "" {
		b.WriteString("- Pesan utama: " + message + "\n")
	}
	b.WriteString("\nArtikel yang dibutuhkan (nomor = slot artikel):\n")
	for _, a := range angles {
		label := a.Name
		if label == "" {
			label = a.Slug
		}
		b.WriteString("- slot " + itoa(a.Slot) + ": angle \"" + label + "\"\n")
	}
	b.WriteString("\nAturan topik:\n")
	b.WriteString("1. Satu kalimat pendek, maksimal 20 kata, tanpa tanda kutip.\n")
	b.WriteString("2. Konkret dan spesifik ke konteks di atas; jangan mengarang angka, nama orang, atau klaim yang tidak ada di konteks.\n")
	b.WriteString("3. Tanpa clickbait, tanpa huruf kapital semua, tanpa emoji.\n")
	b.WriteString("4. Topik tiap slot harus mencerminkan angle-nya dan tidak mengulang topik slot lain.\n")
	b.WriteString("\nBalas JSON: {\"topics\":[{\"slot\":<nomor>,\"topic\":\"<topik>\"}]} dengan satu entri untuk SETIAP slot di atas.\n")
	return b.String()
}

// parseTautTopicSuggestions: ambil topik per slot dari jawaban model. Slot yang
// tidak dijawab model dibiarkan kosong (pemanggil mengisi sisanya).
func parseTautTopicSuggestions(content string, angles []tautTopicAngle) []tautTopicSuggestion {
	raw := extractJSONObject(content)
	if raw == nil {
		return nil
	}
	list, ok := raw["topics"].([]any)
	if !ok {
		return nil
	}
	wanted := map[int]bool{}
	for _, a := range angles {
		wanted[a.Slot] = true
	}
	seen := map[int]bool{}
	out := make([]tautTopicSuggestion, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		slot := 0
		if v, ok := m["slot"].(float64); ok {
			slot = int(v)
		}
		topic := ""
		if s, ok := m["topic"].(string); ok {
			topic = strings.TrimSpace(s)
		}
		if slot < 1 || !wanted[slot] || seen[slot] || topic == "" {
			continue
		}
		seen[slot] = true
		out = append(out, tautTopicSuggestion{Slot: slot, Topic: truncateString(topic, 300)})
	}
	return out
}

// deterministicTautTopics: susunan cadangan tanpa AI — konteks + nama angle.
func deterministicTautTopics(product, message string, angles []tautTopicAngle) []tautTopicSuggestion {
	subject := strings.TrimSpace(product)
	if subject == "" {
		subject = strings.TrimSpace(message)
	}
	if subject == "" {
		subject = "brand ini"
	}
	out := make([]tautTopicSuggestion, 0, len(angles))
	for _, a := range angles {
		label := a.Name
		if label == "" {
			label = a.Slug
		}
		topic := subject
		if message != "" && strings.TrimSpace(product) != "" {
			topic = subject + ": " + strings.TrimSpace(message)
		}
		if label != "" {
			topic = topic + " (" + label + ")"
		}
		out = append(out, tautTopicSuggestion{Slot: a.Slot, Topic: truncateString(topic, 300)})
	}
	return out
}

// optionalTrimmedStringScalar: baca string opsional dari body tanpa error.
func optionalTrimmedStringScalar(body map[string]any, key string, max int) (string, bool, error) {
	raw, ok := body[key]
	if !ok || raw == nil {
		return "", false, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", false, nil
	}
	return truncateString(strings.TrimSpace(s), max), true, nil
}
