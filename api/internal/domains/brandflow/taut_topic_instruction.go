package brandflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
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
	mediaID, _ := strField(body, "media_id")
	angleSlug, _ := strField(body, "angle_slug")
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
		httpx.Error(w, http.StatusUnprocessableEntity, "Prompt Racik Agent belum diatur. Buka admin Taut Pintar > Racik Agent.")
		return
	}
	secret := ""
	if d.Cfg != nil {
		secret = d.Cfg.CmsEncryptionSecret
	}
	agentCfg := summary.LoadAgentConfig(r.Context(), d.Pool, secret, summary.AgentRacik)
	if !agentCfg.Configured() {
		httpx.Error(w, http.StatusUnprocessableEntity, "Racik Agent belum dikonfigurasi. Isi base URL, API key, dan model di admin Taut Pintar > Racik Agent.")
		return
	}

	material := d.buildRacikMaterial(r.Context(), brandRow, chiParam(r, "campaignId"), mediaID, mediaLabel, angleSlug, anchorLines.String())

	client := &ai.ChatClient{
		BaseURL:     agentCfg.BaseURL,
		APIKey:      agentCfg.APIKey,
		Model:       agentCfg.Model,
		Temperature: agentCfg.Temperature,
		MaxTokens:   agentCfg.MaxTokens,
		Timeout:     time.Duration(agentCfg.TimeoutMS) * time.Millisecond,
	}
	startedAt := time.Now()
	// Balasan model berupa {"instruction": "..."}; field stream membaca isinya
	// per potongan supaya teks muncul di textarea selagi model menulis.
	flusher, canFlush := w.(http.Flusher)
	streamStarted := false
	sendEvent := func(event string, payload any) {
		if !canFlush {
			return
		}
		if !streamStarted {
			h := w.Header()
			h.Set("Content-Type", "text/event-stream")
			h.Set("Cache-Control", "no-cache")
			h.Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			streamStarted = true
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonStringify(payload))
		flusher.Flush()
	}
	fail := func(status int, message string) {
		if !streamStarted {
			httpx.Error(w, status, message)
			return
		}
		sendEvent("error", map[string]any{"status": status, "message": message})
	}

	// Textarea topik itu teks polos, jadi penanda **bold** dari model cuma tampil
	// sebagai bintang nyasar. Dibuang di sini, termasuk saat pasangan ** terbelah
	// antar potongan stream (bintang di ujung ditahan dulu).
	field := ai.NewJSONStringFieldStream("instruction")
	pendingStars := ""
	stripEmphasis := func(s string) string {
		pendingStars += s
		keep := len(pendingStars)
		for keep > 0 && pendingStars[keep-1] == '*' {
			keep--
		}
		out := strings.ReplaceAll(pendingStars[:keep], "*", "")
		pendingStars = pendingStars[keep:]
		return out
	}
	result, err := client.CompleteStreamMessages(r.Context(), []ai.ChatMessage{
		{Role: "system", Content: masterPrompt},
		{Role: "user", Content: material},
	}, false, func(chunk string) {
		if delta := stripEmphasis(field.Write(chunk)); delta != "" {
			sendEvent("delta", map[string]any{"append": delta})
		}
	})
	if err != nil {
		slog.Warn("[taut] racik instruksi gagal", "err", err)
		d.logRacikRun(r.Context(), brandRow.ID, agentCfg.Model, material, "", 0, 0,
			int(time.Since(startedAt).Milliseconds()), err)
		fail(http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}

	text := strings.TrimSpace(result.Content)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		fail(http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(text[start:end+1]), &parsed) != nil {
		fail(http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	instruction, _ := parsed["instruction"].(string)
	instruction = strings.ReplaceAll(strings.TrimSpace(instruction), "*", "")
	if instruction == "" {
		fail(http.StatusBadGateway, "AI tidak mengembalikan hasil valid. Coba lagi.")
		return
	}
	d.logRacikRun(r.Context(), brandRow.ID, agentCfg.Model, material, instruction,
		result.InputTokens, result.OutputTokens, int(time.Since(startedAt).Milliseconds()), nil)
	if !canFlush {
		writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "instruction": instruction})
		return
	}
	sendEvent("done", map[string]any{"ok": true, "instruction": instruction})
}

// buildRacikMaterial menyusun bahan yang dikirim ke agent racik. Master prompt
// admin meminta data kampanye (keyword, domain, angle, karakter media), bukan
// hanya anchor — tanpa blok ini model hanya bisa menebak dari anchor.
func (d *Deps) buildRacikMaterial(ctx context.Context, brandRow brandsdomain.BrandRow, campaignID, mediaID, mediaLabel, angleSlug, anchorLines string) string {
	parts := []string{"Bahan dari user:", anchorLines, "Jenis media target: " + mediaLabel + "."}

	brandLine := "Brand: " + brandRow.Name
	if brandRow.Industry != nil && strings.TrimSpace(*brandRow.Industry) != "" {
		brandLine += " (industri: " + strings.TrimSpace(*brandRow.Industry) + ")"
	}
	parts = append(parts, brandLine)

	if campaign, err := d.loadTautCampaignRow(ctx, campaignID, brandRow.ID); err == nil && campaign != nil {
		if name := campaign.strOr("name", ""); name != "" {
			parts = append(parts, "Kampanye: "+name)
		}
		if domain := campaign.strOr("primary_url", ""); domain != "" {
			parts = append(parts, "Domain/website brand: "+domain)
		}
		if kw := campaign.strOr("trigger_keywords", ""); kw != "" {
			parts = append(parts, "Keyword utama: "+kw)
		}
		if brief, bErr := d.loadTautBriefContext(ctx, campaignID); bErr == nil && strings.TrimSpace(brief) != "" {
			parts = append(parts, "Profil bisnis & arahan brand:\n"+truncateString(brief, 4000))
		}
	}

	if mediaID != "" {
		if row, err := d.queryRow(ctx, `
			SELECT name, COALESCE(website_url, '') AS website_url, COALESCE(category, '') AS category
			FROM media WHERE id = $1 LIMIT 1`, mediaID); err == nil && row != nil {
			line := "Media tujuan: " + row.strOr("name", "")
			if site := row.strOr("website_url", ""); site != "" {
				line += " (" + site + ")"
			}
			if cat := row.strOr("category", ""); cat != "" {
				line += " · kategori " + cat
			}
			parts = append(parts, line)
		}
	}

	// Angle yang dipakai adalah yang sedang dipilih di modal. Dropdown itu belum
	// tersimpan ke kampanye saat racik diklik, jadi slug-nya datang dari body —
	// tanpa ini instruksi diracik tanpa angle (hasilnya generik dan tidak nyambung
	// dengan pilihan user). Slug tersimpan hanya jadi cadangan.
	angle := d.loadTautAngleContext(ctx, campaignID, mediaLabel)
	if slug := strings.TrimSpace(angleSlug); slug != "" {
		if picked, err := d.loadTautAngleBySlug(ctx, slug, mediaLabel, false); err == nil && picked != nil {
			angle = picked
		}
	}
	if block, _ := tautAngleBriefBlock(angle); block != "" {
		parts = append(parts, block)
	}
	return strings.Join(parts, "\n")
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
