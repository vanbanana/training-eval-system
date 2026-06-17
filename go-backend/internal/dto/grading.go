package dto

// SubmissionResponse is a single submission in the grading list.
type SubmissionResponse struct {
	UploadID          int64    `json:"upload_id"`
	StudentID         int64    `json:"student_id"`
	StudentName       string   `json:"student_name"`
	Filename          string   `json:"filename"`
	ParseStatus       string   `json:"parse_status"`
	EvaluationID      *int64   `json:"evaluation_id"`
	EvalStatus        *string  `json:"eval_status"`
	TotalScore        *float64 `json:"total_score"`
	SubmittedAt       string   `json:"submitted_at"`
	ScoringInProgress bool     `json:"scoring_in_progress"`
}

// TaskSummaryResponse is the response for GET /api/grading/tasks/{id}/summary.
type TaskSummaryResponse struct {
	TaskID             int64   `json:"task_id"`
	TotalStudents      int     `json:"total_students"`
	TotalUploads       int     `json:"total_uploads"`
	SubmittedCount     int     `json:"submitted_count"`
	ParsedCount        int     `json:"parsed_count"`
	ScoredCount        int     `json:"scored_count"`
	ConfirmedCount     int     `json:"confirmed_count"`
	RejectedCount      int     `json:"rejected_count"`
	SimilarityWarnings int     `json:"similarity_warnings"`
	ProgressPercent    float64 `json:"progress_percent"`
	AverageScore       float64 `json:"average_score"`
	HighestScore       float64 `json:"highest_score"`
	LowestScore        float64 `json:"lowest_score"`
}

// ConfirmRequest is the request body for POST /api/grading/evaluations/{id}/confirm.
type ConfirmRequest struct {
	TeacherComment string            `json:"teacher_comment"`
	ScoreOverrides map[int64]float64 `json:"score_overrides"`
}

// RejectRequest is the request body for POST /api/grading/evaluations/{id}/reject.
type RejectRequest struct {
	Reason string `json:"reason"`
}

// AutoScoreItem is a single result in the auto-score response.
type AutoScoreItem struct {
	UploadID int64  `json:"upload_id"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

// AutoScoreResponse is the response for POST /api/grading/tasks/{id}/auto-score.
type AutoScoreResponse struct {
	TaskID    int64           `json:"task_id"`
	Requested int             `json:"requested"`
	Queued    int             `json:"queued"`
	Skipped   int             `json:"skipped"`
	Failed    int             `json:"failed"`
	Items     []AutoScoreItem `json:"items"`
}
