package clienttaut

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// Port lib/taut/profiler/{repository,prompt-resolver,schema,angle-fields}.ts
// (bagian yang dipakai endpoint klien).

// ---------- enumerasi master (port profiler/schema.ts) ----------

type objectiveOption struct {
	Value string
	Label string
}

var objectives = []objectiveOption{
	{"brand_awareness", "Brand Awareness & Edukasi"},
	{"seo_dominance", "Dominasi SEO & Backlink"},
	{"conversion", "Konversi & Promosi Langsung"},
	{"reputation_management", "Manajemen Reputasi / Klarifikasi"},
}

var objectiveDescriptions = map[string]string{
	"brand_awareness":       "Fokus AI: Menyebarluaskan nama brand dan cerita di baliknya, menggunakan pendekatan soft-selling.",
	"seo_dominance":         "Fokus AI: Struktur heading ketat, optimasi LSI, dan pemetaan keyword organik yang natural.",
	"conversion":            "Fokus AI: Menonjolkan USP tajam dengan CTA menuju landing page.",
	"reputation_management": "Fokus AI: Bahasa berhati-hati, taktis, meredam sentimen negatif, menyajikan fakta kuat.",
}

var tautChannels = []string{"mainstream", "blogger"}

var defaultAudienceOptions = []string{
	"Mahasiswa",
	"Ibu Hamil",
	"Karyawan",
	"Profesional",
	"Pemilik Bisnis",
	"Pengusaha / Entrepreneur",
	"Freelancer",
	"Orang Tua",
	"Pelajar",
	"Fresh Graduate",
	"Manajer / Decision Maker",
	"Developer / Teknisi",
	"Marketing / PR Professional",
	"Investor",
	"Konsumen Rumah Tangga",
}

const otherAudience = "Other / Lainnya"

// ProfilerInputSchemaVersion / ProfilerResultSchemaVersion (parity konstanta).
const (
	profilerInputSchemaVersion  = 1
	profilerResultSchemaVersion = 1
)

// crawlFactLabels: parity CRAWL_FACT_LABELS.
var crawlFactLabels = []struct{ Key, Label string }{
	{"business_name", "Nama bisnis"},
	{"industry", "Industri"},
	{"products_services", "Produk/layanan"},
	{"positioning", "Positioning"},
	{"value_proposition", "Value proposition"},
	{"tone_of_voice", "Tone of voice"},
}

// angleDynamicFields: port lib/taut/profiler/angle-fields.ts.
type dynamicAngleField struct {
	Key         string
	Label       string
	Placeholder string
}

