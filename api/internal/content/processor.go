package content

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
)

// Port proses job content_generation dari lib/jobs/queue.ts: load runtime →
// editorial rules → runContentJob → gate moderasi → update job → buat dokumen
// + versi (html) → reset intake saat gagal final.

type JobPayload struct {
	BrandID                string  `json:"brandId"`
	JobID                  string  `json:"jobId"`
	UserID                 string  `json:"userId"`
	Topic                  string  `json:"topic"`
	ContentType            string  `json:"contentType"`
	TargetWords            int     `json:"targetWords"`
	RequireOutlineApproval bool    `json:"requireOutlineApproval"`
	Tier                   string  `json:"tier"`
	SearchIntent           *string `json:"searchIntent"`
	Angle                  *string `json:"angle"`
	Audience               *string `json:"audience"`
	Goal                   *string `json:"goal"`
	SourceMaterial         *string `json:"sourceMaterial"`
	MediaContext           *string `json:"mediaContext"`
}

// ProcessContentGeneration: menjalankan pipeline satu job (dipakai route
// POST content-jobs saat JOB_INLINE dan worker).
func ProcessContentGeneration(ctx context.Context, pool *pgxpool.Pool, payload JobPayload, attempt, maxAttempts int) error {
	runtime, err := ai.LoadAIRuntime(ctx, pool)
	if err != nil {
		return err
	}

	// Editorial rules (content-type standard + global KB) — failure-tolerant.
	editorialRules := LoadEditorialRulesForGeneration(ctx, pool, payload.ContentType, payload.Topic, payload.SourceMaterial, nil)

	draft := RunContentJob(ctx, pool, RunOptions{
		BrandID: payload.BrandID, UserID: payload.UserID, Topic: payload.Topic,
		ContentType: payload.ContentType, TargetWords: payload.TargetWords,
		RequireOutlineApproval: payload.RequireOutlineApproval, Tier: payload.Tier,
		SearchIntent: payload.SearchIntent, Angle: payload.Angle, Audience: payload.Audience,
		Goal: payload.Goal, SourceMaterial: payload.SourceMaterial,
		EditorialRules: editorialRules, MediaContext: payload.MediaContext,
	}, runtime)

	// Gate moderasi sebelum dokumen dibuat (hanya draft penuh).
	if draft.Status == "completed" && draft.DraftContent != nil {
		title := payload.Topic
		if m := frontMatterRe.FindStringSubmatch(*draft.DraftContent); m != nil {
			if tm := regexpTitleRe.FindStringSubmatch(m[1]); tm != nil {
				title = strings.TrimSpace(tm[1])
			}
		}
		contentSnippet := *draft.DraftContent
		if len(contentSnippet) > 5000 {
			contentSnippet = contentSnippet[:5000]
		}
		mod := moderation.ModerateContent(ctx, pool, moderation.Input{
			Title: &title, Content: &contentSnippet,
			BrandID: &payload.BrandID, UserID: &payload.UserID,
		})
		moderation.RecordAudit(ctx, pool, "content_job", payload.JobID, mod, &payload.BrandID)
		if mod.Status == "blocked" {
			msg := "Konten tidak dapat diproses karena terindikasi tidak sesuai dengan kebijakan publikasi Tulisio."
			_, _ = pool.Exec(ctx,
				"UPDATE content_jobs SET status='failed', error_message=$1 WHERE id=$2",
				msg, payload.JobID)
			return fmt.Errorf("%s Silakan periksa kembali materi yang Anda masukkan.", msg)
		}
		// review_required: lanjut buat dokumen; publication gate yang menahan.
	}

	outlineJSON, _ := json.Marshal(draft.OutlineStructure)
	evalJSON, _ := json.Marshal(draft.Evaluation)
	if _, err := pool.Exec(ctx, `
		UPDATE content_jobs SET status=$1, current_step=$2, progress=$3, outline_structure=$4,
		  draft_content=$5, draft_format=$6, evaluation=$7, error_message=$8 WHERE id=$9`,
		draft.Status, draft.CurrentStep, draft.Progress, string(outlineJSON),
		draft.DraftContent, draft.DraftFormat, string(evalJSON), draft.ErrorMessage, payload.JobID); err != nil {
		return err
	}

	if draft.Status == "completed" && draft.DraftContent != nil {
		if err := ensureDocumentFromDraft(ctx, pool, payload, *draft.DraftContent); err != nil {
			slog.Error("[content] buat dokumen gagal", "err", err, "job", payload.JobID)
		}
	}

	if draft.Status == "failed" {
		// Percobaan terakhir gagal → kembalikan intake ke draft (parity).
		if attempt >= maxAttempts {
			_, _ = pool.Exec(ctx,
				"UPDATE content_intakes SET status = 'draft', job_id = NULL, updated_at = NOW() WHERE job_id = $1 AND status = 'job_created'",
				payload.JobID)
		}
		if draft.ErrorMessage != nil {
			return fmt.Errorf("%s", *draft.ErrorMessage)
		}
		return fmt.Errorf("content generation failed")
	}
	return nil
}

var regexpTitleRe = regexp.MustCompile(`(?m)^title:\s*(.+)$`)

// ensureDocumentFromDraft: parity blok queue.ts — dokumen + versi html
// (judul dari front matter; H1 body dihapus; versi disimpan sebagai HTML).
func ensureDocumentFromDraft(ctx context.Context, pool *pgxpool.Pool, payload JobPayload, draftContent string) error {
	var existingID string
	err := pool.QueryRow(ctx, "SELECT id FROM content_documents WHERE job_id = $1 LIMIT 1", payload.JobID).Scan(&existingID)
	if err == nil {
		return nil // sudah ada (idempoten)
	}

	meta := ParseFrontMatter(draftContent, payload.Topic)
	docID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO content_documents (id, brand_id, job_id, status, title, slug, meta_title, meta_description, current_version, created_by, approved_by, approved_at)
		VALUES ($1, $2, $3, 'approved', $4, $5, $6, $7, 1, $8, $9, NOW())`,
		docID, payload.BrandID, payload.JobID,
		truncRunes(meta.Title, 300), Slugify(meta.Title), truncRunes(meta.MetaTitle, 255), truncRunes(meta.MetaDescription, 500),
		payload.UserID, payload.UserID); err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by)
		VALUES ($1, $2, 1, $3, 'html', 'ai_generated', 'Initial AI draft', $4)`,
		uuid.NewString(), docID, MarkdownToHtml(meta.Body), payload.UserID)
	return err
}
