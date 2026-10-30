package brandflow

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// taut_backlink.go: port lib/taut/agents.ts bagian post-processing artikel
// (demote/normalize headings + enforcement backlink deterministik).

type tautBacklinkSpec struct {
	URL       string
	Anchor    string
	Rel       *string
	Mandatory bool
	// Client: tautan yang diminta klien lewat modal Pengelola Naskah. Beda dari
	// tautan kampanye, klien mengisinya dengan sadar dan mengharapkan tautan itu
	// ADA di artikel. Karena itu penegaknya wajib menanam sendiri saat model
	// tidak menuliskannya — bukan sekadar menunggu URL muncul di teks.
	Client bool
}

// reURLWithScheme: URL ber-skema di dalam teks.
var reURLWithScheme = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)

// stripURLSchemeText: lepas skema http(s):// dari URL pada teks biasa —
// alamat tetap terbaca sebagai teks polos, tanpa rasa tautan.
func stripURLSchemeText(text string) string {
	return reURLWithScheme.ReplaceAllStringFunc(text, func(m string) string {
		return strings.TrimPrefix(strings.TrimPrefix(m, "http://"), "https://")
	})
}

// stripURLSchemeHTML: sama seperti stripURLSchemeText, tetapi hanya menyentuh
// teks di dalam node teks HTML — atribut (href dsb.) tidak disentuh.
func stripURLSchemeHTML(html string) string {
	return mapHTMLTextNodes(html, stripURLSchemeText)
}

// finalizeTautArticleHTML: pasca-proses struktur + kebijakan link.
func finalizeTautArticleHTML(html, mediaType string, allowsLink bool, links []tautBacklinkSpec) string {
	out := html
	if mediaType == "blogger" {
		out = normalizeHeadingBlocks(out)
	} else {
		out = demoteHeadingBlocks(out)
	}
	if allowsLink {
		out = enforceTautBacklinkPolicy(out, links)
	} else {
		out = stripAllLinks(out)
	}
	return tidyTautHTML(out, mediaType)
}

// tidyTautHTML: rapikan markup tersimpan — buang <hr>/garis dekoratif, tag
// kosong, wrapper <div>, atribut inline presentasional, dan paragraf yang
// hanya berisi URL telanjang; untuk blogger pastikan hierarki heading H2→H3.
func tidyTautHTML(html, mediaType string) string {
	if strings.TrimSpace(html) == "" {
		return html
	}
	out := reTidyHR.ReplaceAllString(html, "")
	out = reTidyDecorativeP.ReplaceAllString(out, "")
	// Tag kosong bisa bersarang (mis. <div><p></p></div>), jadi ulang sampai stabil.
	for i := 0; i < 4; i++ {
		next := reTidyEmptyBlock.ReplaceAllStringFunc(out, func(m string) string {
			sub := reTidyEmptyBlock.FindStringSubmatch(m)
			if sub == nil || !strings.EqualFold(sub[1], sub[2]) {
				return m
			}
			switch strings.ToLower(sub[1]) {
			case "p", "h1", "h2", "h3", "h4", "h5", "h6", "div", "section", "li", "ul", "ol", "blockquote":
				return ""
			}
			return m
		})
		if next == out {
			break
		}
		out = next
	}
	out = reTidyDivOpen.ReplaceAllString(out, "")
	out = reTidyDivClose.ReplaceAllString(out, "")
	out = reTidyInlineAttr.ReplaceAllString(out, "")
	out = stripBareURLParagraphs(out)
	if mediaType == "blogger" {
		out = normalizeBloggerHeadingHierarchy(out)
	}
	out = reTidyBlankLines.ReplaceAllString(out, "\n\n")
	return strings.TrimSpace(out)
}

// stripBareURLParagraphs: buang paragraf yang isinya hanya URL telanjang.
func stripBareURLParagraphs(html string) string {
	return reTidyBareURLP.ReplaceAllString(html, "")
}

