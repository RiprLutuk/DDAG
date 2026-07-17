package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONSetsJSONSafetyHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, map[string]string{"status": "ok"})

	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestUpstreamTimeoutsUseGatewayTimeout(t *testing.T) {
	for _, code := range []string{CodeQueryTimeout, CodeDBQueryTimeout} {
		if got := NewError(code, "timeout").HTTPStatus(); got != http.StatusGatewayTimeout {
			t.Fatalf("%s status = %d, want %d", code, got, http.StatusGatewayTimeout)
		}
	}
}
