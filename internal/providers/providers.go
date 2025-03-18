package providers

import (
	"fmt"
	"strings"
	"time"
)

// Provider interface defines methods that all observability providers must implement
type Provider interface {
	GetName() string
	GetUsageData(start, end time.Time) ([]UsageData, error)
	GetCostData(start, end time.Time) ([]CostData, error)
}

// Registry stores all registered providers
var registry = make(map[string]func() (Provider, error))

// RegisterProvider registers a new provider factory function with the given name
func RegisterProvider(name string, factory func() (Provider, error)) {
	registry[strings.ToLower(name)] = factory
}

// GetProvider returns a provider instance by name
func GetProvider(name string) (Provider, error) {
	factory, exists := registry[strings.ToLower(name)]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", name)
	}

	return factory()
}

// ListProviders returns a list of all registered provider names
func ListProviders() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
