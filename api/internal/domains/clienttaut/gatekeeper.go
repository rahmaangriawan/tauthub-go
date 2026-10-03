package clienttaut

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
)

// Port lib/taut/agents.ts bagian Gatekeeper + lib/gatekeeper/audit-repo.ts.

type gatekeeperItemResult struct {
	MediaID string `json:"media_id"`
	Status  string `json:"status"`
	Verdict string `json:"verdict,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// runGatekeeper: audit editorial + tanam hyperlink + safety pre-check.
// HARD STOP bila gatekeeper belum dikonfigurasi.
func (d *Deps) runGatekeeper(ctx context.Context, campaignID, brandID, userID string) ([]gatekeeperItemResult, error) {
	campaign, err := d.loadCampaignForAgents(ctx, campaignID, brandID)
	if err != nil {
		return nil, err
	}
	hyperlink, _ := jsonField(campaign, "hyperlink_config").([]any)
	briefContext, err := d.loadBriefContext(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	items, err := d.queryRows(ctx,
		`SELECT tcm.id, tcm.media_id, tcm.article_document_id, tcm.article_status, m.media_type, m.bahasa_artikel, m.word_count_mode
		 FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id
		 WHERE tcm.campaign_id = $1 AND tcm.status = 'selected' AND tcm.article_document_id IS NOT NULL
		       AND tcm.article_status IN ('draft','revision')`, campaignID)
	if err != nil {
		return nil, err
	}

	results := []gatekeeperItemResult{}
	for _, item := range items {
		itemID := item.strOr("id", "")
		mediaID := item.strOr("media_id", "")
		documentID := item.strOr("article_document_id", "")

		doc, err := d.queryRow(ctx, "SELECT title, current_version FROM content_documents WHERE id = $1 LIMIT 1", documentID)
		if err != nil {
			return nil, err
		}
		title := ""
		currentVersion := 1
		if doc != nil {
			title = doc.strOr("title", "")
			currentVersion = doc.intOr("current_version", 1)
		}
		ver, err := d.queryRow(ctx,
			"SELECT content, attribution FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1", documentID, currentVersion)
		if err != nil {
			return nil, err
		}
		content := ""
		attribution := ""
		if ver != nil {
			content = ver.strOr("content", "")
			attribution = ver.strOr("attribution", "")
		}

		// Hard gate moderation (deterministik + AI classifier existing).
		mod := moderation.ModerateContent(ctx, d.Pool, moderation.Input{
			Title:   strPtrOrNilPtr(title),
			Content: strPtrOrNilPtr(content),
			BrandID: &brandID,
			UserID:  &userID,
		})
		if mod.Status == "blocked" {
			if err := d.exec(ctx, "UPDATE taut_campaign_media SET article_status = 'blocked' WHERE id = $1", itemID); err != nil {
				return nil, err
			}
			moderation.RecordAudit(ctx, d.Pool, "document", documentID, mod, &brandID)
			results = append(results, gatekeeperItemResult{
				MediaID: mediaID, Status: "blocked", Verdict: "gagal",
				Reason: "Diblokir moderasi (" + mod.Category + ")",
			})
			continue
		}

		// 4 pengecek Gatekeeper (sequensial; error per cek tidak menggagalkan cek lain).
		text := extractText(content)
		bahasaList := jsonStringList(item.strOr("bahasa_artikel", "[]"))
		bahasaUtama := "Indonesia"
		if len(bahasaList) > 0 && bahasaList[0] != "" {
			bahasaUtama = bahasaList[0]
		}
		voiceRange := resolveWordRange(item.strPtr("word_count_mode"))

		mediaType := item.strOr("media_type", "")
		if mediaType == "" {
			mediaType = campaign.strOr("media_type", "")
		}
		// Tiga cek independen (AI Overview, safety, link health) paralel.
		var aiOverview map[string]any
		var safety contentSafetyResult
		var links linkHealthResult
		var checksWG sync.WaitGroup
		checksWG.Add(3)
		go func() {
			defer checksWG.Done()
			aiOverview = d.runAIOverviewCheck(ctx, title, mediaType, content, text)
		}()
		go func() {
			defer checksWG.Done()
			safety = d.contentSafetyGuard(ctx, text, extractLinks(content), briefContext)
		}()
		go func() {
			defer checksWG.Done()
			links = d.linkHealthValidator(ctx, content, []string{
				strings.TrimSpace(campaign.strOr("primary_url", "")),
				strings.TrimSpace(campaign.strOr("backup_url", "")),
			})
		}()
		checksWG.Wait()
		minWords, maxWords := 0, 0
		if voiceRange != nil {
			minWords, maxWords = *voiceRange.min, *voiceRange.max
		}
		voice := d.brandVoiceLimits(ctx, content, bahasaUtama, minWords, maxWords)

		safetyStatus := "review_required"
		switch safety.RiskLevel {
		case "high":
			safetyStatus = "failed"
		case "medium":
			safetyStatus = "review_required"
		case "low":
			safetyStatus = "passed"
		}
		linkStatus := links.Verdict
		voiceLevel := voice.Level
		voiceStatus := "passed" // voice informasional: tidak pernah gagal

		checks := map[string]any{}
		checks["aiOverview"] = aiOverview
		checks["contentSafety"] = map[string]any{
			"status": safetyStatus, "riskLevel": safety.RiskLevel,
			"categories": safety.Categories, "confidence": safety.Confidence,
			"reasons": capStrings(safety.Reasons, 3),
		}
		checks["linkHealth"] = map[string]any{"status": linkStatus, "links": links.Links}
		checks["brandVoice"] = map[string]any{
			"status": voiceStatus, "level": voiceLevel, "notes": capStrings(voice.Notes, 3),
		}

		overallVerdict := "passed"
		if safetyStatus == "failed" || linkStatus == "failed" {
			overallVerdict = "failed"
		} else if safetyStatus == "review_required" || linkStatus == "review_required" {
			overallVerdict = "review_required"
		}

		latestAudit, err := d.getLatestGatekeeperAudit(ctx, campaignID, mediaID)
		if err != nil {
			return nil, err
		}
		runRevision := 1
		if latestAudit != nil {
			runRevision = latestAudit.RunRevision + 1
		}
		if err := d.saveGatekeeperAudit(ctx, saveGatekeeperAuditInput{
			CampaignID: campaignID, MediaID: mediaID, RunRevision: runRevision,
			Status: overallVerdict, Checks: checks, OverallVerdict: overallVerdict, UserID: userID,
		}); err != nil {
			return nil, err
		}

		name := d.mediaName(ctx, mediaID)
		allowLink := d.mediaAllowsHyperlink(ctx, mediaID)
		linkURL := ""
		if len(hyperlink) > 0 {
			if first, ok := hyperlink[0].(map[string]any); ok {
				linkURL, _ = first["url"].(string)
			}
		}
		linkRule := "Media TIDAK mengizinkan hyperlink. JANGAN menanam link apa pun (config: " + jsonString(orEmptySlice(hyperlink)) + ")."
		if allowLink {
			linkRule = "Media mengizinkan hyperlink. Tanam hyperlink_config secara natural bila sesuai: " + jsonString(orEmptySlice(hyperlink)) + "."
		}
		promptLines := []string{
			"Kampanye: " + campaign.strOr("name", ""),
			"Media tujuan: " + name,
			linkRule,
			"Periksa judi/SARA/ujaran kebencian.",
			"Judul saat ini: " + title,
			"Isi (HTML): " + truncateString(content, 4000),
		}
		// Catatan AI Overview disambungkan ke pass revisi (bila indikator lemah).
		if notes := ai.AIOverviewRevisionNotes(aiOverview); len(notes) > 0 {
			promptLines = append(promptLines,
				"Catatan revisi AI Overview (WAJIB diperbaiki saat menulis ulang artikel):",
				"- "+strings.Join(notes, "\n- "))
		}
		promptLines = append(promptLines, "", "Output JSON valid {final_html, verdict: layak_tayang|revisi|gagal, catatan}.")
		prompt := strings.Join(promptLines, "\n")

		result, err := d.runTautAgent(ctx, "gatekeeper", campaignID, prompt, &mediaID, true)
		if err != nil {
			return nil, err
		}
		finalHTML := content
		verdict := "layak_tayang"
		var parsed struct {
			FinalHTML string `json:"final_html"`
			Verdict   string `json:"verdict"`
		}
		if json.Unmarshal([]byte(result.Content), &parsed) == nil {
			if parsed.FinalHTML != "" {
				finalHTML = truncateString(parsed.FinalHTML, 200000)
			}
			if parsed.Verdict != "" {
				verdict = parsed.Verdict
			}
		}

		// Deterministik: media tidak izinkan → pastikan tak ada link tersisa.
		if !allowLink {
			finalHTML = stripHyperlinks(finalHTML, linkURL)
		}

		// Artikel buatan AI Tauthub: bila skor di bawah ambang dan pass revisi
		// menulis ulang artikel, nilai ULANG sekali — skor tersimpan adalah
		// kualitas akhir (hijau bila revisi memperbaiki indikator lemah).
		if (attribution == "ai_generated" || attribution == "ai_edited") && ai.AIOverviewRejected(aiOverview) && finalHTML != content {
			rechecked := d.runAIOverviewCheck(ctx, title, mediaType, finalHTML, extractText(finalHTML))
			if rechecked != nil {
				aiOverview = rechecked
				checks["aiOverview"] = rechecked
				if err := d.updateGatekeeperAuditChecks(ctx, campaignID, mediaID, runRevision, checks); err != nil {
					return nil, err
				}
			}
		}

		nextVersion := currentVersion + 1
		if err := d.exec(ctx,
			"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, $3, $4, 'html', 'ai_edited', 'Taut Pintar Gatekeeper', $5)",
			uuid.NewString(), documentID, nextVersion, finalHTML, userID); err != nil {
			return nil, err
		}
		if err := d.exec(ctx,
			"UPDATE content_documents SET current_version = $1, updated_at = NOW() WHERE id = $2", nextVersion, documentID); err != nil {
			return nil, err
		}

		// ck_taut_cm_article_status tidak mengenal 'gagal' → dipetakan ke 'blocked'.
		status := "blocked"
		switch verdict {
		case "layak_tayang":
			status = "ready"
		case "revisi":
			status = "revision"
		}
		if err := d.exec(ctx, "UPDATE taut_campaign_media SET article_status = $1 WHERE id = $2", status, itemID); err != nil {
			return nil, err
		}
		results = append(results, gatekeeperItemResult{MediaID: mediaID, Status: status, Verdict: verdict})
	}
	return results, nil
}

// aiOverviewSettings: toggle + ambang dari taut_agent_configs.settings.aiOverview.
func (d *Deps) aiOverviewSettings(ctx context.Context) ai.AIOverviewSettings {
	return ai.AIOverviewSettingsFromJSON(d.getAgentConfig(ctx, "gatekeeper").Settings["aiOverview"])
}

// runAIOverviewCheck: nilai kesiapan AI Overview (non-blocking). Bagian murni
// (prompt + parsing + aturan gate) ada di internal/ai, sama dengan brandflow.
func (d *Deps) runAIOverviewCheck(ctx context.Context, title, mediaType, contentHTML, text string) map[string]any {
	checkedAt := nowISO()
	st := d.aiOverviewSettings(ctx)
	if !st.Enabled {
		return ai.AIOverviewResult{
			Status: "pending", CheckedAt: checkedAt,
			Info: "Pemeriksaan AI Overview dinonaktifkan admin.",
		}.JSON()
	}
	prompt := ai.AIOverviewUserPrompt(title, mediaType, contentHTML, countWords(text), st)
	result, err := d.callAgentLLM(ctx, "gatekeeper", prompt, true, ai.AIOverviewSystemPrompt, 0)
	if err != nil {
		return ai.AIOverviewResult{
			Status: "review_required", CheckedAt: checkedAt,
			Error: "AI Overview tidak dapat menilai artikel: " + truncateString(err.Error(), 200),
		}.JSON()
	}
	return ai.EvaluateAIOverview(result.Content, st, checkedAt).JSON()
}

// updateGatekeeperAuditChecks: perbarui checks baris audit yang baru dibuat
// (dipakai setelah penilaian ulang AI Overview pasca-revisi).
func (d *Deps) updateGatekeeperAuditChecks(ctx context.Context, campaignID, mediaID string, runRevision int, checks map[string]any) error {
	return d.exec(ctx,
		`UPDATE taut_gatekeeper_audits SET checks = $1::jsonb, updated_at = NOW()
		 WHERE campaign_id = $2 AND media_id = $3 AND run_revision = $4`,
		jsonString(checks), campaignID, mediaID, runRevision)
}

func strPtrOrNilPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// mediaAllowsHyperlink: policy hyperlink media — max_links (0 = tidak boleh),
// allow_hyperlink hanya fallback baris legacy.
func (d *Deps) mediaAllowsHyperlink(ctx context.Context, mediaID string) bool {
	row, err := d.queryRow(ctx, "SELECT allow_hyperlink, max_links FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil || row == nil {
		return false
	}
	if row.has("max_links") {
		return int(row.numOr("max_links")) > 0
	}
	return row.boolOr("allow_hyperlink", false)
}

// ---------- audit repo (port lib/gatekeeper/audit-repo.ts) ----------

type gatekeeperAudit struct {
	ID             string
	CampaignID     string
	MediaID        string
	AgentRunID     any
	RunRevision    int
	Status         string
	Checks         map[string]any
	OverallVerdict string
	CreatedAt      string
	UpdatedAt      string
}

type saveGatekeeperAuditInput struct {
	CampaignID     string
	MediaID        string
	AgentRunID     *string
	RunRevision    int
	Status         string
	Checks         map[string]any
	OverallVerdict string
	UserID         string
}

const gatekeeperAuditCols = "id, campaign_id, media_id, agent_run_id, run_revision, status, checks, overall_verdict, created_at, updated_at"

func mapGatekeeperAudit(m rowMap) gatekeeperAudit {
	checks, _ := jsonField(m, "checks").(map[string]any)
	if checks == nil {
		checks = map[string]any{}
	}
	return gatekeeperAudit{
		ID: m.strOr("id", ""), CampaignID: m.strOr("campaign_id", ""), MediaID: m.strOr("media_id", ""),
		AgentRunID: m.raw("agent_run_id"), RunRevision: m.intOr("run_revision", 1),
		Status: m.strOr("status", ""), Checks: checks, OverallVerdict: m.strOr("overall_verdict", ""),
		CreatedAt: m.timeStr("created_at"), UpdatedAt: m.timeStr("updated_at"),
	}
}

// saveGatekeeperAudit: INSERT dengan id baru setiap run (revisi naik per audit ulang).
func (d *Deps) saveGatekeeperAudit(ctx context.Context, in saveGatekeeperAuditInput) error {
	return d.exec(ctx,
		`INSERT INTO taut_gatekeeper_audits (id, campaign_id, media_id, agent_run_id, run_revision, status, checks, overall_verdict, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)
		 ON CONFLICT (id) DO UPDATE SET
		   status = EXCLUDED.status,
		   checks = EXCLUDED.checks,
		   overall_verdict = EXCLUDED.overall_verdict,
		   updated_at = NOW()`,
		uuid.NewString(), in.CampaignID, in.MediaID, in.AgentRunID, in.RunRevision,
		in.Status, jsonString(in.Checks), in.OverallVerdict, strPtrOrNil(in.UserID))
}

// getLatestGatekeeperAudit: audit terbaru per (campaign, media).
func (d *Deps) getLatestGatekeeperAudit(ctx context.Context, campaignID, mediaID string) (*gatekeeperAudit, error) {
	m, err := d.queryRow(ctx,
		"SELECT "+gatekeeperAuditCols+" FROM taut_gatekeeper_audits WHERE campaign_id = $1 AND media_id = $2 ORDER BY created_at DESC LIMIT 1",
		campaignID, mediaID)
	if err != nil || m == nil {
		return nil, err
	}
	a := mapGatekeeperAudit(m)
	return &a, nil
}

// listGatekeeperAudits: semua audit campaign (created_at DESC).
func (d *Deps) listGatekeeperAudits(ctx context.Context, campaignID string) ([]gatekeeperAudit, error) {
	rows, err := d.queryRows(ctx,
		"SELECT "+gatekeeperAuditCols+" FROM taut_gatekeeper_audits WHERE campaign_id = $1 ORDER BY created_at DESC", campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]gatekeeperAudit, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapGatekeeperAudit(r))
	}
	return out, nil
}
