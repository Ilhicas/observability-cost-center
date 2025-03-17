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
	client *cloudwatch.Client
	region string
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
		client: client,
		region: cfg.Region,
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
	// Create Cost Explorer client using the same region
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(c.region),
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for Cost Explorer: %w", err)
	}

	ceClient := costexplorer.NewFromConfig(cfg)

	// Ensure the end date is exclusive
	endDate := end.AddDate(0, 0, 1)

	fmt.Printf("Querying AWS Cost Explorer from %s to %s\n", start.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// Query with service dimension filter to focus on CloudWatch services
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

			// Always include the entry, even with zero cost
			fmt.Printf("CloudWatch service: %s, Account: %s, Cost: %.6f %s, Usage: %.2f\n",
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

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
