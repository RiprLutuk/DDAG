// Package connectors implements per-database-type query execution with a
// configured connection pool per source database (PRD §11.12, §27). Each driver
// (postgres/mysql/sqlserver/oracle) owns one pool per connection and rewrites
// the :named parameter template into its native placeholder style using prepared
// statements — never string concatenation (PRD §13.5).
package connectors

import (
	"context"
	"time"
)

// PoolConfig is the per-connection pool + connectivity configuration. Every
// field is sourced from the database_connections row so each DB is tuned
// independently.
type PoolConfig struct {
	ConnectionID    string
	DatabaseType    string
	Host            string
	Port            int
	Database        string
	ServiceName     string
	Schema          string
	Username        string
	Password        string
	SSLMode         string
	MinPool         int
	MaxPool         int
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
	MaxConnLifetime time.Duration
	MaxConnIdle     time.Duration
}

// QueryRequest is the internal query contract (PRD §16.2).
type QueryRequest struct {
	RequestID     string         `json:"request_id"`
	QueryTemplate string         `json:"query_template"`
	Parameters    map[string]any `json:"parameters"`
	TimeoutMS     int            `json:"timeout_ms"`
	Limit         int            `json:"limit"`
	Offset        int            `json:"offset"`
}

// QueryResult is the internal query response (PRD §16.2).
type QueryResult struct {
	Success      bool             `json:"success"`
	DurationMS   int64            `json:"duration_ms"`
	RowCount     int              `json:"row_count"`
	CircuitState string           `json:"circuit_state,omitempty"`
	Rows         []map[string]any `json:"rows"`
}

// PoolStats reports live pool utilization for metrics.
type PoolStats struct {
	InUse int
	Idle  int
	Max   int
}

// Connector is a live, pooled connection to one source database.
type Connector interface {
	// Query runs the (already validated) template with bound parameters.
	Query(ctx context.Context, req QueryRequest) (*QueryResult, error)
	// HealthCheck verifies the pool can reach the database.
	HealthCheck(ctx context.Context) error
	// Stats returns live pool utilization.
	Stats() PoolStats
	// Close drains and closes the pool.
	Close()
}

// Builder constructs a Connector for a PoolConfig. Each driver registers one.
type Builder func(ctx context.Context, cfg PoolConfig) (Connector, error)
