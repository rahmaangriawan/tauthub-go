package clienttaut

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// Regex tingkat paket (dibuat sekali — bukan di dalam handler/pure function).

var (
	// reFence: pagar kode markdown di sekitar JSON (parity /```(?:json)?/gi).
	reFence = regexp.MustCompile("(?i)```(?:json)?")
	// reFenceLoose: varian toleran untuk parser ranked (parity /```[a-zA-Z]*/g).
	reFenceLoose = regexp.MustCompile("(?i)```[a-zA-Z]*")
)

// wordBoundaryRe: pola `\bslug\b` (parity \b + escaped slug) untuk humanizeReason.
func wordBoundaryRe(slug string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(slug)
	return regexp.MustCompile(`(^|[^A-Za-z0-9_])` + escaped + `([^A-Za-z0-9_]|$)`)
}

// randomInt: parity Math.floor(Math.random() * n) untuk shuffle wildcard pool.
func randomInt(n int) int {
	if n <= 0 {
		return 0
	}
	return rand.Intn(n)
}

// trimFloat: parity JS String(number) — tanpa nol ekor.
func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// slugify: parity value.toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/(^-|-$)/g,"").
func slugify(value string) string {
	lower := strings.ToLower(value)
	var b strings.Builder
	lastDash := false
	for _, ch := range lower {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// slugOr: slug dengan fallback bila kosong (parity `|| "taut-artikel"`).
func slugOr(value, fallback string) string {
	s := slugify(value)
	if s == "" {
		s = fallback
	}
	return truncateString(s, 200)
}
