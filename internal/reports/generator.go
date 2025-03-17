package reports

import (
	"fmt"
	"time"
)

// ReportType defines the type of report to generate
type ReportType string

const (
	UsageReport ReportType = "usage"
	CostReport  ReportType = "cost"
	FullReport  ReportType = "full"
)

// GenerateReport generates a report based on the specified report type
func (rg *ReportGenerator) GenerateReport(reportType ReportType, start, end time.Time) (*Report, error) {
	report := &Report{
		ProviderName: rg.provider.GetName(),
		StartDate:    start,
		EndDate:      end,
	}

	var err error

	// Fetch usage data if needed
	if reportType == UsageReport || reportType == FullReport {
		report.UsageData, err = rg.provider.GetUsageData(start, end)
		if err != nil {
			return nil, fmt.Errorf("error getting usage data: %w", err)
		}
	}

	// Fetch cost data if needed
	if reportType == CostReport || reportType == FullReport {
		report.CostData, err = rg.provider.GetCostData(start, end)
		if err != nil {
			return nil, fmt.Errorf("error getting cost data: %w", err)
		}

		// Calculate total cost
		report.TotalCost = 0.0
		for _, cost := range report.CostData {
			report.TotalCost += cost.Cost
		}
	}

	return report, nil
}

// GenerateUsageReport generates a report with only usage data
func (rg *ReportGenerator) GenerateUsageReport(start, end time.Time) (*Report, error) {
	return rg.GenerateReport(UsageReport, start, end)
}

// GenerateCostReport generates a report with only cost data
func (rg *ReportGenerator) GenerateCostReport(start, end time.Time) (*Report, error) {
	return rg.GenerateReport(CostReport, start, end)
}

// GenerateFullReport generates a report with both usage and cost data
func (rg *ReportGenerator) GenerateFullReport(start, end time.Time) (*Report, error) {
	return rg.GenerateReport(FullReport, start, end)
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
