package brandflow

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ---------- link-monitor/check (per brand) ----------

const (
	// lmManualCheckWindow: jeda minimum antar pengecekan manual satu brand.
	// Badge di UI menjanjikan "pengecekan manual maksimal 1x sehari", jadi
	// aturannya ditegakkan di server, bukan hanya di tampilan.
	lmManualCheckWindow = 24 * time.Hour
	// lmBrandSweepCap: batas jumlah publikasi yang diantrekan sekali tekan,
	// sama dengan batas sweep admin.
	lmBrandSweepCap = 200
)

// LinkMonitorCheckBrand: POST /v1/brands/{brandId}/link-monitor/check
//
// Versi brand dari tombol "Cek Sekarang" di admin: mengantrekan pengecekan
// tautan hanya untuk publikasi brand ini. Eligibility-nya sengaja identik
// dengan sweep admin/scheduler (terbit, punya URL, masih dalam garansi 12
// bulan, bukan draf, dan belum punya job pengecekan yang masih menunggu),
// supaya pengecekan manual tidak menduplikasi antrean yang sudah ada.
func (d *Deps) LinkMonitorCheckBrand(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	brandID := brandRow.ID

	// Jeda manual check dibaca dari audit_logs (aksi ini sudah tercatat di
	// sana), jadi tidak perlu tabel/kolom baru hanya untuk cooldown.
	var secondsSince int
	if err := d.Pool.QueryRow(r.Context(), `
		SELECT COALESCE(EXTRACT(EPOCH FROM (NOW() - MAX(created_at)))::int, 2147483647)
		  FROM audit_logs
		 WHERE brand_id = $1 AND action = 'link_monitor.manual_check'`, brandID).Scan(&secondsSince); err != nil {
		d.writeErr(w, err)
		return
	}
	if elapsed := time.Duration(secondsSince) * time.Second; elapsed < lmManualCheckWindow {
		next := time.Now().Add(lmManualCheckWindow - elapsed)
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"ok":                   false,
			"error":                "Pengecekan manual dibatasi 1x sehari.",
			"next_manual_check_at": next.UTC().Format(time.RFC3339),
		})
		return
	}

	rows, err := d.queryRows(r.Context(), `
		SELECT p.id FROM publications p
		LEFT JOIN publication_link_checks c ON c.publication_id = p.id
		WHERE p.brand_id = $1
		  AND p.status = 'published'
		  AND p.published_url IS NOT NULL
		  AND p.published_at > NOW() - INTERVAL '12 months'
		  AND p.publish_status <> 'draft'
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		     WHERE j.kind = 'publication_link_check' AND j.status IN ('pending', 'running')
		       AND j.payload->>'publicationId' = p.id
		  )
		ORDER BY c.last_checked_at NULLS FIRST
		LIMIT $2`, brandID, lmBrandSweepCap+1)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	capped := len(rows) > lmBrandSweepCap
	if capped {
		rows = rows[:lmBrandSweepCap]
	}

	queued := 0
	for _, row := range rows {
		publicationID := row.strOr("id", "")
		if publicationID == "" {
			continue
		}
		if _, err := d.enqueueJobAt(r.Context(), "publication_link_check",
			map[string]any{"publicationId": publicationID}, nil, 0); err == nil {
			queued++
		} else {
			slog.Debug("[brandflow] enqueue link check gagal", "publication", publicationID, "err", err)
		}
	}
	// Cooldown baru dihitung setelah benar-benar ada yang diantrekan. Kalau
	// antreannya kosong (semua baru diperiksa / sedang menunggu), jatah harian
	// tidak dipakai supaya mitra bisa mencoba lagi.
	if queued > 0 {
		d.audit(r.Context(), brandID, user.ID, "link_monitor.manual_check", "brand", brandID)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":                  true,
		"queued_publications": queued,
		"capped":              capped,
	})
}
