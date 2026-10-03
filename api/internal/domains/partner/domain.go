package partner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// domainFromWebsite: parity domainFromWebsite — hostname lowercase tanpa www,
// tolak host privat/unsafe (dipakai endpoint SEO media).
func domainFromWebsite(website string) any {
	raw := strings.TrimSpace(website)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(ensureScheme(raw))
	if err != nil {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if host == "" || isUnsafeHost(host) {
		return nil
	}
	return host
}

func ensureScheme(raw string) string {
	if strings.Contains(raw, "://") {
		return raw
	}
	return "https://" + raw
}

func isUnsafeHost(host string) bool {
	if host == "localhost" || host == "::1" {
		return true
	}
	if strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return true
	}
	for _, prefix := range []string{"10.", "127.", "192.168."} {
		if strings.HasPrefix(host, prefix) {
			return true
		}
	}
	if strings.HasPrefix(host, "172.") {
		parts := strings.Split(host, ".")
		if len(parts) > 1 {
			if n, err := parseSmallInt(parts[1]); err == nil && n >= 16 && n <= 31 {
				return true
			}
		}
	}
	return false
}

// verifyDomainOwnership: port verifyDomainOwnership — periksa halaman media
// + token verifikasi lewat security.VerifyDomainOwnership (klasifikasi WAF +
// fallback DNS TXT terpusat). Semua tahap dicatat di kolom domain_* supaya
// mitra bisa melihat penyebab kegagalan (parity alat diagnosa app lama).
func (d *Deps) verifyDomainOwnership(r *http.Request, mediaID string) map[string]any {
	ctx := r.Context()
	row, err := d.queryRow(ctx, `
		SELECT id, name, website_url, domain_verify_token, domain_verified, domain_check_fail_count, domain_health
		FROM media WHERE id = $1 LIMIT 1`, mediaID)
	if err != nil || row == nil {
		return map[string]any{"ok": false, "status": "error", "stage": "load", "trace": []string{"Media tidak ditemukan"}}
	}
	website := strings.TrimSpace(row.strOr("website_url", ""))
	if website == "" {
		return d.recordDomainCheckFailure(ctx, mediaID, row,
			"token_not_found", "Website/domain media belum diisi", "no_website", []string{"Website/domain media kosong"})
	}
	token := row.strOr("domain_verify_token", "")
	target := strings.Replace(ensureScheme(website), "http://", "https://", 1)

	out := security.VerifyDomainOwnership(ctx, target, token)
	switch out.Result {
	case "verified":
		if err := d.exec(ctx, `
			UPDATE media SET domain_verified = TRUE, domain_verified_at = NOW(), domain_last_checked_at = NOW(),
			  domain_check_fail_count = 0, domain_health = 'live', domain_last_result = 'verified',
			  domain_last_error = NULL, domain_last_stage = $1, domain_last_trace = $2, domain_last_error_at = NULL, updated_at = NOW()
			WHERE id = $3`, out.Stage, marshalJSONArr(out.Trace), mediaID); err != nil {
			return map[string]any{"ok": false, "status": "error", "stage": "update", "trace": out.Trace}
		}
		if err := d.exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), NULL, 'media.domain_verified', 'media', $1)",
			mediaID); err != nil {
			d.logger().Warn("audit domain_verified gagal", "media", mediaID, "err", err)
		}
		d.approveReferredDomains(ctx, mediaID)
		return map[string]any{
			"ok": true, "status": "verified", "checked_at": isoMilli(time.Now()),
			"fail_count": 0, "stage": out.Stage, "trace": out.Trace,
		}
	case "blocked_by_waf":
		return d.recordDomainCheckNeutral(ctx, mediaID, out.Result, out.Message, out.Stage, out.Trace)
	case "unreachable":
		return d.recordDomainCheckFailure(ctx, mediaID, row,
			"unreachable", truncateRunes(out.Message, 300), out.Stage, out.Trace)
	case "token_missing":
		result := d.recordDomainCheckFailure(ctx, mediaID, row,
			"token_missing", out.Message, out.Stage, out.Trace)
		result["found_token"] = truncateRunes(out.FoundToken, 80)
		return result
	default:
		return d.recordDomainCheckFailure(ctx, mediaID, row,
			out.Result, out.Message, out.Stage, out.Trace)
	}
}

