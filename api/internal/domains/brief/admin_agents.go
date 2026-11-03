package brief

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
)

// Endpoint admin untuk dua agent ringkasan Brief. Bentuknya mengikuti config &
// prompt Taut Brief yang sudah ada, tetapi terpisah per agent supaya model dan
// prompt bisa berbeda (mis. model murah khusus ringkasan).

func (d *Deps) agentFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	agent := adminshared.ChiParam(r, "agent")
	if !summary.ValidAgent(agent) {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Agent tidak ditemukan."))
		return "", false
	}
	return agent, true
}

// ---------- config ----------

func (d *Deps) TautBriefAgentConfigGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
	if !ok {
		return
	}

	var (
		baseURL, sealed, model *string
		temperature            float64 = 0.2
		maxTokens              *int
		timeoutMS              int  = 120000
		enabled                bool = true
	)
	_ = d.Pool.QueryRow(r.Context(), `
		SELECT base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms, enabled
		FROM taut_brief_agent_configs WHERE agent = $1`, agent).
		Scan(&baseURL, &sealed, &model, &temperature, &maxTokens, &timeoutMS, &enabled)

	effective := summary.LoadAgentConfig(r.Context(), d.Pool, d.Cfg.CmsEncryptionSecret, agent)

	w.Header().Set("Cache-Control", "no-store")
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{
		"agent":              agent,
		"base_url":           derefStr(baseURL),
		"model":              derefStr(model),
		"temperature":        temperature,
		"max_tokens":         maxTokens,
		"timeout_ms":         timeoutMS,
		"enabled":            enabled,
		"api_key_configured": sealed != nil && *sealed != "",
		"effective": map[string]any{
			"base_url":   effective.BaseURL,
			"model":      effective.Model,
			"configured": effective.Configured(),
			"inherited":  effective.Inherited,
		},
	})
}

func (d *Deps) TautBriefAgentConfigUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
	if !ok {
		return
	}
	body := adminshared.DecodeJSON(r)
	baseURL, hasBaseURL := adminshared.StrField(body, "base_url")
	apiKey, hasAPIKey := adminshared.StrField(body, "api_key")
	model, hasModel := adminshared.StrField(body, "model")
	temperature, hasTemperature := adminshared.NumField(body, "temperature")
	maxTokens, hasMaxTokens := adminshared.IntField(body, "max_tokens")
	timeoutMS, hasTimeout := adminshared.IntField(body, "timeout_ms")
	enabled, hasEnabled := adminshared.BoolField(body, "enabled")

	if hasBaseURL && len(baseURL) > 500 {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "String harus 500 karakter atau kurang"))
		return
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
	if hasTimeout && (timeoutMS < 5000 || timeoutMS > 300000) {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity, "Timeout minimal 5000 ms dan maksimal 300000 ms."))
		return
	}

	// Baris sudah di-seed migrasi; ini jaring aman bila dihapus manual.
	if _, err := d.Pool.Exec(r.Context(),
		`INSERT INTO taut_brief_agent_configs (agent) VALUES ($1) ON CONFLICT (agent) DO NOTHING`, agent); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}

	sets := []string{"updated_at = NOW()"}
	args := []any{}
	add := func(col string, val any) {
		args = append(args, val)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if hasBaseURL {
		add("base_url", nullIfEmpty(baseURL))
	}
	if hasAPIKey && apiKey != "" {
		sealed, err := crypto.SealV3(apiKey, d.Cfg.CmsEncryptionSecret, "taut_brief_agent_"+agent)
		if err != nil {
			d.Admin.WriteErr(w, err)
			return
		}
		add("sealed_api_key", sealed)
	}
	if hasModel {
		add("model", nullIfEmpty(model))
	}
	if hasTemperature {
		add("temperature", temperature)
	}
	if hasMaxTokens {
		add("max_tokens", maxTokens)
	}
	if hasTimeout {
		add("timeout_ms", timeoutMS)
	}
	if hasEnabled {
		add("enabled", enabled)
	}
	add("updated_by", userID)
	args = append(args, agent)
	query := fmt.Sprintf("UPDATE taut_brief_agent_configs SET %s WHERE agent = $%d",
		strings.Join(sets, ", "), len(args))
	if _, err := d.Pool.Exec(r.Context(), query, args...); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.agent_config_updated", agent)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *Deps) TautBriefAgentConfigTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
	if !ok {
		return
	}
	cfg := summary.LoadAgentConfig(r.Context(), d.Pool, d.Cfg.CmsEncryptionSecret, agent)
	// Uji koneksi tetap boleh walau agent dinonaktifkan — admin sedang mengatur.
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusUnprocessableEntity,
			"Agent belum dikonfigurasi (base URL, API key, atau model kosong)."))
		return
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	// Balasan uji hanya "OK"; batas kecil supaya OpenRouter tidak menahan
	// kredit sebesar maksimum model (65.535 token) dan menolak 402 saat saldo tipis.
	testMax := 64
	client := &ai.ChatClient{
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Temperature: 0, Timeout: timeout, MaxTokens: &testMax,
	}
	res, err := client.CompleteMessages(r.Context(), []ai.ChatMessage{
		{Role: "system", Content: "Balas hanya dengan kata OK."},
		{Role: "user", Content: "ping"},
	}, false)
	if err != nil {
		d.Admin.WriteErr(w, adminshared.Err(http.StatusBadGateway, "Koneksi gagal: "+err.Error()))
		return
	}
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Koneksi berhasil.",
		"model":   cfg.Model,
		"reply":   strings.TrimSpace(res.Content),
	})
}

