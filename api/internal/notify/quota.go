package notify

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Kuota pengiriman berbayar per user per kanal.
//
// Sebelumnya batas email adalah counter in-memory di paket email. Karena api,
// worker, dan scheduler adalah tiga proses terpisah, jatah 20/jam per user
// sebenarnya bisa menjadi 60/jam. Tabel notify_quotas membuat batasnya tunggal
// dan tahan restart.

// TakeQuota menaikkan hitungan bucket jam berjalan dan melaporkan apakah
// pemanggil masih di bawah batas.
//
// limit <= 0 berarti tanpa batas: fungsi langsung mengembalikan true tanpa
// menyentuh database, supaya kanal yang sengaja dibiarkan terbuka tidak
// membayar biaya query.
//
// Satu statement UPSERT dipakai agar dua proses yang mengirim bersamaan tidak
// bisa sama-sama lolos melewati batas.
func TakeQuota(ctx context.Context, pool *pgxpool.Pool, userID, channel string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	if userID == "" {
		// Tanpa user tidak ada yang bisa dikuotakan. Pemanggil yang
		// seharusnya menolak lebih awal.
		return false, nil
	}
	windowStart := time.Now().Truncate(window)
	var count int
	err := pool.QueryRow(ctx, `
		INSERT INTO notify_quotas (id, user_id, channel, window_start, count)
		VALUES (gen_random_uuid(), $1, $2, $3, 1)
		ON CONFLICT (user_id, channel, window_start) DO UPDATE
		  SET count = notify_quotas.count + 1
		  WHERE notify_quotas.count < $4
		RETURNING count`,
		userID, channel, windowStart, limit).Scan(&count)
	if err != nil {
		// Tidak ada baris yang kembali berarti WHERE menolak kenaikan:
		// kuota bucket ini sudah penuh.
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ReleaseQuota mengembalikan satu slot yang sudah diambil tetapi ternyata tidak
// dipakai, misalnya karena gate lain menolak setelah kuota diambil.
//
// Tanpa ini, setiap penolakan di tengah jalan memakan jatah user tanpa ada
// pesan yang benar-benar terkirim.
func ReleaseQuota(ctx context.Context, pool *pgxpool.Pool, userID, channel string, window time.Duration) error {
	if userID == "" {
		return nil
	}
	windowStart := time.Now().Truncate(window)
	_, err := pool.Exec(ctx, `
		UPDATE notify_quotas
		SET count = GREATEST(count - 1, 0)
		WHERE user_id = $1 AND channel = $2 AND window_start = $3`,
		userID, channel, windowStart)
	return err
}

// QuotaUsed melaporkan berapa slot yang sudah terpakai pada bucket berjalan.
//
// Dipakai jalur digest yang hanya ingin melewati user yang sudah penuh, tanpa
// memesan slot. Enqueue bukan pengiriman: slot baru boleh dipakai saat email
// benar-benar keluar.
func QuotaUsed(ctx context.Context, pool *pgxpool.Pool, userID, channel string, window time.Duration) (int, error) {
	if userID == "" {
		return 0, nil
	}
	windowStart := time.Now().Truncate(window)
	var count int
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(count, 0) FROM notify_quotas
		WHERE user_id = $1 AND channel = $2 AND window_start = $3`,
		userID, channel, windowStart).Scan(&count)
	if err != nil {
		if isNoRows(err) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}
