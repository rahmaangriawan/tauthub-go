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

// pageLink: satu tautan pada halaman (href absolut + teks anchor-nya).
type pageLink struct {
	URL    string
	Anchor string
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
	// Menangkap <a ... href="...">teks</a>, termasuk href bertanda kutip tunggal.
	linkTagRe = regexp.MustCompile(`(?is)<a\b[^>]*?href\s*=\s*(?:"([^"]*)"|'([^']*)')[^>]*>(.*?)</a>`)
	tagRe     = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe   = regexp.MustCompile(`\s+`)
)

// extractPageLinks: tautan absolut http(s) pada HTML beserta anchor yang sudah
// dibersihkan dari tag dan entitas.
func extractPageLinks(pageHTML string) []pageLink {
	if pageHTML == "" {
		return nil
	}
	out := make([]pageLink, 0, 32)
	for _, m := range linkTagRe.FindAllStringSubmatch(pageHTML, -1) {
		href := m[1]
		if href == "" {
			href = m[2]
		}
		href = strings.TrimSpace(html.UnescapeString(href))
		if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
			continue
		}
		anchor := strings.TrimSpace(spaceRe.ReplaceAllString(html.UnescapeString(tagRe.ReplaceAllString(m[3], " ")), " "))
		out = append(out, pageLink{URL: href, Anchor: anchor})
	}
	return out
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
