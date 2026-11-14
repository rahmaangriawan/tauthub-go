package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Port lib/audit/log.ts + lib/public-id.ts + lib/auth/registration-guard/*.
// Semua perilaku fail-safe & pesan dipertahankan (parity).

// WriteAuditLog: best-effort — kegagalan mencatat tidak boleh menggagalkan aksi.
func WriteAuditLog(ctx context.Context, pool *pgxpool.Pool, input AuditLogInput) {
	auditID, err := uuid.NewRandom()
	if err != nil {
		return
	}
	_, _ = pool.Exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id, ip_address, user_agent) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		auditID.String(), input.BrandID, input.ActorUserID, input.Action, input.TargetType, input.TargetID, input.IPAddress, input.UserAgent)
}

type AuditLogInput struct {
	ActorUserID *string
	Action      string
	TargetType  *string
	TargetID    *string
	BrandID     *string
	IPAddress   *string
	UserAgent   *string
}

// ---------- public-id.ts ----------

// Slugify untuk kolom `slug` brands (BUKAN public_id). Parity slugify():
// lowercase, buang non-alnum jadi spasi, collapse separator, trim.
func Slugify(value string) string {
	lower := strings.ToLower(value)
	var b strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

const idAlphabet = "23456789abcdefghjkmnpqrstuvwxyz" // tanpa 0/o, 1/l/i, u

// BuildShortPublicId: 6 karakter acak (parity buildShortPublicId).
func BuildShortPublicId() string {
	bytes := make([]byte, 6)
	_, _ = rand.Read(bytes)
	out := make([]byte, 6)
	for i, b := range bytes {
		out[i] = idAlphabet[int(b)%len(idAlphabet)]
	}
	return string(out)
}

// ---------- registration-guard/signals.ts ----------

var disposableDomains = map[string]bool{
	"mailinator.com": true, "10minutemail.com": true, "guerrillamail.com": true, "guerrillamail.net": true,
	"tempmail.com": true, "temp-mail.org": true, "throwawaymail.com": true, "yopmail.com": true,
	"sharklasers.com": true, "getnada.com": true, "dispostable.com": true, "maildrop.cc": true,
	"trashmail.com": true, "fakeinbox.com": true, "mailnesia.com": true, "mytemp.email": true,
	"spamgourmet.com": true, "discard.email": true, "mailcatch.com": true, "mintemail.com": true,
	"mail7.io": true, "emailondeck.com": true, "tempmailo.com": true, "moakt.com": true,
	"tempinbox.com": true, "mohmal.com": true, "emailtemporanea.net": true, "mailtemp.net": true,
	"grr.la": true, "inboxbear.com": true, "spam4.me": true, "1secmail.com": true,
	"1secmail.net": true, "mvrht.net": true, "tmail.ws": true, "vomoto.com": true,
	"instantemailaddress.com": true, "harakirimail.com": true, "mailsac.com": true, "tempmailaddress.com": true,
}

// GmailLocalPartValid: bagian sebelum "@" harus mungkin ada di Gmail. Gmail hanya
// mengizinkan huruf, angka, dan titik (maks 30 karakter, tanpa titik di awal/akhir
// atau dua titik berurutan); "+tag" diabaikan. Akun baru minimal 6 karakter,
// tetapi akun lama bisa lebih pendek, jadi batas bawah sengaja dilonggarkan ke 3
// agar pengguna asli tidak ikut tertolak. Hanya menolak alamat yang pasti
// mustahil, bukan memastikan alamat itu benar-benar ada.
func GmailLocalPartValid(email string) bool {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return false
	}
	local := strings.ToLower(email[:at])
	if i := strings.IndexByte(local, '+'); i >= 0 {
		local = local[:i]
	}
	if len(local) < 3 || len(local) > 30 {
		return false
	}
	if local[0] == '.' || local[len(local)-1] == '.' || strings.Contains(local, "..") {
		return false
	}
	for i := 0; i < len(local); i++ {
		c := local[i]
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '.' {
			return false
		}
	}
	return true
}

type EmailAnalysis struct {
	Domain       string
	IsGmail      bool
	IsDisposable bool
	ReasonCodes  []string
}

func AnalyzeEmail(email string) EmailAnalysis {
	domain := ""
	if at := strings.LastIndex(email, "@"); at != -1 {
		domain = strings.ToLower(strings.TrimSpace(email[at+1:]))
	}
	return EmailAnalysis{
		Domain:       domain,
		IsGmail:      domain == "gmail.com" || domain == "googlemail.com",
		IsDisposable: disposableDomains[domain],
	}
}

// HashSignal: HMAC-SHA256 hex dengan pepper (parity hashSignal; PII tak pernah mentah).
func HashSignal(value string, pepper string) *string {
	if value == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(value))
	out := hex.EncodeToString(mac.Sum(nil))
	return &out
}

// RequestDeviceSignal: cookie reg_device + UA (maks 256 char, lowercase).
func RequestDeviceSignal(userAgent string, cookieDeviceID string) string {
	ua := strings.TrimSpace(strings.ToLower(userAgent))
	if len(ua) > 256 {
		ua = ua[:256]
	}
	if ua == "" {
		ua = "no-ua"
	}
	device := strings.TrimSpace(strings.ToLower(cookieDeviceID))
	if device == "" {
		device = "no-device"
	}
	return device + "|" + ua
}

var (
	mxMu    sync.Mutex
	mxCache = map[string]struct {
		value     *bool
		expiresAt time.Time
	}{}
)

// HasMxRecord: null = unknown (DNS gagal) — tidak pernah throw. Cache 5 menit.
func HasMxRecord(domain string) *bool {
	if domain == "" {
		return nil
	}
	mxMu.Lock()
	if c, ok := mxCache[domain]; ok && c.expiresAt.After(time.Now()) {
		mxMu.Unlock()
		return c.value
	}
	mxMu.Unlock()
	records, err := net.LookupMX(domain)
	var value *bool
	if err != nil {
		value = nil // unknown — jangan blokir berdasarkan kegagalan DNS
	} else {
		v := len(records) > 0
		value = &v
	}
	mxMu.Lock()
	mxCache[domain] = struct {
		value     *bool
		expiresAt time.Time
	}{value, time.Now().Add(5 * time.Minute)}
	mxMu.Unlock()
	return value
}

// ---------- registration-guard/config.ts ----------

type GuardScores struct {
	Disposable              int
	KnownAbusiveDomain      int
	VeryNewDomain           int
	Workspace               int
	IPIfManyRegistrations   int
	DeviceManyRegistrations int
	RecentRegistration      int
	SuspiciousPattern       int
	PreviouslyBlocked       int
	ExistingGoogleSub       int
	MxMissing               int
	ReferralDeviceMatch     int
	ReferralIPMatch         int
	ReferralBurst           int
}

type GuardThresholds struct {
	Verify   int
	Review   int
	Block    int
	Critical int
}

// GuardRateLimits: bagian rate_limits dari registration_guard_config yang
// dipakai guard. DomainPerDay membatasi pendaftaran per domain email non-Gmail
// dalam 24 jam (Workspace massal).
type GuardRateLimits struct {
	DomainPerDay int
}

type GuardConfig struct {
	Scores     GuardScores
	Thresholds GuardThresholds
	RateLimits GuardRateLimits
}

func defaultGuardConfig() GuardConfig {
	return GuardConfig{
		Scores: GuardScores{
			Disposable: 50, KnownAbusiveDomain: 70, VeryNewDomain: 20, Workspace: 10,
			IPIfManyRegistrations: 20, DeviceManyRegistrations: 25, RecentRegistration: 100,
			SuspiciousPattern: 30, PreviouslyBlocked: 70, ExistingGoogleSub: 100, MxMissing: 15,
			ReferralDeviceMatch: 40, ReferralIPMatch: 25, ReferralBurst: 20,
		},
		Thresholds: GuardThresholds{Verify: 30, Review: 60, Block: 80, Critical: 110},
		RateLimits: GuardRateLimits{DomainPerDay: 3},
	}
}

type guardConfigCache struct {
	cfg       GuardConfig
	expiresAt time.Time
}

var (
	guardCfgMu    sync.Mutex
	guardCfgCache *guardConfigCache
)

// LoadGuardConfig: singleton DB + cache 60s; fail-open ke default (parity).
func LoadGuardConfig(ctx context.Context, pool *pgxpool.Pool) GuardConfig {
	guardCfgMu.Lock()
	defer guardCfgMu.Unlock()
	if guardCfgCache != nil && guardCfgCache.expiresAt.After(time.Now()) {
		return guardCfgCache.cfg
	}
	cfg := defaultGuardConfig()
	var scores, thresholds, rateLimits []byte
	err := pool.QueryRow(ctx,
		"SELECT scores, thresholds, rate_limits FROM registration_guard_config WHERE id = 1 LIMIT 1",
	).Scan(&scores, &thresholds, &rateLimits)
	if err == nil {
		var s map[string]float64
		var t map[string]float64
		var rl map[string]float64
		_ = json.Unmarshal(scores, &s)
		_ = json.Unmarshal(thresholds, &t)
		_ = json.Unmarshal(rateLimits, &rl)
		applyNumbers(rl, map[string]*int{"domain_per_day": &cfg.RateLimits.DomainPerDay})
		applyNumbers(s, map[string]*int{
			"disposable": &cfg.Scores.Disposable, "known_abusive_domain": &cfg.Scores.KnownAbusiveDomain,
			"very_new_domain": &cfg.Scores.VeryNewDomain, "workspace": &cfg.Scores.Workspace,
			"ip_many_registrations": &cfg.Scores.IPIfManyRegistrations, "device_many_registrations": &cfg.Scores.DeviceManyRegistrations,
			"recent_registration": &cfg.Scores.RecentRegistration, "suspicious_pattern": &cfg.Scores.SuspiciousPattern,
			"previously_blocked": &cfg.Scores.PreviouslyBlocked, "existing_google_sub": &cfg.Scores.ExistingGoogleSub,
			"mx_missing": &cfg.Scores.MxMissing, "referral_device_match": &cfg.Scores.ReferralDeviceMatch,
			"referral_ip_match": &cfg.Scores.ReferralIPMatch, "referral_burst": &cfg.Scores.ReferralBurst,
		})
		applyNumbers(t, map[string]*int{
			"verify": &cfg.Thresholds.Verify, "review": &cfg.Thresholds.Review,
			"block": &cfg.Thresholds.Block, "critical": &cfg.Thresholds.Critical,
		})
	} else {
		slog.Warn("[registration-guard] config gagal dimuat, pakai default", "err", err)
	}
	guardCfgCache = &guardConfigCache{cfg: cfg, expiresAt: time.Now().Add(60 * time.Second)}
	return cfg
}

// InvalidateGuardConfigCache: dipanggil setelah admin mengubah konfigurasi.
func InvalidateGuardConfigCache() {
	guardCfgMu.Lock()
	guardCfgCache = nil
	guardCfgMu.Unlock()
}

func applyNumbers(src map[string]float64, dst map[string]*int) {
	for k, v := range src {
		if p, ok := dst[k]; ok && v == float64(int64(v)) {
			*p = int(v)
		}
	}
}
