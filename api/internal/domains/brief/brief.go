package brief

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Taut Brief — chatbot penyusun Profiler brand.
//
// Terpisah dari agent Taut Pintar (taut_agent_configs / taut_agent_runs):
// konfigurasi, prompt, log, dan rate limit dikelola sendiri di tabel
// taut_brief_*. Nilai field Lapis 1 disimpan di profiler_profiles.entity_core,
// Lapis 2 memakai kolom identity/style/seo yang sudah ada.

type Deps struct {
	Pool  *pgxpool.Pool
	Cfg   *config.Config
	Admin *adminshared.Deps
}

const configEntityID = "taut_brief_config"

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func (d *Deps) access(r *http.Request, brandID, userID, minRole string) (brandsdomain.BrandRow, error) {
	return brandsdomain.BrandAccess(r.Context(), d.Pool, brandID, userID, minRole)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, err error) {
	var be *brandsdomain.BrandError
	if errors.As(err, &be) {
		httpx.Error(w, be.Status, be.Message)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, err.Error())
}

func decodeJSON(r *http.Request) map[string]any {
	var body map[string]any
	_ = json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&body)
	if body == nil {
		body = map[string]any{}
	}
	return body
}

func strField(body map[string]any, key string) (string, bool) {
	v, ok := body[key]
	if !ok || v == nil {
		return "", false
	}
	s, isStr := v.(string)
	if !isStr {
		return "", false
	}
	return strings.TrimSpace(s), true
}

// ---------- konfigurasi agent ----------

type briefConfig struct {
	BaseURL         string
	APIKey          string // sudah di-unseal
	Model           string
	Temperature     float64
	MaxTokens       *int
	TimeoutMS       int
	RateLimitPerMin int
	Enabled         bool
	Capabilities    map[string]any
}

func (c *briefConfig) configured() bool {
	return c != nil && c.Enabled &&
		strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.APIKey) != "" &&
		strings.TrimSpace(c.Model) != ""
}

func (d *Deps) loadConfig(ctx context.Context) (*briefConfig, error) {
	cfg := &briefConfig{Temperature: 0.3, TimeoutMS: 60000, RateLimitPerMin: 12}
	var (
		baseURL, sealed, model *string
		temperature            *float64
		maxTokens              *int
		timeoutMS, rateLimit   *int
		enabled                *bool
		settings               []byte
	)
	err := d.Pool.QueryRow(ctx,
		`SELECT base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms,
		        rate_limit_per_min, enabled, settings
		 FROM taut_brief_config WHERE id = 'default'`).
		Scan(&baseURL, &sealed, &model, &temperature, &maxTokens, &timeoutMS, &rateLimit, &enabled, &settings)
	if err != nil {
		return cfg, nil // tabel belum ada / belum di-seed → dianggap belum dikonfigurasi
	}
	cfg.BaseURL = strings.TrimRight(derefStr(baseURL), "/")
	cfg.Model = derefStr(model)
	if temperature != nil {
		cfg.Temperature = *temperature
	}
	cfg.MaxTokens = maxTokens
	if timeoutMS != nil && *timeoutMS > 0 {
		cfg.TimeoutMS = *timeoutMS
	}
	if rateLimit != nil {
		cfg.RateLimitPerMin = *rateLimit
	}
	if enabled != nil {
		cfg.Enabled = *enabled
	}
	if len(settings) > 0 {
		_ = json.Unmarshal(settings, &cfg.Capabilities)
	}
	if sealed != nil && *sealed != "" {
		if plain, uErr := crypto.UnsealV3(*sealed, d.Cfg.CmsEncryptionSecret, configEntityID); uErr == nil {
			cfg.APIKey = plain
		}
	}
	return cfg, nil
}

// ---------- prompt & skema aktif ----------

func (d *Deps) loadActivePrompt(ctx context.Context) (string, error) {
	var prompt *string
	err := d.Pool.QueryRow(ctx,
		`SELECT prompt FROM taut_brief_prompt_versions WHERE status = 'active' ORDER BY version DESC LIMIT 1`).
		Scan(&prompt)
	if err != nil {
		return "", nil
	}
	return derefStr(prompt), nil
}

