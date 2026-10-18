// Package partner: port portal mitra (/mitra) — app/v1/partner/*.
//
// Tenancy: mitra diidentifikasi lewat partners.user_id (status 'active').
// URL publik mitra memakai partners.public_id (6 karakter), tapi seluruh
// endpoint di sini tetap menerima UUID internal (parity BrandAccess).
package partner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

type Deps struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

// ---------- error mapping (port NotFound/Conflict + partnerError) ----------

type PartnerError struct {
	Status  int
	Message string
}

func (e *PartnerError) Error() string { return e.Message }

func errNotFound(msg string) *PartnerError { return &PartnerError{http.StatusNotFound, msg} }
func errConflict(msg string) *PartnerError { return &PartnerError{http.StatusConflict, msg} }
func errUnprocessable(msg string) *PartnerError {
	return &PartnerError{http.StatusUnprocessableEntity, msg}
}
func errBadRequest(msg string) *PartnerError { return &PartnerError{http.StatusBadRequest, msg} }
func errForbidden(msg string) *PartnerError  { return &PartnerError{http.StatusForbidden, msg} }
func errUnauthorized() *PartnerError {
	return &PartnerError{http.StatusUnauthorized, "Invalid or expired access token"}
}

// rateLimitError: parity AuthRateLimitError (429, pesan verbatim).
func errRateLimit() *PartnerError {
	return &PartnerError{http.StatusTooManyRequests, "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi."}
}

// writeErr: port partnerError — NotFound→404, Conflict→409, AuthRateLimit→429,
// duplikat→409, sisanya 500 (di-scrub saat production).
func (d *Deps) writeErr(w http.ResponseWriter, err error) {
	var pe *PartnerError
	if errors.As(err, &pe) {
		httpx.Error(w, pe.Status, pe.Message)
		return
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "duplicate") || strings.Contains(lower, "unique") || strings.Contains(lower, "uq_") {
		httpx.Error(w, http.StatusConflict, "Brand slug, domain, or membership already exists")
		return
	}
	slog.Error("[partner]", "err", err)
	httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// ---------- helpers ----------

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func clampInt(raw string, def, min, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		n = def
	}
	if n < min {
		n = min
	}
	if n > max {
		n = max
	}
	return n
}

func round2(v float64) float64 {
	if v < 0 {
		return float64(int(v*100-0.5)) / 100
	}
	return float64(int(v*100+0.5)) / 100
}

// isoMilli: serialisasi waktu parity JSON.stringify(Date) (milidetik, UTC).
func isoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// tsOrNil: *time.Time → string ISO atau nil.
func tsOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return isoMilli(*t)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrOrNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func itoa(n int) string { return strconv.Itoa(n) }

// ---------- partner access (port requirePartner + cache 30s) ----------

type PartnerRow struct {
	ID                       string
	UserID                   string
	Name                     *string
	PublicID                 *string
	Category                 *string
	Status                   string
	RSSEnabled               bool
	AutoPublishEnabled       bool
	AutoApprove              bool
	IsVIP                    bool
	VIPTier                  *string
	VIPExpiresAt             *time.Time
	IntegrationTermsAccepted *time.Time
	PayoutType               *string
	BankName                 *string
	BankAccountNumber        *string
	BankAccountName          *string
	ReferralCode             *string
	CreatedAt                time.Time
}

type partnerCacheEntry struct {
	row       PartnerRow
	expiresAt time.Time
}

var (
	partnerMu    sync.Mutex
	partnerCache = map[string]partnerCacheEntry{}
)

// InvalidatePartnerCache: parity invalidatePartnerCache(userId).
func InvalidatePartnerCache(userID string) {
	partnerMu.Lock()
	defer partnerMu.Unlock()
	if userID == "" {
		partnerCache = map[string]partnerCacheEntry{}
		return
	}
	delete(partnerCache, userID)
}

const partnerCacheTTL = 30 * time.Second

