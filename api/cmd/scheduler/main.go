package main

import (
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Scheduler loop — skeleton fase 0 (pengganti scripts/scheduler-worker.mjs + cron Vercel).
// Fase 6: runScheduler() — materialisasi publication-plan (SKIP LOCKED), safety sweep,
// dedupe via payload->>'publicationId'. Peta: REWRITE/01_BACKEND_GO.md §7.
func main() {
	secs := envInt("SCHEDULER_INTERVAL_SECONDS", 60)
	interval := time.Duration(secs) * time.Second
	_ = envInt("SCHEDULER_MAX_JOBS", 20) // dipakai saat drain diimplementasi

	slog.Info("scheduler aktif", "interval", interval.String())
	ticker := time.NewTicker(interval)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			// TODO(fase 6): runScheduler(ctx) + drainJobs.
			slog.Debug("tick: runScheduler belum diport")
		case <-stop:
			slog.Info("scheduler berhenti")
			return
		}
	}
}

func envInt(key string, def int) int {
	if raw := os.Getenv(key); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			return v
		}
	}
	return def
}
