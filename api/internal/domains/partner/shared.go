package partner

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

func (d *Deps) logger() *slog.Logger { return slog.Default() }

func stringOrEmpty(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func strconvFormatFloat(f float64) string {
	// Parity Intl default untuk kolom numerik di CSV: tanpa pemisah ribuan.
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// isHTTPURL: parity z.string().url() + refine /^https?:\/\//i untuk kolom URL.
func isHTTPURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

// uploadDir: parity UPLOAD_DIR (.data/uploads), lalu LEGACY_UPLOAD_DIR sebagai fallback.
func (d *Deps) uploadDir() string {
	if v := strings.TrimSpace(os.Getenv("UPLOAD_DIR")); v != "" {
		return v
	}
	return filepath.Join(".data", "uploads")
}

func (d *Deps) legacyUploadDir() string {
	if v := strings.TrimSpace(os.Getenv("LEGACY_UPLOAD_DIR")); v != "" {
		return v
	}
	return filepath.Join("public", "uploads")
}

// thumbnailSizeBytes: parity uploadSizeBytes — nil bila file tidak ada.
func thumbnailSizeBytes(rawURL string) any {
	if rawURL == "" {
		return nil
	}
	key := rawURL
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		key = rawURL[i+1:]
	}
	if key == "" {
		return nil
	}
	dep := &Deps{}
	for _, dir := range []string{dep.uploadDir(), dep.legacyUploadDir()} {
		if info, err := os.Stat(filepath.Join(dir, key)); err == nil && info.Mode().IsRegular() {
			return info.Size()
		}
	}
	return nil
}

// ---------- notifikasi (port lib/notifications/service.ts) ----------

func (d *Deps) createNotification(ctx context.Context, userID, notifType, title, body string, link *string, publicationID *string) error {
	return d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link, publication_id) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)",
		userID, notifType, title, body, link, publicationID)
}

