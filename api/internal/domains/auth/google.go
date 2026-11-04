package authdomain

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port 5 route Google Sign-In: /v1/auth/google/{url,callback,complete,link,exchange-session}.

// GoogleURL: GET — 302 ke Google consent (atau 503 dengan pesan parity).
func (d *Deps) GoogleURL(w http.ResponseWriter, r *http.Request) {
	cfg := d.Google.LoadConfig(r.Context())
	if !d.Google.Configured(r.Context()) {
		// Diagnostik rinci (tabel/env/APP_SECRET/field kurang) HANYA ke log server
		// dan endpoint admin (platform/oauth/google). Respons publik ini tidak
		// boleh membocorkan konfigurasi internal.
		missing := []string{}
		if cfg.ClientID == "" {
			missing = append(missing, "client_id")
		}
		if cfg.RedirectURI == "" {
			missing = append(missing, "redirect_uri")
		}
		if len(missing) == 0 {
			missing = append(missing, "client_secret")
		}
		hint := ""
		switch cfg.ConfigError {
		case "table_missing":
			hint = "tabel platform_oauth_configs belum ada — jalankan migration (migrate-additive)"
		case "record_empty":
			hint = "belum ada record 'google' — simpan konfigurasi di Admin → Integrasi → Google Sign-In"
		case "secret_decrypt_failed":
			hint = "client secret tersimpan tidak dapat di-decrypt (kemungkinan APP_SECRET berubah) — simpan ulang Client Secret di Admin"
		}
		slog.Error("[google/url] konfigurasi Google belum lengkap",
			"source", cfg.Source, "config_error", cfg.ConfigError,
			"missing", strings.Join(missing, ","), "hint", hint)
		httpx.Error(w, http.StatusServiceUnavailable,
			"Layanan masuk dengan Google sedang tidak tersedia. Coba lagi nanti.")
		return
	}

	q := r.URL.Query()
	mode := "login"
	if q.Get("mode") == "register" {
		mode = "register"
	}
	role := ""
	if q.Get("role") == "partner" || q.Get("role") == "client" {
		role = q.Get("role")
	}
	returnTo := auth.SafeReturnTo(q.Get("returnTo"), appOrigin(r))
	state := d.Google.CreateOAuthState(w, mode, role, returnTo, appOrigin(r))

	authorize := auth.GoogleAuthURL + "?client_id=" + urlEscape(cfg.ClientID) +
		"&redirect_uri=" + urlEscape(googleRedirectURI(r, cfg)) +
		"&response_type=code&scope=" + urlEscape("openid email profile") +
		"&state=" + urlEscape(state) + "&prompt=select_account"
	http.Redirect(w, r, authorize, http.StatusFound)
}

// googleRedirectURI: Google menuntut redirect_uri absolut yang terdaftar di
// Console — config yang tersimpan sebagai path relatif (mis. "/v1/auth/
// google/callback") dibangun dari origin request; URI absolut dipakai apa
// adanya. Nilai yang sama wajib dipakai lagi saat tukar code di callback.
func googleRedirectURI(r *http.Request, cfg auth.GoogleOAuthConfig) string {
	if strings.HasPrefix(cfg.RedirectURI, "http://") || strings.HasPrefix(cfg.RedirectURI, "https://") {
		return cfg.RedirectURI
	}
	return appOrigin(r) + cfg.RedirectURI
}

func googleFailRedirect(w http.ResponseWriter, r *http.Request, code string) {
	target := "/login?oauth=" + urlEscape(code)
	http.Redirect(w, r, target, http.StatusFound)
}

