package content

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
)

// Port lib/intake/scoring.ts + lib/intake/create-job.ts — readiness berbasis
// bobot pertanyaan skema (admin-managed), plus pembuatan brief+job dari intake
// dengan sourceMaterial (BAHAN jawaban + ARTIKEL existing) dan gate moderasi.

type IntakeQuestion struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Weight      int      `json:"weight"`
	Helper      *string  `json:"helper"`
	Placeholder *string  `json:"placeholder"`
	Options     []string `json:"options"`
}

type IntakeType struct {
	ID          string           `json:"id"`
	Code        string           `json:"code"`
	Name        string           `json:"name"`
	Description *string          `json:"description"`
	Questions   []IntakeQuestion `json:"questions"`
	Status      string           `json:"status"`
}

type ReadinessCheck struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Status   string `json:"status"` // ok | warn | missing
	Weight   int    `json:"weight"`
	Required bool   `json:"required"`
}

type ReadinessResult struct {
	Score   int              `json:"score"`
	Level   string           `json:"level"` // ready | needs_more | insufficient
	Checks  []ReadinessCheck `json:"checks"`
	Missing []string         `json:"missing"`
}

// ParseQuestions: filter entri yang punya key+label string (parity).
func ParseQuestions(value []byte) []IntakeQuestion {
	if len(value) == 0 {
		return []IntakeQuestion{}
	}
	var raw []map[string]any
	if err := json.Unmarshal(value, &raw); err != nil {
		return []IntakeQuestion{}
	}
	out := []IntakeQuestion{}
	for _, item := range raw {
		key, kOK := item["key"].(string)
		label, lOK := item["label"].(string)
		if !kOK || !lOK {
			continue
		}
		q := IntakeQuestion{Key: key, Label: label}
		if t, ok := item["type"].(string); ok {
			q.Type = t
		}
		if req, ok := item["required"].(bool); ok {
			q.Required = req
		}
		if w, ok := item["weight"].(float64); ok {
			q.Weight = int(w)
		}
		if h, ok := item["helper"].(string); ok {
			q.Helper = &h
		}
		if p, ok := item["placeholder"].(string); ok {
			q.Placeholder = &p
		}
		if opts, ok := item["options"].([]any); ok {
			for _, o := range opts {
				if s, ok := o.(string); ok {
					q.Options = append(q.Options, s)
				}
			}
		}
		out = append(out, q)
	}
	return out
}

// IsAnswered: null/undefined/kosong → belum dijawab (parity).
func IsAnswered(q IntakeQuestion, answers map[string]any) bool {
	value, ok := answers[q.Key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []any:
		return len(v) > 0
	case float64:
		return true
	case bool:
		return v
	default:
		return true
	}
}

