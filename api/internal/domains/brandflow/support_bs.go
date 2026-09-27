package brandflow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	authpkg "github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// support_bs.go: helper khusus modul klien (integrations/content/media/finance/
// reports) — format angka ala Intl id-ID, parsing judul artikel, domain media,
// klien Crawlhub, secret + test koneksi CMS, tanda tangan event webhook, dan
// pemetaan error bersama.

// ---------- error ----------

// bsErr: writeErr + pemetaan AuthRateLimitError → 429 (parity checkWriteRate).
func bsErr(d *Deps, w http.ResponseWriter, err error) {
	var rle *authpkg.AuthRateLimitError
	if errors.As(err, &rle) {
		httpx.Error(w, http.StatusTooManyRequests, rle.Error())
		return
	}
	d.writeErr(w, err)
}

// bsErrStatus: error dengan status eksplisit + pesan.
func bsErrStatus(status int, msg string) error { return &bfStatusError{Status: status, Message: msg} }

// ---------- format angka (Intl.NumberFormat "id-ID") ----------

// bsGroupID: pengelompokan ribuan gaya id-ID (2.500).
func bsGroupID(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	digits := strconv.FormatInt(n, 10)
	var parts []string
	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}
	parts = append([]string{digits}, parts...)
	out := strings.Join(parts, ".")
	if neg {
		return "-" + out
	}
	return out
}

// bsRound1: pembulatan 1 desimal half-up (parity Intl maximumFractionDigits:1).
func bsRound1(v float64) float64 { return math.Floor(v*10+0.5) / 10 }

// bsNum1: angka dengan maksimal 1 desimal, desimal "," (id-ID).
func bsNum1(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	rounded := bsRound1(v)
	whole := math.Floor(rounded)
	frac := int(math.Round((rounded - whole) * 10))
	out := bsGroupID(int64(whole))
	if frac != 0 {
		out += "," + strconv.Itoa(frac)
	}
	if neg {
		return "-" + out
	}
	return out
}

// bsIntlIDR: parity formatIdr — Intl currency IDR tanpa desimal ("Rp 2.500").
func bsIntlIDR(v float64) string {
	neg := v < 0
	sign := ""
	if neg {
		sign = "-"
		v = -v
	}
	rounded := int64(math.Floor(v + 0.5))
	return sign + "Rp\u00a0" + bsGroupID(rounded)
}

// bsIntlCompact: parity formatCompact — Intl notation compact, 1 desimal
// ("1 rb", "1,5 jt", "2 M"). Pembulatan 1 desimal bisa membawa mantissa ke
// 1000 (999.950 → "1 jt"), jadi unit dinaikkan bila itu terjadi.
func bsIntlCompact(v float64) string {
	neg := v < 0
	sign := ""
	if neg {
		sign = "-"
		v = -v
	}
	divs := []float64{1e12, 1e9, 1e6, 1e3}
	sufs := []string{"T", "M", "jt", "rb"}
	for i, div := range divs {
		if v < div {
			continue
		}
		mantissa := bsRound1(v / div)
		if mantissa >= 1000 {
			if i == 0 {
				return sign + bsNum1(v/1e12) + "\u00a0T"
			}
			div = divs[i-1]
			return sign + bsNum1(v/div) + "\u00a0" + sufs[i-1]
		}
		return sign + bsNum1(v/div) + "\u00a0" + sufs[i]
	}
	return sign + bsNum1(v)
}

// ---------- parsing angka/limit ala JS ----------

// bsJSNumber: parity Number(raw) — "" → 0, selain itu parseFloat.
func bsJSNumber(raw string) float64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	f, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return math.NaN()
	}
	return f
}

// bsQueryNumber: nilai searchParams.get(key) (default bila parameter absen).
func bsQueryNumber(values url.Values, key string, def float64) float64 {
	if !values.Has(key) {
		return def
	}
	return bsJSNumber(values.Get(key))
}

