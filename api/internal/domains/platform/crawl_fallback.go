// Fallback pengukuran Content Velocity saat Crawlhub tidak menghasilkan angka
// (diblokir WAF, tidak punya sitemap/feed, atau service tidak terjangkau).
//
// Urutan sumber: WordPress REST API → RSS/Atom → sitemap → Wayback CDX.
// Wayback dipakai terakhir karena diambil dari archive.org, jadi tetap terbaca
// walau situs media memblokir IP datacenter kita. Semua request langsung ke
// situs memakai security.Get (guard anti-SSRF) dan menghormati robots.txt.
package platform

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

const (
	crawlFallbackWindowDays = 7
	crawlFallbackBudget     = 45 * time.Second
	// Drain admin ("Sinkronkan Sekarang") memproses beberapa job di dalam satu
	// request HTTP, jadi tiap job diberi anggaran lebih pendek.
	crawlFallbackDrainBudget = 15 * time.Second
	crawlFallbackReqTimeout  = 10 * time.Second
	crawlFallbackBodyLimit   = 4 << 20
)

type crawlFallbackResult struct {
	Articles   *int
	Confidence string
	Source     string
	// Status: ok (angka terukur), blocked (WAF/robots), no_source (situs hidup
	// tapi tidak ada sitemap/feed/API), error (jaringan/format).
	Status string
	Error  string
}

// crawlMeasureFallback: ukur jumlah artikel 7 hari terakhir tanpa Crawlhub.
func (d *Deps) crawlMeasureFallback(ctx context.Context, websiteURL string) crawlFallbackResult {
	return d.crawlMeasureFallbackWithin(ctx, websiteURL, crawlFallbackBudget)
}

func (d *Deps) crawlMeasureFallbackWithin(ctx context.Context, websiteURL string, budget time.Duration) crawlFallbackResult {
	origin, err := crawlOrigin(websiteURL)
	if err != nil {
		return crawlFallbackResult{Status: "error", Error: "URL media tidak valid"}
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	cutoff := time.Now().Add(-crawlFallbackWindowDays * 24 * time.Hour)
	robots, robotsBlocked, robotsErr := crawlFetchRobots(ctx, origin)
	// robots.txt 403/429 sering hanya rule WAF khusus bot, bukan kebijakan
	// robots. Jangan berhenti di situ: endpoint terstruktur (WP API/feed/sitemap)
	// masih bisa lolos, dan Wayback tidak lewat IP kita. Hanya homepage discovery
	// yang dilewati saat robots tidak terbaca.
	blockedSeen := robotsBlocked
	lastErr := ""
	if robotsErr != nil {
		lastErr = robotsErr.Error()
	}

	if n, conf, found, blocked := crawlWPPosts(ctx, origin, robots, cutoff); found {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: conf, Source: "wordpress_api", Status: "ok"}
	} else if blocked {
		blockedSeen = true
	}

	if n, found, blocked, fErr := crawlFeedPosts(ctx, origin, robots, cutoff, robotsBlocked); found {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: "lower_bound", Source: "feed", Status: "ok"}
	} else {
		blockedSeen = blockedSeen || blocked
		if fErr != nil {
			lastErr = fErr.Error()
		}
	}

	if n, found, blocked, sErr := crawlSitemapPosts(ctx, origin, robots, cutoff); found {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: "estimate", Source: "sitemap", Status: "ok"}
	} else {
		blockedSeen = blockedSeen || blocked
		if sErr != nil {
			lastErr = sErr.Error()
		}
	}

	if n, found, wErr := crawlWaybackPosts(ctx, origin, cutoff); found && n > 0 {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: "estimate", Source: "wayback", Status: "ok"}
	} else if wErr != nil {
		lastErr = wErr.Error()
	}

	if blockedSeen {
		note := "diblokir WAF (HTTP 403/429) di robots/endpoint langsung"
		if robotsBlocked {
			note += " — robots.txt juga ditolak"
		}
		return crawlFallbackResult{Status: "blocked", Error: note + "; Wayback tidak punya capture 7 hari terakhir"}
	}
	if lastErr != "" {
		return crawlFallbackResult{Status: "error", Error: lastErr}
	}
	return crawlFallbackResult{Status: "no_source", Error: "tidak ada sitemap/feed/API yang terbaca"}
}

func crawlIntPtr(n int) *int { return &n }

// crawlArticlesPtr: hasil Crawlhub datang sebagai `any` (JSON number), jadi
// dikonversi ke *int sekali di sini sebelum dipakai fallback.
func crawlArticlesPtr(v any) *int {
	switch n := v.(type) {
	case *int:
		return n
	case int:
		return crawlIntPtr(n)
	case int64:
		return crawlIntPtr(int(n))
	case float64:
		return crawlIntPtr(int(n))
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return crawlIntPtr(int(i))
		}
	}
	return nil
}

