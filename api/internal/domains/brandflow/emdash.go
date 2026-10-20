package brandflow

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// emdash.go: adapter EmDash CMS (REST di bawah /_emdash/api) — port
// lib/publish/emdash.ts + publishToEmdash di executor.ts aplikasi lama.
//
// Envelope EmDash: sukses { success: true, data: {...} }, gagal
// { success: false, error: { code, message } }. Autentikasi memakai personal
// access token: Authorization: Bearer <ec_pat_…>.
//
// Dua tata letak REST yang didukung:
//   - modern (EmDash 1.0.1): konten di /_emdash/api/content/{collection},
//     slug ada di envelope, update memakai PUT, item baru berstatus draft
//     sehingga perlu POST …/{id}/publish agar terbit.
//   - lama: konten di /_emdash/api/{collection}, slug di dalam data, update PATCH.
//
// Field asing di dalam data ditolak EmDash (unknown_field), jadi gambar utama
// dikirim sebagai featured_image dan diulang sekali tanpa field itu bila skema
// koleksi mitra tidak memilikinya.

const emdashDefaultCollection = "posts"

func emdashURL(baseURL, path string) string {
	cleanBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	cleanPath := strings.TrimLeft(path, "/")
	return cleanBase + "/_emdash/api/" + cleanPath
}

func emdashHeaders(secret string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(secret),
		"Content-Type":  "application/json",
	}
}

// emdashDialect: tata letak REST EmDash yang dipakai situs mitra.
type emdashDialect struct {
	modern bool
}

func (dl emdashDialect) contentPath(collection string) string {
	if dl.modern {
		return "content/" + collection
	}
	return collection
}

func (dl emdashDialect) updateMethod() string {
	if dl.modern {
		return http.MethodPut
	}
	return http.MethodPatch
}

// emdashDetectDialect: probe daftar koleksi di jalur modern; envelope sukses
// berarti EmDash 1.0.1+, selain itu pakai tata letak lama.
func emdashDetectDialect(ctx context.Context, baseURL, secret, collection string) emdashDialect {
	path := "content/" + collection + "?limit=1"
	status, body, err := bsHTTPDo(ctx, http.MethodGet, emdashURL(baseURL, path), emdashHeaders(secret), "", 15*time.Second)
	if err != nil {
		return emdashDialect{}
	}
	if _, err := emdashDecodeEnvelope(status, body, path); err != nil {
		return emdashDialect{}
	}
	return emdashDialect{modern: true}
}

// emdashDecodeEnvelope: buka envelope EmDash → data, atau error berpesan jelas
// (kode + pesan dari CMS, bukan sekadar HTTP status).
func emdashDecodeEnvelope(status int, body, path string) (map[string]any, error) {
	var env struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal([]byte(body), &env)
	if status >= 200 && status < 300 && env.Success {
		return env.Data, nil
	}
	if env.Error != nil && env.Error.Message != "" {
		return nil, pePublishErr(env.Error.Code, truncateString(env.Error.Message, 300))
	}
	return nil, pePublishErr("EMDASH_HTTP_ERROR",
		"EmDash membalas "+itoa(status)+" pada "+path+": "+truncateString(body, 200))
}

// emdashUnknownField: apakah kegagalan berasal dari field yang tidak ada di
// skema koleksi mitra (mis. featured_image) — kasus yang boleh dicoba ulang
// tanpa field opsional itu.
func emdashUnknownField(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unknown field") || strings.Contains(message, "unknown_field")
}

// bsTestEmdash: uji koneksi EmDash (parity testEmdash) — GET media?limit=1,
// sekaligus melaporkan tata letak REST yang dipakai situs mitra.
func (d *Deps) bsTestEmdash(ctx context.Context, conn bsRow) (string, string) {
	secret, err := d.bsConnectionSecret(conn)
	if err != nil {
		return "failed", "Token EmDash tidak dapat dibuka."
	}
	baseURL := conn.strOr("base_url", "")
	status, body, err := bsHTTPDo(ctx, http.MethodGet, emdashURL(baseURL, "media?limit=1"), emdashHeaders(secret), "", 15*time.Second)
	if err != nil {
		return "failed", truncateString(err.Error(), 300)
	}
	if _, err := emdashDecodeEnvelope(status, body, "media?limit=1"); err != nil {
		return "failed", truncateString(err.Error(), 300)
	}
	layout := "tata letak lama"
	if dialect := emdashDetectDialect(ctx, baseURL, secret, emdashDefaultCollection); dialect.modern {
		layout = "EmDash 1.0.1"
	}
	return "success", "Terhubung ke EmDash (" + layout + "). Pastikan token punya scope media:write dan content:write."
}

