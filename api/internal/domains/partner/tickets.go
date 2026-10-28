package partner

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- tiket dukungan mitra ----------
//
// Mitra membuka tiket lalu berbincang dengan admin. Semua query selalu
// dibatasi partner_id milik sesi, jadi tiket mitra lain tidak pernah terbaca.
// Unread dihitung dari pesan admin yang belum dibaca (read_at IS NULL).

const (
	ticketSubjectMin = 3
	ticketSubjectMax = 200
	ticketBodyMax    = 5000
	ticketPerHourMax = 10
)

func ticketCode(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func ticketPayload(row rowMap) map[string]any {
	id := row.strOr("id", "")
	return map[string]any{
		"id":              id,
		"code":            ticketCode(id),
		"subject":         row.strOr("subject", ""),
		"category":        row.strOr("category", "lainnya"),
		"priority":        row.strOr("priority", "normal"),
		"status":          row.strOr("status", "buka"),
		"last_message_at": row.timePtr("last_message_at"),
		"last_message_by": row.strOr("last_message_by", ""),
		"unread":          row.intOr("unread", 0),
		"created_at":      row.timePtr("created_at"),
		"updated_at":      row.timePtr("updated_at"),
		"closed_at":       row.timePtr("closed_at"),
	}
}

const partnerTicketSelect = `
	SELECT t.id, t.subject, t.category, t.priority, t.status,
	       t.last_message_at, t.last_message_by, t.closed_at, t.created_at, t.updated_at,
	       (SELECT count(*) FROM support_ticket_messages m
	         WHERE m.ticket_id = t.id AND m.author_role = 'admin' AND m.read_at IS NULL) AS unread
	FROM support_tickets t`

func validTicketStatus(v string) bool {
	return v == "buka" || v == "diproses" || v == "selesai"
}

func validTicketCategory(v string) bool {
	return v == "teknis" || v == "pembayaran" || v == "publikasi" || v == "lainnya"
}

func validTicketPriority(v string) bool {
	return v == "rendah" || v == "normal" || v == "tinggi"
}

// Tickets: GET daftar tiket mitra, POST buat tiket baru.
func (d *Deps) Tickets(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	if r.Method == http.MethodPost {
		d.createTicket(w, r, partner)
		return
	}

	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !validTicketStatus(status) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 30, 1, 100)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	totalRow, err := d.queryRow(r.Context(),
		"SELECT count(*) AS total FROM support_tickets t WHERE t.partner_id = $1 AND ($2 = '' OR t.status = $2)",
		partner.ID, status)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	rows, err := d.queryRows(r.Context(),
		partnerTicketSelect+`
		WHERE t.partner_id = $1 AND ($2 = '' OR t.status = $2)
		ORDER BY t.last_message_at DESC LIMIT $3 OFFSET $4`,
		partner.ID, status, limit, offset)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	unreadRow, err := d.queryRow(r.Context(),
		`SELECT count(*) AS unread FROM support_ticket_messages m
		 JOIN support_tickets t ON t.id = m.ticket_id
		 WHERE t.partner_id = $1 AND m.author_role = 'admin' AND m.read_at IS NULL`, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, ticketPayload(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tickets": items,
		"total":   totalRow.intOr("total", 0),
		"unread":  unreadRow.intOr("unread", 0),
		"limit":   limit,
		"offset":  offset,
	})
}

func (d *Deps) createTicket(w http.ResponseWriter, r *http.Request, partner PartnerRow) {
	var in struct {
		Subject  string `json:"subject"`
		Category string `json:"category"`
		Priority string `json:"priority"`
		Body     string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	in.Subject = strings.TrimSpace(in.Subject)
	in.Body = strings.TrimSpace(in.Body)
	if in.Category == "" {
		in.Category = "lainnya"
	}
	if in.Priority == "" {
		in.Priority = "normal"
	}
	if n := utf8.RuneCountInString(in.Subject); n < ticketSubjectMin || n > ticketSubjectMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Subjek harus 3–200 karakter")
		return
	}
	if n := utf8.RuneCountInString(in.Body); n == 0 || n > ticketBodyMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi tiket 1–5000 karakter")
		return
	}
	if !validTicketCategory(in.Category) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kategori tidak dikenal")
		return
	}
	if !validTicketPriority(in.Priority) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Prioritas tidak dikenal")
		return
	}

	// Batas wajar anti-spam: 10 tiket baru per mitra per jam.
	recent, err := d.queryRow(r.Context(),
		"SELECT count(*) AS n FROM support_tickets WHERE partner_id = $1 AND created_at > now() - interval '1 hour'",
		partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if recent.intOr("n", 0) >= ticketPerHourMax {
		httpx.Error(w, http.StatusTooManyRequests, "Terlalu banyak tiket baru. Coba lagi nanti.")
		return
	}

	// Satu statement: tiket + pesan pertama dibuat bersama (tidak ada tiket kosong).
	created, err := d.queryRow(r.Context(), `
		WITH t AS (
			INSERT INTO support_tickets (partner_id, created_by, subject, category, priority, last_message_at, last_message_by)
			VALUES ($1, $2, $3, $4, $5, now(), 'mitra')
			RETURNING id
		)
		INSERT INTO support_ticket_messages (ticket_id, author_role, author_user_id, body)
		SELECT id, 'mitra', $2, $6 FROM t
		RETURNING ticket_id AS id`,
		partner.ID, partner.UserID, in.Subject, in.Category, in.Priority, in.Body)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	ticketID := created.strOr("id", "")

	row, err := d.queryRow(r.Context(), partnerTicketSelect+" WHERE t.id = $1", ticketID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusInternalServerError, "Tiket gagal dibaca kembali")
		return
	}
	link := "/admin/dukungan?t=" + ticketID
	d.notifyPlatformAdmins(r.Context(), "ticket.created", "Tiket dukungan baru", in.Subject, &link)
	_ = d.exec(r.Context(), `
		INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id)
		VALUES (gen_random_uuid(), $1, 'support.ticket.created', 'support_ticket', $2)`,
		partner.UserID, ticketID)

	writeJSON(w, http.StatusCreated, map[string]any{"ticket": ticketPayload(row)})
}

