package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Handler fase 3 — port route.ts di app/v1/brands/[brandId]/knowledge/*.
// authorize per route: baca = role apa pun; tulis = strategist; chat-learning
// DELETE = admin. Pesan & status code dipertahankan.

type Deps struct {
	Pool *pgxpool.Pool
}

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

// access: reuse BrandAccess dari package brands (cache 30s + 404 lintas-tenant).
func (d *Deps) access(r *http.Request, brandID, userID, minRole string) (brandsdomain.BrandRow, error) {
	return brandsdomain.BrandAccess(r.Context(), d.Pool, brandID, userID, minRole)
}

// ---------- collections ----------

func (d *Deps) CollectionsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 50, 1, 500)
	offset := clampInt(r.URL.Query().Get("offset"), 0, 0, 1<<30)
	rows, err := d.Pool.Query(r.Context(),
		"SELECT id, name, description, created_at FROM knowledge_collections WHERE brand_id = $1 ORDER BY created_at LIMIT $2 OFFSET $3",
		brandID, limit, offset)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name string
		var description *string
		var createdAt any
		if rows.Scan(&id, &name, &description, &createdAt) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "description": description, "created_at": createdAt})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

func (d *Deps) CollectionsCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "strategist"); err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	name := trimS(body.Name)
	if len(name) < 2 || len(name) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 2 character(s)")
		return
	}
	id := uuidNew()
	if _, err := d.Pool.Exec(r.Context(),
		"INSERT INTO knowledge_collections (id, brand_id, name, description, created_by) VALUES ($1, $2, $3, $4, $5)",
		id, brandID, name, body.Description, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	var createdAt any
	var description *string
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT id, name, description, created_at FROM knowledge_collections WHERE id = $1", id).
		Scan(new(string), new(string), &description, &createdAt)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "name": name, "description": description, "created_at": createdAt})
}

func (d *Deps) CollectionGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	item, err := Collection(r.Context(), d.Pool, chiParam(r, "brandId"), chiParam(r, "collectionId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

func (d *Deps) CollectionUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "strategist"); err != nil {
		writeErr(w, err)
		return
	}
	collectionID := chiParam(r, "collectionId")
	if _, err := Collection(r.Context(), d.Pool, brandID, collectionID); err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	if len(trimS(body.Name)) < 2 || len(trimS(body.Name)) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 2 character(s)")
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE knowledge_collections SET name = $1, description = $2 WHERE id = $3 AND brand_id = $4",
		trimS(body.Name), body.Description, collectionID, brandID); err != nil {
		writeErr(w, err)
		return
	}
	item, err := Collection(r.Context(), d.Pool, brandID, collectionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

func (d *Deps) CollectionDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "strategist"); err != nil {
		writeErr(w, err)
		return
	}
	collectionID := chiParam(r, "collectionId")
	if _, err := Collection(r.Context(), d.Pool, brandID, collectionID); err != nil {
		writeErr(w, err)
		return
	}
	var total int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM knowledge_sources WHERE collection_id = $1 AND deleted_at IS NULL", collectionID).Scan(&total)
	if total > 0 {
		httpx.Error(w, http.StatusConflict, "Collection still has active sources")
		return
	}
	for _, q := range []string{
		"DELETE FROM knowledge_chunks WHERE collection_id = $1",
		"DELETE FROM knowledge_sources WHERE collection_id = $1",
		"DELETE FROM knowledge_collections WHERE id = $1 AND brand_id = $2",
	} {
		if _, err := d.Pool.Exec(r.Context(), q, collectionID, brandID); err != nil {
			writeErr(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- sources ----------

func (d *Deps) SourcesList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	collectionID := chiParam(r, "collectionId")
	if _, err := d.access(r, brandID, user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := Collection(r.Context(), d.Pool, brandID, collectionID); err != nil {
		writeErr(w, err)
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 50, 1, 500)
	offset := clampInt(r.URL.Query().Get("offset"), 0, 0, 1<<30)
	var total int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) as total FROM knowledge_sources WHERE collection_id = $1 AND deleted_at IS NULL", collectionID).Scan(&total)
	rows, err := d.Pool.Query(r.Context(), `
		SELECT id, collection_id, title, source_type, original_filename, status, error_message, created_at
		FROM knowledge_sources WHERE collection_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, collectionID, limit, offset)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()
	sources := []map[string]any{}
	for rows.Next() {
		var id, collection, title, sourceType string
		var originalFilename, status *string
		var errorMessage *string
		var createdAt any
		if rows.Scan(&id, &collection, &title, &sourceType, &originalFilename, &status, &errorMessage, &createdAt) == nil {
			sources = append(sources, map[string]any{
				"id": id, "collection_id": collection, "title": title, "source_type": sourceType,
				"original_filename": originalFilename, "status": status, "error_message": errorMessage,
				"created_at": createdAt,
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"sources": sources, "total": total})
}

func (d *Deps) SourcesCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	collectionID := chiParam(r, "collectionId")
	if _, err := d.access(r, brandID, user.ID, "strategist"); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := Collection(r.Context(), d.Pool, brandID, collectionID); err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	// parity sourceSchema: title 2..255, content 20..2_000_000
	title := trimS(body.Title)
	if len(title) < 2 || len(title) > 255 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 2 character(s)")
		return
	}
	if len(body.Content) < 20 || len(body.Content) > 2_000_000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 20 character(s)")
		return
	}
	created, err := InsertSource(r.Context(), d.Pool, InsertSourceOptions{
		BrandID: brandID, CollectionID: collectionID, UserID: user.ID,
		Title: title, Content: body.Content,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	created["status"] = "active"
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(created)
}

func (d *Deps) SourceGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	item, err := Source(r.Context(), d.Pool, chiParam(r, "brandId"), chiParam(r, "sourceId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

func (d *Deps) SourceDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "strategist"); err != nil {
		writeErr(w, err)
		return
	}
	sourceID := chiParam(r, "sourceId")
	if _, err := Source(r.Context(), d.Pool, brandID, sourceID); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE knowledge_sources SET deleted_at = NOW() WHERE id = $1 AND brand_id = $2",
		sourceID, brandID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Deps) SourceContent(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	var content string
	err := d.Pool.QueryRow(r.Context(),
		"SELECT content FROM knowledge_sources WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL LIMIT 1",
		chiParam(r, "sourceId"), brandID).Scan(&content)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Source not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"content": content})
}

// ---------- sitemap ----------

func (d *Deps) SitemapCrawl(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "strategist"); err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		CollectionID string `json:"collection_id"`
		SitemapURL   string `json:"sitemap_url"`
		MaxPages     *int   `json:"max_pages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	maxPages := 50
	if body.MaxPages != nil {
		maxPages = *body.MaxPages
	}
	if maxPages < 1 {
		maxPages = 1
	}
	if maxPages > 200 {
		maxPages = 200
	}
	if body.CollectionID == "" || len(body.SitemapURL) < 10 || len(body.SitemapURL) > 2000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	coll, err := Collection(r.Context(), d.Pool, brandID, body.CollectionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	collectionID, _ := coll["id"].(string)
	// Satu crawl aktif per brand: konsumer antrean dipakai bersama semua tenant.
	var active int
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT count(*) FROM knowledge_crawl_runs WHERE brand_id = $1 AND status IN ('pending','running') AND created_at > NOW() - INTERVAL '30 minutes'",
		brandID).Scan(&active); err != nil {
		writeErr(w, err)
		return
	}
	if active > 0 {
		httpx.Error(w, http.StatusConflict, "Masih ada crawl sitemap yang berjalan untuk brand ini. Tunggu sampai selesai.")
		return
	}
	runID, err := CreateCrawlRun(r.Context(), d.Pool, brandID, collectionID, body.SitemapURL, maxPages)
	if err != nil {
		writeErr(w, err)
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"runId": runID, "brandId": brandID, "collectionId": collectionID,
		"sitemapUrl": body.SitemapURL, "maxPages": maxPages, "userId": user.ID,
	})
	if _, err := d.Pool.Exec(r.Context(),
		"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES ($1, 'sitemap_crawl', $2::jsonb, 'pending', 0, 3, NOW())",
		uuidNew(), string(payload)); err != nil {
		writeErr(w, err)
		return
	}
	// parity: void drainJobs(1) — proses inline fire-and-forget. Context
	// TERPISAH: request context dibatalkan saat 202 terkirim.
	jobCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	go func() {
		defer recoverCrawlPanic()
		defer cancel()
		_ = ProcessSitemapCrawlJob(jobCtx, d.Pool, payload)
	}()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{"run_id": runID, "queued": true})
}

func (d *Deps) SitemapStatus(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		httpx.Error(w, http.StatusBadRequest, "run_id required")
		return
	}
	run, err := GetCrawlRun(r.Context(), d.Pool, runID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(run)
}

// ---------- chat-learning ----------

func (d *Deps) ChatLearningList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	page := clampInt(r.URL.Query().Get("page"), 1, 1, 1<<30)
	pageSize := clampInt(r.URL.Query().Get("page_size"), 25, 1, 100)
	where := "brand_id = $1"
	args := []any{brandID}
	if intent := r.URL.Query().Get("intent"); intent != "" {
		args = append(args, intent)
		where += " AND intent_tag = $" + strconv.Itoa(len(args))
	}
	if search := r.URL.Query().Get("search"); search != "" {
		args = append(args, "%"+search+"%")
		where += " AND question LIKE $" + strconv.Itoa(len(args))
	}
	var total int
	_ = d.Pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM chat_learnings WHERE "+where, args...).Scan(&total)
	rows, err := d.Pool.Query(r.Context(), `
		SELECT id, brand_id, conversation_id, user_id, question, intent_tag, answer, context_used, benchmark_ids, created_at
		FROM chat_learnings WHERE `+where+` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, bID, conversationID, uID, question, intentTag, answer string
		var contextUsed, benchmarkIDs []byte
		var createdAt any
		if rows.Scan(&id, &bID, &conversationID, &uID, &question, &intentTag, &answer, &contextUsed, &benchmarkIDs, &createdAt) == nil {
			items = append(items, map[string]any{
				"id": id, "brand_id": bID, "conversation_id": conversationID, "user_id": uID,
				"question": question, "intent_tag": intentTag, "answer": answer,
				"context_used": jsonRaw(contextUsed), "benchmark_ids": jsonRaw(benchmarkIDs),
				"created_at": createdAt,
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (d *Deps) ChatLearningDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "admin"); err != nil {
		writeErr(w, err)
		return
	}
	intent := r.URL.Query().Get("intent")
	var deleted int64
	var err error
	if intent != "" {
		ct, e := d.Pool.Exec(r.Context(),
			"DELETE FROM chat_learnings WHERE brand_id = $1 AND intent_tag = $2", brandID, intent)
		deleted, err = ct.RowsAffected(), e
	} else {
		ct, e := d.Pool.Exec(r.Context(), "DELETE FROM chat_learnings WHERE brand_id = $1", brandID)
		deleted, err = ct.RowsAffected(), e
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"deleted": deleted})
}

// ---------- helpers ----------

func clampInt(raw string, def, min, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func trimS(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func jsonRaw(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	return v
}

func writeErr(w http.ResponseWriter, err error) {
	var be *brandsdomain.BrandError
	if errors.As(err, &be) {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, err.Error())
}
