package brandflow

import "regexp"

// regex.go: pola statis modul-level (aturan RTK: jangan pindahkan ke badan fungsi).

var (
	reMDH1       = regexp.MustCompile(`(?m)^#\s+.+$`)
	reHTMLH1     = regexp.MustCompile(`(?is)^<h1[^>]*>[\s\S]*?</h1>\s*`)
	reHTMLTag    = regexp.MustCompile(`<[^>]*>`)
	reWhitespace = regexp.MustCompile(`\s+`)

	// reIFrame / reIFrameSrc: dipakai stripDisallowedIframes (port lib/media/embed.ts).
	reIFramePair = regexp.MustCompile(`(?is)<iframe\b[^>]*>[\s\S]*?</iframe>`)
	reIFrameAny  = regexp.MustCompile(`(?is)<iframe\b[^>]*/?>`)
	reIFrameSrc  = regexp.MustCompile(`(?i)src\s*=\s*(?:"([^"]*)"|'([^']*)')`)

	// reImageInText: deteksi gambar di body (port lib/standard/service.ts).
	reImageInText = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)|https?://\S+\.(png|jpe?g|webp|gif)`)

	// reJSONBlock: objek JSON pertama dalam teks (port extractJsonBlock).
	reJSONBlock = regexp.MustCompile(`\{[\s\S]*\}`)

	// reScoreText: fallback "skor ... NNN" untuk parseAnalysis.
	reScoreText = regexp.MustCompile(`(?i)skor.*?(\d{1,3})`)

	// reDateOnly / reHHMM: validasi jadwal berulang (lib/publication/plans.ts).
	reDateOnly = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reHHMM     = regexp.MustCompile(`^\d{2}:\d{2}$`)

	// reAnchorOpenAll / reRelAttr: laporan link artikel (gatekeeper audits).
	reAnchorOpenAll = regexp.MustCompile(`(?i)<a\b[^>]*>`)
	reRelAttr       = regexp.MustCompile(`(?i)\brel\s*=\s*["']([^"']*)["']`)
	reH1234Open     = regexp.MustCompile(`(?i)<h[1-4]\b[^>]*>`)
	reH1234Close    = regexp.MustCompile(`(?i)</h[1-4]\b[^>]*>`)
	reTagSplit      = regexp.MustCompile(`(<[^>]*>)`)
	reOpenA         = regexp.MustCompile(`(?is)^<a[\s>]`)
	reCloseA        = regexp.MustCompile(`(?is)^</a`)
	reHTTPScheme    = regexp.MustCompile(`(?i)^https?://`)
)

// slugifyPlain: parity slugify ala app lama (lowercase, non-alfanumerik → "-").
var reNonSlug = regexp.MustCompile(`[^a-z0-9]+`)
var reEdgeDash = regexp.MustCompile(`(^-|-$)`)

// reAnchorPair: pasangan <a>...</a> untuk melepas hyperlink.
var reAnchorPair = regexp.MustCompile(`(?is)<a\b[^>]*>.*?</a>`)

// mustRegexp: helper kompilasi regex modul-level (pola statis — init-time).
func mustRegexp(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }

// regexpQuoteMeta: shim regexp.QuoteMeta.
func regexpQuoteMeta(s string) string { return regexp.QuoteMeta(s) }

func slugifyPlain(value string) string {
	s := reNonSlug.ReplaceAllString(toLowerASCII(value), "-")
	s = reEdgeDash.ReplaceAllString(s, "")
	return s
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
