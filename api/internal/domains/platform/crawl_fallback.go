// Fallback pengukuran Content Velocity saat Crawlhub tidak menghasilkan angka
// (diblokir WAF, tidak punya sitemap/feed, atau service tidak terjangkau).
//
// Urutan sumber: WordPress REST API → RSS/Atom → sitemap → Wayback CDX.
// Wayback dipakai terakhir karena diambil dari archive.org, jadi tetap terbaca
// walau situs media memblokir IP datacenter kita. Semua request langsung ke
// situs memakai security.Get (guard anti-SSRF) dan menghormati robots.txt.
package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

const (
	// archive.org membatasi request per IP. Job crawl memproses banyak media
	// beruntun, jadi semua panggilan Wayback diserialkan dengan jeda minimum.
	crawlWaybackMinGap = 8 * time.Second
	crawlWaybackUA     = "TauthubCrawler/1.0 (+https://tauthub.com; crawl@tauthub.com)"
	// Setelah 429, istirahatkan Wayback sebentar agar tidak makin diblokir.
	crawlWaybackCooldown = 10 * time.Minute
	// Google News juga membatasi; jeda lebih pendek karena endpoint-nya ringan.
	crawlNewsMinGap = 1500 * time.Millisecond
)

var (
	crawlWaybackMu        sync.Mutex
	crawlWaybackLast      time.Time
	crawlWaybackCooldownT time.Time
	crawlNewsMu           sync.Mutex
	crawlNewsLast         time.Time
	crawlEgressMu         sync.RWMutex
	crawlEgressURL        string
	crawlEgressToken      string
)

// ConfigureCrawlEgress: set Cloudflare Worker egress (dari env
// CRAWL_EGRESS_URL/CRAWL_EGRESS_TOKEN) saat boot. Kosong = langsung.
func ConfigureCrawlEgress(url, token string) {
	crawlEgressMu.Lock()
	crawlEgressURL = strings.TrimRight(strings.TrimSpace(url), "/")
	crawlEgressToken = strings.TrimSpace(token)
	crawlEgressMu.Unlock()
	slog.Info("[crawl-egress] konfigurasi", "url_set", crawlEgressURL != "", "token_set", crawlEgressToken != "")
}

type crawlEgressResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Error   string            `json:"error"`
}

// crawlEgressGet: ambil URL lewat Cloudflare Worker. Dipakai untuk situs media
// yang menolak IP datacenter. Return ok=false bila egress tidak dikonfigurasi
// atau gagal — pemanggil lalu jatuh ke koneksi langsung.
func crawlEgressGet(ctx context.Context, target string) (int, http.Header, string, bool) {
	crawlEgressMu.RLock()
	base, token := crawlEgressURL, crawlEgressToken
	crawlEgressMu.RUnlock()
	if base == "" || token == "" {
		slog.Warn("[crawl-egress] belum dikonfigurasi", "url_set", base != "", "token_set", token != "")
		return 0, nil, "", false
	}
	payload, err := json.Marshal(map[string]string{"url": target})
	if err != nil {
		return 0, nil, "", false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, "", false
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-crawl-token", token)
	// URL egress dikonfigurasi operator (bukan input pengguna), jadi boleh
	// loopback (SSH reverse tunnel 127.0.0.1:8787) tanpa guard SSRF.
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		slog.Warn("[crawl-egress] request gagal", "target", target, "err", err)
		return 0, nil, "", false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, crawlFallbackBodyLimit))
	if resp.StatusCode != http.StatusOK {
		slog.Warn("[crawl-egress] HTTP tidak OK", "target", target, "status", resp.StatusCode)
		return 0, nil, "", false
	}
	var out crawlEgressResponse
	if json.Unmarshal(body, &out) != nil || out.Error != "" || out.Status == 0 {
		slog.Warn("[crawl-egress] respons tidak valid", "target", target, "error", out.Error, "body", ptTruncate(string(body), 200))
		return 0, nil, "", false
	}
	hdr := http.Header{}
	for k, v := range out.Headers {
		hdr.Set(k, v)
	}
	return out.Status, hdr, out.Body, true
}

