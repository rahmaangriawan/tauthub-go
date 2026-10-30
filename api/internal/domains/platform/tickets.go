package platform

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- tiket dukungan (admin) ----------
//
// Antrean lintas mitra. Semua admin boleh membalas (tanpa permission baru,
// sama seperti Log Obrolan). Balasan admin memicu notifikasi lonceng + email +
// WhatsApp ke mitra lewat jalur yang sudah dipakai event media/publikasi.

const adminTicketBodyMax = 5000

const adminTicketSelect = `
	SELECT t.id, t.partner_id, t.subject, t.category, t.priority, t.status,
	       t.last_message_at, t.last_message_by, t.closed_at, t.created_at, t.updated_at,
	       p.name AS partner_name, p.public_id AS partner_public_id, p.user_id AS partner_user_id,
	       COALESCE(u.email, '') AS partner_email,
	       (SELECT count(*) FROM support_ticket_messages m
	         WHERE m.ticket_id = t.id AND m.author_role = 'mitra' AND m.read_at IS NULL) AS unread,
	       COALESCE((SELECT m.body FROM support_ticket_messages m
	         WHERE m.ticket_id = t.id ORDER BY m.created_at DESC, m.id DESC LIMIT 1), '') AS last_body
	FROM support_tickets t
	JOIN partners p ON p.id = t.partner_id
	LEFT JOIN users u ON u.id = p.user_id`

func adminTicketPayload(row rowMap) map[string]any {
	id := row.strOr("id", "")
	code := id
	if len(code) > 8 {
		code = code[:8]
	}
	return map[string]any{
		"id":                id,
		"code":              code,
		"subject":           row.strOr("subject", ""),
		"category":          row.strOr("category", "lainnya"),
		"priority":          row.strOr("priority", "normal"),
		"status":            row.strOr("status", "buka"),
		"last_message_at":   row.timePtr("last_message_at"),
		"last_message_by":   row.strOr("last_message_by", ""),
		"last_body":         row.strOr("last_body", ""),
		"unread":            row.intOr("unread", 0),
		"created_at":        row.timePtr("created_at"),
		"updated_at":        row.timePtr("updated_at"),
		"closed_at":         row.timePtr("closed_at"),
		"partner_id":        row.strOr("partner_id", ""),
		"partner_name":      row.strOr("partner_name", ""),
		"partner_public_id": row.strOr("partner_public_id", ""),
	}
}

// AdminTickets: GET antrean tiket semua mitra.
// AdminTicketsUnreadCount: GET /v1/platform/tickets/unread-count
//
// Jumlah tiket yang masih memuat pesan mitra belum dibaca admin — dipakai badge
// merah di sidebar. Tiket selesai tidak dihitung walau pesannya belum dibaca.
func (d *Deps) AdminTicketsUnreadCount(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	row, err := d.queryRow(r.Context(), `
		SELECT COUNT(DISTINCT t.id) AS count
		FROM support_tickets t
		JOIN support_ticket_messages m ON m.ticket_id = t.id
		WHERE m.author_role = 'mitra' AND m.read_at IS NULL AND t.status <> 'selesai'`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	count := 0
	if row != nil {
		count = row.intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

func (d *Deps) AdminTickets(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "buka" && status != "diproses" && status != "selesai" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid status filter")
		return
	}
	priority := strings.TrimSpace(q.Get("priority"))
	if priority != "" && priority != "rendah" && priority != "normal" && priority != "tinggi" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid priority filter")
		return
	}
	search := strings.TrimSpace(q.Get("q"))
	// Urutan kolom prioritas: default tinggi→rendah; `prioritas_desc` membalik
	// supaya tiket "rendah" bisa disapu lebih dulu.
	sort := strings.TrimSpace(q.Get("urut"))
	if sort != "" && sort != "prioritas" && sort != "prioritas_desc" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid sort")
		return
	}
	limit := clampInt(q.Get("limit"), 50, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := ` WHERE ($1 = '' OR t.status = $1)
	           AND ($2 = '' OR t.priority = $2)
	           AND ($3 = '' OR t.subject ILIKE '%' || $3 || '%' OR p.name ILIKE '%' || $3 || '%')`
	args := []any{status, priority, search}

	totalRow, err := d.queryRow(r.Context(),
		`SELECT count(*) AS total FROM support_tickets t JOIN partners p ON p.id = t.partner_id`+where, args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// ORDER BY dirangkai dari whitelist di atas, bukan dari input mentah.
	orderBy := `ORDER BY CASE t.priority WHEN 'tinggi' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END,
		         t.last_message_at DESC`
	if sort == "prioritas_desc" {
		orderBy = `ORDER BY CASE t.priority WHEN 'tinggi' THEN 2 WHEN 'normal' THEN 1 ELSE 0 END,
		         t.last_message_at DESC`
	}
	rows, err := d.queryRows(r.Context(),
		adminTicketSelect+where+orderBy+`
		LIMIT $4 OFFSET $5`, append(args, limit, offset)...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, adminTicketPayload(row))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"tickets": items,
		"total":   totalRow.intOr("total", 0),
		"limit":   limit,
		"offset":  offset,
	})
}

