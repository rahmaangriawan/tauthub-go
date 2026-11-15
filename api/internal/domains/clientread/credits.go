package clientread

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/[brandId]/credits/route.ts + invoices/route.ts (read-only).

// ledgerCampaignExpr: SQL yang menormalkan PEMILIK KAMPANYE tiap entri ledger.
//   - reserve/capture/release memakai id publikasi sebagai ref_id → dipetakan
//     lewat join `publications p`.
//   - charge AI/Taut Pintar memakai prefiks ref_id: "taut-writer:{campaign}:…",
//     "taut-writer-regen:{campaign}:…", "ai-image:{campaign}:…", dan
//     "profiler-{campaign}-{hash}" (campaign = UUID 36 karakter setelah prefiks).
//   - entri lain (topup, koreksi admin, legacy tanpa ref_id) → NULL = tanpa
//     kampanye; ditampilkan sebagai grup terpisah di UI.
const ledgerCampaignExpr = `(CASE
		WHEN e.ref_id LIKE 'taut-writer:%' THEN split_part(e.ref_id, ':', 2)
		WHEN e.ref_id LIKE 'taut-writer-regen:%' THEN split_part(e.ref_id, ':', 2)
		WHEN e.ref_id LIKE 'taut-writer-derived:%' THEN split_part(e.ref_id, ':', 2)
		WHEN e.ref_id LIKE 'taut-writer-derived-regen:%' THEN split_part(e.ref_id, ':', 2)
		WHEN e.ref_id LIKE 'ai-image:%' THEN split_part(e.ref_id, ':', 2)
		WHEN e.ref_id LIKE 'profiler-%' THEN substring(e.ref_id from 10 for 36)
		WHEN p.id IS NOT NULL THEN p.campaign_id::text
		ELSE NULL
	END)`

// ledgerSpentExpr: biaya yang BENAR-BENAR terpakai per entri — konsisten
// dengan warna baris di UI (merah): capture (tagihan publikasi, tandanya bisa
// +/− tergantung alur) dan adjustment negatif (tulis artikel/gambar AI/regen).
// reserve (dana ditahan) dan release (pengembalian) TIDAK dihitung terpakai.
const ledgerSpentExpr = `(CASE
		WHEN e.ref_type = 'capture' THEN ABS(e.amount_tc)
		WHEN e.ref_type = 'adjustment' AND e.amount_tc < 0 THEN -e.amount_tc
		ELSE 0
	END)`

