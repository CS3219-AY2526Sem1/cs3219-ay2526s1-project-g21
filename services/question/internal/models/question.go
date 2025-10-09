package models

import "time"

type Question struct {
	ID             string     `json:"id"`         // uuid, can also be number
	Title          string     `json:"title"`      // question title
	Difficulty     Difficulty `json:"difficulty"` // enum
	TopicTags      []string   `json:"topic_tags,omitempty" validate:"max=10"`
	PromptMarkdown string     `json:"prompt_markdown"`
	Constraints    string     `json:"constraints,omitempty"`
	TestCases      []TestCase `json:"test_cases,omitempty"`
	ImageURLs      []string   `json:"image_urls,omitempty" validate:"max=5"` // optional; need to validate urls when used

	Status           Status     `json:"status,omitempty"` // active or deprecated. read the struct for more deets
	Author           string     `json:"author,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeprecatedAt     *time.Time `json:"deprecated_at,omitempty"`
	DeprecatedReason string     `json:"deprecated_reason,omitempty"`
}

type Difficulty string

const (
	Easy   Difficulty = "Easy"
	Medium Difficulty = "Medium"
	Hard   Difficulty = "Hard"
)

// status describes lifecycle state of a question
// like for example, if a question is deprecated
// we'd still want to be able to fetch it for historical purposes
type Status string

const (
	StatusActive     Status = "active"
	StatusDeprecated Status = "deprecated"
)

// single testcase
type TestCase struct {
	Input       string `json:"input" validate:"required"`
	Output      string `json:"output" validate:"required"`
	Description string `json:"description,omitempty"` // optional test case description
}