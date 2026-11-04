package partner

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- media JSON (port mediaJson di partner/media/route.ts) ----------

const mediaListSelect = `SELECT m.id, m.name, m.slug, m.category, m.media_type, m.description, m.traffic_estimate, m.health_score, m.health_breakdown, m.website_url, m.adapter_url, m.custom_web_endpoint_url, m.custom_web_api_key_sealed, m.custom_web_status, m.custom_web_last_test_at, m.custom_web_last_test_message, m.blogger_credentials_sealed, m.blogger_blog_id, m.blogger_blog_name, m.blogger_status, m.blogger_last_test_at, m.blogger_last_test_message, m.price, m.pricing_type, m.price_status, m.negotiated_price, m.status,
               m.integration_type, m.website_platform, m.notes, m.publication_mode, m.bahasa_artikel, m.label_sponsor, m.word_count_mode, m.allow_embed, m.topik_terlarang, m.contoh_artikel_url, m.max_links, m.allow_hyperlink, m.link_rel, m.allow_heading, m.link_duration_months, m.auto_publish, m.dewan_pers_verified, m.dewan_pers_note, m.dewan_pers_cert_no, m.dewan_pers_doc_key, m.domain_verified, m.domain_verified_at, m.rejection_reason, m.reviewed_at, m.created_at, m.updated_at, m.ai_summary, m.ai_summary_generated_at,
               m.pending_changes, m.pending_values, m.pending_requested_at,
               COALESCE(
                 (SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = m.id),
                 m.category
               ) AS categories_csv,
               sm.da AS seo_da, sm.pa AS seo_pa, sm.spam_score AS seo_spam_score, sm.checked_at AS seo_checked_at,
               sm.ai_trust_score AS seo_ai_trust, sm.ai_citation AS seo_ai_citation, sm.ai_citation_checked_at,
               m.health_computed_at, m.media_tier, m.media_quality_score,
               (SELECT ROUND(AVG(r.rating)::numeric, 1)::float8 FROM media_ratings r
                  WHERE r.media_id = m.id) AS rating_avg,
               (SELECT COUNT(*)::int FROM media_ratings r
                  WHERE r.media_id = m.id) AS rating_count,
               sw.visits AS sw_visits, sw.created_at AS sw_snapshot_date,
               ct.articles_7d, ct.confidence AS crawl_confidence, ct.source AS crawl_source,
               ct.last_crawled_at, ct.last_status,
               (ct.last_crawled_at IS NOT NULL
                AND ct.last_crawled_at >= NOW() - make_interval(days => 7)) AS crawl_fresh,
              EXISTS (
                SELECT 1 FROM cms_connections c
                WHERE c.partner_id = m.partner_id
                  AND c.provider = m.integration_type
                  AND c.status = 'active'
                  AND m.website_url IS NOT NULL
                  AND regexp_replace(lower(split_part(split_part(c.base_url, '://', 2), '/', 1)), '^www\.', '')
                    = regexp_replace(lower(split_part(split_part(m.website_url, '://', 2), '/', 1)), '^www\.', '')
              ) AS has_connection,
              (SELECT COUNT(*) FROM publications p
                WHERE p.media_id = m.id AND p.status IN ('pending', 'waiting_partner')) AS pending_publications,
              (SELECT COUNT(*) FROM publications p2
                WHERE p2.media_id = m.id AND p2.status = 'published') AS total_terbit,
              (SELECT COALESCE(SUM(p2.client_price), 0) FROM publications p2
                WHERE p2.media_id = m.id AND p2.status = 'published') AS total_harga
        FROM media m
        LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
        LEFT JOIN LATERAL (SELECT CASE WHEN s.insights->>'visits' ~ '^-?[0-9]+([.][0-9]+)?$'
                                       THEN ((s.insights->>'visits')::numeric)::bigint END AS visits, s.created_at
                           FROM media_similarweb_snapshots s WHERE s.media_id = m.id ORDER BY s.created_at DESC LIMIT 1) sw ON TRUE
        LEFT JOIN crawl_targets ct ON ct.media_id = m.id`