// recordDomainCheckNeutral: catat hasil pemeriksaan TANPA penalti — untuk
// blocked_by_waf: situs hidup, WAF mitra hanya menolak IP datacenter kami.
// fail_count, domain_verified, dan domain_health tidak disentuh.
func (d *Deps) recordDomainCheckNeutral(ctx context.Context, mediaID, result, message, stage string, trace []string) map[string]any {
	_ = d.exec(ctx, `
		UPDATE media SET domain_last_checked_at = NOW(), domain_last_result = $1,
		  domain_last_error = $2, domain_last_stage = $3, domain_last_trace = $4,
		  domain_last_error_at = NOW(), updated_at = NOW()
		WHERE id = $5`,
		result, message, stage, marshalJSONArr(trace), mediaID)
	return map[string]any{
		"ok":      false,
		"status":  result,
		"checked_at": isoMilli(time.Now()),
		"stage":   stage,
		"trace":   trace,
		"reason":  message,
		"neutral": true,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// recordDomainCheckFailure: naikkan fail_count; setelah 3 kegagalan berturut
// pada media yang sebelumnya terverifikasi, status verifikasi dicabut.
func (d *Deps) recordDomainCheckFailure(ctx context.Context, mediaID string, before rowMap, result, message, stage string, trace []string) map[string]any {
	failCount := before.intOr("domain_check_fail_count", 0) + 1
	wasVerified := before.boolOr("domain_verified", false)
	revoked := wasVerified && failCount >= 3
	health := before.strOr("domain_health", "unknown")
	if result == "unreachable" {
		health = "unreachable"
	}
	if revoked {
		_ = d.exec(ctx, `
			UPDATE media SET domain_verified = FALSE, domain_last_checked_at = NOW(), domain_check_fail_count = $1,
			  domain_health = $2, domain_last_result = $3, domain_last_error = $4, domain_last_stage = $5,
			  domain_last_trace = $6, domain_last_error_at = NOW(), updated_at = NOW()
			WHERE id = $7`,
			failCount, health, result, message, stage, marshalJSONArr(trace), mediaID)
		if err := d.exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), NULL, 'media.domain_revoked', 'media', $1)",
			mediaID); err != nil {
			d.logger().Warn("audit domain_revoked gagal", "media", mediaID, "err", err)
		}
	} else {
		_ = d.exec(ctx, `
			UPDATE media SET domain_last_checked_at = NOW(), domain_check_fail_count = $1,
			  domain_health = $2, domain_last_result = $3, domain_last_error = $4, domain_last_stage = $5,
			  domain_last_trace = $6, domain_last_error_at = NOW(), updated_at = NOW()
			WHERE id = $7`,
			failCount, health, result, message, stage, marshalJSONArr(trace), mediaID)
	}
	payload := map[string]any{
		"ok":         false,
		"status":     result,
		"checked_at": isoMilli(time.Now()),
		"fail_count": failCount,
		"stage":      stage,
		"trace":      trace,
		"reason":     message,
	}
	if revoked {
		payload["revoked"] = true
	}
	return payload
}

// approveReferredDomains: domain rujukan yang menunggu approve untuk media ini
// ditandai approved saat kepemilikan domain terbukti (best-effort).
func (d *Deps) approveReferredDomains(ctx context.Context, mediaID string) {
	_ = d.exec(ctx,
		"UPDATE referred_domains SET status = 'approved', rejection_reason = NULL, updated_at = NOW() WHERE media_id = $1 AND status = 'pending'",
		mediaID)
}

// ---------- POST /v1/partner/media/{mediaId}/submit-ke-verifikasi (internal) ----------

// domainVerifyTokenHash: sidik jari token untuk log (parity fingerprint).
func domainVerifyTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

var _ = httpx.Error
