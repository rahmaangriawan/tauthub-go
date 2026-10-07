package partner

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Port lib/crawler/verification.ts — inti verifikasi Tauthub Crawler.
// Tuple canonical v1 dipakai BERSAMAAN crawler + Cloudflare Worker mitra;
// mengubah formatnya = semua request lama ditolak (by design).

const (
	crawlerSignatureVersion = "v1"
	// Naikkan bila isi dokumen persetujuan berubah (web: CRAWLER_CONSENT_VERSION)
	// supaya consent_doc_version pada kredensial baru mencatat versi yang benar.
	crawlerConsentVersion = "2"
	crawlerSecretBytes    = 32
	crawlerNonceBytes     = 16
)

// buildCanonical: v1|crawlerId|METHOD|host|path?query|ts|nonce.
func buildCanonical(crawlerID, method, host, pathAndQuery, ts, nonce string) string {
	return strings.Join([]string{
		crawlerSignatureVersion, crawlerID, strings.ToUpper(method),
		strings.ToLower(host), pathAndQuery, ts, nonce,
	}, "\n")
}

func crawlerSignRequest(crawlerID, secretB64, method, host, pathAndQuery, ts, nonce string) string {
	key, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil {
		return ""
	}
	return hmacSHA256Hex(key, buildCanonical(crawlerID, method, host, pathAndQuery, ts, nonce))
}

// generateCredential: crawler_id `site_` + 10 char base64url, secret 32 byte base64.
func generateCrawlerCredential() (string, string) {
	crawlerID := "site_" + randomBase64URL(8)
	if len(crawlerID) > 15 {
		crawlerID = crawlerID[:15]
	}
	return crawlerID, base64.StdEncoding.EncodeToString(secureRandomBytes(crawlerSecretBytes))
}

// crawlerSecretFingerprint: HMAC-SHA256("tauthub-crawler-fp", secret) → 16 hex.
func crawlerSecretFingerprint(secretB64 string) string {
	return hmacSHA256Hex([]byte("tauthub-crawler-fp"), secretB64)[:16]
}

// crawlerBodyFingerprint: HMAC-SHA256("tauthub-crawler-body", body[:4096]).
func crawlerBodyFingerprint(body string) string {
	if len(body) > 4096 {
		body = body[:4096]
	}
	return hmacSHA256Hex([]byte("tauthub-crawler-body"), body)
}

// sealCrawlerSecret: AES-256-GCM dengan master key base64 32 byte.
func sealCrawlerSecret(secretB64, masterKeyB64 string) (enc, nonce, tag []byte, err error) {
	key, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil || len(key) != 32 {
		return nil, nil, nil, errMasterKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}
	nonce = secureRandomBytes(12)
	sealed := gcm.Seal(nil, nonce, []byte(secretB64), nil)
	tag = sealed[len(sealed)-gcm.Overhead():]
	enc = sealed[:len(sealed)-gcm.Overhead()]
	return enc, nonce, tag, nil
}

