package utils

import "regexp"

// isPasswordValid enforces password policy (≥8 chars, ≥1 special char)
func IsPasswordValid(p string) bool {
	if len(p) < 8 {
		return false
	}
	// regex: at least one non-alphanumeric character
	re := regexp.MustCompile(`[!@#\$%\^&\*\(\)\-_=\+\[\]\{\}\\|;:'",<>\./\?]`)
	return re.MatchString(p)
}
