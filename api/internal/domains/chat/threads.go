package chat

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// threadSelect: kolom dasar forum + media/mitra/brand/artikel/publikasi.
// Dipakai bersama sisi klien dan mitra supaya bentuk responsnya identik.
const threadSelect = `
	SELECT t.id, t.brand_id, t.campaign_id, t.media_id, t.partner_id, t.publication_id, t.document_id,
	       t.started_at, t.expires_at, t.last_message_at, t.last_message_by, t.created_at,
	       t.topic, t.topic_note,
	       m.name AS media_name, m.media_type, m.website_url, sm.da,
	       mr.rating AS my_rating,
	       (SELECT ROUND(AVG(r.rating)::numeric, 1)::float8 FROM media_ratings r WHERE r.media_id = m.id) AS rating_avg,
	       (SELECT COUNT(*)::int FROM media_ratings r WHERE r.media_id = m.id) AS rating_count,
	       COALESCE(ptn.name, '') AS partner_name, COALESCE(ptn.public_id, '') AS partner_public_id,
	       b.name AS brand_name, COALESCE(b.public_id, '') AS brand_public_id,
	       COALESCE(d.title, '') AS article_title,
	       p.status AS pub_status, p.published_url AS pub_url
	FROM chat_threads t
	JOIN media m ON m.id = t.media_id
	JOIN brands b ON b.id = t.brand_id
	LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
	LEFT JOIN partners ptn ON ptn.id = t.partner_id
	LEFT JOIN content_documents d ON d.id = t.document_id
	LEFT JOIN publications p ON p.id = t.publication_id
	LEFT JOIN media_ratings mr ON mr.media_id = m.id AND mr.brand_id = t.brand_id`

// threadListSelect: daftar forum + pesan terakhir + jumlah belum dibaca.
const threadListSelect = `
	SELECT t.id, t.brand_id, t.campaign_id, t.media_id, t.partner_id, t.publication_id, t.document_id,
	       t.started_at, t.expires_at, t.last_message_at, t.last_message_by, t.created_at,
	       t.topic, t.topic_note,
	       m.name AS media_name, m.media_type, m.website_url, sm.da,
	       mr.rating AS my_rating,
	       (SELECT ROUND(AVG(r.rating)::numeric, 1)::float8 FROM media_ratings r WHERE r.media_id = m.id) AS rating_avg,
	       (SELECT COUNT(*)::int FROM media_ratings r WHERE r.media_id = m.id) AS rating_count,
	       COALESCE(ptn.name, '') AS partner_name, COALESCE(ptn.public_id, '') AS partner_public_id,
	       b.name AS brand_name, COALESCE(b.public_id, '') AS brand_public_id,
	       COALESCE(d.title, '') AS article_title,
	       p.status AS pub_status, p.published_url AS pub_url,
	       lm.body AS last_body, lm.author_role AS last_author_role, lm.created_at AS last_created_at,
	       COALESCE((SELECT count(*) FROM chat_messages cm
	                 WHERE cm.thread_id = t.id AND cm.author_role <> $1 AND cm.read_at IS NULL), 0) AS unread_count
	FROM chat_threads t
	JOIN media m ON m.id = t.media_id
	JOIN brands b ON b.id = t.brand_id
	LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
	LEFT JOIN partners ptn ON ptn.id = t.partner_id
	LEFT JOIN content_documents d ON d.id = t.document_id
	LEFT JOIN publications p ON p.id = t.publication_id
	LEFT JOIN media_ratings mr ON mr.media_id = m.id AND mr.brand_id = t.brand_id
	LEFT JOIN LATERAL (
		SELECT body, author_role, created_at FROM chat_messages
		WHERE thread_id = t.id ORDER BY created_at DESC, id DESC LIMIT 1
	) lm ON true`

// threadStatus: waiting (belum ada pesan klien) | open | expired.
func threadStatus(startedAt, expiresAt *time.Time) string {
	if startedAt == nil || expiresAt == nil {
		return "waiting"
	}
	if time.Now().Before(*expiresAt) {
		return "open"
	}
	return "expired"
}

