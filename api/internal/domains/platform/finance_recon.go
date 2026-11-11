package platform

import (
	"context"
	"math"
	"time"
)

// Rekonsiliasi saldo gateway dengan "Saldo Rek. Wajib Ada".
//
// Saldo gateway diperkirakan dari net topup production dikurangi penarikan
// profit yang tercatat. Pencairan mitra yang dibayar lewat gateway belum
// dikurangkan (aplikasi tidak tahu sumber dananya), jadi angkanya harus
// dicocokkan dengan saldo di halaman gateway.
//
// Selisih = saldo gateway - wajib ada. Wajib ada di sini kumulatif sampai
// tanggal akhir rentang, bukan hanya profit dalam rentang, supaya tidak
// bergantung pada filter periode. Profit dihitung dari uang yang diterima
// (lihat finProfitInRange), jadi satu-satunya penyebab selisih yang diketahui
// adalah pencairan mitra internal yang dibayar dari luar gateway. Sisanya
// ditampilkan sebagai "selisih lain", yang seharusnya nol.

const finOrphanPubsFrom = `
	FROM publications p
	LEFT JOIN media m ON m.id = p.media_id
	LEFT JOIN partners pt ON pt.id = m.partner_id
	WHERE p.status = 'published'
	  AND p.brand_id::text != ALL($2::text[])
	  AND p.created_at <= $1
	  AND NOT EXISTS (
	    SELECT 1 FROM partner_earnings e
	    WHERE e.publication_id = p.id AND e.kind = 'publication' AND e.status <> 'voided')`

const finOrphanCreditExpr = `COALESCE((SELECT SUM(l.amount_tc) FROM credit_ledger_entries l
	WHERE l.ref_id = p.id AND l.ref_type = 'capture' AND l.created_at <= $1), 0)`

func (d *Deps) finGatewayRecon(ctx context.Context, asOf, toExcl time.Time, excl finFilterCtx, dep finDeposits, partnerUnpaid int64) (map[string]any, error) {
	inRow, err := d.queryRow(ctx, `
		SELECT COALESCE(SUM(t.amount), 0) AS gross,
		       COALESCE(SUM(`+financeTopupFeeExpr+`), 0) AS fee
		FROM payment_transactions t
		WHERE `+financeRealTopupCond+`
		  AND COALESCE(t.paid_at, t.created_at) <= $1`, asOf)
	if err != nil {
		return nil, err
	}
	gross := int64(math.Round(inRow.numOr("gross")))
	fee := int64(math.Round(inRow.numOr("fee")))
	net := gross - fee

	pr, err := d.finProfitInRange(ctx, time.Time{}, toExcl, excl)
	if err != nil {
		return nil, err
	}

	required := dep.realAvailable + dep.reserved + partnerUnpaid + pr.available
	balance := net - pr.adminWithdrawn
	gap := balance - required

	orphanCount, err := d.queryRow(ctx, `SELECT COUNT(*) AS n FROM (SELECT 1 `+finOrphanPubsFrom+`) x`, asOf, excl.brandArr())
	if err != nil {
		return nil, err
	}
	orphanRows, err := d.queryRows(ctx, `
		SELECT COALESCE(m.name, '') AS media, COALESCE(pt.name, '') AS partner,
		       p.client_price AS price, `+finOrphanCreditExpr+` AS credit
		`+finOrphanPubsFrom+`
		ORDER BY p.created_at DESC LIMIT 20`, asOf, excl.brandArr())
	if err != nil {
		return nil, err
	}

	residual := gap - pr.internalPaid

	orphans := make([]map[string]any, 0, len(orphanRows))
	for _, row := range orphanRows {
		orphans = append(orphans, map[string]any{
			"media":        row.strOr("media", ""),
			"partner":      row.strOr("partner", ""),
			"client_price": finReportInt(row.numOr("price")),
			"credit":       finReportInt(row.numOr("credit")),
		})
	}

	return map[string]any{
		"gateway_in_gross":    gross,
		"gateway_fee":         fee,
		"gateway_in_net":      net,
		"admin_withdrawn":     pr.adminWithdrawn,
		"gateway_balance_est": balance,
		"required":            required,
		"gap":                 gap,
		"components": []map[string]any{
			{"key": "internal_paid", "label": "Pencairan mitra internal yang sudah dibayar", "amount": pr.internalPaid,
				"note": "Dihitung sebagai penarikan profit, tetapi saldo gateway tidak berkurang karena dibayar dari luar gateway."},
			{"key": "other", "label": "Selisih lain", "amount": residual,
				"note": "Seharusnya nol. Bila tidak nol, ada uang masuk atau keluar yang belum tercatat di sistem."},
		},
		"orphan_count":        orphanCount.intOr("n", 0),
		"orphan_publications": orphans,
	}, nil
}
