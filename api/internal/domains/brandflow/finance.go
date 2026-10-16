package brandflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	authpkg "github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// finance.go: port app/v1/brands/[brandId]/{credits/topup, credit-reports,
// link-monitor/request-replacement, pr-calculator}/route.ts.

// ---------- credits/topup (GET lookup idempoten) ----------

// CreditsTopupLookup: GET /v1/brands/{brandId}/credits/topup?idempotency_key=…
func (d *Deps) CreditsTopupLookup(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// Lookup memakai id kanonik: baris payment_transactions disimpan dengan
	// brands.id, sehingga public_id dari URL tidak akan pernah cocok.
	rawBrandID := brandRow.ID
	key := strings.TrimSpace(r.URL.Query().Get("idempotency_key"))
	if key == "" {
		writeJSON(w, http.StatusOK, map[string]any{"transaction": nil})
		return
	}
	if len(key) > 100 {
		key = key[:100]
	}
	row, err := d.queryRow(r.Context(),
		"SELECT * FROM payment_transactions WHERE brand_id = $1 AND idempotency_key = $2 ORDER BY created_at DESC LIMIT 1",
		rawBrandID, key)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	var transaction any
	if row != nil {
		transaction = bsTransactionJSON(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"transaction": transaction})
}

// bsTransactionJSON: parity transactionJson (lib/payments/service.ts).
func bsTransactionJSON(row bsRow) map[string]any {
	return map[string]any{
		"id": row.raw("id"), "reference": row.raw("reference"),
		"gateway_provider":        row.raw("gateway_provider"),
		"gateway_environment":     row.raw("gateway_environment"),
		"amount":                  row.numOr("amount"),
		"currency":                row.raw("currency"),
		"status":                  row.raw("status"),
		"payment_method":          row.raw("payment_method"),
		"payment_url":             row.raw("payment_url"),
		"external_transaction_id": row.raw("external_transaction_id"),
		"provider_reference":      row.raw("provider_reference"),
		"purpose_ref_id":          row.raw("purpose_ref_id"),
		"created_at":              row.timePtr("created_at"),
		"paid_at":                 row.timePtr("paid_at"),
		"expired_at":              row.timePtr("expired_at"),
	}
}

// ---------- credits/topup (POST) ----------

// bsTopupSchema / invoiceSchema: parity zod.
type bsInvoiceData struct {
	CompanyName string `json:"company_name"`
	NPWP        string `json:"npwp"`
	NPWPAddress string `json:"npwp_address"`
}

// CreditsTopupCreate: POST /v1/brands/{brandId}/credits/topup
func (d *Deps) CreditsTopupCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// Wajib id kanonik (UUID): createPayment menyimpan brand_id ini lalu webhook
	// mengkredit credit_wallets/credit_ledger_entries yang ber-join ke
	// brands.id. Memakai public_id membuat saldo masuk ke baris phantom dan
	// transaksi tak terlihat oleh klien maupun admin.
	brandID := brandRow.ID
	if err := authpkg.CheckWriteRate(r.Context(), d.Pool, "write:topup:"+user.ID, d.Cfg.WriteRateLimitPerMinute); err != nil {
		bsErr(d, w, err)
		return
	}
	body := decodeJSON(r)

	// topUpSchema: tc int positif max 10_000_000; note trim max2000 nullable
	// optional; idempotency_key trim 8..100 optional nullable.
	rawTC, hasTC := body["tc"]
	if !hasTC || rawTC == nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected number, received undefined")
		return
	}
	tcFloat, isNum := rawTC.(float64)
	if !isNum {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected number, received "+bsZodTypeName(rawTC))
		return
	}
	if tcFloat != math.Trunc(tcFloat) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected int, received number")
		return
	}
	if tcFloat <= 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Too small: expected number to be >0")
		return
	}
	if tcFloat > 10_000_000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Too big: expected number to be <=10000000")
		return
	}
	tc := int(tcFloat)

	note := ""
	if raw, ok := body["note"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected string, received "+bsZodTypeName(raw))
			return
		}
		trimmed := strings.TrimSpace(s)
		if utf8.RuneCountInString(trimmed) > 2000 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Too big: expected string to have <=2000 characters")
			return
		}
		note = trimmed
	}
	idempotencyKey := ""
	if raw, ok := body["idempotency_key"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected string, received "+bsZodTypeName(raw))
			return
		}
		trimmed := strings.TrimSpace(s)
		if utf8.RuneCountInString(trimmed) < 8 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Too small: expected string to have >=8 characters")
			return
		}
		if utf8.RuneCountInString(trimmed) > 100 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Too big: expected string to have <=100 characters")
			return
		}
		idempotencyKey = trimmed
	}

	// note = JSON.stringify(invoice) dari client — payload tidak valid → null.
	var invoiceData any
	if note != "" {
		var parsed map[string]any
		if json.Unmarshal([]byte(note), &parsed) == nil {
			if inv, ok := bsParseInvoice(parsed); ok {
				invoiceData = inv
			}
		}
	}

	config := d.getTcConfig(r.Context())
	rateIDR := config.RateIDR
	amountIDR := tc * rateIDR

	// Redirect balik ke Tulisio: origin request menang.
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = bsRequestOrigin(r)
	}
	returnURL := ""
	if origin != "" {
		returnURL = strings.TrimRight(origin, "/")
	}

	transaction, createErr := d.bsCreatePayment(r.Context(), bsPaymentInput{
		UserID: user.ID, BrandID: brandID, Amount: float64(amountIDR), Currency: "IDR",
		Description: "Top Up Tulisio Credit — " + itoa(tc) + " TC", Purpose: "topup",
		PurposeRefID: itoa(tc), PayerEmail: user.Email, CustomerName: user.DisplayName,
		ReturnURL: returnURL, InvoiceData: invoiceData,
		IdempotencyKey: idempotencyKey,
	})
	if createErr != nil {
		if strings.Contains(createErr.Error(), "Belum ada payment gateway aktif") {
			httpx.Error(w, http.StatusBadRequest, createErr.Error())
			return
		}
		d.writeErr(w, createErr)
		return
	}
	d.audit(r.Context(), brandID, user.ID, "brand.credits.topup_requested", "brand", brandID)
	writeJSON(w, http.StatusOK, map[string]any{
		"payment": bsTransactionJSON(transaction), "tc": tc,
		"rate_idr": rateIDR, "amount_idr": amountIDR,
	})
}

// bsParseInvoice: validasi invoiceSchema (semua field wajib, batas panjang).
func bsParseInvoice(body map[string]any) (bsInvoiceData, bool) {
	out := bsInvoiceData{}
	fields := []struct {
		key string
		min int
		max int
		dst *string
	}{
		{"company_name", 1, 200, &out.CompanyName},
		{"npwp", 1, 30, &out.NPWP},
		{"npwp_address", 1, 500, &out.NPWPAddress},
	}
	for _, f := range fields {
		raw, ok := body[f.key]
		if !ok || raw == nil {
			return out, false
		}
		s, isStr := raw.(string)
		if !isStr {
			return out, false
		}
		trimmed := strings.TrimSpace(s)
		if utf8.RuneCountInString(trimmed) < f.min || utf8.RuneCountInString(trimmed) > f.max {
			return out, false
		}
		*f.dst = trimmed
	}
	return out, true
}

// bsRequestOrigin: origin dari URL request (parity new URL(request.url).origin).
func bsRequestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := r.Host
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

// ---------- credit-reports ----------

// CreditReportsCreate: POST /v1/brands/{brandId}/credit-reports
func (d *Deps) CreditReportsCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.contentErr(w, err)
		return
	}
	// Id kanonik: daftar admin JOIN ke brands.id dan wallet ke brand_id UUID,
	// jadi laporan ber-public_id tidak akan pernah muncul meski tersimpan.
	brandID := brandRow.ID
	body := decodeJSON(r)
	// reportSchema: claimed_tc int positif max 10_000_000; payment_reference trim
	// max200 nullable optional; description trim 10..2000.
	rawClaimed, hasClaimed := body["claimed_tc"]
	if !hasClaimed || rawClaimed == nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Data laporan tidak valid.")
		return
	}
	claimedFloat, isNum := rawClaimed.(float64)
	if !isNum || claimedFloat != math.Trunc(claimedFloat) || claimedFloat <= 0 || claimedFloat > 10_000_000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Data laporan tidak valid.")
		return
	}
	claimedTC := int(claimedFloat)
	var paymentReference any
	if raw, ok := body["payment_reference"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr || utf8.RuneCountInString(strings.TrimSpace(s)) > 200 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Data laporan tidak valid.")
			return
		}
		paymentReference = strings.TrimSpace(s)
	}
	rawDescription, hasDescription := body["description"]
	if !hasDescription || rawDescription == nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Data laporan tidak valid.")
		return
	}
	description, isStr := rawDescription.(string)
	if !isStr {
		httpx.Error(w, http.StatusUnprocessableEntity, "Data laporan tidak valid.")
		return
	}
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) < 10 || utf8.RuneCountInString(description) > 2000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Data laporan tidak valid.")
		return
	}

	rate := d.getTcConfig(r.Context()).RateIDR
	amountIDR := claimedTC * rate
	id := uuid.NewString()
	if err := d.exec(r.Context(), `
		INSERT INTO credit_topup_reports (id, brand_id, user_id, claimed_tc, amount_idr, payment_reference, description, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')`,
		id, brandID, user.ID, claimedTC, amountIDR, paymentReference, description); err != nil {
		d.contentErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"report": map[string]any{
			"id": id, "brand_id": brandID, "claimed_tc": claimedTC,
			"amount_idr": amountIDR, "payment_reference": paymentReference,
			"status": "pending", "created_at": isoNow(),
		},
	})
}

