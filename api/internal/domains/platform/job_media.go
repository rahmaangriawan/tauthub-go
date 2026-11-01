// job_media.go: processor job media di sisi platform — empat kind yang di app
// lama didispatch dari lib/jobs/queue.ts:
//
//   - media_seo_refresh  → checkAndStoreSeoMetrics (lib/media/seo-metrics.ts)
//   - media_ai_summary   → processMediaAiSummary  (lib/media/ai-summary.ts)
//   - crawl_target_sync  → syncCrawlTarget        (lib/crawlhub/sync.ts)
//   - domain_verify      → verifyDomainOwnership  (lib/media/domain-verification.ts)
//
// Provider SEO/AI/Crawlhub TIDAK ditulis ulang di sini: file ini memakai ulang
// helper yang sudah ada (media_support.go, media_summaries.go,
// misc_integrations.go, domain_diagnostics.go) supaya kontrak kolom dan pesan
// error tetap satu sumber. Helper privat ber-prefix `mj` (media job).
package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// ---------- kind: media_seo_refresh ----------

// MediaSeoRefreshJob: port cabang `media_seo_refresh` di queue.ts — ambil
// website_url media dulu, lalu panggil checkAndStoreSeoMetrics dengan aktor
// "scheduler". Media tanpa website dilewati tanpa error (job selesai).
func (d *Deps) MediaSeoRefreshJob(ctx context.Context, mediaID string) error {
	row, err := d.queryRow(ctx,
		"SELECT website_url FROM media WHERE id = $1 AND website_url IS NOT NULL AND website_url <> '' LIMIT 1", mediaID)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	// Nilai diteruskan apa adanya (parity `if (media?.website_url)`) — validasi
	// domain tetap milik mpCheckAndStoreSeoMetrics.
	_, err = d.mpCheckAndStoreSeoMetrics(ctx, mediaID, row.strOr("website_url", ""), "scheduler")
	return err
}

// ---------- kind: media_geo_refresh ----------

// MediaGeoRefreshJob: segarkan HANYA sinyal GEO/AI (AI Trust + status kutipan)
// untuk satu media. Dipisah dari media_seo_refresh supaya tombol "Cek GEO"
// tidak membakar kuota penyedia DA/PA berbayar.
func (d *Deps) MediaGeoRefreshJob(ctx context.Context, mediaID string) error {
	row, err := d.queryRow(ctx,
		"SELECT website_url FROM media WHERE id = $1 AND website_url IS NOT NULL AND website_url <> '' LIMIT 1", mediaID)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	_, err = d.mpCheckAndStoreGeoSignals(ctx, mediaID, row.strOr("website_url", ""), "scheduler")
	return err
}

// ---------- kind: media_similarweb_refresh ----------

