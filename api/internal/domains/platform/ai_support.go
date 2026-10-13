package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

// Port lib/ai/provider.ts (config/runtime helpers), lib/model-knowledge/
// chunker.ts + loader.ts, dan skema zod aiConfiguration/aiProbe/aiTest.
//
// Helper di berkas ini diberi awalan `ai` supaya tidak bentrok dengan helper
// modul platform lain yang berbagi package.

// ---------- validasi body ala zod v4 ----------

// aiZodReceivedType: parity util.parsedType zod (typeof JS + null/array).
func aiZodReceivedType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, int, int64, json.Number:
		return "number"
	case []any:
		return "array"
	default:
		return "object"
	}
}

func aiZodInvalidType(expected, received string) error {
	return errUnprocessable("Invalid input: expected " + expected + ", received " + received)
}

func aiZodInvalidOption(options ...string) error {
	if len(options) == 1 {
		return errUnprocessable("Invalid input: expected " + aiJSONString(options[0]))
	}
	quoted := make([]string, len(options))
	for i, o := range options {
		quoted[i] = aiJSONString(o)
	}
	return errUnprocessable("Invalid option: expected one of " + strings.Join(quoted, "|"))
}

func aiJSONString(v string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `"` + v + `"`
	}
	return string(b)
}

// aiStringField: parity z.string()[.trim()].min(n).max(m)[.nullable().optional()].
// has=false berarti field tidak dikirim (atau null saat nullable) → pemanggil
// memakai nilai default/lama.
func aiStringField(body map[string]any, key string, minLen, maxLen int, trim, nullable bool) (string, bool, error) {
	v, ok := body[key]
	if !ok {
		return "", false, nil
	}
	if v == nil {
		if nullable {
			return "", false, nil
		}
		return "", true, aiZodInvalidType("string", "null")
	}
	s, isString := v.(string)
	if !isString {
		return "", true, aiZodInvalidType("string", aiZodReceivedType(v))
	}
	if trim {
		s = strings.TrimSpace(s)
	}
	if minLen > 0 && utf8.RuneCountInString(s) < minLen {
		return "", true, errUnprocessable(ptZStringTooSmall(minLen))
	}
	if maxLen > 0 && utf8.RuneCountInString(s) > maxLen {
		return "", true, errUnprocessable(ptZStringTooBig(maxLen))
	}
	return s, true, nil
}

// aiEnumField: parity z.enum([...]).default(def) — field kosong → def.
func aiEnumField(body map[string]any, key, def string, options ...string) (string, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return def, nil
	}
	s, isString := v.(string)
	if !isString {
		return "", aiZodInvalidType("string", aiZodReceivedType(v))
	}
	for _, o := range options {
		if o == s {
			return s, nil
		}
	}
	return "", aiZodInvalidOption(options...)
}

// aiRequiredStringField: parity z.string().trim().min(n).max(m) tanpa optional
// — field wajib ada dan bertipe string.
func aiRequiredStringField(body map[string]any, key string, minLen, maxLen int) (string, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return "", aiZodInvalidType("string", "undefined")
	}
	s, isString := v.(string)
	if !isString {
		return "", aiZodInvalidType("string", aiZodReceivedType(v))
	}
	s = strings.TrimSpace(s)
	if minLen > 0 && utf8.RuneCountInString(s) < minLen {
		return "", errUnprocessable(ptZStringTooSmall(minLen))
	}
	if maxLen > 0 && utf8.RuneCountInString(s) > maxLen {
		return "", errUnprocessable(ptZStringTooBig(maxLen))
	}
	return s, nil
}

// aiBoolField: parity z.boolean().optional().
func aiBoolField(body map[string]any, key string) (bool, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return false, false, nil
	}
	b, isBool := v.(bool)
	if !isBool {
		return false, true, aiZodInvalidType("boolean", aiZodReceivedType(v))
	}
	return b, true, nil
}

