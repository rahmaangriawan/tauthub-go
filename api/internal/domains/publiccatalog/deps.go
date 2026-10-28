// Package publiccatalog: endpoint BACA publik untuk direktori media
// (data.tauthub.com). Tanpa autentikasi, hanya media `status='active'` yang
// ditandai `public_listed`, dan hanya kolom yang aman dipublikasikan.
//
// Kontrak: JANGAN menambahkan field yang membocorkan sisi suplai (identitas
// mitra, harga mitra, markup, catatan internal, kredensial integrasi, topik
// terlarang, health breakdown). Semua field di sini terlihat oleh siapa pun
// dan akan diindeks mesin pencari.
package publiccatalog

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

type Deps struct {
	Pool *pgxpool.Pool
	// Enabled = cfg.PublicCatalogEnabled (PUBLIC_CATALOG_ENABLED).
	Enabled bool
}

// publicWhere: satu-satunya definisi "media yang boleh tampil publik".
// Dipakai semua query di paket ini supaya aturannya tidak bercabang.
const publicWhere = `m.status = 'active' AND m.public_listed
	AND m.public_slug IS NOT NULL AND m.public_slug <> ''`

// ---------- rate limit in-memory ----------
//
// Endpoint katalog publik bisa dirayapi crawler. Limiter berbasis DB
// (auth.CheckRate) menulis satu baris per request — terlalu mahal untuk trafik
// baca publik. Cukup penahan kasar per IP di memori proses.

const (
	publicRateLimitPerMinute = 240
	publicRateWindow         = time.Minute
)

type rateBucket struct {
	count int
	start time.Time
}

var (
	rateMu      sync.Mutex
	rateBuckets = map[string]*rateBucket{}
)

func allowPublic(ip string, limit int) bool {
	now := time.Now()
	rateMu.Lock()
	defer rateMu.Unlock()
	b, ok := rateBuckets[ip]
	if !ok || now.Sub(b.start) >= publicRateWindow {
		if len(rateBuckets) > 20_000 {
			for k, v := range rateBuckets {
				if now.Sub(v.start) >= publicRateWindow {
					delete(rateBuckets, k)
				}
			}
		}
		rateBuckets[ip] = &rateBucket{count: 1, start: now}
		return true
	}
	b.count++
	return b.count <= limit
}

// ---------- markup cache ----------

const defaultMarkupPercent = 15

var (
	markupMu  sync.Mutex
	markupVal float64
	markupExp time.Time
	markupOK  bool
)

// markupPercent: persentase markup aktif (pricing_config), cache 30 detik —
// parity getMarkupPercent di clientread. Harga publik = harga mitra × (1+markup).
func (d *Deps) markupPercent(r *http.Request) float64 {
	markupMu.Lock()
	if markupOK && markupExp.After(time.Now()) {
		v := markupVal
		markupMu.Unlock()
		return v
	}
	markupMu.Unlock()

	value := 0.0
	found := false
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT markup_percent FROM pricing_config WHERE id = 1 LIMIT 1").Scan(&value); err == nil {
		found = value >= 0
	}
	if !found {
		value = defaultMarkupPercent
	}
	markupMu.Lock()
	markupVal, markupExp, markupOK = value, time.Now().Add(30*time.Second), true
	markupMu.Unlock()
	return value
}

func clientPrice(partnerPrice, markup float64) float64 {
	base := partnerPrice
	if base < 0 {
		base = 0
	}
	m := markup
	if m < 0 {
		m = 0
	}
	v := base * (1 + m/100)
	return float64(int(v*100+0.5)) / 100
}

// ---------- gate + respons ----------

// gate: kill switch + rate limit. true = boleh lanjut.
func (d *Deps) gate(w http.ResponseWriter, r *http.Request) bool {
	if !d.Enabled {
		httpx.Error(w, http.StatusServiceUnavailable, "Katalog publik sedang tidak tersedia.")
		return false
	}
	// Panggilan internal (SSR web → API lewat loopback, tanpa header proxy)
	// tidak dibatasi: seluruh trafik situs akan berbagi satu bucket "unknown"
	// dan memicu 429 pada pengunjung sungguhan begitu trafik naik.
	if !internalCall(r) {
		ip := auth.RequestIp(r.Header)
		if ip == "" {
			ip = "unknown"
		}
		if !allowPublic(ip, publicRateLimitPerMinute) {
			w.Header().Set("Retry-After", "60")
			httpx.Error(w, http.StatusTooManyRequests, "Terlalu banyak permintaan. Coba lagi sebentar lagi.")
			return false
		}
	}
	return true
}

