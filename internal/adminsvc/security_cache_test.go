package adminsvc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestNoStoreMiddleware(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/api/overview", nil)

	r := chi.NewRouter()
	r.Use(noStore)
	r.Get("/admin/api/overview", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}
