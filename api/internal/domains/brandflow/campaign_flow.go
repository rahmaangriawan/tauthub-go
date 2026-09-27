package brandflow

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
)

// campaign_flow.go: port lib/campaign/flow.ts (FLOW stages, assign documents,
// source material, media item scoping) + generateVariantForItem.

var flowStages = []string{"intake", "content", "media_selection", "final_review", "checkout", "distributing"}

var flowTransitions = map[string][]string{
	"intake":          {"content"},
	"content":         {"intake", "media_selection"},
	"media_selection": {"content", "final_review"},
	"final_review":    {"media_selection", "checkout"},
	"checkout":        {"distributing"},
}

const (
	itemStatusReady   = "article_ready"
	itemStatusRewrite = "needs_rewrite"
)

// campaignSourceDocument: dokumen sumber campaign = intake → job → document.
func (d *Deps) campaignSourceDocument(ctx context.Context, brandID string, campaign bsRow) (bsRow, error) {
	intakeID := campaign.strOr("content_intake_id", "")
	if intakeID == "" {
		return nil, nil
	}
	intake, err := d.queryRow(ctx,
		"SELECT id, job_id FROM content_intakes WHERE id = $1 AND brand_id = $2 LIMIT 1", intakeID, brandID)
	if err != nil || intake == nil || !intake.has("job_id") {
		return nil, err
	}
	return d.queryRow(ctx,
		"SELECT id, title, meta_title, meta_description, current_version FROM content_documents WHERE job_id = $1 AND brand_id = $2 LIMIT 1",
		intake.strOr("job_id", ""), brandID)
}

// assignDocumentsToItems: port assignDocumentsToItems — tautkan dokumen sumber
// ke tiap item; satu item memegang dokumen asli, sisanya needs_rewrite.
func (d *Deps) assignDocumentsToItems(ctx context.Context, brandID, campaignID string, campaign bsRow) (int, error) {
	sourceDoc, err := d.campaignSourceDocument(ctx, brandID, campaign)
	if err != nil || sourceDoc == nil {
		return 0, err
	}
	sourceDocID := sourceDoc.strOr("id", "")
	items, err := d.queryRows(ctx,
		"SELECT id, source_document_id, status, variant_document_id FROM campaign_media_items WHERE campaign_id = $1 ORDER BY created_at ASC", campaignID)
	if err != nil {
		return 0, err
	}
	hasSourceHolder := false
	for _, it := range items {
		if it.strOr("source_document_id", "") == sourceDocID && it.strOr("status", "") == itemStatusReady && !it.has("variant_document_id") {
			hasSourceHolder = true
		}
	}

	assigned := 0
	for _, item := range items {
		src := item.strOr("source_document_id", "")
		status := item.strOr("status", "")
		if src == sourceDocID && status == itemStatusReady && !item.has("variant_document_id") {
			hasSourceHolder = true
			continue
		}
		if src == sourceDocID && status == itemStatusRewrite && !hasSourceHolder {
			if err := d.exec(ctx, "UPDATE campaign_media_items SET status = $1, updated_at = NOW() WHERE id = $2", itemStatusReady, item.strOr("id", "")); err != nil {
				return assigned, err
			}
			hasSourceHolder = true
			assigned++
			continue
		}
		if src == "" {
			newStatus := itemStatusRewrite
			if !hasSourceHolder {
				newStatus = itemStatusReady
				hasSourceHolder = true
			}
			if err := d.exec(ctx,
				"UPDATE campaign_media_items SET source_document_id = $1, status = $2, updated_at = NOW() WHERE id = $3",
				sourceDocID, newStatus, item.strOr("id", "")); err != nil {
				return assigned, err
			}
			assigned++
		}
	}
	return assigned, nil
}

// documentCurrentContent: versi konten terbaru dokumen milik brand.
func (d *Deps) documentCurrentContent(ctx context.Context, brandID, documentID string) *string {
	doc, err := d.queryRow(ctx,
		"SELECT current_version FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1", documentID, brandID)
	if err != nil || doc == nil {
		return nil
	}
	ver, err := d.queryRow(ctx,
		"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 ORDER BY version DESC LIMIT 1",
		documentID, doc.intOr("current_version", 1))
	if err != nil || ver == nil {
		return nil
	}
	content := ver.strOr("content", "")
	return &content
}

