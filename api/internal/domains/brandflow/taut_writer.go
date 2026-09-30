package brandflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// taut_writer.go: port lib/taut/agents.ts bagian Writer + schedule +
// lib/taut/profiler/angle-fields + lib/tracking/short-link (untuk writer-links).

type writerItemResult struct {
	MediaID    string `json:"media_id"`
	Status     string `json:"status"`
	DocumentID any    `json:"document_id"`
}

// runTautWriter: generate artikel untuk SEMUA media terpilih (status='selected').
func (d *Deps) runTautWriter(ctx context.Context, campaignID, brandID, userID string) ([]writerItemResult, error) {
	items, err := d.selectedTautMedia(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	results := make([]writerItemResult, 0, len(items))
	for _, item := range items {
		result, err := d.generateTautWriterArticle(ctx, campaignID, brandID, userID, item.MediaID)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

type selectedTautMediaRow struct {
	ID       string
	MediaID  string
	Document *string
}

func (d *Deps) selectedTautMedia(ctx context.Context, campaignID string) ([]selectedTautMediaRow, error) {
	rows, err := d.queryRows(ctx,
		"SELECT id, media_id, article_document_id FROM taut_campaign_media WHERE campaign_id = $1 AND status = 'selected' ORDER BY created_at", campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]selectedTautMediaRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, selectedTautMediaRow{
			ID: r.strOr("id", ""), MediaID: r.strOr("media_id", ""), Document: r.strPtr("article_document_id"),
		})
	}
	return out, nil
}

// generateTautWriterArticle: generate artikel untuk SATU media terpilih.
// Idempoten: media yang sudah punya dokumen = regenerasi (tambah versi).
func (d *Deps) generateTautWriterArticle(ctx context.Context, campaignID, brandID, userID, mediaID string) (writerItemResult, error) {
	campaign, err := d.loadTautCampaignRow(ctx, campaignID, brandID)
	if err != nil {
		return writerItemResult{}, err
	}
	materials := asMapValue(campaign.jsonField("editorial_standard_snapshot"))
	hyperlink := asSliceValue(campaign.jsonField("hyperlink_config"))
	briefContext, err := d.loadTautBriefContext(ctx, campaignID)
	if err != nil {
		return writerItemResult{}, err
	}
	backlinkEnabled, smartVariations := d.loadTautBacklinkSettings(ctx, campaignID)
	smartVariationRule := ""
	if smartVariations {
		smartVariationRule = tautSmartVariationRule
	}

	item, err := d.queryRow(ctx, `
		SELECT tcm.id, tcm.media_id, tcm.article_document_id, tcm.regen_count, m.media_type, m.category, m.website_url,
		       m.allow_hyperlink, m.max_links, m.label_sponsor, m.word_count_mode, m.topik_terlarang, m.bahasa_artikel
		FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id
		WHERE tcm.campaign_id = $1 AND tcm.media_id = $2 AND tcm.status = 'selected' LIMIT 1`,
		campaignID, mediaID)
	if err != nil {
		return writerItemResult{}, err
	}
	if item == nil {
		return writerItemResult{}, errStatus(500, "Media tidak terpilih di kampanye ini.")
	}

	itemID := item.strOr("id", "")
	regenCount := item.intOr("regen_count", 0)
	hasDocument := item.has("article_document_id")
	if hasDocument {
		quota := d.userGenerateQuota(ctx, userID)
		if regenCount >= quota-1 {
			return writerItemResult{}, errStatus(500, "Batas jatah generate tercapai: artikel ini sudah di-generate "+
				itoa(regenCount+1)+"x (kuota akun "+itoa(quota)+"x = generate pertama + regenerasi).")
		}
		if err := d.chargeCredit(ctx, brandID,
			"taut-writer-regen:"+campaignID+":"+item.strOr("media_id", "")+":"+itoa(regenCount+1),
			2, "Taut Pintar — regenerasi artikel AI", userID); err != nil {
			return writerItemResult{}, err
		}
	}

	name := d.mediaNameFor(ctx, item.strOr("media_id", ""))
	mediaType := item.strOr("media_type", "")
	if mediaType == "" {
		mediaType = campaign.strOr("media_type", "mainstream")
	}
	if mediaType == "" {
		mediaType = "mainstream"
	}
	wordRule := buildWordRule(item.strOr("word_count_mode", ""), mediaType)

	structureRules := []string{
		"Aturan gaya mainstream (WAJIB): tulis sebagai artikel berita dengan kaidah 5W+1H — paragraf pembuka (lead) menjawab siapa, apa, kapan, di mana, mengapa, dan bagaimana secara ringkas.",
		"Aturan format mainstream (WAJIB): JANGAN gunakan heading/subjudul dan JANGAN memecah isi menjadi daftar poin — seluruh isi adalah paragraf mengalir gaya berita.",
	}
	if mediaType == "blogger" {
		structureRules = []string{
			"Aturan heading (WAJIB): tulis judul bagian sebagai tag HTML <h2> atau <h3> (mulai dari H2, jangan H1).",
			"Aturan daftar (WAJIB): poin/langkah WAJIB ditulis sebagai daftar HTML <ul>/<ol>, bukan paragraf biasa.",
		}
	}

	allowsLink := item.boolOr("allow_hyperlink", true)
	if item.has("max_links") {
		allowsLink = int(item.numOr("max_links")) > 0
	}
	sponsoredMedia := item.strOr("label_sponsor", "tidak_ada") == "ada_sponsor"
	var primaryRel *string
	if sponsoredMedia {
		rel := "sponsored"
		primaryRel = &rel
	}

	topikList := jsonStringList(item.jsonField("topik_terlarang"))
	topikBlock := ""
	if len(topikList) > 0 {
		topikBlock = "[TOPIK TERLARANG — WAJIB DIBACA SEBELUM MENULIS]\nMedia ini MELARANG topik berikut: " +
			strings.Join(topikList, ", ") + ".\nJANGAN membahas, menyebut, atau menyentuh topik-topik tersebut."
	}
	bahasaList := jsonStringList(item.jsonField("bahasa_artikel"))
	bahasaUtama := "Indonesia"
	if len(bahasaList) > 0 && bahasaList[0] != "" {
		bahasaUtama = bahasaList[0]
	}
	languageBlock := "[BAHASA ARTIKEL — WAJIB] Tulis SELURUH artikel (judul, isi, dan meta_description) dalam bahasa: " + bahasaUtama + "."

	primaryURL := strings.TrimSpace(campaign.strOr("primary_url", ""))
	primaryAnchor := strings.TrimSpace(campaign.strOr("primary_anchor", ""))
	backupURL := strings.TrimSpace(campaign.strOr("backup_url", ""))
	backupAnchor := strings.TrimSpace(campaign.strOr("backup_anchor", ""))

	spokespersonBlock := ""
	if spList := spokespersonListValue(campaign.jsonField("spokepersons")); len(spList) > 0 {
		lines := make([]string, 0, len(spList))
		for i, s := range spList {
			line := itoa(i+1) + ". " + s.Name
			if s.Role != "" {
				line += ", " + s.Role
			}
			line += " — kutipan: \"" + s.Quote + "\""
			lines = append(lines, line)
		}
		spokespersonBlock = "[NARASUMBER — WAJIB dipakai sebagai kutipan dalam artikel ini]\n" + strings.Join(lines, "\n") +
			"\nGunakan minimal SATU kutipan di atas (atribusi nama + jabatan). DILARANG mengarang kutipan atau narasumber lain."
	}

	trackingMode := campaign.strOr("tracking_mode", "direct")
	plantPrimary := primaryURL
	plantBackup := backupURL
	if trackingMode == "tauthub" && primaryURL != "" {
		configured := strings.TrimSpace(d.getShortLinkDomain(ctx))
		base := strings.TrimRight(d.canonicalAppBase(), "/")
		if configured != "" {
			stripped := strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(configured, "https://"), "http://"), "/")
			base = "https://" + stripped
		}
		if primary, err := d.upsertWriterShortLink(ctx, brandID, campaignID, "writer_primary", primaryURL, userID); err == nil {
			if base != "" {
				plantPrimary = base + "/t/" + primary
			}
			if backupURL != "" {
				if backup, bErr := d.upsertWriterShortLink(ctx, brandID, campaignID, "writer_backup", backupURL, userID); bErr == nil && base != "" {
					plantBackup = base + "/t/" + backup
				}
			}
		}
	}

	backlinkFlag := "dari input Writer"
	if backlinkEnabled {
		backlinkFlag = "AKTIF dari The Profiler — WAJIB di semua artikel"
	}
	linkContext := ""
	if primaryURL != "" {
		var b strings.Builder
		b.WriteString("Konteks tautan kampanye (backlink " + backlinkFlag + "):\n")
		b.WriteString("   Mode pelacakan: " + trackingMode)
		if trackingMode == "tauthub" {
			b.WriteString(" — TANAM SHORT LINK di bawah ini persis apa adanya ke href (jangan karang path lain; jangan pecah kodenya). Klik tercatat otomatis lalu redirect 301 ke URL asli.")
		}
		b.WriteString("\n   Tautan utama (prioritas, WAJIB): " + plantPrimary + " (anchor: \"" + primaryAnchor + "\")")
		if plantBackup != "" {
			b.WriteString("\n   Tautan cadangan (pelengkap — HANYA bila konteks cocok): " + plantBackup + " (anchor: \"" + backupAnchor + "\")")
		}
		linkContext = b.String()
	}

	linkRule := "JANGAN menanam tautan apa pun di artikel ini — media ini tidak mengizinkan hyperlink."
	if allowsLink && primaryURL != "" {
		relRule := "TANPA atribut rel (dofollow)."
		if sponsoredMedia {
			relRule = "DENGAN atribut rel=\"sponsored\" (link bersponsor)."
		}
		primaryObligation := ""
		if backlinkEnabled {
			primaryObligation = " Ini backlink wajib dari The Profiler — artikel tanpa tautan ini dianggap gagal."
		}
		backupRule := " Media ini hanya mengizinkan 1 tautan — jangan tanam tautan cadangan."
		if plantBackup != "" {
			backupRule = " Tautan cadangan (" + plantBackup + ") BUKAN kewajiban — tanam HANYA bila ada kalimat yang konteksnya benar-benar relevan (maksimal 1x), dengan anchor \"" + backupAnchor + "\" dan WAJIB rel=\"nofollow sponsored\"."
		}
		linkRule = "ATURAN PENANAMAN LINK (WAJIB):\n" +
			"   - SEMUA tautan WAJIB berupa <a href=\"URL\">anchor text</a>. DILARANG KERAS menulis URL telanjang (https://... atau www....) sebagai teks artikel — setiap URL yang muncul HARUS terbungkus tag <a> dengan anchor text yang diberikan.\n" +
			"   - Tautan utama (" + plantPrimary + ") WAJIB ada TEPAT 1x di artikel dengan anchor \"" + primaryAnchor + "\" " + relRule + primaryObligation + "\n" +
			"   -" + backupRule + "\n" +
			"   - Tempatkan tautan secara natural di dalam kalimat (bukan berdampingan, bukan daftar link).\n" +
			"   - TEKNIK KONTEKSTUAL (WAJIB): tanamkan link dengan mengubah frasa yang SUDAH ada di kalimat menjadi tautan (sebutan brand/layanan/topik yang sedang dibahas). DILARANG KERAS menambahkan kalimat pembatas seperti \"Baca juga:\", \"Baca selengkapnya di\", \"Lihat juga:\", atau membuat paragraf khusus untuk menampung link.\n" +
			"   - JANGAN menanam link lain selain yang disediakan di atas."
	}

	// Angle pilihan klien adalah acuan tunggal: pakai instruksi dari brief Profiler
	// bila ada, kalau tidak pakai prompt resmi angle yang dipilih klien di
	// submission. Sebelumnya di sini ada blok "[ANGLE TERSEDIA — pilih satu]" yang
	// hanya memuat 3 angle pertama menurut sort_order, sehingga AI bisa menulis
	// dengan angle yang berbeda dari pilihan klien.
	channelInstruction := d.loadTautChannelInstruction(ctx, campaignID, mediaType)
	anglePromptBlock := ""
	if channelInstruction == "" {
		anglePromptBlock = d.loadTautChosenAnglePrompt(ctx, campaignID, mediaType)
	}

	var identity strings.Builder
	identity.WriteString("[IDENTITAS MEDIA]\n   Nama: " + name + "\n   Tipe: " + mediaType)
	if category := item.strOr("category", ""); category != "" {
		identity.WriteString("\n   Kategori: " + category)
	}
	if website := item.strOr("website_url", ""); website != "" {
		identity.WriteString("\n   Website: " + website)
	}
	identity.WriteString("\n   Sesuaikan gaya dan sudut pandang artikel dengan karakter media di atas.")

	profileLine := briefContext
	if profileLine == "" {
		profileLine = "Profil bisnis: " + defaultJSONObjectValue(campaign, "business_profile")
	}
	promptLines := []string{"Kampanye: " + campaign.strOr("name", ""), profileLine}
	if briefContext == "" {
		promptLines = append(promptLines,
			"Target media: "+orDash(campaign.strOr("media_type", "")),
			"Trigger: "+orDash(campaign.strOr("trigger_keywords", "")))
	}
	promptLines = append(promptLines,
		"Media tujuan: "+name,
		"Tipe media: "+mediaType,
		wordRule,
	)
	promptLines = append(promptLines, structureRules...)
	promptLines = append(promptLines,
		languageBlock,
		identity.String(),
		topikBlock,
		channelInstruction,
		anglePromptBlock,
		linkContext,
		linkRule,
		spokespersonBlock,
		smartVariationRule,
		"Bahan: "+jsonStringify(orEmptyMapValue(materials)),
		"Aturan hyperlink media (bila media tidak mengizinkan, gunakan ATURAN PENANAMAN LINK nol): "+jsonStringify(orEmptySliceValue(hyperlink)),
		"Kutipan/fakta hanya dari bahan/brief di atas — jangan mengarang angka, nama, atau jabatan.",
		"Aturan meta deskripsi (WAJIB): sertakan meta_description 120-160 karakter bahasa Indonesia aktif.",
		"", "Tulis artikel unik utk media ini. Output JSON valid {title, content_html, meta_description}.",
	)
	filtered := []string{}
	for _, line := range promptLines {
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	prompt := strings.Join(filtered, "\n")

	result, err := d.runTautAgent(ctx, "writer", campaignID, prompt, &mediaID, true)
	if err != nil {
		return writerItemResult{}, err
	}
	title := ""
	contentHTML := ""
	metaDescription := ""
	var parsed struct {
		Title           string `json:"title"`
		ContentHTML     string `json:"content_html"`
		MetaDescription string `json:"meta_description"`
	}
	if json.Unmarshal([]byte(result.Content), &parsed) == nil {
		title = truncateString(parsed.Title, 300)
		contentHTML = truncateString(parsed.ContentHTML, 200000)
		metaDescription = truncateString(strings.TrimSpace(parsed.MetaDescription), 500)
	} else {
		title = campaign.strOr("name", "") + " — " + name
		contentHTML = truncateString(result.Content, 200000)
	}
	if strings.TrimSpace(contentHTML) == "" {
		return writerItemResult{MediaID: mediaID, Status: "generating", DocumentID: nil}, nil
	}
	if metaDescription == "" {
		plainMeta := stripHTMLToText(contentHTML)
		if len([]rune(plainMeta)) > 160 {
			metaDescription = strings.TrimRight(string([]rune(plainMeta)[:160]), " ") + "…"
		} else {
			metaDescription = plainMeta
		}
		metaDescription = truncateString(metaDescription, 500)
	}
	contentHTML = finalizeTautArticleHTML(contentHTML, mediaType, allowsLink, []tautBacklinkSpec{
		{URL: plantPrimary, Anchor: primaryAnchor, Rel: primaryRel, Mandatory: plantPrimary != ""},
		{URL: plantBackup, Anchor: backupAnchor, Rel: strPtrValue("nofollow sponsored"), Mandatory: false},
	})

	isRegen := hasDocument
	docID := ""
	if isRegen {
		docID = item.strOr("article_document_id", "")
	} else {
		docID = uuid.NewString()
	}
	jobID := uuid.NewString()
	if isRegen {
		doc, err := d.queryRow(ctx, "SELECT current_version FROM content_documents WHERE id = $1 LIMIT 1", docID)
		if err != nil {
			return writerItemResult{}, err
		}
		next := 1
		if doc != nil {
			next = doc.intOr("current_version", 1) + 1
		}
		if err := d.exec(ctx,
			"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, $3, $4, 'html', 'ai_generated', 'Taut Pintar Writer — regenerasi', $5)",
			uuid.NewString(), docID, next, contentHTML, userID); err != nil {
			return writerItemResult{}, err
		}
		if err := d.exec(ctx,
			"UPDATE content_documents SET current_version = $1, meta_description = $2, updated_at = NOW() WHERE id = $3",
			next, metaDescription, docID); err != nil {
			return writerItemResult{}, err
		}
	} else {
		slug := slugOrValue(title, "taut-artikel")
		if err := d.exec(ctx,
			"INSERT INTO content_documents (id, brand_id, job_id, status, title, slug, meta_title, meta_description, current_version, created_by, approved_by, approved_at) VALUES ($1, $2, $3, 'approved', $4, $5, $6, $7, 1, $8, $9, NOW())",
			docID, brandID, jobID, title, slug, truncateString(title, 255), metaDescription, userID, userID); err != nil {
			return writerItemResult{}, err
		}
		if err := d.exec(ctx,
			"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, 1, $3, 'html', 'ai_generated', 'Taut Pintar Writer', $4)",
			uuid.NewString(), docID, contentHTML, userID); err != nil {
			return writerItemResult{}, err
		}
	}
	if isRegen {
		if err := d.exec(ctx,
			"UPDATE taut_campaign_media SET article_status = 'draft', regen_count = regen_count + 1 WHERE id = $1", itemID); err != nil {
			return writerItemResult{}, err
		}
	} else {
		if err := d.exec(ctx,
			"UPDATE taut_campaign_media SET article_document_id = $1, article_status = 'draft' WHERE id = $2", docID, itemID); err != nil {
			return writerItemResult{}, err
		}
	}
	if !isRegen {
		if err := d.chargeCredit(ctx, brandID, "taut-writer:"+campaignID+":"+mediaID, 1,
			"Taut Pintar — penulisan AI artikel "+name, userID); err != nil {
			return writerItemResult{}, err
		}
	}
	return writerItemResult{MediaID: mediaID, Status: "draft", DocumentID: docID}, nil
}