// notifyBrandMembers: satu INSERT..SELECT ke semua anggota brand.
func (d *Deps) notifyBrandMembers(ctx context.Context, brandID, notifType, title, body string, link *string, publicationID *string) error {
	return d.exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, link, publication_id)
		 SELECT gen_random_uuid(), bm.user_id, $1, $2, $3, $4, $5
		 FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		 WHERE bm.brand_id = $6 AND u.role <> 'admin'`,
		notifType, title, body, link, publicationID, brandID)
}

// waBrandMembers: antrekan WA ke setiap anggota brand (klien) yang punya nomor
// dan izin. Dipakai event publikasi supaya klien menerima kabar di portalnya
// sendiri, terpisah dari template mitra. Best-effort.
func (d *Deps) waBrandMembers(ctx context.Context, brandID, event, referenceID, title, link string, data map[string]string) {
	if brandID == "" {
		return
	}
	rows, err := d.queryRows(ctx,
		`SELECT bm.user_id FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		 WHERE bm.brand_id = $1 AND u.role <> 'admin'`, brandID)
	if err != nil {
		return
	}
	for _, row := range rows {
		userID := row.strOr("user_id", "")
		if userID == "" {
			continue
		}
		payload := map[string]string{"judul": title}
		for k, v := range data {
			payload[k] = v
		}
		notify.EnqueueUserWA(ctx, d.Pool, notify.UserWAOptions{
			Audience:    notify.AudienceClient,
			Event:       event,
			UserID:      userID,
			ReferenceID: referenceID,
			Data:        payload,
			Link:        link,
			BaseURL:     notify.BaseURL(),
		})
	}
}

// waPartnerUser: antrekan WA ke user mitra pemilik media. Best-effort.
func (d *Deps) waPartnerUser(ctx context.Context, userID, event, referenceID, title, link string, data map[string]string) {
	if userID == "" {
		return
	}
	payload := map[string]string{"judul": title}
	for k, v := range data {
		payload[k] = v
	}
	notify.EnqueueUserWA(ctx, d.Pool, notify.UserWAOptions{
		Audience:    notify.AudiencePartner,
		Event:       event,
		UserID:      userID,
		ReferenceID: referenceID,
		Data:        payload,
		Link:        link,
		BaseURL:     notify.BaseURL(),
	})
}

// notifyPartner: kirim ke user mitra; false bila mitra tidak punya user.
func (d *Deps) notifyPartner(ctx context.Context, partnerID, notifType, title, body string, link *string) bool {
	row, err := d.queryRow(ctx, "SELECT user_id FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil || row == nil {
		return false
	}
	userID, ok := row.str("user_id")
	if !ok || userID == "" {
		return false
	}
	if err := d.createNotification(ctx, userID, notifType, title, body, link, nil); err != nil {
		return false
	}
	return true
}

// notifyPlatformAdmins: parity notifyPlatformAdmins (users.role='admin').
func (d *Deps) notifyPlatformAdmins(ctx context.Context, notifType, title, body string, link *string) {
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link) SELECT gen_random_uuid(), id, $1, $2, $3, $4 FROM users WHERE role = 'admin' AND is_active = TRUE",
		notifType, title, body, link); err != nil {
		d.logger().Warn("notify admins gagal", "type", notifType, "err", err)
	}
}

// expireMediaNotifications: tandai notifikasi waiting_partner kedaluwarsa.
func (d *Deps) expireMediaNotifications(ctx context.Context, mediaID string) {
	if err := d.exec(ctx,
		`UPDATE notifications SET expired_at = COALESCE(expired_at, NOW())
		 WHERE type = 'publication.waiting_partner'
		   AND expired_at IS NULL
		   AND publication_id IN (SELECT id FROM publications WHERE media_id = $1)`, mediaID); err != nil {
		d.logger().Warn("expire notifikasi gagal", "media", mediaID, "err", err)
	}
}

// notificationJSON: parity notificationJson.
func notificationJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":             row.raw("id"),
		"type":           row.raw("type"),
		"title":          row.raw("title"),
		"body":           row.raw("body"),
		"link":           row.raw("link"),
		"publication_id": row.raw("publication_id"),
		"read_at":        row.timePtr("read_at"),
		"expired_at":     row.timePtr("expired_at"),
		"created_at":     row.timePtr("created_at"),
	}
}

// ---------- publicationEventNotification (port service.ts) ----------

type publicationEventRow struct {
	ID              string
	BrandID         string
	MediaID         string
	RejectionReason *string
	PublishedURL    *string
	PublishStatus   *string
	MediaName       string
	PartnerMode     *string
	BrandName       string
	ClientName      *string
	BrandPublicID   *string
	DocumentTitle   string
	PartnerUserID   *string
	PartnerPublicID *string
	ErrorMessage    *string
}

func (d *Deps) loadPublicationEventRow(ctx context.Context, publicationID string) *publicationEventRow {
	row, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, p.media_id, p.rejection_reason, p.published_url, p.publish_status,
		       m.name AS media_name, m.partner_id, m.publication_mode,
		       b.name AS brand_name, b.client_name, b.public_id AS brand_public_id,
		       d.title AS document_title,
		       pa.user_id AS partner_user_id, pa.public_id AS partner_public_id,
		       (SELECT error_message FROM publication_attempts WHERE publication_id = p.id ORDER BY attempt_number DESC LIMIT 1) AS error_message
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || row == nil {
		return nil
	}
	ev := &publicationEventRow{
		ID:              row.strOr("id", publicationID),
		BrandID:         row.strOr("brand_id", ""),
		MediaID:         row.strOr("media_id", ""),
		MediaName:       row.strOr("media_name", "Media"),
		DocumentTitle:   row.strOr("document_title", "Artikel"),
		RejectionReason: row.strPtr("rejection_reason"),
		PublishedURL:    row.strPtr("published_url"),
		PublishStatus:   row.strPtr("publish_status"),
		PartnerMode:     row.strPtr("publication_mode"),
		ClientName:      row.strPtr("client_name"),
		BrandName:       row.strOr("brand_name", "Klien"),
		BrandPublicID:   row.strPtr("brand_public_id"),
		PartnerUserID:   row.strPtr("partner_user_id"),
		PartnerPublicID: row.strPtr("partner_public_id"),
		ErrorMessage:    row.strPtr("error_message"),
	}
	return ev
}

func derefOr(p *string, fallback string) string {
	if p == nil || *p == "" {
		return fallback
	}
	return *p
}

// publicationEventNotification: fan-out notifikasi in-app untuk event publikasi.
// Best-effort — kegagalan notifikasi tidak boleh menggagalkan transisi status.
//
// Sejak template WA dipisah per audiens, event publikasi juga mengantrekan WA
// ke mitra DAN klien dengan template masing-masing. Semua panggilan WA bersifat
// best-effort dan tidak pernah menggagalkan transisi status.
func (d *Deps) publicationEventNotification(ctx context.Context, publicationID, event string) {
	ev := d.loadPublicationEventRow(ctx, publicationID)
	if ev == nil {
		return
	}
	brandName := derefOr(ev.ClientName, ev.BrandName)
	brandLink := "/klien/" + derefOr(ev.BrandPublicID, ev.BrandID) + "/publications?pub=" + publicationID
	partnerLink := ""
	hasPartner := ev.PartnerUserID != nil && *ev.PartnerUserID != ""
	if hasPartner {
		partnerLink = "/mitra/" + derefOr(ev.PartnerPublicID, "") + "/publications?pub=" + publicationID
	}

	switch event {
	case "publication.waiting_partner":
		if hasPartner {
			title := "Publikasi menunggu persetujuan"
			body := fmt.Sprintf("%q oleh %s menunggu persetujuan Anda untuk terbit di %s.", ev.DocumentTitle, brandName, ev.MediaName)
			_ = d.createNotification(ctx, *ev.PartnerUserID, event, title, body, &partnerLink, &ev.ID)
		}
	case "publication.partner_approved":
		title := "Publikasi disetujui mitra"
		body := fmt.Sprintf("Mitra %s menyetujui %q. Artikel diantrekan untuk dikirim.", ev.MediaName, ev.DocumentTitle)
		_ = d.notifyBrandMembers(ctx, ev.BrandID, event, title, body, &brandLink, &ev.ID)
		// WA ke klien (audiens client) dan mitra (audiens partner) dengan
		// template masing-masing. Judul yang dipakai adalah judul ARTIKEL,
		// bukan judul notifikasi in-app.
		d.waBrandMembers(ctx, ev.BrandID, event, ev.ID, ev.DocumentTitle, brandLink, map[string]string{
			"nama": brandName, "status": "",
		})
		if hasPartner {
			d.waPartnerUser(ctx, *ev.PartnerUserID, event, ev.ID, ev.DocumentTitle, partnerLink, map[string]string{
				"nama": "", "status": "",
			})
		}
	case "publication.partner_rejected":
		reason := derefOr(ev.RejectionReason, "tanpa alasan")
		title := "Publikasi ditolak mitra"
		body := fmt.Sprintf("Mitra %s menolak %q — %s.", ev.MediaName, ev.DocumentTitle, reason)
		_ = d.notifyBrandMembers(ctx, ev.BrandID, event, title, body, &brandLink, &ev.ID)
		d.waBrandMembers(ctx, ev.BrandID, event, ev.ID, ev.DocumentTitle, brandLink, map[string]string{
			"nama": brandName, "status": reason,
		})
		if hasPartner {
			d.waPartnerUser(ctx, *ev.PartnerUserID, event, ev.ID, ev.DocumentTitle, partnerLink, map[string]string{
				"nama": "", "status": reason,
			})
		}
	case "publication.published":
		draftDelivery := derefOr(firstStr(ev.PublishStatus, ev.PartnerMode), "draft") == "draft"
		urlSuffix := ""
		if ev.PublishedURL != nil && *ev.PublishedURL != "" {
			label := "Draf di"
			if !draftDelivery {
				label = "Terbit di"
			}
			urlSuffix = " " + label + ": " + *ev.PublishedURL
		}
		title := "Artikel terbit"
		body := fmt.Sprintf("%q berhasil terbit di %s.%s", ev.DocumentTitle, ev.MediaName, urlSuffix)
		if draftDelivery {
			title = "Artikel terkirim sebagai draf"
			body = fmt.Sprintf("%q berhasil dikirim ke %s sebagai DRAFT dan menunggu tindakan Anda untuk diterbitkan.%s", ev.DocumentTitle, ev.MediaName, urlSuffix)
		}
		_ = d.notifyBrandMembers(ctx, ev.BrandID, event, title, body, &brandLink, &ev.ID)
		if hasPartner {
			ptitle := "Artikel berhasil diterbitkan"
			pbody := fmt.Sprintf("%q dari %s berhasil diterbitkan di %s%s", ev.DocumentTitle, brandName, ev.MediaName, urlSuffix)
			if draftDelivery {
				ptitle = "Artikel baru menunggu review"
				pbody = fmt.Sprintf("%q dari %s dikirim ke %s sebagai draf dan menunggu tindakan Anda.%s", ev.DocumentTitle, brandName, ev.MediaName, urlSuffix)
			}
			_ = d.createNotification(ctx, *ev.PartnerUserID, event, ptitle, pbody, &partnerLink, &ev.ID)
		}
		// WA hanya untuk pengiriman yang benar-benar terbit. Draf sengaja tidak
		// dikabarkan lewat WA: klien belum punya apa pun untuk dibuka, dan satu
		// pesan WA berbayar untuk itu hanya jadi noise.
		if !draftDelivery {
			d.waBrandMembers(ctx, ev.BrandID, event, ev.ID, ev.DocumentTitle, brandLink, map[string]string{
				"nama": brandName, "status": "",
			})
			if hasPartner {
				d.waPartnerUser(ctx, *ev.PartnerUserID, event, ev.ID, ev.DocumentTitle, partnerLink, map[string]string{
					"nama": "", "status": "",
				})
			}
		}
	case "publication.failed":
		safe := ""
		if ev.ErrorMessage != nil && *ev.ErrorMessage != "" {
			safe = truncateRunes(*ev.ErrorMessage, 200)
		}
		title := "Publikasi gagal"
		body := fmt.Sprintf("%q gagal dikirim ke %s.", ev.DocumentTitle, ev.MediaName)
		if safe != "" {
			body += " " + safe
		} else {
			body += " Silakan coba lagi atau hubungi dukungan."
		}
		_ = d.notifyBrandMembers(ctx, ev.BrandID, event, title, body, &brandLink, &ev.ID)
		if hasPartner {
			pbody := fmt.Sprintf("Artikel %q gagal dikirim ke media Anda (%s).", ev.DocumentTitle, ev.MediaName)
			if safe != "" {
				pbody += " " + safe
			}
			_ = d.createNotification(ctx, *ev.PartnerUserID, event, "Publikasi gagal", pbody, &partnerLink, &ev.ID)
		}
		d.waBrandMembers(ctx, ev.BrandID, event, ev.ID, ev.DocumentTitle, brandLink, map[string]string{
			"nama": brandName, "status": safe,
		})
		if hasPartner {
			d.waPartnerUser(ctx, *ev.PartnerUserID, event, ev.ID, ev.DocumentTitle, partnerLink, map[string]string{
				"nama": "", "status": safe,
			})
		}
	}
}

func firstStr(vals ...*string) *string {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------- signature helpers ----------

func hmacSHA256Hex(key []byte, message string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func constantTimeEqualHex(a, b string) bool {
	ab, err1 := hex.DecodeString(a)
	bb, err2 := hex.DecodeString(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return subtle.ConstantTimeCompare(ab, bb) == 1
}

// randomHex: parity randomBytes(n).toString("hex").
func randomHex(n int) string {
	return hex.EncodeToString(secureRandomBytes(n))
}

func secureRandomBytes(n int) []byte {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand gagal berarti sistem kehilangan entropi — jangan
		// hasilkan kredensial lemah secara diam-diam.
		panic("crypto/rand gagal: " + err.Error())
	}
	return buf
}

func randomBase64URL(n int) string {
	return base64.RawURLEncoding.EncodeToString(secureRandomBytes(n))
}

// checkAuthRate: parity checkAuthRate(bucket, limit) — DB sliding window +
// fallback in-memory (implementasi di internal/auth).
func (d *Deps) checkAuthRate(ctx context.Context, bucket string, limit int) error {
	if err := auth.CheckAuthRate(ctx, d.Pool, bucket, limit); err != nil {
		return errRateLimit()
	}
	return nil
}

// checkAuthRateReq: varian yang menerima *http.Request (handler).
func (d *Deps) checkAuthRateReq(r *http.Request, bucket string, limit int) error {
	return d.checkAuthRate(r.Context(), bucket, limit)
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// errorCodeOf: kode errno jaringan dari error (parity cause.code di Node).
func errorCodeOf(err error) string {
	msg := err.Error()
	for _, code := range []string{"ENOTFOUND", "EAI_AGAIN", "ECONNREFUSED", "ECONNRESET", "ETIMEDOUT", "EHOSTUNREACH", "ENETUNREACH"} {
		if strings.Contains(msg, code) {
			return code
		}
	}
	return ""
}

// errorCodeOr: kode errno atau pesan error yang dipotong (untuk trace diagnosa).
func errorCodeOr(err error) string {
	if code := errorCodeOf(err); code != "" {
		return code
	}
	return truncateRunes(err.Error(), 200)
}

func nowUTC() time.Time { return time.Now().UTC() }

func newUUID() string { return uuid.NewString() }

// nullIfEmpty: string kosong → NULL (parity nilai opsional app lama).
func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// envValue: pembaca env sederhana (dipakai kredensial crawler).
func envValue(key string) string { return os.Getenv(key) }

type docsFile struct {
	Body      string
	UpdatedAt any
}

// readDocsFile: baca file markdown dari folder docs repo (lokasi sama seperti
// kandidat app lama: docs/, docs/legacy/, ../docs/).
func readDocsFile(fileName string) (docsFile, error) {
	candidates := []string{}
	if root := strings.TrimSpace(os.Getenv("DOCS_DIR")); root != "" {
		candidates = append(candidates, filepath.Join(root, fileName))
	}
	candidates = append(candidates,
		filepath.Join("docs", fileName),
		filepath.Join("docs", "legacy", fileName),
		filepath.Join("..", "docs", fileName),
		filepath.Join("..", "docs", "legacy", fileName),
		filepath.Join("..", "..", "docs", fileName),
	)
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		return docsFile{Body: string(data), UpdatedAt: isoMilli(info.ModTime())}, nil
	}
	return docsFile{}, os.ErrNotExist
}

func jsonUnmarshal(raw string, out any) error { return json.Unmarshal([]byte(raw), out) }

// ---------- pembantu HTTP & parsing ----------

func urlParse(raw string) (*url.URL, error) { return url.Parse(raw) }
func urlQueryEscape(v string) string        { return url.QueryEscape(v) }
func urlPathEscape(v string) string         { return url.PathEscape(v) }
func parseFloat(v string) (float64, error)  { return strconv.ParseFloat(v, 64) }

func regexpMustCompile(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }

var regexpWhitespace = regexp.MustCompile(`\s+`)

// syncMutex: mutex sederhana untuk cache in-process.
type syncMutex = sync.Mutex

// postForm: POST x-www-form-urlencoded (OAuth token refresh).
func (d *Deps) postForm(r *http.Request, target, form string, timeout time.Duration) (int, string, http.Header) {
	return d.postJSONHeaders(r, target, form, map[string]string{
		"content-type": "application/x-www-form-urlencoded",
		"accept":       "application/json",
	}, timeout)
}

// postJSON: POST JSON dengan header tambahan.
func (d *Deps) postJSON(r *http.Request, target, body string, headers map[string]string, timeout time.Duration) (int, string, http.Header) {
	merged := map[string]string{"content-type": "application/json"}
	for k, v := range headers {
		merged[k] = v
	}
	return d.postJSONHeaders(r, target, body, merged, timeout)
}

func (d *Deps) postJSONHeaders(r *http.Request, target, body string, headers map[string]string, timeout time.Duration) (int, string, http.Header) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		return 0, "", http.Header{}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := security.SafeClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", http.Header{}
	}
	text, _ := readBodyLimited(resp, 512*1024)
	return resp.StatusCode, text, resp.Header
}

// extractApiDetail: ambil pesan error dari body JSON provider (error.message /
// error_description), dipotong 500 karakter.
func extractApiDetail(body string) string {
	var parsed map[string]any
	if jsonUnmarshal(body, &parsed) != nil {
		return ""
	}
	if errObj, ok := parsed["error"].(map[string]any); ok {
		if msg, isString := errObj["message"].(string); isString && msg != "" {
			return truncateRunes(msg, 500)
		}
	}
	if msg, ok := parsed["error_description"].(string); ok && msg != "" {
		return truncateRunes(msg, 500)
	}
	return ""
}

// base64URLSha256: PKCE code_challenge (S256).
func base64URLSha256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// buildGSCAuthURL: parity buildGscAuthUrl.
func buildGSCAuthURL(clientID, redirectURI, state, codeChallenge string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "https://www.googleapis.com/auth/webmasters.readonly")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("state", state)
	params.Set("include_granted_scopes", "true")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}
