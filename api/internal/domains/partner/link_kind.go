package partner

// existingAllowHyperlink: allow_hyperlink media saat ini; baris lama tanpa nilai
// mengikuti max_links > 0.
func existingAllowHyperlink(existing rowMap) bool {
	allow := existing.intOr("max_links", 1) > 0
	if existing.has("allow_hyperlink") {
		allow = existing.boolOr("allow_hyperlink", allow)
	}
	return allow
}

// followKindLabel: nama jenis tautan untuk ringkasan perubahan.
func followKindLabel(allow bool) string {
	if allow {
		return "Dofollow"
	}
	return "Nofollow"
}

// partnerNextAllowHyperlink: apakah allow_hyperlink perlu ditulis, dan nilainya.
//   - mitra memilih jenis tautan (allow_hyperlink dikirim) → pakai pilihannya,
//     kecuali jumlah backlink 0 (tidak ada tautan sama sekali);
//   - selain itu hanya disetel ulang saat media tadinya 0 backlink atau belum
//     punya nilai, supaya pengaturan nofollow tidak terhapus tiap mitra menyimpan.
func partnerNextAllowHyperlink(existing rowMap, hasMax bool, maxLinks int, hasAllow, allow bool) (bool, bool) {
	if hasAllow && (!hasMax || maxLinks > 0) {
		return true, allow
	}
	if hasMax && (existing.intOr("max_links", 1) <= 0 || !existing.has("allow_hyperlink")) {
		return true, maxLinks > 0
	}
	return false, false
}
