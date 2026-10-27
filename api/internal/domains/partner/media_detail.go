package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/imageconvert"
)

// mediaDetailSelect: kolom media untuk GET/PUT detail (parity ownedMedia).
const mediaDetailSelect = mediaListSelect

// ownedMedia: parity ownedMedia(partnerId, mediaId) → 404 "Media tidak ditemukan".
func (d *Deps) ownedMedia(r *http.Request, partnerID, mediaID, notFoundMsg string) (rowMap, error) {
	row, err := d.queryRow(r.Context(),
		mediaDetailSelect+" WHERE m.id = $1 AND m.partner_id = $2 AND m.status <> 'hidden'", mediaID, partnerID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound(notFoundMsg)
	}
	return row, nil
}

// ---------- GET/PUT/DELETE /v1/partner/media/{mediaId} ----------

func (d *Deps) MediaGet(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	row, err := d.ownedMedia(r, partner.ID, chiParam(r, "mediaId"), "Media tidak ditemukan")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d.mediaJSON(r, row, d.getMarkupPercent(r.Context())))
}

func (d *Deps) MediaUpdate(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if err := d.checkAuthRateReq(r, "partner-media-update:"+user.ID, 30); err != nil {
		d.writeErr(w, err)
		return
	}
	existing, err := d.ownedMedia(r, partner.ID, mediaID, "Media tidak ditemukan")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)

	// Mode otomatis hanya untuk mitra yang diizinkan admin.
	if autoPublish, present := boolField(body, "auto_publish"); present && autoPublish && !partner.AutoPublishEnabled {
		httpx.Error(w, http.StatusForbidden, "Mode otomatis belum diaktifkan untuk akun Anda. Hubungi admin Tauthub.")
		return
	}
	if existing.strOr("price_status", "") == "final" {
		_, hasPrice := numField(body, "price")
		_, hasPricingType := strField(body, "pricing_type")
		if hasPrice || hasPricingType {
			d.writeErr(w, errConflict("Harga sudah final (disepakati). Hubungi Admin untuk perubahan selanjutnya."))
			return
		}
	}
	if existing.strOr("status", "") == "rejected" {
		if _, present := strField(body, "status"); present {
			d.writeErr(w, errConflict("Media yang ditolak tidak bisa diaktifkan langsung. Perbaiki data lalu klik 'Ajukan Kembali' agar direview ulang Admin."))
			return
		}
	}

	// --- validasi & nilai opsional ---
	name, hasName := strField(body, "name")
	if hasName && (len(name) < 2 || len(name) > 160) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid name")
		return
	}
	category, hasCategory := strField(body, "category")
	if hasCategory && len(category) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid category")
		return
	}
	categories, hasCategories := stringSliceField(body, "categories")
	if hasCategories && len(categories) > 20 {
		d.writeErr(w, errConflict("Maksimal 20 kategori per media."))
		return
	}
	mediaType, hasMediaType := strField(body, "media_type")
	if hasMediaType && mediaType != "blogger" && mediaType != "mainstream" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid media_type")
		return
	}
	description, hasDescription := strField(body, "description")
	websiteURL, hasWebsiteURL := strField(body, "website_url")
	adapterURL, hasAdapterURL := strField(body, "adapter_url")
	customWebEndpoint, hasCustomWebEndpoint := strField(body, "custom_web_endpoint_url")
	notes, hasNotes := strField(body, "notes")
	contohArtikel, hasContohArtikel := strField(body, "contoh_artikel_url")
	dewanPersDocKey, hasDewanPersDocKey := strField(body, "dewan_pers_doc_key")
	status, hasStatus := strField(body, "status")
	if hasStatus && status != "active" && status != "inactive" && status != "pending_review" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid status")
		return
	}
	integrationType, hasIntegrationType := strField(body, "integration_type")
	if hasIntegrationType && !validIntegrationType(integrationType) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid integration_type")
		return
	}
	publicationMode, hasPublicationMode := strField(body, "publication_mode")
	if hasPublicationMode && publicationMode != "draft" && publicationMode != "publish" && publicationMode != "rss" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid publication_mode")
		return
	}
	bahasa, hasBahasa := stringSliceField(body, "bahasa_artikel")
	if hasBahasa && (len(bahasa) < 1 || len(bahasa) > 10) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid bahasa_artikel")
		return
	}
	labelSponsor, hasLabelSponsor := strField(body, "label_sponsor")
	if hasLabelSponsor && labelSponsor != "tidak_ada" && labelSponsor != "ada_sponsor" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid label_sponsor")
		return
	}
	wordCountMode, hasWordCountMode := strField(body, "word_count_mode")
	if hasWordCountMode && !isValidWordCountMode(wordCountMode) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid word_count_mode")
		return
	}
	allowEmbed, hasAllowEmbed := boolField(body, "allow_embed")
	topik, hasTopik := stringSliceField(body, "topik_terlarang")
	if hasTopik && len(topik) > 20 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid topik_terlarang")
		return
	}
	price, hasPrice := numField(body, "price")
	if hasPrice && (price < 0 || price > 1_000_000_000) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid price")
		return
	}
	traffic, hasTraffic := intField(body, "traffic_estimate")
	if hasTraffic && (traffic < 0 || float64(traffic) > 10_000_000_000) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid traffic_estimate")
		return
	}
	maxLinks, hasMaxLinks := intField(body, "max_links")
	if hasMaxLinks && (maxLinks < 1 || maxLinks > 2) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid max_links")
		return
	}
	// Masa berlaku tautan: 0 = permanen, >0 = bulan.
	linkDuration, hasLinkDuration := intField(body, "link_duration_months")
	if hasLinkDuration && (linkDuration < 0 || linkDuration > 120) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid link_duration_months")
		return
	}
	autoPublish, hasAutoPublish := boolField(body, "auto_publish")
	pricingType, hasPricingType := strField(body, "pricing_type")
	if hasPricingType && pricingType != "nett" && pricingType != "nego" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid pricing_type")
		return
	}

	changed := false
	if hasName && name != existing.strOr("name", "") {
		changed = true
	}
	if hasCategory && category != existing.strOr("category", "") {
		changed = true
	}
	if hasCategories {
		changed = true
	}
	if hasMediaType && mediaType != existing.strOr("media_type", "mainstream") {
		changed = true
	}
	if hasDescription && description != existing.strOr("description", "") {
		changed = true
	}
	if hasWebsiteURL && websiteURL != existing.strOr("website_url", "") {
		changed = true
	}
	if hasAdapterURL && adapterURL != existing.strOr("adapter_url", "") {
		changed = true
	}
	if hasPrice && price != existing.numOr("price") {
		changed = true
	}
	if hasTraffic && traffic != existing.intOr("traffic_estimate", 0) {
		changed = true
	}
	if hasMaxLinks {
		existingMax := existing.intOr("max_links", 1)
		if maxLinks != existingMax {
			changed = true
		}
	}
	if hasLinkDuration && linkDuration != existing.intOr("link_duration_months", 12) {
		changed = true
	}
	if hasIntegrationType && integrationType != existing.strOr("integration_type", "") {
		changed = true
	}
	if hasNotes && notes != existing.strOr("notes", "") {
		changed = true
	}
	if hasPublicationMode && publicationMode != existing.strOr("publication_mode", "draft") {
		changed = true
	}
	if hasBahasa && marshalJSONArr(bahasa) != marshalJSONArr(existing.parseJSONArray("bahasa_artikel")) {
		changed = true
	}
	if hasLabelSponsor && labelSponsor != existing.strOr("label_sponsor", "tidak_ada") {
		changed = true
	}
	if hasTopik && marshalJSONArr(topik) != marshalJSONArr(existing.parseJSONArray("topik_terlarang")) {
		changed = true
	}
	if hasPricingType && pricingType != existing.strOr("pricing_type", "nett") {
		changed = true
	}
	if hasDewanPersDocKey && dewanPersDocKey != existing.strOr("dewan_pers_doc_key", "") {
		changed = true
	}

	isRestoringArchived := existing.strOr("status", "") == "archived"
	requiresReview := existing.strOr("status", "") == "active" && changed
	var nextStatus any
	switch {
	case isRestoringArchived:
		nextStatus = "pending_review"
	case requiresReview:
		nextStatus = "pending_review"
	case hasStatus:
		nextStatus = status
	default:
		nextStatus = nil
	}

	var slug any
	if hasName {
		slug = slugifyMediaName(name)
	}
	tcCfg := d.getTcConfig(r)
	markup := d.getMarkupPercent(r.Context())
	var creditCost any
	if hasPrice {
		clientPrice := computeClientPrice(price, markup)
		cost := 0
		if clientPrice > 0 && tcCfg.RateIDR > 0 {
			cost = int(clientPrice/tcCfg.RateIDR) + 1
			if cost < 1 {
				cost = 1
			}
		}
		creditCost = cost
	}

	var bahasaVal any
	if hasBahasa {
		bahasaVal = marshalJSONArr(bahasa)
	}
	var topikVal any
	if hasTopik && len(topik) > 0 {
		topikVal = marshalJSONArr(topik)
	}
	allowEmbedVal := any(nil)
	if hasAllowEmbed {
		allowEmbedVal = allowEmbed
	}
	autoPublishVal := any(nil)
	if hasAutoPublish {
		autoPublishVal = autoPublish
	}

	if err := d.exec(r.Context(), `
		UPDATE media SET
		  name = COALESCE($1, name),
		  slug = COALESCE($2, slug),
		  category = COALESCE($3, category),
		  media_type = COALESCE($4, media_type),
		  description = COALESCE($5, description),
		  website_url = COALESCE($6, website_url),
		  adapter_url = COALESCE($7, adapter_url),
		  custom_web_endpoint_url = COALESCE($8, custom_web_endpoint_url),
		  price = COALESCE($9, price),
		  credit_cost = COALESCE($10, credit_cost),
		  traffic_estimate = COALESCE($11, traffic_estimate),
		  max_links = COALESCE($12, max_links),
		  status = COALESCE($13, status),
		  rejection_reason = CASE WHEN $14 = 'pending_review' THEN NULL ELSE rejection_reason END,
		  reviewed_by = CASE WHEN $14 = 'pending_review' THEN NULL ELSE reviewed_by END,
		  reviewed_at = CASE WHEN $14 = 'pending_review' THEN NULL ELSE reviewed_at END,
		  integration_type = COALESCE($15, integration_type),
		  notes = COALESCE($16, notes),
		  publication_mode = COALESCE($17, publication_mode),
		  bahasa_artikel = COALESCE($18, bahasa_artikel),
		  label_sponsor = COALESCE($19, label_sponsor),
		  word_count_mode = COALESCE($20, word_count_mode),
		  allow_embed = COALESCE($21, allow_embed),
		  topik_terlarang = COALESCE($22, topik_terlarang),
		  contoh_artikel_url = COALESCE($23, contoh_artikel_url),
		  dewan_pers_doc_key = COALESCE($24, dewan_pers_doc_key),
		  auto_publish = COALESCE($25, auto_publish),
		  pricing_type = COALESCE($26, pricing_type),
		  link_duration_months = COALESCE($27, link_duration_months),
		  updated_at = NOW()
		WHERE id = $28 AND partner_id = $29`,
		nullIfUnset(hasName, name), nullIfUnset(hasName, slug),
		nullIfUnset(hasCategory, category), nullIfUnset(hasMediaType, mediaType),
		nullIfUnset(hasDescription, description), nullIfUnset(hasWebsiteURL, websiteURL),
		nullIfUnset(hasAdapterURL, adapterURL), nullIfUnset(hasCustomWebEndpoint, customWebEndpoint),
		nullIfUnset(hasPrice, price), creditCost,
		nullIfUnset(hasTraffic, traffic), nullIfUnset(hasMaxLinks, maxLinks),
		nextStatus, nextStatus,
		nullIfUnset(hasIntegrationType, integrationType), nullIfUnset(hasNotes, notes),
		nullIfUnset(hasPublicationMode, publicationMode), bahasaVal,
		nullIfUnset(hasLabelSponsor, labelSponsor), nullIfUnset(hasWordCountMode, wordCountMode),
		allowEmbedVal, topikVal, nullIfUnset(hasContohArtikel, contohArtikel),
		nullIfUnset(hasDewanPersDocKey, dewanPersDocKey), autoPublishVal,
		nullIfUnset(hasPricingType, pricingType), nullIfUnset(hasLinkDuration, linkDuration),
		mediaID, partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}

	if hasMaxLinks {
		if err := d.exec(r.Context(),
			"UPDATE media SET allow_hyperlink = $1, updated_at = NOW() WHERE id = $2 AND partner_id = $3",
			maxLinks > 0, mediaID, partner.ID); err != nil {
			d.writeErr(w, err)
			return
		}
	}

	// Sinkron kategori + kolom legacy media.category.
	if hasCategories || hasMediaType {
		if hasCategories {
			if err := d.exec(r.Context(),
				"UPDATE media SET category = $1, media_type = COALESCE($2, media_type), updated_at = NOW() WHERE id = $3 AND partner_id = $4",
				nullableCategory(categories), nullIfUnset(hasMediaType, mediaType), mediaID, partner.ID); err != nil {
				d.writeErr(w, err)
				return
			}
			d.syncMediaCategories(r, mediaID, categories)
		} else {
			_ = d.exec(r.Context(),
				"UPDATE media SET media_type = $1, updated_at = NOW() WHERE id = $2 AND partner_id = $3",
				mediaType, mediaID, partner.ID)
		}
	}

	if hasDewanPersDocKey && dewanPersDocKey == "" {
		_ = d.exec(r.Context(),
			"UPDATE media SET dewan_pers_doc_key = NULL, updated_at = NOW() WHERE id = $1 AND partner_id = $2",
			mediaID, partner.ID)
	}

	// Satu sumber kebenaran: daftar perubahan dipakai untuk snapshot review
	// (media.pending_changes) sekaligus riwayat permanen di audit_logs.
	changes := pendingChangesList(body, existing)
	pending := any(nil)
	if len(changes) > 0 {
		if b, err := json.Marshal(changes); err == nil {
			pending = string(b)
		}
	}
	if err := d.exec(r.Context(),
		"UPDATE media SET pending_changes = $1::jsonb, updated_at = NOW() WHERE id = $2 AND partner_id = $3",
		pending, mediaID, partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	// Riwayat: satu entri audit per field yang berubah, lengkap dengan
	// before/after, supaya admin bisa menelusuri "apa yang diajukan mitra"
	// kapan pun (tidak hilang setelah pengajuan disetujui).
	for _, c := range changes {
		d.auditLogDetail(r.Context(), user.ID, "media.setting_changed", "media", mediaID, map[string]any{
			"field": c.Field,
			"label": c.Label,
			"from":  c.From,
			"to":    c.To,
		})
	}

	if hasPublicationMode {
		row, _ := d.queryRow(r.Context(), "SELECT publication_mode FROM media WHERE id = $1 AND partner_id = $2 LIMIT 1", mediaID, partner.ID)
		if row != nil && row.strOr("publication_mode", "") != existing.strOr("publication_mode", "") {
			d.auditLog(r.Context(), user.ID, "media.publication_mode_changed", "media", mediaID)
		}
	}

	if hasPrice || hasPricingType {
		toPrice := any(existing.numOr("price"))
		if hasPrice {
			toPrice = price
		}
		toPricingType := existing.strOr("pricing_type", "nett")
		if hasPricingType {
			toPricingType = pricingType
		}
		_ = d.exec(r.Context(),
			`INSERT INTO media_price_history (id, media_id, actor_id, from_price, to_price, from_pricing_type, to_pricing_type, from_price_status, to_price_status, note)
			 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			mediaID, user.ID, existing.numPtr("price"), toPrice,
			existing.strOr("pricing_type", "nett"), toPricingType,
			existing.strOr("price_status", "menunggu_review"), existing.strOr("price_status", "menunggu_review"),
			"Edit media oleh mitra")
	}

	switch {
	case requiresReview:
		d.auditLog(r.Context(), user.ID, "media.submitted", "media", mediaID)
		link := "/admin/media?status=pending_review#tab-pending"
		d.notifyPlatformAdmins(r.Context(), "media.submitted", "Media baru menunggu persetujuan",
			`Media "`+existing.strOr("name", "")+`" diedit dan kembali ke review Admin.`, &link)
	case hasStatus && status == "active":
		d.auditLog(r.Context(), user.ID, "media.restored", "media", mediaID)
	default:
		d.auditLog(r.Context(), user.ID, "media.updated", "media", mediaID)
	}

	updated, err := d.ownedMedia(r, partner.ID, mediaID, "Media tidak ditemukan")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d.mediaJSON(r, updated, markup))
}

// nullIfUnset: nil saat field tidak dikirim (parity COALESCE($n, kolom)).
func nullIfUnset(present bool, value any) any {
	if !present {
		return nil
	}
	return value
}

func nullableCategory(categories []string) any {
	if len(categories) == 0 {
		return nil
	}
	return catutil.Canonical(categories[0])
}

// mediaChange: satu baris perubahan konfigurasi media. Bentuknya sengaja sama
// dengan yang dibaca tabel "Perubahan" di panel admin: {field, label, from, to}
// sehingga admin melihat apa yang berubah, bukan objek mentah.
type mediaChange struct {
	Field string `json:"field"`
	Label string `json:"label"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// pendingChangesList: bandingkan body pengajuan mitra dengan data tersimpan.
// Cakupannya sengaja LENGKAP — apa pun yang bisa diubah mitra pada konfigurasi
// medianya harus muncul di panel admin (termasuk masa berlaku tautan).
func pendingChangesList(body map[string]any, existing rowMap) []mediaChange {
	changes := make([]mediaChange, 0, 6)
	add := func(field, label, from, to string) {
		if strings.TrimSpace(from) == strings.TrimSpace(to) {
			return
		}
		changes = append(changes, mediaChange{Field: field, Label: label, From: from, To: to})
	}

	if v, ok := strField(body, "name"); ok {
		add("name", "Nama Media", existing.strOr("name", ""), v)
	}
	if v, ok := strField(body, "media_type"); ok {
		add("media_type", "Tipe Media", mediaTypeLabel(existing.strOr("media_type", "mainstream")), mediaTypeLabel(v))
	}
	if cats, ok := stringSliceField(body, "categories"); ok {
		add("categories", "Kategori", strings.Join(categoriesArray(existing), ", "), strings.Join(cats, ", "))
	} else if v, ok := strField(body, "category"); ok {
		add("category", "Kategori", existing.strOr("category", ""), v)
	}
	if v, ok := strField(body, "website_url"); ok {
		add("website_url", "Website", existing.strOr("website_url", ""), v)
	}
	if v, ok := strField(body, "adapter_url"); ok {
		add("adapter_url", "URL Adapter", existing.strOr("adapter_url", ""), v)
	}
	if v, ok := strField(body, "custom_web_endpoint_url"); ok {
		add("custom_web_endpoint_url", "Endpoint Web Custom", existing.strOr("custom_web_endpoint_url", ""), v)
	}
	if v, ok := strField(body, "description"); ok {
		add("description", "Deskripsi", existing.strOr("description", ""), v)
	}
	if v, ok := strField(body, "notes"); ok {
		add("notes", "Catatan", existing.strOr("notes", ""), v)
	}
	if v, ok := strField(body, "integration_type"); ok {
		add("integration_type", "Tipe Integrasi", existing.strOr("integration_type", ""), v)
	}
	if v, ok := strField(body, "publication_mode"); ok {
		add("publication_mode", "Mode Publikasi",
			publicationModeLabel(existing.strOr("publication_mode", "draft")), publicationModeLabel(v))
	}
	if v, ok := strField(body, "label_sponsor"); ok {
		add("label_sponsor", "Label Sponsor",
			sponsorLabel(existing.strOr("label_sponsor", "tidak_ada")), sponsorLabel(v))
	}
	if v, ok := strField(body, "word_count_mode"); ok {
		add("word_count_mode", "Panjang Artikel",
			wordCountModeLabel(existing.strOr("word_count_mode", "sesuai_permintaan")), wordCountModeLabel(v))
	}
	if v, ok := strField(body, "contoh_artikel_url"); ok {
		add("contoh_artikel_url", "Contoh Artikel", existing.strOr("contoh_artikel_url", ""), v)
	}
	if v, ok := stringSliceField(body, "bahasa_artikel"); ok {
		add("bahasa_artikel", "Bahasa Artikel",
			joinOrNone(existing.parseJSONArray("bahasa_artikel")), joinOrNone(v))
	}
	if v, ok := stringSliceField(body, "topik_terlarang"); ok {
		add("topik_terlarang", "Topik Dilarang",
			joinOrNone(existing.parseJSONArray("topik_terlarang")), joinOrNone(v))
	}
	if v, ok := intField(body, "max_links"); ok {
		add("max_links", "Batas Link", linkLimitLabel(existing.intOr("max_links", 1)), linkLimitLabel(v))
	}
	// Masa berlaku tautan: 0 = permanen, >0 = bulan. Inilah perubahan yang
	// paling sering luput terlihat admin sebelum ringkasan ini diperbaiki.
	if v, ok := intField(body, "link_duration_months"); ok {
		add("link_duration_months", "Masa Berlaku Tautan",
			linkDurationLabel(existing.intOr("link_duration_months", 12)), linkDurationLabel(v))
	}
	if v, ok := numField(body, "price"); ok {
		add("price", "Harga", fmtThousands(existing.numOr("price")), fmtThousands(v))
	}
	if v, ok := strField(body, "pricing_type"); ok {
		add("pricing_type", "Model Harga", pricingTypeLabel(existing.strOr("pricing_type", "nett")), pricingTypeLabel(v))
	}
	if v, ok := intField(body, "traffic_estimate"); ok {
		add("traffic_estimate", "Estimasi Trafik",
			fmtThousands(float64(existing.intOr("traffic_estimate", 0))), fmtThousands(float64(v)))
	}
	if v, ok := boolField(body, "allow_embed"); ok {
		add("allow_embed", "Dukung Embed", yesNo(existing.boolOr("allow_embed", false)), yesNo(v))
	}
	if v, ok := boolField(body, "auto_publish"); ok {
		add("auto_publish", "Mode Otomatis", yesNo(existing.boolOr("auto_publish", false)), yesNo(v))
	}
	if v, ok := strField(body, "dewan_pers_doc_key"); ok {
		add("dewan_pers_doc_key", "Dokumen Dewan Pers",
			docPresenceLabel(existing.strOr("dewan_pers_doc_key", "")), docPresenceLabel(v))
	}
	return changes
}

// ---------- label bantuan untuk ringkasan perubahan ----------

func mediaTypeLabel(v string) string {
	switch v {
	case "blogger":
		return "Blogger"
	case "mainstream":
		return "Mainstream"
	}
	return v
}

func publicationModeLabel(v string) string {
	switch v {
	case "publish":
		return "Langsung terbit"
	case "rss":
		return "RSS"
	case "draft":
		return "Draf (diterbitkan media)"
	}
	return v
}

func sponsorLabel(v string) string {
	switch v {
	case "ada_sponsor":
		return "Ada label sponsor/adv"
	case "tidak_ada":
		return "Tidak ada (organik)"
	}
	return v
}

func wordCountModeLabel(v string) string {
	switch v {
	case "kata_300_700":
		return "300–700 kata"
	case "kata_800_1500":
		return "800–1500 kata"
	case "sesuai_permintaan":
		return "Sesuai permintaan klien"
	}
	if strings.HasPrefix(v, "custom_") {
		parts := strings.Split(strings.TrimPrefix(v, "custom_"), "_")
		if len(parts) == 2 {
			return parts[0] + "–" + parts[1] + " kata"
		}
	}
	return v
}

func pricingTypeLabel(v string) string {
	switch v {
	case "nett":
		return "Nett"
	case "nego":
		return "Nego"
	}
	return v
}

func linkLimitLabel(v int) string {
	if v <= 0 {
		return "Tanpa backlink"
	}
	return fmt.Sprintf("%d link", v)
}

// linkDurationLabel: 0 = permanen, >0 = jumlah bulan ("12 bulan").
func linkDurationLabel(months int) string {
	if months <= 0 {
		return "Permanen"
	}
	return fmt.Sprintf("%d bulan", months)
}

func yesNo(v bool) string {
	if v {
		return "Ya"
	}
	return "Tidak"
}

func docPresenceLabel(v string) string {
	if strings.TrimSpace(v) == "" {
		return "Tidak ada"
	}
	return "Ada dokumen"
}

func joinOrNone(v []string) string {
	clean := make([]string, 0, len(v))
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			clean = append(clean, strings.TrimSpace(s))
		}
	}
	if len(clean) == 0 {
		return "Tidak ada"
	}
	return strings.Join(clean, ", ")
}

// fmtThousands: 50000 → "50.000" (pemisah ribuan gaya Indonesia).
func fmtThousands(v float64) string {
	whole := int64(math.Round(v))
	sign := ""
	if whole < 0 {
		sign = "-"
		whole = -whole
	}
	digits := strconv.FormatInt(whole, 10)
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return sign + b.String()
}

// auditLogDetail: catat perubahan konfigurasi media beserta detail before/after
// supaya panel admin punya riwayat permanen (bukan hanya snapshot pending).
func (d *Deps) auditLogDetail(ctx context.Context, userID, action, targetType, targetID string, detail any) {
	payload, err := json.Marshal(detail)
	if err != nil {
		return
	}
	if _, err := d.Pool.Exec(ctx,
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, detail) VALUES ($1, $2, $3, $4, $5, $6::jsonb)",
		uuid.NewString(), userID, action, targetType, targetID, string(payload)); err != nil {
		slog.Warn("[partner] audit detail gagal", "action", action, "err", err)
	}
}

