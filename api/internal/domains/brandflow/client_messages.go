package brandflow

// Kalimat untuk klien dari kode kegagalan percobaan publikasi. Klien tidak
// perlu tahu penyebab teknisnya (URL gambar, nama platform, koneksi mitra), jadi
// semua penyebab dipetakan ke kalimat biasa. Pesan teknis asli tetap tersimpan
// di publication_attempts untuk admin dan mitra.

// clientFailureText mengembalikan kalimat netral untuk satu kode kegagalan.
// Kode yang tidak dikenal memakai kalimat umum, tidak pernah error mentah.
func clientFailureText(code string) string {
	switch code {
	case "featured_image_upload_failed", "blogger_image_upload_failed":
		return "Gambar artikel ini belum bisa diterbitkan. Tim kami sedang menindaklanjuti."
	case "no_connection", "connection_secret_missing", "partner_not_ready",
		"blogger_config_missing", "blogger_credential_expired", "blogger_not_configured",
		"adapter_unreachable", "adapter_not_accepted", "media_missing":
		return "Media tujuan belum bisa menerima artikel ini. Tim kami sedang menindaklanjuti."
	case "link_policy":
		return "Artikel ini belum memenuhi aturan tautan media. Silakan periksa tautan di dalam artikel."
	case "moderation_blocked", "moderation_review_required", "review_required", "blocked":
		return "Artikel ini perlu ditinjau lebih dulu sebelum bisa terbit. Tim kami akan menghubungi Anda."
	case "document_missing", "version_missing":
		return "Naskah artikel tidak ditemukan. Silakan hubungi dukungan."
	default:
		return "Artikel belum bisa diterbitkan. Tim kami sedang menindaklanjuti."
	}
}
