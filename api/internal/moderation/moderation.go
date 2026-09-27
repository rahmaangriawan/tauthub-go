// Package moderation: port lib/moderation/* — rule layer deterministik
// (keyword/phrase/combo + normalisasi homoglyph/leet), AI classifier opsional,
// cache content_moderation_results, audit content_moderation_audits.
package moderation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

const Version = "v1"

const (
	SafeMax   = 29
	ReviewMax = 69
)

const (
	titleMax        = 300
	metaMax         = 500
	fragmentWindow  = 800
	maxFragments    = 4
	totalAIInputMax = 2000
)

// ClientSafeMessage: pesan aman untuk klien (parity service.ts).
func ClientSafeMessage(status string) string {
	switch status {
	case "blocked":
		return "Konten tidak dapat diproses karena terindikasi tidak sesuai dengan kebijakan publikasi Tulisio. Silakan periksa kembali materi yang Anda masukkan."
	case "review_required":
		return "Konten sedang menunggu pemeriksaan sebelum dapat diproses."
	default:
		return "Konten aman untuk diproses."
	}
}

func ClassifyByScore(score int) string {
	if score <= SafeMax {
		return "safe"
	}
	if score <= ReviewMax {
		return "review_required"
	}
	return "blocked"
}

// ---------- normalize (parity normalize.ts) ----------

var homoglyphs = map[rune]rune{
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'у': 'y', 'х': 'x', 'і': 'i', 'ј': 'j', 'ѕ': 's',
	'А': 'a', 'Е': 'e', 'О': 'o', 'Р': 'p', 'С': 'c', 'У': 'y', 'Х': 'x',
	'α': 'a', 'ε': 'e', 'ο': 'o', 'ρ': 'p',
}

var leet = map[rune]rune{
	'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '6': 'g', '7': 't', '8': 'b', '@': 'a', '$': 's', '+': 't',
}

var (
	compactSepRe = regexp.MustCompile(`([a-z0-9])[.\-_·•\s]+([a-z0-9])`)
	spaceRunRe   = regexp.MustCompile(`\s+`)
)

// collapsePunctRuns: padanan JS /([!?.])\1+/g -> "$1" (RE2 tak punya
// backreference, jadi diulang sampai stabil).
func collapsePunctRuns(s string) string {
	for _, ch := range []string{"!", "?", "."} {
		for strings.Contains(s, ch+ch) {
			s = strings.ReplaceAll(s, ch+ch, ch)
		}
	}
	return s
}

// NormalizeForModeration: NFKC → lowercase → homoglyph → leet → buang
// separator antar alnum (berulang) → "compact spaced" (parity persis).
func NormalizeForModeration(raw string) string {
	if raw == "" {
		return ""
	}
	s := strings.ToLower(raw)
	var b strings.Builder
	for _, r := range s {
		if mapped, ok := homoglyphs[r]; ok {
			b.WriteRune(mapped)
			continue
		}
		if mapped, ok := leet[r]; ok {
			b.WriteRune(mapped)
			continue
		}
		b.WriteRune(r)
	}
	s = b.String()

	compact := s
	for {
		next := compactSepRe.ReplaceAllString(compact, "$1$2")
		if next == compact {
			break
		}
		compact = next
	}
	spaced := s
	spaced = strings.NewReplacer(".", " ", "-", " ", "_", " ", "·", " ", "•", " ").Replace(spaced)
	spaced = collapsePunctRuns(spaced)
	spaced = strings.TrimSpace(spaceRunRe.ReplaceAllString(spaced, " "))
	if compact != spaced {
		return strings.TrimSpace(compact + " " + spaced)
	}
	return spaced
}

// BuildModerationInput: gabung field → raw (maks 5000) + normalized.
func BuildModerationInput(title, slug, metaDescription, excerpt string, fragments []string) (string, string) {
	parts := []string{}
	if title != "" {
		parts = append(parts, title)
	}
	if slug != "" {
		parts = append(parts, strings.ReplaceAll(slug, "-", " "))
	}
	if metaDescription != "" {
		parts = append(parts, metaDescription)
	}
	if excerpt != "" {
		parts = append(parts, excerpt)
	}
	if len(fragments) > 0 {
		parts = append(parts, strings.Join(fragments, " "))
	}
	raw := strings.Join(parts, "\n")
	if len(raw) > 5000 {
		raw = raw[:5000]
	}
	return raw, NormalizeForModeration(raw)
}