// MediaSimilarWebJob: job antrean pemeriksaan trafik SimilarWeb satu media
// (dipicu sweep scheduler atau tombol "Cek Trafik Semua"). force=false karena
// gerak cooldown sudah dinilai di mpCheckSimilarWebInsights: media yang
// snapshottnya masih segar hanya membaca snapshot, sementara media yang sudah
// basi memanggil provider sekali. Media tanpa website_url atau yang sudah
// non-aktif selesai tanpa error — tidak ada yang perlu di-retry.
func (d *Deps) MediaSimilarWebJob(ctx context.Context, mediaID string) error {
	row, err := d.queryRow(ctx, "SELECT website_url, status FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	nrow := ptRow(ptNormalizeMap(row))
	if strings.TrimSpace(nrow.strOr("website_url", "")) == "" || nrow.strOr("status", "") != "active" {
		return nil
	}
	_, sErr := d.mpCheckSimilarWebInsights(ctx, mediaID, nrow.anyOrNil("website_url"), "scheduler", false)
	// JANGAN `return sErr` langsung: sErr bertipe *mpSimilarWebError, sehingga
	// nil yang dikembalikan sebagai error menjadi interface non-nil. Runner
	// menganggapnya gagal lalu memanggil Error() pada receiver nil → panic
	// nil-pointer dan proses scheduler/worker ikut mati. Konversi eksplisit
	// ini memastikan sukses benar-benar nil.
	if sErr != nil {
		return sErr
	}
	return nil
}

// ---------- kind: media_ai_summary ----------

// MediaAISummaryJob: port processMediaAiSummary — idempoten (lewati media yang
// sudah punya kesimpulan), tarik homepage + halaman about, generate tier
// "economy", simpan ke media.ai_summary, dan catat tiap kejadian ke
// media_ai_summary_events. Kegagalan dicatat sebagai event "failed" lalu
// dilempar supaya runner me-retry dengan backoff.
func (d *Deps) MediaAISummaryJob(ctx context.Context, mediaID string) error {
	media, err := d.queryRow(ctx,
		"SELECT id, name, category, description, notes, website_url, ai_summary FROM media WHERE id = $1::text LIMIT 1", mediaID)
	if err != nil {
		return err
	}
	if media == nil {
		return nil // media sudah dihapus → job selesai tanpa efek
	}
	if summary, _ := media.str("ai_summary"); summary != "" {
		return nil // idempoten: kesimpulan sudah ada
	}

	name := media.strOr("name", "")
	websiteURL := strings.TrimSpace(media.strOr("website_url", ""))
	mitraContext := mjMitraContext(media)

	crawled := mjCrawlResult{Pages: []mjCrawledPage{}}
	runErr := func() error {
		if websiteURL != "" {
			crawled = mjCrawlWebsiteText(ctx, websiteURL)
		}
		runtime, rErr := ai.LoadAIRuntime(ctx, d.Pool)
		if rErr != nil {
			return rErr
		}
		if !ai.IsAIConfigured(runtime) {
			// Aturan: provider belum dikonfigurasi → error jelas, bukan panic.
			return errors.New("AI belum dikonfigurasi")
		}
		result, gErr := ai.GenerateChat(ctx, d.Pool, ai.GenerateChatOptions{
			Prompt:   mjSummaryPrompt(mitraContext, crawled.Text),
			Tier:     "economy",
			TaskType: "media_website_summary",
			Runtime:  runtime,
		})
		if gErr != nil {
			return gErr
		}
		if result.Mocked {
			return errors.New("AI belum dikonfigurasi")
		}
		summary := mjTrimQuotes(result.Content)
		if utf8.RuneCountInString(summary) < mjMinSummaryChars {
			return errors.New("Hasil AI terlalu pendek untuk dipakai")
		}
		if err := d.exec(ctx,
			"UPDATE media SET ai_summary = $1, ai_summary_generated_at = NOW() WHERE id = $2::text",
			summary, mediaID); err != nil {
			return err
		}
		// Satu domain = satu profil: sebarkan ke media lain yang menunjuk
		// website sama dan belum punya kesimpulan (mitra kadang daftar dua kali).
		if domain := mjNormalizedDomain(websiteURL); domain != "" {
			if err := d.exec(ctx, `
				UPDATE media SET ai_summary = $1, ai_summary_generated_at = NOW()
				WHERE id <> $2::text AND ai_summary IS NULL
				  AND RTRIM(REGEXP_REPLACE(LOWER(TRIM(website_url)), '^(https?://)?(www\.)?', ''), '/') = $3`,
				summary, mediaID, domain); err != nil {
				return err
			}
		}
		return nil
	}()

	if runErr != nil {
		d.mpRecordSummaryEvent(ctx, mediaID, name, websiteURL, "failed", nil, ptTruncate(runErr.Error(), 1000))
		return runErr
	}
	d.mpRecordSummaryEvent(ctx, mediaID, name, websiteURL, "success", crawled.Pages, "")
	// Crawl VPS gagal total (semua halaman ditolak WAF / DNS) tapi ringkasan
	// tetap tersimpan dari data mitra → minta worker lokal menarik ulang
	// halamannya supaya kesimpulan berikutnya lebih kaya. Tanpa ini media
	// ber-WAF agresif selamanya menyimpan ringkasan tipis.
	if crawled.Text == "" && lcCrawlFullyBlocked(crawled.Pages) {
		d.lcRequestLocalFetch(ctx, mediaID, name, websiteURL)
	}
	return nil
}

// mjMitraContext: konteks dari data mitra untuk prompt (baris kosong dibuang).
func mjMitraContext(row ptRow) string {
	parts := []string{}
	if v := row.strOr("name", ""); v != "" {
		parts = append(parts, "Nama media: "+v)
	}
	if v := row.strOr("category", ""); v != "" {
		parts = append(parts, "Kategori: "+v)
	}
	if v := row.strOr("description", ""); v != "" {
		parts = append(parts, "Deskripsi dari mitra: "+v)
	}
	if v := row.strOr("notes", ""); v != "" {
		parts = append(parts, "Catatan: "+v)
	}
	return strings.Join(parts, "\n")
}

// mjSummaryPrompt: parity buildSummaryPrompt (teks prompt apa adanya).
func mjSummaryPrompt(mitraContext, crawled string) string {
	contextText := mitraContext
	if contextText == "" {
		contextText = "(tidak ada)"
	}
	crawledText := "(tidak tersedia)"
	if crawled != "" {
		crawledText = "<crawled>\n" + crawled + "\n</crawled>"
	}
	return strings.Join([]string{
		"Anda meringkas profil sebuah website media untuk direktori Tauthub.",
		"",
		"Data dari mitra:",
		contextText,
		"",
		"Konten yang ditarik dari website media (boleh kosong):",
		crawledText,
		"",
		"Tulis kesimpulan 3 sampai 6 kalimat dalam bahasa Indonesia yang mendeskripsikan:",
		"karakter media, audiens utama, topik atau rubrik yang dibahas, dan gaya penyajian.",
		"Gunakan HANYA informasi dari data di atas. Jangan mengarang angka, nama, atau klaim",
		"yang tidak ada di sumber. Jika data sedikit, tulis ringkas dan netral tanpa spikulasi.",
		"Balas HANYA teks kesimpulannya, tanpa judul, tanpa daftar poin, tanpa penjelasan lain.",
	}, "\n")
}

// mjTrimQuotes: parity `content.trim().replace(/^["']+|["']+$/g, "").trim()`.
func mjTrimQuotes(s string) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimLeft(strings.TrimSpace(s), "\"'"), "\"'"))
}

