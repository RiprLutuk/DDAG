package connectors

import (
	"fmt"
	"regexp"
	"strings"
)

var orderByRE = regexp.MustCompile(`(?is)\border\s+by\b`)

// ApplyPagination appends dialect-specific SQL-level pagination and returns the
// extra bound args. Limit <= 0 leaves the query unchanged.
func ApplyPagination(query string, args []any, dbType string, limit, offset int) (string, []any) {
	if limit <= 0 {
		return query, args
	}
	if offset < 0 {
		offset = 0
	}
	base := strings.TrimSpace(query)
	base = strings.TrimSuffix(base, ";")
	switch strings.ToLower(dbType) {
	case "postgres":
		n := len(args)
		return fmt.Sprintf("%s LIMIT $%d OFFSET $%d", base, n+1, n+2), append(args, limit, offset)
	case "mysql":
		return base + " LIMIT ?, ?", append(args, offset, limit)
	case "sqlserver":
		n := len(args)
		if !orderByRE.MatchString(base) {
			base += " ORDER BY (SELECT 1)"
		}
		return fmt.Sprintf("%s OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY", base, n+1, n+2), append(args, offset, limit)
	case "oracle":
		n := len(args)
		return fmt.Sprintf("%s OFFSET :%d ROWS FETCH NEXT :%d ROWS ONLY", base, n+1, n+2), append(args, offset, limit)
	default:
		return base, args
	}
}
