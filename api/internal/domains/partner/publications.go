package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
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
	"published":        {"revision"},
	"revision":         {"queued", "cancelled"},
	"failed":           {"queued", "retrying", "cancelled"},
	"retrying":         {"queued", "cancelled"},
	"cancelled":        {},
}

var publicationStatuses = []string{
	"pending", "waiting_partner", "partner_approved", "partner_rejected",
	"queued", "processing", "published", "failed", "retrying", "cancelled",
	"revision",
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

// enqueueSeoRefreshForMedia: pengajuan media langsung dicek DA/AI Trust-nya,
// tidak menunggu sweep scheduler. Dilewati bila website_url kosong (job-nya
// pasti gagal), skornya sudah ada, atau sudah ada job pending/running. Media
// yang sudah berskor tidak diperiksa ulang otomatis; yang masih kosong dicoba
// lagi paling cepat sekali per hari.
func (d *Deps) enqueueSeoRefreshForMedia(ctx context.Context, mediaID string) {
	if strings.TrimSpace(mediaID) == "" {
		return
	}
	var websiteURL string
	var skip bool
	err := d.Pool.QueryRow(ctx, `
		SELECT COALESCE(m.website_url, ''),
		       (EXISTS (SELECT 1 FROM jobs j
		                WHERE j.kind = 'media_seo_refresh' AND j.status IN ('pending','running')
		                  AND j.payload->>'mediaId' = m.id)
		        OR EXISTS (SELECT 1 FROM media_seo_metrics s
		                   WHERE s.media_id = m.id
		                     AND ((s.da IS NOT NULL AND s.ai_trust_score IS NOT NULL)
		                          OR s.checked_at > NOW() - INTERVAL '1 day')))
		FROM media m WHERE m.id = $1`, mediaID).Scan(&websiteURL, &skip)
	if err != nil || strings.TrimSpace(websiteURL) == "" || skip {
		return
	}
	d.enqueueJobBestEffort(ctx, "media_seo_refresh", map[string]any{"mediaId": mediaID}, 2)
}

// partnerScopedPublication: port partnerScopedPublication (tenant scoping mitra).
func (d *Deps) partnerScopedPublication(ctx context.Context, partnerID, publicationID string) (rowMap, error) {
	row, err := d.queryRow(ctx, `
		SELECT p.*, m.name AS media_name, m.category AS media_category, COALESCE(p.category_override, '') AS category_override, m.partner_id AS media_partner_id,
		       m.website_url AS media_website_url, m.publication_mode AS media_publication_mode,
		       m.integration_type AS media_integration_type,
		       m.auto_publish AS media_auto_publish,
		       m.media_type AS media_type,
		       m.allow_embed AS media_allow_embed,
		       d.title AS document_title, d.slug AS document_slug,
		       b.client_name AS client_name, b.name AS brand_name, b.public_id AS brand_public_id,
		       pa.public_id AS partner_public_id,
		       (SELECT pe.amount FROM partner_earnings pe WHERE pe.publication_id = p.id LIMIT 1) AS partner_amount,
		       (SELECT pe.gross_amount FROM partner_earnings pe WHERE pe.publication_id = p.id LIMIT 1) AS partner_amount_gross
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
	// Belum ada baris ledger (publikasi masih menunggu persetujuan mitra):
	// nominal yang ditampilkan harus sudah dipotong platform, sama seperti yang
	// nanti ditulis ke partner_earnings saat publikasi terbit. Tanpa ini mitra
	// melihat harga penuh (Rp 10.000) padahal yang masuk saldo Rp 7.500.
	grossAmount := splitRevenue(clientPrice, partnerPrice, pePlatformCutPercent)
	partnerAmount := partnerNetAfterPlatformCut(grossAmount)
	if v, ok := row.num("partner_amount"); ok {
		partnerAmount = v
		// gross = angka sebelum potongan platform; amount = yang diterima mitra.
		// Baris lama tanpa gross_amount jatuh kembali ke amount (potongan 0)
		// supaya rincian tetap masuk akal tanpa mengubah saldo historis.
		grossAmount = partnerAmount
		if g, okGross := row.num("partner_amount_gross"); okGross {
			grossAmount = g
		}
	}

	return map[string]any{
		"id":                            row.strOr("id", ""),
		"brand_id":                      row.raw("brand_id"),
		"client_name":                   firstNonNil(row.raw("client_name"), row.raw("brand_name")),
		"document_id":                   row.raw("document_id"),
		"order_code":                    row.raw("order_code"),
		"document_title":                row.raw("document_title"),
		"document_slug":                 row.raw("document_slug"),
		"featured_image_url":            relSelfURL(row.strOr("featured_image_url", "")),
		"partner_draft_edits":           editsOrNil(edits),
		"thumbnail_url":                 ptrString(relSelfURLN(thumbnail)),
		"media_id":                      row.raw("media_id"),
		"media_name":                    row.raw("media_name"),
		"media_integration_type":        row.raw("media_integration_type"),
		"media_auto_publish":            row.raw("media_auto_publish"),
		"media_type":                    row.raw("media_type"),
		"media_allow_embed":             row.boolOr("media_allow_embed", false),
		"media_website_url":             row.raw("media_website_url"),
		"media_publication_mode":        row.raw("media_publication_mode"),
		"media_category":                draftCategory(row),
		"category_override":             row.strOr("category_override", ""),
		"status":                        row.raw("status"),
		"client_price":                  clientPrice,
		"partner_price":                 partnerPrice,
		"markup_percent":                markupPercent,
		"partner_amount":                partnerAmount,
		"partner_amount_gross":          grossAmount,
		"partner_cut_percent":           pePlatformCutPercentFor(partnerAmount, grossAmount),
		"publish_status":                row.raw("publish_status"),
		"scheduled_at":                  row.timePtr("scheduled_at"),
		"published_at":                  row.timePtr("published_at"),
		"published_url":                 row.raw("published_url"),
		"remote_url":                    row.raw("remote_url"),
		"rss_import_status":             row.strOr("rss_import_status", "not_imported"),
		"rss_imported_at":               row.timePtr("rss_imported_at"),
		"rss_published_at":              row.timePtr("rss_published_at"),
		"notes":                         row.raw("notes"),
		"rewrite_ai_count":              row.intOr("rewrite_ai_count", 0),
		"rewrite_remain":                maxIntP(0, rewriteMaxPerPublicationPublic-row.intOr("rewrite_ai_count", 0)),
		"rejection_reason":              row.raw("rejection_reason"),
		"cancel_reason":                 row.raw("cancel_reason"),
		"revision_reject_count":         row.intOr("revision_reject_count", 0),
		"revision_reject_limit":         revisionRejectLimit,
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

// publicationListStatuses: status untuk tab "Semua" (tanpa filter). Harus
// memuat SEMUA status — sebelumnya `cancelled` (dan pending/retrying) tidak
// ikut, sehingga pesanan yang dibatalkan otomatis oleh SLA hilang dari daftar
// mitra dan alasannya tidak pernah terlihat.
var publicationListStatuses = publicationStatuses

// revisionRejectLimit: mitra boleh menolak revisi klien maksimal 3 kali. Setelah
// itu hanya tombol Terima yang tersisa dan admin ikut diberi tahu.
const revisionRejectLimit = 3

const publicationSelect = `SELECT p.*, m.name AS media_name, m.category AS media_category, COALESCE(p.category_override, '') AS category_override, m.partner_id AS media_partner_id,
              m.integration_type AS media_integration_type,
              m.media_type AS media_type,
              m.allow_embed AS media_allow_embed,
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
              (SELECT pe.amount FROM partner_earnings pe WHERE pe.publication_id = p.id LIMIT 1) AS partner_amount,
              (SELECT pe.gross_amount FROM partner_earnings pe WHERE pe.publication_id = p.id LIMIT 1) AS partner_amount_gross
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
	// media (id) — deep-link dari lencana "N menunggu" Daftar Media. id media
	// adalah char(36): nilai aneh hanya menghasilkan 0 baris (aman, tanpa cast).
	mediaFilter := strings.TrimSpace(q.Get("media"))

	args := []any{partner.ID}
	ph := make([]string, len(statuses))
	for i, s := range statuses {
		args = append(args, s)
		ph[i] = "$" + itoa(i+2)
	}
	inClause := strings.Join(ph, ",")
	mediaClause := ""
	if mediaFilter != "" {
		args = append(args, mediaFilter)
		mediaClause = " AND m.id = $" + itoa(len(args))
	}

	countRows, err := d.queryRow(r.Context(),
		`SELECT COUNT(*) AS count FROM publications p JOIN media m ON m.id = p.media_id
		 WHERE m.partner_id = $1 AND p.status IN (`+inClause+`)`+mediaClause, args...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	total := countRows.intOr("count", 0)

	if q.Get("format") == "csv" {
		all, err := d.queryRows(r.Context(), publicationSelect+" WHERE m.partner_id = $1 AND p.status IN ("+inClause+")"+mediaClause+" ORDER BY p.created_at DESC LIMIT 5000", args...)
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
		publicationSelect+" WHERE m.partner_id = $1 AND p.status IN ("+inClause+")"+mediaClause+" ORDER BY p.created_at DESC LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2),
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
		`SELECT title, slug, meta_title, meta_description, status, current_version,
		        COALESCE((SELECT cv.content FROM content_versions cv
		                  WHERE cv.document_id = content_documents.id AND cv.version = content_documents.current_version
		                  LIMIT 1), '') AS content_html
		 FROM content_documents WHERE id = $1 LIMIT 1`,
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
	// Komentar revisi dari klien (review_comments belum pernah dibaca paket ini).
	// Tanpa ini mitra tidak tahu apa yang harus diubah saat status `revision`.
	revisionComments, _ := d.queryRows(r.Context(),
		`SELECT id, body, attachment_url, attachment_name, created_at
		 FROM review_comments
		 WHERE document_id = $1 AND resolved = FALSE
		 ORDER BY created_at DESC LIMIT 20`, publication.raw("document_id"))

	// Pemeriksaan konten otomatis (Gatekeeper) untuk artikel yang akan tayang.
	// Publikasi dari kampanye Taut Pintar punya audit per (campaign, media, slot);
	// publikasi manual tidak punya audit — status "unchecked" berarti mitra perlu
	// membaca sendiri lewat pratinjau.
	contentCheck := map[string]any{"status": "unchecked"}
	if checkRow, _ := d.queryRow(r.Context(),
		`SELECT (g.checks->'contentSafety')::text AS check_json, g.created_at AS checked_at
		 FROM taut_campaign_media tcm
		 JOIN campaigns c ON c.id = tcm.campaign_id
		 JOIN taut_gatekeeper_audits g
		   ON g.campaign_id = tcm.campaign_id AND g.media_id = tcm.media_id AND g.slot = tcm.slot
		 WHERE tcm.article_document_id = $1 AND tcm.media_id = $2 AND c.brand_id = $3
		 ORDER BY g.created_at DESC
		 LIMIT 1`,
		publication.raw("document_id"), publication.raw("media_id"), publication.raw("brand_id")); checkRow != nil {
		var check map[string]any
		if jsonUnmarshal(checkRow.strOr("check_json", ""), &check) == nil && check != nil {
			check["checked_at"] = checkRow.timePtr("checked_at")
			contentCheck = check
		}
	}

	payload := partnerPublicationJSON(publication)
	payload["document"] = documentJSON(document)
	payload["content_check"] = contentCheck
	payload["media_website_url"] = media.raw("website_url")
	payload["media_integration_type"] = media.raw("integration_type")
	// publications tidak punya kolom remote_url — URL hasil integrasi hidup di
	// publication_jobs. Tanpa ini tautan "URL Terbit" kosong saat published_url
	// belum diisi mitra.
	if payload["remote_url"] == nil {
		payload["remote_url"] = job.raw("remote_url")
	}
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
	commentList := make([]map[string]any, 0, len(revisionComments))
	for _, c := range revisionComments {
		commentList = append(commentList, map[string]any{
			"id":              c.raw("id"),
			"body":            c.strOr("body", ""),
			"attachment_url":  c.raw("attachment_url"),
			"attachment_name": c.raw("attachment_name"),
			"created_at":      c.timePtr("created_at"),
		})
	}
	payload["revision_comments"] = commentList
	writeJSON(w, http.StatusOK, payload)
}

// publicationExcerptMaxWords: batas kata ringkasan otomatis dari isi artikel.
// Dipakai hanya bila klien tidak mengisi meta deskripsi.
const publicationExcerptMaxWords = 17

var (
	reExcerptDropBlocks = regexp.MustCompile(`(?is)<script[^>]*>.*?</script\s*>|<style[^>]*>.*?</style\s*>|<noscript[^>]*>.*?</noscript\s*>|<svg[^>]*>.*?</svg\s*>|<iframe[^>]*>.*?</iframe\s*>`)
	reExcerptTags       = regexp.MustCompile(`(?s)<[^>]+>`)
	reExcerptSpace      = regexp.MustCompile(`\s+`)
)

// excerptFromHTML: teks polos awal artikel, dibatasi jumlah kata supaya rapi
// dipakai sebagai pengganti meta deskripsi yang kosong.
func excerptFromHTML(raw string, maxWords int) string {
	if maxWords <= 0 {
		return ""
	}
	s := plainTextFromHTML(raw)
	if s == "" {
		return ""
	}
	words := strings.Fields(s)
	if len(words) > maxWords {
		return strings.Join(words[:maxWords], " ") + "…"
	}
	return s
}

// plainTextFromHTML: buang blok non-teks & tag, sisakan teks yang dibaca
// pembaca. Dipakai pratinjau mitra untuk menghitung panjang artikel tanpa
// membawa string HTML ke tempat lain.
func plainTextFromHTML(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	s := reExcerptDropBlocks.ReplaceAllString(raw, " ")
	s = reExcerptTags.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(reExcerptSpace.ReplaceAllString(s, " "))
}

func documentJSON(row rowMap) any {
	if row == nil {
		return nil
	}
	body := row.strOr("content_html", "")
	return map[string]any{
		"title":            row.raw("title"),
		"slug":             row.raw("slug"),
		"meta_title":       row.raw("meta_title"),
		"meta_description": row.raw("meta_description"),
		"excerpt":          excerptFromHTML(body, publicationExcerptMaxWords),
		"status":           row.raw("status"),
		"current_version":  row.raw("current_version"),
		// Isi penuh hanya dikirim di endpoint detail (GET satu publikasi) supaya
		// mitra bisa membaca artikel yang dijadwalkan sebelum terbit.
		"content_html": body,
		"word_count":   len(strings.Fields(plainTextFromHTML(body))),
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

// ---------- POST revision accept / reject ----------

// PublicationRevisionAccept: mitra menyetujui revisi klien. Publikasi
// diantrekan ulang supaya konten (dan cover) versi baru benar-benar dikirim ke
// CMS — worker berhenti setelah publish pertama, jadi tanpa antre ulang ini
// artikel lama tetap tayang.
func (d *Deps) PublicationRevisionAccept(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if err := d.checkAuthRateReq(r, "partner-revision-accept:"+user.ID, 30); err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	current := publication.strOr("status", "")
	if current != "revision" {
		httpx.Error(w, http.StatusConflict, "Publication status cannot transition from "+current+" to queued")
		return
	}
	if err := d.transitionPublicationWith(r.Context(), publicationID, "queued", map[string]any{
		"reviewed_by": user.ID, "reviewed_at": time.Now(), "revision_reject_count": 0,
	}); err != nil {
		d.writeErr(w, err)
		return
	}
	// Komentar revisi dianggap selesai; kalau klien minta revisi lagi, entri baru
	// yang muncul di daftar mitra.
	if documentID := publication.strOr("document_id", ""); documentID != "" {
		if err := d.exec(r.Context(),
			"UPDATE review_comments SET resolved = TRUE WHERE document_id = $1 AND resolved = FALSE",
			documentID); err != nil {
			d.logger().Warn("gagal menandai komentar revisi selesai", "publication", publicationID, "err", err)
		}
	}
	d.auditLog(r.Context(), user.ID, "publication.revision_accepted", "publication", publicationID)

	// Worker memperlakukan pengiriman yang sudah 'completed' (atau sudah punya
	// remote_post_id) sebagai selesai dan tidak menyentuh CMS lagi, jadi penanda
	// pengiriman lama dibersihkan supaya versi revisi benar-benar dikirim.
	// Cache 'resolved' ikut dibuang agar cover/kategori diunggah ulang; WordPress
	// mencocokkan artikel lewat slug, jadi tidak menghasilkan artikel duplikat.
	if err := d.exec(r.Context(),
		`UPDATE publication_jobs
		 SET status = 'queued', remote_post_id = NULL, remote_url = NULL, progress = 0,
		     error_message = NULL, payload_snapshot = payload_snapshot - 'resolved'
		 WHERE publication_id = $1`, publicationID); err != nil {
		d.logger().Warn("gagal mereset pengiriman untuk revisi", "publication", publicationID, "err", err)
	}

	row, _ := d.queryRow(r.Context(), "SELECT brand_id, scheduled_at FROM publications WHERE id = $1 LIMIT 1", publicationID)
	brandID := ""
	var scheduledAt *time.Time
	if row != nil {
		brandID = row.strOr("brand_id", "")
		if t, isTime := row.raw("scheduled_at").(time.Time); isTime {
			scheduledAt = &t
		}
	}
	d.enqueuePublicationExecution(r.Context(), publicationID, brandID, scheduledAt)
	d.revisionEventNotification(r.Context(), publicationID, "publication.revision_accepted")

	updated, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, partnerPublicationJSON(updated))
}

// PublicationRevisionReject: mitra menolak revisi dan menjelaskan alasannya.
// Status tetap `revision` supaya klien bisa memperbaiki lalu mengirim ulang.
func (d *Deps) PublicationRevisionReject(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if err := d.checkAuthRateReq(r, "partner-revision-reject:"+user.ID, 30); err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	body := decodeJSON(r)
	reason, _ := strField(body, "reason")
	if runes := []rune(reason); len(runes) < 5 || len(runes) > 500 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 5 character(s)")
		return
	}
	publication, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if publication.strOr("status", "") != "revision" {
		httpx.Error(w, http.StatusConflict, "Publication status cannot transition to revision")
		return
	}
	rejects := publication.intOr("revision_reject_count", 0)
	if rejects >= revisionRejectLimit {
		httpx.Error(w, http.StatusConflict, "Batas penolakan revisi sudah tercapai. Publikasi ini hanya bisa diterima.")
		return
	}
	rejects++
	if err := d.transitionPublicationWith(r.Context(), publicationID, "revision", map[string]any{
		"reviewed_by": user.ID, "reviewed_at": time.Now(), "revision_reject_count": rejects,
	}); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		`INSERT INTO review_comments (id, document_id, version, body, resolved, created_by)
		 VALUES (gen_random_uuid(), $1, COALESCE((SELECT current_version FROM content_documents WHERE id = $1), 1), $2, FALSE, $3)`,
		publication.strOr("document_id", ""), "Mitra menolak revisi: "+reason, user.ID); err != nil {
		d.logger().Warn("gagal mencatat alasan penolakan revisi", "publication", publicationID, "err", err)
	}
	d.auditLog(r.Context(), user.ID, "publication.revision_rejected", "publication", publicationID)
	d.revisionEventNotification(r.Context(), publicationID, "publication.revision_rejected", reason, rejects >= revisionRejectLimit)

	updated, err := d.partnerScopedPublication(r.Context(), partner.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, partnerPublicationJSON(updated))
}

// revisionEventNotification: kabari klien (dan admin saat penolakan mentok di
// batas) tentang keputusan mitra atas revisi.
func (d *Deps) revisionEventNotification(ctx context.Context, publicationID, event string, args ...any) {
	ev := d.loadPublicationEventRow(ctx, publicationID)
	if ev == nil {
		return
	}
	brandLink := "/klien/" + derefOr(ev.BrandPublicID, ev.BrandID) + "/publications?pub=" + publicationID
	switch event {
	case "publication.revision_accepted":
		title := "Revisi diterima mitra"
		body := fmt.Sprintf("Mitra %s menerima revisi %q. Artikel dikirim ulang ke %s.", ev.MediaName, ev.DocumentTitle, ev.MediaName)
		_ = d.notifyBrandMembers(ctx, ev.BrandID, event, title, body, &brandLink, &ev.ID)
	case "publication.revision_rejected":
		reason := derefOr(ev.RejectionReason, "")
		limitReached := false
		if len(args) > 0 {
			if s, isStr := args[0].(string); isStr && s != "" {
				reason = s
			}
		}
		if len(args) > 1 {
			if b, isBool := args[1].(bool); isBool {
				limitReached = b
			}
		}
		title := "Revisi ditolak mitra"
		body := fmt.Sprintf("Mitra %s menolak revisi %q — %s", ev.MediaName, ev.DocumentTitle, reason)
		if limitReached {
			body += fmt.Sprintf(" Batas penolakan (%d kali) tercapai; revisi berikutnya hanya bisa diterima.", revisionRejectLimit)
		}
		_ = d.notifyBrandMembers(ctx, ev.BrandID, event, title, body, &brandLink, &ev.ID)
		if limitReached {
			adminLink := "/admin/publications?pub=" + publicationID
			adminBody := fmt.Sprintf("Revisi %q di %s ditolak %d kali oleh mitra. Perlu tindakan admin.", ev.DocumentTitle, ev.MediaName, revisionRejectLimit)
			d.notifyPlatformAdmins(ctx, event, "Revisi mentok di batas penolakan", adminBody, &adminLink)
		}
	}
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

// Batas pemakaian Rewrite AI gratis per publikasi (aturan pemilik produk);
// issuance dihitung di handler rewrite, angka ini untuk tampilan frontend.
const rewriteMaxPerPublicationPublic = 2

func maxIntP(a, b int) int {
	if a > b {
		return a
	}
	return b
}