// mjNormalizedDomain: parity normalisasi domain di processMediaAiSummary —
// lowercase, buang skema, buang www, buang slash akhir.
func mjNormalizedDomain(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	v = mjSchemePrefixRe.ReplaceAllString(v, "")
	v = mjWWWPrefixRe.ReplaceAllString(v, "")
	return strings.TrimRight(v, "/")
}

// ---------- crawler teks website (port crawlWebsiteText) ----------

const (
	mjFetchTimeout    = 15 * time.Second
	mjMaxSuccessPage  = 2
	mjMaxHTMLChars    = 200_000
	mjMaxSourceChars  = 6_000
	mjMinSummaryChars = 40
	mjMaxAboutLinks   = 3
	mjMaxAttempts     = 10
)

// UA bergaya browser + header lengkap: sebagian CDN (Cloudflare dsb.) menantang
// permintaan ber-UA bot asing, terutama dari IP datacenter. Sengaja TANPA
// akhiran "Bot"/"Tauthub…": kata "bot" di UA adalah pemicu blokir klasik WAF.
const mjCrawlerUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

var mjCrawlerHeaders = map[string]string{
	"user-agent":      mjCrawlerUA,
	"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"accept-language": "id-ID,id;q=0.9,en;q=0.8",
}

// mjAboutPaths: tebakan path standar bila menu about tak terbaca dari link.
var mjAboutPaths = []string{"/about", "/about-us", "/tentang", "/tentang-kami", "/profil", "/profile"}

var (
	mjSchemePrefixRe = regexp.MustCompile(`(?i)^https?://`)
	mjWWWPrefixRe    = regexp.MustCompile(`(?i)^www\.`)
	mjTimeoutRe      = regexp.MustCompile(`(?i)timeout|abort`)
	// mjAboutLinkRe: kecocokan nama halaman about pada path ATAU teks anchor.
	mjAboutLinkRe = regexp.MustCompile(`(?i)(about|tentang|profil|profile|visi|misi|sejarah|perusahaan|corporate|kenali|siapa\s+kami)`)
	// mjSkipHrefRe: skema yang tidak pernah jadi halaman about.
	mjSkipHrefRe = regexp.MustCompile(`(?i)^(mailto:|tel:|javascript:)`)
	mjAnchorRe   = regexp.MustCompile(`(?is)<a\s[^>]*href\s*=\s*(?:"([^"]*)"|'([^']*)')[^>]*>(.*?)</a>`)

	mjScriptRe   = regexp.MustCompile(`(?is)<script.*?</script>`)
	mjStyleRe    = regexp.MustCompile(`(?is)<style.*?</style>`)
	mjNoscriptRe = regexp.MustCompile(`(?is)<noscript.*?</noscript>`)
	mjCommentRe  = regexp.MustCompile(`(?s)<!--.*?-->`)
	mjTagRe      = regexp.MustCompile(`<[^>]+>`)
	mjSpaceRe    = regexp.MustCompile(`\s+`)

	mjEntityReplacer = strings.NewReplacer(
		"&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", "\"", "&#39;", "'", "&apos;", "'",
	)
)