// categoriesArray: kategori media dari CSV (pemisah E'\u0001'), dengan
// pemisahan tambahan pada koma untuk data lama yang menyimpan beberapa
// kategori dalam satu nilai (mis. "News, Ekonomi, Lifestyle").
func categoriesArray(row rowMap) []string {
	raw, _ := row.str("categories_csv")
	if strings.TrimSpace(raw) == "" {
		if cat, ok := row.str("category"); ok && cat != "" {
			return catutil.Split(cat)
		}
		return []string{}
	}
	return catutil.Split(raw)
}

func (d *Deps) mediaJSON(r *http.Request, row rowMap, markupPercent float64) map[string]any {
	if row == nil {
		return nil
	}
	partnerPrice := row.numOr("price")
	effectivePrice := partnerPrice
	if v, ok := row.num("negotiated_price"); ok {
		effectivePrice = v
	}
	categories := categoriesArray(row)
	var firstCategory any
	if len(categories) > 0 {
		firstCategory = categories[0]
	} else {
		firstCategory = row.raw("category")
	}
	breakdown := breakdownFor(effectivePrice, markupPercent)
	dp := d.dewanPersStatusFor(r.Context(), row.strOr("id", ""))
	// Content Velocity dari salinan crawl lokal. Angka hanya dianggap segar bila
	// crawl terakhir <7 hari DAN statusnya ok — sama dengan aturan modal klien.
	crawlFresh := row.boolOr("crawl_fresh", false)
	crawlLastStatus := row.strOr("last_status", "")
	crawlStale := !(crawlFresh && (crawlLastStatus == "" || crawlLastStatus == "ok"))

	payload := map[string]any{
		"id":                            row.raw("id"),
		"name":                          row.raw("name"),
		"slug":                          row.raw("slug"),
		"media_type":                    row.strOr("media_type", "mainstream"),
		"category":                      firstCategory,
		"categories":                    categories,
		"description":                   row.raw("description"),
		"ai_summary":                    row.raw("ai_summary"),
		"ai_summary_generated_at":       row.timePtr("ai_summary_generated_at"),
		"traffic_estimate":              row.numOr("traffic_estimate"),
		"health_score":                  row.raw("health_score"),
		"health_breakdown":              row.raw("health_breakdown"),
		"website_url":                   row.raw("website_url"),
		"adapter_url":                   row.raw("adapter_url"),
		"custom_web_endpoint_url":       row.raw("custom_web_endpoint_url"),
		"custom_web_status":             row.strOr("custom_web_status", "disconnected"),
		"custom_web_last_test_at":       row.timePtr("custom_web_last_test_at"),
		"custom_web_last_test_message":  row.raw("custom_web_last_test_message"),
		"custom_web_api_key_configured": row.has("custom_web_api_key_sealed"),
		"blogger_blog_id":               row.raw("blogger_blog_id"),
		"blogger_blog_name":             row.raw("blogger_blog_name"),
		"blogger_status":                row.strOr("blogger_status", "disconnected"),
		"blogger_last_test_at":          row.timePtr("blogger_last_test_at"),
		"blogger_last_test_message":     row.raw("blogger_last_test_message"),
		"blogger_connected":             row.has("blogger_credentials_sealed"),
		"price":                         partnerPrice,
		"negotiated_price":              row.numPtr("negotiated_price"),
		"effective_price":               effectivePrice,
		"markup_percent":                breakdown.MarkupPercent,
		"markup_amount":                 breakdown.MarkupAmount,
		"client_price":                  breakdown.ClientPrice,
		"pricing_type":                  row.strOr("pricing_type", "nett"),
		"price_status":                  row.strOr("price_status", "menunggu_review"),
		"pending_changes":               row.raw("pending_changes"),
		"pending_values":                row.raw("pending_values"),
		"pending_requested_at":          row.timePtr("pending_requested_at"),
		"publication_mode":              row.strOr("publication_mode", "draft"),
		"bahasa_artikel":                row.parseJSONArray("bahasa_artikel"),
		"label_sponsor":                 row.strOr("label_sponsor", "tidak_ada"),
		"word_count_mode":               normalizeWordCountMode(row.strPtr("word_count_mode")),
		"allow_embed":                   row.boolOr("allow_embed", false),
		"topik_terlarang":               row.parseJSONArray("topik_terlarang"),
		"contoh_artikel_url":            row.raw("contoh_artikel_url"),
		"max_links":                     row.numPtr("max_links"),
		"allow_hyperlink":               row.boolPtr("allow_hyperlink"),
		"link_rel":                      existingLinkRel(row),
		"allow_heading":                 row.boolPtr("allow_heading"),
		"link_duration_months":          row.intOr("link_duration_months", 12),
		"dewan_pers_verified":           row.boolOr("dewan_pers_verified", false),
		"dewan_pers_note":               row.raw("dewan_pers_note"),
		"dewan_pers_cert_no":            row.raw("dewan_pers_cert_no"),
		"dewan_pers_doc_key":            row.raw("dewan_pers_doc_key"),
		"domain_verified":               row.boolOr("domain_verified", false),
		"domain_verified_at":            row.timePtr("domain_verified_at"),
		"dewan_pers_status":             dp.Status,
		"dewan_pers_rejection_reason":   dp.RejectionReason,
		"dewan_pers_submitted_at":       dp.SubmittedAt,
		"dewan_pers_reviewed_at":        dp.ReviewedAt,
		"seo_da":                        row.raw("seo_da"),
		"seo_pa":                        row.raw("seo_pa"),
		"seo_spam_score":                row.raw("seo_spam_score"),
		"seo_checked_at":                row.timePtr("seo_checked_at"),
		// Metrik yang sama dengan yang dilihat klien di katalog (tab Transparansi).
		"seo_ai_trust":                  row.numPtr("seo_ai_trust"),
		"seo_ai_citation":               row.boolPtr("seo_ai_citation"),
		"seo_ai_citation_checked_at":    row.timePtr("seo_ai_citation_checked_at"),
		"sw_visits":                     row.numPtr("sw_visits"),
		"sw_snapshot_date":              row.timePtr("sw_snapshot_date"),
		"rating_avg":                    row.numPtr("rating_avg"),
		"rating_count":                  row.numPtr("rating_count"),
		"health_computed_at":            row.timePtr("health_computed_at"),
		"media_tier":                    row.raw("media_tier"),
		"media_quality_score":           row.numPtr("media_quality_score"),
		"published_count":               row.numOr("total_terbit"),
		"articles_7d":                   row.numPtr("articles_7d"),
		"crawl_confidence":              row.raw("crawl_confidence"),
		"crawl_source":                  row.raw("crawl_source"),
		"crawl_last_crawled_at":         row.timePtr("last_crawled_at"),
		"crawl_last_status":             row.raw("last_status"),
		"crawl_stale":                   crawlStale,
		"status":                        row.raw("status"),
		"integration_type":              row.raw("integration_type"),
		"website_platform":              row.raw("website_platform"),
		"has_connection":                row.boolOr("has_connection", false),
		"auto_publish":                  row.boolOr("auto_publish", false),
		"notes":                         row.raw("notes"),
		"total_terbit":                  row.numOr("total_terbit"),
		"total_harga":                   row.numOr("total_harga"),
		"pending_publications":          row.numOr("pending_publications"),
		"rejection_reason":              row.raw("rejection_reason"),
		"reviewed_at":                   row.timePtr("reviewed_at"),
		"created_at":                    row.timePtr("created_at"),
		"updated_at":                    row.timePtr("updated_at"),
	}
	return payload
}

