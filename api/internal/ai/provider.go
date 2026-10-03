// Port lib/ai/provider.ts (bagian gateway) — OpenAI-compatible /chat/completions
// client: deteksi provider dari URL, runtime DB + env fallback, retry
// 429/5xx (backoff 500ms*2^n cap 2s), usage ledger model_usage_events,
// mock fallback bila belum dikonfigurasi. Parity perilaku & kategori error.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

const fallbackModel = "gpt-4o-mini"

// DetectProvider: port detectProvider — urutan includes() dipertahankan.
func DetectProvider(baseURL string) string {
	v := strings.ToLower(baseURL)
	switch {
	case strings.Contains(v, "generativelanguage.googleapis.com"):
		return "google-gemini"
	case strings.Contains(v, "cloudflare"):
		return "cloudflare-workers"
	case strings.Contains(v, "openrouter.ai"):
		return "openrouter"
	case strings.Contains(v, "groq.com"):
		return "groq"
	case strings.Contains(v, "openai.com"):
		return "openai"
	case strings.Contains(v, "11434"):
		return "ollama"
	default:
		return "openai-compatible"
	}
}

// SecretCandidates: port secretCandidates() — env + parsing .env/.env.local
// naik 8 level direktori + lokasi umum (parity: unseal tetap berhasil walau
// proses berjalan dari subfolder).
func SecretCandidates() []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(v string) {
		t := strings.TrimSpace(v)
		if len(t) >= 8 && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	add(os.Getenv("CMS_ENCRYPTION_SECRET"))
	add(os.Getenv("APP_SECRET"))

	envFiles := []string{}
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		envFiles = append(envFiles, dir+"/.env", dir+"/.env.local")
		parent := parentDir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	cwd, _ := os.Getwd()
	envFiles = append(envFiles, cwd+"/apps/web/.env.local", cwd+"/.env.local")
	for _, file := range envFiles {
		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			t := strings.TrimSpace(line)
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			eq := strings.Index(t, "=")
			if eq == -1 {
				continue
			}
			key := strings.TrimSpace(t[:eq])
			if key == "APP_SECRET" || key == "CMS_ENCRYPTION_SECRET" {
				val := strings.TrimSpace(t[eq+1:])
				val = strings.Trim(val, `"`)
				add(val)
			}
		}
	}
	return out
}

func parentDir(dir string) string {
	for i := len(dir) - 1; i > 0; i-- {
		if dir[i] == '/' {
			return dir[:i]
		}
	}
	return dir
}

// AIRuntime: port interface AIRuntime.
type AIRuntime struct {
	Provider       string
	BaseURL        string
	APIKey         string
	DefaultModel   string
	EconomyModel   string
	PremiumModel   string
	EmbeddingModel string
	TimeoutSeconds int
	MaxRetries     int
	Source         string // database | environment | none
	APIKeyHint     string
	// Konfigurasi khusus Generate Gambar AI (terpisah dari chat). Kosong =
	// ikuti konfigurasi AI chat.
	ImageBaseURL    string
	ImageAPIKey     string
	ImageModel      string
	ImageAPIKeyHint string
	// ImageCreditCost: harga kredit per gambar (TC), diatur admin di halaman
	// AI Gambar > Atur Harga. Sumber: kolom platform_ai_configuration.
	ImageCreditCost int
}

type cfgCacheEntry struct {
	runtime *AIRuntime
	at      time.Time
}

var (
	runtimeMu    sync.Mutex
	runtimeCache *cfgCacheEntry
)

// InvalidateRuntimeCache: parity invalidateAiRuntimeCache (config berubah di admin).
func InvalidateRuntimeCache() {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	runtimeCache = nil
}

