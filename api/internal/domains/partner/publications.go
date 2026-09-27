package partner

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- state machine publikasi (port lib/publication/state.ts) ----------

var publicationTransitions = map[string][]string{
	"pending":         {"queued", "waiting_partner", "cancelled"},
	"waiting_partner": {"partner_approved", "partner_rejected", "cancelled"},
	// partner_approved → published: alur Mode Manual mitra. Setelah approve,
	// mitra menempelkan URL artikel dan itulah pemicu publish (lihat
	// POST /partner/publications/{id}/url). Tabel transisi app lama hanya
	// memuat "queued" sehingga jalur ini selalu gagal dengan
	// "Publication status cannot transition from partner_approved to published".
	"partner_approved": {"queued", "published"},
	"partner_rejected": {"cancelled"},
	"queued":           {"processing", "cancelled"},
	"processing":       {"published", "failed", "retrying", "cancelled"},
	"published":        {},
	"failed":           {"queued", "retrying", "cancelled"},
	"retrying":         {"queued", "cancelled"},
	"cancelled":        {},
}

var publicationStatuses = []string{
	"pending", "waiting_partner", "partner_approved", "partner_rejected",
	"queued", "processing", "published", "failed", "retrying", "cancelled",
}

func validPublicationStatus(s string) bool {
	for _, v := range publicationStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// publicationStateError: parity PublicationStateError. Bukan Conflict, jadi
// naik sebagai 500 dengan pesan aslinya (di production di-scrub) — persis
// pemetaan errorResponse app lama.
type publicationStateError struct{ message string }

func (e *publicationStateError) Error() string { return e.message }

func assertTransition(current, next string) error {
	if current == next {
		return nil
	}
	for _, t := range publicationTransitions[current] {
		if t == next {
			return nil
		}
	}
	return &publicationStateError{"Publication status cannot transition from " + current + " to " + next}
}

// transitionPublication: port transitionPublication — UPDATE atomik dengan
// guard `WHERE status = current` supaya penulis bersamaan aman.
func (d *Deps) transitionPublication(ctx context.Context, publicationID, to string) error {
	row, err := d.queryRow(ctx, "SELECT status FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	if row == nil {
		return errNotFound("Publication not found")
	}
	current := row.strOr("status", "")
	if err := assertTransition(current, to); err != nil {
		return err
	}
	return d.exec(ctx,
		"UPDATE publications SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3",
		to, publicationID, current)
}

// transitionPublicationWith: varian dengan kolom tambahan (reviewed_by, dst).
func (d *Deps) transitionPublicationWith(ctx context.Context, publicationID, to string, extra map[string]any) error {
	row, err := d.queryRow(ctx, "SELECT status FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	if row == nil {
		return errNotFound("Publication not found")
	}
	current := row.strOr("status", "")
	if err := assertTransition(current, to); err != nil {
		return err
	}
	sets := []string{"status = $1"}
	args := []any{to}
	i := 2
	for col, val := range extra {
		sets = append(sets, col+" = $"+itoa(i))
		args = append(args, val)
		i++
	}
	args = append(args, publicationID, current)
	sql := "UPDATE publications SET " + strings.Join(sets, ", ") + ", updated_at = NOW() WHERE id = $" + itoa(i) + " AND status = $" + itoa(i+1)
	return d.exec(ctx, sql, args...)
}

// recordPublicationAttempt: port recordPublicationAttempt — attempt_number
// dihitung di dalam INSERT (MAX+1) supaya retry bersamaan tidak duplikat.
func (d *Deps) recordPublicationAttempt(ctx context.Context, publicationID, status string, errorCode, errorMessage *string, startedAt, completedAt time.Time) error {
	if err := d.exec(ctx,
		`INSERT INTO publication_attempts (id, publication_id, attempt_number, status, error_code, error_message, started_at, completed_at)
		 VALUES (gen_random_uuid(), $1, (SELECT COALESCE(MAX(attempt_number), 0) + 1 FROM publication_attempts WHERE publication_id = $1), $2, $3, $4, $5, $6)`,
		publicationID, status, errorCode, errorMessage, startedAt, completedAt); err != nil {
		return err
	}
	if status == "published" {
		d.publicationEventNotification(ctx, publicationID, "publication.published")
		d.enqueueJobBestEffort(ctx, "pr_analysis", map[string]any{"publicationId": publicationID}, 2)
		d.enqueueJobBestEffort(ctx, "pr_verify", map[string]any{"publicationId": publicationID}, 2)
		d.capturePublicationCredit(ctx, publicationID)
	}
	if status == "failed" {
		d.publicationEventNotification(ctx, publicationID, "publication.failed")
	}
	return nil
}

// enqueueJob: port enqueueJob — attempts default 3 untuk kind "*generation*", else 2.
func (d *Deps) enqueueJob(ctx context.Context, kind string, payload map[string]any, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		if strings.Contains(kind, "generation") {
			maxAttempts = 3
		} else {
			maxAttempts = 2
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var id string
	err = d.Pool.QueryRow(ctx,
		"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES (gen_random_uuid(), $1, $2::jsonb, 'pending', 0, $3, NOW()) RETURNING id",
		kind, string(raw), maxAttempts).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *Deps) enqueueJobBestEffort(ctx context.Context, kind string, payload map[string]any, maxAttempts int) {
	if _, err := d.enqueueJob(ctx, kind, payload, maxAttempts); err != nil {
		// Job bukan bagian dari kontrak respons — kegagalan antrean tidak boleh
		// menggagalkan transisi status yang sudah tersimpan.
		d.logger().Warn("enqueue job gagal", "kind", kind, "err", err)
	}
}

// partnerScopedPublication: port partnerScopedPublication (tenant scoping mitra).
func (d *Deps) partnerScopedPublication(ctx context.Context, partnerID, publicationID string) (rowMap, error) {
	row, err := d.queryRow(ctx, `
		SELECT p.*, m.name AS media_name, m.category AS media_category, m.partner_id AS media_partner_id,
		       m.website_url AS media_website_url, m.publication_mode AS media_publication_mode,
		       m.integration_type AS media_integration_type,
		       d.title AS document_title, d.slug AS document_slug,
		       b.client_name AS client_name, b.name AS brand_name, b.public_id AS brand_public_id,
		       pa.public_id AS partner_public_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN brands b ON b.id = p.brand_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.id = $1 AND m.partner_id = $2 LIMIT 1`, publicationID, partnerID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Publication not found")
	}
	return row, nil
}

// partnerPublicationJSON: port partnerPublicationJson — bentuk JSON kanonik
// yang dipakai semua endpoint publikasi mitra.
func partnerPublicationJSON(row rowMap) map[string]any {
	var edits map[string]any
	switch t := row.raw("partner_draft_edits").(type) {
	case map[string]any:
		edits = t
	case string:
		var m map[string]any
		if json.Unmarshal([]byte(t), &m) == nil {
			edits = m
		}
	}
	var editsThumbnail *string
	if edits != nil {
		if s, ok := edits["featured_image_url"].(string); ok && strings.TrimSpace(s) != "" {
			v := s
			editsThumbnail = &v
		}
	}
	featuredImageURL := row.strPtr("featured_image_url")
	thumbnail := editsThumbnail
	if thumbnail == nil {
		thumbnail = featuredImageURL
	}

	clientPrice := row.numOr("client_price")
	partnerPrice := clientPrice
	if v, ok := row.num("partner_price"); ok {
		partnerPrice = v
	}
	markupPercent := 0.0
	if v, ok := row.num("markup_percent"); ok {
		markupPercent = v
	}
	partnerAmount := splitRevenue(clientPrice, partnerPrice, 25)
	if v, ok := row.num("partner_amount"); ok {
		partnerAmount = v
	}

	return map[string]any{
		"id":                            row.strOr("id", ""),
		"brand_id":                      row.raw("brand_id"),
		"client_name":                   firstNonNil(row.raw("client_name"), row.raw("brand_name")),
		"document_id":                   row.raw("document_id"),
		"order_code":                    row.raw("order_code"),
		"document_title":                row.raw("document_title"),
		"document_slug":                 row.raw("document_slug"),
		"featured_image_url":            row.raw("featured_image_url"),
		"partner_draft_edits":           editsOrNil(edits),
		"thumbnail_url":                 ptrString(thumbnail),
		"media_id":                      row.raw("media_id"),
		"media_name":                    row.raw("media_name"),
		"media_category":                row.raw("media_category"),
		"media_integration_type":        row.raw("media_integration_type"),
		"media_website_url":             row.raw("media_website_url"),
		"media_publication_mode":        row.raw("media_publication_mode"),
		"status":                        row.raw("status"),
		"client_price":                  clientPrice,
		"partner_price":                 partnerPrice,
		"markup_percent":                markupPercent,
		"partner_amount":                partnerAmount,
		"publish_status":                row.raw("publish_status"),
		"scheduled_at":                  row.timePtr("scheduled_at"),
		"published_at":                  row.timePtr("published_at"),
		"published_url":                 row.raw("published_url"),
		"remote_url":                    row.raw("remote_url"),
		"rss_import_status":             row.strOr("rss_import_status", "not_imported"),
		"rss_imported_at":               row.timePtr("rss_imported_at"),
		"rss_published_at":              row.timePtr("rss_published_at"),
		"notes":                         row.raw("notes"),
		"rejection_reason":              row.raw("rejection_reason"),
		"reviewed_by":                   row.raw("reviewed_by"),
		"reviewed_at":                   row.timePtr("reviewed_at"),
		"publish_progress":              row.numPtr("publish_progress"),
		"verification_status":           row.raw("verification_status"),
		"verification_compliance_score": row.numPtr("verification_compliance_score"),
		"verification_error":            row.raw("verification_error"),
		"verification_checked_at":       row.timePtr("verification_checked_at"),
		"pr_sentiment":                  row.raw("pr_sentiment"),
		"pr_sentiment_confidence":       row.numPtr("pr_sentiment_confidence"),
		"pr_message_score":              row.numPtr("pr_message_score"),
		"pr_message_status":             row.raw("pr_message_status"),
		"created_at":                    row.timePtr("created_at"),
		"updated_at":                    row.timePtr("updated_at"),
	}
}

func firstNonNil(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func editsOrNil(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	return m
}

func ptrString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// ---------- GET /v1/partner/publications ----------

var publicationListStatuses = []string{
	"waiting_partner", "partner_approved", "partner_rejected", "queued", "processing", "published", "failed",
}

const publicationSelect = `SELECT p.*, m.name AS media_name, m.category AS media_category, m.partner_id AS media_partner_id,
              m.integration_type AS media_integration_type,
              m.website_url AS media_website_url,
              m.publication_mode AS media_publication_mode,
              d.title AS document_title, d.slug AS document_slug,
              b.client_name AS client_name, b.name AS brand_name,
              v.status AS verification_status, v.compliance_score AS verification_compliance_score,
              v.error_message AS verification_error, v.verified_at AS verification_checked_at,
              sa.sentiment AS pr_sentiment, sa.confidence AS pr_sentiment_confidence,
              ma.consistency_score AS pr_message_score, ma.status AS pr_message_status,
              (SELECT pj.progress FROM publication_jobs pj WHERE pj.publication_id = p.id ORDER BY pj.created_at DESC LIMIT 1) AS publish_progress,
              (SELECT pj.remote_url FROM publication_jobs pj WHERE pj.publication_id = p.id ORDER BY pj.created_at DESC LIMIT 1) AS remote_url,
              (SELECT pe.amount FROM partner_earnings pe WHERE pe.publication_id = p.id LIMIT 1) AS partner_amount
       FROM publications p
       JOIN media m ON m.id = p.media_id
       JOIN content_documents d ON d.id = p.document_id
       LEFT JOIN brands b ON b.id = p.brand_id
       LEFT JOIN pr_publication_verifications v ON v.publication_id = p.id
       LEFT JOIN pr_sentiment_analyses sa ON sa.publication_id = p.id
       LEFT JOIN pr_message_analyses ma ON ma.publication_id = p.id`

func (d *Deps) PublicationsList(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	requested := []string{}
	if raw := strings.TrimSpace(q.Get("status")); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if !validPublicationStatus(s) {
				httpx.Error(w, http.StatusUnprocessableEntity, "Invalid publication status filter")
				return
			}
			requested = append(requested, s)
		}
	}
	statuses := requested
	if len(statuses) == 0 {
		statuses = publicationListStatuses
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	args := []any{partner.ID}
	ph := make([]string, len(statuses))
	for i, s := range statuses {
		args = append(args, s)
		ph[i] = "$" + itoa(i+2)
	}
	inClause := strings.Join(ph, ",")

	countRows, err := d.queryRow(r.Context(),
		`SELECT COUNT(*) AS count FROM publications p JOIN media m ON m.id = p.media_id
		 WHERE m.partner_id = $1 AND p.status IN (`+inClause+`)`, args...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	total := countRows.intOr("count", 0)

	if q.Get("format") == "csv" {
		all, err := d.queryRows(r.Context(), publicationSelect+" WHERE m.partner_id = $1 AND p.status IN ("+inClause+") ORDER BY p.created_at DESC LIMIT 5000", args...)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		body := publicationsCSV(all)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="publikasi-mitra-`+time.Now().UTC().Format("2006-01-02")+`.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
		return
	}

	pageArgs := append(append([]any{}, args...), limit, offset)
	items, err := d.queryRows(r.Context(),
		publicationSelect+" WHERE m.partner_id = $1 AND p.status IN ("+inClause+") ORDER BY p.created_at DESC LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2),
		pageArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	publications := make([]map[string]any, 0, len(items))
	for _, item := range items {
		pub := partnerPublicationJSON(item)
		thumb, _ := pub["thumbnail_url"].(string)
		pub["thumbnail_size_bytes"] = thumbnailSizeBytes(thumb)
		publications = append(publications, pub)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"publications": publications,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

// publicationsCSV: parity publicationsCsv — BOM + header Indonesia.
func publicationsCSV(items []rowMap) string {
	header := []string{"No. Order", "Status", "Klien", "Artikel", "Media", "Kategori Media", "Integrasi", "Harga Klien", "Jadwal", "Terbit", "URL Terbit", "Status Verifikasi", "Compliance", "Sentiment", "Confidence", "Message Score", "Catatan", "Alasan Penolakan", "Dibuat"}
	lines := []string{csvLine(header)}
	for _, row := range items {
		lines = append(lines, csvLine([]string{
			row.strOr("order_code", row.strOr("id", "")),
			row.strOr("status", ""),
			row.strOr("client_name", ""),
			row.strOr("document_title", ""),
			row.strOr("media_name", ""),
			row.strOr("media_category", ""),
			row.strOr("media_integration_type", ""),
			formatNum(row.numOr("client_price")),
			timeOrEmpty(row.timePtr("scheduled_at")),
			timeOrEmpty(row.timePtr("published_at")),
			row.strOr("published_url", ""),
			row.strOr("verification_status", ""),
			formatNumOrEmpty(row.numPtr("verification_compliance_score")),
			row.strOr("pr_sentiment", ""),
			formatNumOrEmpty(row.numPtr("pr_sentiment_confidence")),
			formatNumOrEmpty(row.numPtr("pr_message_score")),
			row.strOr("notes", ""),
			row.strOr("rejection_reason", ""),
			timeOrEmpty(row.timePtr("created_at")),
		}))
	}
	return "\uFEFF" + strings.Join(lines, "\r\n")
}

func csvLine(cells []string) string {
	escaped := make([]string, len(cells))
	for i, c := range cells {
		escaped[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
	}
	return strings.Join(escaped, ",")
}

func timeOrEmpty(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func formatNum(v float64) string { return strconvFormatFloat(v) }

func formatNumOrEmpty(v any) string {
	f, ok := v.(float64)
	if !ok {
		return ""
	}
	return strconvFormatFloat(f)
}

// ---------- GET /v1/partner/publications/{publicationId} ----------

func (d *Deps) PublicationGet(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	document, _ := d.queryRow(r.Context(),
		"SELECT title, slug, meta_title, meta_description, status, current_version FROM content_documents WHERE id = $1 LIMIT 1",
		publication.raw("document_id"))
	media, _ := d.queryRow(r.Context(),
		"SELECT website_url, integration_type FROM media WHERE id = $1 LIMIT 1", publication.raw("media_id"))
	attempts, _ := d.queryRows(r.Context(),
		"SELECT attempt_number, status, error_code, error_message, started_at, completed_at FROM publication_attempts WHERE publication_id = $1 ORDER BY attempt_number ASC",
		publicationID)
	job, _ := d.queryRow(r.Context(),
		"SELECT status, remote_url, remote_post_id, error_message, progress, created_at FROM publication_jobs WHERE publication_id = $1 ORDER BY created_at DESC LIMIT 1",
		publicationID)
	verification, _ := d.queryRow(r.Context(),
		"SELECT status, compliance_score, error_message, verified_at FROM pr_publication_verifications WHERE publication_id = $1 ORDER BY verified_at DESC LIMIT 1",
		publicationID)
	sentiment, _ := d.queryRow(r.Context(),
		"SELECT sentiment, confidence, status, error_message FROM pr_sentiment_analyses WHERE publication_id = $1 LIMIT 1",
		publicationID)
	message, _ := d.queryRow(r.Context(),
		"SELECT consistency_score, status, error_message FROM pr_message_analyses WHERE publication_id = $1 LIMIT 1",
		publicationID)

	payload := partnerPublicationJSON(publication)
	payload["document"] = documentJSON(document)
	payload["media_website_url"] = media.raw("website_url")
	payload["media_integration_type"] = media.raw("integration_type")
	attemptList := make([]map[string]any, 0, len(attempts))
	for _, a := range attempts {
		attemptList = append(attemptList, map[string]any{
			"attempt_number": a.intOr("attempt_number", 0),
			"status":         a.raw("status"),
			"error_code":     a.raw("error_code"),
			"error_message":  a.raw("error_message"),
			"started_at":     a.timePtr("started_at"),
			"completed_at":   a.timePtr("completed_at"),
		})
	}
	payload["attempts"] = attemptList
	payload["job"] = jobJSON(job)
	payload["verification"] = verificationJSON(verification)
	payload["sentiment_analysis"] = sentimentJSON(sentiment)
	payload["message_analysis"] = messageJSON(message)
	writeJSON(w, http.StatusOK, payload)
}

func documentJSON(row rowMap) any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"title":            row.raw("title"),
		"slug":             row.raw("slug"),
		"meta_title":       row.raw("meta_title"),
		"meta_description": row.raw("meta_description"),
		"status":           row.raw("status"),
		"current_version":  row.raw("current_version"),
	}
}

func jobJSON(row rowMap) any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"status":         row.raw("status"),
		"remote_url":     row.raw("remote_url"),
		"remote_post_id": row.raw("remote_post_id"),
		"progress":       row.numPtr("progress"),
		"error_message":  row.raw("error_message"),
		"created_at":     row.timePtr("created_at"),
	}
}

func verificationJSON(row rowMap) any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"status":           row.raw("status"),
		"compliance_score": row.numPtr("compliance_score"),
		"error_message":    row.raw("error_message"),
		"verified_at":      row.timePtr("verified_at"),
	}
}

func sentimentJSON(row rowMap) any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"sentiment":     row.strOr("sentiment", "neutral"),
		"confidence":    row.numPtr("confidence"),
		"status":        row.strOr("status", "completed"),
		"error_message": row.raw("error_message"),
	}
}

