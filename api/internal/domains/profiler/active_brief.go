package profiler

import (
	"net/http"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Brief aktif: brief yang dipilih pengguna lewat tombol "Pakai untuk Artikel"
// di Taut Brief. Pengelola Naskah memilihnya otomatis supaya klien tidak perlu
// memilih ulang di tiap artikel. Disimpan per pengguna per brand.

// ActiveBriefGet: GET /v1/brands/{brandId}/profiler/active-brief
// Mengembalikan {"profile_id": "<id>"} atau {"profile_id": null}. Brief yang
// sudah dihapus tidak dikembalikan.
func (d *Deps) ActiveBriefGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	var id string
	err = d.Pool.QueryRow(r.Context(), `
		SELECT a.profile_id
		FROM taut_brief_active a
		JOIN profiler_profiles p ON p.id = a.profile_id AND p.deleted_at IS NULL
		WHERE a.brand_id = $1 AND a.user_id = $2`, brand.ID, user.ID).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"profile_id": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile_id": id})
}

// ActiveBriefPut: PUT /v1/brands/{brandId}/profiler/active-brief
// Body {"profile_id": "<id>"}; kosong atau null melepas pilihan.
func (d *Deps) ActiveBriefPut(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	profileID, _ := strField(body, "profile_id")
	if profileID == "" {
		if _, err := d.Pool.Exec(r.Context(),
			"DELETE FROM taut_brief_active WHERE brand_id = $1 AND user_id = $2", brand.ID, user.ID); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"profile_id": nil})
		return
	}
	if !d.profileInBrand(r.Context(), profileID, brand.ID) {
		httpx.Error(w, http.StatusNotFound, "Brief tidak ditemukan.")
		return
	}
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO taut_brief_active (brand_id, user_id, profile_id, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (brand_id, user_id) DO UPDATE SET profile_id = EXCLUDED.profile_id, updated_at = NOW()`,
		brand.ID, user.ID, profileID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile_id": profileID})
}
