package clienttaut

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// plans.go: Rencana Bulanan.
//
// Klien memberi brief dan anggaran; sistem menyusun rencana beberapa minggu
// (media dan topik per artikel). Klien menyunting dan menyetujui sekali. Tiap
// minggu, saat tanggalnya tiba, sistem membuat KAMPANYE DRAF berisi media dan
// topik itu lalu memberi tahu klien. Tidak ada saldo yang dipotong dan tidak
// ada distribusi otomatis: menulis, QC, dan distribusi tetap dilakukan klien
// lewat alur Taut Pintar. Angka biaya di rencana adalah perkiraan dari harga
// hari ini.
//
// Pemilihan media sengaja deterministik (kesehatan media, DA, harga), bukan
// Matchmaker, karena Matchmaker terikat pada baris kampanye dan hasil rankingnya
// menandai semua kandidat sekaligus. Hanya media yang lolos
// MediaCatalogVisibleSQL (bisa dipesan klien) yang dipilih.

const (
	planMaxWeeks   = 8
	planMaxPerWeek = 3
	planJobKind    = "plan_item_run"
)

type planParams struct {
	Name      string
	Brief     string
	Category  string
	BudgetIDR int
	Weeks     int
	PerWeek   int
	StartDate time.Time
}

type planCandidate struct {
	ID   string
	Name string
	TC   int
}

// planCandidates: media yang bisa dipesan, diurut kesehatan + DA lalu harga.
func (d *Deps) planCandidates(ctx context.Context, category string) ([]planCandidate, error) {
	args := []any{}
	where := ""
	if c := strings.TrimSpace(category); c != "" {
		args = append(args, "%"+c+"%")
		where = " AND m.category ILIKE $1"
	}
	rows, err := d.queryRows(ctx, `
		SELECT m.id, m.name, COALESCE(m.credit_cost, 0) AS tc
		FROM media m
		LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
		WHERE `+dbpkg.MediaCatalogVisibleSQL("m")+` AND COALESCE(m.credit_cost, 0) > 0`+where+`
		ORDER BY (COALESCE(m.health_score, 0) + COALESCE(sm.da, 0)) DESC, m.credit_cost ASC
		LIMIT 60`, args...)
	if err != nil {
		return nil, err
	}
	out := make([]planCandidate, 0, len(rows))
	for _, r := range rows {
		out = append(out, planCandidate{ID: r.strOr("id", ""), Name: r.strOr("name", ""), TC: r.intOr("tc", 0)})
	}
	return out, nil
}

// planTopics: satu topik per artikel. LLM bila agen writer terkonfigurasi,
// selain itu atau bila hasilnya tidak lengkap, topik cadangan dari brief.
func (d *Deps) planTopics(ctx context.Context, brief string, mediaNames []string) []string {
	n := len(mediaNames)
	topics := make([]string, n)
	short := strings.TrimSpace(brief)
	if utf8.RuneCountInString(short) > 80 {
		short = string([]rune(short)[:80]) + "…"
	}
	for i := range topics {
		topics[i] = fmt.Sprintf("Artikel %d: %s", i+1, short)
	}
	if d.getAgentConfig(ctx, "writer").Configured {
		var b strings.Builder
		b.WriteString("Susun topik artikel advertorial untuk rencana konten bulanan. Setiap topik spesifik, tidak berulang, dan relevan dengan brief.\n")
		b.WriteString("Brief: " + strings.TrimSpace(brief) + "\n\n")
		b.WriteString("Media per artikel (urut):\n")
		for i, name := range mediaNames {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, name))
		}
		b.WriteString("\nOutput JSON valid: {\"topics\":[{\"index\":1,\"topic\":\"...\"}]} dengan tepat " + itoa(n) + " item. Topik maksimal 120 karakter, bahasa Indonesia.")
		if res, err := d.callAgentLLM(ctx, "writer", b.String(), true, "", 45000); err == nil && res != nil {
			if parsed := extractJSONObject(res.Content); parsed != nil {
				if arr, ok := parsed["topics"].([]any); ok {
					for _, it := range arr {
						obj, ok := it.(map[string]any)
						if !ok {
							continue
						}
						idx, _ := obj["index"].(float64)
						topic, _ := obj["topic"].(string)
						topic = strings.TrimSpace(topic)
						if i := int(idx) - 1; i >= 0 && i < n && topic != "" {
							if utf8.RuneCountInString(topic) > 160 {
								topic = string([]rune(topic)[:160])
							}
							topics[i] = topic
						}
					}
				}
			}
		}
	}
	return topics
}

