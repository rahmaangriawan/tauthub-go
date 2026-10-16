// Package imageconvert: konversi gambar unggahan ke WebP.
//
// Parity `compressCoverImage` / `compressImageIfNeeded` di app lama
// (lib/uploads.ts, memakai sharp). Di Go ada dua encoder:
//   - nativewebp (VP8L, lossless, pure-Go) untuk konversi cepat tanpa
//     penurunan kualitas;
//   - gen2brain/webp (libwebp asli via WASM/purego, LOSSY) untuk kompresi
//     nyata — dipakai CompressToWebP karena foto/ilustrasi AI 1600px bisa
//     1-1,5 MB bila di-encode lossless.
//
// Keduanya tanpa cgo supaya binary tetap portabel di VPS (CGO_ENABLED=0).
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
	"github.com/gen2brain/webp"
	xdraw "golang.org/x/image/draw"

	// Dekoder tambahan: WebP (baca) dan BMP.
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// MaxWidth: lebar maksimum keluaran (parity IMAGE_MAX_DIMENSION app lama).
const MaxWidth = 1600

// LossyQuality: kualitas default encode WebP lossy untuk gambar AI.
const LossyQuality = 82

// TargetMaxBytes: batas keras ukuran gambar yang DISIMPAN oleh sistem (150 KB).
// Gambar AI dikompres bertahap — turunkan kualitas dulu, lalu turunkan dimensi —
// sampai hasilnya di bawah batas ini.
const TargetMaxBytes = 150 << 10

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

// CompressToWebP: dekode gambar apa pun yang didukung, perkecil bila lebih
// lebar dari MaxWidth, lalu encode WebP LOSSY (libwebp) sampai ukurannya di
// bawah TargetMaxBytes (150 KB): kualitas diturunkan dulu (82 → 70 → 58 → 46),
// baru dimensi (×0,8 sampai minimum 800 px).
//
// Ini yang dipakai untuk gambar hasil AI: encoder lossless bisa meninggalkan
// file 1-1,5 MB, lossy + batas 150 KB memangkasnya lebih dari 10x.
//
// Mengembalikan (data, true) bila ada hasil yang LEBIH KECIL dari sumber;
// bila sumber sudah di bawah batas dan/atau tidak ada varian yang menang,
// sumber dipakai apa adanya (false) supaya tidak ada penyimpanan yang membengkak.
func CompressToWebP(data []byte, mime string, quality int) ([]byte, bool) {
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
	if quality <= 0 || quality > 100 {
		quality = LossyQuality
	}
	// Sudah WebP dan sudah di bawah batas: tidak perlu disentuh.
	if IsAlreadyWebP(data) && len(data) <= TargetMaxBytes {
		return data, false
	}

	var best []byte
	width := img.Bounds().Dx()
search:
	for {
		current := img
		if width < img.Bounds().Dx() {
			current = Resize(img, width)
		}
		for q := quality; q >= 40; q -= 12 {
			out := encodeLossy(current, q)
			if out == nil {
				break
			}
			if best == nil || len(out) < len(best) {
				best = out
			}
			if len(out) <= TargetMaxBytes {
				best = out
				break search
			}
		}
		// Kualitas terendah masih kelebihan → turunkan dimensi, ulangi.
		if width <= 800 {
			break
		}
		width = int(float64(width) * 0.8)
		if width < 800 {
			width = 800
		}
	}

	if best == nil || len(best) >= len(data) {
		return data, false
	}
	return best, true
}

// encodeLossy: satu percobaan encode WebP lossy; nil bila gagal.
func encodeLossy(img image.Image, quality int) []byte {
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.Options{Lossless: false, Quality: quality, Method: 4}); err != nil {
		return nil
	}
	if buf.Len() == 0 {
		return nil
	}
	return buf.Bytes()
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
