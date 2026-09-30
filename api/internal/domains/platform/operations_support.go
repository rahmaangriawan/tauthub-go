// Berkas helper internal modul operations: port app lama
// app/v1/platform/operations/_shared (+ kredit wallet, audit, job, koneksi CMS)
// yang dipakai 32 handler Operations* di operations.go.
package platform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	partnerdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/partner"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// ---------- runtime info (parity _shared.providerInfo) ----------

// opProviderInfo: provider + configured dari runtime AI (db + env fallback).
func (d *Deps) opProviderInfo(ctx context.Context) (string, bool) {
	rt, err := ai.LoadAIRuntime(ctx, d.Pool)
	if err != nil || rt == nil {
		return "unknown", false
	}
	provider := strings.TrimSpace(rt.Provider)
	if provider == "" {
		provider = "unknown"
	}
	configured := rt.APIKey != "" && rt.BaseURL != "" && rt.DefaultModel != ""
	return provider, configured
}

// opGenerationDisabled: SELECT system_metadata platform_generation_disabled.
func (d *Deps) opGenerationDisabled(ctx context.Context) bool {
	row, err := d.queryRow(ctx,
		`SELECT value FROM system_metadata WHERE "key" = 'platform_generation_disabled' LIMIT 1`)
	if err != nil || row == nil {
		return false
	}
	return row.strOr("value", "") == "1"
}

// opSetGenerationDisabled: parity setGenerationDisabled.
func (d *Deps) opSetGenerationDisabled(ctx context.Context, disabled bool) error {
	value := "0"
	if disabled {
		value = "1"
	}
	return d.exec(ctx,
		`INSERT INTO system_metadata ("key", value) VALUES ($1, $2) ON CONFLICT ("key") DO UPDATE SET value = $2, updated_at = NOW()`,
		"platform_generation_disabled", value)
}

// ---------- kredit wallet (parity lib/credits/wallet.ts) ----------

// opWalletSummary: {available_tc,reserved_tc,captured_tc,total_tc,rate_idr,...}.
func (d *Deps) opWalletSummary(ctx context.Context, brandID string) map[string]any {
	available, reserved, captured := 0, 0, 0
	hasWallet := true
	if err := d.Pool.QueryRow(ctx,
		"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1 LIMIT 1", brandID).
		Scan(&available, &reserved, &captured); err != nil {
		hasWallet = false
	}
	rate := 1000
	_ = d.Pool.QueryRow(ctx, "SELECT rate_idr FROM tc_config WHERE id = 1 LIMIT 1").Scan(&rate)
	if rate < 1 {
		rate = 1000
	}
	return map[string]any{
		"available_tc": available, "reserved_tc": reserved, "captured_tc": captured,
		"total_tc": available + reserved + captured, "rate_idr": rate,
		"available_idr": available * rate, "has_wallet": hasWallet,
	}
}

