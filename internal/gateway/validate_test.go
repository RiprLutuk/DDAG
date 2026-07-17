package gateway

import (
	"testing"

	"github.com/ddag/ddag/internal/models"
)

func mkParam(name, source, ptype string, required bool) models.APIParameter {
	return models.APIParameter{Name: name, Source: source, ParamType: ptype, Required: required}
}

func TestValidateForPublish_OK(t *testing.T) {
	api := models.APIDefinition{
		QueryTemplate: "SELECT id FROM t WHERE id = :id LIMIT 1",
		DefaultLimit:  1,
	}
	params := []models.APIParameter{mkParam("id", "path", "string", true)}
	if err := ValidateForPublish(api, params); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateForPublish_RejectsWrite(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "DELETE FROM t WHERE id = :id", DefaultLimit: 1}
	params := []models.APIParameter{mkParam("id", "path", "string", true)}
	if err := ValidateForPublish(api, params); err == nil {
		t.Fatal("expected write statement to be rejected")
	}
}

func TestValidateForPublish_RejectsNewSideEffects(t *testing.T) {
	dangerousQueries := []string{
		"CALL dangerous_procedure(:id)",
		"EXEC dangerous_procedure :id",
		"DO $$ BEGIN DELETE FROM t; END $$",
		"COPY t FROM '/etc/passwd'",
		"SELECT id INTO backup_table FROM t WHERE id = :id",
	}

	for _, q := range dangerousQueries {
		api := models.APIDefinition{QueryTemplate: q, Method: "GET", DefaultLimit: 1}
		params := []models.APIParameter{mkParam("id", "query", "string", false)}
		err := ValidateForPublish(api, params)
		if err == nil {
			t.Fatalf("expected error rejecting dangerous side effect query %q on GET API", q)
		}
	}
}

func TestValidateForPublish_AllowsQuotedKeywordsInDialect(t *testing.T) {
	// e.g. "select" column or table named "update" or "insert" when quoted
	queries := []struct {
		q       string
		dialect string
	}{
		{"SELECT `insert`, `delete` FROM t WHERE id = :id", "mysql"},
		{`SELECT "update", "drop" FROM t WHERE id = :id`, "postgres"},
		{"SELECT [alter], [create] FROM t WHERE id = :id", "sqlserver"},
	}

	for _, tt := range queries {
		api := models.APIDefinition{QueryTemplate: tt.q, Method: "GET", ConnectorType: tt.dialect, DefaultLimit: 1}
		params := []models.APIParameter{mkParam("id", "query", "string", false)}
		err := ValidateForPublish(api, params)
		if err != nil {
			t.Fatalf("unexpected error for %q (%s): %v", tt.q, tt.dialect, err)
		}
	}
}
func TestValidateForPublish_RejectsWriteInsideCTE(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "WITH removed AS (DELETE FROM t WHERE id = :id RETURNING id) SELECT id FROM removed", DefaultLimit: 1}
	params := []models.APIParameter{mkParam("id", "path", "string", true)}
	if err := ValidateForPublish(api, params); err == nil {
		t.Fatal("expected CTE containing a write statement to be rejected")
	}
}

func TestValidateForPublish_RejectsWriteHiddenByLeadingComment(t *testing.T) {
	for _, query := range []string{
		"/* operator note */ DELETE FROM t WHERE id = :id",
		"-- operator note\nUPDATE t SET status = 'disabled' WHERE id = :id",
	} {
		t.Run(query[:2], func(t *testing.T) {
			api := models.APIDefinition{QueryTemplate: query, DefaultLimit: 1}
			params := []models.APIParameter{mkParam("id", "path", "string", true)}
			if err := ValidateForPublish(api, params); err == nil {
				t.Fatalf("expected commented write statement to be rejected: %q", query)
			}
		})
	}
}

func TestValidateForExecution_RejectsWriteHiddenByLeadingComment(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "/* operator note */ DELETE FROM t WHERE id = :id", DefaultLimit: 1}
	if err := ValidateForExecution(api); err == nil {
		t.Fatal("expected commented write statement to be rejected")
	}
}

