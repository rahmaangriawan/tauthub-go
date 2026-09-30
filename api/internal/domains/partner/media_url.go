package partner

import (
	"os"
	"strings"
)

// relSelfURL: ubah URL gambar yang menunjuk ke aplikasi ini sendiri menjadi
// path relatif (mis. "https://app.tauthub.com/uploads/x.webp" -> "/uploads/x.webp").
//
// Alasannya bukan kosmetik: berkas /uploads butuh cookie sesi. Jika URL-nya
// absolut ke domain lain, permintaan gambar jadi lintas-situs sehingga cookie
// (SameSite=Lax) tidak ikut terkirim -> 401 -> gambar tidak tampil. Dengan path
// relatif, browser meminta ke origin yang sedang dibuka, apa pun host-nya
// (localhost, staging, produksi).
//
// URL eksternal (host lain) dibiarkan apa adanya.
func relSelfURL(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || !strings.HasPrefix(v, "http") {
		return v
	}
	for _, host := range selfHosts() {
		for _, scheme := range []string{"https://", "http://"} {
			prefix := scheme + host
			if strings.HasPrefix(strings.ToLower(v), prefix) {
				rest := v[len(prefix):]
				if rest == "" {
					return v
				}
				if !strings.HasPrefix(rest, "/") {
					rest = "/" + rest
				}
				return rest
			}
		}
	}
	return v
}

// selfHosts: host yang dianggap milik aplikasi ini sendiri (tanpa skema).
func selfHosts() []string {
	var hosts []string
	if pub := strings.TrimSpace(os.Getenv("PUBLIC_APP_URL")); pub != "" {
		h := pub
		h = strings.TrimPrefix(h, "https://")
		h = strings.TrimPrefix(h, "http://")
		h = strings.TrimSuffix(h, "/")
		if h != "" {
			hosts = append(hosts, strings.ToLower(h))
		}
	}
	hosts = append(hosts, "app.tauthub.com", "tauthub.com", "www.tauthub.com")
	return hosts
}

// relSelfURLN: varian untuk *string (nil tetap nil).
func relSelfURLN(s *string) *string {
	if s == nil {
		return nil
	}
	out := relSelfURL(*s)
	return &out
}

// relSelfURLAny: varian untuk nilai any (dipakai payload yang fieldnya bisa nil).
func relSelfURLAny(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	return relSelfURL(s)
}

// relSelfHTMLAny: varian any dari relSelfHTML (payload field bisa nil).
func relSelfHTMLAny(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	return relSelfHTML(s)
}

// relSelfHTML: tulis ulang src gambar "/uploads/..." yang ditulis absolut ke
// domain sendiri menjadi path relatif, supaya pratinjau artikel tetap memuat
// gambarnya. Hanya menyasar URL yang menunjuk ke /uploads/ — tautan halaman
// biasa dibiarkan absolut.
func relSelfHTML(html string) string {
	if html == "" {
		return html
	}
	lower := strings.ToLower(html)
	for _, host := range selfHosts() {
		for _, scheme := range []string{"https://", "http://"} {
			needle := scheme + host + "/uploads/"
			for from := 0; ; {
				pos := strings.Index(lower[from:], needle)
				if pos < 0 {
					break
				}
				at := from + pos
				html = html[:at] + "/uploads/" + html[at+len(needle):]
				lower = lower[:at] + "/uploads/" + lower[at+len(needle):]
				from = at + len("/uploads/")
			}
		}
	}
	return html
}