// mjCrawledPage: catatan satu percobaan fetch (dikirim ke pages_tried jsonb).
type mjCrawledPage struct {
	URL   string `json:"url"`
	OK    bool   `json:"ok"`
	Chars int    `json:"chars"`
	Note  string `json:"note,omitempty"`
}

type mjCrawlResult struct {
	Text  string
	Pages []mjCrawledPage
}

type mjAttemptResult struct {
	Text    string
	HasText bool
	HTML    string
	HasHTML bool
}

// mjCrawlWebsiteText: tarik teks homepage + halaman about (maks 2 halaman sukses).
func mjCrawlWebsiteText(ctx context.Context, rawURL string) mjCrawlResult {
	base := mjNormalizeURL(rawURL)
	if base == "" {
		return mjCrawlResult{Pages: []mjCrawledPage{}}
	}
	originURL := strings.TrimSuffix(base, "/")
	httpBase := "http://" + mjSchemePrefixRe.ReplaceAllString(originURL, "")

	pages := []mjCrawledPage{}
	chunks := []string{}
	okCount := 0
	attempted := 0

	// Satu percobaan fetch: selalu mencatat hasilnya ke pages untuk monitoring.
	attempt := func(target string) mjAttemptResult {
		attempted++
		status, body, contentType, err := mjFetchPage(ctx, target)
		if err != nil {
			note := "Gagal: " + ptTruncate(err.Error(), 80)
			if mjTimeoutRe.MatchString(err.Error()) {
				note = "Timeout"
			}
			pages = append(pages, mjCrawledPage{URL: target, Note: note})
			return mjAttemptResult{}
		}
		if status < 200 || status >= 300 {
			pages = append(pages, mjCrawledPage{URL: target, Note: "HTTP " + ptItoa(status)})
			return mjAttemptResult{}
		}
		if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml") {
			ct := strings.TrimSpace(contentType)
			switch {
			case ct == "":
				ct = "tanpa content-type"
			default:
				ct = ptTruncate(ct, 40)
			}
			pages = append(pages, mjCrawledPage{URL: target, Note: "Bukan HTML (" + ct + ")"})
			return mjAttemptResult{}
		}
		html := ptTruncate(body, mjMaxHTMLChars)
		text := mjHTMLToText(html)
		chars := utf8.RuneCountInString(text)
		usable := chars > 80
		note := ""
		if !usable {
			note = "Konten terlalu pendek"
		}
		pages = append(pages, mjCrawledPage{URL: target, OK: usable, Chars: chars, Note: note})
		return mjAttemptResult{Text: text, HasText: usable, HTML: html, HasHTML: true}
	}

	// 1) Homepage — https dulu, fallback http. Tanpa homepage yang hidup,
	//    halaman about hampir pasti juga tak terjangkau sehingga dilewati.
	homepageHTML := ""
	hasHomepage := false
	for _, target := range []string{originURL, httpBase} {
		res := attempt(target)
		if res.HasHTML {
			homepageHTML = res.HTML
			hasHomepage = true
		}
		if res.HasText {
			chunks = append(chunks, "Sumber: "+target+"\n"+res.Text)
			okCount++
			break
		}
	}

	// 2) Halaman about: link hasil deteksi di homepage diprioritaskan, lalu
	//    tebakan path standar sebagai cadangan.
	if hasHomepage {
		seen := map[string]bool{
			strings.ToLower(strings.TrimSuffix(originURL, "/")): true,
			strings.ToLower(strings.TrimSuffix(httpBase, "/")):  true,
		}
		guessed := make([]string, 0, len(mjAboutPaths)*2)
		for _, path := range mjAboutPaths {
			guessed = append(guessed, originURL+path, httpBase+path)
		}
		candidates := append(mjDiscoverAboutLinks(homepageHTML, originURL), guessed...)
		aboutOK := 0
		for _, target := range candidates {
			if okCount >= mjMaxSuccessPage || attempted >= mjMaxAttempts {
				break
			}
			key := strings.ToLower(strings.TrimSuffix(target, "/"))
			if seen[key] {
				continue
			}
			seen[key] = true
			if res := attempt(target); res.HasText {
				chunks = append(chunks, "Sumber: "+target+"\n"+res.Text)
				okCount++
				aboutOK++
			}
		}
		// 3) Penanda eksplisit "tanpa halaman about".
		if aboutOK == 0 {
			pages = append(pages, mjCrawledPage{URL: "(halaman about)", Note: "Tidak ditemukan halaman about pada situs ini"})
		}
	}

	return mjCrawlResult{Text: ptTruncate(strings.Join(chunks, "\n\n"), mjMaxSourceChars), Pages: pages}
}

