package email

// Gaya email yang bisa diatur admin dari /admin/konfigurasi (tab Email/SMTP).
//
// Sebelumnya palet dan logo terkunci sebagai konstanta di layout.go. Sekarang
// nilai disimpan di tabel email_branding dan dibaca lewat LoadStyle dengan
// cache 60 detik; setiap proses (api, worker, scheduler) memakai cache sendiri
// sehingga perubahan gaya berlaku paling lambat satu menit kemudian.

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Style adalah tampilan kerangka email.
type Style struct {
	BrandName    string
	LogoURL      string
	PrimaryColor string
	AccentColor  string
	PageBG       string
	CardBG       string
	TextColor    string
	MutedColor   string
	FontStack    string
	ButtonRadius int
	FooterText   string
	SupportEmail string
}

// DefaultStyle: nilai yang sama dengan tampilan email sebelum panel gaya ada.
func DefaultStyle() Style {
	return Style{
		BrandName:    "Tauthub",
		LogoURL:      brandLogoURL,
		PrimaryColor: layoutPrimary,
		AccentColor:  layoutPrimary,
		PageBG:       layoutBg,
		CardBG:       layoutCard,
		TextColor:    layoutText,
		MutedColor:   layoutMuted,
		FontStack:    layoutFontStack,
		ButtonRadius: 9,
		FooterText:   "Email otomatis dari Tauthub.",
		SupportEmail: layoutSupportEmail,
	}
}

func (s Style) brand() string {
	if strings.TrimSpace(s.BrandName) != "" {
		return s.BrandName
	}
	return "Tauthub"
}
func (s Style) logo() string {
	if strings.TrimSpace(s.LogoURL) != "" {
		return s.LogoURL
	}
	return brandLogoURL
}
func (s Style) primary() string { return colorOr(s.PrimaryColor, layoutPrimary) }
func (s Style) accent() string  { return colorOr(s.AccentColor, s.primary()) }
func (s Style) pageBG() string  { return colorOr(s.PageBG, layoutBg) }
func (s Style) cardBG() string  { return colorOr(s.CardBG, layoutCard) }
func (s Style) text() string    { return colorOr(s.TextColor, layoutText) }
func (s Style) muted() string   { return colorOr(s.MutedColor, layoutMuted) }
func (s Style) font() string    { return cssValueOr(s.FontStack, layoutFontStack) }
func (s Style) support() string {
	if strings.TrimSpace(s.SupportEmail) != "" {
		return strings.TrimSpace(s.SupportEmail)
	}
	return layoutSupportEmail
}
func (s Style) footer() string {
	if strings.TrimSpace(s.FooterText) != "" {
		return s.FooterText
	}
	return "Email otomatis dari " + s.brand() + "."
}
func (s Style) radius() int {
	if s.ButtonRadius < 0 {
		return 0
	}
	if s.ButtonRadius > 24 {
		return 24
	}
	if s.ButtonRadius == 0 {
		return 9
	}
	return s.ButtonRadius
}

// colorOr: nilai warna CSS sederhana atau fallback. Nilai dari DB sudah
// divalidasi saat disimpan; ini lapis kedua agar markup tidak rusak.
func colorOr(value, fallback string) string {
	v := strings.TrimSpace(value)
	if v == "" || strings.ContainsAny(v, `"'<>`+"\n\r") {
		return fallback
	}
	return v
}

// cssValueOr: font stack bebas tetapi tanpa karakter yang bisa memutus atribut.
func cssValueOr(value, fallback string) string {
	v := strings.TrimSpace(value)
	if v == "" || strings.ContainsAny(v, `"<>`+"\n\r") {
		return fallback
	}
	return v
}

var (
	styleMu       sync.Mutex
	styleCached   Style
	styleCachedAt time.Time
	styleLoaded   bool
)

const styleTTL = 60 * time.Second

// LoadStyle membaca gaya dari database dengan cache 60 detik. Kegagalan DB
// tidak menggagalkan pengiriman: gaya terakhir (atau default) tetap dipakai.
func LoadStyle(ctx context.Context, pool *pgxpool.Pool) Style {
	styleMu.Lock()
	if styleLoaded && time.Since(styleCachedAt) < styleTTL {
		cached := styleCached
		styleMu.Unlock()
		return cached
	}
	styleMu.Unlock()

	style := DefaultStyle()
	if pool != nil {
		row := pool.QueryRow(ctx, `
			SELECT brand_name, logo_url, primary_color, accent_color, page_bg, card_bg,
			       text_color, muted_color, font_stack, button_radius, footer_text, support_email
			FROM email_branding WHERE id = 'default' LIMIT 1`)
		var s Style
		if err := row.Scan(&s.BrandName, &s.LogoURL, &s.PrimaryColor, &s.AccentColor, &s.PageBG,
			&s.CardBG, &s.TextColor, &s.MutedColor, &s.FontStack, &s.ButtonRadius, &s.FooterText, &s.SupportEmail); err == nil {
			style = s
		}
	}

	styleMu.Lock()
	styleCached = style
	styleCachedAt = time.Now()
	styleLoaded = true
	styleMu.Unlock()
	return style
}

// InvalidateStyleCache dipanggil setelah admin menyimpan gaya agar proses ini
// langsung memakai nilai baru (proses lain menyusul dalam 60 detik).
func InvalidateStyleCache() {
	styleMu.Lock()
	styleLoaded = false
	styleMu.Unlock()
}
