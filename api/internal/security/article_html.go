package security

import (
	"regexp"
	"strconv"
	"strings"
)

// SanitizeArticleHTML: netralkan konstruk berbahaya pada HTML artikel yang masuk
// lewat jalur tulis manusia (edit/revisi klien, edit/unggah manual Taut Pintar).
//
// Prinsipnya: artikel normal HARUS tidak berubah sama sekali. Karena itu ada
// jalur cepat: kalau tidak ada satu pun penanda berbahaya, string dikembalikan
// apa adanya (byte-for-byte). Pipeline penggantian hanya jalan saat ada penanda.
//
// Yang dinetralkan: <script>, <iframe> dari host non-embed-tepercaya,
// <object>/<embed>/<form>/<input>/<button>/<meta>/<base>, atribut event (on*),
// srcdoc, dan URL ber-skema javascript:/vbscript:/data: (data: gambar diizinkan).
//
// Nilai balikan kedua = true bila isi berubah (dipakai untuk audit log).
func SanitizeArticleHTML(html string) (string, bool) {
	if html == "" || !articleDangerRe.MatchString(html) {
		return html, false
	}
	out := html
	// 1) Blok <script>...</script> dan sisa tag <script ...> tanpa penutup.
	out = articleScriptBlockRe.ReplaceAllString(out, "")
	out = articleTagOnlyRe.ReplaceAllString(out, "")
	// 2) <iframe>: hanya boleh dari host embed tepercaya.
	out = articleIframeRe.ReplaceAllStringFunc(out, func(match string) string {
		if articleEmbedSrcRe.MatchString(match) {
			return match
		}
		return ""
	})
	// 3) Blok <form>...</form> utuh, lalu tag pembungkus/berbahaya lain
	// (buka maupun tutup).
	out = articleFormBlockRe.ReplaceAllString(out, "")
	out = articleForbiddenTagRe.ReplaceAllString(out, "")
	// 4) Atribut event (onclick=, onerror=, onload=, …) dan srcdoc.
	out = articleEventAttrRe.ReplaceAllString(out, "")
	out = articleSrcdocRe.ReplaceAllString(out, "")
	// 5) Skema URL berbahaya pada atribut yang bisa memuat/mengarahkan.
	out = articleURLAttrRe.ReplaceAllStringFunc(out, articleNeutralizeURLAttr)
	if out != html {
		return out, true
	}
	return html, false
}

var (
	// Penanda cepat: hanya jalan ke pipeline bila salah satu muncul. Untuk atribut
	// URL, pipeline dipicu begitu nilainya mengandung "javascript"/"vbscript"/
	// "data:" atau entitas (`&#...;`) — trik "jav&#x61;script:" dan
	// "java<newline>script:" ikut terpicu.
	articleDangerRe      = regexp.MustCompile(`(?i)</?(script|iframe|object|embed|form|input|button|meta|base)\b|\son[a-z]+\s*=|\ssrcdoc\s*=|\s(href|src|xlink:href|action|formaction|poster|background)\s*=\s*["']?[^"'>]*(java\s*script|vbscript|data:|&#)`)
	articleScriptBlockRe = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`)
	articleTagOnlyRe     = regexp.MustCompile(`(?is)</?script\b[^>]*>`)
	articleIframeRe      = regexp.MustCompile(`(?is)<iframe\b[^>]*>.*?</iframe\s*>|<iframe\b[^>]*/?>`)
	// Form tidak pernah konten artikel yang sah: buang blok utuh (termasuk isinya)
	// supaya tidak menyisakan teks liar seperti label tombol.
	articleFormBlockRe     = regexp.MustCompile(`(?is)<form\b[^>]*>.*?</form\s*>`)
	articleEmbedSrcRe      = regexp.MustCompile(`(?i)\ssrc\s*=\s*("https://(www\.)?(youtube\.com|youtube-nocookie\.com|google\.com|maps\.google\.com)/[^"]*"|'https://(www\.)?(youtube\.com|youtube-nocookie\.com|google\.com|maps\.google\.com)/[^']*'|https://(www\.)?(youtube\.com|youtube-nocookie\.com|google\.com|maps\.google\.com)/[^\s>]+|"https://player\.vimeo\.com/[^"]*"|'https://player\.vimeo\.com/[^']*'|https://player\.vimeo\.com/[^\s>]+)`)
	articleForbiddenTagRe  = regexp.MustCompile(`(?i)</?(object|embed|form|input|button|meta|base|applet|frame|frameset|link|template)\b[^>]*>`)
	articleEventAttrRe     = regexp.MustCompile(`(?i)\son[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	articleSrcdocRe        = regexp.MustCompile(`(?i)\ssrcdoc\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	articleURLAttrRe       = regexp.MustCompile(`(?i)\s(href|src|xlink:href|action|formaction|poster|background)\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	articleAttrNameRe      = regexp.MustCompile(`(?i)^\s*([a-z:]+)\s*=`)
	articleNumericEntityRe = regexp.MustCompile(`&#x?[0-9a-fA-F]+;`)
)

// articleNeutralizeURLAttr: sisakan atribut aman, ganti nilai berbahaya dengan "#".
func articleNeutralizeURLAttr(match string) string {
	nameMatch := articleAttrNameRe.FindStringSubmatch(match)
	if nameMatch == nil {
		return ""
	}
	name := strings.TrimSpace(nameMatch[1])
	eq := strings.Index(match, "=")
	if eq < 0 {
		return ""
	}
	rawValue := strings.TrimSpace(match[eq+1:])
	value := strings.Trim(rawValue, `"'`)
	if !articleURLDangerous(value) {
		return match
	}
	return " " + name + `="#"`
}

// articleURLDangerous: true untuk javascript:/vbscript:/data: non-gambar.
// Entitas numerik/nama dan karakter kontrol dilepas lebih dulu, supaya trik
// seperti "jav&#x61;script:", "java<newline>script:", atau "java&colon;script:"
// tetap tertangkap.
func articleURLDangerous(value string) bool {
	decoded := articleDecodeEntities(value)
	var b strings.Builder
	for _, r := range decoded {
		// Browser mengabaikan ASCII whitespace/kontrol di dalam skema URL.
		if r <= ' ' || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	lower := strings.ToLower(b.String())
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "vbscript:") {
		return true
	}
	if strings.HasPrefix(lower, "data:") && !strings.HasPrefix(lower, "data:image/") {
		return true
	}
	return false
}

// articleDecodeEntities: dekode entitas numerik (desimal/hex) dan segelintir
// entitas bernama yang lazim dipakai untuk menyusun ulang "javascript:".
func articleDecodeEntities(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	return articleNumericEntityRe.ReplaceAllStringFunc(s, func(entity string) string {
		// Bentuk entity: "&#123;", "&#x1F;" atau "&#X1f;".
		body := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(entity, "&"), "#"), ";")
		base := 10
		if len(body) > 1 && (body[0] == 'x' || body[0] == 'X') {
			base = 16
			body = body[1:]
		}
		v, err := strconv.ParseInt(body, base, 32)
		if err != nil || v <= 0 || v > 0x10FFFF {
			return ""
		}
		return string(rune(v))
	})
}
