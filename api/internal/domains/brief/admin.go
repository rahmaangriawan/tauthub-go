package brief

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// Endpoint admin Taut Brief (/v1/platform/taut-brief/*), terisolasi dari
// /v1/platform/taut-agents. Semua handler: adminGuard → wajib super admin →
// 404 bila bukan (parity: keberadaan bidang admin disembunyikan).

func (d *Deps) adminGate(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, ctx, ok := d.Admin.Require(w, r, "")
	if !ok {
		return "", false
	}
	if !ctx.IsSuperAdmin() {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Resource not found."))
		return "", false
	}
	return user.ID, true
}

func (d *Deps) briefAudit(ctx context.Context, userID, action, targetID string) {
	_, _ = d.Pool.Exec(ctx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id)
		 VALUES (gen_random_uuid(), $1, $2, 'taut_brief', $3)`,
		userID, action, targetID)
}

// ---------- config ----------

func (d *Deps) TautBriefConfigGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	cfg, err := d.loadConfig(r.Context())
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	var (
		sealed         *string
		baseURL, model *string
		maxTokens      *int
	)
	_ = d.Pool.QueryRow(r.Context(),
		`SELECT base_url, model, max_tokens, sealed_api_key FROM taut_brief_config WHERE id = 'default'`).
		Scan(&baseURL, &model, &maxTokens, &sealed)

	w.Header().Set("Cache-Control", "no-store")
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{
		"base_url":           derefStr(baseURL),
		"model":              derefStr(model),
		"temperature":        cfg.Temperature,
		"max_tokens":         maxTokens,
		"timeout_ms":         cfg.TimeoutMS,
		"rate_limit_per_min": cfg.RateLimitPerMin,
		"enabled":            cfg.Enabled,
		"api_key_configured": sealed != nil && *sealed != "",
		"capabilities":       cfg.Capabilities,
		"configured":         cfg.configured(),
	})
}

func (d *Deps) TautBriefConfigUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	body := adminshared.DecodeJSON(r)

	baseURL, hasBaseURL := adminshared.StrField(body, "base_url")
	apiKey, hasAPIKey := adminshared.StrField(body, "api_key")
	model, hasModel := adminshared.StrField(body, "model")
	maxTokens, hasMaxTokens := adminshared.IntField(body, "max_tokens")
	timeoutMs, hasTimeout := adminshared.IntField(body, "timeout_ms")
	rateLimit, hasRateLimit := adminshared.IntField(body, "rate_limit_per_min")
	enabled, hasEnabled := adminshared.BoolField(body, "enabled")
	temperature, hasTemperature := adminshared.NumField(body, "temperature")

	if hasBaseURL {
		if !isHTTPURL(baseURL) {
			d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Invalid url"))
			return
		}
		if len(baseURL) > 500 {
			d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "String harus 500 karakter atau kurang"))
			return
		}
	}
	if hasAPIKey && apiKey != "" && len(apiKey) > 500 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "String harus 500 karakter atau kurang"))
		return
	}
	if hasModel && len(model) > 120 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "String harus 120 karakter atau kurang"))
		return
	}
	if hasTemperature && (temperature < 0 || temperature > 2) {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Temperature harus antara 0 dan 2."))
		return
	}
	if hasMaxTokens && (maxTokens <= 0 || maxTokens > 200000) {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Max tokens harus antara 1 dan 200000."))
		return
	}
	if hasTimeout && (timeoutMs < 5000 || timeoutMs > 300000) {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Timeout minimal 5000 ms dan maksimal 300000 ms."))
		return
	}
	if hasRateLimit && (rateLimit < 0 || rateLimit > 600) {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Rate limit 0-600 permintaan per menit. Isikan 0 untuk tanpa batas."))
		return
	}

	var sealedKey any
	if hasAPIKey && apiKey != "" {
		sealed, err := crypto.SealV3(apiKey, d.Cfg.CmsEncryptionSecret, configEntityID)
		if err != nil {
			d.Admin.WriteErr(w, err)
			return
		}
		sealedKey = sealed
	}
	temperatureArg := any(nil)
	if hasTemperature {
		temperatureArg = temperature
	}
	maxTokensArg := any(nil)
	if hasMaxTokens {
		maxTokensArg = maxTokens
	}
	timeoutArg := any(nil)
	if hasTimeout {
		timeoutArg = timeoutMs
	}
	rateLimitArg := any(nil)
	if hasRateLimit {
		rateLimitArg = rateLimit
	}
	enabledArg := any(nil)
	if hasEnabled {
		enabledArg = enabled
	}
	baseURLArg := any(nil)
	if hasBaseURL {
		baseURLArg = nullIfEmpty(baseURL)
	}
	modelArg := any(nil)
	if hasModel {
		modelArg = nullIfEmpty(model)
	}

	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO taut_brief_config
		  (id, base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms,
		   rate_limit_per_min, enabled, updated_by, updated_at)
		VALUES ('default', $1, $2, $3, COALESCE($4::numeric, 0.3), $5, COALESCE($6::int, 60000),
		        COALESCE($7::int, 12), COALESCE($8::bool, false), $9, NOW())
		ON CONFLICT (id) DO UPDATE SET
		  base_url = COALESCE(EXCLUDED.base_url, taut_brief_config.base_url),
		  sealed_api_key = COALESCE(EXCLUDED.sealed_api_key, taut_brief_config.sealed_api_key),
		  model = COALESCE(EXCLUDED.model, taut_brief_config.model),
		  temperature = COALESCE(EXCLUDED.temperature, taut_brief_config.temperature),
		  max_tokens = COALESCE(EXCLUDED.max_tokens, taut_brief_config.max_tokens),
		  timeout_ms = COALESCE(EXCLUDED.timeout_ms, taut_brief_config.timeout_ms),
		  rate_limit_per_min = COALESCE(EXCLUDED.rate_limit_per_min, taut_brief_config.rate_limit_per_min),
		  enabled = COALESCE(EXCLUDED.enabled, taut_brief_config.enabled),
		  updated_by = EXCLUDED.updated_by,
		  updated_at = NOW()`,
		baseURLArg, sealedKey, modelArg, temperatureArg, maxTokensArg,
		timeoutArg, rateLimitArg, enabledArg, nullIfEmpty(userID)); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.config_updated", "default")
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautBriefConfigTest: dua probe. (a) JSON mode, (b) JSON mode sambil streaming.
// Hasilnya disimpan di settings.capabilities supaya tiap giliran tidak lagi
// mencoba kombinasi yang providernya sudah terbukti tidak mendukung.
func (d *Deps) TautBriefConfigTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	cfg, err := d.loadConfig(r.Context())
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	if !cfg.configured() {
		adminshared.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": "Belum dikonfigurasi (base_url / api key / model kosong atau disabled).",
		})
		return
	}
	client := &ai.ChatClient{
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Temperature: 0, Timeout: 60 * time.Second,
	}
	probeMsgs := []ai.ChatMessage{
		{Role: "system", Content: "Balas hanya dengan JSON."},
		{Role: "user", Content: `Balas persis: {"ok":true}`},
	}

	jsonOK := false
	jsonNote := ""
	started := time.Now()
	if res, err := client.CompleteMessages(r.Context(), probeMsgs, true); err != nil {
		jsonNote = err.Error()
	} else if _, perr := parseEngineOutput(res.Content); perr != nil {
		jsonNote = "respons tidak bisa dibaca sebagai objek JSON"
	} else {
		jsonOK = true
	}
	latencyJSON := time.Since(started).Milliseconds()

	streamOK := false
	streamNote := ""
	if jsonOK {
		started = time.Now()
		if res, err := client.CompleteStreamMessages(r.Context(), probeMsgs, true, func(string) {}); err != nil {
			streamNote = err.Error()
		} else if res.Streamed {
			streamOK = true
		} else {
			streamNote = "gateway membalas JSON biasa, bukan text/event-stream"
		}
	} else {
		streamNote = "dilewati karena probe JSON mode gagal"
	}
	latencyStream := time.Since(started).Milliseconds()

	caps := map[string]any{
		"json_mode":   jsonOK,
		"stream_json": streamOK,
		"probed_at":   time.Now().UTC().Format(time.RFC3339),
		"json_note":   jsonNote,
		"stream_note": streamNote,
		"model":       cfg.Model,
		"latency_ms":  map[string]any{"json": latencyJSON, "stream": latencyStream},
	}
	if raw, mErr := json.Marshal(caps); mErr == nil {
		_, _ = d.Pool.Exec(r.Context(),
			`UPDATE taut_brief_config
			 SET settings = COALESCE(settings, '{}'::jsonb) || jsonb_build_object('capabilities', $1::jsonb),
			     updated_at = NOW()
			 WHERE id = 'default'`, string(raw))
	}

	message := "JSON mode OK · streaming JSON didukung"
	if !jsonOK {
		message = "JSON mode gagal"
	} else if !streamOK {
		message = "JSON mode OK · streaming JSON tidak didukung (brief memakai jalur non-streaming)"
	}
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": jsonOK, "message": message, "capabilities": caps,
	})
}

