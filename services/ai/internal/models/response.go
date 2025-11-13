package models

// generic AI content generation response
type GenerationResponse struct {
	Content   string             `json:"content"`
	RequestID string             `json:"request_id"`
	Metadata  GenerationMetadata `json:"metadata"`
}

// additional information about the generation
type GenerationMetadata struct {
	ProcessingTime int    `json:"processing_time_ms"`
	DetailLevel    string `json:"detail_level"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	ModelVersion   string `json:"model_version,omitempty"` // For feedback tracking
}

// uniform error responses
type ErrorResponse struct {
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	Details []ValidationErrorDetail `json:"details,omitempty"`
}

// Error implements the error interface
func (e *ErrorResponse) Error() string {
	return e.Message
}

// single field validation error
type ValidationErrorDetail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// HintResponse returned by /ai/hint
type HintResponse struct {
	Hint      string             `json:"hint"`
	RequestID string             `json:"request_id"`
	Metadata  GenerationMetadata `json:"metadata"`
}

type TestGenResponse struct {
	TestsCode string             `json:"tests_code"`
	RequestID string             `json:"request_id"`
	Metadata  GenerationMetadata `json:"metadata"`
}

type RefactorTipsTextResponse struct {
	TipsText  string             `json:"tips_text"`
	RequestID string             `json:"request_id"`
	Metadata  GenerationMetadata `json:"metadata"`
}

// Resp is a generic API response wrapper
type Resp struct {
	OK   bool        `json:"ok"`
	Info interface{} `json:"info,omitempty"`
}
