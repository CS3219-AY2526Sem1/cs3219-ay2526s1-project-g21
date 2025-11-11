package handlers

import (
	"context"
	"text/template"

	"peerprep/ai/internal/models"
)

type mockProvider struct {
	generateContentFn func(ctx context.Context, prompt string, requestID string, detailLevel string) (*models.GenerationResponse, error)
	getProviderNameFn func() string
}

func (m *mockProvider) GenerateContent(ctx context.Context, prompt string, requestID string, detailLevel string) (*models.GenerationResponse, error) {
	if m.generateContentFn == nil {
		return &models.GenerationResponse{}, nil
	}
	return m.generateContentFn(ctx, prompt, requestID, detailLevel)
}

func (m *mockProvider) GetProviderName() string {
	if m.getProviderNameFn == nil {
		return "mock"
	}
	return m.getProviderNameFn()
}

type mockPromptManager struct {
	buildPromptFn  func(mode, variant string, data interface{}) (string, error)
	getTemplatesFn func() map[string]map[string]*template.Template
}

func (m *mockPromptManager) BuildPrompt(mode, variant string, data interface{}) (string, error) {
	if m.buildPromptFn == nil {
		return "mock prompt", nil
	}
	return m.buildPromptFn(mode, variant, data)
}

func (m *mockPromptManager) GetTemplates() map[string]map[string]*template.Template {
	if m.getTemplatesFn == nil {
		return map[string]map[string]*template.Template{
			"explain": {
				"beginner": template.Must(template.New("test").Parse("test")),
			},
		}
	}
	return m.getTemplatesFn()
}
