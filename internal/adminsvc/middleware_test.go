package adminsvc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ddag/ddag/internal/config"
)

func TestCSRFMiddlewareRejectsCookieMutationWithoutToken(t *testing.T) {
	s := &service{cfg: config.Config{Session: config.SessionConfig{CookieName: "ddag_session"}}}
	called := false
	handler := s.csrf(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "ddag_session", Value: "session"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not be called")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestCSRFMiddlewareAllowsBearerMutation(t *testing.T) {
	s := &service{cfg: config.Config{Session: config.SessionConfig{CookieName: "ddag_session"}}}
	called := false
	handler := s.csrf(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler should be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
