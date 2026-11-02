package brandflow

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// job_publication_execution.go: port lib/publish/executor.ts executePublication
// (jalur worker) — satu percobaan publikasi per eksekusi job; retry dimiliki
// sistem job (`jobs` backoff) dan dicatat ke `publication_attempts`.
//
// Adapter dispatch per media.integration_type:
//   wordpress/webhook → push koneksi CMS brand/mitra
//   rss/manual        → tandai published tanpa push
//   tulisio_api       → adapter mitra bertanda tangan HMAC
//   custom_web        → Tauthub Publisher Protocol v1
//   blogger           → Blogger API v3
//
// Registrasi ke dispatcher dilakukan dari cmd/api dengan Deps berisi Pool;
// lihat RegisterJobs di package ini.

// pePublishError: error ber-kode mesin (parity PublishError) untuk ditulis ke
// publication_jobs.error_code.
type pePublishError struct {
	Code    string
	Message string
}

func (e *pePublishError) Error() string { return e.Message }

func pePublishErr(code, message string) error { return &pePublishError{Code: code, Message: message} }

// peErrorCode: kode mesin dari error (default publish_failed).
func peErrorCode(err error) string {
	if pe, ok := err.(*pePublishError); ok && pe.Code != "" {
		return pe.Code
	}
	return "publish_failed"
}

// ---------- konfigurasi nil-safe (Cfg boleh nil saat registrasi awal) ----------

func (d *Deps) peAppSecret() string {
	if d.Cfg != nil && d.Cfg.Secret != "" {
		return d.Cfg.Secret
	}
	return os.Getenv("APP_SECRET")
}

func (d *Deps) peCMSSecret() string {
	if d.Cfg != nil && d.Cfg.CmsEncryptionSecret != "" {
		return d.Cfg.CmsEncryptionSecret
	}
	if v := strings.TrimSpace(os.Getenv("CMS_ENCRYPTION_SECRET")); v != "" {
		return v
	}
	return d.peAppSecret()
}

