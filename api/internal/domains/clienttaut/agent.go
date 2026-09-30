package clienttaut

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
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// Port lib/taut/agent-config.ts + lib/taut/orchestrator.ts + lib/taut/json.ts.

// ---------- tipe ----------

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

// ---------- agent config ----------

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
			if plain, uErr := d.unsealSecret(sealed); uErr == nil {
				apiKey = plain
			}
		}
		enabled := row.boolOr("enabled", false)
		baseURL := strings.TrimRight(row.strOr("base_url", ""), "/")
		model := row.strOr("model", "")
		cfg = agentConfig{
			Agent:           agent,
			BaseURL:         baseURL,
			APIKey:          apiKey,
			Model:           model,
			Temperature:     rowTemperature(row),
			MaxTokens:       optionalInt(row, "max_tokens"),
			TimeoutMS:       intOrDefault(row, "timeout_ms", 60000),
			SystemPrompt:    row.strOr("system_prompt", ""),
			Enabled:         enabled,
			RateLimitPerMin: maxInt(intOrDefault(row, "rate_limit_per_min", 3), 0),
			Configured:      enabled && baseURL != "" && apiKey != "" && model != "",
		}
	}

	// Fallback AI gateway global (/admin/ai).
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

func invalidateAgentConfigCache(agent string) {
	agentConfigMu.Lock()
	if agent == "" {
		agentConfigCache = map[string]agentConfigCacheEntry{}
	} else {
		delete(agentConfigCache, agent)
	}
	agentConfigMu.Unlock()
}

func rowTemperature(row rowMap) float64 {
	if !row.has("temperature") {
		return 0.7
	}
	return row.numOr("temperature")
}

func optionalInt(row rowMap, key string) *int {
	if !row.has(key) {
		return nil
	}
	v := int(row.numOr(key))
	return &v
}

