package platform

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

// Laporan keuangan: uang masuk (topup klien yang lunas) vs uang keluar
// (pencairan mitra yang sudah ditransfer). Sumbernya transaksi pembayaran dan
// settlement — bukan tabel `transactions`, karena baris payout di sana tidak
// menyimpan settlement_id sehingga tidak bisa ditelusuri balik.

// financeReportTZ: batas hari laporan memakai WIB. Kalau memakai UTC, transaksi
// pukul 00:00–07:00 WIB akan terhitung di tanggal sebelumnya.
var financeReportTZ = time.FixedZone("WIB", 7*3600)

// financeNetExpr / financeFeeExpr: nominal yang benar-benar ditransfer ke mitra.
// Baris baru menyimpan net_amount_idr; baris lama (net 0) dihitung dengan aturan
// yang sama seperti settPayout — bank biaya flat, e-wallet persen dari bruto.
const financeNetExpr = `CASE
	WHEN s.net_amount_idr > 0 THEN s.net_amount_idr::numeric
	WHEN s.payout_method = 'ewallet' THEN GREATEST(s.total_amount - ROUND(s.total_amount * ($3::numeric / 100.0), 2), 0)
	ELSE GREATEST(s.total_amount - $4::numeric, 0)
END`

const financeFeeExpr = `CASE
	WHEN s.net_amount_idr > 0 THEN s.total_amount - s.net_amount_idr::numeric
	WHEN s.payout_method = 'ewallet' THEN ROUND(s.total_amount * ($3::numeric / 100.0), 2)
	ELSE $4::numeric
END`

// financeEntriesCTE: dua arah arus kas dengan bentuk kolom seragam supaya bisa
// diurutkan bersama, dipaginasi, dan diekspor sebagai satu tabel.
const financeEntriesCTE = `
WITH combined AS (
	SELECT 'in'::text AS direction,
	       t.id::text AS id,
	       COALESCE(t.paid_at, t.created_at) AS occurred_at,
	       'topup'::text AS kind,
	       COALESCE(NULLIF(b.name, ''), b.client_name, '')::text AS counterparty,
	       COALESCE(NULLIF(t.external_transaction_id, ''), NULLIF(t.reference, ''), '')::text AS reference,
	       COALESCE(t.gateway_provider, '')::text AS method,
	       t.amount::numeric AS gross_amount,
	       0::numeric AS fee_amount,
	       t.amount::numeric AS amount,
	       t.status::text AS status
	FROM payment_transactions t
	LEFT JOIN brands b ON b.id = t.brand_id
	WHERE t.purpose = 'topup' AND t.status = 'paid'
	  AND COALESCE(t.paid_at, t.created_at) >= $1
	  AND COALESCE(t.paid_at, t.created_at) < $2
	UNION ALL
	SELECT 'out'::text,
	       s.id::text,
	       COALESCE(s.paid_at, s.updated_at),
	       'settlement'::text,
	       COALESCE(p.name, '')::text,
	       COALESCE(NULLIF(s.invoice_number, ''), NULLIF(s.payment_reference, ''), '')::text,
	       COALESCE(s.payout_method, '')::text,
	       s.total_amount::numeric,
	       ` + financeFeeExpr + `,
	       ` + financeNetExpr + `,
	       s.status::text
	FROM settlements s
	LEFT JOIN partners p ON p.id = s.partner_id
	WHERE s.status = 'paid'
	  AND COALESCE(s.paid_at, s.updated_at) >= $1
	  AND COALESCE(s.paid_at, s.updated_at) < $2
)
`

var financeMonthShort = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func finReportInt(v float64) int { return int(math.Round(v)) }

