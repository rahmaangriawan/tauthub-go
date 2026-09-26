package httpx

import "net/http"

// Port CORS dari src/proxy.ts — allowlist exact, credentials, OPTIONS 204.
var allowedOrigins = map[string]bool{
	"https://ai-writer-five-coral.vercel.app": true,
	"https://tulisio.agencyjogsel.web.id":     true,
	"http://localhost:3000":                   true,
	"http://localhost:3001":                   true, // dev web baru (vite); produksi same-origin lewat nginx
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("origin")
		if origin != "" && allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