// ---------- versi prompt ----------

func (d *Deps) TautBriefPromptsList(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{
		"items": d.listVersions(r.Context(), "taut_brief_prompt_versions"),
	})
}

func (d *Deps) TautBriefPromptGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	var (
		id, status   string
		version      int
		prompt, note *string
		createdAt    time.Time
	)
	err := d.Pool.QueryRow(r.Context(),
		`SELECT id, version, status, prompt, change_note, created_at
		 FROM taut_brief_prompt_versions WHERE id = $1`,
		adminshared.ChiParam(r, "versionId")).Scan(&id, &version, &status, &prompt, &note, &createdAt)
	if err != nil {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Versi prompt tidak ditemukan."))
		return
	}
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"version": map[string]any{
		"id": id, "version": version, "status": status, "prompt": prompt,
		"change_note": note, "created_at": adminshared.IsoMilli(createdAt),
	}})
}

func (d *Deps) TautBriefPromptCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	body := adminshared.DecodeJSON(r)
	prompt, _ := adminshared.StrField(body, "prompt")
	changeNote, _ := adminshared.StrField(body, "change_note")
	cloneActive, _ := adminshared.BoolField(body, "clone_active")
	if len(prompt) > 20000 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Prompt maksimal 20.000 karakter."))
		return
	}
	if len(changeNote) > 255 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Catatan maksimal 255 karakter."))
		return
	}
	if cloneActive && prompt == "" {
		var active *string
		_ = d.Pool.QueryRow(r.Context(),
			`SELECT prompt FROM taut_brief_prompt_versions WHERE status = 'active' LIMIT 1`).Scan(&active)
		prompt = derefStr(active)
	}
	next, err := d.nextVersion(r.Context(), "taut_brief_prompt_versions")
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	id := uuid.NewString()
	if _, err := d.Pool.Exec(r.Context(),
		`WITH archived AS (
		   UPDATE taut_brief_prompt_versions SET status = 'archived' WHERE status = 'active'
		 )
		 INSERT INTO taut_brief_prompt_versions
		   (id, version, status, prompt, change_note, created_by, published_by, published_at)
		 VALUES ($1, $2, 'active', $3, $4, $5, $5, NOW())`,
		id, next, nullIfEmpty(prompt), nullIfEmpty(changeNote), userID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.prompt_created", id)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "version": next})
}

