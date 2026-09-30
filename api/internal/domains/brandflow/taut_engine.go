package brandflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

// taut_engine.go: port lib/taut/agent-config.ts + lib/taut/orchestrator.ts +
// lib/taut/json.ts (bagian yang dipakai endpoint taut pelengkap). Disalin dari
// clienttaut (fungsi unexported tidak bisa diimpor lintas paket).

// ---------- agent config ----------

type agentConfig struct {
	Agent           string
	BaseURL         string
	APIKey          string
	Model           string
	Temperature     float64
	MaxTokens       *int
	TimeoutMS       int
	SystemPrompt    string
	Enabled         bool
	RateLimitPerMin int
	Configured      bool
}

type agentLLMResult struct {
	Content    string
	Model      string
	UsageIn    int
	UsageOut   int
	DurationMS int64
	RunID      string
}

type agentConfigCacheEntry struct {
	value   agentConfig
	expires time.Time
}

var (
	agentConfigMu    sync.Mutex
	agentConfigCache = map[string]agentConfigCacheEntry{}
)

const agentConfigTTL = 60 * time.Second

// agentNotConfiguredError: parity AgentNotConfiguredError (409).
type agentNotConfiguredError struct{ agent string }

func (e *agentNotConfiguredError) Error() string {
	return "Fitur AI \"" + e.agent + "\" belum dikonfigurasi oleh admin. Silakan hubungi admin platform untuk mengaktifkannya."
}

// getAgentConfig: baca config per-agent (cache 60s) + unseal api key +
// fallback ke AI gateway global (platform_ai_configuration).
func (d *Deps) getAgentConfig(ctx context.Context, agent string) agentConfig {
	now := time.Now()
	agentConfigMu.Lock()
	if c, ok := agentConfigCache[agent]; ok && c.expires.After(now) {
		agentConfigMu.Unlock()
		return c.value
	}
	agentConfigMu.Unlock()

	cfg := agentConfig{Agent: agent, Temperature: 0.7, TimeoutMS: 60000, RateLimitPerMin: 3}
	row, err := d.queryRow(ctx,
		"SELECT agent, base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms, system_prompt, enabled, rate_limit_per_min FROM taut_agent_configs WHERE agent = $1 LIMIT 1",
		agent)
	if err != nil {
		row = nil
	}
	if row != nil {
		apiKey := ""
		if sealed := row.strOr("sealed_api_key", ""); sealed != "" {
			if plain, uErr := d.unsealT2(sealed); uErr == nil {
				apiKey = plain
			}
		}
		enabled := row.boolOr("enabled", false)
		baseURL := strings.TrimRight(row.strOr("base_url", ""), "/")
		model := row.strOr("model", "")
		maxTokens := 0
		var maxTokensPtr *int
		if row.has("max_tokens") {
			maxTokens = row.intOr("max_tokens", 0)
			maxTokensPtr = &maxTokens
		}
		cfg = agentConfig{
			Agent:           agent,
			BaseURL:         baseURL,
			APIKey:          apiKey,
			Model:           model,
			Temperature:     row.numOr("temperature"),
			MaxTokens:       maxTokensPtr,
			TimeoutMS:       row.intOr("timeout_ms", 60000),
			SystemPrompt:    row.strOr("system_prompt", ""),
			Enabled:         enabled,
			RateLimitPerMin: row.intOr("rate_limit_per_min", 3),
			Configured:      enabled && baseURL != "" && apiKey != "" && model != "",
		}
		if !row.has("temperature") {
			cfg.Temperature = 0.7
		}
	}

	if !cfg.Configured {
		if rt, rtErr := ai.LoadAIRuntime(ctx, d.Pool); rtErr == nil && rt.BaseURL != "" && rt.APIKey != "" && rt.DefaultModel != "" {
			cfg.BaseURL = strings.TrimRight(rt.BaseURL, "/")
			cfg.APIKey = rt.APIKey
			cfg.Model = rt.DefaultModel
			cfg.Enabled = true
			cfg.Configured = true
		}
	}

	agentConfigMu.Lock()
	agentConfigCache[agent] = agentConfigCacheEntry{value: cfg, expires: now.Add(agentConfigTTL)}
	agentConfigMu.Unlock()
	return cfg
}

