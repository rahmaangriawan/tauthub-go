package summary

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// View: ringkasan + penanda kedaluwarsa untuk UI.
type View struct {
	Summary
	Stale bool `json:"stale"`
}

// Load membaca kedua ringkasan sebuah brief. Baris yang belum ada dikembalikan
// sebagai status `idle` supaya klien tidak perlu menebak.
func Load(ctx context.Context, pool *pgxpool.Pool, profileID string) (map[Kind]Summary, error) {
	out := map[Kind]Summary{
		KindEntity: {ProfileID: profileID, Kind: KindEntity, Status: "idle"},
		KindFile:   {ProfileID: profileID, Kind: KindFile, Status: "idle"},
	}
	rows, err := pool.Query(ctx, `
		SELECT kind, status, COALESCE(content, ''), COALESCE(source_chars, 0),
		       COALESCE(source_hash, ''), COALESCE(model, ''), COALESCE(token_input, 0),
		       COALESCE(token_output, 0), COALESCE(error, ''), generated_at, updated_at
		FROM profiler_brief_summaries WHERE profile_id = $1`, profileID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			kindRaw, status, content, hash, model, errMsg string
			sourceChars, tokIn, tokOut                    int
			generatedAt, updatedAt                        *time.Time
		)
		if err := rows.Scan(&kindRaw, &status, &content, &sourceChars, &hash, &model,
			&tokIn, &tokOut, &errMsg, &generatedAt, &updatedAt); err != nil {
			continue
		}
		kind, ok := ParseKind(kindRaw)
		if !ok {
			continue
		}
		item := Summary{
			ProfileID: profileID, Kind: kind, Status: status, Content: content,
			SourceChars: sourceChars, SourceHash: hash, Model: model,
			TokenInput: tokIn, TokenOutput: tokOut, Error: errMsg,
			GeneratedAt: generatedAt,
		}
		if updatedAt != nil {
			item.UpdatedAt = *updatedAt
		}
		out[kind] = item
	}
	return out, nil
}

// LoadList: kedua ringkasan dalam urutan tetap (entitas dulu, lalu file).
// Dipakai polling status: tanpa menghitung ulang hash bahan.
func LoadList(ctx context.Context, pool *pgxpool.Pool, profileID string) ([]Summary, error) {
	items, err := Load(ctx, pool, profileID)
	if err != nil {
		return nil, err
	}
	return []Summary{items[KindEntity], items[KindFile]}, nil
}

// LoadWithStale: Load + bandingkan hash bahan saat ini dengan hash saat
// ringkasan dibuat. Hanya dipakai endpoint UI; writer cukup memakai status.
func LoadWithStale(ctx context.Context, pool *pgxpool.Pool, profileID string) ([]View, error) {
	items, err := Load(ctx, pool, profileID)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, 2)
	for _, kind := range []Kind{KindEntity, KindFile} {
		s := items[kind]
		stale := false
		if s.Status == "ready" && strings.TrimSpace(s.SourceHash) != "" {
			if src, err := BuildSource(ctx, pool, profileID, kind); err == nil && src.Hash != s.SourceHash {
				stale = true
			}
		}
		out = append(out, View{Summary: s, Stale: stale})
	}
	return out, nil
}
