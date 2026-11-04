package platform

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/catutil"
)

// Perubahan non-sensitif dari mitra pada media aktif diparkir di
// media.pending_values (lihat partner/media_detail.go MediaUpdate). Media tetap
// tayang dengan data live sampai admin memutuskan lewat MediaReview.

// mediaParkedChangesReview: approve menerapkan nilai yang diparkir ke baris
// live; reject membuangnya. Status media tidak pernah berubah di jalur ini.
func (d *Deps) mediaParkedChangesReview(w http.ResponseWriter, r *http.Request, adminID string, media ptRow, decision, reason, pendingJSON string) {
	ctx := r.Context()
	mediaID := media.strOr("id", "")
	mediaName := media.strOr("name", "Media")
	partnerID := media.strOr("partner_id", "")

	var pending map[string]any
	if err := json.Unmarshal([]byte(pendingJSON), &pending); err != nil {
		pending = map[string]any{}
	}
	_, hasPrice := pending["price"]
	_, hasPricingType := pending["pricing_type"]
	oldStatus := media.strOr("price_status", "menunggu_review")

	if decision == "approve" {
		if err := d.mpApplyParkedChanges(ctx, mediaID, adminID, pending); err != nil {
			writeErr(w, d, mpMediaError(err))
			return
		}
		if hasPrice || hasPricingType {
			newPrice, _ := pending["price"].(float64)
			if !hasPrice {
				newPrice = media.numOr("price")
			}
			_ = d.exec(ctx, `
				INSERT INTO media_price_history (id, media_id, actor_id, from_price, to_price, from_price_status, to_price_status, note)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, 'disetujui', $6)`,
				mediaID, adminID, mpNullableArgFloat(media.numOr("price"), media.has("price")), newPrice, oldStatus,
				"Perubahan harga disetujui admin")
		}
		writeAuditBestEffort(d, ctx, adminID, "media.change_approved", "media", mediaID)
	} else {
		if err := d.exec(ctx, `
			UPDATE media SET pending_values = NULL, pending_changes = NULL, pending_requested_at = NULL,
			       pending_reminded_at = NULL, reviewed_by = $1, reviewed_at = NOW(), updated_at = NOW()
			WHERE id = $2`, adminID, mediaID); err != nil {
			writeErr(w, d, mpMediaError(err))
			return
		}
		if hasPrice || hasPricingType {
			_ = d.exec(ctx, `
				INSERT INTO media_price_history (id, media_id, actor_id, from_price, to_price, from_price_status, to_price_status, note)
				VALUES (gen_random_uuid(), $1, $2, $3, $3, 'menunggu_review', $4, $5)`,
				mediaID, adminID, mpNullableArgFloat(media.numOr("price"), media.has("price")), oldStatus,
				"Pengajuan harga ditolak admin: "+reason)
		}
		writeAuditBestEffort(d, ctx, adminID, "media.change_rejected", "media", mediaID)
	}

	if partnerID != "" {
		linkID := partnerID
		if row, _ := d.queryRow(ctx, "SELECT COALESCE(public_id, id) AS link_id FROM partners WHERE id = $1 LIMIT 1", partnerID); row != nil {
			if v := ptRow(ptNormalizeMap(row)).strOr("link_id", ""); v != "" {
				linkID = v
			}
		}
		link := "/mitra/" + linkID + "/media"
		if decision == "approve" {
			d.mpNotifyPartner(ctx, partnerID, "media.approved", "Perubahan media disetujui",
				`Perubahan pada media "`+mediaName+`" telah disetujui Admin dan kini berlaku.`, &link)
		} else {
			d.mpNotifyPartner(ctx, partnerID, "media.rejected", "Perubahan media ditolak",
				`Perubahan pada media "`+mediaName+`" ditolak Admin. Alasan: `+reason+`. Media tetap tayang dengan data lama; ajukan kembali setelah diperbaiki.`, &link)
		}
	}

	updated, err := d.mpScopedMedia(ctx, mediaID)
	if err != nil {
		writeErr(w, d, mpMediaError(err))
		return
	}
	writeJSON(w, http.StatusOK, d.mpMediaJSON(updated, d.mpGetMarkupPercent(ctx)))
}