func messageJSON(row rowMap) any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"consistency_score": row.numPtr("consistency_score"),
		"status":            row.strOr("status", "completed"),
		"error_message":     row.raw("error_message"),
	}
}

// ---------- POST approve / reject / url ----------

func (d *Deps) PublicationApprove(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if err := d.checkAuthRateReq(r, "partner-approve:"+user.ID, 30); err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	publishStatus := ""
	if s, present := strField(body, "publish_status"); present {
		if s != "draft" && s != "publish" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid publish_status")
			return
		}
		publishStatus = s
	}

	if err := assertTransition(publication.strOr("status", ""), "partner_approved"); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.transitionPublicationWith(r.Context(), publicationID, "partner_approved", map[string]any{
		"reviewed_by": user.ID, "reviewed_at": time.Now(),
	}); err != nil {
		d.writeErr(w, err)
		return
	}
	if publishStatus != "" {
		if err := d.exec(r.Context(),
			"UPDATE publications SET publish_status = $1, updated_at = NOW() WHERE id = $2 AND status = 'partner_approved'",
			publishStatus, publicationID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	d.auditLog(r.Context(), user.ID, "publication.partner_approved", "publication", publicationID)
	d.publicationEventNotification(r.Context(), publicationID, "publication.partner_approved")

	// Mode Manual: berhenti di partner_approved sampai mitra mengirim URL.
	// Mode Integrasi: lanjut ke antrean push.
	manualMode := !strings.EqualFold(strings.TrimSpace(stringOrEmpty(publication.raw("media_publication_mode"))), "publish")
	if !manualMode {
		if err := d.transitionPublication(r.Context(), publicationID, "queued"); err != nil {
			d.writeErr(w, err)
			return
		}
		row, _ := d.queryRow(r.Context(), "SELECT brand_id, scheduled_at FROM publications WHERE id = $1 LIMIT 1", publicationID)
		brandID := ""
		var scheduledAt *time.Time
		if row != nil {
			brandID = row.strOr("brand_id", "")
			if t, ok := row.raw("scheduled_at").(time.Time); ok {
				scheduledAt = &t
			}
		}
		d.enqueuePublicationExecution(r.Context(), publicationID, brandID, scheduledAt)
	}
	updated, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, partnerPublicationJSON(updated))
}

