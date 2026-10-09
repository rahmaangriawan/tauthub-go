package brandflow

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PNG 1x1 transparan — cukup untuk membuktikan gambar ikut diekstrak.
const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M8AAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

func buildTestDocx(t *testing.T) []byte {
	t.Helper()
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
            xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Judul Dokumen</w:t></w:r></w:p>
    <w:p><w:r><w:t>Paragraf dengan gambar </w:t></w:r>
      <w:r><w:drawing><a:blip r:embed="rId5"/></w:drawing></w:r>
      <w:r><w:t> setelah gambar.</w:t></w:r>
    </w:p>
    <w:p><w:r><w:t>Paragraf penutup.</w:t></w:r></w:p>
  </w:body>
</w:document>`
	relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
  <Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://contoh.test" TargetMode="External"/>
</Relationships>`
	png, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	add("word/document.xml", documentXML)
	add("word/_rels/document.xml.rels", relsXML)
	w, err := zw.Create("word/media/image1.png")
	if err != nil {
		t.Fatalf("zip create media: %v", err)
	}
	if _, err := w.Write(png); err != nil {
		t.Fatalf("zip write media: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractDocxRichKeepsImages(t *testing.T) {
	uploadDir := t.TempDir()
	t.Setenv("UPLOAD_DIR", uploadDir)

	d := &Deps{}
	html, images, err := d.extractDocxRich(buildTestDocx(t))
	if err != nil {
		t.Fatalf("extractDocxRich: %v", err)
	}
	if images != 1 {
		t.Fatalf("harap 1 gambar, dapat %d (html=%s)", images, html)
	}
	if !strings.Contains(html, `<p>Judul Dokumen</p>`) {
		t.Errorf("paragraf judul hilang: %s", html)
	}
	if !strings.Contains(html, `<p>Paragraf dengan gambar <img src="/uploads/`) {
		t.Errorf("gambar tidak disisipkan di posisi paragraf: %s", html)
	}
	if !strings.Contains(html, ` setelah gambar.</p>`) {
		t.Errorf("teks setelah gambar hilang: %s", html)
	}
	if strings.Contains(html, "contoh.test") {
		t.Errorf("hyperlink seharusnya TIDAK dianggap gambar: %s", html)
	}

	files, err := os.ReadDir(uploadDir)
	if err != nil {
		t.Fatalf("baca upload dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("harap 1 berkas tersimpan, dapat %d", len(files))
	}
	if _, err := os.Stat(filepath.Join(uploadDir, files[0].Name())); err != nil {
		t.Fatalf("berkas gambar tidak ada: %v", err)
	}
}
