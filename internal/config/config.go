package config

import (
	"bytes"
	"errors"
	"log"

	"github.com/spf13/viper"
)

const (
	DefaultConfigName     = "hookrelay"
	DefaultConfigDir      = "."
	defaultConfigTemplate = `# Configuration file for HookRelay
[listener]
http.port = 8082
http.queue_size = 1024
http.workers = 4

[api]
port = 8081

[worker]
scan_duration = 100
min_threads = 1
max_threads = -1
result_handlers_threads = 10

[metrics]
enabled = true

[logging]
log_level = "info"  # possible values: "debug", "info", "warn", "error" (default=info)
log_format = "json"  # possible values: "json", "console" (default=json)
`
)

type HttpListenerConfig struct {
	Port      int `mapstructure:"port"`
	QueueSize int `mapstructure:"queue_size"`
	Workers   int `mapstructure:"workers"`
}

type ListenerConfig struct {
	Http HttpListenerConfig `mapstructure:"http"`
}

type ApiConfig struct {
	Port int `mapstructure:"port"`
}

type WorkerConfig struct {
	ScanDuration         int `mapstructure:"scan_duration"` // in milliseconds
	MinThreads           int `mapstructure:"min_threads"`
	MaxThreads           int `mapstructure:"max_threads"`
	ResultHandlerThreads int `mapstructure:"result_handlers_threads"`
}

type MetricsConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type LoggingConfig struct {
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
}

type Config struct {
	Listener ListenerConfig `mapstructure:"listener"`
	Api      ApiConfig      `mapstructure:"api"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

var HRConfig Config

func LoadConfig(customConfigPath string) (*Config, error) {
	v := viper.New()

	// Set default configuration
	v.SetConfigType("toml")
	if err := v.ReadConfig(bytes.NewBuffer([]byte(defaultConfigTemplate))); err != nil {
		log.Printf("failed to load default configuration: %s", err)
		return nil, err
	}

	// Configure Viper to look for the configuration file
	if customConfigPath != "" {
		v.SetConfigFile(customConfigPath)
		log.Printf("Using custom configuration file: %s\n", customConfigPath)
	}

	// Attempt to read the configuration file
	if err := v.MergeInConfig(); err != nil {
		// If the file is not found and no custom file is specified, log a warning and continue with defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); ok && customConfigPath == "" {
			log.Println("Configuration file not found, using default configuration")
		} else {
			log.Printf("error reading configuration file: %s", err)
			return nil, errors.New("error reading configuration file")
		}
	}

	// Unmarshal the configuration into the Config struct
	if err := v.Unmarshal(&HRConfig); err != nil {
		log.Printf("error unmarshaling configuration: %s", err)
		return nil, errors.New("error unmarshaling configuration")
	}

	return &HRConfig, nil
}
