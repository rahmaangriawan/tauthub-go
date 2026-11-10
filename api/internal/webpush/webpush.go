// Package webpush: pengiriman Web Push (RFC 8030) dengan enkripsi payload
// aes128gcm (RFC 8291/8188) dan otorisasi VAPID (RFC 8292), memakai
// stdlib saja: crypto/ecdh, crypto/hkdf, crypto/aes, dan JWT ES256 manual.
//
// Tidak ada dependensi pihak ketiga di jalur ini secara sengaja — kode ini
// memegang kunci privat VAPID.
package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Subscription: data langganan dari PushManager.subscribe() di browser.
type Subscription struct {
	Endpoint string
	P256dh   string // base64url, kunci publik ECDH klien (65 byte uncompressed)
	Auth     string // base64url, 16 byte auth secret
}

// Options: parameter pengiriman satu pesan.
type Options struct {
	TTL     int    // detik; 0 → default 24 jam
	Urgency string // "very-low" | "low" | "normal" | "high"; kosong → normal
	Topic   string // opsional; push server mengganti pesan bertopik sama yang belum terkirim
}

// Result: hasil pengiriman.
type Result struct {
	StatusCode int
	// Gone true bila endpoint sudah tidak valid (404/410): langganan wajib dihapus.
	Gone bool
	// RetryAfter dari server push, bila ada (mis. 429/5xx).
	RetryAfter time.Duration
	Body       string
}

// Client: pengirim Web Push dengan kunci VAPID tetap.
type Client struct {
	priv    *ecdsa.PrivateKey
	pubB64  string // base64url raw uncompressed (65 byte) — untuk header vapid k=
	subject string // mailto: atau https:
	http    *http.Client

	mu     sync.Mutex
	tokens map[string]cachedToken // per origin push server
}

type cachedToken struct {
	value string
	exp   time.Time
}

// ErrInvalidKey: kunci VAPID tidak valid.
var ErrInvalidKey = errors.New("webpush: kunci VAPID tidak valid")

// GenerateVAPIDKeys menghasilkan pasangan kunci baru (base64url tanpa padding):
// privat = 32 byte skalar, publik = 65 byte titik uncompressed.
func GenerateVAPIDKeys() (privateB64, publicB64 string, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("webpush: generate kunci: %w", err)
	}
	d := priv.D.FillBytes(make([]byte, 32))
	pub := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck // format uncompressed wajib untuk VAPID
	return b64u(d), b64u(pub), nil
}

// NewClient membuat pengirim dari kunci privat VAPID base64url (32 byte) dan
// subject (mailto:ops@domain atau https://domain).
func NewClient(privateB64, subject string, httpClient *http.Client) (*Client, error) {
	d, err := b64uDecode(privateB64)
	if err != nil || len(d) != 32 {
		return nil, ErrInvalidKey
	}
	curve := elliptic.P256()
	x, y := curve.ScalarBaseMult(d)
	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y},
		D:         new(big.Int).SetBytes(d),
	}
	if !curve.IsOnCurve(x, y) {
		return nil, ErrInvalidKey
	}
	subject = strings.TrimSpace(subject)
	if !strings.HasPrefix(subject, "mailto:") && !strings.HasPrefix(subject, "https://") {
		return nil, fmt.Errorf("webpush: subject VAPID harus mailto: atau https: (dapat %q)", subject)
	}
	if httpClient == nil {
		httpClient = security.SafeClient(15 * time.Second)
	}
	// Endpoint berasal dari pengguna: redirect tidak pernah diikuti.
	hc := *httpClient
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient = &hc
	pub := elliptic.Marshal(curve, x, y) //nolint:staticcheck
	return &Client{priv: priv, pubB64: b64u(pub), subject: subject, http: httpClient, tokens: map[string]cachedToken{}}, nil
}

// PublicKey: kunci publik VAPID base64url (yang dipakai applicationServerKey di browser).
func (c *Client) PublicKey() string { return c.pubB64 }