// crawlMeasureFallback: ukur jumlah artikel 7 hari terakhir tanpa Crawlhub.
func (d *Deps) crawlMeasureFallback(ctx context.Context, websiteURL string) crawlFallbackResult {
	return d.crawlMeasureFallbackWithin(ctx, websiteURL, crawlFallbackBudget)
}

func (d *Deps) crawlMeasureFallbackWithin(ctx context.Context, websiteURL string, budget time.Duration) (res crawlFallbackResult) {
	origin, err := crawlOrigin(websiteURL)
	if err != nil {
		return crawlFallbackResult{Status: "error", Error: "URL media tidak valid"}
	}
	defer func() {
		attrs := []any{"origin", origin, "status", res.Status, "source", res.Source}
		if res.Articles != nil {
			attrs = append(attrs, "articles", *res.Articles)
			slog.Info("[crawl-fallback] terukur", attrs...)
		} else {
			attrs = append(attrs, "error", res.Error)
			slog.Warn("[crawl-fallback] gagal", attrs...)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	cutoff := time.Now().Add(-crawlFallbackWindowDays * 24 * time.Hour)
	robots, robotsBlocked, robotsErr := crawlFetchRobots(ctx, origin)
	// robots.txt 403/429 sering hanya rule WAF khusus bot, bukan kebijakan
	// robots. Jangan berhenti di situ: endpoint terstruktur (WP API/feed/sitemap)
	// masih bisa lolos, dan Wayback tidak lewat IP kita. Hanya homepage discovery
	// yang dilewati saat robots tidak terbaca.
	wpBlocked, feedBlocked, sitemapBlocked := false, false, false
	lastErr := ""
	if robotsErr != nil {
		lastErr = robotsErr.Error()
	}

	if n, conf, found, blocked := crawlWPPosts(ctx, origin, robots, cutoff); found {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: conf, Source: "wordpress_api", Status: "ok"}
	} else {
		wpBlocked = blocked
	}

	if n, found, blocked, fErr := crawlFeedPosts(ctx, origin, robots, cutoff, robotsBlocked); found {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: "lower_bound", Source: "feed", Status: "ok"}
	} else {
		feedBlocked = blocked
		if fErr != nil {
			lastErr = fErr.Error()
		}
	}

	if n, found, blocked, sErr := crawlSitemapPosts(ctx, origin, robots, cutoff); found {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: "estimate", Source: "sitemap", Status: "ok"}
	} else {
		sitemapBlocked = blocked
		if sErr != nil {
			lastErr = sErr.Error()
		}
	}

	// Google News RSS: index pihak ketiga, tidak lewat IP kita, jadi tetap
	// terbaca untuk situs berita yang diblokir WAF.
	googleNewsNote := "Google News tidak punya artikel 7 hari"
	if n, found, gErr := crawlGoogleNewsPosts(ctx, origin, cutoff); found {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: "estimate", Source: "google_news", Status: "ok"}
	} else if gErr != nil {
		lastErr = gErr.Error()
		googleNewsNote = gErr.Error()
	}

	waybackLimited := false
	waybackNote := "Wayback tidak punya capture 7 hari"
	if n, found, wErr := crawlWaybackPosts(ctx, origin, cutoff); found && n > 0 {
		return crawlFallbackResult{Articles: crawlIntPtr(n), Confidence: "estimate", Source: "wayback", Status: "ok"}
	} else if wErr != nil {
		lastErr = wErr.Error()
		waybackNote = wErr.Error()
		if strings.Contains(wErr.Error(), "429") || strings.Contains(wErr.Error(), "cooldown") {
			waybackLimited = true
		}
	}

	// Probe halaman utama: bedakan blokir WAF, situs error (5xx), domain parkir,
	// atau memang tak punya sumber — supaya alasan di UI tidak menyesatkan.
	homeBlocked := false
	homeCode, _, homeBody, homeErr := crawlFallbackGetSite(ctx, origin+"/")
	if homeErr == nil {
		switch {
		case homeCode >= 500:
			return crawlFallbackResult{Status: "error", Error: fmt.Sprintf("situs error (HTTP %d) — origin tidak merespons", homeCode)}
		case crawlBlockedStatus(homeCode):
			homeBlocked = true
		case homeCode == http.StatusOK && strings.Contains(homeBody, "/lander"):
			return crawlFallbackResult{Status: "no_source", Error: "situs tampak parkir/tidak aktif"}
		}
	}

	if robotsBlocked || wpBlocked || feedBlocked || sitemapBlocked || homeBlocked {
		parts := []string{}
		if robotsBlocked {
			parts = append(parts, "robots 403")
		}
		if wpBlocked {
			parts = append(parts, "WP API 403")
		}
		if feedBlocked {
			parts = append(parts, "feed 403")
		}
		if sitemapBlocked {
			parts = append(parts, "sitemap 403")
		}
		if homeBlocked {
			parts = append(parts, fmt.Sprintf("homepage %d", homeCode))
		}
		parts = append(parts, googleNewsNote, waybackNote)
		return crawlFallbackResult{Status: "blocked", Error: "diblokir WAF (HTTP 403/429): " + strings.Join(parts, " · ")}
	}
	if lastErr != "" {
		// Wayback rate-limit bukan kegagalan situs: laporkan sebagai "tidak ada
		// sumber" agar UI tidak menampilkan "Gagal: wayback 429" berulang.
		if waybackLimited {
			return crawlFallbackResult{Status: "no_source", Error: "tidak ada sitemap/feed/API yang terbaca; Wayback sedang rate-limit"}
		}
		return crawlFallbackResult{Status: "error", Error: lastErr}
	}
	return crawlFallbackResult{Status: "no_source", Error: "tidak ada sitemap/feed/API yang terbaca"}
}

