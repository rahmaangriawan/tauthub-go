// image.go: port minimal endpoint OpenAI-compatible /images/generations
// (bagian yang dipakai generate gambar artikel Taut Pintar).
//
// Placeholder fallback: bila provider gambar belum dikonfigurasi ATAU panggilan
// gagal, JANGAN hard-fail. Hasilkan PNG lokal deterministik (judul H2 / prompt
// digambar dengan basicfont) supaya alur writer tetap jalan di dev. Provider
// hasil placeholder ditandai "placeholder" agar handler tidak menagih kredit.
package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"math"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// GenerateImageOptions: input generate satu gambar.
type GenerateImageOptions struct {
	Prompt  string
	Size    string // "1024x1024" dll; kosong → default per role
	Role    string // "cover" | "inline"
	Heading string // judul H2 sumber (opsional, untuk placeholder/alt)
	BrandID *string
	UserID  *string
	Runtime *AIRuntime
}

// ImageResult: hasil generate (byte gambar + metadata).
type ImageResult struct {
	Data        []byte
	MimeType    string
	Provider    string
	Model       string
	Width       int
	Height      int
	Placeholder bool
	// FallbackReason: kenapa hasilnya placeholder (kosong bila sukses) — dipakai
	// log monitoring admin supaya "apa & kenapa" terlihat tanpa buka log server.
	FallbackReason string
	// BaseURL: base URL provider yang dipakai saat itu (diagnostik config salah).
	BaseURL string
	// ElapsedMS: durasi panggilan provider.
	ElapsedMS int
}

// imageSizeFor: ukuran default per role.
func imageSizeFor(role string) string {
	return "1920x1080"
}

// parseImageSize: "WxH" → (w, h); fallback 1024x1024.
func parseImageSize(size string) (int, int) {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(size)), "x", 2)
	if len(parts) == 2 {
		w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
		h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errW == nil && errH == nil && w > 0 && h > 0 {
			return w, h
		}
	}
	return 1024, 1024
}

type imagesPayload struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
}

// ImageGenerationConfigured: provider gambar dianggap siap HANYA bila base URL
// gambar, API key gambar, DAN model gambar terisi (kolom DB khusus gambar atau
// fallback env AI_IMAGE_*). Model chat (mis. deepseek) bukan model gambar, jadi
// tanpa model gambar jalur yang benar adalah placeholder lokal.
func ImageGenerationConfigured(rt *AIRuntime) bool {
	if rt == nil || rt.ImageBaseURL == "" || rt.ImageModel == "" {
		return false
	}
	// Pollinations (image.pollinations.ai) tidak butuh API key — gratis.
	return isPollinations(rt.ImageBaseURL) || rt.ImageAPIKey != ""
}

// isPollinations: provider gambar gratis tanpa API key (GET /prompt/{prompt}).
func isPollinations(base string) bool {
	return strings.Contains(strings.ToLower(base), "pollinations")
}

// isOpenRouter: OpenRouter memakai POST /api/v1/images dengan parameter
// aspect_ratio + resolution (bukan size piksel), menolak response_format, dan
// mengembalikan b64_json di data[0] secara default.
func isOpenRouter(base string) bool {
	return strings.Contains(strings.ToLower(base), "openrouter.ai")
}

// fixedImageEndpoint: provider dengan endpoint tetap — Pollinations (GET
// /prompt/{prompt}) dan OpenRouter (POST /images). Percobaan varian path/size
// pada provider ini tidak relevan, jadi dilewati.
func fixedImageEndpoint(base string) bool {
	return isPollinations(base) || isOpenRouter(base)
}

// DefaultImageCreditCost: tarif kredit default per gambar AI (2 TC ≈ Rp 2.000).
const DefaultImageCreditCost = 2

// ImageCreditCost: tarif per gambar dari runtime (kolom
// platform_ai_configuration.image_credit_cost, diatur admin lewat tab "Atur
// Harga"). rt nil → fallback env/lokal supaya pemanggil lama tetap aman.
func ImageCreditCost(rt *AIRuntime) int {
	if rt != nil {
		return rt.ImageCreditCost
	}
	return ImageCreditCostEnv()
}

