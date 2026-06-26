//go:build integration

package connectors

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestPostgresConnectorIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	explicit := postgresIntegrationExplicit()

	cfg := PoolConfig{
		ConnectionID:   "integration-postgres",
		DatabaseType:   "postgres",
		Host:           envOr("DDAG_INTEGRATION_POSTGRES_HOST", "127.0.0.1"),
		Port:           envInt("DDAG_INTEGRATION_POSTGRES_PORT", 5432),
		Database:       envOr("DDAG_INTEGRATION_POSTGRES_DB", "postgres"),
		Username:       envOr("DDAG_INTEGRATION_POSTGRES_USER", "postgres"),
		Password:       envOr("DDAG_INTEGRATION_POSTGRES_PASSWORD", "postgres"),
		SSLMode:        envOr("DDAG_INTEGRATION_POSTGRES_SSLMODE", "disable"),
		MinPool:        0,
		MaxPool:        2,
		ConnectTimeout: 3 * time.Second,
		QueryTimeout:   3 * time.Second,
	}

	conn, err := BuildPostgres(ctx, cfg)
	if err != nil {
		if explicit {
			t.Fatalf("postgres integration target unavailable at %s:%d/%s: %v", cfg.Host, cfg.Port, cfg.Database, err)
		}
		t.Skipf("postgres integration target unavailable at %s:%d/%s: %v", cfg.Host, cfg.Port, cfg.Database, err)
	}
	defer conn.Close()

	res, err := conn.Query(ctx, QueryRequest{
		QueryTemplate: "SELECT n FROM generate_series(:start, :stop) AS t(n) ORDER BY n",
		Parameters:    map[string]any{"start": 1, "stop": 5},
		Limit:         2,
		Offset:        1,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !res.Success || res.RowCount != 2 || len(res.Rows) != 2 {
		t.Fatalf("result = %+v", res)
	}
	if fmt.Sprint(res.Rows[0]["n"]) != "2" || fmt.Sprint(res.Rows[1]["n"]) != "3" {
		t.Fatalf("rows = %+v", res.Rows)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func postgresIntegrationExplicit() bool {
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
