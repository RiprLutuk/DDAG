package store

import (
	"strings"
	"testing"
)

func TestCacheRuleListQueryIncludesAPILabel(t *testing.T) {
	sql := strings.ToLower(cacheRuleListSQL(ListParams{Page: 1, Limit: 10}))
	if !strings.Contains(sql, "as api_name") || !strings.Contains(sql, "as api_method") || !strings.Contains(sql, "as api_path") {
		t.Fatalf("cache rule query must include API display fields, got %s", sql)
	}
}

func TestListPolicyQueriesUsePaginationSearchAndSafeSorting(t *testing.T) {
	p := ListParams{Page: 2, Limit: 10, Search: "client", SortBy: "applies_to", SortDir: "asc"}
	p.Normalize()

	cases := map[string]struct {
		countSQL string
		listSQL  string
		order    string
	}{
		"roles":       {roleListCountSQL(), roleListSQL(p), "order by name asc"},
		"scopes":      {scopeListCountSQL(), scopeListSQL(p), "order by scope_code asc"},
		"rate limits": {rateLimitListCountSQL(), rateLimitListSQL(p), "order by applies_to asc"},
		"ip lists":    {ipWhitelistListCountSQL(), ipWhitelistListSQL(p), "order by created_at asc"},
		"cache rules": {cacheRuleListCountSQL(), cacheRuleListSQL(p), "order by created_at asc"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			countSQL := strings.ToLower(tc.countSQL)
			listSQL := strings.ToLower(tc.listSQL)
			if !strings.Contains(countSQL, "$1='' or") {
				t.Fatalf("count SQL should search with $1, got %s", tc.countSQL)
			}
			if !strings.Contains(listSQL, "$1='' or") {
				t.Fatalf("list SQL should search with $1, got %s", tc.listSQL)
			}
			if !strings.Contains(listSQL, tc.order) {
				t.Fatalf("list SQL should contain %q, got %s", tc.order, tc.listSQL)
			}
			if !strings.Contains(listSQL, "limit $2 offset $3") {
				t.Fatalf("list SQL should paginate with limit/offset args, got %s", tc.listSQL)
			}
		})
	}
}
