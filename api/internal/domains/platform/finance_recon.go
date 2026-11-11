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
// bergantung pada filter periode. Komponen dihitung terpisah dan sisanya
// ditampilkan sebagai "selisih lain", jadi total komponen selalu sama dengan
// selisih.

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

	nonPub, err := d.queryRow(ctx, `
		SELECT COALESCE(-SUM(amount_tc) FILTER (WHERE ref_type = 'capture' AND amount_tc < 0), 0)
		     + COALESCE(-SUM(amount_tc) FILTER (WHERE ref_type = 'adjustment'), 0) AS v
		FROM credit_ledger_entries
		WHERE created_at <= $1 AND brand_id::text != ALL($2::text[])`, asOf, excl.brandArr())
	if err != nil {
		return nil, err
	}
	orphanSum, err := d.queryRow(ctx, `
		SELECT COALESCE(SUM(credit), 0) AS v, COUNT(*) AS n
		FROM (SELECT `+finOrphanCreditExpr+` AS credit `+finOrphanPubsFrom+`) x`, asOf, excl.brandArr())
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

	aInternalPaid := pr.internalPaid
	bNonPub := int64(math.Round(nonPub.numOr("v")))
	cOrphan := int64(math.Round(orphanSum.numOr("v")))
	residual := gap - aInternalPaid - bNonPub - cOrphan + fee

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
			{"key": "internal_paid", "label": "Pencairan mitra internal yang sudah dibayar", "amount": aInternalPaid,
				"note": "Dihitung sebagai penarikan profit, tetapi saldo gateway tidak berkurang karena dibayar dari luar gateway."},
			{"key": "non_publication", "label": "Pemakaian kredit non-publikasi", "amount": bNonPub,
				"note": "AI dan potongan saldo manual. Pendapatan Tauthub yang belum masuk hitungan profit."},
			{"key": "orphan_publications", "label": "Publikasi terbit tanpa catatan penghasilan", "amount": cOrphan,
				"note": "Kredit sudah terpakai tetapi tidak tercatat sebagai profit maupun hak mitra."},
			{"key": "gateway_fee", "label": "Fee gateway", "amount": -fee,
				"note": "Dipotong gateway dari topup, tidak pernah menjadi uang Tauthub."},
			{"key": "other", "label": "Selisih lain", "amount": residual,
				"note": "Mis. klien membayar di bawah harga list, sementara profit dihitung dari harga list."},
		},
		"orphan_count":        orphanSum.intOr("n", 0),
		"orphan_publications": orphans,
	}, nil
}
