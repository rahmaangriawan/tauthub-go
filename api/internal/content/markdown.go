package content

import (
	"regexp"
	"strings"
)

// Port lib/markdown.ts markdownToHtml — konversi minimal untuk editor/preview.
// Urutan & regex dipertahankan (parity).

var (
	mdBoldRe      = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdItalicRe    = regexp.MustCompile(`\*([^*]+?)\*`)
	mdStrikeRe    = regexp.MustCompile(`~~([^~]+)~~`)
	mdCodeRe      = regexp.MustCompile("`([^`]+)`")
	mdImageRe     = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	mdLinkRe      = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdHeadingRe   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdQuoteRe     = regexp.MustCompile(`^>\s+(.*)$`)
	mdOrderedRe   = regexp.MustCompile(`^\d+[.)]\s+(.*)$`)
	mdUnorderedRe = regexp.MustCompile(`^[-*+]\s+(.*)$`)
)

func inline(text string) string {
	out := mdBoldRe.ReplaceAllString(text, "<strong>$1</strong>")
	out = mdItalicRe.ReplaceAllString(out, "<em>$1</em>")
	out = mdStrikeRe.ReplaceAllString(out, "<del>$1</del>")
	out = mdCodeRe.ReplaceAllString(out, "<code>$1</code>")
	out = mdImageRe.ReplaceAllString(out, `<img src="$2" alt="$1" style="max-width:100%;height:auto;border-radius:8px" />`)
	out = mdLinkRe.ReplaceAllString(out, `<a href="$2" target="_blank" rel="noreferrer">$1</a>`)
	return out
}

// MarkdownToHtml: parity markdownToHtml (heading, quote, list, paragraf,
// fence "---" dari front matter di-skip).
func MarkdownToHtml(markdown string) string {
	html := []string{}
	var listType string // "" | ul | ol
	paragraph := []string{}

	flush := func() {
		if len(paragraph) > 0 {
			html = append(html, "<p>"+inline(strings.Join(paragraph, " "))+"</p>")
			paragraph = []string{}
		}
	}
	closeList := func() {
		if listType != "" {
			html = append(html, "</"+listType+">")
			listType = ""
		}
	}
	openList := func(typ string) {
		if listType != typ {
			closeList()
			html = append(html, "<"+typ+">")
			listType = typ
		}
	}

	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	for _, raw := range strings.Split(normalized, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			closeList()
			continue
		}
		if line == "---" {
			continue
		}
		if m := mdHeadingRe.FindStringSubmatch(line); m != nil {
			flush()
			closeList()
			level := len(m[1])
			html = append(html, "<h"+itoa(level)+">"+inline(m[2])+"</h"+itoa(level)+">")
			continue
		}
		if m := mdQuoteRe.FindStringSubmatch(line); m != nil {
			flush()
			closeList()
			html = append(html, "<blockquote>"+inline(m[1])+"</blockquote>")
			continue
		}
		if m := mdOrderedRe.FindStringSubmatch(line); m != nil {
			flush()
			openList("ol")
			html = append(html, "<li>"+inline(m[1])+"</li>")
			continue
		}
		if m := mdUnorderedRe.FindStringSubmatch(line); m != nil {
			flush()
			openList("ul")
			html = append(html, "<li>"+inline(m[1])+"</li>")
			continue
		}
		paragraph = append(paragraph, line)
	}
	flush()
	closeList()
	return strings.Join(html, "\n")
}
