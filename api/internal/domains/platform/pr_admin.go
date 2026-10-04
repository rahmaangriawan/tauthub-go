package platform

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Modul PR admin — port app/v1/platform/pr/** (9 endpoint). Semua admin-only
// (parity isPlatformAdmin → 404 "Resource not found").

var prMessageStatuses = []string{"completed", "failed"}
var prComplianceStatuses = []string{"passed", "failed", "unavailable"}

// PRAnalysisHistory: GET /v1/platform/pr/analysis-history
func (d *Deps) PRAnalysisHistory(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()

	brandID := strings.TrimSpace(q.Get("brand_id"))
	campaignID := strings.TrimSpace(q.Get("campaign_id"))
	mediaID := strings.TrimSpace(q.Get("media_id"))
	messageStatus := strings.TrimSpace(q.Get("message_status"))
	if messageStatus != "" && !aiContains(prMessageStatuses, messageStatus) {
		writeErr(w, d, errUnprocessable("message_status harus completed atau failed"))
		return
	}
	complianceStatus := strings.TrimSpace(q.Get("compliance_status"))
	if complianceStatus != "" && !aiContains(prComplianceStatuses, complianceStatus) {
		writeErr(w, d, errUnprocessable("compliance_status harus passed, failed, atau unavailable"))
		return
	}
	search := strings.TrimSpace(q.Get("q"))
	fromRaw := strings.TrimSpace(q.Get("from"))
	toRaw := strings.TrimSpace(q.Get("to"))
	if (fromRaw != "" && !prIsISODate(fromRaw)) || (toRaw != "" && !prIsISODate(toRaw)) {
		writeErr(w, d, errUnprocessable("from/to harus tanggal valid (ISO)"))
		return
	}
	format := strings.TrimSpace(q.Get("format"))
	limit := clampInt(q.Get("limit"), 50, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	params := []any{}
	add := func(clause string, value any) {
		params = append(params, value)
		where = append(where, strings.Replace(clause, "?", "$"+ptItoa(len(params)), 1))
	}
	if brandID != "" {
		add("p.brand_id = ?", brandID)
	}
	if campaignID != "" {
		add("p.campaign_id = ?", campaignID)
	}
	if mediaID != "" {
		add("p.media_id = ?", mediaID)
	}
	if messageStatus != "" {
		add("ma.status = ?", messageStatus)
	}
	if complianceStatus != "" {
		add("v.status = ?", complianceStatus)
	}
	if fromRaw != "" {
		if t, ok := prParseISODate(fromRaw); ok {
			add("p.published_at >= ?", t)
		}
	}
	if toRaw != "" {
		if t, ok := prParseISODate(toRaw); ok {
			add("p.published_at <= ?", t)
		}
	}
	if search != "" {
		needle := "%" + search + "%"
		base := len(params)
		where = append(where,
			"(d.title ILIKE $"+ptItoa(base+1)+" OR m.name ILIKE $"+ptItoa(base+2)+" OR b.name ILIKE $"+ptItoa(base+3)+")")
		params = append(params, needle, needle, needle)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	joinClause := `
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		LEFT JOIN campaigns c ON c.id = p.campaign_id
		LEFT JOIN pr_message_analyses ma ON ma.publication_id = p.id
		LEFT JOIN pr_sentiment_analyses sa ON sa.publication_id = p.id
		LEFT JOIN pr_publication_verifications v ON v.publication_id = p.id`

	countRow, err := d.queryRow(ctx, "SELECT COUNT(*) AS count"+joinClause+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items, err := d.queryRows(ctx,
		`SELECT p.id AS publication_id, p.status, p.published_at, p.published_url,
		        d.title AS document_title,
		        m.name AS media_name, m.media_tier,
		        b.name AS brand_name, b.public_id AS brand_public_id,
		        c.name AS campaign_name, c.id AS campaign_id,
		        ma.consistency_score, ma.status AS message_status, ma.error_message AS message_error,
		        sa.sentiment, sa.confidence AS sentiment_confidence,
		        v.status AS compliance_status, v.compliance_score, v.error_message AS verification_error,
		        v.verified_at AS verification_checked_at`+joinClause+clause+
			` ORDER BY p.published_at DESC NULLS LAST, p.created_at DESC LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2),
		append(params, limit, offset)...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="pr-analysis-history-`+time.Now().UTC().Format("2006-01-02")+`.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(prAnalysisHistoryCsv(items)))
		return
	}

	brands, _ := d.queryRows(ctx, "SELECT id, name, public_id FROM brands ORDER BY name")
	campaigns, _ := d.queryRows(ctx, "SELECT c.id, c.name, c.brand_id FROM campaigns c ORDER BY c.name")
	mediaRows, _ := d.queryRows(ctx, "SELECT id, name, partner_id FROM media ORDER BY name")

	rowsOut := make([]map[string]any, 0, len(items))
	for _, row := range items {
		rowsOut = append(rowsOut, map[string]any{
			"publication_id":          row.strOr("publication_id", ""),
			"status":                  row.strOr("status", ""),
			"published_at":            row.timePtr("published_at"),
			"published_url":           ptNullableStr(row, "published_url"),
			"document_title":          ptNullableStr(row, "document_title"),
			"media_name":              ptNullableStr(row, "media_name"),
			"media_tier":              aiOrEmpty(row.strOr("media_tier", ""), "tier3"),
			"brand_name":              ptNullableStr(row, "brand_name"),
			"brand_public_id":         ptNullableStr(row, "brand_public_id"),
			"campaign_id":             ptNullableStr(row, "campaign_id"),
			"campaign_name":           ptNullableStr(row, "campaign_name"),
			"message_score":           row.numPtr("consistency_score"),
			"message_status":          ptNullableStr(row, "message_status"),
			"message_error":           ptNullableStr(row, "message_error"),
			"sentiment":               ptNullableStr(row, "sentiment"),
			"sentiment_confidence":    row.numPtr("sentiment_confidence"),
			"compliance_status":       ptNullableStr(row, "compliance_status"),
			"compliance_score":        row.numPtr("compliance_score"),
			"verification_error":      ptNullableStr(row, "verification_error"),
			"verification_checked_at": row.timePtr("verification_checked_at"),
		})
	}
	brandOpts := make([]map[string]any, 0, len(brands))
	for _, b := range brands {
		brandOpts = append(brandOpts, map[string]any{"id": b.strOr("id", ""), "name": b.strOr("name", ""), "public_id": ptNullableStr(b, "public_id")})
	}
	campaignOpts := make([]map[string]any, 0, len(campaigns))
	for _, c := range campaigns {
		campaignOpts = append(campaignOpts, map[string]any{"id": c.strOr("id", ""), "name": c.strOr("name", ""), "brand_id": c.strOr("brand_id", "")})
	}
	mediaOpts := make([]map[string]any, 0, len(mediaRows))
	for _, m := range mediaRows {
		mediaOpts = append(mediaOpts, map[string]any{"id": m.strOr("id", ""), "name": m.strOr("name", ""), "partner_id": ptNullableStr(m, "partner_id")})
	}

	total := 0
	if countRow != nil {
		total = countRow.intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rows": rowsOut, "total": total, "limit": limit, "offset": offset,
		"filters": map[string]any{
			"brands": brandOpts, "campaigns": campaignOpts, "media": mediaOpts,
			"message_statuses":    prMessageStatuses,
			"compliance_statuses": prComplianceStatuses,
		},
	})
}

// PRBrandStandards: GET /v1/platform/pr/brand-standards
func (d *Deps) PRBrandStandards(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	items, err := d.queryRows(r.Context(), `
		SELECT b.id, b.name, b.public_id, b.status,
		       s.rate_per_1k_reach, s.rate_tier1, s.rate_tier2, s.rate_tier3,
		       s.updated_at, s.updated_by,
		       u.display_name AS updated_by_name, u.email AS updated_by_email
		FROM brands b
		LEFT JOIN brand_pr_standards s ON s.brand_id = b.id
		LEFT JOIN users u ON u.id = s.updated_by
		WHERE b.deleted_at IS NULL
		ORDER BY b.name ASC`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	brandsOut := make([]map[string]any, 0, len(items))
	for _, row := range items {
		hasAny := row.has("rate_per_1k_reach") || row.has("rate_tier1") || row.has("rate_tier2") || row.has("rate_tier3")
		var standard any
		if hasAny {
			standard = map[string]any{
				"rate_per_1k_reach": row.numPtr("rate_per_1k_reach"),
				"rate_tier1":        row.numPtr("rate_tier1"),
				"rate_tier2":        row.numPtr("rate_tier2"),
				"rate_tier3":        row.numPtr("rate_tier3"),
				"updated_at":        row.timePtr("updated_at"),
				"updated_by":        prRowAuditUser(row),
			}
		}
		brandsOut = append(brandsOut, map[string]any{
			"id": row.strOr("id", ""), "name": row.strOr("name", ""),
			"public_id": ptNullableStr(row, "public_id"), "status": row.strOr("status", ""),
			"standard": standard,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"brands": brandsOut})
}

// PRBrandStandardUpdate: PUT /v1/platform/pr/brand-standards/{brandId}
func (d *Deps) PRBrandStandardUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	brandParam := chiParam(r, "brandId")
	brandRow, err := d.queryRow(ctx,
		"SELECT id FROM brands WHERE (id = $1 OR public_id = $2) AND deleted_at IS NULL LIMIT 1", brandParam, brandParam)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if brandRow == nil {
		writeErr(w, d, errNotFound("Brand not found"))
		return
	}
	canonicalID := brandRow.strOr("id", "")
	body, okBody := aiSafeParseObject(w, d, r, "Rate harus bilangan bulat 0-10.000.000 atau kosong (pakai global)")
	if !okBody {
		return
	}

	rate, _, err := prRateField(body, "rate_per_1k_reach")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	tier1, _, err := prRateField(body, "rate_tier1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	tier2, _, err := prRateField(body, "rate_tier2")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	tier3, _, err := prRateField(body, "rate_tier3")
	if err != nil {
		writeErr(w, d, err)
		return
	}

	if err := d.exec(ctx,
		`INSERT INTO brand_pr_standards (brand_id, rate_per_1k_reach, rate_tier1, rate_tier2, rate_tier3, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (brand_id) DO UPDATE SET
		   rate_per_1k_reach = EXCLUDED.rate_per_1k_reach,
		   rate_tier1 = EXCLUDED.rate_tier1,
		   rate_tier2 = EXCLUDED.rate_tier2,
		   rate_tier3 = EXCLUDED.rate_tier3,
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		canonicalID, rate, tier1, tier2, tier3, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'pr.brand_standard_updated', 'brand_pr_standards', $4)",
		uuid.NewString(), canonicalID, user.ID, canonicalID); err != nil {
		writeErr(w, d, err)
		return
	}
	row, _ := d.queryRow(ctx,
		"SELECT rate_per_1k_reach, rate_tier1, rate_tier2, rate_tier3, updated_at FROM brand_pr_standards WHERE brand_id = $1 LIMIT 1", canonicalID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"standard": map[string]any{
			"rate_per_1k_reach": row.numPtr("rate_per_1k_reach"),
			"rate_tier1":        row.numPtr("rate_tier1"),
			"rate_tier2":        row.numPtr("rate_tier2"),
			"rate_tier3":        row.numPtr("rate_tier3"),
			"updated_at":        row.timePtr("updated_at"),
		},
	})
}

// prRateField: parity z.coerce.number().int().min(0).max(10_000_000).nullable().optional().
func prRateField(body map[string]any, key string) (any, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return nil, false, nil
	}
	// parity z.coerce.number(): string dikonversi Number() dulu.
	f, hasNum := v.(float64)
	if !hasNum {
		str, isStr := v.(string)
		if !isStr {
			return nil, true, errUnprocessable("Rate harus bilangan bulat 0-10.000.000 atau kosong (pakai global)")
		}
		trimmed := strings.TrimSpace(str)
		parsed, pErr := strconv.ParseFloat(trimmed, 64)
		if pErr != nil || trimmed == "" {
			return nil, true, errUnprocessable("Rate harus bilangan bulat 0-10.000.000 atau kosong (pakai global)")
		}
		f = parsed
	}
	if f != float64(int(f)) {
		return nil, true, errUnprocessable("Rate harus bilangan bulat 0-10.000.000 atau kosong (pakai global)")
	}
	n := int(f)
	if n < 0 || n > 10000000 {
		return nil, true, errUnprocessable("Rate harus bilangan bulat 0-10.000.000 atau kosong (pakai global)")
	}
	return n, true, nil
}

// PRConfigGet: GET /v1/platform/pr/config
func (d *Deps) PRConfigGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT rate_per_1k_reach, rate_tier1, rate_tier2, rate_tier3, updated_by, updated_at FROM pr_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	rate := 5000
	if row != nil {
		if v := row.intOr("rate_per_1k_reach", 0); v != 0 {
			rate = v
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rate_per_1k_reach": rate,
		"rate_tier1":        row.numPtr("rate_tier1"),
		"rate_tier2":        row.numPtr("rate_tier2"),
		"rate_tier3":        row.numPtr("rate_tier3"),
		"updated_at":        row.timePtr("updated_at"),
	})
}

// PRConfigPut: PUT /v1/platform/pr/config
func (d *Deps) PRConfigPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiSafeParseObject(w, d, r, "rate_per_1k_reach (0-10.000.000) wajib valid; rate tier boleh kosong (pakai global)")
	if !okBody {
		return
	}
	rate, hasRate, err := prRateField(body, "rate_per_1k_reach")
	if err != nil || !hasRate {
		writeErr(w, d, errUnprocessable("rate_per_1k_reach (0-10.000.000) wajib valid; rate tier boleh kosong (pakai global)"))
		return
	}
	tier1, _, err := prRateField(body, "rate_tier1")
	if err != nil {
		writeErr(w, d, errUnprocessable("rate_per_1k_reach (0-10.000.000) wajib valid; rate tier boleh kosong (pakai global)"))
		return
	}
	tier2, _, err := prRateField(body, "rate_tier2")
	if err != nil {
		writeErr(w, d, errUnprocessable("rate_per_1k_reach (0-10.000.000) wajib valid; rate tier boleh kosong (pakai global)"))
		return
	}
	tier3, _, err := prRateField(body, "rate_tier3")
	if err != nil {
		writeErr(w, d, errUnprocessable("rate_per_1k_reach (0-10.000.000) wajib valid; rate tier boleh kosong (pakai global)"))
		return
	}
	if err := d.exec(ctx,
		`INSERT INTO pr_config (id, rate_per_1k_reach, rate_tier1, rate_tier2, rate_tier3, updated_by, updated_at)
		 VALUES ('default', $1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   rate_per_1k_reach = EXCLUDED.rate_per_1k_reach,
		   rate_tier1 = EXCLUDED.rate_tier1,
		   rate_tier2 = EXCLUDED.rate_tier2,
		   rate_tier3 = EXCLUDED.rate_tier3,
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		rate, tier1, tier2, tier3, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, NULL, $2, 'pr.config_updated', 'pr_config', 'default')",
		uuid.NewString(), user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PRMediaRulesGet: GET /v1/platform/pr/media-rules
func (d *Deps) PRMediaRulesGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT tier_default_quality, category_relevance, updated_at FROM pr_media_rules WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// parity: `row ? parseNumberMap(row.x) : DEFAULT_RULES.x` — baris ada tapi
	// objeknya kosong dikembalikan apa adanya (bukan default).
	tierQuality := map[string]float64{"tier1": 90, "tier2": 80, "tier3": 70}
	categoryRelevance := map[string]float64{}
	if row != nil {
		tierQuality = prParseJSONNumbers(row.jsonField("tier_default_quality"))
		categoryRelevance = prParseJSONNumbers(row.jsonField("category_relevance"))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tier_default_quality": prNumMapJSON(tierQuality),
		"category_relevance":   prNumMapJSON(categoryRelevance),
		"updated_at":           row.timePtr("updated_at"),
	})
}

// prNumMapJSON: map[string]float64 → map[string]any (nilai = angka atau null).
func prNumMapJSON(m map[string]float64) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// PRMediaRulesPut: PUT /v1/platform/pr/media-rules
func (d *Deps) PRMediaRulesPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiSafeParseObject(w, d, r,
		"tier_default_quality (tier1-3, 0-100) dan category_relevance (kategori → 0-100) wajib valid")
	if !okBody {
		return
	}
	tierRaw, hasTier, _ := aiObjectField(body, "tier_default_quality")
	categoryRaw, hasCategory, _ := aiObjectField(body, "category_relevance")
	if !hasTier || !hasCategory {
		writeErr(w, d, errUnprocessable("tier_default_quality (tier1-3, 0-100) dan category_relevance (kategori → 0-100) wajib valid"))
		return
	}
	invalidRules := errUnprocessable("tier_default_quality (tier1-3, 0-100) dan category_relevance (kategori → 0-100) wajib valid")
	tierQuality := map[string]any{}
	for _, key := range []string{"tier1", "tier2", "tier3"} {
		v, present := tierRaw[key]
		if !present {
			writeErr(w, d, invalidRules)
			return
		}
		n, okNum := prCoercePercent(v)
		if !okNum {
			writeErr(w, d, invalidRules)
			return
		}
		tierQuality[key] = n
	}
	if len(categoryRaw) > 100 {
		writeErr(w, d, invalidRules)
		return
	}
	categoryRelevance := map[string]any{}
	for key, v := range categoryRaw {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || utf8.RuneCountInString(trimmed) > 60 {
			writeErr(w, d, invalidRules)
			return
		}
		n, okNum := prCoercePercent(v)
		if !okNum {
			writeErr(w, d, invalidRules)
			return
		}
		categoryRelevance[trimmed] = n
	}

	tierJSON, _ := json.Marshal(tierQuality)
	categoryJSON, _ := json.Marshal(categoryRelevance)
	if err := d.exec(ctx,
		`INSERT INTO pr_media_rules (id, tier_default_quality, category_relevance, updated_by, updated_at)
		 VALUES ('default', $1::jsonb, $2::jsonb, $3, NOW())
		 ON CONFLICT (id) DO UPDATE SET tier_default_quality = EXCLUDED.tier_default_quality,
		   category_relevance = EXCLUDED.category_relevance, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		string(tierJSON), string(categoryJSON), user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, NULL, $2, 'pr.media_rules_updated', 'pr_media_rules', 'default')",
		uuid.NewString(), user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PRRecomputeScores: POST /v1/platform/pr/recompute-scores
func (d *Deps) PRRecomputeScores(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiSafeParseObject(w, d, r, "campaign_id (opsional) tidak valid")
	if !okBody {
		return
	}
	campaignID, hasCampaign, _ := aiStringField(body, "campaign_id", 1, 0, false, true)

	type campaignRef struct{ ID string }
	campaigns := []campaignRef{}
	if hasCampaign && campaignID != "" {
		campaigns = append(campaigns, campaignRef{ID: campaignID})
	} else {
		rows, qErr := d.queryRows(ctx,
			"SELECT DISTINCT campaign_id AS id FROM publications WHERE campaign_id IS NOT NULL AND status <> 'cancelled' ORDER BY campaign_id LIMIT 200")
		if qErr != nil {
			writeErr(w, d, qErr)
			return
		}
		for _, row := range rows {
			campaigns = append(campaigns, campaignRef{ID: row.strOr("id", "")})
		}
	}

	results := make([]map[string]any, 0, len(campaigns))
	for _, c := range campaigns {
		result := d.prCalculateCampaignScore(ctx, c.ID, true, user.ID)
		var score any
		status := "not_ready"
		if result != nil {
			if result.Score != nil {
				score = *result.Score
			}
			status = result.Status
		}
		results = append(results, map[string]any{"campaign_id": c.ID, "score": score, "status": status})
	}
	writeJSON(w, http.StatusOK, map[string]any{"recalculated": len(results), "results": results})
}

// PRRerunAnalysis: POST /v1/platform/pr/rerun-analysis
func (d *Deps) PRRerunAnalysis(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiSafeParseObject(w, d, r, "campaign_id (opsional) dan all (boolean opsional) tidak valid")
	if !okBody {
		return
	}
	campaignID, hasCampaign, cErr := aiStringField(body, "campaign_id", 1, 0, false, true)
	all, _, allErr := aiBoolField(body, "all")
	if cErr != nil || allErr != nil {
		writeErr(w, d, errUnprocessable("campaign_id (opsional) dan all (boolean opsional) tidak valid"))
		return
	}

	where := []string{"p.status = 'published'"}
	params := []any{}
	if hasCampaign && campaignID != "" {
		params = append(params, campaignID)
		where = append(where, "p.campaign_id = $"+ptItoa(len(params)))
	}
	if !all {
		where = append(where, `(
			(EXISTS (SELECT 1 FROM content_intakes ci
			          WHERE ci.job_id = (SELECT job_id FROM content_documents d WHERE d.id = p.document_id)
			            AND jsonb_array_length(COALESCE(ci.key_messages, '[]'::jsonb)) > 0)
			 AND NOT EXISTS (SELECT 1 FROM pr_message_analyses ma WHERE ma.publication_id = p.id))
			OR NOT EXISTS (SELECT 1 FROM pr_sentiment_analyses sa WHERE sa.publication_id = p.id)
			OR NOT EXISTS (SELECT 1 FROM pr_publication_verifications v WHERE v.publication_id = p.id))`)
	}
	pubs, err := d.queryRows(ctx,
		"SELECT p.id FROM publications p WHERE "+strings.Join(where, " AND ")+" ORDER BY p.created_at LIMIT 500", params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	queued := 0
	for _, pub := range pubs {
		id := pub.strOr("id", "")
		d.mpEnqueueJobBestEffort(ctx, "pr_analysis", map[string]any{"publicationId": id}, 2)
		d.mpEnqueueJobBestEffort(ctx, "pr_verify", map[string]any{"publicationId": id}, 2)
		queued++
	}
	writeJSON(w, http.StatusOK, map[string]any{"queued": queued, "total": len(pubs)})
}

// PRScoringProfiles: GET /v1/platform/pr/scoring-profiles
func (d *Deps) PRScoringProfiles(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	profiles, err := d.queryRows(r.Context(),
		"SELECT id, version, name, weights, thresholds, status, created_by, created_at, updated_at FROM pr_scoring_profiles ORDER BY version DESC")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(profiles))
	for _, row := range profiles {
		out = append(out, map[string]any{
			"id":         row.strOr("id", ""),
			"version":    row.intOr("version", 0),
			"name":       row.strOr("name", ""),
			"weights":    prNumMapJSON(prParseJSONNumbers(row.jsonField("weights"))),
			"thresholds": prNumMapJSON(prParseJSONNumbers(row.jsonField("thresholds"))),
			"status":     row.strOr("status", ""),
			"created_at": row.timePtr("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": out})
}

// PRScoringProfileCreate: POST /v1/platform/pr/scoring-profiles
// (profil versi baru, status 'draft'; bobot wajib total 100%).
func (d *Deps) PRScoringProfileCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiSafeParseObject(w, d, r, "Bobot wajib diisi untuk kelima indikator (angka 0–100)")
	if !okBody {
		return
	}
	const invalidWeights = "Bobot wajib diisi untuk kelima indikator (angka 0–100)"
	name, hasName, nameErr := aiStringField(body, "name", 2, 120, true, false)
	weightsRaw, hasWeights, weightsErr := aiObjectField(body, "weights")
	if nameErr != nil || weightsErr != nil || !hasName || !hasWeights {
		writeErr(w, d, errUnprocessable(invalidWeights))
		return
	}
	weights := map[string]float64{}
	for k, v := range prDefaultWeights {
		weights[k] = v
	}
	for _, key := range prWeightKeys {
		v, present := weightsRaw[key]
		if !present {
			writeErr(w, d, errUnprocessable(invalidWeights))
			return
		}
		n, okWeight := prCoercePercent(v)
		if !okWeight {
			writeErr(w, d, errUnprocessable(invalidWeights))
			return
		}
		weights[key] = float64(n)
	}
	total := 0.0
	for _, key := range prWeightKeys {
		total += weights[key]
	}
	if mathAbs(total-100) > 0.01 {
		writeErr(w, d, errUnprocessable("Total bobot harus 100% (sekarang "+prFormatPercent(total)+"%)"))
		return
	}
	thresholds := map[string]float64{}
	for k, v := range prDefaultThresholds {
		thresholds[k] = v
	}
	thresholdsRaw, hasThresholds, thresholdsErr := aiObjectField(body, "thresholds")
	if thresholdsErr != nil {
		writeErr(w, d, errUnprocessable(invalidWeights))
		return
	}
	if hasThresholds {
		for k, v := range thresholdsRaw {
			n, okThreshold := prCoercePercent(v)
			if !okThreshold {
				writeErr(w, d, errUnprocessable(invalidWeights))
				return
			}
			thresholds[k] = float64(n)
		}
	}

	maxRow, err := d.queryRow(ctx, "SELECT COALESCE(MAX(version), 0)::int AS version FROM pr_scoring_profiles")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	version := maxRow.intOr("version", 0) + 1
	id := uuid.NewString()
	weightsJSON, _ := json.Marshal(weights)
	thresholdsJSON, _ := json.Marshal(thresholds)
	if err := d.exec(ctx,
		"INSERT INTO pr_scoring_profiles (id, version, name, weights, thresholds, status, created_by) VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, 'draft', $6)",
		id, version, name, string(weightsJSON), string(thresholdsJSON), user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	created, _ := d.queryRow(ctx, "SELECT * FROM pr_scoring_profiles WHERE id = $1 LIMIT 1", id)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         created.strOr("id", id),
		"version":    created.intOr("version", version),
		"name":       created.strOr("name", name),
		"weights":    prNumMapJSON(prParseJSONNumbers(created.jsonField("weights"))),
		"thresholds": prNumMapJSON(prParseJSONNumbers(created.jsonField("thresholds"))),
		"status":     created.strOr("status", "draft"),
	})
}

// PRScoringProfileActivate: POST /v1/platform/pr/scoring-profiles/{profileId}/activate
func (d *Deps) PRScoringProfileActivate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	profileID := chiParam(r, "profileId")
	profile, err := d.queryRow(ctx, "SELECT id, status FROM pr_scoring_profiles WHERE id = $1 LIMIT 1", profileID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if profile == nil {
		writeErr(w, d, errNotFound("Scoring profile not found"))
		return
	}
	txErr := d.ptWithTx(ctx, func(tx *ptTx) error {
		if err := tx.exec("UPDATE pr_scoring_profiles SET status = 'archived', updated_at = NOW() WHERE status = 'active'"); err != nil {
			return err
		}
		return tx.exec("UPDATE pr_scoring_profiles SET status = 'active', updated_at = NOW() WHERE id = $1", profileID)
	})
	if txErr != nil {
		writeErr(w, d, txErr)
		return
	}
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, NULL, $2, 'pr.scoring_profile_activated', 'pr_scoring_profile', $3)",
		uuid.NewString(), user.ID, profileID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "profile_id": profileID})
}

// prCoercePercent: parity z.coerce.number().int().min(0).max(100) untuk nilai
// dari JSON (angka atau string numerik).
func prCoercePercent(v any) (int, bool) {
	f, isNum := v.(float64)
	if !isNum {
		str, isStr := v.(string)
		if !isStr {
			return 0, false
		}
		parsed, err := aiParseFloat(strings.TrimSpace(str))
		if err != nil {
			return 0, false
		}
		f = parsed
	}
	if f != float64(int(f)) || f < 0 || f > 100 {
		return 0, false
	}
	return int(f), true
}

// mathAbs: abs sederhana untuk pembanding total bobot.
func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// prFormatPercent: parity Math.round(total * 100) / 100 → "50", "99.5".
func prFormatPercent(v float64) string {
	rounded := float64(int(v*100+0.5)) / 100
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
