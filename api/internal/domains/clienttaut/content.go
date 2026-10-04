package clienttaut

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Port lib/media/word-count.ts, lib/content/html-text.ts,
// lib/content/normalize-headings.ts, lib/media/embed.ts (stripDisallowedIframes),
// lib/standard/check.ts, lib/app-url.ts, lib/content/uploads.ts (absolutizeUrl),
// lib/taut/matchmaker-ai.ts, lib/taut/matchmaker-context.ts, lib/taut/json.ts.

// ---------- word count (port lib/media/word-count.ts) ----------

type wordCountMode struct {
	min *int
	max *int
}

var wordCountModes = map[string]wordCountMode{
	"sesuai_permintaan": {},
	"kata_300_700":      {min: intPtr(300), max: intPtr(700)},
	"kata_800_1500":     {min: intPtr(800), max: intPtr(1500)},
}

const defaultWordCountMode = "sesuai_permintaan"

func intPtr(v int) *int { return &v }

func normalizeWordCountMode(value *string) string {
	if value == nil {
		return defaultWordCountMode
	}
	if _, ok := wordCountModes[*value]; ok {
		return *value
	}
	return defaultWordCountMode
}

// resolveWordRange: rentang kata; nil bila tidak dibatasi.
func resolveWordRange(mode *string) *wordCountMode {
	entry := wordCountModes[normalizeWordCountMode(mode)]
	if entry.min == nil || entry.max == nil {
		return nil
	}
	return &entry
}

// buildWordRule: port buildWordRule — rentang mitra menang atas aturan tipe media.
func buildWordRule(mode *string, mediaType string) string {
	if r := resolveWordRange(mode); r != nil {
		return "Aturan panjang artikel (WAJIB): tulis artikel " + itoa(*r.min) + "-" + itoa(*r.max) + " kata. Jangan kurang dari " +
			itoa(*r.min) + " kata dan jangan lebih dari " + itoa(*r.max) + " kata."
	}
	if mediaType == "blogger" {
		return "Aturan panjang artikel (WAJIB): media BLOGGER minimal 500 kata dan maksimal 5000 kata."
	}
	return "Aturan panjang artikel (WAJIB): media MAINSTREAM minimal 300 kata."
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------- html-text (port lib/content/html-text.ts) ----------

var (
	// RE2 tidak mendukung backreference, jadi script/style dipisah menjadi dua
	// pola — perilaku penyaringan sama dengan satu regex backreference.
	reScriptBlock = regexp.MustCompile(`(?is)<script[^>]*>.*?</script\s*>`)
	reStyleBlock  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style\s*>`)
	reHTMLTag     = regexp.MustCompile(`<[^>]+>`)
	reWhitespace  = regexp.MustCompile(`\s+`)
	reHref        = regexp.MustCompile(`(?i)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["']`)
)

// stripHiddenBlocks: buang <script>/<style> beserta isinya (pengganti regex
// backreference app lama yang tidak didukung RE2).
func stripHiddenBlocks(html string) string {
	out := reScriptBlock.ReplaceAllString(html, " ")
	return reStyleBlock.ReplaceAllString(out, " ")
}

func decodeEntities(s string) string {
	r := strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&quot;", "\"",
		"&#39;", "'",
		"&apos;", "'",
		"&lt;", "<",
		"&gt;", ">",
	)
	return r.Replace(s)
}

// extractText: HTML → visible text (parity extractText).
func extractText(html string) string {
	if html == "" {
		return ""
	}
	stripped := reScriptBlock.ReplaceAllString(html, " ")
	stripped = reStyleBlock.ReplaceAllString(stripped, " ")
	stripped = reHTMLTag.ReplaceAllString(stripped, " ")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(decodeEntities(stripped), " "))
}

