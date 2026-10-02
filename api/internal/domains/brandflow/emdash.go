package brandflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	headingPattern     = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	orderedItemPattern = regexp.MustCompile(`^\d+[.)]\s+(.*)$`)
	bulletItemPattern  = regexp.MustCompile(`^[-*+•]\s+(.*)$`)
	linkPattern        = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\s)]+)\)`)
)

// emdash.go: adapter EmDash CMS (REST di bawah /_emdash/api) — port
// lib/publish/emdash.ts + publishToEmdash di executor.ts aplikasi lama.
//
// Envelope EmDash: sukses { success: true, data: {...} }, gagal
// { success: false, error: { code, message } }. Autentikasi memakai personal
// access token: Authorization: Bearer <ec_pat_…>.
//
// Pemetaan field mengikuti konvensi lama: collection "posts" dengan field
// { title, content, excerpt, slug, featuredImage }. excerpt diisi dari
// meta_description artikel, jadi cuplikan singkat ikut tayang di situs media.

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

// markdownToPortableText: blok Portable Text dari markdown sederhana —
// paragraf, heading (#..######), daftar bullet/nomor, serta inline tebal,
// miring, dan tautan. Port dari lib/publish/portable-text.ts.
func markdownToPortableText(markdown string) []map[string]any {
	blocks := []map[string]any{}
	var paragraf []string
	listType := ""
	var listItems []map[string]any

	flushParagraf := func() {
		if len(paragraf) == 0 {
			return
		}
		blocks = append(blocks, map[string]any{
			"_type": "block", "style": "normal",
			"children": inlineSpans(strings.Join(paragraf, " ")), "markDefs": inlineMarkDefs(strings.Join(paragraf, " ")),
		})
		paragraf = nil
	}
	flushList := func() {
		if len(listItems) == 0 {
			return
		}
		blocks = append(blocks, listItems...)
		listItems = nil
		listType = ""
	}

	for _, raw := range strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			flushParagraf()
			flushList()
			continue
		}
		if m := headingPattern.FindStringSubmatch(line); m != nil {
			flushParagraf()
			flushList()
			level := len(m[1])
			blocks = append(blocks, map[string]any{
				"_type": "block", "style": "h" + itoa(level),
				"children": inlineSpans(m[2]), "markDefs": inlineMarkDefs(m[2]),
			})
			continue
		}
		if m := orderedItemPattern.FindStringSubmatch(line); m != nil {
			flushParagraf()
			if listType != "number" {
				flushList()
				listType = "number"
			}
			listItems = append(listItems, listBlock(m[1], "number"))
			continue
		}
		if m := bulletItemPattern.FindStringSubmatch(line); m != nil {
			flushParagraf()
			if listType != "bullet" {
				flushList()
				listType = "bullet"
			}
			listItems = append(listItems, listBlock(m[1], "bullet"))
			continue
		}
		flushList()
		paragraf = append(paragraf, line)
	}
	flushParagraf()
	flushList()
	return blocks
}

func listBlock(teks, listType string) map[string]any {
	return map[string]any{
		"_type": "block", "style": "normal",
		"listItem": listType, "level": 1,
		"children": inlineSpans(teks), "markDefs": inlineMarkDefs(teks),
	}
}

// inlineSpans: satu span per baris teks (mengikuti converter lama yang tidak
// memecah tebal/miring menjadi span terpisah).
func inlineSpans(teks string) []map[string]any {
	return []map[string]any{{"_type": "span", "text": teks, "marks": []string{}}}
}

// inlineMarkDefs: definisi tautan untuk mark "link:<href>" yang dipakai teks.
func inlineMarkDefs(teks string) []map[string]any {
	defs := []map[string]any{}
	seen := map[string]bool{}
	for _, m := range linkPattern.FindAllStringSubmatch(teks, -1) {
		href := strings.TrimSpace(m[1])
		if href == "" || seen[href] {
			continue
		}
		seen[href] = true
		defs = append(defs, map[string]any{"_key": "link:" + href, "_type": "link", "href": href})
	}
	return defs
}

// bsTestEmdash: uji koneksi EmDash (parity testEmdash) — GET media?limit=1.
func (d *Deps) bsTestEmdash(ctx context.Context, conn bsRow) (string, string) {
	secret, err := d.bsConnectionSecret(conn)
	if err != nil {
		return "failed", "Token EmDash tidak dapat dibuka."
	}
	url := emdashURL(conn.strOr("base_url", ""), "media?limit=1")
	status, body, err := bsHTTPDo(ctx, http.MethodGet, url, emdashHeaders(secret), "", 15*time.Second)
	if err != nil {
		return "failed", truncateString(err.Error(), 300)
	}
	if _, err := emdashDecodeEnvelope(status, body, "media?limit=1"); err != nil {
		return "failed", truncateString(err.Error(), 300)
	}
	return "success", "Terhubung ke EmDash dan token diterima."
}

// emdashContentPayload: data item sesuai konvensi field lama; excerpt diambil
// dari meta_description supaya cuplikan singkat ikut terkirim.
func emdashContentPayload(document bsRow, markdown string) map[string]any {
	slug := document.strOr("slug", "")
	if slug == "" {
		slug = bsSlugify(document.strOr("title", ""))
	}
	data := map[string]any{
		"title":   document.strOr("title", ""),
		"content": markdownToPortableText(markdown),
		"slug":    slug,
	}
	if excerpt := strings.TrimSpace(document.strOr("meta_description", "")); excerpt != "" {
		data["excerpt"] = excerpt
	}
	return data
}

// emdashUpsertContent: buat/perbarui item konten di EmDash. Tanpa DB supaya
// bisa diuji langsung terhadap server tiruan. remoteID kosong = cari lewat
// slug dulu (anti-duplikat), lalu create; ada remoteID = update dengan _rev.
func emdashUpsertContent(ctx context.Context, baseURL, secret, collection, slug string, data map[string]any, remoteID, idempotencyKey string) (string, error) {
	headers := emdashHeaders(secret)
	if remoteID == "" {
		lookup := emdashURL(baseURL, collection+"?slug="+urlQueryEscape(slug)+"&limit=1")
		if status, body, err := bsHTTPDo(ctx, http.MethodGet, lookup, headers, "", 15*time.Second); err == nil {
			if found, err := emdashDecodeEnvelope(status, body, collection); err == nil {
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
		getURL := emdashURL(baseURL, collection+"/"+remoteID)
		if status, body, err := bsHTTPDo(ctx, http.MethodGet, getURL, headers, "", 15*time.Second); err == nil {
			if item, err := emdashDecodeEnvelope(status, body, collection); err == nil {
				rev = bsStringifyAny(item["_rev"])
			} else if peStatusOf(err) == http.StatusNotFound {
				remoteID = ""
			}
		}
		if remoteID != "" {
			payload := map[string]any{"data": data}
			if rev != "" {
				payload["_rev"] = rev
			}
			raw, _ := json.Marshal(payload)
			url := emdashURL(baseURL, collection+"/"+remoteID)
			status, body, err := bsHTTPDo(ctx, http.MethodPatch, url, headers, string(raw), 30*time.Second)
			if err != nil {
				return "", err
			}
			if _, err := emdashDecodeEnvelope(status, body, collection+"/"+remoteID); err != nil {
				return "", err
			}
			return remoteID, nil
		}
	}

	raw, _ := json.Marshal(map[string]any{"data": data})
	headers["X-AI-Writer-Idempotency-Key"] = idempotencyKey
	url := emdashURL(baseURL, collection)
	status, body, err := bsHTTPDo(ctx, http.MethodPost, url, headers, string(raw), 30*time.Second)
	if err != nil {
		return "", err
	}
	item, err := emdashDecodeEnvelope(status, body, collection)
	if err != nil {
		return "", err
	}
	id := bsStringifyAny(item["id"])
	if id == "" {
		return "", pePublishErr("EMDASH_NO_ID", "EmDash tidak mengembalikan id item.")
	}
	return id, nil
}

// pePublishToEmdash: jalur publish satu percobaan untuk media ber-integrasi
// EmDash — antre ulang aman karena pencarian slug + idempotency key.
func (d *Deps) pePublishToEmdash(ctx context.Context, jobID, baseURL, secret, remotePostID string, document bsRow, markdown string) error {
	if strings.TrimSpace(baseURL) == "" {
		return pePublishErr("EMDASH_NO_BASE_URL", "Base URL koneksi EmDash belum diisi.")
	}
	collection := emdashDefaultCollection
	data := emdashContentPayload(document, markdown)
	slug := bsStringifyAny(data["slug"])

	if err := d.exec(ctx, "UPDATE publication_jobs SET status = 'running', attempts = attempts + 1, progress = 70 WHERE id = $1", jobID); err != nil {
		return err
	}
	remoteID, err := emdashUpsertContent(ctx, baseURL, secret, collection, slug, data,
		strings.TrimSpace(remotePostID), jobID)
	if err != nil {
		return err
	}
	if remoteID == "" {
		return errors.New("EmDash tidak mengembalikan id item.")
	}
	remoteURL := strings.TrimRight(baseURL, "/") + "/" + collection + "/" + remoteID
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
