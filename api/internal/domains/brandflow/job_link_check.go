package brandflow

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

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
const LinkFailureThreshold = 2

const linkFetchTimeout = 15 * time.Second

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
func (d *Deps) PublicationLinkCheckJob(ctx context.Context, publicationID string) error {
	pub, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, p.media_id, p.published_url,
		       m.name AS media_name, m.partner_id,
		       d.title AS document_title
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN content_documents d ON d.id = p.document_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || pub == nil {
		return err
	}

	previous, err := d.queryRow(ctx,
		"SELECT state, consecutive_failures, first_detected_gone_at FROM publication_link_checks WHERE publication_id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	previousState := LinkState("")
	previousFailures := 0
	var firstDetected *time.Time
	if previous != nil {
		previousState = LinkState(previous.strOr("state", ""))
		previousFailures = previous.intOr("consecutive_failures", 0)
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
	if resp, fErr := security.Get(ctx, url, nil, linkFetchTimeout); fErr != nil {
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
		// Hanya status yang dibutuhkan; body ditutup rapi agar tidak menarik
		// seluruh halaman mitra (bisa ratusan KB per artikel).
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

	if err := d.saveLinkCheck(ctx, publicationID, linkCheckStateInput{
		URL: url, FinalURL: finalURL, HTTPStatus: httpStatus, State: state,
		Failures: failures, FirstDetected: firstDetectedGoneAt, ErrorMessage: errorMessage,
	}); err != nil {
		return err
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
}

// saveLinkCheck: upsert hasil pengecekan (parity saveCheck).
func (d *Deps) saveLinkCheck(ctx context.Context, publicationID string, in linkCheckStateInput) error {
	return d.exec(ctx, `
		INSERT INTO publication_link_checks
		  (publication_id, url, final_url, http_status, state, consecutive_failures,
		   first_detected_gone_at, last_checked_at, error_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8, NOW(), NOW())
		ON CONFLICT (publication_id) DO UPDATE SET
		  url = EXCLUDED.url,
		  final_url = EXCLUDED.final_url,
		  http_status = EXCLUDED.http_status,
		  state = EXCLUDED.state,
		  consecutive_failures = EXCLUDED.consecutive_failures,
		  first_detected_gone_at = EXCLUDED.first_detected_gone_at,
		  last_checked_at = NOW(),
		  error_message = EXCLUDED.error_message,
		  updated_at = NOW()`,
		publicationID, nullIfEmpty(in.URL), nullIfEmpty(in.FinalURL), in.HTTPStatus, string(in.State),
		in.Failures, in.FirstDetected, nullIfEmpty(in.ErrorMessage))
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
