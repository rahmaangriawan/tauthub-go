package email

import (
	"os"
	"strings"
	"time"
)

// Kerangka email transaksional Tauthub.
//
// Semua pengirim (auth, brandflow, platform, digest) memakai kerangka ini
// supaya tampilan di inbox seragam dan logo benar-benar muncul, bukan hanya
// nama aplikasi dalam teks. Sebelumnya ada empat renderer terpisah: template
// auth tanpa logo, notifikasi platform tanpa logo dan tanpa footer, digest
// berupa fragmen <ul> tanpa bungkus, dan pengingat WhatsApp berupa <p> polos.
//
// Batasan klien email yang menentukan bentuk markup di bawah:
//   - <style> di <head> dibuang Gmail/Outlook → semua gaya harus inline.
//   - flex/grid tidak didukung → tata letak memakai tabel role="presentation".
//   - Outlook tidak mengenal border-radius pada <table> biasa dan mengabaikan
//     max-width → lebar 560px ditulis eksplisit, plus width:100% untuk HP.
//   - WebP belum didukung Outlook lama → logo memakai PNG.

// Palet disalin dari token aplikasi web (src/app.css) agar email senada.
const (
	layoutPrimary   = "#2b67ec"
	layoutBg        = "#f6f8fc"
	layoutCard      = "#ffffff"
	layoutText      = "#111827"
	layoutBody      = "#334155"
	layoutMuted     = "#667085"
	layoutBorder    = "#dfe5ef"
	layoutRule      = "#eef2f8"
	layoutPanel     = "#f8fafc"
	layoutFontStack = "-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif"
)

const layoutSupportEmail = "support@tauthub.com"

// AppBaseURL: alamat aplikasi untuk tautan di email. Nilainya sama dengan
// config NEXT_PUBLIC_APP_URL.
func AppBaseURL() string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("NEXT_PUBLIC_APP_URL")), "/")
	if base == "" {
		base = "https://app.tauthub.com"
	}
	return base
}

// brandLogoURL sengaja absolut dan tidak ikut AppBaseURL: gambar di email
// diambil klien email dari internet, jadi saat aplikasi dijalankan di
// localhost (dev/staging) logo tetap harus menunjuk aset produksi yang publik.
// Kalau mengikuti AppBaseURL, email dari localhost akan menampilkan gambar
// rusak di inbox.
const brandLogoURL = "https://app.tauthub.com/tauthub-logo.webp"

// BrandLogoURL: wordmark Tauthub (908x149) untuk header email.
func BrandLogoURL() string { return brandLogoURL }

// Escape: escape HTML untuk nilai dinamis (nama, judul, email user).
func Escape(v string) string { return escapeHTML(v) }

// Absolutize: tautan relatif ("/mitra/x") dijadikan absolut karena email tidak
// punya konsep origin. Tautan yang sudah absolut dibiarkan.
func Absolutize(link string) string {
	v := strings.TrimSpace(link)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "mailto:") {
		return v
	}
	if !strings.HasPrefix(v, "/") {
		return v
	}
	return AppBaseURL() + v
}

// Page membungkus fragmen HTML apa pun menjadi dokumen email lengkap: header
// logo, kartu putih, footer kontak.
func Page(title, preheader, bodyHTML string) Rendered {
	if strings.TrimSpace(preheader) == "" {
		preheader = title
	}
	base := AppBaseURL()
	year := time.Now().Format("2006")
	doc := `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<title>` + escapeHTML(title) + `</title>
</head>
<body style="margin:0;padding:0;background:` + layoutBg + `;color:` + layoutText + `;font-family:` + layoutFontStack + `;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">` + escapeHTML(preheader) + `</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:` + layoutBg + `;">
  <tr>
    <td align="center" style="padding:32px 12px;">
      <table role="presentation" width="560" cellpadding="0" cellspacing="0" border="0" style="width:100%;max-width:560px;background:` + layoutCard + `;border:1px solid ` + layoutBorder + `;border-radius:14px;">
        <tr>
          <td style="height:3px;background:` + layoutPrimary + `;line-height:3px;font-size:0;">&nbsp;</td>
        </tr>
        <tr>
          <td style="padding:22px 28px 18px;border-bottom:1px solid ` + layoutRule + `;">
            <a href="` + escapeHTML(base) + `" target="_blank" rel="noopener" style="text-decoration:none;display:inline-block;">
              <img src="` + escapeHTML(BrandLogoURL()) + `" alt="Tauthub" width="140" height="23" style="display:block;width:140px;height:23px;border:0;outline:none;text-decoration:none;">
            </a>
          </td>
        </tr>
        <tr>
          <td style="padding:26px 28px 28px;">
            <h1 style="margin:0 0 14px;font-size:20px;line-height:1.35;font-weight:700;color:` + layoutText + `;">` + escapeHTML(title) + `</h1>
            <div style="font-size:14px;line-height:1.6;color:` + layoutBody + `;">` + bodyHTML + `</div>
          </td>
        </tr>
        <tr>
          <td style="padding:16px 28px;background:#fbfcfe;border-top:1px solid ` + layoutRule + `;">
            <p style="margin:0 0 4px;font-size:12px;line-height:1.6;color:` + layoutMuted + `;">Email otomatis dari <strong style="color:` + layoutText + `;">Tauthub</strong>.</p>
            <p style="margin:0;font-size:12px;line-height:1.6;color:` + layoutMuted + `;">Butuh bantuan? Hubungi <a href="mailto:` + layoutSupportEmail + `" style="color:` + layoutPrimary + `;text-decoration:none;">` + layoutSupportEmail + `</a> &middot; &copy; ` + year + ` Tauthub</p>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>`
	return Rendered{Subject: title, HTML: doc}
}