// GoogleCallback: GET — 3 jalur (session/link/register) sesuai parity TEST 8/9.
func (d *Deps) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("error") != "" {
		googleFailRedirect(w, r, "cancelled") // user cancel — tanpa akun baru
		return
	}
	code, state := q.Get("code"), q.Get("state")
	if code == "" || state == "" {
		googleFailRedirect(w, r, "invalid_state")
		return
	}
	stateInfo := d.Google.ConsumeOAuthState(r, w, state)
	if stateInfo == nil {
		googleFailRedirect(w, r, "invalid_state")
		return
	}

	// Rate limit callback per IP.
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "google:callback:"+orUnknown(authRequestIp(r)), 30); err != nil {
		if _, ok := err.(*auth.AuthRateLimitError); ok {
			googleFailRedirect(w, r, "rate_limited")
			return
		}
		googleFailRedirect(w, r, "oauth_error")
		return
	}

	exchanged := d.Google.ExchangeGoogleCode(r.Context(), code, googleRedirectURI(r, d.Google.LoadConfig(r.Context())))
	if exchanged == nil {
		googleFailRedirect(w, r, "oauth_error")
		return
	}
	profile := d.Google.FetchUserinfo(r.Context(), exchanged.AccessToken)
	if profile == nil || profile.Email == "" {
		googleFailRedirect(w, r, "no_email")
		return
	}

	// A. Provider account sudah ter-link → ticket session.
	if uid, linked := d.Google.FindOAuthAccount(r.Context(), "google", profile.Sub); linked {
		d.Google.SetTicket(w, "session", map[string]string{"uid": uid, "returnTo": stateInfo.ReturnTo})
		http.Redirect(w, r, "/login/google-callback", http.StatusFound)
		return
	}

	// B. Email sama sudah ada sebagai akun password → account linking.
	if existing := d.Google.FindUserByEmail(r.Context(), profile.Email); existing != nil {
		d.Google.SetTicket(w, "link", map[string]string{
			"uid": existing.ID, "sub": profile.Sub, "email": profile.Email,
			"name": profile.Name, "picture": profile.Picture,
			"hd": exchanged.IDClaims.HD, "returnTo": stateInfo.ReturnTo,
			"email_verified": boolFlag(exchanged.IDClaims.EmailVerified || profile.EmailVerified),
		})
		http.Redirect(w, r, "/login/oauth-link?email="+urlEscape(profile.Email), http.StatusFound)
		return
	}

	// C. User baru → guard registrasi, lalu pilih role.
	deviceID := readCookie(r, regDeviceCookie)
	if deviceID == "" {
		deviceID = uuid.NewString()
	}
	guard := auth.EvaluateRegistrationGuard(r.Context(), d.Pool, d.Cfg.CmsEncryptionSecret, auth.GuardInput{
		Email:          profile.Email,
		GoogleSub:      profile.Sub,
		GoogleHD:       exchanged.IDClaims.HD,
		GoogleVerified: exchanged.IDClaims.EmailVerified || profile.EmailVerified,
		IPHash:         auth.HashSignal(orUnknown(authRequestIp(r)), d.Cfg.CmsEncryptionSecret),
		DeviceHash:     auth.HashSignal(auth.RequestDeviceSignal(r.Header.Get("user-agent"), deviceID), d.Cfg.CmsEncryptionSecret),
		Source:         "google",
	})
	// Pendaftaran Google baru hanya lolos bila guard benar-benar allow: review,
	// verify, dan admin_required sama-sama ditolak (tidak diam-diam buat akun).
	if guard.Decision != auth.GuardAllow {
		detail := map[string]any{"reason_codes": guard.ReasonCodes, "risk_score": guard.RiskScore, "risk_level": guard.RiskLevel, "source": "google"}
		detailJSON, _ := json.Marshal(detail)
		_, _ = d.Pool.Exec(r.Context(),
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, ip_address, user_agent, detail) VALUES ($1, NULL, 'registration_blocked', 'registration', $2, $3, $4, $5::jsonb)",
			uuid.NewString(), deviceID, nullIfEmpty(authRequestIp(r)), nullIfEmpty(r.Header.Get("user-agent")), string(detailJSON))
		d.recordGoogleAttempt(r, profile.Email, profile.Sub, guard, "BLOCKED")
		googleFailRedirect(w, r, "registration_unavailable")
		return
	}

	roleSuffix := ""
	if stateInfo.RoleHint == "partner" || stateInfo.RoleHint == "client" {
		roleSuffix = "?role=" + stateInfo.RoleHint
	}
	d.Google.SetTicket(w, "register", map[string]string{
		"sub": profile.Sub, "email": profile.Email, "name": profile.Name, "picture": profile.Picture,
		"hd": exchanged.IDClaims.HD, "role": stateInfo.RoleHint, "returnTo": stateInfo.ReturnTo,
		"email_verified": boolFlag(exchanged.IDClaims.EmailVerified || profile.EmailVerified),
	})
	registerURL := "/register-role" + roleSuffix
	if stateInfo.ReturnTo != "" {
		sep := "?"
		if strings.Contains(registerURL, "?") {
			sep = "&"
		}
		registerURL += sep + "returnTo=" + urlEscape(stateInfo.ReturnTo)
	}
	http.Redirect(w, r, registerURL, http.StatusFound)
}

