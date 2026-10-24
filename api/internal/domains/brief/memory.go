package brief

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// memory.go: memori fakta lintas brief. Field terisi dari setiap profile
// sebuah brand diindeks ke brief_memory_facts (satu baris per field, embedding
// lokal 128-dim). Saat chat brief berjalan, pesan user terakhir dipakai sebagai
// query untuk mengingat fakta dari profile LAIN di brand yang sama — pola
// retrieval sama seperti knowledge_chunks: cosine di Go, tanpa pgvector.

// errMemoryPayload: payload job reindex tidak lengkap.
var errMemoryPayload = errors.New("payload brief_memory_reindex tidak lengkap")

const (
	// memoryJobKind: job worker yang mengindeks ulang fakta satu profile.
	memoryJobKind = "brief_memory_reindex"
	// memoryMinScore: ambang cosine minimal supaya fakta dianggap relevan.
	// Embedding lokal bersifat fuzzy-lexical; ambang rendah disengaja.
	memoryMinScore = 0.12
	// memoryAskBoost: fakta yang key-nya sama dengan field yang sedang
	// ditanyakan didorong naik — konteks pertanyaan lebih kuat dari teks.
	memoryAskBoost = 0.2
	// memoryMaxChars: batas blok memori di konteks giliran.
	memoryMaxChars = 1200
	// memoryFactMaxRunes: panjang maksimum teks satu fakta.
	memoryFactMaxRunes = 300
)

// MemoryFact: satu fakta hasil recall, siap dirender ke blok konteks.
type MemoryFact struct {
	ProfileName string `json:"profile_name"`
	FieldKey    string `json:"field_key"`
	Text        string `json:"text"`
}

// RegisterMemoryJobs mendaftarkan processor job reindex memori. Dipanggil
// dari jobwire.Register.
func RegisterMemoryJobs(pool *pgxpool.Pool) {
	jobs.Register(memoryJobKind, func(ctx context.Context, job jobs.JobRow) error {
		profileID, _ := job.Payload["profile_id"].(string)
		if profileID == "" {
			return errMemoryPayload
		}
		d := &Deps{Pool: pool}
		return d.ReindexProfile(ctx, profileID)
	})
}

// QueueMemoryReindex: antre reindex fakta profile (dedupe: satu job pending
// per profile — pola summary/queue.go). Best-effort, tidak menggagalkan
// giliran chat.
func (d *Deps) QueueMemoryReindex(ctx context.Context, profileID string) {
	_, _ = d.Pool.Exec(ctx, `
		INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at)
		SELECT gen_random_uuid()::text, $1, jsonb_build_object('profile_id', $2), 'pending', 0, 2, now() + interval '5 seconds'
		WHERE NOT EXISTS (
			SELECT 1 FROM jobs
			WHERE kind = $1 AND status IN ('pending', 'running') AND payload->>'profile_id' = $2
		)`, memoryJobKind, profileID)
}

