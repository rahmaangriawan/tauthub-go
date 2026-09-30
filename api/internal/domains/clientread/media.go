package clientread

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// regexpReplaceCI: ganti pola di awal string (case-insensitive).
func regexpReplaceCI(s, pattern, repl string) string {
	return regexp.MustCompile("(?i)"+pattern).ReplaceAllString(s, repl)
}

// Port v1/brands/[brandId]/media/route.ts — katalog media aktif untuk klien.
// Read-only: hanya media status 'active', dengan harga klien (markup) & filter.

func parseMediaBahasa(value *string) []string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return []string{}
	}
	var arr []string
	if json.Unmarshal([]byte(*value), &arr) != nil {
		return []string{}
	}
	out := []string{}
	for _, v := range arr {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parseTopikTerlarang(value *string) []string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return []string{}
	}
	var arr []string
	if json.Unmarshal([]byte(*value), &arr) == nil {
		out := []string{}
		for _, s := range arr {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	out := []string{}
	for _, s := range strings.Split(*value, ",") {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// MediaCatalog: GET /v1/brands/{brandId}/media
func (d *Deps) MediaCatalog(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}

	q := r.URL.Query()
	category := strings.TrimSpace(q.Get("category"))
	search := strings.TrimSpace(q.Get("q"))
	if len(search) > 120 {
		search = search[:120]
	}
	seoFilter := strings.TrimSpace(q.Get("seo"))
	spamFilter := strings.TrimSpace(q.Get("spam"))
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	// Parity route lama (`/v1/brands/[brandId]/media/route.ts`): klausa persis
	// `WHERE media.status = 'active'`. Media arsip/nonaktif/hidden/pending_review
	// tidak pernah tampil — baik di katalog klien maupun di The Matchmaker yang
	// memakai endpoint yang sama.
	clause := "WHERE media.status = 'active'"
	args := []any{}
	addArg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if category != "" {
		norm := normalizeCategory(category)
		p1 := addArg(norm)
		p2 := addArg(norm)
		clause += " AND (EXISTS (SELECT 1 FROM media_category_links l JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = media.id AND c.normalized = " + p1 + ") OR LOWER(TRIM(media.category)) = " + p2 + ")"
	}
	if search != "" {
		bare := regexpReplaceCI(search, `^https?://`, "")
		bare = regexpReplaceCI(bare, `^www\.`, "")
		bare = strings.TrimRight(bare, "/")
		like := "%" + search + "%"
		likeBare := "%" + bare + "%"
		p1 := addArg(like)
		p2 := addArg(like)
		clause += " AND (name ILIKE " + p1 + " OR category ILIKE " + p2
		if likeBare != like {
			clause += " OR website_url ILIKE " + addArg(likeBare)
		}
		clause += ")"
	}
	if seoFilter == "checked" {
		clause += " AND sm.da IS NOT NULL"
	} else if seoFilter != "" {
		parts := strings.SplitN(seoFilter, ":", 2)
		if len(parts) == 2 && (parts[0] == "da" || parts[0] == "pa") {
			if min, err := strconv.Atoi(parts[1]); err == nil {
				clause += " AND sm." + parts[0] + " >= " + addArg(min)
			}
		}
	}
	if spamFilter == "low" {
		clause += " AND sm.spam_score IS NOT NULL AND sm.spam_score <= 30"
	} else if spamFilter == "high" {
		clause += " AND sm.spam_score IS NOT NULL AND sm.spam_score > 60"
	}

	var total int
	_ = d.Pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM media "+clause, args...).Scan(&total)

	limitArg := addArg(limit)
	offsetArg := addArg(offset)
	rowsQ, err := d.Pool.Query(r.Context(), `
		SELECT media.id, name, category, description, website_url, contoh_artikel_url, price, credit_cost,
		       integration_type, traffic_estimate, media_tier, media_quality_score, health_score, health_breakdown,
		       media.health_computed_at, media.auto_publish, dewan_pers_verified, dewan_pers_note, dewan_pers_cert_no,
		       domain_verified, media.status, media.created_at, media.media_type, media.bahasa_artikel,
		       media.label_sponsor, media.topik_terlarang, media.max_links, media.allow_hyperlink,
		       media.publication_mode, media.word_count_mode, media.allow_embed, media.pricing_type, media.price_status,
		       COALESCE((SELECT string_agg(c.name, E'\u0001' ORDER BY c.name) FROM media_category_links l
		                 JOIN media_categories c ON c.id = l.category_id WHERE l.media_id = media.id), media.category) AS categories_csv,
		       sm.da, sm.pa, sm.spam_score, sm.checked_at, sm.ai_trust_score, sm.ai_citation,
		       mcs.image_required AS standard_image_required,
		       EXISTS (SELECT 1 FROM gsc_connections g WHERE g.partner_id = media.partner_id AND g.status = 'active') AS gsc_connected,
		       sw.visits AS sw_visits, sw.created_at AS sw_snapshot_date
		FROM media
		LEFT JOIN media_seo_metrics sm ON sm.media_id = media.id
		LEFT JOIN media_content_standards mcs ON mcs.media_id = media.id AND mcs.status = 'active'
		LEFT JOIN LATERAL (SELECT s.insights->>'visits' AS visits, s.created_at FROM media_similarweb_snapshots s WHERE s.media_id = media.id ORDER BY s.created_at DESC LIMIT 1) sw ON TRUE
		`+clause+`
		ORDER BY media.media_tier NULLS LAST, media.name
		LIMIT `+limitArg+` OFFSET `+offsetArg, args...)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()

	markup := d.getMarkupPercent(r)
	// Baseline kualitas per tier (pr_media_rules.tier_default_quality) — dipakai
	// saat media belum punya skor sendiri. Default sama dengan app lama.
	tierDefaults := map[string]float64{"tier1": 90, "tier2": 80, "tier3": 70}
	var tierRaw []byte
	if rErr := d.Pool.QueryRow(r.Context(),
		"SELECT tier_default_quality FROM pr_media_rules WHERE id = 'default' LIMIT 1").Scan(&tierRaw); rErr == nil && len(tierRaw) > 0 {
		if m, ok := jsonRaw(tierRaw, nil).(map[string]any); ok && len(m) > 0 {
			for k, v := range m {
				if f, ok := v.(float64); ok {
					tierDefaults[k] = f
				}
			}
		}
	}
	items := []map[string]any{}
	for rowsQ.Next() {
		var id, name, status string
		var category, description, websiteURL, contohArtikelURL, integrationType, mediaTier, dewanPersNote, dewanPersCertNo *string
		var price float64
		var creditCost int
		var trafficEstimate int64
		var qualityScore, healthScore *int
		var healthBreakdown []byte
		var healthComputedAt *time.Time
		var autoPublish, dewanPersVerified, domainVerified, allowHyperlink, allowEmbed bool
		var createdAt time.Time
		var mediaType, bahasaArtikel, labelSponsor, topikTerlarang, publicationMode, wordCountMode, pricingType, priceStatus *string
		var maxLinks *int
		var categoriesCSV *string
		var seoDA, seoPA, seoSpam, seoAITrust *int
		var seoCheckedAt *time.Time
		var seoAICitation *bool
		var standardImageRequired *bool
		var gscConnected bool
		var swVisits *int64
		var swSnapshotDate *time.Time
		if rowsQ.Scan(&id, &name, &category, &description, &websiteURL, &contohArtikelURL, &price, &creditCost,
			&integrationType, &trafficEstimate, &mediaTier, &qualityScore, &healthScore, &healthBreakdown,
			&healthComputedAt, &autoPublish, &dewanPersVerified, &dewanPersNote, &dewanPersCertNo,
			&domainVerified, &status, &createdAt, &mediaType, &bahasaArtikel,
			&labelSponsor, &topikTerlarang, &maxLinks, &allowHyperlink,
			&publicationMode, &wordCountMode, &allowEmbed, &pricingType, &priceStatus,
			&categoriesCSV, &seoDA, &seoPA, &seoSpam, &seoCheckedAt, &seoAITrust, &seoAICitation,
			&standardImageRequired, &gscConnected, &swVisits, &swSnapshotDate) != nil {
			continue
		}
		// Pemisahan ganda: pemisah internal dari string_agg, plus koma untuk
		// data lama yang menyimpan beberapa kategori dalam satu nilai.
		categories := []string{}
		if categoriesCSV != nil {
			categories = catutil.Split(*categoriesCSV)
		}
		pricing := priceBreakdown(price, markup)
		// Kualitas efektif: skor media (admin) bila ada; jika tidak, baseline tier
		// dari pr_media_rules — parity computeEffectiveQuality app lama.
		var effectiveQuality any
		qualitySource := ""
		if qualityScore != nil {
			effectiveQuality = *qualityScore
			qualitySource = "media"
		} else if tq, ok := tierDefaults[derefStrOr(mediaTier, "")]; ok {
			effectiveQuality = tq
			qualitySource = "tier_default"
		}
		item := map[string]any{
			"id": id, "name": name, "category": category, "description": description,
			"website_url": websiteURL, "contoh_artikel_url": contohArtikelURL,
			"price": price, "credit_cost": creditCost, "integration_type": integrationType,
			"traffic_estimate": trafficEstimate, "media_tier": mediaTier,
			"media_quality_score": qualityScore, "health_score": healthScore,
			"health_breakdown":   jsonRaw(healthBreakdown, map[string]any{}),
			"health_computed_at": tsOrNil(healthComputedAt),
			"auto_publish":       autoPublish, "dewan_pers_verified": dewanPersVerified,
			"dewan_pers_note": dewanPersNote, "dewan_pers_cert_no": dewanPersCertNo,
			"domain_verified": domainVerified, "status": status, "created_at": isoMilli(createdAt),
			"media_type": mediaType, "bahasa_artikel": parseMediaBahasa(bahasaArtikel),
			"label_sponsor": labelSponsor, "topik_terlarang": parseTopikTerlarang(topikTerlarang),
			"max_links": maxLinks, "allow_hyperlink": allowHyperlink,
			"publication_mode": publicationMode, "word_count_mode": wordCountMode, "allow_embed": allowEmbed,
			"pricing_type": pricingType, "price_status": priceStatus,
			"categories":     categories,
			"markup_percent": pricing["markup_percent"],
			"markup_amount":  pricing["markup_amount"],
			"client_price":   pricing["client_price"],
			// Bentuk flat dipertahankan untuk parity kontrak app lama (UI membaca
			// m.seo_da, m.seo_ai_trust, m.sw_visits, ...), plus bentuk nested `seo`
			// yang sudah dipakai halaman lain.
			"seo_da": seoDA, "seo_pa": seoPA, "seo_spam_score": seoSpam,
			"seo_checked_at": tsOrNil(seoCheckedAt), "seo_ai_trust": seoAITrust,
			"seo_ai_citation": seoAICitation,
			"seo": map[string]any{
				"da": seoDA, "pa": seoPA, "spam_score": seoSpam,
				"checked_at": tsOrNil(seoCheckedAt), "ai_trust_score": seoAITrust, "ai_citation": seoAICitation,
			},
			"effective_quality": effectiveQuality, "quality_source": strOrNil(qualitySource),
			"sw_visits": swVisits, "sw_snapshot_date": tsOrNil(swSnapshotDate),
			"gsc_connected":       gscConnected,
			"image_required":      standardImageRequired,
			"is_available":        status == "active",
			"availability_reason": availabilityReason(status),
		}
		items = append(items, item)
	}

	// "media" adalah nama kunci kontrak app lama (UI & flow wizard membacanya);
	// "items" dipertahankan agar pemakai lama tidak rusak.
	writeJSON(w, http.StatusOK, map[string]any{
		"media": items, "items": items, "total": total, "limit": limit, "offset": offset,
	})
}

// MediaCategories: daftar kategori untuk filter (read-only).
func (d *Deps) MediaCategories(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	rowsQ, err := d.Pool.Query(r.Context(), "SELECT id, name FROM media_categories ORDER BY name ASC LIMIT 500")
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rowsQ.Close()
	items := []map[string]any{}
	for rowsQ.Next() {
		var id, name string
		if rowsQ.Scan(&id, &name) == nil {
			items = append(items, map[string]any{"id": id, "name": name})
		}
	}
	writeJSON(w, http.StatusOK, items)
}

// MediaDetail: GET /v1/brands/{brandId}/media/{mediaId} — detail 1 media aktif.
func (d *Deps) MediaDetail(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	mediaID := chiParam(r, "mediaId")
	var id, name, status string
	var category, description, websiteURL, mediaType *string
	var price float64
	var creditCost int
	var trafficEstimate int64
	var mediaTier *string
	var maxLinks *int
	var allowHyperlink bool
	var publicationMode, wordCountMode *string
	var labelSponsor, topikTerlarang *string
	var dewanPersVerified, domainVerified bool
	var dewanPersNote, dewanPersCertNo *string
	err := d.Pool.QueryRow(r.Context(), `
		SELECT id, name, status, category, description, website_url, price, credit_cost, traffic_estimate,
		       media_tier, max_links, allow_hyperlink, publication_mode, word_count_mode, media_type,
		       label_sponsor, topik_terlarang, dewan_pers_verified, dewan_pers_note, dewan_pers_cert_no, domain_verified
		FROM media WHERE id = $1 AND status = 'active' LIMIT 1`, mediaID).
		Scan(&id, &name, &status, &category, &description, &websiteURL, &price, &creditCost, &trafficEstimate,
			&mediaTier, &maxLinks, &allowHyperlink, &publicationMode, &wordCountMode, &mediaType,
			&labelSponsor, &topikTerlarang, &dewanPersVerified, &dewanPersNote, &dewanPersCertNo, &domainVerified)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Media tidak ditemukan")
		return
	}
	pricing := priceBreakdown(price, d.getMarkupPercent(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "name": name, "status": status, "category": category, "description": description,
		"website_url": websiteURL, "price": price, "credit_cost": creditCost,
		"traffic_estimate": trafficEstimate, "media_tier": mediaTier, "max_links": maxLinks,
		"allow_hyperlink": allowHyperlink, "publication_mode": publicationMode, "word_count_mode": wordCountMode,
		"media_type": mediaType, "label_sponsor": labelSponsor, "topik_terlarang": parseTopikTerlarang(topikTerlarang),
		"dewan_pers_verified": dewanPersVerified, "dewan_pers_note": dewanPersNote, "dewan_pers_cert_no": dewanPersCertNo,
		"domain_verified": domainVerified,
		"markup_percent":  pricing["markup_percent"], "markup_amount": pricing["markup_amount"],
		"client_price": pricing["client_price"],
	})
}

func tsOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return isoMilli(*t)
}

// derefStrOr: *string → string ("" bila nil).
func derefStrOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}

// strOrNil: string kosong → nil.
func strOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// availabilityReason: alasan media tidak bisa dipilih (parity app lama).
func availabilityReason(status string) any {
	if status == "pending_review" {
		return "Menunggu Review Admin"
	}
	return nil
}
