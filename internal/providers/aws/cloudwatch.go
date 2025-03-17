package aws

import (
	"context"
	"fmt"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
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
	}

	result := []providers.UsageData{}

	for _, metric := range metrics {
		input := &cloudwatch.GetMetricStatisticsInput{
			Namespace:  stringPtr("AWS/CloudWatch"),
			MetricName: stringPtr(metric),
			StartTime:  &start,
			EndTime:    &end,
			Period:     int32Ptr(86400), // Daily stats
			Statistics: []types.Statistic{types.StatisticSum},
		}

		resp, err := c.client.GetMetricStatistics(context.TODO(), input)
		if err != nil {
			return nil, fmt.Errorf("error getting metrics for %s: %w", metric, err)
		}

		for _, datapoint := range resp.Datapoints {
			result = append(result, providers.UsageData{
				Service:   "CloudWatch",
				Metric:    metric,
				Value:     *datapoint.Sum,
				Unit:      string(datapoint.Unit),
				Timestamp: *datapoint.Timestamp,
			})
		}
	}

	return result, nil
}

// GetCostData retrieves cost data related to CloudWatch
// Note: This would typically use AWS Cost Explorer API, but we're simplifying here
func (c *CloudWatchProvider) GetCostData(start, end time.Time) ([]providers.CostData, error) {
	// In a real implementation, this would query the AWS Cost Explorer API
	// Here we're returning sample data for illustration

	return []providers.CostData{
		{
			Service:   "CloudWatch",
			ItemName:  "MetricStorage",
			Cost:      123.45,
			Currency:  "USD",
			Period:    "Monthly",
			StartTime: start,
			EndTime:   end,
		},
		{
			Service:   "CloudWatch",
			ItemName:  "LogsIngestion",
			Cost:      234.56,
			Currency:  "USD",
			Period:    "Monthly",
			StartTime: start,
			EndTime:   end,
		},
		{
			Service:   "CloudWatch",
			ItemName:  "Dashboards",
			Cost:      45.67,
			Currency:  "USD",
			Period:    "Monthly",
			StartTime: start,
			EndTime:   end,
		},
	}, nil
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
