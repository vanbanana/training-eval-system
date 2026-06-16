package model

import "time"

// Upload represents a student's file submission.
type Upload struct {
	ID          int64     `json:"id"`
	TaskID      int64     `json:"task_id"`
	StudentID   int64     `json:"student_id"`
	Filename    string    `json:"filename"`
	FileType    string    `json:"file_type"`
	FileSize    int64     `json:"file_size"`
	StoragePath string    `json:"storage_path"`
	SHA256      string    `json:"sha256"`
	ParseStatus string    `json:"parse_status"` // pending, parsing, parsed, failed
	Version     int       `json:"version"`
	IsDeleted   bool      `json:"is_deleted"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	ParseResult  *ParseResult  `json:"parse_result,omitempty"`
	VerifyResult *VerifyResult `json:"verify_result,omitempty"`
}

// ParseResult holds the parsed content of an upload.
type ParseResult struct {
	ID                int64     `json:"id"`
	UploadID          int64     `json:"upload_id"`
	StructuredContent any       `json:"structured_content"` // JSON object
	RawText           string    `json:"raw_text"`
	SimHash           *int64    `json:"simhash"`
	Embedding         []float64 `json:"embedding"`
	ErrorMessage      string    `json:"error_message"`
	ParsedAt          time.Time `json:"parsed_at"`
}

// VerifyResult holds the verification result of an upload against task requirements.
type VerifyResult struct {
	ID                int64     `json:"id"`
	UploadID          int64     `json:"upload_id"`
	MatchRate         *float64  `json:"match_rate"`
	Checkpoints       any       `json:"checkpoints"`   // JSON array
	MissingItems      any       `json:"missing_items"` // JSON array
	LogicIssues       any       `json:"logic_issues"`  // JSON array
	OverallConfidence *int      `json:"overall_confidence"`
	ErrorMessage      string    `json:"error_message"`
	VerifiedAt        time.Time `json:"verified_at"`
}
