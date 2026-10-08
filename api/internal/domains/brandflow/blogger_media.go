package brandflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// blogger_media.go: gambar artikel untuk Blogger.
//
// API Blogger v3 tidak punya endpoint unggah media (resource resminya cuma
// blogs, posts, comments, pages, users), jadi gambar /uploads kita TIDAK bisa
// dititipkan ke Blogger. Solusinya: tiap gambar dimirror ke Google Drive milik
// mitra lewat Drive API v3 resmi (scope drive.file), lalu src di artikel
// diganti URL gambar Google. Dengan begitu gambar tidak lagi bergantung pada
// server Tauthub.

const (
	// peDriveUploadURL: files.create dengan metadata + bytes sekaligus.
	peDriveUploadURL  = "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart&fields=id,name,mimeType"
	peDriveFilesURL   = "https://www.googleapis.com/drive/v3/files"
	peDriveFolderMIME = "application/vnd.google-apps.folder"

	// peBloggerDriveScope: scope yang dipakai untuk menitipkan gambar.
	peBloggerDriveScope = "https://www.googleapis.com/auth/drive.file"

	// Kunci cache idempotensi di payload_snapshot.resolved.
	peBloggerMediaCacheKey  = "blogger_media"
	peBloggerMediaFailedKey = "blogger_media_failed"

	// Nama folder gambar di Drive mitra.
	peBloggerDriveFolderPrefix = "Tauthub - "
)

// peBloggerDriveSupportedMIME: format yang boleh dititipkan (sesuai permintaan:
// JPG/JPEG, PNG, WebP, dan GIF).
var peBloggerDriveSupportedMIME = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// peBloggerHasDriveScope: apakah token/kredensial punya izin Drive. Koneksi
// Blogger lama hanya punya scope blogger, jadi jalur ini dipakai untuk memutuskan
// mirror vs jatuh ke URL lama.
func peBloggerHasDriveScope(scope string) bool {
	for _, part := range strings.Fields(scope) {
		switch strings.TrimSpace(part) {
		case peBloggerDriveScope, "https://www.googleapis.com/auth/drive":
			return true
		}
	}
	return false
}

// peBloggerImageURL: URL gambar Google untuk berkas Drive yang sudah dibuka
// publik. webViewLink/webContentLink tidak render sebagai <img>, jadi dipakai
// host gambar Google yang merupakan cara standar menempel gambar Drive.
func peBloggerImageURL(fileID string) string {
	return "https://lh3.googleusercontent.com/d/" + url.PathEscape(fileID)
}

// peBloggerImageURLFallback: bentuk cadangan bila URL utama tidak melayani gambar.
func peBloggerImageURLFallback(fileID string) string {
	return "https://drive.google.com/thumbnail?id=" + url.PathEscape(fileID) + "&sz=w1600"
}

// peDriveMultipartBody: body multipart/related untuk files.create (bagian
// metadata JSON + bagian biner). Dipisah supaya bisa diuji tanpa jaringan.
func peDriveMultipartBody(meta map[string]any, file peImageFile) ([]byte, string, error) {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, "", err
	}
	boundary := "tauthub-drive-" + strconv.FormatUint(uint64(fnv32(file.Filename)), 16)
	var buf bytes.Buffer
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: application/json; charset=UTF-8\r\n\r\n")
	buf.Write(metaJSON)
	buf.WriteString("\r\n--" + boundary + "\r\n")
	if file.ContentType != "" {
		buf.WriteString("Content-Type: " + file.ContentType + "\r\n")
	}
	buf.WriteString("\r\n")
	buf.Write(file.Data)
	buf.WriteString("\r\n--" + boundary + "--\r\n")
	return buf.Bytes(), "multipart/related; boundary=" + boundary, nil
}

