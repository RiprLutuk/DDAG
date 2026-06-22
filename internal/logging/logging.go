// Package logging provides a shared structured (JSON) slog logger with a
// redaction layer so secrets/passwords never reach the logs (PRD §13.4).
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// sensitiveKeys are attribute keys whose values are always masked.
var sensitiveKeys = map[string]struct{}{
	"password":      {},
	"client_secret": {},
	"secret":        {},
	"secret_ref":    {},
	"authorization": {},
	"access_token":  {},
	"refresh_token": {},
	"dsn":           {},
	"private_key":   {},
	"master_key":    {},
}

// Logger is an alias for slog.Logger so callers can depend on this package
// without importing log/slog directly.
type Logger = slog.Logger

// New returns a JSON slog.Logger at the given level with secret redaction and a
// fixed "service" attribute.
func New(service, level string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       parseLevel(level),
		ReplaceAttr: redact,
	})
	return slog.New(h).With("service", service)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func redact(_ []string, a slog.Attr) slog.Attr {
	if _, ok := sensitiveKeys[strings.ToLower(a.Key)]; ok {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

// ctxKey is the private type for storing a logger on a context.
type ctxKey struct{}

// WithContext returns a new context carrying the logger.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext extracts a logger from the context, falling back to the default.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