// opAdjustCredit: parity adjustCredit — idempotent per key, saldo tak negatif.
func (d *Deps) opAdjustCredit(ctx context.Context, brandID string, tc int, reason, userID, idempotencyKey string) error {
	if tc == 0 {
		return fmt.Errorf("Adjustment amount must be a non-zero integer.")
	}
	rate := 1000
	_ = d.Pool.QueryRow(ctx, "SELECT rate_idr FROM tc_config WHERE id = 1 LIMIT 1").Scan(&rate)
	if rate < 1 {
		rate = 1000
	}
	if idempotencyKey == "" {
		idempotencyKey = "adjustment:" + uuid.NewString()
	}
	if reason == "" {
		reason = "Penyesuaian manual"
	}
	key := idempotencyKey
	return db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		var dup int
		if err := tx.QueryRow(ctx,
			"SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&dup); err == nil {
			return nil
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO credit_wallets (brand_id) VALUES ($1) ON CONFLICT (brand_id) DO NOTHING", brandID); err != nil {
			return err
		}
		var available, reserved, captured int
		err := tx.QueryRow(ctx,
			`UPDATE credit_wallets SET available_tc = available_tc + $1, updated_at = NOW()
			 WHERE brand_id = $2 AND available_tc + $1 >= 0
			 RETURNING available_tc, reserved_tc, captured_tc`,
			tc, brandID).Scan(&available, &reserved, &captured)
		if err != nil {
			avail := 0
			_ = tx.QueryRow(ctx, "SELECT available_tc FROM credit_wallets WHERE brand_id = $1", brandID).Scan(&avail)
			if avail < 0 {
				avail = 0
			}
			return fmt.Errorf("Insufficient credits: need %d, available %d", opAbsInt(tc), avail)
		}
		total := available + reserved + captured
		var amountIDR any
		if tc >= 0 {
			amountIDR = tc * rate
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO credit_ledger_entries (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr, amount_idr, reason, idempotency_key, created_by)
			 VALUES ($1, $2, 'adjustment', NULL, $3, $4, $5, $6, $7, $8, $9)`,
			uuid.NewString(), brandID, tc, total, rate, amountIDR, reason, key, userID)
		return err
	})
}

func opAbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---------- audit + job (parity audit/log, jobs/queue) ----------

// opAuditBrand: parity writeAuditLog — kolom brand_id terisi (helper audit di
// types.go tidak punya brand_id).
func (d *Deps) opAuditBrand(ctx context.Context, brandID, actorUserID, action, targetType, targetID string) {
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)",
		nullIfEmpty(brandID), nullIfEmpty(actorUserID), action, targetType, targetID); err != nil {
		_ = err
	}
}

// opEnqueueJob: parity enqueueJob — attempts default 3 untuk "*generation*".
func (d *Deps) opEnqueueJob(ctx context.Context, kind string, payload map[string]any, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		if strings.Contains(kind, "generation") {
			maxAttempts = 3
		} else {
			maxAttempts = 2
		}
	}
	payloadJSON := "{}"
	if payload != nil {
		if raw, err := json.Marshal(payload); err == nil {
			payloadJSON = string(raw)
		}
	}
	var jobID string
	if err := d.Pool.QueryRow(ctx,
		"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES ($1, $2, $3::jsonb, 'pending', 0, $4, NOW()) RETURNING id",
		uuid.NewString(), kind, payloadJSON, maxAttempts).Scan(&jobID); err != nil {
		return "", err
	}
	return jobID, nil
}

// ---------- koneksi CMS (parity _cms.connectionSecret + test) ----------

// opConnectionJSON: port connectionJson — buang sealed_secret.
func opConnectionJSON(row map[string]any) map[string]any {
	out := ptRow(ptNormalizeMap(row))
	delete(out, "sealed_secret")
	return map[string]any(out)
}

// opConnectionSecret: parity connectionSecret (v3 per-entitas bila
// enc_key_ver >= 1, else WordPress pakai APP_SECRET).
func (d *Deps) opConnectionSecret(row map[string]any) (string, error) {
	sealed, _ := row["sealed_secret"].(string)
	if sealed == "" {
		return "", nil
	}
	r := ptRow(ptNormalizeMap(row))
	if r.intOr("enc_key_ver", 0) >= 1 {
		return crypto.UnsealV3(sealed, d.Cfg.Secret, r.strOr("id", ""))
	}
	secret := d.Cfg.Secret
	if r.strOr("provider", "wordpress") != "wordpress" {
		secret = d.Cfg.CmsEncryptionSecret
		if secret == "" {
			secret = d.Cfg.Secret
		}
	}
	return crypto.Unseal(sealed, secret)
}

func opWPURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/wp-json/" + path
}

// opDoRequest: request HTTP dengan guard SSRF + timeout, body dikembalikan.
func opDoRequest(ctx context.Context, method, target string, headers map[string]string, body string) (int, string, error) {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := security.SafeClient(20 * time.Second).Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if readErr != nil {
		return 0, "", readErr
	}
	return resp.StatusCode, string(raw), nil
}

// opTestWordPress: parity testWordPress (GET wp/v2/users/me + edit_posts).
func (d *Deps) opTestWordPress(ctx context.Context, conn ptRow) (string, string) {
	secret, _ := d.opConnectionSecret(map[string]any(conn))
	basic := base64.StdEncoding.EncodeToString([]byte(conn.strOr("username", "") + ":" + secret))
	status, body, err := opDoRequest(ctx, http.MethodGet,
		opWPURL(conn.strOr("base_url", ""), "wp/v2/users/me?context=edit"),
		map[string]string{"Authorization": "Basic " + basic}, "")
	if err != nil {
		return "error", opTruncate(err.Error(), 500)
	}
	if status < 200 || status >= 300 {
		return "error", opTruncate(fmt.Sprintf("WordPress returned %d: %s", status, body), 500)
	}
	var profile struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
		Cap  *struct {
			EditPosts bool `json:"edit_posts"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal([]byte(body), &profile); err != nil {
		return "error", "WordPress returned invalid response"
	}
	who := profile.Slug
	if who == "" {
		who = profile.Name
	}
	if who == "" {
		who = conn.strOr("username", "")
	}
	if profile.Cap == nil || !profile.Cap.EditPosts {
		return "error", opTruncate(fmt.Sprintf("WordPress authenticated as %s, but edit_posts is denied", who), 500)
	}
	return "ok", fmt.Sprintf("Connected as %s with post creation permission", who)
}

// opTestWebhook: parity testWebhook (ping content.ping → ok:true).
func (d *Deps) opTestWebhook(ctx context.Context, conn ptRow) (string, string) {
	secret, _ := d.opConnectionSecret(map[string]any(conn))
	eventID := uuid.NewString()
	raw, _ := json.Marshal(map[string]any{
		"event": "content.ping", "event_id": eventID, "timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + eventID + "." + string(raw)))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	status, text, err := opDoRequest(ctx, http.MethodPost, conn.strOr("base_url", ""), map[string]string{
		"content-type":        "application/json",
		"x-tulisio-event":     "content.ping",
		"x-tulisio-event-id":  eventID,
		"x-tulisio-timestamp": timestamp,
		"x-tulisio-signature": signature,
	}, string(raw))
	if err != nil {
		return "error", opTruncate(err.Error(), 500)
	}
	if status < 200 || status >= 300 {
		return "error", opTruncate(fmt.Sprintf("Webhook returned %d: %s", status, text), 500)
	}
	var parsed struct {
		OK *bool `json:"ok"`
	}
	if json.Unmarshal([]byte(text), &parsed) != nil || parsed.OK == nil || !*parsed.OK {
		return "error", opTruncate(fmt.Sprintf("Webhook did not respond with ok:true (%s)", text), 500)
	}
	return "ok", "Webhook received ping and responded ok:true"
}

// opTestConnection: parity testConnection (webhook vs WordPress).
func (d *Deps) opTestConnection(ctx context.Context, conn ptRow) (string, string) {
	if conn.strOr("provider", "wordpress") == "webhook" {
		return d.opTestWebhook(ctx, conn)
	}
	return d.opTestWordPress(ctx, conn)
}

// ---------- CSV + publikasi + PR ----------

func opCSVEscape(value any) string {
	text := ""
	if value != nil {
		text = fmt.Sprintf("%v", value)
	}
	if strings.ContainsAny(text, "\",\n\r") {
		return `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
	}
	return text
}

func opCSVLine(cells []any) string {
	escaped := make([]string, len(cells))
	for i, c := range cells {
		escaped[i] = opCSVEscape(c)
	}
	return strings.Join(escaped, ",")
}

// opPublicationListItem: parity lib/platform/publications.publicationListItem.
func opPublicationListItem(row ptRow) map[string]any {
	clientPrice := int(row.numOr("client_price"))
	partnerPrice := clientPrice
	if row.has("partner_price") {
		partnerPrice = int(row.numOr("partner_price"))
	}
	markup := 0
	if row.has("markup_percent") {
		markup = int(row.numOr("markup_percent"))
	}
	var variant any
	if angle, ok := row.str("variant_angle"); ok && angle != "" {
		variant = map[string]any{"angle_label": angle, "mode": row.anyOrNil("variant_mode")}
	}
	return map[string]any{
		"id": row.strOr("id", ""), "order_code": ptNullableStr(row, "order_code"),
		"status": row.strOr("status", ""), "client_price": clientPrice,
		"partner_price": partnerPrice, "markup_percent": markup,
		"publish_status": row.anyOrNil("publish_status"),
		"scheduled_at":   row.anyOrNil("scheduled_at"),
		"published_at":   row.anyOrNil("published_at"),
		"published_url":  ptNullableStr(row, "published_url"),
		"created_at":     row.anyOrNil("created_at"), "updated_at": row.anyOrNil("updated_at"),
		"brand_name":             ptNullableStr(row, "brand_name"),
		"brand_public_id":        ptNullableStr(row, "brand_public_id"),
		"media_name":             ptNullableStr(row, "media_name"),
		"media_integration_type": ptNullableStr(row, "media_integration_type"),
		"document_title":         ptNullableStr(row, "document_title"),
		"campaign_name":          ptNullableStr(row, "campaign_name"),
		"partner_name":           ptNullableStr(row, "partner_name"),
		"error_code":             ptNullableStr(row, "error_code"),
		"error_message":          ptNullableStr(row, "error_message"),
		"variant":                variant,
	}
}

// opPublicationsCSV: parity publicationsCsv (BOM + header Indonesia admin).
func opPublicationsCSV(items []map[string]any) string {
	header := []any{"No. Order", "Status", "Artikel", "Brand", "Brand Public ID", "Media", "Integrasi",
		"Campaign", "Partner", "Harga (IDR)", "Draft/Publish", "Jadwal", "Terbit", "URL Terbit",
		"Kode Error", "Pesan Error", "Dibuat"}
	lines := []string{opCSVLine(header)}
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		code := row.anyOrNil("order_code")
		if code == nil {
			code = row.anyOrNil("id")
		}
		lines = append(lines, opCSVLine([]any{
			code, row.anyOrNil("status"), row.anyOrNil("document_title"),
			row.anyOrNil("brand_name"), row.anyOrNil("brand_public_id"),
			row.anyOrNil("media_name"), row.anyOrNil("media_integration_type"),
			row.anyOrNil("campaign_name"), row.anyOrNil("partner_name"),
			row.anyOrNil("client_price"), row.anyOrNil("publish_status"),
			row.anyOrNil("scheduled_at"), row.anyOrNil("published_at"),
			row.anyOrNil("published_url"), row.anyOrNil("error_code"),
			row.anyOrNil("error_message"), row.anyOrNil("created_at"),
		}))
	}
	return "\uFEFF" + strings.Join(lines, "\r\n")
}

// opPublicationTransitions: parity PUBLICATION_TRANSITIONS.
var opPublicationTransitions = map[string][]string{
	"pending":          {"queued", "waiting_partner", "cancelled"},
	"waiting_partner":  {"partner_approved", "partner_rejected", "cancelled"},
	"partner_approved": {"queued"},
	"partner_rejected": {"cancelled"},
	"queued":           {"processing", "cancelled"},
	"processing":       {"published", "failed", "retrying", "cancelled"},
	"published":        {"revision"},
	"revision":         {"queued", "cancelled"},
	"failed":           {"queued", "retrying", "cancelled"},
	"retrying":         {"queued", "cancelled"},
	"cancelled":        {},
}

// opTransitionPublication: parity transitionPublication (guard status atomik).
func (d *Deps) opTransitionPublication(ctx context.Context, publicationID, to string) error {
	row, err := d.queryRow(ctx, "SELECT status FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	if row == nil {
		return errNotFound("Publication not found")
	}
	current := row.strOr("status", "")
	if current != to && !ptContains(opPublicationTransitions[current], to) {
		return errConflict(fmt.Sprintf("Publication status cannot transition from %s to %s", current, to))
	}
	return d.exec(ctx,
		"UPDATE publications SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3",
		to, publicationID, current)
}

// opVariantForPublication: parity variantForPublication (null bila tidak ada).
func (d *Deps) opVariantForPublication(ctx context.Context, publicationID string) map[string]any {
	row, err := d.queryRow(ctx,
		`SELECT v.angle_id, v.angle_label, v.media_id, v.status AS variant_status, v.batch_id,
		        b.mode, b.seed, b.created_at AS batch_created_at
		 FROM publication_variants v
		 JOIN publication_variant_batches b ON b.id = v.batch_id
		 WHERE v.publication_id = $1 LIMIT 1`, publicationID)
	if err != nil || row == nil {
		return nil
	}
	r := ptRow(ptNormalizeMap(row))
	return map[string]any{
		"angle_id": r.anyOrNil("angle_id"), "angle_label": r.anyOrNil("angle_label"),
		"media_id": r.anyOrNil("media_id"), "variant_status": r.anyOrNil("variant_status"),
		"batch_id": r.anyOrNil("batch_id"), "mode": r.anyOrNil("mode"),
		"seed": int(r.numOr("seed")), "batch_created_at": r.anyOrNil("batch_created_at"),
	}
}

// opPublicationPRAnalysis: parity getPublicationPrAnalysis.
func (d *Deps) opPublicationPRAnalysis(ctx context.Context, publicationID string) map[string]any {
	message, _ := d.queryRow(ctx,
		"SELECT key_messages, results, consistency_score, status, error_message, analyzed_at FROM pr_message_analyses WHERE publication_id = $1 LIMIT 1", publicationID)
	sentiment, _ := d.queryRow(ctx,
		"SELECT sentiment, confidence, status, error_message, analyzed_at FROM pr_sentiment_analyses WHERE publication_id = $1 LIMIT 1", publicationID)
	verification, _ := d.queryRow(ctx,
		"SELECT status, compliance_score, checks, error_message, verified_at FROM pr_publication_verifications WHERE publication_id = $1 ORDER BY verified_at DESC LIMIT 1", publicationID)
	var messageOut, sentimentOut, verificationOut any
	if message != nil {
		m := ptRow(ptNormalizeMap(message))
		score := any(nil)
		if m.has("consistency_score") {
			score = m.numOr("consistency_score")
		}
		messageOut = map[string]any{
			"consistency_score": score, "results": opParseJSONArr(m.anyOrNil("results")),
			"status": m.strOr("status", "completed"), "error_message": ptNullableStr(m, "error_message"),
			"analyzed_at": m.anyOrNil("analyzed_at"),
		}
	}
	if sentiment != nil {
		s := ptRow(ptNormalizeMap(sentiment))
		conf := any(nil)
		if s.has("confidence") {
			conf = s.numOr("confidence")
		}
		sentimentOut = map[string]any{
			"sentiment": s.strOr("sentiment", "neutral"), "confidence": conf,
			"status": s.strOr("status", "completed"), "error_message": ptNullableStr(s, "error_message"),
			"analyzed_at": s.anyOrNil("analyzed_at"),
		}
	}
	if verification != nil {
		v := ptRow(ptNormalizeMap(verification))
		score := any(nil)
		if v.has("compliance_score") {
			score = v.numOr("compliance_score")
		}
		verificationOut = map[string]any{
			"status": v.strOr("status", "unavailable"), "compliance_score": score,
			"checks":        opParseJSONArr(v.anyOrNil("checks")),
			"error_message": ptNullableStr(v, "error_message"),
			"verified_at":   v.anyOrNil("verified_at"),
		}
	}
	return map[string]any{"message": messageOut, "sentiment": sentimentOut, "verification": verificationOut}
}

func opParseJSONArr(v any) any {
	switch t := v.(type) {
	case nil:
		return []any{}
	case string:
		var parsed any
		if err := json.Unmarshal([]byte(t), &parsed); err != nil || parsed == nil {
			return []any{}
		}
		return parsed
	default:
		return v
	}
}

// opHTTPStatusRe: parity /\b(?:status|returned)\b[^0-9]{0,6}(\d{3})\b/i.
var opHTTPStatusRe = regexp.MustCompile(`(?i)\b(?:status|returned)\b[^0-9]{0,6}(\d{3})\b`)

// opHTTPStatusFromMessage: parity httpStatusFromMessage.
func opHTTPStatusFromMessage(message string) any {
	match := opHTTPStatusRe.FindStringSubmatch(message)
	if match == nil {
		return nil
	}
	code := 0
	for _, ch := range match[1] {
		code = code*10 + int(ch-'0')
	}
	if code < 100 || code > 599 {
		return nil
	}
	return code
}

// opAnglePool: parity ANGLE_POOL (lib/content/variants.ts) — label + instruksi.
type opAngle struct {
	ID          string
	Label       string
	Instruction string
}

var opAnglePool = []opAngle{
	{"news_value", "Nilai berita", "Tekankan kebaruan dan dampak bagi pembaca/industri — apa yang berubah dan mengapa penting sekarang."},
	{"benefit", "Manfaat pembaca", "Fokus pada manfaat praktis bagi pembaca: apa artinya bagi mereka dan apa yang bisa mereka lakukan."},
	{"story", "Kisah di baliknya", "Sajikan sebagai kisah: latar, tokoh, perjalanan, dan momen kunci — naratif namun tetap faktual."},
	{"data", "Data & angka", "Jadikan data, statistik, dan fakta terukur sebagai tulang punggung penyajian; angka tampil menonjol."},
	{"expert", "Sudut pakar & kutipan", "Bangun artikel di sekitar narasumber/kutipan dan pandangan pakar — kredibilitas lewat suara pihak terkait."},
	{"community", "Dampak komunitas", "Fokus pada dampak terhadap masyarakat/komunitas dan pihak yang terbantu atau terpengaruh."},
	{"how_to", "Panduan praktis", "Sajikan sebagai panduan: langkah demi langkah, tips, dan hal yang perlu diperhatikan pembaca."},
	{"trend", "Konteks tren", "Hubungkan dengan tren, momen, atau isu yang sedang ramai — beri konteks mengapa relevan saat ini."},
	{"solution", "Solusi masalah", "Rangkaikan sebagai jawaban atas masalah: masalah → mengapa terjadi → bagaimana diatasi."},
	{"milestone", "Pencapaian & tonggak", "Tekankan pencapaian, tonggak sejarah, dan apa yang diraih sejauh ini dengan bukti pendukung."},
}

func opAngleByID(id string) (opAngle, bool) {
	for _, a := range opAnglePool {
		if a.ID == id {
			return a, true
		}
	}
	return opAngle{}, false
}

// opParseJobPayload: parity parseJobPayload (jsonb map atau string JSON).
func opParseJobPayload(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return t
	case string:
		var parsed map[string]any
		if err := json.Unmarshal([]byte(t), &parsed); err != nil {
			return map[string]any{}
		}
		return parsed
	}
	return map[string]any{}
}

// opVariantBatchesJSON: parity variantBatchesJson (list batch + varian,
// tanpa brandId = monitoring lintas brand).
func (d *Deps) opVariantBatchesJSON(ctx context.Context, status, q string, limit, offset int) ([]map[string]any, int) {
	clauses := []string{}
	params := []any{}
	if status != "" {
		params = append(params, status)
		clauses = append(clauses, "b.status = $"+ptItoa(len(params)))
	}
	if q != "" {
		like := "%" + q + "%"
		for i := 0; i < 6; i++ {
			params = append(params, like)
		}
		base := len(params) - 5
		clauses = append(clauses, "(br.name ILIKE $"+ptItoa(base)+" OR br.public_id ILIKE $"+ptItoa(base+1)+" OR u.email ILIKE $"+ptItoa(base+2)+" OR b.id ILIKE $"+ptItoa(base+3)+
			" OR EXISTS (SELECT 1 FROM publication_variants sv LEFT JOIN media sm ON sm.id = sv.media_id WHERE sv.batch_id = b.id AND (sm.name ILIKE $"+ptItoa(base+4)+" OR sv.angle_label ILIKE $"+ptItoa(base+5)+")))")
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	from := `FROM publication_variant_batches b
	    JOIN brands br ON br.id = b.brand_id
	    JOIN users u ON u.id = b.user_id`
	countRow, err := d.queryRow(ctx, "SELECT COUNT(*) AS count "+from+" "+where, params...)
	total := 0
	if err == nil && countRow != nil {
		total = countRow.intOr("count", 0)
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	batchRows, err := d.queryRows(ctx,
		`SELECT b.id AS batch_id, b.mode, b.seed, b.status, b.source_document_id, b.created_at, b.completed_at,
		        br.name AS brand_name, br.public_id AS brand_public_id, u.email AS user_email,
		        (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id) AS variant_total,
		        (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'completed') AS variant_done,
		        (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'failed') AS variant_failed
		 `+from+" "+where+" ORDER BY b.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		return []map[string]any{}, total
	}
	variantsByBatch := map[string][]map[string]any{}
	if len(batchRows) > 0 {
		ids := make([]any, 0, len(batchRows))
		placeholders := make([]string, 0, len(batchRows))
		for i, raw := range batchRows {
			bid := ptRow(ptNormalizeMap(raw)).strOr("batch_id", "")
			ids = append(ids, bid)
			placeholders = append(placeholders, "$"+ptItoa(i+1))
		}
		variantRows, vErr := d.queryRows(ctx,
			`SELECT v.batch_id, v.id, v.media_id, v.angle_id, v.angle_label, v.status, v.document_id, v.publication_id,
			        v.error_message, v.created_at, m.name AS media_name, d.title AS document_title, p.status AS publication_status
			 FROM publication_variants v
			 LEFT JOIN media m ON m.id = v.media_id
			 LEFT JOIN content_documents d ON d.id = v.document_id
			 LEFT JOIN publications p ON p.id = v.publication_id
			 WHERE v.batch_id IN (`+strings.Join(placeholders, ",")+`)
			 ORDER BY v.created_at ASC`, ids...)
		if vErr == nil {
			for _, raw := range variantRows {
				v := ptRow(ptNormalizeMap(raw))
				key := v.strOr("batch_id", "")
				variantsByBatch[key] = append(variantsByBatch[key], map[string]any{
					"id": v.anyOrNil("id"), "media_id": v.anyOrNil("media_id"),
					"media_name": ptNullableStr(v, "media_name"), "angle_id": v.anyOrNil("angle_id"),
					"angle_label": v.anyOrNil("angle_label"), "status": v.anyOrNil("status"),
					"document_id": v.anyOrNil("document_id"), "publication_id": v.anyOrNil("publication_id"),
					"document_title":     ptNullableStr(v, "document_title"),
					"publication_status": ptNullableStr(v, "publication_status"),
					"error_message":      ptNullableStr(v, "error_message"),
					"created_at":         v.anyOrNil("created_at"),
				})
			}
		}
	}
	out := make([]map[string]any, 0, len(batchRows))
	for _, raw := range batchRows {
		b := ptRow(ptNormalizeMap(raw))
		key := b.strOr("batch_id", "")
		variants := variantsByBatch[key]
		if variants == nil {
			variants = []map[string]any{}
		}
		out = append(out, map[string]any{
			"batch_id": key, "mode": b.anyOrNil("mode"), "seed": int(b.numOr("seed")),
			"status": b.anyOrNil("status"), "source_document_id": b.anyOrNil("source_document_id"),
			"created_at": b.anyOrNil("created_at"), "completed_at": b.anyOrNil("completed_at"),
			"brand_name": ptNullableStr(b, "brand_name"), "brand_public_id": ptNullableStr(b, "brand_public_id"),
			"user_email": ptNullableStr(b, "user_email"),
			"variant_counts": map[string]any{
				"total": b.intOr("variant_total", 0), "done": b.intOr("variant_done", 0), "failed": b.intOr("variant_failed", 0),
			},
			"variants": variants,
		})
	}
	return out, total
}

// opVariantBatchDetailJSON: parity variantBatchDetailJson.
func (d *Deps) opVariantBatchDetailJSON(ctx context.Context, batchID string) (map[string]any, error) {
	batch, err := d.queryRow(ctx,
		`SELECT b.id AS batch_id, b.brand_id, b.mode, b.seed, b.status, b.source_document_id, b.source_material,
		        b.created_at, b.completed_at, br.name AS brand_name, br.public_id AS brand_public_id, u.email AS user_email,
		        (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id) AS variant_total,
		        (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'completed') AS variant_done,
		        (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'failed') AS variant_failed
		 FROM publication_variant_batches b
		 JOIN brands br ON br.id = b.brand_id
		 JOIN users u ON u.id = b.user_id
		 WHERE b.id = $1 LIMIT 1`, batchID)
	if err != nil {
		return nil, err
	}
	if batch == nil {
		return nil, errNotFound("Variant batch not found")
	}
	variants, err := d.queryRows(ctx,
		`SELECT v.id, v.media_id, v.angle_id, v.angle_label, v.status, v.document_id, v.publication_id,
		        v.error_message, v.created_at, v.updated_at,
		        m.name AS media_name, m.category AS media_category, m.integration_type AS media_integration_type,
		        d.title AS document_title, p.status AS publication_status, p.published_url
		 FROM publication_variants v
		 LEFT JOIN media m ON m.id = v.media_id
		 LEFT JOIN content_documents d ON d.id = v.document_id
		 LEFT JOIN publications p ON p.id = v.publication_id
		 WHERE v.batch_id = $1 ORDER BY v.created_at ASC`, batchID)
	if err != nil {
		return nil, err
	}
	jobsByVariant := map[string][]map[string]any{}
	if len(variants) > 0 {
		ids := make([]any, 0, len(variants))
		placeholders := make([]string, 0, len(variants))
		for i, raw := range variants {
			v := ptRow(ptNormalizeMap(raw))
			ids = append(ids, v.strOr("id", ""))
			placeholders = append(placeholders, "$"+ptItoa(i+1))
		}
		jobRows, jErr := d.queryRows(ctx,
			`SELECT id, status, attempts, max_attempts, last_error, run_at, created_at, payload
			 FROM jobs
			 WHERE kind = 'publication_variant_generation' AND payload->>'variantId' IN (`+strings.Join(placeholders, ",")+`)
			 ORDER BY created_at DESC`, ids...)
		if jErr == nil {
			for _, raw := range jobRows {
				j := ptRow(ptNormalizeMap(raw))
				key := ""
				if pv, ok := opParseJobPayload(j.anyOrNil("payload"))["variantId"].(string); ok {
					key = pv
				}
				jobsByVariant[key] = append(jobsByVariant[key], map[string]any{
					"id": j.anyOrNil("id"), "status": j.anyOrNil("status"),
					"attempts": j.intOr("attempts", 0), "max_attempts": j.intOr("max_attempts", 0),
					"last_error": ptNullableStr(j, "last_error"), "run_at": j.anyOrNil("run_at"),
					"created_at": j.anyOrNil("created_at"),
				})
			}
		}
	}
	variantsOut := make([]map[string]any, 0, len(variants))
	for _, raw := range variants {
		v := ptRow(ptNormalizeMap(raw))
		key := v.strOr("id", "")
		jobs := jobsByVariant[key]
		if jobs == nil {
			jobs = []map[string]any{}
		}
		variantsOut = append(variantsOut, map[string]any{
			"id": key, "media_id": v.anyOrNil("media_id"), "media_name": ptNullableStr(v, "media_name"),
			"media_category":         ptNullableStr(v, "media_category"),
			"media_integration_type": ptNullableStr(v, "media_integration_type"),
			"angle_id":               v.anyOrNil("angle_id"), "angle_label": v.anyOrNil("angle_label"),
			"status": v.anyOrNil("status"), "document_id": v.anyOrNil("document_id"),
			"document_title":     ptNullableStr(v, "document_title"),
			"publication_id":     v.anyOrNil("publication_id"),
			"publication_status": ptNullableStr(v, "publication_status"),
			"published_url":      ptNullableStr(v, "published_url"),
			"error_message":      ptNullableStr(v, "error_message"),
			"created_at":         v.anyOrNil("created_at"), "updated_at": v.anyOrNil("updated_at"),
			"jobs": jobs,
		})
	}
	b := ptRow(ptNormalizeMap(batch))
	return map[string]any{
		"batch": map[string]any{
			"batch_id": b.strOr("batch_id", ""), "brand_id": b.anyOrNil("brand_id"),
			"brand_name": ptNullableStr(b, "brand_name"), "brand_public_id": ptNullableStr(b, "brand_public_id"),
			"user_email": ptNullableStr(b, "user_email"), "mode": b.anyOrNil("mode"),
			"seed": int(b.numOr("seed")), "status": b.anyOrNil("status"),
			"source_document_id": b.anyOrNil("source_document_id"),
			"source_material":    b.anyOrNil("source_material"),
			"created_at":         b.anyOrNil("created_at"), "completed_at": b.anyOrNil("completed_at"),
			"variant_counts": map[string]any{
				"total": b.intOr("variant_total", 0), "done": b.intOr("variant_done", 0), "failed": b.intOr("variant_failed", 0),
			},
		},
		"variants": variantsOut,
	}, nil
}

// opNotifyVariantBatchFailed: parity notifyAdminsVariantBatchFailed +
// sendVariantBatchFailedEmails (notifikasi admin platform; email dilewati bila
// template belum terdaftar di paket email Go).
func (d *Deps) opNotifyVariantBatchFailed(ctx context.Context, batchID, status string) {
	row, err := d.queryRow(ctx,
		`SELECT b.mode, br.name AS brand_name,
		        (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id) AS total,
		        (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'failed') AS failed,
		        (SELECT string_agg(m.name, ', ' ORDER BY v.created_at)
		         FROM publication_variants v JOIN media m ON m.id = v.media_id
		         WHERE v.batch_id = b.id AND v.status = 'failed') AS failed_media
		 FROM publication_variant_batches b
		 JOIN brands br ON br.id = b.brand_id
		 WHERE b.id = $1 LIMIT 1`, batchID)
	if err != nil || row == nil {
		return
	}
	r := ptRow(ptNormalizeMap(row))
	brandName := r.strOr("brand_name", "Brand")
	total := r.intOr("total", 0)
	failed := r.intOr("failed", 0)
	mediaList := r.strOr("failed_media", "")
	mode := "parafrase"
	if r.strOr("mode", "generate") == "generate" {
		mode = "dibuatkan dari bahan"
	}
	body := fmt.Sprintf("Batch %s %s: %d dari %d artikel gagal dibuat", mode, brandName, failed, total)
	if mediaList != "" {
		body += " (" + mediaList + ")"
	}
	body += "."
	title := "Batch variant gagal"
	if status == "partial" {
		title = "Sebagian artikel variant gagal"
	}
	link := "/admin/variants"
	d.notifyPlatformAdmins(ctx, "variant_batch.failed", title, body, &link)
}

// opVariantBatchesCSV: parity variantBatchesCsv — flatten per varian, BOM.
func (d *Deps) opVariantBatchesCSV(ctx context.Context, status, q string) string {
	batches, _ := d.opVariantBatchesJSON(ctx, status, q, 1000, 0)
	header := []any{"Batch ID", "Mode", "Seed", "Status Batch", "Brand", "Brand Public ID", "User Email",
		"Batch Dibuat", "Batch Selesai", "Media", "Sudut Pandang", "Status Varian", "Artikel",
		"Publikasi ID", "Status Publikasi", "Error", "Varian Dibuat"}
	lines := []string{opCSVLine(header)}
	for _, raw := range batches {
		b := ptRow(ptNormalizeMap(raw))
		variants := []map[string]any{{}}
		if list, ok := b.anyOrNil("variants").([]map[string]any); ok && len(list) > 0 {
			variants = list
		}
		emit := func(v map[string]any) {
			vr := ptRow(ptNormalizeMap(v))
			lines = append(lines, opCSVLine([]any{
				b.anyOrNil("batch_id"), b.anyOrNil("mode"), b.anyOrNil("seed"), b.anyOrNil("status"),
				b.anyOrNil("brand_name"), b.anyOrNil("brand_public_id"), b.anyOrNil("user_email"),
				b.anyOrNil("created_at"), b.anyOrNil("completed_at"),
				vr.anyOrNil("media_name"), vr.anyOrNil("angle_label"), vr.anyOrNil("status"),
				vr.anyOrNil("document_title"), vr.anyOrNil("publication_id"), vr.anyOrNil("publication_status"),
				vr.anyOrNil("error_message"), vr.anyOrNil("created_at"),
			}))
		}
		for _, v := range variants {
			emit(v)
		}
	}
	return "\uFEFF" + strings.Join(lines, "\r\n")
}

// ---------- validasi body (pesan gaya zod v3, sama dengan port lain) ----------

// opGetString: nilai string (trim opsional). present=false bila key absen,
// isString=false bila ada tapi bukan string/null.
func opGetString(body map[string]any, key string, trim bool) (string, bool, bool) {
	v, present := body[key]
	if !present || v == nil {
		return "", present, false
	}
	s, isString := v.(string)
	if !isString {
		return "", true, false
	}
	if trim {
		s = strings.TrimSpace(s)
	}
	return s, true, true
}

// opGetNum: nilai numerik (float64/int/int64). Bukan string.
func opGetNum(body map[string]any, key string) (float64, bool, bool) {
	v, present := body[key]
	if !present || v == nil {
		return 0, present, false
	}
	switch t := v.(type) {
	case float64:
		return t, true, true
	case int:
		return float64(t), true, true
	case int64:
		return float64(t), true, true
	}
	return 0, true, false
}

func opGetBool(body map[string]any, key string) (bool, bool, bool) {
	v, present := body[key]
	if !present || v == nil {
		return false, present, false
	}
	b, isBool := v.(bool)
	return b, true, isBool
}

func opGetArr(body map[string]any, key string) ([]any, bool, bool) {
	v, present := body[key]
	if !present || v == nil {
		return nil, present, false
	}
	arr, isArr := v.([]any)
	return arr, true, isArr
}

// opGetObj: sub-objek (present=false bila absen, isObj=false bila bukan objek).
func opGetObj(body map[string]any, key string) (map[string]any, bool, bool) {
	v, present := body[key]
	if !present || v == nil {
		return nil, present, false
	}
	obj, isObj := v.(map[string]any)
	return obj, true, isObj
}

// opJSONTypeName: nama tipe ala zod untuk pesan "expected X, received Y".
func opJSONTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, int, int64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

// opReceivedName: nama tipe yang diterima zod untuk field body (key absen →
// "undefined", null → "null", selainnya nama tipe).
func opReceivedName(body map[string]any, key string) string {
	v, present := body[key]
	if !present {
		return "undefined"
	}
	return opJSONTypeName(v)
}

// opNullableArg: nilai nullable opsional → string apa adanya (termasuk "")
// bila key ada dan bukan null, else nil. Parity `value ?? null`.
func opNullableArg(body map[string]any, key string) any {
	v, present := body[key]
	if !present || v == nil {
		return nil
	}
	if s, isString := v.(string); isString {
		return strings.TrimSpace(s)
	}
	return nil
}

func opEnumExpected(allowed []string) string {
	parts := make([]string, 0, len(allowed))
	for _, a := range allowed {
		parts = append(parts, "'"+a+"'")
	}
	return strings.Join(parts, " | ")
}

// opStrTooBig / opStrTooSmall: pesan zod v3.
func opStrTooBig(max int) string {
	return fmt.Sprintf("String must contain at most %d character(s)", max)
}
func opStrTooSmall(min int) string {
	return fmt.Sprintf("String must contain at least %d character(s)", min)
}

// opJSONStrings: []any → []string (JSON.stringify untuk kolom jsonb).
func opJSONStrings(arr []any) string {
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return ptJSONString(out)
}

// opIsInt: parity Number.isInteger.
func opIsInt(v float64) bool { return v == float64(int64(v)) }

// opCount: COUNT(*) best-effort (0 bila query gagal).
func (d *Deps) opCount(ctx context.Context, sql string, args ...any) int {
	row, err := d.queryRow(ctx, sql, args...)
	if err != nil || row == nil {
		return 0
	}
	return row.intOr("count", 0)
}

// opIsPlatformAdminUser: parity isPlatformAdmin(userId, email).
func (d *Deps) opIsPlatformAdminUser(ctx context.Context, userID, email string) bool {
	return d.Admin.AdminContext(ctx, userID, email) != nil
}

// opNowISO: parity new Date().toISOString().
func opNowISO() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00") }

func opTruncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// opGetArr3: varian ringkas opGetArr yang hanya mengembalikan daftar + status
// "ada isinya" (dipakai handler yang tidak membedakan present vs valid).
func opGetArr3(body map[string]any, key string) ([]any, bool, bool) {
	arr, _, isArr := opGetArr(body, key)
	return arr, len(arr) > 0, isArr
}

// publicationStatusesForFilter: status publikasi yang sah untuk filter admin.
var publicationStatusesForFilter = []string{
	"pending", "waiting_partner", "partner_approved", "partner_rejected",
	"queued", "processing", "published", "failed", "retrying", "cancelled",
	"revision",
}

// asMaps: []rowMap -> []map[string]any untuk helper yang masih memakai map biasa.
func asMaps(rows []rowMap) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = map[string]any(r)
	}
	return out
}

// opEmailRe: validasi email sederhana (parity z.string().email()).
var opEmailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// errForbidden: parity 403 admin.
func errForbidden(msg string) error { return adminshared.Err(http.StatusForbidden, msg) }

// invalidatePartnerCache: buang cache akses mitra milik user (parity
// invalidatePartnerCache di app lama). Cache hidup di paket partner; paket
// partner tidak mengimpor platform sehingga impor ini bebas siklus.
func invalidatePartnerCache(userID string) {
	partnerdomain.InvalidatePartnerCache(userID)
}
