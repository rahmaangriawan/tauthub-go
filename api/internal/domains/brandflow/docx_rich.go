package brandflow

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"html"
	"io"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/imageconvert"
)

// docx_rich.go: ekstraksi DOCX yang MEMPERTAHANKAN gambar embedded.
//
// extractDocument() hanya mengambil teks, jadi gambar di dalam dokumen klien
// hilang sebelum masuk editor. Fungsi di sini membongkar arsip DOCX lagi:
// membaca relasi (word/_rels/document.xml.rels) untuk memetakan rId -> berkas
// di word/media, menyimpan tiap gambar ke uploads Tauthub, lalu menaruh <img>
// tepat di posisi aslinya. Hasilnya dokumen pindah ke editor "sama persis"
// (teks + gambar) dan gambarnya ikut terbawa saat distribusi (karena sudah
// menunjuk ke /uploads Tauthub, bukan ke berkas lokal klien).

// docxRasterMimes: format gambar yang aman ditampilkan browser. EMF/WMF/SVG
// (sering muncul di Word) dilewati supaya editor tidak menampilkan gambar rusak.
var docxRasterMimes = map[string]string{
	"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg",
	"gif": "image/gif", "webp": "image/webp",
}

type docxRel struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
}

type docxRelsRoot struct {
	Rels []docxRel `xml:"Relationship"`
}

// extractDocxRich: hasil HTML (paragraf + gambar) dan jumlah gambar yang
// berhasil dipindahkan. err != nil berarti DOCX tidak bisa dibaca.
func (d *Deps) extractDocxRich(buffer []byte) (string, int, error) {
	zr, err := zip.NewReader(bytes.NewReader(buffer), int64(len(buffer)))
	if err != nil {
		return "", 0, errStatus(422, "DOCX tidak dapat dibuka — arsipnya rusak atau bukan berkas Office yang valid.")
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[f.Name] = f
	}
	docFile := files["word/document.xml"]
	if docFile == nil {
		return "", 0, errStatus(422, "DOCX tidak berisi word/document.xml — kemungkinan bukan dokumen Word.")
	}
	rels := docxMediaRels(readZipBytes(files["word/_rels/document.xml.rels"], 4*1024*1024))

	xmlBytes := readZipBytes(docFile, 32*1024*1024)
	if xmlBytes == nil {
		return "", 0, errStatus(422, "Isi DOCX tidak dapat dibaca.")
	}
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	dec.Strict = false

	var (
		htmlSB strings.Builder
		paraSB strings.Builder
		images int
	)
	flushPara := func() {
		content := strings.TrimSpace(paraSB.String())
		paraSB.Reset()
		if content == "" {
			return
		}
		htmlSB.WriteString("<p>")
		htmlSB.WriteString(content)
		htmlSB.WriteString("</p>")
	}
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", 0, errStatus(422, "Struktur XML di dalam DOCX tidak valid.")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "br":
				paraSB.WriteString("<br>")
			case "tab":
				paraSB.WriteString("&nbsp;&nbsp;")
			}
			for _, a := range t.Attr {
				// a:blip r:embed / v:imagedata r:id — ditapis lewat rels: hanya
				// relasi yang menunjuk berkas media/ yang dianggap gambar (relasi
				// hyperlink menunjuk URL, jadi otomatis terlewat).
				if a.Name.Local != "embed" && a.Name.Local != "id" && a.Name.Local != "relid" {
					continue
				}
				if !strings.HasPrefix(a.Value, "rId") {
					continue
				}
				target, ok := rels[a.Value]
				if !ok {
					continue
				}
				data, mime, ok := readDocxMedia(files, target)
				if !ok {
					continue
				}
				url, sErr := d.storeDocImage(data, mime)
				if sErr != nil {
					continue
				}
				images++
				paraSB.WriteString(`<img src="` + url + `" alt="">`)
			}
		case xml.CharData:
			if inText {
				paraSB.WriteString(html.EscapeString(string(t)))
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				flushPara()
			}
		}
	}
	flushPara()
	return htmlSB.String(), images, nil
}

// docxMediaRels: peta rId -> target, HANYA untuk relasi yang menunjuk media/
// (gambar embedded). Target lain (hyperlink eksternal, dll.) diabaikan.
func docxMediaRels(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var root docxRelsRoot
	if err := xml.Unmarshal(raw, &root); err != nil {
		return out
	}
	for _, rel := range root.Rels {
		if strings.HasPrefix(strings.TrimPrefix(rel.Target, "/"), "media/") || strings.Contains(rel.Target, "/media/") {
			out[rel.ID] = rel.Target
		}
	}
	return out
}

// readDocxMedia: ambil byte + MIME gambar dari target relasi. Hanya format
// raster yang didukung (lihat docxRasterMimes).
func readDocxMedia(files map[string]*zip.File, target string) ([]byte, string, bool) {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(target), "."))
	mime, ok := docxRasterMimes[ext]
	if !ok {
		return nil, "", false
	}
	name := strings.TrimPrefix(target, "/")
	name = strings.TrimPrefix(name, "word/")
	data := readZipBytes(files["word/"+name], 15*1024*1024)
	if data == nil {
		return nil, "", false
	}
	return data, mime, true
}

// storeDocImage: simpan gambar dokumen ke uploads Tauthub (diubah ke WebP bila
// bisa), kembalikan URL publik /uploads/... yang ikut terbawa saat distribusi.
func (d *Deps) storeDocImage(data []byte, mime string) (string, error) {
	ext := imageExtForMime(mime)
	if ext != "webp" {
		if converted, ok := imageconvert.ConvertToWebP(data, mime); ok {
			data = converted
			ext = "webp"
		}
	}
	key := uuid.NewString() + "." + ext
	if err := d.saveUpload(key, data); err != nil {
		return "", err
	}
	return "/uploads/" + key, nil
}

func readZipBytes(f *zip.File, limit int64) []byte {
	if f == nil {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, limit))
	if err != nil {
		return nil
	}
	return b
}
