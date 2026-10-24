package profiler

// attachment_cleanup.go — worker pembersih berkas unggahan sementara.
//
// Dijalankan cmd/scheduler sebagai goroutine dengan konteks cancel-able
// (berhenti rapi saat SIGTERM). Setiap siklus:
//
//   1. Paksa-kadaluarsa berkas yang tertinggal 'uploaded'/'processing'
//      melebihi MaxUnprocessedAge (prosesnya mati di tengah jalan) — file
//      tidak boleh menggantung selamanya hanya karena tidak pernah final.
//   2. Tandai 'delete_pending' untuk: processed/failed yang sudah lewat
//      expires_at, dan lampiran milik percakapan (profil) yang sudah
//      dihapus — penghapusan conversation mengikuti kebijakan retensi
//      attachment, tidak seketika.
//   3. Hapus fisik per batch (FOR UPDATE SKIP LOCKED, aman bila beberapa
//      proses menjalankan worker): os.Remove dulu, baru metadata. Gagal
//      menghapus → baris tetap 'delete_pending' dan dicoba siklus berikutnya.
//      Berkas 'processing' tidak pernah tersentuh langkah ini.
//   4. Sapuan yatim: berkas di UPLOAD_DIR tanpa metadata SETELAH grace
//      period. Penghapusan fisik yatim DIKUNCI env UPLOAD_ORPHAN_CLEANUP=true
//      (default hanya melapor) dan hanya untuk berkas bernama pola internal
//      uuid.ekstensi — nama user tidak pernah jadi path.
//   5. Log metrik: total aktif, per-status, disk.
//
// Efek samping yang MENJAGA konsistensi UI/AI:
//   dokumen → profiler_documents.file_deleted_at ditandai (baris & teks
//             ekstraksi DIPERTAHANKAN sebagai memori AI);
//   gambar  → profiler_images.di-soft-delete (berkasnya memang asetnya).

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// UploadCleanupStats: angka satu siklus cleanup (dilog + bisa dipakai metrik).
type UploadCleanupStats struct {
	ForceExpired    int64 // tertinggal processing → failed (dipaksa)
	MarkedExpired   int64 // processed/failed → delete_pending (kadaluarsa)
	MarkedByProfile int64 // → delete_pending karena profil dihapus
	Deleted         int64 // berkas fisik terhapus + ledger 'deleted'
	DeleteFailures  int64 // os.Remove gagal (dicoba lagi siklus berikutnya)
	OrphansFound    int64 // berkas tanpa metadata melewati grace
	OrphansDeleted  int64
	TimelinePurged  int64 // post timeline > 3 bulan yang dihapus
	ActiveFiles     int64
	ActiveBytes     int64
	DiskFreeBytes   uint64
	DiskTotalBytes  uint64
	DiskUsedPct     float64
}

const cleanupBatchSize = 100

// RunUploadCleanupLoop: ticker sampai ctx dibatalkan. Interval dari
// UPLOAD_CLEANUP_INTERVAL_MINUTES (default 15 menit).
func (d *Deps) RunUploadCleanupLoop(ctx context.Context) {
	lim := LoadUploadLimits()
	interval := lim.CleanupInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	slog.Info("[upload-cleanup] worker aktif", "interval", interval.String(), "dir", uploadDir())
	for {
		select {
		case <-ctx.Done():
			slog.Info("[upload-cleanup] worker berhenti")
			return
		case <-ticker.C:
			d.RunUploadCleanup(ctx)
		}
	}
}

