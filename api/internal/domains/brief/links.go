package brief

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Tautan di pesan user: begitu user menempel URL ke obrolan, halaman itu
// dibaca lebih dulu supaya AI sudah memegang isinya pada giliran yang sama —
// bukan sekadar melihat alamatnya sebagai teks.

const (
	maxLinksPerMessage = 2
	linkFetchTimeout   = 15 * time.Second
	maxLinkRunes       = 100000
)

var reURLInText = regexp.MustCompile(`https?://[^\s<>"'\])}]+`)

// extractURLs: URL http(s) di dalam teks pesan, bersih dari tanda baca
// penutup yang biasanya ikut terbawa (titik, koma, kutip), maksimal dua.
func extractURLs(message string) []string {
	matches := reURLInText.FindAllString(message, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, m := range matches {
		trimmed := strings.TrimRight(m, ".,;:!?…")
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
		if len(out) >= maxLinksPerMessage {
			break
		}
	}
	return out
}

// absorbMessageLinks: pastikan setiap tautan dari pesan user sudah dibaca
// sistem (tersimpan di profiler_urls sebagai sumber brief). Dipanggil sebelum
// konteks giliran dirakit supaya isi halaman ikut jadi bahan prompt giliran
// ini. onStatus (opsional) melaporkan progres ke UI selagi AI "berpikir".
func (d *Deps) absorbMessageLinks(ctx context.Context, profileID, userID, message string, onStatus func(string)) {
	for _, raw := range extractURLs(message) {
		if ctx.Err() != nil {
			return
		}
		id, status, known := d.findLink(ctx, profileID, raw)
		if known && status == "ready" {
			continue // sudah pernah dibaca — sumbernya sudah ikut di konteks
		}
		if !known {
			id = uuid.NewString()
			if _, err := d.Pool.Exec(ctx,
				`INSERT INTO profiler_urls (id, profile_id, url, created_by) VALUES ($1, $2, $3, $4)`,
				id, profileID, raw, nullIfEmpty(userID)); err != nil {
				continue
			}
		}
		if onStatus != nil {
			onStatus("Membaca tautan " + linkHost(raw) + "…")
		}
		d.fetchLink(ctx, id, raw)
	}
}

func (d *Deps) findLink(ctx context.Context, profileID, raw string) (id, status string, found bool) {
	err := d.Pool.QueryRow(ctx,
		`SELECT id, fetch_status FROM profiler_urls
		 WHERE profile_id = $1 AND url = $2 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`, profileID, raw).Scan(&id, &status)
	if err != nil {
		return "", "", false
	}
	return id, status, true
}

// fetchLink: ambil halaman dengan aman (anti-SSRF lewat security.Get),
// bersihkan jadi teks, tandai 'ready' hanya bila isi benar-benar terbaca.
// Paritas dengan profiler.syncURL — dipisah karena beda paket.
func (d *Deps) fetchLink(ctx context.Context, id, raw string) {
	_, _ = d.Pool.Exec(ctx, "UPDATE profiler_urls SET fetch_status = 'fetching', fetch_error = NULL WHERE id = $1", id)
	resp, err := security.Get(ctx, raw, map[string]string{"User-Agent": "TauthubBot/1.0"}, linkFetchTimeout)
	if err != nil {
		d.markLinkFailed(ctx, id, "Gagal mengambil halaman: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		d.markLinkFailed(ctx, id, fmt.Sprintf("Halaman mengembalikan status %d.", resp.StatusCode))
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		d.markLinkFailed(ctx, id, "Isi halaman tidak dapat dibaca.")
		return
	}
	htmlBody := string(body)
	text := linkHTMLToText(htmlBody)
	if len([]rune(text)) < 80 {
		d.markLinkFailed(ctx, id, "Isi halaman terlalu pendek untuk dijadikan referensi (mungkin butuh login atau memblokir crawler).")
		return
	}
	title := linkFirstTitle(htmlBody)
	_, _ = d.Pool.Exec(ctx, `
		UPDATE profiler_urls SET fetch_status = 'ready', extracted_text = $1, fetch_error = NULL,
		       title = COALESCE(NULLIF(title, ''), NULLIF($2, '')), last_synced_at = NOW() WHERE id = $3`,
		truncateRunes(text, maxLinkRunes), title, id)
}

func (d *Deps) markLinkFailed(ctx context.Context, id, msg string) {
	_, _ = d.Pool.Exec(ctx, "UPDATE profiler_urls SET fetch_status = 'failed', fetch_error = $1 WHERE id = $2", truncate(msg, 500), id)
}

func linkHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

var (
	reLinkScript   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reLinkStyle    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reLinkNoscript = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	reLinkSvg      = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
	reLinkTags     = regexp.MustCompile(`(?s)<[^>]+>`)
	reLinkSpaces   = regexp.MustCompile(`[ \t\r\f\v]+`)
	reLinkBlankRun = regexp.MustCompile(`\n{3,}`)
	reLinkTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

// linkHTMLToText: buang script/style/noscript/svg/tag lalu rapikan spasi
// (paritas dengan profiler.htmlToText).
func linkHTMLToText(html string) string {
	s := reLinkScript.ReplaceAllString(html, " ")
	s = reLinkStyle.ReplaceAllString(s, " ")
	s = reLinkNoscript.ReplaceAllString(s, " ")
	s = reLinkSvg.ReplaceAllString(s, " ")
	s = reLinkTags.ReplaceAllString(s, "\n")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'").Replace(s)
	s = reLinkSpaces.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			kept = append(kept, t)
		}
	}
	return strings.TrimSpace(reLinkBlankRun.ReplaceAllString(strings.Join(kept, "\n"), "\n\n"))
}

func linkFirstTitle(html string) string {
	m := reLinkTitle.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return truncateRunes(strings.TrimSpace(reLinkTags.ReplaceAllString(m[1], " ")), 255)
}

// truncateRunes: potong berdasarkan rune supaya karakter UTF-8 tidak terbelah.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if max <= 0 || len(r) <= max {
		return s
	}
	return string(r[:max])
}
