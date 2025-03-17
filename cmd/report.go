package cmd

import (
	"fmt"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
	"github.com/ilhicas/observability-cost-center/internal/providers/aws"
	"github.com/ilhicas/observability-cost-center/internal/providers/newrelic"
	"github.com/ilhicas/observability-cost-center/internal/reports"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	startDate  string
	endDate    string
	reportType string
	outputFile string
)

func init() {
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate reports on observability tool usage and cost",
		Long:  `Generate detailed reports on observability tool usage and cost with historical data from providers.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := executeReport(cmd, args)
			if err != nil {
				fmt.Printf("Error executing report: %v\n", err)
			}
		},
	}

	reportCmd.Flags().StringVar(&startDate, "start-date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"), "Start date for the report (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&endDate, "end-date", time.Now().Format("2006-01-02"), "End date for the report (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&reportType, "type", "full", "Report type: usage, cost, or full")
	reportCmd.Flags().StringVar(&outputFile, "output-file", "", "Output file path. If not provided, outputs to stdout")

	// Bind the flags to viper
	viper.BindPFlag("output.file", reportCmd.Flags().Lookup("output-file"))

	rootCmd.AddCommand(reportCmd)
}

func executeReport(cmd *cobra.Command, args []string) error {
	// Debug information about config file
	fmt.Printf("Using configuration file: %s\n", viper.ConfigFileUsed())

	// Print AWS region from config
	fmt.Printf("AWS region from config: %s\n", viper.GetString("aws.region"))

	provider := viper.GetString("provider")
	if provider == "" {
		return fmt.Errorf("provider is required. Use --provider flag or set in config")
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("error parsing start date: %w", err)
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return fmt.Errorf("error parsing end date: %w", err)
	}

	var costProvider providers.Provider

	// Initialize the appropriate provider
	if provider == "aws" {
		// Make sure we're passing the viper config to our provider
		awsProvider, err := aws.NewCloudWatchProvider(viper.GetViper())
		if err != nil {
			return fmt.Errorf("error initializing AWS CloudWatch provider: %w", err)
		}
		costProvider = awsProvider
	} else if provider == "newrelic" {
		p, err := newrelic.NewProvider()
		if err != nil {
			return fmt.Errorf("error initializing NewRelic provider: %w", err)
		}
		costProvider = p
	} else {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	generator := reports.NewReportGenerator(costProvider)
	var report *reports.Report
	var reportTypeEnum reports.ReportType

	switch reportType {
	case "usage":
		reportTypeEnum = reports.UsageReport
	case "cost":
		reportTypeEnum = reports.CostReport
	case "full":
		reportTypeEnum = reports.FullReport
	default:
		return fmt.Errorf("unsupported report type: %s", reportType)
	}

	report, err = generator.GenerateReport(reportTypeEnum, start, end)
	if err != nil {
		return fmt.Errorf("error generating report: %w", err)
	}

	// Add debug information to help diagnose issues
	fmt.Printf("Generated report with %d usage data entries and %d cost data entries\n",
		len(report.UsageData), len(report.CostData))

	// Get output format from config or default to table
	outputFormat := viper.GetString("output.format")
	if outputFormat == "" {
		outputFormat = "table"
	}
	fmt.Printf("Using output format: %s\n", outputFormat)

	// Get output file path from command line flag or config
	outputFilePath := outputFile
	if outputFilePath == "" {
		outputFilePath = viper.GetString("output.file")
	}
	if outputFilePath != "" {
		fmt.Printf("Writing output to file: %s\n", outputFilePath)
	} else {
		fmt.Println("Writing output to stdout")
	}

	err = report.Output(outputFormat, outputFilePath)
	if err != nil {
		return fmt.Errorf("error outputting report: %w", err)
	}

	return nil
}
