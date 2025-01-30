package config

import (
	"fmt"
	"net/url"
)

type DatabaseProvider string

const (
	PostgresDatabaseProvider DatabaseProvider = "postgres"
	MySQLDatabaseProvider    DatabaseProvider = "mysql"
)

type DatabaseConfiguration struct {
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

	ReadReplicas []DatabaseConfiguration `json:"read_replicas" mapstructure:"read_replicas"`
}

func (dc DatabaseConfiguration) BuildDsn() string {
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
