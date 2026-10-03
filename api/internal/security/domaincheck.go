// domaincheck.go: pemeriksaan kepemilikan domain yang dipakai bersama tiga
// pemanggil (admin recheck, job domain_verify, verifikasi mitra) — fetch
// halaman (HTTPS, anti-SSRF, UA browser) lalu cari token verifikasi: meta tag
// di HTML, atau fallback DNS TXT di _tauthub-verify.<domain> (jalur alternatif
// untuk situs yang WAF-nya menolak IP datacenter). 403 diklasifikasikan
// blocked_by_waf, BUKAN unreachable — situsnya hidup, hanya akses bot/server
// yang ditolak, jadi tidak boleh memicu pencabutan verifikasi.
package security

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// DomainCheckOutcome: hasil pemeriksaan satu domain.
type DomainCheckOutcome struct {
	Result     string // "verified" | "blocked_by_waf" | "unreachable" | "token_missing" | "token_not_found"
	Stage      string // "dns" | "fetch" | "http" | "waf" | "token" | "txt" | "ok"
	Message    string // alasan manusiawi (disimpan ke domain_last_error)
	FoundToken string // fragmen meta domain-verify yang cocok tapi tidak sama (token_missing)
	HTTP2xx    bool   // halaman benar-benar terbaca (untuk keputusan health di pemanggil)
	Trace      []string
}

// TXTRecordName: nama record TXT verifikasi untuk host:
// _tauthub-verify.<host> (host tanpa https:// dan tanpa path).
func TXTRecordName(host string) string {
	return "_tauthub-verify." + strings.ToLower(strings.Trim(strings.TrimSpace(host), "."))
}

// VerifyDomainOwnership: GET target (wajib https://), klasifikasikan hasil,
// cari token di HTML atau di TXT record. Token kosong → selalu gagal verifikasi
// tapi stage tetap informatif (masih berguna sebagai cek kesehatan situs).
func VerifyDomainOwnership(ctx context.Context, target, token string) DomainCheckOutcome {
	trace := []string{"GET " + target}
	resp, err := Get(ctx, target, map[string]string{
		"user-agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36",
		"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"accept-language": "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7",
	}, 30*time.Second)
	if err != nil {
		code := DomainErrCode(err)
		stage := "fetch"
		if code == "ENOTFOUND" || code == "EAI_AGAIN" {
			stage = "dns"
		}
		trace = append(trace, "error: "+code+": "+err.Error())
		if stage == "dns" {
			// Domain tidak resolve — TXT lookup pasti gagal juga, lewati.
			return DomainCheckOutcome{Result: "unreachable", Stage: "dns", Message: err.Error(), Trace: trace}
		}
		if txtVerified(ctx, target, token, &trace) {
			return DomainCheckOutcome{Result: "verified", Stage: "txt", Message: "Token ditemukan di TXT record", Trace: trace}
		}
		return DomainCheckOutcome{Result: "unreachable", Stage: stage, Message: err.Error(), Trace: trace}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 500_000))
	resp.Body.Close()
	status := resp.StatusCode
	trace = append(trace, "HTTP "+fmt.Sprintf("%d", status))

	if status < 200 || status >= 300 {
		if status == 403 {
			trace = append(trace, "Diblokir WAF mitra (403) — situs hidup, akses dari IP server ditolak")
			if txtVerified(ctx, target, token, &trace) {
				return DomainCheckOutcome{Result: "verified", Stage: "txt", Message: "Token ditemukan di TXT record", Trace: trace}
			}
			return DomainCheckOutcome{
				Result: "blocked_by_waf", Stage: "waf",
				Message: fmt.Sprintf("HTTP 403 %s — diblokir WAF mitra", strings.TrimSpace(resp.Status)),
				Trace:   trace,
			}
		}
		if txtVerified(ctx, target, token, &trace) {
			return DomainCheckOutcome{Result: "verified", Stage: "txt", Message: "Token ditemukan di TXT record", Trace: trace}
		}
		return DomainCheckOutcome{
			Result: "unreachable", Stage: "http",
			Message: fmt.Sprintf("HTTP %d %s", status, strings.TrimSpace(resp.Status)),
			Trace:   trace,
		}
	}

	out2xx := DomainCheckOutcome{HTTP2xx: true, Trace: trace}
	if token != "" && strings.Contains(strings.ToLower(string(body)), strings.ToLower(token)) {
		out2xx.Result, out2xx.Stage, out2xx.Message = "verified", "ok", ""
		return out2xx
	}
	// Halaman terbaca tapi token tidak ketemu — TXT adalah jalur cadangan.
	if txtVerified(ctx, target, token, &trace) {
		return DomainCheckOutcome{Result: "verified", Stage: "txt", Message: "Token ditemukan di TXT record", Trace: trace}
	}
	if token != "" {
		if idx := strings.Index(strings.ToLower(string(body)), "domain-verify"); idx >= 0 {
			end := idx + 80
			if end > len(body) {
				end = len(body)
			}
			return DomainCheckOutcome{
				Result: "token_missing", Stage: "token", Message: "token_mismatch",
				FoundToken: string(body[idx:end]), Trace: trace,
			}
		}
	}
	out2xx.Result, out2xx.Stage, out2xx.Message = "token_not_found", "token", "Token verifikasi tidak ditemukan di halaman."
	return out2xx
}

// txtVerified: cari token di TXT _tauthub-verify.<host>, lalu varian apex
// (tanpa www.) — mitra biasanya menambah record di domain utama. Return true
// bila ada record yang cocok; jejak selalu dicatat.
func txtVerified(ctx context.Context, target, token string, trace *[]string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	candidates := []string{host}
	if apex := strings.TrimPrefix(host, "www."); apex != host {
		candidates = append(candidates, apex)
	}
	for _, c := range candidates {
		name := TXTRecordName(c)
		txts, lookupErr := safeResolver.LookupTXT(ctx, name)
		if lookupErr != nil {
			*trace = append(*trace, "TXT "+name+" gagal: "+lookupErr.Error())
			continue
		}
		for _, t := range txts {
			if strings.EqualFold(strings.TrimSpace(strings.Trim(t, `"`)), strings.TrimSpace(token)) {
				*trace = append(*trace, "TXT "+name+" → token cocok")
				return true
			}
		}
		*trace = append(*trace, fmt.Sprintf("TXT %s → %d record, token tidak cocok", name, len(txts)))
	}
	return false
}

// DomainErrCode: kode errno jaringan dari error (parity cause.code di Node).
func DomainErrCode(err error) string {
	msg := err.Error()
	for _, code := range []string{"ENOTFOUND", "EAI_AGAIN", "ECONNREFUSED", "ECONNRESET", "ETIMEDOUT", "EHOSTUNREACH", "ENETUNREACH", "CERT_HAS_EXPIRED"} {
		if strings.Contains(msg, code) {
			return code
		}
	}
	return ""
}
