package dto

// UploadResponse is the response for a single upload.
type UploadResponse struct {
	ID          int64  `json:"id"`
	TaskID      int64  `json:"task_id"`
	StudentID   int64  `json:"student_id"`
	Filename    string `json:"filename"`
	FileType    string `json:"file_type"`
	FileSize    int64  `json:"file_size"`
	SHA256      string `json:"sha256"`
	ParseStatus string `json:"parse_status"`
	Version     int    `json:"version"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// VerifyResultResponse is the response for GET /api/uploads/{id}/verify-result.
type VerifyResultResponse struct {
	ID                int64    `json:"id"`
	UploadID          int64    `json:"upload_id"`
	MatchRate         *float64 `json:"match_rate"`
	Checkpoints       any      `json:"checkpoints"`
	MissingItems      any      `json:"missing_items"`
	LogicIssues       any      `json:"logic_issues"`
	OverallConfidence *int     `json:"overall_confidence"`
	VerifiedAt        string   `json:"verified_at"`
}
