package newrelic

import (
	"fmt"
	"os"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
	"github.com/newrelic/newrelic-client-go/newrelic"
)

// NewRelicProvider implements the Provider interface for New Relic
type NewRelicProvider struct {
	client *newrelic.NewRelic
}

// NewProvider creates a new New Relic provider
func NewProvider() (*NewRelicProvider, error) {
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("NEW_RELIC_API_KEY environment variable not set")
	}

	// Correct way to initialize the New Relic client using Personal API Key
	client, err := newrelic.New(newrelic.ConfigAdminAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating New Relic client: %w", err)
	}

	return &NewRelicProvider{client: client}, nil
}

// GetName returns the provider name
func (nr *NewRelicProvider) GetName() string {
	return "New Relic"
}

// GetUsageData retrieves usage metrics from New Relic
func (nr *NewRelicProvider) GetUsageData(start, end time.Time) ([]providers.UsageData, error) {
	// In a real implementation, this would query the New Relic API for usage data
	// For now, we'll return sample data

	// In a real implementation, you would use the New Relic NerdGraph API to query usage data
	// Something like:
	// query := `{ actor { account(id: $accountId) { nrql(query: "SELECT sum(newRelicDbSize) FROM NrDailyUsage SINCE '` + start.Format("2006-01-02") + `' UNTIL '` + end.Format("2006-01-02") + `' FACET productLine") { results } } } }`

	// For now, return sample data
	return []providers.UsageData{
		{
			Service:   "APM",
			Metric:    "ComputeUnits",
			Value:     1250.5,
			Unit:      "CU",
			Timestamp: start.AddDate(0, 0, 1),
		},
		{
			Service:   "Infrastructure",
			Metric:    "HostHours",
			Value:     720.0,
			Unit:      "Hours",
			Timestamp: start.AddDate(0, 0, 1),
		},
		{
			Service:   "Logs",
			Metric:    "GBIngested",
			Value:     512.75,
			Unit:      "GB",
			Timestamp: start.AddDate(0, 0, 1),
		},
	}, nil
}

// GetCostData retrieves cost data from New Relic
func (nr *NewRelicProvider) GetCostData(start, end time.Time) ([]providers.CostData, error) {
	// In a real implementation, this would query the New Relic API for billing data
	// For now, we'll return sample data
	return []providers.CostData{
		{
			Service:   "APM",
			ItemName:  "ComputeUnits",
			Cost:      625.25,
			Currency:  "USD",
			Period:    "Monthly",
			StartTime: start,
			EndTime:   end,
		},
		{
			Service:   "Infrastructure",
			ItemName:  "HostMonitoring",
			Cost:      360.00,
			Currency:  "USD",
			Period:    "Monthly",
			StartTime: start,
			EndTime:   end,
		},
		{
			Service:   "Logs",
			ItemName:  "Ingestion",
			Cost:      820.40,
			Currency:  "USD",
			Period:    "Monthly",
			StartTime: start,
			EndTime:   end,
		},
	}, nil
}
