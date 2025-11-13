package prompts

import (
	"strings"
	"testing"
)

func TestPromptManagerBuildPrompt(t *testing.T) {
	pm, err := NewPromptManager()
	if err != nil {
		t.Fatalf("NewPromptManager error: %v", err)
	}

	data := map[string]string{
		"Language": "Python",
		"Code":     "print('hello')",
	}
	prompt, err := pm.BuildPrompt("explain", "beginner", data)
	if err != nil {
		t.Fatalf("BuildPrompt error: %v", err)
	}

	if len(prompt) == 0 || !containsAll(prompt, []string{"Python", "print('hello')"}) {
		t.Fatalf("prompt did not contain expected values: %s", prompt)
	}

	if _, err := pm.BuildPrompt("unknown", "beginner", data); err == nil {
		t.Fatalf("expected error for unknown mode")
	}

	if _, err := pm.BuildPrompt("explain", "missing", data); err == nil {
		t.Fatalf("expected error for missing variant")
	}

	if len(pm.GetTemplates()) == 0 {
		t.Fatalf("expected templates to be loaded")
	}
}

func containsAll(haystack string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}
