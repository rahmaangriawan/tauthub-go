package platform

// Modul changelog ("Apa yang Baru"): entri rilis yang diimpor admin dari
// web/static/changelog.md (tabel changelog_entries / changelog_reads, migrasi
// 190) lalu ditampilkan di dashboard klien & mitra. Sumber kebenaran isi entri
// tetap file repo; tabel hanya salinan supaya bisa tampil + memicu notifikasi.
// Sisi user hidup di /v1/changelog (semua role login), sisi admin di
// /v1/platform/changelog.

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Batasan konten entri changelog yang diterima dari impor.
const (
	chgVersionMax   = 40
	chgTitleMin     = 3
	chgTitleMax     = 160
	chgBodyMax      = 40000
	chgSummaryMax   = 400
	chgSourceKeyMax = 120
	chgListMax      = 50
)

var chgCategorySet = map[string]bool{"fitur": true, "perbaikan": true, "performa": true, "lainnya": true}

var chgNotFoundErr = errors.New("Entri changelog tidak ditemukan.")

// chgEntryJSON: bentuk JSON satu entri untuk respons API.
func chgEntryJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":          row.strOr("id", ""),
		"version":     row.strOr("version", ""),
		"released_on": row.strOr("released_on", ""),
		"category":    row.strOr("category", "lainnya"),
		"title":       row.strOr("title", ""),
		"summary":     row.strOr("summary", ""),
		"body":        row.strOr("body", ""),
		"source_key":  row.strOr("source_key", ""),
		"published":   row.boolOr("published", true),
		"created_at":  row.timePtr("created_at"),
	}
}

// ---------- sisi pengguna: /v1/changelog ----------

