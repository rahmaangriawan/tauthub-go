package clienttaut

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
)

// Port app/v1/brands/[brandId]/taut/taut/campaigns/[campaignId]/{profile,profile/*,profiler/options}.

// ---------- profile (GET/PUT/POST) ----------

// TautProfileGet: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile
func (d *Deps) TautProfileGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	c, err := d.queryRow(r.Context(),
		`SELECT id, name, business_profile, hyperlink_config, media_type, trigger_keywords, budget_strategy,
		        budget_config, strategy_brief, profiler_summary, category_input, starts_at,
		        editorial_standard_snapshot, taut_status, spokepersons
		 FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1`, campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if c == nil {
		d.writeErr(w, errStatus(http.StatusInternalServerError, "Kampanye tidak ditemukan."))
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"id":               c.strOr("id", ""),
		"name":             c.strOr("name", ""),
		"business_profile": jsonField(c, "business_profile"),
		"spokepersons":     orEmptyJSONArray(jsonField(c, "spokepersons")),
		"category_input":   jsonField(c, "category_input"),
		"hyperlink_config": jsonField(c, "hyperlink_config"),
		"materials":        jsonField(c, "editorial_standard_snapshot"),
		"profiler_summary": jsonField(c, "profiler_summary"),
		"starts_at":        c.timePtr("starts_at"),
		"taut_status":      c.raw("taut_status"),
		"trigger_keywords": parseTextArray(c.raw("trigger_keywords")),
	})
}

// TautProfilePut: PUT /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile
// (draft autosave partial).
func (d *Deps) TautProfilePut(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	if _, err := d.queryRow(r.Context(),
		"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandID); err != nil {
		d.writeErr(w, err)
		return
	}
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	value, err := parseProfileDraft(body)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	if err := d.exec(r.Context(),
		`UPDATE campaigns SET business_profile = $1::jsonb, category_input = $2::jsonb, editorial_standard_snapshot = $3::jsonb,
		   hyperlink_config = $4::jsonb, trigger_keywords = $5, spokepersons = $6::jsonb,
		   taut_status = CASE WHEN taut_status = 'draft' THEN 'profiling' ELSE taut_status END, updated_at = NOW()
		 WHERE id = $7`,
		jsonString(value.BusinessProfile), jsonString(value.CategoryInput), jsonString(value.MaterialsJSON()),
		jsonString(value.HyperlinkConfig), pqTextArray(value.TriggerKeywords), jsonString(value.Spokepersons), campaignID); err != nil {
		d.writeErr(w, err)
		return
	}

	updated, err := d.queryRow(r.Context(),
		"SELECT taut_status, profiler_summary FROM campaigns WHERE id = $1 LIMIT 1", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	summary := asMap(jsonField(updated, "profiler_summary"))
	currentHash := profilerInputHash(value.BusinessProfile, value.CategoryInput, value.Materials, value.HyperlinkConfig)
	stale := true
	if summary != nil && summary["input_hash"] == currentHash {
		stale = false
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"taut_status":      updated.raw("taut_status"),
		"stale":            stale,
		"profiler_summary": jsonField(updated, "profiler_summary"),
	})
}

