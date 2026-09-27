package brandflow

import (
	"regexp"
	"strings"
)

// taut_backlink.go: port lib/taut/agents.ts bagian post-processing artikel
// (demote/normalize headings + enforcement backlink deterministik).

type tautBacklinkSpec struct {
	URL       string
	Anchor    string
	Rel       *string
	Mandatory bool
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
	return out
}

// demoteHeadingBlocks: turunkan h1-h4 jadi paragraf (media mainstream).
func demoteHeadingBlocks(html string) string {
	if html == "" || !strings.Contains(html, "<h") {
		return html
	}
	out := reH1234Open.ReplaceAllString(html, "<p>")
	return reH1234Close.ReplaceAllString(out, "</p>")
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
		if len([]rune(inner)) > 90 || len(strings.Fields(inner)) > 12 {
			continue
		}
		out = strings.Replace(out, m[0], "<h2>"+strings.TrimSpace(m[1])+"</h2>", 1)
		count++
	}
	return out
}

var reBlockP = regexp.MustCompile(`(?is)<p\b[^>]*>([\s\S]*?)</p>`)

// mapHTMLTextNodes: terapkan fn hanya pada text node (di luar tag; dalam <a> dilewati).
func mapHTMLTextNodes(html string, fn func(string) string) string {
	parts := reTagSplit.Split(html, -1)
	var out strings.Builder
	inA := false
	for _, part := range parts {
		if strings.HasPrefix(part, "<") {
			if reOpenA.MatchString(part) {
				inA = true
			} else if reCloseA.MatchString(part) {
				inA = false
			}
			out.WriteString(part)
			continue
		}
		if inA {
			out.WriteString(part)
		} else {
			out.WriteString(fn(part))
		}
	}
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
		if len([]rune(w)) > 4 && isAlphaPhrase(w) {
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
		re, err := regexp.Compile(`(?i)(^|[^\w@./-])` + regexp.QuoteMeta(trimmed) + `([\s),.;:!?]|$)`)
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
				return sub[1] + wrap(sub[2])
			})
		})
		if replaced {
			return out
		}
	}
	return ""
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
	}
	if len(links) > 0 {
		primary := links[0]
		if primary.Mandatory && primary.URL != "" && primary.Anchor != "" {
			hasPrimary := regexp.MustCompile(`(?i)<a\b[^>]*href=["']` + regexp.QuoteMeta(primary.URL) + `["']`).MatchString(out)
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
	return out
}