// userGenerateQuota: kuota generate per akun (users.writer_generate_quota,
// NULL/kolom tak ada → default 3: 1x generate + 2x regenerasi).
func (d *Deps) userGenerateQuota(ctx context.Context, userID string) int {
	row, err := d.queryRow(ctx, "SELECT writer_generate_quota FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil || row == nil || !row.has("writer_generate_quota") {
		return 3
	}
	quota := row.intOr("writer_generate_quota", 0)
	if quota < 1 {
		return 3
	}
	return quota
}

// ---------- schedule / timeline ----------

type tautTimelineItem struct {
	MediaID     string `json:"media_id"`
	ScheduledAt string `json:"scheduled_at"`
}

type tautCreatedPublication struct {
	MediaID       string `json:"media_id"`
	PublicationID string `json:"publication_id,omitempty"`
	Status        string `json:"status"`
	Detail        string `json:"detail,omitempty"`
}

// timelineFromBody: jadwal dari klien; entri tak dikenal dibuang, waktu lampau
// di-clamp ke sekarang, media tanpa entri default sekarang (serentak).
func timelineFromBody(body map[string]any, readyIDs map[string]bool) ([]tautTimelineItem, error) {
	schedule, ok := body["schedule"].([]any)
	if !ok || len(schedule) == 0 {
		now := isoNow()
		out := make([]tautTimelineItem, 0, len(readyIDs))
		for _, id := range sortedStrings(readyIDs) {
			out = append(out, tautTimelineItem{MediaID: id, ScheduledAt: now})
		}
		return out, nil
	}
	if len(schedule) > 20 {
		return nil, errUnprocessable("Array must contain at most 20 element(s)")
	}
	byID := map[string]any{}
	for _, entry := range schedule {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		mediaID, _ := item["media_id"].(string)
		if mediaID == "" {
			return nil, errUnprocessable("Invalid uuid")
		}
		if raw, present := item["scheduled_at"]; present && raw != nil {
			s, isStr := raw.(string)
			if !isStr {
				return nil, errUnprocessable("Invalid datetime")
			}
			if s != "" {
				if _, ok := parseISO(s); !ok {
					return nil, errUnprocessable("Invalid datetime")
				}
			}
		}
		byID[mediaID] = item["scheduled_at"]
	}
	now := time.Now()
	out := make([]tautTimelineItem, 0, len(readyIDs))
	for _, id := range sortedStrings(readyIDs) {
		scheduled := now.UTC().Format("2006-01-02T15:04:05.000Z")
		if raw, ok := byID[id]; ok {
			if s, isStr := raw.(string); isStr && s != "" {
				if t, ok := parseISO(s); ok && t.After(now) {
					scheduled = t.UTC().Format("2006-01-02T15:04:05.000Z")
				}
			}
		}
		out = append(out, tautTimelineItem{MediaID: id, ScheduledAt: scheduled})
	}
	return out, nil
}

