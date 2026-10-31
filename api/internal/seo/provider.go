// Package seo: klien tunggal provider DA/PA (RapidAPI "Bulk DA PA Checker
// Semrush" oleh KSDeveloper).
//
// Satu tempat untuk kedua pemakai:
//   - domain platform (metrik DA/PA media, media_seo_metrics.da/pa),
//   - domain partner (tool "Cek DA/PA" di portal mitra).
//
// Host/path/nama field body bisa ditimpa lewat platform_seo_config supaya
// penyesuaian kontrak API cukup lewat DB, tanpa ubah kode.
package seo

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

const (
	// Kontrak RapidAPI (KSDeveloper — Bulk DA PA Checker Semrush).
	DefaultHost    = "bulk-da-pa-checker-semrush.p.rapidapi.com"
	DefaultPath    = "/bulk-dapa.php"
	DefaultBodyKey = "domains"
	// Halaman listing untuk kredit di admin Konfigurasi.
	ProviderPageURL = "https://rapidapi.com/ksdeveloper/api/bulk-da-pa-checker-semrush"
	ProviderLabel   = "KSDeveloper"
)

// Config: alamat provider yang dipakai saat ini.
type Config struct {
	Host    string // tanpa skema, mis. bulk-da-pa-checker-semrush.p.rapidapi.com
	Path    string // mis. /bulk-dapa.php
	BodyKey string // nama field form daftar domain (dipisah koma), mis. domains
	// CacheTTLHrs: umur data DA/PA yang dianggap masih segar (media cache).
	CacheTTLHrs int
}

const DefaultCacheTTLHrs = 1440 // 60 hari, setara aturan lama "2 bulan"

func DefaultConfig() Config {
	return Config{Host: DefaultHost, Path: DefaultPath, BodyKey: DefaultBodyKey, CacheTTLHrs: DefaultCacheTTLHrs}
}

// LoadConfig: baca override dari platform_seo_config; kolom kosong = default.
// Kegagalan query tidak mematikan fitur — pakai default.
func LoadConfig(ctx context.Context, pool *pgxpool.Pool) Config {
	cfg := DefaultConfig()
	if pool == nil {
		return cfg
	}
	var host, path, bodyKey *string
	var ttl *int
	err := pool.QueryRow(ctx, `
		SELECT provider_host, provider_path, provider_domain_field, cache_ttl_hours
		FROM platform_seo_config WHERE id = 'default' LIMIT 1`).Scan(&host, &path, &bodyKey, &ttl)
	if err != nil {
		return cfg
	}
	if ttl != nil && *ttl > 0 {
		cfg.CacheTTLHrs = *ttl
	}
	if host != nil && strings.TrimSpace(*host) != "" {
		cfg.Host = strings.TrimPrefix(strings.TrimSpace(*host), "https://")
	}
	if path != nil && strings.TrimSpace(*path) != "" {
		p := strings.TrimSpace(*path)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		cfg.Path = p
	}
	if bodyKey != nil && strings.TrimSpace(*bodyKey) != "" {
		cfg.BodyKey = strings.TrimSpace(*bodyKey)
	}
	return cfg
}

// URL: alamat lengkap endpoint provider.
func (c Config) URL() string { return "https://" + c.Host + c.Path }

// Metrics: nilai yang diambil dari satu domain.
type Metrics struct {
	Domain string
	DA     float64
	PA     float64
	Spam   *float64
}

// Result: hasil satu panggilan batch — metrik per domain + raw respons.
type Result struct {
	Metrics map[string]*Metrics // key = domain ternormalisasi
	Missing []string            // domain valid yang tidak muncul di respons
	Raw     string              // potongan respons mentah (diagnosa admin)
}

// ErrorKind: kategori kegagalan supaya pemanggil bisa memetakan pesan.
type ErrorKind int

const (
	KindUnreachable ErrorKind = iota
	KindAuth
	KindRateLimited
	KindQuota
	KindNotFound
	KindBadStatus
	KindMalformed
)

// Error: kegagalan provider dengan kategori + detail mentah.
type Error struct {
	Kind    ErrorKind
	Status  int
	Message string
	Body    string
}

func (e *Error) Error() string { return e.Message }

