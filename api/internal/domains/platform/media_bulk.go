package platform

// media_bulk.go: aksi massal media admin — arsip massal, dampak arsip, bulk
// arsip/hapus, pulihkan arsip, refresh SEO/SimilarWeb massal, sinkron
// credit_cost, dan recompute health massal. Port route:
// media/archive-all, media/archive-impact, media/bulk, media/restore-archived,
// media/seo-bulk, media/similarweb-bulk, media/sync-credit-cost,
// media/recompute-health.

import (
	"context"
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
)

// ---------- GET /v1/platform/media/archive-impact ----------

// mpActivePublicationCountsByStatus:jumlah publikasi aktif per status pada media non-arsip.
func (d *Deps) mpActivePublicationCountsByStatus(ctx context.Context) (map[string]int, error) {
	ph := mpPlaceholders(len(mpActivePublicationStatuses), 1)
	args := make([]any, 0, len(mpActivePublicationStatuses))
	for _, s := range mpActivePublicationStatuses {
		args = append(args, s)
	}
	rows, err := d.queryRows(ctx, `
		SELECT p.status, COUNT(*) AS total
		FROM publications p
		JOIN media m ON m.id = p.media_id
		WHERE m.status <> 'archived' AND p.status IN (`+ph+`)
		GROUP BY p.status`, args...)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		out[row.strOr("status", "")] = row.intOr("total", 0)
	}
	return out, nil
}

// mpActivePublicationCountsByMedia: rincian per media (nama + jumlah per status).
func (d *Deps) mpActivePublicationCountsByMedia(ctx context.Context) ([]map[string]any, error) {
	ph := mpPlaceholders(len(mpActivePublicationStatuses), 1)
	args := make([]any, 0, len(mpActivePublicationStatuses))
	for _, s := range mpActivePublicationStatuses {
		args = append(args, s)
	}
	rows, err := d.queryRows(ctx, `
		SELECT m.id, m.name, p.status, COUNT(*) AS total
		FROM publications p
		JOIN media m ON m.id = p.media_id
		WHERE m.status <> 'archived' AND p.status IN (`+ph+`)
		GROUP BY m.id, m.name, p.status
		ORDER BY m.name ASC, p.status ASC`, args...)
	if err != nil {
		return nil, err
	}
	type entry struct {
		ID     string
		Name   string
		Total  int
		Counts map[string]int
	}
	order := []string{}
	byMedia := map[string]*entry{}
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		id := row.strOr("id", "")
		e, exists := byMedia[id]
		if !exists {
			e = &entry{ID: id, Name: row.strOr("name", ""), Counts: map[string]int{}}
			byMedia[id] = e
			order = append(order, id)
		}
		total := row.intOr("total", 0)
		e.Total += total
		e.Counts[row.strOr("status", "")] = total
	}
	out := make([]map[string]any, 0, len(order))
	for _, id := range order {
		e := byMedia[id]
		out = append(out, map[string]any{"id": e.ID, "name": e.Name, "total": e.Total, "counts": e.Counts})
	}
	return out, nil
}

func (d *Deps) MediaArchiveImpact(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(), `
		SELECT
		  (SELECT COUNT(*) FROM media WHERE status = 'archived') AS archived,
		  (SELECT COUNT(*) FROM media WHERE status <> 'archived') AS archivable`)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	archived, archivable := 0, 0
	if row != nil {
		pr := ptRow(ptNormalizeMap(row))
		archived = pr.intOr("archived", 0)
		archivable = pr.intOr("archivable", 0)
	}
	byStatus, err := d.mpActivePublicationCountsByStatus(r.Context())
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	byMedia, err := d.mpActivePublicationCountsByMedia(r.Context())
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	blocking := []string{}
	for _, s := range mpArchiveBlockingStatuses() {
		if byStatus[s] > 0 {
			blocking = append(blocking, s)
		}
	}
	byStatusAny := make(map[string]any, len(byStatus))
	for k, v := range byStatus {
		byStatusAny[k] = v
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"archived":             archived,
		"archivable":           archivable,
		"pending_publications": byStatus["waiting_partner"],
		"by_status":            byStatusAny,
		"blocked":              len(blocking) > 0,
		"blocking_statuses":    blocking,
		"by_media":             byMedia,
	})
}

// ---------- POST /v1/platform/media/archive-all ----------