// ---------- link-monitor/request-replacement ----------

// LinkMonitorRequestReplacement: POST /v1/brands/{brandId}/link-monitor/request-replacement
func (d *Deps) LinkMonitorRequestReplacement(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	// requestSchema: publicationId uuid.
	raw, ok := body["publicationId"]
	if !ok || raw == nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected string, received undefined")
		return
	}
	publicationID, isStr := raw.(string)
	if !isStr {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected string, received "+bsZodTypeName(raw))
		return
	}
	if !bsIsUUID(publicationID) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid UUID")
		return
	}

	found, err := d.queryRow(r.Context(), `
		SELECT p.id, d.title, m.name AS media_name
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN content_documents d ON d.id = p.document_id
		WHERE p.id = $1 AND p.brand_id = $2
		LIMIT 1`, publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if found == nil {
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return
	}

	// Klaim dicatat lebih dulu supaya notifikasi admin bisa menunjuk LANGSUNG ke
	// klaim itu (?claim=<id>), bukan hanya ke halaman daftarnya.
	claimRow, err := d.queryRow(r.Context(), `
		INSERT INTO link_guardian_claims
		  (id, publication_id, brand_id, status, detection_state, anchor_expected, created_by)
		SELECT $4, $1, $2, 'submitted',
		       CASE WHEN lc.in_article_state = 'removed' THEN 'link_removed'
		            WHEN lc.state = 'gone' THEN 'gone' END,
		       lc.in_article_anchor, $3
		  FROM (SELECT 1) AS x
		  LEFT JOIN publication_link_checks lc ON lc.publication_id = $1
		ON CONFLICT (publication_id) DO UPDATE SET
		  status = CASE WHEN link_guardian_claims.status = 'rejected' THEN 'submitted'
		                ELSE link_guardian_claims.status END,
		  detection_state = EXCLUDED.detection_state,
		  anchor_expected = COALESCE(EXCLUDED.anchor_expected, link_guardian_claims.anchor_expected),
		  updated_at = NOW()
		RETURNING id`, publicationID, brandRow.ID, user.ID, uuid.NewString())
	if err != nil {
		d.writeErr(w, err)
		return
	}
	claimLink := "/admin/link-monitor?claim=" + claimRow.strOr("id", "")

	dedupeKey := "link_replacement_request:" + publicationID
	title := "Klien mengajukan penggantian artikel"
	articleTitle := found.strOr("title", "Artikel")
	mediaName := found.strOr("media_name", "media")
	bodyText := "\"" + articleTitle + "\" di " + mediaName + " diminta diganti oleh " + user.Email + "."
	if err := d.exec(r.Context(), `
		INSERT INTO notifications (id, user_id, type, title, body, link, publication_id, dedupe_key)
		SELECT gen_random_uuid(), u.id, $1, $2, $3, $4, $5, $6
		  FROM users u
		 WHERE u.role = 'admin' AND u.is_active = TRUE
		   AND NOT EXISTS (
		     SELECT 1 FROM notifications n WHERE n.user_id = u.id AND n.dedupe_key = $6
		   )`,
		"publication.link_replacement_requested", title, bodyText, claimLink,
		publicationID, dedupeKey); err != nil {
		d.writeErr(w, err)
		return
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "publication.link_lost.replacement_requested", "publication", publicationID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "publication_id": publicationID})
}

// ---------- pr-calculator ----------