// ChangelogList: daftar entri yang sudah diterbitkan + penanda sudah dibaca.
func (d *Deps) ChangelogList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT e.id, e.version, e.released_on::text AS released_on, e.category, e.title,
		       e.summary, e.body, e.title_en, e.summary_en, e.body_en,
		       e.source_key, e.published, e.created_at, cr.read_at
		FROM changelog_entries e
		LEFT JOIN changelog_reads cr ON cr.entry_id = e.id AND cr.user_id = $1
		WHERE e.published = TRUE
		ORDER BY e.released_on DESC, e.created_at DESC
		LIMIT $2`, user.ID, chgListMax)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	unreadRow, err := d.queryRow(r.Context(), `
		SELECT count(*) AS total FROM changelog_entries e
		WHERE e.published = TRUE
		  AND NOT EXISTS (SELECT 1 FROM changelog_reads cr WHERE cr.entry_id = e.id AND cr.user_id = $1)`, user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := chgEntryJSON(row)
		item["read_at"] = row.timePtr("read_at")
		items = append(items, item)
	}
	unread := 0
	if unreadRow != nil {
		unread = unreadRow.intOr("total", 0)
	}
	// Antarmuka Inggris: terjemahkan entri yang belum punya versi EN (hasilnya
	// disimpan di kolom *_en supaya permintaan berikutnya tidak memanggil model).
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("lang")), "en") {
		d.chgLocalizeItems(r.Context(), user.ID, rows, items)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "unread": unread})
}

// ChangelogMarkRead: tandai satu entri sudah dibaca (idempoten).
func (d *Deps) ChangelogMarkRead(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	entryID := chiParam(r, "entryId")
	seen, err := d.queryRow(r.Context(),
		"SELECT EXISTS (SELECT 1 FROM changelog_entries e WHERE e.id = $1 AND e.published = TRUE) AS visible", entryID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if seen == nil || !truthy(seen.raw("visible")) {
		httpx.Error(w, http.StatusNotFound, chgNotFoundErr.Error())
		return
	}
	if err := d.exec(r.Context(), `
		INSERT INTO changelog_reads (entry_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (entry_id, user_id) DO NOTHING`, entryID, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- sisi admin: /v1/platform/changelog ----------

// AdminChangelogList: semua entri (termasuk yang belum diterbitkan) + jumlah
// pembaca per entri.
func (d *Deps) AdminChangelogList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT e.id, e.version, e.released_on::text AS released_on, e.category, e.title,
		       e.summary, e.body, e.source_key, e.published, e.created_at,
		       (SELECT count(*) FROM changelog_reads cr WHERE cr.entry_id = e.id) AS read_count
		FROM changelog_entries e
		ORDER BY e.released_on DESC, e.created_at DESC
		LIMIT $1`, chgListMax)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := chgEntryJSON(row)
		item["read_count"] = row.intOr("read_count", 0)
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// AdminChangelogImport: impor entri hasil parse web/static/changelog.md.
// Idempoten per source_key (versi|kategori): baris yang sudah ada diperbarui,
// yang BARU memicu notifikasi lonceng + web push (tipe changelog.new).
func (d *Deps) AdminChangelogImport(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	raw, has := rawField(body, "entries")
	if !has {
		writeErr(w, d, errUnprocessable("entries wajib diisi."))
		return
	}
	arr, isArr := raw.([]any)
	if !isArr || len(arr) == 0 {
		writeErr(w, d, errUnprocessable("entries harus berisi minimal 1 entri."))
		return
	}
	if len(arr) > 200 {
		writeErr(w, d, errUnprocessable("Maksimal 200 entri per impor."))
		return
	}

	created := 0
	updated := 0
	notified := int64(0)
	items := make([]map[string]any, 0, len(arr))
	firstID := ""

	for idx, rawItem := range arr {
		item, itemOK := rawItem.(map[string]any)
		if !itemOK {
			writeErr(w, d, errUnprocessable("Setiap entri harus berupa objek."))
			return
		}
		version, _ := strField(item, "version")
		releasedOn, _ := strField(item, "released_on")
		category, hasCategory := strField(item, "category")
		title, _ := strField(item, "title")
		summary, _ := strField(item, "summary")
		html, _ := strField(item, "body")
		sourceKey, _ := strField(item, "source_key")

		version = strings.TrimSpace(version)
		title = strings.TrimSpace(title)
		html = strings.TrimSpace(html)
		summary = strings.TrimSpace(summary)
		category = strings.ToLower(strings.TrimSpace(category))
		sourceKey = strings.TrimSpace(sourceKey)

		if version == "" || len([]rune(version)) > chgVersionMax {
			writeErr(w, d, errUnprocessable("Versi entri wajib diisi (maksimal 40 karakter)."))
			return
		}
		parsed, parseErr := time.Parse("2006-01-02", strings.TrimSpace(releasedOn))
		if parseErr != nil {
			writeErr(w, d, errUnprocessable("Tanggal entri harus format YYYY-MM-DD."))
			return
		}
		if !hasCategory || !chgCategorySet[category] {
			category = "lainnya"
		}
		if runes := []rune(title); len(runes) < chgTitleMin || len(runes) > chgTitleMax {
			writeErr(w, d, errUnprocessable("Judul entri harus 3-160 karakter."))
			return
		}
		if html == "" || len([]rune(html)) > chgBodyMax {
			writeErr(w, d, errUnprocessable("Isi entri wajib diisi (maksimal 40000 karakter)."))
			return
		}
		if len([]rune(summary)) > chgSummaryMax {
			summary = string([]rune(summary)[:chgSummaryMax])
		}
		if sourceKey == "" {
			sourceKey = version + "|" + category
		}
		if len([]rune(sourceKey)) > chgSourceKeyMax {
			sourceKey = string([]rune(sourceKey)[:chgSourceKeyMax])
		}

		existing, err := d.queryRow(r.Context(),
			"SELECT id FROM changelog_entries WHERE source_key = $1 LIMIT 1", sourceKey)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		isNew := existing == nil

		row, err := d.queryRow(r.Context(), `
			INSERT INTO changelog_entries (version, released_on, category, title, summary, body, source_key, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (source_key) DO UPDATE SET
			  version = EXCLUDED.version, released_on = EXCLUDED.released_on,
			  category = EXCLUDED.category, title = EXCLUDED.title,
			  summary = EXCLUDED.summary, body = EXCLUDED.body, updated_at = NOW()
			RETURNING id, version, released_on::text AS released_on, category, title, summary, body,
			          source_key, published, created_at`,
			version, parsed, category, title, summary, html, sourceKey, user.ID)
		if err != nil {
			writeErr(w, d, err)
			return
		}
		if row == nil {
			continue
		}
		entryID := row.strOr("id", "")
		if firstID == "" {
			firstID = entryID
		}
		if isNew {
			created++
			// Lonceng + web push: satu baris per user aktif (klien & mitra).
			notifTitle := "Apa yang Baru: " + title
			if runes := []rune(notifTitle); len(runes) > 180 {
				notifTitle = string(runes[:180])
			}
			// Tautan per peran: tanpa ini lonceng memakai cadangan bawaan
			// ({base}/publications) sehingga entri "Apa yang Baru" membuka halaman
			// publikasi, bukan halaman changelog-nya.
			// created_at dibuat EKSPLISIT menurut urutan entri: entri pertama di
			// payload = versi terbaru (changelog.md terbaru-di-atas), jadi diberi
			// timestamp paling baru. Tanpa ini semua notifikasi satu batch berbagi
			// waktu yang (hampir) sama dan urutan lonceng jadi acak — versi lama
			// bisa muncul di atas versi baru.
			n, err := d.execRows(r.Context(), `
				INSERT INTO notifications (id, user_id, type, title, body, link, created_at)
				SELECT gen_random_uuid()::text, x.id, 'changelog.new', $1, left($2, 500),
				       CASE x.role
				         WHEN 'client' THEN (
				           SELECT '/klien/' || b.public_id || '/changelog'
				           FROM brand_memberships bm
				           JOIN brands b ON b.id = bm.brand_id
				           WHERE bm.user_id = x.id
				           ORDER BY bm.created_at, bm.id
				           LIMIT 1
				         )
				         WHEN 'partner' THEN (
				           SELECT '/mitra/' || p.public_id || '/changelog'
				           FROM partners p WHERE p.user_id = x.id ORDER BY p.id LIMIT 1
				         )
				       END,
				       NOW() - ($3 * INTERVAL '1 second')
				FROM users x
				WHERE x.is_active = TRUE AND x.role IN ('client', 'partner')`,
				notifTitle, summary, idx)
			if err != nil {
				// Kegagalan ping tidak membatalkan entri yang sudah masuk.
				slog.Warn("[platform] fan-out notifikasi changelog gagal", "err", err)
			} else {
				notified += n
			}
		} else {
			updated++
		}
		items = append(items, chgEntryJSON(row))
	}

	d.audit(r.Context(), user.ID, "changelog.imported", "changelog", firstID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":       true,
		"created":  created,
		"updated":  updated,
		"notified": notified,
		"items":    items,
	})
}

