package connectors

import (
	"fmt"
	"strings"
)

// Dialect identifies the native SQL binder syntax of a connector.
type Dialect string

const (
	DialectPostgres  Dialect = "postgres"
	DialectMySQL     Dialect = "mysql"
	DialectSQLServer Dialect = "sqlserver"
	DialectOracle    Dialect = "oracle"
)

func PlaceholderForDialect(d Dialect) Placeholder {
	switch d {
	case DialectPostgres:
		return PlaceholderDollar
	case DialectMySQL:
		return PlaceholderQuestion
	case DialectSQLServer:
		return PlaceholderAtP
	case DialectOracle:
		return PlaceholderColon
	default:
		return PlaceholderQuestion
	}
}

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
	return bindDialect(template, params, ph, "")
}

// BindDialect rewrites named parameters while respecting dialect-specific quoted regions.
func BindDialect(template string, params map[string]any, dialect Dialect) (string, []any, error) {
	return bindDialect(template, params, PlaceholderForDialect(dialect), dialect)
}

func bindDialect(template string, params map[string]any, ph Placeholder, dialect Dialect) (string, []any, error) {
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
		case dialect == DialectPostgres && c == '$':
			if end := dollarQuoteEnd(runes, i); end > i {
				out = append(out, string(runes[i:end])...)
				i = end
				continue
			}
		case dialect == DialectMySQL && c == '`':
			i = copyDelimited(runes, i, '`', &out)
		case dialect == DialectSQLServer && c == '[':
			i = copyBracketIdentifier(runes, i, &out)
		case c == '"':
			i = copyDelimited(runes, i, '"', &out)
		case dialect == DialectMySQL && c == '#':
			for i < L {
				out = append(out, string(runes[i])...)
				if runes[i] == '\n' {
					i++
					break
				}
				i++
			}
		case dialect == DialectOracle && c == 'q' && i+2 < L && runes[i+1] == '\'':
			if end := oracleQuoteEnd(runes, i); end > i {
				out = append(out, string(runes[i:end])...)
				i = end
				continue
			}
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

// ExtractParamNamesDialect returns the ordered, de-duplicated :param names
// referenced by a template, using dialect-aware parsing rules.
func ExtractParamNamesDialect(template string, dialect Dialect) []string {
	var order []string
	seen := map[string]bool{}
	runes := []rune(template)
	i, L := 0, len(runes)
	for i < L {
		c := runes[i]
		switch {
		case c == '$':
			if end := dollarQuoteEnd(runes, i); end > i {
				i = end
				continue
			}
		case dialect == DialectMySQL && c == '`':
			i = skipDelimited(runes, i, '`')
			continue
		case dialect == DialectSQLServer && c == '[':
			i = skipBracketIdentifier(runes, i)
			continue
		case c == '"':
			i = skipDelimited(runes, i, '"')
			continue
		case dialect == DialectMySQL && c == '#':
			for i < L && runes[i] != '\n' {
				i++
			}
			continue
		case dialect == DialectOracle && c == 'q' && i+2 < L && runes[i+1] == '\'':
			if end := oracleQuoteEnd(runes, i); end > i {
				i = end
				continue
			}
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

func skipDelimited(runes []rune, start int, quote rune) int {
	for i := start + 1; i < len(runes); i++ {
		if runes[i] == quote {
			if i+1 < len(runes) && runes[i+1] == quote {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(runes)
}

func skipBracketIdentifier(runes []rune, start int) int {
	for i := start + 1; i < len(runes); i++ {
		if runes[i] == ']' {
			if i+1 < len(runes) && runes[i+1] == ']' {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(runes)
}

func dollarQuoteEnd(runes []rune, start int) int {
	if start >= len(runes) || runes[start] != '$' {
		return 0
	}
	endTag := start + 1
	for endTag < len(runes) && (isIdentPart(runes[endTag]) || runes[endTag] == '$') {
		endTag++
	}
	if endTag == start+1 || runes[endTag-1] != '$' {
		return 0
	}
	tag := string(runes[start:endTag])
	for i := endTag; i+len([]rune(tag)) <= len(runes); i++ {
		if string(runes[i:i+len([]rune(tag))]) == tag {
			return i + len([]rune(tag))
		}
	}
	return 0
}

func copyDelimited(runes []rune, start int, quote rune, out *[]byte) int {
	for i := start; i < len(runes); i++ {
		*out = append(*out, string(runes[i])...)
		if i > start && runes[i] == quote {
			if i+1 < len(runes) && runes[i+1] == quote {
				*out = append(*out, string(quote)...)
				i++
				continue
			}
			return i + 1
		}
	}
	return len(runes)
}

func copyBracketIdentifier(runes []rune, start int, out *[]byte) int {
	for i := start; i < len(runes); i++ {
		*out = append(*out, string(runes[i])...)
		if i > start && runes[i] == ']' {
			if i+1 < len(runes) && runes[i+1] == ']' {
				*out = append(*out, ']')
				i++
				continue
			}
			return i + 1
		}
	}
	return len(runes)
}

// StripDialect removes quoted literals and comments using dialect-aware rules
// so statement detection ignores semicolons inside strings. This shares the
// same skipping rules as the runtime binder to prevent parser drift.
func StripDialect(q string, dialect Dialect) string {
	var b strings.Builder
	runes := []rune(q)
	i, L := 0, len(runes)
	for i < L {
		c := runes[i]
		switch {
		// PostgreSQL dollar quote $$tag$ ... $tag$$
		case c == '$':
			if end := dollarQuoteEnd(runes, i); end > i {
				i = end
				continue
			}
		// MySQL backtick ``escaped``
		case dialect == DialectMySQL && c == '`':
			i = skipDelimited(runes, i, '`')
			continue
		// SQL Server [bracket]] escaped
		case dialect == DialectSQLServer && c == '[':
			i = skipBracketIdentifier(runes, i)
			continue
		// Double-quoted identifier "odd""escaped"
		case c == '"':
			i = skipDelimited(runes, i, '"')
			continue
		// MySQL # line comment
		case dialect == DialectMySQL && c == '#':
			for i < L && runes[i] != '\n' {
				i++
			}
			continue
		// Oracle q-quote
		case dialect == DialectOracle && c == 'q' && i+2 < L && runes[i+1] == '\'':
			if end := oracleQuoteEnd(runes, i); end > i {
				i = end
				continue
			}
		// Single-quoted string literal
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
			continue
		// -- line comment
		case c == '-' && i+1 < L && runes[i+1] == '-':
			for i < L && runes[i] != '\n' {
				i++
			}
			continue
		// /* block comment */
		case c == '/' && i+1 < L && runes[i+1] == '*':
			i += 2
			for i < L && !(runes[i] == '*' && i+1 < L && runes[i+1] == '/') {
				i++
			}
			if i < L {
				i += 2
			}
			continue
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String()
}

func oracleQuoteEnd(runes []rune, start int) int {
	if start+2 >= len(runes) || runes[start] != 'q' || runes[start+1] != '\'' {
		return 0
	}
	close := map[rune]rune{'[': ']', '(': ')', '{': '}', '<': '>'}[runes[start+2]]
	if close == 0 {
		close = runes[start+2]
	}
	for i := start + 3; i+1 < len(runes); i++ {
		if runes[i] == close && runes[i+1] == '\'' {
			return i + 2
		}
	}
	return 0
}

func isIdentStart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9')
}