// GoogleComplete: POST — pilih role → buat user Google baru (201 + sesi).
func (d *Deps) GoogleComplete(w http.ResponseWriter, r *http.Request) {
	ticket := d.Google.GetTicket(r)
	if ticket == nil || ticket.Kind != "register" {
		httpx.Error(w, http.StatusConflict, "Sesi pendaftaran Google tidak valid atau kedaluwarsa. Coba lagi.")
		return
	}
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "google:complete:"+orUnknown(authRequestIp(r)), 30); err != nil {
		writeRateErr(w, err)
		return
	}
	sub, email, name, picture := ticket.Data["sub"], ticket.Data["email"], ticket.Data["name"], ticket.Data["picture"]
	hd, emailVerified, ticketRole := ticket.Data["hd"], ticket.Data["email_verified"], ticket.Data["role"]
	if sub == "" || email == "" {
		httpx.Error(w, http.StatusConflict, "Sesi pendaftaran Google tidak lengkap. Coba lagi.")
		return
	}

	var body struct {
		Role         string  `json:"role"`
		ReferralCode *string `json:"referral_code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	role := body.Role
	if role == "" {
		role = ticketRole
	}
	if role != "client" && role != "partner" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pilihan role tidak valid.")
		return
	}

	// Defense in depth: guard ulang sebelum INSERT (parity).
	deviceID := readCookie(r, regDeviceCookie)
	ipHash := auth.HashSignal(orUnknown(authRequestIp(r)), d.Cfg.CmsEncryptionSecret)
	deviceHash := auth.HashSignal(auth.RequestDeviceSignal(r.Header.Get("user-agent"), deviceID), d.Cfg.CmsEncryptionSecret)
	guard := auth.EvaluateRegistrationGuard(r.Context(), d.Pool, d.Cfg.CmsEncryptionSecret, auth.GuardInput{
		Email: email, GoogleSub: sub, GoogleHD: hd, GoogleVerified: emailVerified == "1",
		IPHash: ipHash, DeviceHash: deviceHash, ReferralCode: body.ReferralCode, Source: "google",
	})
	if guard.Decision != auth.GuardAllow {
		d.recordGoogleAttempt(r, email, sub, guard, "BLOCKED")
		httpx.Error(w, http.StatusForbidden, auth.GenericBlockMessage)
		return
	}

	// Duplikasi proteksi.
	if _, exists := d.Google.FindOAuthAccount(r.Context(), "google", sub); exists {
		httpx.Error(w, http.StatusConflict, "Akun Google ini sudah terhubung ke akun lain.")
		return
	}
	if existing := d.Google.FindUserByEmail(r.Context(), email); existing != nil {
		httpx.Error(w, http.StatusConflict, "Email sudah terdaftar. Masuk dengan email atau hubungkan akun Google dari halaman login.")
		return
	}

	userID := uuid.NewString()
	displayName := name
	if displayName == "" {
		if at := strings.Index(email, "@"); at > 0 {
			displayName = email[:at]
		} else {
			displayName = "User"
		}
	}

	txErr := db.WithPoolTx(r.Context(), d.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO users (id, email, password_hash, display_name, role, registration_source, is_active, approved_at, created_at) VALUES ($1, $2, '', $3, $4, 'google', TRUE, NOW(), NOW())",
			userID, email, displayName, role); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO user_oauth_accounts (id, user_id, provider, provider_account_id, email, display_name, avatar_url) VALUES ($1, $2, 'google', $3, $4, $5, $6)",
			uuid.NewString(), userID, sub, email, nilIfEmpty(name), nilIfEmpty(picture)); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, 'user.google_register', 'user', $3)",
			uuid.NewString(), userID, userID); err != nil {
			return err
		}
		if role == "partner" {
			return auth.ProvisionPartner(r.Context(), tx, userID, displayName, "")
		}
		return auth.ProvisionClient(r.Context(), tx, userID, displayName)
	})
	if txErr != nil {
		if strings.Contains(strings.ToLower(txErr.Error()), "unique") || strings.Contains(txErr.Error(), "sudah terhubung") {
			httpx.Error(w, http.StatusConflict, "Akun Google ini sudah terhubung ke akun lain.")
			return
		}
		slog.Error("[google/complete]", "err", txErr)
		httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan saat membuat akun. Coba lagi.")
		return
	}

	if body.ReferralCode != nil && *body.ReferralCode != "" {
		_ = auth.AttributeReferral(r.Context(), d.Pool, userID, *body.ReferralCode, auth.ReferralOpts{
			IP: nullIfEmptyPtr(authRequestIp(r)), UserAgent: nullIfEmptyPtr(r.Header.Get("user-agent")),
			IPHash: ipHash, DeviceHash: deviceHash,
		})
	}
	d.Google.ClearTicket(w)

	domain := ""
	if at := strings.LastIndex(email, "@"); at != -1 {
		domain = strings.ToLower(email[at+1:])
	}
	emailHash := auth.HashSignal(strings.ToLower(email), d.Cfg.CmsEncryptionSecret)
	subHash := auth.HashSignal(sub, d.Cfg.CmsEncryptionSecret)
	uidPtr := userID
	auth.RecordRegistration(r.Context(), d.Pool, auth.RegistrationHistoryEntry{
		UserID: &uidPtr, Source: "google", EmailHash: emailHash, GoogleSubHash: subHash,
		IPHash: ipHash, DeviceHash: deviceHash, Domain: &domain,
		RiskScore: guard.RiskScore, RiskLevel: guard.RiskLevel, ReasonCodes: guard.ReasonCodes,
	})

	slog.Info("[google/complete] session issued", "user", userID, "role", role)
	d.issueSession(w, r, userID, SessionUser{ID: userID, Email: email, Role: role, DisplayName: &displayName}, http.StatusCreated)
}

// recordGoogleAttempt: percobaan daftar Google yang ditolak ikut dicatat ke
// registration_history (tanpa user_id) supaya batas per domain menghitung
// percobaan, bukan hanya akun yang berhasil dibuat.
func (d *Deps) recordGoogleAttempt(r *http.Request, email, sub string, guard auth.GuardDecision, tag string) {
	deviceID := readCookie(r, regDeviceCookie)
	domain := ""
	if at := strings.LastIndex(email, "@"); at != -1 {
		domain = strings.ToLower(email[at+1:])
	}
	auth.RecordRegistration(r.Context(), d.Pool, auth.RegistrationHistoryEntry{
		Source:        "google",
		EmailHash:     auth.HashSignal(strings.ToLower(email), d.Cfg.CmsEncryptionSecret),
		GoogleSubHash: auth.HashSignal(sub, d.Cfg.CmsEncryptionSecret),
		IPHash:        auth.HashSignal(orUnknown(authRequestIp(r)), d.Cfg.CmsEncryptionSecret),
		DeviceHash:    auth.HashSignal(auth.RequestDeviceSignal(r.Header.Get("user-agent"), deviceID), d.Cfg.CmsEncryptionSecret),
		Domain:        &domain,
		RiskScore:     guard.RiskScore, RiskLevel: guard.RiskLevel,
		ReasonCodes:   append([]string{tag}, guard.ReasonCodes...),
	})
}

// GoogleLink: POST — hubungkan Google ke akun password existing (wajib password).
func (d *Deps) GoogleLink(w http.ResponseWriter, r *http.Request) {
	ticket := d.Google.GetTicket(r)
	if ticket == nil || ticket.Kind != "link" {
		httpx.Error(w, http.StatusConflict, "Sesi penghubung akun Google tidak valid atau kedaluwarsa. Coba lagi.")
		return
	}
	uid, sub, email, name, picture := ticket.Data["uid"], ticket.Data["sub"], ticket.Data["email"], ticket.Data["name"], ticket.Data["picture"]
	if uid == "" || sub == "" {
		httpx.Error(w, http.StatusConflict, "Sesi penghubung akun Google tidak valid.")
		return
	}
	// Kunci per pengguna dari tiket (tiket sudah diverifikasi di atas).
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "google:link:user:"+uid, 30); err != nil {
		writeRateErr(w, err)
		return
	}

	var passwordHash, uEmail, uRole string
	err := d.Pool.QueryRow(r.Context(),
		"SELECT id, email, role, password_hash FROM users WHERE id = $1 LIMIT 1", uid).
		Scan(new(string), &uEmail, &uRole, &passwordHash)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Akun tidak ditemukan.")
		return
	}
	if passwordHash == "" {
		httpx.Error(w, http.StatusConflict, "Akun ini tidak memakai kata sandi. Hubungi admin.")
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if len(body.Password) < 6 || len(body.Password) > 200 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kata sandi tidak valid.")
		return
	}
	if !auth.VerifyPassword(body.Password, passwordHash) {
		httpx.Error(w, http.StatusUnauthorized, "Kata sandi salah. Hubungkan akun dengan kata sandi yang benar.")
		return
	}

	linkErr := d.Google.LinkOAuthAccount(r.Context(), d.Pool, struct {
		UserID, Provider, ProviderAccountID, Email, DisplayName, AvatarUrl string
	}{UserID: uid, Provider: "google", ProviderAccountID: sub, Email: email, DisplayName: name, AvatarUrl: picture})
	if linkErr != nil {
		if linkErr == auth.ErrDuplicateOAuth {
			httpx.Error(w, http.StatusConflict, "Akun Google ini sudah terhubung ke akun lain.")
			return
		}
		slog.Error("[google/link]", "err", linkErr)
		httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan saat menghubungkan akun. Coba lagi.")
		return
	}
	d.Google.ClearTicket(w)

	displayName := strings.Split(uEmail, "@")[0]
	if displayName == "" {
		displayName = "User"
	}
	_ = db.WithPoolTx(r.Context(), d.Pool, func(tx pgx.Tx) error {
		return auth.HealRoleProvisioning(r.Context(), tx, uRole, uid, displayName)
	})

	d.issueSession(w, r, uid, SessionUser{ID: uid, Email: uEmail, Role: uRole}, http.StatusOK)
}

// GoogleExchangeSession: POST — tukar ticket session → token pair.
func (d *Deps) GoogleExchangeSession(w http.ResponseWriter, r *http.Request) {
	ticket := d.Google.GetTicket(r)
	if ticket == nil || ticket.Kind != "session" {
		httpx.Error(w, http.StatusUnauthorized, "Sesi Google tidak valid atau kedaluwarsa.")
		return
	}
	uid := ticket.Data["uid"]
	if uid == "" {
		httpx.Error(w, http.StatusUnauthorized, "Sesi Google tidak valid.")
		return
	}
	var id, email, displayName, role string
	var isActive bool
	var approvedAt *interface{}
	err := d.Pool.QueryRow(r.Context(),
		"SELECT id, email, display_name, role, is_active, approved_at FROM users WHERE id = $1 LIMIT 1",
		uid).Scan(&id, &email, &displayName, &role, &isActive, &approvedAt)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Akun tidak ditemukan atau dinonaktifkan.")
		return
	}
	if !isActive {
		if d.isSuspendedPartner(r.Context(), id) {
			httpx.Error(w, http.StatusForbidden, suspendedMessage)
			return
		}
		httpx.Error(w, http.StatusNotFound, "Akun tidak ditemukan atau dinonaktifkan.")
		return
	}
	d.Google.ClearTicket(w)

	// Self-heal provisioning (idempoten — parity).
	displayNameFallback := strings.Split(email, "@")[0]
	if displayName == "" {
		displayName = displayNameFallback
	}
	if displayName == "" {
		displayName = "User"
	}
	_ = db.WithPoolTx(r.Context(), d.Pool, func(tx pgx.Tx) error {
		return auth.HealRoleProvisioning(r.Context(), tx, role, id, displayName)
	})

	slog.Info("[google/exchange-session] session issued", "user", id, "role", role)
	d.issueSession(w, r, id, SessionUser{ID: id, Email: email, Role: role, DisplayName: &displayName}, http.StatusOK)
}

func appOrigin(r *http.Request) string {
	if d := r.Header.Get("x-forwarded-proto"); d != "" {
		host := r.Header.Get("x-forwarded-host")
		if host == "" {
			host = r.Host
		}
		return d + "://" + host
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func urlEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "&", "%26"), " ", "%20")
}

func boolFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
