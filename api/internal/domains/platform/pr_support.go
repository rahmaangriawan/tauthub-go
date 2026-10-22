package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Port lib/pr/scoring.ts + lib/pr/metrics.ts (dipakai recompute-scores) dan
// helper CSV analysis-history. Awalan `pr` supaya tidak bentrok dengan helper
// modul platform lain yang berbagi package.

// ---------- konstanta scoring ----------

// prDefaultWeights: parity DEFAULT_PR_WEIGHTS (urutan kunci = WEIGHT_KEYS).
var prWeightKeys = []string{"publication", "coverage", "message", "sentiment", "compliance"}

var prDefaultWeights = map[string]float64{
	"publication": 25, "coverage": 30, "message": 20, "sentiment": 15, "compliance": 10,
}

var prDefaultThresholds = map[string]float64{
	"excellent": 85, "strong": 70, "good": 50, "needs_improvement": 0,
}

// prParseJSONNumbers: parity parseJson/parseNumberMap — objek JSON → angka
// finite saja (string JSON juga diterima).
func prParseJSONNumbers(value any) map[string]float64 {
	out := map[string]float64{}
	var raw any = value
	if s, isString := value.(string); isString {
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			return out
		}
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	for k, v := range obj {
		var f float64
		switch t := v.(type) {
		case float64:
			f = t
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
			if err != nil {
				continue
			}
			f = parsed
		default:
			continue
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		out[k] = f
	}
	return out
}

// prParseWeights: parity parseWeights — mulai dari default, timpa yang finite.
func prParseWeights(value any) map[string]float64 {
	raw := prParseJSONNumbers(value)
	out := map[string]float64{}
	for k, v := range prDefaultWeights {
		out[k] = v
	}
	for _, key := range prWeightKeys {
		if v, ok := raw[key]; ok {
			out[key] = v
		}
	}
	return out
}

// prParseThresholds: parity parseThresholds — merge dengan default.
func prParseThresholds(value any) map[string]float64 {
	raw := prParseJSONNumbers(value)
	out := map[string]float64{}
	for k, v := range prDefaultThresholds {
		out[k] = v
	}
	for k, v := range raw {
		out[k] = v
	}
	return out
}

// prScoreLabel: parity scoreLabel.
func prScoreLabel(score *float64, thresholds map[string]float64) any {
	if score == nil {
		return nil
	}
	excellent := prThresholdOr(thresholds, "excellent", 85)
	strong := prThresholdOr(thresholds, "strong", 70)
	good := prThresholdOr(thresholds, "good", 50)
	switch {
	case *score >= excellent:
		return "Excellent"
	case *score >= strong:
		return "Strong Performance"
	case *score >= good:
		return "Good"
	default:
		return "Needs Improvement"
	}
}

func prThresholdOr(thresholds map[string]float64, key string, def float64) float64 {
	if v, ok := thresholds[key]; ok {
		return v
	}
	return def
}

// ---------- metrik level 1 (lib/pr/metrics.ts) ----------

type prMetrics struct {
	TargetMedia             int
	Published               int
	Pending                 int
	Failed                  int
	Cancelled               int
	SuccessRate             *float64
	AvgPublishHours         *float64
	PublishedPublicationIDs []string
}

// prComputeMetrics: parity computePublicationMetrics.
func (d *Deps) prComputeMetrics(ctx context.Context, campaignID string) prMetrics {
	var m prMetrics
	row, err := d.queryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status <> 'cancelled')::int AS target_media,
		  COUNT(*) FILTER (WHERE status = 'published')::int AS published,
		  COUNT(*) FILTER (WHERE status IN ('pending','waiting_partner','partner_approved','queued','processing','retrying'))::int AS pending,
		  COUNT(*) FILTER (WHERE status = 'failed')::int AS failed,
		  COUNT(*) FILTER (WHERE status = 'cancelled')::int AS cancelled,
		  ROUND(AVG(CASE WHEN published_at IS NOT NULL THEN EXTRACT(EPOCH FROM (published_at - created_at)) / 3600.0 END), 1)::numeric AS avg_publish_hours
		FROM publications
		WHERE campaign_id = $1`, campaignID)
	if err == nil && row != nil {
		m.TargetMedia = row.intOr("target_media", 0)
		m.Published = row.intOr("published", 0)
		m.Pending = row.intOr("pending", 0)
		m.Failed = row.intOr("failed", 0)
		m.Cancelled = row.intOr("cancelled", 0)
		if row.has("avg_publish_hours") {
			v := row.numOr("avg_publish_hours")
			m.AvgPublishHours = &v
		}
	}
	if m.TargetMedia > 0 {
		v := math.Round(float64(m.Published)/float64(m.TargetMedia)*10000) / 100
		m.SuccessRate = &v
	}
	pubs, err := d.queryRows(ctx,
		"SELECT id FROM publications WHERE campaign_id = $1 AND status = 'published' ORDER BY published_at ASC", campaignID)
	if err == nil {
		for _, p := range pubs {
			m.PublishedPublicationIDs = append(m.PublishedPublicationIDs, p.strOr("id", ""))
		}
	}
	return m
}

// prPersistMetrics: parity persistPrMetrics — batch upsert, null tidak ditulis.
func (d *Deps) prPersistMetrics(ctx context.Context, campaignID string, m prMetrics) {
	entries := []struct {
		Key    string
		Value  any
		Source string
	}{
		{"target_media", m.TargetMedia, "Publication records"},
		{"published", m.Published, "Publication records"},
		{"pending", m.Pending, "Publication records"},
		{"failed", m.Failed, "Publication records"},
		{"cancelled", m.Cancelled, "Publication records"},
		{"success_rate", m.SuccessRate, "Publication records"},
		{"avg_publish_hours", m.AvgPublishHours, "Publication records"},
	}
	present := make([]struct {
		Key    string
		Value  any
		Source string
	}, 0, len(entries))
	for _, e := range entries {
		if e.Value == nil {
			continue
		}
		present = append(present, e)
	}
	if len(present) == 0 {
		return
	}
	values := make([]string, 0, len(present))
	params := []any{}
	for i, e := range present {
		base := i*5 + 1
		values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", base, base+1, base+2, base+3, base+4))
		params = append(params, uuid.NewString(), campaignID, e.Key, e.Value, e.Source)
	}
	_ = d.exec(ctx,
		`INSERT INTO pr_metric_results (id, campaign_id, metric_key, metric_value, source)
		 VALUES `+strings.Join(values, ", ")+`
		 ON CONFLICT (campaign_id, metric_key)
		 DO UPDATE SET metric_value = EXCLUDED.metric_value, source = EXCLUDED.source, calculated_at = NOW()`,
		params...)
}

// ---------- profil aktif ----------

// prGetActiveScoringProfile: parity getActiveScoringProfile — seed baseline v1
// bila belum ada profil aktif.
func (d *Deps) prGetActiveScoringProfile(ctx context.Context) rowMap {
	row, _ := d.queryRow(ctx, "SELECT * FROM pr_scoring_profiles WHERE status = 'active' ORDER BY version DESC LIMIT 1")
	if row != nil {
		return row
	}
	id := uuid.NewString()
	weights, _ := json.Marshal(prDefaultWeights)
	thresholds, _ := json.Marshal(prDefaultThresholds)
	if err := d.exec(ctx,
		"INSERT INTO pr_scoring_profiles (id, version, name, weights, thresholds) VALUES ($1, 1, 'Baseline v1', $2::jsonb, $3::jsonb)",
		id, string(weights), string(thresholds)); err != nil {
		return nil
	}
	created, _ := d.queryRow(ctx, "SELECT * FROM pr_scoring_profiles WHERE id = $1 LIMIT 1", id)
	return created
}

// ---------- indikator level 2 ----------

type prIndicator struct {
	Key    string
	Label  string
	Score  *float64
	Weight float64
	Source string
	Detail map[string]any
}

func prAvg(scores []float64) *float64 {
	if len(scores) == 0 {
		return nil
	}
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	v := math.Round(sum/float64(len(scores))*100) / 100
	return &v
}

// prCoverageQuality: avg media_quality_score media yang terbit.
func (d *Deps) prCoverageQuality(ctx context.Context, ids []string) *prIndicator {
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.queryRows(ctx,
		`SELECT m.media_quality_score, m.media_tier
		 FROM publications p JOIN media m ON m.id = p.media_id
		 WHERE p.id = ANY($1)`, ids)
	if err != nil {
		return nil
	}
	scores := make([]float64, 0, len(rows))
	tiers := map[string]int{}
	for _, r := range rows {
		if r.has("media_quality_score") {
			scores = append(scores, r.numOr("media_quality_score"))
		}
		tier := aiOrEmpty(r.strOr("media_tier", ""), "tier3")
		tiers[tier]++
	}
	avg := prAvg(scores)
	if avg == nil {
		return nil
	}
	detail := map[string]any{"media_quality": scores, "published_with_quality": len(scores), "total_published": len(ids)}
	tierAny := map[string]any{}
	for k, v := range tiers {
		tierAny[k] = v
	}
	detail["tier_breakdown"] = tierAny
	return &prIndicator{Key: "coverage", Label: "Coverage Quality", Score: avg, Source: "Media Profile (published media)", Detail: detail}
}

// prMessageConsistency: avg consistency_score analisis yang completed.
func (d *Deps) prMessageConsistency(ctx context.Context, ids []string) *prIndicator {
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.queryRows(ctx,
		`SELECT consistency_score FROM pr_message_analyses
		 WHERE publication_id = ANY($1) AND status = 'completed'`, ids)
	if err != nil {
		return nil
	}
	scores := make([]float64, 0, len(rows))
	for _, r := range rows {
		if r.has("consistency_score") {
			scores = append(scores, r.numOr("consistency_score"))
		}
	}
	avg := prAvg(scores)
	if avg == nil {
		return nil
	}
	return &prIndicator{Key: "message", Label: "Message Consistency", Score: avg,
		Source: "Key Messages + Published Article Analysis",
		Detail: map[string]any{"analyzed": len(scores), "total": len(ids), "consistency_scores": scores}}
}

// prCompliance: avg compliance_score verifikasi passed/failed.
func (d *Deps) prCompliance(ctx context.Context, ids []string) *prIndicator {
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.queryRows(ctx,
		`SELECT compliance_score, status FROM pr_publication_verifications
		 WHERE publication_id = ANY($1) AND status IN ('passed','failed')`, ids)
	if err != nil {
		return nil
	}
	scores := make([]float64, 0, len(rows))
	passed, failed := 0, 0
	for _, r := range rows {
		if r.has("compliance_score") {
			scores = append(scores, r.numOr("compliance_score"))
		}
		switch r.strOr("status", "") {
		case "passed":
			passed++
		case "failed":
			failed++
		}
	}
	avg := prAvg(scores)
	if avg == nil {
		return nil
	}
	return &prIndicator{Key: "compliance", Label: "Publication Compliance", Score: avg,
		Source: "Publication Verification (fetch URL terbit)",
		Detail: map[string]any{"verified": len(scores), "passed": passed, "failed": failed, "total": len(ids)}}
}

// prSentiment: tone share positif/netral/negatif.
func (d *Deps) prSentiment(ctx context.Context, ids []string) *prIndicator {
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.queryRows(ctx,
		`SELECT sentiment, COUNT(*)::int AS count FROM pr_sentiment_analyses
		 WHERE publication_id = ANY($1) AND status = 'completed' AND sentiment IS NOT NULL
		 GROUP BY sentiment`, ids)
	if err != nil {
		return nil
	}
	counts := map[string]int{"positive": 0, "neutral": 0, "negative": 0}
	total := 0
	for _, r := range rows {
		s := r.strOr("sentiment", "")
		c := r.intOr("count", 0)
		if _, ok := counts[s]; ok {
			counts[s] = c
		}
		total += c
	}
	if total == 0 {
		return nil
	}
	score := math.Round((float64(counts["positive"])*100+float64(counts["neutral"])*60+float64(counts["negative"])*20)/float64(total)*100) / 100
	return &prIndicator{Key: "sentiment", Label: "Sentiment / Tone", Score: &score,
		Source: "Published Article Analysis (coverage tone)",
		Detail: map[string]any{"positive": counts["positive"], "neutral": counts["neutral"], "negative": counts["negative"], "analyzed": total}}
}

// ---------- engine ----------

type prScoreResult struct {
	CampaignID     string
	ProfileVersion int
	ProfileName    string
	Weights        map[string]float64
	Status         string
	Score          *float64
	Label          any
	Indicators     []map[string]any
}

// prCalculateCampaignScore: port calculateCampaignPrScore (recalc=true dari
// endpoint admin; reuse snapshot bila recalc=false dan sudah ada).
func (d *Deps) prCalculateCampaignScore(ctx context.Context, campaignID string, recalc bool, userID string) *prScoreResult {
	profile := d.prGetActiveScoringProfile(ctx)
	profileVersion := 1
	var profileID string
	var weights map[string]float64
	var thresholds map[string]float64
	if profile != nil {
		profileVersion = profile.intOr("version", 1)
		profileID = profile.strOr("id", "")
		weights = prParseWeights(profile.jsonField("weights"))
		thresholds = prParseThresholds(profile.jsonField("thresholds"))
	} else {
		weights = prParseWeights(nil)
		thresholds = prParseThresholds(nil)
	}

	if !recalc {
		existing, _ := d.queryRow(ctx,
			"SELECT * FROM pr_campaign_scores WHERE campaign_id = $1 AND profile_version = $2 LIMIT 1",
			campaignID, profileVersion)
		if existing != nil {
			return prBuildScoreResponse(existing, weights, thresholds)
		}
	}

	metrics := d.prComputeMetrics(ctx, campaignID)
	d.prPersistMetrics(ctx, campaignID, metrics)

	if metrics.TargetMedia == 0 {
		indicators, _ := json.Marshal([]any{})
		inputs, _ := json.Marshal(map[string]any{"target_media": 0})
		weightsJSON, _ := json.Marshal(weights)
		d.prUpsertCampaignScore(ctx, campaignID, profileID, profileVersion, string(weightsJSON),
			"not_ready", nil, nil, string(indicators), string(inputs), userID)
		existing, _ := d.queryRow(ctx,
			"SELECT campaign_id, profile_version, STATUS, score, label, breakdown, calculated_at FROM pr_campaign_scores WHERE campaign_id = $1 AND profile_version = $2 LIMIT 1",
			campaignID, profileVersion)
		if existing == nil {
			return &prScoreResult{CampaignID: campaignID, ProfileVersion: profileVersion,
				Status: "not_ready", Weights: weights, Indicators: []map[string]any{}}
		}
		return prBuildScoreResponse(existing, weights, thresholds)
	}

	indicators := []*prIndicator{}
	if metrics.SuccessRate != nil {
		indicators = append(indicators, &prIndicator{
			Key: "publication", Label: "Publication Performance", Score: metrics.SuccessRate,
			Weight: weights["publication"], Source: "Publication records",
			Detail: map[string]any{
				"published": metrics.Published, "target": metrics.TargetMedia,
				"failed": metrics.Failed, "pending": metrics.Pending,
				"avg_publish_hours": metrics.AvgPublishHours,
			},
		})
	}
	if cov := d.prCoverageQuality(ctx, metrics.PublishedPublicationIDs); cov != nil {
		cov.Weight = weights["coverage"]
		indicators = append(indicators, cov)
	}
	if msg := d.prMessageConsistency(ctx, metrics.PublishedPublicationIDs); msg != nil {
		msg.Weight = weights["message"]
		indicators = append(indicators, msg)
	}
	if sent := d.prSentiment(ctx, metrics.PublishedPublicationIDs); sent != nil {
		sent.Weight = weights["sentiment"]
		indicators = append(indicators, sent)
	}
	if comp := d.prCompliance(ctx, metrics.PublishedPublicationIDs); comp != nil {
		comp.Weight = weights["compliance"]
		indicators = append(indicators, comp)
	}

	usedWeight := 0.0
	for _, ind := range indicators {
		usedWeight += ind.Weight
	}
	var score *float64
	if usedWeight > 0 {
		sum := 0.0
		for _, ind := range indicators {
			if ind.Score != nil {
				sum += *ind.Score * ind.Weight
			}
		}
		v := math.Round(sum/usedWeight*100) / 100
		score = &v
	}
	status := "final"
	if metrics.Pending > 0 {
		status = "partial"
	}

	indicatorJSON := make([]map[string]any, 0, len(indicators))
	for _, ind := range indicators {
		indicatorJSON = append(indicatorJSON, prIndicatorJSON(ind))
	}
	breakdown, _ := json.Marshal(map[string]any{"indicators": indicatorJSON})
	inputs, _ := json.Marshal(map[string]any{"metrics_summary": map[string]any{
		"target": metrics.TargetMedia, "published": metrics.Published,
		"pending": metrics.Pending, "failed": metrics.Failed, "cancelled": metrics.Cancelled,
	}})
	weightsJSON, _ := json.Marshal(weights)
	d.prUpsertCampaignScore(ctx, campaignID, profileID, profileVersion, string(weightsJSON),
		status, score, prScoreLabel(score, thresholds), string(breakdown), string(inputs), userID)

	existing, _ := d.queryRow(ctx,
		"SELECT campaign_id, profile_version, STATUS, score, label, breakdown, calculated_at FROM pr_campaign_scores WHERE campaign_id = $1 AND profile_version = $2 LIMIT 1",
		campaignID, profileVersion)
	if existing == nil {
		return &prScoreResult{CampaignID: campaignID, ProfileVersion: profileVersion,
			Status: status, Score: score, Weights: weights, Indicators: indicatorJSON}
	}
	return prBuildScoreResponse(existing, weights, thresholds)
}

func prIndicatorJSON(ind *prIndicator) map[string]any {
	out := map[string]any{
		"key": ind.Key, "label": ind.Label, "score": ind.Score,
		"weight": ind.Weight, "source": ind.Source,
	}
	if ind.Detail != nil {
		out["detail"] = ind.Detail
	}
	return out
}

func (d *Deps) prUpsertCampaignScore(ctx context.Context, campaignID, profileID string, version int,
	weights, status string, score *float64, label any, breakdown, inputSnapshot, userID string) {
	var profileArg any
	if profileID != "" {
		profileArg = profileID
	}
	var userArg any
	if userID != "" {
		userArg = userID
	}
	_ = d.exec(ctx,
		`INSERT INTO pr_campaign_scores (id, campaign_id, profile_id, profile_version, weights, status, score, label, breakdown, input_snapshot, calculated_by, calculated_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9::jsonb, $10::jsonb, $11, NOW(), NOW())
		 ON CONFLICT (campaign_id, profile_version)
		 DO UPDATE SET weights = EXCLUDED.weights, status = EXCLUDED.status, score = EXCLUDED.score, label = EXCLUDED.label,
		               breakdown = EXCLUDED.breakdown, input_snapshot = EXCLUDED.input_snapshot, calculated_by = EXCLUDED.calculated_by,
		               calculated_at = EXCLUDED.calculated_at, updated_at = NOW()`,
		uuid.NewString(), campaignID, profileArg, version, weights, status, score, label,
		breakdown, inputSnapshot, userArg)
}

// prBuildScoreResponse: parity buildScoreResponse (insights + methodology).
func prBuildScoreResponse(row rowMap, weights, thresholds map[string]float64) *prScoreResult {
	var breakdown struct {
		Indicators []map[string]any `json:"indicators"`
	}
	if raw := row.jsonField("breakdown"); raw != nil {
		if b, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(b, &breakdown)
		}
	}
	indicators := breakdown.Indicators
	if indicators == nil {
		indicators = []map[string]any{}
	}
	var score *float64
	if row.has("score") {
		v := row.numOr("score")
		score = &v
	}
	status := aiOrEmpty(row.strOr("status", ""), "not_ready")
	return &prScoreResult{
		CampaignID:     row.strOr("campaign_id", ""),
		ProfileVersion: row.intOr("profile_version", 0),
		ProfileName:    "Scoring v" + ptItoa(row.intOr("profile_version", 0)),
		Weights:        weights,
		Status:         status,
		Score:          score,
		Label:          row.raw("label"),
		Indicators:     indicators,
	}
}

func prNumFromAny(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	}
	return 0
}

// prScoreOf: skor indikator (nil = tidak tersedia).
func prScoreOf(ind map[string]any) (float64, bool) {
	v, ok := ind["score"]
	if !ok || v == nil {
		return 0, false
	}
	return prNumFromAny(v), true
}

// prAnalysisHistoryCsv: parity analysisHistoryCsv (BOM + CRLF, semua sel dikutip).
func prAnalysisHistoryCsv(items []rowMap) string {
	header := []string{"Publikasi ID", "Status", "Artikel", "Media", "Tier", "Brand", "Kampanye", "Terbit", "URL Terbit", "Message Score", "Message Status", "Message Error", "Sentiment", "Confidence", "Compliance Status", "Compliance Score", "Verifikasi Error", "Diverifikasi"}
	rowsOut := [][]string{header}
	for _, row := range items {
		rowsOut = append(rowsOut, []string{
			row.strOr("publication_id", ""), row.strOr("status", ""), row.strOr("document_title", ""),
			row.strOr("media_name", ""), row.strOr("media_tier", ""), row.strOr("brand_name", ""),
			row.strOr("campaign_name", ""), prCsvValue(row.raw("published_at")), prCsvValue(row.raw("published_url")),
			prCsvValue(row.raw("consistency_score")), prCsvValue(row.raw("message_status")), prCsvValue(row.raw("message_error")),
			prCsvValue(row.raw("sentiment")), prCsvValue(row.raw("sentiment_confidence")),
			prCsvValue(row.raw("compliance_status")), prCsvValue(row.raw("compliance_score")),
			prCsvValue(row.raw("verification_error")), prCsvValue(row.raw("verification_checked_at")),
		})
	}
	var b strings.Builder
	b.WriteString("\ufeff")
	for i, line := range rowsOut {
		if i > 0 {
			b.WriteString("\r\n")
		}
		cells := make([]string, len(line))
		for j, cell := range line {
			cells[j] = `"` + strings.ReplaceAll(cell, `"`, `""`) + `"`
		}
		b.WriteString(strings.Join(cells, ","))
	}
	return b.String()
}