// createTautPublications: buat publikasi via createPublicationRecord (per-item
// error tidak menghentikan sisanya).
func (d *Deps) createTautPublications(ctx context.Context, campaignID, brandID, userID string, timeline []tautTimelineItem) ([]tautCreatedPublication, error) {
	out := []tautCreatedPublication{}
	for _, t := range timeline {
		ready, err := d.queryRow(ctx,
			"SELECT article_document_id, featured_image_url FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND article_status = 'ready' LIMIT 1",
			campaignID, t.MediaID)
		if err != nil {
			return nil, err
		}
		if ready == nil || !ready.has("article_document_id") {
			continue
		}
		documentID := ready.strOr("article_document_id", "")
		var scheduledPtr *time.Time
		if parsed, ok := parseISO(t.ScheduledAt); ok {
			scheduledPtr = &parsed
		}
		featured := "/images/article-fallback.png"
		if v := ready.strOr("featured_image_url", ""); v != "" {
			featured = v
		}
		created, err := d.createPublicationRecord(ctx, createPublicationInput{
			BrandID: brandID, UserID: userID, DocumentID: documentID, MediaID: t.MediaID,
			PublishStatus: "publish", ScheduledAt: scheduledPtr, CampaignID: &campaignID,
			FeaturedImageURL: &featured,
		})
		if err != nil {
			message := truncateString(err.Error(), 300)
			if strings.Contains(strings.ToLower(message), "already published") {
				existing, qErr := d.queryRow(ctx,
					"SELECT id FROM publications WHERE document_id = $1 AND media_id = $2 LIMIT 1", documentID, t.MediaID)
				entry := tautCreatedPublication{MediaID: t.MediaID, Status: "already_published"}
				if qErr == nil && existing != nil {
					entry.PublicationID = existing.strOr("id", "")
				}
				out = append(out, entry)
				continue
			}
			out = append(out, tautCreatedPublication{MediaID: t.MediaID, Status: "failed", Detail: message})
			continue
		}
		out = append(out, tautCreatedPublication{
			MediaID: t.MediaID, PublicationID: created.Row.strOr("id", ""), Status: created.Path,
		})
	}
	return out, nil
}

