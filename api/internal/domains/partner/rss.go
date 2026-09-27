package partner

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// ---------- RSS feeds (port rss-feeds/*) ----------

func partnerFeedJSON(row rowMap, origin string) map[string]any {
	token := strings.TrimSpace(row.strOr("token", ""))
	feedPath := "/feed/partner/" + token
	return map[string]any{
		"id":                row.raw("id"),
		"media_id":          row.raw("media_id"),
		"media_name":        row.raw("media_name"),
		"media_website_url": row.raw("media_website_url"),
		"name":              row.raw("name"),
		"slug":              row.raw("slug"),
		"description":       row.raw("description"),
		"item_status":       row.raw("item_status"),
		"status":            row.raw("status"),
		"install_status":    row.strOr("install_status", "pending"),
		"install_error":     row.raw("install_error"),
		"installed_at":      row.timePtr("installed_at"),
		"feed_path":         feedPath,
		"feed_url":          origin + feedPath,
		"created_at":        row.timePtr("created_at"),
		"updated_at":        row.timePtr("updated_at"),
	}
}

func requestOrigin(r *http.Request) string {
	scheme := "https"
	if r.Header.Get("x-forwarded-proto") != "" {
		scheme = strings.TrimSpace(strings.Split(r.Header.Get("x-forwarded-proto"), ",")[0])
	} else if strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1") {
		scheme = "http"
	}
	return scheme + "://" + publicHost(r)
}

