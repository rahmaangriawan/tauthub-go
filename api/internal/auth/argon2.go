// Package auth: port dari src/auth/session.ts, src/auth/http.ts, dan
// src/lib/auth/* — parameter & format HARUS identik agar data lama valid.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parameter argon2 lama (@node-rs/argon2, session.ts hashPassword):
// algorithm argon2id, memoryCost 19456, timeCost 2, parallelism 1, outputLen 32.
const (
	Argon2Memory  = 19456
	Argon2Time    = 2
	Argon2Threads = 1
	Argon2KeyLen  = 32
	Argon2SaltLen = 16
)

// Format PHC dari @node-rs/argon2 (RustCrypto password-hash):
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash> — base64 standar TANPA padding
// (terverifikasi empiris dari hash yang dihasilkan kode lama).
var phcB64 = base64.RawStdEncoding

// HashPassword menghasilkan PHC string yang identik dengan @node-rs/argon2.
func HashPassword(password string) (string, error) {
	salt := make([]byte, Argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt acak: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, Argon2Time, Argon2Memory, Argon2Threads, Argon2KeyLen)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		Argon2Memory, Argon2Time, Argon2Threads,
		phcB64.EncodeToString(salt), phcB64.EncodeToString(key),
	), nil
}

// VerifyPassword: parse PHC (param dari string hash — bukan konstanta),
// hitung ulang, compare constant-time. Error apa pun → false (parity catch).
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=19456,t=2,p=1", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory, time uint32
	var threads uint8
	for _, kv := range strings.Split(parts[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return false
		}
		switch k {
		case "m":
			n, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return false
			}
			memory = uint32(n)
		case "t":
			n, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return false
			}
			time = uint32(n)
		case "p":
			n, err := strconv.ParseUint(v, 10, 8)
			if err != nil {
				return false
			}
			threads = uint8(n)
		default:
			return false
		}
	}
	salt, err := phcB64.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := phcB64.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
