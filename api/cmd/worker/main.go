package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Worker drainer: claim job dari tabel `jobs` (FOR UPDATE SKIP LOCKED) lalu
// dispatch per kind. Fase 4: content_generation + sitemap_crawl.
// Peta 13 job kind: REWRITE/01_BACKEND_GO.md §6.
func main() {
	interval := envDuration("WORKER_POLL_SECONDS", 5)

	slog.Info("worker aktif", "poll", interval.String())
	ticker := time.NewTicker(interval)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			// TODO(fase 4): drainJobs(1) — claim + processJob dispatcher.
			slog.Debug("tick: belum ada processor job")
		case <-stop:
			slog.Info("worker berhenti")
			return
		}
	}
}

func envDuration(key string, def time.Duration) time.Duration {
	if raw := os.Getenv(key); raw != "" {
		if secs, err := time.ParseDuration(raw + "s"); err == nil && secs > 0 {
			return secs
		}
	}
	return def
}
