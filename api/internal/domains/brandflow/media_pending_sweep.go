package brandflow

import (
	"context"
	"log/slog"
	"time"
)

// Perubahan media yang diparkir mitra (media.pending_values, lihat
// partner.MediaUpdate) tidak boleh menggantung: admin diingatkan setelah
// mediaChangeRemindAfter, dan pengajuan dibatalkan otomatis setelah
// mediaChangeExpireAfter. Media tetap tayang dengan data live di kedua kasus.
const (
	mediaChangeRemindAfter = 24 * time.Hour
	mediaChangeExpireAfter = 48 * time.Hour
)

// schSweepMediaPendingChanges: pengingat admin (sekali per pengajuan) lalu
// auto-reject. Mengembalikan jumlah pengingat dan jumlah pengajuan kedaluwarsa.
func (d *Deps) schSweepMediaPendingChanges(ctx context.Context, limit int) (reminded, expired int) {
	if limit < 1 {
		return 0, 0
	}

	// Kedaluwarsa dulu: pengajuan yang sudah lewat 48 jam tidak perlu diingatkan.
	expiredRows, err := d.queryRows(ctx, `
		UPDATE media SET pending_values = NULL, pending_changes = NULL, pending_requested_at = NULL,
		       pending_reminded_at = NULL, updated_at = NOW()
		WHERE id IN (
		  SELECT id FROM media
		  WHERE status = 'active' AND pending_values IS NOT NULL
		    AND pending_requested_at < NOW() - make_interval(secs => $1)
		  ORDER BY pending_requested_at
		  LIMIT $2 FOR UPDATE SKIP LOCKED)
		RETURNING id, name, partner_id`, mediaChangeExpireAfter.Seconds(), limit)
	if err != nil {
		slog.Error("[scheduler] sweep pengajuan perubahan media (kedaluwarsa) gagal", "err", err)
	}
	for _, row := range expiredRows {
		mediaID := row.strOr("id", "")
		mediaName := row.strOr("name", "Media")
		expired++
		d.audit(ctx, "", "", "media.change_auto_rejected", "media", mediaID)

		partner, _ := d.queryRow(ctx,
			"SELECT user_id, COALESCE(public_id, id) AS link_id FROM partners WHERE id = $1 LIMIT 1",
			row.strOr("partner_id", ""))
		if partner == nil || partner.strOr("user_id", "") == "" {
			continue
		}
		link := "/mitra/" + partner.strOr("link_id", "") + "/media"
		if err := d.exec(ctx, `
			INSERT INTO notifications (id, user_id, type, title, body, link)
			VALUES (gen_random_uuid(), $1, 'media.rejected', $2, $3, $4)`,
			partner.strOr("user_id", ""), "Pengajuan perubahan media kedaluwarsa",
			`Pengajuan perubahan media "`+mediaName+`" belum direview dalam 2×24 jam sehingga dibatalkan otomatis. Media tetap tayang dengan data lama; silakan ajukan kembali.`,
			link); err != nil {
			slog.Warn("[scheduler] notifikasi kedaluwarsa perubahan media gagal", "media", mediaID, "err", err)
		}
	}

	// pending_reminded_at menjadi penanda klaim: UPDATE ... RETURNING memastikan
	// satu pengajuan hanya diingatkan sekali walau scheduler berjalan ganda.
	remindRows, err := d.queryRows(ctx, `
		UPDATE media SET pending_reminded_at = NOW()
		WHERE id IN (
		  SELECT id FROM media
		  WHERE status = 'active' AND pending_values IS NOT NULL AND pending_reminded_at IS NULL
		    AND pending_requested_at < NOW() - make_interval(secs => $1)
		  ORDER BY pending_requested_at
		  LIMIT $2 FOR UPDATE SKIP LOCKED)
		RETURNING id, name`, mediaChangeRemindAfter.Seconds(), limit)
	if err != nil {
		slog.Error("[scheduler] sweep pengajuan perubahan media (pengingat) gagal", "err", err)
	}
	link := "/admin/media?status=pending_review#tab-pending"
	for _, row := range remindRows {
		reminded++
		d.notifyPlatformAdmins(ctx, "media.submitted", "Perubahan media belum direview",
			`Perubahan media "`+row.strOr("name", "Media")+`" sudah menunggu lebih dari 24 jam dan akan dibatalkan otomatis pada jam ke-48.`, &link)
	}
	return reminded, expired
}
