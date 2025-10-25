package models

type ExplainRequest struct {
	Code        string `json:"code"`
	Language    string `json:"language"`
	DetailLevel string `json:"detail_level"`
	RequestID   string `json:"request_id"`
}

// Future hint request structure
// type HintRequest struct {
// 	Code       string          `json:"code"`
// 	Language   string          `json:"language"`
// 	QuestionID *int            `json:"question_id,omitempty"`
// 	Question   *QuestionContext `json:"question,omitempty"`
// 	RequestID  string          `json:"request_id"`
// }

// QuestionContext for future hint mode extension
// type QuestionContext struct {
// 	ID             int      `json:"id"`
// 	Title          string   `json:"title"`
// 	PromptMarkdown string   `json:"prompt_markdown"`
// 	Difficulty     string   `json:"difficulty"`
// 	TopicTags      []string `json:"topic_tags"`
// 	Constraints    string   `json:"constraints,omitempty"`
// }
