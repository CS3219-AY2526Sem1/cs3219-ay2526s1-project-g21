package utils

import "strings"

func NormalizeLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}
func NormalizeLevel(level string) string {
	return strings.ToLower(strings.TrimSpace(level))
}

func NormalizeDifficulty(difficulty string) string {
	return strings.ToLower(strings.TrimSpace(difficulty))
}
