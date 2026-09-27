package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	brandflowdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brandflow"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
	"github.com/rahmaangriawan/tauthub-go/internal/jobwire"
)

// Scheduler loop — pengganti scripts/scheduler-worker.mjs + cron Vercel:
// materialisasi publication-plan (SKIP LOCKED), sweep pengingat mitra, drain
// antrean job, dan sweep berkala lainnya.
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("scheduler: konfigurasi gagal", "err", err)
		os.Exit(1)
	}
	database, err := db.Open(cfg.DBDSN(), cfg.PoolSize)
	if err != nil {
		slog.Error("scheduler: koneksi db gagal", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	jobwire.Register(database.Pool, cfg)

	interval := time.Duration(envInt("SCHEDULER_INTERVAL_SECONDS", 60)) * time.Second
	maxJobs := envInt("SCHEDULER_MAX_JOBS", 20)
	brandFlow := &brandflowdomain.Deps{Pool: database.Pool, Cfg: cfg}
	runner := jobs.NewRunner(database.Pool)

	slog.Info("scheduler aktif", "interval", interval.String(), "max_jobs", maxJobs)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	brandFlow.RunSchedulerLoop(ctx, interval, maxJobs, runner)
	slog.Info("scheduler berhenti")
}

func envInt(key string, def int) int {
	if raw := os.Getenv(key); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			return v
		}
	}
	return def
}
