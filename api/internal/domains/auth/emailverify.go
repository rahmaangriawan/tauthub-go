package authdomain

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	emailqueue "github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Verifikasi email SEBELUM akun dibuat. Dua endpoint publik:
//   POST /auth/register/request-code {email, turnstile_token?} → {ok, retry_after}
//   POST /auth/register/verify-code  {email, code}             → {ok, ticket, expires_in}
// Tiket lalu dibawa ke /auth/register (registerBody.EmailTicket) dan dihanguskan
// di dalam transaksi pembuatan akun (auth.ClaimEmailTicket).
//
// Respons request-code sengaja netral (ok:true) untuk email yang sudah
// terdaftar, diblokir, atau kena batas per-email — pola ForgotPassword — supaya
// endpoint ini tidak bisa dipakai menebak email mana yang punya akun.

const emailCodeMsgInvalid = "Kode salah atau sudah kedaluwarsa. Minta kode baru bila perlu."
const emailTicketMsgInvalid = "Verifikasi email belum selesai atau sudah kedaluwarsa. Minta kode baru."
const disposableEmailMsg = "Email sementara (temp-mail) tidak bisa dipakai untuk mendaftar. Gunakan alamat Gmail Anda."

func (d *Deps) RequestRegisterCode(w http.ResponseWriter, r *http.Request) {
	okResp := func(retryAfter int) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "retry_after": retryAfter})
	}
	var body struct {
		Email          string  `json:"email"`
		TurnstileToken *string `json:"turnstile_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := lower(trim(body.Email))
	if !emailRe.MatchString(email) || len(email) > 320 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Email tidak valid.")
		return
	}
	ip := authRequestIp(r)

	// Aturan domain diputuskan sebelum ada kode yang dibuat/dikirim: temp-mail
	// ditolak terang-terangan, non-Gmail diarahkan ke admin (sama seperti guard
	// di /auth/register, hanya lebih awal supaya tidak menunggu kode percuma).
	emailInfo := auth.AnalyzeEmailCtx(r.Context(), d.Pool, email)
	if emailInfo.IsDisposable {
		httpx.Error(w, http.StatusUnprocessableEntity, disposableEmailMsg)
		return
	}
	if !emailInfo.IsGmail {
		httpx.Error(w, http.StatusUnprocessableEntity, auth.AdminRequiredMessage)
		return
	}

	// Per IP tegas (429) supaya UI bisa memberi tahu; per email diam-diam.
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "regcode:ip:"+orUnknown(ip), 5); err != nil {
		writeRateErr(w, err)
		return
	}
	if err := auth.CheckRate(r.Context(), d.Pool, "regcode:email:"+email, 5, 3600); err != nil {
		okResp(60)
		return
	}
	if at, ok := auth.LastCodeIssuedAt(r.Context(), d.Pool, email); ok {
		if wait := auth.EmailCodeResendWait - time.Since(at); wait > 0 {
			okResp(int(wait.Seconds()) + 1)
			return
		}
	}

	// Turnstile: endpoint ini yang mengirim email, jadi gate bot ada di sini.
	turnstileToken := ""
	if body.TurnstileToken != nil {
		turnstileToken = *body.TurnstileToken
	}
	if err := d.Turnstile.RequireToken(r.Context(), turnstileToken, ip); err != nil {
		if te, ok := err.(*auth.TurnstileError); ok {
			httpx.Error(w, http.StatusForbidden, te.Message)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}

	if auth.IsRegistrationEmailBlocked(r.Context(), d.Pool, email) {
		okResp(60)
		return
	}

	// Email sudah punya akun: tidak ada kode, pemilik asli diberi tahu lewat
	// email (maks 1/jam) — respons tetap netral.
	var existingID string
	if err := d.Pool.QueryRow(r.Context(), "SELECT id FROM users WHERE email = $1 LIMIT 1", email).Scan(&existingID); err == nil {
		if auth.CheckRate(r.Context(), d.Pool, "regcode:exists:"+email, 1, 3600) == nil {
			d.sendRegisterEmail(emailqueue.SendOptions{
				Template: "account-exists-notice", To: email, RecipientUserID: &existingID,
				Event: "security.account_exists", DedupeKey: strPtr("account_exists:" + existingID),
				Data: map[string]string{"nama": "Sahabat Tauthub", "email": email, "link": d.Cfg.PublicAppURL + "/login"},
			})
		}
		okResp(60)
		return
	}

	code, err := auth.CreateEmailCode(r.Context(), d.Pool, d.Cfg.Secret, email, ip)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	d.sendRegisterEmail(emailqueue.SendOptions{
		Template: "email-verification-code", To: email,
		Event: "security.email_verification", Audience: emailqueue.AudienceClient,
		Data: map[string]string{"nama": "Sahabat Tauthub", "email": email, "kode": code},
	})
	okResp(int(auth.EmailCodeResendWait.Seconds()))
}

// sendRegisterEmail: antrekan lalu dorong inline di goroutine supaya kode
// sampai dalam hitungan detik tanpa menunggu tick scheduler. MarkSending di
// ProcessEmailInline mencegah kirim ganda dengan worker.
func (d *Deps) sendRegisterEmail(opts emailqueue.SendOptions) {
	if opts.Audience == "" {
		opts.Audience = emailqueue.AudienceClient
	}
	deliveryID, status, err := emailqueue.SendTransactionalEmail(context.Background(), d.Pool, opts)
	if err != nil || status != "queued" || deliveryID == "" {
		return
	}
	cmsSecret := d.Cfg.CmsEncryptionSecret
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		emailqueue.ProcessEmailInline(ctx, d.Pool, cmsSecret, deliveryID)
	}()
}

func (d *Deps) VerifyRegisterCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := lower(trim(body.Email))
	code := trim(body.Code)
	if !emailRe.MatchString(email) || len(email) > 320 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Email tidak valid.")
		return
	}
	if !isSixDigits(code) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kode harus 6 digit.")
		return
	}
	ip := authRequestIp(r)
	if auth.IsDisposableDomain(r.Context(), d.Pool, auth.AnalyzeEmail(email).Domain) {
		httpx.Error(w, http.StatusUnprocessableEntity, disposableEmailMsg)
		return
	}
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "regverify:ip:"+orUnknown(ip), 20); err != nil {
		writeRateErr(w, err)
		return
	}
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "regverify:email:"+email, 10); err != nil {
		writeRateErr(w, err)
		return
	}
	ticket, ok := auth.VerifyEmailCode(r.Context(), d.Pool, d.Cfg.Secret, email, code)
	if !ok {
		httpx.Error(w, http.StatusUnprocessableEntity, emailCodeMsgInvalid)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true, "ticket": ticket, "expires_in": int(auth.EmailTicketTTL.Seconds()),
	})
}
