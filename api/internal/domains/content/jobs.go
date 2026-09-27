// Package contentdomain: port route v1/brands/[brandId]/content-jobs/* dan
// documents/* (fase 4). Engine ada di internal/content.
package contentdomain

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

type Deps struct {
	Pool      *pgxpool.Pool
	JobInline bool
}

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func (d *Deps) access(r *http.Request, brandID, userID, minRole string) (brandsdomain.BrandRow, error) {
	return brandsdomain.BrandAccess(r.Context(), d.Pool, brandID, userID, minRole)
}

func writeErr(w http.ResponseWriter, err error) {
	if be, ok := err.(*brandsdomain.BrandError); ok {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, err.Error())
}

// ---------- job JSON (parity outline/evaluation/source_snapshot) ----------

type jobRow struct {
	ID               string
	BrandID          string
	BriefID          string
	Status           string
	CurrentStep      string
	Progress         int
	ModelTier        string
	ResolvedModel    *string
	OutlineStructure []byte
	DraftContent     *string
	Evaluation       []byte
	SourceSnapshot   []byte
	ErrorMessage     *string
	CancelRequested  bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Topic            *string
	TargetWords      *int
	DocumentID       *string
	PubTotal         int
	PubPublished     int
}

func (j jobRow) toMap() map[string]any {
	out := map[string]any{
		"id": j.ID, "brand_id": j.BrandID, "brief_id": j.BriefID, "status": j.Status,
		"current_step": j.CurrentStep, "progress": j.Progress, "model_tier": j.ModelTier,
		"resolved_model": j.ResolvedModel, "error_message": j.ErrorMessage,
		"cancel_requested": j.CancelRequested,
		"created_at":       isoMilli(j.CreatedAt), "updated_at": isoMilli(j.UpdatedAt),
		"outline":         jsonRaw(j.OutlineStructure, []any{}),
		"evaluation":      jsonRaw(j.Evaluation, map[string]any{}),
		"source_snapshot": jsonRaw(j.SourceSnapshot, []any{}),
		"draft_html":      j.DraftContent,
	}
	if j.Topic != nil {
		out["topic"] = *j.Topic
	}
	if j.TargetWords != nil {
		out["target_words"] = *j.TargetWords
	}
	if j.DocumentID != nil {
		out["document_id"] = *j.DocumentID
	}
	if j.PubTotal > 0 {
		out["pub_total"] = j.PubTotal
		out["pub_published"] = j.PubPublished
	}
	return out
}

// loadJob: withBrief=true menyertakan topic/target_words/pub counts (parity
// bentuk GET /content-jobs/:id dan PUT outline).
func (d *Deps) loadJob(ctx context.Context, brandID, jobID string, withBrief bool) (jobRow, error) {
	var j jobRow
	if withBrief {
		err := d.Pool.QueryRow(ctx, `
			SELECT j.id, j.brand_id, j.brief_id, j.status, j.current_step, j.progress, j.model_tier, j.resolved_model,
			       j.outline_structure, j.draft_content, j.evaluation, j.source_snapshot, j.error_message,
			       j.cancel_requested, j.created_at, j.updated_at, b.topic, b.target_words, d.id,
			       (SELECT COUNT(*) FROM publications p WHERE p.document_id = d.id),
			       (SELECT COUNT(*) FROM publications p WHERE p.document_id = d.id AND p.status = 'published')
			FROM content_jobs j
			JOIN content_briefs b ON b.id = j.brief_id
			LEFT JOIN content_documents d ON d.job_id = j.id
			WHERE j.id = $1 AND j.brand_id = $2 LIMIT 1`, jobID, brandID).
			Scan(&j.ID, &j.BrandID, &j.BriefID, &j.Status, &j.CurrentStep, &j.Progress, &j.ModelTier,
				&j.ResolvedModel, &j.OutlineStructure, &j.DraftContent, &j.Evaluation, &j.SourceSnapshot,
				&j.ErrorMessage, &j.CancelRequested, &j.CreatedAt, &j.UpdatedAt,
				&j.Topic, &j.TargetWords, &j.DocumentID, &j.PubTotal, &j.PubPublished)
		return j, err
	}
	err := d.Pool.QueryRow(ctx, `
		SELECT j.id, j.brand_id, j.brief_id, j.status, j.current_step, j.progress, j.model_tier, j.resolved_model,
		       j.outline_structure, j.draft_content, j.evaluation, j.source_snapshot, j.error_message,
		       j.cancel_requested, j.created_at, j.updated_at, d.id
		FROM content_jobs j LEFT JOIN content_documents d ON d.job_id = j.id
		WHERE j.id = $1 AND j.brand_id = $2 LIMIT 1`, jobID, brandID).
		Scan(&j.ID, &j.BrandID, &j.BriefID, &j.Status, &j.CurrentStep, &j.Progress, &j.ModelTier,
			&j.ResolvedModel, &j.OutlineStructure, &j.DraftContent, &j.Evaluation, &j.SourceSnapshot,
			&j.ErrorMessage, &j.CancelRequested, &j.CreatedAt, &j.UpdatedAt, &j.DocumentID)
	return j, err
}

