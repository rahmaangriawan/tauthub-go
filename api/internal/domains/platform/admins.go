package platform

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
)

// Modul admins: port app/v1/platform/admins/**.
// Endpoint list/create/update/delete super-admin only (parity
// requireSuperAdminOr404); /admins/me boleh admin apa pun (dipakai shell admin).
// Guard memakai d.adminGuard lalu cek ctx.IsSuperAdmin().
//
// Helper di berkas ini diberi awalan `adm` supaya tidak bentrok dengan helper
// modul platform lain yang berbagi package.

const admListSQL = `SELECT pa.user_id, pa.role, pa.permissions, pa.created_at, pa.telegram_user_id,
       u.email, u.display_name, u.is_active,
       np.wa_number, COALESCE(np.wa_consent, false) AS wa_consent
FROM platform_admins pa
JOIN users u ON u.id = pa.user_id
LEFT JOIN notification_preferences np ON np.user_id = pa.user_id`

// admGenericError: parity adminError untuk kegagalan non-duplikat. Dibiarkan
// error biasa supaya WriteErr men-scrub pesan di production (parity apiError).
var admGenericError = errors.New("Terjadi kesalahan. Coba lagi.")

const admRoleOptionMsg = `Invalid option: expected one of "super_admin"|"admin"`

// admAdminJSON: bentuk JSON parity AdminRow (permissions jsonb → objek,
// created_at Date → ISO milidetik).
func admAdminJSON(row rowMap) map[string]any {
	permissions, _ := row.jsonField("permissions").(map[string]any)
	if permissions == nil {
		permissions = map[string]any{}
	}
	// telegram_user_id adalah BIGINT nullable; row.numOr() mengembalikan 0 untuk
	// NULL, jadi kita cek sendiri supaya UI bisa membedakan "belum di-set".
	var tgID any
	if raw, ok := row["telegram_user_id"]; ok && raw != nil {
		switch v := raw.(type) {
		case int64:
			if v != 0 {
				tgID = v
			}
		case float64:
			if v != 0 {
				tgID = int64(v)
			}
		}
	}
	return map[string]any{
		"user_id":          row.strOr("user_id", ""),
		"role":             row.strOr("role", "admin"),
		"permissions":      permissions,
		"created_at":       row.timePtr("created_at"),
		"email":            row.strOr("email", ""),
		"display_name":     row.strOr("display_name", ""),
		"is_active":        row.boolOr("is_active", false),
		"wa_number":        row.strOr("wa_number", ""),
		"wa_consent":       row.boolOr("wa_consent", false),
		"telegram_user_id": tgID,
	}
}

// admSanitizePermissions: parity sanitizePermissions — hanya key ADMIN_PERMISSIONS
// bernilai true yang disimpan.
func admSanitizePermissions(raw map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, key := range adminshared.AdminPermissions {
		if v, ok := raw[key]; ok {
			if b, isBool := v.(bool); isBool && b {
				out[key] = true
			}
		}
	}
	return out
}

// admMergePermissions: parity `{...existing, ...incoming}` sebelum sanitasi.
func admMergePermissions(existing, incoming map[string]any) map[string]any {
	merged := make(map[string]any, len(existing)+len(incoming))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range incoming {
		merged[k] = v
	}
	return merged
}

// ---------- validasi zod ----------