func (d *Deps) MediaArchiveAll(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if err := d.mpWriteRate(r.Context(), "write:platform-media-archive-all:"+user.ID); err != nil {
		writeErr(w, d, err)
		return
	}

	byStatus, err := d.mpActivePublicationCountsByStatus(r.Context())
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	blocking := []string{}
	details := []string{}
	for _, s := range mpArchiveBlockingStatuses() {
		if byStatus[s] > 0 {
			blocking = append(blocking, s)
			details = append(details, ptItoa(byStatus[s])+" "+s)
		}
	}
	if len(blocking) > 0 {
		byStatusAny := make(map[string]any, len(byStatus))
		for k, v := range byStatus {
			byStatusAny[k] = v
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"detail": "Arsip massal ditolak: " + strings.Join(details, ", ") +
				" publikasi masih aktif. Tunggu sampai publikasi tersebut selesai, lalu coba lagi.",
			"blocked":           true,
			"blocking_statuses": blocking,
			"by_status":         byStatusAny,
		})
		return
	}

	media, err := d.queryRows(r.Context(), "SELECT id FROM media WHERE status <> 'archived' ORDER BY id ASC")
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	if len(media) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"archived": 0, "total": 0, "pending_publications": 0, "blocked": false})
		return
	}
	ids := make([]string, 0, len(media))
	for _, raw := range media {
		ids = append(ids, ptRow(ptNormalizeMap(raw)).strOr("id", ""))
	}
	ph := mpPlaceholders(len(ids), 1)
	idArgs := make([]any, 0, len(ids))
	for _, id := range ids {
		idArgs = append(idArgs, id)
	}

	pendingRows, err := d.queryRows(r.Context(), `
		SELECT media_id, COUNT(*) AS total FROM publications
		WHERE status = 'waiting_partner' AND media_id IN (`+ph+`)
		GROUP BY media_id`, idArgs...)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	pending := 0
	for _, raw := range pendingRows {
		pending += ptRow(ptNormalizeMap(raw)).intOr("total", 0)
	}

	updated, err := d.execRows(r.Context(),
		"UPDATE media SET status = 'archived', updated_at = NOW() WHERE status <> 'archived' AND id IN ("+ph+")", idArgs...)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	d.mpExpireMediaNotificationsBulk(r.Context(), ids)

	auditArgs := append([]any{user.ID}, idArgs...)
	auditPh := mpPlaceholders(len(ids), 2)
	if err := d.exec(r.Context(), `
		INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id)
		SELECT gen_random_uuid(), $1, 'media.archived', 'media', m.id FROM media m WHERE m.id IN (`+auditPh+`)`,
		auditArgs...); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"archived":             updated,
		"total":                len(media),
		"pending_publications": pending,
		"blocked":              false,
	})
}

// ---------- POST/DELETE /v1/platform/media/bulk ----------

// mpBulkBlocked: konsistensi bentuk blocked di app lama.
type mpBulkBlocked struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

func mpBulkIDs(body map[string]any) ([]string, bool) {
	ids, has := stringSliceField(body, "ids")
	if !has {
		return nil, false
	}
	return ids, true
}

// MediaBulkUpsert: POST /v1/platform/media/bulk — arsipkan (soft delete) beberapa media.
func (d *Deps) MediaBulkUpsert(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if err := d.mpWriteRate(r.Context(), "write:platform-media-bulk:"+user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	ids, valid := mpBulkIDs(body)
	if !valid || len(ids) < 1 || len(ids) > 100 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}

	blocked := []mpBulkBlocked{}
	archived := 0
	for _, mediaID := range ids {
		existing, err := d.mpScopedMedia(r.Context(), mediaID)
		if err != nil {
			blocked = append(blocked, mpBulkBlocked{ID: mediaID, Name: mediaID, Reason: "tidak ditemukan"})
			continue
		}
		if existing.strOr("status", "") == "archived" {
			blocked = append(blocked, mpBulkBlocked{ID: mediaID, Name: existing.strOr("name", ""), Reason: "sudah diarsipkan"})
			continue
		}
		if err := d.exec(r.Context(), "UPDATE media SET status = 'archived', updated_at = NOW() WHERE id = $1", mediaID); err != nil {
			writeErr(w, d, mpMediaError(err))
			return
		}
		d.mpExpireMediaNotifications(r.Context(), mediaID)
		writeAuditBestEffort(d, r.Context(), user.ID, "media.archived", "media", mediaID)
		archived++
	}
	writeJSON(w, http.StatusOK, map[string]any{"archived": archived, "blocked": blocked})
}

