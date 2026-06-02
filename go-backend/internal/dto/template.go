package dto

// CreateTemplateRequest is the request for POST /api/templates.
type CreateTemplateRequest struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Visibility  string                    `json:"visibility,omitempty"`
	CourseID    *int64                    `json:"course_id,omitempty"`
	Items       []TemplateDimensionReq    `json:"items,omitempty"`
}

// TemplateDimensionReq is a dimension in the template request.
type TemplateDimensionReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
	OrderIndex  int    `json:"order_index"`
}

// CreateFromTaskRequest is the request for POST /api/templates/from-task.
type CreateFromTaskRequest struct {
	TaskID int64  `json:"task_id"`
	Name   string `json:"name"`
}

// TemplateResponse is the response for a single template.
type TemplateResponse struct {
	ID          int64                    `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Visibility  string                   `json:"visibility"`
	OwnerID     *int64                   `json:"owner_id"`
	CourseID    *int64                   `json:"course_id"`
	CreatedAt   string                   `json:"created_at"`
	UpdatedAt   string                   `json:"updated_at"`
	Items       []TemplateDimensionResp  `json:"items"`
}

// TemplateDimensionResp is a dimension in the template response.
type TemplateDimensionResp struct {
	ID          int64  `json:"id"`
	TemplateID  int64  `json:"template_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
	OrderIndex  int    `json:"order_index"`
}
