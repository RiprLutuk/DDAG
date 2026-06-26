package httpx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
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

// ClientIP extracts the direct peer IP. Forwarded headers are intentionally not
// trusted without explicit proxy configuration.
func ClientIP(r *http.Request) string {
	return directIP(r)
}

// ParseTrustedProxies parses a comma-separated set of CIDR ranges or single IPs
// allowed to set X-Forwarded-For/X-Real-IP.
func ParseTrustedProxies(v string) ([]netip.Prefix, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	parts := strings.Split(v, ",")
	out := make([]netip.Prefix, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "/") {
			prefix, err := netip.ParsePrefix(p)
			if err != nil {
				return nil, fmt.Errorf("parse trusted proxy %q: %w", p, err)
			}
			out = append(out, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(p)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy %q: %w", p, err)
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		out = append(out, netip.PrefixFrom(addr, bits))
	}
	return out, nil
}

// ClientIPWithTrustedProxies honors forwarded headers only when the direct peer
// belongs to a configured trusted proxy range.
func ClientIPWithTrustedProxies(r *http.Request, trusted []netip.Prefix) string {
	remote := directIP(r)
	addr, err := netip.ParseAddr(remote)
	if err != nil || !trustedProxy(addr, trusted) {
		return remote
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for _, part := range strings.Split(xff, ",") {
			candidate := strings.TrimSpace(part)
			if _, err := netip.ParseAddr(candidate); err == nil {
				return candidate
			}
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		if _, err := netip.ParseAddr(xr); err == nil {
			return xr
		}
	}
	return remote
}

func trustedProxy(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func directIP(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return strings.Trim(host, "[]")
	}
	return host
}