func (d *Deps) MediaDelete(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	existing, err := d.ownedMedia(r, partner.ID, mediaID, "Media tidak ditemukan")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if existing.strOr("status", "") == "archived" {
		httpx.Error(w, http.StatusConflict, "Media sudah diarsipkan")
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE media SET status = 'archived', updated_at = NOW() WHERE id = $1 AND partner_id = $2",
		mediaID, partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.expireMediaNotifications(r.Context(), mediaID)
	d.auditLog(r.Context(), user.ID, "media.archived", "media", mediaID)
	updated, err := d.ownedMedia(r, partner.ID, mediaID, "Media tidak ditemukan")
	if err != nil {
		// Media baru saja diarsipkan tidak lagi lolos filter status <> 'hidden';
		// fallback query tanpa filter agar respons tetap lengkap.
		updated, err = d.queryRow(r.Context(), mediaDetailSelect+" WHERE m.id = $1 AND m.partner_id = $2", mediaID, partner.ID)
		if err != nil || updated == nil {
			d.writeErr(w, errNotFound("Media tidak ditemukan"))
			return
		}
	}
	writeJSON(w, http.StatusOK, d.mediaJSON(r, updated, d.getMarkupPercent(r.Context())))
}

// ---------- POST /v1/partner/media/{mediaId}/submit ----------

func (d *Deps) MediaSubmit(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	row, err := d.queryRow(r.Context(),
		"SELECT * FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1", mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan"))
		return
	}
	if row.strOr("status", "") != "rejected" {
		d.writeErr(w, errConflict("Hanya media yang ditolak yang bisa diajukan ulang."))
		return
	}
	if err := d.exec(r.Context(),
		`UPDATE media SET status = 'pending_review', rejection_reason = NULL, reviewed_by = NULL, reviewed_at = NULL, updated_at = NOW()
		 WHERE id = $1 AND partner_id = $2`, mediaID, partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "media.resubmitted", "media", mediaID)
	link := "/admin/media?status=pending_review#tab-pending"
	d.notifyPlatformAdmins(r.Context(), "media.resubmitted", "Media diajukan ulang",
		`Media "`+row.strOr("name", "")+`" diajukan ulang oleh Mitra untuk direview.`, &link)

	updated, _ := d.queryRow(r.Context(), `
		SELECT id, name, slug, category, description, traffic_estimate, health_score, health_breakdown, website_url, adapter_url, price, status,
		       integration_type, notes, publication_mode, dewan_pers_verified, dewan_pers_note, dewan_pers_cert_no, dewan_pers_doc_key,
		       rejection_reason, reviewed_at, created_at, updated_at
		FROM media WHERE id = $1 LIMIT 1`, mediaID)
	markup := d.getMarkupPercent(r.Context())
	breakdown := breakdownFor(updated.numOr("price"), markup)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                  updated.raw("id"),
		"name":                updated.raw("name"),
		"slug":                updated.raw("slug"),
		"category":            updated.raw("category"),
		"description":         updated.raw("description"),
		"traffic_estimate":    updated.numOr("traffic_estimate"),
		"health_score":        updated.numPtr("health_score"),
		"health_breakdown":    updated.raw("health_breakdown"),
		"website_url":         updated.raw("website_url"),
		"adapter_url":         updated.raw("adapter_url"),
		"price":               updated.numOr("price"),
		"markup_percent":      breakdown.MarkupPercent,
		"markup_amount":       breakdown.MarkupAmount,
		"client_price":        breakdown.ClientPrice,
		"publication_mode":    updated.raw("publication_mode"),
		"dewan_pers_verified": updated.boolOr("dewan_pers_verified", false),
		"dewan_pers_note":     updated.raw("dewan_pers_note"),
		"dewan_pers_cert_no":  updated.raw("dewan_pers_cert_no"),
		"dewan_pers_doc_key":  updated.raw("dewan_pers_doc_key"),
		"status":              updated.raw("status"),
		"integration_type":    updated.raw("integration_type"),
		"notes":               updated.raw("notes"),
		"rejection_reason":    nil,
		"reviewed_at":         nil,
		"created_at":          updated.timePtr("created_at"),
		"updated_at":          updated.timePtr("updated_at"),
	})
}

