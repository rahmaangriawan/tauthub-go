package email

import (
	"encoding/json"
	"html"
	"strings"
	"time"
)

// Port email/templates/base.ts + bagian registry untuk password-reset/changed.
// HTML & subject dipertahankan (subject = title, gaya detailTemplate).

type Rendered struct {
	Subject string
	HTML    string
}

const brandColor = "#1d4ed8"
const textColor = "#0f172a"
const mutedColor = "#475569"

func escapeHTML(value string) string {
	return html.EscapeString(value)
}

// baseTemplate: wrapper HTML email transaksional (Indonesia, mobile-first).
func baseTemplate(title, bodyHTML, link, linkLabel string) Rendered {
	htmlDoc := `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + escapeHTML(title) + `</title>
</head>
<body style="margin:0;padding:24px;background:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:` + textColor + `;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:12px;overflow:hidden;">
    <tr>
      <td style="padding:24px;">
        <h1 style="margin:0 0 16px;font-size:20px;">` + escapeHTML(title) + `</h1>
        ` + bodyHTML + `
      </td>
    </tr>
    <tr><td style="padding:16px 24px;background:#f8fafc;color:` + mutedColor + `;font-size:12px;">
      Email otomatis dari Tauthub — ` + escapeHTML(time.Now().Format("2006-01-02 15:04 MST")) + `
    </td></tr>
  </table>
</body>
</html>`
	return Rendered{Subject: title, HTML: htmlDoc}
}

// detailTemplate parity: intro + optional rows + optional link button.
func detailTemplate(title, intro string, rows [][2]string, link, linkLabel string) Rendered {
	var b strings.Builder
	b.WriteString(`<p style="margin:0;line-height:1.55;font-size:14px;">` + escapeHTML(intro) + `</p>`)
	if len(rows) > 0 {
		b.WriteString(`<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="margin:16px 0;border-collapse:collapse;">`)
		for _, r := range rows {
			b.WriteString(`<tr><td style="padding:6px 0;color:` + mutedColor + `;font-size:13px;width:120px;">` + escapeHTML(r[0]) + `</td><td style="padding:6px 0;font-size:13px;font-weight:600;">` + escapeHTML(r[1]) + `</td></tr>`)
		}
		b.WriteString(`</table>`)
	}
	if link != "" {
		b.WriteString(`<p style="margin:24px 0;text-align:center;"><a href="` + escapeHTML(link) + `" target="_blank" rel="noopener" style="background:` + brandColor + `;color:#ffffff;padding:12px 24px;border-radius:8px;text-decoration:none;font-weight:600;display:inline-block;">` + escapeHTML(linkLabel) + `</a></p>`)
	}
	return baseTemplate(title, b.String(), "", "")
}

// renderPasswordReset: parity registerTemplate("password-reset").
func renderPasswordReset(data map[string]string) Rendered {
	return detailTemplate(
		"Reset password Tauthub",
		"Kami menerima permintaan reset password untuk akun Anda. Klik tombol di bawah untuk melanjutkan. Link berlaku 10 menit.",
		nil,
		data["link"],
		"Reset Password",
	)
}

// renderPasswordChanged: parity registerTemplate("password-changed").
func renderPasswordChanged(data map[string]string) Rendered {
	changedAt := data["changed_at"]
	if changedAt == "" {
		changedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return detailTemplate(
		"Password Tauthub Anda telah diubah",
		"Password akun Anda berhasil diperbarui. Jika bukan Anda yang melakukan, segera hubungi support.",
		[][2]string{{"Waktu", changedAt}},
		"", "",
	)
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
