package platform

// partners.go: kelola mitra admin — list/create/get/update/delete, status VIP,
// koneksi CMS mitra, dan dokumen verifikasi.
// Port app/v1/platform/partners/route.ts, [partnerId]/route.ts,
// [partnerId]/vip/route.ts, [partnerId]/connections/**, [partnerId]/documents/**,
// + partners/_partner.ts.

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

const mpPartnerSelect = `SELECT p.*, u.email,
       (SELECT COUNT(*) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden') AS media_count,
       (SELECT r.status FROM secure_direct_requests r WHERE r.partner_id = p.id ORDER BY r.requested_at DESC LIMIT 1) AS secure_direct_status`

const mpShortIDAlphabet = "23456789abcdefghjkmnpqrstuvwxyz"

// mpBuildShortPublicID: public id 6 karakter acak (tanpa karakter mudah tertukar).
func mpBuildShortPublicID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return uuid.NewString()[:6]
	}
	out := make([]byte, 6)
	for i, b := range buf {
		out[i] = mpShortIDAlphabet[int(b)%len(mpShortIDAlphabet)]
	}
	return string(out)
}

// mpUniquePartnerPublicID: cari public_id unik (maks 60 percobaan, parity app lama).
func (d *Deps) mpUniquePartnerPublicID(ctx context.Context) (string, error) {
	for i := 0; i < 60; i++ {
		candidate := mpBuildShortPublicID()
		clash, err := d.queryRow(ctx, "SELECT 1 AS x FROM partners WHERE public_id = $1 LIMIT 1", candidate)
		if err != nil {
			return "", err
		}
		if clash == nil {
			return candidate, nil
		}
	}
	return "", errInternal("Could not allocate a unique partner public id")
}

// mpPartnerJSON: parity partnerJson.
func mpPartnerJSON(row ptRow) map[string]any {
	return map[string]any{
		"id":                   row.anyOrNil("id"),
		"user_id":              row.anyOrNil("user_id"),
		"email":                row.anyOrNil("email"),
		"name":                 row.anyOrNil("name"),
		"email_alt1":           row.anyOrNil("email_alt1"),
		"email_alt2":           row.anyOrNil("email_alt2"),
		"wa_number":            row.anyOrNil("wa_number"),
		"wa_consent":           row.boolOr("wa_consent", false),
		"price_min":            row.numPtr("price_min"),
		"price_max":            row.numPtr("price_max"),
		"status":               row.anyOrNil("status"),
		"rss_enabled":          row.boolOr("rss_enabled", false),
		"auto_publish_enabled": row.boolOr("auto_publish_enabled", false),
		"is_vip":               row.boolOr("is_vip", false),
		"vip_tier":             row.strOr("vip_tier", "regular"),
		"vip_contract_months":  row.numPtr("vip_contract_months"),
		"vip_contract_days":    row.numPtr("vip_contract_days"),
		"vip_expires_at":       row.anyOrNil("vip_expires_at"),
		"media_count":          row.intOr("media_count", 0),
		"document_status":      row.anyOrNil("document_status"),
		"document_count":       row.intOr("document_count", 0),
		"secure_direct_status": row.anyOrNil("secure_direct_status"),
		"created_at":           row.anyOrNil("created_at"),
		"updated_at":           row.anyOrNil("updated_at"),
	}
}

// mpPartnerError: port partnerError.
func mpPartnerError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate") || strings.Contains(msg, "uq_partners_user") || strings.Contains(msg, "23505") {
		return errConflict("User is already a partner")
	}
	return err
}

