package models

// represents pagination parameters for queries
type PaginationParams struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// helper to calculate pagination metadata
func CalculatePaginationMeta(page, limit, total int) (totalPages int, hasNext, hasPrev bool) {
	if limit <= 0 {
		limit = 1 // avoid division by zero
	}
	totalPages = (total + limit - 1) / limit // ceiling division
	hasNext = page < totalPages
	hasPrev = page > 1
	return
}

// represents the response structure for /questions endpoint
type QuestionsResponse struct {
	Total      int        `json:"total"`
	Items      []Question `json:"items"`
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
	TotalPages int        `json:"totalPages"`
	HasNext    bool       `json:"hasNext"`
	HasPrev    bool       `json:"hasPrev"`
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
