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

func TestValidateForPublish_AllowsWriteWhenFlagged(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "UPDATE t SET x=1 WHERE id = :id", DefaultLimit: 1, IsWrite: true}
	params := []models.APIParameter{mkParam("id", "path", "string", true)}
	if err := ValidateForPublish(api, params); err != nil {
		t.Fatalf("write API should be allowed when IsWrite=true, got %v", err)
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

func TestValidateForPublish_SemicolonInStringIsFine(t *testing.T) {
	api := models.APIDefinition{QueryTemplate: "SELECT ';' AS sep WHERE id = :id", DefaultLimit: 1}
	params := []models.APIParameter{mkParam("id", "query", "string", true)}
	if err := ValidateForPublish(api, params); err != nil {
		t.Fatalf("semicolon inside a string literal should be allowed, got %v", err)
	}
}
