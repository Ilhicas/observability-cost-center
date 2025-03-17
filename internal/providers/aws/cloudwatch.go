package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/ilhicas/observability-cost-center/internal/providers"
	"github.com/spf13/viper"
)

// CloudWatchProvider implements the Provider interface for AWS CloudWatch
type CloudWatchProvider struct {
	client  *cloudwatch.Client
	region  string
	profile string
}

// NewCloudWatchProvider creates a new AWS CloudWatch provider
func NewCloudWatchProvider(config *viper.Viper) (*CloudWatchProvider, error) {
	// Extract region from config
	region := config.GetString("aws.region")
	profile := config.GetString("aws.profile")

	fmt.Printf("Initializing CloudWatch provider with region: %s and profile: %s\n", region, profile)

	// Ensure we explicitly set the region in our AWS config
	var opts []func(*awsconfig.LoadOptions) error

	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	} else {
		return nil, fmt.Errorf("AWS region is not configured in the config file")
	}

	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	// Load AWS config with our specific options
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Verify we have a region set
	if cfg.Region == "" {
		return nil, fmt.Errorf("AWS region is still empty after loading config")
	}

	// Create the provider with the CloudWatch client using our config
	client := cloudwatch.NewFromConfig(cfg)
	return &CloudWatchProvider{
		client:  client,
		region:  cfg.Region,
		profile: profile,
	}, nil
}

// GetName returns the provider name
func (c *CloudWatchProvider) GetName() string {
	return "AWS CloudWatch"
}

// GetUsageData retrieves usage metrics from CloudWatch
func (c *CloudWatchProvider) GetUsageData(start, end time.Time) ([]providers.UsageData, error) {
	// Metrics to query for CloudWatch usage
	metrics := []string{
		"NumberOfMetricsIngested",
		"NumberOfLogsIngested",
		"NumberOfDashboards",
		"NumberOfAlarms",
		"EstimatedBillableSizeBytes",
		"IncomingBytes",
		"IncomingLogEvents",
		"CallCount",
		"ThrottleCount",
		"PutLogEvents.BytesIngested",
		"GetMetricData.DatapointsReturned",
	}

	result := []providers.UsageData{}

	for _, metric := range metrics {
		// Determine the appropriate namespace based on metric
		namespace := "AWS/CloudWatch"
		if metric == "EstimatedBillableSizeBytes" || metric == "IncomingBytes" ||
			metric == "IncomingLogEvents" || metric == "PutLogEvents.BytesIngested" {
			namespace = "AWS/Logs"
		} else if metric == "CallCount" || metric == "ThrottleCount" {
			namespace = "AWS/Usage"
		}

		input := &cloudwatch.GetMetricStatisticsInput{
			Namespace:  stringPtr(namespace),
			MetricName: stringPtr(metric),
			StartTime:  &start,
			EndTime:    &end,
			Period:     int32Ptr(86400), // Daily stats
			Statistics: []types.Statistic{types.StatisticSum},
		}

		// For AWS/Usage metrics, add the Service dimension
		if namespace == "AWS/Usage" {
			input.Dimensions = []types.Dimension{
				{
					Name:  stringPtr("Service"),
					Value: stringPtr("CloudWatch"),
				},
				{
					Name:  stringPtr("Type"),
					Value: stringPtr("API"),
				},
			}
		}

		resp, err := c.client.GetMetricStatistics(context.TODO(), input)
		if err != nil {
			fmt.Printf("Warning: error getting metrics for %s: %v\n", metric, err)
			continue // Skip this metric but continue with others
		}

		for _, datapoint := range resp.Datapoints {
			value := float64(0)
			if datapoint.Sum != nil {
				value = *datapoint.Sum
			}

			// Convert bytes to gigabytes for better readability
			unit := string(datapoint.Unit)
			if unit == "Bytes" || metric == "EstimatedBillableSizeBytes" ||
				metric == "IncomingBytes" || metric == "PutLogEvents.BytesIngested" {
				value = value / (1024 * 1024 * 1024) // Convert bytes to GB
				unit = "GB"
			}

			result = append(result, providers.UsageData{
				Service:   "CloudWatch",
				Metric:    metric,
				Value:     value,
				Unit:      unit,
				Timestamp: *datapoint.Timestamp,
			})
		}
	}

	return result, nil
}