// ---------- short links ----------

const shortLinkDomainKey = "short_link_domain"
const tautCodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func randomTautCode5() string {
	b := make([]byte, 5)
	for i := range b {
		b[i] = tautCodeAlphabet[randInt(len(tautCodeAlphabet))]
	}
	return string(b)
}

// getShortLinkDomain: domain short link dari platform_settings; "" = domain utama.
func (d *Deps) getShortLinkDomain(ctx context.Context) string {
	row, err := d.queryRow(ctx, "SELECT value FROM platform_settings WHERE key = $1 LIMIT 1", shortLinkDomainKey)
	if err != nil || row == nil {
		return ""
	}
	return strings.TrimSpace(row.strOr("value", ""))
}

// upsertWriterShortLink: upsert idempoten per (campaign, kind); kode tetap.
func (d *Deps) upsertWriterShortLink(ctx context.Context, brandID, campaignID, kind, destinationURL, userID string) (string, error) {
	existing, err := d.queryRow(ctx,
		"SELECT id, code FROM tracking_links WHERE campaign_id = $1 AND kind = $2 LIMIT 1", campaignID, kind)
	if err != nil {
		return "", err
	}
	if existing != nil {
		if err := d.exec(ctx,
			"UPDATE tracking_links SET destination_url = $1, status = 'active', updated_at = NOW() WHERE id = $2",
			destinationURL, existing.strOr("id", "")); err != nil {
			return "", err
		}
		return existing.strOr("code", ""), nil
	}
	for attempt := 0; attempt < 8; attempt++ {
		code := randomTautCode5()
		taken, err := d.queryRow(ctx, "SELECT id FROM tracking_links WHERE code = $1 LIMIT 1", code)
		if err != nil {
			return "", err
		}
		if taken != nil {
			continue
		}
		if err := d.exec(ctx,
			"INSERT INTO tracking_links (id, code, brand_id, campaign_id, kind, destination_url, status, created_by) VALUES ($1, $2, $3, $4, $5, $6, 'active', $7)",
			uuid.NewString(), code, brandID, campaignID, kind, destinationURL, nullIfEmpty(userID)); err != nil {
			continue
		}
		return code, nil
	}
	return "", errStatus(500, "Gagal menghasilkan kode short link unik.")
}