// PartnerAccess: port requirePartner — lookup partner aktif milik user (cache 30s).
func (d *Deps) PartnerAccess(ctx context.Context, userID string) (PartnerRow, error) {
	now := time.Now()
	partnerMu.Lock()
	if c, ok := partnerCache[userID]; ok && c.expiresAt.After(now) {
		partnerMu.Unlock()
		return c.row, nil
	}
	partnerMu.Unlock()

	var row PartnerRow
	err := d.Pool.QueryRow(ctx, `
		SELECT id, user_id, name, public_id, category, status,
		       COALESCE(rss_enabled, FALSE), COALESCE(auto_publish_enabled, FALSE), COALESCE(auto_approve, FALSE),
		       COALESCE(is_vip, FALSE), vip_tier, vip_expires_at, integration_terms_accepted_at,
		       payout_type, bank_name, bank_account_number, bank_account_name, referral_code, created_at
		FROM partners
		WHERE user_id = $1 AND status = 'active'
		LIMIT 1`, userID).
		Scan(&row.ID, &row.UserID, &row.Name, &row.PublicID, &row.Category, &row.Status,
			&row.RSSEnabled, &row.AutoPublishEnabled, &row.AutoApprove,
			&row.IsVIP, &row.VIPTier, &row.VIPExpiresAt, &row.IntegrationTermsAccepted,
			&row.PayoutType, &row.BankName, &row.BankAccountNumber, &row.BankAccountName, &row.ReferralCode, &row.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PartnerRow{}, errNotFound("Partner access required")
		}
		return PartnerRow{}, err
	}

	partnerMu.Lock()
	partnerCache[userID] = partnerCacheEntry{row, now.Add(partnerCacheTTL)}
	partnerMu.Unlock()
	return row, nil
}

// auth: user + partner aktif + cek CSRF untuk metode non-GET/HEAD (parity).
func (d *Deps) auth(w http.ResponseWriter, r *http.Request) (*httpx.AuthUser, PartnerRow, bool) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return nil, PartnerRow{}, false
	}
	method := strings.ToUpper(r.Method)
	if method != "GET" && method != "HEAD" {
		if msg := csrfViolation(r); msg != "" {
			// Parity: error CSRF lolos ke errorResponse generik → 500 (scrub di production).
			slog.Error("[partner] csrf", "err", msg, "path", r.URL.Path)
			httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, msg))
			return nil, PartnerRow{}, false
		}
	}
	partner, err := d.PartnerAccess(r.Context(), user.ID)
	if err != nil {
		d.writeErr(w, err)
		return nil, PartnerRow{}, false
	}
	return user, partner, true
}

// csrfViolation: parity cek Origin/Referer, tapi dibandingkan terhadap allowlist
// origin publik (sama dengan CORS) DAN host request. App lama bisa membandingkan
// langsung ke host karena web+API satu origin; di repo Go web dilayani terpisah
// (dev :3001 → API :8080, produksi nginx same-origin), sehingga pembanding
// host-saja selalu menolak request sah dari browser.
func csrfViolation(r *http.Request) string {
	origin := r.Header.Get("origin")
	referer := r.Header.Get("referer")
	host := publicHost(r)
	if origin != "" {
		if !httpx.OriginAllowed(origin) && !strings.Contains(origin, "://"+host) {
			return "CSRF protection: invalid Origin"
		}
		return ""
	}
	if referer != "" {
		if httpx.OriginAllowed(originOf(referer)) || strings.Contains(referer, "://"+host) {
			return ""
		}
		return "CSRF protection: invalid Referer"
	}
	return ""
}

// originOf: skema+host dari URL referer.
func originOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func publicHost(r *http.Request) string {
	if xf := r.Header.Get("x-forwarded-host"); xf != "" {
		return strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	if h := r.Header.Get("host"); h != "" {
		return h
	}
	return r.Host
}

// ---------- audit ----------

func (d *Deps) auditLog(ctx context.Context, userID, action, targetType, targetID string) {
	if _, err := d.Pool.Exec(ctx,
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, $4, $5)",
		uuid.NewString(), userID, action, targetType, targetID); err != nil {
		slog.Warn("[partner] audit gagal", "action", action, "err", err)
	}
}

// ---------- pricing engine (port lib/pricing/engine.ts) ----------

const defaultMarkupPercent = 15

type markupCacheEntry struct {
	value   float64
	expires time.Time
}

var (
	markupMu     sync.Mutex
	markupCached *markupCacheEntry
)

