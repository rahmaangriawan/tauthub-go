package platform

// Terjemahan entri changelog ke bahasa Inggris ("auto"): dipakai saat pengguna
// memilih antarmuka Inggris. Entri yang belum punya versi EN diterjemahkan lewat
// model AI platform lalu disimpan di kolom *_en (migrasi 191) supaya permintaan
// berikutnya tidak memanggil model lagi. Kegagalan model TIDAK memblokir daftar:
// entri jatuh kembali ke teks aslinya.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

const (
	chgTranslateWorkers = 4
	// Satu entri bisa panjang (HTML >1.000 karakter) dan provider bisa lambat,
	// jadi batasnya lega. Bila belum selesai saat permintaan HTTP tutup, sisanya
	// dilanjutkan di latar dan dipakai pada permintaan berikutnya.
	chgTranslateTimeout = 90 * time.Second
	chgLocalizeBudget   = 45 * time.Second
)

type chgTranslation struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Body    string `json:"body"`
}

// chgLocalizeItems: isi ulang item["title"|"summary"|"body"] dengan versi EN.
// rows dan items sejajar per indeks (keduanya dari query yang sama).
func (d *Deps) chgLocalizeItems(ctx context.Context, userID string, rows []rowMap, items []map[string]any) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, chgTranslateWorkers)
	// Tanpa cancel milik permintaan: terjemahan yang masih jalan tetap selesai
	// (dan tersimpan) walau klien sudah pergi.
	base := context.WithoutCancel(ctx)

	for i := range rows {
		if i >= len(items) {
			break
		}
		row := rows[i]
		item := items[i]
		title := row.strOr("title", "")
		summary := row.strOr("summary", "")
		body := row.strOr("body", "")
		titleEn := row.strOr("title_en", "")
		summaryEn := row.strOr("summary_en", "")
		bodyEn := row.strOr("body_en", "")

		// Sudah ada di cache DB → pakai tanpa memanggil model.
		if !chgNeedsTranslation(title, body, titleEn, bodyEn) {
			chgApplyTranslation(item, titleEn, summaryEn, bodyEn)
			continue
		}

		entryID := row.strOr("id", "")
		if entryID == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			tctx, cancel := context.WithTimeout(base, chgTranslateTimeout)
			defer cancel()
			tr, err := d.chgTranslateEntry(tctx, userID, title, summary, body)
			if err != nil {
				slog.Warn("changelog: terjemahan gagal", "entry", entryID, "err", err)
				return
			}
			if err := d.exec(base, `UPDATE changelog_entries
				SET title_en = $2, summary_en = $3, body_en = $4, updated_at = NOW()
				WHERE id = $1`, entryID, tr.Title, tr.Summary, tr.Body); err != nil {
				slog.Warn("changelog: simpan terjemahan gagal", "entry", entryID, "err", err)
			}
			chgApplyTranslation(item, tr.Title, tr.Summary, tr.Body)
		}()
	}

	// Tunggu maksimal chgLocalizeBudget; sisanya lanjut di latar (goroutine sudah
	// memakai context tanpa cancel, jadi tetap tersimpan ke kolom *_en).
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(chgLocalizeBudget):
		slog.Info("changelog: terjemahan dilanjutkan di latar")
	}
}

// chgNeedsTranslation: entri berisi dan versi EN-nya belum ada.
// summary tidak diwajibkan — ringkasan boleh kosong pada entri aslinya.
func chgNeedsTranslation(title, body, titleEn, bodyEn string) bool {
	if strings.TrimSpace(title) == "" && strings.TrimSpace(body) == "" {
		return false
	}
	return strings.TrimSpace(titleEn) == "" || strings.TrimSpace(bodyEn) == ""
}

func chgApplyTranslation(item map[string]any, titleEn, summaryEn, bodyEn string) {
	if strings.TrimSpace(titleEn) != "" {
		item["title"] = titleEn
	}
	if strings.TrimSpace(summaryEn) != "" {
		item["summary"] = summaryEn
	}
	if strings.TrimSpace(bodyEn) != "" {
		item["body"] = bodyEn
	}
}

// chgTranslateEntry: satu panggilan model dengan keluaran JSON ketat.
func (d *Deps) chgTranslateEntry(ctx context.Context, userID, title, summary, body string) (*chgTranslation, error) {
	payload, err := json.Marshal(map[string]string{"title": title, "summary": summary, "body": body})
	if err != nil {
		return nil, err
	}
	// Tier "default" = model flash pada konfigurasi produksi; tier "economy"
	// dipetakan ke model pro yang jauh lebih lambat (terjemahan panjang pernah
	// melewati batas waktu). Terjemahan tidak butuh model terkuat.
	res, err := ai.GenerateChat(ctx, d.Pool, ai.GenerateChatOptions{
		Prompt:   chgTranslatePrompt(string(payload)),
		Tier:     "default",
		TaskType: "changelog_translate",
		UserID:   &userID,
	})
	if err != nil {
		return nil, err
	}
	// Mode mock (AI platform belum dikonfigurasi) tidak boleh disimpan sebagai
	// terjemahan — lebih baik tampil apa adanya daripada teks palsu.
	if res == nil || res.Mocked || strings.TrimSpace(res.Content) == "" {
		return nil, errors.New("terjemahan tidak tersedia")
	}
	out, err := chgParseTranslation(res.Content)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Title) == "" {
		out.Title = title
	}
	if strings.TrimSpace(out.Summary) == "" {
		out.Summary = summary
	}
	if strings.TrimSpace(out.Body) == "" {
		out.Body = body
	}
	return out, nil
}

func chgTranslatePrompt(inputJSON string) string {
	return "Terjemahkan catatan rilis produk berikut dari bahasa Indonesia ke bahasa Inggris.\n" +
		"Aturan: pertahankan SEMUA tag/atribut HTML dan tautan apa adanya (jangan diubah, ditambah, atau dihapus); " +
		"jangan terjemahkan nama produk (Tauthub), nama orang, atau istilah teknis (API, JSON, SEO, LLM); " +
		"gaya ringkas dan profesional seperti release notes.\n" +
		"Balas HANYA JSON valid berbentuk {\"title\": \"...\", \"summary\": \"...\", \"body\": \"...\"} tanpa penjelasan.\n" +
		"Masukan (JSON):\n" + inputJSON
}

// chgParseTranslation: keluaran model bisa dibungkus pagar kode — ambil objek JSON-nya.
func chgParseTranslation(content string) (*chgTranslation, error) {
	text := strings.TrimSpace(content)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return nil, errors.New("keluaran model bukan JSON")
	}
	var out chgTranslation
	if err := json.Unmarshal([]byte(text[start:end+1]), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
