package models

type ExplainRequest struct {
	Code        string `json:"code"`
	Language    string `json:"language"`
	DetailLevel string `json:"detail_level"`
	RequestID   string `json:"request_id"`
}

// implements the Validator interface
func (r *ExplainRequest) Validate() error {
	// Validate required fields
	if r.Code == "" {
		return &ErrorResponse{
			Code:    "missing_code",
			Message: "Code field is required",
		}
	}

	if r.Language == "" {
		return &ErrorResponse{
			Code:    "missing_language",
			Message: "Language field is required",
		}
	}

	// Validate supported languages
	supportedLanguages := map[string]bool{
		"python":     true,
		"java":       true,
		"cpp":        true,
		"javascript": true,
	}

	if !supportedLanguages[r.Language] {
		return &ErrorResponse{
			Code:    "unsupported_language",
			Message: "Language not supported. Supported languages: python, java, cpp, javascript",
		}
	}

	// TODO: handle default difficulty level if not provided
	if r.DetailLevel == "" {
		r.DetailLevel = "intermediate"
	}

	// Validate detail level
	validDetailLevels := map[string]bool{
		"beginner":     true,
		"intermediate": true,
		"advanced":     true,
	}

	if !validDetailLevels[r.DetailLevel] {
		return &ErrorResponse{
			Code:    "invalid_detail_level",
			Message: "Detail level must be one of: beginner, intermediate, advanced",
		}
	}

	return nil
}

type QuestionContext struct {
	ID             int      `json:"id"`
	Title          string   `json:"title"`
	PromptMarkdown string   `json:"prompt_markdown"`
	Difficulty     string   `json:"difficulty"`
	TopicTags      []string `json:"topic_tags"`
	Constraints    string   `json:"constraints,omitempty"`
}

type HintRequest struct {
	Code      string           `json:"code"`
	Language  string           `json:"language"`
	Question  *QuestionContext `json:"question"`
	HintLevel string           `json:"hint_level"`
	RequestID string           `json:"request_id"`
}

func (r *HintRequest) Validate() error {
	if r.Code == "" {
		return &ErrorResponse{Code: "missing_code", Message: "Code field is required"}
	}
	if r.Language == "" {
		return &ErrorResponse{Code: "missing_language", Message: "Language field is required"}
	}
	supported := map[string]bool{"python": true, "java": true, "cpp": true, "javascript": true}
	if !supported[r.Language] {
		return &ErrorResponse{Code: "unsupported_language", Message: "Language not supported"}
	}
	if r.Question == nil {
		return &ErrorResponse{Code: "missing_question_context", Message: "Question context is required"}
	}
	if r.Question.PromptMarkdown == "" {
		return &ErrorResponse{Code: "missing_question_prompt", Message: "Question prompt_markdown must not be empty"}
	}

	levels := map[string]bool{"basic": true, "intermediate": true, "advanced": true}
	if !levels[r.HintLevel] {
		return &ErrorResponse{Code: "invalid_hint_level", Message: "Hint level must be basic, intermediate, or advanced"}
	}

	return nil
}
