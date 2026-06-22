// Package adminsvc implements the control-plane API consumed by the dashboard:
// dashboard login/session, RBAC-guarded CRUD for every metadata entity,
// connection/query testing, cache purge, and audit/monitoring reads. All
// permission checks run here (backend), never only in the UI (PRD §11.3 AC).
package adminsvc

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// getBool reads a boolean environment variable with a default.
func getBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// decode reads a JSON body into dst, returning false (and writing a 400) on error.
func decode(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(dst); err != nil {
		httpx.ErrorCode(w, r, httpx.CodeBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// idParam parses the {id} URL parameter as a UUID, writing a 400 on failure.
func idParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// listParams reads page/limit/search query parameters.
func listParams(r *http.Request) store.ListParams {
	p := store.ListParams{Search: r.URL.Query().Get("search")}
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil {
		p.Page = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		p.Limit = v
	}
	p.Normalize()
	return p
}

// ok writes a single-object success envelope.
func ok(w http.ResponseWriter, r *http.Request, data interface{}) {
	httpx.OK(w, r, data, nil)
}

// list writes a paginated list envelope.
func list(w http.ResponseWriter, r *http.Request, data interface{}, p store.ListParams, total int64) {
	httpx.List(w, r, data, &httpx.Pagination{Page: p.Page, Limit: p.Limit, Total: total}, nil)
}

// notFoundOr maps store.ErrNotFound to a 404, otherwise a 500.
func storeErr(w http.ResponseWriter, r *http.Request, err error) {
	if err == store.ErrNotFound {
		httpx.ErrorCode(w, r, httpx.CodeNotFound, "resource not found")
		return
	}
	httpx.ErrorCode(w, r, httpx.CodeInternal, "internal error")
}
