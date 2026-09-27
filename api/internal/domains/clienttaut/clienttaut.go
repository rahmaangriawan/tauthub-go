// Package clienttaut: port 48 endpoint Taut Pintar untuk klien —
// app/v1/brands/[brandId]/taut/** di app Next.js lama.
//
// Sumber kontrak: apps/web/src/app/v1/brands/[brandId]/taut/**/route.ts +
// lib/taut/* (agents, orchestrator, schedule, campaign-stats, writer-links,
// json, matchmaker-*), lib/credits/wallet.ts, lib/jobs/queue.ts,
// lib/gatekeeper/*, lib/moderation/*, lib/plagiarism/smodin.ts,
// lib/security/safe-fetch.ts, lib/tracking/short-link.ts.
//
// Tenancy: akses brand lewat brandsdomain.BrandAccess (id ATAU public_id,
// cache 30s, cross-tenant → 404). Semua error memakai envelope {"detail": ...}.
package clienttaut

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

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

// ---------- error mapping (port errorResponse di brands/_helpers.ts) ----------

// tautError: error dengan status HTTP eksplisit (parity apiError/NextResponse).
type tautError struct {
	Status  int
	Message string
}

func (e *tautError) Error() string { return e.Message }

func errStatus(status int, msg string) *tautError { return &tautError{status, msg} }

func errUnprocessable(msg string) *tautError { return errStatus(http.StatusUnprocessableEntity, msg) }
func errNotFound(msg string) *tautError      { return errStatus(http.StatusNotFound, msg) }
func errConflict(msg string) *tautError      { return errStatus(http.StatusConflict, msg) }
func errGone(msg string) *tautError          { return errStatus(http.StatusGone, msg) }

// agentNotConfiguredError: parity AgentNotConfiguredError (409 di errorResponse).
type agentNotConfiguredError struct{ agent string }

func (e *agentNotConfiguredError) Error() string {
	return "Fitur AI \"" + e.agent + "\" belum dikonfigurasi oleh admin. Silakan hubungi admin platform untuk mengaktifkannya."
}

// insufficientCreditsError: parity InsufficientCreditsError lib/credits/wallet.ts.
type insufficientCreditsError struct {
	Needed    int
	Available int
}

func (e *insufficientCreditsError) Error() string { return "Saldo TC tidak cukup." }

// writeErr: port errorResponse — BrandError/RBAC→404, tautError→status, zod→422,
// AgentNotConfigured→409, duplikat→409, sisanya 500 (scrub di production).
func (d *Deps) writeErr(w http.ResponseWriter, err error) {
	var be *brandsdomain.BrandError
	if errors.As(err, &be) {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	var te *tautError
	if errors.As(err, &te) {
		httpx.Error(w, te.Status, te.Message)
		return
	}
	var ance *agentNotConfiguredError
	if errors.As(err, &ance) {
		httpx.Error(w, http.StatusConflict, ance.Error())
		return
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "duplicate") || strings.Contains(lower, "unique") ||
		strings.Contains(lower, "uq_") || strings.Contains(lower, "23505") {
		httpx.Error(w, http.StatusConflict, "Brand slug, domain, or membership already exists")
		return
	}
	slog.Error("[clienttaut]", "err", err)
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

// writeNoStore: parity NextResponse.json(..., { headers: { "Cache-Control": "no-store" } }).
func writeNoStore(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, payload)
}

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

// ---------- audit ----------

// auditLog: parity INSERT INTO audit_logs tanpa brand_id (route create/arsip).
func (d *Deps) auditLog(r *http.Request, userID, action, targetType, targetID string) {
	if _, err := d.Pool.Exec(r.Context(),
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, $2, $3, $4)",
		userID, action, targetType, targetID); err != nil {
		slog.Warn("[clienttaut] audit gagal", "action", action, "err", err)
	}
}

// auditLogBrand: parity varian dengan brand_id (dipakai route advance).
func (d *Deps) auditLogBrand(r *http.Request, brandID, userID, action, targetType, targetID string) {
	if _, err := d.Pool.Exec(r.Context(),
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)",
		brandID, userID, action, targetType, targetID); err != nil {
		slog.Warn("[clienttaut] audit gagal", "action", action, "err", err)
	}
}

// ---------- tiny helpers ----------

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtrOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