var angleDynamicFields = map[string][]dynamicAngleField{
	"listicle": {
		{"list_items", "Apa saja item/entitas yang masuk daftar rekomendasi?", "Contoh: 7 software PR untuk kantor pemasaran digital..."},
		{"ranking_basis", "Apa kriteria urutan/penilaian daftar?", "Contoh: fitur monitoring, harga, integrasi, laporan..."},
		{"brand_edge", "Apa pembeda brand Anda dibanding item lain di daftar?", "Contoh: satu-satunya dengan laporan terjadwal otomatis tiap minggu..."},
	},
	"product_review": {
		{"product_name", "Nama produk yang direview?", "Contoh: Speaker Bluetooth X100..."},
		{"reviewer_experience", "Pengalaman penggunaan produknya seperti apa?", "Contoh: dipakai 2 minggu untuk rapat online, baterai tahan 8 jam..."},
		{"strengths_weaknesses", "Kelebihan dan kekurangan utamanya?", "Contoh: suara jernih, tapi agak berat dibawa..."},
	},
	"how_to": {
		{"task_goal", "Masalah/tugas apa yang mau diselesaikan reader?", "Contoh: cara memilih laptop untuk desain grafis..."},
		{"steps_tips", "Langkah-langkah atau tips utamanya?", "Contoh: cek RAM minimal 16GB, GPU dedicated, layar 100% sRGB..."},
		{"tools_needed", "Alat/bahan yang dibutuhkan?", "Contoh: hanya butuh browser dan spreadsheet..."},
	},
	"comparison": {
		{"compared_items", "Produk/layanan apa saja yang dibandingkan?", "Contoh: paket hosting A vs paket hosting B vs VPS..."},
		{"comparison_aspects", "Aspek apa yang dibandingkan?", "Contoh: harga, performa, support, kemudahan migrasi..."},
		{"your_position", "Posisi produk Anda di perbandingan?", "Contoh: unggul di support 24/7 dan garansi uptime..."},
	},
	"unboxing": {
		{"product_name", "Nama produk yang di-unboxing?", "Contoh: Smartphone flagship merek X..."},
		{"package_contents", "Isi paket/kemasannya apa saja?", "Contoh: unit, casing, kabel, kartu garansi..."},
		{"first_impression", "Kesan pertama saat membuka paket?", "Contoh: build terasa solid, warna lebih premium dari foto..."},
	},
	"success_case": {
		{"case_subject", "Siapa/kasus siapa yang dibahas?", "Contoh: UMKM roti yang naik omzet 3x..."},
		{"before_condition", "Kondisi sebelum memakai solusi Anda?", "Contoh: pencatatan manual, sering salah hitung stok..."},
		{"after_result", "Hasil nyata setelahnya (angka bila ada)?", "Contoh: omzet naik dari 5 juta ke 15 juta per bulan..."},
	},
	"niche_education": {
		{"topic_concept", "Konsep/isu niche apa yang dijelaskan?", "Contoh: apa itu NVMe dan bedanya dengan SSD SATA..."},
		{"common_confusion", "Kebingungan umum reader tentang topik ini?", "Contoh: banyak yang kira semua SSD sama kecepatannya..."},
		{"practical_takeaway", "Takeaway praktis untuk reader?", "Contoh: cek slot M.2 di motherboard sebelum beli NVMe..."},
	},
	"community_event": {
		{"event_name", "Nama acara/event-nya?", "Contoh: Komunitas Pecinta Kopi Fest 2026..."},
		{"event_details", "Kapan, di mana, dan siapa penyelenggaranya?", "Contoh: 12 Agustus di Balai Kartini Jakarta, oleh Asosiasi PR Indonesia..."},
		{"brand_involvement", "Peran brand Anda di acara itu?", "Contoh: sponsor utama dan buka booth demo mesin kopi..."},
	},
	"soft_sell_promo": {
		{"offer_details", "Apa penawaran spesialnya (potongan, bonus, periode)?", "Contoh: diskon 30% semua layanan sampai akhir bulan..."},
		{"offer_terms", "Syarat dan cara klaimnya?", "Contoh: min. transaksi 100rb, klaim via WhatsApp..."},
		{"why_now", "Kenapa pembaca harus segera ambil penawaran ini?", "Contoh: kuota 50 pelanggan pertama saja..."},
	},
	"product_launch": {
		{"product_name", "Nama produk/fitur yang diluncurkan?", "Contoh: Aplikasi Kasir Tauthub Pay..."},
		{"launch_details", "Kapan diluncurkan dan apa yang baru?", "Contoh: rilis 20 Agustus, fitur split bill otomatis..."},
		{"availability", "Di mana/kapan bisa didapat dan harganya?", "Contoh: tersedia di Play Store, gratis 30 hari..."},
	},
	"funding_announcement": {
		{"funding_round", "Tahap dan besaran pendanaan?", "Contoh: Series A sebesar Rp 50 miliar..."},
		{"investors", "Siapa investor yang terlibat?", "Contoh: dipimpin VenturX dengan partisipasi Angel Cap..."},
		{"fund_usage", "Dana akan dipakai untuk apa?", "Contoh: ekspansi ke 5 kota dan riset AI..."},
	},
	"c_level_change": {
		{"executive_name", "Nama dan jabatan baru pejabat?", "Contoh: Budi Santoso ditunjuk sebagai CEO..."},
		{"background", "Latar belakang profesionalnya?", "Contoh: 15 tahun di industri fintech, mantan direktur Bank X..."},
		{"mandate", "Misi/tanggung jawab utamanya?", "Contoh: mempercepat ekspansi regional..."},
	},
	"strategic_partnership_b2b": {
		{"partner_names", "Siapa pihak yang bermitra?", "Contoh: PT Maju Jaya dengan Distribusi Nusantara..."},
		{"partnership_scope", "Apa isi lingkup kerja samanya?", "Contoh: distribusi eksklusif produk di Sumatera..."},
		{"partnership_goal", "Target hasil kemitraannya?", "Contoh: jangkau 500 toko dalam 1 tahun..."},
	},
	"milestone_awards": {
		{"achievement", "Pencapaian/penghargaan apa yang diraih?", "Contoh: penghargaan UMKM Digital Terbaik 2026..."},
		{"milestone_numbers", "Angka milestone-nya (user, cabang, tahun)?", "Contoh: 10.000 pelanggan aktif, cabang ke-12..."},
		{"milestone_context", "Pemberi penghargaan / konteks perayaannya?", "Contoh: dari Dinas Koperasi UKM provinsi..."},
	},
	"research_data_trends": {
		{"research_subject", "Topik riset/tren yang diangkat?", "Contoh: tren belanja online gens Z Indonesia..."},
		{"key_data", "Data/angka kunci yang ada?", "Contoh: 68% Gen Z belanja via marketplace tiap bulan..."},
		{"data_source", "Sumber data/risetnya dari mana?", "Contoh: survei internal 1.200 responden..."},
	},
	"csr_sustainability": {
		{"csr_program", "Nama program CSR/sustainability-nya?", "Contoh: Gerakan Hijau Sekolah..."},
		{"csr_activities", "Kegiatan dan penerima manfaatnya?", "Contoh: tanam 1.000 pohon, melibatkan 20 sekolah..."},
		{"csr_impact", "Dampak/hasil yang ditargetkan?", "Contoh: penghijauan 5 hektar lahan kritis..."},
	},
	"rebranding_transformation": {
		{"rebrand_reason", "Alasan utama rebranding/transformasi?", "Contoh: perluasan pasar ke segmen keluarga..."},
		{"rebrand_changes", "Apa saja yang berubah (nama, logo, layanan)?", "Contoh: logo baru, nama singkat, layanan booking online..."},
		{"rebrand_continuity", "Apa yang tetap dipertahankan?", "Contoh: tim konsultan yang sama dan standar layanan..."},
	},
	"reputation_clarification": {
		{"issue_summary", "Isu apa yang mau diklarifikasi?", "Contoh: isu produk mengandung bahan berbahaya..."},
		{"clarification_facts", "Fakta/data pendukung klarifikasi?", "Contoh: hasil lab Sertifikasi No. 123/LAB/X..."},
		{"corrective_steps", "Langkah perbaikan yang diambil?", "Contoh: recall sukarela dan penggantian gratis..."},
	},
	"thought_leadership": {
		{"expert_identity", "Siapa pakarnya (nama, jabatan)?", "Contoh: dr. Andi, Direktur Medis Klinik Sehat..."},
		{"opinion_topic", "Isu/opini strategis yang diangkat?", "Contoh: krisis air bersih kota besar 2030..."},
		{"key_argument", "Argumen utama/solusi yang ditawarkan?", "Contoh: recycling air wajib di gedung perkantoran..."},
	},
	"inspirational_story": {
		{"main_figure", "Siapa tokoh utama kisahnya?", "Contoh: Ibu Ratna, penjual nasi keliling..."},
		{"struggle_journey", "Perjuangan dan titik baliknya?", "Contoh: dari modal 50 ribu kini punya 3 cabang..."},
		{"inspiration_message", "Pesan inspirasi untuk pembaca?", "Contoh: konsisten kecil-kecil tetapi berdampak..."},
	},
	"large_event_coverage": {
		{"event_name", "Nama event/konferensinya?", "Contoh: Tech Summit Indonesia 2026..."},
		{"event_scale", "Skala, waktu, dan tempat penyelenggaraan?", "Contoh: 5.000 peserta, 15-16 Sep, JCC Senayan..."},
		{"brand_participation", "Partisipasi/presensi brand di event?", "Contoh: jadi exhibitor dan bicara di panel AI..."},
	},
	"special_offer_advertorial": {
		{"offer_headline", "Penawaran utama untuk advertorial?", "Contoh: paket website company profile 2 juta..."},
		{"offer_audience", "Target pembeli penawaran ini?", "Contoh: UMKM yang belum punya website..."},
		{"offer_cta", "Ajakan/CTA yang diinginkan di akhir artikel?", "Contoh: hubungi WhatsApp untuk konsultasi gratis..."},
	},
}

