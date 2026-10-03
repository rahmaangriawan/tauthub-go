// Package crypto: sealed-secret envelope yang byte-compatible dengan
// src/lib/ai/provider.ts (v1 legacy XOR+HMAC dari cms_security.py, v2 `t2:`,
// v3 `t3:` per-entity). Unseal semua versi; seal baru selalu v3.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	macLen    = 32
	gcmIVLen  = 12
	gcmTagLen = 16

	V2Prefix = "t2:"
	V3Prefix = "t3:"
)

var ErrInvalidSealed = errors.New("Invalid sealed secret")

func hashKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// DeriveEntityKey: sha256(appSecret + ":" + entityId) dalam base64 standar
// (port deriveEntityKey di provider.ts).
func DeriveEntityKey(appSecret, entityId string) string {
	sum := sha256.Sum256([]byte(appSecret + ":" + entityId))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// b64Decode toleran: Node `Buffer.from(s, "base64url")` menerima base64url
// tanpa padding DAN base64 standar — replikasi toleransinya.
func b64Decode(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.StdEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("decode base64 gagal")
}

// keystream: blok HMAC-SHA256 counter (uint64 BE dari 0), byte-compatible
// dengan cms_security.py `_stream` dan provider.ts keystream().
func keystream(key []byte, length int) []byte {
	out := make([]byte, length)
	filled := 0
	var counter uint64
	buf := make([]byte, 8)
	for filled < length {
		binary.BigEndian.PutUint64(buf, counter)
		mac := hmac.New(sha256.New, key)
		mac.Write(buf)
		block := mac.Sum(nil)
		n := copy(out[filled:], block)
		filled += n
		counter++
	}
	return out
}

func xorBytes(data, stream []byte) []byte {
	out := make([]byte, len(data))
	for i := range data {
		out[i] = data[i] ^ stream[i]
	}
	return out
}

func gcmOpen(key []byte, raw []byte) (string, error) {
	if len(raw) < gcmIVLen+gcmTagLen {
		return "", ErrInvalidSealed
	}
	iv := raw[:gcmIVLen]
	tag := raw[gcmIVLen : gcmIVLen+gcmTagLen]
	encrypted := raw[gcmIVLen+gcmTagLen:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	// Layout Node: iv + tag + ciphertext. Go GCM.Open butuh ciphertext||tag.
	msg := make([]byte, 0, len(encrypted)+len(tag))
	msg = append(msg, encrypted...)
	msg = append(msg, tag...)
	plain, err := gcm.Open(nil, iv, msg, nil)
	if err != nil {
		return "", ErrInvalidSealed
	}
	return string(plain), nil
}

func gcmSeal(key []byte, secret string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := make([]byte, gcmIVLen)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, iv, []byte(secret), nil)
	// pisah ciphertext || tag → layout Node: iv + tag + ciphertext
	tag := sealed[len(sealed)-gcmTagLen:]
	ct := sealed[:len(sealed)-gcmTagLen]
	out := make([]byte, 0, gcmIVLen+gcmTagLen+len(ct))
	out = append(out, iv...)
	out = append(out, tag...)
	out = append(out, ct...)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

// Unseal: v2 (`t2:` AES-256-GCM) atau legacy v1 (unprefixed, XOR keystream +
// HMAC-SHA256) — port unsealSecret().
func Unseal(sealed, appSecret string) (string, error) {
	key := hashKey(appSecret)
	if strings.HasPrefix(sealed, V2Prefix) {
		raw, err := b64Decode(sealed[len(V2Prefix):])
		if err != nil {
			return "", ErrInvalidSealed
		}
		return gcmOpen(key, raw)
	}

	// Legacy v1.
	raw, err := b64Decode(sealed)
	if err != nil {
		return "", ErrInvalidSealed
	}
	if len(raw) < macLen {
		return "", ErrInvalidSealed
	}
	mac := raw[:macLen]
	cipherBytes := raw[macLen:]
	expected := hmac.New(sha256.New, key)
	expected.Write(cipherBytes)
	if subtle.ConstantTimeCompare(mac, expected.Sum(nil)) != 1 {
		return "", ErrInvalidSealed
	}
	return string(xorBytes(cipherBytes, keystream(key, len(cipherBytes)))), nil
}

// UnsealCompat: coba beberapa secret berurutan sampai satu berhasil. Data lama
// bisa ter-seal dengan APP_SECRET (format provider.ts legacy) sementara runtime
// baru menurunkan secret dari CMS_ENCRYPTION_SECRET — GCM/HMAC menolak secret
// yang salah, jadi mencoba keduanya tidak punya false positive.
func UnsealCompat(sealed string, secrets ...string) (string, error) {
	var err error = ErrInvalidSealed
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		if plain, uErr := Unseal(sealed, secret); uErr == nil {
			return plain, nil
		} else {
			err = uErr
		}
	}
	return "", err
}

// Seal: v2 AES-256-GCM (dipakai hanya untuk data lama yang menyimpan v2).
func Seal(secret, appSecret string) (string, error) {
	body, err := gcmSeal(hashKey(appSecret), secret)
	if err != nil {
		return "", err
	}
	return V2Prefix + body, nil
}

// SealLegacyV1: XOR keystream + HMAC-SHA256 — invers Unseal v1. Hanya untuk
// kebutuhan test/compat; penulisan baru harus v3.
func SealLegacyV1(secret, appSecret string) (string, error) {
	key := hashKey(appSecret)
	plain := []byte(secret)
	stream := keystream(key, len(plain))
	cipherBytes := xorBytes(plain, stream)
	mac := hmac.New(sha256.New, key)
	mac.Write(cipherBytes)
	out := make([]byte, 0, macLen+len(cipherBytes))
	out = append(out, mac.Sum(nil)...)
	out = append(out, cipherBytes...)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

// UnsealV3 / SealV3: per-entity, key = sha256(deriveEntityKey).
// entityId biasanya id media/connection/user (kolom enc_key_ver >= 1).
func UnsealV3(sealed, appSecret, entityId string) (string, error) {
	if !strings.HasPrefix(sealed, V3Prefix) {
		return "", ErrInvalidSealed
	}
	raw, err := b64Decode(sealed[len(V3Prefix):])
	if err != nil {
		return "", ErrInvalidSealed
	}
	return gcmOpen(hashKey(DeriveEntityKey(appSecret, entityId)), raw)
}

func SealV3(secret, appSecret, entityId string) (string, error) {
	body, err := gcmSeal(hashKey(DeriveEntityKey(appSecret, entityId)), secret)
	if err != nil {
		return "", err
	}
	return V3Prefix + body, nil
}
