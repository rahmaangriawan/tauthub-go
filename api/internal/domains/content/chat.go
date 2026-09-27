package contentdomain

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
	knowledge "github.com/rahmaangriawan/tauthub-go/internal/domains/knowledge"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/[brandId]/chat/route.ts — grounding RAG + benchmark chat
// learning + readiness gate + brief wizard + pembuatan artikel.

const (
	chatLearnLimit     = 3
	chatLearnSimThresh = 0.45
)

var createPrefixes = []string{
	"buat artikel", "bikin artikel", "tulis artikel", "generate artikel", "buatin artikel",
	"buat draft", "bikin draft", "tulis draft", "buat konten", "bikin konten", "tulis konten", "generate konten",
}

var lastArticleMarkers = []string{
	"artikel tadi", "artikel barusan", "artikel yang itu", "artikel tersebut", "artikel yang barusan",
	"draft tadi", "draft barusan", "draft yang itu", "draft tersebut", "konten tadi",
}

func chatAction(label, href, variant string) map[string]any {
	return map[string]any{"label": label, "href": href, "variant": variant}
}

// classifyIntent: parity (create_article bila topik terdeteksi, small_talk,
// needs_memory, needs_knowledge).
func classifyIntent(message, topic string) string {
	if topic != "" {
		return "create_article"
	}
	lower := strings.ToLower(strings.TrimSpace(message))
	if regexp.MustCompile(`^(hai|halo|hi|hey|pagi|siang|sore|malam|terima kasih|makasih|thanks|oke|ok)\b`).MatchString(lower) {
		return "small_talk"
	}
	if regexp.MustCompile(`(tadi|barusan|sebelumnya|ingat|ingatkan|percakapan|preferensi|kesukaan|yang saya bilang|jawabanmu)`).MatchString(lower) {
		return "needs_memory"
	}
	return "needs_knowledge"
}

// extractTopic: parity — ambil topik setelah prefix "buat artikel ...".
func extractTopic(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	for _, prefix := range createPrefixes {
		if strings.HasPrefix(lower, prefix) {
			topic := strings.TrimSpace(message[len(prefix):])
			topic = strings.Trim(topic, " \t,:.")
			topic = regexp.MustCompile(`(?i)^(?:tentang|mengenai|soal|dengan judul|berjudul)\s+`).ReplaceAllString(topic, "")
			return topic
		}
	}
	return ""
}

// retrieveBenchmarks: chat_learnings dengan cosine ≥ threshold, top 3.
func (d *Deps) retrieveBenchmarks(ctx context.Context, brandID, question string) []map[string]any {
	queryVector := ai.LocalEmbedding(question)
	rowsQ, err := d.Pool.Query(ctx, `
		SELECT id, question, answer, intent_tag, question_embedding
		FROM chat_learnings WHERE brand_id = $1 ORDER BY created_at DESC LIMIT 50`, brandID)
	if err != nil {
		return nil
	}
	defer rowsQ.Close()
	type scoredRow struct {
		id, question, answer, intentTag string
		sim                             float64
	}
	scored := []scoredRow{}
	for rowsQ.Next() {
		var id, question, answer, intentTag string
		var embeddingRaw []byte
		if rowsQ.Scan(&id, &question, &answer, &intentTag, &embeddingRaw) != nil {
			continue
		}
		var embedding []float64
		if json.Unmarshal(embeddingRaw, &embedding) != nil || len(embedding) != len(queryVector) {
			continue
		}
		sim := ai.CosineSimilarity(queryVector, embedding)
		if sim >= chatLearnSimThresh {
			scored = append(scored, scoredRow{id, question, answer, intentTag, sim})
		}
	}
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0 && scored[j].sim > scored[j-1].sim; j-- {
			scored[j], scored[j-1] = scored[j-1], scored[j]
		}
	}
	out := []map[string]any{}
	for i, s := range scored {
		if i >= chatLearnLimit {
			break
		}
		out = append(out, map[string]any{
			"id": s.id, "question": s.question, "answer": s.answer,
			"intent_tag": s.intentTag, "sim": s.sim,
		})
	}
	return out
}