// bsNumClampInt: Math.min(Math.max(v, min), max) lalu int; NaN → min.
func bsNumClampInt(v float64, min, max int) int {
	if math.IsNaN(v) {
		return min
	}
	if v < float64(min) {
		return min
	}
	if v > float64(max) {
		return max
	}
	return int(v)
}

// ---------- judul artikel (parity articleTitle) ----------

var reH1Lazy = regexp.MustCompile(`(?is)<h1[^>]*>([\s\S]*?)</h1>`)

// bsArticleTitle: teks H1 pertama (tag dibuang, spasi dirapatkan).
func bsArticleTitle(html string) *string {
	m := reH1Lazy.FindStringSubmatch(html)
	if m == nil {
		return nil
	}
	text := strings.TrimSpace(reWhitespace.ReplaceAllString(reHTMLTag.ReplaceAllString(m[1], " "), " "))
	if text == "" {
		return nil
	}
	return &text
}

// ---------- domain media (parity domainFromWebsite) ----------

// bsDomainFromWebsite: hostname dari website_url media; error pesan Indonesia.
func bsDomainFromWebsite(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", bsErrStatus(http.StatusInternalServerError, "Media belum memiliki website/domain.")
	}
	candidate := raw
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Hostname() == "" {
		return "", bsErrStatus(http.StatusInternalServerError, "Website/domain media tidak valid.")
	}
	hostname := strings.ToLower(parsed.Hostname())
	hostname = strings.TrimPrefix(hostname, "www.")
	if hostname == "" || bsUnsafeHost(hostname) {
		return "", bsErrStatus(http.StatusInternalServerError, "Website/domain media tidak dapat diperiksa.")
	}
	return hostname, nil
}

func bsUnsafeHost(hostname string) bool {
	if hostname == "localhost" || hostname == "::1" || strings.HasSuffix(hostname, ".local") || strings.HasSuffix(hostname, ".internal") {
		return true
	}
	ip := net.ParseIP(hostname)
	if ip != nil && ip.To4() != nil {
		o := ip.To4()
		return o[0] == 10 || o[0] == 127 ||
			(o[0] == 172 && o[1] >= 16 && o[1] <= 31) ||
			(o[0] == 192 && o[1] == 168)
	}
	return false
}

// ---------- Crawlhub (port lib/crawlhub/client.ts) ----------

type bsCrawlhubConfig struct {
	BaseURL      string
	HMACSecret   string
	IngestSecret string
}

func bsCrawlhubConfigured(cfg bsCrawlhubConfig) bool {
	return cfg.BaseURL != "" && cfg.HMACSecret != ""
}

// bsLoadCrawlhubConfig: DB menang, env jadi nilai awal; secret tersegel yang
// gagal dibuka dianggap kosong (bukan error).
func (d *Deps) bsLoadCrawlhubConfig(ctx context.Context) bsCrawlhubConfig {
	cfg := bsCrawlhubConfig{
		BaseURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("CRAWLHUB_BASE_URL")), "/"),
		HMACSecret:   strings.TrimSpace(os.Getenv("CRAWLHUB_HMAC_SECRET")),
		IngestSecret: strings.TrimSpace(os.Getenv("CRAWLHUB_INGEST_SECRET")),
	}
	row, err := d.queryRow(ctx,
		"SELECT base_url, sealed_hmac_secret, sealed_ingest_secret FROM platform_crawlhub_config WHERE id = 'default' LIMIT 1")
	if err != nil || row == nil {
		return cfg
	}
	if base := strings.TrimSpace(row.strOr("base_url", "")); base != "" {
		cfg.BaseURL = strings.TrimRight(base, "/")
	}
	open := func(sealed string, fallback string) string {
		if sealed == "" {
			return fallback
		}
		if plain, uErr := d.unsealT2(sealed); uErr == nil {
			return plain
		}
		return ""
	}
	cfg.HMACSecret = open(row.strOr("sealed_hmac_secret", ""), cfg.HMACSecret)
	cfg.IngestSecret = open(row.strOr("sealed_ingest_secret", ""), cfg.IngestSecret)
	return cfg
}