// generatePlan: susun dan simpan rencana berstatus draft. Mengembalikan id rencana
// dan peringatan (mis. anggaran tidak cukup untuk semua artikel).
func (d *Deps) generatePlan(ctx context.Context, brandID, userID string, p planParams) (string, []string, error) {
	rate := d.getTcConfig(ctx).RateIDR
	budgetTC := p.BudgetIDR / rate
	writerTC := d.writerCreditCost(ctx)
	candidates, err := d.planCandidates(ctx, p.Category)
	if err != nil {
		return "", nil, err
	}
	want := p.Weeks * p.PerWeek

	type pick struct {
		week int
		c    planCandidate
	}
	picks := make([]pick, 0, want)
	remaining := budgetTC
	used := map[string]int{}
	for slot := 0; slot < want; slot++ {
		week := slot/p.PerWeek + 1
		var chosen *planCandidate
		// Utamakan media yang belum dipakai; kalau semua sudah dipakai, yang paling jarang.
		bestUse := 1 << 30
		for i := range candidates {
			c := &candidates[i]
			if c.TC+writerTC > remaining {
				continue
			}
			if u := used[c.ID]; u < bestUse {
				bestUse, chosen = u, c
			}
		}
		if chosen == nil {
			break
		}
		picks = append(picks, pick{week, *chosen})
		used[chosen.ID]++
		remaining -= chosen.TC + writerTC
	}

	warnings := []string{}
	if len(candidates) == 0 {
		warnings = append(warnings, "Belum ada media yang bisa dipesan untuk kriteria ini.")
	} else if len(picks) < want {
		warnings = append(warnings, fmt.Sprintf("Anggaran hanya cukup untuk %d dari %d artikel. Naikkan anggaran atau kurangi jumlah artikel.", len(picks), want))
	}

	names := make([]string, len(picks))
	for i, pk := range picks {
		names[i] = pk.c.Name
	}
	topics := d.planTopics(ctx, p.Brief, names)

	planID := newUUID()
	var catArg any
	if strings.TrimSpace(p.Category) != "" {
		catArg = strings.TrimSpace(p.Category)
	}
	if err := d.exec(ctx, `
		INSERT INTO content_plans (id, brand_id, created_by, name, brief, category, budget_idr, weeks, per_week, start_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::date)`,
		planID, brandID, userID, p.Name, p.Brief, catArg, p.BudgetIDR, p.Weeks, p.PerWeek, p.StartDate.Format("2006-01-02")); err != nil {
		return "", nil, err
	}
	for i, pk := range picks {
		runOn := p.StartDate.AddDate(0, 0, (pk.week-1)*7)
		if err := d.exec(ctx, `
			INSERT INTO content_plan_items (id, plan_id, week, run_on, media_id, topic, est_media_tc, est_writer_tc)
			VALUES ($1, $2, $3, $4::date, $5, $6, $7, $8)`,
			newUUID(), planID, pk.week, runOn.Format("2006-01-02"), pk.c.ID, topics[i], pk.c.TC, writerTC); err != nil {
			return "", nil, err
		}
	}
	return planID, warnings, nil
}

func planJSON(row rowMap) map[string]any {
	return map[string]any{
		"id": row.strOr("id", ""), "name": row.strOr("name", ""), "brief": row.strOr("brief", ""),
		"category": row.raw("category"), "budget_idr": row.intOr("budget_idr", 0), "weeks": row.intOr("weeks", 0),
		"per_week": row.intOr("per_week", 1), "start_date": row.raw("start_date_text"), "status": row.strOr("status", ""),
		"auto_renew": row.boolOr("auto_renew", false), "approved_at": row.timePtr("approved_at"),
		"created_at": row.timePtr("created_at"),
	}
}

const planSelect = `SELECT id, name, brief, category, budget_idr, weeks, per_week, to_char(start_date, 'YYYY-MM-DD') AS start_date_text,
	status, auto_renew, approved_at, created_at FROM content_plans`

