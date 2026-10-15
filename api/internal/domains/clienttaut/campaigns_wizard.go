package clienttaut

import (
	"context"
	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port route tahap wizard: advance, validate, schedule, budget-estimate,
// placement, finish.

// loadCampaignForAgents: parity loadCampaign di lib/taut/agents.ts
// (tanpa filter taut_status; error "Kampanye tidak ditemukan.").
func (d *Deps) loadCampaignForAgents(ctx context.Context, campaignID, brandID string) (rowMap, error) {
	row, err := d.queryRow(ctx,
		`SELECT id, brand_id, name, business_profile, hyperlink_config, media_type, trigger_keywords,
		        budget_strategy, budget_config, editorial_standard_snapshot AS materials,
		        tracking_mode, primary_url, primary_anchor, backup_url, backup_anchor, spokepersons
		 FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1`, campaignID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		// Parity: loadCampaign melempar Error polos → errorResponse memetakan 500.
		return nil, errStatus(http.StatusInternalServerError, "Kampanye tidak ditemukan.")
	}
	return row, nil
}

func newUUID() string { return uuid.NewString() }

// ---------- advance ----------

// TautCampaignAdvance: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/advance
func (d *Deps) TautCampaignAdvance(w http.ResponseWriter, r *http.Request) {
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
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	rawTarget, has, err := fieldStr(body, "target")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if !has {
		d.writeErr(w, zodInvalidType("string", "undefined"))
		return
	}
	target, err := validateEnum(rawTarget, "profiled", "placement")
	if err != nil {
		d.writeErr(w, err)
		return
	}

	campaign, err := d.queryRow(r.Context(),
		"SELECT id, taut_status FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if campaign == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}

	if target == "profiled" {
		result, err := d.getLatestCompletedResult(r.Context(), campaignID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if result == nil {
			httpx.Error(w, http.StatusUnprocessableEntity, "Wajib jalankan Analisis Profiler dulu sebelum lanjut.")
			return
		}
		status := campaign.strOr("taut_status", "")
		if status == "" || status == "draft" || status == "profiling" {
			if err := d.exec(r.Context(),
				"UPDATE campaigns SET taut_status = 'profiled', updated_at = NOW() WHERE id = $1", campaignID); err != nil {
				d.writeErr(w, err)
				return
			}
		}
		d.auditLogBrand(r, brandID, user.ID, "taut.profiler_advanced", "taut_profiler_result", result.ID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "taut_status": "profiled", "result_id": result.ID})
		return
	}

	// target == "placement"
	status := campaign.strOr("taut_status", "")
	switch status {
	case "production", "review", "gatekeeper", "submitted", "scheduled", "distributed":
		httpx.Error(w, http.StatusUnprocessableEntity, "Selesaikan Matchmaker dulu sebelum lanjut ke Writer.")
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE campaigns SET taut_status = 'placement', updated_at = NOW() WHERE id = $1", campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLogBrand(r, brandID, user.ID, "taut.placement_advanced", "campaign", campaignID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "taut_status": "placement"})
}

// ---------- validate (tahap 5, Token) ----------

const minTokens = 100

// TautCampaignValidateGet: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/validate
func (d *Deps) TautCampaignValidateGet(w http.ResponseWriter, r *http.Request) {
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
	wallet, err := d.queryRow(r.Context(),
		"SELECT COALESCE(available_tc, 0) AS available FROM credit_wallets WHERE brand_id = $1 LIMIT 1", brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	available := wallet.intOr("available", 0)
	writeNoStore(w, http.StatusOK, map[string]any{
		"available":  available,
		"minimum":    minTokens,
		"sufficient": available >= minTokens,
	})
}

// TautCampaignValidatePost: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/validate
func (d *Deps) TautCampaignValidatePost(w http.ResponseWriter, r *http.Request) {
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
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	hold, _, err := fieldBool(body, "hold")
	if err != nil {
		d.writeErr(w, err)
		return
	}

	wallet, err := d.queryRow(r.Context(),
		"SELECT COALESCE(available_tc, 0) AS available FROM credit_wallets WHERE brand_id = $1 LIMIT 1", brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	available := wallet.intOr("available", 0)
	if available < minTokens {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{
			"detail":    "Saldo token tidak cukup (tersedia " + itoa(available) + ", minimal " + itoa(minTokens) + ").",
			"available": available,
			"minimum":   minTokens,
		})
		return
	}
	if hold {
		if err := d.advanceTautStatus(r.Context(), campaignID, "placement"); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "available": available, "minimum": minTokens})
}

// ---------- schedule (tahap 8) ----------

// TautCampaignSchedule: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/schedule
func (d *Deps) TautCampaignSchedule(w http.ResponseWriter, r *http.Request) {
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
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	mode := "serentak"
	if raw, ok, err := fieldStr(body, "mode"); err != nil {
		d.writeErr(w, err)
		return
	} else if ok {
		v, enumErr := validateEnum(raw, "serentak", "terjadwal")
		if enumErr != nil {
			d.writeErr(w, enumErr)
			return
		}
		mode = v
	}

	timeline, err := d.buildTimeline(r.Context(), campaignID, brandID, mode)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE campaigns SET timeline_plan = $1::jsonb, updated_at = NOW() WHERE id = $2 AND brand_id = $3",
		jsonString(map[string]any{"mode": mode, "items": timeline}), campaignID, brandID); err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.advanceTautStatus(r.Context(), campaignID, "scheduled"); err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "mode": mode, "timeline": timeline})
}

