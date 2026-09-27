package platform

import (
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/email"
)

// Modul settings admin — port app/v1/platform/settings/** (8 endpoint).
// Semua admin-only (parity isPlatformAdmin → 404 "Resource not found").
//
// SMTP password, Turnstile secret, dan PageSpeed API key disimpan ter-seal
// (t2:) dan tidak pernah dikembalikan ke UI.

// ---------- email ----------

// SettingsEmailGet: GET /v1/platform/settings/email
func (d *Deps) SettingsEmailGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row := d.stSMTPRow(r.Context())
	if row == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"smtp_configured":     false,
			"host":                nil,
			"port":                587,
			"secure":              false,
			"username":            nil,
			"password_configured": false,
			"from_email":          "no-reply@tauthub.com",
			"from_name":           "Tauthub",
			"enabled":             false,
			"last_test_at":        nil,
			"last_test_ok":        nil,
			"last_test_message":   nil,
		})
		return
	}
	host := row.strOr("host", "")
	sealed := row.strOr("password_sealed", "")
	var lastTestOK any
	if row.has("last_test_ok") {
		lastTestOK = row.boolOr("last_test_ok", false)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"smtp_configured":     host != "" && sealed != "",
		"host":                nullIfEmpty(host),
		"port":                row.intOr("port", 587),
		"secure":              row.boolOr("secure", false),
		"username":            ptNullableStr(row, "username"),
		"password_configured": sealed != "",
		"from_email":          aiOrEmpty(row.strOr("from_email", ""), "no-reply@tauthub.com"),
		"from_name":           aiOrEmpty(row.strOr("from_name", ""), "Tauthub"),
		"enabled":             row.boolOr("enabled", false),
		"last_test_at":        row.timePtr("last_test_at"),
		"last_test_ok":        lastTestOK,
		"last_test_message":   ptNullableStr(row, "last_test_message"),
	})
}