// ---------- repository ----------

type channelRow struct {
	ID          string
	Name        string
	Slug        string
	Description string
	Active      bool
	SortOrder   int
}

type angleRow struct {
	ID          string
	ChannelID   string
	Name        string
	Slug        string
	Description string
	Active      bool
	SortOrder   int
	HasPrompt   bool
}

func (d *Deps) listActiveChannels(ctx context.Context) ([]channelRow, error) {
	list, err := d.queryRows(ctx,
		"SELECT id, name, slug, description, active, sort_order FROM taut_media_channels WHERE active = TRUE ORDER BY sort_order, name")
	if err != nil {
		return nil, err
	}
	out := make([]channelRow, 0, len(list))
	for _, r := range list {
		out = append(out, channelRow{
			ID: r.strOr("id", ""), Name: r.strOr("name", ""), Slug: r.strOr("slug", ""),
			Description: r.strOr("description", ""), Active: r.boolOr("active", true), SortOrder: r.intOr("sort_order", 0),
		})
	}
	return out, nil
}

func (d *Deps) listAnglesByChannel(ctx context.Context, channelSlug string) ([]angleRow, error) {
	list, err := d.queryRows(ctx,
		`SELECT a.id, a.channel_id, a.name, a.slug, a.description, a.active, a.sort_order,
		        COALESCE((SELECT v.prompt ~ '[^[:space:]]' FROM taut_content_angle_prompt_versions v
		                  WHERE v.content_angle_id = a.id AND v.status = 'active'
		                  ORDER BY v.version DESC LIMIT 1), FALSE) AS has_prompt
		 FROM taut_content_angles a
		 JOIN taut_media_channels c ON c.id = a.channel_id
		 WHERE a.active = TRUE AND c.active = TRUE AND c.slug = $1
		 ORDER BY a.sort_order, a.name`, channelSlug)
	if err != nil {
		return nil, err
	}
	out := make([]angleRow, 0, len(list))
	for _, r := range list {
		out = append(out, angleRow{
			ID: r.strOr("id", ""), ChannelID: r.strOr("channel_id", ""), Name: r.strOr("name", ""),
			Slug: r.strOr("slug", ""), Description: r.strOr("description", ""),
			Active: r.boolOr("active", true), SortOrder: r.intOr("sort_order", 0), HasPrompt: r.boolOr("has_prompt", false),
		})
	}
	return out, nil
}