func (d *Deps) storeChatLearning(ctx context.Context, brandID, conversationID, userID, question, answer, intentTag string) {
	if intentTag == "" {
		intentTag = "general"
	}
	embedding, _ := json.Marshal(ai.LocalEmbedding(question))
	q := truncRunes(question, 4000)
	a := truncRunes(answer, 8000)
	_, _ = d.Pool.Exec(ctx, `
		INSERT INTO chat_learnings (id, brand_id, conversation_id, user_id, question, question_embedding, intent_tag, answer, context_used, benchmark_ids)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, '{}'::jsonb, '[]'::jsonb)`,
		uuid.NewString(), brandID, conversationID, userID, q, string(embedding), intentTag, a)
}

type pendingBrief struct {
	Topic    string  `json:"topic"`
	Angle    *string `json:"angle"`
	Words    *int    `json:"words"`
	Audience *string `json:"audience"`
	Goal     *string `json:"goal"`
}

func (d *Deps) checkPendingBrief(ctx context.Context, conversationID, brandID string) *pendingBrief {
	var answer string
	err := d.Pool.QueryRow(ctx, `
		SELECT answer FROM chat_learnings
		WHERE conversation_id = $1 AND brand_id = $2 AND intent_tag = 'brief_pending'
		ORDER BY created_at DESC LIMIT 1`, conversationID, brandID).Scan(&answer)
	if err != nil {
		return nil
	}
	var pb pendingBrief
	if json.Unmarshal([]byte(answer), &pb) != nil {
		return nil
	}
	return &pb
}

func (d *Deps) storePendingBrief(ctx context.Context, conversationID, brandID, userID, topic string) {
	embedding, _ := json.Marshal(ai.LocalEmbedding(topic))
	details, _ := json.Marshal(pendingBrief{Topic: topic})
	_, _ = d.Pool.Exec(ctx, `
		INSERT INTO chat_learnings (id, brand_id, conversation_id, user_id, question, question_embedding, intent_tag, answer, context_used, benchmark_ids)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'brief_pending', $7, '{}'::jsonb, '[]'::jsonb)`,
		uuid.NewString(), brandID, conversationID, userID, "buat artikel "+topic, string(embedding), string(details))
}

func (d *Deps) updatePendingBrief(ctx context.Context, conversationID, brandID string, details pendingBrief) {
	payload, _ := json.Marshal(details)
	_, _ = d.Pool.Exec(ctx, `
		UPDATE chat_learnings SET answer = $1
		WHERE conversation_id = $2 AND brand_id = $3 AND intent_tag = 'brief_pending'`,
		string(payload), conversationID, brandID)
}

// parseUserDetails: parity — angle/words/audience/goal dari pesan bebas.
func parseUserDetails(message string) pendingBrief {
	lower := strings.ToLower(message)
	var angle *string
	switch {
	case regexp.MustCompile(`informational|edukasi|panduan|guide|tutorial`).MatchString(lower):
		a := "informational"
		angle = &a
	case regexp.MustCompile(`commercial|rekomendasi|review|bandingkan`).MatchString(lower):
		a := "commercial"
		angle = &a
	case regexp.MustCompile(`transactional|beli|daftar|cta|promo`).MatchString(lower):
		a := "transactional"
		angle = &a
	case regexp.MustCompile(`navigational|cari|produk|brand`).MatchString(lower):
		a := "navigational"
		angle = &a
	}
	var words *int
	if m := regexp.MustCompile(`(\d{3,5})\s*(kata|word)`).FindStringSubmatch(message); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			words = &n
		}
	}
	var audience *string
	if m := regexp.MustCompile(`(?i)(?:audience|target|pembaca|untuk)\s*[:\-]?\s*([^,.\n]{3,80})`).FindStringSubmatch(message); m != nil {
		a := strings.TrimSpace(m[1])
		audience = &a
	}
	var goal *string
	if m := regexp.MustCompile(`(?i)(?:goal|tujuan|untuk)\s*[:\-]?\s*([^,.\n]{3,120})`).FindStringSubmatch(message); m != nil {
		g := strings.TrimSpace(m[1])
		goal = &g
	}
	return pendingBrief{Angle: angle, Words: words, Audience: audience, Goal: goal}
}

type chatReadiness struct {
	HasProfile     bool
	HasVoice       bool
	HasRules       bool
	HasPrompt      bool
	KnowledgeCount int
}