// MediaBulkDelete: DELETE /v1/platform/media/bulk — hapus permanen beberapa media.
func (d *Deps) MediaBulkDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if err := d.mpWriteRate(r.Context(), "write:platform-media-bulk:"+user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	ids, valid := mpBulkIDs(body)
	if !valid || len(ids) < 1 || len(ids) > 100 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}

	blocked := []mpBulkBlocked{}
	deleted := 0
	for _, mediaID := range ids {
		existing, err := d.mpScopedMedia(r.Context(), mediaID)
		if err != nil {
			blocked = append(blocked, mpBulkBlocked{ID: mediaID, Name: mediaID, Reason: "tidak ditemukan"})
			continue
		}
		usage, err := d.queryRow(r.Context(), `
			SELECT
			  (SELECT COUNT(*) FROM publications WHERE media_id = $1) AS publications,
			  (SELECT COUNT(*) FROM partner_earnings WHERE media_id = $1) AS earnings,
			  (SELECT COUNT(*) FROM publication_plan_occurrences WHERE media_id = $1) AS plan_occurrences`, mediaID)
		if err != nil {
			writeErr(w, d, mpMediaError(err))
			return
		}
		u := ptRow(ptNormalizeMap(usage))
		reasons := []string{}
		if u.intOr("publications", 0) > 0 {
			reasons = append(reasons, "publikasi")
		}
		if u.intOr("earnings", 0) > 0 {
			reasons = append(reasons, "earning")
		}
		if u.intOr("plan_occurrences", 0) > 0 {
			reasons = append(reasons, "rencana publikasi")
		}
		if len(reasons) > 0 {
			blocked = append(blocked, mpBulkBlocked{ID: mediaID, Name: existing.strOr("name", ""), Reason: strings.Join(reasons, ", ")})
			continue
		}
		if err := d.exec(r.Context(), "DELETE FROM media WHERE id = $1", mediaID); err != nil {
			writeErr(w, d, mpMediaError(err))
			return
		}
		writeAuditBestEffort(d, r.Context(), user.ID, "media.deleted", "media", mediaID)
		deleted++
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted, "blocked": blocked})
}

// ---------- POST /v1/platform/media/restore-archived ----------

func (d *Deps) MediaRestoreArchived(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	archived, err := d.queryRows(r.Context(), "SELECT id FROM media WHERE status = 'archived' ORDER BY id ASC")
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	restored := 0
	for _, raw := range archived {
		mediaID := ptRow(ptNormalizeMap(raw)).strOr("id", "")
		if err := d.exec(r.Context(), "UPDATE media SET status = 'active', updated_at = NOW() WHERE id = $1", mediaID); err != nil {
			d.logWarn("bulk restore media gagal", mediaID, err)
			continue
		}
		writeAuditBestEffort(d, r.Context(), user.ID, "media.restored", "media", mediaID)
		// Media kembali tampil di direktori: pastikan DA dan trafiknya terisi.
		// Guard di dalam helper mencegah antrean ganda dan melewati media yang
		// penyedianya belum dikonfigurasi.
		d.mpEnqueueMediaChecks(r.Context(), mediaID, mpCheckAll, "media.restored")
		restored++
	}
	writeJSON(w, http.StatusOK, map[string]any{"restored": restored, "total": len(archived)})
}

// ---------- POST /v1/platform/media/recompute-health (massal) ----------

func (d *Deps) MediaRecomputeHealthBulk(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	requested := []string{}
	if arr, has := rawField(body, "media_id"); has {
		if list, isList := arr.([]any); isList {
			for _, v := range list {
				if s, isStr := v.(string); isStr && strings.TrimSpace(s) != "" {
					requested = append(requested, strings.TrimSpace(s))
				}
			}
		}
	}

	var targets []rowMap
	var err error
	if len(requested) > 0 {
		ph := mpPlaceholders(len(requested), 1)
		args := make([]any, 0, len(requested))
		for _, id := range requested {
			args = append(args, id)
		}
		targets, err = d.queryRows(r.Context(), "SELECT id FROM media WHERE id IN ("+ph+")", args...)
	} else {
		targets, err = d.queryRows(r.Context(), "SELECT id FROM media")
	}
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}

	computed := 0
	failed := []map[string]any{}
	for _, raw := range targets {
		mediaID := ptRow(ptNormalizeMap(raw)).strOr("id", "")
		if _, _, err := d.mpRefreshMediaHealth(r.Context(), mediaID); err != nil {
			detail := err.Error()
			if len(detail) > 200 {
				detail = detail[:200]
			}
			failed = append(failed, map[string]any{"id": mediaID, "detail": detail})
			continue
		}
		computed++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "refreshed": computed, "total": len(targets), "computed": computed, "failed": failed,
	})
}

// ---------- POST /v1/platform/media/seo-bulk ----------

