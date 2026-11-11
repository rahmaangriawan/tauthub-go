package platform

import (
	"math"
	"net/http"
	"time"
)

// Endpoint detail "Posisi Keuangan" di dashboard admin: masing-masing kartu
// KPI balance-sheet bisa di-klik untuk melihat rincian ("milik siapa", "order
// mana", dst.). Semua hanya read-only, guard sama dengan FinanceReport.

// FinanceClientDepositsDetail: GET /v1/platform/finance/client-deposits-detail
//
// Rincian "Dana Titipan Klien" + "Dana Tertahan (Escrow)": semua brand yang
// masih punya saldo (available + reserved > 0), ordered by total desc.
func (d *Deps) FinanceClientDepositsDetail(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 200, 1, 1000)

	rows, err := d.queryRows(r.Context(),
		`SELECT w.brand_id::text         AS brand_id,
		        b.name                    AS brand_name,
		        COALESCE(b.client_name, '') AS client_name,
		        COALESCE(b.ownership_type, '') AS ownership_type,
		        w.available_tc,
		        w.reserved_tc,
		        MAX(le.created_at)        AS last_activity_at,
		        COALESCE(u.email, '')     AS owner_email,
		        COALESCE(u.display_name, '') AS owner_name
		   FROM credit_wallets w
		   JOIN brands b           ON b.id = w.brand_id
		   LEFT JOIN credit_ledger_entries le ON le.brand_id = w.brand_id
		   LEFT JOIN users u       ON u.id = b.created_by
		  WHERE (w.available_tc + w.reserved_tc) > 0
		    AND b.deleted_at IS NULL
		  GROUP BY w.brand_id, b.name, b.client_name, b.ownership_type,
		           w.available_tc, w.reserved_tc, u.email, u.display_name
		  ORDER BY (w.available_tc + w.reserved_tc) DESC
		  LIMIT $1`, limit)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	entries := make([]map[string]any, 0, len(rows))
	var totalAvailable, totalReserved int64
	for _, row := range rows {
		last, _ := row.raw("last_activity_at").(time.Time)
		var lastStr any
		if !last.IsZero() {
			lastStr = isoMilli(last)
		}
		available := int64(row.intOr("available_tc", 0))
		reserved := int64(row.intOr("reserved_tc", 0))
		totalAvailable += available
		totalReserved += reserved
		entries = append(entries, map[string]any{
			"brand_id":         row.strOr("brand_id", ""),
			"brand_name":       row.strOr("brand_name", ""),
			"client_name":      row.strOr("client_name", ""),
			"ownership_type":   row.strOr("ownership_type", ""),
			"available_tc":     available,
			"reserved_tc":      reserved,
			"total_tc":         available + reserved,
			"last_activity_at": lastStr,
			"owner_email":      row.strOr("owner_email", ""),
			"owner_name":       row.strOr("owner_name", ""),
		})
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":         entries,
		"total":           len(entries),
		"total_available": totalAvailable,
		"total_reserved":  totalReserved,
	})
}

