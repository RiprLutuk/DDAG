package connectors

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	// Side-effect imports register the database/sql drivers.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"
)

// Builders maps a database type to its connector builder.
var Builders = map[string]Builder{
	"postgres":  BuildPostgres,
	"mysql":     BuildMySQL,
	"sqlserver": BuildSQLServer,
	"oracle":    BuildOracle,
}

// BuildFor constructs a connector for the configured database type.
func BuildFor(ctx context.Context, cfg PoolConfig) (Connector, error) {
	b, ok := Builders[cfg.DatabaseType]
	if !ok {
		return nil, fmt.Errorf("unsupported database type %q", cfg.DatabaseType)
	}
	return b(ctx, cfg)
}

// BuildMySQL constructs a MySQL/MariaDB connector.
func BuildMySQL(ctx context.Context, cfg PoolConfig) (Connector, error) {
	// user:pass@tcp(host:port)/dbname?params
	params := url.Values{}
	params.Set("parseTime", "true")
	params.Set("timeout", durStr(connectTO(cfg)))
	params.Set("readTimeout", durStr(queryTO(cfg, 0)))
	if cfg.SSLMode != "" && cfg.SSLMode != "disable" {
		params.Set("tls", cfg.SSLMode)
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database, params.Encode())
	return newSQLConnector(ctx, cfg, "mysql", dsn, PlaceholderQuestion)
}

// BuildSQLServer constructs a Microsoft SQL Server connector.
func BuildSQLServer(ctx context.Context, cfg PoolConfig) (Connector, error) {
	q := url.Values{}
	if cfg.Database != "" {
		q.Set("database", cfg.Database)
	}
	q.Set("connection timeout", strconv.Itoa(int(connectTO(cfg).Seconds())))
	if cfg.SSLMode == "" || cfg.SSLMode == "disable" {
		q.Set("encrypt", "disable")
	} else {
		q.Set("encrypt", cfg.SSLMode)
	}
	u := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(cfg.Username, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		RawQuery: q.Encode(),
	}
	return newSQLConnector(ctx, cfg, "sqlserver", u.String(), PlaceholderAtP)
}

// BuildOracle constructs an Oracle connector (pure-Go go-ora driver — no Oracle
// client install required).
func BuildOracle(ctx context.Context, cfg PoolConfig) (Connector, error) {
	service := cfg.ServiceName
	if service == "" {
		service = cfg.Database
	}
	u := url.URL{
		Scheme: "oracle",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/" + service,
	}
	return newSQLConnector(ctx, cfg, "oracle", u.String(), PlaceholderColon)
}

func durStr(d time.Duration) string {
	if d <= 0 {
		return "30s"
	}
	return d.String()
}
