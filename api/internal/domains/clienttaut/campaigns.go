package clienttaut

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port app/v1/brands/[brandId]/taut/campaigns/route.ts +
// campaigns/[campaignId]/route.ts (+ submit-final).

// ---------- helpers ----------

// brandID: resolusi akses brand (id ATAU public_id) + minRole; error diteruskan.
func (d *Deps) brandID(r *http.Request, userID, minRole string) (string, error) {
	row, err := d.access(r, chiParam(r, "brandId"), userID, minRole)
	if err != nil {
		return "", err
	}
	return row.ID, nil
}

// access: wrapper BrandAccess (id ATAU public_id, cache 30s, cross-tenant → 404).
func (d *Deps) access(r *http.Request, brandIDOrPublic, userID, minRole string) (brandsdomain.BrandRow, error) {
	return brandsdomain.BrandAccess(r.Context(), d.Pool, brandIDOrPublic, userID, minRole)
}

// parseTextArray: text[] pgx → []string dengan guard null/kosong. Nilai TIDAK
// di-trim (parity `String(arr).replace(/^\{|\}$/g,"").split(",")`).
func parseTextArray(v any) []string {
	switch t := v.(type) {
	case nil:
		return []string{}
	case []string:
		out := []string{}
		for _, s := range t {
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := []string{}
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		s := strings.TrimPrefix(strings.TrimSuffix(t, "}"), "{")
		out := []string{}
		for _, part := range strings.Split(s, ",") {
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	}
	return []string{}
}

// ---------- GET / POST /v1/brands/{brandId}/taut/campaigns ----------

var reDefaultCampaignName = regexp.MustCompile(`^Kampanye Baru( #\d+)?$`)

// TautCampaignsList: GET /v1/brands/{brandId}/taut/campaigns
func (d *Deps) TautCampaignsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}

	q := r.URL.Query()
	archivedOnly := q.Get("archived") == "1"
	search := strings.TrimSpace(q.Get("q"))
	if utf8.RuneCountInString(search) > 100 {
		search = string([]rune(search)[:100])
	}
	page := 1
	if raw := strings.TrimSpace(q.Get("page")); raw != "" {
		page = parseIntDefault(raw, 1)
	}
	if page < 1 {
		page = 1
	}
	const perPage = 9

	archivedClause := "IS NULL"
	if archivedOnly {
		archivedClause = "IS NOT NULL"
	}
	where := "brand_id = $1 AND taut_status IS NOT NULL AND archived_at " + archivedClause
	args := []any{brandID}
	if search != "" {
		where += " AND name ILIKE $2"
		args = append(args, "%"+search+"%")
	}

	totalRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS n FROM campaigns WHERE "+where, args...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	total := totalRow.intOr("n", 0)

	listSQL := "SELECT id, public_id, name, taut_status, taut_entry_mode, media_type, budget_strategy, starts_at, created_at FROM campaigns WHERE " +
		where + " ORDER BY created_at DESC LIMIT " + itoa(perPage) + " OFFSET " + itoa((page-1)*perPage)
	list, err := d.queryRows(r.Context(), listSQL, args...)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	ids := make([]string, 0, len(list))
	for _, c := range list {
		ids = append(ids, c.strOr("id", ""))
	}
	stats, err := campaignStatsMany(r.Context(), d, ids)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	campaigns := make([]map[string]any, 0, len(list))
	for _, c := range list {
		id := c.strOr("id", "")
		entry := map[string]any{
			"id":              id,
			"public_id":       c.raw("public_id"),
			"name":            c.strOr("name", ""),
			"taut_status":     c.raw("taut_status"),
			"entry_mode":      c.raw("taut_entry_mode"),
			"media_type":      c.raw("media_type"),
			"budget_strategy": c.raw("budget_strategy"),
			"starts_at":       c.timePtr("starts_at"),
			"created_at":      c.timePtr("created_at"),
			"archived":        archivedOnly,
			"stats":           stats[id],
		}
		campaigns = append(campaigns, entry)
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"campaigns": campaigns,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		// Kurs aktif supaya daftar kampanye menampilkan anggaran serapan dalam rupiah.
		"rate_idr": d.getTcConfig(r.Context()).RateIDR,
	})
}