// admEmailValid: parity z.string().email() zod v4. RE2 Go tidak mendukung
// lookahead milik regex zod, jadi validasi dilakukan manual mengikuti pola:
// local tidak diawali/diakhiri titik, tanpa "..", domain berupa label
// dipisah titik dengan TLD alfabet >= 2 huruf.
func admEmailValid(s string) bool {
	at := strings.Index(s, "@")
	if at <= 0 || strings.Contains(s[at+1:], "@") {
		return false
	}
	local, domain := s[:at], s[at+1:]
	if strings.HasPrefix(local, ".") || strings.Contains(local, "..") || strings.Contains(domain, "..") {
		return false
	}
	for i := 0; i < len(local); i++ {
		if !admLocalChar(local[i]) {
			return false
		}
	}
	switch last := local[len(local)-1]; {
	case last >= 'a' && last <= 'z', last >= 'A' && last <= 'Z',
		last >= '0' && last <= '9', last == '_', last == '+', last == '-':
	default:
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for i, label := range labels {
		if label == "" {
			return false
		}
		head := label[0]
		if !(head >= 'a' && head <= 'z' || head >= 'A' && head <= 'Z' || head >= '0' && head <= '9') {
			return false
		}
		if i == len(labels)-1 {
			if len(label) < 2 {
				return false
			}
			for j := 0; j < len(label); j++ {
				c := label[j]
				if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
					return false
				}
			}
			continue
		}
		for j := 0; j < len(label); j++ {
			c := label[j]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func admLocalChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '_' || c == '\'' || c == '+' || c == '.' || c == '-':
		return true
	}
	return false
}

// admValidateCreate: parity createSchema. Urutan issue mengikuti urutan field
// skema (email, role, permissions); errorResponse memakai issues[0].
func admValidateCreate(body map[string]any) (string, string, map[string]any, string) {
	rawEmail, hasEmail := body["email"]
	if !hasEmail {
		return "", "", nil, "Invalid input: expected string, received undefined"
	}
	emailStr, isString := rawEmail.(string)
	if !isString {
		return "", "", nil, "Invalid input: expected string, received " + mopsJSONType(rawEmail)
	}
	email := strings.ToLower(strings.TrimSpace(emailStr))
	if !admEmailValid(email) {
		return "", "", nil, "Invalid email address"
	}
	if utf8.RuneCountInString(email) > 320 {
		return "", "", nil, "Too big: expected string to have <=320 characters"
	}

	role := "admin"
	if rawRole, hasRole := body["role"]; hasRole {
		s, isStr := rawRole.(string)
		if !isStr || (s != "super_admin" && s != "admin") {
			return "", "", nil, admRoleOptionMsg
		}
		role = s
	}

	permissions := map[string]any{}
	if rawPerms, hasPerms := body["permissions"]; hasPerms {
		obj, isObj := rawPerms.(map[string]any)
		if !isObj {
			return "", "", nil, "Invalid input: expected record, received " + mopsRecordType(rawPerms)
		}
		if msg := admPermissionIssue(obj); msg != "" {
			return "", "", nil, msg
		}
		permissions = obj
	}
	return email, role, permissions, ""
}

// admValidateUpdate: parity updateSchema (role & permissions optional).
func admValidateUpdate(body map[string]any) (string, bool, map[string]any, bool, string) {
	role := ""
	hasRole := false
	if rawRole, present := body["role"]; present {
		s, isStr := rawRole.(string)
		if !isStr || (s != "super_admin" && s != "admin") {
			return "", false, nil, false, admRoleOptionMsg
		}
		role, hasRole = s, true
	}

	var permissions map[string]any
	hasPerms := false
	if rawPerms, present := body["permissions"]; present {
		obj, isObj := rawPerms.(map[string]any)
		if !isObj {
			return "", false, nil, false, "Invalid input: expected record, received " + mopsRecordType(rawPerms)
		}
		if msg := admPermissionIssue(obj); msg != "" {
			return "", false, nil, false, msg
		}
		permissions, hasPerms = obj, true
	}
	return role, hasRole, permissions, hasPerms, ""
}

// admValidateTelegramID: parse body["telegram_user_id"]. Mengembalikan
//
//	set  bool  : apakah field ada di body
//	val  int64 : 0 = permintaan hapus kaitan (atau null); non-0 = pasang
//	zodMsg     : pesan error validasi; kosong = ok
//
// Dipakai AdminsUpdate untuk memberi admin kemampuan memetakan akun Telegram
// ke profil admin yang bersangkutan.
func admValidateTelegramID(body map[string]any) (bool, int64, string) {
	raw, present := body["telegram_user_id"]
	if !present {
		return false, 0, ""
	}
	if raw == nil {
		return true, 0, ""
	}
	switch v := raw.(type) {
	case float64:
		if v != float64(int64(v)) {
			return true, 0, "Invalid input: expected integer, received number"
		}
		return true, int64(v), ""
	case string:
		// UI biasanya kirim sebagai string; Telegram ID bisa > 2^31.
		if v == "" {
			return true, 0, ""
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return true, 0, "Invalid input: expected integer-encoded string"
		}
		return true, n, ""
	}
	return true, 0, "Invalid input: expected integer or null"
}

// admPermissionIssue: pertama nilai non-boolean di record permissions → pesan
// issue zod. Key diurutkan supaya hasil deterministik (urutan key tidak lagi
// signifikan karena pesannya hanya memuat tipe).
func admPermissionIssue(permissions map[string]any) string {
	keys := make([]string, 0, len(permissions))
	for k := range permissions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, isBool := permissions[k].(bool); !isBool {
			return "Invalid input: expected boolean, received " + mopsJSONType(permissions[k])
		}
	}
	return ""
}

