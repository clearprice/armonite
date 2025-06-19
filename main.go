package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configPath string
	version    = "dev" // Will be set by build flags
)

var rootCmd = &cobra.Command{
	Use:   "armonite",
	Short: "A high-performance distributed load testing framework",
	Long:  `Armonite is a headless, open-source load testing framework built for modern infrastructure.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		globalConfig, err = LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger
		_, err = InitLogger(globalConfig.Logging)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}

		SetupStandardLogger()
		return nil
	},
}

var coordinatorCmd = &cobra.Command{
	Use:   "coordinator",
	Short: "Start the coordinator with embedded NATS server",
	RunE:  runCoordinator,
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start an agent that connects to the coordinator",
	RunE:  runAgent,
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
}

var generateConfigCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a default configuration file",
	RunE:  generateConfig,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")
	rootCmd.PersistentFlags().String("log-level", "", "Log level (debug, info, warn, error, fatal)")
	rootCmd.PersistentFlags().String("log-format", "", "Log format (text, json)")
	rootCmd.PersistentFlags().String("output-dir", "", "Output directory for test results")
	rootCmd.PersistentFlags().StringSlice("output-formats", nil, "Output formats (json, csv, xml, yaml)")

	// Coordinator flags
	coordinatorCmd.Flags().String("plan", "", "Path to test plan YAML file")
	coordinatorCmd.Flags().String("host", "", "Server host address")
	coordinatorCmd.Flags().Int("port", 0, "Internal communication port")
	coordinatorCmd.Flags().Int("http-port", 0, "HTTP API port")
	coordinatorCmd.Flags().String("broadcast-interval", "", "Interval for broadcasting test plans")
	coordinatorCmd.Flags().String("telemetry-pull-interval", "", "Interval for pulling telemetry")
	coordinatorCmd.Flags().Int("min-agents", 0, "Minimum number of agents to wait for before starting test")
	coordinatorCmd.Flags().Bool("ui", false, "Enable web UI interface")

	// Agent flags
	agentCmd.Flags().String("master-host", "", "Coordinator host")
	agentCmd.Flags().Int("master-port", 0, "Coordinator port")
	agentCmd.Flags().Int("concurrency", 0, "Number of concurrent requests")
	agentCmd.Flags().Bool("keep-alive", false, "Use HTTP keep-alive")
	agentCmd.Flags().String("region", "", "Agent region identifier")
	agentCmd.Flags().String("id", "", "Agent ID")
	agentCmd.Flags().Bool("dev", false, "Enable development mode (sets sensible resource limits)")
	agentCmd.Flags().Int("rate-limit", 0, "Maximum requests per second (0 = unlimited)")
	agentCmd.Flags().String("default-think-time", "", "Default think time between requests (e.g., '200ms')")

	// Config commands
	configCmd.AddCommand(generateConfigCmd)

	// Version command
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Armonite version %s\n", version)
		},
	}

	rootCmd.AddCommand(coordinatorCmd, agentCmd, configCmd, versionCmd)
}

func generateConfig(cmd *cobra.Command, args []string) error {
	config := CreateDefaultConfig()

	configFile := "armonite.yaml"
	if len(args) > 0 {
		configFile = args[0]
	}

	file, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Generated default configuration in %s\n", configFile)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
