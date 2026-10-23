package chat

import (
	"net/http"
	"strings"
	"time"

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

// PartnerThreadCreate: POST /v1/partner/chat/threads
// Mitra membuka obrolan per publikasi dengan alasan terstruktur (Konfirmasi,
// Revisi, Butuh Kutipan). Pesan pertama dari mitra sekaligus memulai sesi
// 3×24 jam. Identitas mitra tidak pernah dikirim ke klien; author pesan memakai
// nama media.
func (d *Deps) PartnerThreadCreate(w http.ResponseWriter, r *http.Request) {
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
	body := decodeJSON(r)
	publicationID := strings.TrimSpace(strField(body, "publication_id"))
	topic := strings.TrimSpace(strField(body, "topic"))
	note := strings.TrimSpace(strField(body, "note"))
	if publicationID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "publication_id wajib diisi.")
		return
	}
	if len([]rune(note)) > 1000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Catatan maksimal 1000 karakter.")
		return
	}
	switch topic {
	case "confirm", "revision", "quote":
	default:
		httpx.Error(w, http.StatusUnprocessableEntity, "Alasan obrolan tidak dikenal.")
		return
	}

	pub, err := d.queryRow(r.Context(), `
		SELECT p.id, p.brand_id, p.campaign_id, p.media_id, p.document_id, p.status,
		       m.partner_id, m.name AS media_name, m.media_type
		FROM publications p JOIN media m ON m.id = p.media_id
		WHERE p.id = $1 AND m.partner_id = $2 LIMIT 1`, publicationID, partner.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if pub == nil {
		httpx.Error(w, http.StatusNotFound, "Publikasi tidak ditemukan.")
		return
	}
	switch pub.strOr("status", "") {
	case "waiting_partner", "partner_approved", "revision":
	default:
		httpx.Error(w, http.StatusUnprocessableEntity, "Obrolan hanya untuk artikel yang belum terbit.")
		return
	}
	if topic == "quote" && !strings.EqualFold(pub.strOr("media_type", ""), "mainstream") {
		httpx.Error(w, http.StatusUnprocessableEntity, "Butuh kutipan hanya untuk media mainstream.")
		return
	}

	brandID := pub.strOr("brand_id", "")
	var noteArg any
	if note != "" {
		noteArg = note
	}
	// Find-or-create: satu forum per publikasi (unique index brand+publication).
	if err := d.exec(r.Context(), `
		INSERT INTO chat_threads (id, brand_id, campaign_id, media_id, partner_id, publication_id, document_id, topic, topic_note, created_by)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (brand_id, publication_id) WHERE publication_id IS NOT NULL DO NOTHING`,
		brandID, pub.raw("campaign_id"), pub.strOr("media_id", ""), partner.ID,
		publicationID, pub.raw("document_id"), topic, noteArg, user.ID); err != nil {
		writeErr(w, err)
		return
	}

	thread, err := d.queryRow(r.Context(),
		threadSelect+" WHERE t.brand_id = $1 AND t.publication_id = $2 LIMIT 1", brandID, publicationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if thread == nil {
		httpx.Error(w, http.StatusInternalServerError, "Forum obrolan gagal dibuat.")
		return
	}
	// Sesi yang sudah lewat tidak dibuka ulang (keputusan produk): riwayat tetap
	// dapat dibaca, tetapi mitra tidak bisa memulai kontak baru di thread itu.
	if exp := thread.timePtr("expires_at"); exp != nil && time.Now().After(*exp) {
		httpx.Error(w, http.StatusConflict, "Sesi obrolan sudah berakhir. Riwayat tetap dapat dibaca.")
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE chat_threads SET topic = $2, topic_note = $3, updated_at = now() WHERE id = $1",
		thread.strOr("id", ""), topic, noteArg); err != nil {
		writeErr(w, err)
		return
	}

	reason := map[string]string{
		"confirm":  "Halo, ada yang perlu kami konfirmasi sebelum artikel ini terbit.",
		"revision": "Halo, artikel ini perlu direvisi sebelum terbit.",
		"quote":    "Halo, artikel ini memerlukan kutipan narasumber sebelum terbit.",
	}[topic]
	messageBody := reason
	if note != "" {
		messageBody += "\n\n" + note
	}

	message, serr := d.sendMessage(r.Context(), thread.strOr("id", ""), "partner", user.ID,
		pub.strOr("media_name", ""), messageBody)
	if serr != nil {
		writeErr(w, serr)
		return
	}
	updated, _ := d.loadThread(r.Context(), thread.strOr("id", ""))
	writeJSON(w, http.StatusCreated, map[string]any{
		"message": message,
		"thread":  threadPayload(updated, false),
	})
}

// PartnerMessageSend: POST /v1/partner/chat/threads/{threadId}/messages
// Mitra bisa membalas selama sesi 3×24 jam masih berjalan.
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
