package gateway

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ddag/ddag/internal/connectors"
	"github.com/ddag/ddag/internal/models"
)

// writeStmt matches SQL that mutates data; blocked unless an API is explicitly
// flagged is_write (PRD §13.5: query write disabled by default).
var writeStmt = regexp.MustCompile(`(?is)^\s*(insert|update|delete|merge|truncate|drop|alter|create|grant|revoke)\b`)

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
	if !api.IsWrite && writeStmt.MatchString(q) {
		return fmt.Errorf("write statements are not allowed for read-only APIs")
	}
	if hasMultipleStatements(q) {
		return fmt.Errorf("multiple SQL statements are not allowed")
	}

	declared := map[string]bool{}
	for _, p := range params {
		declared[p.Name] = true
	}
	used := ExtractParams(q)
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
	return nil
}

// hasMultipleStatements reports whether the SQL contains more than one statement
// (ignoring a single trailing semicolon).
func hasMultipleStatements(q string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(q), ";")
	// Re-run the binding tokenizer's awareness by stripping quoted strings first.
	stripped := stripStringsAndComments(trimmed)
	return strings.Contains(stripped, ";")
}

// ExtractParams returns the ordered, de-duplicated :param names used in the SQL,
// reusing the connector binder's parser so the rules match exactly.
func ExtractParams(q string) []string {
	return connectors.ExtractParamNames(q)
}

func containsParam(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

// stripStringsAndComments removes quoted literals and comments so statement
// detection ignores semicolons inside strings.
func stripStringsAndComments(q string) string {
	var b strings.Builder
	runes := []rune(q)
	i, L := 0, len(runes)
	for i < L {
		c := runes[i]
		switch {
		case c == '\'':
			i++
			for i < L && runes[i] != '\'' {
				i++
			}
			i++
		case c == '-' && i+1 < L && runes[i+1] == '-':
			for i < L && runes[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < L && runes[i+1] == '*':
			i += 2
			for i < L && !(runes[i] == '*' && i+1 < L && runes[i+1] == '/') {
				i++
			}
			i += 2
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String()
}