// LoadAIRuntime: DB platform_ai_configuration dulu (unseal multi-kandidat),
// env fallback. Cache 30 detik (parity OAUTH-style cache di route pemakaian;
// provider.ts sendiri selalu query — cache di sini mengikuti pemakaian route).
func LoadAIRuntime(ctx context.Context, pool *pgxpool.Pool) (*AIRuntime, error) {
	runtimeMu.Lock()
	if runtimeCache != nil && time.Since(runtimeCache.at) < 30*time.Second {
		rt := runtimeCache.runtime
		runtimeMu.Unlock()
		return rt, nil
	}
	runtimeMu.Unlock()

	rt := &AIRuntime{
		EmbeddingModel:  envStr("AI_EMBEDDING_MODEL"),
		TimeoutSeconds:  envIntOr("AI_REQUEST_TIMEOUT_SECONDS", 120),
		MaxRetries:      envIntOr("AI_MAX_RETRIES", 2),
		ImageCreditCost: ImageCreditCostEnv(),
	}

	var baseURL, sealedKey, keyHint, defaultModel, economyModel, premiumModel *string
	var imageBaseURL, imageSealedKey, imageKeyHint, imageModel *string
	var imageCreditCost *int
	err := pool.QueryRow(ctx,
		"SELECT base_url, sealed_api_key, api_key_hint, default_model, economy_model, premium_model, image_base_url, image_sealed_api_key, image_api_key_hint, image_model, image_credit_cost FROM platform_ai_configuration WHERE id = 'default' LIMIT 1",
	).Scan(&baseURL, &sealedKey, &keyHint, &defaultModel, &economyModel, &premiumModel, &imageBaseURL, &imageSealedKey, &imageKeyHint, &imageModel, &imageCreditCost)

	envDefault := envStr("AI_DEFAULT_MODEL")
	if envDefault == "" {
		envDefault = fallbackModel
	}

	if err == nil {
		apiKey := ""
		if sealedKey != nil && *sealedKey != "" {
			for _, candidate := range SecretCandidates() {
				if plain, unsealErr := UnsealWith(*sealedKey, candidate); unsealErr == nil && plain != "" {
					apiKey = plain
					break
				}
			}
		}
		effectiveKey := apiKey
		if effectiveKey == "" {
			effectiveKey = envStr("AI_API_KEY")
		}
		effectiveBase := derefStr(baseURL)
		if effectiveBase == "" {
			effectiveBase = envStr("AI_BASE_URL")
		}
		dm := derefStr(defaultModel)
		if dm == "" {
			dm = envDefault
		}
		em := derefStr(economyModel)
		if em == "" {
			em = dm
		}
		pm := derefStr(premiumModel)
		if pm == "" {
			pm = dm
		}
		hint := ""
		if apiKey != "" && keyHint != nil {
			hint = *keyHint
		}
		if hint == "" && effectiveKey != "" {
			hint = "••••" + lastN(effectiveKey, 4)
		}
		source := "database"
		if apiKey == "" && envStr("AI_API_KEY") != "" {
			source = "environment"
		}
		rt.Provider = DetectProvider(effectiveBase)
		rt.BaseURL = effectiveBase
		rt.APIKey = effectiveKey
		rt.DefaultModel = dm
		rt.EconomyModel = em
		rt.PremiumModel = pm
		rt.Source = source
		rt.APIKeyHint = hint

		// Konfigurasi gambar: kolom khusus dulu, lalu ikut konfigurasi chat,
		// terakhir env AI_IMAGE_*. Model gambar tidak pernah ikut model chat
		// (model chat bukan model gambar).
		imageKey := ""
		if imageSealedKey != nil && *imageSealedKey != "" {
			for _, candidate := range SecretCandidates() {
				if plain, unsealErr := UnsealWith(*imageSealedKey, candidate); unsealErr == nil && plain != "" {
					imageKey = plain
					break
				}
			}
		}
		imgKey := imageKey
		if imgKey == "" {
			imgKey = effectiveKey
		}
		if imgKey == "" {
			imgKey = envStr("AI_IMAGE_API_KEY")
		}
		imgBase := derefStr(imageBaseURL)
		if imgBase == "" {
			imgBase = effectiveBase
		}
		if imgBase == "" {
			imgBase = envStr("AI_IMAGE_BASE_URL")
		}
		imgModel := derefStr(imageModel)
		if imgModel == "" {
			imgModel = envStr("AI_IMAGE_MODEL")
		}
		imgHint := derefStr(imageKeyHint)
		if imgHint == "" && imgKey != "" {
			imgHint = "••••" + lastN(imgKey, 4)
		}
		rt.ImageBaseURL = imgBase
		rt.ImageAPIKey = imgKey
		rt.ImageModel = imgModel
		rt.ImageAPIKeyHint = imgHint
		if imageCreditCost != nil && *imageCreditCost >= 0 {
			rt.ImageCreditCost = *imageCreditCost
		}
	} else {
		hasEnv := envStr("AI_BASE_URL") != "" || envStr("AI_API_KEY") != ""
		source := "none"
		if hasEnv {
			source = "environment"
		}
		rt.Provider = envStr("AI_PROVIDER")
		rt.BaseURL = envStr("AI_BASE_URL")
		rt.APIKey = envStr("AI_API_KEY")
		rt.DefaultModel = envDefault
		rt.EconomyModel = envOr("AI_ECONOMY_MODEL", envDefault)
		rt.PremiumModel = envOr("AI_PREMIUM_MODEL", envDefault)
		rt.Source = source
		if rt.APIKey != "" {
			rt.APIKeyHint = "••••" + lastN(rt.APIKey, 4)
		}

		// Fallback env yang sama seperti cabang DB: ikut chat, lalu env AI_IMAGE_*.
		rt.ImageBaseURL = rt.BaseURL
		if rt.ImageBaseURL == "" {
			rt.ImageBaseURL = envStr("AI_IMAGE_BASE_URL")
		}
		rt.ImageAPIKey = rt.APIKey
		if rt.ImageAPIKey == "" {
			rt.ImageAPIKey = envStr("AI_IMAGE_API_KEY")
		}
		rt.ImageModel = envStr("AI_IMAGE_MODEL")
		if rt.ImageAPIKey != "" {
			rt.ImageAPIKeyHint = "••••" + lastN(rt.ImageAPIKey, 4)
		}
	}

	runtimeMu.Lock()
	runtimeCache = &cfgCacheEntry{runtime: rt, at: time.Now()}
	runtimeMu.Unlock()
	return rt, nil
}

