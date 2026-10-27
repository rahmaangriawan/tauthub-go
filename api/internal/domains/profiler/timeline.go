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
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
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
var inlineHashtagRe = regexp.MustCompile(`#([\p{L}\p{N}_]{1,40})`)

// hashtagsInBody: ambil #tagar yang diketik langsung di caption.
func hashtagsInBody(body string) []string {
	matches := inlineHashtagRe.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

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

// timelinePost: satu baris post beserta kolom turunannya.
type timelinePost struct {
	id, body        string
	hashtags        []string
	imageID         *string
	imageURL        *string
	linkURL         *string
	linkTitle       *string
	linkDescription *string
	linkImage       *string
	linkSite        *string
	occurredOn      *string
	createdAt       any
	authorName      string
}

func (p timelinePost) view() map[string]any {
	hashtags := p.hashtags
	if hashtags == nil {
		hashtags = []string{}
	}
	return map[string]any{
		"id": p.id, "body": p.body, "hashtags": hashtags,
		"image_url": p.imageURL, "image_id": p.imageID,
		"link_url": p.linkURL, "link_title": p.linkTitle,
		"link_description": p.linkDescription, "link_image": p.linkImage,
		"link_site": p.linkSite,
		"occurred_on": p.occurredOn, "created_at": p.createdAt,
		"author_name": p.authorName,
	}
}

func (d *Deps) listTimeline(ctx context.Context, profileID string) []map[string]any {
	rows, err := d.Pool.Query(ctx, `
		SELECT t.id, t.body, t.hashtags, t.image_url, t.occurred_on, t.image_id, t.created_at,
		       t.link_url, t.link_title, t.link_description, t.link_image, t.link_site,
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
		var p timelinePost
		if rows.Scan(&p.id, &p.body, &p.hashtags, &p.imageURL, &p.occurredOn, &p.imageID, &p.createdAt,
			&p.linkURL, &p.linkTitle, &p.linkDescription, &p.linkImage, &p.linkSite, &p.authorName) != nil {
			continue
		}
		out = append(out, p.view())
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

// ---------- pratinjau tautan (Open Graph) ----------

type linkMeta struct {
	URL, Title, Description, Image, Site string
}

var (
	reTitleTag = regexp.MustCompile(`(?is)<title[^>]*>([\s\S]*?)</title>`)
	reMetaTag  = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	reAttrProp = regexp.MustCompile(`(?is)(?:property|name)\s*=\s*["']([^"']+)["']`)
	reAttrCont = regexp.MustCompile(`(?is)content\s*=\s*["']([^"']*)["']`)
)

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// linkMetaFromHTML: kumpulkan og:*, twitter:*, dan meta description. Urutan
// atribut tidak dijamin, jadi property/name dan content dibaca terpisah per tag.
func linkMetaFromHTML(doc string) map[string]string {
	out := map[string]string{}
	for _, tag := range reMetaTag.FindAllString(doc, -1) {
		prop := reAttrProp.FindStringSubmatch(tag)
		cont := reAttrCont.FindStringSubmatch(tag)
		if len(prop) < 2 || len(cont) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(prop[1]))
		val := strings.Join(strings.Fields(html.UnescapeString(cont[1])), " ")
		if key != "" && val != "" && out[key] == "" {
			out[key] = val
		}
	}
	return out
}

func normalizeLinkURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("url kosong")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("url tidak valid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("skema tidak didukung")
	}
	return u, nil
}

// fetchLinkMeta: ambil <head> sekali (maks 512 KB) lalu baca metadata OG.
// security.Get memblokir alamat privat/loopback (anti-SSRF).
func fetchLinkMeta(ctx context.Context, rawURL string) (linkMeta, error) {
	u, err := normalizeLinkURL(rawURL)
	if err != nil {
		return linkMeta{}, err
	}
	meta := linkMeta{URL: u.String(), Site: strings.TrimPrefix(u.Hostname(), "www.")}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	resp, err := security.Get(ctx, u.String(), map[string]string{
		"User-Agent": "Tauthub-LinkPreview/1.0 (+https://tauthub.com)",
		"Accept":     "text/html,application/xhtml+xml",
	}, 8*time.Second)
	if err != nil {
		return meta, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return meta, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return meta, err
	}
	doc := string(body)
	og := linkMetaFromHTML(doc)
	meta.Title = firstNonEmpty(og["og:title"], og["twitter:title"])
	meta.Description = firstNonEmpty(og["og:description"], og["twitter:description"], og["description"])
	meta.Image = firstNonEmpty(og["og:image"], og["og:image:url"], og["twitter:image"])
	meta.Site = firstNonEmpty(og["og:site_name"], meta.Site)
	if meta.Title == "" {
		if m := reTitleTag.FindStringSubmatch(doc); len(m) > 1 {
			meta.Title = strings.Join(strings.Fields(html.UnescapeString(m[1])), " ")
		}
	}
	if meta.Image != "" {
		if ref, err := url.Parse(meta.Image); err == nil {
			meta.Image = resp.Request.URL.ResolveReference(ref).String()
		}
	}
	meta.Title = truncate(meta.Title, 200)
	meta.Description = truncate(meta.Description, 400)
	return meta, nil
}

// TimelineLinkPreview: GET .../profiler/link-preview?url=…
// Selalu 200; `ok:false` berarti pratinjau tidak tersedia dan UI cukup
// menampilkan tautannya saja.
func (d *Deps) TimelineLinkPreview(w http.ResponseWriter, r *http.Request) {
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
	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	if raw == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "URL kosong.")
		return
	}
	meta, err := fetchLinkMeta(r.Context(), raw)
	w.Header().Set("Cache-Control", "no-store")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "url": raw})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "url": meta.URL, "title": meta.Title,
		"description": meta.Description, "image": meta.Image, "site": meta.Site,
	})
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
	linkRaw, _ := strField(payload, "link_url")
	linkRaw = strings.TrimSpace(linkRaw)
	if body == "" && imageID == "" && linkRaw == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tulis caption, lampirkan gambar, atau tempel tautan.")
		return
	}
	if len([]rune(body)) > timelineBodyMax {
		httpx.Error(w, http.StatusUnprocessableEntity, "Caption maksimal 2000 karakter.")
		return
	}

	// Hashtag: array string, atau string bertanda spasi. Tagar yang diketik
	// langsung di caption (#promo) ikut dikumpulkan supaya bahan AI lengkap.
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
	tags := normalizeHashtags(append(rawTags, hashtagsInBody(body)...))

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

	// Tautan: pratinjau diambil sekali di sini lalu disimpan, supaya feed tidak
	// perlu menghubungi situs pihak ketiga setiap kali dibuka.
	var linkURL, linkTitle, linkDescription, linkImage, linkSite *string
	if linkRaw != "" {
		meta, err := fetchLinkMeta(r.Context(), linkRaw)
		if err != nil {
			if u, e := normalizeLinkURL(linkRaw); e == nil {
				plain, host := u.String(), strings.TrimPrefix(u.Hostname(), "www.")
				linkURL, linkSite = &plain, &host
			}
		} else {
			linkURL = &meta.URL
			linkSite = &meta.Site
			if meta.Title != "" {
				linkTitle = &meta.Title
			}
			if meta.Description != "" {
				linkDescription = &meta.Description
			}
			if meta.Image != "" {
				linkImage = &meta.Image
			}
		}
	}

	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO profiler_timeline_posts
			(id, profile_id, body, hashtags, image_id, image_url, occurred_on, created_by,
			 link_url, link_title, link_description, link_image, link_site)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		postID, profileID, body, tags, imgRef, imageURL, occurredOn, user.ID,
		linkURL, linkTitle, linkDescription, linkImage, linkSite); err != nil {
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
