// Package similarity implements SimHash and cosine similarity algorithms.
package similarity

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// SimHash computes a 64-bit SimHash fingerprint for the given text.
func SimHash(text string) uint64 {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return 0
	}

	var v [64]int
	for _, token := range tokens {
		h := hashToken(token)
		for i := 0; i < 64; i++ {
			if (h>>uint(i))&1 == 1 {
				v[i]++
			} else {
				v[i]--
			}
		}
	}

	var fingerprint uint64
	for i := 0; i < 64; i++ {
		if v[i] > 0 {
			fingerprint |= 1 << uint(i)
		}
	}
	return fingerprint
}

// HammingDistance computes the number of differing bits between two fingerprints.
func HammingDistance(a, b uint64) int {
	xor := a ^ b
	count := 0
	for xor != 0 {
		count++
		xor &= xor - 1 // clear lowest set bit
	}
	return count
}

// CosineSimilarity computes the cosine similarity between two float64 vectors.
// Returns a value in [0, 1] for non-negative vectors, [-1, 1] in general.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// tokenize splits text into word tokens (Chinese characters are individual tokens).
func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			// Flush current word
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			// Each Chinese character is a token
			tokens = append(tokens, string(r))
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func hashToken(token string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(token))
	return h.Sum64()
}

// DefaultEmbeddingDim is the vector dimension for local embedding computation.
const DefaultEmbeddingDim = 64

// ComputeLocalEmbedding generates a lightweight embedding vector from text.
// Uses character-bigram hashing + L2 normalization, avoiding an API call.
func ComputeLocalEmbedding(text string) []float64 {
	if len(text) == 0 {
		return make([]float64, DefaultEmbeddingDim)
	}

	vec := make([]float64, DefaultEmbeddingDim)
	runes := []rune(text)
	if len(runes) < 2 {
		idx := int(runes[0]) % DefaultEmbeddingDim
		if idx < 0 {
			idx = -idx
		}
		vec[idx] = 1.0
		return vec
	}

	for i := 0; i < len(runes)-1; i++ {
		hash := int(runes[i])*31 + int(runes[i+1])
		idx := hash % DefaultEmbeddingDim
		if idx < 0 {
			idx = -idx
		}
		vec[idx] += 1.0
	}

	// L2 normalize
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec
}
