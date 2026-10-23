package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
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
	summaryWorkers := envInt("BRIEF_SUMMARY_CONCURRENCY", 6)
	runner := jobs.NewRunner(database.Pool)
	slog.Info("worker aktif", "poll", interval.String(), "kinds", len(jobs.Registered()),
		"summary_workers", summaryWorkers)

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

	// Antrean umum: semua kind KECUALI ringkasan brief, satu job pada satu waktu.
	// Ringkasan brief dipisah ke worker sendiri karena sifatnya burst: ratusan
	// brief bisa berubah hampir bersamaan, dan satu panggilan model bisa lama.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runWorkerLoop(ctx, runner, interval, jobs.WorkerID("worker"), "", "brief_summary")
	}()
	for i := 0; i < summaryWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runWorkerLoop(ctx, runner, interval, jobs.WorkerID("summary"), "brief_summary", "")
		}()
	}
	wg.Wait()
	slog.Info("worker berhenti")
}

// runWorkerLoop: klaim & proses job terus-menerus selama antrean berisi, lalu
// tidur `idle` saat kosong. onlyKind != "" membatasi ke satu kind; exceptKind
// != "" mengecualikan satu kind.
func runWorkerLoop(ctx context.Context, runner *jobs.Runner, idle time.Duration, workerID, onlyKind, exceptKind string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var (
			job *jobs.JobRow
			err error
		)
		switch {
		case onlyKind != "":
			job, err = runner.ProcessOnceOfKind(ctx, workerID, onlyKind)
		case exceptKind != "":
			job, err = runner.ProcessOnceExceptKind(ctx, workerID, exceptKind)
		default:
			job, err = runner.ProcessOnce(ctx, workerID)
		}
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				slog.Error("[jobs] worker error", "err", err)
			}
			sleepCtx(ctx, idle)
			continue
		}
		if job == nil {
			sleepCtx(ctx, idle)
			continue
		}
		// Jeda kecil supaya DB tidak dihantam saat antrean panjang.
		sleepCtx(ctx, 100*time.Millisecond)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func envInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return n
	}
	return def
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
