package clienttaut

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port app/v1/brands/[brandId]/taut/campaigns/[campaignId]/profile/analyze/route.ts
// (jalur kanonik Profiler v3: validasi → resolve prompt → preflight saldo →
// freeze submission → LLM → simpan result immutable → advance ke profiled).

const analysisCredit = 10

type profilerAnalysisForm struct {
	Name             string
	Objective        any
	BusinessProfile  businessProfile
	Audience         audienceSelection
	Spokesperson     spokesperson
	Spokepersons     []spokesperson
	Guardrails       guardrails
	Backlink         backlinkStrategy
	Channels         []selectedChannel
	MaterialsMode    string
	MaterialsFileIDs []string
	MaterialsDynamic map[string]any
	Budget           budgetStrategy
}

// TautProfileAnalyze: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/analyze
func (d *Deps) TautProfileAnalyze(w http.ResponseWriter, r *http.Request) {
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
		"SELECT id, name, taut_status FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandID)
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
	value, err := parseProfilerAnalysis(body)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	// Fakta dari file upload ready (extracted_text) sebagai source material.
	fileFacts := []string{}
	if len(value.MaterialsFileIDs) > 0 {
		rows, err := d.queryRows(r.Context(),
			`SELECT original_filename, extracted_text FROM taut_campaign_files
			 WHERE campaign_id = $1 AND id = ANY($2) AND parse_status = 'ready' AND deleted_at IS NULL`,
			campaignID, pqTextArray(value.MaterialsFileIDs))
		if err != nil {
			d.writeErr(w, err)
			return
		}
		for _, f := range rows {
			text := truncateString(f.strOr("extracted_text", ""), 8000)
			if text != "" {
				fileFacts = append(fileFacts, "["+f.strOr("original_filename", "dokumen")+"]\n"+text)
			}
		}
	}

	draftSubmission, err := d.getCurrentDraftSubmission(r.Context(), campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	crawlFacts := crawlResultToFacts(asMap(draftSubmissionCrawl(draftSubmission)))

	// Wajib Media Mainstream: narasumber lengkap (nama, jabatan, kutipan).
	spList := []spokesperson{}
	for _, s := range value.Spokepersons {
		if strings.TrimSpace(s.Name) != "" || strings.TrimSpace(s.Role) != "" || strings.TrimSpace(s.Quote) != "" {
			spList = append(spList, s)
		}
	}
	hasMainstream := false
	for _, c := range value.Channels {
		if c.Channel == "mainstream" {
			hasMainstream = true
		}
	}
	if hasMainstream {
		if len(spList) == 0 {
			httpx.Error(w, http.StatusUnprocessableEntity,
				"Jalur Media Mainstream wajib memiliki minimal satu Juru Bicara/Narasumber (nama, jabatan, kutipan).")
			return
		}
		for i, s := range spList {
			if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Role) == "" || strings.TrimSpace(s.Quote) == "" {
				httpx.Error(w, http.StatusUnprocessableEntity,
					"Narasumber #"+itoa(i+1)+" belum lengkap — isi nama, jabatan, dan kutipan.")
				return
			}
		}
	}
	if value.Budget.Mode == "efficient" && value.Budget.Amount < 5000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Anggaran minimal 5.000 Kredit untuk mode efisien.")
		return
	}
	if value.Budget.Mode == "efficient" && value.Budget.Amount > 100000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Anggaran maksimal 100.000 Kredit untuk mode efisien.")
		return
	}

	objectiveValue := ""
	if s, ok := value.Objective.(string); ok {
		objectiveValue = s
	}
	if objectiveValue == "" {
		objectiveValue = "brand_awareness"
	}

	resolved, err := d.resolveProfilerPrompt(r.Context(), resolveProfilerPromptInput{
		BrandID:         brandID,
		Channels:        value.Channels,
		Objective:       objectiveValue,
		Audience:        value.Audience,
		Spokesperson:    value.Spokesperson,
		Spokepersons:    spList,
		BusinessProfile: value.BusinessProfile,
		Guardrails:      value.Guardrails,
		Backlink:        value.Backlink,
		FileFacts:       fileFacts,
		Dynamic:         value.MaterialsDynamic,
		Budget:          value.Budget,
		CampaignName:    value.Name,
		CrawlFacts:      crawlFacts,
	})
	if err != nil {
		writeAnalyzeError(w, err)
		return
	}

	submissionID := uuid.NewString()
	if draftSubmission != nil {
		submissionID = draftSubmission.ID
	}
	chargeRef := "profiler-" + campaignID + "-" + resolved.InputHash
	if err := d.chargeCredit(r.Context(), brandID, chargeRef, analysisCredit, "Analisis The Profiler", user.ID); err != nil {
		if ice, ok := err.(*insufficientCreditsError); ok {
			writeJSON(w, http.StatusPaymentRequired, map[string]any{
				"detail":    "Kredit Saldo tidak mencukupi untuk menjalankan kampanye ini.",
				"code":      "insufficient_credits",
				"needed":    analysisCredit,
				"available": ice.Available,
			})
			return
		}
		d.writeErr(w, err)
		return
	}

	objectiveJSON := jsonString(orEmptyJSONObject(value.Objective))
	businessJSON := jsonString(map[string]any{
		"business": value.BusinessProfile.Business, "industry": value.BusinessProfile.Industry,
		"audience": value.BusinessProfile.Audience, "usp": value.BusinessProfile.USP,
		"website": value.BusinessProfile.Website,
	})
	audienceJSON := jsonString(map[string]any{"selected": value.Audience.Selected, "custom": value.Audience.Custom})
	spokespersonJSON := jsonString(map[string]any{"name": value.Spokesperson.Name, "role": value.Spokesperson.Role, "quote": value.Spokesperson.Quote})
	spokepersonsJSON := jsonString(spokepersonsToAny(spList))
	guardrailsJSON := jsonString(map[string]any{"rules": value.Guardrails.Rules})
	backlinkJSON := jsonString(map[string]any{
		"enabled": value.Backlink.Enabled, "target_url": value.Backlink.TargetURL,
		"anchor_text": value.Backlink.AnchorText, "smart_variations": value.Backlink.SmartVariations,
	})
	materialsJSON := jsonString(map[string]any{
		"mode": value.MaterialsMode, "file_ids": value.MaterialsFileIDs,
		"dynamic": orEmptyMap(value.MaterialsDynamic),
	})
	budgetJSON := jsonString(map[string]any{"mode": value.Budget.Mode, "amount": value.Budget.Amount})

	err = db.WithPoolTx(r.Context(), d.Pool, func(tx pgxTx) error {
		if draftSubmission != nil {
			if _, err := tx.Exec(r.Context(),
				`UPDATE taut_profiler_submissions
				 SET revision = revision + 1, status = 'analyzing', input_schema_version = $1, objective = $2::jsonb,
				     business_profile = $3::jsonb, audience = $4::jsonb, spokesperson = $5::jsonb, spokepersons = $6::jsonb,
				     guardrails = $7::jsonb, backlink = $8::jsonb, materials = $9::jsonb, budget = $10::jsonb,
				     input_hash = $11, updated_by = $12, updated_at = NOW()
				 WHERE id = $13`,
				profilerInputSchemaVersion, objectiveJSON, businessJSON, audienceJSON, spokespersonJSON, spokepersonsJSON,
				guardrailsJSON, backlinkJSON, materialsJSON, budgetJSON, resolved.InputHash, user.ID, submissionID); err != nil {
				return err
			}
			if _, err := tx.Exec(r.Context(),
				"UPDATE taut_profiler_submissions SET status = 'superseded' WHERE campaign_id = $1 AND status IN ('analyzing','failed') AND id <> $2",
				campaignID, submissionID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO taut_profiler_submissions
				   (id, campaign_id, revision, status, input_schema_version, objective, business_profile, audience,
				    spokesperson, spokepersons, guardrails, backlink, materials, budget, input_hash, created_by)
				 VALUES ($1, $2, 1, 'analyzing', $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb,
				         $9::jsonb, $10::jsonb, $11::jsonb, $12::jsonb, $13, $14)`,
				submissionID, campaignID, profilerInputSchemaVersion, objectiveJSON, businessJSON, audienceJSON,
				spokespersonJSON, spokepersonsJSON, guardrailsJSON, backlinkJSON, materialsJSON, budgetJSON,
				resolved.InputHash, user.ID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(r.Context(),
			"DELETE FROM taut_profiler_submission_channels WHERE submission_id = $1", submissionID); err != nil {
			return err
		}
		blocks, _ := resolved.Manifest["channels"].([]resolvedManifestChannel)
		for _, sel := range value.Channels {
			for _, b := range blocks {
				if b.Channel == sel.Channel && b.Angle == sel.AngleSlug {
					if _, err := tx.Exec(r.Context(),
						"INSERT INTO taut_profiler_submission_channels (id, submission_id, channel_id, content_angle_id, prompt_version_id) VALUES ($1, $2, $3, $4, $5)",
						uuid.NewString(), submissionID, b.ChannelID, b.AngleID, b.PromptVersionID); err != nil {
						return err
					}
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		d.writeErr(w, err)
		return
	}

	runID := uuid.NewString()
	var rawOutput *string
	result, llmErr := d.callAgentLLM(r.Context(), "profiler", resolved.UserPrompt, true, resolved.SystemPrompt, 0)
	if llmErr == nil {
		if obj := extractJSONObject(result.Content); obj != nil {
			encoded := jsonString(obj)
			rawOutput = &encoded
			brief, ok := safeParseExecutiveBrief(obj)
			if ok {
				resultID := uuid.NewString()
				txErr := db.WithPoolTx(r.Context(), d.Pool, func(tx pgxTx) error {
					writingInstructions := []any{}
					if channels, ok := brief["channels"].([]any); ok {
						for _, c := range channels {
							ch, ok := c.(map[string]any)
							if !ok {
								continue
							}
							instruction, _ := ch["writing_instruction"].(string)
							channel, _ := ch["channel"].(string)
							angle, _ := ch["angle"].(string)
							writingInstructions = append(writingInstructions, map[string]any{
								"channel": channel, "angle": angle, "instruction": instruction,
							})
						}
					}
					strategicNotes := brief["strategic_notes"]
					if _, err := tx.Exec(r.Context(),
						`INSERT INTO taut_profiler_results
						   (id, submission_id, agent_run_id, result_schema_version, executive_brief, writing_instructions,
						    strategic_notes, compiled_prompt_manifest, model_snapshot, credit_ledger_entry_id)
						 VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, NULL)`,
						resultID, submissionID, runID, profilerResultSchemaVersion, jsonString(brief),
						jsonString(writingInstructions), jsonString(orEmptyJSONArray(strategicNotes)),
						jsonString(map[string]any{
							"prompt":          resolved.Manifest,
							"compiled_system": truncateString(resolved.SystemPrompt, 2000),
						}),
						jsonString(map[string]any{
							"model": result.Model,
							"usage": map[string]any{"input": result.UsageIn, "output": result.UsageOut},
						})); err != nil {
						return err
					}
					if _, err := tx.Exec(r.Context(),
						"UPDATE taut_profiler_submissions SET status = 'completed', updated_at = NOW() WHERE id = $1", submissionID); err != nil {
						return err
					}
					return nil
				})
				if txErr == nil {
					if err := d.exec(r.Context(),
						"INSERT INTO taut_agent_runs (id, campaign_id, media_id, agent, status, model, token_input, token_output, duration_ms, output, finished_at) VALUES ($1, $2, NULL, 'profiler', 'done', $3, $4, $5, $6, $7::jsonb, NOW())",
						runID, campaignID, result.Model, result.UsageIn, result.UsageOut, result.DurationMS,
						jsonString(map[string]any{"content": truncateString(result.Content, 4000)})); err != nil {
						d.writeErr(w, err)
						return
					}
					if err := d.advanceTautStatus(r.Context(), campaignID, "profiled"); err != nil {
						d.writeErr(w, err)
						return
					}
					writeJSON(w, http.StatusOK, map[string]any{
						"ok": true, "result_id": resultID, "taut_status": "profiled",
						"usage": map[string]any{"input": result.UsageIn, "output": result.UsageOut}, "model": result.Model,
					})
					return
				}
				llmErr = errStatus(http.StatusBadGateway, "Output Profiler tidak valid: "+firstIssueMessage(obj))
			} else {
				llmErr = errStatus(http.StatusBadGateway, "Output Profiler tidak valid: "+firstIssueMessage(obj))
			}
		} else {
			llmErr = errStatus(http.StatusBadGateway, "Output Profiler tidak valid: respons AI tidak mengandung JSON.")
		}
	}

	message := "Analisis gagal."
	if llmErr != nil {
		message = llmErr.Error()
	}
	_ = d.exec(r.Context(), "UPDATE taut_profiler_submissions SET status = 'failed', updated_at = NOW() WHERE id = $1", submissionID)
	var rawArg any
	if rawOutput != nil {
		rawArg = truncateString(*rawOutput, 4000)
	}
	_ = d.exec(r.Context(),
		"INSERT INTO taut_agent_runs (id, campaign_id, media_id, agent, status, output, error, finished_at) VALUES ($1, $2, NULL, 'profiler', 'failed', $3::jsonb, $4, NOW())",
		runID, campaignID, rawArg, truncateString(message, 1000))

	writeAnalyzeError(w, llmErr)
}

// writeAnalyzeError: pemetaan error analyze (409 agent, 422 prompt config, 502 output).
func writeAnalyzeError(w http.ResponseWriter, err error) {
	if err == nil {
		httpx.Error(w, http.StatusInternalServerError, "Analisis gagal.")
		return
	}
	var ance *agentNotConfiguredError
	if asErr(err, &ance) {
		httpx.Error(w, http.StatusConflict, ance.Error())
		return
	}
	message := err.Error()
	if rePromptConfigErr.MatchString(message) {
		httpx.Error(w, http.StatusUnprocessableEntity, message)
		return
	}
	if strings.Contains(message, "Output Profiler tidak valid") {
		httpx.Error(w, http.StatusBadGateway, "Analisis AI mengalami kendala sementara. Draft Anda tetap tersimpan.")
		return
	}
	var te *tautError
	if asErr(err, &te) {
		httpx.Error(w, te.Status, te.Message)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, message)
}

func draftSubmissionCrawl(d *profilerSubmission) any {
	if d == nil {
		return nil
	}
	return d.CrawlResult
}

// crawlResultToFacts: ubah crawl_result jadi fakta berlabel (uncertain dibuang).
func crawlResultToFacts(crawl map[string]any) []crawlFact {
	if crawl == nil {
		return nil
	}
	source := ""
	if s, ok := crawl["url"].(string); ok {
		source = s
	}
	facts := []crawlFact{}
	for _, entry := range crawlFactLabels {
		value := ""
		if s, ok := crawl[entry.Key].(string); ok {
			value = strings.TrimSpace(s)
		}
		if value != "" {
			facts = append(facts, crawlFact{Label: entry.Label, Value: value, Source: source})
		}
	}
	if keywords := strSlice(crawl["keywords"]); len(keywords) > 0 {
		facts = append(facts, crawlFact{Label: "Keyword penting", Value: strings.Join(keywords, ", "), Source: source})
	}
	if extra := strSlice(crawl["facts"]); len(extra) > 0 {
		limit := len(extra)
		if limit > 15 {
			limit = 15
		}
		for _, f := range extra[:limit] {
			facts = append(facts, crawlFact{Label: "Fakta situs", Value: f, Source: source})
		}
	}
	return facts
}

func spokepersonsToAny(list []spokesperson) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, s := range list {
		out = append(out, map[string]any{"name": s.Name, "role": s.Role, "quote": s.Quote})
	}
	return out
}

// parseProfilerAnalysis: port profilerAnalysisSchema (validasi ketat).
func parseProfilerAnalysis(body map[string]any) (profilerAnalysisForm, error) {
	var out profilerAnalysisForm
	draft, err := parseProfilerDraft(body)
	if err != nil {
		return out, err
	}
	out.Name = draft.Name
	out.Objective = draft.Objective
	out.BusinessProfile = draft.BusinessProfile
	out.Audience = draft.Audience
	out.Spokesperson = draft.Spokesperson
	out.Spokepersons = draft.Spokepersons
	out.Guardrails = draft.Guardrails
	out.Backlink = draft.Backlink
	out.Channels = draft.Channels
	out.MaterialsMode, out.MaterialsFileIDs, out.MaterialsDynamic = draft.MaterialsMode, draft.MaterialsFileIDs, draft.MaterialsDynamic
	out.Budget = draft.Budget

	// superRefine (urutan pesan mengikuti skema).
	if strings.TrimSpace(out.Name) == "" {
		return out, errUnprocessable("Nama kampanye wajib diisi.")
	}
	if website := strings.TrimSpace(out.BusinessProfile.Website); website != "" && !isZodURL(website) {
		return out, errUnprocessable("Website harus URL valid.")
	}
	validObjective := false
	if s, ok := out.Objective.(string); ok {
		for _, o := range objectives {
			if o.Value == s {
				validObjective = true
				break
			}
		}
	}
	if !validObjective {
		return out, errUnprocessable("Tujuan kampanye wajib dipilih.")
	}
	if len(out.Channels) == 0 {
		return out, errUnprocessable("Pilih minimal satu channel media.")
	}
	for _, c := range out.Channels {
		if strings.TrimSpace(c.AngleSlug) == "" {
			return out, errUnprocessable("Pilih angle untuk channel " + c.Channel + ".")
		}
	}
	hasMainstream := false
	for _, c := range out.Channels {
		if c.Channel == "mainstream" {
			hasMainstream = true
		}
	}
	completeSpeakers := 0
	for _, s := range out.Spokepersons {
		if strings.TrimSpace(s.Name) != "" && strings.TrimSpace(s.Role) != "" && strings.TrimSpace(s.Quote) != "" {
			completeSpeakers++
		}
	}
	legacySpeaker := strings.TrimSpace(out.Spokesperson.Name) != ""
	if hasMainstream && completeSpeakers == 0 && !legacySpeaker {
		return out, errUnprocessable("Media Mainstream membutuhkan minimal 1 narasumber lengkap (nama, jabatan, kutipan).")
	}
	hasOtherAudience := false
	for _, a := range out.Audience.Selected {
		if a == otherAudience {
			hasOtherAudience = true
		}
	}
	if hasOtherAudience && strings.TrimSpace(out.Audience.Custom) == "" {
		return out, errUnprocessable("Target audiens lain wajib diisi.")
	}
	if out.Backlink.Enabled {
		if strings.TrimSpace(out.Backlink.TargetURL) == "" {
			return out, errUnprocessable("URL target backlink wajib diisi.")
		}
		if strings.TrimSpace(out.Backlink.AnchorText) == "" {
			return out, errUnprocessable("Anchor text backlink wajib diisi.")
		}
	}
	if out.MaterialsMode == "upload" && len(out.MaterialsFileIDs) == 0 {
		return out, errUnprocessable("Upload minimal satu dokumen bahan referensi.")
	}
	if out.Budget.Mode == "efficient" && out.Budget.Amount < 1 {
		return out, errUnprocessable("Nominal Kredit Saldo wajib diisi pada Mode Efisien.")
	}
	return out, nil
}

// localEmbedding / cosineSimilarity: parity lib/knowledge-base/embed.ts.
func localEmbedding(text string) []float64 { return ai.LocalEmbedding(text) }

func cosineSimilarity(a, b []float64) float64 { return ai.CosineSimilarity(a, b) }

var _ = json.Marshal
