package imageconvert

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand"
	"testing"

	"github.com/HugoSmits86/nativewebp"
)

func samplePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func sampleJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8((x * 3) % 256), uint8((y * 5) % 256), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// PNG → WebP: keluaran harus benar-benar WebP dan bisa didekode ulang.
func TestConvertPNGToWebP(t *testing.T) {
	src := samplePNG(t, 400, 300)
	out, ok := ConvertToWebP(src, "image/png")
	if !ok {
		t.Fatal("konversi PNG gagal")
	}
	if !IsAlreadyWebP(out) {
		t.Fatalf("keluaran bukan WebP (magic bytes salah), %d byte", len(out))
	}
	img, format, err := Decode(out)
	if err != nil {
		t.Fatalf("hasil tidak bisa didekode: %v", err)
	}
	if format != "webp" {
		t.Fatalf("format hasil = %q, ingin webp", format)
	}
	if img.Bounds().Dx() != 400 || img.Bounds().Dy() != 300 {
		t.Fatalf("dimensi berubah: %v", img.Bounds())
	}
}

func TestConvertJPEGToWebP(t *testing.T) {
	src := sampleJPEG(t, 300, 200)
	out, ok := ConvertToWebP(src, "image/jpeg")
	if !ok {
		t.Fatal("konversi JPEG gagal")
	}
	if !IsAlreadyWebP(out) {
		t.Fatal("keluaran bukan WebP")
	}
}

// Gambar lebih lebar dari MaxWidth harus diperkecil proporsional.
func TestConvertResizesOversizedImage(t *testing.T) {
	src := samplePNG(t, 2400, 1200)
	out, ok := ConvertToWebP(src, "image/png")
	if !ok {
		t.Fatal("konversi gagal")
	}
	img, _, err := Decode(out)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := img.Bounds().Dx(); got != MaxWidth {
		t.Fatalf("lebar = %d, ingin %d", got, MaxWidth)
	}
	if got := img.Bounds().Dy(); got != 800 {
		t.Fatalf("tinggi = %d, ingin 800 (rasio dipertahankan)", got)
	}
}

// WebP yang sudah WebP dan tidak mengecil → byte asli dipertahankan (tanpa membengkakkan).
func TestAlreadyWebPKeptWhenNotSmaller(t *testing.T) {
	src := samplePNG(t, 200, 200)
	webp, ok := ConvertToWebP(src, "image/png")
	if !ok {
		t.Fatal("konversi awal gagal")
	}
	out, ok := ConvertToWebP(webp, "image/webp")
	if ok {
		t.Fatalf("WebP yang sudah WebP seharusnya tidak dikonversi ulang, dapat %d byte", len(out))
	}
	if !bytes.Equal(out, webp) {
		t.Fatal("byte asli harus dipertahankan")
	}
}

// MIME non-gambar tidak dikonversi.
func TestNonImageUntouched(t *testing.T) {
	src := []byte("%PDF-1.4 fake")
	out, ok := ConvertToWebP(src, "application/pdf")
	if ok {
		t.Fatal("PDF tidak boleh dikonversi")
	}
	if !bytes.Equal(out, src) {
		t.Fatal("byte asli harus utuh")
	}
}

// Byte rusak tidak boleh membuat panic / menggagalkan upload.
func TestCorruptImageFallsBack(t *testing.T) {
	src := []byte("bukan gambar sama sekali")
	out, ok := ConvertToWebP(src, "image/png")
	if ok {
		t.Fatal("byte rusak tidak boleh dianggap berhasil dikonversi")
	}
	if !bytes.Equal(out, src) {
		t.Fatal("byte asli harus dikembalikan")
	}
}

// IsConvertibleImage
func TestIsConvertibleImage(t *testing.T) {
	for _, m := range []string{"image/png", "image/jpeg", "IMAGE/WEBP", "image/gif", "image/bmp"} {
		if !IsConvertibleImage(m) {
			t.Errorf("%s harus convertible", m)
		}
	}
	for _, m := range []string{"application/pdf", "text/plain", "video/mp4", ""} {
		if IsConvertibleImage(m) {
			t.Errorf("%s tidak boleh convertible", m)
		}
	}
}

// Kompresi lossy: kasus nyata gambar AI — file WebP LOSSLESS (nativewebp)
// berukuran besar harus menyusut jauh setelah dikompres lossy dan tetap bisa
// didekode ulang.
func TestCompressToWebPLossyShrinks(t *testing.T) {
	// Pola mirip foto: gelombang halus + noise halus. Noise membuat encoder
	// lossless WAJIB menyimpan hampir semua piksel (file besar, seperti
	// gambar AI yang tersimpan sekarang), sedangkan lossy meratakannya.
	img := image.NewRGBA(image.Rect(0, 0, 1600, 900))
	rng := rand.New(rand.NewSource(42))
	for y := 0; y < 900; y++ {
		for x := 0; x < 1600; x++ {
			fx, fy := float64(x), float64(y)
			n := func() int { return rng.Intn(25) - 12 }
			img.Set(x, y, color.RGBA{
				uint8(clamp8(128 + 70*math.Sin(fx/37)*math.Cos(fy/53) + float64(n()))),
				uint8(clamp8(128 + 70*math.Sin(fx/91+fy/71) + float64(n()))),
				uint8(clamp8(128 + 70*math.Cos((fx+fy)/61) + float64(n()))),
				255,
			})
		}
	}
	var losslessBuf bytes.Buffer
	if err := nativewebp.Encode(&losslessBuf, img, nil); err != nil {
		t.Fatalf("encode lossless: %v", err)
	}
	baseline := losslessBuf.Bytes()
	if len(baseline) < 200_000 {
		t.Fatalf("fixture terlalu kecil untuk uji kompresi: %d byte", len(baseline))
	}

	out, ok := CompressToWebP(baseline, "image/webp", LossyQuality)
	if !ok {
		t.Fatalf("kompresi lossy gagal (baseline %d byte)", len(baseline))
	}
	if !IsAlreadyWebP(out) {
		t.Fatalf("keluaran bukan WebP, %d byte", len(out))
	}
	if len(out) >= len(baseline) {
		t.Fatalf("keluaran tidak lebih kecil: %d >= %d", len(out), len(baseline))
	}
	if len(out) > TargetMaxBytes {
		t.Fatalf("keluaran %d byte melebihi batas %d byte", len(out), TargetMaxBytes)
	}
	if _, _, err := image.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("keluaran tidak bisa didekode: %v", err)
	}

	// Sumber yang sudah WebP dan sudah di bawah batas tidak boleh disentuh.
	small := out
	kept, ok2 := CompressToWebP(small, "image/webp", LossyQuality)
	if ok2 {
		t.Fatalf("sumber di bawah batas seharusnya dipakai apa adanya, tapi dikembalikan %d byte", len(kept))
	}
	if !bytes.Equal(kept, small) {
		t.Fatal("byte sumber berubah padahal seharusnya dipakai apa adanya")
	}
}

func clamp8(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return int(v)
}
