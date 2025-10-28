package prompts

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// embeds all .yaml files in the templates folder into Go program at compile time
//
//go:embed templates/*.yaml
var templateFS embed.FS

type PromptManager struct {
	prompts map[string]map[string]string // mode -> detailLevel -> complete prompt
}

// loaded prompt template
type PromptTemplate struct {
	BasePrompt   string            `yaml:"base_prompt"`
	DetailLevels map[string]string `yaml:"detail_levels"`
}

// creates a new prompt manager and loads templates
func NewPromptManager() (*PromptManager, error) {
	pm := &PromptManager{
		prompts: make(map[string]map[string]string),
	}

	if err := pm.loadPrompts(); err != nil {
		return nil, fmt.Errorf("failed to load prompt templates: %w", err)
	}

	return pm, nil
}

// builds a prompt for the given mode and context
func (pm *PromptManager) BuildPrompt(mode, code, language, detailLevel string) (string, error) {
	modePrompts, exists := pm.prompts[mode]
	if !exists {
		return "", fmt.Errorf("template not found for mode: %s", mode)
	}

	promptTemplate, exists := modePrompts[detailLevel]
	if !exists {
		return "", fmt.Errorf("detail level '%s' not found for mode '%s'", detailLevel, mode)
	}

	// Simple string replacement instead of complex template execution
	result := strings.ReplaceAll(promptTemplate, "{{.Language}}", language)
	result = strings.ReplaceAll(result, "{{.Code}}", code)

	return result, nil
}

// loadPrompts loads all YAML prompt files from the embedded filesystem
func (pm *PromptManager) loadPrompts() error {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := templateFS.ReadFile("templates/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %w", entry.Name(), err)
		}

		var promptTemplate PromptTemplate
		if err := yaml.Unmarshal(data, &promptTemplate); err != nil {
			return fmt.Errorf("failed to parse template file %s: %w", entry.Name(), err)
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		pm.prompts[name] = make(map[string]string)

		for detailLevel, detailPrompt := range promptTemplate.DetailLevels {
			var fullPrompt strings.Builder
			if promptTemplate.BasePrompt != "" {
				fullPrompt.WriteString(promptTemplate.BasePrompt)
				fullPrompt.WriteString("\n\n")
			}
			fullPrompt.WriteString(detailPrompt)

			// Store the complete prompt as a string (no template compilation)
			pm.prompts[name][detailLevel] = fullPrompt.String()
		}
	}

	return nil
}
