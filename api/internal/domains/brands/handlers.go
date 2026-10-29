package brands

import (
	"context"
	"encoding/json"
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
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Handler fase 2 — port route.ts [brandId]/*: urutan cek, status code,
// audit, dan Cache-Control dipertahankan.

type Deps struct{ Pool *pgxpool.Pool }

type pgxTx = pgx.Tx

func chiURLParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func uniqueBrandPublicID(ctx context.Context, pool *pgxpool.Pool) string {
	for i := 0; i < 60; i++ {
		candidate := auth.BuildShortPublicId()
		var x int
		if err := pool.QueryRow(ctx, "SELECT 1 FROM brands WHERE public_id = $1 LIMIT 1", candidate).Scan(&x); err != nil {
			return candidate
		}
	}
	return auth.BuildShortPublicId()
}

// NewBrandPublicID: id publik pendek untuk brand baru. Dipakai juga domain
// workspace supaya aturan keunikan public_id tidak ditulis dua kali.
func NewBrandPublicID(ctx context.Context, pool *pgxpool.Pool) string {
	return uniqueBrandPublicID(ctx, pool)
}

func strPtr(s string) *string { return &s }

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---------- GET /v1/brands ----------

func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	rows := []BrandRow{}
	// Brand bisa diakses dua cara: baris membership eksplisit, atau keanggotaan
	// workspace tempat brand bernaung. Keduanya digabung supaya daftar ini sama
	// dengan yang lolos BrandAccess; kalau hanya membership yang dibaca, brand
	// workspace tidak pernah muncul di /klien.
	// COALESCE: peran eksplisit tetap menang, workspace hanya fallback.
	qrows, err := d.Pool.Query(r.Context(), `
		SELECT b.id, b.name, b.slug, b.public_id, b.ownership_type, b.status, b.client_name, b.industry,
		       b.primary_language, b.created_at, b.workspace_id,
		       COALESCE(m.role, wm.role) AS role
		FROM brands b
		LEFT JOIN brand_memberships m ON m.brand_id = b.id AND m.user_id = $1
		LEFT JOIN workspace_members wm ON wm.workspace_id = b.workspace_id AND wm.user_id = $1
		WHERE b.deleted_at IS NULL AND (m.id IS NOT NULL OR wm.user_id IS NOT NULL)
		ORDER BY b.created_at LIMIT 500`, user.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer qrows.Close()
	seen := map[string]bool{}
	for qrows.Next() {
		var row BrandRow
		if err := qrows.Scan(&row.ID, &row.Name, &row.Slug, &row.PublicID, &row.OwnershipType, &row.Status,
			&row.ClientName, &row.Industry, &row.PrimaryLanguage, &row.CreatedAt, &row.WorkspaceID, &row.Role); err == nil {
			// Dedupe: konstrain unik mencegah duplikat hari ini, tapi hasil
			// gabungan dua jalur akses tidak boleh bergantung pada itu.
			if seen[row.ID] {
				continue
			}
			seen[row.ID] = true
			rows = append(rows, row)
		}
	}
	out, err := BrandJsonList(r.Context(), d.Pool, rows)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ---------- POST /v1/brands ----------

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type brandCreateBody struct {
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`
	OwnershipType   string  `json:"ownership_type"`
	ClientName      *string `json:"client_name"`
	Industry        *string `json:"industry"`
	PrimaryLanguage string  `json:"primary_language"`
	Domain          *string `json:"domain"`
}

func (d *Deps) Create(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	var body brandCreateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	name := strings.TrimSpace(body.Name)
	slug := body.Slug
	lang := body.PrimaryLanguage
	if lang == "" {
		lang = "id-ID"
	}
	// parity brandCreateSchema (pesan zod inti)
	if len(name) < 2 || len(name) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 2 character(s)")
		return
	}
	if !slugRe.MatchString(slug) || len(slug) < 2 || len(slug) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid")
		return
	}
	if body.OwnershipType != "internal" && body.OwnershipType != "client" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid enum value. Expected 'internal' | 'client'")
		return
	}
	if body.OwnershipType == "client" && (body.ClientName == nil || strings.TrimSpace(*body.ClientName) == "") {
		httpx.Error(w, http.StatusUnprocessableEntity, "client_name is required for a client brand")
		return
	}
	if len(lang) > 20 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid primary_language")
		return
	}

	id := uuid.NewString()
	publicID := uniqueBrandPublicID(r.Context(), d.Pool)

	txErr := db.WithPoolTx(r.Context(), d.Pool, func(tx pgxTx) error {
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO brands (id, name, slug, public_id, ownership_type, client_name, industry, primary_language, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			id, name, slug, publicID, body.OwnershipType, body.ClientName, body.Industry, lang, user.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO brand_memberships (id, brand_id, user_id, role) VALUES ($1, $2, $3, 'owner')",
			uuid.NewString(), id, user.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), "INSERT INTO brand_profiles (brand_id) VALUES ($1)", id); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO voice_profiles (brand_id, personality, preferred_terms, banned_terms) VALUES ($1, '[]', '[]', '[]')", id); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(),
			"INSERT INTO editorial_rule_sets (brand_id, required_rules, forbidden_rules) VALUES ($1, '[]', '[]')", id); err != nil {
			return err
		}
		if body.Domain != nil && *body.Domain != "" {
			if _, err := tx.Exec(r.Context(),
				"INSERT INTO websites (id, brand_id, domain, language, is_primary) VALUES ($1, $2, $3, $4, TRUE)",
				uuid.NewString(), id, strings.ToLower(*body.Domain), lang); err != nil {
				return err
			}
		}
		_, err := tx.Exec(r.Context(),
			"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'brand.created', 'brand', $4)",
			uuid.NewString(), id, user.ID, id)
		return err
	})
	if txErr != nil {
		writeError(w, txErr)
		return
	}

	var row BrandRow
	err := d.Pool.QueryRow(r.Context(),
		"SELECT id, name, slug, public_id, ownership_type, status, client_name, industry, primary_language, created_at FROM brands WHERE id = $1",
		id).Scan(&row.ID, &row.Name, &row.Slug, &row.PublicID, &row.OwnershipType, &row.Status,
		&row.ClientName, &row.Industry, &row.PrimaryLanguage, &row.CreatedAt)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := BrandJsonList(r.Context(), d.Pool, []BrandRow{row})
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(out[0])
}

// ---------- GET /v1/brands/{brandId} ----------

func (d *Deps) Get(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := BrandJsonList(r.Context(), d.Pool, []BrandRow{row})
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=5, stale-while-revalidate=30")
	_ = json.NewEncoder(w).Encode(out[0])
}

// ---------- POST /v1/brands/{brandId}/activate ----------

func (d *Deps) Activate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "strategist")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := BrandJsonList(r.Context(), d.Pool, []BrandRow{row})
	if err != nil {
		writeError(w, err)
		return
	}
	score, _ := out[0]["readiness_score"].(int)
	if score < 70 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": map[string]any{"message": "Brand readiness below 70", "score": score},
		})
		return
	}
	if _, err := d.Pool.Exec(r.Context(), "UPDATE brands SET status = 'active' WHERE id = $1", row.ID); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": row.ID, "status": "active", "readiness_score": score})
}

// ---------- PUT profile / voice / editorial-rules ----------

func (d *Deps) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "strategist")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Positioning      *string `json:"positioning"`
		ValueProposition *string `json:"value_proposition"`
		TargetAudience   *string `json:"target_audience"`
		ProductsServices *string `json:"products_services"`
		Differentiation  *string `json:"differentiation"`
		DefaultCTA       *string `json:"default_cta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO brand_profiles (brand_id, positioning, value_proposition, target_audience, products_services, differentiation, default_cta)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (brand_id) DO UPDATE SET positioning = EXCLUDED.positioning, value_proposition = EXCLUDED.value_proposition,
		  target_audience = EXCLUDED.target_audience, products_services = EXCLUDED.products_services,
		  differentiation = EXCLUDED.differentiation, default_cta = EXCLUDED.default_cta`,
		row.ID, body.Positioning, body.ValueProposition, body.TargetAudience,
		body.ProductsServices, body.Differentiation, body.DefaultCTA); err != nil {
		writeError(w, err)
		return
	}
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, BrandID: &row.ID, Action: "brand.profile.updated",
		TargetType: strPtr("brand"), TargetID: &row.ID,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "updated"})
}

func (d *Deps) UpdateVoice(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "strategist")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Personality   []string `json:"personality"`
		Formality     *int     `json:"formality"`
		Enthusiasm    *int     `json:"enthusiasm"`
		Directness    *int     `json:"directness"`
		Humor         *int     `json:"humor"`
		PointOfView   string   `json:"point_of_view"`
		PreferredTerms []string `json:"preferred_terms"`
		BannedTerms   []string `json:"banned_terms"`
		CtaStyle      *string  `json:"cta_style"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	// parity voiceSchema defaults
	formality := clamp1to5(body.Formality, 3)
	enthusiasm := clamp1to5(body.Enthusiasm, 3)
	directness := clamp1to5(body.Directness, 3)
	humor := clamp1to5(body.Humor, 1)
	pov := body.PointOfView
	if pov == "" {
		pov = "second_person"
	}
	if body.Personality == nil {
		body.Personality = []string{}
	}
	if body.PreferredTerms == nil {
		body.PreferredTerms = []string{}
	}
	if body.BannedTerms == nil {
		body.BannedTerms = []string{}
	}
	personalityJSON, _ := json.Marshal(body.Personality)
	preferredJSON, _ := json.Marshal(body.PreferredTerms)
	bannedJSON, _ := json.Marshal(body.BannedTerms)

	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO voice_profiles (brand_id, personality, formality, enthusiasm, directness, humor, point_of_view, preferred_terms, banned_terms, cta_style)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (brand_id) DO UPDATE SET personality = EXCLUDED.personality, formality = EXCLUDED.formality,
		  enthusiasm = EXCLUDED.enthusiasm, directness = EXCLUDED.directness, humor = EXCLUDED.humor,
		  point_of_view = EXCLUDED.point_of_view, preferred_terms = EXCLUDED.preferred_terms,
		  banned_terms = EXCLUDED.banned_terms, cta_style = EXCLUDED.cta_style`,
		row.ID, string(personalityJSON), formality, enthusiasm, directness, humor, pov,
		string(preferredJSON), string(bannedJSON), body.CtaStyle); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "updated"})
}

func (d *Deps) UpdateRules(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "strategist")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		RequiredRules     []string `json:"required_rules"`
		ForbiddenRules    []string `json:"forbidden_rules"`
		RequiredDisclaimer string  `json:"required_disclaimer"`
		CitationPolicy    string   `json:"citation_policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	// parity rulesSchema: min 1 item / min 1 char
	if len(body.RequiredRules) < 1 || len(body.ForbiddenRules) < 1 ||
		strings.TrimSpace(body.RequiredDisclaimer) == "" || len(body.CitationPolicy) < 1 || len(body.CitationPolicy) > 2000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 1 character(s)")
		return
	}
	requiredJSON, _ := json.Marshal(body.RequiredRules)
	forbiddenJSON, _ := json.Marshal(body.ForbiddenRules)

	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO editorial_rule_sets (brand_id, required_rules, forbidden_rules, required_disclaimer, citation_policy)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (brand_id) DO UPDATE SET required_rules = EXCLUDED.required_rules, forbidden_rules = EXCLUDED.forbidden_rules,
		  required_disclaimer = EXCLUDED.required_disclaimer, citation_policy = EXCLUDED.citation_policy`,
		row.ID, string(requiredJSON), string(forbiddenJSON), body.RequiredDisclaimer, body.CitationPolicy); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "updated"})
}

// ---------- GET configuration ----------

func (d *Deps) Configuration(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeError(w, err)
		return
	}

	var profile *map[string]any
	func() {
		var positioning, valueProp, audience, products, differentiation, defaultCTA *string
		pRow, err := d.Pool.Query(r.Context(),
			"SELECT positioning, value_proposition, target_audience, products_services, differentiation, default_cta FROM brand_profiles WHERE brand_id = $1", row.ID)
		if err != nil {
			return
		}
		defer pRow.Close()
		if pRow.Next() {
			if err := pRow.Scan(&positioning, &valueProp, &audience, &products, &differentiation, &defaultCTA); err == nil {
				profile = &map[string]any{
					"positioning": positioning, "value_proposition": valueProp, "target_audience": audience,
					"products_services": products, "differentiation": differentiation, "default_cta": defaultCTA,
				}
			}
		}
	}()

	var voice *map[string]any
	func() {
		var personality, preferred, banned []byte
		var formality, enthusiasm, directness, humor int
		var pov string
		var ctaStyle *string
		vRow, err := d.Pool.Query(r.Context(),
			"SELECT personality, formality, enthusiasm, directness, humor, point_of_view, preferred_terms, banned_terms, cta_style FROM voice_profiles WHERE brand_id = $1", row.ID)
		if err != nil {
			return
		}
		defer vRow.Close()
		if vRow.Next() {
			if err := vRow.Scan(&personality, &formality, &enthusiasm, &directness, &humor, &pov, &preferred, &banned, &ctaStyle); err == nil {
				voice = &map[string]any{
					"personality": jsonRaw(personality), "formality": formality, "enthusiasm": enthusiasm,
					"directness": directness, "humor": humor, "point_of_view": pov,
					"preferred_terms": jsonRaw(preferred), "banned_terms": jsonRaw(banned), "cta_style": ctaStyle,
				}
			}
		}
	}()

	var rules *map[string]any
	func() {
		var required, forbidden []byte
		var disclaimer, citation *string
		rRow, err := d.Pool.Query(r.Context(),
			"SELECT required_rules, forbidden_rules, required_disclaimer, citation_policy FROM editorial_rule_sets WHERE brand_id = $1", row.ID)
		if err != nil {
			return
		}
		defer rRow.Close()
		if rRow.Next() {
			if err := rRow.Scan(&required, &forbidden, &disclaimer, &citation); err == nil {
				rules = &map[string]any{
					"required_rules": jsonRaw(required), "forbidden_rules": jsonRaw(forbidden),
					"required_disclaimer": disclaimer, "citation_policy": citation,
				}
			}
		}
	}()

	prompts := []map[string]any{}
	pRows, err := d.Pool.Query(r.Context(),
		"SELECT id, version, status, content, change_note, created_at FROM master_prompt_versions WHERE brand_id = $1 ORDER BY version DESC", row.ID)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var p promptRow
			if pRows.Scan(&p.ID, &p.Version, &p.Status, &p.Content, &p.ChangeNote, &p.CreatedAt) == nil {
				prompts = append(prompts, p.toMap())
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=5, stale-while-revalidate=30")
	_ = json.NewEncoder(w).Encode(map[string]any{"role": row.Role, "profile": profile, "voice": voice, "editorial_rules": rules, "prompts": prompts})
}

// jsonRaw: bytea JSONB → nilai JSON mentah (parity jsonb kolom).
func jsonRaw(b []byte) any {
	if b == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	return v
}

var _ = time.Now

// ---------- prompts ----------

func (d *Deps) CreatePrompt(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "strategist")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Content    string  `json:"content"`
		ChangeNote *string `json:"change_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	if len(body.Content) < 20 {
		httpx.Error(w, http.StatusUnprocessableEntity, "String must contain at least 20 character(s)")
		return
	}
	if body.ChangeNote != nil && len(*body.ChangeNote) > 255 {
		httpx.Error(w, http.StatusUnprocessableEntity, "change_note maksimal 255 karakter")
		return
	}
	var maxVersion int
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT COALESCE(MAX(version), 0) FROM master_prompt_versions WHERE brand_id = $1", row.ID).Scan(&maxVersion); err != nil {
		writeError(w, err)
		return
	}
	id := uuid.NewString()
	version := maxVersion + 1
	if _, err := d.Pool.Exec(r.Context(),
		"INSERT INTO master_prompt_versions (id, brand_id, version, status, content, change_note, created_by) VALUES ($1, $2, $3, 'draft', $4, $5, $6)",
		id, row.ID, version, body.Content, body.ChangeNote, user.ID); err != nil {
		writeError(w, err)
		return
	}
	var createdAt promptRow
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT id, version, status, content, change_note, created_at FROM master_prompt_versions WHERE id = $1", id).
		Scan(&createdAt.ID, &createdAt.Version, &createdAt.Status, &createdAt.Content, &createdAt.ChangeNote, &createdAt.CreatedAt); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(createdAt.toMap())
}

type promptRow struct {
	ID        string     `json:"id"`
	Version   int        `json:"version"`
	Status    string     `json:"status"`
	Content   string     `json:"content"`
	ChangeNote *string   `json:"change_note"`
	CreatedAt time.Time  `json:"created_at"`
}

func (p promptRow) toMap() map[string]any {
	return map[string]any{
		"id": p.ID, "version": p.Version, "status": p.Status, "content": p.Content,
		"change_note": p.ChangeNote, "created_at": isoMilli(p.CreatedAt),
	}
}

func (d *Deps) ActivatePrompt(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "strategist")
	if err != nil {
		writeError(w, err)
		return
	}
	promptID := chiURLParam(r, "promptId")
	var p promptRow
	err = d.Pool.QueryRow(r.Context(),
		"SELECT id, version, status, content, change_note, created_at FROM master_prompt_versions WHERE id = $1 AND brand_id = $2",
		promptID, row.ID).Scan(&p.ID, &p.Version, &p.Status, &p.Content, &p.ChangeNote, &p.CreatedAt)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Prompt not found")
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE master_prompt_versions SET status = 'archived' WHERE brand_id = $1 AND status = 'active'", row.ID); err != nil {
		writeError(w, err)
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE master_prompt_versions SET status = 'active' WHERE id = $1", promptID); err != nil {
		writeError(w, err)
		return
	}
	p.Status = "active"
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p.toMap())
}

// clamp1to5: parity int 1..5 dengan default.
func clamp1to5(v *int, def int) int {
	if v == nil {
		return def
	}
	if *v < 1 {
		return 1
	}
	if *v > 5 {
		return 5
	}
	return *v
}


