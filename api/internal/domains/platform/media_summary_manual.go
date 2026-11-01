package platform

// media_summary_manual.go: kesimpulan media yang diisi admin langsung dari
// panel Monitoring Kesimpulan Website.
//
//   - POST /{mediaId}/draft  — AI merangkum teks yang ditempel admin (atau
//     crawl fallback bila kolom teks kosong). Hasilnya DRAFT saja: tidak ada
//     yang ditulis ke database, admin bisa edit dulu di modal.
//   - POST /{mediaId}/manual — simpan kesimpulan akhir ke media.ai_summary,
//     sebar ke media lain yang menunjuk domain sama (aturan satu profil per
//     domain, sama dengan job), dan catat event berstatus 'manual'.
//
// Kenapa tidak lewat antrean: crawler sering ditolak situs (HTTP 403/429,
// DNS gagal) sehingga job gagal berulang. Jalur manual memakai teks yang
// admin punya, jadi tidak bergantung crawler sama sekali.

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

const (
	mpManualSummaryMaxChars = 2000
	// 60k runes: satu halaman "About" yang dipotong penuh biasanya 5-30k
	// karakter; 20k terbukti terlalu kecil dan paste panjang selalu ditolak.
	mpManualSourceMaxChars = 60000
)

// mpManualMediaRow: muat media untuk kedua endpoint (draft maupun simpan).
func (d *Deps) mpManualMediaRow(ctx context.Context, mediaID string) (ptRow, error) {
	row, err := d.queryRow(ctx, `
		SELECT id, name, category, description, notes, website_url, ai_summary, ai_summary_generated_at
		FROM media WHERE id = $1::text LIMIT 1`, mediaID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Media tidak ditemukan")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// ---------- POST /v1/platform/media-summaries/{mediaId}/draft ----------

func (d *Deps) MediaSummaryAIDraft(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if err := d.mpWriteRate(r.Context(), "media-summary-draft:"+admin.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	mediaID := chiParam(r, "mediaId")
	body := decodeJSON(r)
	text, _ := strField(body, "text")
	text = strings.TrimSpace(text)
	if runes := []rune(text); len(runes) > mpManualSourceMaxChars {
		// Log setiap penolakan supaya kegagalan "Ringkas dengan AI" bisa
		// didiagnosis dari journalctl (dulu diam-diam 422 tanpa jejak).
		slog.Info("[media-summary] draft ditolak: teks sumber terlalu panjang",
			"admin", admin.ID, "media", mediaID, "len", len(runes))
		writeErr(w, d, errUnprocessable(fmt.Sprintf("Teks sumber maksimal %s karakter. Potong dulu teksnya, atau ringkas manual di kolom Kesimpulan akhir.", mpThousandDot(mpManualSourceMaxChars))))
		return
	}

	media, err := d.mpManualMediaRow(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	websiteURL := strings.TrimSpace(media.strOr("website_url", ""))

	// Sumber teks: tempelan admin bila ada, kalau kosong coba crawl seperti
	// job biasa (fallback — berguna saat situs sedang bisa diakses).
	sourceText := text
	pages := []any{}
	if sourceText == "" {
		if websiteURL == "" {
			slog.Info("[media-summary] draft ditolak: tanpa teks sumber dan tanpa website_url",
				"admin", admin.ID, "media", mediaID)
			writeErr(w, d, errUnprocessable("Media tidak punya website_url. Tempel teks situs di kolom sumber dulu."))
			return
		}
		crawled := mjCrawlWebsiteText(r.Context(), websiteURL)
		sourceText = crawled.Text
		for _, p := range crawled.Pages {
			pages = append(pages, p)
		}
	}
	if utf8.RuneCountInString(sourceText) < 80 {
		slog.Info("[media-summary] draft ditolak: teks sumber terlalu pendek",
			"admin", admin.ID, "media", mediaID, "source_len", utf8.RuneCountInString(sourceText), "pasted", text != "")
		writeErr(w, d, errUnprocessable("Teks sumber terlalu pendek (minimal 80 karakter). Tempel lebih banyak isi halaman di kolom Teks sumber DI ATAS tombol ini."))
		return
	}

	runtime, rErr := ai.LoadAIRuntime(r.Context(), d.Pool)
	if rErr != nil {
		writeErr(w, d, rErr)
		return
	}
	if !ai.IsAIConfigured(runtime) {
		writeErr(w, d, errConflict("AI belum dikonfigurasi. Atur provider AI di Konfigurasi AI terlebih dahulu."))
		return
	}
	result, gErr := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		// Prompt sama dengan job media_ai_summary supaya gaya kesimpulan konsisten.
		Prompt:   mjSummaryPrompt(mjMitraContext(media), sourceText),
		Tier:     "economy",
		TaskType: "media_summary_manual",
		UserID:   &admin.ID,
		Runtime:  runtime,
	})
	if gErr != nil {
		slog.Warn("[media-summary] draft AI gagal",
			"admin", admin.ID, "media", mediaID, "source_len", utf8.RuneCountInString(sourceText), "err", gErr)
		writeErr(w, d, gErr)
		return
	}
	if result.Mocked {
		slog.Warn("[media-summary] draft AI mengembalikan mock (provider tidak lengkap)", "admin", admin.ID, "media", mediaID)
		writeErr(w, d, errConflict("AI belum dikonfigurasi. Atur provider AI terlebih dahulu."))
		return
	}
	summary := mjTrimQuotes(result.Content)
	if utf8.RuneCountInString(summary) < mjMinSummaryChars {
		slog.Warn("[media-summary] hasil AI terlalu pendek",
			"admin", admin.ID, "media", mediaID, "out_len", utf8.RuneCountInString(summary))
		writeErr(w, d, errUnprocessable("Hasil AI terlalu pendek untuk dipakai. Perbanyak teks sumber lalu coba lagi."))
		return
	}
	slog.Info("[media-summary] draft AI sukses",
		"admin", admin.ID, "media", mediaID, "source_len", utf8.RuneCountInString(sourceText), "out_len", utf8.RuneCountInString(summary))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "summary": summary, "pages_tried": pages,
	})
}

