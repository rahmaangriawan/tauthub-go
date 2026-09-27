package brandflow

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// integrations.go: port app/v1/brands/[brandId]/{cms-connections,
// notification-webhooks, ga4}/**/route.ts.

// ---------- guard platform admin (parity isPlatformAdmin) ----------

// bsIsPlatformAdmin: platform_admins menang; fallback env PLATFORM_ADMIN_EMAILS.
func (d *Deps) bsIsPlatformAdmin(ctx context.Context, userID, email string) bool {
	row, err := d.queryRow(ctx, "SELECT user_id FROM platform_admins WHERE user_id = $1 LIMIT 1", userID)
	if err != nil {
		return false
	}
	if row != nil {
		return true
	}
	for _, allowed := range d.Cfg.AdminEmails {
		if allowed != "" && strings.EqualFold(allowed, email) {
			return true
		}
	}
	return false
}

// ---------- CMS connections ----------

// cmsConnectionColumns: parity CONNECTION_COLUMNS.
const cmsConnectionColumns = "id, brand_id, partner_id, website_id, provider, name, base_url, username, status, last_test_status, last_test_message, created_at, updated_at"

// bsConnectionJSON: port connectionJson — kolom SELECT sudah tanpa sealed_secret.
func bsConnectionJSON(row bsRow) map[string]any {
	if row == nil {
		return nil
	}
	return bsRowJSON(row)
}

// bsScopedConnection: koneksi milik brand; NotFound "CMS connection not found".
func (d *Deps) bsScopedConnection(ctx context.Context, brandID, connectionID string) (bsRow, error) {
	row, err := d.queryRow(ctx,
		"SELECT "+cmsConnectionColumns+" FROM cms_connections WHERE id = $1 AND brand_id = $2 LIMIT 1",
		connectionID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("CMS connection not found")
	}
	return row, nil
}

// cmsError: port cmsError — NotFound 404, Conflict 409, duplicate 409.
func (d *Deps) cmsError(w http.ResponseWriter, err error) {
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "duplicate") || strings.Contains(lower, "uq_cms_connection") ||
		strings.Contains(lower, "23505") {
		httpx.Error(w, http.StatusConflict, "CMS connection name already exists")
		return
	}
	d.writeErr(w, err)
}

// CmsConnectionsList: GET /v1/brands/{brandId}/cms-connections
func (d *Deps) CmsConnectionsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.cmsError(w, err)
		return
	}
	rawBrandID := bfBrand.ID
	items, err := d.queryRows(r.Context(),
		"SELECT "+cmsConnectionColumns+" FROM cms_connections WHERE brand_id = $1 AND status = 'active' ORDER BY created_at DESC",
		rawBrandID)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, bsConnectionJSON(item))
	}
	writeJSON(w, http.StatusOK, out)
}

