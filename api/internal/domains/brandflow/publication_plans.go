package brandflow

import (
	"context"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// publication_plans.go: port app/v1/brands/[brandId]/publication-plans/** +
// lib/publication/plans.ts (create plan, plan JSON).

// zonedTimeToUTC: parity zonedTimeToUtc — "YYYY-MM-DD HH:mm" di zona IANA → UTC.
// Memakai LoadLocation sehingga aturan DST/offset ditangani library.
func zonedTimeToUTC(dateStr, timeStr, timeZone string) (time.Time, error) {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+timeStr, loc)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func isValidIANAZone(zone string) bool {
	if zone == "" {
		return false
	}
	_, err := time.LoadLocation(zone)
	return err == nil
}

// ---------- GET / POST /v1/brands/{brandId}/publication-plans ----------

// PublicationPlansList: GET /v1/brands/{brandId}/publication-plans
func (d *Deps) PublicationPlansList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	plans, err := d.queryRows(r.Context(), `
		SELECT p.id, p.name, p.cadence, p.start_at, p.end_at, p.time_of_day, p.timezone, p.status, p.created_at,
		       p.media_per_day, p.date_mode, p.custom_dates,
		       d.title AS document_title,
		       COUNT(DISTINCT o.id) AS occurrence_total,
		       COUNT(DISTINCT o.id) FILTER (WHERE o.status = 'materialized') AS occurrence_materialized,
		       COUNT(DISTINCT o.id) FILTER (WHERE o.status = 'failed') AS occurrence_failed
		FROM publication_plans p
		LEFT JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN publication_plan_occurrences o ON o.plan_id = p.id
		WHERE p.brand_id = $1
		GROUP BY p.id, p.name, p.cadence, p.start_at, p.end_at, p.time_of_day, p.timezone, p.status, p.created_at, d.title, p.media_per_day, p.date_mode, p.custom_dates
		ORDER BY p.created_at DESC`, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		out = append(out, map[string]any{
			"id":                      p.raw("id"),
			"name":                    p.raw("name"),
			"document_title":          p.raw("document_title"),
			"cadence":                 p.raw("cadence"),
			"start_at":                dateOnly(p.raw("start_at")),
			"end_at":                  dateOnlyOrNil(p.raw("end_at")),
			"time_of_day":             timeHHMM(p.strOr("time_of_day", "")),
			"timezone":                p.raw("timezone"),
			"status":                  p.raw("status"),
			"media_per_day":           p.intOr("media_per_day", 1),
			"date_mode":               p.strOr("date_mode", "consecutive"),
			"custom_dates":            jsonStringList(p.jsonField("custom_dates")),
			"occurrence_total":        p.intOr("occurrence_total", 0),
			"occurrence_materialized": p.intOr("occurrence_materialized", 0),
			"occurrence_failed":       p.intOr("occurrence_failed", 0),
			"created_at":              p.timePtr("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": out})
}

// PublicationPlansCreate: POST /v1/brands/{brandId}/publication-plans
func (d *Deps) PublicationPlansCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	if len(body) == 0 {
		httpx.Error(w, http.StatusBadRequest, "Body tidak valid")
		return
	}
	name, _ := strField(body, "name")
	documentID, _ := strField(body, "document_id")
	mediaIDs, _ := stringSliceField(body, "media_ids")
	startAt, _ := strField(body, "start_at")
	endAt, _ := strField(body, "end_at")
	timeOfDay, _ := strField(body, "time_of_day")
	if timeOfDay == "" {
		timeOfDay = "09:00"
	}
	timezone, _ := strField(body, "timezone")
	if timezone == "" {
		timezone = "Asia/Jakarta"
	}
	mediaPerDay := 1
	if n, ok := intField(body, "media_per_day"); ok {
		mediaPerDay = n
	}
	dateMode := "consecutive"
	if raw, ok := strField(body, "date_mode"); ok && (raw == "random" || raw == "custom") {
		dateMode = raw
	}
	customDates, _ := stringSliceField(body, "custom_dates")

	planID, err := d.createPublicationPlan(r.Context(), brandRow.ID, user.ID, name, documentID, mediaIDs, startAt, endAt, timeOfDay, timezone, mediaPerDay, dateMode, customDates)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.planJSON(r.Context(), planID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

// createPublicationPlan: port createPublicationPlan (lib/publication/plans.ts).
func (d *Deps) createPublicationPlan(
	ctx context.Context, brandID, userID, name, documentID string, mediaIDs []string,
	startAt, endAt, timeOfDay, timezone string, mediaPerDay int, dateMode string, customDates []string,
) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errBadRequest("Nama jadwal wajib diisi")
	}
	if len(mediaIDs) == 0 {
		return "", errBadRequest("Pilih minimal satu media")
	}
	if !reDateOnly.MatchString(startAt) {
		return "", errBadRequest("Tanggal mulai tidak valid")
	}
	if endAt != "" && !reDateOnly.MatchString(endAt) {
		return "", errBadRequest("Tanggal akhir tidak valid")
	}
	if !reHHMM.MatchString(timeOfDay) {
		return "", errBadRequest("Jam tidak valid (HH:mm)")
	}
	if !isValidIANAZone(timezone) {
		return "", errBadRequest("Timezone tidak dikenal: " + timezone)
	}

	doc, err := d.queryRow(ctx,
		"SELECT id FROM content_documents WHERE id = $1 AND brand_id = $2 AND status = 'approved' LIMIT 1", documentID, brandID)
	if err != nil {
		return "", err
	}
	if doc == nil {
		return "", errBadRequest("Artikel tidak ditemukan atau belum approved")
	}

	mediaRows, err := d.queryRows(ctx,
		"SELECT id FROM media WHERE id IN ("+placeholders(1, len(mediaIDs))+") AND status = 'active'", stringArgs(mediaIDs)...)
	if err != nil {
		return "", err
	}
	if len(mediaRows) != len(mediaIDs) {
		return "", errBadRequest("Beberapa media tidak aktif atau tidak ditemukan")
	}

	perDay := mediaPerDay
	if perDay < 1 {
		perDay = 1
	}
	if perDay > 50 {
		perDay = 50
	}
	if !containsString([]string{"consecutive", "random", "custom"}, dateMode) {
		return "", errBadRequest("Pola tanggal tidak dikenal")
	}
	cleanDates := []string{}
	for _, dt := range customDates {
		if reDateOnly.MatchString(dt) {
			cleanDates = append(cleanDates, dt)
		}
	}
	sort.Strings(cleanDates)
	if dateMode == "custom" && len(cleanDates) == 0 {
		return "", errBadRequest("Pilih minimal satu tanggal untuk pola custom")
	}

	start, err := time.Parse("2006-01-02", startAt)
	if err != nil {
		return "", errBadRequest("Tanggal mulai tidak valid")
	}
	var end *time.Time
	if endAt != "" {
		e, err := time.Parse("2006-01-02", endAt)
		if err != nil {
			return "", errBadRequest("Tanggal akhir tidak valid")
		}
		end = &e
	}

	groups := [][]string{}
	for i := 0; i < len(mediaIDs); i += perDay {
		j := i + perDay
		if j > len(mediaIDs) {
			j = len(mediaIDs)
		}
		groups = append(groups, mediaIDs[i:j])
	}

	type groupDate struct {
		dateStr string
		index   int
	}
	groupDates := []groupDate{}
	switch dateMode {
	case "consecutive":
		for i := range groups {
			day := start.AddDate(0, 0, i)
			if end != nil && day.After(*end) {
				break
			}
			groupDates = append(groupDates, groupDate{dateStr: day.Format("2006-01-02"), index: i})
		}
	case "custom":
		for i := range groups {
			dateStr := cleanDates[i%len(cleanDates)]
			groupDates = append(groupDates, groupDate{dateStr: dateStr, index: i})
			if end != nil {
				parsed, err := time.Parse("2006-01-02", dateStr)
				if err == nil && parsed.After(*end) {
					break
				}
			}
		}
	default:
		first := start
		last := start
		if end != nil {
			last = *end
		}
		if last.Before(first) {
			return "", errBadRequest("Tanggal akhir sebelum tanggal mulai")
		}
		daySpan := int(last.Sub(first).Hours()/24) + 1
		pool := []string{}
		for dayIdx := 0; dayIdx < daySpan; dayIdx++ {
			pool = append(pool, first.AddDate(0, 0, dayIdx).Format("2006-01-02"))
		}
		// Fisher-Yates shuffle.
		for i := len(pool) - 1; i > 0; i-- {
			j := rand.Intn(i + 1)
			pool[i], pool[j] = pool[j], pool[i]
		}
		assigned := pool
		if len(assigned) > len(groups) {
			assigned = assigned[:len(groups)]
		}
		sort.Strings(assigned)
		for i, dateStr := range assigned {
			groupDates = append(groupDates, groupDate{dateStr: dateStr, index: i})
		}
	}
	if len(groupDates) == 0 {
		return "", errBadRequest("Tidak ada hari yang dapat dijadwalkan — periksa tanggal")
	}

	type occurrence struct {
		mediaID     string
		scheduledAt time.Time
	}
	occurrences := []occurrence{}
	for _, g := range groupDates {
		for _, mediaID := range groups[g.index] {
			scheduled, err := zonedTimeToUTC(g.dateStr, timeOfDay, timezone)
			if err != nil {
				return "", errBadRequest("Timezone tidak dikenal: " + timezone)
			}
			occurrences = append(occurrences, occurrence{mediaID: mediaID, scheduledAt: scheduled})
		}
	}

	planID := newUUID()
	var customDatesArg any
	if dateMode == "custom" {
		customDatesArg = marshalJSONArr(cleanDates)
	}
	var endArg any
	if endAt != "" {
		endArg = endAt
	}
	err = d.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO publication_plans (id, brand_id, name, document_id, cadence, start_at, end_at, time_of_day, timezone, status, created_by, media_per_day, date_mode, custom_dates)
			VALUES ($1, $2, $3, $4, 'daily', $5, $6, $7, $8, 'active', $9, $10, $11, $12)`,
			planID, brandID, name, documentID, startAt, endArg, timeOfDay, timezone, userID, perDay, dateMode, customDatesArg); err != nil {
			return err
		}
		for i, mediaID := range mediaIDs {
			if _, err := tx.Exec(ctx,
				"INSERT INTO publication_plan_media (plan_id, media_id, position) VALUES ($1, $2, $3)",
				planID, mediaID, i); err != nil {
				return err
			}
		}
		for _, occ := range occurrences {
			if _, err := tx.Exec(ctx,
				"INSERT INTO publication_plan_occurrences (id, plan_id, media_id, scheduled_at, status) VALUES ($1, $2, $3, $4, 'scheduled')",
				newUUID(), planID, occ.mediaID, occ.scheduledAt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return planID, nil
}

// ---------- GET /v1/brands/{brandId}/publication-plans/{planId} ----------

// PublicationPlanGet: GET /v1/brands/{brandId}/publication-plans/{planId}
func (d *Deps) PublicationPlanGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	planID := chiParam(r, "planId")
	plan, err := d.queryRow(r.Context(),
		"SELECT id FROM publication_plans WHERE id = $1 AND brand_id = $2 LIMIT 1", planID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if plan == nil {
		httpx.Error(w, http.StatusNotFound, "Jadwal tidak ditemukan")
		return
	}
	payload, err := d.planJSON(r.Context(), planID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// ---------- aksi plan ----------

// PublicationPlanPause: POST /v1/brands/{brandId}/publication-plans/{planId}/pause
func (d *Deps) PublicationPlanPause(w http.ResponseWriter, r *http.Request) {
	d.planStatusAction(w, r, "paused", func(status string) error {
		if status != "active" {
			return errConflict("Tidak bisa pause dari status " + status)
		}
		return nil
	})
}

// PublicationPlanResume: POST /v1/brands/{brandId}/publication-plans/{planId}/resume
func (d *Deps) PublicationPlanResume(w http.ResponseWriter, r *http.Request) {
	d.planStatusAction(w, r, "active", func(status string) error {
		if status != "paused" {
			return errConflict("Tidak bisa resume dari status " + status)
		}
		return nil
	})
}

// PublicationPlanCancel: POST /v1/brands/{brandId}/publication-plans/{planId}/cancel
func (d *Deps) PublicationPlanCancel(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	planID := chiParam(r, "planId")
	plan, err := d.queryRow(r.Context(),
		"SELECT id, status FROM publication_plans WHERE id = $1 AND brand_id = $2 LIMIT 1", planID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if plan == nil {
		httpx.Error(w, http.StatusNotFound, "Jadwal tidak ditemukan")
		return
	}
	status := plan.strOr("status", "")
	if status == "completed" || status == "cancelled" {
		httpx.Error(w, http.StatusConflict, "Jadwal sudah "+status)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE publication_plans SET status = 'cancelled', updated_at = NOW() WHERE id = $1", planID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE publication_plan_occurrences SET status = 'skipped', updated_at = NOW() WHERE plan_id = $1 AND status = 'scheduled'", planID); err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.planJSON(r.Context(), planID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// planStatusAction: pembungkus pause/resume (transisi status sederhana).
func (d *Deps) planStatusAction(w http.ResponseWriter, r *http.Request, target string, guard func(status string) error) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	planID := chiParam(r, "planId")
	plan, err := d.queryRow(r.Context(),
		"SELECT id, status FROM publication_plans WHERE id = $1 AND brand_id = $2 LIMIT 1", planID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if plan == nil {
		httpx.Error(w, http.StatusNotFound, "Jadwal tidak ditemukan")
		return
	}
	status := plan.strOr("status", "")
	if err := guard(status); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE publication_plans SET status = $1, updated_at = NOW() WHERE id = $2", target, planID); err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.planJSON(r.Context(), planID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// PublicationPlanOccurrenceSkip: POST /v1/brands/{brandId}/publication-plans/occurrences/{occurrenceId}/skip
func (d *Deps) PublicationPlanOccurrenceSkip(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	occurrenceID := chiParam(r, "occurrenceId")
	occ, err := d.queryRow(r.Context(), `
		SELECT o.id, o.plan_id, o.status
		FROM publication_plan_occurrences o
		JOIN publication_plans p ON p.id = o.plan_id
		WHERE o.id = $1 AND p.brand_id = $2 LIMIT 1`, occurrenceID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if occ == nil {
		httpx.Error(w, http.StatusNotFound, "Hari jadwal tidak ditemukan")
		return
	}
	status := occ.strOr("status", "")
	if status != "scheduled" {
		httpx.Error(w, http.StatusConflict, "Hari ini sudah "+status+" — tidak bisa di-skip")
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE publication_plan_occurrences SET status = 'skipped', updated_at = NOW() WHERE id = $1", occurrenceID); err != nil {
		d.writeErr(w, err)
		return
	}
	payload, err := d.planJSON(r.Context(), occ.strOr("plan_id", ""))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// planJSON: port planJson (lib/publication/plans.ts).
func (d *Deps) planJSON(ctx context.Context, planID string) (map[string]any, error) {
	plan, err := d.queryRow(ctx, `
		SELECT p.*, d.title AS document_title
		FROM publication_plans p
		LEFT JOIN content_documents d ON d.id = p.document_id
		WHERE p.id = $1 LIMIT 1`, planID)
	if err != nil || plan == nil {
		return nil, err
	}
	media, err := d.queryRows(ctx, `
		SELECT ppm.media_id, m.name, m.category
		FROM publication_plan_media ppm
		JOIN media m ON m.id = ppm.media_id
		WHERE ppm.plan_id = $1
		ORDER BY ppm.position ASC`, planID)
	if err != nil {
		return nil, err
	}
	occurrences, err := d.queryRows(ctx, `
		SELECT o.id, o.media_id, m.name AS media_name, o.scheduled_at, o.status, o.publication_id, o.error_message
		FROM publication_plan_occurrences o
		LEFT JOIN media m ON m.id = o.media_id
		WHERE o.plan_id = $1
		ORDER BY o.scheduled_at ASC`, planID)
	if err != nil {
		return nil, err
	}

	mediaJSON := make([]map[string]any, 0, len(media))
	for _, m := range media {
		mediaJSON = append(mediaJSON, map[string]any{
			"id": m.raw("media_id"), "name": m.raw("name"), "category": m.raw("category"),
		})
	}
	scheduled, materialized, skipped, failed := 0, 0, 0, 0
	items := make([]map[string]any, 0, len(occurrences))
	for _, o := range occurrences {
		switch o.strOr("status", "") {
		case "scheduled":
			scheduled++
		case "materialized":
			materialized++
		case "skipped":
			skipped++
		case "failed":
			failed++
		}
		items = append(items, map[string]any{
			"id": o.raw("id"), "media_id": o.raw("media_id"), "media_name": o.raw("media_name"),
			"scheduled_at": o.timePtr("scheduled_at"), "status": o.raw("status"),
			"publication_id": o.raw("publication_id"), "error_message": o.raw("error_message"),
		})
	}
	return map[string]any{
		"id":             plan.raw("id"),
		"name":           plan.raw("name"),
		"document_id":    plan.raw("document_id"),
		"document_title": plan.raw("document_title"),
		"cadence":        plan.raw("cadence"),
		"start_at":       dateOnly(plan.raw("start_at")),
		"end_at":         dateOnlyOrNil(plan.raw("end_at")),
		"time_of_day":    timeHHMM(plan.strOr("time_of_day", "")),
		"timezone":       plan.raw("timezone"),
		"status":         plan.raw("status"),
		"media_per_day":  plan.intOr("media_per_day", 1),
		"date_mode":      plan.strOr("date_mode", "consecutive"),
		"custom_dates":   jsonStringList(plan.jsonField("custom_dates")),
		"media_count":    len(mediaJSON),
		"media":          mediaJSON,
		"occurrences": map[string]any{
			"total": len(items), "scheduled": scheduled, "materialized": materialized,
			"skipped": skipped, "failed": failed, "items": items,
		},
		"created_at": plan.timePtr("created_at"),
	}, nil
}

// ---------- helper tanggal ----------

// dateOnly: DATE/as string → "YYYY-MM-DD".
func dateOnly(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case time.Time:
		return t.Format("2006-01-02")
	case string:
		if len(t) >= 10 {
			return t[:10]
		}
		return t
	}
	return nil
}

func dateOnlyOrNil(v any) any {
	if v == nil {
		return nil
	}
	return dateOnly(v)
}

// timeHHMM: TIME column → "HH:MM".
func timeHHMM(v string) string {
	if len(v) >= 5 {
		return v[:5]
	}
	return v
}

// jsonMarshalArrOrNil: helper untuk custom_dates.
func jsonMarshalArrOrNil(items []string, present bool) any {
	if !present {
		return nil
	}
	return marshalJSONArr(items)
}
