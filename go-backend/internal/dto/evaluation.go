package dto

// EvaluationResponse is the response for a single evaluation.
type EvaluationResponse struct {
	ID             int64    `json:"id"`
	TaskID         int64    `json:"task_id"`
	StudentID      int64    `json:"student_id"`
	UploadID       int64    `json:"upload_id"`
	Status         string   `json:"status"`
	TotalScore     *float64 `json:"total_score"`
	TeacherComment string   `json:"teacher_comment"`
	OverallComment string   `json:"overall_comment"`
	// AIFailed is true when AI scoring failed and the evaluation was left
	// pending for manual review (status=pending with a recorded failure reason).
	// The frontend uses this to surface a clear failure state instead of an
	// indefinite "AI 评价中" spinner.
	AIFailed  bool                 `json:"ai_failed"`
	CreatedAt string               `json:"created_at"`
	UpdatedAt string               `json:"updated_at"`
	Scores    []DimensionScoreResp `json:"scores"`
}

// DimensionScoreResp is a dimension score in the evaluation response.
// Fields are aliased to match what the frontend expects.
type DimensionScoreResp struct {
	ID            int64    `json:"id"`
	EvaluationID  int64    `json:"evaluation_id"`
	DimensionID   int64    `json:"dimension_id"`
	DimensionName string   `json:"dimension_name"`
	Weight        int      `json:"weight"`
	ObjScore      *float64 `json:"obj_score"`
	SubjScore     *float64 `json:"subj_score"`
	Comment       string   `json:"comment"`
	AIScore       *float64 `json:"ai_score"`
	TeacherScore  *float64 `json:"teacher_score"`
	Rationale     string   `json:"rationale"`
}

// EvaluationHistoryResp is a single history entry.
type EvaluationHistoryResp struct {
	ID          int64  `json:"id"`
	Action      string `json:"action"`
	BeforeValue any    `json:"before_value"`
	AfterValue  any    `json:"after_value"`
	ChangedAt   string `json:"changed_at"`
	OperatorID  *int64 `json:"operator_id"`
}

// BulkActionRequest is the request body for POST /api/evaluations/bulk-action.
type BulkActionRequest struct {
	Action        string  `json:"action"` // "confirm" or "reject"
	EvaluationIDs []int64 `json:"evaluation_ids"`
	Reason        string  `json:"reason,omitempty"`
}

// TriggerResponse is the response for POST /api/evaluations/trigger/{uploadId}.
type TriggerResponse struct {
	EvaluationID int64   `json:"evaluation_id"`
	TotalScore   float64 `json:"total_score"`
}

// UpdateDimensionScoreRequest is the request for PATCH /api/evaluations/{id}/dimensions/{dimId}.
type UpdateDimensionScoreRequest struct {
	SubjScore *float64 `json:"subj_score"`
	Comment   string   `json:"comment"`
}
