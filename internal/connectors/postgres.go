package connectors

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pgConnector is a pgxpool-backed connector to a PostgreSQL source.
type pgConnector struct {
	pool *pgxpool.Pool
	cfg  PoolConfig
}

// pgDSN builds a pgx URL DSN for a PoolConfig, handling empty passwords.
func pgDSN(cfg PoolConfig) string {
	u := url.URL{
		Scheme:   "postgres",
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:     "/" + cfg.Database,
		RawQuery: "sslmode=" + defaultStr(cfg.SSLMode, "disable"),
	}
	if cfg.Password != "" {
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	} else {
		u.User = url.User(cfg.Username)
	}
	return u.String()
}

// BuildPostgres constructs a PostgreSQL connector with a configured pool.
func BuildPostgres(ctx context.Context, cfg PoolConfig) (Connector, error) {
	pcfg, err := pgxpool.ParseConfig(pgDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	maxC := max1(cfg.MaxPool)
	minC := cfg.MinPool
	if minC < 0 {
		minC = 0
	}
	if minC > maxC {
		minC = maxC
	}
	// pgxpool can deadlock acquiring its only slot when MinConns==MaxConns==1
	// (background maintenance vs. acquire race); keep at least one free slot.
	if minC == maxC && maxC == 1 {
		minC = 0
	}
	pcfg.MinConns = int32(minC)
	pcfg.MaxConns = int32(maxC)
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	pcfg.MaxConnIdleTime = cfg.MaxConnIdle
	if cfg.ConnectTimeout > 0 {
		pcfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}
	if cfg.Schema != "" {
		pcfg.ConnConfig.RuntimeParams["search_path"] = cfg.Schema
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	c := &pgConnector{pool: pool, cfg: cfg}
	pingCtx, cancel := context.WithTimeout(ctx, connectTO(cfg))
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return c, nil
}

func (c *pgConnector) Query(ctx context.Context, req QueryRequest) (*QueryResult, error) {
	sql, args, err := Bind(req.QueryTemplate, req.Parameters, PlaceholderDollar)
	if err != nil {
		return nil, err
	}
	sql, args = ApplyPagination(sql, args, c.cfg.DatabaseType, req.Limit, req.Offset)

	// Bound connection acquisition separately from the query so an exhausted pool
	// fails fast (pool-exhausted) instead of burning the full query timeout.
	acqCtx, cancelAcq := context.WithTimeout(ctx, connectTO(c.cfg))
	conn, err := c.pool.Acquire(acqCtx)
	cancelAcq()
	if err != nil {
		return nil, fmt.Errorf("pool acquire: %w", err)
	}
	defer conn.Release()

	qctx, cancel := context.WithTimeout(ctx, queryTO(c.cfg, req.TimeoutMS))
	defer cancel()

	start := time.Now()
	rows, err := conn.Query(qctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = string(f.Name)
	}

	out := make([]map[string]any, 0, 16)
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		out = append(out, rowToMap(cols, vals))
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

func (c *pgConnector) HealthCheck(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, connectTO(c.cfg))
	defer cancel()
	return c.pool.Ping(hctx)
}

func (c *pgConnector) Stats() PoolStats {
	s := c.pool.Stat()
	return PoolStats{
		InUse:          int(s.AcquiredConns()),
		Idle:           int(s.IdleConns()),
		Total:          int(s.TotalConns()),
		Max:            int(s.MaxConns()),
		WaitCount:      s.EmptyAcquireCount(),
		WaitDurationMS: s.AcquireDuration().Milliseconds(),
		TimeoutCount:   s.CanceledAcquireCount(),
	}
}

func (c *pgConnector) Close() { c.pool.Close() }
