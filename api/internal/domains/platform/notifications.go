package platform

// Notifikasi dashboard admin dari tabel `notifications` (per user admin yang
// login) — pasangan dari endpoint klien/mitra, supaya admin punya RIWAYAT, bukan
// hanya feed masalah "saat ini" (/platform/operations/notifications).
//
// Guard: adminGuard (khusus admin platform). Semua query disaring user_id admin
// yang sedang login, jadi satu admin tidak bisa membaca notifikasi admin lain.

import (
	"net/http"
	"strings"
)

// plumNotificationJSON: bentuk JSON yang sama dengan endpoint klien/mitra agar
// komponen daftar bisa dipakai ulang.
func plumNotificationJSON(row rowMap) map[string]any {
	var link any = row.raw("link")
	// Notifikasi Secure Direct lama hanya membawa ?q=<email>; arahkan ke tab
	// "Pengajuan Secure Direct" tanpa harus mengubah baris di DB.
	if row.strOr("type", "") == "secure_direct.requested" {
		if s := row.strOr("link", ""); strings.HasPrefix(s, "/admin/partners?q=") {
			link = "/admin/partners?tab=secure-direct&q=" + strings.TrimPrefix(s, "/admin/partners?q=")
		}
	}
	return map[string]any{
		"id":             row.strOr("id", ""),
		"type":           row.strOr("type", ""),
		"title":          row.strOr("title", ""),
		"body":           row.raw("body"),
		"link":           link,
		"publication_id": row.raw("publication_id"),
		"read_at":        row.timePtr("read_at"),
		"expired_at":     row.timePtr("expired_at"),
		"created_at":     row.timePtr("created_at"),
	}
}

// AdminNotifications: GET daftar riwayat (filter status + kategori), POST tandai
// semua dibaca. Filter kategori memakai prefiks type: announcement.* = pengumuman.
func (d *Deps) AdminNotifications(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := d.exec(ctx,
			"UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE user_id = $1 AND read_at IS NULL", user.ID); err != nil {
			writeErr(w, d, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "all" && status != "read" && status != "unread" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "Invalid status filter"})
		return
	}
	category := strings.TrimSpace(q.Get("category"))
	if category != "" && category != "general" && category != "announcement" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "Invalid category filter"})
		return
	}
	limit := clampInt(q.Get("limit"), 30, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	// Hitungan unread mengikuti kategori bila tab aktif mengirimkannya.
	unreadWhere := " AND read_at IS NULL AND expired_at IS NULL"
	switch category {
	case "announcement":
		unreadWhere += " AND type LIKE 'announcement%'"
	case "general":
		unreadWhere += " AND type NOT LIKE 'announcement%'"
	}
	unread := 0
	if row, err := d.queryRow(ctx,
		"SELECT count(*) AS count FROM notifications WHERE user_id = $1"+unreadWhere, user.ID); err == nil && row != nil {
		unread = row.intOr("count", 0)
	}

	where := ""
	switch status {
	case "read":
		where = " AND read_at IS NOT NULL"
	case "unread":
		where = " AND read_at IS NULL AND expired_at IS NULL"
	}
	switch category {
	case "announcement":
		where += " AND type LIKE 'announcement%'"
	case "general":
		where += " AND type NOT LIKE 'announcement%'"
	}

	total := 0
	if row, err := d.queryRow(ctx,
		"SELECT count(*) AS count FROM notifications WHERE user_id = $1"+where, user.ID); err == nil && row != nil {
		total = row.intOr("count", 0)
	}
	rows, err := d.queryRows(ctx,
		"SELECT * FROM notifications WHERE user_id = $1"+where+" ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		user.ID, limit, offset)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, plumNotificationJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": items,
		"unread":        unread,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// AdminNotificationRead: POST /platform/notifications/{notificationId}/read —
// tandai satu notifikasi milik admin ini sebagai dibaca (idempoten).
func (d *Deps) AdminNotificationRead(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	notificationID := chiParam(r, "notificationId")
	if err := d.exec(r.Context(),
		"UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE id = $1 AND user_id = $2",
		notificationID, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
