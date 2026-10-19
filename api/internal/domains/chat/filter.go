package chat

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Filter anti-bypass untuk pesan obrolan: mendeteksi kontak pribadi dan ajakan
// transaksi di luar platform, lalu menyensor HANYA bagian yang terdeteksi.
//
// Tantangan utamanya obfuscation: nomor ditulis "0812,3456,7890", kata ditulis
// "w4ts4pp" atau "w h a t s a p p", karakter zero-width disisipkan, dan email
// ditulis "nama [at] domain [dot] com". Karena itu deteksi memakai beberapa
// "tampilan" ternormalisasi dengan peta balik ke posisi asli:
//
//   - view rune ternormalisasi (lowercase, leet/homoglyph, zero-width dibuang)
//     untuk keyword pendek & email obfuscated;
//   - view alfanumerik rapat (semua pemisah dibuang) untuk keyword panjang
//     sehingga huruf yang diberi spasi tetap tertangkap;
//   - deret digit (semua non-digit di antara digit diabaikan) untuk telepon.
//
// URL biasa (draft artikel/backlink) DILINDUNGI dan tidak ikut disensor; hanya
// host messenger (wa.me, *.whatsapp.com, t.me) yang dianggap kontak.

// MaskedContact menggantikan setiap bagian yang terdeteksi.
const MaskedContact = "[KONTAK DISEMBUNYIKAN OLEH SISTEM]"

type textSpan struct{ start, end int }

