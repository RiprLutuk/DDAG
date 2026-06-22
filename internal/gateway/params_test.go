package gateway

import (
	"testing"

	"github.com/ddag/ddag/internal/models"
)

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

// p builds a one-parameter slice for the resolver tests.
func p(name, source, ptype string, required bool) []models.APIParameter {
	return []models.APIParameter{{Name: name, Source: source, ParamType: ptype, Required: required}}
}

func TestResolveParams_PathAndCoerce(t *testing.T) {
	out, apiErr := ResolveParams(p("id", "path", "int", true), map[string]string{"id": "42"}, nil, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if out["id"] != int64(42) {
		t.Fatalf("id = %#v, want int64(42)", out["id"])
	}
}

func TestResolveParams_MissingRequired(t *testing.T) {
	_, apiErr := ResolveParams(p("id", "path", "string", true), map[string]string{}, nil, nil)
	if apiErr == nil {
		t.Fatal("expected error for missing required param")
	}
}

func TestResolveParams_DefaultApplied(t *testing.T) {
	params := p("status", "query", "string", false)
	params[0].DefaultValue = strp("OPEN")
	out, apiErr := ResolveParams(params, nil, map[string][]string{}, nil)
	if apiErr != nil {
		t.Fatalf("err: %v", apiErr)
	}
	if out["status"] != "OPEN" {
		t.Fatalf("status = %v, want OPEN", out["status"])
	}
}

func TestResolveParams_MaxLength(t *testing.T) {
	params := p("code", "query", "string", true)
	params[0].MaxLength = intp(3)
	_, apiErr := ResolveParams(params, nil, map[string][]string{"code": {"toolong"}}, nil)
	if apiErr == nil {
		t.Fatal("expected max-length violation")
	}
}

func TestResolveParams_BadIntRejected(t *testing.T) {
	_, apiErr := ResolveParams(p("n", "query", "int", true), nil, map[string][]string{"n": {"abc"}}, nil)
	if apiErr == nil {
		t.Fatal("expected int coercion error")
	}
}

func TestResolveParams_BodyParam(t *testing.T) {
	out, apiErr := ResolveParams(p("status", "body", "string", true), nil, nil, map[string]any{"status": "CLOSED"})
	if apiErr != nil {
		t.Fatalf("err: %v", apiErr)
	}
	if out["status"] != "CLOSED" {
		t.Fatalf("status = %v", out["status"])
	}
}