// ErrText: pesan Indonesia standar untuk sebuah kegagalan provider.
func ErrText(e *Error) string {
	if e == nil {
		return ""
	}
	switch e.Kind {
	case KindUnreachable:
		return "Provider SEO tidak dapat dihubungi sekarang."
	case KindAuth:
		return "Autentikasi provider DA/PA ditolak. Pastikan akun RapidAPI berlangganan \"Bulk DA PA Checker Semrush\" (" + ProviderLabel + "), lalu simpan ulang key-nya di Konfigurasi."
	case KindQuota:
		return "Kuota permintaan penyedia DA/PA habis untuk periode ini, jadi kolom DA tidak bisa diisi. Naikkan paket API-nya atau tunggu kuota reset."
	case KindRateLimited:
		return "Provider DA/PA sedang rate-limited. Coba lagi nanti."
	case KindNotFound:
		return "Domain media tidak ditemukan provider."
	case KindMalformed:
		if e.Message != "" {
			return e.Message
		}
		return "Respons provider SEO tidak dapat dibaca (malformed)."
	default:
		return "Provider SEO sedang tidak tersedia."
	}
}

var daKeys = []string{"da", "domain_authority", "domainauthority", "authority", "authority_score", "authorityscore", "mozda", "domain_rating", "domainrating"}
var paKeys = []string{"pa", "page_authority", "pageauthority", "page_rating", "pagerating", "mozpa"}
var spamKeys = []string{"spam_score", "spamscore", "spam", "mozspam"}

// NormalizeDomain: buang skema/path/port, lowercase, tanpa "www.".
// Mengembalikan "" bila bentuknya bukan domain yang wajar.
func NormalizeDomain(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "http://")
	if i := strings.IndexAny(v, "/?#"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, ":"); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimPrefix(v, "www.")
	v = strings.TrimSuffix(v, ".")
	if v == "" || strings.ContainsAny(v, " \t\n") {
		return ""
	}
	// Harus mengandung minimal satu titik dan hanya karakter host yang sah.
	if !strings.Contains(v, ".") || !hostRe.MatchString(v) {
		return ""
	}
	return v
}

var hostRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