// FinanceReport: GET /v1/platform/finance/report
func (d *Deps) FinanceReport(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	ctx := r.Context()
	now := time.Now().In(financeReportTZ)
	q := r.URL.Query()

	normDate := func(value string, fallback time.Time) time.Time {
		value = strings.TrimSpace(value)
		if len(value) < 10 {
			return fallback
		}
		parsed, err := time.ParseInLocation("2006-01-02", value[:10], financeReportTZ)
		if err != nil {
			return fallback
		}
		return parsed
	}
	fromDate := normDate(q.Get("from"), time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, financeReportTZ))
	toDate := normDate(q.Get("to"), time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, financeReportTZ))
	if fromDate.After(toDate) {
		writeErr(w, d, errUnprocessable("Rentang tanggal tidak valid"))
		return
	}
	fromTs := fromDate
	toExcl := toDate.AddDate(0, 0, 1)

	direction := strings.TrimSpace(q.Get("type"))
	switch direction {
	case "", "all":
		direction = ""
	case "in", "out":
	default:
		writeErr(w, d, errUnprocessable("Invalid report type filter"))
		return
	}

	limit := clampInt(q.Get("limit"), 20, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	if q.Get("format") == "csv" {
		limit = 0
		offset = 0
	}

	feeCfg := d.finFeeCfg(ctx)

	// ---------- ringkasan rentang ----------
	inRow, err := d.queryRow(ctx, `
		SELECT COUNT(*)::int AS n, COALESCE(SUM(amount), 0) AS total
		FROM payment_transactions
		WHERE purpose = 'topup' AND status = 'paid'
		  AND COALESCE(paid_at, created_at) >= $1
		  AND COALESCE(paid_at, created_at) < $2`, fromTs, toExcl)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	outRow, err := d.queryRow(ctx, `
		SELECT COUNT(*)::int AS n,
		       COALESCE(SUM(total_amount), 0) AS gross,
		       COALESCE(SUM(`+financeFeeExpr+`), 0) AS fee,
		       COALESCE(SUM(`+financeNetExpr+`), 0) AS net
		FROM settlements s
		WHERE s.status = 'paid'
		  AND COALESCE(s.paid_at, s.updated_at) >= $1
		  AND COALESCE(s.paid_at, s.updated_at) < $2`, fromTs, toExcl, feeCfg.ewalletPct, feeCfg.bankIDR)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	moneyIn := inRow.numOr("total")
	moneyOut := outRow.numOr("net")

	// ---------- posisi berjalan (tidak dibatasi rentang) ----------
	topupPending, _ := d.queryRow(ctx, `
		SELECT COUNT(*)::int AS n, COALESCE(SUM(amount), 0) AS total
		FROM payment_transactions
		WHERE purpose = 'topup' AND status IN ('pending', 'processing')`)
	settlementPending, _ := d.queryRow(ctx, `
		SELECT COUNT(*)::int AS n,
		       COALESCE(SUM(CASE
		         WHEN s.net_amount_idr > 0 THEN s.net_amount_idr::numeric
		         WHEN s.payout_method = 'ewallet' THEN GREATEST(s.total_amount - ROUND(s.total_amount * ($1::numeric / 100.0), 2), 0)
		         ELSE GREATEST(s.total_amount - $2::numeric, 0)
		       END), 0) AS total
		FROM settlements s
		WHERE s.status IN ('pending', 'approved', 'processing')`, feeCfg.ewalletPct, feeCfg.bankIDR)
	// Liabilitas klien = saldo yang belum terpakai. captured tidak dihitung:
	// uang itu sudah menjadi biaya publikasi/AI (pendapatan), bukan kewajiban.
	clientBalance, _ := d.queryRow(ctx, `
		SELECT COALESCE(SUM(available_tc + reserved_tc), 0) AS total FROM credit_wallets`)
	partnerUnpaid, _ := d.queryRow(ctx, `
		SELECT COALESCE(SUM(e.amount), 0) AS total
		FROM partner_earnings e
		LEFT JOIN settlements s ON s.id = e.settlement_id
		WHERE e.status = 'pending'
		   OR (e.status = 'settled' AND (s.id IS NULL OR s.status <> 'paid'))`)

	// ---------- informasi non-kas ----------
	invoiceRow, _ := d.queryRow(ctx, `
		SELECT COUNT(*)::int AS n, COALESCE(SUM(amount), 0) AS total
		FROM invoices
		WHERE status = 'paid'
		  AND COALESCE(paid_at, updated_at) >= $1
		  AND COALESCE(paid_at, updated_at) < $2`, fromTs, toExcl)
	compensationRow, _ := d.queryRow(ctx, `
		SELECT COUNT(*)::int AS n,
		       COALESCE(SUM(COALESCE(NULLIF(amount_idr, 0), claimed_tc)), 0) AS total
		FROM credit_topup_reports
		WHERE status = 'approved'
		  AND reviewed_at >= $1 AND reviewed_at < $2`, fromTs, toExcl)

	summary := map[string]any{
		"money_in":                 finReportInt(moneyIn),
		"money_in_count":           inRow.intOr("n", 0),
		"money_out":                finReportInt(moneyOut),
		"money_out_count":          outRow.intOr("n", 0),
		"money_out_gross":          finReportInt(outRow.numOr("gross")),
		"money_out_fee":            finReportInt(outRow.numOr("fee")),
		"net":                      finReportInt(moneyIn - moneyOut),
		"topup_pending":            finReportInt(topupPending.numOr("total")),
		"topup_pending_count":      topupPending.intOr("n", 0),
		"settlement_pending":       finReportInt(settlementPending.numOr("total")),
		"settlement_pending_count": settlementPending.intOr("n", 0),
		"client_balance":           finReportInt(clientBalance.numOr("total")),
		"partner_unpaid":           finReportInt(partnerUnpaid.numOr("total")),
		"invoice_paid":             finReportInt(invoiceRow.numOr("total")),
		"invoice_paid_count":       invoiceRow.intOr("n", 0),
		"compensation_credits":     finReportInt(compensationRow.numOr("total")),
		"compensation_count":       compensationRow.intOr("n", 0),
	}

	// ---------- tren 12 bulan ----------
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, financeReportTZ).AddDate(0, -11, 0)
	monthEnd := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, financeReportTZ).AddDate(0, 1, 0)
	inMonths, err := d.queryRows(ctx, `
		SELECT to_char(COALESCE(paid_at, created_at) AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM') AS month,
		       COALESCE(SUM(amount), 0) AS total
		FROM payment_transactions
		WHERE purpose = 'topup' AND status = 'paid'
		  AND COALESCE(paid_at, created_at) >= $1
		  AND COALESCE(paid_at, created_at) < $2
		GROUP BY 1`, monthStart, monthEnd)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	outMonths, err := d.queryRows(ctx, `
		SELECT to_char(COALESCE(s.paid_at, s.updated_at) AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM') AS month,
		       COALESCE(SUM(`+financeNetExpr+`), 0) AS total
		FROM settlements s
		WHERE s.status = 'paid'
		  AND COALESCE(s.paid_at, s.updated_at) >= $1
		  AND COALESCE(s.paid_at, s.updated_at) < $2
		GROUP BY 1`, monthStart, monthEnd, feeCfg.ewalletPct, feeCfg.bankIDR)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	inByMonth := make(map[string]float64, len(inMonths))
	for _, row := range inMonths {
		inByMonth[row.strOr("month", "")] = row.numOr("total")
	}
	outByMonth := make(map[string]float64, len(outMonths))
	for _, row := range outMonths {
		outByMonth[row.strOr("month", "")] = row.numOr("total")
	}
	monthly := make([]map[string]any, 0, 12)
	for i := 0; i < 12; i++ {
		m := monthStart.AddDate(0, i, 0)
		key := m.Format("2006-01")
		monthly = append(monthly, map[string]any{
			"month":     key,
			"label":     fmt.Sprintf("%s %02d", financeMonthShort[int(m.Month())-1], m.Year()%100),
			"money_in":  finReportInt(inByMonth[key]),
			"money_out": finReportInt(outByMonth[key]),
		})
	}

	// ---------- rincian arus kas ----------
	entriesSQL := financeEntriesCTE + `
		SELECT * FROM combined
		WHERE ($5::text = '' OR direction = $5::text)
		ORDER BY occurred_at DESC, id DESC`
	args := []any{fromTs, toExcl, feeCfg.ewalletPct, feeCfg.bankIDR, direction}
	if limit > 0 {
		entriesSQL += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	rows, err := d.queryRows(ctx, entriesSQL, args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	if q.Get("format") == "csv" {
		d.finReportCSV(w, rows, toDate)
		return
	}

	entries := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		var occurred any
		if t, ok := row.raw("occurred_at").(time.Time); ok {
			occurred = isoMilli(t)
		}
		entries = append(entries, map[string]any{
			"id":           row.strOr("id", ""),
			"direction":    row.strOr("direction", ""),
			"occurred_at":  occurred,
			"kind":         row.strOr("kind", ""),
			"counterparty": row.strOr("counterparty", ""),
			"reference":    row.strOr("reference", ""),
			"method":       row.strOr("method", ""),
			"gross_amount": finReportInt(row.numOr("gross_amount")),
			"fee_amount":   finReportInt(row.numOr("fee_amount")),
			"amount":       finReportInt(row.numOr("amount")),
			"status":       row.strOr("status", ""),
		})
	}

	countRow, err := d.queryRow(ctx, financeEntriesCTE+`
		SELECT COUNT(*)::int AS n FROM combined
		WHERE ($5::text = '' OR direction = $5::text)`,
		fromTs, toExcl, feeCfg.ewalletPct, feeCfg.bankIDR, direction)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":    fromDate.Format("2006-01-02"),
		"to":      toDate.Format("2006-01-02"),
		"summary": summary,
		"monthly": monthly,
		"entries": entries,
		"total":   countRow.intOr("n", 0),
		"limit":   limit,
		"offset":  offset,
	})
}