// ModerationHash: sha256(version + "\n" + input).
func ModerationHash(input string) string {
	sum := sha256.Sum256([]byte(Version + "\n" + input))
	return hex.EncodeToString(sum[:])
}

// ---------- rules (parity rules.ts) ----------

type ruleHit struct {
	ReasonCode string   `json:"reason_code"`
	Signals    []string `json:"signals"`
	Weight     int      `json:"weight"`
}

type RuleResult struct {
	Score                int       `json:"score"`
	Signals              []string  `json:"signals"`
	ReasonCodes          []string  `json:"reason_codes"`
	HasPromo             bool      `json:"has_promo"`
	HasInformational     bool      `json:"has_informational"`
	Hits                 []ruleHit `json:"hits"`
	SuspiciousFragments  []string  `json:"suspicious_fragments"`
}

type kwDef struct {
	kw   string
	code string
	w    int
}

var keywords = []kwDef{
	{"judi", "kw_judi", 25}, {"judol", "kw_judol", 35}, {"slot", "kw_slot", 20},
	{"gacor", "kw_gacor", 30}, {"casino", "kw_casino", 30}, {"togel", "kw_togel", 30},
	{"betting", "kw_betting", 25}, {"bookmaker", "kw_bookmaker", 25}, {"taruhan", "kw_taruhan", 25},
	{"bandar", "kw_bandar", 15}, {"rtp", "kw_rtp", 15},
}

var phrases = []kwDef{
	{"bonus deposit", "ph_bonus_deposit", 30}, {"deposit withdraw", "ph_deposit_withdraw", 25},
	{"daftar slot", "ph_daftar_slot", 30}, {"link slot", "ph_link_slot", 25},
	{"slot online", "ph_slot_online", 30}, {"judi online", "ph_judi_online", 35},
	{"judol online", "ph_judol_online", 35}, {"casino online", "ph_casino_online", 35},
	{"bonus new member", "ph_bonus_new_member", 30}, {"bonus referral", "ph_bonus_referral", 20},
	{"withdraw wd", "ph_withdraw", 15},
}

type comboDef struct {
	a, b, code string
	w          int
}

var combos = []comboDef{
	{"bonus", "deposit", "combo_bonus_deposit", 20},
	{"rtp", "gacor", "combo_rtp_gacor", 25},
	{"deposit", "withdraw", "combo_deposit_withdraw", 15},
	{"daftar", "slot", "combo_daftar_slot", 20},
	{"link", "slot", "combo_link_slot", 15},
	{"bonus", "slot", "combo_bonus_slot", 15},
}

var informationalSignals = []string{"pemberantasan", "penegakan hukum", "edukasi", "penelitian", "berita", "kebijakan", "analisis", "mencegah", "bahaya", "dampak"}

var promoSignals = []string{
	"daftar sekarang", "klik disini", "klik di sini", "bonus 100%", "deposit 100%",
	"claim bonus", "klaim bonus", "maxwin", "jackpot", "free spin", "freespin",
}

