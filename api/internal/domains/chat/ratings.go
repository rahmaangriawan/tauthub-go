package chat

import (
	"net/http"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Rating kecepatan terbit per media (tabel media_ratings).
//
// Sebelumnya nilai bintang hanya bisa masuk lewat pembukaan forum obrolan
// ("Hubungi" → rating ikut di body). Tombol "Beri Rating" di ringkasan memberi
// jalur langsung tanpa harus membuka obrolan, jadi butuh GET untuk memuat nilai
// yang sudah ada dan PUT untuk menyimpannya.
//
// Satu brand hanya punya satu rating per media (dinilai ulang = menimpa),
// mengikuti unique index media_ratings_brand_media_uidx.

// ratingPayload: bentuk balasan GET/PUT — nilai null berarti belum pernah dinilai.
func ratingPayload(r row) map[string]any {
	var rating any
	var updatedAt any
	if r != nil {
		if r.raw("rating") != nil {
			rating = r.intOr("rating", 0)
		}
		updatedAt = r.timePtr("updated_at")
	}
	return map[string]any{"rating": rating, "updated_at": updatedAt}
}

// MediaRatingGet: GET /v1/brands/{brandId}/media/{mediaId}/rating
func (d *Deps) MediaRatingGet(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.brandAccess(r, user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT rating, updated_at FROM media_ratings WHERE brand_id = $1 AND media_id = $2 LIMIT 1",
		brand.ID, chiURLParam(r, "mediaId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ratingPayload(row))
}

// MediaRatingPut: PUT /v1/brands/{brandId}/media/{mediaId}/rating {rating, publication_id?}
func (d *Deps) MediaRatingPut(w http.ResponseWriter, r *http.Request) {
	user := authUser(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brand, err := d.brandAccess(r, user.ID, "viewer")
	if err != nil {
		writeErr(w, err)
		return
	}
	mediaID := chiURLParam(r, "mediaId")
	body := decodeJSON(r)
	rating, ok := ratingField(body)
	if !ok {
		httpx.Error(w, http.StatusUnprocessableEntity, "Rating wajib diisi dengan angka 1 sampai 5.")
		return
	}

	// Rating hanya masuk akal untuk media yang memang dipakai brand ini.
	// publication_id opsional: bila dikirim harus milik brand & media yang sama.
	publicationID := strField(body, "publication_id")
	pub, err := d.queryRow(r.Context(), `
		SELECT id FROM publications
		WHERE brand_id = $1 AND media_id = $2 AND ($3 = '' OR id = $3)
		ORDER BY created_at DESC LIMIT 1`, brand.ID, mediaID, publicationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if pub == nil {
		httpx.Error(w, http.StatusNotFound, "Belum ada publikasi brand ini pada media tersebut.")
		return
	}

	if err := d.exec(r.Context(), `
		INSERT INTO media_ratings (id, media_id, brand_id, rating, publication_id, created_by)
		VALUES ((gen_random_uuid())::text, $1, $2, $3, $4, $5)
		ON CONFLICT (brand_id, media_id) DO UPDATE
		   SET rating = EXCLUDED.rating,
		       publication_id = EXCLUDED.publication_id,
		       updated_at = now()`,
		mediaID, brand.ID, rating, pub.strOr("id", ""), user.ID); err != nil {
		writeErr(w, err)
		return
	}

	saved, err := d.queryRow(r.Context(),
		"SELECT rating, updated_at FROM media_ratings WHERE brand_id = $1 AND media_id = $2 LIMIT 1",
		brand.ID, mediaID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ratingPayload(saved))
}
