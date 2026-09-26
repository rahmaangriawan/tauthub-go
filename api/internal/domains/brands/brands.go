// Package brands: port v1/brands/_helpers.ts + route inti fase 2.
// Tenancy: membership wajib, lintas-tenant → 404 (bukan 403).
package brands

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- RBAC (port auth/rbac.ts) ----------

var roleRank = map[string]int{
	"viewer": 10, "reviewer": 20, "writer": 30, "strategist": 40, "admin": 50, "owner": 60,
}

type BrandRole = string

// ---------- error mapping (port errorResponse) ----------

func writeError(w http.ResponseWriter, err error) {
	if be, ok := err.(*BrandError); ok {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
		httpx.Error(w, http.StatusConflict, "Brand slug, domain, or membership already exists")
		return
	}
	slog.Error("[brands]", "err", err)
	httpx.Error(w, http.StatusInternalServerError, err.Error())
}

type BrandError struct {
	Status  int
	Message string
}

func (e *BrandError) Error() string { return e.Message }

func errNotFound() *BrandError      { return &BrandError{404, "Brand not found"} }
func errUnauthorized() *BrandError  { return &BrandError{401, "Invalid or expired access token"} }

// ---------- brand access (port brand() + cache 30s) ----------

type BrandRow struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	PublicID        *string   `json:"public_id"`
	OwnershipType   string    `json:"ownership_type"`
	Status          string    `json:"status"`
	ClientName      *string   `json:"client_name"`
	Industry        *string   `json:"industry"`
	PrimaryLanguage string    `json:"primary_language"`
	CreatedAt       time.Time `json:"created_at"`
	Role            string    `json:"role"`
}

type brandCacheEntry struct {
	row       BrandRow
	expiresAt time.Time
}

var (
	cacheMu   sync.Mutex
	brandCache = map[string]brandCacheEntry{}
)

func InvalidateBrandCache(brandID string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if brandID == "" {
		brandCache = map[string]brandCacheEntry{}
		return
	}
	prefix := brandID + ":"
	for k := range brandCache {
		if strings.HasPrefix(k, prefix) {
			delete(brandCache, k)
		}
	}
}

// BrandAccess: port brand() — lookup by id ATAU public_id + membership, cache 30s.
func BrandAccess(ctx context.Context, pool *pgxpool.Pool, brandID, userID string, minRole string) (BrandRow, error) {
	cacheKey := brandID + ":" + userID
	now := time.Now()
	cacheMu.Lock()
	if c, ok := brandCache[cacheKey]; ok && c.expiresAt.After(now) {
		cacheMu.Unlock()
		if roleRank[c.row.Role] < roleRank[minRole] {
			return BrandRow{}, errNotFound()
		}
		return c.row, nil
	}
	cacheMu.Unlock()

	var row BrandRow
	err := pool.QueryRow(ctx, `
		SELECT b.id, b.name, b.slug, b.public_id, b.ownership_type, b.status, b.client_name, b.industry,
		       b.primary_language, b.created_at, COALESCE(m.role, 'viewer') AS role
		FROM brands b
		JOIN brand_memberships m ON m.brand_id = b.id
		WHERE (b.id = $1 OR b.public_id = $1) AND m.user_id = $2 AND b.deleted_at IS NULL
		LIMIT 1`, brandID, userID).
		Scan(&row.ID, &row.Name, &row.Slug, &row.PublicID, &row.OwnershipType, &row.Status,
			&row.ClientName, &row.Industry, &row.PrimaryLanguage, &row.CreatedAt, &row.Role)
	if err != nil {
		return BrandRow{}, errNotFound()
	}
	if roleRank[row.Role] < roleRank[minRole] {
		return BrandRow{}, errNotFound()
	}

	cacheMu.Lock()
	brandCache[cacheKey] = brandCacheEntry{row, now.Add(30 * time.Second)}
	if row.ID != brandID {
		brandCache[row.ID+":"+userID] = brandCacheEntry{row, now.Add(30 * time.Second)}
	}
	cacheMu.Unlock()
	return row, nil
}

// ---------- brand JSON + readiness (port brandJsonList) ----------

