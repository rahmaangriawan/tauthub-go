package security

import "testing"

func TestSignUploadKeyStabilDanTidakTertebak(t *testing.T) {
	const secret = "rahasia-app"
	key := "f1da7a76-46f3-4f5c-bb04-9c15eddeb020.webp"

	first := SignUploadKey(secret, key)
	if first == "" {
		t.Fatal("tanda tangan kosong")
	}
	if first != SignUploadKey(secret, key) {
		t.Fatal("tanda tangan harus deterministik supaya URL tidak berubah tiap terbit")
	}
	if len(first) != uploadSignatureLen {
		t.Fatalf("panjang tanda tangan %d, mau %d", len(first), uploadSignatureLen)
	}
	if first == SignUploadKey("rahasia-lain", key) {
		t.Fatal("secret berbeda harus menghasilkan tanda tangan berbeda")
	}
	if first == SignUploadKey(secret, key+"x") {
		t.Fatal("key berbeda harus menghasilkan tanda tangan berbeda")
	}
	if SignUploadKey("", key) != "" || SignUploadKey(secret, "") != "" {
		t.Fatal("secret/key kosong harus menghasilkan tanda tangan kosong")
	}
}

func TestVerifyUploadKey(t *testing.T) {
	const secret = "rahasia-app"
	key := "a00e80d5-83c1-4da3-aada-aee7c47be6ba.webp"
	sig := SignUploadKey(secret, key)

	if !VerifyUploadKey(secret, key, sig) {
		t.Fatal("tanda tangan sah harus diterima")
	}
	if VerifyUploadKey(secret, key, sig+"x") {
		t.Fatal("tanda tangan salah harus ditolak")
	}
	if VerifyUploadKey(secret, key, "") {
		t.Fatal("tanda tangan kosong harus ditolak")
	}
	if VerifyUploadKey(secret, "key-lain.webp", sig) {
		t.Fatal("tanda tangan milik berkas lain harus ditolak")
	}
	if VerifyUploadKey("", key, sig) {
		t.Fatal("tanpa secret tidak boleh menerima apa pun")
	}
}

func TestUploadKeyFromPath(t *testing.T) {
	cases := map[string]string{
		"/uploads/abc.webp":                              "abc.webp",
		"https://app.tauthub.com/uploads/abc.webp":       "abc.webp",
		"https://app.tauthub.com/uploads/abc.webp?s=xyz": "abc.webp",
		"/uploads/abc.webp?x=1#frag":                     "abc.webp",
		"/uploads/":                                      "",
		"/uploads/a/b.webp":                              "",
		"/uploads/../etc/passwd":                         "",
		"https://cdn.lain.com/gambar.webp":               "",
		"/assets/abc.webp":                               "",
		"":                                               "",
	}
	for in, want := range cases {
		if got := UploadKeyFromPath(in); got != want {
			t.Errorf("UploadKeyFromPath(%q) = %q, mau %q", in, got, want)
		}
	}
}

func TestWithUploadSignature(t *testing.T) {
	const secret = "rahasia-app"
	signed := WithUploadSignature("/uploads/abc.webp", secret)
	if signed != "/uploads/abc.webp?s="+SignUploadKey(secret, "abc.webp") {
		t.Fatalf("hasil tanda tangan salah: %s", signed)
	}
	if again := WithUploadSignature(signed, secret); again != signed {
		t.Fatalf("tanda tangan ganda: %s", again)
	}
	if other := WithUploadSignature("https://cdn.lain.com/a.webp", secret); other != "https://cdn.lain.com/a.webp" {
		t.Fatalf("URL luar tidak boleh diubah: %s", other)
	}
	if empty := WithUploadSignature("", secret); empty != "" {
		t.Fatalf("nilai kosong harus tetap kosong: %q", empty)
	}
}