func (d *Deps) TautBriefPromptUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	body := adminshared.DecodeJSON(r)
	prompt, hasPrompt := adminshared.StrField(body, "prompt")
	changeNote, hasNote := adminshared.StrField(body, "change_note")
	if hasPrompt && len(prompt) > 20000 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Prompt maksimal 20.000 karakter."))
		return
	}
	if hasNote && len(changeNote) > 255 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Catatan maksimal 255 karakter."))
		return
	}
	versionID := adminshared.ChiParam(r, "versionId")
	if _, err := d.Pool.Exec(r.Context(),
		`UPDATE taut_brief_prompt_versions
		 SET prompt = COALESCE($1, prompt), change_note = COALESCE($2, change_note)
		 WHERE id = $3`,
		nullableStr(prompt, hasPrompt), nullableStr(changeNote, hasNote), versionID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.prompt_updated", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *Deps) TautBriefPromptDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	versionID := adminshared.ChiParam(r, "versionId")
	status, found, count, err := d.versionStatus(r.Context(), "taut_brief_prompt_versions", versionID)
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	if !found {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Versi prompt tidak ditemukan."))
		return
	}
	if count <= 1 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusConflict, "Tidak bisa menghapus satu-satunya versi."))
		return
	}
	if status == "active" {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusConflict, "Versi aktif tidak bisa dihapus. Aktifkan versi lain dulu."))
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"DELETE FROM taut_brief_prompt_versions WHERE id = $1", versionID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.prompt_deleted", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *Deps) TautBriefPromptActivate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	versionID := adminshared.ChiParam(r, "versionId")
	if _, found, _, err := d.versionStatus(r.Context(), "taut_brief_prompt_versions", versionID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	} else if !found {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Versi prompt tidak ditemukan."))
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		`WITH archived AS (
		   UPDATE taut_brief_prompt_versions SET status = 'archived' WHERE status = 'active'
		 )
		 UPDATE taut_brief_prompt_versions
		 SET status = 'active', published_by = $1, published_at = NOW()
		 WHERE id = $2`, userID, versionID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.prompt_published", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- versi skema field ----------

func (d *Deps) TautBriefFieldsList(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{
		"items": d.listVersions(r.Context(), "taut_brief_field_versions"),
	})
}

