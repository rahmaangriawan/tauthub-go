package email

import (
	"bufio"
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeSMTP: server SMTP minimal untuk menguji jalur kirim tanpa menyentuh
// penyedia sungguhan. Merekam DATA apa adanya lalu menutup koneksi.
type fakeSMTP struct {
	addr string
	data chan string
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	s := &fakeSMTP{addr: ln.Addr().String(), data: make(chan string, 1)}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		write := func(line string) { conn.Write([]byte(line + "\r\n")) }
		write("220 fake ESMTP")

		inData := false
		var body strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false
					s.data <- body.String()
					body.Reset()
					write("250 2.0.0 OK queued")
					continue
				}
				// Dot-stuffing: baris yang diawali '.' dikirim sebagai '..'.
				body.WriteString(strings.TrimPrefix(line, ".") + "\r\n")
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				write("250-fake")
				write("250 8BITMIME")
			case strings.HasPrefix(line, "MAIL FROM"):
				write("250 2.1.0 OK")
			case strings.HasPrefix(line, "RCPT TO"):
				write("250 2.1.5 OK")
			case line == "DATA":
				inData = true
				write("354 End data with <CR><LF>.<CR><LF>")
			case line == "QUIT":
				write("221 2.0.0 Bye")
				return
			default:
				write("250 OK")
			}
		}
	}()
	return s
}

// smtpSend harus mengirim pesan yang bisa di-parse ulang: header lengkap,
// badan base64 yang decode-nya sama persis dengan HTML aslinya.
func TestSMTPSendDeliversParseableMessage(t *testing.T) {
	srv := startFakeSMTP(t)
	host, portStr, _ := net.SplitHostPort(srv.addr)
	port, _ := strconv.Atoi(portStr)

	body := `<html><body><p>Halo mitra — publikasi disetujui</p></body></html>`
	cfg := &Config{
		Host: host, Port: port, Secure: false,
		FromEmail: "no-reply@tauthub.com", FromName: "Tauthub",
	}
	id, err := smtpSend(cfg, "mitra@example.com", "Publikasi disetujui", body)
	if err != nil {
		t.Fatalf("smtpSend: %v", err)
	}
	if !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, ">") {
		t.Errorf("provider id harus Message-ID, dapat %q", id)
	}

	var raw string
	select {
	case raw = <-srv.data:
	case <-time.After(5 * time.Second):
		t.Fatal("server tidak menerima DATA")
	}

	head, encoded, ok := strings.Cut(raw, "\r\n\r\n")
	if !ok {
		t.Fatal("pesan tanpa pemisah header/badan")
	}
	for _, want := range []string{"Message-ID: " + id, "Content-Transfer-Encoding: base64", "Subject: Publikasi disetujui"} {
		if !strings.Contains(head, want) {
			t.Errorf("header %q hilang di wire", want)
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(encoded, "\r\n", ""))
	if err != nil {
		t.Fatalf("badan bukan base64: %v", err)
	}
	if string(decoded) != body {
		t.Errorf("badan berubah di wire:\n got %q\nwant %q", decoded, body)
	}
}
