package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Port src/auth/session.ts — claim HS256 + issuer APP_NAME, `sid` = refresh_jti.

const SessionMaxAgeSeconds = 24 * 60 * 60

type TokenType string

const (
	TokenAccess  TokenType = "access"
	TokenRefresh TokenType = "refresh"
)

type Claims struct {
	Sub  string
	Type TokenType
	JTI  string
	SID  string // hanya access token
	IAT  int64
	EXP  int64
}

type TokenWithPayload struct {
	Token      string
	ExpiresIn  int64
	JTI        string
}

type TokenPair struct {
	AccessToken      string
	AccessExpiresIn  int64
	AccessJTI        string
	RefreshToken     string
	RefreshExpiresIn int64
	RefreshJTI       string
	SessionExpiresAt time.Time
}

// CreateSessionDeadline: floor ke detik lalu +24 jam (parity session.ts).
func CreateSessionDeadline(now time.Time) time.Time {
	return time.Unix(now.Unix(), 0).Add(SessionMaxAgeSeconds * time.Second)
}

func signToken(secret []byte, issuer, sub string, typ TokenType, jti, sid string, iat, exp int64) (TokenWithPayload, error) {
	claims := jwt.MapClaims{
		"iss":  issuer,
		"sub":  sub,
		"type": string(typ),
		"jti":  jti,
		"iat":  iat,
		"exp":  exp,
	}
	if sid != "" {
		claims["sid"] = sid
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return TokenWithPayload{}, fmt.Errorf("sign jwt: %w", err)
	}
	return TokenWithPayload{Token: signed, ExpiresIn: exp - iat, JTI: jti}, nil
}

func newJTI() string { return uuid.NewString() }

// CreateTokenPair: refresh exp = min(REFRESH_TOKEN_MINUTES*60, remaining),
// access exp = min(ACCESS_TOKEN_MINUTES*60, remaining); sid access = refresh jti.
func CreateTokenPair(secret, issuer string, userID string, accessMinutes, refreshMinutes int64, sessionExpiresAt time.Time) (*TokenPair, error) {
	now := time.Now().Unix()
	deadline := sessionExpiresAt.Unix()
	remaining := deadline - now
	if remaining <= 0 {
		return nil, fmt.Errorf("Session deadline has expired")
	}
	refreshExpiresIn := refreshMinutes * 60
	if refreshExpiresIn > remaining {
		refreshExpiresIn = remaining
	}
	accessExpiresIn := accessMinutes * 60
	if accessExpiresIn > remaining {
		accessExpiresIn = remaining
	}

	refresh, err := signToken([]byte(secret), issuer, userID, TokenRefresh, newJTI(), "", now, now+refreshExpiresIn)
	if err != nil {
		return nil, err
	}
	access, err := signToken([]byte(secret), issuer, userID, TokenAccess, newJTI(), refresh.JTI, now, now+accessExpiresIn)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:      access.Token,
		AccessExpiresIn:  access.ExpiresIn,
		AccessJTI:        access.JTI,
		RefreshToken:     refresh.Token,
		RefreshExpiresIn: refresh.ExpiresIn,
		RefreshJTI:       refresh.JTI,
		SessionExpiresAt: sessionExpiresAt,
	}, nil
}

// VerifyToken: HS256 + issuer + type match (port verify() di session.ts).
func VerifyToken(secret, issuer, token string, expectedType TokenType) (*Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer))
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	typ, _ := mc["type"].(string)
	if typ != string(expectedType) {
		return nil, fmt.Errorf("invalid type")
	}
	sub, _ := mc["sub"].(string)
	jti, _ := mc["jti"].(string)
	if sub == "" || jti == "" {
		return nil, fmt.Errorf("invalid sub/jti")
	}
	sid, _ := mc["sid"].(string)
	iat := int64(0)
	if v, ok := mc["iat"].(float64); ok {
		iat = int64(v)
	}
	exp := int64(0)
	if v, ok := mc["exp"].(float64); ok {
		exp = int64(v)
	}
	return &Claims{Sub: sub, Type: TokenType(typ), JTI: jti, SID: sid, IAT: iat, EXP: exp}, nil
}

func VerifyAccessToken(secret, issuer, token string) (*Claims, error) {
	return VerifyToken(secret, issuer, token, TokenAccess)
}

func VerifyRefreshToken(secret, issuer, token string) (*Claims, error) {
	return VerifyToken(secret, issuer, token, TokenRefresh)
}
