// Package dto defines request/response data transfer objects.
package dto

// PaginatedResponse is the standard paginated response envelope.
type PaginatedResponse[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Detail string `json:"detail"`
}

// SuccessResponse is a generic success message.
type SuccessResponse struct {
	Message string `json:"message"`
}

// IDResponse returns a created resource ID.
type IDResponse struct {
	ID int64 `json:"id"`
}
