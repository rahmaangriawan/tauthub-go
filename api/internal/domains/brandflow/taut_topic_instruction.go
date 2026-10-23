package brandflow

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/produce/topic-instruction ----------

type tautAnchorRow struct {
	Anchor string
	URL    string
}

// TautTopicInstruction: racik instruksi topik otomatis dari anchor yang
// diisi mitra/klien di modal Pengelola Naskah. AI bebas memakai model yang
// sudah dikonfigurasi admin (runtime aktif); yang penting AI memahami
// konteks anchor: judul/arah bahasan WAJIB berbeda dari anchor text —
// memakai kata kunci semantik & kaidah SEO/EEAT — dan bila ada tautan,
// instruksi menuntut penyisipan yang natural.
func (d *Deps) TautTopicInstruction(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	mediaType, _ := strField(body, "media_type")
	rawAnchors, hasAnchors := body["anchors"]
	if !hasAnchors {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid request")
		return
	}
	anchors := make([]tautAnchorRow, 0, 2)
	if list, ok := rawAnchors.([]any); ok {
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			anchor, _ := m["anchor"].(string)
			url, _ := m["url"].(string)
			anchor = strings.TrimSpace(anchor)
			if anchor == "" {
				continue
			}
			anchors = append(anchors, tautAnchorRow{Anchor: anchor, URL: strings.TrimSpace(url)})
		}
	}
	if len(anchors) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi minimal 1 anchor teks di kolom Tautan & Anchor.")
		return
	}
	if len(anchors) > 5 {
		anchors = anchors[:5]
	}

	var anchorLines strings.Builder
	for i, a := range anchors {
		if a.URL != "" {
			anchorLines.WriteString(itoa(i+1) + ". anchor: \"" + a.Anchor + "\" -> " + a.URL + "\n")
		} else {
			anchorLines.WriteString(itoa(i+1) + ". anchor: \"" + a.Anchor + "\"\n")
		}
	}

	mediaLabel := "mainstream"
	if strings.EqualFold(mediaType, "blogger") {
		mediaLabel = "blogger"
	}

	// Master prompt & koneksi datang dari agent "Racik Agent" di admin Taut
	// Brief. Tidak ada fallback ke AI runtime lama: bila admin belum mengisi,
	// permintaan ditolak dengan pesan yang menunjuk halaman konfigurasinya.
	masterPrompt := summary.LoadActivePrompt(r.Context(), d.Pool, summary.AgentRacik)
	if strings.TrimSpace(masterPrompt) == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Prompt Racik Agent belum diatur. Buka admin Taut Brief > Racik Agent.")
		return
	}
	secret := ""
	if d.Cfg != nil {
		secret = d.Cfg.CmsEncryptionSecret
	}
	agentCfg := summary.LoadAgentConfig(r.Context(), d.Pool, secret, summary.AgentRacik)
	if !agentCfg.Configured() {
		httpx.Error(w, http.StatusUnprocessableEntity, "Racik Agent belum dikonfigurasi. Isi base URL, API key, dan model di admin Taut Brief > Racik Agent.")
		return
	}

	material := strings.Join([]string{
		"Bahan dari user:",
		anchorLines.String(),
		"Jenis media target: " + mediaLabel + ".",
	}, "\n")

	client := &ai.ChatClient{
		BaseURL:     agentCfg.BaseURL,
		APIKey:      agentCfg.APIKey,
		Model:       agentCfg.Model,
		Temperature: agentCfg.Temperature,
		MaxTokens:   agentCfg.MaxTokens,
		Timeout:     time.Duration(agentCfg.TimeoutMS) * time.Millisecond,
	}
	started := time.Now()
	result, err := client.CompleteMessages(r.Context(), []ai.ChatMessage{
		{Role: "system", Content: masterPrompt},
		{Role: "user", Content: material},
	}, false)
	if err != nil {
		slog.Warn("[taut] racik instruksi gagal", "err", err)
		d.logRacikRun(r.Context(), brandRow.ID, agentCfg.Model, material, "", 0, 0,
			int(time.Since(started).Milliseconds()), err)
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}

	text := strings.TrimSpace(result.Content)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(text[start:end+1]), &parsed) != nil {
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	instruction, _ := parsed["instruction"].(string)
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	if len(instruction) > 600 {
		instruction = instruction[:600]
	}
	d.logRacikRun(r.Context(), brandRow.ID, agentCfg.Model, material, instruction,
		result.InputTokens, result.OutputTokens, int(time.Since(started).Milliseconds()), nil)
	writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "instruction": instruction})
}

// logRacikRun mencatat pemakaian agent racik instruksi ke taut_brief_runs
// (agent 'racik_instruksi') supaya ikut tampil di tab Log admin. Best-effort:
// kegagalan mencatat tidak boleh menggagalkan respons.
func (d *Deps) logRacikRun(ctx context.Context, brandID, model, input, output string, tokIn, tokOut, durationMS int, runErr error) {
	if strings.TrimSpace(brandID) == "" {
		return
	}
	status := "done"
	errText := ""
	if runErr != nil {
		status = "failed"
		errText = runErr.Error()
	}
	_ = d.exec(ctx, `
		INSERT INTO taut_brief_runs
		  (id, brand_id, profile_id, agent, status, model, token_input, token_output,
		   duration_ms, error, input_preview, output_preview, created_at, finished_at)
		VALUES (gen_random_uuid()::text, $1, NULL, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, now(), now())`,
		brandID, summary.AgentRacik, status, model,
		nullInt(tokIn), nullInt(tokOut), durationMS, errText,
		truncateString(input, 2000), truncateString(output, 2000))
}

func nullInt(v int) any {
	if v <= 0 {
		return nil
	}
	return v
}
