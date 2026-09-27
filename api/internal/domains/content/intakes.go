package contentdomain

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/[brandId]/content-intakes/* (fase 4 slice B):
// list, create, detail, update (+readiness), analyze, create-job.

type intakeJSONRow struct {
	ID              string
	BrandID         string
	Mode            string
	ContentTypeCode *string
	Topic           *string
	Answers         []byte
	ArticleText     *string
	ArticleFilename *string
	KeyMessages     []byte
	ReadinessScore  *int
	ReadinessChecks []byte
	Recommendation  []byte
	Status          string
	JobID           *string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (row intakeJSONRow) toMap() map[string]any {
	keyMessages := []any{}
	if len(row.KeyMessages) > 0 {
		_ = json.Unmarshal(row.KeyMessages, &keyMessages)
	}
	return map[string]any{
		"id": row.ID, "brand_id": row.BrandID, "mode": row.Mode,
		"content_type_code": row.ContentTypeCode, "topic": row.Topic,
		"answers": jsonRaw(row.Answers, map[string]any{}), "article_text": row.ArticleText,
		"article_filename": row.ArticleFilename, "readiness_score": row.ReadinessScore,
		"readiness_checks": jsonRaw(row.ReadinessChecks, []any{}), "recommendation": jsonRaw(row.Recommendation, map[string]any{}),
		"status": row.Status, "job_id": row.JobID, "key_messages": keyMessages,
		"created_by": row.CreatedBy, "created_at": isoMilli(row.CreatedAt), "updated_at": isoMilli(row.UpdatedAt),
	}
}

const intakeSelect = `SELECT id, brand_id, mode, content_type_code, topic, answers, article_text, article_filename,
	key_messages, readiness_score, readiness_checks, recommendation, status, job_id, created_by, created_at, updated_at
	FROM content_intakes`

func scanIntake(scan func(...any) error) (intakeJSONRow, error) {
	var row intakeJSONRow
	err := scan(&row.ID, &row.BrandID, &row.Mode, &row.ContentTypeCode, &row.Topic, &row.Answers,
		&row.ArticleText, &row.ArticleFilename, &row.KeyMessages, &row.ReadinessScore,
		&row.ReadinessChecks, &row.Recommendation, &row.Status, &row.JobID, &row.CreatedBy,
		&row.CreatedAt, &row.UpdatedAt)
	return row, err
}

func (d *Deps) loadIntake(r *http.Request, intakeID string) (intakeJSONRow, error) {
	return scanIntake(d.Pool.QueryRow(r.Context(), intakeSelect+" WHERE id = $1 LIMIT 1", intakeID).Scan)
}

// IntakeList: GET /content-intakes
func (d *Deps) IntakeList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	rowsQ, err := d.Pool.Query(r.Context(), intakeSelect+" WHERE brand_id = $1 ORDER BY created_at DESC LIMIT 50", brand.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()
	items := []map[string]any{}
	for rowsQ.Next() {
		if row, scanErr := scanIntake(rowsQ.Scan); scanErr == nil {
			items = append(items, row.toMap())
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

// IntakeCreate: POST /content-intakes
func (d *Deps) IntakeCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Mode            string   `json:"mode"`
		ContentTypeCode *string  `json:"content_type_code"`
		Topic           *string  `json:"topic"`
		KeyMessages     []string `json:"key_messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	switch body.Mode {
	case "bahan", "artikel", "keduanya", "rekomendasi":
	default:
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid enum value. Expected 'bahan' | 'artikel' | 'keduanya' | 'rekomendasi'")
		return
	}
	if body.ContentTypeCode != nil && *body.ContentTypeCode != "" {
		code := strings.TrimSpace(*body.ContentTypeCode)
		if len(code) < 2 || len(code) > 50 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid content_type_code")
			return
		}
		var found string
		if err := d.Pool.QueryRow(r.Context(),
			"SELECT id FROM content_intake_types WHERE code = $1 AND status = 'active' LIMIT 1", code).Scan(&found); err != nil {
			httpx.Error(w, http.StatusNotFound, "Jenis konten tidak ditemukan")
			return
		}
	}
	if len(body.KeyMessages) > 20 {
		httpx.Error(w, http.StatusUnprocessableEntity, "key_messages maksimal 20")
		return
	}
	for _, m := range body.KeyMessages {
		if len(strings.TrimSpace(m)) == 0 || len(m) > 500 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid key_messages")
			return
		}
	}
	var topic *string
	if body.Topic != nil {
		t := strings.TrimSpace(*body.Topic)
		if len(t) > 300 {
			httpx.Error(w, http.StatusUnprocessableEntity, "topic maksimal 300 karakter")
			return
		}
		if t != "" {
			topic = &t
		}
	}
	var typeCode *string
	if body.ContentTypeCode != nil && *body.ContentTypeCode != "" {
		c := strings.TrimSpace(*body.ContentTypeCode)
		typeCode = &c
	}

	id := uuid.NewString()
	keyMessagesJSON, _ := json.Marshal(orEmpty(body.KeyMessages))
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO content_intakes (id, brand_id, mode, content_type_code, topic, key_messages, answers, article_text,
		  article_filename, readiness_score, readiness_checks, recommendation, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, '{}'::jsonb, NULL, NULL, NULL, '[]'::jsonb, '{}'::jsonb, 'draft', $7)`,
		id, brand.ID, body.Mode, typeCode, topic, string(keyMessagesJSON), user.ID); err != nil {
		writeErr(w, err)
		return
	}
	_, _ = d.Pool.Exec(r.Context(),
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'content.intake_created', 'content_intake', $4)",
		uuid.NewString(), brand.ID, user.ID, id)

	row, err := d.loadIntake(r, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(row.toMap())
}

// IntakeGet: GET /content-intakes/{intakeId}
func (d *Deps) IntakeGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	row, err := d.loadIntake(r, chiParam(r, "intakeId"))
	if err != nil || row.BrandID != brand.ID {
		httpx.Error(w, http.StatusNotFound, "Intake not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(row.toMap())
}

// IntakeUpdate: PUT /content-intakes/{intakeId} — hitung ulang readiness.
func (d *Deps) IntakeUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	intakeID := chiParam(r, "intakeId")
	row, err := d.loadIntake(r, intakeID)
	if err != nil || row.BrandID != brand.ID {
		httpx.Error(w, http.StatusNotFound, "Intake not found")
		return
	}
	if row.Status == "job_created" {
		httpx.Error(w, http.StatusConflict, "Intake ini sudah diproses menjadi content job dan tidak bisa diubah lagi")
		return
	}

	var body struct {
		ContentTypeCode *string        `json:"content_type_code"`
		Topic           *string        `json:"topic"`
		Answers         map[string]any `json:"answers"`
		ArticleText     *string        `json:"article_text"`
		ArticleFilename *string        `json:"article_filename"`
		KeyMessages     []string       `json:"key_messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	if body.ArticleText != nil && len(*body.ArticleText) > 200_000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "article_text terlalu panjang")
		return
	}
	if body.ArticleFilename != nil && len(*body.ArticleFilename) > 255 {
		httpx.Error(w, http.StatusUnprocessableEntity, "article_filename terlalu panjang")
		return
	}
	if len(body.KeyMessages) > 20 {
		httpx.Error(w, http.StatusUnprocessableEntity, "key_messages maksimal 20")
		return
	}

	nextType := row.ContentTypeCode
	if body.ContentTypeCode != nil {
		if c := strings.TrimSpace(*body.ContentTypeCode); c == "" {
			nextType = nil
		} else {
			nextType = &c
		}
	}
	topic := row.Topic
	if body.Topic != nil {
		t := strings.TrimSpace(*body.Topic)
		if t == "" {
			topic = nil
		} else if len(t) > 300 {
			httpx.Error(w, http.StatusUnprocessableEntity, "topic maksimal 300 karakter")
			return
		} else {
			topic = &t
		}
	}
	answers := map[string]any{}
	if len(row.Answers) > 0 {
		_ = json.Unmarshal(row.Answers, &answers)
	}
	if body.Answers != nil {
		answers = body.Answers
	}
	articleText := row.ArticleText
	if body.ArticleText != nil {
		articleText = body.ArticleText
	}
	articleFilename := row.ArticleFilename
	if body.ArticleFilename != nil {
		articleFilename = body.ArticleFilename
	}
	keyMessages := []string{}
	if len(row.KeyMessages) > 0 {
		_ = json.Unmarshal(row.KeyMessages, &keyMessages)
	}
	if body.KeyMessages != nil {
		keyMessages = body.KeyMessages
	}

	intakeType := contentengine.LoadIntakeType(r.Context(), d.Pool, derefStr(nextType))
	readiness := contentengine.ComputeReadiness(intakeType, row.Mode, answers, topic, articleText)
	status := "draft"
	if row.Status == "ready" || readiness.Score >= 80 {
		status = "ready"
	}

	answersJSON, _ := json.Marshal(answers)
	keyMessagesJSON, _ := json.Marshal(orEmpty(keyMessages))
	checksJSON, _ := json.Marshal(readiness.Checks)
	if _, err := d.Pool.Exec(r.Context(), `
		UPDATE content_intakes SET content_type_code = $1, topic = $2, answers = $3::jsonb, article_text = $4,
		  article_filename = $5, key_messages = $6::jsonb, readiness_score = $7, readiness_checks = $8::jsonb,
		  status = $9, updated_at = NOW()
		WHERE id = $10 AND brand_id = $11`,
		nextType, topic, string(answersJSON), articleText, articleFilename, string(keyMessagesJSON),
		readiness.Score, string(checksJSON), status, intakeID, brand.ID); err != nil {
		writeErr(w, err)
		return
	}

	updated, err := d.loadIntake(r, intakeID)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := updated.toMap()
	out["readiness"] = readiness
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// IntakeAnalyze: POST /content-intakes/{intakeId}/analyze
func (d *Deps) IntakeAnalyze(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	intakeID := chiParam(r, "intakeId")
	row, err := d.loadIntake(r, intakeID)
	if err != nil || row.BrandID != brand.ID {
		httpx.Error(w, http.StatusNotFound, "Intake not found")
		return
	}
	answers := map[string]any{}
	if len(row.Answers) > 0 {
		_ = json.Unmarshal(row.Answers, &answers)
	}
	intakeType := contentengine.LoadIntakeType(r.Context(), d.Pool, derefStr(row.ContentTypeCode))
	readiness := contentengine.ComputeReadiness(intakeType, row.Mode, answers, row.Topic, row.ArticleText)
	status := "draft"
	if readiness.Score >= 80 {
		status = "ready"
	}
	checksJSON, _ := json.Marshal(readiness.Checks)
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE content_intakes SET readiness_score = $1, readiness_checks = $2::jsonb, status = $3, updated_at = NOW() WHERE id = $4",
		readiness.Score, string(checksJSON), status, intakeID); err != nil {
		writeErr(w, err)
		return
	}
	out := row.toMap()
	out["readiness_score"] = readiness.Score
	out["readiness_checks"] = readiness.Checks
	out["status"] = status
	out["readiness"] = readiness
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// IntakeCreateJob: POST /content-intakes/{intakeId}/create-job
func (d *Deps) IntakeCreateJob(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	intakeID := chiParam(r, "intakeId")
	row, err := d.loadIntake(r, intakeID)
	if err != nil || row.BrandID != brand.ID {
		httpx.Error(w, http.StatusNotFound, "Intake not found")
		return
	}

	engineRow, err := contentengine.LoadIntake(r.Context(), d.Pool, intakeID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Intake not found")
		return
	}
	result, err := contentengine.CreateJobFromIntake(r.Context(), d.Pool, brand.ID, user.ID, engineRow)
	if err != nil {
		var se *contentengine.IntakeStatusError
		if errors.As(err, &se) {
			httpx.Error(w, se.Status, se.Message)
			return
		}
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(result)
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// IntakeTypesList: GET /content-intake-types — tipe aktif untuk form intake.
func (d *Deps) IntakeTypesList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	rowsQ, err := d.Pool.Query(r.Context(), `
		SELECT id, code, name, description, questions, status, created_at, updated_at
		FROM content_intake_types WHERE status = 'active' ORDER BY name`)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()
	items := []map[string]any{}
	for rowsQ.Next() {
		var id, code, name, status string
		var description *string
		var questions []byte
		var createdAt, updatedAt time.Time
		if rowsQ.Scan(&id, &code, &name, &description, &questions, &status, &createdAt, &updatedAt) == nil {
			items = append(items, map[string]any{
				"id": id, "code": code, "name": name, "description": description,
				"questions": contentengine.ParseQuestions(questions), "status": status,
				"created_at": isoMilli(createdAt), "updated_at": isoMilli(updatedAt),
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}
