package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var configPath string

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `Manage configuration for the observability cost center tool.`,
	}

	generateConfigCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a default configuration file",
		Long:  `Generate a default configuration file with placeholders for AWS CloudWatch and NewRelic providers.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := generateDefaultConfig(configPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating config: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Configuration file generated at: %s\n", configPath)
		},
	}

	generateConfigCmd.Flags().StringVarP(&configPath, "path", "f", "observability-cost-center.yaml", "Path to save the configuration file (default is $HOME/.observability-cost-center.yaml)")

	configCmd.AddCommand(generateConfigCmd)
	rootCmd.AddCommand(configCmd)
}

func generateDefaultConfig(path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}
		path = filepath.Join(home, ".observability-cost-center.yaml")
	}

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("configuration file already exists at %s, use --path to specify a different location or delete the existing file", path)
	}

	defaultConfig := `# Observability Cost Center Configuration

# Default provider (aws or newrelic)
provider: aws

# Output format (table, json, or csv)
output: table

# AWS CloudWatch Provider Configuration
aws:
  # AWS Region (e.g., us-east-1, us-west-2)
  region: us-west-2
  # AWS Profile to use (optional)
  profile: default
  # Alternatively, specify credentials directly (not recommended)
  # access_key_id: YOUR_ACCESS_KEY
  # secret_access_key: YOUR_SECRET_KEY

# NewRelic Provider Configuration
newrelic:
  # Your New Relic Account ID
  account_id: YOUR_ACCOUNT_ID
  # New Relic API Key is recommended to be set via environment variable:
  # export NEW_RELIC_API_KEY=your_api_key
  # Alternatively, specify here (not recommended)
  # api_key: YOUR_API_KEY
`

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory %s: %w", dir, err)
	}

	// Write config file
	if err := os.WriteFile(path, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("could not write configuration file: %w", err)
	}

	return nil
}
