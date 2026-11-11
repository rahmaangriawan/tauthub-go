package brandflow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// link_in_article.go: pemeriksaan tautan DI DALAM artikel.
//
// Pengecekan biasa hanya meminta URL artikel dan mencatat status HTTP-nya
// (live/gone/unreachable). Kalau media mencabut tautan yang tertanam di badan
// artikel, URL-nya tetap 200 sehingga tidak ada satu pun kolom yang menangkap
// kejadian itu. Berkas ini menutup celah tersebut: halaman artikel dibaca,
// tautan di dalamnya diekstrak, lalu dibandingkan dengan tautan yang
// seharusnya tertanam menurut HTML dokumen yang kita simpan.
//
// Bersikap konservatif: kalau halaman tidak bisa dibaca (WAF media memblokir
// server kita) atau dokumen tidak punya tautan acuan, hasilnya 'unknown',
// bukan 'removed'. Menuduh tautan dicabut tanpa bukti lebih buruk daripada
// mengaku belum bisa memeriksa.

// inArticleMaxHTML: batas isi halaman yang dibaca untuk mencari tautan.
const inArticleMaxHTML = 1 << 20 // 1 MiB

// pageLink: satu tautan pada halaman (href absolut + teks anchor-nya + nilai
// atribut rel apa adanya, huruf kecil, kosong bila tidak ada).
type pageLink struct {
	URL    string
	Anchor string
	Rel    string
}

// inArticleResult: hasil pemeriksaan tautan dalam artikel.
type inArticleResult struct {
	State     string // 'present' | 'removed' | 'unknown' | "" (NULL: belum dinilai)
	Anchor    string
	Found     int
	ErrMsg    string
	CheckedAt *time.Time
}