// ---------- GET/POST /content-jobs ----------

func (d *Deps) JobsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 100, 1, 100)
	offset := clampInt(r.URL.Query().Get("offset"), 0, 0, 1<<30)
	where := "WHERE j.brand_id = $1"
	if r.URL.Query().Get("has_article") == "1" {
		where = "WHERE j.brand_id = $1 AND d.id IS NOT NULL"
	}
	var total, active int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM content_jobs j JOIN content_briefs b ON b.id = j.brief_id LEFT JOIN content_documents d ON d.job_id = j.id "+where, row.ID).Scan(&total)
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM content_jobs WHERE brand_id = $1 AND status IN ('queued','running')", row.ID).Scan(&active)
	rowsQ, err := d.Pool.Query(r.Context(), `
		SELECT j.id, j.brand_id, j.brief_id, j.status, j.current_step, j.progress, j.model_tier, j.resolved_model,
		       j.outline_structure, j.draft_content, j.evaluation, j.source_snapshot, j.error_message,
		       j.cancel_requested, j.created_at, j.updated_at, b.topic, b.target_words, d.id,
		       (SELECT COUNT(*) FROM publications p WHERE p.document_id = d.id),
		       (SELECT COUNT(*) FROM publications p WHERE p.document_id = d.id AND p.status = 'published')
		FROM content_jobs j JOIN content_briefs b ON b.id = j.brief_id LEFT JOIN content_documents d ON d.job_id = j.id
		`+where+` ORDER BY j.created_at DESC LIMIT $2 OFFSET $3`, row.ID, limit, offset)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()
	items := []map[string]any{}
	for rowsQ.Next() {
		var j jobRow
		if rowsQ.Scan(&j.ID, &j.BrandID, &j.BriefID, &j.Status, &j.CurrentStep, &j.Progress, &j.ModelTier,
			&j.ResolvedModel, &j.OutlineStructure, &j.DraftContent, &j.Evaluation, &j.SourceSnapshot,
			&j.ErrorMessage, &j.CancelRequested, &j.CreatedAt, &j.UpdatedAt,
			&j.Topic, &j.TargetWords, &j.DocumentID, &j.PubTotal, &j.PubPublished) == nil {
			items = append(items, j.toMap())
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items": items, "total": total, "active": active, "limit": limit, "offset": offset,
	})
}