// pePublicOrigin: parity canonicalAppBase (PUBLIC_APP_URL / NEXT_PUBLIC_APP_URL).
func (d *Deps) pePublicOrigin() string {
	if d.Cfg != nil {
		if v := strings.TrimSpace(d.Cfg.PublicAppURL); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	for _, key := range []string{"NEXT_PUBLIC_APP_URL", "APP_URL", "NEXT_PUBLIC_API_URL"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	return ""
}

// peConnectionSecret: parity connectionSecretValue — enc_key_ver >= 1 → kunci
// per-entitas (t3:); ver 0 → WordPress APP_SECRET, provider lain CMS secret.
func (d *Deps) peConnectionSecret(row bsRow) (string, error) {
	sealed := row.strOr("sealed_secret", "")
	if sealed == "" {
		return "", nil
	}
	if row.intOr("enc_key_ver", 0) >= 1 {
		return crypto.UnsealV3(sealed, d.peAppSecret(), row.strOr("id", ""))
	}
	secret := d.peAppSecret()
	if row.strOr("provider", "wordpress") != "wordpress" {
		secret = d.peCMSSecret()
	}
	return crypto.Unseal(sealed, secret)
}

// ---------- processor ----------

// PublicationExecutionJob: processor job kind "publication_execution".
func (d *Deps) PublicationExecutionJob(ctx context.Context, publicationID string, attempts, maxAttempts int) error {
	if d.Pool == nil {
		return fmt.Errorf("Deps tanpa Pool — registrasi publication_execution belum lengkap")
	}
	publication, err := d.queryRow(ctx, "SELECT * FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	if publication == nil {
		return nil
	}
	terminal := attempts >= maxAttempts
	status := publication.strOr("status", "")

	if status == "published" || status == "cancelled" {
		return nil
	}
	switch status {
	case "pending", "queued", "retrying", "processing":
	default:
		return nil // status gagal gerbang mitra — bukan urusan worker
	}

	startedAt := time.Now()
	if status != "processing" {
		if status != "queued" {
			if err := d.transitionPublication(ctx, publicationID, "queued"); err != nil {
				return err
			}
		}
		if err := d.transitionPublication(ctx, publicationID, "processing"); err != nil {
			return err
		}
	}

	media, err := d.queryRow(ctx, "SELECT * FROM media WHERE id = $1 LIMIT 1", publication.strOr("media_id", ""))
	if err != nil {
		return err
	}
	if media == nil {
		if err := d.peFailAttempt(ctx, publication, "media_missing", "Media not found", startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("media_missing", "Media not found")
	}
	integrationType := media.strOr("integration_type", "manual")
	if integrationType == "" {
		integrationType = "manual"
	}
	publicOrigin := d.pePublicOrigin()

	document, err := d.queryRow(ctx,
		"SELECT * FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1",
		publication.strOr("document_id", ""), publication.strOr("brand_id", ""))
	if err != nil {
		return err
	}
	if document == nil {
		if err := d.peFailAttempt(ctx, publication, "document_missing", "Article not found", startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("document_missing", "Article not found")
	}

	version, err := d.queryRow(ctx,
		"SELECT content, content_format FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		publication.strOr("document_id", ""), document.intOr("current_version", 1))
	if err != nil {
		return err
	}
	if version == nil {
		if err := d.peFailAttempt(ctx, publication, "version_missing", "Current content version is missing", startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("version_missing", "Current content version is missing")
	}

	// Partner edits (fitur edit saat Menunggu): versi editan mitra dipakai
	// untuk delivery tanpa mengubah dokumen klien.
	editsRow, err := d.queryRow(ctx, "SELECT partner_draft_edits FROM publications WHERE id = $1 LIMIT 1", publication.strOr("id", ""))
	if err != nil {
		return err
	}
	var partnerEdits map[string]any
	if editsRow != nil {
		if m, ok := editsRow.jsonField("partner_draft_edits").(map[string]any); ok {
			partnerEdits = m
		}
	}
	document, version, publication = peApplyPartnerEdits(document, version, publication, partnerEdits)

	// Batas tautan keluar media — gerbang FINAL sebelum konten dikirim. Isi
	// artikel dibaca dari versi TERBARU saat pengiriman, jadi klien masih bisa
	// mengedit tautan setelah menekan "Terbit" (kasus nyata: distribusi 11:39,
	// edit tautan 11:42, terbit 12:39). Hitung ulang di sini; alih-alih terbit,
	// publikasi gagal dengan alasan yang bisa diperbaiki klien.
	var maxLinksPtr *int
	if media.has("max_links") {
		v := int(media.numOr("max_links"))
		maxLinksPtr = &v
	}
	capLinks, allowedLinks := mediaLinkCap(media.boolOr("allow_hyperlink", false), maxLinksPtr)
	outgoing := countOutboundLinks(version.strOr("content", ""), hostSet(media.strOr("website_url", "")))
	if (!allowedLinks && outgoing > 0) || (allowedLinks && outgoing > capLinks) {
		msg := "Tautan keluar melebihi kebijakan media: artikel memuat " + itoa(outgoing)
		if !allowedLinks {
			msg += " tautan, media tidak menerima tautan."
		} else {
			msg += " tautan, maksimal " + itoa(capLinks) + "."
		}
		msg += " Hapus kelebihan tautan lalu minta penerbitan ulang."
		if err := d.peFailAttempt(ctx, publication, "link_policy", msg, startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("link_policy", msg)
	}

	// Media nofollow (menerima tautan, tapi hanya nofollow): artikel tidak
	// ditolak. Tautan keluar diberi rel="nofollow" otomatis pada HTML yang
	// dikirim (lihat peRenderedHTML), jadi artikel yang sudah jadi tetap bisa terbit.
	if media.has("allow_hyperlink") && mediaForcesNofollow(media.boolOr("allow_hyperlink", true), maxLinksPtr) {
		version = peCloneRow(version)
		version["x_force_nofollow"] = true
		version["x_own_site"] = media.strOr("website_url", "")
	}

	// Content Safety: gerbang final publikasi — best-effort, tidak boleh
	// mematahkan alur yang sudah ada.
	if modErr := d.peModerationGate(ctx, publicationID, publication, document, version, startedAt); modErr != nil {
		return modErr
	}

	// RSS/manual: pull/manual — tidak ada push; cukup tandai published.
	if integrationType == "rss" || integrationType == "manual" {
		if err := d.exec(ctx,
			"UPDATE publications SET published_at = NOW(), updated_at = NOW() WHERE id = $1", publication.strOr("id", "")); err != nil {
			return err
		}
		if err := d.transitionPublication(ctx, publicationID, "published"); err != nil {
			return err
		}
		if err := d.peRecordAttempt(ctx, publicationID, "published", nil, nil, startedAt); err != nil {
			return err
		}
		d.peRecordFinance(ctx, publication, media)
		return nil
	}

	switch integrationType {
	case "tulisio_api":
		return d.peRunTulisioAPI(ctx, publication, media, document, version, publicOrigin, startedAt, terminal)
	case "custom_web":
		return d.peRunCustomWeb(ctx, publication, media, document, version, publicOrigin, startedAt, terminal)
	case "blogger":
		return d.peRunBlogger(ctx, publication, media, document, version, publicOrigin, startedAt, terminal)
	}

	return d.peRunCMS(ctx, publication, media, document, version, publicOrigin, startedAt, terminal, integrationType)
}

// ---------- percobaan & konten ----------

// peFailAttempt: parity failPublicationAttempt — retrying bila belum terminal,
// failed bila percobaan terakhir.
func (d *Deps) peFailAttempt(ctx context.Context, publication bsRow, code, message string, startedAt time.Time, terminal bool) error {
	pubID := publication.strOr("id", "")
	next := "retrying"
	if terminal {
		next = "failed"
	}
	if err := d.transitionPublication(ctx, pubID, next); err != nil {
		return err
	}
	var codePtr, msgPtr *string
	if code != "" {
		codePtr = &code
	}
	if message != "" {
		msgPtr = &message
	}
	if err := d.recordPublicationAttempt(ctx, pubID, next, codePtr, msgPtr, startedAt, time.Now()); err != nil {
		slog.Warn("[brandflow] record attempt gagal", "publication", pubID, "err", err)
	}
	// Gagal permanen = pesanan tidak akan terbit. Kredit yang masih ditahan
	// WAJIB dikembalikan; tanpa ini saldo klien menggantung di reserved_tc
	// selamanya dan hanya kembali kalau ada manusia membatalkan manual.
	if terminal {
		if brandID := publication.strOr("brand_id", ""); brandID != "" {
			d.releaseCredit(ctx, brandID, pubID, "system")
		}
	}
	return nil
}

// peRecordAttempt: rekam attempt sukses (status published).
func (d *Deps) peRecordAttempt(ctx context.Context, publicationID, status string, code, message *string, startedAt time.Time) error {
	if err := d.recordPublicationAttempt(ctx, publicationID, status, code, message, startedAt, time.Now()); err != nil {
		slog.Warn("[brandflow] record attempt gagal", "publication", publicationID, "err", err)
	}
	return nil
}

// peApplyPartnerEdits: override judul/html/featured dari partner_draft_edits.
func peApplyPartnerEdits(document, version, publication bsRow, edits map[string]any) (bsRow, bsRow, bsRow) {
	if edits == nil {
		return document, version, publication
	}
	editedTitle, hasTitle := edits["title"].(string)
	if hasTitle && strings.TrimSpace(editedTitle) == "" {
		hasTitle = false
	}
	editedHTML, hasHTML := edits["content_html"].(string)
	if hasHTML && strings.TrimSpace(editedHTML) == "" {
		hasHTML = false
	}
	if hasTitle || hasHTML {
		if hasTitle {
			document = peCloneRow(document)
			document["title"] = strings.TrimSpace(editedTitle)
		}
		if hasHTML {
			version = peCloneRow(version)
			version["content"] = editedHTML
			version["content_format"] = "html"
		}
	}
	if editedFeatured, ok := edits["featured_image_url"].(string); ok && strings.TrimSpace(editedFeatured) != "" {
		publication = peCloneRow(publication)
		publication["featured_image_url"] = strings.TrimSpace(editedFeatured)
	}
	return document, version, publication
}

func peCloneRow(row bsRow) bsRow {
	out := make(bsRow, len(row)+2)
	for k, v := range row {
		out[k] = v
	}
	return out
}

// peRenderedHTML: markdown → HTML, rapikan sisa markup editor, buang H1 pertama,
// lalu absolutisasi src="/uploads/..." ke origin publik. URL gambar juga diberi
// tanda tangan supaya bisa dibaca pengunjung situs media tanpa sesi login.
func (d *Deps) peRenderedHTML(version bsRow, publicOrigin string) (content, html string) {
	content = version.strOr("content", "")
	rendered := content
	if strings.ToLower(version.strOr("content_format", "")) == "markdown" {
		rendered = bsPublishMarkdown(content)
	}
	rendered = contentengine.NormalizeArticleHTML(bsStripH1(rendered))
	if forced, _ := version["x_force_nofollow"].(bool); forced {
		own, _ := version["x_own_site"].(string)
		rendered = forceNofollowOutbound(rendered, hostSet(own))
	}
	return content, peAbsolutizeUploadURLs(rendered, publicOrigin, d.peAppSecret())
}

// ---------- gerbang moderasi ----------

// peModerationGate: port blok Content Safety di executePublication. Kegagalan
// moderasi yang bersifat PublishError diteruskan; error lain best-effort.
func (d *Deps) peModerationGate(ctx context.Context, publicationID string, publication, document, version bsRow, startedAt time.Time) error {
	docTitle := document.strOr("title", "")
	content := truncateString(version.strOr("content", ""), 5000)

	var slug, meta *string
	if v := document.strOr("slug", ""); v != "" {
		slug = &v
	}
	if v := document.strOr("meta_description", ""); v != "" {
		meta = &v
	}
	var contentPtr *string
	if content != "" {
		contentPtr = &content
	}
	brandID := publication.strOr("brand_id", "")
	mod := moderateContent(ctx, d.Pool, moderationInput{
		Title: &docTitle, Slug: slug, MetaDescription: meta, Content: contentPtr, BrandID: &brandID,
	})
	approved := false
	if audit, err := d.queryRow(ctx,
		"SELECT review_decision FROM content_moderation_audits WHERE entity_id = $1 ORDER BY created_at DESC LIMIT 1", publicationID); err == nil && audit != nil {
		approved = audit.strOr("review_decision", "") == "approved"
	}
	if !approved {
		recordModerationAudit(ctx, d.Pool, "publication", publicationID, mod, &brandID)
	}
	if mod.Status == "blocked" {
		if err := d.peFailAttempt(ctx, publication, "moderation_blocked", moderationClientSafeMessage("blocked"), startedAt, true); err != nil {
			return err
		}
		return pePublishErr("moderation_blocked", moderationClientSafeMessage("blocked"))
	}
	if mod.Status == "review_required" && !approved {
		if err := d.peFailAttempt(ctx, publication, "moderation_review_required", moderationClientSafeMessage("review_required"), startedAt, true); err != nil {
			return err
		}
		return pePublishErr("moderation_review_required", moderationClientSafeMessage("review_required"))
	}
	return nil
}

// ---------- adapter: tulisio_api ----------

type peTulisioTarget struct {
	AdapterURL string
	APIKey     string
	Secret     string
}

// peResolveTulisioTarget: parity resolveTulisioApiTarget.
func (d *Deps) peResolveTulisioTarget(ctx context.Context, media bsRow) *peTulisioTarget {
	mediaPartnerID := media.strOr("partner_id", "")
	if mediaPartnerID == "" {
		return nil
	}
	adapterURL := strings.TrimSpace(media.strOr("adapter_url", ""))
	if adapterURL == "" {
		return nil
	}
	key, err := d.queryRow(ctx, `
		SELECT id, api_key_sealed, secret_sealed FROM publishing_api_keys
		WHERE partner_id = $1 AND status = 'active' AND (media_id = $2 OR media_id IS NULL)
		ORDER BY (media_id = $2) DESC, created_at ASC LIMIT 1`, mediaPartnerID, media.strOr("id", ""))
	if err != nil || key == nil {
		return nil
	}
	apiKey, errKey := d.unsealT2(key.strOr("api_key_sealed", ""))
	secret, errSecret := d.unsealT2(key.strOr("secret_sealed", ""))
	if errKey != nil || errSecret != nil {
		return nil
	}
	return &peTulisioTarget{AdapterURL: adapterURL, APIKey: apiKey, Secret: secret}
}

func peSignHMAC(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + body))
	return hex.EncodeToString(mac.Sum(nil))
}

func (d *Deps) peRunTulisioAPI(ctx context.Context, publication, media, document, version bsRow, publicOrigin string, startedAt time.Time, terminal bool) error {
	target := d.peResolveTulisioTarget(ctx, media)
	if target == nil {
		missing := "adapter URL belum diatur"
		if media.strOr("adapter_url", "") != "" {
			missing = "publishing API key belum dibuat"
		}
		reason := "Media tulisio_api " + missing
		if err := d.peFailAttempt(ctx, publication, "partner_not_ready", reason, startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("partner_not_ready", reason)
	}

	jobRecord, err := d.peFindOrCreateJob(ctx, publication, "", "", startedAt, nil)
	if err != nil {
		return err
	}
	jobID := jobRecord.strOr("id", "")
	if jobRecord.strOr("status", "") == "completed" {
		return d.peMarkPublished(ctx, publication, startedAt)
	}

	content, html := d.peRenderedHTML(version, publicOrigin)
	err = d.pePublishViaTulisioAPI(ctx, jobID, publication, media, document, content, html, target)
	if err != nil {
		return d.peFailDelivery(ctx, publication, jobID, err, startedAt, terminal)
	}
	if err := d.exec(ctx, "UPDATE publication_jobs SET completed_at = NOW() WHERE id = $1", jobID); err != nil {
		return err
	}
	return d.peMarkPublished(ctx, publication, startedAt)
}

// pePublishViaTulisioAPI: parity publishViaTulisioApi (satu percobaan).
func (d *Deps) pePublishViaTulisioAPI(ctx context.Context, jobID string, publication, media, document bsRow, contentMarkdown, contentHTML string, target *peTulisioTarget) error {
	slug := document.strOr("slug", "")
	if slug == "" {
		slug = bsSlugify(document.strOr("title", ""))
	}
	payload := map[string]any{
		"publication": map[string]any{
			"id": publication.raw("id"), "publish_status": publication.strOr("publish_status", "draft"),
			"scheduled_at": publication.raw("scheduled_at"), "published_at": publication.raw("published_at"),
			"client_price": publication.numOr("client_price"),
		},
		"article": map[string]any{
			"id": document.raw("id"), "title": document.raw("title"), "slug": slug,
			"content_markdown": contentMarkdown, "content_html": contentHTML,
			"excerpt": document.raw("meta_description"), "meta_title": document.raw("meta_title"),
			"meta_description": document.raw("meta_description"), "language": document.raw("language"),
			"primary_keyword": document.raw("primary_keyword"),
			"category": strings.TrimSpace(publication.strOr("category_override", "")),
		},
		"media": map[string]any{
			"id": media.raw("id"), "name": media.raw("name"),
			"website_url": media.raw("website_url"), "partner_id": media.raw("partner_id"),
		},
		"tracking": map[string]any{"publication_id": publication.raw("id")},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := peSignHMAC(target.Secret, timestamp, string(raw))
	headers := map[string]string{
		"Content-Type":              "application/json",
		"Authorization":             "Bearer " + target.APIKey,
		"X-Tauthub-Timestamp":       timestamp,
		"X-Tulisio-Timestamp":       timestamp,
		"X-Tauthub-Signature":       signature,
		"X-Tulisio-Signature":       signature,
		"X-Tauthub-Idempotency-Key": jobID,
		"X-Tulisio-Idempotency-Key": jobID,
	}
	if err := d.exec(ctx,
		"UPDATE publication_jobs SET status = 'running', attempts = attempts + 1 WHERE id = $1", jobID); err != nil {
		return err
	}
	status, text, doErr := bsHTTPDo(ctx, http.MethodPost, target.AdapterURL, headers, string(raw), 30*time.Second)
	if doErr != nil {
		return pePublishErr("adapter_unreachable",
			"Publishing API adapter tidak dapat dijangkau: "+truncateString(doErr.Error(), 200))
	}
	if status < 200 || status >= 300 {
		code := "adapter_rejected"
		if status >= 500 {
			code = "adapter_error"
		}
		return pePublishErr(code, "Adapter menolak payload (status "+itoa(status)+"): "+truncateString(text, 400))
	}
	var result struct {
		Accepted   *bool `json:"accepted"`
		ExternalID any   `json:"externalId"`
		Status     any   `json:"status"`
		Message    any   `json:"message"`
	}
	_ = json.Unmarshal([]byte(text), &result)
	if result.Accepted == nil || !*result.Accepted {
		msg := "Adapter tidak mengembalikan accepted:true"
		if s, ok := result.Message.(string); ok && s != "" {
			msg += " — " + s
		}
		return pePublishErr("adapter_not_accepted", msg)
	}
	var externalID any
	if result.ExternalID != nil {
		externalID = bsStringifyAny(result.ExternalID)
	}
	var remoteURL any
	if statusStr := bsStringifyAny(result.Status); strings.HasPrefix(statusStr, "http://") || strings.HasPrefix(statusStr, "https://") {
		remoteURL = statusStr
	}
	return d.exec(ctx,
		"UPDATE publication_jobs SET remote_post_id = COALESCE($1, remote_post_id), remote_url = COALESCE($2, remote_url), status = 'completed', error_message = NULL WHERE id = $3",
		externalID, remoteURL, jobID)
}

// ---------- adapter: custom_web ----------

type peCustomWebTarget struct {
	EndpointURL string
	APIKey      string
}

// peResolveCustomWebTarget: parity resolveCustomWebTarget.
func (d *Deps) peResolveCustomWebTarget(media bsRow) *peCustomWebTarget {
	endpoint := strings.TrimSpace(media.strOr("custom_web_endpoint_url", ""))
	sealed := media.strOr("custom_web_api_key_sealed", "")
	if endpoint == "" || sealed == "" {
		return nil
	}
	var apiKey string
	var err error
	if media.intOr("enc_key_ver", 0) >= 1 {
		apiKey, err = crypto.UnsealV3(sealed, d.peAppSecret(), media.strOr("id", ""))
	} else {
		apiKey, err = crypto.Unseal(sealed, d.peCMSSecret())
	}
	if err != nil {
		return nil
	}
	return &peCustomWebTarget{EndpointURL: strings.TrimRight(endpoint, "/"), APIKey: apiKey}
}

// peProtocolURL: parity protocolUrl (lib/publish/custom-web.ts) — dukung
// protokol baru /v1/tauthub/* DAN legacy /v1/tulisio/*:
//   - endpoint sudah berisi path publish → dipakai apa adanya
//   - endpoint berisi path ping → tukar ke path publish (atau normalisasi ping)
//   - endpoint dasar → tambahkan path publish/ping
func peProtocolURL(endpointURL, action string) string {
	const (
		publishTauthub = "/v1/tauthub/publish"
		pingTauthub    = "/v1/tauthub/ping"
		publishLegacy  = "/v1/tulisio/publish"
		pingLegacy     = "/v1/tulisio/ping"
	)
	base := strings.TrimRight(endpointURL, "/")
	if strings.HasSuffix(base, publishTauthub) || strings.HasSuffix(base, publishLegacy) {
		return base
	}
	if strings.HasSuffix(base, pingTauthub) || strings.HasSuffix(base, pingLegacy) {
		if action == "publish" {
			suffix := pingTauthub
			if strings.HasSuffix(base, pingLegacy) {
				suffix = pingLegacy
			}
			return strings.TrimSuffix(base, suffix) + publishTauthub
		}
		return strings.TrimSuffix(base, pingLegacy) + pingTauthub
	}
	if action == "ping" {
		return base + pingTauthub
	}
	return base + publishTauthub
}

func (d *Deps) peRunCustomWeb(ctx context.Context, publication, media, document, version bsRow, publicOrigin string, startedAt time.Time, terminal bool) error {
	target := d.peResolveCustomWebTarget(media)
	if target == nil {
		missing := "endpoint URL belum diatur"
		if media.strOr("custom_web_endpoint_url", "") != "" {
			missing = "API key belum dikonfigurasi"
		}
		reason := "Media custom_web " + missing
		if err := d.peFailAttempt(ctx, publication, "partner_not_ready", reason, startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("partner_not_ready", reason)
	}

	jobRecord, err := d.peFindOrCreateJob(ctx, publication, "", "", startedAt, nil)
	if err != nil {
		return err
	}
	jobID := jobRecord.strOr("id", "")
	if jobRecord.strOr("status", "") == "completed" {
		return d.peMarkPublished(ctx, publication, startedAt)
	}

	_, html := d.peRenderedHTML(version, publicOrigin)
	excerpt := truncateString(document.strOr("meta_description", ""), 500)
	slug := document.strOr("slug", "")
	// Kategori pilihan mitra (dari pengeditan draf) yang lebih dulu memasuki
	// payload protokol Publisher — plugin target membuat/melampirkan kategori.
	categoryName := strings.TrimSpace(publication.strOr("category_override", ""))
	if categoryName == "" {
		categoryName = strings.TrimSpace(media.strOr("category", ""))
	}
	categorySlug := ""
	if categoryName != "" {
		categorySlug = strings.ToLower(strings.Join(strings.Fields(categoryName), "-"))
	}
	if err := d.exec(ctx,
		"UPDATE publication_jobs SET status = 'processing', started_at = COALESCE(started_at, NOW()) WHERE id = $1", jobID); err != nil {
		return err
	}
	featured := d.peAbsolutizeValue(publication.strOr("featured_image_url", ""), publicOrigin)
	result, pubErr := pePublishToCustomWeb(ctx, jobID, publication, document, target, html, excerpt, slug,
		featured, document.strOr("title", ""), categoryName, categorySlug)
	if pubErr != nil {
		return d.peFailDelivery(ctx, publication, jobID, pubErr, startedAt, terminal)
	}
	if !result.Success {
		code := result.ErrorCode
		if code == "" {
			code = "publish_failed"
		}
		message := result.Message
		if message == "" {
			message = "Custom Web publish failed"
		}
		return d.peFailDelivery(ctx, publication, jobID, pePublishErr(code, message), startedAt, terminal)
	}
	if err := d.exec(ctx,
		"UPDATE publication_jobs SET remote_post_id = COALESCE(remote_post_id, $1), remote_url = COALESCE(remote_url, $2), status = 'completed', completed_at = NOW(), progress = 100 WHERE id = $3",
		anyOrNil(result.RemoteID), anyOrNil(result.RemoteURL), jobID); err != nil {
		return err
	}
	if err := d.exec(ctx,
		"UPDATE publications SET published_url = COALESCE(published_url, $1), published_at = NOW(), updated_at = NOW() WHERE id = $2",
		anyOrNil(result.RemoteURL), publication.strOr("id", "")); err != nil {
		return err
	}
	return d.peMarkPublished(ctx, publication, startedAt)
}

type peCustomWebResult struct {
	Success   bool
	RemoteID  string
	RemoteURL string
	ErrorCode string
	Message   string
}

// pePublishToCustomWeb: parity publishToCustomWeb (Tauthub Publisher Protocol v1).
func pePublishToCustomWeb(ctx context.Context, jobID string, publication, document bsRow, target *peCustomWebTarget, contentHTML, excerpt, slug string, featuredImageURL any, featuredImageAlt, categoryName, categorySlug string) (peCustomWebResult, error) {
	publishStatus := "draft"
	if publication.strOr("publish_status", "draft") == "publish" {
		publishStatus = "publish"
	}
	payload := map[string]any{
		"publication_id": publication.strOr("id", ""),
		"title":          document.strOr("title", ""),
		"content":        contentHTML,
		"excerpt":        excerpt,
		"slug":           slug,
		"status":         publishStatus,
	}
	if featured, ok := featuredImageURL.(string); ok && featured != "" {
		payload["featured_image"] = map[string]any{"url": featured, "alt": featuredImageAlt}
	}
	if categoryName != "" {
		fallbackSlug := categorySlug
		if fallbackSlug == "" {
			fallbackSlug = categoryName
		}
		payload["category"] = map[string]any{"name": categoryName, "slug": fallbackSlug}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return peCustomWebResult{}, err
	}
	requestID := "pub_" + jobID
	headers := map[string]string{
		"Content-Type":               "application/json",
		"Authorization":              "Bearer " + target.APIKey,
		"X-Tauthub-Protocol-Version": "1.0",
		"X-Tulisio-Protocol-Version": "1.0",
		"X-Tauthub-Request-Id":       requestID,
		"X-Tulisio-Request-Id":       requestID,
	}
	status, text, doErr := bsHTTPDo(ctx, http.MethodPost, peProtocolURL(target.EndpointURL, "publish"), headers, string(raw), 30*time.Second)
	if doErr != nil {
		return peCustomWebResult{}, doErr
	}
	var body struct {
		Success   *bool  `json:"success"`
		Status    any    `json:"status"`
		RemoteID  any    `json:"remote_id"`
		RemoteURL any    `json:"remote_url"`
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	_ = json.Unmarshal([]byte(text), &body)
	if status < 200 || status >= 300 || body.Success == nil || !*body.Success {
		code := body.ErrorCode
		if code == "" {
			code = "PUBLISH_FAILED"
		}
		message := body.Message
		if message == "" {
			message = "HTTP " + itoa(status)
		}
		return peCustomWebResult{Success: false, ErrorCode: code, Message: message}, nil
	}
	return peCustomWebResult{
		Success:   true,
		RemoteID:  bsStringifyAny(body.RemoteID),
		RemoteURL: bsStringifyAny(body.RemoteURL),
	}, nil
}

// ---------- adapter: blogger ----------

type peBloggerCredential struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
	GoogleEmail  string
	Scope        string
	// DriveFolderID: folder Drive mitra untuk gambar artikel, dibuat sekali lalu
	// dipakai ulang supaya berkas tidak menumpuk di root Drive mitra.
	DriveFolderID string
}

func (d *Deps) peSealBloggerCredential(cred peBloggerCredential, mediaID string) (string, error) {
	plain, err := json.Marshal(map[string]any{
		"access_token": cred.AccessToken, "refresh_token": cred.RefreshToken,
		"expires_at": cred.ExpiresAt, "google_email": cred.GoogleEmail, "scope": cred.Scope,
		"drive_folder_id": cred.DriveFolderID,
	})
	if err != nil {
		return "", err
	}
	return crypto.SealV3(string(plain), d.peAppSecret(), mediaID)
}

func (d *Deps) peUnsealBloggerCredential(sealed, mediaID string, encKeyVer int) (*peBloggerCredential, error) {
	var plain string
	var err error
	if encKeyVer >= 1 {
		plain, err = crypto.UnsealV3(sealed, d.peAppSecret(), mediaID)
	} else {
		plain, err = crypto.Unseal(sealed, d.peCMSSecret())
	}
	if err != nil {
		return nil, err
	}
	var parsed struct {
		AccessToken   string `json:"access_token"`
		RefreshToken  string `json:"refresh_token"`
		ExpiresAt     any    `json:"expires_at"`
		GoogleEmail   string `json:"google_email"`
		Scope         string `json:"scope"`
		DriveFolderID string `json:"drive_folder_id"`
	}
	if err := json.Unmarshal([]byte(plain), &parsed); err != nil || parsed.AccessToken == "" {
		return nil, fmt.Errorf("kredensial Blogger tidak valid")
	}
	return &peBloggerCredential{
		AccessToken:   parsed.AccessToken,
		RefreshToken:  parsed.RefreshToken,
		ExpiresAt:     int64(peNumFromAny(parsed.ExpiresAt)),
		GoogleEmail:   parsed.GoogleEmail,
		Scope:         parsed.Scope,
		DriveFolderID: parsed.DriveFolderID,
	}, nil
}

// peBloggerScopeFromRefresh: Google hanya mengembalikan `scope` saat ada
// perubahan; nilai lama dipertahankan bila respons tidak memuatnya.
func peBloggerScopeFromRefresh(previous, fromResponse string) string {
	if strings.TrimSpace(fromResponse) != "" {
		return fromResponse
	}
	return previous
}

// peResolveBloggerTarget: parity resolveBloggerTarget (refresh bila expired).
func (d *Deps) peResolveBloggerTarget(ctx context.Context, media bsRow, clientID, clientSecret string) (*peBloggerCredential, error) {
	sealed := media.strOr("blogger_credentials_sealed", "")
	if sealed == "" {
		return nil, nil
	}
	cred, err := d.peUnsealBloggerCredential(sealed, media.strOr("id", ""), media.intOr("enc_key_ver", 0))
	if err != nil || cred == nil {
		return nil, nil
	}
	if cred.ExpiresAt <= time.Now().UnixMilli()+60_000 {
		if cred.RefreshToken == "" {
			return nil, nil
		}
		accessToken, expiresIn, scope, refreshErr := peRefreshBloggerToken(ctx, cred.RefreshToken, clientID, clientSecret)
		if refreshErr != nil {
			return nil, refreshErr
		}
		cred.AccessToken = accessToken
		cred.ExpiresAt = time.Now().UnixMilli() + int64(expiresIn)*1000
		cred.Scope = peBloggerScopeFromRefresh(cred.Scope, scope)
		if resealed, sealErr := d.peSealBloggerCredential(*cred, media.strOr("id", "")); sealErr == nil {
			_ = d.exec(ctx, "UPDATE media SET blogger_credentials_sealed = $1, enc_key_ver = 1, updated_at = NOW() WHERE id = $2",
				resealed, media.strOr("id", ""))
		}
	}
	return cred, nil
}

func peRefreshBloggerToken(ctx context.Context, refreshToken, clientID, clientSecret string) (string, int, string, error) {
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "refresh_token")
	status, body, err := peHTTPRequest(ctx, http.MethodPost, "https://oauth2.googleapis.com/token",
		map[string]string{"content-type": "application/x-www-form-urlencoded"}, form.Encode(), 20*time.Second)
	if err != nil {
		return "", 0, "", err
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   any    `json:"expires_in"`
		Scope       string `json:"scope"`
		ErrorDesc   string `json:"error_description"`
		ErrorField  string `json:"error"`
	}
	_ = json.Unmarshal(body, &parsed)
	if status < 200 || status >= 300 {
		msg := parsed.ErrorDesc
		if msg == "" {
			msg = parsed.ErrorField
		}
		if msg == "" {
			msg = "Refresh gagal (" + itoa(status) + ")"
		}
		return "", 0, "", fmt.Errorf("%s", msg)
	}
	expiresIn := int(peNumFromAny(parsed.ExpiresIn))
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return parsed.AccessToken, expiresIn, parsed.Scope, nil
}

func (d *Deps) peRunBlogger(ctx context.Context, publication, media, document, version bsRow, publicOrigin string, startedAt time.Time, terminal bool) error {
	clientID, clientSecret := d.peLoadGSCPlatformConfig(ctx)
	if clientID == "" || clientSecret == "" {
		reason := "Konfigurasi Google OAuth belum diatur Admin."
		if err := d.peFailAttempt(ctx, publication, "blogger_config_missing", reason, startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("blogger_config_missing", reason)
	}
	if media.strOr("blogger_blog_id", "") == "" {
		reason := "Media Blogger belum terhubung atau belum memilih blog."
		if err := d.peFailAttempt(ctx, publication, "blogger_not_configured", reason, startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("blogger_not_configured", reason)
	}
	credential, credErr := d.peResolveBloggerTarget(ctx, media, clientID, clientSecret)
	if credErr != nil || credential == nil {
		reason := "Kredensial Blogger tidak valid — Perlu Hubungkan Ulang."
		if err := d.peFailAttempt(ctx, publication, "blogger_credential_expired", reason, startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("blogger_credential_expired", reason)
	}

	jobRecord, err := d.peFindOrCreateJob(ctx, publication, "", "", startedAt, nil)
	if err != nil {
		return err
	}
	jobID := jobRecord.strOr("id", "")
	// Idempotency: job sudah completed / punya remote_post_id → publikasi sukses.
	if jobRecord.strOr("status", "") == "completed" || jobRecord.strOr("remote_post_id", "") != "" {
		return d.peMarkPublished(ctx, publication, startedAt)
	}

	_, html := d.peRenderedHTML(version, publicOrigin)
	isDraftMode := publication.strOr("publish_status", "draft") == "draft"
	var labels []string
	// Label Blogger: kategori pilihan mitra lebih dulu, fallback ke kategori
	// media. Blogger memakai label bebas — kategori baru "tampil" seketika.
	if category := strings.TrimSpace(publication.strOr("category_override", "")); category != "" {
		labels = []string{category}
	} else if category := strings.TrimSpace(media.strOr("category", "")); category != "" {
		labels = []string{category}
	}
	postTitle := document.strOr("title", "")

	if err := d.exec(ctx,
		"UPDATE publication_jobs SET status = 'processing', started_at = COALESCE(started_at, NOW()) WHERE id = $1", jobID); err != nil {
		return err
	}
	if err := d.exec(ctx, "UPDATE publication_jobs SET progress = 30 WHERE id = $1", jobID); err != nil {
		return err
	}

	// Gambar artikel: API Blogger tidak punya endpoint unggah media, jadi tiap
	// gambar /uploads dititipkan ke Google Drive mitra lalu src-nya diganti URL
	// gambar Google. Kredensial lama yang belum punya scope Drive tetap terbit
	// seperti sebelumnya (tautan publik Tauthub) plus peringatan agar mitra
	// menyambung ulang.
	snapshot, _ := jobRecord.jsonField("payload_snapshot").(map[string]any)
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	mediaCache := peBloggerMediaCache(snapshot)
	if mediaCache == nil {
		mediaCache = map[string]any{}
	}
	featured := d.peAbsolutizeValue(publication.strOr("featured_image_url", ""), publicOrigin)
	hasInlineImage := strings.Contains(html, "<img")
	driveReady := peBloggerHasDriveScope(credential.Scope)
	// Catatan dikumpulkan lalu ditulis sekali: peRecordWarning menimpa kolom
	// warning_message, jadi beberapa panggilan terpisah akan saling menghapus.
	notes := []string{}
	if !driveReady {
		notes = append(notes, "Blogger belum punya izin Google Drive, gambar artikel masih memakai tautan Tauthub. Hubungkan ulang Blogger agar gambar disimpan ke Drive mitra.")
	}
	folderID := ""
	if driveReady && (featured != "" || hasInlineImage) {
		id, folderErr := d.peBloggerDriveFolder(ctx, credential.AccessToken, credential, media)
		if folderErr != nil {
			notes = append(notes, "Folder Google Drive mitra tidak bisa disiapkan ("+folderErr.Error()+"), gambar memakai tautan Tauthub.")
			driveReady = false
		} else {
			folderID = id
			if credential.DriveFolderID != id {
				credential.DriveFolderID = id
				if resealed, sealErr := d.peSealBloggerCredential(*credential, media.strOr("id", "")); sealErr == nil {
					_ = d.exec(ctx, "UPDATE media SET blogger_credentials_sealed = $1, enc_key_ver = 1, updated_at = NOW() WHERE id = $2",
						resealed, media.strOr("id", ""))
				}
			}
		}
	}
	var imageFailures []string
	bodyHTML := html
	if driveReady {
		if hasInlineImage {
			bodyHTML, imageFailures = d.peMirrorBloggerImages(ctx, credential.AccessToken, folderID, html, postTitle, mediaCache)
		}
		if featured != "" && peBloggerMirrorableURL(publicOrigin, featured) {
			mirrored, verified, mirrorErr := d.peMirrorBloggerImage(ctx, credential.AccessToken, folderID, featured, postTitle, mediaCache)
			if mirrorErr != nil {
				reason := "Gambar utama tidak bisa dititipkan ke Google Drive mitra (" + featured + "): " + mirrorErr.Error()
				d.peSaveBloggerMediaCache(ctx, jobID, snapshot, mediaCache, imageFailures)
				return d.peFailDelivery(ctx, publication, jobID, pePublishErr("blogger_image_upload_failed", reason), startedAt, terminal)
			}
			featured = mirrored
			if !verified {
				notes = append(notes, "Gambar utama sudah tersimpan di Google Drive mitra, tetapi URL gambarnya belum bisa diverifikasi dari server. Periksa tampilan artikel setelah terbit.")
			}
		}
	}
	if featured != "" && (strings.HasPrefix(featured, "http://") || strings.HasPrefix(featured, "https://")) {
		alt := strings.ReplaceAll(postTitle, `"`, "&quot;")
		src := strings.ReplaceAll(featured, `"`, "&quot;")
		bodyHTML = `<img alt="` + alt + `" src="` + src + `" />` + "\n\n" + bodyHTML
	}
	d.peSaveBloggerMediaCache(ctx, jobID, snapshot, mediaCache, imageFailures)
	if len(imageFailures) > 0 {
		notes = append(notes, fmt.Sprintf("%d gambar di badan artikel tidak bisa dititipkan ke Drive: %s", len(imageFailures), strings.Join(imageFailures, ", ")))
	}
	if len(notes) > 0 {
		d.peRecordWarning(ctx, jobID, notes)
	}
	if err := d.exec(ctx, "UPDATE publication_jobs SET progress = 60 WHERE id = $1", jobID); err != nil {
		return err
	}
	result, pubErr := pePublishBloggerPost(ctx, credential.AccessToken, media.strOr("blogger_blog_id", ""),
		postTitle, bodyHTML, labels, isDraftMode)
	if pubErr != nil {
		return d.peFailDelivery(ctx, publication, jobID, pubErr, startedAt, terminal)
	}
	if err := d.exec(ctx, "UPDATE publication_jobs SET progress = 85 WHERE id = $1", jobID); err != nil {
		return err
	}
	if err := d.exec(ctx,
		"UPDATE publication_jobs SET remote_post_id = COALESCE(remote_post_id, $1), remote_url = COALESCE(remote_url, $2), status = 'completed', completed_at = NOW(), progress = 100 WHERE id = $3",
		anyOrNil(result.ID), anyOrNil(result.URL), jobID); err != nil {
		return err
	}
	if err := d.exec(ctx,
		"UPDATE publications SET published_url = COALESCE(published_url, $1), published_at = NOW(), updated_at = NOW() WHERE id = $2",
		anyOrNil(result.URL), publication.strOr("id", "")); err != nil {
		return err
	}
	return d.peMarkPublished(ctx, publication, startedAt)
}

type peBloggerPostResult struct {
	ID  string
	URL string
}

// pePublishBloggerPost: parity publishBloggerPost (posts.insert + publish).
func pePublishBloggerPost(ctx context.Context, accessToken, blogID, title, contentHTML string, labels []string, isDraft bool) (peBloggerPostResult, error) {
	body := map[string]any{
		"kind": "blogger#post", "title": title, "content": contentHTML, "isDraft": isDraft,
	}
	if len(labels) > 0 {
		body["labels"] = labels
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return peBloggerPostResult{}, err
	}
	headers := map[string]string{"authorization": "Bearer " + accessToken, "content-type": "application/json"}
	target := "https://www.googleapis.com/blogger/v3/blogs/" + url.PathEscape(blogID) + "/posts"
	status, resp, err := peHTTPRequest(ctx, http.MethodPost, target, headers, string(raw), 30*time.Second)
	if err != nil {
		return peBloggerPostResult{}, err
	}
	var data struct {
		ID     string `json:"id"`
		URL    string `json:"url"`
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(resp, &data)
	if status < 200 || status >= 300 {
		switch status {
		case http.StatusNotFound:
			return peBloggerPostResult{}, fmt.Errorf("Blog tidak ditemukan atau tidak dapat diakses.")
		case http.StatusUnauthorized:
			return peBloggerPostResult{}, fmt.Errorf("Kredensial Blogger tidak valid — perlu hubungkan ulang.")
		case http.StatusForbidden:
			return peBloggerPostResult{}, fmt.Errorf("Akses ditolak Blogger — periksa izin akun.")
		}
		if data.Error != nil && data.Error.Message != "" {
			return peBloggerPostResult{}, fmt.Errorf("%s", data.Error.Message)
		}
		return peBloggerPostResult{}, fmt.Errorf("Blogger menolak request (%d)", status)
	}
	if isDraft {
		return peBloggerPostResult{ID: data.ID, URL: data.URL}, nil
	}
	if data.Status == "LIVE" {
		return peBloggerPostResult{ID: data.ID, URL: data.URL}, nil
	}
	publishTarget := target + "/" + url.PathEscape(data.ID) + "/publish"
	pubStatus, pubResp, pubErr := peHTTPRequest(ctx, http.MethodPost, publishTarget,
		map[string]string{"authorization": "Bearer " + accessToken}, "", 30*time.Second)
	if pubErr != nil {
		return peBloggerPostResult{}, pubErr
	}
	var pubData struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(pubResp, &pubData)
	if pubStatus < 200 || pubStatus >= 300 {
		if pubData.Error != nil && pubData.Error.Message != "" {
			return peBloggerPostResult{}, fmt.Errorf("%s", pubData.Error.Message)
		}
		return peBloggerPostResult{}, fmt.Errorf("Gagal mempublikasikan post (%d)", pubStatus)
	}
	if pubData.ID != "" {
		data.ID = pubData.ID
	}
	if pubData.URL != "" {
		data.URL = pubData.URL
	}
	return peBloggerPostResult{ID: data.ID, URL: data.URL}, nil
}

// peLoadGSCPlatformConfig: parity loadGscPlatformConfig — DB dulu, fallback env.
func (d *Deps) peLoadGSCPlatformConfig(ctx context.Context) (string, string) {
	row, err := d.queryRow(ctx,
		"SELECT client_id, sealed_client_secret FROM platform_gsc_config WHERE id = 'default' LIMIT 1")
	if err == nil && row != nil {
		clientID := strings.TrimSpace(row.strOr("client_id", ""))
		if clientID != "" {
			secret := ""
			if sealed := row.strOr("sealed_client_secret", ""); sealed != "" {
				if plain, uErr := crypto.Unseal(sealed, d.peCMSSecret()); uErr == nil {
					secret = plain
				}
			}
			if secret == "" {
				secret = strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
			}
			return clientID, secret
		}
	}
	return strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")), strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
}

// ---------- adapter: wordpress / webhook via koneksi CMS ----------

func peSameSite(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return strings.EqualFold(ua.Hostname(), ub.Hostname())
}

func (d *Deps) peRunCMS(ctx context.Context, publication, media, document, version bsRow, publicOrigin string, startedAt time.Time, terminal bool, integrationType string) error {
	mediaPartnerID := media.strOr("partner_id", "")

	var connection bsRow
	if connectionID := publication.strOr("connection_id", ""); connectionID != "" {
		ownerClause := "brand_id = $2"
		ownerValue := publication.strOr("brand_id", "")
		if mediaPartnerID != "" {
			ownerClause = "partner_id = $2"
			ownerValue = mediaPartnerID
		}
		found, err := d.queryRow(ctx,
			"SELECT * FROM cms_connections WHERE id = $1 AND status = 'active' AND "+ownerClause+" LIMIT 1",
			connectionID, ownerValue)
		if err != nil {
			return err
		}
		connection = found
	}

	provider := "wordpress"
	if integrationType == "webhook" {
		provider = "webhook"
	}
	if connection == nil {
		if mediaPartnerID != "" {
			bound, err := d.queryRow(ctx,
				"SELECT * FROM cms_connections WHERE partner_id = $1 AND provider = $2 AND status = 'active' AND website_id = $3 ORDER BY created_at DESC LIMIT 1",
				mediaPartnerID, provider, media.strOr("id", ""))
			if err != nil {
				return err
			}
			connection = bound
			if connection == nil && media.strOr("website_url", "") != "" {
				candidates, err := d.queryRows(ctx,
					"SELECT * FROM cms_connections WHERE partner_id = $1 AND provider = $2 AND status = 'active' ORDER BY created_at DESC",
					mediaPartnerID, provider)
				if err != nil {
					return err
				}
				for _, c := range candidates {
					if peSameSite(c.strOr("base_url", ""), media.strOr("website_url", "")) {
						connection = c
						break
					}
				}
			}
		} else {
			found, err := d.queryRow(ctx,
				"SELECT * FROM cms_connections WHERE brand_id = $1 AND provider = $2 AND status = 'active' ORDER BY created_at DESC LIMIT 1",
				publication.strOr("brand_id", ""), provider)
			if err != nil {
				return err
			}
			connection = found
		}
	}
	if connection == nil {
		hint := ""
		if mediaPartnerID != "" {
			name := media.strOr("website_url", media.strOr("name", "media"))
			hint = " — hubungkan situs " + name + " di dashboard mitra"
		}
		mediaName := media.strOr("name", media.strOr("id", ""))
		reason := "No active " + integrationType + " connection for media " + mediaName + hint
		if err := d.peFailAttempt(ctx, publication, "no_connection", reason, startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("no_connection", reason)
	}

	// Delivery record (publication_jobs) linked ke publikasi + koneksi.
	jobRecord, err := d.peFindOrCreateJob(ctx, publication, publication.strOr("document_id", ""), connection.strOr("id", ""), startedAt, map[string]any{
		"publish_status": publication.strOr("publish_status", "draft"),
		"excerpt":        nil,
		"categories":     []any{},
		"tags":           []any{},
	})
	if err != nil {
		return err
	}
	jobID := jobRecord.strOr("id", "")
	if jobRecord.strOr("status", "") == "completed" {
		return d.peMarkPublished(ctx, publication, startedAt)
	}

	sealed, err := d.queryRow(ctx, "SELECT id, sealed_secret, enc_key_ver FROM cms_connections WHERE id = $1 LIMIT 1", connection.strOr("id", ""))
	if err != nil {
		return err
	}
	if sealed == nil {
		reason := "CMS connection is not active"
		if err := d.peFailAttempt(ctx, publication, "connection_secret_missing", reason, startedAt, terminal); err != nil {
			return err
		}
		return pePublishErr("connection_secret_missing", reason)
	}
	secret, err := d.peConnectionSecret(sealed)
	if err != nil {
		return d.peFailDelivery(ctx, publication, jobID, pePublishErr("connection_secret_missing", err.Error()), startedAt, terminal)
	}

	snapshot, _ := jobRecord.jsonField("payload_snapshot").(map[string]any)
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	defaults := peParseDefaults(media.jsonField("publish_defaults"))
	if featured := strings.TrimSpace(publication.strOr("featured_image_url", "")); featured != "" {
		defaults["featured_image_url"] = featured
	}
	if (integrationType == "wordpress" || integrationType == "emdash") && bsStringifyAny(defaults["featured_image_url"]) == "" {
		// Publikasi tanpa cover membuat artikel terbit tanpa gambar utama tanpa
		// error apa pun. Ambil cover dari kampanye/gambar AI, lalu pakai aset
		// fallback; cover fallback bersifat lunak (gagal unggah ≠ gagal terbit).
		if cover := d.peResolveCoverURL(ctx, publication, media); cover != "" {
			defaults["featured_image_url"] = cover
			_ = d.exec(ctx,
				"UPDATE publications SET featured_image_url = $1, updated_at = NOW() WHERE id = $2 AND (featured_image_url IS NULL OR featured_image_url = '')",
				cover, publication.strOr("id", ""))
		} else {
			defaults["featured_image_url"] = peFallbackCoverURL
			snapshot["cover_soft"] = true
		}
	}

	if err := d.exec(ctx, "UPDATE publication_jobs SET progress = 30 WHERE id = $1", jobID); err != nil {
		return err
	}
	// Kategori yang dipilih mitra saat mengedit draf (category_override)
	// jadi prioritas: artikel masuk kategori tersebut di sistem target,
	// term dibuat otomatis bila belum ada (WordPress).
	preferredCategory := strings.TrimSpace(publication.strOr("category_override", ""))
	if integrationType == "wordpress" {
		if err := d.peResolveWordPressDefaults(ctx, connection, media, document, snapshot, defaults, secret, jobID, preferredCategory); err != nil {
			return d.peFailDelivery(ctx, publication, jobID, err, startedAt, terminal)
		}
	} else {
		if featured := d.peAbsolutizeValue(bsStringifyAny(defaults["featured_image_url"]), publicOrigin); featured != "" {
			snapshot["featured_image_url"] = featured
		}
		if author, ok := defaults["author_id"]; ok {
			snapshot["author_id"] = peNumFromAny(author)
		}
		if fields, ok := defaults["custom_fields"].(map[string]any); ok {
			snapshot["custom_fields"] = fields
		}
	}

	content, html := d.peRenderedHTML(version, publicOrigin)
	if integrationType == "wordpress" {
		html = d.peUploadArticleImages(ctx, connection, document, snapshot, jobID, secret, html)
	}

	var publishErr error
	switch integrationType {
	case "webhook":
		publishErr = d.bsPublishToWebhook(ctx, jobID, connection, document, snapshot, secret, content, html)
	case "emdash":
		publishErr = d.pePublishToEmdash(ctx, jobID, connection.strOr("base_url", ""), secret,
			jobRecord.strOr("remote_post_id", ""), document, html,
			publication.strOr("publish_status", "draft"),
			bsStringifyAny(defaults["featured_image_url"]), preferredCategory, snapshot)
	default:
		publishErr = d.bsPublishToWordpress(ctx, jobID, connection, jobRecord, document, snapshot, secret, html)
	}
	if publishErr != nil {
		return d.peFailDelivery(ctx, publication, jobID, publishErr, startedAt, terminal)
	}
	_ = d.exec(ctx, "UPDATE publication_jobs SET progress = 95 WHERE id = $1", jobID)
	_ = d.exec(ctx, "UPDATE publication_jobs SET completed_at = NOW() WHERE id = $1", jobID)
	_ = d.exec(ctx, "UPDATE publication_jobs SET progress = 100 WHERE id = $1", jobID)
	return d.peMarkPublished(ctx, publication, startedAt)
}

// peResolveWordPressDefaults: resolve kategori/tag + upload featured image
// sekali, simpan hasil di payload_snapshot.resolved agar retry tidak mengulang.
func (d *Deps) peResolveWordPressDefaults(ctx context.Context, connection, media, document bsRow, snapshot, defaults map[string]any, secret, jobID, preferredCategory string) error {
	headers := peWPAuthHeaders(connection.strOr("username", ""), secret)
	baseURL := connection.strOr("base_url", "")
	resolvedSoFar, _ := snapshot["resolved"].(map[string]any)
	resolved := map[string]any{}
	for k, v := range resolvedSoFar {
		resolved[k] = v
	}
	hasResolved := snapshot["resolved"] != nil
	needsFeatured := bsStringifyAny(defaults["featured_image_url"]) != "" && peNumFromAny(resolved["featured_media"]) <= 0
	hasMediaCategory := strings.TrimSpace(media.strOr("category", "")) != ""
	// Snapshot lama bisa sudah ada TAPI belum memuat categories/featured_media
	// (mis. percobaan pertama saat cover belum siap). Tanpa syarat tambahan ini,
	// penyelesaian istilah dan gambar utama dilewati selamanya sehingga artikel
	// terbit "Uncategorized" dan tanpa gambar utama.
	resolvedCategoryCount := 0
	if arr, isArr := resolved["categories"].([]any); isArr {
		resolvedCategoryCount = len(arr)
	}
	needsCategories := (hasMediaCategory || len(peNameList(defaults["categories"])) > 0 || preferredCategory != "") && resolvedCategoryCount == 0
	shouldResolve := (!hasResolved && len(defaults) > 0) || needsFeatured || needsCategories || (!hasResolved && (hasMediaCategory || preferredCategory != ""))
	if !shouldResolve {
		if author, ok := defaults["author_id"]; ok {
			snapshot["author_id"] = peNumFromAny(author)
		}
		if fields, ok := defaults["custom_fields"].(map[string]any); ok {
			snapshot["custom_fields"] = fields
		}
		return nil
	}
	create := bsStringifyAny(defaults["term_action"]) != "search_only"
	categoryNames := peNameList(defaults["categories"])
	if len(categoryNames) == 0 && hasMediaCategory {
		categoryNames = append(categoryNames, strings.TrimSpace(media.strOr("category", "")))
	}
	// Kategori pilihan mitra (dari pengeditan draf) diutamakan di urutan depan.
	if preferredCategory != "" {
		existing := categoryNames
		categoryNames = []string{preferredCategory}
		for _, n := range existing {
			if n != preferredCategory {
				categoryNames = append(categoryNames, n)
			}
		}
	}
	tagNames := peNameList(defaults["tags"])
	if len(categoryNames) > 0 && (!hasResolved || needsCategories) {
		ids := peIntListAny(peResolveTermIDs(ctx, baseURL, headers, categoryNames, "categories", create))
		if len(ids) > 0 {
			resolved["categories"] = ids
		} else if skipCategories := false; !skipCategories {
			d.peRecordWarning(ctx, jobID, []string{
				"Kategori (" + strings.Join(categoryNames, ", ") + ") tidak berhasil dipetakan di WordPress, artikel bisa terbit tanpa kategori.",
			})
		}
	}
	if !hasResolved && len(tagNames) > 0 {
		resolved["tags"] = peIntListAny(peResolveTermIDs(ctx, baseURL, headers, tagNames, "tags", create))
	}
	_ = d.exec(ctx, "UPDATE publication_jobs SET progress = 45 WHERE id = $1", jobID)
	if needsFeatured {
		imageURL := bsStringifyAny(defaults["featured_image_url"])
		mediaID := d.peUploadFeaturedImage(ctx, baseURL, headers, imageURL, document.strOr("title", ""))
		if mediaID > 0 {
			resolved["featured_media"] = mediaID
		} else if coverSoft, _ := snapshot["cover_soft"].(bool); coverSoft {
			// Cover fallback (artikel memang belum punya gambar): kegagalan
			// mengunggah tidak boleh menahan terbitnya artikel.
			d.peRecordWarning(ctx, jobID, []string{
				"Gambar cover fallback tidak bisa diunggah ke media mitra (" + imageURL + "), artikel terbit tanpa gambar utama.",
			})
		} else {
			reason := "Gambar cover artikel tidak bisa diunggah ke WordPress (" + imageURL + "). " +
				"Publikasi dihentikan sebelum terbit — periksa file/URL cover lalu coba lagi."
			_ = d.exec(ctx, "UPDATE publication_jobs SET error_message = $1 WHERE id = $2", truncateString(reason, 500), jobID)
			return pePublishErr("featured_image_upload_failed", reason)
		}
		_ = d.exec(ctx, "UPDATE publication_jobs SET progress = 60 WHERE id = $1", jobID)
	}
	if bsStringifyAny(defaults["featured_image_url"]) != "" && peNumFromAny(resolved["featured_media"]) <= 0 {
		d.peRecordWarning(ctx, jobID, []string{
			"Gambar utama tidak terpasang di WordPress (featured_media kosong), artikel terbit tanpa thumbnail.",
		})
	}
	snapshot["resolved"] = resolved
	if err := d.exec(ctx, "UPDATE publication_jobs SET payload_snapshot = $1::jsonb WHERE id = $2", marshalJSON(snapshot), jobID); err != nil {
		return err
	}
	if author, ok := defaults["author_id"]; ok {
		snapshot["author_id"] = peNumFromAny(author)
	}
	if fields, ok := defaults["custom_fields"].(map[string]any); ok {
		snapshot["custom_fields"] = fields
	}
	return nil
}

func peWPAuthHeaders(username, secret string) map[string]string {
	basic := base64.StdEncoding.EncodeToString([]byte(username + ":" + secret))
	return map[string]string{"Content-Type": "application/json", "Authorization": "Basic " + basic}
}

// peFetchWPJSON: request JSON ke WordPress; (status, text).
func peFetchWPJSON(ctx context.Context, baseURL string, headers map[string]string, path, method, body string) (int, string, error) {
	return bsHTTPDo(ctx, method, bsWPURL(baseURL, path), headers, body, 30*time.Second)
}

func peSlugifyTerm(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
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

// peFindTermID: cari term WordPress (exact slug dulu, lalu exact name).
func peFindTermID(ctx context.Context, baseURL string, headers map[string]string, taxonomy, name string) (int, bool) {
	status, text, err := peFetchWPJSON(ctx, baseURL, headers,
		"wp/v2/"+taxonomy+"?search="+url.QueryEscape(name)+"&per_page=100", http.MethodGet, "")
	if err != nil || status < 200 || status >= 300 {
		return 0, false
	}
	var terms []map[string]any
	if json.Unmarshal([]byte(text), &terms) != nil {
		return 0, false
	}
	wantedSlug := peSlugifyTerm(name)
	wantedName := strings.ToLower(strings.TrimSpace(name))
	for _, term := range terms {
		if peSlugifyTerm(bsStringifyAny(term["slug"])) == wantedSlug {
			if id := int(peNumFromAny(term["id"])); id > 0 {
				return id, true
			}
		}
	}
	for _, term := range terms {
		if strings.ToLower(strings.TrimSpace(bsStringifyAny(term["name"]))) == wantedName {
			if id := int(peNumFromAny(term["id"])); id > 0 {
				return id, true
			}
		}
	}
	return 0, false
}

// peResolveTermIDs: resolve nama term → ID (search, opsional create). Error
// per-term tidak menggagalkan publikasi (dicatat & dilewati).
//
// Nama yang berulang dalam satu panggilan di-resolve sekali: request
// pencariannya identik byte-per-byte (`?search=<nama>`), jadi mengulanginya
// per kemunculan tidak bisa menghasilkan id lain — kemunculan berikutnya
// cukup menyalin hasil kemunculan pertama. Penggabungan semua nama ke satu
// GET per taxonomy (match nama lokal tanpa ?search=) sengaja tidak dilakukan:
// pemilihan hasil bergantung urutan balikan server (slug exact dulu, lalu
// name exact, ambil pertama), dan urutan kandidat `?search=` tidak dijamin
// sama dengan urutan daftar penuh — apalagi pada mitra yang hanya meniru
// wp/v2. Batas rate mitra juga belum terdokumentasi di mana pun.
func peResolveTermIDs(ctx context.Context, baseURL string, headers map[string]string, names []string, taxonomy string, create bool) []int {
	return pePlanTermResolution(names, func(name string) int {
		return peResolveOneTerm(ctx, baseURL, headers, taxonomy, name, create)
	})
}

// peResolveOneTerm: resolve satu nama — search, lalu POST create bila
// diizinkan. 0 berarti tidak ditemukan/gagal dibuat (dilewati tanpa error).
func peResolveOneTerm(ctx context.Context, baseURL string, headers map[string]string, taxonomy, name string, create bool) int {
	if id, found := peFindTermID(ctx, baseURL, headers, taxonomy, name); found {
		return id
	}
	if !create {
		slog.Warn("[brandflow] term WordPress belum ditemukan — dilewati", "taxonomy", taxonomy, "name", name)
		return 0
	}
	raw, err := json.Marshal(map[string]any{"name": name})
	if err != nil {
		return 0
	}
	status, text, err := peFetchWPJSON(ctx, baseURL, headers, "wp/v2/"+taxonomy, http.MethodPost, string(raw))
	if err != nil || status < 200 || status >= 300 {
		slog.Warn("[brandflow] create term WordPress gagal", "taxonomy", taxonomy, "name", name)
		return 0
	}
	var created map[string]any
	if json.Unmarshal([]byte(text), &created) == nil {
		if id := int(peNumFromAny(created["id"])); id > 0 {
			return id
		}
	}
	return 0
}

// peTermDedupeSlots: untuk tiap posisi nama, indeks kemunculan PERTAMA nama
// yang sama persis. Pencarian mitra memakai nama mentah, jadi varian huruf
// besar/kecil atau spasi tetap query berbeda dan tidak boleh digabung.
func peTermDedupeSlots(names []string) []int {
	slots := make([]int, len(names))
	seen := make(map[string]int, len(names))
	for i, name := range names {
		if first, dup := seen[name]; dup {
			slots[i] = first
			continue
		}
		slots[i] = i
		seen[name] = i
	}
	return slots
}

// pePlanTermResolution: resolve tiap nama unik sekali lewat resolve, lalu
// salin hasilnya (id maupun 0 = dilewati) ke kemunculan ulangnya. Urutan
// keluaran mengikuti urutan kemunculan nama, sama seperti resolusi per-term
// penuh.
func pePlanTermResolution(names []string, resolve func(name string) int) []int {
	slots := peTermDedupeSlots(names)
	perName := make([]int, len(names))
	for i, name := range names {
		if slots[i] != i {
			perName[i] = perName[slots[i]]
			continue
		}
		perName[i] = resolve(name)
	}
	out := make([]int, 0, len(names))
	for _, id := range perName {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}

var peImageMIME = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".webp": "image/webp", ".gif": "image/gif", ".avif": "image/avif",
}

// peReadLocalUpload: baca berkas /uploads/<name> dari UPLOAD_DIR / legacy dir.
func (d *Deps) peReadLocalUpload(imagePath string) ([]byte, string, string, bool) {
	name := filepath.Base(imagePath)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return nil, "", "", false
	}
	mime, ok := peImageMIME[strings.ToLower(filepath.Ext(name))]
	if !ok {
		return nil, "", "", false
	}
	for _, dir := range []string{d.uploadDir(), peLegacyUploadDir()} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return data, mime, name, true
		}
	}
	return nil, "", "", false
}

func peLegacyUploadDir() string {
	if v := strings.TrimSpace(os.Getenv("LEGACY_UPLOAD_DIR")); v != "" {
		return v
	}
	return filepath.Join("public", "uploads")
}

// peMaxImageBytes: batas unduh berkas gambar dari HTTP. peHTTPRequestBytes
// memotong di 1 MiB, terlalu kecil untuk foto 1920x1080.
const peMaxImageBytes = 12 << 20

// peMaxInlineUploads: batas jumlah gambar badan artikel yang diunggah per
// publikasi supaya satu artikel tidak menahan job terlalu lama.
const peMaxInlineUploads = 20

// peFallbackCoverURL: aset statis publik aplikasi, dipakai saat artikel tidak
// punya cover sama sekali.
const peFallbackCoverURL = "/images/article-fallback.png"

// peImageFile: berkas gambar siap dikirim ke media library CMS mitra.
type peImageFile struct {
	Data        []byte
	ContentType string
	Filename    string
}

// peFetchUploadBytes: ambil isi berkas gambar yang dipakai artikel.
// Urutan resolusi: /uploads/* dibaca dari disk (API & scheduler berbagi
// UPLOAD_DIR), URL absolut ber-path /uploads/* juga dicoba dari disk dulu,
// path relatif lain lewat origin publik, terakhir unduh via HTTP.
func (d *Deps) peFetchUploadBytes(ctx context.Context, imageURL string) (peImageFile, error) {
	raw := strings.TrimSpace(imageURL)
	if raw == "" {
		return peImageFile{}, errors.New("URL gambar kosong")
	}
	if strings.HasPrefix(raw, "/uploads/") {
		data, mime, name, ok := d.peReadLocalUpload(raw)
		if !ok {
			return peImageFile{}, fmt.Errorf("berkas %s tidak ditemukan di server", raw)
		}
		return peImageFile{Data: data, ContentType: mime, Filename: name}, nil
	}
	target := raw
	if strings.HasPrefix(target, "/") {
		target = strings.TrimRight(d.pePublicOrigin(), "/") + target
	}
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return peImageFile{}, fmt.Errorf("URL gambar tidak valid: %s", raw)
	}
	name := filepath.Base(parsed.Path)
	if decoded, decErr := url.PathUnescape(name); decErr == nil {
		name = decoded
	}
	if name == "" || name == "." || name == "/" {
		name = "image"
	}
	if strings.HasPrefix(parsed.Path, "/uploads/") {
		if data, mime, localName, ok := d.peReadLocalUpload(parsed.Path); ok {
			return peImageFile{Data: data, ContentType: mime, Filename: localName}, nil
		}
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return peImageFile{}, fmt.Errorf("gambar %s hanya tersedia di host lokal", raw)
	}
	status, body, err := peFetchRemoteBytes(ctx, parsed.String(), peMaxImageBytes)
	if err != nil {
		return peImageFile{}, fmt.Errorf("unduh gambar %s gagal: %w", raw, err)
	}
	if status < 200 || status >= 300 {
		return peImageFile{}, fmt.Errorf("unduh gambar %s ditolak (HTTP %d)", raw, status)
	}
	if len(body) == 0 {
		return peImageFile{}, fmt.Errorf("gambar %s kosong", raw)
	}
	return peImageFile{Data: body, ContentType: peImageContentType(name), Filename: name}, nil
}

// peFetchRemoteBytes: unduh berkas biner dengan batas ukuran khusus gambar.
func peFetchRemoteBytes(ctx context.Context, target string, maxBytes int64) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := security.SafeClient(30 * time.Second).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if readErr != nil {
		return resp.StatusCode, nil, readErr
	}
	if int64(len(data)) > maxBytes {
		return resp.StatusCode, nil, fmt.Errorf("berkas melebihi batas %d byte", maxBytes)
	}
	return resp.StatusCode, data, nil
}

// peImageContentType: MIME dari ekstensi berkas. Jangan hardcode image/jpeg —
// gambar artikel kita WebP, dan WordPress menolak mime yang tidak cocok.
func peImageContentType(name string) string {
	if mime, ok := peImageMIME[strings.ToLower(filepath.Ext(name))]; ok {
		return mime
	}
	return "image/jpeg"
}

func peSafeFilename(name string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
	if safe == "" {
		return "image.jpg"
	}
	return safe
}

// peWordPressUploadMedia: POST berkas ke wp/v2/media; balikan id + source_url.
func (d *Deps) peWordPressUploadMedia(ctx context.Context, baseURL string, headers map[string]string, file peImageFile, friendlyName string) (int, string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	filename := file.Filename
	if trimmed := strings.TrimSpace(friendlyName); trimmed != "" {
		slug := peSlugifyTerm(trimmed)
		if slug == "" {
			slug = "featured-image"
		}
		if len(slug) > 80 {
			slug = slug[:80]
		}
		filename = slug + ext
	}
	safeName := peSafeFilename(filename)
	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = peImageContentType(safeName)
	}
	uploadHeaders := map[string]string{
		"Authorization":       headers["Authorization"],
		"Content-Disposition": `attachment; filename="` + safeName + `"`,
		"Content-Type":        contentType,
	}
	status, text, err := bsHTTPDoBytes(ctx, http.MethodPost, bsWPURL(baseURL, "wp/v2/media"), uploadHeaders, file.Data, 30*time.Second)
	if err != nil {
		return 0, "", err
	}
	if status < 200 || status >= 300 {
		return 0, "", fmt.Errorf("WordPress media menolak unggahan (HTTP %d): %s", status, truncateString(text, 200))
	}
	var created map[string]any
	if json.Unmarshal([]byte(text), &created) != nil {
		return 0, "", errors.New("respons media WordPress tidak valid")
	}
	id := int(peNumFromAny(created["id"]))
	if id <= 0 {
		return 0, "", errors.New("respons media WordPress tanpa id")
	}
	return id, bsStringifyAny(created["source_url"]), nil
}

// peUploadMediaHook: titik injeksi untuk tes — menggantikan unggahan nyata ke
// media library CMS mitra. Nil berlaku = pakai peWordPressUploadMedia.
var peUploadMediaHook func(ctx context.Context, d *Deps, baseURL string, headers map[string]string, file peImageFile, friendlyName string) (int, string, error)

// peUploadMedia: unggah satu berkas gambar ke media library CMS mitra.
func (d *Deps) peUploadMedia(ctx context.Context, baseURL string, headers map[string]string, file peImageFile, friendlyName string) (int, string, error) {
	if peUploadMediaHook != nil {
		return peUploadMediaHook(ctx, d, baseURL, headers, file, friendlyName)
	}
	return d.peWordPressUploadMedia(ctx, baseURL, headers, file, friendlyName)
}

// peUploadFeaturedImage: parity uploadFeaturedImage — balikan 0 (skip) pada
// kegagalan supaya masalah thumbnail tidak menggagalkan publikasi. Namun
// pemanggil memperlakukan 0 sebagai kegagalan bila artikel punya cover.
func (d *Deps) peUploadFeaturedImage(ctx context.Context, baseURL string, headers map[string]string, imageURL, friendlyName string) int {
	file, err := d.peFetchUploadBytes(ctx, imageURL)
	if err != nil {
		slog.Warn("[brandflow] cover tidak bisa dibaca untuk diunggah", "url", imageURL, "err", err)
		return 0
	}
	id, _, err := d.peUploadMedia(ctx, baseURL, headers, file, friendlyName)
	if err != nil {
		slog.Warn("[brandflow] upload featured image gagal", "url", imageURL, "err", err)
		return 0
	}
	return id
}

// peUploadInlineImages: unggah tiap gambar /uploads di badan artikel ke media
// library CMS mitra lalu ganti src-nya. Tanpa ini gambar tetap hotlink ke
// /uploads kita yang butuh cookie sesi, jadi pengunjung mitra selalu dapat 401.
// cache (dari payload_snapshot.resolved.inline_media) dipakai agar retry tidak
// mengunggah ulang. Balikan HTML baru, cache terbaru, dan daftar kegagalan.
func (d *Deps) peUploadInlineImages(ctx context.Context, baseURL string, headers map[string]string, html string, cache map[string]any, friendlyName string) (string, map[string]any, []string) {
	updated := cache
	if updated == nil {
		updated = map[string]any{}
	}
	uploaded := 0
	rewrite := func(tag string) (string, bool, string) {
		rewritten, didUpload, failure := d.peRewriteImgTag(ctx, baseURL, headers, tag, updated, friendlyName, uploaded)
		if didUpload {
			uploaded++
		}
		return rewritten, didUpload, failure
	}
	rewritten, failures := peRewriteImages(html, rewrite)
	return rewritten, updated, failures
}

// peRewriteImages: jalankan rewriteTag untuk setiap tag <img> di HTML; tag yang
// tidak dikenali dibiarkan apa adanya. Balikan HTML baru + daftar kegagalan.
// Penelusuran tag dipakai bersama jalur WordPress dan EmDash.
func peRewriteImages(html string, rewriteTag func(tag string) (string, bool, string)) (string, []string) {
	if html == "" || !strings.Contains(html, "<img") {
		return html, nil
	}
	failures := []string{}
	var b strings.Builder
	i := 0
	for i < len(html) {
		idx := peIndexFold(html[i:], "<img")
		if idx < 0 {
			b.WriteString(html[i:])
			break
		}
		start := i + idx
		b.WriteString(html[i:start])
		end := peTagEnd(html, start)
		if end < 0 {
			b.WriteString(html[start:])
			break
		}
		rewritten, _, failure := rewriteTag(html[start:end])
		if failure != "" {
			failures = append(failures, failure)
		}
		b.WriteString(rewritten)
		i = end
	}
	return b.String(), failures
}

// peUploadArticleImages: unggah gambar badan artikel ke media library mitra lalu
// simpan cache + peringatan di payload_snapshot agar retry tidak mengunggah ulang.
func (d *Deps) peUploadArticleImages(ctx context.Context, connection, document bsRow, snapshot map[string]any, jobID, secret, html string) string {
	if !strings.Contains(html, "<img") {
		return html
	}
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	headers := peWPAuthHeaders(connection.strOr("username", ""), secret)
	rewritten, cache, failures := d.peUploadInlineImages(ctx, connection.strOr("base_url", ""), headers,
		html, peInlineMediaCache(snapshot), document.strOr("title", ""))
	if len(cache) == 0 && len(failures) == 0 {
		return rewritten
	}
	resolved, _ := snapshot["resolved"].(map[string]any)
	if resolved == nil {
		resolved = map[string]any{}
	}
	if len(cache) > 0 {
		resolved["inline_media"] = cache
	}
	if len(failures) > 0 {
		resolved["inline_upload_failed"] = failures
	}
	snapshot["resolved"] = resolved
	if err := d.exec(ctx, "UPDATE publication_jobs SET payload_snapshot = $1::jsonb WHERE id = $2",
		marshalJSON(snapshot), jobID); err != nil {
		slog.Warn("[brandflow] gagal menyimpan cache gambar publikasi", "job", jobID, "err", err)
	}
	if len(failures) > 0 {
		d.peRecordWarning(ctx, jobID, []string{
			fmt.Sprintf("%d gambar artikel tidak bisa diunggah ke media mitra: %s", len(failures), strings.Join(failures, ", ")),
		})
	}
	return rewritten
}

// peInlineImageName: nama berkas gambar badan artikel — judul artikel plus
// potongan hash URL asal, supaya tiap gambar unik di media library mitra (nama
// UUID asli tidak informatif, sedangkan judul saja membuat semua berkas sama).
func peInlineImageName(title, src string) string {
	// padanan "%08x": heksadesimal huruf kecil, lebar minimal 8 dengan nol di
	// depan (fnv32 = uint32, jadi tidak pernah lebih dari 8 digit).
	suffix := strconv.FormatUint(uint64(fnv32(src)), 16)
	if len(suffix) < 8 {
		suffix = "00000000"[:8-len(suffix)] + suffix
	}
	slug := peSlugifyTerm(title)
	if len(slug) > 60 {
		slug = slug[:60]
	}
	if slug == "" {
		return "gambar-artikel-" + suffix
	}
	return slug + "-" + suffix
}

// fnv32: hash FNV-1a 32-bit untuk penamaan berkas yang stabil antar retry.
func fnv32(value string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(value); i++ {
		hash ^= uint32(value[i])
		hash *= 16777619
	}
	return hash
}

// peRewriteImgTag: ganti src satu tag <img> bila menunjuk berkas /uploads kita.
func (d *Deps) peRewriteImgTag(ctx context.Context, baseURL string, headers map[string]string, tag string, cache map[string]any, friendlyName string, uploadedSoFar int) (string, bool, string) {
	value, vStart, vEnd, _ := peAttrValue(tag, "src")
	if value == "" || !peIsUploadImageRef(value) {
		return tag, false, ""
	}
	if entry, ok := cache[value].(map[string]any); ok {
		if url := bsStringifyAny(entry["url"]); url != "" {
			rewritten := peReplaceAttrValue(tag, vStart, vEnd, url)
			return peAddImageClass(rewritten, int(peNumFromAny(entry["id"]))), false, ""
		}
	}
	if uploadedSoFar >= peMaxInlineUploads {
		return tag, false, ""
	}
	file, err := d.peFetchUploadBytes(ctx, value)
	if err != nil {
		slog.Warn("[brandflow] gambar artikel tidak bisa dibaca", "url", value, "err", err)
		return tag, false, value + " (" + err.Error() + ")"
	}
	id, sourceURL, err := d.peUploadMedia(ctx, baseURL, headers, file, peInlineImageName(friendlyName, value))
	if err != nil {
		slog.Warn("[brandflow] upload gambar artikel gagal", "url", value, "err", err)
		return tag, false, value + " (" + err.Error() + ")"
	}
	if sourceURL == "" {
		sourceURL = value
	}
	cache[value] = map[string]any{"id": id, "url": sourceURL}
	rewritten := peAddImageClass(peReplaceAttrValue(tag, vStart, vEnd, sourceURL), id)
	return rewritten, true, ""
}

// peIsUploadImageRef: apakah src menunjuk berkas gambar milik /uploads kita.
func peIsUploadImageRef(value string) bool {
	path := value
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		path = parsed.Path
	}
	if !strings.HasPrefix(path, "/uploads/") {
		return false
	}
	_, ok := peImageMIME[strings.ToLower(filepath.Ext(path))]
	return ok
}

// peIndexFold: strings.Index case-insensitive ASCII tanpa menyalin haystack.
// Versi lama memanggil strings.ToLower(haystack) — salinan seluruh sisa dokumen
// per panggilan, dan peRewriteImages memanggilnya per tag <img> di dokumen
// ratusan KB.
//
// Beda perilaku dengan versi ToLower hanya untuk rune non-ASCII yang bentuk
// lowercase-nya satu karakter ASCII (mis. U+0130/U+212A): untuk needle "<img"
// tidak ada rune seperti itu. Pada kasus ekstrem itu versi lama sendiri sudah
// mengembalikan indeks yang melenceng dari string asli, karena ToLower bisa
// mengubah panjang byte.
func peIndexFold(haystack, needle string) int {
	if needle == "" {
		return 0
	}
	first := needle[0]
	low, up := first, first
	if first >= 'A' && first <= 'Z' {
		low += 'a' - 'A'
	} else if first >= 'a' && first <= 'z' {
		up -= 'a' - 'A'
	}
	rest := needle[1:]
	for pos := 0; pos+len(needle) <= len(haystack); {
		if asciiEqByte(haystack[pos], first) && asciiEqFold(haystack[pos+1:pos+len(needle)], rest) {
			return pos
		}
		// Kandidat berikutnya: kemunculan terdekat varian huruf pertama
		// (IndexByte dipercepat SIMD, jauh lebih murah daripada membandingkan
		// tiap posisi).
		next := -1
		if j := strings.IndexByte(haystack[pos+1:], low); j >= 0 {
			next = pos + 1 + j
		}
		if up != low {
			if j := strings.IndexByte(haystack[pos+1:], up); j >= 0 && (next < 0 || pos+1+j < next) {
				next = pos + 1 + j
			}
		}
		if next < 0 || next+len(needle) > len(haystack) {
			return -1
		}
		pos = next
	}
	return -1
}

func asciiEqByte(a, b byte) bool {
	if a == b {
		return true
	}
	if a >= 'A' && a <= 'Z' {
		a += 'a' - 'A'
	}
	if b >= 'A' && b <= 'Z' {
		b += 'a' - 'A'
	}
	return a == b
}

func asciiEqFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if !asciiEqByte(a[i], b[i]) {
			return false
		}
	}
	return true
}

// peTagEnd: indeks tepat setelah '>' penutup tag, memperhitungkan nilai
// atribut yang dikutip. -1 bila tag tidak pernah ditutup.
func peTagEnd(html string, start int) int {
	quote := byte(0)
	for i := start; i < len(html); i++ {
		ch := html[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '>' {
			return i + 1
		}
	}
	return -1
}

// peAttrValue: nilai atribut di dalam satu tag; balikan nilai + rentang indeks
// nilai di dalam tag + karakter kutipnya. Kosong bila atribut tidak ada.
func peAttrValue(tag, name string) (string, int, int, byte) {
	lower := strings.ToLower(tag)
	needle := strings.ToLower(name) + "="
	search := 0
	for {
		idx := strings.Index(lower[search:], needle)
		if idx < 0 {
			return "", 0, 0, 0
		}
		pos := search + idx
		// Hindari kecocokan di tengah nama atribut lain (mis. data-src=).
		if pos > 0 {
			prev := tag[pos-1]
			if !(prev == ' ' || prev == '\t' || prev == '\n' || prev == '\r' || prev == '"') {
				search = pos + len(needle)
				continue
			}
		}
		valueStart := pos + len(needle)
		if valueStart >= len(tag) {
			return "", 0, 0, 0
		}
		if tag[valueStart] == '"' || tag[valueStart] == '\'' {
			quote := tag[valueStart]
			valueStart++
			rel := strings.IndexByte(tag[valueStart:], quote)
			if rel < 0 {
				return "", 0, 0, 0
			}
			return tag[valueStart : valueStart+rel], valueStart, valueStart + rel, quote
		}
		end := valueStart
		for end < len(tag) && tag[end] != ' ' && tag[end] != '>' && tag[end] != '\t' {
			end++
		}
		return tag[valueStart:end], valueStart, end, 0
	}
}

// peReplaceAttrValue: ganti nilai atribut pada rentang yang dikembalikan
// peAttrValue.
func peReplaceAttrValue(tag string, start, end int, value string) string {
	return tag[:start] + value + tag[end:]
}

// peAddImageClass: tambahkan kelas wp-image-<id> supaya WordPress mengenali
// lampiran gambar; kelas lama tidak diubah.
func peAddImageClass(tag string, id int) string {
	if id <= 0 {
		return tag
	}
	class := "wp-image-" + itoa(id)
	if _, cStart, cEnd, quote := peAttrValue(tag, "class"); cStart != 0 && quote != 0 {
		if strings.Contains(tag[cStart:cEnd], class) {
			return tag
		}
		return tag[:cStart] + tag[cStart:cEnd] + " " + class + tag[cEnd:]
	}
	end := peTagEnd(tag, 0)
	if end <= 0 {
		return tag
	}
	insertAt := end - 1
	if insertAt > 0 && tag[insertAt-1] == '/' {
		insertAt--
	}
	return tag[:insertAt] + ` class="` + class + `"` + tag[insertAt:]
}

// peInlineMediaCache: cache hasil unggah gambar inline dari payload_snapshot.
func peInlineMediaCache(snapshot map[string]any) map[string]any {
	resolved, _ := snapshot["resolved"].(map[string]any)
	if resolved == nil {
		return nil
	}
	if raw, ok := resolved["inline_media"].(map[string]any); ok {
		return raw
	}
	return nil
}

// peResolveCoverURL: cari gambar utama artikel saat publications.featured_image_url
// kosong — kolom taut_campaign_media lalu gambar AI cover terbaru di
// article_images. Balikan "" bila artikel memang belum punya gambar.
func (d *Deps) peResolveCoverURL(ctx context.Context, publication, media bsRow) string {
	campaignID := publication.strOr("campaign_id", "")
	mediaID := publication.strOr("media_id", "")
	if campaignID == "" || mediaID == "" {
		return ""
	}
	// Satu media bisa punya beberapa artikel, jadi utamakan slot yang artikelnya
	// SAMA dengan publikasi ini (dicocokkan lewat document_id). Tanpa ini, empat
	// artikel satu domain bisa saling meminjam cover.
	if documentID := publication.strOr("document_id", ""); documentID != "" {
		own, ownErr := d.queryRow(ctx,
			"SELECT featured_image_url FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND article_document_id = $3 AND featured_image_url IS NOT NULL AND featured_image_url <> '' LIMIT 1",
			campaignID, mediaID, documentID)
		if ownErr == nil && own != nil {
			if v := strings.TrimSpace(own.strOr("featured_image_url", "")); v != "" {
				return v
			}
		}
	}
	row, err := d.queryRow(ctx,
		"SELECT featured_image_url FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND featured_image_url IS NOT NULL AND featured_image_url <> '' LIMIT 1",
		campaignID, mediaID)
	if err == nil && row != nil {
		if v := strings.TrimSpace(row.strOr("featured_image_url", "")); v != "" {
			return v
		}
	}
	img, err := d.queryRow(ctx,
		"SELECT url FROM article_images WHERE campaign_id = $1 AND media_id = $2 AND role = 'cover' ORDER BY created_at DESC LIMIT 1",
		campaignID, mediaID)
	if err == nil && img != nil {
		if v := strings.TrimSpace(img.strOr("url", "")); v != "" {
			return v
		}
	}
	return ""
}

// peRecordWarning: catat peringatan non-fatal (mis. gambar gagal diunggah) pada
// job supaya tidak hilang diam-diam.
func (d *Deps) peRecordWarning(ctx context.Context, jobID string, lines []string) {
	if jobID == "" || len(lines) == 0 {
		return
	}
	message := truncateString(strings.Join(lines, " | "), 500)
	if err := d.exec(ctx, "UPDATE publication_jobs SET warning_message = $1 WHERE id = $2", message, jobID); err != nil {
		slog.Warn("[brandflow] gagal mencatat warning publikasi", "job", jobID, "err", err)
	}
}

// ---------- job record & kesuksesan ----------

// peFindOrCreateJob: cari publication_jobs terkait publikasi (atau
// document+connection), buat bila belum ada. snapshot non-nil hanya dipakai
// saat membuat baris baru.
func (d *Deps) peFindOrCreateJob(ctx context.Context, publication bsRow, documentID, connectionID string, startedAt time.Time, snapshot map[string]any) (bsRow, error) {
	pubID := publication.strOr("id", "")
	job, err := d.queryRow(ctx, "SELECT * FROM publication_jobs WHERE publication_id = $1 LIMIT 1", pubID)
	if err != nil {
		return nil, err
	}
	if job == nil && documentID != "" && connectionID != "" {
		job, err = d.queryRow(ctx,
			"SELECT * FROM publication_jobs WHERE document_id = $1 AND connection_id = $2 LIMIT 1", documentID, connectionID)
		if err != nil {
			return nil, err
		}
		if job != nil && job.strOr("publication_id", "") == "" {
			if err := d.exec(ctx, "UPDATE publication_jobs SET publication_id = $1 WHERE id = $2", pubID, job.strOr("id", "")); err != nil {
				return nil, err
			}
			job["publication_id"] = pubID
		}
	}
	if job != nil {
		return job, nil
	}

	pjID := newUUID()
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	publishStatus := publication.strOr("publish_status", "draft")
	if err := d.exec(ctx, `
		INSERT INTO publication_jobs (id, brand_id, document_id, connection_id, publication_id, status, publish_status, payload_snapshot, started_at, created_by)
		VALUES ($1, $2, $3, $4, $5, 'queued', $6, $7::jsonb, $8, $9)`,
		pjID, publication.strOr("brand_id", ""), publication.strOr("document_id", ""),
		nullIfEmpty(connectionID), pubID, publishStatus, marshalJSON(snapshot), startedAt,
		publication.strOr("created_by", "")); err != nil {
		return nil, err
	}
	return bsRow{
		"id": pjID, "status": "queued", "publication_id": pubID,
		"payload_snapshot": marshalJSON(snapshot), "remote_post_id": nil, "remote_url": nil,
		"connection_id": nullIfEmpty(connectionID),
	}, nil
}

// peMarkPublished: selesai sukses — publications.published_at + transisi
// published + attempt + finance.
func (d *Deps) peMarkPublished(ctx context.Context, publication bsRow, startedAt time.Time) error {
	pubID := publication.strOr("id", "")
	if err := d.exec(ctx,
		"UPDATE publications SET published_at = NOW(), updated_at = NOW() WHERE id = $1", pubID); err != nil {
		return err
	}
	if err := d.transitionPublication(ctx, pubID, "published"); err != nil {
		return err
	}
	if err := d.peRecordAttempt(ctx, pubID, "published", nil, nil, startedAt); err != nil {
		return err
	}
	media, err := d.queryRow(ctx, "SELECT * FROM media WHERE id = $1 LIMIT 1", publication.strOr("media_id", ""))
	if err != nil {
		return err
	}
	if media != nil {
		d.peRecordFinance(ctx, publication, media)
		// Skor kesehatan ("Kualitas") media ikut disegarkan begitu ada publikasi
		// yang benar-benar terbit, supaya angkanya tidak menunggu jendela refresh
		// 14 hari milik scheduler. Best-effort: kegagalan tidak membatalkan
		// publikasi yang sudah sukses.
		if err := d.schRefreshMediaHealth(ctx, media.strOr("id", "")); err != nil {
			slog.Warn("[brandflow] gagal menyegarkan skor kesehatan media", "media", media.strOr("id", ""), "err", err)
		}
	}
	return nil
}

// peFailDelivery: catat error_code ke job lalu fail attempt; meneruskan error.
func (d *Deps) peFailDelivery(ctx context.Context, publication bsRow, jobID string, cause error, startedAt time.Time, terminal bool) error {
	code := peErrorCode(cause)
	message := truncateString(cause.Error(), 500)
	if jobID != "" {
		_ = d.exec(ctx, "UPDATE publication_jobs SET error_code = $1, error_message = $2 WHERE id = $3", code, message, jobID)
	}
	if err := d.peFailAttempt(ctx, publication, code, message, startedAt, terminal); err != nil {
		return err
	}
	slog.Error("[brandflow] publication delivery gagal", "publication", publication.strOr("id", ""), "code", code, "err", cause)
	return cause
}

// ---------- finance (port recordPublicationFinance, best-effort) ----------

// peRecordFinance: catat transaksi `publication.revenue` + earning mitra.
// Sengaja fire-and-forget — gangguan ledger tidak boleh menggagalkan publish.
func (d *Deps) peRecordFinance(ctx context.Context, publication, media bsRow) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Warn("[brandflow] finance recording panic", "err", rec)
		}
	}()
	clientPrice := publication.numOr("client_price")
	if clientPrice <= 0 {
		return
	}
	// Kirim ulang setelah revisi memakai publikasi yang sama; ledger tidak boleh
	// dicatat dua kali untuk satu publikasi.
	priorTx, priorErr := d.queryRow(ctx,
		"SELECT id FROM transactions WHERE publication_id = $1 AND type = 'publication.revenue' LIMIT 1",
		publication.strOr("id", ""))
	if priorErr == nil && priorTx != nil {
		return
	}
	partnerID := media.strOr("partner_id", "")
	err := d.exec(ctx,
		"INSERT INTO transactions (id, brand_id, partner_id, publication_id, campaign_id, amount, type, created_by) VALUES ($1, $2, $3, $4, $5, $6, 'publication.revenue', $7)",
		newUUID(), publication.strOr("brand_id", ""), nullIfEmpty(partnerID), publication.strOr("id", ""),
		publication.raw("campaign_id"), clientPrice, nullIfEmpty(publication.strOr("created_by", "")))
	if err != nil {
		slog.Warn("[brandflow] transaksi publikasi gagal", "err", err)
	}
	if partnerID == "" {
		return
	}

	basePrice := 0.0
	if publication.has("partner_price") {
		basePrice = publication.numOr("partner_price")
	}
	if basePrice <= 0 {
		if err := d.exec(ctx,
			"INSERT INTO partner_earnings (id, partner_id, publication_id, media_id, rule_id, amount, gross_amount, client_price, status) VALUES ($1, $2, $3, $4, NULL, $5, $5, $6, 'pending')",
			newUUID(), partnerID, publication.strOr("id", ""), media.strOr("id", ""), peNetAfterPlatformCut(clientPrice), clientPrice); err != nil {
			slog.Warn("[brandflow] partner earning gagal", "err", err)
		}
		return
	}

	grossShare := basePrice
	var ruleID *string
	if split, splitErr := d.peResolveRevenueShare(ctx, media.strOr("id", ""), partnerID, clientPrice, basePrice); splitErr == nil {
		grossShare = split.PartnerAmount
		ruleID = split.RuleID
	}
	partnerAmount := peNetAfterPlatformCut(grossShare)
	earningID := newUUID()
	if err := d.exec(ctx,
		"INSERT INTO partner_earnings (id, partner_id, publication_id, media_id, rule_id, amount, gross_amount, client_price, status, kind, source_earning_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', 'publication', NULL)",
		earningID, partnerID, publication.strOr("id", ""), media.strOr("id", ""), ruleID, partnerAmount, grossShare, clientPrice); err != nil {
		slog.Warn("[brandflow] partner earning gagal", "err", err)
		return
	}
	d.peCreateReferralCommission(ctx, earningID, partnerID, media.strOr("id", ""), partnerAmount, clientPrice)
	d.peCreditVipTransactionCommission(ctx, publication, partnerID, clientPrice)
}

type peRevenueSplit struct {
	RuleID        *string
	PartnerAmount float64
}

// Potongan platform atas penghasilan yang diajukan mitra (aturan pemilik
// produk 25%, mitra menerima 75%). amount di ledger menyimpan pendapatan
// bersih, gross_amount menyimpan angka sebelum potongan supaya popover
// rincian bisa tampil jujur.
const pePlatformCutRate = 0.25

func peNetAfterPlatformCut(gross float64) float64 {
	if gross < 0 {
		gross = 0
	}
	return round2(gross * (1 - pePlatformCutRate))
}

// peResolveRevenueShare: parity resolveRevenueShare (bagi hasil berbasis markup).
func (d *Deps) peResolveRevenueShare(ctx context.Context, mediaID, partnerID string, clientPrice, partnerPrice float64) (peRevenueSplit, error) {
	price := clientPrice
	if price < 0 {
		price = 0
	}
	base := partnerPrice
	if base < 0 {
		base = 0
	}
	pool := price - base
	if pool < 0 {
		pool = 0
	}
	if price <= 0 {
		return peRevenueSplit{PartnerAmount: 0}, nil
	}
	rule, err := d.queryRow(ctx, `
		SELECT * FROM revenue_shares
		WHERE status = 'active' AND (
		  (media_id = $1 AND partner_id = $2)
		  OR (media_id = $1 AND partner_id IS NULL)
		  OR (media_id IS NULL AND partner_id = $2)
		  OR (media_id IS NULL AND partner_id IS NULL)
		)
		ORDER BY CASE
		  WHEN media_id IS NOT NULL AND partner_id IS NOT NULL THEN 0
		  WHEN media_id IS NOT NULL THEN 1
		  WHEN partner_id IS NOT NULL THEN 2
		  ELSE 3 END
		LIMIT 1`, mediaID, partnerID)
	if err != nil {
		return peRevenueSplit{}, err
	}
	partnerAmount := base + round2((pool*25)/100)
	var ruleID *string
	if rule != nil {
		id := rule.strOr("id", "")
		ruleID = &id
		if rule.strOr("share_type", "percentage") == "fixed" {
			fixed := rule.numOr("partner_fixed")
			if fixed > pool {
				fixed = pool
			}
			partnerAmount = base + fixed
		} else {
			partnerAmount = base + round2((pool*rule.numOr("partner_percent"))/100)
		}
	}
	partnerAmount = round2(partnerAmount)
	if partnerAmount > price {
		partnerAmount = price
	}
	return peRevenueSplit{RuleID: ruleID, PartnerAmount: partnerAmount}, nil
}

// peReferralRateBpsByRole: parity getReferralRateBpsByRole (mitra 1%, klien 5%).
func (d *Deps) peReferralRateBpsByRole(ctx context.Context, role string) int {
	fallback := 100
	key := "mitra_commission_rate_bps"
	if role == "klien" {
		fallback = 500
		key = "klien_commission_rate_bps"
	}
	row, err := d.queryRow(ctx, "SELECT value FROM referral_settings WHERE key = $1 LIMIT 1", key)
	if err != nil || row == nil {
		return fallback
	}
	rate := int(peNumFromAny(row.strOr("value", "")))
	if rate < 0 || rate > 10000 {
		return fallback
	}
	return rate
}

// peCreateReferralCommission: parity createReferralCommissionForEarning.
func (d *Deps) peCreateReferralCommission(ctx context.Context, earningID, refereePartnerID, mediaID string, amount, clientPrice float64) {
	if earningID == "" || amount <= 0 || refereePartnerID == "" {
		return
	}
	refereePartner, err := d.queryRow(ctx, "SELECT user_id FROM partners WHERE id = $1 LIMIT 1", refereePartnerID)
	if err != nil || refereePartner == nil {
		return
	}
	refereeUserID := refereePartner.strOr("user_id", "")
	if refereeUserID == "" {
		return
	}
	user, err := d.queryRow(ctx, "SELECT referred_by_partner_id, email, role FROM users WHERE id = $1 LIMIT 1", refereeUserID)
	if err != nil || user == nil {
		return
	}
	referrerPartnerID := user.strOr("referred_by_partner_id", "")
	if referrerPartnerID == "" || referrerPartnerID == refereePartnerID {
		return
	}
	referrer, err := d.queryRow(ctx, "SELECT status FROM partners WHERE id = $1 LIMIT 1", referrerPartnerID)
	if err != nil || referrer == nil || referrer.strOr("status", "") != "active" {
		return
	}
	existing, err := d.queryRow(ctx,
		"SELECT id FROM partner_referral_commissions WHERE earning_id = $1 AND referrer_partner_id = $2 LIMIT 1", earningID, referrerPartnerID)
	if err != nil || existing != nil {
		return
	}
	needsReview := false
	if attr, err := d.queryRow(ctx,
		"SELECT notes FROM referral_attributions WHERE referee_user_id = $1 ORDER BY created_at DESC LIMIT 1", refereeUserID); err == nil && attr != nil {
		needsReview = strings.HasPrefix(attr.strOr("notes", ""), "fraud:")
	}
	rateBps := d.peReferralRateBpsByRole(ctx, user.strOr("role", "partner"))
	commissionAmount := round2((amount * float64(rateBps)) / 10000)
	if commissionAmount <= 0 {
		return
	}
	commissionStatus := "available"
	if needsReview {
		commissionStatus = "review_needed"
	}
	_ = d.exec(ctx,
		"INSERT INTO partner_referral_commissions (id, referrer_partner_id, referee_partner_id, referee_user_id, earning_id, amount, rate_bps, status) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		newUUID(), referrerPartnerID, refereePartnerID, refereeUserID, earningID, commissionAmount, rateBps, commissionStatus)
	if commissionStatus != "available" {
		return
	}
	// Legacy menulis earning dengan publication_id = earningId (FK publications
	// dapat menolak) — best-effort, sama seperti app lama.
	if err := d.exec(ctx,
		"INSERT INTO partner_earnings (id, partner_id, publication_id, media_id, rule_id, amount, gross_amount, client_price, status, settlement_id, kind, source_earning_id) VALUES ($1, $2, $3, $4, NULL, $5, $5, $6, 'pending', NULL, 'referral', $7)",
		newUUID(), referrerPartnerID, earningID, mediaID, commissionAmount, clientPrice, earningID); err != nil {
		slog.Warn("[brandflow] referral earning gagal", "err", err)
	}
}

// peCreditVipTransactionCommission: parity creditVipTransactionCommission
// (hanya Brand Ambassador).
func (d *Deps) peCreditVipTransactionCommission(ctx context.Context, publication bsRow, ownerPartnerID string, clientPrice float64) {
	if clientPrice <= 0 || ownerPartnerID == "" {
		return
	}
	ownerPartner, err := d.queryRow(ctx, "SELECT user_id FROM partners WHERE id = $1 LIMIT 1", ownerPartnerID)
	if err != nil || ownerPartner == nil {
		return
	}
	ownerUserID := ownerPartner.strOr("user_id", "")
	if ownerUserID == "" {
		return
	}
	ownerUser, err := d.queryRow(ctx, "SELECT referred_by_partner_id FROM users WHERE id = $1 LIMIT 1", ownerUserID)
	if err != nil || ownerUser == nil {
		return
	}
	referrerPartnerID := ownerUser.strOr("referred_by_partner_id", "")
	if referrerPartnerID == "" {
		return
	}
	vip := d.pePartnerVipInfo(ctx, referrerPartnerID)
	if !vip.IsVip || vip.Tier != "brand_ambassador" || vip.CommissionRate <= 0 {
		return
	}
	publicationID := publication.strOr("id", "")
	if publicationID == "" {
		return
	}
	existing, err := d.queryRow(ctx,
		"SELECT id FROM partner_earnings WHERE partner_id = $1 AND publication_id = $2 AND kind = 'transaction_commission' LIMIT 1",
		referrerPartnerID, publicationID)
	if err != nil || existing != nil {
		return
	}
	amount := round2((clientPrice * float64(vip.CommissionRate)) / 10000)
	if amount <= 0 {
		return
	}
	_ = d.exec(ctx,
		"INSERT INTO partner_earnings (id, partner_id, publication_id, media_id, rule_id, amount, gross_amount, client_price, status, kind, source_earning_id) VALUES ($1, $2, $3, $4, NULL, $5, $5, $6, 'pending', 'transaction_commission', $7) ON CONFLICT (publication_id) DO NOTHING",
		newUUID(), referrerPartnerID, publicationID, publication.strOr("media_id", ""), amount, clientPrice, publicationID)
}

type peVipInfo struct {
	IsVip          bool
	Tier           string
	CommissionRate int
}

func (d *Deps) pePartnerVipInfo(ctx context.Context, partnerID string) peVipInfo {
	out := peVipInfo{Tier: "regular"}
	row, err := d.queryRow(ctx,
		"SELECT is_vip, vip_tier, vip_expires_at FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil || row == nil {
		return out
	}
	rawIsVip := row.boolOr("is_vip", false)
	out.Tier = row.strOr("vip_tier", "regular")
	isExpired := false
	if expiresAt, ok := row.raw("vip_expires_at").(time.Time); ok && rawIsVip && !expiresAt.After(time.Now()) {
		isExpired = true
	}
	if isExpired {
		_ = d.exec(ctx,
			"UPDATE partners SET is_vip = false, vip_tier = 'regular', updated_at = NOW() WHERE id = $1 AND is_vip = true", partnerID)
	}
	out.IsVip = rawIsVip && !isExpired
	if settings, err := d.queryRow(ctx,
		"SELECT commission_rate_bp FROM affiliate_settings WHERE partner_id = $1 LIMIT 1", partnerID); err == nil && settings != nil {
		out.CommissionRate = int(settings.numOr("commission_rate_bp"))
	} else if out.IsVip {
		out.CommissionRate = 500
	} else {
		out.CommissionRate = 100
	}
	return out
}

// ---------- util ----------

// peAbsolutizeUploadURLs: parity absolutizeUploadUrls — rewrite semua
// src="/uploads/..." menjadi URL absolut origin publik plus tanda tangan
// publiknya, karena gambar artikel dibaca pengunjung situs media.
func peAbsolutizeUploadURLs(html, origin, appSecret string) string {
	if html == "" || origin == "" {
		return html
	}
	base := strings.TrimRight(origin, "/")
	var b strings.Builder
	i := 0
	for i < len(html) {
		idx := strings.Index(html[i:], "src=")
		if idx < 0 {
			b.WriteString(html[i:])
			break
		}
		pos := i + idx
		b.WriteString(html[i:pos])
		rest := pos + 4
		if rest >= len(html) {
			b.WriteString(html[pos:])
			break
		}
		quote := html[rest]
		if quote != '"' && quote != '\'' {
			b.WriteString(html[pos : rest+1])
			i = rest + 1
			continue
		}
		valueStart := rest + 1
		rel := strings.IndexByte(html[valueStart:], quote)
		if rel < 0 {
			b.WriteString(html[pos:])
			break
		}
		valueEnd := valueStart + rel
		value := html[valueStart:valueEnd]
		if strings.HasPrefix(value, "/uploads/") && !strings.ContainsAny(value, " \t\r\n>") {
			b.WriteString("src=")
			b.WriteByte(quote)
			b.WriteString(security.WithUploadSignature(base+value, appSecret))
			b.WriteByte(quote)
		} else {
			b.WriteString(html[pos : valueEnd+1])
		}
		i = valueEnd + 1
	}
	return b.String()
}

// peAbsolutizeValue: parity absolutizeUrl untuk satu nilai (string → string,
// kosong → ""). Berkas unggahan ikut ditandatangani supaya bisa dibuka publik
// di situs media.
func (d *Deps) peAbsolutizeValue(value, origin string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	absolute := trimmed
	if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") && origin != "" {
		absolute = strings.TrimRight(origin, "/") + trimmed
	}
	return security.WithUploadSignature(absolute, d.peAppSecret())
}

// peParseDefaults: parity parseDefaults (jsonb string/object → map).
func peParseDefaults(raw any) map[string]any {
	out := map[string]any{}
	switch v := raw.(type) {
	case map[string]any:
		return v
	case string:
		if strings.TrimSpace(v) == "" {
			return out
		}
		if json.Unmarshal([]byte(v), &out) != nil {
			return map[string]any{}
		}
		return out
	}
	return out
}

// peNameList: parity nameList — array → trim string non-kosong.
func peNameList(value any) []string {
	arr, ok := value.([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, item := range arr {
		if s := strings.TrimSpace(bsStringifyAny(item)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func peIntListAny(items []int) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

// peNumFromAny: angka dari any (float64/int/string).
func peNumFromAny(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case string:
		f, err := parseFloat(t)
		if err != nil {
			return 0
		}
		return f
	}
	return 0
}

// peHTTPRequest: request dengan SafeClient, balikan (status, body bytes).
func peHTTPRequest(ctx context.Context, method, target string, headers map[string]string, body string, timeout time.Duration) (int, []byte, error) {
	return peHTTPRequestBytes(ctx, method, target, headers, body, timeout)
}

func peHTTPRequestBytes(ctx context.Context, method, target string, headers map[string]string, body string, timeout time.Duration) (int, []byte, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := security.SafeClient(timeout).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return 0, nil, readErr
	}
	return resp.StatusCode, data, nil
}

// bsHTTPDoBytes: varian byte-body dari bsHTTPDo (untuk upload media biner).
func bsHTTPDoBytes(ctx context.Context, method, target string, headers map[string]string, body []byte, timeout time.Duration) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
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
