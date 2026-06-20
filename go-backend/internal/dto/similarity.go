package dto

// SimilarityRecordResponse is the response for a similarity record.
type SimilarityRecordResponse struct {
	ID               int64    `json:"id"`
	TaskID           int64    `json:"task_id"`
	UploadAID        int64    `json:"upload_a_id"`
	UploadBID        int64    `json:"upload_b_id"`
	HammingDistance  *int     `json:"hamming_distance"`
	CosineSimilarity *float64 `json:"cosine_similarity"`
	State            string   `json:"state"`
	ReviewedBy       *int64   `json:"reviewed_by_id"`
	CreatedAt        string   `json:"created_at"`
	DecidedAt        *string  `json:"decided_at"`
}

// SimilarityDecisionRequest is the request for POST /api/similarity/{id}/decision.
type SimilarityDecisionRequest struct {
	Action string `json:"action"` // "confirmed" or "ignored"
}

// SegmentPairResponse is a similar text segment pair with position offsets.
type SegmentPairResponse struct {
	AStart   int     `json:"a_start"`
	AEnd     int     `json:"a_end"`
	BStart   int     `json:"b_start"`
	BEnd     int     `json:"b_end"`
	SnippetA string  `json:"snippet_a"`
	SnippetB string  `json:"snippet_b"`
	Ratio    float64 `json:"ratio"`
}
