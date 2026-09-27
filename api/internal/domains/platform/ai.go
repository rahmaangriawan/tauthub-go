package platform

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

// Modul AI gateway — port app/v1/platform/ai/** (7 endpoint). Semua endpoint
// admin-only (parity isPlatformAdmin → 404 "Resource not found").
//
// Config menyimpan API key TER-SEAL (t2:) dan tidak pernah mengembalikannya:
// respons hanya berisi flag api_key_configured + hint.

// AIConfigurationGet: GET /v1/platform/ai/configuration
func (d *Deps) AIConfigurationGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	rt, err := ai.LoadAIRuntime(r.Context(), d.Pool)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, aiConfigJSON(d.aiStoredConfig(r.Context()), rt))
}

// AIConfigurationPut: PUT /v1/platform/ai/configuration
func (d *Deps) AIConfigurationPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiParsedObject(w, d, r)
	if !okBody {
		return
	}
	// Schema (parity aiConfigurationSchema): base_url trim 8..500 (wajib),
	// api_key min 8 max 1000 nullable optional, default_model trim 1..200
	// (wajib), economy_model/premium_model trim max 200 nullable optional.
	baseRaw, err := aiRequiredStringField(body, "base_url", 8, 500)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	apiKey, hasAPIKey, err := aiStringField(body, "api_key", 8, 1000, false, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	defaultModel, err := aiRequiredStringField(body, "default_model", 1, 200)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	economy, hasEconomy, err := aiStringField(body, "economy_model", 0, 200, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	premium, hasPremium, err := aiStringField(body, "premium_model", 0, 200, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	base := aiNormalizeURL(baseRaw)
	if base == nil {
		writeErr(w, d, errBadRequest("Base URL must use HTTP or HTTPS and cannot contain credentials"))
		return
	}
	dm := strings.TrimSpace(defaultModel)
	if dm == "" {
		writeErr(w, d, errBadRequest("Default model is required"))
		return
	}
	// parity: `parsed.economy_model?.trim() || null`.
	var econ, prem any
	if hasEconomy && economy != "" {
		econ = economy
	}
	if hasPremium && premium != "" {
		prem = premium
	}

	existing, err := d.queryRow(ctx, "SELECT * FROM platform_ai_configuration WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if existing == nil && !(hasAPIKey && apiKey != "") {
		writeErr(w, d, errBadRequest("API key is required"))
		return
	}
	// parity: hint = api_key ? `••••${api_key.slice(-4)}` : existing?.api_key_hint ?? null
	var hint any
	if hasAPIKey && apiKey != "" {
		hint = "••••" + aiTail(apiKey, 4)
	} else if existing != nil {
		hint = ptNullableStr(existing, "api_key_hint")
	}

	if existing == nil {
		sealed, sErr := d.sealT2(apiKey)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		if err := d.exec(ctx,
			"INSERT INTO platform_ai_configuration (id, base_url, sealed_api_key, api_key_hint, default_model, economy_model, premium_model, updated_by) VALUES ('default', $1, $2, $3, $4, $5, $6, $7)",
			*base, sealed, hint, dm, econ, prem, user.ID); err != nil {
			writeErr(w, d, err)
			return
		}
	} else {
		query := "UPDATE platform_ai_configuration SET base_url = $1, default_model = $2, economy_model = $3, premium_model = $4, updated_by = $5"
		params := []any{*base, dm, econ, prem, user.ID}
		if hasAPIKey && apiKey != "" {
			sealed, sErr := d.sealT2(apiKey)
			if sErr != nil {
				writeErr(w, d, sErr)
				return
			}
			query += ", sealed_api_key = $6, api_key_hint = $7"
			params = append(params, sealed, hint)
		}
		query += " WHERE id = 'default'"
		if err := d.exec(ctx, query, params...); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	d.audit(ctx, user.ID, "platform.ai.configuration_updated", "platform_ai_configuration", "default")
	ai.InvalidateRuntimeCache()

	rt, _ := ai.LoadAIRuntime(ctx, d.Pool)
	writeJSON(w, http.StatusOK, aiConfigJSON(d.aiStoredConfig(ctx), rt))
}

// aiTail: n karakter terakhir (parity slice(-4)).
func aiTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// AIModelsList: GET /v1/platform/ai/models
func (d *Deps) AIModelsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	rt, err := ai.LoadAIRuntime(r.Context(), d.Pool)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	stored := d.aiStoredConfig(r.Context())

	rows, err := d.queryRows(r.Context(),
		`SELECT id, model_id, model_name, provider, reasoning_level, summary, characteristics,
		        suitable_for, unsuitable_for, guidelines, limitations, editorial_context,
		        status, chunk_count, created_at, updated_at
		 FROM ai_model_knowledge
		 ORDER BY updated_at DESC`)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	knowledgeMap := map[string]map[string]any{}
	for _, row := range rows {
		knowledgeMap[row.strOr("model_id", "")] = aiKnowledgeJSON(row)
	}

	// Model aktif (default/economy/premium) + model ber-knowledge + katalog.
	type activeModel struct {
		modelName string
		provider  string
		tier      string
		isActive  bool
	}
	order := []string{}
	activeModels := map[string]*activeModel{}

	defaultModelID := aiOrEmpty(stored.strOr("default_model", ""), rt.DefaultModel)
	economyModelID := aiOrEmpty(stored.strOr("economy_model", ""), aiOrEmpty(rt.EconomyModel, defaultModelID))
	premiumModelID := aiOrEmpty(stored.strOr("premium_model", ""), aiOrEmpty(rt.PremiumModel, defaultModelID))
	provider := aiOrEmpty(rt.Provider, "configured")

	add := func(id, name, prov, tier string, isActive bool) {
		if id == "" {
			return
		}
		if _, exists := activeModels[id]; exists {
			return
		}
		activeModels[id] = &activeModel{modelName: name, provider: prov, tier: tier, isActive: isActive}
		order = append(order, id)
	}
	add(defaultModelID, defaultModelID, provider, "default", true)
	add(economyModelID, economyModelID, provider, "economy", true)
	add(premiumModelID, premiumModelID, provider, "premium", true)
	for _, row := range rows {
		id := row.strOr("model_id", "")
		name, prov := id, "custom"
		if k := knowledgeMap[id]; k != nil {
			if s, isStr := k["model_name"].(string); isStr && s != "" {
				name = s
			}
			if s, isStr := k["provider"].(string); isStr && s != "" {
				prov = s
			}
		}
		add(id, name, prov, "custom", false)
	}
	for _, km := range aiKnownModels {
		add(km.ModelID, km.ModelName, km.Provider, "custom", false)
	}

	items := make([]map[string]any, 0, len(order))
	for _, id := range order {
		m := activeModels[id]
		var knowledge any
		if k := knowledgeMap[id]; k != nil {
			knowledge = k
		}
		items = append(items, aiModelJSON(map[string]any{
			"model_id": id, "model_name": m.modelName, "provider": m.provider,
			"tier": m.tier, "is_active": m.isActive, "knowledge": knowledge,
		}))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AIModelKnowledgeGet: GET /v1/platform/ai/models/{modelId}/knowledge
func (d *Deps) AIModelKnowledgeGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	modelID := strings.TrimSpace(aiDecodePath(chiParam(r, "modelId")))
	if modelID == "" {
		writeErr(w, d, errBadRequest("Model ID required"))
		return
	}
	knowledge := d.aiLoadKnowledge(r.Context(), modelID)
	if knowledge == nil {
		writeJSON(w, http.StatusOK, map[string]any{"knowledge": nil, "chunks": []any{}})
		return
	}
	chunks, err := d.queryRows(r.Context(),
		`SELECT id, section, chunk_index, content, priority, created_at
		 FROM ai_model_knowledge_chunks
		 WHERE knowledge_id = $1
		 ORDER BY priority ASC, created_at ASC`,
		knowledge["id"])
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, map[string]any{
			"id":          c.strOr("id", ""),
			"section":     c.strOr("section", ""),
			"chunk_index": c.intOr("chunk_index", 0),
			"content":     c.strOr("content", ""),
			"priority":    c.intOr("priority", 0),
			"created_at":  c.timePtr("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"knowledge": knowledge, "chunks": out})
}

// AIModelKnowledgePut: PUT /v1/platform/ai/models/{modelId}/knowledge
func (d *Deps) AIModelKnowledgePut(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	modelID := strings.TrimSpace(aiDecodePath(chiParam(r, "modelId")))
	if modelID == "" {
		writeErr(w, d, errBadRequest("Model ID required"))
		return
	}
	body, okBody := aiParsedObject(w, d, r)
	if !okBody {
		return
	}
	modelName, _, err := aiStringField(body, "model_name", 0, 150, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	provRaw, _, err := aiStringField(body, "provider", 0, 60, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	reasoning, err := aiEnumField(body, "reasoning_level", "sedang", "rendah", "sedang", "tinggi")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	summary, _, err := aiStringField(body, "summary", 0, 4000, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	characteristics, _, err := aiStringField(body, "characteristics", 0, 4000, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	suitableFor, _, err := aiStringField(body, "suitable_for", 0, 4000, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	unsuitableFor, _, err := aiStringField(body, "unsuitable_for", 0, 4000, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	guidelines, _, err := aiStringField(body, "guidelines", 0, 8000, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	limitations, _, err := aiStringField(body, "limitations", 0, 8000, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	editorial, _, err := aiStringField(body, "editorial_context", 0, 8000, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	status, err := aiEnumField(body, "status", "active", "active", "draft", "archived")
	if err != nil {
		writeErr(w, d, err)
		return
	}

	existing, err := d.queryRow(ctx, "SELECT id FROM ai_model_knowledge WHERE model_id = $1 LIMIT 1", modelID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	knowledgeID := uuid.NewString()
	if existing != nil {
		knowledgeID = existing.strOr("id", knowledgeID)
	}

	chunks := aiGenerateChunks(modelID, aiOrEmpty(modelName, modelID), reasoning, map[string]string{
		"summary": summary, "characteristics": characteristics, "suitable_for": suitableFor,
		"unsuitable_for": unsuitableFor, "guidelines": guidelines, "limitations": limitations,
		"editorial_context": editorial,
	})

	txErr := d.ptWithTx(ctx, func(tx *ptTx) error {
		if existing != nil {
			if err := tx.exec(
				`UPDATE ai_model_knowledge
				 SET model_name = $1, provider = $2, reasoning_level = $3, summary = $4,
				     characteristics = $5, suitable_for = $6, unsuitable_for = $7, guidelines = $8,
				     limitations = $9, editorial_context = $10, status = $11, updated_at = NOW()
				 WHERE id = $12`,
				nullIfEmpty(modelName), nullIfEmpty(provRaw), reasoning, nullIfEmpty(summary),
				nullIfEmpty(characteristics), nullIfEmpty(suitableFor), nullIfEmpty(unsuitableFor),
				nullIfEmpty(guidelines), nullIfEmpty(limitations), nullIfEmpty(editorial),
				status, knowledgeID); err != nil {
				return err
			}
		} else if err := tx.exec(
			`INSERT INTO ai_model_knowledge (
			   id, model_id, model_name, provider, reasoning_level, summary,
			   characteristics, suitable_for, unsuitable_for, guidelines,
			   limitations, editorial_context, status, created_at, updated_at
			 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW())`,
			knowledgeID, modelID, nullIfEmpty(modelName), nullIfEmpty(provRaw), reasoning,
			nullIfEmpty(summary), nullIfEmpty(characteristics), nullIfEmpty(suitableFor),
			nullIfEmpty(unsuitableFor), nullIfEmpty(guidelines), nullIfEmpty(limitations),
			nullIfEmpty(editorial), status); err != nil {
			return err
		}

		if err := tx.exec("DELETE FROM ai_model_knowledge_chunks WHERE knowledge_id = $1", knowledgeID); err != nil {
			return err
		}
		for _, c := range chunks {
			embedding, _ := json.Marshal(c.Embedding)
			if err := tx.exec(
				`INSERT INTO ai_model_knowledge_chunks (
				   id, knowledge_id, model_id, section, chunk_index, content, embedding, priority, created_at
				 ) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, NOW())`,
				uuid.NewString(), knowledgeID, modelID, c.Section, c.ChunkIndex,
				c.Content, string(embedding), c.Priority); err != nil {
				return err
			}
		}
		return tx.exec("UPDATE ai_model_knowledge SET chunk_count = $1, updated_at = NOW() WHERE id = $2",
			len(chunks), knowledgeID)
	})
	if txErr != nil {
		writeErr(w, d, txErr)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"knowledge":   d.aiLoadKnowledge(ctx, modelID),
		"chunk_count": len(chunks),
	})
}

// AIModelKnowledgeDelete: DELETE /v1/platform/ai/models/{modelId}/knowledge
func (d *Deps) AIModelKnowledgeDelete(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	modelID := strings.TrimSpace(aiDecodePath(chiParam(r, "modelId")))
	if modelID == "" {
		writeErr(w, d, errBadRequest("Model ID required"))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM ai_model_knowledge WHERE model_id = $1", modelID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Model knowledge deleted"})
}

// AIProbe: POST /v1/platform/ai/probe
func (d *Deps) AIProbe(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body, okBody := aiStrictObject(w, d, r)
	if !okBody {
		return
	}
	tier, err := aiEnumField(body, "model_tier", "default", "default", "economy", "premium")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	result, genErr := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt: "Reply with exactly: OK", Tier: tier, TaskType: "provider_probe",
		UserID: &user.ID,
	})
	if genErr != nil {
		writeErr(w, d, genErr)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// AIStatus: GET /v1/platform/ai/status
func (d *Deps) AIStatus(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	rt, err := ai.LoadAIRuntime(r.Context(), d.Pool)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"provider":        rt.Provider,
		"configured":      ai.IsAIConfigured(rt),
		"models":          map[string]any{"default": rt.DefaultModel, "economy": rt.EconomyModel, "premium": rt.PremiumModel, "embedding": rt.EmbeddingModel},
		"timeout_seconds": rt.TimeoutSeconds,
		"max_retries":     rt.MaxRetries,
		"circuit":         "closed",
	})
}

// AITestGeneration: POST /v1/platform/ai/test-generation
func (d *Deps) AITestGeneration(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body, okBody := aiStrictObject(w, d, r)
	if !okBody {
		return
	}
	tier, err := aiEnumField(body, "model_tier", "default", "default", "economy", "premium")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	prompt, _, err := aiStringField(body, "prompt", 1, 4000, false, false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	var brandID *string
	// parity z.string().uuid().nullable().optional(): null → null, string →
	// wajib UUID (termasuk string kosong yang ditolak).
	brandRaw, hasBrand, bErr := aiStringField(body, "brand_id", 0, 0, false, true)
	if bErr != nil {
		writeErr(w, d, bErr)
		return
	}
	if hasBrand {
		if !aiIsUUID(brandRaw) {
			writeErr(w, d, errUnprocessable("Invalid UUID"))
			return
		}
		brandID = &brandRaw
	}
	if brandID != nil {
		brandRow, qErr := d.queryRow(r.Context(), "SELECT id FROM brands WHERE id = $1 LIMIT 1", *brandID)
		if qErr != nil {
			writeErr(w, d, qErr)
			return
		}
		if brandRow == nil {
			writeErr(w, d, errNotFound("Brand not found"))
			return
		}
	}
	result, genErr := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt: prompt, Tier: tier, TaskType: "admin_test_generation",
		BrandID: brandID, UserID: &user.ID,
	})
	if genErr != nil {
		writeErr(w, d, genErr)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// AIUsage: GET /v1/platform/ai/usage
func (d *Deps) AIUsage(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	// parity `Number(url.searchParams.get("days") ?? 30)`: param ada tapi kosong
	// → Number("") = 0 → di-clamp ke 1 (bukan default 30).
	days := aiClampDays(q.Get("days"), q.Has("days"), 30)
	brandID := strings.TrimSpace(q.Get("brand_id"))
	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	filter := "created_at >= $1"
	params := []any{since}
	if brandID != "" {
		filter = "created_at >= $1 AND brand_id = $2"
		params = append(params, brandID)
	}
	summary, err := d.queryRow(ctx,
		`SELECT COUNT(*) AS calls,
		        COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) AS successful_calls,
		        COALESCE(SUM(total_tokens), 0) AS total_tokens,
		        COALESCE(SUM(input_tokens), 0) AS input_tokens,
		        COALESCE(SUM(output_tokens), 0) AS output_tokens
		 FROM model_usage_events WHERE `+filter, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	events, err := d.queryRows(ctx,
		`SELECT id, brand_id, task_type, provider, model, total_tokens, latency_ms, success, error_category, created_at
		 FROM model_usage_events WHERE `+filter+` ORDER BY created_at DESC LIMIT 100`, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]any{
			"id":             e.strOr("id", ""),
			"brand_id":       ptNullableStr(e, "brand_id"),
			"task_type":      ptNullableStr(e, "task_type"),
			"provider":       ptNullableStr(e, "provider"),
			"model":          ptNullableStr(e, "model"),
			"total_tokens":   e.intOr("total_tokens", 0),
			"latency_ms":     e.numPtr("latency_ms"),
			"success":        e.boolOr("success", false),
			"error_category": ptNullableStr(e, "error_category"),
			"created_at":     e.timePtr("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"calls":            summary.intOr("calls", 0),
			"successful_calls": summary.intOr("successful_calls", 0),
			"total_tokens":     summary.intOr("total_tokens", 0),
			"input_tokens":     summary.intOr("input_tokens", 0),
			"output_tokens":    summary.intOr("output_tokens", 0),
		},
		"events": out,
	})
}

// aiClampDays: parity Math.min(365, Math.max(1, Number(days ?? 30))).
// `present` membedakan param tidak ada (default) dari param kosong (0 → 1).
func aiClampDays(raw string, present bool, def int) int {
	if !present {
		return def
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 1
	}
	if n < 1 {
		return 1
	}
	if n > 365 {
		return 365
	}
	return int(n)
}

// aiDecodePath: parity decodeURIComponent pada param path.
func aiDecodePath(raw string) string {
	if decoded, err := url.PathUnescape(raw); err == nil {
		return decoded
	}
	return raw
}

// aiIsUUID: parity z.string().uuid() (bentuk UUID apa pun yang dikenali zod).
func aiIsUUID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, c := range v {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