// RunUploadCleanup: satu siklus penuh. Aman dipanggil berkali-kali.
func (d *Deps) RunUploadCleanup(ctx context.Context) UploadCleanupStats {
	lim := LoadUploadLimits()
	var st UploadCleanupStats

	// 1. Berkas macet di 'uploaded'/'processing' — proses pemiliknya mati
	//    (deploy/restart di tengah request). Batas umurnya sendiri,
	//    terpisah dari retensi berkas yang selesai diproses.
	if tag, err := d.Pool.Exec(ctx, `
		UPDATE chat_attachments
		SET status = 'failed',
		    parse_error = 'Terhenti di tengah pemrosesan (dibersihkan otomatis).',
		    expires_at = now() + ($1 || ' seconds')::interval
		WHERE status IN ('uploaded', 'processing')
		  AND created_at < now() - ($2 || ' seconds')::interval`,
		strconv.Itoa(int(lim.MaxUnprocessedAge.Seconds())),
		strconv.Itoa(int(lim.MaxUnprocessedAge.Seconds()))); err == nil {
		st.ForceExpired = tag.RowsAffected()
	} else {
		slog.Error("[upload-cleanup] paksa-kadaluarsa berkas macet gagal", "err", err)
	}

	// 2a. Kadaluarsa retensi.
	if tag, err := d.Pool.Exec(ctx, `
		UPDATE chat_attachments SET status = 'delete_pending'
		WHERE status IN ('processed', 'failed')
		  AND expires_at IS NOT NULL AND expires_at <= now()`); err == nil {
		st.MarkedExpired = tag.RowsAffected()
	} else {
		slog.Error("[upload-cleanup] tandai kadaluarsa gagal", "err", err)
	}

	// 2b. Profil (percakapan) sudah dihapus > retensi → lampirannya ikut
	//     dibersihkan, baik gambar (retensi panjang) maupun dokumen.
	if tag, err := d.Pool.Exec(ctx, `
		UPDATE chat_attachments ca SET status = 'delete_pending'
		FROM profiler_profiles p
		WHERE ca.profile_id = p.id
		  AND p.deleted_at IS NOT NULL
		  AND p.deleted_at < now() - ($1 || ' seconds')::interval
		  AND ca.status IN ('processed', 'failed')`,
		strconv.Itoa(int(lim.Retention.Seconds()))); err == nil {
		st.MarkedByProfile = tag.RowsAffected()
	} else {
		slog.Error("[upload-cleanup] tandai lampiran profil terhapus gagal", "err", err)
	}

	// 3. Hapus fisik per batch. SKIP LOCKED: bila dua proses menjalankan
	//    siklus bersamaan, tidak ada baris yang dipegang dua kali.
	for batch := 0; batch < 20; batch++ {
		n := d.deletePendingBatch(ctx, &st)
		if n < cleanupBatchSize {
			break
		}
	}

	// 4. Sapuan yatim (default: lapor saja; hapus fisik via opt-in env).
	d.sweepOrphans(ctx, lim, &st)

	// 4b. Timeline brief: post lebih tua dari 3 bulan dihapus beserta
	//     gambarnya (gambar ikut delete_pending lewat releaseTimelineImage).
	if n := d.purgeExpiredTimeline(ctx); n > 0 {
		st.TimelinePurged = n
		slog.Info("[upload-cleanup] timeline kedaluwarsa dihapus", "jumlah", n)
	}

	// 5. Metrik penyimpanan.
	d.collectStorageStats(ctx, &st)

	slog.Info("[upload-cleanup] siklus selesai",
		"dipaksa_gagal", st.ForceExpired,
		"ditandai_kadaluarsa", st.MarkedExpired,
		"ditandai_profil_terhapus", st.MarkedByProfile,
		"terhapus", st.Deleted,
		"gagal_hapus", st.DeleteFailures,
		"yatim_ditemukan", st.OrphansFound,
		"yatim_dihapus", st.OrphansDeleted,
		"berkas_aktif", st.ActiveFiles,
		"total_aktif_mb", st.ActiveBytes>>20,
		"disk_terpakai_persen", strconv.FormatFloat(st.DiskUsedPct, 'f', 1, 64),
	)
	return st
}

