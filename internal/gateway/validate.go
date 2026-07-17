package gateway

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ddag/ddag/internal/connectors"
	"github.com/ddag/ddag/internal/models"
)

// writeStmt matches SQL keywords that mutate data or have side effects.
// Read-only validation scans SQL with quoted literals and comments removed,
// so a mutation cannot be hidden behind a leading comment or inside a CTE
// (PRD §13.5).
var writeStmt = regexp.MustCompile(`(?i)\b(insert|update|delete|merge|truncate|drop|alter|create|grant|revoke|call|exec|execute|do|copy|vacuum|reindex|cluster|restore|backup|select\s+.*\s+into)\b`)

// ValidateForPublish performs the pre-publish safety checks (PRD §11.7 AC):
//   - query is non-empty and single-statement
//   - read-only unless the API is explicitly a write API
//   - every :param in the template is declared, and every declared param is used
//   - parameters are bound (the presence of :params), never concatenated
//   - list/search endpoints have a row limit available (default_limit > 0)
func ValidateForPublish(api models.APIDefinition, params []models.APIParameter) error {
	if _, ok, apiErr := QueryBuilderFromAPI(api); apiErr != nil {
		return errors.New(apiErr.Message)
	} else if ok {
		if api.DefaultLimit <= 0 {
			return fmt.Errorf("default_limit must be greater than zero")
		}
		return nil
	}

	q := strings.TrimSpace(api.QueryTemplate)
	if q == "" {
		return fmt.Errorf("query template is empty")
	}
	operation := api.OperationType()
	if operation == models.OperationRead && writeStmt.MatchString(connectors.StripDialect(q, connectors.Dialect(api.ConnectorType))) {
		return fmt.Errorf("%s APIs cannot execute side-effecting SQL", strings.ToUpper(strings.TrimSpace(api.Method)))
	}
	if !api.IsWrite && writeStmt.MatchString(connectors.StripDialect(q, connectors.Dialect(api.ConnectorType))) {
		return fmt.Errorf("write statements are not allowed for read-only APIs")
	}
	if hasMultipleStatements(q, api.ConnectorType) {
		return fmt.Errorf("multiple SQL statements are not allowed")
	}

	declared := map[string]bool{}
	for _, p := range params {
		declared[p.Name] = true
	}
	used := ExtractParams(q, api.ConnectorType)
	for _, name := range used {
		if !declared[name] {
			return fmt.Errorf("query references undeclared parameter :%s", name)
		}
	}
	for name := range declared {
		if !containsParam(used, name) {
			return fmt.Errorf("declared parameter %q is not used in the query", name)
		}
	}
	if api.DefaultLimit <= 0 {
		return fmt.Errorf("default_limit must be greater than zero")
	}
	if err := validateParameterRegexes(params); err != nil {
		return err
	}
	return nil
}

func isReadOnlyOperation(api models.APIDefinition) bool {
	op := models.OperationFromMethod(api.Method)
	return op == models.OperationRead || !api.IsWrite
}

func validateParameterRegexes(params []models.APIParameter) error {
	for _, p := range params {
		if p.ValidationRule == nil || *p.ValidationRule == "" {
			continue
		}
		if _, err := regexp.Compile(*p.ValidationRule); err != nil {
			return fmt.Errorf("parameter %q has invalid validation_rule: %w", p.Name, err)
		}
	}
	return nil
}

// ValidateForExecution performs the safety checks needed before admin-side query
// test/explain execution. Unlike publish validation, this does not require a full
// parameter declaration set because test requests provide sample values directly.
func ValidateForExecution(api models.APIDefinition) error {
	if _, ok, apiErr := QueryBuilderFromAPI(api); apiErr != nil {
		return errors.New(apiErr.Message)
	} else if ok {
		if api.DefaultLimit <= 0 {
			return fmt.Errorf("default_limit must be greater than zero")
		}
		return nil
	}

	q := strings.TrimSpace(api.QueryTemplate)
	if q == "" {
		return fmt.Errorf("query template is empty")
	}
	operation := api.OperationType()
	if operation == models.OperationRead && writeStmt.MatchString(connectors.StripDialect(q, connectors.Dialect(api.ConnectorType))) {
		return fmt.Errorf("%s APIs cannot execute side-effecting SQL", strings.ToUpper(strings.TrimSpace(api.Method)))
	}
	if !api.IsWrite && writeStmt.MatchString(connectors.StripDialect(q, connectors.Dialect(api.ConnectorType))) {
		return fmt.Errorf("write statements are not allowed for read-only APIs")
	}
	if hasMultipleStatements(q, api.ConnectorType) {
		return fmt.Errorf("multiple SQL statements are not allowed")
	}
	if api.DefaultLimit <= 0 {
		return fmt.Errorf("default_limit must be greater than zero")
	}
	return nil
}

// hasMultipleStatements reports whether the SQL contains more than one statement
// (ignoring a single trailing semicolon).
func hasMultipleStatements(q string, dialect string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(q), ";")
	// Re-run the binding tokenizer's awareness by stripping quoted strings first.
	stripped := connectors.StripDialect(trimmed, connectors.Dialect(dialect))
	return strings.Contains(stripped, ";")
}

// ExtractParams returns the ordered, de-duplicated :param names used in the SQL,
// reusing the connector binder's parser so the rules match exactly.
func ExtractParams(q string, dialect string) []string {
	return connectors.ExtractParamNamesDialect(q, connectors.Dialect(dialect))
}

func containsParam(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

// (deleted stripDialect from validate.go)
