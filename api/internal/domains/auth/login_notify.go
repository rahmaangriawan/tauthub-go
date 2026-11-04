package authdomain

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	emailqueue "github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
)

// notifyLogin: email + WA "login baru ke akun Anda" untuk setiap sesi baru.
// Dijalankan di goroutine dari issueSession; tidak boleh menggagalkan login.
// Dedupe per sessionID supaya retry/refresh tidak mengirim dua kali. WA hanya
// terkirim bila pengguna sudah mengisi nomor + consent (notify.EnqueueUserWA).
func (d *Deps) notifyLogin(sessionID string, user SessionUser, ip, ua string, at time.Time) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("[auth] notifikasi login panic", "err", rec)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	name := strings.TrimSpace(derefStr(user.DisplayName))
	if user.Role == "partner" {
		var partnerName *string
		if err := d.Pool.QueryRow(ctx, "SELECT name FROM partners WHERE user_id = $1 LIMIT 1", user.ID).Scan(&partnerName); err == nil && partnerName != nil && strings.TrimSpace(*partnerName) != "" {
			name = strings.TrimSpace(*partnerName)
		}
	}
	if name == "" {
		name = user.Email
	}
	browser, platform := auth.ParseUserAgent(ua)
	data := map[string]string{
		"nama":     name,
		"ip":       orUnknown(ip),
		"waktu":    auth.FormatWIB(at),
		"browser":  browser,
		"platform": platform,
	}

	uid := user.ID
	deliveryID, status, err := emailqueue.SendTransactionalEmail(ctx, d.Pool, emailqueue.SendOptions{
		Template:        "login-alert",
		To:              user.Email,
		RecipientUserID: &uid,
		ReferenceID:     &sessionID,
		Event:           "security.login",
		DedupeKey:       strPtr("login:" + sessionID),
		Data:            data,
		Audience:        emailqueue.AudienceForRole(user.Role),
	})
	if err != nil {
		slog.Warn("[auth] email notifikasi login gagal diantrekan", "err", err, "user", user.ID)
	} else if status == "queued" && deliveryID != "" {
		emailqueue.ProcessEmailInline(ctx, d.Pool, d.Cfg.CmsEncryptionSecret, deliveryID)
	}

	audience := notify.AudienceClient
	if user.Role == "partner" {
		audience = notify.AudiencePartner
	}
	if user.Role == "client" || user.Role == "partner" {
		notify.EnqueueUserWA(ctx, d.Pool, notify.UserWAOptions{
			Audience: audience, Event: "security.login", UserID: user.ID,
			ReferenceID: sessionID, Data: data,
		})
	}
}
