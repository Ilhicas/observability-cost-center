package reports

import (
	"fmt"
	"os"
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
func (r *Report) outputAsTable() error {
	fmt.Printf("\n%s Report for %s\n", r.ReportType, r.ProviderName)
	fmt.Printf("Period: %s to %s\n\n", r.StartDate.Format("2006-01-02"), r.EndDate.Format("2006-01-02"))

	// Display usage data if available
	if len(r.UsageData) > 0 && (r.ReportType == "usage" || r.ReportType == "full") {
		fmt.Println("Usage Data:")
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
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Service", "Item", "Cost", "Currency", "Quantity", "Period", "Start Date", "End Date"})
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

		for _, cost := range r.CostData {
			table.Append([]string{
				cost.Service,
				cost.ItemName,
				fmt.Sprintf("%.6f", cost.Cost),
				cost.Currency,
				fmt.Sprintf("%.2f", cost.Quantity),
				cost.Period,
				cost.StartTime.Format("2006-01-02"),
				cost.EndTime.Format("2006-01-02"),
			})
		}
		table.Render()
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
