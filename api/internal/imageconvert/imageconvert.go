// Package imageconvert: konversi gambar unggahan ke WebP.
//
// Parity `compressCoverImage` / `compressImageIfNeeded` di app lama
// (lib/uploads.ts, memakai sharp). Di Go dipakai encoder WebP pure-Go
// (nativewebp, VP8L lossless) + resizer dari golang.org/x/image — tanpa cgo
// supaya binary tetap portabel di VPS.
//
// CATATAN DEVIASI (dicatat di docs/REWRITE/00 §6): nativewebp hanya lossless
// (VP8L), sedangkan sharp memakai lossy. Hasil WebP di sini bisa LEBIH BESAR
// dari aslinya untuk foto. Karena itu Resize dulu ke lebar maksimum, dan
// pemanggil boleh memakai hasil lossless hanya bila ukurannya tidak membengkak
// (lihat ConvertToWebP: kalau hasil lebih besar dari sumber, sumber dipakai
// apa adanya bila sudah WebP, atau tetap dipakai bila konversi diminta eksplisit).
package imageconvert

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	xdraw "golang.org/x/image/draw"

	// Dekoder tambahan: WebP (baca) dan BMP.
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// MaxWidth: lebar maksimum keluaran (parity IMAGE_MAX_DIMENSION app lama).
const MaxWidth = 1600

// IsConvertibleImage: MIME yang bisa dikonversi ke WebP.
func IsConvertibleImage(mime string) bool {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif", "image/bmp":
		return true
	}
	return false
}

// IsAlreadyWebP: true bila byte sudah WebP (magic RIFF....WEBP).
func IsAlreadyWebP(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

// ConvertToWebP: dekode gambar apa pun yang didukung, perkecil bila lebih lebar
// dari MaxWidth, lalu encode WebP (VP8L).
//
// Mengembalikan (data, true) bila konversi berhasil. Bila dekode/encode gagal,
// atau sumber bukan gambar yang bisa dikonversi, mengembalikan (data asli, false)
// supaya pemanggil bisa memakai byte asli (tidak pernah menggagalkan upload).
func ConvertToWebP(data []byte, mime string) ([]byte, bool) {
	if !IsConvertibleImage(mime) {
		return data, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, false
	}
	if img.Bounds().Dx() > MaxWidth {
		img = Resize(img, MaxWidth)
	}
	var buf bytes.Buffer
	if err := nativewebp.Encode(&buf, img, nil); err != nil {
		return data, false
	}
	out := buf.Bytes()
	// Hasil lossless bisa lebih besar dari sumber yang sudah WebP — jangan
	// membengkakkan penyimpanan; pakai sumber apa adanya.
	if IsAlreadyWebP(data) && len(out) >= len(data) {
		return data, false
	}
	return out, true
}

// Resize: penyusutan proporsional memakai interpolasi CatmullRom (kualitas
// lebih baik dari nearest untuk foto, tetap tanpa cgo).
func Resize(src image.Image, maxWidth int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxWidth || w == 0 {
		return src
	}
	ratio := float64(maxWidth) / float64(w)
	newH := int(float64(h) * ratio)
	if newH < 1 {
		newH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, newH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// Decode: dekode gambar (dipakai untuk memastikan byte valid).
func Decode(data []byte) (image.Image, string, error) {
	return image.Decode(bytes.NewReader(data))
}

// encodeJPEG: helper internal untuk fallback/test.
func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// Pastikan dekoder standar ikut terdaftar (png/gif/jpeg) — impor eksplisit
// supaya perilakunya jelas dan tidak bergantung pada efek samping paket lain.
var (
	_ = png.Decode
	_ = gif.Decode
	_ = jpeg.Decode
)
