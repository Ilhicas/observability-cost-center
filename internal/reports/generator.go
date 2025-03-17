package reports

import (
	"fmt"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
)

// ReportGenerator is responsible for generating reports using provider data
type ReportGenerator struct {
	provider providers.Provider
}

// Report represents a generated report with usage and cost data
type Report struct {
	ProviderName string
	StartDate    time.Time
	EndDate      time.Time
	UsageData    []providers.UsageData
	CostData     []providers.CostData
	TotalCost    float64
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(provider providers.Provider) *ReportGenerator {
	return &ReportGenerator{
		provider: provider,
	}
}

// GenerateUsageReport generates a report with only usage data
func (rg *ReportGenerator) GenerateUsageReport(start, end time.Time) (*Report, error) {
	usageData, err := rg.provider.GetUsageData(start, end)
	if err != nil {
		return nil, fmt.Errorf("error getting usage data: %w", err)
	}

	return &Report{
		ProviderName: rg.provider.GetName(),
		StartDate:    start,
		EndDate:      end,
		UsageData:    usageData,
	}, nil
}

// GenerateCostReport generates a report with only cost data
func (rg *ReportGenerator) GenerateCostReport(start, end time.Time) (*Report, error) {
	costData, err := rg.provider.GetCostData(start, end)
	if err != nil {
		return nil, fmt.Errorf("error getting cost data: %w", err)
	}

	totalCost := 0.0
	for _, cost := range costData {
		totalCost += cost.Cost
	}

	return &Report{
		ProviderName: rg.provider.GetName(),
		StartDate:    start,
		EndDate:      end,
		CostData:     costData,
		TotalCost:    totalCost,
	}, nil
}

// GenerateFullReport generates a report with both usage and cost data
func (rg *ReportGenerator) GenerateFullReport(start, end time.Time) (*Report, error) {
	usageData, err := rg.provider.GetUsageData(start, end)
	if err != nil {
		return nil, fmt.Errorf("error getting usage data: %w", err)
	}

	costData, err := rg.provider.GetCostData(start, end)
	if err != nil {
		return nil, fmt.Errorf("error getting cost data: %w", err)
	}

	totalCost := 0.0
	for _, cost := range costData {
		totalCost += cost.Cost
	}

	return &Report{
		ProviderName: rg.provider.GetName(),
		StartDate:    start,
		EndDate:      end,
		UsageData:    usageData,
		CostData:     costData,
		TotalCost:    totalCost,
	}, nil
}

// Output formats and outputs the report according to the specified format
func (r *Report) Output(format string) error {
	switch format {
	case "json":
		return r.outputJSON()
	case "csv":
		return r.outputCSV()
	case "table":
		return r.outputTable()
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// outputJSON outputs the report in JSON format
func (r *Report) outputJSON() error {
	// In a real implementation, use JSON marshaling
	fmt.Println("JSON output format not yet implemented")
	return nil
}

// outputCSV outputs the report in CSV format
func (r *Report) outputCSV() error {
	// In a real implementation, write CSV data
	fmt.Println("CSV output format not yet implemented")
	return nil
}

// outputTable outputs the report in a tabular format
func (r *Report) outputTable() error {
	fmt.Printf("Report for %s\n", r.ProviderName)
	fmt.Printf("Period: %s to %s\n\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))

	if len(r.UsageData) > 0 {
		fmt.Println("=== Usage Data ===")
		fmt.Printf("%-20s %-20s %-10s %-10s %-20s\n", "Service", "Metric", "Value", "Unit", "Timestamp")
		fmt.Println("-------------------------------------------------------------------------")

		for _, usage := range r.UsageData {
			fmt.Printf("%-20s %-20s %-10.2f %-10s %-20s\n",
				usage.Service,
				usage.Metric,
				usage.Value,
				usage.Unit,
				usage.Timestamp.Format("2006-01-02"))
		}
		fmt.Println()
	}

	if len(r.CostData) > 0 {
		fmt.Println("=== Cost Data ===")
		fmt.Printf("%-20s %-20s %-10s %-10s %-15s\n", "Service", "Item", "Cost", "Currency", "Period")
		fmt.Println("-------------------------------------------------------------------------")

		for _, cost := range r.CostData {
			fmt.Printf("%-20s %-20s %-10.2f %-10s %-15s\n",
				cost.Service,
				cost.ItemName,
				cost.Cost,
				cost.Currency,
				cost.Period)
		}
		fmt.Println()
		fmt.Printf("Total Cost: %.2f\n", r.TotalCost)
	}

	return nil
}