func crawlIntPtr(n int) *int { return &n }

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
		        $6::text, CASE WHEN $6::text = 'ok' THEN 0 ELSE 1 END, $7, NOW())
		ON CONFLICT (media_id) DO UPDATE SET
		  website_url = EXCLUDED.website_url,
		  articles_7d = COALESCE(EXCLUDED.articles_7d, crawl_targets.articles_7d),
		  confidence = COALESCE(EXCLUDED.confidence, crawl_targets.confidence),
		  source = COALESCE(EXCLUDED.source, crawl_targets.source),
		  last_crawled_at = CASE WHEN EXCLUDED.articles_7d IS NOT NULL THEN NOW() ELSE crawl_targets.last_crawled_at END,
		  last_status = EXCLUDED.last_status,
		  consecutive_failures = CASE
		    WHEN EXCLUDED.last_status = 'ok' THEN 0
		    ELSE crawl_targets.consecutive_failures + 1
		  END,
		  last_error = EXCLUDED.last_error,
		  updated_at = NOW()`,
		mediaID, websiteURL, articles, nullIfEmpty(fb.Confidence), nullIfEmpty(fb.Source), fb.Status, lastErr)
}

func crawlBlockedStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusTooManyRequests
}

// ---------- HTTP + robots ----------

func crawlFallbackGet(ctx context.Context, target string) (int, http.Header, string, error) {
	return crawlFallbackGetWith(ctx, target, mjCrawlerHeaders)
}

func crawlFallbackGetWith(ctx context.Context, target string, headers map[string]string) (int, http.Header, string, error) {
	resp, err := security.Get(ctx, target, headers, crawlFallbackReqTimeout)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, crawlFallbackBodyLimit))
	return resp.StatusCode, resp.Header, string(body), nil
}

// crawlFallbackGetSite: request ke SITUS MEDIA. Egress Cloudflare dicoba lebih
// dulu (IP-nya biasanya tidak diblokir WAF mitra); bila egress tidak
// dikonfigurasi/gagal, jatuh ke koneksi langsung dari server.
func crawlFallbackGetSite(ctx context.Context, target string) (int, http.Header, string, error) {
	if status, hdr, body, ok := crawlEgressGet(ctx, target); ok {
		slog.Debug("[crawl-egress] dipakai", "target", target, "status", status)
		return status, hdr, body, nil
	}
	slog.Debug("[crawl-egress] langsung", "target", target)
	return crawlFallbackGet(ctx, target)
}

// crawlRetryAfter: detik dari header Retry-After, dibatasi agar tidak menahan
// job terlalu lama. Nilai bukan angka (tanggal HTTP) diabaikan.
func crawlRetryAfter(raw string, fallback time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return fallback
	}
	if secs > 10 {
		secs = 10
	}
	return time.Duration(secs) * time.Second
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

func crawlHost(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
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
	status, _, body, err := crawlFallbackGetSite(ctx, origin+"/robots.txt")
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
		status, hdr, _, err := crawlFallbackGetSite(ctx, a.base+sep+q.Encode())
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
		status, _, body, err := crawlFallbackGetSite(ctx, a.base+sep+q.Encode())
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
	status, _, body, err := crawlFallbackGetSite(ctx, origin+"/")
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
		status, _, body, err := crawlFallbackGetSite(ctx, target)
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
		status, _, body, err := crawlFallbackGetSite(ctx, origin+path)
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
				childStatus, _, childBody, childErr := crawlFallbackGetSite(ctx, loc)
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

// ---------- Google News RSS (index pihak ketiga) ----------

// crawlGoogleNewsPosts: hitung item Google News untuk `site:<host>` dalam 7
// hari. Feed ini diambil dari Google, bukan dari situs media, jadi tidak kena
// WAF/Cloudflare mitra. Cocok untuk media berita; blog yang tidak terindeks
// akan menghasilkan 0 dan fallback lanjut ke Wayback.
func crawlGoogleNewsPosts(ctx context.Context, origin string, cutoff time.Time) (int, bool, error) {
	host := crawlHost(origin)
	if host == "" {
		return 0, false, errors.New("host kosong")
	}
	q := url.Values{}
	q.Set("q", "site:"+host)
	q.Set("hl", "id")
	q.Set("gl", "ID")
	q.Set("ceid", "ID:id")
	target := "https://news.google.com/rss/search?" + q.Encode()
	headers := map[string]string{
		"user-agent": crawlWaybackUA,
		"accept":     "application/rss+xml,application/atom+xml,application/xml,text/xml",
	}

	crawlNewsMu.Lock()
	defer crawlNewsMu.Unlock()
	if !crawlNewsLast.IsZero() {
		if wait := crawlNewsMinGap - time.Since(crawlNewsLast); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return 0, false, ctx.Err()
			}
		}
	}
	status, hdr, body, err := crawlFallbackGetWith(ctx, target, headers)
	if err == nil && status == http.StatusTooManyRequests {
		wait := crawlRetryAfter(hdr.Get("Retry-After"), 3*time.Second)
		select {
		case <-time.After(wait):
			status, _, body, err = crawlFallbackGetWith(ctx, target, headers)
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	crawlNewsLast = time.Now()
	if err != nil {
		return 0, false, err
	}
	if status == http.StatusTooManyRequests {
		return 0, false, errors.New("google news HTTP 429")
	}
	if status != http.StatusOK {
		return 0, false, fmt.Errorf("google news HTTP %d", status)
	}
	count, ok := crawlCountFeed(body, cutoff)
	if !ok || count == 0 {
		// 0/format tak dikenal bukan bukti media stagnan; biarkan Wayback
		// menjadi penentu terakhir.
		return 0, false, nil
	}
	return count, true, nil
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
	target := "https://web.archive.org/cdx/search/cdx?" + q.Encode()
	headers := map[string]string{
		"user-agent": crawlWaybackUA,
		"accept":     "application/json",
	}

	// Serialkan seluruh panggilan Wayback + jeda minimum. Tanpa ini, satu sweep
	// puluhan media diblokir langsung kena 429 dari archive.org.
	crawlWaybackMu.Lock()
	defer crawlWaybackMu.Unlock()
	if time.Now().Before(crawlWaybackCooldownT) {
		return 0, false, errors.New("wayback cooldown (429 sebelumnya)")
	}
	if !crawlWaybackLast.IsZero() {
		if wait := crawlWaybackMinGap - time.Since(crawlWaybackLast); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return 0, false, ctx.Err()
			}
		}
	}

	status, hdr, body, err := crawlFallbackGetWith(ctx, target, headers)
	if err == nil && status == http.StatusTooManyRequests {
		wait := crawlRetryAfter(hdr.Get("Retry-After"), 4*time.Second)
		select {
		case <-time.After(wait):
			status, _, body, err = crawlFallbackGetWith(ctx, target, headers)
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	crawlWaybackLast = time.Now()
	if err != nil {
		return 0, false, err
	}
	if status == http.StatusTooManyRequests {
		crawlWaybackCooldownT = time.Now().Add(crawlWaybackCooldown)
		return 0, false, errors.New("wayback HTTP 429 (rate limit archive.org)")
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
