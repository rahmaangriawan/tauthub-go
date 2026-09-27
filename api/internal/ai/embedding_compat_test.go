// Test kompat embedding fase 3: fixture dibuat oleh embed.ts ASLI (Node/bun)
// dari repo lama. Vektor HARUS bit-identik — embedding lama di DB
// knowledge_chunks tetap cocok tanpa re-index.
package ai

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type embedFixtures struct {
	Dim        int       `json:"dim"`
	Samples    []string  `json:"samples"`
	Embeddings [][]float64 `json:"embeddings"`
	Cosines    []struct {
		A     int     `json:"a"`
		B     int     `json:"b"`
		Value float64 `json:"value"`
	} `json:"cosines"`
	Jaccards []struct {
		A     string  `json:"a"`
		B     string  `json:"b"`
		Value float64 `json:"value"`
	} `json:"jaccards"`
}

func loadEmbedFixtures(t *testing.T) embedFixtures {
	t.Helper()
	raw, err := os.ReadFile("../../compat/embed-fixtures.json")
	if err != nil {
		t.Skipf("fixtures belum digenerate: %v", err)
	}
	var f embedFixtures
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}
	return f
}

func TestLocalEmbeddingBitIdentical(t *testing.T) {
	f := loadEmbedFixtures(t)
	if f.Dim != EmbeddingDim {
		t.Fatalf("dim fixture %d != %d", f.Dim, EmbeddingDim)
	}
	for i, sample := range f.Samples {
		got := LocalEmbedding(sample)
		want := f.Embeddings[i]
		if len(got) != len(want) {
			t.Fatalf("sample %d: panjang vektor %d != %d", i, len(got), len(want))
		}
		for j := range got {
			// IEEE-754 float64 di kedua sisi + urutan operasi sama → harus
			// identik hingga epsilon kecil (JSON round-trip aman untuk float64).
			if math.Abs(got[j]-want[j]) > 1e-12 {
				t.Fatalf("sample %d dim %d: got %v want %v (delta %v)", i, j, got[j], want[j], math.Abs(got[j]-want[j]))
			}
		}
		// L2 norm = 1 (atau 0-vektor dinormalisasi 1)
		norm := 0.0
		for _, v := range got {
			norm += v * v
		}
		norm = math.Sqrt(norm)
		if math.Abs(norm-1) > 1e-9 && norm > 0 {
			t.Fatalf("sample %d: norm = %v, harus ~1", i, norm)
		}
	}
}

func TestCosineSimilarityBitIdentical(t *testing.T) {
	f := loadEmbedFixtures(t)
	embeddings := make([][]float64, len(f.Samples))
	for i, s := range f.Samples {
		embeddings[i] = LocalEmbedding(s)
	}
	for _, pair := range f.Cosines {
		got := CosineSimilarity(embeddings[pair.A], embeddings[pair.B])
		if math.Abs(got-pair.Value) > 1e-12 {
			t.Fatalf("cosine(%d,%d): got %v want %v", pair.A, pair.B, got, pair.Value)
		}
	}
}

func TestLexicalJaccardBitIdentical(t *testing.T) {
	f := loadEmbedFixtures(t)
	for _, j := range f.Jaccards {
		got := LexicalJaccard(j.A, j.B)
		if math.Abs(got-j.Value) > 1e-12 {
			t.Fatalf("jaccard(%q,%q): got %v want %v", j.A, j.B, got, j.Value)
		}
	}
}
