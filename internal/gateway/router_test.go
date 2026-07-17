package gateway

import (
	"testing"

	"github.com/ddag/ddag/internal/models"
)

func TestRouterMatch(t *testing.T) {
	r := NewRouter()
	r.Build([]models.APIDefinition{
		{Method: "GET", Path: "/api/v1/brim/sites/{site_id}"},
		{Method: "POST", Path: "/api/v1/brim/workorders/search"},
		{Method: "QUERY", Path: "/api/v1/brim/sites/search"},
		{Method: "GET", Path: "/api/v1/brim/sites/active"}, // literal that overlaps the param route
	})
	if r.Count() != 4 {
		t.Fatalf("want 4 routes, got %d", r.Count())
	}

	// RFC 10008 QUERY method routes independently and can carry a JSON body.
	if rt, _, ok := r.Match("QUERY", "/api/v1/brim/sites/search"); !ok || rt.API.Method != "QUERY" {
		t.Fatalf("expected QUERY route, got %+v ok=%v", rt, ok)
	}

	// Param match.
	rt, params, ok := r.Match("GET", "/api/v1/brim/sites/ABC123")
	if !ok {
		t.Fatal("expected match for sites/ABC123")
	}
	if params["site_id"] != "ABC123" {
		t.Fatalf("site_id = %q", params["site_id"])
	}
	if rt.API.Path != "/api/v1/brim/sites/{site_id}" {
		t.Fatalf("matched wrong route: %s", rt.API.Path)
	}

	// Literal beats param.
	rt, _, ok = r.Match("GET", "/api/v1/brim/sites/active")
	if !ok || rt.API.Path != "/api/v1/brim/sites/active" {
		t.Fatalf("expected literal route to win, got %+v ok=%v", rt, ok)
	}

	// Method mismatch.
	if _, _, ok := r.Match("DELETE", "/api/v1/brim/sites/ABC123"); ok {
		t.Fatal("DELETE should not match a GET route")
	}

	// Unknown path.
	if _, _, ok := r.Match("GET", "/nope"); ok {
		t.Fatal("unknown path should not match")
	}
}
