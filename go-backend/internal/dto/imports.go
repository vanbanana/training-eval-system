package dto

// ImportResultResponse is the response for POST /api/imports/users or /students.
type ImportResultResponse struct {
	JobID        int64  `json:"job_id"`
	TotalCount   int    `json:"total_count"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
	Status       string `json:"status"`
}