func (d *Deps) MediaSEOBulk(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	limit := 25
	if v, has := intField(body, "limit"); has {
		if v < 1 || v > 50 {
			writeErr(w, d, errUnprocessable("Number must be less than or equal to 50"))
			return
		}
		limit = v
	}

	stale, err := d.queryRows(r.Context(), `
		SELECT media.id, media.name, media.website_url
		FROM media
		LEFT JOIN media_seo_metrics sm ON sm.media_id = media.id
		WHERE media.status = 'active'
		  AND media.website_url IS NOT NULL AND media.website_url <> ''
		  AND (`+mpSeoNeedsCheck+`)
		ORDER BY sm.checked_at ASC NULLS FIRST
		LIMIT $1`, limit)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	results := []map[string]any{}
	okCount := 0
	for _, raw := range stale {
		row := ptRow(ptNormalizeMap(raw))
		mediaID := row.strOr("id", "")
		name := row.strOr("name", mediaID)
		_, seoErr := d.mpCheckAndStoreSeoMetrics(r.Context(), mediaID, row.anyOrNil("website_url"), user.ID)
		if seoErr == nil {
			results = append(results, map[string]any{"media_id": mediaID, "name": name, "ok": true, "detail": "fresh"})
			okCount++
			continue
		}
		results = append(results, map[string]any{"media_id": mediaID, "name": name, "ok": false, "detail": mpShortError(seoErr, 160)})
		// Provider mati/rate-limited/key bermasalah → hentikan batch.
		if mpErr, isMP := seoErr.(*mpSeoError); isMP && mpStopStatuses[mpErr.Status] {
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"requested":  limit,
		"candidates": len(stale),
		"checked":    len(results),
		"ok":         okCount,
		"failed":     len(results) - okCount,
		"results":    results,
	})
}

var mpStopStatuses = map[int]bool{401: true, 403: true, 409: true, 429: true, 502: true, 503: true}

func mpShortError(err error, max int) string {
	if err == nil {
		return "Gagal tidak diketahui"
	}
	msg := err.Error()
	if len(msg) > max {
		msg = msg[:max]
	}
	return msg
}

// ---------- POST /v1/platform/media/seo-refresh-all ----------

// mpSeoNeedsCheck: predikat "media belum punya skor" — belum ada baris metrik,
// atau DA / AI Trust masih kosong. Media yang skornya sudah ada tidak diperiksa
// ulang. Semua kueri memakai alias `sm` untuk media_seo_metrics.
//
// Tanpa penahan waktu: tombol ini dijalankan karena admin memang minta, dan
// penahan "sekali per hari" pernah membuat tombolnya menjawab "semua media sudah
// punya skor" tepat setelah kunci penyedia diperbaiki (semua media baru dicek
// beberapa menit sebelumnya sehingga terlewat). Pembatas spam-nya sekarang:
// dedupe job pending/running + rate limit 2 permintaan/menit. Penahan harian
// tetap dipakai jalur otomatis (sweep scheduler + pemicu saat media dibuat).
const mpSeoNeedsCheck = `
			  (sm.media_id IS NULL OR sm.checked_at IS NULL
			   OR sm.da IS NULL OR sm.ai_trust_score IS NULL)`

// mpStaleSeoWhere: predikat media yang metrik SEO/AI-nya perlu diperiksa.
const mpStaleSeoWhere = `
			m.status = 'active'
			  AND m.website_url IS NOT NULL AND m.website_url <> ''
			  AND (` + mpSeoNeedsCheck + `)`

// mpDaMissingWhere: predikat tombol "Cek DA semua" — media aktif yang DOMAIN
// AUTHORITY-nya belum ada. Pemeriksaan DA dan AI Trust dijalankan oleh job yang
// sama (satu panggilan penyedia + satu pengukuran kecepatan), jadi tombol ini
// memilih media berdasarkan DA; media yang sudah punya DA dilewati.
const mpDaMissingWhere = `
			m.status = 'active'
			  AND m.website_url IS NOT NULL AND m.website_url <> ''
			  AND sm.da IS NULL`

