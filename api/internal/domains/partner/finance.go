package partner

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- GET /v1/partner/earnings ----------

func (d *Deps) Earnings(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "pending" && status != "settled" && status != "voided" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid earnings status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	clause := "WHERE e.partner_id = $1"
	baseArgs := []any{partner.ID}
	if status != "" {
		clause += " AND e.status = $2"
		baseArgs = append(baseArgs, status)
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM partner_earnings e "+clause, baseArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	pageArgs := append(append([]any{}, baseArgs...), limit, offset)
	items, err := d.queryRows(r.Context(), `
		SELECT e.id, e.publication_id, e.media_id, e.rule_id, e.amount, e.client_price, e.status, e.settlement_id, e.created_at,
		       m.name AS media_name, m.website_url AS media_website_url, d.title AS document_title, r.name AS rule_name,
		       p.partner_price AS partner_price,
		       r.partner_percent AS rule_partner_percent
		FROM partner_earnings e
		JOIN media m ON m.id = e.media_id
		JOIN publications p ON p.id = e.publication_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN revenue_shares r ON r.id = e.rule_id
		`+clause+`
		ORDER BY e.created_at DESC LIMIT $`+itoa(len(baseArgs)+1)+` OFFSET $`+itoa(len(baseArgs)+2), pageArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	summary := d.partnerFinanceSummary(r.Context(), partner.ID)
	tcCfg := d.getTcConfig(r)

	earnings := make([]map[string]any, 0, len(items))
	for _, row := range items {
		clientPrice := row.numOr("client_price")
		partnerPrice := clientPrice
		if v, ok := row.num("partner_price"); ok {
			partnerPrice = v
		}
		rulePercent := 25.0
		if v, ok := row.num("rule_partner_percent"); ok {
			rulePercent = v
		}
		earnings = append(earnings, map[string]any{
			"id":                   row.raw("id"),
			"publication_id":       row.raw("publication_id"),
			"media_id":             row.raw("media_id"),
			"media_name":           row.raw("media_name"),
			"media_website_url":    row.raw("media_website_url"),
			"document_title":       row.raw("document_title"),
			"rule_id":              row.raw("rule_id"),
			"rule_name":            row.raw("rule_name"),
			"amount":               row.numOr("amount"),
			"client_price":         clientPrice,
			"partner_price":        partnerPrice,
			"rule_partner_percent": rulePercent,
			"status":               row.raw("status"),
			"settlement_id":        row.raw("settlement_id"),
			"created_at":           row.timePtr("created_at"),
		})
	}
	w.Header().Set("Cache-Control", "private, max-age=5, stale-while-revalidate=30")
	writeJSON(w, http.StatusOK, map[string]any{
		"minimum_withdraw_idr": tcCfg.MinimumWithdrawIDR,
		"summary": map[string]any{
			"pending":          summary.PendingEarnings,
			"settled":          summary.SettledEarnings,
			"paid_settlements": summary.PaidSettlements,
		},
		"earnings": earnings,
		"total":    countRow.intOr("count", 0),
		"limit":    limit,
		"offset":   offset,
	})
}

// ---------- GET/POST /v1/partner/settlements ----------

func settlementJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":                row.raw("id"),
		"period_start":      row.timePtr("period_start"),
		"period_end":        row.timePtr("period_end"),
		"status":            row.raw("status"),
		"total_amount":      row.numOr("total_amount"),
		"payout_method":     row.strOr("payout_method", "bank"),
		"transfer_fee_idr":  row.numOr("transfer_fee_idr"),
		"net_amount_idr":    row.numOr("net_amount_idr"),
		"invoice_number":    row.raw("invoice_number"),
		"notes":             row.raw("notes"),
		"receipt_url":       row.raw("receipt_url"),
		"payment_reference": row.raw("payment_reference"),
		"rejection_reason":  row.raw("rejection_reason"),
		"paid_at":           row.timePtr("paid_at"),
		"created_at":        row.timePtr("created_at"),
		"updated_at":        row.timePtr("updated_at"),
	}
}