func (d *Deps) RSSFeeds(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 100, 1, 500)
	offset := clampInt(r.URL.Query().Get("offset"), 0, 0, 1<<30)
	count, err := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM rss_feeds WHERE partner_id = $1 AND status = 'active'", partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT f.id, f.media_id, f.name, f.slug, f.token, f.description, f.item_status,
		       f.install_status, f.install_error, f.installed_at, f.created_at, f.updated_at,
		       m.name AS media_name, m.website_url AS media_website_url
		FROM rss_feeds f LEFT JOIN media m ON m.id = f.media_id
		WHERE f.partner_id = $1 AND f.status = 'active'
		ORDER BY f.created_at DESC LIMIT $2 OFFSET $3`, partner.ID, limit, offset)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	origin := requestOrigin(r)
	feeds := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		feeds = append(feeds, partnerFeedJSON(row, origin))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"feeds": feeds, "total": count.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

func (d *Deps) rssFeedScoped(r *http.Request, partnerID, feedID, extra string) (rowMap, error) {
	sql := `SELECT f.*, m.name AS media_name, m.website_url AS media_website_url
		FROM rss_feeds f LEFT JOIN media m ON m.id = f.media_id
		WHERE f.id = $1 AND f.partner_id = $2`
	if extra != "" {
		sql += " AND " + extra
	}
	sql += " LIMIT 1"
	row, err := d.queryRow(r.Context(), sql, feedID, partnerID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("RSS feed not found")
	}
	return row, nil
}

func (d *Deps) RSSFeedDelete(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	feedID := chiParam(r, "feedId")
	feed, err := d.rssFeedScoped(r, partner.ID, feedID, "")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE rss_feeds SET status = 'revoked', updated_at = NOW() WHERE id = $1", feedID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "rss_feed.revoked", "rss_feed", feedID)
	feed["status"] = "revoked"
	writeJSON(w, http.StatusOK, partnerFeedJSON(feed, requestOrigin(r)))
}

func (d *Deps) RSSFeedRotate(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	feedID := chiParam(r, "feedId")
	if _, err := d.rssFeedScoped(r, partner.ID, feedID, ""); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(), `
		UPDATE rss_feeds SET token = $1, install_status = 'pending', install_error = NULL, installed_at = NULL, updated_at = NOW()
		WHERE id = $2`, randomHex(32), feedID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "rss_feed.token_rotated", "rss_feed", feedID)
	updated, err := d.rssFeedScoped(r, partner.ID, feedID, "")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, partnerFeedJSON(updated, requestOrigin(r)))
}

func (d *Deps) RSSFeedTest(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	feedID := chiParam(r, "feedId")
	feed, err := d.rssFeedScoped(r, partner.ID, feedID, "")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	feedURL := requestOrigin(r) + "/v1/publishing/feed.json?token=" + strings.TrimSpace(feed.strOr("token", ""))
	status, body, _ := d.safeGetWithHeader(r, feedURL, map[string]string{"user-agent": "TulisioFeedTester/1.0"}, 10*time.Second)
	okResult := false
	var itemsCount any
	var generatedAt any
	var errMessage any
	if status >= 200 && status < 300 {
		okResult = true
		var parsed map[string]any
		if jsonUnmarshal(body, &parsed) == nil {
			if items, isArr := parsed["items"].([]any); isArr {
				itemsCount = len(items)
			} else {
				itemsCount = 0
			}
			if feedObj, isMap := parsed["feed"].(map[string]any); isMap {
				generatedAt = feedObj["generated_at"]
			}
		}
	} else if status == 0 {
		errMessage = "Gagal menguji feed"
	} else {
		errMessage = "Endpoint merespons HTTP " + itoa(status)
	}
	d.auditLog(r.Context(), user.ID, "rss_feed.test_connection", "rss_feed", feedID)
	feedName := feed.strOr("media_name", "")
	website := feed.strOr("media_website_url", "")
	var websiteVal any
	if website != "" {
		websiteVal = website
	}
	var mediaNameVal any
	if feedName != "" {
		mediaNameVal = feedName
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           feedID,
		"ok":           okResult,
		"status":       status,
		"items_count":  itemsCount,
		"generated_at": generatedAt,
		"media_name":   mediaNameVal,
		"website_url":  websiteVal,
		"feed_url":     feedURL,
		"error":        errMessage,
	})
}

// RSSFeedVerify: cek apakah URL feed terpasang di halaman website mitra.
func (d *Deps) RSSFeedVerify(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	feedID := chiParam(r, "feedId")
	feed, err := d.rssFeedScoped(r, partner.ID, feedID, "f.status = 'active'")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	siteURL := strings.TrimSpace(feed.strOr("media_website_url", ""))
	if siteURL != "" && !strings.HasPrefix(strings.ToLower(siteURL), "http://") && !strings.HasPrefix(strings.ToLower(siteURL), "https://") {
		siteURL = "https://" + siteURL
	}
	token := strings.TrimSpace(feed.strOr("token", ""))
	feedPath := "/feed/partner/" + token
	jsonPath := "/v1/publishing/feed.json?token=" + token
	if siteURL == "" {
		reason := "Media feed tidak memiliki website (website_url kosong). Set website media dahulu di admin."
		_ = d.exec(r.Context(),
			"UPDATE rss_feeds SET install_status = 'failed', install_error = $1, installed_at = NULL WHERE id = $2", reason, feedID)
		d.auditLog(r.Context(), user.ID, "rss_feed.verify_failed", "rss_feed", feedID)
		writeJSON(w, http.StatusOK, map[string]any{
			"id": feedID, "install_status": "failed", "installed": false,
			"install_error": "Media feed tidak memiliki website.",
		})
		return
	}
	status, body, _ := d.safeGetWithHeader(r, siteURL, map[string]string{"user-agent": "TulisioRSSVerifier/1.0"}, 10*time.Second)
	if status < 200 || status >= 300 {
		reason := "Situs tidak dapat dijangkau"
		if status != 0 {
			reason = "Situs merespons HTTP " + itoa(status)
		}
		d.writeJSONFeedVerifyFailure(w, r, user.ID, feedID, siteURL, reason, feedPath, jsonPath)
		return
	}
	if len(body) > 512_000 {
		body = body[:512_000]
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, strings.ToLower(feedPath)) || strings.Contains(lower, strings.ToLower(jsonPath)) {
		verifiedVia := "rss"
		if strings.Contains(lower, strings.ToLower(jsonPath)) {
			verifiedVia = "feed.json"
		}
		if err := d.exec(r.Context(),
			"UPDATE rss_feeds SET install_status = 'verified', install_error = NULL, installed_at = NOW(), verified_by = $1 WHERE id = $2",
			user.ID, feedID); err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLog(r.Context(), user.ID, "rss_feed.verified", "rss_feed", feedID)
		writeJSON(w, http.StatusOK, map[string]any{
			"id": feedID, "install_status": "verified", "installed": true, "install_error": nil,
			"installed_at": isoMilli(nowUTC()), "site_url": siteURL, "verified_via": verifiedVia,
		})
		return
	}
	reason := "URL feed tidak ditemukan di halaman " + siteURL + ". Pasang salah satu di situs Anda, lalu coba lagi:\n" +
		"• RSS: <link rel=\"alternate\" type=\"application/rss+xml\" href=\"" + feedPath + "\"> di <head>\n" +
		"• JSON Pull: <link rel=\"alternate\" type=\"application/json\" href=\"" + jsonPath + "\"> (website custom)"
	d.writeJSONFeedVerifyFailure(w, r, user.ID, feedID, siteURL, reason, feedPath, jsonPath)
}

func (d *Deps) writeJSONFeedVerifyFailure(w http.ResponseWriter, r *http.Request, userID, feedID, siteURL, reason, _, _ string) {
	reason = truncateRunes(reason, 1000)
	if err := d.exec(r.Context(),
		"UPDATE rss_feeds SET install_status = 'failed', install_error = $1, installed_at = NULL WHERE id = $2", reason, feedID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), userID, "rss_feed.verify_failed", "rss_feed", feedID)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": feedID, "install_status": "failed", "installed": false,
		"install_error": reason, "site_url": siteURL,
	})
}

// ---------- publishing API keys ----------

func publishingKeyJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":           row.raw("id"),
		"label":        row.raw("label"),
		"media_id":     row.raw("media_id"),
		"status":       row.raw("status"),
		"last_used_at": row.timePtr("last_used_at"),
		"created_at":   row.timePtr("created_at"),
	}
}

func (d *Deps) PublishingAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := d.queryRows(r.Context(), `
			SELECT k.id, k.label, k.media_id, k.status, k.last_used_at, k.created_at, m.name AS media_name
			FROM publishing_api_keys k LEFT JOIN media m ON m.id = k.media_id
			WHERE k.partner_id = $1 ORDER BY k.created_at DESC`, partner.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		keys := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			keys = append(keys, publishingKeyJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
		return
	}
	if err := d.checkAuthRateReq(r, "partner-apikey:"+user.ID, 5); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	label, present := strField(body, "label")
	if !present {
		label = "Default"
	}
	if len(label) < 2 || len(label) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid label")
		return
	}
	var mediaID any
	if v, hasMedia := body["media_id"]; hasMedia && v != nil {
		s, isString := v.(string)
		if !isString || s == "" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid media_id")
			return
		}
		media, err := d.queryRow(r.Context(),
			"SELECT id, partner_id, integration_type FROM media WHERE id = $1 LIMIT 1", s)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if media == nil || media.strOr("partner_id", "") != partner.ID {
			d.writeErr(w, errNotFound("Media tidak ditemukan"))
			return
		}
		mediaID = s
	}
	apiKey, secret := randomHex(24), randomHex(24)
	sealedKey, err := d.sealT2(apiKey)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	sealedSecret, err := d.sealT2(secret)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(), `
		INSERT INTO publishing_api_keys (id, partner_id, media_id, label, api_key_hash, api_key_sealed, secret_sealed, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', $8)`,
		id, partner.ID, mediaID, label, sha256Hex(apiKey), sealedKey, sealedSecret, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "publishing_api_key.created", "publishing_api_key", id)
	created, err := d.queryRow(r.Context(),
		"SELECT id, label, media_id, status, last_used_at, created_at FROM publishing_api_keys WHERE id = $1 LIMIT 1", id)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"key":     publishingKeyJSON(created),
		"api_key": apiKey,
		"secret":  secret,
		"note":    "Simpan api_key dan secret sekarang — hanya ditampilkan sekali ini.",
	})
}