// CmsConnectionCreate: POST /v1/brands/{brandId}/cms-connections
func (d *Deps) CmsConnectionCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	// Manajemen koneksi sisi Tulisio (platform admin); klien hanya mengirim.
	if !d.bsIsPlatformAdmin(r.Context(), user.ID, user.Email) {
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.cmsError(w, err)
		return
	}
	rawBrandID := bfBrand.ID
	body := decodeJSON(r)

	// cmsCreateSchema: name 2..120, provider enum wordpress|webhook default
	// wordpress, base_url 8..500, username 1..160 nullable optional,
	// application_password 1..500 optional, secret 16..500 optional,
	// website_id string nullable optional + superRefine per provider.
	name, err := bsZodStringRaw(body, "name", 2, 120, false, false)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	provider := "wordpress"
	if raw, ok := body["provider"]; ok {
		s, isStr := raw.(string)
		if !isStr || (s != "wordpress" && s != "webhook") {
			// null pun ditolak zod (default hanya berlaku untuk undefined).
			d.cmsError(w, errUnprocessable("Invalid option: expected one of \"wordpress\"|\"webhook\""))
			return
		}
		provider = s
	}
	baseURL, err := bsZodStringRaw(body, "base_url", 8, 500, false, false)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	var username *string
	if raw, ok := body["username"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			d.cmsError(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
		if len([]rune(s)) < 1 || len([]rune(s)) > 160 {
			if len([]rune(s)) > 160 {
				d.cmsError(w, errUnprocessable("Too big: expected string to have <=160 characters"))
				return
			}
			d.cmsError(w, errUnprocessable("Too small: expected string to have >=1 characters"))
			return
		}
		username = &s
	}
	applicationPassword := ""
	if raw, ok := body["application_password"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			d.cmsError(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
		if s == "" {
			d.cmsError(w, errUnprocessable("Too small: expected string to have >=1 characters"))
			return
		}
		if len([]rune(s)) > 500 {
			d.cmsError(w, errUnprocessable("Too big: expected string to have <=500 characters"))
			return
		}
		applicationPassword = s
	}
	secret := ""
	if raw, ok := body["secret"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			d.cmsError(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
		if len([]rune(s)) < 16 {
			d.cmsError(w, errUnprocessable("Too small: expected string to have >=16 characters"))
			return
		}
		if len([]rune(s)) > 500 {
			d.cmsError(w, errUnprocessable("Too big: expected string to have <=500 characters"))
			return
		}
		secret = s
	}
	var websiteID *string
	if raw, ok := body["website_id"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			d.cmsError(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
		websiteID = &s
	}
	// superRefine
	if provider == "wordpress" {
		if username == nil {
			d.cmsError(w, errUnprocessable("username is required for WordPress connections"))
			return
		}
		if _, has := body["application_password"]; !has || applicationPassword == "" {
			d.cmsError(w, errUnprocessable("application_password is required for WordPress connections"))
			return
		}
	} else if secret == "" {
		d.cmsError(w, errUnprocessable("secret is required for webhook connections"))
		return
	}

	if websiteID != nil && *websiteID != "" {
		site, err := d.queryRow(r.Context(),
			"SELECT id FROM websites WHERE id = $1 AND brand_id = $2", *websiteID, rawBrandID)
		if err != nil {
			d.cmsError(w, err)
			return
		}
		if site == nil {
			httpx.Error(w, http.StatusNotFound, "Website not found")
			return
		}
	}
	if provider == "wordpress" {
		active, err := d.queryRow(r.Context(),
			"SELECT id FROM cms_connections WHERE brand_id = $1 AND provider = 'wordpress' AND status = 'active' LIMIT 1", rawBrandID)
		if err != nil {
			d.cmsError(w, err)
			return
		}
		if active != nil {
			httpx.Error(w, http.StatusConflict, "Brand already has an active WordPress connection")
			return
		}
	}

	connectionID := uuid.NewString()
	plain := secret
	if provider == "wordpress" {
		plain = bsStripSpaces(applicationPassword)
	}
	sealed, sealErr := d.bsSealV3ForEntity(plain, connectionID)
	if sealErr != nil {
		d.cmsError(w, sealErr)
		return
	}
	var usernameArg any
	if username != nil {
		usernameArg = strings.TrimSpace(*username)
	}
	// Parity `value.website_id ?? null` — string kosong tetap disimpan kosong.
	var websiteArg any
	if websiteID != nil {
		websiteArg = *websiteID
	}
	if err := d.exec(r.Context(),
		"INSERT INTO cms_connections (id, brand_id, website_id, provider, name, base_url, username, sealed_secret, enc_key_ver, status, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, 'active', $9)",
		connectionID, rawBrandID, websiteArg, provider, name,
		strings.TrimRight(baseURL, "/"), usernameArg, sealed, user.ID); err != nil {
		d.cmsError(w, err)
		return
	}
	d.audit(r.Context(), rawBrandID, user.ID, "cms.connection.created", "cms_connection", connectionID)
	created, err := d.queryRow(r.Context(),
		"SELECT "+cmsConnectionColumns+" FROM cms_connections WHERE id = $1", connectionID)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, bsConnectionJSON(created))
}

// CmsConnectionGet: GET /v1/brands/{brandId}/cms-connections/{connectionId}
func (d *Deps) CmsConnectionGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.cmsError(w, err)
		return
	}
	row, err := d.bsScopedConnection(r.Context(), bfBrand.ID, chiParam(r, "connectionId"))
	if err != nil {
		d.cmsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bsConnectionJSON(row))
}

