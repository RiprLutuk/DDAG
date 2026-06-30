package connectors

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// sqlConnector is a database/sql-backed connector shared by MySQL, SQL Server
// and Oracle. The pool is configured via the standard sql.DB knobs so each
// connection's pool is tuned independently (PRD §27).
type sqlConnector struct {
	db          *sql.DB
	cfg         PoolConfig
	placeholder Placeholder
}

func newSQLConnector(ctx context.Context, cfg PoolConfig, driver, dsn string, ph Placeholder) (Connector, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driver, err)
	}
	db.SetMaxOpenConns(max1(cfg.MaxPool))
	db.SetMaxIdleConns(max1(cfg.MinPool))
	if cfg.MaxConnLifetime > 0 {
		db.SetConnMaxLifetime(cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdle > 0 {
		db.SetConnMaxIdleTime(cfg.MaxConnIdle)
	}
	c := &sqlConnector{db: db, cfg: cfg, placeholder: ph}
	pingCtx, cancel := context.WithTimeout(ctx, connectTO(cfg))
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return c, nil
}

func (c *sqlConnector) Query(ctx context.Context, req QueryRequest) (*QueryResult, error) {
	query, args, err := Bind(req.QueryTemplate, req.Parameters, c.placeholder)
	if err != nil {
		return nil, err
	}
	query, args = ApplyPagination(query, args, c.cfg.DatabaseType, req.Limit, req.Offset)

	// Bound connection acquisition separately from the query so an exhausted pool
	// fails fast (pool-exhausted) instead of burning the full query timeout.
	acqCtx, cancelAcq := context.WithTimeout(ctx, connectTO(c.cfg))
	conn, err := c.db.Conn(acqCtx)
	cancelAcq()
	if err != nil {
		return nil, fmt.Errorf("pool acquire: %w", err)
	}
	defer conn.Close()

	qctx, cancel := context.WithTimeout(ctx, queryTO(c.cfg, req.TimeoutMS))
	defer cancel()

	start := time.Now()
	rows, err := conn.QueryContext(qctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, 16)
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		out = append(out, rowToMap(cols, raw))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &QueryResult{
		Success:    true,
		DurationMS: time.Since(start).Milliseconds(),
		RowCount:   len(out),
		Rows:       out,
	}, nil
}

func (c *sqlConnector) HealthCheck(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, connectTO(c.cfg))
	defer cancel()
	return c.db.PingContext(hctx)
}

func (c *sqlConnector) Stats() PoolStats {
	s := c.db.Stats()
	return PoolStats{
		InUse:          s.InUse,
		Idle:           s.Idle,
		Total:          s.OpenConnections,
		Max:            s.MaxOpenConnections,
		WaitCount:      s.WaitCount,
		WaitDurationMS: s.WaitDuration.Milliseconds(),
	}
}

func (c *sqlConnector) Close() { _ = c.db.Close() }