// ---------- budget estimate (tahap 4) ----------

// TautBudgetEstimate: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/budget-estimate
func (d *Deps) TautBudgetEstimate(w http.ResponseWriter, r *http.Request) {
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
	estimate, err := d.estimateBudget(r.Context(), campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, estimate)
}

// ---------- placement (tahap 6) ----------

// tautCampaignCharges: kredit yang SUDAH ditagih kampanye ini — penulisan AI
// (artikel baru + setiap regenerasi) dan gambar AI — dibaca dari ledger. Dipakai
// bar biaya writer supaya uang yang tampil adalah yang benar-benar keluar:
// klien yang menulis atau mengunggah artikelnya sendiri tidak melihat biaya
// penulisan AI sama sekali.
func (d *Deps) tautCampaignCharges(ctx context.Context, campaignID string) (writerTC, writerN, writerRegenN, imageTC, imageN int) {
	row, err := d.queryRow(ctx, `
		SELECT
		  COALESCE(-SUM(amount_tc) FILTER (WHERE ref_id LIKE 'taut-writer:%' OR ref_id LIKE 'taut-writer-regen:%'), 0) AS writer_tc,
		  COUNT(*) FILTER (WHERE ref_id LIKE 'taut-writer:%') AS writer_n,
		  COUNT(*) FILTER (WHERE ref_id LIKE 'taut-writer-regen:%') AS writer_regen_n,
		  COALESCE(-SUM(amount_tc) FILTER (WHERE ref_id LIKE 'ai-image:%'), 0) AS image_tc,
		  COUNT(*) FILTER (WHERE ref_id LIKE 'ai-image:%') AS image_n
		FROM credit_ledger_entries
		WHERE split_part(ref_id, ':', 2) = $1`, campaignID)
	if err != nil {
		return 0, 0, 0, 0, 0
	}
	return row.intOr("writer_tc", 0), row.intOr("writer_n", 0), row.intOr("writer_regen_n", 0), row.intOr("image_tc", 0), row.intOr("image_n", 0)
}