// threadPayload: bentuk JSON yang dipakai dua portal (klien & mitra).
func threadPayload(r row, includeLast bool) map[string]any {
	started := r.timePtr("started_at")
	expires := r.timePtr("expires_at")
	out := map[string]any{
		"id":                r.raw("id"),
		"brand_id":          r.raw("brand_id"),
		"campaign_id":       r.raw("campaign_id"),
		"media_id":          r.raw("media_id"),
		"partner_name":      r.strOr("partner_name", ""),
		"partner_public_id": r.strOr("partner_public_id", ""),
		"publication_id":    r.raw("publication_id"),
		"document_id":       r.raw("document_id"),
		"media_name":        r.strOr("media_name", ""),
		"media_type":        r.strOr("media_type", ""),
		"website_url":       r.strOr("website_url", ""),
		"da":                r.raw("da"),
		// Rating kecepatan terbit: milik brand ini untuk media tersebut (kalau
		// sudah pernah menilai) + rata-rata semua klien sebagai pembanding.
		"my_rating":       r.raw("my_rating"),
		"rating_avg":      r.raw("rating_avg"),
		"rating_count":    r.intOr("rating_count", 0),
		"brand_name":      r.strOr("brand_name", ""),
		"brand_public_id": r.strOr("brand_public_id", ""),
		"article_title":   r.strOr("article_title", ""),
		"pub_status":      r.raw("pub_status"),
		"pub_url":         r.raw("pub_url"),
		"started_at":      started,
		"expires_at":      expires,
		"last_message_at": r.timePtr("last_message_at"),
		"last_message_by": r.raw("last_message_by"),
		"topic":           r.raw("topic"),
		"topic_note":      r.strOr("topic_note", ""),
		"status":          threadStatus(started, expires),
		"created_at":      r.raw("created_at"),
	}
	if includeLast {
		out["unread_count"] = r.intOr("unread_count", 0)
		if body := r.strOr("last_body", ""); body != "" || r.timePtr("last_created_at") != nil {
			out["last_message"] = map[string]any{
				"body":        body,
				"author_role": r.strOr("last_author_role", ""),
				"created_at":  r.timePtr("last_created_at"),
			}
		} else {
			out["last_message"] = nil
		}
	}
	return out
}

// clientThreadPayload: bentuk forum untuk portal klien. Identitas mitra (nama
// perusahaan & public id) tidak pernah dikirim ke klien — klien hanya mengenal
// media partner-nya.
func clientThreadPayload(r row, includeLast bool) map[string]any {
	out := threadPayload(r, includeLast)
	delete(out, "partner_name")
	delete(out, "partner_public_id")
	return out
}

// loadThread: satu forum lengkap (tanpa pesan).
func (d *Deps) loadThread(ctx context.Context, threadID string) (row, error) {
	if threadID == "" {
		return nil, nil
	}
	return d.queryRow(ctx, threadSelect+" WHERE t.id = $1 LIMIT 1", threadID)
}

// markRead: tandai pesan lawan sebagai sudah dibaca oleh pembaca `role`.
func (d *Deps) markRead(ctx context.Context, threadID, role string) {
	_ = d.exec(ctx,
		"UPDATE chat_messages SET read_at = now() WHERE thread_id = $1 AND author_role <> $2 AND read_at IS NULL",
		threadID, role)
}

// messagesPayload: daftar pesan (asc), cap 500 supaya muatan tetap wajar.
func (d *Deps) messagesPayload(ctx context.Context, threadID string) ([]map[string]any, error) {
	rows, err := d.queryRows(ctx, `
		SELECT id, author_role, COALESCE(author_name, '') AS author_name, body, read_at, created_at
		FROM chat_messages WHERE thread_id = $1
		ORDER BY created_at ASC, id ASC LIMIT 500`, threadID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		out = append(out, map[string]any{
			"id":          m.raw("id"),
			"author_role": m.strOr("author_role", ""),
			"author_name": m.strOr("author_name", ""),
			"body":        m.strOr("body", ""),
			"read_at":     m.timePtr("read_at"),
			"created_at":  m.raw("created_at"),
		})
	}
	return out, nil
}