// extractLinks: daftar href absolut dari HTML (unik, tanpa anchor/javascript:).
func extractLinks(html string) []string {
	if html == "" {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, m := range reHref.FindAllStringSubmatch(html, -1) {
		url := strings.TrimSpace(m[1])
		if url == "" || strings.HasPrefix(url, "#") || strings.HasPrefix(url, "javascript:") {
			continue
		}
		if !seen[url] {
			seen[url] = true
			out = append(out, url)
		}
	}
	return out
}

// truncateText: potong ke maxChars (parity truncateText).
func truncateText(s string, maxChars int) (string, bool) {
	if len(s) <= maxChars {
		return s, false
	}
	return s[:maxChars], true
}

// ---------- normalize headings (port lib/content/normalize-headings.ts) ----------

var (
	reMDLine     = regexp.MustCompile(`(?m)^([ \t]*)(#{2,4})[ \t]+(.+?)[ \t]*$`)
	reH1234Open  = regexp.MustCompile(`(?i)<h[1-4]\b[^>]*>`)
	reH1234Close = regexp.MustCompile(`(?i)</h[1-4]\b[^>]*>`)
)

// demoteHeadings: turunkan SEMUA heading h1-h4 jadi paragraf (media mainstream).
func demoteHeadings(html string) string {
	if html == "" || !strings.Contains(html, "<h") {
		return html
	}
	out := reH1234Open.ReplaceAllString(html, "<p>")
	return reH1234Close.ReplaceAllString(out, "</p>")
}

// demoteH1ToH2: H1 dilarang di isi artikel (judul hidup di field judul
// terpisah); H1 yang lolos dari model/manual diturunkan deterministik jadi H2.
var (
	reH1OpenTag  = regexp.MustCompile(`(?i)<h1\b[^>]*>`)
	reH1CloseTag = regexp.MustCompile(`(?i)</h1\s*>`)
)

func demoteH1ToH2(html string) string {
	if html == "" || !strings.Contains(strings.ToLower(html), "<h1") {
		return html
	}
	out := reH1OpenTag.ReplaceAllString(html, "<h2>")
	return reH1CloseTag.ReplaceAllString(out, "</h2>")
}

// normalizeHeadings: port normalizeHeadings — blok <p>/<div> yang jelas judul
// dinaikkan jadi <h2>, sisa markdown ##/###/#### jadi heading. Idempoten.
func normalizeHeadings(html string) string {
	if html == "" || !strings.Contains(html, "<") {
		return html
	}
	return transformHeadings(html, 0)
}

func transformHeadings(html string, depth int) string {
	if depth > 4 {
		return html
	}
	segments := splitTopLevel(html)
	blockCount := 0
	var only *headingSegment
	strayText := false
	for i := range segments {
		if segments[i].kind == "block" {
			blockCount++
			only = &segments[i]
		} else if strings.TrimSpace(segments[i].raw) != "" {
			strayText = true
		}
	}
	// Satu pembungkus div/section/article memuat seluruh isi → scan isinya.
	if !strayText && blockCount == 1 && only != nil {
		if isWrapperTag(only.tag) && strings.HasSuffix(strings.ToLower(only.raw), "</"+only.tag+">") {
			startIdx := strings.Index(html, only.raw)
			if startIdx >= 0 {
				prefix := html[:startIdx]
				suffix := html[startIdx+len(only.raw):]
				return prefix + openTag(only.tag, only.attrs) + transformHeadings(only.inner, depth+1) + "</" + only.tag + ">" + suffix
			}
		}
	}
	return transformSegments(segments)
}

type headingSegment struct {
	kind  string // "block" | "text"
	tag   string
	attrs string
	raw   string
	inner string
}

var (
	reTagToken   = regexp.MustCompile(`<(\/?)\s*([a-zA-Z][a-zA-Z0-9]*)\b((?:[^>"']|"[^"]*"|'[^']*')*?)(\/?)>`)
	reMDBLock    = regexp.MustCompile(`^(#{2,4})[ \t]+(.+?)[ \t]*$`)
	reNestedBlok = regexp.MustCompile(`(?i)<(?:p|div|h[1-6]|ul|ol|li|dl|dt|dd|blockquote|pre|code|table|figure|section|article|form)\b`)
)

var structuralTags = map[string]bool{
	"p": true, "div": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"blockquote": true, "pre": true, "code": true, "table": true, "figure": true, "figcaption": true,
	"section": true, "article": true, "aside": true, "header": true, "footer": true, "nav": true, "form": true,
}

var voidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true,
	"input": true, "link": true, "meta": true, "param": true, "source": true, "track": true, "wbr": true,
}

var wrapperTags = map[string]bool{"div": true, "section": true, "article": true}