// crawlStoreFallback: simpan hasil fallback ke salinan lokal yang dibaca modal
// media & monitoring. Angka lama dipertahankan bila run ini tidak menghasilkan
// angka (blocked/no_source), sama seperti kebijakan igChSyncCrawlTarget.
func (d *Deps) crawlStoreFallback(ctx context.Context, mediaID, websiteURL string, fb crawlFallbackResult) error {
	var articles any
	if fb.Articles != nil {
		articles = *fb.Articles
	}
	var lastErr any
	if s := strings.TrimSpace(fb.Error); s != "" {
		lastErr = ptTruncate(s, 500)
	}
	return d.exec(ctx, `
		INSERT INTO crawl_targets (media_id, website_url, active, articles_7d, confidence, source, last_crawled_at, last_status, consecutive_failures, last_error, updated_at)
		VALUES ($1, $2, TRUE, $3, $4, $5,
		        CASE WHEN $3::integer IS NOT NULL THEN NOW() ELSE NULL END,
		        $6, 0, $7, NOW())
		ON CONFLICT (media_id) DO UPDATE SET
		  website_url = EXCLUDED.website_url,
		  articles_7d = COALESCE(EXCLUDED.articles_7d, crawl_targets.articles_7d),
		  confidence = COALESCE(EXCLUDED.confidence, crawl_targets.confidence),
		  source = COALESCE(EXCLUDED.source, crawl_targets.source),
		  last_crawled_at = CASE WHEN EXCLUDED.articles_7d IS NOT NULL THEN NOW() ELSE crawl_targets.last_crawled_at END,
		  last_status = EXCLUDED.last_status,
		  consecutive_failures = 0,
		  last_error = EXCLUDED.last_error,
		  updated_at = NOW()`,
		mediaID, websiteURL, articles, nullIfEmpty(fb.Confidence), nullIfEmpty(fb.Source), fb.Status, lastErr)
}

func crawlBlockedStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusTooManyRequests
}

// ---------- HTTP + robots ----------

func crawlFallbackGet(ctx context.Context, target string) (int, http.Header, string, error) {
	resp, err := security.Get(ctx, target, mjCrawlerHeaders, crawlFallbackReqTimeout)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, crawlFallbackBodyLimit))
	return resp.StatusCode, resp.Header, string(body), nil
}

func crawlOrigin(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("website_url kosong")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", errors.New("website_url tidak valid")
	}
	scheme := u.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	return scheme + "://" + u.Host, nil
}

func crawlResolve(origin, href string) string {
	base, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

type crawlRobotsRule struct {
	allow bool
	path  string
}

type crawlRobots struct {
	rules []crawlRobotsRule
}

// allowed: aturan dengan prefix terpanjang menang; Allow menang saat seri.
// Hanya grup `User-agent: *` (dan yang menyebut Tauthub/Crawlhub) yang dipakai.
func (r crawlRobots) allowed(path string) bool {
	if path == "" {
		path = "/"
	}
	allow := true
	best := -1
	for _, rule := range r.rules {
		if !strings.HasPrefix(path, rule.path) {
			continue
		}
		if len(rule.path) > best || (len(rule.path) == best && rule.allow) {
			best = len(rule.path)
			allow = rule.allow
		}
	}
	return allow
}

func crawlParseRobots(body string) crawlRobots {
	var rules []crawlRobotsRule
	inGroup := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		switch key {
		case "user-agent":
			ua := strings.ToLower(val)
			inGroup = ua == "*" || strings.Contains(ua, "tauthub") || strings.Contains(ua, "crawlhub")
		case "disallow":
			if inGroup && val != "" {
				rules = append(rules, crawlRobotsRule{allow: false, path: val})
			}
		case "allow":
			if inGroup && val != "" {
				rules = append(rules, crawlRobotsRule{allow: true, path: val})
			}
		}
	}
	return crawlRobots{rules: rules}
}

func crawlFetchRobots(ctx context.Context, origin string) (crawlRobots, bool, error) {
	status, _, body, err := crawlFallbackGet(ctx, origin+"/robots.txt")
	if err != nil {
		return crawlRobots{}, false, err
	}
	if crawlBlockedStatus(status) {
		return crawlRobots{}, true, nil
	}
	if status != http.StatusOK {
		return crawlRobots{}, false, nil
	}
	return crawlParseRobots(body), false, nil
}

// ---------- WordPress REST API ----------

