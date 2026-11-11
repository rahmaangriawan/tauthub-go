package platform

import (
	"context"
	"math"
	"time"
)

// Konteks filter untuk dashboard keuangan admin: daftar ID brand & partner
// yang DIKECUALIKAN dari semua agregasi (saldo klien, hak mitra, profit,
// publikasi), plus helper AS-OF untuk merekonstruksi posisi wallet pada
// tanggal lampau dari credit_ledger_entries.

// finExcludedUserEmails: akun internal / dev / smoke-test yang tidak boleh
// ikut hitungan dashboard keuangan. Email ini di-resolve ke brand_id (via
// brands.created_by) DAN partner_id (via partners.user_id) — jadi akun klien
// otomatis bikin saldo & order-nya dikecualikan, dan akun mitra otomatis
// bikin earning-nya dikecualikan. Perubahan daftar butuh deploy ulang
// (sengaja, biar auditable).
var finExcludedUserEmails = []string{
	"farihcoid@gmail.com",      // internal/dev
	"wantek@gmail.com",         // internal/dev
	"mitra@tauthub.com",        // akun mitra internal (exclude penghasilan)
	"smoke.test@example.com",   // akun klien smoke-test (exclude transaksi)
}

// finFilterCtx: ID yang dikecualikan, dihitung sekali per request.
type finFilterCtx struct {
	brandIDs   []string
	partnerIDs []string
}

// finLoadFilter: resolve email yang dikecualikan ke list brand_id + partner_id.
// Dipakai di semua agregasi finansial. Gagalnya non-fatal — kembalikan slice
// kosong supaya laporan tetap jalan (tanpa exclusion).
func (d *Deps) finLoadFilter(ctx context.Context) finFilterCtx {
	out := finFilterCtx{brandIDs: []string{}, partnerIDs: []string{}}
	if len(finExcludedUserEmails) == 0 {
		return out
	}

	brandRows, err := d.queryRows(ctx, `
		SELECT b.id::text AS id
		FROM brands b JOIN users u ON u.id = b.created_by
		WHERE u.email = ANY($1::text[])`, finExcludedUserEmails)
	if err == nil {
		for _, row := range brandRows {
			if id := row.strOr("id", ""); id != "" {
				out.brandIDs = append(out.brandIDs, id)
			}
		}
	}

	partnerRows, err := d.queryRows(ctx, `
		SELECT pt.id::text AS id
		FROM partners pt JOIN users u ON u.id = pt.user_id
		WHERE u.email = ANY($1::text[])`, finExcludedUserEmails)
	if err == nil {
		for _, row := range partnerRows {
			if id := row.strOr("id", ""); id != "" {
				out.partnerIDs = append(out.partnerIDs, id)
			}
		}
	}
	return out
}

// Helper array aman untuk "!= ALL" di postgres — slice kosong tetap jalan,
// karena `x != ALL(ARRAY[]::text[])` selalu true.
func (f finFilterCtx) brandArr() []string {
	if f.brandIDs == nil {
		return []string{}
	}
	return f.brandIDs
}
func (f finFilterCtx) partnerArr() []string {
	if f.partnerIDs == nil {
		return []string{}
	}
	return f.partnerIDs
}

// finDeposits: posisi saldo klien pada satu titik waktu, dipecah menurut
// sumber uangnya.
type finDeposits struct {
	realAvailable int64 // bagian available yang ditopang topup asli (titipan nyata)
	adjusted      int64 // sisa available: penyesuaian manual / bonus / kompensasi
	reserved      int64 // dana tertahan (escrow), apa pun sumbernya
}