// MediaSEORefreshAll: antrekan pemeriksaan DA/SEO untuk media yang metriknya
// belum ada. Dijalankan di latar lewat job `media_seo_refresh` (kind yang sama
// dipakai scheduler), jadi aman untuk ratusan media dan tidak menahan request.
// Media yang masih punya job pending/running dilewati.
//   - da_only: true → hanya media yang DA-nya belum ada (tombol "Cek DA semua")
//   - force: true   → semua media aktif, termasuk yang sudah berskor
func (d *Deps) MediaSEORefreshAll(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if err := auth.CheckWriteRate(r.Context(), d.Pool, "media-seo-refresh-all:"+user.ID, 2); err != nil {
		writeErr(w, d, ptErrTooMany("Antrean cek DA baru saja dijalankan. Tunggu satu menit lalu coba lagi."))
		return
	}
	body := decodeJSON(r)
	force, _ := boolField(body, "force")
	daOnly, _ := boolField(body, "da_only")
	needsCheck := mpStaleSeoWhere
	switch {
	case force:
		needsCheck = `
			m.status = 'active'
			  AND m.website_url IS NOT NULL AND m.website_url <> ''`
	case daOnly:
		needsCheck = mpDaMissingWhere
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT m.id
		FROM media m
		LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
		WHERE`+needsCheck+`
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'media_seo_refresh' AND j.status IN ('pending','running')
		      AND j.payload->>'mediaId' = m.id
		  )
		ORDER BY sm.checked_at ASC NULLS FIRST
		LIMIT 500`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	enqueued := 0
	skipped := 0
	for _, row := range rows {
		mediaID := row.strOr("id", "")
		if mediaID == "" {
			skipped++
			continue
		}
		if _, jErr := d.mpEnqueueJob(r.Context(), "media_seo_refresh", map[string]any{"mediaId": mediaID}, 2); jErr != nil {
			d.logWarn("enqueue media_seo_refresh gagal", mediaID, jErr)
			skipped++
			continue
		}
		enqueued++
	}
	// Total kandidat (tanpa dedupe) supaya UI bisa menyebut sisa antrean. Memakai
	// predikat yang sama dengan pemilihannya: dulu selalu memakai "belum berskor
	// lengkap", sehingga tombol "Cek DA semua" melaporkan "66 dari 71 kandidat"
	// padahal kandidat DA memang 66.
	totalRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) AS total FROM media m
		LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
		WHERE`+needsCheck)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	total := 0
	if totalRow != nil {
		total = totalRow.intOr("total", 0)
	}
	d.audit(r.Context(), user.ID, "media.seo_refresh_all", "media", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"enqueued":    enqueued,
		"skipped":     skipped,
		"candidates":  len(rows),
		"total_stale": total,
	})
}