// TautProfileAnalyzeLegacy: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile
// (endpoint lama: jalankan runProfilerAgent + RAG).
func (d *Deps) TautProfileAnalyzeLegacy(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	if _, err := d.loadTautCampaignRow(r.Context(), campaignID, brandID); err != nil {
		d.writeErr(w, err)
		return
	}
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	value, err := parseProfileStrict(body)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	hasCategory := len(value.CategoryInput.Selected) > 0 || strings.TrimSpace(value.CategoryInput.Custom) != ""
	if !hasCategory {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kategori wajib diisi (pilih dari daftar atau isi custom).")
		return
	}

	if err := d.exec(r.Context(),
		`UPDATE campaigns SET business_profile = $1::jsonb, category_input = $2::jsonb, editorial_standard_snapshot = $3::jsonb,
		   hyperlink_config = $4::jsonb, trigger_keywords = $5,
		   taut_status = CASE WHEN taut_status = 'draft' THEN 'profiling' ELSE taut_status END, updated_at = NOW()
		 WHERE id = $6`,
		jsonString(value.BusinessProfile), jsonString(value.CategoryInput), jsonString(value.MaterialsJSON()),
		jsonString(value.HyperlinkConfig), pqTextArray(value.TriggerKeywords), campaignID); err != nil {
		d.writeErr(w, err)
		return
	}

	cfg := d.getAgentConfig(r.Context(), "profiler")
	if !cfg.Configured {
		d.writeErr(w, &agentNotConfiguredError{agent: "profiler"})
		return
	}

	inputHash := profilerInputHash(value.BusinessProfile, value.CategoryInput, value.Materials, value.HyperlinkConfig)
	prevRow, err := d.queryRow(r.Context(), "SELECT profiler_summary FROM campaigns WHERE id = $1 LIMIT 1", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	prevSummary := asMap(jsonField(prevRow, "profiler_summary"))
	if prevSummary != nil && prevSummary["input_hash"] == inputHash {
		_ = d.advanceTautStatus(r.Context(), campaignID, "profiled")
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "cached": true, "taut_status": "profiled",
			"profiler_summary": prevSummary, "usage": prevSummary["_usage"], "model": prevSummary["_model"],
		})
		return
	}

	refID := uuid.NewString()
	if err := d.chargeCredit(r.Context(), brandID, refID, 10, "Analisis Profiler Taut Pintar", user.ID); err != nil {
		if ice, ok := err.(*insufficientCreditsError); ok {
			writeJSON(w, http.StatusPaymentRequired, map[string]any{
				"detail":    "Saldo TC tidak cukup. Butuh 10 TC, tersedia " + itoa(ice.Available) + " TC. Silakan Top Up.",
				"code":      "insufficient_credits",
				"needed":    10,
				"available": ice.Available,
			})
			return
		}
		d.writeErr(w, err)
		return
	}

	campaignRow, err := d.queryRow(r.Context(), "SELECT starts_at, name FROM campaigns WHERE id = $1 LIMIT 1", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	startsAt := "-"
	if s := campaignRow.timeStr("starts_at"); s != "" {
		startsAt = s
	}
	campaignName := campaignRow.strOr("name", campaignID)

	userPrompt := strings.Join([]string{
		"Kampanye: " + campaignName,
		"Tanggal mulai: " + startsAt,
		"Kategori (pilih): " + orDash(strings.Join(value.CategoryInput.Selected, ", ")),
		"Kategori (custom): " + orDash(value.CategoryInput.Custom),
		"Profil bisnis: " + jsonString(value.BusinessProfile),
		"Bahan baku brief: " + truncateString(value.Materials.Brief, 3000),
		"Key messages: " + strings.Join(value.Materials.KeyMessages, " | "),
		"Referensi: " + strings.Join(value.Materials.References, ", "),
		"Assets: " + strings.Join(value.Materials.Assets, ", "),
		"Hyperlink: " + jsonString(value.HyperlinkConfig),
		"Trigger: " + strings.Join(value.TriggerKeywords, ", "),
		"",
		"Buat summary komprehensif JSON dengan field: profil (1 baris), target_media (Blogger/Mainstream/kombinasi + alasan), bahan_baku (konfirmasi + catatan), anggaran (saran Efisien/Optimal/Brutal + kisaran). Keluarkan JSON valid saja.",
	}, "\n")

	enrichedPrompt, err := d.enrichWithPastProfiles(r.Context(), brandID, campaignID, userPrompt)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	result, err := d.runProfilerAgent(r.Context(), campaignID, enrichedPrompt)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	var summary map[string]any
	var parsed any
	if json.Unmarshal([]byte(result.Content), &parsed) == nil {
		if obj, ok := parsed.(map[string]any); ok {
			summary = obj
		}
	}
	if summary == nil {
		summary = map[string]any{"raw": result.Content}
	}
	summary["input_hash"] = inputHash
	summary["_model"] = result.Model
	summary["_usage"] = map[string]any{"input": result.UsageIn, "output": result.UsageOut}

	if err := d.exec(r.Context(),
		"UPDATE campaigns SET profiler_summary = $1::jsonb, updated_at = NOW() WHERE id = $2", jsonString(summary), campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.advanceTautStatus(r.Context(), campaignID, "profiled"); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "taut_status": "profiled", "profiler_summary": summary,
		"usage": map[string]any{"input": result.UsageIn, "output": result.UsageOut}, "model": result.Model,
	})
}

