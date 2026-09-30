package httpx

import "net/http"

// SecurityHeaders: header keamanan dasar untuk SEMUA respons (API, /uploads,
// /feed, /t/{code}, dsb). Murni menambah header — tidak menyentuh isi, status,
// maupun perilaku endpoint.
//
// Catatan pilihan:
//   - X-Frame-Options SAMEORIGIN (bukan DENY) supaya /uploads/* masih bisa
//     ditampilkan di iframe same-origin; framing pihak ketiga tetap diblokir.
//   - CSP sengaja hanya berisi direktif yang TIDAK menyentuh script-src/style-src
//     (object-src/base-uri/frame-ancestors), jadi aplikasi SPA tidak mungkin
//     rusak karenanya. CSP penuh (script-src/style-src) adalah pekerjaan terpisah.
//   - HSTS hanya saat produksi (dev berjalan di http://localhost).
func SecurityHeaders(production bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("X-Frame-Options", "SAMEORIGIN")
			h.Set("Content-Security-Policy", "object-src 'none'; base-uri 'self'; frame-ancestors 'self'")
			if production {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