// emdashSlug: slug dokumen atau turunan judul.
func emdashSlug(document bsRow) string {
	if slug := strings.TrimSpace(document.strOr("slug", "")); slug != "" {
		return slug
	}
	return bsSlugify(document.strOr("title", ""))
}

// emdashContentPayload: data item untuk koleksi posts. slug tidak ikut di sini
// karena tata letak modern menaruh slug di envelope, dan field asing ditolak.
func emdashContentPayload(document bsRow, blocks []map[string]any, featured map[string]any, category string) map[string]any {
	data := map[string]any{
		"title":   document.strOr("title", ""),
		"content": blocks,
	}
	if excerpt := strings.TrimSpace(document.strOr("meta_description", "")); excerpt != "" {
		data["excerpt"] = excerpt
	}
	if featured != nil {
		data["featured_image"] = featured
	}
	// Kategori artikel (pilihan mitra atau kategori media). Responsibility
	// pembuatan/e-kategori di sisi EmDash; bila skema koleksi mitra tidak punya
	// field kategori, field ini ikut dilepas lewat emdashFieldCandidates.
	if c := strings.TrimSpace(category); c != "" {
		data["category"] = c
	}
	return data
}

// emdashFieldCandidates: urutan field opsional yang dicoba dilepas bila EmDash
// menolak suatu field karena tidak ada di skema koleksi mitra — dari yang paling
// tidak penting (gambar utama) sampai excerpt, dan kategori paling terakhir
// (kategori adalah bagian dari perilaku "buat otomatis di sistem target").
func emdashFieldCandidates(data map[string]any) []map[string]bool {
	candidates := []map[string]bool{{}}
	hasFeatured := data["featured_image"] != nil
	_, hasExcerpt := data["excerpt"]
	hasCategory := data["category"] != nil
	if hasFeatured {
		candidates = append(candidates, map[string]bool{"featured_image": true})
	}
	if hasExcerpt {
		candidates = append(candidates, map[string]bool{"excerpt": true})
		if hasFeatured {
			candidates = append(candidates, map[string]bool{"excerpt": true, "featured_image": true})
		}
	}
	if hasCategory {
		if hasFeatured {
			candidates = append(candidates, map[string]bool{"category": true, "featured_image": true})
		} else {
			candidates = append(candidates, map[string]bool{"category": true})
		}
		if hasExcerpt {
			if hasFeatured {
				candidates = append(candidates, map[string]bool{"category": true, "excerpt": true, "featured_image": true})
			} else {
				candidates = append(candidates, map[string]bool{"category": true, "excerpt": true})
			}
		}
	}
	return candidates
}

// emdashDropFields: salinan data tanpa field yang dilepas.
func emdashDropFields(data map[string]any, drops map[string]bool) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		if drops[k] {
			continue
		}
		out[k] = v
	}
	return out
}

func emdashFieldNames(drops map[string]bool) []string {
	names := make([]string, 0, len(drops))
	for name := range drops {
		names = append(names, name)
	}
	return names
}

// emdashWriteEnvelope: bentuk body tulis sesuai tata letak situs mitra.
func emdashWriteEnvelope(data map[string]any, slug, rev string, dialect emdashDialect) map[string]any {
	payload := map[string]any{}
	if dialect.modern {
		payload["data"] = data
		if slug != "" {
			payload["slug"] = slug
		}
	} else {
		legacyData := make(map[string]any, len(data)+1)
		for k, v := range data {
			legacyData[k] = v
		}
		if slug != "" {
			legacyData["slug"] = slug
		}
		payload["data"] = legacyData
	}
	if rev != "" {
		payload["_rev"] = rev
	}
	return payload
}

