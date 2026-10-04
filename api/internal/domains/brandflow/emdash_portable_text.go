package brandflow

import (
	"html"
	"strings"
)

// emdash_portable_text.go: konversi HTML artikel (content_versions.content
// berformat html) menjadi blok Portable Text EmDash.
//
// Adapter lama hanya punya markdownToPortableText, padahal isi artikel yang
// tersimpan selalu HTML — akibatnya seluruh badan artikel terkirim sebagai teks
// bertag dan gambar tidak pernah terbaca. Konverter ini menangani tag yang
// memang dipakai penulis: p, h1..h6, ul/ol/li, blockquote, strong/em/del/code,
// a, img, br, dan tabel (diratakan jadi baris teks karena tidak ada bentuk
// tabel di skema konten).

// emdashImageRef: gambar yang sudah ada di media library EmDash.
type emdashImageRef struct {
	ID  string
	URL string
}

// htmlToPortableText: blok PT dari HTML. images memetakan src gambar ke
// referensi media hasil unggahan; src yang tidak ada di peta dipakai apa adanya
// (gambar eksternal pada badan artikel).
func htmlToPortableText(document string, images map[string]emdashImageRef) []map[string]any {
	blocks := []map[string]any{}
	var spans []map[string]any
	var markDefs []map[string]any
	var markStack []string
	href := ""
	style := "normal"
	listItem := ""
	listStyle := ""
	quoteDepth := 0
	tableRow := false
	rowCells := 0

	flush := func() {
		if len(spans) == 0 {
			return
		}
		block := map[string]any{
			"_type": "block", "style": style,
			"children": spans, "markDefs": markDefs,
		}
		if listItem != "" {
			block["listItem"] = listItem
			block["level"] = 1
		}
		blocks = append(blocks, block)
		spans = nil
		markDefs = nil
	}
	// resetBlock: tutup blok berjalan lalu kembalikan gaya dasar sesuai konteks.
	resetBlock := func() {
		flush()
		style = "normal"
		if quoteDepth > 0 {
			style = "blockquote"
		}
		listItem = ""
	}

	appendText := func(text string) {
		if text == "" {
			return
		}
		marks := []string{}
		marks = append(marks, markStack...)
		if href != "" {
			marks = append(marks, "link:"+href)
			if !peHasMarkDef(markDefs, href) {
				markDefs = append(markDefs, map[string]any{
					"_key": "link:" + href, "_type": "link", "href": href,
				})
			}
		}
		spans = append(spans, map[string]any{
			"_type": "span", "text": html.UnescapeString(text), "marks": marks,
		})
	}

	i := 0
	for i < len(document) {
		next := strings.IndexByte(document[i:], '<')
		if next < 0 {
			appendText(document[i:])
			break
		}
		appendText(document[i : i+next])
		start := i + next
		end := peTagEnd(document, start)
		if end < 0 {
			appendText(document[start:])
			break
		}
		tag := document[start:end]
		name := peTagName(tag)
		closing := strings.HasPrefix(strings.TrimSpace(tag[1:]), "/")
		switch name {
		case "p", "div", "section", "article":
			resetBlock()
		case "h1", "h2", "h3", "h4", "h5", "h6":
			resetBlock()
			if !closing {
				style = name
			}
		case "blockquote":
			resetBlock()
			if closing {
				if quoteDepth > 0 {
					quoteDepth--
				}
				style = "normal"
			} else {
				quoteDepth++
				style = "blockquote"
			}
		case "ul", "ol":
			resetBlock()
			if closing {
				listStyle = ""
			} else if name == "ol" {
				listStyle = "number"
			} else {
				listStyle = "bullet"
			}
		case "li":
			resetBlock()
			if !closing {
				listItem = listStyle
				if listItem == "" {
					listItem = "bullet"
				}
			}
		case "br":
			appendText("\n")
		case "strong", "b":
			markStack = peToggleMark(markStack, "strong", !closing)
		case "em", "i":
			markStack = peToggleMark(markStack, "em", !closing)
		case "del", "s", "strike":
			markStack = peToggleMark(markStack, "strike-through", !closing)
		case "code":
			markStack = peToggleMark(markStack, "code", !closing)
		case "a":
			if closing {
				href = ""
			} else if value, _, _, _ := peAttrValue(tag, "href"); value != "" {
				href = strings.TrimSpace(value)
			}
		case "img":
			resetBlock()
			if block, ok := emdashImageBlock(tag, images); ok {
				blocks = append(blocks, block)
			}
		case "table":
			resetBlock()
		case "tr":
			resetBlock()
			tableRow = !closing
			rowCells = 0
		case "td", "th":
			if tableRow && !closing {
				if rowCells > 0 {
					appendText(" | ")
				}
				rowCells++
			}
		case "thead", "tbody", "tfoot":
			// struktur tabel tidak dipakai; baris sudah diratakan jadi teks.
		}
		i = end
	}
	resetBlock()
	if len(spans) > 0 {
		flush()
	}
	return blocks
}

// emdashImageBlock: blok image PT; asset.url dipakai renderer EmDash untuk
// media penyimpanan lokal (asset._ref saja tidak menyelesaikan berkas).
func emdashImageBlock(tag string, images map[string]emdashImageRef) (map[string]any, bool) {
	src, _, _, _ := peAttrValue(tag, "src")
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, false
	}
	alt, _, _, _ := peAttrValue(tag, "alt")
	ref := images[src]
	url := ref.URL
	if url == "" {
		url = src
	}
	block := map[string]any{
		"_type": "image",
		"_key":  "img-" + peInlineImageName("", src),
		"asset": map[string]any{"_ref": ref.ID, "url": url},
	}
	if strings.TrimSpace(alt) != "" {
		block["alt"] = html.UnescapeString(alt)
	}
	return block, true
}

// peTagName: nama tag huruf kecil tanpa '/' dan atribut.
func peTagName(tag string) string {
	body := strings.TrimSpace(tag)
	body = strings.TrimPrefix(body, "<")
	body = strings.TrimPrefix(body, "/")
	end := 0
	for end < len(body) {
		ch := body[end]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			end++
			continue
		}
		break
	}
	return strings.ToLower(body[:end])
}

// peToggleMark: tambah/hapus mark pada tumpukan mark aktif.
func peToggleMark(stack []string, mark string, add bool) []string {
	if add {
		for _, m := range stack {
			if m == mark {
				return stack
			}
		}
		return append(stack, mark)
	}
	out := make([]string, 0, len(stack))
	for _, m := range stack {
		if m != mark {
			out = append(out, m)
		}
	}
	return out
}

func peHasMarkDef(defs []map[string]any, href string) bool {
	for _, def := range defs {
		if bsStringifyAny(def["_key"]) == "link:"+href {
			return true
		}
	}
	return false
}
