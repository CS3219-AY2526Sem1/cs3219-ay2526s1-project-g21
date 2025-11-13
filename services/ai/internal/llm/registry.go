package llm

import "fmt"

// defines a function that creates a new provider instance
type ProviderFactory func() (Provider, error)

// global registry of available providers
var providers = make(map[string]ProviderFactory)

// registers a provider factory with the given name
func RegisterProvider(name string, factory ProviderFactory) {
	providers[name] = factory
}

// creates a new provider instance based on the given name
func NewProvider(name string) (Provider, error) {
	factory, exists := providers[name]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", name)
	}
	return factory()
}
