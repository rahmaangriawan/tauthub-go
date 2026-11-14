package brandflow

import (
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// calendar.go: Kalender editorial klien, penempatan artikel per bulan.
//
// Tanggal sebuah penempatan = published_at bila sudah terbit, selain itu
// scheduled_at. Hanya membaca data; penjadwalan tidak diubah di sini.

// CalendarMonth: GET /v1/brands/{brandId}/calendar?month=YYYY-MM
func (d *Deps) CalendarMonth(w http.ResponseWriter, r *http.Request) {
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
	monthParam := r.URL.Query().Get("month")
	if monthParam == "" {
		monthParam = time.Now().Format("2006-01")
	}
	start, err := time.Parse("2006-01", monthParam)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Format bulan harus YYYY-MM")
		return
	}
	end := start.AddDate(0, 1, 0)

	rows, err := d.queryRows(r.Context(), `
		SELECT p.id, p.status, p.publish_status, p.scheduled_at, p.published_at, p.published_url,
		       p.campaign_id, d.title AS document_title, m.name AS media_name,
		       COALESCE(p.published_at, p.scheduled_at) AS event_at
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		WHERE p.brand_id = $1 AND p.status <> 'cancelled'
		  AND COALESCE(p.published_at, p.scheduled_at) >= $2
		  AND COALESCE(p.published_at, p.scheduled_at) < $3
		ORDER BY COALESCE(p.published_at, p.scheduled_at) ASC
		LIMIT 1000`, brandRow.ID, start, end)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id": row.strOr("id", ""), "status": row.strOr("status", ""),
			"publish_status": row.strOr("publish_status", ""),
			"document_title": row.strOr("document_title", ""), "media_name": row.strOr("media_name", ""),
			"scheduled_at": row.timePtr("scheduled_at"), "published_at": row.timePtr("published_at"),
			"event_at": row.timePtr("event_at"),
			"published_url": bsStrOrNil(row.strOr("published_url", "")),
			"campaign_id": bsStrOrNil(row.strOr("campaign_id", "")),
		})
	}
	writeNoStore(w, http.StatusOK, map[string]any{"month": start.Format("2006-01"), "items": items})
}