// sendMessage: aturan sesi + simpan pesan + kabari pihak lain.
//   - hanya boleh kirim saat window terbuka (waiting/open),
//   - sesi 72 jam dimulai oleh pesan PERTAMA dari pihak mana pun (klien atau
//     mitra — mitra memakai jalur "Hubungi Klien").
func (d *Deps) sendMessage(ctx context.Context, threadID, role, userID, authorName, body string) (map[string]any, *statusError) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, &statusError{http.StatusInternalServerError, "Terjadi kesalahan internal. Coba lagi."}
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		startedAt, expiresAt *time.Time
		brandID              string
		partnerID            *string
		brandName            string
		brandPublicID        string
		partnerPublicID      string
		mediaName            string
	)
	var partnerUserID *string
	row := tx.QueryRow(ctx, `
		SELECT t.started_at, t.expires_at, t.brand_id, t.partner_id,
		       b.name, COALESCE(b.public_id, ''),
		       COALESCE(ptn.public_id, ''), ptn.user_id, COALESCE(m.name, '')
		FROM chat_threads t
		JOIN brands b ON b.id = t.brand_id
		JOIN media m ON m.id = t.media_id
		LEFT JOIN partners ptn ON ptn.id = t.partner_id
		WHERE t.id = $1 FOR UPDATE OF t`, threadID)
	if err := row.Scan(&startedAt, &expiresAt, &brandID, &partnerID, &brandName, &brandPublicID, &partnerPublicID, &partnerUserID, &mediaName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &statusError{http.StatusNotFound, "Forum obrolan tidak ditemukan."}
		}
		return nil, &statusError{http.StatusInternalServerError, "Terjadi kesalahan internal. Coba lagi."}
	}

	if startedAt != nil && expiresAt != nil && time.Now().After(*expiresAt) {
		return nil, &statusError{http.StatusConflict, "Sesi obrolan sudah berakhir. Riwayat tetap dapat dibaca."}
	}
	if startedAt == nil {
		if _, err := tx.Exec(ctx,
			"UPDATE chat_threads SET started_at = now(), expires_at = now() + interval '72 hours', updated_at = now() WHERE id = $1",
			threadID); err != nil {
			return nil, &statusError{http.StatusInternalServerError, "Terjadi kesalahan internal. Coba lagi."}
		}
	}

	// Anti-bypass: sensor kontak/ajakan transaksi di luar platform. Yang
	// ditampilkan (dan dikirim ke notifikasi) adalah versi tersensor; teks asli
	// disimpan di original_body hanya untuk bukti super admin.
	masked, flagged := MaskChatMessage(body)

	var msgID string
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO chat_messages (id, thread_id, author_role, author_user_id, author_name, body, original_body, is_flagged)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`, threadID, role, userID, authorName, masked, body, flagged).Scan(&msgID, &createdAt); err != nil {
		return nil, &statusError{http.StatusInternalServerError, "Terjadi kesalahan internal. Coba lagi."}
	}
	if flagged {
		if _, err := tx.Exec(ctx,
			"UPDATE chat_threads SET last_message_at = now(), last_message_by = $2, is_flagged = TRUE, updated_at = now() WHERE id = $1",
			threadID, role); err != nil {
			return nil, &statusError{http.StatusInternalServerError, "Terjadi kesalahan internal. Coba lagi."}
		}
	} else if _, err := tx.Exec(ctx,
		"UPDATE chat_threads SET last_message_at = now(), last_message_by = $2, updated_at = now() WHERE id = $1",
		threadID, role); err != nil {
		return nil, &statusError{http.StatusInternalServerError, "Terjadi kesalahan internal. Coba lagi."}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, &statusError{http.StatusInternalServerError, "Terjadi kesalahan internal. Coba lagi."}
	}

	// Notifikasi setelah commit supaya tidak menahan transaksi.
	brandRef := brandPublicID
	if brandRef == "" {
		brandRef = brandID
	}
	if role == "client" {
		if partnerUserID != nil && *partnerUserID != "" {
			d.notifyUser(ctx, *partnerUserID, "Pesan baru dari "+brandName, masked,
				"/mitra/"+partnerPublicID+"/chat?t="+threadID)
		}
	} else {
		title := "Pesan baru dari mitra"
		if mediaName != "" {
			title = "Pesan baru dari " + mediaName
		}
		d.notifyBrandMembers(ctx, brandID, userID, title, masked,
			"/klien/"+brandRef+"/chat?t="+threadID)
	}

	return map[string]any{
		"id":          msgID,
		"author_role": role,
		"author_name": authorName,
		"body":        masked,
		"is_flagged":  flagged,
		"read_at":     nil,
		"created_at":  createdAt,
	}, nil
}

// statusError: error dengan status eksplisit (409/404 untuk aturan sesi).
type statusError struct {
	Status  int
	Message string
}

func (e *statusError) Error() string { return e.Message }

// authUser: pembungkus tipis supaya handler tidak mengulang nil-check.
func authUser(r *http.Request) *httpx.AuthUser { return httpx.AuthUserFromCtx(r) }
