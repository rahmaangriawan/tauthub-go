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
	pushdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/push"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
	"github.com/rahmaangriawan/tauthub-go/internal/jobwire"
)

// Worker: drainer antrean `jobs` — claim (FOR UPDATE SKIP LOCKED) lalu dispatch
// per kind lewat registry internal/jobs, dengan retry + backoff standar.
// Peta job kind: REWRITE/01_BACKEND_GO.md §6.
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("worker: konfigurasi gagal", "err", err)
		os.Exit(1)
	}
	database, err := db.Open(cfg.DBDSN(), cfg.PoolSize)
	if err != nil {
		slog.Error("worker: koneksi db gagal", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	jobwire.Register(database.Pool, cfg)

	interval := envDuration("WORKER_POLL_SECONDS", 5*time.Second)
	runner := jobs.NewRunner(database.Pool)
	slog.Info("worker aktif", "poll", interval.String(), "kinds", len(jobs.Registered()))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Pengiriman Web Push berjalan di loop terpisah: antreannya bisa berisi
	// ribuan baris saat pengumuman dikirim, sedangkan antrean `jobs` umum
	// memproses satu job per interval. Konkurensi dibatasi PUSH_WORKERS.
	pushDeps := pushdomain.New(database.Pool, cfg)
	pushInterval := envDuration("PUSH_POLL_SECONDS", 2*time.Second)
	slog.Info("[push] loop aktif", "enabled", pushDeps.Enabled(), "interval", pushInterval.String(), "workers", cfg.PushWorkers)
	go func() {
		workerID := jobs.WorkerID("push")
		ticker := time.NewTicker(pushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for {
					n, err := pushDeps.ProcessBatch(ctx, workerID, 100)
					if err != nil {
						slog.Error("[push] worker error", "err", err)
						break
					}
					if n == 0 {
						break
					}
				}
			}
		}
	}()

	runner.Loop(ctx, interval, jobs.WorkerID("worker"))
	slog.Info("worker berhenti")
}

func envDuration(key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	return def
}