// BrandJsonList: readiness score + websites dengan jumlah query tetap (batch IN).
func BrandJsonList(ctx context.Context, pool *pgxpool.Pool, rows []BrandRow) ([]map[string]any, error) {
	if len(rows) == 0 {
		return []map[string]any{}, nil
	}
	ids := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, r := range rows {
		if !seen[r.ID] {
			ids = append(ids, r.ID)
			seen[r.ID] = true
		}
	}

	// IN list dinamis → $1..$n
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	for i := range ph {
		ph[i] = "$" + itoa(i+1)
	}
	inClause := strings.Join(ph, ", ")

	type websiteRow struct {
		BrandID   string
		ID        string
		Domain    string
		Language  string
		IsPrimary bool
	}
	sitesByBrand := map[string][]map[string]any{}
	siteRows, err := pool.Query(ctx,
		`SELECT brand_id, id, domain, language, is_primary FROM websites WHERE brand_id IN (`+inClause+`) ORDER BY brand_id, is_primary DESC, created_at`, args...)
	if err == nil {
		defer siteRows.Close()
		for siteRows.Next() {
			var w websiteRow
			if err := siteRows.Scan(&w.BrandID, &w.ID, &w.Domain, &w.Language, &w.IsPrimary); err == nil {
				sitesByBrand[w.BrandID] = append(sitesByBrand[w.BrandID], map[string]any{
					"id": w.ID, "domain": w.Domain, "language": w.Language, "is_primary": w.IsPrimary,
				})
			}
		}
	}

	profileExists := map[string]bool{}
	profileComplete := map[string]bool{}
	profileRows, err := pool.Query(ctx,
		`SELECT brand_id, target_audience, value_proposition FROM brand_profiles WHERE brand_id IN (`+inClause+`)`, args...)
	if err == nil {
		defer profileRows.Close()
		firstSeen := map[string]bool{}
		for profileRows.Next() {
			var brandID string
			var audience, valueProp *string
			if err := profileRows.Scan(&brandID, &audience, &valueProp); err == nil && !firstSeen[brandID] {
				firstSeen[brandID] = true
				// Baris pertama per brand menang (parity const [profile] = ...)
				profileExists[brandID] = true
				profileComplete[brandID] = audience != nil && valueProp != nil
			}
		}
	}

	hasVoice := map[string]bool{}
	voiceRows, err := pool.Query(ctx, `SELECT brand_id FROM voice_profiles WHERE brand_id IN (`+inClause+`)`, args...)
	if err == nil {
		defer voiceRows.Close()
		for voiceRows.Next() {
			var brandID string
			if voiceRows.Scan(&brandID) == nil {
				hasVoice[brandID] = true
			}
		}
	}

	hasRules := map[string]bool{}
	rulesRows, err := pool.Query(ctx, `SELECT brand_id FROM editorial_rule_sets WHERE brand_id IN (`+inClause+`)`, args...)
	if err == nil {
		defer rulesRows.Close()
		for rulesRows.Next() {
			var brandID string
			if rulesRows.Scan(&brandID) == nil {
				hasRules[brandID] = true
			}
		}
	}

	hasPrompt := map[string]bool{}
	promptRows, err := pool.Query(ctx, `SELECT brand_id FROM master_prompt_versions WHERE brand_id IN (`+inClause+`) AND status = 'active'`, args...)
	if err == nil {
		defer promptRows.Close()
		for promptRows.Next() {
			var brandID string
			if promptRows.Scan(&brandID) == nil {
				hasPrompt[brandID] = true
			}
		}
	}

	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		score := 10
		if row.Name != "" && row.Industry != nil && *row.Industry != "" {
			score = 20
		}
		if profileExists[row.ID] {
			if profileComplete[row.ID] {
				score += 25
			} else {
				score += 10
			}
		}
		if hasVoice[row.ID] {
			score += 20
		}
		if hasRules[row.ID] {
			score += 15
		}
		if hasPrompt[row.ID] {
			score += 20
		}
		if score > 100 {
			score = 100
		}
		websites := sitesByBrand[row.ID]
		if websites == nil {
			websites = []map[string]any{}
		}
		entry := map[string]any{
			"id": row.ID, "name": row.Name, "slug": row.Slug, "public_id": row.PublicID,
			"ownership_type": row.OwnershipType, "status": row.Status, "client_name": row.ClientName,
			"industry": row.Industry, "primary_language": row.PrimaryLanguage,
			"created_at": isoMilli(row.CreatedAt),
			"readiness_score": score, "websites": websites,
		}
		// Parity: JSON create dari app lama tidak punya field role (row polos);
		// role hanya muncul pada list/detail yang join membership.
		if row.Role != "" {
			entry["role"] = row.Role
		}
		out = append(out, entry)
	}
	return out, nil
}

func itoa(i int) string { return strconv.Itoa(i) }

func isoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

