package email

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testCfg() *Config {
	return &Config{
		Host: "smtp.example.com", Port: 587,
		FromEmail: "no-reply@tauthub.com", FromName: "Tauthub",
	}
}

// Pesan harus punya header wajib, badan base64 yang bisa di-decode kembali, dan
// setiap baris di bawah batas 998 karakter RFC 5322.
func TestBuildMessageHeadersAndEncoding(t *testing.T) {
	body := `<p style="` + strings.Repeat("color:#111827;", 200) + `">Halo &amp; selamat datang</p>`
	msg, messageID := buildMessage(testCfg(), "mitra@example.com", "Pengingat publikasi", body)

	for _, want := range []string{
		"Date: ", "From: Tauthub <no-reply@tauthub.com>", "To: mitra@example.com",
		"Subject: Pengingat publikasi", "Message-ID: " + messageID,
		"MIME-Version: 1.0", `Content-Type: text/html; charset="utf-8"`,
		"Content-Transfer-Encoding: base64",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("header %q tidak ada di pesan", want)
		}
	}
	if !strings.HasSuffix(messageID, "@tauthub.com>") {
		t.Errorf("Message-ID harus memakai domain FromEmail, dapat %q", messageID)
	}

	head, encoded, ok := strings.Cut(msg, "\r\n\r\n")
	if !ok {
		t.Fatal("pesan tidak punya pemisah header/badan")
	}
	for _, line := range strings.Split(head+"\r\n"+encoded, "\r\n") {
		if len(line) > 998 {
			t.Fatalf("baris %d karakter, melebihi batas 998", len(line))
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(encoded, "\r\n", ""))
	if err != nil {
		t.Fatalf("badan bukan base64 valid: %v", err)
	}
	if string(decoded) != body {
		t.Errorf("badan tidak utuh setelah decode")
	}
}

// Judul dari data pengguna tidak boleh bisa menyuntik header baru.
func TestBuildMessageRejectsHeaderInjection(t *testing.T) {
	msg, _ := buildMessage(testCfg(), "mitra@example.com",
		"Topik\r\nBcc: penyerang@example.com", "<p>isi</p>")

	head, _, _ := strings.Cut(msg, "\r\n\r\n")
	if strings.Contains(head, "\r\nBcc:") || strings.HasPrefix(head, "Bcc:") {
		t.Fatal("CRLF di subject berhasil menyuntik header Bcc")
	}
	if strings.Count(head, "\r\n") != strings.Count(strings.TrimSuffix(head, "\r\n"), "\n") {
		t.Fatalf("header tidak konsisten: %q", head)
	}
}

// Nama pengirim non-ASCII dan subject non-ASCII di-encode RFC 2047.
func TestBuildMessageEncodesNonASCII(t *testing.T) {
	cfg := testCfg()
	cfg.FromName = "Tauthub Indonesia"
	msg, _ := buildMessage(cfg, "mitra@example.com", "Publikasi melewati batas waktu — tinjau", "<p>x</p>")
	if !strings.Contains(msg, "Subject: =?UTF-8?B?") {
		t.Error("subject non-ASCII harus di-encode")
	}
	if !strings.Contains(msg, "From: Tauthub Indonesia <no-reply@tauthub.com>") {
		t.Error("nama pengirim ASCII biasa tidak perlu di-quote")
	}

	cfg.FromName = "Tauthub — Tim"
	msg, _ = buildMessage(cfg, "mitra@example.com", "Halo", "<p>x</p>")
	if !strings.Contains(msg, `From: =?UTF-8?B?`) {
		t.Error("nama pengirim non-ASCII harus di-encode")
	}
}

// Semua template memakai kerangka brand: logo absolut dan footer kontak.
func TestTemplatesUseBrandLayout(t *testing.T) {
	cases := map[string]map[string]string{
		"password-reset":   {"link": "/reset-password?token=abc"},
		"password-changed": {},
		"account-invite":   {"link": "/reset-password?token=abc", "display_name": "Rahma & Rekan", "role": "klien"},
	}
	for name, data := range cases {
		r, err := RenderTemplate(name, data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(r.HTML, BrandLogoURL()) {
			t.Errorf("%s: logo brand tidak ada", name)
		}
		if !strings.Contains(r.HTML, layoutSupportEmail) {
			t.Errorf("%s: footer kontak tidak ada", name)
		}
		if strings.Contains(r.HTML, "<script") {
			t.Errorf("%s: tidak boleh ada script", name)
		}
	}
}

// Tautan relatif harus jadi absolut, dan nilai dinamis harus di-escape.
func TestDetailEscapesAndAbsolutizes(t *testing.T) {
	r := Detail("Judul", "Intro <b>mentah</b>", [][2]string{{"Nama", "<script>alert(1)</script>"}}, "/mitra/kxaax7", "Buka")
	if strings.Contains(r.HTML, "<script>") {
		t.Fatal("nilai dinamis tidak di-escape")
	}
	if !strings.Contains(r.HTML, `href="`+AppBaseURL()+`/mitra/kxaax7"`) {
		t.Error("tautan relatif harus dijadikan absolut")
	}
	if !strings.Contains(r.HTML, "Intro &lt;b&gt;mentah&lt;/b&gt;") {
		t.Error("intro harus di-escape, bukan dirender sebagai HTML")
	}
}
