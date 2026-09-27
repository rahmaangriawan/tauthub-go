package brandflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// batches.go: port app/v1/brands/[brandId]/batches/**/route.ts + _batch.ts.

const batchMaxRows = 500

// batchCreate: validasi payload (port batchCreate zod).
type batchBriefRow struct {
	Topic             string
	Goal              string
	PrimaryKeyword    any
	SecondaryKeywords []string
	Audience          any
	Angle             any
	SearchIntent      any
	RequiredPoints    []string
	ForbiddenContent  []string
	CTA               any
	TargetWords       int
	Language          string
	ContentType       string
	WebsiteID         any
	IncludeTable      bool
	IncludeList       bool
	Raw               map[string]any
}

func parseBatchBrief(raw any) (batchBriefRow, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return batchBriefRow{}, errUnprocessable("Expected object, received array")
	}
	row := batchBriefRow{Raw: m, TargetWords: 1000, Language: "id-ID", ContentType: "article"}
	topic, ok := m["topic"].(string)
	if !ok || len([]rune(strings.TrimSpace(topic))) < 3 {
		return row, errUnprocessable("String must contain at least 3 character(s)")
	}
	if len([]rune(topic)) > 300 {
		return row, errUnprocessable("String must contain at most 300 character(s)")
	}
	row.Topic = strings.TrimSpace(topic)
	goal, ok := m["goal"].(string)
	if !ok || len([]rune(strings.TrimSpace(goal))) < 3 {
		return row, errUnprocessable("String must contain at least 3 character(s)")
	}
	if len([]rune(goal)) > 2000 {
		return row, errUnprocessable("String must contain at most 2000 character(s)")
	}
	row.Goal = strings.TrimSpace(goal)

	if kw, present := m["primary_keyword"]; present && kw != nil {
		s, isStr := kw.(string)
		if !isStr || len([]rune(s)) > 200 {
			return row, errUnprocessable("String must contain at most 200 character(s)")
		}
		row.PrimaryKeyword = s
	}
	if ids, ok := stringSliceField(m, "secondary_keywords"); ok {
		if len(ids) > 30 {
			return row, errUnprocessable("Array must contain at most 30 element(s)")
		}
		row.SecondaryKeywords = ids
	}
	for _, key := range []string{"audience", "angle", "search_intent", "cta"} {
		if v, present := m[key]; present && v != nil {
			s, isStr := v.(string)
			if !isStr {
				return row, errUnprocessable("Expected string, received number")
			}
			limit := 2000
			if key == "search_intent" {
				limit = 80
			}
			if len([]rune(s)) > limit {
				return row, errUnprocessable("String must contain at most " + itoa(limit) + " character(s)")
			}
			switch key {
			case "audience":
				row.Audience = s
			case "angle":
				row.Angle = s
			case "search_intent":
				row.SearchIntent = s
			case "cta":
				row.CTA = s
			}
		}
	}
	if pts, ok := stringSliceField(m, "required_points"); ok {
		if len(pts) > 50 {
			return row, errUnprocessable("Array must contain at most 50 element(s)")
		}
		row.RequiredPoints = pts
	}
	if fc, ok := stringSliceField(m, "forbidden_content"); ok {
		if len(fc) > 30 {
			return row, errUnprocessable("Array must contain at most 30 element(s)")
		}
		row.ForbiddenContent = fc
	}
	if tw, present := m["target_words"]; present {
		f, isNum := tw.(float64)
		if !isNum || f != float64(int(f)) || f < 300 || f > 10000 {
			return row, errUnprocessable("Number must be greater than or equal to 300")
		}
		row.TargetWords = int(f)
	}
	if s, ok := m["language"].(string); ok && s != "" {
		if len([]rune(s)) > 20 {
			return row, errUnprocessable("String must contain at most 20 character(s)")
		}
		row.Language = s
	}
	if s, ok := m["content_type"].(string); ok && s != "" {
		if len([]rune(s)) > 50 {
			return row, errUnprocessable("String must contain at most 50 character(s)")
		}
		row.ContentType = s
	}
	if v, present := m["website_id"]; present && v != nil {
		s, isStr := v.(string)
		if !isStr {
			return row, errUnprocessable("Expected string, received number")
		}
		row.WebsiteID = s
	}
	if b, ok := m["include_table"].(bool); ok {
		row.IncludeTable = b
	}
	if b, ok := m["include_list"].(bool); ok {
		row.IncludeList = b
	}
	return row, nil
}

