package dto

// CreateCourseRequest is the request body for POST /api/courses.
type CreateCourseRequest struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// CourseResponse is the response for a single course.
type CourseResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	IsArchived   bool   `json:"is_archived"`
	StudentCount int    `json:"student_count,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