// ---------- dewan pers status (port lib/media/dewan-pers.ts) ----------

type dewanPersStatus struct {
	Status          any
	CertificateNo   any
	RejectionReason any
	SubmittedAt     any
	ReviewedAt      any
}

func (d *Deps) dewanPersStatusFor(ctx context.Context, mediaID string) dewanPersStatus {
	row, err := d.queryRow(ctx, `
		SELECT status, certificate_no, rejection_reason, submitted_at, reviewed_at
		FROM media_dewan_pers_verifications
		WHERE media_id = $1
		ORDER BY CASE status WHEN 'pending' THEN 0 WHEN 'verified' THEN 1 WHEN 'conflict' THEN 2 WHEN 'rejected' THEN 3 ELSE 4 END, submitted_at DESC
		LIMIT 1`, mediaID)
	if err != nil || row == nil {
		return dewanPersStatus{}
	}
	return dewanPersStatus{
		Status:          row.raw("status"),
		CertificateNo:   row.raw("certificate_no"),
		RejectionReason: row.raw("rejection_reason"),
		SubmittedAt:     row.timePtr("submitted_at"),
		ReviewedAt:      row.timePtr("reviewed_at"),
	}
}

// ---------- GET/POST /v1/partner/media ----------

var mediaStatusFilters = map[string]bool{
	"active": true, "inactive": true, "draft": true, "archived": true,
	"pending_review": true, "rejected": true, "hidden": true,
}

