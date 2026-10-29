// Package workspace: agency workspace — pengelompokan brand lintas tim.
//
// Satu agency mengelola banyak brand klien. Keanggotaan brand tetap jadi izin
// yang paling spesifik; keanggotaan workspace adalah lapis kedua supaya
// anggota agency tidak perlu didaftarkan ulang di tiap brand (lihat
// brands.EffectiveRole). Handler di sini tidak pernah memutuskan akses brand
// sendiri: kalau butuh peran pada brand, pakai brands.BrandAccess.
//
// Aturan tenancy mengikuti domain brand: bukan anggota (atau perannya kurang)
// dijawab 404, bukan 403, supaya keberadaan workspace lain tidak bocor.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

type Deps struct{ Pool *pgxpool.Pool }

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validMemberRole: peran yang boleh diberikan lewat endpoint members. "owner"
// sengaja tidak ada — pemilik workspace hanya pemilik saat pembuatan.
var validMemberRole = map[string]bool{
	"admin": true, "strategist": true, "writer": true, "reviewer": true, "viewer": true,
}

// ---------- error + util ----------

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string { return e.Message }

func errNotFound() error { return &apiError{http.StatusNotFound, "Workspace not found"} }

func writeErr(w http.ResponseWriter, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		httpx.Error(w, ae.Status, ae.Message)
		return
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "duplicate") || strings.Contains(lower, "unique") {
		httpx.Error(w, http.StatusConflict, "Workspace slug or membership already exists")
		return
	}
	slog.Error("[workspace]", "err", err)
	httpx.Error(w, http.StatusInternalServerError, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func isoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

func strPtr(s string) *string { return &s }

// jsonValue: jsonb → nilai JSON untuk respons. Kolom kosong dianggap objek
// kosong supaya klien tidak perlu menangani null vs {}.
func jsonValue(b []byte) any {
	if len(b) == 0 {
		return map[string]any{}
	}
	var v any
	if json.Unmarshal(b, &v) != nil || v == nil {
		return map[string]any{}
	}
	return v
}

// jsonObject: validasi body jsonb. Selain objek (array, teks) ditolak supaya
// kolom yang di-merge ke brief tidak pernah berisi bentuk yang salah.
func jsonObject(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "{}", nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return "", &apiError{http.StatusUnprocessableEntity, "Invalid Input"}
	}
	return string(raw), nil
}

// ---------- akses workspace ----------

type access struct {
	ID    string
	Name  string
	Slug  string
	Owner string
	Role  string
}

// require: keanggotaan workspace + peran minimum. Non-anggota dan peran kurang
// sama-sama 404 (konvensi tenancy), jadi endpoint ini tidak membocorkan daftar
// workspace yang ada.
func (d *Deps) require(ctx context.Context, workspaceID, userID, minRole string) (access, error) {
	var a access
	err := d.Pool.QueryRow(ctx, `
		SELECT w.id, w.name, w.slug, w.owner_user_id, wm.role
		FROM agency_workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		WHERE w.id = $1 AND wm.user_id = $2 AND w.deleted_at IS NULL
		LIMIT 1`, workspaceID, userID).
		Scan(&a.ID, &a.Name, &a.Slug, &a.Owner, &a.Role)
	if err != nil || brandsdomain.RoleRank(a.Role) < brandsdomain.RoleRank(minRole) {
		return access{}, errNotFound()
	}
	return a, nil
}

// ---------- GET /v1/workspaces ----------