// mjFetchPage: GET aman (anti-SSRF) dengan timeout + header crawler.
// status 0 + err bila koneksi gagal.
func mjFetchPage(ctx context.Context, target string) (int, string, string, error) {
	resp, err := security.Get(ctx, target, mjCrawlerHeaders, mjFetchTimeout)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(mjMaxHTMLChars)))
	return resp.StatusCode, string(body), resp.Header.Get("content-type"), nil
}

// mjNormalizeURL: parity normalizeUrl — paksa skema, hanya http/https.
func mjNormalizeURL(raw string) string {
	candidate := raw
	if !strings.Contains(raw, "://") {
		candidate = "https://" + raw
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}

// mjDiscoverAboutLinks: temukan kandidat halaman about dari link internal di
// homepage — dikenali dari gabungan href dan teks anchor, diperingkat skor.
func mjDiscoverAboutLinks(html, originURL string) []string {
	origin, err := url.Parse(originURL)
	if err != nil {
		return nil
	}
	originHost := strings.TrimPrefix(strings.ToLower(origin.Hostname()), "www.")

	order := []string{}
	scores := map[string]int{}
	for _, match := range mjAnchorRe.FindAllStringSubmatch(html, -1) {
		href := strings.TrimSpace(match[1])
		if href == "" {
			href = strings.TrimSpace(match[2])
		}
		anchorText := strings.ToLower(mjHTMLToText(match[3]))
		if href == "" || strings.HasPrefix(href, "#") || mjSkipHrefRe.MatchString(href) {
			continue
		}
		resolved, rErr := origin.Parse(href)
		if rErr != nil {
			continue
		}
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			continue
		}
		// Hanya link internal.
		if strings.TrimPrefix(strings.ToLower(resolved.Hostname()), "www.") != originHost {
			continue
		}
		score := 0
		if mjAboutLinkRe.MatchString(resolved.Path) {
			score += 2
		}
		if mjAboutLinkRe.MatchString(anchorText) {
			score++
		}
		if score == 0 {
			continue
		}
		target := resolved.Scheme + "://" + resolved.Host + resolved.Path
		if _, exists := scores[target]; !exists {
			order = append(order, target)
		}
		if score > scores[target] {
			scores[target] = score
		}
	}

	// Sort skor desc; tie mempertahankan urutan penemuan (parity sort stabil JS).
	sort.SliceStable(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })
	if len(order) > mjMaxAboutLinks {
		order = order[:mjMaxAboutLinks]
	}
	return order
}

// mjHTMLToText: parity htmlToText — buang script/style/noscript/komentar/tag,
// decode entitas dasar, rapatkan whitespace.
func mjHTMLToText(html string) string {
	text := mjScriptRe.ReplaceAllString(html, " ")
	text = mjStyleRe.ReplaceAllString(text, " ")
	text = mjNoscriptRe.ReplaceAllString(text, " ")
	text = mjCommentRe.ReplaceAllString(text, " ")
	text = mjTagRe.ReplaceAllString(text, " ")
	text = mjEntityReplacer.Replace(text)
	return strings.TrimSpace(mjSpaceRe.ReplaceAllString(text, " "))
}

