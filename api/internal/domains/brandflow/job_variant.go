package brandflow

import (
	"context"
	"strings"
	"time"
)

// job_variant_linkcheck.go: processor job kind `publication_variant_generation`
// dan `publication_link_check` (port lib/publication/variants.ts
// processVariantJob + lib/publication/link-check.ts checkPublicationLink).
//
// Registrasi dispatcher dilakukan di RegisterJobs (jobs_register.go).

// PublicationVariantGenerationJob: buat artikel varian untuk SATU baris
// publication_variants, lalu publikasi + reservasi TC. Idempoten: dokumen dan
// publikasi yang sudah tertaut tidak dibuat ulang (retry aman).
func (d *Deps) PublicationVariantGenerationJob(ctx context.Context, payload map[string]any) error {
	variantID := asString(payload["variantId"])
	batchID := asString(payload["batchId"])
	brandID := asString(payload["brandId"])
	userID := asString(payload["userId"])
	mode := asString(payload["mode"])
	mediaID := asString(payload["mediaId"])
	mediaName := asString(payload["mediaName"])
	if mediaName == "" {
		mediaName = "Media"
	}
	angle := variantAngle{
		ID:          asString(payload["angleId"]),
		Label:       asString(payload["angleLabel"]),
		Instruction: asString(payload["angleInstruction"]),
	}
	if variantID == "" || batchID == "" || brandID == "" || mediaID == "" {
		return errUnprocessable("Payload varian tidak lengkap.")
	}

	if err := d.exec(ctx, "UPDATE publication_variants SET status = 'processing', updated_at = NOW() WHERE id = $1", variantID); err != nil {
		return err
	}
	if err := d.exec(ctx, "UPDATE publication_variant_batches SET status = 'processing' WHERE id = $1 AND status = 'queued'", batchID); err != nil {
		return err
	}

	runErr := d.runVariantJob(ctx, variantJobInput{
		VariantID: variantID, BatchID: batchID, BrandID: brandID, UserID: userID,
		Mode: mode, MediaID: mediaID, MediaName: mediaName, Angle: angle, Payload: payload,
	})
	if runErr != nil {
		message := truncateString(runErr.Error(), 1000)
		_ = d.exec(ctx, "UPDATE publication_variants SET status = 'failed', error_message = $1, updated_at = NOW() WHERE id = $2", message, variantID)
		_ = d.updateVariantBatchProgress(ctx, batchID)
		// Konflik bisnis terminal (kredit kurang, media nonaktif, ...) tidak
		// membakar retry — parity processVariantJob.
		var se *bfStatusError
		if isBusinessTerminal(runErr, &se) {
			return nil
		}
		return runErr
	}

	if err := d.exec(ctx, "UPDATE publication_variants SET status = 'completed', updated_at = NOW() WHERE id = $1", variantID); err != nil {
		return err
	}
	return d.updateVariantBatchProgress(ctx, batchID)
}

type variantJobInput struct {
	VariantID string
	BatchID   string
	BrandID   string
	UserID    string
	Mode      string
	MediaID   string
	MediaName string
	Angle     variantAngle
	Payload   map[string]any
}

// isBusinessTerminal: error bisnis (409/404/402) dianggap terminal.
func isBusinessTerminal(err error, se **bfStatusError) bool {
	if e, ok := err.(*bfStatusError); ok {
		*se = e
		return e.Status == 409 || e.Status == 404 || e.Status == 402
	}
	var ice *insufficientCreditsError
	if asErr(err, &ice) {
		return true
	}
	return false
}

func (d *Deps) runVariantJob(ctx context.Context, in variantJobInput) error {
	variant, err := d.queryRow(ctx,
		"SELECT document_id, publication_id FROM publication_variants WHERE id = $1 LIMIT 1", in.VariantID)
	if err != nil {
		return err
	}
	documentID := ""
	publicationID := ""
	if variant != nil {
		documentID = variant.strOr("document_id", "")
		publicationID = variant.strOr("publication_id", "")
	}

	if documentID == "" {
		var topic *string
		if s := asString(in.Payload["topic"]); s != "" {
			topic = &s
		}
		var sourceMaterialAny *string
		if s := asString(in.Payload["sourceMaterial"]); s != "" {
			sourceMaterialAny = &s
		}
		editorialRules := d.editorialRulesFor(ctx, in.BrandID, "press_release", topic, sourceMaterialAny)

		var srcMaterial *string
		if in.Mode == "generate" && sourceMaterialAny != nil {
			srcMaterial = sourceMaterialAny
		}
		var existingArticle *string
		if in.Mode == "paraphrase" {
			if s := asString(in.Payload["sourceArticleText"]); s != "" {
				existingArticle = &s
			}
		}
		parsed, gErr := d.generateVariantArticle(ctx, in.Mode, srcMaterial, existingArticle, in.Angle, in.MediaName, editorialRules)
		if gErr != nil {
			return gErr
		}
		docID, iErr := d.insertVariantDocument(ctx, in.BrandID, in.UserID, parsed, in.Angle, in.Mode)
		if iErr != nil {
			return iErr
		}
		documentID = docID
		if err := d.exec(ctx, "UPDATE publication_variants SET document_id = $1 WHERE id = $2", documentID, in.VariantID); err != nil {
			return err
		}
	}

	if publicationID != "" {
		return nil
	}

	publishStatus := "draft"
	if asString(in.Payload["publishStatus"]) == "publish" {
		publishStatus = "publish"
	}
	input := createPublicationInput{
		BrandID:           in.BrandID,
		UserID:            in.UserID,
		DocumentID:        documentID,
		MediaID:           in.MediaID,
		PublishStatus:     publishStatus,
		SkipStandardCheck: asBool(in.Payload["skipStandardCheck"]),
	}
	if s := asString(in.Payload["connectionId"]); s != "" {
		input.ConnectionID = &s
	}
	if s := asString(in.Payload["scheduledAt"]); s != "" {
		if t, ok := parseISO(s); ok {
			input.ScheduledAt = &t
		}
	}
	if s := asString(in.Payload["campaignId"]); s != "" {
		input.CampaignID = &s
	}

	created, cErr := d.createPublicationRecord(ctx, input)
	if cErr != nil {
		return cErr
	}
	publicationID = created.Row.strOr("id", "")

	// Wallet TC (brand) — reservasi idempoten per publikasi.
	cost := d.creditCostForMedia(ctx, in.BrandID, in.MediaID)
	if cost > 0 {
		if _, rErr := d.reserveCredit(ctx, in.BrandID, publicationID, cost, in.UserID); rErr != nil {
			// Jangan tinggalkan publikasi yatim saat reservasi gagal.
			_ = d.exec(ctx, "DELETE FROM publications WHERE id = $1", publicationID)
			return rErr
		}
	}
	// Paket kredit lama (packages/credit_transactions) — hanya saat kampanye.
	if asString(in.Payload["campaignId"]) != "" && cost > 0 {
		if _, pErr := d.reservePackageCredits(ctx, in.BrandID, publicationID, cost, in.UserID); pErr != nil {
			_ = d.exec(ctx, "DELETE FROM publications WHERE id = $1", publicationID)
			return pErr
		}
	}

	d.audit(ctx, in.BrandID, in.UserID, "publication.created", "publication", publicationID)
	return d.exec(ctx, "UPDATE publication_variants SET publication_id = $1 WHERE id = $2", publicationID, in.VariantID)
}