// Credits: GET /v1/brands/{brandId}/credits — wallet + ledger + transaksi.
// Param opsional:
//   - group_by=campaign → kembalikan `ledger_groups` (agregat per kampanye,
//     lengkap lintas halaman) alih-alih daftar entri mentah.
//   - campaign_id=<uuid|none> → filter daftar entri ke satu kampanye (untuk
//     tampilan rincian saat grup diklik).
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
	campaignFilter := strings.TrimSpace(q.Get("campaign_id"))
	groupByCampaign := strings.TrimSpace(q.Get("group_by")) == "campaign"

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
	// Filter campaign_id: "none" = entri tanpa kampanye (topup/koreksi admin).
	campaignWhere := ""
	countCampaignWhere := ""
	countArgs := []any{brandID}
	ledgerArgs := []any{brandID, limit, offset}
	if campaignFilter == "none" {
		campaignWhere = " AND " + ledgerCampaignExpr + " IS NULL"
		countCampaignWhere = campaignWhere
	} else if campaignFilter != "" {
		campaignWhere = fmt.Sprintf(" AND %s = $4", ledgerCampaignExpr)
		countCampaignWhere = fmt.Sprintf(" AND %s = $2", ledgerCampaignExpr)
		countArgs = append(countArgs, campaignFilter)
		ledgerArgs = append(ledgerArgs, campaignFilter)
	}
	var ledgerTotal int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM credit_ledger_entries e LEFT JOIN publications p ON p.id = e.ref_id WHERE e.brand_id = $1"+countCampaignWhere,
		countArgs...).Scan(&ledgerTotal)
	ledger := []map[string]any{}
	lRows, lErr := d.Pool.Query(r.Context(), `
		SELECT e.id, e.ref_type, e.ref_id, e.amount_tc, e.balance_after_tc, e.rate_idr, e.amount_idr, e.reason,
		       COALESCE(d.title, td.title), COALESCE(m.name, tm.name), p.status, e.created_at
		FROM credit_ledger_entries e
		LEFT JOIN publications p ON p.id = e.ref_id
		LEFT JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN media m ON m.id = p.media_id
		-- Entri penulisan/gambar AI (ref_id "taut-writer:{kampanye}:{media}", dst.)
		-- belum punya publikasi, jadi judulnya diambil dari artikel kampanye itu
		-- supaya klien tahu biaya ini untuk artikel yang mana.
		LEFT JOIN LATERAL (
			SELECT cd.title
			FROM taut_campaign_media tcm
			JOIN content_documents cd ON cd.id = tcm.article_document_id
			WHERE e.ref_id ~ '^(taut-writer|taut-writer-regen|taut-writer-derived|taut-writer-derived-regen|ai-image):'
			  AND tcm.campaign_id::text = split_part(e.ref_id, ':', 2)
			  AND tcm.media_id::text = split_part(e.ref_id, ':', 3)
			ORDER BY (tcm.status = 'selected') DESC, tcm.created_at DESC
			LIMIT 1
		) td ON TRUE
		LEFT JOIN media tm ON e.ref_id ~ '^(taut-writer|taut-writer-regen|ai-image):'
		                  AND tm.id::text = split_part(e.ref_id, ':', 3)
		WHERE e.brand_id = $1`+campaignWhere+`
		ORDER BY e.created_at DESC LIMIT $2 OFFSET $3`, ledgerArgs...)
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

	// Agregat per kampanye (group_by=campaign): dasar tampilan "rincian
	// penggunaan saldo per kampanye". Angka terpakai memakai ledgerSpentExpr —
	// semantik yang sama dengan warna merah di tabel entri.
	ledgerGroups := []map[string]any{}
	ledgerGroupsTotal := 0
	if groupByCampaign {
		_ = d.Pool.QueryRow(r.Context(),
			"SELECT COUNT(*) FROM (SELECT 1 FROM credit_ledger_entries e LEFT JOIN publications p ON p.id = e.ref_id WHERE e.brand_id = $1 GROUP BY "+ledgerCampaignExpr+") t",
			brandID).Scan(&ledgerGroupsTotal)
		gRows, gErr := d.Pool.Query(r.Context(), `
			SELECT COALESCE(sub.campaign_id, '') AS campaign_id,
			       COALESCE(c.name, '') AS campaign_name,
			       COUNT(*) AS entries,
			       COALESCE(SUM(sub.spent_tc), 0) AS spent_tc,
			       COALESCE(SUM(sub.net_tc), 0) AS net_tc,
			       COALESCE(SUM(sub.held_tc), 0) AS held_tc,
			       MAX(sub.created_at) AS last_at
			FROM (
				SELECT `+ledgerCampaignExpr+` AS campaign_id,
				       `+ledgerSpentExpr+` AS spent_tc,
				       e.amount_tc AS net_tc,
				       CASE WHEN e.ref_type = 'reserve' THEN -e.amount_tc
				            WHEN e.ref_type = 'release' THEN -e.amount_tc
				            WHEN e.ref_type = 'capture' AND e.amount_tc > 0 THEN -e.amount_tc
				            ELSE 0 END AS held_tc,
				       e.created_at
				FROM credit_ledger_entries e
				LEFT JOIN publications p ON p.id = e.ref_id
				WHERE e.brand_id = $1
			) sub
			LEFT JOIN campaigns c ON c.id::text = sub.campaign_id
			GROUP BY sub.campaign_id, c.name
			ORDER BY MAX(sub.created_at) DESC
			LIMIT $2 OFFSET $3`, brandID, limit, offset)
		if gErr == nil {
			defer gRows.Close()
			for gRows.Next() {
				var campaignID, campaignName string
				var entries, spentTC, netTC, heldTC int
				var lastAt time.Time
				if gRows.Scan(&campaignID, &campaignName, &entries, &spentTC, &netTC, &heldTC, &lastAt) == nil {
					ledgerGroups = append(ledgerGroups, map[string]any{
						"campaign_id": campaignID, "campaign_name": campaignName,
						"entries": entries, "spent_tc": spentTC, "net_tc": netTC,
						"held_tc": heldTC, "last_at": isoMilli(lastAt),
					})
				}
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

	// Pending topup payments (payment_transactions). `amount` bertipe NUMERIC —
	// dulu di-scan ke *int sehingga SELALU gagal dan banner pending tak pernah
	// muncul; sekarang dibaca float64 → rupiah dibulatkan, `purpose_ref_id`
	// (character) berisi jumlah TC. Semua baris pending dikembalikan (bisa lebih
	// dari satu bila klien membuka beberapa tagihan lalu ditinggalkan) supaya UI
	// bisa menghitung totalnya.
	//
	// MASA BERLAKU SESI TOPUP = 24 JAM: tagihan yang lewat masa berlaku tidak
	// lagi dihitung/ditampilkan (notif hilang). Baris lama kadang tidak punya
	// `expired_at` (dibuat sebelum kolom itu diisi), jadi jatuh tempo dihitung
	// dari created_at + 24 jam — sama dengan yang ditulis saat pembuatan baru.
	pendingPayments := []map[string]any{}
	pRows, pErr := d.Pool.Query(r.Context(), `
		SELECT id, COALESCE(payment_url, ''), amount, COALESCE(purpose_ref_id, ''), status, created_at
		FROM payment_transactions
		WHERE brand_id = $1 AND purpose = 'topup' AND status IN ('pending','processing')
		  AND COALESCE(expired_at, created_at + interval '24 hours') > NOW()
		ORDER BY created_at DESC`, brandID)
	if pErr == nil {
		defer pRows.Close()
		for pRows.Next() {
			var id, url, ref, status string
			var amountF float64
			var createdAt time.Time
			if pRows.Scan(&id, &url, &amountF, &ref, &status, &createdAt) != nil {
				continue
			}
			tc, tcErr := strconv.Atoi(strings.TrimSpace(ref))
			var tcVal any
			if tcErr == nil && tc > 0 {
				tcVal = tc
			}
			pendingPayments = append(pendingPayments, map[string]any{
				"id": id, "payment_url": url, "amount_idr": int(math.Round(amountF)),
				"tc": tcVal, "status": status, "created_at": isoMilli(createdAt),
			})
		}
	}
	// `pending_payment` (tunggal) = tagihan terbaru — dipakai alur lama/modal.
	var pending any
	if len(pendingPayments) > 0 {
		pending = pendingPayments[0]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pending_payment":  pending,
		"pending_payments": pendingPayments,
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
		"ledger_groups": ledgerGroups, "ledger_groups_total": ledgerGroupsTotal,
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
		       i.notes, i.created_at, i.updated_at, c.name, i.po_number, i.bill_to
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
		var campaignName, poNumber *string
		var billTo map[string]any
		if rowsQ.Scan(&id, &campaignID, &number, &invStatus, &amount, &issuedAt, &dueAt, &paidAt,
			&notes, &createdAt, &updatedAt, &campaignName, &poNumber, &billTo) == nil {
			invoices = append(invoices, map[string]any{
				"id": id, "campaign_id": campaignID, "campaign_name": campaignName,
				"number": number, "status": invStatus, "amount": amount,
				"issued_at": tsOrNil(issuedAt), "due_at": tsOrNil(dueAt), "paid_at": tsOrNil(paidAt),
				"notes": notes, "created_at": isoMilli(createdAt), "updated_at": isoMilli(updatedAt),
				"po_number": poNumber, "bill_to": billTo,
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