// bsCrawlhubGet: GET bertanda tangan, balikan JSON atau error pesan.
func (d *Deps) bsCrawlhubGet(ctx context.Context, cfg bsCrawlhubConfig, path string) (map[string]any, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(cfg.HMACSecret))
	mac.Write([]byte(timestamp + "."))
	digest := hex.EncodeToString(mac.Sum(nil))

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, cfg.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-crawlhub-timestamp", timestamp)
	req.Header.Set("x-crawlhub-signature", "sha256="+digest)

	resp, err := security.SafeClient(15 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateString(string(text), 300))
	}
	var parsed map[string]any
	if err := json.Unmarshal(text, &parsed); err != nil {
		return nil, errors.New("respons Crawlhub bukan JSON")
	}
	return parsed, nil
}

// bsFetchCrawlhubTargetStatus: GET /v1/targets/{mediaId}/status.
func (d *Deps) bsFetchCrawlhubTargetStatus(ctx context.Context, cfg bsCrawlhubConfig, mediaID string) (map[string]any, error) {
	return d.bsCrawlhubGet(ctx, cfg, "/v1/targets/"+url.PathEscape(mediaID)+"/status")
}

// ---------- koneksi CMS (parity _cms.connectionSecret + testConnection) ----------

// bsConnectionSecret: enc_key_ver >= 1 → kunci per-entitas (t3:); ver 0 →
// WordPress pakai APP_SECRET, provider lain CMS_ENCRYPTION_SECRET.
func (d *Deps) bsConnectionSecret(row bsRow) (string, error) {
	sealed := row.strOr("sealed_secret", "")
	if sealed == "" {
		return "", nil
	}
	if row.intOr("enc_key_ver", 0) >= 1 {
		return crypto.UnsealV3(sealed, d.Cfg.Secret, row.strOr("id", ""))
	}
	secret := d.Cfg.Secret
	if row.strOr("provider", "wordpress") != "wordpress" {
		secret = d.Cfg.CmsEncryptionSecret
		if secret == "" {
			secret = d.Cfg.Secret
		}
	}
	return crypto.Unseal(sealed, secret)
}

func bsWPURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/wp-json/" + path
}

// bsHTTPDo: request dengan guard SSRF + timeout, balikan status + body.
func bsHTTPDo(ctx context.Context, method, target string, headers map[string]string, body string, timeout time.Duration) (int, string, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := security.SafeClient(timeout).Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return 0, "", readErr
	}
	return resp.StatusCode, string(raw), nil
}

// bsEventHeaders: parity eventHeaders lib/push/signature.ts.
func bsEventHeaders(secret, event, eventID, rawBody string) map[string]string {
	timestamp := isoNow()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + eventID + "." + rawBody))
	return map[string]string{
		"content-type":        "application/json",
		"x-tulisio-event":     event,
		"x-tulisio-event-id":  eventID,
		"x-tulisio-timestamp": timestamp,
		"x-tulisio-signature": "sha256=" + hex.EncodeToString(mac.Sum(nil)),
	}
}

