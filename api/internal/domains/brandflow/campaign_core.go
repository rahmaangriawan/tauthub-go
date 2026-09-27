package brandflow

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// campaign_core.go: port _campaign.ts + lib/campaign/report.ts +
// _publication.ts createPublicationRecord.

// CAMPAIGN_AGGREGATE_SELECT: salinan persis lib/campaign/report.ts.
const campaignAggregateSelect = `
  (SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status <> 'cancelled') AS publication_count,
  (SELECT COUNT(*) FROM campaign_media_items cmi
    WHERE cmi.campaign_id = c.id AND COALESCE(cmi.variant_document_id, cmi.source_document_id) IS NOT NULL AND cmi.status <> 'cancelled') AS articles_created,
  (SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status = 'published') AS published_count,
  (SELECT COUNT(DISTINCT p.media_id) FROM publications p WHERE p.campaign_id = c.id AND p.status = 'published') AS media_published_count,
  (SELECT STRING_AGG(DISTINCT m.name, ' · ') FROM publications p JOIN media m ON m.id = p.media_id
    WHERE p.campaign_id = c.id AND p.status = 'published') AS published_media_names,
  (SELECT COALESCE(SUM(t.used_tc), 0) FROM (
      SELECT COALESCE(SUM(CASE WHEN l.ref_type = 'capture' THEN l.amount_tc ELSE 0 END), 0)
             + GREATEST(-COALESCE(SUM(l.amount_tc), 0), 0) AS used_tc
      FROM credit_ledger_entries l
      WHERE l.brand_id = c.brand_id AND l.ref_type IN ('reserve', 'release', 'capture')
        AND EXISTS (SELECT 1 FROM publications p WHERE p.id = l.ref_id AND p.campaign_id = c.id)
      GROUP BY l.ref_id
   ) t) AS tc_used
`

// campaignSelect: SELECT c.* + agregat.
const campaignSelect = `SELECT c.*, ` + campaignAggregateSelect + ` FROM campaigns c`

var campaignStatuses = []string{"draft", "active", "paused", "completed", "cancelled"}

