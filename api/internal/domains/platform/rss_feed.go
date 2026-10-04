package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Port app/feed/partner/[token]/* — feed RSS 2.0 per feed mitra + callback
// importer (foundation §13). Feed bersifat publik (di-pull CMS mitra) tetapi
// token per feed adalah rahasianya; token tak dikenal selalu 404 tanpa
// membocorkan keberadaan feed. Rate limit dihitung per feed per menit dari
// rss_access_logs (parity app lama: 30/menit untuk feed, 60/menit untuk impor).

const (
	rssFeedRateLimitPerMinute   = 30
	rssImportRateLimitPerMinute = 60
	rssMaxItems                 = 50
)

// rssLogAccess: catat akses feed/callback. Kegagalan log TIDAK boleh
// mematahkan respons (parity komentar app lama).
func (d *Deps) rssLogAccess(ctx context.Context, feedID string, r *http.Request, status int) {
	ip := r.Header.Get("x-forwarded-for")
	if idx := strings.IndexByte(ip, ','); idx >= 0 {
		ip = ip[:idx]
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = strings.TrimSpace(r.Header.Get("x-real-ip"))
	}
	var ipPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	ua := r.Header.Get("User-Agent")
	if utf8.RuneCountInString(ua) > 300 {
		ua = string([]rune(ua)[:300])
	}
	_ = d.exec(ctx,
		"INSERT INTO rss_access_logs (id, feed_id, ip_address, user_agent, status) VALUES ($1, $2, $3, $4, $5)",
		uuid.NewString(), feedID, ipPtr, ua, status)
}

func (d *Deps) rssRecentAccess(ctx context.Context, feedID string) int {
	row, err := d.queryRow(ctx,
		"SELECT COUNT(*) AS count FROM rss_access_logs WHERE feed_id = $1 AND created_at > NOW() - INTERVAL '60 seconds'",
		feedID)
	if err != nil || row == nil {
		return 0
	}
	return row.intOr("count", 0)
}

// ---------- GET /feed/partner/{token} ----------

func (d *Deps) RSSFeedServe(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chiParam(r, "token"))
	feed, err := d.queryRow(r.Context(), `
		SELECT f.id, f.partner_id, f.media_id, f.name, f.description, f.item_status,
		       m.name AS media_name, m.website_url AS media_website_url
		FROM rss_feeds f LEFT JOIN media m ON m.id = f.media_id
		WHERE f.token = $1 AND f.status = 'active' LIMIT 1`, token)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if feed == nil {
		http.NotFound(w, r)
		return
	}
	feedID := feed.strOr("id", "")
	if d.rssRecentAccess(r.Context(), feedID) >= rssFeedRateLimitPerMinute {
		d.rssLogAccess(r.Context(), feedID, r, http.StatusTooManyRequests)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("Too many requests"))
		return
	}

	// Feed level media memfilter publikasi media itu; feed partner mengambil
	// semua media mitra. Item draft hanya muncul saat feed.item_status='draft'.
	statuses := []string{"published"}
	if feed.strOr("item_status", "") == "draft" {
		statuses = []string{"queued", "published"}
	}
	mediaID := feed.strOr("media_id", "")
	args := []any{feed.strOr("partner_id", "")}
	mediaClause := "TRUE"
	if mediaID != "" {
		mediaClause = "p.media_id = $2"
		args = append(args, mediaID)
	}
	ph := make([]string, len(statuses))
	for i, s := range statuses {
		ph[i] = "$" + strconv.Itoa(len(args)+i+1)
		args = append(args, s)
	}
	items, err := d.queryRows(r.Context(), `
		SELECT d.title AS document_title, d.slug AS document_slug, d.meta_description,
		       p.published_at, p.published_url, m.website_url AS media_website_url, m.name AS media_name
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		WHERE m.partner_id = $1 AND `+mediaClause+` AND p.status IN (`+strings.Join(ph, ",")+`)
		ORDER BY p.published_at DESC NULLS LAST LIMIT `+strconv.Itoa(rssMaxItems), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	mediaWebsite := feed.strOr("media_website_url", "")
	origin := "http://" + r.Host
	if r.TLS != nil {
		origin = "https://" + r.Host
	}
	channelLink := origin
	if mediaWebsite != "" {
		channelLink = mediaWebsite
	}
	channelTitle := feed.strOr("media_name", "")
	if channelTitle == "" {
		channelTitle = feed.strOr("name", "")
	}
	selfURL := origin + r.URL.RequestURI()

	var b strings.Builder
	desc := feed.strOr("description", "")
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">` + "\n  <channel>\n")
	b.WriteString("    <title>" + rssEscapeXML(channelTitle) + "</title>\n")
	b.WriteString("    <link>" + rssEscapeXML(channelLink) + "</link>\n")
	b.WriteString(`    <atom:link href="` + rssEscapeXML(selfURL) + `" rel="self" type="application/rss+xml" />` + "\n")
	if desc != "" {
		b.WriteString("    <description>" + rssEscapeXML(desc) + "</description>\n")
	}
	for _, item := range items {
		title := item.strOr("document_title", "")
		publishedURL := item.strOr("published_url", "")
		slug := item.strOr("document_slug", "")
		itemMediaWebsite := item.strOr("media_website_url", "")
		itemChannelLink := channelLink
		if itemMediaWebsite != "" {
			itemChannelLink = itemMediaWebsite
		}
		link := publishedURL
		if link == "" {
			link = strings.TrimRight(itemChannelLink, "/") + "/" + slug
		}
		guid := "tulisio-" + feedID + "-" + slug
		if slug == "" {
			guid = "tulisio-" + feedID + "-" + title
		}
		b.WriteString("    <item>\n")
		b.WriteString("      <title>" + rssEscapeXML(title) + "</title>\n")
		b.WriteString("      <link>" + rssEscapeXML(link) + "</link>\n")
		b.WriteString(`      <guid isPermaLink="false">` + rssEscapeXML(guid) + "</guid>\n")
		if meta := item.strOr("meta_description", ""); meta != "" {
			b.WriteString("      <description>" + rssEscapeXML(meta) + "</description>\n")
		}
		b.WriteString("      <pubDate>" + rssRFC2822(item.raw("published_at")) + "</pubDate>\n")
		b.WriteString("    </item>\n")
	}
	b.WriteString("  </channel>\n</rss>\n")

	d.rssLogAccess(r.Context(), feedID, r, http.StatusOK)
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

// ---------- POST /feed/partner/{token}/import ----------

// rssImportSchema: parity zod importSchema (guid 1..300, status enum, url ≤ 500).
type rssImportBody struct {
	GUID   string `json:"guid"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

func (d *Deps) RSSFeedImport(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chiParam(r, "token"))
	feed, err := d.queryRow(r.Context(),
		"SELECT id, partner_id, media_id FROM rss_feeds WHERE token = $1 AND status = 'active' LIMIT 1", token)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if feed == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found"})
		return
	}
	feedID := feed.strOr("id", "")
	if d.rssRecentAccess(r.Context(), feedID) >= rssImportRateLimitPerMinute {
		d.rssLogAccess(r.Context(), feedID, r, http.StatusTooManyRequests)
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"detail": "Too many requests"})
		return
	}

	var body rssImportBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "Invalid JSON body"})
		return
	}
	guid := strings.TrimSpace(body.GUID)
	if guid == "" || utf8.RuneCountInString(guid) > 300 || (body.Status != "imported" && body.Status != "published") {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "Invalid payload"})
		return
	}
	if body.URL != "" && utf8.RuneCountInString(body.URL) > 500 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "Invalid payload"})
		return
	}

	// GUID adalah `tulisio-{feedID}-{document_slug}` dari feed ini.
	prefix := "tulisio-" + feedID + "-"
	if !strings.HasPrefix(guid, prefix) {
		d.rssLogAccess(r.Context(), feedID, r, http.StatusNotFound)
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Unknown guid"})
		return
	}
	slug := strings.TrimSpace(guid[len(prefix):])
	if slug == "" {
		d.rssLogAccess(r.Context(), feedID, r, http.StatusNotFound)
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Unknown guid"})
		return
	}

	// Publikasi yang cocok: artikel terbit pada salah satu media feed.
	mediaID := feed.strOr("media_id", "")
	args := []any{slug, feed.strOr("partner_id", "")}
	mediaClause := ""
	if mediaID != "" {
		mediaClause = " AND p.media_id = $3"
		args = append(args, mediaID)
	}
	publication, err := d.queryRow(r.Context(), `
		SELECT p.id FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		WHERE d.slug = $1 AND m.partner_id = $2`+mediaClause+` AND p.status = 'published'
		ORDER BY p.published_at DESC NULLS LAST LIMIT 1`, args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if publication == nil {
		d.rssLogAccess(r.Context(), feedID, r, http.StatusNotFound)
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found"})
		return
	}
	publicationID := publication.strOr("id", "")

	if body.Status == "published" {
		_ = d.exec(r.Context(),
			"UPDATE publications SET rss_import_status = 'published', rss_published_at = NOW(), rss_imported_at = COALESCE(rss_imported_at, NOW()), updated_at = NOW() WHERE id = $1",
			publicationID)
	} else {
		_ = d.exec(r.Context(),
			"UPDATE publications SET rss_import_status = 'imported', rss_imported_at = NOW(), updated_at = NOW() WHERE id = $1",
			publicationID)
	}

	d.rssLogAccess(r.Context(), feedID, r, http.StatusOK)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "publication_id": publicationID, "status": body.Status})
}

// ---------- helper XML ----------

func rssEscapeXML(v string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(v)
}

// rssRFC2822: RFC 2822 dalam UTC; nilai tak terbaca → waktu sekarang (parity).
func rssRFC2822(v any) string {
	if t, ok := v.(time.Time); ok && !t.IsZero() {
		return t.UTC().Format(time.RFC1123Z)
	}
	return time.Now().UTC().Format(time.RFC1123Z)
}
