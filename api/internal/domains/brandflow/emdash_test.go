package brandflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// emdash_test.go: konversi Portable Text, unggah gambar ke media library
// EmDash, dan bentuk payload tulis per tata letak REST.

func emdashTestDeps(t *testing.T) *Deps {
	t.Helper()
	t.Setenv("UPLOAD_DIR", t.TempDir())
	t.Setenv("LEGACY_UPLOAD_DIR", t.TempDir())
	t.Setenv("NEXT_PUBLIC_APP_URL", "https://staging.tauthub.com")
	return &Deps{}
}

func peFindBlock(blocks []map[string]any, style string) map[string]any {
	for _, block := range blocks {
		if bsStringifyAny(block["style"]) == style {
			return block
		}
	}
	return nil
}

func TestHtmlToPortableTextStruktur(t *testing.T) {
	html := `<p>Halo <strong>dunia</strong> &amp; sekitar</p>` +
		`<h2>Sub Judul</h2>` +
		`<ul><li>Satu</li><li>Dua</li></ul>` +
		`<p>Baca <a href="https://contoh.test/artikel">tautan ini</a>.</p>` +
		`<blockquote><p>Kutipan penting.</p></blockquote>`

	blocks := htmlToPortableText(html, nil)

	first := blocks[0]
	if bsStringifyAny(first["_type"]) != "block" || bsStringifyAny(first["style"]) != "normal" {
		t.Fatalf("blok pertama bukan paragraf: %v", first)
	}
	spans, _ := first["children"].([]map[string]any)
	if len(spans) != 3 {
		t.Fatalf("span paragraf = %d, mau 3: %v", len(spans), spans)
	}
	if bsStringifyAny(spans[1]["text"]) != "dunia" {
		t.Fatalf("span tengah = %q", bsStringifyAny(spans[1]["text"]))
	}
	if marks, _ := spans[1]["marks"].([]string); len(marks) != 1 || marks[0] != "strong" {
		t.Fatalf("mark tebal tidak terpasang: %v", spans[1]["marks"])
	}
	if bsStringifyAny(spans[2]["text"]) != " & sekitar" {
		t.Fatalf("entitas HTML tidak dibuka: %q", bsStringifyAny(spans[2]["text"]))
	}

	if peFindBlock(blocks, "h2") == nil {
		t.Fatalf("heading h2 tidak terbentuk: %v", blocks)
	}

	bullets := 0
	for _, block := range blocks {
		if bsStringifyAny(block["listItem"]) == "bullet" {
			bullets++
		}
	}
	if bullets != 2 {
		t.Fatalf("item daftar = %d, mau 2: %v", bullets, blocks)
	}

	if peFindBlock(blocks, "blockquote") == nil {
		t.Fatalf("blockquote tidak terbentuk: %v", blocks)
	}

	linkFound := false
	for _, block := range blocks {
		defs, _ := block["markDefs"].([]map[string]any)
		for _, def := range defs {
			if bsStringifyAny(def["href"]) == "https://contoh.test/artikel" {
				linkFound = true
			}
		}
	}
	if !linkFound {
		t.Fatalf("markDefs tautan tidak ada: %v", blocks)
	}
}

func TestHtmlToPortableTextGambarDanTabel(t *testing.T) {
	images := map[string]emdashImageRef{
		"/uploads/a.webp": {ID: "media-1", URL: "/_emdash/api/media/file/abc.webp"},
	}
	html := `<p>Sebelum</p><img src="/uploads/a.webp" alt="Ilustrasi"><p>Sesudah</p>` +
		`<table><thead><tr><th>Nama</th><th>Nilai</th></tr></thead><tbody><tr><td>A</td><td>1</td></tr></tbody></table>`

	blocks := htmlToPortableText(html, images)

	var imageBlock map[string]any
	for _, block := range blocks {
		if bsStringifyAny(block["_type"]) == "image" {
			imageBlock = block
		}
	}
	if imageBlock == nil {
		t.Fatalf("blok gambar tidak terbentuk: %v", blocks)
	}
	asset, _ := imageBlock["asset"].(map[string]any)
	if bsStringifyAny(asset["url"]) != "/_emdash/api/media/file/abc.webp" {
		t.Fatalf("asset.url = %q", bsStringifyAny(asset["url"]))
	}
	if bsStringifyAny(asset["_ref"]) != "media-1" {
		t.Fatalf("asset._ref = %q", bsStringifyAny(asset["_ref"]))
	}
	if bsStringifyAny(imageBlock["alt"]) != "Ilustrasi" {
		t.Fatalf("alt = %q", bsStringifyAny(imageBlock["alt"]))
	}

	rows := []string{}
	for _, block := range blocks {
		children, _ := block["children"].([]map[string]any)
		var text strings.Builder
		for _, span := range children {
			text.WriteString(bsStringifyAny(span["text"]))
		}
		rows = append(rows, text.String())
	}
	header, body := false, false
	for _, row := range rows {
		if strings.Contains(row, "Nama") && strings.Contains(row, "Nilai") {
			header = true
		}
		if strings.Contains(row, "A") && strings.Contains(row, "1") && strings.Contains(row, "|") {
			body = true
		}
	}
	if !header || !body {
		t.Fatalf("tabel tidak diratakan jadi baris teks: %v", rows)
	}
}