func validCampaignStatus(s string) bool {
	for _, v := range campaignStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// scopedCampaign: lookup kampanye ter-scope brand; NotFound bila tidak ada.
func (d *Deps) scopedCampaign(ctx context.Context, brandID, campaignID string) (bsRow, error) {
	row, err := d.queryRow(ctx, campaignSelect+" WHERE c.id = $1 AND c.brand_id = $2 LIMIT 1", campaignID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Campaign not found")
	}
	return row, nil
}

// campaignJSON: port campaignJson (_campaign.ts).
func campaignJSON(row bsRow) map[string]any {
	var standardSnapshot any
	if row.has("editorial_standard_snapshot") {
		standardSnapshot = row.jsonField("editorial_standard_snapshot")
	}
	flowStatus := row.raw("campaign_flow_status")
	if flowStatus == nil {
		flowStatus = row.raw("flow_status")
	}
	return map[string]any{
		"id":                          row.raw("id"),
		"brand_id":                    row.raw("brand_id"),
		"name":                        row.raw("name"),
		"description":                 row.raw("description"),
		"status":                      row.raw("status"),
		"article_quota":               row.intOr("article_quota", 0),
		"budget":                      row.numOr("budget"),
		"order_code":                  row.raw("order_code"),
		"starts_at":                   row.timePtr("starts_at"),
		"ends_at":                     row.timePtr("ends_at"),
		"publication_count":           row.intOr("publication_count", 0),
		"articles_created":            row.intOr("articles_created", 0),
		"published_count":             row.intOr("published_count", 0),
		"media_published_count":       row.intOr("media_published_count", 0),
		"published_media_names":       row.raw("published_media_names"),
		"tc_used":                     row.intOr("tc_used", 0),
		"content_intake_id":           row.raw("content_intake_id"),
		"campaign_flow_status":        flowStatus,
		"media_type":                  row.raw("media_type"),
		"selected_media_id":           row.raw("selected_media_id"),
		"editorial_standard_snapshot": standardSnapshot,
		"media_locked_at":             row.timePtr("media_locked_at"),
		"created_by":                  row.raw("created_by"),
		"created_at":                  row.timePtr("created_at"),
		"updated_at":                  row.timePtr("updated_at"),
	}
}

// assertCampaignQuota: parity assertCampaignQuota (_campaign.ts).
func assertCampaignQuota(campaign bsRow, additional int) error {
	quota := campaign.intOr("article_quota", 0)
	if quota <= 0 {
		return nil
	}
	used := campaign.intOr("publication_count", 0)
	if used+additional > quota {
		return errConflict("Campaign quota exceeded: " + itoa(used) + " of " + itoa(quota) +
			" publications used, " + itoa(additional) + " more requested")
	}
	return nil
}

// ---------- publication JSON (port toPublicationJson) ----------

var publicationSelect = `
  SELECT p.*, d.title AS document_title, d.slug AS document_slug,
         m.name AS media_name, m.category AS media_category, m.website_url AS media_website_url,
         COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = m.id), m.category) AS media_categories_csv,
         (SELECT pj.progress FROM publication_jobs pj WHERE pj.publication_id = p.id ORDER BY pj.created_at DESC LIMIT 1) AS publish_progress
  FROM publications p
  JOIN content_documents d ON d.id = p.document_id
  JOIN media m ON m.id = p.media_id
`

// prAnalysisFromRow: parity prAnalysisFromRow (_publication.ts).
func prAnalysisFromRow(row bsRow) map[string]any {
	var message any
	if row.has("pr_message_score") || row.has("pr_message_status") {
		message = map[string]any{
			"score":  row.numPtr("pr_message_score"),
			"status": row.raw("pr_message_status"),
		}
	}
	var sentiment any
	if row.has("pr_sentiment") {
		sentiment = map[string]any{
			"sentiment":  row.raw("pr_sentiment"),
			"confidence": row.numPtr("pr_sentiment_confidence"),
		}
	}
	var verification any
	if row.has("pr_verification_status") {
		verification = map[string]any{
			"status":           row.raw("pr_verification_status"),
			"compliance_score": row.numPtr("pr_verification_compliance_score"),
			"verified_at":      row.timePtr("pr_verification_verified_at"),
		}
	}
	return map[string]any{
		"message":      message,
		"sentiment":    sentiment,
		"verification": verification,
	}
}

// mediaCategories: kategori media dari CSV (pemisah \u0001).
func mediaCategories(row bsRow) []string {
	if csv := row.strOr("media_categories_csv", ""); csv != "" {
		out := []string{}
		for _, s := range strings.Split(csv, "\u0001") {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	if cat := row.strPtr("media_category"); cat != nil && *cat != "" {
		return []string{*cat}
	}
	return []string{}
}

// featuredImageFromRow: parity logika featured_image_url toPublicationJson.
func featuredImageFromRow(row bsRow) any {
	var editsImage any
	if edits, ok := row.jsonField("partner_draft_edits").(map[string]any); ok {
		if s, ok := edits["featured_image_url"].(string); ok && strings.TrimSpace(s) != "" {
			editsImage = s
		}
	}
	if editsImage != nil {
		return editsImage
	}
	return row.raw("featured_image_url")
}

// toPublicationJSON: port toPublicationJson (_publication.ts).
func toPublicationJSON(row bsRow, attemptCount int) map[string]any {
	return map[string]any{
		"id":                    row.raw("id"),
		"brand_id":              row.raw("brand_id"),
		"document_id":           row.raw("document_id"),
		"media_id":              row.raw("media_id"),
		"status":                row.raw("status"),
		"client_price":          row.numOr("client_price"),
		"publish_status":        row.raw("publish_status"),
		"connection_id":         row.raw("connection_id"),
		"scheduled_at":          row.timePtr("scheduled_at"),
		"published_at":          row.timePtr("published_at"),
		"published_url":         row.raw("published_url"),
		"notes":                 row.raw("notes"),
		"cancel_reason":         row.raw("cancel_reason"),
		"campaign_id":           row.raw("campaign_id"),
		"order_code":            row.raw("order_code"),
		"auto_replace_disabled": row.boolOr("auto_replace_disabled", false),
		"created_by":            row.raw("created_by"),
		"created_at":            row.timePtr("created_at"),
		"updated_at":            row.timePtr("updated_at"),
		"document_title":        row.raw("document_title"),
		"document_slug":         row.raw("document_slug"),
		"media_name":            row.raw("media_name"),
		"media_category":        row.raw("media_category"),
		"media_categories":      mediaCategories(row),
		"media_website_url":     row.raw("media_website_url"),
		"media_status":          row.raw("media_status"),
		"featured_image_url":    featuredImageFromRow(row),
		"rss_import_status":     row.strOr("rss_import_status", "not_imported"),
		"rss_imported_at":       row.timePtr("rss_imported_at"),
		"rss_published_at":      row.timePtr("rss_published_at"),
		"attempt_count":         attemptCount,
		"publish_progress":      row.numPtr("publish_progress"),
		"pr_analysis":           prAnalysisFromRow(row),
	}
}

// scopedPublication: lookup publikasi ter-scope brand (id ATAU public_id brand).
func (d *Deps) scopedPublication(ctx context.Context, brandID, publicationID string) (bsRow, error) {
	row, err := d.queryRow(ctx,
		publicationSelect+" WHERE p.id = $1 AND (p.brand_id = $2 OR p.brand_id = (SELECT id FROM brands WHERE id = $2 OR public_id = $2 OR slug = $2 LIMIT 1)) LIMIT 1",
		publicationID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Publication not found")
	}
	return row, nil
}

// publicationJSON: parity publicationJson (row + attempt_count).
func (d *Deps) publicationJSON(ctx context.Context, brandID, publicationID string) (map[string]any, error) {
	row, err := d.scopedPublication(ctx, brandID, publicationID)
	if err != nil {
		return nil, err
	}
	countRow, err := d.queryRow(ctx,
		"SELECT COUNT(*) AS count FROM publication_attempts WHERE publication_id = $1", publicationID)
	if err != nil {
		return nil, err
	}
	count := 0
	if countRow != nil {
		count = countRow.intOr("count", 0)
	}
	return toPublicationJSON(row, count), nil
}

// ---------- createPublicationRecord (port _publication.ts) ----------

type createPublicationInput struct {
	BrandID           string
	UserID            string
	DocumentID        string
	MediaID           string
	ConnectionID      *string
	PublishStatus     string
	ScheduledAt       *time.Time
	Notes             *string
	CampaignID        *string
	SkipStandardCheck bool
	FeaturedImageURL  *string
	// ConnectionValidated: pemanggil sudah memvalidasi connection_id (parity
	// endpoint publikasi yang memeriksa cms_connections sendiri).
	ConnectionValidated bool
}

type createdPublication struct {
	Row  bsRow
	Path string // "queued" | "waiting_partner"
}

// scopedDocument: cari dokumen milik brand.
func (d *Deps) scopedDocument(ctx context.Context, brandID, documentID string) (bsRow, error) {
	row, err := d.queryRow(ctx,
		"SELECT * FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1", documentID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Document not found")
	}
	return row, nil
}

// moderationPreCheck: parity blok Content Safety pre-check (best-effort kecuali Conflict).
func (d *Deps) moderationPreCheck(ctx context.Context, in createPublicationInput) error {
	docRow, err := d.queryRow(ctx,
		"SELECT title, slug, meta_description FROM content_documents WHERE id = $1 LIMIT 1", in.DocumentID)
	if err != nil || docRow == nil {
		return nil
	}
	ver, err := d.queryRow(ctx,
		"SELECT content FROM content_versions WHERE document_id = $1 ORDER BY version DESC LIMIT 1", in.DocumentID)
	if err != nil {
		ver = nil
	}
	var title, slug, meta *string
	if v := docRow.strOr("title", ""); v != "" {
		title = &v
	}
	if v := docRow.strOr("slug", ""); v != "" {
		slug = &v
	}
	if v := docRow.strOr("meta_description", ""); v != "" {
		meta = &v
	}
	var content *string
	if ver != nil {
		if c := truncateString(ver.strOr("content", ""), 5000); c != "" {
			content = &c
		}
	}
	mod := moderateContent(ctx, d.Pool, moderationInput{
		Title: title, Slug: slug, MetaDescription: meta, Content: content,
		BrandID: &in.BrandID, UserID: &in.UserID,
	})

	docApproved := false
	if audit, err := d.queryRow(ctx,
		"SELECT review_decision FROM content_moderation_audits WHERE entity_id = $1 ORDER BY created_at DESC LIMIT 1", in.DocumentID); err == nil && audit != nil {
		docApproved = audit.strOr("review_decision", "") == "approved"
	}
	if mod.Status == "blocked" {
		return errConflict(moderationClientSafeMessage("blocked"))
	}
	if mod.Status == "review_required" && !docApproved {
		return errConflict(moderationClientSafeMessage("review_required"))
	}
	recordModerationAudit(ctx, d.Pool, "document", in.DocumentID, mod, &in.BrandID)
	return nil
}

// createPublicationRecord: validasi dokumen/media, duplicate guard, moderation
// pre-check, standard check, INSERT publications, lalu gate mitra atau queue.
func (d *Deps) createPublicationRecord(ctx context.Context, in createPublicationInput) (createdPublication, error) {
	document, err := d.scopedDocument(ctx, in.BrandID, in.DocumentID)
	if err != nil {
		return createdPublication{}, err
	}
	if document.strOr("status", "") != "approved" {
		return createdPublication{}, errConflict("Only approved content can be published")
	}

	media, err := d.queryRow(ctx, `
		SELECT m.id, m.name, m.price, m.status, m.partner_id, m.publication_mode, m.integration_type, m.auto_publish,
		       p.status AS partner_status, p.user_id AS partner_user_id, p.auto_publish_enabled,
		       (SELECT client_name FROM brands WHERE id = $1) AS brand_client_name
		FROM media m LEFT JOIN partners p ON p.id = m.partner_id
		WHERE m.id = $2 LIMIT 1`, in.BrandID, in.MediaID)
	if err != nil {
		return createdPublication{}, err
	}
	if media == nil {
		return createdPublication{}, errNotFound("Media not found")
	}
	if media.strOr("status", "") != "active" {
		return createdPublication{}, errConflict("Media is not active")
	}

	existing, err := d.queryRow(ctx,
		"SELECT id FROM publications WHERE document_id = $1 AND media_id = $2 LIMIT 1", in.DocumentID, in.MediaID)
	if err != nil {
		return createdPublication{}, err
	}
	if existing != nil {
		return createdPublication{}, errConflict("This article is already published to this media")
	}

	// Snapshot target koneksi saat pembuatan (partner media).
	connectionID := in.ConnectionID
	partnerID := media.strOr("partner_id", "")
	if connectionID == nil && partnerID != "" {
		provider := "wordpress"
		if media.strOr("integration_type", "manual") == "webhook" {
			provider = "webhook"
		}
		bound, err := d.queryRow(ctx,
			"SELECT id FROM cms_connections WHERE partner_id = $1 AND provider = $2 AND status = 'active' AND website_id = $3 LIMIT 1",
			partnerID, provider, in.MediaID)
		if err != nil {
			return createdPublication{}, err
		}
		if bound != nil {
			if id := bound.strOr("id", ""); id != "" {
				connectionID = &id
			}
		}
	}

	if err := d.moderationPreCheck(ctx, in); err != nil {
		return createdPublication{}, err
	}
	if !in.SkipStandardCheck {
		if err := d.finalStandardCheck(ctx, in.BrandID, in.DocumentID, in.MediaID); err != nil {
			return createdPublication{}, err
		}
	}

	id := uuid.NewString()
	var scheduledAt any
	if in.ScheduledAt != nil {
		scheduledAt = *in.ScheduledAt
	}
	pricing := breakdownFor(media.numOr("price"), d.getMarkupPercent(ctx))
	publicationMode := "draft"
	if media.strOr("publication_mode", "draft") == "publish" {
		publicationMode = "publish"
	}
	featuredImage := d.absolutizeUpload(in.FeaturedImageURL)
	var campaignArg any
	if in.CampaignID != nil && *in.CampaignID != "" {
		campaignArg = *in.CampaignID
	}
	var notesArg any
	if in.Notes != nil && *in.Notes != "" {
		notesArg = *in.Notes
	}
	var connectionArg any
	if connectionID != nil {
		connectionArg = *connectionID
	}
	err = d.exec(ctx, `
		INSERT INTO publications (id, brand_id, document_id, media_id, status, client_price, partner_price, markup_percent,
		       publish_status, connection_id, scheduled_at, notes, campaign_id, featured_image_url, created_by, order_code)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
		        'ORD-' || to_char(NOW(), 'YYYYMM') || '-' || lpad(nextval('publication_order_seq')::text, 5, '0'))`,
		id, in.BrandID, in.DocumentID, in.MediaID, pricing.ClientPrice, pricing.PartnerPrice, pricing.MarkupPercent,
		publicationMode, connectionArg, scheduledAt, notesArg, campaignArg, featuredImage, in.UserID)
	if err != nil {
		return createdPublication{}, err
	}

	autoPublish := media.boolOr("auto_publish", false) && media.boolOr("auto_publish_enabled", false)
	if partnerID != "" && media.strOr("partner_status", "") == "active" && !autoPublish {
		if err := assertPublicationTransition("pending", "waiting_partner"); err != nil {
			return createdPublication{}, err
		}
		if err := d.exec(ctx,
			"UPDATE publications SET status = 'waiting_partner', updated_at = NOW() WHERE id = $1 AND status = 'pending'", id); err != nil {
			return createdPublication{}, err
		}
		d.publicationWaitingPartnerNotification(ctx, id)
		row, err := d.scopedPublication(ctx, in.BrandID, id)
		if err != nil {
			return createdPublication{}, err
		}
		return createdPublication{Row: row, Path: "waiting_partner"}, nil
	}

	if err := assertPublicationTransition("pending", "queued"); err != nil {
		return createdPublication{}, err
	}
	if err := d.exec(ctx,
		"UPDATE publications SET status = 'queued', updated_at = NOW() WHERE id = $1 AND status = 'pending'", id); err != nil {
		return createdPublication{}, err
	}
	d.enqueuePublicationExecution(ctx, id, in.BrandID, in.ScheduledAt)
	row, err := d.scopedPublication(ctx, in.BrandID, id)
	if err != nil {
		return createdPublication{}, err
	}
	return createdPublication{Row: row, Path: "queued"}, nil
}

// enqueuePublicationExecution: parity enqueuePublicationExecution (attempts 3).
func (d *Deps) enqueuePublicationExecution(ctx context.Context, publicationID, brandID string, scheduledAt *time.Time) {
	payload := map[string]any{"publicationId": publicationID, "brandId": brandID}
	if _, err := d.enqueueJobAt(ctx, "publication_execution", payload, scheduledAt, 3); err != nil {
		// Job bukan bagian dari kontrak respons.
		_ = err
	}
}

// recordPublicationAttempt: parity recordPublicationAttempt — attempt_number
// dihitung di dalam INSERT (MAX+1).
func (d *Deps) recordPublicationAttempt(ctx context.Context, publicationID, status string, errorCode, errorMessage *string, startedAt, completedAt time.Time) error {
	if err := d.exec(ctx,
		`INSERT INTO publication_attempts (id, publication_id, attempt_number, status, error_code, error_message, started_at, completed_at)
		 VALUES (gen_random_uuid(), $1, (SELECT COALESCE(MAX(attempt_number), 0) + 1 FROM publication_attempts WHERE publication_id = $1), $2, $3, $4, $5, $6)`,
		publicationID, status, errorCode, errorMessage, startedAt, completedAt); err != nil {
		return err
	}
	if status == "published" {
		d.publicationPublishedNotification(ctx, publicationID)
		d.enqueueJobBestEffort(ctx, "pr_analysis", map[string]any{"publicationId": publicationID}, 2)
		d.enqueueJobBestEffort(ctx, "pr_verify", map[string]any{"publicationId": publicationID}, 2)
		row, _ := d.queryRow(ctx, "SELECT brand_id FROM publications WHERE id = $1 LIMIT 1", publicationID)
		if row != nil {
			d.captureCredit(ctx, row.strOr("brand_id", ""), publicationID, "system")
		}
	}
	if status == "failed" {
		d.publicationFailedNotification(ctx, publicationID)
	}
	return nil
}

// absolutizeUpload: parity absolutizeUrl — path relatif "/uploads/..." → origin
// publik platform (PUBLIC_APP_URL / canonicalAppBase).
func (d *Deps) absolutizeUpload(raw *string) any {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "//") {
		return value
	}
	base := strings.TrimRight(d.canonicalAppBase(), "/")
	if base == "" {
		return value
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return base + value
}

// canonicalAppBase: parity canonicalAppBase — PUBLIC_APP_URL env / APP_URL.
func (d *Deps) canonicalAppBase() string {
	if d.Cfg == nil {
		return ""
	}
	if v := strings.TrimSpace(d.Cfg.PublicAppURL); v != "" {
		return v
	}
	return ""
}

// auditPublication: helper audit publikasi.
func (d *Deps) auditPublication(ctx context.Context, brandID, userID, action, publicationID string) {
	d.audit(ctx, brandID, userID, action, "publication", publicationID)
}

// marshalJSON: JSON kompak untuk parameter jsonb.
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
