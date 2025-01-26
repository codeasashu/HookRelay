package config

import (
	"bytes"
	"errors"
	"log"

	"github.com/spf13/viper"
)

type WorkerType string

const (
	QueueWorker  WorkerType = "queue"
	PubSubWorker WorkerType = "pubsub"
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
queue_size = 200000  # divided equally b/w worker queue and result queue

[metrics]
enabled = true
worker_addr = ":2112"  # worker metrics address

[logging]
log_level = "info"  # possible values: "debug", "info", "warn", "error" (default=info)
log_format = "json"  # possible values: "json", "console" (default=json)

[queue_worker]
addr = "127.0.0.1:6379"
db = 0

[pubsub_worker]
addr = "127.0.0.1:6379"
db = 0
channel = "hookrelay:pubsub"
threads = 100
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
	QueueSize            int `mapstructure:"queue_size"`
}

type QueueWorkerConfig struct {
	Addr     string `mapstructure:"addr"` // in milliseconds
	Db       int    `mapstructure:"db"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type PubsubWorkerConfig struct {
	Addr      string `mapstructure:"addr"` // in milliseconds
	Db        int    `mapstructure:"db"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	Channel   string `mapstructure:"channel"`
	Threads   int    `mapstructure:"threads"`
	QueueSize int    `mapstructure:"queue_size"`
}

type MetricsConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WorkerAddr string `mapstructure:"worker_addr"`
}

type LoggingConfig struct {
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
}

type Config struct {
	Listener     ListenerConfig     `mapstructure:"listener"`
	Api          ApiConfig          `mapstructure:"api"`
	Worker       WorkerConfig       `mapstructure:"worker"`
	Metrics      MetricsConfig      `mapstructure:"metrics"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	QueueWorker  QueueWorkerConfig  `mapstructure:"queue_worker"`
	PubsubWorker PubsubWorkerConfig `mapstructure:"pubsub_worker"`

	IsWorker   bool
	WorkerType WorkerType
}

var HRConfig Config

func (c *Config) IsQueueWorker() bool {
	return c.WorkerType == QueueWorker
}

func (c *Config) IsPubsubWorker() bool {
	return c.WorkerType == PubSubWorker
}

func LoadConfig(customConfigPath string, workerType string) (*Config, error) {
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

	HRConfig.IsWorker = false
	if workerType != "" {
		HRConfig.IsWorker = true
		HRConfig.WorkerType = WorkerType(workerType)
	}

	return &HRConfig, nil
}