// TautCampaignCreate: POST /v1/brands/{brandId}/taut/campaigns
func (d *Deps) TautCampaignCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	rawName, has, err := fieldStr(body, "name")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !has {
		d.writeErr(w, zodInvalidType("string", "undefined"))
		return
	}
	name, err := validateTrimmedString(rawName, 2, 120, "", "")
	if err != nil {
		d.writeErr(w, err)
		return
	}

	startsAtRaw, has, err := fieldStr(body, "starts_at")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !has {
		d.writeErr(w, zodInvalidType("string", "undefined"))
		return
	}
	startsAt, parseErr := parseCoerceDate(startsAtRaw)
	if parseErr != nil {
		d.writeErr(w, parseErr)
		return
	}

	entryMode := "ai"
	if raw, ok, err := fieldStr(body, "entry_mode"); err != nil {
		d.writeErr(w, err)
		return
	} else if ok {
		v, enumErr := validateEnum(raw, "ai", "manual", "kilat")
		if enumErr != nil {
			d.writeErr(w, enumErr)
			return
		}
		entryMode = v
	}

	mediaIDs := []string{}
	if arr, ok, err := fieldArray(body, "media_ids"); err != nil {
		d.writeErr(w, err)
		return
	} else if ok {
		ids, valErr := validateUUIDArray(arr, 250)
		if valErr != nil {
			d.writeErr(w, valErr)
			return
		}
		mediaIDs = ids
	}

	id := uuid.NewString()

	// Nama default "Kampanye Baru #N" dinomori dari MAX(#N)+1 (bukan COUNT).
	resolvedName := name
	if reDefaultCampaignName.MatchString(name) {
		row, err := d.queryRow(r.Context(),
			"SELECT COALESCE(MAX(substring(name from '#([0-9]+)$')::bigint), 0) AS n FROM campaigns WHERE brand_id = $1", brandID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		resolvedName = "Kampanye Baru #" + itoa(row.intOr("n", 0)+1)
	}

	status := "draft"
	if entryMode == "manual" && len(mediaIDs) > 0 {
		status = "placement"
	}
	// public_id diisi DEFAULT DB (public._random_short_id, migrasi 170) supaya
	// insert dari app lama pun ikut dapat; di sini dibaca balik untuk respons.
	created, err := d.queryRow(r.Context(),
		`INSERT INTO campaigns (id, brand_id, name, created_by, status,
		   business_profile, hyperlink_config, starts_at, taut_status, taut_entry_mode, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'active', $5::jsonb, $6::jsonb, $7, $8, $9, NOW(), NOW())
		 RETURNING public_id`,
		id, brandID, resolvedName, user.ID, "{}", "[]", startsAt, status, entryMode)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	publicID := created.strOr("public_id", "")

	added := map[string]any{"added": []string{}, "skipped": []string{}}
	if entryMode == "manual" && len(mediaIDs) > 0 {
		result, err := d.addManualMedia(r.Context(), id, brandID, mediaIDs)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		added = result
	}
	d.auditLog(r, user.ID, "taut.campaign_created", "campaign", id)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              id,
		"public_id":       publicID,
		"name":            resolvedName,
		"taut_status":     status,
		"taut_entry_mode": entryMode,
		"starts_at":       isoMilli(startsAt.UTC()),
		"media":           added,
	})
}

// ---------- GET / PUT / DELETE / POST /v1/brands/{brandId}/taut/campaigns/{campaignId} ----------

// loadTautCampaignForSetup: parity loadCampaign di campaigns/[campaignId]/route.ts
// (taut_status IS NOT NULL, 404 "Kampanye tidak ditemukan.").
func (d *Deps) loadTautCampaignForSetup(ctx context.Context, campaignID, brandID string) (rowMap, error) {
	row, err := d.queryRow(ctx,
		`SELECT id, public_id, name, business_profile, hyperlink_config, media_type, trigger_keywords,
		        budget_strategy, budget_config, strategy_brief, taut_status, taut_entry_mode,
		        editorial_standard_snapshot AS materials
		 FROM campaigns WHERE id = $1 AND brand_id = $2 AND taut_status IS NOT NULL LIMIT 1`,
		campaignID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Kampanye tidak ditemukan.")
	}
	return row, nil
}

// TautCampaignDelete: DELETE /v1/brands/{brandId}/taut/campaigns/{campaignId}
// (soft delete → arsip).
func (d *Deps) TautCampaignDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")

	row, err := d.queryRow(r.Context(),
		"SELECT id, archived_at FROM campaigns WHERE id = $1 AND brand_id = $2 AND taut_status IS NOT NULL LIMIT 1",
		campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE campaigns SET archived_at = NOW(), updated_at = NOW() WHERE id = $1", campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r, user.ID, "taut.campaign_archived", "campaign", campaignID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "archived": true})
}

