package email

import (
	"context"
	"errors"
	"log/slog"
	"net/textproto"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Daftar alamat mati (hard bounce). Tabel email_suppressions diisi saat server
// SMTP menolak penerima secara permanen; alamat di dalamnya tidak dikirimi lagi.
//
// Semua fungsi di sini gagal-terbuka: bila tabel belum ada atau query gagal,
// email tetap dikirim seperti biasa. Daftar ini hanya pengaman reputasi, tidak
// boleh sampai menghentikan surel penting.

// hardBounceHints: potongan pesan penolakan SMTP yang berarti alamat tujuan tidak
// ada. Penolakan karena kebijakan atau spam (5.7.x) sengaja tidak termasuk,
// karena itu bukan bukti alamatnya mati.
var hardBounceHints = []string{
	"5.1.1", "5.1.2", "5.1.3", "user unknown", "unknown user", "no such user",
	"does not exist", "doesn't exist", "invalid recipient", "recipient rejected",
	"mailbox not found", "mailbox unavailable", "address rejected", "no mailbox here",
}

// IsHardBounce: true bila err penolakan permanen (5xx) yang menyatakan alamat
// penerima tidak ada. Penolakan sementara (4xx) dan galat jaringan bukan bounce.
func IsHardBounce(err error) bool {
	var te *textproto.Error
	if !errors.As(err, &te) || te.Code < 500 || te.Code > 599 {
		return false
	}
	msg := strings.ToLower(te.Msg)
	for _, h := range hardBounceHints {
		if strings.Contains(msg, h) {
			return true
		}
	}
	return false
}

// IsHardBounceReason: sama dengan IsHardBounce, tetapi untuk teks alasan dari
// webhook penyedia (mis. "550 5.1.1 mailbox unavailable"), yang tidak membawa
// jenis bounce terpisah. Harus berawalan kode 5xx supaya penolakan sementara
// (4xx) tidak ikut tercatat.
func IsHardBounceReason(reason string) bool {
	r := strings.ToLower(strings.TrimSpace(reason))
	if r == "" || r[0] != '5' {
		return false
	}
	for _, h := range hardBounceHints {
		if strings.Contains(r, h) {
			return true
		}
	}
	return false
}

// IsSuppressed: true bila alamat pernah memantul permanen. Baris bertipe
// "complaint" hanya informasi untuk admin dan tidak menghentikan pengiriman,
// supaya orang yang menandai satu email sebagai spam tetap bisa menerima email
// keamanan seperti reset password.
func IsSuppressed(ctx context.Context, pool *pgxpool.Pool, email string) bool {
	var one int
	err := pool.QueryRow(ctx,
		"SELECT 1 FROM email_suppressions WHERE email = $1 AND reason = 'hard_bounce' LIMIT 1",
		strings.ToLower(strings.TrimSpace(email))).Scan(&one)
	return err == nil
}

// MarkSuppressed mencatat alamat mati. Best-effort.
func MarkSuppressed(ctx context.Context, pool *pgxpool.Pool, email, reason, detail string) {
	_, err := pool.Exec(ctx, `
		INSERT INTO email_suppressions (email, reason, detail) VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING`,
		strings.ToLower(strings.TrimSpace(email)), reason, truncate(detail, 500))
	if err != nil {
		slog.Warn("[email] gagal mencatat suppression", "err", err)
	}
}

// markSkipped menandai pengiriman selesai tanpa dikirim dan tanpa dicoba ulang.
func markSkipped(ctx context.Context, pool *pgxpool.Pool, id, code, message string) {
	_, _ = pool.Exec(ctx,
		"UPDATE email_deliveries SET status = 'skipped', error_code = $1, error_message = $2 WHERE id = $3",
		code, truncate(message, 500), id)
}
