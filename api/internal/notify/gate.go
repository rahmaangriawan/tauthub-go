package notify

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dua gerbang yang menahan pengiriman berbayar sebelum biaya keluar.
//
// Keduanya menjawab pertanyaan berbeda:
//   - SuppressedByRead: user sudah melihat kabar ini di aplikasi, jadi email
//     atau WA-nya tidak lagi membawa informasi baru.
//   - AlreadySentRecently: kanal ini sudah pernah mengirim event yang sama,
//     jadi kiriman kedua hanya menggandakan biaya.

// SuppressedByRead melaporkan apakah notifikasi in-app bertipe sama sudah
// dibaca user dalam jendela waktu terakhir.
//
// dedupeKey boleh kosong; kalau kosong, pencocokan hanya memakai tipe. Untuk
// event yang bisa berulang pada entitas berbeda (mis. pengingat tenggat pada
// dua publikasi), pemanggil sebaiknya mengirim dedupe key supaya pembacaan
// publikasi A tidak membatalkan pengingat publikasi B.
func SuppressedByRead(ctx context.Context, pool *pgxpool.Pool, userID, notifType, dedupeKey string, window time.Duration) (bool, error) {
	if userID == "" || notifType == "" {
		return false, nil
	}
	var exists bool
	var err error
	if dedupeKey != "" {
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM notifications
				WHERE user_id = $1
				  AND type = $2
				  AND dedupe_key = $3
				  AND read_at IS NOT NULL
				  AND created_at > now() - $4::interval
				LIMIT 1
			)`, userID, notifType, dedupeKey, intervalString(window)).Scan(&exists)
	} else {
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM notifications
				WHERE user_id = $1
				  AND type = $2
				  AND read_at IS NOT NULL
				  AND created_at > now() - $3::interval
				LIMIT 1
			)`, userID, notifType, intervalString(window)).Scan(&exists)
	}
	if err != nil {
		return false, err
	}
	return exists, nil
}

// AlreadySentRecently melaporkan apakah kanal ini sudah mengirim event dengan
// dedupe key yang sama dalam jendela waktu terakhir.
//
// channel: "email" atau "wa". Nilai lain dianggap tidak pernah terkirim supaya
// salah ketik nama kanal tidak diam-diam memblokir pengiriman.
func AlreadySentRecently(ctx context.Context, pool *pgxpool.Pool, channel, dedupeKey string, window time.Duration) (bool, error) {
	if dedupeKey == "" {
		return false, nil
	}
	var table string
	switch channel {
	case "email":
		table = "email_deliveries"
	case "wa":
		table = "wa_deliveries"
	default:
		return false, nil
	}
	// Nama tabel berasal dari switch di atas, bukan dari input, jadi aman
	// dirangkai ke query.
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM `+table+`
			WHERE dedupe_key = $1
			  AND created_at > now() - $2::interval
			LIMIT 1
		)`, dedupeKey, intervalString(window)).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// intervalString mengubah durasi ke literal interval Postgres.
//
// Postgres tidak menerima bentuk "1h0m0s" milik Go, jadi durasi dikonversi ke
// jumlah detik. Parameter tetap terikat, tidak ada input user yang dirangkai
// ke SQL.
func intervalString(d time.Duration) string {
	if d <= 0 {
		return "0 seconds"
	}
	secs := int64(d / time.Second)
	if secs < 1 {
		secs = 1
	}
	return strconv.FormatInt(secs, 10) + " seconds"
}