// ---------- kind: crawl_target_sync ----------

// CrawlTargetSyncJob: port cabang `crawl_target_sync` di queue.ts — register
// target + tarik status Crawlhub, lalu simpan salinannya ke crawl_targets.
// Semua langkah ada di igChSyncCrawlTarget (misc_integrations.go); hasil
// ok=false diterjemahkan menjadi error supaya runner me-retry dengan backoff.
func (d *Deps) CrawlTargetSyncJob(ctx context.Context, mediaID string) error {
	// Legacy memeriksa media lebih dulu (media mitra + punya website_url) sebelum
	// konfigurasi Crawlhub, jadi urutan error-nya dipertahankan di sini.
	if _, err := d.igChMediaWebsite(ctx, mediaID); err != nil {
		return err
	}
	cfg, err := d.igChLoadConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.igConfigured() {
		return errors.New("Crawlhub belum dikonfigurasi")
	}
	outcome := d.igChSyncCrawlTarget(ctx, cfg, mediaID)
	if !outcome.OK {
		return errors.New(outcome.Error)
	}
	return nil
}

// ---------- kind: domain_verify ----------

// DomainVerifyJob: port cabang `domain_verify` di queue.ts. Memakai ulang
// mpRecordDomainCheckFailure (domain_diagnostics.go) untuk seluruh penulisan
// kolom domain_*, dan mereplikasi alur fetch mpVerifyDomainOwnership versi
// context (versi aslinya menerima *http.Request). Notifikasi admin dikirim
// saat health bertransisi ke unreachable (parity verifyDomainOwnership).
func (d *Deps) DomainVerifyJob(ctx context.Context, mediaID string) error {
	before, err := d.queryRow(ctx, `
		SELECT id, name, website_url, domain_verify_token, domain_verified, domain_check_fail_count, domain_health, partner_id
		FROM media WHERE id = $1 LIMIT 1`, mediaID)
	if err != nil {
		return err
	}
	if before == nil {
		return errors.New("Media tidak ditemukan")
	}
	previousHealth := before.strOr("domain_health", "unknown")
	result := d.mjVerifyDomainOwnership(ctx, mediaID, before)

	// Kabari admin saat domain PERTAMA kali jatuh (transisi -> unreachable),
	// bukan tiap siklus. Best-effort.
	if status, _ := result["status"].(string); status == "unreachable" && previousHealth != "unreachable" {
		label := before.strOr("name", "")
		if label == "" {
			label = before.strOr("website_url", "")
		}
		website := before.strOr("website_url", "")
		failCount, _ := result["fail_count"].(int)
		body := fmt.Sprintf("Situs %s tidak bisa diakses saat pengecekan otomatis (%dx gagal berturut-turut). Artikel yang tayang di media ini perlu ditinjau.", website, failCount)
		link := "/admin/link-monitor"
		d.notifyPlatformAdminsDomainDown(ctx, mediaID, "Domain mitra tidak terjangkau: "+label, body, &link)
	}

	// Mitra pemilik media diberi tahu setelah 3x gagal berturut-turut ATAU saat
	// diblokir WAF (403) — supaya mengizinkan akses crawler Tauthub atau
	// memasang verifikasi meta tag / TXT. Idempoten per media (30 hari).
	if status, _ := result["status"].(string); status == "unreachable" || status == "blocked_by_waf" {
		failCount, _ := result["fail_count"].(int)
		if failCount >= 3 || status == "blocked_by_waf" {
			d.notifyPartnerCrawlBlocked(ctx, before.strOr("partner_id", ""), mediaID,
				before.strOr("name", ""), before.strOr("website_url", ""), status, failCount)
		}
	}
	return nil
}