// FinanceEscrowDetail: GET /v1/platform/finance/escrow-detail
//
// Rincian dana tertahan per publikasi aktif: order yang sedang dikerjakan
// mitra (belum terbit, belum dibatalkan). client_price dijumlahkan sebagai
// perkiraan dana escrow — angka pastinya ada di reserved_tc wallet; endpoint
// ini menjelaskan ORDER MANA yang mengunci dana itu.
func (d *Deps) FinanceEscrowDetail(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 200, 1, 1000)

	rows, err := d.queryRows(r.Context(),
		`SELECT p.id::text              AS publication_id,
		        p.status,
		        p.client_price,
		        p.partner_price,
		        p.created_at,
		        p.scheduled_at,
		        b.id::text              AS brand_id,
		        b.name                  AS brand_name,
		        COALESCE(b.client_name, '') AS client_name,
		        m.id::text              AS media_id,
		        COALESCE(m.name, '')    AS media_name,
		        pt.id::text             AS partner_id,
		        COALESCE(pt.name, '')   AS partner_name
		   FROM publications p
		   JOIN brands b   ON b.id = p.brand_id
		   LEFT JOIN media m   ON m.id = p.media_id
		   LEFT JOIN partners pt ON pt.id = m.partner_id
		  WHERE p.status IN ('pending','waiting_partner','partner_approved',
		                     'queued','processing','retrying')
		  ORDER BY p.created_at DESC
		  LIMIT $1`, limit)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	entries := make([]map[string]any, 0, len(rows))
	var totalEscrow float64
	for _, row := range rows {
		created, _ := row.raw("created_at").(time.Time)
		price := row.numOr("client_price")
		totalEscrow += price
		entries = append(entries, map[string]any{
			"publication_id": row.strOr("publication_id", ""),
			"status":         row.strOr("status", ""),
			"client_price":   finReportInt(price),
			"partner_price":  finReportInt(row.numOr("partner_price")),
			"created_at":     isoMilli(created),
			"brand_id":       row.strOr("brand_id", ""),
			"brand_name":     row.strOr("brand_name", ""),
			"client_name":    row.strOr("client_name", ""),
			"media_id":       row.strOr("media_id", ""),
			"media_name":     row.strOr("media_name", ""),
			"partner_id":     row.strOr("partner_id", ""),
			"partner_name":   row.strOr("partner_name", ""),
		})
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":            entries,
		"total":              len(entries),
		"total_client_price": finReportInt(totalEscrow),
	})
}

// FinancePartnerUnpaidDetail: GET /v1/platform/finance/partner-unpaid-detail
//
// Breakdown "Hak Mitra Belum Cair" per mitra:
// - amount_pending: earning belum di-settle (dibayar masih lama)
// - amount_settled_unpaid: sudah masuk settlement tapi settlement belum 'paid'
//   (siap transfer / sedang diproses / ditahan)
func (d *Deps) FinancePartnerUnpaidDetail(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 300, 1, 1000)

	rows, err := d.queryRows(r.Context(),
		`SELECT pt.id::text          AS partner_id,
		        COALESCE(pt.name, '') AS partner_name,
		        COALESCE(u.email, '') AS partner_email,
		        COALESCE(SUM(CASE WHEN e.status = 'pending' THEN e.amount ELSE 0 END), 0)
		          AS amount_pending,
		        COALESCE(SUM(CASE WHEN e.status = 'settled'
		                      AND (s.status IS NULL OR s.status <> 'paid')
		                     THEN e.amount ELSE 0 END), 0)
		          AS amount_settled_unpaid,
		        COUNT(*) FILTER (WHERE e.status = 'pending') AS count_pending,
		        COUNT(*) FILTER (WHERE e.status = 'settled'
		                           AND (s.status IS NULL OR s.status <> 'paid')) AS count_settled_unpaid,
		        MAX(e.created_at) AS last_earning_at
		   FROM partner_earnings e
		   JOIN partners pt ON pt.id = e.partner_id
		   LEFT JOIN users u ON u.id = pt.user_id
		   LEFT JOIN settlements s ON s.id = e.settlement_id
		  WHERE e.status = 'pending'
		     OR (e.status = 'settled' AND (s.id IS NULL OR s.status <> 'paid'))
		  GROUP BY pt.id, pt.name, u.email
		  ORDER BY (COALESCE(SUM(CASE WHEN e.status = 'pending' THEN e.amount ELSE 0 END), 0)
		          + COALESCE(SUM(CASE WHEN e.status = 'settled'
		                             AND (s.status IS NULL OR s.status <> 'paid')
		                            THEN e.amount ELSE 0 END), 0)) DESC
		  LIMIT $1`, limit)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	entries := make([]map[string]any, 0, len(rows))
	var sumTotal int64
	for _, row := range rows {
		pending := row.numOr("amount_pending")
		settled := row.numOr("amount_settled_unpaid")
		total := int64(math.Round(pending + settled))
		sumTotal += total
		last, _ := row.raw("last_earning_at").(time.Time)
		var lastStr any
		if !last.IsZero() {
			lastStr = isoMilli(last)
		}
		entries = append(entries, map[string]any{
			"partner_id":            row.strOr("partner_id", ""),
			"partner_name":          row.strOr("partner_name", ""),
			"partner_email":         row.strOr("partner_email", ""),
			"amount_pending":        finReportInt(pending),
			"amount_settled_unpaid": finReportInt(settled),
			"total":                 total,
			"count_pending":         row.intOr("count_pending", 0),
			"count_settled_unpaid":  row.intOr("count_settled_unpaid", 0),
			"last_earning_at":       lastStr,
		})
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"total":   len(entries),
		"sum":     sumTotal,
	})
}

