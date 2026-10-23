package brief

import (
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// BriefSummariesGet: GET .../brief/summaries
//
// Mengembalikan kedua ringkasan (entitas & file) beserta status proses dan
// penanda kedaluwarsa. UI memakai ini untuk menampilkan hasil dan polling
// selama job masih berjalan.
func (d *Deps) BriefSummariesGet(w http.ResponseWriter, r *http.Request) {
	_, _, profileID, ok := d.accessOrFail(w, r, "viewer")
	if !ok {
		return
	}
	// ?status=1 dipakai polling: cukup status + isi tersimpan, tanpa menghitung
	// ulang hash bahan (mahal bila banyak tab menunggu job yang sama).
	statusOnly := r.URL.Query().Get("status") == "1"
	var items any
	var err error
	if statusOnly {
		items, err = summary.LoadList(r.Context(), d.Pool, profileID)
	} else {
		items, err = summary.LoadWithStale(r.Context(), d.Pool, profileID)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// BriefSummaryRegenerate: POST .../brief/summaries/{kind}/regenerate
//
// Hanya menjadwalkan; hasilnya diambil lewat GET di atas. Dipakai tombol
// "Perbarui sekarang" ketika ringkasan gagal atau perlu disegarkan manual.
func (d *Deps) BriefSummaryRegenerate(w http.ResponseWriter, r *http.Request) {
	_, brandID, profileID, ok := d.accessOrFail(w, r, "writer")
	if !ok {
		return
	}
	kind, valid := normalizeKindParam(chiParam(r, "kind"))
	if !valid {
		httpx.Error(w, http.StatusNotFound, "Jenis ringkasan tidak dikenal.")
		return
	}
	if err := summary.QueueForce(r.Context(), d.Pool, profileID, brandID, kind); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "queued"})
}

// normalizeKindParam: "entitas"/"file" dari UI juga diterima.
func normalizeKindParam(raw string) (summary.Kind, bool) {
	if kind, ok := summary.ParseKind(raw); ok {
		return kind, true
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "entitas":
		return summary.KindEntity, true
	case "berkas":
		return summary.KindFile, true
	}
	return "", false
}
