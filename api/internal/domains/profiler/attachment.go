package profiler

// attachment.go — operasi ledger chat_attachments: reservasi kuota saat
// unggah, finalisasi status setelah diproses, dan penandaan siap-hapus saat
// user menghapus lampiran secara eksplisit.

import (
	"context"
	"time"
)

// reserveAttachment: cek kuota lalu pesan slotnya dalam SATU transaksi.
// Advisory lock per user membuat check-then-insert atomik terhadap unggahan
// bersamaan — dua request paralel user yang sama tidak bisa sama-sama lolos
// saat sisa kuota tinggal satu.
//
// user_id tanpa FK (lihat migrasi 247); profile_id sudah diverifikasi
// accessOrFail + profileInBrand oleh pemanggil, jadi lingkupnya aman.
func (d *Deps) reserveAttachment(ctx context.Context, userID, profileID, kind, source, storageKey, filename, mime string, size int64) error {
	lim := LoadUploadLimits()
	viaChat := source != "library"

	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "chat_upload:"+userID); err != nil {
		return err
	}

	var u UploadUsage
	if err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE profile_id = $1 AND source = 'chat' AND status <> 'deleted'),
			COALESCE(SUM(size_bytes) FILTER (WHERE profile_id = $1 AND source = 'chat' AND status <> 'deleted'), 0),
			COUNT(*) FILTER (WHERE user_id = $2 AND created_at >= date_trunc('day', now())),
			COALESCE(SUM(size_bytes) FILTER (WHERE user_id = $2 AND created_at >= date_trunc('day', now())), 0),
			COALESCE(SUM(size_bytes) FILTER (WHERE user_id = $2 AND status <> 'deleted'), 0)
		FROM chat_attachments
		WHERE (profile_id = $1 OR user_id = $2)`, profileID, userID).
		Scan(&u.ConversationFiles, &u.ConversationBytes, &u.DailyFiles, &u.DailyBytes, &u.ActiveTempBytes); err != nil {
		return err
	}

	if v := lim.Check(u, size, viaChat); v != nil {
		return v
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO chat_attachments (id, user_id, profile_id, kind, source, storage_key, original_filename, mime_type, size_bytes, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'processing')`,
		storageKey, userID, profileID, kind, source, storageKey, filename, mime, size)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// finalizeAttachment: tutup siklus reservasi setelah berkas diproses.
// status 'processed' → berkas hidup sampai expires_at; 'failed' → umurnya
// disingkat (retensi tetap, cukup untuk user melihat error lalu dibersihkan).
func (d *Deps) finalizeAttachment(ctx context.Context, storageKey, kind, status, parseErr string) {
	lim := LoadUploadLimits()
	expires := expiryFor(kind, lim, time.Now())
	if status == "failed" {
		// Gagal diproses = tidak bernilai; berkas fisiknya boleh pergi lebih
		// cepat, tetap dibatasi umur (MaxUnprocessedAge) bukan seketika.
		expires = time.Now().Add(lim.MaxUnprocessedAge)
	}
	var errPtr any
	if parseErr != "" {
		errPtr = parseErr
	}
	_, _ = d.Pool.Exec(ctx, `
		UPDATE chat_attachments
		SET status = $2, processed_at = now(), expires_at = $3, parse_error = $4
		WHERE storage_key = $1 AND status = 'processing'`,
		storageKey, status, expires, errPtr)
}

// markAttachmentPendingDelete: berkas milik lampiran yang dihapus user
// (dokumen/gambar) masuk antrean penghapusan fisik pada siklus cleanup
// berikutnya — tidak menghapus langsung supaya tetap aman bila sedang dibaca.
func (d *Deps) markAttachmentPendingDelete(ctx context.Context, storageKey string) {
	_, _ = d.Pool.Exec(ctx, `
		UPDATE chat_attachments SET status = 'delete_pending', expires_at = now()
		WHERE storage_key = $1 AND status NOT IN ('delete_pending', 'deleted')`, storageKey)
}
