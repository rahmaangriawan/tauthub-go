package brandflow

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPeBloggerHasDriveScope(t *testing.T) {
	cases := map[string]bool{
		"https://www.googleapis.com/auth/blogger https://www.googleapis.com/auth/drive.file": true,
		"https://www.googleapis.com/auth/drive.file":                                         true,
		"https://www.googleapis.com/auth/drive":                                              true,
		"https://www.googleapis.com/auth/blogger":                                            false,
		"": false,
	}
	for scope, want := range cases {
		if got := peBloggerHasDriveScope(scope); got != want {
			t.Errorf("peBloggerHasDriveScope(%q) = %v, mau %v", scope, got, want)
		}
	}
}

func TestPeBloggerScopeFromRefresh(t *testing.T) {
	// Google hanya mengirim scope saat ada perubahan; nilai lama harus bertahan
	// supaya deteksi izin Drive tidak hilang setelah refresh.
	if got := peBloggerScopeFromRefresh("blogger drive.file", ""); got != "blogger drive.file" {
		t.Fatalf("scope lama hilang: %q", got)
	}
	if got := peBloggerScopeFromRefresh("blogger", "blogger drive.file"); got != "blogger drive.file" {
		t.Fatalf("scope baru tidak dipakai: %q", got)
	}
}

func TestPeBloggerImageURL(t *testing.T) {
	if got := peBloggerImageURL("abc123"); got != "https://lh3.googleusercontent.com/d/abc123" {
		t.Fatalf("URL utama salah: %s", got)
	}
	if got := peBloggerImageURLFallback("abc123"); !strings.Contains(got, "id=abc123") {
		t.Fatalf("URL cadangan salah: %s", got)
	}
}

func TestPeBloggerDriveFolderName(t *testing.T) {
	if got := peBloggerDriveFolderName("Pramugari", "dzkc8g"); got != "Tauthub - Pramugari (dzkc8g)" {
		t.Fatalf("nama folder salah: %s", got)
	}
	// Kutip tunggal bisa merusak query Drive, dan nama panjang dipotong.
	got := peBloggerDriveFolderName("O'Reilly Media", "")
	if strings.Contains(got, "'") {
		t.Fatalf("kutip tunggal tidak dibersihkan: %s", got)
	}
	long := peBloggerDriveFolderName(strings.Repeat("a", 200), "")
	if len([]rune(long)) > len(peBloggerDriveFolderPrefix)+60 {
		t.Fatalf("nama folder tidak dipotong: %d rune", len([]rune(long)))
	}
	empty := peBloggerDriveFolderName("", "")
	if empty != "Tauthub - Media" {
		t.Fatalf("nama folder fallback salah: %s", empty)
	}
}

func TestPeBloggerMediaFileName(t *testing.T) {
	// Nama harus stabil (dipakai ulang saat retry) dan mempertahankan ekstensi.
	a := peBloggerMediaFileName("Tips Menjaga Kebugaran", "https://app.tauthub.com/uploads/a.webp", "a.webp")
	b := peBloggerMediaFileName("Tips Menjaga Kebugaran", "https://app.tauthub.com/uploads/a.webp", "a.webp")
	if a != b {
		t.Fatalf("nama berkas tidak stabil: %q vs %q", a, b)
	}
	if !strings.HasSuffix(a, ".webp") {
		t.Fatalf("ekstensi asli hilang: %s", a)
	}
	png := peBloggerMediaFileName("Judul", "/uploads/x.png", "x.png")
	if !strings.HasSuffix(png, ".png") {
		t.Fatalf("ekstensi PNG hilang: %s", png)
	}
	// Ekstensi tak dikenal → jatuh ke .jpg supaya Drive tetap memperlakukannya
	// sebagai gambar.
	odd := peBloggerMediaFileName("Judul", "/uploads/x.bin", "x.bin")
	if !strings.HasSuffix(odd, ".jpg") {
		t.Fatalf("ekstensi cadangan salah: %s", odd)
	}
	if strings.Contains(a, " ") {
		t.Fatalf("nama berkas tidak boleh berspasi: %s", a)
	}
}

func TestPeDriveMultipartBody(t *testing.T) {
	meta := map[string]any{"name": "gambar.webp", "mimeType": "image/webp", "parents": []string{"folder1"}}
	file := peImageFile{Data: []byte("BINER"), ContentType: "image/webp", Filename: "gambar.webp"}
	body, contentType, err := peDriveMultipartBody(meta, file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(contentType, "multipart/related; boundary=") {
		t.Fatalf("content-type salah: %s", contentType)
	}
	boundary := strings.TrimPrefix(contentType, "multipart/related; boundary=")
	if !bytes.Contains(body, []byte("--"+boundary)) {
		t.Fatal("boundary tidak ada di body")
	}
	if !bytes.HasSuffix(body, []byte("--"+boundary+"--\r\n")) {
		t.Fatal("body multipart tidak ditutup dengan boundary akhir")
	}
	// Metadata harus JSON valid supaya Drive mengenali nama, mime, dan folder.
	start := bytes.Index(body, []byte("{"))
	end := bytes.LastIndex(body, []byte("}"))
	if start < 0 || end <= start {
		t.Fatal("bagian metadata tidak ditemukan")
	}
	var parsed map[string]any
	if err := json.Unmarshal(body[start:end+1], &parsed); err != nil {
		t.Fatalf("metadata bukan JSON valid: %v", err)
	}
	if parsed["mimeType"] != "image/webp" || parsed["name"] != "gambar.webp" {
		t.Fatalf("metadata tidak lengkap: %v", parsed)
	}
	if !bytes.Contains(body, []byte("BINER")) {
		t.Fatal("bytes gambar tidak ikut terkirim")
	}
	if !bytes.Contains(body, []byte("Content-Type: image/webp")) {
		t.Fatal("content-type gambar tidak ikut terkirim")
	}
}

func TestPeBloggerMirrorableURL(t *testing.T) {
	const origin = "https://app.tauthub.com"
	cases := map[string]bool{
		"/uploads/a.webp":                         true,
		origin + "/uploads/a.webp?s=tanda":        true,
		origin + "/images/article-fallback.png":   true, // aset statis milik kita
		"https://cdn.lain.com/gambar.webp":        false,
		"https://blogger.googleusercontent.com/x": false,
		"": false,
	}
	for value, want := range cases {
		if got := peBloggerMirrorableURL(origin, value); got != want {
			t.Errorf("peBloggerMirrorableURL(%q) = %v, mau %v", value, got, want)
		}
	}
}

func TestPeBloggerMediaCacheRoundTrip(t *testing.T) {
	snapshot := map[string]any{"resolved": map[string]any{
		peBloggerMediaCacheKey: map[string]any{
			"https://app.tauthub.com/uploads/a.webp": map[string]any{"file_id": "f1", "url": "https://lh3.googleusercontent.com/d/f1"},
		},
	}}
	cache := peBloggerMediaCache(snapshot)
	if len(cache) != 1 {
		t.Fatalf("cache tidak terbaca: %v", cache)
	}
	if url := bsStringifyAny(cache["https://app.tauthub.com/uploads/a.webp"].(map[string]any)["url"]); url == "" {
		t.Fatal("URL hasil mirror hilang dari cache")
	}
	// Snapshot kosong harus aman (tanpa panic) untuk job baru.
	if got := peBloggerMediaCache(map[string]any{}); got != nil {
		t.Fatalf("snapshot kosong seharusnya nil, dapat %v", got)
	}
}