// ResolveModel: port resolveModel.
func ResolveModel(rt *AIRuntime, tier string) string {
	switch tier {
	case "economy":
		if rt.EconomyModel != "" {
			return rt.EconomyModel
		}
	case "premium":
		if rt.PremiumModel != "" {
			return rt.PremiumModel
		}
	}
	return rt.DefaultModel
}

// IsAIConfigured: parity isAiConfigured.
func IsAIConfigured(rt *AIRuntime) bool {
	return rt.BaseURL != "" && rt.APIKey != "" && rt.DefaultModel != ""
}

// ---------- usage ledger ----------

type UsageEvent struct {
	BrandID      *string
	UserID       *string
	TaskType     string
	Provider     string
	Model        string
	RequestID    *string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	LatencyMS    int64
	Success      bool
	ErrorCategory *string
	CacheHit     bool
}

// RecordUsage: INSERT model_usage_events (parity kolom).
func RecordUsage(ctx context.Context, pool *pgxpool.Pool, event UsageEvent) {
	_, err := pool.Exec(ctx,
		"INSERT INTO model_usage_events (id, brand_id, user_id, task_type, provider, model, request_id, input_tokens, output_tokens, total_tokens, latency_ms, success, error_category, cache_hit) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)",
		uuid.NewString(), event.BrandID, event.UserID, event.TaskType, event.Provider, event.Model,
		event.RequestID, event.InputTokens, event.OutputTokens, event.TotalTokens,
		event.LatencyMS, event.Success, event.ErrorCategory, event.CacheHit)
	if err != nil {
		slog.Error("[ai] recordUsage gagal", "err", err)
	}
}

func mockContent(prompt string) string {
	head := strings.TrimSpace(prompt)
	suffix := ""
	if len(prompt) > 160 {
		head = truncateRunes(head, 160)
		suffix = "…"
	}
	return "[mock] AI_API_KEY is not configured. Prompt echo: " + head + suffix
}

// ---------- generateChat ----------

type ProviderError struct {
	Category string
	Message  string
}

func (e *ProviderError) Error() string { return e.Message }

type GenerationResult struct {
	Content      string `json:"content"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	RequestID    *string `json:"request_id"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	LatencyMS    int64  `json:"latency_ms"`
	Mocked       bool   `json:"mocked"`
}

type GenerateChatOptions struct {
	Prompt        string
	Tier          string
	TaskType      string
	BrandID       *string
	UserID        *string
	ModelOverride string
	Runtime       *AIRuntime
}

