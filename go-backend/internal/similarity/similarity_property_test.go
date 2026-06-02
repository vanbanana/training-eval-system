package similarity

import (
	"math"
	"testing"

	"pgregory.net/rapid"
)

// Property 25: SimHash similarity property
func TestProperty_SimHashSameText(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		text := rapid.StringOfN(rapid.Rune(), 10, 200, -1).Draw(t, "text")

		h1 := SimHash(text)
		h2 := SimHash(text)

		dist := HammingDistance(h1, h2)
		if dist != 0 {
			t.Fatalf("same text should have hamming distance 0, got %d", dist)
		}
	})
}

// Property 26: Cosine similarity bounds
func TestProperty_CosineSimilarityBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		dim := rapid.IntRange(1, 100).Draw(t, "dim")
		a := make([]float64, dim)
		b := make([]float64, dim)

		// Generate non-zero vectors with positive values
		for i := range a {
			a[i] = rapid.Float64Range(0.01, 10.0).Draw(t, "a")
			b[i] = rapid.Float64Range(0.01, 10.0).Draw(t, "b")
		}

		sim := CosineSimilarity(a, b)

		// For non-negative vectors, cosine similarity should be in [0, 1]
		if sim < -0.0001 || sim > 1.0001 {
			t.Fatalf("cosine similarity out of bounds [0,1]: got %f", sim)
		}

		// Self-similarity should be 1.0
		selfSim := CosineSimilarity(a, a)
		if math.Abs(selfSim-1.0) > 1e-10 {
			t.Fatalf("self-similarity should be 1.0, got %f", selfSim)
		}
	})
}

// Property 27: Similarity scope isolation (engine only compares within same task)
func TestProperty_SimilarityScopeIsolation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		engine := NewEngine(3, 0.85)

		target := Fingerprint{
			UploadID: 1,
			SimHash:  SimHash("hello world test document"),
		}

		// Candidates from same "task" (simulated by being in the candidates list)
		candidates := []Fingerprint{
			{UploadID: 2, SimHash: SimHash("hello world test document")}, // same text
			{UploadID: 3, SimHash: SimHash("completely different content xyz")},
		}

		results := engine.Detect(target, candidates)

		// Should not compare target with itself
		for _, r := range results {
			if r.UploadAID == target.UploadID && r.UploadBID == target.UploadID {
				t.Fatal("engine should not compare target with itself")
			}
		}

		// upload_a_id should always be < upload_b_id
		for _, r := range results {
			if r.UploadAID >= r.UploadBID {
				t.Fatalf("upload_a_id (%d) should be < upload_b_id (%d)", r.UploadAID, r.UploadBID)
			}
		}
	})
}
