package clienttaut

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
)

// Port lib/taut/agents.ts bagian Matchmaker + Writer.

// matchmakerOutputItem: parity MatchmakerOutputItem.
type matchmakerOutputItem struct {
	MediaID string `json:"media_id"`
	Name    string `json:"name"`
	Score   any    `json:"score"`
	Status  string `json:"status"`
	Reason  any    `json:"reason"`
}

// runMatchmaker: rekomendasi media berbasis AI (tanpa fallback formula).
// LLM WAJIB sukses — gagal/timeout/parse kosong = error, bukan hasil formula.
func (d *Deps) runMatchmaker(ctx context.Context, campaignID, brandID string) (map[string]any, error) {
	campaign, err := d.loadCampaignForAgents(ctx, campaignID, brandID)
	if err != nil {
		return nil, err
	}
	budgetConfig := asMap(jsonField(campaign, "budget_config"))
	budgetStrategy := campaign.strOr("budget_strategy", "")
	var budget *float64
	if budgetStrategy == "efisien" && budgetConfig != nil {
		if v, ok := budgetConfig["nominal"].(float64); ok {
			budget = &v
		}
	}

	briefContext, err := d.loadBriefContext(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	mediaType := campaign.strOr("media_type", "kombinasi")
	if !campaign.has("media_type") {
		mediaType = "kombinasi"
	}
	typeFilter := ""
	if mediaType == "blogger" {
		typeFilter = "AND m.media_type = 'blogger'"
	} else if mediaType == "mainstream" {
		typeFilter = "AND m.media_type = 'mainstream'"
	}
	mediaRows, err := d.queryRows(ctx,
		`SELECT m.id, m.name, m.website_url, m.price, m.credit_cost, m.health_score, m.category, m.media_type,
		        sm.da, sm.pa, m.ai_summary
		 FROM media m
		 LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
		 WHERE m.status = 'active' AND m.integration_type <> 'manual' `+typeFilter)
	if err != nil {
		return nil, err
	}

	candidates := make([]candidateInput, 0, len(mediaRows))
	for _, m := range mediaRows {
		candidates = append(candidates, candidateInput{
			MediaID:     m.strOr("id", ""),
			Name:        m.strOr("name", ""),
			Category:    optionalString(m, "category"),
			Price:       optionalFloat(m, "price"),
			CreditCost:  optionalFloat(m, "credit_cost"),
			HealthScore: optionalFloat(m, "health_score"),
			DA:          optionalFloat(m, "da"),
			PA:          optionalFloat(m, "pa"),
		})
	}
	pool := pickCandidatePool(candidates, budget, 50, 5)
	if len(pool) == 0 {
		return map[string]any{"selected": 0, "media": []matchmakerOutputItem{}}, nil
	}

	// Profil AI per media (Kesimpulan Website) — konteks untuk LLM ranking.
	summaryByID := map[string]string{}
	for _, m := range mediaRows {
		if s := m.strOr("ai_summary", ""); s != "" {
			summaryByID[m.strOr("id", "")] = truncateString(s, 300)
		}
	}
	profileLines := []string{}
	for _, p := range pool {
		if summary, ok := summaryByID[p.MediaID]; ok && summary != "" {
			profileLines = append(profileLines, p.MediaID+": "+summary)
		}
	}

	cfg := d.getAgentConfig(ctx, "matchmaker")
	if !cfg.Configured {
		return nil, &agentNotConfiguredError{agent: "matchmaker"}
	}

	candidateLines := make([]string, 0, len(pool))
	for _, p := range pool {
		candidateLines = append(candidateLines, fmtCandidate(p))
	}
	budgetJSON := "{}"
	if budgetConfig != nil {
		budgetJSON = jsonString(budgetConfig)
	}

	promptLines := []string{
		"Kampanye: " + campaign.strOr("name", ""),
		"Profil: " + defaultJSONObject(campaign, "business_profile"),
		"Target media: " + orDash(campaign.strOr("media_type", "")),
		"Anggaran: " + budgetJSON,
		briefContext,
		"Kandidat (id|nama|kategori|harga|DA|health): " + strings.Join(candidateLines, " ;; "),
		"Profil media (kesimpulan AI per id, boleh kosong): " + orDash(strings.Join(profileLines, " ;; ")),
		"",
		"Keluarkan HANYA JSON valid tanpa kalimat pengantar, tanpa penutup, dan TANPA pagar kode ```: {\"ranked\":[{\"media_id\":\"...\",\"alasan\":\"...\",\"skor\":0-100}]}",
		"Alasan harus bahasa manusiawi — JANGAN sebut kode angle/slug (mis. strategic_partnership_b2b) atau field teknis; gunakan istilah yang dipahami klien.",
	}
	if briefContext != "" {
		promptLines = append(promptLines,
			"Rank media yang RELEVAN dengan brief di atas. Media tidak relevan BOLEH tidak masuk ranked. Beri skor 0-100 (relevansi) dan alasan 1 kalimat yang merujuk brief/metrik. Semua id yang tidak masuk ranked akan otomatis disisihkan.")
	} else {
		promptLines = append(promptLines, "Urutkan kandidat terbaik. Beri skor 0-100 dan alasan 1 kalimat per media.")
	}
	prompt := strings.Join(promptLines, "\n")

	result, err := d.runTautAgent(ctx, "matchmaker", campaignID, prompt, nil, true)
	if err != nil {
		return nil, err
	}
	ranked := parseRanked(result.Content)
	if len(ranked) == 0 {
		slog.Error("[matchmaker] ranking kosong",
			"kandidat", len(pool),
			"panjang_respons", len(result.Content),
			"cuplikan", truncateString(result.Content, 500))
		return nil, errStatus(500, "Matchmaker AI tidak menghasilkan ranking valid. Coba lagi.")
	}

	byID := map[string]candidateLine{}
	for _, p := range pool {
		byID[p.MediaID] = p
	}

	existing, err := d.queryRows(ctx, "SELECT media_id FROM taut_campaign_media WHERE campaign_id = $1", campaignID)
	if err != nil {
		return nil, err
	}
	existingIDs := map[string]bool{}
	for _, e := range existing {
		existingIDs[e.strOr("media_id", "")] = true
	}

	out, plan := planMatchmakerWrites(campaignID, pool, ranked, existingIDs)
	// Tulis semua baris dengan DUA statement (sebelumnya satu UPDATE/INSERT per
	// media): baris yang sudah ada lewat satu UPDATE berbasis jsonb_to_recordset,
	// baris baru lewat satu multi-row INSERT. Cabang existing vs baru tetap
	// memakai snapshot existingIDs yang sama seperti jalur lama.
	if len(plan.updateRows) > 0 {
		if err := d.exec(ctx,
			`UPDATE taut_campaign_media tcm
			 SET match_reason = v.match_reason, status = v.status, ai_score = v.ai_score, match_rank = v.match_rank
			 FROM jsonb_to_recordset($1::jsonb) AS v(media_id text, match_reason text, status text, ai_score int, match_rank int)
			 WHERE tcm.campaign_id = $2 AND tcm.media_id::text = v.media_id`,
			jsonString(plan.updateRows), campaignID); err != nil {
			return nil, err
		}
	}
	if len(plan.insertArgs) > 0 {
		if err := d.exec(ctx, plan.insertSQL, plan.insertArgs...); err != nil {
			return nil, err
		}
	}

	selectedCount := 0
	for _, m := range out {
		if m.Status == "selected" {
			selectedCount++
		}
	}
	return map[string]any{"selected": selectedCount, "media": out}, nil
}

func rankedItemAlasan(item *rankedItem) string {
	if item == nil {
		return ""
	}
	return item.Alasan
}

// matchmakerWritePlan: hasil pemisahan baris pool untuk penulisan batch.
type matchmakerWritePlan struct {
	// updateRows: baris media yang sudah ada → payload UPDATE (jsonb).
	updateRows []map[string]any
	// insertSQL/insertArgs: multi-row INSERT untuk media baru; argumen per
	// baris urut (id baru, campaign_id, media_id, status, reason, score, rank).
	insertSQL  string
	insertArgs []any
}

// planMatchmakerWrites: hitung keluaran matchmaker + rencana penulisannya
// (murni, tanpa DB). Cabang UPDATE vs INSERT memakai snapshot existingIDs,
// sama seperti jalur satu-query-per-media; nilai kolom tidak berubah.
func planMatchmakerWrites(campaignID string, pool []candidateLine, ranked []rankedItem, existingIDs map[string]bool) ([]matchmakerOutputItem, matchmakerWritePlan) {
	out := make([]matchmakerOutputItem, 0, len(pool))
	plan := matchmakerWritePlan{}
	insertValues := []string{}
	insertParam := 1
	for _, item := range pool {
		idx := -1
		for i, r := range ranked {
			if r.MediaID == item.MediaID {
				idx = i
				break
			}
		}
		var rankedItem *rankedItem
		if idx >= 0 {
			rankedItem = &ranked[idx]
		}
		status := "removed"
		var aiScore any
		var rank any
		if rankedItem != nil {
			status = "selected"
			aiScore = rankedItem.Skor
			rank = idx + 1
		}
		reason := humanizeReason(rankedItemAlasan(rankedItem))
		if reason == "" {
			reason = item.Name + ": kurang relevan dengan brief kampanye."
		}

		if existingIDs[item.MediaID] {
			plan.updateRows = append(plan.updateRows, map[string]any{
				"media_id": item.MediaID, "match_reason": reason, "status": status,
				"ai_score": aiScore, "match_rank": rank,
			})
		} else {
			insertValues = append(insertValues, "("+placeholders(insertParam, 7)+")")
			plan.insertArgs = append(plan.insertArgs, newUUID(), campaignID, item.MediaID, status, reason, aiScore, rank)
			insertParam += 7
		}
		out = append(out, matchmakerOutputItem{
			MediaID: item.MediaID, Name: item.Name, Score: aiScore, Status: status, Reason: reason,
		})
	}
	if len(insertValues) > 0 {
		plan.insertSQL = "INSERT INTO taut_campaign_media (id, campaign_id, media_id, source, status, match_reason, ai_score, match_rank) VALUES " + strings.Join(insertValues, ", ")
	}
	return out, plan
}

func optionalString(row rowMap, key string) *string {
	if !row.has(key) {
		return nil
	}
	s := row.strOr(key, "")
	if s == "" {
		return nil
	}
	return &s
}

func optionalFloat(row rowMap, key string) *float64 {
	if !row.has(key) {
		return nil
	}
	v := row.numOr(key)
	return &v
}

func defaultJSONObject(row rowMap, key string) string {
	v := jsonField(row, key)
	if v == nil {
		return "{}"
	}
	return jsonString(v)
}

// loadBriefContext: port loadBriefContext — rangkai konteks brief profiler
// (best-effort, string kosong bila tidak ada).
func (d *Deps) loadBriefContext(ctx context.Context, campaignID string) (string, error) {
	resultRow, err := d.getLatestCompletedResult(ctx, campaignID)
	if err != nil {
		return "", nil
	}
	if resultRow == nil || resultRow.RawExecutiveBriefMap == nil {
		return "", nil
	}
	_, ok := safeParseExecutiveBrief(resultRow.RawExecutiveBriefMap)
	if !ok {
		return "", nil
	}
	brief := resultRow.RawExecutiveBriefMap

	usp := ""
	if draft, err := d.getCurrentDraftSubmission(ctx, campaignID); err == nil && draft != nil {
		if bp, ok := draft.BusinessProfile.(map[string]any); ok {
			if s, ok := bp["usp"].(string); ok {
				usp = s
			}
		}
	}
	base := buildMatchmakerContext(brief, usp)

	rawInstructions := resultRow.WritingInstructions
	if rawInstructions == nil {
		return base, nil
	}
	instructions := ""
	if s, ok := rawInstructions.(string); ok {
		instructions = strings.TrimSpace(s)
	} else {
		instructions = truncateString(jsonString(rawInstructions), 4000)
	}
	if instructions == "" {
		return base, nil
	}
	return base + "\n\n[INSTRUKSI PENULISAN PROFILER]\n" + truncateString(instructions, 4000), nil
}

// loadAnglePromptBlock: prompt angle aktif dari admin (Taut Prompt Studio)
// untuk SATU tipe media. String kosong bila tidak dikonfigurasi.
func (d *Deps) loadAnglePromptBlock(ctx context.Context, campaignID, mediaType string) string {
	slug := d.draftChannelAngleSlug(ctx, campaignID, mediaType)
	if slug == "" {
		return ""
	}
	return d.loadAnglePromptBlockBySlug(ctx, campaignID, mediaType, slug)
}

// draftChannelAngleSlug: slug angle yang tercatat pada channel draft kampanye.
func (d *Deps) draftChannelAngleSlug(ctx context.Context, campaignID, mediaType string) string {
	draft, err := d.getCurrentDraftSubmission(ctx, campaignID)
	if err != nil || draft == nil {
		return ""
	}
	channels, ok := draft.Channels.([]any)
	if !ok {
		return ""
	}
	for _, c := range channels {
		entry, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if ch, _ := entry["channel"].(string); ch == mediaType {
			if s, ok := entry["angle_slug"].(string); ok {
				return strings.TrimSpace(s)
			}
			return ""
		}
	}
	return ""
}

// loadAnglePromptBlockBySlug: blok prompt untuk slug angle tertentu.
func (d *Deps) loadAnglePromptBlockBySlug(ctx context.Context, campaignID, mediaType, slug string) string {
	angle, err := d.getAngleBySlug(ctx, slug)
	if err != nil || angle == nil || !angle.Active {
		return ""
	}
	_, prompt, _, ok := d.getActiveAnglePrompt(ctx, angle.ID)
	prompt = strings.TrimSpace(prompt)
	if !ok || prompt == "" {
		return ""
	}
	return "[PROMPT ANGLE (dari Admin) — " + mediaType + "/" + slug + "]\n" + truncateString(prompt, 12000)
}

// resolveAnglePromptBlock: AI memilih angle paling tepat dari channel sesuai
// jenis media (angle di Taut Prompt Studio dikelompokkan per channel:
// mainstream, blogger, dst.), lalu prompt angle aktifnya menjadi knowledge
// artikel. Fallback: slug statis dari draft kampanye (perilaku lama).
func (d *Deps) resolveAnglePromptBlock(ctx context.Context, campaignID, mediaType, contextSummary string) string {
	if angles, err := d.listAnglesByChannel(ctx, mediaType); err == nil {
		switch {
		case len(angles) == 1:
			if block := d.loadAnglePromptBlockBySlug(ctx, campaignID, mediaType, angles[0].Slug); block != "" {
				return block
			}
		case len(angles) > 1:
			if slug := d.pickAngleForArticle(ctx, mediaType, contextSummary, angles); slug != "" {
				if block := d.loadAnglePromptBlockBySlug(ctx, campaignID, mediaType, slug); block != "" {
					return block
				}
			}
		}
	}
	return d.loadAnglePromptBlock(ctx, campaignID, mediaType)
}

// pickAngleForArticle: LLM memilih SATU angle dari daftar angle aktif pada
// channel jenis media. "" bila gagal — pemanggil fallback ke draft. Panggilan
// hanya terjadi bila channel punya >= 2 angle aktif.
func (d *Deps) pickAngleForArticle(ctx context.Context, mediaType, contextSummary string, angles []angleRow) string {
	var list strings.Builder
	for _, a := range angles {
		list.WriteString("- " + a.Slug + " — " + a.Name)
		if a.Description != "" {
			list.WriteString(": " + a.Description)
		}
		list.WriteString("\n")
	}
	prompt := "Konteks artikel:\n" + truncateString(contextSummary, 900) +
		"\n\nDaftar angle yang tersedia untuk media bertipe \"" + mediaType + "\":" +
		"\n" + list.String() +
		"\nPilih SATU angle yang paling tepat untuk artikel ini pada media bertipe tersebut." +
		"\nJawab HANYA JSON: {\"angle_slug\": \"<slug dari daftar>\"}"
	result, err := d.callAgentLLM(ctx, "matchmaker", prompt, true,
		"Kamu pemilih sudut pandang (angle) artikel. Jawab HANYA JSON tanpa penjelasan apa pun.", 0)
	if err != nil || result == nil {
		return ""
	}
	obj := extractJSONObject(result.Content)
	slug, _ := obj["angle_slug"].(string)
	slug = strings.TrimSpace(slug)
	for _, a := range angles {
		if a.Slug == slug {
			slog.Info("[writer] angle dipilih AI", "media_type", mediaType, "angle_slug", slug, "angle", a.Name)
			return slug
		}
	}
	return ""
}

// channelInstructionFromBrief: instruksi angle channel dari executive_brief.
func channelInstructionFromBrief(brief map[string]any, mediaType string) string {
	channels, ok := brief["channels"].([]any)
	if !ok {
		return ""
	}
	for _, c := range channels {
		ch, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if channel, _ := ch["channel"].(string); channel != mediaType {
			continue
		}
		instruction, _ := ch["writing_instruction"].(string)
		if strings.TrimSpace(instruction) == "" {
			return ""
		}
		angle, _ := ch["angle"].(string)
		return "[INSTRUKSI ANGLE (PROFILER) — " + mediaType + "/" + angle + "]\n" + truncateString(strings.TrimSpace(instruction), 1500)
	}
	return ""
}

var _ = json.Marshal