func (d *Deps) TautBriefFieldGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	var (
		id, status string
		version    int
		raw        []byte
		note       *string
		createdAt  time.Time
	)
	err := d.Pool.QueryRow(r.Context(),
		`SELECT id, version, status, fields, change_note, created_at
		 FROM taut_brief_field_versions WHERE id = $1`,
		adminshared.ChiParam(r, "versionId")).Scan(&id, &version, &status, &raw, &note, &createdAt)
	if err != nil {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Versi skema tidak ditemukan."))
		return
	}
	var fields []Field
	_ = json.Unmarshal(raw, &fields)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"version": map[string]any{
		"id": id, "version": version, "status": status, "fields": fields,
		"change_note": note, "created_at": adminshared.IsoMilli(createdAt),
	}})
}

// TautBriefFieldCreate menyimpan daftar field sebagai VERSI BARU yang langsung
// aktif, meniru pola versi prompt: versi lama diarsipkan dalam transaksi yang sama.
func (d *Deps) TautBriefFieldCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	body := adminshared.DecodeJSON(r)
	fields, hasFields := decodeFields(body)
	if !hasFields {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Daftar field wajib dikirim."))
		return
	}
	if err := validateFields(fields); err != nil {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, err.Error()))
		return
	}
	changeNote, _ := adminshared.StrField(body, "change_note")
	if len(changeNote) > 255 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Catatan maksimal 255 karakter."))
		return
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	next, err := d.nextVersion(r.Context(), "taut_brief_field_versions")
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	id := uuid.NewString()
	if _, err := d.Pool.Exec(r.Context(),
		`WITH archived AS (
		   UPDATE taut_brief_field_versions SET status = 'archived' WHERE status = 'active'
		 )
		 INSERT INTO taut_brief_field_versions
		   (id, version, status, fields, change_note, created_by, published_by, published_at)
		 VALUES ($1, $2, 'active', $3::jsonb, $4, $5, $5, NOW())`,
		id, next, string(raw), nullIfEmpty(changeNote), userID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.fields_created", id)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true, "id": id, "version": next, "count": len(fields),
	})
}

func (d *Deps) TautBriefFieldUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	body := adminshared.DecodeJSON(r)
	fields, hasFields := decodeFields(body)
	if !hasFields {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Daftar field wajib dikirim."))
		return
	}
	if err := validateFields(fields); err != nil {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, err.Error()))
		return
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	versionID := adminshared.ChiParam(r, "versionId")
	changeNote, hasNote := adminshared.StrField(body, "change_note")
	if hasNote && len(changeNote) > 255 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Catatan maksimal 255 karakter."))
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		`UPDATE taut_brief_field_versions
		 SET fields = $1::jsonb, change_note = COALESCE($2, change_note)
		 WHERE id = $3`,
		string(raw), nullableStr(changeNote, hasNote), versionID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.fields_updated", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "count": len(fields)})
}