// mpScopedPartner: parity scopedPartner → NotFound "Partner not found".
func (d *Deps) mpScopedPartner(ctx context.Context, partnerID string) (ptRow, error) {
	row, err := d.queryRow(ctx, mpPartnerSelect+" FROM partners p JOIN users u ON u.id = p.user_id WHERE p.id = $1 LIMIT 1", partnerID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Partner not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// ---------- GET/POST /v1/platform/partners ----------

func (d *Deps) PartnersList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "active" && status != "inactive" {
		writeErr(w, d, errUnprocessable("Invalid partner status filter"))
		return
	}
	search := strings.TrimSpace(q.Get("search"))
	contact := strings.TrimSpace(q.Get("contact"))
	if contact != "" && contact != "wa" && contact != "alt" && contact != "incomplete" {
		writeErr(w, d, errUnprocessable("Invalid contact filter"))
		return
	}
	hasMedia := strings.TrimSpace(q.Get("has_media"))
	if hasMedia != "" && hasMedia != "1" && hasMedia != "0" {
		writeErr(w, d, errUnprocessable("Invalid has_media filter"))
		return
	}
	autoPublish := strings.TrimSpace(q.Get("auto_publish"))
	if autoPublish != "" && autoPublish != "1" && autoPublish != "0" {
		writeErr(w, d, errUnprocessable("Invalid auto_publish filter"))
		return
	}
	limit := mpClampInt(q.Get("limit"), 100, 1, 500)
	offset := mpClampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	args := []any{}
	next := func(v any) string {
		args = append(args, v)
		return "$" + ptItoa(len(args))
	}
	if status != "" {
		where = append(where, "p.status = "+next(status))
	}
	if search != "" {
		pattern := "%" + search + "%"
		ph := next(pattern)
		where = append(where, "(p.name ILIKE "+ph+" OR u.email ILIKE "+ph+")")
	}
	switch contact {
	case "wa":
		where = append(where, "(np.wa_number IS NOT NULL AND np.wa_number <> '')")
	case "alt":
		where = append(where, "((np.email_alt1 IS NOT NULL AND np.email_alt1 <> '') OR (np.email_alt2 IS NOT NULL AND np.email_alt2 <> ''))")
	case "incomplete":
		where = append(where, "(COALESCE(np.wa_number, '') = '' AND COALESCE(np.email_alt1, '') = '' AND COALESCE(np.email_alt2, '') = '')")
	}
	if hasMedia == "1" {
		where = append(where, "EXISTS (SELECT 1 FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden')")
	} else if hasMedia == "0" {
		where = append(where, "NOT EXISTS (SELECT 1 FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden')")
	}
	if autoPublish == "1" {
		where = append(where, "p.auto_publish_enabled = TRUE")
	} else if autoPublish == "0" {
		where = append(where, "p.auto_publish_enabled = FALSE")
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}

	// Ekspor CSV kontak mitra — tanpa pagination agar seluruh partner cocok ikut.
	if q.Get("format") == "csv" {
		all, err := d.queryRows(r.Context(), `
			SELECT p.name, p.status, u.email, np.email_alt1, np.email_alt2, np.wa_number, np.wa_consent,
			       (SELECT MIN(m.price) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden' AND m.price > 0) AS price_min,
			       (SELECT MAX(m.price) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden' AND m.price > 0) AS price_max,
			       (SELECT COUNT(*) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden') AS media_count
			FROM partners p JOIN users u ON u.id = p.user_id
			LEFT JOIN notification_preferences np ON np.user_id = p.user_id
			`+clause+`
			ORDER BY p.name ASC NULLS LAST`, args...)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		lines := []string{strings.Join([]string{
			"Nama Partner", "Email", "Email Alternatif 1", "Email Alternatif 2", "WhatsApp",
			"Izin WA", "Jumlah Media", "Harga Min", "Harga Maks", "Status",
		}, ";")}
		for _, raw := range all {
			row := ptRow(ptNormalizeMap(raw))
			waConsent := "tidak"
			if row.boolOr("wa_consent", false) {
				waConsent = "ya"
			}
			lines = append(lines, strings.Join([]string{
				mpCSVEscape(row.anyOrNil("name")),
				mpCSVEscape(row.anyOrNil("email")),
				mpCSVEscape(row.anyOrNil("email_alt1")),
				mpCSVEscape(row.anyOrNil("email_alt2")),
				mpCSVEscape(row.anyOrNil("wa_number")),
				waConsent,
				ptItoa(row.intOr("media_count", 0)),
				mpFormatIDRGrouped(row.numPtr("price_min")),
				mpFormatIDRGrouped(row.numPtr("price_max")),
				mpCSVEscape(row.anyOrNil("status")),
			}, ";"))
		}
		// BOM agar Excel membaca UTF-8; delimiter ; untuk locale Indonesia.
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="kontak-mitra-`+time.Now().UTC().Format("2006-01-02")+`.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("\uFEFF" + strings.Join(lines, "\r\n")))
		return
	}

	countRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) AS count FROM partners p JOIN users u ON u.id = p.user_id
		LEFT JOIN notification_preferences np ON np.user_id = p.user_id `+clause, args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	pageArgs := append(append([]any{}, args...), limit, offset)
	items, err := d.queryRows(r.Context(), `
		SELECT p.*, u.email, np.email_alt1, np.email_alt2, np.wa_number, np.wa_consent,
		       (SELECT MIN(m.price) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden' AND m.price > 0) AS price_min,
		       (SELECT MAX(m.price) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden' AND m.price > 0) AS price_max,
		       (SELECT COUNT(*) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden') AS media_count,
		       (SELECT d.status FROM partner_documents d WHERE d.partner_id = p.id ORDER BY d.uploaded_at DESC LIMIT 1) AS document_status,
		       (SELECT COUNT(*) FROM partner_documents d WHERE d.partner_id = p.id) AS document_count,
		       (SELECT r.status FROM secure_direct_requests r WHERE r.partner_id = p.id ORDER BY r.requested_at DESC LIMIT 1) AS secure_direct_status
		FROM partners p JOIN users u ON u.id = p.user_id
		LEFT JOIN notification_preferences np ON np.user_id = p.user_id
		`+clause+`
		ORDER BY p.created_at DESC LIMIT $`+ptItoa(len(args)+1)+` OFFSET $`+ptItoa(len(args)+2), pageArgs...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		out = append(out, mpPartnerJSON(ptRow(ptNormalizeMap(raw))))
	}
	total := 0
	if countRow != nil {
		total = ptRow(ptNormalizeMap(countRow)).intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"partners": out, "total": total, "limit": limit, "offset": offset})
}

// mpCSVEscape: parity esc di route CSV (kutip bila ada ", ; " atau newline).
func mpCSVEscape(value any) string {
	if value == nil {
		return ""
	}
	s := fmt.Sprint(value)
	if strings.ContainsAny(s, "\",;\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// mpFormatIDRGrouped: parity Intl.NumberFormat("id-ID") untuk kolom CSV.
func mpFormatIDRGrouped(v any) string {
	if v == nil {
		return ""
	}
	f, ok := v.(float64)
	if !ok {
		return ""
	}
	return mpGroupDigits(int64(math.Round(f)))
}

func mpGroupDigits(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	digits := strconv.FormatInt(n, 10)
	parts := []string{}
	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}
	parts = append([]string{digits}, parts...)
	out := strings.Join(parts, ".")
	if neg {
		out = "-" + out
	}
	return out
}

func (d *Deps) PartnerCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	email, hasEmail := strField(body, "email")
	if !hasEmail || email == "" || !mpLooksLikeEmail(email) {
		writeErr(w, d, errUnprocessable("Invalid email address"))
		return
	}
	email = strings.ToLower(email)
	name, hasName := strField(body, "name")
	if hasName && len(name) > 160 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=160 characters"))
		return
	}
	status := "active"
	if v, has := strField(body, "status"); has {
		if v != "active" && v != "inactive" {
			writeErr(w, d, errUnprocessable(mpZEnum([]string{"active", "inactive"})))
			return
		}
		status = v
	}

	found, err := d.queryRow(r.Context(), "SELECT id FROM users WHERE email = $1 LIMIT 1", email)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if found == nil {
		writeErr(w, d, errNotFound("User not found"))
		return
	}
	userID := ptRow(ptNormalizeMap(found)).strOr("id", "")

	publicID, err := d.mpUniquePartnerPublicID(r.Context())
	if err != nil {
		writeErr(w, d, err)
		return
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO partners (id, user_id, name, public_id, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())",
		id, userID, nullIfEmpty(name), publicID, status); err != nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}
	created, err := d.queryRow(r.Context(),
		mpPartnerSelect+" FROM partners p JOIN users u ON u.id = p.user_id WHERE p.id = $1 LIMIT 1", id)
	if err != nil || created == nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "partner.created", "partner", id)
	writeJSON(w, http.StatusCreated, mpPartnerJSON(ptRow(ptNormalizeMap(created))))
}

func mpLooksLikeEmail(v string) bool {
	at := strings.Index(v, "@")
	if at <= 0 || at == len(v)-1 {
		return false
	}
	domain := v[at+1:]
	return strings.Contains(domain, ".") && !strings.ContainsAny(v, " \t")
}

// ---------- GET/PUT/DELETE /v1/platform/partners/{partnerId} ----------

func (d *Deps) PartnerGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.mpScopedPartner(r.Context(), chiParam(r, "partnerId"))
	if err != nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}
	writeJSON(w, http.StatusOK, mpPartnerJSON(row))
}

func (d *Deps) PartnerUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}
	body := decodeJSON(r)

	name, hasName := strField(body, "name")
	if hasName && len(name) > 160 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=160 characters"))
		return
	}
	status, hasStatus := strField(body, "status")
	if hasStatus && status != "active" && status != "inactive" {
		writeErr(w, d, errUnprocessable(mpZEnum([]string{"active", "inactive"})))
		return
	}
	rssEnabled, hasRSS := boolField(body, "rss_enabled")
	autoPublishEnabled, hasAutoPublish := boolField(body, "auto_publish_enabled")

	updates := []string{}
	dbArgs := []any{}
	if hasName {
		dbArgs = append(dbArgs, nullIfEmpty(name))
		updates = append(updates, "name = COALESCE($"+ptItoa(len(dbArgs))+", name)")
	}
	if hasStatus {
		dbArgs = append(dbArgs, status)
		updates = append(updates, "status = COALESCE($"+ptItoa(len(dbArgs))+", status)")
	}
	if hasRSS {
		dbArgs = append(dbArgs, rssEnabled)
		updates = append(updates, "rss_enabled = $"+ptItoa(len(dbArgs)))
	}
	if hasAutoPublish {
		dbArgs = append(dbArgs, autoPublishEnabled)
		updates = append(updates, "auto_publish_enabled = $"+ptItoa(len(dbArgs)))
	}
	updates = append(updates, "updated_at = NOW()")
	dbArgs = append(dbArgs, partnerID)
	if err := d.exec(r.Context(),
		"UPDATE partners SET "+strings.Join(updates, ", ")+" WHERE id = $"+ptItoa(len(dbArgs)), dbArgs...); err != nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}
	// Perubahan izin fitur dicatat: gerbang "terbit tanpa ACC" harus bisa ditelusuri.
	if hasAutoPublish {
		action := "partner.auto_publish_disabled"
		if autoPublishEnabled {
			action = "partner.auto_publish_enabled"
		}
		writeAuditBestEffort(d, r.Context(), user.ID, action, "partner", partnerID)
	}
	// Pengajuan Secure Direct aktif saat admin memberi izin: tutup baris
	// pending dan notifkan mitra (best-effort — jangan gagalkan update utama).
	if hasAutoPublish && autoPublishEnabled {
		d.closeSecureDirectRequestOnApproval(r.Context(), partnerID, user.ID)
	}
	updated, err := d.mpScopedPartner(r.Context(), partnerID)
	if err != nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}
	writeJSON(w, http.StatusOK, mpPartnerJSON(updated))
}

// closeSecureDirectRequestOnApproval: bila partner punya pengajuan Secure
// Direct berstatus pending, tandai approved + kirim notifikasi ke user mitra
// dengan link ke halaman media mitra. Best-effort, tanpa error ke pemanggil.
func (d *Deps) closeSecureDirectRequestOnApproval(ctx context.Context, partnerID, adminUserID string) {
	row, err := d.queryRow(ctx, `
		UPDATE secure_direct_requests
		SET status = 'approved', decided_at = now(), decided_by = $2
		WHERE partner_id = $1 AND status = 'pending'
		RETURNING id`, partnerID, adminUserID)
	if err != nil || row == nil {
		return
	}
	p, perr := d.queryRow(ctx, `
		SELECT u.id AS user_id, p.public_id AS public_id
		FROM partners p JOIN users u ON u.id = p.user_id
		WHERE p.id = $1 LIMIT 1`, partnerID)
	if perr != nil || p == nil {
		return
	}
	userID := p.strOr("user_id", "")
	if userID == "" {
		return
	}
	link := fmt.Sprintf("/mitra/%s/media", p.strOr("public_id", ""))
	title := "Secure Direct diaktifkan untuk akun Anda"
	body := "Menu 'Izinkan mode otomatis' sudah dikonfirmasi admin Tauthub. Centang 'Secure Direct' pada media yang ingin langsung terbit tanpa ACC per artikel."
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link) VALUES (gen_random_uuid(), $1, 'secure_direct.approved', $2, $3, $4)",
		userID, title, body, link); err != nil {
		slog.Warn("[platform] notify secure_direct.approved gagal", "partner", partnerID, "err", err)
	}
}

func (d *Deps) PartnerDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}

	// Hard delete: partner + seluruh data turunan dalam satu transaksi. Urutan
	// dipertahankan sama dengan app lama (RESTRICT FK, tabel tanpa FK manual).
	txErr := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		mediaRows, err := tx.queryRows("SELECT id FROM media WHERE partner_id = $1", partnerID)
		if err != nil {
			return err
		}
		mediaIDs := make([]string, 0, len(mediaRows))
		for _, row := range mediaRows {
			mediaIDs = append(mediaIDs, row.strOr("id", ""))
		}
		if len(mediaIDs) > 0 {
			ph := mpPlaceholders(len(mediaIDs), 1)
			args := make([]any, 0, len(mediaIDs))
			for _, id := range mediaIDs {
				args = append(args, id)
			}
			if err := tx.exec(`DELETE FROM publication_attempts WHERE publication_id IN (SELECT id FROM publications WHERE media_id IN (`+ph+`))`, args...); err != nil {
				return err
			}
			if err := tx.exec(`DELETE FROM publications WHERE media_id IN (`+ph+`)`, args...); err != nil {
				return err
			}
			if err := tx.exec(`DELETE FROM campaign_media_items WHERE media_id IN (`+ph+`)`, args...); err != nil {
				return err
			}
			if err := tx.exec(`DELETE FROM media WHERE id IN (`+ph+`)`, args...); err != nil {
				return err
			}
		}

		statements := []struct {
			sql  string
			args []any
		}{
			{"DELETE FROM partner_documents WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM gsc_properties WHERE connection_id IN (SELECT id FROM gsc_connections WHERE partner_id = $1)", []any{partnerID}},
			{"DELETE FROM gsc_connections WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM publishing_callback_logs WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM cms_connections WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM transactions WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM publishing_api_keys WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM rss_feeds WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM revenue_shares WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM partner_earnings WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM settlements WHERE partner_id = $1", []any{partnerID}},
			{"DELETE FROM partners WHERE id = $1", []any{partnerID}},
			{"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, 'partner.deleted', 'partner', $2)", []any{user.ID, partnerID}},
		}
		for _, stmt := range statements {
			if err := tx.exec(stmt.sql, stmt.args...); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		writeErr(w, d, mpPartnerError(txErr))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- GET/PUT /v1/platform/partners/{partnerId}/vip ----------

func (d *Deps) PartnerVipGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, err)
		return
	}
	info, err := d.mpPartnerVipInfo(r.Context(), partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"partner_id": partnerID, "vip": info})
}

func (d *Deps) PartnerVipUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	isVip, hasIsVip := boolField(body, "is_vip")
	if !hasIsVip {
		writeErr(w, d, errUnprocessable("Invalid input: expected boolean, received undefined"))
		return
	}
	tier := "brand_ambassador"
	if v, has := strField(body, "vip_tier"); has {
		if v != "brand_ambassador" && v != "vip_reguler" {
			writeErr(w, d, errUnprocessable(mpZEnum([]string{"brand_ambassador", "vip_reguler"})))
			return
		}
		tier = v
	}
	var contractMonths *int
	if v, has := intField(body, "contract_months"); has {
		if v < 0 || v > 120 {
			writeErr(w, d, errUnprocessable("Invalid contract_months"))
			return
		}
		contractMonths = &v
	}
	var contractDays *int
	if v, has := intField(body, "contract_days"); has {
		if v < 0 || v > 3650 {
			writeErr(w, d, errUnprocessable("Invalid contract_days"))
			return
		}
		contractDays = &v
	}

	if err := d.mpSetVipStatus(r.Context(), partnerID, isVip, user.ID, tier, contractMonths, contractDays); err != nil {
		writeErr(w, d, err)
		return
	}

	// Entitlement/CPA opsional: hanya ditulis bila ada salah satu field-nya.
	entitlements, hasEntitlements := body["entitlements"]
	_, hasCommission := intField(body, "commission_rate_bp")
	_, hasCpaRate := intField(body, "cpa_rate")
	_, hasCpaThreshold := intField(body, "cpa_tier_threshold")
	_, hasCpaTier1 := intField(body, "cpa_tier1_rate")
	_, hasCpaTier2 := intField(body, "cpa_tier2_rate")
	if hasEntitlements || hasCommission || hasCpaRate || hasCpaThreshold || hasCpaTier1 || hasCpaTier2 {
		patch := mpVipPatch{
			Entitlements:     mpEntitlementsFrom(entitlements),
			CommissionRateBp: mpIntPtrField(body, "commission_rate_bp"),
			CpaRate:          mpIntPtrField(body, "cpa_rate"),
			CpaTierThreshold: mpIntPtrField(body, "cpa_tier_threshold"),
			CpaTier1Rate:     mpIntPtrField(body, "cpa_tier1_rate"),
			CpaTier2Rate:     mpIntPtrField(body, "cpa_tier2_rate"),
		}
		if err := d.mpUpsertAffiliateSettings(r.Context(), partnerID, patch, user.ID); err != nil {
			writeErr(w, d, err)
			return
		}
	}

	// Notifikasi in-app ke user mitra (best-effort).
	if row, err := d.queryRow(r.Context(), "SELECT user_id FROM partners WHERE id = $1 LIMIT 1", partnerID); err == nil && row != nil {
		userID := ptRow(ptNormalizeMap(row)).strOr("user_id", "")
		if userID != "" {
			event := "vip.disabled"
			title := "Status VIP Mitra dinonaktifkan"
			body := "Status VIP Mitra Anda telah dinonaktifkan oleh Admin."
			if isVip {
				isReguler := tier == "vip_reguler"
				event = "vip.enabled"
				title = "Selamat! Anda sekarang VVIP"
				body = "Anda sekarang menjadi VVIP Tauthub dengan komisi 5% dan CPA Rp15.000/domain."
				if isReguler {
					title = "Selamat! Anda sekarang VIP Reguler"
					body = "Fee affiliate Rp10.000 per domain untuk 1000 domain pertama, Rp5.000 untuk domain berikutnya. Kontrak berlaku 60 hari dan komisi transaksi tetap 1%."
				}
			}
			link := "/mitra"
			if err := d.exec(r.Context(),
				"INSERT INTO notifications (id, user_id, type, title, body, link) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)",
				userID, event, title, body, link); err != nil {
				d.logWarn("notifikasi VIP gagal", partnerID, err)
			}
		}
	}

	info, err := d.mpPartnerVipInfo(r.Context(), partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"partner_id": partnerID, "vip": info})
}

func mpIntPtrField(body map[string]any, key string) *int {
	if v, has := intField(body, key); has {
		return &v
	}
	return nil
}

// ---------- VIP mitra (port lib/vip/entitlement.ts) ----------

const (
	mpBrandAmbassadorTier   = "brand_ambassador"
	mpVipRegulerTier        = "vip_reguler"
	mpDefaultTierThreshold  = 1000
	mpDefaultTier1Rate      = 10000
	mpDefaultTier2Rate      = 5000
	mpVipRegulerContractDay = 60
)

var mpDefaultEntitlements = map[string]bool{
	"client_workspace": true, "ai_premium": true, "seo_tools": true, "keyword_research": true,
}

var mpNoEntitlements = map[string]bool{
	"client_workspace": false, "ai_premium": false, "seo_tools": false, "keyword_research": false,
}

func mpEntitlementsFrom(raw any) map[string]bool {
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]bool{}
	for _, key := range []string{"client_workspace", "ai_premium", "seo_tools", "keyword_research"} {
		if v, has := obj[key].(bool); has {
			out[key] = v
		}
	}
	return out
}

// mpParseEntitlements: parity parseEntitlements — hanya false eksplisit yang menonaktifkan.
func mpParseEntitlements(raw any) map[string]bool {
	obj, ok := raw.(map[string]any)
	if !ok {
		out := map[string]bool{}
		for k, v := range mpDefaultEntitlements {
			out[k] = v
		}
		return out
	}
	out := map[string]bool{}
	for _, key := range []string{"client_workspace", "ai_premium", "seo_tools", "keyword_research"} {
		out[key] = obj[key] != false
	}
	return out
}

// mpPartnerVipInfo: port getPartnerVipInfo (auto-switch kedaluwarsa + settings).
func (d *Deps) mpPartnerVipInfo(ctx context.Context, partnerID string) (map[string]any, error) {
	partner, err := d.queryRow(ctx,
		"SELECT is_vip, vip_tier, vip_enabled_at, vip_contract_months, vip_contract_days, vip_expires_at FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil {
		return nil, err
	}
	row := ptRow(ptNormalizeMap(partner))
	rawIsVip := row.boolOr("is_vip", false)
	tier := row.strOr("vip_tier", "regular")
	vipEnabledAt := row.anyOrNil("vip_enabled_at")
	var vipContractMonths any
	if row.has("vip_contract_months") {
		vipContractMonths = row.numOr("vip_contract_months")
	}
	var vipContractDays any
	if row.has("vip_contract_days") {
		vipContractDays = row.numOr("vip_contract_days")
	}
	expiresAt := row.anyOrNil("vip_expires_at")

	isExpired := false
	if rawIsVip && expiresAt != nil {
		if s, ok := expiresAt.(string); ok {
			if parsed, pErr := time.Parse(time.RFC3339, s); pErr == nil && !parsed.After(time.Now()) {
				isExpired = true
			}
		}
	}
	if isExpired {
		_ = d.exec(ctx,
			"UPDATE partners SET is_vip = false, vip_tier = 'regular', updated_at = NOW() WHERE id = $1 AND is_vip = true", partnerID)
	}
	isVip := rawIsVip && !isExpired
	hasVip := rawIsVip || tier == mpBrandAmbassadorTier || tier == mpVipRegulerTier || vipEnabledAt != nil

	settings, err := d.queryRow(ctx,
		"SELECT commission_rate_bp, cpa_rate, cpa_tier_threshold, cpa_tier1_rate, cpa_tier2_rate, entitlements FROM affiliate_settings WHERE partner_id = $1 LIMIT 1", partnerID)
	if err != nil {
		return nil, err
	}
	srow := ptRow(ptNormalizeMap(settings))

	commissionRateBp := 100.0
	if isVip {
		commissionRateBp = 500
	}
	if srow.has("commission_rate_bp") {
		commissionRateBp = srow.numOr("commission_rate_bp")
	}
	var cpaRate float64
	if isVip {
		cpaRate = 15000
	}
	if srow.has("cpa_rate") {
		cpaRate = srow.numOr("cpa_rate")
	}
	entitlements := mpNoEntitlements
	if isVip {
		entitlements = mpParseEntitlements(srow.jsonField("entitlements"))
	}

	out := map[string]any{
		"partnerId":         partnerID,
		"isVip":             isVip,
		"hasVip":            hasVip,
		"tier":              tier,
		"isExpired":         isExpired,
		"vipExpiresAt":      expiresAt,
		"vipContractMonths": nil,
		"vipContractDays":   nil,
		"entitlements":      entitlements,
		"commissionRateBp":  commissionRateBp,
		"cpaRate":           cpaRate,
		"cpaTierThreshold":  mpSettingInt(srow, "cpa_tier_threshold", mpDefaultTierThreshold),
		"cpaTier1Rate":      mpSettingInt(srow, "cpa_tier1_rate", mpDefaultTier1Rate),
		"cpaTier2Rate":      mpSettingInt(srow, "cpa_tier2_rate", mpDefaultTier2Rate),
	}
	if hasVip {
		out["vipContractMonths"] = vipContractMonths
		out["vipContractDays"] = vipContractDays
	}
	return out, nil
}

func mpSettingInt(row ptRow, key string, fallback int) int {
	if row.has(key) {
		return row.intOr(key, fallback)
	}
	return fallback
}

// mpSetVipStatus: port setVipStatus (kontrak bulan/hari + affiliate settings).
func (d *Deps) mpSetVipStatus(ctx context.Context, partnerID string, isVip bool, actorUserID, tier string, contractMonths, contractDays *int) error {
	if !isVip {
		if err := d.exec(ctx,
			"UPDATE partners SET is_vip = $1, vip_tier = 'regular', vip_enabled_at = NULL, vip_contract_months = NULL, vip_contract_days = NULL, vip_expires_at = NULL, updated_at = NOW() WHERE id = $2",
			false, partnerID); err != nil {
			return err
		}
		return d.exec(ctx,
			"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, 'vip.disabled', 'partner', $2)",
			actorUserID, partnerID)
	}

	existing, err := d.queryRow(ctx, "SELECT vip_contract_months, vip_contract_days FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil {
		return err
	}
	erow := ptRow(ptNormalizeMap(existing))
	isReguler := tier == mpVipRegulerTier

	var days *int
	var months *int
	if isReguler {
		value := mpVipRegulerContractDay
		if contractDays != nil {
			value = int(math.Max(0, math.Round(float64(*contractDays))))
		} else if erow.has("vip_contract_days") {
			value = erow.intOr("vip_contract_days", mpVipRegulerContractDay)
		}
		days = &value
	} else {
		if contractMonths != nil {
			value := int(math.Max(0, math.Round(float64(*contractMonths))))
			months = &value
		} else if erow.has("vip_contract_months") {
			value := erow.intOr("vip_contract_months", 0)
			months = &value
		}
	}

	var expiresAt any
	if isReguler {
		if days != nil && *days > 0 {
			expiresAt = isoMilli(time.Now().AddDate(0, 0, *days))
		}
	} else if months != nil && *months > 0 {
		// Brand Ambassador: bulan dihitung 30 hari (perilaku lama).
		expiresAt = isoMilli(time.Now().AddDate(0, 0, *months*30))
	}

	if err := d.exec(ctx,
		"UPDATE partners SET is_vip = $1, vip_tier = $2, vip_enabled_at = NOW(), vip_contract_months = $3, vip_contract_days = $4, vip_expires_at = $5, updated_at = NOW() WHERE id = $6",
		true, tier, mpIntPtr(months), mpIntPtr(days), expiresAt, partnerID); err != nil {
		return err
	}

	if isReguler {
		// Komisi transaksi tetap 1% dan tidak dibatasi kontrak; fee per-domain
		// memakai entitlement kosong (hanya fee per-domain).
		if err := d.exec(ctx, `
			INSERT INTO affiliate_settings (partner_id, commission_rate_bp, cpa_rate, cpa_tier_threshold, cpa_tier1_rate, cpa_tier2_rate, entitlements, updated_by)
			VALUES ($1, 100, 0, $2, $3, $4, $5, $6)
			ON CONFLICT (partner_id) DO UPDATE SET
			  commission_rate_bp = EXCLUDED.commission_rate_bp,
			  cpa_rate = EXCLUDED.cpa_rate,
			  cpa_tier_threshold = EXCLUDED.cpa_tier_threshold,
			  cpa_tier1_rate = EXCLUDED.cpa_tier1_rate,
			  cpa_tier2_rate = EXCLUDED.cpa_tier2_rate,
			  entitlements = EXCLUDED.entitlements,
			  updated_by = EXCLUDED.updated_by,
			  updated_at = NOW()`,
			partnerID, mpDefaultTierThreshold, mpDefaultTier1Rate, mpDefaultTier2Rate,
			mpJSONString(mpNoEntitlements), actorUserID); err != nil {
			return err
		}
	} else {
		if err := d.exec(ctx,
			"INSERT INTO affiliate_settings (partner_id, commission_rate_bp, cpa_rate, entitlements, updated_by) VALUES ($1, 500, 15000, $2, $3) ON CONFLICT (partner_id) DO UPDATE SET commission_rate_bp = EXCLUDED.commission_rate_bp, cpa_rate = EXCLUDED.cpa_rate, updated_by = EXCLUDED.updated_by, updated_at = NOW()",
			partnerID, mpJSONString(mpDefaultEntitlements), actorUserID); err != nil {
			return err
		}
	}
	return d.exec(ctx,
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, 'vip.enabled', 'partner', $2)",
		actorUserID, partnerID)
}

// mpVipPatch: patch entitlement/CPA opsional.
type mpVipPatch struct {
	Entitlements     map[string]bool
	CommissionRateBp *int
	CpaRate          *int
	CpaTierThreshold *int
	CpaTier1Rate     *int
	CpaTier2Rate     *int
}

// mpUpsertAffiliateSettings: port upsertAffiliateSettings (merge dengan nilai lama).
func (d *Deps) mpUpsertAffiliateSettings(ctx context.Context, partnerID string, patch mpVipPatch, actorUserID string) error {
	existing, err := d.queryRow(ctx,
		"SELECT commission_rate_bp, cpa_rate, cpa_tier_threshold, cpa_tier1_rate, cpa_tier2_rate, entitlements FROM affiliate_settings WHERE partner_id = $1 LIMIT 1", partnerID)
	if err != nil {
		return err
	}
	erow := ptRow(ptNormalizeMap(existing))

	commissionRateBp := mpDefaultIntFromRow(erow, "commission_rate_bp", 500)
	if patch.CommissionRateBp != nil {
		commissionRateBp = int(math.Max(0, math.Min(10000, math.Round(float64(*patch.CommissionRateBp)))))
	}
	cpaRate := mpDefaultIntFromRow(erow, "cpa_rate", 15000)
	if patch.CpaRate != nil {
		cpaRate = int(math.Max(0, math.Round(float64(*patch.CpaRate))))
	}
	threshold := mpDefaultIntFromRow(erow, "cpa_tier_threshold", mpDefaultTierThreshold)
	if patch.CpaTierThreshold != nil {
		threshold = int(math.Max(0, math.Round(float64(*patch.CpaTierThreshold))))
	}
	tier1 := mpDefaultIntFromRow(erow, "cpa_tier1_rate", mpDefaultTier1Rate)
	if patch.CpaTier1Rate != nil {
		tier1 = int(math.Max(0, math.Round(float64(*patch.CpaTier1Rate))))
	}
	tier2 := mpDefaultIntFromRow(erow, "cpa_tier2_rate", mpDefaultTier2Rate)
	if patch.CpaTier2Rate != nil {
		tier2 = int(math.Max(0, math.Round(float64(*patch.CpaTier2Rate))))
	}
	ent := mpParseEntitlements(erow.jsonField("entitlements"))
	for k, v := range patch.Entitlements {
		ent[k] = v
	}

	return d.exec(ctx, `
		INSERT INTO affiliate_settings (partner_id, commission_rate_bp, cpa_rate, cpa_tier_threshold, cpa_tier1_rate, cpa_tier2_rate, entitlements, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (partner_id) DO UPDATE SET
		  commission_rate_bp = EXCLUDED.commission_rate_bp,
		  cpa_rate = EXCLUDED.cpa_rate,
		  cpa_tier_threshold = EXCLUDED.cpa_tier_threshold,
		  cpa_tier1_rate = EXCLUDED.cpa_tier1_rate,
		  cpa_tier2_rate = EXCLUDED.cpa_tier2_rate,
		  entitlements = EXCLUDED.entitlements,
		  updated_by = EXCLUDED.updated_by,
		  updated_at = NOW()`,
		partnerID, commissionRateBp, cpaRate, threshold, tier1, tier2, mpJSONString(ent), actorUserID)
}

func mpDefaultIntFromRow(row ptRow, key string, fallback int) int {
	if row.has(key) {
		return row.intOr(key, fallback)
	}
	return fallback
}

// ---------- koneksi CMS mitra ----------

const mpCmsConnectionColumns = `SELECT id, brand_id, partner_id, website_id, provider, name, base_url, username, status, last_test_status, last_test_message, created_at, updated_at FROM cms_connections`

func mpConnectionJSON(row ptRow) map[string]any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"id":                row.anyOrNil("id"),
		"brand_id":          row.anyOrNil("brand_id"),
		"partner_id":        row.anyOrNil("partner_id"),
		"website_id":        row.anyOrNil("website_id"),
		"provider":          row.anyOrNil("provider"),
		"name":              row.anyOrNil("name"),
		"base_url":          row.anyOrNil("base_url"),
		"username":          row.anyOrNil("username"),
		"status":            row.anyOrNil("status"),
		"last_test_status":  row.anyOrNil("last_test_status"),
		"last_test_message": row.anyOrNil("last_test_message"),
		"created_at":        row.anyOrNil("created_at"),
		"updated_at":        row.anyOrNil("updated_at"),
	}
}

func (d *Deps) mpScopedPartnerConnection(ctx context.Context, partnerID, connectionID string) (ptRow, error) {
	row, err := d.queryRow(ctx, mpCmsConnectionColumns+" WHERE id = $1 AND partner_id = $2 LIMIT 1", connectionID, partnerID)
	if err != nil || row == nil {
		return nil, errNotFound("CMS connection not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

func (d *Deps) PartnerConnections(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, err)
		return
	}

	if r.Method == http.MethodGet {
		found, err := d.queryRows(r.Context(), mpCmsConnectionColumns+" WHERE partner_id = $1 ORDER BY created_at DESC", partnerID)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		out := make([]map[string]any, 0, len(found))
		for _, raw := range found {
			out = append(out, mpConnectionJSON(ptRow(ptNormalizeMap(raw))))
		}
		writeJSON(w, http.StatusOK, map[string]any{"connections": out, "total": len(out)})
		return
	}

	// POST: buat koneksi partner.
	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	if !hasName || len(name) < 2 || len(name) > 120 {
		writeErr(w, d, errUnprocessable("Invalid name"))
		return
	}
	provider := "wordpress"
	if v, has := strField(body, "provider"); has {
		if v != "wordpress" && v != "webhook" {
			writeErr(w, d, errUnprocessable(mpZEnum([]string{"wordpress", "webhook"})))
			return
		}
		provider = v
	}
	baseURL, hasBaseURL := strField(body, "base_url")
	if !hasBaseURL || len(baseURL) < 8 || len(baseURL) > 500 {
		writeErr(w, d, errUnprocessable("Invalid base_url"))
		return
	}
	username, hasUsername := strField(body, "username")
	if hasUsername && (len(username) < 1 || len(username) > 160) {
		writeErr(w, d, errUnprocessable("Invalid username"))
		return
	}
	applicationPassword, hasAppPassword := strField(body, "application_password")
	secret, hasSecret := strField(body, "secret")
	websiteID, hasWebsiteID := strField(body, "website_id")

	if provider == "wordpress" {
		if !hasUsername || username == "" {
			writeErr(w, d, errUnprocessable("username is required for WordPress connections"))
			return
		}
		if !hasAppPassword || applicationPassword == "" {
			writeErr(w, d, errUnprocessable("application_password is required for WordPress connections"))
			return
		}
	} else if !hasSecret || secret == "" {
		writeErr(w, d, errUnprocessable("secret is required for webhook connections"))
		return
	}
	if hasSecret && (len(secret) < 16 || len(secret) > 500) {
		writeErr(w, d, errUnprocessable("Invalid secret"))
		return
	}
	if hasAppPassword && (len(applicationPassword) < 1 || len(applicationPassword) > 500) {
		writeErr(w, d, errUnprocessable("Invalid application_password"))
		return
	}
	cleanBaseURL := strings.TrimRight(baseURL, "/")

	if provider == "wordpress" {
		var active map[string]any
		var err error
		if hasWebsiteID && websiteID != "" {
			active, err = d.queryRow(r.Context(),
				"SELECT id FROM cms_connections WHERE partner_id = $1 AND website_id = $2 AND provider = 'wordpress' AND status = 'active' LIMIT 1",
				partnerID, websiteID)
		} else {
			active, err = d.queryRow(r.Context(),
				"SELECT id FROM cms_connections WHERE partner_id = $1 AND base_url = $2 AND provider = 'wordpress' AND status = 'active' LIMIT 1",
				partnerID, cleanBaseURL)
		}
		if err != nil {
			writeErr(w, d, err)
			return
		}
		if active != nil {
			writeErr(w, d, errConflict("Koneksi WordPress untuk website atau URL ini sudah aktif"))
			return
		}
	}

	connectionID := uuid.NewString()
	plain := secret
	if provider == "wordpress" {
		plain = strings.ReplaceAll(applicationPassword, " ", "")
		plain = strings.ReplaceAll(plain, "\t", "")
		plain = strings.ReplaceAll(plain, "\n", "")
	}
	sealed, err := crypto.SealV3(plain, d.Cfg.Secret, connectionID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(r.Context(), `
		INSERT INTO cms_connections (id, brand_id, partner_id, website_id, provider, name, base_url, username, sealed_secret, enc_key_ver, status, created_by)
		VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, 1, 'active', $9)`,
		connectionID, partnerID, mpNullableArgStrOrNil(websiteID, hasWebsiteID), provider, name, cleanBaseURL,
		mpNullableArgStrOrNil(username, hasUsername), sealed, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "cms.connection.created", "cms_connection", connectionID)
	created, err := d.queryRow(r.Context(), mpCmsConnectionColumns+" WHERE id = $1", connectionID)
	if err != nil || created == nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, mpConnectionJSON(ptRow(ptNormalizeMap(created))))
}

func mpNullableArgStrOrNil(v string, has bool) any {
	if !has || v == "" {
		return nil
	}
	return v
}

func (d *Deps) PartnerConnectionUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	connectionID := chiParam(r, "connectionId")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, err)
		return
	}
	if _, err := d.mpScopedPartnerConnection(r.Context(), partnerID, connectionID); err != nil {
		writeErr(w, d, err)
		return
	}

	if r.Method == http.MethodDelete {
		// Parity: revoke (bukan hapus) — nama diberi penanda revoked.
		if err := d.exec(r.Context(),
			"UPDATE cms_connections SET status = 'revoked', name = CONCAT(name, ' (revoked ', LEFT(id, 8), ')') WHERE id = $1 AND partner_id = $2",
			connectionID, partnerID); err != nil {
			writeErr(w, d, err)
			return
		}
		writeAuditBestEffort(d, r.Context(), user.ID, "cms.connection.revoked", "cms_connection", connectionID)
		updated, _ := d.mpScopedPartnerConnection(r.Context(), partnerID, connectionID)
		writeJSON(w, http.StatusOK, mpConnectionJSON(updated))
		return
	}

	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	if hasName && (len(name) < 2 || len(name) > 120) {
		writeErr(w, d, errUnprocessable("Invalid name"))
		return
	}
	baseURL, hasBaseURL := strField(body, "base_url")
	if hasBaseURL && (len(baseURL) < 8 || len(baseURL) > 500) {
		writeErr(w, d, errUnprocessable("Invalid base_url"))
		return
	}
	username, hasUsername := strField(body, "username")
	if hasUsername && (len(username) < 1 || len(username) > 160) {
		writeErr(w, d, errUnprocessable("Invalid username"))
		return
	}
	secret, hasSecret := strField(body, "secret")
	if hasSecret && (len(secret) < 16 || len(secret) > 500) {
		writeErr(w, d, errUnprocessable("Invalid secret"))
		return
	}

	sets := []string{
		"name = COALESCE($1, name)",
		"base_url = COALESCE($2, base_url)",
		"username = COALESCE($3, username)",
	}
	args := []any{
		mpNullableArgStrOrNil(name, hasName),
		mpNullableTrimmed(baseURL, hasBaseURL),
		mpNullableArgStrOrNil(username, hasUsername),
	}
	if hasSecret && secret != "" {
		sealed, err := crypto.SealV3(secret, d.Cfg.Secret, connectionID)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		args = append(args, sealed)
		sets = append(sets, "sealed_secret = $"+ptItoa(len(args)), "enc_key_ver = 1")
	}
	args = append(args, connectionID, partnerID)
	if err := d.exec(r.Context(),
		"UPDATE cms_connections SET "+strings.Join(sets, ", ")+" WHERE id = $"+ptItoa(len(args)-1)+" AND partner_id = $"+ptItoa(len(args)),
		args...); err != nil {
		writeErr(w, d, err)
		return
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "cms.connection.updated", "cms_connection", connectionID)
	updated, _ := d.mpScopedPartnerConnection(r.Context(), partnerID, connectionID)
	writeJSON(w, http.StatusOK, mpConnectionJSON(updated))
}

func mpNullableTrimmed(v string, has bool) any {
	if !has {
		return nil
	}
	return strings.TrimRight(v, "/")
}

func (d *Deps) PartnerConnectionDelete(w http.ResponseWriter, r *http.Request) {
	// DELETE ditangani di PartnerConnectionUpdate lewat cabang method; handler ini
	// disediakan agar wiring route bisa memakai method handler terpisah.
	d.PartnerConnectionUpdate(w, r)
}

func (d *Deps) PartnerConnectionTest(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	connectionID := chiParam(r, "connectionId")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, err)
		return
	}
	if _, err := d.mpScopedPartnerConnection(r.Context(), partnerID, connectionID); err != nil {
		writeErr(w, d, err)
		return
	}
	conn, err := d.queryRow(r.Context(),
		"SELECT id, provider, base_url, username, sealed_secret, enc_key_ver FROM cms_connections WHERE id = $1 AND partner_id = $2 LIMIT 1",
		connectionID, partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if conn == nil {
		writeErr(w, d, errNotFound("CMS connection not found"))
		return
	}
	status, message := d.opTestConnection(r.Context(), ptRow(ptNormalizeMap(conn)))
	if err := d.exec(r.Context(),
		"UPDATE cms_connections SET last_test_status = $1, last_test_message = $2 WHERE id = $3",
		status, message, connectionID); err != nil {
		writeErr(w, d, err)
		return
	}
	payload := map[string]any{"status": status, "message": message}
	if status != "ok" {
		payload["detail"] = message
		writeJSON(w, http.StatusUnprocessableEntity, payload)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// ---------- dokumen mitra ----------

func mpPartnerDocumentJSON(row ptRow) map[string]any {
	storageKey := row.strOr("storage_key", "")
	return map[string]any{
		"id":          row.anyOrNil("id"),
		"partner_id":  row.anyOrNil("partner_id"),
		"name":        row.anyOrNil("name"),
		"type":        row.anyOrNil("type"),
		"size_bytes":  row.numOr("size_bytes"),
		"url":         "/uploads/" + storageKey,
		"status":      row.anyOrNil("status"),
		"note":        row.anyOrNil("note"),
		"uploaded_at": row.anyOrNil("uploaded_at"),
		"reviewed_at": row.anyOrNil("reviewed_at"),
	}
}

func (d *Deps) mpListPartnerDocuments(ctx context.Context, partnerID string) ([]map[string]any, error) {
	rows, err := d.queryRows(ctx, `
		SELECT id, partner_id, name, type, size_bytes, storage_key, status, note, uploaded_at, reviewed_at
		FROM partner_documents WHERE partner_id = $1 ORDER BY uploaded_at DESC`, partnerID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		out = append(out, mpPartnerDocumentJSON(ptRow(ptNormalizeMap(raw))))
	}
	return out, nil
}

func (d *Deps) PartnerDocuments(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}
	docs, err := d.mpListPartnerDocuments(r.Context(), partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

func (d *Deps) PartnerDocumentUpdate(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	documentID := chiParam(r, "id")
	if _, err := d.mpScopedPartner(r.Context(), partnerID); err != nil {
		writeErr(w, d, mpPartnerError(err))
		return
	}
	body := decodeJSON(r)
	status, hasStatus := strField(body, "status")
	if !hasStatus || (status != "pending" && status != "verified" && status != "revision") {
		writeErr(w, d, errUnprocessable("Status atau catatan tidak valid."))
		return
	}
	note, hasNote := strField(body, "note")
	if hasNote && len(note) > 500 {
		writeErr(w, d, errUnprocessable("Status atau catatan tidak valid."))
		return
	}
	if status == "revision" && len(strings.TrimSpace(note)) < 5 {
		writeErr(w, d, errUnprocessable("Alasan penolakan wajib diisi (minimal 5 karakter)."))
		return
	}

	if err := d.exec(r.Context(), `
		UPDATE partner_documents SET status = $1, note = $2, reviewed_at = NOW()
		WHERE id = $3 AND partner_id = $4`,
		status, mpNullableArgStrOrNil(note, hasNote), documentID, partnerID); err != nil {
		writeErr(w, d, err)
		return
	}
	docs, err := d.mpListPartnerDocuments(r.Context(), partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	for _, doc := range docs {
		if id, _ := doc["id"].(string); id == documentID {
			writeJSON(w, http.StatusOK, map[string]any{"document": doc})
			return
		}
	}
	writeErr(w, d, errNotFound("Dokumen tidak ditemukan."))
}
