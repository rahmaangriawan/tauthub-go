package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Port lib/auth/registration-guard/guard.ts + history.ts — engine keputusan
// registrasi. Fail-safe: error internal → "review" (medium), tidak pernah throw.

const AdminRequiredMessage = "Pendaftaran menggunakan email perusahaan/domain tidak tersedia melalui pendaftaran umum. Silakan hubungi administrator Tauthub untuk proses pendaftaran dan verifikasi akun."

const GenericBlockMessage = "Pendaftaran tidak dapat dilanjutkan. Silakan coba kembali nanti atau hubungi administrator jika Anda membutuhkan bantuan."

type GuardDecisionKind string

const (
	GuardAllow         GuardDecisionKind = "allow"
	GuardVerify        GuardDecisionKind = "verify"
	GuardReview        GuardDecisionKind = "review"
	GuardBlock         GuardDecisionKind = "block"
	GuardAdminRequired GuardDecisionKind = "admin_required"
)

type GuardDecision struct {
	Decision      GuardDecisionKind
	RiskScore     int
	RiskLevel     string
	ReasonCodes   []string
	CustomMessage string
	Confidence    int // 1 = keputusan penuh; 0 = fail-safe error
}

type GuardInput struct {
	Email        string
	GoogleSub    string
	GoogleHD     string
	GoogleVerified bool
	IPHash       *string
	DeviceHash   *string
	ReferralCode *string
	Source       string // public | google | admin
}

// ResolveReferralCode: port referral/service.ts resolveReferralCode.
func ResolveReferralCode(ctx context.Context, pool *pgxpool.Pool, code string) (partnerID, referrerUserID string, ok bool) {
	var id, userID, publicID interface{}
	err := pool.QueryRow(ctx,
		"SELECT id, user_id, referral_code, public_id FROM partners WHERE referral_code_normalized = $1 LIMIT 1",
		NormalizeReferralCode(code),
	).Scan(&id, &userID, new(interface{}), new(interface{}))
	_ = publicID
	if err != nil {
		return "", "", false
	}
	idStr, _ := id.(string)
	uidStr, _ := userID.(string)
	return idStr, uidStr, true
}

// NormalizeReferralCode: port normalizeCode (trim + lower).
func NormalizeReferralCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

type guardSignals struct {
	IdentityPublicRegisteredAt *time.Time
	IPRegistrations30d         int
	DeviceRegistrations30d     int
	DistinctEmailHashes7d      int
	// DomainRegistrations24h: pendaftaran (termasuk percobaan Google yang
	// ditolak) dari domain email yang sama dalam 24 jam. 0 untuk Gmail.
	DomainRegistrations24h int
}

// CountDomainRegistrations24h: hitungan per domain untuk batas Workspace massal.
func CountDomainRegistrations24h(ctx context.Context, pool *pgxpool.Pool, domain string) int {
	var n int
	_ = pool.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM registration_history WHERE domain = $1 AND registered_at > NOW() - INTERVAL '24 hours'",
		strings.ToLower(domain)).Scan(&n)
	return n
}

// QuerySignals: 4 kueri velocity/identitas, best-effort → nol sinyal saat gagal.
func QuerySignals(ctx context.Context, pool *pgxpool.Pool, emailHash, googleSubHash, ipHash, deviceHash *string) guardSignals {
	var out guardSignals
	var lastAt *time.Time
	err := pool.QueryRow(ctx, `
		SELECT MAX(registered_at) FROM registration_history
		WHERE registration_source = 'public'
		  AND registered_at > NOW() - INTERVAL '365 days'
		  AND (($1::text IS NOT NULL AND email_hash = $1::text) OR ($2::text IS NOT NULL AND google_sub_hash = $2::text))`,
		emailHash, googleSubHash).Scan(&lastAt)
	if err == nil && lastAt != nil && lastAt.After(time.Now().Add(-365*24*time.Hour)) {
		out.IdentityPublicRegisteredAt = lastAt
	}
	_ = pool.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM registration_history WHERE ip_hash = $1::text AND registered_at > NOW() - INTERVAL '30 days'",
		ipHash).Scan(&out.IPRegistrations30d)
	_ = pool.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM registration_history WHERE device_hash = $1::text AND registered_at > NOW() - INTERVAL '30 days'",
		deviceHash).Scan(&out.DeviceRegistrations30d)
	_ = pool.QueryRow(ctx,
		"SELECT COUNT(DISTINCT email_hash)::int FROM registration_history WHERE device_hash = $1::text AND ip_hash = $2::text AND email_hash IS NOT NULL AND registered_at > NOW() - INTERVAL '7 days'",
		deviceHash, ipHash).Scan(&out.DistinctEmailHashes7d)
	return out
}