var (
	// Menangkap <a ...atribut...>teks</a>; href dan rel dibaca terpisah dari
	// atribut supaya urutan atributnya tidak berpengaruh.
	linkTagRe  = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	hrefAttrRe = regexp.MustCompile(`(?is)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	relAttrRe  = regexp.MustCompile(`(?is)\brel\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	tagRe      = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe    = regexp.MustCompile(`\s+`)

	// <meta name="robots|googlebot" content="..."> dalam urutan atribut apa pun.
	metaTagRe     = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	metaNameRe    = regexp.MustCompile(`(?is)\bname\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	metaContentRe = regexp.MustCompile(`(?is)\bcontent\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

// attrValue: nilai atribut dari dua grup tangkap (kutip ganda / tunggal).
func attrValue(m []string) string {
	if len(m) < 3 {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// extractPageLinks: tautan absolut http(s) pada HTML beserta anchor yang sudah
// dibersihkan dari tag dan entitas, dan atribut rel-nya.
func extractPageLinks(pageHTML string) []pageLink {
	if pageHTML == "" {
		return nil
	}
	out := make([]pageLink, 0, 32)
	for _, m := range linkTagRe.FindAllStringSubmatch(pageHTML, -1) {
		hm := hrefAttrRe.FindStringSubmatch(m[1])
		if hm == nil {
			continue
		}
		href := strings.TrimSpace(html.UnescapeString(attrValue(hm)))
		if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
			continue
		}
		rel := ""
		if rm := relAttrRe.FindStringSubmatch(m[1]); rm != nil {
			rel = strings.ToLower(strings.TrimSpace(spaceRe.ReplaceAllString(html.UnescapeString(attrValue(rm)), " ")))
		}
		anchor := strings.TrimSpace(spaceRe.ReplaceAllString(html.UnescapeString(tagRe.ReplaceAllString(m[2], " ")), " "))
		out = append(out, pageLink{URL: href, Anchor: anchor, Rel: rel})
	}
	return out
}

// pageRobotsMeta: gabungan isi <meta name="robots"> dan <meta name="googlebot">
// (huruf kecil, dipisah koma). Kosong bila halaman tidak punya meta robots.
func pageRobotsMeta(pageHTML string) string {
	if pageHTML == "" {
		return ""
	}
	parts := []string{}
	for _, tag := range metaTagRe.FindAllString(pageHTML, -1) {
		nm := metaNameRe.FindStringSubmatch(tag)
		if nm == nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(attrValue(nm)))
		if name != "robots" && name != "googlebot" {
			continue
		}
		cm := metaContentRe.FindStringSubmatch(tag)
		if cm == nil {
			continue
		}
		content := strings.ToLower(strings.TrimSpace(html.UnescapeString(attrValue(cm))))
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, ", ")
}

// xRobotsGoogleDirectives: isi header X-Robots-Tag yang berlaku untuk Google.
// Nilai berawalan nama bot lain ("bingbot: noindex") diabaikan; "googlebot:"
// dan nilai tanpa awalan dipakai. "unavailable_after: <tanggal>" adalah
// direktif, bukan nama bot.
func xRobotsGoogleDirectives(values []string) string {
	parts := []string{}
	for _, raw := range values {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if i := strings.IndexByte(v, ':'); i > 0 {
			prefix := strings.TrimSpace(v[:i])
			if prefix != "unavailable_after" && !strings.ContainsAny(prefix, " ,") {
				if prefix != "googlebot" && prefix != "google" {
					continue
				}
				v = strings.TrimSpace(v[i+1:])
			}
		}
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}

// robotsHasNoindex: direktif "noindex" atau "none" (= noindex + nofollow).
func robotsHasNoindex(directives string) bool {
	if directives == "" {
		return false
	}
	for _, tok := range strings.FieldsFunc(directives, func(r rune) bool { return r == ',' || r == ' ' }) {
		if tok == "noindex" || tok == "none" {
			return true
		}
	}
	return false
}

// relTokens: token atribut rel (huruf kecil).
func relTokens(rel string) []string {
	return strings.Fields(strings.ToLower(rel))
}

// relIsDofollow: tautan tidak membawa nofollow, sponsored, maupun ugc.
func relIsDofollow(rel string) bool {
	for _, t := range relTokens(rel) {
		if t == "nofollow" || t == "sponsored" || t == "ugc" {
			return false
		}
	}
	return true
}

// linkSignals: sinyal tambahan dari halaman artikel di luar status HTTP dan
// ada/tidaknya tautan: atribut rel tautan utama dan direktif noindex.
type linkSignals struct {
	Href         string
	Anchor       string
	Rel          string
	RelState     string // 'ok' | 'changed' | 'unknown' | "" (belum dinilai)
	Robots       string // gabungan meta robots dan X-Robots-Tag
	NoindexState string // 'ok' | 'noindex' | 'unknown' | "" (belum dinilai)
}

// evaluateLinkSignals: nilai rel tautan utama (tautan luar pertama di dokumen)
// terhadap rel yang dijanjikan media, dan cari direktif noindex.
//
// Konservatif seperti pemeriksaan tautan: halaman tidak terbaca atau tautan
// utama tidak ditemukan berarti 'unknown', bukan pelanggaran. Janji rel hanya
// ditegakkan untuk media 'dofollow'; media nofollow/sponsored tidak menjanjikan
// apa pun soal rel, jadi relnya hanya dicatat.
func evaluateLinkSignals(expected []pageLink, liveHTML string, xRobots []string, promisedRel string) linkSignals {
	sig := linkSignals{RelState: "unknown", NoindexState: "unknown"}

	pageReadable := strings.TrimSpace(liveHTML) != ""
	headerRobots := xRobotsGoogleDirectives(xRobots)
	metaRobots := pageRobotsMeta(liveHTML)
	sig.Robots = strings.Trim(strings.Join(nonEmpty(metaRobots, headerRobots), ", "), ", ")

	switch {
	case robotsHasNoindex(metaRobots) || robotsHasNoindex(headerRobots):
		sig.NoindexState = "noindex"
	case pageReadable:
		sig.NoindexState = "ok"
	}

	if len(expected) == 0 || !pageReadable {
		return sig
	}
	primary := expected[0]
	sig.Anchor = primary.Anchor
	found := extractPageLinks(liveHTML)
	var match *pageLink
	for i := range found {
		if normLinkURL(found[i].URL) == normLinkURL(primary.URL) || normLinkPath(found[i].URL) == normLinkPath(primary.URL) {
			match = &found[i]
			break
		}
	}
	if match == nil {
		return sig
	}
	sig.Href = match.URL
	sig.Rel = match.Rel
	if promisedRel == "dofollow" && !relIsDofollow(match.Rel) {
		sig.RelState = "changed"
	} else {
		sig.RelState = "ok"
	}
	return sig
}

func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// checkLinkSignals: baca tautan acuan dokumen lalu nilai sinyal halaman.
func (d *Deps) checkLinkSignals(ctx context.Context, documentID, liveHTML string, xRobots []string, promisedRel string) linkSignals {
	var expected []pageLink
	if strings.TrimSpace(documentID) != "" {
		expected = expectedLinksFromDocument(d.documentHTML(ctx, documentID))
	}
	return evaluateLinkSignals(expected, liveHTML, xRobots, promisedRel)
}

// normLinkURL: bentuk pembanding. Fragmen dibuang dan garis miring di ujung
// dirapikan supaya perbedaan kosmetik tidak dianggap tautan hilang.
func normLinkURL(raw string) string {
	u := strings.TrimSpace(raw)
	if i := strings.IndexByte(u, '#'); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimRight(u, "/")
	return strings.ToLower(u)
}

// normLinkPath: pembanding cadangan (host+path+query tanpa skema), dipakai saat
// media menulis ulang skema atau menambah parameter.
func normLinkPath(raw string) string {
	u := strings.TrimSpace(raw)
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexByte(u, '#'); i >= 0 {
		u = u[:i]
	}
	return strings.ToLower(strings.TrimRight(u, "/"))
}

// checkInArticleLinks: bandingkan tautan acuan dari dokumen dengan tautan yang
// benar-benar ada di halaman artikel.
func (d *Deps) checkInArticleLinks(ctx context.Context, documentID, liveHTML string) inArticleResult {
	now := time.Now()
	res := inArticleResult{CheckedAt: &now}

	if strings.TrimSpace(documentID) == "" {
		res.State = "unknown"
		res.ErrMsg = "dokumen artikel tidak tersedia, tidak ada tautan acuan"
		return res
	}
	expected := expectedLinksFromDocument(d.documentHTML(ctx, documentID))
	if len(expected) == 0 {
		res.State = "unknown"
		res.ErrMsg = "dokumen artikel tidak memuat tautan luar"
		return res
	}
	res.Anchor = expected[0].Anchor

	if strings.TrimSpace(liveHTML) == "" {
		res.State = "unknown"
		res.ErrMsg = "halaman artikel tidak bisa dibaca (diblokir atau gagal dimuat)"
		return res
	}

	found := extractPageLinks(liveHTML)
	state, anchor, foundCount := compareArticleLinks(expected, found)
	res.State = state
	res.Anchor = anchor
	res.Found = foundCount
	return res
}

// compareArticleLinks: inti penilaian tautan dalam artikel. Dipisah dari I/O
// (pengambilan halaman dan pembacaan dokumen) supaya bisa diuji langsung.
//
// Toleran terhadap perbedaan kosmetik: fragmen dan garis miring di ujung
// diabaikan, dan pembanding kedua memakai host+path sehingga tautan yang
// ditulis ulang skemanya oleh media tidak dianggap hilang.
func compareArticleLinks(expected, found []pageLink) (string, string, int) {
	if len(expected) == 0 {
		return "unknown", "", len(found)
	}
	firstAnchor := expected[0].Anchor
	if len(found) == 0 {
		return "unknown", firstAnchor, 0
	}
	byFull := make(map[string]bool, len(found))
	byPath := make(map[string]bool, len(found))
	for _, l := range found {
		byFull[normLinkURL(l.URL)] = true
		byPath[normLinkPath(l.URL)] = true
	}
	for _, e := range expected {
		if byFull[normLinkURL(e.URL)] || byPath[normLinkPath(e.URL)] {
			continue
		}
		// Anchor yang hilang lebih berguna ditampilkan daripada tautan pertama.
		return "removed", e.Anchor, len(found)
	}
	return "present", firstAnchor, len(found)
}

// expectedLinksFromDocument: tautan luar yang tertanam di HTML dokumen.
func expectedLinksFromDocument(docHTML string) []pageLink {
	links := extractPageLinks(docHTML)
	if len(links) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(links))
	out := make([]pageLink, 0, len(links))
	for _, l := range links {
		key := normLinkURL(l.URL)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, l)
	}
	return out
}

// documentHTML: HTML dokumen artikel (kosong bila belum ada / tidak tersimpan).
func (d *Deps) documentHTML(ctx context.Context, documentID string) string {
	row, err := d.queryRow(ctx, "SELECT content_html FROM content_documents WHERE id = $1 LIMIT 1", documentID)
	if err != nil || row == nil {
		return ""
	}
	return row.strOr("content_html", "")
}

// crawlhubPageHTML: minta Crawlhub mengambilkan halaman lewat POST /v1/crawl/test.
// Bentuk responsnya belum terkonfirmasi di repo ini, jadi beberapa kunci yang
// mungkin dipakai diterima longgar; kalau tidak ada yang cocok, dianggap gagal
// supaya pemanggil jatuh ke pengambilan langsung alih-alih menebak isi.
func (d *Deps) crawlhubPageHTML(ctx context.Context, url string) (string, error) {
	cfg := d.bsLoadCrawlhubConfig(ctx)
	if !bsCrawlhubConfigured(cfg) {
		return "", errors.New("Crawlhub belum dikonfigurasi")
	}
	res, err := d.bsCrawlhubPost(ctx, cfg, "/v1/crawl/test", map[string]any{"url": url})
	if err != nil {
		return "", err
	}
	if s := firstHTMLField(res); s != "" {
		return s, nil
	}
	if nested, ok := res["data"].(map[string]any); ok {
		if s := firstHTMLField(nested); s != "" {
			return s, nil
		}
	}
	return "", errors.New("respons Crawlhub tidak memuat isi halaman")
}

func firstHTMLField(m map[string]any) string {
	for _, k := range []string{"html", "html_content", "content", "body", "raw", "rendered_html"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// bsCrawlhubPost: POST bertanda tangan. Tanda tangannya sama dengan GET, yaitu
// HMAC-SHA256 atas "<timestamp>.<rawBody>" (lihat igCrawlhubAPIInfoJSON).
func (d *Deps) bsCrawlhubPost(ctx context.Context, cfg bsCrawlhubConfig, path string, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(cfg.HMACSecret))
	mac.Write([]byte(timestamp + "." + string(body)))
	digest := hex.EncodeToString(mac.Sum(nil))

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.BaseURL+path, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-crawlhub-timestamp", timestamp)
	req.Header.Set("x-crawlhub-signature", "sha256="+digest)

	resp, err := security.SafeClient(30 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateString(string(text), 300))
	}
	var parsed map[string]any
	if err := json.Unmarshal(text, &parsed); err != nil {
		return nil, errors.New("respons Crawlhub bukan JSON")
	}
	return parsed, nil
}