// SettingsEmailPut: PUT /v1/platform/settings/email
func (d *Deps) SettingsEmailPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	// parity body(request, smtpUpdateSchema): JSON rusak/body kosong →
	// "Invalid input: expected object, received null" (422).
	body, okBody := aiStrictObject(w, d, r)
	if !okBody {
		return
	}
	// Schema (parity smtpUpdateSchema): semua field optional.
	host, hasHost, err := aiStringField(body, "host", 0, 200, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	port, hasPort, err := aiIntField(body, "port", 1, 65535)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	secure, hasSecure, err := aiBoolField(body, "secure")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	username, hasUsername, err := aiStringField(body, "username", 0, 200, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	password, hasPassword, err := aiStringField(body, "password", 0, 500, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// parity z.string().trim().email().max(200).optional(): cek format lebih dulu,
	// baru panjang.
	fromEmail, hasFromEmail, err := aiStringField(body, "from_email", 0, 0, true, false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if hasFromEmail && !stEmailRe.MatchString(fromEmail) {
		writeErr(w, d, stInvalidEmailErr())
		return
	}
	if hasFromEmail && len([]rune(fromEmail)) > 200 {
		writeErr(w, d, errUnprocessable(ptZStringTooBig(200)))
		return
	}
	fromName, hasFromName, err := aiStringField(body, "from_name", 0, 120, true, false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	enabled, hasEnabled, err := aiBoolField(body, "enabled")
	if err != nil {
		writeErr(w, d, err)
		return
	}

	existing := d.stSMTPRow(ctx)
	var existingSealed string
	if existing != nil {
		existingSealed = existing.strOr("password_sealed", "")
	}
	var nextPassword any
	if hasPassword && password != "" {
		sealed, sErr := d.sealT2(password)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		nextPassword = sealed
	} else if existingSealed != "" {
		nextPassword = existingSealed
	}
	nextEnabled := false
	if hasEnabled {
		nextEnabled = enabled
	} else if existing != nil {
		nextEnabled = existing.boolOr("enabled", false)
	}
	nextPort := 587
	if hasPort {
		nextPort = port
	}
	nextSecure := false
	if hasSecure {
		nextSecure = secure
	}
	// parity `options.host ?? null` — field yang DIKIRIM tersimpan apa adanya
	// (termasuk string kosong); hanya field yang tidak dikirim jadi NULL.
	var nextHost any
	if hasHost {
		nextHost = host
	}
	var nextUsername any
	if hasUsername {
		nextUsername = username
	}
	nextFromEmail := "no-reply@tauthub.com"
	if hasFromEmail {
		nextFromEmail = fromEmail
	}
	nextFromName := "Tauthub"
	if hasFromName {
		nextFromName = fromName
	}

	if existing != nil {
		if err := d.exec(ctx,
			`UPDATE smtp_settings SET
			   host = $1, port = $2, secure = $3, username = $4, password_sealed = $5,
			   from_email = $6, from_name = $7, enabled = $8, updated_at = NOW(), updated_by = $9
			 WHERE id = 'default'`,
			nextHost, nextPort, nextSecure, nextUsername, nextPassword,
			nextFromEmail, nextFromName, nextEnabled, user.ID); err != nil {
			writeErr(w, d, err)
			return
		}
	} else if err := d.exec(ctx,
		`INSERT INTO smtp_settings (id, provider, host, port, secure, username, password_sealed, from_email, from_name, enabled, updated_at, updated_by)
		 VALUES ('default','smtp',$1,$2,$3,$4,$5,$6,$7,$8,NOW(),$9)`,
		nextHost, nextPort, nextSecure, nextUsername, nextPassword,
		nextFromEmail, nextFromName, nextEnabled, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}

	// Catatan: cache config internal/email (TTL 60s) tidak punya fungsi
	// invalidasi yang diekspor, jadi proses ini bisa memakai config lama maks
	// 60 detik setelah simpan. Handler test-connection/test-send membaca ulang
	// smtp_settings langsung dari DB sehingga hasil uji selalu memakai nilai
	// yang baru disimpan.
	d.audit(ctx, user.ID, "smtp.config_updated", "smtp_settings", "default")

	updated := d.stSMTPRow(ctx)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"smtp_configured": updated != nil &&
			updated.strOr("host", "") != "" && updated.strOr("password_sealed", "") != "",
		"enabled": updated != nil && updated.boolOr("enabled", false),
	})
}

// SettingsEmailDeliveries: GET /v1/platform/settings/email/deliveries
func (d *Deps) SettingsEmailDeliveries(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := clampInt(q.Get("limit"), 50, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	status := strings.TrimSpace(q.Get("status"))

	where := ""
	params := []any{}
	if status != "" && aiContains([]string{"queued", "sending", "sent", "failed", "rate_limited", "skipped"}, status) {
		where = "WHERE status = $1"
		params = append(params, status)
	}
	// LIMIT/OFFSET memakai placeholder setelah param filter.
	limitIdx := len(params) + 1
	sql := `SELECT id, template, recipient_email, user_id, reference_id, event, status, attempts, max_attempts, next_attempt_at, provider, provider_message_id, error_code, error_message, sent_at, created_at
		FROM email_deliveries ` + where + ` ORDER BY created_at DESC LIMIT $` + ptItoa(limitIdx) + ` OFFSET $` + ptItoa(limitIdx+1)
	items, err := d.queryRows(r.Context(), sql, append(params, limit, offset)...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM email_deliveries "+where, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, row := range items {
		out = append(out, map[string]any{
			"id":                  row.strOr("id", ""),
			"template":            row.strOr("template", ""),
			"recipient_email":     row.strOr("recipient_email", ""),
			"user_id":             ptNullableStr(row, "user_id"),
			"reference_id":        ptNullableStr(row, "reference_id"),
			"event":               ptNullableStr(row, "event"),
			"status":              row.strOr("status", ""),
			"attempts":            row.intOr("attempts", 0),
			"max_attempts":        row.intOr("max_attempts", 0),
			"next_attempt_at":     row.timePtr("next_attempt_at"),
			"provider":            ptNullableStr(row, "provider"),
			"provider_message_id": ptNullableStr(row, "provider_message_id"),
			"error_code":          ptNullableStr(row, "error_code"),
			"error_message":       ptNullableStr(row, "error_message"),
			"sent_at":             row.timePtr("sent_at"),
			"created_at":          row.timePtr("created_at"),
		})
	}
	total := 0
	if countRow != nil {
		total = countRow.intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out, "total": total, "limit": limit, "offset": offset,
	})
}

// SettingsEmailTestConnection: POST /v1/platform/settings/email/test-connection
func (d *Deps) SettingsEmailTestConnection(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	cfg, err := d.stResolveSMTP(ctx, true)
	if err != nil {
		d.stUpdateSMTPTest(ctx, false, err.Error())
		if errorsIs(err, email.ErrDecryptFailed) {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"success": false, "ok": false, "code": "SMTP_DECRYPTION_FAILED",
				"message": "Konfigurasi SMTP tidak dapat dibaca. Simpan ulang kredensial SMTP.",
			})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"success": false, "ok": false, "code": "SMTP_CONFIG_MISSING",
			"message": "SMTP belum dikonfigurasi.",
		})
		return
	}
	started := time.Now()
	result := stVerifySMTP(cfg)
	latency := time.Since(started).Milliseconds()
	d.stUpdateSMTPTest(ctx, result.Code == "SMTP_OK", result.Message)
	if result.Code != "SMTP_OK" {
		status := result.Status
		if status < 400 || status >= 600 {
			status = 502
		}
		writeJSON(w, status, map[string]any{
			"success": false, "ok": false, "code": result.Code,
			"message": result.Message, "detail": result.Message, "latency_ms": latency,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true, "ok": true, "code": "SMTP_OK",
		"message": result.Message, "latency_ms": latency,
	})
}

