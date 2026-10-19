package partner

import (
	"net/http"
)

// Badge "Publikasi baru" milik mitra: publikasi yang dibuat SETELAH mitra
// terakhir kali membuka halaman Publikasinya (pub_seen_at) terhitung belum
// dibaca. Membuka halamannya → mark-seen → angka kembali 0; publikasi baru
// masuk (distribusi klien / terbit otomatis) → angka muncul lagi di menu.

// PublicationsUnread: GET /v1/partner/publications/unread-count
// Jumlah publikasi milik media partner ini yang belum pernah dilihat.
func (d *Deps) PublicationsUnread(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(), `
		SELECT count(*) AS n
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN partners pt ON pt.id = m.partner_id
		WHERE m.partner_id = $1
		  AND (pt.pub_seen_at IS NULL OR p.created_at > pt.pub_seen_at)`, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread": row.intOr("n", 0)})
}

// PublicationsMarkSeen: POST /v1/partner/publications/mark-seen
// Dipanggil halaman Publikasi setelah daftar terrender → badge hilang.
func (d *Deps) PublicationsMarkSeen(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE partners SET pub_seen_at = now(), updated_at = now() WHERE id = $1", partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