func (d *Deps) planItems(ctx context.Context, planID string) ([]map[string]any, error) {
	rows, err := d.queryRows(ctx, `
		SELECT i.id, i.week, to_char(i.run_on, 'YYYY-MM-DD') AS run_on, i.media_id, m.name AS media_name, m.website_url,
		       i.topic, i.est_media_tc, i.est_writer_tc, i.status, i.campaign_id, i.note
		FROM content_plan_items i JOIN media m ON m.id = i.media_id
		WHERE i.plan_id = $1 ORDER BY i.week, i.created_at`, planID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.strOr("id", ""), "week": r.intOr("week", 0), "run_on": r.strOr("run_on", ""),
			"media_id": r.strOr("media_id", ""), "media_name": r.strOr("media_name", ""), "website_url": r.raw("website_url"),
			"topic": r.strOr("topic", ""), "est_media_tc": r.intOr("est_media_tc", 0), "est_writer_tc": r.intOr("est_writer_tc", 0),
			"status": r.strOr("status", ""), "campaign_id": r.raw("campaign_id"), "note": r.raw("note"),
		})
	}
	return out, nil
}

func planTotals(items []map[string]any) (int, int) {
	media, writer := 0, 0
	for _, it := range items {
		if it["status"] == "skipped" {
			continue
		}
		media += it["est_media_tc"].(int)
		writer += it["est_writer_tc"].(int)
	}
	return media, writer
}

// ---------- handlers ----------

// PlanCreate: POST /v1/brands/{brandId}/taut/plans
func (d *Deps) PlanCreate(w http.ResponseWriter, r *http.Request) {
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
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	str := func(k string) string { s, _ := body[k].(string); return strings.TrimSpace(s) }
	num := func(k string, def int) int {
		if f, ok := body[k].(float64); ok {
			return int(f)
		}
		return def
	}
	p := planParams{
		Name: str("name"), Brief: str("brief"), Category: str("category"),
		BudgetIDR: num("budget_idr", 0), Weeks: num("weeks", 4), PerWeek: num("per_week", 1),
	}
	if n := utf8.RuneCountInString(p.Brief); n < 20 || n > 3000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Brief wajib diisi, 20 sampai 3000 karakter: apa yang dijual, untuk siapa, dan pesan utamanya.")
		return
	}
	if p.BudgetIDR < 50_000 || p.BudgetIDR > 2_000_000_000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Anggaran minimal Rp 50.000.")
		return
	}
	if p.Weeks < 1 || p.Weeks > planMaxWeeks || p.PerWeek < 1 || p.PerWeek > planMaxPerWeek {
		httpx.Error(w, http.StatusUnprocessableEntity, fmt.Sprintf("Jumlah minggu 1 sampai %d dan artikel per minggu 1 sampai %d.", planMaxWeeks, planMaxPerWeek))
		return
	}
	if utf8.RuneCountInString(p.Category) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kategori terlalu panjang.")
		return
	}
	wib := time.FixedZone("WIB", 7*3600)
	p.StartDate = time.Now().In(wib).AddDate(0, 0, 1)
	if s := str("start_date"); s != "" {
		t, perr := time.ParseInLocation("2006-01-02", s, wib)
		if perr != nil || t.Before(time.Now().In(wib).Truncate(24*time.Hour)) {
			httpx.Error(w, http.StatusUnprocessableEntity, "Tanggal mulai tidak valid atau sudah lewat.")
			return
		}
		p.StartDate = t
	}
	if p.Name == "" {
		p.Name = "Rencana " + p.StartDate.Format("Jan 2006")
	}
	if utf8.RuneCountInString(p.Name) > 160 {
		p.Name = string([]rune(p.Name)[:160])
	}

	planID, warnings, err := d.generatePlan(r.Context(), brandID, user.ID, p)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r, user.ID, "taut.plan_created", "content_plan", planID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": planID, "warnings": warnings})
}

// PlansList: GET /v1/brands/{brandId}/taut/plans
func (d *Deps) PlansList(w http.ResponseWriter, r *http.Request) {
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
	rows, err := d.queryRows(r.Context(), planSelect+" WHERE brand_id = $1 ORDER BY created_at DESC LIMIT 50", brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	plans := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		pj := planJSON(row)
		items, _ := d.planItems(r.Context(), row.strOr("id", ""))
		mtc, wtc := planTotals(items)
		ready := 0
		for _, it := range items {
			if it["status"] == "ready" {
				ready++
			}
		}
		pj["items_total"], pj["items_ready"], pj["est_total_tc"] = len(items), ready, mtc+wtc
		plans = append(plans, pj)
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans, "rate_idr": d.getTcConfig(r.Context()).RateIDR})
}

func (d *Deps) loadPlan(r *http.Request, brandID string) (rowMap, error) {
	return d.queryRow(r.Context(), planSelect+" WHERE id = $1 AND brand_id = $2 LIMIT 1", chiParam(r, "planId"), brandID)
}