type briefBody struct {
	WebsiteID              *string  `json:"website_id"`
	ContentType            string   `json:"content_type"`
	Topic                  string   `json:"topic"`
	Angle                  *string  `json:"angle"`
	Goal                   string   `json:"goal"`
	Audience               *string  `json:"audience"`
	SearchIntent           *string  `json:"search_intent"`
	PrimaryKeyword         *string  `json:"primary_keyword"`
	SecondaryKeywords      []string `json:"secondary_keywords"`
	RequiredPoints         []string `json:"required_points"`
	RequiredContent        []string `json:"required_content"`
	ForbiddenContent       []string `json:"forbidden_content"`
	CTA                    *string  `json:"cta"`
	TargetWords            *int     `json:"target_words"`
	Language               *string  `json:"language"`
	RequireOutlineApproval bool     `json:"require_outline_approval"`
	IncludeTable           bool     `json:"include_table"`
	IncludeList            bool     `json:"include_list"`
	ModelTier              *string  `json:"model_tier"`
	IdempotencyKey         *string  `json:"idempotency_key"`
}

func (d *Deps) JobsCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	if brandRow.Status != "active" {
		httpx.Error(w, http.StatusConflict, "Activate the brand before generation")
		return
	}
	var body briefBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	topic := trimS(body.Topic)
	goal := trimS(body.Goal)
	contentType := body.ContentType
	if contentType == "" {
		contentType = "article"
	}
	lang := "id-ID"
	if body.Language != nil && *body.Language != "" {
		lang = *body.Language
	}
	if len(topic) < 3 || len(topic) > 300 || len(goal) < 3 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 3 character(s)")
		return
	}
	tier := "default"
	if body.ModelTier != nil && *body.ModelTier != "" {
		tier = *body.ModelTier
	}

	// Jumlah kata: admin default (brand_profiles) menang; cap 300..600.
	adminDefault := 0
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT default_target_words FROM brand_profiles WHERE brand_id = $1 LIMIT 1", brandRow.ID).Scan(&adminDefault)
	resolvedWords := 0
	if adminDefault > 0 {
		resolvedWords = clampInt(strconv.Itoa(adminDefault), 1000, 300, 600)
	} else {
		req := 1000
		if body.TargetWords != nil {
			req = *body.TargetWords
		}
		resolvedWords = clampInt(strconv.Itoa(req), 1000, 300, 600)
	}

	key := r.Header.Get("idempotency-key")
	if key == "" && body.IdempotencyKey != nil {
		key = *body.IdempotencyKey
	}
	if key == "" {
		key = "brief:" + uuid.NewString()[:12]
	}
	var existingID string
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT id FROM content_jobs WHERE brand_id = $1 AND idempotency_key = $2 LIMIT 1", brandRow.ID, key).Scan(&existingID); err == nil {
		j, loadErr := d.loadJob(r.Context(), brandRow.ID, existingID, false)
		if loadErr != nil {
			writeErr(w, loadErr)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(j.toMap())
		return
	}
	if body.WebsiteID != nil && *body.WebsiteID != "" {
		var wid string
		if err := d.Pool.QueryRow(r.Context(),
			"SELECT id FROM websites WHERE id = $1 AND brand_id = $2 LIMIT 1", *body.WebsiteID, brandRow.ID).Scan(&wid); err != nil {
			httpx.Error(w, http.StatusNotFound, "Website not found")
			return
		}
	}

	briefID := uuid.NewString()
	jobID := uuid.NewString()
	runtime, err := ai.LoadAIRuntime(r.Context(), d.Pool)
	if err != nil {
		writeErr(w, err)
		return
	}
	model := ai.ResolveModel(runtime, tier)
	secondary, _ := json.Marshal(orEmpty(body.SecondaryKeywords))
	requiredPoints, _ := json.Marshal(orEmpty(body.RequiredPoints))
	requiredContent, _ := json.Marshal(orEmpty(body.RequiredContent))
	forbiddenContent, _ := json.Marshal(orEmpty(body.ForbiddenContent))

	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO content_briefs (id, brand_id, website_id, content_type, topic, angle, goal, audience, search_intent,
		  primary_keyword, secondary_keywords, required_points, required_content, forbidden_content, cta, target_words,
		  language, require_outline_approval, include_table, include_list, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12::jsonb, $13::jsonb, $14::jsonb, $15, $16, $17, $18, $19, $20, $21)`,
		briefID, brandRow.ID, body.WebsiteID, contentType, topic, body.Angle, goal, body.Audience, body.SearchIntent,
		body.PrimaryKeyword, string(secondary), string(requiredPoints), string(requiredContent), string(forbiddenContent),
		body.CTA, resolvedWords, lang, body.RequireOutlineApproval, body.IncludeTable, body.IncludeList, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	promptSnapshot, _ := json.Marshal(map[string]any{"topic": topic, "target_words": resolvedWords})
	configSnapshot, _ := json.Marshal(map[string]any{"provider": runtime.Provider})
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO content_jobs (id, brand_id, brief_id, status, current_step, progress, idempotency_key, model_tier,
		  resolved_model, config_snapshot, prompt_snapshot, source_snapshot, evaluation, created_by)
		VALUES ($1, $2, $3, 'queued', 'queued', 0, $4, $5, $6, $7::jsonb, $8::jsonb, '[]'::jsonb, '{}'::jsonb, $9)`,
		jobID, brandRow.ID, briefID, key, tier, model, string(configSnapshot), string(promptSnapshot), user.ID); err != nil {
		writeErr(w, err)
		return
	}

	goalPtr := goal
	payload := contentengine.JobPayload{
		BrandID: brandRow.ID, JobID: jobID, UserID: user.ID, Topic: topic,
		ContentType: contentType, TargetWords: resolvedWords, Tier: tier,
		SearchIntent: body.SearchIntent, Angle: body.Angle, Audience: body.Audience, Goal: &goalPtr,
	}

	if body.RequireOutlineApproval {
		draft := contentengine.RunContentJob(r.Context(), d.Pool, contentengine.RunOptions{
			BrandID: payload.BrandID, UserID: payload.UserID, Topic: topic, ContentType: contentType,
			TargetWords: resolvedWords, RequireOutlineApproval: true, Tier: tier,
			SearchIntent: body.SearchIntent, Angle: body.Angle, Audience: body.Audience, Goal: &goalPtr,
		}, runtime)
		outlineJSON, _ := json.Marshal(draft.OutlineStructure)
		_, _ = d.Pool.Exec(r.Context(),
			"UPDATE content_jobs SET status = $1, current_step = $2, progress = $3, outline_structure = $4 WHERE id = $5 AND brand_id = $6",
			draft.Status, draft.CurrentStep, draft.Progress, string(outlineJSON), jobID, brandRow.ID)
	} else if d.JobInline {
		jobCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		_ = contentengine.ProcessContentGeneration(jobCtx, d.Pool, payload, 1, 3)
		cancel()
	} else {
		jobPayload, _ := json.Marshal(payload)
		_, _ = d.Pool.Exec(r.Context(),
			"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES ($1, 'content_generation', $2::jsonb, 'pending', 0, 3, NOW())",
			uuid.NewString(), string(jobPayload))
	}

	j, err := d.loadJob(r.Context(), brandRow.ID, jobID, false)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(j.toMap())
}

