// Package adminsvc implements the control-plane API consumed by the dashboard:
// dashboard login/session, RBAC-guarded CRUD for every metadata entity,
// connection/query testing, cache purge, and audit/monitoring reads. All
// permission checks run here (backend), never only in the UI (PRD §11.3 AC).
package adminsvc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

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

// decode reads exactly one JSON value into dst. It distinguishes request-size
// failures from malformed payloads and rejects trailing JSON values.
func decode(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	if err := dec.Decode(dst); err != nil {
		writeDecodeError(w, r, err)
		return false
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			httpx.ErrorCode(w, r, httpx.CodeBadRequest, "JSON body must contain a single value")
		} else {
			writeDecodeError(w, r, err)
		}
		return false
	}
	return true
}

func writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large") {
		httpx.ErrorCode(w, r, httpx.CodePayloadTooLarge, "JSON body exceeds 2 MiB limit")
		return
	}
	httpx.ErrorCode(w, r, httpx.CodeBadRequest, "invalid JSON body")
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
	p := store.ListParams{
		Search:  r.URL.Query().Get("search"),
		SortBy:  r.URL.Query().Get("sort_by"),
		SortDir: r.URL.Query().Get("sort_dir"),
	}
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
	offset := p.Offset()
	httpx.List(w, r, data, &httpx.Pagination{
		Page:    p.Page,
		Limit:   p.Limit,
		Offset:  offset,
		Total:   total,
		HasNext: int64(offset+p.Limit) < total,
	}, nil)
}

// notFoundOr maps store.ErrNotFound to a 404, otherwise a 500.
func storeErr(w http.ResponseWriter, r *http.Request, err error) {
	if err == store.ErrNotFound {
		httpx.ErrorCode(w, r, httpx.CodeNotFound, "resource not found")
		return
	}
	httpx.ErrorCode(w, r, httpx.CodeInternal, "internal error")
}