// AdminChangelogUpdate: ubah status terbit / isi satu entri.
func (d *Deps) AdminChangelogUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	entryID := chiParam(r, "entryId")
	existing, err := d.queryRow(r.Context(),
		"SELECT id FROM changelog_entries WHERE id = $1 LIMIT 1", entryID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound(chgNotFoundErr.Error()))
		return
	}
	body := decodeJSON(r)
	if published, has := boolField(body, "published"); has {
		if err := d.exec(r.Context(),
			"UPDATE changelog_entries SET published = $1, updated_at = NOW() WHERE id = $2",
			published, entryID); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	if title, has := strField(body, "title"); has {
		title = strings.TrimSpace(title)
		if runes := []rune(title); len(runes) < chgTitleMin || len(runes) > chgTitleMax {
			writeErr(w, d, errUnprocessable("Judul entri harus 3-160 karakter."))
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE changelog_entries SET title = $1, updated_at = NOW() WHERE id = $2",
			title, entryID); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	if html, has := strField(body, "body"); has {
		html = strings.TrimSpace(html)
		if html == "" || len([]rune(html)) > chgBodyMax {
			writeErr(w, d, errUnprocessable("Isi entri wajib diisi (maksimal 40000 karakter)."))
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE changelog_entries SET body = $1, updated_at = NOW() WHERE id = $2",
			html, entryID); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	if category, has := strField(body, "category"); has {
		category = strings.ToLower(strings.TrimSpace(category))
		if !chgCategorySet[category] {
			writeErr(w, d, errUnprocessable("Kategori harus salah satu dari fitur|perbaikan|performa|lainnya."))
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE changelog_entries SET category = $1, updated_at = NOW() WHERE id = $2",
			category, entryID); err != nil {
			writeErr(w, d, err)
			return
		}
	}
	row, err := d.queryRow(r.Context(), `
		SELECT e.id, e.version, e.released_on::text AS released_on, e.category, e.title,
		       e.summary, e.body, e.source_key, e.published, e.created_at
		FROM changelog_entries e WHERE e.id = $1 LIMIT 1`, entryID)
	if err != nil || row == nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "changelog.updated", "changelog", entryID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "entry": chgEntryJSON(row)})
}

// AdminChangelogDelete: hapus satu entri (pembacaan ikut ter-cascade).
func (d *Deps) AdminChangelogDelete(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	entryID := chiParam(r, "entryId")
	existing, err := d.queryRow(r.Context(),
		"SELECT id FROM changelog_entries WHERE id = $1 LIMIT 1", entryID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound(chgNotFoundErr.Error()))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM changelog_entries WHERE id = $1", entryID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "changelog.deleted", "changelog", entryID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
