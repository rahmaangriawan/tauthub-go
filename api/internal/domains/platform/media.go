package platform

// media.go: CRUD media admin + audit/review/SEO/SimilarWeb/health/intelligence.
// Port app/v1/platform/media/route.ts, [mediaId]/route.ts, [mediaId]/audit,
// [mediaId]/review, [mediaId]/recompute-health, [mediaId]/seo,
// [mediaId]/similarweb, dan intelligence/route.ts.
//
// Semua route lama hanya memeriksa isPlatformAdmin (tanpa permission modul),
// jadi guard di sini memakai permission "" (cukup admin) agar perilakunya sama.

import (
	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// mpMediaListSelect: kolom daftar media admin (parity SELECT di route GET).
var mpMediaListSelect = `SELECT m.id, m.name, m.slug, m.public_slug, m.public_listed, m.category, m.description, m.website_url, m.contoh_artikel_url, m.adapter_url, m.price, m.credit_cost, m.status, m.integration_type, m.publication_mode, m.partner_id, p.name AS partner_name, u.email AS partner_email, u.referred_by_partner_id, referrer.name AS referrer_name, m.publish_defaults, m.notes, m.traffic_estimate, m.media_tier, m.media_quality_score, m.media_quality_notes, m.health_score, m.health_breakdown, m.health_computed_at, m.auto_publish, m.word_count_mode, m.pending_changes, (m.pending_values IS NOT NULL) AS has_pending_values, m.pending_requested_at, m.dewan_pers_verified, m.dewan_pers_note, m.dewan_pers_cert_no, m.dewan_pers_doc_key, m.rejection_reason, m.reviewed_by, m.reviewed_at, m.created_by, m.created_at, m.updated_at, m.pricing_type, m.price_status, m.negotiated_price, m.link_duration_months, m.max_links, m.allow_hyperlink, m.link_rel, m.website_platform, m.label_sponsor, m.domain_verified, m.domain_verified_at,
       COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = m.id), m.category) AS categories_csv,
       sm.da AS seo_da, sm.ai_trust_score AS seo_ai_trust, sm.ai_citation AS seo_ai_citation, sm.checked_at AS seo_checked_at,
       COALESCE(np.wa_number, '') AS partner_wa_number, COALESCE(np.wa_consent, FALSE) AS partner_wa_consent,
       (SELECT COUNT(*) FROM media mm WHERE mm.partner_id = m.partner_id AND mm.status <> 'hidden') AS partner_media_count,
       (SELECT COUNT(*) FROM publications p WHERE p.media_id = m.id AND p.status = 'published') AS published_count,
       m.suspended_at, m.suspended_reason,
       COALESCE((SELECT v.level FROM media_violations v
                 WHERE v.media_id = m.id AND v.revoked_at IS NULL
                 ORDER BY v.created_at DESC LIMIT 1), 'none') AS violation_level,
       ` + dbpkg.MediaIntegrationConnectedSQL("m") + ` AS integration_connected
FROM media m
LEFT JOIN partners p ON p.id = m.partner_id
LEFT JOIN users u ON u.id = p.user_id
LEFT JOIN partners referrer ON referrer.id = u.referred_by_partner_id
LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
LEFT JOIN notification_preferences np ON np.user_id = p.user_id`

// mpPartnerWASet: mitra sudah punya nomor WA DAN menyetujui dihubungi.
func mpPartnerWASet(row rowMap) bool {
	return strings.TrimSpace(row.strOr("partner_wa_number", "")) != "" && row.boolOr("partner_wa_consent", false)
}

// mpScopedMedia: parity scopedMedia → NotFound "Media not found".
func (d *Deps) mpScopedMedia(ctx context.Context, mediaID string) (ptRow, error) {
	row, err := d.queryRow(ctx, "SELECT * FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Media not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// mpMediaError: pemetaan error parity mediaError (slug duplikat / partner hilang).
func mpMediaError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "uq_media_public_slug") {
		return errConflict("Slug publik sudah dipakai media lain")
	}
	if strings.Contains(msg, "duplicate") || strings.Contains(msg, "uq_media_slug") || strings.Contains(msg, "23505") {
		return errConflict("Media slug already exists")
	}
	if strings.Contains(msg, "fk_media_partner") || strings.Contains(msg, "23503") {
		return errNotFound("Partner not found")
	}
	return err
}

// ---------- GET /v1/platform/media ----------

func (d *Deps) MediaList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	scope := strings.TrimSpace(q.Get("scope"))
	partnerID := strings.TrimSpace(q.Get("partner_id"))
	integrationType := strings.TrimSpace(q.Get("integration_type"))
	// integration: connected | not_connected — media non-manual dengan/tanpa
	// koneksi aktif (definisi sama dengan katalog klien dan daftar mitra).
	integration := strings.TrimSpace(q.Get("integration"))
	autoPublish := strings.TrimSpace(q.Get("auto_publish"))
	// media_type: mainstream (media berita & publisher profesional) atau
	// blogger (blog personal / media independen).
	mediaType := strings.TrimSpace(q.Get("media_type"))

	if mediaType != "" && mediaType != "mainstream" && mediaType != "blogger" {
		writeErr(w, d, errUnprocessable("Invalid media_type filter"))
		return
	}

	if integration != "" && integration != "connected" && integration != "not_connected" {
		writeErr(w, d, errUnprocessable("Invalid integration filter"))
		return
	}
	if autoPublish != "" && autoPublish != "1" && autoPublish != "0" {
		writeErr(w, d, errUnprocessable("Invalid auto_publish filter"))
		return
	}
	if scope != "" && scope != "archive" && scope != "all" && scope != "recent" {
		writeErr(w, d, errUnprocessable("Invalid media scope filter"))
		return
	}
	if status != "" && !mpContains(mpMediaStatuses, status) {
		writeErr(w, d, errUnprocessable("Invalid media status filter"))
		return
	}
	search := strings.TrimSpace(q.Get("search"))
	// Filter & urutan skor SEO. Kosong = tidak memfilter; 0 tetap berarti
	// "minimal 0" (semua), sama seperti yang diharapkan UI.
	daMinRaw := strings.TrimSpace(q.Get("da_min"))
	geoMinRaw := strings.TrimSpace(q.Get("geo_min"))
	sortBy := strings.TrimSpace(q.Get("sort"))
	// recent_days: jendela tampil tab "Baru Ditambahkan". Dihitung di server
	// supaya tidak bergantung jam perangkat admin.
	recentDays := mpClampInt(q.Get("recent_days"), 7, 1, 90)
	limit := mpClampInt(q.Get("limit"), 100, 1, 500)
	offset := mpClampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	args := []any{}
	next := func(v any) string {
		args = append(args, v)
		return "$" + ptItoa(len(args))
	}
	switch integration {
	case "connected":
		where = append(where, "m.integration_type <> 'manual' AND ("+dbpkg.MediaIntegrationConnectedSQL("m")+")")
	case "not_connected":
		where = append(where, "m.integration_type <> 'manual' AND NOT ("+dbpkg.MediaIntegrationConnectedSQL("m")+")")
	}
	switch {
	case scope == "archive":
		where = append(where, "m.status = 'archived'")
	case scope == "recent":
		// Media yang baru disetujui: status aktif DAN disetujui dalam jendela
		// recentDays. reviewed_at diisi saat admin menekan Setujui, jadi ini
		// penanda "baru masuk direktori" — bukan created_at (mitra bisa
		// menyimpan draf berhari-hari sebelum diajukan).
		where = append(where, "m.status = 'active' AND m.reviewed_at IS NOT NULL AND m.reviewed_at >= NOW() - make_interval(days => "+next(recentDays)+")")
	case scope == "all":
		// hidden tetap tampil di daftar (hanya tidak muncul di katalog klien).
		where = append(where, "TRUE")
	case status == "pending_review":
		// Tab Pending juga memuat media aktif yang perubahannya menunggu admin.
		where = append(where, "(m.status = 'pending_review' OR (m.status = 'active' AND m.pending_values IS NOT NULL))")
	case status != "":
		where = append(where, "m.status = "+next(status))
	default:
		where = append(where, "m.status <> 'archived' AND m.status <> 'pending_review'")
	}
	if partnerID != "" {
		where = append(where, "m.partner_id = "+next(partnerID))
	}
	if integrationType != "" {
		where = append(where, "m.integration_type = "+next(integrationType))
	}
	if mediaType != "" {
		where = append(where, "m.media_type = "+next(mediaType))
	}
	if autoPublish == "1" {
		where = append(where, "m.auto_publish = TRUE")
	} else if autoPublish == "0" {
		where = append(where, "m.auto_publish = FALSE")
	}
	if daMinRaw != "" {
		v := mpClampInt(daMinRaw, 0, 0, 100)
		where = append(where, "sm.da >= "+next(v))
	}
	if geoMinRaw != "" {
		v := mpClampInt(geoMinRaw, 0, 0, 100)
		where = append(where, "sm.ai_trust_score >= "+next(v))
	}
	searchPH := ""
	if search != "" {
		pattern := "%" + search + "%"
		ph := next(pattern)
		searchPH = ph
		where = append(where, "(m.name ILIKE "+ph+" OR m.slug ILIKE "+ph+" OR m.category ILIKE "+ph+" OR m.website_url ILIKE "+ph+" OR p.name ILIKE "+ph+" OR u.email ILIKE "+ph+" OR m.partner_id::text ILIKE "+ph+")")
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}

	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM media m LEFT JOIN partners p ON p.id = m.partner_id LEFT JOIN users u ON u.id = p.user_id LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id "+clause, args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	// Urutan skor memakai kolom yang sudah di-LEFT JOIN (sm).
	//
	// Arah "terkecil → terbesar" menaruh media TANPA skor di paling atas
	// (NULLS FIRST), bukan di belakang: media yang belum punya DA adalah nilai
	// terendah yang sebenarnya (belum diukur), dan itulah yang biasanya ingin
	// dilihat admin saat memilih urutan ini — mereka yang perlu dicek, bukan
	// yang sudah punya angka kecil.
	//
	// Arah "terbesar → terkecil" tetap NULLS LAST: media berskor tinggi dulu,
	// media belum diukur di belakang.
	orderBy := "m.updated_at DESC"
	if scope == "recent" {
		// Tab "Baru Ditambahkan": yang paling baru disetujui tampil paling atas.
		orderBy = "m.reviewed_at DESC, m.name ASC"
	}
	// Pencarian juga mencocokkan nama/email pemilik. Bila nama mitra sama dengan
	// domainnya (mis. mitra bernama "Cirebonbagus.id"), seluruh media mitra itu
	// ikut cocok dan media yang dicari bisa terdorong ke halaman terakhir.
	// Karena itu hasil yang cocok pada media itu sendiri (nama/slug/domain)
	// diurutkan lebih dulu; urutan pilihan admin (sort) tetap dihormati.
	searchRank := ""
	if searchPH != "" && sortBy == "" {
		searchRank = "(CASE WHEN m.name ILIKE " + searchPH + " OR m.slug ILIKE " + searchPH + " OR m.website_url ILIKE " + searchPH + " THEN 0 ELSE 1 END), "
	}
	switch sortBy {
	case "da_desc":
		orderBy = "sm.da DESC NULLS LAST, m.name ASC"
	case "da_asc":
		orderBy = "sm.da ASC NULLS FIRST, m.name ASC"
	case "geo_desc":
		orderBy = "sm.ai_trust_score DESC NULLS LAST, m.name ASC"
	case "geo_asc":
		orderBy = "sm.ai_trust_score ASC NULLS FIRST, m.name ASC"
	// Trafik: kolomnya NOT NULL DEFAULT 0, jadi "terkecil" otomatis mulai dari
	// nol — termasuk media yang belum pernah cek (0). Nama dipakai sebagai
	// pemecah seri supaya urutan halaman stabil.
	case "traffic_desc":
		orderBy = "m.traffic_estimate DESC, m.name ASC"
	case "traffic_asc":
		orderBy = "m.traffic_estimate ASC, m.name ASC"
	case "name_asc":
		orderBy = "m.name ASC"
	}
	pageArgs := append(append([]any{}, args...), limit, offset)
	items, err := d.queryRows(r.Context(),
		mpMediaListSelect+" "+clause+" ORDER BY "+searchRank+orderBy+" LIMIT $"+ptItoa(len(args)+1)+" OFFSET $"+ptItoa(len(args)+2),
		pageArgs...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	markup := d.mpGetMarkupPercent(r.Context())
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		item := d.mpMediaJSON(row, markup)
		item["integration_connected"] = row.boolOr("integration_connected", false)
		item["seo_da"] = row.numPtr("seo_da")
		item["seo_ai_trust"] = row.numPtr("seo_ai_trust")
		item["seo_ai_citation"] = mpBoolPtrOrNil(row, "seo_ai_citation")
		item["seo_checked_at"] = row.anyOrNil("seo_checked_at")
		out = append(out, item)
	}

	total := 0
	if countRow != nil {
		total = ptRow(ptNormalizeMap(countRow)).intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"media": out, "total": total, "limit": limit, "offset": offset,
	})
}

