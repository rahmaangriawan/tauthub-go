// Modul operations platform: port app/v1/platform/operations/** (32 route.ts).
// Guard, validasi zod, SQL, bentuk JSON, dan efek samping (audit/notifikasi/job)
// disalin dari app lama; hanya placeholder `?` → `$n`.
package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	emailqueue "github.com/rahmaangriawan/tauthub-go/internal/email"
)

// ---------- access ----------

// OperationsAccess: GET /v1/platform/operations/access
func (d *Deps) OperationsAccess(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"platform_admin": true})
}

// ---------- summary ----------

// OperationsSummary: GET /v1/platform/operations/summary
func (d *Deps) OperationsSummary(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	now := time.Now().UTC()
	day := now.Add(-24 * time.Hour)
	since30d := now.Add(-30 * 24 * time.Hour)

	calls24h := d.opCount(ctx, "SELECT COUNT(*) AS count FROM model_usage_events WHERE created_at >= $1", day)
	failed24h := d.opCount(ctx, "SELECT COUNT(*) AS count FROM model_usage_events WHERE created_at >= $1 AND success = FALSE", day)
	tokens30d := 0
	if row, err := d.queryRow(ctx,
		"SELECT COALESCE(SUM(total_tokens), 0) AS total FROM model_usage_events WHERE created_at >= $1", since30d); err == nil && row != nil {
		tokens30d = int(row.numOr("total"))
	}
	runningBatches := d.opCount(ctx, "SELECT COUNT(*) AS count FROM content_batches WHERE status = 'running'")
	deadLetterItems := d.opCount(ctx, "SELECT COUNT(*) AS count FROM content_batch_items WHERE status = 'dead_letter'")

	ecosystem := map[string]any{
		"total_users": 0, "client_users": 0, "partner_users": 0, "total_brands": 0,
		"total_media": 0, "active_media": 0, "total_publications": 0, "published_publications": 0,
		"total_agent_runs": 0, "agent_runs_done": 0, "gatekeeper_passed": 0, "total_gatekeeper": 0,
	}
	recentPublications := []rowMap{}
	recentAgentRuns := []rowMap{}
	if eco, err := d.queryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM users) as total_users,
		  (SELECT count(*) FROM users WHERE role = 'client') as client_users,
		  (SELECT count(*) FROM users WHERE role = 'partner') as partner_users,
		  (SELECT count(*) FROM brands) as total_brands,
		  (SELECT count(*) FROM media) as total_media,
		  (SELECT count(*) FROM media WHERE status = 'active') as active_media,
		  (SELECT count(*) FROM publications) as total_publications,
		  (SELECT count(*) FROM publications WHERE status = 'published') as published_publications,
		  (SELECT count(*) FROM taut_agent_runs) as total_agent_runs,
		  (SELECT count(*) FROM taut_agent_runs WHERE status = 'done') as agent_runs_done,
		  (SELECT count(*) FROM taut_gatekeeper_audits WHERE overall_verdict = 'passed') as gatekeeper_passed,
		  (SELECT count(*) FROM taut_gatekeeper_audits) as total_gatekeeper`); err == nil && eco != nil {
		e := ptRow(ptNormalizeMap(eco))
		ecosystem = map[string]any{
			"total_users": e.intOr("total_users", 0), "client_users": e.intOr("client_users", 0),
			"partner_users": e.intOr("partner_users", 0), "total_brands": e.intOr("total_brands", 0),
			"total_media": e.intOr("total_media", 0), "active_media": e.intOr("active_media", 0),
			"total_publications": e.intOr("total_publications", 0), "published_publications": e.intOr("published_publications", 0),
			"total_agent_runs": e.intOr("total_agent_runs", 0), "agent_runs_done": e.intOr("agent_runs_done", 0),
			"gatekeeper_passed": e.intOr("gatekeeper_passed", 0), "total_gatekeeper": e.intOr("total_gatekeeper", 0),
		}
	}
	if rows, err := d.queryRows(ctx, `
		SELECT p.id, p.status, p.publish_status, p.published_url, p.published_at,
		       m.name as media_name, b.name as brand_name
		FROM publications p
		LEFT JOIN media m ON m.id = p.media_id
		LEFT JOIN brands b ON b.id = p.brand_id
		ORDER BY p.published_at DESC NULLS LAST
		LIMIT 5`); err == nil {
		recentPublications = ptRows(rows)
	}
	if rows, err := d.queryRows(ctx, `
		SELECT tar.id, tar.agent, tar.status, tar.model, tar.duration_ms, tar.created_at,
		       c.name as campaign_name
		FROM taut_agent_runs tar
		LEFT JOIN campaigns c ON c.id = tar.campaign_id
		ORDER BY tar.created_at DESC
		LIMIT 5`); err == nil {
		recentAgentRuns = ptRows(rows)
	}

	provider, configured := d.opProviderInfo(ctx)
	disabled := d.opGenerationDisabled(ctx)

	alerts := []string{}
	if disabled {
		alerts = append(alerts, "Generation global sedang dihentikan")
	}
	if calls24h > 0 && float64(failed24h)/float64(calls24h) >= 0.1 {
		alerts = append(alerts, "Error provider 24 jam melewati 10%")
	}
	if deadLetterItems > 0 {
		alerts = append(alerts, fmt.Sprintf("%d item berada di dead letter", deadLetterItems))
	}
	// Satu peringatan agregat untuk kegagalan queue 24 jam — detailnya di tab
	// "Queue gagal"; jangan satu alert per kegagalan.
	if qRow, qErr := d.queryRow(ctx,
		"SELECT COUNT(*) AS failures, COUNT(DISTINCT kind) AS kinds FROM jobs WHERE status = 'failed' AND updated_at > $1", day); qErr == nil && qRow != nil {
		failures := int(qRow.numOr("failures"))
		kinds := int(qRow.numOr("kinds"))
		if failures > 0 {
			alerts = append(alerts, fmt.Sprintf("%d kegagalan queue dalam 24 jam (%d jenis job) — lihat tab Queue gagal", failures, kinds))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"generation_disabled": disabled, "provider": provider, "provider_configured": configured,
		"circuit": "closed", "calls_24h": calls24h, "failed_calls_24h": failed24h,
		"tokens_30d": tokens30d, "running_batches": runningBatches, "dead_letter_items": deadLetterItems,
		"alerts": alerts, "ecosystem": ecosystem,
		"recent_publications": recentPublications, "recent_agent_runs": recentAgentRuns,
	})
}

// ---------- audit + audit-log ----------

// OperationsAudit: GET /v1/platform/operations/audit (alias audit-log).
func (d *Deps) OperationsAudit(w http.ResponseWriter, r *http.Request) {
	d.operationsAuditLog(w, r)
}

// OperationsAuditLog: GET /v1/platform/operations/audit-log
func (d *Deps) OperationsAuditLog(w http.ResponseWriter, r *http.Request) {
	d.operationsAuditLog(w, r)
}

func (d *Deps) operationsAuditLog(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	filters := []string{}
	values := []any{}
	if brandID := strings.TrimSpace(q.Get("brand_id")); brandID != "" {
		values = append(values, brandID)
		filters = append(filters, "a.brand_id = $"+ptItoa(len(values)))
	}
	if action := q.Get("action"); action != "" {
		values = append(values, action+"%")
		filters = append(filters, "a.action LIKE $"+ptItoa(len(values)))
	}
	if search := q.Get("search"); search != "" {
		like := "%" + search + "%"
		values = append(values, like, like, like)
		base := len(values) - 2
		filters = append(filters, "(a.action LIKE $"+ptItoa(base)+" OR a.target_type LIKE $"+ptItoa(base+1)+" OR u.email LIKE $"+ptItoa(base+2)+")")
	}
	where := ""
	if len(filters) > 0 {
		where = "WHERE " + strings.Join(filters, " AND ")
	}
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	limit := clampInt(q.Get("limit"), 50, 1, 100)

	total := 0
	if row, err := d.queryRow(ctx,
		"SELECT COUNT(*) AS total FROM audit_logs a LEFT JOIN users u ON u.id = a.actor_user_id "+where, values...); err == nil && row != nil {
		total = row.intOr("total", 0)
	}
	args := append([]any{}, values...)
	args = append(args, limit, offset)
	items, err := d.queryRows(ctx,
		`SELECT a.id, a.brand_id, b.name AS brand_name, a.actor_user_id, u.email AS actor_email,
		        a.action, a.target_type, a.target_id, a.created_at
		 FROM audit_logs a
		 LEFT JOIN users u ON u.id = a.actor_user_id
		 LEFT JOIN brands b ON b.id = a.brand_id `+where+
			" ORDER BY a.created_at DESC LIMIT $"+ptItoa(len(values)+1)+" OFFSET $"+ptItoa(len(values)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": ptRows(items)})
}

// ---------- brands ----------

// OperationsBrands: GET /v1/platform/operations/brands
func (d *Deps) OperationsBrands(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	base := `
	  SELECT b.id, b.name, b.slug, u.email AS owner_email,
	    (SELECT COUNT(*) FROM brand_memberships bm WHERE bm.brand_id = b.id) AS member_count,
	    COALESCE(q.plan_code, 'starter') AS plan_code,
	    COALESCE(q.generation_enabled, TRUE) AS generation_enabled,
	    b.created_at
	  FROM brands b
	  JOIN users u ON u.id = b.created_by
	  LEFT JOIN brand_ai_quotas q ON q.brand_id = b.id
	  WHERE b.deleted_at IS NULL`
	var rows []rowMap
	var err error
	if search != "" {
		like := "%" + search + "%"
		rows, err = d.queryRows(r.Context(),
			base+" AND (b.name LIKE $1 OR b.slug LIKE $2 OR u.email LIKE $3) ORDER BY b.created_at DESC LIMIT 200",
			like, like, like)
	} else {
		rows, err = d.queryRows(r.Context(), base+" ORDER BY b.created_at DESC LIMIT 200")
	}
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, ptRows(rows))
}

// OperationsBrandCampaigns: GET /v1/platform/operations/brands/{brandId}/campaigns
func (d *Deps) OperationsBrandCampaigns(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT c.id, c.name, c.status, c.budget, c.created_at,
		        (SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status <> 'cancelled') AS publication_count
		 FROM campaigns c
		 WHERE c.brand_id = $1
		 ORDER BY c.created_at DESC LIMIT 200`, chiParam(r, "brandId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	campaigns := make([]rowMap, 0, len(rows))
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		budget := 0
		if row.has("budget") {
			budget = int(row.numOr("budget"))
		}
		campaigns = append(campaigns, map[string]any{
			"id": row.strOr("id", ""), "name": row.strOr("name", ""), "status": row.strOr("status", ""),
			"budget": budget, "publication_count": row.intOr("publication_count", 0),
			"created_at": row.anyOrNil("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"campaigns": campaigns, "total": len(campaigns)})
}

// ---------- brand CMS connections ----------

const opConnectionColumns = "id, brand_id, partner_id, website_id, provider, name, base_url, username, status, last_test_status, last_test_message, created_at, updated_at"

// opScopedConnection: parity scopedConnection → 404 "CMS connection not found".
func (d *Deps) opScopedConnection(ctx context.Context, brandID, connectionID string) (map[string]any, error) {
	row, err := d.queryRow(ctx,
		"SELECT "+opConnectionColumns+" FROM cms_connections WHERE id = $1 AND brand_id = $2 LIMIT 1",
		connectionID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("CMS connection not found")
	}
	return row, nil
}

// OperationsBrandConnections: GET/POST /v1/platform/operations/brands/{brandId}/connections
func (d *Deps) OperationsBrandConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		d.operationsBrandConnectionsList(w, r)
		return
	}
	d.operationsBrandConnectionsCreate(w, r)
}

func (d *Deps) operationsBrandConnectionsList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(),
		"SELECT "+opConnectionColumns+" FROM cms_connections WHERE brand_id = $1 ORDER BY created_at DESC",
		chiParam(r, "brandId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	connections := make([]rowMap, 0, len(rows))
	for _, raw := range rows {
		connections = append(connections, opConnectionJSON(raw))
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": connections, "total": len(connections)})
}

func (d *Deps) operationsBrandConnectionsCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	brandID := chiParam(r, "brandId")
	ctx := r.Context()
	body := decodeJSON(r)

	name, namePresent, nameString := opGetString(body, "name", false)
	if !namePresent {
		writeErr(w, d, errUnprocessable("Invalid input: expected string, received undefined"))
		return
	}
	if !nameString {
		writeErr(w, d, errUnprocessable("Invalid input: expected string, received "+opJSONTypeName(body["name"])))
		return
	}
	if len(name) < 2 {
		writeErr(w, d, errUnprocessable(opStrTooSmall(2)))
		return
	}
	if len(name) > 120 {
		writeErr(w, d, errUnprocessable(opStrTooBig(120)))
		return
	}
	provider := "wordpress"
	if p, hasP, isStr := opGetString(body, "provider", false); isStr {
		if p != "wordpress" && p != "webhook" {
			writeErr(w, d, errUnprocessable("Invalid enum value. Expected 'wordpress' | 'webhook'"))
			return
		}
		provider = p
	} else if hasP {
		writeErr(w, d, errUnprocessable("Invalid enum value. Expected 'wordpress' | 'webhook'"))
		return
	}
	baseURL, basePresent, baseString := opGetString(body, "base_url", false)
	if !basePresent {
		writeErr(w, d, errUnprocessable("Invalid input: expected string, received undefined"))
		return
	}
	if !baseString {
		writeErr(w, d, errUnprocessable("Invalid input: expected string, received "+opJSONTypeName(body["base_url"])))
		return
	}
	if len(baseURL) < 8 {
		writeErr(w, d, errUnprocessable(opStrTooSmall(8)))
		return
	}
	if len(baseURL) > 500 {
		writeErr(w, d, errUnprocessable(opStrTooBig(500)))
		return
	}
	username, hasUsername, usernameIsStr := opGetString(body, "username", false)
	var usernameArg any
	if hasUsername && username != "" {
		if !usernameIsStr {
			writeErr(w, d, errUnprocessable("Invalid input: expected string, received "+opJSONTypeName(body["username"])))
			return
		}
		if len(username) > 160 {
			writeErr(w, d, errUnprocessable(opStrTooBig(160)))
			return
		}
		usernameArg = nullIfEmpty(strings.TrimSpace(username))
	}
	applicationPassword, hasAppPw, appPwIsStr := opGetString(body, "application_password", false)
	secret, hasSecret, secretIsStr := opGetString(body, "secret", false)
	websiteID, hasWebsite, websiteIsValid := opGetString(body, "website_id", false)
	if hasWebsite && websiteID != "" && !websiteIsValid {
		writeErr(w, d, errUnprocessable("Invalid input: expected string, received "+opJSONTypeName(body["website_id"])))
		return
	}
	if provider == "wordpress" {
		if usernameArg == nil {
			writeErr(w, d, errUnprocessable("username is required for WordPress connections"))
			return
		}
		if !hasAppPw || applicationPassword == "" {
			writeErr(w, d, errUnprocessable("application_password is required for WordPress connections"))
			return
		}
		if !appPwIsStr {
			writeErr(w, d, errUnprocessable("Invalid input: expected string, received "+opJSONTypeName(body["application_password"])))
			return
		}
		if len(applicationPassword) > 500 {
			writeErr(w, d, errUnprocessable(opStrTooBig(500)))
			return
		}
	} else {
		if !hasSecret || secret == "" {
			writeErr(w, d, errUnprocessable("secret is required for webhook connections"))
			return
		}
		if !secretIsStr {
			writeErr(w, d, errUnprocessable("Invalid input: expected string, received "+opJSONTypeName(body["secret"])))
			return
		}
		if len(secret) < 16 {
			writeErr(w, d, errUnprocessable(opStrTooSmall(16)))
			return
		}
		if len(secret) > 500 {
			writeErr(w, d, errUnprocessable(opStrTooBig(500)))
			return
		}
	}

	if hasWebsite && websiteID != "" {
		site, err := d.queryRow(ctx, "SELECT id FROM websites WHERE id = $1 AND brand_id = $2", websiteID, brandID)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		if site == nil {
			writeErr(w, d, errNotFound("Website not found"))
			return
		}
	}
	if provider == "wordpress" {
		active, err := d.queryRow(ctx,
			"SELECT id FROM cms_connections WHERE brand_id = $1 AND provider = 'wordpress' AND status = 'active' LIMIT 1", brandID)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		if active != nil {
			writeErr(w, d, errConflict("Brand already has an active WordPress connection"))
			return
		}
	}

	connectionID := uuid.NewString()
	plain := ""
	if provider == "wordpress" {
		plain = opStripSpaces(applicationPassword)
	} else {
		plain = secret
	}
	sealed, sealErr := d.sealV3ForEntity(plain, connectionID)
	if sealErr != nil {
		writeErr(w, d, sealErr)
		return
	}
	if err := d.exec(ctx,
		"INSERT INTO cms_connections (id, brand_id, website_id, provider, name, base_url, username, sealed_secret, enc_key_ver, status, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, 'active', $9)",
		connectionID, brandID, ptNullableArgStr(websiteID, hasWebsite && websiteID != ""), provider, name,
		strings.TrimRight(baseURL, "/"), usernameArg, sealed, user.ID); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errConflict("CMS connection name already exists"))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.opAuditBrand(ctx, brandID, user.ID, "cms.connection.created", "cms_connection", connectionID)
	created, err := d.queryRow(ctx,
		"SELECT "+opConnectionColumns+" FROM cms_connections WHERE id = $1", connectionID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, opConnectionJSON(created))
}

