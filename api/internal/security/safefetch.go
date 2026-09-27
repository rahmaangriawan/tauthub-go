// Port lib/security/safe-fetch.ts — anti-SSRF: resolve DNS dulu, blok IP
// privat/loopback/CGNAT/link-local/cloud metadata (v4+v6), limit redirect 5.
package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// SafeFetch client: dialer dengan Control yang memeriksa setiap IP hasil
// resolve (mencegah rebinding ke IP privat), redirect maks 5.
func SafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
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
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
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