// mpBoolPtrOrNil: parity `row.x == null ? null : Boolean(row.x)`.
func mpBoolPtrOrNil(row ptRow, key string) any {
	v, ok := row[key]
	if !ok || v == nil {
		return nil
	}
	if b, isBool := v.(bool); isBool {
		return b
	}
	return nil
}

// ---------- POST /v1/platform/media ----------

func (d *Deps) MediaCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)

	name, hasName := strField(body, "name")
	if !hasName || len(name) < 2 {
		writeErr(w, d, errUnprocessable("Too small: expected string to have >=2 characters"))
		return
	}
	if len(name) > 160 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=160 characters"))
		return
	}

	slug, hasSlug := strField(body, "slug")
	if hasSlug {
		if len(slug) > 160 || !mpSlugRe.MatchString(slug) {
			writeErr(w, d, errUnprocessable("Invalid string: must match pattern "+mpSlugRe.String()))
			return
		}
	} else {
		slug = mpSlugifyMediaName(name)
	}

	// Slug publik (data.tauthub.com/media/...) dipisah dari slug internal supaya
	// rename media tidak memutus URL yang sudah terindeks. Default = slug saat
	// dibuat; setelah itu hanya berubah bila admin mengirimkannya eksplisit.
	publicSlug := slug
	if v, has := strField(body, "public_slug"); has {
		if len(v) > 160 || !mpSlugRe.MatchString(v) {
			writeErr(w, d, errUnprocessable("Invalid public_slug"))
			return
		}
		publicSlug = v
	}
	publicListed := true
	if v, has := boolField(body, "public_listed"); has {
		publicListed = v
	}

	category, hasCategory := strField(body, "category")
	if hasCategory && len(category) > 120 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=120 characters"))
		return
	}
	description, hasDescription := strField(body, "description")
	if hasDescription && len(description) > 2000 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=2000 characters"))
		return
	}
	websiteURL, hasWebsite := strField(body, "website_url")
	if hasWebsite && len(websiteURL) > 500 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=500 characters"))
		return
	}
	adapterURL, hasAdapter := strField(body, "adapter_url")
	if hasAdapter && len(adapterURL) > 500 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=500 characters"))
		return
	}
	notes, hasNotes := strField(body, "notes")
	if hasNotes && len(notes) > 2000 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=2000 characters"))
		return
	}
	qualityNotes, hasQualityNotes := strField(body, "media_quality_notes")
	if hasQualityNotes && len(qualityNotes) > 500 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=500 characters"))
		return
	}

	price := 0.0
	if v, has := numField(body, "price"); has {
		if v < 0 || v > 1_000_000_000 {
			writeErr(w, d, errUnprocessable("Number must be less than or equal to 1000000000"))
			return
		}
		price = v
	}
	if v, has := intField(body, "credit_cost"); has {
		if v < 0 || v > 1_000_000 {
			writeErr(w, d, errUnprocessable("Number must be less than or equal to 1000000"))
			return
		}
	}
	if v, has := intField(body, "traffic_estimate"); has {
		if v < 0 || float64(v) > 10_000_000_000 {
			writeErr(w, d, errUnprocessable("Number must be less than or equal to 10000000000"))
			return
		}
	}
	if v, has := intField(body, "media_quality_score"); has {
		if v < 0 || v > 100 {
			writeErr(w, d, errUnprocessable("Number must be less than or equal to 100"))
			return
		}
	}

	status := "active"
	if v, has := strField(body, "status"); has {
		if !mpContains(mpMediaStatuses, v) {
			writeErr(w, d, errUnprocessable(mpZEnum(mpMediaStatuses)))
			return
		}
		status = v
	}
	integrationType := "manual"
	if v, has := strField(body, "integration_type"); has {
		if !mpValidIntegrationType(v) {
			writeErr(w, d, errUnprocessable(mpZEnum(mpMediaIntegrationTypes)))
			return
		}
		integrationType = v
	}
	mediaTier := "tier3"
	if v, has := strField(body, "media_tier"); has {
		if !mpContains(mpMediaTiers, v) {
			writeErr(w, d, errUnprocessable(mpZEnum(mpMediaTiers)))
			return
		}
		mediaTier = v
	}
	partnerID, hasPartner := strField(body, "partner_id")

	// Masa berlaku tautan media (kebijakan media): 0 = permanen, >0 = bulan.
	// Bawaan permanen — tautan media Tauthub dijaga tetap tayang.
	linkDuration := 0
	if v, has := intField(body, "link_duration_months"); has {
		if v < 0 || v > 120 {
			writeErr(w, d, errUnprocessable("Invalid link_duration_months"))
			return
		}
		linkDuration = v
	}

	newLinkPolicy, linkErr := mpParseLinkPolicy(body)
	if linkErr != nil {
		writeErr(w, d, linkErr)
		return
	}

	id := uuid.NewString()
	markup := d.mpGetMarkupPercent(r.Context())
	clientPrice := mpComputeClientPrice(price, markup)
	creditCost := mpCreditCostFor(clientPrice, d.mpGetRateIDR(r.Context()))

	var publishDefaults any
	if v, has := rawField(body, "publish_defaults"); has && v != nil {
		publishDefaults = mpJSONString(v)
	}

	if err := d.exec(r.Context(), `
		INSERT INTO media (id, name, slug, public_slug, public_listed, category, description, website_url, adapter_url, price, credit_cost, status, integration_type, partner_id, publish_defaults, notes, traffic_estimate, media_tier, media_quality_score, media_quality_notes, created_by, link_duration_months)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
		id, name, slug, publicSlug, publicListed, ptNullableArgStr(category, hasCategory), ptNullableArgStr(description, hasDescription),
		ptNullableArgStr(websiteURL, hasWebsite), ptNullableArgStr(adapterURL, hasAdapter), price, creditCost,
		status, integrationType, ptNullableArgStr(partnerID, hasPartner), publishDefaults,
		ptNullableArgStr(notes, hasNotes), mpIntArgDefault(body, "traffic_estimate", 0), mediaTier,
		mpNullableAnyIntFromBody(body, "media_quality_score"), ptNullableArgStr(qualityNotes, hasQualityNotes), user.ID,
		linkDuration); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}

	if err := d.mpApplyLinkPolicy(r.Context(), id, newLinkPolicy); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}

	writeAuditBestEffort(d, r.Context(), user.ID, "platform.media.created", "media", id)
	// Media baru langsung dicek DA/AI Trust dan trafiknya, tidak menunggu sweep
	// scheduler. Keduanya lewat satu pintu supaya guard kredensial berlaku.
	d.mpEnqueueMediaChecks(r.Context(), id, mpCheckAll, "media.created")
	created, err := d.queryRow(r.Context(), "SELECT * FROM media WHERE id = $1 LIMIT 1", id)
	if err != nil || created == nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	writeJSON(w, http.StatusCreated, d.mpMediaJSON(ptRow(ptNormalizeMap(created)), markup))
}

// mpParseLinkPolicy: baca max_links (0-3) dan label_sponsor dari body.
// Hanya field yang dikirim yang dianggap berubah.
type mpLinkPolicy struct {
	MaxLinks   int
	HasMax     bool
	Sponsor    string
	HasSponsor bool
	// Allow = allow_hyperlink. false dengan MaxLinks > 0 berarti media menerima
	// tautan hanya sebagai nofollow (artikel tidak ditolak, rel diberi otomatis).
	Allow    bool
	HasAllow bool
	// Rel tautan (dofollow|nofollow|sponsored); nofollow menyinkronkan Allow=false.
	LinkRel    string
	HasLinkRel bool
}

func mpParseLinkPolicy(body map[string]any) (mpLinkPolicy, error) {
	var p mpLinkPolicy
	p.MaxLinks, p.HasMax = intField(body, "max_links")
	if p.HasMax && (p.MaxLinks < 0 || p.MaxLinks > 3) {
		return mpLinkPolicy{}, errUnprocessable("Invalid max_links")
	}
	p.Sponsor, p.HasSponsor = strField(body, "label_sponsor")
	if p.HasSponsor && p.Sponsor != "tidak_ada" && p.Sponsor != "ada_sponsor" {
		return mpLinkPolicy{}, errUnprocessable("Invalid label_sponsor")
	}
	p.Allow, p.HasAllow = boolField(body, "allow_hyperlink")
	p.LinkRel, p.HasLinkRel = strField(body, "link_rel")
	if p.HasLinkRel {
		if p.LinkRel != "dofollow" && p.LinkRel != "nofollow" && p.LinkRel != "sponsored" {
			return mpLinkPolicy{}, errUnprocessable("Invalid link_rel")
		}
		p.Allow, p.HasAllow = p.LinkRel != "nofollow", true
	}
	return p, nil
}

// mpApplyLinkPolicy: simpan kebijakan tautan. Diperbarui terpisah dari UPDATE
// utama agar penomoran parameter di sana tidak bergeser. allow_hyperlink
// mengikuti max_links > 0, sama seperti di sisi mitra.
func (d *Deps) mpApplyLinkPolicy(ctx context.Context, mediaID string, p mpLinkPolicy) error {
	switch {
	case p.HasMax:
		allow := p.MaxLinks > 0
		if p.HasAllow && p.MaxLinks > 0 {
			allow = p.Allow
		}
		if err := d.exec(ctx,
			"UPDATE media SET max_links = $1, allow_hyperlink = $2, updated_at = NOW() WHERE id = $3",
			p.MaxLinks, allow, mediaID); err != nil {
			return err
		}
	case p.HasAllow:
		if err := d.exec(ctx,
			"UPDATE media SET allow_hyperlink = $1, updated_at = NOW() WHERE id = $2",
			p.Allow, mediaID); err != nil {
			return err
		}
	}
	if p.HasSponsor {
		if err := d.exec(ctx,
			"UPDATE media SET label_sponsor = $1, updated_at = NOW() WHERE id = $2",
			p.Sponsor, mediaID); err != nil {
			return err
		}
	}
	if p.HasLinkRel {
		if err := d.exec(ctx,
			"UPDATE media SET link_rel = $1, updated_at = NOW() WHERE id = $2",
			p.LinkRel, mediaID); err != nil {
			return err
		}
	}
	return nil
}

// mpSameDomain: bandingkan dua URL berdasarkan domainnya, bukan string mentah,
// supaya "https://a.com" dan "http://www.a.com/" dianggap sama dan tidak memicu
// pemeriksaan ulang yang tidak perlu.
func mpSameDomain(a, b string) bool {
	da, _ := mpDomainFromWebsite(a).(string)
	db, _ := mpDomainFromWebsite(b).(string)
	da = strings.ToLower(strings.TrimPrefix(da, "www."))
	db = strings.ToLower(strings.TrimPrefix(db, "www."))
	return da == db
}

var mpSlugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func writeAuditBestEffort(d *Deps, ctx context.Context, actor, action, targetType, targetID string) {
	if err := d.exec(ctx,
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, $2, $3, $4)",
		nullIfEmpty(actor), action, targetType, targetID); err != nil {
		d.logWarn("audit gagal", action, err)
	}
}

func mpIntArgDefault(body map[string]any, key string, def int) int {
	if v, has := intField(body, key); has {
		return v
	}
	return def
}

func mpNullableAnyIntFromBody(body map[string]any, key string) any {
	if v, has := intField(body, key); has {
		return v
	}
	return nil
}

// ---------- GET/PUT/DELETE /v1/platform/media/{mediaId} ----------

func (d *Deps) MediaGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.mpScopedMedia(r.Context(), chiParam(r, "mediaId"))
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	writeJSON(w, http.StatusOK, d.mpMediaJSON(row, d.mpGetMarkupPercent(r.Context())))
}

func (d *Deps) MediaUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	existing, err := d.mpScopedMedia(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	body := decodeJSON(r)

	name, hasName := strField(body, "name")
	if hasName && (len(name) < 2 || len(name) > 160) {
		writeErr(w, d, errUnprocessable("Invalid name"))
		return
	}
	slug, hasSlug := strField(body, "slug")
	if hasSlug && (len(slug) > 160 || !mpSlugRe.MatchString(slug)) {
		writeErr(w, d, errUnprocessable("Invalid slug"))
		return
	}
	category, hasCategory := strField(body, "category")
	if hasCategory && len(category) > 120 {
		writeErr(w, d, errUnprocessable("Invalid category"))
		return
	}
	description, hasDescription := strField(body, "description")
	if hasDescription && len(description) > 2000 {
		writeErr(w, d, errUnprocessable("Invalid description"))
		return
	}
	websiteURL, hasWebsite := strField(body, "website_url")
	if hasWebsite && len(websiteURL) > 500 {
		writeErr(w, d, errUnprocessable("Invalid website_url"))
		return
	}
	adapterURL, hasAdapter := strField(body, "adapter_url")
	if hasAdapter && len(adapterURL) > 500 {
		writeErr(w, d, errUnprocessable("Invalid adapter_url"))
		return
	}
	notes, hasNotes := strField(body, "notes")
	if hasNotes && len(notes) > 2000 {
		writeErr(w, d, errUnprocessable("Invalid notes"))
		return
	}
	qualityNotes, hasQualityNotes := strField(body, "media_quality_notes")
	if hasQualityNotes && len(qualityNotes) > 500 {
		writeErr(w, d, errUnprocessable("Invalid media_quality_notes"))
		return
	}

	price, hasPrice := numField(body, "price")
	if hasPrice && (price < 0 || price > 1_000_000_000) {
		writeErr(w, d, errUnprocessable("Invalid price"))
		return
	}
	creditCostOverride, hasCreditCost := intField(body, "credit_cost")
	if hasCreditCost && (creditCostOverride < 0 || creditCostOverride > 1_000_000) {
		writeErr(w, d, errUnprocessable("Invalid credit_cost"))
		return
	}
	traffic, hasTraffic := intField(body, "traffic_estimate")
	if hasTraffic && (traffic < 0 || float64(traffic) > 10_000_000_000) {
		writeErr(w, d, errUnprocessable("Invalid traffic_estimate"))
		return
	}
	qualityScore, hasQualityScore := intField(body, "media_quality_score")
	if hasQualityScore && (qualityScore < 0 || qualityScore > 100) {
		writeErr(w, d, errUnprocessable("Invalid media_quality_score"))
		return
	}

	status, hasStatus := strField(body, "status")
	if hasStatus && !mpContains(mpMediaStatuses, status) {
		writeErr(w, d, errUnprocessable(mpZEnum(mpMediaStatuses)))
		return
	}
	integrationType, hasIntegration := strField(body, "integration_type")
	if hasIntegration && !mpValidIntegrationType(integrationType) {
		writeErr(w, d, errUnprocessable(mpZEnum(mpMediaIntegrationTypes)))
		return
	}
	mediaTier, hasTier := strField(body, "media_tier")
	if hasTier && !mpContains(mpMediaTiers, mediaTier) {
		writeErr(w, d, errUnprocessable(mpZEnum(mpMediaTiers)))
		return
	}
	pricingType, hasPricingType := strField(body, "pricing_type")
	if hasPricingType && pricingType != "nett" && pricingType != "nego" {
		writeErr(w, d, errUnprocessable(mpZEnum([]string{"nett", "nego"})))
		return
	}
	partnerID, hasPartner := strField(body, "partner_id")
	// Masa berlaku tautan (kebijakan media): 0 = permanen, >0 = bulan.
	linkDuration, hasLinkDuration := intField(body, "link_duration_months")
	if hasLinkDuration && (linkDuration < 0 || linkDuration > 120) {
		writeErr(w, d, errUnprocessable("Invalid link_duration_months"))
		return
	}
	// Kebijakan tautan (sama dengan sisi mitra): jumlah backlink maksimal dan
	// label sponsor. allow_hyperlink mengikuti max_links > 0.
	linkPolicy, linkErr := mpParseLinkPolicy(body)
	if linkErr != nil {
		writeErr(w, d, linkErr)
		return
	}
	autoPublish, hasAutoPublish := boolField(body, "auto_publish")
	// Keterlihatan di katalog publik (data.tauthub.com) — hanya admin.
	publicListed, hasPublicListed := boolField(body, "public_listed")
	// Slug publik untuk URL yang sudah terindeks. Hanya berubah bila dikirim
	// eksplisit; rename media tidak boleh memutus tautan lama.
	publicSlug, hasPublicSlug := strField(body, "public_slug")
	if hasPublicSlug && (len(publicSlug) > 160 || !mpSlugRe.MatchString(publicSlug)) {
		writeErr(w, d, errUnprocessable("Invalid public_slug"))
		return
	}
	// DA manual: admin boleh mengisi Domain Authority sendiri saat deteksi
	// otomatis tidak menemukannya. Hanya divalidasi sebagai bilangan bulat 0-100.
	// Dikirim null = hapus nilai; tipe lain yang tak terbaca = tolak.
	daRaw, seoDAPresent := body["seo_da"]
	seoDAClear := seoDAPresent && daRaw == nil
	seoDA, hasSeoDA := intField(body, "seo_da")
	if seoDAPresent && !seoDAClear && !hasSeoDA {
		writeErr(w, d, errUnprocessable("Invalid seo_da"))
		return
	}
	if hasSeoDA && (seoDA < 0 || seoDA > 100) {
		writeErr(w, d, errUnprocessable("Invalid seo_da"))
		return
	}

	markup := d.mpGetMarkupPercent(r.Context())
	var creditCost any
	if hasCreditCost {
		creditCost = creditCostOverride
	}
	if hasPrice {
		clientPrice := mpComputeClientPrice(price, markup)
		creditCost = mpCreditCostFor(clientPrice, d.mpGetRateIDR(r.Context()))
	}
	var publishDefaults any
	if v, has := rawField(body, "publish_defaults"); has && v != nil {
		publishDefaults = mpJSONString(v)
	}

	if err := d.exec(r.Context(), `
		UPDATE media SET
		  name = COALESCE($1, name),
		  slug = COALESCE($2, slug),
		  category = COALESCE($3, category),
		  description = COALESCE($4, description),
		  website_url = COALESCE($5, website_url),
		  adapter_url = COALESCE($6, adapter_url),
		  price = COALESCE($7, price),
		  credit_cost = COALESCE($8, credit_cost),
		  status = COALESCE($9, status),
		  integration_type = COALESCE($10, integration_type),
		  partner_id = COALESCE($11, partner_id),
		  auto_publish = COALESCE($12, auto_publish),
		  publish_defaults = COALESCE($13, publish_defaults),
		  notes = COALESCE($14, notes),
		  traffic_estimate = COALESCE($15, traffic_estimate),
		  media_tier = COALESCE($16, media_tier),
		  media_quality_score = COALESCE($17, media_quality_score),
		  media_quality_notes = COALESCE($18, media_quality_notes),
			  pricing_type = COALESCE($19, pricing_type),
			  link_duration_months = COALESCE($20, link_duration_months),
			  public_listed = COALESCE($21, public_listed),
			  public_slug = COALESCE($22, public_slug),
			  price_status = CASE WHEN $23::text IS NOT NULL THEN 'final' ELSE price_status END,
			  updated_at = NOW()
		WHERE id = $24`,
		ptNullableArgStr(name, hasName), ptNullableArgStr(slug, hasSlug), ptNullableArgStr(category, hasCategory),
		ptNullableArgStr(description, hasDescription), ptNullableArgStr(websiteURL, hasWebsite), ptNullableArgStr(adapterURL, hasAdapter),
		mpNullableArgFloat(price, hasPrice), creditCost, ptNullableArgStr(status, hasStatus),
		ptNullableArgStr(integrationType, hasIntegration), ptNullableArgStr(partnerID, hasPartner),
		ptNullableArgBool(autoPublish, hasAutoPublish), publishDefaults, ptNullableArgStr(notes, hasNotes),
		ptNullableArgInt(traffic, hasTraffic), ptNullableArgStr(mediaTier, hasTier),
		ptNullableAnyInt(qualityScore, hasQualityScore), ptNullableArgStr(qualityNotes, hasQualityNotes),
		ptNullableArgStr(pricingType, hasPricingType), ptNullableArgInt(linkDuration, hasLinkDuration),
		ptNullableArgBool(publicListed, hasPublicListed), ptNullableArgStr(publicSlug, hasPublicSlug),
		ptNullableArgStr(pricingType, hasPricingType), mediaID); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	if err := d.mpApplyLinkPolicy(r.Context(), mediaID, linkPolicy); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}

	// ---------- domain berubah: arsipkan angka lama, lalu periksa ulang ----------
	// Ini kasus "domain baru" yang paling sering terlewat: media sudah ada,
	// tapi website_url-nya diganti. Tanpa penanganan di sini, DA/PA/AI Trust dan
	// angka trafik milik domain LAMA tetap tampil seolah milik domain baru.
	oldWebsite := existing.strOr("website_url", "")
	domainChanged := hasWebsite && !mpSameDomain(oldWebsite, websiteURL)
	if domainChanged {
		d.mpArchiveSeoMetricsForDomainChange(r.Context(), mediaID, oldWebsite, websiteURL)
		d.mpResetTrafficForDomainChange(r.Context(), mediaID)
		writeAuditBestEffort(d, r.Context(), user.ID, "media.domain_changed", "media", mediaID)
	}
	// Status menjadi aktif (termasuk pemulihan dari arsip/hidden) juga memicu
	// pemeriksaan: media baru kembali tampil di direktori dengan data lama.
	becameActive := hasStatus && status == "active" &&
		existing.strOr("status", "") != "active"
	switch {
	case domainChanged:
		d.mpEnqueueMediaChecks(r.Context(), mediaID, mpCheckAll, "media.domain_changed")
	case becameActive:
		d.mpEnqueueMediaChecks(r.Context(), mediaID, mpCheckAll, "media.activated")
	}

	// Simpan DA manual sebelum membaca ulang media, supaya nilainya ikut terlihat.
	if seoDAPresent {
		if seoDAClear {
			if err := d.mpClearManualDA(r.Context(), mediaID); err != nil {
				writeErr(w, d, mpMediaError(err))
				return
			}
		} else if err := d.mpSetManualDA(r.Context(), mediaID, seoDA); err != nil {
			writeErr(w, d, mpMediaError(err))
			return
		}
		writeAuditBestEffort(d, r.Context(), user.ID, "media.da_updated", "media", mediaID)
	}

	// Konfirmasi harga nego↔nett oleh Admin: harga final + catatan ke mitra.
	if hasPricingType {
		note := strings.TrimSpace(ptStringField(body, "price_change_note"))
		if err := d.exec(r.Context(), "UPDATE media SET rejection_reason = $1 WHERE id = $2", nullIfEmpty(note), mediaID); err != nil {
			writeErr(w, d, mpMediaError(err))
			return
		}
		if note != "" {
			_ = d.exec(r.Context(),
				"INSERT INTO media_price_history (id, media_id, action, note, changed_by) VALUES (gen_random_uuid(), $1, 'admin.price_confirmed', $2, $3)",
				mediaID, note, user.ID)
		}
		writeAuditBestEffort(d, r.Context(), user.ID, "media.price_confirmed", "media", mediaID)
	}

	nextStatus := existing.strOr("status", "")
	if hasStatus {
		nextStatus = status
	}
	switch {
	case (existing.strOr("status", "") == "archived" || existing.strOr("status", "") == "hidden") && nextStatus == "active":
		writeAuditBestEffort(d, r.Context(), user.ID, "media.restored", "media", mediaID)
	case nextStatus == "hidden":
		writeAuditBestEffort(d, r.Context(), user.ID, "media.hidden", "media", mediaID)
	case nextStatus == "archived":
		writeAuditBestEffort(d, r.Context(), user.ID, "media.archived", "media", mediaID)
	case hasStatus:
		writeAuditBestEffort(d, r.Context(), user.ID, "media.updated", "media", mediaID)
	}

	if (hasStatus && (status == "archived" || status == "hidden")) || hasPartner {
		d.mpExpireMediaNotifications(r.Context(), mediaID)
	}

	updated, err := d.mpScopedMedia(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	writeJSON(w, http.StatusOK, d.mpMediaJSON(updated, markup))
}

func (d *Deps) MediaDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	existing, err := d.mpScopedMedia(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	if s := existing.strOr("status", ""); s != "archived" && s != "hidden" {
		writeErr(w, d, errConflict("Media masih aktif — arsipkan dulu sebelum menghapus permanen."))
		return
	}

	usage, err := d.queryRow(r.Context(), `
		SELECT
		  (SELECT COUNT(*) FROM publications WHERE media_id = $1) AS publications,
		  (SELECT COUNT(*) FROM partner_earnings WHERE media_id = $1) AS earnings,
		  (SELECT COUNT(*) FROM publication_plan_occurrences WHERE media_id = $1) AS plan_occurrences`,
		mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	u := ptRow(ptNormalizeMap(usage))
	blocked := []string{}
	if u.intOr("publications", 0) > 0 {
		blocked = append(blocked, "publikasi")
	}
	if u.intOr("earnings", 0) > 0 {
		blocked = append(blocked, "earning")
	}
	if u.intOr("plan_occurrences", 0) > 0 {
		blocked = append(blocked, "rencana publikasi")
	}
	if len(blocked) > 0 {
		writeErr(w, d, errConflict("Tidak bisa dihapus permanen: media masih punya riwayat "+strings.Join(blocked, ", ")+"."))
		return
	}

	if err := d.exec(r.Context(), "DELETE FROM media WHERE id = $1", mediaID); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	writeAuditBestEffort(d, r.Context(), user.ID, "media.deleted", "media", mediaID)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func mpNullableArgFloat(v float64, has bool) any {
	if !has {
		return nil
	}
	return v
}

// ---------- GET /v1/platform/media/{mediaId}/audit ----------

func (d *Deps) MediaAudit(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.mpScopedMedia(r.Context(), mediaID); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	items, err := d.queryRows(r.Context(), `
		SELECT a.id, a.action, u.email AS actor_email, a.created_at, a.detail
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.target_type = 'media' AND a.target_id = $1
		ORDER BY a.created_at DESC
		LIMIT 100`, mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		// detail JSONB: kirim sebagai objek, bukan string ter-escape.
		var detail any
		if s := row.strOr("detail", ""); s != "" {
			var parsed any
			if json.Unmarshal([]byte(s), &parsed) == nil {
				detail = parsed
			}
		}
		out = append(out, map[string]any{
			"id":          row.anyOrNil("id"),
			"action":      row.anyOrNil("action"),
			"detail":      detail,
			"actor_email": row.anyOrNil("actor_email"),
			"created_at":  row.anyOrNil("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ---------- POST /v1/platform/media/{mediaId}/recompute-health ----------

func (d *Deps) MediaRecomputeHealth(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.mpScopedMedia(r.Context(), mediaID); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	score, breakdown, err := d.mpRefreshMediaHealth(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"media_id": mediaID, "health_score": score, "breakdown": breakdown,
	})
}

// ---------- POST /v1/platform/media/{mediaId}/review ----------

func (d *Deps) MediaReview(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.mpScopedMedia(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	if !media.has("partner_id") {
		writeErr(w, d, errConflict("Media tanpa partner tidak bisa direview lewat alur ini."))
		return
	}
	body := decodeJSON(r)
	decision, hasDecision := strField(body, "decision")
	if !hasDecision || (decision != "approve" && decision != "reject" && decision != "negotiate") {
		writeErr(w, d, errUnprocessable(mpZEnum([]string{"approve", "reject", "negotiate"})))
		return
	}
	reason, hasReason := strField(body, "reason")
	if hasReason && len(reason) > 1000 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=1000 characters"))
		return
	}
	if (decision == "reject" || decision == "negotiate") && reason == "" {
		writeErr(w, d, errUnprocessable("Alasan/catatan wajib diisi untuk aksi ini."))
		return
	}
	if decision == "negotiate" {
		// Aturan zod tetap dievaluasi lebih dulu (harga wajib), lalu fitur ini
		// menolak seluruh aksi negosiasi: harga ditentukan mitra.
		if _, hasNegotiated := numField(body, "negotiated_price"); !hasNegotiated {
			writeErr(w, d, errUnprocessable("Harga negosiasi wajib diisi."))
			return
		}
		writeErr(w, d, errUnprocessable("Negosiasi harga dinonaktifkan sementara — harga ditentukan mitra."))
		return
	}

	// Media aktif dengan perubahan yang diparkir mitra: keputusan hanya berlaku
	// untuk perubahan itu, status media (tayang) tidak berubah.
	if cur, _ := d.queryRow(r.Context(),
		"SELECT status, pending_values::text AS pv FROM media WHERE id = $1 LIMIT 1", mediaID); cur != nil {
		curRow := ptRow(ptNormalizeMap(cur))
		if pv := strings.TrimSpace(curRow.strOr("pv", "")); curRow.strOr("status", "") == "active" && pv != "" {
			d.mediaParkedChangesReview(w, r, user.ID, media, decision, reason, pv)
			return
		}
	}

	nextStatus := "rejected"
	nextPriceStatus := "ditolak"
	if decision == "approve" {
		// Media tanpa harga tidak boleh disetujui: begitu berstatus active ia
		// tampil di katalog klien sebagai "Rp 0" dan bisa dipesan, sehingga
		// biaya publikasinya 0 TC dan klien tidak pernah ditagih.
		if media.numOr("price") <= 0 {
			writeErr(w, d, errUnprocessable(
				"Media ini belum punya harga. Minta mitra mengisi harga dulu, atau tolak pengajuannya."))
			return
		}
		nextStatus, nextPriceStatus = "active", "disetujui"
	}
	priceStatus := media.strOr("price_status", "menunggu_review")
	mediaPrice := media.numOr("price")

	if err := d.exec(r.Context(), `
		UPDATE media SET status = $1, price_status = $2, rejection_reason = $3, reviewed_by = $4, reviewed_at = NOW(), pending_changes = NULL, updated_at = NOW()
		WHERE id = $5`,
		nextStatus, nextPriceStatus, nullIfEmpty(reason), user.ID, mediaID); err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	_ = d.exec(r.Context(), `
		INSERT INTO media_price_history (id, media_id, actor_id, from_price, to_price, from_price_status, to_price_status, note)
		VALUES (gen_random_uuid(), $1, $2, $3, $3, $4, $5, $6)`,
		mediaID, user.ID, mpNullableArgFloat(mediaPrice, media.has("price")), priceStatus, nextPriceStatus, nullIfEmpty(reason))

	action := "media.rejected"
	if decision == "approve" {
		action = "media.approved"
		// Pengajuan mitra yang belum punya metrik (mis. website_url diisi belakangan
		// atau pemeriksaan sebelumnya gagal) dicek saat disetujui — sekaligus
		// trafiknya, karena media baru masuk direktori pada momen ini.
		d.mpEnqueueMediaChecks(r.Context(), mediaID, mpCheckAll, "media.approved")
	}
	writeAuditBestEffort(d, r.Context(), user.ID, action, "media", mediaID)

	partnerRow, _ := d.queryRow(r.Context(),
		"SELECT public_id, name, user_id, (SELECT email FROM users WHERE id = user_id) AS email FROM partners WHERE id = $1 LIMIT 1",
		media.anyOrNil("partner_id"))
	partnerPublicID := ""
	partnerName := "Mitra"
	partnerEmail := ""
	partnerUserID := ""
	partnerID := media.strOr("partner_id", "")
	if partnerRow != nil {
		pr := ptRow(ptNormalizeMap(partnerRow))
		partnerPublicID = pr.strOr("public_id", "")
		if n := pr.strOr("name", ""); n != "" {
			partnerName = n
		}
		if e := pr.strOr("email", ""); e != "" {
			partnerEmail = e
		}
		partnerUserID = pr.strOr("user_id", "")
	}
	linkTarget := partnerPublicID
	if linkTarget == "" {
		linkTarget = partnerID
	}
	partnerLink := "/mitra/" + linkTarget + "/media"
	mediaName := media.strOr("name", "Media")

	if decision == "approve" {
		d.mpNotifyPartner(r.Context(), partnerID, "media.approved", "Media disetujui",
			`Media "`+mediaName+`" telah disetujui Admin dan dapat digunakan untuk distribusi.`, &partnerLink)
	} else {
		d.mpNotifyPartner(r.Context(), partnerID, "media.rejected", "Media ditolak",
			`Media "`+mediaName+`" ditolak Admin. Alasan: `+reason+`. Silakan perbaiki data lalu ajukan kembali.`, &partnerLink)
	}

	// Email + WhatsApp follow-up (best-effort; kegagalan tidak menggagalkan review).
	d.mpEmailFollowUp(r.Context(), action, partnerUserID, partnerEmail, mediaID, map[string]string{
		"media_name": mediaName,
		"domain":     strings.TrimSpace(media.strOr("website_url", "-")),
		"reason":     reason,
		"link":       partnerLink,
	})
	d.mpWAFollowUp(r.Context(), action, partnerUserID, mediaID, map[string]string{
		"nama": partnerName, "judul": mediaName, "status": reason, "link": partnerLink,
	})

	updated, err := d.mpScopedMedia(r.Context(), mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	writeJSON(w, http.StatusOK, d.mpMediaJSON(updated, d.mpGetMarkupPercent(r.Context())))
}

// ---------- GET/POST /v1/platform/media/{mediaId}/seo ----------

func (d *Deps) MediaSEO(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT id, website_url FROM media WHERE id = $1 AND status <> 'hidden' LIMIT 1", mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if media == nil {
		writeErr(w, d, errNotFound("Media tidak ditemukan"))
		return
	}
	mrow := ptRow(ptNormalizeMap(media))

	if r.Method == http.MethodPost {
		result, seoErr := d.mpCheckAndStoreSeoMetrics(r.Context(), mediaID, mrow.anyOrNil("website_url"), user.ID)
		if seoErr != nil {
			if mpErr, isMP := seoErr.(*mpSeoError); isMP {
				writeErr(w, d, ptErr(mpErr.Status, mpErr.Message))
				return
			}
			writeErr(w, d, seoErr)
			return
		}
		writeJSON(w, http.StatusOK, result.JSON())
		return
	}

	// GET: hanya subset kolom (parity rowToResult) — tanpa spam_score.
	metric, err := d.queryRow(r.Context(),
		"SELECT media_id, da, pa, ai_trust_score, ai_citation, ai_citation_checked_at, checked_at, source FROM media_seo_metrics WHERE media_id = $1 LIMIT 1",
		mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if metric == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"media_id": mediaID, "domain": nil, "da": nil, "pa": nil, "ai_trust_score": nil,
			"ai_citation": nil, "ai_citation_checked_at": nil, "checked_at": nil, "source": nil, "cached": true,
		})
		return
	}
	row := ptRow(ptNormalizeMap(metric))
	writeJSON(w, http.StatusOK, map[string]any{
		"media_id":               mediaID,
		"domain":                 mpDomainFromWebsite(mrow.anyOrNil("website_url")),
		"da":                     row.numPtr("da"),
		"pa":                     row.numPtr("pa"),
		"ai_trust_score":         row.numPtr("ai_trust_score"),
		"ai_citation":            row.anyOrNil("ai_citation"),
		"ai_citation_checked_at": row.anyOrNil("ai_citation_checked_at"),
		"checked_at":             row.anyOrNil("checked_at"),
		"source":                 row.anyOrNil("source"),
		"cached":                 true,
	})
}

// ---------- GET/POST /v1/platform/media/{mediaId}/similarweb ----------

func (d *Deps) MediaSimilarWeb(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")

	if r.Method == http.MethodPost {
		media, err := d.queryRow(r.Context(),
			"SELECT id, website_url FROM media WHERE id = $1 AND status <> 'hidden' LIMIT 1", mediaID)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		if media == nil {
			writeErr(w, d, errNotFound("Media tidak ditemukan"))
			return
		}
		result, swErr := d.mpCheckSimilarWebInsights(r.Context(), mediaID, ptRow(ptNormalizeMap(media)).anyOrNil("website_url"), user.ID, true)
		if swErr != nil {
			writeErr(w, d, ptErr(swErr.Status, swErr.Message))
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	media, err := d.queryRow(r.Context(),
		"SELECT id FROM media WHERE id = $1 AND status <> 'hidden' LIMIT 1", mediaID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if media == nil {
		writeErr(w, d, errNotFound("Media tidak ditemukan"))
		return
	}
	if snapshot := d.mpGetLatestSimilarWebSnapshot(r.Context(), mediaID); snapshot != nil {
		writeJSON(w, http.StatusOK, snapshot)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"media_id": mediaID, "insights": nil, "cached": true})
}

// ---------- GET /v1/platform/media/intelligence ----------

func mpParseBreakdown(value any) map[string]any {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return obj
}

func mpIntelligenceWarnings(row ptRow, breakdown map[string]any) []string {
	warnings := []string{}
	if row.strOr("status", "") != "active" {
		warnings = append(warnings, "status "+row.strOr("status", ""))
	}
	score := row.numPtr("health_score")
	if score != nil && score.(float64) < 40 {
		warnings = append(warnings, "health score rendah (<40)")
	}
	if breakdown != nil {
		if sr, ok := breakdown["success_rate"].(float64); ok && sr < 12 {
			warnings = append(warnings, "success rate rendah")
		}
		rejected := mpMaybeNum(breakdown["rejected"])
		published := mpMaybeNum(breakdown["published"])
		if published+rejected > 0 && rejected/(published+rejected) > 0.5 {
			warnings = append(warnings, "rasio penolakan tinggi")
		}
		// Nilai mentah: tampilan sudah dibatasi 72 jam, peringatan admin harus
		// tetap melihat angka sebenarnya.
		raw := breakdown["avg_publish_hours_raw"]
		if raw == nil {
			raw = breakdown["avg_publish_hours"]
		}
		if ah := mpMaybeNumPtr(raw); ah != nil && *ah > 72 {
			warnings = append(warnings, "publish lambat (>72 jam)")
		}
	}
	if row.numOr("traffic_estimate") == 0 {
		warnings = append(warnings, "traffic estimate belum diisi")
	}
	return warnings
}

func mpMaybeNum(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	}
	return 0
}

func mpMaybeNumPtr(v any) *float64 {
	if v == nil {
		return nil
	}
	f := mpMaybeNum(v)
	return &f
}

func (d *Deps) MediaIntelligence(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("search"))
	status := strings.TrimSpace(q.Get("status"))
	includeArchived := q.Get("include_archived") == "1"

	where := []string{}
	args := []any{}
	if search != "" {
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern)
		where = append(where, "(name ILIKE $1 OR category ILIKE $2)")
	}
	if status != "" && (status == "active" || status == "draft" || status == "inactive" || status == "archived") {
		args = append(args, status)
		where = append(where, "status = $"+ptItoa(len(args)))
	} else if !includeArchived {
		where = append(where, "status <> 'archived'")
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}

	items, err := d.queryRows(r.Context(), `
		SELECT id, name, slug, category, website_url, price, credit_cost, status, integration_type, partner_id,
		       traffic_estimate, media_tier, media_quality_score, health_score, health_breakdown, health_computed_at
		FROM media `+clause+`
		ORDER BY health_score ASC NULLS LAST, name ASC LIMIT 500`, args...)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}

	result := make([]map[string]any, 0, len(items))
	scored := 0
	unhealthy := 0
	withWarnings := 0
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		breakdown := mpParseBreakdown(row.anyOrNil("health_breakdown"))
		warnings := mpIntelligenceWarnings(row, breakdown)
		score := row.numPtr("health_score")
		if score != nil {
			scored++
			if score.(float64) < 40 {
				unhealthy++
			}
		}
		if len(warnings) > 0 {
			withWarnings++
		}
		result = append(result, map[string]any{
			"id":                  row.anyOrNil("id"),
			"name":                row.anyOrNil("name"),
			"slug":                row.anyOrNil("slug"),
			"category":            row.anyOrNil("category"),
			"website_url":         row.anyOrNil("website_url"),
			"price":               row.numOr("price"),
			"credit_cost":         row.intOr("credit_cost", 1),
			"status":              row.anyOrNil("status"),
			"integration_type":    row.strOr("integration_type", "manual"),
			"partner_id":          row.anyOrNil("partner_id"),
			"traffic_estimate":    row.numOr("traffic_estimate"),
			"media_tier":          row.strOr("media_tier", "tier3"),
			"media_quality_score": row.numPtr("media_quality_score"),
			"health_score":        score,
			"health_breakdown":    breakdown,
			"health_computed_at":  row.anyOrNil("health_computed_at"),
			"warnings":            warnings,
		})
	}

	archivedCount, _ := d.queryRow(r.Context(), "SELECT COUNT(*) AS total FROM media WHERE status = 'archived'")
	archived := 0
	if archivedCount != nil {
		archived = ptRow(ptNormalizeMap(archivedCount)).intOr("total", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"media": result,
		"summary": map[string]any{
			"total":         len(result),
			"scored":        scored,
			"unhealthy":     unhealthy,
			"with_warnings": withWarnings,
			"archived":      archived,
		},
	})
}
