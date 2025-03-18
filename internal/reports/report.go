package reports

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
	"github.com/olekukonko/tablewriter"
)

// Report represents a generated report with usage and cost data
type Report struct {
	ProviderName   string
	StartDate      time.Time
	EndDate        time.Time
	UsageData      []providers.UsageData
	CostData       []providers.CostData
	ReportType     string
	TotalCost      float64
	CustomSections map[string]string
}

func (r *Report) OutputJSON(w io.Writer) error {
	// Create a structured representation of the report for JSON output
	type jsonReport struct {
		Provider       string                 `json:"provider"`
		ReportType     string                 `json:"reportType"`
		StartDate      string                 `json:"startDate"`
		EndDate        string                 `json:"endDate"`
		UsageData      []providers.UsageData  `json:"usageData,omitempty"`
		CostData       []providers.CostData   `json:"costData,omitempty"`
		CustomSections map[string]string      `json:"customSections,omitempty"`
		Summary        map[string]interface{} `json:"summary"`
	}

	// Calculate summary information
	summary := make(map[string]interface{})

	// Count of data entries
	summary["usageDataEntries"] = len(r.UsageData)
	summary["costDataEntries"] = len(r.CostData)

	// Cost summary if cost data is available
	if len(r.CostData) > 0 {
		// Group cost data by account
		accountCosts := make(map[string]float64)
		accountCurrency := make(map[string]string)

		for _, cost := range r.CostData {
			accountCosts[cost.AccountID] += cost.Cost
			accountCurrency[cost.AccountID] = cost.Currency
		}

		// Total cost across all accounts
		var totalCost float64
		primaryCurrency := ""

		accounts := make([]map[string]interface{}, 0, len(accountCosts))
		for accountID, cost := range accountCosts {
			accounts = append(accounts, map[string]interface{}{
				"accountId": accountID,
				"cost":      cost,
				"currency":  accountCurrency[accountID],
			})
			totalCost += cost
			if primaryCurrency == "" {
				primaryCurrency = accountCurrency[accountID]
			}
		}

		summary["accounts"] = accounts
		summary["totalCost"] = totalCost
		summary["currency"] = primaryCurrency
	}

	// Create the JSON report
	report := jsonReport{
		Provider:       r.ProviderName,
		ReportType:     r.ReportType,
		StartDate:      r.StartDate.Format("2006-01-02"),
		EndDate:        r.EndDate.Format("2006-01-02"),
		UsageData:      r.UsageData,
		CostData:       r.CostData,
		CustomSections: r.CustomSections,
		Summary:        summary,
	}

	// Marshal the report to JSON
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ") // Pretty print with 2-space indentation
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("error encoding report to JSON: %w", err)
	}

	return nil
}