// ---------- GET/PATCH /v1/partner/media/{mediaId}/category ----------

func (d *Deps) MediaCategory(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.queryRow(r.Context(),
		"SELECT id FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1", mediaID, partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	category, present := strField(body, "category")
	if !present || category == "" || len(category) > 120 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid category")
		return
	}
	category = catutil.Canonical(category)
	if err := d.exec(r.Context(),
		"UPDATE media SET category = $1, updated_at = NOW() WHERE id = $2 AND partner_id = $3",
		category, mediaID, partner.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	d.syncMediaCategories(r, mediaID, []string{category})
	writeJSON(w, http.StatusOK, map[string]any{"category": category})
}

// ---------- GET /v1/partner/media/categories ----------

func (d *Deps) MediaCategories(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	// Gabungan: field legacy m.category + katalog media_categories yang
	// tersambung ke media mitra lewat media_category_links. Hasilnya daftar
	// opsi kategori selalu memuat kategori yang ditambahkan mitra (mis. lewat
	// editor draf atau popover kelola kategori), supaya bisa dipakai ulang.
	rows, err := d.queryRows(r.Context(), `
		SELECT name FROM (
			SELECT m.category AS name FROM media m
			WHERE m.category IS NOT NULL AND TRIM(m.category) <> '' AND m.partner_id = $1
			UNION
			SELECT c.name FROM media_category_links l
			JOIN media_categories c ON c.id = l.category_id
			JOIN media m ON m.id = l.media_id
			WHERE m.partner_id = $1
		) cat_names
		ORDER BY name ASC`,
		partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if name := row.strOr("name", ""); name != "" {
			out = append(out, name)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}

// ---------- GET /v1/partner/media/template-url ----------

func (d *Deps) MediaTemplateURL(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.auth(w, r); !ok {
		return
	}
	row, err := d.queryRow(r.Context(),
		`SELECT value FROM system_metadata WHERE "key" = 'bulk_media_template_url' LIMIT 1`)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": row.raw("value")})
}

// ---------- GET/POST /v1/partner/media/{mediaId}/negotiation ----------

func (d *Deps) MediaNegotiation(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.ownedMedia(r, partner.ID, mediaID, "Media tidak ditemukan"); err != nil {
		d.writeErr(w, err)
		return
	}
	if r.Method == http.MethodPost {
		// Negosiasi harga dinonaktifkan: harga ditentukan mitra.
		httpx.Error(w, http.StatusUnprocessableEntity, "Negosiasi harga dinonaktifkan sementara — harga ditentukan mitra.")
		return
	}
	history, err := d.queryRows(r.Context(),
		"SELECT id, from_price, to_price, from_pricing_type, to_pricing_type, from_price_status, to_price_status, note, created_at FROM media_price_history WHERE media_id = $1 ORDER BY created_at DESC LIMIT 50",
		mediaID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(history))
	for _, h := range history {
		items = append(items, map[string]any{
			"id":                h.raw("id"),
			"from_price":        h.numPtr("from_price"),
			"to_price":          h.numPtr("to_price"),
			"from_pricing_type": h.raw("from_pricing_type"),
			"to_pricing_type":   h.raw("to_pricing_type"),
			"from_price_status": h.raw("from_price_status"),
			"to_price_status":   h.raw("to_price_status"),
			"note":              h.raw("note"),
			"created_at":        h.timePtr("created_at"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"history": items})
}

// ---------- GET /v1/partner/media/{mediaId}/ai-summary ----------

func (d *Deps) MediaAISummary(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT id, name, website_url, ai_summary, ai_summary_generated_at FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1",
		mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if media.has("ai_summary") {
		event, _ := d.queryRow(r.Context(),
			"SELECT status, pages_tried, error_message, attempt, created_at FROM media_ai_summary_events WHERE media_id = $1 AND status = 'success' ORDER BY created_at DESC LIMIT 1",
			mediaID)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "success",
			"summary":      media.strOr("ai_summary", ""),
			"generated_at": media.timePtr("ai_summary_generated_at"),
			"pages":        parsePagesTried(event),
			"error":        nil,
		})
		return
	}
	job, _ := d.queryRow(r.Context(),
		"SELECT status, attempts, last_error FROM jobs WHERE kind = 'media_ai_summary' AND payload->>'mediaId' = $1 ORDER BY created_at DESC LIMIT 1", mediaID)
	event, _ := d.queryRow(r.Context(),
		"SELECT status, pages_tried, error_message, attempt, created_at FROM media_ai_summary_events WHERE media_id = $1 ORDER BY created_at DESC LIMIT 1", mediaID)
	status := "scheduled"
	var lastError any
	if job != nil {
		switch job.strOr("status", "") {
		case "running":
			status = "processing"
		case "pending":
			status = "queued"
		case "failed":
			status = "failed"
			if job.has("last_error") {
				lastError = job.strOr("last_error", "")
			} else if event != nil {
				lastError = event.raw("error_message")
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       status,
		"summary":      nil,
		"generated_at": nil,
		"pages":        parsePagesTried(event),
		"error":        lastError,
	})
}

func parsePagesTried(event rowMap) []map[string]any {
	if event == nil {
		return []map[string]any{}
	}
	raw := event.raw("pages_tried")
	var arr []any
	switch t := raw.(type) {
	case string:
		if json.Unmarshal([]byte(t), &arr) != nil {
			return []map[string]any{}
		}
	case []any:
		arr = t
	default:
		return []map[string]any{}
	}
	out := []map[string]any{}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		url, ok := m["url"].(string)
		if !ok {
			continue
		}
		entry := map[string]any{"url": url, "ok": m["ok"] == true, "chars": toFloat(m["chars"])}
		if note, ok := m["note"].(string); ok {
			entry["note"] = truncateRunes(note, 120)
		}
		out = append(out, entry)
	}
	return out
}

func toFloat(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

// ---------- GET/POST /v1/partner/media/{mediaId}/similarweb ----------

func (d *Deps) MediaSimilarWeb(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT id, website_url FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1",
		mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan"))
		return
	}
	if r.Method == http.MethodPost {
		// Ambil snapshot SimilarWeb butuh kredensial provider (fase integrasi
		// SEO lanjutan); kontrak respons dipertahankan.
		httpx.Error(w, http.StatusServiceUnavailable, "Provider SimilarWeb belum dikonfigurasi.")
		return
	}
	snapshot, _ := d.queryRow(r.Context(),
		"SELECT insights, snapshot_date, created_at FROM media_similarweb_snapshots WHERE media_id = $1 ORDER BY snapshot_date DESC LIMIT 1",
		mediaID)
	if snapshot == nil {
		writeJSON(w, http.StatusOK, map[string]any{"media_id": mediaID, "insights": nil, "cached": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"media_id":      mediaID,
		"insights":      snapshot.raw("insights"),
		"snapshot_date": snapshot.timePtr("snapshot_date"),
		"created_at":    snapshot.timePtr("created_at"),
	})
}

// ---------- dewan pers (document + verification) ----------

func (d *Deps) MediaDewanPersDocument(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT id FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1", mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File tidak ditemukan.")
		return
	}
	defer file.Close()
	const maxBytes = 10 * 1024 * 1024
	if header.Size > maxBytes {
		httpx.Error(w, http.StatusUnprocessableEntity, "Ukuran file maksimal 10 MB.")
		return
	}
	contentType := header.Header.Get("Content-Type")
	nameLower := strings.ToLower(header.Filename)
	isPDF := contentType == "application/pdf" || strings.HasSuffix(nameLower, ".pdf")
	isImage := strings.HasPrefix(contentType, "image/")
	if !isPDF && !isImage {
		httpx.Error(w, http.StatusUnprocessableEntity, "Format file harus berupa PDF, JPG, atau PNG.")
		return
	}
	data, err := readAllLimited(file, maxBytes)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// Gambar dinormalisasi ke WebP; PDF apa adanya.
	payload := data
	ext := "pdf"
	if !isPDF {
		if converted, ok := imageconvert.ConvertToWebP(data, contentType); ok {
			payload = converted
			ext = "webp"
		} else {
			ext = sanitizeExt(nameLower, "jpg")
		}
	}
	storageKey := uuid.NewString() + "." + ext
	if err := d.saveUpload(storageKey, payload); err != nil {
		d.writeErr(w, err)
		return
	}
	updated, err := d.queryRow(r.Context(),
		`UPDATE media SET dewan_pers_doc_key = $1, updated_at = NOW() WHERE id = $2 AND partner_id = $3
		 RETURNING id, name, website_url, dewan_pers_verified, dewan_pers_note, dewan_pers_cert_no, dewan_pers_doc_key, updated_at`,
		storageKey, mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	d.auditLog(r.Context(), user.ID, "media.dewan_pers_doc_uploaded", "media", mediaID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"media": map[string]any{
			"id":                  updated.raw("id"),
			"name":                updated.raw("name"),
			"website_url":         updated.raw("website_url"),
			"dewan_pers_verified": updated.boolOr("dewan_pers_verified", false),
			"dewan_pers_note":     updated.raw("dewan_pers_note"),
			"dewan_pers_cert_no":  updated.raw("dewan_pers_cert_no"),
			"dewan_pers_doc_key":  updated.raw("dewan_pers_doc_key"),
			"updated_at":          updated.timePtr("updated_at"),
		},
	})
}

func sanitizeExt(name, fallback string) string {
	parts := strings.Split(name, ".")
	ext := fallback
	if len(parts) > 1 {
		ext = parts[len(parts)-1]
	}
	var b strings.Builder
	for _, ch := range ext {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}

// mediaDewanPersCertRegex: parity DEWAN_PERS_CERT_REGEX.
var mediaDewanPersCertRegex = regexp.MustCompile(`^[0-9]+/DP-Verifikasi/K/(I|II|III|IV|V|VI|VII|VIII|IX|X|XI|XII)/[0-9]{4}$`)

func validDewanPersCert(cert string) bool {
	return mediaDewanPersCertRegex.MatchString(strings.TrimSpace(cert))
}

func (d *Deps) MediaDewanPersVerification(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT id, name, partner_id, status FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1",
		mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan"))
		return
	}
	body := decodeJSON(r)
	cert, present := strField(body, "certificate_no")
	if !present || cert == "" || len(cert) > 200 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid certificate_no")
		return
	}
	result, err := d.submitDewanPersVerification(r.Context(), mediaID, partner.ID, cert, user.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if result.Conflict {
		link := "/admin/dewan-pers-reviews?status=conflict"
		d.notifyPlatformAdmins(r.Context(), "media.dewan_pers.submitted", "Konflik nomor sertifikat Dewan Pers",
			`Media "`+media.strOr("name", "")+`" mengajukan sertifikat yang sudah dipakai media lain.`, &link)
		httpx.Error(w, http.StatusConflict, "Nomor sertifikat ini sudah digunakan oleh media lain dan memerlukan pemeriksaan Admin.")
		return
	}
	if result.AlreadyPending {
		d.writeErr(w, errConflict("Verifikasi Dewan Pers sedang menunggu review Admin. Anda belum bisa mengubah nomor sertifikat."))
		return
	}
	if !result.AlreadyVerified {
		link := "/admin/dewan-pers-reviews?status=pending"
		d.notifyPlatformAdmins(r.Context(), "media.dewan_pers.submitted", "Verifikasi Dewan Pers menunggu review",
			`Media "`+media.strOr("name", "")+`" mengajukan sertifikat `+cert+`.`, &link)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"status":         result.Status,
		"certificate_no": result.CertificateNo,
		"submitted_at":   result.SubmittedAt,
	})
}

type dewanPersSubmitResult struct {
	Status          string
	CertificateNo   string
	SubmittedAt     any
	Conflict        bool
	AlreadyPending  bool
	AlreadyVerified bool
}

// submitDewanPersVerification: port submitDewanPersVerification.
func (d *Deps) submitDewanPersVerification(ctx context.Context, mediaID, partnerID, cert, actorUserID string) (dewanPersSubmitResult, error) {
	cert = strings.TrimSpace(cert)
	if !validDewanPersCert(cert) {
		return dewanPersSubmitResult{}, &PartnerError{http.StatusUnprocessableEntity,
			"Format nomor sertifikat tidak valid. Contoh: 206/DP-Verifikasi/K/X/2025"}
	}
	// 1) Media lain sudah mengklaim nomor ini (pending/verified) → conflict.
	other, _ := d.queryRow(ctx, `
		SELECT * FROM media_dewan_pers_verifications
		WHERE certificate_no = $1 AND media_id <> $2 AND status IN ('pending','verified')
		ORDER BY submitted_at DESC LIMIT 1`, cert, mediaID)
	if other != nil {
		id := uuid.NewString()
		if err := d.exec(ctx,
			"INSERT INTO media_dewan_pers_verifications (id, media_id, partner_id, certificate_no, status, conflict_with) VALUES ($1, $2, $3, $4, 'conflict', $5)",
			id, mediaID, partnerID, cert, other.raw("media_id")); err != nil {
			return dewanPersSubmitResult{}, err
		}
		inserted, _ := d.queryRow(ctx, "SELECT * FROM media_dewan_pers_verifications WHERE id = $1 LIMIT 1", id)
		d.syncDewanPersMediaColumns(ctx, mediaID)
		d.auditLog(ctx, actorUserID, "media.dewan_pers.conflict", "media", mediaID)
		return dewanPersSubmitResult{
			Status:        "conflict",
			CertificateNo: cert,
			SubmittedAt:   inserted.timePtr("submitted_at"),
			Conflict:      true,
		}, nil
	}
	existing := d.latestDewanPersVerification(ctx, mediaID)
	if existing != nil && existing.strOr("status", "") == "pending" {
		return dewanPersSubmitResult{AlreadyPending: true}, nil
	}
	if existing != nil && existing.strOr("status", "") == "verified" &&
		strings.TrimSpace(existing.strOr("certificate_no", "")) == cert {
		return dewanPersSubmitResult{
			Status:          "verified",
			CertificateNo:   cert,
			SubmittedAt:     existing.timePtr("submitted_at"),
			AlreadyVerified: true,
		}, nil
	}
	if existing != nil && existing.strOr("status", "") == "verified" {
		_ = d.exec(ctx,
			"UPDATE media_dewan_pers_verifications SET status = 'superseded', updated_at = NOW() WHERE id = $1",
			existing.raw("id"))
	}
	id := uuid.NewString()
	if err := d.exec(ctx,
		"INSERT INTO media_dewan_pers_verifications (id, media_id, partner_id, certificate_no, status) VALUES ($1, $2, $3, $4, 'pending')",
		id, mediaID, partnerID, cert); err != nil {
		return dewanPersSubmitResult{}, err
	}
	inserted, _ := d.queryRow(ctx, "SELECT * FROM media_dewan_pers_verifications WHERE id = $1 LIMIT 1", id)
	d.syncDewanPersMediaColumns(ctx, mediaID)
	d.auditLog(ctx, actorUserID, "media.dewan_pers.submitted", "media", mediaID)
	return dewanPersSubmitResult{
		Status:        "pending",
		CertificateNo: cert,
		SubmittedAt:   inserted.timePtr("submitted_at"),
	}, nil
}

func (d *Deps) latestDewanPersVerification(ctx context.Context, mediaID string) rowMap {
	row, err := d.queryRow(ctx, `
		SELECT * FROM media_dewan_pers_verifications WHERE media_id = $1
		ORDER BY CASE status WHEN 'pending' THEN 0 WHEN 'verified' THEN 1 WHEN 'conflict' THEN 2 WHEN 'rejected' THEN 3 ELSE 4 END, submitted_at DESC
		LIMIT 1`, mediaID)
	if err != nil {
		return nil
	}
	return row
}

func (d *Deps) syncDewanPersMediaColumns(ctx context.Context, mediaID string) {
	v := d.latestDewanPersVerification(ctx, mediaID)
	if v == nil {
		return
	}
	if v.strOr("status", "") == "verified" {
		_ = d.exec(ctx,
			"UPDATE media SET dewan_pers_verified = TRUE, dewan_pers_cert_no = $1, dewan_pers_note = COALESCE($2, dewan_pers_note), updated_at = NOW() WHERE id = $3",
			v.strOr("certificate_no", ""), v.raw("rejection_reason"), mediaID)
		return
	}
	_ = d.exec(ctx, "UPDATE media SET dewan_pers_verified = FALSE, updated_at = NOW() WHERE id = $1", mediaID)
}

// ---------- sewajarnya media SEO (GET/POST) ----------

func (d *Deps) MediaSEO(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT id, website_url FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1",
		mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan"))
		return
	}
	if r.Method == http.MethodPost {
		// Refresh metrik SEO butuh RapidAPI key platform (fase integrasi SEO).
		slog.Error("[seo] segarkan metrik media ditolak: konfigurasi SEO platform belum siap", "media_id", mediaID)
		httpx.Error(w, http.StatusConflict, "Konfigurasi integrasi SEO belum siap. Hubungi admin Tauthub.")
		return
	}
	metric, _ := d.queryRow(r.Context(),
		"SELECT media_id, da, pa, spam_score, ai_trust_score, ai_citation, ai_citation_checked_at, checked_at, source FROM media_seo_metrics WHERE media_id = $1 LIMIT 1",
		mediaID)
	domain := domainFromWebsite(media.strOr("website_url", ""))
	if metric == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"media_id": mediaID, "domain": nil, "da": nil, "pa": nil, "spam_score": nil,
			"ai_trust_score": nil, "ai_citation": nil, "ai_citation_checked_at": nil,
			"checked_at": nil, "source": nil, "cached": true,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"media_id":               mediaID,
		"domain":                 domain,
		"da":                     metric.numPtr("da"),
		"pa":                     metric.numPtr("pa"),
		"spam_score":             metric.numPtr("spam_score"),
		"ai_trust_score":         metric.numPtr("ai_trust_score"),
		"ai_citation":            metric.raw("ai_citation"),
		"ai_citation_checked_at": metric.timePtr("ai_citation_checked_at"),
		"checked_at":             metric.timePtr("checked_at"),
		"source":                 metric.raw("source"),
		"cached":                 true,
	})
}