// getTautWriterShortLinks: pasangan kode (primary, backup) untuk kampanye.
func (d *Deps) getTautWriterShortLinks(ctx context.Context, campaignID string) (any, any) {
	rows, err := d.queryRows(ctx,
		"SELECT code, kind FROM tracking_links WHERE campaign_id = $1 AND kind IN ('writer_primary','writer_backup')", campaignID)
	if err != nil {
		return nil, nil
	}
	var primary, backup any
	for _, row := range rows {
		switch row.strOr("kind", "") {
		case "writer_primary":
			primary = row.strOr("code", "")
		case "writer_backup":
			backup = row.strOr("code", "")
		}
	}
	return primary, backup
}

// ---------- crawl helpers ----------

type tautAngleRow struct {
	Channel string
	Slug    string
	Name    string
}

// activeTautAngles: angle aktif dari DB untuk rekomendasi AI.
func (d *Deps) activeTautAngles(ctx context.Context) ([]tautAngleRow, error) {
	rows, err := d.queryRows(ctx, `
		SELECT c.slug AS channel, a.slug, a.name
		FROM taut_content_angles a JOIN taut_media_channels c ON c.id = a.channel_id
		WHERE a.active = TRUE ORDER BY c.slug, a.sort_order`)
	if err != nil {
		return nil, err
	}
	out := make([]tautAngleRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, tautAngleRow{Channel: r.strOr("channel", ""), Slug: r.strOr("slug", ""), Name: r.strOr("name", "")})
	}
	return out, nil
}

