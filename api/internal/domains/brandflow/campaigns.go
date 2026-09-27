package brandflow

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// campaigns.go: port app/v1/brands/[brandId]/campaigns/**/route.ts.

// ---------- GET / POST /v1/brands/{brandId}/campaigns ----------

// CampaignsList: GET /v1/brands/{brandId}/campaigns
func (d *Deps) CampaignsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !validCampaignStatus(status) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid campaign status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := intParamDef(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	clause := "WHERE c.brand_id = $1 AND c.taut_status IS NULL"
	baseParams := []any{brandRow.ID}
	if status != "" {
		clause += " AND c.status = $2"
		baseParams = append(baseParams, status)
	}

	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM campaigns c "+clause, baseParams...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	total := 0
	if countRow != nil {
		total = countRow.intOr("count", 0)
	}
	pageArgs := append(append([]any{}, baseParams...), limit, offset)
	items, err := d.queryRows(r.Context(),
		"SELECT c.*, "+campaignAggregateSelect+" FROM campaigns c "+clause+
			" ORDER BY c.created_at DESC LIMIT $"+itoa(len(baseParams)+1)+" OFFSET $"+itoa(len(baseParams)+2),
		pageArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaigns := make([]map[string]any, 0, len(items))
	for _, item := range items {
		campaigns = append(campaigns, campaignJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaigns": campaigns,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// CampaignsCreate: POST /v1/brands/{brandId}/campaigns
func (d *Deps) CampaignsCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)

	name, _, err := requiredTrimmedString(body, "name", 2, 160, nil)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	description, err := optionalString(body, "description", 2000)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	status := "draft"
	if raw, ok := body["status"]; ok {
		s, isStr := raw.(string)
		if !isStr || !validCampaignStatus(s) {
			d.writeErr(w, errUnprocessable("Invalid option: expected one of \"draft\"|\"active\"|\"paused\"|\"completed\"|\"cancelled\""))
			return
		}
		status = s
	}
	quota := 0
	if raw, ok := body["article_quota"]; ok {
		f, isNum := raw.(float64)
		if !isNum || f != float64(int(f)) || f < 0 || f > 1_000_000 {
			d.writeErr(w, errUnprocessable("Number must be less than or equal to 1000000"))
			return
		}
		quota = int(f)
	}
	budget := 0.0
	if raw, ok := body["budget"]; ok {
		if f, ok := toFloat(raw); ok {
			if f < 0 || f > 1_000_000_000_000 {
				d.writeErr(w, errUnprocessable("Number must be less than or equal to 1000000000000"))
				return
			}
			budget = f
		} else {
			d.writeErr(w, errUnprocessable("Expected number, received string"))
			return
		}
	}
	startsAt, err := optionalDatetime(body, "starts_at")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	endsAt, err := optionalDatetime(body, "ends_at")
	if err != nil {
		d.writeErr(w, err)
		return
	}

	id := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO campaigns (id, brand_id, name, description, status, article_quota, budget, starts_at, ends_at, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
		id, brandRow.ID, name, description, status, quota, budget, startsAt, endsAt, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	created, err := d.queryRow(r.Context(),
		"SELECT c.*, 0 AS publication_count FROM campaigns c WHERE c.id = $1 LIMIT 1", id)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if created == nil {
		d.writeErr(w, errNotFound("Campaign not found"))
		return
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "campaign.created", "campaign", id)
	writeJSON(w, http.StatusCreated, campaignJSON(created))
}

// ---------- GET / PUT / DELETE /v1/brands/{brandId}/campaigns/{campaignId} ----------

// CampaignGet: GET /v1/brands/{brandId}/campaigns/{campaignId}
func (d *Deps) CampaignGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, chiParam(r, "campaignId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, campaignJSON(campaign))
}

// CampaignUpdate: PUT /v1/brands/{brandId}/campaigns/{campaignId}
func (d *Deps) CampaignUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	if _, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)

	var nameArg any
	if raw, ok := body["name"]; ok {
		s, isStr := raw.(string)
		if !isStr {
			d.writeErr(w, errUnprocessable("Expected string, received number"))
			return
		}
		trimmed := strings.TrimSpace(s)
		if len([]rune(trimmed)) < 2 {
			d.writeErr(w, errUnprocessable("String must contain at least 2 character(s)"))
			return
		}
		if len([]rune(trimmed)) > 160 {
			d.writeErr(w, errUnprocessable("String must contain at most 160 character(s)"))
			return
		}
		nameArg = trimmed
	}
	var descArg any
	if raw, ok := body["description"]; ok {
		if raw == nil {
			descArg = nil
		} else if s, isStr := raw.(string); isStr {
			if len([]rune(s)) > 2000 {
				d.writeErr(w, errUnprocessable("String must contain at most 2000 character(s)"))
				return
			}
			descArg = s
		} else {
			d.writeErr(w, errUnprocessable("Expected string, received number"))
			return
		}
	}
	var statusArg any
	if raw, ok := body["status"]; ok {
		s, isStr := raw.(string)
		if !isStr || !validCampaignStatus(s) {
			d.writeErr(w, errUnprocessable("Invalid option: expected one of \"draft\"|\"active\"|\"paused\"|\"completed\"|\"cancelled\""))
			return
		}
		statusArg = s
	}
	var quotaArg any
	if raw, ok := body["article_quota"]; ok {
		f, isNum := raw.(float64)
		if !isNum || f != float64(int(f)) || f < 0 || f > 1_000_000 {
			d.writeErr(w, errUnprocessable("Number must be less than or equal to 1000000"))
			return
		}
		quotaArg = int(f)
	}
	var budgetArg any
	if raw, ok := body["budget"]; ok {
		if f, ok := toFloat(raw); ok {
			if f < 0 || f > 1_000_000_000_000 {
				d.writeErr(w, errUnprocessable("Number must be less than or equal to 1000000000000"))
				return
			}
			budgetArg = f
		} else {
			d.writeErr(w, errUnprocessable("Expected number, received string"))
			return
		}
	}
	startsAt, err := optionalDatetime(body, "starts_at")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	endsAt, err := optionalDatetime(body, "ends_at")
	if err != nil {
		d.writeErr(w, err)
		return
	}

	if err := d.exec(r.Context(), `
		UPDATE campaigns SET
		  name = COALESCE($1, name),
		  description = COALESCE($2, description),
		  status = COALESCE($3, status),
		  article_quota = COALESCE($4, article_quota),
		  budget = COALESCE($5, budget),
		  starts_at = COALESCE($6, starts_at),
		  ends_at = COALESCE($7, ends_at),
		  updated_at = NOW()
		WHERE id = $8`,
		nameArg, descArg, statusArg, quotaArg, budgetArg, startsAt, endsAt, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(), "SELECT * FROM campaigns WHERE id = $1 LIMIT 1", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		d.writeErr(w, errNotFound("Campaign not found"))
		return
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "campaign.updated", "campaign", campaignID)
	writeJSON(w, http.StatusOK, campaignJSON(row))
}

// CampaignCancel: DELETE /v1/brands/{brandId}/campaigns/{campaignId}
func (d *Deps) CampaignCancel(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	if _, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(), "UPDATE campaigns SET status = 'cancelled', updated_at = NOW() WHERE id = $1", campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "campaign.cancelled", "campaign", campaignID)
	updated, err := d.scopedCampaign(r.Context(), brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, campaignJSON(updated))
}

