package model

import "time"

// EvalTemplate represents a reusable evaluation template.
type EvalTemplate struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"` // private, team, system
	OwnerID     *int64    `json:"owner_id"`
	CourseID    *int64    `json:"course_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Items []TemplateDimension `json:"items,omitempty"`
}

// TemplateDimension represents a dimension within an evaluation template.
type TemplateDimension struct {
	ID          int64  `json:"id"`
	TemplateID  int64  `json:"template_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
	OrderIndex  int    `json:"order_index"`
}
