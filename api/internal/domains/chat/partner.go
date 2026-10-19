package chat

import (
	"net/http"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// PartnerThreadsList: GET /v1/partner/chat/threads
// Forum milik media partner ini, hanya yang sudah dipakai klien (ada pesan
// pertama) — forum yang masih kosong tak ada gunanya muncul di sisi mitra.
func (d *Deps) PartnerThreadsList(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	partner, ok := d.partnerAccess(r, user.ID)
	if !ok {
		httpx.Error(w, http.StatusForbidden, "Akun ini bukan mitra aktif.")
		return
	}
	rows, err := d.queryRows(r.Context(),
		threadListSelect+" WHERE t.partner_id = $2 AND t.started_at IS NOT NULL ORDER BY COALESCE(t.last_message_at, t.created_at) DESC LIMIT 200",
		"partner", partner.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		out = append(out, threadPayload(t, true))
	}
	writeJSON(w, http.StatusOK, map[string]any{"threads": out})
}

// PartnerThreadGet: GET /v1/partner/chat/threads/{threadId}
func (d *Deps) PartnerThreadGet(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	partner, ok := d.partnerAccess(r, user.ID)
	if !ok {
		httpx.Error(w, http.StatusForbidden, "Akun ini bukan mitra aktif.")
		return
	}
	thread, err := d.loadThread(r.Context(), chiURLParam(r, "threadId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if thread == nil || thread.strOr("partner_id", "") != partner.ID {
		httpx.Error(w, http.StatusNotFound, "Forum obrolan tidak ditemukan.")
		return
	}
	messages, err := d.messagesPayload(r.Context(), thread.strOr("id", ""))
	if err != nil {
		writeErr(w, err)
		return
	}
	d.markRead(r.Context(), thread.strOr("id", ""), "partner")
	writeJSON(w, http.StatusOK, map[string]any{"thread": threadPayload(thread, false), "messages": messages})
}

// PartnerMessageSend: POST /v1/partner/chat/threads/{threadId}/messages
// Mitra hanya bisa membalas setelah klien mengirim pesan pertama (window jalan).
func (d *Deps) PartnerMessageSend(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	partner, ok := d.partnerAccess(r, user.ID)
	if !ok {
		httpx.Error(w, http.StatusForbidden, "Akun ini bukan mitra aktif.")
		return
	}
	body := strField(decodeJSON(r), "body")
	if body == "" || len([]rune(body)) > 4000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pesan wajib diisi (maks. 4000 karakter).")
		return
	}
	threadID := chiURLParam(r, "threadId")
	thread, err := d.loadThread(r.Context(), threadID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if thread == nil || thread.strOr("partner_id", "") != partner.ID {
		httpx.Error(w, http.StatusNotFound, "Forum obrolan tidak ditemukan.")
		return
	}
	message, serr := d.sendMessage(r.Context(), threadID, "partner", user.ID, partner.Name, body)
	if serr != nil {
		writeErr(w, serr)
		return
	}
	updated, _ := d.loadThread(r.Context(), threadID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"message": message,
		"thread":  threadPayload(updated, false),
	})
}
