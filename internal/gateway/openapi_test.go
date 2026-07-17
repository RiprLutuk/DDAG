package gateway

import (
	"testing"

	"github.com/ddag/ddag/internal/models"
)

func TestGenerateOpenAPISpecFromAPIs(t *testing.T) {
	spec := GenerateOpenAPISpec([]models.APIDefinition{
		{
			Name:          "Get site",
			Namespace:     "brim",
			Path:          "/api/v1/sites/{site_id}",
			Method:        "GET",
			Description:   "Fetch one site",
			RequiredScope: "brim.site.read",
			Parameters: []models.APIParameter{
				{Name: "site_id", Source: "path", ParamType: "string", Required: true},
				{Name: "include_inactive", Source: "query", ParamType: "bool", Required: false},
			},
		},
	})

	if spec.OpenAPI != "3.0.3" {
		t.Fatalf("openapi = %q", spec.OpenAPI)
	}
	path := spec.Paths["/api/v1/sites/{site_id}"]
	op, ok := path["get"]
	if !ok {
		t.Fatalf("missing get operation: %#v", path)
	}
	if op.Summary != "Get site" || op.Description != "Fetch one site" {
		t.Fatalf("operation = %+v", op)
	}
	if got := op.Security[0]["bearerAuth"][0]; got != "brim.site.read" {
		t.Fatalf("scope = %q", got)
	}
	if len(op.Parameters) != 2 {
		t.Fatalf("parameters = %#v", op.Parameters)
	}
	if op.Parameters[0].In != "path" || !op.Parameters[0].Required {
		t.Fatalf("path parameter = %+v", op.Parameters[0])
	}
}

func TestGenerateOpenAPISpecQUERYAsPost(t *testing.T) {
	spec := GenerateOpenAPISpec([]models.APIDefinition{
		{
			Name:          "Search sites",
			Namespace:     "brim",
			Path:          "/api/v1/sites/search",
			Method:        "QUERY",
			Description:   "Search sites with JSON body",
			RequiredScope: "brim.site.search",
		},
	})

	if spec.OpenAPI != "3.0.3" {
		t.Fatalf("openapi = %q", spec.OpenAPI)
	}
	path := spec.Paths["/api/v1/sites/search"]
	if path == nil {
		t.Fatalf("missing path")
	}

	// QUERY maps to POST in OpenAPI 3.0
	op, ok := path["post"]
	if !ok {
		t.Fatalf("missing post operation for QUERY: %#v", path)
	}
	if op.Summary != "Search sites" || op.Description != "Search sites with JSON body" {
		t.Fatalf("operation = %+v", op)
	}

	// Extension must carry original method
	if got := op.DDAGMethod; got != "QUERY" {
		t.Fatalf("x-ddag-http-method = %q, want %q", got, "QUERY")
	}

	// Security scope preserved
	if got := op.Security[0]["bearerAuth"][0]; got != "brim.site.search" {
		t.Fatalf("scope = %q", got)
	}
}