func (d *Deps) getMarkupPercent(ctx context.Context) float64 {
	now := time.Now()
	markupMu.Lock()
	if markupCached != nil && markupCached.expires.After(now) {
		v := markupCached.value
		markupMu.Unlock()
		return v
	}
	markupMu.Unlock()

	value := 0.0
	found := false
	if err := d.Pool.QueryRow(ctx, "SELECT markup_percent FROM pricing_config WHERE id = 1 LIMIT 1").Scan(&value); err == nil {
		found = value >= 0
	}
	if !found {
		value = defaultMarkupPercent
	}
	markupMu.Lock()
	markupCached = &markupCacheEntry{value: value, expires: now.Add(30 * time.Second)}
	markupMu.Unlock()
	return value
}

func computeClientPrice(partnerPrice, markupPercent float64) float64 {
	base := partnerPrice
	if base < 0 {
		base = 0
	}
	markup := markupPercent
	if markup < 0 {
		markup = 0
	}
	return round2(base * (1 + markup/100))
}

type priceBreakdown struct {
	PartnerPrice  float64 `json:"partner_price"`
	MarkupPercent float64 `json:"markup_percent"`
	MarkupAmount  float64 `json:"markup_amount"`
	ClientPrice   float64 `json:"client_price"`
}

func breakdownFor(partnerPrice, markupPercent float64) priceBreakdown {
	partner := partnerPrice
	if partner < 0 {
		partner = 0
	}
	markup := markupPercent
	if markup < 0 {
		markup = 0
	}
	client := computeClientPrice(partner, markup)
	return priceBreakdown{
		PartnerPrice:  partner,
		MarkupPercent: markup,
		MarkupAmount:  round2(client - partner),
		ClientPrice:   client,
	}
}

// ---------- tc_config (port getTcConfig) ----------

type tcConfig struct {
	RateIDR               float64
	MinimumWithdrawIDR    float64
	WithdrawBankFeeIDR    float64
	WithdrawEwalletFeePct float64
}

func (d *Deps) getTcConfig(r *http.Request) tcConfig {
	cfg := tcConfig{RateIDR: 1000, MinimumWithdrawIDR: 50000, WithdrawBankFeeIDR: 2400, WithdrawEwalletFeePct: 0.7}
	var rate, minWithdraw, bankFee, ewalletPct *float64
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT rate_idr, minimum_withdraw_idr, withdraw_bank_fee_idr, withdraw_ewallet_fee_percent FROM tc_config WHERE id = 1 LIMIT 1").
		Scan(&rate, &minWithdraw, &bankFee, &ewalletPct); err == nil {
		if rate != nil && *rate >= 1 {
			cfg.RateIDR = *rate
		}
		if minWithdraw != nil {
			cfg.MinimumWithdrawIDR = *minWithdraw
		}
		if bankFee != nil {
			cfg.WithdrawBankFeeIDR = *bankFee
		}
		if ewalletPct != nil {
			cfg.WithdrawEwalletFeePct = *ewalletPct
		}
	}
	return cfg
}

// ---------- finance summary (port lib/finance/service.ts) ----------

type financeSummary struct {
	PendingEarnings float64
	SettledEarnings float64
	PaidSettlements float64
}

func (d *Deps) partnerFinanceSummary(ctx context.Context, partnerID string) financeSummary {
	var pending, settled, paid float64
	_ = d.Pool.QueryRow(ctx, `
		SELECT
		  COALESCE((SELECT SUM(amount) FROM partner_earnings WHERE partner_id = $1 AND status = 'pending'), 0)::numeric(14,2),
		  (COALESCE((SELECT SUM(amount) FROM partner_earnings WHERE partner_id = $1 AND status = 'settled' AND settlement_id IS NULL), 0)
		   - COALESCE((SELECT SUM(amount_idr) FROM partner_service_charges WHERE partner_id = $1), 0))::numeric(14,2),
		  COALESCE((SELECT SUM(total_amount) FROM settlements WHERE partner_id = $1 AND status = 'paid'), 0)::numeric(14,2)`,
		partnerID).Scan(&pending, &settled, &paid)
	if settled < 0 {
		settled = 0
	}
	return financeSummary{PendingEarnings: pending, SettledEarnings: settled, PaidSettlements: paid}
}

func (d *Deps) partnerServiceChargesTotal(ctx context.Context, partnerID string) float64 {
	var total float64
	_ = d.Pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(amount_idr), 0)::numeric(14,2) FROM partner_service_charges WHERE partner_id = $1",
		partnerID).Scan(&total)
	return total
}

