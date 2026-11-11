package brandflow

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// job_link_check.go: processor job kind `publication_link_check` — port
// lib/publication/link-check.ts checkPublicationLink: cek artikel tayang masih
// bisa diakses, simpan hasil, dan beri tahu saat kondisi BERTRANSISI dari sehat
// → bermasalah setelah melewati ambang gagal berturut-turut.

// LinkState: kondisi satu artikel tayang.
type LinkState string

const (
	LinkLive        LinkState = "live"
	LinkGone        LinkState = "gone"
	LinkUnreachable LinkState = "unreachable"
	LinkNoURL       LinkState = "no_url"
)

// LinkFailureThreshold: gagal berturut-turut sebelum dianggap masalah nyata.
// Sekali gagal bisa cuma jaringan berkedip.
const LinkFailureThreshold = db.LinkViolationThreshold

const linkFetchTimeout = 15 * time.Second

// linkCrawlerHeaders: identitas browser pada pengecekan tautan. Tanpa ini Go
// mengirim User-Agent telanjang ("Go-http-client/1.1") dan WAF/CDN media
// (Blogger, Cloudflare, dst.) memanakan request dari IP datacenter dengan
// 403/429 — artikel yang sehat lalu tercatat "Tak Terjangkau" (lihat
// mjCrawlerHeaders di platform/job_media.go yang sudah memakai pola sama).
var linkCrawlerHeaders = map[string]string{
	"user-agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 TauthubLinkCheckBot/1.0",
	"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"accept-language": "id-ID,id;q=0.9,en;q=0.8",
}

// classifyLinkState: peta status HTTP → LinkState. Sengaja konservatif: HANYA
// 404/410 yang dianggap artikel benar-benar hilang; 403/robots dan 5xx masuk
// "unreachable" tanpa menuduh mitra menghapus artikel.
func classifyLinkState(httpStatus *int) LinkState {
	if httpStatus == nil {
		return LinkUnreachable
	}
	if *httpStatus == http.StatusNotFound || *httpStatus == http.StatusGone {
		return LinkGone
	}
	if *httpStatus >= 200 && *httpStatus < 300 {
		return LinkLive
	}
	return LinkUnreachable
}

// linkCheckResult: hasil satu pengecekan.
type linkCheckResult struct {
	State               LinkState
	HTTPStatus          *int
	ConsecutiveFailures int
	URL                 string
	ErrorMessage        string
	Notified            bool
}

