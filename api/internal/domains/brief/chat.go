package brief

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Endpoint brand-scoped Taut Brief. Percakapan terikat pada satu Profiler
// (profiler_profiles) milik brand, jadi otorisasi memakai BrandAccess seperti
// domain profiler — bukan guard admin.

// accessOrFail: 401/404 standar. Mengembalikan false bila respons sudah ditulis.
func (d *Deps) accessOrFail(w http.ResponseWriter, r *http.Request, minRole string) (userID, brandID, profileID string, ok bool) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return "", "", "", false
	}
	brandID = chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, minRole); err != nil {
		writeErr(w, err)
		return "", "", "", false
	}
	profileID = chiParam(r, "profileId")
	if _, found, err := d.loadProfileMeta(r.Context(), profileID, brandID); err != nil {
		writeErr(w, err)
		return "", "", "", false
	} else if !found {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return "", "", "", false
	}
	return user.ID, brandID, profileID, true
}

func (d *Deps) briefRateLimit(r *http.Request, userID string) error {
	limit := d.Cfg.WriteRateLimitPerMinute
	if cfg, err := d.loadConfig(r.Context()); err == nil && cfg.RateLimitPerMin > 0 {
		limit = cfg.RateLimitPerMin
	}
	if limit <= 0 {
		return nil
	}
	return auth.CheckWriteRate(r.Context(), d.Pool, "write:brief_chat:"+userID, limit)
}

// ---------- GET: hidrasi percakapan ----------

func (d *Deps) BriefChatGet(w http.ResponseWriter, r *http.Request) {
	userID, brandID, profileID, ok := d.accessOrFail(w, r, "viewer")
	if !ok {
		return
	}
	_ = userID

	fields, _, err := d.loadActiveFieldSet(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	states, err := d.loadStates(r.Context(), profileID)
	if err != nil {
		writeErr(w, err)
		return
	}
	values, err := d.loadValues(r.Context(), profileID)
	if err != nil {
		writeErr(w, err)
		return
	}
	messages, err := d.loadMessages(r.Context(), profileID, 60)
	if err != nil {
		writeErr(w, err)
		return
	}
	cfg, err := d.loadConfig(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}

	sources, _ := d.loadSources(r.Context(), profileID)
	filled, total := Progress(fields, states)
	phase, err := d.currentPhase(r.Context(), profileID, fields)
	if err != nil {
		writeErr(w, err)
		return
	}

	msgOut := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		msgOut = append(msgOut, map[string]any{
			"id": m.ID, "role": m.Role, "content": m.Content,
			"created_at": m.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}
	stateOut := make(map[string]any, len(states))
	for k, v := range states {
		stateOut[k] = map[string]any{"status": v.Status, "source": v.Source, "note": v.Note}
	}
	sourceOut := make([]map[string]any, 0, len(sources))
	for _, s := range sources {
		sourceOut = append(sourceOut, map[string]any{"kind": s.Kind, "label": s.Label})
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"profile":      map[string]any{"id": profileID, "name": profileNameOr(r, d, profileID, brandID)},
		"phase":        string(phase),
		"progress":     map[string]any{"filled": filled, "total": total},
		"messages":     msgOut,
		"fields":       fields,
		"field_states": stateOut,
		"values":       values,
		"candidates":   Candidates(fields, states, 3),
		"sources":      sourceOut,
		"configured":   cfg.configured(),
		"capabilities": cfg.Capabilities,
	})
}

func profileNameOr(r *http.Request, d *Deps, profileID, brandID string) string {
	name, _, err := d.loadProfileMeta(r.Context(), profileID, brandID)
	if err != nil {
		return ""
	}
	return name
}

// ---------- POST: kirim pesan ----------

func (d *Deps) BriefChatPost(w http.ResponseWriter, r *http.Request) {
	userID, brandID, profileID, ok := d.accessOrFail(w, r, "writer")
	if !ok {
		return
	}
	if err := d.briefRateLimit(r, userID); err != nil {
		httpx.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}
	body := decodeJSON(r)
	message, hasMessage := strField(body, "message")
	if !hasMessage || message == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pesan tidak boleh kosong.")
		return
	}
	if len(message) > 4000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pesan maksimal 4000 karakter.")
		return
	}

	run, err := d.executeTurn(r.Context(), executeInput{
		BrandID: brandID, ProfileID: profileID, UserID: userID,
		Message: message, UserName: userDisplayName(r),
	})
	if err != nil {
		writeEngineErr(w, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, run.response())
}

// ---------- POST: stream ----------

