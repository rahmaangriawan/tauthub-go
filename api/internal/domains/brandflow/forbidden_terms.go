package brandflow

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pemeriksaan deterministik kata terlarang dari brief. Gatekeeper dan Writer
// sudah membaca ringkasan brief lewat model, tetapi model bisa melewatkan
// satu kata. Pemeriksaan ini mencocokkan teks artikel dengan daftar mentah
// forbidden_words dan forbidden_terms milik brief yang dipakai artikel itu
// (taut_campaign_media.proper_brief_id). forbidden_claims sengaja tidak
// dipindai: isinya aturan berbentuk kalimat, bukan kata yang bisa dicocokkan
// harfiah, jadi tetap dinilai model.

var forbiddenSplitRe = regexp.MustCompile(`[\r\n;]+`)

// BriefForbiddenTerms membaca daftar kata terlarang brief untuk satu item
// kampanye. Item tanpa brief (gaya Sederhana Aja) mengembalikan daftar kosong.
func BriefForbiddenTerms(ctx context.Context, pool *pgxpool.Pool, itemID string) []string {
	if itemID == "" || pool == nil {
		return nil
	}
	var entity, identity, style, seo []byte
	err := pool.QueryRow(ctx, `
		SELECT p.entity_core, p.identity, p.style, p.seo
		FROM taut_campaign_media tcm
		JOIN profiler_profiles p ON p.id = tcm.proper_brief_id AND p.deleted_at IS NULL
		WHERE tcm.id = $1`, itemID).Scan(&entity, &identity, &style, &seo)
	if err != nil {
		return nil
	}
	merged := map[string]any{}
	for _, raw := range [][]byte{entity, identity, style, seo} {
		var m map[string]any
		if len(raw) > 0 && json.Unmarshal(raw, &m) == nil {
			for k, v := range m {
				merged[k] = v
			}
		}
	}
	return splitForbiddenTerms(merged["forbidden_words"], merged["forbidden_terms"])
}

// splitForbiddenTerms menerima nilai berbentuk daftar atau teks (dipisah baris
// atau titik koma) dan mengembalikan istilah unik tanpa duplikat.
func splitForbiddenTerms(values ...any) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(s string) {
		s = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(s), "-•*"))
		if len([]rune(s)) < 2 {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	for _, v := range values {
		switch t := v.(type) {
		case string:
			for _, part := range forbiddenSplitRe.Split(t, -1) {
				add(part)
			}
		case []any:
			for _, item := range t {
				if s, ok := item.(string); ok {
					for _, part := range forbiddenSplitRe.Split(s, -1) {
						add(part)
					}
				}
			}
		}
	}
	return out
}

// FindForbiddenHits mengembalikan istilah terlarang yang muncul di teks.
// Kata tunggal dicocokkan sebagai kata utuh (tanpa melompat ke dalam kata lain),
// frasa dicocokkan sebagai rangkaian harfiah. Huruf besar-kecil diabaikan.
func FindForbiddenHits(text string, terms []string) []string {
	if strings.TrimSpace(text) == "" || len(terms) == 0 {
		return nil
	}
	hay := strings.ToLower(text)
	hits := []string{}
	for _, term := range terms {
		re, err := regexp.Compile(`(?i)(^|[^\p{L}\p{N}])` + regexp.QuoteMeta(strings.ToLower(term)) + `($|[^\p{L}\p{N}])`)
		if err != nil {
			continue
		}
		if re.MatchString(hay) {
			hits = append(hits, term)
		}
	}
	return hits
}
