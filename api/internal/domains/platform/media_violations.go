package platform

import (
	"net/http"
	"strings"
)

// Pelanggaran media + penangguhan (suspend) domain.
//
// Keduanya dipakai panel detail media di /admin/media:
//   - GET  /v1/platform/media/{mediaId}/violations   → riwayat + tingkat terakhir
//   - POST /v1/platform/media/{mediaId}/violations   → catat pelanggaran
//   - POST /v1/platform/media/{mediaId}/violations/revoke → cabut pelanggaran aktif
//   - POST /v1/platform/media/{mediaId}/suspend      → sembunyikan dari katalog
//   - POST /v1/platform/media/{mediaId}/unsuspend    → pulihkan
//
// Suspend sengaja TIDAK memakai kolom status: status 'hidden' bisa diubah mitra
// lewat halaman media mereka, sedangkan suspend harus kendali admin. Penyaringan
// katalog membaca kolom suspended_at.

// mpViolationLevels: tingkat pelanggaran yang diterima API.
var mpViolationLevels = []string{"none", "ringan", "berat"}

// mpMediaViolationsJSON: bentuk JSON satu baris pelanggaran.
func mpMediaViolationJSON(row ptRow) map[string]any {
	out := map[string]any{
		"id":          row.anyOrNil("id"),
		"media_id":    row.anyOrNil("media_id"),
		"level":       row.strOr("level", "none"),
		"note":        ptNullableStr(row, "note"),
		"created_by":  row.anyOrNil("created_by"),
		"created_at":  row.anyOrNil("created_at"),
		"revoked_at":  row.anyOrNil("revoked_at"),
		"revoked_by":  row.anyOrNil("revoked_by"),
		"actor_email": ptNullableStr(row, "actor_email"),
	}
	return out
}

// MediaViolationsList: GET /v1/platform/media/{mediaId}/violations
func (d *Deps) MediaViolationsList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.mpScopedMedia(r.Context(), mediaID); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	// media_id bertipe character(36), sedangkan users.id bertipe uuid — jangan
	// dibandingkan langsung. Cast eksplisit supaya join & filter tidak error
	// "operator does not exist: character = uuid".
	rows, err := d.queryRows(r.Context(), `
		SELECT v.id, v.media_id, v.level, v.note, v.created_by, v.created_at,
		       v.revoked_at, v.revoked_by, u.email AS actor_email
		FROM media_violations v
		LEFT JOIN users u ON u.id::text = v.created_by::text
		WHERE v.media_id = $1::text
		ORDER BY v.created_at DESC
		LIMIT 100`, mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	// Tingkat yang berlaku = pelanggaran aktif (belum dicabut) paling baru.
	activeLevel := "none"
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		items = append(items, mpMediaViolationJSON(row))
		if activeLevel == "none" && !row.has("revoked_at") {
			activeLevel = row.strOr("level", "none")
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":        items,
		"active_level": activeLevel,
		"total":        len(items),
	})
}

// MediaViolationCreate: POST /v1/platform/media/{mediaId}/violations
//
// body: { level: "none"|"ringan"|"berat", note?: string }
//
// level "none" berarti pelanggaran dicabut: baris aktif ditandai revoked,
// bukan dihapus, supaya jejaknya tetap ada.
func (d *Deps) MediaViolationCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.mpScopedMedia(r.Context(), mediaID); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	body := decodeJSON(r)
	level, hasLevel := strField(body, "level")
	if !hasLevel || !mpContains(mpViolationLevels, level) {
		writeErr(w, d, errUnprocessable(mpZEnum(mpViolationLevels)))
		return
	}
	note, hasNote := strField(body, "note")
	if hasNote && len(note) > 2000 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=2000 characters"))
		return
	}
	// Pelanggaran ringan/berat wajib punya keterangan: tanpa itu admin lain
	// tidak tahu apa yang terjadi saat meninjau riwayat.
	if level != "none" && strings.TrimSpace(note) == "" {
		writeErr(w, d, errUnprocessable("Keterangan pelanggaran wajib diisi."))
		return
	}

	if level == "none" {
		// Cabut semua pelanggaran aktif untuk media ini.
		if err := d.exec(r.Context(), `
			UPDATE media_violations
			SET revoked_at = NOW(), revoked_by = $2
			WHERE media_id = $1::text AND revoked_at IS NULL`, mediaID, user.ID); err != nil {
			writeErr(w, d, err)
			return
		}
		writeAuditBestEffort(d, r.Context(), user.ID, "media.violation_revoked", "media", mediaID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active_level": "none"})
		return
	}

	var id string
	row, err := d.queryRow(r.Context(), `
		INSERT INTO media_violations (id, media_id, level, note, created_by)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
		RETURNING id`, mediaID, level, strings.TrimSpace(note), user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row != nil {
		id = row.strOr("id", "")
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "media.violation_recorded", "media", mediaID)
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": id, "active_level": level})
}

// MediaSuspend: POST /v1/platform/media/{mediaId}/suspend
//
// body: { reason: string }
//
// Menyembunyikan media dari katalog klien & mitra dan menolak publikasi baru.
// Data, riwayat, dan harga tidak disentuh — pencabutan mengembalikan semula.
func (d *Deps) MediaSuspend(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.mpScopedMedia(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	body := decodeJSON(r)
	reason, _ := strField(body, "reason")
	reason = strings.TrimSpace(reason)
	if reason == "" {
		writeErr(w, d, errUnprocessable("Alasan penangguhan wajib diisi."))
		return
	}
	if len(reason) > 1000 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=1000 characters"))
		return
	}
	if media.has("suspended_at") {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "already_suspended": true})
		return
	}
	if err := d.exec(r.Context(), `
		UPDATE media SET suspended_at = NOW(), suspended_reason = $2, suspended_by = $3, updated_at = NOW()
		WHERE id = $1`, mediaID, reason, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	// Publikasi yang belum terbit dibatalkan supaya tidak ada artikel yang lolos
	// ke domain bermasalah setelah suspend. Yang sudah terbit dibiarkan (tidak
	// bisa ditarik kembali dari situs mitra).
	if err := d.exec(r.Context(), `
		UPDATE publications
		SET status = 'cancelled', updated_at = NOW()
		WHERE media_id = $1 AND status IN ('pending','waiting_partner','partner_approved','queued','processing','retrying','revision')`,
		mediaID); err != nil {
		d.logWarn("batalkan publikasi saat suspend gagal", mediaID, err)
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "media.suspended", "media", mediaID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "suspended_at": d.isoNow()})
}

// MediaUnsuspend: POST /v1/platform/media/{mediaId}/unsuspend
func (d *Deps) MediaUnsuspend(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.mpScopedMedia(r.Context(), mediaID); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	if err := d.exec(r.Context(), `
		UPDATE media
		SET suspended_at = NULL, suspended_reason = NULL, suspended_by = NULL, updated_at = NOW()
		WHERE id = $1`, mediaID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "media.unsuspended", "media", mediaID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
