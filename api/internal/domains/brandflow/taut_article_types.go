package brandflow

import (
	"context"
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Jenis artikel Taut (taut_article_types): daftar untuk klien + loader prompt
// berversi yang dipakai The Writer saat klien memilih genre di modal Pengelola
// Naskah. Prompt tiap genre adalah system prompt pengganti untuk agent Writer.

// loadTautArticleTypePrompt: prompt system AKTIF untuk satu kode jenis artikel.
// Mengembalikan ("", false) bila kode kosong, tidak ditemukan, atau promptnya
// kosong — Editorial sengaja dikosongkan agar Writer memakai prompt agent
// standar (perilaku lama), jadi pemilihan "Editorial" tidak mengubah gaya.
func (d *Deps) loadTautArticleTypePrompt(ctx context.Context, code string) (string, bool) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false
	}
	row, err := d.queryRow(ctx,
		`SELECT v.prompt FROM taut_article_type_prompt_versions v
		 JOIN taut_article_types t ON t.id = v.article_type_id
		 WHERE t.code = $1 AND v.status = 'active' LIMIT 1`, code)
	if err != nil || row == nil {
		return "", false
	}
	prompt := strings.TrimSpace(row.strOr("prompt", ""))
	if prompt == "" {
		return "", false
	}
	return prompt, true
}

// TautArticleTypesList: GET /v1/brands/{brandId}/taut/article-types
// Daftar jenis artikel AKTIF untuk toggle di modal Pengelola Naskah (tab
// "Generate dengan AI"). has_prompt=false berarti belum ada prompt aktif.
func (d *Deps) TautArticleTypesList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if _, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer"); err != nil {
		d.writeErr(w, err)
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT t.code, t.name, t.description,
		        (v.id IS NOT NULL AND COALESCE(v.prompt, '') <> '') AS has_prompt
		 FROM taut_article_types t
		 LEFT JOIN taut_article_type_prompt_versions v
		   ON v.article_type_id = t.id AND v.status = 'active'
		 WHERE t.active
		 ORDER BY t.sort_order, t.name`)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"code":        row.strOr("code", ""),
			"name":        row.strOr("name", ""),
			"description": row.raw("description"),
			"has_prompt":  row.boolOr("has_prompt", false),
		})
	}
	writeNoStore(w, http.StatusOK, map[string]any{"items": items})
}