func TestHtmlToPortableTextTidakMenyisakanTag(t *testing.T) {
	// Bentuk HTML seperti yang benar-benar tersimpan di content_versions
	// (content_format html) — termasuk gambar AI dan tautan dengan atribut.
	html := `<p>Di era disrupsi digital, <strong>Dr Ayu</strong> menyoroti pentingnya konsultasi.</p>` +
		`<h2>Langkah Praktis</h2>` +
		`<ol><li>Petakan audiens</li><li>Ukur dampak</li></ol>` +
		`<img src="/uploads/bcc47177.webp" alt="Ilustrasi" style="max-width:100%;height:auto;border-radius:8px" />` +
		`<p>Rujukan: <a href="https://contoh.test/laporan" target="_blank" rel="noreferrer">laporan tahunan</a>.</p>` +
		`<blockquote><p>Kutipan penting.</p></blockquote>`

	images := map[string]emdashImageRef{"/uploads/bcc47177.webp": {ID: "m-1", URL: "/_emdash/api/media/file/a.webp"}}
	blocks := htmlToPortableText(html, images)

	if len(blocks) == 0 {
		t.Fatal("tidak ada blok yang dihasilkan")
	}
	imageBlocks := 0
	for _, block := range blocks {
		if bsStringifyAny(block["_type"]) == "image" {
			imageBlocks++
			continue
		}
		children, _ := block["children"].([]map[string]any)
		for _, span := range children {
			text := bsStringifyAny(span["text"])
			if strings.ContainsAny(text, "<>") {
				t.Fatalf("tag HTML bocor jadi teks: %q", text)
			}
		}
	}
	if imageBlocks != 1 {
		t.Fatalf("blok gambar = %d, mau 1: %v", imageBlocks, blocks)
	}
	styles := map[string]bool{}
	for _, block := range blocks {
		styles[bsStringifyAny(block["style"])] = true
	}
	for _, want := range []string{"normal", "h2", "blockquote"} {
		if !styles[want] {
			t.Fatalf("gaya %s tidak terbentuk: %v", want, styles)
		}
	}
}

func TestEmdashUploadContentImagesGantiSrcDanCache(t *testing.T) {
	d := emdashTestDeps(t)
	peWriteUpload(t, d, "a.webp", []byte("aaa"))

	calls := 0
	peEmdashUploadHook = func(_ context.Context, _ *Deps, _, _ string, file peImageFile, _ string) (string, string, error) {
		calls++
		return "media-9", "/_emdash/api/media/file/9.webp", nil
	}
	defer func() { peEmdashUploadHook = nil }()

	html := `<img src="/uploads/a.webp" alt="x"><img src="/uploads/a.webp">`
	out, cache, failures := d.emdashUploadContentImages(context.Background(), "https://mitra.test", "tok", html, nil, "Judul")
	if calls != 1 {
		t.Fatalf("unggahan = %d, mau 1", calls)
	}
	if len(failures) != 0 {
		t.Fatalf("kegagalan tak terduga: %v", failures)
	}
	if strings.Count(out, "/_emdash/api/media/file/9.webp") != 2 {
		t.Fatalf("src tidak diganti seluruhnya: %s", out)
	}
	if _, ok := cache["/uploads/a.webp"]; !ok {
		t.Fatalf("cache kosong: %v", cache)
	}

	if _, _, failures := d.emdashUploadContentImages(context.Background(), "https://mitra.test", "tok", html, cache, "Judul"); len(failures) != 0 {
		t.Fatalf("retry dengan cache tidak boleh gagal: %v", failures)
	}
	if calls != 1 {
		t.Fatalf("retry mengunggah ulang (%d)", calls)
	}
}

func TestEmdashUploadContentImagesCatatKegagalan(t *testing.T) {
	d := emdashTestDeps(t)
	html := `<img src="/uploads/hilang.webp">`
	out, _, failures := d.emdashUploadContentImages(context.Background(), "https://mitra.test", "tok", html, nil, "")
	if out != html {
		t.Fatalf("tag gagal harus dibiarkan: %s", out)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "hilang.webp") {
		t.Fatalf("kegagalan tidak tercatat: %v", failures)
	}
}

