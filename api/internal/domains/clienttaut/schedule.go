package clienttaut

import (
	"context"
	"regexp"
	"strings"
	"time"
)

// Port lib/taut/schedule.ts — buildTimeline + createPublications.

type timelineItem struct {
	MediaID     string `json:"media_id"`
	ScheduledAt string `json:"scheduled_at"`
}

// readyItems: baris taut_campaign_media siap (article ready).
func (d *Deps) readyItems(ctx context.Context, campaignID string) ([]selectedMediaRow, error) {
	rows, err := d.queryRows(ctx,
		`SELECT id, media_id, article_document_id FROM taut_campaign_media
		 WHERE campaign_id = $1 AND status = 'selected' AND article_status = 'ready' AND article_document_id IS NOT NULL
		 ORDER BY created_at`, campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]selectedMediaRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, selectedMediaRow{
			ID: r.strOr("id", ""), MediaID: r.strOr("media_id", ""), Document: r.strPtr("article_document_id"),
		})
	}
	return out, nil
}

func parseBudgetDays(raw any) int {
	m, ok := raw.(map[string]any)
	if !ok {
		return 7
	}
	days, ok := m["duration"].(float64)
	if !ok || days <= 0 {
		return 7
	}
	return int(days)
}

// buildTimeline: mode serentak → semua sekarang; terjadwal → AI Timeline Planner
// (Profiler LLM bila configured), gagal → fallback sebar merata sesuai durasi.
func (d *Deps) buildTimeline(ctx context.Context, campaignID, brandID, mode string) ([]timelineItem, error) {
	items, err := d.readyItems(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []timelineItem{}, nil
	}
	if mode == "serentak" {
		now := nowISO()
		out := make([]timelineItem, 0, len(items))
		for _, it := range items {
			out = append(out, timelineItem{MediaID: it.MediaID, ScheduledAt: now})
		}
		return out, nil
	}

	cfg, err := d.queryRow(ctx, "SELECT budget_config FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandID)
	if err != nil {
		return nil, err
	}
	durationDays := 7
	if cfg != nil {
		durationDays = parseBudgetDays(jsonField(cfg, "budget_config"))
	}

	profiler := d.getAgentConfig(ctx, "profiler")
	if profiler.Configured {
		ids := make([]string, 0, len(items))
		byID := map[string]bool{}
		for _, it := range items {
			ids = append(ids, it.MediaID)
			byID[it.MediaID] = true
		}
		prompt := strings.Join([]string{
			"Kampanye Taut Pintar — susun jadwal publikasi terjadwal.",
			"Durasi distribusi: " + itoa(durationDays) + " hari dari sekarang.",
			"Media terpilih (" + itoa(len(ids)) + "): " + strings.Join(ids, ", "),
			"",
			"Output JSON valid: {\"plan\":[{\"media_id\":\"...\",\"scheduled_at\":\"ISO8601\"}]}",
			"Sebar jadwal merata dalam rentang durasi, hindari tabrakan slot publik yang berdekatan untuk media sejenis.",
		}, "\n")
		result, err := d.runTautAgent(ctx, "profiler", campaignID, prompt, nil, true)
		if err == nil {
			parsed := extractJSONObject(result.Content)
			if parsed != nil {
				plan := []timelineItem{}
				if arr, ok := parsed["plan"].([]any); ok {
					for _, p := range arr {
						obj, ok := p.(map[string]any)
						if !ok {
							continue
						}
						mediaID, _ := obj["media_id"].(string)
						if !byID[mediaID] {
							continue
						}
						scheduled := nowISO()
						if s, ok := obj["scheduled_at"].(string); ok && s != "" {
							if t, ok := parseISO(s); ok {
								scheduled = isoMilli(t)
							}
						}
						plan = append(plan, timelineItem{MediaID: mediaID, ScheduledAt: scheduled})
					}
				}
				covered := map[string]bool{}
				for _, p := range plan {
					covered[p.MediaID] = true
				}
				if len(covered) > 0 {
					base := time.Now()
					step := durationDays * 86_400_000 / len(items)
					if step < 1 {
						step = 1
					}
					idx := 0
					for _, it := range items {
						if covered[it.MediaID] {
							continue
						}
						plan = append(plan, timelineItem{
							MediaID:     it.MediaID,
							ScheduledAt: isoMilli(base.Add(time.Duration(idx*step) * time.Millisecond)),
						})
						idx++
					}
					return plan, nil
				}
			}
		}
	}

	// Fallback deterministik: sebar merata.
	base := time.Now()
	step := durationDays * 86_400_000 / len(items)
	if step < 1 {
		step = 1
	}
	out := make([]timelineItem, 0, len(items))
	for idx, it := range items {
		out = append(out, timelineItem{
			MediaID:     it.MediaID,
			ScheduledAt: isoMilli(base.Add(time.Duration(idx*step) * time.Millisecond)),
		})
	}
	return out, nil
}

func parseISO(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// createdPublication: parity {media_id, publication_id?, status, detail?}.
type createdPublication struct {
	MediaID       string `json:"media_id"`
	PublicationID any    `json:"publication_id,omitempty"`
	Status        string `json:"status"`
	Detail        any    `json:"detail,omitempty"`
}

// createPublications: buat publikasi via createPublicationRecord per item;
// error per item tidak menghentikan sisanya.
func (d *Deps) createPublications(ctx context.Context, campaignID, brandID, userID string, timeline []timelineItem) ([]createdPublication, error) {
	out := []createdPublication{}
	for _, t := range timeline {
		ready, err := d.queryRow(ctx,
			"SELECT article_document_id, featured_image_url FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND article_status = 'ready' LIMIT 1",
			campaignID, t.MediaID)
		if err != nil {
			return nil, err
		}
		if ready == nil || !ready.has("article_document_id") {
			continue
		}
		documentID := ready.strOr("article_document_id", "")
		featured := ready.strOr("featured_image_url", "")
		if featured == "" {
			featured = "/images/article-fallback.png"
		}

		created, err := d.createPublicationRecord(ctx, createPublicationInput{
			BrandID:          brandID,
			UserID:           userID,
			DocumentID:       documentID,
			MediaID:          t.MediaID,
			PublishStatus:    "publish",
			ScheduledAt:      t.ScheduledAt,
			CampaignID:       campaignID,
			FeaturedImageURL: featured,
		})
		if err != nil {
			message := err.Error()
			if reAlreadyPublished.MatchString(message) {
				existing, qErr := d.queryRow(ctx,
					"SELECT id FROM publications WHERE document_id = $1 AND media_id = $2 LIMIT 1", documentID, t.MediaID)
				entry := createdPublication{MediaID: t.MediaID, Status: "already_published"}
				if qErr == nil && existing != nil {
					entry.PublicationID = existing.strOr("id", "")
				}
				out = append(out, entry)
				continue
			}
			out = append(out, createdPublication{MediaID: t.MediaID, Status: "failed", Detail: truncateString(message, 300)})
			continue
		}
		out = append(out, createdPublication{MediaID: t.MediaID, PublicationID: created.PublicationID, Status: created.Path})
	}
	return out, nil
}

var reAlreadyPublished = regexp.MustCompile(`(?i)already published`)
