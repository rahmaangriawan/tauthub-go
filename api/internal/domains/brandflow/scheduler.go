package brandflow

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// scheduler.go: port lib/scheduler/publish-scheduler.ts (runScheduler) + helper
// sweep di sekitarnya, plus lib/publication/plans.ts materializeDueOccurrences,
// lib/media/intelligence.ts refreshStaleMediaHealth/refreshMediaHealth,
// lib/media/ai-summary.ts sweepMediaAiSummaries, dan lib/uploads.ts
// purgeStaleUploads.
//
// Satu siklus scheduler independen browser: materialisasi jadwal berulang,
// safety sweep publikasi due, pengingat mitra, notifikasi telat setuju, drain
// antrean job, safety net antrean email/WA, refresh kesehatan media, sweep
// enqueue berkala, lalu purge upload basi. Setiap langkah best-effort —
// kegagalan satu sweep tidak boleh menghentikan siklus (parity try/catch TS).

// SchedulerResult: hitungan per langkah (parity bentuk respons runScheduler).
type SchedulerResult struct {
	PlanOccurrencesMaterialized int `json:"plan_occurrences_materialized"`
	PlanOccurrencesFailed       int `json:"plan_occurrences_failed"`
	PublicationsTriggered       int `json:"publications_triggered"`
	PartnerRemindersSent        int `json:"partner_reminders_sent"`
	LateApprovalNotified        int `json:"late_approval_notified"`
	MediaHealthRefreshed        int `json:"media_health_refreshed"`
	UploadsPurged               int `json:"uploads_purged"`
	JobsProcessed               int `json:"jobs_processed"`
	RetentionDeleted            int64 `json:"retention_deleted"`
	RetentionArchived           int64 `json:"retention_archived"`
}

// toMap: bentuk respons JSON (parity nama field snake_case app lama).
func (r SchedulerResult) toMap() map[string]any {
	return map[string]any{
		"plan_occurrences_materialized": r.PlanOccurrencesMaterialized,
		"plan_occurrences_failed":       r.PlanOccurrencesFailed,
		"publications_triggered":        r.PublicationsTriggered,
		"partner_reminders_sent":        r.PartnerRemindersSent,
		"late_approval_notified":        r.LateApprovalNotified,
		"media_health_refreshed":        r.MediaHealthRefreshed,
		"uploads_purged":                r.UploadsPurged,
		"jobs_processed":                r.JobsProcessed,
		"retention_deleted":             r.RetentionDeleted,
		"retention_archived":            r.RetentionArchived,
	}
}

// RunSchedulerResult: RunScheduler dengan keluaran map — dipakai hook cron
// platform (/v1/internal/cron/scheduler) yang tidak boleh mengimpor paket ini.
func (d *Deps) RunSchedulerResult(ctx context.Context, maxJobs int, runner *jobs.Runner) map[string]any {
	return d.RunScheduler(ctx, maxJobs, runner).toMap()
}

// RunScheduler: satu siklus scheduler lengkap (port runScheduler).
func (d *Deps) RunScheduler(ctx context.Context, maxJobs int, runner *jobs.Runner) SchedulerResult {
	// Parity: Math.min(Math.max(maxJobs ?? 20, 1), 100).
	if maxJobs < 1 {
		maxJobs = 1
	}
	if maxJobs > 100 {
		maxJobs = 100
	}
	var res SchedulerResult

	// 1. Jadwal berulang: materialisasi occurrence due → publikasi nyata +
	//    reservasi TC saat jatuh tempo. Best-effort.
	if materialized, failed, err := d.schMaterializeDueOccurrences(ctx, schMinInt(maxJobs, 10)); err != nil {
		slog.Error("[scheduler] materialisasi jadwal gagal", "err", err)
	} else {
		res.PlanOccurrencesMaterialized = materialized
		res.PlanOccurrencesFailed = failed
	}

	// 2. Safety sweep publikasi terjadwal yang belum punya job hidup.
	res.PublicationsTriggered = d.schEnqueueDuePublications(ctx, maxJobs)

	// 3. Pengingat mitra mendekati batas approve (window 2 jam).
	res.PartnerRemindersSent = d.schSweepPartnerDeadlines(ctx, schMinInt(maxJobs, 20), false)

	// 4. Notifikasi telat menyetujui (jadwal terbit sudah lewat).
	res.LateApprovalNotified = d.schSweepPartnerDeadlines(ctx, schMinInt(maxJobs, 20), true)

	// 5. Bersihkan job yang tertinggal `running` (proses pemiliknya berhenti saat
	//    deploy/restart), lalu drain antrean.
	if runner != nil {
		if n, err := runner.ReapStuck(ctx, 10*time.Minute); err != nil {
			slog.Error("[scheduler] bersihkan job macet gagal", "err", err)
		} else if n > 0 {
			slog.Info("[scheduler] job macet dikembalikan ke antrean", "jumlah", n)
		}
		if n, err := runner.Drain(ctx, maxJobs, jobs.WorkerID("scheduler")); err != nil {
			slog.Error("[scheduler] drain gagal", "err", err)
		} else {
			res.JobsProcessed = n
		}
	}

	// 5b. Baris run Taut tertinggal `running` karena proses pemiliknya berhenti di
	//     tengah generasi (deploy/restart) ditandai gagal, supaya riwayat run dan
	//     statistik admin tidak selamanya menghitungnya sebagai "aktif".
	if n, err := d.reapStuckTautRuns(ctx, 20*time.Minute); err != nil {
		slog.Error("[scheduler] bersihkan run Taut macet gagal", "err", err)
	} else if n > 0 {
		slog.Info("[scheduler] run Taut macet ditandai gagal", "jumlah", n)
	}

	// 6. Safety net antrean email & WA. App lama memanggil processEmailQueue(10) /
	//    processWaQueue(10) langsung dari scheduler. Port Go-nya hidup sebagai
	//    processor job kind `send_email` / `send_wa` di paket platform
	//    (job_messaging.go — ditulis agent lain, tidak boleh dipanggil dari sini).
	//    DEVIASI yang disengaja: kami tidak memanggil fungsi lintas-paket,
	//    melainkan mendeteksi apakah antrean punya baris due lalu meng-enqueue
	//    job `send_email` / `send_wa` agar worker yang sama mengurasnya. Intent
	//    safety net tetap terjaga tanpa kopling paket.
	d.schEnqueueQueueDrain(ctx, "send_email", "email_deliveries")
	d.schEnqueueWADrain(ctx)

	// 7. Refresh kesehatan media basi (light by design: 3 per siklus).
	res.MediaHealthRefreshed = d.schRefreshStaleMediaHealth(ctx, 3)

	// 8. Sweep enqueue berkala — semua best-effort, urutan mengikuti legacy.
	d.schSweepMediaAiSummaries(ctx, 3)
	d.schEnqueueDomainRechecks(ctx, 20)
	d.schEnqueueCrawlTargetSyncs(ctx, 20)
	d.schEnqueuePublicationLinkChecks(ctx, 20)
	d.schEnqueueMediaAiRefreshes(ctx, 20)

	// 9. Purge upload basi (>30 hari & tak direferensikan DB).
	res.UploadsPurged = d.schPurgeStaleUploads(ctx)

	// 10. Retensi data: pangkas log/cache/event yang melebihi ambang supaya
	//     database tidak membengkak. Maksimal sekali sehari (klaim harian).
	res.RetentionDeleted, res.RetentionArchived = d.schRetention(ctx)

	return res
}