// opStripSpaces: parity replace(/\s+/g, "") pada application password.
func opStripSpaces(s string) string {
	var b strings.Builder
	for _, ch := range s {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\v' || ch == '\f' {
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// OperationsBrandConnectionDelete: DELETE /v1/platform/operations/brands/{brandId}/connections/{connectionId}
func (d *Deps) OperationsBrandConnectionDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	brandID := chiParam(r, "brandId")
	connectionID := chiParam(r, "connectionId")
	ctx := r.Context()
	if _, err := d.opScopedConnection(ctx, brandID, connectionID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(ctx,
		"UPDATE cms_connections SET status = 'revoked', name = CONCAT(name, ' (revoked ', LEFT(id, 8), ')') WHERE id = $1 AND brand_id = $2",
		connectionID, brandID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.opAuditBrand(ctx, brandID, user.ID, "cms.connection.revoked", "cms_connection", connectionID)
	refreshed, err := d.opScopedConnection(ctx, brandID, connectionID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, opConnectionJSON(refreshed))
}

// OperationsBrandConnectionTest: POST /v1/platform/operations/brands/{brandId}/connections/{connectionId}/test
func (d *Deps) OperationsBrandConnectionTest(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	brandID := chiParam(r, "brandId")
	connectionID := chiParam(r, "connectionId")
	ctx := r.Context()
	if _, err := d.opScopedConnection(ctx, brandID, connectionID); err != nil {
		writeErr(w, d, err)
		return
	}
	connection, err := d.queryRow(ctx, "SELECT * FROM cms_connections WHERE id = $1 AND brand_id = $2 LIMIT 1", connectionID, brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if connection == nil {
		writeErr(w, d, errNotFound("CMS connection not found"))
		return
	}
	status, message := d.opTestConnection(ctx, ptRow(ptNormalizeMap(connection)))
	if err := d.exec(ctx,
		"UPDATE cms_connections SET last_test_status = $1, last_test_message = $2 WHERE id = $3",
		status, message, connectionID); err != nil {
		writeErr(w, d, err)
		return
	}
	code := http.StatusOK
	if status != "ok" {
		code = http.StatusUnprocessableEntity
	}
	writeJSON(w, code, map[string]any{"status": status, "message": message})
}

// ---------- brand content settings ----------

var opPointOfViewValues = []string{"first_person", "second_person", "third_person"}

// OperationsBrandContentSettings: GET/PUT /v1/platform/operations/brands/{brandId}/content-settings
func (d *Deps) OperationsBrandContentSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		d.operationsBrandContentSettingsGet(w, r)
		return
	}
	d.operationsBrandContentSettingsPut(w, r)
}

func (d *Deps) operationsBrandContentSettingsGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	brandID := chiParam(r, "brandId")
	ctx := r.Context()
	profile, err := d.queryRow(ctx,
		"SELECT positioning, value_proposition, target_audience, products_services, differentiation, default_cta, default_target_words FROM brand_profiles WHERE brand_id = $1", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	voice, err := d.queryRow(ctx,
		"SELECT personality, formality, enthusiasm, directness, humor, point_of_view, preferred_terms, banned_terms, cta_style FROM voice_profiles WHERE brand_id = $1", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	rules, err := d.queryRow(ctx,
		"SELECT required_rules, forbidden_rules, required_disclaimer, citation_policy FROM editorial_rule_sets WHERE brand_id = $1", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	prompts, err := d.queryRows(ctx,
		"SELECT id, version, status, content, change_note, created_at FROM master_prompt_versions WHERE brand_id = $1 ORDER BY version DESC", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profile": ptNullableRow(profile), "voice": ptNullableRow(voice),
		"editorial_rules": ptNullableRow(rules), "prompts": ptRows(prompts),
	})
}

// ptNullableRow: baris map atau nil (parity `const [profile] = ...`).
func ptNullableRow(row map[string]any) any {
	if row == nil {
		return nil
	}
	return map[string]any(ptRow(ptNormalizeMap(row)))
}

func (d *Deps) operationsBrandContentSettingsPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	brandID := chiParam(r, "brandId")
	ctx := r.Context()
	body := decodeJSON(r)

	profile, hasProfile, profileOk := opGetObj(body, "profile")
	voice, hasVoice, voiceOk := opGetObj(body, "voice")
	rules, hasRules, rulesOk := opGetObj(body, "editorial_rules")
	if hasProfile && !profileOk {
		writeErr(w, d, errUnprocessable("Invalid input: expected object, received "+opReceivedName(body, "profile")))
		return
	}
	if hasVoice && !voiceOk {
		writeErr(w, d, errUnprocessable("Invalid input: expected object, received "+opReceivedName(body, "voice")))
		return
	}
	if hasRules && !rulesOk {
		writeErr(w, d, errUnprocessable("Invalid input: expected object, received "+opReceivedName(body, "editorial_rules")))
		return
	}

	if hasVoice {
		personality, _, _ := opGetArr3(voice, "personality")
		preferred, _, _ := opGetArr3(voice, "preferred_terms")
		banned, _, _ := opGetArr3(voice, "banned_terms")
		formality := 3
		if v, present, valid := opGetNum(voice, "formality"); present {
			if !valid {
				writeErr(w, d, errUnprocessable("Invalid input: expected number, received "+opJSONTypeName(voice["formality"])))
				return
			}
			if v < 1 {
				writeErr(w, d, errUnprocessable(ptZNumberTooSmall(1)))
				return
			}
			if v > 5 {
				writeErr(w, d, errUnprocessable(ptZNumberTooBig(5)))
				return
			}
			formality = int(v)
		}
		enthusiasm := 3
		if v, present, valid := opGetNum(voice, "enthusiasm"); present {
			if !valid {
				writeErr(w, d, errUnprocessable("Invalid input: expected number, received "+opJSONTypeName(voice["enthusiasm"])))
				return
			}
			if v < 1 {
				writeErr(w, d, errUnprocessable(ptZNumberTooSmall(1)))
				return
			}
			if v > 5 {
				writeErr(w, d, errUnprocessable(ptZNumberTooBig(5)))
				return
			}
			enthusiasm = int(v)
		}
		directness := 3
		if v, present, valid := opGetNum(voice, "directness"); present {
			if !valid {
				writeErr(w, d, errUnprocessable("Invalid input: expected number, received "+opJSONTypeName(voice["directness"])))
				return
			}
			if v < 1 {
				writeErr(w, d, errUnprocessable(ptZNumberTooSmall(1)))
				return
			}
			if v > 5 {
				writeErr(w, d, errUnprocessable(ptZNumberTooBig(5)))
				return
			}
			directness = int(v)
		}
		humor := 1
		if v, present, valid := opGetNum(voice, "humor"); present {
			if !valid {
				writeErr(w, d, errUnprocessable("Invalid input: expected number, received "+opJSONTypeName(voice["humor"])))
				return
			}
			if v < 1 {
				writeErr(w, d, errUnprocessable(ptZNumberTooSmall(1)))
				return
			}
			if v > 5 {
				writeErr(w, d, errUnprocessable(ptZNumberTooBig(5)))
				return
			}
			humor = int(v)
		}
		pov := "second_person"
		if v, present, valid := opGetString(voice, "point_of_view", false); present && v != "" {
			if !valid || !ptContains(opPointOfViewValues, v) {
				writeErr(w, d, errUnprocessable("Invalid enum value. Expected "+opEnumExpected(opPointOfViewValues)))
				return
			}
			pov = v
		}
		ctaStyle, _, _ := opGetString(voice, "cta_style", false)
		if len(ctaStyle) > 500 {
			writeErr(w, d, errUnprocessable(opStrTooBig(500)))
			return
		}
		if err := d.exec(ctx,
			`INSERT INTO voice_profiles (brand_id, personality, formality, enthusiasm, directness, humor, point_of_view, preferred_terms, banned_terms, cta_style)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			 ON CONFLICT (brand_id) DO UPDATE SET personality = EXCLUDED.personality, formality = EXCLUDED.formality, enthusiasm = EXCLUDED.enthusiasm, directness = EXCLUDED.directness, humor = EXCLUDED.humor, point_of_view = EXCLUDED.point_of_view, preferred_terms = EXCLUDED.preferred_terms, banned_terms = EXCLUDED.banned_terms, cta_style = EXCLUDED.cta_style`,
			brandID, opJSONStrings(personality), formality, enthusiasm, directness, humor, pov,
			opJSONStrings(preferred), opJSONStrings(banned), ctaStyle); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	if hasRules {
		required, _, _ := opGetArr3(rules, "required_rules")
		forbidden, _, _ := opGetArr3(rules, "forbidden_rules")
		disclaimer, _, _ := opGetString(rules, "required_disclaimer", false)
		citation, _, _ := opGetString(rules, "citation_policy", false)
		if len(disclaimer) > 2000 {
			writeErr(w, d, errUnprocessable(opStrTooBig(2000)))
			return
		}
		if len(citation) > 500 {
			writeErr(w, d, errUnprocessable(opStrTooBig(500)))
			return
		}
		if err := d.exec(ctx,
			`INSERT INTO editorial_rule_sets (brand_id, required_rules, forbidden_rules, required_disclaimer, citation_policy)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (brand_id) DO UPDATE SET required_rules = EXCLUDED.required_rules, forbidden_rules = EXCLUDED.forbidden_rules, required_disclaimer = EXCLUDED.required_disclaimer, citation_policy = EXCLUDED.citation_policy`,
			brandID, opJSONStrings(required), opJSONStrings(forbidden), disclaimer, citation); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	if hasProfile {
		positioning, _, _ := opGetString(profile, "positioning", false)
		valueProp, _, _ := opGetString(profile, "value_proposition", false)
		audience, _, _ := opGetString(profile, "target_audience", false)
		products, _, _ := opGetString(profile, "products_services", false)
		differentiation, _, _ := opGetString(profile, "differentiation", false)
		defaultCTA, _, _ := opGetString(profile, "default_cta", false)
		for _, limitCheck := range []struct {
			value string
			max   int
		}{
			{positioning, 2000}, {valueProp, 5000}, {audience, 2000},
			{products, 5000}, {differentiation, 5000}, {defaultCTA, 500},
		} {
			if len(limitCheck.value) > limitCheck.max {
				writeErr(w, d, errUnprocessable(opStrTooBig(limitCheck.max)))
				return
			}
		}
		words := -1
		if v, present, valid := opGetNum(profile, "default_target_words"); present && valid {
			words = ptRoundInt(v)
			if words < 300 {
				words = 300
			}
			if words > 600 {
				words = 600
			}
		}
		if words >= 0 {
			if err := d.exec(ctx,
				`INSERT INTO brand_profiles (brand_id, positioning, value_proposition, target_audience, products_services, differentiation, default_cta, default_target_words)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				 ON CONFLICT (brand_id) DO UPDATE SET positioning = EXCLUDED.positioning, value_proposition = EXCLUDED.value_proposition, target_audience = EXCLUDED.target_audience, products_services = EXCLUDED.products_services, differentiation = EXCLUDED.differentiation, default_cta = EXCLUDED.default_cta, default_target_words = EXCLUDED.default_target_words`,
				brandID, positioning, valueProp, audience, products, differentiation, defaultCTA, words); err != nil {
				writeErr(w, d, err)
				return
			}
		} else {
			if err := d.exec(ctx,
				`INSERT INTO brand_profiles (brand_id, positioning, value_proposition, target_audience, products_services, differentiation, default_cta)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)
				 ON CONFLICT (brand_id) DO UPDATE SET positioning = EXCLUDED.positioning, value_proposition = EXCLUDED.value_proposition, target_audience = EXCLUDED.target_audience, products_services = EXCLUDED.products_services, differentiation = EXCLUDED.differentiation, default_cta = EXCLUDED.default_cta`,
				brandID, positioning, valueProp, audience, products, differentiation, defaultCTA); err != nil {
				writeErr(w, d, err)
				return
			}
		}
	}
	d.opAuditBrand(ctx, brandID, user.ID, "platform.brand.content_settings_updated", "brand", brandID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- credit adjustment ----------

// OperationsCreditAdjustment: POST /v1/platform/operations/credit-adjustment
func (d *Deps) OperationsCreditAdjustment(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body := decodeJSON(r)
	brandID, hasBrand, brandIsStr := opGetString(body, "brand_id", true)
	tc, hasTC, tcIsNum := opGetNum(body, "tc")
	reason, hasReason, reasonIsStr := opGetString(body, "reason", true)
	if !brandIsStr {
		writeErr(w, d, errUnprocessable("Data tidak valid."))
		return
	}
	if !hasTC || !tcIsNum || !opIsInt(tc) || tc == 0 {
		writeErr(w, d, errUnprocessable("Jumlah saldo harus bilangan bulat dan tidak boleh 0."))
		return
	}
	// Nilai negatif = kurangi saldo. Nominal memakai rupiah penuh (satuan basis
	// data sudah 1 unit = Rp1), batas per penyesuaian Rp10 miliar — setara batas
	// lama 10.000.000 TC sebelum konversi satuan.
	if tc > 10000000000 || tc < -10000000000 {
		writeErr(w, d, errUnprocessable("Nominal maksimal Rp10.000.000.000 per penyesuaian."))
		return
	}
	if !reasonIsStr {
		writeErr(w, d, errUnprocessable("Data tidak valid."))
		return
	}
	if !hasBrand || brandID == "" {
		writeErr(w, d, errUnprocessable("Data tidak valid."))
		return
	}
	if !hasReason || len(reason) < 3 || len(reason) > 500 {
		writeErr(w, d, errUnprocessable("Data tidak valid."))
		return
	}
	brand, err := d.queryRow(ctx,
		"SELECT id, name FROM brands WHERE id = $1 AND deleted_at IS NULL LIMIT 1", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if brand == nil {
		writeErr(w, d, errNotFound("Brand tidak ditemukan."))
		return
	}
	tcInt := int(tc)
	// Pengurangan harus dijawab dengan pesan yang bisa ditindaklanjuti admin,
	// bukan error server dari constraint saldo non-negatif di database.
	if tcInt < 0 {
		available := 0
		if wRow, wErr := d.queryRow(ctx,
			"SELECT COALESCE(available_tc, 0) AS available_tc FROM credit_wallets WHERE brand_id = $1 LIMIT 1", brandID); wErr == nil && wRow != nil {
			available = wRow.intOr("available_tc", 0)
		}
		if available+tcInt < 0 {
			writeErr(w, d, errUnprocessable(fmt.Sprintf("Saldo tidak cukup: tersedia %d, tidak bisa dikurangi %d.", available, -tcInt)))
			return
		}
	}
	if err := d.opAdjustCredit(ctx, brandID, tcInt, reason, user.ID, ""); err != nil {
		writeErr(w, d, err)
		return
	}
	d.opAuditBrand(ctx, brandID, user.ID, "finance.credit.manual_topup", "brand", brandID)
	summary := d.opWalletSummary(ctx, brandID)
	writeJSON(w, http.StatusOK, map[string]any{
		"brand_id": brandID, "brand_name": ptNullableStr(ptRow(ptNormalizeMap(brand)), "name"),
		"tc_added": tcInt, "reason": reason,
		"wallet": map[string]any{
			"available_tc": summary["available_tc"], "reserved_tc": summary["reserved_tc"],
			"captured_tc": summary["captured_tc"], "total_tc": summary["total_tc"],
		},
	})
}

// ---------- credit reports ----------

// OperationsCreditReports: GET /v1/platform/operations/credit-reports
func (d *Deps) OperationsCreditReports(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	status := q.Get("status")
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	params := []any{}
	if status != "" && ptContains([]string{"pending", "approved", "rejected"}, status) {
		params = append(params, status)
		where = append(where, "r.status = $"+ptItoa(len(params)))
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	total := 0
	if row, err := d.queryRow(ctx, "SELECT COUNT(*) AS count FROM credit_topup_reports r "+clause, params...); err == nil && row != nil {
		total = row.intOr("count", 0)
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(ctx,
		`SELECT r.id, r.brand_id, r.user_id, r.claimed_tc, r.amount_idr, r.payment_reference, r.description,
		        r.status, r.review_note, r.reviewed_by, r.reviewed_at, r.created_at,
		        b.name AS brand_name,
		        u.email AS user_email, u.display_name AS user_display_name,
		        COALESCE(w.available_tc, 0) AS available_tc
		 FROM credit_topup_reports r
		 JOIN brands b ON b.id = r.brand_id
		 JOIN users u ON u.id = r.user_id
		 LEFT JOIN credit_wallets w ON w.brand_id = r.brand_id
		 `+clause+" ORDER BY r.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	reports := make([]rowMap, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		var amountIDR any
		if row.has("amount_idr") {
			amountIDR = int(row.numOr("amount_idr"))
		}
		reports = append(reports, map[string]any{
			"id": row.anyOrNil("id"), "brand_id": row.anyOrNil("brand_id"),
			"brand_name": ptNullableStr(row, "brand_name"), "user_id": row.anyOrNil("user_id"),
			"user_email":        ptNullableStr(row, "user_email"),
			"user_display_name": ptNullableStr(row, "user_display_name"),
			"claimed_tc":        row.intOr("claimed_tc", 0), "amount_idr": amountIDR,
			"payment_reference": ptNullableStr(row, "payment_reference"),
			"description":       ptNullableStr(row, "description"), "status": row.anyOrNil("status"),
			"review_note": ptNullableStr(row, "review_note"), "reviewed_at": row.anyOrNil("reviewed_at"),
			"created_at": row.anyOrNil("created_at"), "available_tc": row.intOr("available_tc", 0),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"reports": reports, "total": total, "limit": limit, "offset": offset,
	})
}

// OperationsCreditReportReview: POST /v1/platform/operations/credit-reports/{reportId}/review
func (d *Deps) OperationsCreditReportReview(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	reportID := chiParam(r, "reportId")
	body := decodeJSON(r)
	action, _, actionValid := opGetString(body, "action", false)
	if !actionValid || (action != "approve" && action != "reject") {
		writeErr(w, d, errUnprocessable("Aksi tidak valid."))
		return
	}
	note, hasNote, noteIsStr := opGetString(body, "note", true)
	if hasNote && !noteIsStr {
		writeErr(w, d, errUnprocessable("Aksi tidak valid."))
		return
	}
	if len(note) > 500 {
		writeErr(w, d, errUnprocessable("Aksi tidak valid."))
		return
	}
	report, err := d.queryRow(ctx,
		"SELECT id, brand_id, user_id, claimed_tc, status FROM credit_topup_reports WHERE id = $1 LIMIT 1", reportID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if report == nil {
		writeErr(w, d, errNotFound("Laporan tidak ditemukan."))
		return
	}
	row := ptRow(ptNormalizeMap(report))
	if row.strOr("status", "") != "pending" {
		writeErr(w, d, errConflict("Laporan ini sudah diproses."))
		return
	}
	targetStatus := "rejected"
	if action == "approve" {
		targetStatus = "approved"
	}
	noteArg := opNullableArg(body, "note")
	affected, err := d.execRows(ctx,
		`UPDATE credit_topup_reports
		 SET status = $1, review_note = $2, reviewed_by = $3, reviewed_at = NOW()
		 WHERE id = $4 AND status = 'pending'`,
		targetStatus, noteArg, user.ID, reportID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if affected == 0 {
		writeErr(w, d, errConflict("Laporan ini sudah diproses."))
		return
	}
	brandID := row.strOr("brand_id", "")
	claimedTC := int(row.numOr("claimed_tc"))
	if action == "approve" {
		reason := fmt.Sprintf("Kompensasi topup gagal (laporan %s)", opFirstN(reportID, 8))
		if err := d.opAdjustCredit(ctx, brandID, claimedTC, reason, user.ID, "credit-report-approve:"+reportID); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	auditAction := "finance.credit.report_rejected"
	if action == "approve" {
		auditAction = "finance.credit.report_approved"
	}
	d.opAuditBrand(ctx, brandID, user.ID, auditAction, "credit_topup_report", reportID)

	tcAdded := 0
	var wallet any
	if action == "approve" {
		tcAdded = claimedTC
		summary := d.opWalletSummary(ctx, brandID)
		wallet = map[string]any{
			"available_tc": summary["available_tc"], "reserved_tc": summary["reserved_tc"],
			"captured_tc": summary["captured_tc"], "total_tc": summary["total_tc"],
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"report": map[string]any{
			"id": reportID, "status": targetStatus,
			"review_note": noteArg,
			"reviewed_at": opNowISO(),
		},
		"tc_added": tcAdded, "wallet": wallet,
	})
}

// opFirstN: parity id.slice(0, n).
func opFirstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------- dead letters ----------

// OperationsDeadLetters: GET /v1/platform/operations/dead-letters
func (d *Deps) OperationsDeadLetters(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT i.id, i.batch_id, cb.name AS batch_name, i.brand_id, b.name AS brand_name, i.row_number,
		        i.retry_count, i.error_category, i.error_message, i.updated_at
		 FROM content_batch_items i
		 JOIN content_batches cb ON cb.id = i.batch_id
		 JOIN brands b ON b.id = i.brand_id
		 WHERE i.status = 'dead_letter' ORDER BY i.updated_at DESC LIMIT 100`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, ptRows(rows))
}

// OperationsDeadLetterRetry: POST /v1/platform/operations/dead-letters/{id}/retry
func (d *Deps) OperationsDeadLetterRetry(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	id := chiParam(r, "id")
	item, err := d.queryRow(ctx,
		"SELECT id, brand_id, batch_id, retry_count FROM content_batch_items WHERE id = $1 AND status = 'dead_letter' LIMIT 1", id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if item == nil {
		writeErr(w, d, errNotFound("Dead-letter item not found"))
		return
	}
	row := ptRow(ptNormalizeMap(item))
	if int(row.numOr("retry_count")) >= 3 {
		writeErr(w, d, errConflict("Retry limit reached"))
		return
	}
	if err := d.exec(ctx,
		"UPDATE content_batch_items SET retry_count = retry_count + 1, status = 'pending', job_id = NULL, error_category = NULL, error_message = NULL WHERE id = $1", id); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(ctx, "UPDATE content_batches SET status = 'running' WHERE id = $1", row.strOr("batch_id", "")); err != nil {
		writeErr(w, d, err)
		return
	}
	d.opAuditBrand(ctx, row.strOr("brand_id", ""), user.ID, "platform.dead_letter.retried", "content_batch_item", id)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued"})
}

// ---------- errors ----------

// OperationsErrors: GET /v1/platform/operations/errors
func (d *Deps) OperationsErrors(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 120 {
		q = q[:120]
	}
	var like any
	if q != "" {
		like = "%" + q + "%"
	}
	pubFilter, pubParams := opLikeClause(like, "(b.name ILIKE %s OR m.name ILIKE %s OR d.title ILIKE %s OR c.name ILIKE %s)", 4)
	jobFilter, jobParams := opLikeClause(like, "(b.name ILIKE %s OR m.name ILIKE %s OR d.title ILIKE %s OR j.error_message ILIKE %s)", 4)
	rssFilter, rssParams := opLikeClause(like, "(f.name ILIKE %s OR p.name ILIKE %s OR m.name ILIKE %s OR f.install_error ILIKE %s)", 4)
	queueFilter, queueParams := opLikeClause(like, "(kind ILIKE %s OR last_error ILIKE %s)", 2)
	batchFilter, batchParams := opLikeClause(like, "(br.name ILIKE %s)", 1)

	publications, err := d.queryRows(ctx, `
		SELECT p.id, p.status, p.updated_at,
		       b.name AS brand_name, b.public_id AS brand_public_id, m.name AS media_name,
		       d.title AS document_title, c.name AS campaign_name,
		       (SELECT pa.error_message FROM publication_attempts pa WHERE pa.publication_id = p.id ORDER BY pa.attempt_number DESC LIMIT 1) AS error_message,
		       (SELECT pa.error_code FROM publication_attempts pa WHERE pa.publication_id = p.id ORDER BY pa.attempt_number DESC LIMIT 1) AS error_code
		FROM publications p
		JOIN brands b ON b.id = p.brand_id
		JOIN media m ON m.id = p.media_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN campaigns c ON c.id = p.campaign_id
		WHERE p.status IN ('failed', 'retrying') `+pubFilter+`
		ORDER BY p.updated_at DESC LIMIT 100`, pubParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	jobs, err := d.queryRows(ctx, `
		SELECT j.id, j.publication_id, j.status, j.error_message, j.error_code, j.attempts, j.updated_at,
		       b.name AS brand_name, m.name AS media_name, d.title AS document_title
		FROM publication_jobs j
		LEFT JOIN publications p ON p.id = j.publication_id
		LEFT JOIN brands b ON b.id = j.brand_id
		LEFT JOIN media m ON m.id = p.media_id
		LEFT JOIN content_documents d ON d.id = j.document_id
		WHERE j.status = 'failed' `+jobFilter+`
		ORDER BY j.updated_at DESC LIMIT 100`, jobParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// Peringatan non-fatal (mis. gambar gagal diunggah ke media mitra) — artikel
	// tetap terbit, tapi admin harus bisa melihatnya di sini.
	warnFilter, warnParams := opLikeClause(like, "(b.name ILIKE %s OR m.name ILIKE %s OR d.title ILIKE %s OR j.warning_message ILIKE %s)", 4)
	warnings, err := d.queryRows(ctx, `
		SELECT j.id, j.publication_id, j.status, j.warning_message, j.updated_at,
		       b.name AS brand_name, m.name AS media_name, d.title AS document_title
		FROM publication_jobs j
		LEFT JOIN publications p ON p.id = j.publication_id
		LEFT JOIN brands b ON b.id = j.brand_id
		LEFT JOIN media m ON m.id = p.media_id
		LEFT JOIN content_documents d ON d.id = j.document_id
		WHERE j.warning_message IS NOT NULL AND j.warning_message <> '' `+warnFilter+`
		ORDER BY j.updated_at DESC LIMIT 100`, warnParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	rss, err := d.queryRows(ctx, `
		SELECT f.id, f.name, f.install_status, f.install_error, f.updated_at,
		       p.name AS partner_name, m.name AS media_name
		FROM rss_feeds f
		LEFT JOIN partners p ON p.id = f.partner_id
		LEFT JOIN media m ON m.id = f.media_id
		WHERE f.install_status = 'failed' `+rssFilter+`
		ORDER BY f.updated_at DESC LIMIT 100`, rssParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	queue, err := d.queryRows(ctx, `
		SELECT id, kind, status, last_error, attempts, updated_at
		FROM jobs WHERE status = 'failed' `+queueFilter+`
		ORDER BY updated_at DESC LIMIT 100`, queueParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	variantBatches, err := d.queryRows(ctx, `
		SELECT b.id, b.mode, b.status, COALESCE(b.completed_at, b.created_at) AS updated_at,
		       br.name AS brand_name, br.public_id AS brand_public_id,
		       (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id) AS total,
		       (SELECT COUNT(*) FROM publication_variants v WHERE v.batch_id = b.id AND v.status = 'failed') AS failed,
		       (SELECT string_agg(m.name, ', ' ORDER BY v.created_at)
		        FROM publication_variants v JOIN media m ON m.id = v.media_id
		        WHERE v.batch_id = b.id AND v.status = 'failed') AS failed_media
		FROM publication_variant_batches b
		JOIN brands br ON br.id = b.brand_id
		WHERE b.status IN ('failed', 'partial') `+batchFilter+`
		ORDER BY COALESCE(b.completed_at, b.created_at) DESC LIMIT 50`, batchParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	pubsOut := make([]rowMap, 0, len(publications))
	for _, raw := range publications {
		row := ptRow(ptNormalizeMap(raw))
		errMsg := ptNullableStr(row, "error_message")
		var httpStatus any
		if s, okString := errMsg.(string); okString {
			httpStatus = opHTTPStatusFromMessage(s)
		}
		pubsOut = append(pubsOut, map[string]any{
			"id": row.anyOrNil("id"), "status": row.anyOrNil("status"),
			"brand_name": ptNullableStr(row, "brand_name"), "brand_public_id": ptNullableStr(row, "brand_public_id"),
			"media_name": ptNullableStr(row, "media_name"), "document_title": ptNullableStr(row, "document_title"),
			"campaign_name": ptNullableStr(row, "campaign_name"), "error_code": ptNullableStr(row, "error_code"),
			"error_message": errMsg, "http_status": httpStatus, "updated_at": row.anyOrNil("updated_at"),
		})
	}
	jobsOut := make([]rowMap, 0, len(jobs))
	for _, raw := range jobs {
		row := ptRow(ptNormalizeMap(raw))
		errMsg := ptNullableStr(row, "error_message")
		var httpStatus any
		if s, okString := errMsg.(string); okString {
			httpStatus = opHTTPStatusFromMessage(s)
		}
		jobsOut = append(jobsOut, map[string]any{
			"id": row.anyOrNil("id"), "publication_id": ptNullableStr(row, "publication_id"),
			"status": row.anyOrNil("status"), "brand_name": ptNullableStr(row, "brand_name"),
			"media_name": ptNullableStr(row, "media_name"), "document_title": ptNullableStr(row, "document_title"),
			"error_code": ptNullableStr(row, "error_code"), "error_message": errMsg,
			"http_status": httpStatus, "attempts": row.intOr("attempts", 0), "updated_at": row.anyOrNil("updated_at"),
		})
	}
	warningsOut := make([]rowMap, 0, len(warnings))
	for _, raw := range warnings {
		row := ptRow(ptNormalizeMap(raw))
		warningsOut = append(warningsOut, map[string]any{
			"id": row.anyOrNil("id"), "publication_id": ptNullableStr(row, "publication_id"),
			"status": row.anyOrNil("status"), "brand_name": ptNullableStr(row, "brand_name"),
			"media_name": ptNullableStr(row, "media_name"), "document_title": ptNullableStr(row, "document_title"),
			"warning_message": ptNullableStr(row, "warning_message"), "updated_at": row.anyOrNil("updated_at"),
		})
	}
	rssOut := make([]rowMap, 0, len(rss))
	for _, raw := range rss {
		row := ptRow(ptNormalizeMap(raw))
		rssOut = append(rssOut, map[string]any{
			"id": row.anyOrNil("id"), "name": row.anyOrNil("name"),
			"install_status": row.anyOrNil("install_status"), "install_error": ptNullableStr(row, "install_error"),
			"partner_name": ptNullableStr(row, "partner_name"), "media_name": ptNullableStr(row, "media_name"),
			"updated_at": row.anyOrNil("updated_at"),
		})
	}
	queueOut := make([]rowMap, 0, len(queue))
	for _, raw := range queue {
		row := ptRow(ptNormalizeMap(raw))
		queueOut = append(queueOut, map[string]any{
			"id": row.anyOrNil("id"), "kind": row.anyOrNil("kind"), "status": row.anyOrNil("status"),
			"last_error": ptNullableStr(row, "last_error"), "attempts": row.intOr("attempts", 0),
			"updated_at": row.anyOrNil("updated_at"),
		})
	}
	batchesOut := make([]rowMap, 0, len(variantBatches))
	for _, raw := range variantBatches {
		row := ptRow(ptNormalizeMap(raw))
		batchesOut = append(batchesOut, map[string]any{
			"id": row.anyOrNil("id"), "mode": row.anyOrNil("mode"), "status": row.anyOrNil("status"),
			"brand_name": ptNullableStr(row, "brand_name"), "brand_public_id": ptNullableStr(row, "brand_public_id"),
			"total": row.intOr("total", 0), "failed": row.intOr("failed", 0),
			"failed_media": ptNullableStr(row, "failed_media"), "updated_at": row.anyOrNil("updated_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"publications": len(publications), "jobs": len(jobs), "rss": len(rss),
			"queue": len(queue), "variant_batches": len(variantBatches),
			"warnings": len(warnings),
		},
		"publications": pubsOut, "jobs": jobsOut, "rss": rssOut,
		"queue": queueOut, "variant_batches": batchesOut, "warnings": warningsOut,
	})
}

// opLikeClause: susun fragmen filter ILIKE ber-placeholder $n berurutan.
// `template` memakai penanda "%s" yang diganti $n (params diisi `like`).
// Penggantian dilakukan manual (bukan fmt.Sprintf) supaya format string tetap
// konstan di mata vet.
func opLikeClause(like any, template string, count int, start ...int) (string, []any) {
	if like == nil {
		return "", nil
	}
	first := 1
	if len(start) > 0 {
		first = start[0]
	}
	params := make([]any, 0, count)
	sql := template
	for i := 0; i < count; i++ {
		idx := strings.Index(sql, "%s")
		if idx < 0 {
			break
		}
		sql = sql[:idx] + "$" + ptItoa(first+i) + sql[idx+2:]
		params = append(params, like)
	}
	return "AND " + sql, params
}

// ---------- generation switch ----------

// OperationsGenerationSwitch: PUT /v1/platform/operations/generation-switch
func (d *Deps) OperationsGenerationSwitch(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body := decodeJSON(r)
	disabled, hasDisabled, isBool := opGetBool(body, "generation_disabled")
	if !hasDisabled || !isBool {
		writeErr(w, d, errUnprocessable("generation_disabled is required"))
		return
	}
	if err := d.opSetGenerationDisabled(ctx, disabled); err != nil {
		writeErr(w, d, err)
		return
	}
	action := "platform.ai.generation_enabled"
	if disabled {
		action = "platform.ai.generation_disabled"
	}
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type) VALUES ($1, $2, $3, 'platform')",
		uuid.NewString(), user.ID, action); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"generation_disabled": d.opGenerationDisabled(ctx)})
}

// ---------- notifications ----------

// opNotificationWindow: parity WINDOW (24 jam terakhir).
const opNotificationWindow = "NOW() - INTERVAL '24 hours'"

// OperationsNotifications: GET /v1/platform/operations/notifications
func (d *Deps) OperationsNotifications(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	publications, err := d.queryRows(ctx,
		`SELECT p.id, p.status, p.updated_at, m.name AS media_name, d.title AS document_title,
		        (SELECT pa.error_message FROM publication_attempts pa WHERE pa.publication_id = p.id ORDER BY pa.attempt_number DESC LIMIT 1) AS error_message
		 FROM publications p
		 JOIN media m ON m.id = p.media_id
		 JOIN content_documents d ON d.id = p.document_id
		 WHERE p.status IN ('failed', 'retrying') AND p.updated_at > `+opNotificationWindow+`
		 ORDER BY p.updated_at DESC LIMIT 20`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	jobs, err := d.queryRows(ctx,
		`SELECT j.id, j.status, j.error_message, j.updated_at, b.name AS brand_name, d.title AS document_title
		 FROM publication_jobs j
		 LEFT JOIN brands b ON b.id = j.brand_id
		 LEFT JOIN content_documents d ON d.id = j.document_id
		 WHERE j.status = 'failed' AND j.updated_at > `+opNotificationWindow+`
		 ORDER BY j.updated_at DESC LIMIT 20`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	rss, err := d.queryRows(ctx,
		`SELECT f.id, f.name, f.install_error, f.updated_at, p.name AS partner_name
		 FROM rss_feeds f
		 LEFT JOIN partners p ON p.id = f.partner_id
		 WHERE f.install_status = 'failed' AND f.updated_at > `+opNotificationWindow+`
		 ORDER BY f.updated_at DESC LIMIT 20`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	pendingMedia, err := d.queryRow(ctx,
		"SELECT COUNT(*) AS count, MAX(updated_at) AS updated_at FROM media WHERE status = 'pending_review'")
	if err != nil {
		writeErr(w, d, err)
		return
	}

	items := []opNotifItem{}

	if pendingMedia != nil {
		pendingRow := ptRow(ptNormalizeMap(pendingMedia))
		pendingCount := pendingRow.intOr("count", 0)
		if pendingCount > 0 {
			createdAt := pendingRow.strOr("updated_at", "")
			if createdAt == "" {
				createdAt = opNowISO()
			}
			items = append(items, opNotifItem{
				ID: "media-pending", Type: "media.submitted", Title: "Media baru menunggu persetujuan",
				Body:   fmt.Sprintf("%d media menunggu review Admin.", pendingCount),
				Link:   "/admin/media?status=pending_review#tab-pending",
				ReadAt: nil, CreatedAt: createdAt,
			})
		}
	}
	for _, raw := range publications {
		row := ptRow(ptNormalizeMap(raw))
		title := "Publikasi gagal"
		if row.strOr("status", "") == "retrying" {
			title = "Publikasi mencoba ulang"
		}
		title2 := row.strOr("document_title", "Artikel")
		mediaName := row.strOr("media_name", "media")
		body := title2 + " → " + mediaName
		if errMsg, okString := row.str("error_message"); okString {
			body += ": " + opTruncate(errMsg, 160)
		}
		items = append(items, opNotifItem{
			ID: "pub-" + row.strOr("id", ""), Type: "publication.failed", Title: title,
			Body: body, Link: "/admin/errors", ReadAt: nil, CreatedAt: row.strOr("updated_at", ""),
		})
	}
	for _, raw := range jobs {
		row := ptRow(ptNormalizeMap(raw))
		body := row.strOr("document_title", "Artikel")
		if errMsg, okString := row.str("error_message"); okString {
			body += ": " + opTruncate(errMsg, 160)
		}
		items = append(items, opNotifItem{
			ID: "job-" + row.strOr("id", ""), Type: "job.failed", Title: "Job distribusi gagal",
			Body: body, Link: "/admin/errors", ReadAt: nil, CreatedAt: row.strOr("updated_at", ""),
		})
	}
	for _, raw := range rss {
		row := ptRow(ptNormalizeMap(raw))
		body := `Feed "` + row.strOr("name", "") + `"`
		if partnerName, okPartner := row.str("partner_name"); okPartner {
			body += " (" + partnerName + ")"
		}
		if errMsg, okString := row.str("install_error"); okString {
			body += ": " + opTruncate(errMsg, 160)
		}
		items = append(items, opNotifItem{
			ID: "rss-" + row.strOr("id", ""), Type: "rss.install_failed", Title: "RSS gagal verifikasi",
			Body: body, Link: "/admin/rss-feeds", ReadAt: nil, CreatedAt: row.strOr("updated_at", ""),
		})
	}
	// Queue sengaja TIDAK dimasukkan ke lonceng: kegagalan job berulang
	// dianggap bising. Pantauannya ada di halaman Operations (tab "Queue
	// gagal", dikelompokkan per kind) dan satu ringkasan di daftar alerts.
	// Riwayat notifikasi admin dari tabel notifications (semua tipe, 7 hari).
	// Tanpa ini lonceng admin hanya menampilkan masalah "saat ini" dari feed
	// sintetis dan tidak punya riwayat sama sekali.
	notifs, err := d.queryRows(ctx,
		`SELECT id, type, title, body, link, read_at, created_at FROM notifications
		 WHERE user_id = $1 AND created_at > NOW() - INTERVAL '7 days'
		 ORDER BY created_at DESC LIMIT 20`, user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	for _, raw := range notifs {
		row := ptRow(ptNormalizeMap(raw))
		notifType := row.strOr("type", "notification")
		// Tautan hanya boleh ke halaman admin sendiri: link tersimpan bisa
		// menunjuk ke halaman klien/mitra yang bukan milik admin ini. Tautan
		// admin yang valid (mis. tiket dukungan /admin/dukungan?t=...) dipakai
		// apa adanya supaya klik notifikasi langsung membuka konteksnya.
		link := "/admin/notifications"
		if strings.HasPrefix(notifType, "announcement") {
			link = "/admin/pengumuman"
		} else if stored := strings.TrimSpace(row.strOr("link", "")); strings.HasPrefix(stored, "/admin/") {
			link = stored
		}
		items = append(items, opNotifItem{
			ID: "ntf-" + row.strOr("id", ""), Type: notifType,
			Title: row.strOr("title", ""), Body: row.raw("body"),
			Link: link, ReadAt: row.anyOrNil("read_at"), CreatedAt: row.strOr("created_at", ""),
		})
	}
	sortNotificationsDesc(items)
	if len(items) > 50 {
		items = items[:50]
	}
	// `unread` dihitung dari item yang dikembalikan: baris notifications yang
	// read_at-nya terisi sudah dibaca (dulu semuanya dihitung belum dibaca,
	// sehingga badge lonceng bisa muncul tanpa isi yang belum dibaca), sedangkan
	// item sintetis selalu ReadAt nil karena memang butuh perhatian.
	unread := 0
	for _, item := range items {
		if item.ReadAt == nil {
			unread++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": items, "unread": unread})
}

// OperationsQueueAlerts: GET /v1/platform/operations/queue-alerts
// Log kegagalan job queue 24 jam terakhir, dikelompokkan per kind — halaman
// Operations menampilkan seluruhnya; lonceng hanya 1 peringatan per kind.
func (d *Deps) OperationsQueueAlerts(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT DISTINCT ON (kind) kind, updated_at AS last_at, last_error,
		        count(*) OVER (PARTITION BY kind) AS failures
		 FROM jobs WHERE status = 'failed' AND updated_at > `+opNotificationWindow+`
		 ORDER BY kind, updated_at DESC`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		items = append(items, map[string]any{
			"kind":       row.strOr("kind", ""),
			"failures":   int(row.numOr("failures")),
			"last_at":    row.strOr("last_at", ""),
			"last_error": row.strOr("last_error", ""),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// sortNotificationsDesc: urutkan feed menurun berdasarkan created_at (ISO).
func sortNotificationsDesc(items []opNotifItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return opISOValue(items[i].CreatedAt) > opISOValue(items[j].CreatedAt)
	})
}

// opNotifItem: item feed lonceng admin.
type opNotifItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      any    `json:"body"`
	Link      string `json:"link"`
	ReadAt    any    `json:"read_at"`
	CreatedAt string `json:"created_at"`
}

// opISOValue: kunci sortir ISO yang konsisten (string ISO selalu bisa
// dibandingkan leksikografis); kosong → "0" agar di akhir.
func opISOValue(v string) string {
	if strings.TrimSpace(v) == "" {
		return "0"
	}
	return v
}

// ---------- publications ----------

// opPublicationSelectColumns: kolom + subquery yang dipakai list admin platform.
const opPublicationSelectColumns = `p.id, p.order_code, p.status, p.client_price, p.partner_price, p.markup_percent, p.publish_status, p.scheduled_at, p.published_at, p.published_url, p.created_at, p.updated_at,
              b.name AS brand_name, b.public_id AS brand_public_id,
              m.name AS media_name, m.integration_type AS media_integration_type,
              d.title AS document_title, c.name AS campaign_name, pa.name AS partner_name,
              (SELECT pa2.error_code FROM publication_attempts pa2 WHERE pa2.publication_id = p.id ORDER BY pa2.attempt_number DESC LIMIT 1) AS error_code,
              (SELECT pa2.error_message FROM publication_attempts pa2 WHERE pa2.publication_id = p.id ORDER BY pa2.attempt_number DESC LIMIT 1) AS error_message,
              (SELECT v.angle_label FROM publication_variants v WHERE v.publication_id = p.id LIMIT 1) AS variant_angle,
              (SELECT b.mode FROM publication_variants v JOIN publication_variant_batches b ON b.id = v.batch_id WHERE v.publication_id = p.id LIMIT 1) AS variant_mode,
              (SELECT pj.remote_post_id FROM publication_jobs pj WHERE pj.publication_id = p.id ORDER BY pj.created_at DESC LIMIT 1) AS remote_post_id`

const opPublicationFromClause = `FROM publications p
       JOIN brands b ON b.id = p.brand_id
       JOIN media m ON m.id = p.media_id
       JOIN content_documents d ON d.id = p.document_id
       LEFT JOIN campaigns c ON c.id = p.campaign_id
       LEFT JOIN partners pa ON pa.id = m.partner_id`

// OperationsPublications: GET /v1/platform/operations/publications
func (d *Deps) OperationsPublications(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	status := q.Get("status")
	if status != "" && !ptContains(publicationStatusesForFilter, status) {
		writeErr(w, d, errUnprocessable("Invalid publication status filter"))
		return
	}
	search := strings.TrimSpace(q.Get("q"))
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	clauses := []string{}
	baseParams := []any{}
	if status != "" {
		baseParams = append(baseParams, status)
		clauses = append(clauses, "p.status = $"+ptItoa(len(baseParams)))
	}
	if search != "" {
		like := "%" + search + "%"
		for i := 0; i < 7; i++ {
			baseParams = append(baseParams, like)
		}
		base := len(baseParams) - 6
		clauses = append(clauses, "(d.title ILIKE $"+ptItoa(base)+" OR b.name ILIKE $"+ptItoa(base+1)+
			" OR m.name ILIKE $"+ptItoa(base+2)+" OR c.name ILIKE $"+ptItoa(base+3)+
			" OR pa.name ILIKE $"+ptItoa(base+4)+" OR p.id ILIKE $"+ptItoa(base+5)+
			" OR p.order_code ILIKE $"+ptItoa(base+6)+")")
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	fromClause := opPublicationFromClause + " " + where

	total := 0
	if row, err := d.queryRow(ctx, "SELECT COUNT(*) AS count "+fromClause, baseParams...); err == nil && row != nil {
		total = row.intOr("count", 0)
	}

	if q.Get("format") == "csv" {
		all, err := d.queryRows(ctx,
			"SELECT "+opPublicationSelectColumns+" "+fromClause+" ORDER BY p.created_at DESC", baseParams...)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition",
			`attachment; filename="tulisio-publikasi-`+opDateOnly()+`.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(opPublicationsCSV(asMaps(all))))
		return
	}

	args := append([]any{}, baseParams...)
	args = append(args, limit, offset)
	items, err := d.queryRows(ctx,
		"SELECT "+opPublicationSelectColumns+" "+fromClause+
			" ORDER BY p.created_at DESC LIMIT $"+ptItoa(len(baseParams)+1)+" OFFSET $"+ptItoa(len(baseParams)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	publications := make([]rowMap, 0, len(items))
	for _, raw := range items {
		publications = append(publications, opPublicationListItem(ptRow(ptNormalizeMap(raw))))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"publications": publications, "total": total, "limit": limit, "offset": offset,
	})
}

// opDateOnly: parity new Date().toISOString().slice(0, 10).
func opDateOnly() string { return time.Now().UTC().Format("2006-01-02") }

// OperationsPublicationDetail: GET /v1/platform/operations/publications/{publicationId}
func (d *Deps) OperationsPublicationDetail(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	publicationID := chiParam(r, "publicationId")
	row, err := d.queryRow(ctx,
		`SELECT p.id, p.status, p.client_price, p.partner_price, p.markup_percent, p.publish_status, p.connection_id, p.scheduled_at, p.published_at, p.published_url,
		        p.notes, p.cancel_reason, p.rejection_reason, p.created_by, p.created_at, p.updated_at,
		        b.name AS brand_name, b.public_id AS brand_public_id,
		        m.name AS media_name, m.integration_type AS media_integration_type, m.adapter_url, m.partner_id,
		        d.id AS document_id, d.title AS document_title, d.slug AS document_slug, d.current_version,
		        c.name AS campaign_name, pa.name AS partner_name, pa.public_id AS partner_public_id,
		        u.email AS created_by_email
		 FROM publications p
		 JOIN brands b ON b.id = p.brand_id
		 JOIN media m ON m.id = p.media_id
		 JOIN content_documents d ON d.id = p.document_id
		 LEFT JOIN campaigns c ON c.id = p.campaign_id
		 LEFT JOIN partners pa ON pa.id = m.partner_id
		 LEFT JOIN users u ON u.id = p.created_by
		 WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Publication not found"))
		return
	}
	attempts, err := d.queryRows(ctx,
		`SELECT attempt_number, status, error_code, error_message, started_at, completed_at
		 FROM publication_attempts WHERE publication_id = $1 ORDER BY attempt_number ASC`, publicationID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	jobs, err := d.queryRows(ctx,
		`SELECT id, status, publish_status, payload_snapshot, remote_post_id, remote_url, error_code, error_message, warning_message, attempts, started_at, completed_at
		 FROM publication_jobs WHERE publication_id = $1 ORDER BY created_at DESC LIMIT 1`, publicationID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	var jobRow map[string]any
	if len(jobs) > 0 {
		jobRow = jobs[0]
	}
	callbacks, err := d.queryRows(ctx,
		`SELECT status_code, accepted, error_message, ip_address, user_agent, created_at
		 FROM publishing_callback_logs WHERE publication_id = $1 ORDER BY created_at DESC LIMIT 50`, publicationID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	replacesRow, err := d.queryRow(ctx,
		`SELECT p.id, p.status, p.created_at, m.name AS media_name, d.title AS document_title,
		        b.public_id AS brand_public_id
		 FROM publications p
		 JOIN media m ON m.id = p.media_id
		 JOIN content_documents d ON d.id = p.document_id
		 LEFT JOIN brands b ON b.id = p.brand_id
		 WHERE p.id = (SELECT replaces_id FROM publications WHERE id = $1) LIMIT 1`, publicationID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	replacedByRow, err := d.queryRow(ctx,
		`SELECT p.id, p.status, p.created_at, p.published_at, p.published_url, m.name AS media_name,
		        d.title AS document_title, b.public_id AS brand_public_id
		 FROM publications p
		 JOIN media m ON m.id = p.media_id
		 JOIN content_documents d ON d.id = p.document_id
		 LEFT JOIN brands b ON b.id = p.brand_id
		 WHERE p.replaces_id = $1 ORDER BY p.created_at ASC LIMIT 1`, publicationID)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	var payloadSnapshot any
	if jobRow != nil {
		jr := ptRow(ptNormalizeMap(jobRow))
		if rawSnapshot := jr.anyOrNil("payload_snapshot"); rawSnapshot != nil {
			switch t := rawSnapshot.(type) {
			case string:
				var parsed map[string]any
				if json.Unmarshal([]byte(t), &parsed) == nil {
					payloadSnapshot = parsed
				}
			case map[string]any:
				payloadSnapshot = t
			}
		}
	}

	base := opPublicationListItem(ptRow(ptNormalizeMap(row)))
	publication := map[string]any{}
	for k, v := range base {
		publication[k] = v
	}
	publication["connection_id"] = ptNullableStr(ptRow(ptNormalizeMap(row)), "connection_id")
	publication["notes"] = ptNullableStr(ptRow(ptNormalizeMap(row)), "notes")
	publication["cancel_reason"] = ptNullableStr(ptRow(ptNormalizeMap(row)), "cancel_reason")
	publication["rejection_reason"] = ptNullableStr(ptRow(ptNormalizeMap(row)), "rejection_reason")
	publication["adapter_url"] = ptNullableStr(ptRow(ptNormalizeMap(row)), "adapter_url")
	publication["partner_public_id"] = ptNullableStr(ptRow(ptNormalizeMap(row)), "partner_public_id")
	publication["document_id"] = row["document_id"]
	publication["document_slug"] = ptNullableStr(ptRow(ptNormalizeMap(row)), "document_slug")
	currentVersion := 1
	if rv := ptRow(ptNormalizeMap(row)); rv.has("current_version") {
		currentVersion = rv.intOr("current_version", 1)
	}
	publication["current_version"] = currentVersion
	publication["created_by_email"] = ptNullableStr(ptRow(ptNormalizeMap(row)), "created_by_email")

	attemptsOut := make([]rowMap, 0, len(attempts))
	for _, raw := range attempts {
		a := ptRow(ptNormalizeMap(raw))
		attemptsOut = append(attemptsOut, map[string]any{
			"attempt_number": a.intOr("attempt_number", 0), "status": a.anyOrNil("status"),
			"error_code": ptNullableStr(a, "error_code"), "error_message": ptNullableStr(a, "error_message"),
			"started_at": a.anyOrNil("started_at"), "completed_at": a.anyOrNil("completed_at"),
		})
	}
	var jobOut any
	if jobRow != nil {
		jr := ptRow(ptNormalizeMap(jobRow))
		jobOut = map[string]any{
			"id": jr.anyOrNil("id"), "status": jr.anyOrNil("status"),
			"publish_status":  jr.anyOrNil("publish_status"),
			"remote_post_id":  ptNullableStr(jr, "remote_post_id"),
			"remote_url":      ptNullableStr(jr, "remote_url"),
			"error_code":      ptNullableStr(jr, "error_code"),
			"error_message":   ptNullableStr(jr, "error_message"),
			"warning_message": ptNullableStr(jr, "warning_message"),
			"attempts":        jr.intOr("attempts", 0),
			"started_at":      jr.anyOrNil("started_at"), "completed_at": jr.anyOrNil("completed_at"),
			"payload_snapshot": payloadSnapshot,
		}
	}
	callbacksOut := make([]rowMap, 0, len(callbacks))
	for _, raw := range callbacks {
		c := ptRow(ptNormalizeMap(raw))
		callbacksOut = append(callbacksOut, map[string]any{
			"status_code": c.anyOrNil("status_code"), "accepted": c.anyOrNil("accepted"),
			"error_message": ptNullableStr(c, "error_message"), "ip_address": ptNullableStr(c, "ip_address"),
			"user_agent": ptNullableStr(c, "user_agent"), "created_at": c.anyOrNil("created_at"),
		})
	}
	var replaces any
	if replacesRow != nil {
		rr := ptRow(ptNormalizeMap(replacesRow))
		replaces = map[string]any{
			"id": rr.anyOrNil("id"), "status": rr.anyOrNil("status"),
			"media_name": ptNullableStr(rr, "media_name"), "document_title": ptNullableStr(rr, "document_title"),
			"created_at": rr.anyOrNil("created_at"), "brand_public_id": ptNullableStr(rr, "brand_public_id"),
		}
	}
	var replacedBy any
	if replacedByRow != nil {
		rr := ptRow(ptNormalizeMap(replacedByRow))
		replacedBy = map[string]any{
			"id": rr.anyOrNil("id"), "status": rr.anyOrNil("status"),
			"media_name": ptNullableStr(rr, "media_name"), "document_title": ptNullableStr(rr, "document_title"),
			"published_at": rr.anyOrNil("published_at"), "published_url": ptNullableStr(rr, "published_url"),
			"created_at": rr.anyOrNil("created_at"), "brand_public_id": ptNullableStr(rr, "brand_public_id"),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"publication": publication, "attempts": attemptsOut, "job": jobOut,
		"callbacks": callbacksOut, "variant": d.opVariantForPublication(ctx, publicationID),
		"replaces": replaces, "replaced_by": replacedBy,
		"pr_analysis": d.opPublicationPRAnalysis(ctx, publicationID),
	})
}

// OperationsPublicationRetry: POST /v1/platform/operations/publications/{publicationId}/retry
func (d *Deps) OperationsPublicationRetry(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	publicationID := chiParam(r, "publicationId")
	publication, err := d.queryRow(ctx,
		"SELECT id, status, brand_id FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if publication == nil {
		writeErr(w, d, errNotFound("Publication not found"))
		return
	}
	row := ptRow(ptNormalizeMap(publication))
	current := row.strOr("status", "")
	if current != "failed" && current != "retrying" {
		writeErr(w, d, errConflict(fmt.Sprintf("Publication is not retryable (current status: %s)", current)))
		return
	}
	if err := d.opTransitionPublication(ctx, publicationID, "queued"); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(ctx,
		"UPDATE publication_jobs SET error_code = NULL, error_message = NULL, status = 'queued' WHERE publication_id = $1 AND status <> 'completed'",
		publicationID); err != nil {
		writeErr(w, d, err)
		return
	}
	jobID, err := d.opEnqueueJob(ctx, "publication_execution",
		map[string]any{"publicationId": publicationID, "brandId": row.strOr("brand_id", "")}, 3)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "queued", "job_id": jobID})
}

// opPublicationRelink: inti aksi "perbaiki / sinkron ulang". Menandai ulang
// publikasi terbit supaya job eksekusi jalan lagi dan kategori, gambar utama,
// serta gambar badan artikel di-resolve dari awal. Post mitra DIPERBARUI, bukan
// dibuat ulang: remote_post_id dipertahankan (worker memakai ID itu untuk
// mengirim ulang ke wp/v2/posts/{id}).
//
// label dipakai di pesan kesalahan agar tiap tombol menjelaskan dirinya
// sendiri; wordpressOnly=true untuk tombol sinkron kategori + gambar, yang
// hanya masuk akal pada integrasi WordPress.
func (d *Deps) opPublicationRelink(ctx context.Context, publicationID, label string, wordpressOnly bool) (map[string]any, error) {
	row, err := d.queryRow(ctx,
		`SELECT p.id, p.status, p.brand_id, p.published_url, m.integration_type
		 FROM publications p
		 JOIN media m ON m.id = p.media_id
		 WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Publication not found")
	}
	pub := ptRow(ptNormalizeMap(row))
	current := pub.strOr("status", "")
	if current != "published" {
		return nil, errConflict(fmt.Sprintf("Hanya publikasi terbit yang bisa %s (status saat ini: %s)", label, current))
	}
	integration := pub.strOr("integration_type", "")
	if wordpressOnly && integration != "wordpress" {
		return nil, errConflict(fmt.Sprintf("Sinkron ulang gambar & kategori hanya untuk media WordPress (integrasi media ini: %s).", integration))
	}
	jobRow, err := d.queryRow(ctx,
		"SELECT id, remote_post_id FROM publication_jobs WHERE publication_id = $1 ORDER BY created_at DESC LIMIT 1",
		publicationID)
	if err != nil {
		return nil, err
	}
	if jobRow == nil {
		return nil, errConflict(fmt.Sprintf("Publikasi belum punya job pengiriman, tidak ada yang bisa %s.", label))
	}
	job := ptRow(ptNormalizeMap(jobRow))
	remotePostID := strings.TrimSpace(job.strOr("remote_post_id", ""))
	// Post lama kadang hanya menyimpan tautannya (…/?p=152), bukan ID post-nya.
	derivedID := ""
	if remotePostID == "" {
		derivedID = opPostIDFromWPURL(pub.strOr("published_url", ""))
		if derivedID == "" {
			return nil, errConflict(fmt.Sprintf("Publikasi belum punya post mitra (remote_post_id) dan tautan terbitnya tidak memuat ID post, tidak ada yang bisa %s.", label))
		}
	}
	// published → revision → queued: satu-satunya jalur transisi yang diizinkan
	// menuju eksekusi ulang.
	if err := d.opTransitionPublication(ctx, publicationID, "revision"); err != nil {
		return nil, err
	}
	if err := d.opTransitionPublication(ctx, publicationID, "queued"); err != nil {
		return nil, err
	}
	// payload_snapshot direset ke bentuk "job baru" (publish_status dipertahankan)
	// supaya kategori, gambar utama, dan gambar badan artikel di-resolve serta
	// diunggah ulang. Tidak boleh dikosongkan seluruhnya: kolomnya NOT NULL, dan
	// tanpa publish_status post live akan turun jadi draft. remote_post_id
	// sengaja disimpan; bila kosong, ID dari tautan dipakai supaya post yang sama
	// yang diperbarui.
	if err := d.exec(ctx, `
		UPDATE publication_jobs
		SET status = 'queued', progress = 0, error_code = NULL, error_message = NULL,
		    warning_message = NULL,
		    payload_snapshot = jsonb_build_object(
		        'publish_status', COALESCE(NULLIF(publish_status, ''), 'draft'),
		        'excerpt', NULL,
		        'categories', '[]'::jsonb,
		        'tags', '[]'::jsonb),
		    remote_post_id = COALESCE(NULLIF($2, ''), remote_post_id)
		WHERE id = $1`, job.strOr("id", ""), derivedID); err != nil {
		return nil, err
	}
	jobID, err := d.opEnqueueJob(ctx, "publication_execution",
		map[string]any{"publicationId": publicationID, "brandId": pub.strOr("brand_id", "")}, 3)
	if err != nil {
		return nil, err
	}
	remotePost := remotePostID
	if remotePost == "" {
		remotePost = derivedID
	}
	return map[string]any{
		"ok": true, "status": "queued", "job_id": jobID,
		"integration_type": integration, "remote_post_id": remotePost,
	}, nil
}

// opPostIDFromWPURL: ID post dari tautan gaya "…?p=152". Permalink cantik
// ("/judul-artikel/") tidak memuat ID sehingga hasilnya kosong.
func opPostIDFromWPURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(parsed.Query().Get("p"))
	if id == "" {
		return ""
	}
	for _, ch := range id {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return id
}

// OperationsPublicationRepairImages: POST /v1/platform/operations/publications/{publicationId}/repair-images
// Jalankan ulang pengiriman untuk publikasi yang SUDAH terbit supaya gambar
// artikel diunggah ke media library mitra dan gambar utama dipasang.
func (d *Deps) OperationsPublicationRepairImages(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	res, err := d.opPublicationRelink(r.Context(), chiParam(r, "publicationId"), "diperbaiki", false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// OperationsPublicationResyncWordpress: POST /v1/platform/operations/publications/{publicationId}/resync-wordpress
// Sama dengan repair-images tetapi khusus WordPress: dipakai tombol "Sinkron
// ulang gambar & kategori" ketika post mitra terbit tanpa keduanya.
func (d *Deps) OperationsPublicationResyncWordpress(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	res, err := d.opPublicationRelink(r.Context(), chiParam(r, "publicationId"), "disinkronkan", true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------- running batches ----------

// OperationsRunningBatches: GET /v1/platform/operations/running-batches
func (d *Deps) OperationsRunningBatches(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT cb.id, cb.brand_id, b.name AS brand_name, cb.name, cb.status, cb.total_items, cb.updated_at
		 FROM content_batches cb JOIN brands b ON b.id = cb.brand_id
		 WHERE cb.status = 'running' ORDER BY cb.updated_at DESC LIMIT 100`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, ptRows(rows))
}

// ---------- users ----------

var opUserRoles = []string{"client", "partner"}

// OperationsUsers: GET/POST /v1/platform/operations/users
func (d *Deps) OperationsUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		d.operationsUsersList(w, r)
		return
	}
	d.operationsUsersCreate(w, r)
}

func (d *Deps) operationsUsersList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("search"))
	role := q.Get("role")
	base := `
      SELECT u.id, u.email, u.display_name, u.role, u.is_active, u.approved_at,
        (SELECT COUNT(*) FROM brand_memberships bm WHERE bm.user_id = u.id) AS brand_count,
        u.writer_generate_quota,
        COALESCE((SELECT SUM(w.available_tc)
                    FROM brand_memberships bm
                    JOIN credit_wallets w ON w.brand_id = bm.brand_id
                   WHERE bm.user_id = u.id), 0) AS available_tc,
        u.created_at, np.wa_number AS wa_number
      FROM users u
      LEFT JOIN notification_preferences np ON np.user_id = u.id`
	conditions := []string{}
	params := []any{}
	if role == "client" || role == "partner" {
		params = append(params, role)
		conditions = append(conditions, "u.role = $"+ptItoa(len(params)))
	}
	if search != "" {
		like := "%" + search + "%"
		params = append(params, like, like)
		base2 := len(params) - 1
		// Email/nama, plus nomor WhatsApp pada digitnya saja — "2922" harus
		// menemukan nomor yang tersimpan "+62 812-2922".
		cond := "u.email LIKE $" + ptItoa(base2) + " OR u.display_name LIKE $" + ptItoa(base2+1)
		for _, term := range phoneSearchTerms(search) {
			params = append(params, "%"+term+"%")
			cond += " OR regexp_replace(COALESCE(np.wa_number, ''), '[^0-9]', '', 'g') LIKE $" + ptItoa(len(params))
		}
		conditions = append(conditions, "("+cond+")")
	}
	clause := ""
	if len(conditions) > 0 {
		clause = " WHERE " + strings.Join(conditions, " AND ")
	}
	rows, err := d.queryRows(r.Context(), base+clause+" ORDER BY u.created_at DESC LIMIT 200", params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		approvedAt := row.anyOrNil("approved_at")
		out = append(out, map[string]any{
			"id": row.anyOrNil("id"), "email": row.anyOrNil("email"),
			"display_name": row.anyOrNil("display_name"), "role": row.anyOrNil("role"),
			"is_active": row.anyOrNil("is_active"), "is_approved": approvedAt != nil,
			"approved_at": approvedAt, "brand_count": row.numOr("brand_count"),
			"writer_generate_quota": row.anyOrNil("writer_generate_quota"),
			// Saldo TC akun klien: total available di seluruh brand miliknya.
			"available_tc": row.numOr("available_tc"),
			"wa_number":    row.anyOrNil("wa_number"),
			"created_at":   row.anyOrNil("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (d *Deps) operationsUsersCreate(w http.ResponseWriter, r *http.Request) {
	admin, adminCtx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body := decodeJSON(r)
	type createUserBody struct {
		Email       string  `json:"email"`
		DisplayName string  `json:"display_name"`
		Role        string  `json:"role"`
		Password    *string `json:"password"`
		Redirect    *string `json:"redirect"`
	}
	var parsed createUserBody
	if raw, mErr := json.Marshal(body); mErr == nil {
		_ = json.Unmarshal(raw, &parsed)
	}
	email := strings.ToLower(strings.TrimSpace(parsed.Email))
	if !opEmailRe.MatchString(email) || len(email) > 320 {
		writeErr(w, d, errUnprocessable("Invalid email"))
		return
	}
	displayName := strings.TrimSpace(parsed.DisplayName)
	if len(displayName) < 2 || len(displayName) > 120 {
		writeErr(w, d, errUnprocessable(opStrTooSmall(2)))
		return
	}
	if !ptContains(opUserRoles, parsed.Role) {
		writeErr(w, d, errUnprocessable("Invalid enum value. Expected "+opEnumExpected(opUserRoles)))
		return
	}
	if parsed.Password != nil && (len(*parsed.Password) < 10 || len(*parsed.Password) > 200) {
		writeErr(w, d, errUnprocessable(opStrTooSmall(10)))
		return
	}
	if parsed.Redirect != nil && len(*parsed.Redirect) > 500 {
		writeErr(w, d, errUnprocessable(opStrTooBig(500)))
		return
	}
	password := parsed.Password
	if password != nil && !adminCtx.IsSuperAdmin() {
		writeErr(w, d, errForbidden("Hanya Super Admin yang boleh menetapkan password langsung."))
		return
	}

	existing, err := d.queryRow(ctx, "SELECT id FROM users WHERE email = $1 LIMIT 1", email)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if existing != nil {
		writeErr(w, d, errConflict("Email sudah terdaftar."))
		return
	}
	passwordHash := ""
	if password != nil {
		hash, hErr := auth.HashPassword(*password)
		if hErr != nil {
			writeErr(w, d, hErr)
			return
		}
		passwordHash = hash
	}
	id := uuid.NewString()
	txErr := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			"INSERT INTO users (id, email, password_hash, display_name, role, registration_source, is_active, approved_at, created_at) VALUES ($1, $2, $3, $4, $5, 'admin', TRUE, NOW(), NOW())",
			id, email, passwordHash, displayName, parsed.Role); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, 'user.admin_created', 'user', $3)",
			uuid.NewString(), admin.ID, id); err != nil {
			return err
		}
		if parsed.Role == "partner" {
			return auth.ProvisionPartner(ctx, tx, id, displayName, "")
		}
		return auth.ProvisionClient(ctx, tx, id, displayName)
	})
	if txErr != nil {
		writeErr(w, d, txErr)
		return
	}

	if password != nil {
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": id, "activation_sent": false, "password_set": true})
		return
	}

	activationSent := false
	token, tErr := auth.CreateResetToken(ctx, d.Pool, id, 60)
	if tErr == nil {
		appURL := d.Cfg.PublicAppURL
		link := appURL + "/reset-password?token=" + token
		if _, _, sendErr := emailqueue.SendTransactionalEmail(ctx, d.Pool, emailqueue.SendOptions{
			Template: "account-invite", To: email, RecipientUserID: &id,
			Event: "security.account_invite",
			Data:  map[string]string{"link": link, "display_name": displayName, "role": parsed.Role},
		}); sendErr == nil {
			activationSent = true
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": id, "activation_sent": activationSent})
}

// OperationsUserBrands: GET /v1/platform/operations/users/{userId}/brands
func (d *Deps) OperationsUserBrands(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT b.id, b.name, b.public_id, bm.role,
		        COALESCE(w.available_tc, 0) AS available_tc,
		        COALESCE(w.reserved_tc, 0) AS reserved_tc,
		        COALESCE(w.captured_tc, 0) AS captured_tc
		 FROM brand_memberships bm
		 JOIN brands b ON b.id = bm.brand_id
		 LEFT JOIN credit_wallets w ON w.brand_id = b.id
		 WHERE bm.user_id = $1 AND b.deleted_at IS NULL
		 ORDER BY b.created_at DESC`, chiParam(r, "userId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	brands := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		brands = append(brands, map[string]any{
			"id": row.anyOrNil("id"), "name": row.strOr("name", "Tanpa nama"),
			"public_id": ptNullableStr(row, "public_id"), "role": row.anyOrNil("role"),
			"available_tc": row.intOr("available_tc", 0), "reserved_tc": row.intOr("reserved_tc", 0),
			"captured_tc": row.intOr("captured_tc", 0),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"brands": brands})
}

// OperationsUserRole: PUT /v1/platform/operations/users/{userId}/role
func (d *Deps) OperationsUserRole(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "users")
	if !ok {
		return
	}
	ctx := r.Context()
	userID := chiParam(r, "userId")
	body := decodeJSON(r)
	role, _, roleValid := opGetString(body, "role", false)
	if !roleValid || !ptContains(opUserRoles, role) {
		writeErr(w, d, errUnprocessable("role harus client atau partner"))
		return
	}
	keepBrands, hasKeep, keepValid := opGetBool(body, "keep_brands")
	if hasKeep && !keepValid {
		writeErr(w, d, errUnprocessable("role harus client atau partner"))
		return
	}
	if admin.ID == userID {
		writeErr(w, d, errBadRequest("Anda tidak dapat mengubah peran akun sendiri"))
		return
	}
	user, err := d.queryRow(ctx,
		"SELECT id, email, display_name, role FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if user == nil {
		writeErr(w, d, errNotFound("User not found"))
		return
	}
	userRow := ptRow(ptNormalizeMap(user))
	targetEmail := userRow.strOr("email", "")
	if d.opIsPlatformAdminUser(ctx, userRow.strOr("id", ""), targetEmail) {
		writeErr(w, d, errForbidden("Akun platform admin tidak dapat diubah perannya"))
		return
	}
	if userRow.strOr("role", "") == role {
		writeJSON(w, http.StatusOK, map[string]any{
			"detail": fmt.Sprintf("Akun sudah berperan sebagai %s.", role), "role": role, "changed": false,
		})
		return
	}
	displayName := userRow.strOr("display_name", "")
	if displayName == "" {
		displayName = targetEmail
	}
	removedBrands := []string{}
	txErr := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "UPDATE users SET role = $1 WHERE id = $2", role, userID); err != nil {
			return err
		}
		// Sinkronkan baris mitra dengan peran baru supaya daftar admin ikut pindah:
		// turun jadi klien → baris mitra dinonaktifkan (media, publikasi, dan
		// riwayat penghasilan tetap utuh), naik jadi mitra → baris yang sudah ada
		// diaktifkan kembali supaya tidak terbentuk mitra duplikat.
		if role == "client" {
			if _, err := tx.Exec(ctx,
				"UPDATE partners SET status = 'inactive', updated_at = NOW() WHERE user_id = $1 AND status <> 'inactive'",
				userID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx,
				`UPDATE partners SET status = 'active', updated_at = NOW()
				   WHERE id = (SELECT id FROM partners WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1)
				     AND status <> 'active'`, userID); err != nil {
				return err
			}
		}
		if err := auth.HealRoleProvisioning(ctx, tx, role, userID, displayName); err != nil {
			return err
		}
		if role == "partner" && !keepBrands {
			brands, rErr := d.operationsRemovableBrands(ctx, tx, userID)
			if rErr != nil {
				return rErr
			}
			for _, brandID := range brands {
				if _, err := tx.Exec(ctx, "UPDATE brands SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", brandID); err != nil {
					return err
				}
			}
			removedBrands = brands
		}
		_, err := tx.Exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, 'platform.user.role_changed', 'user', $3)",
			uuid.NewString(), admin.ID, userID)
		return err
	})
	if txErr != nil {
		writeErr(w, d, txErr)
		return
	}
	invalidatePartnerCache(userID)

	retained := []string{}
	if role == "partner" {
		brandRows, bErr := d.queryRows(ctx,
			`SELECT b.name FROM brands b
			 JOIN brand_memberships bm ON bm.brand_id = b.id
			 WHERE bm.user_id = $1 AND b.deleted_at IS NULL
			 ORDER BY b.name ASC`, userID)
		if bErr != nil {
			writeErr(w, d, bErr)
			return
		}
		for _, raw := range brandRows {
			retained = append(retained, ptRow(ptNormalizeMap(raw)).strOr("name", "Tanpa nama"))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": userID, "email": targetEmail, "display_name": userRow.anyOrNil("display_name"),
		"role": role, "changed": true, "removed_brands": len(removedBrands),
		"retained_brands": retained,
		"note":            "Pengguna perlu login ulang agar sesi memakai peran baru.",
	})
}

// operationsRemovableBrands: parity removableBrands — brand milik user sendiri,
// tanpa transaksi, tanpa anggota lain (hanya boleh dibuang saat peran pindah).
func (d *Deps) operationsRemovableBrands(ctx context.Context, tx pgx.Tx, userID string) ([]string, error) {
	rows, err := tx.Query(ctx,
		`SELECT b.id
		   FROM brands b
		   JOIN brand_memberships bm ON bm.brand_id = b.id AND bm.user_id = $1 AND bm.role = 'owner'
		  WHERE b.deleted_at IS NULL
		    AND (SELECT COUNT(*) FROM brand_memberships x WHERE x.brand_id = b.id) = 1
		    AND NOT EXISTS (SELECT 1 FROM publications p WHERE p.brand_id = b.id)
		    AND NOT EXISTS (SELECT 1 FROM campaigns c WHERE c.brand_id = b.id)
		    AND NOT EXISTS (SELECT 1 FROM credit_wallets w WHERE w.brand_id = b.id
		                      AND (w.available_tc <> 0 OR w.reserved_tc <> 0 OR w.captured_tc <> 0))
		    AND NOT EXISTS (SELECT 1 FROM credit_ledger_entries l WHERE l.brand_id = b.id)`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// OperationsUserPassword: PUT /v1/platform/operations/users/{userId}/password
func (d *Deps) OperationsUserPassword(w http.ResponseWriter, r *http.Request) {
	admin, adminCtx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	if !adminCtx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found"))
		return
	}
	userID := chiParam(r, "userId")
	body := decodeJSON(r)
	newPassword, present, isStr := opGetString(body, "new_password", false)
	if !isStr {
		writeErr(w, d, errUnprocessable("Invalid input: expected string, received "+opReceivedName(body, "new_password")))
		return
	}
	if !present || len(newPassword) < 10 {
		writeErr(w, d, errUnprocessable(opStrTooSmall(10)))
		return
	}
	if len(newPassword) > 200 {
		writeErr(w, d, errUnprocessable(opStrTooBig(200)))
		return
	}
	user, err := d.queryRow(ctx, "SELECT id, email FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if user == nil {
		writeErr(w, d, errNotFound("User tidak ditemukan."))
		return
	}
	userEmail := ptRow(ptNormalizeMap(user)).strOr("email", "")
	hash, hErr := auth.HashPassword(newPassword)
	if hErr != nil {
		writeErr(w, d, hErr)
		return
	}
	txErr := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2", hash, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE user_sessions SET revoked = TRUE WHERE user_id = $1", userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			"UPDATE password_reset_tokens SET consumed_at = NOW() WHERE user_id = $1 AND consumed_at IS NULL", userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, 'platform.user.password_set', 'user', $3)",
			uuid.NewString(), admin.ID, userID)
		return err
	})
	if txErr != nil {
		writeErr(w, d, txErr)
		return
	}
	// Notifikasi pemilik akun (best-effort; parity template password-changed).
	dedupe := fmt.Sprintf("password_changed:%s:%d", userID, time.Now().UnixMilli())
	if _, _, sendErr := emailqueue.SendTransactionalEmail(ctx, d.Pool, emailqueue.SendOptions{
		Template: "password-changed", To: userEmail, RecipientUserID: &userID,
		Event: "security.password_changed", DedupeKey: &dedupe,
		Data: map[string]string{"changed_at": opNowISO()},
	}); sendErr == nil {
		// best-effort: kegagalan email tidak membatalkan aksi
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// OperationsUserStatus: PUT /v1/platform/operations/users/{userId}/status
func (d *Deps) OperationsUserStatus(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	userID := chiParam(r, "userId")
	body := decodeJSON(r)
	isActive, hasActive, activeValid := opGetBool(body, "is_active")
	if !hasActive || !activeValid {
		writeErr(w, d, errUnprocessable("is_active is required"))
		return
	}
	approve, _, _ := opGetBool(body, "approve")
	if admin.ID == userID {
		writeErr(w, d, errBadRequest("You cannot suspend your own account"))
		return
	}
	user, err := d.queryRow(ctx,
		"SELECT id, email, display_name, is_active, approved_at, created_at FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if user == nil {
		writeErr(w, d, errNotFound("User not found"))
		return
	}
	userRow := ptRow(ptNormalizeMap(user))
	if d.opIsPlatformAdminUser(ctx, userRow.strOr("id", ""), userRow.strOr("email", "")) {
		writeErr(w, d, errForbidden("Platform admin accounts are protected"))
		return
	}
	if approve {
		if err := d.exec(ctx,
			"UPDATE users SET is_active = TRUE, approved_at = COALESCE(approved_at, NOW()) WHERE id = $1", userID); err != nil {
			writeErr(w, d, err)
			return
		}
		if err := d.exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'user', $4)",
			uuid.NewString(), admin.ID, "platform.user.approved", userID); err != nil {
			writeErr(w, d, err)
			return
		}
	} else {
		if err := d.exec(ctx, "UPDATE users SET is_active = $1 WHERE id = $2", isActive, userID); err != nil {
			writeErr(w, d, err)
			return
		}
		action := "platform.user.suspended"
		if isActive {
			action = "platform.user.reactivated"
		}
		if err := d.exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'user', $4)",
			uuid.NewString(), admin.ID, action, userID); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	brandCount := d.opCount(ctx, "SELECT COUNT(*) AS count FROM brand_memberships WHERE user_id = $1", userID)
	approvedAt := userRow.anyOrNil("approved_at")
	if approve {
		approvedAt = opNowISO()
	}
	active := isActive
	if approve {
		active = true
	}
	isApproved := approvedAt != nil
	writeJSON(w, http.StatusOK, map[string]any{
		"id": userRow.anyOrNil("id"), "email": userRow.anyOrNil("email"),
		"display_name": userRow.anyOrNil("display_name"), "is_active": active,
		"is_approved": isApproved, "approved_at": approvedAt,
		"brand_count": brandCount, "created_at": userRow.anyOrNil("created_at"),
	})
}

// OperationsUserDailyQuota: GET/PUT /v1/platform/operations/users/{userId}/daily-quota
func (d *Deps) OperationsUserDailyQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		d.operationsUserDailyQuotaGet(w, r)
		return
	}
	d.operationsUserDailyQuotaPut(w, r)
}

func (d *Deps) operationsUserDailyQuotaGet(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	userID := chiParam(r, "userId")
	user, err := d.queryRow(ctx, "SELECT id FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if user == nil {
		writeErr(w, d, errNotFound("User not found"))
		return
	}
	quota, err := d.queryRow(ctx,
		"SELECT user_id, daily_token_limit, updated_by, updated_at FROM user_daily_quotas WHERE user_id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if quota == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"user_id": userID, "daily_token_limit": 0, "updated_by": admin.ID, "updated_at": nil,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any(ptRow(ptNormalizeMap(quota))))
}

func (d *Deps) operationsUserDailyQuotaPut(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	userID := chiParam(r, "userId")
	body := decodeJSON(r)
	limit, hasLimit, limitIsNum := opGetNum(body, "daily_token_limit")
	if !hasLimit || !limitIsNum || !opIsInt(limit) || limit < 0 || limit > 100000000 {
		writeErr(w, d, errUnprocessable("daily_token_limit must be an integer between 0 and 100000000"))
		return
	}
	user, err := d.queryRow(ctx, "SELECT id FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if user == nil {
		writeErr(w, d, errNotFound("User not found"))
		return
	}
	if err := d.exec(ctx,
		"INSERT INTO user_daily_quotas (user_id, daily_token_limit, updated_by) VALUES ($1, $2, $3) ON CONFLICT (user_id) DO UPDATE SET daily_token_limit = EXCLUDED.daily_token_limit, updated_by = EXCLUDED.updated_by",
		userID, int(limit), admin.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(ctx, admin.ID, "platform.user.daily_quota_updated", "user", userID)
	quota, err := d.queryRow(ctx,
		"SELECT user_id, daily_token_limit, updated_by, updated_at FROM user_daily_quotas WHERE user_id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any(ptRow(ptNormalizeMap(quota))))
}

// opWriterDefaultQuota: parity DEFAULT_QUOTA = 3.
const opWriterDefaultQuota = 3

// OperationsUserGenerateQuota: GET/PUT /v1/platform/operations/users/{userId}/generate-quota
func (d *Deps) OperationsUserGenerateQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		d.operationsUserGenerateQuotaGet(w, r)
		return
	}
	d.operationsUserGenerateQuotaPut(w, r)
}

func (d *Deps) operationsUserGenerateQuotaGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	userID := chiParam(r, "userId")
	user, err := d.queryRow(ctx, "SELECT id, writer_generate_quota FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if user == nil {
		writeErr(w, d, errNotFound("User not found"))
		return
	}
	row := ptRow(ptNormalizeMap(user))
	effective := opWriterDefaultQuota
	if row.has("writer_generate_quota") {
		if quota := int(row.numOr("writer_generate_quota")); quota >= 1 {
			effective = quota
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": userID, "writer_generate_quota": row.anyOrNil("writer_generate_quota"),
		"effective": effective,
	})
}

func (d *Deps) operationsUserGenerateQuotaPut(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	userID := chiParam(r, "userId")
	body := decodeJSON(r)
	// Parity: `value !== null` — hanya null eksplisit yang berarti "kembali ke
	// default"; key absen (undefined) maupun non-integer sama-sama 422.
	rawValue, hasValue := body["writer_generate_quota"]
	var valueArg any
	effective := opWriterDefaultQuota
	quotaOut := any(nil)
	if hasValue && rawValue == nil {
		// null → default (tidak ada validasi lanjutan)
	} else {
		num, isNum := rawValue.(float64)
		if !isNum {
			if i, isInt := rawValue.(int); isInt {
				num, isNum = float64(i), true
			}
		}
		if !isNum || !opIsInt(num) || num < 1 || num > 999 {
			writeErr(w, d, errUnprocessable("writer_generate_quota must be an integer between 1 and 999, or null for the default (3)"))
			return
		}
		valueArg = int(num)
		quotaOut = int(num)
		effective = int(num)
	}
	user, err := d.queryRow(ctx, "SELECT id FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if user == nil {
		writeErr(w, d, errNotFound("User not found"))
		return
	}
	if err := d.exec(ctx, "UPDATE users SET writer_generate_quota = $1 WHERE id = $2", valueArg, userID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(ctx, admin.ID, "platform.user.generate_quota_updated", "user", userID)
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": userID, "writer_generate_quota": quotaOut, "effective": effective,
	})
}

// ---------- variant batches ----------

var opVariantBatchStatuses = []string{"queued", "processing", "completed", "partial", "failed"}

// OperationsVariantBatches: GET /v1/platform/operations/variant-batches
func (d *Deps) OperationsVariantBatches(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	status := q.Get("status")
	if status != "" && !ptContains(opVariantBatchStatuses, status) {
		writeErr(w, d, errUnprocessable("Invalid batch status filter"))
		return
	}
	search := strings.TrimSpace(q.Get("q"))

	if q.Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition",
			`attachment; filename="tulisio-variant-batches-`+opDateOnly()+`.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(d.opVariantBatchesCSV(ctx, status, search)))
		return
	}
	limit := clampInt(q.Get("limit"), 20, 1, 100)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	batches, total := d.opVariantBatchesJSON(ctx, status, search, limit, offset)
	writeJSON(w, http.StatusOK, map[string]any{
		"batches": batches, "total": total, "limit": limit, "offset": offset,
	})
}

// OperationsVariantBatchDetail: GET /v1/platform/operations/variant-batches/{batchId}
func (d *Deps) OperationsVariantBatchDetail(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	payload, err := d.opVariantBatchDetailJSON(r.Context(), chiParam(r, "batchId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// OperationsVariantBatchRetry: POST /v1/platform/operations/variant-batches/{batchId}/retry
func (d *Deps) OperationsVariantBatchRetry(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ctx := r.Context()
	batchID := chiParam(r, "batchId")
	batch, err := d.queryRow(ctx,
		"SELECT id, brand_id, user_id, mode, status, source_material, source_document_id FROM publication_variant_batches WHERE id = $1 LIMIT 1", batchID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if batch == nil {
		writeErr(w, d, errNotFound("Variant batch not found"))
		return
	}
	batchRow := ptRow(ptNormalizeMap(batch))
	mode := batchRow.strOr("mode", "")
	batchStatus := batchRow.strOr("status", "")
	if batchStatus != "failed" && batchStatus != "partial" {
		writeErr(w, d, errConflict(fmt.Sprintf("Batch tidak bisa di-retry (status: %s)", batchStatus)))
		return
	}
	failedVariants, err := d.queryRows(ctx,
		`SELECT v.id, v.media_id, v.angle_id, v.angle_label, m.name AS media_name,
		        (SELECT j.payload FROM jobs j
		          WHERE j.kind = 'publication_variant_generation' AND j.payload->>'variantId' = v.id
		          ORDER BY j.created_at DESC LIMIT 1) AS last_payload
		 FROM publication_variants v
		 LEFT JOIN media m ON m.id = v.media_id
		 WHERE v.batch_id = $1 AND v.status = 'failed'
		 ORDER BY v.created_at ASC`, batchID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if len(failedVariants) == 0 {
		writeErr(w, d, errConflict("Tidak ada varian gagal untuk di-retry"))
		return
	}
	sourceDocumentID := batchRow.strOr("source_document_id", "")
	var paraphraseSource any
	if mode == "paraphrase" && sourceDocumentID != "" {
		if version, vErr := d.queryRow(ctx,
			"SELECT content FROM content_versions WHERE document_id = $1 ORDER BY version DESC LIMIT 1", sourceDocumentID); vErr == nil && version != nil {
			paraphraseSource = ptRow(ptNormalizeMap(version)).anyOrNil("content")
		}
	}
	brandID := batchRow.strOr("brand_id", "")
	userID := batchRow.strOr("user_id", "")
	retried := []string{}
	for _, raw := range failedVariants {
		variant := ptRow(ptNormalizeMap(raw))
		variantID := variant.strOr("id", "")
		angleID := variant.strOr("angle_id", "")
		mediaName := variant.strOr("media_name", "Media")
		angleLabel := variant.strOr("angle_label", "")
		angleInstruction := ""
		if angle, found := opAngleByID(angleID); found {
			if angleLabel == "" {
				angleLabel = angle.Label
			}
			angleInstruction = angle.Instruction
		} else if angleLabel == "" {
			angleLabel = "Sudut"
			angleInstruction = "Sudut"
		}

		var payload map[string]any
		if lastPayload := variant.anyOrNil("last_payload"); lastPayload != nil {
			payload = opParseJobPayload(lastPayload)
			payload["batchId"] = batchID
			payload["variantId"] = variantID
		} else {
			var sourceMaterial any
			if mode == "generate" {
				sourceMaterial = batchRow.anyOrNil("source_material")
			}
			var sourceArticleText any
			if mode == "paraphrase" {
				sourceArticleText = paraphraseSource
			}
			payload = map[string]any{
				"batchId": batchID, "brandId": brandID, "userId": userID, "mode": mode,
				"campaignId": nil, "connectionId": nil, "publishStatus": "draft",
				"scheduledAt": nil, "skipStandardCheck": false,
				"variantId": variantID, "mediaId": variant.strOr("media_id", ""), "mediaName": mediaName,
				"angleId": angleID, "angleLabel": angleLabel, "angleInstruction": angleInstruction,
				"sourceMaterial": sourceMaterial, "sourceArticleText": sourceArticleText,
			}
		}
		if err := d.exec(ctx,
			"UPDATE publication_variants SET status = 'queued', error_message = NULL, updated_at = NOW() WHERE id = $1", variantID); err != nil {
			writeErr(w, d, err)
			return
		}
		if _, err := d.opEnqueueJob(ctx, "publication_variant_generation", payload, 3); err != nil {
			writeErr(w, d, err)
			return
		}
		retried = append(retried, variantID)
	}
	if err := d.exec(ctx, "UPDATE publication_variant_batches SET status = 'queued' WHERE id = $1", batchID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "batch_id": batchID, "retried": len(retried)})
}
