package content

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

// Port lib/ai/provider.ts runContentJob + parse/word-count rules.

type RunOptions struct {
	BrandID               string
	UserID                string
	Topic                 string
	ContentType           string
	TargetWords           int
	RequireOutlineApproval bool
	Tier                  string
	SearchIntent          *string
	Angle                 *string
	Audience              *string
	Goal                  *string
	SourceMaterial        *string
	EditorialRules        *string
	MediaContext          *string
}

type Draft struct {
	Status           string
	CurrentStep      string
	Progress         int
	OutlineStructure []OutlineItem
	DraftContent     *string
	DraftFormat      string
	Evaluation       map[string]any
	ErrorMessage     *string
}

// RunContentJob: parity runContentJob — mode approval outline atau draft penuh
// (minimal 300 kata; kurang → failed agar job di-retry).
func RunContentJob(ctx context.Context, pool *pgxpool.Pool, opts RunOptions, runtime *ai.AIRuntime) Draft {
	brief := Brief{
		Topic: opts.Topic, ContentType: opts.ContentType, TargetWords: opts.TargetWords,
		SearchIntent: opts.SearchIntent, Angle: opts.Angle, Audience: opts.Audience,
		Goal: opts.Goal, SourceMaterial: opts.SourceMaterial,
		EditorialRules: opts.EditorialRules, MediaContext: opts.MediaContext,
	}

	if opts.RequireOutlineApproval {
		result, err := ai.GenerateChat(ctx, pool, ai.GenerateChatOptions{
			Prompt: outlinePrompt(brief), Tier: opts.Tier, TaskType: "outline_generation",
			BrandID: &opts.BrandID, UserID: &opts.UserID, Runtime: runtime,
		})
		if err != nil {
			msg := truncRunes(err.Error(), 500)
			return Draft{
				Status: "failed", CurrentStep: "outline_failed", Progress: 20,
				OutlineStructure: []OutlineItem{}, DraftFormat: "markdown",
				Evaluation: map[string]any{}, ErrorMessage: &msg,
			}
		}
		parsed := ParseOutline(result.Content)
		if len(parsed) == 0 {
			parsed = []OutlineItem{{Level: 2, Title: opts.Topic}}
		}
		return Draft{
			Status: "waiting_outline_approval", CurrentStep: "outline_ready", Progress: 40,
			OutlineStructure: parsed, DraftFormat: "markdown",
			Evaluation: map[string]any{
				"provider": result.Provider, "model": result.Model,
				"total_tokens": result.TotalTokens, "latency_ms": result.LatencyMS,
				"mocked": result.Mocked, "outline_parsed": len(ParseOutline(result.Content)),
			},
		}
	}

	outline := []OutlineItem{{Level: 2, Title: opts.Topic}}
	result, err := ai.GenerateChat(ctx, pool, ai.GenerateChatOptions{
		Prompt: articlePrompt(opts, brief), Tier: opts.Tier, TaskType: "content_generation",
		BrandID: &opts.BrandID, UserID: &opts.UserID, Runtime: runtime,
	})
	if err != nil {
		msg := truncRunes(err.Error(), 500)
		return Draft{
			Status: "failed", CurrentStep: "failed", Progress: 100,
			OutlineStructure: outline, DraftFormat: "markdown",
			Evaluation: map[string]any{}, ErrorMessage: &msg,
		}
	}

	bodyWords := BodyWordCount(result.Content)
	if bodyWords < 300 {
		msg := fmt.Sprintf("Artikel hanya %d kata — minimal 300 kata. Coba generate ulang.", bodyWords)
		return Draft{
			Status: "failed", CurrentStep: "failed", Progress: 100,
			OutlineStructure: outline, DraftFormat: "markdown",
			Evaluation: map[string]any{
				"provider": result.Provider, "model": result.Model,
				"word_count": bodyWords, "note": "Artikel kurang dari 300 kata",
			},
			ErrorMessage: &msg,
		}
	}

	content := result.Content
	return Draft{
		Status: "completed", CurrentStep: "completed", Progress: 100,
		OutlineStructure: outline, DraftContent: &content, DraftFormat: "markdown",
		Evaluation: map[string]any{
			"provider": result.Provider, "model": result.Model,
			"total_tokens": result.TotalTokens, "latency_ms": result.LatencyMS,
			"mocked": result.Mocked, "word_count": bodyWords,
		},
	}
}

