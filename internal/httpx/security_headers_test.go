package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want %q", got, "DENY")
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want %q", got, "strict-origin-when-cross-origin")
	}
	if got := rec.Header().Get("X-Permitted-Cross-Domain-Policies"); got != "none" {
		t.Errorf("X-Permitted-Cross-Domain-Policies = %q, want %q", got, "none")
	}
}

func TestWWWAuthenticateOn401(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	Error(rec, req, NewError(CodeUnauthorized, "invalid credentials"))

	if got := rec.Header().Get("WWW-Authenticate"); got != `Bearer realm="DDAG API"` {
		t.Errorf("WWW-Authenticate = %q, want %q", got, `Bearer realm="DDAG API"`)
	}
}