var objectives = []string{"brand_awareness", "seo_dominance", "conversion", "reputation_management"}

var defaultAudienceOptions = []string{
	"Mahasiswa", "Ibu Hamil", "Karyawan", "Profesional", "Pemilik Bisnis",
	"Pengusaha / Entrepreneur", "Freelancer", "Orang Tua", "Pelajar", "Fresh Graduate",
	"Manajer / Decision Maker", "Developer / Teknisi", "Marketing / PR Professional",
	"Investor", "Konsumen Rumah Tangga",
}

const otherAudience = "Other / Lainnya"

var tautChannels = []string{"mainstream", "blogger"}

// buildCrawlPrompt: prompt ekstraksi fakta website (port route crawl).
func buildCrawlPrompt(text string, angles []tautAngleRow, rawURL string) string {
	angleList := []string{}
	for _, a := range angles {
		angleList = append(angleList, "- "+a.Channel+"/"+a.Slug+": "+a.Name)
	}
	lines := []string{
		"Ekstrak fakta dari konten website berikut. HANYA gunakan informasi yang benar-benar ada di teks.",
		"Jangan mengarang, menebak, atau menyimpulkan data yang tidak tertulis.",
		"",
		`Kembalikan JSON valid: {"business_name":"","industry":"","products_services":"","positioning":"","value_proposition":"","tone_of_voice":"","keywords":[],"facts":["..."],"uncertain":["..."],"objective":"","audience":[],"guardrails":[],"spokepersons":[],"channels":[],"backlink":{"enabled":true,"target_url":"","anchor_text":""},"recommended_angles":{"mainstream":"slug|kosongkan bila tidak relevan","blogger":"slug|kosongkan bila tidak relevan"},"budget_recommendation":{"mode":"efficient|optimal","amount":0,"reason":"1 kalimat alasan"}}`,
		"",
		`objective: pilih SATU dari: ` + strings.Join(objectives, ", ") + ". Kosongkan bila tidak jelas.",
		`audience: array string, pilih dari: ` + strings.Join(append(append([]string{}, defaultAudienceOptions...), otherAudience), ", ") + ". Kosongkan bila tidak jelas.",
		`channels: array string, pilih dari: ` + strings.Join(tautChannels, ", ") + ".",
		"recommended_angles: untuk SETIAP channel yang Anda pilih, rekomendasikan angle paling cocok:",
		strings.Join(angleList, "\n"),
		"",
		"TEKS WEBSITE:",
		text,
	}
	return strings.Join(lines, "\n")
}

