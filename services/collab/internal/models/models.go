package models

type Language string

const (
	LangPython Language = "python"
	LangJava   Language = "java"
	LangCPP    Language = "cpp"
)

type LanguageSpec struct {
	Name            Language `json:"name"`
	FileName        string   `json:"fileName"`
	RunCmd          []string `json:"runCmd"`
	CompileCmd      []string `json:"compileCmd"`
	ExecCmd         []string `json:"execCmd"`
	DefaultTabSize  int      `json:"defaultTabSize"`
	Formatter       []string `json:"formatter"`
	ExampleTemplate string   `json:"exampleTemplate"`
}

type RunRequest struct {
	Language Language `json:"language"`
	Code     string   `json:"code"`
	Stdin    string   `json:"stdin,omitempty"`
}

type RunResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Exit     int    `json:"exit"`
	TimedOut bool   `json:"timedOut"`
}

type FormatRequest struct {
	Language Language `json:"language"`
	Code     string   `json:"code"`
}

type FormatResponse struct {
	Formatted string `json:"formatted"`
}

/*** Collaboration session state ***/
type DocState struct {
	Text    string `json:"text"`
	Version int64  `json:"version"`
}

type WSFrame struct {
	Type string      `json:"type"` // "init","edit","cursor","chat","run","language","stdout","stderr","exit","error","doc"
	Data interface{} `json:"data"`
}

type InitRequest struct {
	SessionID string   `json:"sessionId"`
	Language  Language `json:"language"` // current language tab
}

type InitResponse struct {
	SessionID string   `json:"sessionId"`
	Doc       DocState `json:"doc"`
	Language  Language `json:"language"`
}

type Edit struct {
	BaseVersion int64  `json:"baseVersion"`
	RangeStart  int    `json:"rangeStart"` // inclusive index
	RangeEnd    int    `json:"rangeEnd"`   // exclusive
	Text        string `json:"text"`
}

type Cursor struct {
	UserID string `json:"userId"`
	Pos    int    `json:"pos"`
}

type Chat struct {
	UserID  string `json:"userId"`
	Message string `json:"message"`
}

type RunCmd struct {
	Language Language `json:"language"`
	Code     string   `json:"code"`
	Stdin    string   `json:"stdin,omitempty"`
}

type LanguageChange struct {
	Language Language `json:"language"`
}

// Room status and question info (used for both match events and room status)
type RoomInfo struct {
	MatchId          string    `json:"matchId"`
	User1            string    `json:"user1"`
	User2            string    `json:"user2"`
	Category         string    `json:"category"`
	Difficulty       string    `json:"difficulty"`
	Status           string    `json:"status"` // "pending", "ready", "error"
	Question         *Question `json:"question,omitempty"`
	RerollsRemaining int       `json:"rerollsRemaining"`
	CreatedAt        string    `json:"createdAt"`
	Token1           string    `json:"token1,omitempty"`
	Token2           string    `json:"token2,omitempty"`
}

type QuestionUpdate struct {
	Question         *Question `json:"question"`
	RerollsRemaining int       `json:"rerollsRemaining"`
}

// Question model (simplified from question service)
type Question struct {
	ID             int        `json:"id"`
	Title          string     `json:"title"`
	Difficulty     string     `json:"difficulty"`
	TopicTags      []string   `json:"topic_tags,omitempty"`
	PromptMarkdown string     `json:"prompt_markdown"`
	Constraints    string     `json:"constraints,omitempty"`
	TestCases      []TestCase `json:"test_cases,omitempty"`
	ImageURLs      []string   `json:"image_urls,omitempty"`
}

type TestCase struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Description string `json:"description,omitempty"`
}

// SessionEndedEvent is published when a session ends
type SessionEndedEvent struct {
	MatchID      string   `json:"matchId"`
	User1        string   `json:"user1"`
	User2        string   `json:"user2"`
	QuestionID   int      `json:"questionId"`
	QuestionTitle string  `json:"questionTitle"`
	Category     string   `json:"category"`
	Difficulty   string   `json:"difficulty"`
	Language     string   `json:"language"`
	FinalCode    string   `json:"finalCode"`
	StartedAt    string   `json:"startedAt"`
	EndedAt      string   `json:"endedAt"`
	DurationSec  int      `json:"durationSeconds"`
	RerollsUsed  int      `json:"rerollsUsed"`
}