var (
	// URL dengan skema (http/https).
	reSchemeURL = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"']+`)
	// Domain tanpa skema, mis. namadomain.com/artikel-1.
	reBareDomain = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+(?:com|id|net|org|co|io|ly|me|app|dev|site|online|xyz|info|biz|news|blog|tech|store|pro|asia|tv|cc|sg|my|uk|us|gov|edu|ac|sch|or|web)\b(?:/[^\s<>"']*)?`)
	// Email standar.
	reEmail = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9\-]+(?:\.[a-z0-9\-]+)+`)
	// Email terobfuscasi: "nama [at] domain [dot] com", "nama(at)domain(dot)com", dst.
	reObfuscatedEmail = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+\s*(?:\[at\]|\(at\)|\bat\b)\s*[a-z0-9\-]+(?:\s*\[dot\]|\s*\(dot\)|\s*\bdot\b|\s*\btitik\b)\s*[a-z]{2,}`)
	// Keyword pendek: whole-word, hanya pada view ternormalisasi (bukan rapat)
	// supaya "waktu"/"kawan" tidak ikut.
	reShortKeyword = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(wa|tele)(?:[^a-z0-9]|$)`)
)

// Keyword panjang dicocokkan pada view rapat (tanpa spasi/pemisah).
var longKeywords = []string{
	"whatsapp",
	// Varian ejaan/singkatan yang lazim (huruf 'h' sering hilang saat di-leet).
	"whatsap",
	"watsapp",
	"watsap",
	"wasap",
	"telegram",
	"rekening",
	"norek",
	"transferlangsung",
	"tflangsung",
	"potongkomisi",
	"bypass",
}

// MaskChatMessage mengembalikan versi tersensor dan status pelanggaran.
// Panjang pesan tidak diubah validasinya di sini (dilakukan handler).
func MaskChatMessage(text string) (string, bool) {
	if strings.TrimSpace(text) == "" {
		return text, false
	}

	hits := make([]textSpan, 0, 4)

	// 1) URL: kumpulkan span yang dilindungi; host messenger jadi hit.
	protected := make([]textSpan, 0, 2)
	addURL := func(raw string, start int) {
		raw = strings.TrimRight(raw, ".,;:!?)]}\"'")
		if raw == "" {
			return
		}
		end := start + len(raw)
		host := urlHost(raw)
		if isMessengerHost(host) {
			hits = append(hits, textSpan{start, end})
			return
		}
		protected = append(protected, textSpan{start, end})
	}
	for _, m := range reSchemeURL.FindAllStringIndex(text, -1) {
		addURL(text[m[0]:m[1]], m[0])
	}
	for _, m := range reBareDomain.FindAllStringIndex(text, -1) {
		// Domain yang merupakan bagian dari email (didahului '@') bukan URL;
		// penanganannya di deteksi email.
		if m[0] > 0 && text[m[0]-1] == '@' {
			continue
		}
		addURL(text[m[0]:m[1]], m[0])
	}

	// 2) Email standar + terobfuscasi.
	for _, m := range reEmail.FindAllStringIndex(text, -1) {
		hits = append(hits, textSpan{m[0], m[1]})
	}
	for _, m := range reObfuscatedEmail.FindAllStringIndex(text, -1) {
		hits = append(hits, textSpan{m[0], m[1]})
	}

	// 3) Telepon: deret digit dengan pemisah apa pun (koma, strip, garis miring,
	//    underscore, newline, zero-width, dst).
	hits = append(hits, findPhoneSpans(text)...)

	// 4) Keyword: bangun view ternormalisasi (leet + zero-width) sekali.
	view := newNormView(text)
	hits = append(hits, findLongKeywordSpans(text, view)...)
	hits = append(hits, findShortKeywordSpans(view)...)

	// 5) Buang hit yang berada di dalam URL terlindungi, gabung overlap.
	filtered := hits[:0]
	for _, h := range hits {
		if h.start < 0 || h.end <= h.start || h.end > len(text) {
			continue
		}
		if overlapsAny(h, protected) {
			continue
		}
		filtered = append(filtered, h)
	}
	if len(filtered) == 0 {
		return text, false
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].start < filtered[j].start })
	merged := make([]textSpan, 0, len(filtered))
	for _, h := range filtered {
		if len(merged) > 0 && h.start <= merged[len(merged)-1].end {
			if h.end > merged[len(merged)-1].end {
				merged[len(merged)-1].end = h.end
			}
			continue
		}
		merged = append(merged, h)
	}

	var b strings.Builder
	last := 0
	for _, h := range merged {
		b.WriteString(text[last:h.start])
		b.WriteString(MaskedContact)
		last = h.end
	}
	b.WriteString(text[last:])
	return b.String(), true
}

// ---------- URL ----------

func urlHost(raw string) string {
	s := raw
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "www.")
	return s
}

func isMessengerHost(host string) bool {
	switch host {
	case "wa.me", "t.me", "telegram.me", "chat.whatsapp.com", "api.whatsapp.com", "whatsapp.com":
		return true
	}
	return strings.HasSuffix(host, ".whatsapp.com") || strings.HasSuffix(host, ".t.me")
}

func overlapsAny(h textSpan, spans []textSpan) bool {
	for _, s := range spans {
		if h.start < s.end && s.start < h.end {
			return true
		}
	}
	return false
}

// ---------- normalisasi ----------

func isFormatRune(r rune) bool {
	switch {
	case r >= 0x200B && r <= 0x200F,
		r == 0x2060,
		r == 0xFEFF,
		r == 0x00AD,
		r >= 0x2061 && r <= 0x2064:
		return true
	}
	return false
}

func mapLeet(r rune) rune {
	switch r {
	case '0':
		return 'o'
	case '1':
		return 'i'
	case '3':
		return 'e'
	case '4':
		return 'a'
	case '5':
		return 's'
	case '7':
		return 't'
	case '9':
		return 'g'
	case '@':
		return 'a'
	case '$':
		return 's'
	}
	return r
}

type normView struct {
	runes     []rune
	origStart []int // byte start di teks asli
	origEnd   []int // byte end di teks asli
	byteAt    []int // byte start rune ini di string(runes)
	atByte    map[int]int
}

func newNormView(text string) normView {
	v := normView{atByte: map[int]int{}}
	bytePos := 0
	for i, r := range text {
		size := utf8.RuneLen(r)
		if isFormatRune(r) {
			continue
		}
		lr := unicode.ToLower(r)
		if lr == '\u00a0' {
			lr = ' '
		}
		lr = mapLeet(lr)
		v.atByte[bytePos] = len(v.runes)
		v.runes = append(v.runes, lr)
		v.origStart = append(v.origStart, i)
		v.origEnd = append(v.origEnd, i+size)
		v.byteAt = append(v.byteAt, bytePos)
		bytePos += utf8.RuneLen(lr)
	}
	v.atByte[bytePos] = len(v.runes)
	return v
}

// findLongKeywordSpans: cocokkan pada view alfanumerik rapat, lalu petakan
// balik ke span asli. Batas panjang span (3x keyword) mencegah pencocokan
// melintasi kalimat panjang.
func findLongKeywordSpans(text string, v normView) []textSpan {
	var compact []rune
	var compStart, compEnd []int
	for i, r := range v.runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			compact = append(compact, r)
			compStart = append(compStart, v.origStart[i])
			compEnd = append(compEnd, v.origEnd[i])
		}
	}
	if len(compact) == 0 {
		return nil
	}
	compactStr := string(compact)
	var out []textSpan
	for _, kw := range longKeywords {
		kwLen := utf8.RuneCountInString(kw)
		searchFrom := 0
		for searchFrom+len(kw) <= len(compactStr) {
			idx := strings.Index(compactStr[searchFrom:], kw)
			if idx < 0 {
				break
			}
			absByte := searchFrom + idx
			startRune := utf8.RuneCountInString(compactStr[:absByte])
			endRune := startRune + kwLen
			if endRune <= len(compact) {
				start, end := compStart[startRune], compEnd[endRune-1]
				// Batas 3x panjang kata: cukup longgar untuk huruf berspasi,
				// tapi tidak sampai menelan kalimat panjang.
				if runeSpanLen(text, start, end) <= kwLen*3 {
					out = append(out, textSpan{start, end})
				}
			}
			searchFrom = absByte + len(kw)
		}
	}
	return out
}

func runeSpanLen(s string, start, end int) int {
	return utf8.RuneCountInString(s[start:end])
}

// findShortKeywordSpans: keyword pendek whole-word pada view ternormalisasi.
func findShortKeywordSpans(v normView) []textSpan {
	normStr := string(v.runes)
	var out []textSpan
	for _, m := range reShortKeyword.FindAllStringSubmatchIndex(normStr, -1) {
		// grup 1 = kata (wa/tele)
		bs, be := m[2], m[3]
		startRune, ok1 := v.atByte[bs]
		endRune, ok2 := v.atByte[be]
		if !ok1 || !ok2 || startRune >= endRune {
			continue
		}
		out = append(out, textSpan{v.origStart[startRune], v.origEnd[endRune-1]})
	}
	return out
}

// ---------- telepon ----------

type digitPos struct {
	start, end int
	ch         byte
}

// findPhoneSpans mencari nomor Indonesia yang digitnya dipisah karakter apa pun
// selain huruf/digit (spasi, strip, titik, koma, underscore, newline, dst).
func findPhoneSpans(text string) []textSpan {
	digits := make([]digitPos, 0, 16)
	for i, r := range text {
		if r >= '0' && r <= '9' {
			digits = append(digits, digitPos{start: i, end: i + 1, ch: byte(r)})
		}
	}
	var out []textSpan
	for i := 0; i < len(digits); i++ {
		if !phonePrefixAt(digits, i) {
			continue
		}
		// Mulai kumpulkan digit selama pemisah di antaranya non-huruf.
		last := i
		count := 1
		j := i
		for {
			if j+1 >= len(digits) {
				break
			}
			gap := text[digits[j].end:digits[j+1].start]
			if hasLetter(gap) {
				break
			}
			j++
			count++
			last = j
			if count > 15 {
				break
			}
		}
		if count < 10 || count > 14 {
			continue
		}
		start := digits[i].start
		// Ikutkan tanda '+' tepat sebelum nomor (mis. +62 812…).
		if start > 0 {
			if prev, size := utf8.DecodeLastRuneInString(text[:start]); prev == '+' {
				start -= size
			}
		}
		out = append(out, textSpan{start, digits[last].end})
	}
	return out
}

func phonePrefixAt(digits []digitPos, i int) bool {
	// 08…
	if digits[i].ch == '0' {
		return i+1 < len(digits) && digits[i+1].ch == '8'
	}
	// 62 8…
	if digits[i].ch == '6' {
		return i+2 < len(digits) && digits[i+1].ch == '2' && digits[i+2].ch == '8'
	}
	return false
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