// prCsvValue: parity String(value ?? "") pada nilai RowDataPacket. Timestamp
// ikut diserialkan ISO seperti JSON.stringify(Date) di app lama.
func prCsvValue(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case time.Time:
		// CSV lama memakai String(Date) (format lokal server). Go tidak bisa
		// mereproduksi locale proses Node; dipakai ISO yang deterministik.
		return isoMilli(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// prRowAuditUser: nama + email user yang terakhir mengubah standar brand.
func prRowAuditUser(row rowMap) any {
	if !row.has("updated_by") {
		return nil
	}
	return map[string]any{
		"id":    row.strOr("updated_by", ""),
		"name":  ptNullableStr(row, "updated_by_name"),
		"email": ptNullableStr(row, "updated_by_email"),
	}
}

// prISODateRe: bentuk tanggal yang diterima Date.parse JS (ISO/Indonesian "YYYY-MM-DD HH:mm:ss").
var prISODateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(?:[T ]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?)?$`)

// prIsISODate: parity `!Number.isNaN(Date.parse(x))`.
func prIsISODate(raw string) bool {
	return prISODateRe.MatchString(strings.TrimSpace(raw))
}

// prParseISODate: parse tanggal filter analysis-history.
func prParseISODate(raw string) (time.Time, bool) {
	v := strings.TrimSpace(raw)
	if !prISODateRe.MatchString(v) {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano, time.RFC3339,
		"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// aiParseFloat: konversi angka dari string body (parity z.coerce.number()).
func aiParseFloat(raw string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(raw), 64)
}