// CmsConnectionUpdate: PUT /v1/brands/{brandId}/cms-connections/{connectionId}
func (d *Deps) CmsConnectionUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if !d.bsIsPlatformAdmin(r.Context(), user.ID, user.Email) {
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.cmsError(w, err)
		return
	}
	rawBrandID := bfBrand.ID
	connectionID := chiParam(r, "connectionId")
	body := decodeJSON(r)

	// cmsUpdateSchema: name 2..120 optional, base_url 8..500 optional,
	// username 1..160 nullable optional, secret 16..500 optional.
	var nameArg any
	if raw, ok := body["name"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			d.cmsError(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
		if len(s) < 2 {
			d.cmsError(w, errUnprocessable("Too small: expected string to have >=2 characters"))
			return
		}
		if len(s) > 120 {
			d.cmsError(w, errUnprocessable("Too big: expected string to have <=120 characters"))
			return
		}
		nameArg = s
	}
	var baseURLArg any
	if raw, ok := body["base_url"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			d.cmsError(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
		if len(s) < 8 {
			d.cmsError(w, errUnprocessable("Too small: expected string to have >=8 characters"))
			return
		}
		if len(s) > 500 {
			d.cmsError(w, errUnprocessable("Too big: expected string to have <=500 characters"))
			return
		}
		baseURLArg = strings.TrimRight(s, "/")
	}
	var usernameArg any
	if raw, ok := body["username"]; ok {
		if raw == nil {
			usernameArg = nil
		} else if s, isStr := raw.(string); isStr {
			if len(s) < 1 {
				d.cmsError(w, errUnprocessable("Too small: expected string to have >=1 characters"))
				return
			}
			if len(s) > 160 {
				d.cmsError(w, errUnprocessable("Too big: expected string to have <=160 characters"))
				return
			}
			usernameArg = s
		} else {
			d.cmsError(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
	}
	if _, err := d.bsScopedConnection(r.Context(), rawBrandID, connectionID); err != nil {
		d.cmsError(w, err)
		return
	}

	query := "UPDATE cms_connections SET name = COALESCE($1, name), base_url = COALESCE($2, base_url), username = COALESCE($3, username)"
	args := []any{nameArg, baseURLArg, usernameArg}
	if raw, ok := body["secret"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			d.cmsError(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
		if len(s) < 16 {
			d.cmsError(w, errUnprocessable("Too small: expected string to have >=16 characters"))
			return
		}
		if len(s) > 500 {
			d.cmsError(w, errUnprocessable("Too big: expected string to have <=500 characters"))
			return
		}
		sealed, sealErr := d.bsSealV3ForEntity(s, connectionID)
		if sealErr != nil {
			d.cmsError(w, sealErr)
			return
		}
		args = append(args, sealed)
		query += ", sealed_secret = $" + itoa(len(args)) + ", enc_key_ver = 1"
	}
	args = append(args, connectionID, rawBrandID)
	query += " WHERE id = $" + itoa(len(args)-1) + " AND brand_id = $" + itoa(len(args))
	if err := d.exec(r.Context(), query, args...); err != nil {
		d.cmsError(w, err)
		return
	}
	refreshed, err := d.bsScopedConnection(r.Context(), rawBrandID, connectionID)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bsConnectionJSON(refreshed))
}

// CmsConnectionDelete: DELETE /v1/brands/{brandId}/cms-connections/{connectionId}
func (d *Deps) CmsConnectionDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if !d.bsIsPlatformAdmin(r.Context(), user.ID, user.Email) {
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.cmsError(w, err)
		return
	}
	rawBrandID := bfBrand.ID
	connectionID := chiParam(r, "connectionId")
	if _, err := d.bsScopedConnection(r.Context(), rawBrandID, connectionID); err != nil {
		d.cmsError(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE cms_connections SET status = 'revoked', name = CONCAT(name, ' (revoked ', LEFT(id, 8), ')') WHERE id = $1 AND brand_id = $2",
		connectionID, rawBrandID); err != nil {
		d.cmsError(w, err)
		return
	}
	d.audit(r.Context(), rawBrandID, user.ID, "cms.connection.revoked", "cms_connection", connectionID)
	refreshed, err := d.bsScopedConnection(r.Context(), rawBrandID, connectionID)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bsConnectionJSON(refreshed))
}

// CmsConnectionTest: POST /v1/brands/{brandId}/cms-connections/{connectionId}/test
func (d *Deps) CmsConnectionTest(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if !d.bsIsPlatformAdmin(r.Context(), user.ID, user.Email) {
		httpx.Error(w, http.StatusNotFound, "Resource not found")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.cmsError(w, err)
		return
	}
	rawBrandID := bfBrand.ID
	connectionID := chiParam(r, "connectionId")
	connection, err := d.queryRow(r.Context(),
		"SELECT * FROM cms_connections WHERE id = $1 AND brand_id = $2 LIMIT 1", connectionID, rawBrandID)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	if connection == nil {
		httpx.Error(w, http.StatusNotFound, "CMS connection not found")
		return
	}
	status, message := d.bsTestCMSConnection(r.Context(), connection)
	if err := d.exec(r.Context(),
		"UPDATE cms_connections SET last_test_status = $1, last_test_message = $2 WHERE id = $3",
		status, message, connectionID); err != nil {
		d.cmsError(w, err)
		return
	}
	code := http.StatusOK
	if status != "ok" {
		code = http.StatusUnprocessableEntity
	}
	writeJSON(w, code, map[string]any{"status": status, "message": message})
}

// ---------- notification webhooks ----------

// bsWebhookJSON: parity webhookJson lib/partner/notification-webhook.ts.
func bsWebhookJSON(row bsRow) map[string]any {
	return map[string]any{
		"id":         row.raw("id"),
		"url":        row.raw("url"),
		"active":     row.boolOr("active", false),
		"created_at": row.timePtr("created_at"),
		"updated_at": row.timePtr("updated_at"),
	}
}

// bsScopedWebhook: webhook milik user; NotFound "Webhook not found".
func (d *Deps) bsScopedWebhook(ctx context.Context, userID, webhookID string) (bsRow, error) {
	row, err := d.queryRow(ctx,
		"SELECT * FROM notification_webhooks WHERE id = $1 AND user_id = $2 LIMIT 1", webhookID, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Webhook not found")
	}
	return row, nil
}

// NotificationWebhookList: GET /v1/brands/{brandId}/notification-webhooks
func (d *Deps) NotificationWebhookList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	items, err := d.queryRows(r.Context(),
		"SELECT id, url, active, created_at, updated_at FROM notification_webhooks WHERE user_id = $1 ORDER BY created_at DESC",
		user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, bsWebhookJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": out})
}

// NotificationWebhookCreate: POST /v1/brands/{brandId}/notification-webhooks
func (d *Deps) NotificationWebhookCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	// webhookSchema: url trim url() max 500, secret min16 max200 nullable optional.
	urlValue, err := bsZodURL(body, "url", 500, false)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	var secretArg any
	if raw, ok := body["secret"]; ok {
		if raw == nil {
			secretArg = nil
		} else if s, isStr := raw.(string); isStr {
			if len(s) < 16 {
				d.writeErr(w, errUnprocessable("Too small: expected string to have >=16 characters"))
				return
			}
			if len(s) > 200 {
				d.writeErr(w, errUnprocessable("Too big: expected string to have <=200 characters"))
				return
			}
			secretArg = s
		} else {
			d.writeErr(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO notification_webhooks (id, user_id, url, secret, active) VALUES ($1, $2, $3, $4, TRUE)",
		id, user.ID, urlValue, secretArg); err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id, url, active, created_at, updated_at FROM notification_webhooks WHERE id = $1 LIMIT 1", id)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"webhook": bsWebhookJSON(row)})
}

// NotificationWebhookUpdate: PUT /v1/brands/{brandId}/notification-webhooks/{webhookId}
func (d *Deps) NotificationWebhookUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	webhookID := chiParam(r, "webhookId")
	if _, err := d.bsScopedWebhook(r.Context(), user.ID, webhookID); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	// updateSchema: url optional, secret nullable optional, active optional.
	var urlArg any
	if raw, ok := body["url"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			d.writeErr(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
		value, err := bsZodURL(map[string]any{"url": s}, "url", 500, false)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		urlArg = value
	}
	var secretArg any
	if raw, ok := body["secret"]; ok {
		if raw == nil {
			secretArg = nil
		} else if s, isStr := raw.(string); isStr {
			if len(s) < 16 {
				d.writeErr(w, errUnprocessable("Too small: expected string to have >=16 characters"))
				return
			}
			if len(s) > 200 {
				d.writeErr(w, errUnprocessable("Too big: expected string to have <=200 characters"))
				return
			}
			secretArg = s
		} else {
			d.writeErr(w, errUnprocessable("Invalid input: expected string, received "+bsZodTypeName(raw)))
			return
		}
	}
	var activeArg any
	if raw, ok := body["active"]; ok {
		if b, isBool := raw.(bool); isBool {
			activeArg = b
		} else {
			d.writeErr(w, errUnprocessable("Invalid input: expected boolean, received "+bsZodTypeName(raw)))
			return
		}
	}
	if err := d.exec(r.Context(),
		"UPDATE notification_webhooks SET url = COALESCE($1, url), secret = COALESCE($2, secret), active = COALESCE($3, active), updated_at = NOW() WHERE id = $4 AND user_id = $5",
		urlArg, secretArg, activeArg, webhookID, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.bsScopedWebhook(r.Context(), user.ID, webhookID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhook": bsWebhookJSON(row)})
}

// NotificationWebhookDelete: DELETE /v1/brands/{brandId}/notification-webhooks/{webhookId}
func (d *Deps) NotificationWebhookDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	webhookID := chiParam(r, "webhookId")
	if _, err := d.bsScopedWebhook(r.Context(), user.ID, webhookID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"DELETE FROM notification_webhooks WHERE id = $1 AND user_id = $2", webhookID, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- GA4 ----------

// Ga4Get: GET /v1/brands/{brandId}/ga4
func (d *Deps) Ga4Get(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	rawBrandID := bfBrand.ID
	config, err := d.queryRow(r.Context(),
		"SELECT id, measurement_id, event_mapping, enabled, consent_mode, updated_at FROM ga4_configs WHERE brand_id = $1 LIMIT 1",
		rawBrandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	mapping := map[string]any{}
	if config != nil {
		if parsed, ok := config.jsonField("event_mapping").(map[string]any); ok {
			mapping = parsed
		}
	}
	var ga4 any
	if config != nil {
		ga4 = map[string]any{
			"id":             config.raw("id"),
			"measurement_id": config.raw("measurement_id"),
			"enabled":        config.boolOr("enabled", false),
			"consent_mode":   config.raw("consent_mode"),
			"event_mapping":  mapping,
			"updated_at":     config.timePtr("updated_at"),
		}
	}
	defaults := map[string]any{}
	for k, v := range bsGA4EventMap {
		defaults[k] = v
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ga4":                       ga4,
		"default_event_definitions": defaults,
	})
}

// ---------- util zod ----------

// bsZodTypeName: parity nama tipe yang diterima zod (pesan invalid_type).
func bsZodTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, int, int64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

// bsZodString: field string wajib/opsional dengan trim + batas panjang.
func bsZodString(body map[string]any, key string, min, max int, optional, nullable bool) (string, error) {
	return bsZodStringMode(body, key, min, max, optional, nullable, true)
}

// bsZodStringRaw: seperti bsZodString tapi TANPA trim (schema _cms.ts tidak
// memakai .trim(); nilai disimpan apa adanya).
func bsZodStringRaw(body map[string]any, key string, min, max int, optional, nullable bool) (string, error) {
	return bsZodStringMode(body, key, min, max, optional, nullable, false)
}

func bsZodStringMode(body map[string]any, key string, min, max int, optional, nullable, trim bool) (string, error) {
	raw, ok := body[key]
	if !ok {
		if optional {
			return "", nil
		}
		return "", errUnprocessable("Invalid input: expected string, received undefined")
	}
	if raw == nil {
		if optional || nullable {
			return "", nil
		}
		return "", errUnprocessable("Invalid input: expected string, received null")
	}
	s, isStr := raw.(string)
	if !isStr {
		return "", errUnprocessable("Invalid input: expected string, received " + bsZodTypeName(raw))
	}
	value := s
	if trim {
		value = strings.TrimSpace(s)
	}
	if min > 0 && len([]rune(value)) < min {
		return "", errUnprocessable("Too small: expected string to have >=" + itoa(min) + " characters")
	}
	if max > 0 && len([]rune(value)) > max {
		return "", errUnprocessable("Too big: expected string to have <=" + itoa(max) + " characters")
	}
	return value, nil
}

// bsZodURL: field string trim url() max 500 (parity z.string().trim().url().max()).
// Urutan check zod: trim → url → max.
func bsZodURL(body map[string]any, key string, max int, nullable bool) (string, error) {
	raw, ok := body[key]
	if !ok {
		return "", errUnprocessable("Invalid input: expected string, received undefined")
	}
	if raw == nil {
		if nullable {
			return "", nil
		}
		return "", errUnprocessable("Invalid input: expected string, received null")
	}
	s, isStr := raw.(string)
	if !isStr {
		return "", errUnprocessable("Invalid input: expected string, received " + bsZodTypeName(raw))
	}
	trimmed := strings.TrimSpace(s)
	if !bsIsURL(trimmed) {
		return "", errUnprocessable("Invalid URL")
	}
	if len([]rune(trimmed)) > max {
		return "", errUnprocessable("Too big: expected string to have <=" + itoa(max) + " characters")
	}
	return trimmed, nil
}

// derefPtr: pointer string → string.
func derefPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
