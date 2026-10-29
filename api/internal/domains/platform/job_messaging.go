// Job messaging platform: port kind `send_email` (lib/email/sender.ts
// processEmailQueue) dan `send_wa` (lib/wa/queue.ts processWaQueue).
//
// Keduanya adalah QUEUE DRAIN, bukan job per-baris: satu pemanggilan mengambil
// sampai 10 baris due lalu memprosesnya satu per satu. Handler per-delivery
// email sudah ada di internal/email (ProcessEmailInline + marker queue) dan
// client Fonnte sudah ada di misc_integrations.go (igWASendMessage) — file ini
// hanya menambahkan loop drain + pemilihan baris due yang belum di-port.
package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"unicode/utf8"

	emailqueue "github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

const (
	// processEmailQueue(10) / processWaQueue(10) di app lama.
	mjEmailDrainLimit = 10
	mjWADrainLimit    = 10
	// MAX_ATTEMPTS di lib/wa/queue.ts.
	mjWAMaxAttempts = 3
)

// mjCmsSecret: cmsEncryptionSecret dengan fallback APP_SECRET (parity
// serverConfig.cmsEncryptionSecret) — dipakai untuk membuka password SMTP sealed.
func (d *Deps) mjCmsSecret() string {
	if d == nil || d.Cfg == nil {
		return ""
	}
	if d.Cfg.CmsEncryptionSecret != "" {
		return d.Cfg.CmsEncryptionSecret
	}
	return d.Cfg.Secret
}

// ---------- send_email ----------

