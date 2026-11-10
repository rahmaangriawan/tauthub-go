// Package jobs: runner antrean `jobs` (port lib/jobs/queue.ts) — claim
// SKIP LOCKED, dispatch per kind, backoff, dan loop worker/scheduler.
//
// Processor per kind didaftarkan lewat Register agar pemanggil (cmd/worker,
// cmd/scheduler, /v1/internal/jobs) berbagi satu dispatcher yang sama.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobRow: baris `jobs` yang di-claim.
type JobRow struct {
	ID          string
	Kind        string
	Payload     map[string]any
	Attempts    int
	MaxAttempts int
}

// Handler memproses satu job. attempt sudah dinaikkan saat claim.
type Handler func(ctx context.Context, job JobRow) error

var (
	mu       sync.RWMutex
	handlers = map[string]Handler{}
)

// Register mendaftarkan processor untuk satu kind. Kind yang sama didaftarkan
// ulang akan menimpa (berguna saat test).
func Register(kind string, h Handler) {
	mu.Lock()
	defer mu.Unlock()
	handlers[kind] = h
}

// Registered mengembalikan daftar kind yang sudah punya processor (terurut
// tidak dijamin) — dipakai endpoint kesehatan ops.
func Registered() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(handlers))
	for k := range handlers {
		out = append(out, k)
	}
	return out
}

// UnknownKindError: kind tanpa processor. Dicatat sebagai error supaya job
// tidak "sukses palsu" lalu hilang.
type UnknownKindError struct{ Kind string }

func (e *UnknownKindError) Error() string { return "job kind tanpa processor: " + e.Kind }

// Runner: klaim + eksekusi job dari tabel `jobs`.
type Runner struct {
	Pool *pgxpool.Pool

	mu     sync.Mutex
	closed bool
}

// NewRunner membuat runner dengan pool yang diberikan.
func NewRunner(pool *pgxpool.Pool) *Runner { return &Runner{Pool: pool} }

// backoff: min(30s, 1s * 2^attempt) — parity backoffMs.
func backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	shift := attempt
	if shift > 5 {
		shift = 5
	}
	d := time.Second * time.Duration(1<<uint(shift))
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// ClaimNextJob: ambil satu job pending ATAU failed yang waktunya tiba, kunci
// barisnya (SKIP LOCKED), tandai running, attempts+1. Parity claimNextJob —
// termasuk ikut memungut baris `failed` yang run_at-nya sudah lewat.
//
// `attempts < max_attempts` ditambahkan supaya percobaan benar-benar berhenti:
// tanpa syarat itu job yang sudah kehabisan percobaan diklaim ulang tanpa
// batas (attempts pernah tercatat 156 dari batas 2) dan menghajar penyedia
// eksternal terus-menerus.
func (r *Runner) ClaimNextJob(ctx context.Context, workerID string) (*JobRow, error) {
	return r.claimNext(ctx, workerID, "", false)
}

// ClaimNextJobOfKind / ClaimNextJobExceptKind: varian claim untuk worker yang
// mengkhususkan diri pada satu jenis job (mis. ringkasan brief) tanpa berebut
// dengan antrean umum.
func (r *Runner) ClaimNextJobOfKind(ctx context.Context, workerID, kind string) (*JobRow, error) {
	return r.claimNext(ctx, workerID, kind, false)
}

func (r *Runner) ClaimNextJobExceptKind(ctx context.Context, workerID, kind string) (*JobRow, error) {
	return r.claimNext(ctx, workerID, kind, true)
}

func (r *Runner) claimNext(ctx context.Context, workerID, kind string, exclude bool) (*JobRow, error) {
	filter := ""
	args := []any{workerID}
	if kind != "" {
		if exclude {
			filter = " AND kind <> $2"
		} else {
			filter = " AND kind = $2"
		}
		args = append(args, kind)
	}
	row := r.Pool.QueryRow(ctx, `
		WITH picked AS (
		  SELECT id FROM jobs
		  WHERE status IN ('pending','failed') AND run_at <= NOW()
		    AND attempts < max_attempts`+filter+`
		  ORDER BY run_at
		  LIMIT 1
		  FOR UPDATE SKIP LOCKED
		)
		UPDATE jobs SET status = 'running', locked_at = NOW(), locked_by = $1,
		                attempts = attempts + 1, updated_at = NOW()
		WHERE id IN (SELECT id FROM picked)
		RETURNING id, kind, payload, attempts, max_attempts`, args...)
	var id, jobKind string
	var raw []byte
	var attempts, maxAttempts int
	if err := row.Scan(&id, &jobKind, &raw, &attempts, &maxAttempts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			slog.Warn("[jobs] payload bukan objek JSON", "id", id, "kind", jobKind, "err", err)
		}
	}
	return &JobRow{ID: id, Kind: jobKind, Payload: payload, Attempts: attempts, MaxAttempts: maxAttempts}, nil
}