func (d *Deps) MediaList(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !mediaStatusFilters[status] {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid media status filter")
		return
	}
	dewanPers := strings.TrimSpace(q.Get("dewan_pers"))
	if dewanPers != "" && dewanPers != "yes" && dewanPers != "no" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid dewan_pers filter (yes|no)")
		return
	}
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	clauses := []string{"m.partner_id = $1", "m.status <> 'hidden'"}
	args := []any{partner.ID}
	if status != "" {
		args = append(args, status)
		clauses = append(clauses, "m.status = $"+itoa(len(args)))
	} else if q.Get("exclude_archived") == "true" {
		clauses = append(clauses, "m.status <> 'archived'")
	}
	if dewanPers == "yes" {
		clauses = append(clauses, "m.dewan_pers_verified = TRUE")
	} else if dewanPers == "no" {
		clauses = append(clauses, "m.dewan_pers_verified = FALSE")
	}
	// Pencarian nama/domain di server: daftar dipaginasi, jadi mencari hanya di
	// halaman yang sedang termuat membuat media di halaman lain "tidak ada".
	if search := strings.TrimSpace(q.Get("search")); search != "" {
		if runes := []rune(search); len(runes) > 100 {
			search = string(runes[:100])
		}
		like := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)
		args = append(args, "%"+like+"%")
		ph := "$" + itoa(len(args))
		clauses = append(clauses, "(m.name ILIKE "+ph+" OR m.slug ILIKE "+ph+" OR m.website_url ILIKE "+ph+")")
	}
	where := "WHERE " + strings.Join(clauses, " AND ")

	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM media m "+where, args...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	pageArgs := append(append([]any{}, args...), limit, offset)
	items, err := d.queryRows(r.Context(),
		mediaListSelect+" "+where+" ORDER BY m.created_at DESC LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2),
		pageArgs...)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	categoryRows, err := d.queryRows(r.Context(),
		"SELECT DISTINCT category FROM media WHERE category IS NOT NULL AND TRIM(category) <> '' AND partner_id = $1 ORDER BY category ASC",
		partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	categories := make([]string, 0, len(categoryRows))
	for _, c := range categoryRows {
		categories = append(categories, c.strOr("category", ""))
	}

	markup := d.getMarkupPercent(r.Context())
	mediaItems := make([]map[string]any, 0, len(items))
	for _, row := range items {
		mediaItems = append(mediaItems, d.mediaJSON(r, row, markup))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"media":                mediaItems,
		"categories":           categories,
		"auto_publish_enabled": partner.AutoPublishEnabled,
		"bulk_unlocked":        d.bulkMediaUnlocked(r.Context(), partner.ID),
		"total":                countRow.intOr("count", 0),
		"limit":                limit,
		"offset":               offset,
	})
}

