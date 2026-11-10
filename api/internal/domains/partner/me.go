package partner

import (
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- GET/PATCH /v1/partner/me ----------

const partnerMeSelect = `SELECT p.*,
         (SELECT COUNT(*) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden') AS media_count,
         (SELECT COUNT(*) FROM publications pb
            JOIN media m ON m.id = pb.media_id
            WHERE m.partner_id = p.id AND pb.status = 'waiting_partner') AS pending_count,
         (SELECT COUNT(*) FROM publications pb
            JOIN media m ON m.id = pb.media_id
            WHERE m.partner_id = p.id AND pb.status = 'published' AND pb.publish_status = 'draft') AS draft_delivered,
         (SELECT COUNT(*) FROM publications pb
            JOIN media m ON m.id = pb.media_id
            WHERE m.partner_id = p.id AND pb.status = 'published' AND pb.publish_status = 'publish') AS auto_published,
         (SELECT COUNT(*) FROM publications pb
            JOIN media m ON m.id = pb.media_id
            WHERE m.partner_id = p.id AND pb.status = 'failed') AS failed_count,
         COALESCE((SELECT nn.wa_number FROM notification_preferences nn WHERE nn.user_id = p.user_id LIMIT 1), '') AS wa_number,
         COALESCE((SELECT nn.wa_consent FROM notification_preferences nn WHERE nn.user_id = p.user_id LIMIT 1), FALSE) AS wa_consent
       FROM partners p`

func partnerMeJSON(row rowMap) map[string]any {
	if row == nil {
		return nil
	}
	mediaCount := row.intOr("media_count", 0)
	pendingCount := row.intOr("pending_count", 0)
	waNumber := strings.TrimSpace(row.strOr("wa_number", ""))
	waConsent := row.boolOr("wa_consent", false)
	// Mitra wajib melengkapi nomor WA begitu medianya lebih dari satu.
	waRequired := mediaCount > waMediaThreshold && (waNumber == "" || !waConsent)
	return map[string]any{
		"id":                            row.raw("id"),
		"public_id":                     row.raw("public_id"),
		"name":                          row.raw("name"),
		"category":                      row.raw("category"),
		"status":                        row.raw("status"),
		"payout_type":                   row.strOr("payout_type", "bank"),
		"bank_name":                     row.raw("bank_name"),
		"bank_account_number":           row.raw("bank_account_number"),
		"bank_account_name":             row.raw("bank_account_name"),
		"media_count":                   mediaCount,
		"pending_count":                 pendingCount,
		"wa_number_set":                 waNumber != "",
		"wa_consent":                    waConsent,
		"wa_required":                   waRequired,
		"integration_terms_accepted_at": row.timePtr("integration_terms_accepted_at"),
		"publication_stats": map[string]any{
			"draft_delivered": row.intOr("draft_delivered", 0),
			"auto_published":  row.intOr("auto_published", 0),
			"failed":          row.intOr("failed_count", 0),
			"waiting":         pendingCount,
		},
		"created_at": row.timePtr("created_at"),
	}
}

func (d *Deps) Me(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := d.queryRow(r.Context(), partnerMeSelect+" WHERE p.user_id = $1 AND p.status = 'active' LIMIT 1", user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	resp := map[string]any{"user": map[string]any{"id": user.ID, "email": user.Email}}
	payload := partnerMeJSON(row)
	// publication_counts: sebaran seluruh 11 status publikasi media mitra
	// (read-only, untuk distribusi status di dashboard Ringkasan). Field baru —
	// tidak mengubah respons lama.
	publisherID := row.strOr("id", "")
	if publisherID != "" {
		counts, countErr := d.queryRows(r.Context(), `
			SELECT p.status, COUNT(*)::int AS count
			FROM publications p
			JOIN media m ON m.id = p.media_id
			WHERE m.partner_id = $1
			GROUP BY p.status`, publisherID)
		if countErr == nil {
			countMap := make(map[string]any, len(counts))
			for _, c := range counts {
				countMap[c.strOr("status", "")] = c.intOr("count", 0)
			}
			payload["publication_counts"] = countMap
		}
	}
	resp["partner"] = payload
	w.Header().Set("Cache-Control", "private, max-age=5, stale-while-revalidate=30")
	writeJSON(w, http.StatusOK, resp)
}

func (d *Deps) MeUpdate(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	body := decodeJSON(r)
	sets := []string{}
	args := []any{}

	addSet := func(clause string, value any) {
		args = append(args, value)
		sets = append(sets, clause+" = $"+itoa(len(args)))
	}

	if name, present := strField(body, "name"); present {
		if name == "" {
			httpx.Error(w, http.StatusBadRequest, "Nama mitra wajib diisi")
			return
		}
		addSet("name", name)
	}
	if payoutType, present := strField(body, "payout_type"); present {
		if payoutType != "bank" && payoutType != "ewallet" && payoutType != "manual" {
			httpx.Error(w, http.StatusBadRequest, "Metode payout tidak valid")
			return
		}
		addSet("payout_type", payoutType)
	}
	if bankName, present := strField(body, "bank_name"); present {
		addSet("bank_name", ptrOrNil(bankName))
	}
	if accountNumber, present := strField(body, "bank_account_number"); present {
		if accountNumber != "" && !isDigits(accountNumber) {
			httpx.Error(w, http.StatusBadRequest, "Nomor rekening / nomor HP hanya boleh berisi angka (0-9).")
			return
		}
		addSet("bank_account_number", ptrOrNil(accountNumber))
	}
	if accountName, present := strField(body, "bank_account_name"); present {
		addSet("bank_account_name", ptrOrNil(accountName))
	}
	// Tujuan payout hanya boleh berubah dengan konfirmasi password, dan tidak
	// saat masih ada penarikan berjalan: admin membaca rekening langsung dari
	// baris mitra, jadi mengubahnya akan mengalihkan settlement yang menunggu.
	_, hasType := strField(body, "payout_type")
	_, hasBank := strField(body, "bank_name")
	_, hasNumber := strField(body, "bank_account_number")
	_, hasHolder := strField(body, "bank_account_name")
	if hasType || hasBank || hasNumber || hasHolder {
		cur, curErr := d.queryRow(r.Context(),
			"SELECT payout_type, bank_name, bank_account_number, bank_account_name FROM partners WHERE id = $1 LIMIT 1", partner.ID)
		if curErr != nil || cur == nil {
			httpx.Error(w, http.StatusInternalServerError, "Gagal membaca pengaturan payout.")
			return
		}
		differs := func(field, key string, present bool) bool {
			if !present {
				return false
			}
			v, _ := strField(body, key)
			return strings.TrimSpace(v) != strings.TrimSpace(cur.strOr(field, ""))
		}
		if differs("payout_type", "payout_type", hasType) || differs("bank_name", "bank_name", hasBank) ||
			differs("bank_account_number", "bank_account_number", hasNumber) || differs("bank_account_name", "bank_account_name", hasHolder) {
			pw, _ := strField(body, "current_password")
			if pw == "" {
				httpx.Error(w, http.StatusUnprocessableEntity, "Password wajib diisi untuk mengubah rekening penarikan.")
				return
			}
			var hash string
			if err := d.Pool.QueryRow(r.Context(), "SELECT password_hash FROM users WHERE id = $1 LIMIT 1", user.ID).Scan(&hash); err != nil || !auth.VerifyPassword(pw, hash) {
				httpx.Error(w, http.StatusUnauthorized, "Password salah.")
				return
			}
			busy, _ := d.queryRow(r.Context(),
				"SELECT id FROM settlements WHERE partner_id = $1 AND status IN ('pending','approved','processing') LIMIT 1", partner.ID)
			if busy != nil {
				httpx.Error(w, http.StatusConflict, "Rekening tidak bisa diubah selama masih ada penarikan yang diproses. Tunggu sampai selesai.")
				return
			}
		}
	}
	// Mitra hanya boleh MEMATIKAN RSS (pengaktifan hanya oleh Admin).
	if rssEnabled, present := boolField(body, "rss_enabled"); present {
		if rssEnabled {
			httpx.Error(w, http.StatusConflict, "Pengaktifan Sindikasi Konten (RSS) hanya dapat dilakukan oleh Admin.")
			return
		}
		addSet("rss_enabled", false)
	}
	if accept, present := boolField(body, "accept_integration_terms"); present && accept {
		sets = append(sets, "integration_terms_accepted_at = COALESCE(integration_terms_accepted_at, NOW())")
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at = NOW()")
		args = append(args, partner.ID)
		sql := "UPDATE partners SET " + strings.Join(sets, ", ") + " WHERE id = $" + itoa(len(args))
		if err := d.exec(r.Context(), sql, args...); err != nil {
			d.writeErr(w, err)
			return
		}
		InvalidatePartnerCache(partner.UserID)
	}

	row, err := d.queryRow(r.Context(), partnerMeSelect+" WHERE p.id = $1 LIMIT 1", partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	payload := partnerMeJSON(row)
	if row != nil {
		payload["rss_enabled"] = row.boolOr("rss_enabled", false)
	}
	writeJSON(w, http.StatusOK, map[string]any{"partner": payload})
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// ---------- GET /v1/partner/me/vip ----------

type vipInfo struct {
	PartnerID         string
	IsVip             bool
	HasVip            bool
	Tier              string
	IsExpired         bool
	VipExpiresAt      any
	VipContractMonths any
	VipContractDays   any
	Entitlements      map[string]bool
	CommissionRateBp  int
	CpaRate           int
	CpaTierThreshold  int
	CpaTier1Rate      int
	CpaTier2Rate      int
}

const (
	cpaTierThresholdDefault = 10
	cpaTier1RateDefault     = 10000
	cpaTier2RateDefault     = 15000
)

var vipEntitlementKeys = []string{"client_workspace", "ai_premium", "seo_tools", "keyword_research"}

// getPartnerVipInfo: port getPartnerVipInfo — auto-expiry VIP (self-healing).
func (d *Deps) getPartnerVipInfo(r *http.Request, partnerID string) vipInfo {
	row, err := d.queryRow(r.Context(),
		"SELECT is_vip, vip_tier, vip_enabled_at, vip_contract_months, vip_contract_days, vip_expires_at FROM partners WHERE id = $1 LIMIT 1",
		partnerID)
	info := vipInfo{
		PartnerID:        partnerID,
		Tier:             "regular",
		Entitlements:     map[string]bool{},
		CpaTierThreshold: cpaTierThresholdDefault,
		CpaTier1Rate:     cpaTier1RateDefault,
		CpaTier2Rate:     cpaTier2RateDefault,
	}
	for _, k := range vipEntitlementKeys {
		info.Entitlements[k] = false
	}
	if err != nil || row == nil {
		return info
	}
	rawIsVip := row.boolOr("is_vip", false)
	info.Tier = row.strOr("vip_tier", "regular")
	var expiresAt *time.Time
	if t, ok := row.raw("vip_expires_at").(time.Time); ok {
		expiresAt = &t
	}
	info.IsExpired = rawIsVip && expiresAt != nil && !expiresAt.After(time.Now())
	if info.IsExpired {
		_ = d.exec(r.Context(),
			"UPDATE partners SET is_vip = false, vip_tier = 'regular', updated_at = NOW() WHERE id = $1 AND is_vip = true",
			partnerID)
	}
	isVip := rawIsVip && !info.IsExpired
	hasVip := rawIsVip || info.Tier == "brand_ambassador" || info.Tier == "vip_reguler" || row.has("vip_enabled_at")
	info.IsVip = isVip
	info.HasVip = hasVip
	info.VipExpiresAt = row.timePtr("vip_expires_at")
	if hasVip {
		info.VipContractMonths = row.raw("vip_contract_months")
		info.VipContractDays = row.raw("vip_contract_days")
	}

	commissionBp := 100
	cpaRate := 0
	if isVip {
		commissionBp = 500
		cpaRate = 15000
	}
	entitlements := map[string]bool{}
	for _, k := range vipEntitlementKeys {
		entitlements[k] = isVip
	}
	if settings, err := d.queryRow(r.Context(),
		`SELECT commission_rate_bp, cpa_rate, cpa_tier_threshold, cpa_tier1_rate, cpa_tier2_rate, entitlements
		 FROM affiliate_settings WHERE partner_id = $1 LIMIT 1`, partnerID); err == nil && settings != nil {
		if v, ok := settings.num("commission_rate_bp"); ok {
			commissionBp = int(v)
		}
		if v, ok := settings.num("cpa_rate"); ok {
			cpaRate = int(v)
		}
		if v, ok := settings.num("cpa_tier_threshold"); ok {
			info.CpaTierThreshold = int(v)
		}
		if v, ok := settings.num("cpa_tier1_rate"); ok {
			info.CpaTier1Rate = int(v)
		}
		if v, ok := settings.num("cpa_tier2_rate"); ok {
			info.CpaTier2Rate = int(v)
		}
		if raw := settings.raw("entitlements"); raw != nil {
			if m, ok := raw.(map[string]any); ok {
				for _, k := range vipEntitlementKeys {
					if v, present := m[k]; present {
						if b, isBool := v.(bool); isBool {
							entitlements[k] = b
						}
					}
				}
			}
		}
	}
	info.CommissionRateBp = commissionBp
	info.CpaRate = cpaRate
	info.Entitlements = entitlements
	return info
}

func (d *Deps) MeVip(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	info := d.getPartnerVipInfo(r, partner.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"partnerId":         info.PartnerID,
		"isVip":             info.IsVip,
		"hasVip":            info.HasVip,
		"tier":              info.Tier,
		"isExpired":         info.IsExpired,
		"vipExpiresAt":      info.VipExpiresAt,
		"vipContractMonths": info.VipContractMonths,
		"vipContractDays":   info.VipContractDays,
		"entitlements":      info.Entitlements,
		"commissionRateBp":  info.CommissionRateBp,
		"cpaRate":           info.CpaRate,
		"cpaTierThreshold":  info.CpaTierThreshold,
		"cpaTier1Rate":      info.CpaTier1Rate,
		"cpaTier2Rate":      info.CpaTier2Rate,
	})
}