// normalizeCrawlExtraction: validasi + bentuk nilai hasil ekstraksi crawl.
func normalizeCrawlExtraction(parsed map[string]any, rawURL string, angles []tautAngleRow) map[string]any {
	knownKeys := []string{
		"business_name", "industry", "products_services", "positioning", "value_proposition",
		"tone_of_voice", "keywords", "facts", "uncertain", "objective", "audience",
		"guardrails", "spokepersons", "channels",
	}
	hasKnown := false
	for _, k := range knownKeys {
		if _, ok := parsed[k]; ok {
			hasKnown = true
			break
		}
	}
	data := parsed
	if !hasKnown {
		for _, v := range parsed {
			if inner, ok := v.(map[string]any); ok {
				innerHas := false
				for _, k := range knownKeys {
					if _, ok := inner[k]; ok {
						innerHas = true
						break
					}
				}
				if innerHas {
					data = inner
					break
				}
			}
		}
	}
	objective := ""
	if s, ok := data["objective"].(string); ok && containsString(objectives, s) {
		objective = s
	}
	validAudience := map[string]bool{otherAudience: true}
	for _, a := range defaultAudienceOptions {
		validAudience[a] = true
	}
	audience := []string{}
	if arr, ok := data["audience"].([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && validAudience[s] && len(audience) < 10 {
				audience = append(audience, s)
			}
		}
	}
	validChannels := map[string]bool{}
	for _, c := range tautChannels {
		validChannels[c] = true
	}
	channels := []string{}
	if arr, ok := data["channels"].([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && validChannels[s] && len(channels) < 2 {
				channels = append(channels, s)
			}
		}
	}
	backlink := map[string]any{"enabled": false, "target_url": "", "anchor_text": ""}
	if bl, ok := data["backlink"].(map[string]any); ok {
		enabled, _ := bl["enabled"].(bool)
		targetURL, _ := bl["target_url"].(string)
		anchorText, _ := bl["anchor_text"].(string)
		backlink = map[string]any{"enabled": enabled, "target_url": targetURL, "anchor_text": anchorText}
	}
	recommended := map[string]any{}
	if ra, ok := data["recommended_angles"].(map[string]any); ok {
		anglesByChannel := map[string]bool{}
		for _, a := range angles {
			anglesByChannel[a.Channel+"/"+a.Slug] = true
		}
		for _, ch := range tautChannels {
			if slug, ok := ra[ch].(string); ok && slug != "" && containsString(channels, ch) && anglesByChannel[ch+"/"+slug] {
				recommended[ch] = slug
			}
		}
	}
	budget := map[string]any{"mode": "efficient", "amount": 0, "reason": ""}
	if br, ok := data["budget_recommendation"].(map[string]any); ok {
		mode := "efficient"
		if br["mode"] == "optimal" {
			mode = "optimal"
		}
		amount := 0
		if f, ok := toFloat(br["amount"]); ok {
			amount = int(f)
			if amount < 5000 {
				amount = 5000
			}
			if amount > 100000 {
				amount = 100000
			}
		}
		reason, _ := br["reason"].(string)
		budget = map[string]any{"mode": mode, "amount": amount, "reason": truncateString(reason, 200)}
	}
	return map[string]any{
		"url":                   rawURL,
		"business_name":         stringValue(data["business_name"]),
		"industry":              stringValue(data["industry"]),
		"products_services":     stringValue(data["products_services"]),
		"positioning":           stringValue(data["positioning"]),
		"value_proposition":     stringValue(data["value_proposition"]),
		"tone_of_voice":         stringValue(data["tone_of_voice"]),
		"keywords":              capStrings(stringArrayValue(data["keywords"]), 20),
		"facts":                 stringArrayValue(data["facts"]),
		"uncertain":             stringArrayValue(data["uncertain"]),
		"objective":             objective,
		"audience":              audience,
		"guardrails":            capStrings(stringArrayValue(data["guardrails"]), 20),
		"spokepersons":          normalizeSpokespersons(data["spokepersons"]),
		"channels":              channels,
		"backlink":              backlink,
		"recommended_angles":    recommended,
		"dynamic_questions":     map[string]any{},
		"dynamic_pr_answers":    map[string]any{},
		"budget_recommendation": budget,
	}
}