// finReportCSV: ekspor seluruh rentang (tanpa paginasi) sebagai CSV yang bisa
// langsung dibuka di Excel — BOM UTF-8 supaya huruf beraksen tidak rusak.
func (d *Deps) finReportCSV(w http.ResponseWriter, rows []rowMap, toDate time.Time) {
	var b strings.Builder
	b.WriteString("\ufeff")
	b.WriteString(opCSVLine([]any{"Tanggal", "Arus", "Keterangan", "Klien/Mitra", "Referensi",
		"Metode", "Bruto (IDR)", "Biaya (IDR)", "Nominal (IDR)", "Status"}))
	b.WriteString("\n")
	for _, row := range rows {
		occurred := ""
		if t, ok := row.raw("occurred_at").(time.Time); ok {
			occurred = t.In(financeReportTZ).Format("2006-01-02 15:04")
		}
		direction, kind := "Masuk", "Top Up Saldo"
		if row.strOr("direction", "") == "out" {
			direction, kind = "Keluar", "Pencairan Mitra"
		}
		b.WriteString(opCSVLine([]any{occurred, direction, kind,
			row.strOr("counterparty", ""), row.strOr("reference", ""), row.strOr("method", ""),
			finReportInt(row.numOr("gross_amount")), finReportInt(row.numOr("fee_amount")),
			finReportInt(row.numOr("amount")), row.strOr("status", "")}))
		b.WriteString("\n")
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		`attachment; filename="tauthub-laporan-keuangan-`+toDate.Format("2006-01-02")+`.csv"`)
	_, _ = w.Write([]byte(b.String()))
}
