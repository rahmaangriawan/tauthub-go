// Package jobwire: satu tempat pendaftaran seluruh processor job supaya
// cmd/api, cmd/worker, dan cmd/scheduler memakai dispatcher yang sama.
package jobwire

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	brandflowdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brandflow"
	briefdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brief"
	briefsummary "github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	clienttautdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/clienttaut"
	partnerdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/partner"
	platformdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/platform"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// Register mendaftarkan semua handler job yang tersedia. Aman dipanggil lebih
// dari sekali (pendaftaran menimpa kind yang sama).
func Register(pool *pgxpool.Pool, cfg *config.Config) {
	// Egress Cloudflare Worker untuk fallback crawl (opsional; kosong = langsung).
	platformdomain.ConfigureCrawlEgress(cfg.CrawlEgressURL, cfg.CrawlEgressToken)
	BrandFlow(pool, cfg).RegisterJobs()
	(&partnerdomain.Deps{Pool: pool, Cfg: cfg}).RegisterJobs()
	(&clienttautdomain.Deps{Pool: pool, Cfg: cfg}).RegisterJobs()
	Platform(pool, cfg).RegisterMessagingJobs()
	Platform(pool, cfg).RegisterMediaJobs()
	Platform(pool, cfg).RegisterPRJobs()
	briefsummary.RegisterJobs(pool, cfg)
	briefdomain.RegisterMemoryJobs(pool)
	registerContentJobs(pool, cfg)
}

// WirePlatform melengkapi Deps platform dengan runner dan hook scheduler yang
// dipakai endpoint /v1/internal/jobs dan /v1/internal/cron/scheduler.
func WirePlatform(platform *platformdomain.Deps, pool *pgxpool.Pool, cfg *config.Config, runner *jobs.Runner) {
	platform.Runner = runner
	brandFlow := BrandFlow(pool, cfg)
	platform.RunScheduler = brandFlow.RunSchedulerResult
}

// BrandFlow: Deps brandflow untuk pool+cfg yang diberikan.
func BrandFlow(pool *pgxpool.Pool, cfg *config.Config) *brandflowdomain.Deps {
	return &brandflowdomain.Deps{Pool: pool, Cfg: cfg}
}

// Platform: Deps platform untuk pool+cfg yang diberikan.
func Platform(pool *pgxpool.Pool, cfg *config.Config) *platformdomain.Deps {
	return &platformdomain.Deps{Pool: pool, Cfg: cfg}
}
