package reports

import (
	"fmt"
	"sort"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
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
		return r.outputAsTable()
	case "summary":
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

		// Group cost data by account
		accountGroups := make(map[string][]providers.CostData)
		accountIDs := make([]string, 0)

		for _, cost := range r.CostData {
			if _, exists := accountGroups[cost.AccountID]; !exists {
				accountIDs = append(accountIDs, cost.AccountID)
			}
			accountGroups[cost.AccountID] = append(accountGroups[cost.AccountID], cost)
		}

		// Sort account IDs for consistent output
		sort.Strings(accountIDs)

		// For each account, group by day
		for _, accountID := range accountIDs {
			fmt.Printf("\n=== Account: %s ===\n", accountID)

			// Group by day
			dayGroups := make(map[string][]providers.CostData)
			days := make([]string, 0)

			for _, cost := range accountGroups[accountID] {
				day := cost.StartTime.Format("2006-01-02")
				if _, exists := dayGroups[day]; !exists {
					days = append(days, day)
				}
				dayGroups[day] = append(dayGroups[day], cost)
			}

			// Sort days chronologically
			sort.Strings(days)

			// Display costs by day
			fmt.Printf("%-12s %-25s %-10s %-10s %-20s %-15s\n",
				"Date", "Service", "Cost", "Usage", "Unit", "Description")
			fmt.Println("-----------------------------------------------------------------------------")

			// Track totals
			totalCost := 0.0
			totalUsage := 0.0
			currency := ""

			for _, day := range days {
				costs := dayGroups[day]

				// Daily subtotal
				dailyCost := 0.0
				dailyUsage := 0.0

				for _, cost := range costs {
					dailyCost += cost.Cost
					dailyUsage += cost.Quantity
					currency = cost.Currency

					fmt.Printf("%-12s %-25s %-10.4f %-10.2f %-20s %-15s\n",
						cost.StartTime.Format("2006-01-02"),
						cost.Service,
						cost.Cost,
						cost.Quantity,
						cost.UsageUnit,
						cost.Description)
				}

				// Add daily totals
				fmt.Printf("%-12s %-25s %-10.4f %-10.2f %s\n",
					day,
					"DAILY TOTAL",
					dailyCost,
					dailyUsage,
					currency)
				fmt.Println("-----------------------------------------------------------------------------")

				totalCost += dailyCost
				totalUsage += dailyUsage
			}

			// Account total
			fmt.Printf("\nAccount Total: %.4f %s (Usage: %.2f units)\n", totalCost, currency, totalUsage)
		}

		// Overall total
		fmt.Println("\n=== Overall Totals ===")
		var grandTotalCost float64
		var grandTotalUsage float64
		currency := ""

		fmt.Printf("%-20s %-10s %-10s\n", "Account", "Cost", "Usage")
		fmt.Println("------------------------------------------")

		for _, accountID := range accountIDs {
			accountCost := 0.0
			accountUsage := 0.0

			for _, cost := range accountGroups[accountID] {
				accountCost += cost.Cost
				accountUsage += cost.Quantity
				currency = cost.Currency
			}

			fmt.Printf("%-20s %-10.4f %-10.2f\n", accountID, accountCost, accountUsage)
			grandTotalCost += accountCost
			grandTotalUsage += accountUsage
		}

		fmt.Println("------------------------------------------")
		fmt.Printf("%-20s %-10.4f %-10.2f %s\n", "GRAND TOTAL", grandTotalCost, grandTotalUsage, currency)
	}

	return nil
}
