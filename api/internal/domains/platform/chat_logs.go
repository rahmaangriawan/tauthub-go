package platform

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Log obrolan klien ↔ mitra untuk admin platform: HANYA BACA. Admin tidak
// pernah muncul sebagai peserta, tidak bisa mengirim pesan, dan membuka
// percakapan di sini TIDAK menandai pesan sudah dibaca milik peserta.
const chatLogListMax = 300
const chatLogMsgMax = 500

// AdminChatThreads: GET /v1/platform/chat/threads?q=
// Daftar semua forum obrolan lintas brand, terbaru dulu.
func (d *Deps) AdminChatThreads(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, err := d.queryRows(r.Context(), `
		SELECT t.id, t.brand_id, t.media_id, t.publication_id,
		       b.name AS brand_name, m.name AS media_name,
		       COALESCE(ptn.name, '') AS partner_name,
		       COALESCE(d.title, '') AS article_title,
		       t.started_at, t.expires_at, t.last_message_at, t.last_message_by, t.created_at,
		       CASE WHEN t.started_at IS NULL THEN 'waiting'
		            WHEN t.expires_at > now() THEN 'open' ELSE 'expired' END AS status,
		       (SELECT count(*) FROM chat_messages cm WHERE cm.thread_id = t.id) AS message_count,
		       (SELECT count(*) FROM chat_messages cm WHERE cm.thread_id = t.id AND cm.is_flagged) AS flagged_count,
		       t.is_flagged,
		       lm.body AS last_body, lm.author_role AS last_author_role
		FROM chat_threads t
		JOIN brands b ON b.id = t.brand_id
		JOIN media m ON m.id = t.media_id
		LEFT JOIN partners ptn ON ptn.id = t.partner_id
		LEFT JOIN content_documents d ON d.id = t.document_id
		LEFT JOIN LATERAL (
			SELECT body, author_role FROM chat_messages
			WHERE thread_id = t.id ORDER BY created_at DESC, id DESC LIMIT 1
		) lm ON true
		WHERE ($1 = '' OR m.name ILIKE '%' || $1 || '%' OR b.name ILIKE '%' || $1 || '%'
		       OR COALESCE(ptn.name, '') ILIKE '%' || $1 || '%')
		ORDER BY COALESCE(t.last_message_at, t.created_at) DESC
		LIMIT $2`, q, chatLogListMax)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"id":              row.strOr("id", ""),
			"brand_name":      row.strOr("brand_name", ""),
			"media_name":      row.strOr("media_name", ""),
			"partner_name":    row.strOr("partner_name", ""),
			"article_title":   row.strOr("article_title", ""),
			"publication_id":  row.raw("publication_id"),
			"status":          row.strOr("status", ""),
			"message_count":   row.intOr("message_count", 0),
			"flagged_count":   row.intOr("flagged_count", 0),
			"is_flagged":      row.raw("is_flagged"),
			"started_at":      row.timePtr("started_at"),
			"expires_at":      row.timePtr("expires_at"),
			"last_message_at": row.timePtr("last_message_at"),
			"last_message_by": row.raw("last_message_by"),
			"created_at":      row.raw("created_at"),
		}
		if body := row.strOr("last_body", ""); body != "" {
			item["last_message"] = map[string]any{
				"body":        body,
				"author_role": row.strOr("last_author_role", ""),
			}
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"threads": items})
}