// normalizeBloggerHeadingHierarchy: untuk blogger, jadikan heading mulai H2 dan
// tidak melompat level (H2 lalu H3, dst).
func normalizeBloggerHeadingHierarchy(html string) string {
	matches := reAnyHeadingTag.FindAllStringSubmatchIndex(html, -1)
	if len(matches) == 0 {
		return html
	}
	var b strings.Builder
	prev := 1
	open := 0
	lastEnd := 0
	for _, m := range matches {
		b.WriteString(html[lastEnd:m[0]])
		lastEnd = m[1]
		closing := m[2] != -1 && html[m[2]:m[3]] == "/"
		lvl := int(html[m[4]] - '0')
		if closing {
			if open == 0 {
				open = 2
			}
			b.WriteString("</h" + strconv.Itoa(open) + ">")
			open = 0
			continue
		}
		if lvl < 2 {
			lvl = 2
		}
		if lvl > prev+1 {
			lvl = prev + 1
		}
		prev = lvl
		open = lvl
		b.WriteString("<h" + strconv.Itoa(lvl) + ">")
	}
	b.WriteString(html[lastEnd:])
	return b.String()
}

// demoteHeadingBlocks: turunkan h1-h4 jadi paragraf (media mainstream).
func demoteHeadingBlocks(html string) string {
	if html == "" || !strings.Contains(html, "<h") {
		return html
	}
	out := reH1234Open.ReplaceAllString(html, "<p>")
	return reH1234Close.ReplaceAllString(out, "</p>")
}

// demoteH1ToH2: H1 dilarang di isi artikel (judul hidup di field judul
// terpisah); H1 yang lolos dari model/manual diturunkan deterministik jadi H2.
var (
	reH1OpenTag  = regexp.MustCompile(`(?i)<h1\b[^>]*>`)
	reH1CloseTag = regexp.MustCompile(`(?i)</h1\s*>`)
)

func demoteH1ToH2(html string) string {
	if html == "" || !strings.Contains(strings.ToLower(html), "<h1") {
		return html
	}
	out := reH1OpenTag.ReplaceAllString(html, "<h2>")
	return reH1CloseTag.ReplaceAllString(out, "</h2>")
}

// normalizeHeadingBlocks: naikkan blok judul sederhana jadi <h2> untuk blogger
// (port normalizeHeadings, versi ringkas deterministik: paragraf pendek tanpa
// titik akhir yang diapit baris kosong → heading).
func normalizeHeadingBlocks(html string) string {
	if html == "" || !strings.Contains(html, "<") {
		return html
	}
	blocks := reBlockP.FindAllStringSubmatch(html, -1)
	if len(blocks) == 0 {
		return html
	}
	out := html
	count := 0
	for _, m := range blocks {
		if count >= 40 {
			break
		}
		inner := strings.TrimSpace(reHTMLTag.ReplaceAllString(m[1], ""))
		if inner == "" || strings.Contains(inner, ".") {
			continue
		}
		if utf8.RuneCountInString(inner) > 90 || len(strings.Fields(inner)) > 12 {
			continue
		}
		out = strings.Replace(out, m[0], "<h2>"+strings.TrimSpace(m[1])+"</h2>", 1)
		count++
	}
	return out
}

var reBlockP = regexp.MustCompile(`(?is)<p\b[^>]*>([\s\S]*?)</p>`)

// Regex pasca-proses kerapian HTML (modul-level, pola statis).
var (
	reTidyHR          = regexp.MustCompile(`(?is)<hr\b[^>]*/?>`)
	reTidyDecorativeP = regexp.MustCompile(`(?is)<p\b[^>]*>\s*(?:[-*_=]\s*){3,}</p>`)
	reTidyEmptyBlock  = regexp.MustCompile(`(?is)<([a-z0-9]+)\b[^>]*>(?:\s|&nbsp;|<br\s*/?>)*</([a-z0-9]+)>`)
	reTidyDivOpen     = regexp.MustCompile(`(?is)<div\b[^>]*>`)
	reTidyDivClose    = regexp.MustCompile(`(?is)</div\s*>`)
	reTidyInlineAttr  = regexp.MustCompile(`(?is)\s(?:style|class|align|bgcolor)\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`)
	reTidyBareURLP    = regexp.MustCompile(`(?is)<p\b[^>]*>\s*(?:https?://|www\.)[^\s<>]*\s*</p>`)
	reTidyBlankLines  = regexp.MustCompile(`\n{3,}`)
	reAnyHeadingTag   = regexp.MustCompile(`(?i)<(/?)h([1-6])\b[^>]*>`)
)

