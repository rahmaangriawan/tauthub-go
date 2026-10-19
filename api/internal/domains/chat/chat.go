// Package chat: obrolan klien ↔ mitra per publikasi (forum 3×24 jam).
//
// Satu forum = satu publikasi (brand + campaign + media). Sesi dihitung sejak
// pesan pertama KLIEN dikirim; sesudah lewat, dua pihak tetap bisa membaca
// riwayat tetapi tidak bisa mengirim lagi. Semua akses sisi klien lewat
// BrandAccess (id ATAU public_id, lintas-tenant → 404), sisi mitra lewat
// partners.user_id.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

type Deps struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

// SessionTTL: masa berlaku satu sesi obrolan sejak pesan pertama klien.
const SessionTTL = 72 * time.Hour

// ---------- akses brand (sisi klien) ----------

func (d *Deps) brandAccess(r *http.Request, userID, minRole string) (brandsdomain.BrandRow, error) {
	return brandsdomain.BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), userID, minRole)
}

// ---------- akses mitra (sisi mitra) ----------

type partnerRow struct {
	ID       string
	UserID   string
	Name     string
	PublicID string
}

func (d *Deps) partnerAccess(r *http.Request, userID string) (partnerRow, bool) {
	var p partnerRow
	row := d.Pool.QueryRow(r.Context(),
		"SELECT id, user_id, COALESCE(name, ''), COALESCE(public_id, '') FROM partners WHERE user_id = $1 AND status = 'active' LIMIT 1", userID)
	if err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.PublicID); err != nil {
		return partnerRow{}, false
	}
	return p, true
}

// ---------- error ----------

func writeErr(w http.ResponseWriter, err error) {
	var be *brandsdomain.BrandError
	if errors.As(err, &be) {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	var se *statusError
	if errors.As(err, &se) {
		httpx.Error(w, se.Status, se.Message)
		return
	}
	slog.Error("[chat]", "err", err)
	httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan internal. Coba lagi.")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func chiURLParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

// ---------- notifikasi ----------

// notifyUser: satu baris notifikasi (type 'chat.message').
func (d *Deps) notifyUser(ctx context.Context, userID, title, body, link string) {
	if userID == "" {
		return
	}
	preview := body
	if len([]rune(preview)) > 140 {
		preview = string([]rune(preview)[:140]) + "…"
	}
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link) VALUES (gen_random_uuid(), $1, 'chat.message', $2, $3, $4)",
		userID, title, preview, link); err != nil {
		slog.Warn("[chat] notifikasi gagal", "err", err)
	}
}

// notifyBrandMembers: fan-out ke anggota brand (satu INSERT..SELECT).
// Admin platform tidak perlu menerima notifikasi obrolan (pantauannya di menu
// Log Obrolan). Pengirim pesan juga dikecualikan: akun mitra bisa terdaftar
// sebagai anggota brand (mis. VVIP), jangan sampai menerima notifikasi
// balasannya sendiri.
func (d *Deps) notifyBrandMembers(ctx context.Context, brandID, senderUserID, title, body, link string) {
	preview := body
	if len([]rune(preview)) > 140 {
		preview = string([]rune(preview)[:140]) + "…"
	}
	if err := d.exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, link)
		 SELECT gen_random_uuid(), bm.user_id, 'chat.message', $1, $2, $3
		 FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		 WHERE bm.brand_id = $4 AND u.role <> 'admin' AND bm.user_id <> $5`,
		title, preview, link, brandID, senderUserID); err != nil {
		slog.Warn("[chat] notifikasi brand gagal", "err", err)
	}
}

// ---------- query helpers (rowMap ringkas, pola sama dengan brandflow) ----------

type row map[string]any

func (r row) strOr(key, def string) string {
	if r == nil {
		return def
	}
	if s, ok := r[key].(string); ok {
		return s
	}
	return def
}

func (r row) intOr(key string, def int) int {
	if r == nil {
		return def
	}
	switch n := r[key].(type) {
	case int64:
		return int(n)
	case int32:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	}
	return def
}

func (r row) timePtr(key string) *time.Time {
	if r == nil {
		return nil
	}
	if t, ok := r[key].(time.Time); ok {
		return &t
	}
	return nil
}

func (r row) raw(key string) any {
	if r == nil {
		return nil
	}
	return r[key]
}

func (d *Deps) queryRows(ctx context.Context, sql string, args ...any) ([]row, error) {
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
	out := []row{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := make(row, len(names))
		for i, name := range names {
			item[name] = values[i]
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *Deps) queryRow(ctx context.Context, sql string, args ...any) (row, error) {
	rows, err := d.queryRows(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (d *Deps) exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.Pool.Exec(ctx, sql, args...)
	return err
}

// ---------- body parsing ----------

func decodeJSON(r *http.Request) map[string]any {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		return map[string]any{}
	}
	return body
}

func strField(body map[string]any, key string) string {
	v, ok := body[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
