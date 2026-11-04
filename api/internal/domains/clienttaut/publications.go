package clienttaut

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
)

// Port app/v1/brands/[brandId]/_publication.ts createPublicationRecord +
// lib/pricing/engine.ts (breakdownFor) + lib/publication/state.ts.

type createPublicationInput struct {
	BrandID          string
	UserID           string
	DocumentID       string
	MediaID          string
	PublishStatus    string
	ScheduledAt      string
	CampaignID       string
	FeaturedImageURL string
}

type createdPublicationRecord struct {
	PublicationID string
	Path          string // "queued" | "waiting_partner"
}

// ---------- pricing (port lib/pricing/engine.ts) ----------

const defaultMarkupPercent = 15

var (
	markupMu     sync.Mutex
	markupCached *struct {
		value   float64
		expires time.Time
	}
)

func (d *Deps) getMarkupPercent(ctx context.Context) float64 {
	now := time.Now()
	markupMu.Lock()
	if markupCached != nil && markupCached.expires.After(now) {
		v := markupCached.value
		markupMu.Unlock()
		return v
	}
	markupMu.Unlock()

	value := float64(defaultMarkupPercent)
	var raw *float64
	if err := d.Pool.QueryRow(ctx, "SELECT markup_percent FROM pricing_config WHERE id = 1 LIMIT 1").Scan(&raw); err == nil && raw != nil {
		value = *raw
	}
	markupMu.Lock()
	markupCached = &struct {
		value   float64
		expires time.Time
	}{value: value, expires: now.Add(30 * time.Second)}
	markupMu.Unlock()
	return value
}

type pricingBreakdown struct {
	PartnerPrice  float64 `json:"partner_price"`
	MarkupPercent float64 `json:"markup_percent"`
	MarkupAmount  float64 `json:"markup_amount"`
	ClientPrice   float64 `json:"client_price"`
}

func computeClientPrice(partnerPrice, markupPercent float64) float64 {
	base := partnerPrice
	if base < 0 {
		base = 0
	}
	markup := markupPercent
	if markup < 0 {
		markup = 0
	}
	return round2(base * (1 + markup/100))
}

func breakdownFor(partnerPrice, markupPercent float64) pricingBreakdown {
	partner := partnerPrice
	if partner < 0 {
		partner = 0
	}
	markup := markupPercent
	if markup < 0 {
		markup = 0
	}
	client := computeClientPrice(partner, markup)
	return pricingBreakdown{
		PartnerPrice: partner, MarkupPercent: markup,
		MarkupAmount: round2(client - partner), ClientPrice: client,
	}
}

func round2(v float64) float64 {
	if v < 0 {
		return float64(int(v*100-0.5)) / 100
	}
	return float64(int(v*100+0.5)) / 100
}

// ---------- publication status machine (port lib/publication/state.ts) ----------