// aiIntField: parity z.number().int().min(a).max(b).optional().
func aiIntField(body map[string]any, key string, min, max int) (int, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return 0, false, nil
	}
	f, isNum := v.(float64)
	if !isNum {
		return 0, true, aiZodInvalidType("number", aiZodReceivedType(v))
	}
	if f != math.Trunc(f) {
		return 0, true, errUnprocessable("Invalid input: expected int, received number")
	}
	n := int(f)
	if n < min {
		return 0, true, errUnprocessable(ptZNumberTooSmall(min))
	}
	if n > max {
		return 0, true, errUnprocessable(ptZNumberTooBig(max))
	}
	return n, true, nil
}

// aiObjectField: parity z.record(...) / z.object(...) — objek JSON.
func aiObjectField(body map[string]any, key string) (map[string]any, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return nil, false, nil
	}
	obj, isObj := v.(map[string]any)
	if !isObj {
		return nil, true, aiZodInvalidType("object", aiZodReceivedType(v))
	}
	return obj, true, nil
}

// aiReadJSONAny: baca body apa adanya. ok=false → JSON rusak / body kosong
// (route lama `await request.json()` throw → 500); JSON literal "null"
// menghasilkan value nil dengan ok=true.
func aiReadJSONAny(r *http.Request) (any, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		return nil, false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false
	}
	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

// aiParsedObject: parity `schema.parse(await request.json())` — JSON rusak /
// body kosong → 500 (error dibiarkan naik), body valid non-objek → 422 dengan
// pesan invalid_type zod.
func aiParsedObject(w http.ResponseWriter, d *Deps, r *http.Request) (map[string]any, bool) {
	value, ok := aiReadJSONAny(r)
	if !ok {
		writeErr(w, d, errors.New("Unexpected end of JSON input"))
		return nil, false
	}
	obj, isObj := value.(map[string]any)
	if !isObj {
		writeErr(w, d, aiZodInvalidType("object", aiZodReceivedType(value)))
		return nil, false
	}
	return obj, true
}

// aiStrictObject: parity `schema.parse(await request.json().catch(() => null))`
// — JSON rusak / body kosong → null → 422 invalid_type(object, null).
func aiStrictObject(w http.ResponseWriter, d *Deps, r *http.Request) (map[string]any, bool) {
	value, ok := aiReadJSONAny(r)
	if !ok {
		writeErr(w, d, aiZodInvalidType("object", "null"))
		return nil, false
	}
	obj, isObj := value.(map[string]any)
	if !isObj {
		writeErr(w, d, aiZodInvalidType("object", aiZodReceivedType(value)))
		return nil, false
	}
	return obj, true
}

// aiSafeParseObject: parity `request.json().then(d => schema.safeParse(d))` —
// JSON rusak → 500, nilai selain objek / field tidak valid → 422 customMsg.
func aiSafeParseObject(w http.ResponseWriter, d *Deps, r *http.Request, customMsg string) (map[string]any, bool) {
	value, ok := aiReadJSONAny(r)
	if !ok {
		writeErr(w, d, errors.New("Unexpected end of JSON input"))
		return nil, false
	}
	obj, isObj := value.(map[string]any)
	if !isObj {
		writeErr(w, d, errUnprocessable(customMsg))
		return nil, false
	}
	return obj, true
}

// aiOrEmpty: parity `a || b` untuk string (empty → fallback).
func aiOrEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ---------- config AI ----------

