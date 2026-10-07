package content

import (
	"regexp"
	"strings"
)

// NormalizeArticleHTML merapikan HTML artikel yang berasal dari editor
// berbasis contenteditable. Chrome menulis satu <div> per baris dan
// <div><br></div> untuk baris kosong, sehingga HTML tersimpan penuh tag
// sampah. Fungsi ini:
//
//   - mengubah blok <div>/<section> menjadi paragraf <p> (termasuk yang
//     bersarang), bukan sekadar menghapus tag-nya, supaya baris tidak menyatu;
//   - membuang blok kosong (<div><br></div>, <p>&nbsp;</p>, dan sejenisnya);
//   - membungkus teks telanjang di luar blok menjadi paragraf (baris pertama
//     hasil editor biasanya tanpa pembungkus).
//
// Elemen lain (heading, daftar, kutipan, tabel, gambar, tautan, dan entitas
// seperti &amp;) dibiarkan apa adanya. Fungsi ini idempoten: HTML yang sudah
// berparagraf rapi tidak berubah.
func NormalizeArticleHTML(html string) string {
	if strings.TrimSpace(html) == "" {
		return html
	}

	var out strings.Builder
	var buf strings.Builder
	rawRoot := ""
	rawDepth := 0

	writeSep := func() {
		if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
			out.WriteString("\n")
		}
	}
	flush := func() {
		chunk := strings.TrimSpace(buf.String())
		buf.Reset()
		if !articleHasVisibleContent(chunk) {
			return
		}
		for _, part := range articleParagraphs(chunk) {
			writeSep()
			out.WriteString("<p>")
			out.WriteString(part)
			out.WriteString("</p>")
		}
	}

	last := 0
	for _, loc := range reArticleTag.FindAllStringIndex(html, -1) {
		lead := html[last:loc[0]]
		if rawRoot != "" {
			out.WriteString(lead)
		} else {
			buf.WriteString(lead)
		}
		last = loc[1]
		tag := html[loc[0]:loc[1]]
		name, closing := articleTagName(tag)

		if rawRoot != "" {
			out.WriteString(tag)
			switch {
			case closing && name == rawRoot:
				rawDepth--
				if rawDepth <= 0 {
					rawRoot = ""
					rawDepth = 0
				}
			case !closing && name == rawRoot && !articleVoidTag(name) && !strings.HasSuffix(tag, "/>"):
				rawDepth++
			}
			continue
		}

		switch {
		case name == "hr" && !closing:
			flush()
		case articleRawTags[name]:
			flush()
			writeSep()
			out.WriteString(tag)
			if !closing && !strings.HasSuffix(tag, "/>") {
				rawRoot = name
				rawDepth = 1
			}
		case articleStructuralTags[name]:
			// Pembungkus layout: batas paragraf, tag-nya dibuang.
			flush()
		case articleVoidTag(name):
			if !closing {
				buf.WriteString(tag)
			}
		default:
			buf.WriteString(tag)
		}
	}
	tail := html[last:]
	if rawRoot != "" {
		out.WriteString(tail)
	} else {
		buf.WriteString(tail)
	}
	flush()
	return tidyArticleHTML(out.String())
}

// tidyArticleHTML: sapuan akhir, membuang blok yang tetap kosong (mis. <p></p>
// dari editor) sampai stabil, lalu merapikan baris kosong.
func tidyArticleHTML(html string) string {
	out := html
	for i := 0; i < 4; i++ {
		next := reArticleEmptyBlock.ReplaceAllStringFunc(out, func(m string) string {
			sub := reArticleEmptyBlock.FindStringSubmatch(m)
			if sub == nil || !strings.EqualFold(sub[1], sub[2]) {
				return m
			}
			if articleEmptyRemovable[strings.ToLower(sub[1])] {
				return ""
			}
			return m
		})
		if next == out {
			break
		}
		out = next
	}
	out = reArticleBlankRuns.ReplaceAllString(out, "\n")
	return strings.TrimSpace(out)
}

// articleParagraphs memecah isi satu blok. Teks tanpa tag sama sekali dipecah
// per baris kosong supaya tempelan teks polos tetap jadi paragraf terpisah.
func articleParagraphs(chunk string) []string {
	if strings.Contains(chunk, "<") {
		return []string{chunk}
	}
	parts := reArticleBlankLine.Split(chunk, -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return []string{chunk}
	}
	return out
}

// articleHasVisibleContent: blok dianggap kosong bila hanya berisi tag, spasi,
// dan &nbsp; (mis. <div><br></div>) tanpa gambar/media.
func articleHasVisibleContent(chunk string) bool {
	if chunk == "" {
		return false
	}
	text := reArticleAnyTag.ReplaceAllString(chunk, "")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	if strings.TrimSpace(text) != "" {
		return true
	}
	return reArticleMediaTag.MatchString(chunk)
}

// articleTagName mengembalikan nama tag huruf kecil dan apakah ia tag penutup.
func articleTagName(tag string) (string, bool) {
	m := reArticleTagName.FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	return strings.ToLower(m[1]), strings.HasPrefix(strings.TrimSpace(tag[1:]), "/")
}

// articleStructuralTags: pembungkus layout dari editor. Tag-nya dibuang dan
// isinya dipecah jadi paragraf.
var articleStructuralTags = map[string]bool{
	"div": true, "section": true, "article": true, "main": true, "aside": true,
	"header": true, "footer": true, "nav": true, "address": true, "center": true,
	"form": true, "fieldset": true, "font": true,
}

// articleRawTags: elemen yang isinya dibiarkan apa adanya (tidak dipecah jadi
// paragraf). Elemen di dalamnya ikut apa adanya karena mode ini bertahan
// sampai tag penutup yang sepadan.
var articleRawTags = map[string]bool{
	"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "blockquote": true, "pre": true, "figure": true,
	"table": true, "dl": true, "script": true, "style": true,
}

var articleVoidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "param": true,
	"source": true, "track": true, "wbr": true,
}

func articleVoidTag(name string) bool { return articleVoidTags[name] }

// articleEmptyRemovable: blok yang boleh hilang total bila isinya kosong.
var articleEmptyRemovable = map[string]bool{
	"p": true, "div": true, "section": true, "span": true, "li": true,
	"ul": true, "ol": true, "blockquote": true, "figure": true, "table": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

var (
	// Token tag: atribut berisi ">" di dalam tanda kutip tetap utuh.
	reArticleTag        = regexp.MustCompile(`(?is)<(?:[^>"']|"[^"]*"|'[^']*')*>`)
	reArticleAnyTag     = regexp.MustCompile(`(?is)<[^>]*>`)
	reArticleTagName    = regexp.MustCompile(`(?is)^</?\s*([a-z][a-z0-9]*)`)
	reArticleMediaTag   = regexp.MustCompile(`(?is)<(?:img|iframe|video|audio|embed|picture|svg|object)\b`)
	reArticleBlankLine  = regexp.MustCompile(`\n\s*\n`)
	reArticleEmptyBlock = regexp.MustCompile(`(?is)<([a-z][a-z0-9]*)\b[^>]*>(?:\s|&nbsp;|<br\s*/?>)*</([a-z][a-z0-9]*)\s*>`)
	reArticleBlankRuns  = regexp.MustCompile(`\n{2,}`)
)
