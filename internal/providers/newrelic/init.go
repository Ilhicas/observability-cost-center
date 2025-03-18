package newrelic

import "github.com/ilhicas/observability-cost-center/internal/providers"

func init() {
	// Register New Relic provider factory
	providers.RegisterProvider("newrelic", func() (providers.Provider, error) {
		return NewProvider()
	})
}
