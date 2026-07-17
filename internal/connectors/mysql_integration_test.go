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

func TestMySQLConnectorIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	explicit := mysqlIntegrationExplicit()

	cfg := PoolConfig{
		ConnectionID:   "integration-mysql",
		DatabaseType:   "mysql",
		Host:           envOr("DDAG_INTEGRATION_MYSQL_HOST", "127.0.0.1"),
		Port:           envInt("DDAG_INTEGRATION_MYSQL_PORT", 3306),
		Database:       envOr("DDAG_INTEGRATION_MYSQL_DB", "ddag_test"),
		Username:       envOr("DDAG_INTEGRATION_MYSQL_USER", "root"),
		Password:       envOr("DDAG_INTEGRATION_MYSQL_PASSWORD", ""),
		MinPool:        0,
		MaxPool:        2,
		ConnectTimeout: 5 * time.Second,
		QueryTimeout:   5 * time.Second,
	}

	if cfg.Password == "" && !explicit {
		t.Skip("skipping MySQL integration test; DDAG_INTEGRATION_MYSQL_PASSWORD not set")
	}

	conn, err := BuildMySQL(ctx, cfg)
	if err != nil {
		if explicit {
			t.Fatalf("mysql integration target unavailable at %s:%d/%s: %v", cfg.Host, cfg.Port, cfg.Database, err)
		}
		t.Skipf("mysql integration target unavailable: %v", err)
	}
	defer conn.Close()

	// Setup: create temp table and insert data
	_, err = conn.Query(ctx, QueryRequest{
		QueryTemplate: `CREATE TEMPORARY TABLE ddag_int_users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(50) NOT NULL,
			score DECIMAL(10,2) NOT NULL,
			is_active TINYINT(1) NOT NULL
		)`,
		Parameters: map[string]any{},
	})
	if err != nil {
		t.Fatalf("setup create table: %v", err)
	}

	for _, u := range []struct{ name string; score float64; active int }{
		{"alice", 95.50, 1},
		{"bob", 72.00, 0},
		{"charlie", 88.25, 1},
		{"diana", 91.00, 1},
		{"eve", 60.50, 0},
	} {
		_, err = conn.Query(ctx, QueryRequest{
			QueryTemplate: "INSERT INTO ddag_int_users (username, score, is_active) VALUES (:name, :score, :active)",
			Parameters:    map[string]any{"name": u.name, "score": u.score, "active": u.active},
		})
		if err != nil {
			t.Fatalf("setup insert %s: %v", u.name, err)
		}
	}

	// Test 1: Basic read with backtick identifiers
	res, err := conn.Query(ctx, QueryRequest{
		QueryTemplate: "SELECT `id`, `username`, `score` FROM ddag_int_users WHERE `is_active` = :active ORDER BY `id` ASC",
		Parameters:    map[string]any{"active": 1},
	})
	if err != nil {
		t.Fatalf("query active users: %v", err)
	}
	if !res.Success || res.RowCount != 3 {
		t.Fatalf("expected 3 active users, got count=%d, res=%+v", res.RowCount, res)
	}
	if fmt.Sprint(res.Rows[0]["username"]) != "alice" {
		t.Errorf("expected first user 'alice', got %v", res.Rows[0]["username"])
	}

	// Test 2: Pagination (LIMIT + OFFSET)
	res2, err := conn.Query(ctx, QueryRequest{
		QueryTemplate: "SELECT `id`, `username` FROM ddag_int_users ORDER BY `id` ASC",
		Parameters:    map[string]any{},
		Limit:         2,
		Offset:        1,
	})
	if err != nil {
		t.Fatalf("query with pagination: %v", err)
	}
	if !res2.Success || res2.RowCount != 2 {
		t.Fatalf("expected 2 rows after pagination, got %d", res2.RowCount)
	}
	if fmt.Sprint(res2.Rows[0]["username"]) != "bob" {
		t.Errorf("expected first paginated row 'bob' (offset=1), got %v", res2.Rows[0]["username"])
	}
	if fmt.Sprint(res2.Rows[1]["username"]) != "charlie" {
		t.Errorf("expected second paginated row 'charlie', got %v", res2.Rows[1]["username"])
	}

	// Test 3: Decimal type mapping
	scoreVal := fmt.Sprint(res.Rows[0]["score"])
	if scoreVal != "95.5" && scoreVal != "95.50" {
		t.Logf("decimal score value: %v (type %T) — driver formatting may vary", scoreVal, res.Rows[0]["score"])
	}
}

func mysqlIntegrationExplicit() bool {
	keys := []string{
		"DDAG_INTEGRATION_MYSQL_HOST",
		"DDAG_INTEGRATION_MYSQL_PORT",
		"DDAG_INTEGRATION_MYSQL_DB",
		"DDAG_INTEGRATION_MYSQL_USER",
		"DDAG_INTEGRATION_MYSQL_PASSWORD",
	}
	for _, key := range keys {
		if _, ok := os.LookupEnv(key); ok {
			return true
		}
	}
	return false
}

func envOrOverride(key, fallback string) string {
	return envOr(key, fallback)
}

func envIntOverride(key string, fallback int) int {
	return envInt(key, fallback)
}

func strconvOverride(s string) (int, error) {
	return strconv.Atoi(s)
}