// GetCostData retrieves cost data related to CloudWatch using AWS Cost Explorer API
func (c *CloudWatchProvider) GetCostData(start, end time.Time) ([]providers.CostData, error) {
	// Create Cost Explorer client using the same region and profile
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(c.region),
	}

	// Add profile if it's set
	if c.profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(c.profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for Cost Explorer: %w", err)
	}

	ceClient := costexplorer.NewFromConfig(cfg)

	// Ensure the end date is exclusive
	endDate := end.AddDate(0, 0, 1)

	fmt.Printf("Querying AWS Cost Explorer from %s to %s\n", start.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// Keep the original input with only two GroupBy dimensions (AWS limit)
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &cetypes.DateInterval{
			Start: stringPtr(start.Format("2006-01-02")),
			End:   stringPtr(endDate.Format("2006-01-02")),
		},
		Granularity: cetypes.GranularityDaily,
		Metrics:     []string{"UnblendedCost", "UsageQuantity"},
		GroupBy: []cetypes.GroupDefinition{
			{
				Type: cetypes.GroupDefinitionTypeDimension,
				Key:  stringPtr("SERVICE"),
			},
			{
				Type: cetypes.GroupDefinitionTypeDimension,
				Key:  stringPtr("LINKED_ACCOUNT"),
			},
		},
		Filter: &cetypes.Expression{
			Or: []cetypes.Expression{
				{
					Dimensions: &cetypes.DimensionValues{
						Key:    cetypes.DimensionService,
						Values: []string{"AmazonCloudWatch", "CloudWatch"},
					},
				},
				{
					Dimensions: &cetypes.DimensionValues{
						Key:    cetypes.DimensionService,
						Values: []string{"AmazonCloudWatchLogs", "CloudWatchLogs"},
					},
				},
				{
					Dimensions: &cetypes.DimensionValues{
						Key:    cetypes.DimensionService,
						Values: []string{"AmazonCloudWatchMetrics", "CloudWatchMetrics"},
					},
				},
			},
		},
	}

	// Execute the query
	resp, err := ceClient.GetCostAndUsage(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("error getting cost data from AWS Cost Explorer: %w", err)
	}

	fmt.Printf("Cost Explorer returned %d result periods\n", len(resp.ResultsByTime))

	// Process the results
	var results []providers.CostData

	for _, resultByTime := range resp.ResultsByTime {
		periodStart, _ := time.Parse("2006-01-02", *resultByTime.TimePeriod.Start)
		periodEnd, _ := time.Parse("2006-01-02", *resultByTime.TimePeriod.End)

		// Check for totals
		if resultByTime.Total != nil && resultByTime.Total["UnblendedCost"].Amount != nil {
			fmt.Printf("Total cost for period %s to %s: %s %s\n",
				*resultByTime.TimePeriod.Start,
				*resultByTime.TimePeriod.End,
				*resultByTime.Total["UnblendedCost"].Amount,
				*resultByTime.Total["UnblendedCost"].Unit)
		}

		// Process each service group
		for _, group := range resultByTime.Groups {
			// Skip if we don't have at least two keys (service name and account)
			if len(group.Keys) < 2 {
				continue
			}

			serviceName := group.Keys[0]
			accountId := group.Keys[1]

			// Parse cost amount
			cost := 0.0
			if group.Metrics["UnblendedCost"].Amount != nil {
				if parsedCost, err := strconv.ParseFloat(*group.Metrics["UnblendedCost"].Amount, 64); err == nil {
					cost = parsedCost
				}
			}

			// Get currency
			currency := "USD"
			if group.Metrics["UnblendedCost"].Unit != nil {
				currency = *group.Metrics["UnblendedCost"].Unit
			}

			// Parse usage quantity
			usage := 0.0
			if group.Metrics["UsageQuantity"].Amount != nil {
				if parsedUsage, err := strconv.ParseFloat(*group.Metrics["UsageQuantity"].Amount, 64); err == nil {
					usage = parsedUsage
				}
			}

			// Determine usage unit and description based on the service name
			usageUnit, description := inferUsageInfo(serviceName, usage)

			// Always include the entry, even with zero cost
			fmt.Printf("CloudWatch service: %s, Account: %s, Date: %s, Cost: %.6f %s, Usage: %.2f %s (%s)\n",
				serviceName, accountId, periodStart.Format("2006-01-02"), cost, currency, usage, usageUnit, description)

			results = append(results, providers.CostData{
				Service:     serviceName,
				ItemName:    fmt.Sprintf("Account: %s", accountId),
				Cost:        cost,
				Currency:    currency,
				Quantity:    usage,
				UsageUnit:   usageUnit,
				Period:      "Daily",
				StartTime:   periodStart,
				EndTime:     periodEnd,
				AccountID:   accountId,
				Description: description,
				Region:      c.region,
			})
		}
	}

	// If we still have no results, try without the filter as a fallback
	if len(results) == 0 {
		fmt.Println("No CloudWatch services found with filters, trying without filters...")

		// Remove the filter and try again
		input.Filter = nil

		resp, err = ceClient.GetCostAndUsage(context.TODO(), input)
		if err == nil {
			// Now search for CloudWatch in the results
			cloudWatchKeywords := []string{"cloudwatch", "logs", "metrics"}

			for _, resultByTime := range resp.ResultsByTime {
				periodStart, _ := time.Parse("2006-01-02", *resultByTime.TimePeriod.Start)
				periodEnd, _ := time.Parse("2006-01-02", *resultByTime.TimePeriod.End)

				for _, group := range resultByTime.Groups {
					if len(group.Keys) < 2 {
						continue
					}

					serviceName := group.Keys[0]
					accountId := group.Keys[1]

					// Check if this is a CloudWatch related service
					isCloudWatchService := false
					serviceNameLower := strings.ToLower(serviceName)
					for _, keyword := range cloudWatchKeywords {
						if strings.Contains(serviceNameLower, keyword) {
							isCloudWatchService = true
							break
						}
					}

					if !isCloudWatchService {
						continue
					}

					// Process similarly as above
					cost := 0.0
					if group.Metrics["UnblendedCost"].Amount != nil {
						if parsedCost, err := strconv.ParseFloat(*group.Metrics["UnblendedCost"].Amount, 64); err == nil {
							cost = parsedCost
						}
					}

					currency := "USD"
					if group.Metrics["UnblendedCost"].Unit != nil {
						currency = *group.Metrics["UnblendedCost"].Unit
					}

					usage := 0.0
					if group.Metrics["UsageQuantity"].Amount != nil {
						if parsedUsage, err := strconv.ParseFloat(*group.Metrics["UsageQuantity"].Amount, 64); err == nil {
							usage = parsedUsage
						}
					}

					fmt.Printf("Found CloudWatch service: %s, Account: %s, Cost: %.6f %s, Usage: %.2f\n",
						serviceName, accountId, cost, currency, usage)

					results = append(results, providers.CostData{
						Service:   serviceName,
						ItemName:  fmt.Sprintf("Account: %s", accountId),
						Cost:      cost,
						Currency:  currency,
						Quantity:  usage,
						Period:    "Daily",
						StartTime: periodStart,
						EndTime:   periodEnd,
					})
				}
			}
		}
	}

	// If no results were found, provide a helpful message
	if len(results) == 0 {
		return nil, fmt.Errorf("no CloudWatch cost data found for the specified period. Check your AWS account has Cost Explorer enabled and data available. Try with a longer date range as there might be a delay in data")
	}

	return results, nil
}

