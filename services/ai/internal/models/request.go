package models

import (
	"fmt"
	"peerprep/ai/internal/utils"
	"strings"
)

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

	// normalize and validate language
	originalLanguage := r.Language
	r.Language = utils.NormalizeLanguage(r.Language)

	if !SupportedLanguages[r.Language] {
		return &ErrorResponse{
			Code:    "unsupported_language",
			Message: fmt.Sprintf("Language '%s' not supported. Supported languages: %s", originalLanguage, strings.Join(SupportedLanguagesList(), ", ")),
		}
	}

	// handle default difficulty level if not provided
	if r.DetailLevel == "" {
		r.DetailLevel = "intermediate"
	} else {
		originalDetailLevel := r.DetailLevel
		r.DetailLevel = utils.NormalizeLevel(r.DetailLevel)

		if !ValidDetailLevels[r.DetailLevel] {
			return &ErrorResponse{
				Code:    "invalid_detail_level",
				Message: fmt.Sprintf("Detail level '%s' not supported. Valid levels: %s", originalDetailLevel, strings.Join(ValidDetailLevelsList(), ", ")),
			}
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

	// normalize and validate language
	originalLanguage := r.Language
	r.Language = utils.NormalizeLanguage(r.Language)

	if !SupportedLanguages[r.Language] {
		return &ErrorResponse{
			Code:    "unsupported_language",
			Message: fmt.Sprintf("Language '%s' not supported. Supported languages: %s", originalLanguage, strings.Join(SupportedLanguagesList(), ", ")),
		}
	}

	if r.Question == nil {
		return &ErrorResponse{Code: "missing_question_context", Message: "Question context is required"}
	}
	if r.Question.PromptMarkdown == "" {
		return &ErrorResponse{Code: "missing_question_prompt", Message: "Question prompt_markdown must not be empty"}
	}

	originalHintLevel := r.HintLevel
	r.HintLevel = utils.NormalizeLevel(r.HintLevel)

	if !ValidHintLevels[r.HintLevel] {
		return &ErrorResponse{
			Code:    "invalid_hint_level",
			Message: fmt.Sprintf("Hint level '%s' not supported. Valid levels: %s", originalHintLevel, strings.Join(ValidHintLevelsList(), ", ")),
		}
	}

	if r.Question.Difficulty != "" {
		r.Question.Difficulty = utils.NormalizeDifficulty(r.Question.Difficulty)
	}

	return nil
}
