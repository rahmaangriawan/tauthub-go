package brandflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// publication_images_test.go: perilaku unggah gambar artikel ke media library
// mitra — pemetaan src, dedupe, cache retry, dan batas jumlah unggahan.

func peTestDeps(t *testing.T) *Deps {
	t.Helper()
	t.Setenv("UPLOAD_DIR", t.TempDir())
	t.Setenv("LEGACY_UPLOAD_DIR", filepath.Join(t.TempDir(), "legacy"))
	t.Setenv("NEXT_PUBLIC_APP_URL", "https://staging.tauthub.com")
	return &Deps{}
}

func peWriteUpload(t *testing.T, d *Deps, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(d.uploadDir(), name), data, 0o644); err != nil {
		t.Fatalf("tulis berkas uji: %v", err)
	}
}

func TestPeIsUploadImageRef(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"/uploads/a.webp", true},
		{"/uploads/a.png", true},
		{"https://staging.tauthub.com/uploads/a.webp", true},
		{"https://contoh.test/gambar.png", false},
		{"/images/article-fallback.png", false},
		{"/uploads/berkas.pdf", false},
		{"data:image/png;base64,AAAA", false},
		{"", false},
	}
	for _, c := range cases {
		if got := peIsUploadImageRef(c.value); got != c.want {
			t.Errorf("peIsUploadImageRef(%q) = %v, mau %v", c.value, got, c.want)
		}
	}
}

func TestPeAttrValueHanyaAtributPersis(t *testing.T) {
	tag := `<img data-src="/uploads/lain.webp" src='/uploads/a.webp' class="foo">`
	value, start, end, quote := peAttrValue(tag, "src")
	if value != "/uploads/a.webp" {
		t.Fatalf("nilai src = %q", value)
	}
	if quote != '\'' {
		t.Fatalf("kutip = %q", quote)
	}
	if got := peReplaceAttrValue(tag, start, end, "https://mitra.test/x.webp"); !strings.Contains(got, "https://mitra.test/x.webp") || !strings.Contains(got, `data-src="/uploads/lain.webp"`) {
		t.Fatalf("ganti nilai salah: %s", got)
	}
}

func TestPeAddImageClass(t *testing.T) {
	got := peAddImageClass(`<img src="/uploads/a.webp">`, 42)
	if !strings.Contains(got, `class="wp-image-42"`) {
		t.Fatalf("kelas tidak ditambahkan: %s", got)
	}
	got = peAddImageClass(`<img src="/uploads/a.webp" class="foo">`, 42)
	if !strings.Contains(got, `class="foo wp-image-42"`) {
		t.Fatalf("kelas lama harus dipertahankan: %s", got)
	}
	got = peAddImageClass(`<img src="/uploads/a.webp" />`, 7)
	if !strings.Contains(got, `class="wp-image-7"`) || !strings.Contains(got, "/>") {
		t.Fatalf("tag self-closing rusak: %s", got)
	}
}

func TestPeFetchUploadBytesDariDisk(t *testing.T) {
	d := peTestDeps(t)
	peWriteUpload(t, d, "a.webp", []byte("webp-bytes"))

	file, err := d.peFetchUploadBytes(context.Background(), "/uploads/a.webp")
	if err != nil {
		t.Fatalf("baca upload lokal: %v", err)
	}
	if string(file.Data) != "webp-bytes" || file.ContentType != "image/webp" || file.Filename != "a.webp" {
		t.Fatalf("berkas salah: %+v", file)
	}

	// URL absolut ke origin kita tetap dibaca dari disk (tidak lewat HTTP yang 401).
	file, err = d.peFetchUploadBytes(context.Background(), "https://staging.tauthub.com/uploads/a.webp")
	if err != nil {
		t.Fatalf("baca upload lewat URL absolut: %v", err)
	}
	if string(file.Data) != "webp-bytes" {
		t.Fatalf("isi berkas salah: %q", file.Data)
	}

	if _, err := d.peFetchUploadBytes(context.Background(), "/uploads/hilang.webp"); err == nil {
		t.Fatal("berkas hilang harus menghasilkan error")
	}
	if _, err := d.peFetchUploadBytes(context.Background(), ""); err == nil {
		t.Fatal("URL kosong harus menghasilkan error")
	}
}

