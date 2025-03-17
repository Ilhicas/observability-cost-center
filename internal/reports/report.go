package reports

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
	"github.com/olekukonko/tablewriter"
)

// Report represents a generated report with usage and cost data
type Report struct {
	ProviderName string
	StartDate    time.Time
	EndDate      time.Time
	UsageData    []providers.UsageData
	CostData     []providers.CostData
	ReportType   string
	TotalCost    float64
}

// ReportGenerator is responsible for generating reports from provider data
type ReportGenerator struct {
	provider providers.Provider
}

// NewReportGenerator creates a new report generator for a given provider
func NewReportGenerator(provider providers.Provider) *ReportGenerator {
	return &ReportGenerator{
		provider: provider,
	}
}

// outputAsTable formats and displays the report as ASCII tables
func (r *Report) outputAsTableV1() error {
	fmt.Printf("\n%s Report for %s\n", r.ReportType, r.ProviderName)
	fmt.Printf("Period: %s to %s\n\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))

	// Display usage data if available
	if len(r.UsageData) > 0 && (r.ReportType == "usage" || r.ReportType == "full") {
		fmt.Println("Usage Data:")

		// Sort usage data by timestamp from oldest to newest
		sort.Slice(r.UsageData, func(i, j int) bool {
			return r.UsageData[i].Timestamp.Before(r.UsageData[j].Timestamp)
		})

		table := tablewriter.NewWriter(os.Stdout)
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
		fmt.Println()
	}

	// Display cost data if available
	if len(r.CostData) > 0 && (r.ReportType == "cost" || r.ReportType == "full") {
		fmt.Println("Cost Data:")

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
		summaryTable := tablewriter.NewWriter(os.Stdout)
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

		fmt.Println("\nAccount Cost Summary:")
		summaryTable.Render()

		// Now show detailed tables by account
		fmt.Println("\nDetailed Cost Breakdown by Account:")

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
			accountTable := tablewriter.NewWriter(os.Stdout)
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
			fmt.Println()
		}
	}

	return nil
}

// outputAsJSON outputs the report in JSON format
func (r *Report) outputAsJSON() error {
	// Implementation would go here
	fmt.Println("JSON output not yet implemented")
	return nil
}

// outputAsCSV outputs the report in CSV format
func (r *Report) outputAsCSV() error {
	// Implementation would go here
	fmt.Println("CSV output not yet implemented")
	return nil
}