// campaignSourceMaterial: bahan mentah dari intake (mode generate).
func (d *Deps) campaignSourceMaterial(ctx context.Context, brandID string, campaign bsRow) *string {
	intakeID := campaign.strOr("content_intake_id", "")
	if intakeID == "" {
		return nil
	}
	intake, err := d.queryRow(ctx,
		"SELECT topic, answers, article_text FROM content_intakes WHERE id = $1 AND brand_id = $2 LIMIT 1", intakeID, brandID)
	if err != nil || intake == nil {
		return nil
	}
	parts := []string{}
	if topic := intake.strOr("topic", ""); topic != "" {
		parts = append(parts, "Topik: "+topic)
	}
	if text := intake.strOr("article_text", ""); text != "" {
		parts = append(parts, "Artikel/bahan klien:\n"+text)
	}
	if answers, ok := intake.jsonField("answers").(map[string]any); ok {
		answerParts := []string{}
		for _, key := range sortedKeys(answers) {
			if v, ok := answers[key].(string); ok && strings.TrimSpace(v) != "" {
				answerParts = append(answerParts, key+": "+strings.TrimSpace(v))
			}
		}
		if len(answerParts) > 0 {
			parts = append(parts, "Informasi dari klien:\n"+strings.Join(answerParts, "\n"))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	joined := strings.Join(parts, "\n\n")
	return &joined
}

// scopedMediaItem: satu item media milik campaign+brand; NotFound bila tidak ada.
func (d *Deps) scopedMediaItem(ctx context.Context, brandID, campaignID, itemID string) (bsRow, error) {
	item, err := d.queryRow(ctx,
		`SELECT cmi.*, m.name AS media_name
		 FROM campaign_media_items cmi
		 JOIN campaigns c ON c.id = cmi.campaign_id AND c.brand_id = $1
		 JOIN media m ON m.id = cmi.media_id
		 WHERE cmi.id = $2 AND cmi.campaign_id = $3
		 LIMIT 1`, brandID, itemID, campaignID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errNotFound("Media item tidak ditemukan")
	}
	return item, nil
}

// generateVariantForItem: port generateVariantForItem — parafrase/angle untuk
// satu item media; artikel asli tidak diubah.
func (d *Deps) generateVariantForItem(ctx context.Context, brandID, userID string, campaign, item bsRow, mode string, angle *variantAngle, angleID string) (string, variantAngle, error) {
	if !item.has("source_document_id") {
		return "", variantAngle{}, errUnprocessable("Item belum punya artikel sumber — pilih media dulu.")
	}
	mediaName := item.strOr("media_name", "")
	if mediaName == "" {
		return "", variantAngle{}, errUnprocessable("Media tidak ditemukan.")
	}

	var chosen variantAngle
	if angle != nil {
		chosen = *angle
	} else if angleID != "" {
		found, ok := angleByID(angleID)
		if !ok {
			return "", variantAngle{}, errUnprocessable("Angle tidak dikenali")
		}
		chosen = found
	} else {
		picked := pickAngles(1, rand.Intn(1<<31))
		chosen = picked[0]
	}

	sourceDocID := item.strOr("source_document_id", "")
	existingArticle := d.documentCurrentContent(ctx, brandID, sourceDocID)
	sourceMaterial := d.campaignSourceMaterial(ctx, brandID, campaign)

	topic := campaign.strOr("topic", "")
	rules := d.editorialRulesFor(ctx, brandID, "press_release", ptrOrNil(topic), sourceMaterial)

	parsed, err := d.generateVariantArticle(ctx, mode, sourceMaterial, existingArticle, chosen, mediaName, rules)
	if err != nil {
		return "", variantAngle{}, err
	}
	documentID, err := d.insertVariantDocument(ctx, brandID, userID, parsed, chosen, mode)
	if err != nil {
		return "", variantAngle{}, err
	}
	variantType := "angle"
	if mode == "paraphrase" {
		variantType = "paraphrase"
	}
	if err := d.exec(ctx,
		"UPDATE campaign_media_items SET variant_document_id = $1, angle_label = $2, variant_type = $3, status = $4, updated_at = NOW() WHERE id = $5",
		documentID, truncateString(chosen.Label, 200), variantType, itemStatusReady, item.strOr("id", "")); err != nil {
		return "", variantAngle{}, err
	}
	return documentID, chosen, nil
}

// angleOptionsJSON: parity angleOptions(count, seed) → daftar angle.
func angleOptionsJSON(count, seed int) []map[string]any {
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}
	picked := pickAngles(count, seed)
	out := make([]map[string]any, 0, len(picked))
	for _, a := range picked {
		out = append(out, map[string]any{"id": a.ID, "label": a.Label, "instruction": a.Instruction})
	}
	return out
}

// documentContent: parity documentContent — doc + versi terpilih.
func (d *Deps) documentContent(ctx context.Context, brandID, documentID string) (bsRow, error) {
	doc, err := d.queryRow(ctx,
		"SELECT id, title, meta_title, meta_description, current_version FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1",
		documentID, brandID)
	if err != nil || doc == nil {
		return nil, err
	}
	ver, err := d.queryRow(ctx,
		"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 ORDER BY version DESC LIMIT 1",
		documentID, doc.intOr("current_version", 1))
	if err != nil {
		return nil, err
	}
	if ver != nil {
		doc["content"] = ver.strOr("content", "")
	} else {
		doc["content"] = ""
	}
	return doc, nil
}

// resolveEditorialStandardSnapshot: port resolveEditorialStandard +
// snapshotForCampaign (lib/editorial/standard-resolver.ts).
func (d *Deps) resolveEditorialStandardSnapshot(ctx context.Context, mediaID string, mediaType *string) map[string]any {
	standardSource := "global"
	var standardID any
	var version any
	var versionLabel any
	var fallbackType any
	mediaName := ""

	if mediaID != "" {
		media, err := d.queryRow(ctx, "SELECT id, name FROM media WHERE id = $1 LIMIT 1", mediaID)
		if err == nil && media != nil {
			mediaName = media.strOr("name", "")
			if std := d.mediaStandardRow(ctx, mediaID); std != nil {
				standardSource = "media_specific"
				standardID = std.MediaID
			}
		}
	}
	if standardSource != "media_specific" {
		if mediaType != nil && *mediaType == "mainstream" {
			standardSource = "mainstream_fallback"
			fallbackType = "mainstream"
		} else if mediaType != nil && *mediaType == "blogger" {
			standardSource = "blogger_fallback"
			fallbackType = "blogger"
		}
	}
	var mt any
	if mediaType != nil {
		mt = *mediaType
	}
	return map[string]any{
		"media_id":               nullIfEmpty(mediaID),
		"media_type":             mt,
		"media_name":             nullIfEmpty(mediaName),
		"standard_source":        standardSource,
		"standard_id":            standardID,
		"standard_version":       version,
		"standard_version_label": versionLabel,
		"fallback_type":          fallbackType,
		"resolved_at":            isoNow(),
	}
}

// splitTcAcrossPublications: port lib/credits/split.ts.
func splitTcAcrossPublications(totalTC, count int) []int {
	if count <= 0 {
		return nil
	}
	safeTotal := totalTC
	if safeTotal < 0 {
		safeTotal = 0
	}
	base := safeTotal / count
	remainder := safeTotal - base*count
	out := make([]int, count)
	for i := range out {
		if i == 0 {
			out[i] = base + remainder
		} else {
			out[i] = base
		}
	}
	return out
}

// jsonUnmarshalMap: helper decode payload jsonb.
func jsonUnmarshalMap(raw any) map[string]any {
	switch t := raw.(type) {
	case map[string]any:
		return t
	case string:
		var m map[string]any
		if json.Unmarshal([]byte(t), &m) == nil {
			return m
		}
	}
	return map[string]any{}
}
