package partner

import (
	"net/http"
	"strings"
)

// ---------- Pengajuan Secure Direct (mode terbit otomatis) ----------
// Mitra tanpa izin mengajukan lewat tombol "Ajukan Secure Direct" di Daftar
// Media → baris pending menunggu ACC admin → admin mengaktifkan lewat gerbang
// "Izinkan mode otomatis" di /admin/partners (PUT auto_publish_enabled), yang
// menutup pengajuan pending dan memberi tahu mitra.

// SecureDirectStatus: GET /v1/partner/secure-direct/request — status pengajuan
// terakhir mitra (mengisi state popup Ajuan).
func (d *Deps) SecureDirectStatus(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(), `
		SELECT status, note, requested_at, decided_at
		FROM secure_direct_requests
		WHERE partner_id = $1
		ORDER BY requested_at DESC
		LIMIT 1`, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		w.Header().Set("Cache-Control", "private, max-age=5")
		writeJSON(w, http.StatusOK, map[string]any{"request": nil})
		return
	}
	var notePtr *string
	if strings.TrimSpace(row.strOr("note", "")) != "" {
		note := row.strOr("note", "")
		notePtr = &note
	}
	w.Header().Set("Cache-Control", "private, max-age=5")
	writeJSON(w, http.StatusOK, map[string]any{
		"request": map[string]any{
			"status":       row.strOr("status", "pending"),
			"note":         notePtr,
			"requested_at": row.timePtr("requested_at"),
			"decided_at":   row.timePtr("decided_at"),
		},
	})
}

// SecureDirectRequestCreate: POST /v1/partner/secure-direct/request. Satu
// pengajuan pending per partner (unique index); setelah ditolak boleh diajukan
// ulang. Admin diberi notifikasi dengan deep link filtered ?q=<email mitra>.
func (d *Deps) SecureDirectRequestCreate(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if partner.AutoPublishEnabled {
		d.writeErr(w, errConflict("Secure Direct sudah aktif di akun Anda."))
		return
	}
	pending, err := d.queryRow(r.Context(), `
		SELECT 1 AS one FROM secure_direct_requests
		WHERE partner_id = $1 AND status = 'pending' LIMIT 1`, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if pending != nil {
		d.writeErr(w, errConflict("Pengajuan Secure Direct Anda masih menunggu persetujuan admin Tauthub."))
		return
	}
	row, err := d.queryRow(r.Context(), `
		INSERT INTO secure_direct_requests (id, partner_id, status)
		VALUES (gen_random_uuid()::text, $1, 'pending')
		RETURNING requested_at`, partner.ID)
	if err != nil {
		// Kalau kalah race dengan unique index (pdf 23505), artinya sudah ada.
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(err.Error(), "23505") {
			d.writeErr(w, errConflict("Pengajuan Secure Direct Anda masih menunggu persetujuan admin Tauthub."))
			return
		}
		d.writeErr(w, err)
		return
	}
	// Detail mitra untuk notifikasi admin (nama bisa NULL → pakai email).
	detail, derr := d.queryRow(r.Context(), `
		SELECT COALESCE(p.name, '') AS partner_name, u.email AS email
		FROM partners p JOIN users u ON u.id = p.user_id
		WHERE p.id = $1 LIMIT 1`, partner.ID)
	if derr == nil && detail != nil {
		name := strings.TrimSpace(detail.strOr("partner_name", ""))
		if name == "" {
			name = detail.strOr("email", "")
		}
		title := "Pengajuan Secure Direct baru: " + name
		body := name + " mengajukan fitur Secure Direct (terbit otomatis). Buka halaman Mitra, lalu lewat menu titik-tiga pilih 'Izinkan mode otomatis'."
		link := "/admin/partners?q=" + detail.strOr("email", "")
		d.notifyPlatformAdmins(r.Context(), "secure_direct.requested", title, body, &link)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"request": map[string]any{
			"status":       "pending",
			"note":         nil,
			"requested_at": row.timePtr("requested_at"),
			"decided_at":   nil,
		},
	})
}