func crawlWPPosts(ctx context.Context, origin string, robots crawlRobots, cutoff time.Time) (int, string, bool, bool) {
	// Dua bentuk endpoint: permalink cantik (/wp-json/...) dan situs dengan
	// pretty permalink mati (?rest_route=...).
	type attempt struct {
		base  string
		robot string
	}
	attempts := []attempt{
		{base: origin + "/wp-json/wp/v2/posts", robot: "/wp-json/wp/v2/posts"},
		{base: origin + "/?rest_route=/wp/v2/posts", robot: "/"},
	}
	blocked := false
	for _, a := range attempts {
		if !robots.allowed(a.robot) {
			continue
		}
		q := url.Values{}
		q.Set("after", cutoff.UTC().Format(time.RFC3339))
		q.Set("per_page", "1")
		q.Set("_fields", "id")
		sep := "?"
		if strings.Contains(a.base, "?") {
			sep = "&"
		}
		status, hdr, _, err := crawlFallbackGet(ctx, a.base+sep+q.Encode())
		if err != nil {
			continue
		}
		if crawlBlockedStatus(status) {
			blocked = true
			continue
		}
		if status != http.StatusOK {
			continue
		}
		// X-WP-Total = jumlah seluruh post yang cocok dengan filter `after`.
		if total := strings.TrimSpace(hdr.Get("X-WP-Total")); total != "" {
			if n, convErr := strconv.Atoi(total); convErr == nil && n >= 0 {
				return n, "exact", true, false
			}
		}
		// Server tanpa header: hitung maksimal 100 post terbaru dalam rentang.
		q.Set("per_page", "100")
		q.Set("_fields", "date")
		status, _, body, err := crawlFallbackGet(ctx, a.base+sep+q.Encode())
		if err != nil || status != http.StatusOK {
			continue
		}
		var posts []struct {
			Date string `json:"date"`
		}
		if json.Unmarshal([]byte(body), &posts) != nil {
			continue
		}
		conf := "exact"
		if len(posts) >= 100 {
			conf = "lower_bound"
		}
		return len(posts), conf, true, false
	}
	return 0, "", false, blocked
}

// ---------- RSS / Atom ----------

type crawlFeedItem struct {
	PubDate string `xml:"pubDate"`
	Date    string `xml:"date"`
}

type crawlFeedEntry struct {
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
}

type crawlFeedDoc struct {
	Channel struct {
		Items []crawlFeedItem `xml:"item"`
	} `xml:"channel"`
	Items   []crawlFeedItem  `xml:"item"` // RSS 1.0 / RDF (item di root)
	Entries []crawlFeedEntry `xml:"entry"`
}

var (
	crawlFeedLinkRe = regexp.MustCompile(`(?is)<link[^>]+>`)
	crawlFeedHrefRe = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)
	crawlFeedTypeRe = regexp.MustCompile(`(?i)type\s*=\s*["']application/(?:rss|atom)\+xml["']`)
)

func crawlDiscoverFeed(ctx context.Context, origin string, robots crawlRobots) string {
	if !robots.allowed("/") {
		return ""
	}
	status, _, body, err := crawlFallbackGet(ctx, origin+"/")
	if err != nil || status != http.StatusOK {
		return ""
	}
	for _, tag := range crawlFeedLinkRe.FindAllString(body, -1) {
		if !crawlFeedTypeRe.MatchString(tag) {
			continue
		}
		m := crawlFeedHrefRe.FindStringSubmatch(tag)
		if len(m) < 2 {
			continue
		}
		if resolved := crawlResolve(origin, m[1]); resolved != "" {
			return resolved
		}
	}
	return ""
}

func crawlFeedPosts(ctx context.Context, origin string, robots crawlRobots, cutoff time.Time, skipDiscovery bool) (int, bool, bool, error) {
	candidates := []string{}
	if !skipDiscovery {
		if discovered := crawlDiscoverFeed(ctx, origin, robots); discovered != "" {
			candidates = append(candidates, discovered)
		}
	}
	for _, p := range []string{"/feed", "/rss.xml", "/atom.xml", "/index.xml"} {
		candidates = append(candidates, origin+p)
	}

	blocked := false
	lastErr := error(nil)
	seen := map[string]bool{}
	for _, target := range candidates {
		if seen[target] {
			continue
		}
		seen[target] = true
		parsed, parseErr := url.Parse(target)
		if parseErr != nil || !robots.allowed(parsed.Path) {
			continue
		}
		status, _, body, err := crawlFallbackGet(ctx, target)
		if err != nil {
			lastErr = err
			continue
		}
		if crawlBlockedStatus(status) {
			blocked = true
			continue
		}
		if status != http.StatusOK {
			continue
		}
		if count, ok := crawlCountFeed(body, cutoff); ok {
			return count, true, blocked, nil
		}
	}
	return 0, false, blocked, lastErr
}

