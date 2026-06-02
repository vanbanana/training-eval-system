package dto

// ParseResultResponse is the response for GET /api/parse/{uploadId}/result.
type ParseResultResponse struct {
	ID                int64  `json:"id"`
	UploadID          int64  `json:"upload_id"`
	StructuredContent any    `json:"structured_content"`
	RawText           string `json:"raw_text"`
	SimHash           *int64 `json:"simhash"`
	ErrorMessage      string `json:"error_message"`
	ParsedAt          string `json:"parsed_at"`
}
