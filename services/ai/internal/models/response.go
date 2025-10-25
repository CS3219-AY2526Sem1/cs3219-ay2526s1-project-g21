package models

// code explanation requests
type ExplainResponse struct {
	Explanation string              `json:"explanation"`
	RequestID   string              `json:"request_id"`
	Metadata    ExplanationMetadata `json:"metadata"`
}

// additional information about the explanation
type ExplanationMetadata struct {
	ProcessingTime int    `json:"processing_time_ms"`
	DetailLevel    string `json:"detail_level"`
}

// uniform error responses
type ErrorResponse struct {
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	Details []ValidationErrorDetail `json:"details,omitempty"`
}

// single field validation error
type ValidationErrorDetail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