// TicketUnreadCount: GET angka badge sidebar.
func (d *Deps) TicketUnreadCount(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		`SELECT count(*) AS unread FROM support_ticket_messages m
		 JOIN support_tickets t ON t.id = m.ticket_id
		 WHERE t.partner_id = $1 AND m.author_role = 'admin' AND m.read_at IS NULL`, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"unread": row.intOr("unread", 0)})
}

// TicketDetail: GET detail + thread. Membuka tiket menandai balasan admin dibaca.
func (d *Deps) TicketDetail(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	ticketID := chiParam(r, "ticketId")
	row, err := d.queryRow(r.Context(),
		partnerTicketSelect+" WHERE t.id = $1 AND t.partner_id = $2", ticketID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}
	if err := d.exec(r.Context(),
		`UPDATE support_ticket_messages SET read_at = now()
		 WHERE ticket_id = $1 AND author_role = 'admin' AND read_at IS NULL`, ticketID); err != nil {
		d.writeErr(w, err)
		return
	}
	msgs, err := d.queryRows(r.Context(),
		`SELECT id, author_role, body, created_at, read_at
		 FROM support_ticket_messages WHERE ticket_id = $1 ORDER BY created_at ASC, id ASC`, ticketID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ticket":   ticketPayload(row),
		"messages": ticketMessages(msgs),
	})
}

// TicketReply: POST balasan mitra. Balasan pada tiket selesai membuka kembali.
func (d *Deps) TicketReply(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
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
	if n := utf8.RuneCountInString(in.Body); n == 0 || n > ticketBodyMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi pesan 1–5000 karakter")
		return
	}
	owns, err := d.ticketOwned(r.Context(), ticketID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !owns {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}

	row, err := d.queryRow(r.Context(), `
		WITH prev AS (SELECT status FROM support_tickets WHERE id = $1),
		m AS (
			INSERT INTO support_ticket_messages (ticket_id, author_role, author_user_id, body)
			VALUES ($1, 'mitra', $2, $3)
			RETURNING id, created_at
		),
		u AS (
			UPDATE support_tickets SET
				last_message_at = now(), last_message_by = 'mitra', updated_at = now(),
				status = CASE WHEN status = 'selesai' THEN 'buka' ELSE status END,
				closed_at = CASE WHEN status = 'selesai' THEN NULL ELSE closed_at END
			WHERE id = $1
			RETURNING status
		)
		SELECT m.id, m.created_at, u.status, (SELECT status FROM prev) AS prev_status FROM m, u`,
		ticketID, user.ID, in.Body)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row.strOr("prev_status", "") == "selesai" && row.strOr("status", "") == "buka" {
		link := "/admin/dukungan?t=" + ticketID
		d.notifyPlatformAdmins(r.Context(), "ticket.replied", "Tiket dukungan dibuka kembali", "", &link)
	}

	ticket, err := d.queryRow(r.Context(), partnerTicketSelect+" WHERE t.id = $1", ticketID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if ticket == nil {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ticket": ticketPayload(ticket),
		"message": map[string]any{
			"id":          row.strOr("id", ""),
			"author_role": "mitra",
			"body":        in.Body,
			"created_at":  row.timePtr("created_at"),
		},
	})
}

// TicketStatus: POST {status} — mitra menutup sendiri atau membuka lagi.
func (d *Deps) TicketStatus(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
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
	if in.Status != "buka" && in.Status != "selesai" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Status hanya bisa buka atau selesai")
		return
	}
	owns, err := d.ticketOwned(r.Context(), ticketID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !owns {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}
	if err := d.exec(r.Context(), `
		UPDATE support_tickets SET status = $2, updated_at = now(),
			closed_at = CASE WHEN $2 = 'selesai' THEN now() ELSE NULL END
		WHERE id = $1`, ticketID, in.Status); err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(), partnerTicketSelect+" WHERE t.id = $1", ticketID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Tiket tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": ticketPayload(row)})
}

func (d *Deps) ticketOwned(ctx context.Context, ticketID, partnerID string) (bool, error) {
	if ticketID == "" {
		return false, nil
	}
	row, err := d.queryRow(ctx,
		"SELECT 1 AS ok FROM support_tickets WHERE id = $1 AND partner_id = $2 LIMIT 1", ticketID, partnerID)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

func ticketMessages(rows []rowMap) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":          row.strOr("id", ""),
			"author_role": row.strOr("author_role", ""),
			"body":        row.strOr("body", ""),
			"created_at":  row.timePtr("created_at"),
			"read_at":     row.timePtr("read_at"),
		})
	}
	return items
}