func (d *Deps) getAngleBySlug(ctx context.Context, slug string) (*angleRow, error) {
	r, err := d.queryRow(ctx,
		"SELECT id, channel_id, name, slug, description, active, sort_order FROM taut_content_angles WHERE slug = $1 AND active = TRUE LIMIT 1", slug)
	if err != nil || r == nil {
		return nil, err
	}
	return &angleRow{
		ID: r.strOr("id", ""), ChannelID: r.strOr("channel_id", ""), Name: r.strOr("name", ""),
		Slug: r.strOr("slug", ""), Description: r.strOr("description", ""),
		Active: r.boolOr("active", true), SortOrder: r.intOr("sort_order", 0),
	}, nil
}

// getActiveAnglePrompt: versi prompt ACTIVE untuk satu angle.
func (d *Deps) getActiveAnglePrompt(ctx context.Context, angleID string) (id string, prompt string, version int, ok bool) {
	r, err := d.queryRow(ctx,
		`SELECT id, version, prompt FROM taut_content_angle_prompt_versions
		 WHERE content_angle_id = $1 AND status = 'active' ORDER BY version DESC LIMIT 1`, angleID)
	if err != nil || r == nil {
		return "", "", 0, false
	}
	return r.strOr("id", ""), r.strOr("prompt", ""), r.intOr("version", 0), true
}

// getActiveBasePrompt: versi base prompt Profiler ACTIVE.
func (d *Deps) getActiveBasePrompt(ctx context.Context) (id string, prompt string, version int, ok bool) {
	r, err := d.queryRow(ctx,
		"SELECT id, version, prompt FROM taut_profiler_base_prompt_versions WHERE status = 'active' ORDER BY version DESC LIMIT 1")
	if err != nil || r == nil {
		return "", "", 0, false
	}
	return r.strOr("id", ""), r.strOr("prompt", ""), r.intOr("version", 0), true
}

// profilerSubmission: baris taut_profiler_submissions (draft aktif / terbaru).
type profilerSubmission struct {
	ID              string
	CampaignID      string
	Revision        int
	Status          string
	Objective       any
	BusinessProfile any
	Audience        any
	Spokesperson    any
	Spokepersons    any
	Guardrails      any
	Backlink        any
	Materials       any
	Budget          any
	Channels        any
	CrawlResult     any
}

const profilerSubmissionCols = `id, campaign_id, revision, status, objective, business_profile, audience,
	spokesperson, spokepersons, guardrails, backlink, materials, budget, channels, crawl_result`

func mapProfilerSubmission(m rowMap) profilerSubmission {
	return profilerSubmission{
		ID:              m.strOr("id", ""),
		CampaignID:      m.strOr("campaign_id", ""),
		Revision:        m.intOr("revision", 1),
		Status:          m.strOr("status", ""),
		Objective:       jsonField(m, "objective"),
		BusinessProfile: jsonField(m, "business_profile"),
		Audience:        jsonField(m, "audience"),
		Spokesperson:    jsonField(m, "spokesperson"),
		Spokepersons:    jsonField(m, "spokepersons"),
		Guardrails:      jsonField(m, "guardrails"),
		Backlink:        jsonField(m, "backlink"),
		Materials:       jsonField(m, "materials"),
		Budget:          jsonField(m, "budget"),
		Channels:        jsonField(m, "channels"),
		CrawlResult:     jsonField(m, "crawl_result"),
	}
}

