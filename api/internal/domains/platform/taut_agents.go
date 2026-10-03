package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// Taut agents: konfigurasi per-agent (taut_agent_configs) + pemanggilan LLM
// OpenAI-compatible, port lib/taut/agent-config.ts. Fallback ke AI gateway
// global (platform_ai_configuration) bila agent belum dikonfigurasi sendiri.

type tautAgentConfig struct {
	Agent           string
	BaseURL         string
	APIKey          string
	Model           string
	Temperature     float64
	MaxTokens       *int
	TimeoutMs       int
	SystemPrompt    string
	Enabled         bool
	RateLimitPerMin int
	Configured      bool
}

type tautLLMResult struct {
	Content    string
	Model      string
	Input      int
	Output     int
	DurationMs int64
}

// agentNotConfiguredError: parity AgentNotConfiguredError (pesan sama persis).
type agentNotConfiguredError struct{ agent string }

func (e *agentNotConfiguredError) Error() string {
	return fmt.Sprintf("Fitur AI %q belum dikonfigurasi oleh admin. Silakan hubungi admin platform untuk mengaktifkannya.", e.agent)
}

var tautAgentCache = newTTLCache(60 * time.Second)

// invalidateTautAgentCache: parity invalidateAgentConfigCache.
func (d *Deps) invalidateTautAgentCache(agent string) {
	tautAgentCache.invalidate(agent)
}

