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

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
