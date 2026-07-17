package gateway

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
)

var (
	identifierRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)
	aliasRE      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	selectAggRE  = regexp.MustCompile(`^(COUNT|SUM|AVG|MIN|MAX)\(([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?|\*)\)( AS [A-Za-z_][A-Za-z0-9_]*)?$`)
	selectColRE  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?( AS [A-Za-z_][A-Za-z0-9_]*)?$`)
)

type queryBuilderEnvelope struct {
	QueryBuilder *QueryBuilderConfig `json:"query_builder"`
}

type QueryBuilderConfig struct {
	BaseTable       string           `json:"base_table"`
	Select          []string         `json:"select"`
	Joins           []JoinConfig     `json:"joins"`
	GroupBy         []string         `json:"group_by"`
	Filters         []FilterConfig   `json:"filters"`
	SortableColumns []SortableColumn `json:"sortable_columns"`
	DefaultSort     string           `json:"default_sort"`
}

type JoinConfig struct {
	Type  string        `json:"type"`
	Table string        `json:"table"`
	Alias string        `json:"alias"`
	On    JoinCondition `json:"on"`
}

type JoinCondition struct {
	Left     string `json:"left"`
	Operator string `json:"operator"`
	Right    string `json:"right"`
}

type FilterConfig struct {
	Name      string   `json:"name"`
	Column    string   `json:"column"`
	Type      string   `json:"type"`
	Operators []string `json:"operators"`
}

type SortableColumn struct {
	Name   string `json:"name"`
	Column string `json:"column"`
}

type BuiltQuery struct {
	SQL    string
	Params map[string]any
}

func BuildDynamicQuery(api models.APIDefinition, query url.Values, params map[string]any) (BuiltQuery, *httpx.APIError) {
	cfg, ok, apiErr := QueryBuilderFromAPI(api)
	if apiErr != nil {
		return BuiltQuery{}, apiErr
	}
	out := BuiltQuery{SQL: api.QueryTemplate, Params: copyParams(params)}
	if !ok {
		return out, nil
	}
	sql, generated, err := cfg.Build(query)
	if err != nil {
		return BuiltQuery{}, httpx.NewError(httpx.CodeQueryValidationFailed, err.Error())
	}
	for k, v := range generated {
		out.Params[k] = v
	}
	out.SQL = sql
	return out, nil
}

func QueryBuilderFromAPI(api models.APIDefinition) (*QueryBuilderConfig, bool, *httpx.APIError) {
	if len(api.ResponseMapping) == 0 || string(api.ResponseMapping) == "null" {
		return nil, false, nil
	}
	var env queryBuilderEnvelope
	if err := json.Unmarshal(api.ResponseMapping, &env); err != nil {
		return nil, false, httpx.NewError(httpx.CodeQueryValidationFailed, "query builder config is invalid JSON")
	}
	if env.QueryBuilder == nil {
		return nil, false, nil
	}
	if err := env.QueryBuilder.Validate(); err != nil {
		return nil, false, httpx.NewError(httpx.CodeQueryValidationFailed, err.Error())
	}
	return env.QueryBuilder, true, nil
}

func (c *QueryBuilderConfig) Build(query url.Values) (string, map[string]any, error) {
	if err := c.Validate(); err != nil {
		return "", nil, err
	}
	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(strings.Join(c.Select, ", "))
	b.WriteString(" FROM ")
	b.WriteString(c.BaseTable)
	for _, j := range c.Joins {
		b.WriteByte(' ')
		b.WriteString(strings.ToUpper(j.Type))
		b.WriteString(" JOIN ")
		b.WriteString(j.Table)
		if j.Alias != "" {
			b.WriteByte(' ')
			b.WriteString(j.Alias)
		}
		b.WriteString(" ON ")
		b.WriteString(j.On.Left)
		b.WriteByte(' ')
		b.WriteString(j.On.Operator)
		b.WriteByte(' ')
		b.WriteString(j.On.Right)
	}

	where, params, err := c.where(query)
	if err != nil {
		return "", nil, err
	}
	if len(where) > 0 {
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(where, " AND "))
	}
	if len(c.GroupBy) > 0 {
		b.WriteString(" GROUP BY ")
		b.WriteString(strings.Join(c.GroupBy, ", "))
	}
	order, err := c.orderBy(query.Get("sort"))
	if err != nil {
		return "", nil, err
	}
	if order != "" {
		b.WriteString(" ORDER BY ")
		b.WriteString(order)
	}
	return b.String(), params, nil
}

func (c *QueryBuilderConfig) Validate() error {
	if !safeIdent(c.BaseTable) {
		return fmt.Errorf("base_table is not whitelisted")
	}
	if len(c.Select) == 0 {
		return fmt.Errorf("select must contain at least one column")
	}
	for _, sel := range c.Select {
		if !safeSelect(sel) {
			return fmt.Errorf("select column %q is not whitelisted", sel)
		}
	}
	for _, g := range c.GroupBy {
		if !safeIdent(g) {
			return fmt.Errorf("group_by column %q is not whitelisted", g)
		}
	}
	for _, j := range c.Joins {
		t := strings.ToLower(j.Type)
		if t != "inner" && t != "left" {
			return fmt.Errorf("join type %q is not allowed", j.Type)
		}
		if !safeIdent(j.Table) || (j.Alias != "" && !aliasRE.MatchString(j.Alias)) ||
			!safeIdent(j.On.Left) || !safeIdent(j.On.Right) || j.On.Operator != "=" {
			return fmt.Errorf("join config is not safe")
		}
	}
	for _, f := range c.Filters {
		if f.Name == "" || !safeIdent(f.Column) {
			return fmt.Errorf("filter %q is not whitelisted", f.Name)
		}
	}
	for _, s := range c.SortableColumns {
		if s.Name == "" || !safeIdent(s.Column) {
			return fmt.Errorf("sort column %q is not whitelisted", s.Name)
		}
	}
	return nil
}

func (c *QueryBuilderConfig) where(query url.Values) ([]string, map[string]any, error) {
	params := map[string]any{}
	filters := map[string]FilterConfig{}
	for _, f := range c.Filters {
		filters[f.Name] = f
	}
	clauses := []string{}
	for name, values := range query {
		if ignoredQueryParam(name) || len(values) == 0 || values[0] == "" {
			continue
		}
		if _, ok := filters[name]; !ok {
			return nil, nil, fmt.Errorf("filter column %q is not allowed", name)
		}
	}
	for _, f := range c.Filters {
		values := query[f.Name]
		if len(values) == 0 || values[0] == "" {
			continue
		}
		op, raw := splitOperator(values[0])
		if op == "" {
			op = "eq"
			raw = values[0]
		}
		if !operatorAllowed(f, op) {
			return nil, nil, fmt.Errorf("filter operator %q is not allowed for %q", op, f.Name)
		}
		clause, err := buildWhereClause(f, op, raw, params)
		if err != nil {
			return nil, nil, err
		}
		if clause != "" {
			clauses = append(clauses, clause)
		}
	}
	return clauses, params, nil
}

func buildWhereClause(f FilterConfig, op, raw string, params map[string]any) (string, error) {
	base := "ddag_filter_" + sanitizeName(f.Name)
	add := func(v any) string {
		key := fmt.Sprintf("%s_%d", base, len(paramsForPrefix(params, base)))
		params[key] = v
		return ":" + key
	}
	switch op {
	case "eq":
		return f.Column + " = " + add(raw), nil
	case "neq":
		return f.Column + " <> " + add(raw), nil
	case "like":
		return f.Column + " LIKE " + add("%"+raw+"%"), nil
	case "gt":
		return f.Column + " > " + add(raw), nil
	case "gte":
		return f.Column + " >= " + add(raw), nil
	case "lt":
		return f.Column + " < " + add(raw), nil
	case "lte":
		return f.Column + " <= " + add(raw), nil
	case "between":
		parts := splitCSV(raw)
		if len(parts) != 2 {
			return "", fmt.Errorf("between filter %q requires two values", f.Name)
		}
		return f.Column + " BETWEEN " + add(parts[0]) + " AND " + add(parts[1]), nil
	case "in":
		parts := splitCSV(raw)
		if len(parts) == 0 {
			return "", fmt.Errorf("in filter %q requires at least one value", f.Name)
		}
		holders := make([]string, 0, len(parts))
		for _, p := range parts {
			holders = append(holders, add(p))
		}
		return f.Column + " IN (" + strings.Join(holders, ", ") + ")", nil
	case "isnull":
		if strings.EqualFold(strings.TrimSpace(raw), "true") {
			return f.Column + " IS NULL", nil
		}
		return f.Column + " IS NOT NULL", nil
	default:
		return "", fmt.Errorf("filter operator %q is not allowed", op)
	}
}

func (c *QueryBuilderConfig) orderBy(raw string) (string, error) {
	if raw == "" {
		raw = c.DefaultSort
	}
	if raw == "" {
		return "", nil
	}
	sortable := map[string]string{}
	for _, s := range c.SortableColumns {
		sortable[s.Name] = s.Column
	}
	var parts []string
	for _, token := range splitCSV(raw) {
		dir := "ASC"
		name := token
		if strings.HasPrefix(token, "-") {
			dir = "DESC"
			name = strings.TrimPrefix(token, "-")
		}
		col, ok := sortable[name]
		if !ok {
			return "", fmt.Errorf("sort column %q is not allowed", name)
		}
		parts = append(parts, col+" "+dir)
	}
	return strings.Join(parts, ", "), nil
}

func operatorAllowed(f FilterConfig, op string) bool {
	for _, allowed := range f.Operators {
		if allowed == op {
			return true
		}
	}
	return len(f.Operators) == 0 && op == "eq"
}

func splitOperator(v string) (string, string) {
	op, raw, ok := strings.Cut(v, ":")
	if !ok {
		return "", v
	}
	return strings.ToLower(strings.TrimSpace(op)), raw
}

func splitCSV(v string) []string {
	raw := strings.Split(v, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func safeIdent(v string) bool { return identifierRE.MatchString(v) }

func safeSelect(v string) bool {
	up := strings.ToUpper(strings.TrimSpace(v))
	return selectColRE.MatchString(v) || selectAggRE.MatchString(up)
}

func ignoredQueryParam(name string) bool {
	switch name {
	case "page", "limit", "offset", "sort":
		return true
	default:
		return false
	}
}

func copyParams(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sanitizeName(v string) string {
	v = strings.ReplaceAll(v, ".", "_")
	v = strings.ReplaceAll(v, "-", "_")
	return v
}

func paramsForPrefix(params map[string]any, prefix string) []string {
	out := []string{}
	for k := range params {
		if strings.HasPrefix(k, prefix+"_") {
			out = append(out, k)
		}
	}
	return out
}