// ---------- custom web (PATCH/POST) ----------

func (d *Deps) MediaCustomWeb(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT * FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1", mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan."))
		return
	}
	if media.strOr("integration_type", "") != "custom_web" {
		d.writeErr(w, errConflict("Media ini bukan platform Custom Web."))
		return
	}
	body := decodeJSON(r)
	if r.Method == http.MethodPost {
		// Regenerate API key.
		if !media.has("custom_web_endpoint_url") {
			httpx.Error(w, http.StatusUnprocessableEntity, "Atur endpoint URL terlebih dahulu.")
			return
		}
		if err := d.saveCustomWebConfig(r.Context(), mediaID, media.strOr("custom_web_endpoint_url", ""), "thb_"+randomBase64URL(32)); err != nil {
			d.writeErr(w, err)
			return
		}
		_ = d.exec(r.Context(), "UPDATE media SET custom_web_status = 'disconnected', updated_at = NOW() WHERE id = $1", mediaID)
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "API Key baru telah dibuat. Test connection ulang setelah mengganti key di sisi Mitra.",
		})
		return
	}
	endpoint, present := strField(body, "endpoint_url")
	if !present || endpoint == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "endpoint_url wajib diisi.")
		return
	}
	if msg := validateCustomWebEndpoint(endpoint, d.Cfg.IsProduction()); msg != "" {
		httpx.Error(w, http.StatusUnprocessableEntity, msg)
		return
	}
	apiKey, hasAPIKey := strField(body, "api_key")
	trimmedEndpoint := strings.TrimRight(endpoint, "/")
	if hasAPIKey && apiKey != "" {
		if err := d.saveCustomWebConfig(r.Context(), mediaID, trimmedEndpoint, apiKey); err != nil {
			d.writeErr(w, err)
			return
		}
	} else {
		if err := d.exec(r.Context(),
			"UPDATE media SET custom_web_endpoint_url = $1, custom_web_status = 'disconnected', updated_at = NOW() WHERE id = $2 AND partner_id = $3",
			trimmedEndpoint, mediaID, partner.ID); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success":            true,
		"api_key_configured": (hasAPIKey && apiKey != "") || media.has("custom_web_api_key_sealed"),
	})
}

