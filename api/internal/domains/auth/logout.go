package authdomain

import (
	"encoding/json"
	"net/http"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
)

// Port v1/auth/logout/route.ts — revoke sesi by refresh jti + hapus cookie, 204.

func (d *Deps) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	token := ""
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.RefreshToken != "" {
		token = body.RefreshToken
	}
	if token == "" {
		if c, err := r.Cookie("refresh_token"); err == nil {
			token = c.Value
		}
	}

	if token != "" {
		if claims, err := auth.VerifyRefreshToken(d.Cfg.Secret, d.Cfg.AppName, token); err == nil {
			_, _ = d.Pool.Exec(r.Context(),
				"UPDATE user_sessions SET revoked = TRUE WHERE refresh_jti = $1", claims.JTI)
			d.Auth.InvalidateAuthCache(claims.Sub)
		}
	}

	// 204 + hapus cookie (parity NextResponse.cookies.delete)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   d.Cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