// PublicationLinkCheckJob: cek satu publikasi tayang lalu simpan hasilnya
// (parity checkPublicationLink). Mengembalikan nil bila publikasi tidak ada.
//
// Publikasi yang diterbitkan sebagai DRAF di website mitra (publish_status =
// 'draft') TIDAK dicatat: URL-nya belum tampil untuk publik, jadi pengecekan
// hanya akan melaporkan artikel "hilang" palsu di Pelacak 404.
func (d *Deps) PublicationLinkCheckJob(ctx context.Context, publicationID string) error {
	pub, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, p.media_id, p.document_id, p.published_url, p.publish_status,
		       m.name AS media_name, m.partner_id, m.link_rel,
		       d.title AS document_title
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN content_documents d ON d.id = p.document_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || pub == nil {
		return err
	}
	if pub.strOr("publish_status", "draft") == "draft" {
		return nil
	}

	previous, err := d.queryRow(ctx,
		"SELECT state, consecutive_failures, first_detected_gone_at, violation_streak, violation_kind FROM publication_link_checks WHERE publication_id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	previousState := LinkState("")
	previousFailures := 0
	previousStreak := 0
	previousViolation := ""
	var firstDetected *time.Time
	if previous != nil {
		previousState = LinkState(previous.strOr("state", ""))
		previousFailures = previous.intOr("consecutive_failures", 0)
		previousStreak = previous.intOr("violation_streak", 0)
		previousViolation = previous.strOr("violation_kind", "")
		if t := previous.timePtr("first_detected_gone_at"); t != nil {
			if parsed, ok := parseISOTime(t); ok {
				firstDetected = &parsed
			}
		}
	}

	url := strings.TrimSpace(pub.strOr("published_url", ""))
	if url == "" {
		if err := d.saveLinkCheck(ctx, publicationID, linkCheckStateInput{
			State: LinkNoURL, ErrorMessage: "published_url belum diisi",
		}); err != nil {
			return err
		}
		return nil
	}

	var httpStatus *int
	var finalURL string
	var errorMessage string
	liveHTML := ""
	var xRobots []string
	if resp, fErr := security.Get(ctx, url, linkCrawlerHeaders, linkFetchTimeout); fErr != nil {
		// Timeout / DNS gagal / diblokir SSRF guard — bukan bukti artikel hilang.
		msg := fErr.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
		errorMessage = msg
	} else {
		code := resp.StatusCode
		httpStatus = &code
		if resp.Request != nil && resp.Request.URL != nil {
			finalURL = resp.Request.URL.String()
		}
		if finalURL == "" {
			finalURL = url
		}
		// Isi halaman dibaca (terbatas) HANYA untuk mencari tautan di dalam
		// artikel; halaman 404/410 tidak perlu diunduh isinya.
		if code >= 200 && code < 300 {
			xRobots = resp.Header.Values("X-Robots-Tag")
			if body, rErr := io.ReadAll(io.LimitReader(resp.Body, inArticleMaxHTML)); rErr == nil {
				liveHTML = string(body)
			}
		}
		_ = resp.Body.Close()
	}

	state := classifyLinkState(httpStatus)
	failed := state == LinkGone || state == LinkUnreachable
	failures := 0
	if failed {
		failures = previousFailures + 1
	}

	var firstDetectedGoneAt *time.Time
	if state == LinkLive || state == LinkNoURL {
		firstDetectedGoneAt = nil
	} else if firstDetected != nil {
		firstDetectedGoneAt = firstDetected
	} else {
		now := time.Now()
		firstDetectedGoneAt = &now
	}

	// Tautan di dalam artikel hanya bisa dinilai kalau halamannya terbaca.
	// Pengambilan langsung sering diblokir WAF media (state 'unreachable'),
	// jadi Crawlhub dicoba lebih dulu saat isi halaman kosong.
	inArticle := inArticleResult{}
	if state == LinkLive {
		if strings.TrimSpace(liveHTML) == "" {
			if chHTML, chErr := d.crawlhubPageHTML(ctx, url); chErr == nil {
				liveHTML = chHTML
			}
		}
		inArticle = d.checkInArticleLinks(ctx, pub.strOr("document_id", ""), liveHTML)
	}

	// Sinyal tambahan: atribut rel tautan utama dan direktif noindex. Hanya
	// dinilai bila artikel terbaca (state live).
	signals := linkSignals{}
	if state == LinkLive {
		signals = d.checkLinkSignals(ctx, pub.strOr("document_id", ""), liveHTML, xRobots, pub.strOr("link_rel", "dofollow"))
	}

	// Pelanggaran garansi dihitung berturut-turut. Pengecekan yang tidak
	// berinformasi (situs tak terjangkau, halaman diblokir WAF) tidak menambah
	// maupun mereset hitungan, supaya jaringan berkedip tidak menghapus catatan
	// dan tidak pula menuduh media.
	violation := db.LinkViolationKind(string(state), inArticle.State, signals.RelState, signals.NoindexState)
	informative := state == LinkGone ||
		(state == LinkLive && (signals.NoindexState != "unknown" || inArticle.State == "present" || inArticle.State == "removed"))
	streak, violationKind := previousStreak, previousViolation
	if informative {
		if violation != "" {
			streak, violationKind = previousStreak+1, violation
		} else {
			streak, violationKind = 0, ""
		}
	}

	if err := d.saveLinkCheck(ctx, publicationID, linkCheckStateInput{
		URL: url, FinalURL: finalURL, HTTPStatus: httpStatus, State: state,
		Failures: failures, FirstDetected: firstDetectedGoneAt, ErrorMessage: errorMessage,
		InArticleState: inArticle.State, InArticleAnchor: inArticle.Anchor,
		InArticleFound: inArticle.Found, InArticleChecked: inArticle.CheckedAt,
		InArticleErrorMsg: inArticle.ErrMsg,
		RelValue:          signals.Rel, RelState: signals.RelState,
		RobotsValue: signals.Robots, NoindexState: signals.NoindexState,
		ViolationStreak: streak, ViolationKind: violationKind,
	}); err != nil {
		return err
	}

	if informative {
		d.recordLinkSnapshot(ctx, publicationID, state, httpStatus, inArticle, signals, violation)
	}
	if violationKind != "" && streak >= LinkFailureThreshold {
		d.openGuaranteeClaim(ctx, pub, violationKind, inArticle.Anchor)
	}

	wasProblem := previousState == LinkGone || previousState == LinkUnreachable
	shouldNotify := failed && failures >= LinkFailureThreshold && !wasProblem
	if shouldNotify {
		d.alertLinkLost(ctx, pub, state, url, httpStatus)
	}
	return nil
}

type linkCheckStateInput struct {
	URL           string
	FinalURL      string
	HTTPStatus    *int
	State         LinkState
	Failures      int
	FirstDetected *time.Time
	ErrorMessage  string
	// Hasil pemeriksaan tautan di dalam artikel (lihat link_in_article.go).
	InArticleState    string
	InArticleAnchor   string
	InArticleFound    int
	InArticleChecked  *time.Time
	InArticleErrorMsg string
	// Sinyal rel/noindex dan hitungan pelanggaran garansi (lihat db.LinkViolationKind).
	RelValue        string
	RelState        string
	RobotsValue     string
	NoindexState    string
	ViolationStreak int
	ViolationKind   string
}

// saveLinkCheck: upsert hasil pengecekan (parity saveCheck).
func (d *Deps) saveLinkCheck(ctx context.Context, publicationID string, in linkCheckStateInput) error {
	return d.exec(ctx, `
		INSERT INTO publication_link_checks
		  (publication_id, url, final_url, http_status, state, consecutive_failures,
		   first_detected_gone_at, last_checked_at, error_message,
		   in_article_state, in_article_anchor, in_article_found, in_article_checked_at, in_article_error,
		   rel_value, rel_state, robots_value, noindex_state, violation_streak, violation_kind,
		   created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, NOW(), NOW())
		ON CONFLICT (publication_id) DO UPDATE SET
		  url = EXCLUDED.url,
		  final_url = EXCLUDED.final_url,
		  http_status = EXCLUDED.http_status,
		  state = EXCLUDED.state,
		  consecutive_failures = EXCLUDED.consecutive_failures,
		  first_detected_gone_at = EXCLUDED.first_detected_gone_at,
		  last_checked_at = NOW(),
		  error_message = EXCLUDED.error_message,
		  in_article_state = EXCLUDED.in_article_state,
		  in_article_anchor = EXCLUDED.in_article_anchor,
		  in_article_found = EXCLUDED.in_article_found,
		  in_article_checked_at = EXCLUDED.in_article_checked_at,
		  in_article_error = EXCLUDED.in_article_error,
		  rel_value = EXCLUDED.rel_value,
		  rel_state = EXCLUDED.rel_state,
		  robots_value = EXCLUDED.robots_value,
		  noindex_state = EXCLUDED.noindex_state,
		  violation_streak = EXCLUDED.violation_streak,
		  violation_kind = EXCLUDED.violation_kind,
		  updated_at = NOW()`,
		publicationID, nullIfEmpty(in.URL), nullIfEmpty(in.FinalURL), in.HTTPStatus, string(in.State),
		in.Failures, in.FirstDetected, nullIfEmpty(in.ErrorMessage),
		nullIfEmpty(in.InArticleState), nullIfEmpty(in.InArticleAnchor), in.InArticleFound,
		in.InArticleChecked, nullIfEmpty(in.InArticleErrorMsg),
		nullIfEmpty(in.RelValue), nullIfEmpty(in.RelState), nullIfEmpty(in.RobotsValue), nullIfEmpty(in.NoindexState),
		in.ViolationStreak, nullIfEmpty(in.ViolationKind))
}

// recordLinkSnapshot: tulis riwayat kondisi tautan. Baseline kind='publish'
// disimpan sekali, dari pengecekan pertama yang sehat. Baris kind='check'
// ditulis bila kondisinya berbeda dari catatan terakhir atau sudah seminggu
// tanpa catatan, supaya tabel tidak membengkak oleh pengecekan harian yang
// hasilnya sama. Best-effort: kegagalan tidak menggagalkan pengecekan.
func (d *Deps) recordLinkSnapshot(ctx context.Context, publicationID string, state LinkState, httpStatus *int,
	inArticle inArticleResult, sig linkSignals, violation string) {
	insert := func(kind string) error {
		return d.exec(ctx, `
			INSERT INTO publication_link_snapshots
			  (publication_id, kind, href, anchor, rel, robots_meta, x_robots_tag, http_status,
			   link_state, in_article_state, rel_state, noindex_state, checked_at)
			VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, $8, $9, $10, $11, NOW())
			ON CONFLICT DO NOTHING`,
			publicationID, kind, nullIfEmpty(sig.Href), nullIfEmpty(firstNonEmpty(sig.Anchor, inArticle.Anchor)),
			nullIfEmpty(sig.Rel), nullIfEmpty(sig.Robots), httpStatus, string(state),
			nullIfEmpty(inArticle.State), nullIfEmpty(sig.RelState), nullIfEmpty(sig.NoindexState))
	}

	if state == LinkLive && violation == "" {
		baseline, err := d.queryRow(ctx,
			"SELECT 1 AS one FROM publication_link_snapshots WHERE publication_id = $1 AND kind = 'publish' LIMIT 1", publicationID)
		if err == nil && baseline == nil {
			if err := insert("publish"); err != nil {
				slog.Warn("[link-check] simpan baseline gagal", "publication", publicationID, "err", err)
			}
			return
		}
	}

	last, err := d.queryRow(ctx, `
		SELECT link_state, in_article_state, rel, rel_state, noindex_state,
		       (checked_at < NOW() - INTERVAL '7 days') AS stale
		FROM publication_link_snapshots
		WHERE publication_id = $1 ORDER BY checked_at DESC, id DESC LIMIT 1`, publicationID)
	if err != nil {
		return
	}
	if last != nil &&
		!last.boolOr("stale", false) &&
		last.strOr("link_state", "") == string(state) &&
		last.strOr("in_article_state", "") == inArticle.State &&
		last.strOr("rel", "") == sig.Rel &&
		last.strOr("rel_state", "") == sig.RelState &&
		last.strOr("noindex_state", "") == sig.NoindexState {
		return
	}
	if err := insert("check"); err != nil {
		slog.Warn("[link-check] simpan snapshot gagal", "publication", publicationID, "err", err)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// openGuaranteeClaim: buka klaim garansi otomatis setelah pelanggaran terdeteksi
// pada dua pengecekan berturut-turut. Hanya untuk artikel yang masih dalam masa
// garansi media dan medianya terverifikasi (syarat yang sama dengan refund
// admin). Satu klaim per publikasi (UNIQUE), jadi pengecekan berikutnya tidak
// membuka klaim baru dan pemberitahuan hanya terkirim sekali.
func (d *Deps) openGuaranteeClaim(ctx context.Context, pub bsRow, kind, anchor string) {
	publicationID := pub.strOr("id", "")
	claimID := uuid.NewString()
	created, err := d.queryRow(ctx, `
		INSERT INTO link_guardian_claims
		  (id, publication_id, brand_id, status, detection_state, anchor_expected, auto_detected)
		SELECT $1, p.id, p.brand_id, 'submitted', $3, NULLIF($4, ''), TRUE
		FROM publications p
		JOIN media m ON m.id = p.media_id
		WHERE p.id = $2
		  AND COALESCE(m.domain_verified, FALSE)
		  AND `+db.LinkWarrantySQL("p", "m")+`
		ON CONFLICT (publication_id) DO NOTHING
		RETURNING id`, claimID, publicationID, kind, anchor)
	if err != nil {
		slog.Warn("[link-check] buka klaim garansi gagal", "publication", publicationID, "err", err)
		return
	}
	if created == nil {
		return
	}

	title := pub.strOr("document_title", "")
	if title == "" {
		title = "Artikel"
	}
	mediaName := pub.strOr("media_name", "")
	if mediaName == "" {
		mediaName = "media mitra"
	}
	reason := db.LinkViolationLabel(kind)
	brandID := pub.strOr("brand_id", "")

	clientBody := title + " di " + mediaName + ": " + reason + ". Garansi tayang Anda aktif. Pilih artikel pengganti di media lain atau kredit kembali ke saldo."
	clientLink := "/klien/" + brandID + "/link-guardian?publication=" + publicationID
	d.notifyBrandMembersOnPub(ctx, brandID, "publication.guarantee_triggered", "Garansi tautan aktif: "+title, clientBody, &clientLink, publicationID)

	adminLink := "/admin/link-monitor?claim=" + claimID
	d.notifyPlatformAdmins(ctx, "publication.guarantee_triggered", "Garansi tautan terpicu: "+title,
		title+" di "+mediaName+": "+reason+". Klaim dibuka otomatis, klien memilih pengganti atau kredit.", &adminLink)

	// Untuk 404 mitra sudah diberi tahu lewat alertLinkLost; jenis lain belum.
	if partnerID := pub.strOr("partner_id", ""); partnerID != "" && kind != "gone" {
		d.notifyPartnerLinkLost(ctx, partnerID, "Perbaiki tautan: "+title,
			title+" di "+mediaName+": "+reason+". Jika tidak dipulihkan, penghasilan artikel ini dibatalkan.")
	}
}

// alertLinkLost: beri tahu admin, brand member, dan mitra bahwa artikel tidak
// bisa diakses lagi (parity alertLinkLost). Best-effort: kegagalan notifikasi
// tidak boleh menggagalkan hasil pengecekan.
func (d *Deps) alertLinkLost(ctx context.Context, pub bsRow, state LinkState, url string, httpStatus *int) {
	title := pub.strOr("document_title", "")
	if title == "" {
		title = "Artikel"
	}
	mediaName := pub.strOr("media_name", "")
	if mediaName == "" {
		mediaName = "media mitra"
	}
	statusText := "-"
	if httpStatus != nil {
		statusText = itoa(*httpStatus)
	}
	reason := "sementara tidak bisa diakses (HTTP " + statusText + ")"
	if state == LinkGone {
		reason = "tidak ditemukan lagi (HTTP " + statusText + ")"
	}
	body := title + " di " + mediaName + " " + reason + ". URL: " + url

	brandID := pub.strOr("brand_id", "")
	publicationID := pub.strOr("id", "")

	adminLink := "/admin/link-monitor"
	d.notifyPlatformAdmins(ctx, "publication.link_lost", "Artikel hilang: "+title, body, &adminLink)

	brandLink := "/klien/" + brandID
	d.notifyBrandMembersOnPub(ctx, brandID, "publication.link_lost", "Artikel Anda tidak bisa diakses: "+title, body, &brandLink, publicationID)

	if partnerID := pub.strOr("partner_id", ""); partnerID != "" {
		d.notifyPartnerLinkLost(ctx, partnerID, "Artikel perlu dipulihkan: "+title, body)
	}
}

// notifyPartnerLinkLost: notifikasi mitra pemilik media (parity notifyPartner).
func (d *Deps) notifyPartnerLinkLost(ctx context.Context, partnerID, title, body string) {
	partnerLink := "/mitra"
	if err := d.exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, link)
		 SELECT gen_random_uuid(), user_id, 'publication.link_lost', $1, $2, $3
		 FROM partners WHERE id = $4 AND user_id IS NOT NULL`,
		title, body, partnerLink, partnerID); err != nil {
		// Best-effort: jangan gagalkan pengecekan karena notifikasi.
		_ = err
	}
}

// parseISOTime: terima time.Time apa pun dari driver JSON/native.
func parseISOTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		return parseISO(t)
	}
	return time.Time{}, false
}

var _ = errors.Is