func (d *Deps) MediaCreate(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if err := d.checkAuthRateReq(r, "partner-media-create:"+user.ID, 5); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)

	name, present := strField(body, "name")
	if !present || len(name) < 2 || len(name) > 160 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input")
		return
	}
	categories, _ := stringSliceField(body, "categories")
	if len(categories) > 20 {
		d.writeErr(w, errConflict("Maksimal 20 kategori per media."))
		return
	}
	mediaType := "mainstream"
	if v, present := strField(body, "media_type"); present {
		if v != "blogger" && v != "mainstream" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid media_type")
			return
		}
		mediaType = v
	}
	integrationType := "manual"
	if v, present := strField(body, "integration_type"); present {
		if !validIntegrationType(v) {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid integration_type")
			return
		}
		integrationType = v
	}
	publicationMode := "draft"
	if v, present := strField(body, "publication_mode"); present {
		if v != "draft" && v != "publish" && v != "rss" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid publication_mode")
			return
		}
		publicationMode = v
	}
	labelSponsor := "tidak_ada"
	if v, present := strField(body, "label_sponsor"); present {
		if v != "tidak_ada" && v != "ada_sponsor" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid label_sponsor")
			return
		}
		labelSponsor = v
	}
	wordCountMode := defaultWordCountMode
	if v, present := strField(body, "word_count_mode"); present {
		if !isValidWordCountMode(v) {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid word_count_mode")
			return
		}
		wordCountMode = v
	}
	pricingType := "nett"
	if v, present := strField(body, "pricing_type"); present {
		if v != "nett" && v != "nego" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid pricing_type")
			return
		}
		pricingType = v
	}

	price := 0.0
	if v, present := numField(body, "price"); present {
		if v < 0 || v > 1_000_000_000 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid price")
			return
		}
		price = v
	}
	// Harga wajib. Media tanpa harga tampil di katalog klien sebagai "Rp 0" dan
	// bisa dipesan seolah gratis — biaya publikasinya jadi 0 TC, dan klien
	// menerima artikel tanpa pernah ditagih. Karena harga ditentukan mitra,
	// pengajuan tanpa harga harus ditolak di sini, bukan dibiarkan masuk
	// pending_review lalu disetujui admin tanpa angka.
	if price <= 0 {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Harga media wajib diisi (lebih dari 0) sebelum diajukan untuk review.")
		return
	}
	traffic := 0
	if v, present := intField(body, "traffic_estimate"); present {
		if v < 0 || float64(v) > 10_000_000_000 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid traffic_estimate")
			return
		}
		traffic = v
	}
	maxLinks := 1
	if v, present := intField(body, "max_links"); present {
		if v < 1 || v > 2 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid max_links")
			return
		}
		maxLinks = v
	}
	// Jenis tautan: false = nofollow (tautan tetap boleh, rel nofollow ditambah
	// otomatis saat ditulis dan saat terbit). Bawaan mengikuti max_links > 0.
	allowHyperlink := maxLinks > 0
	if v, present := boolField(body, "allow_hyperlink"); present && maxLinks > 0 {
		allowHyperlink = v
	}
	// Rel tautan terpisah dari label sponsor; nofollow menyinkronkan allow_hyperlink.
	linkRel := "dofollow"
	if !allowHyperlink {
		linkRel = "nofollow"
	}
	if v, present := strField(body, "link_rel"); present {
		if !validLinkRel(v) {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid link_rel")
			return
		}
		linkRel = v
		if maxLinks > 0 {
			allowHyperlink = v != "nofollow"
		}
	}
	// Masa berlaku tautan: 0 = permanen, >0 = bulan (bawaan permanen).
	linkDuration := 0
	if v, present := intField(body, "link_duration_months"); present {
		if v < 0 || v > 120 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid link_duration_months")
			return
		}
		linkDuration = v
	}
	bahasa, hasBahasa := stringSliceField(body, "bahasa_artikel")
	if !hasBahasa || len(bahasa) == 0 {
		bahasa = []string{"Indonesia"}
	}
	if len(bahasa) > 10 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid bahasa_artikel")
		return
	}
	topik, hasTopik := stringSliceField(body, "topik_terlarang")
	if hasTopik && len(topik) > 20 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid topik_terlarang")
		return
	}
	allowEmbed, _ := boolField(body, "allow_embed")
	description, _ := strField(body, "description")
	websiteURL, _ := strField(body, "website_url")
	notes, _ := strField(body, "notes")
	contohArtikel, _ := strField(body, "contoh_artikel_url")
	dewanPersNote, _ := strField(body, "dewan_pers_note")
	dewanPersCertNo, _ := strField(body, "dewan_pers_cert_no")
	dewanPersVerified, _ := boolField(body, "dewan_pers_verified")

	id := uuid.NewString()
	slug := slugifyMediaName(name)
	tcCfg := d.getTcConfig(r)
	markup := d.getMarkupPercent(r.Context())
	clientPrice := computeClientPrice(price, markup)
	creditCost := 0
	if clientPrice > 0 && tcCfg.RateIDR > 0 {
		// Parity: Math.max(1, Math.ceil(clientPrice / rate_idr)).
		creditCost = int(math.Ceil(clientPrice / tcCfg.RateIDR))
		if creditCost < 1 {
			creditCost = 1
		}
	}

	legacyCategory := any(nil)
	if len(categories) > 0 {
		legacyCategory = categories[0]
	} else if cat, present := strField(body, "category"); present && cat != "" {
		legacyCategory = cat
	}
	bahasaJSON := marshalJSONArr(bahasa)
	var topikJSON any
	if len(topik) > 0 {
		topikJSON = marshalJSONArr(topik)
	}

	// Menambah media kedua dan seterusnya wajib mengisi nomor WhatsApp + izin dihubungi.
	if d.waRequirementMissing(r.Context(), partner.UserID, partner.ID, 1) {
		writeWARequired(w)
		return
	}

	if err := d.exec(r.Context(), `
		INSERT INTO media (id, name, slug, category, media_type, description, traffic_estimate, max_links, allow_hyperlink, website_url, price, pricing_type, price_status, credit_cost, status, integration_type, partner_id, notes, publication_mode, bahasa_artikel, label_sponsor, word_count_mode, allow_embed, topik_terlarang, contoh_artikel_url, dewan_pers_verified, dewan_pers_note, dewan_pers_cert_no, created_by, link_duration_months, link_rel)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'menunggu_review', $13, 'pending_review', $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)`,
		id, name, slug, legacyCategory, mediaType, ptrOrNil(description), traffic, maxLinks, allowHyperlink,
		ptrOrNil(websiteURL), price, pricingType, creditCost, integrationType, partner.ID, ptrOrNil(notes),
		publicationMode, bahasaJSON, labelSponsor, wordCountMode, allowEmbed, topikJSON,
		ptrOrNil(contohArtikel), dewanPersVerified, ptrOrNil(dewanPersNote), ptrOrNil(dewanPersCertNo), user.ID, linkDuration, linkRel); err != nil {
		d.writeErr(w, err)
		return
	}

	if len(categories) > 0 {
		d.syncMediaCategories(r, id, categories)
	}
	d.persistMediaDomainCurated(r, id, user.ID, websiteURL)
	_ = d.exec(r.Context(),
		"INSERT INTO media_price_history (id, media_id, actor_id, from_price, to_price, from_pricing_type, to_pricing_type, from_price_status, to_price_status, note) VALUES (gen_random_uuid(), $1, $2, NULL, $3, NULL, $4, NULL, $5, $6)",
		id, user.ID, price, pricingType, "menunggu_review", "Pengajuan awal")
	d.auditLog(r.Context(), user.ID, "media.submitted", "media", id)
	// Pengajuan baru diperiksa metrik SEO/AI-nya lebih awal supaya admin sudah
	// punya DA/AI Trust saat menilai.
	d.enqueueSeoRefreshForMedia(r.Context(), id)
	link := "/admin/media?status=pending_review#tab-pending"
	d.notifyPlatformAdmins(r.Context(), "media.submitted", "Media baru menunggu review",
		`Mitra mengajukan media "`+name+`" untuk direview.`, &link)
	// WA ke admin (Fonnte): pengajuan media/domain tidak boleh menunggu admin
	// membuka dashboard.
	notify.EnqueueAdminsWA(r.Context(), d.Pool, notify.AdminWAOptions{
		Event: notify.AdminEventMediaSubmitted, ReferenceID: id,
		Data: map[string]string{"nama": derefOr(partner.Name, "Mitra"), "judul": name},
		Link: link, BaseURL: d.Cfg.PublicAppURL,
	})

	created, err := d.queryRow(r.Context(), mediaListSelect+" WHERE m.id = $1 LIMIT 1", id)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if created != nil {
		created["has_connection"] = false
		created["pending_publications"] = 0
	}
	writeJSON(w, http.StatusCreated, d.mediaJSON(r, created, markup))
}