// SettingsEmailTestSend: POST /v1/platform/settings/email/test-send
func (d *Deps) SettingsEmailTestSend(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiStrictObject(w, d, r)
	if !okBody {
		return
	}
	recipient, _, err := aiStringField(body, "recipient_email", 1, 200, true, false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if !stEmailRe.MatchString(recipient) {
		writeErr(w, d, stInvalidEmailErr())
		return
	}

	// parity sendTransactionalEmailNow: kirim langsung (tanpa antrean).
	cfg, cfgErr := d.stResolveSMTP(ctx, true)
	if cfgErr != nil {
		// parity: NotConfigured (503) & Decryption (500) sama-sama membalas
		// pesan "Konfigurasi SMTP tidak dapat dibaca…" dengan code masing-masing.
		status := http.StatusServiceUnavailable
		if errorsIs(cfgErr, email.ErrDecryptFailed) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]any{
			"success": false,
			"code":    stSMTPErrCode(cfgErr),
			"message": "Konfigurasi SMTP tidak dapat dibaca. Simpan ulang kredensial SMTP.",
		})
		return
	}
	subject, html := stSMTPTestTemplate()
	messageID, sendErr := stSMTPSendNow(cfg, recipient, subject, html)
	if sendErr != nil {
		classified := stClassify("", "", sendErr)
		status := classified.Status
		if status < 400 || status >= 600 {
			status = 502
		}
		writeJSON(w, status, map[string]any{
			"success": false, "code": classified.Code,
			"message": classified.Message, "detail": classified.Message,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true, "ok": true, "status": "sent", "id": messageID,
		"message": "Test email sent successfully.",
	})
}

// stSMTPSendNow ada di settings_support.go (dipakai handler di atas).

// ---------- pagespeed ----------

// SettingsPageSpeedGet: GET /v1/platform/settings/pagespeed
func (d *Deps) SettingsPageSpeedGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT sealed_api_key, api_key_hint, updated_at FROM platform_pagespeed_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"provider":           "google-pagespeed",
		"api_key_configured": row != nil && row.has("sealed_api_key"),
		"api_key_hint":       ptNullableStr(row, "api_key_hint"),
		"updated_at":         row.timePtr("updated_at"),
	})
}