// internalCall: permintaan dari loopback TANPA header IP proxy — yaitu SSR web
// sendiri. Trafik publik selalu melewati nginx yang menambahkan X-Forwarded-For.
func internalCall(r *http.Request) bool {
	if auth.RequestIp(r.Header) != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writePublicJSON(w http.ResponseWriter, status int, payload any, maxAge int) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "public, max-age="+strconv.Itoa(maxAge)+", stale-while-revalidate="+strconv.Itoa(maxAge*4))
	// API baca publik: boleh dipanggil dari mana saja, tanpa kredensial.
	h.Set("Access-Control-Allow-Origin", "*")
	h.Del("Access-Control-Allow-Credentials")
	h.Set("Vary", "Accept-Encoding")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// ---------- query helpers ----------

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

func atoiOr(raw string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return n
}

// row: hasil query sebagai map kolom → nilai (pgx sudah menormalkan tipe).
type row map[string]any

func (m row) raw(key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}

func (m row) str(key string) string {
	switch t := m.raw(key).(type) {
	case string:
		return t
	case []byte:
		return string(t)
	}
	return ""
}

func (m row) num(key string) (float64, bool) {
	switch t := m.raw(key).(type) {
	case float64:
		return t, true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case int16:
		return float64(t), true
	case int:
		return float64(t), true
	case pgtype.Numeric:
		if !t.Valid {
			return 0, false
		}
		f8, err := t.Float64Value()
		if err != nil || !f8.Valid {
			return 0, false
		}
		return f8.Float64, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func (m row) numOr(key string) float64 {
	f, _ := m.num(key)
	return f
}

func (m row) numPtr(key string) any {
	if f, ok := m.num(key); ok {
		return f
	}
	return nil
}

func (m row) intOr(key string, fallback int) int {
	if f, ok := m.num(key); ok {
		return int(f)
	}
	return fallback
}

func (m row) intPtr(key string) any {
	if f, ok := m.num(key); ok {
		return int(f)
	}
	return nil
}

func (m row) boolOr(key string, fallback bool) bool {
	if b, ok := m.raw(key).(bool); ok {
		return b
	}
	return fallback
}

func (m row) timePtr(key string) any {
	t, ok := m.raw(key).(time.Time)
	if !ok || t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// queryRows: jalankan SELECT dan kembalikan []row dengan nama kolom asli.
func (d *Deps) queryRows(r *http.Request, sql string, args ...any) ([]row, error) {
	rows, err := d.Pool.Query(r.Context(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = string(f.Name)
	}
	out := []row{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := make(row, len(names))
		for i, name := range names {
			item[name] = normalizeValue(values[i])
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func normalizeValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeValue(item)
		}
		return out
	case pgtype.Numeric:
		if !t.Valid {
			return nil
		}
		f8, err := t.Float64Value()
		if err != nil || !f8.Valid {
			return nil
		}
		return f8.Float64
	default:
		return v
	}
}

// ---------- pemetaan field publik ----------

// splitCategories: pisahkan gabungan kategori dari string_agg (pemisah \u0001)
// plus koma untuk data lama yang menaruh beberapa kategori dalam satu nilai.
func splitCategories(csv string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, part := range strings.Split(csv, "\u0001") {
		for _, piece := range strings.Split(part, ",") {
			name := strings.TrimSpace(piece)
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true
			out = append(out, name)
		}
	}
	return out
}

// hostOf: "https://www.kompas.com/x" → "kompas.com".
func hostOf(rawURL string) string {
	s := strings.TrimSpace(rawURL)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimPrefix(strings.ToLower(s), "www.")
	return s
}

// parseLanguageList: bahasa_artikel bisa berupa JSON array, CSV, atau teks.
func parseLanguageList(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return []string{}
	}
	if strings.HasPrefix(s, "[") {
		var list []string
		if err := json.Unmarshal([]byte(s), &list); err == nil {
			out := []string{}
			for _, v := range list {
				if v = strings.TrimSpace(v); v != "" {
					out = append(out, v)
				}
			}
			return out
		}
	}
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func logErr(what string, err error) {
	slog.Error("[publiccatalog] "+what, "err", err)
}
