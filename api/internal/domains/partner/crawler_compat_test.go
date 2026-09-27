package partner

import (
	"encoding/base64"
	"testing"
)

// Kompatibilitas crawler terhadap vektor uji app lama (lib/crawler/verification.test.ts).
// Tuple canonical, fingerprint, dan PKCE challenge HARUS identik byte-per-byte —
// Worker mitra di Cloudflare memakai format yang sama.
func TestCrawlerSignatureCompat(t *testing.T) {
	secretB64 := base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31})
	const (
		crawlerID = "site_abcdefghij"
		ts        = "1737451200"
		nonce     = "9f2c41ab77d04e88b1c2f3a0d5e6c7b8"
		wantCanon = "v1\nsite_abcdefghij\nGET\nexample.com\n/artikel?page=2\n1737451200\n9f2c41ab77d04e88b1c2f3a0d5e6c7b8"
		wantSign  = "694754e9fc8aa7053ecc06818c3a3d3a9501f61f656be716a7513788025075aa"
		wantFP    = "4e24c4e400fe6908"
		wantBF    = "14385deb5ee712bb82f136882e04e7bae94e028f5e81fa19cfa79385efb5cef2"
		wantPKCE  = "WnSqVbF7Y7pjeoO6dJyIsrRrEWwKsy9y10Ct4r1jxtQ"
	)

	canonical := buildCanonical(crawlerID, "get", "Example.COM", "/artikel?page=2", ts, nonce)
	if canonical != wantCanon {
		t.Fatalf("canonical mismatch:\n got %q\nwant %q", canonical, wantCanon)
	}
	if got := crawlerSignRequest(crawlerID, secretB64, "GET", "example.com", "/artikel?page=2", ts, nonce); got != wantSign {
		t.Fatalf("signature mismatch: got %s want %s", got, wantSign)
	}
	if got := crawlerSecretFingerprint(secretB64); got != wantFP {
		t.Fatalf("fingerprint mismatch: got %s want %s", got, wantFP)
	}
	if got := crawlerBodyFingerprint("<title>Just a moment</title>"); got != wantBF {
		t.Fatalf("body fingerprint mismatch: got %s want %s", got, wantBF)
	}
	if got := base64URLSha256("verifier-uji-32-byte-acak-abcdefgh"); got != wantPKCE {
		t.Fatalf("pkce mismatch: got %s want %s", got, wantPKCE)
	}
}

// Secret crawler harus bisa dibuka kembali persis seperti disegel, dan menolak
// master key yang salah (tanpa mengembalikan secret apa pun).
func TestCrawlerSecretSealCompat(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7})
	const secret = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="

	enc, nonce, tag, err := sealCrawlerSecret(secret, key)
	if err != nil {
		t.Fatalf("seal gagal: %v", err)
	}
	got, err := unsealCrawlerSecret(enc, nonce, tag, key)
	if err != nil {
		t.Fatalf("unseal gagal: %v", err)
	}
	if got != secret {
		t.Fatalf("secret berubah: got %q want %q", got, secret)
	}

	wrongKey := base64.StdEncoding.EncodeToString([]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
	if _, err := unsealCrawlerSecret(enc, nonce, tag, wrongKey); err == nil {
		t.Fatal("unseal dengan master key salah seharusnya gagal")
	}
	if _, _, _, err := sealCrawlerSecret(secret, base64.StdEncoding.EncodeToString([]byte{1, 2, 3})); err == nil {
		t.Fatal("master key bukan 32 byte seharusnya ditolak")
	}
}

// Klasifikasi blokir: jangan menuduh Cloudflare tanpa bukti.
func TestCrawlerClassify(t *testing.T) {
	cases := []struct {
		status                 int
		cfRay, cfMitigated, bs string
		want                   string
	}{
		{200, "", "", "artikel", "ok"},
		{403, "RAY123", "", "forbidden", "blocked_waf"},
		{403, "RAY123", "challenge", "<title>Just a moment…</title>", "blocked_cloudflare"},
		{403, "", "", "<title>Attention Required! | Cloudflare</title>", "blocked_cloudflare"},
		{403, "", "", "forbidden", "blocked_origin"},
		{429, "", "", "slow down", "rate_limited"},
	}
	for _, c := range cases {
		if got := classifyCrawlerResponse(c.status, c.cfRay, c.cfMitigated, c.bs); got != c.want {
			t.Fatalf("status %d cfRay=%q mitigated=%q → got %s want %s", c.status, c.cfRay, c.cfMitigated, got, c.want)
		}
	}
}

func TestCrawlerExtractHost(t *testing.T) {
	cases := map[string]string{
		"https://Example.COM/artikel": "example.com",
		"https://example.com:8443/x":  "example.com",
		"example.com/path":            "example.com",
		"":                            "",
	}
	for in, want := range cases {
		if got := extractHost(in); got != want {
			t.Fatalf("extractHost(%q) = %q want %q", in, got, want)
		}
	}
}