func (d *Deps) loadActiveFieldSet(ctx context.Context) ([]Field, int, error) {
	var (
		version int
		raw     []byte
	)
	err := d.Pool.QueryRow(ctx,
		`SELECT version, fields FROM taut_brief_field_versions WHERE status = 'active' ORDER BY version DESC LIMIT 1`).
		Scan(&version, &raw)
	if err != nil {
		return nil, 0, nil
	}
	var fields []Field
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, version, err
	}
	return fields, version, nil
}

// ---------- state brief ----------

func (d *Deps) loadProfileMeta(ctx context.Context, profileID, brandID string) (string, bool, error) {
	var name string
	err := d.Pool.QueryRow(ctx,
		`SELECT name FROM profiler_profiles
		 WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL`, profileID, brandID).Scan(&name)
	if err != nil {
		return "", false, nil
	}
	return name, true, nil
}

func (d *Deps) loadStates(ctx context.Context, profileID string) (map[string]FieldState, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT field_key, status, source, note FROM taut_brief_field_state WHERE profile_id = $1`, profileID)
	if err != nil {
		return map[string]FieldState{}, nil
	}
	defer rows.Close()
	out := map[string]FieldState{}
	for rows.Next() {
		var key, status, source string
		var note *string
		if err := rows.Scan(&key, &status, &source, &note); err != nil {
			continue
		}
		out[key] = FieldState{Status: status, Source: source, Note: derefStr(note)}
	}
	return out, nil
}

// loadValues menggabungkan entity_core + identity + style + seo menjadi satu
// peta key → nilai. Kunci antar kolom tidak bertabrakan (dijaga skema).
func (d *Deps) loadValues(ctx context.Context, profileID string) (map[string]any, error) {
	var entityCore, identity, style, seo []byte
	err := d.Pool.QueryRow(ctx,
		`SELECT entity_core, identity, style, seo FROM profiler_profiles
		 WHERE id = $1 AND deleted_at IS NULL`, profileID).
		Scan(&entityCore, &identity, &style, &seo)
	if err != nil {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	for _, raw := range [][]byte{entityCore, identity, style, seo} {
		mergeJSONObject(out, raw)
	}
	return out, nil
}

func mergeJSONObject(dst map[string]any, raw []byte) {
	if len(raw) == 0 {
		return
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return
	}
	for k, v := range obj {
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			continue
		}
		dst[k] = v
	}
}

// applyUpdates menulis nilai yang lolos validasi ke profiler_profiles, lalu
// menyimpan status + sumbernya. Penggabungan per kolom memakai operator `||`
// supaya tidak menimpa field lain yang mungkin sedang diubah paralel.
func (d *Deps) applyUpdates(ctx context.Context, profileID, userID string, fields []Field, states map[string]FieldState, accepted []FieldUpdate) error {
	if err := d.writeUpdates(ctx, d.Pool, profileID, userID, fields, accepted); err != nil {
		return err
	}
	// Nilai field berubah -> indeks memori lintas brief ikut diperbarui
	// (async, best-effort — tidak boleh menggagalkan giliran).
	d.QueueMemoryReindex(ctx, profileID)
	return nil
}

// applyUpdatesAtomic: versi transaksi untuk jalur yang menyimpan beberapa field
// sekaligus (form Profil Brand). Semua nilai dan status ditulis dalam satu
// transaksi, jadi kegagalan di tengah tidak meninggalkan simpanan separuh.
func (d *Deps) applyUpdatesAtomic(ctx context.Context, profileID, userID string, fields []Field, accepted []FieldUpdate) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := d.writeUpdates(ctx, tx, profileID, userID, fields, accepted); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	d.QueueMemoryReindex(ctx, profileID)
	return nil
}

// dbExec: irisan metode yang dipakai pgxpool.Pool dan pgx.Tx, supaya penulisan
// field bisa jalan di luar maupun di dalam transaksi dengan kode yang sama.
type dbExec interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func (d *Deps) writeUpdates(ctx context.Context, ex dbExec, profileID, userID string, fields []Field, accepted []FieldUpdate) error {
	if len(accepted) == 0 {
		return nil
	}
	grouped := map[string]map[string]any{}
	for _, u := range accepted {
		f, ok := fieldByKey(fields, u.Key)
		if !ok {
			continue
		}
		col, ok := StorageColumn(f)
		if !ok {
			continue
		}
		if grouped[col] == nil {
			grouped[col] = map[string]any{}
		}
		grouped[col][u.Key] = u.Value
	}
	for col, values := range grouped {
		raw, err := json.Marshal(values)
		if err != nil {
			continue
		}
		if _, err := ex.Exec(ctx,
			`UPDATE profiler_profiles SET `+col+` = `+col+` || $2::jsonb, updated_at = NOW(), updated_by = $3
			 WHERE id = $1 AND deleted_at IS NULL`,
			profileID, string(raw), nullIfEmpty(userID)); err != nil {
			return err
		}
	}
	for _, u := range accepted {
		if err := upsertFieldStateOn(ctx, ex, profileID, u.Key, u.Status, u.Source, u.Note); err != nil {
			return err
		}
	}
	return nil
}

func upsertFieldStateOn(ctx context.Context, ex dbExec, profileID, key, status, source, note string) error {
	_, err := ex.Exec(ctx,
		`INSERT INTO taut_brief_field_state (profile_id, field_key, status, source, note, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (profile_id, field_key) DO UPDATE SET
		   status = EXCLUDED.status, source = EXCLUDED.source,
		   note = EXCLUDED.note, updated_at = NOW()`,
		profileID, key, status, source, nullIfEmpty(note))
	return err
}

func (d *Deps) upsertFieldState(ctx context.Context, profileID, key, status, source, note string) error {
	return upsertFieldStateOn(ctx, d.Pool, profileID, key, status, source, note)
}

// ---------- pesan ----------

type message struct {
	ID        string
	Role      string
	Content   string
	Meta      map[string]any
	CreatedAt time.Time
}

// loadChatSummary: ringkasan bergulir percakapan untuk profile ini.
func (d *Deps) loadChatSummary(ctx context.Context, profileID string) string {
	var s string
	_ = d.Pool.QueryRow(ctx,
		`SELECT chat_summary FROM profiler_profiles WHERE id = $1`, profileID).Scan(&s)
	return s
}

// loadRecentTimeline mengambil caption post Timeline terbaru untuk brief ini.
//
// Dipakai sebagai konteks anti-duplikat saat model mempertimbangkan usulan aksi
// "timeline_post": tanpa daftar ini, model cenderung mengusulkan ulang kabar
// yang sudah tayang. Error apa pun menghasilkan daftar kosong, bukan kegagalan
// giliran — aksi hanya kehilangan pembanding.
func (d *Deps) loadRecentTimeline(ctx context.Context, profileID string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	rows, err := d.Pool.Query(ctx, `
		SELECT body FROM profiler_timeline_posts
		WHERE profile_id = $1 AND deleted_at IS NULL
		ORDER BY COALESCE(occurred_on, created_at::date) DESC, created_at DESC
		LIMIT $2`, profileID, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]string, 0, limit)
	for rows.Next() {
		var body string
		if rows.Scan(&body) != nil {
			continue
		}
		if body = strings.TrimSpace(body); body != "" {
			out = append(out, body)
		}
	}
	return out
}

// appendChatSummary: tambah satu baris ringkasan untuk giliran yang baru
// selesai, lalu pangkas dari depan supaya total tetap ringkas (maks 24 baris
// / 2400 karakter). Deterministik dari data giliran — tanpa panggilan LLM
// tambahan.
func (d *Deps) appendChatSummary(ctx context.Context, profileID, userMsg string, accepted []FieldUpdate, out *EngineOutput) {
	const maxLines = 24
	const maxChars = 2400

	line := "User: \"" + truncateRunes(strings.TrimSpace(userMsg), 140) + "\""
	var parts []string
	for _, u := range accepted {
		parts = append(parts, u.Key+"="+truncateRunes(fmt.Sprint(u.Value), 60))
	}
	if len(parts) > 0 {
		line += " -> diisi: " + strings.Join(parts, ", ")
	}
	if out != nil && out.AskedField != nil && strings.TrimSpace(*out.AskedField) != "" {
		line += "; ditanya: " + *out.AskedField
	}
	line += "\n"

	var prev string
	_ = d.Pool.QueryRow(ctx,
		`SELECT chat_summary FROM profiler_profiles WHERE id = $1`, profileID).Scan(&prev)
	lines := append(strings.Split(prev, "\n"), line)
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	s := strings.Join(lines, "\n")
	if len(s) > maxChars {
		s = s[len(s)-maxChars:]
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
	}
	_, _ = d.Pool.Exec(ctx,
		`UPDATE profiler_profiles SET chat_summary = $1, updated_at = NOW() WHERE id = $2`, s, profileID)
}

func (d *Deps) loadMessages(ctx context.Context, profileID string, limit int) ([]message, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.Pool.Query(ctx,
		`SELECT id, role, content, meta, created_at FROM (
		   SELECT id, role, content, meta, created_at FROM taut_brief_messages
		   WHERE profile_id = $1 ORDER BY created_at DESC LIMIT $2
		 ) t ORDER BY created_at ASC`, profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []message{}
	for rows.Next() {
		var m message
		var raw []byte
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &raw, &m.CreatedAt); err != nil {
			continue
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &m.Meta)
		}
		out = append(out, m)
	}
	return out, nil
}

// insertMessage menyimpan satu pesan dan mengembalikan id-nya.
//
// Id perlu dikembalikan karena pesan assistant yang membawa aksi harus bisa
// dirujuk frontend saat tombolnya diklik (POST /brief/actions/consume butuh
// message_id). Sebelumnya id dibuat di dalam fungsi ini tanpa dikembalikan,
// sehingga respons giliran tidak punya id dan tombol aksi mati senyap.
func (d *Deps) insertMessage(ctx context.Context, profileID, brandID, role, content, userID string, meta map[string]any) (string, error) {
	var metaArg any
	if len(meta) > 0 {
		if raw, err := json.Marshal(meta); err == nil {
			metaArg = string(raw)
		}
	}
	id := uuid.NewString()
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO taut_brief_messages (id, profile_id, brand_id, role, content, meta, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)`,
		id, profileID, brandID, role, content, metaArg, nullIfEmpty(userID))
	if err != nil {
		return "", err
	}
	return id, nil
}