// updateVariantBatchProgress: hitung ulang agregat batch dan finalkan status
// secara atomik (parity updateBatchProgress). Notifikasi admin dikirim dari
// pemanggil setelah transisi terminal.
func (d *Deps) updateVariantBatchProgress(ctx context.Context, batchID string) error {
	agg, err := d.queryRow(ctx, `
		SELECT COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE status = 'completed') AS done,
		       COUNT(*) FILTER (WHERE status = 'failed') AS failed
		FROM publication_variants WHERE batch_id = $1`, batchID)
	if err != nil || agg == nil {
		return err
	}
	total := agg.intOr("total", 0)
	done := agg.intOr("done", 0)
	failed := agg.intOr("failed", 0)
	if total == 0 || done+failed < total {
		return nil
	}
	status := "completed"
	if failed > 0 {
		if done == 0 {
			status = "failed"
		} else {
			status = "partial"
		}
	}
	res, err := d.execRows(ctx,
		"UPDATE publication_variant_batches SET status = $1, completed_at = NOW() WHERE id = $2 AND status IN ('queued','processing')",
		status, batchID)
	if err != nil {
		return err
	}
	if res == 0 {
		return nil // worker lain sudah memfinalkan
	}
	if status == "failed" || status == "partial" {
		d.notifyVariantBatchFailed(ctx, batchID, status)
	}
	return nil
}

// notifyVariantBatchFailed: beri tahu admin platform saat batch gagal/sebagian
// (port notifyAdminsVariantBatchFailed — tipe `variant_batch.failed`).
func (d *Deps) notifyVariantBatchFailed(ctx context.Context, batchID, status string) {
	row, err := d.queryRow(ctx, `
		SELECT b.mode, br.name AS brand_name,
		       (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id) AS total,
		       (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'failed') AS failed,
		       (SELECT string_agg(m.name, ', ' ORDER BY v.created_at)
		        FROM publication_variants v JOIN media m ON m.id = v.media_id
		        WHERE v.batch_id = b.id AND v.status = 'failed') AS failed_media
		FROM publication_variant_batches b
		JOIN brands br ON br.id = b.brand_id
		WHERE b.id = $1 LIMIT 1`, batchID)
	if err != nil || row == nil {
		return
	}
	brandName := row.strOr("brand_name", "")
	if brandName == "" {
		brandName = "Brand"
	}
	total := row.intOr("total", 0)
	failed := row.intOr("failed", 0)
	mediaList := strings.TrimSpace(row.strOr("failed_media", ""))
	mode := "parafrase"
	if row.strOr("mode", "generate") == "generate" {
		mode = "dibuatkan dari bahan"
	}
	body := "Batch " + mode + " " + brandName + ": " + itoa(failed) + " dari " + itoa(total) + " artikel gagal dibuat"
	if mediaList != "" {
		body += " (" + mediaList + ")"
	}
	body += "."
	title := "Batch variant gagal"
	if status == "partial" {
		title = "Sebagian artikel variant gagal"
	}
	link := "/admin/variants"
	d.notifyPlatformAdmins(ctx, "variant_batch.failed", title, body, &link)
}

// ---------- value helpers (payload job) ----------

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return strings.TrimSpace(jsonStringify(t))
	}
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// asErr: errors.As tanpa impor errors di setiap pemanggil.
func asErr(err error, target any) bool {
	switch t := target.(type) {
	case **insufficientCreditsError:
		if e, ok := err.(*insufficientCreditsError); ok {
			*t = e
			return true
		}
	}
	return false
}

var _ = time.Now
