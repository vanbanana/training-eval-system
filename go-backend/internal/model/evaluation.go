package model

import "time"

// Evaluation represents an AI/teacher evaluation of a student submission.
type Evaluation struct {
	ID             int64     `json:"id"`
	TaskID         int64     `json:"task_id"`
	StudentID      int64     `json:"student_id"`
	UploadID       int64     `json:"upload_id"`
	Status         string    `json:"status"` // pending, scored, confirmed, rejected
	TotalScore     *float64  `json:"total_score"`
	ObjectiveRatio *float64  `json:"objective_ratio"` // AI objective weight, default 0.6
	TeacherComment string    `json:"teacher_comment"`
	OverallComment string    `json:"overall_comment"` // System-level comment (e.g. failure reason)
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Scores  []DimensionScore    `json:"scores,omitempty"`
	History []EvaluationHistory `json:"history,omitempty"`
}

// DimensionScore holds the score for a single evaluation dimension.
type DimensionScore struct {
	ID           int64    `json:"id"`
	EvaluationID int64    `json:"evaluation_id"`
	DimensionID  int64    `json:"dimension_id"`
	AIScore      *float64 `json:"ai_score"`
	TeacherScore *float64 `json:"teacher_score"`
	Rationale    string   `json:"rationale"`
}

// EvaluationHistory records each modification to an evaluation (audit trail).
type EvaluationHistory struct {
	ID           int64     `json:"id"`
	EvaluationID int64     `json:"evaluation_id"`
	OperatorID   *int64    `json:"operator_id"`
	Action       string    `json:"action"`
	BeforeValue  any       `json:"before_value"` // JSON
	AfterValue   any       `json:"after_value"`  // JSON
	ChangedAt    time.Time `json:"changed_at"`
}