// mapHTMLTextNodes: terapkan fn hanya pada text node (di luar tag; dalam <a> dilewati).
// Tokenizer manual di atas FindAllStringIndex: Regexp.Split di Go membuang
// capture group-nya (berbeda dari String.split di JS), jadi Split di sini
// dulu menghapus seluruh tag HTML dari artikel.
func mapHTMLTextNodes(html string, fn func(string) string) string {
	var out strings.Builder
	last := 0
	inA := false
	for _, m := range reTagSplit.FindAllStringIndex(html, -1) {
		text := html[last:m[0]]
		if !inA {
			text = fn(text)
		}
		out.WriteString(text)
		tag := html[m[0]:m[1]]
		if reOpenA.MatchString(tag) {
			inA = true
		} else if reCloseA.MatchString(tag) {
			inA = false
		}
		out.WriteString(tag)
		last = m[1]
	}
	tail := html[last:]
	if !inA {
		tail = fn(tail)
	}
	out.WriteString(tail)
	return out.String()
}

// escapeHTMLText: escape minimal untuk isi anchor.
func escapeHTMLText(v string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(v)
}

// isAlphaPhrase: true bila kata bisa dipakai sebagai anchor parsial.
func isAlphaPhrase(w string) bool {
	if w == "" {
		return false
	}
	first := w[0]
	if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
		return false
	}
	for i := 0; i < len(w); i++ {
		c := w[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '\'', c == '-', c == ' ', c == '_':
		default:
			return false
		}
	}
	return true
}

// weaveContextualLink: sisipkan link dengan membungkus frasa anchor yang ada.
func weaveContextualLink(html, url, anchor string, rel *string) string {
	attr := ""
	if rel != nil && *rel != "" {
		attr = ` rel="` + *rel + `"`
	}
	wrap := func(m string) string { return `<a href="` + url + `"` + attr + `>` + m + `</a>` }
	candidates := []string{anchor}
	words := []string{}
	for _, w := range strings.Fields(anchor) {
		if utf8.RuneCountInString(w) > 4 && isAlphaPhrase(w) {
			words = append(words, w)
		}
	}
	for i := 1; i < len(words); i++ {
		for j := i; j > 0 && len(words[j]) > len(words[j-1]); j-- {
			words[j], words[j-1] = words[j-1], words[j]
		}
	}
	candidates = append(candidates, words...)

	for _, phrase := range candidates {
		trimmed := strings.TrimSpace(phrase)
		if trimmed == "" {
			continue
		}
		re, err := regexp.Compile(`(?i)(^|[^\w@./-])(` + regexp.QuoteMeta(trimmed) + `)([\s),.;:!?]|$)`)
		if err != nil {
			continue
		}
		replaced := false
		out := mapHTMLTextNodes(html, func(text string) string {
			if replaced || !re.MatchString(text) {
				return text
			}
			replaced = true
			return re.ReplaceAllStringFunc(text, func(m string) string {
				sub := re.FindStringSubmatch(m)
				if sub == nil {
					return m
				}
				// Bungkus frasa (sub[2]); boundary kiri/kanan tetap di luar <a>.
				return sub[1] + wrap(sub[2]) + sub[3]
			})
		})
		if replaced {
			return out
		}
	}
	return ""
}