// DetailBody: paragraf pembuka, tabel rincian opsional, dan tombol aksi.
// Nilai dinamis di-escape di sini; teks pembuka juga, jadi pemanggil tidak
// perlu (dan tidak boleh) menyisipkan HTML mentah lewat jalur ini.
func DetailBody(intro string, rows [][2]string, link, linkLabel string) string {
	var b strings.Builder
	if strings.TrimSpace(intro) != "" {
		b.WriteString(`<p style="margin:0;font-size:14px;line-height:1.6;color:` + layoutBody + `;">` + escapeHTML(intro) + `</p>`)
	}
	if len(rows) > 0 {
		b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:18px 0 0;border-collapse:separate;border-spacing:0;background:` + layoutPanel + `;border:1px solid ` + layoutRule + `;border-radius:10px;">`)
		for i, r := range rows {
			edge := ""
			if i > 0 {
				edge = `border-top:1px solid ` + layoutRule + `;`
			}
			b.WriteString(`<tr>` +
				`<td style="padding:11px 14px;font-size:13px;line-height:1.5;color:` + layoutMuted + `;width:140px;vertical-align:top;` + edge + `">` + escapeHTML(r[0]) + `</td>` +
				`<td style="padding:11px 14px;font-size:14px;line-height:1.5;font-weight:600;color:` + layoutText + `;vertical-align:top;` + edge + `">` + escapeHTML(r[1]) + `</td>` +
				`</tr>`)
		}
		b.WriteString(`</table>`)
	}
	if link != "" {
		href := Absolutize(link)
		label := strings.TrimSpace(linkLabel)
		if label == "" {
			label = "Buka Tauthub"
		}
		b.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" align="center" style="margin:24px auto 0;">` +
			`<tr><td align="center" bgcolor="` + layoutPrimary + `" style="border-radius:9px;">` +
			`<a href="` + escapeHTML(href) + `" target="_blank" rel="noopener" style="display:inline-block;padding:12px 24px;font-size:15px;line-height:1.4;font-weight:600;color:#ffffff;text-decoration:none;border-radius:9px;">` + escapeHTML(label) + `</a>` +
			`</td></tr></table>`)
		b.WriteString(`<p style="margin:14px 0 0;font-size:12px;line-height:1.6;color:` + layoutMuted + `;text-align:center;word-break:break-all;">Tombol tidak berfungsi? Salin tautan ini: <a href="` + escapeHTML(href) + `" target="_blank" rel="noopener" style="color:` + layoutPrimary + `;text-decoration:none;">` + escapeHTML(href) + `</a></p>`)
	}
	return b.String()
}

// Detail: email standar (judul + paragraf + rincian + tombol) dalam kerangka
// brand.
func Detail(title, intro string, rows [][2]string, link, linkLabel string) Rendered {
	return Page(title, intro, DetailBody(intro, rows, link, linkLabel))
}

// DetailHTML: seperti Detail, tetapi badan email sudah berupa HTML aman
// (dipakai pengingat WhatsApp yang perlu <strong>/<ul>). Pemanggil wajib
// meng-escape nilai dinamis dengan Escape.
func DetailHTML(title, bodyHTML, link, linkLabel string) Rendered {
	var anchor string
	if link != "" {
		anchor = `<p style="margin:20px 0 0;"><a href="` + escapeHTML(Absolutize(link)) + `" target="_blank" rel="noopener" style="color:` + layoutPrimary + `;font-weight:600;text-decoration:none;">` + escapeHTML(linkLabel) + `</a></p>`
	}
	return Page(title, title, bodyHTML+anchor)
}