// lastAssistantMeta: apakah pesan assistant terakhir menandai brief siap
// dikonfirmasi, kapan pesan itu dibuat, dan field apa yang ia tanyakan.
func (d *Deps) lastAssistantMeta(ctx context.Context, profileID string) (bool, int64, string) {
	var raw []byte
	var createdAt time.Time
	err := d.Pool.QueryRow(ctx,
		`SELECT meta, created_at FROM taut_brief_messages
		 WHERE profile_id = $1 AND role = 'assistant' ORDER BY created_at DESC LIMIT 1`,
		profileID).Scan(&raw, &createdAt)
	if err != nil {
		return false, 0, ""
	}
	var meta map[string]any
	if json.Unmarshal(raw, &meta) != nil {
		return false, createdAt.UnixMilli(), ""
	}
	ready, _ := meta["ready_for_confirmation"].(bool)
	asked, _ := meta["asked_field"].(string)
	return ready, createdAt.UnixMilli(), asked
}

func (d *Deps) lastUserAt(ctx context.Context, profileID string) (int64, bool) {
	var createdAt time.Time
	err := d.Pool.QueryRow(ctx,
		`SELECT created_at FROM taut_brief_messages
		 WHERE profile_id = $1 AND role = 'user' ORDER BY created_at DESC LIMIT 1`,
		profileID).Scan(&createdAt)
	if err != nil {
		return 0, false
	}
	return createdAt.UnixMilli(), true
}

