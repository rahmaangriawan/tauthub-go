package authdomain

import (
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
)

// Helper kecil bersama untuk handler auth.

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func authRequestIp(r *http.Request) string {
	return auth.RequestIp(r.Header)
}

func truncate24(s string) string {
	if len(s) < 24 {
		return s
	}
	return s[:24]
}

func isSixDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}