// inferUsageInfo infers usage type information based on the service name
func inferUsageInfo(serviceName string, usage float64) (string, string) {
	// Default values
	unit := "Count"
	description := "Standard usage"

	// Try to infer usage type from service name
	switch {
	case strings.Contains(strings.ToLower(serviceName), "logs"):
		if usage > 1000 {
			unit = "Events"
			description = "Log Events"
		} else {
			unit = "GB"
			description = "Log Data"
		}
	case strings.Contains(strings.ToLower(serviceName), "metric"):
		unit = "MetricMonths"
		description = "Metrics Monitored"
	case strings.Contains(strings.ToLower(serviceName), "dashboard"):
		unit = "DashboardMonths"
		description = "Dashboard Usage"
	case strings.Contains(strings.ToLower(serviceName), "alarm"):
		unit = "AlarmMonths"
		description = "Alarm Monitoring"
	case strings.Contains(strings.ToLower(serviceName), "api"):
		unit = "API-Requests"
		description = "API Calls"
	case usage > 1000000:
		unit = "Count"
		description = "High Volume Events"
	case usage > 1000:
		unit = "Count"
		description = "Medium Volume Events"
	case usage > 1:
		unit = "GB"
		description = "Data Processing"
	default:
		unit = "Units"
		description = "Standard Usage"
	}

	return unit, description
}

// determineUsageTypeInfo extracts information about the usage type
func determineUsageTypeInfo(usageType string) (string, string) {
	// Default values
	unit := "Count"
	description := "Standard usage"

	// Process CloudWatch usage types
	switch {
	case strings.Contains(usageType, "DataIngestion"):
		unit = "GB"
		description = "Data Ingestion"
	case strings.Contains(usageType, "DataStorage"):
		unit = "GB-Month"
		description = "Data Storage"
	case strings.Contains(usageType, "Metrics"):
		unit = "MetricMonitorHours"
		description = "Metrics Monitored"
	case strings.Contains(usageType, "Dashboard"):
		unit = "DashboardMonthHours"
		description = "Dashboard Usage"
	case strings.Contains(usageType, "Alarm"):
		unit = "AlarmMonitorHours"
		description = "Alarm Monitoring"
	case strings.Contains(usageType, "Requests"):
		unit = "API-Requests"
		description = "API Requests"
	case strings.Contains(usageType, "Log"):
		unit = "GB"
		description = "Log Processing"
	case strings.Contains(usageType, "Events"):
		unit = "Events"
		description = "Event Processing"
	case strings.Contains(usageType, "TimedStorage"):
		unit = "GB-Month"
		description = "Log Storage"
	case strings.Contains(usageType, "DataScanned"):
		unit = "GB"
		description = "Data Scanned"
	}

	return unit, description
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
