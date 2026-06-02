package dto

// CreateClassRequest is the request body for POST /api/classes.
type CreateClassRequest struct {
	Name     string `json:"name"`
	CourseID int64  `json:"course_id"`
}

// ClassResponse is the response for a single class.
type ClassResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CourseID     int64  `json:"course_id"`
	TeacherID    int64  `json:"teacher_id"`
	StudentCount int    `json:"student_count"`
	IsArchived   bool   `json:"is_archived"`
	CreatedAt    string `json:"created_at"`
}

// BulkAddStudentsRequest is the request for POST /api/classes/{id}/students/bulk.
type BulkAddStudentsRequest struct {
	StudentIDs []int64 `json:"student_ids"`
}

// StudentInClassResponse is a student entry in the class member list.
type StudentInClassResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	JoinedAt    string `json:"joined_at"`
}
