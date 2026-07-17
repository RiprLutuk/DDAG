package connectors

import (
	"reflect"
	"testing"
)

func TestBindBasicDollar(t *testing.T) {
	sql, args, err := Bind(
		"SELECT * FROM site WHERE id = :id AND status = :status",
		map[string]any{"id": "ABC", "status": "ACTIVE"}, PlaceholderDollar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT * FROM site WHERE id = $1 AND status = $2"
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 2 || args[0] != "ABC" || args[1] != "ACTIVE" {
		t.Fatalf("args = %v", args)
	}
}

func TestBindRepeatedParamReusesArgPosition(t *testing.T) {
	// :status appears twice; each occurrence is its own positional arg in order.
	sql, args, err := Bind(
		"SELECT 1 WHERE (:status = '' OR status = :status)",
		map[string]any{"status": "OPEN"}, PlaceholderDollar)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sql != "SELECT 1 WHERE ($1 = '' OR status = $2)" {
		t.Fatalf("sql = %q", sql)
	}
	if len(args) != 2 {
		t.Fatalf("want 2 args, got %v", args)
	}
}

func TestBindIgnoresStringLiteralsAndCasts(t *testing.T) {
	// ':' inside a string literal and the '::' cast must NOT be treated as params.
	sql, args, err := Bind(
		"SELECT 'a:b' AS lit, id::text FROM t WHERE id = :id",
		map[string]any{"id": 7}, PlaceholderDollar)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := "SELECT 'a:b' AS lit, id::text FROM t WHERE id = $1"
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != 7 {
		t.Fatalf("args = %v", args)
	}
}

func TestBindMissingParamErrors(t *testing.T) {
	_, _, err := Bind("SELECT :missing", map[string]any{}, PlaceholderDollar)
	if err == nil {
		t.Fatal("expected error for missing parameter")
	}
}

func TestBindInjectionValueStaysBound(t *testing.T) {
	// A classic injection payload must remain a single bound arg, never SQL.
	payload := "x'; DROP TABLE users; --"
	sql, args, err := Bind("SELECT * FROM t WHERE name = :name",
		map[string]any{"name": payload}, PlaceholderQuestion)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sql != "SELECT * FROM t WHERE name = ?" {
		t.Fatalf("sql = %q", sql)
	}
	if len(args) != 1 || args[0] != payload {
		t.Fatalf("payload not bound verbatim: %v", args)
	}
}

func TestExtractParamNamesDialect(t *testing.T) {
	got := ExtractParamNamesDialect("SELECT :a, :b, :a FROM t WHERE x = :c::int -- :notparam\n# :notparam_mysql", DialectMySQL)
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	gotOracle := ExtractParamNamesDialect("SELECT :a FROM t WHERE x = q'[ :notparam ]'", DialectOracle)
	if len(gotOracle) != 1 || gotOracle[0] != "a" {
		t.Errorf("got %v, want [a]", gotOracle)
	}
}

func TestBindDialectEscapedIdentifiers(t *testing.T) {
	tests := []struct {
		sql     string
		dialect Dialect
	}{
		{"SELECT `:odd``fake`, :real", DialectMySQL},
		{`SELECT "odd"":fake", :real`, DialectPostgres},
		{"SELECT [odd]]:fake], :real", DialectSQLServer},
		{"SELECT ':odd''fake', :real", DialectMySQL}, // Escaped single quote
	}

	for _, tt := range tests {
		_, args, err := BindDialect(
			tt.sql,
			map[string]any{"real": 7, "fake": 8},
			tt.dialect,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(args) != 1 || args[0] != 7 {
			t.Fatalf("for %s, unexpected args: %#v", tt.sql, args)
		}
	}
}

func TestBindSkipsDialectQuotedRegions(t *testing.T) {
	tests := []struct {
		name, sql string
		dialect   Dialect
	}{
		{"postgres dollar quote", "SELECT $$:fake$$, :real", DialectPostgres},
		{"postgres tagged dollar quote", "SELECT $body$:fake$body$, :real", DialectPostgres},
		{"mysql backtick", "SELECT `odd:name`, :real", DialectMySQL},
		{"sqlserver bracket", "SELECT [odd:name], :real", DialectSQLServer},
		{"double quoted identifier", `SELECT "odd:name", :real`, DialectPostgres},
		{"oracle q quote", "SELECT q'[odd:name]', :real FROM dual", DialectOracle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, args, err := BindDialect(tt.sql, map[string]any{"real": 7, "fake": 8, "name": 9}, tt.dialect)
			if err != nil {
				t.Fatal(err)
			}
			if len(args) != 1 || args[0] != 7 {
				t.Fatalf("args = %v, want [7] (SQL=%q)", args, got)
			}
			if got == tt.sql {
				t.Fatalf("real parameter was not bound: %q", got)
			}
		})
	}
}
