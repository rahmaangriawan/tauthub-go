package brandflow

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ReportsDaily: GET /v1/brands/{brandId}/reports/daily?days=30
//
// Deret harian untuk grafik di beranda klien. Memakai tabel dan jenis kejadian
// yang sama dengan kartu ringkasan (ReportsGet): "reach" = tracking_link_click
// (Estimasi Jangkauan) dan "clicks" = outbound_click (Total Klik Smart Link),
// supaya angka grafik dan kartu tidak berbeda. Hari tanpa kejadian diisi nol
// dan hari dikelompokkan menurut WIB.
func (d *Deps) ReportsDaily(w http.ResponseWriter, r *http.Request) {
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
	days := 30
	if n, convErr := strconv.Atoi(r.URL.Query().Get("days")); convErr == nil {
		days = n
	}
	if days < 7 {
		days = 7
	}
	if days > 90 {
		days = 90
	}

	loc, locErr := time.LoadLocation("Asia/Jakarta")
	if locErr != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	today := time.Now().In(loc)
	startDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(days - 1))

	// Dua id dipakai bersamaan: kartu ringkasan menghitung klik tautan dengan id
	// kanonik dan klik keluar dengan id dari URL (parity route lama), jadi
	// keduanya dicocokkan di sini supaya angkanya sama dengan kartu.
	rows, err := d.queryRows(r.Context(), `
		SELECT to_char(created_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD') AS day, event_type, COUNT(*)::int AS n
		FROM tracking_events
		WHERE (brand_id = $1 OR brand_id = $2) AND created_at >= $3
		  AND event_type IN ('tracking_link_click', 'outbound_click')
		GROUP BY 1, 2`,
		brandRow.ID, chiParam(r, "brandId"), startDay)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	type counts struct{ reach, clicks int }
	byDay := map[string]*counts{}
	for _, row := range rows {
		c := byDay[row.strOr("day", "")]
		if c == nil {
			c = &counts{}
			byDay[row.strOr("day", "")] = c
		}
		switch row.strOr("event_type", "") {
		case "tracking_link_click":
			c.reach += row.intOr("n", 0)
		case "outbound_click":
			c.clicks += row.intOr("n", 0)
		}
	}
	points := make([]map[string]any, 0, days)
	totalReach, totalClicks := 0, 0
	for i := 0; i < days; i++ {
		key := startDay.AddDate(0, 0, i).Format("2006-01-02")
		reach, clicks := 0, 0
		if c := byDay[key]; c != nil {
			reach, clicks = c.reach, c.clicks
		}
		totalReach += reach
		totalClicks += clicks
		points = append(points, map[string]any{"date": key, "reach": reach, "clicks": clicks})
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"days": days, "points": points, "total_reach": totalReach, "total_clicks": totalClicks,
	})
}