func marshalJSONArr(items []string) string {
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// syncMediaCategories: port syncMediaCategories — upsert kategori + link.
// Nilai yang masih bergabung dengan koma dipisah lebih dulu supaya tersimpan
// sebagai kategori terpisah, bukan satu label gabungan.
func (d *Deps) syncMediaCategories(r *http.Request, mediaID string, names []string) {
	ctx := r.Context()
	seen := map[string]bool{}
	ids := []string{}
	for _, raw := range catutil.SplitAll(names) {
		name := raw
		normalized := strings.ToLower(name)
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		var categoryID string
		err := d.Pool.QueryRow(ctx,
			`INSERT INTO media_categories (id, name, normalized, created_at) VALUES (gen_random_uuid(), $1, $2, NOW())
			 ON CONFLICT (normalized) DO UPDATE SET name = media_categories.name
			 RETURNING id`, name, normalized).Scan(&categoryID)
		if err != nil {
			d.logger().Warn("sync kategori gagal", "media", mediaID, "err", err)
			return
		}
		ids = append(ids, categoryID)
	}
	if err := d.exec(ctx, "DELETE FROM media_category_links WHERE media_id = $1", mediaID); err != nil {
		d.logger().Warn("hapus link kategori gagal", "media", mediaID, "err", err)
		return
	}
	for _, categoryID := range ids {
		if err := d.exec(ctx,
			"INSERT INTO media_category_links (media_id, category_id, created_at) VALUES ($1, $2, NOW()) ON CONFLICT DO NOTHING",
			mediaID, categoryID); err != nil {
			d.logger().Warn("insert link kategori gagal", "media", mediaID, "err", err)
		}
	}
}

// persistMediaDomainCurated: port curateAndPersistMediaDomain (best-effort).
// Referral domain curation menyusul di fase referral lengkap; di sini hanya
// pencatatan domain yang sudah ada agar perilaku media tidak berubah.
func (d *Deps) persistMediaDomainCurated(r *http.Request, mediaID, userID, websiteURL string) {
	if strings.TrimSpace(websiteURL) == "" {
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE media SET updated_at = NOW() WHERE id = $1", mediaID); err != nil {
		return
	}
}
