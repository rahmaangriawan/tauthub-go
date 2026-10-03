// image_events.go: catatan log generate gambar AI untuk monitoring admin
// (halaman Taut Pintar > AI Gambar > Log & Monitoring).
//
// GenerateImage sengaja tidak hard-fail: provider yang belum lengkap atau gagal
// menghasilkan placeholder lokal, dan alasannya hanya masuk slog. Tanpa catatan
// ini admin tidak bisa tahu KENAPA sebuah gambar jadi placeholder. Penulisan
// best-effort: kegagalan mencatat tidak pernah menggagalkan alur generate.
package ai

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ImageEvent: satu percobaan generate/uji koneksi gambar.
type ImageEvent struct {
	Kind           string // "generation" | "connection_test"
	Status         string // "succeeded" | "fallback" | "failed"
	Provider       string
	Model          string
	BaseURL        string
	ErrorDetail    string // apa yang salah dan kenapa (pesan provider/alasan fallback)
	Prompt         string
	Role           string
	BrandID        string
	CampaignID     string
	MediaID        string
	ArticleID      string
	RequestedBy    string
	Width          int
	Height         int
	CreditsCharged int
	ElapsedMS      int
	ImageID        string
	ImageURL       string
}

// RecordImageEvent: simpan satu event gambar. Best-effort (gagal = log warning).
func RecordImageEvent(ctx context.Context, pool *pgxpool.Pool, ev ImageEvent) {
	if pool == nil {
		return
	}
	kind := ev.Kind
	if kind != "connection_test" {
		kind = "generation"
	}
	status := ev.Status
	switch status {
	case "succeeded", "fallback", "failed":
	default:
		status = "failed"
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO ai_image_events (id, event_kind, status, provider, model, base_url, error_detail,
		  prompt, role, brand_id, campaign_id, media_id, article_id, requested_by,
		  width, height, credits_charged, elapsed_ms, image_id, image_url)
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
		kind, status, imageNullText(ev.Provider), imageNullText(ev.Model), imageNullText(ev.BaseURL),
		imageNullText(truncateRunes(ev.ErrorDetail, 1000)), imageNullText(truncateRunes(ev.Prompt, 500)),
		imageNullText(ev.Role), imageNullText(ev.BrandID), imageNullText(ev.CampaignID),
		imageNullText(ev.MediaID), imageNullText(ev.ArticleID), imageNullText(ev.RequestedBy),
		ev.Width, ev.Height, ev.CreditsCharged, ev.ElapsedMS,
		imageNullText(ev.ImageID), imageNullText(ev.ImageURL)); err != nil {
		slog.Warn("[ai] catat event gambar gagal", "kind", kind, "status", status, "err", err)
	}
}

// imageNullText: string kosong → NULL (kolom id/teks opsional).
func imageNullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}
