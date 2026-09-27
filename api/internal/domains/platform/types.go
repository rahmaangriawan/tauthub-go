// Package platform: port endpoint kontrol-plane admin (/v1/platform/*) dan
// pembayaran publik (/v1/payments/*) dari app/v1/platform + app/v1/payments.
//
// Paket ini dipecah per modul (operations, media, partners, ai, settings, pr,
// payments, taut, misc) tetapi berbagi satu struct Deps dan helper respons di
// sini supaya gaya dan kontraknya seragam.
package platform

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Deps: dependensi bersama seluruh modul platform.
type Deps struct {
	Pool  *pgxpool.Pool
	Cfg   *config.Config
	Admin *adminshared.Deps
}

// ---------- helper parameter & batas ----------

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func clampInt(raw string, def, min, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// pageParams: limit/offset standar endpoint admin (default 50, maks 500).
func pageParams(r *http.Request) (int, int) {
	q := r.URL.Query()
	return clampInt(q.Get("limit"), 50, 1, 500), clampInt(q.Get("offset"), 0, 0, 1<<30)
}

// ---------- helper body ----------

func decodeJSON(r *http.Request) map[string]any {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		return map[string]any{}
	}
	return body
}

func strField(body map[string]any, key string) (string, bool) {
	v, ok := body[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(s), true
}

func boolField(body map[string]any, key string) (bool, bool) {
	v, ok := body[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	if !ok {
		return false, false
	}
	return b, true
}

func numField(body map[string]any, key string) (float64, bool) {
	v, ok := body[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func intField(body map[string]any, key string) (int, bool) {
	f, ok := numField(body, key)
	if !ok {
		return 0, false
	}
	return int(f), true
}

// stringSliceField: array string dari body (mis. daftar id terpilih).
func stringSliceField(body map[string]any, key string) ([]string, bool) {
	v, ok := body[key]
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, isString := item.(string); isString && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out, true
}

// rawField: nilai JSON apa adanya (untuk kolom jsonb).
func rawField(body map[string]any, key string) (any, bool) {
	v, ok := body[key]
	return v, ok
}

// ---------- helper respons ----------

func writeJSON(w http.ResponseWriter, status int, payload any) {
	adminshared.WriteJSON(w, status, payload)
}

func writeErr(w http.ResponseWriter, d *Deps, err error) {
	d.Admin.WriteErr(w, err)
}

func errNotFound(msg string) error   { return adminshared.Err(http.StatusNotFound, msg) }
func errBadRequest(msg string) error { return adminshared.Err(http.StatusBadRequest, msg) }
func errConflict(msg string) error   { return adminshared.Err(http.StatusConflict, msg) }
func errUnprocessable(msg string) error {
	return adminshared.Err(http.StatusUnprocessableEntity, msg)
}

func isoMilli(t time.Time) string { return adminshared.IsoMilli(t) }

// tsPtr: waktu → ISO string atau nil (parity JSON Date).
func tsPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return isoMilli(*t)
}

func derefOr(p *string, fallback string) string {
	if p == nil || *p == "" {
		return fallback
	}
	return *p
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// ---------- guard admin: delegasi ke adminshared ----------

// adminGuard: user + context admin + permission. `permission` "" = cukup admin.
func (d *Deps) adminGuard(w http.ResponseWriter, r *http.Request, permission string) (*httpx.AuthUser, adminshared.Context, bool) {
	return d.Admin.Require(w, r, permission)
}

// audit: catat audit_logs (best-effort; kegagalan tidak menggagalkan aksi).
func (d *Deps) audit(ctx context.Context, actorUserID, action, targetType, targetID string) {
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, $2, $3, $4)",
		nullIfEmpty(actorUserID), action, targetType, targetID); err != nil {
		slog.Warn("[platform] audit gagal", "action", action, "err", err)
	}
}

func (d *Deps) exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.Pool.Exec(ctx, sql, args...)
	return err
}

func (d *Deps) execRows(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := d.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// notifyPlatformAdmins: parity notifyPlatformAdmins di app lama.
func (d *Deps) notifyPlatformAdmins(ctx context.Context, notifType, title, body string, link *string) {
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link) SELECT gen_random_uuid(), id, $1, $2, $3, $4 FROM users WHERE role = 'admin' AND is_active = TRUE",
		notifType, title, body, link); err != nil {
		slog.Warn("[platform] notify admins gagal", "type", notifType, "err", err)
	}
}

// ---------- query rows → map (kolom dinamis) ----------

// queryRows: jalankan query dan kembalikan baris sebagai map kolom → nilai.
// Dipakai endpoint yang menulis `SELECT *` (parity RowDataPacket app lama).
func (d *Deps) queryRows(ctx context.Context, sql string, args ...any) ([]rowMap, error) {
	rows, err := d.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = string(f.Name)
	}
	out := []rowMap{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := make(rowMap, len(names))
		for i, name := range names {
			item[name] = normalizeValue(values[i])
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *Deps) queryRow(ctx context.Context, sql string, args ...any) (rowMap, error) {
	rows, err := d.queryRows(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// ---------- konversi nilai pgtype ----------

func normalizeValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeValue(item)
		}
		return out
	default:
		return normalizeJSONNumeric(v)
	}
}

// ---------- cache in-process kecil ----------

type ttlCache struct {
	mu      sync.Mutex
	entries map[string]ttlEntry
	ttl     time.Duration
}

type ttlEntry struct {
	value     any
	expiresAt time.Time
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{entries: map[string]ttlEntry{}, ttl: ttl}
}

func (c *ttlCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !entry.expiresAt.After(time.Now()) {
		return nil, false
	}
	return entry.value, true
}

func (c *ttlCache) set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = ttlEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
}

func (c *ttlCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if key == "" {
		c.entries = map[string]ttlEntry{}
		return
	}
	delete(c.entries, key)
}

var errNoRows = errors.New("no rows")

// normalizeJSONNumeric: pgtype.Numeric → float64 supaya encoding/json menulis
// angka (bukan objek internal pgtype yang tidak bisa diserialkan).
func normalizeJSONNumeric(v any) any {
	n, ok := v.(pgtype.Numeric)
	if !ok {
		return v
	}
	if !n.Valid {
		return nil
	}
	f8, err := n.Float64Value()
	if err != nil || !f8.Valid {
		return nil
	}
	return f8.Float64
}

// decodeStringArray: parse string JSON array → []string (dipakai rowMap.jsonArr).
func decodeStringArray(raw string) ([]string, bool) {
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out, true
}
