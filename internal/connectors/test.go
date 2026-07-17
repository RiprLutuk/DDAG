package connectors

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TestConnectivity verifies that the database described by cfg is reachable,
// without building a long-lived pool. It is used by the dashboard's
// "Test connection" action (PRD §11.6) and returns a (sanitized by caller)
// error on failure.
func TestConnectivity(ctx context.Context, cfg PoolConfig) error {
	tctx, cancel := context.WithTimeout(ctx, connectTO(cfg))
	defer cancel()

	if cfg.DatabaseType == "postgres" {
		conn, err := pgx.Connect(tctx, pgDSN(cfg))
		if err != nil {
			return err
		}
		defer conn.Close(context.Background())
		return conn.Ping(tctx)
	}

	// MySQL/SQL Server/Oracle use database/sql, whose pool does not have the
	// eager-maintenance race; a one-shot connector that pings on build is fine.
	b, ok := Builders[cfg.DatabaseType]
	if !ok {
		return fmt.Errorf("unsupported database type %q", cfg.DatabaseType)
	}
	c, err := b(tctx, cfg)
	if err != nil {
		return err
	}
	c.Close()
	return nil
}
