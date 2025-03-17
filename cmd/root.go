package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "observability-cost-center",
	Short: "Generate reports on observability tools usage and cost",
	Long: `A CLI tool that connects to observability providers like AWS CloudWatch and NewRelic
to generate detailed reports on usage and cost with historical data.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "observability-cost-center.yaml", "config file (default is $HOME/.observability-cost-center.yaml)")
	rootCmd.PersistentFlags().StringP("provider", "p", "", "Provider to use (aws, newrelic)")
	rootCmd.PersistentFlags().StringP("output", "o", "table", "Output format (json, csv, table)")

	viper.BindPFlag("provider", rootCmd.PersistentFlags().Lookup("provider"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("Error finding home directory:", err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.AddConfigPath(".") // Also look in current directory
		viper.SetConfigType("yaml")
		viper.SetConfigName(".observability-cost-center")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		fmt.Println("Warning: Could not read config file:", err)
	}

	// Debug output to verify loaded configuration
	fmt.Println("AWS Region from config:", viper.GetString("aws.region"))
}