// MarkCompleted: tandai job selesai (locked_* dilepas). Parity markCompleted —
// tabel `jobs` tidak punya kolom completed_at, jadi hanya status + updated_at.
func (r *Runner) MarkCompleted(ctx context.Context, jobID, workerID string) error {
	_, err := r.Pool.Exec(ctx,
		`UPDATE jobs SET status='completed', locked_at=NULL, locked_by=NULL, updated_at=NOW()
		 WHERE id=$1 AND ($2 = '' OR locked_by=$2)`, jobID, workerID)
	return err
}

// MarkFailed: terminal jika attempts >= max_attempts, selain itu kembali
// pending dengan backoff. Parity markFailed (pesan dipotong 1000).
func (r *Runner) MarkFailed(ctx context.Context, job JobRow, message, workerID string) error {
	status := "pending"
	if job.Attempts >= job.MaxAttempts {
		status = "failed"
	}
	msg := message
	if len(msg) > 1000 {
		msg = msg[:1000]
	}
	runAt := time.Now().Add(backoff(job.Attempts))
	_, err := r.Pool.Exec(ctx,
		`UPDATE jobs SET status=$1, locked_at=NULL, locked_by=NULL, last_error=$2, run_at=$3, updated_at=NOW()
		 WHERE id=$4 AND ($5 = '' OR locked_by=$5)`, status, msg, runAt, job.ID, workerID)
	return err
}

// ReapStuck: selamatkan job yang tertinggal berstatus `running` karena proses
// pemiliknya berhenti (restart/deploy) atau menggantung. Tanpa ini barisnya
// menempel selamanya — tidak pernah diklaim lagi karena claim hanya memungut
// pending/failed — dan indikator antrean di UI tidak pernah bersih (di produksi
// ada dua baris tersangkut sejak September dengan attempts 156 dari batas 2).
// Job yang sudah kehabisan percobaan ditutup sebagai `failed`, sisanya
// dikembalikan ke antrean.
func (r *Runner) ReapStuck(ctx context.Context, staleAfter time.Duration) (int, error) {
	if staleAfter <= 0 {
		staleAfter = 10 * time.Minute
	}
	tag, err := r.Pool.Exec(ctx, `
		UPDATE jobs
		SET status = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'pending' END,
		    last_error = CASE WHEN attempts >= max_attempts
		                      THEN COALESCE(last_error, 'Pemroses berhenti saat job berjalan (dibersihkan otomatis).')
		                      ELSE last_error END,
		    locked_at = NULL, locked_by = NULL, run_at = NOW(), updated_at = NOW()
		WHERE status = 'running'
		  AND (locked_at IS NULL OR locked_at < NOW() - $1::interval)`,
		fmt.Sprintf("%d seconds", int(staleAfter.Seconds())))
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ProcessOnce: claim satu job lalu proses. Mengembalikan (nil, nil) bila
// antrean kosong. Error yang dikembalikan adalah kegagalan infra (DB), bukan
// kegagalan handler — kegagalan handler sudah tercatat ke baris job.
func (r *Runner) ProcessOnce(ctx context.Context, workerID string) (*JobRow, error) {
	return r.processOnce(ctx, workerID, "", false)
}

// ProcessOnceOfKind: klaim & proses satu job dengan kind tertentu.
func (r *Runner) ProcessOnceOfKind(ctx context.Context, workerID, kind string) (*JobRow, error) {
	return r.processOnce(ctx, workerID, kind, false)
}

// ProcessOnceExceptKind: klaim & proses satu job selain kind yang dikecualikan.
func (r *Runner) ProcessOnceExceptKind(ctx context.Context, workerID, kind string) (*JobRow, error) {
	return r.processOnce(ctx, workerID, kind, true)
}

func (r *Runner) processOnce(ctx context.Context, workerID, kind string, exclude bool) (*JobRow, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, nil
	}
	r.mu.Unlock()

	var (
		job *JobRow
		err error
	)
	if kind == "" {
		job, err = r.ClaimNextJob(ctx, workerID)
	} else if exclude {
		job, err = r.ClaimNextJobExceptKind(ctx, workerID, kind)
	} else {
		job, err = r.ClaimNextJobOfKind(ctx, workerID, kind)
	}
	if err != nil || job == nil {
		return nil, err
	}

	mu.RLock()
	h := handlers[job.Kind]
	mu.RUnlock()

	if h == nil {
		if err := r.MarkFailed(ctx, *job, (&UnknownKindError{Kind: job.Kind}).Error(), workerID); err != nil {
			return job, err
		}
		return job, nil
	}

	runErr := runHandler(ctx, h, *job)
	// Pembukuan status memakai konteks baru: konteks job bisa sudah lewat
	// deadline-nya, dan UPDATE dengan konteks kedaluwarsa akan gagal sehingga
	// baris tertahan di 'running'.
	bctx, bcancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer bcancel()
	if runErr != nil {
		if err := r.MarkFailed(bctx, *job, runErr.Error(), workerID); err != nil {
			return job, err
		}
		return job, nil
	}
	if err := r.MarkCompleted(bctx, job.ID, workerID); err != nil {
		return job, err
	}
	return job, nil
}

