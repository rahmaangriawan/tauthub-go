// Test kompat fase 1: fixture dibuat oleh KODE LAMA (bun + jose + @node-rs/argon2
// + lib/ai/provider + lib/auth/*). Lihat apps/web/scripts/compat-fixtures.ts di repo lama.
//
//   go test ./internal/compat/ -v
//
// Test ini juga menulis compat/go-output.json (nilai yang DIBUAT Go) untuk
// diverifikasi balik oleh kode lama (arah Go → Node).
package compat

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

type fixtures struct {
	Issuer  string `json:"issuer"`
	UserID  string `json:"user_id"`
	Argon2  struct {
		Password string `json:"password"`
		Hash     string `json:"hash"`
	} `json:"argon2"`
	JWT struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		AccessJTI        string `json:"access_jti"`
		RefreshJTI       string `json:"refresh_jti"`
		SessionExpiresAt string `json:"session_expires_at"`
	} `json:"jwt"`
	SealedV1 struct {
		Plaintext string `json:"plaintext"`
		Sealed    string `json:"sealed"`
	} `json:"sealed_v1"`
	SealedV2 struct {
		Plaintext string `json:"plaintext"`
		Sealed    string `json:"sealed"`
	} `json:"sealed_v2"`
	SealedV3 struct {
		Plaintext string `json:"plaintext"`
		EntityID  string `json:"entity_id"`
		Sealed    string `json:"sealed"`
	} `json:"sealed_v3"`
	Totp struct {
		Secret      string `json:"secret"`
		Counter     int64  `json:"counter"`
		Code        string `json:"code"`
		UnixSeconds int64  `json:"unix_seconds"`
	} `json:"totp"`
	Challenge struct {
		UID    string `json:"uid"`
		Email  string `json:"email"`
		Ticket string `json:"ticket"`
	} `json:"challenge"`
}

func loadFixtures(t *testing.T) fixtures {
	t.Helper()
	raw, err := os.ReadFile("../../compat/fixtures.json")
	if err != nil {
		t.Skipf("fixtures belum digenerate: %v", err)
	}
	var f fixtures
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}
	return f
}

func testSecret(t *testing.T) string {
	t.Helper()
	s := os.Getenv("APP_SECRET")
	if s == "" {
		t.Skip("APP_SECRET tidak diset")
	}
	return s
}

func TestUnsealLegacyV1(t *testing.T) {
	f := loadFixtures(t)
	secret := testSecret(t)
	got, err := crypto.Unseal(f.SealedV1.Sealed, secret)
	if err != nil {
		t.Fatalf("unseal v1 gagal: %v", err)
	}
	if got != f.SealedV1.Plaintext {
		t.Fatalf("v1 plaintext beda: got=%q want=%q", got, f.SealedV1.Plaintext)
	}
}

func TestUnsealV2(t *testing.T) {
	f := loadFixtures(t)
	secret := testSecret(t)
	got, err := crypto.Unseal(f.SealedV2.Sealed, secret)
	if err != nil {
		t.Fatalf("unseal v2 gagal: %v", err)
	}
	if got != f.SealedV2.Plaintext {
		t.Fatalf("v2 plaintext beda")
	}
}

func TestUnsealV3(t *testing.T) {
	f := loadFixtures(t)
	secret := testSecret(t)
	got, err := crypto.UnsealV3(f.SealedV3.Sealed, secret, f.SealedV3.EntityID)
	if err != nil {
		t.Fatalf("unseal v3 gagal: %v", err)
	}
	if got != f.SealedV3.Plaintext {
		t.Fatalf("v3 plaintext beda")
	}
}

func TestArgon2VerifyNodeHash(t *testing.T) {
	f := loadFixtures(t)
	if !auth.VerifyPassword(f.Argon2.Password, f.Argon2.Hash) {
		t.Fatal("hash argon2 buatan Node tidak terverifikasi oleh Go")
	}
	if auth.VerifyPassword("password-salah", f.Argon2.Hash) {
		t.Fatal("password salah seharusnya ditolak")
	}
}