func (d *Deps) PublishingAPIKeyAction(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	keyID := chiParam(r, "keyId")
	key, err := d.queryRow(r.Context(),
		"SELECT id, label, media_id, status, last_used_at, created_at FROM publishing_api_keys WHERE id = $1 AND partner_id = $2 LIMIT 1",
		keyID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if key == nil {
		d.writeErr(w, errNotFound("API key tidak ditemukan"))
		return
	}
	if r.Method == http.MethodDelete {
		if key.strOr("status", "") == "revoked" {
			httpx.Error(w, http.StatusConflict, "API key sudah dicabut")
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE publishing_api_keys SET status = 'revoked', updated_at = NOW() WHERE id = $1 AND partner_id = $2",
			keyID, partner.ID); err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLog(r.Context(), user.ID, "publishing_api_key.revoked", "publishing_api_key", keyID)
		updated, _ := d.queryRow(r.Context(),
			"SELECT id, label, media_id, status, last_used_at, created_at FROM publishing_api_keys WHERE id = $1 AND partner_id = $2 LIMIT 1",
			keyID, partner.ID)
		writeJSON(w, http.StatusOK, publishingKeyJSON(updated))
		return
	}
	// Rotate.
	apiKey, secret := randomHex(24), randomHex(24)
	sealedKey, err := d.sealT2(apiKey)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	sealedSecret, err := d.sealT2(secret)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(), `
		UPDATE publishing_api_keys SET api_key_hash = $1, api_key_sealed = $2, secret_sealed = $3, status = 'active', updated_at = NOW()
		WHERE id = $4 AND partner_id = $5`,
		sha256Hex(apiKey), sealedKey, sealedSecret, keyID, partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "publishing_api_key.rotated", "publishing_api_key", keyID)
	updated, _ := d.queryRow(r.Context(),
		"SELECT id, label, media_id, status, last_used_at, created_at FROM publishing_api_keys WHERE id = $1 AND partner_id = $2 LIMIT 1",
		keyID, partner.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"key":     publishingKeyJSON(updated),
		"api_key": apiKey,
		"secret":  secret,
		"note":    "Simpan api_key dan secret sekarang — hanya ditampilkan sekali ini.",
	})
}

// PublishingAPIDocs: dokumen API publishing dari folder docs repo.
func (d *Deps) PublishingAPIDocs(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.auth(w, r); !ok {
		return
	}
	doc := strings.TrimSpace(r.URL.Query().Get("doc"))
	if doc != "publishing" && doc != "protocol" {
		doc = "publishing"
	}
	fileName := "publishing-api.md"
	if doc == "protocol" {
		fileName = "publisher-protocol.md"
	}
	content, err := readDocsFile(fileName)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Dokumen docs/"+strings.TrimSuffix(fileName, ".md")+".md belum tersedia di server.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"markdown": content.Body, "updated_at": content.UpdatedAt})
}

// safeGetWithHeader: varian safeGet yang mengembalikan header (dipakai RSS test).
func (d *Deps) safeGetWithHeader(r *http.Request, target string, headers map[string]string, timeout time.Duration) (int, string, http.Header) {
	resp, err := security.Get(r.Context(), target, headers, timeout)
	if err != nil {
		return 0, "", http.Header{}
	}
	body, _ := readBodyLimited(resp, 512*1024)
	return resp.StatusCode, body, resp.Header
}
