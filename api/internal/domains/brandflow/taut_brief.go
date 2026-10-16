package brandflow

import (
	"context"
	"encoding/json"
	"strings"
)

// Port lib/taut/agents.ts loadBriefContext + matchmaker-context.ts +
// loadChannelInstruction: konteks brief Profiler dipakai Writer & Gatekeeper.

// loadTautBriefContext: konteks brief Profiler terbaru (best-effort, "" bila
// kampanye belum punya hasil Profiler valid) — port loadBriefContext.
func (d *Deps) loadTautBriefContext(ctx context.Context, campaignID string, omitAngles ...bool) (string, error) {
	row, err := d.queryRow(ctx, `
		SELECT r.executive_brief, r.writing_instructions
		FROM taut_profiler_results r
		JOIN taut_profiler_submissions s ON s.id = r.submission_id
		WHERE s.campaign_id = $1
		ORDER BY r.created_at DESC LIMIT 1`, campaignID)
	if err != nil || row == nil {
		return "", nil
	}
	brief := asMapValue(row.jsonField("executive_brief"))
	if !executiveBriefShapeValid(brief) {
		return "", nil
	}

	usp := ""
	if draft, dErr := d.queryRow(ctx,
		`SELECT business_profile FROM taut_profiler_submissions
		 WHERE campaign_id = $1 AND status = 'draft' ORDER BY revision DESC LIMIT 1`, campaignID); dErr == nil && draft != nil {
		if bp := asMapValue(draft.jsonField("business_profile")); bp != nil {
			if s, ok := bp["usp"].(string); ok {
				usp = s
			}
		}
	}

	if len(omitAngles) > 0 && omitAngles[0] {
		// These generated instructions describe the old Profiler choices, not the
		// per-media Writer override. Keep the business facts and guardrails only.
		delete(brief, "channels")
		return buildMatchmakerContextBrief(brief, usp), nil
	}
	base := buildMatchmakerContextBrief(brief, usp)
	raw := row.jsonField("writing_instructions")
	if raw == nil {
		return base, nil
	}
	instructions := ""
	if s, ok := raw.(string); ok {
		instructions = strings.TrimSpace(s)
	} else if b, mErr := json.Marshal(raw); mErr == nil {
		instructions = truncateString(string(b), 4000)
	}
	if instructions == "" {
		return base, nil
	}
	return base + "\n\n[INSTRUKSI PENULISAN PROFILER]\n" + truncateString(instructions, 4000), nil
}

// loadTautChannelInstruction: instruksi angle channel ini dari executive_brief
// (dipisah per tipe media: mainstream vs blogger) — port blok channelInstruction.
func (d *Deps) loadTautChannelInstruction(ctx context.Context, campaignID, mediaType string) string {
	row, err := d.queryRow(ctx, `
		SELECT r.executive_brief
		FROM taut_profiler_results r
		JOIN taut_profiler_submissions s ON s.id = r.submission_id
		WHERE s.campaign_id = $1
		ORDER BY r.created_at DESC LIMIT 1`, campaignID)
	if err != nil || row == nil {
		return ""
	}
	brief := asMapValue(row.jsonField("executive_brief"))
	if !executiveBriefShapeValid(brief) {
		return ""
	}
	channels, _ := brief["channels"].([]any)
	for _, c := range channels {
		ch, ok := c.(map[string]any)
		if !ok || mapGetString(ch, "channel", "") != mediaType {
			continue
		}
		instruction := strings.TrimSpace(mapGetString(ch, "writing_instruction", ""))
		if instruction == "" {
			return ""
		}
		return "[INSTRUKSI ANGLE (PROFILER) — " + mediaType + "/" + mapGetString(ch, "angle", "") + "]\n" +
			truncateString(instruction, 1500)
	}
	return ""
}

// loadTautBacklinkSettings: pengaturan backlink Profiler (submission selesai
// terbaru) — penanda backlink aktif + Smart Variations.
func (d *Deps) loadTautBacklinkSettings(ctx context.Context, campaignID string) (bool, bool) {
	row, err := d.queryRow(ctx,
		`SELECT backlink FROM taut_profiler_submissions
		 WHERE campaign_id = $1 AND status = 'completed' ORDER BY revision DESC LIMIT 1`, campaignID)
	if err != nil || row == nil {
		return false, false
	}
	bl := asMapValue(row.jsonField("backlink"))
	if bl == nil {
		return false, false
	}
	enabled, _ := bl["enabled"].(bool)
	smart, _ := bl["smart_variations"].(bool)
	return enabled, smart
}