// deletePendingBatch: satu batch penghapusan fisik + metadata. Mengembalikan
// jumlah baris yang diproses (termasuk yang gagal dihitung ulang siklus ini).
func (d *Deps) deletePendingBatch(ctx context.Context, st *UploadCleanupStats) int {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		slog.Error("[upload-cleanup] mulai tx gagal", "err", err)
		return cleanupBatchSize // coba lagi siklus berikutnya; hentikan loop batch
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, kind, storage_key FROM chat_attachments
		WHERE status = 'delete_pending'
		ORDER BY expires_at NULLS FIRST
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, cleanupBatchSize)
	if err != nil {
		slog.Error("[upload-cleanup] ambil batch delete_pending gagal", "err", err)
		return cleanupBatchSize
	}
	type pending struct {
		id, kind, key string
	}
	var items []pending
	for rows.Next() {
		var it pending
		if err := rows.Scan(&it.id, &it.kind, &it.key); err != nil {
			continue
		}
		items = append(items, it)
	}
	rows.Close()
	if len(items) == 0 {
		return 0
	}

	var doneIDs, docKeys, imgKeys []string
	for _, it := range items {
		path := filepath.Join(uploadDir(), filepath.FromSlash(it.key))
		// Path internal selalu nama file tunggal buatan server; pemeriksaan
		// ekstra untuk berjaga bila data lama bermasalah.
		if filepath.Base(path) != it.key {
			slog.Error("[upload-cleanup] storage_key mencurigakan, lewati", "key", it.key)
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			// Gagal (mis. izin/EACCES): catat & coba lagi siklus berikutnya —
			// metadata TIDAK ditandai deleted sebelum fisiknya benar hilang.
			st.DeleteFailures++
			slog.Error("[upload-cleanup] hapus fisik gagal", "key", it.key, "err", err)
			continue
		}
		st.Deleted++
		doneIDs = append(doneIDs, it.id)
		if it.kind == "image" {
			imgKeys = append(imgKeys, it.key)
		} else {
			docKeys = append(docKeys, it.key)
		}
	}
	if len(doneIDs) == 0 {
		return len(items)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE chat_attachments SET status = 'deleted', deleted_at = now()
		WHERE id = ANY($1)`, doneIDs); err != nil {
		slog.Error("[upload-cleanup] tandai ledger deleted gagal", "err", err)
		return len(items)
	}
	// Konsistensi UI/AI: dokumen → tandai berkas fisik hilang (teks tetap);
	// gambar → soft-delete barisnya (berkas = asetnya).
	if len(docKeys) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE profiler_documents SET file_deleted_at = now()
			WHERE storage_key = ANY($1) AND file_deleted_at IS NULL`, docKeys); err != nil {
			slog.Error("[upload-cleanup] tandai file_deleted_at gagal", "err", err)
		}
	}
	if len(imgKeys) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE profiler_images SET deleted_at = now()
			WHERE storage_key = ANY($1) AND deleted_at IS NULL`, imgKeys); err != nil {
			slog.Error("[upload-cleanup] soft-delete gambar gagal", "err", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Error("[upload-cleanup] commit batch gagal", "err", err)
		// Fisik sudah terhapus tetapi metadata belum — siklus berikutnya
		// akan menemukan baris masih delete_pending dan os.Remove kini
		// ENOENT (dihitung sukses), lalu metadata tertutup.
	}
	return len(items)
}

// uploadRefRe: tautan /uploads/... di konten HTML (pola sama dengan sapuan
// brandflow schPurgeStaleUploads).
var uploadRefRe = regexp.MustCompile(`/uploads/([A-Za-z0-9._-]+)`)

// sweepOrphans: berkas di UPLOAD_DIR yang tidak direferensikan metadata mana
// pun setelah grace period. Referensi diperiksa lintas domain (ledger profiler,
// dokumen/gambar profiler, berkas kampanye, featured image, HTML content
// versions) supaya berkas milik domain lain tidak salah hapus.
func (d *Deps) sweepOrphans(ctx context.Context, lim UploadLimits, st *UploadCleanupStats) {
	dir := uploadDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("[upload-cleanup] baca direktori upload gagal", "err", err)
		return
	}
	used := map[string]struct{}{}
	refs := []struct {
		sql   string
		field string
	}{
		{`SELECT storage_key FROM chat_attachments`, "storage_key"},
		{`SELECT storage_key FROM profiler_documents WHERE storage_key IS NOT NULL`, "storage_key"},
		{`SELECT storage_key FROM profiler_images WHERE storage_key IS NOT NULL`, "storage_key"},
		{`SELECT storage_key FROM taut_campaign_files WHERE storage_key IS NOT NULL`, "storage_key"},
		{`SELECT featured_image_url FROM taut_campaign_media WHERE featured_image_url IS NOT NULL AND featured_image_url <> ''`, "featured_image_url"},
	}
	for _, ref := range refs {
		rows, err := d.Pool.Query(ctx, ref.sql)
		if err != nil {
			// Gagal memeriksa satu sumber referensi → batal hapus yatim
			// siklus ini (lebih aman tidak menghapus daripada salah hapus).
			slog.Error("[upload-cleanup] scan referensi yatim gagal", "sumber", ref.field, "err", err)
			st.OrphansFound, st.OrphansDeleted = 0, 0
			return
		}
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err == nil {
				used[filepath.Base(strings.SplitN(v, "?", 2)[0])] = struct{}{}
			}
		}
		rows.Close()
	}
	contentRows, err := d.Pool.Query(ctx, `SELECT content FROM content_versions WHERE content LIKE '%/uploads/%'`)
	if err != nil {
		slog.Error("[upload-cleanup] scan HTML yatim gagal", "err", err)
		return
	}
	for contentRows.Next() {
		var content string
		if err := contentRows.Scan(&content); err == nil {
			for _, m := range uploadRefRe.FindAllStringSubmatch(content, -1) {
				used[m[1]] = struct{}{}
			}
		}
	}
	contentRows.Close()

	cutoff := time.Now().Add(-lim.OrphanGrace)
	allowDelete := strings.EqualFold(strings.TrimSpace(os.Getenv("UPLOAD_ORPHAN_CLEANUP")), "true")
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := used[name]; ok {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.ModTime().After(cutoff) {
			continue
		}
		st.OrphansFound++
		// Hanya berkas bernama pola internal (uuid.ext) yang boleh dihapus
		// otomatis; sisanya (nama domain lain) dilaporkan saja.
		if !allowDelete || !looksLikeManagedUpload(name) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err == nil {
			st.OrphansDeleted++
		} else {
			slog.Error("[upload-cleanup] hapus yatim gagal", "name", name, "err", err)
		}
	}
}

// collectStorageStats: total aktif ledger + kapasitas partisi UPLOAD_DIR.
func (d *Deps) collectStorageStats(ctx context.Context, st *UploadCleanupStats) {
	if err := d.Pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM chat_attachments WHERE status <> 'deleted'`).
		Scan(&st.ActiveFiles, &st.ActiveBytes); err != nil {
		slog.Error("[upload-cleanup] statistik ledger gagal", "err", err)
	}
	var fsStat syscall.Statfs_t
	if err := syscall.Statfs(uploadDir(), &fsStat); err == nil {
		st.DiskTotalBytes = fsStat.Blocks * uint64(fsStat.Bsize)
		st.DiskFreeBytes = fsStat.Bavail * uint64(fsStat.Bsize)
		if st.DiskTotalBytes > 0 {
			st.DiskUsedPct = float64(st.DiskTotalBytes-st.DiskFreeBytes) / float64(st.DiskTotalBytes) * 100
		}
	} else {
		slog.Error("[upload-cleanup] statistik disk gagal", "err", err)
	}
}
