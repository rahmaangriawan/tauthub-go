package brandflow

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// campaign_flow_handlers.go: port campaigns/[campaignId]/{flow,publications} +
// flow/{analyze,article-regen,checkout,media,media-type,media-items}.

const regenCostTC = 5

// ---------- GET / POST /v1/brands/{brandId}/campaigns/{campaignId}/flow ----------

// CampaignFlowGet: GET /v1/brands/{brandId}/campaigns/{campaignId}/flow
func (d *Deps) CampaignFlowGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, chiParam(r, "campaignId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := campaign.strOr("id", "")
	flowStatus := campaign.strOr("campaign_flow_status", "intake")
	intakeID := campaign.strOr("content_intake_id", "")

	var intake any
	if intakeID != "" {
		intakeRow, err := d.queryRow(r.Context(),
			"SELECT id, brand_id, mode, content_type_code, topic, answers, article_text, article_filename, key_messages, readiness_score, readiness_checks, recommendation, status, job_id, created_at, updated_at FROM content_intakes WHERE id = $1 AND brand_id = $2 LIMIT 1",
			intakeID, brandRow.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if intakeRow != nil {
			intake = map[string]any{
				"id":                intakeRow.raw("id"),
				"brand_id":          intakeRow.raw("brand_id"),
				"mode":              intakeRow.raw("mode"),
				"content_type_code": intakeRow.raw("content_type_code"),
				"topic":             intakeRow.raw("topic"),
				"answers":           jsonOr(intakeRow.jsonField("answers"), map[string]any{}),
				"article_text":      intakeRow.raw("article_text"),
				"article_filename":  intakeRow.raw("article_filename"),
				"key_messages":      jsonOr(intakeRow.jsonField("key_messages"), []any{}),
				"readiness_score":   intakeRow.raw("readiness_score"),
				"readiness_checks":  jsonOr(intakeRow.jsonField("readiness_checks"), []any{}),
				"recommendation":    jsonOr(intakeRow.jsonField("recommendation"), nil),
				"status":            intakeRow.raw("status"),
				"job_id":            intakeRow.raw("job_id"),
				"created_at":        intakeRow.timePtr("created_at"),
				"updated_at":        intakeRow.timePtr("updated_at"),
			}
		}
	}

	mediaItems, err := d.queryRows(r.Context(), `
		SELECT cmi.id, cmi.media_id, cmi.variant_type, cmi.angle_label,
		       cmi.source_document_id, cmi.variant_document_id,
		       cmi.tc_cost, cmi.status, cmi.validation, cmi.thumbnail_url,
		       m.name AS media_name, m.category AS media_category,
		       m.website_url AS media_website_url, m.price AS media_price,
		       m.integration_type AS media_integration_type, m.dewan_pers_verified AS media_dewan_pers_verified,
		       (SELECT p.status FROM publications p
		         WHERE p.media_id = cmi.media_id AND p.campaign_id = cmi.campaign_id
		         ORDER BY p.created_at DESC LIMIT 1) AS publication_status,
		       (SELECT p.published_url FROM publications p
		         WHERE p.media_id = cmi.media_id AND p.campaign_id = cmi.campaign_id
		         ORDER BY p.created_at DESC LIMIT 1) AS publication_url,
		       (SELECT p.id FROM publications p
		         WHERE p.media_id = cmi.media_id AND p.campaign_id = cmi.campaign_id
		         ORDER BY p.created_at DESC LIMIT 1) AS publication_id,
		       (SELECT p.order_code FROM publications p
		         WHERE p.media_id = cmi.media_id AND p.campaign_id = cmi.campaign_id
		         ORDER BY p.created_at DESC LIMIT 1) AS publication_order_code,
		       (SELECT tl.code FROM publications p
		         JOIN tracking_links tl ON tl.publication_id = p.id
		         WHERE p.media_id = cmi.media_id AND p.campaign_id = cmi.campaign_id
		         ORDER BY p.created_at DESC LIMIT 1) AS tracking_code
		FROM campaign_media_items cmi
		JOIN media m ON m.id = cmi.media_id
		WHERE cmi.campaign_id = $1
		ORDER BY cmi.created_at ASC`, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	docCache := map[string]bsRow{}
	docJSON := func(documentID string) any {
		if documentID == "" {
			return nil
		}
		if cached, ok := docCache[documentID]; ok {
			return map[string]any{
				"title":            cached.strOr("title", ""),
				"meta_title":       cached.raw("meta_title"),
				"meta_description": cached.raw("meta_description"),
				"content":          cached.strOr("content", ""),
			}
		}
		return nil
	}
	for _, item := range mediaItems {
		for _, docID := range []string{item.strOr("source_document_id", ""), item.strOr("variant_document_id", "")} {
			if docID == "" {
				continue
			}
			if _, ok := docCache[docID]; ok {
				continue
			}
			doc, err := d.documentContent(r.Context(), brandRow.ID, docID)
			if err != nil {
				d.writeErr(w, err)
				return
			}
			if doc != nil {
				docCache[docID] = doc
			}
		}
	}

	wallet := d.getWallet(r.Context(), brandRow.ID)
	tcConfig := d.getTcConfig(r.Context())
	totalTcCost := 0
	for _, item := range mediaItems {
		totalTcCost += item.intOr("tc_cost", 0)
	}

	itemsJSON := make([]map[string]any, 0, len(mediaItems))
	for _, item := range mediaItems {
		itemsJSON = append(itemsJSON, map[string]any{
			"id":                        item.raw("id"),
			"media_id":                  item.raw("media_id"),
			"media_name":                item.raw("media_name"),
			"media_category":            item.raw("media_category"),
			"media_website_url":         item.raw("media_website_url"),
			"media_price":               item.numOr("media_price"),
			"media_integration_type":    item.strOr("media_integration_type", "manual"),
			"media_dewan_pers_verified": item.boolOr("media_dewan_pers_verified", false),
			"variant_type":              item.strOr("variant_type", "original"),
			"angle_label":               item.raw("angle_label"),
			"source_document_id":        item.raw("source_document_id"),
			"variant_document_id":       item.raw("variant_document_id"),
			"source_document":           docJSON(item.strOr("source_document_id", "")),
			"variant_document":          docJSON(item.strOr("variant_document_id", "")),
			"thumbnail_url":             item.raw("thumbnail_url"),
			"tc_cost":                   item.intOr("tc_cost", 0),
			"status":                    item.strOr("status", "pending"),
			"publication_status":        item.raw("publication_status"),
			"publication_url":           item.raw("publication_url"),
			"publication_id":            item.raw("publication_id"),
			"publication_order_code":    item.raw("publication_order_code"),
			"tracking_code":             item.raw("tracking_code"),
			"validation":                jsonOr(item.jsonField("validation"), map[string]any{}),
		})
	}

	var walletJSON any
	if wallet != nil {
		walletJSON = map[string]any{
			"available_tc": wallet.AvailableTC,
			"reserved_tc":  wallet.ReservedTC,
			"captured_tc":  wallet.CapturedTC,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaign":      campaignJSON(campaign),
		"flow_status":   flowStatus,
		"intake":        intake,
		"media_items":   itemsJSON,
		"wallet":        walletJSON,
		"tc_config":     map[string]any{"rate_idr": tcConfig.RateIDR},
		"total_tc_cost": totalTcCost,
	})
}

// CampaignFlowPost: POST /v1/brands/{brandId}/campaigns/{campaignId}/flow
func (d *Deps) CampaignFlowPost(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)

	if newArticle, ok := boolField(body, "new_article"); ok && newArticle {
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET content_intake_id = NULL, campaign_flow_status = 'intake', updated_at = NOW() WHERE id = $1", campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
		d.audit(r.Context(), brandRow.ID, user.ID, "campaign.new_article", "campaign", campaignID)
		updated, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"campaign":    campaignJSON(updated),
			"flow_status": "intake",
			"intake":      nil,
		})
		return
	}

	updates := []string{}
	params := []any{}
	if intakeID, ok := strField(body, "intake_id"); ok && intakeID != "" {
		row, err := d.queryRow(r.Context(),
			"SELECT id FROM content_intakes WHERE id = $1 AND brand_id = $2 LIMIT 1", intakeID, brandRow.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if row == nil {
			d.writeErr(w, errStatus(500, "Intake not found"))
			return
		}
		updates = append(updates, "content_intake_id = $"+itoa(len(params)+1))
		params = append(params, intakeID)
	}
	if flowStatus, ok := strField(body, "flow_status"); ok && flowStatus != "" {
		currentStatus := campaign.strOr("campaign_flow_status", "intake")
		allowed := flowTransitions[currentStatus]
		valid := false
		for _, a := range allowed {
			if a == flowStatus {
				valid = true
			}
		}
		if !valid {
			d.writeErr(w, errStatus(500, "Invalid flow transition: "+currentStatus+" → "+flowStatus))
			return
		}
		if flowStatus == "final_review" && currentStatus == "media_selection" {
			if _, err := d.assignDocumentsToItems(r.Context(), brandRow.ID, campaignID, campaign); err != nil {
				// Best-effort seperti app lama (try/catch kosong).
				_ = err
			}
		}
		if flowStatus == "intake" && currentStatus == "content" && campaign.strOr("content_intake_id", "") != "" {
			if err := d.exec(r.Context(),
				"UPDATE content_intakes SET status = 'draft', job_id = NULL, updated_at = NOW() WHERE id = $1 AND brand_id = $2 AND status = 'job_created'",
				campaign.strOr("content_intake_id", ""), brandRow.ID); err != nil {
				d.writeErr(w, err)
				return
			}
		}
		updates = append(updates, "campaign_flow_status = $"+itoa(len(params)+1))
		params = append(params, flowStatus)
	}

	if len(updates) > 0 {
		updates = append(updates, "updated_at = NOW()")
		params = append(params, campaignID)
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET "+strings.Join(updates, ", ")+" WHERE id = $"+itoa(len(params)), params...); err != nil {
			d.writeErr(w, err)
			return
		}
		d.audit(r.Context(), brandRow.ID, user.ID, "campaign.flow_updated", "campaign", campaignID)
	}

	updated, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaign":    campaignJSON(updated),
		"flow_status": updated.strOr("campaign_flow_status", "intake"),
	})
}

