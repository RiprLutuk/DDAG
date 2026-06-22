package connectors

import "fmt"

// Placeholder formats the Nth positional placeholder (1-based) for a driver.
type Placeholder func(n int) string

// Common placeholder styles.
var (
	PlaceholderDollar   Placeholder = func(n int) string { return fmt.Sprintf("$%d", n) }  // postgres
	PlaceholderQuestion Placeholder = func(n int) string { return "?" }                    // mysql
	PlaceholderAtP      Placeholder = func(n int) string { return fmt.Sprintf("@p%d", n) } // sqlserver
	PlaceholderColon    Placeholder = func(n int) string { return fmt.Sprintf(":%d", n) }  // oracle
)

// Bind rewrites a :named-parameter SQL template into the driver's positional
// placeholder style and returns the ordered argument slice. It is parser-aware:
// it ignores ':' inside single-quoted string literals, dollar-quoted strings,
// line/block comments, and the PostgreSQL '::' cast operator, so only genuine
// bind parameters are substituted. A missing parameter is an error — values are
// always passed as bound arguments, never interpolated into SQL.
func Bind(template string, params map[string]any, ph Placeholder) (string, []any, error) {
	var (
		out  []byte
		args []any
		n    int
	)
	runes := []rune(template)
	i := 0
	L := len(runes)

	for i < L {
		c := runes[i]
		switch {
		case c == '\'':
			// single-quoted string literal: copy verbatim until closing quote
			out = append(out, string(c)...)
			i++
			for i < L {
				out = append(out, string(runes[i])...)
				if runes[i] == '\'' {
					// handle escaped '' inside string
					if i+1 < L && runes[i+1] == '\'' {
						out = append(out, '\'')
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == '-' && i+1 < L && runes[i+1] == '-':
			// line comment until end of line
			for i < L && runes[i] != '\n' {
				out = append(out, string(runes[i])...)
				i++
			}
		case c == '/' && i+1 < L && runes[i+1] == '*':
			// block comment until */
			out = append(out, '/', '*')
			i += 2
			for i < L {
				if runes[i] == '*' && i+1 < L && runes[i+1] == '/' {
					out = append(out, '*', '/')
					i += 2
					break
				}
				out = append(out, string(runes[i])...)
				i++
			}
		case c == ':' && i+1 < L && runes[i+1] == ':':
			// postgres cast operator '::' — not a parameter
			out = append(out, ':', ':')
			i += 2
		case c == ':' && i+1 < L && isIdentStart(runes[i+1]):
			// :named parameter
			j := i + 1
			for j < L && isIdentPart(runes[j]) {
				j++
			}
			name := string(runes[i+1 : j])
			val, ok := params[name]
			if !ok {
				return "", nil, fmt.Errorf("missing value for parameter :%s", name)
			}
			n++
			out = append(out, ph(n)...)
			args = append(args, val)
			i = j
		default:
			out = append(out, string(c)...)
			i++
		}
	}
	return string(out), args, nil
}

// ExtractParamNames returns the ordered, de-duplicated :param names referenced
// by a template, using the exact same parsing rules as Bind. Used by the
// publish-time validator without needing parameter values.
func ExtractParamNames(template string) []string {
	var order []string
	seen := map[string]bool{}
	runes := []rune(template)
	i, L := 0, len(runes)
	for i < L {
		c := runes[i]
		switch {
		case c == '\'':
			i++
			for i < L {
				if runes[i] == '\'' {
					if i+1 < L && runes[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
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
		case c == ':' && i+1 < L && runes[i+1] == ':':
			i += 2
		case c == ':' && i+1 < L && isIdentStart(runes[i+1]):
			j := i + 1
			for j < L && isIdentPart(runes[j]) {
				j++
			}
			name := string(runes[i+1 : j])
			if !seen[name] {
				seen[name] = true
				order = append(order, name)
			}
			i = j
		default:
			i++
		}
	}
	return order
}

func isIdentStart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9')
}
