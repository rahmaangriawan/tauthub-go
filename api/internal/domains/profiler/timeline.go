package profiler

// timeline.go — Timeline brief brand: post bergaya feed (caption + satu gambar
// + hashtag) yang menjadi bahan SEGAR untuk AI Writer.
//
// Kenapa ada: artikel terasa generik bila hanya bersumber identitas statis.
// Kabar terbaru ("restoran kami dikunjungi Chef Juna", "promo Ramadan") yang
// dicatat di sini ikut masuk ke ringkasan File (lihat brief/summary/source.go),
// sehingga Writer punya sudut yang belum dipakai kompetitor.
//
// Retensi 3 bulan (permintaan pemilik): berjalan lewat worker upload-cleanup,
// yang menandai gambar terkait delete_pending agar tidak ada berkas yatim.

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

const (
	timelineBodyMax     = 2000
	timelineHashtagMax  = 10
	timelineHashtagLen  = 40
	timelineRetention   = 90 * 24 * time.Hour // 3 bulan
	timelineKeepInBrief = 12                 // post terbaru yang dikirim ke AI
)

// hashtagRe: huruf/angka/underscore, termasuk huruf beraksen.
var hashtagRe = regexp.MustCompile(`^[\p{L}\p{N}_]+$`)

// normalizeHashtags: buang "#" di depan, rapikan spasi, dedupe (case-insensitive),
// batasi jumlah & panjang. Nilai invalid dilewati, bukan menggagalkan post.
func normalizeHashtags(raw []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, h := range raw {
		h = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(h), "#"))
		if h == "" || len([]rune(h)) > timelineHashtagLen || !hashtagRe.MatchString(h) {
			continue
		}
		key := strings.ToLower(h)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, h)
		if len(out) >= timelineHashtagMax {
			break
		}
	}
	return out
}

// timelinePostView: satu kartu feed.
func timelinePostView(id, body string, hashtags []string, imageURL, occurredOn *string, imageID *string, createdAt any, authorName string) map[string]any {
	if hashtags == nil {
		hashtags = []string{}
	}
	return map[string]any{
		"id": id, "body": body, "hashtags": hashtags,
		"image_url": imageURL, "image_id": imageID,
		"occurred_on": occurredOn, "created_at": createdAt,
		"author_name": authorName,
	}
}

func (d *Deps) listTimeline(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx, `
		SELECT t.id, t.body, t.hashtags, t.image_url, t.occurred_on, t.image_id, t.created_at,
		       COALESCE(u.display_name, '') AS author_name
		FROM profiler_timeline_posts t
		LEFT JOIN users u ON u.id = t.created_by
		WHERE t.profile_id = $1 AND t.deleted_at IS NULL
		ORDER BY COALESCE(t.occurred_on, t.created_at::date) DESC, t.created_at DESC
		LIMIT 100`, profileID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var (
			id, body, author string
			hashtags         []string
			imageURL, occ    *string
			imageID          *string
			createdAt        any
		)
		if rows.Scan(&id, &body, &hashtags, &imageURL, &occ, &imageID, &createdAt, &author) != nil {
			continue
		}
		out = append(out, timelinePostView(id, body, hashtags, imageURL, occ, imageID, createdAt, author))
	}
	return out
}

// TimelineList: GET .../profiler/profiles/{profileId}/timeline
func (d *Deps) TimelineList(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": d.listTimeline(r.Context(), profileID)})
}