func (d *Deps) BriefChatStream(w http.ResponseWriter, r *http.Request) {
	userID, brandID, profileID, ok := d.accessOrFail(w, r, "writer")
	if !ok {
		return
	}
	if err := d.briefRateLimit(r, userID); err != nil {
		httpx.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}
	body := decodeJSON(r)
	message, hasMessage := strField(body, "message")
	if !hasMessage || message == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pesan tidak boleh kosong.")
		return
	}
	if len(message) > 4000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pesan maksimal 4000 karakter.")
		return
	}

	flusher, canFlush := w.(http.Flusher)
	started := false
	sendEvent := func(event string, payload any) {
		if !canFlush {
			return
		}
		if !started {
			h := w.Header()
			h.Set("Content-Type", "text/event-stream")
			h.Set("Cache-Control", "no-cache")
			h.Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			started = true
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonStringify(payload))
		flusher.Flush()
	}

	run, err := d.executeTurn(r.Context(), executeInput{
		BrandID: brandID, ProfileID: profileID, UserID: userID,
		Message: message, UserName: userDisplayName(r),
		OnReply:  func(delta string) { sendEvent("delta", map[string]any{"append": delta}) },
		OnStatus: func(status string) { sendEvent("status", map[string]any{"message": status}) },
	})
	if err != nil {
		// Detail lengkap (respons mentah provider LLM, dsb.) hanya untuk log
		// server + taut_brief_runs (Log admin Taut Brief). Klien menerima
		// pesan yang sudah disanitasi.
		slog.Error("brief stream: giliran gagal", "profile_id", profileID, "err", err.Error())
		if !started {
			writeEngineErr(w, err)
			return
		}
		// Sebagian balasan sudah terkirim: kirim error sebagai event, bukan
		// status HTTP, supaya klien bisa menampilkan apa yang sudah ada.
		sendEvent("error", map[string]any{"message": clientBriefErrMsg(err)})
		return
	}
	sendEvent("done", run.response())
}

// ---------- DELETE: mulai percakapan baru ----------

func (d *Deps) BriefChatReset(w http.ResponseWriter, r *http.Request) {
	_, _, profileID, ok := d.accessOrFail(w, r, "writer")
	if !ok {
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"DELETE FROM taut_brief_messages WHERE profile_id = $1", profileID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- POST: set field langsung dari widget ----------

// BriefFieldsPost: picker, chip, toggle, dan upload memanggil ini. Tidak ada
// putaran LLM — pilihan user sudah eksplisit, jadi langsung "confirmed".
func (d *Deps) BriefFieldsPost(w http.ResponseWriter, r *http.Request) {
	userID, brandID, profileID, ok := d.accessOrFail(w, r, "writer")
	if !ok {
		return
	}
	body := decodeJSON(r)
	key, hasKey := strField(body, "key")
	value, hasValue := strField(body, "value")
	if !hasKey || key == "" || !hasValue {
		httpx.Error(w, http.StatusUnprocessableEntity, "Key dan value wajib diisi.")
		return
	}
	status := StatusConfirmed
	if s, hasStatus := strField(body, "status"); hasStatus && s == StatusFilled {
		status = StatusFilled
	}
	source := SourceUser
	if s, hasSource := strField(body, "source"); hasSource && validSources[s] {
		source = s
	}
	note, _ := strField(body, "note")

	fields, _, err := d.loadActiveFieldSet(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	states, err := d.loadStates(r.Context(), profileID)
	if err != nil {
		writeErr(w, err)
		return
	}
	accepted, rejected := ValidateUpdates(fields, states, []FieldUpdate{{
		Key: key, Value: value, Status: status, Source: source, Note: note,
	}})
	if len(accepted) == 0 {
		reason := "Field tidak dikenal atau nilainya tidak valid."
		if len(rejected) > 0 {
			reason = "Tidak bisa menyimpan: " + rejected[0]
		}
		httpx.Error(w, http.StatusUnprocessableEntity, reason)
		return
	}
	if err := d.applyUpdates(r.Context(), profileID, userID, fields, states, accepted); err != nil {
		writeErr(w, err)
		return
	}
	_ = summary.Queue(r.Context(), d.Pool, profileID, brandID, summary.KindEntity)
	states, _ = d.loadStates(r.Context(), profileID)
	filled, total := Progress(fields, states)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"progress": map[string]any{"filled": filled, "total": total},
	})
}

// ---------- util ----------

// clientBriefErrMsg: pesan error yang boleh sampai ke klien. Hanya dua pesan
// yang memang ditujukan untuk klien (belum dikonfigurasi admin, format balasan
// tidak valid); sisanya — respons mentah provider LLM (mis. kredit OpenRouter
// habis), kegagalan internal, dll. — diganti pesan umum agar tidak bocor ke
// bubble chat. Detail lengkapnya tetap tercatat di log server (slog) dan
// taut_brief_runs yang tampil di Log admin Taut Brief.
func clientBriefErrMsg(err error) string {
	switch {
	case errors.Is(err, errNotConfigured), errors.Is(err, errBadOutput):
		return err.Error()
	default:
		return "Maaf, AI Copilot sedang mengalami gangguan. Detail teknisnya sudah tercatat di log admin — coba lagi beberapa saat ya."
	}
}

func writeEngineErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNotConfigured):
		httpx.Error(w, http.StatusServiceUnavailable, clientBriefErrMsg(err))
	case errors.Is(err, errBadOutput):
		httpx.Error(w, http.StatusBadGateway, clientBriefErrMsg(err))
	default:
		httpx.Error(w, http.StatusInternalServerError, clientBriefErrMsg(err))
	}
}

func userDisplayName(r *http.Request) string {
	if user := httpx.AuthUserFromCtx(r); user != nil {
		if strings.TrimSpace(user.DisplayName) != "" {
			return user.DisplayName
		}
		return user.Email
	}
	return ""
}

func jsonStringify(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
