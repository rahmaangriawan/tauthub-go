package partner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Outbound HTTP untuk adonan integrasi mitra (custom-web, Tulisio adapter,
// crawler test, RSS verify, WP draft). Semua lewat SafeClient supaya guard SSRF
// dan aturan HTTPS produksi berlaku sama seperti app lama.

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

func readBodyLimited(resp *http.Response, max int) (string, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(max)))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ---------- custom web protocol ----------

// protocolURL: parity protocolUrl — pastikan path target Tauthub protocol.
func protocolURL(endpointURL, action string) string {
	base := strings.TrimRight(endpointURL, "/")
	for _, suffix := range []string{"/v1/tauthub/ping", "/v1/tulisio/ping"} {
		if strings.HasSuffix(base, suffix) {
			if action == "ping" {
				return base
			}
			return strings.TrimSuffix(base, "/ping") + "/" + action
		}
	}
	return base + "/v1/tauthub/" + action
}

// pingCustomWeb: parity pingCustomWeb — coba endpoint kanonik lalu legacy.
func (d *Deps) pingCustomWeb(r *http.Request, endpointURL, apiKey string) (bool, string) {
	canonical := protocolURL(endpointURL, "ping")
	candidates := []string{canonical}
	if legacy := strings.Replace(canonical, "/v1/tauthub/", "/v1/tulisio/", 1); legacy != canonical {
		candidates = append(candidates, legacy)
	}
	var lastMessage string
	for i, target := range candidates {
		ok, message := d.pingCustomWebOnce(r.Context(), target, apiKey)
		if ok {
			return true, message
		}
		lastMessage = message
		if i == 0 && len(candidates) == 1 {
			break
		}
	}
	return false, lastMessage
}

func (d *Deps) pingCustomWebOnce(ctx context.Context, target, apiKey string) (bool, string) {
	requestID := "ping_" + uuid.NewString()
	body := marshalJSON(map[string]any{
		"protocol_version": "1.0",
		"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
		"request_id":       requestID,
	})
	headers := map[string]string{
		"Content-Type":               "application/json",
		"Authorization":              "Bearer " + apiKey,
		"X-Tauthub-Protocol-Version": "1.0",
		"X-Tulisio-Protocol-Version": "1.0",
		"X-Tauthub-Request-Id":       requestID,
		"X-Tulisio-Request-Id":       requestID,
	}
	client := security.SafeClient(10 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewBufferString(body))
	if err != nil {
		return false, "Endpoint tidak dapat diakses."
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "timeout") ||
			strings.Contains(strings.ToLower(err.Error()), "deadline") {
			return false, "Endpoint tidak dapat dijangkau (timeout)."
		}
		return false, "Endpoint tidak dapat diakses."
	}
	text, _ := readBodyLimited(resp, 64*1024)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Sprintf("Endpoint merespons HTTP %d", resp.StatusCode)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return false, "Response tidak sesuai Tauthub Publisher Protocol."
	}
	if parsed["success"] != true {
		return false, "Response tidak sesuai Tauthub Publisher Protocol."
	}
	service, _ := parsed["service"].(string)
	if service != "tauthub" && service != "tulisio" {
		return false, "Response tidak sesuai Tauthub Publisher Protocol."
	}
	return true, "Endpoint terhubung"
}

// ---------- Tulisio publishing adapter ----------

// pingTulisioAdapter: parity test-publishing-api — signature HMAC
// sha256(secret, "<unix-ts>.<raw-body>") + Bearer api key.
func (d *Deps) pingTulisioAdapter(r *http.Request, adapterURL, apiKey, secret, mediaID string) map[string]any {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	body := marshalJSON(map[string]any{
		"event":     "ping",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"sender":    "tulisio",
		"media_id":  mediaID,
	})
	signature := hmacSHA256Hex([]byte(secret), timestamp+"."+body)
	headers := map[string]string{
		"Content-Type":              "application/json",
		"Authorization":             "Bearer " + apiKey,
		"X-Tulisio-Timestamp":       timestamp,
		"X-Tulisio-Signature":       signature,
		"X-Tulisio-Idempotency-Key": "ping:" + mediaID,
	}
	client := security.SafeClient(15 * time.Second)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, adapterURL, bytes.NewBufferString(body))
	if err != nil {
		return map[string]any{"ok": false, "message": "Adapter tidak dapat dijangkau: alamat tidak valid"}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		message := truncateRunes(err.Error(), 200)
		if message == "" {
			message = "network error"
		}
		return map[string]any{"ok": false, "message": "Adapter tidak dapat dijangkau: " + message}
	}
	text, _ := readBodyLimited(resp, 64*1024)
	var parsed map[string]any
	_ = json.Unmarshal([]byte(text), &parsed)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || parsed["accepted"] != true {
		detail := ""
		if msg, ok := parsed["message"].(string); ok {
			detail = msg
		} else if strings.TrimSpace(text) != "" {
			detail = truncateRunes(text, 200)
		} else {
			detail = "tidak ada body respons"
		}
		return map[string]any{
			"ok":      false,
			"message": fmt.Sprintf("Adapter merespons status %d: %s", resp.StatusCode, detail),
		}
	}
	return map[string]any{
		"ok":      true,
		"status":  resp.StatusCode,
		"message": "Koneksi berhasil — adapter memverifikasi signature dan merespons accepted:true.",
	}
}

// ---------- WordPress draft (mode manual mitra) ----------

type wpConnection struct {
	ID       string
	BaseURL  string
	Username string
	Secret   string
}