// notifyPlatformAdminsDomainDown: notifikasi admin "domain tidak terjangkau"
// dengan dedupe per media (24 jam). Jaring pengaman: guard transisi di
// DomainVerifyJob bergantung pada domain_health yang tersimpan — bila penulisan
// diagnostik gagal, guard itu lolos tiap siklus scheduler dan notifikasi akan
// terulang.
//
// Cek-dan-insert dijalankan DI BAWAH advisory lock per media. Sebelumnya
// keduanya terpisah, sehingga beberapa worker yang memverifikasi media yang
// sama secara bersamaan bisa sama-sama lolos cek lalu menulis duplikat
// (badai 2–3 Okt: 16 ribu baris untuk hanya 23 media).
func (d *Deps) notifyPlatformAdminsDomainDown(ctx context.Context, mediaID, title, body string, link *string) {
	dedupeKey := "media.domain_down:" + mediaID
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		d.logWarn("notifikasi domain_down ke admin gagal", mediaID, err)
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, dedupeKey); err != nil {
		d.logWarn("lock notifikasi domain_down gagal", mediaID, err)
		return
	}
	var dup int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE dedupe_key = $1 AND created_at > NOW() - INTERVAL '24 hours'`,
		dedupeKey).Scan(&dup); err != nil || dup > 0 {
		return
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (id, user_id, type, title, body, link, dedupe_key)
		SELECT gen_random_uuid(), id, 'media.domain_down', $1, $2, $3, $4
		FROM users WHERE role = 'admin' AND is_active = TRUE`,
		title, body, link, dedupeKey); err != nil {
		d.logWarn("notifikasi domain_down ke admin gagal", mediaID, err)
		return
	}
	_ = tx.Commit(ctx)
}

// notifyPartnerCrawlBlocked: notifikasi ke mitra pemilik media bila situsnya
// tidak bisa di-crawl — gagal >= 3x berturut-turut ATAU diblokir WAF — beserta
// dua jalan keluar: izinkan akses crawler di WAF, atau verifikasi domain lewat
// meta tag / TXT record. Idempoten: maksimal satu per media per 30 hari.
func (d *Deps) notifyPartnerCrawlBlocked(ctx context.Context, partnerID, mediaID, mediaName, website, status string, failCount int) {
	if strings.TrimSpace(partnerID) == "" {
		return
	}
	detail := fmt.Sprintf("Pengecekan otomatis sudah gagal %dx berturut-turut", failCount)
	if status == "blocked_by_waf" {
		detail = "Situs Anda hidup, tetapi WAF/firewall menolak crawler Tauthub (HTTP 403)"
	}
	title := "Situs " + mediaName + " tidak bisa diakses crawler Tauthub"
	body := detail + " pada " + website + ". Agar artikel dan pemeriksaan berjalan normal, mohon: " +
		"(1) izinkan akses crawler Tauthub di pengaturan WAF/Cloudflare Anda, atau " +
		"(2) buka halaman Media lalu verifikasi kepemilikan domain lewat meta tag atau TXT record _tauthub-verify, kemudian klik Verifikasi Sekarang."
	link := "/mitra"
	dedupeKey := "media.crawl_blocked:" + mediaID
	// Paritas notifyPartnerReminder: cek duplikat eksplisit, bukan ON CONFLICT.
	dup, dErr := d.queryRow(ctx, `
		SELECT n.id FROM notifications n
		JOIN partners p ON p.user_id = n.user_id
		WHERE p.id = $1 AND n.dedupe_key = $2 AND n.created_at > NOW() - INTERVAL '30 days'
		LIMIT 1`, partnerID, dedupeKey)
	if dErr != nil || dup != nil {
		return
	}
	if err := d.exec(ctx, `
		INSERT INTO notifications (id, user_id, type, title, body, link, dedupe_key)
		SELECT gen_random_uuid(), user_id, 'media.crawl_blocked', $1, $2, $3, $4
		FROM partners WHERE id = $5 AND user_id IS NOT NULL`,
		title, body, link, dedupeKey, partnerID); err != nil {
		d.logWarn("notifikasi crawl_blocked ke mitra gagal", mediaID, err)
	}
}