// TautCampaignAction: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}?action=restore|purge
func (d *Deps) TautCampaignAction(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	action := r.URL.Query().Get("action")

	row, err := d.queryRow(r.Context(),
		"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 AND taut_status IS NOT NULL LIMIT 1", campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}

	switch action {
	case "restore":
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET archived_at = NULL, updated_at = NOW() WHERE id = $1", campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLog(r, user.ID, "taut.campaign_restored", "campaign", campaignID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restored": true})
	case "purge":
		if err := d.exec(r.Context(), "DELETE FROM campaigns WHERE id = $1", campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
		d.auditLog(r, user.ID, "taut.campaign_purged", "campaign", campaignID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "purged": true})
	default:
		httpx.Error(w, http.StatusUnprocessableEntity, "Aksi tidak dikenal. Gunakan action=restore atau action=purge.")
	}
}

// TautCampaignGet: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}
func (d *Deps) TautCampaignGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	c, err := d.loadTautCampaignForSetup(r.Context(), campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	stats, err := campaignStats(r.Context(), d, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"id":               c.strOr("id", ""),
		"public_id":        c.raw("public_id"),
		"name":             c.strOr("name", ""),
		"business_profile": jsonField(c, "business_profile"),
		"hyperlink_config": jsonField(c, "hyperlink_config"),
		"media_type":       c.raw("media_type"),
		"trigger_keywords": parseTextArray(c.raw("trigger_keywords")),
		"materials":        jsonField(c, "materials"),
		"budget_strategy":  c.raw("budget_strategy"),
		"budget_config":    jsonField(c, "budget_config"),
		"strategy_brief":   jsonField(c, "strategy_brief"),
		"taut_status":      c.raw("taut_status"),
		"entry_mode":       c.raw("taut_entry_mode"),
		"stats":            stats,
		// Kurs aktif supaya biaya kampanye ditampilkan dalam rupiah.
		"rate_idr": d.getTcConfig(r.Context()).RateIDR,
	})
}