// ImageCreditCostEnv: tarif dari env AI_IMAGE_CREDIT_COST, fallback default.
func ImageCreditCostEnv() int {
	if v := envStr("AI_IMAGE_CREDIT_COST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return DefaultImageCreditCost
}

// GenerateImage: panggil POST {baseURL}/images/generations. Bila provider tidak
// siap atau gagal, hasilkan placeholder lokal (tidak pernah error keras).
func GenerateImage(ctx context.Context, pool *pgxpool.Pool, opts GenerateImageOptions) (*ImageResult, error) {
	rt := opts.Runtime
	if rt == nil {
		loaded, err := LoadAIRuntime(ctx, pool)
		if err != nil {
			return nil, err
		}
		rt = loaded
	}
	size := strings.TrimSpace(opts.Size)
	if size == "" {
		size = imageSizeFor(opts.Role)
	}
	w, h := parseImageSize(size)
	// Model gambar khusus (rt.ImageModel); TIDAK memakai DefaultModel chat
	// (bukan model gambar).
	model := rt.ImageModel
	started := time.Now()

	// Belum dikonfigurasi → placeholder (dev-friendly, gratis).
	if !ImageGenerationConfigured(rt) {
		return placeholderResult(opts, w, h,
			"Provider gambar belum lengkap: isi Base URL, API key, dan Model gambar di halaman AI Gambar.",
			rt.ImageBaseURL, started)
	}

	data, mime, err := callImageProvider(ctx, rt, model, opts.Prompt, size, "/images/generations", true)
	// Endpoint beda antar provider: OpenAI-compatible umumnya
	// /images/generations, OpenRouter memakai /images. Sebagian juga menolak
	// parameter size/aspect ratio. Coba berurutan sampai ada yang berhasil.
	for _, attempt := range []struct {
		path     string
		withSize bool
	}{
		{"/images/generations", false},
		{"/images", true},
		{"/images", false},
	} {
		if err == nil && len(data) > 0 {
			break
		}
		if fixedImageEndpoint(rt.ImageBaseURL) {
			break
		}
		if err != nil {
			slog.Warn("[ai] generate gambar gagal, coba varian lain", "path", attempt.path, "with_size", attempt.withSize, "err", err)
		}
		data, mime, err = callImageProvider(ctx, rt, model, opts.Prompt, size, attempt.path, attempt.withSize)
	}
	if err != nil {
		slog.Warn("[ai] generate gambar gagal, memakai placeholder", "err", err)
		return placeholderResult(opts, w, h, "Panggilan provider gagal: "+err.Error(), rt.ImageBaseURL, started)
	}
	if len(data) == 0 {
		slog.Warn("[ai] provider mengembalikan gambar kosong, memakai placeholder", "model", model)
		return placeholderResult(opts, w, h,
			"Provider mengembalikan gambar kosong (model mungkin bukan model gambar).", rt.ImageBaseURL, started)
	}
	// Semua gambar (cover & inline) dipaksa 16:9 dan maksimum 1920x1280.
	if norm, nw, nh, nerr := NormalizeTo16x9(data); nerr == nil {
		return &ImageResult{
			Data: norm, MimeType: "image/png", Provider: rt.Provider, Model: model,
			Width: nw, Height: nh, Placeholder: false,
			BaseURL: rt.ImageBaseURL, ElapsedMS: int(time.Since(started).Milliseconds()),
		}, nil
	} else {
		slog.Warn("[ai] normalisasi 16:9 gagal, pakai gambar asli", "err", nerr)
	}
	return &ImageResult{
		Data: data, MimeType: mime, Provider: rt.Provider, Model: model,
		Width: w, Height: h, Placeholder: false,
		BaseURL: rt.ImageBaseURL, ElapsedMS: int(time.Since(started).Milliseconds()),
	}, nil
}

// Batas keluaran gambar: rasio 16:9, maksimum 1920x1280.
const (
	maxImageWidth  = 1920
	maxImageHeight = 1280
	imageRatio169  = 16.0 / 9.0
)

// NormalizeTo16x9: crop tengah ke rasio 16:9 lalu perkecil bila melebihi
// 1920x1280, supaya semua gambar (AI maupun upload) seragam dan tidak gepeng.
func NormalizeTo16x9(data []byte) ([]byte, int, int, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return nil, 0, 0, fmt.Errorf("gambar kosong")
	}
	cw, ch := sw, int(math.Round(float64(sw)/imageRatio169))
	if ch > sh {
		ch = sh
		cw = int(math.Round(float64(sh) * imageRatio169))
	}
	ox := b.Min.X + (sw-cw)/2
	oy := b.Min.Y + (sh-ch)/2
	cropped := image.NewRGBA(image.Rect(0, 0, cw, ch))
	xdraw.Draw(cropped, cropped.Bounds(), src, image.Pt(ox, oy), xdraw.Src)

	out := cropped
	ow, oh := cw, ch
	if ow > maxImageWidth || oh > maxImageHeight {
		scale := math.Min(float64(maxImageWidth)/float64(cw), float64(maxImageHeight)/float64(ch))
		ow = max(1, int(math.Round(float64(cw)*scale)))
		oh = max(1, int(math.Round(float64(ch)*scale)))
		dst := image.NewRGBA(image.Rect(0, 0, ow, oh))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), cropped, cropped.Bounds(), xdraw.Over, nil)
		out = dst
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), ow, oh, nil
}