// weaveExactAnchorLink: seperti weaveContextualLink, tetapi HANYA mencocokkan
// frasa anchor secara utuh — tidak memecahnya jadi kata tunggal.
//
// Dipakai untuk tautan yang diminta klien: teks tautan yang terlihat harus
// persis anchor yang diketik klien. weaveContextualLink sengaja longgar (coba
// anchor penuh lalu kata-kata besarnya) karena untuk backlink kampanye frasa
// apa pun yang relevan masih dapat diterima; untuk tautan klien, melonggarkan
// berarti menampilkan anchor yang salah — mis. "sewa venue pernikahan" menjadi
// "venue" saja.
func weaveExactAnchorLink(html, url, anchor string, rel *string) string {
	trimmed := strings.TrimSpace(anchor)
	if trimmed == "" {
		return ""
	}
	attr := ""
	if rel != nil && *rel != "" {
		attr = ` rel="` + *rel + `"`
	}
	wrap := func(m string) string { return `<a href="` + url + `"` + attr + `>` + m + `</a>` }
	re, err := regexp.Compile(`(?i)(^|[^\w@./-])(` + regexp.QuoteMeta(trimmed) + `)([\s),.;:!?]|$)`)
	if err != nil {
		return ""
	}
	replaced := false
	out := mapHTMLTextNodes(html, func(text string) string {
		if replaced || !re.MatchString(text) {
			return text
		}
		replaced = true
		return re.ReplaceAllStringFunc(text, func(m string) string {
			sub := re.FindStringSubmatch(m)
			if sub == nil {
				return m
			}
			return sub[1] + wrap(sub[2]) + sub[3]
		})
	})
	if !replaced {
		return ""
	}
	return out
}

// hrefLinkReFor: pasangan <a> lengkap untuk satu URL. href ditoleransi &amp;
// (model sering meng-escape & di atribut) dan trailing slash — tanpa ini link
// yang sudah ditanam dianggap hilang lalu disisipkan lagi jadi link ganda.
func hrefLinkReFor(url string) *regexp.Regexp {
	meta := regexp.QuoteMeta(url)
	meta = strings.ReplaceAll(meta, "&", "&(?:amp;)?")
	return regexp.MustCompile(`(?i)<a\b[^>]*href=["']` + meta + `/?["'][^>]*>([\s\S]*?)</a>`)
}

// setAnchorTextForURL menimpa teks yang terlihat dari setiap <a href=url> yang
// SUDAH ada supaya persis sama dengan anchor yang diminta klien.
//
// Kasus nyata: klien mengisi anchor "resto murah jogja" untuk
// https://salsariresto.com, tetapi model menautkan URL itu dengan teks
// karangannya sendiri ("Salsari Resto website"). Penegak kebijakan lama hanya
// mengganti anchor ketika model menulis URL telanjang, jadi anchor klien tidak
// pernah dipakai dan tautannya tetap salah teks.
//
// Isi tautan yang mengandung tag (mis. <strong> atau <img>) dilewati: mengganti
// seluruhnya akan membuang markup yang mungkin disengaja.
func setAnchorTextForURL(html, url, anchor string) string {
	if url == "" || anchor == "" {
		return html
	}
	re := hrefLinkReFor(url)
	return re.ReplaceAllStringFunc(html, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if sub == nil {
			return m
		}
		inner := sub[1]
		if strings.Contains(inner, "<") {
			return m
		}
		if strings.TrimSpace(inner) == strings.TrimSpace(anchor) {
			return m
		}
		// Ganti hanya isi di antara tag pembuka dan penutup.
		open := strings.Index(m, ">")
		closeIdx := strings.LastIndex(m, "</a>")
		if open < 0 || closeIdx < 0 || closeIdx <= open {
			return m
		}
		return m[:open+1] + escapeHTMLText(anchor) + m[closeIdx:]
	})
}