// getCurrentDraftSubmission: submission status='draft' dengan revision terbesar.
func (d *Deps) getCurrentDraftSubmission(ctx context.Context, campaignID string) (*profilerSubmission, error) {
	m, err := d.queryRow(ctx,
		"SELECT "+profilerSubmissionCols+" FROM taut_profiler_submissions WHERE campaign_id = $1 AND status = 'draft' ORDER BY revision DESC LIMIT 1",
		campaignID)
	if err != nil || m == nil {
		return nil, err
	}
	s := mapProfilerSubmission(m)
	return &s, nil
}

// profilerResult: baris taut_profiler_results + kolom pendamping.
type profilerResult struct {
	ID                   string
	SubmissionID         string
	ResultSchemaVersion  int
	ExecutiveBrief       any
	WritingInstructions  any
	StrategicNotes       any
	CompiledPromptManif  any
	ModelSnapshot        any
	CreatedAt            any
	RawExecutiveBriefMap map[string]any
}

func mapProfilerResult(m rowMap) profilerResult {
	brief := jsonField(m, "executive_brief")
	var briefMap map[string]any
	if bm, ok := brief.(map[string]any); ok {
		briefMap = bm
	}
	return profilerResult{
		ID:                   m.strOr("id", ""),
		SubmissionID:         m.strOr("submission_id", ""),
		ResultSchemaVersion:  m.intOr("result_schema_version", 1),
		ExecutiveBrief:       brief,
		WritingInstructions:  jsonField(m, "writing_instructions"),
		StrategicNotes:       jsonField(m, "strategic_notes"),
		CompiledPromptManif:  jsonField(m, "compiled_prompt_manifest"),
		ModelSnapshot:        jsonField(m, "model_snapshot"),
		CreatedAt:            m.timePtr("created_at"),
		RawExecutiveBriefMap: briefMap,
	}
}

const profilerResultCols = `id, submission_id, result_schema_version, executive_brief, writing_instructions,
	strategic_notes, compiled_prompt_manifest, model_snapshot, created_at`

// getLatestCompletedResult: hasil profiler terbaru untuk campaign.
func (d *Deps) getLatestCompletedResult(ctx context.Context, campaignID string) (*profilerResult, error) {
	m, err := d.queryRow(ctx,
		"SELECT r.id, r.submission_id, r.result_schema_version, r.executive_brief, r.writing_instructions, r.strategic_notes, r.compiled_prompt_manifest, r.model_snapshot, r.created_at FROM taut_profiler_results r JOIN taut_profiler_submissions s ON s.id = r.submission_id WHERE s.campaign_id = $1 ORDER BY r.created_at DESC LIMIT 1",
		campaignID)
	if err != nil || m == nil {
		return nil, err
	}
	r := mapProfilerResult(m)
	return &r, nil
}

// getResultByID: hasil profiler berdasarkan id.
func (d *Deps) getResultByID(ctx context.Context, resultID string) (*profilerResult, error) {
	m, err := d.queryRow(ctx, "SELECT "+profilerResultCols+" FROM taut_profiler_results WHERE id = $1 LIMIT 1", resultID)
	if err != nil || m == nil {
		return nil, err
	}
	r := mapProfilerResult(m)
	return &r, nil
}

// ---------- prompt resolver (port profiler/prompt-resolver.ts) ----------

type resolvedAngleBlock struct {
	ChannelSlug     string
	ChannelID       string
	AngleSlug       string
	AngleID         string
	PromptVersionID string
	PromptVersion   int
	Prompt          string
}

type resolvedManifestChannel struct {
	Channel         string
	ChannelID       string
	Angle           string
	AngleID         string
	PromptVersionID string
	PromptVersion   int
}

type resolvedProfilerPrompt struct {
	SystemPrompt string
	UserPrompt   string
	Manifest     map[string]any
	InputHash    string
}

type resolveProfilerPromptInput struct {
	BrandID         string
	Channels        []selectedChannel
	Objective       string
	Audience        audienceSelection
	Spokesperson    spokesperson
	Spokepersons    []spokesperson
	BusinessProfile businessProfile
	Guardrails      guardrails
	Backlink        backlinkStrategy
	FileFacts       []string
	Dynamic         map[string]any
	Budget          budgetStrategy
	CampaignName    string
	CrawlFacts      []crawlFact
}

type crawlFact struct {
	Label  string
	Value  string
	Source string
}

type selectedChannel struct {
	Channel   string
	AngleSlug string
}

type audienceSelection struct {
	Selected []string
	Custom   string
}