func TestValidateForPublish_AllowsWriteWhenFlagged(t *testing.T) {
	api := models.APIDefinition{Method: "PATCH", QueryTemplate: "UPDATE t SET x=1 WHERE id = :id", DefaultLimit: 1, IsWrite: true}
	params := []models.APIParameter{mkParam("id", "path", "string", true)}
	if err := ValidateForPublish(api, params); err != nil {
		t.Fatalf("write API should be allowed when IsWrite=true, got %v", err)
	}
}

func TestValidateForPublish_GETRejectsWriteEvenWhenLegacyFlagged(t *testing.T) {
	api := models.APIDefinition{Method: "GET", QueryTemplate: "DELETE FROM t WHERE id = :id", DefaultLimit: 1, IsWrite: true}
	params := []models.APIParameter{mkParam("id", "path", "string", true)}
	if err := ValidateForPublish(api, params); err == nil {
		t.Fatal("GET must reject side effects regardless of legacy IsWrite")
	}
}

func TestValidateForExecution_QUERYRejectsWriteEvenWhenLegacyFlagged(t *testing.T) {
	api := models.APIDefinition{Method: "QUERY", QueryTemplate: "UPDATE t SET x=1", DefaultLimit: 1, IsWrite: true}
	if err := ValidateForExecution(api); err == nil {
		t.Fatal("QUERY must reject side effects regardless of legacy IsWrite")
	}
}

func TestValidateForPublish_RejectsMultiStatement(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "SELECT 1; SELECT 2", DefaultLimit: 1}
	if err := ValidateForPublish(api, nil); err == nil {
		t.Fatal("expected multi-statement rejection")
	}
}

func TestValidateForPublish_UndeclaredParam(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "SELECT :a, :b", DefaultLimit: 1}
	params := []models.APIParameter{mkParam("a", "query", "string", true)}
	if err := ValidateForPublish(api, params); err == nil {
		t.Fatal("expected error for undeclared :b")
	}
}

func TestValidateForPublish_UnusedDeclaredParam(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "SELECT :a", DefaultLimit: 1}
	params := []models.APIParameter{mkParam("a", "query", "string", true), mkParam("ghost", "query", "string", false)}
	if err := ValidateForPublish(api, params); err == nil {
		t.Fatal("expected error for unused declared param")
	}
}

func TestValidateForPublish_RequiresLimit(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "SELECT :a", DefaultLimit: 0}
	params := []models.APIParameter{mkParam("a", "query", "string", true)}
	if err := ValidateForPublish(api, params); err == nil {
		t.Fatal("expected error when default_limit <= 0")
	}
}

func TestValidateForPublish_RejectsInvalidParameterRegex(t *testing.T) {
	invalid := "[unterminated"
	api := models.APIDefinition{QueryTemplate: "SELECT :id", DefaultLimit: 1}
	params := []models.APIParameter{{Name: "id", Source: "query", ParamType: "string", ValidationRule: &invalid}}
	if err := ValidateForPublish(api, params); err == nil {
		t.Fatal("expected invalid parameter regex to be rejected")
	}
}

func TestValidateForPublish_SemicolonInStringIsFine(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "SELECT ';' AS sep WHERE id = :id", DefaultLimit: 1}
	params := []models.APIParameter{mkParam("id", "query", "string", true)}
	if err := ValidateForPublish(api, params); err != nil {
		t.Fatalf("semicolon inside a string literal should be allowed, got %v", err)
	}
}

func TestValidateForExecutionRejectsWriteStatement(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "DELETE FROM t WHERE id = :id", DefaultLimit: 1}
	if err := ValidateForExecution(api); err == nil {
		t.Fatal("expected execution validation to reject write statement")
	}
}

func TestValidateForExecutionRejectsMultiStatement(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "SELECT 1; SELECT 2", DefaultLimit: 1}
	if err := ValidateForExecution(api); err == nil {
		t.Fatal("expected execution validation to reject multi-statement SQL")
	}
}

func TestValidateForExecutionAllowsReadQueryWithSampleParams(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "SELECT id FROM t WHERE id = :id", DefaultLimit: 1}
	if err := ValidateForExecution(api); err != nil {
		t.Fatalf("read-only execution query should be allowed, got %v", err)
	}
}