// ---------- prompt berversi per agent ----------

func (d *Deps) TautBriefAgentPromptsList(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
	if !ok {
		return
	}
	rows, err := d.Pool.Query(r.Context(), `
		SELECT id, version, status, change_note, created_at, published_at
		FROM taut_brief_agent_prompt_versions WHERE agent = $1
		ORDER BY version DESC`, agent)
	if err != nil {
		d.Admin.WriteErr(w, err)
		return
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
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (d *Deps) TautBriefAgentPromptGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := d.adminGate(w, r); !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
	if !ok {
		return
	}
	var (
		id, status   string
		version      int
		prompt, note *string
		createdAt    time.Time
	)
	err := d.Pool.QueryRow(r.Context(), `
		SELECT id, version, status, prompt, change_note, created_at
		FROM taut_brief_agent_prompt_versions WHERE id = $1 AND agent = $2`,
		adminshared.ChiParam(r, "versionId"), agent).
		Scan(&id, &version, &status, &prompt, &note, &createdAt)
	if err != nil {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Versi prompt tidak ditemukan."))
		return
	}
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"version": map[string]any{
		"id": id, "version": version, "status": status, "prompt": prompt,
		"change_note": note, "created_at": adminshared.IsoMilli(createdAt),
	}})
}

func (d *Deps) TautBriefAgentPromptCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
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
		prompt = summary.LoadActivePrompt(r.Context(), d.Pool, agent)
	}
	var next int
	if err := d.Pool.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(version), 0) + 1 FROM taut_brief_agent_prompt_versions WHERE agent = $1`, agent).
		Scan(&next); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	id := uuid.NewString()
	if _, err := d.Pool.Exec(r.Context(), `
		WITH archived AS (
		  UPDATE taut_brief_agent_prompt_versions SET status = 'archived'
		  WHERE agent = $6 AND status = 'active'
		)
		INSERT INTO taut_brief_agent_prompt_versions
		  (id, agent, version, status, prompt, change_note, created_by, published_by, published_at)
		VALUES ($1, $2, $3, 'active', $4, $5, $7, $7, NOW())`,
		id, agent, next, nullIfEmpty(prompt), nullIfEmpty(changeNote), agent, userID); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.agent_prompt_created", id)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "version": next})
}

func (d *Deps) TautBriefAgentPromptUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
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
	if _, err := d.Pool.Exec(r.Context(), `
		UPDATE taut_brief_agent_prompt_versions
		SET prompt = COALESCE($1, prompt), change_note = COALESCE($2, change_note)
		WHERE id = $3 AND agent = $4`,
		nullableStr(prompt, hasPrompt), nullableStr(changeNote, hasNote), versionID, agent); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.agent_prompt_updated", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *Deps) TautBriefAgentPromptDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
	if !ok {
		return
	}
	versionID := adminshared.ChiParam(r, "versionId")
	var count int
	if err := d.Pool.QueryRow(r.Context(),
		`SELECT COUNT(*)::int FROM taut_brief_agent_prompt_versions WHERE agent = $1`, agent).Scan(&count); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	var status string
	if err := d.Pool.QueryRow(r.Context(),
		`SELECT status FROM taut_brief_agent_prompt_versions WHERE id = $1 AND agent = $2`,
		versionID, agent).Scan(&status); err != nil {
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
		`DELETE FROM taut_brief_agent_prompt_versions WHERE id = $1 AND agent = $2`, versionID, agent); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.agent_prompt_deleted", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *Deps) TautBriefAgentPromptActivate(w http.ResponseWriter, r *http.Request) {
	userID, ok := d.adminGate(w, r)
	if !ok {
		return
	}
	agent, ok := d.agentFromPath(w, r)
	if !ok {
		return
	}
	versionID := adminshared.ChiParam(r, "versionId")
	var exists bool
	if err := d.Pool.QueryRow(r.Context(),
		`SELECT true FROM taut_brief_agent_prompt_versions WHERE id = $1 AND agent = $2`,
		versionID, agent).Scan(&exists); err != nil || !exists {
		d.Admin.WriteErr(w, adminshared.ErrNotFound("Versi prompt tidak ditemukan."))
		return
	}
	if _, err := d.Pool.Exec(r.Context(), `
		WITH archived AS (
		  UPDATE taut_brief_agent_prompt_versions SET status = 'archived'
		  WHERE agent = $3 AND status = 'active'
		)
		UPDATE taut_brief_agent_prompt_versions
		SET status = 'active', published_by = $1, published_at = NOW()
		WHERE id = $2 AND agent = $3`, userID, versionID, agent); err != nil {
		d.Admin.WriteErr(w, err)
		return
	}
	d.briefAudit(r.Context(), userID, "taut_brief.agent_prompt_published", versionID)
	adminshared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
