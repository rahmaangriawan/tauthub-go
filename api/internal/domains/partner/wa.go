package partner

// Aturan "wajib nomor WhatsApp": mitra yang punya LEBIH DARI waMediaThreshold
// media harus mengisi nomor WA + centang izin dihubungi (notification_preferences
// per user_id, migrasi 112) sebelum boleh menambah media lagi. Media yang sudah
// ada tidak diganggu; tangga pengaman untuk mitra lama ada di sisi admin
// (pengingat + tanda merah + konfirmasi saat ACC).

import (
	"context"
	"net/http"
	"strings"
)

// waMediaThreshold: jumlah media yang masih boleh tanpa nomor WA. Media
// ke-(waMediaThreshold+1) dan seterusnya wajib punya kontak WA.
const waMediaThreshold = 2

// waPreferences: nomor WA + izin dihubungi milik user mitra ("" bila belum ada).
func (d *Deps) waPreferences(ctx context.Context, userID string) (string, bool) {
	if strings.TrimSpace(userID) == "" {
		return "", false
	}
	row, err := d.queryRow(ctx,
		"SELECT wa_number, wa_consent FROM notification_preferences WHERE user_id = $1 LIMIT 1", userID)
	if err != nil || row == nil {
		return "", false
	}
	return strings.TrimSpace(row.strOr("wa_number", "")), row.boolOr("wa_consent", false)
}

// partnerMediaCount: jumlah media mitra kecuali yang disembunyikan — definisi
// yang sama dengan angka "N media" di /admin/partners.
func (d *Deps) partnerMediaCount(ctx context.Context, partnerID string) int {
	row, err := d.queryRow(ctx,
		"SELECT COUNT(*) AS total FROM media WHERE partner_id = $1 AND status <> 'hidden'", partnerID)
	if err != nil || row == nil {
		return 0
	}
	return row.intOr("total", 0)
}

// waRequirementMissing: true bila penambahan `adding` media akan membuat total
// media mitra melewati ambang DAN kontak WA-nya belum lengkap.
func (d *Deps) waRequirementMissing(ctx context.Context, userID, partnerID string, adding int) bool {
	number, consent := d.waPreferences(ctx, userID)
	if number != "" && consent {
		return false // sudah lengkap
	}
	return d.partnerMediaCount(ctx, partnerID)+adding > waMediaThreshold
}

// waRequiredMessage: pesan seragam untuk mitra (dipakai MediaCreate & MediaImport).
const waRequiredMessage = "Mitra dengan lebih dari 2 media wajib mengisi nomor WhatsApp dan mencentang izin dihubungi. " +
	"Isi dulu di Profil → tab Notifikasi, lalu ulangi."

// writeWARequired: balasan 422 dengan kode mesin supaya UI bisa mengarahkan
// mitra langsung ke pengisian nomor (bukan error mentah).
func writeWARequired(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"code":   "wa_required",
		"detail": waRequiredMessage,
	})
}
