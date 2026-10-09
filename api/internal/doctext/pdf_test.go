package doctext

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// buildMinimalPDF: PDF satu halaman berisi satu baris teks, lengkap dengan xref
// yang offset-nya dihitung supaya parser membacanya sebagai PDF valid.
func buildMinimalPDF(text string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, 5)
	obj := func(s string) {
		offsets = append(offsets, b.Len())
		b.WriteString(s)
	}
	obj("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	obj("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	obj("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n")
	content := "BT /F1 14 Tf 20 100 Td (" + text + ") Tj ET"
	obj(fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content))
	obj("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	xrefPos := b.Len()
	b.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for _, off := range offsets {
		b.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}
	b.WriteString(fmt.Sprintf("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefPos))
	return b.Bytes()
}

func TestFromPDFExtractsText(t *testing.T) {
	got, err := FromPDF(buildMinimalPDF("Halo Dunia Tauthub"))
	if err != nil {
		t.Fatalf("FromPDF gagal: %v", err)
	}
	flat := strings.Join(strings.Fields(got), " ")
	if !strings.Contains(flat, "Halo") || !strings.Contains(flat, "Tauthub") {
		t.Errorf("teks PDF tidak terbaca; hasil=%q", got)
	}
}

func TestFromPDFRejectsGarbage(t *testing.T) {
	if _, err := FromPDF([]byte("bukan pdf sama sekali")); err == nil {
		t.Error("berkas bukan PDF harus mengembalikan error")
	}
	if _, err := FromPDF(nil); err == nil {
		t.Error("berkas kosong harus mengembalikan error")
	}
}