// unsealCrawlerSecret: buka secret (GCM).
func unsealCrawlerSecret(enc, nonce, tag []byte, masterKeyB64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil || len(key) != 32 {
		return "", errMasterKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	combined := append(append([]byte{}, enc...), tag...)
	plain, err := gcm.Open(nil, nonce, combined, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

var errMasterKey = &PartnerError{http.StatusServiceUnavailable,
	"CRAWLER_MASTER_KEY harus 32 byte base64 — periksa konfigurasi (jangan pakai nilai default)."}

// generateCrawlerNonce: 16 byte acak → hex 32 char.
func generateCrawlerNonce() string { return randomHex(crawlerNonceBytes) }

// extractHost: ambil host tanpa skema/port.
var crawlerHostRe = regexp.MustCompile(`^(?:[a-z][a-z0-9+.-]*://)?([^/?#]+)`)
var crawlerPortRe = regexp.MustCompile(`:\d+$`)

func extractHost(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	m := crawlerHostRe.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) < 2 || m[1] == "" {
		return ""
	}
	host := strings.ToLower(m[1])
	return crawlerPortRe.ReplaceAllString(host, "")
}

// classifyCrawlerResponse: jangan langsung menuduh Cloudflare.
var cfChallengeRe = regexp.MustCompile(`(?i)<title>\s*(just a moment|attention required|access denied)`)

func classifyCrawlerResponse(status int, cfRay, cfMitigated, bodyPrefix string) string {
	switch {
	case status >= 200 && status < 300:
		return "ok"
	case status == 429:
		return "rate_limited"
	case cfMitigated != "" || cfChallengeRe.MatchString(bodyPrefix):
		return "blocked_cloudflare"
	case cfRay != "" && status == 403:
		return "blocked_waf"
	default:
		return "blocked_origin"
	}
}

// crawlerMasterKey: nilai env (32 byte base64).
func crawlerMasterKey() string { return strings.TrimSpace(envValue("CRAWLER_MASTER_KEY")) }

// ---------- credential store (port lib/crawler/store.ts) ----------

// activeCrawlerCredential: kredensial aktif + secret terbuka.
func (d *Deps) activeCrawlerCredential(r *http.Request, partnerID, mediaID string) (rowMap, string, error) {
	row, err := d.queryRow(r.Context(),
		"SELECT * FROM crawler_credentials WHERE partner_id = $1 AND media_id = $2 AND status IN ('active','rotating') ORDER BY created_at DESC LIMIT 1",
		partnerID, mediaID)
	if err != nil {
		return nil, "", err
	}
	if row == nil {
		return nil, "", nil
	}
	secret, err := d.unsealCrawlerRow(row)
	if err != nil {
		return nil, "", err
	}
	return row, secret, nil
}

func (d *Deps) unsealCrawlerRow(row rowMap) (string, error) {
	enc := toBytes(row.raw("secret_enc"))
	nonce := toBytes(row.raw("secret_nonce"))
	tag := toBytes(row.raw("secret_tag"))
	return unsealCrawlerSecret(enc, nonce, tag, crawlerMasterKey())
}

func toBytes(v any) []byte {
	switch t := v.(type) {
	case []byte:
		return t
	case string:
		return []byte(t)
	}
	return nil
}

func (d *Deps) crawlerAction(w http.ResponseWriter, r *http.Request, partner PartnerRow) {
	body := decodeJSON(r)
	action, _ := strField(body, "action")
	mediaID, _ := strField(body, "media_id")
	if mediaID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Request tidak valid")
		return
	}
	valid := false
	for _, a := range []string{"issue", "rotate", "disable", "revoke", "test"} {
		if action == a {
			valid = true
			break
		}
	}
	if !valid {
		httpx.Error(w, http.StatusUnprocessableEntity, "Request tidak valid")
		return
	}
	media, err := d.scopedMedia(r, partner.ID, mediaID)
	if err != nil {
		// Parity: route crawler memakai errorResponse, sehingga "Media tidak
		// ditemukan" tampil sebagai 500.
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}

	switch action {
	case "issue":
		consent, _ := boolField(body, "consent")
		if !consent {
			httpx.Error(w, http.StatusUnprocessableEntity, "Persetujuan dokumen akses crawler diperlukan sebelum membuat kredensial.")
			return
		}
		host := extractHost(media.strOr("website_url", ""))
		if host == "" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Media belum punya website_url — isi dulu di tab Media.")
			return
		}
		crawlerID, secretB64 := generateCrawlerCredential()
		enc, nonce, tag, err := sealCrawlerSecret(secretB64, crawlerMasterKey())
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if err := d.exec(r.Context(), `
			INSERT INTO crawler_credentials (media_id, partner_id, domain, crawler_id, secret_enc, secret_nonce, secret_tag, secret_fingerprint, status, allowed_hosts, consent_at, consent_doc_version)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active', $9, NOW(), $10)`,
			mediaID, partner.ID, host, crawlerID, enc, nonce, tag, crawlerSecretFingerprint(secretB64),
			pqTextArray([]string{host}), crawlerConsentVersion); err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLogActorless(r.Context(), "crawler.credential_created", "media", mediaID)
		writeJSON(w, http.StatusOK, map[string]any{
			"crawlerId":           crawlerID,
			"secretB64":           secretB64,
			"status":              "active",
			"domain":              host,
			"consent_at":          isoMilli(nowUTC()),
			"consent_doc_version": crawlerConsentVersion,
			"worker_template":     workerTemplateFor(crawlerID),
			"secret_note":         "Secret hanya ditampilkan SEKALI. Salin ke Worker Secret di Cloudflare.",
		})
	case "rotate":
		row, _, err := d.activeCrawlerCredential(r, partner.ID, mediaID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if row == nil {
			httpx.Error(w, http.StatusNotFound, "Belum ada kredensial untuk media ini.")
			return
		}
		crawlerID := row.strOr("crawler_id", "")
		_, secretB64 := generateCrawlerCredential()
		enc, nonce, tag, err := sealCrawlerSecret(secretB64, crawlerMasterKey())
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if err := d.exec(r.Context(), `
			UPDATE crawler_credentials SET secret_enc = $1, secret_nonce = $2, secret_tag = $3, secret_fingerprint = $4, status = 'active', rotated_at = NOW()
			WHERE id = $5 AND partner_id = $6`,
			enc, nonce, tag, crawlerSecretFingerprint(secretB64), row.raw("id"), partner.ID); err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLogActorless(r.Context(), "crawler.credential_rotated", "media", mediaID)
		writeJSON(w, http.StatusOK, map[string]any{
			"crawlerId":   crawlerID,
			"secretB64":   secretB64,
			"status":      "active",
			"secret_note": "Secret baru. Update TAUTHUB_SECRET di Worker (secret lama tetap valid 24 jam selama rotasi).",
		})
	case "disable", "revoke":
		status := "disabled"
		auditAction := "crawler.credential_disabled"
		if action == "revoke" {
			status = "revoked"
			auditAction = "crawler.credential_revoked"
		}
		affected, err := d.execRows(r.Context(), `
			UPDATE crawler_credentials SET status = $1, revoked_at = CASE WHEN $1 = 'revoked' THEN NOW() ELSE revoked_at END
			WHERE partner_id = $2 AND media_id = $3 AND status IN ('active','rotating')`,
			status, partner.ID, mediaID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLogActorless(r.Context(), auditAction, "media", mediaID)
		if affected == 0 {
			httpx.Error(w, http.StatusNotFound, "Belum ada kredensial aktif.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "test":
		writeJSON(w, http.StatusOK, d.crawlerTestConnection(r, partner.ID, media))
	}
}

// auditLogActorless: audit dengan actor NULL (parity crawler store).
func (d *Deps) auditLogActorless(ctx context.Context, action, targetType, targetID string) {
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), NULL, $1, $2, $3)",
		action, targetType, targetID); err != nil {
		d.logger().Warn("audit gagal", "action", action, "err", err)
	}
}

// crawlerTestConnection: parity runTestConnection — request normal dulu, lalu
// signed retry bila terblokir.
func (d *Deps) crawlerTestConnection(r *http.Request, partnerID string, media rowMap) map[string]any {
	website := media.strOr("website_url", "")
	target := website
	if !strings.HasPrefix(strings.ToLower(website), "http://") && !strings.HasPrefix(strings.ToLower(website), "https://") {
		target = "https://" + website
	}
	browser := map[string]string{
		"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	}
	status, body, headers := d.safeGet(r, target, browser, 15*time.Second)
	cfRay, cfMitigated, serverHeader := headerOf(headers, "cf-ray"), headerOf(headers, "cf-mitigated"), headerOf(headers, "server")
	verdict := classifyCrawlerResponse(status, cfRay, cfMitigated, body)
	domain := extractHost(website)
	if domain == "" {
		if parsed, err := url.Parse(target); err == nil {
			domain = parsed.Hostname()
		}
	}
	d.recordCrawlerDiagnostic(r, media.strOr("id", ""), domain, target, status, cfRay, cfMitigated, serverHeader, crawlerBodyFingerprint(body), verdict)

	if verdict == "ok" {
		return map[string]any{"ok": true, "verdict": verdict,
			"message": "Website dapat diakses crawler tanpa kendala (tidak terblokir)."}
	}
	cred, secret, err := d.activeCrawlerCredential(r, partnerID, media.strOr("id", ""))
	if err != nil || cred == nil {
		return map[string]any{"ok": false, "verdict": verdict,
			"message": "Website terblokir dan belum ada kredensial crawler. Klik \"Buat Kredensial\" lalu ikuti langkah berikutnya."}
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return map[string]any{"ok": false, "verdict": verdict, "message": "URL website tidak valid."}
	}
	ts := itoa(int(time.Now().Unix()))
	nonce := generateCrawlerNonce()
	host := strings.ToLower(parsed.Hostname())
	sign := crawlerSignRequest(cred.strOr("crawler_id", ""), secret, http.MethodGet, host, parsed.Path+"?"+parsed.RawQuery, ts, nonce)
	if parsed.RawQuery == "" {
		sign = crawlerSignRequest(cred.strOr("crawler_id", ""), secret, http.MethodGet, host, parsed.Path, ts, nonce)
	}
	signedHeaders := map[string]string{}
	for k, v := range browser {
		signedHeaders[k] = v
	}
	signedHeaders["x-tauthub-site"] = cred.strOr("crawler_id", "")
	signedHeaders["x-tauthub-v"] = crawlerSignatureVersion
	signedHeaders["x-tauthub-ts"] = ts
	signedHeaders["x-tauthub-nonce"] = nonce
	signedHeaders["x-tauthub-sign"] = sign
	signedStatus, signedBody, signedRespHeaders := d.safeGet(r, target, signedHeaders, 15*time.Second)
	signedCfRay := headerOf(signedRespHeaders, "cf-ray")
	signedVerdict := classifyCrawlerResponse(signedStatus, signedCfRay, headerOf(signedRespHeaders, "cf-mitigated"), signedBody)
	recordedVerdict := signedVerdict
	if signedVerdict != "ok" {
		recordedVerdict = "signature_rejected"
	}
	d.recordCrawlerDiagnostic(r, media.strOr("id", ""), domain, target, signedStatus, signedCfRay,
		headerOf(signedRespHeaders, "cf-mitigated"), headerOf(signedRespHeaders, "server"), crawlerBodyFingerprint(signedBody), recordedVerdict)

	if signedVerdict == "ok" {
		return map[string]any{
			"ok": true, "verdict": "worker_active", "signed_status": signedStatus,
			"message": "Kredensial diterima worker Anda — crawler bisa melewati proteksi. (Tanpa signature tetap diblokir, sesuai desain.)",
		}
	}
	if signedStatus == 401 {
		hint := "Worker merespons 401 — pastikan TAUTHUB_SITE_ID dan TAUTHUB_SECRET terisi benar, dan Worker sudah di-route ke domain ini."
		if strings.Contains(signedBody, "TAUTHUB_") {
			hint = strings.TrimSpace(signedBody)
		}
		return map[string]any{"ok": false, "verdict": verdict, "signed_verdict": "signature_rejected",
			"hint": hint, "message": "Kredensial belum diterima worker. Periksa variabel Worker."}
	}
	return map[string]any{
		"ok": false, "verdict": verdict, "signed_verdict": signedVerdict,
		"message": "Kredensial terkirim tapi masih terblokir. Pastikan Worker sudah di-deploy dan route-nya mencakup domain ini.",
	}
}

// safeGet: GET via safe client; status 0 saat gagal (parity `?? null`).
func (d *Deps) safeGet(r *http.Request, target string, headers map[string]string, timeout time.Duration) (int, string, http.Header) {
	resp, err := security.Get(r.Context(), target, headers, timeout)
	if err != nil {
		return 0, "", http.Header{}
	}
	body, _ := readBodyLimited(resp, 512*1024)
	return resp.StatusCode, body, resp.Header
}

func headerOf(h http.Header, key string) string {
	if h == nil {
		return ""
	}
	return h.Get(key)
}

func (d *Deps) recordCrawlerDiagnostic(r *http.Request, mediaID, domain, target string, status int, cfRay, cfMitigated, serverHeader, fingerprint, verdict string) {
	var statusVal any
	if status != 0 {
		statusVal = status
	}
	if err := d.exec(r.Context(), `
		INSERT INTO crawler_diagnostics (media_id, domain, url, http_status, cf_ray, cf_mitigated, server_header, body_fingerprint, verdict)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		mediaID, domain, target, statusVal, ptrOrNil(cfRay), ptrOrNil(cfMitigated), ptrOrNil(serverHeader), fingerprint, verdict); err != nil {
		d.logger().Warn("diagnostik crawler gagal", "err", err)
	}
}

// pqTextArray: literalisasi text[] untuk parameter pgx (hindari tipe driver).
func pqTextArray(items []string) any {
	if len(items) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, item := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(item))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

// ---------- Cloudflare Worker template (port worker-template.ts) ----------

const crawlerWorkerTemplate = `// Tauthub Crawler Verification v1 — salin ke Cloudflare Workers.
// Isi 3 variabel di Settings → Variables: TAUTHUB_SITE_ID (Text),
// TAUTHUB_SECRET (Secret), TAUTHUB_SECRET_PREVIOUS (Secret, opsional saat rotasi).
const WINDOW_SECONDS = 300; // toleransi jam server — jangan < 120

// Konstanta header yang sama dengan crawler Tauthub.
const H_SITE = "x-tauthub-site";
const H_VER = "x-tauthub-v";
const H_TS = "x-tauthub-ts";
const H_NONCE = "x-tauthub-nonce";
const H_SIGN = "x-tauthub-sign";

function timingSafeEqualHex(a, b) {
  if (typeof a !== "string" || typeof b !== "string" || a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return diff === 0;
}

async function hmacHex(secretB64, canonical) {
  const key = await crypto.subtle.importKey("raw", atob(secretB64), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const sig = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(canonical));
  return [...new Uint8Array(sig)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

function buildCanonical(req, siteId, ts, nonce) {
  const url = new URL(req.url);
  return ["v1", siteId, req.method.toUpperCase(), url.hostname.toLowerCase(), url.pathname + url.search, ts, nonce].join("\n");
}

export default {
  async fetch(request, env) {
    const siteId = request.headers.get(H_SITE);
    const ver = request.headers.get(H_VER) || "v1";
    const ts = request.headers.get(H_TS);
    const nonce = request.headers.get(H_NONCE);
    const sign = request.headers.get(H_SIGN);

    // Bukan request crawler bertanda → biarkan WAF normal bekerja.
    if (!siteId || !ts || !nonce || !sign) return fetch(request);
    if (ver !== "v1") return new Response("TAUTHUB_UNSUPPORTED_VERSION", { status: 401 });
    if (siteId !== env.TAUTHUB_SITE_ID) return new Response("TAUTHUB_UNKNOWN_SITE", { status: 401 });

    // Window waktu — request lama ditolak (anti-replay).
    const now = Math.floor(Date.now() / 1000);
    const tsNum = Number(ts);
    if (!Number.isFinite(tsNum) || Math.abs(now - tsNum) > WINDOW_SECONDS) {
      return new Response("TAUTHUB_EXPIRED", { status: 401 });
    }

    // Rate limit sederhana via cache (per site per menit).
    const rlKey = "tauthub-rl:" + siteId + ":" + Math.floor(now / 60);
    const rlCount = Number(await env.TAUTHUB_KV?.get(rlKey) || "0");
    if (rlCount >= 60) return new Response("TAUTHUB_RATE_LIMITED", { status: 429 });
    await env.TAUTHUB_KV?.put(rlKey, String(rlCount + 1), { expirationTtl: 120 });

    // Nonce single-use — cek KV; kalau sudah ada = replay.
    const nonceKey = "tauthub-nonce:" + nonce;
    if (env.TAUTHUB_KV && await env.TAUTHUB_KV.get(nonceKey)) {
      return new Response("TAUTHUB_REPLAY", { status: 401 });
    }
    if (env.TAUTHUB_KV) await env.TAUTHUB_KV.put(nonceKey, "1", { expirationTtl: 600 });

    // Verifikasi HMAC terhadap secret aktif & secret lama (masa rotasi).
    const canonical = buildCanonical(request, siteId, ts, nonce);
    const candidates = [env.TAUTHUB_SECRET, env.TAUTHUB_SECRET_PREVIOUS].filter(Boolean);
    let ok = false;
    for (const secretB64 of candidates) {
      const expected = await hmacHex(secretB64, canonical);
      if (timingSafeEqualHex(expected, sign)) { ok = true; break; }
    }
    if (!ok) return new Response("TAUTHUB_INVALID_SIGNATURE", { status: 401 });

    // Valid → teruskan, beri tanda ke origin. Origin HANYA boleh mempercayai
    // header ini bila request berasal dari IP Cloudflare (lihat panduan).
    const forwarded = new Request(request);
    forwarded.headers.set("x-tauthub-verified", siteId);
    return fetch(forwarded);
  },
};
`

func workerTemplateFor(siteID string) string {
	return strings.Replace(crawlerWorkerTemplate, "TAUTHUB_SITE_ID (Text),", "TAUTHUB_SITE_ID (Text, isi: "+siteID+"),", 1)
}
