// Port lib/security/safe-fetch.ts — anti-SSRF: resolve DNS dulu, blok IP
// privat/loopback/CGNAT/link-local/cloud metadata (v4+v6), limit redirect 5.
package security

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

// safeResolver: resolver DNS yang mengutamakan public DNS (1.1.1.1, 8.8.8.8)
// agar resolusi domain publik (media partner/WordPress/webhook) tidak terbajak
// oleh DNS ISP lokal (mis. blockpage XL Axiata / Internet Positif), dengan fallback
// ke DNS sistem bawaan bila koneksi UDP ke public DNS gagal.
var safeResolver = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 3 * time.Second}
		if custom := strings.TrimSpace(os.Getenv("DNS_SERVER")); custom != "" {
			if !strings.Contains(custom, ":") {
				custom += ":53"
			}
			conn, err := d.DialContext(ctx, "udp", custom)
			if err == nil {
				return conn, nil
			}
		}
		// Coba Cloudflare DNS 1.1.1.1:53
		conn, err := d.DialContext(ctx, "udp", "1.1.1.1:53")
		if err == nil {
			return conn, nil
		}
		// Fallback Google DNS 8.8.8.8:53
		conn, err = d.DialContext(ctx, "udp", "8.8.8.8:53")
		if err == nil {
			return conn, nil
		}
		// Fallback ke resolver sistem default
		return d.DialContext(ctx, network, address)
	},
}

// safeSharedTransport: SATU transport untuk semua SafeClient. Sebelumnya tiap
// panggilan SafeClient membuat transport baru → keep-alive tidak pernah
// terpakai ulang dan tiap request membayar handshake TLS lagi (LLM, WordPress,
// cek link). Dial timeout tetap dibatasi; batas total per request dipegang
// http.Client.Timeout milik pemanggil.
var (
	safeTransportOnce   sync.Once
	safeSharedTransport *http.Transport
)

func sharedSafeTransport() *http.Transport {
	safeTransportOnce.Do(func() {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  safeResolver,
			Control: func(_ string, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				ip := net.ParseIP(host)
				if ip == nil {
					return fmt.Errorf("alamat tidak valid")
				}
				if isBlockedIP(ip) {
					return fmt.Errorf("diblokir safe-fetch: alamat privat/loopback")
				}
				return nil
			},
		}
		safeSharedTransport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Coba tcp4 terlebih dahulu agar koneksi ke CDN/host publik tidak terhambat
				// rute IPv6 yang reset/timeout di ISP lokal (mis. rute IPv6 Cloudflare).
				conn, err := dialer.DialContext(ctx, "tcp4", addr)
				if err == nil {
					return conn, nil
				}
				return dialer.DialContext(ctx, network, addr)
			},
			// Tuning mengikuti goperf.dev: default Go hanya menyimpan 2 idle conn
			// per host — terlalu kecil untuk backend yang memanggil host yang sama
			// terus-menerus (LLM/Google/WordPress).
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   25,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	})
	return safeSharedTransport
}

// SafeClient: klien dengan guard anti-SSRF (IP privat/loopback ditolak saat
// dial, termasuk setelah DNS rebinding) + redirect maks 5. Transport dipakai
// bersama supaya koneksi tetap hidup (keep-alive) antar panggilan.
func SafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: sharedSafeTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("terlalu banyak redirect")
			}
			return nil
		},
	}
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// CGNAT 100.64.0.0/10
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] < 128 {
			return true
		}
		// cloud metadata 169.254.169.254 sudah tercakup link-local, eksplisit:
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}

// Get: GET dengan timeout + header (dipakai crawler/domain verification).
// SafeClientInsecure: klien yang sama (anti-SSRF tetap aktif) tetapi TIDAK
// memvalidasi sertifikat TLS. Dipakai hanya untuk memastikan situs benar-benar
// merespons saat sertifikatnya bermasalah — bukan untuk verifikasi token,
// supaya jalur verifikasi tetap butuh kanal tepercaya (atau TXT).
func SafeClientInsecure(timeout time.Duration) *http.Client {
	c := SafeClient(timeout)
	if tr, ok := c.Transport.(*http.Transport); ok {
		clone := tr.Clone()
		clone.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // probe keterjangkauan saja
		c.Transport = clone
	}
	return c
}

func Get(ctx context.Context, url string, headers map[string]string, timeout time.Duration) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return SafeClient(timeout).Do(req)
}
