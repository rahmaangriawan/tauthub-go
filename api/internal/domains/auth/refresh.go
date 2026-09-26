package authdomain

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

var errRotationLost = errors.New("rotation lost race")

// Port v1/auth/refresh/route.ts — rotasi sesi dalam SATU transaksi:
// revoke lama (rowCount harus 1) + insert baru dengan expires_at sesi yang sama.

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func (d *Deps) Refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	token := ""
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.RefreshToken != "" {
		token = body.RefreshToken
	}
	if token == "" {
		if c, err := r.Cookie("refresh_token"); err == nil {
			token = c.Value
		}
	}
	if token == "" {
		httpx.Error(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	if err := auth.CheckAuthRate(r.Context(), d.Pool, "refresh:ip:"+orUnknown(auth.RequestIp(r.Header)), d.Cfg.AuthRateLimitPerMinute); err != nil {
		writeRateErr(w, err)
		return
	}

	claims, err := auth.VerifyRefreshToken(d.Cfg.Secret, d.Cfg.AppName, token)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	var sessionRefreshJTI, sessionUserID string
	var sessionRevoked bool
	var sessionExpiresAt time.Time
	err = d.Pool.QueryRow(r.Context(),
		"SELECT refresh_jti, user_id, revoked, expires_at FROM user_sessions WHERE refresh_jti = $1 LIMIT 1",
		claims.JTI,
	).Scan(&sessionRefreshJTI, &sessionUserID, &sessionRevoked, &sessionExpiresAt)
	if err != nil || sessionRevoked || !sessionExpiresAt.After(time.Now()) {
		httpx.Error(w, http.StatusUnauthorized, "Refresh token revoked")
		return
	}

	var u SessionUser
	var displayName *string
	var createdAt time.Time
	err = d.Pool.QueryRow(r.Context(),
		"SELECT id, email, display_name, role, created_at FROM users WHERE id = $1 AND is_active = TRUE LIMIT 1",
		sessionUserID,
	).Scan(&u.ID, &u.Email, &displayName, &u.Role, &createdAt)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "User no longer active")
		return
	}
	u.DisplayName = displayName
	u.CreatedAt = &createdAt

	pair, err := auth.CreateTokenPair(d.Cfg.Secret, d.Cfg.AppName, u.ID,
		int64(d.Cfg.AccessTokenMinutes), int64(d.Cfg.RefreshTokenMinutes), sessionExpiresAt)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}

	rotated := false
	tx, txErr := d.Pool.Begin(r.Context())
	if txErr != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, txErr.Error()))
		return
	}
	ct, txErr := tx.Exec(r.Context(),
		"UPDATE user_sessions SET revoked = TRUE WHERE refresh_jti = $1 AND revoked = FALSE AND expires_at > NOW()",
		sessionRefreshJTI)
	if txErr == nil && ct.RowsAffected() != 1 {
		txErr = errRotationLost // rotasi gagal → 401 (parity rowCount !== 1)
	}
	if txErr == nil {
		newID, idErr := uuid.NewRandom()
		if idErr != nil {
			txErr = idErr
		} else {
			_, txErr = tx.Exec(r.Context(),
				"INSERT INTO user_sessions (id, user_id, refresh_jti, expires_at, revoked, created_at) VALUES ($1, $2, $3, $4, FALSE, NOW())",
				newID.String(), u.ID, pair.RefreshJTI, sessionExpiresAt)
		}
	}
	if txErr != nil {
		_ = tx.Rollback(r.Context())
		if errors.Is(txErr, errRotationLost) {
			httpx.Error(w, http.StatusUnauthorized, "Refresh token revoked")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, txErr.Error()))
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	rotated = true
	_ = rotated

	tokenPairResponse(w, d.Cfg, pair, u, http.StatusOK)
}