// FinanceProfitBreakdown: GET /v1/platform/finance/profit-breakdown
//
// Top publikasi pembentuk Profit Bersih Tauthub: komisi = client_price × 25%
// per publikasi 'published'. Paginasi dan sort desc dari komisi, supaya admin
// bisa cek top kontributor. Juga ringkasan agregat per bulan (12 bulan).
func (d *Deps) FinanceProfitBreakdown(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 100, 1, 500)
	offset := clampInt(r.URL.Query().Get("offset"), 0, 0, 1<<30)

	countRow, _ := d.queryRow(r.Context(),
		`SELECT COUNT(*)::int AS n FROM publications WHERE status = 'published'`)

	rows, err := d.queryRows(r.Context(),
		`SELECT p.id::text              AS publication_id,
		        p.client_price,
		        ROUND(p.client_price * 0.25) AS commission,
		        p.published_at,
		        p.published_url,
		        b.name                   AS brand_name,
		        COALESCE(b.client_name, '') AS client_name,
		        COALESCE(m.name, '')     AS media_name,
		        COALESCE(pt.name, '')    AS partner_name
		   FROM publications p
		   JOIN brands b   ON b.id = p.brand_id
		   LEFT JOIN media m   ON m.id = p.media_id
		   LEFT JOIN partners pt ON pt.id = m.partner_id
		  WHERE p.status = 'published'
		  ORDER BY p.client_price DESC, p.published_at DESC
		  LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	entries := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		publishedAt, _ := row.raw("published_at").(time.Time)
		var publishedStr any
		if !publishedAt.IsZero() {
			publishedStr = isoMilli(publishedAt)
		}
		entries = append(entries, map[string]any{
			"publication_id": row.strOr("publication_id", ""),
			"client_price":   finReportInt(row.numOr("client_price")),
			"commission":     finReportInt(row.numOr("commission")),
			"published_at":   publishedStr,
			"published_url":  row.strOr("published_url", ""),
			"brand_name":     row.strOr("brand_name", ""),
			"client_name":    row.strOr("client_name", ""),
			"media_name":     row.strOr("media_name", ""),
			"partner_name":   row.strOr("partner_name", ""),
		})
	}

	// Ringkasan per bulan (12 bulan terakhir) — konsisten dengan chart utama.
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -11, 0)
	monthEnd := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	monthRows, _ := d.queryRows(r.Context(),
		`SELECT to_char(published_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM') AS month,
		        COALESCE(SUM(client_price), 0)             AS gmv,
		        COALESCE(ROUND(SUM(client_price) * 0.25), 0) AS commission,
		        COUNT(*)                                     AS count
		   FROM publications
		  WHERE status = 'published' AND published_at IS NOT NULL
		    AND published_at >= $1 AND published_at < $2
		  GROUP BY 1 ORDER BY 1`, monthStart, monthEnd)

	monthly := make([]map[string]any, 0, len(monthRows))
	for _, row := range monthRows {
		monthly = append(monthly, map[string]any{
			"month":      row.strOr("month", ""),
			"gmv":        finReportInt(row.numOr("gmv")),
			"commission": finReportInt(row.numOr("commission")),
			"count":      row.intOr("count", 0),
		})
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":          entries,
		"total":            countRow.intOr("n", 0),
		"limit":            limit,
		"offset":           offset,
		"monthly":          monthly,
	})
}
