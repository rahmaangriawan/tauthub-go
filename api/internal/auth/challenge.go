package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

// Port src/lib/auth/totp-challenge.ts — challenge ticket 2FA:
// base64url(JSON{uid,email,nonce,exp}) + "." + base64url(HMAC-SHA256(APP_SECRET, body)).
// HMAC-signed, 5 menit, sekali pakai via nonce cache (bukan JWT).

const ticketTTL = 5 * time.Minute
const nonceCacheMaxAge = 5 * time.Minute

type ChallengePayload struct {
	UID   string `json:"uid"`
	Email string `json:"email"`
}

type ticketBody struct {
	ChallengePayload
	Nonce string `json:"nonce"`
	Exp   int64  `json:"exp"`
}

var (
	nonceMu    sync.Mutex
	nonceCache = map[string]int64{} // nonce → issuedAt (ms)

	// timeNow bisa di-pin oleh test (tiket TTL 5 menit — fixture lama bisa kedaluwarsa).
	timeNow = time.Now
)

// PinNowChallenge mem-pin jam untuk test; kembalikan restore func.
func PinNowChallenge(t time.Time) (restore func()) {
	prev := timeNow
	timeNow = func() time.Time { return t }
	return func() { timeNow = prev }
}

func ticketHMAC(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SignChallenge membuat ticket (ttl param diabaikan karena lama selalu "5m").
func SignChallenge(secret string, payload ChallengePayload, _ ...any) (string, error) {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	body := ticketBody{
		ChallengePayload: payload,
		Nonce:            base64.RawURLEncoding.EncodeToString(nonce),
		Exp:              timeNow().Add(ticketTTL).UnixMilli(),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return encoded + "." + ticketHMAC(secret, encoded), nil
}

// VerifyTicket: HMAC + exp + nonce sekali pakai. null → payload kosong + false.
func VerifyTicket(secret, ticket string) (ChallengePayload, bool) {
	bodyB64, sig, ok := strings.Cut(ticket, ".")
	if !ok || bodyB64 == "" || sig == "" {
		return ChallengePayload{}, false
	}
	expected := ticketHMAC(secret, bodyB64)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return ChallengePayload{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		return ChallengePayload{}, false
	}
	var parsed ticketBody
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ChallengePayload{}, false
	}
	if parsed.UID == "" || parsed.Nonce == "" || parsed.Exp < timeNow().UnixMilli() {
		return ChallengePayload{}, false
	}
	now := timeNow().UnixMilli()
	nonceMu.Lock()
	for n, t := range nonceCache {
		if t < now-nonceCacheMaxAge.Milliseconds() {
			delete(nonceCache, n)
		}
	}
	if _, seen := nonceCache[parsed.Nonce]; seen {
		nonceMu.Unlock()
		return ChallengePayload{}, false
	}
	nonceCache[parsed.Nonce] = now
	nonceMu.Unlock()
	return parsed.ChallengePayload, true
}

// fallback untuk env APP_SECRET langsung (port process.env.APP_SECRET ?? "").
func SecretFromEnv() string { return os.Getenv("APP_SECRET") }
