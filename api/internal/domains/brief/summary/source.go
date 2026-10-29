package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BuildSource menyusun bahan mentah untuk satu kind. Deterministik (urutan
// tetap) supaya hash hanya berubah saat isi brief benar-benar berubah.
func BuildSource(ctx context.Context, pool *pgxpool.Pool, profileID string, kind Kind) (Source, error) {
	if kind == KindFile {
		return buildFileSource(ctx, pool, profileID)
	}
	return buildEntitySource(ctx, pool, profileID)
}

func buildEntitySource(ctx context.Context, pool *pgxpool.Pool, profileID string) (Source, error) {
	var (
		name                             *string
		entityCore, identity, style, seo []byte
		instructions                     *string
	)
	err := pool.QueryRow(ctx, `
		SELECT name,
		       COALESCE(entity_core, '{}'::jsonb),
		       COALESCE(identity, '{}'::jsonb),
		       COALESCE(style, '{}'::jsonb),
		       COALESCE(seo, '{}'::jsonb),
		       instructions
		FROM profiler_profiles
		WHERE id = $1 AND deleted_at IS NULL`, profileID).
		Scan(&name, &entityCore, &identity, &style, &seo, &instructions)
	if err != nil {
		return newSource(""), nil
	}

	var b strings.Builder
	if v := strings.TrimSpace(deref(name)); v != "" {
		b.WriteString("[NAMA BRIEF]\n" + v + "\n\n")
	}
	writeJSONSection(&b, "ENTITY CORE", entityCore, entityCoreCap)
	writeJSONSection(&b, "IDENTITAS", identity, identityCap)
	writeJSONSection(&b, "GAYA", style, styleCap)
	writeJSONSection(&b, "SEO", seo, seoCap)
	if v := strings.TrimSpace(deref(instructions)); v != "" {
		b.WriteString("[INSTRUKSI KHUSUS KLIEN]\n" + clip(v, instructionsCap) + "\n\n")
	}

	rows, err := pool.Query(ctx, `
		SELECT title, COALESCE(content, '') FROM profiler_blocks
		WHERE profile_id = $1 ORDER BY sort_order, created_at, id`, profileID)
	if err == nil {
		defer rows.Close()
		var blocks strings.Builder
		for rows.Next() {
			var title, content string
			if rows.Scan(&title, &content) != nil {
				continue
			}
			if strings.TrimSpace(content) == "" {
				continue
			}
			blocks.WriteString("- " + strings.TrimSpace(title) + ": " + clip(strings.TrimSpace(content), knowledgeBlockCap) + "\n")
		}
		if blocks.Len() > 0 {
			b.WriteString("[KNOWLEDGE BLOCKS]\n" + clip(blocks.String(), knowledgeBlocksCap) + "\n")
		}
	}

	return newSource(clip(b.String(), entitySourceCap)), nil
}

func buildFileSource(ctx context.Context, pool *pgxpool.Pool, profileID string) (Source, error) {
	var b strings.Builder

	// TIMELINE ditaruh PALING ATAS: kabar terbaru (promo, kunjungan, peluncuran)
	// adalah bahan paling unik, jadi tidak boleh terpotong oleh cap ukuran di
	// akhir. Tanggal kejadian disertakan supaya Writer tahu mana yang segar.
	//
	// Tiap seksi punya builder + jatah sendiri. Dulu seluruh teks dipotong di
	// 24000 sehingga seksi yang ditulis belakangan (gambar) selalu kalah.
	var timeline strings.Builder
	tlRows, err := pool.Query(ctx, `
		SELECT body, hashtags, COALESCE(occurred_on::text, created_at::date::text)
		FROM profiler_timeline_posts
		WHERE profile_id = $1 AND deleted_at IS NULL
		ORDER BY COALESCE(occurred_on, created_at::date) DESC, created_at DESC
		LIMIT $2`, profileID, maxTimeline)
	if err == nil {
		var lines []string
		for tlRows.Next() {
			var body, when string
			var tags []string
			if tlRows.Scan(&body, &tags, &when) != nil {
				continue
			}
			body = strings.TrimSpace(body)
			if body == "" {
				continue
			}
			line := "[" + when + "] " + clip(body, timelineTextCap)
			if len(tags) > 0 {
				line += " #" + strings.Join(tags, " #")
			}
			lines = append(lines, line)
		}
		tlRows.Close()
		if len(lines) > 0 {
			timeline.WriteString("[TIMELINE TERBARU (kabar terbaru brand — utamakan sebagai sudut unik)]\n")
			timeline.WriteString(strings.Join(lines, "\n"))
		}
	}
	appendBlock(&b, timeline.String(), fileTimelineCap)

	var docs strings.Builder
	docRows, err := pool.Query(ctx, `
		SELECT original_filename, COALESCE(extracted_text, '')
		FROM profiler_documents
		WHERE profile_id = $1 AND deleted_at IS NULL AND parse_status = 'ready'
		ORDER BY created_at DESC, id DESC
		LIMIT $2`, profileID, maxDocs)
	if err == nil {
		defer docRows.Close()
		for docRows.Next() {
			var filename, text string
			if docRows.Scan(&filename, &text) != nil {
				continue
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			docs.WriteString("[DOKUMEN: " + strings.TrimSpace(filename) + "]\n")
			docs.WriteString(clip(strings.TrimSpace(text), docTextCap) + "\n\n")
		}
	}
	appendBlock(&b, docs.String(), fileDocsCap)

	var urls strings.Builder
	urlRows, err := pool.Query(ctx, `
		SELECT url, COALESCE(extracted_text, '')
		FROM profiler_urls
		WHERE profile_id = $1 AND deleted_at IS NULL AND fetch_status = 'ready'
		ORDER BY last_synced_at DESC NULLS LAST, created_at DESC
		LIMIT $2`, profileID, maxURLs)
	if err == nil {
		defer urlRows.Close()
		for urlRows.Next() {
			var url, text string
			if urlRows.Scan(&url, &text) != nil {
				continue
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			urls.WriteString("[TAUTAN: " + strings.TrimSpace(url) + "]\n")
			urls.WriteString(clip(strings.TrimSpace(text), urlTextCap) + "\n\n")
		}
	}
	appendBlock(&b, urls.String(), fileURLsCap)

	var images strings.Builder
	imgRows, err := pool.Query(ctx, `
		SELECT COALESCE(file_name, ''), COALESCE(note, '')
		FROM profiler_images
		WHERE profile_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 20`, profileID)
	if err == nil {
		defer imgRows.Close()
		var lines []string
		for imgRows.Next() {
			var filename, note string
			if imgRows.Scan(&filename, &note) != nil {
				continue
			}
			line := strings.TrimSpace(filename)
			if n := strings.TrimSpace(note); n != "" {
				line += " — " + clip(n, 300)
			}
			if line != "" {
				lines = append(lines, "- "+line)
			}
		}
		if len(lines) > 0 {
			images.WriteString("[GAMBAR (metadata)]\n" + strings.Join(lines, "\n"))
		}
	}
	appendBlock(&b, images.String(), fileImagesCap)

	return newSource(clip(b.String(), fileSourceCap)), nil
}

// appendBlock: tempel satu seksi bahan yang sudah dipotong jatahnya. Pemisah
// satu baris kosong menjaga format antar seksi walau isinya terpotong.
func appendBlock(b *strings.Builder, body string, max int) {
	if strings.TrimSpace(body) == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(strings.TrimRight(clip(body, max), "\n"))
	b.WriteString("\n")
}

// writeJSONSection: render objek JSON menjadi baris "label: nilai" dengan
// urutan kunci tetap (kunci umum dulu, sisanya alfabetis). max membatasi isi
// seksi ini saja, bukan seluruh bahan, supaya seksi lain tetap kebagian.
func writeJSONSection(b *strings.Builder, title string, raw []byte, max int) {
	var obj map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &obj) != nil || len(obj) == 0 {
		return
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	rank := func(k string) int {
		for i, known := range sectionKeyOrder {
			if known == k {
				return i
			}
		}
		return len(sectionKeyOrder)
	}
	sort.Slice(keys, func(i, j int) bool {
		ri, rj := rank(keys[i]), rank(keys[j])
		if ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	var lines []string
	for _, k := range keys {
		if line := valueLine(k, obj[k]); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return
	}
	b.WriteString("[" + title + "]\n" + clip(strings.Join(lines, "\n"), max) + "\n\n")
}

var sectionKeyOrder = []string{
	"canonical_name", "entity_category", "canonical_description", "official_facts",
	"allowed_claims", "forbidden_claims", "forbidden_words", "forbidden_terms",
	"usp", "region", "website", "target_questions", "boilerplate", "canonical_links",
	"tone", "formality", "point_of_view", "language", "greeting",
}

func valueLine(key string, v any) string {
	value := ""
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		value = strings.TrimSpace(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		value = strings.Join(parts, "; ")
	default:
		raw, err := json.Marshal(v)
		if err != nil || string(raw) == "null" {
			return ""
		}
		value = string(raw)
	}
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", strings.ReplaceAll(key, "_", " "), value)
}
