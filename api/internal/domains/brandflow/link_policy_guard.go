package brandflow

import (
	"net/url"
	"regexp"
	"strings"
)

// link_policy_guard.go: penegakan BATAS JUMLAH tautan keluar per artikel
// terhadap kebijakan media (allow_hyperlink + max_links).
//
// Sebelumnya hanya "boleh/tidak boleh" yang dicek (max_links > 0). Akibatnya
// artikel bisa lolos QC dan terbit dengan 5 tautan padahal media hanya
// menyetujui 2. Guard ini menghitung tautan keluar yang sebenarnya lalu
// dipakai gatekeeper (status artikel) dan gerbang distribusi (hard stop).

var reHrefAttr = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)

// MediaLinkCap / CountOutboundLinks / HostSet — pembungkus ekspor untuk
// domain lain (clienttaut) yang menjalankan gatekeeper versi wizard lama.
func MediaLinkCap(allowHyperlink bool, maxLinks *int) (int, bool) {
	return mediaLinkCap(allowHyperlink, maxLinks)
}

func CountOutboundLinks(html string, ownDomains map[string]bool) int {
	return countOutboundLinks(html, ownDomains)
}

func HostSet(urls ...string) map[string]bool { return hostSet(urls...) }

// mediaLinkCap: batas tautan keluar menurut kebijakan media.
//   - tidak menerima tautan        → cap 0 (tidak boleh ada tautan sama sekali)
//   - menerima, max_links terisi   → cap = max_links
//   - menerima, max_links NULL     → cap 2 (baris legacy; parity label UI
//     "Maks. 2 Dofollow")
//
// allowed=false + cap 0 tetap dibedakan oleh pemanggil lewat return kedua.
func mediaLinkCap(allowHyperlink bool, maxLinks *int) (int, bool) {
	if maxLinks != nil {
		if *maxLinks <= 0 {
			return 0, false
		}
		return *maxLinks, true
	}
	if !allowHyperlink {
		return 0, false
	}
	return 2, true
}

// countOutboundLinks: jumlah anchor http(s) yang menuju domain LAIN dari
// domain media (self-link situs media bukan backlink). mailto:/tel:/
// javascript:/#/URL relatif tidak dihitung. Satu anchor = satu tautan, walau
// URL-nya sama (media membatasi jumlah tautan fisik di artikel).
func countOutboundLinks(html string, ownDomains map[string]bool) int {
	if strings.TrimSpace(html) == "" {
		return 0
	}
	n := 0
	for _, m := range reHrefAttr.FindAllStringSubmatch(html, -1) {
		href := strings.TrimSpace(m[1])
		low := strings.ToLower(href)
		if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
			continue
		}
		u, err := url.Parse(href)
		if err != nil || u.Hostname() == "" {
			continue
		}
		host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
		if ownDomains != nil && ownDomains[host] {
			continue
		}
		n++
	}
	return n
}

// hostSet: himpunan host (tanpa www) dari daftar URL; dipakai untuk
// mengecualikan tautan ke situs media sendiri.
func hostSet(urls ...string) map[string]bool {
	out := map[string]bool{}
	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue
		}
		out[strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")] = true
	}
	return out
}
