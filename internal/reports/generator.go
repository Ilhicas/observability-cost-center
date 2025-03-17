package reports

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
	"github.com/olekukonko/tablewriter"
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
		ReportType:   string(reportType), // Add this line to set the report type
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
// and optionally writes to a file if filePath is provided
func (r *Report) Output(format string, filePath string) error {
	var writer io.Writer = os.Stdout

	// If filePath is provided, create and use the file
	if filePath != "" {
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		writer = file
	}

	switch format {
	case "json":
		return r.outputJSON(writer)
	case "csv":
		return r.outputCSV(writer)
	case "table":
		return r.outputAsTable(writer)
	case "summary":
		return r.outputTable(writer)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// outputJSON outputs the report in JSON format
func (r *Report) outputJSON(w io.Writer) error {
	// In a real implementation, use JSON marshaling
	fmt.Fprintln(w, "JSON output format not yet implemented")
	return nil
}

// outputCSV outputs the report in CSV format
func (r *Report) outputCSV(w io.Writer) error {
	// In a real implementation, write CSV data
	fmt.Fprintln(w, "CSV output format not yet implemented")
	return nil
}

// outputTable outputs the report in a tabular format
func (r *Report) outputTable(w io.Writer) error {
	// Add debug information to help diagnose issues
	fmt.Fprintf(w, "Report for %s\n", r.ProviderName)
	fmt.Fprintf(w, "Period: %s to %s\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))
	fmt.Fprintf(w, "Report Type: %s\n\n", r.ReportType)

	// Add debug counts
	fmt.Fprintf(w, "Usage data entries: %d\n", len(r.UsageData))
	fmt.Fprintf(w, "Cost data entries: %d\n\n", len(r.CostData))

	fmt.Fprintf(w, "Report for %s\n", r.ProviderName)
	fmt.Fprintf(w, "Period: %s to %s\n\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))

	if len(r.UsageData) > 0 {
		fmt.Fprintln(w, "=== Usage Data ===")
		fmt.Fprintf(w, "%-20s %-20s %-10s %-10s %-20s\n", "Service", "Metric", "Value", "Unit", "Timestamp")
		fmt.Fprintln(w, "-------------------------------------------------------------------------")

		for _, usage := range r.UsageData {
			fmt.Fprintf(w, "%-20s %-20s %-10.2f %-10s %-20s\n",
				usage.Service,
				usage.Metric,
				usage.Value,
				usage.Unit,
				usage.Timestamp.Format("2006-01-02"))
		}
		fmt.Fprintln(w, "")
	}

	if len(r.CostData) > 0 {
		fmt.Fprintln(w, "=== Cost Data ===")

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
			fmt.Fprintf(w, "\n=== Account: %s ===\n", accountID)

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
			fmt.Fprintf(w, "%-12s %-25s %-10s %-10s %-20s %-15s\n",
				"Date", "Service", "Cost", "Usage", "Unit", "Description")
			fmt.Fprintln(w, "-----------------------------------------------------------------------------")

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

					fmt.Fprintf(w, "%-12s %-25s %-10.4f %-10.2f %-20s %-15s\n",
						cost.StartTime.Format("2006-01-02"),
						cost.Service,
						cost.Cost,
						cost.Quantity,
						cost.UsageUnit,
						cost.Description)
				}

				// Add daily totals
				fmt.Fprintf(w, "%-12s %-25s %-10.4f %-10.2f %s\n",
					day,
					"DAILY TOTAL",
					dailyCost,
					dailyUsage,
					currency)
				fmt.Fprintln(w, "-----------------------------------------------------------------------------")

				totalCost += dailyCost
				totalUsage += dailyUsage
			}

			// Account total
			fmt.Fprintf(w, "\nAccount Total: %.4f %s (Usage: %.2f units)\n", totalCost, currency, totalUsage)
		}

		// Overall total
		fmt.Fprintln(w, "\n=== Overall Totals ===")
		var grandTotalCost float64
		var grandTotalUsage float64
		currency := ""

		fmt.Fprintf(w, "%-20s %-10s %-10s\n", "Account", "Cost", "Usage")
		fmt.Fprintln(w, "------------------------------------------")

		for _, accountID := range accountIDs {
			accountCost := 0.0
			accountUsage := 0.0

			for _, cost := range accountGroups[accountID] {
				accountCost += cost.Cost
				accountUsage += cost.Quantity
				currency = cost.Currency
			}

			fmt.Fprintf(w, "%-20s %-10.4f %-10.2f\n", accountID, accountCost, accountUsage)
			grandTotalCost += accountCost
			grandTotalUsage += accountUsage
		}

		fmt.Fprintln(w, "------------------------------------------")
		fmt.Fprintf(w, "%-20s %-10.4f %s %-10.2f events\n", "GRAND TOTAL", grandTotalCost, currency, grandTotalUsage)
	}

	return nil
}

// outputAsTable formats and displays the report as ASCII tables
func (r *Report) outputAsTable(w io.Writer) error {
	// Add debug information to help diagnose issues
	fmt.Fprintf(w, "Report for %s\n", r.ProviderName)
	fmt.Fprintf(w, "Period: %s to %s\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))
	fmt.Fprintf(w, "Report Type: %s\n\n", r.ReportType)

	// Add debug counts
	fmt.Fprintf(w, "Usage data entries: %d\n", len(r.UsageData))
	fmt.Fprintf(w, "Cost data entries: %d\n\n", len(r.CostData))

	// Create a writer that writes to the provided io.Writer
	tableWriter := &writerAdapter{w: w}

	fmt.Fprintf(w, "\n%s Report for %s\n", r.ReportType, r.ProviderName)
	fmt.Fprintf(w, "Period: %s to %s\n\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))

	// Display usage data if available
	if len(r.UsageData) > 0 && (r.ReportType == "usage" || r.ReportType == "full") {
		fmt.Fprintln(w, "Usage Data:")

		// Sort usage data by timestamp from oldest to newest
		sort.Slice(r.UsageData, func(i, j int) bool {
			return r.UsageData[i].Timestamp.Before(r.UsageData[j].Timestamp)
		})

		table := tablewriter.NewWriter(tableWriter)
		table.SetHeader([]string{"Service", "Metric", "Value", "Unit", "Timestamp"})
		table.SetBorder(false)
		table.SetAutoWrapText(false)
		table.SetAutoFormatHeaders(true)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetTablePadding("\t")
		table.SetNoWhiteSpace(true)

		for _, usage := range r.UsageData {
			table.Append([]string{
				usage.Service,
				usage.Metric,
				fmt.Sprintf("%.4f", usage.Value),
				usage.Unit,
				usage.Timestamp.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
		fmt.Fprintln(w)
	}

	// Display cost data if available
	if len(r.CostData) > 0 && (r.ReportType == "cost" || r.ReportType == "full") {
		fmt.Fprintln(w, "Cost Data:")

		// Group by account ID
		accountGroups := make(map[string][]providers.CostData)
		accountIDs := []string{}

		for _, cost := range r.CostData {
			if _, exists := accountGroups[cost.AccountID]; !exists {
				accountIDs = append(accountIDs, cost.AccountID)
			}
			accountGroups[cost.AccountID] = append(accountGroups[cost.AccountID], cost)
		}

		// Sort account IDs
		sort.Strings(accountIDs)

		// Generate a summary table first
		summaryTable := tablewriter.NewWriter(tableWriter)
		summaryTable.SetHeader([]string{"Account ID", "Total Cost", "Currency"})
		summaryTable.SetBorder(false)
		summaryTable.SetColumnSeparator(" ")

		var grandTotal float64
		currency := ""

		for _, accountID := range accountIDs {
			costs := accountGroups[accountID]

			// Calculate account totals
			accountTotal := 0.0

			for _, cost := range costs {
				accountTotal += cost.Cost
				currency = cost.Currency
			}

			grandTotal += accountTotal

			summaryTable.Append([]string{
				accountID,
				fmt.Sprintf("%.4f", accountTotal),
				currency,
			})
		}

		// Add grand total
		summaryTable.SetFooter([]string{
			"TOTAL",
			fmt.Sprintf("%.4f", grandTotal),
			currency,
		})
		summaryTable.SetFooterAlignment(tablewriter.ALIGN_LEFT)

		fmt.Fprintln(w, "\nAccount Cost Summary:")
		summaryTable.Render()

		// Now show detailed tables by account
		fmt.Fprintln(w, "\nDetailed Cost Breakdown by Account:")

		for _, accountID := range accountIDs {
			costs := accountGroups[accountID]

			// Group by day
			dayGroups := make(map[string][]providers.CostData)
			days := make([]string, 0)

			for _, cost := range costs {
				day := cost.StartTime.Format("2006-01-02")
				if _, exists := dayGroups[day]; !exists {
					days = append(days, day)
				}
				dayGroups[day] = append(dayGroups[day], cost)
			}

			// Sort days
			sort.Strings(days)

			// Account table
			accountTable := tablewriter.NewWriter(tableWriter)
			accountTable.SetHeader([]string{"Date", "Service", "Cost", "Usage", "Description"})
			accountTable.SetCaption(true, fmt.Sprintf("Account ID: %s", accountID))
			accountTable.SetBorder(false)
			accountTable.SetColumnSeparator(" | ")

			accountTotal := 0.0

			for _, day := range days {
				dayCosts := dayGroups[day]
				dayTotal := 0.0

				for _, cost := range dayCosts {
					accountTable.Append([]string{
						cost.StartTime.Format("2006-01-02"),
						cost.Service,
						fmt.Sprintf("%.4f %s", cost.Cost, cost.Currency),
						fmt.Sprintf("%.2f %s", cost.Quantity, cost.UsageUnit),
						cost.Description,
					})

					dayTotal += cost.Cost
				}

				accountTotal += dayTotal
				accountTable.Append([]string{
					day,
					"DAILY TOTAL",
					fmt.Sprintf("%.4f", dayTotal),
					"",
					"",
				})
			}

			accountTable.SetFooter([]string{
				"",
				"ACCOUNT TOTAL",
				fmt.Sprintf("%.4f", accountTotal),
				"",
				"",
			})

			accountTable.Render()
			fmt.Fprintln(w)
		}
	}

	return nil
}

// writerAdapter adapts an io.Writer to work with tablewriter
type writerAdapter struct {
	w io.Writer
}

// Write implements the io.Writer interface
func (wa *writerAdapter) Write(p []byte) (n int, err error) {
	return wa.w.Write(p)
}
