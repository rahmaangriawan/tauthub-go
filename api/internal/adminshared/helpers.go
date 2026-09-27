package adminshared

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func clampInt(raw string, def, min, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		n = def
	}
	if n < min {
		n = min
	}
	if n > max {
		n = max
	}
	return n
}

func trimSpace(s string) string { return strings.TrimSpace(s) }

func parseFloat(s string) (float64, error) { return strconv.ParseFloat(s, 64) }
