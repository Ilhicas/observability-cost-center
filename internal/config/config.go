package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Provider string `mapstructure:"provider"`
	Output   string `mapstructure:"output"`
	AWS      AWSConfig
	NewRelic NewRelicConfig
}

// AWSConfig holds AWS-specific configuration
type AWSConfig struct {
	Region          string `mapstructure:"region"`
	Profile         string `mapstructure:"profile"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
}

// NewRelicConfig holds New Relic-specific configuration
type NewRelicConfig struct {
	APIKey    string `mapstructure:"api_key"`
	AccountID string `mapstructure:"account_id"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	var config Config

	// Set defaults
	viper.SetDefault("output", "table")

	// Read from config file
	configFile := viper.GetString("config")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("error getting user home directory: %w", err)
		}

		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".observability-cost-center")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("OBS")
	viper.AutomaticEnv()

	// Special handling for provider-specific environment variables
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		viper.Set("aws.access_key_id", os.Getenv("AWS_ACCESS_KEY_ID"))
	}
	if os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		viper.Set("aws.secret_access_key", os.Getenv("AWS_SECRET_ACCESS_KEY"))
	}
	if os.Getenv("NEW_RELIC_API_KEY") != "" {
		viper.Set("newrelic.api_key", os.Getenv("NEW_RELIC_API_KEY"))
	}

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	// Unmarshal config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