func (d *Deps) chatReadiness(ctx context.Context, brandID string) chatReadiness {
	var r chatReadiness
	var has any
	_ = d.Pool.QueryRow(ctx,
		"SELECT (positioning IS NOT NULL OR value_proposition IS NOT NULL OR target_audience IS NOT NULL) FROM brand_profiles WHERE brand_id = $1 LIMIT 1",
		brandID).Scan(&has)
	r.HasProfile = truthyBool(has)
	has = nil
	_ = d.Pool.QueryRow(ctx,
		"SELECT (personality IS NOT NULL OR preferred_terms IS NOT NULL) FROM voice_profiles WHERE brand_id = $1 LIMIT 1",
		brandID).Scan(&has)
	r.HasVoice = truthyBool(has)
	has = nil
	_ = d.Pool.QueryRow(ctx,
		"SELECT (required_rules IS NOT NULL OR forbidden_rules IS NOT NULL OR required_disclaimer IS NOT NULL) FROM editorial_rule_sets WHERE brand_id = $1 LIMIT 1",
		brandID).Scan(&has)
	r.HasRules = truthyBool(has)
	var x int
	r.HasPrompt = d.Pool.QueryRow(ctx,
		"SELECT 1 FROM master_prompt_versions WHERE brand_id = $1 AND status = 'active' LIMIT 1", brandID).Scan(&x) == nil
	if d.Pool.QueryRow(ctx,
		"SELECT 1 FROM knowledge_sources WHERE brand_id = $1 AND deleted_at IS NULL LIMIT 1", brandID).Scan(&x) == nil {
		r.KnowledgeCount = 1
	}
	return r
}

func chatReadinessScore(r chatReadiness) int {
	sum := 0
	for _, ok := range []bool{r.HasProfile, r.HasVoice, r.HasRules, r.HasPrompt} {
		if ok {
			sum += 20
		}
	}
	if r.KnowledgeCount == 1 {
		sum += 20
	}
	return sum
}

type lastArticleInfo struct {
	ID         string
	Title      *string
	Status     string
	CurrentStep string
	Progress   int
	DocumentID *string
}

func (d *Deps) lastArticle(ctx context.Context, brandID string) *lastArticleInfo {
	var info lastArticleInfo
	var briefID string
	err := d.Pool.QueryRow(ctx,
		"SELECT id, status, current_step, progress, brief_id FROM content_jobs WHERE brand_id = $1 ORDER BY created_at DESC LIMIT 1",
		brandID).Scan(&info.ID, &info.Status, &info.CurrentStep, &info.Progress, &briefID)
	if err != nil {
		return nil
	}
	var topic *string
	_ = d.Pool.QueryRow(ctx, "SELECT topic FROM content_briefs WHERE id = $1 LIMIT 1", briefID).Scan(&topic)
	info.Title = topic
	var docID string
	if d.Pool.QueryRow(ctx, "SELECT id FROM content_documents WHERE job_id = $1 LIMIT 1", info.ID).Scan(&docID) == nil {
		info.DocumentID = &docID
	}
	return &info
}