// AdminTicketDetail: GET detail + thread; menandai pesan mitra sudah dibaca.
func (d *Deps) AdminTicketDetail(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	ticketID := chiParam(r, "ticketId")
	row, err := d.queryRow(r.Context(), adminTicketSelect+" WHERE t.id = $1", ticketID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}
	if err := d.exec(r.Context(),
		`UPDATE support_ticket_messages SET read_at = now()
		 WHERE ticket_id = $1 AND author_role = 'mitra' AND read_at IS NULL`, ticketID); err != nil {
		writeErr(w, d, err)
		return
	}
	msgs, err := d.queryRows(r.Context(),
		`SELECT id, author_role, body, created_at, read_at
		 FROM support_ticket_messages WHERE ticket_id = $1 ORDER BY created_at ASC, id ASC`, ticketID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	messages := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		messages = append(messages, map[string]any{
			"id":          m.strOr("id", ""),
			"author_role": m.strOr("author_role", ""),
			"body":        m.strOr("body", ""),
			"created_at":  m.timePtr("created_at"),
			"read_at":     m.timePtr("read_at"),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"ticket":   adminTicketPayload(row),
		"messages": messages,
	})
}

// AdminTicketReply: POST balasan admin; tiket "buka" otomatis jadi "diproses".
func (d *Deps) AdminTicketReply(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ticketID := chiParam(r, "ticketId")
	var in struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	if n := utf8.RuneCountInString(in.Body); n == 0 || n > adminTicketBodyMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi pesan 1–5000 karakter")
		return
	}
	ticket, err := d.queryRow(r.Context(), adminTicketSelect+" WHERE t.id = $1", ticketID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if ticket == nil {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}

	// Id balasan diambil kembali lewat RETURNING supaya bisa dipakai sebagai
	// bagian kunci dedupe notifikasi. Tanpa id ini, dua balasan admin pada
	// tiket yang sama memakai kunci yang sama dan balasan kedua tidak pernah
	// dikirim (notifikasi, email, dan WA sama-sama tertahan).
	replyRow, err := d.queryRow(r.Context(), `
		WITH m AS (
			INSERT INTO support_ticket_messages (ticket_id, author_role, author_user_id, body)
			VALUES ($1, 'admin', $2, $3)
			RETURNING id
		)
		UPDATE support_tickets SET
			status = CASE WHEN status = 'buka' THEN 'diproses' ELSE status END,
			last_message_at = now(), last_message_by = 'admin', updated_at = now()
		WHERE id = $1
		RETURNING (SELECT id FROM m) AS reply_id`, ticketID, admin.ID, in.Body)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	replyID := ""
	if replyRow != nil {
		replyID = replyRow.strOr("reply_id", "")
	}

	d.notifyTicketPartner(r, ticket, "ticket.replied", "Balasan baru tiket dukungan", in.Body, replyID)

	fresh, err := d.queryRow(r.Context(), adminTicketSelect+" WHERE t.id = $1", ticketID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if fresh == nil {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}
	d.audit(r.Context(), admin.ID, "support.ticket.replied", "support_ticket", ticketID)
	writeJSON(w, http.StatusOK, map[string]any{"ticket": adminTicketPayload(fresh)})
}

// AdminTicketStatus: PATCH {status} — ubah status tanpa membalas.
func (d *Deps) AdminTicketStatus(w http.ResponseWriter, r *http.Request) {
	admin, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	ticketID := chiParam(r, "ticketId")
	var in struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if in.Status != "buka" && in.Status != "diproses" && in.Status != "selesai" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Status tidak dikenal")
		return
	}
	ticket, err := d.queryRow(r.Context(), adminTicketSelect+" WHERE t.id = $1", ticketID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if ticket == nil {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}
	if err := d.exec(r.Context(), `
		UPDATE support_tickets SET status = $2, updated_at = now(),
			closed_at = CASE WHEN $2 = 'selesai' THEN now() ELSE NULL END
		WHERE id = $1`, ticketID, in.Status); err != nil {
		writeErr(w, d, err)
		return
	}

	if in.Status == "selesai" {
		// Tutup tiket tidak punya id balasan; kuncinya cukup id tiket karena
		// event ini memang hanya boleh terjadi sekali per tiket.
		d.notifyTicketPartner(r, ticket, "ticket.closed", "Tiket dukungan ditandai selesai",
			"Tiket dukungan Anda sudah ditandai selesai oleh admin.", "")
	}

	fresh, err := d.queryRow(r.Context(), adminTicketSelect+" WHERE t.id = $1", ticketID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if fresh == nil {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}
	d.audit(r.Context(), admin.ID, "support.ticket.status", "support_ticket", ticketID)
	writeJSON(w, http.StatusOK, map[string]any{"ticket": adminTicketPayload(fresh)})
}

// notifyTicketPartner: lonceng + email + WhatsApp ke mitra pemilik tiket.
// Email/WA best-effort: kegagalan tidak menggagalkan balasan yang sudah tersimpan.
//
// replyID membedakan dua balasan pada tiket yang sama. Sebelumnya kunci dedupe
// hanya memakai id tiket, sehingga balasan kedua dan seterusnya tidak pernah
// terkirim ke email maupun WA.
func (d *Deps) notifyTicketPartner(r *http.Request, ticket rowMap, notifType, title, preview, replyID string) {
	userID := ticket.strOr("partner_user_id", "")
	if userID == "" {
		return
	}
	ticketID := ticket.strOr("id", "")
	publicID := ticket.strOr("partner_public_id", "")
	subject := ticket.strOr("subject", "")
	link := "/mitra/" + publicID + "/dukungan/" + ticketID
	body := subject
	if preview != "" {
		cut := []rune(preview)
		if len(cut) > 140 {
			preview = string(cut[:140]) + "…"
		}
		body = subject + ": " + preview
	}
	// Identitas referensi untuk dedupe: sertakan id balasan bila ada, supaya
	// dua balasan berbeda tidak bertabrakan.
	referenceID := ticketID
	if replyID != "" {
		referenceID = ticketID + ":" + replyID
	}
	if err := d.exec(r.Context(),
		"INSERT INTO notifications (id, user_id, type, title, body, link) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)",
		userID, notifType, title, body, link); err != nil {
		slog.Warn("[support] notifikasi mitra gagal", "err", err)
	}

	data := map[string]string{
		"nama":   ticket.strOr("partner_name", ""),
		"judul":  subject,
		"status": preview,
		"link":   link,
	}
	if email := ticket.strOr("partner_email", ""); email != "" {
		d.mpEmailFollowUp(r.Context(), notifType, userID, email, referenceID, data)
	}
	d.mpWAFollowUp(r.Context(), notifType, userID, referenceID, data)
}