func (d *Deps) TautBriefFieldDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	versionID := adminshared.ChiParam(r, "versionId")
	status, found, count, err := d.versionStatus(r.Context(), "taut_brief_field_versions", versionID)
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	if !found {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Versi skema tidak ditemukan."))
		return
	}
	if count <= 1 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusConflict, "Tidak bisa menghapus satu-satunya versi."))
		return
	}
	if status == "active" {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusConflict, "Versi aktif tidak bisa dihapus. Aktifkan versi lain dulu."))
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"DELETE FROM taut_brief_field_versions WHERE id = $1", versionID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.fields_deleted", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *Deps) TautBriefFieldActivate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	versionID := adminshared.ChiParam(r, "versionId")
	if _, found, _, err := d.versionStatus(r.Context(), "taut_brief_field_versions", versionID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	} else if !found {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Versi skema tidak ditemukan."))
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		`WITH archived AS (
		   UPDATE taut_brief_field_versions SET status = 'archived' WHERE status = 'active'
		 )
		 UPDATE taut_brief_field_versions
		 SET status = 'active', published_by = $1, published_at = NOW()
		 WHERE id = $2`, userID, versionID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.fields_published", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- uji prompt ----------

// TautBriefTest menjalankan satu giliran terhadap state kosong, TANPA menyentuh
// Profiler mana pun, supaya prompt dan skema bisa diuji sebelum dipakai klien.
func (d *Deps) TautBriefTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	body := adminshared.DecodeJSON(r)
	scenario, hasScenario := adminshared.StrField(body, "scenario")
	if !hasScenario || scenario == "" {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Skenario tidak boleh kosong."))
		return
	}
	if len(scenario) > 8000 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Skenario maksimal 8000 karakter."))
		return
	}
	cfg, err := d.loadConfig(r.Context())
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	if !cfg.configured() {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusServiceUnavailable, errNotConfigured.Error()))
		return
	}

	prompt := ""
	if p, hasPrompt := adminshared.StrField(body, "prompt"); hasPrompt && p != "" {
		if len(p) > 20000 {
			d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Prompt maksimal 20.000 karakter."))
			return
		}
		prompt = p
	} else if active, err := d.loadActivePrompt(r.Context()); err == nil {
		prompt = active
	}

	fields, hasFields := decodeFields(body)
	if !hasFields || len(fields) == 0 {
		fields, _, _ = d.loadActiveFieldSet(r.Context())
	}
	material, _ := adminshared.StrField(body, "material")

	t := &turn{
		Config:      cfg,
		Prompt:      prompt,
		Fields:      fields,
		States:      map[string]FieldState{},
		Values:      map[string]any{},
		History:     []message{{Role: "user", Content: scenario}},
		ProfileName: "(uji)",
		UserName:    "Admin",
		Sources:     sourceItemsFromMaterial(material),
	}
	t.Phase = DeterminePhase(PhaseInput{
		HasUserMessage: true,
		HasMaterial:    strings.TrimSpace(material) != "",
		RequiredTotal:  totalRequired(fields),
	})

	run, err := d.runEngine(r.Context(), t, nil)
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	accepted, rejected := ValidateUpdates(t.Fields, t.States, run.Output.FieldUpdates)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{
		"phase":                  string(t.Phase),
		"reply":                  run.Output.Reply,
		"asked_field":            run.Output.AskedField,
		"ready_for_confirmation": run.Output.ReadyForConfirmation,
		"suggested_values":       run.Output.SuggestedValues,
		"field_updates":          accepted,
		"rejected_updates":       rejected,
		"usage": map[string]any{
			"model": run.Result.Model, "input": run.Result.InputTokens,
			"output": run.Result.OutputTokens, "durationMs": run.Result.DurationMS,
			"streamed": run.Streamed,
		},
		"promptChars": len(prompt),
		"fieldCount":  len(fields),
	})
}

// ---------- log ----------

