package gateway

import (
	"strings"
	"testing"

	"github.com/ddag/ddag/internal/models"
)

func TestValidateForPublishRejectsGETSideEffectsEvenWithLegacyWriteFlag(t *testing.T) {
	api := models.APIDefinition{Method: "GET", QueryTemplate: "UPDATE users SET active=false", DefaultLimit: 1, IsWrite: true}
	if err := ValidateForPublish(api, nil); err == nil || !strings.Contains(err.Error(), "GET") {
		t.Fatalf("GET side effect must be rejected, got %v", err)
	}
}

func TestValidateForExecutionRejectsQUERYSideEffects(t *testing.T) {
	api := models.APIDefinition{Method: "QUERY", QueryTemplate: "DELETE FROM users", DefaultLimit: 1}
	if err := ValidateForExecution(api); err == nil || !strings.Contains(err.Error(), "QUERY") {
		t.Fatalf("QUERY side effect must be rejected, got %v", err)
	}
}

func TestAPIDefinitionOperationTypeIsDerivedWithoutSchemaColumn(t *testing.T) {
	api := models.APIDefinition{Method: "PATCH"}
	if got := api.OperationType(); got != models.OperationUpdate {
		t.Fatalf("operation = %q", got)
	}
}
