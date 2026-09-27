package clientread

import (
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/[brandId]/credits/route.ts + invoices/route.ts (read-only).

// Credits: GET /v1/brands/{brandId}/credits — wallet + ledger + transaksi.
func (d *Deps) Credits(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	brandID := brandRow.ID
	q := r.URL.Query()
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	// Wallet (credit_wallets + tc_config.rate_idr) — parity walletSummary.
	available, reserved, captured := 0, 0, 0
	hasWallet := true
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1 LIMIT 1", brandID).
		Scan(&available, &reserved, &captured); err != nil {
		hasWallet = false
	}
	rateIDR := 0
	_ = d.Pool.QueryRow(r.Context(), "SELECT rate_idr FROM tc_config LIMIT 1").Scan(&rateIDR)
	var topupPackages []byte
	_ = d.Pool.QueryRow(r.Context(), "SELECT topup_packages FROM tc_config LIMIT 1").Scan(&topupPackages)

	// Ledger (credit_ledger_entries + join dokumen/media/publikasi).
	var ledgerTotal int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM credit_ledger_entries WHERE brand_id = $1", brandID).Scan(&ledgerTotal)
	ledger := []map[string]any{}
	lRows, lErr := d.Pool.Query(r.Context(), `
		SELECT e.id, e.ref_type, e.ref_id, e.amount_tc, e.balance_after_tc, e.rate_idr, e.amount_idr, e.reason,
		       d.title, m.name, p.status, e.created_at
		FROM credit_ledger_entries e
		LEFT JOIN publications p ON p.id = e.ref_id
		LEFT JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN media m ON m.id = p.media_id
		WHERE e.brand_id = $1
		ORDER BY e.created_at DESC LIMIT $2 OFFSET $3`, brandID, limit, offset)
	if lErr == nil {
		defer lRows.Close()
		for lRows.Next() {
			var id, refType string
			var refID *string
			var amountTC, balanceAfterTC, rate int
			var amountIDR *int
			var reason *string
			var documentTitle, mediaName, pubStatus *string
			var createdAt time.Time
			if lRows.Scan(&id, &refType, &refID, &amountTC, &balanceAfterTC, &rate, &amountIDR, &reason,
				&documentTitle, &mediaName, &pubStatus, &createdAt) == nil {
				ledger = append(ledger, map[string]any{
					"id": id, "ref_type": refType, "ref_id": refID, "amount_tc": amountTC,
					"balance_after_tc": balanceAfterTC, "rate_idr": rate, "amount_idr": amountIDR,
					"reason": reason, "document_title": documentTitle, "media_name": mediaName,
					"publication_status": pubStatus, "created_at": isoMilli(createdAt),
				})
			}
		}
	}

	// Transaksi kredit (credit_transactions) + saldo paket.
	var txTotal int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM credit_transactions WHERE brand_id = $1", brandID).Scan(&txTotal)
	transactions := []map[string]any{}
	tRows, tErr := d.Pool.Query(r.Context(), `
		SELECT t.id, t.package_id, t.publication_id, t.amount, t.reason, t.balance_after, t.created_at,
		       pkg.name, pub.document_id, d.title, m.name
		FROM credit_transactions t
		LEFT JOIN packages pkg ON pkg.id = t.package_id
		LEFT JOIN publications pub ON pub.id = t.publication_id
		LEFT JOIN content_documents d ON d.id = pub.document_id
		LEFT JOIN media m ON m.id = pub.media_id
		WHERE t.brand_id = $1
		ORDER BY t.created_at DESC LIMIT $2 OFFSET $3`, brandID, limit, offset)
	if tErr == nil {
		defer tRows.Close()
		for tRows.Next() {
			var id string
			var packageID, publicationID *string
			var amount int
			var reason string
			var balanceAfter int
			var createdAt time.Time
			var packageName, documentID, documentTitle, mediaName *string
			if tRows.Scan(&id, &packageID, &publicationID, &amount, &reason, &balanceAfter, &createdAt,
				&packageName, &documentID, &documentTitle, &mediaName) == nil {
				transactions = append(transactions, map[string]any{
					"id": id, "package_id": packageID, "package_name": packageName,
					"publication_id": publicationID, "document_title": documentTitle, "media_name": mediaName,
					"amount": amount, "reason": reason, "balance_after": balanceAfter,
					"created_at": isoMilli(createdAt),
				})
			}
		}
	}

	// Saldo paket (port brandCreditBalance): total/used/remaining dari packages+credit_transactions.
	totalCredits, usedCredits := 0, 0
	_ = d.Pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(p.credits), 0) FROM credit_transactions t
		JOIN packages p ON p.id = t.package_id
		WHERE t.brand_id = $1 AND t.reason = 'package.grant'`, brandID).Scan(&totalCredits)
	_ = d.Pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(ABS(amount)), 0) FROM credit_transactions
		WHERE brand_id = $1 AND reason = 'publication.usage'`, brandID).Scan(&usedCredits)
	packageCount := 0
	_ = d.Pool.QueryRow(r.Context(), `
		SELECT COUNT(DISTINCT package_id) FROM credit_transactions
		WHERE brand_id = $1 AND package_id IS NOT NULL`, brandID).Scan(&packageCount)

	// Pending topup payment (payment_transactions).
	var pending any
	var pID, pURL, pStatus *string
	var pAmount int
	var pTC *int
	var pCreatedAt *time.Time
	pErr := d.Pool.QueryRow(r.Context(), `
		SELECT id, payment_url, amount, purpose_ref_id, status, created_at FROM payment_transactions
		WHERE brand_id = $1 AND purpose = 'topup' AND status IN ('pending','processing')
		ORDER BY created_at DESC LIMIT 1`, brandID).
		Scan(&pID, &pURL, &pAmount, &pTC, &pStatus, &pCreatedAt)
	if pErr == nil && pID != nil {
		pending = map[string]any{
			"id": *pID, "payment_url": pURL, "amount_idr": pAmount,
			"tc": pTC, "status": deref(pStatus), "created_at": tsOrNil(pCreatedAt),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pending_payment": pending,
		"balance": map[string]any{
			"total": totalCredits, "used": usedCredits,
			"remaining": totalCredits - usedCredits, "package_count": packageCount,
		},
		"wallet": map[string]any{
			"available_tc": available, "reserved_tc": reserved, "captured_tc": captured,
			"total_tc": available + reserved + captured, "rate_idr": rateIDR,
			"available_idr": available * rateIDR, "has_wallet": hasWallet,
		},
		"tc_config": map[string]any{
			"rate_idr": rateIDR, "topup_packages": jsonRaw(topupPackages, []any{}),
		},
		"ledger": ledger, "ledger_total": ledgerTotal,
		"transactions": transactions, "total": txTotal,
		"limit": limit, "offset": offset,
	})
}

