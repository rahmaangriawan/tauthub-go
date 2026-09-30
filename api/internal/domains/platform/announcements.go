package platform

// Modul pengumuman: broadcast admin ke pengguna klien/mitra + status dibaca
// per user (tabel announcements / announcement_reads, migrasi 172). Sisi user
// hidup di /v1/announcements (semua role), sisi admin di /v1/platform/announcements.

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Batasan konten: judul & isi pengumuman, dan jumlah item yang dikirim ke user.
const (
	annTitleMin = 5
	annTitleMax = 120
	annBodyMin  = 5
	annBodyMax  = 4000
	annListMax  = 30
)

var annAudienceSet = map[string]bool{"all": true, "client": true, "partner": true}

var annNotFoundErr = errors.New("Pengumuman tidak ditemukan.")

// annAdminJSON: bentuk JSON pengumuman untuk sisi admin.
func annAdminJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":              row.strOr("id", ""),
		"title":           row.strOr("title", ""),
		"body":            row.strOr("body", ""),
		"audience":        row.strOr("audience", "all"),
		"author_name":     row.strPtr("author_name"),
		"read_count":      row.intOr("read_count", 0),
		"recipient_count": row.intOr("recipient_count", 0),
		"created_at":      row.timePtr("created_at"),
	}
}

// annVisibleSQL: predikat visibilitas untuk satu role ($n diganti pemanggil).
// 'all' berarti klien + mitra; admin melihat semuanya.
func annVisibleSQL(roleParam string) string {
	return "((a.audience = 'all' AND " + roleParam + " IN ('client','partner')) OR a.audience = " + roleParam + " OR " + roleParam + " = 'admin')"
}

// ---------- sisi pengguna: /v1/announcements ----------

// AnnouncementsList: kotak pengumuman untuk pengguna yang sedang login.
// Audiens disaring per role; item belum dibaca diberi tanda + hitungan unread.
func (d *Deps) AnnouncementsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT a.id, a.title, a.body, a.audience, a.created_at, ar.read_at
		FROM announcements a
		LEFT JOIN announcement_reads ar ON ar.announcement_id = a.id AND ar.user_id = $1
		WHERE `+annVisibleSQL("$2")+`
		ORDER BY a.created_at DESC
		LIMIT $3`, user.ID, user.Role, annListMax)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	unreadRow, err := d.queryRow(r.Context(), `
		SELECT count(*) AS total FROM announcements a
		WHERE `+annVisibleSQL("$1")+`
		  AND NOT EXISTS (SELECT 1 FROM announcement_reads ar WHERE ar.announcement_id = a.id AND ar.user_id = $2)`,
		user.Role, user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":         row.strOr("id", ""),
			"title":      row.strOr("title", ""),
			"body":       row.strOr("body", ""),
			"audience":   row.strOr("audience", "all"),
			"created_at": row.timePtr("created_at"),
			"read_at":    row.timePtr("read_at"),
		})
	}
	unread := 0
	if unreadRow != nil {
		unread = unreadRow.intOr("total", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "unread": unread})
}

// AnnouncementMarkRead: tandai satu pengumuman sudah dibaca (idempoten).
func (d *Deps) AnnouncementMarkRead(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	announcementID := chiParam(r, "announcementId")
	// Cek visibilitas dulu supaya konflik "sudah pernah dibaca" tidak terbaca
	// sebagai 404 — klik berulang tetap 200.
	seen, err := d.queryRow(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM announcements a WHERE a.id = $1 AND `+annVisibleSQL("$2")+`
		) AS visible`, announcementID, user.Role)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if seen == nil || !seen.has("visible") || !truthy(seen.raw("visible")) {
		httpx.Error(w, http.StatusNotFound, annNotFoundErr.Error())
		return
	}
	if err := d.exec(r.Context(), `
		INSERT INTO announcement_reads (announcement_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (announcement_id, user_id) DO NOTHING`, announcementID, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// truthy: boolean dari EXISTS (bisa datang sebagai bool atau string driver).
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "t"
	default:
		return false
	}
}