const tautSmartVariationRule = "   - SMART VARIATIONS (WAJIB — aktif dari The Profiler): variasi anchor berlaku ANTAR ARTIKEL — artikel lain boleh memakai frasa LSI/natural yang bermakna sama dengan anchor yang diberikan. Tapi DALAM artikel INI setiap tautan yang diberikan ditanam TEPAT 1x dengan anchor persis seperti yang diberikan — DILARANG menanam URL yang sama dua kali atau menambah versi anchor kedua untuk URL yang sama di artikel ini."

// executiveBriefShapeValid: padanan longgar executiveBriefSchema.safeParse —
// hanya field yang dipakai blok konteks yang diperiksa.
func executiveBriefShapeValid(brief map[string]any) bool {
	if brief == nil {
		return false
	}
	if strings.TrimSpace(mapGetString(brief, "campaign_name", "")) == "" {
		return false
	}
	objective, ok := brief["objective"].(map[string]any)
	if !ok {
		return false
	}
	if strings.TrimSpace(mapGetString(objective, "type", "")) == "" ||
		strings.TrimSpace(mapGetString(objective, "label", "")) == "" {
		return false
	}
	channels, ok := brief["channels"].([]any)
	if !ok {
		return false
	}
	for _, c := range channels {
		ch, ok := c.(map[string]any)
		if !ok {
			return false
		}
		channel := mapGetString(ch, "channel", "")
		if channel != "mainstream" && channel != "blogger" {
			return false
		}
		if strings.TrimSpace(mapGetString(ch, "angle", "")) == "" {
			return false
		}
	}
	return true
}

// buildMatchmakerContextBrief: port buildMatchmakerContext — blok brief kampanye
// untuk prompt agen Taut.
func buildMatchmakerContextBrief(brief map[string]any, usp string) string {
	if brief == nil {
		return ""
	}
	audienceList := []string{}
	if aud, ok := brief["audience"].(map[string]any); ok {
		audienceList = append(audienceList, mapGetStringSlice(aud, "selected")...)
		if custom := strings.TrimSpace(mapGetString(aud, "custom", "")); custom != "" {
			audienceList = append(audienceList, custom)
		}
	}
	filtered := make([]string, 0, len(audienceList))
	for _, a := range audienceList {
		if a != "" {
			filtered = append(filtered, a)
		}
	}
	audienceList = filtered

	blocks := []string{"[BRIEF KAMPANYE]"}
	if s := strings.TrimSpace(mapGetString(brief, "business_summary", "")); s != "" {
		blocks = append(blocks, "Bisnis: "+truncateString(s, 800))
	}
	if strings.TrimSpace(usp) != "" {
		blocks = append(blocks, "USP: "+truncateString(strings.TrimSpace(usp), 500))
	}
	if len(audienceList) > 0 {
		blocks = append(blocks, "Audiens: "+truncateString(strings.Join(audienceList, ", "), 500))
	}
	if g, ok := brief["guardrails"].(map[string]any); ok {
		active, _ := g["active"].(bool)
		summary := strings.TrimSpace(mapGetString(g, "summary", ""))
		if active && summary != "" {
			blocks = append(blocks, "Guardrails (WAJIB dipatuhi): "+truncateString(summary, 500))
		}
	}

	angleLines := []string{}
	if channels, ok := brief["channels"].([]any); ok {
		for _, c := range channels {
			ch, ok := c.(map[string]any)
			if !ok {
				continue
			}
			angle := mapGetString(ch, "angle", "")
			instruction := strings.TrimSpace(mapGetString(ch, "writing_instruction", ""))
			if angle == "" && instruction == "" {
				continue
			}
			line := strings.TrimSpace("- [" + mapGetString(ch, "channel", "") + ":" + angle + "] " + truncateString(instruction, 500))
			if len(line) > len("- [:]") {
				angleLines = append(angleLines, line)
			}
		}
	}
	if len(angleLines) > 0 {
		blocks = append(blocks, "", "[ANGLE & INSTRUKSI PENULISAN]")
		blocks = append(blocks, angleLines...)
	}

	blocks = append(blocks, "",
		"[ATURAN RANKING — WAJIB]",
		"1. Media dengan kategori/topik yang tidak relevan dengan brief di atas WAJIB berada di peringkat bawah atau tidak dimasukkan sama sekali.",
		"2. Tolak kandidat yang melanggar guardrails.",
		"3. Jangan mengarang: alasan 1 kalimat harus merujuk pada brief atau metrik kandidat.")
	return strings.Join(blocks, "\n")
}