// ---------- GET /content-jobs/{jobId} + actions ----------

func (d *Deps) JobGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	j, err := d.loadJob(r.Context(), row.ID, chiParam(r, "jobId"), true)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Content job not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(j.toMap())
}

func (d *Deps) JobApproveOutline(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	jobID := chiParam(r, "jobId")
	j, err := d.loadJob(r.Context(), row.ID, jobID, false)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Content job not found")
		return
	}
	if j.Status != "waiting_outline_approval" {
		httpx.Error(w, http.StatusConflict, "Job is not waiting for outline approval")
		return
	}
	var topic, contentType string
	var targetWords int
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT topic, content_type, target_words FROM content_briefs WHERE id = $1 AND brand_id = $2 LIMIT 1",
		j.BriefID, row.ID).Scan(&topic, &contentType, &targetWords); err != nil {
		httpx.Error(w, http.StatusNotFound, "Brief not found")
		return
	}
	runtime, err := ai.LoadAIRuntime(r.Context(), d.Pool)
	if err != nil {
		writeErr(w, err)
		return
	}
	model := ai.ResolveModel(runtime, "default")
	key := "approve:" + jobID + ":" + uuid.NewString()[:8]
	configSnapshot, _ := json.Marshal(map[string]any{"provider": runtime.Provider})
	if _, err := d.Pool.Exec(r.Context(), `
		UPDATE content_jobs SET current_step = 'outline_approved', status = 'queued', idempotency_key = $1,
		  model_tier = 'default', resolved_model = $2, config_snapshot = $3::jsonb
		WHERE id = $4 AND brand_id = $5`,
		key, model, string(configSnapshot), jobID, row.ID); err != nil {
		writeErr(w, err)
		return
	}

	payload := contentengine.JobPayload{
		BrandID: row.ID, JobID: jobID, UserID: user.ID, Topic: topic,
		ContentType: contentType, TargetWords: targetWords, Tier: "default",
	}
	if d.JobInline {
		jobCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		_ = contentengine.ProcessContentGeneration(jobCtx, d.Pool, payload, 1, 3)
		cancel()
	} else {
		jobPayload, _ := json.Marshal(payload)
		_, _ = d.Pool.Exec(r.Context(),
			"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES ($1, 'content_generation', $2::jsonb, 'pending', 0, 3, NOW())",
			uuid.NewString(), string(jobPayload))
	}

	updated, err := d.loadJob(r.Context(), row.ID, jobID, false)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated.toMap())
}