func (r *Report) OutputTable(w io.Writer) error {
	// Write report header
	fmt.Fprintf(w, "Report for %s\n", r.ProviderName)
	fmt.Fprintf(w, "Period: %s to %s\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))
	fmt.Fprintf(w, "Report Type: %s\n\n", r.ReportType)

	// Write metrics - only show counts, not empty entries
	fmt.Fprintf(w, "Usage data entries: %d\n", len(r.UsageData))
	fmt.Fprintf(w, "Cost data entries: %d\n\n", len(r.CostData))

	// Write detailed report
	fmt.Fprintf(w, "\n%s Report for %s\n", r.ReportType, r.ProviderName)
	fmt.Fprintf(w, "Period: %s to %s\n\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))

	// Display usage data if available
	if len(r.UsageData) > 0 && (r.ReportType == "usage" || r.ReportType == "full") {
		fmt.Fprintln(w, "Usage Data:")
		fmt.Fprintf(w, "%-14s\t%-20s\t%-9s\t%-5s\t%-20s\n", "SERVICE", "METRIC", "VALUE", "UNIT", "TIMESTAMP")

		for _, usage := range r.UsageData {
			fmt.Fprintf(w, "%-14s\t%-20s\t%-9.4f\t%-5s\t%-20s\n",
				usage.Service,
				usage.Metric,
				usage.Value,
				usage.Unit,
				usage.Timestamp.Format("2006-01-02 15:04:05"))
		}
		fmt.Fprintln(w, "")
	}

	// Display cost data if available
	if len(r.CostData) > 0 && (r.ReportType == "cost" || r.ReportType == "full") {
		fmt.Fprintln(w, "Cost Data:\n")

		// Group cost data by account
		accountGroups := make(map[string][]providers.CostData)
		accountIDs := make([]string, 0)

		for _, cost := range r.CostData {
			if _, exists := accountGroups[cost.AccountID]; !exists {
				accountIDs = append(accountIDs, cost.AccountID)
			}
			accountGroups[cost.AccountID] = append(accountGroups[cost.AccountID], cost)
		}

		// Account cost summary
		fmt.Fprintln(w, "Account Cost Summary:")
		fmt.Fprintf(w, "  %-12s %-12s %-10s\n", "ACCOUNT ID", "TOTAL COST", "CURRENCY")
		fmt.Fprintln(w, "-------------+------------+-----------")

		var totalCost float64
		var currency string

		for _, accountID := range accountIDs {
			costs := accountGroups[accountID]
			accountTotal := 0.0
			for _, cost := range costs {
				accountTotal += cost.Cost
				currency = cost.Currency
			}

			fmt.Fprintf(w, "  %-12s %-12.4f %-10s\n", accountID, accountTotal, currency)
			totalCost += accountTotal
		}

		fmt.Fprintln(w, "-------------+------------+-----------")
		fmt.Fprintf(w, "  %-12s %-12.4f %-10s\n", "TOTAL", totalCost, currency)
		fmt.Fprintln(w, "-------------+------------+-----------")
		fmt.Fprintln(w, "")

		// Detailed cost breakdown by account
		fmt.Fprintln(w, "Detailed Cost Breakdown by Account:")
		fmt.Fprintf(w, "%-13s | %-15s | %-15s | %-12s | %-33s\n",
			"DATE", "SERVICE", "COST", "USAGE", "DESCRIPTION")
		fmt.Fprintln(w, "-------------+---------------+---------------+------------+---------------------------------")

		for _, accountID := range accountIDs {
			costs := accountGroups[accountID]

			// Group costs by day
			dayGroups := make(map[string][]providers.CostData)
			days := make([]string, 0)

			for _, cost := range costs {
				day := cost.StartTime.Format("2006-01-02")
				if _, exists := dayGroups[day]; !exists {
					days = append(days, day)
				}
				dayGroups[day] = append(dayGroups[day], cost)
			}

			// Sort days chronologically
			sort.Strings(days)

			// Track account total
			accountTotal := 0.0

			for _, day := range days {
				costs := dayGroups[day]

				// Daily subtotal
				dailyCost := 0.0

				for _, cost := range costs {
					dailyCost += cost.Cost

					// Split description into multiple lines if too long
					desc := cost.Description
					maxDescLength := 33

					fmt.Fprintf(w, "  %-10s  |  %-14s|  %-14s |  %-10s |  %-33s\n",
						cost.StartTime.Format("2006-01-02"),
						cost.Service,
						fmt.Sprintf("%.4f %s", cost.Cost, cost.Currency),
						fmt.Sprintf("%.2f %s", cost.Quantity, cost.UsageUnit),
						truncateString(desc, maxDescLength))
				}

				fmt.Fprintf(w, "  %-10s  |  %-14s|      %-10.4f |              |                                  \n",
					day, "DAILY TOTAL", dailyCost)
				fmt.Fprintln(w, "-------------+---------------+---------------+------------+---------------------------------")

				accountTotal += dailyCost
			}

			fmt.Fprintf(w, "%-15s |    %-10.4f    |                                               \n",
				"ACCOUNT TOTAL", accountTotal)
			fmt.Fprintln(w, "             ----------------+---------------+------------+---------------------------------")
			fmt.Fprintf(w, "Account ID: %s\n", accountID)
		}
	}

	// Display custom sections if any
	if len(r.CustomSections) > 0 {
		for title, content := range r.CustomSections {
			fmt.Fprintf(w, "\n=== %s ===\n\n", title)
			fmt.Fprintln(w, content)
		}
	}

	return nil
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

// Helper function to truncate strings for better report formatting
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}
