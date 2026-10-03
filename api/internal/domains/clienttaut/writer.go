package clienttaut

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// Port lib/taut/agents.ts bagian Writer + backlink policy enforcement.

// writerItemResult: parity WriterItemResult.
type writerItemResult struct {
	MediaID    string `json:"media_id"`
	Status     string `json:"status"`
	DocumentID any    `json:"document_id"`
}

// runWriter: generate artikel untuk SEMUA media terpilih (status='selected').
func (d *Deps) runWriter(ctx context.Context, campaignID, brandID, userID, appBase string) ([]writerItemResult, error) {
	items, err := d.selectedMedia(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	results := make([]writerItemResult, 0, len(items))
	for _, item := range items {
		result, err := d.generateWriterArticle(ctx, campaignID, brandID, userID, item.MediaID, appBase)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

type selectedMediaRow struct {
	ID       string
	MediaID  string
	Document *string
}

func (d *Deps) selectedMedia(ctx context.Context, campaignID string) ([]selectedMediaRow, error) {
	rows, err := d.queryRows(ctx,
		"SELECT id, media_id, article_document_id FROM taut_campaign_media WHERE campaign_id = $1 AND status = 'selected' ORDER BY created_at", campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]selectedMediaRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, selectedMediaRow{
			ID:       r.strOr("id", ""),
			MediaID:  r.strOr("media_id", ""),
			Document: r.strPtr("article_document_id"),
		})
	}
	return out, nil
}

// generateWriterArticle: generate artikel untuk SATU media terpilih.
// Idempoten: media yang sudah punya dokumen = regenerasi (tambah versi).
func (d *Deps) generateWriterArticle(ctx context.Context, campaignID, brandID, userID, mediaID, appBase string) (writerItemResult, error) {
	campaign, err := d.loadCampaignForAgents(ctx, campaignID, brandID)
	if err != nil {
		return writerItemResult{}, err
	}
	materials := asMap(jsonField(campaign, "materials"))
	hyperlink, _ := jsonField(campaign, "hyperlink_config").([]any)
	briefContext, err := d.loadBriefContext(ctx, campaignID)
	if err != nil {
		return writerItemResult{}, err
	}

	item, err := d.queryRow(ctx,
		`SELECT tcm.id, tcm.media_id, tcm.article_document_id, tcm.regen_count, m.media_type, m.category, m.website_url,
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
		// Limit mengikuti jatah akun (Admin Users); NULL = tanpa batas.
		quota := d.userGenerateQuota(ctx, userID)
		if quota > 0 && regenCount+1 > quota {
			return writerItemResult{}, errStatus(500, "Batas jatah generate tercapai: artikel ini sudah di-generate "+
				itoa(regenCount+1)+"x (kuota akun "+itoa(quota)+"x = generate pertama + regenerasi).")
		}
		if err := d.chargeCredit(ctx, brandID,
			"taut-writer-regen:"+campaignID+":"+item.strOr("media_id", "")+":"+itoa(regenCount+1),
			2, "Taut Pintar — regenerasi artikel AI", userID); err != nil {
			return writerItemResult{}, err
		}
	}

	name := d.mediaName(ctx, item.strOr("media_id", ""))
	mediaType := item.strOr("media_type", "")
	if mediaType == "" {
		mediaType = campaign.strOr("media_type", "mainstream")
	}
	if mediaType == "" {
		mediaType = "mainstream"
	}
	wordRule := buildWordRule(item.strPtr("word_count_mode"), mediaType)

	structureRules := []string{
		"Aturan gaya mainstream (WAJIB): tulis sebagai artikel berita dengan kaidah 5W+1H — paragraf pembuka (lead) menjawab siapa, apa, kapan, di mana, mengapa, dan bagaimana secara ringkas; paragraf berikutnya mengurai detail secara naratif dari yang paling penting ke pendukung.",
		"Aturan format mainstream (WAJIB): JANGAN gunakan heading/subjudul (<h2>/<h3>) dan JANGAN memecah isi menjadi daftar poin (<ul>/<ol>) — seluruh isi adalah paragraf mengalir gaya berita, tanpa markdown.",
	}
	if mediaType == "blogger" {
		structureRules = []string{
			"Aturan heading (WAJIB): tulis judul bagian sebagai tag HTML <h2> atau <h3> (dimulai dari H2, jangan pernah H1); jangan tinggalkan judul bagian sebagai teks biasa.",
			"Aturan daftar (WAJIB): poin, fitur, langkah, atau hal berurutan WAJIB ditulis sebagai daftar HTML <ul><li>…</li></ul> (tanpa urutan) atau <ol><li>…</li></ol> (berurutan) — jangan tulis daftar sebagai paragraf biasa atau kalimat berawal tanda '-' / '1.'.",
		}
	}

	maxLinks := item.numOr("max_links")
	hasMaxLinks := item.has("max_links")
	allowsLink := item.boolOr("allow_hyperlink", true)
	if hasMaxLinks {
		allowsLink = maxLinks > 0
	}
	linkCap := 0
	if allowsLink {
		cap := 1
		if hasMaxLinks {
			cap = int(maxLinks)
		}
		if cap < 1 {
			cap = 1
		}
		if cap > 2 {
			cap = 2
		}
		linkCap = cap
	}
	sponsoredMedia := item.strOr("label_sponsor", "tidak_ada") == "ada_sponsor"
	var primaryRel *string
	if sponsoredMedia {
		rel := "sponsored"
		primaryRel = &rel
	}

	topikList := jsonStringList(item.strOr("topik_terlarang", "[]"))
	topikBlock := ""
	if len(topikList) > 0 {
		topikBlock = "[TOPIK TERLARANG — WAJIB DIBACA SEBELUM MENULIS]\nMedia ini MELARANG topik berikut: " + strings.Join(topikList, ", ") +
			".\nJANGAN membahas, menyebut, atau menyentuh topik-topik tersebut dalam artikel ini dalam bentuk apa pun."
	}
	bahasaList := jsonStringList(item.strOr("bahasa_artikel", "[]"))
	bahasaUtama := "Indonesia"
	if len(bahasaList) > 0 && bahasaList[0] != "" {
		bahasaUtama = bahasaList[0]
	}
	languageBlock := "[BAHASA ARTIKEL — WAJIB] Tulis SELURUH artikel (judul, isi, dan meta_description) dalam bahasa: " + bahasaUtama + "."

	primaryURL := strings.TrimSpace(campaign.strOr("primary_url", ""))
	primaryAnchor := strings.TrimSpace(campaign.strOr("primary_anchor", ""))
	backupURL := strings.TrimSpace(campaign.strOr("backup_url", ""))
	backupAnchor := strings.TrimSpace(campaign.strOr("backup_anchor", ""))

	// Narasumber dari draft Profiler — kutipan WAJIB bila ada.
	spokespersonBlock := ""
	spRaw := jsonField(campaign, "spokepersons")
	if spList := spokespersonList(spRaw); len(spList) > 0 {
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
	if trackingMode == "" {
		trackingMode = "direct"
	}
	plantPrimary := primaryURL
	plantBackup := backupURL
	if trackingMode == "tauthub" && primaryURL != "" {
		configured := strings.TrimSpace(d.getShortLinkBase(ctx))
		base := strings.TrimRight(strings.TrimSpace(appBase), "/")
		if configured != "" {
			stripped := strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(configured, "https://"), "http://"), "/")
			base = "https://" + stripped
		}
		primary, err := d.upsertWriterShortLink(ctx, brandID, campaignID, "writer_primary", primaryURL, userID)
		if err == nil {
			if base != "" {
				plantPrimary = base + "/t/" + primary.Code
			}
			if backupURL != "" && linkCap >= 2 {
				if backup, bErr := d.upsertWriterShortLink(ctx, brandID, campaignID, "writer_backup", backupURL, userID); bErr == nil {
					if base != "" {
						plantBackup = base + "/t/" + backup.Code
					}
				}
			}
		} else {
			plantPrimary = primaryURL
			plantBackup = backupURL
		}
	}

	// Pengaturan backlink Profiler: backlink aktif + Smart Variations.
	backlinkEnabled := false
	smartVariationRule := ""
	sub, _ := d.queryRow(ctx,
		"SELECT backlink FROM taut_profiler_submissions WHERE campaign_id = $1 AND status = 'completed' ORDER BY revision DESC LIMIT 1", campaignID)
	if sub != nil {
		bl := asMap(jsonField(sub, "backlink"))
		if bl != nil {
			if enabled, ok := bl["enabled"].(bool); ok && enabled {
				backlinkEnabled = true
			}
			if sv, ok := bl["smart_variations"].(bool); ok && sv {
				smartVariationRule = "   - SMART VARIATIONS (WAJIB — aktif dari The Profiler): jangan tanam anchor yang sama persis dua kali — variasikan dengan frasa LSI/natural yang bermakna sama (lokasi, kategori, konteks kalimat). Semua tautan tetap wajib ber-anchor; tautan utama tetap URL prioritas di semua artikel."
			}
		}
	}

	linkContext := ""
	if primaryURL != "" {
		backlinkLabel := "dari input Writer"
		if backlinkEnabled {
			backlinkLabel = "AKTIF dari The Profiler — WAJIB di semua artikel"
		}
		var b strings.Builder
		b.WriteString("Konteks tautan kampanye (backlink " + backlinkLabel + "):\n")
		b.WriteString("   Mode pelacakan: " + trackingMode)
		if trackingMode == "tauthub" {
			b.WriteString(" — TANAM SHORT LINK di bawah ini persis apa adanya ke href (jangan karang path lain; jangan pecah kodenya). Klik tercatat otomatis lalu redirect 301 ke URL asli.")
		}
		b.WriteString("\n   Tautan utama (prioritas, WAJIB): " + plantPrimary + " (anchor: \"" + primaryAnchor + "\")")
		if plantBackup != "" && linkCap >= 2 {
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
		backlinkNote := ""
		if backlinkEnabled {
			backlinkNote = " Ini backlink wajib dari The Profiler — artikel tanpa tautan ini dianggap gagal."
		}
		backupRule := " Media ini hanya mengizinkan 1 tautan — jangan tanam tautan cadangan."
		if plantBackup != "" && linkCap >= 2 {
			backupRule = " Tautan cadangan (" + plantBackup + ") BUKAN kewajiban — tanam HANYA bila ada kalimat yang konteksnya benar-benar relevan (maksimal 1x), dengan anchor \"" + backupAnchor + "\" dan WAJIB rel=\"nofollow sponsored\"."
		}
		linkRule = "ATURAN PENANAMAN LINK (WAJIB):\n" +
			"   - SEMUA tautan WAJIB berupa <a href=\"URL\">anchor text</a>. DILARANG KERAS menulis URL telanjang (https://... atau www....) sebagai teks artikel — setiap URL yang muncul HARUS terbungkus tag <a> dengan anchor text yang diberikan.\n" +
			"   - Tautan utama (" + plantPrimary + ") WAJIB ada TEPAT 1x di artikel dengan anchor \"" + primaryAnchor + "\" " + relRule + backlinkNote + "\n" +
			"   -" + backupRule + "\n" +
			"   - Tempatkan tautan secara natural di dalam kalimat (bukan berdampingan, bukan daftar link).\n" +
			"   - TEKNIK KONTEKSUAL (WAJIB): tanamkan link dengan mengubah frasa yang SUDAH ada di kalimat menjadi tautan (sebutan brand/layanan/topik yang sedang dibahas). DILARANG KERAS menambahkan kalimat pembatas seperti \"Baca juga:\", \"Baca selengkapnya di\", \"Lihat juga:\", atau membuat paragraf khusus untuk menampung link.\n" +
			"   - JANGAN menanam link lain selain yang disediakan di atas."
	}

	channelInstruction := ""
	if resultRow, err := d.getLatestCompletedResult(ctx, campaignID); err == nil && resultRow != nil && resultRow.RawExecutiveBriefMap != nil {
		channelInstruction = channelInstructionFromBrief(resultRow.RawExecutiveBriefMap, mediaType)
	}
	anglePromptBlock := d.loadAnglePromptBlock(ctx, campaignID, mediaType)

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
		profileLine = "Profil bisnis: " + defaultJSONObject(campaign, "business_profile")
	}

	promptLines := []string{
		"Kampanye: " + campaign.strOr("name", ""),
		profileLine,
	}
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
		"Bahan: "+jsonString(orEmptyMap(materials)),
		"Aturan hyperlink media (bila media tidak mengizinkan, gunakan ATURAN PENANAMAN LINK nol): "+jsonString(orEmptySlice(hyperlink)),
		"Kutipan/fakta hanya dari bahan/brief di atas — jangan mengarang angka, nama, atau jabatan.",
		"Aturan meta deskripsi (WAJIB): sertakan meta_description 120-160 karakter — ringkasan menarik isi artikel untuk hasil pencarian, bahasa Indonesia aktif, tanpa tanda kutip ganda.",
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
	if err := json.Unmarshal([]byte(result.Content), &parsed); err == nil {
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
		plainMeta := stripHiddenBlocks(contentHTML)
		plainMeta = reHTMLTag.ReplaceAllString(plainMeta, " ")
		plainMeta = strings.TrimSpace(reWhitespace.ReplaceAllString(plainMeta, " "))
		if len([]rune(plainMeta)) > 160 {
			metaDescription = strings.TrimRight(string([]rune(plainMeta)[:160]), " ") + "…"
		} else {
			metaDescription = plainMeta
		}
		metaDescription = truncateString(metaDescription, 500)
	}

	// Pasca-proses struktur deterministik.
	if mediaType == "blogger" {
		contentHTML = normalizeHeadings(contentHTML)
	} else {
		contentHTML = demoteHeadings(contentHTML)
	}
	if allowsLink {
		contentHTML = enforceBacklinkPolicy(contentHTML, []backlinkSpec{
			{URL: plantPrimary, Anchor: primaryAnchor, Rel: primaryRel, Mandatory: plantPrimary != ""},
			{URL: plantBackup, Anchor: backupAnchor, Rel: strPtr("nofollow sponsored"), Mandatory: false},
		})
	}

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
		next := doc.intOr("current_version", 1) + 1
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
		slug := slugOr(title, "taut-artikel")
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

func orEmptyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func orEmptySlice(s []any) []any {
	if s == nil {
		return []any{}
	}
	return s
}

// mediaName: nama media, fallback ke id.
func (d *Deps) mediaName(ctx context.Context, mediaID string) string {
	row, err := d.queryRow(ctx, "SELECT name FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil || row == nil {
		return mediaID
	}
	return row.strOr("name", mediaID)
}

// jsonStringList: kolom teks berisi JSON array string → []string.
func jsonStringList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var arr []any
	if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
		return nil
	}
	out := []string{}
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func spokespersonList(raw any) []spokesperson {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []spokesperson{}
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := obj["name"].(string)
		role, _ := obj["role"].(string)
		quote, _ := obj["quote"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		out = append(out, spokesperson{Name: strings.TrimSpace(name), Role: strings.TrimSpace(role), Quote: strings.TrimSpace(quote)})
	}
	return out
}

func strPtr(s string) *string { return &s }

// ---------- backlink policy enforcement (deterministik) ----------

type backlinkSpec struct {
	URL       string
	Anchor    string
	Rel       *string
	Mandatory bool
}

var reAnchorPair = regexp.MustCompile(`(?is)<a\b[^>]*>.*?</a>`)
var reTagSplit = regexp.MustCompile(`(<[^>]*>)`)
var reOpenA = regexp.MustCompile(`(?is)^<a[\s>]`)
var reCloseA = regexp.MustCompile(`(?is)^</a`)
var reHTTPScheme = regexp.MustCompile(`(?i)^https?://`)

// stripHyperlinks: buang hyperlink tertanam di HTML (media tidak izinkan).
func stripHyperlinks(html, targetURL string) string {
	out := html
	if targetURL != "" {
		re := regexp.MustCompile(`(?is)<a\b[^>]*href=["']` + regexp.QuoteMeta(targetURL) + `["'][^>]*>(.*?)</a>`)
		out = re.ReplaceAllString(out, "$1")
	}
	out = reAnchorPair.ReplaceAllString(out, "$1")
	return out
}

func escapeHTMLText(v string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(v)
}

// mapHTMLTextNodes: terapkan fn hanya pada text node (di luar tag; di dalam <a> dilewati).
// Tokenizer manual di atas FindAllStringIndex: Regexp.Split di Go membuang
// capture group-nya (berbeda dari String.split di JS), jadi Split di sini
// dulu menghapus seluruh tag HTML dari artikel.
func mapHTMLTextNodes(html string, fn func(string) string) string {
	var out strings.Builder
	last := 0
	inA := false
	for _, m := range reTagSplit.FindAllStringIndex(html, -1) {
		text := html[last:m[0]]
		if !inA {
			text = fn(text)
		}
		out.WriteString(text)
		tag := html[m[0]:m[1]]
		if reOpenA.MatchString(tag) {
			inA = true
		} else if reCloseA.MatchString(tag) {
			inA = false
		}
		out.WriteString(tag)
		last = m[1]
	}
	tail := html[last:]
	if !inA {
		tail = fn(tail)
	}
	out.WriteString(tail)
	return out.String()
}

// weaveContextualLink: sisipkan link kontekstual dengan membungkus frasa anchor
// yang sudah ada di artikel. Return HTML baru, atau "" bila tak ada konteks cocok.
func weaveContextualLink(html, url, anchor string, rel *string) string {
	attr := ""
	if rel != nil && *rel != "" {
		attr = ` rel="` + *rel + `"`
	}
	wrap := func(m string) string { return `<a href="` + url + `"` + attr + `>` + m + `</a>` }

	candidates := []string{anchor}
	words := []string{}
	for _, w := range strings.Fields(anchor) {
		if len([]rune(w)) > 4 && isAlphaPhrase(w) {
			words = append(words, w)
		}
	}
	// Urutkan kata terpanjang lebih dulu (parity sort((a,b)=>b.length-a.length)).
	for i := 1; i < len(words); i++ {
		for j := i; j > 0 && len(words[j]) > len(words[j-1]); j-- {
			words[j], words[j-1] = words[j-1], words[j]
		}
	}
	candidates = append(candidates, words...)

	for _, phrase := range candidates {
		trimmed := strings.TrimSpace(phrase)
		if trimmed == "" {
			continue
		}
		re, err := regexp.Compile(`(?i)(^|[^\w@./-])(` + regexp.QuoteMeta(trimmed) + `)([\s),.;:!?]|$)`)
		if err != nil {
			continue
		}
		replaced := false
		out := mapHTMLTextNodes(html, func(text string) string {
			if replaced {
				return text
			}
			if !re.MatchString(text) {
				return text
			}
			replaced = true
			// Bungkus frasa (sub[2]); boundary kiri/kanan tetap di luar <a>.
			return re.ReplaceAllStringFunc(text, func(m string) string {
				sub := re.FindStringSubmatch(m)
				if sub == nil {
					return m
				}
				return sub[1] + wrap(sub[2]) + sub[3]
			})
		})
		if replaced {
			return out
		}
	}
	return ""
}

func isAlphaPhrase(w string) bool {
	if w == "" {
		return false
	}
	first := w[0]
	if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
		return false
	}
	for i := 0; i < len(w); i++ {
		c := w[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '\'', c == '-', c == ' ', c == '_':
		default:
			return false
		}
	}
	return true
}

// enforceBacklinkPolicy: penegak kebijakan link (deterministik).
func enforceBacklinkPolicy(html string, links []backlinkSpec) string {
	out := html
	for _, spec := range links {
		if spec.URL == "" || spec.Anchor == "" {
			continue
		}
		bare := reHTTPScheme.ReplaceAllString(spec.URL, "")
		// a) <a href="url">URL telanjang</a> → isi diganti anchor text.
		selfRe := regexp.MustCompile(`(?i)(<a\b[^>]*href=["']` + regexp.QuoteMeta(spec.URL) + `["'][^>]*>)\s*(?:https?://)?` + regexp.QuoteMeta(bare) + `\s*(</a>)`)
		out = selfRe.ReplaceAllString(out, "${1}"+escapeHTMLText(spec.Anchor)+"${2}")
		// b) URL telanjang sebagai teks (di luar <a>) → bungkus jadi link.
		nakedRe := regexp.MustCompile(`(?i)(^|[^\w@./-])(?:https?://)?` + regexp.QuoteMeta(bare) + `([\s),.;:!?]|$)`)
		attr := ""
		if spec.Rel != nil && *spec.Rel != "" {
			attr = ` rel="` + *spec.Rel + `"`
		}
		out = mapHTMLTextNodes(out, func(text string) string {
			return nakedRe.ReplaceAllStringFunc(text, func(m string) string {
				sub := nakedRe.FindStringSubmatch(m)
				if sub == nil {
					return m
				}
				return sub[1] + `<a href="` + spec.URL + `"` + attr + `>` + escapeHTMLText(spec.Anchor) + `</a>` + sub[2]
			})
		})
	}

	// c) Tautan utama wajib ada.
	if len(links) > 0 {
		primary := links[0]
		if primary.Mandatory && primary.URL != "" && primary.Anchor != "" {
			hasPrimary := regexp.MustCompile(`(?i)<a\b[^>]*href=["']` + regexp.QuoteMeta(primary.URL) + `["']`).MatchString(out)
			if !hasPrimary {
				woven := weaveContextualLink(out, primary.URL, primary.Anchor, primary.Rel)
				if woven != "" {
					out = woven
				} else {
					attr := ""
					if primary.Rel != nil && *primary.Rel != "" {
						attr = ` rel="` + *primary.Rel + `"`
					}
					sentence := ` Sebagai referensi, pembahasan mendalam tentang <a href="` + primary.URL + `"` + attr + `>` +
						escapeHTMLText(primary.Anchor) + `</a> tersedia pada situs resminya.`
					lastP := strings.LastIndex(out, "</p>")
					if lastP != -1 {
						out = out[:lastP] + sentence + "</p>" + out[lastP+4:]
					} else {
						out += "<p>" + strings.TrimSpace(sentence) + "</p>"
					}
				}
			}
		}
	}
	return out
}