// TautPlacementGet: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/placement
func (d *Deps) TautPlacementGet(w http.ResponseWriter, r *http.Request) {
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
	media, err := d.listPlacement(r.Context(), campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	quota := d.userGenerateQuota(r.Context(), user.ID)
	// writer_cost_credits: tarif penulisan dari platform_settings, supaya bar
	// biaya di frontend tidak memakai angka mati yang bisa berbeda dari tagihan.
	// image_credit_cost: tarif per gambar dari konfigurasi AI, dipakai bar biaya
	// dan panel Biaya Gambar supaya sama dengan yang ditagihkan.
	imageCost := 0
	if rt, rtErr := ai.LoadAIRuntime(r.Context(), d.Pool); rtErr == nil {
		imageCost = ai.ImageCreditCost(rt)
	}
	writerTC, writerN, writerRegenN, imageTC, imageN := d.tautCampaignCharges(r.Context(), campaignID)
	writeNoStore(w, http.StatusOK, map[string]any{
		"media":               media,
		"regen_max":           quota - 1,
		"writer_cost_credits": d.writerCreditCost(r.Context()),
		"image_credit_cost":   imageCost,
		// Nyata yang sudah ditagih kampanye ini (ledger): biaya penulisan &
		// gambar AI. Kamus tarif (di atas) tetap dipakai untuk proyeksi aksi.
		"writer_charged_tc":   writerTC,
		"writer_charge_count": writerN,
		"writer_regen_count":  writerRegenN,
		"image_charged_tc":    imageTC,
		"image_charge_count":  imageN,
	})
}

// TautPlacementPost: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/placement
// (jalankan Matchmaker).
func (d *Deps) TautPlacementPost(w http.ResponseWriter, r *http.Request) {
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
	result, err := d.runMatchmaker(r.Context(), campaignID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, result)
}

// TautPlacementPut: PUT /v1/brands/{brandId}/taut/campaigns/{campaignId}/placement
// (user hapus/ganti/manual seleksi media).
func (d *Deps) TautPlacementPut(w http.ResponseWriter, r *http.Request) {
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
	body, err := requireJSONObject(r)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	selected := []string{}
	hasSelected := false
	if arr, ok, err := fieldArray(body, "selected"); err != nil {
		d.writeErr(w, err)
		return
	} else if ok {
		ids, valErr := validateUUIDArray(arr, 0)
		if valErr != nil {
			d.writeErr(w, valErr)
			return
		}
		selected = ids
		hasSelected = true
	}
	add := []string{}
	if arr, ok, err := fieldArray(body, "add"); err != nil {
		d.writeErr(w, err)
		return
	} else if ok {
		ids, valErr := validateUUIDArray(arr, 20)
		if valErr != nil {
			d.writeErr(w, valErr)
			return
		}
		add = ids
	}

	added := map[string]any{"added": []string{}, "skipped": []string{}}
	if len(add) > 0 {
		result, err := d.addManualMedia(r.Context(), campaignID, brandID, add)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		added = result
	}
	if hasSelected {
		if err := d.exec(r.Context(),
			"UPDATE taut_campaign_media SET status = 'removed' WHERE campaign_id = $1", campaignID); err != nil {
			d.writeErr(w, err)
			return
		}
		if len(selected) > 0 {
			args := []any{campaignID}
			for _, id := range selected {
				args = append(args, id)
			}
			sql := "UPDATE taut_campaign_media SET status = 'selected' WHERE campaign_id = $1 AND media_id IN (" +
				placeholders(2, len(selected)) + ")"
			if err := d.exec(r.Context(), sql, args...); err != nil {
				d.writeErr(w, err)
				return
			}
		}
	}
	payload := map[string]any{"ok": true}
	for k, v := range added {
		payload[k] = v
	}
	writeJSON(w, http.StatusOK, payload)
}

// ---------- finish (tahap 7, Gatekeeper) ----------

// TautCampaignFinish: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/finish
func (d *Deps) TautCampaignFinish(w http.ResponseWriter, r *http.Request) {
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
	results, err := d.runGatekeeper(r.Context(), campaignID, brandID, user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if err := d.advanceTautStatus(r.Context(), campaignID, "review"); err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"items": results})
}

// ---------- time helpers ----------

func trimQuotes(s string) string { return strings.TrimSpace(s) }
