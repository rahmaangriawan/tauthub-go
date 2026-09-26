package httpx

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// Error menulis envelope error kontrak app lama: {"detail": "..."} (gaya FastAPI,
// port dari src/auth/http.ts). Semua handler wajib memakai ini untuk error.
func Error(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": detail})
}

// Scrub500: di production, pesan 500 diganti pesan generik berbahasa Indonesia
// (parity apiError di src/auth/http.ts). Detail asli tetap dilog.
func Scrub500(appEnv, detail string) string {
	if appEnv == "production" {
		return "Terjadi kesalahan internal. Coba lagi."
	}
	return detail
}

// Recover mengubah panic handler menjadi 500 ter-scrub, bukan crash server.
func Recover(appEnv string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("panic di handler", "err", rec, "method", r.Method, "path", r.URL.Path)
					Error(w, http.StatusInternalServerError, Scrub500(appEnv, fmt.Sprintf("%v", rec)))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
