package authdomain

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/auth/register/route.ts — validasi, guard, anti-enumerasi,
// provisioning, referral, reg_device cookie: semuanya dipertahankan.

const regDeviceCookie = "reg_device"
const regDeviceMaxAge = 60 * 60 * 24 * 365

type registerBody struct {
	Email           string  `json:"email"`
	Password        string  `json:"password"`
	DisplayName     string  `json:"display_name"`
	Role            string  `json:"role"`
	PartnerCategory *string `json:"partner_category"`
	ReferralCode    *string `json:"referral_code"`
	TurnstileToken  *string `json:"turnstile_token"`
}

func readCookie(r *http.Request, name string) string {
	header := r.Header.Get("cookie")
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ";") {
		eq := strings.Index(part, "=")
		if eq == -1 {
			continue
		}
		if strings.TrimSpace(part[:eq]) == name {
			return strings.TrimSpace(part[eq+1:])
		}
	}
	return ""
}

func (d *Deps) setRegDeviceCookie(w http.ResponseWriter, deviceID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     regDeviceCookie,
		Value:    deviceID,
		Path:     "/",
		MaxAge:   regDeviceMaxAge,
		HttpOnly: true,
		Secure:   d.Cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (d *Deps) Register(w http.ResponseWriter, r *http.Request) {
	var body registerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := lower(trim(body.Email))
	password := body.Password
	displayName := trim(body.DisplayName)
	role := body.Role
	partnerCategory := body.PartnerCategory

	// parity registerSchema
	if !emailRe.MatchString(email) || len(email) > 320 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid email")
		return
	}
	if len(password) < 10 || len(password) > 128 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String should have at least 10 characters")
		return
	}
	if len(displayName) < 2 || len(displayName) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 2 character(s)")
		return
	}
	if role != "client" && role != "partner" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid enum value. Expected 'client' | 'partner'")
		return
	}
	if role == "partner" && (partnerCategory == nil || *partnerCategory == "") {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kategori media partner wajib dipilih")
		return
	}
	if partnerCategory != nil && *partnerCategory != "berita" && *partnerCategory != "lifestyle" && *partnerCategory != "lainnya" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kategori media partner tidak valid")
		return
	}

	// Referral: body → header x-referral-code (parity).
	referralCode := body.ReferralCode
	if referralCode == nil || *referralCode == "" {
		if h := r.Header.Get("x-referral-code"); h != "" {
			referralCode = &h
		}
	}
	ip := authRequestIp(r)

	// Rate limit per IP + per email (jendela 1 jam).
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "register:ip:"+orUnknown(ip), 10); err != nil {
		writeRateErr(w, err)
		return
	}
	if err := auth.CheckRate(r.Context(), d.Pool, "register:email:"+email, 5, 3600); err != nil {
		writeRateErr(w, err)
		return
	}

	// Turnstile.
	if body.TurnstileToken != nil {
		if err := d.Turnstile.RequireToken(r.Context(), *body.TurnstileToken, ip); err != nil {
			if te, ok := err.(*auth.TurnstileError); ok {
				httpx.Error(w, http.StatusForbidden, te.Message)
				return
			}
			httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
			return
		}
	}

	// Registration Abuse Guard.
	deviceID := readCookie(r, regDeviceCookie)
	if deviceID == "" {
		deviceID = uuid.NewString()
	}
	ipHash := auth.HashSignal(orUnknown(ip), d.Cfg.CmsEncryptionSecret)
	deviceHash := auth.HashSignal(auth.RequestDeviceSignal(r.Header.Get("user-agent"), deviceID), d.Cfg.CmsEncryptionSecret)
	guard := auth.EvaluateRegistrationGuard(r.Context(), d.Pool, d.Cfg.CmsEncryptionSecret, auth.GuardInput{
		Email:        email,
		IPHash:       ipHash,
		DeviceHash:   deviceHash,
		ReferralCode: referralCode,
		Source:       "public",
	})

	// Email sudah dipakai akun lain. Diperiksa paling awal, sebelum keputusan
	// guard, karena email yang sudah terdaftar tidak akan pernah bisa dipakai
	// lagi: guard memberi skor 100 untuknya (RecentRegistration) dan domain
	// non-Gmail memicu jalur "hubungi admin", sehingga keduanya menutupi alasan
	// sebenarnya. Bila perannya BERBEDA (mis. email mitra didaftarkan lagi
	// sebagai klien), tolak eksplisit supaya login dan izin tidak bertabrakan;
	// peran yang sama tetap dibalas netral 201 (anti-enumerasi, parity lama).
	var existingID, existingRole string
	err := d.Pool.QueryRow(r.Context(), "SELECT id, role FROM users WHERE email = $1 LIMIT 1", email).Scan(&existingID, &existingRole)
	if err == nil {
		emailHash := auth.HashSignal(email, d.Cfg.CmsEncryptionSecret)
		domain := ""
		if at := strings.LastIndex(email, "@"); at != -1 {
			domain = strings.ToLower(email[at+1:])
		}
		roleConflict := existingRole != role
		reasons := append(guard.ReasonCodes, "DUPLICATE")
		if roleConflict {
			reasons = append(reasons, "ROLE_CONFLICT")
		}
		auth.RecordRegistration(r.Context(), d.Pool, auth.RegistrationHistoryEntry{
			Source: "public", EmailHash: emailHash, IPHash: ipHash, DeviceHash: deviceHash,
			Domain: &domain, RiskScore: guard.RiskScore, RiskLevel: guard.RiskLevel, ReasonCodes: reasons,
		})
		d.setRegDeviceCookie(w, deviceID)
		if roleConflict {
			detail, _ := json.Marshal(map[string]any{"requested_role": role, "existing_role": existingRole})
			_, _ = d.Pool.Exec(r.Context(),
				"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, ip_address, user_agent, detail) VALUES ($1, NULL, 'registration_role_conflict', 'registration', $2, $3, $4, $5::jsonb)",
				uuid.NewString(), existingID, nullIfEmpty(ip), nullIfEmpty(r.Header.Get("user-agent")), string(detail))
			httpx.Error(w, http.StatusConflict,
				"Email ini sudah terdaftar sebagai "+roleLabelID(existingRole)+". Masuk memakai akun itu, atau daftar dengan email lain.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		return
	}

	if guard.Decision == auth.GuardAdminRequired {
		msg := guard.CustomMessage
		if msg == "" {
			msg = auth.AdminRequiredMessage
		}
		d.setRegDeviceCookie(w, deviceID)
		httpx.Error(w, http.StatusUnprocessableEntity, msg)
		return
	}
	if guard.Decision == auth.GuardBlock || guard.Decision == auth.GuardReview {
		// Tanpa detail alasan di response (anti-enumerasi) — detail di audit.
		detail := map[string]any{"reason_codes": guard.ReasonCodes, "risk_score": guard.RiskScore, "risk_level": guard.RiskLevel}
		detailJSON, _ := json.Marshal(detail)
		_, _ = d.Pool.Exec(r.Context(),
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, ip_address, user_agent, detail) VALUES ($1, NULL, 'registration_blocked', 'registration', $2, $3, $4, $5::jsonb)",
			uuid.NewString(), deviceID, nullIfEmpty(ip), nullIfEmpty(r.Header.Get("user-agent")), string(detailJSON))
		d.setRegDeviceCookie(w, deviceID)
		httpx.Error(w, http.StatusForbidden, auth.GenericBlockMessage)
		return
	}
	if guard.Decision == auth.GuardVerify {
		detail := map[string]any{"reason_codes": guard.ReasonCodes, "risk_score": guard.RiskScore, "risk_level": guard.RiskLevel}
		detailJSON, _ := json.Marshal(detail)
		_, _ = d.Pool.Exec(r.Context(),
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, detail) VALUES ($1, NULL, 'registration_review', 'registration', $2, $3::jsonb)",
			uuid.NewString(), deviceID, string(detailJSON))
	}

	userID := uuid.NewString()
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}

	txErr := db.WithPoolTx(r.Context(), d.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO users (id, email, password_hash, display_name, role, registration_source, is_active, approved_at, created_at) VALUES ($1, $2, $3, $4, $5, 'public', TRUE, NOW(), NOW())",
			userID, email, passwordHash, displayName, role); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, 'user.registered', 'user', $3)",
			uuid.NewString(), userID, userID); err != nil {
			return err
		}
		if role == "partner" {
			return auth.ProvisionPartner(r.Context(), tx, userID, displayName, derefStr(partnerCategory))
		}
		if role == "client" {
			return auth.ProvisionClient(r.Context(), tx, userID, displayName)
		}
		return nil
	})
	if txErr != nil {
		if strings.Contains(strings.ToLower(txErr.Error()), "duplicate key") {
			// race email unik → perlakukan seperti existing (parity anti-enumerasi)
			d.setRegDeviceCookie(w, deviceID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, txErr.Error()))
		return
	}

	// Atribusi referral SETELAH commit (parity: memakai pool, bukan tx).
	if referralCode != nil && *referralCode != "" {
		ua := r.Header.Get("user-agent")
		_ = auth.AttributeReferral(r.Context(), d.Pool, userID, *referralCode, auth.ReferralOpts{
			IP: nullIfEmptyPtr(ip), UserAgent: nullIfEmptyPtr(ua), IPHash: ipHash, DeviceHash: deviceHash,
		})
	}

	// History guard registrasi sukses (best-effort, setelah commit).
	emailHash := auth.HashSignal(email, d.Cfg.CmsEncryptionSecret)
	domain := ""
	if at := strings.LastIndex(email, "@"); at != -1 {
		domain = strings.ToLower(email[at+1:])
	}
	domainPtr := &domain
	uid := userID
	auth.RecordRegistration(r.Context(), d.Pool, auth.RegistrationHistoryEntry{
		UserID: &uid, Source: "public", EmailHash: emailHash, IPHash: ipHash, DeviceHash: deviceHash,
		Domain: domainPtr, RiskScore: guard.RiskScore, RiskLevel: guard.RiskLevel, ReasonCodes: guard.ReasonCodes,
	})

	// Response: user JSON 201 (parity toUser).
	var id, uEmail, uDisplayName, uRole string
	var createdAt time.Time
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT id, email, display_name, role, created_at FROM users WHERE id = $1 LIMIT 1",
		userID).Scan(&id, &uEmail, &uDisplayName, &uRole, &createdAt); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Failed to create user")
		return
	}
	d.setRegDeviceCookie(w, deviceID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": id, "email": uEmail, "display_name": uDisplayName, "role": uRole, "created_at": isoMilli(createdAt),
	})
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// roleLabelID: sebutan peran untuk pesan yang dibaca pengguna.
func roleLabelID(role string) string {
	switch role {
	case "partner":
		return "mitra"
	case "client":
		return "klien"
	default:
		return "akun Tauthub"
	}
}

func nullIfEmptyPtr(s string) *string { return nullIfEmpty(s) }

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