func (d *Deps) Settlements(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		d.createSettlement(w, r, partner)
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "pending" && status != "approved" && status != "processing" && status != "paid" && status != "failed" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid settlement status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	clause := "WHERE s.partner_id = $1"
	baseArgs := []any{partner.ID}
	if status != "" {
		clause += " AND s.status = $2"
		baseArgs = append(baseArgs, status)
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM settlements s "+clause, baseArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	pageArgs := append(append([]any{}, baseArgs...), limit, offset)
	items, err := d.queryRows(r.Context(),
		`SELECT s.id, s.period_start, s.period_end, s.status, s.total_amount, s.payout_method, s.transfer_fee_idr, s.net_amount_idr,
		        s.invoice_number, s.notes, s.receipt_url, s.payment_reference, s.rejection_reason, s.paid_at, s.created_at, s.updated_at
		 FROM settlements s `+clause+`
		 ORDER BY s.created_at DESC LIMIT $`+itoa(len(baseArgs)+1)+` OFFSET $`+itoa(len(baseArgs)+2), pageArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	settlements := make([]map[string]any, 0, len(items))
	for _, row := range items {
		settlements = append(settlements, settlementJSON(row))
	}
	tcCfg := d.getTcConfig(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"minimum_withdraw_idr":         tcCfg.MinimumWithdrawIDR,
		"withdraw_bank_fee_idr":        tcCfg.WithdrawBankFeeIDR,
		"withdraw_ewallet_fee_percent": tcCfg.WithdrawEwalletFeePct,
		"settlements":                  settlements,
		"total":                        countRow.intOr("count", 0),
		"limit":                        limit,
		"offset":                       offset,
	})
}

func (d *Deps) createSettlement(w http.ResponseWriter, r *http.Request, partner PartnerRow) {
	user := httpx.AuthUserFromCtx(r)
	body := decodeJSON(r)
	bankNotes, _ := strField(body, "notes")
	mode := "all"
	if v, present := strField(body, "mode"); present && v == "partial" {
		mode = "partial"
	}
	requestedAmount := round2(mustNum(body, "amount_idr"))

	if partner.BankAccountNumber == nil || *partner.BankAccountNumber == "" {
		httpx.Error(w, http.StatusBadRequest, "Lengkapi dulu nomor rekening atau e-wallet pencairan di Profil → Pengaturan Withdraw / Rekening.")
		return
	}
	payoutMethod := "bank"
	if partner.PayoutType != nil && *partner.PayoutType == "ewallet" {
		payoutMethod = "ewallet"
	}
	cfg := d.getTcConfig(r)

	availableEarnings, err := d.queryRows(r.Context(),
		"SELECT id, amount, created_at FROM partner_earnings WHERE partner_id = $1 AND status = 'settled' AND settlement_id IS NULL ORDER BY created_at ASC",
		partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	totalEarnings := 0.0
	for _, row := range availableEarnings {
		totalEarnings = round2(totalEarnings + row.numOr("amount"))
	}
	totalCharges := d.partnerServiceChargesTotal(r.Context(), partner.ID)
	netAvailable := round2(totalEarnings - totalCharges)
	if netAvailable < 0 {
		netAvailable = 0
	}

	gross := netAvailable
	if mode == "partial" {
		if !(requestedAmount > 0) {
			httpx.Error(w, http.StatusBadRequest, "Nominal penarikan wajib diisi untuk tarik sebagian.")
			return
		}
		if requestedAmount > netAvailable {
			httpx.Error(w, http.StatusBadRequest,
				"Nominal melebihi saldo yang tersedia ("+formatIDR(netAvailable)+").")
			return
		}
		if requestedAmount < cfg.MinimumWithdrawIDR {
			httpx.Error(w, http.StatusBadRequest,
				"Minimal saldo untuk penarikan dana adalah "+formatIDR(cfg.MinimumWithdrawIDR)+".")
			return
		}
		gross = requestedAmount
	} else if netAvailable < cfg.MinimumWithdrawIDR {
		httpx.Error(w, http.StatusBadRequest,
			"Minimal saldo untuk penarikan dana adalah "+formatIDR(cfg.MinimumWithdrawIDR)+".")
		return
	}

	fee := cfg.WithdrawBankFeeIDR
	if payoutMethod == "ewallet" {
		fee = round2(gross * cfg.WithdrawEwalletFeePct / 100)
	}
	net := round2(gross - fee)
	if net < 0 {
		net = 0
	}

	firstDate := time.Now()
	if len(availableEarnings) > 0 {
		if t, ok := availableEarnings[0].raw("created_at").(time.Time); ok {
			firstDate = t
		}
	}
	lastDate := time.Now()

	invCount, _ := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM settlements WHERE partner_id = $1 AND invoice_number IS NOT NULL", partner.ID)
	invoiceNumber := fmt.Sprintf("INV/%s/%s", partner.ID, padLeft(itoa(invCount.intOr("count", 0)+1), 3))

	id := uuid.NewString()
	if err := d.exec(r.Context(), `
		INSERT INTO settlements (id, partner_id, period_start, period_end, status, total_amount, payout_method, transfer_fee_idr, net_amount_idr, invoice_number, notes, created_by)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9, $10, $11)`,
		id, partner.ID, firstDate.Format("2006-01-02"), lastDate.Format("2006-01-02"), gross, payoutMethod, fee, net,
		invoiceNumber, ptrOrNil(bankNotes), actorID(user)); err != nil {
		d.writeErr(w, err)
		return
	}

	// Kunci earning dari yang TERBARU supaya earning tertua yang sudah
	// "membiayai" service charge tidak ikut terkunci.
	sorted := append([]rowMap{}, availableEarnings...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			tj, _ := sorted[j].raw("created_at").(time.Time)
			tp, _ := sorted[j-1].raw("created_at").(time.Time)
			if tj.After(tp) {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			}
		}
	}
	earningIDs := []string{}
	acc := 0.0
	for _, row := range sorted {
		if acc >= gross {
			break
		}
		earningIDs = append(earningIDs, row.strOr("id", ""))
		acc = round2(acc + row.numOr("amount"))
	}
	if acc < gross {
		httpx.Error(w, http.StatusBadRequest, "Saldo tidak dapat ditutup dengan earning utuh — pilih nominal lain.")
		return
	}
	if len(earningIDs) > 0 {
		ph := make([]string, len(earningIDs))
		args := []any{id}
		for i, eid := range earningIDs {
			args = append(args, eid)
			ph[i] = "$" + itoa(i+2)
		}
		if err := d.exec(r.Context(),
			"UPDATE partner_earnings SET settlement_id = $1 WHERE id IN ("+strings.Join(ph, ",")+")", args...); err != nil {
			d.writeErr(w, err)
			return
		}
	}

	created, err := d.queryRow(r.Context(), "SELECT * FROM settlements WHERE id = $1 LIMIT 1", id)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	link := "/admin/settlements"
	d.notifyPlatformAdmins(r.Context(), "withdrawal.requested", "Penarikan dana baru",
		"Mitra mengajukan penarikan "+formatIDR(net)+" (net setelah biaya transfer) dan menunggu review Admin.", &link)
	writeJSON(w, http.StatusCreated, map[string]any{"settlement": settlementJSON(created)})
}

func actorID(user *httpx.AuthUser) any {
	if user == nil {
		return nil
	}
	return user.ID
}

func mustNum(body map[string]any, key string) float64 {
	v, _ := numField(body, key)
	return v
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}

// formatIDR: parity Intl.NumberFormat("id-ID", {style:"currency",currency:"IDR",maximumFractionDigits:0}).
func formatIDR(value float64) string {
	rounded := int64(math.Round(value))
	digits := itoa(int(abs64(rounded)))
	var parts []string
	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}
	parts = append([]string{digits}, parts...)
	sign := ""
	if rounded < 0 {
		sign = "-"
	}
	return sign + "Rp" + strings.Join(parts, ".")
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---------- GET /v1/partner/referrals ----------

func (d *Deps) Referrals(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	code := d.ensureReferralCode(r.Context(), partner.ID)
	base := strings.TrimRight(strings.TrimSpace(d.Cfg.PublicAppURL), "/")
	if base == "" {
		base = "https://" + publicHost(r)
	}
	summary := d.referralSummaryForPartner(r.Context(), partner.ID)
	earned, _ := d.queryRow(r.Context(),
		"SELECT COALESCE(SUM(amount),0) AS s FROM partner_earnings WHERE partner_id = $1 AND kind = 'referral' AND status != 'voided'",
		partner.ID)
	clicks, _ := d.queryRow(r.Context(),
		"SELECT COALESCE(SUM(click_count),0) AS total_clicks, COUNT(*)::int AS unique_fingerprints FROM referral_clicks WHERE referrer_partner_id = $1",
		partner.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"link":             base + "/register/mitra?ref=" + code,
		"link_mitra":       base + "/register/mitra?ref=" + code,
		"link_klien":       base + "/register/klien?ref=" + code,
		"code":             code,
		"rate_bps":         summary.RateBps,
		"totalReferrals":   summary.TotalReferrals,
		"referrals":        summary.Referrals,
		"commission":       summary.Commission,
		"referral_earning": earned.numOr("s"),
		"clicks": map[string]any{
			"total":  clicks.numOr("total_clicks"),
			"unique": clicks.intOr("unique_fingerprints", 0),
		},
	})
}