func (d *Deps) PublicationReject(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if err := d.checkAuthRateReq(r, "partner-reject:"+user.ID, 30); err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	body := decodeJSON(r)
	reason, _ := strField(body, "reason")
	if len(reason) < 5 || len(reason) > 500 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 5 character(s)")
		return
	}
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if err := assertTransition(publication.strOr("status", ""), "partner_rejected"); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.transitionPublicationWith(r.Context(), publicationID, "partner_rejected", map[string]any{
		"rejection_reason": reason, "reviewed_by": user.ID, "reviewed_at": time.Now(),
	}); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "publication.partner_rejected", "publication", publicationID)
	// Tulisio Credit: kembalikan TC yang ter-reserve ke dompet klien
	// (no-op bila brand tidak punya dompet / tidak ada reservasi).
	d.releaseCredit(r.Context(), publication.strOr("brand_id", ""), publicationID, user.ID)
	d.publicationEventNotification(r.Context(), publicationID, "publication.partner_rejected")
	updated, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, partnerPublicationJSON(updated))
}

func (d *Deps) PublicationURL(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	publishedURL, present := strField(body, "published_url")
	if !present || !isHTTPURL(publishedURL) || len(publishedURL) > 1000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "published_url harus URL http(s) yang valid")
		return
	}
	status := publication.strOr("status", "")
	if status != "published" && status != "partner_approved" {
		httpx.Error(w, http.StatusConflict, "URL hanya dapat dikirim untuk publikasi berstatus published atau partner_approved (mode manual)")
		return
	}
	isManualPublish := status == "partner_approved"
	wasDraft := publication.strOr("publish_status", "draft") == "draft"

	if err := d.exec(r.Context(), "UPDATE publications SET published_url = $1, updated_at = NOW() WHERE id = $2", publishedURL, publicationID); err != nil {
		d.writeErr(w, err)
		return
	}
	if wasDraft {
		if err := d.exec(r.Context(), "UPDATE publications SET publish_status = 'publish', updated_at = NOW() WHERE id = $1", publicationID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	if isManualPublish {
		if err := d.transitionPublication(r.Context(), publicationID, "published"); err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLog(r.Context(), user.ID, "publication.published_from_url", "publication", publicationID)
		now := time.Now()
		if err := d.recordPublicationAttempt(r.Context(), publicationID, "published", nil, nil, now, now); err != nil {
			d.writeErr(w, err)
			return
		}
	} else {
		d.enqueueJobBestEffort(r.Context(), "pr_verify", map[string]any{"publicationId": publicationID}, 2)
	}

	updated, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	payload := partnerPublicationJSON(updated)
	payload["verification_requeued"] = true
	writeJSON(w, http.StatusOK, payload)
}

// enqueuePublicationExecution: parity enqueuePublicationExecution (attempts 3).
func (d *Deps) enqueuePublicationExecution(ctx context.Context, publicationID, brandID string, scheduledAt *time.Time) {
	payload := map[string]any{"publicationId": publicationID, "brandId": brandID}
	if _, err := d.enqueueJobAt(ctx, "publication_execution", payload, scheduledAt, 3); err != nil {
		d.logger().Warn("enqueue publication_execution gagal", "err", err)
	}
}

func (d *Deps) enqueueJobAt(ctx context.Context, kind string, payload map[string]any, runAt *time.Time, maxAttempts int) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if runAt == nil {
		now := time.Now()
		runAt = &now
	}
	var id string
	err = d.Pool.QueryRow(ctx,
		"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES (gen_random_uuid(), $1, $2::jsonb, 'pending', 0, $3, $4) RETURNING id",
		kind, string(raw), maxAttempts, *runAt).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

var _ = pgx.ErrNoRows