// bsTestWordPress: parity testWordPress (users/me + capabilities.edit_posts).
func (d *Deps) bsTestWordPress(ctx context.Context, conn bsRow) (string, string) {
	secret, _ := d.bsConnectionSecret(conn)
	basic := base64.StdEncoding.EncodeToString([]byte(conn.strOr("username", "") + ":" + secret))
	status, body, err := bsHTTPDo(ctx, http.MethodGet,
		bsWPURL(conn.strOr("base_url", ""), "wp/v2/users/me?context=edit"),
		map[string]string{"Authorization": "Basic " + basic}, "", 20*time.Second)
	if err != nil {
		return "error", truncateString(err.Error(), 500)
	}
	if status < 200 || status >= 300 {
		return "error", truncateString(fmt.Sprintf("WordPress returned %d: %s", status, body), 500)
	}
	var profile struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
		Cap  *struct {
			EditPosts bool `json:"edit_posts"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal([]byte(body), &profile); err != nil {
		return "error", truncateString(err.Error(), 500)
	}
	who := profile.Slug
	if who == "" {
		who = profile.Name
	}
	if who == "" {
		who = conn.strOr("username", "")
	}
	if profile.Cap == nil || !profile.Cap.EditPosts {
		return "error", truncateString(fmt.Sprintf("WordPress authenticated as %s, but edit_posts is denied", who), 500)
	}
	return "ok", fmt.Sprintf("Connected as %s with post creation permission", who)
}

// bsTestWebhook: parity testWebhook (ping content.ping → ok:true).
func (d *Deps) bsTestWebhook(ctx context.Context, conn bsRow) (string, string) {
	secret, _ := d.bsConnectionSecret(conn)
	eventID := uuid.NewString()
	raw, _ := json.Marshal(map[string]any{
		"event": "content.ping", "event_id": eventID, "timestamp": isoNow(),
	})
	status, text, err := bsHTTPDo(ctx, http.MethodPost, conn.strOr("base_url", ""),
		bsEventHeaders(secret, "content.ping", eventID, string(raw)), string(raw), 20*time.Second)
	if err != nil {
		return "error", truncateString(err.Error(), 500)
	}
	if status < 200 || status >= 300 {
		return "error", truncateString(fmt.Sprintf("Webhook returned %d: %s", status, text), 500)
	}
	var parsed struct {
		OK *bool `json:"ok"`
	}
	if json.Unmarshal([]byte(text), &parsed) != nil || parsed.OK == nil || !*parsed.OK {
		return "error", truncateString(fmt.Sprintf("Webhook did not respond with ok:true (%s)", truncateString(text, 200)), 500)
	}
	return "ok", "Webhook received ping and responded ok:true"
}

// bsTestCMSConnection: parity testConnection (webhook vs WordPress).
func (d *Deps) bsTestCMSConnection(ctx context.Context, conn bsRow) (string, string) {
	if conn.strOr("provider", "wordpress") == "webhook" {
		return d.bsTestWebhook(ctx, conn)
	}
	return d.bsTestWordPress(ctx, conn)
}

// ---------- markdown executor publish (port publish/executor.ts) ----------

var (
	rePubItalic = regexp.MustCompile(`(^|[\s(])\*([^*\n]+)\*`)
	rePubLink   = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\s)]+)\)`)
	rePubCode   = regexp.MustCompile("`([^`]+)`")
	rePubBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	rePubH      = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	rePubOl     = regexp.MustCompile(`^\d+[.)]\s+(.*)$`)
	rePubUl     = regexp.MustCompile(`^[-*+•]\s+(.*)$`)
	reStripH1   = regexp.MustCompile(`(?is)<h1(?:\s[^>]*)?>[\s\S]*?</h1>`)
)

func bsPublishInline(text string) string {
	out := strings.ReplaceAll(text, "&", "&amp;")
	out = strings.ReplaceAll(out, "<", "&lt;")
	out = strings.ReplaceAll(out, ">", "&gt;")
	out = rePubCode.ReplaceAllString(out, "<code>$1</code>")
	out = rePubBold.ReplaceAllString(out, "<strong>$1</strong>")
	out = bsReplaceItalic(out)
	out = rePubLink.ReplaceAllString(out, `<a href="$2">$1</a>`)
	return out
}

// bsReplaceItalic: pengganti regex ber-lookahead JS
// `(^|[\s(])\*([^*\n]+)\*(?=[\s).,!?:;]|$)` (RE2 tidak mendukung lookahead).
func bsReplaceItalic(text string) string {
	locs := rePubItalic.FindAllStringSubmatchIndex(text, -1)
	if len(locs) == 0 {
		return text
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		end := loc[1]
		if end < len(text) {
			next := text[end]
			if !strings.ContainsRune(" \t\r\n).,!?:;", rune(next)) {
				continue
			}
		}
		b.WriteString(text[last:loc[0]])
		b.WriteString(text[loc[2]:loc[3]])
		b.WriteString("<em>")
		b.WriteString(text[loc[4]:loc[5]])
		b.WriteString("</em>")
		last = loc[1]
	}
	b.WriteString(text[last:])
	return b.String()
}

// bsPublishMarkdown: parity markdownToHtml lokal executor.ts (tanpa blockquote,
// tanpa gambar, tanpa skip "---").
func bsPublishMarkdown(markdown string) string {
	html := []string{}
	listType := ""
	paragraph := []string{}
	flush := func() {
		if len(paragraph) > 0 {
			html = append(html, "<p>"+bsPublishInline(strings.Join(paragraph, " "))+"</p>")
			paragraph = []string{}
		}
	}
	closeList := func() {
		if listType != "" {
			html = append(html, "</"+listType+">")
			listType = ""
		}
	}
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	for _, raw := range strings.Split(normalized, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			closeList()
			continue
		}
		if m := rePubH.FindStringSubmatch(line); m != nil {
			flush()
			closeList()
			level := strconv.Itoa(len(m[1]))
			html = append(html, "<h"+level+">"+bsPublishInline(m[2])+"</h"+level+">")
			continue
		}
		if m := rePubOl.FindStringSubmatch(line); m != nil {
			flush()
			if listType != "ol" {
				closeList()
				html = append(html, "<ol>")
				listType = "ol"
			}
			html = append(html, "<li>"+bsPublishInline(m[1])+"</li>")
			continue
		}
		if m := rePubUl.FindStringSubmatch(line); m != nil {
			flush()
			if listType != "ul" {
				closeList()
				html = append(html, "<ul>")
				listType = "ul"
			}
			html = append(html, "<li>"+bsPublishInline(m[1])+"</li>")
			continue
		}
		closeList()
		paragraph = append(paragraph, line)
	}
	flush()
	closeList()
	return strings.Join(html, "\n")
}