// callAgentLLM: port callAgentLLM — POST {baseUrl}/chat/completions.
func (d *Deps) callAgentLLM(ctx context.Context, agent, userPrompt string, jsonMode bool, systemOverride string, timeoutMS int) (*agentLLMResult, error) {
	cfg := d.getAgentConfig(ctx, agent)
	if !cfg.Configured {
		return nil, &agentNotConfiguredError{agent: agent}
	}
	timeout := cfg.TimeoutMS
	if timeoutMS > 0 {
		timeout = timeoutMS
	}
	if timeout <= 0 {
		timeout = 60000
	}
	system := cfg.SystemPrompt
	if systemOverride != "" {
		system = systemOverride
	}
	payload := map[string]any{
		"model":       cfg.Model,
		"temperature": cfg.Temperature,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": userPrompt},
		},
	}
	if cfg.MaxTokens != nil {
		payload["max_tokens"] = *cfg.MaxTokens
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("LLM %s: %w", agent, err)
	}
	startedAt := time.Now()
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("LLM %s: %w", agent, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM %s: %w", agent, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("LLM %s: HTTP %d", agent, resp.StatusCode)
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("LLM %s: respons kosong.", agent)
	}
	return &agentLLMResult{
		Content: content, Model: cfg.Model,
		UsageIn: parsed.Usage.PromptTokens, UsageOut: parsed.Usage.CompletionTokens,
		DurationMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

// ---------- orchestrator ----------

var tautStatusFlow = []string{
	"draft", "profiling", "profiled", "materials", "budget", "placement",
	"production", "review", "scheduled", "submitted",
}

// tautInputHash: sha256(`agent|campaignId|mediaId|prompt`) untuk dedup idempoten.
func tautInputHash(agent, campaignID, mediaID, prompt string) string {
	sum := sha256.Sum256([]byte(agent + "|" + campaignID + "|" + mediaID + "|" + prompt))
	return hex.EncodeToString(sum[:])
}

// advanceTautStatus: naikkan status MONOTON (hanya maju mengikuti flow).
func (d *Deps) advanceTautStatus(ctx context.Context, campaignID, status string) error {
	idx := -1
	for i, s := range tautStatusFlow {
		if s == status {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return d.exec(ctx, "UPDATE campaigns SET taut_status = $1, updated_at = NOW() WHERE id = $2", status, campaignID)
	}
	behind := tautStatusFlow[:idx]
	ph := make([]string, len(behind))
	args := []any{status, campaignID}
	for i, s := range behind {
		args = append(args, s)
		ph[i] = "$" + itoa(i+3)
	}
	return d.exec(ctx,
		"UPDATE campaigns SET taut_status = $1, updated_at = NOW() WHERE id = $2 AND (taut_status IS NULL OR taut_status IN ("+strings.Join(ph, ",")+"))",
		args...)
}

// waitForTautRateSlot: tunggu slot rate limit agent (spin-wait maks ~60 dtk).
func (d *Deps) waitForTautRateSlot(ctx context.Context, agent string) {
	limit := d.getAgentConfig(ctx, agent).RateLimitPerMin
	if limit <= 0 {
		return
	}
	for i := 0; i < 20; i++ {
		row, err := d.queryRow(ctx,
			"SELECT COUNT(*) AS n FROM taut_agent_runs WHERE agent = $1 AND created_at > NOW() - INTERVAL '60 seconds'", agent)
		if err == nil && row.intOr("n", 0) < limit {
			return
		}
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// runTautAgent: port runTautAgent — dedup run done, retry max 2 + backoff,
// catat taut_agent_runs + training sample.
func (d *Deps) runTautAgent(ctx context.Context, agent, campaignID, userPrompt string, mediaID *string, jsonMode bool) (*agentLLMResult, error) {
	mediaKey := ""
	if mediaID != nil {
		mediaKey = *mediaID
	}
	inputHash := tautInputHash(agent, campaignID, mediaKey, userPrompt)

	cached, err := d.queryRow(ctx,
		"SELECT id, model, token_input, token_output, duration_ms, output FROM taut_agent_runs WHERE agent = $1 AND input_hash = $2 AND status = 'done' ORDER BY created_at DESC LIMIT 1",
		agent, inputHash)
	if err == nil && cached != nil {
		if content := outputContent(cached.raw("output")); content != "" {
			return &agentLLMResult{
				Content: content, Model: cached.strOr("model", ""),
				UsageIn: cached.intOr("token_input", 0), UsageOut: cached.intOr("token_output", 0),
				DurationMS: int64(cached.intOr("duration_ms", 0)), RunID: cached.strOr("id", ""),
			}, nil
		}
	}

	const maxRetries = 2
	attempt := 0
	for {
		attempt++
		runID := uuid.NewString()
		if err := d.exec(ctx,
			"INSERT INTO taut_agent_runs (id, campaign_id, media_id, agent, status, input, input_hash) VALUES ($1, $2, $3, $4, 'running', $5::jsonb, $6)",
			runID, campaignID, mediaID, agent, jsonStringify(map[string]any{"prompt": truncateString(userPrompt, 8000)}), inputHash); err != nil {
			return nil, err
		}
		d.waitForTautRateSlot(ctx, agent)
		result, llmErr := d.callAgentLLM(ctx, agent, userPrompt, jsonMode, "", 0)
		if llmErr == nil {
			_ = d.exec(ctx,
				"UPDATE taut_agent_runs SET status='done', model=$1, token_input=$2, token_output=$3, duration_ms=$4, output=$5::jsonb, finished_at=NOW() WHERE id=$6",
				result.Model, result.UsageIn, result.UsageOut, result.DurationMS,
				jsonStringify(map[string]any{"content": result.Content}), runID)
			d.saveTautTrainingSample(ctx, campaignID, mediaID, agent, "done", userPrompt, result.Content, result.Model, result.UsageIn, result.UsageOut)
			result.RunID = runID
			return result, nil
		}
		message := llmErr.Error()
		_ = d.exec(ctx, "UPDATE taut_agent_runs SET status='failed', error=$1, finished_at=NOW() WHERE id=$2",
			truncateString(message, 1000), runID)
		d.saveTautTrainingSample(ctx, campaignID, mediaID, agent, "failed", userPrompt, message, "", 0, 0)
		if attempt > maxRetries {
			return nil, llmErr
		}
		select {
		case <-time.After(time.Duration(500*attempt) * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// saveTautTrainingSample: simpan pasangan input→output agent (best-effort).
func (d *Deps) saveTautTrainingSample(ctx context.Context, campaignID string, mediaID *string, agent, status, input, output, model string, tokenIn, tokenOut int) {
	var modelArg any
	if model != "" {
		modelArg = model
	}
	var tokenInArg, tokenOutArg any
	if tokenIn > 0 {
		tokenInArg = tokenIn
	}
	if tokenOut > 0 {
		tokenOutArg = tokenOut
	}
	if err := d.exec(ctx,
		`INSERT INTO taut_training_samples (id, campaign_id, media_id, agent, status, input_text, output_text, model, token_input, token_output)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		uuid.NewString(), campaignID, mediaID, agent, status,
		truncateString(input, 8000), truncateString(output, 12000), modelArg, tokenInArg, tokenOutArg); err != nil {
		slog.Debug("[brandflow] training sample gagal", "err", err)
	}
}

// outputContent: ekstrak .content dari kolom output jsonb (map atau string).
func outputContent(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if s, ok := t["content"].(string); ok {
			return s
		}
	case string:
		var parsed map[string]any
		if json.Unmarshal([]byte(t), &parsed) == nil {
			if s, ok := parsed["content"].(string); ok {
				return s
			}
		}
	}
	return ""
}

// extractJSONObject: objek JSON pertama dalam teks (port extractJsonObject).
func extractJSONObject(raw string) map[string]any {
	m := reJSONBlock.FindString(raw)
	if m == "" {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(m), &parsed); err != nil {
		return nil
	}
	return parsed
}

// ---------- campaign helpers ----------

// loadTautCampaignRow: kampanye milik brand (untuk agen Taut).
func (d *Deps) loadTautCampaignRow(ctx context.Context, campaignID, brandID string) (bsRow, error) {
	row, err := d.queryRow(ctx,
		`SELECT c.*, b.name AS brand_name, b.client_name, b.industry, b.primary_language
		 FROM campaigns c JOIN brands b ON b.id = c.brand_id
		 WHERE c.id = $1 AND c.brand_id = $2 LIMIT 1`, campaignID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Kampanye tidak ditemukan")
	}
	return row, nil
}

// mediaNameFor: nama media (fallback id).
func (d *Deps) mediaNameFor(ctx context.Context, mediaID string) string {
	row, err := d.queryRow(ctx, "SELECT name FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil || row == nil {
		return mediaID
	}
	return row.strOr("name", mediaID)
}

// wordCountRangeFor: port resolveWordRange (lib/media/word-count.ts).
func wordCountRangeFor(mode string) (int, int, bool) {
	switch strings.TrimSpace(mode) {
	case "kata_300_700":
		return 300, 700, true
	case "kata_800_1500":
		return 800, 1500, true
	}
	return 0, 0, false
}

// buildWordRule: port buildWordRule.
func buildWordRule(mode string, mediaType string) string {
	if min, max, ok := wordCountRangeFor(mode); ok {
		return fmt.Sprintf("Aturan panjang artikel (WAJIB): tulis artikel %d-%d kata. Jangan kurang dari %d kata dan jangan lebih dari %d kata.", min, max, min, max)
	}
	if mediaType == "blogger" {
		return "Aturan panjang artikel (WAJIB): media BLOGGER minimal 500 kata dan maksimal 5000 kata."
	}
	return "Aturan panjang artikel (WAJIB): media MAINSTREAM minimal 300 kata."
}

// unsealAgentSecret: parity unsealSecretValue untuk api key agent.
func (d *Deps) unsealAgentSecret(sealed string) (string, error) {
	return d.unsealT2(sealed)
}
