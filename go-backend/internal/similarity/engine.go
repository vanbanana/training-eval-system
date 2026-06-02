package similarity

// DetectResult holds the result of a similarity detection.
type DetectResult struct {
	UploadAID        int64
	UploadBID        int64
	HammingDistance  int
	CosineSimilarity float64
	IsSuspect        bool
}

// Engine performs two-phase similarity detection.
type Engine struct {
	HammingThreshold int     // max hamming distance to consider similar (default 3)
	CosineThreshold  float64 // min cosine similarity to flag (default 0.85)
}

// NewEngine creates a similarity engine with configurable thresholds.
func NewEngine(hammingThreshold int, cosineThreshold float64) *Engine {
	return &Engine{
		HammingThreshold: hammingThreshold,
		CosineThreshold:  cosineThreshold,
	}
}

// Fingerprint holds a document's computed fingerprints.
type Fingerprint struct {
	UploadID  int64
	SimHash   uint64
	Embedding []float64
}

// Detect performs two-phase detection:
// Phase 1: SimHash coarse filter (hamming distance)
// Phase 2: Cosine similarity fine ranking (only for phase 1 candidates)
func (e *Engine) Detect(target Fingerprint, candidates []Fingerprint) []DetectResult {
	var results []DetectResult

	for _, c := range candidates {
		if c.UploadID == target.UploadID {
			continue
		}

		// Phase 1: SimHash coarse filter
		hd := HammingDistance(target.SimHash, c.SimHash)
		if hd > e.HammingThreshold {
			continue
		}

		// Phase 2: Cosine similarity (if embeddings available)
		var cosine float64
		if len(target.Embedding) > 0 && len(c.Embedding) > 0 {
			cosine = CosineSimilarity(target.Embedding, c.Embedding)
		}

		isSuspect := cosine >= e.CosineThreshold || (len(target.Embedding) == 0 && hd <= e.HammingThreshold)

		// Ensure upload_a < upload_b for uniqueness
		aID, bID := target.UploadID, c.UploadID
		if aID > bID {
			aID, bID = bID, aID
		}

		results = append(results, DetectResult{
			UploadAID:        aID,
			UploadBID:        bID,
			HammingDistance:  hd,
			CosineSimilarity: cosine,
			IsSuspect:        isSuspect,
		})
	}

	return results
}