// mjVerifyDomainOwnership: alur verifikasi yang sama dengan
// mpVerifyDomainOwnership — klasifikasi + fallback DNS TXT terpusat di
// security.VerifyDomainOwnership (dipakai job, tanpa request).
func (d *Deps) mjVerifyDomainOwnership(ctx context.Context, mediaID string, before ptRow) map[string]any {
	website := strings.TrimSpace(before.strOr("website_url", ""))
	if website == "" {
		return d.mpRecordDomainCheckFailure(ctx, mediaID, before,
			"token_not_found", "Website/domain media belum diisi", "no_website", []string{"Website/domain media kosong"})
	}
	token := before.strOr("domain_verify_token", "")
	// Selalu HTTPS: produksi menolak HTTP (safe-fetch) dan hampir semua situs
	// mitra mengalihkan HTTP ke HTTPS sendiri.
	target := strings.Replace(mpEnsureScheme(website), "http://", "https://", 1)

	out := security.VerifyDomainOwnership(ctx, target, token)
	switch out.Result {
	case "verified":
		if err := d.exec(ctx, `
			UPDATE media SET domain_verified = TRUE, domain_verified_at = NOW(), domain_last_checked_at = NOW(),
			  domain_check_fail_count = 0, domain_health = 'live', domain_last_result = 'verified',
			  domain_last_error = NULL, domain_last_stage = $1, domain_last_trace = $2, domain_last_error_at = NULL, updated_at = NOW()
			WHERE id = $3`, out.Stage, mpJSONString(out.Trace), mediaID); err != nil {
			return map[string]any{"ok": false, "status": "error", "stage": "update", "trace": out.Trace}
		}
		if err := d.exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), NULL, 'media.domain_verified', 'media', $1)",
			mediaID); err != nil {
			d.logWarn("audit domain_verified gagal", mediaID, err)
		}
		// Fee affiliate per domain: domain rujukan yang menunggu approve ikut
		// disetujui, lalu CPA-nya dicairkan (idempoten, best-effort).
		_ = d.exec(ctx,
			"UPDATE referred_domains SET status = 'approved', rejection_reason = NULL, updated_at = NOW() WHERE media_id = $1 AND status = 'pending'", mediaID)
		if pendingFee, _ := d.queryRow(ctx,
			"SELECT id FROM referred_domains WHERE media_id = $1 AND status = 'approved' AND cpa_paid = false LIMIT 1", mediaID); pendingFee != nil {
			d.finCreditCPAForDomain(ctx, pendingFee.strOr("id", ""))
		}
		return map[string]any{
			"ok": true, "status": "verified", "checked_at": d.isoNow(),
			"fail_count": 0, "stage": out.Stage, "trace": out.Trace,
		}
	case "blocked_by_waf", "tls_error":
		return d.mpRecordDomainCheckNeutral(ctx, mediaID, out.Result, out.Message, out.Stage, out.Trace)
	case "unreachable":
		return d.mpRecordDomainCheckFailure(ctx, mediaID, before,
			"unreachable", ptTruncate(out.Message, 300), out.Stage, out.Trace)
	case "token_missing":
		result := d.mpRecordDomainCheckFailure(ctx, mediaID, before,
			"token_missing", out.Message, out.Stage, out.Trace)
		result["found_token"] = ptTruncate(out.FoundToken, 80)
		return result
	default:
		return d.mpRecordDomainCheckFailure(ctx, mediaID, before,
			out.Result, out.Message, out.Stage, out.Trace)
	}
}

// ---------- registrasi ----------

// RegisterMediaJobs dipanggil dari cmd/api saat boot.
func (d *Deps) RegisterMediaJobs() {
	jobs.Register("media_seo_refresh", func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["mediaId"].(string)
		if id == "" {
			return fmt.Errorf("payload mediaId kosong")
		}
		return d.MediaSeoRefreshJob(ctx, id)
	})
	jobs.Register("media_geo_refresh", func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["mediaId"].(string)
		if id == "" {
			return fmt.Errorf("payload mediaId kosong")
		}
		return d.MediaGeoRefreshJob(ctx, id)
	})
	jobs.Register("media_ai_summary", func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["mediaId"].(string)
		if id == "" {
			return fmt.Errorf("payload mediaId kosong")
		}
		return d.MediaAISummaryJob(ctx, id)
	})
	jobs.Register("crawl_target_sync", func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["mediaId"].(string)
		if id == "" {
			return fmt.Errorf("payload mediaId kosong")
		}
		return d.CrawlTargetSyncJob(ctx, id)
	})
	jobs.Register("media_similarweb_refresh", func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["mediaId"].(string)
		if id == "" {
			return fmt.Errorf("payload mediaId kosong")
		}
		return d.MediaSimilarWebJob(ctx, id)
	})
	jobs.Register("domain_verify", func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["mediaId"].(string)
		if id == "" {
			return fmt.Errorf("payload mediaId kosong")
		}
		return d.DomainVerifyJob(ctx, id)
	})
}