// ---------- flow/media-type ----------

// CampaignFlowMediaType: POST /v1/brands/{brandId}/campaigns/{campaignId}/flow/media-type
func (d *Deps) CampaignFlowMediaType(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	if _, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	mediaType, ok := strField(body, "media_type")
	if !ok || (mediaType != "blogger" && mediaType != "mainstream") {
		d.writeErr(w, errStatusCode422Message("Invalid option: expected one of \"blogger\"|\"mainstream\""))
		return
	}
	if err := d.exec(r.Context(), "UPDATE campaigns SET media_type = $1, updated_at = NOW() WHERE id = $2", mediaType, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "campaign.media_type_selected", "campaign", campaignID)
	writeJSON(w, http.StatusOK, map[string]any{"media_type": mediaType})
}

// ---------- flow/media ----------

// CampaignFlowMedia: POST /v1/brands/{brandId}/campaigns/{campaignId}/flow/media
func (d *Deps) CampaignFlowMedia(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	rawIDs, ok := stringSliceField(body, "media_ids")
	if !ok || len(rawIDs) == 0 {
		d.writeErr(w, errUnprocessable("Array must contain at least 1 element(s)"))
		return
	}
	if len(rawIDs) > 50 {
		d.writeErr(w, errUnprocessable("Array must contain at most 50 element(s)"))
		return
	}
	mediaIDs := dedupeStrings(rawIDs)

	ph := placeholders(1, len(mediaIDs))
	args := make([]any, len(mediaIDs))
	for i, id := range mediaIDs {
		args[i] = id
	}
	mediaRows, err := d.queryRows(r.Context(), "SELECT id, price, status FROM media WHERE id IN ("+ph+")", args...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	mediaByID := map[string]bsRow{}
	for _, m := range mediaRows {
		mediaByID[m.strOr("id", "")] = m
	}
	for _, id := range mediaIDs {
		media, ok := mediaByID[id]
		if !ok {
			d.writeErr(w, errStatus(500, "Media "+id+" not found"))
			return
		}
		if media.strOr("status", "") != "active" {
			d.writeErr(w, errStatus(500, "Media "+id+" is not active"))
			return
		}
	}

	existingRows, err := d.queryRows(r.Context(),
		"SELECT media_id FROM campaign_media_items WHERE campaign_id = $1 AND media_id IN ("+placeholders(2, len(mediaIDs))+")",
		append([]any{campaignID}, args...)...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	existingSet := map[string]bool{}
	for _, row := range existingRows {
		existingSet[row.strOr("media_id", "")] = true
	}

	costs := d.creditCostsForMedia(r.Context(), brandRow.ID, mediaIDs)
	created := []map[string]any{}
	for _, mediaID := range mediaIDs {
		if existingSet[mediaID] {
			continue
		}
		tcCost := costs[mediaID]
		id := uuid.NewString()
		if err := d.exec(r.Context(),
			"INSERT INTO campaign_media_items (id, campaign_id, media_id, tc_cost, status) VALUES ($1, $2, $3, $4, 'pending')",
			id, campaignID, mediaID, tcCost); err != nil {
			d.writeErr(w, err)
			return
		}
		created = append(created, map[string]any{
			"id": id, "campaign_id": campaignID, "media_id": mediaID, "tc_cost": tcCost, "status": "pending",
		})
	}

	// Lock media ke campaign + snapshot editorial standard (media-first flow).
	firstMediaID := mediaIDs[0]
	var mediaTypePtr *string
	if mt := campaign.strOr("media_type", ""); mt != "" {
		mediaTypePtr = &mt
	}
	snapshot := d.resolveEditorialStandardSnapshot(r.Context(), firstMediaID, mediaTypePtr)
	if err := d.exec(r.Context(),
		`UPDATE campaigns SET selected_media_id = COALESCE(selected_media_id, $1), editorial_standard_snapshot = COALESCE(editorial_standard_snapshot, $2::jsonb), media_locked_at = COALESCE(media_locked_at, NOW()), updated_at = NOW() WHERE id = $3`,
		firstMediaID, marshalJSON(snapshot), campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	if _, err := d.assignDocumentsToItems(r.Context(), brandRow.ID, campaignID, campaign); err != nil {
		d.writeErr(w, err)
		return
	}
	currentStatus := campaign.strOr("campaign_flow_status", "intake")
	if containsString(flowStages, currentStatus) && currentStatus != "media_selection" {
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET campaign_flow_status = 'media_selection', updated_at = NOW() WHERE id = $1", campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "campaign.media_selected", "campaign", campaignID)
	writeJSON(w, http.StatusCreated, map[string]any{"campaign_media_items": created})
}

// ---------- flow/media-items/{itemId} ----------

// CampaignMediaItemPatch: PATCH /v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}
func (d *Deps) CampaignMediaItemPatch(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	itemID := chiParam(r, "itemId")
	if _, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	if _, err := d.scopedMediaItem(r.Context(), brandRow.ID, campaignID, itemID); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	if raw, ok := body["thumbnail_url"]; ok {
		var value any
		if raw == nil {
			value = nil
		} else if s, isStr := raw.(string); isStr {
			value = nullIfEmpty(strings.TrimSpace(s))
		} else {
			d.writeErr(w, errUnprocessable("Expected string, received number"))
			return
		}
		if err := d.exec(r.Context(), "UPDATE campaign_media_items SET thumbnail_url = $1 WHERE id = $2", value, itemID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	updated, err := d.queryRow(r.Context(), "SELECT * FROM campaign_media_items WHERE id = $1 LIMIT 1", itemID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": rowToAnyMap(updated)})
}

// CampaignMediaItemDelete: DELETE /v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}
func (d *Deps) CampaignMediaItemDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	itemID := chiParam(r, "itemId")
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	flowStatus := campaign.strOr("campaign_flow_status", "")
	if flowStatus == "checkout" || flowStatus == "distributing" {
		httpx.Error(w, http.StatusConflict, "Tidak bisa menghapus media setelah checkout.")
		return
	}
	if _, err := d.scopedMediaItem(r.Context(), brandRow.ID, campaignID, itemID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM campaign_media_items WHERE id = $1", itemID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- flow/media-items/{itemId}/angle-options ----------

// CampaignMediaItemAngleOptions: GET /v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/angle-options
func (d *Deps) CampaignMediaItemAngleOptions(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	itemID := chiParam(r, "itemId")
	if _, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	// itemId "none" dipakai untuk "buat ulang artikel utama".
	if itemID != "none" {
		if _, err := d.scopedMediaItem(r.Context(), brandRow.ID, campaignID, itemID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	seed := randomSeed()
	writeJSON(w, http.StatusOK, map[string]any{"seed": seed, "options": angleOptionsJSON(4, seed)})
}

// ---------- flow/media-items/{itemId}/angle ----------

// CampaignMediaItemAngle: POST /v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/angle
func (d *Deps) CampaignMediaItemAngle(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	itemID := chiParam(r, "itemId")
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	item, err := d.scopedMediaItem(r.Context(), brandRow.ID, campaignID, itemID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	angleID, ok := strField(body, "angle_id")
	if !ok || angleID == "" {
		d.writeErr(w, errUnprocessable("String must contain at least 1 character(s)"))
		return
	}
	angle, found := angleByID(angleID)
	if !found {
		httpx.Error(w, http.StatusUnprocessableEntity, "Angle tidak dikenali")
		return
	}
	documentID, chosen, err := d.generateVariantForItem(r.Context(), brandRow.ID, user.ID, campaign, item, "generate", &angle, "")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "document_id": documentID, "angle_label": chosen.Label})
}

// ---------- flow/media-items/{itemId}/paraphrase ----------

// CampaignMediaItemParaphrase: POST /v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/paraphrase
func (d *Deps) CampaignMediaItemParaphrase(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	itemID := chiParam(r, "itemId")
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	item, err := d.scopedMediaItem(r.Context(), brandRow.ID, campaignID, itemID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	angleID, _ := strField(body, "angle_id")
	documentID, chosen, err := d.generateVariantForItem(r.Context(), brandRow.ID, user.ID, campaign, item, "paraphrase", nil, angleID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "document_id": documentID, "angle_label": chosen.Label})
}

// ---------- flow/analyze ----------

// CampaignFlowAnalyze: POST /v1/brands/{brandId}/campaigns/{campaignId}/flow/analyze
func (d *Deps) CampaignFlowAnalyze(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, chiParam(r, "campaignId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	contentArg, _ := strField(body, "content")

	intakeID := campaign.strOr("content_intake_id", "")
	if intakeID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Campaign belum memiliki content intake.")
		return
	}
	intake, err := d.queryRow(r.Context(),
		"SELECT topic, answers, article_text FROM content_intakes WHERE id = $1 LIMIT 1", intakeID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if intake == nil {
		httpx.Error(w, http.StatusNotFound, "Content intake tidak ditemukan.")
		return
	}
	mediaID := campaign.strOr("selected_media_id", "")
	mediaType := campaign.strOr("media_type", "")

	content := contentArg
	if content == "" {
		job, err := d.queryRow(r.Context(),
			"SELECT id FROM content_jobs WHERE id = (SELECT job_id FROM content_intakes WHERE id = $1) LIMIT 1", intakeID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if job != nil {
			doc, err := d.queryRow(r.Context(),
				"SELECT id, current_version FROM content_documents WHERE job_id = $1 ORDER BY created_at DESC LIMIT 1", job.strOr("id", ""))
			if err != nil {
				d.writeErr(w, err)
				return
			}
			if doc != nil {
				ver, err := d.queryRow(r.Context(),
					"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
					doc.strOr("id", ""), doc.intOr("current_version", 1))
				if err != nil {
					d.writeErr(w, err)
					return
				}
				if ver != nil {
					content = ver.strOr("content", "")
				}
			}
		}
	}
	if content == "" {
		content = intake.strOr("article_text", "")
		if content == "" {
			content = intake.strOr("topic", "")
		}
	}

	analysis, err := d.analyzeContentForMedia(r.Context(), brandRow.ID, user.ID, intakeID, campaign.strOr("id", ""), mediaID, mediaType, intake, content)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"analysis": analysis})
}

// ---------- flow/article-regen ----------

// CampaignFlowArticleRegen: POST /v1/brands/{brandId}/campaigns/{campaignId}/flow/article-regen
func (d *Deps) CampaignFlowArticleRegen(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, chiParam(r, "campaignId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	angleID, _ := strField(body, "angle_id")
	documentIDArg, _ := strField(body, "document_id")

	var doc bsRow
	if documentIDArg != "" {
		found, err := d.queryRow(r.Context(),
			"SELECT id, title, meta_title, meta_description, current_version FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1",
			documentIDArg, brandRow.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if found == nil {
			d.writeErr(w, errStatus(500, "Dokumen artikel tidak ditemukan."))
			return
		}
		doc = found
	} else {
		intakeID := campaign.strOr("content_intake_id", "")
		if intakeID == "" {
			d.writeErr(w, errStatus(500, "Campaign belum punya content intake."))
			return
		}
		intake, err := d.queryRow(r.Context(),
			"SELECT job_id FROM content_intakes WHERE id = $1 AND brand_id = $2 LIMIT 1", intakeID, brandRow.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if intake == nil || !intake.has("job_id") {
			d.writeErr(w, errStatus(500, "Belum ada artikel — generate dulu."))
			return
		}
		found, err := d.queryRow(r.Context(),
			"SELECT id, title, meta_title, meta_description, current_version FROM content_documents WHERE job_id = $1 AND brand_id = $2 LIMIT 1",
			intake.strOr("job_id", ""), brandRow.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if found == nil {
			d.writeErr(w, errStatus(500, "Dokumen artikel tidak ditemukan."))
			return
		}
		doc = found
	}

	wallet := d.getWallet(r.Context(), brandRow.ID)
	available := 0
	if wallet != nil {
		available = wallet.AvailableTC
	}
	if available < regenCostTC {
		httpx.Error(w, http.StatusConflict, "Saldo TC tidak mencukupi untuk buat ulang (perlu "+itoa(regenCostTC)+" TC).")
		return
	}
	if err := d.adjustCredit(r.Context(), brandRow.ID, -regenCostTC, "Buat ulang artikel (angle berbeda)", user.ID, ""); err != nil {
		d.writeErr(w, err)
		return
	}

	docID := doc.strOr("id", "")
	ver, err := d.queryRow(r.Context(),
		"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 ORDER BY version DESC LIMIT 1",
		docID, doc.intOr("current_version", 1))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	existingArticle := ""
	if ver != nil {
		existingArticle = ver.strOr("content", "")
	}
	sourceMaterial := d.campaignSourceMaterial(r.Context(), brandRow.ID, campaign)

	angle, found := angleByID(angleID)
	if !found {
		d.writeErr(w, errStatus(500, "Angle tidak dikenali."))
		return
	}
	parsed, err := d.generateVariantArticle(r.Context(), "paraphrase", sourceMaterial, &existingArticle, angle, campaign.strOr("name", "Tulisio"), nil)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	nextVersion := doc.intOr("current_version", 1) + 1
	if err := d.exec(r.Context(),
		"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, $3, $4, 'html', 'ai_regenerated', $5, $6)",
		uuid.NewString(), docID, nextVersion, markdownToHTML(parsed.Body), truncateString("Buat ulang — sudut: "+angle.Label, 255), user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE content_documents SET current_version = $1, title = $2, meta_title = $3, meta_description = $4, status = 'approved' WHERE id = $5",
		nextVersion, truncateString(parsed.Title, 300), truncateString(parsed.MetaTitle, 255), truncateString(parsed.MetaDescription, 500), docID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "document_id": docID, "title": parsed.Title, "angle_label": angle.Label, "tc_used": regenCostTC,
	})
}

// ---------- flow/checkout ----------

// CampaignFlowCheckout: POST /v1/brands/{brandId}/campaigns/{campaignId}/flow/checkout
func (d *Deps) CampaignFlowCheckout(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	if _, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	publishStatus := "draft"
	if s, ok := strField(body, "publish_status"); ok && s == "publish" {
		publishStatus = "publish"
	}

	items, err := d.queryRows(r.Context(),
		`SELECT cmi.id, cmi.media_id, cmi.source_document_id, cmi.variant_document_id, cmi.tc_cost, cmi.status, cmi.thumbnail_url
		 FROM campaign_media_items cmi
		 WHERE cmi.campaign_id = $1`, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if len(items) == 0 {
		httpx.Error(w, http.StatusBadRequest, "Belum ada media dipilih untuk campaign ini.")
		return
	}
	notReady := 0
	for _, item := range items {
		if item.strOr("status", "") != itemStatusReady {
			notReady++
		}
	}
	if notReady > 0 {
		httpx.Error(w, http.StatusConflict, itoa(notReady)+" media belum siap distribusi — selesaikan parafrase/angle terlebih dahulu.")
		return
	}
	totalTcCost := 0
	for _, item := range items {
		totalTcCost += item.intOr("tc_cost", 0)
	}
	wallet := d.getWallet(r.Context(), brandRow.ID)
	availableTc := 0
	if wallet != nil {
		availableTc = wallet.AvailableTC
	}
	if totalTcCost > 0 && availableTc < totalTcCost {
		httpx.Error(w, http.StatusConflict, "Saldo TC tidak mencukupi: perlu "+itoa(totalTcCost)+" TC, tersedia "+itoa(availableTc)+" TC. Lakukan top-up terlebih dahulu.")
		return
	}

	if err := d.exec(r.Context(), "UPDATE campaigns SET campaign_flow_status = 'checkout', updated_at = NOW() WHERE id = $1", campaignID); err != nil {
		d.writeErr(w, err)
		return
	}

	createdIDs := []string{}
	createdPublications := []map[string]any{}
	rollback := func() {
		for _, id := range createdIDs {
			d.releaseCredit(r.Context(), brandRow.ID, id, user.ID)
			_ = d.exec(r.Context(), "DELETE FROM publications WHERE id = $1", id)
		}
		for _, item := range items {
			_ = d.exec(r.Context(), "UPDATE campaign_media_items SET status = 'article_ready', updated_at = NOW() WHERE id = $1", item.strOr("id", ""))
		}
		_ = d.exec(r.Context(), "UPDATE campaigns SET campaign_flow_status = 'final_review', updated_at = NOW() WHERE id = $1", campaignID)
	}

	for _, item := range items {
		documentID := item.strOr("variant_document_id", "")
		if documentID == "" {
			documentID = item.strOr("source_document_id", "")
		}
		if documentID == "" {
			rollback()
			d.writeErr(w, errStatus(500, "Item "+item.strOr("id", "")+" belum punya artikel."))
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE content_documents SET status = 'approved', approved_by = $1, approved_at = NOW() WHERE id = $2 AND status <> 'approved'",
			user.ID, documentID); err != nil {
			rollback()
			d.writeErr(w, err)
			return
		}
		result, err := d.createPublicationRecord(r.Context(), createPublicationInput{
			BrandID: brandRow.ID, UserID: user.ID, DocumentID: documentID, MediaID: item.strOr("media_id", ""),
			PublishStatus: publishStatus, CampaignID: &campaignID, SkipStandardCheck: false,
		})
		if err != nil {
			rollback()
			d.writeErr(w, err)
			return
		}
		pubID := result.Row.strOr("id", "")
		if thumb := item.strOr("thumbnail_url", ""); thumb != "" {
			if err := d.exec(r.Context(), "UPDATE publications SET featured_image_url = $1, updated_at = NOW() WHERE id = $2", thumb, pubID); err != nil {
				rollback()
				d.writeErr(w, err)
				return
			}
		}
		createdIDs = append(createdIDs, pubID)
		createdPublications = append(createdPublications, map[string]any{
			"id": pubID, "order_code": result.Row.raw("order_code"),
			"media_id": item.strOr("media_id", ""), "document_id": documentID,
			"status": result.Row.raw("status"),
		})
		if tcCost := item.intOr("tc_cost", 0); tcCost > 0 {
			if _, err := d.reserveCredit(r.Context(), brandRow.ID, pubID, tcCost, user.ID); err != nil {
				rollback()
				d.writeErr(w, err)
				return
			}
		}
		if err := d.exec(r.Context(), "UPDATE campaign_media_items SET status = 'processing', updated_at = NOW() WHERE id = $1", item.strOr("id", "")); err != nil {
			rollback()
			d.writeErr(w, err)
			return
		}
	}
	if err := d.exec(r.Context(), "UPDATE campaigns SET campaign_flow_status = 'distributing', updated_at = NOW() WHERE id = $1", campaignID); err != nil {
		rollback()
		d.writeErr(w, err)
		return
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "campaign.checkout", "campaign", campaignID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "total_tc": totalTcCost, "publication_count": len(createdPublications), "publications": createdPublications,
	})
}

// ---------- campaigns/{campaignId}/publications ----------

// CampaignPublicationsList: GET /v1/brands/{brandId}/campaigns/{campaignId}/publications
func (d *Deps) CampaignPublicationsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	if _, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID); err != nil {
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
	clause := "WHERE p.campaign_id = $1"
	baseParams := []any{campaignID}
	if status != "" {
		clause += " AND p.status = $2"
		baseParams = append(baseParams, status)
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM publications p "+clause, baseParams...)
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
		SELECT p.id, p.brand_id, p.document_id, p.media_id, p.status, p.client_price, p.publish_status, p.connection_id, p.scheduled_at, p.published_at, p.published_url, p.notes, p.cancel_reason, p.campaign_id, p.order_code, p.created_by, p.created_at, p.updated_at,
		       d.title AS document_title, d.slug AS document_slug,
		       m.name AS media_name, m.category AS media_category, m.website_url AS media_website_url
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
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

// CampaignPublicationsCreate: POST /v1/brands/{brandId}/campaigns/{campaignId}/publications
func (d *Deps) CampaignPublicationsCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	body := decodeJSON(r)

	documentID, _, err := requiredTrimmedString(body, "document_id", 1, 1<<30, nil)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	mediaIDs, ok := stringSliceField(body, "media_ids")
	if !ok || len(mediaIDs) == 0 {
		d.writeErr(w, errUnprocessable("Array must contain at least 1 element(s)"))
		return
	}
	if len(mediaIDs) > 50 {
		d.writeErr(w, errUnprocessable("Array must contain at most 50 element(s)"))
		return
	}
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
	skipStandardCheck := false
	if b, ok := boolField(body, "skip_standard_check"); ok {
		skipStandardCheck = b
	}

	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	uniqueMediaIDs := dedupeStrings(mediaIDs)
	document, err := d.scopedDocument(r.Context(), brandRow.ID, documentID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if document.strOr("status", "") != "approved" {
		d.writeErr(w, errConflict("Only approved content can be published"))
		return
	}

	costs := d.creditCostsForMedia(r.Context(), brandRow.ID, uniqueMediaIDs)
	totalCost := 0
	for _, mediaID := range uniqueMediaIDs {
		totalCost += costs[mediaID]
	}
	if totalCost > 0 {
		balance := d.brandCreditBalance(r.Context(), brandRow.ID)
		if balance.PackageCount > 0 && balance.Remaining < totalCost {
			d.writeErr(w, errConflict("Insufficient credits: need "+itoa(totalCost)+", available "+itoa(balance.Remaining)))
			return
		}
	}
	if err := assertCampaignQuota(campaign, len(uniqueMediaIDs)); err != nil {
		d.writeErr(w, err)
		return
	}

	if !skipStandardCheck {
		results, err := d.finalStandardCheckMany(r.Context(), brandRow.ID, documentID, uniqueMediaIDs)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		failed := []string{}
		for _, result := range results {
			if result.Status != "fail" {
				continue
			}
			name := "media"
			if s, ok := result.MediaName.(string); ok && s != "" {
				name = s
			}
			failed = append(failed, name+" ("+strings.Join(capStrings(result.Failed, 2), "; ")+")")
		}
		if len(failed) > 0 {
			d.writeErr(w, errConflict("Artikel belum memenuhi standar editorial: "+strings.Join(capStrings(failed, 3), "; ")+
				". Perbaiki artikel atau jalankan standard check untuk detail."))
			return
		}
	}

	createdRows := []bsRow{}
	createdIDs := []string{}
	rollback := func() {
		for _, id := range createdIDs {
			d.refundPackageCredits(r.Context(), brandRow.ID, id, user.ID)
			d.releaseCredit(r.Context(), brandRow.ID, id, user.ID)
			_ = d.exec(r.Context(), "DELETE FROM publications WHERE id = $1", id)
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

	for _, mediaID := range uniqueMediaIDs {
		result, err := d.createPublicationRecord(r.Context(), createPublicationInput{
			BrandID: brandRow.ID, UserID: user.ID, DocumentID: documentID, MediaID: mediaID,
			PublishStatus: publishStatus, ScheduledAt: scheduledPtr, Notes: notesPtr,
			CampaignID: &campaignID, SkipStandardCheck: skipStandardCheck,
		})
		if err != nil {
			rollback()
			d.writeErr(w, err)
			return
		}
		createdRows = append(createdRows, result.Row)
		pubID := result.Row.strOr("id", "")
		createdIDs = append(createdIDs, pubID)
		if costs[mediaID] > 0 {
			if _, err := d.reservePackageCredits(r.Context(), brandRow.ID, pubID, costs[mediaID], user.ID); err != nil {
				rollback()
				d.writeErr(w, err)
				return
			}
		}
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "campaign.publications.created", "campaign", campaignID)
	publications := make([]map[string]any, 0, len(createdRows))
	for _, row := range createdRows {
		publications = append(publications, toPublicationJSON(row, 0))
	}
	writeJSON(w, http.StatusCreated, map[string]any{"publications": publications})
}

// analyzeContentForMedia: port lib/ai/analysis.ts analyzeContentForMedia.
// Best-effort: kegagalan AI tidak menggagalkan flow (diagnosis berisi pesan).
func (d *Deps) analyzeContentForMedia(ctx context.Context, brandID, userID, intakeID, campaignID, mediaID, mediaType string, intake bsRow, content string) (map[string]any, error) {
	topic := intake.strOr("topic", "")
	if strings.TrimSpace(content) == "" && strings.TrimSpace(topic) == "" {
		return emptyAnalysisResult("global", nil, nil), nil
	}
	if mediaType == "" {
		if mediaID != "" {
			if m, err := d.queryRow(ctx, "SELECT media_type FROM media WHERE id = $1 LIMIT 1", mediaID); err == nil && m != nil {
				mediaType = m.strOr("media_type", "")
			}
		}
	}
	standardSource, promptBlock := d.resolveStandardBlock(ctx, mediaID, mediaType, brandID)
	hash := contentHash(topic, content, mediaID, standardSource)

	cached, err := d.queryRow(ctx,
		"SELECT * FROM content_ai_analyses WHERE content_hash = $1 AND content_intake_id = $2 ORDER BY created_at DESC LIMIT 1", hash, intakeID)
	if err == nil && cached != nil {
		return analysisRowToJSON(cached), nil
	}

	prompt := buildAnalysisPrompt(topic, content, mediaType, mediaID, intake, promptBlock, standardSource)
	result := emptyAnalysisResult(standardSource, nil, nil)
	runtime, rtErr := ai.LoadAIRuntime(ctx, d.Pool)
	if rtErr == nil && ai.IsAIConfigured(runtime) {
		gen, genErr := ai.GenerateChat(ctx, d.Pool, ai.GenerateChatOptions{
			Prompt: prompt, Tier: "default", TaskType: "content_analysis",
			BrandID: &brandID, UserID: &userID, Runtime: runtime,
		})
		if genErr == nil {
			result = parseAnalysisResult(gen.Content, standardSource)
		} else {
			result["diagnosis"] = "Analisis gagal: " + truncateString(genErr.Error(), 200)
		}
	} else {
		result["diagnosis"] = "Analisis gagal: AI belum dikonfigurasi"
	}
	result["standard_source"] = standardSource
	result["standard_version"] = standardSource
	if standardSource == "mainstream_fallback" {
		result["fallback_type"] = "mainstream"
	} else if standardSource == "blogger_fallback" {
		result["fallback_type"] = "blogger"
	}

	if err := d.exec(ctx, `
		INSERT INTO content_ai_analyses (id, content_intake_id, campaign_id, media_id, media_type, editorial_score, diagnosis, strengths, violations, content_type_detected, editorial_fit_score, readability, clarity, search_intent, structure_score, potential_value, headline_suggestions, revised_draft, standard_source, standard_version, fallback_type, content_hash) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
		uuid.NewString(), intakeID, anyOrNil(campaignID), anyOrNil(mediaID), anyOrNil(mediaType),
		result["editorial_score"], result["diagnosis"], marshalJSON(result["strengths"]), marshalJSON(result["violations"]),
		result["content_type_detected"], result["editorial_fit_score"], result["readability"], result["clarity"],
		result["search_intent"], result["structure_score"], result["potential_value"],
		marshalJSON(result["headline_suggestions"]), result["revised_draft"],
		standardSource, standardSource, result["fallback_type"], hash); err != nil {
		// Persist best-effort (parity try/catch kosong).
		_ = err
	}
	return result, nil
}

func jsonOr(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}

func errStatusCode422Message(msg string) error {
	return errUnprocessable(msg)
}