type chatPayload struct {
	ID     string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// GenerateChat: port generateChat — mock fallback, retry 429/5xx dengan
// backoff min(500ms*2^attempt, 2s), non-2xx lain langsung gagal, usage ledger.
func GenerateChat(ctx context.Context, pool *pgxpool.Pool, opts GenerateChatOptions) (*GenerationResult, error) {
	rt := opts.Runtime
	if rt == nil {
		loaded, err := LoadAIRuntime(ctx, pool)
		if err != nil {
			return nil, err
		}
		rt = loaded
	}
	tier := opts.Tier
	if tier == "" {
		tier = "default"
	}
	model := opts.ModelOverride
	if model == "" {
		model = ResolveModel(rt, tier)
	}
	if model == "" {
		model = fallbackModel
	}
	taskType := opts.TaskType
	if taskType == "" {
		taskType = "test_generation"
	}
	started := time.Now()

	// Mock fallback (parity: tanpa apiKey/baseUrl → mock + usage cache_hit).
	if rt.APIKey == "" || rt.BaseURL == "" {
		latency := time.Since(started).Milliseconds()
		provider := rt.Provider
		if provider == "" {
			provider = "mock"
		}
		RecordUsage(ctx, pool, UsageEvent{
			BrandID: opts.BrandID, UserID: opts.UserID, TaskType: taskType,
			Provider: provider, Model: model, LatencyMS: latency,
			Success: true, CacheHit: true,
		})
		return &GenerationResult{
			Content: mockContent(opts.Prompt), Provider: provider, Model: model,
			RequestID: nil, LatencyMS: latency, Mocked: true,
		}, nil
	}

	url := strings.TrimRight(rt.BaseURL, "/") + "/chat/completions"
	var payload *chatPayload
	var failure *ProviderError

	client := &http.Client{Timeout: time.Duration(rt.TimeoutSeconds) * time.Second}
	for attempt := 0; attempt <= rt.MaxRetries; attempt++ {
		body, _ := json.Marshal(map[string]any{
			"model":    model,
			"messages": []map[string]string{{"role": "user", "content": opts.Prompt}},
		})
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if reqErr != nil {
			failure = &ProviderError{Category: "network", Message: "Provider connection failed"}
		} else {
			req.Header.Set("Authorization", "Bearer "+rt.APIKey)
			req.Header.Set("Content-Type", "application/json")
			resp, doErr := client.Do(req)
			if doErr != nil {
				failure = &ProviderError{Category: "network", Message: "Provider connection failed"}
			} else if resp.StatusCode == 429 || resp.StatusCode >= 500 {
				resp.Body.Close()
				failure = &ProviderError{Category: "upstream_retryable", Message: "Provider temporarily unavailable"}
			} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				detail, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
				resp.Body.Close()
				failure = &ProviderError{Category: "upstream_rejected", Message: fmt.Sprintf("Provider rejected (%d): %s", resp.StatusCode, string(detail))}
				break
			} else {
				var parsed chatPayload
				if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
					resp.Body.Close()
					failure = &ProviderError{Category: "network", Message: "Provider connection failed"}
				} else {
					resp.Body.Close()
					payload = &parsed
					failure = nil
					break
				}
			}
		}
		if attempt < rt.MaxRetries {
			backoff := time.Duration(500*(1<<attempt)) * time.Millisecond
			if backoff > 2*time.Second {
				backoff = 2 * time.Second
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	latency := time.Since(started).Milliseconds()

	if payload == nil {
		err := failure
		if err == nil {
			err = &ProviderError{Category: "unknown", Message: "AI generation failed"}
		}
		cat := err.Category
		RecordUsage(ctx, pool, UsageEvent{
			BrandID: opts.BrandID, UserID: opts.UserID, TaskType: taskType,
			Provider: rt.Provider, Model: model, LatencyMS: latency,
			Success: false, ErrorCategory: &cat,
		})
		return nil, err
	}

	content := ""
	if len(payload.Choices) > 0 {
		content = payload.Choices[0].Message.Content
	}
	if content == "" {
		cat := "invalid_response"
		RecordUsage(ctx, pool, UsageEvent{
			BrandID: opts.BrandID, UserID: opts.UserID, TaskType: taskType,
			Provider: rt.Provider, Model: model, LatencyMS: latency,
			Success: false, ErrorCategory: &cat,
		})
		return nil, &ProviderError{Category: "invalid_response", Message: "Provider returned an invalid response"}
	}

	usageIn := payload.Usage.PromptTokens
	usageOut := payload.Usage.CompletionTokens
	usageTotal := payload.Usage.TotalTokens
	if usageTotal == 0 {
		usageTotal = usageIn + usageOut
	}
	var requestID *string
	if payload.ID != "" {
		requestID = &payload.ID
	}
	RecordUsage(ctx, pool, UsageEvent{
		BrandID: opts.BrandID, UserID: opts.UserID, TaskType: taskType,
		Provider: rt.Provider, Model: model, RequestID: requestID,
		InputTokens: usageIn, OutputTokens: usageOut, TotalTokens: usageTotal,
		LatencyMS: latency, Success: true,
	})

	return &GenerationResult{
		Content: content, Provider: rt.Provider, Model: model, RequestID: requestID,
		InputTokens: usageIn, OutputTokens: usageOut, TotalTokens: usageTotal,
		LatencyMS: latency, Mocked: false,
	}, nil
}

// UnsealWith: wrapper agar provider.go memakai crypto.Unseal.
func UnsealWith(sealed, appSecret string) (string, error) {
	return crypto.Unseal(sealed, appSecret)
}

func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func fmtSscan(v string, out *int) (int, error) {
	n, err := strconv.Atoi(v)
	if err == nil {
		*out = n
	}
	return 0, err
}

// ---------- env helpers ----------

func envStr(key string) string { return strings.TrimSpace(os.Getenv(key)) }

func envIntOr(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		n := 0
		if _, err := fmtSscan(v, &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envOr(key, def string) string {
	if v := envStr(key); v != "" {
		return v
	}
	return def
}

func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