// reapStuckTautRuns: tandai baris run Taut yang tertinggal `running` sebagai
// gagal. Terjadi kalau proses pemiliknya berhenti di tengah generasi (deploy
// atau restart), sehingga barisnya tidak pernah punya status akhir dan membuat
// riwayat run serta statistik admin berbohong. Ambang 20 menit dipilih agar run
// yang masih sah tidak tersentuh: timeout LLM 5 menit dikali tiga percobaan
// plus tunggu slot rate limit masih di bawah itu.
func (d *Deps) reapStuckTautRuns(ctx context.Context, staleAfter time.Duration) (int, error) {
	if staleAfter <= 0 {
		staleAfter = 20 * time.Minute
	}
	affected, err := d.execRows(ctx, `
		UPDATE taut_agent_runs
		SET status = 'failed',
		    error = COALESCE(error, 'Proses berhenti saat generasi berjalan (dibersihkan otomatis).'),
		    finished_at = NOW()
		WHERE status = 'running'
		  AND finished_at IS NULL
		  AND created_at < NOW() - $1::interval`,
		strconv.Itoa(int(staleAfter.Seconds()))+" seconds")
	return int(affected), err
}

// ---------- 10. retensi data (brandflow/retention.go) ----------

// schRetention: pangkas log/cache/event melebihi ambang retensi — MAKSIMAL
// SEKALI SEHARI. Klaim hari ini lewat tabel retention_runs (INSERT ON CONFLICT
// DO NOTHING): proses pertama yang berhasil klaim yang menjalankan, proses lain
// (API dan scheduler bisa hidup berdampingan) melewati hari itu. Baris klaim
// juga menjadi jejak audit kapan retensi terakhir dijalankan + statistiknya.
func (d *Deps) schRetention(ctx context.Context) (deleted, archived int64) {
	tag, err := d.Pool.Exec(ctx,
		`INSERT INTO retention_runs (day, stats) VALUES (current_date, '{}'::jsonb) ON CONFLICT (day) DO NOTHING`)
	if err != nil {
		slog.Error("[scheduler] klaim hari retensi gagal", "err", err)
		return 0, 0
	}
	if tag.RowsAffected() == 0 {
		return 0, 0 // sudah dijalankan hari ini oleh proses lain
	}
	rep, err := RunRetention(ctx, d.Pool, DefaultRetentionConfig())
	if err != nil {
		// Lepaskan klaim supaya siklus berikutnya mencoba lagi — jangan sampai
		// satu hari penuh tanpa retensi hanya karena satu query gagal.
		if _, delErr := d.Pool.Exec(ctx, `DELETE FROM retention_runs WHERE day = current_date`); delErr != nil {
			slog.Error("[scheduler] lepas klaim retensi gagal", "err", delErr)
		}
		slog.Error("[scheduler] retensi data gagal", "err", err)
		return 0, 0
	}
	if _, err := d.Pool.Exec(ctx,
		`UPDATE retention_runs SET stats = $1::jsonb WHERE day = current_date`,
		jsonStringify(rep)); err != nil {
		slog.Warn("[scheduler] simpan statistik retensi gagal", "err", err)
	}
	slog.Info("[scheduler] retensi data selesai",
		"dihapus", rep.Deleted, "diarsipkan", rep.Archived, "ms", rep.DurationMS)
	return rep.Deleted, rep.Archived
}

// RunSchedulerLoop: polling periodik sampai ctx dibatalkan (cmd/scheduler).
func (d *Deps) RunSchedulerLoop(ctx context.Context, interval time.Duration, maxJobs int, runner *jobs.Runner) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			res := d.RunScheduler(ctx, maxJobs, runner)
			slog.Info("[scheduler] siklus selesai",
				"ms", time.Since(start).Milliseconds(),
				"publikasi", res.PublicationsTriggered,
				"job", res.JobsProcessed,
				"occurrence", res.PlanOccurrencesMaterialized,
				"pengingat", res.PartnerRemindersSent,
				"telat", res.LateApprovalNotified)
		}
	}
}

// ---------- 1. materialisasi jadwal berulang (plans.ts) ----------