type spokesperson struct {
	Name  string
	Role  string
	Quote string
}

type businessProfile struct {
	Business string
	Industry string
	Audience string
	USP      string
	Website  string
}

type guardrails struct {
	Rules []string
}

type backlinkStrategy struct {
	Enabled         bool
	TargetURL       string
	AnchorText      string
	SmartVariations bool
}

// Tag json wajib huruf kecil: tanpa ini draft kembali sebagai {"Mode","Amount"}
// dan frontend membacanya sebagai amount=0 (anggaran hilang saat halaman dimuat ulang).
type budgetStrategy struct {
	Mode   string `json:"mode"`
	Amount int    `json:"amount"`
}

// resolveProfilerPrompt: susun SYSTEM PROMPT + USER PROMPT berlabel + manifest
// provenance. Melempar error bila channel/angle belum punya versi prompt aktif.
func (d *Deps) resolveProfilerPrompt(ctx context.Context, in resolveProfilerPromptInput) (*resolvedProfilerPrompt, error) {
	_, basePrompt, _, _ := d.getActiveBasePrompt(ctx)
	channels, err := d.listActiveChannels(ctx)
	if err != nil {
		return nil, err
	}
	known := map[string]channelRow{}
	for _, c := range channels {
		known[c.Slug] = c
	}

	blocks := []resolvedAngleBlock{}
	for _, sel := range in.Channels {
		channel, ok := known[sel.Channel]
		if !ok {
			return nil, errUnprocessable("Channel media \"" + sel.Channel + "\" tidak tersedia.")
		}
		angle, err := d.getAngleBySlug(ctx, sel.AngleSlug)
		if err != nil {
			return nil, err
		}
		if angle == nil || angle.ChannelID != channel.ID {
			return nil, errUnprocessable("Angle \"" + sel.AngleSlug + "\" tidak kompatibel dengan channel \"" + sel.Channel + "\".")
		}
		versionID, prompt, version, ok := d.getActiveAnglePrompt(ctx, angle.ID)
		if !ok {
			return nil, errUnprocessable("Angle \"" + angle.Name + "\" belum memiliki versi prompt aktif. Hubungi super admin untuk publish di Taut Prompt Studio.")
		}
		blocks = append(blocks, resolvedAngleBlock{
			ChannelSlug: channel.Slug, ChannelID: channel.ID,
			AngleSlug: angle.Slug, AngleID: angle.ID,
			PromptVersionID: versionID, PromptVersion: version, Prompt: prompt,
		})
	}

	objectiveLabel := objectiveDescriptions[in.Objective]
	if objectiveLabel == "" {
		objectiveLabel = in.Objective
	}
	audienceList := append([]string{}, in.Audience.Selected...)
	if in.Audience.Custom != "" {
		audienceList = append(audienceList, in.Audience.Custom)
	}
	filteredAudience := []string{}
	for _, a := range audienceList {
		if a != "" {
			filteredAudience = append(filteredAudience, a)
		}
	}
	guardrailList := in.Guardrails.Rules

	userBlocks := []string{}
	userBlocks = append(userBlocks, "[CAMPAIGN NAME]\n"+in.CampaignName)
	userBlocks = append(userBlocks, "[OBJECTIVE]\n"+in.Objective+" — "+objectiveLabel)

	if in.BusinessProfile.Business != "" {
		userBlocks = append(userBlocks, "[BUSINESS PROFILE]\n"+jsonIndent2(map[string]any{
			"business": in.BusinessProfile.Business,
			"industry": orDash(in.BusinessProfile.Industry),
			"audience": orDash(in.BusinessProfile.Audience),
			"usp":      orDash(in.BusinessProfile.USP),
			"website":  orDash(in.BusinessProfile.Website),
		}))
	}

	if len(in.CrawlFacts) > 0 {
		lines := make([]string, 0, len(in.CrawlFacts))
		for _, f := range in.CrawlFacts {
			suffix := ""
			if f.Source != "" {
				suffix = " (sumber: " + f.Source + ")"
			}
			lines = append(lines, "- "+f.Label+": "+f.Value+suffix)
		}
		userBlocks = append(userBlocks, "[CRAWLED FACTS — hanya fakta yang berhasil diekstrak dari website]\n"+strings.Join(lines, "\n"))
	}

	if len(filteredAudience) > 0 {
		userBlocks = append(userBlocks, "[TARGET AUDIENCE]\n"+strings.Join(filteredAudience, ", "))
	}
	if in.Spokesperson.Name != "" {
		quote := ""
		if strings.TrimSpace(in.Spokesperson.Quote) != "" {
			quote = "\nKutipan: \"" + strings.TrimSpace(in.Spokesperson.Quote) + "\""
		}
		line := in.Spokesperson.Name
		if in.Spokesperson.Role != "" {
			line += ", " + in.Spokesperson.Role
		}
		userBlocks = append(userBlocks, "[SPOKESPERSON / NARASUMBER]\n"+line+quote)
	}
	spList := []spokesperson{}
	for _, s := range in.Spokepersons {
		if strings.TrimSpace(s.Name) != "" {
			spList = append(spList, s)
		}
	}
	if len(spList) > 0 {
		lines := make([]string, 0, len(spList))
		for i, s := range spList {
			line := itoa(i+1) + ". " + s.Name
			if s.Role != "" {
				line += ", " + s.Role
			}
			if strings.TrimSpace(s.Quote) != "" {
				line += " — kutipan: \"" + strings.TrimSpace(s.Quote) + "\""
			}
			lines = append(lines, line)
		}
		userBlocks = append(userBlocks, "[SPOKESPERSON / NARASUMBER — WAJIB dipakai sebagai kutipan dalam artikel]\n"+strings.Join(lines, "\n"))
	}

	if len(guardrailList) > 0 {
		userBlocks = append(userBlocks, "[GUARDRAILS — hard instruction, jangan dilanggar]\n"+strings.Join(guardrailList, "\n"))
	} else {
		userBlocks = append(userBlocks, "[GUARDRAILS]\nTidak ada pantangan khusus.")
	}

	if in.Backlink.Enabled {
		variations := "nonaktif"
		if in.Backlink.SmartVariations {
			variations = "aktif"
		}
		userBlocks = append(userBlocks, "[BACKLINK STRATEGY]\nAktif\nTarget URL: "+in.Backlink.TargetURL+
			"\nAnchor: "+in.Backlink.AnchorText+"\nAI Smart Variations: "+variations)
	} else {
		userBlocks = append(userBlocks, "[BACKLINK STRATEGY]\nPenanaman link tidak diaktifkan.")
	}

	if len(in.FileFacts) > 0 {
		userBlocks = append(userBlocks, "[SOURCE MATERIAL — fakta yang diekstrak dari dokumen klien]\n"+strings.Join(in.FileFacts, "\n"))
	}
	if in.Dynamic != nil {
		if generic := dynamicGenericBlock(in.Dynamic); generic != "" {
			userBlocks = append(userBlocks, generic)
		}
		for _, b := range blocks {
			perAngle, ok := in.Dynamic[b.AngleSlug].(map[string]any)
			if !ok {
				continue
			}
			entries := []string{}
			for k, v := range perAngle {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					entries = append(entries, "- "+k+": "+strings.TrimSpace(s))
				}
			}
			if len(entries) > 0 {
				userBlocks = append(userBlocks, "[DYNAMIC PR ANSWERS — angle "+b.AngleSlug+"]\n"+strings.Join(entries, "\n"))
			}
		}
	}

	budgetLine := "[BUDGET]\nMode: Optimal (strategis)\nTanpa batas awal — rekomendasikan skala terbaik."
	if in.Budget.Mode == "efficient" {
		budgetLine = "[BUDGET]\nMode: Efisien (kalkulator)\nNominal Kredit Saldo: " + itoa(in.Budget.Amount)
	}
	userBlocks = append(userBlocks, budgetLine)

	for _, b := range blocks {
		if b.Prompt != "" {
			userBlocks = append(userBlocks, "[CHANNEL: "+b.ChannelSlug+"][ANGLE: "+b.AngleSlug+"][PROFILER INSTRUCTION]\n"+b.Prompt)
		}
	}

	userBlocks = append(userBlocks, "[OUTPUT REQUIREMENT]\n"+
		"Buat Executive PR Brief dalam SATU objek JSON valid dengan struktur berikut (tanpa teks lain):\n"+
		jsonIndent2(executiveBriefOutputShape()))

	manifestChannels := make([]resolvedManifestChannel, 0, len(blocks))
	for _, b := range blocks {
		manifestChannels = append(manifestChannels, resolvedManifestChannel{
			Channel: b.ChannelSlug, ChannelID: b.ChannelID,
			Angle: b.AngleSlug, AngleID: b.AngleID,
			PromptVersionID: b.PromptVersionID, PromptVersion: b.PromptVersion,
		})
	}
	manifest := map[string]any{
		"basePromptVersionId": nil,
		"basePromptVersion":   nil,
		"channels":            manifestChannels,
		"objective":           in.Objective,
		"inputHash":           "",
	}
	if id, _, version, ok := d.getActiveBasePrompt(ctx); ok {
		manifest["basePromptVersionId"] = id
		manifest["basePromptVersion"] = version
	}

	userPrompt := strings.Join(userBlocks, "\n\n---\n\n")
	hashInput := jsonString(map[string]any{
		"systemPrompt": basePrompt,
		"userPrompt":   userPrompt,
		"manifest":     manifest,
	}) + in.BrandID
	sum := sha256.Sum256([]byte(hashInput))
	inputHash := hex.EncodeToString(sum[:])
	manifest["inputHash"] = inputHash

	return &resolvedProfilerPrompt{
		SystemPrompt: basePrompt,
		UserPrompt:   userPrompt,
		Manifest:     manifest,
		InputHash:    inputHash,
	}, nil
}

func orDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func dynamicGenericBlock(dyn map[string]any) string {
	lines := []string{}
	if s, ok := dyn["product_function"].(string); ok && s != "" {
		lines = append(lines, "- Produk/fungsi: "+s)
	}
	if s, ok := dyn["problem_solved"].(string); ok && s != "" {
		lines = append(lines, "- Masalah yang diselesaikan: "+s)
	}
	if s, ok := dyn["offer_or_launch"].(string); ok && s != "" {
		lines = append(lines, "- Penawaran/rilis: "+s)
	}
	if len(lines) == 0 {
		return ""
	}
	return "[DYNAMIC PR ANSWERS — umum]\n" + strings.Join(lines, "\n")
}

// executiveBriefOutputShape: struktur contoh di [OUTPUT REQUIREMENT].
func executiveBriefOutputShape() map[string]any {
	return map[string]any{
		"campaign_name":      "string",
		"objective":          map[string]any{"type": "string", "label": "string", "priority": "string"},
		"business_summary":   "string",
		"audience":           map[string]any{"selected": []string{"string"}, "custom": "string", "note": "string"},
		"spokesperson":       map[string]any{"name": "string", "role": "string", "quote": "string"},
		"guardrails":         map[string]any{"active": "boolean", "summary": "string"},
		"channels":           []map[string]any{{"channel": "mainstream|blogger", "angle": "string", "writing_instruction": "string"}},
		"seo":                map[string]any{"enabled": "boolean", "target_url": "string", "anchor_text": "string", "smart_variations": "boolean"},
		"strategic_notes":    []string{"string"},
		"sources":            []string{"string"},
		"needs_verification": []string{"string"},
	}
}