func normalizeSpokespersons(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return []map[string]any{}
	}
	out := []map[string]any{}
	for _, item := range arr {
		if len(out) >= 3 {
			break
		}
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(stringValue(m["name"]))
		if name == "" {
			continue
		}
		out = append(out, map[string]any{
			"name": name, "role": stringValue(m["role"]), "quote": stringValue(m["quote"]),
		})
	}
	return out
}

// loadTautChosenAnglePrompt: prompt resmi untuk angle yang benar-benar dipilih
// klien pada jenis media ini.
//
// Dua langkah: submission Profiler menyimpan pilihan klien (`channels` jsonb,
// [{channel, angle_slug}]), lalu slug itu dipetakan ke versi prompt aktif di Taut
// Prompt Studio (`taut_content_angle_prompt_versions`). Ini juga jalur yang
// membuat prompt per-angle yang diatur admin akhirnya sampai ke penulisan
// artikel, bukan hanya ke The Profiler.
//
// Dipakai hanya sebagai cadangan saat brief Profiler belum ada, misalnya
// kampanye jalur Kilat yang tidak menjalankan analisis Profiler.
func (d *Deps) loadTautChosenAnglePrompt(ctx context.Context, campaignID, mediaType string) string {
	sub, err := d.queryRow(ctx, `
		SELECT channels
		FROM taut_profiler_submissions
		WHERE campaign_id = $1
		ORDER BY revision DESC LIMIT 1`, campaignID)
	if err != nil || sub == nil {
		return ""
	}
	angleSlug := ""
	for _, entry := range asSliceValue(sub.jsonField("channels")) {
		ch := asMapValue(entry)
		if ch == nil || mapGetString(ch, "channel", "") != mediaType {
			continue
		}
		if slug := mapGetString(ch, "angle_slug", ""); slug != "" {
			angleSlug = slug
			break
		}
	}
	if angleSlug == "" {
		return ""
	}
	row, err := d.queryRow(ctx, `
		SELECT v.prompt
		FROM taut_content_angle_prompt_versions v
		JOIN taut_content_angles a ON a.id = v.content_angle_id
		JOIN taut_media_channels c ON c.id = a.channel_id
		WHERE a.slug = $1 AND c.slug = $2 AND v.status = 'active'
		ORDER BY v.version DESC LIMIT 1`, angleSlug, mediaType)
	if err != nil || row == nil {
		return ""
	}
	prompt := strings.TrimSpace(row.strOr("prompt", ""))
	if prompt == "" {
		return ""
	}
	return "[ANGLE PILIHAN KLIEN — " + mediaType + "/" + angleSlug + "]\n" + truncateString(prompt, 1500)
}

// securityGet: safeFetch GET dengan header (port safeFetch).
func securityGet(r *http.Request, rawURL string, headers map[string]string) (*http.Response, error) {
	return security.Get(r.Context(), rawURL, headers, 30*time.Second)
}

// ---------- value helpers ----------

func asMapValue(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asSliceValue(v any) []any {
	s, _ := v.([]any)
	return s
}

func orEmptyMapValue(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func orEmptySliceValue(s []any) []any {
	if s == nil {
		return []any{}
	}
	return s
}

func defaultJSONObjectValue(row bsRow, key string) string {
	v := row.jsonField(key)
	if v == nil {
		return "{}"
	}
	return jsonStringify(v)
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func stringArrayValue(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func strPtrValue(s string) *string { return &s }

func slugOrValue(title, fallback string) string {
	slug := slugifyPlain(title)
	if slug == "" {
		return fallback
	}
	return truncateString(slug, 200)
}

func sortedStrings(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

type spokespersonValue struct {
	Name  string
	Role  string
	Quote string
}

func spokespersonListValue(raw any) []spokespersonValue {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []spokespersonValue{}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(stringValue(m["name"]))
		if name == "" {
			continue
		}
		out = append(out, spokespersonValue{
			Name: name, Role: strings.TrimSpace(stringValue(m["role"])), Quote: strings.TrimSpace(stringValue(m["quote"])),
		})
	}
	return out
}
