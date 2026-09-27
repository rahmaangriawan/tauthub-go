package platform

// media_packages.go: paket media (bundle) admin — GET/POST /v1/platform/media-packages
// dan PUT/DELETE /v1/platform/media-packages/{packageId}.
// Port app/v1/platform/media-packages/route.ts + [packageId]/route.ts.

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const mpPackageSelectBase = `SELECT mp.id, mp.name, mp.slug, mp.description, mp.status, mp.discount_percent, mp.source,
       mp.created_at, mp.updated_at,
       COUNT(mpi.media_id) AS media_count,
       COALESCE(SUM(m.credit_cost), 0)::bigint AS total_credit_cost
  FROM media_packages mp
  LEFT JOIN media_package_items mpi ON mpi.package_id = mp.id
  LEFT JOIN media m ON m.id = mpi.media_id`

const mpPackageGroupBy = `GROUP BY mp.id, mp.name, mp.slug, mp.description, mp.status, mp.discount_percent, mp.source, mp.created_at, mp.updated_at`

// mpPackageJSON: bentuk baris paket + ringkasan diskon (port payload GET/POST/PUT).
func mpPackageJSON(row ptRow, forcedDiscount *int) map[string]any {
	total := row.numOr("total_credit_cost")
	discount := row.numOr("discount_percent")
	if forcedDiscount != nil {
		discount = float64(*forcedDiscount)
	}
	discountPercent := math.Max(0, math.Min(100, discount))
	discounted := total
	if discountPercent > 0 {
		discounted = math.Max(0, math.Round(total*(100-discountPercent)/100))
	}
	out := map[string]any{
		"id":                     row.anyOrNil("id"),
		"name":                   row.anyOrNil("name"),
		"slug":                   row.anyOrNil("slug"),
		"description":            row.anyOrNil("description"),
		"status":                 row.anyOrNil("status"),
		"source":                 row.anyOrNil("source"),
		"discount_percent":       discountPercent,
		"media_count":            row.intOr("media_count", 0),
		"total_credit_cost":      total,
		"discounted_credit_cost": discounted,
		"savings_credit":         math.Max(0, total-discounted),
	}
	if row.has("created_at") {
		out["created_at"] = row.anyOrNil("created_at")
	}
	if row.has("updated_at") {
		out["updated_at"] = row.anyOrNil("updated_at")
	}
	return out
}

// ---------- GET/POST /v1/platform/media-packages ----------

func (d *Deps) MediaPackagesList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	packages, err := d.queryRows(r.Context(), mpPackageSelectBase+" "+mpPackageGroupBy+" ORDER BY mp.name ASC")
	if err != nil {
		// Parity: log + 500 "Gagal memuat media packages".
		writeErr(w, d, errInternal("Gagal memuat media packages"))
		return
	}
	out := make([]map[string]any, 0, len(packages))
	for _, raw := range packages {
		out = append(out, mpPackageJSON(ptRow(ptNormalizeMap(raw)), nil))
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": out})
}

func (d *Deps) MediaPackageCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	name := ""
	if v, has := strField(body, "name"); has {
		name = v
	}
	mediaIds := []string{}
	if arr, has := rawField(body, "media_ids"); has {
		if list, isList := arr.([]any); isList {
			for _, item := range list {
				if s, isStr := item.(string); isStr && strings.TrimSpace(s) != "" {
					mediaIds = append(mediaIds, strings.TrimSpace(s))
				}
			}
		}
	}
	discountPercent := 0
	if v, has := numField(body, "discount_percent"); has {
		discountPercent = int(math.Max(0, math.Min(100, math.Round(v))))
	}

	if name == "" || len(name) > 120 {
		writeErr(w, d, errUnprocessable("Nama paket wajib diisi (maks. 120 karakter)"))
		return
	}
	if len(mediaIds) == 0 {
		writeErr(w, d, errUnprocessable("Pilih minimal 1 media"))
		return
	}
	unique := []string{}
	seen := map[string]bool{}
	for _, id := range mediaIds {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) != len(mediaIds) {
		writeErr(w, d, errUnprocessable("Ada media yang dipilih ganda"))
		return
	}

	ph := mpPlaceholders(len(unique), 1)
	foundArgs := make([]any, 0, len(unique))
	for _, id := range unique {
		foundArgs = append(foundArgs, id)
	}
	found, err := d.queryRows(r.Context(),
		"SELECT id FROM media WHERE id IN ("+ph+") AND status = 'active'", foundArgs...)
	if err != nil {
		writeErr(w, d, errInternal("Gagal membuat media package"))
		return
	}
	if len(found) != len(unique) {
		writeErr(w, d, errUnprocessable("Satu atau lebih media tidak ditemukan / tidak aktif"))
		return
	}

	id := uuid.NewString()
	slugBase := mpPackageSlug(name)
	if slugBase == "" {
		slugBase = "paket"
	}
	slug := slugBase + "-" + id[:8]

	// description: string → trim (boleh ""), selain itu NULL — parity route lama.
	var description any
	if v, has := strField(body, "description"); has {
		description = v
	}

	if err := d.exec(r.Context(), `
		INSERT INTO media_packages (id, name, slug, description, status, source, discount_percent, created_by)
		VALUES ($1, $2, $3, $4, 'active', 'admin', $5, $6)`,
		id, name, slug, description, discountPercent, user.ID); err != nil {
		writeErr(w, d, errInternal("Gagal membuat media package"))
		return
	}

	values := []string{}
	itemArgs := []any{}
	for i, mediaID := range unique {
		base := len(itemArgs)
		values = append(values, "($"+ptItoa(base+1)+", $"+ptItoa(base+2)+", $"+ptItoa(base+3)+")")
		itemArgs = append(itemArgs, id, mediaID, i)
	}
	if err := d.exec(r.Context(),
		"INSERT INTO media_package_items (package_id, media_id, position) VALUES "+strings.Join(values, ","),
		itemArgs...); err != nil {
		writeErr(w, d, errInternal("Gagal membuat media package"))
		return
	}

	pkg, err := d.queryRow(r.Context(), mpPackageSelectBase+" WHERE mp.id = $1 "+mpPackageGroupBy, id)
	if err != nil {
		writeErr(w, d, errInternal("Gagal membuat media package"))
		return
	}
	if pkg == nil {
		writeJSON(w, http.StatusCreated, map[string]any{"package": nil})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"package": mpPackageJSON(ptRow(ptNormalizeMap(pkg)), &discountPercent)})
}