// Potongan platform atas penghasilan mitra per publikasi (aturan pemilik
// produk: 5% dari angka yang diajukan). Dibagi di sini, bukan per-tampilan,
// supaya popover rincian dan angka saldo selalu berasal dari hitungan sama.
const pePlatformCutPercent = 5.0

// earningGross: gross_amount dari baris ledger; baris lama tanpa gross jatuh
// kembali ke amount (tidak ada potongan yang bisa ditampilkan).
func earningGross(row rowMap) float64 {
	if v, ok := row.num("gross_amount"); ok {
		return v
	}
	return row.numOr("amount")
}

// pePlatformCutPercentFor: persentase potongan yang benar-benar berlaku pada
// satu earning. Kalau gross kosong/lebih kecil dari amount (baris historis
// sebelum kolom gross_amount ada) hasilnya 0 — jangan mengarang potongan.
func pePlatformCutPercentFor(amount, gross float64) float64 {
	if gross <= 0 || amount >= gross {
		return 0
	}
	return round2(((gross - amount) / gross) * 100)
}

// splitRevenue: port lib/finance/revenue-share.ts splitRevenue (default 25% mitra).
func splitRevenue(clientPrice, partnerPrice, partnerPercent float64) float64 {
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
	percent := partnerPercent
	if percent <= 0 {
		percent = 25
	}
	if percent > 100 {
		percent = 100
	}
	amount := round2(base + (pool*percent)/100)
	if amount > price {
		amount = price
	}
	return amount
}

// ---------- sealing (port sealSecret t2 / sealSecretV3 t3) ----------

// sealT2: parity sealSecretValue — v2 (t2:) dengan cmsEncryptionSecret.
func (d *Deps) sealT2(plain string) (string, error) {
	secret := d.Cfg.CmsEncryptionSecret
	if secret == "" {
		secret = d.Cfg.Secret
	}
	return crypto.Seal(plain, secret)
}

// unsealT2: parity unsealSecretValue — terima t2 (GCM) dan v1 legacy.
func (d *Deps) unsealT2(sealed string) (string, error) {
	secret := d.Cfg.CmsEncryptionSecret
	if secret == "" {
		secret = d.Cfg.Secret
	}
	return crypto.Unseal(sealed, secret)
}

// ---------- slug + enum (port app/v1/platform/media/_media.ts) ----------

func slugifyMediaName(name string) string {
	lower := strings.ToLower(name)
	var b strings.Builder
	lastDash := false
	for _, ch := range lower {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 160 {
		out = out[:160]
	}
	return out
}

var mediaIntegrationTypes = []string{"manual", "wordpress", "rss", "webhook", "tulisio_api", "custom_web", "blogger", "emdash"}

func validIntegrationType(v string) bool {
	for _, t := range mediaIntegrationTypes {
		if t == v {
			return true
		}
	}
	return false
}

var wordCountModes = map[string]bool{
	"sesuai_permintaan": true, "kata_300_700": true, "kata_800_1500": true,
}

const defaultWordCountMode = "sesuai_permintaan"

func normalizeWordCountMode(v *string) string {
	if v == nil {
		return defaultWordCountMode
	}
	if wordCountModes[*v] {
		return *v
	}
	return defaultWordCountMode
}

// ---------- JSON kecil ----------

func decodeJSON(r *http.Request) map[string]any {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		return map[string]any{}
	}
	return body
}

func strField(body map[string]any, key string) (string, bool) {
	v, ok := body[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(s), true
}

func boolField(body map[string]any, key string) (bool, bool) {
	v, ok := body[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	if !ok {
		return false, false
	}
	return b, true
}

func numField(body map[string]any, key string) (float64, bool) {
	v, ok := body[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func intField(body map[string]any, key string) (int, bool) {
	f, ok := numField(body, key)
	if !ok {
		return 0, false
	}
	return int(f), true
}

func stringSliceField(body map[string]any, key string) ([]string, bool) {
	v, ok := body[key]
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			continue
		}
		t := strings.TrimSpace(s)
		if t != "" {
			out = append(out, t)
		}
	}
	return out, true
}

func errf(format string, args ...any) *PartnerError {
	return &PartnerError{http.StatusInternalServerError, fmt.Sprintf(format, args...)}
}
