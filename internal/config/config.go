package config

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/url"

	"github.com/spf13/viper"
)

const (
	DefaultConfigName     = "hookrelay"
	DefaultConfigDir      = "."
	TraceIDHeaderName     = "X-Hookrelay-Trace-Id"
	defaultConfigTemplate = `# Configuration file for HookRelay
[listener]
http.queue_size = 1024
http.workers = 4

[api]
addr = ":8081"

[metrics]
enabled = true
worker_addr = ":2112"  # worker metrics address

[logging]
log_level = "info"  # possible values: "debug", "info", "warn", "error" (default=info)
log_format = "json"  # possible values: "json", "console" (default=json)

[local_worker]
scan_duration = 100
min_threads = 1
max_threads = -1
result_handlers_threads = 10
queue_size = 200000  # divided equally b/w worker queue and result queue

[queue_worker]
addr = "127.0.0.1:6379"
db = 0
concurrency = 10

[target]
max_retries = 2

# postgres DB (Uncomment this to enable postgreSQL)
# [database]
# scheme = "postgres"
# host = "localhost"
# username = "admin"
# password = "admin"
# database = "hookrelay"
# options = "tls-insecure-skip-verify=false&connect_timeout=30"
# port = 5432

[wal]
path = "/tmp/hookrelay.wal"
format = "20060102_1504"  # second level timestamp 

[subscription]
db.scheme = "mysql"
db.host = "localhost"
db.username = "admin"
db.password = "admin"
db.database = "hookrelay"
db.options = "tls-insecure-skip-verify=false&connect_timeout=30"
db.port = 3306

[delivery]
db.scheme = "mysql"
db.host = "localhost"
db.username = "admin"
db.password = "admin"
db.database = "hookrelay"
db.options = "tls-insecure-skip-verify=false&connect_timeout=30"
db.port = 3306

[aws]
aws_access_key_id = ""
aws_secret_access_key = ""
aws_region = "ap-south-1"
`
)

type DatabaseProvider string

const (
	PostgresDatabaseProvider DatabaseProvider = "postgres"
	MySQLDatabaseProvider    DatabaseProvider = "mysql"
)

type DatabaseConfig struct {
	Type DatabaseProvider `json:"type" mapstructure:"type"`

	Scheme   string `json:"scheme" mapstructure:"scheme"`
	Host     string `json:"host" mapstructure:"host"`
	Username string `json:"username" mapstructure:"username"`
	Password string `json:"password" mapstructure:"password"`
	Database string `json:"database" mapstructure:"database"`
	Options  string `json:"options" mapstructure:"options"`
	Port     int    `json:"port" mapstructure:"port"`

	DSN string `json:"dsn" mapstructure:"dsn"`

	SetMaxOpenConnections int `json:"max_open_conn" mapstructure:"max_open_conn"`
	SetMaxIdleConnections int `json:"max_idle_conn" mapstructure:"max_idle_conn"`
	SetConnMaxLifetime    int `json:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`

	ReadReplicas []DatabaseConfig `json:"read_replicas" mapstructure:"read_replicas"`
}

type HttpListenerConfig struct {
	QueueSize int `mapstructure:"queue_size"`
	Workers   int `mapstructure:"workers"`
}

type ListenerConfig struct {
	Http HttpListenerConfig `mapstructure:"http"`
}

type ApiConfig struct {
	Addr string `mapstructure:"addr"`
}

type LocalWorkerConfig struct {
	ScanDuration         int `mapstructure:"scan_duration"` // in milliseconds
	MinThreads           int `mapstructure:"min_threads"`
	MaxThreads           int `mapstructure:"max_threads"`
	ResultHandlerThreads int `mapstructure:"result_handlers_threads"`
	QueueSize            int `mapstructure:"queue_size"`
}

type QueueWorkerConfig struct {
	Addr        string `mapstructure:"addr"` // in milliseconds
	Db          int    `mapstructure:"db"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	Concurrency int    `mapstructure:"concurrency"`
}

type MetricsConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WorkerAddr string `mapstructure:"worker_addr"`
}

type LoggingConfig struct {
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
}

type WALConfig struct {
	Path   string `mapstructure:"path"`
	Format string `mapstructure:"format"`
}

type HttpTargetConfig struct {
	MaxRetries uint16 `mapstructure:"max_retries"`
}

type SubscriptionConfig struct {
	Database DatabaseConfig `mapstructure:"db"`
}

type AwsConfig struct {
	AccessKeyID string `mapstructure:"aws_access_key_id"`
	SecretKey   string `mapstructure:"aws_secret_access_key"`
	Region      string `mapstructure:"aws_region"`
}

type DeliveryConfig struct {
	Database DatabaseConfig `mapstructure:"db"`
}

type Config struct {
	Listener  ListenerConfig `mapstructure:"listener"`
	Api       ApiConfig      `mapstructure:"api"`
	Metrics   MetricsConfig  `mapstructure:"metrics"`
	Logging   LoggingConfig  `mapstructure:"logging"`
	WalConfig WALConfig      `mapstructure:"wal"`
	Aws       AwsConfig      `mapstructure:"aws"`

	// Subscription
	Subscription SubscriptionConfig `mapstructure:"subscription"`
	Delivery     DeliveryConfig     `mapstructure:"delivery"`

	// Worker
	LocalWorker LocalWorkerConfig `mapstructure:"local_worker"`
	QueueWorker QueueWorkerConfig `mapstructure:"queue_worker"`

	// TargetConfig
	HttpTarget HttpTargetConfig `json:"http_target" mapstructure:"http_target"`
}

func LoadConfig(customConfigPath string) (*Config, error) {
	var cfg Config
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
	if err := v.Unmarshal(&cfg); err != nil {
		log.Printf("error unmarshaling configuration: %s", err)
		return nil, errors.New("error unmarshaling configuration")
	}

	return &cfg, nil
}

func (dc DatabaseConfig) BuildDsn() string {
	if len(dc.DSN) > 0 {
		return dc.DSN
	}
	if dc.Scheme == "" {
		return ""
	}

	authPart := ""
	if dc.Username != "" || dc.Password != "" {
		authPrefix := url.UserPassword(dc.Username, dc.Password)
		authPart = fmt.Sprintf("%s@", authPrefix)
	}

	dbPart := ""
	if dc.Database != "" {
		dbPart = fmt.Sprintf("/%s", dc.Database)
	}

	optPart := ""
	if dc.Options != "" {
		optPart = fmt.Sprintf("?%s", dc.Options)
	}

	return fmt.Sprintf("%s://%s%s:%d%s%s", dc.Scheme, authPart, dc.Host, dc.Port, dbPart, optPart)
}
