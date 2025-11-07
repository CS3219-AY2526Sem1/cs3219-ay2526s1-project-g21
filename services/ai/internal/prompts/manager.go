package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// embeds all .yaml files in the templates folder into Go program at compile time
//
//go:embed templates/*.yaml
var templateFS embed.FS

type PromptManager struct {
	templates map[string]map[string]*template.Template // mode -> variant -> compiled template
}

// loaded prompt template
type PromptTemplate struct {
	BasePrompt string            `yaml:"base_prompt"`
	Prompts    map[string]string `yaml:"prompts"`
}

// creates a new prompt manager and loads templates
func NewPromptManager() (*PromptManager, error) {
	pm := &PromptManager{
		templates: make(map[string]map[string]*template.Template),
	}

	if err := pm.loadPrompts(); err != nil {
		return nil, fmt.Errorf("failed to load prompt templates: %w", err)
	}

	return pm, nil
}

// builds a prompt for the given mode and variant with dynamic data
func (pm *PromptManager) BuildPrompt(mode, variant string, data interface{}) (string, error) {
	modeTemplates, exists := pm.templates[mode]
	if !exists {
		return "", fmt.Errorf("template not found for mode: %s", mode)
	}

	tmpl, exists := modeTemplates[variant]
	if !exists {
		return "", fmt.Errorf("variant '%s' not found for mode '%s'", variant, mode)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// return the lloaded templates map for health checking
func (pm *PromptManager) GetTemplates() map[string]map[string]*template.Template {
	return pm.templates
}

// loadPrompts loads all YAML prompt files from the embedded filesystem
func (pm *PromptManager) loadPrompts() error {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return fmt.Errorf("failed to read templates directory: %w", err)
	}

	// Helper function for template functions
	funcMap := template.FuncMap{
		"join": strings.Join,
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
		pm.templates[name] = make(map[string]*template.Template)

		for variant, variantPrompt := range promptTemplate.Prompts {
			var fullPrompt strings.Builder
			if promptTemplate.BasePrompt != "" {
				fullPrompt.WriteString(promptTemplate.BasePrompt)
				fullPrompt.WriteString("\n\n")
			}
			fullPrompt.WriteString(variantPrompt)

			// Compile template with helper functions
			tmpl, err := template.New(fmt.Sprintf("%s_%s", name, variant)).Funcs(funcMap).Parse(fullPrompt.String())
			if err != nil {
				return fmt.Errorf("failed to compile template %s:%s: %w", name, variant, err)
			}

			pm.templates[name][variant] = tmpl
		}
	}

	return nil
}
