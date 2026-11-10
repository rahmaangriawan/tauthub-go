package brandflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// publications.go: port app/v1/brands/[brandId]/publications/**/route.ts.

// ---------- GET / POST /v1/brands/{brandId}/publications ----------

// PublicationsList: GET /v1/brands/{brandId}/publications
func (d *Deps) PublicationsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !validPublicationStatus(status) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid publication status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := intParamDef(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	search := strings.TrimSpace(q.Get("q"))
	if utf8.RuneCountInString(search) > 120 {
		search = string([]rune(search)[:120])
	}

	clause := "WHERE p.brand_id = $1"
	baseParams := []any{brandRow.ID}
	if status != "" {
		clause += " AND p.status = $" + itoa(len(baseParams)+1)
		baseParams = append(baseParams, status)
	}
	if search != "" {
		like := "%" + search + "%"
		clause += " AND (d.title ILIKE $" + itoa(len(baseParams)+1) + " OR m.name ILIKE $" + itoa(len(baseParams)+2) + " OR m.category ILIKE $" + itoa(len(baseParams)+3) + ")"
		baseParams = append(baseParams, like, like, like)
	}
	countRow, err := d.queryRow(r.Context(),
		`SELECT COUNT(*) AS count FROM publications p
		 JOIN content_documents d ON d.id = p.document_id
		 JOIN media m ON m.id = p.media_id
		 `+clause, baseParams...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	total := 0
	if countRow != nil {
		total = countRow.intOr("count", 0)
	}
	pageArgs := append(append([]any{}, baseParams...), limit, offset)
	items, err := d.queryRows(r.Context(), `
		SELECT p.id, p.brand_id, p.document_id, p.media_id, p.status, p.client_price, p.publish_status, p.connection_id, p.scheduled_at, p.published_at, p.published_url, p.featured_image_url, p.partner_draft_edits, p.notes, p.cancel_reason, p.campaign_id, p.order_code, p.created_by, p.created_at, p.updated_at,
		       d.title AS document_title, d.slug AS document_slug,
		       m.name AS media_name, m.category AS media_category, m.website_url AS media_website_url, m.status AS media_status,
		       COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = m.id), m.category) AS media_categories_csv,
		       ma.consistency_score AS pr_message_score, ma.status AS pr_message_status,
		       sa.sentiment AS pr_sentiment, sa.confidence AS pr_sentiment_confidence,
		       v.status AS pr_verification_status, v.compliance_score AS pr_verification_compliance_score,
		       v.verified_at AS pr_verification_verified_at
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		LEFT JOIN pr_message_analyses ma ON ma.publication_id = p.id
		LEFT JOIN pr_sentiment_analyses sa ON sa.publication_id = p.id
		LEFT JOIN pr_publication_verifications v ON v.publication_id = p.id
		`+clause+`
		ORDER BY p.created_at DESC LIMIT $`+itoa(len(baseParams)+1)+` OFFSET $`+itoa(len(baseParams)+2),
		pageArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publications := make([]map[string]any, 0, len(items))
	for _, item := range items {
		publications = append(publications, toPublicationJSON(item, 0))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"publications": publications, "total": total, "limit": limit, "offset": offset,
	})
}

// PublicationsCreate: POST /v1/brands/{brandId}/publications
func (d *Deps) PublicationsCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)

	documentID, _, err := requiredTrimmedString(body, "document_id", 1, 1<<30, nil)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	mediaID, _, err := requiredTrimmedString(body, "media_id", 1, 1<<30, nil)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	connectionID, _ := strField(body, "connection_id")
	publishStatus := "draft"
	if s, ok := strField(body, "publish_status"); ok && s == "publish" {
		publishStatus = "publish"
	}
	scheduledAt, err := optionalDatetime(body, "scheduled_at")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	notes, err := optionalString(body, "notes", 1000)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID, _ := strField(body, "campaign_id")
	skipStandardCheck := false
	if b, ok := boolField(body, "skip_standard_check"); ok {
		skipStandardCheck = b
	}
	packageID, _ := strField(body, "package_id")
	var packagePickIDs []string
	if ids, ok := stringSliceField(body, "package_pick_media_ids"); ok {
		packagePickIDs = ids
	}

	// Package pick enforcement (server-side): maks. 5 media, termasuk paket.
	if packageID != "" && len(packagePickIDs) > 0 {
		if len(packagePickIDs) > 5 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Maksimal 5 media dapat dipilih dari satu paket media.")
			return
		}
		if !containsString(packagePickIDs, mediaID) {
			httpx.Error(w, http.StatusUnprocessableEntity, "Media yang dipublikasikan tidak termasuk dalam pilihan paket.")
			return
		}
		countRow, err := d.queryRow(r.Context(),
			"SELECT COUNT(*) AS c FROM media_package_items WHERE package_id = $1 AND media_id IN ("+placeholders(2, len(packagePickIDs))+")",
			append([]any{packageID}, stringArgs(packagePickIDs)...)...)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if countRow == nil || countRow.intOr("c", 0) != len(packagePickIDs) {
			httpx.Error(w, http.StatusUnprocessableEntity, "Salah satu media pilihan tidak termasuk dalam paket ini.")
			return
		}
	}

	// Phase 5 campaign context: quota + credits hanya untuk publikasi kampanye.
	// Biaya media sama untuk jalur kampanye dan jalur dompet, jadi dihitung
	// sekali per request — dulu jalur dompet mengulang query yang identik.
	cost := 0
	var (
		mediaCostVal  int
		mediaCostDone bool
	)
	mediaCost := func() int {
		if !mediaCostDone {
			mediaCostVal = d.creditCostForMedia(r.Context(), mediaID)
			mediaCostDone = true
		}
		return mediaCostVal
	}
	var campaignIDPtr *string
	if campaignID != "" {
		campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if err := assertCampaignQuota(campaign, 1); err != nil {
			d.writeErr(w, err)
			return
		}
		cost = mediaCost()
		if cost > 0 {
			balance := d.brandCreditBalance(r.Context(), brandRow.ID)
			if balance.PackageCount > 0 && balance.Remaining < cost {
				d.writeErr(w, errConflict("Insufficient credits: need "+itoa(cost)+", available "+itoa(balance.Remaining)))
				return
			}
		}
		campaignIDPtr = &campaignID
	}

	// Tulisio Credit: brand dengan dompet membayar TC untuk setiap publikasi.
	wallet := d.getWallet(r.Context(), brandRow.ID)
	if wallet != nil {
		cost = mediaCost()
		if packageID != "" {
			pkg, err := d.queryRow(r.Context(),
				`SELECT mp.discount_percent
				 FROM media_packages mp
				 JOIN media_package_items mpi ON mpi.package_id = mp.id
				 WHERE mp.id = $1 AND mp.status = 'active' AND mpi.media_id = $2
				 LIMIT 1`, packageID, mediaID)
			if err != nil {
				d.writeErr(w, err)
				return
			}
			if pkg != nil {
				discount := pkg.numOr("discount_percent")
				if discount < 0 {
					discount = 0
				}
				if discount > 100 {
					discount = 100
				}
				if discount > 0 {
					cost = int(float64(cost) * (100 - discount))
					cost = (cost + 50) / 100 // Math.round((cost*(100-discount))/100)
					if cost < 1 {
						cost = 1
					}
				}
			}
		}
	}

	var scheduledPtr *time.Time
	if scheduledAt != nil {
		if s, ok := scheduledAt.(string); ok {
			if t, ok := parseISO(s); ok {
				scheduledPtr = &t
			}
		}
	}
	var notesPtr *string
	if s, ok := notes.(string); ok {
		notesPtr = &s
	}
	var connectionPtr *string
	if connectionID != "" {
		connectionPtr = &connectionID
	}

	// Reserve dijalankan di dalam createPublicationRecord — SEBELUM publikasi
	// di-enqueue — supaya worker tidak bisa menerbitkan artikel sebelum saldo
	// ditahan (kalau itu terjadi, captureCredit melihat outstanding 0 dan
	// artikel terbit tanpa pernah menagih).
	var reserveFn func(string) error
	if wallet != nil && cost > 0 {
		reserveFn = func(pubID string) error {
			_, err := d.reserveCredit(r.Context(), brandRow.ID, pubID, cost, user.ID)
			return err
		}
	} else if wallet == nil && campaignID != "" && cost > 0 {
		reserveFn = func(pubID string) error {
			_, err := d.reservePackageCredits(r.Context(), brandRow.ID, pubID, cost, user.ID)
			return err
		}
	}

	created, err := d.createPublicationRecord(r.Context(), createPublicationInput{
		BrandID: brandRow.ID, UserID: user.ID, DocumentID: documentID, MediaID: mediaID,
		ConnectionID: connectionPtr, PublishStatus: publishStatus, ScheduledAt: scheduledPtr,
		Notes: notesPtr, CampaignID: campaignIDPtr, SkipStandardCheck: skipStandardCheck,
		ReserveCredit: reserveFn,
	})
	if err != nil {
		var ic *insufficientCreditsError
		if asInsufficient(err, &ic) {
			writeCreditError(w, cost, ic.Available)
			return
		}
		d.writeErr(w, err)
		return
	}
	id := created.Row.strOr("id", "")

	d.auditPublication(r.Context(), brandRow.ID, user.ID, "publication.created", id)
	payload, err := d.publicationJSON(r.Context(), brandRow.ID, id)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

