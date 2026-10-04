package brandflow

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// emdash_media.go: unggah gambar artikel ke media library EmDash.
//
// EmDash menyimpan berkas lewat POST /_emdash/api/media (multipart, field
// "file", Bearer PAT dengan scope media:write) dan menyajikannya publik di
// /_emdash/api/media/file/<storageKey> — tanpa autentikasi. Sebelumnya adapter
// hanya mengirim markdown/HTML mentah, jadi gambar artikel tidak pernah sampai
// ke situs mitra.

// peEmdashUploadHook: titik injeksi untuk tes — menggantikan unggahan nyata ke
// media library EmDash. Nil berlaku = pakai peEmdashUploadMedia.
var peEmdashUploadHook func(ctx context.Context, d *Deps, baseURL, secret string, file peImageFile, friendlyName string) (string, string, error)

// peEmdashUpload: unggah satu berkas gambar ke media library EmDash.
func (d *Deps) peEmdashUpload(ctx context.Context, baseURL, secret string, file peImageFile, friendlyName string) (string, string, error) {
	if peEmdashUploadHook != nil {
		return peEmdashUploadHook(ctx, d, baseURL, secret, file, friendlyName)
	}
	return d.peEmdashUploadMedia(ctx, baseURL, secret, file, friendlyName)
}

// peEmdashUploadMedia: unggah satu berkas; balikan id media + URL publiknya.
func (d *Deps) peEmdashUploadMedia(ctx context.Context, baseURL, secret string, file peImageFile, friendlyName string) (string, string, error) {
	name := file.Filename
	if trimmed := strings.TrimSpace(friendlyName); trimmed != "" {
		ext := strings.ToLower(filepath.Ext(name))
		if ext == "" {
			ext = ".jpg"
		}
		slug := peSlugifyTerm(trimmed)
		if slug == "" {
			slug = "gambar-artikel"
		}
		if len(slug) > 80 {
			slug = slug[:80]
		}
		name = slug + ext
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", peSafeFilename(name))
	if err != nil {
		return "", "", err
	}
	if _, err := part.Write(file.Data); err != nil {
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}

	headers := emdashHeaders(secret)
	headers["Content-Type"] = writer.FormDataContentType()
	status, text, err := bsHTTPDoBytes(ctx, http.MethodPost, emdashURL(baseURL, "media"), headers, body.Bytes(), 60*time.Second)
	if err != nil {
		return "", "", err
	}
	data, err := emdashDecodeEnvelope(status, text, "media")
	if err != nil {
		return "", "", err
	}
	item, _ := data["item"].(map[string]any)
	if item == nil {
		return "", "", pePublishErr("EMDASH_NO_MEDIA", "EmDash tidak mengembalikan item media.")
	}
	id := bsStringifyAny(item["id"])
	if id == "" {
		return "", "", pePublishErr("EMDASH_NO_MEDIA", "EmDash tidak mengembalikan id media.")
	}
	publicURL := bsStringifyAny(item["url"])
	if publicURL == "" {
		if key := bsStringifyAny(item["storageKey"]); key != "" {
			publicURL = "/_emdash/api/media/file/" + key
		}
	}
	return id, publicURL, nil
}

// emdashUploadContentImages: unggah tiap gambar /uploads di badan artikel ke
// media library EmDash lalu ganti src-nya ke URL publik EmDash. cache
// (payload_snapshot.resolved.inline_media) membuat retry tidak mengunggah ulang.
func (d *Deps) emdashUploadContentImages(ctx context.Context, baseURL, secret, html string, cache map[string]any, friendlyName string) (string, map[string]any, []string) {
	updated := cache
	if updated == nil {
		updated = map[string]any{}
	}
	uploaded := 0
	rewrite := func(tag string) (string, bool, string) {
		value, vStart, vEnd, _ := peAttrValue(tag, "src")
		if value == "" || !peIsUploadImageRef(value) {
			return tag, false, ""
		}
		if entry, ok := updated[value].(map[string]any); ok {
			if url := bsStringifyAny(entry["url"]); url != "" {
				return peReplaceAttrValue(tag, vStart, vEnd, url), false, ""
			}
		}
		if uploaded >= peMaxInlineUploads {
			return tag, false, ""
		}
		file, err := d.peFetchUploadBytes(ctx, value)
		if err != nil {
			return tag, false, value + " (" + err.Error() + ")"
		}
		id, publicURL, err := d.peEmdashUpload(ctx, baseURL, secret, file, peInlineImageName(friendlyName, value))
		if err != nil {
			return tag, false, value + " (" + err.Error() + ")"
		}
		if publicURL == "" {
			publicURL = value
		}
		updated[value] = map[string]any{"id": id, "url": publicURL}
		uploaded++
		return peReplaceAttrValue(tag, vStart, vEnd, publicURL), true, ""
	}
	rewritten, failures := peRewriteImages(html, rewrite)
	return rewritten, updated, failures
}

// peEmdashImageRefs: peta src → referensi media untuk konverter Portable Text.
// Kunci ganda (URL asal dan URL hasil unggah) dipakai karena konversi berjalan
// setelah src di badan artikel sudah diganti ke URL media EmDash.
func peEmdashImageRefs(cache map[string]any) map[string]emdashImageRef {
	refs := map[string]emdashImageRef{}
	for src, raw := range cache {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ref := emdashImageRef{ID: bsStringifyAny(entry["id"]), URL: bsStringifyAny(entry["url"])}
		refs[src] = ref
		if ref.URL != "" {
			refs[ref.URL] = ref
		}
	}
	return refs
}

// peEmdashUploadCover: unggah gambar utama artikel ke media library EmDash.
func (d *Deps) peEmdashUploadCover(ctx context.Context, baseURL, secret, imageURL, title string) (emdashImageRef, error) {
	file, err := d.peFetchUploadBytes(ctx, imageURL)
	if err != nil {
		return emdashImageRef{}, err
	}
	id, publicURL, err := d.peEmdashUpload(ctx, baseURL, secret, file, title)
	if err != nil {
		return emdashImageRef{}, fmt.Errorf("unggah cover ke EmDash: %w", err)
	}
	return emdashImageRef{ID: id, URL: publicURL}, nil
}
