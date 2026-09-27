package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Riwayat run Taut (taut_agent_runs) + retry + cleanup. Port
// app/v1/platform/taut/runs/** dan lib/taut/orchestrator.ts bagian run.

// TautRunsList: GET /v1/platform/taut/runs
func (d *Deps) TautRunsList(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	campaigns, err := d.queryRows(r.Context(),
		`SELECT c.id AS campaign_id, c.name, c.brand_id, b.name AS brand_name,
		        c.taut_status, c.budget_strategy,
		        (SELECT COUNT(*) FROM taut_agent_runs r WHERE r.campaign_id = c.id) AS runs,
		        (SELECT COUNT(*) FROM taut_agent_runs r WHERE r.campaign_id = c.id AND r.status = 'failed') AS failed_runs,
		        c.updated_at
		 FROM campaigns c LEFT JOIN brands b ON b.id = c.brand_id
		 WHERE c.taut_status IS NOT NULL
		 ORDER BY c.updated_at DESC LIMIT 50`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	perAgent, err := d.queryRows(r.Context(),
		`SELECT agent,
		        COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'done') AS done,
		        COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		        COUNT(*) FILTER (WHERE status IN ('pending','running')) AS active,
		        COALESCE(SUM(token_input), 0) AS token_input,
		        COALESCE(SUM(token_output), 0) AS token_output
		 FROM taut_agent_runs GROUP BY agent ORDER BY agent`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	totals := map[string]int{
		"total": 0, "done": 0, "failed": 0, "active": 0, "token_input": 0, "token_output": 0,
	}
	perAgentOut := make([]map[string]any, 0, len(perAgent))
	for _, raw := range perAgent {
		a := ptRow(ptNormalizeMap(raw))
		item := map[string]any{
			"agent":        a.strOr("agent", ""),
			"total":        int(a.numOr("total")),
			"done":         int(a.numOr("done")),
			"failed":       int(a.numOr("failed")),
			"active":       int(a.numOr("active")),
			"token_input":  int(a.numOr("token_input")),
			"token_output": int(a.numOr("token_output")),
		}
		totals["total"] += item["total"].(int)
		totals["done"] += item["done"].(int)
		totals["failed"] += item["failed"].(int)
		totals["active"] += item["active"].(int)
		totals["token_input"] += item["token_input"].(int)
		totals["token_output"] += item["token_output"].(int)
		perAgentOut = append(perAgentOut, item)
	}
	campaignsOut := make([]map[string]any, 0, len(campaigns))
	for _, raw := range campaigns {
		c := ptRow(ptNormalizeMap(raw))
		campaignsOut = append(campaignsOut, map[string]any{
			"campaign_id":     c.strOr("campaign_id", ""),
			"name":            c.strOr("name", ""),
			"brand_id":        ptNullableStr(c, "brand_id"),
			"brand_name":      ptNullableStr(c, "brand_name"),
			"taut_status":     ptNullableStr(c, "taut_status"),
			"budget_strategy": ptNullableStr(c, "budget_strategy"),
			"runs":            int(c.numOr("runs")),
			"failed_runs":     int(c.numOr("failed_runs")),
			"updated_at":      c.anyOrNil("updated_at"),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"campaigns": campaignsOut,
		"stats": map[string]any{
			"per_agent": perAgentOut,
			"totals":    totals,
		},
	})
}

// TautRunsByCampaign: GET /v1/platform/taut/runs/{campaignId}
func (d *Deps) TautRunsByCampaign(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	runs, err := d.queryRows(r.Context(),
		`SELECT r.id, r.agent, r.status, r.model, r.token_input, r.token_output, r.duration_ms, r.error, r.created_at, r.finished_at,
		        COALESCE(m.name, '-') AS media_name
		 FROM taut_agent_runs r
		 LEFT JOIN media m ON m.id = r.media_id
		 WHERE r.campaign_id = $1
		 ORDER BY r.created_at DESC LIMIT 100`, chiParam(r, "campaignId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"runs": ptRows(runs)})
}

// TautRunsCleanup: POST /v1/platform/taut/runs/cleanup
func (d *Deps) TautRunsCleanup(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	body := decodeJSON(r)
	days := 30
	if n, hasNum := intField(body, "older_than_days"); hasNum {
		days = n
	}
	if days < 1 || days > 365 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	affected, err := d.execRows(r.Context(),
		"DELETE FROM taut_agent_runs WHERE created_at < NOW() - ($1 * INTERVAL '1 day')", days)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.runs_cleanup", "taut_agent_runs", "cleanup:"+ptItoa(days))
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": affected, "older_than_days": days})
}

// TautRunRetry: POST /v1/platform/taut/runs/retry/{runId}
func (d *Deps) TautRunRetry(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	runID := chiParam(r, "runId")
	runRow, err := d.queryRow(r.Context(),
		"SELECT id, campaign_id, media_id, agent, status, input FROM taut_agent_runs WHERE id = $1 LIMIT 1", runID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if runRow == nil {
		writeErr(w, d, errNotFound("Run tidak ditemukan."))
		return
	}
	if runRow.strOr("status", "") != "failed" {
		writeErr(w, d, errConflict("Hanya run berstatus failed yang bisa di-retry."))
		return
	}
	agent := runRow.strOr("agent", "")
	if !ptContains(tautAgentNames, agent) {
		writeErr(w, d, errBadRequest("Agent tidak dikenal."))
		return
	}
	prompt := extractRunPrompt(runRow.jsonField("input"))
	if prompt == "" {
		writeErr(w, d, errConflict("Input prompt run tidak tersedia untuk retry."))
		return
	}
	var mediaID *string
	if m, hasMedia := runRow.str("media_id"); hasMedia && m != "" {
		mediaID = &m
	}
	result, err := d.runTautAgent(r.Context(), agent, runRow.strOr("campaign_id", ""), prompt, mediaID, true)
	if err != nil {
		var notCfg *agentNotConfiguredError
		if errors.As(err, &notCfg) {
			writeErr(w, d, errConflict(notCfg.Error()))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.agent_run_retried", "taut_agent_runs", runID)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "run_id": result.RunID, "agent": agent, "model": result.Result.Model,
		"token_input": result.Result.Input, "token_output": result.Result.Output,
		"duration_ms": result.Result.DurationMs,
	})
}

// ---------- orchestrator (port lib/taut/orchestrator.ts bagian runTautAgent) ----------

type tautRunOutcome struct {
	Result tautLLMResult
	RunID  string
}

func extractRunPrompt(input any) string {
	if input == nil {
		return ""
	}
	m, ok := input.(map[string]any)
	if !ok {
		return ""
	}
	if s, isString := m["prompt"].(string); isString {
		return s
	}
	return ""
}

// tautInputHash: sha256(agent|campaign|media|prompt) — parity tautInputHash.
func tautInputHash(agent, campaignID, mediaID, prompt string) string {
	sum := sha256.Sum256([]byte(agent + "|" + campaignID + "|" + mediaID + "|" + prompt))
	return hex.EncodeToString(sum[:])
}

// runTautAgent: port runTautAgent — dedup run `done` dgn input_hash sama,
// retry maks 2 + backoff 500ms*attempt, simpan output + training sample.
func (d *Deps) runTautAgent(ctx context.Context, agent, campaignID, userPrompt string, mediaID *string, jsonMode bool) (*tautRunOutcome, error) {
	mediaKey := ""
	if mediaID != nil {
		mediaKey = *mediaID
	}
	inputHash := tautInputHash(agent, campaignID, mediaKey, userPrompt)

	cached, err := d.queryRow(ctx,
		"SELECT id, model, token_input, token_output, duration_ms, output FROM taut_agent_runs WHERE agent = $1 AND input_hash = $2 AND status = 'done' ORDER BY created_at DESC LIMIT 1",
		agent, inputHash)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		content := extractCachedContent(cached.jsonField("output"))
		if content != "" {
			return &tautRunOutcome{
				Result: tautLLMResult{
					Content: content, Model: cached.strOr("model", ""),
					Input: int(cached.numOr("token_input")), Output: int(cached.numOr("token_output")),
					DurationMs: int64(cached.numOr("duration_ms")),
				},
				RunID: cached.strOr("id", ""),
			}, nil
		}
	}

	const maxRetries = 2
	var lastErr error
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		runID := uuid.NewString()
		if err := d.exec(ctx,
			"INSERT INTO taut_agent_runs (id, campaign_id, media_id, agent, status, input, input_hash) VALUES ($1, $2, $3, $4, 'running', $5, $6)",
			runID, campaignID, mediaID, agent, ptJSONString(map[string]any{"prompt": ptTruncate(userPrompt, 8000)}), inputHash); err != nil {
			return nil, err
		}
		d.waitForTautRateSlot(ctx, agent)
		result, callErr := d.callAgentLLM(ctx, agent, userPrompt, jsonMode, "", 0)
		if callErr == nil {
			_ = d.exec(ctx,
				"UPDATE taut_agent_runs SET status='done', model=$1, token_input=$2, token_output=$3, duration_ms=$4, output=$5, finished_at=NOW() WHERE id=$6",
				result.Model, result.Input, result.Output, result.DurationMs,
				ptJSONString(map[string]any{"content": result.Content}), runID)
			d.saveTautTrainingSample(ctx, campaignID, mediaID, agent, "done", userPrompt, result.Content, result.Model, &result.Input, &result.Output)
			return &tautRunOutcome{Result: *result, RunID: runID}, nil
		}
		lastErr = callErr
		message := ptTruncate(callErr.Error(), 1000)
		_ = d.exec(ctx, "UPDATE taut_agent_runs SET status='failed', error=$1, finished_at=NOW() WHERE id=$2", message, runID)
		d.saveTautTrainingSample(ctx, campaignID, mediaID, agent, "failed", userPrompt, message, "", nil, nil)
		if attempt > maxRetries {
			break
		}
		select {
		case <-time.After(time.Duration(500*attempt) * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func extractCachedContent(output any) string {
	if output == nil {
		return ""
	}
	m, ok := output.(map[string]any)
	if !ok {
		return ""
	}
	if s, isString := m["content"].(string); isString {
		return s
	}
	return ""
}

// waitForTautRateSlot: spin-wait maks ~60 dtk (20 iterasi x 3 dtk) seperti app lama.
func (d *Deps) waitForTautRateSlot(ctx context.Context, agent string) {
	rateRow, _ := d.queryRow(ctx, "SELECT rate_limit_per_min FROM taut_agent_configs WHERE agent = $1 LIMIT 1", agent)
	limit := 3
	if rateRow != nil && rateRow.has("rate_limit_per_min") {
		limit = int(rateRow.numOr("rate_limit_per_min"))
	}
	if limit <= 0 {
		return
	}
	for i := 0; i < 20; i++ {
		countRow, err := d.queryRow(ctx,
			"SELECT COUNT(*) AS n FROM taut_agent_runs WHERE agent = $1 AND created_at > NOW() - INTERVAL '60 seconds'", agent)
		if err == nil && countRow != nil && int(countRow.numOr("n")) < limit {
			return
		}
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// saveTautTrainingSample: best-effort ke taut_training_samples.
func (d *Deps) saveTautTrainingSample(ctx context.Context, campaignID string, mediaID *string, agent, status, input, output, model string, tokenIn, tokenOut *int) {
	var modelArg any
	if model != "" {
		modelArg = model
	}
	_, _ = d.Pool.Exec(ctx,
		`INSERT INTO taut_training_samples (id, campaign_id, media_id, agent, status, input_text, output_text, model, token_input, token_output)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		uuid.NewString(), campaignID, mediaID, agent, status,
		ptTruncate(input, 8000), ptTruncate(output, 12000), modelArg, tokenIn, tokenOut)
}
