package utils

import (
	"fmt"
	"strings"
)

func NormalizeLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func NormalizeLevel(level string) string {
	return strings.ToLower(strings.TrimSpace(level))
}

func NormalizeDifficulty(difficulty string) string {
	return strings.ToLower(strings.TrimSpace(difficulty))
}

// StripFences removes markdown code fence markers from a string
func StripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s[3:], '\n'); i >= 0 {
			s = s[3+i+1:]
		}
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// AddLineNumbers adds line numbers to source code for LLM reference
func AddLineNumbers(src string) string {
	if src == "" {
		return ""
	}
	lines := strings.Split(src, "\n")
	for i := range lines {
		lines[i] = fmt.Sprintf("%d: %s", i+1, lines[i])
	}
	return strings.Join(lines, "\n")
}
