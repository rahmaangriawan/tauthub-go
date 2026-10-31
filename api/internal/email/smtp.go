package email

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Port email/transport.ts getTransport().sendMail — Go net/smtp:
// secure=true → implicit TLS; port 587 → wajib STARTTLS bila server mendukung
// (parity requireTLS); auth PLAIN bila username diisi.

func smtpSend(cfg *Config, to, subject, htmlBody string) (string, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	msg, messageID := buildMessage(cfg, to, subject, htmlBody)

	host := cfg.Host
	var client *smtp.Client
	var err error

	if cfg.Secure {
		conn, dErr := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
		if dErr != nil {
			return "", fmt.Errorf("smtp dial TLS: %w", dErr)
		}
		client, err = smtp.NewClient(conn, host)
	} else {
		client, err = smtp.Dial(addr)
	}
	if err != nil {
		return "", fmt.Errorf("smtp connect: %w", err)
	}
	defer client.Close()

	// requireTLS parity untuk port 587 non-secure.
	if !cfg.Secure && cfg.Port == 587 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return "", fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, host)
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return "", fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	if err := client.Mail(cfg.FromEmail); err != nil {
		return "", fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return "", fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return "", fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return "", fmt.Errorf("smtp quit: %w", err)
	}

	return messageID, nil
}

// buildMessage merangkai pesan MIME lengkap.
//
// Tiga hal yang membuat pesan ini diterima dengan baik:
//
//  1. Date + Message-ID. Tanpa keduanya banyak filter (dan Gmail) menandai
//     pesan sebagai mencurigakan, dan pesan yang sama tidak ter-thread di
//     kotak masuk penerima.
//  2. Content-Transfer-Encoding: base64. HTML email kita punya baris sangat
//     panjang (atribut style inline satu baris). RFC 5322 membatasi 998
//     karakter per baris, dan UTF-8 8-bit tanpa deklarasi encoding bikin
//     server ketat menolak atau memotong isi. Base64 membuat semuanya ASCII
//     7-bit dan baris pendek (dibungkus 76 karakter).
//  3. Nilai header disanitasi dari CR/LF. Judul email memuat data pengguna
//     (mis. topik artikel), jadi tanpa ini karakter baris baru bisa menyuntik
//     header tambahan.
func buildMessage(cfg *Config, to, subject, htmlBody string) (string, string) {
	fromName := strings.TrimSpace(cfg.FromName)
	if fromName == "" {
		fromName = "Tauthub"
	}
	from := sanitizeHeader(cfg.FromEmail)
	recipient := sanitizeHeader(to)

	messageID := fmt.Sprintf("<%s@%s>", uuid.NewString(), messageIDDomain(from))

	headers := [][2]string{
		{"Date", time.Now().Format(time.RFC1123Z)},
		{"From", encodeDisplayName(fromName) + " <" + from + ">"},
		{"To", recipient},
		{"Subject", mimeHeaderEncode(sanitizeHeader(subject))},
		{"Message-ID", messageID},
		{"MIME-Version", "1.0"},
		{"Content-Type", `text/html; charset="utf-8"`},
		{"Content-Transfer-Encoding", "base64"},
		// Menandai pesan sebagai otomatis supaya autoresponder penerima tidak
		// membalas dan membuat loop.
		{"Auto-Submitted", "auto-generated"},
	}

	var msg strings.Builder
	for _, h := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", h[0], h[1])
	}
	msg.WriteString("\r\n")
	msg.WriteString(base64Wrap(htmlBody, 76))
	return msg.String(), messageID
}

// sanitizeHeader membuang CR/LF (dan spasi kendali) agar nilai header tidak
// bisa memecah pesan menjadi header tambahan.
func sanitizeHeader(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return ' '
		}
		return r
	}, v)
}

// encodeDisplayName: nama pengirim non-ASCII dibungkus encoded-word RFC 2047
// supaya tidak tampil sebagai mojibake ("Ã¢Â€Â¦") di Outlook.
func encodeDisplayName(name string) string {
	if isASCII(name) {
		// Nama dengan karakter khusus tetap perlu quoting agar koma/titik dua
		// tidak dibaca sebagai pemisah alamat.
		if strings.ContainsAny(name, `,;:"<>@[]()`) {
			return `"` + strings.ReplaceAll(name, `"`, `\"`) + `"`
		}
		return name
	}
	return "=?UTF-8?B?" + base64Std(name) + "?="
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// messageIDDomain: domain dari FromEmail, dipakai sebagai bagian kanan
// Message-ID. Alamat tanpa "@" jatuh ke domain aplikasi.
func messageIDDomain(fromEmail string) string {
	if i := strings.LastIndex(fromEmail, "@"); i >= 0 && i+1 < len(fromEmail) {
		return strings.Trim(fromEmail[i+1:], "<> ")
	}
	return "tauthub.com"
}

// base64Wrap: encode base64 lalu bungkus per `width` karakter dengan CRLF,
// sesuai batas baris MIME.
func base64Wrap(s string, width int) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	var b strings.Builder
	for len(encoded) > width {
		b.WriteString(encoded[:width])
		b.WriteString("\r\n")
		encoded = encoded[width:]
	}
	b.WriteString(encoded)
	b.WriteString("\r\n")
	return b.String()
}

func base64Std(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func mimeHeaderEncode(s string) string {
	// Subject plain UTF-8 cukup untuk konten kita; encode bila ada non-ASCII.
	if isASCII(s) {
		return s
	}
	return "=?UTF-8?B?" + base64Std(s) + "?="
}