// bsStripH1: buang H1 pertama (parity stripH1).
func bsStripH1(html string) string { return reStripH1.ReplaceAllString(html, "") }

// bsSlugify: parity slugify executor.ts.
func bsSlugify(value string) string {
	lower := strings.ToLower(value)
	var b strings.Builder
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

// ---------- GA4 ----------

// bsGA4EventMap: parity GA4_EVENT_MAP lib/ga4/service.ts.
var bsGA4EventMap = map[string]string{
	"article_view":        "article_view",
	"article_scroll":      "article_scroll",
	"outbound_click":      "outbound_click",
	"cta_click":           "cta_click",
	"tracking_link_click": "tracking_link_click",
}

// ---------- editorial standard resolver (khusus sumber + nama media) ----------

// bsResolveStandardSource: parity resolveEditorialStandard hanya untuk
// `source` + `mediaName` (dipakai auto-key-messages).
func (d *Deps) bsResolveStandardSource(ctx context.Context, mediaID string, mediaType *string) (string, string) {
	mt := ""
	if mediaType != nil {
		mt = *mediaType
	}
	mediaName := ""
	if mediaID != "" {
		media, err := d.queryRow(ctx, "SELECT id, name, media_type FROM media WHERE id = $1 LIMIT 1", mediaID)
		if err == nil && media != nil {
			mediaName = media.strOr("name", "")
			if d.mediaStandardRow(ctx, mediaID) != nil {
				return "media_specific", mediaName
			}
		}
	}
	switch mt {
	case "mainstream":
		return "mainstream_fallback", ""
	case "blogger":
		return "blogger_fallback", ""
	}
	return "global", ""
}

// ---------- standar PR (port lib/media/intelligence.ts) ----------

type bsPrRates struct {
	Global *float64
	Tier1  *float64
	Tier2  *float64
	Tier3  *float64
}

type bsBrandPrRates struct {
	Rates     bsPrRates
	Source    string
	UpdatedAt any
}

// bsResolvePrRates: parity resolvePrRates (tier kosong → global).
func bsResolvePrRates(rates bsPrRates) map[string]float64 {
	global := 0.0
	if rates.Global != nil {
		if v := *rates.Global; v > 0 {
			global = v
		}
	}
	pick := func(v *float64) float64 {
		if v == nil {
			return global
		}
		n := *v
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return global
		}
		return n
	}
	return map[string]float64{
		"global": global, "tier1": pick(rates.Tier1),
		"tier2": pick(rates.Tier2), "tier3": pick(rates.Tier3),
	}
}