func (d *Deps) JobUpdateOutline(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	jobID := chiParam(r, "jobId")
	var body struct {
		Outline []contentengine.OutlineItem `json:"outline"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Body tidak valid")
		return
	}
	invalid := len(body.Outline) == 0 || len(body.Outline) > 40
	for _, item := range body.Outline {
		if (item.Level != 2 && item.Level != 3) || trimS(item.Title) == "" || len([]rune(item.Title)) > 300 {
			invalid = true
			break
		}
	}
	if invalid {
		httpx.Error(w, http.StatusUnprocessableEntity, "Outline harus berisi 1–40 bagian dengan level 2/3 dan judul tidak kosong")
		return
	}
	j, err := d.loadJob(r.Context(), row.ID, jobID, false)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Content job not found")
		return
	}
	if j.Status != "waiting_outline_approval" {
		httpx.Error(w, http.StatusConflict, "Job tidak dalam status menunggu outline")
		return
	}
	outlineJSON, _ := json.Marshal(body.Outline)
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE content_jobs SET outline_structure = $1::jsonb, updated_at = NOW() WHERE id = $2 AND brand_id = $3",
		string(outlineJSON), jobID, row.ID); err != nil {
		writeErr(w, err)
		return
	}
	updated, err := d.loadJob(r.Context(), row.ID, jobID, true)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated.toMap())
}

func (d *Deps) JobCancel(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	jobID := chiParam(r, "jobId")
	j, err := d.loadJob(r.Context(), row.ID, jobID, false)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Content job not found")
		return
	}
	switch j.Status {
	case "completed", "failed", "cancelled":
		httpx.Error(w, http.StatusConflict, "Job can no longer be cancelled")
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE content_jobs SET cancel_requested = TRUE, status = 'cancelled', current_step = 'cancelled' WHERE id = $1 AND brand_id = $2",
		jobID, row.ID); err != nil {
		writeErr(w, err)
		return
	}
	updated, err := d.loadJob(r.Context(), row.ID, jobID, false)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated.toMap())
}

// ---------- helpers ----------

func clampInt(raw string, def, min, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		n = def
	}
	if n < min {
		n = min
	}
	if n > max {
		n = max
	}
	return n
}

func trimS(s string) string { return strings.TrimSpace(s) }

func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func jsonRaw(b []byte, fallback any) any {
	if len(b) == 0 {
		return fallback
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil || v == nil {
		return fallback
	}
	return v
}

func isoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
