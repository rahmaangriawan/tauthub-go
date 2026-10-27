package profiler

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

// BuildContextText: konteks Profiler tanpa topik (dipakai saat belum ada topik).
func BuildContextText(ctx context.Context, pool *pgxpool.Pool, profileID string) string {
	return BuildContextTextForTopic(ctx, pool, profileID, "")
}

// BuildContextTextForTopic: susun konteks Profiler yang siap dipakai The Writer.
//
// KONTRAK: fungsi ini 100% deterministik — tidak memanggil LLM, tidak memanggil
// jaringan. Semua pemilihan (field, blok, dokumen, URL) memakai skor lokal
// (embedding lokal + leksikal) dengan batas karakter tetap, sehingga hasilnya
// bisa diulang dan biaya token Writer dapat diprediksi.
//
// Bukan menyerahkan system prompt mentah: field terstruktur, instruksi khusus,
// blok knowledge, dan cuplikan dokumen/URL yang sudah diekstrak dirangkai jadi
// blok berlabel, lalu DIPILIH berdasarkan relevansi ke topik artikel (retrieval)
// supaya tidak menyertakan seluruh dokumen panjang.
func BuildContextTextForTopic(ctx context.Context, pool *pgxpool.Pool, profileID, topic string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" || pool == nil {
		return ""
	}
	var (
		name                 string
		identity, style, seo []byte
		entityCore           []byte
		instructions         *string
	)
	if err := pool.QueryRow(ctx,
		"SELECT name, identity, style, seo, entity_core, instructions FROM profiler_profiles WHERE id = $1 AND deleted_at IS NULL",
		profileID).Scan(&name, &identity, &style, &seo, &entityCore, &instructions); err != nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("[PROFILER KLIEN — acuan brand & gaya penulisan]\n")
	fmt.Fprintf(&b, "Nama Profiler: %s\n", name)

	// Entity Core ditulis paling atas: inilah fakta kanonik yang harus sama di
	// semua artikel, supaya sinyal entitas brand tidak pecah di mata mesin AI.
	writeKV(&b, "ENTITY CORE — fakta kanonik, jangan diparafrase atau diubah angkanya", entityCore,
		[]string{"canonical_name", "canonical_description", "entity_category", "official_facts",
			"allowed_claims", "forbidden_claims", "forbidden_words", "target_questions",
			"boilerplate", "canonical_links", "target_keywords_urls"})

	writeKV(&b, "Identitas Bisnis", identity,
		[]string{"brand_name", "industry", "business_description", "products", "audience",
			"problems_solved", "usp", "values", "vision", "mission", "region", "website", "contact", "key_facts"})
	writeKV(&b, "Gaya Penulisan", style,
		[]string{"tone", "brand_character", "formality", "greeting", "point_of_view", "depth",
			"length_preference", "language", "preferred_terms", "forbidden_terms", "eyd_guideline",
			"opening_preference", "closing_preference"})
	writeKV(&b, "Pedoman Konten & SEO", seo,
		[]string{"article_goal", "search_intent", "primary_keyword", "secondary_keywords",
			"required_topics", "forbidden_topics", "heading_structure", "use_tables", "use_lists",
			"use_faq", "use_examples", "internal_link_rules", "brand_mention_rules",
			"meta_title_preference", "meta_description_preference", "allowed_cta", "ideal_references"})

	if instructions != nil && strings.TrimSpace(*instructions) != "" {
		b.WriteString("\n[INSTRUKSI KHUSUS KLIEN — patuhi]\n")
		b.WriteString(strings.TrimSpace(*instructions))
		b.WriteString("\n")
	}

	// Retrieval: pilih knowledge paling relevan dengan topik.
	var topicVec []float64
	if strings.TrimSpace(topic) != "" {
		topicVec = ai.LocalEmbedding(clip(strings.TrimSpace(topic), 2000))
	}

	blocks := rankKnowledge(ctx, pool,
		"SELECT title, content FROM profiler_blocks WHERE profile_id = $1 ORDER BY sort_order, created_at", profileID,
		func(scan func(...any) error) (string, string, bool) {
			var title string
			var content *string
			if scan(&title, &content) != nil || content == nil || strings.TrimSpace(*content) == "" {
				return "", "", false
			}
			return title, strings.TrimSpace(*content), true
		}, topic, topicVec)
	if len(blocks) > 0 {
		b.WriteString("\n[KNOWLEDGE KLIEN]\n")
		for _, it := range blocks {
			b.WriteString("• " + it.label + ": " + clip(it.text, 700) + "\n")
		}
	}

	docs := rankKnowledge(ctx, pool, `
		SELECT original_filename, extracted_text FROM profiler_documents
		WHERE profile_id = $1 AND deleted_at IS NULL AND parse_status = 'ready' AND extracted_text IS NOT NULL
		ORDER BY created_at DESC LIMIT 40`, profileID,
		func(scan func(...any) error) (string, string, bool) {
			var filename string
			var text *string
			if scan(&filename, &text) != nil || text == nil || strings.TrimSpace(*text) == "" {
				return "", "", false
			}
			return filename, strings.TrimSpace(*text), true
		}, topic, topicVec)
	for _, it := range docs {
		b.WriteString("\n[REFERENSI DOKUMEN: " + it.label + "]\n")
		b.WriteString(clip(it.text, 1500))
		b.WriteString("\n")
	}

	urls := rankKnowledge(ctx, pool, `
		SELECT url, extracted_text FROM profiler_urls
		WHERE profile_id = $1 AND deleted_at IS NULL AND fetch_status = 'ready' AND extracted_text IS NOT NULL
		ORDER BY last_synced_at DESC NULLS LAST LIMIT 40`, profileID,
		func(scan func(...any) error) (string, string, bool) {
			var u string
			var text *string
			if scan(&u, &text) != nil || text == nil || strings.TrimSpace(*text) == "" {
				return "", "", false
			}
			return u, strings.TrimSpace(*text), true
		}, topic, topicVec)
	for _, it := range urls {
		b.WriteString("\n[REFERENSI URL: " + it.label + "]\n")
		b.WriteString(clip(it.text, 1200))
		b.WriteString("\n")
	}

	b.WriteString("\nCatatan: ini referensi dari klien. Standar sistem dan aturan media tetap berlaku; jangan mengarang fakta yang tidak ada di bahan.\n")
	return clip(b.String(), 9000)
}

