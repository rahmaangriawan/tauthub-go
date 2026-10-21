package db

import (
	"strings"
	"testing"
)

// Predikat garansi tayang harus mengikuti kebijakan media: 0 = permanen
// (tidak pernah kedaluwarsa), >0 = jumlah bulan sejak tanggal tayang.
// Tanpa ini semua media kembali dianggap bergaransi 12 bulan.
func TestLinkWarrantySQL(t *testing.T) {
	got := LinkWarrantySQL("p", "m")
	if !strings.Contains(got, "m.link_duration_months = 0") {
		t.Fatalf("predikat harus memperlakukan 0 sebagai permanen: %s", got)
	}
	if !strings.Contains(got, "make_interval(months => m.link_duration_months)") {
		t.Fatalf("predikat harus memakai durasi media, bukan interval tetap: %s", got)
	}
	if strings.Contains(got, "12 months") {
		t.Fatalf("predikat tidak boleh lagi mematok 12 bulan: %s", got)
	}
	if !strings.Contains(got, "p.published_at") {
		t.Fatalf("predikat harus memakai tanggal tayang publikasi: %s", got)
	}
}

func TestLinkWarrantyMonthsSQL(t *testing.T) {
	if got := LinkWarrantyMonthsSQL("p", "m"); got != "COALESCE(m.link_duration_months, 12)" {
		t.Fatalf("masa garansi efektif salah: %s", got)
	}
}
