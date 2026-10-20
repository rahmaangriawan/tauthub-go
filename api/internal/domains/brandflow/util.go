package brandflow

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

// uploadKeyFrom: nama berkas ramah-SEO dari petunjuk teks (judul artikel,
// prompt AI, atau nama asli berkas) + sufiks hash pendek, lalu -2/-3/... bila
// nama sudah ada di disk. Slug kosong (mis. nama non-latin) → "berkas".
func (d *Deps) uploadKeyFrom(hint, ext string) string {
	slug := slugifyPlain(hint)
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	if slug == "" {
		slug = "berkas"
	}
	suffix := strconv.FormatUint(uint64(fnv32(hint)), 16)
	if len(suffix) < 8 {
		suffix = "00000000"[:8-len(suffix)] + suffix
	}
	base := slug + "-" + suffix
	key := base + "." + ext
	for n := 2; n < 1000; n++ {
		_, err := os.Stat(filepath.Join(d.uploadDir(), key))
		if errors.Is(err, fs.ErrNotExist) {
			return key
		}
		if err != nil {
			// Stat gagal karena hal lain (izin, dsb) — pakai nama hash UUID
			// supaya berkas tidak mungkin menimpa unggahan lain.
			return newUUID() + "." + ext
		}
		key = base + "-" + strconv.Itoa(n) + "." + ext
	}
	return newUUID() + "." + ext
}

// firstWords: n kata pertama (nama berkas gambar AI dari prompt).
func firstWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) > n {
		fields = fields[:n]
	}
	return strings.Join(fields, " ")
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