// finDepositsAsOf: saldo semua brand yang tidak dikecualikan pada `asOf`.
//
// Titik tolaknya `credit_wallets` hari ini (otoritatif), lalu mutasi ledger
// SETELAH asOf dibalik per brand. Ledger sendiri tidak lengkap, jadi menjumlah
// dari nol akan meleset.
//
// Pemisahan nyata vs penyesuaian, per brand: saldo dianggap "nyata" sebesar
// total topup production brand itu sampai asOf, dibatasi saldo yang tersisa.
// Artinya saldo penyesuaian dianggap terpakai lebih dulu — pilihan konservatif
// supaya uang yang wajib ada di rekening tidak terhitung kurang. Dana tertahan
// mengambil jatah nyata lebih dulu sebelum available.
//
// Konvensi tanda amount_tc (lihat insertLedger di clienttaut/brandflow):
//   topup/refund/adjustment/release : bertanda, mengubah available
//   reserve (negatif)               : available turun, reserved naik
//   capture positif                 : reserved -> captured
//   capture negatif                 : available -> captured (tanpa reserve)
func (d *Deps) finDepositsAsOf(ctx context.Context, asOf time.Time, excl finFilterCtx) (finDeposits, error) {
	row, err := d.queryRow(ctx, `
		WITH w AS (
		  SELECT brand_id::text AS id, available_tc AS a, reserved_tc AS r
		  FROM credit_wallets
		  WHERE brand_id::text != ALL($1::text[])
		), dl AS (
		  SELECT brand_id::text AS id,
		    SUM(CASE
		      WHEN ref_type IN ('topup','refund','adjustment','reserve','release') THEN amount_tc
		      WHEN ref_type = 'capture' AND amount_tc < 0 THEN amount_tc
		      ELSE 0 END) AS da,
		    SUM(CASE
		      WHEN ref_type IN ('reserve','release') THEN -amount_tc
		      WHEN ref_type = 'capture' AND amount_tc > 0 THEN -amount_tc
		      ELSE 0 END) AS dr
		  FROM credit_ledger_entries
		  WHERE created_at > $2
		  GROUP BY 1
		), tp AS (
		  SELECT t.brand_id::text AS id, SUM(t.amount) AS real_in
		  FROM payment_transactions t
		  WHERE `+financeRealTopupCond+`
		    AND COALESCE(t.paid_at, t.created_at) <= $2
		  GROUP BY 1
		), b AS (
		  SELECT GREATEST(w.a - COALESCE(dl.da, 0), 0) AS a,
		         GREATEST(w.r - COALESCE(dl.dr, 0), 0) AS r,
		         COALESCE(tp.real_in, 0) AS real_in
		  FROM w LEFT JOIN dl USING (id) LEFT JOIN tp USING (id)
		)
		SELECT COALESCE(SUM(a), 0) AS available,
		       COALESCE(SUM(r), 0) AS reserved,
		       COALESCE(SUM(LEAST(a, GREATEST(LEAST(a + r, real_in) - r, 0))), 0) AS real_available
		FROM b`, excl.brandArr(), asOf)
	if err != nil {
		return finDeposits{}, err
	}
	available := int64(math.Round(row.numOr("available")))
	real := int64(math.Round(row.numOr("real_available")))
	return finDeposits{
		realAvailable: real,
		adjusted:      available - real,
		reserved:      int64(math.Round(row.numOr("reserved"))),
	}, nil
}

// finPartnerUnpaidAsOf: komisi mitra yang BELUM dibayar pada titik `asOf`.
// Earning dari order brand yang dikecualikan (akun uji) tidak dihitung: tidak
// ada uang gateway di baliknya. Best-effort: earning yang dibuat <= asOf dan status sekarang bukan
// "sudah dibayar sebelum asOf". Earning voided belakangan tetap tidak dihitung
// (approksimasi — audit log untuk status historis tidak tersedia).
func (d *Deps) finPartnerUnpaidAsOf(ctx context.Context, asOf time.Time, excl finFilterCtx) (int64, error) {
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(SUM(e.amount), 0) AS total
		FROM partner_earnings e
		LEFT JOIN settlements s ON s.id = e.settlement_id
		WHERE e.created_at <= $1
		  AND e.status IN ('pending','settled')
		  AND e.partner_id::text != ALL($2::text[])
		  AND NOT EXISTS (
		    SELECT 1 FROM publications pb
		    WHERE pb.id = e.publication_id AND pb.brand_id::text = ANY($3::text[]))
		  AND NOT (
		    e.status = 'settled'
		    AND s.id IS NOT NULL
		    AND s.status = 'paid'
		    AND s.paid_at IS NOT NULL
		    AND s.paid_at <= $1
		  )`, asOf, excl.partnerArr(), excl.brandArr())
	if err != nil {
		return 0, err
	}
	return int64(math.Round(row.numOr("total"))), nil
}

// finProfitInRange: profit Tauthub (gross 25% × GMV) dari publikasi
// `published_at` dalam [from, to) + pencatatan penarikan admin pada rentang
// yang sama. Brand yang dikecualikan tidak dihitung.
//
// Range konvensi: from inklusif, to eksklusif (sudah di-+1 hari oleh caller).
func (d *Deps) finProfitInRange(ctx context.Context, from, toExcl time.Time, excl finFilterCtx) (gross, withdrawn, available int64, err error) {
	row, qErr := d.queryRow(ctx, `
		SELECT COALESCE(ROUND(SUM(client_price) * 0.25), 0)::bigint AS gross,
		       COALESCE(COUNT(*), 0) AS count
		FROM publications
		WHERE status = 'published'
		  AND published_at IS NOT NULL
		  AND published_at >= $1 AND published_at < $2
		  AND brand_id::text != ALL($3::text[])`, from, toExcl, excl.brandArr())
	if qErr != nil {
		return 0, 0, 0, qErr
	}
	gross = int64(math.Round(row.numOr("gross")))

	wdRow, qErr := d.queryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)::bigint AS total
		FROM admin_profit_withdrawals
		WHERE status = 'paid'
		  AND created_at >= $1 AND created_at < $2`, from, toExcl)
	if qErr != nil {
		return gross, 0, gross, qErr
	}
	withdrawn = int64(math.Round(wdRow.numOr("total")))
	available = gross - withdrawn
	if available < 0 {
		available = 0
	}
	return
}