// saveCustomWebConfig: seal API key v3 per-entity (parity saveCustomWebConfig).
func (d *Deps) saveCustomWebConfig(ctx context.Context, mediaID, endpointURL, apiKey string) error {
	sealed, err := crypto.SealV3(apiKey, d.Cfg.Secret, mediaID)
	if err != nil {
		return err
	}
	return d.exec(ctx,
		`UPDATE media
		   SET custom_web_endpoint_url = $1, custom_web_api_key_sealed = $2, custom_web_status = 'disconnected', enc_key_ver = 1, updated_at = NOW()
		 WHERE id = $3`,
		endpointURL, sealed, mediaID)
}

func validateCustomWebEndpoint(raw string, production bool) string {
	if !isHTTPURL(raw) {
		return "Endpoint URL tidak valid."
	}
	lower := strings.ToLower(strings.TrimSpace(raw))
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return "Endpoint harus menggunakan HTTP atau HTTPS."
	}
	if production {
		if !strings.HasPrefix(lower, "https://") || isPrivateHost(lower) {
			return "Production hanya menerima endpoint HTTPS publik."
		}
	}
	return ""
}

func isPrivateHost(raw string) bool {
	host := raw
	if i := strings.Index(raw, "://"); i >= 0 {
		host = raw[i+3:]
	}
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || host == "::1" {
		return true
	}
	if strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return true
	}
	for _, prefix := range []string{"10.", "127.", "192.168."} {
		if strings.HasPrefix(host, prefix) {
			return true
		}
	}
	if strings.HasPrefix(host, "172.") {
		parts := strings.Split(host, ".")
		if len(parts) > 1 {
			if n, err := parseSmallInt(parts[1]); err == nil && n >= 16 && n <= 31 {
				return true
			}
		}
	}
	return false
}