func TestEmdashContentPayloadDanDialect(t *testing.T) {
	document := bsRow{"title": "Judul Artikel", "slug": "judul-artikel", "meta_description": "Ringkasan"}
	blocks := []map[string]any{{"_type": "block", "style": "normal"}}
	featured := map[string]any{"provider": "local", "id": "media-1"}

	data := emdashContentPayload(document, blocks, featured)
	if _, ok := data["slug"]; ok {
		t.Fatalf("slug tidak boleh ada di dalam data: %v", data)
	}
	if bsStringifyAny(data["excerpt"]) != "Ringkasan" {
		t.Fatalf("excerpt hilang: %v", data)
	}
	if bsStringifyAny(data["featured_image"].(map[string]any)["id"]) != "media-1" {
		t.Fatalf("featured_image tidak terpasang: %v", data)
	}
	if _, ok := emdashContentPayload(document, blocks, nil)["featured_image"]; ok {
		t.Fatal("tanpa cover, featured_image tidak boleh dikirim")
	}

	modern := emdashWriteEnvelope(data, "judul-artikel", "rev-1", emdashDialect{modern: true})
	if bsStringifyAny(modern["slug"]) != "judul-artikel" {
		t.Fatalf("slug modern harus di envelope: %v", modern)
	}
	if bsStringifyAny(modern["_rev"]) != "rev-1" {
		t.Fatalf("_rev hilang: %v", modern)
	}
	if _, ok := modern["data"].(map[string]any)["slug"]; ok {
		t.Fatalf("data modern tidak boleh memuat slug: %v", modern["data"])
	}

	legacy := emdashWriteEnvelope(data, "judul-artikel", "", emdashDialect{})
	if bsStringifyAny(legacy["data"].(map[string]any)["slug"]) != "judul-artikel" {
		t.Fatalf("slug legacy harus di dalam data: %v", legacy)
	}
	if _, ok := data["slug"]; ok {
		t.Fatalf("data sumber tidak boleh ikut berubah: %v", data)
	}

	if got := (emdashDialect{modern: true}).contentPath("posts"); got != "content/posts" {
		t.Fatalf("jalur modern = %q", got)
	}
	if got := (emdashDialect{}).contentPath("posts"); got != "posts" {
		t.Fatalf("jalur lama = %q", got)
	}
	if (emdashDialect{modern: true}).updateMethod() != "PUT" || (emdashDialect{}).updateMethod() != "PATCH" {
		t.Fatal("metode update per dialect salah")
	}
}

func TestEmdashUnknownField(t *testing.T) {
	if !emdashUnknownField(pePublishErr("VALIDATION_ERROR", "featured_image: unknown field on collection 'posts'")) {
		t.Fatal("error field asing harus terdeteksi")
	}
	if emdashUnknownField(errors.New("WordPress returned 500")) {
		t.Fatal("error lain tidak boleh dianggap field asing")
	}
	if emdashUnknownField(nil) {
		t.Fatal("nil bukan field asing")
	}
}

func TestPeEmdashImageRefs(t *testing.T) {
	cache := map[string]any{
		"/uploads/a.webp": map[string]any{"id": "m-1", "url": "/_emdash/api/media/file/a.webp"},
	}
	refs := peEmdashImageRefs(cache)
	if refs["/uploads/a.webp"].ID != "m-1" || refs["/uploads/a.webp"].URL != "/_emdash/api/media/file/a.webp" {
		t.Fatalf("refs salah: %+v", refs)
	}
	// Konversi Portable Text berjalan setelah src diganti, jadi URL hasil
	// unggah harus ikut menjadi kunci.
	if refs["/_emdash/api/media/file/a.webp"].ID != "m-1" {
		t.Fatalf("refs per URL hasil unggah hilang: %+v", refs)
	}
	if len(peEmdashImageRefs(nil)) != 0 {
		t.Fatal("cache kosong harus menghasilkan peta kosong")
	}
}

func TestHtmlToPortableTextPakaiRefSetelahRewrite(t *testing.T) {
	// src sudah diganti ke URL media EmDash oleh langkah unggah.
	html := `<img src="/_emdash/api/media/file/a.webp" alt="x">`
	cache := map[string]any{
		"/uploads/a.webp": map[string]any{"id": "m-7", "url": "/_emdash/api/media/file/a.webp"},
	}
	blocks := htmlToPortableText(html, peEmdashImageRefs(cache))
	if len(blocks) != 1 {
		t.Fatalf("blok = %d, mau 1: %v", len(blocks), blocks)
	}
	asset, _ := blocks[0]["asset"].(map[string]any)
	if bsStringifyAny(asset["_ref"]) != "m-7" {
		t.Fatalf("asset._ref = %q", bsStringifyAny(asset["_ref"]))
	}
}

func TestEmdashWriteEnvelopeBisaDiMarshal(t *testing.T) {
	data := emdashContentPayload(bsRow{"title": "T", "slug": "t"}, []map[string]any{
		{"_type": "image", "_key": "img-1", "asset": map[string]any{"_ref": "m-1", "url": "/x.webp"}},
	}, map[string]any{"provider": "local", "id": "m-1"})
	raw, err := json.Marshal(emdashWriteEnvelope(data, "t", "", emdashDialect{modern: true}))
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if !strings.Contains(string(raw), `"featured_image":{"id":"m-1","provider":"local"}`) {
		t.Fatalf("bentuk featured_image tidak sesuai: %s", raw)
	}
}
