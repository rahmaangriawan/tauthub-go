package db

import "fmt"

// LinkWarrantySQL: predikat SQL "artikel masih dalam masa garansi tayang".
//
// Masa garansi mengikuti kebijakan tiap media (media.link_duration_months):
//   - 0  → tautan permanen, jadi garansinya tidak pernah kedaluwarsa;
//   - >0 → jumlah bulan sejak tanggal tayang (bawaan 12 bulan / "12 Bln SLA").
//
// Query pemanggil WAJIB sudah meng-join tabel media dengan alias `mediaAlias`
// dan tabel publications dengan alias `pubAlias` (dipakai juga oleh pelacak
// 404, sapuan scheduler, dan kelayakan klaim Link Guardian).
func LinkWarrantySQL(pubAlias, mediaAlias string) string {
	return fmt.Sprintf(
		"(%[2]s.link_duration_months = 0 OR %[1]s.published_at > NOW() - make_interval(months => %[2]s.link_duration_months))",
		pubAlias, mediaAlias)
}

// LinkWarrantyMonthsSQL: teks masa garansi efektif sebuah publikasi (untuk
// pesan alasan yang jujur, mis. "permanen" vs "12 bulan").
func LinkWarrantyMonthsSQL(pubAlias, mediaAlias string) string {
	return fmt.Sprintf("COALESCE(%s.link_duration_months, 12)", mediaAlias)
}
