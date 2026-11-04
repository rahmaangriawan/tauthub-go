package auth

import (
	"regexp"
	"strings"
	"time"
)

// Ringkasan perangkat untuk notifikasi login (email + WA). Tidak perlu akurat
// seperti pustaka UA lengkap: cukup nama browser + versi mayor dan platform
// yang dikenali pengguna awam.

var (
	uaVersionRe = map[string]*regexp.Regexp{
		"Edge":             regexp.MustCompile(`Edg(?:e|A|iOS)?/([\d.]+)`),
		"Opera":            regexp.MustCompile(`OPR/([\d.]+)`),
		"Samsung Internet": regexp.MustCompile(`SamsungBrowser/([\d.]+)`),
		"Firefox":          regexp.MustCompile(`(?:Firefox|FxiOS)/([\d.]+)`),
		"Chrome":           regexp.MustCompile(`(?:Chrome|CriOS)/([\d.]+)`),
		"Safari":           regexp.MustCompile(`Version/([\d.]+)`),
	}
	wibZone    = time.FixedZone("WIB", 7*3600)
	hariID     = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	bulanID    = []string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	uaBrowsers = []string{"Edge", "Opera", "Samsung Internet", "Firefox", "Chrome", "Safari"}
)

// ParseUserAgent → ("Chrome 131", "Windows"). Tidak dikenal → "Browser lain", "Lainnya".
func ParseUserAgent(ua string) (browser, platform string) {
	s := strings.ToLower(ua)
	browser, platform = "Browser lain", "Lainnya"
	for _, name := range uaBrowsers {
		if m := uaVersionRe[name].FindStringSubmatch(ua); m != nil {
			// Safari: "Version/x" juga muncul di UA lain, pastikan ada kata safari.
			if name == "Safari" && !strings.Contains(s, "safari") {
				continue
			}
			major := m[1]
			if i := strings.Index(major, "."); i > 0 {
				major = major[:i]
			}
			browser = name + " " + major
			break
		}
	}
	switch {
	case strings.Contains(s, "windows"):
		platform = "Windows"
	case strings.Contains(s, "android"):
		platform = "Android"
	case strings.Contains(s, "ipad"):
		platform = "iPad"
	case strings.Contains(s, "iphone") || strings.Contains(s, "ipod"):
		platform = "iPhone"
	case strings.Contains(s, "mac os x") || strings.Contains(s, "macintosh"):
		platform = "Macintosh"
	case strings.Contains(s, "cros"):
		platform = "Chrome OS"
	case strings.Contains(s, "linux"):
		platform = "Linux"
	}
	return browser, platform
}

// FormatWIB → "Rabu, 04 Nov 2026 02:21 WIB".
func FormatWIB(t time.Time) string {
	t = t.In(wibZone)
	return hariID[t.Weekday()] + ", " + t.Format("02") + " " + bulanID[t.Month()] + " " + t.Format("2006 15:04") + " WIB"
}