// Invoices: GET /v1/brands/{brandId}/invoices — filter status valid.
func (d *Deps) Invoices(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	if status != "" && status != "draft" && status != "issued" && status != "paid" && status != "void" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid invoice status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	clause := "WHERE i.brand_id = $1"
	args := []any{brandRow.ID}
	if status != "" {
		clause += " AND i.status = $2"
		args = append(args, status)
	}
	var total int
	_ = d.Pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM invoices i "+clause, args...).Scan(&total)

	args = append(args, limit, offset)
	rowsQ, err := d.Pool.Query(r.Context(), `
		SELECT i.id, i.campaign_id, i.number, i.status, i.amount, i.issued_at, i.due_at, i.paid_at,
		       i.notes, i.created_at, i.updated_at, c.name
		FROM invoices i LEFT JOIN campaigns c ON c.id = i.campaign_id
		`+clause+`
		ORDER BY i.created_at DESC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()
	invoices := []map[string]any{}
	for rowsQ.Next() {
		var id, number, invStatus string
		var campaignID *string
		var amount float64
		var issuedAt, dueAt, paidAt *time.Time
		var notes *string
		var createdAt, updatedAt time.Time
		var campaignName *string
		if rowsQ.Scan(&id, &campaignID, &number, &invStatus, &amount, &issuedAt, &dueAt, &paidAt,
			&notes, &createdAt, &updatedAt, &campaignName) == nil {
			invoices = append(invoices, map[string]any{
				"id": id, "campaign_id": campaignID, "campaign_name": campaignName,
				"number": number, "status": invStatus, "amount": amount,
				"issued_at": tsOrNil(issuedAt), "due_at": tsOrNil(dueAt), "paid_at": tsOrNil(paidAt),
				"notes": notes, "created_at": isoMilli(createdAt), "updated_at": isoMilli(updatedAt),
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"invoices": invoices, "total": total, "limit": limit, "offset": offset,
	})
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	out := []byte{}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