// Batas waktu per kind: konsumer antrean umum berurutan dan dipakai bersama
// semua tenant, jadi satu job tidak boleh menahannya tanpa batas. Default
// sedikit di bawah ambang ReapStuck (10 menit) agar job yang kena deadline
// tidak diklaim ganda oleh reaper.
const defaultJobTimeout = 9 * time.Minute

var kindTimeouts = map[string]time.Duration{
	"sitemap_crawl": 5 * time.Minute,
}

// JobTimeout: batas waktu eksekusi handler untuk kind tertentu.
func JobTimeout(kind string) time.Duration {
	if d, ok := kindTimeouts[kind]; ok {
		return d
	}
	return defaultJobTimeout
}

// runHandler: jalankan handler dengan deadline per kind dan pemulihan panic
// supaya satu job rusak/lambat tidak mematikan atau menahan proses worker.
func runHandler(ctx context.Context, h Handler, job JobRow) (err error) {
	ctx, cancel := context.WithTimeout(ctx, JobTimeout(job.Kind))
	defer cancel()
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic saat memproses job %s: %v", job.Kind, rec)
		}
	}()
	return h(ctx, job)
}

// Drain: proses sampai `limit` job atau antrean kosong; mengembalikan jumlah
// job yang diproses (parity drainJobs — termasuk yang gagal).
func (r *Runner) Drain(ctx context.Context, limit int, workerID string) (int, error) {
	if limit <= 0 {
		limit = 10
	}
	count := 0
	for i := 0; i < limit; i++ {
		job, err := r.ProcessOnce(ctx, workerID)
		if err != nil {
			return count, err
		}
		if job == nil {
			break
		}
		count++
	}
	return count, nil
}

// Loop: polling periodik sampai ctx dibatalkan (dipakai cmd/worker).
func (r *Runner) Loop(ctx context.Context, interval time.Duration, workerID string) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.ProcessOnce(ctx, workerID); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("[jobs] worker error", "err", err)
			}
		}
	}
}

// Dispatch menjalankan handler terdaftar untuk satu job tanpa claim (dipakai
// pemanggil yang sudah memegang barisnya sendiri).
func Dispatch(ctx context.Context, job JobRow) error {
	mu.RLock()
	h := handlers[job.Kind]
	mu.RUnlock()
	if h == nil {
		return &UnknownKindError{Kind: job.Kind}
	}
	return runHandler(ctx, h, job)
}

// WorkerID: id worker unik per proses.
func WorkerID(prefix string) string {
	if prefix == "" {
		prefix = "worker"
	}
	return fmt.Sprintf("%s-%s", prefix, uuid.NewString()[:8])
}