// TimelineCreate: POST .../profiler/profiles/{profileId}/timeline (JSON)
// Body: { body, hashtags: []string, image_id?, occurred_on? (YYYY-MM-DD) }
// Gambar diunggah lebih dulu lewat endpoint images yang sudah ada (kuota,
// sniff format, dan ledger berlaku), lalu ID-nya dirujuk di sini.
func (d *Deps) TimelineCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	profileID := chiParam(r, "profileId")
	if !d.profileInBrand(r.Context(), profileID, brandID) {
		httpx.Error(w, http.StatusNotFound, "Profiler tidak ditemukan.")
		return
	}
	payload := decodeJSON(r)
	body, _ := strField(payload, "body")
	body = strings.TrimSpace(body)
	imageID, _ := strField(payload, "image_id")
	imageID = strings.TrimSpace(imageID)
	if body == "" && imageID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tulis caption atau lampirkan gambar.")
		return
	}
	if len([]rune(body)) > timelineBodyMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Caption maksimal 2000 karakter.")
		return
	}

	// Hashtag: array string, atau string bertanda spasi.
	var rawTags []string
	if arr, ok := payload["hashtags"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				rawTags = append(rawTags, s)
			}
		}
	} else if s, _ := strField(payload, "hashtags"); strings.TrimSpace(s) != "" {
		rawTags = strings.Fields(s)
	}
	tags := normalizeHashtags(rawTags)

	var occurredOn any
	if raw, _ := strField(payload, "occurred_on"); strings.TrimSpace(raw) != "" {
		raw = strings.TrimSpace(raw)
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			occurredOn = t
		}
	}

	// Gambar harus milik profil ini (cegah merujuk gambar brief lain).
	var imageURL *string
	if imageID != "" {
		var url string
		err := d.Pool.QueryRow(r.Context(), `
			SELECT url FROM profiler_images
			WHERE id = $1 AND profile_id = $2 AND deleted_at IS NULL`, imageID, profileID).Scan(&url)
		if err != nil {
			httpx.Error(w, http.StatusUnprocessableEntity, "Gambar tidak ditemukan di brief ini.")
			return
		}
		imageURL = &url
	}

	postID := uuid.NewString()
	var imgRef any
	if imageID != "" {
		imgRef = imageID
	}
	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO profiler_timeline_posts (id, profile_id, body, hashtags, image_id, image_url, occurred_on, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		postID, profileID, body, tags, imgRef, imageURL, occurredOn, user.ID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": postID})
}

// TimelineDelete: DELETE .../profiler/timeline/{postId}
// Post ditandai hapus; gambarnya ikut ke antrean pembersihan fisik.
func (d *Deps) TimelineDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandID := chiParam(r, "brandId")
	if _, err := d.access(r, brandID, user.ID, "writer"); err != nil {
		writeErr(w, err)
		return
	}
	postID := chiParam(r, "postId")
	var profileID string
	var imageID *string
	if err := d.Pool.QueryRow(r.Context(), `
		SELECT t.profile_id, t.image_id
		FROM profiler_timeline_posts t
		JOIN profiler_profiles p ON p.id = t.profile_id
		WHERE t.id = $1 AND p.brand_id = $2 AND t.deleted_at IS NULL`, postID, brandID).
		Scan(&profileID, &imageID); err != nil {
		httpx.Error(w, http.StatusNotFound, "Post tidak ditemukan.")
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		`UPDATE profiler_timeline_posts SET deleted_at = NOW() WHERE id = $1`, postID); err != nil {
		writeErr(w, err)
		return
	}
	if imageID != nil {
		d.releaseTimelineImage(r.Context(), *imageID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// releaseTimelineImage: hapus baris gambar + tandai berkas fisiknya untuk
// dibersihkan worker (status delete_pending di ledger). Aman bila gambar sudah
// tidak ada.
func (d *Deps) releaseTimelineImage(ctx context.Context, imageID string) {
	var storageKey string
	if err := d.Pool.QueryRow(ctx,
		`SELECT storage_key FROM profiler_images WHERE id = $1`, imageID).Scan(&storageKey); err != nil {
		return
	}
	_, _ = d.Pool.Exec(ctx, `UPDATE profiler_images SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, imageID)
	d.markAttachmentPendingDelete(ctx, storageKey)
}

// purgeExpiredTimeline: hapus post berumur > 3 bulan beserta gambarnya.
// Dipanggil worker cleanup; mengembalikan jumlah post yang dihapus.
func (d *Deps) purgeExpiredTimeline(ctx context.Context) int64 {
	rows, err := d.Pool.Query(ctx, `
		UPDATE profiler_timeline_posts
		SET deleted_at = NOW()
		WHERE deleted_at IS NULL
		  AND created_at < now() - ($1 || ' seconds')::interval
		RETURNING image_id`, int64(timelineRetention/time.Second))
	if err != nil {
		return 0
	}
	var imageIDs []string
	var n int64
	for rows.Next() {
		var img *string
		if rows.Scan(&img) == nil {
			n++
			if img != nil && *img != "" {
				imageIDs = append(imageIDs, *img)
			}
		}
	}
	rows.Close()
	for _, id := range imageIDs {
		d.releaseTimelineImage(ctx, id)
	}
	return n
}