// TautCampaignUpdate: PUT /v1/brands/{brandId}/taut/campaigns/{campaignId}
// Tahap 2–4 step-wise: media_type → materials → budget_strategy.
func (d *Deps) TautCampaignUpdate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	campaign, err := d.loadTautCampaignForSetup(r.Context(), campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	mediaType, hasMediaType, err := fieldStr(body, "media_type")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if hasMediaType {
		v, enumErr := validateEnum(mediaType, "blogger", "mainstream", "kombinasi")
		if enumErr != nil {
			d.writeErr(w, enumErr)
			return
		}
		mediaType = v
	}

	triggerKeywords := []string{}
	if arr, ok, err := fieldArray(body, "trigger_keywords"); err != nil {
		d.writeErr(w, err)
		return
	} else if ok {
		items, valErr := validateStringArray(arr, 20, 1, 80)
		if valErr != nil {
			d.writeErr(w, valErr)
			return
		}
		triggerKeywords = items
	}

	materials, hasMaterials, err := fieldMap(body, "materials")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if hasMaterials {
		if _, err := validateMaterials(materials); err != nil {
			d.writeErr(w, err)
			return
		}
	}

	budgetStrategy, hasBudgetStrategy, err := fieldStr(body, "budget_strategy")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	budgetConfig, hasBudgetConfig, err := fieldMap(body, "budget_config")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if hasBudgetStrategy {
		v, enumErr := validateEnum(budgetStrategy, "efisien", "optimal", "brutal")
		if enumErr != nil {
			d.writeErr(w, enumErr)
			return
		}
		budgetStrategy = v
		if hasBudgetConfig {
			if err := validateBudgetConfig(budgetConfig); err != nil {
				d.writeErr(w, err)
				return
			}
		}
	}

	if hasMediaType {
		keywordsArg := any(pqTextArray(triggerKeywords))
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET media_type = $1, trigger_keywords = $2, updated_at = NOW() WHERE id = $3",
			mediaType, keywordsArg, campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
		if campaign.strOr("taut_status", "draft") == "draft" {
			if err := d.setCampaignTautStatus(r.Context(), campaignID, "materials"); err != nil {
				d.writeErr(w, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "taut_status": "materials"})
		return
	}

	if hasMaterials {
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET editorial_standard_snapshot = $1::jsonb, updated_at = NOW() WHERE id = $2",
			jsonString(materials), campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
		status := campaign.strOr("taut_status", "")
		if status == "draft" || status == "materials" {
			if err := d.setCampaignTautStatus(r.Context(), campaignID, "budget"); err != nil {
				d.writeErr(w, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "taut_status": "budget"})
		return
	}

	if hasBudgetStrategy {
		if budgetStrategy == "" || !hasBudgetConfig {
			httpx.Error(w, http.StatusUnprocessableEntity, "budget_config wajib menyertai budget_strategy.")
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE campaigns SET budget_strategy = $1, budget_config = $2::jsonb, updated_at = NOW() WHERE id = $3",
			budgetStrategy, jsonString(budgetConfig), campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
		if err := d.advanceTautStatus(r.Context(), campaignID, "budget"); err != nil {
			d.writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "taut_status": "budget", "profiler": map[string]any{"configured": true}})
		return
	}

	httpx.Error(w, http.StatusUnprocessableEntity, "Tidak ada perubahan yang dikirim.")
}

// ---------- submit-final (410 Gone) ----------

// TautSubmitFinal: semua method di /submit-final → 410 setelah auth+akses brand.
func (d *Deps) TautSubmitFinal(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.brandID(r, user.ID, "writer"); err != nil {
		d.writeErr(w, err)
		return
	}
	httpx.Error(w, http.StatusGone, "Distribusi kini melalui Agent 4 Gatekeeper.")
}

// ---------- validasi payload ----------

// validateMaterials: parity setupSchema.materials.
func validateMaterials(m map[string]any) (map[string]any, error) {
	rawBrief, has, err := fieldStr(m, "brief")
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, zodInvalidType("string", "undefined")
	}
	brief, err := validateTrimmedString(rawBrief, 10, 8000, "", "")
	if err != nil {
		return nil, err
	}
	out := map[string]any{"brief": brief}
	if arr, ok, err := fieldArray(m, "key_messages"); err != nil {
		return nil, err
	} else if ok {
		if len(arr) > 15 {
			return nil, zodTooBigArray(15)
		}
		items := []string{}
		for _, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, zodInvalidType("string", receivedType(item))
			}
			if utf8.RuneCountInString(s) > 200 {
				return nil, zodTooBigString(200)
			}
			items = append(items, strings.TrimSpace(s))
		}
		out["key_messages"] = items
	} else {
		out["key_messages"] = []string{}
	}
	if arr, ok, err := fieldArray(m, "references"); err != nil {
		return nil, err
	} else if ok {
		if len(arr) > 15 {
			return nil, zodTooBigArray(15)
		}
		items := []string{}
		for _, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, zodInvalidType("string", receivedType(item))
			}
			trimmed := strings.TrimSpace(s)
			if utf8.RuneCountInString(trimmed) > 500 {
				return nil, zodTooBigString(500)
			}
			if !isZodURL(trimmed) {
				return nil, zodInvalidURL()
			}
			items = append(items, trimmed)
		}
		out["references"] = items
	} else {
		out["references"] = []string{}
	}
	if arr, ok, err := fieldArray(m, "assets"); err != nil {
		return nil, err
	} else if ok {
		if len(arr) > 10 {
			return nil, zodTooBigArray(10)
		}
		items := []string{}
		for _, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, zodInvalidType("string", receivedType(item))
			}
			if utf8.RuneCountInString(s) > 500 {
				return nil, zodTooBigString(500)
			}
			items = append(items, strings.TrimSpace(s))
		}
		out["assets"] = items
	} else {
		out["assets"] = []string{}
	}
	return out, nil
}

// validateBudgetConfig: parity setupSchema.budget_config.
func validateBudgetConfig(m map[string]any) error {
	if v, ok, err := fieldNum(m, "nominal"); err != nil {
		return err
	} else if ok && v <= 0 {
		return zodTooSmallNumberExclusive("0")
	}
	if v, ok, err := fieldNum(m, "duration"); err != nil {
		return err
	} else if ok {
		if v <= 0 {
			return zodTooSmallNumberExclusive("0")
		}
		if v != float64(int(v)) {
			return errUnprocessable("Invalid input: expected int, received number")
		}
	}
	if v, ok, err := fieldStr(m, "unit"); err != nil {
		return err
	} else if ok {
		if _, enumErr := validateEnum(v, "harian", "bulanan"); enumErr != nil {
			return enumErr
		}
	}
	if _, _, err := fieldBool(m, "full_scale"); err != nil {
		return err
	}
	return nil
}

// parseCoerceDate: parity z.coerce.date() — terima string ISO (atau timestamp).
func parseCoerceDate(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, zodInvalidDate()
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, zodInvalidDate()
}

func parseIntDefault(raw string, def int) int {
	n := 0
	neg := false
	if strings.HasPrefix(raw, "-") {
		neg = true
		raw = raw[1:]
	}
	if raw == "" {
		return def
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	if neg {
		n = -n
	}
	return n
}

var _ = context.Background
