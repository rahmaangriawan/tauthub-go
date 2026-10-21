package brandflow

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// taut_writer.go: port lib/taut/agents.ts bagian Writer + schedule +
// lib/taut/profiler/angle-fields + lib/tracking/short-link (untuk writer-links).

type writerItemResult struct {
	MediaID    string `json:"media_id"`
	MediaType  string `json:"media_type"`
	Status     string `json:"status"`
	DocumentID any    `json:"document_id"`
	Title      string `json:"title"`
	// Rubrik yang dipilih AI dari daftar kategori media (lihat matchAllowedCategory);
	// kosong bila media tidak punya daftar kategori.
	Category string `json:"category"`
	// Gambar isi yang dibuat otomatis dari instruksi klien + biayanya (transparansi).
	ImagesGenerated   int `json:"images_generated"`
	ImagesCostCredits int `json:"images_cost_credits"`
}

// tautWriterLink: satu pasangan anchor + URL dari modal Pengelola Naskah.
type tautWriterLink struct {
	Anchor string
	URL    string
}

// tautWriterLinkInput: bentuk mentah dari body JSON {anchor_text, url}.
type tautWriterLinkInput struct {
	AnchorText string `json:"anchor_text"`
	URL        string `json:"url"`
}

// tautWriterOptions: arahan opsional dari klien saat generate satu artikel.
type tautWriterOptions struct {
	Topic string
	Links []tautWriterLink
	// ArticleType (opsional): kode jenis artikel (taut_article_types.code) dari
	// toggle di modal Pengelola Naskah. Bila prompt aktifnya ada, dipakai sebagai
	// system prompt pengganti The Writer. Kosong = prompt agent apa adanya.
	ArticleType string
	AngleSlug   *string
	// Proper (opsional): isian mode "Yang Proper" — strict grounding ke fakta
	// terkunci / Brief Copilot / kutipan resmi. Nil vs isian kosong = gaya
	// Sederhana (perilaku lama).
	Proper *tautProperInput
	// StreamHTML (opsional): dipanggil dengan TAMBAHAN html parsial setiap kali
	// LLM menulis lebih banyak isi — untuk streaming SSE ke editor. Nilai masih
	// mentah (belum lewat finalizeTautArticleHTML); finalize hanya di akhir.
	StreamHTML func(partial string)
}

// normalizeTautWriterLinks: validasi + rapikan tautan dari klien. Maksimal 10
// baris; baris kosong diabaikan; tiap baris wajib pasangan anchor + URL
// http/https (menolak URL tanpa anchor, anchor tanpa URL, dan URL non-http).
func normalizeTautWriterLinks(raw []tautWriterLinkInput) ([]tautWriterLink, error) {
	if len(raw) > 10 {
		return nil, errUnprocessable("Maksimal 10 tautan.")
	}
	out := make([]tautWriterLink, 0, len(raw))
	for _, item := range raw {
		anchor := strings.TrimSpace(item.AnchorText)
		rawURL := strings.TrimSpace(item.URL)
		if anchor == "" && rawURL == "" {
			continue
		}
		if rawURL == "" {
			return nil, errUnprocessable("URL wajib diisi bila anchor diisi.")
		}
		if !isHTTPURL(rawURL) || len(rawURL) > 2000 {
			return nil, errUnprocessable("URL tautan tidak valid (wajib http/https).")
		}
		if anchor == "" {
			return nil, errUnprocessable("Anchor teks wajib diisi bila URL diisi.")
		}
		if utf8.RuneCountInString(anchor) > 120 {
			return nil, errUnprocessable("Anchor teks maksimal 120 karakter.")
		}
		out = append(out, tautWriterLink{Anchor: anchor, URL: rawURL})
	}
	return out, nil
}