// ---------- material (dokumen & URL) ----------

func (d *Deps) loadSources(ctx context.Context, profileID string) ([]SourceItem, int64) {
	items := []SourceItem{}
	var latest int64

	rows, err := d.Pool.Query(ctx,
		`SELECT original_filename, extracted_text, created_at FROM profiler_documents
		 WHERE profile_id = $1 AND deleted_at IS NULL AND parse_status = 'ready'
		 ORDER BY created_at DESC LIMIT 20`, profileID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var text *string
			var createdAt time.Time
			if rows.Scan(&name, &text, &createdAt) != nil {
				continue
			}
			if createdAt.UnixMilli() > latest {
				latest = createdAt.UnixMilli()
			}
			items = append(items, SourceItem{Kind: "dokumen", Label: name, Text: derefStr(text)})
		}
	}

	urlRows, err := d.Pool.Query(ctx,
		`SELECT url, title, extracted_text, COALESCE(last_synced_at, created_at) AS at FROM profiler_urls
		 WHERE profile_id = $1 AND deleted_at IS NULL AND fetch_status = 'ready'
		 ORDER BY COALESCE(last_synced_at, created_at) DESC LIMIT 20`, profileID)
	if err == nil {
		defer urlRows.Close()
		for urlRows.Next() {
			var url string
			var title, text *string
			var at time.Time
			if urlRows.Scan(&url, &title, &text, &at) != nil {
				continue
			}
			if at.UnixMilli() > latest {
				latest = at.UnixMilli()
			}
			label := url
			if t := strings.TrimSpace(derefStr(title)); t != "" {
				label = t + " — " + url
			}
			items = append(items, SourceItem{Kind: "url", Label: label, Text: derefStr(text)})
		}
	}

	// URL yang SUDAH DICOBA tetapi gagal dibaca tetap diperlihatkan ke model
	// dengan status gagal — supaya ia bisa menjelaskan keterbatasannya secara
	// jujur dan menawarkan alternatif, bukan menganggap user belum memberi
	// apa-apa. Gagal membaca TIDAK dianggap material (tidak menggeser `latest`,
	// jadi fase tidak melompat ke confirm).
	failRows, err := d.Pool.Query(ctx,
		`SELECT url, COALESCE(NULLIF(fetch_error, ''), 'tidak diketahui penyebabnya') FROM profiler_urls
		 WHERE profile_id = $1 AND deleted_at IS NULL AND fetch_status IN ('failed', 'fetching')
		 ORDER BY created_at DESC LIMIT 5`, profileID)
	if err == nil {
		for failRows.Next() {
			var url, reason string
			if failRows.Scan(&url, &reason) != nil {
				continue
			}
			items = append(items, SourceItem{
				Kind:  "url",
				Label: url,
				Text:  "Sistem SUDAH mencoba membaca halaman ini tetapi GAGAL: " + reason + ". Isi halaman TIDAK tersedia — jangan mengarang isinya, jelaskan ke user dan tawarkan alternatif (jawab singkat atau tempel teksnya).",
			})
		}
		failRows.Close()
	}

	// Skema lama: material dianggap ada hanya bila isinya benar-benar terbaca.
	filtered := make([]SourceItem, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Text) == "" {
			continue
		}
		filtered = append(filtered, it)
	}
	if len(filtered) == 0 {
		return filtered, 0
	}
	return filtered, latest
}