// AdminChatThreadDetail: GET /v1/platform/chat/threads/{threadId}
// Riwayat pesan satu forum (tanpa menandai dibaca); akses dicatat di audit log.
func (d *Deps) AdminChatThreadDetail(w http.ResponseWriter, r *http.Request) {
	user, adminCtx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	superAdmin := adminCtx.IsSuperAdmin()
	threadID := chi.URLParam(r, "threadId")
	thread, err := d.queryRow(r.Context(), `
		SELECT t.id, b.name AS brand_name, m.name AS media_name,
		       COALESCE(ptn.name, '') AS partner_name,
		       COALESCE(d.title, '') AS article_title,
		       t.started_at, t.expires_at, t.last_message_at, t.is_flagged,
		       CASE WHEN t.started_at IS NULL THEN 'waiting'
		            WHEN t.expires_at > now() THEN 'open' ELSE 'expired' END AS status
		FROM chat_threads t
		JOIN brands b ON b.id = t.brand_id
		JOIN media m ON m.id = t.media_id
		LEFT JOIN partners ptn ON ptn.id = t.partner_id
		LEFT JOIN content_documents d ON d.id = t.document_id
		WHERE t.id = $1 LIMIT 1`, threadID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if thread == nil {
		httpx.Error(w, http.StatusNotFound, "Forum obrolan tidak ditemukan.")
		return
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT id, author_role, COALESCE(author_name, '') AS author_name,
		       body, original_body, is_flagged, read_at, created_at
		FROM chat_messages WHERE thread_id = $1
		ORDER BY created_at ASC, id ASC LIMIT $2`, threadID, chatLogMsgMax)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// Jejak audit khusus akses log obrolan (debounce 15 menit).
	if user != nil {
		d.recordChatAudit(r, user.ID, threadID)
	}
	messages := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		item := map[string]any{
			"id":          m.strOr("id", ""),
			"author_role": m.strOr("author_role", ""),
			"author_name": m.strOr("author_name", ""),
			"body":        m.strOr("body", ""),
			"is_flagged":  m.raw("is_flagged"),
			"read_at":     m.timePtr("read_at"),
			"created_at":  m.raw("created_at"),
		}
		// Teks asli hanya untuk super admin (bukti pelanggaran).
		if superAdmin {
			item["original_body"] = m.strOr("original_body", "")
		}
		messages = append(messages, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"thread": map[string]any{
			"id":              thread.strOr("id", ""),
			"brand_name":      thread.strOr("brand_name", ""),
			"media_name":      thread.strOr("media_name", ""),
			"partner_name":    thread.strOr("partner_name", ""),
			"article_title":   thread.strOr("article_title", ""),
			"status":          thread.strOr("status", ""),
			"is_flagged":      thread.raw("is_flagged"),
			"started_at":      thread.timePtr("started_at"),
			"expires_at":      thread.timePtr("expires_at"),
			"last_message_at": thread.timePtr("last_message_at"),
		},
		"messages": messages,
	})
}

// ---------- audit trail akses admin ----------

// recordChatAudit mencatat admin yang membuka log obrolan. Debounce 15 menit:
// pembukaan berulang oleh admin yang sama pada room yang sama hanya menambah
// access_count + memperbarui last_accessed_at, bukan membuat baris baru.
func (d *Deps) recordChatAudit(r *http.Request, adminID, threadID string) {
	ip := truncateRunes(chatAuditIP(r), 45)
	ua := truncateRunes(strings.TrimSpace(r.UserAgent()), 500)
	var ipArg, uaArg any
	if ip != "" {
		ipArg = ip
	}
	if ua != "" {
		uaArg = ua
	}
	err := d.exec(r.Context(), `
		WITH updated AS (
			UPDATE admin_chat_audit_logs
			SET access_count = access_count + 1,
			    last_accessed_at = now(),
			    ip_address = COALESCE($3, ip_address),
			    user_agent = COALESCE($4, user_agent)
			WHERE id = (
				SELECT id FROM admin_chat_audit_logs
				WHERE admin_id = $1 AND chat_room_id = $2 AND action = 'VIEW_CHAT_LOG'
				  AND last_accessed_at > now() - interval '15 minutes'
				ORDER BY last_accessed_at DESC
				LIMIT 1
			)
			RETURNING id
		)
		INSERT INTO admin_chat_audit_logs (id, admin_id, chat_room_id, ip_address, user_agent, action)
		SELECT gen_random_uuid(), $1, $2, $3, $4, 'VIEW_CHAT_LOG'
		WHERE NOT EXISTS (SELECT 1 FROM updated)`,
		adminID, threadID, ipArg, uaArg)
	if err != nil {
		slog.Warn("[platform] audit log obrolan gagal", "err", err)
	}
}

// chatAuditIP: IP klien dari reverse-proxy (X-Forwarded-For → X-Real-IP →
// RemoteAddr).
func chatAuditIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		if first := strings.TrimSpace(strings.Split(xf, ",")[0]); first != "" {
			return first
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return xr
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func truncateRunes(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max])
}
