package adminsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/store"
)

func TestListResponseIncludesCompletePaginationMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resources?page=2&limit=10", nil)
	rec := httptest.NewRecorder()
	params := store.ListParams{Page: 2, Limit: 10}

	list(rec, req, []string{"item"}, params, 25)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Success    bool             `json:"success"`
		Pagination httpx.Pagination `json:"pagination"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success {
		t.Fatal("success = false, want true")
	}
	if body.Pagination.Page != 2 || body.Pagination.Limit != 10 || body.Pagination.Offset != 10 || body.Pagination.Total != 25 || !body.Pagination.HasNext {
		t.Fatalf("pagination = %+v, want page=2 limit=10 offset=10 total=25 has_next=true", body.Pagination)
	}
}

func TestCORSExposesRequestID(t *testing.T) {
	s := &service{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	s.cors(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != httpx.RequestIDHeader {
		t.Fatalf("Access-Control-Expose-Headers = %q, want %q", got, httpx.RequestIDHeader)
	}
}