// ---------- log run ----------

func (d *Deps) startRun(ctx context.Context, brandID, profileID, model, input string) string {
	id := uuid.NewString()
	// input_preview = pesan user (bukan prompt lengkap) — itu memang isi
	// percakapan yang diketik sendiri oleh klien, bukan kutipan dokumen privat.
	_, _ = d.Pool.Exec(ctx,
		`INSERT INTO taut_brief_runs (id, brand_id, profile_id, status, model, input_preview)
		 VALUES ($1, $2, $3, 'running', $4, $5)`,
		id, brandID, nullIfEmpty(profileID), nullIfEmpty(model), truncate(input, 4000))
	return id
}

func (d *Deps) finishRun(ctx context.Context, runID string, status string, in, out int, durationMS int64, output, errMsg string) {
	_, _ = d.Pool.Exec(ctx,
		`UPDATE taut_brief_runs SET status = $2, token_input = $3, token_output = $4,
		   duration_ms = $5, output_preview = $6, error = $7, finished_at = NOW()
		 WHERE id = $1`,
		runID, status, nullIfZero(in), nullIfZero(out), nullIfZero64(durationMS),
		nullIfEmpty(truncate(output, 4000)), nullIfEmpty(errMsg))
}

// ---------- util ----------

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullIfZero64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