// dedupeLinksByURL: satu URL wajib muncul maksimal sekali per artikel
// (aturan TEPAT 1x). Bila model menanam URL yang sama beberapa kali — dengan
// anchor diulang atau variasi — sisakan satu (yang memakai anchor spesifikasi
// bila ada, kalau tidak kemunculan pertama) dan kembalikan sisanya jadi teks.
func dedupeLinksByURL(html, url, anchor string) string {
	all := hrefLinkReFor(url).FindAllStringSubmatchIndex(html, -1)
	if len(all) < 2 {
		return html
	}
	// Prioritas keep: inner yang persis sama dengan anchor spesifikasi
	// (contoh model menulis "Bukalapak" vs "bukalapak"), baru yang mirip.
	keep := -1
	want := escapeHTMLText(anchor)
	for i, m := range all {
		if strings.TrimSpace(html[m[2]:m[3]]) == want {
			keep = i
			break
		}
	}
	if keep == -1 {
		wantLower := strings.ToLower(want)
		for i, m := range all {
			if strings.EqualFold(strings.TrimSpace(html[m[2]:m[3]]), wantLower) {
				keep = i
				break
			}
		}
	}
	if keep == -1 {
		keep = 0
	}
	var b strings.Builder
	last := 0
	for i, m := range all {
		if i == keep {
			b.WriteString(html[last:m[0]])
			b.WriteString(html[m[0]:m[1]])
			last = m[1]
			continue
		}
		b.WriteString(html[last:m[0]])
		b.WriteString(html[m[2]:m[3]])
		last = m[1]
	}
	b.WriteString(html[last:])
	return b.String()
}

// enforceTautBacklinkPolicy: penegak kebijakan link (deterministik).
func enforceTautBacklinkPolicy(html string, links []tautBacklinkSpec) string {
	out := html
	for _, spec := range links {
		if spec.URL == "" || spec.Anchor == "" {
			continue
		}
		bare := reHTTPScheme.ReplaceAllString(spec.URL, "")
		selfRe := regexp.MustCompile(`(?i)(<a\b[^>]*href=["']` + regexp.QuoteMeta(spec.URL) + `["'][^>]*>)\s*(?:https?://)?` + regexp.QuoteMeta(bare) + `\s*(</a>)`)
		out = selfRe.ReplaceAllString(out, "${1}"+escapeHTMLText(spec.Anchor)+"${2}")
		nakedRe := regexp.MustCompile(`(?i)(^|[^\w@./-])(?:https?://)?` + regexp.QuoteMeta(bare) + `([\s),.;:!?]|$)`)
		attr := ""
		if spec.Rel != nil && *spec.Rel != "" {
			attr = ` rel="` + *spec.Rel + `"`
		}
		out = mapHTMLTextNodes(out, func(text string) string {
			return nakedRe.ReplaceAllStringFunc(text, func(m string) string {
				sub := nakedRe.FindStringSubmatch(m)
				if sub == nil {
					return m
				}
				return sub[1] + `<a href="` + spec.URL + `"` + attr + `>` + escapeHTMLText(spec.Anchor) + `</a>` + sub[2]
			})
		})
		// TEPAT 1x: URL yang sama ditanam lebih dari satu → sisakan satu.
		out = dedupeLinksByURL(out, spec.URL, spec.Anchor)
		// Anchor milik klien WAJIB dipakai, termasuk ketika model sudah
		// menautkan URL yang benar dengan teks karangannya sendiri.
		out = setAnchorTextForURL(out, spec.URL, spec.Anchor)
	}
	if len(links) > 0 {
		primary := links[0]
		if primary.Mandatory && primary.URL != "" && primary.Anchor != "" {
			hasPrimary := hrefLinkReFor(primary.URL).MatchString(out)
			if !hasPrimary {
				woven := weaveContextualLink(out, primary.URL, primary.Anchor, primary.Rel)
				if woven != "" {
					out = woven
				} else {
					attr := ""
					if primary.Rel != nil && *primary.Rel != "" {
						attr = ` rel="` + *primary.Rel + `"`
					}
					sentence := ` Sebagai referensi, pembahasan mendalam tentang <a href="` + primary.URL + `"` + attr + `>` +
						escapeHTMLText(primary.Anchor) + `</a> tersedia pada situs resminya.`
					lastP := strings.LastIndex(out, "</p>")
					if lastP != -1 {
						out = out[:lastP] + sentence + "</p>" + out[lastP+4:]
					} else {
						out += "<p>" + strings.TrimSpace(sentence) + "</p>"
					}
				}
			}
		}
	}
	// Tautan klien: model sering mengabaikannya (terutama saat prompt angle
	// punya aturan tautan sendiri), dan jalur di atas hanya membungkus URL yang
	// SUDAH ditulis model — jadi anchor klien bisa hilang total. Di sini tiap
	// tautan klien yang belum ada di naskah ditanam paksa sebagai jaring
	// pengaman: bungkus frasa anchor bila frasa itu ada di teks, kalau tidak
	// sisipkan ke paragraf yang PALING BERKAITAN dengan anchor (bukan asal
	// paragraf terakhir) supaya kalimatnya tidak terasa ditempel.
	for _, spec := range links {
		if !spec.Client || spec.URL == "" || spec.Anchor == "" {
			continue
		}
		if hrefLinkReFor(spec.URL).MatchString(out) {
			continue
		}
		if woven := weaveExactAnchorLink(out, spec.URL, spec.Anchor, spec.Rel); woven != "" {
			out = woven
			continue
		}
		attr := ""
		if spec.Rel != nil && *spec.Rel != "" {
			attr = ` rel="` + *spec.Rel + `"`
		}
		link := `<a href="` + spec.URL + `"` + attr + `>` + escapeHTMLText(spec.Anchor) + `</a>`
		sentence := " Untuk " + link + ", Anda bisa melihat informasi lengkapnya lebih lanjut."
		out = insertSentenceIntoBestParagraph(out, spec.Anchor, sentence)
	}
	return out
}