// peBloggerDriveFolderName: nama folder gambar per media.
func peBloggerDriveFolderName(mediaName, mediaPublicID string) string {
	name := strings.TrimSpace(mediaName)
	if name == "" {
		name = "Media"
	}
	name = strings.ReplaceAll(name, "'", "")
	if runes := []rune(name); len(runes) > 60 {
		name = string(runes[:60])
	}
	out := peBloggerDriveFolderPrefix + name
	if mediaPublicID != "" {
		out += " (" + mediaPublicID + ")"
	}
	return out
}

// peBloggerMediaFileName: nama berkas di Drive — judul artikel + hash URL asal
// (stabil antar retry) dan ekstensi aslinya supaya enak dilihat mitra.
func peBloggerMediaFileName(title, src, srcFilename string) string {
	base := peInlineImageName(title, src)
	ext := strings.ToLower(filepath.Ext(srcFilename))
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(pePathOf(src)))
	}
	switch ext {
	case ".jpeg", ".jpg", ".png", ".webp", ".gif":
		return base + ext
	default:
		return base + ".jpg"
	}
}

// pePathOf: bagian path dari URL (untuk mengambil ekstensi).
func pePathOf(value string) string {
	if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
		return parsed.Path
	}
	return value
}

// ---- Drive API ----

// peDriveFindFolder: cari folder gambar yang pernah dibuat aplikasi ini.
func (d *Deps) peDriveFindFolder(ctx context.Context, accessToken, name string) string {
	q := "name='" + strings.ReplaceAll(name, "'", "") + "' and mimeType='" + peDriveFolderMIME + "' and trashed=false"
	target := peDriveFilesURL + "?fields=files(id,name)&spaces=drive&q=" + url.QueryEscape(q)
	status, body, err := peHTTPRequest(ctx, http.MethodGet, target,
		map[string]string{"authorization": "Bearer " + accessToken}, "", 30*time.Second)
	if err != nil || status < 200 || status >= 300 {
		return ""
	}
	var parsed struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if json.Unmarshal(body, &parsed) != nil || len(parsed.Files) == 0 {
		return ""
	}
	return parsed.Files[0].ID
}

// peDriveCreateFolder: buat folder gambar (dipakai sekali per media).
func (d *Deps) peDriveCreateFolder(ctx context.Context, accessToken, name string) (string, error) {
	payload, err := json.Marshal(map[string]any{"name": name, "mimeType": peDriveFolderMIME})
	if err != nil {
		return "", err
	}
	status, body, err := peHTTPRequest(ctx, http.MethodPost, peDriveFilesURL+"?fields=id",
		map[string]string{"authorization": "Bearer " + accessToken, "content-type": "application/json"},
		string(payload), 30*time.Second)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("Drive menolak pembuatan folder (%d): %s", status, peDriveErrorSnippet(body))
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(body, &parsed) != nil || parsed.ID == "" {
		return "", fmt.Errorf("Drive tidak mengembalikan id folder")
	}
	return parsed.ID, nil
}

// peBloggerDriveFolder: folder gambar untuk satu media — pakai id dari kredensial
// bila ada, kalau belum cari/buat lalu simpan lewat pemanggil.
func (d *Deps) peBloggerDriveFolder(ctx context.Context, accessToken string, cred *peBloggerCredential, media bsRow) (string, error) {
	if cred.DriveFolderID != "" {
		return cred.DriveFolderID, nil
	}
	name := peBloggerDriveFolderName(media.strOr("name", ""), media.strOr("public_id", ""))
	if existing := d.peDriveFindFolder(ctx, accessToken, name); existing != "" {
		return existing, nil
	}
	return d.peDriveCreateFolder(ctx, accessToken, name)
}