// schMaterializeDueOccurrences: port materializeDueOccurrences — claim occurrence
// due dengan FOR UPDATE SKIP LOCKED, buat publikasi lewat createPublicationRecord,
// reservasi TC saat jatuh tempo, lalu tandai materialized/failed.
func (d *Deps) schMaterializeDueOccurrences(ctx context.Context, limit int) (materialized, failed int, err error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}

	// Catatan parity: seperti app lama (rows() = query autocommit), lock baris
	// `FOR UPDATE SKIP LOCKED` dilepas di akhir statement SELECT — jadi dua
	// scheduler yang overlap bisa sama-sama memilih occurrence yang sama. Yang
	// mencegah duplikat adalah guard di createPublicationRecord (document_id +
	// media_id unik); percobaan kedua gagal Conflict lalu occurrence ditandai
	// failed. Perilaku ini identik dengan legacy dan sengaja tidak diubah agar
	// kontraknya sama.
	due, err := d.queryRows(ctx, `
		SELECT o.id, o.plan_id, o.media_id, o.scheduled_at,
		       p.brand_id, p.document_id, p.created_by
		FROM publication_plan_occurrences o
		JOIN publication_plans p ON p.id = o.plan_id
		WHERE o.status = 'scheduled'
		  AND o.scheduled_at <= NOW()
		  AND p.status = 'active'
		ORDER BY o.scheduled_at ASC
		LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return 0, 0, err
	}

	for _, occ := range due {
		occID := occ.strOr("id", "")
		mediaID := occ.strOr("media_id", "")
		brandID := occ.strOr("brand_id", "")
		documentID := occ.strOr("document_id", "")
		userID := occ.strOr("created_by", "")
		if userID == "" {
			userID = brandID
		}
		scheduledAt := schRowTime(occ, "scheduled_at")

		// 1) Buat publikasi (check standar + gate mitra/antrean ikut).
		created, cErr := d.createPublicationRecord(ctx, createPublicationInput{
			BrandID:           brandID,
			UserID:            userID,
			DocumentID:        documentID,
			MediaID:           mediaID,
			PublishStatus:     "publish",
			ScheduledAt:       scheduledAt,
			SkipStandardCheck: false,
		})
		if cErr != nil {
			d.schMarkOccurrenceFailed(ctx, occID, cErr.Error(), brandID)
			failed++
			continue
		}
		publicationID := created.Row.strOr("id", "")

		// 2) Reservasi TC saat jatuh tempo — saldo dicek SEKARANG, bukan saat
		//    plan dibuat, sehingga plan berbulan-bulan tidak mengunci wallet.
		//    Parity plans.ts: hanya brand yang punya dompet TC yang direservasi.
		if wallet := d.getWallet(ctx, brandID); wallet != nil {
			cost := d.creditCostForMedia(ctx, mediaID)
			if cost > 0 {
				if _, rErr := d.reserveCredit(ctx, brandID, publicationID, cost, userID); rErr != nil {
					var ice *insufficientCreditsError
					if errors.As(rErr, &ice) {
						_ = d.exec(ctx, "DELETE FROM publications WHERE id = $1", publicationID)
						d.schMarkOccurrenceFailed(ctx, occID,
							"Saldo Tulisio Credit tidak cukup: butuh "+itoa(cost)+" TC, tersedia "+
								itoa(ice.Available)+" TC. Top Up agar jadwal bisa berjalan.", brandID)
						failed++
						continue
					}
					// Bukan saldo kurang: publication DIBIARKAN (parity outer catch).
					d.schMarkOccurrenceFailed(ctx, occID, rErr.Error(), brandID)
					failed++
					continue
				}
			}
		}

		if uErr := d.exec(ctx,
			"UPDATE publication_plan_occurrences SET status = 'materialized', publication_id = $1, updated_at = NOW() WHERE id = $2",
			publicationID, occID); uErr != nil {
			d.schMarkOccurrenceFailed(ctx, occID, uErr.Error(), brandID)
			failed++
			continue
		}
		materialized++
	}
	return materialized, failed, nil
}

// schMarkOccurrenceFailed: tandai occurrence gagal + notifikasi brand members
// (parity markOccurrenceFailed — pesan dipotong 1000, notifikasi best-effort).
func (d *Deps) schMarkOccurrenceFailed(ctx context.Context, occurrenceID, message, brandID string) {
	msg := truncateString(message, 1000)
	if err := d.exec(ctx,
		"UPDATE publication_plan_occurrences SET status = 'failed', error_message = $1, updated_at = NOW() WHERE id = $2",
		msg, occurrenceID); err != nil {
		slog.Warn("[scheduler] tandai occurrence failed gagal", "id", occurrenceID, "err", err)
	}
	link := "/publications"
	d.notifyBrandMembers(ctx, brandID, "publication.failed",
		"Jadwal terjadwal gagal diproses",
		"Satu hari dari jadwal berulang gagal didistribusikan: "+truncateString(msg, 200)+". Hari lain tetap berjalan.",
		&link)
}

// ---------- 2. safety sweep publikasi due ----------

// schEnqueueDuePublications: publikasi dengan scheduled_at <= NOW() dan status
// non-terminal yang belum punya job hidup → enqueue publication_execution
// (maxAttempts 3), dedupe lewat payload->>'publicationId'.
func (d *Deps) schEnqueueDuePublications(ctx context.Context, limit int) int {
	due, err := d.queryRows(ctx, `
		SELECT p.id, p.brand_id
		FROM publications p
		WHERE p.scheduled_at IS NOT NULL
		  AND p.scheduled_at <= NOW()
		  AND p.status IN ('pending','waiting_partner','partner_approved','retrying')
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'publication_execution'
		      AND j.status NOT IN ('completed','failed')
		      AND j.payload->>'publicationId' = p.id
		  )
		ORDER BY p.scheduled_at
		LIMIT $1`, limit)
	if err != nil {
		slog.Error("[scheduler] sweep publikasi due gagal", "err", err)
		return 0
	}
	enqueued := 0
	for _, row := range due {
		id := row.strOr("id", "")
		if _, err := d.enqueueJobAt(ctx, "publication_execution", map[string]any{
			"publicationId": id, "brandId": row.strOr("brand_id", ""),
		}, nil, 3); err != nil {
			slog.Warn("[scheduler] enqueue publikasi gagal", "id", id, "err", err)
			continue
		}
		enqueued++
	}
	return enqueued
}

// ---------- 3 & 4. pengingat mitra & notifikasi telat ----------

// schSweepPartnerDeadlines: publikasi waiting_partner yang mendekati batas
// (late=false, window 2 jam) atau sudah lewat (late=true) → notifikasi in-app
// (idempoten via dedupe_key) + email follow-up.
//
// Catatan parity: app lama memakai notifyPartnerReminder untuk DUA jalur dan
// fungsi itu mengunci type 'publication.reminder' apa pun jalurnya — termasuk
// jalur telat. Nilai itu yang kita pertahankan supaya tidak menyimpang dari
// perilaku produksi.
func (d *Deps) schSweepPartnerDeadlines(ctx context.Context, limit int, late bool) int {
	if limit < 1 {
		return 0
	}
	var (
		scheduledCond string
		dedupePrefix  string
		emailEvent    string
		title         string
	)
	if late {
		scheduledCond = "p.scheduled_at < NOW()"
		dedupePrefix = "late_approval:"
		emailEvent = "publication.late_approval"
		title = "Terlambat menyetujui publikasi"
	} else {
		scheduledCond = "p.scheduled_at <= (NOW() + INTERVAL '2 hours')"
		dedupePrefix = "waiting_partner_reminder:"
		emailEvent = "publication.reminder"
		title = "Publikasi membutuhkan persetujuan"
	}

	rows, err := d.queryRows(ctx, `
		SELECT p.id, p.scheduled_at, m.name AS media_name, d.title AS document_title,
		       prt.id AS partner_id, prt.public_id AS partner_public_id, prt.user_id AS partner_user_id,
		       (SELECT u.email FROM users u WHERE u.id = prt.user_id LIMIT 1) AS partner_email
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN content_documents d ON d.id = p.document_id
		JOIN partners prt ON prt.id = m.partner_id
		WHERE p.status = 'waiting_partner'
		  AND p.scheduled_at IS NOT NULL
		  AND `+scheduledCond+`
		  AND NOT EXISTS (
		    SELECT 1 FROM notifications n
		    WHERE n.publication_id = p.id
		      AND n.dedupe_key = $1 || p.id
		  )
		ORDER BY p.scheduled_at
		LIMIT $2`, dedupePrefix, limit)
	if err != nil {
		slog.Error("[scheduler] sweep pengingat mitra gagal", "late", late, "err", err)
		return 0
	}

	sent := 0
	for _, pub := range rows {
		publicationID := pub.strOr("id", "")
		// user_id mitra sudah ikut di SELECT (sebelumnya satu query per publikasi).
		recipientUserID := pub.strOr("partner_user_id", "")
		if recipientUserID == "" {
			continue
		}
		documentTitle := pub.strOr("document_title", "")
		mediaName := pub.strOr("media_name", "")
		if mediaName == "" {
			mediaName = "media"
		}
		partnerPublicID := pub.strOr("partner_public_id", "")
		link := "/mitra/publications"
		if partnerPublicID != "" {
			link = "/mitra/" + partnerPublicID + "/publications?pub=" + publicationID
		}

		body := "Artikel \"" + documentTitle + "\" untuk " + mediaName +
			" belum disetujui dan mendekati batas waktu publikasi."
		if late {
			scheduledText := ""
			if t := schRowTime(pub, "scheduled_at"); t != nil {
				// Parity toLocaleString("id-ID", {dateStyle:"medium", timeStyle:"short"}).
				scheduledText = "pada " + t.Local().Format("2 Jan 2006, 15.04")
			}
			body = "Artikel \"" + documentTitle + "\" untuk " + mediaName +
				" seharusnya terbit " + scheduledText + ", tetapi belum Anda setujui. Segera tinjau agar tidak telat."
		}

		// notifyPartnerReminder: cek duplikat eksplisit (user_id + dedupe_key +
		// publication_id), bukan ON CONFLICT.
		dedupeKey := dedupePrefix + publicationID
		dup, dErr := d.queryRow(ctx,
			"SELECT id FROM notifications WHERE user_id = $1 AND dedupe_key = $2 AND publication_id = $3 LIMIT 1",
			recipientUserID, dedupeKey, publicationID)
		if dErr != nil || dup != nil {
			continue
		}
		if err := d.exec(ctx, `
			INSERT INTO notifications (id, user_id, type, title, body, link, publication_id, dedupe_key)
			VALUES (gen_random_uuid(), $1, 'publication.reminder', $2, $3, $4, $5, $6)`,
			recipientUserID, title, body, link, publicationID, dedupeKey); err != nil {
			slog.Warn("[scheduler] notifikasi mitra gagal", "pub", publicationID, "err", err)
			continue
		}
		sent++

		// Email follow-up (best-effort; kegagalan email tidak membatalkan notifikasi).
		emailAddr := pub.strOr("partner_email", "")
		if emailAddr != "" {
			dedupeEmail := emailEvent + ":" + publicationID
			d.schEmailFollowUp(ctx, emailEvent, recipientUserID, emailAddr, publicationID, dedupeEmail, documentTitle, mediaName, link)
		}
	}
	return sent
}

// schEmailFollowUp: port routeEmailNotification untuk event pengingat mitra —
// gerbang matriks+preferensi, render template, enqueue email_deliveries, lalu
// pastikan ada job send_email (parity sendTransactionalEmail).
func (d *Deps) schEmailFollowUp(ctx context.Context, event, recipientUserID, recipientEmail, referenceID, dedupeKey, topic, mediaName, link string) {
	if recipientEmail == "" || !d.schRouteEmailEnabled(ctx, event, recipientUserID) {
		return
	}
	var subject, html string
	if event == "publication.late_approval" {
		subject, html = d.schRenderDetailEmail(ctx, "Publikasi melewati batas waktu",
			"Artikel ini melewati batas waktu persetujuan dan otomatis dianggap ditolak.",
			[][2]string{{"Topik", schOrDash(topic)}, {"Media", schOrDash(mediaName)}},
			link, "Lihat Detail")
	} else {
		subject, html = d.schRenderLinkEmail(ctx, "Pengingat publikasi",
			"Ada artikel yang menunggu persetujuan Anda lebih dari batas waktu yang ditentukan.",
			link, "Tinjau Sekarang")
	}

	id, status, err := email.EnqueueEmail(ctx, d.Pool, email.EnqueueOptions{
		Template: event, RecipientEmail: recipientEmail,
		UserID: ptrOrNil(recipientUserID), ReferenceID: ptrOrNil(referenceID),
		Event: event, Subject: subject, HTML: html, DedupeKey: ptrOrNil(dedupeKey),
	})
	if err != nil {
		slog.Warn("[scheduler] enqueue email mitra gagal", "event", event, "pub", referenceID, "err", err)
		return
	}
	if status == "queued" && id != "" {
		d.schEnqueueQueueDrain(ctx, "send_email", "")
	}
}

// schRouteEmailEnabled: parity routeEmailDecision — event di matriks dan
// preferensi user (kategori partner → email_partner) tidak mematikannya.
//
// Catatan: app lama meneruskan `recipientUserId: pub.partner_id` (id partner,
// bukan id user) sehingga query notification_preferences user_id = <partner_id>
// selalu kosong dan gerbang preferensi praktis terlewat. Kita meneruskan id user
// yang benar supaya preferensi email_partner benar-benar dihormati.
func (d *Deps) schRouteEmailEnabled(ctx context.Context, event, userID string) bool {
	category := ""
	switch event {
	case "publication.reminder", "publication.late_approval":
		category = "partner"
	}
	if category == "" {
		return false
	}
	if userID == "" {
		return true
	}
	pref, err := d.queryRow(ctx,
		"SELECT email_partner FROM notification_preferences WHERE user_id = $1 LIMIT 1", userID)
	if err != nil || pref == nil {
		return true
	}
	return pref.boolOr("email_partner", true)
}

// ---------- 6. safety net antrean email/WA ----------

// schEnqueueQueueDrain: bila `table` punya baris due dan belum ada job kind ini
// yang menunggu, enqueue satu job drain. table kosong → enqueue tanpa cek
// (pemanggil sudah tahu ada baris baru). Parity intent processEmailQueue(10).
func (d *Deps) schEnqueueQueueDrain(ctx context.Context, kind, table string) {
	if table != "" {
		row, err := d.queryRow(ctx, `
			SELECT COUNT(*)::int AS pending FROM `+table+`
			WHERE status IN ('queued','failed') AND next_attempt_at <= NOW() AND attempts < max_attempts`)
		if err != nil || row == nil || row.intOr("pending", 0) == 0 {
			return
		}
	}
	existing, err := d.queryRow(ctx,
		"SELECT 1 AS ada FROM jobs WHERE kind = $1 AND status IN ('pending','running') LIMIT 1", kind)
	if err == nil && existing != nil {
		return
	}
	if _, err := d.enqueueJobAt(ctx, kind, map[string]any{"trigger": "scheduler.safety_net"}, nil, 3); err != nil {
		slog.Warn("[scheduler] enqueue drain gagal", "kind", kind, "err", err)
	}
}

// schEnqueueWADrain: safety net processWaQueue(10) — wa_deliveries memakai
// predikat due sendiri (tanpa next_attempt_at; hanya status 'queued').
func (d *Deps) schEnqueueWADrain(ctx context.Context) {
	row, err := d.queryRow(ctx,
		"SELECT COUNT(*)::int AS pending FROM wa_deliveries WHERE status = 'queued' AND attempts < 3")
	if err != nil || row == nil || row.intOr("pending", 0) == 0 {
		return
	}
	existing, err := d.queryRow(ctx,
		"SELECT 1 AS ada FROM jobs WHERE kind = 'send_wa' AND status IN ('pending','running') LIMIT 1")
	if err == nil && existing != nil {
		return
	}
	if _, err := d.enqueueJobAt(ctx, "send_wa", map[string]any{"trigger": "scheduler.safety_net"}, nil, 3); err != nil {
		slog.Warn("[scheduler] enqueue drain WA gagal", "err", err)
	}
}

// ---------- 7. kesehatan media (intelligence.ts) ----------

// schRefreshStaleMediaHealth: port refreshStaleMediaHealth — hanya `limit` media
// paling basi per siklus (light by design). maxAgeHours dari env
// MEDIA_HEALTH_REFRESH_HOURS (default 336 = 14 hari, parity serverConfig).
func (d *Deps) schRefreshStaleMediaHealth(ctx context.Context, limit int) int {
	if limit < 1 {
		return 0
	}
	if limit > 50 {
		limit = 50
	}
	maxAgeHours := schEnvInt("MEDIA_HEALTH_REFRESH_HOURS", 336)
	if maxAgeHours < 1 {
		maxAgeHours = 1
	}
	stale, err := d.queryRows(ctx, `
		SELECT id FROM media
		WHERE status IN ('active','draft')
		  AND (health_computed_at IS NULL OR health_computed_at < NOW() - make_interval(hours => $1::int))
		ORDER BY health_computed_at ASC NULLS FIRST, id ASC
		LIMIT $2`, maxAgeHours, limit)
	if err != nil {
		slog.Error("[scheduler] sweep kesehatan media gagal", "err", err)
		return 0
	}
	refreshed := 0
	for _, row := range stale {
		mediaID := row.strOr("id", "")
		if err := d.schRefreshMediaHealth(ctx, mediaID); err != nil {
			slog.Warn("[scheduler] refresh kesehatan media gagal", "id", mediaID, "err", err)
			continue
		}
		refreshed++
	}
	return refreshed
}

// schRefreshMediaHealth: port refreshMediaHealth — hitung + simpan skor.
func (d *Deps) schRefreshMediaHealth(ctx context.Context, mediaID string) error {
	score, breakdown, err := d.schComputeMediaHealth(ctx, mediaID)
	if err != nil {
		return err
	}
	return d.exec(ctx,
		"UPDATE media SET health_score = $1, health_breakdown = $2::jsonb, health_computed_at = NOW(), updated_at = NOW() WHERE id = $3",
		score, marshalJSON(breakdown), mediaID)
}

// schComputeMediaHealth: port computeMediaHealth + scoreMediaHealth — skor 0..100
// dari riwayat operasional (availability 15, success rate 30, publish time 35,
// rejection rate 20) yang dicampur kualitas editorial 85:15 saat ada riwayat.
func (d *Deps) schComputeMediaHealth(ctx context.Context, mediaID string) (int, map[string]any, error) {
	media, err := d.queryRow(ctx,
		"SELECT id, status, traffic_estimate, media_quality_score, media_tier FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		return 0, nil, err
	}
	if media == nil {
		return 0, nil, errNotFound("Media not found")
	}

	stats, err := d.queryRow(ctx, `
		SELECT
		  COUNT(*)::int AS total,
		  COUNT(*) FILTER (WHERE status = 'published')::int AS published,
		  COUNT(*) FILTER (WHERE status = 'partner_rejected')::int AS rejected,
		  COUNT(*) FILTER (WHERE status = 'failed')::int AS failed,
		  ROUND(AVG(EXTRACT(EPOCH FROM (published_at - created_at)) / 3600.0), 2)::double precision AS avg_publish_hours
		FROM publications
		WHERE media_id = $1 AND status <> 'cancelled'`, mediaID)
	if err != nil {
		return 0, nil, err
	}
	total, published, rejected, failed := 0, 0, 0, 0
	var avgPublishHours *float64
	if stats != nil {
		total = stats.intOr("total", 0)
		published = stats.intOr("published", 0)
		rejected = stats.intOr("rejected", 0)
		failed = stats.intOr("failed", 0)
		if stats.has("avg_publish_hours") {
			v := stats.numOr("avg_publish_hours")
			avgPublishHours = &v
		}
	}

	status := media.strOr("status", "")
	availability := 0.0
	switch status {
	case "active":
		availability = 15
	case "draft":
		availability = 8
	}

	successRate := 10.0
	if total > 0 {
		successRate = float64(published) / float64(total) * 30
	}

	var publishTime float64
	switch {
	case avgPublishHours == nil:
		publishTime = 21
	case *avgPublishHours <= 6:
		publishTime = 35
	case *avgPublishHours <= 24:
		publishTime = 28
	case *avgPublishHours <= 72:
		publishTime = 18
	default:
		publishTime = 9
	}

	rejectionRate := 12.0
	if total > 0 {
		rejectionRate = math.Max(0, 20-(float64(rejected)/float64(total))*20)
	}

	operationalTotal := math.Max(0, math.Min(100, math.Round(availability+successRate+publishTime+rejectionRate)))

	tierDefaults := d.schTierDefaultQuality(ctx)
	var quality *float64
	qualitySource := any(nil)
	if media.has("media_quality_score") {
		v := math.Max(0, math.Min(100, media.numOr("media_quality_score")))
		quality = &v
		qualitySource = "media"
	} else if tier, ok := media.str("media_tier"); ok && tier != "" {
		if def, exists := tierDefaults[tier]; exists {
			v := math.Max(0, math.Min(100, def))
			quality = &v
			qualitySource = "tier_default"
		}
	}

	totalScore := 0.0
	if total > 0 {
		if quality != nil {
			totalScore = math.Round(operationalTotal*0.85 + *quality*0.15)
		} else {
			totalScore = operationalTotal
		}
	}

	noHistory := total == 0
	breakdown := map[string]any{
		"availability":      schZeroIf(noHistory, availability),
		"success_rate":      schRound2If(noHistory, successRate),
		"publish_time":      schZeroIf(noHistory, publishTime),
		"rejection_rate":    schRound2If(noHistory, rejectionRate),
		"operational_total": schZeroIf(noHistory, operationalTotal),
		"quality":           schFloatArg(quality),
		"quality_source":    qualitySource,
		"reach":             nil,
		"total":             int(totalScore),
		"published":         published,
		"rejected":          rejected,
		"failed":            failed,
		"avg_publish_hours": schFloatArg(avgPublishHours),
	}
	return int(totalScore), breakdown, nil
}

// schTierDefaultQuality: pr_media_rules.tier_default_quality (default tier1 90 /
// tier2 80 / tier3 70), parity tierDefaultQuality.
func (d *Deps) schTierDefaultQuality(ctx context.Context) map[string]float64 {
	fallback := map[string]float64{"tier1": 90, "tier2": 80, "tier3": 70}
	row, err := d.queryRow(ctx, "SELECT tier_default_quality FROM pr_media_rules WHERE id = 'default' LIMIT 1")
	if err != nil || row == nil {
		return fallback
	}
	parsed, ok := row.jsonField("tier_default_quality").(map[string]any)
	if !ok || len(parsed) == 0 {
		return fallback
	}
	out := map[string]float64{}
	for k, v := range parsed {
		if f, ok := v.(float64); ok {
			out[k] = f
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

// ---------- 8a. sweep domain mitra (enqueueDomainRechecks) ----------

// schEnqueueDomainRechecks: enqueue domain_verify untuk SEMUA media mitra yang
// punya website dan terakhir dicek > 7 hari lalu (atau belum pernah).
func (d *Deps) schEnqueueDomainRechecks(ctx context.Context, limit int) {
	stale, err := d.queryRows(ctx, `
		SELECT id FROM media
		WHERE partner_id IS NOT NULL
		  AND website_url IS NOT NULL AND TRIM(website_url) <> ''
		  AND (domain_last_checked_at IS NULL OR domain_last_checked_at < NOW() - INTERVAL '7 days')
		ORDER BY domain_last_checked_at NULLS FIRST
		LIMIT $1`, limit)
	if err != nil {
		slog.Error("[scheduler] sweep domain recheck gagal", "err", err)
		return
	}
	for _, row := range stale {
		mediaID := row.strOr("id", "")
		if _, err := d.enqueueJobAt(ctx, "domain_verify", map[string]any{"mediaId": mediaID}, nil, 2); err != nil {
			slog.Warn("[scheduler] enqueue domain verify gagal", "id", mediaID, "err", err)
		}
	}
}

// ---------- 8b. crawl target sync (enqueueCrawlTargetSyncs) ----------

// schEnqueueCrawlTargetSyncs: kandidat media mitra aktif yang target crawl-nya
// belum ada, gagal 1-2x > 24 jam, atau hasil terakhir lebih tua dari
// CRAWLHUB_STALE_MONTHS. No-op bila Crawlhub belum dikonfigurasi.
func (d *Deps) schEnqueueCrawlTargetSyncs(ctx context.Context, limit int) {
	cfg := d.bsLoadCrawlhubConfig(ctx)
	if !bsCrawlhubConfigured(cfg) {
		return
	}
	// Parity `Number(process.env.CRAWLHUB_STALE_MONTHS ?? 3) || 3`: nilai 0/NaN
	// jatuh ke default 3.
	staleMonths := schEnvInt("CRAWLHUB_STALE_MONTHS", 3)
	if staleMonths <= 0 {
		staleMonths = 3
	}

	candidates, err := d.queryRows(ctx, `
		SELECT m.id FROM media m
		LEFT JOIN crawl_targets ct ON ct.media_id = m.id
		WHERE m.partner_id IS NOT NULL
		  AND m.status = 'active'
		  AND m.website_url IS NOT NULL AND TRIM(m.website_url) <> ''
		  AND (
		    ct.media_id IS NULL
		    OR ct.last_crawled_at IS NULL
		    OR (COALESCE(ct.consecutive_failures, 0) BETWEEN 1 AND 2
		        AND ct.updated_at < NOW() - INTERVAL '24 hours')
		    OR ct.last_crawled_at < NOW() - make_interval(months => $1::int)
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'crawl_target_sync' AND j.status IN ('pending','running')
		      AND j.payload->>'mediaId' = m.id
		  )
		ORDER BY ct.last_crawled_at ASC NULLS FIRST
		LIMIT $2`, staleMonths, limit)
	if err != nil {
		slog.Error("[scheduler] sweep crawl target sync gagal", "err", err)
		return
	}
	for _, row := range candidates {
		mediaID := row.strOr("id", "")
		if _, err := d.enqueueJobAt(ctx, "crawl_target_sync", map[string]any{"mediaId": mediaID}, nil, 2); err != nil {
			slog.Warn("[scheduler] enqueue crawl target sync gagal", "id", mediaID, "err", err)
		}
	}
}

// ---------- 8c. pelacak 404 (enqueuePublicationLinkChecks) ----------

// schEnqueuePublicationLinkChecks: artikel published dalam garansi 12 bulan yang
// belum pernah dicek atau > 7 hari lalu, dedupe job pending/running. Publikasi
// berstatus DRAF di website mitra dilewati — URL-nya belum tampil publik.
func (d *Deps) schEnqueuePublicationLinkChecks(ctx context.Context, limit int) {
	stale, err := d.queryRows(ctx, `
		SELECT p.id FROM publications p
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		WHERE p.status = 'published'
		  AND p.published_url IS NOT NULL
		  AND p.published_at > NOW() - INTERVAL '12 months'
		  AND p.publish_status <> 'draft'
		  AND (c.last_checked_at IS NULL OR c.last_checked_at < NOW() - INTERVAL '7 days')
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'publication_link_check' AND j.status IN ('pending','running')
		      AND j.payload->>'publicationId' = p.id
		  )
		ORDER BY c.last_checked_at NULLS FIRST
		LIMIT $1`, limit)
	if err != nil {
		slog.Error("[scheduler] sweep link check gagal", "err", err)
		return
	}
	for _, row := range stale {
		publicationID := row.strOr("id", "")
		if _, err := d.enqueueJobAt(ctx, "publication_link_check", map[string]any{"publicationId": publicationID}, nil, 2); err != nil {
			slog.Warn("[scheduler] enqueue link check gagal", "id", publicationID, "err", err)
		}
	}
}

// ---------- 8d. kesimpulan AI media (sweepMediaAiSummaries) ----------

// schSweepMediaAiSummaries: pilih media tertua tanpa ai_summary (satu per
// domain, dedupe job + domain kembar), enqueue media_ai_summary, dan catat event
// tracking 'queued'. No-op bila AI belum dikonfigurasi.
func (d *Deps) schSweepMediaAiSummaries(ctx context.Context, limit int) {
	runtime, err := ai.LoadAIRuntime(ctx, d.Pool)
	if err != nil || !ai.IsAIConfigured(runtime) {
		return
	}
	delayDays := schEnvInt("MEDIA_AI_SUMMARY_DELAY_DAYS", 0)
	if delayDays < 0 {
		delayDays = 0
	}

	candidates, err := d.queryRows(ctx, `
		SELECT DISTINCT ON (RTRIM(REGEXP_REPLACE(LOWER(TRIM(m.website_url)), '^(https?://)?(www\.)?', ''), '/'))
		        m.id, m.name, m.website_url
		FROM media m
		WHERE m.website_url IS NOT NULL AND TRIM(m.website_url) <> ''
		  AND m.ai_summary IS NULL
		  AND m.created_at <= NOW() - make_interval(days => $1::int)
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'media_ai_summary'
		      AND j.status IN ('pending','running')
		      AND j.payload->>'mediaId' = m.id::text
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM media m2
		    WHERE m2.id <> m.id
		      AND RTRIM(REGEXP_REPLACE(LOWER(TRIM(m2.website_url)), '^(https?://)?(www\.)?', ''), '/')
		          = RTRIM(REGEXP_REPLACE(LOWER(TRIM(m.website_url)), '^(https?://)?(www\.)?', ''), '/')
		      AND (m2.ai_summary IS NOT NULL OR EXISTS (
		        SELECT 1 FROM jobs j2
		        WHERE j2.kind = 'media_ai_summary'
		          AND j2.status IN ('pending','running')
		          AND j2.payload->>'mediaId' = m2.id::text
		      ))
		  )
		ORDER BY RTRIM(REGEXP_REPLACE(LOWER(TRIM(m.website_url)), '^(https?://)?(www\.)?', ''), '/'), m.created_at ASC
		LIMIT $2`, delayDays, limit)
	if err != nil {
		slog.Error("[scheduler] sweep kesimpulan AI gagal", "err", err)
		return
	}
	for _, row := range candidates {
		mediaID := row.strOr("id", "")
		if _, err := d.enqueueJobAt(ctx, "media_ai_summary", map[string]any{"mediaId": mediaID}, nil, 2); err != nil {
			slog.Warn("[scheduler] enqueue kesimpulan AI gagal", "id", mediaID, "err", err)
			continue
		}
		// recordSummaryEvent(status 'queued') — best-effort.
		var nameArg, urlArg any
		if v := row.strOr("name", ""); v != "" {
			nameArg = v
		}
		if v := row.strOr("website_url", ""); v != "" {
			urlArg = v
		}
		if err := d.exec(ctx, `
			INSERT INTO media_ai_summary_events (id, media_id, media_name, website_url, status, pages_tried, error_message, attempt)
			VALUES (gen_random_uuid(), $1, $2, $3, 'queued', '[]'::jsonb, NULL, 0)`,
			mediaID, nameArg, urlArg); err != nil {
			slog.Warn("[scheduler] catat event ringkasan AI gagal", "id", mediaID, "err", err)
		}
	}
}

// ---------- 8e. sinyal SEO & AI (enqueueMediaAiRefreshes) ----------

// schEnqueueMediaAiRefreshes: media aktif yang belum punya skor — belum ada
// baris metrik, atau DA / AI Trust masih kosong. Media yang sudah berskor tidak
// diperiksa ulang otomatis (pemeriksaan berkala 5 bulan dihapus atas permintaan
// pemilik produk); yang masih kosong dicoba lagi paling cepat sekali per hari.
// Predikat ini harus tetap sama dengan platform.mpSeoNeedsCheck.
func (d *Deps) schEnqueueMediaAiRefreshes(ctx context.Context, limit int) {
	stale, err := d.queryRows(ctx, `
		SELECT media.id FROM media
		LEFT JOIN media_seo_metrics sm ON sm.media_id = media.id
		WHERE media.status = 'active'
		  AND media.website_url IS NOT NULL AND media.website_url <> ''
		  AND (sm.media_id IS NULL OR sm.checked_at IS NULL
		       OR ((sm.da IS NULL OR sm.ai_trust_score IS NULL) AND sm.checked_at < NOW() - INTERVAL '1 day'))
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'media_seo_refresh' AND j.status IN ('pending','running')
		      AND j.payload->>'mediaId' = media.id
		  )
		ORDER BY sm.checked_at ASC NULLS FIRST
		LIMIT $1`, limit)
	if err != nil {
		slog.Error("[scheduler] sweep refresh SEO/AI gagal", "err", err)
		return
	}
	for _, row := range stale {
		mediaID := row.strOr("id", "")
		if _, err := d.enqueueJobAt(ctx, "media_seo_refresh", map[string]any{"mediaId": mediaID}, nil, 2); err != nil {
			slog.Warn("[scheduler] enqueue refresh SEO/AI gagal", "id", mediaID, "err", err)
		}
	}
}

// ---------- 9. purge upload basi (uploads.ts) ----------

// schUploadRefRe: ekstraktor nama berkas dari HTML `/uploads/<name>`.
var schUploadRefRe = regexp.MustCompile(`/uploads/([A-Za-z0-9._-]+)`)

// schPurgeStaleUploads: port purgeStaleUploads — berkas di UPLOAD_DIR yang lebih
// tua dari 30 hari DAN tidak lagi direferensikan DB (storage_key,
// featured_image_url, HTML content_versions) layak dibuang.
//
// KEAMANAN: penghapusan fisik hanya dijalankan bila env
// SCHEDULER_PURGE_UPLOADS=true. Default nonaktif — scheduler otomatis tidak
// boleh menghapus berkas produksi tanpa persetujuan operator (mis. bila
// UPLOAD_DIR menunjuk direktori upload bersama app lama). Saat nonaktif, jumlah
// kandidat tetap dihitung dan dilaporkan lewat log, lalu 0 dikembalikan.
func (d *Deps) schPurgeStaleUploads(ctx context.Context) int {
	dir := d.uploadDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	// Kumpulkan nama berkas yang masih dipakai di mana pun. Bila scan DB gagal,
	// jangan hapus apa pun (parity catch → purged 0).
	used := map[string]struct{}{}
	files, err := d.queryRows(ctx, "SELECT storage_key FROM taut_campaign_files WHERE storage_key IS NOT NULL")
	if err != nil {
		slog.Error("[scheduler] purge upload: scan storage_key gagal", "err", err)
		return 0
	}
	for _, r := range files {
		if key := r.strOr("storage_key", ""); key != "" {
			used[schUploadBasename(key)] = struct{}{}
		}
	}
	featured, err := d.queryRows(ctx, "SELECT featured_image_url FROM taut_campaign_media WHERE featured_image_url IS NOT NULL AND featured_image_url <> ''")
	if err != nil {
		slog.Error("[scheduler] purge upload: scan featured_image_url gagal", "err", err)
		return 0
	}
	for _, r := range featured {
		if url := r.strOr("featured_image_url", ""); url != "" {
			used[schUploadBasename(url)] = struct{}{}
		}
	}
	versions, err := d.queryRows(ctx, "SELECT content FROM content_versions WHERE content LIKE '%/uploads/%'")
	if err != nil {
		slog.Error("[scheduler] purge upload: scan HTML gagal", "err", err)
		return 0
	}
	for _, r := range versions {
		for _, m := range schUploadRefRe.FindAllString(r.strOr("content", ""), -1) {
			used[schUploadBasename(m)] = struct{}{}
		}
	}

	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	candidates := 0
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := used[name]; ok {
			continue
		}
		info, sErr := entry.Info()
		if sErr != nil || !info.Mode().IsRegular() {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		candidates++
	}

	if !schBoolEnv("SCHEDULER_PURGE_UPLOADS") {
		if candidates > 0 {
			slog.Info("[scheduler] purge upload dilewati (aktifkan SCHEDULER_PURGE_UPLOADS untuk menghapus)",
				"kandidat", candidates, "dir", dir)
		}
		return 0
	}

	purged := 0
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := used[name]; ok {
			continue
		}
		full := filepath.Join(dir, name)
		info, sErr := entry.Info()
		if sErr != nil || !info.Mode().IsRegular() {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if rErr := os.Remove(full); rErr == nil {
			purged++
		}
	}
	return purged
}

// ---------- renderer email (parity templates/base.ts + registry.ts) ----------

// schRenderLinkEmail: parity linkOnlyTemplate (judul + body teks + tautan).
func (d *Deps) schRenderLinkEmail(ctx context.Context, title, body, link, linkLabel string) (string, string) {
	return d.schBaseEmail(title, schHTMLEscape(body), link, linkLabel)
}

// schRenderDetailEmail: parity detailTemplate (intro + tabel baris).
func (d *Deps) schRenderDetailEmail(ctx context.Context, title, intro string, rows [][2]string, link, linkLabel string) (string, string) {
	var rowHTML strings.Builder
	for _, r := range rows {
		rowHTML.WriteString(`<tr><td style="padding:6px 0;color:#475569;font-size:13px;width:140px;">` +
			schHTMLEscape(r[0]) + `</td><td style="padding:6px 0;font-size:14px;color:#0f172a;">` + schHTMLEscape(r[1]) + `</td></tr>`)
	}
	tableHTML := ""
	if rowHTML.Len() > 0 {
		tableHTML = `<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="margin:16px 0;border-collapse:collapse;">` + rowHTML.String() + `</table>`
	}
	return d.schBaseEmail(title, schHTMLEscape(intro)+tableHTML, link, linkLabel)
}

// schBaseEmail: parity baseTemplate — subject "[App] Title" + HTML mobile-first.
func (d *Deps) schBaseEmail(title, bodyHTML, link, linkLabel string) (string, string) {
	// Parity templates/base.ts: appName default "Tauthub" (bukan APP_NAME);
	// pemanggil template di app lama tidak pernah menimpanya.
	appName := "Tauthub"
	appURL := "https://app.tauthub.com"
	if base := strings.TrimSpace(d.canonicalAppBase()); base != "" {
		appURL = base
	}
	contact := "support@tauthub.com"
	logoURL := strings.TrimRight(appURL, "/") + "/tauthub-icon.png"

	linkBlock := ""
	if link != "" {
		linkBlock = `<p style="margin: 24px 0; text-align: center;"><a href="` + schHTMLEscape(appURL+link) +
			`" target="_blank" rel="noopener" style="background:#1d4ed8;color:#ffffff;padding:12px 24px;border-radius:8px;text-decoration:none;font-weight:600;display:inline-block;">` +
			schHTMLEscape(linkLabel) + `</a></p>`
	}

	html := `<!DOCTYPE html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` +
		schHTMLEscape(title) + `</title></head><body style="margin:0;padding:24px;background:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0f172a;">` +
		`<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:12px;overflow:hidden;">` +
		`<tr><td style="padding:24px 24px 0;text-align:center;"><a href="` + schHTMLEscape(appURL) + `" style="text-decoration:none;display:inline-block;">` +
		`<img src="` + schHTMLEscape(logoURL) + `" alt="" width="28" height="28" style="width:28px;height:28px;display:block;border:0;outline:none;" /></a></td></tr>` +
		`<tr><td style="padding:8px 24px 24px;"><h1 style="margin:0 0 16px;font-size:20px;line-height:1.3;">` + schHTMLEscape(title) + `</h1>` +
		`<div style="margin:0;line-height:1.55;font-size:14px;color:#0f172a;">` + bodyHTML + `</div>` + linkBlock + `</td></tr>` +
		`<tr><td style="padding:16px 24px;border-top:1px solid #e2e8f0;background:#f8fafc;"><p style="margin:0;font-size:12px;color:#475569;">Email ini dikirim oleh ` +
		schHTMLEscape(appName) + `. Jika butuh bantuan, hubungi <a href="mailto:` + schHTMLEscape(contact) + `" style="color:#1d4ed8;">` + schHTMLEscape(contact) + `</a>.</p></td></tr>` +
		`</table></body></html>`
	return "[" + appName + "] " + title, html
}

func schHTMLEscape(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(v)
}

func schOrDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

// ---------- helper kecil ----------

// schRowTime: kolom timestamp dari bsRow → *time.Time.
func schRowTime(row bsRow, key string) *time.Time {
	switch t := row.raw(key).(type) {
	case time.Time:
		v := t
		return &v
	case *time.Time:
		return t
	}
	return nil
}

// schUploadBasename: nama berkas terakhir dari storage_key/URL.
func schUploadBasename(v string) string {
	if i := strings.LastIndex(v, "/"); i >= 0 {
		return v[i+1:]
	}
	return v
}

func schZeroIf(noHistory bool, v float64) any {
	if noHistory {
		return 0
	}
	return v
}

// schRound2If: parity Math.round(v*100)/100, dipaksa 0 saat tanpa riwayat.
func schRound2If(noHistory bool, v float64) any {
	if noHistory {
		return 0
	}
	return math.Round(v*100) / 100
}

func schFloatArg(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func schMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// schEnvInt: env integer dengan default (parity Number(process.env)).
func schEnvInt(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

// schBoolEnv: env boolean longgar (1/true/yes/on).
func schBoolEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