func parseSmallInt(s string) (int, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}

func (d *Deps) MediaCustomWebTest(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT * FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1", mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan."))
		return
	}
	if media.strOr("integration_type", "") != "custom_web" {
		d.writeErr(w, errConflict("Media ini bukan platform Custom Web."))
		return
	}
	endpointURL := media.strOr("custom_web_endpoint_url", "")
	sealedKey := media.strOr("custom_web_api_key_sealed", "")
	if endpointURL == "" || sealedKey == "" {
		d.updateCustomWebStatus(r, mediaID, "failed", "Endpoint atau API key belum dikonfigurasi.")
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false, "status": "failed",
			"message": "Endpoint atau API key belum dikonfigurasi.",
		})
		return
	}
	apiKey, err := crypto.UnsealV3(sealedKey, d.Cfg.Secret, mediaID)
	if err != nil {
		slog.Error("[media-credential] API key Custom Web gagal di-unseal (APP_SECRET berubah?)",
			"media_id", mediaID, "err", err)
		message := "Kredensial tersimpan tidak dapat dibuka. Hubungi admin Tauthub untuk menyimpan ulang."
		d.updateCustomWebStatus(r, mediaID, "failed", message)
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false, "status": "failed",
			"message": message,
		})
		return
	}
	d.updateCustomWebStatus(r, mediaID, "testing", "Menguji koneksi...")
	ok, message := d.pingCustomWeb(r, endpointURL, apiKey)
	status := "failed"
	if ok {
		status = "connected"
	}
	d.updateCustomWebStatus(r, mediaID, status, message)
	writeJSON(w, http.StatusOK, map[string]any{"success": ok, "status": status, "message": message})
}