// getTautAgentConfig: baca config agent + unseal api key (cache 60s) +
// fallback ke AI gateway global.
func (d *Deps) getTautAgentConfig(ctx context.Context, agent string) tautAgentConfig {
	if cached, ok := tautAgentCache.get(agent); ok {
		if cfg, isCfg := cached.(*tautAgentConfig); isCfg {
			return *cfg
		}
	}
	cfg := tautAgentConfig{Agent: agent, Temperature: 0.7, TimeoutMs: 60000, RateLimitPerMin: 3}
	row, err := d.queryRow(ctx,
		"SELECT agent, base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms, system_prompt, enabled, rate_limit_per_min FROM taut_agent_configs WHERE agent = $1 LIMIT 1",
		agent)
	if err != nil {
		row = nil
	}
	if row != nil {
		apiKey := ""
		var unsealErr error
		if sealed, okStr := row.str("sealed_api_key"); okStr && sealed != "" {
			if plain, uErr := d.unsealT2(sealed); uErr == nil {
				apiKey = plain
			} else {
				unsealErr = uErr
			}
		}
		enabled := row.boolOr("enabled", false)
		baseURL := strings.TrimRight(row.strOr("base_url", ""), "/")
		model := row.strOr("model", "")
		complete := enabled && baseURL != "" && apiKey != "" && model != ""
		cfg = tautAgentConfig{
			Agent:           agent,
			BaseURL:         baseURL,
			APIKey:          apiKey,
			Model:           model,
			Temperature:     ptRowTemperature(row),
			MaxTokens:       ptRowIntPtr(row, "max_tokens"),
			TimeoutMs:       ptRowIntDefault(row, "timeout_ms", 60000),
			SystemPrompt:    row.strOr("system_prompt", ""),
			Enabled:         enabled,
			RateLimitPerMin: ptMaxInt(ptRowIntDefault(row, "rate_limit_per_min", 3), 0),
			Configured:      complete,
		}
		// Key per-agent ada tapi tak terbaca: tetap hormati model pilihan admin
		// dengan meminjam key gateway global — hanya untuk base URL yang sama,
		// karena key milik provider lain tidak akan valid di sini.
		if unsealErr != nil && enabled && baseURL != "" && model != "" {
			if rt, rtErr := ai.LoadAIRuntime(ctx, d.Pool); rtErr == nil && rt.APIKey != "" && strings.TrimRight(rt.BaseURL, "/") == baseURL {
				cfg.APIKey = rt.APIKey
				cfg.Configured = true
				slog.Warn("[taut-agent] api key per-agent gagal di-unseal, memakai key gateway global dengan model config",
					"agent", agent, "model", model, "unseal_err", unsealErr.Error())
			}
		}
		if enabled && !cfg.Configured {
			slog.Warn("[taut-agent] config agent tidak lengkap, memakai gateway global",
				"agent", agent, "base_url", baseURL, "model", model,
				"has_key", apiKey != "", "unseal_err", unsealErr)
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
	snapshot := cfg
	tautAgentCache.set(agent, &snapshot)
	return cfg
}

func ptRowTemperature(row ptRow) float64 {
	if !row.has("temperature") {
		return 0.7
	}
	return row.numOr("temperature")
}

func ptRowIntPtr(row ptRow, key string) *int {
	if !row.has(key) {
		return nil
	}
	v := int(row.numOr(key))
	return &v
}

func ptRowIntDefault(row ptRow, key string, fallback int) int {
	if !row.has(key) {
		return fallback
	}
	return int(row.numOr(key))
}

func (d *Deps) unsealT2(sealed string) (string, error) {
	// Data lama ter-seal dengan APP_SECRET (provider.ts legacy), runtime baru
	// memakai CMS_ENCRYPTION_SECRET — coba keduanya.
	return crypto.UnsealCompat(sealed, d.Cfg.CmsEncryptionSecret, d.Cfg.Secret)
}

// sealT2: v2 (t2:) dengan cmsEncryptionSecret / fallback APP_SECRET.
func (d *Deps) sealT2(plain string) (string, error) {
	secret := d.Cfg.CmsEncryptionSecret
	if secret == "" {
		secret = d.Cfg.Secret
	}
	return crypto.Seal(plain, secret)
}

// callAgentLLM: port callAgentLLM — POST {baseUrl}/chat/completions.
func (d *Deps) callAgentLLM(ctx context.Context, agent, userPrompt string, jsonMode bool, systemOverride string, timeoutMs int) (*tautLLMResult, error) {
	cfg := d.getTautAgentConfig(ctx, agent)
	if !cfg.Configured {
		return nil, &agentNotConfiguredError{agent: agent}
	}
	timeout := cfg.TimeoutMs
	if timeoutMs > 0 {
		timeout = timeoutMs
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
	bodyBytes, _ := json.Marshal(payload)

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("LLM %s: %w", agent, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	started := time.Now()
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM %s: %w", agent, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

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
		Error any `json:"error"`
	}
	_ = json.Unmarshal(raw, &parsed)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if parsed.Error != nil {
			if b, mErr := json.Marshal(parsed.Error); mErr == nil {
				detail = ptTruncate(string(b), 300)
			}
		}
		return nil, fmt.Errorf("LLM %s: %s", agent, detail)
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("LLM %s: respons kosong.", agent)
	}
	return &tautLLMResult{
		Content: content, Model: cfg.Model,
		Input: parsed.Usage.PromptTokens, Output: parsed.Usage.CompletionTokens,
		DurationMs: time.Since(started).Milliseconds(),
	}, nil
}

// testTautAgentConnection: port testAgentConnection.
func (d *Deps) testTautAgentConnection(ctx context.Context, agent string) map[string]any {
	cfg := d.getTautAgentConfig(ctx, agent)
	if !cfg.Configured {
		return map[string]any{"ok": false, "message": "Belum dikonfigurasi (base_url / api key / model kosong atau disabled)."}
	}
	started := time.Now()
	result, err := d.callAgentLLM(ctx, agent, `Balas persis: {"ok":true}`, true, "", 0)
	if err != nil {
		return map[string]any{"ok": false, "message": ptTruncate(err.Error(), 300)}
	}
	return map[string]any{
		"ok": true, "message": "Koneksi OK (" + result.Model + ").",
		"latencyMs": time.Since(started).Milliseconds(),
	}
}

// extractJSONObject: port lib/taut/json.ts.
func extractJSONObject(raw string) map[string]any {
	asObject := func(v any) map[string]any {
		if obj, ok := v.(map[string]any); ok {
			return obj
		}
		return nil
	}
	trimmed := strings.TrimSpace(raw)
	var direct any
	if err := json.Unmarshal([]byte(trimmed), &direct); err == nil {
		if obj := asObject(direct); obj != nil {
			return obj
		}
	}
	fenced := strings.TrimSpace(reTautFence.ReplaceAllString(trimmed, ""))
	var parsedFenced any
	if err := json.Unmarshal([]byte(fenced), &parsedFenced); err == nil {
		if obj := asObject(parsedFenced); obj != nil {
			return obj
		}
	}
	start := strings.Index(fenced, "{")
	end := strings.LastIndex(fenced, "}")
	if start != -1 && end > start {
		var sliced any
		if err := json.Unmarshal([]byte(fenced[start:end+1]), &sliced); err == nil {
			if obj := asObject(sliced); obj != nil {
				return obj
			}
		}
	}
	return nil
}

// ---------- endpoint agent ----------

// TautAgentsList: GET /v1/platform/taut-agents
func (d *Deps) TautAgentsList(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT agent, base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms, system_prompt, enabled, rate_limit_per_min, updated_at
		 FROM taut_agent_configs ORDER BY
		   CASE agent WHEN 'profiler' THEN 1 WHEN 'matchmaker' THEN 2 WHEN 'writer' THEN 3 WHEN 'gatekeeper' THEN 4 ELSE 5 END`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	agents := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		agents = append(agents, map[string]any{
			"agent":              row.strOr("agent", ""),
			"base_url":           ptNullableStr(row, "base_url"),
			"model":              ptNullableStr(row, "model"),
			"temperature":        row.numPtr("temperature"),
			"max_tokens":         row.numPtr("max_tokens"),
			"timeout_ms":         row.numPtr("timeout_ms"),
			"system_prompt":      ptNullableStr(row, "system_prompt"),
			"api_key_configured": row.has("sealed_api_key"),
			"enabled":            row.boolOr("enabled", false),
			"rate_limit_per_min": ptRowRateLimit(row),
			"updated_at":         row.anyOrNil("updated_at"),
			"configured": row.boolOr("enabled", false) && row.has("base_url") &&
				row.has("sealed_api_key") && row.has("model"),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

func ptRowRateLimit(row ptRow) int {
	if !row.has("rate_limit_per_min") {
		return 3
	}
	return ptMaxInt(int(row.numOr("rate_limit_per_min")), 0)
}

// TautAgentGet: GET /v1/platform/taut-agents/{agent}
func (d *Deps) TautAgentGet(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	agent := chiParam(r, "agent")
	if !ptContains(tautAgentNames, agent) {
		writeErr(w, d, errNotFound("Agent tidak dikenal."))
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT agent, base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms, system_prompt, enabled, rate_limit_per_min, settings, updated_at FROM taut_agent_configs WHERE agent = $1 LIMIT 1",
		agent)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if row == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"agent": agent, "base_url": nil, "model": nil, "temperature": 0.7,
			"max_tokens": nil, "timeout_ms": 60000, "system_prompt": nil,
			"api_key_configured": false, "enabled": false, "rate_limit_per_min": 3,
			"settings": nil, "updated_at": nil, "configured": false,
		})
		return
	}
	writeJSON(w, http.StatusOK, tautAgentConfigJSON(ptRow(ptNormalizeMap(row))))
}

func tautAgentConfigJSON(row ptRow) map[string]any {
	configured := row.boolOr("enabled", false) && row.has("base_url") && row.has("sealed_api_key") && row.has("model")
	return map[string]any{
		"agent":              row.strOr("agent", ""),
		"base_url":           ptNullableStr(row, "base_url"),
		"model":              ptNullableStr(row, "model"),
		"temperature":        row.numPtr("temperature"),
		"max_tokens":         row.numPtr("max_tokens"),
		"timeout_ms":         row.numPtr("timeout_ms"),
		"system_prompt":      ptNullableStr(row, "system_prompt"),
		"api_key_configured": row.has("sealed_api_key"),
		"enabled":            row.boolOr("enabled", false),
		"rate_limit_per_min": ptRowRateLimit(row),
		"settings":           row.jsonField("settings"),
		"updated_at":         row.anyOrNil("updated_at"),
		"configured":         configured,
	}
}

// TautAgentUpdate: PUT /v1/platform/taut-agents/{agent}
func (d *Deps) TautAgentUpdate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	agent := chiParam(r, "agent")
	if !ptContains(tautAgentNames, agent) {
		writeErr(w, d, errNotFound("Agent tidak dikenal."))
		return
	}
	body := decodeJSON(r)
	baseURL, hasBaseURL := strField(body, "base_url")
	apiKey, hasAPIKey := strField(body, "api_key")
	model, hasModel := strField(body, "model")
	temperature, hasTemperature := numField(body, "temperature")
	maxTokens, hasMaxTokens := intField(body, "max_tokens")
	timeoutMs, hasTimeout := intField(body, "timeout_ms")
	systemPrompt, hasSystemPrompt := strField(body, "system_prompt")
	enabled, hasEnabled := boolField(body, "enabled")
	rateLimit, hasRateLimit := intField(body, "rate_limit_per_min")
	settings, hasSettings := rawField(body, "settings")

	if hasBaseURL {
		if !ptIsHTTPURL(baseURL) {
			writeErr(w, d, errUnprocessable("Invalid url"))
			return
		}
		if len(baseURL) > 500 {
			writeErr(w, d, errUnprocessable(ptZStringTooBig(500)))
			return
		}
	}
	if hasAPIKey {
		if apiKey == "" {
			writeErr(w, d, errUnprocessable(ptZStringTooSmall(1)))
			return
		}
		if len(apiKey) > 500 {
			writeErr(w, d, errUnprocessable(ptZStringTooBig(500)))
			return
		}
	}
	if hasModel && len(model) > 120 {
		writeErr(w, d, errUnprocessable(ptZStringTooBig(120)))
		return
	}
	if hasTemperature {
		if temperature < 0 {
			writeErr(w, d, errUnprocessable(ptZNumberTooSmall(0)))
			return
		}
		if temperature > 2 {
			writeErr(w, d, errUnprocessable(ptZNumberTooBig(2)))
			return
		}
	}
	if hasMaxTokens {
		if maxTokens <= 0 {
			writeErr(w, d, errUnprocessable("Number must be greater than 0"))
			return
		}
		if maxTokens > 200000 {
			writeErr(w, d, errUnprocessable(ptZNumberTooBig(200000)))
			return
		}
	}
	if hasTimeout {
		if timeoutMs < 5000 {
			writeErr(w, d, errUnprocessable("Timeout minimal 5000 ms."))
			return
		}
		if timeoutMs > 300000 {
			writeErr(w, d, errUnprocessable("Timeout maksimal 300000 ms (5 menit)."))
			return
		}
	}
	if hasSystemPrompt && len(systemPrompt) > 20000 {
		writeErr(w, d, errUnprocessable("System prompt maksimal 20.000 karakter."))
		return
	}
	if hasRateLimit {
		if rateLimit < 0 {
			writeErr(w, d, errUnprocessable("Rate limit tidak boleh negatif."))
			return
		}
		if rateLimit > 600 {
			writeErr(w, d, errUnprocessable("Rate limit maksimal 600 permintaan per menit. Isikan 0 untuk tanpa batas."))
			return
		}
	}

	existing, err := d.queryRow(r.Context(),
		"SELECT agent, base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms, system_prompt, enabled, rate_limit_per_min, settings, updated_at FROM taut_agent_configs WHERE agent = $1 LIMIT 1",
		agent)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	existingRow := ptRow{}
	if existing != nil {
		existingRow = ptRow(ptNormalizeMap(existing))
	}
	effectiveBase := ptPickStr(baseURL, hasBaseURL, existingRow.strOr("base_url", ""))
	var sealedKey *string
	if hasAPIKey && apiKey != "" {
		sealed, sErr := d.sealT2(apiKey)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealedKey = &sealed
	}
	effectiveModel := ptPickStr(model, hasModel, existingRow.strOr("model", ""))
	effectiveSystem := ptPickStr(systemPrompt, hasSystemPrompt, existingRow.strOr("system_prompt", ""))
	effectiveEnabled := enabled
	if !hasEnabled {
		effectiveEnabled = existingRow.boolOr("enabled", false)
	}
	if effectiveEnabled && (effectiveBase == "" || effectiveModel == "" || (sealedKey == nil && !existingRow.has("sealed_api_key"))) {
		writeErr(w, d, errUnprocessable("Tidak bisa mengaktifkan agent: base_url, api key, dan model wajib diisi."))
		return
	}
	var settingsArg any
	if hasSettings {
		settingsArg = settings
	} else if existing != nil {
		settingsArg = existingRow.jsonField("settings")
	} else {
		settingsArg = nil
	}

	maxTokensArg := ptNullableAnyInt(maxTokens, hasMaxTokens)
	if !hasMaxTokens && existingRow.has("max_tokens") {
		maxTokensArg = int(existingRow.numOr("max_tokens"))
	}
	timeoutArg := 60000
	if existingRow.has("timeout_ms") {
		timeoutArg = int(existingRow.numOr("timeout_ms"))
	}
	if hasTimeout {
		timeoutArg = timeoutMs
	}
	temperatureArg := 0.7
	if existingRow.has("temperature") {
		temperatureArg = existingRow.numOr("temperature")
	}
	if hasTemperature {
		temperatureArg = temperature
	}
	rateLimitArg := ptNullableAnyInt(rateLimit, hasRateLimit)

	if err := d.exec(r.Context(),
		`INSERT INTO taut_agent_configs (agent, base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms, system_prompt, enabled, rate_limit_per_min, settings, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		 ON CONFLICT (agent) DO UPDATE SET
		   base_url = EXCLUDED.base_url,
		   sealed_api_key = COALESCE(EXCLUDED.sealed_api_key, taut_agent_configs.sealed_api_key),
		   model = EXCLUDED.model,
		   temperature = EXCLUDED.temperature,
		   max_tokens = EXCLUDED.max_tokens,
		   timeout_ms = EXCLUDED.timeout_ms,
		   system_prompt = EXCLUDED.system_prompt,
		   enabled = EXCLUDED.enabled,
		   rate_limit_per_min = COALESCE(EXCLUDED.rate_limit_per_min, taut_agent_configs.rate_limit_per_min, 3),
		   settings = EXCLUDED.settings,
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		agent, nullIfEmpty(effectiveBase), sealedKey, nullIfEmpty(effectiveModel),
		temperatureArg, maxTokensArg, timeoutArg, nullIfEmpty(effectiveSystem),
		effectiveEnabled, rateLimitArg, ptJSONStringArg(settingsArg), user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	auditAction := "taut.agent_config_updated"
	if agent == "gatekeeper" && hasSettings {
		auditAction = "taut.gatekeeper_settings_updated"
	}
	d.audit(r.Context(), user.ID, auditAction, "taut_agent_config", agent)
	d.invalidateTautAgentCache(agent)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "agent": agent})
}

// TautAgentTest: POST /v1/platform/taut-agents/{agent}/test
func (d *Deps) TautAgentTest(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	agent := chiParam(r, "agent")
	if !ptContains(tautAgentNames, agent) {
		writeErr(w, d, errNotFound("Agent tidak dikenal."))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, d.testTautAgentConnection(r.Context(), agent))
}

// TautAgentLogs: GET /v1/platform/taut-agent-logs
func (d *Deps) TautAgentLogs(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	q := r.URL.Query()
	agent := strings.TrimSpace(q.Get("agent"))
	status := strings.TrimSpace(q.Get("status"))
	limit := clampInt(q.Get("limit"), 100, 1, 500)

	conds := []string{}
	params := []any{}
	if agent != "" && ptContains(tautAgentNames, agent) {
		params = append(params, agent)
		conds = append(conds, "agent = $"+ptItoa(len(params)))
	}
	if status != "" && ptContains([]string{"done", "failed", "running"}, status) {
		params = append(params, status)
		conds = append(conds, "status = $"+ptItoa(len(params)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	limitIdx := len(params) + 1

	runs, err := d.queryRows(r.Context(),
		`SELECT r.id, r.agent, r.status, r.error, r.model, r.token_input, r.token_output, r.duration_ms,
		        r.created_at, r.finished_at, c.name AS campaign_name, m.name AS media_name
		 FROM taut_agent_runs r
		 LEFT JOIN campaigns c ON c.id = r.campaign_id
		 LEFT JOIN media m ON m.id = r.media_id
		 `+where+`
		 ORDER BY r.created_at DESC
		 LIMIT $`+ptItoa(limitIdx), append(params, limit)...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	trainingLimit := limit
	if trainingLimit > 200 {
		trainingLimit = 200
	}
	training, err := d.queryRows(r.Context(),
		`SELECT id, agent, status, model, token_input, token_output, created_at,
		        LEFT(input_text, 600) AS input_preview, LEFT(output_text, 600) AS output_preview
		 FROM taut_training_samples
		 `+where+`
		 ORDER BY created_at DESC
		 LIMIT $`+ptItoa(limitIdx), append(params, trainingLimit)...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	summary, err := d.queryRows(r.Context(),
		`SELECT agent, status, COUNT(*)::int AS n
		 FROM taut_agent_runs
		 WHERE created_at > NOW() - INTERVAL '7 days'
		 GROUP BY agent, status`)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	runsOut := make([]map[string]any, 0, len(runs))
	for _, raw := range runs {
		x := ptRow(ptNormalizeMap(raw))
		runsOut = append(runsOut, map[string]any{
			"id":            x.strOr("id", ""),
			"agent":         x.strOr("agent", ""),
			"status":        x.strOr("status", ""),
			"error":         ptNullableStr(x, "error"),
			"model":         ptNullableStr(x, "model"),
			"token_input":   x.numPtr("token_input"),
			"token_output":  x.numPtr("token_output"),
			"duration_ms":   x.numPtr("duration_ms"),
			"campaign_name": ptNullableStr(x, "campaign_name"),
			"media_name":    ptNullableStr(x, "media_name"),
			"created_at":    x.anyOrNil("created_at"),
			"finished_at":   x.anyOrNil("finished_at"),
		})
	}
	trainingOut := make([]map[string]any, 0, len(training))
	for _, raw := range training {
		t := ptRow(ptNormalizeMap(raw))
		trainingOut = append(trainingOut, map[string]any{
			"id":             t.strOr("id", ""),
			"agent":          t.strOr("agent", ""),
			"status":         t.strOr("status", ""),
			"model":          ptNullableStr(t, "model"),
			"token_input":    t.numPtr("token_input"),
			"token_output":   t.numPtr("token_output"),
			"created_at":     t.anyOrNil("created_at"),
			"input_preview":  ptNullableStr(t, "input_preview"),
			"output_preview": ptNullableStr(t, "output_preview"),
		})
	}
	summaryOut := make([]map[string]any, 0, len(summary))
	for _, raw := range summary {
		s := ptRow(ptNormalizeMap(raw))
		summaryOut = append(summaryOut, map[string]any{
			"agent": s.strOr("agent", ""), "status": s.strOr("status", ""), "count": int(s.numOr("n")),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"runs": runsOut, "training": trainingOut, "summary_7d": summaryOut,
	})
}

func ptPickStr(value string, has bool, fallback string) string {
	if has {
		return value
	}
	return fallback
}

// ptIsHTTPURL: parity z.string().url() untuk base_url agent.
func ptIsHTTPURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return false
	}
	rest := trimmed[strings.Index(trimmed, "://")+3:]
	return rest != "" && !strings.HasPrefix(rest, "/")
}
