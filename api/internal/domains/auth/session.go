package authdomain

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port issueSessionResponse + tokenPairResponse (src/auth/session.ts):
// insert user_sessions (sid = refresh_jti) + set cookie httpOnly + body token pair.

type Deps struct {
	Pool      *pgxpool.Pool
	Cfg       *config.Config
	Turnstile *auth.Turnstile
	Auth      *httpx.Authenticator
}

// SessionUser: payload user untuk body respons (parity SessionUserPayload).
type SessionUser struct {
	ID          string
	Email       string
	Role        string
	DisplayName *string
	CreatedAt   *time.Time
}

// isoMilli: format ISO-8601 presisi milidetik (parity JSON.stringify(new Date())).
func isoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

func (d *Deps) issueSession(w http.ResponseWriter, r *http.Request, userID string, user SessionUser, status int) {
	ctx := r.Context()
	sessionExpires := auth.CreateSessionDeadline(time.Now())
	pair, err := auth.CreateTokenPair(d.Cfg.Secret, d.Cfg.AppName, userID,
		int64(d.Cfg.AccessTokenMinutes), int64(d.Cfg.RefreshTokenMinutes), sessionExpires)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	sessionID := uuid.NewString()
	if _, err := d.Pool.Exec(ctx,
		"INSERT INTO user_sessions (id, user_id, refresh_jti, expires_at, revoked, created_at) VALUES ($1, $2, $3, $4, FALSE, NOW())",
		sessionID, userID, pair.RefreshJTI, sessionExpires,
	); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	tokenPairResponse(w, d.Cfg, pair, user, status)
}

func tokenPairResponse(w http.ResponseWriter, cfg *config.Config, pair *auth.TokenPair, user SessionUser, status int) {
	userJSON := map[string]any{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	}
	if user.DisplayName != nil {
		userJSON["display_name"] = *user.DisplayName
	}
	if user.CreatedAt != nil {
		userJSON["created_at"] = isoMilli(*user.CreatedAt)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// Cookie HARSSUS ditambahkan sebelum WriteHeader (header terkirim saat WriteHeader).
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    pair.RefreshToken,
		Path:     "/",
		MaxAge:   int(pair.RefreshExpiresIn),
		HttpOnly: true,
		Secure:   cfg.IsProduction(),
		// Lax: cookie tetap terkirim setelah redirect top-level (OAuth/email link),
		// POST lintas situs tetap tidak menerima cookie (parity session.ts).
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":       pair.AccessToken,
			"refresh_token":      pair.RefreshToken,
			"expires_in":         pair.AccessExpiresIn,
			"refresh_expires_in": pair.RefreshExpiresIn,
			"session_expires_at": isoMilli(pair.SessionExpiresAt),
			"user":               userJSON,
		})
}