// RuleCheck: parity ruleCheck — skor, dampening informasional, promo boost.
func RuleCheck(rawInput, normalizedOverride string) RuleResult {
	normalized := normalizedOverride
	if normalized == "" {
		normalized = NormalizeForModeration(rawInput)
	}
	lowerRaw := strings.ToLower(rawInput)
	hits := []ruleHit{}
	signals := []string{}
	reasonCodes := []string{}

	for _, k := range keywords {
		if strings.Contains(normalized, k.kw) {
			hits = append(hits, ruleHit{k.code, []string{k.kw}, k.w})
			reasonCodes = append(reasonCodes, k.code)
			signals = append(signals, k.kw)
		}
	}
	for _, p := range phrases {
		if strings.Contains(normalized, p.kw) {
			hits = append(hits, ruleHit{p.code, []string{p.kw}, p.w})
			reasonCodes = append(reasonCodes, p.code)
			signals = append(signals, p.kw)
		}
	}
	for _, c := range combos {
		if strings.Contains(normalized, c.a) && strings.Contains(normalized, c.b) {
			key := c.a + "+" + c.b
			if !contains(signals, key) {
				hits = append(hits, ruleHit{c.code, []string{c.a, c.b}, c.w})
				reasonCodes = append(reasonCodes, c.code)
				signals = append(signals, key)
			}
		}
	}

	hasPromo := false
	for _, p := range promoSignals {
		if strings.Contains(lowerRaw, p) || strings.Contains(normalized, strings.ReplaceAll(p, " ", "")) {
			hasPromo = true
			hits = append(hits, ruleHit{"promo_cta", []string{p}, 25})
			reasonCodes = append(reasonCodes, "promo_cta")
			signals = append(signals, p)
			break
		}
	}

	hasInformational := false
	for _, s := range informationalSignals {
		if strings.Contains(normalized, s) {
			hasInformational = true
			signals = append(signals, "info:"+s)
			break
		}
	}

	score := 0
	for _, h := range hits {
		score += h.Weight
	}
	if hasInformational && !hasPromo && score > 0 {
		score = int(round(float64(score) * 0.8))
		if score > 69 {
			score = 69
		}
	}
	if hasPromo && anyGamblingSignal(signals) {
		score += 15
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	// Fragmen mencurigakan untuk konteks AI (parity).
	fragments := []string{}
	slice := rawInput
	if len(slice) > 5000 {
		slice = slice[:5000]
	}
	lowerSlice := strings.ToLower(slice)
	for i, sig := range signals {
		if i >= 6 {
			break
		}
		clean := strings.Split(strings.TrimPrefix(sig, "info:"), "+")[0]
		idx := strings.Index(lowerSlice, clean)
		if idx >= 0 {
			start := idx - 120
			if start < 0 {
				start = 0
			}
			end := idx + len(clean) + 120
			if end > len(slice) {
				end = len(slice)
			}
			fragments = append(fragments, strings.TrimSpace(slice[start:end]))
		}
	}
	if len(fragments) > 4 {
		fragments = fragments[:4]
	}

	return RuleResult{
		Score: score, Signals: dedupe(signals), ReasonCodes: dedupe(reasonCodes),
		HasPromo: hasPromo, HasInformational: hasInformational, Hits: hits,
		SuspiciousFragments: fragments,
	}
}

// ---------- service (parity service.ts) ----------

type Input struct {
	Title           *string
	Slug            *string
	MetaDescription *string
	Excerpt         *string
	Content         *string
	BrandID         *string
	UserID          *string
}

type Result struct {
	Status            string   `json:"status"`
	RiskScore         int      `json:"risk_score"`
	Category          string   `json:"category"`
	Confidence        float64  `json:"confidence"`
	ReasonCode        string   `json:"reason_code"`
	Signals           []string `json:"signals"`
	ContentHash       string   `json:"content_hash"`
	ModerationVersion string   `json:"moderation_version"`
	Model             *string  `json:"model"`
	AICalled          bool     `json:"ai_called"`
	CacheHit          bool     `json:"cache_hit"`
	RuleScore         int      `json:"rule_score"`
	ResultID          *string  `json:"result_id"`
	TokenInput        int      `json:"-"`
	TokenOutput       int      `json:"-"`
	TokenTotal        int      `json:"-"`
}

func buildInput(in Input) (string, string) {
	fragments := []string{}
	if in.Content != nil && *in.Content != "" {
		c := *in.Content
		if len(c) > 2000 {
			c = c[:2000]
		}
		fragments = append(fragments, c)
	}
	return BuildModerationInput(derefS(in.Title), derefS(in.Slug), derefS(in.MetaDescription), derefS(in.Excerpt), fragments)
}

// ModerateContent: cache lookup → rule check → (opsional) AI → simpan cache.
func ModerateContent(ctx context.Context, pool *pgxpool.Pool, in Input) Result {
	_, normalized := buildInput(in)
	hashInput := strings.TrimSpace(normalized)
	if len(hashInput) > 5000 {
		hashInput = hashInput[:5000]
	}
	contentHash := ModerationHash(hashInput)

	if cached, ok := lookupCache(ctx, pool, contentHash); ok {
		return cached
	}

	rule := RuleCheck(normalized, normalized)
	needAI := decideWhetherAIIsNeeded(rule.Score, rule.Signals, rule.HasPromo, rule.HasInformational)

	status := ClassifyByScore(rule.Score)
	result := Result{
		Status: status, RiskScore: rule.Score,
		Category: categoryFor(status, rule.HasPromo, rule.Signals),
		Confidence: 0.9, ReasonCode: firstOr(rule.ReasonCodes, "rule_only"),
		Signals: rule.Signals, ContentHash: contentHash, ModerationVersion: Version,
		RuleScore: rule.Score,
	}

	if needAI {
		aiRes, err := aiModerate(ctx, pool, in, rule.SuspiciousFragments)
		if err == nil && aiRes != nil {
			result.Status = aiRes.Classification
			result.RiskScore = aiRes.RiskScore
			result.Category = aiRes.Category
			result.Confidence = aiRes.Confidence
			result.ReasonCode = aiRes.ReasonCode
			result.Signals = dedupe(append(result.Signals, aiRes.Signals...))
			result.Model = &aiRes.Model
			result.AICalled = true
			result.TokenInput = aiRes.InputTokens
			result.TokenOutput = aiRes.OutputTokens
			result.TokenTotal = aiRes.TotalTokens
		}
	}

	// Simpan cache best-effort + result_id.
	resultID := uuid.NewString()
	signalsJSON, _ := json.Marshal(result.Signals)
	if _, err := pool.Exec(ctx, `
		INSERT INTO content_moderation_results
		  (id, content_hash, moderation_version, status, risk_score, category, confidence, reason_code, signals, model, ai_called, token_input, token_output, token_total)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13, $14)`,
		resultID, result.ContentHash, result.ModerationVersion, result.Status, result.RiskScore,
		result.Category, result.Confidence, result.ReasonCode, string(signalsJSON), result.Model,
		result.AICalled, result.TokenInput, result.TokenOutput, result.TokenTotal); err == nil {
		result.ResultID = &resultID
	}
	return result
}

func lookupCache(ctx context.Context, pool *pgxpool.Pool, contentHash string) (Result, bool) {
	var out Result
	var signalsRaw []byte
	var model *string
	var aiCalled bool
	err := pool.QueryRow(ctx, `
		SELECT id, status, risk_score, category, confidence, reason_code, signals, model, ai_called
		FROM content_moderation_results WHERE content_hash = $1 AND moderation_version = $2 LIMIT 1`,
		contentHash, Version).
		Scan(&out.ResultID, &out.Status, &out.RiskScore, &out.Category, &out.Confidence, &out.ReasonCode, &signalsRaw, &model, &aiCalled)
	if err != nil {
		return Result{}, false
	}
	_ = json.Unmarshal(signalsRaw, &out.Signals)
	out.Model = model
	out.AICalled = aiCalled
	out.CacheHit = true
	out.ContentHash = contentHash
	out.ModerationVersion = Version
	out.RuleScore = out.RiskScore
	return out, true
}

func decideWhetherAIIsNeeded(ruleScore int, signals []string, hasPromo, hasInformational bool) bool {
	if ruleScore <= 15 && !hasPromo {
		return false
	}
	if ruleScore >= 75 {
		return false
	}
	if hasInformational && ruleScore >= 20 && ruleScore < 75 {
		return true
	}
	if ruleScore >= 20 && ruleScore < 75 {
		return true
	}
	if hasPromo {
		for _, s := range signals {
			if strings.Contains(s, "slot") || strings.Contains(s, "judi") || strings.Contains(s, "bonus") ||
				strings.Contains(s, "rtp") || strings.Contains(s, "gacor") {
				return true
			}
		}
	}
	return false
}

func categoryFor(status string, hasPromo bool, signals []string) string {
	if status == "safe" {
		return "safe"
	}
	for _, s := range signals {
		if strings.Contains(s, "judi") || strings.Contains(s, "slot") || strings.Contains(s, "casino") ||
			strings.Contains(s, "togel") || strings.Contains(s, "gacor") || strings.Contains(s, "rtp") {
			return "gambling"
		}
	}
	if hasPromo {
		return "gambling"
	}
	return "other_policy"
}

// ---------- AI classifier (parity moderation/ai.ts) ----------

const moderationSystemPrompt = `You are a content safety classifier for gambling policy (Indonesian market). Classify the given text fragments.

Return ONLY valid JSON with keys:
- risk_score: 0-100 integer
- classification: "safe" | "review_required" | "blocked"
- category: "gambling" | "other_policy" | "safe"
- confidence: 0-1 float
- reason_code: short snake_case string
- signals: string array (2-6 short keywords)

Rules:
- Informational/educational/news about gambling without promotion => review_required or safe, NOT blocked unless it promotes.
- Promotional/transactional/call-to-action for slot/casino/togel/betting/judol => blocked (high risk).
- Ambiguous => review_required.
- Be conservative: do not over-block journalism.
- Output JSON only, no explanation.`

type aiModerationResult struct {
	RiskScore      int
	Classification string
	Category       string
	Confidence     float64
	ReasonCode     string
	Signals        []string
	Model          string
	Provider       string
	InputTokens    int
	OutputTokens   int
	TotalTokens    int
}

func aiModerate(ctx context.Context, pool *pgxpool.Pool, in Input, fragments []string) (*aiModerationResult, error) {
	parts := []string{}
	if in.Title != nil {
		parts = append(parts, "Title: "+truncRunes(*in.Title, titleMax))
	}
	if in.Slug != nil {
		parts = append(parts, "Slug: "+truncRunes(*in.Slug, 200))
	}
	if in.MetaDescription != nil {
		parts = append(parts, "Meta: "+truncRunes(*in.MetaDescription, metaMax))
	}
	if len(fragments) > 0 {
		limited := fragments
		if len(limited) > maxFragments {
			limited = limited[:maxFragments]
		}
		lines := []string{}
		for i, f := range limited {
			lines = append(lines, itoa(i+1)+". "+truncRunes(f, fragmentWindow))
		}
		parts = append(parts, "Fragments:\n"+strings.Join(lines, "\n"))
	}
	prompt := moderationSystemPrompt + "\n\n" + strings.Join(parts, "\n\n")
	if len(prompt) > totalAIInputMax {
		prompt = prompt[:totalAIInputMax]
	}

	result, err := ai.GenerateChat(ctx, pool, ai.GenerateChatOptions{
		Prompt: prompt, Tier: "economy", TaskType: "content_moderation",
		BrandID: in.BrandID, UserID: in.UserID,
	})
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(result.Content)
	cleaned = regexp.MustCompile(`(?i)^` + "```" + `(?:json)?\s*`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile("```\\s*$").ReplaceAllString(cleaned, "")
	cleaned = strings.TrimSpace(cleaned)
	var parsed struct {
		RiskScore      any      `json:"risk_score"`
		Classification string   `json:"classification"`
		Category       string   `json:"category"`
		Confidence     any      `json:"confidence"`
		ReasonCode     string   `json:"reason_code"`
		Signals        []string `json:"signals"`
	}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, err
	}
	out := &aiModerationResult{
		RiskScore: clampScore(parsed.RiskScore), Classification: parsed.Classification,
		Category: parsed.Category, Confidence: clampConf(parsed.Confidence),
		ReasonCode: parsed.ReasonCode, Signals: parsed.Signals,
		Model: result.Model, Provider: result.Provider,
		InputTokens: result.InputTokens, OutputTokens: result.OutputTokens, TotalTokens: result.TotalTokens,
	}
	if out.Classification != "safe" && out.Classification != "review_required" && out.Classification != "blocked" {
		out.Classification = ClassifyByScore(out.RiskScore)
	}
	if out.Category != "gambling" && out.Category != "other_policy" && out.Category != "safe" {
		out.Category = "other_policy"
	}
	if out.ReasonCode == "" {
		out.ReasonCode = "ai_classified"
	}
	return out, nil
}

// RecordAudit: INSERT content_moderation_audits (best-effort).
func RecordAudit(ctx context.Context, pool *pgxpool.Pool, entityType, entityID string, result Result, brandID *string) {
	signalsJSON, _ := json.Marshal(result.Signals)
	_, _ = pool.Exec(ctx, `
		INSERT INTO content_moderation_audits
		  (id, entity_type, entity_id, brand_id, content_hash, result_id, status, risk_score, category, reason_code,
		   signals, moderation_version, model, ai_called, token_input, token_output, token_total)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, $13, $14, $15, $16, $17)`,
		uuid.NewString(), entityType, entityID, brandID, result.ContentHash, result.ResultID,
		result.Status, result.RiskScore, result.Category, result.ReasonCode, string(signalsJSON),
		result.ModerationVersion, result.Model, result.AICalled,
		result.TokenInput, result.TokenOutput, result.TokenTotal)
}

// ---------- helpers ----------

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range list {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func anyGamblingSignal(signals []string) bool {
	for _, s := range signals {
		clean := strings.Split(s, "+")[0]
		switch clean {
		case "judi", "judol", "slot", "casino", "togel", "gacor", "rtp":
			return true
		}
	}
	return false
}

func firstOr(list []string, fallback string) string {
	if len(list) > 0 && list[0] != "" {
		return list[0]
	}
	return fallback
}

func round(v float64) float64 {
	if v < 0 {
		return float64(int(v - 0.5))
	}
	return float64(int(v + 0.5))
}

func clampScore(n any) int {
	v, ok := n.(float64)
	if !ok {
		return 50
	}
	s := int(round(v))
	if s < 0 {
		s = 0
	}
	if s > 100 {
		s = 100
	}
	return s
}

func clampConf(n any) float64 {
	v, ok := n.(float64)
	if !ok {
		return 0.5
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func derefS(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := []byte{}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}
