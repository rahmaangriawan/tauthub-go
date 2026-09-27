// Port lib/knowledge-base/embed.ts — deterministic bag-of-token embedding
// (SHA-256 hash buckets), 128-dim, L2-normalized. HARUS bit-identik dengan
// Node: vektor sama untuk teks sama (embedding lama di DB tetap cocok).
package ai

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"regexp"
	"strings"
)

const EmbeddingDim = 128

// tokenRe = JS /[\w-]+/g pada teks lowercase (\w ASCII = [0-9A-Za-z_]).
var tokenRe = regexp.MustCompile(`[\w-]+`)

// LocalEmbedding: sha256(token) → index = uint16BE(d[0:2]) % 128,
// tanda = parity d[2]; L2-normalize (norm 0 → bagi 1). Parity persis.
func LocalEmbedding(text string) []float64 {
	vector := make([]float64, EmbeddingDim)
	lower := toLowerASCII(text)
	for _, token := range tokenRe.FindAllString(lower, -1) {
		sum := sha256.Sum256([]byte(token))
		index := int(binary.BigEndian.Uint16(sum[0:2])) % EmbeddingDim
		if sum[2]&1 == 0 {
			vector[index] += 1
		} else {
			vector[index] -= 1
		}
	}
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		norm = 1
	}
	out := make([]float64, EmbeddingDim)
	for i, v := range vector {
		out[i] = v / norm
	}
	return out
}

// CosineSimilarity: dot product dua vektor L2-normalized (parity).
func CosineSimilarity(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// LexicalJaccard: port lexicalSimilarity di embed.ts (union-based) —
// dipakai knowledge-base gen-2. CATATAN: _knowledge.ts punya varian lain
// (hits/queryTerms) — port terpisah di package knowledge.
func LexicalJaccard(a, b string) float64 {
	tokens := func(text string) map[string]bool {
		set := map[string]bool{}
		for _, t := range tokenRe.FindAllString(toLowerASCII(text), -1) {
			set[t] = true
		}
		return set
	}
	setA := tokens(a)
	setB := tokens(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	intersection := 0
	for token := range setA {
		if setB[token] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// toLowerASCII: strings.ToLower setara toLowerCase JS untuk tokenisasi
// ASCII [\w-]+ (parity).
func toLowerASCII(s string) string {
	return strings.ToLower(s)
}