// ---------- editorial rules loader (port knowledge-base/generation.ts + loader.ts) ----------

var (
	coreCategoryDefault = []string{"News Article", "Journalistic Structure", "Lead", "Headline", "5W+1H", "Fact & Opinion", "Attribution", "Anti Hallucination", "Language Style", "Ethics", "AI Governance", "SEO"}
	coreByType          = map[string][]string{
		"press_release":    {"Press Release", "Journalistic Structure", "Lead", "Headline", "5W+1H", "Attribution", "Quote", "Data & Numbers", "Anti Hallucination", "Fact & Opinion", "Language Style", "Ethics", "Anti Clickbait", "Corporate Language", "AI Governance"},
		"event":            {"Press Release", "Journalistic Structure", "Lead", "Headline", "5W+1H", "Attribution", "Quote", "Data & Numbers", "Anti Hallucination", "Ethics"},
		"csr":              {"Press Release", "Journalistic Structure", "Lead", "Headline", "5W+1H", "Attribution", "Quote", "Data & Numbers", "Anti Hallucination", "Ethics"},
		"product_launch":   {"Press Release", "Journalistic Structure", "Lead", "Headline", "5W+1H", "Attribution", "Quote", "Data & Numbers", "Anti Hallucination", "Fact & Opinion", "Corporate Language"},
		"personal_branding": {"Journalistic Structure", "Lead", "Headline", "5W+1H", "Attribution", "Anti Hallucination", "Fact & Opinion", "Privacy", "Ethics"},
		"news":             {"News Article", "Journalistic Structure", "Lead", "Headline", "5W+1H", "Fact & Opinion", "Attribution", "Anti Hallucination", "Language Style", "Ethics", "AI Governance", "SEO"},
		"feature":          {"Journalistic Structure", "Lead", "Headline", "Fact & Opinion", "Attribution", "Anti Hallucination", "Language Style", "SEO"},
		"opinion":          {"Fact & Opinion", "Attribution", "Anti Hallucination", "Ethics", "Language Style"},
		"seo_article":      {"SEO", "Language Style", "Anti Hallucination", "Headline", "Paragraph"},
		"product_content":  {"Fact & Opinion", "Attribution", "Anti Hallucination", "Corporate Language", "SEO", "Privacy"},
	}
)

type editorialRule struct {
	ID                string
	Category          string
	Title             string
	Instruction       string
	Examples          []string
	ForbiddenExamples []string
	ContentTypes      []string
	Audience          []string
	Priority          int
	Status            string
}

type kbBundle struct {
	KBID    string
	KBName  string
	KBVer   int
	Rules   []editorialRule
}

// LoadEditorialRulesForGeneration: content-type standard block + global KB
// rules (prefilter + hybrid RAG), failure-tolerant per layer.
func LoadEditorialRulesForGeneration(ctx context.Context, pool *pgxpool.Pool, contentType, topic string, sourceMaterial *string, audience *string) *string {
	blocks := []string{}

	if block := loadContentTypeStandardBlock(ctx, pool, contentType); block != "" {
		blocks = append(blocks, block)
	}

	if bundles := loadActiveKnowledgeBases(ctx, pool); len(bundles) > 0 {
		query := strings.TrimSpace(strings.Join([]string{topic, deref(sourceMaterial)}, " "))
		if len(query) > 1500 {
			query = query[:1500]
		}
		if query == "" {
			query = contentType
		}
		for _, bundle := range bundles {
			selected := selectRules(ctx, pool, bundle, contentType, topic, query, audience)
			if len(selected) == 0 {
				continue
			}
			if block := buildEditorialRulesBlock(selected, bundle.KBName, bundle.KBVer); block != "" {
				blocks = append(blocks, block)
			}
		}
	}

	if len(blocks) == 0 {
		return nil
	}
	joined := strings.Join(blocks, "\n\n---\n\n")
	return &joined
}