// emdashUpsertContent: buat/perbarui item konten di EmDash. Tanpa DB supaya
// bisa diuji langsung terhadap server tiruan. remoteID kosong = cari lewat
// slug dulu (anti-duplikat), lalu create; ada remoteID = update dengan _rev.
func emdashUpsertContent(ctx context.Context, baseURL, secret, collection, slug string, data map[string]any, remoteID, idempotencyKey string, dialect emdashDialect) (string, error) {
	headers := emdashHeaders(secret)
	path := dialect.contentPath(collection)
	if remoteID == "" {
		lookup := emdashURL(baseURL, path+"?slug="+urlQueryEscape(slug)+"&limit=1")
		if status, body, err := bsHTTPDo(ctx, http.MethodGet, lookup, headers, "", 15*time.Second); err == nil {
			if found, err := emdashDecodeEnvelope(status, body, path); err == nil {
				if items, ok := found["items"].([]any); ok && len(items) > 0 {
					if first, ok := items[0].(map[string]any); ok {
						remoteID = bsStringifyAny(first["id"])
					}
				}
			}
		}
	}

	if remoteID != "" {
		rev := ""
		itemPath := path + "/" + remoteID
		if status, body, err := bsHTTPDo(ctx, http.MethodGet, emdashURL(baseURL, itemPath), headers, "", 15*time.Second); err == nil {
			if item, err := emdashDecodeEnvelope(status, body, itemPath); err == nil {
				rev = bsStringifyAny(item["_rev"])
			} else if peStatusOf(err) == http.StatusNotFound {
				remoteID = ""
			}
		}
		if remoteID != "" {
			raw, _ := json.Marshal(emdashWriteEnvelope(data, slug, rev, dialect))
			status, body, err := bsHTTPDo(ctx, dialect.updateMethod(), emdashURL(baseURL, itemPath), headers, string(raw), 30*time.Second)
			if err != nil {
				return "", err
			}
			if _, err := emdashDecodeEnvelope(status, body, itemPath); err != nil {
				return "", err
			}
			return remoteID, nil
		}
	}

	raw, _ := json.Marshal(emdashWriteEnvelope(data, slug, "", dialect))
	headers["X-AI-Writer-Idempotency-Key"] = idempotencyKey
	status, body, err := bsHTTPDo(ctx, http.MethodPost, emdashURL(baseURL, path), headers, string(raw), 30*time.Second)
	if err != nil {
		return "", err
	}
	item, err := emdashDecodeEnvelope(status, body, path)
	if err != nil {
		return "", err
	}
	created, _ := item["item"].(map[string]any)
	id := bsStringifyAny(item["id"])
	if created != nil {
		if createdID := bsStringifyAny(created["id"]); createdID != "" {
			id = createdID
		}
	}
	if id == "" {
		return "", pePublishErr("EMDASH_NO_ID", "EmDash tidak mengembalikan id item.")
	}
	return id, nil
}

// emdashPublishItem: terbitkan item yang masih draft (khusus EmDash 1.0.1, yang
// hanya mengizinkan pembuatan item berstatus draft). Best-effort: kegagalan di
// sini dilaporkan sebagai peringatan, bukan kegagalan publikasi.
func emdashPublishItem(ctx context.Context, baseURL, secret, collection, remoteID string) error {
	path := "content/" + collection + "/" + remoteID + "/publish"
	status, body, err := bsHTTPDo(ctx, http.MethodPost, emdashURL(baseURL, path), emdashHeaders(secret), "{}", 30*time.Second)
	if err != nil {
		return err
	}
	if _, err := emdashDecodeEnvelope(status, body, path); err != nil {
		return err
	}
	return nil
}

