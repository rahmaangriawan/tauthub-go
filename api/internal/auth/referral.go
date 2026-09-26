package auth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Port lib/referral/service.ts attributeReferral — atribusi first-valid
// referrer + record attribution. Semua best-effort branch dipertahankan.

func AttributeReferral(ctx context.Context, pool *pgxpool.Pool, newUserID string, code string, opts ReferralOpts) *string {
	if code == "" {
		return nil
	}
	referrerPartnerID, referrerUserID, found := ResolveReferralCode(ctx, pool, code)
	if !found {
		return nil
	}
	if !opts.SkipSelfCheck && referrerUserID == newUserID {
		return nil
	}

	var newUserEmail string
	if err := pool.QueryRow(ctx, "SELECT email FROM users WHERE id = $1 LIMIT 1", newUserID).Scan(&newUserEmail); err != nil {
		return nil
	}
	if !opts.SkipSelfCheck && referrerUserID != "" {
		var refUserEmail string
		if err := pool.QueryRow(ctx, "SELECT email FROM users WHERE id = $1 LIMIT 1", referrerUserID).Scan(&refUserEmail); err == nil {
			if strings.ToLower(refUserEmail) == strings.ToLower(newUserEmail) {
				return nil
			}
		}
	}

	var currentRef *string
	if err := pool.QueryRow(ctx, "SELECT referred_by_partner_id FROM users WHERE id = $1 LIMIT 1", newUserID).Scan(&currentRef); err != nil {
		return nil
	}
	if currentRef != nil {
		return nil
	}

	// Deteksi self-referral pattern via hash device/IP → atribusi tetap dicatat
	// tapi ditandai fraud di notes (komisi jadi review_needed).
	var fraudNotes []string
	if referrerUserID != "" && (opts.DeviceHash != nil || opts.IPHash != nil) {
		var refDevice, refIP *string
		if err := pool.QueryRow(ctx,
			"SELECT ip_hash, device_hash FROM registration_history WHERE user_id = $1::text ORDER BY registered_at DESC LIMIT 1",
			referrerUserID).Scan(&refIP, &refDevice); err == nil {
			deviceMatch := opts.DeviceHash != nil && refDevice != nil && *opts.DeviceHash == *refDevice
			ipMatch := opts.IPHash != nil && refIP != nil && *opts.IPHash == *refIP
			if deviceMatch || ipMatch {
				parts := []string{}
				if deviceMatch {
					parts = append(parts, "device")
				}
				if ipMatch {
					parts = append(parts, "ip")
				}
				fraudNotes = append(fraudNotes, "fraud: self-referral pattern ("+strings.Join(parts, "+")+")")
			}
		} else if err != pgx.ErrNoRows {
			slog.Debug("referral history lookup gagal", "err", err)
		}
	}

	var recent int
	_ = pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM referral_attributions WHERE code = $1 AND created_at > NOW() - INTERVAL '1 hour'",
		NormalizeReferralCode(code)).Scan(&recent)
	if recent >= 20 {
		fraudNotes = append(fraudNotes, "fraud: burst >20/h")
	}
	var notes *string
	if len(fraudNotes) > 0 {
		joined := strings.Join(fraudNotes, " | ")
		notes = &joined
	}

	_, _ = pool.Exec(ctx,
		"UPDATE users SET referred_by_partner_id = $1 WHERE id = $2 AND referred_by_partner_id IS NULL",
		referrerPartnerID, newUserID)

	// click_id: fingerprint ip|ua (maks 200) bila ada klik tercatat.
	var clickID *string
	fingerprint := deref(opts.IP) + "|" + deref(opts.UserAgent)
	if len(fingerprint) > 200 {
		fingerprint = fingerprint[:200]
	}
	if fingerprint == "|" {
		fingerprint = "fp:" + NormalizeReferralCode(code) + ":unknown"
	}
	var foundClick string
	if err := pool.QueryRow(ctx,
		"SELECT id FROM referral_clicks WHERE code = $1 AND fingerprint = $2 LIMIT 1",
		NormalizeReferralCode(code), fingerprint).Scan(&foundClick); err == nil && foundClick != "" {
		clickID = &foundClick
	}

	attribID := uuid.NewString()
	if _, err := pool.Exec(ctx,
		"INSERT INTO referral_attributions (id, code, referrer_partner_id, referee_user_id, ip, user_agent, notes, click_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		attribID, NormalizeReferralCode(code), referrerPartnerID, newUserID, opts.IP, opts.UserAgent, notes, clickID); err != nil {
		_, _ = pool.Exec(ctx,
			"INSERT INTO referral_attributions (id, code, referrer_partner_id, referee_user_id, ip, user_agent, notes) VALUES ($1, $2, $3, $4, $5, $6, $7)",
			attribID, NormalizeReferralCode(code), referrerPartnerID, newUserID, opts.IP, opts.UserAgent, notes)
	}

	return &referrerPartnerID
}

type ReferralOpts struct {
	IP            *string
	UserAgent     *string
	IPHash        *string
	DeviceHash    *string
	SkipSelfCheck bool
}

