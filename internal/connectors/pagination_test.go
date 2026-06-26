package connectors

import (
	"reflect"
	"testing"
)

func TestApplyPaginationPostgres(t *testing.T) {
	sql, args := ApplyPagination("SELECT * FROM t WHERE status = $1", []any{"OPEN"}, "postgres", 50, 100)
	wantSQL := "SELECT * FROM t WHERE status = $1 LIMIT $2 OFFSET $3"
	wantArgs := []any{"OPEN", 50, 100}
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestApplyPaginationMySQLUsesOffsetThenLimit(t *testing.T) {
	sql, args := ApplyPagination("SELECT * FROM t WHERE status = ?", []any{"OPEN"}, "mysql", 50, 100)
	wantSQL := "SELECT * FROM t WHERE status = ? LIMIT ?, ?"
	wantArgs := []any{"OPEN", 100, 50}
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestApplyPaginationSQLServerAddsStableOrder(t *testing.T) {
	sql, args := ApplyPagination("SELECT * FROM t WHERE status = @p1", []any{"OPEN"}, "sqlserver", 50, 100)
	wantSQL := "SELECT * FROM t WHERE status = @p1 ORDER BY (SELECT 1) OFFSET @p2 ROWS FETCH NEXT @p3 ROWS ONLY"
	wantArgs := []any{"OPEN", 100, 50}
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestApplyPaginationOracle(t *testing.T) {
	sql, args := ApplyPagination("SELECT * FROM t WHERE status = :1", []any{"OPEN"}, "oracle", 50, 100)
	wantSQL := "SELECT * FROM t WHERE status = :1 OFFSET :2 ROWS FETCH NEXT :3 ROWS ONLY"
	wantArgs := []any{"OPEN", 100, 50}
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestApplyPaginationNoLimitKeepsQuery(t *testing.T) {
	sql, args := ApplyPagination("SELECT * FROM t", []any{}, "postgres", 0, 10)
	if sql != "SELECT * FROM t" {
		t.Fatalf("sql = %q", sql)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v", args)
	}
}
