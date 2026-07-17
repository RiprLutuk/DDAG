//go:build integration

package store

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConsumeRefreshTokenAllowsOnlyOneConcurrentConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminPool, explicit, err := openStoreIntegrationPool(ctx, "")
	if err != nil {
		if explicit {
			t.Fatalf("postgres integration target unavailable: %v", err)
		}
		t.Skipf("postgres integration target unavailable: %v", err)
	}
	defer adminPool.Close()

	schema := "ddag_store_it_" + stringsWithoutDashes(uuid.NewString())
	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer adminPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)

	pool, _, err := openStoreIntegrationPool(ctx, schema)
	if err != nil {
		t.Fatalf("open schema-scoped pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `
		CREATE TABLE clients (
			id UUID PRIMARY KEY,
			client_id TEXT NOT NULL,
			client_name TEXT NOT NULL,
			client_secret_hash TEXT NOT NULL,
			environment TEXT NOT NULL,
			status TEXT NOT NULL,
			access_token_ttl_seconds INT NOT NULL,
			refresh_token_ttl_seconds INT NOT NULL,
			description TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE refresh_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			token_hash TEXT NOT NULL UNIQUE,
			client_id UUID NOT NULL REFERENCES clients(id),
			scope TEXT NOT NULL DEFAULT '',
			expires_at TIMESTAMPTZ NOT NULL,
			revoked BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`); err != nil {
		t.Fatalf("create tables: %v", err)
	}

	clientID := uuid.New()
	tokenHash := "refresh-token-race-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO clients (id, client_id, client_name, client_secret_hash, environment, status,
			access_token_ttl_seconds, refresh_token_ttl_seconds, description)
		VALUES ($1, 'client-a', 'Client A', 'hash', 'dev', 'active', 3600, 2592000, '')`,
		clientID); err != nil {
		t.Fatalf("insert client: %v", err)
	}
	if err := New(pool).CreateRefreshToken(ctx, tokenHash, clientID, "read write", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	st := New(pool)
	const workers = 12
	var successes atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := st.ConsumeRefreshToken(context.Background(), tokenHash); err == nil {
				successes.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("successful concurrent consumers = %d, want 1", got)
	}
}

func openStoreIntegrationPool(ctx context.Context, searchPath string) (*pgxpool.Pool, bool, error) {
	explicit := storePostgresIntegrationExplicit()
	host := storeEnvOr("DDAG_INTEGRATION_POSTGRES_HOST", "127.0.0.1")
	port := storeEnvInt("DDAG_INTEGRATION_POSTGRES_PORT", 5432)
	db := storeEnvOr("DDAG_INTEGRATION_POSTGRES_DB", "postgres")
	user := storeEnvOr("DDAG_INTEGRATION_POSTGRES_USER", "postgres")
	pass := storeEnvOr("DDAG_INTEGRATION_POSTGRES_PASSWORD", "postgres")
	sslmode := storeEnvOr("DDAG_INTEGRATION_POSTGRES_SSLMODE", "disable")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", user, pass, host, port, db, sslmode)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, explicit, err
	}
	if searchPath != "" {
		cfg.ConnConfig.RuntimeParams["search_path"] = searchPath
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, explicit, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, explicit, err
	}
	return pool, explicit, nil
}

func storeEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func storeEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func storePostgresIntegrationExplicit() bool {
	keys := []string{
		"DDAG_INTEGRATION_POSTGRES_HOST",
		"DDAG_INTEGRATION_POSTGRES_PORT",
		"DDAG_INTEGRATION_POSTGRES_DB",
		"DDAG_INTEGRATION_POSTGRES_USER",
		"DDAG_INTEGRATION_POSTGRES_PASSWORD",
	}
	for _, key := range keys {
		if _, ok := os.LookupEnv(key); ok {
			return true
		}
	}
	return false
}

func stringsWithoutDashes(v string) string {
	out := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		if v[i] != '-' {
			out = append(out, v[i])
		}
	}
	return string(out)
}