// ---------- handler ----------

// AdminsList: GET /v1/platform/admins
func (d *Deps) AdminsList(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found"))
		return
	}
	rows, err := d.queryRows(r.Context(), admListSQL+" ORDER BY pa.created_at ASC")
	if err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, admAdminJSON(row))
	}
	writeJSON(w, http.StatusOK, out)
}

// AdminsCreate: POST /v1/platform/admins
func (d *Deps) AdminsCreate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found"))
		return
	}

	body, zodMsg := admRawBody(r)
	if zodMsg != "" {
		writeErr(w, d, errUnprocessable(zodMsg))
		return
	}
	email, role, permissions, zodMsg := admValidateCreate(body)
	if zodMsg != "" {
		writeErr(w, d, errUnprocessable(zodMsg))
		return
	}

	target, err := d.queryRow(r.Context(), "SELECT id FROM users WHERE email = $1 LIMIT 1", email)
	if err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	if target == nil {
		writeErr(w, d, errNotFound("User dengan email tersebut tidak ditemukan."))
		return
	}
	userID := target.strOr("id", "")

	if err := d.exec(r.Context(),
		`INSERT INTO platform_admins (user_id, role, permissions, created_at)
		 VALUES ($1, $2, $3::jsonb, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET role = EXCLUDED.role, permissions = EXCLUDED.permissions`,
		userID, role, ptJSONString(admSanitizePermissions(permissions))); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errConflict("Admin ini sudah terdaftar."))
			return
		}
		writeErr(w, d, admGenericError)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE users SET role = 'admin' WHERE id = $1 AND role <> 'admin'", userID); err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	adminshared.InvalidateAdminCache(userID)
	d.audit(r.Context(), user.ID, "platform.admin.upsert", "platform_admin", userID)
	d.audit(r.Context(), user.ID, "platform.admin.granted", "user", userID)

	row, err := d.queryRow(r.Context(), admListSQL+" WHERE pa.user_id = $1 LIMIT 1", userID)
	if err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	if row == nil {
		// parity fetchAdminRow → null (NextResponse.json(null) status 201).
		writeJSON(w, http.StatusCreated, nil)
		return
	}
	writeJSON(w, http.StatusCreated, admAdminJSON(row))
}

// AdminsMe: GET /v1/platform/admins/me
// Membalas bentuk datar (parity route lama) sekaligus key `admin` yang dipakai
// shell admin baru (`admin = data.admin ?? data`).
func (d *Deps) AdminsMe(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	role := "admin"
	if ctx.IsSuperAdmin() {
		role = "super_admin"
	}

	// created_at: kolom platform_admins, fallback now utk admin lewat env
	// (parity getAdminContext app lama).
	createdISO := isoMilli(time.Now())
	row, err := d.queryRow(r.Context(),
		"SELECT created_at FROM platform_admins WHERE user_id = $1 LIMIT 1", user.ID)
	if err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	if row != nil {
		if v, isStr := row.timePtr("created_at").(string); isStr {
			createdISO = v
		}
	}

	perms := ctx.AllowedPermissions()
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":     user.ID,
		"role":        role,
		"permissions": perms,
		"created_at":  createdISO,
		"admin": map[string]any{
			"role":        role,
			"permissions": perms,
			"created_at":  createdISO,
		},
	})
}

