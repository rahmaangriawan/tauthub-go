package platform

import (
	"net/http"
	"strconv"
	"strings"
)

// Log job media untuk tab "Log" di /admin/media.
//
// Sebelumnya pesan kegagalan (mis. "ERROR: new row for relation
// media_seo_metrics violates check constraint ...") hanya muncul sebagai panel
// kuning di atas tabel direktori. Tempatnya salah: panel itu memakan ruang di
// halaman kerja utama, hilang begitu halaman dimuat ulang, dan tidak
// menyimpan riwayat — padahal justru pesan seperti itu yang perlu ditelusuri.
//
// Endpoint ini membaca tabel `jobs` apa adanya (sumber kebenaran antrean) dan
// mengembalikan riwayat terbaru beserta media yang terkait, supaya admin bisa
// melihat pola kegagalan, bukan hanya satu pesan terakhir.

// mpMediaJobKinds: jenis job yang ditampilkan di tab Log. Semuanya berhubungan
// dengan pemeriksaan media; job sistem lain (email, publikasi, dsb.) sengaja
// tidak ikut supaya tab ini tetap fokus.
var mpMediaJobKinds = []string{
	"media_seo_refresh",
	"media_geo_refresh",
	"media_similarweb_refresh",
	"media_ai_summary",
	"domain_verify",
	"crawl_target_sync",
}

// MediaJobLog: GET /v1/platform/media/job-log
//
// Query: status (opsional: failed|pending|running|completed|cancelled),
// kind (opsional), limit (default 50, maks 200), offset (default 0).
//
// Paginasi disediakan di server karena tabel jobs terus bertambah: tanpa
// offset, "limit 200" hanya menampilkan 200 baris terbaru dan riwayat lama
// tidak bisa dijangkau sama sekali. `total` dihitung dengan filter yang sama
// supaya UI bisa menampilkan jumlah halaman.
func (d *Deps) MediaJobLog(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	q := r.URL.Query()
	statusFilter := strings.TrimSpace(q.Get("status"))
	kindFilter := strings.TrimSpace(q.Get("kind"))
	limit := mpClampInt(q.Get("limit"), 50, 1, 200)
	offset := mpClampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{"j.kind = ANY($1)"}
	args := []any{mpMediaJobKinds}
	if statusFilter != "" {
		args = append(args, statusFilter)
		where = append(where, "j.status = $"+strconv.Itoa(len(args)))
	}
	if kindFilter != "" {
		args = append(args, kindFilter)
		where = append(where, "j.kind = $"+strconv.Itoa(len(args)))
	}
	clause := "WHERE " + strings.Join(where, " AND ")

	// Total untuk filter aktif. Dihitung terpisah dari halaman agar angka
	// "x dari y" tidak berubah hanya karena limit.
	totalRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS n FROM jobs j "+clause, args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	total := 0
	if totalRow != nil {
		total = totalRow.intOr("n", 0)
	}

	pageArgs := append(append([]any{}, args...), limit, offset)
	rows, err := d.queryRows(r.Context(), `
		SELECT j.id, j.kind, j.status, j.attempts, j.max_attempts,
		       COALESCE(j.last_error, '') AS last_error,
		       j.payload->>'mediaId' AS media_id,
		       COALESCE(m.name, '') AS media_name,
		       COALESCE(m.website_url, '') AS website_url,
		       j.created_at, j.updated_at
		FROM jobs j
		LEFT JOIN media m ON m.id = (j.payload->>'mediaId')
		`+clause+`
		ORDER BY j.updated_at DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), pageArgs...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		row := ptRow(ptNormalizeMap(raw))
		items = append(items, map[string]any{
			"id":           row.anyOrNil("id"),
			"kind":         row.strOr("kind", ""),
			"status":       row.strOr("status", ""),
			"attempts":     row.intOr("attempts", 0),
			"max_attempts": row.intOr("max_attempts", 0),
			"last_error":   row.strOr("last_error", ""),
			"media_id":     row.anyOrNil("media_id"),
			"media_name":   row.strOr("media_name", ""),
			"website_url":  row.strOr("website_url", ""),
			"created_at":   row.anyOrNil("created_at"),
			"updated_at":   row.anyOrNil("updated_at"),
		})
	}

	// Ringkasan per status supaya UI bisa menampilkan chip angka tanpa
	// menghitung ulang di klien (dan tanpa bergantung pada limit halaman).
	summaryRows, err := d.queryRows(r.Context(), `
		SELECT status, COUNT(*) AS n
		FROM jobs
		WHERE kind = ANY($1) AND updated_at > NOW() - INTERVAL '24 hours'
		GROUP BY status`, mpMediaJobKinds)
	summary := map[string]int{}
	if err == nil {
		for _, raw := range summaryRows {
			row := ptRow(ptNormalizeMap(raw))
			summary[row.strOr("status", "")] = row.intOr("n", 0)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"kinds":  mpMediaJobKinds,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"summary_24h": map[string]any{
			"failed":    summary["failed"],
			"pending":   summary["pending"],
			"running":   summary["running"],
			"completed": summary["completed"],
			"cancelled": summary["cancelled"],
		},
	})
}