func loadActiveKnowledgeBases(ctx context.Context, pool *pgxpool.Pool) []kbBundle {
	rowsQ, err := pool.Query(ctx, `
		SELECT kb.id, kb.name, v.version, r.id, r.category, r.title, r.instruction,
		       r.examples, r.forbidden_examples, r.content_types, r.audience, r.priority, r.status
		FROM knowledge_bases kb
		JOIN knowledge_base_versions v ON v.knowledge_base_id = kb.id AND v.status = 'active'
		JOIN knowledge_base_rules r ON r.version_id = v.id
		WHERE kb.status = 'active' AND r.status = 'active'
		ORDER BY kb.id, r.priority, r.category, r.id`)
	if err != nil {
		return nil
	}
	defer rowsQ.Close()
	order := []string{}
	byID := map[string]*kbBundle{}
	for rowsQ.Next() {
		var kbID, kbName, ruleID, category, title, instruction, status string
		var version, priority int
		var examples, forbidden, contentTypes, audienceRaw []byte
		if rowsQ.Scan(&kbID, &kbName, &version, &ruleID, &category, &title, &instruction,
			&examples, &forbidden, &contentTypes, &audienceRaw, &priority, &status) != nil {
			continue
		}
		bundle, ok := byID[kbID]
		if !ok {
			bundle = &kbBundle{KBID: kbID, KBName: kbName, KBVer: version}
			byID[kbID] = bundle
			order = append(order, kbID)
		}
		if priority == 0 {
			priority = 5
		}
		bundle.Rules = append(bundle.Rules, editorialRule{
			ID: ruleID, Category: category, Title: title, Instruction: instruction,
			Examples: parseStrArray(examples), ForbiddenExamples: parseStrArray(forbidden),
			ContentTypes: parseStrArray(contentTypes), Audience: parseStrArray(audienceRaw),
			Priority: priority, Status: status,
		})
	}
	out := []kbBundle{}
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out
}

func passesPrefilter(rule editorialRule, contentType, audience *string) bool {
	ct := strings.ToLower(strings.TrimSpace(deref(contentType)))
	core := coreByType[ct]
	if core == nil {
		core = coreCategoryDefault
	}
	for _, c := range core {
		if c == rule.Category {
			return true
		}
	}
	if contentTypeMatches(rule.ContentTypes, ct) {
		return true
	}
	if audience != nil && *audience != "" && len(rule.Audience) > 0 {
		for _, a := range rule.Audience {
			if strings.EqualFold(a, *audience) {
				return true
			}
		}
	}
	return false
}

func contentTypeMatches(ruleTypes []string, ct string) bool {
	if len(ruleTypes) == 0 {
		return true
	}
	if ct == "" {
		return true
	}
	for _, t := range ruleTypes {
		norm := strings.ToLower(strings.TrimSpace(t))
		if norm == ct || (ct == "article" && norm == "news") {
			return true
		}
	}
	return false
}

// selectRules: prefilter deterministik + hybrid RAG (chunk embeddings) →
// dedupe → sort priority → cap panjang (700 utk RAG, parity 3500 char block).
func selectRules(ctx context.Context, pool *pgxpool.Pool, bundle kbBundle, contentType, topic, query string, audience *string) []editorialRule {
	prefiltered := []editorialRule{}
	byID := map[string]editorialRule{}
	for _, r := range bundle.Rules {
		if r.Status != "active" || !passesPrefilter(r, &contentType, audience) {
			continue
		}
		prefiltered = append(prefiltered, r)
		byID[r.ID] = r
	}
	if len(prefiltered) == 0 {
		return nil
	}

	// RAG: chunk rules → skor hybrid 0.65 cosine + 0.35 leksikal, ambil top-6.
	const semanticLimit = 6
	const similarityThreshold = 0.1
	ids := []string{}
	for _, r := range prefiltered {
		ids = append(ids, r.ID)
	}
	scores := map[string]float64{}
	if chunks, err := loadRuleChunks(ctx, pool, ids); err == nil {
		qvec := ai.LocalEmbedding(query)
		for _, ch := range chunks {
			var embedding []float64
			if json.Unmarshal(ch.Embedding, &embedding) != nil || len(embedding) != len(qvec) {
				continue
			}
			semantic := ai.CosineSimilarity(qvec, embedding)
			lexical := ai.LexicalJaccard(query, ch.Content)
			score := 0.65*semantic + 0.35*lexical
			if score < similarityThreshold {
				continue
			}
			if best, ok := scores[ch.RuleID]; !ok || score > best {
				scores[ch.RuleID] = score
			}
		}
	}

	topIDs := []string{}
	for id, _ := range scores {
		topIDs = append(topIDs, id)
	}
	sortByScoreDesc(topIDs, scores)
	if len(topIDs) > semanticLimit {
		topIDs = topIDs[:semanticLimit]
	}

	merged := map[string]editorialRule{}
	for _, r := range prefiltered {
		merged[r.ID] = r
	}
	for _, id := range topIDs {
		if r, ok := byID[id]; ok {
			merged[id] = r
		}
	}
	rules := []editorialRule{}
	for _, r := range merged {
		rules = append(rules, r)
	}
	sortRules(rules)

	// Cap panjang: prioritas ≤3 selalu penuh; sisanya sampai 3500 char.
	protected := []editorialRule{}
	truncatable := []editorialRule{}
	for _, r := range rules {
		if r.Priority <= 3 {
			protected = append(protected, r)
		} else {
			truncatable = append(truncatable, r)
		}
	}
	out := append([]editorialRule{}, protected...)
	used := 0
	for _, r := range protected {
		used += len(r.Instruction) + len(r.Title)
	}
	for _, r := range truncatable {
		added := len(r.Instruction) + len(r.Title)
		if used+added > 3500 {
			break
		}
		out = append(out, r)
		used += added
	}
	sortRules(out)
	return out
}

