package brandflow

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// util.go: helper kecil untuk upload berkas + JSON + HTTP.

func jsonUnmarshalStd(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
}

// uploadDir: parity UPLOAD_DIR (.data/uploads).
func (d *Deps) uploadDir() string {
	if v := strings.TrimSpace(os.Getenv("UPLOAD_DIR")); v != "" {
		return v
	}
	return filepath.Join(".data", "uploads")
}

// saveUpload: tulis berkas ke UPLOAD_DIR (buat direktori bila perlu).
func (d *Deps) saveUpload(storageKey string, data []byte) error {
	dir := d.uploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, storageKey), data, 0o644)
}

// readAllLimited: baca body multipart dengan batas ukuran.
func readAllLimited(r io.Reader, max int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, max+1))
}

// sanitizeExt: ekstensi aman dari nama berkas.
func sanitizeExt(nameLower, fallback string) string {
	idx := strings.LastIndex(nameLower, ".")
	if idx < 0 || idx == len(nameLower)-1 {
		return fallback
	}
	ext := strings.TrimSpace(nameLower[idx+1:])
	if len(ext) == 0 || len(ext) > 5 {
		return fallback
	}
	for i := 0; i < len(ext); i++ {
		c := ext[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return fallback
		}
	}
	return ext
}

// formFile: ambil file dari multipart form (nama field "file").
func formFile(r *http.Request, field string) (io.ReadCloser, string, int64, string, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, "", 0, "", err
	}
	file, header, err := r.FormFile(field)
	if err != nil {
		return nil, "", 0, "", err
	}
	return file, header.Filename, header.Size, header.Header.Get("Content-Type"), nil
}