// mjPickDueEmails: parity pickDueEmails — baris queued/failed yang waktunya tiba
// dan percobaannya belum habis. `next_attempt_at <= NOW()` sekaligus menangani
// baris overdue (mis. job tertinggal lama), terurut paling lama dulu.
func (d *Deps) mjPickDueEmails(ctx context.Context, limit int) ([]string, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id FROM email_deliveries
		WHERE status IN ('queued','failed') AND next_attempt_at <= NOW() AND attempts < max_attempts
		ORDER BY next_attempt_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SendEmailQueueJob: kind `send_email` — drain sampai 10 delivery due.
//
// Klaim/status per-baris sepenuhnya memakai marker internal/email (MarkSending,
// MarkSent, MarkFailed) lewat ProcessEmailInline supaya status, attempts,
// next_attempt_at (backoff 2^attempts menit), dan error_code NOT_CONFIGURED
// sama dengan app lama. Proses ini idempotent: klaim `status IN ('queued','failed')`
// + `next_attempt_at <= NOW()` membuat baris yang sudah dipegang worker lain
// terlewat, dan job boleh dijalankan berulang.
func (d *Deps) SendEmailQueueJob(ctx context.Context) error {
	if d == nil || d.Pool == nil {
		return fmt.Errorf("send_email: pool database belum dikonfigurasi")
	}

	secret := d.mjCmsSecret()

	// Antrean kosong → sukses no-op (parity processEmailQueue: pickDueEmails
	// dulu, barulah proses). Konfigurasi SMTP tidak disyaratkan bila tak ada
	// yang perlu dikirim.
	ids, err := d.mjPickDueEmails(ctx, mjEmailDrainLimit)
	if err != nil {
		return fmt.Errorf("send_email: gagal membaca antrean email_deliveries: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}

	// Preflight konfigurasi SMTP sebelum menyentuh baris: bila belum lengkap,
	// berhenti dengan error jelas tanpa membakar attempts — baris tetap 'queued'
	// sampai admin memperbaiki smtp_settings.
	cfg, cfgErr := emailqueue.ResolveConfig(ctx, d.Pool, secret)
	if cfgErr != nil {
		return fmt.Errorf("send_email: SMTP belum dikonfigurasi: %w", cfgErr)
	}
	if cfg == nil {
		return fmt.Errorf("send_email: SMTP belum dikonfigurasi")
	}

	processed, sent, failed := 0, 0, 0
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		ok, _, msg := emailqueue.ProcessEmailInline(ctx, d.Pool, secret, id)
		processed++
		if ok {
			sent++
			continue
		}
		failed++
		slog.Warn("[queue] send_email delivery gagal", "delivery", id, "err", msg)
	}
	slog.Info("[queue] send_email selesai", "processed", processed, "sent", sent, "failed", failed)
	return nil
}

// ---------- send_wa ----------

// mjWADueRow: kolom yang dibutuhkan drain WA (parity DeliveryRow).
type mjWADueRow struct {
	ID           string
	Target       string
	RenderedBody string
	Attempts     int
}

// mjPickDueWA: klaim atomik baris yang siap kirim, lalu kembalikan isinya.
//
// Klaim memakai satu UPDATE ... RETURNING yang menandai baris 'sending' dan
// mengisi claimed_at. Ini menggantikan SELECT ... FOR UPDATE SKIP LOCKED:
// kunci baris hanya bertahan selama transaksi, sedangkan pemungutan di sini
// autocommit, jadi tanpa penandaan status dua job send_wa (worker dan
// scheduler bisa sama-sama memungut) akan mengambil baris yang sama dan
// mengirim pesan dua kali.
//
// next_attempt_at memberi jeda setelah kegagalan; sebelumnya baris yang gagal
// langsung kembali 'queued' sehingga dicoba lagi dalam hitungan detik.
func (d *Deps) mjPickDueWA(ctx context.Context, limit int) ([]mjWADueRow, error) {
	rows, err := d.Pool.Query(ctx, `
		WITH due AS (
			SELECT id FROM wa_deliveries
			WHERE status = 'queued'
			  AND attempts < $1
			  AND (next_attempt_at IS NULL OR next_attempt_at <= now())
			ORDER BY created_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE wa_deliveries w
		SET status = 'sending', claimed_at = now()
		FROM due
		WHERE w.id = due.id
		RETURNING w.id, w.target, w.rendered_body, w.attempts`, mjWAMaxAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []mjWADueRow{}
	for rows.Next() {
		var r mjWADueRow
		if err := rows.Scan(&r.ID, &r.Target, &r.RenderedBody, &r.Attempts); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// mjWAMarkSent: parity markSent.
func (d *Deps) mjWAMarkSent(ctx context.Context, id string) {
	_, _ = d.Pool.Exec(ctx,
		"UPDATE wa_deliveries SET status = 'sent', sent_at = NOW(), attempts = attempts + 1, last_error = NULL, claimed_at = NULL WHERE id = $1", id)
}

// mjWAMarkFailed: kembali 'queued' selama percobaan belum habis, 'failed' saat
// attempts+1 mencapai MAX_ATTEMPTS.
//
// next_attempt_at diisi jarak eksponensial (2^attempts menit) supaya gateway
// tidak ditembak berulang dalam hitungan detik. Tanpa jeda ini, scheduler yang
// berjalan tiap 10 detik akan mencoba pesan yang sama tiga kali hampir
// bersamaan dan menghabiskan kuota Fonnte untuk penolakan yang sama.
func (d *Deps) mjWAMarkFailed(ctx context.Context, id, message string, attempts int) {
	status := "queued"
	if attempts+1 >= mjWAMaxAttempts {
		status = "failed"
	}
	// 2^attempts menit: 1, 2, 4 menit. Dibatasi supaya tidak melompat jauh.
	delayMinutes := 1 << attempts
	if delayMinutes > 30 {
		delayMinutes = 30
	}
	_, _ = d.Pool.Exec(ctx,
		`UPDATE wa_deliveries
		    SET status = $1, attempts = attempts + 1, last_error = $2,
		        claimed_at = NULL,
		        next_attempt_at = CASE WHEN $1 = 'queued'
		                               THEN now() + ($4 || ' minutes')::interval
		                               ELSE next_attempt_at END
		  WHERE id = $3`,
		status, mjClip500(message), id, strconv.Itoa(delayMinutes))
}

// mjWAMarkFailedPermanent: penolakan permanen (token invalid, nomor salah, kuota)
// — langsung 'failed' supaya tidak membuang percobaan berulang.
func (d *Deps) mjWAMarkFailedPermanent(ctx context.Context, id, message string) {
	_, _ = d.Pool.Exec(ctx,
		"UPDATE wa_deliveries SET status = 'failed', attempts = attempts + 1, last_error = $1, claimed_at = NULL WHERE id = $2",
		mjClip500(message), id)
}

// mjClip500: parity message.slice(0, 500), dipotong di batas rune supaya kolom
// teks tidak menerima UTF-8 rusak.
func mjClip500(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// SendWAQueueJob: kind `send_wa` — drain sampai 10 pesan WA queued.
//
// Klasifikasi kegagalan mengikuti app lama: respons Fonnte dengan status=false
// (penolakan) memakai aturan attempts (kembali queued sampai 3x), error 503
// (gateway timeout/tidak terjangkau) juga retryable, sedangkan error lain
// (tidak dikonfigurasi, nomor/isi kosong, respons malformed) langsung final
// 'failed'. Sukses selalu menandai 'sent'.
func (d *Deps) SendWAQueueJob(ctx context.Context) error {
	if d == nil || d.Pool == nil {
		return fmt.Errorf("send_wa: pool database belum dikonfigurasi")
	}

	due, err := d.mjPickDueWA(ctx, mjWADrainLimit)
	if err != nil {
		return fmt.Errorf("send_wa: gagal membaca antrean wa_deliveries: %w", err)
	}
	if len(due) == 0 {
		// Tidak ada antrean → sukses no-op.
		return nil
	}

	// Preflight kredensial: tanpa token setiap baris akan ditolak permanen dan
	// antrean ikut hangus. Lebih aman berhenti sekarang dengan error jelas —
	// baris tetap 'queued' dan terkirim saat admin melengkapi konfigurasi.
	token, tokErr := d.igWAToken(ctx)
	if tokErr != nil {
		return fmt.Errorf("send_wa: gagal membaca konfigurasi WA Gateway: %w", tokErr)
	}
	if token == "" {
		return fmt.Errorf("send_wa: WA Gateway belum dikonfigurasi oleh Admin.")
	}

	processed, sent, failed := 0, 0, 0
	for _, row := range due {
		if ctx.Err() != nil {
			break
		}
		result, status, sendErr := d.igWASendMessage(ctx, row.Target, row.RenderedBody)
		processed++
		switch {
		case sendErr == nil && result.OK:
			d.mjWAMarkSent(ctx, row.ID)
			sent++
		case sendErr == nil:
			// Gateway membalas tetapi menolak pesan (status body false).
			d.mjWAMarkFailed(ctx, row.ID, result.Detail, row.Attempts)
			failed++
			slog.Warn("[queue] send_wa ditolak gateway", "id", row.ID, "detail", result.Detail)
		case status == http.StatusServiceUnavailable:
			// Timeout / tidak terjangkau → retryable sampai batas percobaan.
			d.mjWAMarkFailed(ctx, row.ID, sendErr.Error(), row.Attempts)
			failed++
			slog.Warn("[queue] send_wa gagal retryable", "id", row.ID, "err", sendErr)
		default:
			// Konfigurasi/token/nomor bermasalah — tidak berguna diulang.
			d.mjWAMarkFailedPermanent(ctx, row.ID, sendErr.Error())
			failed++
			slog.Warn("[queue] send_wa gagal permanen", "id", row.ID, "status", status, "err", sendErr)
		}
	}
	slog.Info("[queue] send_wa selesai", "processed", processed, "sent", sent, "failed", failed)
	return nil
}

// RegisterMessagingJobs dipanggil dari cmd/api saat boot.
func (d *Deps) RegisterMessagingJobs() {
	jobs.Register("send_email", func(ctx context.Context, job jobs.JobRow) error {
		return d.SendEmailQueueJob(ctx)
	})
	jobs.Register("send_wa", func(ctx context.Context, job jobs.JobRow) error {
		return d.SendWAQueueJob(ctx)
	})
}