type tagToken struct {
	start     int
	end       int
	closing   bool
	tag       string
	attrs     string
	selfClose bool
}

func tokenizeTags(html string) []tagToken {
	out := []tagToken{}
	for _, m := range reTagToken.FindAllStringSubmatchIndex(html, -1) {
		out = append(out, tagToken{
			start:     m[0],
			end:       m[1],
			closing:   m[2] >= 0 && m[3] > m[2],
			tag:       strings.ToLower(html[m[4]:m[5]]),
			attrs:     html[m[6]:m[7]],
			selfClose: m[8] >= 0 && m[9] > m[8],
		})
	}
	return out
}

func findBlockEnd(tokens []tagToken, openIdx int, tag string, htmlLen int) (closeStart, end, closeIdx int) {
	depth := 0
	for j := openIdx; j < len(tokens); j++ {
		t := tokens[j]
		if t.tag != tag {
			continue
		}
		if t.closing {
			depth--
			if depth == 0 {
				return t.start, t.end, j
			}
		} else if !t.selfClose {
			depth++
		}
	}
	return htmlLen, htmlLen, len(tokens) - 1
}

func splitTopLevel(html string) []headingSegment {
	tokens := tokenizeTags(html)
	segments := []headingSegment{}
	textStart := 0
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		isBlockOpen := !t.closing && !t.selfClose && structuralTags[t.tag] && !voidTags[t.tag]
		if isBlockOpen {
			if textStart < t.start {
				segments = append(segments, headingSegment{kind: "text", raw: html[textStart:t.start]})
			}
			closeStart, end, closeIdx := findBlockEnd(tokens, i, t.tag, len(html))
			inner := ""
			if end <= closeStart {
				inner = html[end:closeStart]
			}
			segments = append(segments, headingSegment{
				kind: "block", tag: t.tag, attrs: t.attrs,
				raw: html[t.start:end], inner: inner,
			})
			i = closeIdx + 1
			textStart = end
			continue
		}
		i++
	}
	if textStart < len(html) {
		segments = append(segments, headingSegment{kind: "text", raw: html[textStart:]})
	}
	return segments
}

func openTag(tag, attrs string) string {
	a := strings.TrimSpace(attrs)
	if a == "" {
		return "<" + tag + ">"
	}
	return "<" + tag + " " + a + ">"
}

func isWrapperTag(tag string) bool { return wrapperTags[tag] }

func innerText(html string) string {
	return strings.TrimSpace(reWhitespace.ReplaceAllString(decodeEntities(reHTMLTag.ReplaceAllString(html, " ")), " "))
}

func isHeadingCandidate(text string) bool {
	if utf8.RuneCountInString(text) < 4 || utf8.RuneCountInString(text) > 90 {
		return false
	}
	if strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?") || strings.HasSuffix(text, ":") {
		return false
	}
	first := []rune(text)[0]
	if !(unicode.IsUpper(first) || unicode.IsDigit(first)) {
		return false
	}
	if strings.Contains(text, ". ") {
		return false
	}
	return true
}

func transformSegments(segments []headingSegment) string {
	titles := map[string]bool{}
	for _, s := range segments {
		if s.kind == "block" && s.tag == "h1" {
			titles[innerText(s.inner)] = true
		}
	}
	firstContentSeen := false
	var out strings.Builder
	for _, s := range segments {
		if s.kind == "text" {
			out.WriteString(reMDLine.ReplaceAllStringFunc(s.raw, func(m string) string {
				sub := reMDLine.FindStringSubmatch(m)
				if sub == nil {
					return m
				}
				level := len(sub[2])
				return "<h" + itoa(level) + ">" + sub[3] + "</h" + itoa(level) + ">"
			}))
			continue
		}
		// H1 dilarang di isi artikel (judul hidup di field judul) → diturunkan.
		// Lewati s.raw (blok utuh), bukan s.inner — inner bisa kosong.
		if s.tag == "h1" {
			out.WriteString(demoteH1ToH2(s.raw))
			continue
		}
		if s.tag == "p" || s.tag == "div" {
			inner := strings.TrimSpace(s.inner)
			if inner == "" {
				out.WriteString(s.raw)
				continue
			}
			if md := reMDBLock.FindStringSubmatch(inner); md != nil {
				level := len(md[1])
				out.WriteString("<h" + itoa(level) + ">" + md[2] + "</h" + itoa(level) + ">")
				continue
			}
			if !firstContentSeen {
				firstContentSeen = true
				out.WriteString(s.raw)
				continue
			}
			text := innerText(inner)
			if isHeadingCandidate(text) && !reNestedBlok.MatchString(inner) && !titles[text] {
				out.WriteString(openTag("h2", s.attrs) + inner + "</h2>")
				continue
			}
			out.WriteString(s.raw)
			continue
		}
		out.WriteString(s.raw)
	}
	return out.String()
}