// FetchMany: cek DA/PA untuk BANYAK domain dalam satu panggilan (bulk).
// Domain dinormalisasi, dideduplikasi, dan yang tidak valid dilewati.
func FetchMany(ctx context.Context, domains []string, apiKey string, cfg Config) (*Result, *Error) {
	clean := make([]string, 0, len(domains))
	seen := map[string]bool{}
	for _, raw := range domains {
		d := NormalizeDomain(raw)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		clean = append(clean, d)
	}
	if len(clean) == 0 {
		return nil, &Error{Kind: KindNotFound, Message: "Tidak ada domain yang valid untuk diperiksa."}
	}
	if cfg.Host == "" {
		cfg = DefaultConfig()
	}

	// multipart/form-data dibuat dengan builder standar (boundary otomatis).
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField(cfg.BodyKey, strings.Join(clean, ",")); err != nil {
		return nil, &Error{Kind: KindMalformed, Message: "Gagal menyusun permintaan provider."}
	}
	if err := writer.Close(); err != nil {
		return nil, &Error{Kind: KindMalformed, Message: "Gagal menyusun permintaan provider."}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.URL(), &buf)
	if err != nil {
		return nil, &Error{Kind: KindUnreachable, Message: err.Error()}
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", writer.FormDataContentType())
	req.Header.Set("x-rapidapi-host", cfg.Host)
	req.Header.Set("x-rapidapi-key", apiKey)

	resp, err := security.SafeClient(20 * time.Second).Do(req)
	if err != nil {
		return nil, &Error{Kind: KindUnreachable, Message: "Provider SEO tidak dapat dihubungi sekarang."}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	text := string(raw)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch resp.StatusCode {
		case 401, 403:
			return nil, &Error{Kind: KindAuth, Status: resp.StatusCode, Body: text}
		case 429:
			if strings.Contains(strings.ToLower(text), "quota") {
				return nil, &Error{Kind: KindQuota, Status: resp.StatusCode, Body: text}
			}
			return nil, &Error{Kind: KindRateLimited, Status: resp.StatusCode, Body: text}
		case 404:
			return nil, &Error{Kind: KindNotFound, Status: resp.StatusCode, Body: text}
		default:
			return nil, &Error{Kind: KindBadStatus, Status: resp.StatusCode, Body: text}
		}
	}
	if strings.TrimSpace(text) == "" {
		return nil, &Error{Kind: KindMalformed, Message: "Respons provider SEO kosong (malformed)."}
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, &Error{Kind: KindMalformed, Message: "Respons provider SEO tidak dapat dibaca (malformed).", Body: truncate(text, 400)}
	}

	result := &Result{Metrics: map[string]*Metrics{}, Raw: truncate(text, 4000)}
	for _, d := range clean {
		da, pa, spam := findMetrics(parsed, d)
		if da == nil || pa == nil {
			result.Missing = append(result.Missing, d)
			continue
		}
		result.Metrics[d] = &Metrics{Domain: d, DA: *da, PA: *pa, Spam: spam}
	}
	if len(result.Metrics) == 0 {
		return nil, &Error{Kind: KindMalformed, Message: "Provider SEO tidak mengembalikan DA/PA.", Body: truncate(text, 400)}
	}
	return result, nil
}

// Fetch: cek DA/PA satu domain (pembungkus FetchMany).
func Fetch(ctx context.Context, domain, apiKey string, cfg Config) (*Metrics, *Error) {
	res, seoErr := FetchMany(ctx, []string{domain}, apiKey, cfg)
	if seoErr != nil {
		return nil, seoErr
	}
	for _, m := range res.Metrics {
		return m, nil
	}
	return nil, &Error{Kind: KindMalformed, Message: "Provider SEO tidak mengembalikan DA/PA untuk domain ini.", Body: truncate(res.Raw, 400)}
}

// findMetrics: cari objek yang memuat pasangan DA/PA di struktur JSON apa pun.
// API bulk bisa membalas array, map per-domain, atau objek tunggal.
func findMetrics(root any, domain string) (*float64, *float64, *float64) {
	var bestDA, bestPA *float64
	var spam *float64
	var walk func(node any)
	walk = func(node any) {
		if bestDA != nil && bestPA != nil {
			return
		}
		switch v := node.(type) {
		case map[string]any:
			// Utamakan objek milik domain yang diminta bila ada penandanya.
			if d, _ := v["domain"].(string); d != "" && !strings.EqualFold(strings.TrimSpace(d), domain) {
				// tetap telusuri anaknya, tapi jangan ambil angka dari objek domain lain
				for _, child := range v {
					walk(child)
				}
				return
			}
			da := pickNumber(v, daKeys)
			pa := pickNumber(v, paKeys)
			if da != nil && pa != nil {
				// Clamp di sini, bukan di pemakai: kolom DB punya CHECK 0..100 dan
				// beberapa penyedia mengirim -1 untuk "tidak diketahui".
				// Nilai negatif jadi nil → domain dianggap "tidak lengkap" dan
				// masuk daftar Missing, bukan tersimpan sebagai 0 palsu.
				cda, cpa := clampScore(da), clampScore(pa)
				if cda == nil || cpa == nil {
					return
				}
				bestDA, bestPA = cda, cpa
				spam = clampScore(pickNumber(v, spamKeys))
				return
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			// Array: cari elemen dengan domain yang cocok dulu.
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					if d, _ := m["domain"].(string); d != "" && strings.EqualFold(strings.TrimSpace(d), domain) {
						walk(m)
						if bestDA != nil && bestPA != nil {
							return
						}
					}
				}
			}
			for _, item := range v {
				walk(item)
			}
		}
	}
	// Map ber-key domain (mis. {"example.com": {...}}): utamakan key itu.
	if m, ok := root.(map[string]any); ok {
		for k, v := range m {
			if strings.EqualFold(strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(k)), "www."), "/"), domain) {
				walk(v)
				if bestDA != nil && bestPA != nil {
					return bestDA, bestPA, spam
				}
			}
		}
	}
	walk(root)
	return bestDA, bestPA, spam
}

// clampScore: paksa nilai ke 0..100. Provider kadang mengirim -1 (tanda
// "tidak diketahui"), persen di atas 100, atau angka pecahan; kolom DB
// (da/pa/spam_score smallint) punya CHECK 0..100 sehingga nilai mentah
// langsung menggagalkan INSERT dengan "violates check constraint".
// Nilai negatif dianggap "tidak ada data" → nil, bukan 0: 0 berarti "spam
// sangat rendah" (kabar baik), sedangkan -1 berarti "tidak diukur".
func clampScore(v *float64) *float64 {
	if v == nil {
		return nil
	}
	f := *v
	if f < 0 {
		return nil
	}
	if f > 100 {
		f = 100
	}
	f = math.Round(f)
	out := f
	return &out
}

func pickNumber(m map[string]any, keys []string) *float64 {
	lower := make(map[string]any, len(m))
	for k, v := range m {
		lower[strings.ToLower(strings.TrimSpace(k))] = v
	}
	for _, key := range keys {
		if v, ok := lower[key]; ok {
			if f, ok := toNumber(v); ok {
				return &f
			}
		}
	}
	return nil
}

func toNumber(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
