package brandflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
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
			// Tanpa URL tidak ada tautan yang bisa dipasang. Instruksi yang
			// menyuruh "sisipkan tautan" hanya membuat Writer menebak/meng- bold
			// frasa tanpa <a href> — jadi tandai eksplisit di bahan.
			anchorLines.WriteString(itoa(i+1) + ". anchor: \"" + a.Anchor + "\" (TANPA URL — jangan menyuruh memasang tautan/hyperlink; cukup sebut frasa ini sebagai teks biasa)\n")
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

	material := d.buildRacikMaterial(r.Context(), chiParam(r, "campaignId"), mediaID, mediaLabel, angleSlug, anchorLines.String(), anchors)
	// Aturan keluaran ditempel di system prompt supaya berlaku juga bila prompt
	// admin di database longgar: hasilnya paragraf pendek yang hanya memakai
	// bahan, bukan artikel mini atau fakta karangan.
	systemPrompt := strings.TrimSpace(masterPrompt) + "\n\n" + racikOutputRules

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
		{Role: "system", Content: systemPrompt},
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

// racikOutputRules: batas keluaran agent racik. Ditulis sebagai aturan keras
// karena instruksi ini dipakai apa adanya sebagai topik Writer — kalau model
// melebar (menambah fakta, angka, atau struktur artikel), hasil generate ikut
// melenceng dan harus diperbaiki manual.
const racikOutputRules = `Aturan keluaran (WAJIB):
- Tulis SATU paragraf instruksi 3-6 kalimat, bahasa Indonesia, tanpa judul dan tanpa daftar bernomor.
- Pakai HANYA informasi pada "Bahan dari user" di bawah. JANGAN menambah fakta, angka, nama orang, tanggal, tempat, atau klaim yang tidak ada di bahan.
- Jangan memakai teks anchor sebagai judul artikel; susun arah bahasan yang relevan dengan angle yang dipilih.
- Tautan hanya boleh diperintahkan bila anchor MEMILIKI URL (format "anchor -> URL"). Anchor yang ditandai "TANPA URL" hanya disebut sebagai frasa/nama biasa — JANGAN menulis "sisipkan tautan", "backlink", atau "hyperlink" untuk anchor itu.
- Untuk SETIAP anchor yang MEMILIKI URL, instruksimu WAJIB memerintahkan agar anchor itu dipakai PERSIS sebagai teks tautan dan ditempatkan di dalam paragraf yang topiknya memang berkaitan. Sertakan izin eksplisit untuk menulis kalimat baru yang wajar bila belum ada kalimat yang cocok — JANGAN memaksa anchor masuk ke kalimat yang maknanya tidak nyambung, dan JANGAN menaruhnya sebagai kalimat berdiri sendiri, daftar tautan, atau frasa pembatas seperti "Baca juga".
- WAJIB menyebut nama subjek/brand (dari anchor atau Konteks domain) secara eksplisit di instruksi, supaya judul artikel yang ditulis Writer jelas konteksnya (bukan frasa industri generik seperti "transformasi model bisnis").
- "Konteks domain (hasil crawl)" hanya untuk mengenali jenis usaha/subjek. JANGAN menyalin angka, tanggal, atau klaim dari sana.
- Sebut target panjang artikel dan jumlah tautan sesuai aturan media di bahan.
- Balas hanya JSON: {"instruction": "..."} tanpa teks lain, tanpa markdown.`

// buildRacikMaterial menyusun bahan yang dikirim ke agent racik. Sengaja
// ramping: anchor + angle + aturan teknis media (panjang & kapasitas tautan).
// Profil bisnis/kampanye tidak lagi ikut — model sempat memakainya untuk
// menambah klaim yang tidak diminta, sehingga instruksi harus diperbaiki manual
// sebelum generate.
func (d *Deps) buildRacikMaterial(ctx context.Context, campaignID, mediaID, mediaLabel, angleSlug, anchorLines string, anchors []tautAnchorRow) string {
	parts := []string{"Bahan dari user:", anchorLines, "Jenis media target: " + mediaLabel + "."}

	// Konteks domain hasil crawl halaman URL anchor: teks anchor saja ("Giovani
	// Barbershop") tidak memberi tahu jenis usaha/subjeknya, sehingga instruksi
	// yang diracik jatuh ke frasa industri generik. Crawl ringan (best-effort)
	// menambahkan judul halaman, deskripsi meta, dan cuplikan teks.
	if crawl := d.racikCrawlAnchorContext(ctx, anchors); crawl != "" {
		parts = append(parts, crawl)
	}

	wordRule := buildWordRule("", mediaLabel)
	linkRule := "Kapasitas tautan: maksimal 1 tautan; pakai anchor di atas apa adanya sebagai teks tautan dan sisipkan natural di paragraf yang relevan."
	if mediaID != "" {
		if row, err := d.queryRow(ctx, `
			SELECT name, COALESCE(website_url, '') AS website_url, COALESCE(category, '') AS category,
			       COALESCE(word_count_mode, '') AS word_count_mode, max_links
			FROM media WHERE id = $1 LIMIT 1`, mediaID); err == nil && row != nil {
			line := "Media tujuan: " + row.strOr("name", "")
			if site := row.strOr("website_url", ""); site != "" {
				line += " (" + site + ")"
			}
			if cat := row.strOr("category", ""); cat != "" {
				line += " · kategori " + cat
			}
			parts = append(parts, line)
			wordRule = buildWordRule(row.strOr("word_count_mode", ""), mediaLabel)
			if n := row.intOr("max_links", 0); n > 0 {
				linkRule = fmt.Sprintf("Kapasitas tautan: maksimal %d tautan dalam artikel; pakai anchor di atas apa adanya sebagai teks tautan dan sisipkan natural di paragraf yang relevan.", n)
			} else {
				linkRule = "Media ini tidak mengizinkan tautan: jangan menyuruh menyisipkan anchor."
			}
		}
	}
	parts = append(parts, wordRule, linkRule)

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
	if block := racikAngleBlock(angle); block != "" {
		parts = append(parts, block)
	}
	return strings.Join(parts, "\n")
}

