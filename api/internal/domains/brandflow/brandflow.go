// Package brandflow: port endpoint klien /v1/brands/{brandId}/* yang belum ada
// handler-nya:
//   - campaigns (19)          — kampanye reguler + flow wizard
//   - publications (16)       — publikasi + varian + tracking
//   - publication-plans (7)   — jadwal berulang
//   - batches (7)             — batch generation konten
//   - taut extras (15)        — /taut/agent-status, crawl, files, gatekeeper,
//     produce, writer-links, articles
//   - analyze, assets, usage
//
// Sumber kontrak: apps/web/src/app/v1/brands/[brandId]/**/route.ts +
// lib/{publication,campaign,media,credits,jobs,taut}/*.
//
// Tenancy: akses brand lewat brandsdomain.BrandAccess (id ATAU public_id,
// cache 30s, lintas-tenant → 404). Semua error memakai envelope {"detail": ...}.
package brandflow

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

type Deps struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

// ---------- akses brand ----------

// access: delegasi ke BrandAccess (menerima UUID atau public_id, cache 30s).
func (d *Deps) access(r *http.Request, brandID, userID, minRole string) (brandsdomain.BrandRow, error) {
	return brandsdomain.BrandAccess(r.Context(), d.Pool, brandID, userID, minRole)
}

// brandID: brand kanonik (UUID) dari parameter path.
func (d *Deps) brandID(r *http.Request, userID, minRole string) (string, error) {
	row, err := d.access(r, chiParam(r, "brandId"), userID, minRole)
	if err != nil {
		return "", err
	}
	return row.ID, nil
}

// ---------- error ----------

func errNotFound(msg string) error {
	return &brandsdomain.BrandError{Status: http.StatusNotFound, Message: msg}
}
func errConflict(msg string) error {
	return &brandsdomain.BrandError{Status: http.StatusConflict, Message: msg}
}
func errUnprocessable(msg string) error {
	return &brandsdomain.BrandError{Status: http.StatusUnprocessableEntity, Message: msg}
}
func errBadRequest(msg string) error {
	return &brandsdomain.BrandError{Status: http.StatusBadRequest, Message: msg}
}

// bfStatusError: error umum dengan status eksplisit (429/402/502, dst).
type bfStatusError struct {
	Status  int
	Message string
}

func (e *bfStatusError) Error() string { return e.Message }

// writeErr: memetakan BrandError dan error umum ke envelope {"detail": ...}.
func (d *Deps) writeErr(w http.ResponseWriter, err error) {
	var be *brandsdomain.BrandError
	if errors.As(err, &be) {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	var se *bfStatusError
	if errors.As(err, &se) {
		httpx.Error(w, se.Status, se.Message)
		return
	}
	var ic *insufficientCreditsError
	if errors.As(err, &ic) {
		httpx.Error(w, http.StatusConflict, "Saldo TC tidak cukup.")
		return
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "duplicate") || strings.Contains(lower, "unique") {
		httpx.Error(w, http.StatusConflict, "Brand slug, domain, or membership already exists")
		return
	}
	slog.Error("[brandflow]", "err", err)
	appEnv := ""
	if d.Cfg != nil {
		appEnv = d.Cfg.AppEnv
	}
	httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(appEnv, err.Error()))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// chiParam: pembaca parameter path chi (router memakai {param}).
func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

// slotParam: nomor slot artikel pada satu media (?slot=N), default 1.
//
// Sejak migrasi 272 satu domain bisa punya beberapa artikel, masing-masing
// satu baris taut_campaign_media dengan nomor slot. Parameter ini yang
// membedakan "artikel pertama" dari "artikel kedua" pada media yang sama.
// Nilai kosong/tidak valid jatuh ke slot 1 supaya pemanggil lama (tanpa
// parameter) tetap bekerja apa adanya.
func slotParam(r *http.Request) int {
	raw := strings.TrimSpace(r.URL.Query().Get("slot"))
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 1000 {
		return 1
	}
	return n
}

// ---------- body parsing ----------

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
	return b, ok
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
		f, err := parseFloat(n)
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
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out, true
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func derefOr(p *string, fallback string) string {
	if p == nil || *p == "" {
		return fallback
	}
	return *p
}

// ---------- json helpers ----------

func marshalJSONArr(items []string) string {
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func marshalJSONMap(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ---------- crypto (internal/crypto) ----------

func (d *Deps) sealT2(plain string) (string, error) {
	secret := ""
	if d.Cfg != nil {
		secret = d.Cfg.CmsEncryptionSecret
		if secret == "" {
			secret = d.Cfg.Secret
		}
	}
	return crypto.Seal(plain, secret)
}

func (d *Deps) unsealT2(sealed string) (string, error) {
	cms, app := "", ""
	if d.Cfg != nil {
		cms = d.Cfg.CmsEncryptionSecret
		app = d.Cfg.Secret
	}
	// Data lama ter-seal dengan APP_SECRET (provider.ts legacy), runtime baru
	// memakai CMS_ENCRYPTION_SECRET — coba keduanya.
	return crypto.UnsealCompat(sealed, cms, app)
}

// ---------- audit & notifikasi ----------

// audit: catat audit_logs (best-effort).
func (d *Deps) audit(ctx context.Context, brandID, actorUserID, action, targetType, targetID string) {
	if err := d.exec(ctx,
		`INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)`,
		nullIfEmpty(brandID), nullIfEmpty(actorUserID), action, targetType, targetID); err != nil {
		slog.Warn("[brandflow] audit gagal", "action", action, "err", err)
	}
}

// notifyBrandMembers: fan-out notifikasi ke seluruh anggota brand (tanpa publication_id).
func (d *Deps) notifyBrandMembers(ctx context.Context, brandID, notifType, title, body string, link *string) {
	if err := d.exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, link)
		 SELECT gen_random_uuid(), bm.user_id, $1, $2, $3, $4
		 FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		 WHERE bm.brand_id = $5 AND u.role <> 'admin'`,
		notifType, title, body, link, brandID); err != nil {
		slog.Warn("[brandflow] notifikasi brand gagal", "type", notifType, "err", err)
	}
}

// notifyPlatformAdmins: parity notifyPlatformAdmins (best-effort).
func (d *Deps) notifyPlatformAdmins(ctx context.Context, notifType, title, body string, link *string) {
	if err := d.exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, link)
		 SELECT gen_random_uuid(), id, $1, $2, $3, $4 FROM users WHERE role = 'admin' AND is_active = TRUE`,
		notifType, title, body, link); err != nil {
		slog.Warn("[brandflow] notifikasi admin gagal", "type", notifType, "err", err)
	}
}

// ---------- transaksi ----------

func (d *Deps) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
