package summary

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Queue: NONAKTIF — ringkasan brief HANYA dibuat lewat tombol manual
// "Buat ringkasan" (QueueForce) di panel Ringkasan. Panggilan otomatis lama
// (setelah chat, upload dokumen/gambar, edit profil, dsb.) tetap memanggil
// fungsi ini sebagai no-op supaya puluhan call site tidak perlu disentuh;
// kembalikan ke `enqueue(ctx, pool, profileID, brandID, kind, false)` bila
// kebijakan otomatis diaktifkan lagi.
func Queue(ctx context.Context, pool *pgxpool.Pool, profileID, brandID string, kind Kind) error {
	_ = ctx
	_ = pool
	_ = profileID
	_ = brandID
	_ = kind
	return nil
}

// QueueForce: dipakai tombol manual. Job yang menunggu ikut ditandai force
// supaya model tetap dipanggil walau bahan brief tidak berubah (mis. setelah
// admin mengganti prompt/model agent).
func QueueForce(ctx context.Context, pool *pgxpool.Pool, profileID, brandID string, kind Kind) error {
	return enqueue(ctx, pool, profileID, brandID, kind, true)
}

// UnstickStale: baris ringkasan tertinggal 'queued'/'running' padahal tidak
// ada job yang hidup lagi — terjadi bila worker/proses mati di tengah
// pemrosesan (mis. restart saat deploy) sehingga status tak pernah
// tertutup dan UI klien berputar selamanya. Kembalikan ke status diam
// (ready bila isinya sudah ada, idle bila belum); tombol manual selalu bisa
// memicu ulang. Guard 5 menit mencegah balapan dengan enqueue yang baru.
func UnstickStale(ctx context.Context, pool *pgxpool.Pool) int64 {
	tag, err := pool.Exec(ctx, `
		UPDATE profiler_brief_summaries s
		SET status = CASE WHEN COALESCE(s.content, '') <> '' THEN 'ready' ELSE 'idle' END
		WHERE s.status IN ('queued', 'running')
		  AND s.updated_at < now() - interval '5 minutes'
		  AND NOT EXISTS (
			SELECT 1 FROM jobs j
			WHERE j.kind = $1
			  AND j.status IN ('pending', 'running')
			  AND j.payload->>'profileId' = s.profile_id
			  AND j.payload->>'kind' = s.kind
		  )`, JobKind)
	if err != nil {
		slog.Warn("[brief-summary] gagal melepas status macet", "err", err)
		return 0
	}
	return tag.RowsAffected()
}

func enqueue(ctx context.Context, pool *pgxpool.Pool, profileID, brandID string, kind Kind, force bool) error {
	if profileID == "" {
		return nil
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiler_brief_summaries (profile_id, kind, status, updated_at)
		VALUES ($1, $2, 'queued', now())
		ON CONFLICT (profile_id, kind) DO UPDATE
		SET status = 'queued', error = NULL, updated_at = now()`,
		profileID, string(kind)); err != nil {
		slog.Warn("[brief-summary] gagal menandai queued", "profile_id", profileID, "kind", kind, "err", err)
		return err
	}
	if force {
		// Job pending yang sudah ada cukup ditandai force — tidak perlu
		// menambah job kedua untuk brief & kind yang sama.
		if _, err := pool.Exec(ctx, `
			UPDATE jobs SET payload = payload || '{"force":true}'::jsonb, updated_at = now()
			WHERE id = (
				SELECT id FROM jobs
				WHERE kind = $1::text AND status = 'pending'
				  AND payload->>'profileId' = $2::text AND payload->>'kind' = $3::text
				LIMIT 1
			)`, JobKind, profileID, string(kind)); err != nil {
			slog.Warn("[brief-summary] gagal menandai force", "profile_id", profileID, "kind", kind, "err", err)
			return err
		}
	}

	// Delay: tombol manual tidak perlu menunggu debounce. Debounce 20–40 detik
	// hanya berguna untuk enqueue otomatis beruntun (yang kini sudah mati);
	// untuk klik manual, ditunda justru terasa seperti sistem menggantung.
	//
	// Jitter acak juga hanya untuk jalur otomatis. Dulu jitter sampai 20 detik
	// (plus 15 detik khusus kind=file) ikut dipakai saat force, sehingga klik
	// "Buat ringkasan" menunggu rata-rata 10-17 detik SEBELUM job mulai
	// dikerjakan — terasa seperti agent menggantung padahal modelnya cepat.
	delay := "interval '0 seconds'"
	jitter := "interval '0 seconds'"
	if !force {
		delay = "interval '20 seconds'"
		jitter = "interval '20 seconds'"
	}
	fileDelay := "interval '0 seconds'"
	if !force {
		fileDelay = "interval '15 seconds'"
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at)
		SELECT gen_random_uuid()::text, $1::text,
		       jsonb_build_object('profileId', $2::text, 'brandId', btrim($3::text), 'kind', $4::text)
		         || CASE WHEN $5::boolean THEN '{"force":true}'::jsonb ELSE '{}'::jsonb END,
		       'pending', 0, 2,
		       now() + `+delay+`
		         + CASE WHEN $4::text = 'file' THEN `+fileDelay+` ELSE interval '0 seconds' END
		         + (random() * `+jitter+`)
		WHERE NOT EXISTS (
			SELECT 1 FROM jobs
			WHERE kind = $1::text AND status = 'pending'
			  AND payload->>'profileId' = $2::text
			  AND payload->>'kind' = $4::text
		)`, JobKind, profileID, brandID, string(kind), force); err != nil {
		slog.Warn("[brief-summary] gagal menjadwalkan job", "profile_id", profileID, "kind", kind, "err", err)
		// Jangan tinggalkan status 'queued' tanpa job — UI akan berputar
		// selamanya. Tandai gagal dengan alasan yang terbaca admin di panel.
		_, _ = pool.Exec(ctx, `
			UPDATE profiler_brief_summaries
			SET status = 'failed', error = $3, updated_at = now()
			WHERE profile_id = $1 AND kind = $2`,
			profileID, string(kind), "Gagal menjadwalkan ringkasan: "+truncForSummary(err.Error()))
		return err
	}
	return nil
}

// truncForSummary: pesan error untuk kolom UI — cukup bagian pembukanya.
func truncForSummary(s string) string {
	const max = 300
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// QueueAll: kedua ringkasan (dipakai saat brief baru dibuat/diduplikasi).
func QueueAll(ctx context.Context, pool *pgxpool.Pool, profileID, brandID string) {
	_ = Queue(ctx, pool, profileID, brandID, KindEntity)
	_ = Queue(ctx, pool, profileID, brandID, KindFile)
}
