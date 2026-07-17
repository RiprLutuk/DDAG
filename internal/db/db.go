// Package db manages the metadata PostgreSQL connection pool (PRD §10.2) and
// the schema migration runner.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/ddag/ddag/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the metadata database connection pool. It is an alias so callers
// import this package rather than pgx directly.
type Pool = pgxpool.Pool

// Connect builds a configured pgxpool for the metadata database. All pool sizing
// (min/max conns, lifetimes, health-check period) comes from config so it can be
// tuned per environment (PRD §27: every connection has a configured pool).
func Connect(ctx context.Context, c config.PostgresConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(c.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse metadata dsn: %w", err)
	}
	cfg.MinConns = c.MinConns
	cfg.MaxConns = c.MaxConns
	cfg.MaxConnLifetime = c.MaxConnLife
	cfg.MaxConnIdleTime = c.MaxConnIdle
	cfg.HealthCheckPeriod = c.HealthPeriod
	if c.ConnectTO > 0 {
		cfg.ConnConfig.ConnectTimeout = c.ConnectTO
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create metadata pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping metadata db: %w", err)
	}
	return pool, nil
}