// insertSentenceIntoBestParagraph: sisipkan satu kalimat ke paragraf yang
// paling berkaitan dengan anchor, diukur dari jumlah kata anchor yang muncul di
// paragraf itu. Dipakai hanya sebagai jaring pengaman ketika model melewatkan
// tautan klien sepenuhnya — memilih paragraf yang relevan membuat kalimat
// sisipan tidak terasa sebagai tempelan acak. Bila tidak ada paragraf yang
// berbagi satu kata pun dengan anchor, kalimat masuk ke paragraf terakhir.
func insertSentenceIntoBestParagraph(html, anchor, sentence string) string {
	lastP := strings.LastIndex(html, "</p>")
	if lastP == -1 {
		return html + "<p>" + strings.TrimSpace(sentence) + "</p>"
	}
	bestStart, bestScore := -1, 0
	for _, m := range reBlockP.FindAllStringSubmatchIndex(html, -1) {
		text := strings.ToLower(stripTagsForScoring(html[m[2]:m[3]]))
		score := 0
		for _, w := range strings.Fields(strings.ToLower(anchor)) {
			if utf8.RuneCountInString(w) < 4 {
				continue
			}
			if strings.Contains(text, w) {
				score++
			}
		}
		if score > bestScore {
			// m[3] = akhir isi paragraf (sebelum </p>), bukan m[1] yang sudah
			// melewati tag penutup — menyisipkan di m[1] menaruh kalimat di luar
			// paragraf sehingga hasilnya bukan kalimat dalam paragraf.
			bestScore, bestStart = score, m[3]
		}
	}
	if bestStart == -1 {
		bestStart = lastP
	}
	return html[:bestStart] + sentence + html[bestStart:]
}

// reAnyTag: semua tag HTML, dipakai untuk mencocokkan kata tanpa terganggu markup.
var reAnyTag = regexp.MustCompile(`(?is)<[^>]*>`)

// stripTagsForScoring: buang tag supaya pencocokan kata tidak terganggu markup.
func stripTagsForScoring(s string) string {
	return reAnyTag.ReplaceAllString(s, " ")
}