type rankedItem struct {
	label string
	text  string
	score float64
}

// rankKnowledge: ambil kandidat, hitung skor relevansi ke topik, kembalikan
// top-N. Bila topik kosong, kembalikan yang teratas sesuai urutan query.
func rankKnowledge(
	ctx context.Context, pool *pgxpool.Pool, query, profileID string,
	scan func(scan func(...any) error) (string, string, bool),
	topic string, topicVec []float64,
) []rankedItem {
	rows, err := pool.Query(ctx, query, profileID)
	if err != nil {
		return nil
	}
	var items []rankedItem
	for rows.Next() {
		label, text, ok := scan(rows.Scan)
		if !ok {
			continue
		}
		items = append(items, rankedItem{label: label, text: text})
	}
	rows.Close()
	if strings.TrimSpace(topic) == "" {
		if len(items) > 4 {
			items = items[:4]
		}
		return items
	}
	for i := range items {
		items[i].score = relevanceScore(topic, topicVec, items[i].text)
	}
	sort.SliceStable(items, func(a, c int) bool { return items[a].score > items[c].score })
	const topN = 4
	if len(items) > topN {
		items = items[:topN]
	}
	return items
}

// relevanceScore: gabungan kemiripan embedding lokal (semantik) + tumpang tindih
// leksikal sederhana, untuk memilih potongan knowledge yang paling nyambung.
func relevanceScore(topic string, topicVec []float64, text string) float64 {
	piece := clip(text, 4000)
	sem := 0.0
	if topicVec != nil {
		sem = ai.CosineSimilarity(topicVec, ai.LocalEmbedding(piece))
	}
	return sem*0.7 + lexicalOverlap(topic, piece)*0.3
}

func lexicalOverlap(topic, text string) float64 {
	terms := strings.FieldsFunc(strings.ToLower(topic), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	if len(terms) == 0 {
		return 0
	}
	lower := strings.ToLower(text)
	hit, total := 0, 0
	seen := map[string]bool{}
	for _, t := range terms {
		if len([]rune(t)) < 4 || seen[t] {
			continue
		}
		seen[t] = true
		total++
		if strings.Contains(lower, t) {
			hit++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hit) / float64(total)
}

// writeKV: tulis field jsonb sebagai baris "Label: nilai" (hanya yang terisi).
func writeKV(b *strings.Builder, section string, raw []byte, order []string) {
	if len(raw) == 0 {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return
	}
	var lines []string
	for _, k := range order {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		s := renderValue(v)
		if s == "" {
			continue
		}
		lines = append(lines, "• "+k+": "+s)
	}
	if len(lines) == 0 {
		return
	}
	b.WriteString("\n[" + section + "]\n")
	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n")
}

func renderValue(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "ya"
		}
		return "tidak"
	case []any:
		var parts []string
		for _, it := range t {
			if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, ", ")
	case float64:
		return fmt.Sprintf("%v", t)
	default:
		return ""
	}
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
