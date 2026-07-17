package gateway

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// ResolveParams reads each declared parameter from its source (path/query/body),
// applies defaults, enforces required/type/max-length/regex, and returns the
// typed, validated values ready for binding. Unknown body keys are ignored;
// values are never injected into SQL — only returned as bound args.
func ResolveParams(params []models.APIParameter, pathParams map[string]string,
	query map[string][]string, body map[string]any) (map[string]any, *httpx.APIError) {

	out := make(map[string]any, len(params))
	for _, p := range params {
		raw, present := rawValue(p, pathParams, query, body)
		if !present || raw == "" {
			if p.DefaultValue != nil {
				present = true
				raw = *p.DefaultValue
			} else if p.Required {
				return nil, httpx.NewError(httpx.CodeValidation,
					fmt.Sprintf("missing required parameter %q", p.Name))
			} else {
				out[p.Name] = nil
				continue
			}
		}

		if p.MaxLength != nil && *p.MaxLength > 0 && len(raw) > *p.MaxLength {
			return nil, httpx.NewError(httpx.CodeValidation,
				fmt.Sprintf("parameter %q exceeds max length %d", p.Name, *p.MaxLength))
		}
		if p.ValidationRule != nil && *p.ValidationRule != "" {
			re, err := regexp.Compile(*p.ValidationRule)
			if err == nil && !re.MatchString(raw) {
				return nil, httpx.NewError(httpx.CodeValidation,
					fmt.Sprintf("parameter %q failed validation", p.Name))
			}
		}

		val, err := coerce(p, raw)
		if err != nil {
			return nil, httpx.NewError(httpx.CodeValidation, err.Error())
		}
		out[p.Name] = val
	}
	return out, nil
}

// rawValue extracts the string form of a parameter from its declared source.
// Body values are stringified for uniform validation, then coerced by type.
func rawValue(p models.APIParameter, pathParams map[string]string,
	query map[string][]string, body map[string]any) (string, bool) {
	switch p.Source {
	case "path":
		v, ok := pathParams[p.Name]
		return v, ok
	case "query":
		if vs, ok := query[p.Name]; ok && len(vs) > 0 {
			return vs[0], true
		}
		return "", false
	case "body":
		if v, ok := body[p.Name]; ok && v != nil {
			return fmt.Sprintf("%v", v), true
		}
		return "", false
	}
	return "", false
}

func coerce(p models.APIParameter, raw string) (any, error) {
	switch p.ParamType {
	case "int":
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parameter %q must be an integer", p.Name)
		}
		return n, nil
	case "number":
		f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return nil, fmt.Errorf("parameter %q must be a number", p.Name)
		}
		return f, nil
	case "bool":
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("parameter %q must be a boolean", p.Name)
		}
		return b, nil
	case "uuid":
		if _, err := uuid.Parse(strings.TrimSpace(raw)); err != nil {
			return nil, fmt.Errorf("parameter %q must be a UUID", p.Name)
		}
		return raw, nil
	default: // string, date
		return raw, nil
	}
}
