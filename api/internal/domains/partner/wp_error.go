package partner

import (
	"context"
	"errors"
	"net"
	"strings"
)

// describeWordPressNetErr: ubah error jaringan mentah dari wpRequest menjadi
// pesan yang bisa ditindaklanjuti di UI. Tanpa ini, kolom "Test terakhir"
// hanya menampilkan dump Go seperti `Get "https://…": context deadline exceeded`,
// yang tidak memberi tahu penyebab maupun langkah berikutnya.
func describeWordPressNetErr(err error, target string) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "Situs tidak merespons dalam 20 detik (timeout). Hosting situs kemungkinan sedang tidak dapat dijangkau — coba lagi nanti atau periksa status server WordPress-nya. [" + target + "]"
	case isTimeoutErr(err):
		return "Koneksi ke situs kehabisan waktu. Periksa apakah situs dapat diakses dari jaringan lain. [" + target + "]"
	case errors.Is(err, context.Canceled):
		return "Permintaan dibatalkan sebelum selesai."
	case isConnResetErr(err):
		return "Koneksi diputus oleh jaringan atau server situs (connection reset). Situs tidak dapat dijangkau dari server ini saat ini. [" + target + "]"
	}

	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "eof"):
		return "Situs menutup koneksi tanpa balasan (EOF). Hosting situs tidak dapat dijangkau dari server ini. [" + target + "]"
	case strings.Contains(lower, "x509") || strings.Contains(lower, "certificate"):
		return "Sertifikat SSL situs tidak valid atau tidak dapat diverifikasi. Periksa konfigurasi HTTPS situs. [" + target + "]"
	case strings.Contains(lower, "no such host") || strings.Contains(lower, "dns"):
		return "Nama domain situs tidak dapat di-resolve (DNS). Periksa ejaan URL situs. [" + target + "]"
	case strings.Contains(lower, "connection refused"):
		return "Situs menolak koneksi (port 443 tertutup atau server mati). [" + target + "]"
	case strings.Contains(lower, "diblokir safe-fetch"):
		return "URL situs menunjuk ke alamat internal/privat sehingga diblokir oleh pengaman SSRF. [" + target + "]"
	}
	return msg
}

func isTimeoutErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout()
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

func isConnResetErr(err error) bool {
	var oe *net.OpError
	if errors.As(err, &oe) {
		return strings.Contains(strings.ToLower(oe.Error()), "reset")
	}
	return strings.Contains(strings.ToLower(err.Error()), "reset by peer")
}

// wordpressTestGuidance: ubah badan error REST WordPress menjadi pesan dengan
// langkah perbaikan, bukan dump JSON. Semua situs yang diamati membalas
// kegagalan auth dasar dengan kode generik rest_not_logged_in — dari luar
// tidak bisa dibedakan apakah kredensialnya salah atau header Authorization
// disaring hosting, jadi petunjuknya mencakup keduanya. Kode aslinya tetap
// disertakan supaya bisa didiagnosis lebih lanjut.
func wordpressTestGuidance(status int, body string) string {
	var parsed struct {
		Code string `json:"code"`
	}
	_ = jsonUnmarshal(body, &parsed)
	switch parsed.Code {
	case "rest_not_logged_in":
		return "WordPress menolak kredensial ini (rest_not_logged_in). Periksa: 1) username memang ada di situs dan ditulis benar; 2) Application Password masih aktif, buat baru di WP Admin lewat Pengguna → Profil; 3) simpan ulang koneksi lalu uji lagi. Kalau password baru pun tetap gagal, minta penyedia hosting mengizinkan header Authorization."
	case "incorrect_password":
		return "Application Password salah untuk username ini (incorrect_password). Buat password aplikasi baru di WP Admin (Pengguna → Profil → Application Passwords), lalu perbarui koneksi dan uji lagi."
	case "invalid_username":
		return "Username tidak dikenali WordPress (invalid_username). Periksa ejaan username di situs, lalu perbarui koneksi dan uji lagi."
	case "rest_forbidden", "rest_cannot_create":
		return "User ini tidak punya izin membuat post (rest_forbidden). Pastikan rolenya minimal Author dan Application Password dibuat untuk user tersebut."
	case "rest_no_route":
		return "REST API situs tidak mengenali endpoint ini (rest_no_route). Pastikan WordPress 5.6+ dan plugin keamanan tidak mematikan REST API."
	}
	return "WordPress menanggapi status " + itoa(status) + ": " + truncateRunes(body, 300)
}