// ---------- report share ----------

// CampaignReportShare: POST /v1/brands/{brandId}/campaigns/{campaignId}/report/share
func (d *Deps) CampaignReportShare(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaign, err := d.scopedCampaign(r.Context(), brandRow.ID, chiParam(r, "campaignId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := campaign.strOr("id", "")
	existing, err := d.queryRow(r.Context(),
		"SELECT token FROM campaign_report_shares WHERE campaign_id = $1 AND revoked_at IS NULL LIMIT 1", campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if existing != nil {
		token := existing.strOr("token", "")
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "share_path": "/berita-acara/" + token})
		return
	}
	token := randomToken()
	if err := d.exec(r.Context(),
		"INSERT INTO campaign_report_shares (id, campaign_id, brand_id, token, created_by) VALUES ($1, $2, $3, $4, $5)",
		uuid.NewString(), campaignID, brandRow.ID, token, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.audit(r.Context(), brandRow.ID, user.ID, "campaign.report_shared", "campaign", campaignID)
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "share_path": "/berita-acara/" + token})
}

// ---------- validasi kecil ----------

func requiredTrimmedString(body map[string]any, key string, min, max int, _ []string) (string, bool, error) {
	raw, ok := body[key]
	if !ok || raw == nil {
		return "", false, errUnprocessable("Required")
	}
	s, isStr := raw.(string)
	if !isStr {
		return "", false, errUnprocessable("Expected string, received number")
	}
	trimmed := strings.TrimSpace(s)
	if len([]rune(trimmed)) < min {
		return "", false, errUnprocessable("String must contain at least " + itoa(min) + " character(s)")
	}
	if len([]rune(trimmed)) > max {
		return "", false, errUnprocessable("String must contain at most " + itoa(max) + " character(s)")
	}
	return trimmed, true, nil
}

func optionalString(body map[string]any, key string, max int) (any, error) {
	raw, ok := body[key]
	if !ok || raw == nil {
		return nil, nil
	}
	s, isStr := raw.(string)
	if !isStr {
		return nil, errUnprocessable("Expected string, received number")
	}
	if len([]rune(s)) > max {
		return nil, errUnprocessable("String must contain at most " + itoa(max) + " character(s)")
	}
	return s, nil
}

// optionalDatetime: "" | null → nil; tidak valid → error zod datetime.
func optionalDatetime(body map[string]any, key string) (any, error) {
	raw, ok := body[key]
	if !ok || raw == nil {
		return nil, nil
	}
	s, isStr := raw.(string)
	if !isStr {
		return nil, errUnprocessable("Invalid datetime")
	}
	if s == "" {
		return nil, errUnprocessable("Invalid datetime")
	}
	t, ok := parseISO(s)
	if !ok {
		return nil, errUnprocessable("Invalid datetime")
	}
	return t, nil
}

func toFloat(raw any) (float64, bool) {
	switch n := raw.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		if f, err := parseFloat(n); err == nil {
			return f, true
		}
	}
	return 0, false
}

func intParamDef(raw string, def int) int {
	if raw == "" {
		return def
	}
	n, err := parseIntSafe(raw)
	if err != nil {
		return def
	}
	return n
}

func randomToken() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "") + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
}
