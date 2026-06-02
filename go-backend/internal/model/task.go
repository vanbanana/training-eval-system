package model

import "time"

// TrainingTask represents an evaluation task created by a teacher.
type TrainingTask struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	Requirements       string     `json:"requirements"`
	EvaluationCriteria string     `json:"evaluation_criteria"`
	TeacherID          int64      `json:"teacher_id"`
	CourseID           int64      `json:"course_id"`
	Status             string     `json:"status"` // draft, published, closed
	Deadline           *time.Time `json:"deadline"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	Dimensions []Dimension `json:"dimensions,omitempty"`
	ClassIDs   []int64     `json:"class_ids,omitempty"`
}

// Dimension represents an evaluation dimension within a task.
type Dimension struct {
	ID          int64  `json:"id"`
	TaskID      int64  `json:"task_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
	OrderIndex  int    `json:"order_index"`
}
