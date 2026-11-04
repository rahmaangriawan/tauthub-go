package auth

import (
	"bufio"
	"context"
	_ "embed"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Daftar domain temp-mail. Dua sumber:
//   - bawaan: disposable_domains.txt (disposable-email-domains, ~10 ribu domain),
//     dimuat sekali dari embed;
//   - tabel blocked_email_domains yang dikelola admin (cache 60 detik).
// Cocok persis atau suffix: "x.tempmail.io" kena bila "tempmail.io" terdaftar.
// Domain di disposableAllowlist tidak pernah dianggap disposable, apa pun isi
// daftarnya, supaya subdomain sah tidak ikut tertangkap.

//go:embed disposable_domains.txt
var disposableDomainsRaw string

var (
	disposableOnce    sync.Once
	disposableBuiltin map[string]struct{}

	disposableAllowlist = map[string]struct{}{
		"gmail.com": {}, "googlemail.com": {}, "tauthub.com": {}, "app.tauthub.com": {},
	}

	blockedDomainsMu    sync.Mutex
	blockedDomainsCache map[string]struct{}
	blockedDomainsExp   time.Time
)

func builtinDisposable() map[string]struct{} {
	disposableOnce.Do(func() {
		m := make(map[string]struct{}, 10500)
		sc := bufio.NewScanner(strings.NewReader(disposableDomainsRaw))
		for sc.Scan() {
			line := strings.ToLower(strings.TrimSpace(sc.Text()))
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			m[line] = struct{}{}
		}
		// Daftar lama yang hardcoded ikut dipertahankan.
		for d := range disposableDomains {
			m[d] = struct{}{}
		}
		disposableBuiltin = m
	})
	return disposableBuiltin
}

// BuiltinDisposableCount: jumlah domain bawaan (ditampilkan di panel admin).
func BuiltinDisposableCount() int { return len(builtinDisposable()) }

// adminBlockedDomains: isi tabel blocked_email_domains, cache 60 detik.
func adminBlockedDomains(ctx context.Context, pool *pgxpool.Pool) map[string]struct{} {
	blockedDomainsMu.Lock()
	defer blockedDomainsMu.Unlock()
	if blockedDomainsCache != nil && blockedDomainsExp.After(time.Now()) {
		return blockedDomainsCache
	}
	m := map[string]struct{}{}
	if pool != nil {
		rows, err := pool.Query(ctx, "SELECT domain FROM blocked_email_domains")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var d string
				if rows.Scan(&d) == nil {
					m[strings.ToLower(strings.TrimSpace(d))] = struct{}{}
				}
			}
		}
	}
	blockedDomainsCache, blockedDomainsExp = m, time.Now().Add(60*time.Second)
	return m
}

// InvalidateBlockedDomainsCache: dipanggil setelah admin mengubah tabel.
func InvalidateBlockedDomainsCache() {
	blockedDomainsMu.Lock()
	blockedDomainsCache = nil
	blockedDomainsMu.Unlock()
}

func domainOrSuffixIn(domain string, set map[string]struct{}) bool {
	d := domain
	for d != "" {
		if _, ok := set[d]; ok {
			return true
		}
		i := strings.Index(d, ".")
		if i < 0 {
			return false
		}
		d = d[i+1:]
	}
	return false
}

// IsDisposableDomain: domain (atau induknya) ada di daftar bawaan atau tabel admin.
func IsDisposableDomain(ctx context.Context, pool *pgxpool.Pool, domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return false
	}
	if _, ok := disposableAllowlist[domain]; ok {
		return false
	}
	if domainOrSuffixIn(domain, builtinDisposable()) {
		return true
	}
	return domainOrSuffixIn(domain, adminBlockedDomains(ctx, pool))
}

// AnalyzeEmailCtx: AnalyzeEmail dengan IsDisposable dari kedua sumber daftar.
func AnalyzeEmailCtx(ctx context.Context, pool *pgxpool.Pool, email string) EmailAnalysis {
	info := AnalyzeEmail(email)
	info.IsDisposable = IsDisposableDomain(ctx, pool, info.Domain)
	return info
}
