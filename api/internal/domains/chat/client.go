package chat

import (
	"log/slog"
	"net/http"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ClientThreadsList: GET /v1/brands/{brandId}/chat/threads
// Daftar forum obrolan brand, terbaru dulu (yang belum pernah dipakai ikut
// tampil dengan last_message null).
func (d *Deps) ClientThreadsList(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.brandAccess(r, user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	rows, err := d.queryRows(r.Context(),
		threadListSelect+" WHERE t.brand_id = $2 ORDER BY COALESCE(t.last_message_at, t.created_at) DESC LIMIT 200",
		"client", brand.ID)
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

// ClientThreadCreate: POST /v1/brands/{brandId}/chat/threads {publication_id}
// Find-or-create forum untuk publikasi; idempoten supaya klik "Hubungi"
// berulang tidak membuat forum ganda.
func (d *Deps) ClientThreadCreate(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.brandAccess(r, user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	publicationID := strField(body, "publication_id")
	if publicationID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "publication_id wajib diisi.")
		return
	}

	// Publikasi harus milik brand ini; ambil media & dokumen untuk konteks forum.
	pub, err := d.queryRow(r.Context(), `
		SELECT p.id, p.media_id, p.document_id, p.campaign_id, m.partner_id
		FROM publications p JOIN media m ON m.id = p.media_id
		WHERE p.id = $1 AND p.brand_id = $2 LIMIT 1`, publicationID, brand.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if pub == nil {
		httpx.Error(w, http.StatusNotFound, "Publikasi tidak ditemukan.")
		return
	}

	// Rating kecepatan terbit (1–5) ikut terkirim saat klien menekan "Hubungi"
	// di halaman ringkasan. Satu brand hanya punya satu rating per media, jadi
	// penilaian ulang menimpa nilai lama; kegagalan di sini tidak boleh
	// menggagalkan pembuatan forum obrolan.
	if rating, ok := ratingField(body); ok {
		if mediaID := pub.strOr("media_id", ""); mediaID != "" {
			if err := d.exec(r.Context(), `
				INSERT INTO media_ratings (id, media_id, brand_id, rating, publication_id, created_by)
				VALUES ((gen_random_uuid())::text, $1, $2, $3, $4, $5)
				ON CONFLICT (brand_id, media_id) DO UPDATE
				   SET rating = EXCLUDED.rating,
				       publication_id = EXCLUDED.publication_id,
				       updated_at = now()`,
				mediaID, brand.ID, rating, publicationID, user.ID); err != nil {
				slog.Warn("[chat] simpan rating media gagal", "publication", publicationID, "err", err)
			}
		}
	}

	if err := d.exec(r.Context(), `
		INSERT INTO chat_threads (id, brand_id, campaign_id, media_id, partner_id, publication_id, document_id, created_by)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (brand_id, publication_id) WHERE publication_id IS NOT NULL DO NOTHING`,
		brand.ID, pub.raw("campaign_id"), pub.strOr("media_id", ""), pub.raw("partner_id"),
		publicationID, pub.raw("document_id"), user.ID); err != nil {
		writeErr(w, err)
		return
	}

	thread, err := d.queryRow(r.Context(), threadSelect+" WHERE t.brand_id = $1 AND t.publication_id = $2 LIMIT 1", brand.ID, publicationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if thread == nil {
		httpx.Error(w, http.StatusInternalServerError, "Forum obrolan gagal dibuat.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"thread": threadPayload(thread, false)})
}

// ClientThreadGet: GET /v1/brands/{brandId}/chat/threads/{threadId}
// Meta forum + seluruh pesan; sekaligus menandai pesan mitra sudah dibaca.
func (d *Deps) ClientThreadGet(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.brandAccess(r, user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	thread, err := d.loadThread(r.Context(), chiURLParam(r, "threadId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if thread == nil || thread.strOr("brand_id", "") != brand.ID {
		httpx.Error(w, http.StatusNotFound, "Forum obrolan tidak ditemukan.")
		return
	}
	messages, err := d.messagesPayload(r.Context(), thread.strOr("id", ""))
	if err != nil {
		writeErr(w, err)
		return
	}
	d.markRead(r.Context(), thread.strOr("id", ""), "client")
	writeJSON(w, http.StatusOK, map[string]any{"thread": threadPayload(thread, false), "messages": messages})
}

// ClientMessageSend: POST /v1/brands/{brandId}/chat/threads/{threadId}/messages
func (d *Deps) ClientMessageSend(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.brandAccess(r, user.ID, "writer")
	if err != nil {
		writeErr(w, err)
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
	if thread == nil || thread.strOr("brand_id", "") != brand.ID {
		httpx.Error(w, http.StatusNotFound, "Forum obrolan tidak ditemukan.")
		return
	}
	message, serr := d.sendMessage(r.Context(), threadID, "client", user.ID, user.DisplayName, body)
	if serr != nil {
		writeErr(w, serr)
		return
	}
	// State forum terbaru supaya UI langsung tahu window mulai/berakhir.
	updated, _ := d.loadThread(r.Context(), threadID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"message": message,
		"thread":  threadPayload(updated, false),
	})
}