// ---------- embed (port stripDisallowedIframes lib/media/embed.ts) ----------

var allowedEmbedHosts = map[string]bool{
	"www.youtube.com": true, "youtube.com": true, "www.youtube-nocookie.com": true, "youtube-nocookie.com": true,
	"player.vimeo.com": true,
	"www.google.com":   true, "google.com": true, "maps.google.com": true,
}

var (
	reIframePair = regexp.MustCompile(`(?is)<iframe\b[^>]*>.*?</iframe>`)
	reIframeVoid = regexp.MustCompile(`(?is)<iframe\b[^>]*\/?>`)
	reIframeSrc  = regexp.MustCompile(`(?i)src\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

func iframeSrcOf(tag string) string {
	m := reIframeSrc.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

func isAllowedEmbedSrc(src string) bool {
	if src == "" {
		return false
	}
	lower := strings.ToLower(src)
	rest := ""
	if strings.HasPrefix(lower, "https://") {
		rest = src[len("https://"):]
	} else {
		return false
	}
	host := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		host = rest[:i]
	}
	host = strings.ToLower(host)
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if colon := strings.Index(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	return allowedEmbedHosts[host]
}

// stripDisallowedIframes: buang iframe dari host tak tepercaya (server guard).
func stripDisallowedIframes(html string) string {
	if html == "" {
		return html
	}
	out := reIframePair.ReplaceAllStringFunc(html, func(tag string) string {
		if isAllowedEmbedSrc(iframeSrcOf(tag)) {
			return tag
		}
		return ""
	})
	return reIframeVoid.ReplaceAllStringFunc(out, func(tag string) string {
		if isAllowedEmbedSrc(iframeSrcOf(tag)) {
			return tag
		}
		return ""
	})
}

// ---------- app url + upload urls ----------

// CanonicalAppBase: port canonicalAppBase(request?) — env dulu, lalu host
// publik request; host privat → domain produksi.
func (d *Deps) CanonicalAppBase(r *http.Request) string {
	envBase := ""
	if d.Cfg != nil {
		envBase = strings.TrimSpace(d.Cfg.PublicAppURL)
	}
	if envBase != "" {
		return strings.TrimRight(envBase, "/")
	}
	if r == nil {
		return ""
	}
	host := r.Host
	lower := strings.ToLower(host)
	if host == "" || isPrivateHost(lower) {
		return "https://app.tauthub.com"
	}
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	if xf := r.Header.Get("x-forwarded-proto"); xf != "" {
		scheme = strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	return scheme + "://" + host
}

func isPrivateHost(host string) bool {
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.") ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") {
		return true
	}
	if strings.HasPrefix(host, "172.") {
		// 172.16.0.0 – 172.31.255.255
		parts := strings.Split(host, ".")
		if len(parts) >= 2 {
			second := parts[1]
			if second >= "16" && second <= "31" && len(second) == 2 {
				return true
			}
		}
	}
	return false
}

// absolutizeURL: path relatif "/..." → origin + path; lainnya apa adanya.
func absolutizeURL(value string, origin string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return trimmed
	}
	if origin == "" {
		return trimmed
	}
	return strings.TrimRight(origin, "/") + trimmed
}

// ---------- standard check (port lib/standard/check.ts) ----------

type mediaStandard struct {
	MediaID           string
	MinWords          int
	MaxWords          int
	JournalisticStyle string
	HardSellingTol    string
	QuoteRequired     bool
	MinQuotes         int
	ImageRequired     bool
	CategoryRequired  bool
	RequiredInfo      []string
	ForbiddenContent  []string
}

type standardCheck struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type standardCheckResult struct {
	MediaID   string          `json:"media_id"`
	MediaName any             `json:"media_name"`
	Status    string          `json:"status"`
	Score     int             `json:"score"`
	Checks    []standardCheck `json:"checks"`
	Failed    []string        `json:"failed"`
	Warnings  []string        `json:"warnings"`
}

var promotionalTerms = []string{
	"diskon", "discount", "promo", "promosi", "gratis", "beli sekarang", "order sekarang",
	"dapatkan sekarang", "harga spesial", "terbatas", "stok terbatas", "klik di sini", "klik disini",
	"murah", "best price", "hubungi kami", "segera daftar", "bursa", "bundling", "cashback",
}

func countPromotional(text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, term := range promotionalTerms {
		from := 0
		for {
			idx := strings.Index(lower[from:], term)
			if idx < 0 {
				break
			}
			count++
			from += idx + len(term)
		}
	}
	return count
}

func countWords(text string) int {
	cleaned := reHTMLTag.ReplaceAllString(text, " ")
	cleaned = strings.Map(func(r rune) rune {
		switch r {
		case '#', '*', '_', '`', '~', '>', '-':
			return ' '
		}
		return r
	}, cleaned)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return 0
	}
	return len(strings.Fields(cleaned))
}

