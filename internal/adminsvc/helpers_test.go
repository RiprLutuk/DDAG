package adminsvc

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRejectsTrailingJSONValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"first"} {"name":"second"}`))
	rec := httptest.NewRecorder()
	var dst struct {
		Name string `json:"name"`
	}

	if decode(rec, req, &dst) {
		t.Fatal("expected multiple JSON values to be rejected")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDecodeRejectsOversizedJSON(t *testing.T) {
	body := []byte(`{"payload":"` + strings.Repeat("x", 2<<20) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	var dst map[string]any

	if decode(rec, req, &dst) {
		t.Fatal("expected oversized JSON to be rejected")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestDecodeAcceptsSingleJSONValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"valid"}`))
	rec := httptest.NewRecorder()
	var dst struct {
		Name string `json:"name"`
	}

	if !decode(rec, req, &dst) {
		t.Fatal("expected a valid single JSON object to decode")
	}
	if dst.Name != "valid" {
		t.Fatalf("name = %q, want valid", dst.Name)
	}
}