// peDriveUploadImage: unggah bytes gambar ke Drive lalu buka akses bacanya
// untuk umum (gambar harus bisa dibaca pengunjung blog).
func (d *Deps) peDriveUploadImage(ctx context.Context, accessToken, parentID, name string, file peImageFile) (string, error) {
	meta := map[string]any{"name": name, "mimeType": file.ContentType}
	if parentID != "" {
		meta["parents"] = []string{parentID}
	}
	body, contentType, err := peDriveMultipartBody(meta, file)
	if err != nil {
		return "", err
	}
	status, resp, err := bsHTTPDoBytes(ctx, http.MethodPost, peDriveUploadURL, map[string]string{
		"authorization": "Bearer " + accessToken,
		"content-type":  contentType,
	}, body, 90*time.Second)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("Drive menolak unggahan (%d): %s", status, peDriveErrorSnippet([]byte(resp)))
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if json.Unmarshal([]byte(resp), &parsed) != nil || parsed.ID == "" {
		return "", fmt.Errorf("Drive tidak mengembalikan id berkas")
	}
	shareStatus, shareBody, shareErr := peHTTPRequest(ctx, http.MethodPost,
		peDriveFilesURL+"/"+url.PathEscape(parsed.ID)+"/permissions?fields=id",
		map[string]string{"authorization": "Bearer " + accessToken, "content-type": "application/json"},
		`{"role":"reader","type":"anyone"}`, 30*time.Second)
	if shareErr != nil {
		return "", fmt.Errorf("gagal membuka akses gambar (%v)", shareErr)
	}
	if shareStatus < 200 || shareStatus >= 300 {
		return "", fmt.Errorf("Drive menolak membuka akses gambar (%d): %s", shareStatus, peDriveErrorSnippet(shareBody))
	}
	return parsed.ID, nil
}

// peDriveErrorSnippet: ringkas pesan error Google untuk log.
func peDriveErrorSnippet(body []byte) string {
	if len(body) == 0 {
		return "(tanpa pesan)"
	}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Message != "" {
		return truncateString(parsed.Error.Message, 200)
	}
	return truncateString(string(body), 200)
}

// peBloggerImageReachable: pastikan URL gambar benar-benar melayani gambar
// (bukan halaman HTML atau 403). Dipakai untuk memilih URL utama atau cadangan.
func peBloggerImageReachable(ctx context.Context, imageURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := security.SafeClient(15 * time.Second).Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	return strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/")
}

// peBloggerMirrorableURL: apakah URL gambar perlu dititipkan ke Drive. Berkas
// /uploads kita selalu dititipkan; URL di domain kita sendiri (mis. aset
// statis) ikut dititipkan; URL pihak ketiga yang sudah publik dibiarkan apa
// adanya supaya tidak ada unggahan yang tidak perlu.
func peBloggerMirrorableURL(publicOrigin, value string) bool {
	if peIsUploadImageRef(value) {
		return true
	}
	origin := strings.TrimRight(strings.TrimSpace(publicOrigin), "/")
	return origin != "" && strings.HasPrefix(value, origin+"/")
}

// peMirrorBloggerImage: satu gambar /uploads → URL Drive (memakai cache agar
// retry tidak mengunggah ulang). Balikan kedua (verified) menandai apakah URL
// gambarnya sudah dipastikan melayani gambar; gagal unggah tetap error, tetapi
// verifikasi yang tidak konklusif tidak menggagalkan publikasi.
func (d *Deps) peMirrorBloggerImage(ctx context.Context, accessToken, folderID, src, title string, cache map[string]any) (string, bool, error) {
	if cache == nil {
		// Pemanggil selalu menyiapkan map agar hasilnya bisa disimpan sebagai
		// cache idempotensi; ini jaring pengaman supaya tidak panic.
		cache = map[string]any{}
	}
	if entry, ok := cache[src].(map[string]any); ok {
		if url := bsStringifyAny(entry["url"]); url != "" {
			if entry["verified"] != nil {
				return url, entry["verified"] == true, nil
			}
			return url, true, nil
		}
	}
	file, err := d.peFetchUploadBytes(ctx, src)
	if err != nil {
		return "", false, err
	}
	if !peBloggerDriveSupportedMIME[strings.ToLower(file.ContentType)] {
		return "", false, fmt.Errorf("format %s tidak didukung untuk gambar Blogger", file.ContentType)
	}
	if int64(len(file.Data)) > peMaxImageBytes {
		return "", false, fmt.Errorf("ukuran gambar %d byte melebihi batas %d byte", len(file.Data), peMaxImageBytes)
	}
	fileID, err := d.peDriveUploadImage(ctx, accessToken, folderID, peBloggerMediaFileName(title, src, file.Filename), file)
	if err != nil {
		return "", false, err
	}
	imageURL := peBloggerImageURL(fileID)
	verified := peBloggerImageReachable(ctx, imageURL)
	if !verified {
		if fallback := peBloggerImageURLFallback(fileID); peBloggerImageReachable(ctx, fallback) {
			imageURL = fallback
			verified = true
		}
	}
	cache[src] = map[string]any{"file_id": fileID, "url": imageURL, "verified": verified}
	return imageURL, verified, nil
}

