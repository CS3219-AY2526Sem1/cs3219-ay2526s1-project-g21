package gemini

import "peerprep/ai/internal/llm"

// Register Gemini provider on package import
func init() {
	llm.RegisterProvider("gemini", func() (llm.Provider, error) {
		config, err := NewConfig()
		if err != nil {
			return nil, err
		}
		return NewClient(config)
	})
}