// enrichWithPastProfiles: RAG — ranking cosine similarity 3 hasil profiler brand
// sebelumnya yang paling mirip.
func (d *Deps) enrichWithPastProfiles(ctx context.Context, brandID, campaignID, prompt string) (string, error) {
	past, err := d.queryRows(ctx,
		`SELECT r.input, r.output FROM taut_agent_runs r
		 JOIN campaigns c ON c.id = r.campaign_id
		 WHERE r.agent = 'profiler' AND r.status = 'done' AND c.brand_id = $1 AND r.campaign_id <> $2
		 ORDER BY r.created_at DESC LIMIT 20`, brandID, campaignID)
	if err != nil {
		return prompt, nil
	}
	if len(past) == 0 {
		return prompt, nil
	}
	qvec := localEmbedding(prompt)
	type scored struct {
		input  string
		output string
		sim    float64
	}
	list := []scored{}
	for _, r := range past {
		input := tryParseJSONKey(r.strOr("input", ""), "prompt")
		output := tryParseJSONKey(r.strOr("output", ""), "content")
		if input == "" || output == "" {
			continue
		}
		sim := cosineSimilarity(qvec, localEmbedding(input))
		list = append(list, scored{input: truncateString(input, 500), output: truncateString(output, 1000), sim: sim})
	}
	if len(list) == 0 {
		return prompt, nil
	}
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j].sim > list[j-1].sim; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
	limit := 3
	if limit > len(list) {
		limit = len(list)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, "[Contoh "+itoa(i+1)+" (similaritas "+format2(list[i].sim)+")]\nInput: "+list[i].input+"\nOutput: "+list[i].output)
	}
	return prompt + "\n\n---\n\n### Contoh analisis sebelumnya (hasil pembelajaran dari campaign lain)\n" + strings.Join(parts, "\n\n---\n\n"), nil
}

func tryParseJSONKey(raw, key string) string {
	var parsed map[string]any
	if json.Unmarshal([]byte(raw), &parsed) != nil {
		return ""
	}
	s, _ := parsed[key].(string)
	return s
}

func format2(f float64) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(formatFloat(f, 2)), "0"), ".")
}

func formatFloat(f float64, prec int) string {
	return strconvFormat(f, prec)
}

// ---------- profile/draft ----------

