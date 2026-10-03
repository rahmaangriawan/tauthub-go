package brandflow

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// taut_doc_cleanup.go: perapian naskah hasil unggahan dokumen (tab "Unggah
// Dokumen") dengan AI sebelum dimasukkan ke editor. Tanpa pemotongan kredit —
// hanya mencatat pemakaian model lewat usage ledger di ai.GenerateChat.

// docCleanupMaxChars: batas teks yang dikirim ke model (potong aman).
const docCleanupMaxChars = 40000

// TautDocCleanup: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/documents/cleanup
func (d *Deps) TautDocCleanup(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.queryRow(r.Context(),
		"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}

	body := decodeJSON(r)
	text, _ := strField(body, "text")
	title, _ := strField(body, "title")
	if strings.TrimSpace(text) == "" {
		httpx.Error(w, http.StatusBadRequest, "Teks dokumen kosong.")
		return
	}
	if len([]rune(text)) > docCleanupMaxChars*2 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Teks dokumen terlalu panjang untuk dirapikan.")
		return
	}

	runtime, err := ai.LoadAIRuntime(r.Context(), d.Pool)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !ai.IsAIConfigured(runtime) {
		httpx.Error(w, http.StatusConflict, "AI perapian dokumen belum dikonfigurasi. Hubungi admin.")
		return
	}

	brandID := brandRow.ID
	result, genErr := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt:   buildDocCleanupPrompt(title, truncateString(text, docCleanupMaxChars)),
		Tier:     "default",
		TaskType: "document_cleanup",
		BrandID:  &brandID,
		UserID:   &user.ID,
		Runtime:  runtime,
	})
	if genErr != nil {
		httpx.Error(w, http.StatusBadGateway, "AI gagal merapikan dokumen: "+genErr.Error())
		return
	}

	html, ok := normalizeCleanupHTML(result.Content)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"html": "", "cleaned": false,
			"reason":   "AI mengembalikan hasil yang tidak bisa dipakai.",
			"provider": result.Provider, "model": result.Model,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"html": html, "cleaned": true,
		"provider": result.Provider, "model": result.Model,
	})
}

// buildDocCleanupPrompt: instruksi perapian naskah (Indonesia, hasil HTML).
func buildDocCleanupPrompt(title, text string) string {
	lines := []string{
		"Anda editor naskah profesional berbahasa Indonesia. Tugas: rapikan naskah mentah hasil ekstraksi dokumen berikut agar siap dimasukkan ke editor artikel.",
		"",
		"== ATURAN ==",
		"1. Perbaiki pemenggalan baris yang rusak: gabungkan kalimat yang terpotong dan satukan paragraf yang terbelah.",
		"2. Hapus duplikasi, header/footer yang berulang, dan nomor halaman.",
		"3. Pertahankan makna, fakta, angka, nama, dan kutipan asli. JANGAN menambah fakta, opini, atau informasi baru.",
		"4. Subjudul yang ada di naskah ditulis sebagai <h2> atau <h3>. JANGAN membuat <h1> dan JANGAN mengulang judul dokumen di dalam isi — judul sudah disimpan di field judul terpisah.",
		"5. Pertahankan bahasa asli naskah.",
		"6. Keluarkan HANYA HTML bersih: <p>, <h2>, <h3>, <ul>, <ol>, <li>, <blockquote>. Tanpa <html>, <head>, <body>, <h1>, markdown, atau penjelasan tambahan.",
		"",
	}
	if title != "" {
		lines = append(lines, "Judul dokumen (konteks, jangan diulang sebagai paragraf): "+title)
	}
	lines = append(lines, "", "== NASKAH MENTAH ==", text)
	return strings.Join(lines, "\n")
}

// normalizeCleanupHTML: bersihkan keluaran AI. false = kosong/tak terpakai.
func normalizeCleanupHTML(content string) (string, bool) {
	out := strings.TrimSpace(content)
	// Buang pagar kode markdown bila AI membungkus HTML di dalamnya.
	if strings.HasPrefix(out, "```") {
		if idx := strings.Index(out, "\n"); idx != -1 {
			out = out[idx+1:]
		}
		out = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(out), "```"))
	}
	if out == "" || strings.Contains(out, "[mock]") {
		return "", false
	}
	lower := strings.ToLower(out)
	hasTag := strings.Contains(lower, "<p") || strings.Contains(lower, "<h") ||
		strings.Contains(lower, "<ul") || strings.Contains(lower, "<ol") ||
		strings.Contains(lower, "<blockquote")
	if !hasTag {
		// Model mengabaikan instruksi HTML → bungkus teks polos jadi paragraf.
		out = plainTextToHTML(out)
	}
	out = demoteH1ToH2(out)
	if cleanupVisibleLen(out) < 20 {
		return "", false
	}
	return out, true
}

func plainTextToHTML(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var sb strings.Builder
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		sb.WriteString("<p>")
		sb.WriteString(htmlEscape(t))
		sb.WriteString("</p>")
	}
	return sb.String()
}

var cleanupEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func htmlEscape(s string) string { return cleanupEscaper.Replace(s) }

func cleanupVisibleLen(html string) int {
	inTag := false
	count := 0
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag && !unicode.IsSpace(r):
			count++
		}
	}
	return count
}
