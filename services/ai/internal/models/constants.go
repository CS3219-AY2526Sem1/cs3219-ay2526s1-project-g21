package models

// contains all supported programming languages (in lowercase)
var SupportedLanguages = map[string]bool{
	"python":     true,
	"java":       true,
	"cpp":        true,
	"javascript": true,
}

// contains all valid detail levels for explanations (in lowercase)
var ValidDetailLevels = map[string]bool{
	"beginner":     true,
	"intermediate": true,
	"advanced":     true,
}

// contains all valid hint levels (in lowercase)
var ValidHintLevels = map[string]bool{
	"basic":        true,
	"intermediate": true,
	"advanced":     true,
}

func SupportedLanguagesList() []string {
	return []string{"python", "java", "cpp", "javascript"}
}

func ValidDetailLevelsList() []string {
	return []string{"beginner", "intermediate", "advanced"}
}

func ValidHintLevelsList() []string {
	return []string{"basic", "intermediate", "advanced"}
}
