package httpx

import (
	"context"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ddag/ddag/internal/logging"
	"github.com/google/uuid"
)

// requestIDKey is the context key for the per-request id.
type requestIDKey struct{}

// RequestIDHeader is the canonical header used to read/propagate request ids.
const RequestIDHeader = "X-Request-ID"

// RequestID returns the request id stored on the context, or "" if absent.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// withRequestID stores a request id on the context.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDMiddleware ensures every request has a request id (PRD §11.13,
// §14.5). It honors an incoming X-Request-ID for cross-service propagation,
// otherwise it generates one, and echoes it back on the response.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(RequestIDHeader))
		if id == "" {
			id = "req-" + uuid.NewString()
		}
		ctx := withRequestID(r.Context(), id)
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder captures the status code for access logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// LoggingMiddleware emits a structured access log per request and attaches a
// request-scoped logger (carrying request_id) to the context.
func LoggingMiddleware(base *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rid := RequestID(r.Context())
			l := base.With("request_id", rid)
			ctx := logging.WithContext(r.Context(), l)
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r.WithContext(ctx))
			l.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"remote", ClientIP(r),
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// RecoverMiddleware converts panics into a standard INTERNAL_ERROR response so
// one bad handler can never crash the process (PRD §11.12 AC).
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logging.FromContext(r.Context()).Error("panic_recovered",
					"panic", rec, "stack", string(debug.Stack()))
				Error(w, r, NewError(CodeInternal, "An internal error occurred"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ClientIP extracts the best-effort client IP, honoring X-Forwarded-For and
// X-Real-IP set by trusted ingress/proxies (used for IP whitelist + audit).
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First entry is the original client.
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		return host[:i]
	}
	return host
}