// aiConfigJSON: port configResponse di ai/configuration/route.ts.
func aiConfigJSON(stored rowMap, rt *ai.AIRuntime, writerCost int) map[string]any {
	imgCost := ai.ImageCreditCost(rt)
	if stored != nil {
		dm := aiOrEmpty(stored.strOr("default_model", ""), rt.DefaultModel)
		imgBase := aiOrEmpty(stored.strOr("image_base_url", ""), rt.ImageBaseURL)
		imgModel := aiOrEmpty(stored.strOr("image_model", ""), rt.ImageModel)
		imgHint := aiOrEmpty(stored.strOr("image_api_key_hint", ""), rt.ImageAPIKeyHint)
		imgConfigured := rt.ImageAPIKey != "" || stored.strOr("image_sealed_api_key", "") != ""
		return map[string]any{
			"base_url":                 stored.strOr("base_url", ""),
			"api_key_configured":       true,
			"api_key_hint":             ptNullableStr(stored, "api_key_hint"),
			"source":                   "database",
			"default_model":            dm,
			"economy_model":            aiOrEmpty(stored.strOr("economy_model", ""), dm),
			"premium_model":            aiOrEmpty(stored.strOr("premium_model", ""), dm),
			"image_base_url":           imgBase,
			"image_model":              imgModel,
			"image_api_key_configured": imgConfigured,
			"image_api_key_hint":       nullIfEmpty(imgHint),
			"image_credit_cost":        imgCost,
			"writer_credit_cost":       writerCost,
		}
	}
	if rt.Source != "none" {
		return map[string]any{
			"base_url":                 rt.BaseURL,
			"api_key_configured":       rt.APIKey != "",
			"api_key_hint":             nullIfEmpty(rt.APIKeyHint),
			"source":                   rt.Source,
			"default_model":            rt.DefaultModel,
			"economy_model":            aiOrEmpty(rt.EconomyModel, rt.DefaultModel),
			"premium_model":            aiOrEmpty(rt.PremiumModel, rt.DefaultModel),
			"image_base_url":           rt.ImageBaseURL,
			"image_model":              rt.ImageModel,
			"image_api_key_configured": rt.ImageAPIKey != "",
			"image_api_key_hint":       nullIfEmpty(rt.ImageAPIKeyHint),
			"image_credit_cost":        imgCost,
			"writer_credit_cost":       writerCost,
		}
	}
	dm := rt.DefaultModel
	return map[string]any{
		"base_url":                 "",
		"api_key_configured":       false,
		"api_key_hint":             nil,
		"source":                   "none",
		"default_model":            dm,
		"economy_model":            aiOrEmpty(rt.EconomyModel, dm),
		"premium_model":            aiOrEmpty(rt.PremiumModel, dm),
		"image_base_url":           rt.ImageBaseURL,
		"image_model":              rt.ImageModel,
		"image_api_key_configured": rt.ImageAPIKey != "",
		"image_api_key_hint":       nullIfEmpty(rt.ImageAPIKeyHint),
	}
}

// aiNormalizeURL: port normalizeUrl — trim, buang trailing slash, wajib
// http/https tanpa kredensial. nil = tidak valid.
func aiNormalizeURL(raw string) *string {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	lower := strings.ToLower(v)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return nil
	}
	rest := v[strings.Index(v, "://")+3:]
	if rest == "" {
		return nil
	}
	authority := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		authority = rest[:i]
	}
	if authority == "" || strings.Contains(authority, "@") {
		return nil
	}
	return &v
}

// aiStoredConfig: parity storedAiConfiguration (kolom terpilih).
func (d *Deps) aiStoredConfig(ctx context.Context) rowMap {
	row, err := d.queryRow(ctx,
		"SELECT base_url, sealed_api_key, api_key_hint, default_model, economy_model, premium_model, image_base_url, image_sealed_api_key, image_api_key_hint, image_model, updated_by, updated_at FROM platform_ai_configuration WHERE id = 'default' LIMIT 1")
	if err != nil {
		return nil
	}
	return row
}

// ---------- model knowledge ----------

// aiKnowledgeSections: parity MODEL_KNOWLEDGE_SECTIONS (urutan wajib sama).
var aiKnowledgeSections = []struct {
	key      string
	label    string
	priority int
}{
	{"summary", "Ringkasan Model", 1},
	{"characteristics", "Karakteristik Model", 2},
	{"suitable_for", "Cocok Untuk", 1},
	{"unsuitable_for", "Tidak Ideal Untuk", 1},
	{"guidelines", "Pedoman Penggunaan AI", 1},
	{"limitations", "Catatan / Limitasi", 2},
	{"editorial_context", "RAG Knowledge / Editorial Context", 2},
}

const aiEmbeddingDim = 128

var aiEmbedTokenRe = regexp.MustCompile(`[^a-z0-9_/-]+`)