// ---------- POST /v1/platform/media-summaries/{mediaId}/manual ----------

func (d *Deps) MediaSummaryManualSet(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	body := decodeJSON(r)
	summary, _ := strField(body, "summary")
	summary = strings.TrimSpace(summary)
	source, _ := strField(body, "source")
	if source != "ai" && source != "manual" {
		source = "manual"
	}
	if runes := []rune(summary); len(runes) < mjMinSummaryChars {
		writeErr(w, d, errUnprocessable(fmt.Sprintf("Kesimpulan minimal %d karakter.", mjMinSummaryChars)))
		return
	} else if len(runes) > mpManualSummaryMaxChars {
		writeErr(w, d, errUnprocessable(fmt.Sprintf("Kesimpulan maksimal %d karakter.", mpManualSummaryMaxChars)))
		return
	}

	media, err := d.mpManualMediaRow(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	websiteURL := strings.TrimSpace(media.strOr("website_url", ""))

	if err := d.exec(r.Context(),
		"UPDATE media SET ai_summary = $1, ai_summary_generated_at = NOW() WHERE id = $2::text",
		summary, mediaID); err != nil {
		writeErr(w, d, err)
		return
	}
	// Satu domain = satu profil: sebarkan ke media lain yang menunjuk website
	// sama dan belum punya kesimpulan (SQL sama dengan job media_ai_summary).
	if domain := mjNormalizedDomain(websiteURL); domain != "" {
		if err := d.exec(r.Context(), `
			UPDATE media SET ai_summary = $1, ai_summary_generated_at = NOW()
			WHERE id <> $2::text AND ai_summary IS NULL
			  AND RTRIM(REGEXP_REPLACE(LOWER(TRIM(website_url)), '^(https?://)?(www\.)?', ''), '/') = $3`,
			summary, mediaID, domain); err != nil {
			d.logWarn("sebar kesimpulan manual ke domain sama gagal", mediaID, err)
		}
	}
	d.mpRecordSummaryEventWithSource(r.Context(), mediaID, media.strOr("name", ""), websiteURL, "manual", nil, "", source)

	d.audit(r.Context(), admin.ID, "platform.media_summary_manual_set", "media", mediaID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "manual", "source": source})
}

func mpThousandDot(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) > 3 {
		return s[:len(s)-3] + "." + s[len(s)-3:]
	}
	return s
}