// fingerprint: sha256(JSON.stringify(row)).
func fingerprint(row map[string]any) string {
	raw, err := json.Marshal(row)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// scopedBatch: batch milik brand; 404 bila tidak ada.
func (d *Deps) scopedBatch(ctx context.Context, brandID, batchID string) (bsRow, error) {
	row, err := d.queryRow(ctx, "SELECT * FROM content_batches WHERE id = $1 AND brand_id = $2", batchID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Batch not found")
	}
	return row, nil
}

// batchJSON: port batchJson (_batch.ts).
func (d *Deps) batchJSON(ctx context.Context, batch bsRow, includeItems bool) (map[string]any, error) {
	counts, err := d.queryRows(ctx,
		"SELECT status, COUNT(*) AS count FROM content_batch_items WHERE batch_id = $1 GROUP BY status", batch.strOr("id", ""))
	if err != nil {
		return nil, err
	}
	countMap := map[string]any{}
	for _, c := range counts {
		countMap[c.strOr("status", "")] = c.intOr("count", 0)
	}
	items := make([]map[string]any, 0)
	if includeItems {
		rows, err := d.queryRows(ctx,
			"SELECT id, row_number, brief_payload, status, job_id, retry_count, error_category, error_message, updated_at FROM content_batch_items WHERE batch_id = $1 ORDER BY row_number LIMIT 500",
			batch.strOr("id", ""))
		if err != nil {
			return nil, err
		}
		for _, item := range rows {
			items = append(items, map[string]any{
				"id": item.raw("id"), "batch_id": batch.strOr("id", ""), "brand_id": item.raw("brand_id"),
				"row_number": item.intOr("row_number", 0), "fingerprint": item.raw("fingerprint"),
				"brief_payload": item.jsonField("brief_payload"), "status": item.raw("status"),
				"job_id": item.raw("job_id"), "retry_count": item.intOr("retry_count", 0),
				"error_category": item.raw("error_category"), "error_message": item.raw("error_message"),
				"created_at": item.timePtr("created_at"), "updated_at": item.timePtr("updated_at"),
			})
		}
	}
	out := rowToAnyMap(batch)
	out["counts"] = countMap
	out["items"] = items
	return out, nil
}

// batchCountsForMany: hitung status per banyak batch sekaligus.
func (d *Deps) batchCountsForMany(ctx context.Context, batchIDs []string) (map[string]map[string]int, error) {
	out := map[string]map[string]int{}
	if len(batchIDs) == 0 {
		return out, nil
	}
	if ctx == nil {
		return out, nil
	}
	rows, err := d.queryRows(ctx,
		"SELECT batch_id, status, COUNT(*) AS count FROM content_batch_items WHERE batch_id = ANY($1) GROUP BY batch_id, status",
		batchIDs)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		key := row.strOr("batch_id", "")
		entry := out[key]
		if entry == nil {
			entry = map[string]int{}
			out[key] = entry
		}
		entry[row.strOr("status", "")] = row.intOr("count", 0)
	}
	return out, nil
}

// ---------- GET / POST /v1/brands/{brandId}/batches ----------

// BatchesList: GET /v1/brands/{brandId}/batches
func (d *Deps) BatchesList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 50, 1, 200)
	offset := intParamDef(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	found, err := d.queryRows(r.Context(),
		"SELECT * FROM content_batches WHERE brand_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		brandRow.ID, limit, offset)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	ids := make([]string, 0, len(found))
	for _, b := range found {
		ids = append(ids, b.strOr("id", ""))
	}
	counts, err := d.batchCountsForMany(r.Context(), ids)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(found))
	for _, batch := range found {
		entry := rowToAnyMap(batch)
		key := batch.strOr("id", "")
		if c, ok := counts[key]; ok {
			entry["counts"] = c
		} else {
			entry["counts"] = map[string]int{}
		}
		entry["items"] = []any{}
		out = append(out, entry)
	}
	writeJSON(w, http.StatusOK, out)
}

