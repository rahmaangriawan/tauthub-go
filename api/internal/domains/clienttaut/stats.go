package clienttaut

import (
	"context"

	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
)

// Port lib/taut/campaign-stats.ts.

type campaignStatsResult struct {
	Articles       int `json:"articles"`
	PublishedMedia int `json:"published_media"`
	TCUsed         int `json:"tc_used"`
}

func emptyCampaignStats() campaignStatsResult {
	return campaignStatsResult{Articles: 0, PublishedMedia: 0, TCUsed: 0}
}

// campaignStats: statistik ringkas satu kampanye (3 agregat DB).
func campaignStats(ctx context.Context, d *Deps, campaignID string) (campaignStatsResult, error) {
	out := emptyCampaignStats()
	art, err := d.queryRow(ctx,
		"SELECT COUNT(*)::int AS n FROM taut_campaign_media WHERE campaign_id = $1 AND article_document_id IS NOT NULL", campaignID)
	if err != nil {
		return out, err
	}
	out.Articles = art.intOr("n", 0)

	pub, err := d.queryRow(ctx,
		"SELECT COUNT(*)::int AS n FROM publications WHERE campaign_id = $1 AND status = 'published'", campaignID)
	if err != nil {
		return out, err
	}
	out.PublishedMedia = pub.intOr("n", 0)

	tc, err := d.queryRow(ctx,
		`SELECT COALESCE(SUM(-amount_tc), 0)::int AS n FROM credit_ledger_entries
		 WHERE ref_type = 'capture' AND ref_id LIKE 'taut:' || $1 || ':%'`, campaignID)
	if err != nil {
		return out, err
	}
	out.TCUsed = tc.intOr("n", 0)
	return out, nil
}

// campaignStatsMany: statistik banyak kampanye dalam 3 query (grid daftar).
func campaignStatsMany(ctx context.Context, d *Deps, campaignIDs []string) (map[string]campaignStatsResult, error) {
	out := map[string]campaignStatsResult{}
	if len(campaignIDs) == 0 {
		return out, nil
	}
	for _, id := range campaignIDs {
		out[id] = emptyCampaignStats()
	}
	ph := placeholders(1, len(campaignIDs))
	args := make([]any, 0, len(campaignIDs))
	for _, id := range campaignIDs {
		args = append(args, id)
	}

	art, err := d.queryRows(ctx,
		`SELECT campaign_id, COUNT(*)::int AS n FROM taut_campaign_media
		 WHERE campaign_id IN (`+ph+`) AND article_document_id IS NOT NULL GROUP BY campaign_id`, args...)
	if err != nil {
		return nil, err
	}
	for _, r := range art {
		cur := out[r.strOr("campaign_id", "")]
		cur.Articles = r.intOr("n", 0)
		out[r.strOr("campaign_id", "")] = cur
	}

	pub, err := d.queryRows(ctx,
		`SELECT campaign_id, COUNT(*)::int AS n FROM publications
		 WHERE campaign_id IN (`+ph+`) AND status = 'published' GROUP BY campaign_id`, args...)
	if err != nil {
		return nil, err
	}
	for _, r := range pub {
		key := r.strOr("campaign_id", "")
		cur := out[key]
		cur.PublishedMedia = r.intOr("n", 0)
		out[key] = cur
	}

	tc, err := d.queryRows(ctx,
		`SELECT split_part(ref_id, ':', 2) AS campaign_id, COALESCE(SUM(-amount_tc), 0)::int AS n
		 FROM credit_ledger_entries
		 WHERE ref_type = 'capture' AND ref_id LIKE 'taut:%:%' AND split_part(ref_id, ':', 2) IN (`+ph+`)
		 GROUP BY split_part(ref_id, ':', 2)`, args...)
	if err != nil {
		return nil, err
	}
	for _, r := range tc {
		key := r.strOr("campaign_id", "")
		if _, ok := out[key]; !ok {
			continue
		}
		cur := out[key]
		cur.TCUsed = r.intOr("n", 0)
		out[key] = cur
	}
	return out, nil
}

