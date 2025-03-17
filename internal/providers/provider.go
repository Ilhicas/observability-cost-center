package providers

import (
	"time"
)

// UsageData represents usage metrics from a provider
type UsageData struct {
	Service   string
	Metric    string
	Value     float64
	Unit      string
	Timestamp time.Time
}

// CostData represents cost metrics from a provider
type CostData struct {
	Service     string
	ItemName    string
	Cost        float64
	Currency    string
	Period      string
	StartTime   time.Time
	EndTime     time.Time
	Region      string
	Quantity    float64
	UsageUnit   string
	AccountID   string
	Description string
}

// Provider interface that must be implemented by each observability provider
type Provider interface {
	// GetUsageData retrieves usage metrics for the given time range
	GetUsageData(start, end time.Time) ([]UsageData, error)

	// GetCostData retrieves cost metrics for the given time range
	GetCostData(start, end time.Time) ([]CostData, error)

	// GetName returns the provider name
	GetName() string
}