// PlanGet: GET /v1/brands/{brandId}/taut/plans/{planId}
func (d *Deps) PlanGet(w http.ResponseWriter, r *http.Request) {
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
	row, err := d.loadPlan(r, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Rencana tidak ditemukan.")
		return
	}
	items, err := d.planItems(r.Context(), row.strOr("id", ""))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	mtc, wtc := planTotals(items)
	rate := d.getTcConfig(r.Context()).RateIDR
	pj := planJSON(row)
	pj["items"] = items
	pj["est_media_tc"], pj["est_writer_tc"], pj["est_total_tc"] = mtc, wtc, mtc+wtc
	pj["rate_idr"] = rate
	writeJSON(w, http.StatusOK, pj)
}

// PlanItemUpdate: PATCH /v1/brands/{brandId}/taut/plans/{planId}/items/{itemId}
// Hanya untuk rencana draft: ubah topik, ganti media, atau lewati artikel.
func (d *Deps) PlanItemUpdate(w http.ResponseWriter, r *http.Request) {
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
	plan, err := d.loadPlan(r, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if plan == nil {
		httpx.Error(w, http.StatusNotFound, "Rencana tidak ditemukan.")
		return
	}
	if plan.strOr("status", "") != "draft" {
		httpx.Error(w, http.StatusConflict, "Rencana yang sudah disetujui tidak bisa diubah. Batalkan lalu buat rencana baru.")
		return
	}
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	itemID := chiParam(r, "itemId")
	planID := plan.strOr("id", "")
	if topic, ok := body["topic"].(string); ok {
		topic = strings.TrimSpace(topic)
		if n := utf8.RuneCountInString(topic); n < 3 || n > 160 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Topik 3 sampai 160 karakter.")
			return
		}
		if err := d.exec(r.Context(), "UPDATE content_plan_items SET topic = $1, updated_at = NOW() WHERE id = $2 AND plan_id = $3", topic, itemID, planID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	if skip, ok := body["skip"].(bool); ok {
		status := "pending"
		if skip {
			status = "skipped"
		}
		if err := d.exec(r.Context(), "UPDATE content_plan_items SET status = $1, updated_at = NOW() WHERE id = $2 AND plan_id = $3", status, itemID, planID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	if mediaID, ok := body["media_id"].(string); ok && strings.TrimSpace(mediaID) != "" {
		m, mErr := d.queryRow(r.Context(), `SELECT m.id, COALESCE(m.credit_cost, 0) AS tc, `+dbpkg.MediaCatalogVisibleSQL("m")+` AS ok FROM media m WHERE m.id = $1 LIMIT 1`, strings.TrimSpace(mediaID))
		if mErr != nil {
			d.writeErr(w, mErr)
			return
		}
		if m == nil || !m.boolOr("ok", false) || m.intOr("tc", 0) <= 0 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Media itu tidak bisa dipesan sekarang.")
			return
		}
		if err := d.exec(r.Context(), "UPDATE content_plan_items SET media_id = $1, est_media_tc = $2, updated_at = NOW() WHERE id = $3 AND plan_id = $4",
			m.strOr("id", ""), m.intOr("tc", 0), itemID, planID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PlanApprove: POST /v1/brands/{brandId}/taut/plans/{planId}/approve
func (d *Deps) PlanApprove(w http.ResponseWriter, r *http.Request) {
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
	plan, err := d.loadPlan(r, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if plan == nil {
		httpx.Error(w, http.StatusNotFound, "Rencana tidak ditemukan.")
		return
	}
	var n int
	_ = d.Pool.QueryRow(r.Context(), "SELECT COUNT(*)::int FROM content_plan_items WHERE plan_id = $1 AND status = 'pending'", plan.strOr("id", "")).Scan(&n)
	if n == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Rencana belum punya artikel yang bisa dijalankan.")
		return
	}
	row, err := d.queryRow(r.Context(), `
		UPDATE content_plans SET status = 'approved', approved_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND brand_id = $2 AND status = 'draft' RETURNING id`, plan.strOr("id", ""), brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusConflict, "Rencana ini sudah disetujui atau dibatalkan.")
		return
	}
	d.auditLog(r, user.ID, "taut.plan_approved", "content_plan", plan.strOr("id", ""))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PlanUpdate: PATCH /v1/brands/{brandId}/taut/plans/{planId}  {auto_renew}
// dan POST .../cancel (PlanCancel).
func (d *Deps) PlanUpdate(w http.ResponseWriter, r *http.Request) {
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
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	renew, ok := body["auto_renew"].(bool)
	if !ok {
		httpx.Error(w, http.StatusUnprocessableEntity, "auto_renew wajib true atau false.")
		return
	}
	row, err := d.queryRow(r.Context(), `
		UPDATE content_plans SET auto_renew = $1, updated_at = NOW()
		WHERE id = $2 AND brand_id = $3 AND status IN ('draft', 'approved', 'running') RETURNING id`, renew, chiParam(r, "planId"), brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Rencana tidak ditemukan atau sudah selesai.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *Deps) PlanCancel(w http.ResponseWriter, r *http.Request) {
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
	row, err := d.queryRow(r.Context(), `
		UPDATE content_plans SET status = 'cancelled', auto_renew = FALSE, updated_at = NOW()
		WHERE id = $1 AND brand_id = $2 AND status IN ('draft', 'approved', 'running') RETURNING id`, chiParam(r, "planId"), brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Rencana tidak ditemukan atau sudah selesai.")
		return
	}
	// Artikel yang belum jalan tidak akan dijalankan; yang sudah menjadi kampanye draf tetap ada.
	_ = d.exec(r.Context(), "UPDATE content_plan_items SET status = 'skipped', updated_at = NOW() WHERE plan_id = $1 AND status = 'pending'", chiParam(r, "planId"))
	d.auditLog(r, user.ID, "taut.plan_cancelled", "content_plan", chiParam(r, "planId"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- eksekusi mingguan ----------

// RegisterJobs: processor job paket clienttaut. Dipanggil jobwire.Register.
func (d *Deps) RegisterJobs() {
	jobs.Register(planJobKind, func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["itemId"].(string)
		if id == "" {
			return fmt.Errorf("payload itemId kosong")
		}
		return d.PlanItemRunJob(ctx, id)
	})
}

func (d *Deps) planNotify(ctx context.Context, brandID, notifType, title, body, link string) {
	if err := d.exec(ctx, `
		INSERT INTO notifications (id, user_id, type, title, body, link)
		SELECT gen_random_uuid(), bm.user_id, $2, $3, $4, $5
		FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		WHERE bm.brand_id = $1 AND bm.role IN ('owner', 'admin', 'writer') AND u.role <> 'admin'`,
		brandID, notifType, title, body, link); err != nil {
		slog.Warn("[plan] notifikasi gagal", "brand", brandID, "err", err)
	}
}

// PlanItemRunJob: jalankan satu artikel rencana: buat kampanye draf berisi media
// dan topik itu, lalu beri tahu klien. Idempoten: item diklaim pending -> running
// sekali; job ulang tidak membuat kampanye kedua. Tidak memotong saldo.
func (d *Deps) PlanItemRunJob(ctx context.Context, itemID string) error {
	claimed, err := d.queryRow(ctx, `
		UPDATE content_plan_items i SET status = 'running', updated_at = NOW()
		FROM content_plans p
		WHERE i.id = $1 AND i.status = 'pending' AND p.id = i.plan_id AND p.status IN ('approved', 'running')
		RETURNING i.plan_id, i.media_id, i.topic, i.week, p.brand_id, p.name, p.brief, p.created_by`, itemID)
	if err != nil {
		return err
	}
	if claimed == nil {
		return nil
	}
	planID, mediaID := claimed.strOr("plan_id", ""), claimed.strOr("media_id", "")
	brandID, planName := claimed.strOr("brand_id", ""), claimed.strOr("name", "Rencana")
	week, topic := claimed.intOr("week", 1), claimed.strOr("topic", "")
	_ = d.exec(ctx, "UPDATE content_plans SET status = 'running', updated_at = NOW() WHERE id = $1 AND status = 'approved'", planID)

	fail := func(note string) error {
		_ = d.exec(ctx, "UPDATE content_plan_items SET status = 'needs_attention', note = $1, updated_at = NOW() WHERE id = $2", note, itemID)
		d.planNotify(ctx, brandID, "plan.needs_attention", "Rencana butuh perhatian",
			planName+" minggu "+itoa(week)+": "+note, "/klien/"+brandID+"/rencana/"+planID)
		d.finishPlanIfDone(ctx, planID)
		return nil
	}

	ok, err := d.queryRow(ctx, `SELECT `+dbpkg.MediaCatalogVisibleSQL("m")+` AS ok, m.name FROM media m WHERE m.id = $1 LIMIT 1`, mediaID)
	if err != nil {
		_ = d.exec(ctx, "UPDATE content_plan_items SET status = 'pending', updated_at = NOW() WHERE id = $1", itemID)
		return err
	}
	if ok == nil || !ok.boolOr("ok", false) {
		return fail("media sudah tidak bisa dipesan. Ganti media atau lewati artikel ini di halaman rencana.")
	}

	profile, _ := json.Marshal(map[string]any{
		"business": truncateRunes(claimed.strOr("brief", ""), 500), "usp": topic,
	})
	campaignID := newUUID()
	name := truncateRunes(planName+" minggu "+itoa(week), 120)
	created, err := d.queryRow(ctx, `
		INSERT INTO campaigns (id, brand_id, name, created_by, status, business_profile, hyperlink_config, starts_at,
		                       taut_status, taut_entry_mode, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', $5::jsonb, '[]'::jsonb, NOW(), 'placement', 'kilat', NOW(), NOW())
		RETURNING public_id`, campaignID, brandID, name, claimed.raw("created_by"), string(profile))
	if err != nil {
		slog.Error("[plan] gagal membuat kampanye draf", "item", itemID, "err", err)
		return fail("kampanye draf gagal dibuat. Coba lagi dari halaman rencana atau hubungi dukungan.")
	}
	if _, err := d.addManualMedia(ctx, campaignID, brandID, []string{mediaID}); err != nil {
		slog.Error("[plan] gagal memasang media", "item", itemID, "err", err)
	}
	_ = d.exec(ctx, "UPDATE content_plan_items SET status = 'ready', campaign_id = $1, updated_at = NOW() WHERE id = $2", campaignID, itemID)
	url := campaignID
	if created != nil && created.strOr("public_id", "") != "" {
		url = created.strOr("public_id", "")
	}
	d.planNotify(ctx, brandID, "plan.week_ready", "Artikel minggu "+itoa(week)+" siap ditulis",
		"Kampanye draf \""+name+"\" sudah dibuat dengan topik: "+topic+". Buka untuk menulis, memeriksa mutu, dan mengirimnya. Saldo belum terpotong.",
		"/klien/"+brandID+"/taut-pintar/"+url)
	d.finishPlanIfDone(ctx, planID)
	return nil
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// finishPlanIfDone: tandai rencana selesai bila tak ada item pending atau running.
// Dengan auto_renew, siapkan rencana bulan berikutnya sebagai DRAF (tetap harus
// disetujui klien lagi; tidak ada penagihan otomatis).
func (d *Deps) finishPlanIfDone(ctx context.Context, planID string) {
	row, err := d.queryRow(ctx, `
		UPDATE content_plans p SET status = 'done', updated_at = NOW()
		WHERE p.id = $1 AND p.status = 'running'
		  AND NOT EXISTS (SELECT 1 FROM content_plan_items i WHERE i.plan_id = p.id AND i.status IN ('pending', 'running'))
		RETURNING p.brand_id, p.created_by, p.name, p.brief, p.category, p.budget_idr, p.weeks, p.per_week, p.auto_renew,
		          to_char(p.start_date + (p.weeks * 7), 'YYYY-MM-DD') AS next_start`, planID)
	if err != nil || row == nil || !row.boolOr("auto_renew", false) {
		return
	}
	wib := time.FixedZone("WIB", 7*3600)
	next, perr := time.ParseInLocation("2006-01-02", row.strOr("next_start", ""), wib)
	if perr != nil {
		return
	}
	if next.Before(time.Now().In(wib)) {
		next = time.Now().In(wib).AddDate(0, 0, 1)
	}
	brandID := row.strOr("brand_id", "")
	newID, _, gErr := d.generatePlan(ctx, brandID, row.strOr("created_by", ""), planParams{
		Name: truncateRunes(row.strOr("name", "Rencana")+" (lanjutan)", 160), Brief: row.strOr("brief", ""),
		Category: row.strOr("category", ""), BudgetIDR: row.intOr("budget_idr", 0),
		Weeks: row.intOr("weeks", 4), PerWeek: row.intOr("per_week", 1), StartDate: next,
	})
	if gErr != nil {
		slog.Warn("[plan] gagal menyiapkan rencana lanjutan", "plan", planID, "err", gErr)
		return
	}
	d.planNotify(ctx, brandID, "plan.renewal_draft", "Rencana bulan berikutnya siap ditinjau",
		"Rencana lanjutan sudah disusun sebagai draf dengan harga media hari ini. Tinjau dan setujui bila cocok; tidak ada yang dijalankan atau ditagih sebelum Anda menyetujui.",
		"/klien/"+brandID+"/rencana/"+newID)
}