var publicationTransitions = map[string][]string{
	"pending":          {"queued", "waiting_partner", "cancelled"},
	"waiting_partner":  {"partner_approved", "partner_rejected", "cancelled"},
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

// assertPublicationTransition: parity assertPublicationTransition
// (PublicationStateError → Conflict 409 di pemanggil).
func assertPublicationTransition(current, next string) error {
	if current == next {
		return nil
	}
	for _, allowed := range publicationTransitions[current] {
		if allowed == next {
			return nil
		}
	}
	return errConflict("Publication status cannot transition from " + current + " to " + next)
}

// ---------- scoped document ----------

func (d *Deps) scopedDocument(ctx context.Context, brandID, documentID string) (rowMap, error) {
	row, err := d.queryRow(ctx, "SELECT * FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1", documentID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Document not found")
	}
	return row, nil
}

// createPublicationRecord: validasi dokumen/media, duplicate guard, moderation
// pre-check, standard check, INSERT publications, lalu gate mitra atau queue.
func (d *Deps) createPublicationRecord(ctx context.Context, in createPublicationInput) (createdPublicationRecord, error) {
	document, err := d.scopedDocument(ctx, in.BrandID, in.DocumentID)
	if err != nil {
		return createdPublicationRecord{}, err
	}
	if document.strOr("status", "") != "approved" {
		return createdPublicationRecord{}, errConflict("Only approved content can be published")
	}

	media, err := d.queryRow(ctx, `
		SELECT m.id, m.name, m.price, m.status, m.partner_id, m.publication_mode, m.integration_type, m.auto_publish, m.website_url,
		       p.status AS partner_status, p.user_id AS partner_user_id, p.auto_publish_enabled,
		       (SELECT client_name FROM brands WHERE id = $1) AS brand_client_name,
		       m.partner_hidden,
		       `+dbpkg.MediaIntegrationReadySQL("m")+` AS integration_ready
		FROM media m LEFT JOIN partners p ON p.id = m.partner_id
		WHERE m.id = $2 LIMIT 1`, in.BrandID, in.MediaID)
	if err != nil {
		return createdPublicationRecord{}, err
	}
	if media == nil {
		return createdPublicationRecord{}, errNotFound("Media not found")
	}
	if media.strOr("status", "") != "active" {
		return createdPublicationRecord{}, errConflict("Media is not active")
	}
	if !media.boolOr("integration_ready", true) {
		return createdPublicationRecord{}, errConflict(dbpkg.MediaNotReadyMessage)
	}
	if media.boolOr("partner_hidden", false) {
		return createdPublicationRecord{}, errConflict(dbpkg.MediaHiddenByPartnerMessage)
	}

	existing, err := d.queryRow(ctx,
		"SELECT id FROM publications WHERE document_id = $1 AND media_id = $2 LIMIT 1", in.DocumentID, in.MediaID)
	if err != nil {
		return createdPublicationRecord{}, err
	}
	if existing != nil {
		return createdPublicationRecord{}, errConflict("This article is already published to this media")
	}

	// Snapshot target koneksi saat pembuatan (partner media).
	partnerID := media.strOr("partner_id", "")
	provider := "wordpress"
	if media.strOr("integration_type", "manual") == "webhook" {
		provider = "webhook"
	}
	var connectionID any
	if partnerID != "" {
		bound, err := d.queryRow(ctx, dbpkg.CMSConnectionForMediaSQL,
			partnerID, provider, in.MediaID, media.strOr("website_url", ""))
		if err != nil {
			return createdPublicationRecord{}, err
		}
		if bound != nil {
			connectionID = bound.strOr("id", "")
		}
	}

	// Content Safety pre-check (best-effort; Conflict tetap dilempar).
	if err := d.publicationModerationPreCheck(ctx, in); err != nil {
		return createdPublicationRecord{}, err
	}

	// Final media standard check (wajib lulus bila media punya standard aktif).
	if err := d.finalStandardCheck(ctx, in.BrandID, in.DocumentID, in.MediaID); err != nil {
		return createdPublicationRecord{}, err
	}

	id := uuid.NewString()
	var scheduledAt any
	if in.ScheduledAt != "" {
		if t, ok := parseISO(in.ScheduledAt); ok {
			scheduledAt = t
		}
	}
	pricing := breakdownFor(media.numOr("price"), d.getMarkupPercent(ctx))
	publicationMode := "draft"
	if media.strOr("publication_mode", "draft") == "publish" {
		publicationMode = "publish"
	}
	featuredImage := absolutizeURL(in.FeaturedImageURL, d.CanonicalAppBase(nil))

	var campaignArg any
	if in.CampaignID != "" {
		campaignArg = in.CampaignID
	}
	err = d.exec(ctx, `
		INSERT INTO publications (id, brand_id, document_id, media_id, status, client_price, partner_price, markup_percent,
		       publish_status, connection_id, scheduled_at, notes, campaign_id, featured_image_url, created_by, order_code)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9, $10, NULL, $11, $12, $13,
		        'ORD-' || to_char(NOW(), 'YYYYMM') || '-' || lpad(nextval('publication_order_seq')::text, 5, '0'))`,
		id, in.BrandID, in.DocumentID, in.MediaID, pricing.ClientPrice, pricing.PartnerPrice, pricing.MarkupPercent,
		publicationMode, connectionID, scheduledAt, campaignArg, featuredImage, in.UserID)
	if err != nil {
		return createdPublicationRecord{}, err
	}

	autoPublish := media.boolOr("auto_publish", false) && media.boolOr("auto_publish_enabled", false)
	if partnerID != "" && media.strOr("partner_status", "") == "active" && !autoPublish {
		if err := assertPublicationTransition("pending", "waiting_partner"); err != nil {
			return createdPublicationRecord{}, err
		}
		if err := d.exec(ctx,
			"UPDATE publications SET status = 'waiting_partner', updated_at = NOW() WHERE id = $1 AND status = 'pending'", id); err != nil {
			return createdPublicationRecord{}, err
		}
		d.publicationWaitingPartnerNotification(ctx, id)
		return createdPublicationRecord{PublicationID: id, Path: "waiting_partner"}, nil
	}

	if err := assertPublicationTransition("pending", "queued"); err != nil {
		return createdPublicationRecord{}, err
	}
	if err := d.exec(ctx,
		"UPDATE publications SET status = 'queued', updated_at = NOW() WHERE id = $1 AND status = 'pending'", id); err != nil {
		return createdPublicationRecord{}, err
	}
	d.enqueuePublicationExecution(ctx, id, in.BrandID, in.ScheduledAt)
	return createdPublicationRecord{PublicationID: id, Path: "queued"}, nil
}

// enqueuePublicationExecution: parity enqueuePublicationExecution (attempts 3).
func (d *Deps) enqueuePublicationExecution(ctx context.Context, publicationID, brandID, scheduledAt string) {
	payload := map[string]any{"publicationId": publicationID, "brandId": brandID}
	var runAt *time.Time
	if scheduledAt != "" {
		if t, ok := parseISO(scheduledAt); ok {
			runAt = &t
		}
	}
	if _, err := d.enqueueJobAt(ctx, "publication_execution", payload, runAt, 3); err != nil {
		// Job bukan bagian dari kontrak respons.
		_ = err
	}
}

// publicationModerationPreCheck: parity blok Content Safety pre-check.
func (d *Deps) publicationModerationPreCheck(ctx context.Context, in createPublicationInput) error {
	docRow, err := d.queryRow(ctx, "SELECT title, slug, meta_description FROM content_documents WHERE id = $1 LIMIT 1", in.DocumentID)
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
	mod := moderation.ModerateContent(ctx, d.Pool, moderation.Input{
		Title: title, Slug: slug, MetaDescription: meta, Content: content,
		BrandID: &in.BrandID, UserID: &in.UserID,
	})

	docApproved := false
	if audit, err := d.queryRow(ctx,
		"SELECT review_decision FROM content_moderation_audits WHERE entity_id = $1 ORDER BY created_at DESC LIMIT 1", in.DocumentID); err == nil && audit != nil {
		docApproved = audit.strOr("review_decision", "") == "approved"
	}

	if mod.Status == "blocked" {
		return errConflict(moderation.ClientSafeMessage("blocked"))
	}
	if mod.Status == "review_required" && !docApproved {
		return errConflict(moderation.ClientSafeMessage("review_required"))
	}
	moderation.RecordAudit(ctx, d.Pool, "document", in.DocumentID, mod, &in.BrandID)
	return nil
}

// finalStandardCheck: parity finalStandardCheck — null bila media tanpa standard aktif.
func (d *Deps) finalStandardCheck(ctx context.Context, brandID, documentID, mediaID string) error {
	standard, err := d.mediaStandard(ctx, mediaID)
	if err != nil {
		return nil
	}
	if standard == nil {
		return nil
	}
	article, err := d.articleContent(ctx, brandID, documentID)
	if err != nil {
		return err
	}
	media, err := d.queryRow(ctx, "SELECT name FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		return err
	}
	var mediaName any
	if media != nil && media.has("name") {
		mediaName = media.strOr("name", "")
	}
	result := runStandardCheck(*standard, article, mediaName)
	if result.Status == "fail" {
		name := "media"
		if s, ok := mediaName.(string); ok && s != "" {
			name = s
		}
		return errConflict("Artikel belum memenuhi standar editorial " + name + ": " +
			strings.Join(capStrings(result.Failed, 3), "; ") + ". Perbaiki artikel atau jalankan standard check untuk detail.")
	}
	return nil
}

func (d *Deps) mediaStandard(ctx context.Context, mediaID string) (*mediaStandard, error) {
	row, err := d.queryRow(ctx,
		`SELECT id, media_id, min_words, max_words, journalistic_style, hard_selling_tolerance, quote_required,
		        min_quotes, image_required, category_required, required_info, forbidden_content, status
		 FROM media_content_standards WHERE media_id = $1 AND status = 'active' LIMIT 1`, mediaID)
	if err != nil || row == nil {
		return nil, err
	}
	return &mediaStandard{
		MediaID:           row.strOr("media_id", ""),
		MinWords:          row.intOr("min_words", 0),
		MaxWords:          row.intOr("max_words", 0),
		JournalisticStyle: row.strOr("journalistic_style", "news"),
		HardSellingTol:    row.strOr("hard_selling_tolerance", "low"),
		QuoteRequired:     row.boolOr("quote_required", false),
		MinQuotes:         row.intOr("min_quotes", 0),
		ImageRequired:     row.boolOr("image_required", false),
		CategoryRequired:  row.boolOr("category_required", false),
		RequiredInfo:      jsonStringListFromAny(jsonField(row, "required_info")),
		ForbiddenContent:  jsonStringListFromAny(jsonField(row, "forbidden_content")),
	}, nil
}

func (d *Deps) articleContent(ctx context.Context, brandID, documentID string) (string, error) {
	document, err := d.queryRow(ctx,
		"SELECT id, title, current_version FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1", documentID, brandID)
	if err != nil || document == nil {
		return "", err
	}
	version, err := d.queryRow(ctx,
		"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		documentID, document.intOr("current_version", 1))
	if err != nil || version == nil {
		return "", err
	}
	return version.strOr("content", ""), nil
}

func jsonStringListFromAny(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}
