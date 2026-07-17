// Package gateway implements the dynamic data-plane primitives: a route table
// built from published API definitions, parameter resolution + validation, and
// query-template safety checks. The orchestration (auth → policy → cache →
// connector) lives in the api-gateway command.
package gateway

import (
	"strings"
	"sync"

	"github.com/ddag/ddag/internal/models"
)

// Route is a compiled, matchable API definition.
type Route struct {
	API      models.APIDefinition
	segments []segment
}

type segment struct {
	literal string
	param   string // non-empty when this segment is a {param}
}

// Router holds the live route table. It is safe for concurrent use and is
// swapped atomically when API definitions change.
type Router struct {
	mu     sync.RWMutex
	routes []*Route
}

// NewRouter builds an empty router.
func NewRouter() *Router { return &Router{} }

// Build replaces the route table from a set of API definitions.
func (r *Router) Build(apis []models.APIDefinition) {
	routes := make([]*Route, 0, len(apis))
	for _, a := range apis {
		routes = append(routes, &Route{API: a, segments: parsePath(a.Path)})
	}
	r.mu.Lock()
	r.routes = routes
	r.mu.Unlock()
}

// Count returns the number of routes.
func (r *Router) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes)
}

// APIs returns a snapshot of the compiled API definitions.
func (r *Router) APIs() []models.APIDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	apis := make([]models.APIDefinition, 0, len(r.routes))
	for _, rt := range r.routes {
		apis = append(apis, rt.API)
	}
	return apis
}

// Match finds the route for a method+path, returning the route and the extracted
// path parameters. Literal segments take priority over parameter segments.
func (r *Router) Match(method, path string) (*Route, map[string]string, bool) {
	reqSegs := splitPath(path)
	r.mu.RLock()
	defer r.mu.RUnlock()

	var paramMatch *Route
	var paramParams map[string]string
	for _, rt := range r.routes {
		if !strings.EqualFold(rt.API.Method, method) {
			continue
		}
		if len(rt.segments) != len(reqSegs) {
			continue
		}
		params := map[string]string{}
		ok := true
		hasParam := false
		for i, seg := range rt.segments {
			if seg.param != "" {
				params[seg.param] = reqSegs[i]
				hasParam = true
				continue
			}
			if seg.literal != reqSegs[i] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if !hasParam {
			return rt, params, true // exact literal match wins immediately
		}
		if paramMatch == nil {
			paramMatch = rt
			paramParams = params
		}
	}
	if paramMatch != nil {
		return paramMatch, paramParams, true
	}
	return nil, nil, false
}

func parsePath(path string) []segment {
	parts := splitPath(path)
	segs := make([]segment, len(parts))
	for i, p := range parts {
		if len(p) >= 2 && p[0] == '{' && p[len(p)-1] == '}' {
			segs[i] = segment{param: p[1 : len(p)-1]}
		} else {
			segs[i] = segment{literal: p}
		}
	}
	return segs
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}