// ReindexProfile: sinkronkan brief_memory_facts dengan nilai field profile
// saat ini. Field yang kini kosong dihapus; profile yang dihapus dibersihkan.
func (d *Deps) ReindexProfile(ctx context.Context, profileID string) error {
	var brandID, name string
	err := d.Pool.QueryRow(ctx,
		`SELECT brand_id, name FROM profiler_profiles WHERE id = $1 AND deleted_at IS NULL`,
		profileID).Scan(&brandID, &name)
	if err != nil {
		// Profile sudah tidak ada: bersihkan fakta-faktanya lalu selesai.
		_, _ = d.Pool.Exec(ctx, `DELETE FROM brief_memory_facts WHERE profile_id = $1`, profileID)
		return nil
	}

	fields, _, err := d.loadActiveFieldSet(ctx)
	if err != nil {
		return err
	}
	states, err := d.loadStates(ctx, profileID)
	if err != nil {
		return err
	}
	values, err := d.loadValues(ctx, profileID)
	if err != nil {
		return err
	}

	labels := map[string]string{}
	for _, f := range fields {
		labels[f.Key] = f.Label
	}

	type factRow struct {
		key, text, status string
	}
	rows := []factRow{}
	seen := map[string]bool{}
	for key, raw := range values {
		st := states[key]
		if st.Status != "filled" && st.Status != "confirmed" {
			continue
		}
		text := memoryFactText(labels[key], raw)
		if text == "" {
			continue
		}
		status := st.Status
		if st.Source != "" && st.Status == "confirmed" {
			status = st.Status // confirmed disimpan apa adanya
		}
		rows = append(rows, factRow{key: key, text: text, status: status})
		seen[key] = true
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].key < rows[j].key })

	for _, r := range rows {
		emb, err := json.Marshal(ai.LocalEmbedding(r.text))
		if err != nil {
			continue
		}
		_, _ = d.Pool.Exec(ctx, `
			INSERT INTO brief_memory_facts (brand_id, profile_id, profile_name, field_key, text, embedding, source_status, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, now())
			ON CONFLICT (profile_id, field_key) DO UPDATE SET
			  text = EXCLUDED.text, embedding = EXCLUDED.embedding,
			  source_status = EXCLUDED.source_status, profile_name = EXCLUDED.profile_name,
			  brand_id = EXCLUDED.brand_id, updated_at = now()`,
			brandID, profileID, name, r.key, r.text, string(emb), r.status)
	}
	return nil
}

// memoryFactText: "label: nilai" — array digabung koma, panjang dibatasi.
func memoryFactText(label string, raw any) string {
	parts := []string{}
	switch v := raw.(type) {
	case string:
		parts = append(parts, v)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, s)
			}
		}
	default:
		return ""
	}
	value := strings.Join(parts, ", ")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if label != "" {
		value = label + ": " + value
	}
	return truncateRunes(value, memoryFactMaxRunes)
}

// RecallFacts: cari fakta dari profile LAIN di brand yang sama yang paling
// relevan dengan query (pesan user terakhir). Boost untuk field yang sedang
// ditanyakan. Kembalikan maksimal `limit`, skor >= memoryMinScore.
func (d *Deps) RecallFacts(ctx context.Context, brandID, excludeProfileID, query, askedField string, limit int) []MemoryFact {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil
	}
	rows, err := d.Pool.Query(ctx, `
		SELECT f.profile_name, f.field_key, f.text, f.embedding
		FROM brief_memory_facts f
		JOIN profiler_profiles p ON p.id = f.profile_id AND p.deleted_at IS NULL
		WHERE f.brand_id = $1 AND f.profile_id <> $2`, brandID, excludeProfileID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	qvec := ai.LocalEmbedding(query)
	type scored struct {
		fact  MemoryFact
		score float64
	}
	hits := []scored{}
	for rows.Next() {
		var f MemoryFact
		var embRaw []byte
		if rows.Scan(&f.ProfileName, &f.FieldKey, &f.Text, &embRaw) != nil {
			continue
		}
		var vec []float64
		if json.Unmarshal(embRaw, &vec) != nil || len(vec) != len(qvec) {
			continue
		}
		score := ai.CosineSimilarity(qvec, vec)
		if askedField != "" && f.FieldKey == askedField {
			score += memoryAskBoost
		}
		if score >= memoryMinScore {
			hits = append(hits, scored{fact: f, score: score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]MemoryFact, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.fact)
	}
	return out
}

// memoryBlock: render blok "fakta dari brief lain" untuk konteks giliran.
func memoryBlock(items []MemoryFact) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Fakta dari brief lain brand ini (memori lama)\n")
	b.WriteString("Fakta berikut berasal dari brief LAIN di brand ini — tawarkan sebagai tebakan (suggested_values) dan minta konfirmasi; JANGAN langsung isi field dari memori tanpa persetujuan user. Sebutkan asalnya (\"dari brief <nama>\").\n")
	for _, f := range items {
		name := f.ProfileName
		if name == "" {
			name = "brief lain"
		}
		b.WriteString("- (dari brief " + name + ") " + f.Text + "\n")
	}
	return truncateRunes(b.String(), memoryMaxChars)
}
