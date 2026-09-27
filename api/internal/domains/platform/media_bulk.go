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
		  AND (sm.media_id IS NULL OR sm.checked_at IS NULL OR sm.checked_at < NOW() - INTERVAL '5 months')
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

// ---------- POST /v1/platform/media/similarweb-bulk ----------

func (d *Deps) MediaSimilarWebBulk(w http.ResponseWriter, r *http.Request) {
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
		WHERE media.status = 'active'
		  AND media.website_url IS NOT NULL AND media.website_url <> ''
		  AND NOT EXISTS (
		    SELECT 1 FROM media_similarweb_snapshots s
		    WHERE s.media_id = media.id
		      AND s.created_at >= NOW() - INTERVAL '2 months'
		      AND (s.insights->>'visits') IS NOT NULL
		      AND (s.insights->>'visits')::numeric > 0
		  )
		ORDER BY media.updated_at ASC
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
		insight, swErr := d.mpCheckSimilarWebInsights(r.Context(), mediaID, row.anyOrNil("website_url"), user.ID, false)
		if swErr == nil {
			detail := "fresh"
			if cached, isBool := insight["cached"].(bool); isBool && cached {
				detail = "cached"
			}
			results = append(results, map[string]any{
				"media_id": mediaID, "name": name, "ok": true, "visits": insight["visits"], "detail": detail,
			})
			okCount++
			continue
		}
		results = append(results, map[string]any{
			"media_id": mediaID, "name": name, "ok": false, "visits": nil, "detail": mpShortError(swErr, 160),
		})
		if mpStopStatuses[swErr.Status] {
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
