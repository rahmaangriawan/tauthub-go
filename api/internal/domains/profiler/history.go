package profiler

// history.go: riwayat perubahan nilai field brief.
//
// Tabel profiler_field_history diisi oleh paket brief setiap kali nilai field
// berubah. Endpoint ini hanya membaca, dengan label field diambil dari skema
// aktif supaya frontend tidak perlu memetakan key ke nama yang bisa berubah.

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

type fieldHistoryRow struct {
	ID          string    `json:"id"`
	FieldKey    string    `json:"field_key"`
	FieldLabel  string    `json:"field_label"`
	OldValue    *string   `json:"old_value"`
	NewValue    *string   `json:"new_value"`
	Status      *string   `json:"status"`
	Source      *string   `json:"source"`
	ChangedBy   *string   `json:"changed_by"`
	ChangedName *string   `json:"changed_by_name"`
	ChangedAt   time.Time `json:"changed_at"`
}

// ProfileHistory: GET /v1/brands/{brandId}/profiler/profiles/{profileId}/history
//
// Query: field (opsional, satu key), limit (default 100, maks 500).
// Hanya viewer ke atas, dan profile harus milik brand di URL.
func (d *Deps) ProfileHistory(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "viewer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")
	if !d.profileInBrand(r.Context(), profileID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}

	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 500 {
		limit = 500
	}
	field := r.URL.Query().Get("field")

	// Label diambil dari skema aktif sebagai JSON, lalu dipetakan di Go. Cara
	// ini menghindari JOIN ke jsonb array yang mahal dan tetap benar saat admin
	// mengganti label field.
	labels := map[string]string{}
	if raw, err := d.loadActiveFieldsRaw(r.Context()); err == nil && len(raw) > 0 {
		var fields []struct {
			Key   string `json:"key"`
			Label string `json:"label"`
		}
		if json.Unmarshal(raw, &fields) == nil {
			for _, f := range fields {
				labels[f.Key] = f.Label
			}
		}
	}

	args := []any{profileID}
	where := "profile_id = $1"
	if field != "" {
		args = append(args, field)
		where += " AND field_key = $2"
	}
	args = append(args, limit)
	q := `
		SELECT h.id, h.field_key, h.old_value, h.new_value, h.status, h.source,
		       h.changed_by, u.display_name, h.changed_at
		FROM profiler_field_history h
		LEFT JOIN users u ON u.id = h.changed_by
		WHERE ` + where + `
		ORDER BY h.changed_at DESC
		LIMIT $` + strconv.Itoa(len(args))
	rows, err := d.Pool.Query(r.Context(), q, args...)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()

	out := make([]fieldHistoryRow, 0, limit)
	for rows.Next() {
		var row fieldHistoryRow
		if rows.Scan(&row.ID, &row.FieldKey, &row.OldValue, &row.NewValue, &row.Status,
			&row.Source, &row.ChangedBy, &row.ChangedName, &row.ChangedAt) != nil {
			continue
		}
		row.FieldLabel = labels[row.FieldKey]
		if row.FieldLabel == "" {
			row.FieldLabel = row.FieldKey
		}
		out = append(out, row)
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}

// loadActiveFieldsRaw mengambil array field dari versi skema aktif.
func (d *Deps) loadActiveFieldsRaw(ctx context.Context) ([]byte, error) {
	var raw []byte
	err := d.Pool.QueryRow(ctx,
		`SELECT fields FROM taut_brief_field_versions WHERE status = 'active' ORDER BY version DESC LIMIT 1`).
		Scan(&raw)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