// MediaSEORefreshCancel: POST /v1/platform/media/seo-refresh-cancel — hentikan
// antrean pemeriksaan yang sedang berjalan/menunggu. Dipakai tombol "Batalkan
// antrean": pekerjaan yang sudah terlanjur mulai tetap menyelesaikan panggilan
// yang sedang berjalan, tapi sisanya berhenti dan tidak diklaim ulang.
func (d *Deps) MediaSEORefreshCancel(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	tag, err := d.Pool.Exec(r.Context(), `
		UPDATE jobs
		SET status = 'failed', attempts = max_attempts,
		    last_error = 'Dibatalkan admin: antrean pemeriksaan dihentikan.',
		    locked_at = NULL, locked_by = NULL, updated_at = NOW()
		WHERE kind = 'media_seo_refresh' AND status IN ('pending','running')`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	cancelled := int(tag.RowsAffected())
	d.audit(r.Context(), user.ID, "media.seo_refresh_cancelled", "media", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cancelled": cancelled})
}

// MediaSEORefreshOne: POST /v1/platform/media/{mediaId}/seo-refresh — antrekan
// pemeriksaan DA/AI untuk SATU media (tombol "Cek DA & AI" per baris).
//
// Endpoint sinkron `POST /media/{mediaId}/seo` tetap ada untuk pemanggil yang
// memang menunggu hasilnya, tapi UI tidak boleh memakainya: satu pemeriksaan
// butuh 24-30 detik (pengukuran nyata) sementara klien memutus di 12 detik.
// Di sini pekerjaan dilempar ke job `media_seo_refresh`, jadi responsnya
// seketika dan tabel menyusul lewat indikator antrean.
func (d *Deps) MediaSEORefreshOne(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(), `
		SELECT m.id, COALESCE(m.website_url, '') AS website_url,
		       EXISTS (SELECT 1 FROM jobs j
		               WHERE j.kind = 'media_seo_refresh' AND j.status IN ('pending','running')
		                 AND j.payload->>'mediaId' = m.id) AS queued
		FROM media m WHERE m.id = $1 AND m.status <> 'hidden' LIMIT 1`, mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if media == nil {
		writeErr(w, d, errNotFound("Media tidak ditemukan"))
		return
	}
	row := ptRow(ptNormalizeMap(media))
	if strings.TrimSpace(row.strOr("website_url", "")) == "" {
		writeErr(w, d, errUnprocessable("Media belum memiliki website/domain yang bisa diperiksa."))
		return
	}
	if row.boolOr("queued", false) {
		writeJSON(w, http.StatusOK, map[string]any{"queued": false, "reason": "already_queued"})
		return
	}
	if _, jErr := d.mpEnqueueJob(r.Context(), "media_seo_refresh", map[string]any{"mediaId": mediaID}, 2); jErr != nil {
		writeErr(w, d, jErr)
		return
	}
	d.audit(r.Context(), user.ID, "media.seo_refresh_one", "media", mediaID)
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": true})
}

// ---------- POST /v1/platform/media/{mediaId}/geo-refresh ----------

// MediaGEORefreshOne: antrekan pemeriksaan GEO/AI Trust untuk SATU media
// (tombol "Cek GEO" per baris).
//
// Beda dari MediaSEORefreshOne: job `media_geo_refresh` hanya menghitung AI
// Trust (PageSpeed + schema.org) dan status kutipan, TANPA memanggil penyedia
// DA/PA berbayar. Dipakai admin yang cuma ingin menyegarkan kolom GEO/AI Trust.
func (d *Deps) MediaGEORefreshOne(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(), `
		SELECT m.id, COALESCE(m.website_url, '') AS website_url,
		       EXISTS (SELECT 1 FROM jobs j
		               WHERE j.kind = 'media_geo_refresh' AND j.status IN ('pending','running')
		                 AND j.payload->>'mediaId' = m.id) AS queued
		FROM media m WHERE m.id = $1 AND m.status <> 'hidden' LIMIT 1`, mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if media == nil {
		writeErr(w, d, errNotFound("Media tidak ditemukan"))
		return
	}
	row := ptRow(ptNormalizeMap(media))
	if strings.TrimSpace(row.strOr("website_url", "")) == "" {
		writeErr(w, d, errUnprocessable("Media belum memiliki website/domain yang bisa diperiksa."))
		return
	}
	if row.boolOr("queued", false) {
		writeJSON(w, http.StatusOK, map[string]any{"queued": false, "reason": "already_queued"})
		return
	}
	if _, jErr := d.mpEnqueueJob(r.Context(), "media_geo_refresh", map[string]any{"mediaId": mediaID}, 2); jErr != nil {
		writeErr(w, d, jErr)
		return
	}
	d.audit(r.Context(), user.ID, "media.geo_refresh_one", "media", mediaID)
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": true})
}

// ---------- GET /v1/platform/media/seo-refresh-status ----------

// MediaSEORefreshStatus: indikator antrean cek DA — berapa media yang masih
// perlu dicek, berapa job berjalan, dan (bila ada) pesan kegagalan terakhir
// supaya masalah kunci provider/rate limit langsung kelihatan.
func (d *Deps) MediaSEORefreshStatus(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	jobsRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) FILTER (WHERE status = 'pending') AS queued,
		       COUNT(*) FILTER (WHERE status = 'running') AS running,
		       COUNT(*) FILTER (WHERE status IN ('pending','running')) AS pending,
		       COUNT(*) FILTER (WHERE status = 'failed' AND updated_at > NOW() - INTERVAL '1 hour') AS failed
		FROM jobs WHERE kind = 'media_seo_refresh'`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	staleRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) AS total FROM media m
		LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
		WHERE`+mpStaleSeoWhere)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	lastErrRow, _ := d.queryRow(r.Context(), `
		SELECT last_error FROM jobs
		WHERE kind = 'media_seo_refresh' AND status = 'failed' AND last_error IS NOT NULL
		ORDER BY updated_at DESC LIMIT 1`)
	// Domain mana yang gagal (bukan hanya pesan terakhir): admin perlu tahu
	// media mana yang perlu dicek ulang. Diambil dari payload job (mediaId).
	failedRows, _ := d.queryRows(r.Context(), `
		SELECT COALESCE(m.name, '') AS media_name, COALESCE(m.website_url, '') AS website_url,
		       COALESCE(j.last_error, '') AS last_error, j.updated_at
		FROM jobs j
		LEFT JOIN media m ON m.id = (j.payload->>'mediaId')
		WHERE j.kind = 'media_seo_refresh' AND j.status = 'failed'
		  AND j.updated_at > NOW() - INTERVAL '1 hour'
		ORDER BY j.updated_at DESC LIMIT 10`)
	// Kegagalan penyedia DA/PA tingkat platform (langganan/kuota): tanpa ini
	// kolom DA tampak "kosong tanpa sebab" karena job tetap selesai berkat AI Trust.
	daErrRow, _ := d.queryRow(r.Context(), `
		SELECT last_da_error, last_da_error_at FROM platform_seo_config WHERE id = 'default' LIMIT 1`)
	pending, queued, running, failed, stale := 0, 0, 0, 0, 0
	if jobsRow != nil {
		pending = jobsRow.intOr("pending", 0)
		queued = jobsRow.intOr("queued", 0)
		running = jobsRow.intOr("running", 0)
		failed = jobsRow.intOr("failed", 0)
	}
	if staleRow != nil {
		stale = staleRow.intOr("total", 0)
	}
	lastError := ""
	if lastErrRow != nil {
		lastError = lastErrRow.strOr("last_error", "")
	}
	daError := ""
	var daErrorAt any
	if daErrRow != nil {
		daError = daErrRow.strOr("last_da_error", "")
		daErrorAt = daErrRow.anyOrNil("last_da_error_at")
	}
	failedItems := make([]map[string]any, 0, len(failedRows))
	for _, raw := range failedRows {
		row := ptRow(ptNormalizeMap(raw))
		failedItems = append(failedItems, map[string]any{
			"media_name":  row.strOr("media_name", ""),
			"website_url": row.strOr("website_url", ""),
			"last_error":  row.strOr("last_error", ""),
			"at":          row.anyOrNil("updated_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pending_jobs":     pending,
		"queued_jobs":      queued,
		"running_jobs":     running,
		"failed_last_hour": failed,
		"failed_items":     failedItems,
		"stale":            stale,
		"last_error":       lastError,
		"da_error":         daError,
		"da_error_at":      daErrorAt,
	})
}

// ---------- POST /v1/platform/media/similarweb-bulk ----------

// mpSimilarWebStaleWhere: media aktif yang trafiknya perlu diperiksa —
// belum pernah dicek ATAU snapshot terakhirnya basi (>cool down 6 bulan).
//
// Snapshot dengan 0 kunjungan dihitung "sudah dicek": provider menjawab 0
// untuk domain yang tidak terindeks, dan mengulanginya hanya membakar kuota.
// Harus sepadan dengan query sweep di brandflow/scheduler.go.
const mpSimilarWebStaleWhere = `
			media.status = 'active'
			  AND media.website_url IS NOT NULL AND media.website_url <> ''
			  AND NOT EXISTS (
			    SELECT 1 FROM media_similarweb_snapshots s
			    WHERE s.media_id = media.id
			      AND s.created_at >= NOW() - INTERVAL '6 months'
			      AND (s.insights->>'visits') IS NOT NULL
			  )`

// mpSimilarWebNeverCheckedWhere: media aktif yang BELUM PERNAH diperiksa
// SimilarWeb sama sekali — tidak punya satu pun snapshot.
//
// Dipakai tombol "Antrekan Cek Trafik Semua". Tombol ini TIDAK melihat kolom
// traffic_estimate: media yang angkanya diketik manual (mis. 150 rb) tetapi
// belum pernah lewat provider tetap diperiksa. Sebaliknya media yang sudah
// punya snapshot — termasuk hasil 0 kunjungan — dilewati, berapa pun umur
// datanya. Pemeriksaan ulang berkala (6 bulan) tetap urusan scheduler.
const mpSimilarWebNeverCheckedWhere = `
			media.status = 'active'
			  AND media.website_url IS NOT NULL AND media.website_url <> ''
			  AND NOT EXISTS (
			    SELECT 1 FROM media_similarweb_snapshots s
			    WHERE s.media_id = media.id
			  )`

// MediaSimilarWebBulk: POST /v1/platform/media/similarweb-bulk — tulis ulang
// jadi ANTREAN, bukan pemeriksaan inline. Dulu fungsi ini memanggil provider
// hingga `limit` media berurutan di satu request (tiap media bisa 15 detik),
// sehingga tombol "Cek Trafik Semua" selalu berakhir timeout proxy padahal
// backend masih memproses. Sekarang reply cepat: semua media kandidat masuk
// tabel jobs (kind media_similarweb_refresh) dan scheduler/worker yang
// melakukan panggilan provider satu per satu.
//
// Yang diantrekan hanya media yang BELUM PERNAH punya snapshot (lihat
// mpSimilarWebNeverCheckedWhere). Job gagal boleh dicoba lagi lewat tombol
// ini (dedupe hanya terhadap pending/running): klik adalah tindakan sadar
// admin, berbeda dari sweep otomatis yang punya jeda 1 jam.
func (d *Deps) MediaSimilarWebBulk(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if err := auth.CheckWriteRate(r.Context(), d.Pool, "media-similarweb-bulk:"+user.ID, 2); err != nil {
		writeErr(w, d, ptErrTooMany("Antrean cek trafik baru saja dijalankan. Tunggu satu menit lalu coba lagi."))
		return
	}

	stale, err := d.queryRows(r.Context(), `
		SELECT media.id, media.name
		FROM media
		WHERE`+mpSimilarWebNeverCheckedWhere+`
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'media_similarweb_refresh' AND j.status IN ('pending','running')
		      AND j.payload->>'mediaId' = media.id
		  )
		ORDER BY media.updated_at ASC
		LIMIT 200`)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	enqueued, skipped := 0, 0
	for _, raw := range stale {
		row := ptRow(ptNormalizeMap(raw))
		mediaID := row.strOr("id", "")
		if mediaID == "" {
			skipped++
			continue
		}
		if _, jErr := d.mpEnqueueJob(r.Context(), "media_similarweb_refresh", map[string]any{"mediaId": mediaID}, 2); jErr != nil {
			d.logWarn("enqueue media_similarweb_refresh gagal", mediaID, jErr)
			skipped++
			continue
		}
		enqueued++
	}

	totalRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS total FROM media WHERE"+mpSimilarWebNeverCheckedWhere)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	totalStale := 0
	if totalRow != nil {
		totalStale = totalRow.intOr("total", 0)
	}
	d.audit(r.Context(), user.ID, "media.similarweb_refresh_all", "media", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"enqueued":    enqueued,
		"skipped":     skipped,
		"candidates":  len(stale),
		"total_stale": totalStale,
	})
}

// ---------- GET /v1/platform/media/similarweb-status ----------

// MediaSimilarWebStatus: indikator antrean cek trafik — dipakai tombol admin
// untuk melihat sisa pekerjaan dan (bila ada) sebab kegagalannya, seperti
// indikator antrean cek DA. "stale" dihitung tanpa dedupe antrean supaya
// angkanya stabil terhadap jumlah pending.
func (d *Deps) MediaSimilarWebStatus(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	jobsRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) FILTER (WHERE status = 'pending') AS queued,
		       COUNT(*) FILTER (WHERE status = 'running') AS running,
		       COUNT(*) FILTER (WHERE status IN ('pending','running')) AS pending,
		       COUNT(*) FILTER (WHERE status = 'failed' AND updated_at > NOW() - INTERVAL '1 hour') AS failed
		FROM jobs WHERE kind = 'media_similarweb_refresh'`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	staleRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS total FROM media WHERE"+mpSimilarWebStaleWhere)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	lastErrRow, _ := d.queryRow(r.Context(), `
		SELECT last_error FROM jobs
		WHERE kind = 'media_similarweb_refresh' AND status = 'failed' AND last_error IS NOT NULL
		ORDER BY updated_at DESC LIMIT 1`)
	queued, running, pending, failed, stale := 0, 0, 0, 0, 0
	if jobsRow != nil {
		queued = jobsRow.intOr("queued", 0)
		running = jobsRow.intOr("running", 0)
		pending = jobsRow.intOr("pending", 0)
		failed = jobsRow.intOr("failed", 0)
	}
	if staleRow != nil {
		stale = staleRow.intOr("total", 0)
	}
	lastError := ""
	if lastErrRow != nil {
		lastError = lastErrRow.strOr("last_error", "")
	}
	// Kesiapan penyedia: UI memakai ini untuk membedakan "tidak ada yang perlu
	// dicek" dari "pemeriksaan otomatis berhenti karena kredensial kurang".
	// Tanpa penanda ini, admin hanya melihat angka 0 dan mengira sistem diam.
	ready := d.mpCheckProviderReadiness(r.Context())

	writeJSON(w, http.StatusOK, map[string]any{
		"pending_jobs":     pending,
		"queued_jobs":      queued,
		"running_jobs":     running,
		"failed_last_hour": failed,
		"stale":            stale,
		"last_error":       lastError,
		// auto_ready = apakah pemeriksaan otomatis bisa berjalan sekarang.
		"auto_ready":         ready.Traffic && ready.Seo,
		"traffic_ready":      ready.Traffic,
		"traffic_blocked_by": ready.TrafficReason,
		"seo_ready":          ready.Seo,
		"seo_blocked_by":     ready.SeoReason,
	})
}

// ---------- POST /v1/platform/media/sync-credit-cost ----------

func (d *Deps) MediaSyncCreditCost(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	rate := d.mpGetRateIDR(r.Context())
	// Formula sama persis dengan lib/pricing/engine (client price → ceil ÷ rate);
	// markup dibaca langsung dari pricing_config agar UPDATE massal mengevaluasi
	// seluruh baris dalam satu statement.
	markupExpr := "(SELECT COALESCE(markup_percent, 15) FROM pricing_config WHERE id = 1)"
	before, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) AS count FROM media
		WHERE price > 0 AND credit_cost <> GREATEST(1, CEIL(price::numeric * (1 + `+markupExpr+` / 100) / $1))::int`, rate)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	if err := d.exec(r.Context(), `
		UPDATE media SET
		  credit_cost = CASE WHEN price > 0 THEN GREATEST(1, CEIL(price::numeric * (1 + `+markupExpr+` / 100) / $2))::int ELSE 0 END,
		  updated_at = NOW()
		WHERE price > 0 AND credit_cost <> GREATEST(1, CEIL(price::numeric * (1 + `+markupExpr+` / 100) / $1))::int`,
		rate, rate); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	synced := 0
	if before != nil {
		synced = ptRow(ptNormalizeMap(before)).intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"synced": synced, "rate_idr": rate})
}