// RecordRegistration: best-effort insert registration_history (parity history.ts).
func RecordRegistration(ctx context.Context, pool *pgxpool.Pool, entry RegistrationHistoryEntry) {
	reasonCodes := "[]"
	if entry.ReasonCodes != nil {
		if b, err := json.Marshal(entry.ReasonCodes); err == nil {
			reasonCodes = string(b)
		}
	}
	riskLevel := entry.RiskLevel
	if riskLevel == "" {
		riskLevel = "low"
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO registration_history
		  (id, user_id, registration_source, email_hash, google_sub_hash, ip_hash, device_hash, domain, risk_score, risk_level, reason_codes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)`,
		uuid.NewString(), entry.UserID, entry.Source, entry.EmailHash, entry.GoogleSubHash, entry.IPHash, entry.DeviceHash,
		entry.Domain, entry.RiskScore, riskLevel, reasonCodes)
}

type RegistrationHistoryEntry struct {
	UserID        *string
	Source        string
	EmailHash     *string
	GoogleSubHash *string
	IPHash        *string
	DeviceHash    *string
	Domain        *string
	RiskScore     int
	RiskLevel     string
	ReasonCodes   []string
}

func decisionForScore(score int, t GuardThresholds) (GuardDecisionKind, string) {
	switch {
	case score >= t.Critical:
		return GuardBlock, "critical"
	case score >= t.Block:
		return GuardBlock, "high"
	case score >= t.Review:
		return GuardReview, "high"
	case score >= t.Verify:
		return GuardVerify, "medium"
	default:
		return GuardAllow, "low"
	}
}

// EvaluateRegistrationGuard: port evaluateRegistrationGuard — urutan sinyal
// dan skor harus identik. Fail-safe: panic/error apa pun → review (medium).
func EvaluateRegistrationGuard(ctx context.Context, pool *pgxpool.Pool, pepper string, input GuardInput) (decision GuardDecision) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("[registration-guard] panic, fail-safe review", "err", r)
			decision = GuardDecision{Decision: GuardReview, RiskScore: 0, RiskLevel: "medium", ReasonCodes: []string{"GUARD_ERROR"}, Confidence: 0}
		}
	}()

	d, err := evaluateGuardInner(ctx, pool, pepper, input)
	if err != nil {
		slog.Error("[registration-guard] error internal, fail-safe review", "err", err)
		return GuardDecision{Decision: GuardReview, RiskScore: 0, RiskLevel: "medium", ReasonCodes: []string{"GUARD_ERROR"}, Confidence: 0}
	}
	return d
}

func evaluateGuardInner(ctx context.Context, pool *pgxpool.Pool, pepper string, input GuardInput) (GuardDecision, error) {
	if input.Source == "admin" {
		return GuardDecision{Decision: GuardAllow, RiskScore: 0, RiskLevel: "low", ReasonCodes: []string{}, Confidence: 1}, nil
	}

	cfg := LoadGuardConfig(ctx, pool)
	emailInfo := AnalyzeEmailCtx(ctx, pool, input.Email)
	reasons := []string{}
	score := 0

	// Google tanpa email_verified: identitasnya tidak bisa dipercaya → blokir.
	if input.Source == "google" && !input.GoogleVerified {
		return GuardDecision{Decision: GuardBlock, RiskScore: cfg.Thresholds.Block, RiskLevel: "high",
			ReasonCodes: []string{"GOOGLE_EMAIL_UNVERIFIED"}, Confidence: 1}, nil
	}

	// Domain disposable selalu blokir (aturan keras).
	if emailInfo.IsDisposable {
		reasons = append(reasons, "DISPOSABLE_EMAIL")
		score += cfg.Scores.Disposable
	}

	// Non-Gmail lewat pendaftaran umum → alur khusus hubungi admin.
	if input.Source == "public" && !emailInfo.IsGmail && !emailInfo.IsDisposable && emailInfo.Domain != "" {
		return GuardDecision{Decision: GuardAdminRequired, RiskScore: score, RiskLevel: "low",
			ReasonCodes: append([]string{"ADMIN_REQUIRED"}, reasons...), CustomMessage: AdminRequiredMessage, Confidence: 1}, nil
	}

	// Google sub sudah terdaftar → sinyal saja tanpa skor.
	if input.GoogleSub != "" {
		var linkedUserID string
		err := pool.QueryRow(ctx,
			"SELECT user_id FROM user_oauth_accounts WHERE provider = 'google' AND provider_account_id = $1 LIMIT 1",
			input.GoogleSub).Scan(&linkedUserID)
		if err == nil {
			reasons = append(reasons, "GOOGLE_SUB_ALREADY_REGISTERED")
		}
	}

	emailHash := HashSignal(strings.ToLower(strings.TrimSpace(input.Email)), pepper)
	var googleSubHash *string
	if input.GoogleSub != "" {
		googleSubHash = HashSignal(input.GoogleSub, pepper)
	}
	signals := QuerySignals(ctx, pool, emailHash, googleSubHash, input.IPHash, input.DeviceHash)
	if !emailInfo.IsGmail && emailInfo.Domain != "" {
		signals.DomainRegistrations24h = CountDomainRegistrations24h(ctx, pool, emailInfo.Domain)
	}

	// Batas per domain non-Gmail (Workspace custom domain bisa dibuat massal):
	// melewati rate_limits.domain_per_day → minimal review (ditolak di jalur
	// pendaftaran baru). Dihitung sebelum skor lain supaya tidak bisa "ditebus".
	if cfg.RateLimits.DomainPerDay > 0 && signals.DomainRegistrations24h >= cfg.RateLimits.DomainPerDay {
		reasons = append(reasons, "DOMAIN_MANY_REGISTRATIONS")
		score += cfg.Thresholds.Review
	}

	// Domain tanpa MX — hanya untuk sumber Google.
	if input.Source == "google" && !emailInfo.IsGmail && emailInfo.Domain != "" {
		if mx := HasMxRecord(emailInfo.Domain); mx != nil && !*mx {
			reasons = append(reasons, "NO_MX_RECORD")
			score += cfg.Scores.MxMissing
		}
	}
	if input.GoogleSub != "" && input.GoogleHD != "" && input.GoogleHD != "gmail.com" {
		reasons = append(reasons, "GOOGLE_WORKSPACE")
		score += cfg.Scores.Workspace
	}
	if signals.IPRegistrations30d > 3 {
		reasons = append(reasons, "IP_MANY_REGISTRATIONS")
		score += cfg.Scores.IPIfManyRegistrations
	}
	if signals.DeviceRegistrations30d > 3 {
		reasons = append(reasons, "DEVICE_MANY_REGISTRATIONS")
		score += cfg.Scores.DeviceManyRegistrations
	}
	if signals.IdentityPublicRegisteredAt != nil {
		reasons = append(reasons, "RECENT_REGISTRATION")
		score += cfg.Scores.RecentRegistration
	}
	if signals.DistinctEmailHashes7d >= 3 {
		reasons = append(reasons, "SUSPICIOUS_PATTERN")
		score += cfg.Scores.SuspiciousPattern
	}

	// Sinyal self-referral (parity: gagal → dilewati, bukan fail-safe).
	if input.ReferralCode != nil && *input.ReferralCode != "" {
		referrerPartnerID, referrerUserID, found := ResolveReferralCode(ctx, pool, *input.ReferralCode)
		if found && referrerUserID != "" {
			_ = referrerPartnerID
			var refIPHash, refDeviceHash *string
			_ = pool.QueryRow(ctx,
				"SELECT ip_hash, device_hash FROM registration_history WHERE user_id = $1::text ORDER BY registered_at DESC LIMIT 1",
				referrerUserID).Scan(&refIPHash, &refDeviceHash)
			if input.DeviceHash != nil && refDeviceHash != nil && *input.DeviceHash == *refDeviceHash {
				reasons = append(reasons, "REFERRAL_DEVICE_MATCH")
				score += cfg.Scores.ReferralDeviceMatch
			}
			if input.IPHash != nil && refIPHash != nil && *input.IPHash == *refIPHash {
				reasons = append(reasons, "REFERRAL_IP_MATCH")
				score += cfg.Scores.ReferralIPMatch
			}
			var burst int
			_ = pool.QueryRow(ctx,
				"SELECT COUNT(*)::int FROM referral_attributions WHERE code = $1 AND created_at > NOW() - INTERVAL '24 hours'",
				NormalizeReferralCode(*input.ReferralCode)).Scan(&burst)
			if burst >= 3 {
				reasons = append(reasons, "REFERRAL_BURST")
				score += cfg.Scores.ReferralBurst
			}
		}
	}

	decision, riskLevel := decisionForScore(score, cfg.Thresholds)

	// Disposable dipaksa blokir walau skor di bawah threshold.
	if emailInfo.IsDisposable && decision != GuardBlock {
		lvl := riskLevel
		if lvl == "critical" {
			lvl = "critical"
		} else {
			lvl = "high"
		}
		return GuardDecision{Decision: GuardBlock, RiskScore: score, RiskLevel: lvl, ReasonCodes: reasons, Confidence: 1}, nil
	}

	return GuardDecision{Decision: decision, RiskScore: score, RiskLevel: riskLevel, ReasonCodes: reasons, Confidence: 1}, nil
}
