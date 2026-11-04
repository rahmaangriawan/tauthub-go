package brandflow

import (
	"context"
	"log/slog"
	"time"
)

// Email yang meminta kode OTP pendaftaran tetapi tidak pernah memverifikasinya
// dalam registrationBlockAfter masuk registration_email_blocks: tidak bisa
// minta kode atau mendaftar lagi sampai admin membukanya. Baris kode yang
// sudah dinilai ditandai consumed_at supaya tidak diproses ulang.
const registrationBlockAfter = 48 * time.Hour

// schSweepUnansweredEmailCodes: mengembalikan jumlah email yang baru diblokir.
func (d *Deps) schSweepUnansweredEmailCodes(ctx context.Context, limit int) int {
	if limit < 1 {
		return 0
	}
	rows, err := d.queryRows(ctx, `
		WITH stale AS (
		  SELECT DISTINCT c.email FROM email_verification_codes c
		  WHERE c.purpose = 'register' AND c.verified_at IS NULL AND c.consumed_at IS NULL
		    AND c.created_at < NOW() - make_interval(secs => $1)
		    AND NOT EXISTS (SELECT 1 FROM users u WHERE u.email = c.email)
		  LIMIT $2
		), blocked AS (
		  INSERT INTO registration_email_blocks (email, reason)
		  SELECT email, 'otp_unanswered_48h' FROM stale
		  ON CONFLICT (email) DO NOTHING
		  RETURNING email
		), closed AS (
		  UPDATE email_verification_codes SET consumed_at = NOW()
		  WHERE email IN (SELECT email FROM stale) AND consumed_at IS NULL
		)
		SELECT email FROM blocked`, registrationBlockAfter.Seconds(), limit)
	if err != nil {
		slog.Error("[scheduler] sweep OTP pendaftaran tak dijawab gagal", "err", err)
		return 0
	}
	if len(rows) > 0 {
		slog.Info("[scheduler] email diblokir dari pendaftaran (OTP tak dijawab 48 jam)", "jumlah", len(rows))
	}
	return len(rows)
}
