package knowledge

import (
	"strings"
	"testing"
)

// lexicalHits berjalan per baris chunk di RetrieveChunks (≤500 baris per query),
// jadi biaya per panggilannya dikalikan jumlah kandidat chunk.

func BenchmarkLexicalHits(b *testing.B) {
	query := "epoxy lantai industri pelapis beton tahan kimia"
	content := strings.Repeat("Epoxy lantai adalah sistem pelapis dua komponen untuk permukaan beton industri. ", 40)
	b.ReportAllocs()
	b.SetBytes(int64(len(content)))
	var sink float64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = lexicalHits(query, content)
	}
	_ = sink
}

// BenchmarkLexicalScoreChunks: pola nyata RetrieveChunks — satu query, banyak
// chunk. Mengukur beda "term query dihitung per baris" (lama) vs "sekali" (baru).
func BenchmarkLexicalScoreChunks(b *testing.B) {
	query := "epoxy lantai industri pelapis beton tahan kimia"
	body := "Epoxy lantai adalah sistem pelapis dua komponen untuk permukaan beton industri. "
	chunks := make([]string, 500)
	for i := range chunks {
		chunks[i] = strings.Repeat(body, 8)
	}
	var sink float64

	b.Run("term per baris", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, c := range chunks {
				sink = lexicalHits(query, c)
			}
		}
	})
	b.Run("term sekali", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			terms := lexicalTerms(query)
			for _, c := range chunks {
				sink = lexicalHitsFromTerms(terms, c)
			}
		}
	})
	_ = sink
}