func crawlCountFeed(body string, cutoff time.Time) (int, bool) {
	var doc crawlFeedDoc
	if xml.Unmarshal([]byte(body), &doc) != nil {
		return 0, false
	}
	items := append([]crawlFeedItem{}, doc.Channel.Items...)
	items = append(items, doc.Items...)
	dates := make([]string, 0, len(items))
	for _, item := range items {
		if item.PubDate != "" {
			dates = append(dates, item.PubDate)
		} else if item.Date != "" {
			dates = append(dates, item.Date)
		}
	}
	for _, entry := range doc.Entries {
		if entry.Published != "" {
			dates = append(dates, entry.Published)
		} else if entry.Updated != "" {
			dates = append(dates, entry.Updated)
		}
	}
	if len(items)+len(doc.Entries) == 0 || len(dates) == 0 {
		return 0, false
	}
	count := 0
	for _, raw := range dates {
		if ts, ok := crawlParseTime(raw); ok && !ts.Before(cutoff) {
			count++
		}
	}
	return count, true
}

// ---------- sitemap ----------

type crawlSitemapDoc struct {
	URLs []struct {
		LastMod string `xml:"lastmod"`
	} `xml:"url"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

func crawlSitemapPosts(ctx context.Context, origin string, robots crawlRobots, cutoff time.Time) (int, bool, bool, error) {
	blocked := false
	lastErr := error(nil)
	for _, path := range []string{"/sitemap.xml", "/sitemap_index.xml", "/wp-sitemap.xml", "/sitemap-index.xml"} {
		if !robots.allowed(path) {
			continue
		}
		status, _, body, err := crawlFallbackGet(ctx, origin+path)
		if err != nil {
			lastErr = err
			continue
		}
		if crawlBlockedStatus(status) {
			blocked = true
			continue
		}
		if status != http.StatusOK {
			continue
		}
		var doc crawlSitemapDoc
		if xml.Unmarshal([]byte(body), &doc) != nil {
			continue
		}
		if len(doc.Sitemaps) > 0 {
			count, dated := 0, 0
			for i, sm := range doc.Sitemaps {
				if i >= 5 {
					break
				}
				loc := strings.TrimSpace(sm.Loc)
				if loc == "" {
					continue
				}
				child, parseErr := url.Parse(loc)
				if parseErr != nil || !robots.allowed(child.Path) {
					continue
				}
				childStatus, _, childBody, childErr := crawlFallbackGet(ctx, loc)
				if childErr != nil || childStatus != http.StatusOK {
					continue
				}
				var childDoc crawlSitemapDoc
				if xml.Unmarshal([]byte(childBody), &childDoc) != nil {
					continue
				}
				c, n := crawlCountSitemapURLs(childDoc, cutoff)
				count += c
				dated += n
			}
			if dated > 0 {
				return count, true, blocked, nil
			}
			continue
		}
		if count, dated := crawlCountSitemapURLs(doc, cutoff); dated > 0 {
			return count, true, blocked, nil
		}
	}
	return 0, false, blocked, lastErr
}

func crawlCountSitemapURLs(doc crawlSitemapDoc, cutoff time.Time) (int, int) {
	count, dated := 0, 0
	for _, u := range doc.URLs {
		raw := strings.TrimSpace(u.LastMod)
		if raw == "" {
			continue
		}
		ts, ok := crawlParseTime(raw)
		if !ok {
			continue
		}
		dated++
		if !ts.Before(cutoff) {
			count++
		}
	}
	return count, dated
}

// ---------- Wayback CDX (tidak terkena blokir situs) ----------

func crawlWaybackPosts(ctx context.Context, origin string, cutoff time.Time) (int, bool, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return 0, false, err
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host == "" {
		return 0, false, errors.New("host kosong")
	}
	q := url.Values{}
	q.Set("url", host+"/*")
	q.Set("from", cutoff.Format("20060102"))
	q.Set("to", time.Now().Format("20060102"))
	q.Set("output", "json")
	q.Set("fl", "timestamp,original")
	q.Add("filter", "statuscode:200")
	q.Add("filter", "mimetype:text/html")
	q.Set("collapse", "urlkey")
	q.Set("limit", "2000")

	status, _, body, err := crawlFallbackGet(ctx, "https://web.archive.org/cdx/search/cdx?"+q.Encode())
	if err != nil {
		return 0, false, err
	}
	if status != http.StatusOK {
		return 0, false, fmt.Errorf("wayback HTTP %d", status)
	}
	var rows [][]string
	if json.Unmarshal([]byte(body), &rows) != nil {
		return 0, false, errors.New("respons wayback tidak valid")
	}
	if len(rows) <= 1 {
		return 0, true, nil // hanya header → tidak ada capture
	}
	return len(rows) - 1, true, nil
}

// ---------- util tanggal ----------

func crawlParseTime(raw string) (time.Time, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}
