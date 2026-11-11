package platform

import (
	"math"
	"net/http"
	"time"
)

// FinanceDormantBrands: GET /v1/platform/finance/dormant-brands
//
// Daftar brand yang punya saldo kredit > 0 tapi tidak ada aktivitas wallet
// sejak N hari (default 90). Memakai created_at terakhir di
// credit_ledger_entries sebagai penanda "aktivitas terakhir"; brand yang
// belum pernah ada ledger entry sama sekali juga ikut dengan last_activity
// NULL (days_dormant 999) supaya admin bisa follow up.
func (d *Deps) FinanceDormantBrands(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	days := clampInt(r.URL.Query().Get("days"), 90, 7, 3650)
	limit := clampInt(r.URL.Query().Get("limit"), 100, 1, 500)

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
		    AND (b.deleted_at IS NULL)
		  GROUP BY w.brand_id, b.name, b.client_name, b.ownership_type,
		           w.available_tc, w.reserved_tc, u.email, u.display_name
		 HAVING MAX(le.created_at) IS NULL
		     OR MAX(le.created_at) < NOW() - ($1 || ' days')::interval
		  ORDER BY (w.available_tc + w.reserved_tc) DESC
		  LIMIT $2`, days, limit)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	entries := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		last, _ := row.raw("last_activity_at").(time.Time)
		var lastStr any
		var daysDormant int
		if !last.IsZero() {
			lastStr = isoMilli(last)
			daysDormant = int(math.Floor(time.Since(last).Hours() / 24))
		} else {
			lastStr = nil
			daysDormant = -1 // tidak pernah ada aktivitas
		}
		available := row.intOr("available_tc", 0)
		reserved := row.intOr("reserved_tc", 0)
		entries = append(entries, map[string]any{
			"brand_id":         row.strOr("brand_id", ""),
			"brand_name":       row.strOr("brand_name", ""),
			"client_name":      row.strOr("client_name", ""),
			"ownership_type":   row.strOr("ownership_type", ""),
			"available_tc":     available,
			"reserved_tc":      reserved,
			"total_tc":         available + reserved,
			"last_activity_at": lastStr,
			"days_dormant":     daysDormant,
			"owner_email":      row.strOr("owner_email", ""),
			"owner_name":       row.strOr("owner_name", ""),
		})
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":   entries,
		"threshold": days,
		"total":     len(entries),
	})
}
