package imageconvert

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
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
