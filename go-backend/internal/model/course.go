package model

import "time"

// Course represents an academic course.
type Course struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Code       string    `json:"code"`
	IsArchived bool      `json:"is_archived"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	Classes []Class `json:"classes,omitempty"`
}

// Class represents a class within a course.
type Class struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	CourseID     int64     `json:"course_id"`
	TeacherID    int64     `json:"teacher_id"`
	StudentCount int       `json:"student_count"`
	IsArchived   bool      `json:"is_archived"`
	CreatedAt    time.Time `json:"created_at"`
}

// ClassMembership represents a student's membership in a class.
type ClassMembership struct {
	ID        int64     `json:"id"`
	ClassID   int64     `json:"class_id"`
	StudentID int64     `json:"student_id"`
	JoinedAt  time.Time `json:"joined_at"`
}
