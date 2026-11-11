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
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
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
	PlanOccurrencesMaterialized int   `json:"plan_occurrences_materialized"`
	PlanOccurrencesFailed       int   `json:"plan_occurrences_failed"`
	PublicationsTriggered       int   `json:"publications_triggered"`
	PartnerRemindersSent        int   `json:"partner_reminders_sent"`
	LateApprovalNotified        int   `json:"late_approval_notified"`
	SLACancelled                int   `json:"sla_cancelled"`
	MediaChangesReminded        int   `json:"media_changes_reminded"`
	MediaIntegrationMissingSent int   `json:"media_integration_missing_sent"`
	MediaChangesExpired         int   `json:"media_changes_expired"`
	RegistrationEmailsBlocked   int   `json:"registration_emails_blocked"`
	SitePlatformsDetected       int   `json:"site_platforms_detected"`
	MediaHealthRefreshed        int   `json:"media_health_refreshed"`
	UploadsPurged               int   `json:"uploads_purged"`
	JobsProcessed               int   `json:"jobs_processed"`
	RetentionDeleted            int64 `json:"retention_deleted"`
	RetentionArchived           int64 `json:"retention_archived"`
}

// toMap: bentuk respons JSON (parity nama field snake_case app lama).
func (r SchedulerResult) toMap() map[string]any {
	return map[string]any{
		"plan_occurrences_materialized":  r.PlanOccurrencesMaterialized,
		"plan_occurrences_failed":        r.PlanOccurrencesFailed,
		"publications_triggered":         r.PublicationsTriggered,
		"partner_reminders_sent":         r.PartnerRemindersSent,
		"late_approval_notified":         r.LateApprovalNotified,
		"sla_cancelled":                  r.SLACancelled,
		"media_changes_reminded":         r.MediaChangesReminded,
		"media_integration_missing_sent": r.MediaIntegrationMissingSent,
		"media_changes_expired":          r.MediaChangesExpired,
		"registration_emails_blocked":    r.RegistrationEmailsBlocked,
		"site_platforms_detected":        r.SitePlatformsDetected,
		"media_health_refreshed":         r.MediaHealthRefreshed,
		"uploads_purged":                 r.UploadsPurged,
		"jobs_processed":                 r.JobsProcessed,
		"retention_deleted":              r.RetentionDeleted,
		"retention_archived":             r.RetentionArchived,
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

	// 4. SLA 3×24 jam persetujuan media: lewat tenggat langsung dibatalkan
	//    otomatis + saldo dikembalikan (tanpa masa perpanjangan).
	res.SLACancelled = d.schSweepPublicationSLA(ctx, schMinInt(maxJobs, 20))

	// 4a. Perubahan media yang diparkir mitra: pengingat admin 24 jam, batal 48 jam.
	res.MediaChangesReminded, res.MediaChangesExpired = d.schSweepMediaPendingChanges(ctx, schMinInt(maxJobs, 20))

	// 4a-bis. Media non-manual tanpa koneksi: sembunyi dari katalog, mitra
	//         diberi tahu sekali per media.
	res.MediaIntegrationMissingSent = d.schSweepMediaIntegrationMissing(ctx, schMinInt(maxJobs, 20))

	// 4a'. OTP pendaftaran yang tidak dijawab 48 jam: email diblokir dari pendaftaran.
	res.RegistrationEmailsBlocked = d.schSweepUnansweredEmailCodes(ctx, schMinInt(maxJobs, 50))

	// 4a''. Platform website media (WordPress/Blogspot/Custom) yang belum terdeteksi.
	res.SitePlatformsDetected = d.schSweepSitePlatforms(ctx, schMinInt(maxJobs, 6))

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
	//    Batch domain/crawl diperkecil (20 -> 5): cooldown di query sudah
	//    membatasi percobaan per media, dan batch kecil memuluskan beban job
	//    per siklus alih-alih meledak sekaligus.
	d.schSweepMediaAiSummaries(ctx, 3)
	d.schEnqueueDomainRechecks(ctx, 5)
	d.schEnqueueCrawlTargetSyncs(ctx, 5)
	d.schEnqueuePublicationLinkChecks(ctx, 20)
	d.schEnqueueMediaAiRefreshes(ctx, 20)
	// Media aktif tanpa data trafik SimilarWeb otomatis masuk antrean; media
	// berdata dijeda 6 bulan (lihat schEnqueueSimilarWebChecks).
	d.schEnqueueSimilarWebChecks(ctx, 5)

	// 8b. Email untuk event notifikasi yang kanalnya dinyalakan admin di panel
	//     Template. Hanya event generic (tanpa pengirim sendiri) dan hanya
	//     notifikasi setelah kanal dinyalakan.
	if n, err := email.DispatchNotificationEmails(ctx, d.Pool, 20); err != nil {
		slog.Error("[scheduler] dispatch email notifikasi gagal", "err", err)
	} else if n > 0 {
		slog.Info("[scheduler] email notifikasi diantrekan", "jumlah", n)
	}

	// 9. Purge upload basi (>30 hari & tak direferensikan DB).
	res.UploadsPurged = d.schPurgeStaleUploads(ctx)

	// 10. Retensi data: pangkas log/cache/event yang melebihi ambang supaya
	//     database tidak membengkak. Maksimal sekali sehari (klaim harian).
	res.RetentionDeleted, res.RetentionArchived = d.schRetention(ctx)

	// 10b. Digest notifikasi: kumpulkan event tier 3 yang belum dibaca jadi
	//      satu email per user per hari. Memakai klaim harian yang sama dengan
	//      retensi, jadi tidak perlu tabel penanda baru dan tidak mungkin
	//      berjalan dua kali dalam sehari.
	//
	//      Tanpa digest, event informasi yang dipetakan ke email menjadi satu
	//      email berbayar per event. Dengan digest, satu user menerima paling
	//      banyak satu email ringkasan per hari, dan hanya kalau ada yang
	//      benar-benar belum ia buka di aplikasi.
	//
	//      MATI SECARA DEFAULT. Digest menyentuh seluruh basis user sekaligus,
	//      jadi tidak boleh aktif karena deploy. Nyalakan sengaja lewat
	//      NOTIFY_DIGEST_ENABLED=true setelah volume dan isinya ditinjau.
	//      Batas per putaran dibatasi NOTIFY_DIGEST_MAX_USERS supaya kesalahan
	//      konfigurasi tidak langsung mengirim ke semua orang.
	if schBoolEnv("NOTIFY_DIGEST_ENABLED") {
		if d.schClaimDaily(ctx, "notify_digest") {
			maxUsers := schEnvInt("NOTIFY_DIGEST_MAX_USERS", 25)
			if maxUsers < 1 {
				maxUsers = 1
			}
			if n, err := notify.EnqueueDigestJobs(ctx, d.Pool, time.Now(), 12*time.Hour, maxUsers); err != nil {
				slog.Error("[scheduler] enqueue digest notifikasi gagal", "err", err)
				d.schReleaseDaily(ctx, "notify_digest")
			} else if n > 0 {
				slog.Info("[scheduler] digest notifikasi diantrekan", "jumlah_user", n)
			}
		}
	}

	// 11. Ringkasan brief: lepaskan baris yang macet 'queued'/'running' tanpa
	//     job hidup (proses pemiliknya mati saat deploy) supaya UI klien tidak
	//     berputar selamanya. Ringkasan kini manual-only — sapuan ini hanya
	//     membersihkan sisa, bukan memicu pembuatan.
	if n := summary.UnstickStale(ctx, d.Pool); n > 0 {
		slog.Info("[scheduler] ringkasan brief macet dilepas", "jumlah", n)
	}

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

// schClaimDaily mengklaim satu tugas harian bernama name.
//
// Mengembalikan true hanya untuk proses yang berhasil memasukkan baris hari
// ini. Scheduler berjalan tiap 10-60 detik, jadi tanpa klaim tugas seperti
// digest akan diantrekan berkali-kali. Pola ini sama dengan schRetention, hanya
// dipisah per nama tugas karena retention_runs memakai tanggal sebagai kunci.
func (d *Deps) schClaimDaily(ctx context.Context, name string) bool {
	tag, err := d.Pool.Exec(ctx,
		`INSERT INTO daily_claims (name, day) VALUES ($1, current_date) ON CONFLICT DO NOTHING`, name)
	if err != nil {
		slog.Error("[scheduler] klaim tugas harian gagal", "tugas", name, "err", err)
		return false
	}
	return tag.RowsAffected() > 0
}

// schReleaseDaily melepaskan klaim supaya siklus berikutnya mencoba lagi.
// Dipakai saat tugas gagal di tengah jalan; tanpa ini satu kegagalan berarti
// seharian penuh tanpa tugas tersebut.
func (d *Deps) schReleaseDaily(ctx context.Context, name string) {
	if _, err := d.Pool.Exec(ctx,
		`DELETE FROM daily_claims WHERE name = $1 AND day = current_date`, name); err != nil {
		slog.Error("[scheduler] lepas klaim tugas harian gagal", "tugas", name, "err", err)
	}
}

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
				"sla_batal", res.SLACancelled)
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

// schSweepPublicationSLA: penegakan SLA 3×24 jam untuk persetujuan media.
//
// Kebijakan (lihat migrasi 287): pesanan `waiting_partner` yang melewati
// COALESCE(scheduled_at, created_at) + 3 hari langsung dibatalkan otomatis dan
// nilainya dikembalikan ke saldo akun klien. Tidak ada masa perpanjangan.
//
// Titik mulai memakai scheduled_at bila ada (itu jadwal yang dijanjikan ke
// klien), dan created_at bila pesanan tidak dijadwalkan — tanpa ini pesanan
// tanpa jadwal tidak akan pernah punya tenggat.
//
// Mengembalikan jumlah pembatalan.
func (d *Deps) schSweepPublicationSLA(ctx context.Context, limit int) int {
	if limit < 1 {
		return 0
	}
	return d.schSweepSLACancel(ctx, limit)
}

// schSweepSLACancel: pembatalan otomatis saat SLA 3×24 jam lewat.
func (d *Deps) schSweepSLACancel(ctx context.Context, limit int) int {
	rows, err := d.queryRows(ctx, `
		SELECT p.id, p.brand_id, m.name AS media_name, d.title AS document_title,
		       pa.public_id AS partner_public_id, pa.user_id AS partner_user_id,
		       b.public_id AS brand_public_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.status = 'waiting_partner'
		  AND COALESCE(p.scheduled_at, p.created_at) + INTERVAL '3 days' < NOW()
		ORDER BY COALESCE(p.scheduled_at, p.created_at)
		LIMIT $1`, limit)
	if err != nil {
		slog.Error("[scheduler] sweep SLA (pembatalan) gagal", "err", err)
		return 0
	}
	cancelled := 0
	for _, pub := range rows {
		publicationID := pub.strOr("id", "")
		brandID := pub.strOr("brand_id", "")
		if brandID == "" {
			continue
		}
		reason := "Dibatalkan otomatis: media tidak merespons melebihi SLA 3×24 jam."
		// Penanda transisi WAJIB dicek: media bisa menekan Setujui di antara
		// pembacaan daftar dan penulisan. Bila transisi tidak terjadi, refund
		// TIDAK boleh jalan — releaseCredit idempoten per publikasi, jadi
		// memanggilnya untuk pesanan yang masih berjalan akan mengembalikan
		// saldo pada pesanan yang tetap diterbitkan.
		ok, err := d.transitionPublicationIf(ctx, publicationID, "cancelled",
			map[string]any{"cancel_reason": reason})
		if err != nil {
			slog.Warn("[scheduler] batalkan pesanan SLA gagal", "pub", publicationID, "err", err)
			continue
		}
		if !ok {
			continue // sudah berpindah status (mis. disetujui media) — lewati
		}
		d.settleCancelledPublication(ctx, brandID, publicationID, "", "publication.sla_cancelled")
		cancelled++

		article := pub.strOr("document_title", "Artikel")
		mediaName := pub.strOr("media_name", "Media")
		dedupeKey := "sla_cancel:" + publicationID
		if partnerUserID := pub.strOr("partner_user_id", ""); partnerUserID != "" {
			link := "/mitra/publications?pub=" + publicationID
			if publicID := pub.strOr("partner_public_id", ""); publicID != "" {
				link = "/mitra/" + publicID + "/publications?pub=" + publicationID
			}
			body := "Pesanan \"" + article + "\" dibatalkan otomatis karena tidak disetujui dalam SLA 3×24 jam."
			d.insertNotificationOnce(ctx, partnerUserID, "publication.sla_cancelled",
				"Pesanan dibatalkan (SLA)", body, link, publicationID, dedupeKey)
		}
		brandPublicID := pub.strOr("brand_public_id", brandID)
		brandLink := "/klien/" + brandPublicID + "/publications?pub=" + publicationID
		brandBody := "Pesanan \"" + article + "\" ke " + mediaName +
			" dibatalkan otomatis karena media tidak merespons melebihi SLA. Nilai pesanan sudah dikembalikan ke saldo akun Anda."
		d.notifyBrandMembersOnPub(ctx, brandID, "publication.sla_cancelled",
			"Pesanan dibatalkan, saldo dikembalikan", brandBody, &brandLink, publicationID)
	}
	return cancelled
}

// insertNotificationOnce: INSERT notifikasi dengan dedupe eksplisit (user +
// dedupe_key + publication). Mengembalikan true bila benar-benar ditulis.
func (d *Deps) insertNotificationOnce(ctx context.Context, userID, notifType, title, body, link, publicationID, dedupeKey string) bool {
	dup, err := d.queryRow(ctx,
		"SELECT id FROM notifications WHERE user_id = $1 AND dedupe_key = $2 AND publication_id = $3 LIMIT 1",
		userID, dedupeKey, publicationID)
	if err != nil || dup != nil {
		return false
	}
	if err := d.exec(ctx, `
		INSERT INTO notifications (id, user_id, type, title, body, link, publication_id, dedupe_key)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)`,
		userID, notifType, title, body, link, publicationID, dedupeKey); err != nil {
		slog.Warn("[scheduler] notifikasi SLA gagal", "pub", publicationID, "err", err)
		return false
	}
	return true
}

// schEmailFollowUp: port routeEmailNotification untuk event pengingat mitra —
// pastikan ada job send_email (parity sendTransactionalEmail).
func (d *Deps) schEmailFollowUp(ctx context.Context, event, recipientUserID, recipientEmail, referenceID, dedupeKey, topic, mediaName, link string) {
	if recipientEmail == "" || !d.schRouteEmailEnabled(ctx, event, recipientUserID) {
		return
	}
	// Registry pusat: publication.reminder dan publication.late_approval
	// sengaja bermode WA, bukan email, jadi keduanya berhenti di sini tanpa
	// biaya. Pemeriksaan tetap dipanggil supaya kalau nanti email diaktifkan
	// lagi, gate baca dan kuota otomatis ikut berlaku.
	decision := notify.ShouldSend(ctx, d.Pool, notify.Request{
		UserID: recipientUserID, Type: event, DedupeKey: dedupeKey, Channel: "email",
	})
	notify.LogDecision(decision, notify.Request{
		UserID: recipientUserID, Type: event, DedupeKey: dedupeKey, Channel: "email",
	})
	if !decision.Send {
		return
	}
	var subject, html string
	style := email.LoadStyle(ctx, d.Pool)
	tplData := map[string]string{
		"nama":   "",
		"judul":  topic,
		"isi":    "Artikel menunggu persetujuan Anda.",
		"status": "",
		"jumlah": "",
		"link":   link,
	}
	usedStored := false
	if event == "publication.late_approval" {
		tplData["isi"] = "Artikel ini melewati batas waktu persetujuan dan otomatis dianggap ditolak."
	}
	if s, h, ok := email.RenderEvent(ctx, d.Pool, style, email.AudiencePartner, event, tplData); ok {
		subject, html = s, h
		usedStored = true
	}
	if !usedStored {
		if event == "publication.late_approval" {
			subject, html = d.schRenderDetailEmail(ctx, style, "Publikasi melewati batas waktu",
				"Artikel ini melewati batas waktu persetujuan dan otomatis dianggap ditolak.",
				[][2]string{{"Topik", schOrDash(topic)}, {"Media", schOrDash(mediaName)}},
				link, "Lihat Detail")
		} else {
			subject, html = d.schRenderLinkEmail(ctx, style, "Pengingat publikasi",
				"Ada artikel yang menunggu persetujuan Anda lebih dari batas waktu yang ditentukan.",
				link, "Tinjau Sekarang")
		}
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

// schEnqueueWADrain: safety net processWaQueue(10).
//
// Predikat due mengikuti kolom next_attempt_at: baris yang baru gagal tidak
// ikut dipungut sampai jeda percobaannya lewat, jadi scheduler tidak menembak
// gateway yang sama berulang tiap 10 detik.
//
// Sebelum memeriksa antrean, baris 'sending' yang macet dilepas lebih dulu.
// Tanpa itu, proses yang mati di tengah pengiriman meninggalkan pesan
// tergantung selamanya.
func (d *Deps) schEnqueueWADrain(ctx context.Context) {
	if _, err := d.Pool.Exec(ctx, `SELECT wa_release_stuck(interval '10 minutes')`); err != nil {
		// Fungsi belum ada sebelum migrasi 266 diterapkan; bukan alasan
		// melewati pengurasan antrean.
		slog.Debug("[scheduler] lepas WA macet gagal", "err", err)
	}
	row, err := d.queryRow(ctx,
		`SELECT COUNT(*)::int AS pending FROM wa_deliveries
		 WHERE status = 'queued' AND attempts < 3
		   AND (next_attempt_at IS NULL OR next_attempt_at <= now())`)
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
		"SELECT id, status, traffic_estimate, media_quality_score, media_tier, integration_type FROM media WHERE id = $1 LIMIT 1", mediaID)
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
		  -- SLA hanya dari publikasi yang terbit <= 7 hari. Publikasi yang
		  -- menggantung karena mitra lupa ACC (atau baru ACC setelah lewat
		  -- seminggu) tidak boleh menyeret rata-rata: durasinya dibuang dari
		  -- data SLA, bukan dicatat sebagai "responsiveness" media.
		  ROUND(AVG(CASE WHEN published_at IS NOT NULL
		                  AND published_at <= created_at + INTERVAL '7 days'
		                 THEN EXTRACT(EPOCH FROM (published_at - created_at)) / 3600.0 END), 2)::double precision AS avg_publish_hours
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
	// Batas atas TAMPILAN SLA 72 jam untuk media berintegrasi API: rata-rata di
	// atas 3 hari hampir pasti sisa publikasi ACC yang telat (outlier > 7 hari
	// sudah dibuang di query). Media integrasi MANUAL dikecualikan — ACC manual
	// memang butuh hari. Skor tetap dihitung dari nilai mentah supaya kualitas
	// tidak naik palsu karena pembulatan tampilan.
	displayAvg := avgPublishHours
	if displayAvg != nil && *displayAvg > 72 && media.strOr("integration_type", "") != "manual" {
		v := 72.0
		displayAvg = &v
	}
	breakdown := map[string]any{
		"availability":          schZeroIf(noHistory, availability),
		"success_rate":          schRound2If(noHistory, successRate),
		"publish_time":          schZeroIf(noHistory, publishTime),
		"rejection_rate":        schRound2If(noHistory, rejectionRate),
		"operational_total":     schZeroIf(noHistory, operationalTotal),
		"quality":               schFloatArg(quality),
		"quality_source":        qualitySource,
		"reach":                 nil,
		"total":                 int(totalScore),
		"published":             published,
		"rejected":              rejected,
		"failed":                failed,
		"avg_publish_hours":     schFloatArg(displayAvg),
		"avg_publish_hours_raw": schFloatArg(avgPublishHours),
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
//
// Cooldown 6 jam berdasarkan riwayat job (bukan hanya status pending): bila
// verifikasi gagal sebelum sempat memperbarui domain_last_checked_at, media
// yang sama akan lolos gate 7-hari SETIAP siklus scheduler (10 detik) dan
// menghasilkan ribuan job sia-sia. Satu percobaan per media per 6 jam sudah
// cukup responsif untuk pemulihan.
func (d *Deps) schEnqueueDomainRechecks(ctx context.Context, limit int) {
	stale, err := d.queryRows(ctx, `
		SELECT id FROM media
		WHERE partner_id IS NOT NULL
		  AND website_url IS NOT NULL AND TRIM(website_url) <> ''
		  AND (domain_last_checked_at IS NULL OR domain_last_checked_at < NOW() - INTERVAL '7 days')
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'domain_verify'
		      AND j.payload->>'mediaId' = media.id
		      AND (j.status IN ('pending', 'running') OR j.created_at > NOW() - INTERVAL '6 hours')
		  )
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

// defaultCrawlStaleDays: umur maksimum salinan crawl_targets sebelum dianggap
// basi. Selaras dengan interval crawl Crawlhub (CRAWL_INTERVAL_HOURS, default
// 240 jam = 10 hari) plus kelonggaran, jadi angka velocity di katalog bergerak
// tiap siklus crawl — bukan tiap kuartal seperti default lama (3 bulan).
const defaultCrawlStaleDays = 7

// crawlStaleDays: CRAWLHUB_STALE_DAYS. CRAWLHUB_STALE_MONTHS yang lama tetap
// dihormati (×30) supaya env produksi yang sudah terpasang tidak berubah diam-diam.
func crawlStaleDays() int {
	if v := schEnvInt("CRAWLHUB_STALE_DAYS", 0); v > 0 {
		return v
	}
	if v := schEnvInt("CRAWLHUB_STALE_MONTHS", 0); v > 0 {
		return v * 30
	}
	return defaultCrawlStaleDays
}

// schEnqueueCrawlTargetSyncs: kandidat media mitra aktif yang target crawl-nya
// belum ada, gagal 1-2x > 24 jam, belum punya angka sama sekali, atau hasil
// terakhirnya lebih tua dari CRAWLHUB_STALE_DAYS. No-op bila Crawlhub belum
// dikonfigurasi.
func (d *Deps) schEnqueueCrawlTargetSyncs(ctx context.Context, limit int) {
	cfg := d.bsLoadCrawlhubConfig(ctx)
	if !bsCrawlhubConfigured(cfg) {
		return
	}
	staleDays := crawlStaleDays()

	candidates, err := d.queryRows(ctx, `
		SELECT m.id FROM media m
		LEFT JOIN crawl_targets ct ON ct.media_id = m.id
		WHERE m.partner_id IS NOT NULL
		  AND m.status = 'active'
		  AND m.website_url IS NOT NULL AND TRIM(m.website_url) <> ''
		  AND (
		    ct.media_id IS NULL
		    OR ct.last_crawled_at IS NULL
		    -- Gagal/terblokir berulang: tetap dicoba tiap 24 jam (fallback
		    -- Google News/Wayback bisa berhasil walau endpoint situs 403).
		    OR COALESCE(ct.consecutive_failures, 0) > 0
		    OR ct.last_status IN ('blocked', 'error', 'no_source')
		    -- Belum pernah dapat angka (baru terdaftar / crawl terakhir diblokir):
		    -- cek ulang tiap hari sampai angkanya ada, jangan tunggu 7 hari.
		    OR ct.articles_7d IS NULL
		    OR ct.last_crawled_at < NOW() - make_interval(days => $1::int)
		  )
		  AND (ct.updated_at IS NULL OR ct.updated_at < NOW() - INTERVAL '24 hours')
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'crawl_target_sync'
		      AND j.payload->>'mediaId' = m.id
		      -- Cooldown 24 jam dari percobaan terakhir, bukan hanya status
		      -- pending: media tanpa baris crawl_targets (sync selalu gagal
		      -- sebelum menulis ct) akan memenuhi gate SETIAP siklus dan
		      -- membanjiri antrean.
		      AND (j.status IN ('pending', 'running') OR j.created_at > NOW() - INTERVAL '24 hours')
		  )
		ORDER BY ct.last_crawled_at ASC NULLS FIRST
		LIMIT $2`, staleDays, limit)
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

// schEnqueuePublicationLinkChecks: artikel published yang masih dalam masa
// garansi medianya (lihat db.LinkWarrantySQL; media dengan tautan permanen
// = 0 tidak pernah kedaluwarsa) dan belum pernah dicek, atau sudah waktunya
// dicek lagi: HARIAN selama 30 hari pertama setelah tayang (masa garansi
// jalan, pelanggaran perlu dua pengecekan berturut-turut), lalu MINGGUAN.
// Dedupe job pending/running. Publikasi berstatus DRAF di website mitra
// dilewati, URL-nya belum tampil publik. Publikasi yang klaim garansinya
// sudah selesai (dana dikembalikan atau digantikan) tidak dipantau lagi.
func (d *Deps) schEnqueuePublicationLinkChecks(ctx context.Context, limit int) {
	stale, err := d.queryRows(ctx, `
		SELECT p.id FROM publications p
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		JOIN media m ON m.id = p.media_id
		WHERE p.status = 'published'
		  AND p.published_url IS NOT NULL
		  AND `+db.LinkWarrantySQL("p", "m")+`
		  AND p.publish_status <> 'draft'
		  AND (c.last_checked_at IS NULL
		       OR (p.published_at > NOW() - INTERVAL '30 days' AND c.last_checked_at < NOW() - INTERVAL '1 day')
		       OR c.last_checked_at < NOW() - INTERVAL '7 days')
		  AND NOT EXISTS (
		    SELECT 1 FROM link_guardian_claims g
		    WHERE g.publication_id = p.id AND g.status IN ('refunded', 'replaced')
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'publication_link_check'
		      AND j.payload->>'publicationId' = p.id
		      -- Cooldown 24 jam: cek yang gagal sebelum mencatat hasil tidak
		      -- boleh memicu percobaan baru setiap siklus scheduler.
		      AND (j.status IN ('pending', 'running') OR j.created_at > NOW() - INTERVAL '24 hours')
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

// ---------- 8c. similarweb check (antrean otomatis) ----------

// schTrafficProviderReady: apakah provider trafik benar-benar bisa dipakai.
//
// Dipisah dari paket platform karena platform meng-import brandflow (bukan
// sebaliknya), jadi helper tidak boleh saling memanggil. Query-nya sengaja
// sepadan dengan platform.mpCheckProviderReadiness: key RapidAPI SimilarWeb
// wajib ada.
//
// last_da_error tidak dibaca di sini: kolom itu milik provider DA/PA (listing
// RapidAPI lain dengan langganan terpisah). Dulu penolakan provider DA ikut
// menghentikan cek trafik. Kegagalan trafik per media sudah diredam oleh
// cooldown 1 jam di query sweep di bawah.
func (d *Deps) schTrafficProviderReady(ctx context.Context) (bool, string) {
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(sealed_similarweb_api_key, '') AS rapid_key
		FROM platform_seo_config WHERE id = 'default' LIMIT 1`)
	if err != nil || row == nil {
		return false, "konfigurasi SEO platform belum ada"
	}
	if strings.TrimSpace(row.strOr("rapid_key", "")) == "" {
		return false, "key RapidAPI SimilarWeb belum diisi"
	}
	return true, ""
}

// schEnqueueSimilarWebChecks: aturan pemilik produk — media aktif yang BELUM
// pernah punya data trafik SimilarWeb otomatis masuk antrian pemeriksaan tiap
// siklus scheduler (limit kecil per tick supaya panggilan provider tidak
// jebol), sementara media yang datanya sudah ada dibiarkan tersimpan sampai
// usia snapshotnya melewati 6 bulan baru minta ulang. Job-nya kind
// `media_similarweb_refresh` — processor yang sama dengan tombol admin.
//
// Snapshot dengan kunjungan 0 tetap dihitung "sudah punya data": provider
// menjawab 0 untuk domain yang tidak terindeks, dan mengulanginya tiap siklus
// hanya membakar kuota RapidAPI. Pemeriksaan ulang terjadi setelah 6 bulan.
//
// Prioritas: media yang BELUM PERNAH dicek (tanpa snapshot) didahulukan —
// termasuk domain yang baru ditambahkan — lalu media yang datanya basi. Job
// yang gagal dicoba lagi setelah 1 jam, bukan 24 jam, supaya domain baru
// tidak tertahan sehari hanya karena provider sempat rate-limited.
func (d *Deps) schEnqueueSimilarWebChecks(ctx context.Context, limit int) {
	if limit < 1 {
		limit = 1
	}
	// Pengaman kredensial: tanpa key RapidAPI SimilarWeb (atau saat provider
	// sedang menolak), setiap siklus (60 detik) melahirkan job yang pasti
	// gagal — pernah 184 baris `failed` dari provider lama, dan tiap percobaan
	// tetap memakai kuota RapidAPI.
	if ready, reason := d.schTrafficProviderReady(ctx); !ready {
		// Log sekali per siklus sudah cukup berisik; cukup diamkan dan biarkan
		// chip status di UI admin yang memberi tahu.
		_ = reason
		return
	}
	stale, err := d.queryRows(ctx, `
		SELECT media.id FROM media
		WHERE media.status = 'active'
		  AND media.website_url IS NOT NULL AND media.website_url <> ''
		AND NOT EXISTS (
		    SELECT 1 FROM media_similarweb_snapshots s
		    WHERE s.media_id = media.id
		      AND s.created_at >= NOW() - INTERVAL '6 months'
		      AND (s.insights->>'visits') IS NOT NULL
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'media_similarweb_refresh' AND j.status IN ('pending','running')
		      AND j.payload->>'mediaId' = media.id
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'media_similarweb_refresh' AND j.status = 'failed'
		      AND j.payload->>'mediaId' = media.id
		      AND j.updated_at > NOW() - INTERVAL '1 hour'
		  )
		ORDER BY
		  -- Media yang BELUM PERNAH dicek (tanpa snapshot) didahulukan,
		  -- termasuk domain yang baru ditambahkan. Di dalam kelompok itu yang
		  -- paling baru dibuat diperiksa lebih dulu. Media yang hanya basi
		  -- (>6 bulan) menyusul.
		  CASE WHEN EXISTS (
		    SELECT 1 FROM media_similarweb_snapshots s0 WHERE s0.media_id = media.id
		  ) THEN 1 ELSE 0 END,
		  media.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		slog.Error("[scheduler] sweep similarweb gagal", "err", err)
		return
	}
	for _, row := range stale {
		mediaID := row.strOr("id", "")
		if _, err := d.enqueueJobAt(ctx, "media_similarweb_refresh", map[string]any{"mediaId": mediaID}, nil, 2); err != nil {
			slog.Warn("[scheduler] enqueue similarweb gagal", "id", mediaID, "err", err)
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
	// Unggahan profiler/chatbot (dokumen, gambar, dan ledger siklus hidupnya —
	// termasuk status deleted: berkasnya mungkin belum tersapu worker cleanup,
	// biarkan worker profiler yang mengelola, bukan sweep 30-hari ini).
	for _, scan := range []string{
		"SELECT storage_key FROM chat_attachments",
		"SELECT storage_key FROM profiler_documents WHERE storage_key IS NOT NULL",
		"SELECT storage_key FROM profiler_images WHERE storage_key IS NOT NULL",
	} {
		rows, err := d.queryRows(ctx, scan)
		if err != nil {
			slog.Error("[scheduler] purge upload: scan storage_key profiler gagal", "query", scan, "err", err)
			return 0
		}
		for _, r := range rows {
			if key := r.strOr("storage_key", ""); key != "" {
				used[schUploadBasename(key)] = struct{}{}
			}
		}
	}
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

// schRenderLinkEmail: pengingat singkat (judul + paragraf + tautan).
// Kerangka, logo, dan footer berasal dari package email supaya mitra menerima
// tampilan yang sama dengan email auth/platform.
func (d *Deps) schRenderLinkEmail(ctx context.Context, style email.Style, title, body, link, linkLabel string) (string, string) {
	rendered := email.DetailStyled(style, title, body, nil, link, linkLabel)
	return "[" + schAppName + "] " + rendered.Subject, rendered.HTML
}

// schRenderDetailEmail: intro + tabel baris rincian.
func (d *Deps) schRenderDetailEmail(ctx context.Context, style email.Style, title, intro string, rows [][2]string, link, linkLabel string) (string, string) {
	rendered := email.DetailStyled(style, title, intro, rows, link, linkLabel)
	return "[" + schAppName + "] " + rendered.Subject, rendered.HTML
}

// schAppName: parity templates/base.ts — appName default "Tauthub" (bukan
// APP_NAME); pemanggil template di app lama tidak pernah menimpanya.
const schAppName = "Tauthub"

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