// racikAngleBlock: blok angle untuk agent racik. Berbeda dari
// tautAngleBriefBlock (dipakai Writer/Gatekeeper) yang memangkas prompt jadi
// 1500 karakter: prompt angle di Prompt Studio adalah master prompt utuh yang
// menjelaskan cara menulis sudut itu, dan potongannya habis di bagian pembuka
// generik sebelum aturan khas angle-nya terbaca. Di sini prompt dibawa utuh
// (batas 16.000 sama dengan loadTautChosenAnglePrompt) supaya instruksi yang
// diracik benar-benar mengikuti angle pilihan user.
func racikAngleBlock(a *tautAngleContext) string {
	if a == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("[ANGLE PILIHAN KLIEN — " + a.Channel + "/" + a.Name + "]")
	if a.Slug != "" {
		b.WriteString("\nSlug angle: " + a.Slug)
	}
	if a.Description != "" {
		b.WriteString("\nDeskripsi angle: " + a.Description)
	}
	if strings.TrimSpace(a.SourceNotes) != "" {
		b.WriteString("\nSumber informasi angle (bahan fakta dari admin — pakai yang relevan, jangan menambah fakta di luar ini):\n" +
			truncateString(a.SourceNotes, 4000))
	}
	// Master prompt angle TIDAK ikut dikirim: isinya arahan menulis artikel utuh
	// (struktur, dateline, kutipan, format keluaran Writer) yang bertabrakan
	// dengan tugas racik. Dulu blok itu membuat model menyusun "artikel mini",
	// bukan instruksi topik. Untuk racik, deskripsi + catatan sumber sudah cukup.
	return strings.TrimSpace(b.String())
}

// racikCrawlAnchorContext: ringkasan halaman URL anchor (maks 2 domain) supaya
// agent racik mengenali konteks bisnisnya. Best-effort: gagal fetch = bagian
// ini tidak dikirim, instruksi tetap diracik dari bahan lain.
func (d *Deps) racikCrawlAnchorContext(ctx context.Context, anchors []tautAnchorRow) string {
	seen := map[string]bool{}
	lines := []string{}
	for _, a := range anchors {
		raw := strings.TrimSpace(a.URL)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
		if seen[host] {
			continue
		}
		seen[host] = true
		if info := d.racikCrawlPage(ctx, raw); info != "" {
			lines = append(lines, "- "+host+": "+info)
		}
		if len(lines) >= 2 {
			break
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "Konteks domain (hasil crawl halaman URL anchor — untuk mengenali jenis usaha/subjek, bukan bahan angka):\n" + strings.Join(lines, "\n")
}

var racikReTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
var racikReMetaDesc = regexp.MustCompile(`(?is)<meta[^>]+name=["']description["'][^>]+content=["']([^"']*)["']`)
var racikReOGSite = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:site_name["'][^>]+content=["']([^"']*)["']`)

func (d *Deps) racikCrawlPage(ctx context.Context, rawURL string) string {
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en;q=0.8")
	resp, err := security.SafeClient(8 * time.Second).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return ""
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 300*1024))
	html := string(body)
	parts := []string{}
	if m := racikReOGSite.FindStringSubmatch(html); m != nil {
		if v := strings.TrimSpace(htmlUnescapeLight(m[1])); v != "" {
			parts = append(parts, "situs: "+truncateString(v, 120))
		}
	}
	if m := racikReTitle.FindStringSubmatch(html); m != nil {
		if v := strings.TrimSpace(htmlUnescapeLight(m[1])); v != "" {
			parts = append(parts, "judul halaman: "+truncateString(v, 160))
		}
	}
	if m := racikReMetaDesc.FindStringSubmatch(html); m != nil {
		if v := strings.TrimSpace(htmlUnescapeLight(m[1])); v != "" {
			parts = append(parts, "deskripsi: "+truncateString(v, 240))
		}
	}
	text := stripHTMLToText(html)
	if text != "" {
		parts = append(parts, "cuplikan: "+truncateString(text, 400))
	}
	return strings.Join(parts, " | ")
}

// htmlUnescapeLight: bersihkan entitas umum pada judul/deskripsi (cukup untuk
// bahan prompt, bukan rendering halaman).
func htmlUnescapeLight(v string) string {
	return strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">", "&nbsp;", " ").Replace(v)
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