// bsLoadBrandPrRates: parity loadBrandPrRates (pr_config + override brand).
func (d *Deps) bsLoadBrandPrRates(ctx context.Context, brandID string) bsBrandPrRates {
	cfg, _ := d.queryRow(ctx,
		"SELECT rate_per_1k_reach, rate_tier1, rate_tier2, rate_tier3, updated_at FROM pr_config WHERE id = 'default' LIMIT 1")
	platformGlobal := 5000.0
	if cfg != nil {
		if cfg.has("rate_per_1k_reach") {
			if v := cfg.numOr("rate_per_1k_reach"); v > 0 {
				platformGlobal = v
			}
		}
	}
	platformTier := func(row bsRow, key string) *float64 {
		if row == nil || !row.has(key) {
			return nil
		}
		v := row.numOr(key)
		return &v
	}
	var cfgUpdatedAt any
	if cfg != nil {
		cfgUpdatedAt = cfg.timePtr("updated_at")
	}
	brandStd, _ := d.queryRow(ctx,
		"SELECT rate_per_1k_reach, rate_tier1, rate_tier2, rate_tier3, updated_at FROM brand_pr_standards WHERE brand_id = $1 LIMIT 1", brandID)
	if brandStd == nil {
		return bsBrandPrRates{
			Rates: bsPrRates{
				Global: &platformGlobal,
				Tier1:  platformTier(cfg, "rate_tier1"),
				Tier2:  platformTier(cfg, "rate_tier2"),
				Tier3:  platformTier(cfg, "rate_tier3"),
			},
			Source: "global", UpdatedAt: cfgUpdatedAt,
		}
	}
	brandGlobal := platformGlobal
	if brandStd.has("rate_per_1k_reach") {
		brandGlobal = brandStd.numOr("rate_per_1k_reach")
	}
	pick := func(brandVal *float64, platformVal *float64) *float64 {
		if brandVal != nil {
			return brandVal
		}
		if platformVal != nil {
			return platformVal
		}
		return &brandGlobal
	}
	rates := bsPrRates{Global: &brandGlobal}
	rates.Tier1 = pick(platformTier(brandStd, "rate_tier1"), platformTier(cfg, "rate_tier1"))
	rates.Tier2 = pick(platformTier(brandStd, "rate_tier2"), platformTier(cfg, "rate_tier2"))
	rates.Tier3 = pick(platformTier(brandStd, "rate_tier3"), platformTier(cfg, "rate_tier3"))
	return bsBrandPrRates{Rates: rates, Source: "brand", UpdatedAt: brandStd.timePtr("updated_at")}
}

type bsPrValueEstimate struct {
	Published         int
	EstimatedMediaVal float64
	EstimatedReach    int64
	CampaignCost      float64
	EstimatedSaving   float64
	EstimatedPRValue  float64
}

