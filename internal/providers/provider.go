package providers

import (
	"time"
)

// UsageData represents a single usage metric
type UsageData struct {
	Service   string                 `json:"service"`
	Metric    string                 `json:"metric"`
	Value     float64                `json:"value"`
	Unit      string                 `json:"unit"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Added for additional data like license utilization
}

// CostData represents a single cost item
type CostData struct {
	Service     string    `json:"service"`
	ItemName    string    `json:"itemName"`
	Cost        float64   `json:"cost"`
	Currency    string    `json:"currency"`
	Period      string    `json:"period"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	AccountID   string    `json:"accountId"`
	Region      string    `json:"region,omitempty"`
	Quantity    float64   `json:"quantity,omitempty"`
	UsageUnit   string    `json:"usageUnit,omitempty"`
	Description string    `json:"description,omitempty"`
}

// Provider interface defines methods all providers must implement
type Provider interface {
	GetName() string
	GetUsageData(start, end time.Time) ([]UsageData, error)
	GetCostData(start, end time.Time) ([]CostData, error)
}
