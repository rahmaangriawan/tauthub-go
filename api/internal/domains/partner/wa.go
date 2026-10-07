package partner

// Gerbang mitra sebelum boleh menambah media:
//
//  1. Impor massal (XLSX) hanya terbuka setelah mitra membuktikan kepemilikan
//     satu domain: ada media yang SUDAH DISETUJUI admin (status 'active') dan
//     domainnya terverifikasi (meta verifikasi / TXT, lihat bulkMediaUnlocked).
//  2. Aturan "wajib nomor WhatsApp": mitra yang punya LEBIH DARI waMediaThreshold
//     media harus mengisi nomor WA + centang izin dihubungi (notification_preferences
//     per user_id, migrasi 112) sebelum boleh menambah media lagi.
//
// Media yang sudah ada tidak diganggu; tangga pengaman untuk mitra lama ada di
// sisi admin (pengingat + tanda merah + konfirmasi saat ACC).

import (
	"context"
	"net/http"
	"strings"
)

// waMediaThreshold: jumlah media yang masih boleh tanpa nomor WA. Media
// ke-(waMediaThreshold+1) dan seterusnya wajib punya kontak WA. Nilainya 1
// supaya media PERTAMA tetap bebas (itu pintu masuk mitra baru) dan setiap
// penambahan berikutnya sudah memerlukan kontak yang bisa dihubungi.
const waMediaThreshold = 1

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
	return waExceededBy(d.partnerMediaCount(ctx, partnerID), adding)
}

// waExceededBy: aritmetika ambang, dipisah dari query supaya bisa diuji tanpa
// database. Dengan waMediaThreshold = 1: media pertama lolos, setiap penambahan
// berikutnya kena.
func waExceededBy(existingCount, adding int) bool {
	return existingCount+adding > waMediaThreshold
}

// waRequiredMessage: pesan seragam untuk mitra (dipakai MediaCreate & MediaImport).
const waRequiredMessage = "Menambah media kedua dan seterusnya wajib mengisi nomor WhatsApp dan mencentang izin dihubungi. " +
	"Isi dulu di Profil → tab Notifikasi, lalu ulangi."

// writeWARequired: balasan 422 dengan kode mesin supaya UI bisa mengarahkan
// mitra langsung ke pengisian nomor (bukan error mentah).
func writeWARequired(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"code":   "wa_required",
		"detail": waRequiredMessage,
	})
}

// bulkMediaUnlocked: input massal (Impor XLSX) hanya terbuka setelah mitra
// membuktikan kepemilikan satu domain — ada media yang sudah disetujui admin
// (status 'active') DAN domainnya terverifikasi. Tujuannya menahan agensi atau
// penulis yang mengunggah domain milik orang lain secara borongan: mitra baru
// mengajukan satu media dulu, lalu syaratnya terbuka sendiri tanpa perlu admin
// menyalakan apa pun per mitra.
//
// Syaratnya dihitung ulang setiap kali, jadi kalau verifikasi domain dicabut
// (tiga kali gagal berturut-turut) impor massal ikut terkunci lagi.
func (d *Deps) bulkMediaUnlocked(ctx context.Context, partnerID string) bool {
	if strings.TrimSpace(partnerID) == "" {
		return false
	}
	row, err := d.queryRow(ctx, `
		SELECT COUNT(*) AS total FROM media
		WHERE partner_id = $1 AND status = 'active' AND domain_verified = TRUE`, partnerID)
	if err != nil || row == nil {
		return false
	}
	return row.intOr("total", 0) > 0
}

// bulkLockedMessage: arahan langkah demi langkah, bukan sekadar "ditolak".
const bulkLockedMessage = "Impor massal belum terbuka. Ajukan satu media dulu lewat \"Tambah Media\", " +
	"tunggu persetujuan admin, lalu buka \"Verifikasi domain\" pada media itu dan pasang kode verifikasi " +
	"di situs Anda. Setelah domain pertama terverifikasi, impor massal aktif dengan sendirinya."

// writeBulkLocked: 422 dengan kode mesin supaya UI bisa menampilkan panduan
// alih-alih error mentah.
func writeBulkLocked(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"code":   "bulk_locked",
		"detail": bulkLockedMessage,
	})
}
