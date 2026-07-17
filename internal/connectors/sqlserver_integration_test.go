//go:build integration

package connectors

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSQLServerConnectorIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	explicit := sqlserverIntegrationExplicit()

	cfg := PoolConfig{
		ConnectionID:   "integration-sqlserver",
		DatabaseType:   "sqlserver",
		Host:           envOr("DDAG_INTEGRATION_MSSQL_HOST", "127.0.0.1"),
		Port:           envInt("DDAG_INTEGRATION_MSSQL_PORT", 1433),
		Database:       envOr("DDAG_INTEGRATION_MSSQL_DB", "master"),
		Username:       envOr("DDAG_INTEGRATION_MSSQL_USER", "sa"),
		Password:       envOr("DDAG_INTEGRATION_MSSQL_PASSWORD", ""),
		MinPool:        0,
		MaxPool:        2,
		ConnectTimeout: 5 * time.Second,
		QueryTimeout:   5 * time.Second,
	}

	if cfg.Password == "" && !explicit {
		t.Skip("skipping SQL Server integration test; DDAG_INTEGRATION_MSSQL_PASSWORD not set")
	}

	conn, err := BuildSQLServer(ctx, cfg)
	if err != nil {
		if explicit {
			t.Fatalf("sqlserver integration target unavailable at %s:%d/%s: %v", cfg.Host, cfg.Port, cfg.Database, err)
		}
		t.Skipf("sqlserver integration target unavailable: %v", err)
	}
	defer conn.Close()

	// Clean up old table if exists
	_, _ = conn.Query(ctx, QueryRequest{
		QueryTemplate: "IF OBJECT_ID('ddag_int_users', 'U') IS NOT NULL DROP TABLE ddag_int_users;",
	})

	// Create table
	_, err = conn.Query(ctx, QueryRequest{
		QueryTemplate: `CREATE TABLE ddag_int_users (
			id INT IDENTITY(1,1) PRIMARY KEY,
			username NVARCHAR(50) NOT NULL,
			score DECIMAL(10,2) NOT NULL,
			is_active BIT NOT NULL
		)`,
	})
	if err != nil {
		t.Fatalf("setup create table: %v", err)
	}
	defer func() {
		_, _ = conn.Query(context.Background(), QueryRequest{QueryTemplate: "DROP TABLE ddag_int_users"})
	}()

	// Insert data
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

	// Test 1: Basic read with bracket identifiers
	res, err := conn.Query(ctx, QueryRequest{
		QueryTemplate: "SELECT [id], [username], [score] FROM ddag_int_users WHERE [is_active] = :active ORDER BY [id] ASC",
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

	// Test 2: Pagination (ORDER BY + OFFSET + FETCH NEXT)
	res2, err := conn.Query(ctx, QueryRequest{
		QueryTemplate: "SELECT [id], [username] FROM ddag_int_users ORDER BY [id] ASC",
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
}

func sqlserverIntegrationExplicit() bool {
	keys := []string{
		"DDAG_INTEGRATION_MSSQL_HOST",
		"DDAG_INTEGRATION_MSSQL_PORT",
		"DDAG_INTEGRATION_MSSQL_DB",
		"DDAG_INTEGRATION_MSSQL_USER",
		"DDAG_INTEGRATION_MSSQL_PASSWORD",
	}
	for _, key := range keys {
		if _, ok := os.LookupEnv(key); ok {
			return true
		}
	}
	return false
}