package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
	"github.com/ilhicas/observability-cost-center/internal/providers/newrelic"
	"github.com/ilhicas/observability-cost-center/internal/reports"
)

// GenerateReport generates a report based on the given parameters
func GenerateReport(providerName, reportType, startDate, endDate, outputPath, format string, includeLicenseDetails bool, inactiveDays int) error {
	// Get the provider
	provider, err := providers.GetProvider(providerName)
	if err != nil {
		return fmt.Errorf("error getting provider: %w", err)
	}

	// Parse dates
	start, end, err := parseDates(startDate, endDate)
	if err != nil {
		return fmt.Errorf("error parsing dates: %w", err)
	}

	// Create report generator
	rg := reports.NewReportGenerator(provider)

	// Generate the report
	report, err := rg.GenerateReport(reports.ReportType(reportType), start, end)
	if err != nil {
		return fmt.Errorf("error generating report: %w", err)
	}

	// For New Relic, always add detailed license information
	if provider.GetName() == "newrelic" {
		// Cast to New Relic provider to access license report methods
		if nrProvider, ok := provider.(*newrelic.NewRelicProvider); ok {
			licenseReport, err := nrProvider.GetLicenseUsageReport(inactiveDays)
			if err != nil {
				return fmt.Errorf("error generating license details: %w", err)
			}

			// Add license report to the main report
			report.AppendCustomSection("License Usage Details", licenseReport)
		}
	}

	// Output the report
	if outputPath == "" {
		// Output to stdout
		if format == "json" {
			err = report.OutputJSON(os.Stdout)
		} else {
			err = report.OutputTable(os.Stdout)
		}
	} else {
		// Output to file
		file, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("error creating output file: %w", err)
		}
		defer file.Close()

		if format == "json" {
			err = report.OutputJSON(file)
		} else {
			err = report.OutputTable(file)
		}
	}

	if err != nil {
		return fmt.Errorf("error outputting report: %w", err)
	}

	return nil
}

// parseDates parses date strings into time.Time values
func parseDates(startDate, endDate string) (time.Time, time.Time, error) {
	var start, end time.Time
	var err error

	// Parse dates or use defaults
	if startDate != "" {
		start, err = time.Parse("2006-01-02", startDate)
		if err != nil {
			return start, end, fmt.Errorf("invalid start date format: %w", err)
		}
	} else {
		// Default to 30 days ago
		start = time.Now().AddDate(0, 0, -30)
	}

	if endDate != "" {
		end, err = time.Parse("2006-01-02", endDate)
		if err != nil {
			return start, end, fmt.Errorf("invalid end date format: %w", err)
		}
	} else {
		// Default to today
		end = time.Now()
	}

	return start, end, nil
}
