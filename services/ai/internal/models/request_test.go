package models

import (
	"strings"
	"testing"
)

func expectErrCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s but got nil", code)
	}
	resp, ok := err.(*ErrorResponse)
	if !ok {
		t.Fatalf("expected ErrorResponse, got %T", err)
	}
	if resp.Code != code {
		t.Fatalf("expected error code %s, got %s", code, resp.Code)
	}
}

func TestErrorResponse_Error(t *testing.T) {
	err := &ErrorResponse{Message: "failed"}
	if err.Error() != "failed" {
		t.Fatalf("expected message to be returned, got %s", err.Error())
	}
}

func TestSupportedLists(t *testing.T) {
	if got := strings.Join(SupportedLanguagesList(), ","); got != "python,java,cpp,javascript" {
		t.Fatalf("unexpected languages list: %s", got)
	}
	if got := strings.Join(ValidDetailLevelsList(), ","); got != "beginner,intermediate,advanced" {
		t.Fatalf("unexpected detail levels: %s", got)
	}
	if got := strings.Join(ValidHintLevelsList(), ","); got != "beginner,intermediate,advanced" {
		t.Fatalf("unexpected hint levels: %s", got)
	}
}

func TestExplainRequestValidate(t *testing.T) {
	t.Run("missing code", func(t *testing.T) {
		req := &ExplainRequest{}
		expectErrCode(t, req.Validate(), "missing_code")
	})

	t.Run("missing language", func(t *testing.T) {
		req := &ExplainRequest{Code: "print(1)"}
		expectErrCode(t, req.Validate(), "missing_language")
	})

	t.Run("unsupported language", func(t *testing.T) {
		req := &ExplainRequest{Code: "print(1)", Language: "Ruby"}
		expectErrCode(t, req.Validate(), "unsupported_language")
	})

	t.Run("invalid detail level", func(t *testing.T) {
		req := &ExplainRequest{Code: "print(1)", Language: "python", DetailLevel: "expert"}
		expectErrCode(t, req.Validate(), "invalid_detail_level")
	})

	t.Run("valid request normalizes values", func(t *testing.T) {
		req := &ExplainRequest{Code: "print(1)", Language: " PYTHON ", DetailLevel: ""}
		if err := req.Validate(); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if req.DetailLevel != DefaultDetailLevel {
			t.Fatalf("expected default detail level %s, got %s", DefaultDetailLevel, req.DetailLevel)
		}
		if req.Language != "python" {
			t.Fatalf("expected normalized language python, got %s", req.Language)
		}
	})
}

func TestHintRequestValidate(t *testing.T) {
	baseQuestion := &QuestionContext{PromptMarkdown: "Explain", Difficulty: "Medium"}

	t.Run("missing question", func(t *testing.T) {
		req := &HintRequest{Code: "x", Language: "python"}
		expectErrCode(t, req.Validate(), "missing_question_context")
	})

	t.Run("missing question prompt", func(t *testing.T) {
		req := &HintRequest{Code: "x", Language: "python", Question: &QuestionContext{}}
		expectErrCode(t, req.Validate(), "missing_question_prompt")
	})

	t.Run("invalid hint level", func(t *testing.T) {
		req := &HintRequest{Code: "x", Language: "python", Question: baseQuestion, HintLevel: "expert"}
		expectErrCode(t, req.Validate(), "invalid_hint_level")
	})

	t.Run("invalid language", func(t *testing.T) {
		req := &HintRequest{Code: "x", Language: "ruby", Question: baseQuestion, HintLevel: "beginner"}
		expectErrCode(t, req.Validate(), "unsupported_language")
	})

	t.Run("valid request normalizes", func(t *testing.T) {
		req := &HintRequest{
			Code:      "print",
			Language:  " PYTHON ",
			HintLevel: "BEGINNER",
			Question:  baseQuestion,
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if req.Language != "python" {
			t.Fatalf("language not normalized: %s", req.Language)
		}
		if req.HintLevel != "beginner" {
			t.Fatalf("hint level not normalized: %s", req.HintLevel)
		}
		if req.Question.Difficulty != "medium" {
			t.Fatalf("difficulty not normalized: %s", req.Question.Difficulty)
		}
	})
}

func TestTestGenRequestValidate(t *testing.T) {
	baseQuestion := &QuestionContext{PromptMarkdown: "Prompt"}

	t.Run("missing trimmed code", func(t *testing.T) {
		req := &TestGenRequest{Code: "   ", Language: "python"}
		expectErrCode(t, req.Validate(), "missing_code")
	})

	t.Run("missing prompt", func(t *testing.T) {
		req := &TestGenRequest{Code: "x", Language: "python", Question: &QuestionContext{}}
		expectErrCode(t, req.Validate(), "missing_question_prompt")
	})

	t.Run("missing question", func(t *testing.T) {
		req := &TestGenRequest{Code: "x", Language: "python"}
		expectErrCode(t, req.Validate(), "missing_question_context")
	})

	t.Run("invalid language", func(t *testing.T) {
		req := &TestGenRequest{Code: "x", Language: "ruby", Question: baseQuestion}
		expectErrCode(t, req.Validate(), "unsupported_language")
	})

	t.Run("normalize framework", func(t *testing.T) {
		req := &TestGenRequest{
			Code:      "x",
			Language:  "PYTHON",
			Question:  baseQuestion,
			Framework: "PyTest",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if req.Framework != "pytest" {
			t.Fatalf("framework not normalized: %s", req.Framework)
		}
	})
}

func TestRefactorTipsRequestValidate(t *testing.T) {
	t.Run("missing question context", func(t *testing.T) {
		req := &RefactorTipsRequest{Code: "x", Language: "python"}
		expectErrCode(t, req.Validate(), "missing_question_context")
	})

	t.Run("invalid language", func(t *testing.T) {
		req := &RefactorTipsRequest{Code: "x", Language: "ruby", Question: &QuestionContext{PromptMarkdown: "q"}}
		expectErrCode(t, req.Validate(), "unsupported_language")
	})

	t.Run("valid request", func(t *testing.T) {
		req := &RefactorTipsRequest{
			Code:     "x",
			Language: " PYTHON ",
			Question: &QuestionContext{PromptMarkdown: "q"},
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})
}
