package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`
	Output   OutputConfig   `yaml:"output" json:"output"`
	Defaults DefaultsConfig `yaml:"defaults" json:"defaults"`
}

type ServerConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`           // Internal NATS port
	HTTPPort int    `yaml:"http_port" json:"http_port"` // Public HTTP API port
	EnableUI bool   `yaml:"enable_ui" json:"enable_ui"` // Enable web UI
}

type DatabaseConfig struct {
	DSN         string `yaml:"dsn" json:"dsn"`                   // Database connection string
	MaxOpen     int    `yaml:"max_open" json:"max_open"`         // Maximum open connections
	MaxIdle     int    `yaml:"max_idle" json:"max_idle"`         // Maximum idle connections
	MaxLifetime string `yaml:"max_lifetime" json:"max_lifetime"` // Connection maximum lifetime
}

type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	File   string `yaml:"file" json:"file"`
}

type OutputConfig struct {
	Directory string   `yaml:"directory" json:"directory"`
	Formats   []string `yaml:"formats" json:"formats"`
	Filename  string   `yaml:"filename" json:"filename"`
}

type DefaultsConfig struct {
	Concurrency       int    `yaml:"concurrency" json:"concurrency"`
	Duration          string `yaml:"duration" json:"duration"`
	BroadcastInterval string `yaml:"broadcast_interval" json:"broadcast_interval"`
	TelemetryInterval string `yaml:"telemetry_interval" json:"telemetry_interval"`
	KeepAlive         bool   `yaml:"keep_alive" json:"keep_alive"`
	MinAgents         int    `yaml:"min_agents" json:"min_agents"`
}

var globalConfig *Config

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Host:     "0.0.0.0",
			Port:     4222,
			HTTPPort: 8080,
		},
		Database: DatabaseConfig{
			DSN:         "./armonite.db",
			MaxOpen:     25,
			MaxIdle:     5,
			MaxLifetime: "1h",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   "",
		},
		Output: OutputConfig{
			Directory: "./results",
			Formats:   []string{"json"},
			Filename:  "armonite-results",
		},
		Defaults: DefaultsConfig{
			Concurrency:       100,
			Duration:          "1m",
			BroadcastInterval: "5s",
			TelemetryInterval: "5s",
			KeepAlive:         true,
			MinAgents:         1,
		},
	}

	if configPath == "" {
		// Try to find config file in common locations
		possiblePaths := []string{
			"armonite.yaml",
			"armonite.yml",
			"config/armonite.yaml",
			"config/armonite.yml",
			filepath.Join(os.Getenv("HOME"), ".armonite.yaml"),
			filepath.Join(os.Getenv("HOME"), ".armonite.yml"),
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
	}

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}

		log.Printf("Loaded configuration from %s", configPath)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	globalConfig = config
	return config, nil
}

func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate logging level
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLevels, strings.ToLower(c.Logging.Level)) {
		return fmt.Errorf("invalid log level: %s. Valid levels: %s",
			c.Logging.Level, strings.Join(validLevels, ", "))
	}

	// Validate logging format
	validFormats := []string{"text", "json"}
	if !contains(validFormats, strings.ToLower(c.Logging.Format)) {
		return fmt.Errorf("invalid log format: %s. Valid formats: %s",
			c.Logging.Format, strings.Join(validFormats, ", "))
	}

	// Validate output formats
	validOutputFormats := []string{"json", "csv", "xml", "yaml"}
	for _, format := range c.Output.Formats {
		if !contains(validOutputFormats, strings.ToLower(format)) {
			return fmt.Errorf("invalid output format: %s. Valid formats: %s",
				format, strings.Join(validOutputFormats, ", "))
		}
	}

	// Validate defaults
	if c.Defaults.Concurrency < 1 {
		return fmt.Errorf("invalid default concurrency: %d", c.Defaults.Concurrency)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(c.Output.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", c.Output.Directory, err)
	}

	return nil
}

func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func (c *Config) GetNATSURL() string {
	return fmt.Sprintf("nats://%s:%d", c.Server.Host, c.Server.Port)
}

func GetConfig() *Config {
	if globalConfig == nil {
		config, err := LoadConfig("")
		if err != nil {
			log.Printf("Failed to load config, using defaults: %v", err)
			return &Config{
				Server:   ServerConfig{Host: "0.0.0.0", Port: 4222},
				Database: DatabaseConfig{DSN: "./armonite.db", MaxOpen: 25, MaxIdle: 5, MaxLifetime: "1h"},
				Logging:  LoggingConfig{Level: "info", Format: "text"},
				Output:   OutputConfig{Directory: "./results", Formats: []string{"json"}, Filename: "armonite-results"},
				Defaults: DefaultsConfig{Concurrency: 100, Duration: "1m", BroadcastInterval: "5s", TelemetryInterval: "5s", KeepAlive: true},
			}
		}
		globalConfig = config
	}
	return globalConfig
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func CreateDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 4222,
		},
		Database: DatabaseConfig{
			DSN:         "./armonite.db",
			MaxOpen:     25,
			MaxIdle:     5,
			MaxLifetime: "1h",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   "",
		},
		Output: OutputConfig{
			Directory: "./results",
			Formats:   []string{"json", "csv"},
			Filename:  "armonite-results",
		},
		Defaults: DefaultsConfig{
			Concurrency:       100,
			Duration:          "1m",
			BroadcastInterval: "5s",
			TelemetryInterval: "5s",
			KeepAlive:         true,
		},
	}
}