func intOrDefault(row rowMap, key string, fallback int) int {
	if !row.has(key) {
		return fallback
	}
	return int(row.numOr(key))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// unsealSecret: parity unsealSecretValue — cmsEncryptionSecret / fallback APP_SECRET.
func (d *Deps) unsealSecret(sealed string) (string, error) {
	secret := ""
	if d.Cfg != nil {
		secret = d.Cfg.CmsEncryptionSecret
		if secret == "" {
			secret = d.Cfg.Secret
		}
	}
	return crypto.Unseal(sealed, secret)
}

// callAgentLLM: port callAgentLLM — POST {baseUrl}/chat/completions.
// jsonMode → response_format json_object; timeoutMs > 0 menimpa config.
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
		Error any `json:"error"`
	}
	_ = json.Unmarshal(raw, &parsed)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := "HTTP " + itoa(resp.StatusCode)
		if parsed.Error != nil {
			if b, mErr := json.Marshal(parsed.Error); mErr == nil {
				detail = truncateString(string(b), 300)
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
	return &agentLLMResult{
		Content:    content,
		Model:      cfg.Model,
		UsageIn:    parsed.Usage.PromptTokens,
		UsageOut:   parsed.Usage.CompletionTokens,
		DurationMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------- orchestrator (port lib/taut/orchestrator.ts) ----------

var tautStatusFlow = []string{"draft", "profiling", "profiled", "materials", "budget", "placement", "production", "review", "scheduled", "submitted"}

// tautInputHash: sha256(`agent|campaignId|mediaId|prompt`) untuk dedup idempoten.
func tautInputHash(agent, campaignID, mediaID, prompt string) string {
	sum := sha256.Sum256([]byte(agent + "|" + campaignID + "|" + mediaID + "|" + prompt))
	return hex.EncodeToString(sum[:])
}

// profilerInputHash: sha256(JSON.stringify({businessProfile, categoryInput,
// materials, hyperlinkConfig})) — urutan field dipertahankan seperti app lama.
func profilerInputHash(businessProfile, categoryInput, materials, hyperlinkConfig any) string {
	payload := struct {
		BusinessProfile any `json:"businessProfile"`
		CategoryInput   any `json:"categoryInput"`
		Materials       any `json:"materials"`
		HyperlinkConfig any `json:"hyperlinkConfig"`
	}{businessProfile, categoryInput, materials, hyperlinkConfig}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// agentRateLimit: rate limit per menit dari DB (0 = tanpa limit).
func (d *Deps) agentRateLimit(ctx context.Context, agent string) int {
	row, err := d.queryRow(ctx, "SELECT rate_limit_per_min FROM taut_agent_configs WHERE agent = $1 LIMIT 1", agent)
	if err != nil || row == nil || !row.has("rate_limit_per_min") {
		return 3
	}
	return maxInt(int(row.numOr("rate_limit_per_min")), 0)
}

// waitForRateSlot: tunggu slot rate limit agent (spin-wait maks ~60 dtk).
func (d *Deps) waitForRateSlot(ctx context.Context, agent string) {
	limit := d.agentRateLimit(ctx, agent)
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

// runTautAgent: port runTautAgent — dedup run 'done' dengan input_hash sama,
// retry max 2 + backoff, catat taut_agent_runs + training sample.
func (d *Deps) runTautAgent(ctx context.Context, agent, campaignID, userPrompt string, mediaID *string, jsonMode bool) (*agentLLMResult, error) {
	mediaKey := ""
	if mediaID != nil {
		mediaKey = *mediaID
	}
	inputHash := tautInputHash(agent, campaignID, mediaKey, userPrompt)

	// Dedup: reuse run done dengan input sama.
	cached, err := d.queryRow(ctx,
		"SELECT id, model, token_input, token_output, duration_ms, output FROM taut_agent_runs WHERE agent = $1 AND input_hash = $2 AND status = 'done' ORDER BY created_at DESC LIMIT 1",
		agent, inputHash)
	if err == nil && cached != nil {
		content := outputContent(cached.raw("output"))
		if content != "" {
			return &agentLLMResult{
				Content:    content,
				Model:      cached.strOr("model", ""),
				UsageIn:    cached.intOr("token_input", 0),
				UsageOut:   cached.intOr("token_output", 0),
				DurationMS: int64(cached.intOr("duration_ms", 0)),
				RunID:      cached.strOr("id", ""),
			}, nil
		}
	}

	maxRetries := 2
	attempt := 0
	for {
		attempt++
		runID := uuid.NewString()
		if err := d.exec(ctx,
			"INSERT INTO taut_agent_runs (id, campaign_id, media_id, agent, status, input, input_hash) VALUES ($1, $2, $3, $4, 'running', $5::jsonb, $6)",
			runID, campaignID, mediaID, agent, jsonString(map[string]any{"prompt": truncateString(userPrompt, 8000)}), inputHash); err != nil {
			return nil, err
		}
		d.waitForRateSlot(ctx, agent)
		result, llmErr := d.callAgentLLM(ctx, agent, userPrompt, jsonMode, "", 0)
		if llmErr == nil {
			_ = d.exec(ctx,
				"UPDATE taut_agent_runs SET status='done', model=$1, token_input=$2, token_output=$3, duration_ms=$4, output=$5::jsonb, finished_at=NOW() WHERE id=$6",
				result.Model, result.UsageIn, result.UsageOut, result.DurationMS,
				jsonString(map[string]any{"content": result.Content}), runID)
			d.saveTrainingSample(ctx, campaignID, mediaID, agent, "done", userPrompt, result.Content, result.Model, result.UsageIn, result.UsageOut)
			result.RunID = runID
			return result, nil
		}
		message := llmErr.Error()
		_ = d.exec(ctx, "UPDATE taut_agent_runs SET status='failed', error=$1, finished_at=NOW() WHERE id=$2",
			truncateString(message, 1000), runID)
		d.saveTrainingSample(ctx, campaignID, mediaID, agent, "failed", userPrompt, message, "", 0, 0)
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

// saveTrainingSample: simpan pasangan input→output agent (best-effort).
func (d *Deps) saveTrainingSample(ctx context.Context, campaignID string, mediaID *string, agent, status, input, output, model string, tokenIn, tokenOut int) {
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
		slog.Debug("[clienttaut] training sample gagal", "err", err)
	}
}

// runProfilerAgent: port runProfilerAgent (tanpa dedup; tiap klik sebagai run baru).
func (d *Deps) runProfilerAgent(ctx context.Context, campaignID, userPrompt string) (*agentLLMResult, error) {
	runID := uuid.NewString()
	inputHash := uuid.NewString()
	if err := d.exec(ctx,
		"INSERT INTO taut_agent_runs (id, campaign_id, media_id, agent, status, input, input_hash) VALUES ($1, $2, $3, 'profiler', 'running', $4::jsonb, $5)",
		runID, campaignID, nil, jsonString(map[string]any{"prompt": truncateString(userPrompt, 8000)}), inputHash); err != nil {
		return nil, err
	}
	d.waitForRateSlot(ctx, "profiler")
	result, err := d.callAgentLLM(ctx, "profiler", userPrompt, false, "", 0)
	if err != nil {
		_ = d.exec(ctx, "UPDATE taut_agent_runs SET status='failed', error=$1, finished_at=NOW() WHERE id=$2",
			truncateString(err.Error(), 1000), runID)
		return nil, err
	}
	_ = d.exec(ctx,
		"UPDATE taut_agent_runs SET status='done', model=$1, token_input=$2, token_output=$3, duration_ms=$4, output=$5::jsonb, finished_at=NOW() WHERE id=$6",
		result.Model, result.UsageIn, result.UsageOut, result.DurationMS, jsonString(map[string]any{"content": result.Content}), runID)
	result.RunID = runID
	return result, nil
}

// setCampaignTautStatus: parity setCampaignTautStatus.
func (d *Deps) setCampaignTautStatus(ctx context.Context, campaignID, status string) error {
	return d.exec(ctx, "UPDATE campaigns SET taut_status = $1, updated_at = NOW() WHERE id = $2", status, campaignID)
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
		return d.setCampaignTautStatus(ctx, campaignID, status)
	}
	behind := tautStatusFlow[:idx]
	ph := placeholders(3, len(behind))
	sql := "UPDATE campaigns SET taut_status = $1, updated_at = NOW() WHERE id = $2 AND (taut_status IS NULL OR taut_status IN (" + ph + "))"
	args := []any{status, campaignID}
	for _, s := range behind {
		args = append(args, s)
	}
	return d.exec(ctx, sql, args...)
}

// ---------- json extraction (port lib/taut/json.ts) ----------

// extractJSONObject: ambil objek JSON dari respons AI (tahan markdown fence,
// prosa di sekitar JSON, dan output yang dibungkus key lain).
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
	fenced := strings.TrimSpace(reFence.ReplaceAllString(trimmed, ""))
	var fencedVal any
	if err := json.Unmarshal([]byte(fenced), &fencedVal); err == nil {
		if obj := asObject(fencedVal); obj != nil {
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

// ---------- matchmaker helpers (port lib/taut/matchmaker-ai.ts) ----------

type candidateInput struct {
	MediaID     string
	Name        string
	Category    *string
	Price       *float64
	CreditCost  *float64
	HealthScore *float64
	DA          *float64
	PA          *float64
}

type candidateLine struct {
	MediaID  string
	Name     string
	Category string
	PriceIDR float64
	DA       *float64
	Health   *float64
	Screen   float64
}

func screenScore(m candidateInput, budget *float64) float64 {
	price := 0.0
	if m.Price != nil {
		price = *m.Price
	}
	nameLower := strings.ToLower(m.Name)
	if strings.Contains(nameLower, "smoke") || strings.Contains(nameLower, "test") {
		return -999
	}
	s := 0.0
	if budget == nil {
		s += 40
	} else if price > 0 && price <= *budget {
		s += 50
	} else if price == 0 {
		s += 45
	} else {
		s += 15
	}
	if m.HealthScore != nil {
		s += (*m.HealthScore / 100) * 30
	} else {
		s += 15
	}
	if m.DA != nil || m.PA != nil {
		s += 15
	}
	if m.Category != nil {
		s += 5
	}
	return s
}

func toLine(m candidateInput, screen float64) candidateLine {
	category := "-"
	if m.Category != nil {
		category = *m.Category
	}
	price := 0.0
	if m.Price != nil {
		price = *m.Price
	}
	return candidateLine{
		MediaID: m.MediaID, Name: m.Name, Category: category, PriceIDR: price,
		DA: m.DA, Health: m.HealthScore, Screen: screen,
	}
}

// pickCandidatePool: pilih pool kandidat luas (max N) + wildcard acak.
func pickCandidatePool(media []candidateInput, budget *float64, maxPool, randomSeed int) []candidateLine {
	type scored struct {
		m      candidateInput
		screen float64
	}
	list := []scored{}
	for _, m := range media {
		s := screenScore(m, budget)
		if s >= 0 {
			list = append(list, scored{m, s})
		}
	}
	sortScored := func(items []scored) {
		// Stable sort menurun berdasarkan screen (parity [...].sort((a,b)=>b.screen-a.screen)).
		for i := 1; i < len(items); i++ {
			for j := i; j > 0 && items[j].screen > items[j-1].screen; j-- {
				items[j], items[j-1] = items[j-1], items[j]
			}
		}
	}
	sortScored(list)
	pool := []candidateLine{}
	for i := 0; i < len(list) && i < maxPool; i++ {
		pool = append(pool, toLine(list[i].m, list[i].screen))
	}
	if len(pool) < maxPool && len(list) > len(pool) {
		inPool := map[string]bool{}
		for _, p := range pool {
			inPool[p.MediaID] = true
		}
		outside := []scored{}
		for _, x := range list {
			if !inPool[x.m.MediaID] {
				outside = append(outside, x)
			}
		}
		// Fisher-Yates lalu ambil randomSeed pertama (parity Math.random shuffle).
		for i := len(outside) - 1; i > 0; i-- {
			j := randomInt(i + 1)
			outside[i], outside[j] = outside[j], outside[i]
		}
		limit := randomSeed
		if limit > len(outside) {
			limit = len(outside)
		}
		for i := 0; i < limit; i++ {
			pool = append(pool, toLine(outside[i].m, outside[i].screen))
		}
	}
	return pool
}

// fmtCandidate: format satu kandidat untuk prompt LLM.
func fmtCandidate(c candidateLine) string {
	price := "nego"
	if c.PriceIDR > 0 {
		price = "Rp" + formatIDNumber(int64(c.PriceIDR))
	}
	return c.MediaID + "|" + c.Name + "|" + c.Category + "|" + price + "|DA " + fmtNumOrDash(c.DA) + "|health " + fmtNumOrDash(c.Health)
}

func fmtNumOrDash(v *float64) string {
	if v == nil {
		return "-"
	}
	return trimFloat(*v)
}

// formatIDNumber: pemisah ribuan titik (parity toLocaleString("id-ID")).
func formatIDNumber(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	digits := itoa(int(n))
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, ch := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

type rankedItem struct {
	MediaID string
	Alasan  string
	Skor    int
}

var rankedKeys = []string{"ranked", "rankings", "results", "data", "items"}

// parseRanked: parse output LLM Matchmaker → RankedItem[] (toleran).
func parseRanked(raw string) []rankedItem {
	body := extractJSONAny(raw)
	var list []any
	if arr, ok := body.([]any); ok {
		list = arr
	} else if obj, ok := body.(map[string]any); ok {
		for _, key := range rankedKeys {
			if arr, ok := obj[key].([]any); ok {
				list = arr
				break
			}
		}
	}
	if list == nil {
		return nil
	}
	out := []rankedItem{}
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := ""
		for _, key := range []string{"media_id", "mediaId", "id"} {
			if v, exists := obj[key]; exists {
				switch t := v.(type) {
				case string:
					id = strings.TrimSpace(t)
				case float64:
					id = trimFloat(t)
				}
				if id != "" {
					break
				}
			}
		}
		if id == "" {
			continue
		}
		alasan := ""
		if s, ok := obj["alasan"].(string); ok {
			alasan = truncateString(strings.TrimSpace(s), 300)
		}
		skor := 0
		if v, exists := obj["skor"]; exists {
			if n, ok := v.(float64); ok {
				skor = clampInt(int(roundHalfUp(n)), 0, 100)
			}
		} else if v, exists := obj["score"]; exists {
			if n, ok := v.(float64); ok {
				skor = clampInt(int(roundHalfUp(n)), 0, 100)
			}
		}
		out = append(out, rankedItem{MediaID: id, Alasan: alasan, Skor: skor})
	}
	return out
}

// extractJSONAny: nilai JSON pertama dari teks LLM (objek/array terluar).
func extractJSONAny(raw string) any {
	cleaned := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	cleaned = strings.TrimSpace(reFenceLoose.ReplaceAllString(cleaned, ""))
	candidates := []string{cleaned}
	if s := strings.Index(cleaned, "{"); s >= 0 {
		if e := strings.LastIndex(cleaned, "}"); e > s {
			candidates = append(candidates, cleaned[s:e+1])
		}
	}
	if s := strings.Index(cleaned, "["); s >= 0 {
		if e := strings.LastIndex(cleaned, "]"); e > s {
			candidates = append(candidates, cleaned[s:e+1])
		}
	}
	for _, c := range candidates {
		var v any
		if err := json.Unmarshal([]byte(c), &v); err == nil {
			return v
		}
	}
	return nil
}

func roundHalfUp(v float64) float64 {
	if v < 0 {
		return float64(int(v - 0.5))
	}
	return float64(int(v + 0.5))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// angleLabels: parity ANGLE_LABELS (slug teknis → nama manusiawi).
var angleLabels = map[string]string{
	"product_launch":            "Peluncuran Produk",
	"special_offer_advertorial": "Promosi & Penawaran Spesial",
	"thought_leadership":        "Opini Pakar",
	"research_data_trends":      "Riset, Data & Tren Industri",
	"strategic_partnership_b2b": "Kemitraan Strategis & Ekspansi B2B",
	"milestone_awards":          "Pencapaian & Penghargaan",
	"funding_announcement":      "Pengumuman Pendanaan",
	"csr_sustainability":        "CSR & Keberlanjutan",
	"rebranding_transformation": "Rebranding & Transformasi",
	"c_level_change":            "Pergantian Manajemen",
	"inspirational_story":       "Kisah Inspiratif",
	"reputation_clarification":  "Manajemen Reputasi",
	"large_event_coverage":      "Liputan Event",
	"product_review":            "Review Produk",
	"comparison":                "Perbandingan Produk",
	"how_to":                    "Tips & Trik / How-to",
	"listicle":                  "Listicle / Rekomendasi",
	"success_case":              "Studi Kasus",
	"unboxing":                  "Unboxing",
	"soft_sell_promo":           "Promo Soft-sell",
	"niche_education":           "Edukasi Niche",
	"community_event":           "Liputan Event Komunitas",
}

// humanizeReason: ganti slug angle teknis dengan label manusiawi.
func humanizeReason(text string) string {
	if text == "" {
		return ""
	}
	out := text
	for slug, label := range angleLabels {
		re := wordBoundaryRe(slug)
		out = re.ReplaceAllString(out, label)
	}
	return strings.TrimSpace(out)
}

// buildMatchmakerContext: port buildMatchmakerContext — blok konteks brief
// profiler untuk prompt LLM Matchmaker.
func buildMatchmakerContext(brief map[string]any, usp string) string {
	if brief == nil {
		return ""
	}
	audienceList := []string{}
	if aud, ok := brief["audience"].(map[string]any); ok {
		audienceList = append(audienceList, strSlice(aud["selected"])...)
		if custom, ok := aud["custom"].(string); ok && custom != "" {
			audienceList = append(audienceList, custom)
		}
	}
	filtered := []string{}
	for _, a := range audienceList {
		if a != "" {
			filtered = append(filtered, a)
		}
	}
	audienceList = filtered

	blocks := []string{"[BRIEF KAMPANYE]"}
	if s, ok := brief["business_summary"].(string); ok && strings.TrimSpace(s) != "" {
		blocks = append(blocks, "Bisnis: "+truncateString(strings.TrimSpace(s), 800))
	}
	if strings.TrimSpace(usp) != "" {
		blocks = append(blocks, "USP: "+truncateString(strings.TrimSpace(usp), 500))
	}
	if len(audienceList) > 0 {
		blocks = append(blocks, "Audiens: "+truncateString(strings.Join(audienceList, ", "), 500))
	}
	if g, ok := brief["guardrails"].(map[string]any); ok {
		active, _ := g["active"].(bool)
		summary, _ := g["summary"].(string)
		if active && summary != "" {
			blocks = append(blocks, "Guardrails (WAJIB dipatuhi): "+truncateString(strings.TrimSpace(summary), 500))
		}
	}

	angleLines := []string{}
	if channels, ok := brief["channels"].([]any); ok {
		for _, c := range channels {
			ch, ok := c.(map[string]any)
			if !ok {
				continue
			}
			channel, _ := ch["channel"].(string)
			angle, _ := ch["angle"].(string)
			instruction, _ := ch["writing_instruction"].(string)
			if angle == "" && instruction == "" {
				continue
			}
			line := strings.TrimSpace("- [" + channel + ":" + angle + "] " + truncateString(strings.TrimSpace(instruction), 500))
			if len(line) > len("- [:]") {
				angleLines = append(angleLines, line)
			}
		}
	}
	if len(angleLines) > 0 {
		blocks = append(blocks, "")
		blocks = append(blocks, "[ANGLE & INSTRUKSI PENULISAN]")
		blocks = append(blocks, angleLines...)
	}

	blocks = append(blocks, "")
	blocks = append(blocks, "[ATURAN RANKING — WAJIB]")
	blocks = append(blocks, "1. Media dengan kategori/topik yang tidak relevan dengan brief di atas WAJIB berada di peringkat bawah atau tidak dimasukkan sama sekali.")
	blocks = append(blocks, "2. Tolak kandidat yang melanggar guardrails.")
	blocks = append(blocks, "3. Jangan mengarang: alasan 1 kalimat harus merujuk pada brief atau metrik kandidat.")
	return strings.Join(blocks, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