func TestJWTNodeTokens(t *testing.T) {
	f := loadFixtures(t)
	secret := testSecret(t)

	access, err := auth.VerifyAccessToken(secret, f.Issuer, f.JWT.AccessToken)
	if err != nil {
		t.Fatalf("access token Node ditolak Go: %v", err)
	}
	if access.Sub != f.UserID || access.SID != f.JWT.RefreshJTI {
		t.Fatalf("claim access beda: sub=%s sid=%s", access.Sub, access.SID)
	}

	refresh, err := auth.VerifyRefreshToken(secret, f.Issuer, f.JWT.RefreshToken)
	if err != nil {
		t.Fatalf("refresh token Node ditolak Go: %v", err)
	}
	if refresh.JTI != f.JWT.RefreshJTI {
		t.Fatalf("refresh jti beda")
	}

	// type mismatch harus ditolak
	if _, err := auth.VerifyAccessToken(secret, f.Issuer, f.JWT.RefreshToken); err == nil {
		t.Fatal("refresh token diterima sebagai access — type check bocor")
	}
}

func TestTotpNodeSecret(t *testing.T) {
	f := loadFixtures(t)
	if got := auth.HOTPAt(f.Totp.Secret, f.Totp.Counter); got != f.Totp.Code {
		t.Fatalf("HOTPAt beda: got=%s want=%s", got, f.Totp.Code)
	}
	pinned := time.Unix(f.Totp.UnixSeconds, 0)
	restore := auth.PinNow(pinned)
	defer restore()
	if !auth.VerifyTotp(f.Totp.Secret, f.Totp.Code) {
		t.Fatal("VerifyTotp dengan waktu yang di-pin gagal")
	}
}

func TestChallengeNodeTicket(t *testing.T) {
	f := loadFixtures(t)
	secret := testSecret(t)

	// Pin jam ke exp - 1 menit: fixture dibuat sekali, tiket TTL 5 menit bisa
	// kedaluwarsa saat test dijalankan lagi. exp diekstrak dari body tiket.
	payload := struct {
		Exp int64 `json:"exp"`
	}{}
	bodyB64, _, _ := strings.Cut(f.Challenge.Ticket, ".")
	raw, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		t.Fatalf("decode body tiket: %v", err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse body tiket: %v", err)
	}
	restore := auth.PinNowChallenge(time.UnixMilli(payload.Exp - 60_000))
	defer restore()

	got, ok := auth.VerifyTicket(secret, f.Challenge.Ticket)
	if !ok {
		t.Fatal("challenge ticket Node ditolak Go")
	}
	if got.UID != f.Challenge.UID || got.Email != f.Challenge.Email {
		t.Fatalf("payload challenge beda: %+v", got)
	}
}

// Reverse fixtures: nilai yang dibuat GO, untuk diverifikasi kode lama.
func TestWriteReverseFixtures(t *testing.T) {
	f := loadFixtures(t)
	secret := testSecret(t)

	goSealedV1, err := crypto.SealLegacyV1("go-sealed-legacy-value", secret)
	if err != nil {
		t.Fatal(err)
	}
	goSealedV2, err := crypto.Seal("go-sealed-v2-value", secret)
	if err != nil {
		t.Fatal(err)
	}
	goSealedV3, err := crypto.SealV3("go-sealed-v3-value", secret, f.SealedV3.EntityID)
	if err != nil {
		t.Fatal(err)
	}
	goHash, err := auth.HashPassword("go-password-123")
	if err != nil {
		t.Fatal(err)
	}
	pair, err := auth.CreateTokenPair(secret, f.Issuer, f.UserID, 30, 43200, auth.CreateSessionDeadline(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	goTicket, err := auth.SignChallenge(secret, auth.ChallengePayload{UID: f.UserID, Email: "go@test.local"})
	if err != nil {
		t.Fatal(err)
	}

	out := map[string]any{
		"issuer":            f.Issuer,
		"user_id":           f.UserID,
		"go_sealed_v1":      goSealedV1,
		"go_plain_v1":       "go-sealed-legacy-value",
		"go_sealed_v2":      goSealedV2,
		"go_plain_v2":       "go-sealed-v2-value",
		"go_sealed_v3":      goSealedV3,
		"go_plain_v3":       "go-sealed-v3-value",
		"entity_id":         f.SealedV3.EntityID,
		"go_argon2_hash":    goHash,
		"go_argon2_pass":    "go-password-123",
		"go_access_token":   pair.AccessToken,
		"go_refresh_token":  pair.RefreshToken,
		"go_refresh_jti":    pair.RefreshJTI,
		"go_session_exp":    pair.SessionExpiresAt.UTC().Format(time.RFC3339),
		"go_challenge":      goTicket,
		"go_challenge_uid":  f.UserID,
		"go_challenge_mail": "go@test.local",
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("../../compat/go-output.json", raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Log("go-output.json ditulis untuk verifikasi balik via bun")
}
