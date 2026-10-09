// Package doctext: ekstraksi teks dokumen yang dipakai lintas domain
// (Profiler, unggah dokumen Taut Pintar). Parser pihak ketiga hanya dipakai
// untuk PDF; DOCX/TXT diparsing dengan stdlib di pemanggilnya.
package doctext

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// maxPDFText: batas pengaman teks yang diekstrak dari PDF.
const maxPDFText = 32 << 20

// FromPDF: ambil seluruh teks dari berkas PDF.
//
// Library github.com/ledongthuc/pdf dipilih karena murni Go (tanpa cgo) dan
// khusus ekstraksi teks. PDF hasil pemindaian (gambar) tidak punya lapisan
// teks → hasilnya kosong dan dikembalikan sebagai error supaya pemanggil
// menandainya "gagal" (bukan diam-diam jadi knowledge kosong).
func FromPDF(buffer []byte) (string, error) {
	if len(buffer) == 0 {
		return "", errors.New("Berkas PDF kosong.")
	}
	reader, err := pdf.NewReader(bytes.NewReader(buffer), int64(len(buffer)))
	if err != nil {
		return "", errors.New("PDF tidak dapat dibaca — berkas rusak atau terproteksi.")
	}
	plain, err := reader.GetPlainText()
	if err != nil {
		return "", errors.New("Isi teks PDF tidak dapat diekstrak.")
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(plain, maxPDFText)); err != nil {
		return "", errors.New("Gagal membaca teks PDF.")
	}
	text := strings.TrimSpace(buf.String())
	if text == "" {
		return "", errors.New("PDF tidak berisi teks yang bisa diekstrak (kemungkinan hasil pemindaian/gambar).")
	}
	return text, nil
}