func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	rows, err := d.Pool.Query(r.Context(), `
		SELECT w.id, w.name, w.slug, wm.role, w.created_at
		FROM agency_workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		WHERE wm.user_id = $1 AND w.deleted_at IS NULL
		ORDER BY w.created_at`, user.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, slug, role string
		var createdAt time.Time
		if rows.Scan(&id, &name, &slug, &role, &createdAt) == nil {
			out = append(out, map[string]any{
				"id": id, "name": name, "slug": slug, "role": role, "created_at": isoMilli(createdAt),
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- POST /v1/workspaces ----------

func (d *Deps) Create(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	var body struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	name := strings.TrimSpace(body.Name)
	slug := strings.TrimSpace(body.Slug)
	if len(name) < 2 || len(name) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 2 character(s)")
		return
	}
	if !slugRe.MatchString(slug) || len(slug) < 2 || len(slug) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid")
		return
	}

	id := uuid.NewString()
	var createdAt time.Time
	txErr := db.WithPoolTx(r.Context(), d.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(),
			"INSERT INTO agency_workspaces (id, name, slug, owner_user_id) VALUES ($1, $2, $3, $4) RETURNING created_at",
			id, name, slug, user.ID).Scan(&createdAt); err != nil {
			return err
		}
		// Pembuat langsung jadi owner. Tanpa baris ini workspace tidak bisa
		// dibuka siapa pun, termasuk pembuatnya (semua handler butuh member).
		_, err := tx.Exec(r.Context(),
			"INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')", id, user.ID)
		return err
	})
	if txErr != nil {
		writeErr(w, txErr)
		return
	}
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, Action: "workspace.created",
		TargetType: strPtr("workspace"), TargetID: &id,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": id, "name": name, "slug": slug, "role": "owner", "created_at": isoMilli(createdAt),
	})
}

// ---------- members ----------

