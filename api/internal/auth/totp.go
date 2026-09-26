package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Port src/lib/auth/totp.ts — TOTP RFC 6238 tanpa dependency:
// HMAC-SHA1, 6 digit, 30 detik, window ±1, base32 RFC 4648.

const base32Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

func GenerateTotpSecret(bytes int) (string, error) {
	if bytes <= 0 {
		bytes = 20
	}
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32Encode(buf), nil
}

func base32Encode(b []byte) string {
	var bits uint
	var value uint32
	var out strings.Builder
	for _, byteVal := range b {
		value = (value << 8) | uint32(byteVal)
		bits += 8
		for bits >= 5 {
			out.WriteByte(base32Alphabet[(value>>(bits-5))&31])
			bits -= 5
		}
	}
	if bits > 0 {
		out.WriteByte(base32Alphabet[(value<<(5-bits))&31])
	}
	return out.String()
}

func base32Decode(input string) []byte {
	clean := strings.ToUpper(strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= '2' && r <= '7') {
			return r
		}
		return -1
	}, input))
	var bits uint
	var value uint32
	out := make([]byte, 0, len(clean)*5/8)
	for _, char := range clean {
		idx := strings.IndexRune(base32Alphabet, char)
		if idx < 0 {
			continue
		}
		value = (value << 5) | uint32(idx)
		bits += 5
		if bits >= 8 {
			out = append(out, byte(value>>(bits-8)))
			bits -= 8
		}
	}
	return out
}

func hotp(secret []byte, counter int64) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))
	mac := hmac.New(sha1.New, secret)
	mac.Write(buf)
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	code := (uint32(digest[offset]&0x7f) << 24) | (uint32(digest[offset+1]) << 16) | (uint32(digest[offset+2]) << 8) | uint32(digest[offset+3])
	return fmt.Sprintf("%06d", code%1_000_000)
}

// HOTPAt diekspos untuk testing kompat (kode pada counter tertentu).
func HOTPAt(secretBase32 string, counter int64) string {
	return hotp(base32Decode(secretBase32), counter)
}

// VerifyTotp: window ±1 step untuk clock skew.
func VerifyTotp(secretBase32, code string) bool {
	if len(code) != 6 || !isDigits(code) {
		return false
	}
	secret := base32Decode(secretBase32)
	counter := unixTime() / 30
	for _, drift := range []int64{-1, 0, 1} {
		expected := hotp(secret, counter+drift)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// nowFunc dipisah agar test kompat bisa mem-pin waktu (window TOTP deterministik).
var nowFunc = time.Now

// PinNow mem-pin jam untuk test; kembalikan restore func.
func PinNow(t time.Time) (restore func()) {
	prev := nowFunc
	nowFunc = func() time.Time { return t }
	return func() { nowFunc = prev }
}

func unixTime() int64 { return nowFunc().Unix() }

// TotpAuthUrl: issuer default "Tauthub" (parity totp.ts).
func TotpAuthUrl(secretBase32, email, issuer string) string {
	if issuer == "" {
		issuer = "Tauthub"
	}
	label := url.PathEscape(issuer + ":" + email)
	params := url.Values{}
	params.Set("secret", secretBase32)
	params.Set("issuer", issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", "6")
	params.Set("period", "30")
	return fmt.Sprintf("otpauth://totp/%s?%s", label, params.Encode())
}

// Recovery codes: 10 kode, raw = hex(random 5 byte), yang disimpan = sha256 hex.
func GenerateRecoveryCodes(count int) (raw []string, hashed []string) {
	if count <= 0 {
		count = 10
	}
	buf := make([]byte, 5)
	for i := 0; i < count; i++ {
		if _, err := rand.Read(buf); err != nil {
			continue
		}
		r := fmt.Sprintf("%x", buf)
		raw = append(raw, r)
		hashed = append(hashed, HashRecoveryCode(r))
	}
	return raw, hashed
}

// HashRecoveryCode: sha256 hex (port hashRecoveryCode).
func HashRecoveryCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", sum)
}