// SettingsPageSpeedPut: PUT /v1/platform/settings/pagespeed
func (d *Deps) SettingsPageSpeedPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiParsedObject(w, d, r)
	if !okBody {
		return
	}
	apiKey, hasAPIKey, err := aiStringField(body, "api_key", 8, 1000, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	existing, err := d.queryRow(ctx,
		"SELECT sealed_api_key, api_key_hint, updated_at FROM platform_pagespeed_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if (existing == nil || !existing.has("sealed_api_key")) && !(hasAPIKey && apiKey != "") {
		writeErr(w, d, errUnprocessable("Google PageSpeed API key wajib diisi saat pertama kali"))
		return
	}
	var sealed, hint any
	if hasAPIKey && apiKey != "" {
		s, sErr := d.sealT2(apiKey)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealed = s
		hint = "••••" + aiTail(apiKey, 4)
	} else if existing != nil {
		hint = ptNullableStr(existing, "api_key_hint")
	}
	if err := d.exec(ctx,
		`INSERT INTO platform_pagespeed_config (id, sealed_api_key, api_key_hint, updated_by, updated_at)
		 VALUES ('default', $1, $2, $3, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   sealed_api_key = COALESCE(EXCLUDED.sealed_api_key, platform_pagespeed_config.sealed_api_key),
		   api_key_hint = COALESCE(EXCLUDED.api_key_hint, platform_pagespeed_config.api_key_hint),
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		sealed, hint, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(ctx, user.ID, "platform.pagespeed.configuration_updated", "platform_pagespeed_config", "default")
	updated, _ := d.queryRow(ctx,
		"SELECT sealed_api_key, api_key_hint, updated_at FROM platform_pagespeed_config WHERE id = 'default' LIMIT 1")
	writeJSON(w, http.StatusOK, map[string]any{
		"provider":           "google-pagespeed",
		"api_key_configured": updated != nil && updated.has("sealed_api_key"),
		"api_key_hint":       ptNullableStr(updated, "api_key_hint"),
		"updated_at":         updated.timePtr("updated_at"),
	})
}

// SettingsPageSpeedTest: POST /v1/platform/settings/pagespeed/test
func (d *Deps) SettingsPageSpeedTest(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	// parity `await request.json().catch(() => null)` + `typeof body?.url === "string"`:
	// body rusak / url bukan string → pakai default, tanpa error.
	body := decodeJSON(r)
	target := "https://example.com"
	if v, has := body["url"]; has {
		if s, isStr := v.(string); isStr {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				target = trimmed
			}
		}
	}
	started := time.Now()
	result, err := d.stFetchPageSpeed(r.Context(), target)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		pe, isPageSpeed := err.(*stPageSpeedError)
		if isPageSpeed {
			status := 400
			switch pe.Status {
			case 503:
				status = 503
			case 429:
				status = 429
			}
			writeJSON(w, status, map[string]any{"ok": false, "message": pe.Message})
			return
		}
		writeErr(w, d, err)
		return
	}
	msg := "PageSpeed OK — Performance —/100"
	if score, isNum := result.PerformanceScore.(int); isNum {
		msg = "PageSpeed OK — Performance " + ptItoa(score) + "/100"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "message": msg, "latency_ms": latency,
		"preview": map[string]any{
			"url": result.URL, "domain": result.Domain,
			"performance_score":   result.PerformanceScore,
			"accessibility_score": result.AccessibilityScore,
		},
	})
}

// ---------- turnstile ----------

// SettingsTurnstileGet: GET /v1/platform/settings/turnstile
func (d *Deps) SettingsTurnstileGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT site_key, sealed_secret_key, enabled, updated_at FROM platform_turnstile_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":           row != nil && row.boolOr("enabled", false),
		"site_key":          ptNullableStr(row, "site_key"),
		"secret_configured": row != nil && row.has("sealed_secret_key"),
		"updated_at":        row.timePtr("updated_at"),
	})
}

// SettingsTurnstilePut: PUT /v1/platform/settings/turnstile
func (d *Deps) SettingsTurnstilePut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	body, okBody := aiStrictObject(w, d, r)
	if !okBody {
		return
	}
	siteKeyIn, hasSiteKey, err := aiStringField(body, "site_key", 0, 512, true, false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	secretKeyIn, hasSecret, err := aiStringField(body, "secret_key", 0, 2000, true, false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	enabled, _, err := aiBoolField(body, "enabled")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	existing, err := d.queryRow(ctx,
		"SELECT site_key, sealed_secret_key, enabled, updated_at FROM platform_turnstile_config WHERE id = 'default' LIMIT 1")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	siteKey := ""
	if hasSiteKey {
		siteKey = siteKeyIn
	}
	if siteKey == "" && existing != nil {
		siteKey = existing.strOr("site_key", "")
	}
	if siteKey == "" {
		writeErr(w, d, errUnprocessable("Site Key wajib diisi."))
		return
	}
	var sealed any
	if hasSecret && secretKeyIn != "" {
		s, sErr := d.sealT2(secretKeyIn)
		if sErr != nil {
			writeErr(w, d, sErr)
			return
		}
		sealed = s
	} else if existing != nil && existing.has("sealed_secret_key") {
		sealed = existing.raw("sealed_secret_key")
	}
	if sealed == nil && !d.stTurnstileSecretConfigured(ctx) {
		writeErr(w, d, errUnprocessable("Secret Key wajib diisi saat pertama kali konfigurasi."))
		return
	}
	if err := d.exec(ctx,
		`INSERT INTO platform_turnstile_config (id, site_key, sealed_secret_key, enabled, updated_by, updated_at)
		 VALUES ('default', $1, $2, $3, $4, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   site_key = EXCLUDED.site_key,
		   sealed_secret_key = COALESCE(EXCLUDED.sealed_secret_key, platform_turnstile_config.sealed_secret_key),
		   enabled = EXCLUDED.enabled,
		   updated_by = EXCLUDED.updated_by,
		   updated_at = NOW()`,
		siteKey, sealed, enabled, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(ctx, user.ID, "platform.turnstile.configuration_updated", "platform_turnstile_config", "default")
	updated, _ := d.queryRow(ctx,
		"SELECT site_key, sealed_secret_key, enabled, updated_at FROM platform_turnstile_config WHERE id = 'default' LIMIT 1")
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":           updated != nil && updated.boolOr("enabled", false),
		"site_key":          ptNullableStr(updated, "site_key"),
		"secret_configured": updated != nil && updated.has("sealed_secret_key"),
		"updated_at":        updated.timePtr("updated_at"),
	})
}

// SettingsTurnstileTest: POST /v1/platform/settings/turnstile/test
func (d *Deps) SettingsTurnstileTest(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	okTest, message := d.stTestTurnstileSecret(r.Context())
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"ok": okTest, "message": message})
}

// aiContains: keanggotaan string dalam daftar.
func aiContains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