// addManualMedia: port addManualMedia — tambah media manual ke placement.
// Hanya media active; duplikat diabaikan (UNIQUE guard).
func (d *Deps) addManualMedia(ctx context.Context, campaignID, brandID string, mediaIDs []string) (map[string]any, error) {
	if _, err := d.loadCampaignForAgents(ctx, campaignID, brandID); err != nil {
		return nil, err
	}
	added := []string{}
	skipped := []string{}
	for _, mediaID := range mediaIDs {
		m, err := d.queryRow(ctx, "SELECT id FROM media WHERE id = $1 AND status = 'active' LIMIT 1", mediaID)
		if err != nil {
			return nil, err
		}
		if m == nil {
			skipped = append(skipped, mediaID)
			continue
		}
		err = d.exec(ctx,
			"INSERT INTO taut_campaign_media (id, campaign_id, media_id, source, status) VALUES ($1, $2, $3, 'manual', 'selected')",
			newUUID(), campaignID, mediaID)
		if err != nil {
			skipped = append(skipped, mediaID)
			continue
		}
		added = append(added, mediaID)
	}
	return map[string]any{"added": added, "skipped": skipped}, nil
}

// listPlacement: daftar placement tersimpan (resume wizard tanpa re-run Matchmaker).
func (d *Deps) listPlacement(ctx context.Context, campaignID, brandID string) ([]map[string]any, error) {
	if _, err := d.loadCampaignForAgents(ctx, campaignID, brandID); err != nil {
		return nil, err
	}
	rateIDR := d.getTcConfig(ctx).RateIDR
	if rateIDR < 1 {
		rateIDR = 1
	}
	items, err := d.queryRows(ctx,
		`SELECT tcm.media_id, m.name, m.media_type, m.website_url, tcm.status, tcm.article_status, tcm.match_reason, tcm.ai_score, tcm.match_rank,
		        m.credit_cost, m.allow_hyperlink, m.max_links, m.allow_embed, m.label_sponsor, tcm.regen_count, tcm.angle_slug,
		        COALESCE(tcm.price, m.price, 0) AS price,
		        COALESCE((SELECT (length(cv.content) - length(replace(cv.content, '<img', ''))) / 4
		                  FROM content_documents cd
		                  JOIN content_versions cv ON cv.document_id = cd.id AND cv.version = cd.current_version
		                  WHERE cd.id = tcm.article_document_id), 0) AS images_count,
		        COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = m.id), m.category) AS categories_csv
		 FROM taut_campaign_media tcm
		 JOIN media m ON m.id = tcm.media_id
		 WHERE tcm.campaign_id = $1
		 ORDER BY tcm.match_rank ASC NULLS LAST, m.name ASC`, campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, i := range items {
		mediaID := i.strOr("media_id", "")
		name := i.strOr("name", mediaID)
		var score any
		if i.has("ai_score") {
			score = clampInt(i.intOr("ai_score", 0), 0, 100)
		}
		var reason any
		if i.has("match_reason") {
			reason = i.strOr("match_reason", "")
		}
		var creditCost any
		if i.has("credit_cost") {
			creditCost = i.numOr("credit_cost")
		}
		var allowHyperlink any
		if i.has("allow_hyperlink") {
			allowHyperlink = i.boolOr("allow_hyperlink", false)
		}
		var maxLinks any
		if i.has("max_links") {
			maxLinks = i.numOr("max_links")
		}
		var regenCount any
		if i.has("regen_count") {
			regenCount = i.numOr("regen_count")
		}
		categories := catutil.Split(i.strOr("categories_csv", ""))
		out = append(out, map[string]any{
			"media_id":        mediaID,
			"name":            name,
			"media_type":      i.strOr("media_type", ""),
			"website_url":     i.raw("website_url"),
			"score":           score,
			"status":          i.strOr("status", ""),
			"article_status":  i.strOr("article_status", "none"),
			"reason":          reason,
			"credit_cost":     creditCost,
			"allow_hyperlink": allowHyperlink,
			"max_links":       maxLinks,
			"allow_embed":     i.boolOr("allow_embed", false),
			"regen_count":     regenCount,
			"angle_slug":      i.raw("angle_slug"),
			"categories":      categories,
			"label_sponsor":   i.strOr("label_sponsor", "tidak_ada"),
			// price + cost_tc: sumber biaya media yang SAMA dengan penagihan
			// (COALESCE(price penempatan, harga media) → kredit dibulatkan).
			// credit_cost lama tetap dikirim untuk kompatibilitas, tapi bukan
			// angka yang ditagihkan.
			"price":        i.numOr("price"),
			"cost_tc":      (int(i.numOr("price")) + rateIDR/2) / rateIDR,
			"images_count": int(i.numOr("images_count")),
		})
	}
	return out, nil
}

// userGenerateQuota: jatah generate per akun (users.writer_generate_quota).
// NULL / belum diatur = 0 artinya TANPA BATAS; limit hanya berlaku bila admin
// mengisinya di halaman Admin Users.
func (d *Deps) userGenerateQuota(ctx context.Context, userID string) int {
	row, err := d.queryRow(ctx, "SELECT writer_generate_quota FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil || row == nil {
		return 0
	}
	return row.intOr("writer_generate_quota", 0)
}

// estimateBudgetItem: hasil estimateBudget (parity interface BudgetEstimate).
type estimateBudgetResult struct {
	CandidateCount  int  `json:"candidate_count"`
	MaxSelection    int  `json:"max_selection"`
	EstimatedMedia  int  `json:"estimated_media"`
	EstimatedCostTC int  `json:"estimated_cost_tc"`
	AvailableTC     int  `json:"available_tc"`
	MinimumTC       int  `json:"minimum_tc"`
	Sufficient      bool `json:"sufficient"`
}

// estimateBudget: estimasi biaya tahap anggaran — deterministik tanpa LLM.
func (d *Deps) estimateBudget(ctx context.Context, campaignID, brandID string) (estimateBudgetResult, error) {
	campaign, err := d.loadCampaignForAgents(ctx, campaignID, brandID)
	if err != nil {
		return estimateBudgetResult{}, err
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
	candidates, err := d.queryRows(ctx,
		`SELECT m.id, m.credit_cost FROM media m
		 WHERE m.status = 'active' AND m.integration_type <> 'manual' `+typeFilter)
	if err != nil {
		return estimateBudgetResult{}, err
	}
	costs := make([]int, 0, len(candidates))
	for _, c := range candidates {
		v := int(c.numOr("credit_cost"))
		if v < 1 {
			v = 1
		}
		costs = append(costs, v)
	}
	// Sort menaik (parity sort((a,b)=>a-b)) — insertion sort stabil.
	for i := 1; i < len(costs); i++ {
		for j := i; j > 0 && costs[j] < costs[j-1]; j-- {
			costs[j], costs[j-1] = costs[j-1], costs[j]
		}
	}
	estimatedMedia := len(costs)
	if estimatedMedia > 10 {
		estimatedMedia = 10
	}
	estimatedCost := 0
	for i := 0; i < estimatedMedia; i++ {
		estimatedCost += costs[i]
	}

	wallet, err := d.queryRow(ctx,
		"SELECT COALESCE(available_tc, 0) AS available_tc FROM credit_wallets WHERE brand_id = $1 LIMIT 1", brandID)
	if err != nil {
		return estimateBudgetResult{}, err
	}
	available := wallet.intOr("available_tc", 0)

	minimum := 100
	threshold := minimum
	if estimatedCost > threshold {
		threshold = estimatedCost
	}
	return estimateBudgetResult{
		CandidateCount:  len(candidates),
		MaxSelection:    10,
		EstimatedMedia:  estimatedMedia,
		EstimatedCostTC: estimatedCost,
		AvailableTC:     available,
		MinimumTC:       minimum,
		Sufficient:      available >= threshold,
	}, nil
}