// bsEstimatePRValue: parity estimatePRValue (media value + earned per tier).
func (d *Deps) bsEstimatePRValue(ctx context.Context, brandID string, campaignID *string, from, to string, rates *bsPrRates, ratePer1kReach *float64) bsPrValueEstimate {
	scope := []string{"p.brand_id = $1"}
	params := []any{brandID}
	if campaignID != nil && *campaignID != "" {
		params = append(params, *campaignID)
		scope = append(scope, "p.campaign_id = $"+itoa(len(params)))
	}
	if from != "" {
		params = append(params, from+"T00:00:00.000Z")
		scope = append(scope, "p.published_at >= $"+itoa(len(params)))
	}
	if to != "" {
		params = append(params, bsDayAfterISO(to))
		scope = append(scope, "p.published_at < $"+itoa(len(params)))
	}

	publishedRow, _ := d.queryRow(ctx, `
		SELECT
		  COUNT(*)::int AS published,
		  COALESCE(SUM(p.client_price), 0)::numeric(14,2) AS media_value,
		  COALESCE(SUM(CASE WHEN m.media_tier = 'tier1' THEN COALESCE(m.traffic_estimate, 0) ELSE 0 END), 0)::bigint AS reach_tier1,
		  COALESCE(SUM(CASE WHEN m.media_tier = 'tier2' THEN COALESCE(m.traffic_estimate, 0) ELSE 0 END), 0)::bigint AS reach_tier2,
		  COALESCE(SUM(CASE WHEN m.media_tier = 'tier3' THEN COALESCE(m.traffic_estimate, 0) ELSE 0 END), 0)::bigint AS reach_tier3,
		  COALESCE(SUM(CASE WHEN m.media_tier IS NULL OR m.media_tier NOT IN ('tier1','tier2','tier3') THEN COALESCE(m.traffic_estimate, 0) ELSE 0 END), 0)::bigint AS reach_other
		FROM publications p
		LEFT JOIN media m ON m.id = p.media_id
		WHERE p.status = 'published' AND `+strings.Join(scope, " AND "), params...)

	costSQL := "SELECT COALESCE(SUM(budget), 0)::numeric(14,2) AS campaign_cost FROM campaigns WHERE brand_id = $1 AND status IN ('active','completed')"
	costParams := []any{brandID}
	if campaignID != nil && *campaignID != "" {
		costParams = append(costParams, *campaignID)
		costSQL += " AND id = $2"
	}
	costRow, _ := d.queryRow(ctx, costSQL, costParams...)

	campaignCost := 0.0
	if costRow != nil {
		campaignCost = costRow.numOr("campaign_cost")
	}
	published := 0
	mediaValue := 0.0
	var reachT1, reachT2, reachT3, reachOther int64
	if publishedRow != nil {
		published = publishedRow.intOr("published", 0)
		mediaValue = publishedRow.numOr("media_value")
		reachT1 = int64(publishedRow.numOr("reach_tier1"))
		reachT2 = int64(publishedRow.numOr("reach_tier2"))
		reachT3 = int64(publishedRow.numOr("reach_tier3"))
		reachOther = int64(publishedRow.numOr("reach_other"))
	}
	reach := reachT1 + reachT2 + reachT3 + reachOther

	flatRate := 0.0
	if ratePer1kReach != nil {
		if v := *ratePer1kReach; v > 0 {
			flatRate = v
		}
	}
	resolved := map[string]float64{"global": flatRate, "tier1": flatRate, "tier2": flatRate, "tier3": flatRate}
	if rates != nil {
		resolved = bsResolvePrRates(*rates)
	}
	earned := (float64(reachT1)/1000)*resolved["tier1"] +
		(float64(reachT2)/1000)*resolved["tier2"] +
		(float64(reachT3)/1000)*resolved["tier3"] +
		(float64(reachOther)/1000)*resolved["global"]
	return bsPrValueEstimate{
		Published:         published,
		EstimatedMediaVal: mediaValue,
		EstimatedReach:    reach,
		CampaignCost:      campaignCost,
		EstimatedSaving:   math.Max(0, math.Round(mediaValue-campaignCost)),
		EstimatedPRValue:  math.Round(mediaValue + earned),
	}
}

// bsDayAfterISO: "YYYY-MM-DD" → ISO awal hari berikutnya (parity +86400000ms).
func bsDayAfterISO(day string) string {
	t, ok := parseISO(day)
	if !ok {
		t = time.Now().UTC()
	}
	return t.UTC().AddDate(0, 0, 1).Format("2006-01-02T15:04:05.000Z")
}

// bsNormDate: normalisasi tanggal laporan (YYYY-MM-DD prefix) atau fallback.
func bsNormDate(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	m := reDatePrefix.FindStringSubmatch(trimmed)
	if m == nil {
		return fallback
	}
	if _, ok := parseISO(m[1]); !ok {
		return fallback
	}
	return m[1]
}

