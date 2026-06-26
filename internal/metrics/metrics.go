// Package metrics provides the shared Prometheus metric set, the /metrics,
// /healthz and /readyz endpoints (PRD §11.15, §19.3), and helpers to record
// HTTP, cache, rate-limit, auth, connector and pool activity.
package metrics

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds every counter/histogram/gauge a DDAG service may emit. A single
// instance is created per process via New and registered on a private registry.
type Metrics struct {
	reg *prometheus.Registry

	HTTPRequests *prometheus.CounterVec   // by service, method, route, status
	HTTPLatency  *prometheus.HistogramVec // by service, route — p50/p95/p99

	CacheHits   *prometheus.CounterVec // by route
	CacheMisses *prometheus.CounterVec // by route

	RateLimited  *prometheus.CounterVec // by client, route
	IPBlocked    *prometheus.CounterVec // by client
	Unauthorized prometheus.Counter
	Forbidden    prometheus.Counter
	TokenIssued  prometheus.Counter
	TokenFailed  prometheus.Counter
	TokenRevoked prometheus.Counter

	SingleflightActive prometheus.Gauge
	SingleflightShared prometheus.Counter
	MetadataSync       prometheus.Counter

	QueryDuration   *prometheus.HistogramVec // by connection, db_type
	ConnectorErr    *prometheus.CounterVec   // by connection, db_type
	CircuitState    *prometheus.GaugeVec     // by connection, db_type
	CircuitOpen     *prometheus.CounterVec   // by connection, db_type
	CircuitHalfOpen *prometheus.CounterVec   // by connection, db_type
	PoolInUse       *prometheus.GaugeVec     // by connection
	PoolIdle        *prometheus.GaugeVec     // by connection
	PoolMax         *prometheus.GaugeVec     // by connection
}

// New creates and registers the DDAG metric set for the given service.
func New(service string) *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	f := promauto.With(reg)
	labels := prometheus.Labels{"service": service}

	m := &Metrics{
		reg: reg,
		HTTPRequests: f.NewCounterVec(prometheus.CounterOpts{
			Name: "ddag_http_requests_total", Help: "Total HTTP requests.",
			ConstLabels: labels,
		}, []string{"method", "route", "status"}),
		HTTPLatency: f.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ddag_http_request_duration_seconds", Help: "HTTP request latency.",
			ConstLabels: labels,
			Buckets:     []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"route"}),
		CacheHits: f.NewCounterVec(prometheus.CounterOpts{
			Name: "ddag_cache_hits_total", Help: "Cache hits.", ConstLabels: labels,
		}, []string{"route"}),
		CacheMisses: f.NewCounterVec(prometheus.CounterOpts{
			Name: "ddag_cache_misses_total", Help: "Cache misses.", ConstLabels: labels,
		}, []string{"route"}),
		RateLimited: f.NewCounterVec(prometheus.CounterOpts{
			Name: "ddag_rate_limited_total", Help: "Rate-limited requests.", ConstLabels: labels,
		}, []string{"client", "route"}),
		IPBlocked: f.NewCounterVec(prometheus.CounterOpts{
			Name: "ddag_ip_blocked_total", Help: "IP-whitelist blocked requests.", ConstLabels: labels,
		}, []string{"client"}),
		Unauthorized: f.NewCounter(prometheus.CounterOpts{
			Name: "ddag_unauthorized_total", Help: "401 responses.", ConstLabels: labels,
		}),
		Forbidden: f.NewCounter(prometheus.CounterOpts{
			Name: "ddag_forbidden_total", Help: "403 responses.", ConstLabels: labels,
		}),
		TokenIssued: f.NewCounter(prometheus.CounterOpts{
			Name: "ddag_token_issued_total", Help: "Tokens issued.", ConstLabels: labels,
		}),
		TokenFailed: f.NewCounter(prometheus.CounterOpts{
			Name: "ddag_token_failed_total", Help: "Token request failures.", ConstLabels: labels,
		}),
		TokenRevoked: f.NewCounter(prometheus.CounterOpts{
			Name: "ddag_token_revoked_total", Help: "Tokens revoked.", ConstLabels: labels,
		}),
		SingleflightActive: f.NewGauge(prometheus.GaugeOpts{
			Name: "ddag_singleflight_active", Help: "Active singleflight cache fills.", ConstLabels: labels,
		}),
		SingleflightShared: f.NewCounter(prometheus.CounterOpts{
			Name: "ddag_singleflight_shared", Help: "Shared singleflight results.", ConstLabels: labels,
		}),
		MetadataSync: f.NewCounter(prometheus.CounterOpts{
			Name: "ddag_metadata_sync_total", Help: "Metadata sync events received via Pub/Sub.", ConstLabels: labels,
		}),
		QueryDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ddag_db_query_duration_seconds", Help: "Source DB query duration.",
			ConstLabels: labels,
			Buckets:     []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		}, []string{"connection", "db_type"}),
		ConnectorErr: f.NewCounterVec(prometheus.CounterOpts{
			Name: "ddag_connector_errors_total", Help: "Connector errors.", ConstLabels: labels,
		}, []string{"connection", "db_type"}),
		CircuitState: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ddag_circuit_state", Help: "Circuit breaker state: 0=closed, 1=half-open, 2=open.", ConstLabels: labels,
		}, []string{"connection", "db_type"}),
		CircuitOpen: f.NewCounterVec(prometheus.CounterOpts{
			Name: "ddag_circuit_open_total", Help: "Circuit breaker open events.", ConstLabels: labels,
		}, []string{"connection", "db_type"}),
		CircuitHalfOpen: f.NewCounterVec(prometheus.CounterOpts{
			Name: "ddag_circuit_half_open_total", Help: "Circuit breaker half-open transitions.", ConstLabels: labels,
		}, []string{"connection", "db_type"}),
		PoolInUse: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ddag_pool_in_use_connections", Help: "In-use pool connections.", ConstLabels: labels,
		}, []string{"connection"}),
		PoolIdle: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ddag_pool_idle_connections", Help: "Idle pool connections.", ConstLabels: labels,
		}, []string{"connection"}),
		PoolMax: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ddag_pool_max_connections", Help: "Configured max pool connections.", ConstLabels: labels,
		}, []string{"connection"}),
	}
	return m
}

// Handler returns the Prometheus /metrics HTTP handler for this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// HTTPMiddleware records request count + latency for every request, labeling by
// a low-cardinality route (provided by the caller via SetRoute, else the path).
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, _ := withRouteHolder(r.Context())
		r = r.WithContext(ctx)
		rec := &codeRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		route := RouteLabel(r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		m.HTTPRequests.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
		m.HTTPLatency.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

type codeRecorder struct {
	http.ResponseWriter
	status int
}

func (c *codeRecorder) WriteHeader(code int) { c.status = code; c.ResponseWriter.WriteHeader(code) }
func (c *codeRecorder) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	return c.ResponseWriter.Write(b)
}

// ReadyFunc reports whether the service is ready to serve traffic.
type ReadyFunc func() bool

var ready atomic.Bool

// SetReady toggles process readiness (used by /readyz).
func SetReady(v bool) { ready.Store(v) }

// MountHealth registers /healthz, /readyz and /metrics on the given mux.
func (m *Metrics) MountHealth(mux *http.ServeMux, readyFn ReadyFunc) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		ok := ready.Load()
		if readyFn != nil {
			ok = ok && readyFn()
		}
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not_ready"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	mux.Handle("/metrics", m.Handler())
}