// AdminsUpdate: PATCH /v1/platform/admins/{adminId}
func (d *Deps) AdminsUpdate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found"))
		return
	}
	adminID := chiParam(r, "adminId")
	if adminID == user.ID {
		writeErr(w, d, errBadRequest("Tidak bisa mengubah role akun sendiri."))
		return
	}

	existing, err := d.queryRow(r.Context(),
		"SELECT user_id, permissions FROM platform_admins WHERE user_id = $1 LIMIT 1", adminID)
	if err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound("Admin tidak ditemukan."))
		return
	}

	body, zodMsg := admRawBody(r)
	if zodMsg != "" {
		writeErr(w, d, errUnprocessable(zodMsg))
		return
	}
	role, hasRole, permissions, hasPerms, zodMsg := admValidateUpdate(body)
	if zodMsg != "" {
		writeErr(w, d, errUnprocessable(zodMsg))
		return
	}

	if hasRole {
		if err := d.exec(r.Context(),
			"UPDATE platform_admins SET role = $1 WHERE user_id = $2", role, adminID); err != nil {
			writeErr(w, d, admGenericError)
			return
		}
	}
	if hasPerms {
		priorPerms, _ := existing.jsonField("permissions").(map[string]any)
		merged := admSanitizePermissions(admMergePermissions(priorPerms, permissions))
		if err := d.exec(r.Context(),
			"UPDATE platform_admins SET permissions = $1::jsonb WHERE user_id = $2",
			ptJSONString(merged), adminID); err != nil {
			writeErr(w, d, admGenericError)
			return
		}
	}
	// telegram_user_id: nilai 0 (atau null eksplisit) → kolom dikosongkan,
	// supaya admin bisa memutus kaitan akun Telegram tanpa menghapus baris.
	if tgSet, tgVal, zMsg := admValidateTelegramID(body); zMsg != "" {
		writeErr(w, d, errUnprocessable(zMsg))
		return
	} else if tgSet {
		var arg any
		if tgVal != 0 {
			arg = tgVal
		}
		if err := d.exec(r.Context(),
			"UPDATE platform_admins SET telegram_user_id = $1 WHERE user_id = $2",
			arg, adminID); err != nil {
			// Kemungkinan konflik unique (ID yang sama sudah dipakai admin lain).
			writeErr(w, d, errConflict("Telegram ID sudah dipakai admin lain."))
			return
		}
	}
	d.audit(r.Context(), user.ID, "platform.admin.updated", "user", adminID)

	result, err := d.queryRow(r.Context(), admListSQL+" WHERE pa.user_id = $1 LIMIT 1", adminID)
	if err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	if result == nil {
		// parity fetchAdminRow → null (NextResponse.json(null)).
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, admAdminJSON(result))
}

// AdminsDelete: DELETE /v1/platform/admins/{adminId}
func (d *Deps) AdminsDelete(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found"))
		return
	}
	adminID := chiParam(r, "adminId")
	if adminID == user.ID {
		writeErr(w, d, errBadRequest("Tidak bisa menghapus role akun sendiri."))
		return
	}

	existing, err := d.queryRow(r.Context(),
		"SELECT user_id FROM platform_admins WHERE user_id = $1 LIMIT 1", adminID)
	if err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound("Admin tidak ditemukan."))
		return
	}

	if err := d.exec(r.Context(), "DELETE FROM platform_admins WHERE user_id = $1", adminID); err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	d.audit(r.Context(), user.ID, "platform.admin.revoked", "user", adminID)
	w.WriteHeader(http.StatusNoContent)
}

// AdminsWANumber: PUT /v1/platform/admins/{adminId}/wa-number
// Mengisi/mengganti/menghapus nomor WhatsApp admin. Nomor itu hanya dipakai
// untuk mengirim informasi login admin lewat WA (email tidak dikirim).
func (d *Deps) AdminsWANumber(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found"))
		return
	}
	adminID := chiParam(r, "adminId")
	target, err := d.queryRow(r.Context(), "SELECT user_id FROM platform_admins WHERE user_id = $1 LIMIT 1", adminID)
	if err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	if target == nil {
		writeErr(w, d, errNotFound("Admin tidak ditemukan."))
		return
	}
	var body struct {
		WaNumber string `json:"wa_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	raw := strings.TrimSpace(body.WaNumber)
	var number any
	consent := false
	if raw != "" {
		country := "62"
		if row, qerr := d.queryRow(r.Context(), "SELECT country_code FROM platform_wa_config WHERE id = 'default' LIMIT 1"); qerr == nil && row != nil {
			country = row.strOr("country_code", "62")
		}
		normalized := notify.NormalizeWANumber(raw, country)
		if normalized == "" {
			writeErr(w, d, errUnprocessable("Nomor WA tidak valid. Contoh: 0812 3456 7890"))
			return
		}
		number = normalized
		consent = true
	}
	if err := d.exec(r.Context(), `
		INSERT INTO notification_preferences (user_id, wa_number, wa_consent, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET wa_number = EXCLUDED.wa_number, wa_consent = EXCLUDED.wa_consent, updated_at = NOW()`,
		adminID, number, consent); err != nil {
		writeErr(w, d, admGenericError)
		return
	}
	action := "platform.admin.wa_set"
	if number == nil {
		action = "platform.admin.wa_cleared"
	}
	d.audit(r.Context(), user.ID, action, "user", adminID)
	row, err := d.queryRow(r.Context(), admListSQL+" WHERE pa.user_id = $1 LIMIT 1", adminID)
	if err != nil || row == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	writeJSON(w, http.StatusOK, admAdminJSON(row))
}