// cleanImageBase: bersihkan akhiran endpoint seperti /images atau /images/generations
// bila user menempelkan URL lengkap ke field Base URL di admin, supaya tidak
// dobel (mis. .../images/images -> 404).
func cleanImageBase(raw string) string {
	b := strings.TrimRight(strings.TrimSpace(raw), "/")
	for _, suffix := range []string{"/images/generations", "/images", "/chat/completions"} {
		if strings.HasSuffix(strings.ToLower(b), suffix) {
			b = b[:len(b)-len(suffix)]
			b = strings.TrimRight(b, "/")
		}
	}
	return b
}

// callImageProvider: satu percobaan request; dukung b64_json maupun url.
func callImageProvider(ctx context.Context, rt *AIRuntime, model, prompt, size, path string, withSize bool) ([]byte, string, error) {
	// Pollinations: endpoint GET tanpa API key, bukan POST JSON.
	if isPollinations(rt.ImageBaseURL) {
		return callPollinations(ctx, rt, model, prompt, size)
	}
	// OpenRouter: endpoint POST /images dengan parameter modelnya sendiri.
	if isOpenRouter(rt.ImageBaseURL) {
		return callOpenRouterImages(ctx, rt, model, prompt)
	}
	base := cleanImageBase(rt.ImageBaseURL)
	endpoint := base + path
	payload := map[string]any{
		"model":           model,
		"prompt":          prompt,
		"response_format": "b64_json",
	}
	if withSize {
		payload["size"] = size
	}
	body, _ := json.Marshal(payload)
	timeout := rt.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if reqErr != nil {
		return nil, "", reqErr
	}
	req.Header.Set("Authorization", "Bearer "+rt.ImageAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, doErr := client.Do(req)
	if doErr != nil {
		return nil, "", fmt.Errorf("provider connection failed: %w", doErr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("provider rejected (%d): %s", resp.StatusCode, truncateRunes(string(raw), 300))
	}
	var parsed imagesPayload
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Data) == 0 {
		return nil, "", fmt.Errorf("provider returned an invalid image response")
	}
	item := parsed.Data[0]
	if item.B64JSON != "" {
		decoded, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			// Sebagian provider memakai varian URL-safe tanpa padding.
			decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(item.B64JSON, "="))
			if err != nil {
				return nil, "", fmt.Errorf("decode b64 gambar gagal: %w", err)
			}
		}
		return decoded, detectImageMime(decoded), nil
	}
	if item.URL != "" {
		// Provider mengabaikan response_format dan mengembalikan URL.
		imgReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
		imgResp, err := client.Do(imgReq)
		if err != nil {
			return nil, "", fmt.Errorf("ambil gambar dari url gagal: %w", err)
		}
		defer imgResp.Body.Close()
		if imgResp.StatusCode < 200 || imgResp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("ambil gambar dari url gagal (%d)", imgResp.StatusCode)
		}
		imgData, _ := io.ReadAll(io.LimitReader(imgResp.Body, 20<<20))
		return imgData, detectImageMime(imgData), nil
	}
	return nil, "", fmt.Errorf("provider tidak mengembalikan gambar")
}