// PrCalculator: GET /v1/brands/{brandId}/pr-calculator
func (d *Deps) PrCalculator(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	canonicalID := brandRow.ID
	q := r.URL.Query()
	campaignID := strings.TrimSpace(q.Get("campaign_id"))
	recalc := q.Get("recalc") == "1"
	from := strings.TrimSpace(q.Get("from"))
	to := strings.TrimSpace(q.Get("to"))
	if (from != "" && !bsParsableDate(from)) || (to != "" && !bsParsableDate(to)) {
		httpx.Error(w, http.StatusUnprocessableEntity, "from/to harus tanggal valid (ISO)")
		return
	}

	if campaignID != "" {
		campaign, err := d.queryRow(r.Context(),
			"SELECT id, brand_id FROM campaigns WHERE id = $1 LIMIT 1", campaignID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if campaign == nil || campaign.strOr("brand_id", "") != canonicalID {
			httpx.Error(w, http.StatusNotFound, "Campaign not found")
			return
		}
	}

	campaigns, err := d.queryRows(r.Context(),
		"SELECT id, name, description, status, budget, starts_at, ends_at FROM campaigns WHERE brand_id = $1 ORDER BY created_at DESC LIMIT 50",
		canonicalID)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	var score any
	var metrics any
	var mediaPerformance any
	var analysisHistory any
	var benchmark any
	var campaignInfo any
	if campaignID != "" {
		for _, c := range campaigns {
			if c.strOr("id", "") == campaignID {
				campaignInfo = map[string]any{
					"id": c.raw("id"), "name": c.raw("name"), "description": c.raw("description"),
					"status": c.raw("status"), "budget": c.numPtr("budget"),
					"starts_at": c.timePtr("starts_at"), "ends_at": c.timePtr("ends_at"),
				}
				break
			}
		}
		score = d.bsPrScore(r.Context(), campaignID, recalc, user.ID)
		metrics = d.bsPublicationMetrics(r.Context(), campaignID)
		mediaPerformance = d.bsMediaPerformance(r.Context(), campaignID)
		history, err := d.bsAnalysisHistory(r.Context(), campaignID, from, to)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		analysisHistory = history
		benchmark = d.bsPrBenchmark(r.Context(), campaignID, from, to)
	}

	brandRates := d.bsLoadBrandPrRates(r.Context(), canonicalID)
	var prValue any
	if campaignID != "" {
		campaignPtr := campaignID
		value := d.bsEstimatePRValue(r.Context(), canonicalID, &campaignPtr, "", "", &brandRates.Rates, nil)
		prValue = map[string]any{
			"estimated_media_value": value.EstimatedMediaVal,
			"estimated_reach":       value.EstimatedReach,
			"campaign_cost":         value.CampaignCost,
			"estimated_pr_value":    value.EstimatedPRValue,
			"estimated_saving":      value.EstimatedSaving,
			"standard":              bsPrStandardJSON(brandRates),
		}
	}
	campaignList := make([]map[string]any, 0, len(campaigns))
	for _, c := range campaigns {
		campaignList = append(campaignList, map[string]any{
			"id": c.raw("id"), "name": c.raw("name"),
			"description": c.raw("description"), "status": c.raw("status"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaign_id":       nullIfEmpty(campaignID),
		"period":            map[string]any{"from": nullIfEmpty(from), "to": nullIfEmpty(to)},
		"pr_standard":       bsPrStandardJSON(brandRates),
		"pr_value":          prValue,
		"media_performance": mediaPerformance,
		"campaign":          campaignInfo,
		"campaigns":         campaignList,
		"score":             score,
		"metrics":           metrics,
		"analysis_history":  analysisHistory,
		"benchmark":         benchmark,
	})
}

// bsPrStandardJSON: parity prStandard (source + rate + updated_at).
func bsPrStandardJSON(rates bsBrandPrRates) map[string]any {
	resolved := bsResolvePrRates(rates.Rates)
	return map[string]any{
		"source":            rates.Source,
		"rate_per_1k_reach": resolved["global"],
		"rate_tier1":        resolved["tier1"],
		"rate_tier2":        resolved["tier2"],
		"rate_tier3":        resolved["tier3"],
		"updated_at":        rates.UpdatedAt,
	}
}

// bsPublicationMetrics: parity computePublicationMetrics (level 1) + persist.
func (d *Deps) bsPublicationMetrics(ctx context.Context, campaignID string) map[string]any {
	row, _ := d.queryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status <> 'cancelled')::int AS target_media,
		  COUNT(*) FILTER (WHERE status = 'published')::int AS published,
		  COUNT(*) FILTER (WHERE status IN ('pending','waiting_partner','partner_approved','queued','processing','retrying'))::int AS pending,
		  COUNT(*) FILTER (WHERE status = 'failed')::int AS failed,
		  COUNT(*) FILTER (WHERE status = 'cancelled')::int AS cancelled,
		  COUNT(*) FILTER (WHERE scheduled_at IS NOT NULL)::int AS scheduled,
		  COUNT(*) FILTER (WHERE scheduled_at IS NOT NULL AND published_at IS NOT NULL AND published_at <= scheduled_at + interval '24 hours')::int AS scheduled_on_time,
		  ROUND(AVG(CASE WHEN published_at IS NOT NULL THEN EXTRACT(EPOCH FROM (published_at - created_at)) / 3600.0 END), 1)::numeric AS avg_publish_hours
		FROM publications
		WHERE campaign_id = $1`, campaignID)
	medianRow, _ := d.queryRow(ctx, `
		SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (published_at - created_at)) / 3600.0)::numeric AS median
		FROM publications
		WHERE campaign_id = $1 AND published_at IS NOT NULL`, campaignID)
	publishedIDs, _ := d.queryRows(ctx,
		"SELECT id FROM publications WHERE campaign_id = $1 AND status = 'published' ORDER BY published_at ASC", campaignID)

	target, published, pending, failed, cancelled, scheduled, scheduledOnTime := 0, 0, 0, 0, 0, 0, 0
	var avgHours, medianHours any
	if row != nil {
		target = row.intOr("target_media", 0)
		published = row.intOr("published", 0)
		pending = row.intOr("pending", 0)
		failed = row.intOr("failed", 0)
		cancelled = row.intOr("cancelled", 0)
		scheduled = row.intOr("scheduled", 0)
		scheduledOnTime = row.intOr("scheduled_on_time", 0)
		avgHours = row.numPtr("avg_publish_hours")
	}
	if medianRow != nil {
		medianHours = medianRow.numPtr("median")
	}

	var successRate, completionRate, scheduledOnTimePct any
	if target > 0 {
		successRate = math.Round(float64(published)/float64(target)*10000) / 100
		completionRate = math.Round(float64(published+failed+cancelled)/float64(target)*10000) / 100
	}
	if scheduled > 0 {
		scheduledOnTimePct = math.Round(float64(scheduledOnTime)/float64(scheduled)*10000) / 100
	}
	ids := make([]string, 0, len(publishedIDs))
	for _, p := range publishedIDs {
		ids = append(ids, p.strOr("id", ""))
	}

	result := map[string]any{
		"target_media": target, "published": published, "pending": pending,
		"failed": failed, "cancelled": cancelled,
		"success_rate": successRate, "completion_rate": completionRate,
		"avg_publish_hours": avgHours, "median_publish_hours": medianHours,
		"scheduled": scheduled, "scheduled_on_time": scheduledOnTimePct,
		"published_publication_ids": ids,
	}
	d.bsPersistPrMetrics(ctx, campaignID, result)
	return result
}

// bsPersistPrMetrics: parity persistPrMetrics — hanya nilai non-null.
func (d *Deps) bsPersistPrMetrics(ctx context.Context, campaignID string, metrics map[string]any) {
	entries := []struct {
		Key    string
		Source string
	}{
		{"target_media", "Publication records"}, {"published", "Publication records"},
		{"pending", "Publication records"}, {"failed", "Publication records"},
		{"cancelled", "Publication records"}, {"success_rate", "Publication records"},
		{"completion_rate", "Publication records"}, {"avg_publish_hours", "Publication records"},
		{"median_publish_hours", "Publication records"}, {"scheduled", "Publication records"},
		{"scheduled_on_time", "Publication records"},
	}
	type pair struct {
		Key    string
		Value  any
		Source string
	}
	present := []pair{}
	for _, e := range entries {
		v, ok := metrics[e.Key]
		if !ok || v == nil {
			continue
		}
		present = append(present, pair{e.Key, v, e.Source})
	}
	if len(present) == 0 {
		return
	}
	values := make([]string, 0, len(present))
	params := []any{}
	for i, e := range present {
		base := i*5 + 1
		values = append(values, "($"+itoa(base)+", $"+itoa(base+1)+", $"+itoa(base+2)+", $"+itoa(base+3)+", $"+itoa(base+4)+")")
		params = append(params, uuid.NewString(), campaignID, e.Key, e.Value, e.Source)
	}
	_ = d.exec(ctx,
		`INSERT INTO pr_metric_results (id, campaign_id, metric_key, metric_value, source)
		 VALUES `+strings.Join(values, ", ")+`
		 ON CONFLICT (campaign_id, metric_key)
		 DO UPDATE SET metric_value = EXCLUDED.metric_value, source = EXCLUDED.source, calculated_at = NOW()`,
		params...)
}

// bsMediaPerformance: parity computeMediaPerformance.
func (d *Deps) bsMediaPerformance(ctx context.Context, campaignID string) []map[string]any {
	rulesRow, _ := d.queryRow(ctx,
		"SELECT tier_default_quality, category_relevance FROM pr_media_rules WHERE id = 'default' LIMIT 1")
	tierDefaults := map[string]float64{"tier1": 90, "tier2": 80, "tier3": 70}
	categoryRelevance := map[string]float64{}
	if rulesRow != nil {
		if m, ok := rulesRow.jsonField("tier_default_quality").(map[string]any); ok && len(m) > 0 {
			tierDefaults = map[string]float64{}
			for k, v := range m {
				if f, isNum := bsToFloat(v); isNum {
					tierDefaults[k] = f
				}
			}
		}
		if m, ok := rulesRow.jsonField("category_relevance").(map[string]any); ok {
			for k, v := range m {
				if f, isNum := bsToFloat(v); isNum {
					categoryRelevance[k] = f
				}
			}
		}
	}
	pubs, err := d.queryRows(ctx, `
		SELECT p.id AS publication_id, p.status, p.created_at, p.published_at, p.client_price,
		       m.id AS media_id, m.name AS media_name, m.category AS media_category,
		       m.media_tier, m.media_quality_score, m.website_url,
		       v.status AS verification_status, v.compliance_score, v.error_message AS verification_error
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN pr_publication_verifications v ON v.publication_id = p.id
		WHERE p.campaign_id = $1
		ORDER BY p.created_at ASC`, campaignID)
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(pubs))
	for _, row := range pubs {
		tier := row.strOr("media_tier", "tier3")
		var mediaQuality *float64
		if row.has("media_quality_score") {
			v := row.numOr("media_quality_score")
			mediaQuality = &v
		}
		var tierDefault *float64
		if v, ok := tierDefaults[tier]; ok {
			tierDefault = &v
		}
		var qualityScore any
		var qualitySource any
		if mediaQuality != nil {
			qualityScore = *mediaQuality
			qualitySource = "media"
		} else if tierDefault != nil {
			qualityScore = *tierDefault
			qualitySource = "tier_default"
		}
		var publishHours any
		createdAt, hasCreated := row.raw("created_at").(time.Time)
		publishedAt, hasPublished := row.raw("published_at").(time.Time)
		if hasCreated && hasPublished {
			publishHours = math.Round(float64(publishedAt.Sub(createdAt))/float64(time.Hour)*10) / 10
		}
		mediaName := row.strOr("media_name", "Media")
		if mediaName == "" {
			mediaName = "Media"
		}
		out = append(out, map[string]any{
			"publication_id": row.raw("publication_id"), "status": row.raw("status"),
			"media_id": row.raw("media_id"), "media_name": mediaName,
			"media_category": row.raw("media_category"), "media_tier": tier,
			"media_quality_score": qualityScore, "quality_source": qualitySource,
			"relevance":     bsRelevanceForCategory(row.strOr("media_category", ""), categoryRelevance),
			"publish_hours": publishHours, "published_at": row.timePtr("published_at"),
			"created_at": row.timePtr("created_at"), "client_price": row.numOr("client_price"),
			"compliance_status": row.raw("verification_status"), "compliance_score": row.numPtr("compliance_score"),
			"verification_error": row.raw("verification_error"),
		})
	}
	return out
}

// bsRelevanceForCategory: parity relevanceForCategory (cocok persis lalu parsial).
func bsRelevanceForCategory(category string, rules map[string]float64) any {
	if category == "" {
		return nil
	}
	if v, ok := rules[category]; ok {
		return v
	}
	needle := strings.ToLower(category)
	for key, value := range rules {
		candidate := strings.ToLower(key)
		if strings.Contains(candidate, needle) || strings.Contains(needle, candidate) {
			return value
		}
	}
	return nil
}

// bsAnalysisHistory: parity computeAnalysisHistory.
func (d *Deps) bsAnalysisHistory(ctx context.Context, campaignID, from, to string) ([]map[string]any, error) {
	rangeClause := ""
	params := []any{campaignID}
	if from != "" && bsParsableDate(from) {
		params = append(params, from)
		rangeClause += " AND p.published_at >= $" + itoa(len(params))
	}
	if to != "" && bsParsableDate(to) {
		params = append(params, to)
		rangeClause += " AND p.published_at <= $" + itoa(len(params))
	}
	pubs, err := d.queryRows(ctx, `
		SELECT p.id AS publication_id, p.status, p.published_at, p.published_url,
		       d.title AS document_title, m.name AS media_name, m.media_tier,
		       ma.consistency_score, ma.status AS message_status,
		       sa.sentiment, sa.confidence AS sentiment_confidence,
		       v.status AS compliance_status, v.compliance_score, v.error_message AS verification_error
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		LEFT JOIN pr_message_analyses ma ON ma.publication_id = p.id
		LEFT JOIN pr_sentiment_analyses sa ON sa.publication_id = p.id
		LEFT JOIN pr_publication_verifications v ON v.publication_id = p.id
		WHERE p.campaign_id = $1 AND p.status = 'published'`+rangeClause+`
		ORDER BY p.published_at DESC NULLS LAST, p.created_at DESC`, params...)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(pubs))
	for _, row := range pubs {
		out = append(out, map[string]any{
			"publication_id": row.raw("publication_id"), "document_title": row.raw("document_title"),
			"media_name": row.raw("media_name"), "media_tier": row.strOr("media_tier", "tier3"),
			"status": row.raw("status"), "published_at": row.timePtr("published_at"),
			"published_url": row.raw("published_url"), "message_score": row.numPtr("consistency_score"),
			"message_status": row.raw("message_status"), "sentiment": row.raw("sentiment"),
			"sentiment_confidence": row.numPtr("sentiment_confidence"),
			"compliance_status":    row.raw("compliance_status"),
			"compliance_score":     row.numPtr("compliance_score"),
			"verification_error":   row.raw("verification_error"),
		})
	}
	return out, nil
}

// bsPrBenchmark: parity computePrBenchmark (min 5 peer).
func (d *Deps) bsPrBenchmark(ctx context.Context, campaignID, from, to string) map[string]any {
	const minPeers = 5
	yours, _ := d.queryRow(ctx, `
		SELECT cs.score, cs.status, cs.breakdown, mr.metric_value AS success_rate
		FROM pr_campaign_scores cs
		LEFT JOIN pr_metric_results mr ON mr.campaign_id = cs.campaign_id AND mr.metric_key = 'success_rate'
		WHERE cs.campaign_id = $1 ORDER BY cs.profile_version DESC LIMIT 1`, campaignID)

	peerWhere := []string{"cs.campaign_id <> $1", "cs.score IS NOT NULL", "cs.status IN ('partial', 'ready', 'final')"}
	peerParams := []any{campaignID}
	if from != "" {
		parsed, ok := parseISO(from)
		if ok {
			peerParams = append(peerParams, parsed.UTC())
			peerWhere = append(peerWhere, "c.starts_at >= $"+itoa(len(peerParams)))
		}
	}
	if to != "" {
		parsed, ok := parseISO(to)
		if ok {
			peerParams = append(peerParams, parsed.UTC())
			peerWhere = append(peerWhere, "c.starts_at <= $"+itoa(len(peerParams)))
		}
	}
	peers, err := d.queryRows(ctx, `
		SELECT cs.score, cs.breakdown, mr.metric_value AS success_rate
		FROM pr_campaign_scores cs
		JOIN campaigns c ON c.id = cs.campaign_id
		LEFT JOIN pr_metric_results mr ON mr.campaign_id = cs.campaign_id AND mr.metric_key = 'success_rate'
		WHERE `+strings.Join(peerWhere, " AND ")+`
		ORDER BY cs.campaign_id`, peerParams...)
	if err != nil {
		peers = []bsRow{}
	}
	periodNote := ""
	if from != "" || to != "" {
		periodNote = " dalam periode terpilih"
	}
	if len(peers) < minPeers {
		reason := "Belum cukup data historis" + periodNote + " (butuh minimal " + itoa(minPeers) +
			" kampanye pembanding, sekarang " + itoa(len(peers)) + ")."
		if len(peers) == 0 {
			reason = "Belum ada kampanye lain dengan skor terhitung" + periodNote + "."
		}
		return map[string]any{
			"available": false, "reason": reason, "peers": len(peers), "comparison": nil,
		}
	}
	yoursIndicators := map[string]float64{}
	var yoursSuccess any
	if yours != nil {
		yoursIndicators = bsParseIndicators(yours.jsonField("breakdown"))
		if yours.has("success_rate") {
			yoursSuccess = yours.numOr("success_rate")
		}
	}
	peerIndicators := make([]map[string]float64, 0, len(peers))
	peerScores := make([]any, 0, len(peers))
	peerSuccessRates := make([]any, 0, len(peers))
	for _, peer := range peers {
		peerIndicators = append(peerIndicators, bsParseIndicators(peer.jsonField("breakdown")))
		if peer.has("score") {
			peerScores = append(peerScores, peer.numOr("score"))
		} else {
			peerScores = append(peerScores, nil)
		}
		if peer.has("success_rate") {
			peerSuccessRates = append(peerSuccessRates, peer.numOr("success_rate"))
		} else {
			peerSuccessRates = append(peerSuccessRates, nil)
		}
	}
	var yoursScore any
	if yours != nil && yours.has("score") {
		yoursScore = yours.numOr("score")
	}
	return map[string]any{
		"available": true, "reason": nil, "peers": len(peers),
		"comparison": map[string]any{
			"score":        map[string]any{"yours": yoursScore, "peers": bsAverageNumbers(peerScores)},
			"success_rate": map[string]any{"yours": yoursSuccess, "peers": bsAverageNumbers(peerSuccessRates)},
			"coverage":     bsBenchmarkMetric(yoursIndicators, peerIndicators, "coverage"),
			"message":      bsBenchmarkMetric(yoursIndicators, peerIndicators, "message"),
			"sentiment":    bsBenchmarkMetric(yoursIndicators, peerIndicators, "sentiment"),
			"compliance":   bsBenchmarkMetric(yoursIndicators, peerIndicators, "compliance"),
		},
	}
}

// bsBenchmarkMetric: {yours, peers} untuk satu indikator.
func bsBenchmarkMetric(yours map[string]float64, peers []map[string]float64, key string) map[string]any {
	var yoursVal any
	if v, ok := yours[key]; ok {
		yoursVal = v
	}
	values := make([]any, 0, len(peers))
	for _, p := range peers {
		if v, ok := p[key]; ok {
			values = append(values, v)
		} else {
			values = append(values, nil)
		}
	}
	return map[string]any{"yours": yoursVal, "peers": bsAverageNumbers(values)}
}

// bsAverageNumbers: rata-rata nilai valid (nil bila tidak ada), 2 desimal.
func bsAverageNumbers(values []any) any {
	valid := []float64{}
	for _, v := range values {
		if v == nil {
			continue
		}
		if f, ok := bsToFloat(v); ok {
			valid = append(valid, f)
		}
	}
	if len(valid) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range valid {
		sum += v
	}
	return math.Round(sum/float64(len(valid))*100) / 100
}

// bsParseIndicators: parity parseIndicators — breakdown.indicators[].{key,score}.
func bsParseIndicators(breakdown any) map[string]float64 {
	out := map[string]float64{}
	raw, ok := breakdown.(map[string]any)
	if !ok {
		return out
	}
	indicators, ok := raw["indicators"].([]any)
	if !ok {
		return out
	}
	for _, item := range indicators {
		ind, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key, hasKey := ind["key"].(string)
		if !hasKey || key == "" {
			continue
		}
		if ind["score"] == nil {
			continue
		}
		if f, isNum := bsToFloat(ind["score"]); isNum {
			out[key] = f
		}
	}
	return out
}

// bsToFloat: nilai apa pun → float64 bila finite.
func bsToFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return 0, false
		}
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// ---------- PR score engine (port lib/pr/scoring.ts) ----------

// bsDefaultPrWeights: parity DEFAULT_PR_WEIGHTS.
var bsDefaultPrWeights = map[string]float64{
	"publication": 25, "coverage": 30, "message": 20, "sentiment": 15, "compliance": 10,
}

var bsDefaultPrThresholds = map[string]float64{
	"excellent": 85, "strong": 70, "good": 50, "needs_improvement": 0,
}

// bsPrScore: port calculateCampaignPrScore + buildScoreResponse.
func (d *Deps) bsPrScore(ctx context.Context, campaignID string, recalc bool, userID string) map[string]any {
	profile, _ := d.queryRow(ctx,
		"SELECT * FROM pr_scoring_profiles WHERE status = 'active' ORDER BY version DESC LIMIT 1")
	if profile == nil {
		id := uuid.NewString()
		weightsJSON, _ := json.Marshal(bsDefaultPrWeights)
		thresholdsJSON, _ := json.Marshal(bsDefaultPrThresholds)
		if err := d.exec(ctx,
			"INSERT INTO pr_scoring_profiles (id, version, name, weights, thresholds) VALUES ($1, 1, 'Baseline v1', $2::jsonb, $3::jsonb)",
			id, string(weightsJSON), string(thresholdsJSON)); err == nil {
			profile, _ = d.queryRow(ctx, "SELECT * FROM pr_scoring_profiles WHERE id = $1 LIMIT 1", id)
		}
	}
	profileVersion := 1
	profileID := ""
	weights := map[string]float64{}
	for k, v := range bsDefaultPrWeights {
		weights[k] = v
	}
	thresholds := map[string]float64{}
	for k, v := range bsDefaultPrThresholds {
		thresholds[k] = v
	}
	if profile != nil {
		profileVersion = profile.intOr("version", 1)
		profileID = profile.strOr("id", "")
		weights = bsParseWeights(profile.jsonField("weights"))
		thresholds = bsParseThresholds(profile.jsonField("thresholds"))
	}

	if !recalc {
		existing, _ := d.queryRow(ctx,
			"SELECT * FROM pr_campaign_scores WHERE campaign_id = $1 AND profile_version = $2 LIMIT 1",
			campaignID, profileVersion)
		if existing != nil {
			return bsBuildScoreResponse(existing, weights, thresholds)
		}
	}

	metrics := d.bsPublicationMetrics(ctx, campaignID)
	target := bsAnyInt(metrics["target_media"])
	hasAny := target > 0
	if !hasAny {
		indicators, _ := json.Marshal([]any{})
		inputs, _ := json.Marshal(map[string]any{"target_media": 0})
		weightsJSON, _ := json.Marshal(weights)
		d.bsUpsertCampaignScore(ctx, campaignID, profileID, profileVersion, string(weightsJSON),
			"not_ready", nil, nil, string(indicators), string(inputs), userID)
		snapshot := map[string]any{
			"campaign_id": campaignID, "profile_version": profileVersion,
			"status": "not_ready", "score": nil, "label": nil,
			"breakdown": map[string]any{"indicators": []any{}}, "calculated_at": isoNow(),
		}
		return bsBuildScoreResponse(bsRow(snapshot), weights, thresholds)
	}

	successRate := metrics["success_rate"]
	pending := bsAnyInt(metrics["pending"])
	publishedIDs := bsAnyStringSlice(metrics["published_publication_ids"])
	failed := bsAnyInt(metrics["failed"])
	published := bsAnyInt(metrics["published"])

	indicators := []map[string]any{}
	if successRate != nil {
		indicators = append(indicators, map[string]any{
			"key": "publication", "label": "Publication Performance", "score": successRate,
			"weight": weights["publication"], "source": "Publication records",
			"detail": map[string]any{
				"published": published, "target": target, "failed": failed, "pending": pending,
				"avg_publish_hours": metrics["avg_publish_hours"],
			},
		})
	}
	if cov := d.bsCoverageQuality(ctx, publishedIDs); cov != nil {
		cov["weight"] = weights["coverage"]
		indicators = append(indicators, cov)
	}
	if msg := d.bsMessageConsistency(ctx, publishedIDs); msg != nil {
		msg["weight"] = weights["message"]
		indicators = append(indicators, msg)
	}
	if sent := d.bsSentiment(ctx, publishedIDs); sent != nil {
		sent["weight"] = weights["sentiment"]
		indicators = append(indicators, sent)
	}
	if comp := d.bsCompliance(ctx, publishedIDs); comp != nil {
		comp["weight"] = weights["compliance"]
		indicators = append(indicators, comp)
	}

	usedWeight := 0.0
	for _, ind := range indicators {
		usedWeight += bsAnyFloat(ind["weight"])
	}
	var score *float64
	if usedWeight > 0 {
		sum := 0.0
		for _, ind := range indicators {
			if ind["score"] != nil {
				sum += bsAnyFloat(ind["score"]) * bsAnyFloat(ind["weight"])
			}
		}
		v := math.Round(sum/usedWeight*100) / 100
		score = &v
	}
	status := "final"
	if pending > 0 {
		status = "partial"
	}
	indicatorJSON := make([]map[string]any, 0, len(indicators))
	for _, ind := range indicators {
		indicatorJSON = append(indicatorJSON, ind)
	}
	breakdown := map[string]any{"indicators": indicatorJSON}
	inputs := map[string]any{"metrics_summary": map[string]any{
		"target": target, "published": published, "pending": pending,
		"failed": failed, "cancelled": bsAnyInt(metrics["cancelled"]),
	}}
	weightsJSON, _ := json.Marshal(weights)
	var label any
	if score != nil {
		label = bsPrScoreLabel(*score, thresholds)
	}
	d.bsUpsertCampaignScore(ctx, campaignID, profileID, profileVersion, string(weightsJSON),
		status, score, label, marshalJSONMap(breakdown), marshalJSONMap(inputs), userID)

	// Snapshot in-memory: nilai harus sudah tipe JSON-native (bukan pointer),
	// supaya buildScoreResponse membacanya dengan benar.
	var scoreValue any
	if score != nil {
		scoreValue = *score
	}
	snapshot := map[string]any{
		"campaign_id": campaignID, "profile_version": profileVersion, "status": status,
		"score": scoreValue, "label": label, "breakdown": breakdown,
		"calculated_at": isoNow(),
	}
	return bsBuildScoreResponse(bsRow(snapshot), weights, thresholds)
}

// bsCoverageQuality: parity computeCoverageQuality.
func (d *Deps) bsCoverageQuality(ctx context.Context, ids []string) map[string]any {
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.queryRows(ctx,
		"SELECT m.media_quality_score, m.media_tier, m.name AS media_name FROM publications p JOIN media m ON m.id = p.media_id WHERE p.id = ANY($1)",
		ids)
	if err != nil {
		return nil
	}
	scores := []float64{}
	tierBreakdown := map[string]int{}
	for _, row := range rows {
		if row.has("media_quality_score") {
			scores = append(scores, row.numOr("media_quality_score"))
		}
		tier := row.strOr("media_tier", "tier3")
		tierBreakdown[tier]++
	}
	if len(scores) == 0 {
		return nil
	}
	avg := math.Round(bsSum(scores)/float64(len(scores))*100) / 100
	tierAny := map[string]any{}
	for k, v := range tierBreakdown {
		tierAny[k] = v
	}
	return map[string]any{
		"key": "coverage", "label": "Coverage Quality", "score": avg, "weight": 0.0,
		"source": "Media Profile (published media)",
		"detail": map[string]any{
			"media_quality": scores, "published_with_quality": len(scores),
			"total_published": len(ids), "tier_breakdown": tierAny,
		},
	}
}

// bsMessageConsistency: parity computeMessageConsistency.
func (d *Deps) bsMessageConsistency(ctx context.Context, ids []string) map[string]any {
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.queryRows(ctx,
		"SELECT consistency_score, status, error_message FROM pr_message_analyses WHERE publication_id = ANY($1) AND status = 'completed'", ids)
	if err != nil {
		return nil
	}
	scores := []float64{}
	for _, row := range rows {
		if row.has("consistency_score") {
			scores = append(scores, row.numOr("consistency_score"))
		}
	}
	if len(scores) == 0 {
		return nil
	}
	avg := math.Round(bsSum(scores)/float64(len(scores))*100) / 100
	return map[string]any{
		"key": "message", "label": "Message Consistency", "score": avg, "weight": 0.0,
		"source": "Key Messages + Published Article Analysis",
		"detail": map[string]any{
			"analyzed": len(scores), "total": len(ids), "consistency_scores": scores,
		},
	}
}

// bsSentiment: parity computeSentiment.
func (d *Deps) bsSentiment(ctx context.Context, ids []string) map[string]any {
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.queryRows(ctx,
		"SELECT sentiment, COUNT(*)::int AS count FROM pr_sentiment_analyses WHERE publication_id = ANY($1) AND status = 'completed' AND sentiment IS NOT NULL GROUP BY sentiment", ids)
	if err != nil {
		return nil
	}
	counts := map[string]int{"positive": 0, "neutral": 0, "negative": 0}
	total := 0
	for _, row := range rows {
		s := row.strOr("sentiment", "")
		count := row.intOr("count", 0)
		if _, ok := counts[s]; ok {
			counts[s] = count
		}
		total += count
	}
	if total == 0 {
		return nil
	}
	score := math.Round((float64(counts["positive"])*100+float64(counts["neutral"])*60+float64(counts["negative"])*20)/float64(total)*100) / 100
	return map[string]any{
		"key": "sentiment", "label": "Sentiment / Tone", "score": score, "weight": 0.0,
		"source": "Published Article Analysis (coverage tone)",
		"detail": map[string]any{
			"positive": counts["positive"], "neutral": counts["neutral"],
			"negative": counts["negative"], "analyzed": total,
		},
	}
}

// bsCompliance: parity computeCompliance.
func (d *Deps) bsCompliance(ctx context.Context, ids []string) map[string]any {
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.queryRows(ctx,
		"SELECT compliance_score, status FROM pr_publication_verifications WHERE publication_id = ANY($1) AND status IN ('passed','failed')", ids)
	if err != nil {
		return nil
	}
	scores := []float64{}
	passed, failed := 0, 0
	for _, row := range rows {
		if row.has("compliance_score") {
			scores = append(scores, row.numOr("compliance_score"))
		}
		switch row.strOr("status", "") {
		case "passed":
			passed++
		case "failed":
			failed++
		}
	}
	if len(scores) == 0 {
		return nil
	}
	avg := math.Round(bsSum(scores)/float64(len(scores))*100) / 100
	return map[string]any{
		"key": "compliance", "label": "Publication Compliance", "score": avg, "weight": 0.0,
		"source": "Publication Verification (fetch URL terbit)",
		"detail": map[string]any{
			"verified": len(scores), "passed": passed, "failed": failed, "total": len(ids),
		},
	}
}

// bsUpsertCampaignScore: parity upsertCampaignScore.
func (d *Deps) bsUpsertCampaignScore(ctx context.Context, campaignID, profileID string, version int,
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

// bsBuildScoreResponse: parity buildScoreResponse (methodology + insights).
func bsBuildScoreResponse(row bsRow, weights, thresholds map[string]float64) map[string]any {
	breakdown := map[string]any{}
	if raw, ok := row.jsonField("breakdown").(map[string]any); ok {
		breakdown = raw
	}
	indicators := []map[string]any{}
	if arr, ok := breakdown["indicators"].([]any); ok {
		for _, item := range arr {
			if m, isMap := item.(map[string]any); isMap {
				indicators = append(indicators, m)
			}
		}
	}
	var score any
	if row.has("score") {
		score = row.numOr("score")
	}
	status := row.strOr("status", "not_ready")
	if status == "" {
		status = "not_ready"
	}
	totalWeight := 0.0
	for _, v := range weights {
		totalWeight += v
	}
	usedWeight := 0.0
	for _, ind := range indicators {
		usedWeight += bsAnyFloat(ind["weight"])
	}
	contribution := make([]map[string]any, 0, len(indicators))
	for _, ind := range indicators {
		var contrib any
		if ind["score"] != nil && usedWeight > 0 {
			contrib = math.Round(bsAnyFloat(ind["score"])*bsAnyFloat(ind["weight"])/usedWeight*100) / 100
		}
		contribution = append(contribution, map[string]any{
			"key": ind["key"], "label": ind["label"], "score": ind["score"],
			"weight": ind["weight"], "contribution": contrib,
		})
	}
	profileVersion := row.intOr("profile_version", 0)
	return map[string]any{
		"campaign_id":     row.strOr("campaign_id", ""),
		"profile_version": profileVersion,
		"profile_name":    "Scoring v" + itoa(profileVersion),
		"weights":         weights, "status": status, "score": score, "label": row.raw("label"),
		"indicators": indicators,
		"methodology": map[string]any{
			"contribution": contribution, "total_weight": totalWeight, "used_weight": usedWeight,
		},
		"insights":      bsBuildInsights(indicators, status),
		"calculated_at": bsTimeOrString(row.raw("calculated_at")),
	}
}

// bsTimeOrString: nilai kolom waktu apa pun → ISO string (time.Time) atau nilai
// yang sudah string (snapshot in-memory memakai isoNow()).
func bsTimeOrString(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case time.Time:
		return isoMilli(t)
	case *time.Time:
		if t == nil {
			return nil
		}
		return isoMilli(*t)
	}
	return v
}

// bsBuildInsights: parity buildInsights.
func bsBuildInsights(indicators []map[string]any, status string) []string {
	insights := []string{}
	if status == "not_ready" {
		return append(insights, "Campaign belum memiliki publikasi — skor belum dapat dihitung.")
	}
	if len(indicators) == 0 {
		return append(insights, "Data yang dapat diverifikasi belum tersedia untuk indikator apa pun.")
	}
	pub := bsFindIndicator(indicators, "publication")
	if pub != nil && pub["score"] != nil {
		v := bsAnyFloat(pub["score"])
		if v >= 85 {
			insights = append(insights, "Tingkat keberhasilan distribusi tinggi (publication success rate).")
		} else if v < 50 {
			insights = append(insights, "Tingkat keberhasilan distribusi masih rendah — banyak target belum terbit.")
		}
	}
	msg := bsFindIndicator(indicators, "message")
	if msg != nil && msg["score"] != nil {
		v := bsAnyFloat(msg["score"])
		if v < 70 {
			insights = append(insights, "Message consistency perlu ditingkatkan — sebagian key message belum terverifikasi pada artikel terbit.")
		} else if v >= 85 {
			insights = append(insights, "Key message client tersampaikan dengan konsisten pada coverage yang dianalisis.")
		}
	}
	cov := bsFindIndicator(indicators, "coverage")
	if cov != nil {
		if detail, ok := cov["detail"].(map[string]any); ok {
			if tb, ok := detail["tier_breakdown"].(map[string]any); ok {
				tier1 := bsAnyInt(tb["tier1"])
				if tier1 > 0 {
					insights = append(insights, "Sebagian besar coverage berada pada media Tier 1 ("+itoa(tier1)+" publikasi).")
				}
			}
		}
	}
	sent := bsFindIndicator(indicators, "sentiment")
	if sent != nil {
		if detail, ok := sent["detail"].(map[string]any); ok {
			negative := bsAnyInt(detail["negative"])
			if negative > 0 {
				insights = append(insights, itoa(negative)+" coverage terklasifikasi ber-tone negatif — tinjau konteks pemberitaan.")
			}
		}
	}
	comp := bsFindIndicator(indicators, "compliance")
	if comp != nil && comp["score"] != nil && bsAnyFloat(comp["score"]) < 70 {
		insights = append(insights, "Sebagian publikasi belum memenuhi publication compliance — verifikasi URL terbit perlu ditindaklanjuti.")
	}
	insights = append(insights, "Skor dihitung dari data distribusi dan kualitas coverage yang dapat diverifikasi — bukan AVE dan bukan jaminan dampak bisnis.")
	return insights
}

func bsFindIndicator(indicators []map[string]any, key string) map[string]any {
	for _, ind := range indicators {
		if s, ok := ind["key"].(string); ok && s == key {
			return ind
		}
	}
	return nil
}

// bsParseWeights: parity parseWeights (default + override finite).
func bsParseWeights(value any) map[string]float64 {
	out := map[string]float64{}
	for k, v := range bsDefaultPrWeights {
		out[k] = v
	}
	raw := bsParseNumberMap(value)
	for k := range bsDefaultPrWeights {
		if v, ok := raw[k]; ok {
			out[k] = v
		}
	}
	return out
}

// bsParseThresholds: parity parseThresholds (merge dengan default).
func bsParseThresholds(value any) map[string]float64 {
	out := map[string]float64{}
	for k, v := range bsDefaultPrThresholds {
		out[k] = v
	}
	for k, v := range bsParseNumberMap(value) {
		out[k] = v
	}
	return out
}

// bsParseNumberMap: objek JSON (atau string JSON) → angka finite.
func bsParseNumberMap(value any) map[string]float64 {
	out := map[string]float64{}
	var raw any = value
	if s, isStr := value.(string); isStr {
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			return out
		}
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	for k, v := range obj {
		if f, isNum := bsToFloat(v); isNum {
			out[k] = f
		}
	}
	return out
}

// bsPrScoreLabel: parity scoreLabel.
func bsPrScoreLabel(score float64, thresholds map[string]float64) string {
	excellent := thresholds["excellent"]
	if excellent == 0 {
		excellent = 85
	}
	strong := thresholds["strong"]
	if strong == 0 {
		strong = 70
	}
	good := thresholds["good"]
	if good == 0 {
		good = 50
	}
	switch {
	case score >= excellent:
		return "Excellent"
	case score >= strong:
		return "Strong Performance"
	case score >= good:
		return "Good"
	default:
		return "Needs Improvement"
	}
}

// ---------- util ----------

func bsSum(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum
}

// bsAnyFloat: nilai apa pun → float64 (0 bila tidak dikenali).
func bsAnyFloat(v any) float64 {
	f, _ := bsToFloat(v)
	return f
}

// bsAnyInt: nilai apa pun → int.
func bsAnyInt(v any) int {
	f, ok := bsToFloat(v)
	if !ok {
		return 0
	}
	return int(f)
}

// bsAnyStringSlice: nilai apa pun → []string.
func bsAnyStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		if list, isStrings := v.([]string); isStrings {
			return list
		}
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, isStr := item.(string); isStr {
			out = append(out, s)
		}
	}
	return out
}

// bsIsUUID: parity z.string().uuid() (versi 1..8 atau nil/ffff spesial).
func bsIsUUID(value string) bool {
	if _, err := uuid.Parse(value); err == nil {
		return true
	}
	switch value {
	case "00000000-0000-0000-0000-000000000000", "ffffffff-ffff-ffff-ffff-ffffffffffff":
		return true
	}
	return false
}

// ---------- payments (port lib/payments/service.ts createPayment) ----------

type bsPaymentInput struct {
	UserID         string
	BrandID        string
	Amount         float64
	Currency       string
	Description    string
	Purpose        string
	PurposeRefID   string
	PayerEmail     string
	CustomerName   string
	ReturnURL      string
	InvoiceData    any
	IdempotencyKey string
}

// bsCreatePayment: port createPayment — gateway aktif + idempotency key,
// attempt langsung ke provider (parity adapter createPayment).
func (d *Deps) bsCreatePayment(ctx context.Context, input bsPaymentInput) (bsRow, error) {
	active, err := d.queryRow(ctx, bsPaymentConfigSelect+" WHERE is_active = TRUE LIMIT 1")
	if err != nil {
		return nil, err
	}
	if active == nil {
		return nil, errors.New("Belum ada payment gateway aktif. Hubungi admin.")
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if len(key) > 100 {
		key = key[:100]
	}
	if key != "" && input.BrandID != "" {
		existing, _ := d.queryRow(ctx,
			"SELECT * FROM payment_transactions WHERE brand_id = $1 AND idempotency_key = $2 ORDER BY created_at DESC LIMIT 1",
			input.BrandID, key)
		if existing != nil {
			if existing.strOr("status", "") == "failed" && !existing.has("payment_url") {
				return d.bsAttemptGatewayPayment(ctx, input, active, existing.strOr("id", ""), key)
			}
			return existing, nil
		}
	}
	return d.bsAttemptGatewayPayment(ctx, input, active, "", key)
}

// bsPaymentConfigSelect: kolom konfigurasi gateway.
const bsPaymentConfigSelect = `SELECT id, provider, environment, encrypted_credentials, is_configured, is_active,
	last_tested_at, last_test_ok, created_by, created_at, updated_at FROM payment_gateway_configs`

// bsAttemptGatewayPayment: port attemptGatewayPayment.
func (d *Deps) bsAttemptGatewayPayment(ctx context.Context, input bsPaymentInput, gateway bsRow, existingID, idempotencyKey string) (bsRow, error) {
	transactionID := existingID
	provider := gateway.strOr("provider", "")
	if transactionID == "" {
		transactionID = uuid.NewString()
		currency := input.Currency
		if currency == "" {
			currency = "IDR"
		}
		purpose := input.Purpose
		if purpose == "" {
			purpose = "generic"
		}
		var invoiceArg any
		if input.InvoiceData != nil {
			raw, mErr := json.Marshal(input.InvoiceData)
			if mErr == nil {
				invoiceArg = string(raw)
			}
		}
		var keyArg any
		if idempotencyKey != "" {
			keyArg = idempotencyKey
		}
		var refArg any
		if input.PurposeRefID != "" {
			refArg = input.PurposeRefID
		}
		_, insertErr := d.Pool.Exec(ctx,
			`INSERT INTO payment_transactions (id, user_id, brand_id, gateway_provider, gateway_config_id, gateway_environment, external_transaction_id, amount, currency, status, purpose, purpose_ref_id, invoice_data, idempotency_key, reference, expired_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', $10, $11, $12::jsonb, $13,
			         'PAY-' || to_char(NOW(), 'YYYYMM') || '-' || lpad(nextval('payment_transaction_ref_seq')::text, 5, '0'),
			         NOW() + interval '24 hours')`,
			transactionID, input.UserID, nullIfEmpty(input.BrandID), provider,
			gateway.strOr("id", ""), gateway.strOr("environment", ""), transactionID,
			input.Amount, currency, purpose, refArg, invoiceArg, keyArg)
		if insertErr != nil {
			if bsIsUniqueViolation(insertErr) && input.BrandID != "" && idempotencyKey != "" {
				existing, _ := d.queryRow(ctx,
					"SELECT * FROM payment_transactions WHERE brand_id = $1 AND idempotency_key = $2 ORDER BY created_at DESC LIMIT 1",
					input.BrandID, idempotencyKey)
				if existing != nil {
					return existing, nil
				}
			}
			return nil, insertErr
		}
	}

	credentials := bsUnsealCredentials(d, gateway.strOr("encrypted_credentials", ""))
	purposeRef := input.PurposeRefID
	intent, adapterErr := d.bsAdapterCreatePayment(ctx, provider, gateway.strOr("environment", ""), credentials, input, transactionID, &purposeRef)
	if adapterErr != nil {
		_ = d.exec(ctx, "UPDATE payment_transactions SET status = 'failed', updated_at = NOW() WHERE id = $1", transactionID)
		failed, _ := d.queryRow(ctx, "SELECT * FROM payment_transactions WHERE id = $1 LIMIT 1", transactionID)
		return failed, adapterErr
	}
	if err := d.exec(ctx,
		"UPDATE payment_transactions SET payment_url = $1, payment_method = $2, status = $3, provider_reference = $4, updated_at = NOW() WHERE id = $5",
		intent.PaymentURL, intent.PaymentMethod, bsNormalizePaymentStatus(provider, intent.RawStatus),
		intent.ProviderReference, transactionID); err != nil {
		return nil, err
	}
	updated, _ := d.queryRow(ctx, "SELECT * FROM payment_transactions WHERE id = $1 LIMIT 1", transactionID)
	if updated == nil {
		return nil, errors.New("Payment transaction not found")
	}
	return updated, nil
}

// bsIsUniqueViolation: kode PostgreSQL 23505.
func bsIsUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return strings.Contains(err.Error(), "23505")
}

// bsUnsealCredentials: coba semua kandidat secret (parity unsealCredentials).
func bsUnsealCredentials(d *Deps, encrypted string) map[string]string {
	for _, secret := range bsSecretCandidates() {
		plain, err := unsealWithSecret(encrypted, secret)
		if err != nil {
			continue
		}
		var parsed map[string]string
		if json.Unmarshal([]byte(plain), &parsed) != nil {
			continue
		}
		if parsed == nil {
			return map[string]string{}
		}
		return parsed
	}
	return map[string]string{}
}

// bsPaymentIntent: hasil adapter createPayment.
type bsPaymentIntent struct {
	RawStatus         string
	PaymentURL        any
	PaymentMethod     any
	ProviderReference any
}

// bsSecretCandidates: parity secretCandidates (env saja; cukup untuk unseal
// konfigurasi yang disegel dengan secret proses).
func bsSecretCandidates() []string {
	return ai.SecretCandidates()
}

// unsealWithSecret: shim crypto.Unseal.
func unsealWithSecret(sealed, secret string) (string, error) {
	return crypto.Unseal(sealed, secret)
}

// urlValues: alias url.Values (kredensial Duitku form-encoded).
type urlValues = url.Values

// urlPathEscape: shim url.PathEscape.
func urlPathEscape(s string) string { return url.PathEscape(s) }

// bsNormalizePaymentStatus: parity normalizeProviderStatus (case-insensitive).
func bsNormalizePaymentStatus(provider, raw string) string {
	if raw == "" {
		return "pending"
	}
	m := bsPayStatusMap[provider]
	if status, ok := m[strings.ToUpper(raw)]; ok {
		return status
	}
	if status, ok := m[raw]; ok {
		return status
	}
	return "pending"
}

var bsPayStatusMap = map[string]map[string]string{
	"xendit": {
		"PENDING": "pending", "PROCESSING": "processing", "REVIEW": "processing",
		"PAID": "paid", "COMPLETED": "paid", "SETTLED": "paid",
		"FAILED": "failed", "EXPIRED": "expired", "VOIDED": "cancelled", "REFUNDED": "refunded",
	},
	"duitku": {
		"WAITING": "pending", "PROCESSING": "processing", "SUCCESS": "paid", "PAID": "paid",
		"FAILED": "failed", "EXPIRED": "expired", "CANCELLED": "cancelled",
	},
	"midtrans": {
		"PENDING": "pending", "CAPTURE": "processing", "AUTHORIZE": "processing", "CHALLENGE": "processing",
		"SETTLEMENT": "paid", "SUCCESS": "paid", "CANCEL": "cancelled", "EXPIRE": "expired",
		"DENY": "failed", "FAILURE": "failed", "REFUND": "refunded", "PARTIAL_REFUND": "refunded",
	},
	"tripay": {
		"UNPAID": "pending", "PAID": "paid", "EXPIRED": "expired", "FAILED": "failed",
		"CANCEL": "cancelled", "CANCELLED": "cancelled", "REFUND": "refunded",
	},
	"sumopod": {
		"PENDING": "pending", "PAID": "paid", "COMPLETED": "paid", "FAILED": "failed",
		"EXPIRED": "expired", "CANCELLED": "cancelled",
	},
	"sandbox": {
		"UNPAID": "pending", "PAID": "paid", "SETTLED": "paid", "COMPLETED": "paid",
		"FAILED": "failed", "EXPIRED": "expired", "CANCELLED": "cancelled",
	},
}

// bsPaymentProviderBaseURL: parity baseUrl per provider + environment.
func bsPaymentProviderBaseURL(provider, environment string) string {
	switch provider {
	case "duitku":
		if environment == "sandbox" {
			return "https://sandbox.duitku.com"
		}
		return "https://duitku.com"
	case "midtrans":
		if environment == "sandbox" {
			return "https://app.sandbox.midtrans.com"
		}
		return "https://app.midtrans.com"
	case "tripay":
		if environment == "sandbox" {
			return "https://tripay.co.id/api-sandbox"
		}
		return "https://tripay.co.id/api"
	case "sumopod":
		if environment == "sandbox" {
			return "https://api-pay-sandbox.sumopod.com"
		}
		return "https://api-pay.sumopod.com"
	case "xendit":
		return "https://api.xendit.co"
	}
	return ""
}

// bsPaymentFetch: request HTTP provider (timeout 15s).
func bsPaymentFetch(ctx context.Context, method, target string, headers map[string]string, body string) (int, string) {
	status, text, err := bsHTTPDo(ctx, method, target, headers, body, 15*time.Second)
	if err != nil {
		return 0, ""
	}
	return status, text
}

func bsReadJSONObject(body string) map[string]any {
	var parsed map[string]any
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return map[string]any{}
	}
	return parsed
}

// bsAdapterCreatePayment: port adapter createPayment untuk provider aktif.
func (d *Deps) bsAdapterCreatePayment(ctx context.Context, provider, environment string, credentials map[string]string, input bsPaymentInput, externalID string, purposeRefID *string) (bsPaymentIntent, error) {
	base := bsPaymentProviderBaseURL(provider, environment)
	currency := input.Currency
	if currency == "" {
		currency = "IDR"
	}
	description := input.Description
	if description == "" {
		description = "Pembayaran Tulisio"
	}
	amount := int(math.Round(input.Amount))

	switch provider {
	case "xendit":
		payload, _ := json.Marshal(map[string]any{
			"external_id": externalID, "amount": amount,
			"payer_email": nullIfEmpty(input.PayerEmail), "description": description, "currency": currency,
		})
		headers := map[string]string{"Content-Type": "application/json", "x-api-key": credentials["api_key"]}
		status, body := bsPaymentFetch(ctx, http.MethodPost, base+"/v2/invoices", headers, string(payload))
		data := bsReadJSONObject(body)
		if status < 200 || status >= 300 {
			msg := bsAnyString(data["message"])
			if msg == "" {
				msg = bsAnyString(data["error"])
			}
			if msg == "" {
				msg = "HTTP " + itoa(status)
			}
			return bsPaymentIntent{}, errors.New("Xendit: " + msg)
		}
		rawStatus := bsAnyString(data["status"])
		if rawStatus == "" {
			rawStatus = "PENDING"
		}
		return bsPaymentIntent{
			RawStatus: rawStatus, PaymentURL: data["invoice_url"],
			PaymentMethod: "invoice", ProviderReference: data["id"],
		}, nil

	case "duitku":
		form := urlValues{}
		form.Set("merchantCode", credentials["merchant_code"])
		form.Set("merchantOrderId", externalID)
		form.Set("paymentAmount", itoa(amount))
		form.Set("productDetails", description)
		form.Set("email", input.PayerEmail)
		customerName := input.CustomerName
		if customerName == "" {
			customerName = "Customer Tauthub"
		}
		form.Set("customerVaName", customerName)
		returnURL := input.ReturnURL
		statusURL := "https://app.tauthub.com/payments/status"
		if returnURL != "" {
			form.Set("merchantUrl", returnURL)
			statusURL = returnURL
		} else {
			form.Set("merchantUrl", "https://app.tauthub.com/")
		}
		form.Set("returnUrl", statusURL)
		headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
		status, body := bsPaymentFetch(ctx, http.MethodPost, base+"/webapi/api/merchant/v2/inquiry", headers, form.Encode())
		data := bsReadJSONObject(body)
		if status < 200 || status >= 300 || bsAnyString(data["statusCode"]) != "00" {
			msg := bsAnyString(data["statusMessage"])
			if msg == "" {
				msg = bsAnyString(data["message"])
			}
			if msg == "" {
				msg = "HTTP " + itoa(status)
			}
			return bsPaymentIntent{}, errors.New("Duitku: " + msg)
		}
		return bsPaymentIntent{
			RawStatus: "WAITING", PaymentURL: data["paymentUrl"],
			PaymentMethod: data["paymentMethod"], ProviderReference: data["reference"],
		}, nil

	case "midtrans":
		basic := base64.StdEncoding.EncodeToString([]byte(credentials["server_key"] + ":"))
		payload, _ := json.Marshal(map[string]any{
			"transaction_details": map[string]any{"order_id": externalID, "gross_amount": amount},
			"item_details": []map[string]any{{
				"id": "tauthub-payment", "price": amount, "quantity": 1, "name": description,
			}},
			"customer_details": map[string]any{
				"first_name": bsFirstNonEmpty(input.CustomerName, "Customer"), "email": input.PayerEmail,
			},
		})
		headers := map[string]string{"Content-Type": "application/json", "Authorization": "Basic " + basic}
		status, body := bsPaymentFetch(ctx, http.MethodPost, base+"/snap/v1/transactions", headers, string(payload))
		data := bsReadJSONObject(body)
		if status < 200 || status >= 300 {
			detail := bsAnyString(data["status_message"])
			if arr, ok := data["error_messages"].([]any); ok && len(arr) > 0 {
				detail = bsAnyString(arr[0])
			}
			if detail == "" {
				detail = "HTTP " + itoa(status)
			}
			return bsPaymentIntent{}, errors.New("Midtrans: " + detail)
		}
		return bsPaymentIntent{
			RawStatus: "pending", PaymentURL: data["redirect_url"], PaymentMethod: "snap",
		}, nil

	case "tripay":
		customerName := input.CustomerName
		if customerName == "" {
			customerName = "Customer Tauthub"
		}
		payload, _ := json.Marshal(map[string]any{
			"method": "QRIS", "merchant_ref": externalID, "amount": amount,
			"customer_name": customerName, "customer_email": input.PayerEmail,
			"order_items": []map[string]any{{"sku": "tauthub-payment", "name": description, "price": amount, "quantity": 1}},
		})
		headers := map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + credentials["api_key"]}
		status, body := bsPaymentFetch(ctx, http.MethodPost, base+"/transaction/create", headers, string(payload))
		data := bsReadJSONObject(body)
		item, _ := data["data"].(map[string]any)
		if status < 200 || status >= 300 || data["success"] == false {
			msg := bsAnyString(data["message"])
			if msg == "" {
				msg = "HTTP " + itoa(status)
			}
			return bsPaymentIntent{}, errors.New("Tripay: " + msg)
		}
		rawStatus := "UNPAID"
		var checkout, reference any
		if item != nil {
			if s := bsAnyString(item["status"]); s != "" {
				rawStatus = s
			}
			checkout = item["checkout_url"]
			reference = item["reference"]
		}
		return bsPaymentIntent{
			RawStatus: rawStatus, PaymentURL: checkout,
			PaymentMethod: "QRIS", ProviderReference: reference,
		}, nil

	case "sumopod":
		appBase := strings.TrimRight(d.Cfg.PublicAppURL, "/")
		if input.ReturnURL != "" {
			if base2 := strings.TrimRight(input.ReturnURL, "/"); base2 != "" {
				appBase = base2
			}
		}
		toHTTPS := func(v string) string {
			if strings.HasPrefix(strings.ToLower(v), "http://") {
				return "https://" + v[len("http://"):]
			}
			return v
		}
		brandSegment := ""
		if input.BrandID != "" {
			brandSegment = "/" + urlPathEscape(input.BrandID)
		}
		payloadBody := map[string]any{
			"order_id": externalID, "amount": amount, "currency": currency, "expires_in_hours": 24,
			"payment_method_type_code": bsFirstNonEmpty(strings.TrimSpace(credentials["payment_method_code"]), "QRIS"),
		}
		if appBase != "" {
			payloadBody["success_return_url"] = toHTTPS(appBase + "/payments/success" + brandSegment)
			payloadBody["cancel_return_url"] = toHTTPS(appBase + "/payments/cancelled" + brandSegment)
		}
		payload, _ := json.Marshal(payloadBody)
		headers := map[string]string{"Content-Type": "application/json", "X-Api-Key": credentials["api_key"]}
		status, raw := bsPaymentFetch(ctx, http.MethodPost, base+"/api/v1/payments", headers, string(payload))
		data := bsReadJSONObject(raw)
		if status < 200 || status >= 300 {
			msg := bsAnyString(data["message"])
			if msg == "" {
				msg = bsAnyString(data["error"])
			}
			if msg == "" {
				msg = "HTTP " + itoa(status)
			}
			return bsPaymentIntent{}, errors.New("SumoPod: " + msg)
		}
		rawStatus := bsAnyString(data["status"])
		if rawStatus == "" {
			rawStatus = "pending"
		}
		return bsPaymentIntent{
			RawStatus: rawStatus, PaymentURL: data["payment_link_url"],
			PaymentMethod: "QRIS", ProviderReference: data["payment_id"],
		}, nil

	case "sandbox":
		return bsPaymentIntent{
			RawStatus: "UNPAID", PaymentURL: "/payments/sandbox/" + externalID,
			PaymentMethod: "sandbox", ProviderReference: externalID,
		}, nil
	}
	return bsPaymentIntent{}, errors.New("Unknown payment provider")
}

// bsAnyString: nilai apa pun → string (parity String(v) untuk pesan provider).
func bsAnyString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == math.Trunc(t) {
			return itoa(int(t))
		}
		b, _ := json.Marshal(t)
		return string(b)
	case nil:
		return ""
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// bsFirstNonEmpty: nilai pertama non-kosong.
func bsFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
