package contentdomain

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Port v1/brands/[brandId]/documents/* (fase 4): list, create-from-job,
// detail (documentJson), update (versi baru human_edited), html.

type docRow struct {
	ID               string
	JobID            string
	Status           string
	Title            string
	Slug             *string
	MetaTitle        *string
	MetaDescription  *string
	AssignedReviewer *string
	CurrentVersion   int
	ApprovedBy       *string
	ApprovedAt       *time.Time
	UpdatedAt        time.Time
	CreatedAt        time.Time
}

func (d *Deps) DocumentsList(w http.ResponseWriter, r *http.Request) {
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
	limit := clampInt(r.URL.Query().Get("limit"), 50, 1, 500)
	offset := clampInt(r.URL.Query().Get("offset"), 0, 0, 1<<30)
	rowsQ, err := d.Pool.Query(r.Context(), `
		SELECT id, job_id, status, title, slug, meta_title, meta_description, assigned_reviewer_id,
		       current_version, approved_by, approved_at, updated_at, created_at
		FROM content_documents WHERE brand_id = $1 ORDER BY updated_at DESC LIMIT $2 OFFSET $3`,
		row.ID, limit, offset)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()
	items := []map[string]any{}
	for rowsQ.Next() {
		var doc docRow
		if rowsQ.Scan(&doc.ID, &doc.JobID, &doc.Status, &doc.Title, &doc.Slug, &doc.MetaTitle,
			&doc.MetaDescription, &doc.AssignedReviewer, &doc.CurrentVersion, &doc.ApprovedBy,
			&doc.ApprovedAt, &doc.UpdatedAt, &doc.CreatedAt) == nil {
			items = append(items, map[string]any{
				"id": doc.ID, "job_id": doc.JobID, "status": doc.Status, "title": doc.Title,
				"slug": doc.Slug, "meta_title": doc.MetaTitle, "meta_description": doc.MetaDescription,
				"assigned_reviewer_id": doc.AssignedReviewer, "current_version": doc.CurrentVersion,
				"approved_by": doc.ApprovedBy, "approved_at": tsOrNil(doc.ApprovedAt),
				"updated_at": isoMilli(doc.UpdatedAt), "created_at": isoMilli(doc.CreatedAt),
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

// DocumentsCreate: dari job completed → dokumen + versi 1 (ai_generated).
func (d *Deps) DocumentsCreate(w http.ResponseWriter, r *http.Request) {
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
	var body struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JobID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}

	var existingID, existingBrand string
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT id, brand_id FROM content_documents WHERE job_id = $1 LIMIT 1", body.JobID).Scan(&existingID, &existingBrand); err == nil {
		if existingBrand != brandRow.ID {
			httpx.Error(w, http.StatusConflict, "Content job not found")
			return
		}
		out, err := d.documentJSON(r.Context(), brandRow.ID, existingID)
		if err != nil {
			writeErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
		return
	}

	var jobID, briefID, draftContent, draftFormat, topic string
	err = d.Pool.QueryRow(r.Context(), `
		SELECT j.id, j.brief_id, j.draft_content, j.draft_format, b.topic
		FROM content_jobs j JOIN content_briefs b ON b.id = j.brief_id
		WHERE j.id = $1 AND j.brand_id = $2 AND j.status = 'completed' AND j.draft_content IS NOT NULL LIMIT 1`,
		body.JobID, brandRow.ID).Scan(&jobID, &briefID, &draftContent, &draftFormat, &topic)
	if err != nil {
		httpx.Error(w, http.StatusConflict, "A completed content job is required")
		return
	}

	id := uuid.NewString()
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO content_documents (id, brand_id, job_id, status, title, slug, current_version, created_by)
		VALUES ($1, $2, $3, 'draft', $4, $5, 1, $6)`,
		id, brandRow.ID, jobID, topic, contentengine.Slugify(topic), user.ID); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by)
		VALUES ($1, $2, 1, $3, $4, 'ai_generated', 'Initial AI draft', $5)`,
		uuid.NewString(), id, draftContent, draftFormat, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'content.document.created', 'content_document', $4)",
		uuid.NewString(), brandRow.ID, user.ID, id); err != nil {
		writeErr(w, err)
		return
	}
	out, err := d.documentJSON(r.Context(), brandRow.ID, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(out)
}

func (d *Deps) DocumentGet(w http.ResponseWriter, r *http.Request) {
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
	out, err := d.documentJSON(r.Context(), row.ID, chiParam(r, "documentId"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Document not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

var h1Re = regexp.MustCompile(`(?is)<h1[^>]*>([\s\S]*?)</h1>`)
var tagRe = regexp.MustCompile(`<[^>]+>`)
var wsRe = regexp.MustCompile(`\s+`)

func articleTitle(html string) *string {
	m := h1Re.FindStringSubmatch(html)
	if m == nil {
		return nil
	}
	text := strings.TrimSpace(wsRe.ReplaceAllString(tagRe.ReplaceAllString(m[1], " "), " "))
	if text == "" {
		return nil
	}
	return &text
}

// DocumentUpdate: versi baru human_edited; status approved → kembali draft
// (parity: guard export).
func (d *Deps) DocumentUpdate(w http.ResponseWriter, r *http.Request) {
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
	documentID := chiParam(r, "documentId")
	var body struct {
		HTML            *string `json:"html"`
		Content         *string `json:"content"`
		ContentFormat   *string `json:"content_format"`
		ChangeNote      *string `json:"change_note"`
		BaseVersion     *int    `json:"base_version"`
		Slug            *string `json:"slug"`
		MetaTitle       *string `json:"meta_title"`
		MetaDescription *string `json:"meta_description"`
		Title           *string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	content := ""
	if body.HTML != nil {
		content = *body.HTML
	} else if body.Content != nil {
		content = *body.Content
	}
	if content == "" || len(content) > 2_000_000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	// Netralkan konstruk berbahaya (script, event handler, iframe non-embed
	// tepercaya, skema javascript:/data:) SEBELUM disimpan. Artikel normal tidak
	// berubah sama sekali — sanitizer punya jalur cepat yang mengembalikan string
	// apa adanya bila tak ada penanda berbahaya.
	if safe, changed := security.SanitizeArticleHTML(content); changed {
		content = safe
		_, _ = d.Pool.Exec(r.Context(), `
			INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id)
			VALUES (gen_random_uuid(), $1, $2, 'content.html_sanitized', 'content_document', $3)`,
			brandRow.ID, user.ID, documentID)
	}

	var currentVersion int
	var status, title string
	err = d.Pool.QueryRow(r.Context(),
		"SELECT current_version, status, title FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1",
		documentID, brandRow.ID).Scan(&currentVersion, &status, &title)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Document not found")
		return
	}
	if body.BaseVersion != nil && *body.BaseVersion != currentVersion {
		httpx.Error(w, http.StatusConflict, "A newer version exists; reload before saving")
		return
	}
	nextVersion := currentVersion + 1
	nextTitle := title
	if t := articleTitle(content); t != nil {
		nextTitle = *t
	} else if body.Title != nil && *body.Title != "" {
		nextTitle = *body.Title
	}
	changeNote := "Autosave"
	if body.ChangeNote != nil {
		changeNote = *body.ChangeNote
	}
	format := "html"
	if body.ContentFormat != nil && *body.ContentFormat != "" {
		format = *body.ContentFormat
	}

	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by)
		VALUES ($1, $2, $3, $4, $5, 'human_edited', $6, $7)`,
		uuid.NewString(), documentID, nextVersion, content, format, changeNote, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := d.Pool.Exec(r.Context(), `
		UPDATE content_documents SET current_version = $1, title = $2,
		  slug = COALESCE($3, slug), meta_title = COALESCE($4, meta_title), meta_description = COALESCE($5, meta_description),
		  status = CASE WHEN status = 'approved' THEN 'draft' ELSE status END,
		  approved_by = CASE WHEN status = 'approved' THEN NULL ELSE approved_by END,
		  approved_at = CASE WHEN status = 'approved' THEN NULL ELSE approved_at END
		WHERE id = $6`,
		nextVersion, nextTitle, body.Slug, body.MetaTitle, body.MetaDescription, documentID); err != nil {
		writeErr(w, err)
		return
	}
	_, _ = d.Pool.Exec(r.Context(),
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'content.document.version_created', 'content_document', $4)",
		uuid.NewString(), brandRow.ID, user.ID, documentID)

	out, err := d.documentJSON(r.Context(), brandRow.ID, documentID)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (d *Deps) DocumentHTML(w http.ResponseWriter, r *http.Request) {
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
	documentID := chiParam(r, "documentId")
	var currentVersion int
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT current_version FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1",
		documentID, row.ID).Scan(&currentVersion); err != nil {
		httpx.Error(w, http.StatusNotFound, "Document not found")
		return
	}
	var content, format string
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT content, content_format FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		documentID, currentVersion).Scan(&content, &format)
	if format == "" {
		format = "html"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"content": content, "content_format": format})
}

// documentJSON: parity documentJson — dokumen + versi + komentar + versi aktif.
func (d *Deps) documentJSON(ctx context.Context, brandID, documentID string) (map[string]any, error) {
	var doc docRow
	err := d.Pool.QueryRow(ctx, `
		SELECT id, job_id, status, title, slug, meta_title, meta_description, assigned_reviewer_id,
		       current_version, approved_by, approved_at, updated_at, created_at
		FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1`, documentID, brandID).
		Scan(&doc.ID, &doc.JobID, &doc.Status, &doc.Title, &doc.Slug, &doc.MetaTitle, &doc.MetaDescription,
			&doc.AssignedReviewer, &doc.CurrentVersion, &doc.ApprovedBy, &doc.ApprovedAt, &doc.UpdatedAt, &doc.CreatedAt)
	if err != nil {
		return nil, err
	}

	versions := []map[string]any{}
	vRows, err := d.Pool.Query(ctx, `
		SELECT id, version, content AS html, attribution, change_note, created_by, created_at
		FROM content_versions WHERE document_id = $1 ORDER BY version DESC`, documentID)
	if err == nil {
		defer vRows.Close()
		for vRows.Next() {
			var id string
			var version int
			var html string
			var attribution string
			var changeNote *string
			var createdBy *string
			var createdAt time.Time
			if vRows.Scan(&id, &version, &html, &attribution, &changeNote, &createdBy, &createdAt) == nil {
				versions = append(versions, map[string]any{
					"id": id, "version": version, "html": html, "attribution": attribution,
					"change_note": changeNote, "created_by": createdBy, "created_at": isoMilli(createdAt),
				})
			}
		}
	}

	comments := []map[string]any{}
	cRows, err := d.Pool.Query(ctx, `
		SELECT id, version, body, selection_quote, line_start, line_end, resolved, created_by, created_at
		FROM review_comments WHERE document_id = $1 ORDER BY created_at`, documentID)
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var id, body string
			var version *int
			var quote *string
			var lineStart, lineEnd *int
			var resolved bool
			var createdBy *string
			var createdAt time.Time
			if cRows.Scan(&id, &version, &body, &quote, &lineStart, &lineEnd, &resolved, &createdBy, &createdAt) == nil {
				comments = append(comments, map[string]any{
					"id": id, "version": version, "body": body, "selection_quote": quote,
					"line_start": lineStart, "line_end": lineEnd, "resolved": resolved,
					"created_by": createdBy, "created_at": isoMilli(createdAt),
				})
			}
		}
	}

	var sourceSnapshot, evaluation []byte
	_ = d.Pool.QueryRow(ctx,
		"SELECT source_snapshot, evaluation FROM content_jobs WHERE id = $1 LIMIT 1", doc.JobID).
		Scan(&sourceSnapshot, &evaluation)

	var current any
	for _, v := range versions {
		if v["version"] == doc.CurrentVersion {
			current = v
			break
		}
	}
	if current == nil {
		return nil, errDocumentVersionMissing
	}

	return map[string]any{
		"id": doc.ID, "job_id": doc.JobID, "status": doc.Status, "title": doc.Title,
		"slug": doc.Slug, "meta_title": doc.MetaTitle, "meta_description": doc.MetaDescription,
		"assigned_reviewer_id": doc.AssignedReviewer, "current_version": doc.CurrentVersion,
		"approved_by": doc.ApprovedBy, "approved_at": tsOrNil(doc.ApprovedAt),
		"updated_at": isoMilli(doc.UpdatedAt), "created_at": isoMilli(doc.CreatedAt),
		"content": current, "versions": versions, "comments": comments,
		"source_snapshot": jsonRaw(sourceSnapshot, []any{}),
		"evaluation":      jsonRaw(evaluation, map[string]any{}),
	}, nil
}

var errDocumentVersionMissing = &docVersionError{}

type docVersionError struct{}

func (e *docVersionError) Error() string { return "Document version is missing" }

func tsOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return isoMilli(*t)
}