type ruleChunk struct {
	RuleID    string
	Content   string
	Embedding []byte
}

func loadRuleChunks(ctx context.Context, pool *pgxpool.Pool, ruleIDs []string) ([]ruleChunk, error) {
	if len(ruleIDs) == 0 {
		return nil, nil
	}
	out := []ruleChunk{}
	for start := 0; start < len(ruleIDs); start += 500 {
		end := start + 500
		if end > len(ruleIDs) {
			end = len(ruleIDs)
		}
		batch := ruleIDs[start:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, id := range batch {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}
		rowsQ, err := pool.Query(ctx,
			"SELECT rule_id, content, embedding FROM knowledge_base_rule_chunks WHERE rule_id IN ("+strings.Join(placeholders, ", ")+") ORDER BY rule_id, chunk_index",
			args...)
		if err != nil {
			return nil, err
		}
		for rowsQ.Next() {
			var rc ruleChunk
			if rowsQ.Scan(&rc.RuleID, &rc.Content, &rc.Embedding) == nil {
				out = append(out, rc)
			}
		}
		rowsQ.Close()
	}
	return out, nil
}

// buildEditorialRulesBlock: parity injector.ts.
func buildEditorialRulesBlock(rules []editorialRule, kbName string, kbVersion int) string {
	if len(rules) == 0 {
		return ""
	}
	lines := []string{
		`ATURAN EDITORIAL (Knowledge Base): "` + kbName + `" v` + itoa(kbVersion) + ".",
		"Ikuti semua aturan ini. Jika aturan bertentangan, prioritas dengan angka lebih kecil menang (1 = keselamatan/etika/akurasi, 8 = engagement).",
		"Aturan ini TIDAK BOLEH mengubah fakta dari bahan baku, dan TIDAK BOLEH menambah fakta yang tidak tersedia.",
		"",
	}
	for _, rule := range rules {
		parts := []string{"- [" + rule.Category + "] (prioritas " + itoa(rule.Priority) + ") " + rule.Instruction}
		if len(rule.Examples) > 0 && rule.Examples[0] != "" {
			parts = append(parts, "  Contoh baik: "+rule.Examples[0])
		}
		if len(rule.ForbiddenExamples) > 0 && rule.ForbiddenExamples[0] != "" {
			parts = append(parts, "  Hindari: "+rule.ForbiddenExamples[0])
		}
		lines = append(lines, strings.Join(parts, "\n"))
	}
	out := strings.Join(lines, "\n")
	if len(out) > 3500 {
		out = out[:3500]
	}
	return out
}

// loadContentTypeStandardBlock: port buildContentTypeEditorialPromptBlock.
func loadContentTypeStandardBlock(ctx context.Context, pool *pgxpool.Pool, contentType string) string {
	code := strings.ToLower(strings.TrimSpace(contentType))
	if code == "" {
		return ""
	}
	var v struct {
		ContentPurpose        *string
		NewsAngle             *string
		JournalisticStyle     *string
		RequiredInformation   *string
		OptionalInformation   *string
		StructureGuidance     *string
		LeadGuidance          *string
		FactAttributionRules  *string
		QuoteGuidance         *string
		LanguageTone          *string
		WhatToAvoid           *string
		AngleVariationRules   *string
		SourcePriority        *string
		FactConflictHandling  *string
		EditorialQualityChecklist *string
	}
	err := pool.QueryRow(ctx, `
		SELECT v.content_purpose, v.news_angle, v.journalistic_style, v.required_information,
		       v.optional_information, v.structure_guidance, v.lead_guidance, v.fact_attribution_rules,
		       v.quote_guidance, v.language_tone, v.what_to_avoid, v.angle_variation_rules,
		       v.source_priority, v.fact_conflict_handling, v.editorial_quality_checklist
		FROM content_type_standard_versions v
		JOIN content_type_standards s ON s.id = v.standard_id
		WHERE s.content_type_code = $1 AND s.status = 'active' AND v.status = 'active'
		LIMIT 1`, code).Scan(
		&v.ContentPurpose, &v.NewsAngle, &v.JournalisticStyle, &v.RequiredInformation,
		&v.OptionalInformation, &v.StructureGuidance, &v.LeadGuidance, &v.FactAttributionRules,
		&v.QuoteGuidance, &v.LanguageTone, &v.WhatToAvoid, &v.AngleVariationRules,
		&v.SourcePriority, &v.FactConflictHandling, &v.EditorialQualityChecklist)
	if err != nil {
		return ""
	}
	sections := [][2]string{
		{"TUJUAN KONTEN", deref(v.ContentPurpose)},
		{"ANGLE BERITA", deref(v.NewsAngle)},
		{"GAYA JURNALISTIK", deref(v.JournalisticStyle)},
		{"INFORMASI WAJIB", deref(v.RequiredInformation)},
		{"INFORMASI OPSIONAL", deref(v.OptionalInformation)},
		{"PANDUAN STRUKTUR", deref(v.StructureGuidance)},
		{"PANDUAN LEAD", deref(v.LeadGuidance)},
		{"ATURAN ATRIBUSI FAKTA", deref(v.FactAttributionRules)},
		{"PANDUAN KUTIPAN", deref(v.QuoteGuidance)},
		{"BAHASA & NADA", deref(v.LanguageTone)},
		{"YANG HARUS DIHINDARI", deref(v.WhatToAvoid)},
		{"ATURAN VARIASI ANGLE", deref(v.AngleVariationRules)},
		{"PRIORITAS SUMBER", deref(v.SourcePriority)},
		{"PENANGANAN KONFLIK FAKTA", deref(v.FactConflictHandling)},
		{"CHECKLIST KUALITAS EDITORIAL", deref(v.EditorialQualityChecklist)},
	}
	lines := []string{"STANDAR EDITORIAL KHUSUS TIPE KONTEN: " + code + "."}
	for _, s := range sections {
		if strings.TrimSpace(s[1]) != "" {
			lines = append(lines, "", "## "+s[0], s[1])
		}
	}
	if len(lines) == 1 {
		return ""
	}
	out := strings.Join(lines, "\n")
	if len(out) > 3500 {
		out = out[:3500]
	}
	return out
}

// ---------- helpers ----------

func parseStrArray(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return arr
	}
	var single string
	if json.Unmarshal(raw, &single) == nil && single != "" {
		return []string{single}
	}
	return nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func sortByScoreDesc(ids []string, scores map[string]float64) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && scores[ids[j]] > scores[ids[j-1]]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}

func sortRules(rules []editorialRule) {
	for i := 1; i < len(rules); i++ {
		for j := i; j > 0; j-- {
			a, b := rules[j], rules[j-1]
			if a.Priority < b.Priority || (a.Priority == b.Priority && a.Category < b.Category) {
				rules[j], rules[j-1] = rules[j-1], rules[j]
				continue
			}
			break
		}
	}
}

var _ = regexp.MustCompile
