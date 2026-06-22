package metrics

import (
	"context"
	"net/http"
)

// routeHolder is a mutable container so a handler can set a low-cardinality
// route label (e.g. "GET /api/v1/brim/sites/{id}") that the metrics middleware
// reads after the handler returns. Without it, path parameters would explode
// metric cardinality.
type routeHolder struct{ label string }

type routeKey struct{}

// withRouteHolder attaches an empty holder to the context.
func withRouteHolder(ctx context.Context) (context.Context, *routeHolder) {
	h := &routeHolder{}
	return context.WithValue(ctx, routeKey{}, h), h
}

// SetRouteLabel sets the route label for the current request, if a holder is
// present (it is, for any request that passed through HTTPMiddleware).
func SetRouteLabel(ctx context.Context, label string) {
	if h, ok := ctx.Value(routeKey{}).(*routeHolder); ok && h != nil {
		h.label = label
	}
}

// RouteLabel returns the handler-set route label, falling back to the raw path.
func RouteLabel(r *http.Request) string {
	if h, ok := r.Context().Value(routeKey{}).(*routeHolder); ok && h != nil && h.label != "" {
		return h.label
	}
	return r.URL.Path
}