// resolveWPConnection: parity resolveConnection di partner publications/draft.
func (d *Deps) resolveWPConnection(r *http.Request, partnerID, connectionID string) (*wpConnection, error) {
	var row rowMap
	var err error
	if connectionID != "" {
		row, err = d.queryRow(r.Context(),
			"SELECT * FROM cms_connections WHERE id = $1 AND partner_id = $2 AND provider = 'wordpress' AND status = 'active' LIMIT 1",
			connectionID, partnerID)
		if err != nil {
			return nil, err
		}
	}
	if row == nil {
		row, err = d.queryRow(r.Context(),
			"SELECT * FROM cms_connections WHERE partner_id = $1 AND provider = 'wordpress' AND status = 'active' ORDER BY created_at DESC LIMIT 1",
			partnerID)
		if err != nil {
			return nil, err
		}
	}
	if row == nil {
		return nil, errConflict("Tidak ada koneksi WordPress aktif untuk media ini.")
	}
	conn := &wpConnection{
		ID:       row.strOr("id", ""),
		BaseURL:  row.strOr("base_url", ""),
		Username: row.strOr("username", ""),
	}
	secret, err := d.connectionSecret(row)
	if err != nil {
		return nil, err
	}
	conn.Secret = secret
	return conn, nil
}

// connectionSecret: parity connectionSecret — v3 per-entitas bila enc_key_ver >= 1,
// else v2 global (wordpress pakai APP_SECRET).
func (d *Deps) connectionSecret(row rowMap) (string, error) {
	sealed := row.strOr("sealed_secret", "")
	if sealed == "" {
		return "", nil
	}
	if row.intOr("enc_key_ver", 0) >= 1 {
		return d.unsealV3ForEntity(sealed, row.strOr("id", ""))
	}
	if row.strOr("provider", "wordpress") == "wordpress" {
		return d.unsealT2WithSecret(sealed, d.Cfg.Secret)
	}
	return d.unsealT2(sealed)
}

func (d *Deps) unsealT2WithSecret(sealed, secret string) (string, error) {
	return unsealWithSecret(sealed, secret)
}

func (d *Deps) unsealV3ForEntity(sealed, entityID string) (string, error) {
	return unsealV3(sealed, d.Cfg.Secret, entityID)
}

func wpHeaders(username, secret string) map[string]string {
	basic := base64.StdEncoding.EncodeToString([]byte(username + ":" + secret))
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Basic " + basic,
	}
}

func wpURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/wp-json/" + path
}

func unsealWithSecret(sealed, secret string) (string, error) {
	return crypto.Unseal(sealed, secret)
}

func unsealV3(sealed, appSecret, entityID string) (string, error) {
	return crypto.UnsealV3(sealed, appSecret, entityID)
}

// wpResponse: hasil request WordPress.
type wpResponse struct {
	status int
	body   string
}

// wpRequest: request ke WordPress REST dengan guard SSRF (safe client).
func wpRequest(ctx context.Context, target, method string, headers map[string]string, body string, timeout time.Duration) (*wpResponse, error) {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := security.SafeClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	text, err := readBodyLimited(resp, 512*1024)
	if err != nil {
		return nil, err
	}
	return &wpResponse{status: resp.StatusCode, body: text}, nil
}

// generateChat: pembungkus GenerateChat dengan tier default (parity mitra rewrite).
func (d *Deps) generateChat(r *http.Request, prompt, taskType string, brandID *string, userID string) (*ai.GenerationResult, error) {
	return ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt:   prompt,
		Tier:     "default",
		TaskType: taskType,
		BrandID:  brandID,
		UserID:   &userID,
	})
}

// compressImageIfNeeded: parity compressImageIfNeeded — turunkan kualitas
// bertahap sampai <=110KB. Implementasi Go memakai resize+re-encode bertahap
// via paket image standar; bila gambar tidak dapat didekode, kembalikan asli.
func compressImageIfNeeded(data []byte, mime string) ([]byte, error) {
	const targetBytes = 110 * 1024
	if len(data) <= targetBytes {
		return data, nil
	}
	switch mime {
	case "image/png", "image/jpeg", "image/webp", "image/bmp":
	default:
		return data, nil
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, nil
	}
	_ = format
	bounds := decoded.Bounds()
	width := bounds.Dx()
	if width > 1600 {
		decoded = resizeNearest(decoded, 1600)
	}
	for _, quality := range []int{78, 70, 50} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, decoded, &jpeg.Options{Quality: quality}); err != nil {
			return data, nil
		}
		if buf.Len() <= targetBytes {
			return buf.Bytes(), nil
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, decoded, &jpeg.Options{Quality: 50}); err != nil {
		return data, nil
	}
	return buf.Bytes(), nil
}

// resizeNearest: penyusutan proporsional sederhana (tanpa dependensi eksternal).
func resizeNearest(src image.Image, maxWidth int) image.Image {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= maxWidth {
		return src
	}
	ratio := float64(maxWidth) / float64(width)
	newHeight := int(float64(height) * ratio)
	if newHeight < 1 {
		newHeight = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		srcY := bounds.Min.Y + int(float64(y)/ratio)
		for x := 0; x < maxWidth; x++ {
			srcX := bounds.Min.X + int(float64(x)/ratio)
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

// sealV3ForEntity: seal v3 (t3:) per-entitas (CMS connection, custom web).
func (d *Deps) sealV3ForEntity(plain, entityID string) (string, error) {
	return crypto.SealV3(plain, d.Cfg.Secret, entityID)
}