// callPollinations: GET {base}/prompt/{prompt}?width&height&nologo — gratis,
// tanpa API key, mengembalikan bytes gambar langsung.
func callPollinations(ctx context.Context, rt *AIRuntime, model, prompt, size string) ([]byte, string, error) {
	w, h := parseImageSize(size)
	endpoint := strings.TrimRight(rt.ImageBaseURL, "/") + "/prompt/" + neturl.PathEscape(prompt) +
		"?width=" + strconv.Itoa(w) + "&height=" + strconv.Itoa(h) + "&nologo=true"
	if m := strings.TrimSpace(model); m != "" && !strings.EqualFold(m, "pollinations") {
		endpoint += "&model=" + neturl.QueryEscape(m)
	}
	timeout := rt.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("pollinations connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("pollinations rejected (%d): %s", resp.StatusCode, truncateRunes(string(body), 200))
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if len(data) == 0 {
		return nil, "", fmt.Errorf("pollinations mengembalikan gambar kosong")
	}
	return data, detectImageMime(data), nil
}

// callOpenRouterImages: POST {base}/images (mis. https://openrouter.ai/api/v1
// + /images). Parameter mengikuti model terdaftar di GET /api/v1/images/models:
// Seedream menerima aspect_ratio + resolution, bukan size piksel, dan respons
// selalu b64_json (response_format tidak ada di daftar parameternya). Rasio
// dikunci 16:9 karena semua gambar dinormalisasi ke 16:9 di GenerateImage.
func callOpenRouterImages(ctx context.Context, rt *AIRuntime, model, prompt string) ([]byte, string, error) {
	if strings.TrimSpace(model) == "" {
		return nil, "", fmt.Errorf("model gambar OpenRouter wajib diisi (mis. bytedance-seed/seedream-5-0-flash)")
	}
	endpoint := cleanImageBase(rt.ImageBaseURL) + "/images"
	payload := map[string]any{
		"model":        model,
		"prompt":       prompt,
		"aspect_ratio": "16:9",
		"resolution":   "2K",
	}
	body, _ := json.Marshal(payload)
	timeout := rt.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if reqErr != nil {
		return nil, "", reqErr
	}
	req.Header.Set("Authorization", "Bearer "+rt.ImageAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, doErr := client.Do(req)
	if doErr != nil {
		return nil, "", fmt.Errorf("openrouter connection failed: %w", doErr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("openrouter rejected (%d): %s", resp.StatusCode, truncateRunes(string(raw), 300))
	}
	var parsed imagesPayload
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Data) == 0 {
		return nil, "", fmt.Errorf("openrouter returned an invalid image response")
	}
	item := parsed.Data[0]
	if item.B64JSON == "" {
		return nil, "", fmt.Errorf("openrouter tidak mengembalikan b64 gambar")
	}
	decoded, err := base64.StdEncoding.DecodeString(item.B64JSON)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(item.B64JSON, "="))
		if err != nil {
			return nil, "", fmt.Errorf("decode b64 gambar gagal: %w", err)
		}
	}
	return decoded, detectImageMime(decoded), nil
}