// aiEmbedText: port embedModelKnowledgeText — hash bucket SHA-256 + bigram +
// bobot posisi + L2 normalize (vektor harus identik dengan Node agar cocok
// dengan chunk lama di DB).
func aiEmbedText(text string) []float64 {
	vec := make([]float64, aiEmbeddingDim)
	normalized := strings.Join(strings.Fields(strings.ToLower(text)), " ")
	if normalized == "" {
		return vec
	}
	tokens := make([]string, 0, 16)
	for _, t := range aiEmbedTokenRe.Split(normalized, -1) {
		if len(t) > 1 {
			tokens = append(tokens, t)
		}
	}
	if len(tokens) == 0 {
		return vec
	}
	for i, token := range tokens {
		hash := sha256.Sum256([]byte(token))
		bucket := int(binary.BigEndian.Uint16(hash[0:2])) % aiEmbeddingDim
		sign := -1.0
		if hash[2]&1 == 1 {
			sign = 1.0
		}
		weight := 1 + math.Log(1+1/float64(i+1))
		vec[bucket] += sign * weight
		if i < len(tokens)-1 {
			bi := sha256.Sum256([]byte(token + "_" + tokens[i+1]))
			biBucket := int(binary.BigEndian.Uint16(bi[0:2])) % aiEmbeddingDim
			biSign := -1.0
			if bi[2]&1 == 1 {
				biSign = 1.0
			}
			vec[biBucket] += biSign * 0.5
		}
	}
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

type aiKnowledgeChunk struct {
	Section    string
	ChunkIndex int
	Content    string
	Embedding  []float64
	Priority   int
}

// aiGenerateChunks: port generateModelKnowledgeChunks.
func aiGenerateChunks(modelID, modelName, reasoningLevel string, sections map[string]string) []aiKnowledgeChunk {
	if modelID == "" {
		modelID = "unknown-model"
	}
	if modelName == "" {
		modelName = modelID
	}
	level := strings.ToUpper(reasoningLevel)
	if level == "" {
		level = "SEDANG"
	}
	out := []aiKnowledgeChunk{}
	for _, s := range aiKnowledgeSections {
		raw := strings.TrimSpace(sections[s.key])
		if s.key == "summary" && reasoningLevel != "" {
			prefix := "Tingkat Reasoning: " + level + "."
			if raw != "" {
				raw = prefix + " " + raw
			} else {
				raw = prefix
			}
		}
		if raw == "" {
			continue
		}
		full := "[AI Model Knowledge: " + modelName + " (" + modelID + ") — " + s.label + "]\n" + raw
		out = append(out, aiKnowledgeChunk{
			Section: s.key, ChunkIndex: 0, Content: full,
			Embedding: aiEmbedText(full), Priority: s.priority,
		})
	}
	return out
}

const aiKnowledgeSelect = `SELECT id, model_id, model_name, provider, reasoning_level, summary, characteristics,
	suitable_for, unsuitable_for, guidelines, limitations, editorial_context,
	status, chunk_count, created_at, updated_at
	FROM ai_model_knowledge WHERE model_id = $1 AND status = 'active' LIMIT 1`

// aiKnowledgeJSON: port rowToKnowledge (loadModelKnowledge).
func aiKnowledgeJSON(row rowMap) map[string]any {
	reasoning := aiOrEmpty(row.strOr("reasoning_level", ""), "sedang")
	status := aiOrEmpty(row.strOr("status", ""), "active")
	return map[string]any{
		"id":                row.strOr("id", ""),
		"model_id":          row.strOr("model_id", ""),
		"model_name":        ptNullableStr(row, "model_name"),
		"provider":          ptNullableStr(row, "provider"),
		"reasoning_level":   reasoning,
		"summary":           ptNullableStr(row, "summary"),
		"characteristics":   ptNullableStr(row, "characteristics"),
		"suitable_for":      ptNullableStr(row, "suitable_for"),
		"unsuitable_for":    ptNullableStr(row, "unsuitable_for"),
		"guidelines":        ptNullableStr(row, "guidelines"),
		"limitations":       ptNullableStr(row, "limitations"),
		"editorial_context": ptNullableStr(row, "editorial_context"),
		"status":            status,
		"chunk_count":       row.intOr("chunk_count", 0),
		"created_at":        aiIsoOrNow(row.timePtr("created_at")),
		"updated_at":        aiIsoOrNow(row.timePtr("updated_at")),
	}
}

// aiLoadKnowledge: parity loadModelKnowledge — baris aktif untuk satu model.
func (d *Deps) aiLoadKnowledge(ctx context.Context, modelID string) map[string]any {
	id := strings.TrimSpace(modelID)
	if id == "" {
		return nil
	}
	row, err := d.queryRow(ctx, aiKnowledgeSelect, id)
	if err != nil || row == nil {
		return nil
	}
	return aiKnowledgeJSON(row)
}

// aiIsoOrNow: parity `r.x ? new Date(r.x).toISOString() : new Date().toISOString()`.
func aiIsoOrNow(v any) any {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return isoMilli(time.Now())
}

// aiKnownModels: parity KNOWN_MODELS (katalog bawaan untuk pre-fill admin).
var aiKnownModels = []struct{ ModelID, ModelName, Provider string }{
	{"gpt-4o-mini", "GPT-4o Mini", "openai"},
	{"gpt-4o", "GPT-4o", "openai"},
	{"gemini-2.0-flash", "Gemini 2.0 Flash", "google"},
	{"gemini-1.5-flash", "Gemini 1.5 Flash", "google"},
	{"gemini-1.5-pro", "Gemini 1.5 Pro", "google"},
	{"google/gemini-2.0-flash-exp:free", "Gemini 2.0 Flash (OpenRouter)", "openrouter"},
	{"meta-llama/llama-3.3-70b-instruct", "Llama 3.3 70B (OpenRouter)", "openrouter"},
	{"anthropic/claude-3.5-sonnet", "Claude 3.5 Sonnet (OpenRouter)", "openrouter"},
	{"@cf/meta/llama-3.1-8b-instruct-fast", "Llama 3.1 8B Fast (Cloudflare)", "cloudflare"},
	{"@cf/meta/llama-3.1-8b-instruct", "Llama 3.1 8B (Cloudflare)", "cloudflare"},
	{"@cf/meta/llama-3.1-405b-instruct", "Llama 3.1 405B (Cloudflare)", "cloudflare"},
	{"llama-3.3-70b-versatile", "Llama 3.3 70B (Groq)", "groq"},
	{"llama-3.1-8b-instant", "Llama 3.1 8B Instant (Groq)", "groq"},
	{"llama3.2", "Llama 3.2 (Ollama)", "ollama"},
	{"llama3.2:1b", "Llama 3.2 1B (Ollama)", "ollama"},
	{"qwen2.5:14b", "Qwen 2.5 14B (Ollama)", "ollama"},
}

// aiModelJSON: item unified list di ai/models/route.ts.
func aiModelJSON(item map[string]any) map[string]any {
	knowledge, _ := item["knowledge"].(map[string]any)
	modelName := item["model_name"]
	provider := item["provider"]
	var reasoning any
	var chunkCount any = 0
	var knowledgeOut any
	if knowledge != nil {
		knowledgeOut = knowledge
		if s, ok := knowledge["model_name"].(string); ok && s != "" {
			modelName = s
		}
		if s, ok := knowledge["provider"].(string); ok && s != "" {
			provider = s
		}
		reasoning = knowledge["reasoning_level"]
		if n, ok := knowledge["chunk_count"]; ok {
			chunkCount = n
		}
	}
	return map[string]any{
		"model_id":        item["model_id"],
		"model_name":      modelName,
		"provider":        provider,
		"tier":            item["tier"],
		"is_active":       item["is_active"],
		"has_knowledge":   knowledge != nil,
		"reasoning_level": reasoning,
		"chunk_count":     chunkCount,
		"knowledge":       knowledgeOut,
	}
}

// writerCreditCostSetting: tarif penulisan The Writer dari platform_settings.
// Dipakai endpoint setelan AI supaya halaman admin bisa menampilkan dan
// mengubahnya tanpa deploy.
func (d *Deps) writerCreditCostSetting(ctx context.Context) int {
	row, err := d.queryRow(ctx, "SELECT value FROM platform_settings WHERE key = 'writer_credit_cost' LIMIT 1")
	if err == nil && row != nil {
		if n, convErr := strconv.Atoi(strings.TrimSpace(row.strOr("value", ""))); convErr == nil && n > 0 {
			return n
		}
	}
	return 2
}
