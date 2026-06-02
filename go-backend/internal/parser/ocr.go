package parser

// OCRRequest holds parameters for cloud OCR via multimodal LLM.
type OCRRequest struct {
	ImageBase64 string // base64-encoded image data
	MimeType    string // e.g. "image/png", "image/jpeg"
}

// OCRResult holds the extracted text from OCR.
type OCRResult struct {
	Text  string
	Error string
}

// OCR text extraction is implemented directly on llm.Client.ExtractTextFromImage.
// This file provides domain-level types for reference.