var reDatePrefix = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)

// bsParsableDate: parity `!Number.isNaN(Date.parse(v))` (ISO/Indonesia).
func bsParsableDate(raw string) bool {
	return prISODateRe.MatchString(strings.TrimSpace(raw))
}

// prISODateRe: bentuk tanggal yang diterima Date.parse (mirror paket platform).
var prISODateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(?:[T ]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?)?$`)

// ---------- util zod ----------

// bsIsURL: parity z.string().url() — skema + host wajib (mailto/ftp diterima,
// path relatif ditolak). Skema "mailto:" tidak punya host → tetap dianggap
// valid seperti zod (format URL WHATWG).
func bsIsURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "" {
		return false
	}
	// Skema tanpa "//" (mailto:, tel:, urn:) valid tanpa host.
	if !strings.Contains(raw[len(scheme):], "://") {
		return true
	}
	return parsed.Host != ""
}

// bsStripSpaces: parity replace(/\s+/g, "") pada application password.
func bsStripSpaces(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// bsParseStringArrayLenient: parity parser keluaran key-messages AI —
// JSON array (filter non-kosong) atau fallback baris (`replace(/^[-*\d.\s]+/,"")`,
// filter panjang > 5). Dipotong `max` item.
func bsParseStringArrayLenient(raw string, max int) []string {
	cleaned := bsStripAIFence(raw)
	var arr []any
	if err := json.Unmarshal([]byte(strings.TrimSpace(cleaned)), &arr); err == nil {
		out := []string{}
		for _, item := range arr {
			s := strings.TrimSpace(bsStringifyAny(item))
			if s != "" {
				out = append(out, s)
			}
			if len(out) >= max {
				break
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	out := []string{}
	for _, line := range strings.Split(cleaned, "\n") {
		s := bsStripListPrefix(line)
		if len(s) > 5 {
			out = append(out, s)
		}
		if len(out) >= max {
			break
		}
	}
	return out
}

// bsStripListPrefix: parity replace(/^[-*\d.\s]+/, "") + trim.
func bsStripListPrefix(line string) string {
	s := line
	idx := 0
	for idx < len(s) {
		c := s[idx]
		if c == '-' || c == '*' || c == '.' || c == ' ' || c == '\t' || c == '\r' || c == '\n' || (c >= '0' && c <= '9') {
			idx++
			continue
		}
		break
	}
	return strings.TrimSpace(s[idx:])
}

// bsStripAIFence: parity replace(/^```(?:json)?\s*/i,"") + replace(/\s*```$/i,"").
func bsStripAIFence(raw string) string {
	out := reAIFencePrefix.ReplaceAllString(raw, "")
	return reAIFenceSuffix.ReplaceAllString(out, "")
}

var (
	reAIFencePrefix = regexp.MustCompile("(?i)^```(?:json)?\\s*")
	reAIFenceSuffix = regexp.MustCompile("(?i)\\s*```$")
)

// bsSealV3ForEntity: seal v3 (t3:) kunci per-entitas (APP_SECRET + entityID).
func (d *Deps) bsSealV3ForEntity(plain, entityID string) (string, error) {
	return crypto.SealV3(plain, d.Cfg.Secret, entityID)
}

// bsRowJSON: bsRow → map JSON dengan time.Time → ISO milidetik (parity
// JSON.stringify(Date)).
func bsRowJSON(row bsRow) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		if t, ok := v.(time.Time); ok {
			out[k] = isoMilli(t)
			continue
		}
		out[k] = v
	}
	return out
}

// ---------- misc ----------

func bsJSONRaw(b []byte, fallback any) any {
	if len(b) == 0 {
		return fallback
	}
	var parsed any
	if err := json.Unmarshal(b, &parsed); err != nil {
		return fallback
	}
	return parsed
}

// bsDedupeStrings: [...new Set(items)] (urutan pertama dipertahankan).
func bsDedupeStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}