// StringifyAnswer: parity stringifyAnswer.
func StringifyAnswer(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := []string{}
		for _, item := range v {
			if s := fmt.Sprint(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	default:
		return fmt.Sprint(v)
	}
}

func modeIncludesArticle(mode string) bool {
	return mode == "artikel" || mode == "keduanya"
}

// ComputeReadiness: port computeReadiness — bobot pertanyaan + topik wajib
// (jika skema tak punya pertanyaan "topic") + artikel existing saat mode
// menyertakan artikel. Ambang: ≥80 ready, ≥50 needs_more, selain itu insufficient.
func ComputeReadiness(intakeType *IntakeType, mode string, answers map[string]any, topic *string, articleText *string) ReadinessResult {
	questions := []IntakeQuestion{}
	if intakeType != nil {
		questions = intakeType.Questions
	}
	checks := []ReadinessCheck{}
	missing := []string{}
	achieved, total := 0, 0

	for _, q := range questions {
		total += q.Weight
		if IsAnswered(q, answers) {
			achieved += q.Weight
			checks = append(checks, ReadinessCheck{q.Key, q.Label, "ok", q.Weight, q.Required})
		} else if q.Required {
			missing = append(missing, q.Label)
			checks = append(checks, ReadinessCheck{q.Key, q.Label, "missing", 0, true})
		} else {
			checks = append(checks, ReadinessCheck{q.Key, q.Label, "warn", 0, false})
		}
	}

	hasTopicQuestion := false
	for _, q := range questions {
		if q.Key == "topic" {
			hasTopicQuestion = true
			break
		}
	}
	if !hasTopicQuestion {
		const weight = 10
		total += weight
		if topic != nil && len(strings.TrimSpace(*topic)) > 2 {
			achieved += weight
			checks = append(checks, ReadinessCheck{"topic", "Topik", "ok", weight, true})
		} else {
			missing = append(missing, "Topik")
			checks = append(checks, ReadinessCheck{"topic", "Topik", "missing", 0, true})
		}
	}

	if modeIncludesArticle(mode) {
		const weight = 15
		total += weight
		if articleText != nil && len(strings.TrimSpace(*articleText)) > 40 {
			achieved += weight
			checks = append(checks, ReadinessCheck{"existing_article", "Artikel existing", "ok", weight, true})
		} else {
			missing = append(missing, "Artikel existing")
			checks = append(checks, ReadinessCheck{"existing_article", "Artikel existing", "missing", 0, true})
		}
	}

	score := 0
	if total > 0 {
		score = int(roundHalf(float64(achieved) / float64(total) * 100))
	}
	level := "insufficient"
	if score >= 80 {
		level = "ready"
	} else if score >= 50 {
		level = "needs_more"
	}
	return ReadinessResult{Score: score, Level: level, Checks: checks, Missing: missing}
}

// LoadIntakeType: ambil tipe intake aktif per code.
func LoadIntakeType(ctx context.Context, pool *pgxpool.Pool, code string) *IntakeType {
	if code == "" {
		return nil
	}
	var t IntakeType
	var questions []byte
	err := pool.QueryRow(ctx, `
		SELECT id, code, name, description, questions, status
		FROM content_intake_types WHERE code = $1 AND status = 'active' LIMIT 1`, code).
		Scan(&t.ID, &t.Code, &t.Name, &t.Description, &questions, &t.Status)
	if err != nil {
		return nil
	}
	t.Questions = ParseQuestions(questions)
	return &t
}

// IntakeRow: bentuk baris content_intakes yang dipakai pipeline.
type IntakeRow struct {
	ID               string
	BrandID          string
	Mode             string
	ContentTypeCode  *string
	Topic            *string
	Answers          map[string]any
	ArticleText      *string
	ArticleFilename  *string
	KeyMessages      []string
	ReadinessScore   *int
	ReadinessChecks  []byte
	Recommendation   []byte
	Status           string
	JobID            *string
	CreatedBy        string
}

// LoadIntake: baris intake (error "Intake not found" bila tak ada).
func LoadIntake(ctx context.Context, pool *pgxpool.Pool, intakeID string) (IntakeRow, error) {
	var row IntakeRow
	var answers, keyMessages []byte
	err := pool.QueryRow(ctx, `
		SELECT id, brand_id, mode, content_type_code, topic, answers, article_text, article_filename,
		       key_messages, readiness_score, readiness_checks, recommendation, status, job_id, created_by
		FROM content_intakes WHERE id = $1 LIMIT 1`, intakeID).
		Scan(&row.ID, &row.BrandID, &row.Mode, &row.ContentTypeCode, &row.Topic, &answers,
			&row.ArticleText, &row.ArticleFilename, &keyMessages, &row.ReadinessScore,
			&row.ReadinessChecks, &row.Recommendation, &row.Status, &row.JobID, &row.CreatedBy)
	if err != nil {
		return IntakeRow{}, err
	}
	if len(answers) > 0 {
		_ = json.Unmarshal(answers, &row.Answers)
	}
	if row.Answers == nil {
		row.Answers = map[string]any{}
	}
	var km []string
	if len(keyMessages) > 0 {
		_ = json.Unmarshal(keyMessages, &km)
	}
	row.KeyMessages = km
	return row, nil
}

// CreateJobFromIntake: port lib/intake/create-job.ts.
// Mengembalikan map {job_id, already_created, status?, current_step?, progress?}
// atau error dengan StatusCode 422 untuk kasus "isi dulu" / diblokir moderasi.
type IntakeStatusError struct {
	Status  int
	Message string
}

func (e *IntakeStatusError) Error() string { return e.Message }

func CreateJobFromIntake(ctx context.Context, pool *pgxpool.Pool, brandID, userID string, intake IntakeRow) (map[string]any, error) {
	intakeType := LoadIntakeType(ctx, pool, deref(intake.ContentTypeCode))

	// Target kata: ditentukan admin per brand (brand_profiles.default_target_words),
	// dibatasi 300–600 (standar jurnalistik Tulisia); fallback 600.
	adminDefault := 0
	_ = pool.QueryRow(ctx,
		"SELECT default_target_words FROM brand_profiles WHERE brand_id = $1 LIMIT 1", brandID).Scan(&adminDefault)
	targetWords := 600
	if adminDefault > 0 {
		targetWords = adminDefault
		if targetWords < 300 {
			targetWords = 300
		}
		if targetWords > 600 {
			targetWords = 600
		}
	}

	questions := []IntakeQuestion{}
	typeName := "Artikel"
	typeCode := "article"
	if intakeType != nil {
		questions = intakeType.Questions
		typeName = intakeType.Name
		typeCode = intakeType.Code
	}

	// sourceMaterial: BAHAN (jawaban kuesioner) + ARTIKEL EXISTING.
	answerLines := []string{}
	for _, q := range questions {
		if text := questionAnswerText(q, intake.Answers); text != "" {
			answerLines = append(answerLines, "- "+q.Label+": "+text)
		}
	}
	sourceParts := []string{}
	if len(answerLines) > 0 {
		sourceParts = append(sourceParts, "BAHAN (jawaban kuesioner):\n"+strings.Join(answerLines, "\n"))
	}
	if intake.ArticleText != nil && strings.TrimSpace(*intake.ArticleText) != "" {
		filename := "tempel teks"
		if intake.ArticleFilename != nil && *intake.ArticleFilename != "" {
			filename = *intake.ArticleFilename
		}
		sourceParts = append(sourceParts, "ARTIKEL EXISTING ("+filename+"):\n"+truncRunes(strings.TrimSpace(*intake.ArticleText), 40000))
	}
	sourceMaterial := truncRunes(strings.Join(sourceParts, "\n\n---\n\n"), 50000)

	topic := ""
	if intake.Topic != nil {
		topic = strings.TrimSpace(*intake.Topic)
	}
	topicFallback := topic
	if topicFallback == "" {
		topicFallback = "Artikel tanpa topik"
	}
	if topic == "" && sourceMaterial == "" {
		return nil, &IntakeStatusError{422, "Intake belum punya topik atau bahan — isi dulu sebelum membuat artikel"}
	}

	// Gate moderasi intake (hemat token: topic + fragments).
	var sourceMaterialPtr *string
	if sourceMaterial != "" {
		sourceMaterialPtr = &sourceMaterial
	}
	modContent := topicFallback
	if sourceMaterial != "" || intake.ArticleText != nil {
		combined := sourceMaterial + "\n\n" + deref(intake.ArticleText)
		modContent = truncRunes(combined, 5000)
		if modContent == "" {
			modContent = topicFallback
		}
	}
	mod := moderation.ModerateContent(ctx, pool, moderation.Input{
		Title: &topicFallback, Content: &modContent,
		BrandID: &brandID, UserID: &userID,
	})
	moderation.RecordAudit(ctx, pool, "intake", intake.ID, mod, &brandID)
	if mod.Status == "blocked" {
		return nil, &IntakeStatusError{422, moderation.ClientSafeMessage("blocked")}
	}

	goal := ""
	if why, ok := intake.Answers["why"].(string); ok {
		goal = strings.TrimSpace(why)
	}
	if goal == "" {
		goal = `Tulis artikel ` + strings.ToLower(typeName) + ` tentang "` + topicFallback + `" berdasarkan bahan yang diberikan`
	}

	requiredPoints := []string{}
	for _, q := range questions {
		if text := questionAnswerText(q, intake.Answers); text != "" {
			requiredPoints = append(requiredPoints, q.Label+": "+text)
		}
	}
	if len(requiredPoints) > 50 {
		requiredPoints = requiredPoints[:50]
	}

	// Idempotency: intake:{id} + logika stale-job (parity create-job.ts).
	idempotencyKey := "intake:" + intake.ID
	var oldID, oldStatus string
	err := pool.QueryRow(ctx,
		"SELECT id, status FROM content_jobs WHERE brand_id = $1 AND idempotency_key = $2 LIMIT 1",
		brandID, idempotencyKey).Scan(&oldID, &oldStatus)
	if err == nil {
		intakeStillLinked := intake.JobID != nil && *intake.JobID == oldID
		if intakeStillLinked && (oldStatus == "queued" || oldStatus == "running") {
			return map[string]any{"job_id": oldID, "already_created": true}, nil
		}
		if intakeStillLinked && oldStatus == "completed" {
			var docID string
			if pool.QueryRow(ctx, "SELECT id FROM content_documents WHERE job_id = $1 LIMIT 1", oldID).Scan(&docID) == nil {
				var promptSnapshot []byte
				_ = pool.QueryRow(ctx, "SELECT prompt_snapshot FROM content_jobs WHERE id = $1 LIMIT 1", oldID).Scan(&promptSnapshot)
				var parsed struct {
					Topic          any `json:"topic"`
					SourceMaterial any `json:"sourceMaterial"`
				}
				if json.Unmarshal(promptSnapshot, &parsed) == nil {
					oldTopic, _ := parsed.Topic.(string)
					oldSource, _ := parsed.SourceMaterial.(string)
					curSource := ""
					if sourceMaterial != "" {
						curSource = truncRunes(sourceMaterial, 2000)
					}
					if oldTopic == topicFallback && oldSource == curSource {
						return map[string]any{"job_id": oldID, "already_created": true}, nil
					}
				}
			}
		}
		// Sisa kasus: job lama tak berguna → bersihkan agar kunci sama bisa dipakai.
		var oldBrief string
		_ = pool.QueryRow(ctx, "SELECT brief_id FROM content_jobs WHERE id = $1 LIMIT 1", oldID).Scan(&oldBrief)
		_, _ = pool.Exec(ctx, "DELETE FROM content_documents WHERE job_id = $1", oldID)
		_, _ = pool.Exec(ctx, "DELETE FROM content_jobs WHERE id = $1", oldID)
		if oldBrief != "" {
			_, _ = pool.Exec(ctx,
				"DELETE FROM content_briefs WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM content_jobs WHERE brief_id = $1)", oldBrief)
		}
	}

	briefID := uuid.NewString()
	jobID := uuid.NewString()
	runtime, err := ai.LoadAIRuntime(ctx, pool)
	if err != nil {
		return nil, err
	}
	model := ai.ResolveModel(runtime, "default")

	angle := ""
	if a, ok := intake.Answers["angle"].(string); ok && strings.TrimSpace(a) != "" {
		angle = strings.TrimSpace(a)
	} else if len(intake.Recommendation) > 0 {
		var rec struct {
			Angle any `json:"angle"`
		}
		if json.Unmarshal(intake.Recommendation, &rec) == nil {
			if s, ok := rec.Angle.(string); ok {
				angle = truncRunes(s, 500)
			}
		}
	}
	var anglePtr *string
	if angle != "" {
		anglePtr = &angle
	}

	requiredJSON, _ := json.Marshal(requiredPoints)
	if _, err := pool.Exec(ctx, `
		INSERT INTO content_briefs (id, brand_id, content_type, topic, angle, goal, audience, search_intent,
		  primary_keyword, secondary_keywords, required_points, required_content, forbidden_content, cta,
		  target_words, language, require_outline_approval, include_table, include_list, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, NULL, 'informational', NULL, '[]'::jsonb, $7::jsonb, '[]'::jsonb, '[]'::jsonb,
		  NULL, $8, 'id-ID', FALSE, FALSE, FALSE, $9)`,
		briefID, brandID, typeCode, topicFallback, anglePtr, goal, string(requiredJSON), targetWords, userID); err != nil {
		return nil, err
	}

	// modelTier: default → premium (parity create-job.ts).
	modelTier := "premium"
	snapshotSource := (*string)(nil)
	if sourceMaterial != "" {
		s := truncRunes(sourceMaterial, 2000)
		snapshotSource = &s
	}
	configSnapshot, _ := json.Marshal(map[string]any{"provider": runtime.Provider, "source": "content_intake"})
	promptSnapshot, _ := json.Marshal(map[string]any{"topic": topicFallback, "sourceMaterial": snapshotSource})
	if _, err := pool.Exec(ctx, `
		INSERT INTO content_jobs (id, brand_id, brief_id, status, current_step, progress, idempotency_key, model_tier,
		  resolved_model, config_snapshot, prompt_snapshot, source_snapshot, evaluation, created_by)
		VALUES ($1, $2, $3, 'queued', 'queued', 0, $4, $5, $6, $7::jsonb, $8::jsonb, '[]'::jsonb, '{}'::jsonb, $9)`,
		jobID, brandID, briefID, idempotencyKey, modelTier, model,
		string(configSnapshot), string(promptSnapshot), userID); err != nil {
		return nil, err
	}

	// Intake → job_created; generation selalu tanpa outline approval (jurnalistik).
	if _, err := pool.Exec(ctx,
		"UPDATE content_intakes SET status = 'job_created', job_id = $1, updated_at = NOW() WHERE id = $2",
		jobID, intake.ID); err != nil {
		return nil, err
	}

	payload := JobPayload{
		BrandID: brandID, JobID: jobID, UserID: userID, Topic: topicFallback,
		ContentType: typeCode, TargetWords: targetWords, Tier: modelTier,
		SearchIntent: strPtrLocal("informational"), Angle: anglePtr, Goal: &goal,
		SourceMaterial: sourceMaterialPtr,
	}
	jobCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	_ = ProcessContentGeneration(jobCtx, pool, payload, 1, 3)
	cancel()

	var status, currentStep string
	var progress int
	_ = pool.QueryRow(ctx,
		"SELECT status, current_step, progress FROM content_jobs WHERE id = $1", jobID).
		Scan(&status, &currentStep, &progress)
	return map[string]any{
		"job_id": jobID, "already_created": false,
		"status": status, "current_step": currentStep, "progress": progress,
	}, nil
}

func questionAnswerText(q IntakeQuestion, answers map[string]any) string {
	value, ok := answers[q.Key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(StringifyAnswer(value))
}

func strPtrLocal(s string) *string { return &s }

func roundHalf(v float64) float64 {
	if v < 0 {
		return float64(int(v - 0.5))
	}
	return float64(int(v + 0.5))
}

var _ = pgx.ErrNoRows