// Send mengenkripsi payload dan mengirimnya ke endpoint langganan.
// Error hanya untuk kegagalan lokal/jaringan; status HTTP non-2xx dikembalikan
// lewat Result supaya pemanggil bisa membedakan 404/410 (hapus) dari 429/5xx (retry).
func (c *Client) Send(ctx context.Context, sub Subscription, payload []byte, opt Options) (*Result, error) {
	if len(payload) > 4000 {
		return nil, fmt.Errorf("webpush: payload %d byte melebihi batas 4000", len(payload))
	}
	endpoint, err := url.Parse(strings.TrimSpace(sub.Endpoint))
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return nil, fmt.Errorf("webpush: endpoint tidak valid")
	}

	body, err := encrypt(sub, payload)
	if err != nil {
		return nil, err
	}

	origin := endpoint.Scheme + "://" + endpoint.Host
	token, err := c.vapidToken(origin)
	if err != nil {
		return nil, err
	}

	ttl := opt.TTL
	if ttl <= 0 {
		ttl = 24 * 3600
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", strconv.Itoa(ttl))
	req.Header.Set("Authorization", "vapid t="+token+", k="+c.pubB64)
	if opt.Urgency != "" {
		req.Header.Set("Urgency", opt.Urgency)
	}
	if opt.Topic != "" {
		req.Header.Set("Topic", opt.Topic)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	res := &Result{StatusCode: resp.StatusCode, Body: string(raw)}
	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusGone:
		res.Gone = true
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			res.RetryAfter = time.Duration(secs) * time.Second
		} else if t, err := http.ParseTime(ra); err == nil {
			if d := time.Until(t); d > 0 {
				res.RetryAfter = d
			}
		}
	}
	return res, nil
}

// vapidToken: JWT ES256 per origin push server, di-cache 12 jam (batas spec 24 jam).
func (c *Client) vapidToken(origin string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.tokens[origin]; ok && time.Now().Before(t.exp.Add(-5*time.Minute)) {
		return t.value, nil
	}
	exp := time.Now().Add(12 * time.Hour)
	header := b64u([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, err := json.Marshal(map[string]any{
		"aud": origin,
		"exp": exp.Unix(),
		"sub": c.subject,
	})
	if err != nil {
		return "", err
	}
	signing := header + "." + b64u(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, c.priv, digest[:])
	if err != nil {
		return "", fmt.Errorf("webpush: tanda tangan VAPID: %w", err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	token := signing + "." + b64u(sig)
	c.tokens[origin] = cachedToken{value: token, exp: exp}
	return token, nil
}

// encrypt: RFC 8291 (aes128gcm, satu record) dengan kunci server dan salt acak.
func encrypt(sub Subscription, payload []byte) ([]byte, error) {
	asPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return encryptWith(sub, payload, asPriv, salt)
}

// encryptWith: inti RFC 8291 dengan kunci server dan salt yang diberikan —
// dipisah supaya bisa diverifikasi terhadap vektor uji resmi (Appendix A).
// Keluaran = header 86 byte (salt 16 | rs 4 | idlen 1 | pubkey 65) +
// ciphertext(payload | 0x02) + tag 16.
func encryptWith(sub Subscription, payload []byte, asPriv *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	uaPubRaw, err := b64uDecode(sub.P256dh)
	if err != nil || len(uaPubRaw) != 65 {
		return nil, fmt.Errorf("webpush: p256dh langganan tidak valid")
	}
	authSecret, err := b64uDecode(sub.Auth)
	if err != nil || len(authSecret) != 16 {
		return nil, fmt.Errorf("webpush: auth langganan tidak valid")
	}
	if len(salt) != 16 {
		return nil, fmt.Errorf("webpush: salt harus 16 byte")
	}
	uaPub, err := ecdh.P256().NewPublicKey(uaPubRaw)
	if err != nil {
		return nil, fmt.Errorf("webpush: p256dh bukan titik kurva valid")
	}
	asPubRaw := asPriv.PublicKey().Bytes()
	shared, err := asPriv.ECDH(uaPub)
	if err != nil {
		return nil, err
	}

	// IKM = HKDF(auth_secret, ecdh_secret, "WebPush: info\0" | ua_pub | as_pub, 32)
	info := append([]byte("WebPush: info\x00"), uaPubRaw...)
	info = append(info, asPubRaw...)
	ikm, err := hkdf.Key(sha256.New, shared, authSecret, string(info), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// Satu record: plaintext | 0x02 (delimiter record terakhir).
	plain := append(append([]byte{}, payload...), 0x02)
	// rs = 4096 (nilai default RFC 8188). Harus >= len(plain)+16 dan <= 2^36-31.
	const rs = 4096
	cipherText := gcm.Seal(nil, nonce, plain, nil)

	out := make([]byte, 0, 86+len(cipherText))
	out = append(out, salt...)
	out = binary.BigEndian.AppendUint32(out, rs)
	out = append(out, byte(len(asPubRaw)))
	out = append(out, asPubRaw...)
	out = append(out, cipherText...)
	return out, nil
}

func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func b64uDecode(s string) ([]byte, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	return base64.RawURLEncoding.DecodeString(s)
}
