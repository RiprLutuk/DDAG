package adminsvc

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

func canPublishAPIStatus(status string) error {
	return canTransitionAPIStatus(status, "PUBLISHED")
}

// canTransitionAPIStatus enforces DDAG's one-way API lifecycle. Terminal
// statuses intentionally cannot be revived: operators must create a new draft
// from the desired definition and send it through review again.
func canTransitionAPIStatus(from, to string) error {
	allowed := map[string]map[string]bool{
		"DRAFT":     {"REVIEW": true},
		"REVIEW":    {"APPROVED": true},
		"APPROVED":  {"PUBLISHED": true},
		"PUBLISHED": {"DEPRECATED": true, "ARCHIVED": true, "DISABLED": true},
	}
	if allowed[from][to] {
		return nil
	}
	return fmt.Errorf("API cannot transition from %s to %s", from, to)
}

type jsonChange struct {
	Before any `json:"before"`
	After  any `json:"after"`
}

type jsonDiff struct {
	Added   map[string]any        `json:"added"`
	Removed map[string]any        `json:"removed"`
	Changed map[string]jsonChange `json:"changed"`
}

func diffJSON(before, after json.RawMessage) (jsonDiff, error) {
	var b, a any
	if len(before) == 0 {
		before = json.RawMessage(`{}`)
	}
	if len(after) == 0 {
		after = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(before, &b); err != nil {
		return jsonDiff{}, err
	}
	if err := json.Unmarshal(after, &a); err != nil {
		return jsonDiff{}, err
	}
	out := jsonDiff{Added: map[string]any{}, Removed: map[string]any{}, Changed: map[string]jsonChange{}}
	diffValue("", b, a, &out)
	return out, nil
}

func diffValue(path string, before, after any, out *jsonDiff) {
	bm, bok := before.(map[string]any)
	am, aok := after.(map[string]any)
	if bok && aok {
		keys := map[string]bool{}
		for k := range bm {
			keys[k] = true
		}
		for k := range am {
			keys[k] = true
		}
		ordered := make([]string, 0, len(keys))
		for k := range keys {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		for _, k := range ordered {
			child := k
			if path != "" {
				child = path + "." + k
			}
			bv, be := bm[k]
			av, ae := am[k]
			switch {
			case !be && ae:
				out.Added[child] = av
			case be && !ae:
				out.Removed[child] = bv
			default:
				diffValue(child, bv, av, out)
			}
		}
		return
	}
	if !reflect.DeepEqual(before, after) {
		if path == "" {
			path = "$"
		}
		out.Changed[path] = jsonChange{Before: before, After: after}
	}
}

type promotionBundle struct {
	Version string         `json:"version"`
	APIs    []promotionAPI `json:"apis"`
}

type promotionAPI struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

type promotionDryRunResult struct {
	Valid    bool           `json:"valid"`
	Errors   []string       `json:"errors"`
	Warnings []string       `json:"warnings"`
	Counts   map[string]int `json:"counts"`
}

func validatePromotionBundle(bundle promotionBundle) promotionDryRunResult {
	result := promotionDryRunResult{Valid: true, Counts: map[string]int{"apis": len(bundle.APIs)}}
	seen := map[string]string{}
	for i, a := range bundle.APIs {
		method := strings.ToUpper(strings.TrimSpace(a.Method))
		path := strings.TrimSpace(a.Path)
		name := strings.TrimSpace(a.Name)
		if name == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("apis[%d]: name is required", i))
		}
		if method != "GET" && method != "QUERY" && method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
			result.Errors = append(result.Errors, fmt.Sprintf("apis[%d]: unsupported method %q", i, method))
		}
		if !strings.HasPrefix(path, "/") {
			result.Errors = append(result.Errors, fmt.Sprintf("apis[%d]: path must start with /", i))
		}
		key := method + " " + path
		if prev, ok := seen[key]; ok {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate route %s used by %q and %q", key, prev, name))
		} else {
			seen[key] = name
		}
	}
	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result
}

func decodePromotionBundle(raw json.RawMessage) (promotionBundle, error) {
	if len(raw) == 0 {
		return promotionBundle{}, errors.New("promotion bundle is required")
	}
	var bundle promotionBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return promotionBundle{}, err
	}
	return bundle, nil
}