// mpPackageSlug: parity slug pembuatan paket (lowercase, non-alfanumerik → -).
func mpPackageSlug(name string) string {
	lower := strings.ToLower(name)
	var b strings.Builder
	lastDash := false
	for _, ch := range lower {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

// ---------- PUT/DELETE /v1/platform/media-packages/{packageId} ----------

func (d *Deps) MediaPackageUpdate(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	packageID := chiParam(r, "packageId")
	body := decodeJSON(r)
	rawDiscount, hasDiscount := numField(body, "discount_percent")
	if !hasDiscount {
		writeErr(w, d, errUnprocessable("discount_percent wajib berupa angka"))
		return
	}
	discountPercent := int(math.Max(0, math.Min(100, math.Round(rawDiscount))))

	existing, err := d.queryRow(r.Context(), "SELECT id FROM media_packages WHERE id = $1 LIMIT 1", packageID)
	if err != nil {
		writeErr(w, d, errInternal("Gagal memperbarui media package"))
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound("Media package tidak ditemukan"))
		return
	}
	if err := d.exec(r.Context(),
		"UPDATE media_packages SET discount_percent = $1, updated_at = NOW() WHERE id = $2", discountPercent, packageID); err != nil {
		writeErr(w, d, errInternal("Gagal memperbarui media package"))
		return
	}

	pkg, err := d.queryRow(r.Context(), `
		SELECT mp.id, mp.name, mp.slug, mp.description, mp.status, mp.discount_percent, mp.source,
		       COUNT(mpi.media_id)::int AS media_count,
		       COALESCE(SUM(m.credit_cost), 0)::bigint AS total_credit_cost
		FROM media_packages mp
		LEFT JOIN media_package_items mpi ON mpi.package_id = mp.id
		LEFT JOIN media m ON m.id = mpi.media_id
		WHERE mp.id = $1
		GROUP BY mp.id, mp.name, mp.slug, mp.description, mp.status, mp.discount_percent, mp.source`, packageID)
	if err != nil {
		writeErr(w, d, errInternal("Gagal memperbarui media package"))
		return
	}
	if pkg == nil {
		writeJSON(w, http.StatusOK, map[string]any{"package": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"package": mpPackageJSON(ptRow(ptNormalizeMap(pkg)), &discountPercent)})
}

func (d *Deps) MediaPackageDelete(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	packageID := chiParam(r, "packageId")
	pkg, err := d.queryRow(r.Context(), "SELECT id FROM media_packages WHERE id = $1 AND source = 'admin' LIMIT 1", packageID)
	if err != nil {
		writeErr(w, d, errInternal("Gagal menghapus media package"))
		return
	}
	if pkg == nil {
		writeErr(w, d, errNotFound("Media package tidak ditemukan"))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM media_packages WHERE id = $1", packageID); err != nil {
		writeErr(w, d, errInternal("Gagal menghapus media package"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// errInternal: 500 dengan pesan yang di-scrub di production (parity apiError).
// Sengaja error biasa (bukan adminshared.Error) supaya WriteErr men-scrub
// detailnya saat APP_ENV=production, sama seperti app lama.
func errInternal(msg string) error { return errors.New(msg) }