// ---------- sisi admin: /v1/platform/announcements ----------

// AdminAnnouncementsList: daftar pengumuman yang pernah dikirim + statistik
// dibaca (sudah dibaca vs penerima aktif sesuai audiens).
func (d *Deps) AdminAnnouncementsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT a.id, a.title, a.body, a.audience, a.created_at, u.display_name AS author_name,
		       (SELECT count(*) FROM announcement_reads ar WHERE ar.announcement_id = a.id) AS read_count,
		       (SELECT count(*) FROM users x WHERE x.is_active = TRUE
		           AND ((a.audience = 'all' AND x.role IN ('client','partner','admin')) OR x.role = a.audience)) AS recipient_count
		FROM announcements a
		LEFT JOIN users u ON u.id = a.created_by
		ORDER BY a.created_at DESC
		LIMIT 100`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, annAdminJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AdminAnnouncementCreate: kirim pengumuman baru. Sekali insert — pengguna
// melihatnya lewat kotak pengumuman tanpa fan-out per user.
func (d *Deps) AdminAnnouncementCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	title, _ := strField(body, "title")
	text, _ := strField(body, "body")
	audience, hasAudience := strField(body, "audience")
	if !hasAudience || strings.TrimSpace(audience) == "" {
		audience = "all"
	}
	title = strings.TrimSpace(title)
	text = strings.TrimSpace(text)
	if runes := []rune(title); len(runes) < annTitleMin || len(runes) > annTitleMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Judul harus 5-120 karakter.")
		return
	}
	if runes := []rune(text); len(runes) < annBodyMin || len(runes) > annBodyMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi pengumuman harus 5-4000 karakter.")
		return
	}
	if !annAudienceSet[audience] {
		httpx.Error(w, http.StatusUnprocessableEntity, `Invalid option: expected one of "all"|"client"|"partner"`)
		return
	}
	row, err := d.queryRow(r.Context(), `
		INSERT INTO announcements (id, title, body, audience, created_by)
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4)
		RETURNING id, title, body, audience, created_at, NULL::varchar AS author_name, 0 AS read_count,
		  (SELECT count(*) FROM users x WHERE x.is_active = TRUE
		      AND ((announcements.audience = 'all' AND x.role IN ('client','partner','admin')) OR x.role = announcements.audience)) AS recipient_count`,
		title, text, audience, user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// Lonceng: setiap penerima aktif dapat ping agar pengumuman langsung
	// terlihat tanpa menunggu buka dashboard. Audiens "all" mencakup admin
	// (biasa & super) — feed lonceng admin menyaring type announcement.*.
	// Kotak pengumuman tetap menjadi arsip lengkap + pelacak dibaca-nya.
	notified, err := d.execRows(r.Context(), `
		INSERT INTO notifications (id, user_id, type, title, body)
		SELECT gen_random_uuid()::text, x.id, 'announcement.new', $1, left($2, 500)
		FROM users x
		WHERE x.is_active = TRUE AND (($3 = 'all' AND x.role IN ('client','partner','admin')) OR x.role = $3)`,
		"Pengumuman: "+title, text, audience)
	if err != nil {
		// Kegagalan ping lonceng tidak membatalkan pengumuman yang sudah dibuat.
		slog.Warn("[platform] fan-out notifikasi pengumuman gagal", "err", err)
	}
	d.audit(r.Context(), user.ID, "announcement.created", "announcement", row.strOr("id", ""))
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":             true,
		"notified":       notified,
		"announcement":   annAdminJSON(row),
	})
}

// AdminAnnouncementDelete: hapus pengumuman (pembacaan ikut ter-cascade).
func (d *Deps) AdminAnnouncementDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	announcementID := chiParam(r, "announcementId")
	removed, err := d.execRows(r.Context(), "DELETE FROM announcements WHERE id = $1", announcementID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if removed == 0 {
		writeErr(w, d, annNotFoundErr)
		return
	}
	d.audit(r.Context(), user.ID, "announcement.deleted", "announcement", announcementID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
