package clientread

import (
	"context"
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/[brandId]/notifications (GET list, POST mark-all-read) dan
// v1/brands/[brandId]/notifications/[notificationId]/read (POST).
//
// Notifikasi di-scope ke user yang sedang login (user_id dari session), bukan
// ke brand: brandId hanya dipakai sebagai gerbang membership. Klien tidak
// pernah bisa menandai notifikasi user lain.

// notificationJSON: parity notificationJson() di lib/notifications/service.ts.
func notificationJSON(m rowMap) map[string]any {
	return map[string]any{
		"id":             m.strOr("id", ""),
		"type":           m.strOr("type", ""),
		"title":          m.strOr("title", ""),
		"body":           nilIfEmptyStr(m),
		"link":           nilIfEmptyStrKey(m, "link"),
		"publication_id": nilIfEmptyStrKey(m, "publication_id"),
		"read_at":        m.timePtr("read_at"),
		"expired_at":     m.timePtr("expired_at"),
		"created_at":     m.timePtr("created_at"),
	}
}

// nilIfEmptyStr: body bisa NULL (kolom nullable) — jangan paksa jadi string kosong.
func nilIfEmptyStr(m rowMap) any {
	if s, ok := m.str("body"); ok {
		return s
	}
	return nil
}

func nilIfEmptyStrKey(m rowMap, key string) any {
	if s, ok := m.str(key); ok {
		return s
	}
	return nil
}

// BrandNotifications: GET /v1/brands/{brandId}/notifications — daftar notifikasi
// user dengan hitungan unread/total + filter status.
// POST pada path yang sama menandai SEMUA notifikasi user sebagai dibaca.
// ensureTopupNudge: notifikasi "saldo masih kosong" untuk brand yang belum
// pernah menerima kredit sama sekali. Idempoten lewat dedupe_key, dan tidak
// pernah menggangu brand yang sudah punya saldo maupun riwayat top up.
func (d *Deps) ensureTopupNudge(ctx context.Context, brandID, userID string) {
	// Brand yang sudah top up: pengingatnya tidak relevan lagi, jadi ditutup
	// (expired) alih-alih dibiarkan menumpuk sebagai belum dibaca.
	_, _ = d.Pool.Exec(ctx, `
		UPDATE notifications n SET expired_at = NOW()
		WHERE n.user_id = $1 AND n.type = 'billing.topup_needed' AND n.expired_at IS NULL
		  AND EXISTS (
		      SELECT 1 FROM brands b
		      LEFT JOIN credit_wallets w ON w.brand_id = b.id
		      WHERE (b.id::text = $2 OR b.public_id = $2)
		        AND n.dedupe_key = 'billing.topup_needed:' || b.id
		        AND (COALESCE(w.available_tc, 0) > 0
		             OR EXISTS (SELECT 1 FROM credit_transactions ct WHERE ct.brand_id = b.id AND ct.amount > 0))
		  )`, userID, brandID)
	_, _ = d.Pool.Exec(ctx, `
		INSERT INTO notifications (id, user_id, type, title, body, link, dedupe_key)
		SELECT gen_random_uuid(), bm.user_id,
		       'billing.topup_needed',
		       'Saldo kampanye masih kosong',
		       'Belum ada top up sama sekali. Isi saldo dulu supaya The Writer bisa menulis artikel dan mendistribusikannya ke media.',
		       '/klien/' || b.public_id || '/credits',
		       'billing.topup_needed:' || b.id
		FROM brands b
		JOIN brand_memberships bm ON bm.brand_id = b.id AND bm.user_id = $1
		JOIN users u ON u.id = bm.user_id
		LEFT JOIN credit_wallets w ON w.brand_id = b.id
		WHERE (b.id::text = $2 OR b.public_id = $2)
		  AND b.deleted_at IS NULL
		  AND u.role <> 'admin'
		  AND COALESCE(w.available_tc, 0) <= 0
		  AND COALESCE(w.reserved_tc, 0) <= 0
		  AND NOT EXISTS (SELECT 1 FROM credit_transactions ct WHERE ct.brand_id = b.id AND ct.amount > 0)
		  AND NOT EXISTS (
		      SELECT 1 FROM notifications n
		      WHERE n.user_id = $1 AND n.dedupe_key = 'billing.topup_needed:' || b.id
		  )`, userID, brandID)
}

func (d *Deps) BrandNotifications(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	if r.Method == http.MethodPost {
		d.brandNotificationsMarkAll(w, r, user.ID)
		return
	}
	// Pintu masuk klien baru: kalau saldo masih nol dan belum pernah top up,
	// tanam satu notifikasi onboarding. Ditaruh di sini (bukan sapuan latar)
	// supaya lonceng langsung berisi begitu klien membuka aplikasi; dedupe_key
	// per brand menjaga notifikasinya tetap satu seumur akun.
	d.ensureTopupNudge(r.Context(), chiParam(r, "brandId"), user.ID)

	q := r.URL.Query()
	status := q.Get("status")
	if status != "" && status != "all" && status != "read" && status != "unread" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 30, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	// category=announcement|general: tab "Pengumuman" vs "Umum" di lonceng &
	// halaman riwayat. Hitungan unread (badge lonceng) sengaja tetap global.
	category := q.Get("category")
	if category != "" && category != "general" && category != "announcement" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid category filter")
		return
	}

	// Badge lonceng (tanpa category) tetap global; halaman riwayat dengan tab
	// aktif menerima hitungan unread yang ter-filter juga.
	unreadWhere := " AND read_at IS NULL AND expired_at IS NULL"
	where := ""
	switch category {
	case "announcement":
		unreadWhere += " AND type LIKE 'announcement%'"
		where += " AND type LIKE 'announcement%'"
	case "general":
		unreadWhere += " AND type NOT LIKE 'announcement%'"
		where += " AND type NOT LIKE 'announcement%'"
	}

	var unread int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1"+unreadWhere,
		user.ID).Scan(&unread)

	switch status {
	case "read":
		where += " AND read_at IS NOT NULL"
	case "unread":
		where += " AND read_at IS NULL AND expired_at IS NULL"
	}

	var total int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1"+where, user.ID).Scan(&total)

	items, err := d.queryRows(r.Context(),
		"SELECT * FROM notifications WHERE user_id = $1"+where+" ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		user.ID, limit, offset)
	if err != nil {
		writeErr(w, err)
		return
	}
	notifications := make([]map[string]any, 0, len(items))
	for _, item := range items {
		notifications = append(notifications, notificationJSON(item))
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": notifications,
		"unread":        unread,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// brandNotificationsMarkAll: POST list — idempotent, hanya menyentuh notifikasi
// milik user; read_at yang sudah terisi tidak diubah (COALESCE).
func (d *Deps) brandNotificationsMarkAll(w http.ResponseWriter, r *http.Request, userID string) {
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE user_id = $1 AND read_at IS NULL",
		userID); err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"updated_at": isoMilli(time.Now()),
	})
}

// BrandNotificationRead: POST /v1/brands/{brandId}/notifications/{notificationId}/read.
// Scoped ke user login — tidak menerima user_id dari body. Idempotent.
func (d *Deps) BrandNotificationRead(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	notificationID := chiParam(r, "notificationId")
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE id = $1 AND user_id = $2",
		notificationID, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
