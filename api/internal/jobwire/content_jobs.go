package jobwire

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
	knowledgedomain "github.com/rahmaangriawan/tauthub-go/internal/domains/knowledge"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// registerContentJobs: kind yang processornya sudah ada di paket lain.
// Wrapper di sini hanya memetakan payload job → pemanggilan fungsi; claim baris
// job sudah dilakukan Runner, jadi jangan memakai fungsi yang meng-claim lagi.
func registerContentJobs(pool *pgxpool.Pool, cfg *config.Config) {
	jobs.Register("sitemap_crawl", func(ctx context.Context, job jobs.JobRow) error {
		var p struct {
			RunID        string `json:"runId"`
			BrandID      string `json:"brandId"`
			CollectionID string `json:"collectionId"`
			SitemapURL   string `json:"sitemapUrl"`
			MaxPages     int    `json:"maxPages"`
			UserID       string `json:"userId"`
		}
		if err := remarshal(job.Payload, &p); err != nil {
			return err
		}
		_, _, _, _, crawlErr := knowledgedomain.CrawlSitemap(
			ctx, pool, p.RunID, p.BrandID, p.CollectionID, p.SitemapURL, p.MaxPages, p.UserID)
		if crawlErr != nil {
			failedStatus := "failed"
			message := crawlErr.Error()
			knowledgedomain.UpdateCrawlRun(ctx, pool, p.RunID, knowledgedomain.CrawlRunUpdate{
				Status: &failedStatus, ErrorMessage: &message,
			})
			return crawlErr // biarkan Runner mengurus retry/backoff
		}
		done := "completed"
		knowledgedomain.UpdateCrawlRun(ctx, pool, p.RunID, knowledgedomain.CrawlRunUpdate{Status: &done})
		return nil
	})

	jobs.Register("content_generation", func(ctx context.Context, job jobs.JobRow) error {
		var payload contentengine.JobPayload
		if err := remarshal(job.Payload, &payload); err != nil {
			return err
		}
		return contentengine.ProcessContentGeneration(ctx, pool, payload, job.Attempts, job.MaxAttempts)
	})
}

// remarshal: map payload job → struct tujuan (lewat JSON agar tag json hormati).
func remarshal(src map[string]any, dst any) error {
	raw, err := json.Marshal(src)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		slog.Warn("[jobs] payload tidak sesuai bentuk", "err", err)
		return err
	}
	return nil
}
