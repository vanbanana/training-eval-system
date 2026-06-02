package dto

// CreateTaskRequest is the request body for POST /api/tasks.
type CreateTaskRequest struct {
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	Requirements       string  `json:"requirements"`
	EvaluationCriteria string  `json:"evaluation_criteria"`
	CourseID           int64   `json:"course_id"`
	ClassIDs           []int64 `json:"class_ids"`
	Deadline           *string `json:"deadline"`
	Status             string  `json:"status,omitempty"`
}

// UpdateTaskRequest is the request body for PATCH /api/tasks/{id}.
type UpdateTaskRequest struct {
	Name               *string `json:"name"`
	Description        *string `json:"description"`
	Requirements       *string `json:"requirements"`
	EvaluationCriteria *string `json:"evaluation_criteria"`
	CourseID           *int64  `json:"course_id"`
	Deadline           *string `json:"deadline"`
}

// DimensionRequest is a single dimension in PUT /api/tasks/{id}/dimensions.
type DimensionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
	OrderIndex  int    `json:"order_index"`
}

// ReplaceDimensionsRequest is the request body for PUT /api/tasks/{id}/dimensions.
type ReplaceDimensionsRequest struct {
	Dimensions []DimensionRequest `json:"dimensions"`
}

// DimensionResponse is a dimension in the task response.
type DimensionResponse struct {
	ID          int64  `json:"id"`
	TaskID      int64  `json:"task_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
	OrderIndex  int    `json:"order_index"`
}

// TaskResponse is the response for a single task.
type TaskResponse struct {
	ID                 int64               `json:"id"`
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	Requirements       string              `json:"requirements"`
	EvaluationCriteria string              `json:"evaluation_criteria"`
	TeacherID          int64               `json:"teacher_id"`
	CourseID           int64               `json:"course_id"`
	Status             string              `json:"status"`
	Deadline           *string             `json:"deadline"`
	CreatedAt          string              `json:"created_at"`
	UpdatedAt          string              `json:"updated_at"`
	Dimensions         []DimensionResponse `json:"dimensions"`
	ClassIDs           []int64             `json:"class_ids"`
}
