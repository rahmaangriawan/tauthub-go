package auth

import (
	"net/url"
	"strings"
)

// Port lib/auth/deep-link.ts safeReturnTo — terima hanya path relatif atau URL
// absolut same-origin; tolak login target & pola protokol-relative.

const fallbackOrigin = "https://return-to.invalid"

func normalizedPathname(pathname string) string {
	decoded := pathname
	for i := 0; i < 2; i++ {
		next, err := url.PathUnescape(decoded)
		if err != nil || next == decoded {
			break
		}
		decoded = next
	}
	return strings.ReplaceAll(decoded, "\\", "/")
}

func isLoginTarget(pathname string) bool {
	normalized := strings.TrimRight(normalizedPathname(pathname), "/")
	if normalized == "" {
		normalized = "/"
	}
	return normalized == "/login" || strings.HasPrefix(normalized, "/login/")
}

// SafeReturnTo mengembalikan path same-origin (termasuk query & hash) atau "".
func SafeReturnTo(value string, origin string) string {
	if value == "" || len(value) > 4096 || strings.ContainsAny(value, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
		return ""
	}
	base := origin
	if base == "" {
		base = fallbackOrigin
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	target, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if target.Host != "" && target.Scheme != "" && target.Host != baseURL.Host {
		return ""
	}
	if target.Scheme != "" && target.Scheme != "http" && target.Scheme != "https" {
		return ""
	}
	absoluteSameOrigin := (strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")) && target.Host == baseURL.Host
	if !strings.HasPrefix(value, "/") && !absoluteSameOrigin {
		return ""
	}
	if strings.HasPrefix(value, "//") || strings.HasPrefix(normalizedPathname(target.Path), "//") {
		return ""
	}
	if isLoginTarget(target.Path) {
		return ""
	}
	out := target.Path
	if target.RawQuery != "" {
		out += "?" + target.RawQuery
	}
	if target.Fragment != "" {
		out += "#" + target.Fragment
	}
	return out
}