// ---------- GET /v1/brands/{brandId}/publications/{publicationId} ----------

// PublicationGet: GET /v1/brands/{brandId}/publications/{publicationId}
func (d *Deps) PublicationGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	base, err := d.publicationJSON(r.Context(), brandRow.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	documentID := stringOrEmpty(base["document_id"])
	mediaID := stringOrEmpty(base["media_id"])

	document, err := d.queryRow(r.Context(),
		"SELECT title, slug, meta_title, meta_description, status FROM content_documents WHERE id = $1 LIMIT 1", documentID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	media, err := d.queryRow(r.Context(),
		"SELECT website_url, integration_type, category FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	attempts, err := d.queryRows(r.Context(),
		"SELECT attempt_number, status, error_code, error_message, started_at, completed_at FROM publication_attempts WHERE publication_id = $1 ORDER BY attempt_number ASC", publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	job, err := d.queryRow(r.Context(),
		"SELECT status, remote_url, error_message, created_at FROM publication_jobs WHERE publication_id = $1 ORDER BY created_at DESC LIMIT 1", publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	replacesRow, err := d.queryRow(r.Context(), `
		SELECT p.id, p.status, p.created_at, m.name AS media_name, d.title AS document_title
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN content_documents d ON d.id = p.document_id
		WHERE p.id = (SELECT replaces_id FROM publications WHERE id = $1) LIMIT 1`, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	replacedByRow, err := d.queryRow(r.Context(), `
		SELECT p.id, p.status, p.created_at, p.published_at, p.published_url, m.name AS media_name, d.title AS document_title
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN content_documents d ON d.id = p.document_id
		WHERE p.replaces_id = $1 ORDER BY p.created_at ASC LIMIT 1`, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	var documentJSON any
	if document != nil {
		documentJSON = map[string]any{
			"title":            document.raw("title"),
			"slug":             document.raw("slug"),
			"meta_title":       document.raw("meta_title"),
			"meta_description": document.raw("meta_description"),
			"status":           document.raw("status"),
		}
	}
	attemptJSON := make([]map[string]any, 0, len(attempts))
	for _, a := range attempts {
		attemptJSON = append(attemptJSON, map[string]any{
			"attempt_number": a.intOr("attempt_number", 0),
			"status":         a.raw("status"),
			"error_code":     a.raw("error_code"),
			"error_message":  a.raw("error_message"),
			"started_at":     a.timePtr("started_at"),
			"completed_at":   a.timePtr("completed_at"),
		})
	}
	var jobJSON any
	if job != nil {
		jobJSON = map[string]any{
			"status":        job.raw("status"),
			"remote_url":    job.raw("remote_url"),
			"error_message": job.raw("error_message"),
			"created_at":    job.timePtr("created_at"),
		}
	}
	var replacesJSON any
	if replacesRow != nil {
		replacesJSON = map[string]any{
			"id": replacesRow.raw("id"), "status": replacesRow.raw("status"),
			"media_name": replacesRow.raw("media_name"), "document_title": replacesRow.raw("document_title"),
			"created_at": replacesRow.timePtr("created_at"),
		}
	}
	var replacedByJSON any
	if replacedByRow != nil {
		replacedByJSON = map[string]any{
			"id": replacedByRow.raw("id"), "status": replacedByRow.raw("status"),
			"media_name": replacedByRow.raw("media_name"), "document_title": replacedByRow.raw("document_title"),
			"published_at": replacedByRow.timePtr("published_at"), "published_url": replacedByRow.raw("published_url"),
			"created_at": replacedByRow.timePtr("created_at"),
		}
	}

	base["document"] = documentJSON
	if media != nil {
		base["media_website_url"] = media.raw("website_url")
		base["media_integration_type"] = media.raw("integration_type")
		base["media_category"] = media.raw("category")
	} else {
		base["media_website_url"] = nil
		base["media_integration_type"] = nil
		base["media_category"] = nil
	}
	base["attempts"] = attemptJSON
	base["job"] = jobJSON
	base["replaces"] = replacesJSON
	base["replaced_by"] = replacedByJSON
	base["pr_analysis"] = d.publicationPrAnalysis(r.Context(), publicationID)
	writeJSON(w, http.StatusOK, base)
}

// publicationPrAnalysis: parity getPublicationPrAnalysis (lib/pr/publication-summary.ts).
func (d *Deps) publicationPrAnalysis(ctx context.Context, publicationID string) any {
	message, err := d.queryRow(ctx,
		"SELECT consistency_score, status FROM pr_message_analyses WHERE publication_id = $1 LIMIT 1", publicationID)
	if err != nil {
		message = nil
	}
	sentiment, err := d.queryRow(ctx,
		"SELECT sentiment, confidence, status FROM pr_sentiment_analyses WHERE publication_id = $1 LIMIT 1", publicationID)
	if err != nil {
		sentiment = nil
	}
	verification, err := d.queryRow(ctx,
		"SELECT status, compliance_score, verified_at FROM pr_publication_verifications WHERE publication_id = $1 ORDER BY verified_at DESC LIMIT 1", publicationID)
	if err != nil {
		verification = nil
	}
	if message == nil && sentiment == nil && verification == nil {
		return nil
	}
	var messageJSON any
	if message != nil {
		messageJSON = map[string]any{"score": message.numPtr("consistency_score"), "status": message.raw("status")}
	}
	var sentimentJSON any
	if sentiment != nil {
		sentimentJSON = map[string]any{"sentiment": sentiment.strOr("sentiment", "neutral"), "confidence": sentiment.numPtr("confidence"), "status": sentiment.strOr("status", "completed")}
	}
	var verificationJSON any
	if verification != nil {
		verificationJSON = map[string]any{"status": verification.raw("status"), "compliance_score": verification.numPtr("compliance_score"), "verified_at": verification.timePtr("verified_at")}
	}
	return map[string]any{"message": messageJSON, "sentiment": sentimentJSON, "verification": verificationJSON}
}

// ---------- lifecycle actions ----------

// PublicationCancel: POST /v1/brands/{brandId}/publications/{publicationId}/cancel
func (d *Deps) PublicationCancel(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	body := decodeJSON(r)
	reason, err := optionalString(body, "reason", 500)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id, status FROM publications WHERE id = $1 AND brand_id = $2 LIMIT 1", publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		d.writeErr(w, errNotFound("Publication not found"))
		return
	}
	// Selama processing worker sudah memegang payload dan bisa mendorong artikel
	// ke CMS mitra; membatalkan + refund di titik itu = artikel live tanpa tagihan.
	if row.strOr("status", "") == "processing" {
		d.writeErr(w, errConflict("Publikasi sedang diproses dan belum bisa dibatalkan; coba lagi setelah selesai."))
		return
	}
	extra := map[string]any{"cancel_reason": reason}
	moved, err := d.transitionPublicationIf(r.Context(), publicationID, "cancelled", extra)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !moved {
		d.writeErr(w, errConflict("Status publikasi berubah saat dibatalkan; muat ulang dan coba lagi."))
		return
	}
	d.settleCancelledPublication(r.Context(), brandRow.ID, publicationID, user.ID, "publication.cancelled")
	payload, err := d.publicationJSON(r.Context(), brandRow.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// settleCancelledPublication: efek samping setelah status publikasi menjadi
// 'cancelled' — kembalikan kredit paket legacy + lepas reservasi TC wallet,
// lalu catat audit.
//
// Dipisah dari PublicationCancel supaya scheduler SLA bisa memakai jalur yang
// sama persis (satu tempat untuk aturan pengembalian saldo). Pemanggil WAJIB
// memastikan transisi status benar-benar terjadi lebih dulu: releaseCredit
// idempoten per publicationID, jadi memanggilnya untuk pesanan yang batal
// transisi akan mengembalikan saldo pada pesanan yang masih berjalan.
func (d *Deps) settleCancelledPublication(ctx context.Context, brandID, publicationID, userID, auditAction string) {
	d.refundPackageCredits(ctx, brandID, publicationID, userID)
	d.releaseCredit(ctx, brandID, publicationID, userID)
	d.auditPublication(ctx, brandID, userID, auditAction, publicationID)
}

// PublicationAccept: POST /v1/brands/{brandId}/publications/{publicationId}/accept
func (d *Deps) PublicationAccept(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.queryRow(r.Context(), `
		SELECT p.id, p.status, p.client_approved_at, m.name AS media_name,
		       prt.id AS partner_id, prt.public_id AS partner_public_id,
		       prt.user_id AS partner_user_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN partners prt ON prt.id = m.partner_id
		WHERE p.id = $1 AND p.brand_id = $2
		LIMIT 1`, publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if publication == nil {
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return
	}
	alreadyApproved := publication.has("client_approved_at")
	if err := d.exec(r.Context(),
		`UPDATE publications
		    SET client_approved_at = COALESCE(client_approved_at, NOW()),
		        client_approved_by = COALESCE(client_approved_by, $1)
		  WHERE id = $2 AND brand_id = $3`, user.ID, publicationID, brandRow.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	if !alreadyApproved {
		mediaName := publication.strOr("media_name", "media")
		partnerUserID := publication.strOr("partner_user_id", "")
		partnerSlug := publication.strOr("partner_public_id", publication.strOr("partner_id", ""))
		title := "Artikel disetujui klien"
		message := "Klien menyetujui artikel di " + mediaName + " untuk diterbitkan."
		var partnerLink *string
		if partnerSlug != "" {
			link := "/mitra/" + partnerSlug + "/publications?pub=" + publicationID
			partnerLink = &link
		}
		if partnerUserID != "" {
			d.createNotification(r.Context(), partnerUserID, "publication.client_approved", title, message, partnerLink, &publicationID)
		}
	}
	d.auditPublication(r.Context(), brandRow.ID, user.ID, "publication.client_approved", publicationID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "publication_id": publicationID, "already_approved": alreadyApproved,
		"publication_status": publication.raw("status"),
	})
}

// PublicationRetry: POST /v1/brands/{brandId}/publications/{publicationId}/retry
func (d *Deps) PublicationRetry(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	row, err := d.queryRow(r.Context(),
		"SELECT id, status FROM publications WHERE id = $1 AND brand_id = $2 LIMIT 1", publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		d.writeErr(w, errNotFound("Publication not found"))
		return
	}
	// Gagal permanen melepas reservasi kredit; tahan ulang SEBELUM mengantrekan,
	// atau tolak (saldo kurang) agar artikel tidak terbit tanpa tagihan.
	reacquired := false
	if st := row.strOr("status", ""); st == "failed" || st == "retrying" {
		reacquired, err = d.reacquireReservation(r.Context(), brandRow.ID, publicationID, user.ID)
		if err != nil {
			var ic *insufficientCreditsError
			if asInsufficient(err, &ic) {
				writeCreditError(w, ic.Needed, ic.Available)
				return
			}
			d.writeErr(w, err)
			return
		}
	}
	if err := d.transitionPublication(r.Context(), publicationID, "queued"); err != nil {
		if reacquired {
			d.releaseCredit(r.Context(), brandRow.ID, publicationID, user.ID)
		}
		d.writeErr(w, err)
		return
	}
	d.enqueuePublicationExecution(r.Context(), publicationID, brandRow.ID, nil)
	d.auditPublication(r.Context(), brandRow.ID, user.ID, "publication.retried", publicationID)
	payload, err := d.publicationJSON(r.Context(), brandRow.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// PublicationSchedule: POST /v1/brands/{brandId}/publications/{publicationId}/schedule
// Atur ulang waktu terbit publikasi yang belum terbit. `scheduled_at` kosong/null
// berarti terbit sekarang — scheduler mengambilnya pada sweep berikutnya.
func (d *Deps) PublicationSchedule(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	body := decodeJSON(r)
	parsed, err := optionalDatetime(body, "scheduled_at")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id, status, publish_status, published_at FROM publications WHERE id = $1 AND brand_id = $2 LIMIT 1",
		publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		d.writeErr(w, errNotFound("Publication not found"))
		return
	}
	if row.timePtr("published_at") != nil || row.strOr("publish_status", "") == "published" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Publikasi sudah terbit; jadwal tidak bisa diubah.")
		return
	}
	switch row.strOr("status", "") {
	case "pending", "waiting_partner", "partner_approved", "retrying", "queued":
	default:
		httpx.Error(w, http.StatusConflict, "Status publikasi ini tidak bisa dijadwalkan ulang.")
		return
	}
	scheduled := time.Now().UTC()
	if t, ok := parsed.(time.Time); ok {
		scheduled = t.UTC()
	}
	if err := d.exec(r.Context(),
		"UPDATE publications SET scheduled_at = $1, updated_at = NOW() WHERE id = $2 AND brand_id = $3",
		scheduled, publicationID, brandRow.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditPublication(r.Context(), brandRow.ID, user.ID, "publication.rescheduled", publicationID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "scheduled_at": scheduled.Format(time.RFC3339)})
}

var resettablePublicationStatuses = map[string]bool{
	"processing": true, "failed": true, "retrying": true, "cancelled": true, "queued": true,
}

// PublicationResetApproval: POST /v1/brands/{brandId}/publications/{publicationId}/reset-approval
func (d *Deps) PublicationResetApproval(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	row, err := d.queryRow(r.Context(), `
		SELECT p.id, p.status, p.brand_id, p.media_id, p.document_id,
		       m.name AS media_name, m.partner_id, ptn.user_id AS partner_user_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN partners ptn ON ptn.id = m.partner_id
		WHERE p.id = $1 AND p.brand_id = $2 LIMIT 1`, publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		d.writeErr(w, errNotFound("Publication not found"))
		return
	}
	if !row.has("partner_id") {
		httpx.Error(w, http.StatusUnprocessableEntity, "Publikasi ini tidak melalui persetujuan mitra.")
		return
	}
	currentStatus := row.strOr("status", "")
	if !resettablePublicationStatuses[currentStatus] {
		httpx.Error(w, http.StatusConflict, "Status "+currentStatus+" tidak bisa di-reset ke persetujuan.")
		return
	}
	// failed/retrying/cancelled sudah melepas reservasi kredit; tahan ulang dulu
	// (atau tolak bila saldo kurang), kalau tidak publikasi terbit tanpa tagihan
	// sementara penghasilan mitra tetap dicatat.
	reacquired := false
	if currentStatus == "failed" || currentStatus == "retrying" || currentStatus == "cancelled" {
		reacquired, err = d.reacquireReservation(r.Context(), brandRow.ID, publicationID, user.ID)
		if err != nil {
			var ic *insufficientCreditsError
			if asInsufficient(err, &ic) {
				writeCreditError(w, ic.Needed, ic.Available)
				return
			}
			d.writeErr(w, err)
			return
		}
	}
	// Tulis hanya bila status masih sama dengan yang dibaca (cegah balapan).
	affected, err := d.execRows(r.Context(),
		"UPDATE publications SET status = 'waiting_partner', updated_at = NOW() WHERE id = $1 AND brand_id = $2 AND status = $3",
		publicationID, brandRow.ID, currentStatus)
	if err != nil || affected != 1 {
		if reacquired {
			d.releaseCredit(r.Context(), brandRow.ID, publicationID, user.ID)
		}
		if err != nil {
			d.writeErr(w, err)
			return
		}
		httpx.Error(w, http.StatusConflict, "Status publikasi berubah. Muat ulang lalu coba lagi.")
		return
	}
	d.publicationWaitingPartnerNotification(r.Context(), publicationID)
	d.auditPublication(r.Context(), brandRow.ID, user.ID, "publication.reset_to_approval", publicationID)
	payload, err := d.publicationJSON(r.Context(), brandRow.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// PublicationAutoReplace: POST /v1/brands/{brandId}/publications/{publicationId}/auto-replace
func (d *Deps) PublicationAutoReplace(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	body := decodeJSON(r)
	disabled, ok := boolField(body, "disabled")
	if !ok {
		httpx.Error(w, http.StatusUnprocessableEntity, "disabled (boolean) wajib diisi")
		return
	}
	affected, err := d.execRows(r.Context(),
		"UPDATE publications SET auto_replace_disabled = $1, updated_at = NOW() WHERE id = $2 AND brand_id = $3",
		disabled, publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if affected == 0 {
		httpx.Error(w, http.StatusNotFound, "Publication not found")
		return
	}
	payload, err := d.publicationJSON(r.Context(), brandRow.ID, publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// PublicationRequestRevision: POST /v1/brands/{brandId}/publications/{publicationId}/request-revision
func (d *Deps) PublicationRequestRevision(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	body := decodeJSON(r)
	comment, ok := strField(body, "comment")
	if !ok || utf8.RuneCountInString(comment) < 5 {
		d.writeErr(w, errUnprocessable("String must contain at least 5 character(s)"))
		return
	}
	if utf8.RuneCountInString(comment) > 1000 {
		d.writeErr(w, errUnprocessable("String must contain at most 1000 character(s)"))
		return
	}
	attachmentURL, err := optionalString(body, "attachmentUrl", 500)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	attachmentName, err := optionalString(body, "attachmentName", 255)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	documentVersion := 0
	if raw, ok := body["documentVersion"]; ok {
		if f, isNum := raw.(float64); isNum && f > 0 {
			documentVersion = int(f)
		}
	}
	featuredImageURL, err := optionalString(body, "featuredImageUrl", 500)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	publication, err := d.queryRow(r.Context(), `
		SELECT p.id, p.status, p.document_id, m.name AS media_name,
		       d.current_version,
		       prt.id AS partner_id, prt.public_id AS partner_public_id,
		       prt.user_id AS partner_user_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN partners prt ON prt.id = m.partner_id
		LEFT JOIN content_documents d ON d.id = p.document_id
		WHERE p.id = $1 AND p.brand_id = $2
		LIMIT 1`, publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if publication == nil {
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return
	}
	documentID := publication.strOr("document_id", "")
	if documentID == "" {
		httpx.Error(w, http.StatusConflict, "Artikel untuk media ini belum punya dokumen, jadi revisi belum bisa dicatat.")
		return
	}
	version := documentVersion
	if version == 0 {
		if publication.has("current_version") {
			version = publication.intOr("current_version", 1)
		} else {
			version = 1
		}
	}
	if err := d.exec(r.Context(),
		`INSERT INTO review_comments (id, document_id, version, body, attachment_url, attachment_name, resolved, created_by)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, FALSE, $6)`,
		documentID, version, comment, attachmentURL, attachmentName, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	mediaName := publication.strOr("media_name", "media")
	pubStatus := publication.strOr("status", "")
	// Publikasi yang sudah tayang dipindah ke status 'revision' supaya pemilik
	// domain melihat permintaan ini dan memutuskan terima/tolak. Cover baru ikut
	// disimpan karena CMS memakai publications.featured_image_url saat kirim ulang.
	if pubStatus == "published" {
		extra := map[string]any{}
		if featuredImageURL != "" {
			extra["featured_image_url"] = featuredImageURL
		}
		if err := d.transitionPublicationWith(r.Context(), publicationID, "revision", extra); err != nil {
			d.writeErr(w, err)
			return
		}
		pubStatus = "revision"
	} else if featuredImageURL != "" {
		if err := d.exec(r.Context(),
			"UPDATE publications SET featured_image_url = $1, updated_at = NOW() WHERE id = $2",
			featuredImageURL, publicationID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	partnerUserID := publication.strOr("partner_user_id", "")
	partnerSlug := publication.strOr("partner_public_id", publication.strOr("partner_id", ""))
	title := "Revisi artikel dari klien"
	message := truncateString("Klien meminta revisi untuk artikel di "+mediaName+": "+comment, 1000)
	var partnerLink *string
	if partnerSlug != "" {
		link := "/mitra/" + partnerSlug + "/publications?pub=" + publicationID
		partnerLink = &link
	}
	if partnerUserID != "" {
		d.createNotification(r.Context(), partnerUserID, "publication.revision_requested", title, message, partnerLink, &publicationID)
	}
	d.auditPublication(r.Context(), brandRow.ID, user.ID, "publication.revision_requested", publicationID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "publication_id": publicationID, "version": version,
		"notified_partner": partnerUserID != "",
		"status":           pubStatus,
	})
}

// (PublicationReplacementGet ditambahkan agen lain di bagian bawah file ini.)

// PublicationRequestURL: POST /v1/brands/{brandId}/publications/{publicationId}/request-url
func (d *Deps) PublicationRequestURL(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	row, err := d.queryRow(r.Context(), `
		SELECT p.id, p.status, p.published_url, p.published_at, p.created_at,
		       v.status AS verification_status
		FROM publications p
		LEFT JOIN pr_publication_verifications v ON v.publication_id = p.id
		WHERE p.id = $1 AND p.brand_id = $2
		ORDER BY v.verified_at DESC
		LIMIT 1`, publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		d.writeErr(w, errNotFound("Publication not found"))
		return
	}
	if row.strOr("status", "") != "published" {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "reason": "not_published"})
		return
	}
	verified := row.strOr("verification_status", "") == "passed"
	if row.strOr("published_url", "") != "" || verified {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "reason": "already_verified"})
		return
	}
	anchor := row.timePtr("published_at")
	if anchor == nil {
		anchor = row.timePtr("created_at")
	}
	ageDays := 0.0
	if s, ok := anchor.(string); ok {
		if t, ok := parseISO(s); ok {
			ageDays = time.Since(t).Hours() / 24
		}
	}
	if ageDays < 7 {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "reason": "too_early", "age_days": int(ageDays)})
		return
	}
	notified := d.notifyPartnerURLRequested(r.Context(), publicationID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "partner_notified": notified})
}

// notifyPartnerURLRequested: parity notifyPartnerUrlRequested — notifikasi ke
// user mitra (best-effort), true bila terkirim.
func (d *Deps) notifyPartnerURLRequested(ctx context.Context, publicationID string) bool {
	row, err := d.queryRow(ctx, `
		SELECT p.brand_id, m.name AS media_name, d.title AS document_title,
		       pa.user_id AS partner_user_id, pa.public_id AS partner_public_id,
		       b.public_id AS brand_public_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || row == nil {
		return false
	}
	partnerUserID := row.strOr("partner_user_id", "")
	if partnerUserID == "" {
		return false
	}
	mediaName := row.strOr("media_name", "Media")
	article := row.strOr("document_title", "Artikel")
	var link *string
	if pp := row.strOr("partner_public_id", ""); pp != "" {
		l := "/mitra/" + pp + "/publications?pub=" + publicationID
		link = &l
	}
	body := "Klien meminta URL publikasi \"" + article + "\" di " + mediaName + " karena verifikasi belum tersedia. Mohon perbarui URL artikel."
	d.createNotification(ctx, partnerUserID, "publication.url_requested", "Permintaan URL publikasi", body, link, &publicationID)
	return true
}

// ---------- varians ----------

// PublicationVariantsCreate: POST /v1/brands/{brandId}/publications/variants
func (d *Deps) PublicationVariantsCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	mode, _ := strField(body, "mode")
	if mode != "generate" && mode != "paraphrase" {
		d.writeErr(w, errUnprocessable("Invalid option: expected one of \"generate\"|\"paraphrase\""))
		return
	}
	mediaIDs, ok := stringSliceField(body, "media_ids")
	if !ok || len(mediaIDs) == 0 {
		d.writeErr(w, errUnprocessable("Array must contain at least 1 element(s)"))
		return
	}
	if len(mediaIDs) > 20 {
		d.writeErr(w, errUnprocessable("Array must contain at most 20 element(s)"))
		return
	}
	documentID, _ := strField(body, "document_id")
	sourceMaterial, err := optionalString(body, "source_material", 200000)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID, _ := strField(body, "campaign_id")
	connectionID, _ := strField(body, "connection_id")
	publishStatus := "draft"
	if s, ok := strField(body, "publish_status"); ok && s == "publish" {
		publishStatus = "publish"
	}
	scheduledAt, err := optionalDatetime(body, "scheduled_at")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	skipStandardCheck := false
	if b, ok := boolField(body, "skip_standard_check"); ok {
		skipStandardCheck = b
	}

	sourceMaterialStr := ""
	if s, ok := sourceMaterial.(string); ok {
		sourceMaterialStr = s
	}
	if mode == "generate" && utf8.RuneCountInString(strings.TrimSpace(sourceMaterialStr)) < 40 {
		d.writeErr(w, errConflict("Bahan terlalu singkat — isi minimal 40 karakter agar artikel bisa dibuatkan dengan baik."))
		return
	}
	if mode == "paraphrase" && documentID == "" {
		d.writeErr(w, errConflict("Pilih artikel yang akan diparafrase."))
		return
	}

	mediaRows, err := d.queryRows(r.Context(),
		"SELECT id, name, price, status FROM media WHERE id IN ("+placeholders(1, len(mediaIDs))+")", stringArgs(mediaIDs)...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	mediaByID := map[string]bsRow{}
	for _, m := range mediaRows {
		mediaByID[m.strOr("id", "")] = m
	}
	invalid := []string{}
	for _, id := range mediaIDs {
		media, ok := mediaByID[id]
		if !ok || media.strOr("status", "") != "active" {
			invalid = append(invalid, id)
		}
	}
	if len(invalid) > 0 {
		d.writeErr(w, errNotFound("Satu atau lebih media tidak ditemukan atau tidak aktif."))
		return
	}

	totalCost := 0
	if campaignID != "" {
		campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if err := assertCampaignQuota(campaign, len(mediaIDs)); err != nil {
			d.writeErr(w, err)
			return
		}
		costs := d.creditCostsForMedia(r.Context(), mediaIDs)
		for _, id := range mediaIDs {
			totalCost += costs[id]
		}
		if totalCost > 0 {
			balance := d.brandCreditBalance(r.Context(), brandRow.ID)
			if balance.PackageCount > 0 && balance.Remaining < totalCost {
				d.writeErr(w, errConflict("Insufficient credits: need "+itoa(totalCost)+", available "+itoa(balance.Remaining)))
				return
			}
		}
	}

	var sourceArticleText *string
	if mode == "paraphrase" && documentID != "" {
		document, err := d.scopedDocument(r.Context(), brandRow.ID, documentID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if document.strOr("status", "") != "approved" {
			d.writeErr(w, errConflict("Artikel sumber harus berstatus approved."))
			return
		}
		version, err := d.queryRow(r.Context(),
			"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1", documentID, document.intOr("current_version", 1))
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if version == nil {
			d.writeErr(w, errConflict("Versi artikel sumber tidak ditemukan."))
			return
		}
		text := version.strOr("content", "")
		sourceArticleText = &text
	}

	seed := randomSeed()
	angles := pickAngles(len(mediaIDs), seed)
	batchID := newUUID()
	var sourceDocIDArg any
	if mode == "paraphrase" && documentID != "" {
		sourceDocIDArg = documentID
	}
	var sourceMatArg any
	if mode == "generate" {
		sourceMatArg = nullIfEmpty(sourceMaterialStr)
	}
	if err := d.exec(r.Context(),
		"INSERT INTO publication_variant_batches (id, brand_id, user_id, mode, seed, status, source_document_id, source_material) VALUES ($1, $2, $3, $4, $5, 'queued', $6, $7)",
		batchID, brandRow.ID, user.ID, mode, seed, sourceDocIDArg, sourceMatArg); err != nil {
		d.writeErr(w, err)
		return
	}

	results := make([]map[string]any, 0, len(mediaIDs))
	var scheduledISO any
	if scheduledAt != nil {
		if s, ok := scheduledAt.(string); ok {
			scheduledISO = s
		}
	}
	for i, mediaID := range mediaIDs {
		angle := angles[i]
		variantID := newUUID()
		if err := d.exec(r.Context(),
			"INSERT INTO publication_variants (id, batch_id, media_id, angle_id, angle_label, status) VALUES ($1, $2, $3, $4, $5, 'queued')",
			variantID, batchID, mediaID, angle.ID, angle.Label); err != nil {
			d.writeErr(w, err)
			return
		}
		payload := map[string]any{
			"batchId": batchID, "variantId": variantID, "brandId": brandRow.ID, "userId": user.ID,
			"mode": mode, "mediaId": mediaID, "mediaName": mediaDerefName(mediaByID[mediaID]),
			"angleId": angle.ID, "angleLabel": angle.Label, "angleInstruction": angle.Instruction,
			"publishStatus": publishStatus, "skipStandardCheck": skipStandardCheck,
		}
		if mode == "generate" && sourceMaterialStr != "" {
			payload["sourceMaterial"] = sourceMaterialStr
		}
		if mode == "paraphrase" && sourceArticleText != nil {
			payload["sourceArticleText"] = *sourceArticleText
		}
		if campaignID != "" {
			payload["campaignId"] = campaignID
		}
		if connectionID != "" {
			payload["connectionId"] = connectionID
		}
		if scheduledISO != nil {
			payload["scheduledAt"] = scheduledISO
		}
		d.enqueueJobBestEffort(r.Context(), "publication_variant_generation", payload, 3)
		results = append(results, map[string]any{
			"media_id": mediaID, "media_name": mediaByID[mediaID].raw("name"),
			"angle": angle.Label, "status": "queued",
		})
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"batch_id": batchID, "seed": seed, "results": results,
	})
}

// PublicationVariantsList: GET /v1/brands/{brandId}/publications/variants
func (d *Deps) PublicationVariantsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !containsString([]string{"queued", "processing", "completed", "partial", "failed"}, status) {
		d.writeErr(w, errNotFound("Status batch tidak valid."))
		return
	}
	limit := clampInt(q.Get("limit"), 10, 1, 50)
	offset := intParamDef(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	clauses := []string{"b.brand_id = $1"}
	params := []any{brandRow.ID}
	if status != "" {
		params = append(params, status)
		clauses = append(clauses, "b.status = $"+itoa(len(params)))
	}
	where := "WHERE " + strings.Join(clauses, " AND ")
	from := `FROM publication_variant_batches b
	     JOIN brands br ON br.id = b.brand_id
	     JOIN users u ON u.id = b.user_id`

	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count "+from+" "+where, params...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	total := 0
	if countRow != nil {
		total = countRow.intOr("count", 0)
	}
	batchRows, err := d.queryRows(r.Context(), `
		SELECT b.id AS batch_id, b.mode, b.seed, b.status, b.source_document_id, b.created_at, b.completed_at,
		       br.name AS brand_name, br.public_id AS brand_public_id,
		       u.email AS user_email,
		       (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id) AS variant_total,
		       (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'completed') AS variant_done,
		       (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'failed') AS variant_failed
		`+from+`
		`+where+`
		ORDER BY b.created_at DESC
		LIMIT $`+itoa(len(params)+1)+` OFFSET $`+itoa(len(params)+2),
		append(append([]any{}, params...), limit, offset)...)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	variantsByBatch := map[string][]bsRow{}
	if len(batchRows) > 0 {
		ids := make([]string, 0, len(batchRows))
		for _, b := range batchRows {
			ids = append(ids, b.strOr("batch_id", ""))
		}
		variantRows, err := d.queryRows(r.Context(), `
			SELECT v.batch_id, v.id, v.media_id, v.angle_id, v.angle_label, v.status, v.document_id, v.publication_id,
			       v.error_message, v.created_at,
			       m.name AS media_name, d.title AS document_title, p.status AS publication_status
			FROM publication_variants v
			LEFT JOIN media m ON m.id = v.media_id
			LEFT JOIN content_documents d ON d.id = v.document_id
			LEFT JOIN publications p ON p.id = v.publication_id
			WHERE v.batch_id IN (`+placeholders(1, len(ids))+`)
			ORDER BY v.created_at ASC`, stringArgs(ids)...)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		for _, id := range ids {
			variantsByBatch[id] = []bsRow{}
		}
		for _, row := range variantRows {
			key := row.strOr("batch_id", "")
			variantsByBatch[key] = append(variantsByBatch[key], row)
		}
	}

	batches := make([]map[string]any, 0, len(batchRows))
	for _, b := range batchRows {
		key := b.strOr("batch_id", "")
		variants := make([]map[string]any, 0, len(variantsByBatch[key]))
		for _, v := range variantsByBatch[key] {
			variants = append(variants, map[string]any{
				"id": v.raw("id"), "media_id": v.raw("media_id"), "media_name": v.raw("media_name"),
				"angle_id": v.raw("angle_id"), "angle_label": v.raw("angle_label"), "status": v.raw("status"),
				"document_id": v.raw("document_id"), "publication_id": v.raw("publication_id"),
				"document_title": v.raw("document_title"), "publication_status": v.raw("publication_status"),
				"error_message": v.raw("error_message"), "created_at": v.timePtr("created_at"),
			})
		}
		batches = append(batches, map[string]any{
			"batch_id": b.raw("batch_id"), "mode": b.raw("mode"), "seed": b.intOr("seed", 0),
			"status": b.raw("status"), "source_document_id": b.raw("source_document_id"),
			"created_at": b.timePtr("created_at"), "completed_at": b.timePtr("completed_at"),
			"brand_name": b.raw("brand_name"), "brand_public_id": b.raw("brand_public_id"),
			"user_email": b.raw("user_email"),
			"variant_counts": map[string]any{
				"total": b.intOr("variant_total", 0), "done": b.intOr("variant_done", 0), "failed": b.intOr("variant_failed", 0),
			},
			"variants": variants,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": batches, "total": total})
}

// PublicationVariantBatchGet: GET /v1/brands/{brandId}/publications/variants/{batchId}
func (d *Deps) PublicationVariantBatchGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	batchID := chiParam(r, "batchId")
	batch, err := d.queryRow(r.Context(),
		"SELECT id, brand_id, mode, seed, status, source_document_id, source_material, created_at, completed_at FROM publication_variant_batches WHERE id = $1 LIMIT 1", batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if batch == nil || batch.strOr("brand_id", "") != brandRow.ID {
		d.writeErr(w, errNotFound("Batch not found"))
		return
	}
	variants, err := d.queryRows(r.Context(), `
		SELECT v.id, v.media_id, v.angle_id, v.angle_label, v.status, v.document_id, v.publication_id, v.error_message, v.created_at, v.updated_at,
		       m.name AS media_name,
		       d.title AS document_title,
		       p.status AS publication_status
		FROM publication_variants v
		LEFT JOIN media m ON m.id = v.media_id
		LEFT JOIN content_documents d ON d.id = v.document_id
		LEFT JOIN publications p ON p.id = v.publication_id
		WHERE v.batch_id = $1 ORDER BY v.created_at ASC`, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(variants))
	for _, v := range variants {
		items = append(items, map[string]any{
			"id": v.raw("id"), "media_id": v.raw("media_id"), "media_name": v.raw("media_name"),
			"angle_id": v.raw("angle_id"), "angle_label": v.raw("angle_label"), "status": v.raw("status"),
			"document_id": v.raw("document_id"), "publication_id": v.raw("publication_id"),
			"document_title": v.raw("document_title"), "publication_status": v.raw("publication_status"),
			"error_message": v.raw("error_message"), "created_at": v.timePtr("created_at"), "updated_at": v.timePtr("updated_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"batch_id": batch.raw("id"), "mode": batch.raw("mode"), "seed": batch.intOr("seed", 0),
		"status": batch.raw("status"), "source_document_id": batch.raw("source_document_id"),
		"source_material": batch.raw("source_material"),
		"created_at":      batch.timePtr("created_at"), "completed_at": batch.timePtr("completed_at"),
		"variants": items,
	})
}

// ---------- tracking ----------

// PublicationTrackingGet: GET /v1/brands/{brandId}/publications/{publicationId}/tracking
func (d *Deps) PublicationTrackingGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	publication, err := d.queryRow(r.Context(),
		"SELECT id FROM publications WHERE id = $1 AND brand_id = $2 LIMIT 1", publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if publication == nil {
		d.writeErr(w, errNotFound("Publication not found"))
		return
	}
	link, counts, err := d.publicationTrackingSummary(r.Context(), publicationID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	var tracking any
	if link != nil {
		tracking = map[string]any{
			"id": link.strOr("id", ""), "code": link.strOr("code", ""),
			"tracking_path":   "/t/" + link.strOr("code", ""),
			"destination_url": link.raw("destination_url"), "status": link.raw("status"),
			"created_at": link.timePtr("created_at"), "updated_at": link.timePtr("updated_at"),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracking": tracking, "counts": counts})
}

// PublicationTrackingPost: POST /v1/brands/{brandId}/publications/{publicationId}/tracking
func (d *Deps) PublicationTrackingPost(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	body := decodeJSON(r)
	destinationURL, ok := strField(body, "destination_url")
	if ok {
		if len(destinationURL) > 2000 || (destinationURL != "" && !isHTTPURL(destinationURL)) {
			d.writeErr(w, errUnprocessable("Invalid url"))
			return
		}
	} else if raw, present := body["destination_url"]; present && raw != nil {
		d.writeErr(w, errUnprocessable("Invalid url"))
		return
	}
	publication, err := d.queryRow(r.Context(),
		"SELECT id FROM publications WHERE id = $1 AND brand_id = $2 LIMIT 1", publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if publication == nil {
		d.writeErr(w, errNotFound("Publication not found"))
		return
	}
	link, err := d.upsertTrackingLink(r.Context(), brandRow.ID, publicationID, nullIfEmpty(destinationURL), user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"tracking": map[string]any{
			"id": link.strOr("id", ""), "code": link.strOr("code", ""),
			"tracking_path":   "/t/" + link.strOr("code", ""),
			"destination_url": link.raw("destination_url"), "status": link.raw("status"),
			"created_at": link.timePtr("created_at"), "updated_at": link.timePtr("updated_at"),
		},
	})
}

// publicationTrackingSummary: parity publicationTrackingSummary (lib/tracking/service.ts).
func (d *Deps) publicationTrackingSummary(ctx context.Context, publicationID string) (bsRow, map[string]any, error) {
	link, err := d.queryRow(ctx,
		"SELECT id, code, destination_url, status, created_at, updated_at FROM tracking_links WHERE publication_id = $1 LIMIT 1", publicationID)
	if err != nil {
		return nil, nil, err
	}
	counts := map[string]any{}
	for _, t := range trackingEventTypes {
		counts[t] = 0
	}
	rows, err := d.queryRows(ctx,
		"SELECT event_type, COUNT(*)::int AS count FROM tracking_events WHERE publication_id = $1 GROUP BY event_type", publicationID)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		counts[row.strOr("event_type", "")] = row.intOr("count", 0)
	}
	return link, counts, nil
}

// trackingEventTypes: parity TRACKING_EVENT_TYPES (db/models).
var trackingEventTypes = []string{
	"link_click", "article_view", "article_scroll", "outbound_click", "cta_click",
}

// upsertTrackingLink: parity upsertTrackingLink — satu link per publikasi,
// kode 8 char unik di-generate server-side (retry 5x).
func (d *Deps) upsertTrackingLink(ctx context.Context, brandID, publicationID string, destinationURL any, userID string) (bsRow, error) {
	existing, err := d.queryRow(ctx,
		"SELECT id FROM tracking_links WHERE publication_id = $1 LIMIT 1", publicationID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := d.exec(ctx,
			"UPDATE tracking_links SET destination_url = $1, status = 'active', updated_at = NOW() WHERE id = $2",
			destinationURL, existing.strOr("id", "")); err != nil {
			return nil, err
		}
		return d.queryRow(ctx,
			"SELECT id, code, destination_url, status, created_at, updated_at FROM tracking_links WHERE id = $1 LIMIT 1",
			existing.strOr("id", ""))
	}
	for attempt := 0; attempt < 5; attempt++ {
		code := randomTrackingCode()
		taken, err := d.queryRow(ctx, "SELECT id FROM tracking_links WHERE code = $1 LIMIT 1", code)
		if err != nil {
			return nil, err
		}
		if taken != nil {
			continue
		}
		id := newUUID()
		if err := d.exec(ctx,
			"INSERT INTO tracking_links (id, code, publication_id, brand_id, destination_url, created_by) VALUES ($1, $2, $3, $4, $5, $6)",
			id, code, publicationID, brandID, destinationURL, nullIfEmpty(userID)); err != nil {
			return nil, err
		}
		return d.queryRow(ctx,
			"SELECT id, code, destination_url, status, created_at, updated_at FROM tracking_links WHERE id = $1 LIMIT 1", id)
	}
	return nil, errStatus(500, "Could not generate a unique tracking code")
}

// ---------- helper ----------

func mediaDerefName(row bsRow) string {
	if row == nil {
		return "Media"
	}
	if name := row.strOr("name", ""); name != "" {
		return name
	}
	return "Media"
}

func stringArgs(items []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

func asInsufficient(err error, target **insufficientCreditsError) bool {
	if ic, ok := err.(*insufficientCreditsError); ok {
		*target = ic
		return true
	}
	return false
}

// writeCreditError: parity respons 409 kaya kode insufficient_credits.
func writeCreditError(w http.ResponseWriter, cost, available int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"detail":           "Credit tidak mencukupi: butuh " + itoa(cost) + " TC, tersedia " + itoa(available) + " TC. Top Up Tulisio Credit terlebih dahulu.",
		"code":             "insufficient_credits",
		"credit_needed":    cost,
		"credit_available": available,
	})
}

// PublicationReplacementGet: GET /v1/brands/{brandId}/publications/{publicationId}/replacement
//
// Automatic Replacement (fase 10 / fondasi §9): saat mitra menolak publikasi,
// klien meminta media alternatif terbaik. Media yang sudah dicoba untuk dokumen
// yang sama dikecualikan supaya penggantian tidak menarget outlet yang sama dua
// kali. Port lib/media/intelligence.ts suggestReplacement.
func (d *Deps) PublicationReplacementGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicationID := chiParam(r, "publicationId")
	pub, err := d.queryRow(r.Context(), `
		SELECT p.id, p.brand_id, p.document_id, p.media_id, p.client_price, m.category
		FROM publications p LEFT JOIN media m ON m.id = p.media_id
		WHERE p.id = $1 AND p.brand_id = $2 LIMIT 1`, publicationID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if pub == nil {
		d.writeErr(w, errNotFound("Publication not found"))
		return
	}
	if pub.strOr("status", "") != "partner_rejected" {
		d.writeErr(w, errConflict("Replacement is only available for rejected publications"))
		return
	}

	// Media yang sudah dicoba untuk dokumen ini (termasuk media saat ini).
	tried, err := d.queryRows(r.Context(),
		"SELECT media_id FROM publications WHERE document_id = $1 AND media_id IS NOT NULL",
		pub.strOr("document_id", ""))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	exclude := []string{pub.strOr("media_id", "")}
	for _, t := range tried {
		if id, ok := t.str("media_id"); ok && id != "" {
			exclude = append(exclude, id)
		}
	}

	var budget *float64
	if price, ok := pub.num("client_price"); ok && price > 0 {
		budget = &price
	}
	candidates, err := d.bsRecommendMedia(r.Context(), pub.strOr("category", ""), budget, 10, exclude)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	var replacement any
	if len(candidates) > 0 {
		replacement = candidates[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"replacement": replacement,
		"candidates":  candidates,
	})
}
