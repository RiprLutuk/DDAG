package gatewaysvc

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ddag/ddag/internal/httpx"
)

func TestReadBodyAcceptsValidQUERYJSON(t *testing.T) {
	req := httptest.NewRequest("QUERY", "/search", strings.NewReader(`{"status":"active"}`))
	req.Header.Set("Content-Type", "application/json")
	body, apiErr := (&service{}).readBody(req)
	if apiErr != nil || body["status"] != "active" {
		t.Fatalf("body=%v err=%v", body, apiErr)
	}
}

func TestReadBodyRejectsMalformedQUERYJSON(t *testing.T) {
	req := httptest.NewRequest("QUERY", "/search", strings.NewReader(`{"status":`))
	req.Header.Set("Content-Type", "application/json")
	_, apiErr := (&service{}).readBody(req)
	if apiErr == nil || apiErr.Code != httpx.CodeValidation {
		t.Fatalf("err=%v, want %s", apiErr, httpx.CodeValidation)
	}
}

func TestReadBodyRejectsTrailingQUERYJSON(t *testing.T) {
	req := httptest.NewRequest("QUERY", "/search", strings.NewReader(`{"a":1} {"b":2}`))
	req.Header.Set("Content-Type", "application/json")
	_, apiErr := (&service{}).readBody(req)
	if apiErr == nil || apiErr.Code != httpx.CodeValidation {
		t.Fatalf("err=%v, want %s", apiErr, httpx.CodeValidation)
	}
}

func TestReadBodyRejectsOversizedQUERYJSON(t *testing.T) {
	payload := `{"value":"` + strings.Repeat("x", (1<<20)+1) + `"}`
	req := httptest.NewRequest("QUERY", "/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	_, apiErr := (&service{}).readBody(req)
	if apiErr == nil || apiErr.Code != httpx.CodePayloadTooLarge {
		t.Fatalf("err=%v, want %s", apiErr, httpx.CodePayloadTooLarge)
	}
}
