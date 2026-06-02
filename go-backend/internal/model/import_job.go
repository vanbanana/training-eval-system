package model

import "time"

// ImportJob represents a batch user import operation.
type ImportJob struct {
	ID             int64      `json:"id"`
	OperatorID     int64      `json:"operator_id"`
	JobType        string     `json:"job_type"`
	Status         string     `json:"status"` // pending, processing, done, failed
	TotalCount     int        `json:"total_count"`
	SuccessCount   int        `json:"success_count"`
	FailedCount    int        `json:"failed_count"`
	FailedFilePath *string    `json:"failed_file_path"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`

	Records []ImportRecord `json:"records,omitempty"`
}

// ImportRecord represents a single row result in an import job.
type ImportRecord struct {
	ID           int64  `json:"id"`
	JobID        int64  `json:"job_id"`
	RowNumber    int    `json:"row_number"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
}
