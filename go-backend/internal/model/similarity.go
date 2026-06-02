package model

import "time"

// SimilarityRecord represents a detected similarity pair between two uploads.
type SimilarityRecord struct {
	ID               int64      `json:"id"`
	TaskID           int64      `json:"task_id"`
	UploadAID        int64      `json:"upload_a_id"`
	UploadBID        int64      `json:"upload_b_id"`
	HammingDistance  int        `json:"hamming_distance"`
	CosineSimilarity *float64   `json:"cosine_similarity"`
	State            string     `json:"state"` // suspect, confirmed, ignored
	ReviewedBy       *int64     `json:"reviewed_by"`
	CreatedAt        time.Time  `json:"created_at"`
	DecidedAt        *time.Time `json:"decided_at"`
}