type referralSummary struct {
	RateBps        int
	TotalReferrals int
	Referrals      []map[string]any
	Commission     float64
}

// ensureReferralCode: port ensureReferralCode — kode 5 karakter a-z0-9.
func (d *Deps) ensureReferralCode(ctx context.Context, partnerID string) string {
	row, _ := d.queryRow(ctx, "SELECT referral_code FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if row != nil {
		if code, ok := row.str("referral_code"); ok && code != "" {
			return code
		}
	}
	existing, _ := d.queryRows(ctx, "SELECT referral_code FROM partners WHERE referral_code IS NOT NULL")
	taken := map[string]bool{}
	for _, r := range existing {
		taken[strings.ToLower(r.strOr("referral_code", ""))] = true
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	for attempt := 0; attempt < 200; attempt++ {
		candidate := randomCode(alphabet, 5)
		if taken[candidate] {
			continue
		}
		if err := d.exec(ctx,
			"UPDATE partners SET referral_code = $1, referral_code_normalized = $2 WHERE id = $3 AND referral_code IS NULL",
			candidate, candidate, partnerID); err != nil {
			continue
		}
		check, _ := d.queryRow(ctx, "SELECT referral_code FROM partners WHERE id = $1 LIMIT 1", partnerID)
		if check != nil {
			if code, ok := check.str("referral_code"); ok && code != "" {
				return code
			}
		}
	}
	return ""
}

func randomCode(alphabet string, n int) string {
	buf := secureRandomBytes(n)
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(out)
}

// referralSummaryForPartner: ringkasan referral mitra (komisi, jumlah rujukan).
func (d *Deps) referralSummaryForPartner(ctx context.Context, partnerID string) referralSummary {
	rateBps := 100
	if settings, _ := d.queryRow(ctx,
		"SELECT commission_rate_bp FROM affiliate_settings WHERE partner_id = $1 LIMIT 1", partnerID); settings != nil {
		if v, ok := settings.num("commission_rate_bp"); ok {
			rateBps = int(v)
		}
	}
	attributions, _ := d.queryRows(ctx, `
		SELECT a.id, a.code, a.referee_user_id, a.created_at, u.email AS referee_email
		FROM referral_attributions a
		LEFT JOIN users u ON u.id = a.referee_user_id
		WHERE a.referrer_partner_id = $1
		ORDER BY a.created_at DESC LIMIT 100`, partnerID)
	referrals := make([]map[string]any, 0, len(attributions))
	for _, a := range attributions {
		referrals = append(referrals, map[string]any{
			"id":              a.raw("id"),
			"code":            a.raw("code"),
			"referee_user_id": a.raw("referee_user_id"),
			"referee_email":   a.raw("referee_email"),
			"created_at":      a.timePtr("created_at"),
		})
	}
	commissionRow, _ := d.queryRow(ctx,
		"SELECT COALESCE(SUM(amount),0) AS s FROM partner_earnings WHERE partner_id = $1 AND kind = 'referral'",
		partnerID)
	return referralSummary{
		RateBps:        rateBps,
		TotalReferrals: len(referrals),
		Referrals:      referrals,
		Commission:     commissionRow.numOr("s"),
	}
}

// ---------- GET /v1/partner/vip/wallet ----------

func (d *Deps) VipWallet(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	// Satu query GROUP BY untuk semua jenis earning (sebelumnya 4 query).
	sums := map[string]float64{"cpa": 0, "transaction_commission": 0, "publication": 0, "referral": 0}
	rows, err := d.queryRows(r.Context(), `
		SELECT kind, COALESCE(SUM(amount),0) AS s FROM partner_earnings
		WHERE partner_id = $1 AND kind IN ('cpa','transaction_commission','publication','referral') AND status != 'voided'
		GROUP BY kind`, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	for _, row := range rows {
		sums[row.strOr("kind", "")] = row.numOr("s")
	}
	history, _ := d.queryRows(r.Context(), `
		SELECT id, kind, amount, client_price, status, created_at, source_referred_domain_id
		FROM partner_earnings
		WHERE partner_id = $1 AND kind IN ('cpa','transaction_commission','referral','publication')
		ORDER BY created_at DESC LIMIT 200`, partner.ID)
	items := make([]map[string]any, 0, len(history))
	for _, h := range history {
		items = append(items, map[string]any{
			"id":                        h.raw("id"),
			"kind":                      h.raw("kind"),
			"amount":                    h.numOr("amount"),
			"client_price":              h.numOr("client_price"),
			"status":                    h.raw("status"),
			"created_at":                h.timePtr("created_at"),
			"source_referred_domain_id": h.raw("source_referred_domain_id"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"cpa":                    sums["cpa"],
		"transaction_commission": sums["transaction_commission"],
		"publication":            sums["publication"],
		"referral":               sums["referral"],
		"history":                items,
	})
}

// ---------- GET /v1/partner/vip/domains ----------

func (d *Deps) VipDomains(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 50, 1, 200)
	items, err := d.queryRows(r.Context(), `
		SELECT rd.id, rd.domain_name, rd.status, rd.rejection_reason, rd.cpa_paid, rd.cpa_amount, rd.is_tld, rd.created_at,
		       u.email AS owner_email, m.name AS media_name
		FROM referred_domains rd
		LEFT JOIN users u ON u.id = rd.owner_user_id
		LEFT JOIN media m ON m.id = rd.media_id
		WHERE rd.referrer_partner_id = $1 ORDER BY rd.created_at DESC LIMIT $2`, partner.ID, limit)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	stats, _ := d.queryRow(r.Context(), `
		SELECT COUNT(*)::int AS total,
		       COUNT(*) FILTER (WHERE status = 'approved')::int AS approved,
		       COUNT(*) FILTER (WHERE status = 'pending')::int AS pending,
		       COUNT(*) FILTER (WHERE status = 'rejected')::int AS rejected
		FROM referred_domains WHERE referrer_partner_id = $1`, partner.ID)
	clicks, _ := d.queryRow(r.Context(),
		"SELECT COALESCE(SUM(click_count),0) AS c FROM referral_clicks WHERE referrer_partner_id = $1", partner.ID)

	out := make([]map[string]any, 0, len(items))
	for _, row := range items {
		out = append(out, map[string]any{
			"id":               row.raw("id"),
			"domain_name":      row.raw("domain_name"),
			"status":           row.raw("status"),
			"rejection_reason": row.raw("rejection_reason"),
			"cpa_paid":         row.boolOr("cpa_paid", false),
			"cpa_amount":       row.numPtr("cpa_amount"),
			"is_tld":           row.boolOr("is_tld", false),
			"created_at":       row.timePtr("created_at"),
			"owner_email":      row.raw("owner_email"),
			"media_name":       row.raw("media_name"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out,
		"stats": map[string]any{
			"total":    stats.intOr("total", 0),
			"approved": stats.intOr("approved", 0),
			"pending":  stats.intOr("pending", 0),
			"rejected": stats.intOr("rejected", 0),
		},
		"clicks": clicks.numOr("c"),
	})
}

// ---------- POST /v1/partner/vip/brand ----------

func (d *Deps) VipBrand(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	info := d.getPartnerVipInfo(r, partner.ID)
	// Client Workspace hanya untuk VVIP (brand_ambassador) — gerbang tier
	// yang sama dengan app lama; entitlement tidak dilibatkan.
	if !info.IsVip || (info.Tier != "brand_ambassador" && info.Tier != "vvip") {
		httpx.Error(w, http.StatusForbidden, "Hanya VIP Brand Ambassador yang memiliki akses workspace klien.")
		return
	}
	existing, _ := d.queryRow(r.Context(),
		"SELECT b.id, b.public_id FROM brands b JOIN brand_memberships bm ON bm.brand_id = b.id WHERE bm.user_id = $1 AND b.deleted_at IS NULL LIMIT 1",
		user.ID)
	if existing != nil {
		writeJSON(w, http.StatusOK, map[string]any{"id": existing.raw("id"), "public_id": existing.raw("public_id")})
		return
	}
	partnerRow, _ := d.queryRow(r.Context(), "SELECT name FROM partners WHERE id = $1 LIMIT 1", partner.ID)
	name := partnerRow.strOr("name", "Brand VIP")
	slug := vipBrandSlug(name)
	brandID := uuid.NewString()
	publicID := "vip-" + brandID[:8]

	if err := d.exec(r.Context(),
		"INSERT INTO brands (id, name, slug, public_id, status, created_at, updated_at) VALUES ($1, $2, $3, $4, 'active', NOW(), NOW())",
		brandID, name, slug, publicID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"INSERT INTO brand_memberships (id, brand_id, user_id, role, created_at) VALUES (gen_random_uuid(), $1, $2, 'owner', NOW())",
		brandID, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	created, _ := d.queryRow(r.Context(), "SELECT id, public_id FROM brands WHERE id = $1 LIMIT 1", brandID)
	writeJSON(w, http.StatusOK, map[string]any{"id": created.raw("id"), "public_id": created.raw("public_id")})
}

func vipBrandSlug(name string) string {
	lower := strings.ToLower(name)
	var b strings.Builder
	lastDash := false
	for _, ch := range lower {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 60 {
		out = out[:60]
	}
	out = strings.Trim(out, "-")
	if out == "" {
		return fmt.Sprintf("vip-brand-%d", time.Now().UnixMilli())
	}
	return out
}
