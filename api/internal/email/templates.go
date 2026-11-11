package email

import (
	"encoding/json"
	"html"
	"strings"
	"time"
)

// Port email/templates/base.ts + registry password-reset/changed/invite.
// Kerangka HTML-nya ada di layout.go supaya semua email memakai header logo
// dan footer yang sama.

type Rendered struct {
	Subject string
	HTML    string
}

func escapeHTML(value string) string {
	return html.EscapeString(value)
}

// renderPasswordReset: parity registerTemplate("password-reset").
func renderPasswordReset(style Style, data map[string]string) Rendered {
	return DetailStyled(
		style,
		"Reset password Tauthub",
		"Kami menerima permintaan reset password untuk akun Anda. Klik tombol di bawah untuk melanjutkan. Link berlaku 10 menit.",
		nil,
		data["link"],
		"Reset Password",
	)
}

// renderPasswordChanged: parity registerTemplate("password-changed").
func renderPasswordChanged(style Style, data map[string]string) Rendered {
	changedAt := data["changed_at"]
	if changedAt == "" {
		changedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return DetailStyled(
		style,
		"Password Tauthub Anda telah diubah",
		"Password akun Anda berhasil diperbarui. Jika bukan Anda yang melakukan, segera hubungi support.",
		[][2]string{{"Waktu", changedAt}},
		"", "",
	)
}

// renderAccountInvite: email aktivasi untuk user yang dibuat admin.
//
// Data yang tersedia dari pemanggil (operations.go:2214): link reset token yang
// berlaku 60 menit, display_name, dan role. Token itu memang token reset, jadi
// tautannya mengarah ke halaman atur password, bukan halaman login.
func renderAccountInvite(style Style, data map[string]string) Rendered {
	name := strings.TrimSpace(data["display_name"])
	greeting := "Akun Tauthub Anda sudah dibuat."
	if name != "" {
		greeting = "Halo " + name + ", akun Tauthub Anda sudah dibuat."
	}
	rows := [][2]string{}
	if role := strings.TrimSpace(data["role"]); role != "" {
		rows = append(rows, [2]string{"Peran", role})
	}
	return DetailStyled(
		style,
		"Aktifkan akun Tauthub Anda",
		greeting+" Atur password lewat tombol di bawah untuk mulai masuk. Tautan berlaku 60 menit.",
		rows,
		data["link"],
		"Atur Password",
	)
}

// renderBrandInvitation: undangan bergabung ke brand untuk email yang belum
// punya akun. Tautan membawa token sekali pakai; peran terlihat di baris rincian.
func renderBrandInvitation(style Style, data map[string]string) Rendered {
	inviter := strings.TrimSpace(data["inviter"])
	if inviter == "" {
		inviter = "Seorang admin"
	}
	brand := strings.TrimSpace(data["brand"])
	return DetailStyled(
		style,
		"Undangan bergabung ke "+brand+" di Tauthub",
		inviter+" mengundang Anda bergabung ke brand "+brand+". Buka tautan di bawah, daftar atau masuk memakai email ini, lalu terima undangan. Tautan berlaku 7 hari.",
		[][2]string{{"Brand", brand}, {"Peran", data["role"]}},
		data["link"],
		"Terima undangan",
	)
}

// renderEmailVerificationCode: kode OTP pendaftaran (sebelum akun dibuat).
// Kode ditampilkan besar dan berjarak supaya mudah disalin dari ponsel.
func renderEmailVerificationCode(style Style, data map[string]string) Rendered {
	code := strings.TrimSpace(data["kode"])
	bodyHTML := `<p style="margin:0;font-size:15px;line-height:1.6;color:` + style.text() + `;">Gunakan kode di bawah untuk melanjutkan pendaftaran akun Tauthub Anda.</p>` +
		`<p style="margin:20px 0 0;text-align:center;"><span style="display:inline-block;padding:14px 28px;font-size:30px;line-height:1.2;font-weight:700;letter-spacing:8px;font-family:SFMono-Regular,Menlo,Consolas,'Liberation Mono',monospace;color:` + style.text() + `;background:` + layoutPanel + `;border:1px solid ` + layoutRule + `;border-radius:10px;">` + escapeHTML(code) + `</span></p>` +
		`<p style="margin:20px 0 0;font-size:13px;line-height:1.6;color:` + style.muted() + `;">Kode berlaku 10 menit dan hanya bisa dipakai satu kali. Bila Anda tidak mendaftar di Tauthub, abaikan email ini.</p>`
	return DetailHTMLStyled(style, "Kode verifikasi Tauthub", bodyHTML, "", "")
}

// renderAccountExistsNotice: dikirim saat email yang sudah punya akun dipakai
// mendaftar lagi. Respons HTTP tetap netral; pemilik asli diberi tahu lewat
// email ini.
func renderAccountExistsNotice(style Style, data map[string]string) Rendered {
	link := strings.TrimSpace(data["link"])
	if link == "" {
		link = "/login"
	}
	return DetailStyled(
		style,
		"Email ini sudah punya akun Tauthub",
		"Seseorang mencoba mendaftar dengan email ini, tetapi email ini sudah terdaftar di Tauthub. Masuk memakai akun yang ada, atau pakai \"Lupa password\" di halaman masuk bila lupa kata sandi. Bila bukan Anda yang mencoba mendaftar, abaikan email ini.",
		nil,
		link,
		"Masuk ke Tauthub",
	)
}

// renderLoginAlert: dikirim pada setiap login berhasil (password, 2FA, Google).
func renderLoginAlert(style Style, data map[string]string) Rendered {
	name := strings.TrimSpace(data["nama"])
	if name == "" {
		name = "Sahabat Tauthub"
	}
	return DetailStyled(
		style,
		"Login baru ke akun Tauthub Anda",
		"Halo "+name+", kami mendeteksi bahwa Anda berhasil login dengan perangkat berikut. "+
			"Jika ini adalah Anda, abaikan pesan ini. Jika bukan Anda, silakan hubungi kami di WA : care@tauthub.com atau Email : +62 851-2466-5015.",
		[][2]string{
			{"IP Address", data["ip"]},
			{"Tanggal dan Waktu", data["waktu"]},
			{"Web Browser", data["browser"]},
			{"Platform", data["platform"]},
		},
		"", "",
	)
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