func (d *Deps) TautBriefLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	limit := adminshared.ClampInt(q.Get("limit"), 100, 1, 500)
	conds := []string{}
	params := []any{}
	if status == "running" || status == "done" || status == "failed" {
		params = append(params, status)
		conds = append(conds, fmt.Sprintf("r.status = $%d", len(params)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := d.Pool.Query(r.Context(),
		`SELECT r.id, r.brand_id, r.profile_id, r.status, r.model, r.token_input, r.token_output,
		        r.duration_ms, r.error, r.input_preview, r.output_preview, r.created_at, r.finished_at,
		        COALESCE(p.name, '') AS profile_name, COALESCE(b.name, '') AS brand_name
		 FROM taut_brief_runs r
		 LEFT JOIN profiler_profiles p ON p.id = r.profile_id
		 LEFT JOIN brands b ON b.id = r.brand_id
		 `+where+fmt.Sprintf(" ORDER BY r.created_at DESC LIMIT $%d", len(params)+1),
		append(params, limit)...)
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var (
			id, brandID, st                           string
			profileID, model, errMsg, inPrev, outPrev *string
			profileName, brandName                    string
			in, out, dur                              *int
			createdAt                                 time.Time
			finishedAt                                *time.Time
		)
		if rows.Scan(&id, &brandID, &profileID, &st, &model, &in, &out, &dur, &errMsg,
			&inPrev, &outPrev, &createdAt, &finishedAt, &profileName, &brandName) != nil {
			continue
		}
		items = append(items, map[string]any{
			"id": id, "brand_id": brandID, "profile_id": profileID,
			"profile_name": profileName, "brand_name": brandName,
			"status": st, "model": model,
			"token_input": in, "token_output": out, "duration_ms": dur, "error": errMsg,
			"input_preview": inPrev, "output_preview": outPrev,
			"created_at": adminshared.IsoMilli(createdAt), "finished_at": isoMilliPtr(finishedAt),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---------- util admin ----------

func (d *Deps) listVersions(ctx context.Context, table string) []map[string]any {
	rows, err := d.Pool.Query(ctx,
		`SELECT id, version, status, change_note, created_at, published_at FROM `+table+` ORDER BY version DESC`)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var (
			id, status  string
			version     int
			note        *string
			createdAt   time.Time
			publishedAt *time.Time
		)
		if rows.Scan(&id, &version, &status, &note, &createdAt, &publishedAt) != nil {
			continue
		}
		items = append(items, map[string]any{
			"id": id, "version": version, "status": status, "change_note": note,
			"created_at": adminshared.IsoMilli(createdAt), "published_at": isoMilliPtr(publishedAt),
		})
	}
	return items
}

func (d *Deps) versionStatus(ctx context.Context, table, versionID string) (status string, found bool, count int, err error) {
	if err = d.Pool.QueryRow(ctx, "SELECT COUNT(*)::int FROM "+table).Scan(&count); err != nil {
		return "", false, 0, err
	}
	var st string
	if qErr := d.Pool.QueryRow(ctx, "SELECT status FROM "+table+" WHERE id = $1", versionID).Scan(&st); qErr != nil {
		return "", false, count, nil
	}
	return st, true, count, nil
}

func (d *Deps) nextVersion(ctx context.Context, table string) (int, error) {
	var max int
	if err := d.Pool.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM "+table).Scan(&max); err != nil {
		return 0, err
	}
	return max + 1, nil
}

func decodeFields(body map[string]any) ([]Field, bool) {
	raw, ok := body["fields"]
	if !ok || raw == nil {
		return nil, false
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var fields []Field
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, false
	}
	return fields, true
}

// validateFields menolak skema yang pasti rusak di runtime: key kosong/duplikat,
// lapis di luar 1-3, dan seksi tanpa kolom penyimpanan (admin wajib mengisi
// "column" kalau menambah seksi baru).
func validateFields(fields []Field) error {
	if len(fields) == 0 {
		return errors.New("Daftar field tidak boleh kosong.")
	}
	if len(fields) > 300 {
		return errors.New("Maksimal 300 field.")
	}
	seen := map[string]bool{}
	for i, f := range fields {
		key := strings.TrimSpace(f.Key)
		if key == "" {
			return fmt.Errorf("Field ke-%d belum punya key.", i+1)
		}
		if seen[key] {
			return fmt.Errorf("Key %s muncul lebih dari sekali.", key)
		}
		seen[key] = true
		if f.Layer < 1 || f.Layer > 3 {
			return fmt.Errorf("Field %s: lapis harus 1, 2, atau 3.", key)
		}
		if f.Layer != 3 {
			if _, ok := StorageColumn(f); !ok {
				return fmt.Errorf("Field %s: seksi %q belum punya kolom penyimpanan. Isi \"column\" untuk seksi baru.", key, f.Section)
			}
		}
	}
	return nil
}

func totalRequired(fields []Field) int {
	n := 0
	for _, f := range fields {
		if f.Enabled && f.Required && f.Layer != 3 {
			n++
		}
	}
	return n
}

func sourceItemsFromMaterial(material string) []SourceItem {
	if strings.TrimSpace(material) == "" {
		return nil
	}
	return []SourceItem{{Kind: "url", Label: "(bahan uji)", Text: material}}
}

// nullableStr: nilai string opsional untuk COALESCE di SQL.
func nullableStr(v string, has bool) any {
	if !has {
		return nil
	}
	return v
}

func isoMilliPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return adminshared.IsoMilli(*t)
}

func isHTTPURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return false
	}
	rest := trimmed[strings.Index(trimmed, "://")+3:]
	return rest != "" && !strings.HasPrefix(rest, "/")
}