// BatchesCreate: POST /v1/brands/{brandId}/batches
func (d *Deps) BatchesCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)

	name, ok := body["name"].(string)
	if !ok || len([]rune(strings.TrimSpace(name))) < 2 {
		d.writeErr(w, errUnprocessable("String must contain at least 2 character(s)"))
		return
	}
	if len([]rune(name)) > 200 {
		d.writeErr(w, errUnprocessable("String must contain at most 200 character(s)"))
		return
	}
	rawRows, ok := body["rows"].([]any)
	if !ok || len(rawRows) == 0 {
		d.writeErr(w, errUnprocessable("Array must contain at least 1 element(s)"))
		return
	}
	if len(rawRows) > batchMaxRows {
		d.writeErr(w, errUnprocessable("Array must contain at most 500 element(s)"))
		return
	}
	modelTier := "default"
	if s, ok := body["model_tier"].(string); ok {
		if s != "default" && s != "economy" && s != "premium" {
			d.writeErr(w, errUnprocessable("Invalid option: expected one of \"default\"|\"economy\"|\"premium\""))
			return
		}
		modelTier = s
	}
	concurrency := 3
	if f, ok := body["concurrency_limit"].(float64); ok {
		if f != float64(int(f)) || f < 1 || f > 10 {
			d.writeErr(w, errUnprocessable("Number must be less than or equal to 10"))
			return
		}
		concurrency = int(f)
	}

	parsed := make([]batchBriefRow, 0, len(rawRows))
	fingerprints := make([]string, 0, len(rawRows))
	for _, rawRow := range rawRows {
		brief, err := parseBatchBrief(rawRow)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		parsed = append(parsed, brief)
		if m, ok := rawRow.(map[string]any); ok {
			fingerprints = append(fingerprints, fingerprint(m))
		} else {
			fingerprints = append(fingerprints, "")
		}
	}
	seen := map[string]bool{}
	for _, fp := range fingerprints {
		if seen[fp] {
			d.writeErr(w, errConflict("Batch masih memiliki baris duplikat"))
			return
		}
		seen[fp] = true
	}

	// Kuota per batch: VVIP (brand_ambassador) dikecualikan.
	quota, err := d.queryRow(r.Context(),
		"SELECT plan_code, per_batch_limit FROM brand_ai_quotas WHERE brand_id = $1", brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	partner, err := d.queryRow(r.Context(),
		"SELECT p.is_vip, p.vip_tier FROM partners p JOIN users u ON u.id = p.user_id JOIN brand_memberships bm ON bm.brand_id = $1 AND bm.user_id = u.id WHERE p.status = 'active' LIMIT 1",
		brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	isVipBrand := partner != nil && partner.boolOr("is_vip", false) && partner.strOr("vip_tier", "regular") == "brand_ambassador"
	if !isVipBrand && quota != nil && len(rawRows) > quota.intOr("per_batch_limit", 0) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"detail": "Paket " + quota.strOr("plan_code", "") + " membatasi " + itoa(quota.intOr("per_batch_limit", 0)) + " artikel per batch",
		})
		return
	}
	if brandRow.Status != "active" {
		httpx.Error(w, http.StatusConflict, "Activate the brand before batch generation")
		return
	}

	websiteIDs := []string{}
	seenSite := map[string]bool{}
	for _, brief := range parsed {
		if s, ok := brief.WebsiteID.(string); ok && s != "" && !seenSite[s] {
			seenSite[s] = true
			websiteIDs = append(websiteIDs, s)
		}
	}
	for _, siteID := range websiteIDs {
		existing, err := d.queryRow(r.Context(),
			"SELECT id FROM websites WHERE id = $1 AND brand_id = $2 LIMIT 1", siteID, brandRow.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if existing == nil {
			httpx.Error(w, http.StatusNotFound, "Website not found")
			return
		}
	}

	batchID := uuid.NewString()
	estimatedTokens := estimateTokens(parsed)
	if err := d.exec(r.Context(),
		"INSERT INTO content_batches (id, brand_id, name, model_tier, concurrency_limit, status, total_items, estimated_tokens, created_by) VALUES ($1, $2, $3, $4, $5, 'draft', $6, $7, $8)",
		batchID, brandRow.ID, name, modelTier, concurrency, len(rawRows), estimatedTokens, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	for i, brief := range parsed {
		if err := d.exec(r.Context(),
			"INSERT INTO content_batch_items (id, brand_id, batch_id, row_number, fingerprint, brief_payload, status) VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'pending')",
			uuid.NewString(), brandRow.ID, batchID, i+1, fingerprints[i], marshalJSON(brief.Raw)); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "content.batch.created", "content_batch", batchID)
	created, err := d.queryRow(r.Context(), "SELECT * FROM content_batches WHERE id = $1", batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if created == nil {
		d.writeErr(w, errNotFound("Batch not found"))
		return
	}
	payload, err := d.batchJSON(r.Context(), created, true)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

// estimateTokens: parity estimateTokens (min 900 per baris).
func estimateTokens(rows []batchBriefRow) int {
	total := 0
	for _, row := range rows {
		v := int(float64(row.TargetWords)*1.8) + 1200
		if v < 900 {
			v = 900
		}
		total += v
	}
	return total
}

// ---------- GET /v1/brands/{brandId}/batches/{batchId} ----------

// BatchGet: GET /v1/brands/{brandId}/batches/{batchId}
func (d *Deps) BatchGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	batch, err := d.scopedBatch(r.Context(), brandRow.ID, chiParam(r, "batchId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.batchJSON(r.Context(), batch, true)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// ---------- pause / cancel / retry / start ----------

// BatchPause: POST /v1/brands/{brandId}/batches/{batchId}/pause
func (d *Deps) BatchPause(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	batchID := chiParam(r, "batchId")
	batch, err := d.scopedBatch(r.Context(), brandRow.ID, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if batch.strOr("status", "") != "running" {
		httpx.Error(w, http.StatusConflict, "Hanya batch running yang bisa di-pause")
		return
	}
	if err := d.exec(r.Context(), "UPDATE content_batches SET status = 'paused' WHERE id = $1", batchID); err != nil {
		d.writeErr(w, err)
		return
	}
	updated, err := d.scopedBatch(r.Context(), brandRow.ID, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.batchJSON(r.Context(), updated, true)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// BatchCancel: POST /v1/brands/{brandId}/batches/{batchId}/cancel
func (d *Deps) BatchCancel(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	batchID := chiParam(r, "batchId")
	batch, err := d.scopedBatch(r.Context(), brandRow.ID, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	status := batch.strOr("status", "")
	if status == "completed" || status == "cancelled" {
		httpx.Error(w, http.StatusConflict, "Batch sudah terminal")
		return
	}
	if err := d.exec(r.Context(), "UPDATE content_batches SET status = 'cancelled' WHERE id = $1", batchID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE content_batch_items SET status = 'cancelled' WHERE batch_id = $1 AND status IN ('pending','queued','running')", batchID); err != nil {
		d.writeErr(w, err)
		return
	}
	jobs, err := d.queryRows(r.Context(),
		"SELECT job_id FROM content_batch_items WHERE batch_id = $1 AND job_id IS NOT NULL", batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	for _, job := range jobs {
		if err := d.exec(r.Context(),
			"UPDATE content_jobs SET cancel_requested = TRUE, status = 'cancelled', current_step = 'cancelled' WHERE id = $1 AND status NOT IN ('completed','failed','cancelled')",
			job.strOr("job_id", "")); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	updated, err := d.scopedBatch(r.Context(), brandRow.ID, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.batchJSON(r.Context(), updated, true)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// BatchRetry: POST /v1/brands/{brandId}/batches/{batchId}/retry
func (d *Deps) BatchRetry(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	batchID := chiParam(r, "batchId")
	batch, err := d.scopedBatch(r.Context(), brandRow.ID, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items, err := d.queryRows(r.Context(),
		"SELECT * FROM content_batch_items WHERE batch_id = $1 AND status IN ('failed','dead_letter','cancelled') AND retry_count < 3 ORDER BY row_number", batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if len(items) == 0 {
		httpx.Error(w, http.StatusConflict, "Item belum bisa di-retry")
		return
	}
	if err := d.prepareBatchJobs(r.Context(), brandRow.ID, batchID, batch, items, true); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(), "UPDATE content_batches SET status = 'running' WHERE id = $1", batchID); err != nil {
		d.writeErr(w, err)
		return
	}
	updated, err := d.scopedBatch(r.Context(), brandRow.ID, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.batchJSON(r.Context(), updated, true)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

var batchStartAllowed = []string{"draft", "paused", "partial"}

// BatchStart: POST /v1/brands/{brandId}/batches/{batchId}/start
func (d *Deps) BatchStart(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	batchID := chiParam(r, "batchId")
	batch, err := d.scopedBatch(r.Context(), brandRow.ID, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !containsString(batchStartAllowed, batch.strOr("status", "")) {
		httpx.Error(w, http.StatusConflict, "Batch tidak bisa dijalankan dari status ini")
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE content_batches SET status = 'running' WHERE id = $1 AND brand_id = $2", batchID, brandRow.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	items, err := d.queryRows(r.Context(),
		"SELECT * FROM content_batch_items WHERE batch_id = $1 AND status = 'pending' ORDER BY row_number", batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.prepareBatchJobs(r.Context(), brandRow.ID, batchID, batch, items, false); err != nil {
		d.writeErr(w, err)
		return
	}
	finished, err := d.queryRows(r.Context(),
		"SELECT status, COUNT(*) AS count FROM content_batch_items WHERE batch_id = $1 GROUP BY status", batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	counts := map[string]int{}
	for _, c := range finished {
		counts[c.strOr("status", "")] = c.intOr("count", 0)
	}
	unfinished := counts["pending"] + counts["queued"] + counts["running"]
	if unfinished == 0 {
		failed := counts["failed"] + counts["cancelled"] + counts["dead_letter"]
		newStatus := "completed"
		if failed > 0 {
			newStatus = "partial"
		}
		if err := d.exec(r.Context(), "UPDATE content_batches SET status = $1 WHERE id = $2", newStatus, batchID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	updated, err := d.scopedBatch(r.Context(), brandRow.ID, batchID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.batchJSON(r.Context(), updated, true)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// prepareBatchJobs: port blok start/retry — buat brief + job + (opsional) generate inline.
func (d *Deps) prepareBatchJobs(ctx context.Context, brandID, batchID string, batch bsRow, items []bsRow, isRetry bool) error {
	runtime, _ := ai.LoadAIRuntime(ctx, d.Pool)
	configured := runtime != nil && runtime.BaseURL != "" && runtime.APIKey != "" && runtime.DefaultModel != ""
	modelTier := batch.strOr("model_tier", "default")
	createdBy := batch.strOr("created_by", "")

	for _, item := range items {
		payload := jsonUnmarshalMap(item.jsonField("brief_payload"))
		retryCount := item.intOr("retry_count", 0)
		if isRetry {
			if err := d.exec(ctx,
				"UPDATE content_batch_items SET retry_count = retry_count + 1, status = 'pending', job_id = NULL, error_category = NULL, error_message = NULL WHERE id = $1",
				item.strOr("id", "")); err != nil {
				return err
			}
		}
		briefID := uuid.NewString()
		jobID := uuid.NewString()
		topic := mapGetString(payload, "topic", "")
		contentType := mapGetString(payload, "content_type", "article")
		targetWords := mapGetInt(payload, "target_words", 1000)
		language := mapGetString(payload, "language", "id-ID")
		websiteID := nullableMapString(payload, "website_id")
		if err := d.exec(ctx,
			`INSERT INTO content_briefs (id, brand_id, website_id, content_type, topic, angle, goal, audience, search_intent, primary_keyword, secondary_keywords, required_points, required_content, forbidden_content, cta, target_words, language, require_outline_approval, include_table, include_list, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12::jsonb, $13::jsonb, $14::jsonb, $15, $16, $17, FALSE, $18, $19, $20)`,
			briefID, brandID, websiteID, contentType, topic,
			nullableMapString(payload, "angle"), mapGetGoal(payload, topic), nullableMapString(payload, "audience"),
			nullableMapString(payload, "search_intent"), nullableMapString(payload, "primary_keyword"),
			marshalJSON(mapGetStringSlice(payload, "secondary_keywords")), marshalJSON(mapGetStringSlice(payload, "required_points")),
			"[]", marshalJSON(mapGetStringSlice(payload, "forbidden_content")), nullableMapString(payload, "cta"),
			targetWords, language, mapGetBool(payload, "include_table"), mapGetBool(payload, "include_list"), createdBy); err != nil {
			return err
		}
		idempotencyKey := "batch:" + batchID + ":" + item.strOr("id", "") + ":" + itoa(retryCount)
		if isRetry {
			idempotencyKey = "batch:" + batchID + ":" + item.strOr("id", "") + ":" + itoa(retryCount+1)
		}
		provider := ""
		if runtime != nil {
			provider = runtime.Provider
		}
		if err := d.exec(ctx,
			`INSERT INTO content_jobs (id, brand_id, brief_id, status, current_step, progress, idempotency_key, model_tier, resolved_model, config_snapshot, prompt_snapshot, source_snapshot, evaluation, created_by)
			 VALUES ($1, $2, $3, 'queued', 'queued', 0, $4, $5, NULL, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10)`,
			jobID, brandID, briefID, idempotencyKey, modelTier,
			marshalJSON(map[string]any{"provider": provider}), marshalJSON(payload), "[]", "{}", createdBy); err != nil {
			return err
		}
		if isRetry {
			if err := d.exec(ctx, "UPDATE content_batch_items SET job_id = $1, status = 'queued' WHERE id = $2", jobID, item.strOr("id", "")); err != nil {
				return err
			}
		} else {
			if err := d.exec(ctx,
				"UPDATE content_batch_items SET job_id = $1, status = 'queued', error_category = NULL, error_message = NULL WHERE id = $2",
				jobID, item.strOr("id", "")); err != nil {
				return err
			}
		}
		if !configured {
			continue
		}
		draft := contentengine.RunContentJob(ctx, d.Pool, contentengine.RunOptions{
			BrandID: brandID, UserID: createdBy, Topic: topic, ContentType: contentType,
			TargetWords: targetWords, RequireOutlineApproval: false, Tier: modelTier,
		}, runtime)
		if err := d.exec(ctx,
			"UPDATE content_jobs SET status = $1, current_step = $2, progress = $3, outline_structure = $4::jsonb, draft_content = $5, draft_format = $6, evaluation = $7::jsonb, error_message = $8 WHERE id = $9",
			draft.Status, draft.CurrentStep, draft.Progress, marshalJSON(draft.OutlineStructure),
			draft.DraftContent, draft.DraftFormat, marshalJSON(draft.Evaluation), draft.ErrorMessage, jobID); err != nil {
			return err
		}
		newItemStatus := "failed"
		if draft.Status == "completed" {
			newItemStatus = "completed"
		}
		if err := d.exec(ctx, "UPDATE content_batch_items SET status = $1 WHERE id = $2", newItemStatus, item.strOr("id", "")); err != nil {
			return err
		}
	}
	return nil
}

// ---------- helper payload ----------

func mapGetString(m map[string]any, key, fallback string) string {
	if s, ok := m[key].(string); ok && s != "" {
		return s
	}
	return fallback
}

func mapGetGoal(m map[string]any, topic string) string {
	if s, ok := m["goal"].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return topic
}

func mapGetInt(m map[string]any, key string, fallback int) int {
	if f, ok := m[key].(float64); ok {
		return int(f)
	}
	return fallback
}

func mapGetBool(m map[string]any, key string) bool {
	if b, ok := m[key].(bool); ok {
		return b
	}
	return false
}

func nullableMapString(m map[string]any, key string) any {
	if s, ok := m[key].(string); ok && s != "" {
		return s
	}
	return nil
}

func mapGetStringSlice(m map[string]any, key string) []string {
	arr, ok := m[key].([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