// pePublishToEmdash: jalur publish satu percobaan untuk media ber-integrasi
// EmDash — antre ulang aman karena pencarian slug + idempotency key.
//
// Gambar badan artikel diunggah ke media library EmDash lalu dikonversi menjadi
// blok Portable Text; cover diunggah dan dikirim sebagai featured_image.
func (d *Deps) pePublishToEmdash(ctx context.Context, jobID, baseURL, secret, remotePostID string, document bsRow, html, publishStatus, coverURL, category string, snapshot map[string]any) error {
	if strings.TrimSpace(baseURL) == "" {
		return pePublishErr("EMDASH_NO_BASE_URL", "Base URL koneksi EmDash belum diisi.")
	}
	collection := emdashDefaultCollection
	dialect := emdashDetectDialect(ctx, baseURL, secret, collection)
	if snapshot == nil {
		snapshot = map[string]any{}
	}

	cache := peInlineMediaCache(snapshot)
	rewritten, cache, failures := d.emdashUploadContentImages(ctx, baseURL, secret, html, cache, document.strOr("title", ""))
	resolved, _ := snapshot["resolved"].(map[string]any)
	if resolved == nil {
		resolved = map[string]any{}
	}
	if len(cache) > 0 {
		resolved["inline_media"] = cache
	}
	if len(failures) > 0 {
		resolved["inline_upload_failed"] = failures
	}

	featured := map[string]any{}
	if cached, ok := resolved["emdash_cover"].(map[string]any); ok {
		if id := bsStringifyAny(cached["id"]); id != "" {
			featured = map[string]any{"provider": "local", "id": id}
		}
	} else if strings.TrimSpace(coverURL) != "" {
		if ref, err := d.peEmdashUploadCover(ctx, baseURL, secret, coverURL, document.strOr("title", "")); err == nil {
			featured = map[string]any{"provider": "local", "id": ref.ID}
			resolved["emdash_cover"] = map[string]any{"id": ref.ID, "url": ref.URL}
		} else {
			slog.Warn("[brandflow] cover EmDash gagal diunggah", "url", coverURL, "err", err)
			failures = append(failures, "cover: "+err.Error())
		}
	}
	snapshot["resolved"] = resolved
	if err := d.exec(ctx, "UPDATE publication_jobs SET payload_snapshot = $1::jsonb WHERE id = $2",
		marshalJSON(snapshot), jobID); err != nil {
		slog.Warn("[brandflow] gagal menyimpan cache gambar EmDash", "job", jobID, "err", err)
	}

	blocks := htmlToPortableText(rewritten, peEmdashImageRefs(cache))
	data := emdashContentPayload(document, blocks, featured, category)
	slug := emdashSlug(document)

	if err := d.exec(ctx, "UPDATE publication_jobs SET status = 'running', attempts = attempts + 1, progress = 70 WHERE id = $1", jobID); err != nil {
		return err
	}
	// EmDash menolak field yang tidak ada di skema koleksi mitra. Lepas field
	// opsional satu per satu supaya artikel tetap terbit, lalu catat peringatan.
	var remoteID string
	var err error
	var dropped []string
	candidates := emdashFieldCandidates(data)
	for i, drops := range candidates {
		remoteID, err = emdashUpsertContent(ctx, baseURL, secret, collection, slug,
			emdashDropFields(data, drops), strings.TrimSpace(remotePostID), jobID, dialect)
		if err == nil {
			if len(drops) > 0 {
				dropped = emdashFieldNames(drops)
			}
			break
		}
		if !emdashUnknownField(err) || i == len(candidates)-1 {
			break
		}
		slog.Warn("[brandflow] EmDash menolak field opsional, kirim ulang", "fields", emdashFieldNames(drops), "err", err)
	}
	if err != nil {
		return err
	}
	if len(dropped) > 0 {
		failures = append(failures, "field dilepas karena tidak ada di skema koleksi mitra: "+strings.Join(dropped, ", "))
	}
	if remoteID == "" {
		return errors.New("EmDash tidak mengembalikan id item.")
	}
	if dialect.modern && strings.TrimSpace(publishStatus) != "draft" {
		if err := emdashPublishItem(ctx, baseURL, secret, collection, remoteID); err != nil {
			slog.Warn("[brandflow] terbitkan item EmDash gagal", "remote_id", remoteID, "err", err)
			failures = append(failures, "item terkirim sebagai draft: "+err.Error())
		}
	}
	if len(failures) > 0 {
		d.peRecordWarning(ctx, jobID, failures)
	}
	remoteURL := strings.TrimRight(baseURL, "/") + "/" + collection + "/" + remoteID
	if dialect.modern {
		remoteURL = strings.TrimRight(baseURL, "/") + "/posts/" + slug
	}
	return d.exec(ctx,
		"UPDATE publication_jobs SET remote_post_id = $1, remote_url = $2, status = 'completed', progress = 100, error_message = NULL WHERE id = $3",
		remoteID, remoteURL, jobID)
}

// peStatusOf: ambil status HTTP dari error pePublishError bila ada (404 dipakai
// untuk memutuskan item EmDash sudah hilang → buat ulang).
func peStatusOf(err error) int {
	var pe *pePublishError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "NOT_FOUND", "not_found":
			return http.StatusNotFound
		}
	}
	return 0
}