// TautProfileDraftGet: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/draft
func (d *Deps) TautProfileDraftGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")

	campaign, err := d.queryRow(r.Context(),
		"SELECT id, name, taut_status, spokepersons, taut_entry_mode FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1",
		campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}

	draft, err := d.getCurrentDraftSubmission(r.Context(), campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	result, err := d.getLatestCompletedResult(r.Context(), campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	wallet, err := d.queryRow(r.Context(), "SELECT available_tc FROM credit_wallets WHERE brand_id = $1 LIMIT 1", brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	available := 0
	if wallet != nil {
		available = wallet.intOr("available_tc", 0)
	}
	tcCfg := d.getTcConfig(r.Context())
	usageRow, err := d.queryRow(r.Context(),
		"SELECT COALESCE(SUM(total_tokens), 0) AS total FROM model_usage_events WHERE user_id = $1 AND success = TRUE", user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	tokensUsed := usageRow.intOr("total", 0)

	latestSubmission, err := d.queryRow(r.Context(),
		"SELECT backlink FROM taut_profiler_submissions WHERE campaign_id = $1 ORDER BY revision DESC LIMIT 1", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	var profilerBacklink any
	if latestSubmission != nil {
		if bl := asMap(jsonField(latestSubmission, "backlink")); bl != nil {
			enabled, _ := bl["enabled"].(bool)
			profilerBacklink = map[string]any{
				"enabled":     enabled,
				"target_url":  bl["target_url"],
				"anchor_text": bl["anchor_text"],
			}
		}
	}

	var draftJSON any
	if draft != nil {
		draftJSON = map[string]any{
			"id": draft.ID, "revision": draft.Revision, "name": campaign.strOr("name", ""),
			"objective": draft.Objective, "business_profile": draft.BusinessProfile,
			"audience": draft.Audience, "spokesperson": draft.Spokesperson,
			"spokepersons": orEmptyJSONArray(draft.Spokepersons),
			"guardrails":   draft.Guardrails, "backlink": draft.Backlink,
			"materials": draft.Materials, "budget": draft.Budget,
			"channels": draft.Channels, "crawl_result": draft.CrawlResult,
			"status": draft.Status,
		}
	}
	var resultJSON any
	if result != nil {
		resultJSON = map[string]any{"id": result.ID, "executive_brief": result.ExecutiveBrief, "created_at": result.CreatedAt}
	}
	var entryMode any
	if campaign.has("taut_entry_mode") {
		entryMode = campaign.raw("taut_entry_mode")
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"campaign": map[string]any{
			"id": campaign.strOr("id", ""), "name": campaign.strOr("name", ""),
			"taut_status": campaign.raw("taut_status"), "spokepersons": jsonField(campaign, "spokepersons"),
		},
		"entry_mode":       entryMode,
		"profilerBacklink": profilerBacklink,
		"draft":            draftJSON,
		"result":           resultJSON,
		"wallet":           map[string]any{"available": available, "rate_idr": tcCfg.RateIDR},
		"tokens_used":      tokensUsed,
	})
}

// TautProfileDraftPut: PUT /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/draft
func (d *Deps) TautProfileDraftPut(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.queryRow(r.Context(),
		"SELECT id, name FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	value, err := parseProfilerDraft(body)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	if strings.TrimSpace(value.Name) != "" && strings.TrimSpace(value.Name) != campaign.strOr("name", "") {
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET name = $1, updated_at = NOW() WHERE id = $2", truncateString(strings.TrimSpace(value.Name), 200), campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
	}

	existing, err := d.getCurrentDraftSubmission(r.Context(), campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	objective := jsonString(orEmptyJSONObject(value.Objective))
	items := []any{
		objective,
		jsonString(value.BusinessProfile),
		jsonString(value.Audience),
		jsonString(value.Spokesperson),
		jsonString(value.Spokepersons),
		jsonString(value.Guardrails),
		jsonString(value.Backlink),
		jsonString(map[string]any{"mode": value.MaterialsMode, "file_ids": value.MaterialsFileIDs, "dynamic": value.MaterialsDynamic}),
		jsonString(value.Budget),
	}
	if existing != nil {
		if err := d.exec(r.Context(),
			`UPDATE taut_profiler_submissions SET
			   objective = $1::jsonb, business_profile = $2::jsonb, audience = $3::jsonb, spokesperson = $4::jsonb,
			   spokepersons = $5::jsonb, guardrails = $6::jsonb, backlink = $7::jsonb, materials = $8::jsonb, budget = $9::jsonb,
			   channels = $10::jsonb, updated_by = $11, updated_at = NOW()
			 WHERE id = $12`,
			items[0], items[1], items[2], items[3], items[4], items[5], items[6], items[7], items[8],
			jsonString(value.Channels), user.ID, existing.ID); err != nil {
			d.writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": existing.ID, "revision": existing.Revision})
		return
	}

	id := uuid.NewString()
	if err := d.exec(r.Context(),
		`INSERT INTO taut_profiler_submissions
		   (id, campaign_id, revision, status, input_schema_version, objective, business_profile, audience,
		    spokesperson, spokepersons, guardrails, backlink, materials, budget, channels, created_by)
		 VALUES ($1, $2, 1, 'draft', 1, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb, $11::jsonb, $12::jsonb, $13)`,
		id, campaignID, items[0], items[1], items[2], items[3], items[4], items[5], items[6], items[7], items[8],
		jsonString(value.Channels), user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "revision": 1})
}

// ---------- profile/result (DELETE) ----------

// TautProfileResultDelete: DELETE /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/result
func (d *Deps) TautProfileResultDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.queryRow(r.Context(),
		"SELECT id, taut_status FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	result, err := d.getLatestCompletedResult(r.Context(), campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if result == nil {
		httpx.Error(w, http.StatusNotFound, "Tidak ada hasil analisis untuk dihapus.")
		return
	}
	if err := d.exec(r.Context(),
		"DELETE FROM taut_profiler_submission_channels WHERE submission_id = $1", result.SubmissionID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM taut_profiler_results WHERE id = $1", result.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE taut_profiler_submissions SET status = 'draft', updated_at = NOW() WHERE id = $1", result.SubmissionID); err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign.strOr("taut_status", "") == "profiled" {
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET taut_status = 'profiling', updated_at = NOW() WHERE id = $1", campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautProfileResultGet: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/results/{resultId}
func (d *Deps) TautProfileResultGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.brandID(r, user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	result, err := d.getResultByID(r.Context(), chiParam(r, "resultId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if result == nil {
		httpx.Error(w, http.StatusNotFound, "Hasil tidak ditemukan.")
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"id":                       result.ID,
		"submission_id":            result.SubmissionID,
		"result_schema_version":    result.ResultSchemaVersion,
		"executive_brief":          result.ExecutiveBrief,
		"writing_instructions":     result.WritingInstructions,
		"strategic_notes":          result.StrategicNotes,
		"compiled_prompt_manifest": result.CompiledPromptManif,
		"model_snapshot":           result.ModelSnapshot,
		"created_at":               result.CreatedAt,
	})
}

// TautProfileVerify: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/verify
func (d *Deps) TautProfileVerify(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	indexRaw, has, err := fieldNum(body, "index")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !has {
		d.writeErr(w, zodInvalidType("number", "undefined"))
		return
	}
	if indexRaw != float64(int(indexRaw)) || indexRaw < 0 || indexRaw > 29 {
		d.writeErr(w, zodTooSmallNumber("0"))
		return
	}
	index := int(indexRaw)
	actionRaw, has, err := fieldStr(body, "action")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !has {
		d.writeErr(w, zodInvalidType("string", "undefined"))
		return
	}
	action, err := validateEnum(actionRaw, "answered", "ignored")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	response := ""
	if raw, ok, err := fieldStr(body, "response"); err != nil {
		d.writeErr(w, err)
		return
	} else if ok {
		if len([]rune(raw)) > 2000 {
			d.writeErr(w, zodTooBigString(2000))
			return
		}
		response = raw
	}

	campaign, err := d.queryRow(r.Context(),
		"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL LIMIT 1", campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	result, err := d.getLatestCompletedResult(r.Context(), campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if result == nil || result.ID == "" || result.RawExecutiveBriefMap == nil {
		httpx.Error(w, http.StatusNotFound, "Hasil Profiler tidak ditemukan.")
		return
	}

	brief := map[string]any{}
	for k, v := range result.RawExecutiveBriefMap {
		brief[k] = v
	}
	questions := []any{}
	if arr, ok := brief["needs_verification"].([]any); ok {
		questions = arr
	}
	if index >= len(questions) {
		httpx.Error(w, http.StatusNotFound, "Item verifikasi tidak ditemukan.")
		return
	}
	state := map[string]any{}
	if raw, ok := brief["verification_state"].(map[string]any); ok {
		for k, v := range raw {
			state[k] = v
		}
	}
	if action == "answered" {
		entry := map[string]any{"status": "answered"}
		if trimmed := strings.TrimSpace(response); trimmed != "" {
			entry["response"] = trimmed
		}
		state[itoa(index)] = entry
	} else {
		state[itoa(index)] = map[string]any{"status": "ignored"}
	}
	brief["verification_state"] = state

	if err := d.exec(r.Context(),
		"UPDATE taut_profiler_results SET executive_brief = $1::jsonb WHERE id = $2", jsonString(brief), result.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "brief": brief})
}

// ---------- profiler/options ----------

// TautProfilerOptions: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/profiler/options
func (d *Deps) TautProfilerOptions(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.brandID(r, user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	channels, err := d.listActiveChannels(r.Context())
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(channels))
	for _, c := range channels {
		angles, err := d.listAnglesByChannel(r.Context(), c.Slug)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		angleItems := make([]map[string]any, 0, len(angles))
		for _, a := range angles {
			angleItems = append(angleItems, map[string]any{
				"id": a.ID, "name": a.Name, "slug": a.Slug, "description": a.Description,
			})
		}
		items = append(items, map[string]any{
			"id": c.ID, "name": c.Name, "slug": c.Slug, "description": c.Description, "angles": angleItems,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": items})
}

// ---------- parsing skema profiler ----------

type profileForm struct {
	BusinessProfile businessProfile
	CategoryInput   struct {
		Selected []string
		Custom   string
	}
	Materials       materialsForm
	HyperlinkConfig []hyperlinkEntry
	TriggerKeywords []string
	Spokepersons    []spokesperson
}

type materialsForm struct {
	Brief       string
	KeyMessages []string
	References  []string
	Assets      []string
}

type hyperlinkEntry struct {
	Anchor string
	URL    string
	Notes  string
}

func (p profileForm) MaterialsJSON() map[string]any {
	return map[string]any{
		"brief": p.Materials.Brief, "key_messages": p.Materials.KeyMessages,
		"references": p.Materials.References, "assets": p.Materials.Assets,
	}
}

func (p profileForm) CategoryJSON() map[string]any {
	return map[string]any{"selected": p.CategoryInput.Selected, "custom": p.CategoryInput.Custom}
}

func (p profileForm) HyperlinkJSON() []map[string]any {
	out := make([]map[string]any, 0, len(p.HyperlinkConfig))
	for _, h := range p.HyperlinkConfig {
		entry := map[string]any{"anchor": h.Anchor, "url": h.URL}
		if h.Notes != "" {
			entry["notes"] = h.Notes
		}
		out = append(out, entry)
	}
	return out
}

// parseProfileStrict: parity profileSchema (POST /profile — validasi ketat).
func parseProfileStrict(body map[string]any) (profileForm, error) {
	var out profileForm
	bp, has, err := fieldMap(body, "business_profile")
	if err != nil {
		return out, err
	}
	if !has {
		return out, zodInvalidType("object", "undefined")
	}
	rawBusiness, has, err := fieldStr(bp, "business")
	if err != nil {
		return out, err
	}
	if !has {
		return out, zodInvalidType("string", "undefined")
	}
	business, err := validateTrimmedString(rawBusiness, 2, 200, "", "")
	if err != nil {
		return out, err
	}
	industry, err := optionalTrimmed(bp, "industry", 120)
	if err != nil {
		return out, err
	}
	audience, err := optionalTrimmed(bp, "audience", 500)
	if err != nil {
		return out, err
	}
	usp, err := optionalTrimmed(bp, "usp", 500)
	if err != nil {
		return out, err
	}
	website, err := optionalTrimmed(bp, "website", 300)
	if err != nil {
		return out, err
	}
	out.BusinessProfile = businessProfile{Business: business, Industry: industry, Audience: audience, USP: usp, Website: website}

	ci, has, err := fieldMap(body, "category_input")
	if err != nil {
		return out, err
	}
	if !has {
		return out, zodInvalidType("object", "undefined")
	}
	if arr, ok, err := fieldArray(ci, "selected"); err != nil {
		return out, err
	} else if ok {
		items, valErr := validateStringArray(arr, 20, 1, 120)
		if valErr != nil {
			return out, valErr
		}
		out.CategoryInput.Selected = items
	} else {
		out.CategoryInput.Selected = []string{}
	}
	if raw, ok, err := fieldStr(ci, "custom"); err != nil {
		return out, err
	} else if ok {
		if len([]rune(raw)) > 120 {
			return out, zodTooBigString(120)
		}
		out.CategoryInput.Custom = strings.TrimSpace(raw)
	}

	materials, has, err := fieldMap(body, "materials")
	if err != nil {
		return out, err
	}
	if !has {
		return out, zodInvalidType("object", "undefined")
	}
	parsedMaterials, err := validateMaterials(materials)
	if err != nil {
		return out, err
	}
	out.Materials = materialsForm{
		Brief:       parsedMaterials["brief"].(string),
		KeyMessages: toStringSlice(parsedMaterials["key_messages"]),
		References:  toStringSlice(parsedMaterials["references"]),
		Assets:      toStringSlice(parsedMaterials["assets"]),
	}

	if arr, ok, err := fieldArray(body, "hyperlink_config"); err != nil {
		return out, err
	} else if ok {
		if len(arr) > 10 {
			return out, zodTooBigArray(10)
		}
		for _, item := range arr {
			obj, isObj := item.(map[string]any)
			if !isObj {
				return out, zodInvalidType("object", receivedType(item))
			}
			rawAnchor, hasAnchor, err := fieldStr(obj, "anchor")
			if err != nil {
				return out, err
			}
			if !hasAnchor {
				return out, zodInvalidType("string", "undefined")
			}
			anchor, err := validateTrimmedString(rawAnchor, 1, 120, "", "")
			if err != nil {
				return out, err
			}
			rawURL, hasURL, err := fieldStr(obj, "url")
			if err != nil {
				return out, err
			}
			if !hasURL {
				return out, zodInvalidType("string", "undefined")
			}
			linkURL := strings.TrimSpace(rawURL)
			if len([]rune(linkURL)) > 500 {
				return out, zodTooBigString(500)
			}
			if !isZodURL(linkURL) {
				return out, zodInvalidURL()
			}
			notes, err := optionalTrimmed(obj, "notes", 300)
			if err != nil {
				return out, err
			}
			out.HyperlinkConfig = append(out.HyperlinkConfig, hyperlinkEntry{Anchor: anchor, URL: linkURL, Notes: notes})
		}
	}

	if arr, ok, err := fieldArray(body, "trigger_keywords"); err != nil {
		return out, err
	} else if ok {
		items, valErr := validateStringArray(arr, 20, 1, 80)
		if valErr != nil {
			return out, valErr
		}
		out.TriggerKeywords = items
	}
	return out, nil
}

// parseProfileDraft: parity draftSchema di profile/route.ts (PUT /profile — boleh parsial).
func parseProfileDraft(body map[string]any) (profileForm, error) {
	var out profileForm
	out.BusinessProfile = businessProfile{}
	out.CategoryInput.Selected = []string{}
	out.Spokepersons = []spokesperson{}
	out.HyperlinkConfig = []hyperlinkEntry{}
	if m, has, err := fieldMap(body, "business_profile"); err != nil {
		return out, err
	} else if has {
		for _, key := range []string{"business", "industry", "audience", "usp", "website"} {
			if raw, ok, err := fieldStr(m, key); err != nil {
				return out, err
			} else if ok {
				limit := 200
				switch key {
				case "industry":
					limit = 120
				case "audience", "usp":
					limit = 500
				case "website":
					limit = 300
				}
				if len([]rune(raw)) > limit {
					return out, zodTooBigString(limit)
				}
				trimmed := strings.TrimSpace(raw)
				switch key {
				case "business":
					out.BusinessProfile.Business = trimmed
				case "industry":
					out.BusinessProfile.Industry = trimmed
				case "audience":
					out.BusinessProfile.Audience = trimmed
				case "usp":
					out.BusinessProfile.USP = trimmed
				case "website":
					out.BusinessProfile.Website = trimmed
				}
			}
		}
	}
	if m, has, err := fieldMap(body, "category_input"); err != nil {
		return out, err
	} else if has {
		if arr, ok, err := fieldArray(m, "selected"); err != nil {
			return out, err
		} else if ok {
			items, valErr := validateStringArray(arr, 20, 1, 120)
			if valErr != nil {
				return out, valErr
			}
			out.CategoryInput.Selected = items
		}
		if raw, ok, err := fieldStr(m, "custom"); err != nil {
			return out, err
		} else if ok {
			if len([]rune(raw)) > 120 {
				return out, zodTooBigString(120)
			}
			out.CategoryInput.Custom = strings.TrimSpace(raw)
		}
	}
	if m, has, err := fieldMap(body, "materials"); err != nil {
		return out, err
	} else if has {
		for _, key := range []string{"brief", "key_messages", "references", "assets"} {
			if key == "brief" {
				if raw, ok, err := fieldStr(m, key); err != nil {
					return out, err
				} else if ok {
					if len([]rune(raw)) > 8000 {
						return out, zodTooBigString(8000)
					}
					out.Materials.Brief = strings.TrimSpace(raw)
				}
				continue
			}
			if arr, ok, err := fieldArray(m, key); err != nil {
				return out, err
			} else if ok {
				limit := 15
				if key == "assets" {
					limit = 10
				}
				items, valErr := validateStringArray(arr, limit, 0, 200)
				if valErr != nil {
					return out, valErr
				}
				switch key {
				case "key_messages":
					out.Materials.KeyMessages = items
				case "references":
					out.Materials.References = items
				case "assets":
					out.Materials.Assets = items
				}
			}
		}
	}
	if arr, ok, err := fieldArray(body, "hyperlink_config"); err != nil {
		return out, err
	} else if ok {
		if len(arr) > 10 {
			return out, zodTooBigArray(10)
		}
		for _, item := range arr {
			obj, isObj := item.(map[string]any)
			if !isObj {
				return out, zodInvalidType("object", receivedType(item))
			}
			entry := hyperlinkEntry{}
			if raw, ok, err := fieldStr(obj, "anchor"); err != nil {
				return out, err
			} else if ok {
				if len([]rune(raw)) > 120 {
					return out, zodTooBigString(120)
				}
				entry.Anchor = strings.TrimSpace(raw)
			}
			if raw, ok, err := fieldStr(obj, "url"); err != nil {
				return out, err
			} else if ok {
				if len([]rune(raw)) > 500 {
					return out, zodTooBigString(500)
				}
				entry.URL = strings.TrimSpace(raw)
			}
			if raw, ok, err := fieldStr(obj, "notes"); err != nil {
				return out, err
			} else if ok {
				if len([]rune(raw)) > 300 {
					return out, zodTooBigString(300)
				}
				entry.Notes = strings.TrimSpace(raw)
			}
			out.HyperlinkConfig = append(out.HyperlinkConfig, entry)
		}
	}
	if arr, ok, err := fieldArray(body, "trigger_keywords"); err != nil {
		return out, err
	} else if ok {
		items, valErr := validateStringArray(arr, 20, 1, 80)
		if valErr != nil {
			return out, valErr
		}
		out.TriggerKeywords = items
	}
	if arr, ok, err := fieldArray(body, "spokepersons"); err != nil {
		return out, err
	} else if ok {
		if len(arr) > 20 {
			return out, zodTooBigArray(20)
		}
		for _, item := range arr {
			obj, isObj := item.(map[string]any)
			if !isObj {
				return out, zodInvalidType("object", receivedType(item))
			}
			sp := spokesperson{}
			for _, key := range []string{"name", "role", "quote"} {
				if raw, ok, err := fieldStr(obj, key); err != nil {
					return out, err
				} else if ok {
					limit := 200
					if key == "quote" {
						limit = 500
					}
					if len([]rune(raw)) > limit {
						return out, zodTooBigString(limit)
					}
					trimmed := strings.TrimSpace(raw)
					switch key {
					case "name":
						sp.Name = trimmed
					case "role":
						sp.Role = trimmed
					case "quote":
						sp.Quote = trimmed
					}
				}
			}
			out.Spokepersons = append(out.Spokepersons, sp)
		}
	}
	return out, nil
}

func optionalTrimmed(m map[string]any, key string, limit int) (string, error) {
	raw, ok, err := fieldStr(m, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	if len([]rune(raw)) > limit {
		return "", zodTooBigString(limit)
	}
	return strings.TrimSpace(raw), nil
}

func toStringSlice(v any) []string {
	arr, ok := v.([]string)
	if !ok {
		return []string{}
	}
	return arr
}

func orEmptyJSONArray(v any) any {
	if v == nil {
		return []any{}
	}
	return v
}

func orEmptyJSONObject(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

var _ = moderation.ModerateContent
var _ = time.Now
