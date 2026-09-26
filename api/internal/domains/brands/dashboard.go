package brands

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/{brandId}/dashboard/route.ts — metrik + aktivitas + kuota,
// Cache-Control private max-age=10.

func (d *Deps) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	scoped, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeError(w, err)
		return
	}
	canonicalID := scoped.ID

	month := time.Now()
	month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)

	count := func(query string, args ...any) int {
		var n int
		_ = d.Pool.QueryRow(r.Context(), query, args...).Scan(&n)
		return n
	}

	inProgress := count(
		"SELECT COUNT(*) FROM content_jobs WHERE brand_id = $1 AND status IN ('queued','running','waiting_outline_approval')", canonicalID)
	review := count(
		"SELECT COUNT(*) FROM content_documents WHERE brand_id = $1 AND status = 'in_review'", canonicalID)
	failed := count(
		"SELECT COUNT(*) FROM content_jobs WHERE brand_id = $1 AND status = 'failed'", canonicalID)
	articles := count(
		"SELECT COUNT(*) FROM content_documents WHERE brand_id = $1 AND created_at >= $2", canonicalID, month)

	activity := []map[string]any{}
	aRows, aErr := d.Pool.Query(r.Context(),
		"SELECT id, action, target_type, target_id, created_at FROM audit_logs WHERE brand_id = $1 ORDER BY created_at DESC LIMIT 8", canonicalID)
	if aErr == nil {
		defer aRows.Close()
		for aRows.Next() {
			var id, action string
			var targetType, targetID *string
			var createdAt time.Time
			if aRows.Scan(&id, &action, &targetType, &targetID, &createdAt) == nil {
				activity = append(activity, map[string]any{
					"id": id, "action": action, "target_type": targetType, "target_id": targetID,
					"created_at": isoMilli(createdAt),
				})
			}
		}
	}

	var monthlyLimit int
	var warningPercent int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT monthly_article_limit, warning_percent FROM brand_ai_quotas WHERE brand_id = $1", canonicalID).
		Scan(&monthlyLimit, &warningPercent)
	if monthlyLimit == 0 {
		monthlyLimit = 50
		warningPercent = 80
	}
	percent := 0
	if monthlyLimit != 0 {
		percent = int(float64(articles) / float64(monthlyLimit) * 1000)
		if percent%10 >= 5 && percent >= 0 {
			percent = (percent + 5) / 10
		} else {
			percent = percent / 10
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=10, stale-while-revalidate=30")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"brand_id":           canonicalID,
		"role":               scoped.Role,
		"active_brands":      boolToInt(scoped.Status == "active"),
		"total_brands":       1,
		"content_in_progress": inProgress,
		"waiting_review":     review,
		"failed_jobs":        failed,
		"monthly_articles": map[string]any{
			"used": articles, "limit": monthlyLimit, "percent": percent,
			"warning": percent >= warningPercent,
		},
		"recent_activity": activity,
	})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var _ = httpx.Error