func TestPeUploadInlineImagesGantiSrcDedupeDanCache(t *testing.T) {
	d := peTestDeps(t)
	peWriteUpload(t, d, "a.webp", []byte("aaa"))

	calls := 0
	peUploadMediaHook = func(_ context.Context, _ *Deps, _ string, _ map[string]string, file peImageFile, _ string) (int, string, error) {
		calls++
		return 42, "https://mitra.test/wp-content/uploads/" + file.Filename, nil
	}
	defer func() { peUploadMediaHook = nil }()

	html := `<p>teks</p>` +
		`<img src="/uploads/a.webp" alt="satu">` +
		`<img src="/uploads/a.webp" class="foo">` +
		`<img src="https://contoh.test/luar.png">` +
		`<img src="/images/article-fallback.png">`

	out, cache, failures := d.peUploadInlineImages(context.Background(), "https://mitra.test", nil, html, nil, "Judul Artikel")
	if calls != 1 {
		t.Fatalf("unggahan = %d, mau 1 (dua src identik harus di-dedupe)", calls)
	}
	if len(failures) != 0 {
		t.Fatalf("kegagalan tak terduga: %v", failures)
	}
	if got := strings.Count(out, "https://mitra.test/wp-content/uploads/a.webp"); got != 2 {
		t.Fatalf("src mitra = %d, mau 2: %s", got, out)
	}
	if !strings.Contains(out, "https://contoh.test/luar.png") {
		t.Fatalf("gambar host lain tidak boleh diubah: %s", out)
	}
	if !strings.Contains(out, "/images/article-fallback.png") {
		t.Fatalf("aset non-uploads tidak boleh diubah: %s", out)
	}
	if !strings.Contains(out, `class="wp-image-42"`) || !strings.Contains(out, `class="foo wp-image-42"`) {
		t.Fatalf("kelas wp-image tidak dipasang benar: %s", out)
	}
	if _, ok := cache["/uploads/a.webp"]; !ok {
		t.Fatalf("cache tidak terisi: %v", cache)
	}

	// Retry memakai cache: tidak ada unggahan baru dan HTML sama.
	out2, _, failures2 := d.peUploadInlineImages(context.Background(), "https://mitra.test", nil, html, cache, "Judul Artikel")
	if calls != 1 {
		t.Fatalf("retry mengunggah ulang (%d unggahan)", calls)
	}
	if out2 != out {
		t.Fatalf("hasil retry berbeda:\n%s\n%s", out, out2)
	}
	if len(failures2) != 0 {
		t.Fatalf("kegagalan saat retry: %v", failures2)
	}
}

func TestPeUploadInlineImagesCatatKegagalan(t *testing.T) {
	d := peTestDeps(t)
	html := `<img src="/uploads/hilang.webp">`
	out, _, failures := d.peUploadInlineImages(context.Background(), "https://mitra.test", nil, html, nil, "")
	if out != html {
		t.Fatalf("tag gagal harus dibiarkan apa adanya: %s", out)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "hilang.webp") {
		t.Fatalf("daftar gagal = %v", failures)
	}
}

func TestPeUploadInlineImagesBatasJumlah(t *testing.T) {
	d := peTestDeps(t)
	var html strings.Builder
	for i := 0; i < peMaxInlineUploads+3; i++ {
		name := fmt.Sprintf("img-%02d.webp", i)
		peWriteUpload(t, d, name, []byte("x"))
		fmt.Fprintf(&html, `<img src="/uploads/%s">`, name)
	}
	calls := 0
	peUploadMediaHook = func(_ context.Context, _ *Deps, _ string, _ map[string]string, _ peImageFile, _ string) (int, string, error) {
		calls++
		return calls, "https://mitra.test/wp-content/uploads/gambar.webp", nil
	}
	defer func() { peUploadMediaHook = nil }()

	out, _, _ := d.peUploadInlineImages(context.Background(), "https://mitra.test", nil, html.String(), nil, "")
	if calls != peMaxInlineUploads {
		t.Fatalf("unggahan = %d, mau %d", calls, peMaxInlineUploads)
	}
	over := fmt.Sprintf(`src="/uploads/img-%02d.webp"`, peMaxInlineUploads+2)
	if !strings.Contains(out, over) {
		t.Fatalf("gambar di atas batas harus dibiarkan: %s", out)
	}
}