// executiveBriefValid: validasi schema executive_brief hasil LLM (port
// executiveBriefSchema.safeParse) — pesan error dirakit seperti ZodError pertama.
func executiveBriefValid(brief map[string]any) (string, bool) {
	if s, ok := brief["campaign_name"].(string); !ok || strings.TrimSpace(s) == "" {
		return "campaign_name: Too small: expected string to have >=1 characters", false
	}
	objective, ok := brief["objective"].(map[string]any)
	if !ok {
		return "objective: Invalid input: expected object, received undefined", false
	}
	if s, ok := objective["type"].(string); !ok || strings.TrimSpace(s) == "" {
		return "objective.type: Too small: expected string to have >=1 characters", false
	}
	if s, ok := objective["label"].(string); !ok || strings.TrimSpace(s) == "" {
		return "objective.label: Too small: expected string to have >=1 characters", false
	}
	channels, ok := brief["channels"].([]any)
	if !ok {
		return "channels: Invalid input: expected array, received undefined", false
	}
	for i, c := range channels {
		ch, ok := c.(map[string]any)
		if !ok {
			return "channels." + itoa(i) + ": Invalid input: expected object", false
		}
		channel, _ := ch["channel"].(string)
		if channel != "mainstream" && channel != "blogger" {
			return "channels." + itoa(i) + ".channel: Invalid option: expected one of \"mainstream\"|\"blogger\"", false
		}
		if s, ok := ch["angle"].(string); !ok || strings.TrimSpace(s) == "" {
			return "channels." + itoa(i) + ".angle: Too small: expected string to have >=1 characters", false
		}
	}
	return "", true
}

// safeParseExecutiveBrief: parse + validasi; mengembalikan bentuk ternormalisasi.
func safeParseExecutiveBrief(raw any) (map[string]any, bool) {
	brief, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	if detail, valid := executiveBriefValid(brief); !valid {
		_ = detail
		return nil, false
	}
	return brief, true
}

// firstIssueMessage: pesan issue pertama untuk error 502 "Output Profiler tidak valid".
func firstIssueMessage(brief map[string]any) string {
	if detail, valid := executiveBriefValid(brief); !valid {
		return detail
	}
	return "schema mismatch"
}

var _ = json.Marshal