func (d *Deps) MembersList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	a, err := d.require(r.Context(), chiParam(r, "workspaceId"), user.ID, "admin")
	if err != nil {
		writeErr(w, err)
		return
	}
	rows, err := d.Pool.Query(r.Context(), `
		SELECT wm.user_id, wm.role, wm.created_at, u.email, u.display_name
		FROM workspace_members wm JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = $1 ORDER BY wm.created_at, u.display_name`, a.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var userID, role, email, displayName string
		var createdAt time.Time
		if rows.Scan(&userID, &role, &createdAt, &email, &displayName) == nil {
			out = append(out, map[string]any{
				"workspace_id": a.ID, "user_id": userID, "role": role,
				"created_at": isoMilli(createdAt), "email": email, "display_name": displayName,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (d *Deps) MembersCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	a, err := d.require(r.Context(), chiParam(r, "workspaceId"), user.ID, "admin")
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	if !validMemberRole[body.Role] {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid enum value. Expected 'admin' | 'strategist' | 'writer' | 'reviewer' | 'viewer'")
		return
	}

	var targetID, targetEmail, displayName string
	if body.UserID != "" {
		err = d.Pool.QueryRow(r.Context(),
			"SELECT id, email, display_name FROM users WHERE id = $1 LIMIT 1", body.UserID).
			Scan(&targetID, &targetEmail, &displayName)
	} else if strings.TrimSpace(body.Email) != "" {
		err = d.Pool.QueryRow(r.Context(),
			"SELECT id, email, display_name FROM users WHERE email = $1 LIMIT 1", strings.ToLower(strings.TrimSpace(body.Email))).
			Scan(&targetID, &targetEmail, &displayName)
	} else {
		httpx.Error(w, http.StatusUnprocessableEntity, "user_id or email is required")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Registered user not found")
		return
	}

	// Guard peran yang sama dengan endpoint members brand: admin tidak boleh
	// membuat atau menyentuh admin lain, apalagi owner.
	exists := false
	var existingRole string
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2", a.ID, targetID).
		Scan(&existingRole); err == nil {
		exists = true
		if existingRole == "owner" {
			httpx.Error(w, http.StatusBadRequest, "Workspace owner cannot be changed")
			return
		}
		if a.Role == "admin" && existingRole == "admin" {
			httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
			return
		}
	}
	if a.Role == "admin" && body.Role == "admin" {
		httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
		return
	}

	// Anggota yang sudah ada diubah perannya (POST ini juga jalur ganti peran,
	// karena hanya ada GET/POST/DELETE pada rute members). Tanpa cabang update,
	// menurunkan peran mustahil kecuali dengan menghapus lalu menambah lagi.
	var createdAt time.Time
	if exists {
		err = d.Pool.QueryRow(r.Context(), `
			UPDATE workspace_members SET role = $3 WHERE workspace_id = $1 AND user_id = $2
			RETURNING created_at`, a.ID, targetID, body.Role).Scan(&createdAt)
	} else {
		err = d.Pool.QueryRow(r.Context(), `
			INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)
			RETURNING created_at`, a.ID, targetID, body.Role).Scan(&createdAt)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	// Cache BrandAccess dikunci brandID+userID, jadi perubahan keanggotaan
	// workspace (tambah, cabut, ganti peran) tidak terlihat tanpa sapuan ini;
	// sampai 30 detik anggota bisa memakai peran lama atau tertahan 404.
	brandsdomain.InvalidateWorkspaceCache(a.ID)
	action, status := "workspace.member.added", http.StatusCreated
	if exists {
		action, status = "workspace.member.role_changed", http.StatusOK
	}
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, Action: action,
		TargetType: strPtr("workspace_member"), TargetID: &targetID,
	})
	writeJSON(w, status, map[string]any{
		"workspace_id": a.ID, "user_id": targetID, "role": body.Role,
		"created_at": isoMilli(createdAt), "email": targetEmail, "display_name": displayName,
	})
}

func (d *Deps) MemberDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	a, err := d.require(r.Context(), chiParam(r, "workspaceId"), user.ID, "admin")
	if err != nil {
		writeErr(w, err)
		return
	}
	targetID := chiParam(r, "userId")
	if targetID == user.ID {
		httpx.Error(w, http.StatusBadRequest, "You cannot remove your own membership")
		return
	}
	var targetRole string
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2", a.ID, targetID).
		Scan(&targetRole); err != nil {
		httpx.Error(w, http.StatusNotFound, "Member not found")
		return
	}
	if targetRole == "owner" {
		httpx.Error(w, http.StatusBadRequest, "Workspace owner cannot be removed")
		return
	}
	if a.Role == "admin" && targetRole == "admin" {
		httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2", a.ID, targetID); err != nil {
		writeErr(w, err)
		return
	}
	// Mencabut keanggotaan workspace mencabut akses fallback ke semua brand
	// workspace itu, jadi seluruh entri cache yang menunjuk workspace ini harus
	// dibuang sekarang juga.
	brandsdomain.InvalidateWorkspaceCache(a.ID)
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, Action: "workspace.member.removed",
		TargetType: strPtr("workspace_member"), TargetID: &targetID,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ---------- rules ----------

func (d *Deps) rulesMap(ctx context.Context, workspaceID string) (map[string]any, error) {
	var entity, style, seo []byte
	var words, claims, updatedBy *string
	var updatedAt *time.Time
	err := d.Pool.QueryRow(ctx, `
		SELECT entity_defaults, style_defaults, seo_defaults, forbidden_words, forbidden_claims, updated_by, updated_at
		FROM workspace_rules WHERE workspace_id = $1`, workspaceID).
		Scan(&entity, &style, &seo, &words, &claims, &updatedBy, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{
			"workspace_id":    workspaceID,
			"entity_defaults": map[string]any{}, "style_defaults": map[string]any{}, "seo_defaults": map[string]any{},
			"forbidden_words": nil, "forbidden_claims": nil, "updated_by": nil, "updated_at": nil,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"workspace_id":    workspaceID,
		"entity_defaults": jsonValue(entity), "style_defaults": jsonValue(style), "seo_defaults": jsonValue(seo),
		"forbidden_words": words, "forbidden_claims": claims,
		"updated_by": updatedBy, "updated_at": updatedAt,
	}, nil
}

func (d *Deps) RulesGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	// Baca aturan boleh semua anggota: ini konteks kerja bersama, bukan data
	// rahasia antar-anggota.
	a, err := d.require(r.Context(), chiParam(r, "workspaceId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	out, err := d.rulesMap(r.Context(), a.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (d *Deps) RulesUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	a, err := d.require(r.Context(), chiParam(r, "workspaceId"), user.ID, "admin")
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		EntityDefaults  json.RawMessage `json:"entity_defaults"`
		StyleDefaults   json.RawMessage `json:"style_defaults"`
		SeoDefaults     json.RawMessage `json:"seo_defaults"`
		ForbiddenWords  *string         `json:"forbidden_words"`
		ForbiddenClaims *string         `json:"forbidden_claims"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	entity, err := jsonObject(body.EntityDefaults)
	if err != nil {
		writeErr(w, err)
		return
	}
	style, err := jsonObject(body.StyleDefaults)
	if err != nil {
		writeErr(w, err)
		return
	}
	seo, err := jsonObject(body.SeoDefaults)
	if err != nil {
		writeErr(w, err)
		return
	}

	// Perubahan aturan TIDAK menyentuh brand yang sudah ada. Nilai ini hanya
	// disemai ke brief pertama brand baru (SeedBrandBriefDefaults), supaya
	// jawaban yang sudah diisi klien tidak tertimpa diam-diam.
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO workspace_rules (workspace_id, entity_defaults, style_defaults, seo_defaults, forbidden_words, forbidden_claims, updated_by, updated_at)
		VALUES ($1, $2::jsonb, $3::jsonb, $4::jsonb, $5, $6, $7, now())
		ON CONFLICT (workspace_id) DO UPDATE SET
		  entity_defaults = EXCLUDED.entity_defaults, style_defaults = EXCLUDED.style_defaults,
		  seo_defaults = EXCLUDED.seo_defaults, forbidden_words = EXCLUDED.forbidden_words,
		  forbidden_claims = EXCLUDED.forbidden_claims, updated_by = EXCLUDED.updated_by,
		  updated_at = now()`,
		a.ID, entity, style, seo, body.ForbiddenWords, body.ForbiddenClaims, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, Action: "workspace.rules.updated",
		TargetType: strPtr("workspace"), TargetID: &a.ID,
	})
	out, err := d.rulesMap(r.Context(), a.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- brands ----------

func (d *Deps) BrandsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	a, err := d.require(r.Context(), chiParam(r, "workspaceId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	// Peran per brand tetap dihitung dengan urutan yang sama seperti
	// EffectiveRole: membership eksplisit menang, workspace fallback.
	rows, err := d.Pool.Query(r.Context(), `
		SELECT b.id, b.name, b.slug, b.public_id, b.ownership_type, b.status, b.client_name, b.industry,
		       b.primary_language, b.created_at, b.workspace_id,
		       COALESCE(bm.role, wm.role) AS role
		FROM brands b
		LEFT JOIN brand_memberships bm ON bm.brand_id = b.id AND bm.user_id = $2
		LEFT JOIN workspace_members wm ON wm.workspace_id = b.workspace_id AND wm.user_id = $2
		WHERE b.workspace_id = $1 AND b.deleted_at IS NULL
		ORDER BY b.created_at`, a.ID, user.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()
	brandRows := []brandsdomain.BrandRow{}
	for rows.Next() {
		var row brandsdomain.BrandRow
		if rows.Scan(&row.ID, &row.Name, &row.Slug, &row.PublicID, &row.OwnershipType, &row.Status,
			&row.ClientName, &row.Industry, &row.PrimaryLanguage, &row.CreatedAt, &row.WorkspaceID, &row.Role) == nil {
			brandRows = append(brandRows, row)
		}
	}
	out, err := brandsdomain.BrandJsonList(r.Context(), d.Pool, brandRows)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (d *Deps) BrandCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	// Writer boleh membuat brand di workspace-nya: tim agency yang menyiapkan
	// brand klien baru tidak perlu menunggu admin, dan pembuatnya langsung
	// mendapat baris membership owner pada brand itu.
	a, err := d.require(r.Context(), chiParam(r, "workspaceId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name       string  `json:"name"`
		Slug       string  `json:"slug"`
		ClientName *string `json:"client_name"`
		Industry   *string `json:"industry"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	name := strings.TrimSpace(body.Name)
	slug := strings.TrimSpace(body.Slug)
	if len(name) < 2 || len(name) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 2 character(s)")
		return
	}
	if !slugRe.MatchString(slug) || len(slug) < 2 || len(slug) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid")
		return
	}
	// client_name menentukan ownership_type, sama seperti form brand: brand
	// dengan nama klien adalah brand klien, selain itu brand internal agency.
	ownershipType := "internal"
	if body.ClientName != nil && strings.TrimSpace(*body.ClientName) != "" {
		ownershipType = "client"
	}

	brandID := uuid.NewString()
	profileID := uuid.NewString()
	publicID := brandsdomain.NewBrandPublicID(r.Context(), d.Pool)
	code := uniqueShortCode(r.Context(), d.Pool)

	txErr := db.WithPoolTx(r.Context(), d.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO brands (id, name, slug, public_id, ownership_type, client_name, industry, primary_language, created_by, workspace_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'id-ID', $8, $9)`,
			brandID, name, slug, publicID, ownershipType, body.ClientName, body.Industry, user.ID, a.ID); err != nil {
			return err
		}
		// Membership owner eksplisit: brand baru tetap bisa dikelola walau
		// keanggotaan workspace berubah nanti, dan peran eksplisit menang atas
		// peran workspace.
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO brand_memberships (id, brand_id, user_id, role) VALUES ($1, $2, $3, 'owner')",
			uuid.NewString(), brandID, user.ID); err != nil {
			return err
		}
		// Brief pertama brand (profiler) supaya seeding aturan workspace punya
		// sasaran. Pola yang sama dengan profil bawaan di app lama.
		_, err := tx.Exec(r.Context(), `
			INSERT INTO profiler_profiles (id, brand_id, name, short_code, created_by)
			VALUES ($1, $2, $3, $4, $5)`, profileID, brandID, name, code, user.ID)
		return err
	})
	if txErr != nil {
		writeErr(w, txErr)
		return
	}
	// Brand yang ditempelkan ke workspace mengubah arti "anggota brand" bagi
	// seluruh anggota workspace, jadi cache peran brand dibuang. Saat ini brand
	// hanya bisa lahir langsung dengan workspace_id; pemanggilan ini juga
	// menjaga jalur attach/detach brand lama nanti.
	brandsdomain.InvalidateWorkspaceCache(a.ID)

	// Seeding best-effort: brand sudah sah terbentuk, jadi kegagalan menulis
	// default tidak boleh membuat klien mengira brand gagal dibuat lalu
	// mencoba lagi dan kena konflik slug.
	if err := SeedBrandBriefDefaults(r.Context(), d.Pool, brandID, profileID, a.ID, user.ID); err != nil {
		slog.Warn("[workspace] seeding aturan brief gagal", "brand", brandID, "workspace", a.ID, "err", err)
	}
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, BrandID: &brandID, Action: "brand.created",
		TargetType: strPtr("brand"), TargetID: &brandID,
	})

	var row brandsdomain.BrandRow
	if err := d.Pool.QueryRow(r.Context(), `
		SELECT id, name, slug, public_id, ownership_type, status, client_name, industry, primary_language, created_at, workspace_id
		FROM brands WHERE id = $1`, brandID).
		Scan(&row.ID, &row.Name, &row.Slug, &row.PublicID, &row.OwnershipType, &row.Status,
			&row.ClientName, &row.Industry, &row.PrimaryLanguage, &row.CreatedAt, &row.WorkspaceID); err != nil {
		writeErr(w, err)
		return
	}
	row.Role = "owner"
	out, err := brandsdomain.BrandJsonList(r.Context(), d.Pool, []brandsdomain.BrandRow{row})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out[0])
}

// uniqueShortCode: kode URL ringkas Profiler (5 karakter), dicoba sampai tidak
// bentrok dengan unique index uq_profiler_profiles_short_code.
func uniqueShortCode(ctx context.Context, pool *pgxpool.Pool) string {
	for i := 0; i < 5; i++ {
		code := strings.ToLower(uuid.NewString())[:5]
		var x int
		if err := pool.QueryRow(ctx, "SELECT 1 FROM profiler_profiles WHERE short_code = $1 LIMIT 1", code).Scan(&x); err != nil {
			return code
		}
	}
	return strings.ToLower(uuid.NewString())[:5]
}

// ---------- seeding brief ----------

// SeedBrandBriefDefaults: tulis nilai default workspace ke brief pertama brand baru.
//
// Hanya kunci yang BELUM ada di profil yang ditulis, supaya aturan agency tidak
// menimpa jawaban yang sudah diisi klien (brand bisa saja dibuat ulang dari
// draft yang sudah berisi). Nilai field ditandai source 'workspace' agar UI
// dan prompt tahu ini warisan, bukan jawaban user.
func SeedBrandBriefDefaults(ctx context.Context, pool *pgxpool.Pool, brandID, profileID, workspaceID, userID string) error {
	var entityRaw, styleRaw, seoRaw []byte
	var forbiddenWords, forbiddenClaims *string
	err := pool.QueryRow(ctx,
		"SELECT entity_defaults, style_defaults, seo_defaults, forbidden_words, forbidden_claims FROM workspace_rules WHERE workspace_id = $1",
		workspaceID).Scan(&entityRaw, &styleRaw, &seoRaw, &forbiddenWords, &forbiddenClaims)
	if errors.Is(err, pgx.ErrNoRows) {
		// Workspace belum punya aturan; bukan error.
		return nil
	}
	if err != nil {
		return err
	}
	// forbidden_words dan forbidden_claims disimpan sebagai teks terpisah di
	// workspace_rules, tetapi di brief keduanya adalah kunci entity_core.
	// Tanpa digabung di sini, dua aturan yang paling ingin dibagikan agensi
	// (istilah terlarang dan batasan klaim) tidak pernah ikut tersemai.
	entityRaw = mergeTextIntoJSON(entityRaw, map[string]*string{
		"forbidden_words":  forbiddenWords,
		"forbidden_claims": forbiddenClaims,
	})

	return db.WithPoolTx(ctx, pool, func(tx pgx.Tx) error {
		var curEntity, curStyle, curSeo []byte
		if err := tx.QueryRow(ctx, `
			SELECT entity_core, style, seo FROM profiler_profiles
			WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL FOR UPDATE`,
			profileID, brandID).Scan(&curEntity, &curStyle, &curSeo); err != nil {
			return err
		}

		sections := []struct {
			column string
			def    []byte
			cur    []byte
		}{
			{"entity_core", entityRaw, curEntity},
			{"style", styleRaw, curStyle},
			{"seo", seoRaw, curSeo},
		}

		setSQL := ""
		args := []any{profileID, brandID, userID}
		written := map[string]bool{}
		for _, s := range sections {
			fresh := missingKeys(s.def, s.cur)
			if len(fresh) == 0 {
				continue
			}
			raw, err := json.Marshal(fresh)
			if err != nil {
				return err
			}
			args = append(args, string(raw))
			if setSQL != "" {
				setSQL += ", "
			}
			// `kolom || $n`: kunci lama di sisi kiri menang. Kunci yang masuk
			// juga sudah disaring di Go, jadi balapan tulis tidak menimpa data
			// yang lebih baru.
			setSQL += fmt.Sprintf("%s = %s || $%d::jsonb", s.column, s.column, len(args))
			for k := range fresh {
				written[k] = true
			}
		}
		if setSQL == "" {
			return nil
		}
		if _, err := tx.Exec(ctx,
			"UPDATE profiler_profiles SET "+setSQL+", updated_by = $3, updated_at = now() WHERE id = $1 AND brand_id = $2",
			args...); err != nil {
			return err
		}

		for key := range written {
			// field_key varchar(80); kunci yang lebih panjang bukan field brief
			// yang dikenal, jadi dilewati daripada membuat seluruh seeding gagal.
			if len(key) > 80 {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO taut_brief_field_state (profile_id, field_key, status, source)
				VALUES ($1, $2, 'filled', 'workspace')
				ON CONFLICT (profile_id, field_key) DO NOTHING`, profileID, key); err != nil {
				return err
			}
		}
		return nil
	})
}

// missingKeys: kunci default yang belum ada di nilai profil saat ini.
func missingKeys(defRaw, curRaw []byte) map[string]any {
	var def, cur map[string]any
	if json.Unmarshal(defRaw, &def) != nil || len(def) == 0 {
		return nil
	}
	_ = json.Unmarshal(curRaw, &cur)
	fresh := map[string]any{}
	for k, v := range def {
		if _, exists := cur[k]; exists {
			continue
		}
		fresh[k] = v
	}
	return fresh
}

// mergeTextIntoJSON menambahkan nilai teks ke objek jsonb default.
//
// Dipakai untuk dua kolom yang bentuknya teks di workspace_rules tetapi harus
// menjadi kunci entity_core di brief. Nilai kosong dilewati supaya aturan yang
// memang belum diisi tidak menimpa data brand dengan string kosong.
func mergeTextIntoJSON(raw []byte, extra map[string]*string) []byte {
	obj := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &obj)
	}
	added := false
	for k, v := range extra {
		if v == nil || strings.TrimSpace(*v) == "" {
			continue
		}
		if _, exists := obj[k]; exists {
			continue
		}
		obj[k] = strings.TrimSpace(*v)
		added = true
	}
	if !added {
		return raw
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

// ---------- attach / detach brand ke workspace ----------

// BrandAttach: POST /v1/workspaces/{workspaceId}/brands/{brandId}/attach
//
// Memindahkan brand lama ke workspace tanpa membuat brand baru. Dibutuhkan
// karena agency_workspaces baru ada setelah brand-nya terlanjur dibuat; tanpa
// jalur ini, brand lama tidak akan pernah bisa memakai aturan induk.
//
// Syarat: admin di workspace DAN admin di brand. Dua-duanya harus, supaya
// pengelola workspace tidak bisa menyerobot brand yang bukan miliknya hanya
// dengan menebak id.
func (d *Deps) BrandAttach(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	workspaceID := chiParam(r, "workspaceId")
	wa, err := d.require(r.Context(), workspaceID, user.ID, "admin")
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = wa
	brandID := chiParam(r, "brandId")
	if _, err := brandsdomain.BrandAccess(r.Context(), d.Pool, brandID, user.ID, "admin"); err != nil {
		writeErr(w, err)
		return
	}

	tag, err := d.Pool.Exec(r.Context(),
		`UPDATE brands SET workspace_id = $1, updated_at = now()
		 WHERE (id = $2 OR public_id = $2) AND deleted_at IS NULL`, workspaceID, brandID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, errNotFound())
		return
	}
	brandsdomain.InvalidateWorkspaceCache(workspaceID)
	if _, err := d.Pool.Exec(r.Context(),
		`INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id, detail)
		 VALUES (gen_random_uuid(), $1, $2, 'workspace.brand_attached', 'workspace', $3, $4::jsonb)`,
		brandID, user.ID, workspaceID, `{"workspace_id":"`+workspaceID+`"}`); err != nil {
		slog.Warn("[workspace] audit attach brand gagal", "brand", brandID, "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "workspace_id": workspaceID})
}

// BrandDetach: DELETE /v1/workspaces/{workspaceId}/brands/{brandId}/attach
//
// Melepas brand dari workspace. Anggota workspace langsung kehilangan akses
// lewat jalur fallback; anggota brand eksplisit tidak terpengaruh.
func (d *Deps) BrandDetach(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	workspaceID := chiParam(r, "workspaceId")
	if _, err := d.require(r.Context(), workspaceID, user.ID, "admin"); err != nil {
		writeErr(w, err)
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := brandsdomain.BrandAccess(r.Context(), d.Pool, brandID, user.ID, "admin"); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		`UPDATE brands SET workspace_id = NULL, updated_at = now()
		 WHERE (id = $1 OR public_id = $1) AND workspace_id = $2 AND deleted_at IS NULL`,
		brandID, workspaceID); err != nil {
		writeErr(w, err)
		return
	}
	brandsdomain.InvalidateWorkspaceCache(workspaceID)
	if _, err := d.Pool.Exec(r.Context(),
		`INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id)
		 VALUES (gen_random_uuid(), $1, $2, 'workspace.brand_detached', 'workspace', $3)`,
		brandID, user.ID, workspaceID); err != nil {
		slog.Warn("[workspace] audit detach brand gagal", "brand", brandID, "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