// countQuotes: jumlah pasangan tanda kutip (parity countQuotes).
func countQuotes(text string) int {
	n := 0
	for _, r := range text {
		switch r {
		case '"', '\u201c', '\u201d', '\u00ab', '\u00bb', '\u201e':
			n++
		}
	}
	return n / 2
}

var toleranceThresholds = map[string][2]int{
	"none":   {1, 1},
	"low":    {1, 4},
	"medium": {3, 7},
	"high":   {6, 11},
}

var reImageInText = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)|https?://\S+\.(png|jpe?g|webp|gif)`)

// runStandardCheck: port runStandardCheck — 7 pengecek deterministik berbobot.
func runStandardCheck(s mediaStandard, articleText string, mediaName any) standardCheckResult {
	checks := []standardCheck{}
	failed := []string{}
	warnings := []string{}
	text := articleText

	words := countWords(text)
	if s.MinWords > 0 && words < s.MinWords {
		checks = append(checks, standardCheck{"length", "Panjang artikel", "fail", itoa(words) + " kata — kurang dari minimum " + itoa(s.MinWords)})
		failed = append(failed, "Panjang artikel ("+itoa(words)+" kata < "+itoa(s.MinWords)+")")
	} else if s.MaxWords > 0 && words > s.MaxWords {
		checks = append(checks, standardCheck{"length", "Panjang artikel", "fail", itoa(words) + " kata — melebihi maksimum " + itoa(s.MaxWords)})
		failed = append(failed, "Panjang artikel ("+itoa(words)+" kata > "+itoa(s.MaxWords)+")")
	} else {
		checks = append(checks, standardCheck{"length", "Panjang artikel", "ok", itoa(words) + " kata (batas " + itoa(s.MinWords) + "-" + itoa(s.MaxWords) + ")"})
	}

	promo := countPromotional(text)
	thresholds, ok := toleranceThresholds[s.HardSellingTol]
	if !ok {
		thresholds = toleranceThresholds["low"]
	}
	if promo >= thresholds[1] {
		checks = append(checks, standardCheck{"hard_selling", "Konten promosi berlebihan", "fail", itoa(promo) + " istilah promosi ditemukan (toleransi " + s.HardSellingTol + ")"})
		failed = append(failed, "Konten promosi ("+itoa(promo)+" istilah)")
	} else if promo >= thresholds[0] {
		checks = append(checks, standardCheck{"hard_selling", "Konten promosi", "warn", itoa(promo) + " istilah promosi — kurangi agar sesuai gaya " + s.JournalisticStyle})
		warnings = append(warnings, "Konten promosi ("+itoa(promo)+" istilah)")
	} else {
		checks = append(checks, standardCheck{"hard_selling", "Konten promosi", "ok", itoa(promo) + " istilah promosi (toleransi " + s.HardSellingTol + ")"})
	}

	quotes := countQuotes(text)
	quoteThreshold := s.MinQuotes
	if s.QuoteRequired {
		quoteThreshold = s.MinQuotes
		if quoteThreshold < 1 {
			quoteThreshold = 1
		}
	}
	if quoteThreshold > 0 && quotes < quoteThreshold {
		checks = append(checks, standardCheck{"quotes", "Kutipan narasumber", "fail", "Ditemukan " + itoa(quotes) + " kutipan — butuh minimal " + itoa(quoteThreshold)})
		failed = append(failed, "Kutipan ("+itoa(quotes)+" < "+itoa(quoteThreshold)+")")
	} else if quoteThreshold > 0 {
		checks = append(checks, standardCheck{"quotes", "Kutipan narasumber", "ok", itoa(quotes) + " kutipan"})
	} else if s.QuoteRequired {
		checks = append(checks, standardCheck{"quotes", "Kutipan narasumber", "fail", "Belum ada kutipan"})
		failed = append(failed, "Kutipan narasumber")
	} else {
		checks = append(checks, standardCheck{"quotes", "Kutipan narasumber", "warn", "Tidak wajib, tapi memperkuat artikel"})
		warnings = append(warnings, "Kutipan narasumber belum ada")
	}

	lowerText := strings.ToLower(text)
	forbiddenHits := []string{}
	for _, term := range s.ForbiddenContent {
		if term != "" && strings.Contains(lowerText, strings.ToLower(term)) {
			forbiddenHits = append(forbiddenHits, term)
		}
	}
	if len(forbiddenHits) > 0 {
		checks = append(checks, standardCheck{"forbidden", "Konten terlarang", "fail", "Ditemukan: " + strings.Join(capStrings(forbiddenHits, 5), ", ")})
		failed = append(failed, "Konten terlarang ("+strings.Join(forbiddenHits, ", ")+")")
	} else {
		checks = append(checks, standardCheck{"forbidden", "Konten terlarang", "ok", "Bersih"})
	}

	if len(s.RequiredInfo) > 0 {
		missing := []string{}
		for _, term := range s.RequiredInfo {
			if term != "" && !strings.Contains(lowerText, strings.ToLower(term)) {
				missing = append(missing, term)
			}
		}
		if len(missing) == 0 {
			checks = append(checks, standardCheck{"required_info", "Informasi wajib", "ok", "Semua tercakup"})
		} else {
			checks = append(checks, standardCheck{"required_info", "Informasi wajib", "warn", "Belum ada: " + strings.Join(capStrings(missing, 5), ", ")})
			warnings = append(warnings, "Informasi wajib belum ada ("+strings.Join(missing, ", ")+")")
		}
	} else {
		checks = append(checks, standardCheck{"required_info", "Informasi wajib", "ok", "Tidak ada ketentuan"})
	}

	hasImage := reImageInText.MatchString(text)
	if s.ImageRequired && !hasImage {
		checks = append(checks, standardCheck{"image", "Gambar pendukung", "warn", "Belum ada gambar — media memintanya"})
		warnings = append(warnings, "Gambar pendukung belum ada")
	} else if s.ImageRequired {
		checks = append(checks, standardCheck{"image", "Gambar pendukung", "ok", "Ada"})
	} else {
		checks = append(checks, standardCheck{"image", "Gambar pendukung", "ok", "Tidak wajib"})
	}

	if s.CategoryRequired {
		checks = append(checks, standardCheck{"category", "Kategori media", "warn", "Pastikan kategori dipilih saat mengirim ke media"})
		warnings = append(warnings, "Kategori media perlu dipilih")
	} else {
		checks = append(checks, standardCheck{"category", "Kategori media", "ok", "Tidak wajib"})
	}

	weights := map[string]int{"length": 20, "hard_selling": 20, "quotes": 15, "forbidden": 25, "required_info": 15, "image": 5, "category": 5}
	achieved := 0
	for _, c := range checks {
		if c.Status == "ok" {
			achieved += weights[c.Key]
		}
	}
	status := "pass"
	if len(failed) > 0 {
		status = "fail"
	} else if len(warnings) > 0 {
		status = "warning"
	}
	return standardCheckResult{
		MediaID: s.MediaID, MediaName: mediaName, Status: status, Score: achieved,
		Checks: checks, Failed: failed, Warnings: warnings,
	}
}

func capStrings(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return list[:n]
}