func (d *Deps) updateCustomWebStatus(r *http.Request, mediaID, status, message string) {
	_ = d.exec(r.Context(),
		"UPDATE media SET custom_web_status = $1, custom_web_last_test_at = NOW(), custom_web_last_test_message = $2, updated_at = NOW() WHERE id = $3",
		status, message, mediaID)
}

// ---------- blogger ----------

func (d *Deps) bloggerMedia(r *http.Request, partnerID, mediaID string) (rowMap, error) {
	media, err := d.queryRow(r.Context(),
		"SELECT * FROM media WHERE id = $1 AND partner_id = $2 AND status <> 'hidden' LIMIT 1", mediaID, partnerID)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, errNotFound("Media tidak ditemukan.")
	}
	if media.strOr("integration_type", "") != "blogger" {
		return nil, errConflict("Media ini bukan platform Blogger.")
	}
	return media, nil
}

func (d *Deps) MediaBlogger(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.bloggerMedia(r, partner.ID, mediaID); err != nil {
		d.writeErr(w, err)
		return
	}
	if r.Method == http.MethodDelete {
		if err := d.exec(r.Context(),
			`UPDATE media SET blogger_credentials_sealed = NULL, blogger_blog_id = NULL, blogger_blog_name = NULL,
			   blogger_status = 'disconnected', blogger_last_test_at = NULL, blogger_last_test_message = NULL, updated_at = NOW()
			 WHERE id = $1`, mediaID); err != nil {
			d.writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}
	cfg, cfgErr := d.loadGSCPlatformConfig(r)
	if cfgErr != nil || cfg.ClientID == "" || cfg.ClientSecret == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "Konfigurasi integrasi Google belum siap. Hubungi admin Tauthub.")
		return
	}
	var body struct {
		BlogID   string `json:"blog_id"`
		BlogName string `json:"blog_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	blogID := strings.TrimSpace(body.BlogID)
	blogName := strings.TrimSpace(body.BlogName)
	if blogID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "blog_id wajib diisi.")
		return
	}
	media, mErr := d.bloggerMedia(r, partner.ID, mediaID)
	if mErr != nil {
		d.writeErr(w, mErr)
		return
	}
	cred := d.bloggerResolveTarget(r, media, cfg.ClientID, cfg.ClientSecret)
	if cred == nil {
		httpx.Error(w, http.StatusConflict, "Kredensial Blogger tidak valid — hubungkan ulang.")
		return
	}
	blogs, bErr := bloggerListBlogs(cred.AccessToken)
	if bErr != nil {
		httpx.Error(w, http.StatusBadGateway, bErr.Error())
		return
	}
	var found *bloggerBlog
	for i := range blogs {
		if blogs[i].ID == blogID {
			found = &blogs[i]
			break
		}
	}
	if found == nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Blog tidak ditemukan di akun Google yang terhubung.")
		return
	}
	finalName := found.Name
	if finalName == "" {
		finalName = blogName
	}
	if finalName == "" {
		finalName = found.ID
	}
	if err := d.exec(r.Context(),
		"UPDATE media SET blogger_blog_id = $1, blogger_blog_name = $2, blogger_status = 'connected', blogger_last_test_at = NOW(), blogger_last_test_message = 'Terhubung', updated_at = NOW() WHERE id = $3",
		found.ID, finalName, mediaID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "blog_id": found.ID, "blog_name": finalName})
}

func (d *Deps) MediaBloggerBlogs(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	media, err := d.bloggerMedia(r, partner.ID, chiParam(r, "mediaId"))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	cfg, cfgErr := d.loadGSCPlatformConfig(r)
	if cfgErr != nil || cfg.ClientID == "" || cfg.ClientSecret == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "Konfigurasi integrasi Google belum siap. Hubungi admin Tauthub.")
		return
	}
	cred := d.bloggerResolveTarget(r, media, cfg.ClientID, cfg.ClientSecret)
	if cred == nil {
		httpx.Error(w, http.StatusConflict, "Kredensial Blogger tidak valid — hubungkan ulang.")
		return
	}
	blogs, bErr := bloggerListBlogs(cred.AccessToken)
	if bErr != nil {
		httpx.Error(w, http.StatusBadGateway, bErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blogs": blogs})
}

func (d *Deps) MediaBloggerConnect(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	if _, err := d.bloggerMedia(r, partner.ID, mediaID); err != nil {
		d.writeErr(w, err)
		return
	}
	cfg, cfgErr := d.loadGSCPlatformConfig(r)
	if cfgErr != nil || cfg.ClientID == "" || cfg.ClientSecret == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "Konfigurasi integrasi Google belum siap. Hubungi admin Tauthub.")
		return
	}
	relayURI := d.bloggerRedirectURI(r)
	state := bloggerRandomToken(16)
	verifier := bloggerRandomToken(32)

	// Bersihkan state lama partner ini + state kedaluwarsa (parity).
	_ = d.exec(r.Context(),
		"DELETE FROM blogger_oauth_states WHERE partner_id = $1 OR created_at < NOW() - INTERVAL '30 minutes'", partner.ID)
	if err := d.exec(r.Context(),
		"INSERT INTO blogger_oauth_states (id, partner_id, media_id, state, code_verifier) VALUES (gen_random_uuid(), $1, $2, $3, $4)",
		partner.ID, mediaID, state, verifier); err != nil {
		d.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url": bloggerBuildAuthURL(cfg.ClientID, relayURI, state, bloggerPKCEChallenge(verifier)),
	})
}

func (d *Deps) MediaBloggerTest(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.bloggerMedia(r, partner.ID, mediaID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	_ = d.exec(r.Context(), "UPDATE media SET blogger_status = 'testing', updated_at = NOW() WHERE id = $1", mediaID)
	message := "Koneksi Blogger gagal."
	okResult := false

	blogID := media.strOr("blogger_blog_id", "")
	if blogID == "" {
		message = "Belum ada blog dipilih — hubungkan dan pilih blog terlebih dahulu."
	} else if strings.TrimSpace(media.strOr("blogger_credentials_sealed", "")) == "" {
		message = "Kredensial Blogger belum tersimpan — hubungkan terlebih dahulu."
	} else if cfg, cfgErr := d.loadGSCPlatformConfig(r); cfgErr != nil || cfg.ClientID == "" || cfg.ClientSecret == "" {
		message = "Konfigurasi integrasi Google belum siap. Hubungi admin Tauthub."
	} else if cred := d.bloggerResolveTarget(r, media, cfg.ClientID, cfg.ClientSecret); cred == nil {
		message = "Kredensial Blogger tidak valid — perlu hubungkan ulang."
	} else if blogs, bErr := bloggerListBlogs(cred.AccessToken); bErr != nil {
		message = bErr.Error()
	} else {
		found := false
		for _, b := range blogs {
			if b.ID == blogID {
				found = true
				break
			}
		}
		if !found {
			message = "Blog tidak ditemukan atau tidak dapat diakses dengan akun ini."
		} else {
			okResult = true
			message = "Blogger terhubung"
		}
	}
	status := "failed"
	if okResult {
		status = "connected"
	}
	_ = d.exec(r.Context(),
		"UPDATE media SET blogger_status = $1, blogger_last_test_at = NOW(), blogger_last_test_message = $2, updated_at = NOW() WHERE id = $3",
		status, message, mediaID)
	writeJSON(w, http.StatusOK, map[string]any{"success": okResult, "status": status, "message": message})
}

// ---------- test publishing api ----------

func (d *Deps) MediaTestPublishingAPI(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT * FROM media WHERE id = $1 AND partner_id = $2 LIMIT 1", mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan"))
		return
	}
	if media.strOr("integration_type", "") != "tulisio_api" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Media ini tidak ber-integrasi Tulisio Publishing API")
		return
	}
	if media.strOr("adapter_url", "") == "" {
		d.auditLog(r.Context(), user.ID, "media.test_connection", "media", mediaID)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": "Adapter URL atau API key belum diatur untuk media ini. Lengkapi Adapter URL dan buat API key di menu Publishing API.",
		})
		return
	}
	key, _ := d.queryRow(r.Context(), `
		SELECT id, api_key_sealed, secret_sealed FROM publishing_api_keys
		WHERE partner_id = $1 AND status = 'active' AND (media_id = $2 OR media_id IS NULL)
		ORDER BY (media_id = $2) DESC, created_at ASC LIMIT 1`, partner.ID, mediaID)
	if key == nil {
		d.auditLog(r.Context(), user.ID, "media.test_connection", "media", mediaID)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": "Adapter URL atau API key belum diatur untuk media ini. Lengkapi Adapter URL dan buat API key di menu Publishing API.",
		})
		return
	}
	apiKey, errKey := d.unsealT2(key.strOr("api_key_sealed", ""))
	secret, errSecret := d.unsealT2(key.strOr("secret_sealed", ""))
	if errKey != nil || errSecret != nil {
		slog.Error("[media-credential] API key/secret Publishing API gagal di-unseal (APP_SECRET berubah?)",
			"media_id", mediaID, "err_key", errKey, "err_secret", errSecret)
		d.auditLog(r.Context(), user.ID, "media.test_connection", "media", mediaID)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": "Kredensial tersimpan tidak dapat dibuka. Buat ulang API key di menu Publishing API atau hubungi admin Tauthub.",
		})
		return
	}
	pingResult := d.pingTulisioAdapter(r, media.strOr("adapter_url", ""), apiKey, secret, mediaID)
	d.auditLog(r.Context(), user.ID, "media.test_connection", "media", mediaID)
	writeJSON(w, http.StatusOK, pingResult)
}

// ---------- GET/POST /v1/partner/media/{mediaId}/domain-verification ----------

func (d *Deps) MediaDomainVerification(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	media, err := d.queryRow(r.Context(),
		"SELECT id, website_url, domain_verified, domain_verified_at FROM media WHERE id = $1 AND partner_id = $2 LIMIT 1",
		mediaID, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if media == nil {
		d.writeErr(w, errNotFound("Media tidak ditemukan"))
		return
	}
	if r.Method == http.MethodGet {
		token := d.getOrCreateDomainVerifyToken(r, mediaID)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":                 true,
			"domain_verified":    media.boolOr("domain_verified", false),
			"domain_verified_at": media.timePtr("domain_verified_at"),
			"token":              token,
		})
		return
	}
	if strings.TrimSpace(media.strOr("website_url", "")) == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isi Website / Domain media terlebih dahulu")
		return
	}
	if err := d.checkAuthRateReq(r, "domain-verify:"+user.ID, 5); err != nil {
		d.writeErr(w, err)
		return
	}
	result := d.verifyDomainOwnership(r, mediaID)
	after, _ := d.queryRow(r.Context(), "SELECT domain_verified FROM media WHERE id = $1 LIMIT 1", mediaID)
	result["domain_verified"] = after != nil && after.boolOr("domain_verified", false)
	writeJSON(w, http.StatusOK, result)
}

func (d *Deps) getOrCreateDomainVerifyToken(r *http.Request, mediaID string) string {
	var token string
	if err := d.Pool.QueryRow(r.Context(),
		`UPDATE media SET domain_verify_token = $1, updated_at = NOW() WHERE id = $2 AND domain_verify_token IS NULL RETURNING domain_verify_token`,
		randomHex(16), mediaID).Scan(&token); err == nil && token != "" {
		return token
	}
	row, _ := d.queryRow(r.Context(), "SELECT domain_verify_token FROM media WHERE id = $1 LIMIT 1", mediaID)
	return row.strOr("domain_verify_token", "")
}

// ---------- upload helpers ----------

func (d *Deps) saveUpload(storageKey string, data []byte) error {
	dir := d.uploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, storageKey), data, 0o644)
}

func readAllLimited(file interface{ Read([]byte) (int, error) }, max int64) ([]byte, error) {
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	var total int64
	for {
		n, err := file.Read(tmp)
		if n > 0 {
			total += int64(n)
			if total > max {
				return nil, &PartnerError{http.StatusUnprocessableEntity, "Ukuran file maksimal 10 MB."}
			}
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}
	return buf, nil
}