// Chat: POST /v1/brands/{brandId}/chat
func (d *Deps) Chat(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := auth.CheckWriteRate(r.Context(), d.Pool, "write:chat:"+user.ID, d.WriteRateLimitPerMinute); err != nil {
		if _, ok := err.(*auth.AuthRateLimitError); ok {
			httpx.Error(w, http.StatusTooManyRequests, err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var body struct {
		Message        string  `json:"message"`
		ConversationID *string `json:"conversation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Message) < 1 || len(body.Message) > 4000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	conversationID := uuid.NewString()
	if body.ConversationID != nil && *body.ConversationID != "" {
		conversationID = *body.ConversationID
	}
	brandID := brandRow.ID
	lower := strings.ToLower(strings.TrimSpace(body.Message))

	topic := extractTopic(body.Message)
	intent := classifyIntent(body.Message, topic)
	rdy := d.chatReadiness(r.Context(), brandID)
	score := chatReadinessScore(rdy)

	if score < 70 || brandRow.Status != "active" {
		missing := []string{}
		if !rdy.HasProfile {
			missing = append(missing, "Brand Profile")
		}
		if !rdy.HasVoice {
			missing = append(missing, "Voice Profile")
		}
		if !rdy.HasRules {
			missing = append(missing, "Editorial Rules")
		}
		if !rdy.HasPrompt {
			missing = append(missing, "Master Prompt")
		}
		if rdy.KnowledgeCount == 0 {
			missing = append(missing, "Knowledge Base")
		}
		reply := "Mohon maaf, brand **" + brandRow.Name + "** belum siap untuk produksi (skor kesiapan: " + strconv.Itoa(score) + "/100).\n\nKomponen yang belum lengkap:\n"
		for _, m := range missing {
			reply += "- " + m + "\n"
		}
		reply += "\nSilakan lengkapi Brand Setup dan Knowledge Base terlebih dahulu agar saya bisa bekerja optimal."
		actions := []map[string]any{
			chatAction("Lengkapi Brand Setup", "/klien/"+chiParam(r, "brandId")+"/profile", "solid"),
			chatAction("Buka Knowledge Base", "/klien/"+chiParam(r, "brandId")+"/knowledge", "soft"),
		}
		d.storeChatLearning(r.Context(), brandID, conversationID, user.ID, body.Message, reply, "readiness_gate")
		writeJSON(w, http.StatusOK, map[string]any{"reply": reply, "actions": actions, "conversation_id": conversationID})
		return
	}

	// Brief wizard lanjutan.
	if pending := d.checkPendingBrief(r.Context(), conversationID, brandID); pending != nil {
		details := parseUserDetails(body.Message)
		if details.Angle != nil {
			pending.Angle = details.Angle
		}
		if details.Words != nil {
			pending.Words = details.Words
		}
		if details.Audience != nil {
			pending.Audience = details.Audience
		}
		if details.Goal != nil {
			pending.Goal = details.Goal
		}
		d.updatePendingBrief(r.Context(), conversationID, brandID, *pending)
		missing := []string{}
		if pending.Angle == nil {
			missing = append(missing, "angle artikel (informational/commercial/transactional)")
		}
		if pending.Words == nil {
			missing = append(missing, "jumlah kata target")
		}
		if pending.Audience == nil {
			missing = append(missing, "target audience")
		}
		if pending.Goal == nil {
			missing = append(missing, "tujuan artikel")
		}
		if len(missing) > 0 {
			reply := "Baik, detail tersimpan. Masih perlu:\n"
			for _, m := range missing {
				reply += "- " + m + "\n"
			}
			reply += "\nBisa kasih tau sekaligus, misalnya: *\"Informational, 1500 kata, untuk pemilik barbershop, tujuan edukasi perawatan rambut\"*"
			writeJSON(w, http.StatusOK, map[string]any{"reply": reply, "conversation_id": conversationID})
			return
		}

		// Lengkap → buat brief + job + jalan generasi (parity: require outline approval).
		_, _ = d.Pool.Exec(r.Context(),
			"DELETE FROM chat_learnings WHERE conversation_id = $1 AND brand_id = $2 AND intent_tag = 'brief_pending'",
			conversationID, brandID)
		briefID := uuid.NewString()
		jobID := uuid.NewString()
		runtime, err := ai.LoadAIRuntime(r.Context(), d.Pool)
		if err != nil {
			writeErr(w, err)
			return
		}
		model := ai.ResolveModel(runtime, "default")
		words := 1000
		if pending.Words != nil {
			words = *pending.Words
		}
		goal := pending.Topic
		if pending.Goal != nil && *pending.Goal != "" {
			goal = *pending.Goal
		}
		if _, err := d.Pool.Exec(r.Context(), `
			INSERT INTO content_briefs (id, brand_id, content_type, topic, goal, secondary_keywords, required_points,
			  required_content, forbidden_content, target_words, language, require_outline_approval, created_by)
			VALUES ($1, $2, 'article', $3, $4, '[]'::jsonb, '[]'::jsonb, '[]'::jsonb, '[]'::jsonb, $5, 'id-ID', TRUE, $6)`,
			briefID, brandID, pending.Topic, goal, words, user.ID); err != nil {
			writeErr(w, err)
			return
		}
		key := "chat:" + conversationID + ":" + uuid.NewString()[:8]
		configSnapshot, _ := json.Marshal(map[string]any{"provider": runtime.Provider})
		promptSnapshot, _ := json.Marshal(map[string]any{
			"topic": pending.Topic, "angle": pending.Angle, "audience": pending.Audience, "goal": pending.Goal,
		})
		if _, err := d.Pool.Exec(r.Context(), `
			INSERT INTO content_jobs (id, brand_id, brief_id, status, current_step, progress, idempotency_key, model_tier,
			  resolved_model, config_snapshot, prompt_snapshot, source_snapshot, evaluation, created_by)
			VALUES ($1, $2, $3, 'waiting_outline_approval', 'outline_ready', 20, $4, 'default', $5, $6::jsonb, $7::jsonb, '[]'::jsonb, '{}'::jsonb, $8)`,
			jobID, brandID, briefID, key, model, string(configSnapshot), string(promptSnapshot), user.ID); err != nil {
			writeErr(w, err)
			return
		}
		draft := contentengine.RunContentJob(r.Context(), d.Pool, contentengine.RunOptions{
			BrandID: brandID, UserID: user.ID, Topic: pending.Topic, ContentType: "article",
			TargetWords: words, RequireOutlineApproval: true, Tier: "default",
			SearchIntent: pending.Angle, Audience: pending.Audience, Goal: pending.Goal,
		}, runtime)
		outlineJSON, _ := json.Marshal(draft.OutlineStructure)
		_, _ = d.Pool.Exec(r.Context(),
			"UPDATE content_jobs SET status = $1, current_step = $2, progress = $3, outline_structure = $4 WHERE id = $5 AND brand_id = $6",
			draft.Status, draft.CurrentStep, draft.Progress, string(outlineJSON), jobID, brandID)
		d.storeChatLearning(r.Context(), brandID, conversationID, user.ID, body.Message, "Artikel dibuat: "+pending.Topic, "create_article")
		reply := "Kerangka artikel **" + pending.Topic + "** siap (" + strconv.Itoa(words) + " kata, " + derefStr(pending.Angle) + ").\n\nKlik **Setujui & Jalankan Worker** di kartu."
		if derefStr(pending.Angle) == "" {
			reply = "Kerangka artikel **" + pending.Topic + "** siap (" + strconv.Itoa(words) + " kata).\n\nKlik **Setujui & Jalankan Worker** di kartu."
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"reply": reply,
			"actions": []map[string]any{
				chatAction("Lihat History", "/klien/"+chiParam(r, "brandId")+"/generate", "soft"),
			},
			"article": map[string]any{
				"id": jobID, "title": pending.Topic, "status": draft.Status,
				"brand_id": brandID, "document_id": nil,
			},
			"conversation_id": conversationID,
		})
		return
	}

	// Topik terlalu pendek.
	if topic != "" {
		if len([]rune(strings.TrimSpace(topic))) < 3 {
			d.storeChatLearning(r.Context(), brandID, conversationID, user.ID, body.Message, "Topik terlalu pendek.", "general")
			writeJSON(w, http.StatusOK, map[string]any{
				"reply":    "Topik terlalu pendek. Coba tulis lebih jelas, misalnya *\"buat artikel tentang cara merawat rambut\"*.",
				"actions":  []map[string]any{chatAction("Buka halaman Generate", "/klien/"+chiParam(r, "brandId")+"/generate", "soft")},
				"conversation_id": conversationID,
			})
			return
		}
		d.storePendingBrief(r.Context(), conversationID, brandID, user.ID, topic)
		reply := "Oke, mau buat artikel **" + topic + "**.\n\nBiar optimal, kasih tau:\n- **Angle**: informational (edukasi) / commercial (rekomendasi) / transactional (CTA)\n- **Jumlah kata**: berapa?\n- **Audience**: siapa?\n- **Tujuan**: apa?\n\nContoh: *\"Informational, 1500 kata, untuk peminat barbershop, edukasi cara merawat rambut bleaching\"*"
		d.storeChatLearning(r.Context(), brandID, conversationID, user.ID, body.Message, reply, "brief_wizard")
		writeJSON(w, http.StatusOK, map[string]any{"reply": reply, "conversation_id": conversationID})
		return
	}

	// Artikel terakhir.
	for _, marker := range lastArticleMarkers {
		if strings.Contains(lower, marker) {
			last := d.lastArticle(r.Context(), brandID)
			if last == nil {
				writeJSON(w, http.StatusOK, map[string]any{
					"reply":   "Belum ada artikel yang dibuat untuk brand ini. Mau kubuatkan sekarang? Coba tulis *\"buat artikel tentang …\"*.",
					"actions": []map[string]any{chatAction("Buat artikel", "/klien/"+chiParam(r, "brandId")+"/generate", "solid")},
					"conversation_id": conversationID,
				})
				return
			}
			actions := []map[string]any{}
			if last.DocumentID != nil {
				actions = append(actions, chatAction("Buka artikel", "/klien/"+chiParam(r, "brandId")+"/editor/"+*last.DocumentID, "solid"))
			}
			actions = append(actions, chatAction("Lihat History", "/klien/"+chiParam(r, "brandId")+"/generate", "soft"))
			title := "Tanpa judul"
			if last.Title != nil && *last.Title != "" {
				title = *last.Title
			}
			d.storeChatLearning(r.Context(), brandID, conversationID, user.ID, body.Message, "Status artikel terakhir: "+last.Status, "last_article")
			writeJSON(w, http.StatusOK, map[string]any{
				"reply": "Artikel terakhir: **" + title + "**\nStatus: *" + last.Status + "* · langkah: " + last.CurrentStep + " · progress " + strconv.Itoa(last.Progress) + "%.",
				"actions": actions,
				"article": map[string]any{
					"id": last.ID, "title": title, "status": last.Status,
					"brand_id": brandID, "document_id": last.DocumentID,
				},
				"conversation_id": conversationID,
			})
			return
		}
	}

	// Grounding: benchmark + RAG chunks.
	benchmarks := []map[string]any{}
	if intent == "needs_memory" || intent == "needs_knowledge" {
		benchmarks = d.retrieveBenchmarks(r.Context(), brandID, body.Message)
	}
	chunks := []knowledge.ScoredChunk{}
	if intent == "needs_knowledge" || intent == "create_article" {
		chunks, _ = knowledge.RetrieveChunks(r.Context(), d.Pool, brandID, body.Message, 5)
	}
	benchmarkSection := ""
	if len(benchmarks) > 0 {
		lines := []string{}
		for _, b := range benchmarks {
			lines = append(lines, "Q: "+truncRunes(derefStr2(b["question"]), 200)+"\nA: "+truncRunes(derefStr2(b["answer"]), 400))
		}
		benchmarkSection = "PAST BENCHMARKS:\n" + strings.Join(lines, "\n") + "\n"
	}
	knowledgeContext := ""
	if len(chunks) > 0 {
		lines := []string{}
		for _, c := range chunks {
			lines = append(lines, "["+c.SourceTitle+"] "+truncRunes(c.Content, 600))
		}
		knowledgeContext = "KNOWLEDGE (RAG retrieved chunks):\n" + strings.Join(lines, "\n---\n") + "\n"
	}

	// Cache: jawaban lama berskor tinggi → kembalikan tanpa panggil provider.
	for _, b := range benchmarks {
		intentTag := derefStr2(b["intent_tag"])
		sim, _ := b["sim"].(float64)
		if (intentTag == "general" || intentTag == "needs_memory" || intentTag == "needs_knowledge") &&
			derefStr2(b["question"]) != body.Message && sim >= chatLearnSimThresh {
			answer := derefStr2(b["answer"])
			d.storeChatLearning(r.Context(), brandID, conversationID, user.ID, body.Message, answer, intent)
			writeJSON(w, http.StatusOK, map[string]any{
				"reply":     answer,
				"cached":    true,
				"actions":   []map[string]any{chatAction("Buka Brand", "/klien/"+chiParam(r, "brandId"), "soft")},
				"conversation_id": conversationID,
			})
			return
		}
	}

	industry := "-"
	if brandRow.Industry != nil && *brandRow.Industry != "" {
		industry = *brandRow.Industry
	}
	prompt := "You are Tulisio for brand **" + brandRow.Name + "** (industry: " + industry + "). Readiness: " + strconv.Itoa(score) + "/100.\n" +
		"Intent: " + intent + ".\n" + benchmarkSection + knowledgeContext +
		"Answer in Indonesian. Follow brand context. Never invent platform data. Be concise.\n\nUser: " + body.Message + "\nAssistant:"

	result, err := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt: prompt, Tier: "default", TaskType: "brand_chat:" + intent,
		BrandID: &brandID, UserID: &user.ID,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	d.storeChatLearning(r.Context(), brandID, conversationID, user.ID, body.Message, result.Content, intent)
	writeJSON(w, http.StatusOK, map[string]any{
		"reply":     result.Content,
		"cached":    false,
		"actions":   []map[string]any{chatAction("Buka Brand", "/klien/"+chiParam(r, "brandId"), "soft")},
		"conversation_id": conversationID,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func truthyBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	case float64:
		return t != 0
	case string:
		return t == "true" || t == "t"
	default:
		return false
	}
}

func derefStr2(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
