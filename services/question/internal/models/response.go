package models

// represents the response structure for /questions endpoint, all questions sent for now
// TODO: implement other question fetching endpoints
// - Fetch by id
// - Fetch random truly
// - Fetch with criteria
type QuestionsResponse struct {
	Total int        `json:"total"`
	Items []Question `json:"items"`
}

// uniform error payload
type ErrorResponse struct {
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	Details []ValidationErrorDetail `json:"details,omitempty"`
}

// a single field error
type ValidationErrorDetail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
