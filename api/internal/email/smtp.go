package email

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// Port email/transport.ts getTransport().sendMail — Go net/smtp:
// secure=true → implicit TLS; port 587 → wajib STARTTLS bila server mendukung
// (parity requireTLS); auth PLAIN bila username diisi.

func smtpSend(cfg *Config, to, subject, htmlBody string) (string, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	from := cfg.FromEmail
	fromName := cfg.FromName
	if fromName == "" {
		fromName = "Tauthub"
	}

	headers := map[string]string{
		"From":         fmt.Sprintf("%q <%s>", fromName, from),
		"To":           to,
		"Subject":      mimeHeaderEncode(subject),
		"MIME-Version": "1.0",
		"Content-Type": `text/html; charset="utf-8"`,
	}
	var msg strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
	}
	msg.WriteString("\r\n" + htmlBody)

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

	if err := client.Mail(from); err != nil {
		return "", fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return "", fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		return "", fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return "", fmt.Errorf("smtp quit: %w", err)
	}

	// net/smtp tidak membedakan message id — pakai timestamp sebagai provider id.
	return fmt.Sprintf("go-smtp-%d", time.Now().UnixNano()), nil
}

func base64Std(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func mimeHeaderEncode(s string) string {
	// Subject plain UTF-8 cukup untuk konten kita; encode bila ada non-ASCII.
	needs := false
	for _, r := range s {
		if r > 127 {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	return "=?UTF-8?B?" + base64Std(s) + "?="
}
