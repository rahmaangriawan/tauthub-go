package brandflow

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// taut_gatekeeper.go: port lib/taut/agents.ts bagian Gatekeeper +
// lib/gatekeeper/{content-safety,link-health,voice-check}.ts + audit-repo.
// Port ringkas: hard gate moderasi deterministik, content safety AI, link health
// (probe + klasifikasi AI), brand voice deterministik.

// gatekeeperItemResult: parity GatekeeperItemResult.
type gatekeeperItemResult struct {
	MediaID string `json:"media_id"`
	Slot    int    `json:"slot"`
	Status  string `json:"status"`
	Verdict string `json:"verdict,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type gatekeeperAuditRow struct {
	ID             string
	CampaignID     string
	MediaID        string
	Slot           int
	AgentRunID     any
	RunRevision    int
	Status         string
	Checks         map[string]any
	OverallVerdict string
	CreatedAt      string
	UpdatedAt      string
}

const gatekeeperAuditCols = "id, campaign_id, media_id, slot, agent_run_id, run_revision, status, checks, overall_verdict, created_at, updated_at"

func mapGatekeeperAudit(row bsRow) gatekeeperAuditRow {
	checks, _ := row.jsonField("checks").(map[string]any)
	if checks == nil {
		checks = map[string]any{}
	}
	return gatekeeperAuditRow{
		ID: row.strOr("id", ""), CampaignID: row.strOr("campaign_id", ""), MediaID: row.strOr("media_id", ""),
		Slot:       row.intOr("slot", 1),
		AgentRunID: row.raw("agent_run_id"), RunRevision: row.intOr("run_revision", 1),
		Status: row.strOr("status", ""), Checks: checks, OverallVerdict: row.strOr("overall_verdict", ""),
		CreatedAt: row.timeStr("created_at"), UpdatedAt: row.timeStr("updated_at"),
	}
}

// listGatekeeperAudits: semua audit kampanye (created_at DESC).
func (d *Deps) listGatekeeperAudits(ctx context.Context, campaignID string) ([]gatekeeperAuditRow, error) {
	rows, err := d.queryRows(ctx,
		"SELECT "+gatekeeperAuditCols+" FROM taut_gatekeeper_audits WHERE campaign_id = $1 ORDER BY created_at DESC", campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]gatekeeperAuditRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapGatekeeperAudit(r))
	}
	return out, nil
}

// getLatestGatekeeperAudit: audit terbaru per (campaign, media, slot).
func (d *Deps) getLatestGatekeeperAudit(ctx context.Context, campaignID, mediaID string, slot int) (*gatekeeperAuditRow, error) {
	row, err := d.queryRow(ctx,
		"SELECT "+gatekeeperAuditCols+" FROM taut_gatekeeper_audits WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 ORDER BY created_at DESC LIMIT 1",
		campaignID, mediaID, slot)
	if err != nil || row == nil {
		return nil, err
	}
	a := mapGatekeeperAudit(row)
	return &a, nil
}

// saveGatekeeperAudit: INSERT id baru setiap run (revisi naik per audit ulang).
func (d *Deps) saveGatekeeperAudit(ctx context.Context, campaignID, mediaID string, slot, runRevision int, checks map[string]any, overallVerdict, userID string) error {
	return d.exec(ctx,
		`INSERT INTO taut_gatekeeper_audits (id, campaign_id, media_id, slot, agent_run_id, run_revision, status, checks, overall_verdict, created_by)
		 VALUES ($1, $2, $3, $4, NULL, $5, $6, $7::jsonb, $8, $9)`,
		uuid.NewString(), campaignID, mediaID, slot, runRevision, overallVerdict,
		marshalJSON(checks), overallVerdict, nullIfEmpty(userID))
}

// runGatekeeper: audit editorial + tanam hyperlink deterministik + safety
// pre-check. HARD STOP bila gatekeeper belum dikonfigurasi (409).
// Paralel 4 pekerja: tiap media butuh 3-4 panggilan LLM (safety, klasifikasi
// link, AI Overview, pass akhir) — serial membuat scan banyak media melewati
// timeout klien. Tiap media independen (baris artikel & auditnya sendiri).
// onlyMediaIDs (opsional): batasi + paksa pindai media tertentu meski
// article_status-nya sudah 'ready' — dipakai tombol "Scan QC Sekarang"/"Cek
// Validasi" per artikel supaya cek baru (mis. AI Overview) benar-benar jalan.
func (d *Deps) runGatekeeper(ctx context.Context, campaignID, brandID, userID string, onlyMediaIDs []string, onlySlots []int) ([]gatekeeperItemResult, error) {
	if !d.getAgentConfig(ctx, "gatekeeper").Configured {
		return nil, &agentNotConfiguredError{agent: "gatekeeper"}
	}
	campaign, err := d.loadTautCampaignRow(ctx, campaignID, brandID)
	if err != nil {
		return nil, err
	}

	itemsQuery := `SELECT tcm.id, tcm.media_id, tcm.slot, tcm.article_document_id, tcm.article_status, tcm.angle_slug, m.media_type, m.bahasa_artikel, m.word_count_mode, m.website_url, m.allow_hyperlink, m.max_links
		 FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id
		 WHERE tcm.campaign_id = $1 AND tcm.status = 'selected' AND tcm.article_document_id IS NOT NULL`
	queryArgs := []any{campaignID}
	if len(onlyMediaIDs) > 0 {
		// Permintaan eksplisit: sertakan artikel 'ready' (pindai ulang atas
		// permintaan), tetap kecualikan 'blocked'.
		itemsQuery += ` AND tcm.article_status IN ('draft','revision','ready') AND tcm.media_id = ANY($2)`
		queryArgs = append(queryArgs, onlyMediaIDs)
		if len(onlySlots) > 0 {
			// Satu kartu artikel = satu slot. Tanpa filter ini, memindai artikel 1
			// ikut memindai artikel 2..4 di domain yang sama (biaya AI berlipat).
			itemsQuery += ` AND tcm.slot = ANY($3)`
			queryArgs = append(queryArgs, onlySlots)
		}
	} else {
		itemsQuery += ` AND tcm.article_status IN ('draft','revision')`
	}
	items, err := d.queryRows(ctx, itemsQuery, queryArgs...)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []gatekeeperItemResult{}, nil
	}

	st := d.aiOverviewSettings(ctx)
	results := make([]gatekeeperItemResult, len(items))
	errs := make([]error, len(items))
	workers := len(items)
	if workers > 4 {
		workers = 4
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, item := range items {
		wg.Add(1)
		go func(i int, item bsRow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], errs[i] = d.gatekeeperAuditOne(ctx, campaign, campaignID, brandID, userID, item, st)
		}(i, item)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}
	return results, nil
}

// gatekeeperAuditOne: satu putaran audit untuk satu media (dipanggil paralel).
func (d *Deps) gatekeeperAuditOne(ctx context.Context, campaign bsRow, campaignID, brandID, userID string, item bsRow, aiOverviewSet ai.AIOverviewSettings) (gatekeeperItemResult, error) {
	startedAt := time.Now()
	itemID := item.strOr("id", "")
	mediaID := item.strOr("media_id", "")
	slot := item.intOr("slot", 1)
	if slot < 1 {
		slot = 1
	}
	documentID := item.strOr("article_document_id", "")

	// Bahan fakta kanonik yang SAMA dengan yang dipakai Writer (brief Profiler +
	// materials klien + topik klien + narasumber). Tanpa ini, Checker menilai
	// klaim bersumber sebagai "tanpa dasar" dan skor entitas/E-E-A-T jatuh.
	refMaterial, _, err := d.tautReferenceMaterial(ctx, campaign, campaignID, mediaID, slot)
	if err != nil {
		return gatekeeperItemResult{}, err
	}

	doc, err := d.queryRow(ctx, "SELECT title, current_version FROM content_documents WHERE id = $1 LIMIT 1", documentID)
	if err != nil {
		return gatekeeperItemResult{}, err
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
		return gatekeeperItemResult{}, err
	}
	content := ""
	attribution := ""
	if ver != nil {
		content = ver.strOr("content", "")
		attribution = ver.strOr("attribution", "")
	}

	// Hard gate moderasi (deterministik + AI classifier existing).
	mod := moderation.ModerateContent(ctx, d.Pool, moderation.Input{
		Title: strPtrOrNil(title), Content: strPtrOrNil(content),
		BrandID: &brandID, UserID: &userID,
	})
	if mod.Status == "blocked" {
		if err := d.exec(ctx, "UPDATE taut_campaign_media SET article_status = 'blocked' WHERE id = $1", itemID); err != nil {
			return gatekeeperItemResult{}, err
		}
		moderation.RecordAudit(ctx, d.Pool, "document", documentID, mod, &brandID)
		return gatekeeperItemResult{
			MediaID: mediaID, Slot: slot, Status: "blocked", Verdict: "gagal",
			Reason: "Diblokir moderasi (" + mod.Category + ")",
		}, nil
	}

	text := stripHTMLToText(content)
	voiceRange := 0
	if min, _, ok := wordCountRangeFor(item.strOr("word_count_mode", "")); ok {
		voiceRange = min
	}
	mediaType := item.strOr("media_type", "")
	if mediaType == "" {
		mediaType = campaign.strOr("media_type", "")
	}
	// Tiga cek independen (safety, link health, AI Overview) dijalankan paralel:
	// masing-masing memanggil LLM sendiri, serial membuat satu artikel ~4 menit.
	var safety contentSafetyResult
	var links linkHealthResult
	var aiOverview map[string]any
	var checksWG sync.WaitGroup
	checksWG.Add(2)
	go func() {
		defer checksWG.Done()
		safety, aiOverview = d.contentSafetyAndOverview(ctx, title, mediaType, content, text,
			refMaterial, extractHTMLLinks(content), aiOverviewSet)
	}()
	go func() {
		defer checksWG.Done()
		links = d.linkHealthValidator(ctx, content,
			[]string{
				strings.TrimSpace(campaign.strOr("primary_url", "")),
				strings.TrimSpace(campaign.strOr("backup_url", "")),
			},
			d.ownLinkDomains(ctx, campaign, item.strOr("website_url", "")))
	}()
	checksWG.Wait()
	batchMS := time.Since(startedAt).Milliseconds()
	voice := brandVoiceLimits(text, voiceRange)

	// Batas tautan keluar: media hanya menyetujui N tautan. Hitung tautan fisik
	// di artikel (self-link situs media tidak dihitung) lalu bandingkan dengan
	// max_links. Kelebihan tautan = pelanggaran kebijakan media, bukan sekadar
	// peringatan — artikel tidak boleh menjadi 'ready'.
	maxLinksPtr := (*int)(nil)
	if item.has("max_links") {
		v := int(item.numOr("max_links"))
		maxLinksPtr = &v
	}
	linkCap, linkAllowed := mediaLinkCap(item.boolOr("allow_hyperlink", false), maxLinksPtr)
	outgoingLinks := countOutboundLinks(content, hostSet(item.strOr("website_url", "")))
	linkPolicyFailed := (!linkAllowed && outgoingLinks > 0) || (linkAllowed && outgoingLinks > linkCap)
	linkVerdict := links.Verdict
	if linkPolicyFailed {
		linkVerdict = "failed"
	}
	linkPolicy := map[string]any{
		"status": "passed", "allowed": linkAllowed, "max_links": linkCap, "outgoing": outgoingLinks,
	}
	if linkPolicyFailed {
		linkPolicy["status"] = "failed"
	}

	safetyStatus := "review_required"
	switch safety.RiskLevel {
	case "high":
		safetyStatus = "failed"
	case "medium":
		safetyStatus = "review_required"
	case "low":
		safetyStatus = "passed"
	}

	checks := map[string]any{
		"aiOverview": aiOverview,
		"contentSafety": map[string]any{
			"status": safetyStatus, "riskLevel": safety.RiskLevel,
			"categories": safety.Categories, "confidence": safety.Confidence,
			"reasons": capStrings(safety.Reasons, 3),
		},
		"linkHealth": map[string]any{"status": linkVerdict, "links": links.Links, "policy": linkPolicy},
		"brandVoice": map[string]any{"status": "passed", "level": voice.Level, "notes": capStrings(voice.Notes, 3)},
	}

	overallVerdict := "passed"
	if safetyStatus == "failed" || linkVerdict == "failed" {
		overallVerdict = "failed"
	} else if safetyStatus == "review_required" || linkVerdict == "review_required" {
		overallVerdict = "review_required"
	}

	latestAudit, err := d.getLatestGatekeeperAudit(ctx, campaignID, mediaID, slot)
	if err != nil {
		return gatekeeperItemResult{}, err
	}
	runRevision := 1
	if latestAudit != nil {
		runRevision = latestAudit.RunRevision + 1
	}
	if err := d.saveGatekeeperAudit(ctx, campaignID, mediaID, slot, runRevision, checks, overallVerdict, userID); err != nil {
		return gatekeeperItemResult{}, err
	}

	name := d.mediaNameFor(ctx, mediaID)
	allowLink := d.mediaAllowsHyperlink(ctx, mediaID)
	// Percepatan: pass revisi akhir (LLM yang menulis ulang SELURUH artikel,
	// 26-79 dtk) hanya dijalankan bila AI Overview menolak (catatan revisinya
	// perlu diterapkan). Masalah safety/link bukan tugas penulisan ulang —
	// statusnya sudah tercermin di overallVerdict + hard gate distribusi.
	needsRepair := ai.AIOverviewRejected(aiOverview)
	linkRule := "Media TIDAK mengizinkan hyperlink. JANGAN menanam link apa pun."
	if allowLink {
		linkRule = "Media mengizinkan hyperlink. Tanam link sesuai konfigurasi kampanye secara natural bila relevan."
	}
	if linkAllowed {
		linkRule += " MAKSIMAL " + itoa(linkCap) + " tautan keluar di seluruh artikel (sekarang: " + itoa(outgoingLinks) + ")."
	}
	promptLines := []string{
		"Kampanye: " + campaign.strOr("name", ""),
		"Media tujuan: " + name,
		linkRule,
		"Periksa judi/SARA/ujaran kebencian.",
		"Judul saat ini: " + title,
		"Isi (HTML): " + truncateString(content, 4000),
	}
	if strings.TrimSpace(item.strOr("angle_slug", "")) != "" {
		promptLines = append(promptLines, "Pertahankan sudut konten dan bahan Writer berikut:\n"+refMaterial)
	}
	// Catatan AI Overview disambungkan ke pass revisi: bila indikator lemah,
	// penulisan ulang wajib memperbaikinya.
	if notes := ai.AIOverviewRevisionNotes(aiOverview); len(notes) > 0 {
		promptLines = append(promptLines,
			"Catatan revisi AI Overview (WAJIB diperbaiki saat menulis ulang artikel):",
			"- "+strings.Join(notes, "\n- "))
	}
	promptLines = append(promptLines, "", "Output JSON valid {final_html, verdict: layak_tayang|revisi|gagal, catatan}.")
	prompt := strings.Join(promptLines, "\n")

	finalHTML := content
	verdict := "layak_tayang"
	revisiMS, reevalMS := int64(0), int64(0)
	if !needsRepair {
		// Tanpa LLM: status diturunkan dari hasil pengecekan (failed → blocked,
		// review_required → revision) supaya artikel berisiko tetap tidak 'ready'.
		switch {
		case safetyStatus == "failed" || links.Verdict == "failed":
			verdict = "gagal"
		case safetyStatus == "review_required" || linkVerdict == "review_required" || linkPolicyFailed:
			verdict = "revisi"
		}
	}
	if needsRepair {
		revisiStart := time.Now()
		result, err := d.runTautAgent(ctx, "gatekeeper", campaignID, prompt, &mediaID, true)
		if err != nil {
			return gatekeeperItemResult{}, err
		}
		revisiMS = time.Since(revisiStart).Milliseconds()
		var parsed struct {
			FinalHTML string `json:"final_html"`
			Verdict   string `json:"verdict"`
		}
		if jsonUnmarshal([]byte(result.Content), &parsed) == nil {
			if parsed.FinalHTML != "" {
				finalHTML = truncateString(parsed.FinalHTML, 200000)
			}
			if parsed.Verdict != "" {
				verdict = parsed.Verdict
			}
		}
	}
	// H1 dilarang di isi artikel (judul ada di field judul) — hasil tulisan
	// ulang model yang memakai H1 ikut diturunkan; tanpa H1 tidak berubah.
	finalHTML = demoteH1ToH2(finalHTML)
	if !allowLink {
		if url := campaign.strOr("primary_url", ""); url != "" {
			finalHTML = stripLinksTo(finalHTML, url)
		}
		finalHTML = stripAllLinks(finalHTML)
	}

	// Artikel buatan AI Tauthub sendiri: bila skor di bawah ambang dan pass
	// revisi di atas sudah menulis ulang artikel, nilai ULANG sekali pada hasil
	// revisi — skor yang tersimpan adalah kualitas akhir (hijau bila revisi
	// memperbaiki indikator lemah). Satu putaran saja, tanpa loop.
	if (attribution == "ai_generated" || attribution == "ai_edited") && ai.AIOverviewRejected(aiOverview) && finalHTML != content {
		reevalStart := time.Now()
		rechecked := d.runAIOverviewCheck(ctx, title, mediaType, finalHTML, stripHTMLToText(finalHTML), refMaterial, aiOverviewSet)
		reevalMS = time.Since(reevalStart).Milliseconds()
		if rechecked != nil {
			aiOverview = rechecked
			checks["aiOverview"] = rechecked
			if err := d.updateGatekeeperAuditChecks(ctx, campaignID, mediaID, runRevision, checks); err != nil {
				return gatekeeperItemResult{}, err
			}
		}
	}

	// Versi baru hanya ditulis bila isi benar-benar berubah (revisi LLM atau
	// penghapusan link deterministik) — menghindari churn versi tiap scan.
	if finalHTML != content {
		nextVersion := currentVersion + 1
		if err := d.exec(ctx,
			"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, $3, $4, 'html', 'ai_edited', 'Taut Pintar Gatekeeper', $5)",
			uuid.NewString(), documentID, nextVersion, finalHTML, userID); err != nil {
			return gatekeeperItemResult{}, err
		}
		if err := d.exec(ctx,
			"UPDATE content_documents SET current_version = $1, updated_at = NOW() WHERE id = $2", nextVersion, documentID); err != nil {
			return gatekeeperItemResult{}, err
		}
	}
	// Status artikel diturunkan dari GERBANG KERAS yang sama dengan syarat
	// distribusi (contentSafety + linkHealth) — BUKAN dari verdict yang dilaporkan
	// model saat pass revisi. Model kerap menjawab "revisi" walaupun pemeriksaan
	// ulang sudah lulus, dan status pun tersangkut 'revision': kampanye mustahil
	// diterbitkan padahal panel QC bilang "Siap Distribusi".
	status := "ready"
	switch {
	case safetyStatus == "failed" || links.Verdict == "failed":
		status = "blocked"
	case linkPolicyFailed || safetyStatus == "review_required" || linkVerdict == "review_required":
		// Tautan berlebih bisa diperbaiki klien (hapus tautan lalu Cek
		// Validasi ulang) — 'revision', bukan 'blocked' yang terasa final.
		status = "revision"
	}
	if err := d.exec(ctx, "UPDATE taut_campaign_media SET article_status = $1 WHERE id = $2", status, itemID); err != nil {
		return gatekeeperItemResult{}, err
	}
	// Log timing per tahap: satu-satunya cara menilai di mana waktu habis saat
	// QC terasa lambat (model lambat vs pass revisi vs re-evaluasi).
	slog.Info("[gatekeeper] audit selesai", "media", mediaID,
		"batch_ms", batchMS, "revisi", needsRepair, "revisi_ms", revisiMS, "reeval_ms", reevalMS,
		"total_ms", time.Since(startedAt).Milliseconds(), "status", status)
	return gatekeeperItemResult{MediaID: mediaID, Slot: slot, Status: status, Verdict: verdict}, nil
}

// updateGatekeeperAuditChecks: perbarui checks baris audit yang baru dibuat
// (dipakai setelah penilaian ulang AI Overview pasca-revisi).
func (d *Deps) updateGatekeeperAuditChecks(ctx context.Context, campaignID, mediaID string, runRevision int, checks map[string]any) error {
	return d.exec(ctx,
		`UPDATE taut_gatekeeper_audits SET checks = $1::jsonb, updated_at = NOW()
		 WHERE campaign_id = $2 AND media_id = $3 AND run_revision = $4`,
		marshalJSON(checks), campaignID, mediaID, runRevision)
}

// mediaAllowsHyperlink: kebijakan hyperlink media — max_links (0 = tidak boleh).
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

// ---------- content safety (port lib/gatekeeper/content-safety.ts) ----------

const maxSafetyInputChars = 6000

type contentSafetyResult struct {
	RiskLevel  string   `json:"riskLevel"`
	Categories []string `json:"categories"`
	Reasons    []string `json:"reasons"`
	Confidence float64  `json:"confidence"`
}

var contentSafetyCategories = map[string]bool{
	"sexual_content": true, "child_safety": true, "gambling": true, "fraud_scam": true,
	"violence": true, "extortion": true, "hate_speech": true, "violent_extremism": true,
	"harmful_propaganda": true, "dangerous_activity": true, "illegal_activity_facilitation": true,
	"human_exploitation": true, "self_harm": true, "other_illegal_content": true,
}

var contentSafetyFailsafe = contentSafetyResult{
	RiskLevel: "medium", Categories: []string{},
	Reasons: []string{"Analisis gagal — konten perlu review manual."}, Confidence: 0,
}

const contentSafetySystemPrompt = `You are the Gatekeeper content safety classifier for an Indonesian publishing platform. Classify by CONTEXT, not keywords. Do not over-block: discussion, education, news/reporting, criticism, prevention and research about a harmful topic are LOW risk. Promotion, facilitation/instruction, glorification, recruitment, or targeted harm are HIGH risk.

Return ONLY valid JSON, no markdown:
{"allowed": boolean, "riskLevel": "low"|"medium"|"high", "categories": [string], "reasons": [string], "confidence": number 0-1}

Rules:
- allowed=true ONLY for low risk. medium = suspicious/ambiguous -> allowed=false (manual review). high = block.
- reasons: max 3 short Indonesian strings, user-safe.

Output JSON only, no explanation.`

// contentSafetyGuard: fail-safe — error apa pun → review manual (medium risk).
func (d *Deps) contentSafetyGuard(ctx context.Context, text string, links []string, briefContext string) contentSafetyResult {
	parts := []string{"Konten:\n" + bodyText(text, maxSafetyInputChars)}
	if len(links) > 0 {
		limit := len(links)
		if limit > 20 {
			limit = 20
		}
		parts = append(parts, "Links:\n"+strings.Join(links[:limit], "\n"))
	}
	if briefContext != "" {
		parts = append(parts, "Konteks:\n"+bodyText(briefContext, 500))
	}
	result, err := d.callAgentLLM(ctx, "gatekeeper", strings.Join(parts, "\n\n"), true, contentSafetySystemPrompt, 0)
	if err != nil {
		slog.Warn("[brandflow] content-safety gagal -> review_required", "err", truncateString(err.Error(), 200))
		return contentSafetyFailsafe
	}
	parsed := extractJSONObject(result.Content)
	if parsed == nil {
		return contentSafetyFailsafe
	}
	return contentSafetyFromParsed(parsed)
}

// contentSafetyFromParsed: validasi field safety dari JSON yang sudah diparse
// (dipakai jalur gabungan safety+AI Overview; whitelist kategori & clamp tetap
// di sini supaya hasilnya identik dengan jalur terpisah).
func contentSafetyFromParsed(parsed map[string]any) contentSafetyResult {
	riskLevel := parseRiskLevel(parsed["riskLevel"])
	categories := []string{}
	if arr, ok := parsed["categories"].([]any); ok {
		seen := map[string]bool{}
		for _, c := range arr {
			s, ok := c.(string)
			if !ok || seen[s] || !contentSafetyCategories[s] {
				continue
			}
			seen[s] = true
			categories = append(categories, s)
			if len(categories) >= 6 {
				break
			}
		}
	}
	reasons := []string{}
	if arr, ok := parsed["reasons"].([]any); ok {
		for _, r := range arr {
			s, ok := r.(string)
			if !ok {
				continue
			}
			trimmed := truncateString(strings.TrimSpace(s), 120)
			if trimmed != "" {
				reasons = append(reasons, trimmed)
			}
			if len(reasons) >= 3 {
				break
			}
		}
	}
	return contentSafetyResult{RiskLevel: riskLevel, Categories: categories, Reasons: reasons, Confidence: clampConfidence(parsed["confidence"])}
}

func bodyText(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max]
}

func parseRiskLevel(v any) string {
	s := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
	if s == "high" || s == "medium" || s == "low" {
		return s
	}
	return "medium"
}

func clampConfidence(v any) float64 {
	f, _ := toFloat(v)
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// contentSafetyAndOverview: SATU panggilan LLM untuk klasifikasi keamanan
// konten + penilaian AI Overview (keduanya membaca teks yang sama). Bila
// respons gabungan gagal/tidak lengkap, fallback ke dua panggilan terpisah —
// kualitas cek tidak pernah dikorbankan demi kecepatan.
func (d *Deps) contentSafetyAndOverview(ctx context.Context, title, mediaType, content, text, brief string, links []string, st ai.AIOverviewSettings) (contentSafetyResult, map[string]any) {
	checkedAt := nowISO()
	briefAvailable := strings.TrimSpace(brief) != ""
	if !st.Enabled {
		return d.contentSafetyGuard(ctx, text, links, brief), ai.AIOverviewResult{
			Status: "pending", CheckedAt: checkedAt, BriefAvailable: briefAvailable,
			Info: "Pemeriksaan AI Overview dinonaktifkan admin.",
		}.JSON()
	}
	prompt := ai.GuardUserPrompt(title, mediaType, content, text, brief, links, countWords(text), st)
	result, err := d.callAgentLLM(ctx, "gatekeeper", prompt, true, ai.GuardSystemPrompt(contentSafetySystemPrompt), 0)
	if err != nil {
		slog.Warn("[brandflow] guard gabungan gagal, fallback dua panggilan", "err", truncateString(err.Error(), 200))
	} else if overviewFields, safetyFields, ok := ai.ParseGuardResponse(result.Content); ok {
		return contentSafetyFromParsed(safetyFields), ai.EvaluateAIOverviewFields(overviewFields, st, checkedAt, briefAvailable).JSON()
	} else {
		slog.Warn("[brandflow] guard gabungan tidak lengkap, fallback dua panggilan")
	}
	return d.contentSafetyGuard(ctx, text, links, brief), d.runAIOverviewCheck(ctx, title, mediaType, content, text, brief, st)
}

// ---------- link health (port lib/gatekeeper/link-health.ts) ----------

type linkHealthItem struct {
	URL        string `json:"url"`
	Status     string `json:"status"`
	HTTPStatus *int   `json:"httpStatus"`
	Category   any    `json:"category"`
	Reason     string `json:"reason,omitempty"`
	CheckedAt  string `json:"checkedAt"`
}

type linkHealthResult struct {
	Links          []linkHealthItem `json:"links"`
	Verdict        string           `json:"verdict"`
	BlockedReasons []string         `json:"blockedReasons"`
}

var unsafeLinkCategories = map[string]bool{"gambling": true, "pornography": true, "malware": true}
var validLinkCategories = map[string]bool{"gambling": true, "pornography": true, "malware": true, "safe": true, "unknown": true}
var linkReasons = map[string]string{
	"gambling":    "Mengarah ke konten judi",
	"pornography": "Mengarah ke konten pornografi",
	"malware":     "Berpotensi malware",
	"unknown":     "Kategori tidak pasti, perlu review",
}

// linkHealthValidator: probe reachability + klasifikasi AI semua link reachable.
// Provider AI link-classifier memakai agent gatekeeper; bila tidak dikonfigurasi
// hasilnya review_required (fail-safe), bukan 500.
func (d *Deps) linkHealthValidator(ctx context.Context, html string, excludeURLs []string, ownDomains map[string]bool) linkHealthResult {
	checkedAt := nowISO()
	excluded := map[string]bool{}
	for _, u := range excludeURLs {
		if trimmed := strings.TrimSpace(u); trimmed != "" {
			excluded[trimmed] = true
		}
	}
	// Tautan ke domain sendiri (target backlink kampanye, situs media, link
	// kanonik brand) TIDAK diprobe dan TIDAK ditanya ke AI: dari teks URL saja
	// model selalu menjawab "unknown", yang dulu membuat QC selalu "perlu
	// review" hanya karena artikel memuat tautan normal milik klien sendiri.
	ownLinks := []linkHealthItem{}
	urls := []string{}
	for _, u := range extractHTMLLinks(html) {
		if excluded[u] {
			continue
		}
		if hostOwned(u, ownDomains) {
			ownLinks = append(ownLinks, linkHealthItem{
				URL: u, Status: "ok", Category: "safe",
				Reason: "Tautan milik brand/kampanye sendiri", CheckedAt: checkedAt,
			})
			continue
		}
		urls = append(urls, u)
	}
	if len(urls) == 0 {
		return linkHealthResult{Links: []linkHealthItem{}, Verdict: "passed", BlockedReasons: []string{}}
	}

	type probe struct {
		url        string
		httpStatus *int
	}
	// Probe paralel (maks 6 sekaligus): serial 8 dtk/URL membuat artikel dengan
	// 5 tautan menunggu ~40 dtk hanya untuk pengecekan link.
	probes := make([]probe, len(urls))
	var probeWG sync.WaitGroup
	probeSem := make(chan struct{}, 6)
	for i, u := range urls {
		probeWG.Add(1)
		go func(i int, u string) {
			defer probeWG.Done()
			probeSem <- struct{}{}
			defer func() { <-probeSem }()
			status, ok := d.probeURL(ctx, u)
			if ok {
				s := status
				probes[i] = probe{url: u, httpStatus: &s}
			} else {
				probes[i] = probe{url: u, httpStatus: nil}
			}
		}(i, u)
	}
	probeWG.Wait()
	reachable := []string{}
	for _, p := range probes {
		if p.httpStatus != nil {
			reachable = append(reachable, p.url)
		}
	}
	categories, classificationFailed := d.classifyLinks(ctx, reachable)

	links := append([]linkHealthItem{}, ownLinks...)
	for _, p := range probes {
		if p.httpStatus == nil {
			links = append(links, linkHealthItem{URL: p.url, Status: "unreachable", HTTPStatus: nil, Reason: "Tidak dapat dijangkau", CheckedAt: checkedAt})
			continue
		}
		if classificationFailed {
			// Alat klasifikasi tidak tersedia ≠ temuan. Dicatat, tidak menggagalkan.
			links = append(links, linkHealthItem{URL: p.url, Status: "ok", HTTPStatus: p.httpStatus,
				Reason: "Klasifikasi otomatis tidak tersedia — dicatat, tidak menghalangi", CheckedAt: checkedAt})
			continue
		}
		category := categories[p.url]
		if category == "" {
			category = "unknown"
		}
		if unsafeLinkCategories[category] {
			links = append(links, linkHealthItem{URL: p.url, Status: "unsafe", HTTPStatus: p.httpStatus, Category: category, Reason: linkReasons[category], CheckedAt: checkedAt})
			continue
		}
		if category == "unknown" {
			// Hanya judi/pornografi/malware yang menggagalkan. Ragu terhadap URL
			// normal dicatat sebagai keterangan supaya QC tidak selalu merah.
			links = append(links, linkHealthItem{URL: p.url, Status: "ok", HTTPStatus: p.httpStatus,
				Category: category,
				Reason:   "Tidak dapat dipastikan dari URL saja — dicatat, tidak menghalangi", CheckedAt: checkedAt})
			continue
		}
		links = append(links, linkHealthItem{URL: p.url, Status: "ok", HTTPStatus: p.httpStatus, Category: "safe", CheckedAt: checkedAt})
	}

	blockedReasons := []string{}
	unsafeCount := 0
	hasSuspicious := false
	for _, l := range links {
		if l.Status == "unsafe" {
			unsafeCount++
			cat, _ := l.Category.(string)
			reason := linkReasons[cat]
			if reason == "" {
				reason = "Konten berbahaya"
			}
			blockedReasons = append(blockedReasons, reason+": "+l.URL)
		}
		if l.Status == "suspicious" {
			hasSuspicious = true
		}
	}
	verdict := "passed"
	if unsafeCount > 0 {
		verdict = "failed"
	} else if hasSuspicious {
		verdict = "review_required"
	}
	return linkHealthResult{Links: links, Verdict: verdict, BlockedReasons: blockedReasons}
}

var reAnyURL = regexp.MustCompile(`https?://[^\s,;|()\[\]]+`)

// hostOwned: true bila host tautan adalah domain sendiri, termasuk subdomainnya.
func hostOwned(rawURL string, own map[string]bool) bool {
	if len(own) == 0 {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if own[host] {
		return true
	}
	for d := range own {
		if strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// ownLinkDomains: domain yang jelas milik sendiri — target backlink kampanye,
// situs media tujuannya, link kanonik + website dari Profiler brand yang dipakai
// kampanye, dan Tauthub sendiri. Pencocokan berbasis DOMAIN, bukan string URL
// persis, supaya `https://situs.com`, `https://www.situs.com`, dan
// `https://situs.com/halaman` sama-sama dikenali sebagai milik sendiri.
func (d *Deps) ownLinkDomains(ctx context.Context, campaign bsRow, mediaWebsite string) map[string]bool {
	own := map[string]bool{"tauthub.com": true}
	add := func(raw string) {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Hostname() == "" {
			return
		}
		own[strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")] = true
	}
	add(campaign.strOr("primary_url", ""))
	add(campaign.strOr("backup_url", ""))
	add(mediaWebsite)
	for _, link := range d.profilerOwnLinks(ctx, campaign) {
		add(link)
	}
	return own
}

// profilerOwnLinks: link kanonik serta website dari Profiler yang dipakai
// kampanye — tautan milik klien sendiri yang tidak perlu dinilai AI.
func (d *Deps) profilerOwnLinks(ctx context.Context, campaign bsRow) []string {
	profileID := strings.TrimSpace(campaign.strOr("profiler_id", ""))
	if profileID == "" {
		return nil
	}
	row, err := d.queryRow(ctx,
		"SELECT entity_core, identity FROM profiler_profiles WHERE id = $1 AND deleted_at IS NULL", profileID)
	if err != nil || row == nil {
		return nil
	}
	out := []string{}
	pick := func(raw any, keys ...string) {
		m, ok := raw.(map[string]any)
		if !ok {
			return
		}
		for _, k := range keys {
			v, isStr := m[k].(string)
			if !isStr || strings.TrimSpace(v) == "" {
				continue
			}
			if found := reAnyURL.FindAllString(v, -1); len(found) > 0 {
				out = append(out, found...)
			}
			out = append(out, strings.TrimSpace(v))
		}
	}
	pick(row.jsonField("entity_core"), "canonical_links")
	pick(row.jsonField("identity"), "website")
	return out
}

// probeURL: safeFetch HEAD/GET → status code; false bila gagal.
func (d *Deps) probeURL(ctx context.Context, rawURL string) (int, bool) {
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	resp, err := security.Get(reqCtx, rawURL, map[string]string{
		"user-agent": "Mozilla/5.0 (compatible; TautHub/1.0)",
		"accept":     "*/*",
	}, 8*time.Second)
	if err != nil || resp == nil {
		return 0, false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
	return resp.StatusCode, true
}

// classifyLinks: satu panggilan AI untuk semua URL reachable → (map, failed).
func (d *Deps) classifyLinks(ctx context.Context, urls []string) (map[string]string, bool) {
	if len(urls) == 0 {
		return map[string]string{}, false
	}
	lines := make([]string, 0, len(urls))
	for i, u := range urls {
		lines = append(lines, itoa(i+1)+". "+u)
	}
	prompt := strings.Join([]string{
		"Daftar URL berikut muncul di artikel yang akan didistribusikan.",
		"Klasifikasikan tiap URL apakah mengarah ke konten judi (\"gambling\"), pornografi (\"pornography\"), malware, atau aman (\"safe\").",
		"Bila tidak yakin, gunakan \"unknown\" — jangan menebak domain legit sebagai berbahaya.",
		"Balas JSON object: {\"links\": [{\"url\": \"<url persis dari daftar>\", \"category\": \"gambling|pornography|malware|safe|unknown\", \"confidence\": 0.0}]}",
		"URL:\n" + strings.Join(lines, "\n"),
	}, "\n")
	result, err := d.callAgentLLM(ctx, "gatekeeper", prompt, true, "", 0)
	if err != nil {
		return nil, true
	}
	parsed := extractJSONObject(result.Content)
	entries := []any{}
	ok := false
	if parsed != nil {
		if arr, isArr := parsed["links"].([]any); isArr {
			entries = arr
			ok = true
		}
	}
	if !ok {
		return nil, true
	}
	m := map[string]string{}
	for _, entry := range entries {
		obj, isObj := entry.(map[string]any)
		if !isObj {
			continue
		}
		url, _ := obj["url"].(string)
		category, _ := obj["category"].(string)
		if url != "" && validLinkCategories[category] {
			m[url] = category
		}
	}
	if len(m) == 0 {
		return nil, true
	}
	return m, false
}

// ---------- brand voice (port lib/gatekeeper/voice-check.ts) ----------

type voiceCheckResult struct {
	Level string
	Notes []string
}

// brandVoiceLimits: cek deterministik; hasil informasional (tidak pernah gagal).
func brandVoiceLimits(text string, minWords int) voiceCheckResult {
	if text == "" {
		return voiceCheckResult{Level: "skipped", Notes: []string{}}
	}
	wordCount := countWords(text)
	promoHits := countPromotional(text)
	notes := []string{}
	level := ""
	switch {
	case minWords > 0 && wordCount < int(float64(minWords)*0.7):
		level = "off_brand"
		notes = append(notes, "Artikel terlalu pendek: "+itoa(wordCount)+" kata (min "+itoa(minWords)+").")
	case promoHits > 8:
		level = "off_brand"
		notes = append(notes, "Promosi berlebihan: "+itoa(promoHits)+" istilah promosi.")
	case minWords > 0 && wordCount < minWords:
		level = "minor_drift"
		notes = append(notes, "Jumlah kata "+itoa(wordCount)+" di bawah minimum "+itoa(minWords)+".")
	case promoHits > 0:
		level = "minor_drift"
		notes = append(notes, "Ada "+itoa(promoHits)+" istilah promosi — kurangi nada jualan.")
	default:
		level = "consistent"
	}
	return voiceCheckResult{Level: level, Notes: notes}
}

// ---------- AI Overview (kesiapan artikel untuk Google AI Overviews) ----------

// aiOverviewSettings: toggle + ambang dari taut_agent_configs.settings.aiOverview.
func (d *Deps) aiOverviewSettings(ctx context.Context) ai.AIOverviewSettings {
	return ai.AIOverviewSettingsFromJSON(d.getAgentConfig(ctx, "gatekeeper").Settings["aiOverview"])
}

// runAIOverviewCheck: nilai artikel untuk kesiapan AI Overview (non-blocking).
// Bagian murni (prompt + parsing + aturan gate) ada di internal/ai supaya skema
// sama dengan jalur clienttaut.
func (d *Deps) runAIOverviewCheck(ctx context.Context, title, mediaType, contentHTML, text, brief string, st ai.AIOverviewSettings) map[string]any {
	checkedAt := nowISO()
	briefAvailable := strings.TrimSpace(brief) != ""
	if !st.Enabled {
		return ai.AIOverviewResult{
			Status: "pending", CheckedAt: checkedAt, BriefAvailable: briefAvailable,
			Info: "Pemeriksaan AI Overview dinonaktifkan admin.",
		}.JSON()
	}
	prompt := ai.AIOverviewUserPrompt(title, mediaType, contentHTML, countWords(text), st, brief)
	result, err := d.callAgentLLM(ctx, "gatekeeper", prompt, true, ai.AIOverviewSystemPrompt, 0)
	if err != nil {
		slog.Warn("[brandflow] ai-overview gagal -> review_required", "err", truncateString(err.Error(), 200))
		return ai.AIOverviewResult{
			Status: "review_required", CheckedAt: checkedAt, BriefAvailable: briefAvailable,
			Error: "AI Overview tidak dapat menilai artikel: " + truncateString(err.Error(), 200),
		}.JSON()
	}
	return ai.EvaluateAIOverview(result.Content, st, checkedAt, briefAvailable).JSON()
}

// ---------- html helpers ----------

var reScriptBlock = mustRegexp(`(?is)<script[^>]*>.*?</script\s*>`)
var reStyleBlock = mustRegexp(`(?is)<style[^>]*>.*?</style\s*>`)
var reHref = mustRegexp(`(?i)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["']`)

// stripHTMLToText: HTML → visible text.
func stripHTMLToText(html string) string {
	if html == "" {
		return ""
	}
	stripped := reScriptBlock.ReplaceAllString(html, " ")
	stripped = reStyleBlock.ReplaceAllString(stripped, " ")
	stripped = reHTMLTag.ReplaceAllString(stripped, " ")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(stripped, " "))
}

// extractHTMLLinks: daftar href absolut (unik, tanpa anchor/javascript:).
func extractHTMLLinks(html string) []string {
	if html == "" {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, m := range reHref.FindAllStringSubmatch(html, -1) {
		u := strings.TrimSpace(m[1])
		if u == "" || strings.HasPrefix(u, "#") || strings.HasPrefix(u, "javascript:") {
			continue
		}
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// stripLinksTo: buang hyperlink yang menunjuk URL tertentu (konten link → teks).
func stripLinksTo(html, targetURL string) string {
	if html == "" || targetURL == "" {
		return html
	}
	re := mustRegexp(`(?is)<a\b[^>]*href=["']` + regexpQuoteMeta(targetURL) + `["'][^>]*>(.*?)</a>`)
	return re.ReplaceAllString(html, "$1")
}

// stripAllLinks: semua hyperlink → teks (media tidak mengizinkan link).
func stripAllLinks(html string) string {
	return reAnchorPair.ReplaceAllString(html, "$1")
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

func jsonUnmarshal(raw []byte, target any) error {
	return jsonUnmarshalStd(raw, target)
}

func nowISO() string {
	return time.Now().UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z07:00")
}
