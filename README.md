# Observability Cost Center

A command-line tool for generating reports on the usage and cost of observability tools. Currently supports AWS CloudWatch and New Relic.

## Features

- Connect to different observability providers (AWS CloudWatch, New Relic)
- Generate usage reports showing metrics consumption
- Generate cost reports with detailed billing information
- View historical data and trends
- Output reports in different formats (table, JSON, CSV)

## Installation

```bash
go install github.com/ilhicas/observability-cost-center@latest
```

## Configuration

Create a config file at `~/.observability-cost-center.yaml`:

```yaml
provider: aws  # or newrelic
output: table  # or json, csv

aws:
  region: us-west-2
  profile: default

newrelic:
  account_id: YOUR_ACCOUNT_ID
```

Or use environment variables:

```bash
export OBS_PROVIDER=aws
export AWS_ACCESS_KEY_ID=your_key
export AWS_SECRET_ACCESS_KEY=your_secret
# For New Relic
export NEW_RELIC_API_KEY=your_api_key
```

## Usage

```bash
# Generate a full report for the last 30 days
observability-cost-center report --provider aws

# Generate a cost-only report for a specific time period
observability-cost-center report --provider newrelic --type cost --start-date 2023-01-01 --end-date 2023-01-31

# Generate a usage-only report and output as JSON
observability-cost-center report --provider aws --type usage --output json
```

## Providers

### AWS CloudWatch

Reports on:
- Metrics ingestion
- Logs ingestion
- Dashboards
- Alarms
- Associated costs

### New Relic

Reports on:
- APM usage
- Infrastructure monitoring
- Log ingestion
- Dashboard usage
- Associated costs

## License

MIT
