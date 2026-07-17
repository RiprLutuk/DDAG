// Package httpx provides the standard DDAG response envelope (PRD §11.13),
// the error-code table (§18), request-id propagation, and JSON helpers shared
// by every service.
package httpx

import (
	"encoding/json"
	"net/http"
)

// Meta carries response metadata included on every success response.
type Meta struct {
	Cached       bool   `json:"cached"`
	DurationMS   int64  `json:"duration_ms"`
	CircuitState string `json:"circuit_state,omitempty"`
}

// Pagination is included on list/search responses.
type Pagination struct {
	Page    int   `json:"page"`
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	Total   int64 `json:"total"`
	HasNext bool  `json:"has_next"`
}

// SuccessEnvelope is the standard single-object success response.
type SuccessEnvelope struct {
	Success   bool        `json:"success"`
	RequestID string      `json:"request_id"`
	Data      interface{} `json:"data"`
	Meta      *Meta       `json:"meta,omitempty"`
}

// ListEnvelope is the standard list/search success response.
type ListEnvelope struct {
	Success    bool        `json:"success"`
	RequestID  string      `json:"request_id"`
	Data       interface{} `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Meta       *Meta       `json:"meta,omitempty"`
}

// ErrorBody is the error detail object.
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// ErrorEnvelope is the standard error response.
type ErrorEnvelope struct {
	Success   bool      `json:"success"`
	RequestID string    `json:"request_id"`
	Error     ErrorBody `json:"error"`
}

// WriteJSON marshals v and writes it with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// OK writes a single-object success envelope.
func OK(w http.ResponseWriter, r *http.Request, data interface{}, meta *Meta) {
	WriteJSON(w, http.StatusOK, SuccessEnvelope{
		Success:   true,
		RequestID: RequestID(r.Context()),
		Data:      data,
		Meta:      meta,
	})
}

// Created writes a 201 success envelope.
func Created(w http.ResponseWriter, r *http.Request, data interface{}) {
	WriteJSON(w, http.StatusCreated, SuccessEnvelope{
		Success:   true,
		RequestID: RequestID(r.Context()),
		Data:      data,
	})
}

// List writes a paginated list envelope.
func List(w http.ResponseWriter, r *http.Request, data interface{}, p *Pagination, meta *Meta) {
	WriteJSON(w, http.StatusOK, ListEnvelope{
		Success:    true,
		RequestID:  RequestID(r.Context()),
		Data:       data,
		Pagination: p,
		Meta:       meta,
	})
}
