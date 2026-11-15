package clienttaut

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// campaign_duplicate.go: "Ulangi kampanye".
//
// Membuat kampanye draf baru dari kampanye lama: media yang sama (satu slot per
// media), Profiler dan snapshot-nya, ringkasan strategi, dan konfigurasi tautan.
// Tanpa artikel, tanpa publikasi, dan tanpa reserve kredit: klien menulis topik
// baru lewat alur biasa, lalu estimasi anggaran dihitung ulang dari harga media
// hari ini (harga, markup, dan kurs TC bisa sudah berubah). Tidak ada
// distribusi otomatis.
//
// Beda dengan "ronde baru" (TautCampaignNewRound): ronde menambah tagihan ke
// kampanye yang sama dan hanya jalan pada status tertentu; duplikat memberi
// kampanye bersih dengan nomor order sendiri.

// TautCampaignDuplicate: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/duplicate
func (d *Deps) TautCampaignDuplicate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	sourceID := chiParam(r, "campaignId")
	source, err := d.queryRow(r.Context(), `
		SELECT id, name FROM campaigns
		WHERE id = $1 AND brand_id = $2 AND taut_status IS NOT NULL AND deleted_at IS NULL LIMIT 1`,
		sourceID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if source == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}

	// Media kampanye lama yang masih dipilih. Yang kini tidak tampil di katalog
	// (nonaktif, tersembunyi, belum tersambung, tanpa harga) dilewati dan
	// dilaporkan, bukan membuat kampanye yang pasti gagal dipesan.
	mediaRows, err := d.queryRows(r.Context(), `
		SELECT DISTINCT tcm.media_id,
		       `+dbpkg.MediaCatalogVisibleSQL("m")+` AS orderable
		FROM taut_campaign_media tcm
		JOIN media m ON m.id = tcm.media_id
		WHERE tcm.campaign_id = $1 AND tcm.status = 'selected'`, sourceID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	orderable := make([]string, 0, len(mediaRows))
	unavailable := make([]string, 0)
	for _, row := range mediaRows {
		id := row.strOr("media_id", "")
		if row.boolOr("orderable", false) {
			orderable = append(orderable, id)
		} else {
			unavailable = append(unavailable, id)
		}
	}

	newName := strings.TrimSpace(source.strOr("name", "Kampanye")) + " (ulang)"
	if len([]rune(newName)) > 120 {
		newName = string([]rune(newName)[:120])
	}
	status := "draft"
	if len(orderable) > 0 {
		status = "placement"
	}
	newID := uuid.NewString()
	created, err := d.queryRow(r.Context(), `
		INSERT INTO campaigns (id, brand_id, name, created_by, status,
		   business_profile, hyperlink_config, media_type, trigger_keywords,
		   budget_strategy, budget_config, strategy_brief, starts_at,
		   taut_status, taut_entry_mode, profiler_id, profiler_snapshot, duplicated_from_campaign_id, created_at, updated_at)
		SELECT $1::varchar, c.brand_id, $3::varchar, $4::varchar, 'active',
		       c.business_profile, c.hyperlink_config, c.media_type, c.trigger_keywords,
		       c.budget_strategy, c.budget_config, c.strategy_brief, $5::timestamptz,
		       $6::varchar, c.taut_entry_mode, c.profiler_id, c.profiler_snapshot, c.id, NOW(), NOW()
		FROM campaigns c WHERE c.id = $2 AND c.brand_id = $7
		RETURNING public_id`,
		newID, sourceID, newName, user.ID, time.Now(), status, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if created == nil {
		httpx.Error(w, http.StatusNotFound, "Kampanye tidak ditemukan.")
		return
	}

	added := map[string]any{"added": []string{}, "skipped": []string{}}
	if len(orderable) > 0 {
		result, err := d.addManualMedia(r.Context(), newID, brandID, orderable)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		added = result
	}
	d.auditLog(r, user.ID, "taut.campaign_duplicated", "campaign", newID)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          newID,
		"public_id":   created.strOr("public_id", ""),
		"name":        newName,
		"taut_status": status,
		"source_id":   sourceID,
		"media":       added,
		"unavailable": unavailable,
	})
}

// TautPreviousArticles: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/previous-articles
//
// Artikel kampanye asal ("Ulangi kampanye") sebagai rujukan BACA-SAJA di Dapur
// Produksi kampanye baru: hanya judul, thumbnail, dan nama media. Tanpa isi
// artikel. Kosong bila kampanye ini bukan hasil duplikat, atau kampanye asalnya
// sudah dihapus atau bukan milik brand yang sama.
func (d *Deps) TautPreviousArticles(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID, err := d.brandID(r, user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	ctx := r.Context()
	empty := map[string]any{"campaign": nil, "items": []any{}}
	cur, err := d.queryRow(ctx,
		`SELECT duplicated_from_campaign_id FROM campaigns WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL LIMIT 1`,
		chiParam(r, "campaignId"), brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if cur == nil || !cur.has("duplicated_from_campaign_id") {
		writeJSON(w, http.StatusOK, empty)
		return
	}
	srcID := strings.TrimSpace(cur.strOr("duplicated_from_campaign_id", ""))
	src, err := d.queryRow(ctx,
		`SELECT id, name FROM campaigns WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL LIMIT 1`, srcID, brandID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if src == nil {
		writeJSON(w, http.StatusOK, empty)
		return
	}
	rows, err := d.queryRows(ctx, `
		SELECT tcm.media_id, tcm.slot, m.name AS media_name, cd.title, tcm.featured_image_url
		FROM taut_campaign_media tcm
		JOIN media m ON m.id = tcm.media_id
		JOIN content_documents cd ON cd.id = tcm.article_document_id
		WHERE tcm.campaign_id = $1 AND tcm.status = 'selected' AND tcm.article_document_id IS NOT NULL
		ORDER BY m.name, tcm.slot LIMIT 100`, srcID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, it := range rows {
		items = append(items, map[string]any{
			"media_id": it.strOr("media_id", ""), "slot": it.intOr("slot", 1),
			"media_name": it.strOr("media_name", ""), "title": it.strOr("title", ""),
			"thumbnail": it.raw("featured_image_url"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaign": map[string]any{"id": srcID, "name": src.strOr("name", "")},
		"items":    items,
	})
}
