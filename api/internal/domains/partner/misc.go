package partner

import (
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/imageconvert"
)

// ---------- /v1/partner/documents (+ DELETE /{id}) ----------

func partnerDocumentJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":          row.raw("id"),
		"partner_id":  row.raw("partner_id"),
		"name":        row.raw("name"),
		"type":        row.raw("type"),
		"size_bytes":  row.numOr("size_bytes"),
		"url":         "/uploads/" + row.strOr("storage_key", ""),
		"status":      row.raw("status"),
		"note":        row.raw("note"),
		"uploaded_at": row.timePtr("uploaded_at"),
		"reviewed_at": row.timePtr("reviewed_at"),
	}
}

func (d *Deps) Documents(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := d.queryRows(r.Context(),
			`SELECT id, partner_id, name, type, size_bytes, storage_key, status, note, uploaded_at, reviewed_at
			 FROM partner_documents WHERE partner_id = $1 ORDER BY uploaded_at DESC`, partner.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		docs := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			docs = append(docs, partnerDocumentJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
		return
	}

	if err := d.checkAuthRateReq(r, "partner-doc:"+user.ID, 5); err != nil {
		d.writeErr(w, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File tidak ditemukan.")
		return
	}
	defer file.Close()
	mime := header.Header.Get("Content-Type")
	nameLower := strings.ToLower(header.Filename)
	isPDF := mime == "application/pdf" || strings.HasSuffix(nameLower, ".pdf")
	isImage := strings.HasPrefix(mime, "image/")
	if !isPDF && !isImage {
		httpx.Error(w, http.StatusUnprocessableEntity, "Format file harus berupa PDF, JPG, atau PNG.")
		return
	}
	const maxBytes = 10 * 1024 * 1024
	if header.Size > maxBytes {
		httpx.Error(w, http.StatusUnprocessableEntity, "Ukuran file maksimal 10 MB.")
		return
	}
	data, err := readAllLimited(file, maxBytes)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// Gambar dinormalisasi ke WebP (parity app lama); PDF apa adanya.
	payload := data
	ext := "pdf"
	docType := "pdf"
	if !isPDF {
		docType = "image"
		if converted, ok := imageconvert.ConvertToWebP(data, mime); ok {
			payload = converted
			ext = "webp"
		} else {
			ext = sanitizeExt(nameLower, "jpg")
		}
	}
	storageKey := uuid.NewString() + "." + ext
	if err := d.saveUpload(storageKey, payload); err != nil {
		d.writeErr(w, err)
		return
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO partner_documents (id, partner_id, name, type, size_bytes, storage_key, status) VALUES ($1, $2, $3, $4, $5, $6, 'pending')",
		id, partner.ID, header.Filename, docType, len(payload), storageKey); err != nil {
		d.writeErr(w, err)
		return
	}
	created, err := d.queryRow(r.Context(),
		`SELECT id, partner_id, name, type, size_bytes, storage_key, status, note, uploaded_at, reviewed_at
		 FROM partner_documents WHERE id = $1 AND partner_id = $2 LIMIT 1`, id, partner.ID)
	if err != nil || created == nil {
		d.writeErr(w, errf("gagal membaca dokumen yang baru dibuat"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"document": partnerDocumentJSON(created)})
}

func (d *Deps) DocumentDelete(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	documentID := chiParam(r, "id")
	doc, err := d.queryRow(r.Context(),
		`SELECT id, storage_key FROM partner_documents WHERE id = $1 AND partner_id = $2 LIMIT 1`, documentID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if doc == nil {
		httpx.Error(w, http.StatusNotFound, "Dokumen tidak ditemukan.")
		return
	}
	if _, err := d.execRows(r.Context(),
		"DELETE FROM partner_documents WHERE id = $1 AND partner_id = $2", documentID, partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	storageKey := doc.strOr("storage_key", "")
	if storageKey != "" {
		for _, dir := range []string{d.uploadDir(), d.legacyUploadDir()} {
			_ = os.Remove(dir + "/" + storageKey)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- /v1/partner/notifications (+ mark-all + read) ----------

func (d *Deps) Notifications(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		if err := d.exec(r.Context(),
			"UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE user_id = $1 AND read_at IS NULL", user.ID); err != nil {
			d.writeErr(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated_at": isoMilli(nowUTC())})
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "all" && status != "read" && status != "unread" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid status filter")
		return
	}
	limit := clampInt(q.Get("limit"), 30, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	// category=announcement|general: tab "Pengumuman" vs "Umum" di lonceng &
	// halaman riwayat. Hitungan unread (badge lonceng) sengaja tetap global.
	category := strings.TrimSpace(q.Get("category"))
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
	unread, err := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM notifications WHERE user_id = $1"+unreadWhere, user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	switch status {
	case "read":
		where += " AND read_at IS NOT NULL"
	case "unread":
		where += " AND read_at IS NULL AND expired_at IS NULL"
	}
	total, err := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM notifications WHERE user_id = $1 "+where, user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items, err := d.queryRows(r.Context(),
		"SELECT * FROM notifications WHERE user_id = $1 "+where+" ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		user.ID, limit, offset)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, row := range items {
		out = append(out, notificationJSON(row))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": out,
		"unread":        unread.intOr("count", 0),
		"total":         total.intOr("count", 0),
		"limit":         limit,
		"offset":        offset,
	})
}

func (d *Deps) NotificationRead(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.auth(w, r)
	if !ok {
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE id = $1 AND user_id = $2",
		chiParam(r, "notificationId"), user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- /v1/partner/notification-preferences ----------

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func (d *Deps) NotificationPreferences(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		row, err := d.queryRow(r.Context(),
			"SELECT email_alt1, email_alt2, wa_number, wa_consent FROM notification_preferences WHERE user_id = $1 LIMIT 1",
			partner.UserID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"email_alt1": row.strOr("email_alt1", ""),
			"email_alt2": row.strOr("email_alt2", ""),
			"wa_number":  row.strOr("wa_number", ""),
			"wa_consent": row.boolOr("wa_consent", false),
		})
		return
	}
	body := decodeJSON(r)
	emailAlt1, _ := strField(body, "email_alt1")
	emailAlt2, _ := strField(body, "email_alt2")
	waNumber, _ := strField(body, "wa_number")
	waConsent, _ := boolField(body, "wa_consent")
	emailAlt1 = strings.ToLower(emailAlt1)
	emailAlt2 = strings.ToLower(emailAlt2)
	if emailAlt1 != "" && (!emailRe.MatchString(emailAlt1) || len(emailAlt1) > 320) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Email tambahan 1 tidak valid")
		return
	}
	if emailAlt2 != "" && (!emailRe.MatchString(emailAlt2) || len(emailAlt2) > 320) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Email tambahan 2 tidak valid")
		return
	}
	if len(waNumber) > 30 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Nomor WhatsApp terlalu panjang")
		return
	}
	if err := d.exec(r.Context(), `
		INSERT INTO notification_preferences (user_id, email_alt1, email_alt2, wa_number, wa_consent, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
		  email_alt1 = EXCLUDED.email_alt1,
		  email_alt2 = EXCLUDED.email_alt2,
		  wa_number = EXCLUDED.wa_number,
		  wa_consent = EXCLUDED.wa_consent,
		  updated_at = NOW()`,
		partner.UserID, ptrOrNil(emailAlt1), ptrOrNil(emailAlt2), ptrOrNil(waNumber), waConsent); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- /v1/partner/notification-webhooks ----------

func webhookJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":         row.raw("id"),
		"url":        row.raw("url"),
		"active":     row.boolOr("active", false),
		"created_at": row.timePtr("created_at"),
		"updated_at": row.timePtr("updated_at"),
	}
}

func (d *Deps) NotificationWebhooks(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.auth(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := d.queryRows(r.Context(),
			"SELECT id, url, active, created_at, updated_at FROM notification_webhooks WHERE user_id = $1 ORDER BY created_at DESC",
			user.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, webhookJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"webhooks": out})
		return
	}
	body := decodeJSON(r)
	url, present := strField(body, "url")
	if !present || !isHTTPURL(url) || len(url) > 500 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid url")
		return
	}
	var secret any
	if v, hasSecret := body["secret"]; hasSecret {
		if s, isString := v.(string); isString {
			if len(s) < 16 || len(s) > 200 {
				httpx.Error(w, http.StatusUnprocessableEntity, "Invalid secret")
				return
			}
			secret = s
		} else if v != nil {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid secret")
			return
		}
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO notification_webhooks (id, user_id, url, secret, active) VALUES ($1, $2, $3, $4, TRUE)",
		id, user.ID, url, secret); err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id, url, active, created_at, updated_at FROM notification_webhooks WHERE id = $1 LIMIT 1", id)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"webhook": webhookJSON(row)})
}

func (d *Deps) NotificationWebhookUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.auth(w, r)
	if !ok {
		return
	}
	webhookID := chiParam(r, "webhookId")
	existing, err := d.queryRow(r.Context(),
		"SELECT * FROM notification_webhooks WHERE id = $1 AND user_id = $2 LIMIT 1", webhookID, user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if existing == nil {
		d.writeErr(w, errNotFound("Webhook not found"))
		return
	}
	if r.Method == http.MethodDelete {
		if _, err := d.execRows(r.Context(),
			"DELETE FROM notification_webhooks WHERE id = $1 AND user_id = $2", webhookID, user.ID); err != nil {
			d.writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	body := decodeJSON(r)
	var urlVal, secretVal, activeVal any
	if url, present := strField(body, "url"); present {
		if !isHTTPURL(url) || len(url) > 500 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid url")
			return
		}
		urlVal = url
	}
	if v, present := body["secret"]; present {
		if s, isString := v.(string); isString {
			if len(s) < 16 || len(s) > 200 {
				httpx.Error(w, http.StatusUnprocessableEntity, "Invalid secret")
				return
			}
			secretVal = s
		} else if v == nil {
			secretVal = nil
		} else {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid secret")
			return
		}
	}
	if active, present := boolField(body, "active"); present {
		activeVal = active
	}
	if err := d.exec(r.Context(),
		`UPDATE notification_webhooks SET url = COALESCE($1, url), secret = COALESCE($2, secret), active = COALESCE($3, active), updated_at = NOW()
		 WHERE id = $4 AND user_id = $5`,
		urlVal, secretVal, activeVal, webhookID, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT id, url, active, created_at, updated_at FROM notification_webhooks WHERE id = $1 AND user_id = $2 LIMIT 1", webhookID, user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhook": webhookJSON(row)})
}