// mpApplyParkedChanges: tulis nilai yang diparkir ke kolom live dalam satu UPDATE,
// lalu kosongkan penanda pengajuan. Kunci mengikuti body API mitra.
func (d *Deps) mpApplyParkedChanges(ctx context.Context, mediaID, adminID string, pv map[string]any) error {
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+ptItoa(len(args)))
	}
	str := func(key string) (string, bool) {
		s, ok := pv[key].(string)
		return s, ok
	}
	num := func(key string) (float64, bool) {
		f, ok := pv[key].(float64)
		return f, ok
	}
	list := func(key string) ([]string, bool) {
		raw, ok := pv[key].([]any)
		if !ok {
			return nil, false
		}
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out, true
	}

	if v, ok := str("name"); ok {
		add("name", v)
		add("slug", mpSlugifyMediaName(v))
	}
	if v, ok := str("description"); ok {
		add("description", v)
	}
	if v, ok := str("notes"); ok {
		add("notes", v)
	}
	if v, ok := str("pricing_type"); ok {
		add("pricing_type", v)
	}
	if v, ok := num("price"); ok {
		add("price", v)
		clientPrice := mpComputeClientPrice(v, d.mpGetMarkupPercent(ctx))
		add("credit_cost", mpCreditCostFor(clientPrice, d.mpGetRateIDR(ctx)))
	}
	if _, ok := pv["price"]; ok {
		sets = append(sets, "price_status = 'disetujui'")
	} else if _, ok := pv["pricing_type"]; ok {
		sets = append(sets, "price_status = 'disetujui'")
	}
	if v, ok := num("traffic_estimate"); ok {
		add("traffic_estimate", int64(v))
	}
	if v, ok := num("max_links"); ok {
		add("max_links", int(v))
	}
	if v, ok := num("link_duration_months"); ok {
		add("link_duration_months", int(v))
	}
	if v, ok := pv["allow_hyperlink"].(bool); ok {
		add("allow_hyperlink", v)
	}
	if v, ok := str("label_sponsor"); ok {
		add("label_sponsor", v)
	}
	if v, ok := str("link_rel"); ok && (v == "dofollow" || v == "nofollow" || v == "sponsored") {
		add("link_rel", v)
	}
	if v, ok := str("dewan_pers_doc_key"); ok {
		if v == "" {
			sets = append(sets, "dewan_pers_doc_key = NULL")
		} else {
			add("dewan_pers_doc_key", v)
		}
	}
	if v, ok := list("bahasa_artikel"); ok && len(v) > 0 {
		b, _ := json.Marshal(v)
		add("bahasa_artikel", string(b))
	}
	if v, ok := list("topik_terlarang"); ok {
		if len(v) == 0 {
			sets = append(sets, "topik_terlarang = NULL")
		} else {
			b, _ := json.Marshal(v)
			add("topik_terlarang", string(b))
		}
	}
	categories, hasCategories := list("categories")
	if hasCategories && len(categories) > 0 {
		add("category", catutil.Canonical(categories[0]))
	} else if v, ok := str("category"); ok {
		add("category", v)
	}

	args = append(args, adminID)
	reviewer := "$" + ptItoa(len(args))
	args = append(args, mediaID)
	where := "$" + ptItoa(len(args))
	sets = append(sets,
		"pending_values = NULL", "pending_changes = NULL", "pending_requested_at = NULL", "pending_reminded_at = NULL",
		"reviewed_by = "+reviewer, "reviewed_at = NOW()", "updated_at = NOW()")
	if err := d.exec(ctx, "UPDATE media SET "+strings.Join(sets, ", ")+" WHERE id = "+where, args...); err != nil {
		return err
	}
	if hasCategories {
		d.mpSyncMediaCategories(ctx, mediaID, categories)
	}
	return nil
}

// mpSyncMediaCategories: padanan partner.syncMediaCategories (upsert kategori + link).
func (d *Deps) mpSyncMediaCategories(ctx context.Context, mediaID string, names []string) {
	seen := map[string]bool{}
	ids := []string{}
	for _, name := range catutil.SplitAll(names) {
		normalized := strings.ToLower(name)
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		var categoryID string
		if err := d.Pool.QueryRow(ctx,
			`INSERT INTO media_categories (id, name, normalized, created_at) VALUES (gen_random_uuid(), $1, $2, NOW())
			 ON CONFLICT (normalized) DO UPDATE SET name = media_categories.name
			 RETURNING id`, name, normalized).Scan(&categoryID); err != nil {
			slog.Warn("sync kategori gagal", "media", mediaID, "err", err)
			return
		}
		ids = append(ids, categoryID)
	}
	if err := d.exec(ctx, "DELETE FROM media_category_links WHERE media_id = $1", mediaID); err != nil {
		slog.Warn("hapus link kategori gagal", "media", mediaID, "err", err)
		return
	}
	for _, categoryID := range ids {
		if err := d.exec(ctx,
			"INSERT INTO media_category_links (media_id, category_id, created_at) VALUES ($1, $2, NOW()) ON CONFLICT DO NOTHING",
			mediaID, categoryID); err != nil {
			slog.Warn("insert link kategori gagal", "media", mediaID, "err", err)
		}
	}
}
