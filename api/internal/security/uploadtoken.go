package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// uploadSignatureLen: potongan base64url dari HMAC 256 bit (22 karakter = 128 bit).
const uploadSignatureLen = 22

// UploadSignatureParam: nama query untuk tanda tangan berkas unggahan.
const UploadSignatureParam = "s"

// SignUploadKey: tanda tangan deterministik untuk berkas /uploads/{key}.
//
// Dipakai supaya gambar artikel yang sudah terbit di situs media (Blogger,
// custom web) bisa dibuka pengunjung tanpa sesi login, sementara berkas
// sensitif lain di direktori yang sama (bukti transfer, sertifikat) tetap
// tertutup: tanpa `s` yang sah, /uploads/{key} tetap meminta login.
func SignUploadKey(appSecret, key string) string {
	if appSecret == "" || key == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte("uploads:" + key))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if len(sig) > uploadSignatureLen {
		sig = sig[:uploadSignatureLen]
	}
	return sig
}

// VerifyUploadKey: cocokkan tanda tangan dari query `?s=` dengan hitungan sendiri.
func VerifyUploadKey(appSecret, key, signature string) bool {
	expected := SignUploadKey(appSecret, key)
	if expected == "" || signature == "" {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(signature))
}

// UploadKeyFromPath: key berkas dari nilai `/uploads/<key>` (relatif maupun
// absolut ke origin sendiri). String kosong bila bukan berkas unggahan.
func UploadKeyFromPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if i := strings.Index(trimmed, "://"); i >= 0 {
		rest := trimmed[i+3:]
		slash := strings.IndexByte(rest, '/')
		if slash < 0 {
			return ""
		}
		trimmed = rest[slash:]
	}
	if !strings.HasPrefix(trimmed, "/uploads/") {
		return ""
	}
	key := strings.TrimPrefix(trimmed, "/uploads/")
	if cut := strings.IndexAny(key, "?#"); cut >= 0 {
		key = key[:cut]
	}
	if key == "" || strings.ContainsAny(key, "/\\") {
		return ""
	}
	return key
}

// WithUploadSignature: tambahkan `?s=` pada URL berkas unggahan; nilai lain
// dikembalikan apa adanya. Query yang sudah punya tanda tangan tidak diubah.
func WithUploadSignature(value, appSecret string) string {
	key := UploadKeyFromPath(value)
	if key == "" {
		return value
	}
	if strings.Contains(value, UploadSignatureParam+"=") {
		return value
	}
	sig := SignUploadKey(appSecret, key)
	if sig == "" {
		return value
	}
	sep := "?"
	if strings.Contains(value, "?") {
		sep = "&"
	}
	return value + sep + UploadSignatureParam + "=" + sig
}