// runTautWriter: generate artikel untuk SEMUA media terpilih (status='selected').
// Paralel terbatas (4 pekerja): satu generate adalah panggilan LLM 20-35 detik,
// serial membuat kampanye 4 media menunggu hampir 2 menit.
func (d *Deps) runTautWriter(ctx context.Context, campaignID, brandID, userID string) ([]writerItemResult, error) {
	items, err := d.selectedTautMedia(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	results := make([]writerItemResult, len(items))
	errs := make([]error, len(items))
	workers := len(items)
	if workers > 4 {
		workers = 4
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, item := range items {
		wg.Add(1)
		go func(i int, item selectedTautMediaRow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result, gErr := d.generateTautWriterArticle(ctx, campaignID, brandID, userID, item.MediaID)
			results[i], errs[i] = result, gErr
		}(i, item)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return nil, e
		}
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

// tautWordCountRe: frasa "N kata"/"N words" di topik/instruksi klien.
var tautWordCountRe = regexp.MustCompile(`(?i)(\d{2,5})\s*(?:kata|words?|word)\b`)

// tautWordRangeRe: rentang eksplisit "800-900 kata", "800–1000 kata",
// "800 sampai 900 kata".
var tautWordRangeRe = regexp.MustCompile(`(?i)(\d{2,5})\s*(?:-|–|—|sampai|s/d|hingga)\s*(\d{2,5})\s*(?:kata|words?|word)\b`)

// tautWordCapCueRe: kata pembatas ATAS. "maksimal 2500 kata" bukan target
// panjang, jadi angkanya tidak dipakai memaksa artikel jadi 2500 kata.
var tautWordCapCueRe = regexp.MustCompile(`(?i)\b(maksimal|maks|max|maksimum|paling banyak|tidak lebih dari|kurang dari)\b`)

// tautWordMinCueRe: kata pembatas BAWAH. "minimal 2500 kata" berarti jendela
// terbuka ke atas (boleh lebih panjang), bukan target sempit di 2500.
var tautWordMinCueRe = regexp.MustCompile(`(?i)\b(minimal|minimum|min|paling sedikit|setidaknya)\b`)

// wordTarget: jendela panjang artikel yang diminta klien. Exact > 0 bila
// permintaannya satu angka ("800 kata") sehingga jendelanya ±10% di sekitarnya;
// untuk "minimal N kata" jendela terbuka ke atas; untuk rentang eksplisit
// dipakai apa adanya.
type wordTarget struct {
	Min   int
	Max   int
	Exact int
}

func (t wordTarget) accepts(words int) bool {
	return words >= t.Min && words <= t.Max
}

// describe: teks rentang untuk prompt/koreksi, mis. "tepat 800 kata (720–880 kata)".
func (t wordTarget) describe() string {
	if t.Exact > 0 {
		return itoa(t.Exact) + " kata (jendela " + itoa(t.Min) + "–" + itoa(t.Max) + " kata)"
	}
	return itoa(t.Min) + "–" + itoa(t.Max) + " kata"
}

// requestedWordTarget: jendela jumlah kata yang diminta klien dari kolom
// konteks/topik (mis. "minimal 2500 kata", "800 kata", "800-900 kata").
// Angka yang didahului kata pembatas atas ("maksimal N kata") diabaikan.
// Bila ada beberapa angka, permintaan terbesar yang dipakai.
func requestedWordTarget(topic string) (wordTarget, bool) {
	clamp := func(n int) (int, bool) {
		if n < 150 || n > 20000 {
			return 0, false
		}
		return n, true
	}
	if m := tautWordRangeRe.FindStringSubmatch(topic); m != nil {
		a, errA := strconv.Atoi(m[1])
		b, errB := strconv.Atoi(m[2])
		if errA == nil && errB == nil {
			if a > b {
				a, b = b, a
			}
			if lo, okLo := clamp(a); okLo {
				if hi, okHi := clamp(b); okHi {
					return wordTarget{Min: lo, Max: hi}, true
				}
			}
		}
	}
	best := wordTarget{}
	bestN := 0
	for _, loc := range tautWordCountRe.FindAllStringSubmatchIndex(topic, -1) {
		n, err := strconv.Atoi(topic[loc[2]:loc[3]])
		if err != nil || n < 150 || n > 20000 {
			continue
		}
		start := loc[0] - 24
		if start < 0 {
			start = 0
		}
		cue := topic[start:loc[0]]
		if tautWordCapCueRe.MatchString(cue) {
			continue
		}
		if n <= bestN {
			continue
		}
		bestN = n
		if tautWordMinCueRe.MatchString(cue) {
			// Batas bawah eksplisit: jendela terbuka ke atas (+25%, minimal +150).
			headroom := n / 4
			if headroom < 150 {
				headroom = 150
			}
			best = wordTarget{Min: n, Max: n + headroom}
			continue
		}
		// Angka tunggal ("800 kata", "sekitar 800 kata"): target di 800 dengan
		// toleransi ±10% — simetris, supaya tidak bias ke atas.
		tol := n / 10
		if tol < 30 {
			tol = 30
		}
		best = wordTarget{Min: n - tol, Max: n + tol, Exact: n}
	}
	if bestN == 0 {
		return wordTarget{}, false
	}
	return best, true
}

// clientWordRule: aturan prompt untuk target kata eksplisit dari klien.
func clientWordRule(t wordTarget) string {
	if t.Exact > 0 {
		return "Aturan panjang artikel (WAJIB — PERMINTAAN EKSPLISIT KLIEN, PRIORITAS UTAMA): tulis artikel tepat " +
			itoa(t.Exact) + " kata; panjang akhir yang diterima " + itoa(t.Min) + "–" + itoa(t.Max) +
			" kata (±10%). Perkirakan jumlah kata saat menulis, jangan menutup artikel di bawah " + itoa(t.Min) +
			" kata, dan jangan melewati " + itoa(t.Max) + " kata. Aturan ini MENANG atas aturan panjang media mana pun."
	}
	return "Aturan panjang artikel (WAJIB — PERMINTAAN EKSPLISIT KLIEN, PRIORITAS UTAMA): tulis artikel sepanjang " +
		itoa(t.Min) + "–" + itoa(t.Max) + " kata. Jangan kurang dari " + itoa(t.Min) + " kata dan jangan lebih dari " +
		itoa(t.Max) + " kata. Aturan ini MENANG atas aturan panjang media mana pun."
}

// correctionPrompt: instruksi tulis ulang saat draf pertama meleset dari jendela.
func (t wordTarget) correction(actual int) string {
	return "KOREKSI PANJANG (WAJIB): draf sebelumnya " + itoa(actual) + " kata, sedangkan permintaan klien " +
		t.describe() + ". Tulis ULANG seluruh artikel dari awal dengan panjang dalam jendela itu, sambil mempertahankan " +
		"topik, fakta, sudut pandang, dan seluruh aturan link/format yang sama."
}

// enforceClientWordTarget: penegakan panjang artikel untuk permintaan eksplisit
// klien. Prompt saja tidak cukup — model kerap meleset (kasus nyata: klien minta
// 800 kata, hasil 1000). Bila draf di luar jendela, artikel ditulis ulang SEKALI
// dengan koreksi panjang eksplisit; bila tulis ulang gagal/masih meleset, draf
// pertama yang dipakai (generate tidak pernah digagalkan oleh langkah ini).
func (d *Deps) enforceClientWordTarget(ctx context.Context, campaignID, mediaID, prompt string, target wordTarget, html, title, metaDescription string) (string, string, string) {
	actual := countWords(html)
	if target.accepts(actual) {
		return html, title, metaDescription
	}
	slog.Info("[taut-writer] jumlah kata di luar jendela — tulis ulang",
		"campaign", campaignID, "media_id", mediaID, "actual", actual, "min", target.Min, "max", target.Max)
	retry, err := d.runTautAgent(ctx, "writer", campaignID, prompt+"\n"+target.correction(actual), &mediaID, true)
	if err != nil || retry == nil {
		slog.Warn("[taut-writer] tulis ulang koreksi panjang gagal", "media_id", mediaID, "err", err)
		return html, title, metaDescription
	}
	var parsed struct {
		Title           string `json:"title"`
		ContentHTML     string `json:"content_html"`
		MetaDescription string `json:"meta_description"`
	}
	if json.Unmarshal([]byte(retry.Content), &parsed) != nil || strings.TrimSpace(parsed.ContentHTML) == "" {
		slog.Warn("[taut-writer] hasil tulis ulang tidak bisa diparsing", "media_id", mediaID)
		return html, title, metaDescription
	}
	newHTML := truncateString(parsed.ContentHTML, 200000)
	if t := truncateString(parsed.Title, 300); t != "" {
		title = t
	}
	if m := truncateString(strings.TrimSpace(parsed.MetaDescription), 500); m != "" {
		metaDescription = m
	}
	slog.Info("[taut-writer] hasil tulis ulang koreksi panjang",
		"media_id", mediaID, "before", actual, "after", countWords(newHTML))
	return newHTML, title, metaDescription
}

// pruneContentVersions: sisakan HANYA versi terbaru sebuah dokumen. Editor
// Taut Pintar tidak punya fitur pemulihan versi, jadi versi lama yang tertimpa
// (regenerasi AI maupun edit manual) tidak perlu menumpuk di database — semua
// pembaca konten selalu mengambil content_documents.current_version.
func (d *Deps) pruneContentVersions(ctx context.Context, documentID string, keepVersion int) error {
	return d.exec(ctx,
		"DELETE FROM content_versions WHERE document_id = $1 AND version <> $2", documentID, keepVersion)
}

// generateTautWriterArticle: generate artikel untuk SATU media terpilih.
// Idempoten: media yang sudah punya dokumen = regenerasi (tambah versi).
// opts opsional: arahan topik/konteks + daftar tautan tambahan dari klien
// (modal Pengelola Naskah). Kosong = perilaku lama (topik murni dari brief).
func (d *Deps) generateTautWriterArticle(ctx context.Context, campaignID, brandID, userID, mediaID string, opts ...tautWriterOptions) (writerItemResult, error) {
	clientTopic := ""
	articleType := ""
	var requestedAngle *string
	var proper *tautProperInput
	customLinks := []tautWriterLink{}
	var streamFn func(string)
	if len(opts) > 0 {
		clientTopic = strings.TrimSpace(opts[0].Topic)
		articleType = strings.TrimSpace(opts[0].ArticleType)
		requestedAngle = opts[0].AngleSlug
		proper = opts[0].Proper
		customLinks = opts[0].Links
		streamFn = opts[0].StreamHTML
	}
	campaign, err := d.loadTautCampaignRow(ctx, campaignID, brandID)
	if err != nil {
		return writerItemResult{}, err
	}
	item, err := d.queryRow(ctx, `
		SELECT tcm.id, tcm.media_id, tcm.article_document_id, tcm.regen_count, tcm.angle_slug,
		       m.media_type, m.category, m.website_url, m.allow_hyperlink, m.max_links,
		       m.label_sponsor, m.word_count_mode, m.topik_terlarang, m.bahasa_artikel,
		       COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = m.id), m.category) AS categories_csv
		FROM taut_campaign_media tcm JOIN media m ON m.id = tcm.media_id
		WHERE tcm.campaign_id = $1 AND tcm.media_id = $2 AND tcm.status = 'selected' LIMIT 1`,
		campaignID, mediaID)
	if err != nil {
		return writerItemResult{}, err
	}
	if item == nil {
		return writerItemResult{}, errStatus(500, "Media tidak terpilih di kampanye ini.")
	}
	// Rubrik media yang diizinkan mitra (urut nama). Dipakai dua hal: daftar
	// pilihan untuk AI, dan cadangan bila model tidak memilih dengan yakin —
	// supaya artikel hasil generate tidak pernah tertahan "Kategori belum
	// dipilih" di popover kesiapan terbit.
	allowedCategories := catutil.Split(item.strOr("categories_csv", ""))
	mediaType := resolveTautMediaType(item.strOr("media_type", ""), campaign.strOr("media_type", ""))
	angle, angleOverride, err := d.resolveTautWriterAngle(ctx, campaignID, mediaType, item.strOr("angle_slug", ""), requestedAngle)
	if err != nil {
		return writerItemResult{}, err
	}
	if angleOverride && articleType != "" {
		return writerItemResult{}, errUnprocessable("Kirim angle_slug atau article_type, bukan keduanya.")
	}
	materials := asMapValue(campaign.jsonField("editorial_standard_snapshot"))
	hyperlink := asSliceValue(campaign.jsonField("hyperlink_config"))
	briefContext, err := d.loadTautBriefContext(ctx, campaignID, angleOverride)
	if err != nil {
		return writerItemResult{}, err
	}
	// Regenerasi tanpa topik baru tetap memakai topik tersimpan, supaya prompt
	// penulisan dan pengecekan (yang membaca kolom yang sama) memakai konteks
	// yang sama.
	writerTopic := clientTopic
	if writerTopic == "" {
		writerTopic = d.loadTautWriterTopic(ctx, campaignID, mediaID)
	}
	// Editorial zero-link: instruksi tautan pada topik klien dianggap batal.
	// Skema URL ikut dibuang supaya model tidak menyalin alamat web ke badan
	// berita — topik tersimpan di DB tetap yang asli (dipakai Gatekeeper).
	if articleType == "editorial" {
		writerTopic = stripURLSchemeText(writerTopic)
	}
	// Bahan fakta kanonik: dipakai di prompt penulisan DAN diverifikasi ulang
	// oleh Gatekeeper AI Overview Checker lewat fungsi yang sama, sehingga klaim
	// dari materials/topik/narasumber tidak lagi ditandai "tanpa dasar".
	// Blok angle berita (butuh mediaType) ditambahkan setelah tipe media
	// diresolusi di bawah; di sini masih kosong.
	referenceMaterial, hasFacts := buildTautReferenceMaterial(briefContext, materials, writerTopic, spokespersonListValue(campaign.jsonField("spokepersons")), "")
	// Profiler klien (kalau kampanye memakainya): tambahkan sebagai bahan rujukan
	// terstruktur (identitas brand, gaya, pedoman, dokumen/URL siap, knowledge blok).
	if profilerText := d.loadCampaignProfilerContext(ctx, campaignID, writerTopic); profilerText != "" {
		referenceMaterial = referenceMaterial + "\n\n" + profilerText
		hasFacts = true
	}
	// Mode "Yang Proper": strict grounding ke fakta terkunci/Brief Copilot.
	// Kiriman klien disimpan per media (regenerasi & Gatekeeper memakai isian
	// yang sama), lalu blok grounding bak masuk bahan rujukan Writer.
	if proper == nil {
		proper = d.loadTautProperStored(ctx, item.strOr("id", ""))
	}
	if proper != nil {
		d.saveTautProperFields(ctx, item.strOr("id", ""), proper)
		if properBlock := buildTautProperBlock(proper, d.loadTautProperBrief(ctx, brandID, proper.BriefID)); properBlock != "" {
			referenceMaterial = strings.TrimSpace(referenceMaterial + "\n\n" + properBlock)
			hasFacts = true
		}
	}
	backlinkEnabled, smartVariations := d.loadTautBacklinkSettings(ctx, campaignID)
	smartVariationRule := ""
	if smartVariations {
		smartVariationRule = tautSmartVariationRule
	}

	itemID := item.strOr("id", "")
	// Topik/konteks klien dari modal Pengelola Naskah disimpan per media supaya
	// Gatekeeper (request terpisah) bisa memverifikasi klaim ke konteks yang sama
	// dengan yang dipakai Writer. Non-fatal bila kolom belum dimigrasi.
	if clientTopic != "" && !angleOverride {
		if err := d.exec(ctx, "UPDATE taut_campaign_media SET writer_topic = $1 WHERE id = $2", clientTopic, itemID); err != nil {
			slog.Warn("[taut-writer] gagal menyimpan writer_topic", "media_id", mediaID, "err", truncateString(err.Error(), 200))
		}
	}
	// Pilihan jenis artikel dari modal Pengelola Naskah disimpan per media supaya
	// regenerate & Gatekeeper memakai genre yang sama. Non-fatal bila kolom belum
	// dimigrasi (migrasi 200).
	if articleType != "" && !angleOverride {
		if err := d.exec(ctx, "UPDATE taut_campaign_media SET content_type = $1 WHERE id = $2", articleType, itemID); err != nil {
			slog.Warn("[taut-writer] gagal menyimpan content_type", "media_id", mediaID, "err", truncateString(err.Error(), 200))
		}
	}
	// Prompt system pengganti per jenis artikel (kosong = prompt agent standar).
	articleTypePrompt, _ := d.loadTautArticleTypePrompt(ctx, articleType)
	regenCount := item.intOr("regen_count", 0)
	hasDocument := item.has("article_document_id")
	if hasDocument {
		// Limit mengikuti jatah akun di halaman Admin Users
		// (users.writer_generate_quota) dan dihitung dari TOTAL generate akun,
		// bukan per artikel. Satu artikel boleh diregenerasi berkali-kali
		// selama jatah akun belum habis.
		// Limit hanya berlaku bila admin mengisi jatah akun (writer_generate_quota).
		// NULL = tanpa batas. Jika terisi, dipakai terhadap TOTAL generate akun.
		quota := d.userGenerateQuota(ctx, userID)
		used := d.userGenerateUsed(ctx, brandID)
		if quota > 0 && used >= quota {
			return writerItemResult{}, errStatus(500, "Batas jatah generate akun tercapai: sudah "+
				itoa(used)+"x generate dari kuota "+itoa(quota)+"x. Ubah jatahnya di halaman Admin Users bila perlu.")
		}
		if err := d.chargeCredit(ctx, brandID,
			"taut-writer-regen:"+campaignID+":"+item.strOr("media_id", "")+":"+itoa(regenCount+1),
			d.writerCreditCost(ctx), "Taut Pintar — regenerasi artikel AI", userID); err != nil {
			return writerItemResult{}, err
		}
	}

	name := d.mediaNameFor(ctx, item.strOr("media_id", ""))
	slog.Info("[taut-writer] media type resolved", "campaign", campaignID, "media_id", mediaID, "media_type", mediaType)
	wordRule := buildWordRule(item.strOr("word_count_mode", ""), mediaType)

	// Jumlah kata EKSPLISIT dari klien (mis. "minimal 2500 kata" / "800 kata" di
	// kolom topik) MENANG atas aturan panjang bawaan media. Tanpa ini, model
	// mengikuti batas media (mis. "minimal 300 kata") lalu berhenti jauh dari
	// permintaan — kasus nyata: minta 2500 kata, hasil hanya ~1873.
	// Mode Proper: target kata juga dicek dari Instruksi Tambahan klien.
	wordsSource := writerTopic
	if wordsSource == "" && proper != nil {
		wordsSource = proper.Fokus
	}
	wordTarget, hasClientWords := requestedWordTarget(wordsSource)
	if hasClientWords {
		wordRule = clientWordRule(wordTarget)
	}

	structureRules := []string{
		"Aturan gaya mainstream (WAJIB): tulis sebagai artikel berita dengan kaidah 5W+1H — paragraf pembuka (lead) menjawab siapa, apa, kapan, di mana, mengapa, dan bagaimana secara ringkas.",
		"Aturan format mainstream (WAJIB): JANGAN gunakan heading/subjudul (<h2>/<h3>) dan JANGAN memecah isi menjadi daftar poin (<ul>/<ol>) — seluruh isi adalah paragraf mengalir gaya berita, tanpa markdown.",
		"Aturan paragraf mainstream (WAJIB): pecah isi menjadi BANYAK paragraf <p> pendek (maksimal 3-5 kalimat per <p>) — DILARANG menulis satu gundukan teks panjang dalam satu <p>.",
	}
	if mediaType == "blogger" {
		structureRules = []string{
			"Aturan heading (WAJIB): tulis judul bagian sebagai tag HTML <h2> atau <h3> (dimulai dari H2, jangan pernah H1); jangan tinggalkan judul bagian sebagai teks biasa.",
			"Aturan daftar (WAJIB): poin, fitur, langkah, atau hal berurutan WAJIB ditulis sebagai daftar HTML <ul><li>…</li></ul> (tanpa urutan) atau <ol><li>…</li></ol> (berurutan) — jangan tulis daftar sebagai paragraf biasa atau kalimat berawal tanda '-' / '1.'.",
		}
	}
	precedenceBlock := "PRIORITAS ATURAN (WAJIB): (1) Sudut/angle pilihan klien menentukan SUDUT PANDANG dan isi bahasan. (2) Gaya & format media menentukan BENTUK tulisan (panjang, struktur, gaya bahasa). Bila keduanya berbenturan, angle menang untuk sudut pandang, gaya media menang untuk format."
	if mediaType == "blogger" {
		precedenceBlock += " Terapkan juga: blogger -> artikel evergreen dengan heading/subjudul yang mengalir dan bahasa lebih santai."
	} else {
		precedenceBlock += " Terapkan juga: mainstream -> lead 5W+1H dan paragraf mengalir gaya berita tanpa heading."
	}
	if hasClientWords {
		precedenceBlock += " PENGECUALIAN: jumlah kata eksplisit dari klien (lihat aturan panjang di atas) MENANG atas panjang bawaan media."
	}
	tidyHTMLBlock := "ATURAN HTML RAPI (WAJIB): setiap paragraf dibungkus <p> yang utuh (tidak ada paragraf terpotong); JANGAN menulis heading nyasar atau heading kosong dan JANGAN memakai <hr> atau garis dekoratif; JANGAN meninggalkan tag kosong (mis. <p></p>, <h2></h2>); JANGAN menulis teks yang hanya berisi URL telanjang; heading hanya dipakai bila memang diizinkan tipe media (blogger) dan hierarkinya berurutan (H2 lalu H3, tidak melompat); buang atribut/style inline yang tidak perlu dan JANGAN menambah wrapper <div>. Kutipan narasumber ditulis sebagai <blockquote><p>...</p></blockquote>. Bila memakai tabel (hanya media yang mengizinkan daftar/tabel), tulis HTML standar <table><thead><tr><th>...</th></tr></thead><tbody><tr><td>...</td></tr></tbody></table> TANPA atribut style/border/width inline — tampilan diatur editor."
	// Rubrik AI Overview (Gatekeeper QC) diambil dari SATU sumber yang sama
	// dengan penilai (internal/ai): indikator, judul, dan ambang kelulusan tidak
	// bisa lagi menyimpang antara prompt penulisan dan prompt pengecekan.
	aiOverviewRules := ai.AIOverviewWriterRubric(d.aiOverviewSettings(ctx), mediaType)

	allowsLink := item.boolOr("allow_hyperlink", true)
	if item.has("max_links") {
		allowsLink = int(item.numOr("max_links")) > 0
	}
	// Editorial = standar redaksi berita: ZERO-LINK. Pilihan jenis artikel ini
	// menang atas aturan hyperlink media maupun backlink kampanye — finalize
	// nanti akan mengupas <a> apa pun yang lolos (stripAllLinks).
	if articleType == "editorial" {
		allowsLink = false
	}
	sponsoredMedia := item.strOr("label_sponsor", "tidak_ada") == "ada_sponsor"
	// Advertorial = sponsored content: paksa rel="sponsored" pada setiap tautan
	// yang ditanam (backlink kampanye maupun tautan yang diminta klien), sejalan
	// dengan master prompt advertorial. Tautan tetap hanya ditanam bila ada.
	if articleType == "advertorial" {
		sponsoredMedia = true
	}
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
	// Editorial tidak membawa konteks backlink sama sekali (zero-link).
	if primaryURL != "" && articleType != "editorial" {
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
			"   - JANGAN menanam link lain selain tautan kampanye di atas maupun tautan yang diminta klien di blok TAUTAN DARI KLIEN."
	} else if allowsLink {
		// Media mengizinkan hyperlink tapi kampanye tidak mengatur backlink:
		// keputusan tautan menempel pada permintaan klien (blok TAUTAN DARI
		// KLIEN di bawah), bukan larangan media — tanpa cabang ini model
		// menuruti larangan mutlak dan menolak menanam anchor milik klien.
		linkRule = "ATURAN PENANAMAN LINK: Media ini mengizinkan hyperlink. Tidak ada backlink kampanye untuk artikel ini — tanam HANYA tautan yang diminta klien pada blok TAUTAN DARI KLIEN di bawah (tidak ada tautan lain).\n" +
			"   - SEMUA tautan WAJIB berupa <a href=\"URL\">anchor text</a>. DILARANG menulis URL telanjang sebagai teks artikel.\n" +
			"   - Tempatkan tautan secara natural dan semantik di dalam kalimat yang relevan (bukan berdampingan, bukan daftar link).\n" +
			"   - TEKNIK KONTEKSTUAL (WAJIB): tanamkan link dengan mengubah frasa yang SUDAH ada di kalimat menjadi tautan (sebutan brand/layanan/topik yang sedang dibahas). DILARANG menambahkan kalimat pembatas seperti \"Baca juga:\"."
	}

	// Tautan tambahan dari klien (modal Pengelola Naskah): mandat level sistem
	// yang menang atas ketentuan link di prompt angle mana pun, sehingga klien
	// tidak perlu menyentuh prompt angle untuk mendapatkan tautannya. Format
	// baris "- [anchor] -> URL" juga dipakai pasca-proses membungkus URL
	// telanjang menjadi <a>.
	customLinkBlock := ""
	if allowsLink && len(customLinks) > 0 {
		lines := make([]string, 0, len(customLinks))
		for _, l := range customLinks {
			lines = append(lines, "- ["+l.Anchor+"] -> "+l.URL)
		}
		customLinkBlock = "[TAUTAN DARI KLIEN — MANDAT SISTEM, MENANG ATAS ATURAN TAUTAN DI PROMPT ANGLE MANA PUN]\n" +
			"Klien secara eksplisit meminta tautan berikut disertakan dalam artikel. Sisipkan masing-masing TEPAT 1x secara natural dan semantik pada bagian yang relevan, dibungkus <a href=\"URL\">anchor text</a>, dan DILARANG menulis URL telanjang:\n" +
			strings.Join(lines, "\n")
	}

	channelInstruction := ""
	anglePromptBlock := ""
	if angleOverride {
		// Keep the complete admin instructions; the brief block is only a summary.
		anglePromptBlock = "[ANGLE PILIHAN KLIEN - " + mediaType + "/" + angle.Slug + "]\n" + angle.Prompt
	} else {
		channelInstruction = d.loadTautChannelInstruction(ctx, campaignID, mediaType)
		// Prompt angle penuh SELALU ikut — bukan hanya saat brief kosong.
		// Aturan editorial verbatim di dalamnya (blokir gaya "disarankan...",
		// kewajiban kutipan bernama, dst.) harus terbaca The Writer di setiap
		// generate; ringkasan brief saja tidak cukup menjaga aturan itu utuh.
		anglePromptBlock = d.loadTautChosenAnglePrompt(ctx, campaignID, mediaType)
	}
	angleBriefBlock, angleHasSources := tautAngleBriefBlock(angle)
	if angleBriefBlock != "" {
		referenceMaterial = strings.TrimSpace(referenceMaterial + "\n\n[ANGLE BERITA]\n" + angleBriefBlock)
	}
	if angleHasSources {
		hasFacts = true
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
	if writerTopic != "" {
		promptLines = append(promptLines,
			"KONTEKS UTAMA DARI KLIEN (WAJIB DIPATUHI): "+writerTopic,
			"Seluruh artikel (judul, lead, isi, penutup) WAJIB mengikuti konteks di atas. Jangan menambah fakta, angka, nama, atau klaim yang tidak ada di konteks tersebut, dan jangan mengganti topik.",
		)
		// Editorial zero-link: klien boleh saja menulis "sertakan backlink/link
		// ke ..." di kolom topik — untuk Editorial itu dianggap tidak pernah
		// ditulis. Tanpa blok ini model menurutinya dan menuliskan tautan/URL.
		if articleType == "editorial" {
			promptLines = append(promptLines,
				"ZERO-LINK OVERRIDE (WAJIB — JENIS ARTIKEL EDITORIAL): Abaikan SETIAP bagian konteks klien di atas yang meminta menyematkan tautan, link, backlink, anchor, hyperlink, atau alamat web — bagian itu dianggap tidak pernah ditulis dan tidak boleh memengaruhi naskah. Naskah Editorial TIDAK boleh memuat tautan aktif maupun alamat web di badan berita; sebutkan nama brand/layanan secara natural tanpa alamat web. Aturan ini MENANG atas instruksi tautan apa pun dari klien.",
			)
		}
	}
	promptLines = append(promptLines, precedenceBlock)
	promptLines = append(promptLines, structureRules...)
	// Gambar isi (kondisional): klien boleh minta gambar lewat topik/konteks.
	// Writer hanya menaruh PENANDA; gambar aslinya dibuat sistem sepulang generate
	// (lihat resolveTautInlineImages). Cover tidak lewat jalur ini.
	imageMarkerBlock := "ATURAN GAMBAR ISI (KONDISIONAL): Bila instruksi pengguna meminta gambar, foto, atau ilustrasi DI DALAM artikel, sisipkan penanda persis berformat [[IMAGE: deskripsi singkat gambar]] pada posisi yang tepat di content_html (di antara paragraf, setelah bagian yang relevan). Sistem akan mengganti penanda itu dengan gambar asli. JANGAN membuat penanda untuk gambar utama/cover — cover diatur terpisah oleh klien. Bila pengguna TIDAK meminta gambar, JANGAN menulis penanda apa pun."
	promptLines = append(promptLines, aiOverviewRules...)
	promptLines = append(promptLines, tidyHTMLBlock, imageMarkerBlock)
	// Guard anti-halusinasi: tanpa bahan rujukan, model cenderung mengarang nama
	// tokoh/perusahaan/angka agar artikel terasa spesifik (dan justru dipuji
	// "kepadatan entitas" oleh QC). Larang eksplisit saat bahan kosong. hasFacts
	// dihitung dari SEMUA sumber yang dipakai Writer, bukan hanya brief Profiler.
	if !hasFacts {
		promptLines = append(promptLines,
			"BAHAN KOSONG (PENTING): tidak ada brief/bahan rujukan untuk kampanye ini. JANGAN menyebut nama orang, nama perusahaan/lembaga, studi kasus, angka statistik, persentase, harga, tanggal, atau kutipan narasumber yang spesifik — tulis penjelasan generik tanpa mengarang fakta. Bila butuh contoh, pakai kalimat umum (mis. \"umumnya\", \"banyak pelaku industri\") TANPA angka atau nama.")
	}

	// Rubrik: AI memilih dari daftar kategori media (bukan diketik klien).
	// Daftar hanya dikirim bila medianya memang punya kategori; aturan cadangan
	// "pilih yang pertama" dipasang di prompt dan ditegakkan lagi di Go saat
	// jawabannya divalidasi (matchAllowedCategory), jadi hasilnya tidak
	// bergantung pada kepatuhan model.
	categoryRule := ""
	schemaLine := "Tulis artikel unik utk media ini. Output JSON valid {title, content_html, meta_description}"
	if len(allowedCategories) > 0 {
		categoryRule = "Aturan rubrik/kategori (WAJIB): pilih SATU rubrik paling relevan dengan isi artikel HANYA dari daftar kategori media berikut (sudah urut): " +
			strings.Join(allowedCategories, " | ") +
			". Tulis di field category persis seperti tertulis di daftar, tanpa menambah atau mengarang rubrik baru. Bila isinya tidak jelas condong ke mana, pilih rubrik pertama pada daftar itu."
		schemaLine += ", category"
	}
	schemaLine += ". Penanda [[IMAGE: ...]] (bila klien minta gambar) ditulis di dalam content_html — JANGAN sebagai field terpisah."

	promptLines = append(promptLines,
		languageBlock,
		identity.String(),
		topikBlock,
		channelInstruction,
		anglePromptBlock,
		angleBriefBlock,
		linkContext,
		linkRule,
		customLinkBlock,
		spokespersonBlock,
		smartVariationRule,
		"Bahan (rujukan fakta yang boleh dipakai; QC AI Overview memverifikasi klaim ke bahan yang sama):\n"+referenceMaterial,
		"Aturan hyperlink media (bila media tidak mengizinkan, gunakan ATURAN PENANAMAN LINK nol): "+jsonStringify(orEmptySliceValue(hyperlink)),
		"Kutipan/fakta hanya dari bahan/brief di atas — jangan mengarang angka, tanggal, atau harga. Soal narasumber: pakai [SPOKESPERSON] bila tersedia; bila brief tidak memuatnya dan [PROMPT ANGLE (dari Admin)] di atas mewajibkan kutipan bernama, penuhi mandat angle itu dengan satu figur strategis yang wajar.",
		"Aturan meta deskripsi (WAJIB): sertakan meta_description 120-160 karakter bahasa Indonesia aktif.",
		categoryRule,
		"Variasi tulisan: gunakan sudut pembuka yang berbeda dari draf sebelumnya; jangan menyalin kalimat pembuka yang sama.",
		"Nonce variasi (abaikan isinya, hanya penanda sesi agar tiap generate berbeda): "+itoa(int(time.Now().UnixNano())),
		"", schemaLine,
	)
	filtered := []string{}
	for _, line := range promptLines {
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	prompt := strings.Join(filtered, "\n")

	// Subagent The Writer per tipe media (migrasi 195). Kalau agent spesifiknya
	// belum dikonfigurasi admin, kembali ke agent "writer" generik supaya gaya
	// artikel produksi tidak berubah tanpa sengaja.
	writerAgent := writerAgentName(mediaType)
	if !d.getAgentConfig(ctx, writerAgent).Configured {
		writerAgent = "writer"
	}

	var result *agentLLMResult
	if streamFn != nil {
		// Streaming: kumpulkan delta JSON mentah, ekstrak content_html parsial,
		// dan kirim TAMBAHANNYA saja ke klien (escape sudah di-decode). Decoder
		// inkremental — dulu tiap delta memindai ulang seluruh akumulasi.
		stream := newPEStreamHTML()
		onDelta := func(delta string) {
			if tail, _ := stream.Write(delta); tail != "" {
				streamFn(tail)
			}
		}
		if articleTypePrompt != "" {
			result, err = d.runTautAgentSystem(ctx, writerAgent, campaignID, prompt, &mediaID, true, articleTypePrompt, onDelta)
		} else {
			result, err = d.runTautAgentStream(ctx, writerAgent, campaignID, prompt, &mediaID, true, onDelta)
		}
	} else {
		if articleTypePrompt != "" {
			result, err = d.runTautAgentSystem(ctx, writerAgent, campaignID, prompt, &mediaID, true, articleTypePrompt, nil)
		} else {
			result, err = d.runTautAgent(ctx, writerAgent, campaignID, prompt, &mediaID, true)
		}
	}
	if err != nil {
		return writerItemResult{}, err
	}
	title := ""
	contentHTML := ""
	metaDescription := ""
	chosenCategory := ""
	var parsed struct {
		Title           string `json:"title"`
		ContentHTML     string `json:"content_html"`
		MetaDescription string `json:"meta_description"`
		Category        string `json:"category"`
	}
	if json.Unmarshal([]byte(result.Content), &parsed) == nil {
		title = truncateString(parsed.Title, 300)
		contentHTML = truncateString(parsed.ContentHTML, 200000)
		metaDescription = truncateString(strings.TrimSpace(parsed.MetaDescription), 500)
		chosenCategory = matchAllowedCategory(parsed.Category, allowedCategories)
	} else {
		title = campaign.strOr("name", "") + " — " + name
		contentHTML = truncateString(result.Content, 200000)
		// Jawaban bukan JSON: pilihan rubrik dari model tidak terbaca, pakai
		// cadangan supaya artikel tetap punya rubrik.
		chosenCategory = matchAllowedCategory("", allowedCategories)
	}
	if strings.TrimSpace(contentHTML) == "" {
		return writerItemResult{MediaID: mediaID, MediaType: mediaType, Status: "generating", DocumentID: nil}, nil
	}
	// Kepastian panjang untuk permintaan klien: prompt saja tidak cukup, draf di
	// luar jendela ditulis ulang SEKALI di sini (sebelum pasca-proses link dsb).
	if hasClientWords {
		contentHTML, title, metaDescription = d.enforceClientWordTarget(ctx, campaignID, mediaID, prompt, wordTarget, contentHTML, title, metaDescription)
	}
	if metaDescription == "" {
		plainMeta := stripHTMLToText(contentHTML)
		if utf8.RuneCountInString(plainMeta) > 160 {
			metaDescription = strings.TrimRight(string([]rune(plainMeta)[:160]), " ") + "…"
		} else {
			metaDescription = plainMeta
		}
		metaDescription = truncateString(metaDescription, 500)
	}
	linkSpecs := []tautBacklinkSpec{
		{URL: plantPrimary, Anchor: primaryAnchor, Rel: primaryRel, Mandatory: plantPrimary != ""},
		{URL: plantBackup, Anchor: backupAnchor, Rel: strPtrValue("nofollow sponsored"), Mandatory: false},
	}
	// Tautan klien ikut dinegosiasikan lewat pasca-proses (non-mandatory): bila
	// AI menulis URL telanjang, URL itu dibungkus jadi <a> dengan anchor klien.
	if allowsLink {
		for _, l := range customLinks {
			var rel *string
			if sponsoredMedia {
				rel = strPtrValue("sponsored")
			}
			linkSpecs = append(linkSpecs, tautBacklinkSpec{URL: l.URL, Anchor: l.Anchor, Rel: rel, Mandatory: false})
		}
	}
	contentHTML = finalizeTautArticleHTML(contentHTML, mediaType, allowsLink, linkSpecs)
	// Editorial zero-link: <a> sudah diupas finalize; URL telanjang yang mungkin
	// ditulis model (mis. karena topik klien menyebut alamat web) dibuat teks
	// polos tanpa skema supaya tidak terasa seperti tautan.
	if articleType == "editorial" {
		contentHTML = stripURLSchemeHTML(contentHTML)
	}

	// Gambar isi otomatis: kalau klien meminta gambar/foto/ilustrasi di dalam
	// artikel, Writer menaruh penanda [[IMAGE: ...]] dan di sini penanda itu
	// diganti gambar AI yang benar-benar dibuat. Gambar utama (cover) TIDAK
	// disentuh — cover tetap manual/upload dari modal. Bila klien tidak meminta
	// gambar, penanda (kalau ada) dibuang tanpa membuat gambar.
	imagesGenerated, imagesCost := 0, 0
	if strings.Contains(strings.ToLower(contentHTML), "[[image") {
		if userRequestedInlineImages(writerTopic) {
			var imgWarnings []string
			contentHTML, imagesGenerated, imagesCost, imgWarnings = d.resolveTautInlineImages(ctx, brandID, campaignID, mediaID, userID, contentHTML, title)
			for _, w := range imgWarnings {
				slog.Warn("[taut-writer] gambar inline", "campaign", campaignID, "media_id", mediaID, "info", w)
			}
		} else {
			contentHTML = tautImageMarkerRe.ReplaceAllString(contentHTML, "")
		}
	}

	isRegen := hasDocument
	docID := ""
	if isRegen {
		docID = item.strOr("article_document_id", "")
	} else {
		docID = uuid.NewString()
	}
	jobID := uuid.NewString()
	err = d.withTx(ctx, func(tx pgx.Tx) error {
		if isRegen {
			var currentVersion int
			if err := tx.QueryRow(ctx, "SELECT current_version FROM content_documents WHERE id = $1 FOR UPDATE", docID).Scan(&currentVersion); err != nil {
				return err
			}
			next := currentVersion + 1
			if _, err := tx.Exec(ctx,
				"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, $3, $4, 'html', 'ai_generated', 'Taut Pintar Writer - regenerasi', $5)",
				uuid.NewString(), docID, next, contentHTML, userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				"UPDATE content_documents SET current_version = $1, meta_description = $2, title = $3, slug = $4, updated_at = NOW() WHERE id = $5",
				next, metaDescription, title, slugOrValue(title, "taut-artikel"), docID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "DELETE FROM content_versions WHERE document_id = $1 AND version <> $2", docID, next); err != nil {
				return err
			}
		} else {
			slug := slugOrValue(title, "taut-artikel")
			if _, err := tx.Exec(ctx,
				"INSERT INTO content_documents (id, brand_id, job_id, status, title, slug, meta_title, meta_description, current_version, created_by, approved_by, approved_at) VALUES ($1, $2, $3, 'approved', $4, $5, $6, $7, 1, $8, $9, NOW())",
				docID, brandID, jobID, title, slug, truncateString(title, 255), metaDescription, userID, userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, 1, $3, 'html', 'ai_generated', 'Taut Pintar Writer', $4)",
				uuid.NewString(), docID, contentHTML, userID); err != nil {
				return err
			}
		}
		if isRegen {
			if _, err := tx.Exec(ctx,
				"UPDATE taut_campaign_media SET article_status = 'draft', regen_count = regen_count + 1 WHERE id = $1", itemID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx,
				"UPDATE taut_campaign_media SET article_document_id = $1, article_status = 'draft' WHERE id = $2", docID, itemID); err != nil {
				return err
			}
		}
		if chosenCategory != "" {
			// Isi kolom rubrik hanya bila masih kosong: pilihan manual klien
			// tidak ditimpa saat artikel diregenerasi.
			if _, err := tx.Exec(ctx,
				"UPDATE taut_campaign_media SET category = $1 WHERE id = $2 AND (category IS NULL OR category = '')",
				chosenCategory, itemID); err != nil {
				return err
			}
		}
		if angleOverride {
			// The article and its QC context must change together. Empty article_type
			// clears legacy editorial/advertorial policy without reusing it for angles.
			if _, err := tx.Exec(ctx, `UPDATE taut_campaign_media
				SET angle_slug = $1, content_type = NULLIF($2, ''),
				    writer_topic = COALESCE(NULLIF($3, ''), writer_topic) WHERE id = $4`,
				angle.Slug, articleType, clientTopic, itemID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return writerItemResult{}, err
	}
	if !isRegen {
		if err := d.chargeCredit(ctx, brandID, "taut-writer:"+campaignID+":"+mediaID,
			d.writerCreditCost(ctx), "Taut Pintar — penulisan AI artikel "+name, userID); err != nil {
			return writerItemResult{}, err
		}
	}
	return writerItemResult{MediaID: mediaID, MediaType: mediaType, Status: "draft", DocumentID: docID, Title: title,
		Category: chosenCategory, ImagesGenerated: imagesGenerated, ImagesCostCredits: imagesCost}, nil
}

// matchAllowedCategory: cocokkan pilihan rubrik dari model dengan daftar
// kategori media (abaikan besar-kecil huruf, spasi, dan tanda kutip yang biasa
// ditambahkan model). Jawaban yang tidak ada di daftar — termasuk jawaban
// kosong karena model ragu atau outputnya bukan JSON — jatuh ke kategori
// PERTAMA pada daftar, karena daftar media sudah diurutkan dan mitra memilih
// urutan itu. Daftar kosong berarti media tidak punya kategori: kembalikan
// kosong, jangan mengarang rubrik baru.
func matchAllowedCategory(answer string, allowed []string) string {
	if len(allowed) == 0 {
		return ""
	}
	want := strings.ToLower(strings.Trim(strings.TrimSpace(answer), `"'`))
	if want != "" {
		for _, a := range allowed {
			if strings.ToLower(strings.TrimSpace(a)) == want {
				return a
			}
		}
	}
	return allowed[0]
}

// userGenerateUsed: total generate akun (generate pertama + regenerasi) pada
// semua kampanye brand ini. Dipakai bersama userGenerateQuota sebagai limit.
func (d *Deps) userGenerateUsed(ctx context.Context, brandID string) int {
	row, err := d.queryRow(ctx, `
		SELECT (COALESCE(SUM(tcm.regen_count), 0) + COUNT(tcm.article_document_id))::int AS used
		FROM taut_campaign_media tcm
		JOIN campaigns c ON c.id = tcm.campaign_id
		WHERE c.brand_id = $1 AND tcm.article_document_id IS NOT NULL`, brandID)
	if err != nil || row == nil {
		return 0
	}
	return row.intOr("used", 0)
}

// userGenerateQuota: kuota generate per akun (users.writer_generate_quota).
// NULL / belum diatur = 0 artinya TANPA BATAS. Limit hanya berlaku bila admin
// mengisinya di halaman Admin Users.
func (d *Deps) userGenerateQuota(ctx context.Context, userID string) int {
	row, err := d.queryRow(ctx, "SELECT writer_generate_quota FROM users WHERE id = $1 LIMIT 1", userID)
	if err != nil || row == nil || !row.has("writer_generate_quota") {
		return 0
	}
	return row.intOr("writer_generate_quota", 0)
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

// writerCreditCostKey: kunci platform_settings untuk tarif penulisan artikel.
const writerCreditCostKey = "writer_credit_cost"

// defaultWriterCreditCost: tarif bawaan sebelum admin mengubahnya (dalam kredit).
const defaultWriterCreditCost = 2

// writerCreditCost: tarif kredit per artikel yang dihasilkan The Writer —
// BERLAKU RATA untuk artikel baru maupun tulis ulang. Dibaca dari satu tempat
// (platform_settings) supaya penagihan dan tampilan memakai angka yang sama;
// sebelumnya angka ini tertulis terpisah di backend dan frontend.
func (d *Deps) writerCreditCost(ctx context.Context) int {
	row, err := d.queryRow(ctx,
		"SELECT value FROM platform_settings WHERE key = $1 LIMIT 1", writerCreditCostKey)
	if err == nil && row != nil {
		if n, convErr := strconv.Atoi(strings.TrimSpace(row.strOr("value", ""))); convErr == nil && n > 0 {
			return n
		}
	}
	return defaultWriterCreditCost
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
// tautChosenAngleSlug: slug angle yang dipilih klien untuk tipe media ini
// (submission Profiler terbaru). Kosong bila belum ada pilihan.
func (d *Deps) tautChosenAngleSlug(ctx context.Context, campaignID, mediaType string) string {
	sub, err := d.queryRow(ctx, `
		SELECT channels
		FROM taut_profiler_submissions
		WHERE campaign_id = $1
		ORDER BY revision DESC LIMIT 1`, campaignID)
	if err != nil || sub == nil {
		return ""
	}
	for _, entry := range asSliceValue(sub.jsonField("channels")) {
		ch := asMapValue(entry)
		if ch == nil || mapGetString(ch, "channel", "") != mediaType {
			continue
		}
		if slug := mapGetString(ch, "angle_slug", ""); slug != "" {
			return slug
		}
	}
	return ""
}

func (d *Deps) loadTautChosenAnglePrompt(ctx context.Context, campaignID, mediaType string) string {
	angleSlug := d.tautChosenAngleSlug(ctx, campaignID, mediaType)
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
	return "[ANGLE PILIHAN KLIEN — " + mediaType + "/" + angleSlug + "]\n" + truncateString(prompt, 16000)
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

// partialContentHTML: ekstrak nilai content_html dari akumulasi JSON mentah
// yang mengalir dari LLM (json_mode). Escape JSON di-decode sampai posisi
// terakhir yang valid, jadi teks parsial aman dirender di editor. closed=true
// berarti nilai sudah lengkap (kutip penutup ditemukan).
func partialContentHTML(acc string) (visible string, closed bool) {
	const key = `"content_html"`
	i := strings.Index(acc, key)
	if i < 0 {
		return "", false
	}
	j := i + len(key)
	for j < len(acc) && (acc[j] == ' ' || acc[j] == ':') {
		j++
	}
	if j >= len(acc) || acc[j] != '"' {
		return "", false
	}
	j++
	var out strings.Builder
	for j < len(acc) {
		c := acc[j]
		if c == '"' {
			return out.String(), true
		}
		if c != '\\' {
			out.WriteByte(c)
			j++
			continue
		}
		// Escape: tunggu delta berikutnya bila terpotong di ujung.
		if j+1 >= len(acc) {
			break
		}
		esc := acc[j+1]
		switch esc {
		case '"', '\\', '/':
			out.WriteByte(esc)
			j += 2
		case 'n':
			out.WriteByte('\n')
			j += 2
		case 't':
			out.WriteByte('\t')
			j += 2
		case 'r':
			out.WriteByte('\r')
			j += 2
		case 'b':
			out.WriteByte('\b')
			j += 2
		case 'f':
			out.WriteByte('\f')
			j += 2
		case 'u':
			if j+6 > len(acc) {
				return out.String(), false
			}
			code, pErr := strconv.ParseUint(acc[j+2:j+6], 16, 32)
			if pErr != nil {
				out.WriteByte(esc)
				j += 2
				continue
			}
			out.WriteRune(rune(code))
			j += 6
		default:
			out.WriteByte(esc)
			j += 2
		}
	}
	return out.String(), false
}

// peStreamHTML: dekoder inkremental untuk partialContentHTML. Keluaran tiap
// Write() adalah TAMBAHAN yang akan dihasilkan partialContentHTML pada prefix
// akumulasi tersebut — decode kiri-ke-kanan deterministik dan hanya bertambah,
// jadi kursor boleh dibawa antar delta tanpa memindai ulang seluruh akumulasi.
type peStreamHTML struct {
	raw     strings.Builder
	scanned int // batas pencarian key; key bisa terbelah di tengah delta
	stage   int // 0 = cari key, 1 = tunggu quote pembuka, 2 = decode isi
	j       int // posisi baca berikutnya di raw
	out     strings.Builder
	outLen  int
	closed  bool
}

func newPEStreamHTML() *peStreamHTML { return &peStreamHTML{} }

// Write menambah delta dan mengembalikan tambahan teks yang ter-decode.
func (s *peStreamHTML) Write(delta string) (tail string, closed bool) {
	if s.closed {
		return "", true
	}
	s.raw.WriteString(delta)
	acc := s.raw.String()

	const key = `"content_html"`
	if s.stage == 0 {
		from := s.scanned - len(key) + 1
		if from < 0 {
			from = 0
		}
		i := strings.Index(acc[from:], key)
		if i < 0 {
			s.scanned = len(acc)
			return "", false
		}
		s.j = from + i + len(key)
		s.stage = 1
	}
	if s.stage == 1 {
		for s.j < len(acc) && (acc[s.j] == ' ' || acc[s.j] == ':') {
			s.j++
		}
		if s.j >= len(acc) {
			return "", false
		}
		if acc[s.j] != '"' {
			return "", false
		}
		s.j++
		s.stage = 2
	}
	for s.j < len(acc) {
		c := acc[s.j]
		if c == '"' {
			s.closed = true
			break
		}
		if c != '\\' {
			s.out.WriteByte(c)
			s.j++
			continue
		}
		// Escape: tunggu delta berikutnya bila terpotong di ujung.
		if s.j+1 >= len(acc) {
			break
		}
		esc := acc[s.j+1]
		switch esc {
		case '"', '\\', '/':
			s.out.WriteByte(esc)
			s.j += 2
		case 'n':
			s.out.WriteByte('\n')
			s.j += 2
		case 't':
			s.out.WriteByte('\t')
			s.j += 2
		case 'r':
			s.out.WriteByte('\r')
			s.j += 2
		case 'b':
			s.out.WriteByte('\b')
			s.j += 2
		case 'f':
			s.out.WriteByte('\f')
			s.j += 2
		case 'u':
			if s.j+6 > len(acc) {
				return s.tail(), false
			}
			code, pErr := strconv.ParseUint(acc[s.j+2:s.j+6], 16, 32)
			if pErr != nil {
				s.out.WriteByte(esc)
				s.j += 2
				continue
			}
			s.out.WriteRune(rune(code))
			s.j += 6
		default:
			s.out.WriteByte(esc)
			s.j += 2
		}
	}
	return s.tail(), s.closed
}

func (s *peStreamHTML) tail() string {
	out := s.out.String()
	if len(out) <= s.outLen {
		return ""
	}
	tail := out[s.outLen:]
	s.outLen = len(out)
	return tail
}