// peMirrorBloggerImages: ganti tiap <img src="/uploads/..."> di HTML dengan URL
// Drive. URL luar (sudah publik) dibiarkan. Balikan HTML baru + daftar gagal.
func (d *Deps) peMirrorBloggerImages(ctx context.Context, accessToken, folderID, html, title string, cache map[string]any) (string, []string) {
	if html == "" || !strings.Contains(html, "<img") {
		return html, nil
	}
	uploaded := 0
	rewrite := func(tag string) (string, bool, string) {
		value, vStart, vEnd, _ := peAttrValue(tag, "src")
		if value == "" || !peIsUploadImageRef(value) {
			return tag, false, ""
		}
		if entry, ok := cache[value].(map[string]any); ok {
			if url := bsStringifyAny(entry["url"]); url != "" {
				return peReplaceAttrValue(tag, vStart, vEnd, url), false, ""
			}
		}
		if uploaded >= peMaxInlineUploads {
			return tag, false, ""
		}
		url, _, err := d.peMirrorBloggerImage(ctx, accessToken, folderID, value, title, cache)
		if err != nil {
			slog.Warn("[brandflow] gambar Blogger tidak bisa dititipkan ke Drive", "url", value, "err", err)
			return tag, false, value + " (" + err.Error() + ")"
		}
		uploaded++
		return peReplaceAttrValue(tag, vStart, vEnd, url), true, ""
	}
	rewritten, failures := peRewriteImages(html, rewrite)
	return rewritten, failures
}

// peBloggerMediaCache: baca cache hasil mirror dari payload_snapshot.
func peBloggerMediaCache(snapshot map[string]any) map[string]any {
	resolved, _ := snapshot["resolved"].(map[string]any)
	if resolved == nil {
		return nil
	}
	cache, _ := resolved[peBloggerMediaCacheKey].(map[string]any)
	return cache
}

// peSaveBloggerMediaCache: simpan cache + daftar gagal ke payload_snapshot.
func (d *Deps) peSaveBloggerMediaCache(ctx context.Context, jobID string, snapshot, cache map[string]any, failures []string) {
	if snapshot == nil || len(cache) == 0 && len(failures) == 0 {
		return
	}
	resolved, _ := snapshot["resolved"].(map[string]any)
	if resolved == nil {
		resolved = map[string]any{}
	}
	if len(cache) > 0 {
		resolved[peBloggerMediaCacheKey] = cache
	}
	if len(failures) > 0 {
		resolved[peBloggerMediaFailedKey] = failures
	}
	snapshot["resolved"] = resolved
	if err := d.exec(ctx, "UPDATE publication_jobs SET payload_snapshot = $1::jsonb WHERE id = $2",
		marshalJSON(snapshot), jobID); err != nil {
		slog.Warn("[brandflow] gagal menyimpan cache gambar Blogger", "job", jobID, "err", err)
	}
}