// ImageTestResult: hasil uji koneksi provider gambar (tombol "Tes Koneksi" admin).
type ImageTestResult struct {
	Configured  bool   `json:"configured"`
	OK          bool   `json:"ok"`
	BaseURL     string `json:"base_url"`
	Model       string `json:"model"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Bytes       int    `json:"bytes"`
	ElapsedMS   int64  `json:"elapsed_ms"`
	Message     string `json:"message"`
}

// TestImageConnection: panggil provider gambar SEKALI tanpa fallback placeholder,
// supaya error asli (401/402/404/429, dsb.) terlihat saat debugging dari admin.
func TestImageConnection(ctx context.Context, pool *pgxpool.Pool) *ImageTestResult {
	InvalidateRuntimeCache()
	res := &ImageTestResult{}
	rt, err := LoadAIRuntime(ctx, pool)
	if err != nil {
		res.Message = "Gagal memuat konfigurasi: " + err.Error()
		return res
	}
	res.BaseURL, res.Model = rt.ImageBaseURL, rt.ImageModel
	if !ImageGenerationConfigured(rt) {
		res.Message = "Provider gambar belum lengkap: isi Base URL, API Key, dan Model gambar."
		return res
	}
	res.Configured = true

	// Key OpenRouter selalu diawali "sk-or-" (sk-or-v1- + 64 hex). Token lain
	// ditolak OpenRouter sebagai 401 "Missing Authentication header" — pesan
	// yang menyesatkan, jadi tangkap di sini dengan petunjuk yang jelas.
	if isOpenRouter(rt.ImageBaseURL) && !strings.HasPrefix(strings.TrimSpace(rt.ImageAPIKey), "sk-or") {
		res.Message = "API key gambar tidak berformat key OpenRouter (harus diawali \"sk-or-\"). Salin ulang key utuh dari openrouter.ai/settings/keys, lalu simpan lagi."
		return res
	}

	prompt := "Tauthub connection test: a simple red apple on a white background"
	start := time.Now()
	type attempt struct {
		path     string
		withSize bool
	}
	attempts := []attempt{{"/images/generations", true}, {"/images/generations", false}, {"/images", true}, {"/images", false}}
	if fixedImageEndpoint(rt.ImageBaseURL) {
		// Endpoint tetap: callImageProvider memilih jalurnya sendiri (path diabaikan).
		attempts = []attempt{{"", true}}
	}
	var data []byte
	var lastErr error
	for _, a := range attempts {
		data, _, lastErr = callImageProvider(ctx, rt, rt.ImageModel, prompt, "1920x1080", a.path, a.withSize)
		if lastErr == nil && len(data) > 0 {
			break
		}
	}
	res.ElapsedMS = time.Since(start).Milliseconds()
	if lastErr != nil {
		res.Message = lastErr.Error()
		return res
	}
	if len(data) == 0 {
		res.Message = "Provider mengembalikan gambar kosong (model mungkin bukan model gambar)."
		return res
	}
	if cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(data)); cfgErr == nil {
		res.Width, res.Height = cfg.Width, cfg.Height
	}
	res.Bytes = len(data)
	res.OK = true
	res.Message = "Koneksi OK — provider berhasil membuat gambar."
	return res
}

// detectImageMime: tebak MIME dari magic bytes (self-contained, bukan http sniff).
func detectImageMime(data []byte) string {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	case len(data) >= 4 && string(data[:4]) == "GIF8":
		return "image/gif"
	}
	return "image/png"
}

// placeholderResult: gambar lokal deterministik (gradient + teks) + alasan
// kenapa fallback (dipakai log monitoring admin).
func placeholderResult(opts GenerateImageOptions, w, h int, reason, baseURL string, started time.Time) (*ImageResult, error) {
	label := strings.TrimSpace(opts.Heading)
	if label == "" {
		label = strings.TrimSpace(opts.Prompt)
	}
	data, err := drawPlaceholder(w, h, label)
	if err != nil {
		return nil, err
	}
	return &ImageResult{
		Data: data, MimeType: "image/png", Provider: "placeholder",
		Model: "local-placeholder", Width: w, Height: h, Placeholder: true,
		FallbackReason: reason, BaseURL: baseURL, ElapsedMS: int(time.Since(started).Milliseconds()),
	}, nil
}

// drawPlaceholder: gradien biru + label terbungkus, teks di-scale agar terbaca.
func drawPlaceholder(w, h int, label string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		t := float64(y) / float64(maxInt(h-1, 1))
		r := uint8(30 + 40*t)
		g := uint8(64 + 60*t)
		b := uint8(120 + 80*t)
		row := color.RGBA{R: r, G: g, B: b, A: 255}
		for x := 0; x < w; x++ {
			img.Set(x, y, row)
		}
	}

	text := strings.TrimSpace(label)
	if text == "" {
		text = "Tauthub AI Image"
	}
	lines := wrapText(text, 40)
	face := basicfont.Face7x13
	advance := face.Advance
	lineH := face.Metrics().Height.Ceil()
	textW := 1
	for _, ln := range lines {
		if wpx := len([]rune(ln)) * advance; wpx > textW {
			textW = wpx
		}
	}
	textH := lineH * len(lines)
	if textH < 1 {
		textH = 1
	}
	textImg := image.NewRGBA(image.Rect(0, 0, textW, textH))
	drawer := &font.Drawer{
		Dst:  textImg,
		Src:  image.NewUniform(color.White),
		Face: face,
	}
	ascent := face.Metrics().Ascent.Ceil()
	for i, ln := range lines {
		drawer.Dot = fixed.P(0, i*lineH+ascent)
		drawer.DrawString(ln)
	}

	// Skalakan teks ke 80% lebar kanvas, lalu tempatkan di tengah.
	targetW := int(float64(w) * 0.8)
	targetH := textH * targetW / textW
	if targetH > int(float64(h)*0.6) {
		targetH = int(float64(h) * 0.6)
		targetW = textW * targetH / textH
	}
	dstX := (w - targetW) / 2
	dstY := (h - targetH) / 2
	dstRect := image.Rect(dstX, dstY, dstX+targetW, dstY+targetH)
	xdraw.CatmullRom.Scale(img, dstRect, textImg, textImg.Bounds(), xdraw.Over, nil)

	// Strip kredit biar jelas ini gambar placeholder.
	credit := "placeholder image"
	cd := &font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{255, 255, 255, 180}), Face: face}
	cd.Dot = fixed.P(10, h-10)
	cd.DrawString(credit)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// wrapText: bungkus teks per kata pada lebar karakter maksimum.
func wrapText(text string, maxChars int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	cur := ""
	for _, word := range words {
		if cur == "" {
			cur = word
			continue
		}
		if len([]rune(cur))+1+len([]rune(word)) <= maxChars {
			cur += " " + word
		} else {
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > 6 {
		lines = lines[:6]
	}
	return lines
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
