# PRD DDAG v3 Observability - Grafana, Prometheus, and Runtime Metrics

**Product:** DDAG - Dynamic Data API Gateway
**Version:** v3 Observability Repo-Aligned Draft
**Owner:** Heri Riski Anto
**Date:** 2026-06-27
**Status:** Draft / Ready for Implementation
**Target Environment:** Docker Compose, VPS 908MB RAM, systemd, Caddy HTTPS, Prometheus, Grafana

---

## 1. Background

DDAG v3 sudah menyediakan observability dasar:

- Setiap service expose `/healthz`, `/readyz`, dan `/metrics`.
- Prometheus scrape config tersedia di `deploy/prometheus/prometheus.yml`.
- Grafana dashboard provisioning tersedia di `deploy/grafana/dashboards/ddag-overview.json`.
- Shared metrics didefinisikan di `internal/metrics/metrics.go`.

Dashboard Grafana saat ini sudah memonitor beberapa area inti:

- Request rate by service.
- p95 HTTP latency by service.
- 5xx error rate.
- Cache hit ratio.
- Tokens issued vs failed.
- Source DB query p95.
- Connection pool usage.
- Security events.
- Connector errors.

Namun dashboard dan PRD sebelumnya belum sepenuhnya selaras dengan implementasi repo saat ini. Beberapa asumsi label/metric berbeda dengan kode, beberapa panel masih bisa menampilkan `No data` saat idle, dan daftar service belum mencakup `policy-engine`, `cache-service`, `worker`, serta `connector-oracle`.

PRD ini menjadi baseline observability v3 yang selaras dengan repo per 2026-06-27.

---

## 2. Source of Truth

PRD ini disusun berdasarkan file berikut:

| Area | Source |
|---|---|
| Shared metrics | `internal/metrics/metrics.go` |
| Gateway cache, security, queue metrics | `internal/gatewaysvc/service.go` |
| Connector query/error/circuit metrics | `internal/connector/service.go` |
| Pool metrics publisher | `internal/connectorpool/registry.go` |
| Prometheus scrape targets | `deploy/prometheus/prometheus.yml` |
| Current Grafana dashboard | `deploy/grafana/dashboards/ddag-overview.json` |
| Compose services and ports | `docker-compose.yml` |
| Operational docs | `docs/OPERATIONS.md`, `docs/ARCHITECTURE.md`, `README.md` |

---

## 3. Current Baseline

### 3.1 Services in Scope

| Service | Port | Metrics Path | Notes |
|---|---:|---|---|
| `admin-backend` | 8080 | `/metrics` | Dashboard API and audit/control-plane operations |
| `auth-service` | 8081 | `/metrics` | OAuth2 token, refresh, revoke, introspect, JWKS |
| `api-gateway` | 8082 | `/metrics` | Data plane, cache, policy, backpressure, connector dispatch |
| `policy-engine` | 8083 | `/metrics` | Optional remote policy decisions |
| `cache-service` | 8084 | `/metrics` | Standalone cache management service |
| `worker` | 8085 | `/metrics` | Background maintenance |
| `connector-postgres` | 8090 | `/metrics` | PostgreSQL source DB connector |
| `connector-mysql` | 8091 | `/metrics` | MySQL/MariaDB source DB connector |
| `connector-oracle` | 8092 | `/metrics` | Oracle source DB connector |
| `connector-sqlserver` | 8093 | `/metrics` | SQL Server source DB connector |

### 3.2 Label Convention

The `service` label is implemented as a Prometheus `ConstLabels` value in `metrics.New(service)`.

Implication:

- Prometheus output includes `service="<service-name>"`.
- Go code must not pass `service` to `WithLabelValues`.
- Metric specs in this PRD may show `service` as an output label, but implementation examples only include variable labels.

Example:

```go
m := metrics.New("api-gateway")
m.CacheHits.WithLabelValues("/api/v1/orders").Inc()
```

Prometheus output:

```text
ddag_cache_hits_total{service="api-gateway",route="/api/v1/orders"} 1
```

---

## 4. Problem Statement

Observability v3 sudah berjalan, tetapi belum production-ready untuk operasi harian karena:

1. Grafana dashboard masih menggunakan sebagian query lama, terutama connection pool usage.
2. Source DB query p95 belum group by `service`, `db_type`, dan `connection`.
3. Beberapa panel aggregate dapat menampilkan `No data` saat service sehat tetapi belum ada traffic.
4. Backpressure queue, circuit breaker, pool wait/timeout, dan optional node-exporter belum lengkap di dashboard.
5. PRD lama menyebut metric/label yang belum ada di implementasi saat ini, seperti:
   - `ddag_db_pool_max`
   - `ddag_connector_errors_total{error_class}`
   - `ddag_cache_bypass_total`
   - `ddag_cache_fill_duration_seconds_bucket`
   - `ddag_invalid_token_total`
6. Connector Oracle sudah ada di repo, tetapi belum tercakup di PRD lama.
7. Counter `ddag_singleflight_shared` belum mengikuti naming best-practice Prometheus karena tidak memakai suffix `_total`. v3 harus expose nama canonical `ddag_singleflight_shared_total` sambil mempertahankan nama legacy selama minimal satu transition cycle.

---

## 5. Goals

### 5.1 Product Goals

1. Membuat Grafana dashboard cukup jelas untuk operasi DDAG harian.
2. Menampilkan kondisi control plane, data plane, connector, cache, policy, dan pool secara terpisah.
3. Mendeteksi bottleneck sebelum user melaporkan slowness.
4. Mendukung PostgreSQL, MySQL/MariaDB, Oracle, dan SQL Server.
5. Tetap ringan untuk VPS 908MB RAM.

### 5.2 Engineering Goals

1. Selaraskan dashboard PromQL dengan metric yang benar-benar diexpose repo saat ini.
2. Hindari `No data` untuk panel aggregate yang bisa disajikan sebagai `0`.
3. Standardize label usage tanpa menaikkan cardinality berlebihan.
4. Pertahankan compatibility dengan metric pool legacy selama transisi.
5. Buat acceptance criteria dan smoke test yang bisa dijalankan dari server.

---

## 6. Non-Goals

1. Tidak memasang stack berat seperti Loki, Tempo, OpenTelemetry Collector, atau ELK di VPS 908MB.
2. Tidak expose Prometheus langsung ke publik.
3. Tidak memakai label high-cardinality seperti SQL text, request body, token, full IP, atau raw error message.
4. Tidak menyimpan raw payload atau sensitive database values di metric.
5. Tidak menambah instrumentation per-row.
6. Tidak mengubah kontrak metric yang sudah ada kecuali ada task migrasi terpisah.

---

## 7. Metric Inventory

### 7.1 Implemented Metrics

Observed from code and tests:

```text
ddag_http_requests_total
ddag_http_request_duration_seconds_bucket
ddag_cache_hits_total
ddag_cache_misses_total
ddag_rate_limited_total
ddag_ip_blocked_total
ddag_unauthorized_total
ddag_forbidden_total
ddag_token_issued_total
ddag_token_failed_total
ddag_token_revoked_total
ddag_singleflight_active
ddag_singleflight_shared_total
ddag_singleflight_shared
ddag_metadata_sync_total
ddag_db_query_duration_seconds_bucket
ddag_connector_requests_total
ddag_connector_errors_total
ddag_circuit_state
ddag_circuit_open_total
ddag_circuit_half_open_total
ddag_pool_in_use_connections
ddag_pool_idle_connections
ddag_pool_max_connections
ddag_db_pool_active
ddag_db_pool_idle
ddag_db_pool_wait_count
ddag_db_pool_wait_duration_ms
ddag_db_pool_timeout_count
ddag_queued_requests_total
ddag_queue_depth
ddag_queue_timeout_total
ddag_rejected_requests_total
```

### 7.2 Current Variable Labels

`service` exists as const label on all DDAG metrics. The table below lists only variable labels passed by code.

| Metric | Variable Labels |
|---|---|
| `ddag_http_requests_total` | `method`, `route`, `status` |
| `ddag_http_request_duration_seconds_bucket` | `route`, `le` |
| `ddag_cache_hits_total` | `route` |
| `ddag_cache_misses_total` | `route` |
| `ddag_rate_limited_total` | `client`, `route` |
| `ddag_ip_blocked_total` | `client` |
| `ddag_unauthorized_total` | none |
| `ddag_forbidden_total` | none |
| `ddag_token_issued_total` | none |
| `ddag_token_failed_total` | none |
| `ddag_token_revoked_total` | none |
| `ddag_singleflight_active` | none |
| `ddag_singleflight_shared_total` | none |
| `ddag_singleflight_shared` | none, legacy compatibility alias |
| `ddag_metadata_sync_total` | none |
| `ddag_db_query_duration_seconds_bucket` | `connection`, `db_type`, `le` |
| `ddag_connector_requests_total` | `connection`, `db_type` |
| `ddag_connector_errors_total` | `connection`, `db_type` |
| `ddag_circuit_state` | `connection`, `db_type` |
| `ddag_circuit_open_total` | `connection`, `db_type` |
| `ddag_circuit_half_open_total` | `connection`, `db_type` |
| `ddag_pool_in_use_connections` | `connection` |
| `ddag_pool_idle_connections` | `connection` |
| `ddag_pool_max_connections` | `connection` |
| `ddag_db_pool_active` | `connection` |
| `ddag_db_pool_idle` | `connection` |
| `ddag_db_pool_wait_count` | `connection` |
| `ddag_db_pool_wait_duration_ms` | `connection` |
| `ddag_db_pool_timeout_count` | `connection` |
| `ddag_queued_requests_total` | `route` |
| `ddag_queue_depth` | `route` |
| `ddag_queue_timeout_total` | `route` |
| `ddag_rejected_requests_total` | `route` |

### 7.3 Not Yet Implemented

These are not required for the immediate dashboard fix:

```text
ddag_db_pool_max
ddag_cache_bypass_total
ddag_cache_fill_duration_seconds_bucket
ddag_connector_errors_total{error_class}
ddag_invalid_token_total
```

If these are added later, they must be implemented with low-cardinality labels and tests in `internal/metrics/metrics_test.go`.

---

## 8. Required Metric Specification

### 8.1 HTTP Request Metrics

Metric:

```text
ddag_http_requests_total{service, method, route, status}
```

Type: Counter

Purpose:

- Request rate per service.
- HTTP 4xx/5xx error rate.
- Traffic split by method/route when needed.

Grafana queries:

```promql
sum by (service) (rate(ddag_http_requests_total[1m]))
```

```promql
sum by (service) (rate(ddag_http_requests_total{status=~"5.."}[1m])) or vector(0)
```

```promql
sum by (service) (rate(ddag_http_requests_total{status=~"4.."}[1m])) or vector(0)
```

Acceptance criteria:

- Request rate panel shows every service that has received HTTP traffic.
- 4xx/5xx aggregate panels show `0` instead of empty when no errors exist.

### 8.2 HTTP Latency Metrics

Metric:

```text
ddag_http_request_duration_seconds_bucket{service, route, le}
```

Type: Histogram

Grafana queries:

```promql
histogram_quantile(
  0.95,
  sum by (le, service) (rate(ddag_http_request_duration_seconds_bucket[5m]))
)
```

```promql
histogram_quantile(
  0.95,
  sum by (le, service, route) (rate(ddag_http_request_duration_seconds_bucket[5m]))
)
```

Acceptance criteria:

- p95 service panel shows `admin-backend`, `auth-service`, and `api-gateway` after smoke traffic.
- Route panel uses sanitized route labels, not raw high-cardinality paths where a route template is available.

### 8.3 Cache Metrics

Implemented metrics:

```text
ddag_cache_hits_total{service, route}
ddag_cache_misses_total{service, route}
ddag_singleflight_active{service}
ddag_singleflight_shared_total{service}
ddag_singleflight_shared{service} # legacy compatibility alias during migration
```

Type: Counter + Gauge

Purpose:

- Measure API response cache effectiveness.
- Detect duplicate cache-fill suppression via singleflight.

Grafana queries:

```promql
(
  sum(rate(ddag_cache_hits_total[5m]))
  /
  clamp_min(
    sum(rate(ddag_cache_hits_total[5m])) + sum(rate(ddag_cache_misses_total[5m])),
    1
  )
) or vector(0)
```

```promql
sum by (route) (rate(ddag_cache_hits_total[5m]))
/
clamp_min(
  sum by (route) (rate(ddag_cache_hits_total[5m])) + sum by (route) (rate(ddag_cache_misses_total[5m])),
  1
)
```

```promql
ddag_singleflight_active{service="api-gateway"} or vector(0)
```

```promql
rate(ddag_singleflight_shared_total{service="api-gateway"}[5m])
or
rate(ddag_singleflight_shared{service="api-gateway"}[5m])
or vector(0)
```

Implementation requirement:

- Gateway already increments hit/miss on cache lookup.
- Add startup zero-series registration for known low-cardinality defaults:

```go
m.CacheHits.WithLabelValues("unknown").Add(0)
m.CacheMisses.WithLabelValues("unknown").Add(0)
```

Acceptance criteria:

- Cache hit ratio panel shows `0` instead of `No data` on a healthy idle gateway.
- Repeated request to a cacheable route increases hit ratio above `0`.
- Singleflight panels show active/shared behavior during concurrent cache misses.

### 8.4 Token Metrics

Metrics:

```text
ddag_token_issued_total{service}
ddag_token_failed_total{service}
ddag_token_revoked_total{service}
```

Type: Counter

Grafana queries:

```promql
sum(rate(ddag_token_issued_total[1m])) or vector(0)
```

```promql
sum(rate(ddag_token_failed_total[1m])) or vector(0)
```

```promql
sum(rate(ddag_token_revoked_total[1m])) or vector(0)
```

Acceptance criteria:

- Successful `/oauth/token` increments `ddag_token_issued_total`.
- Invalid client credentials increments `ddag_token_failed_total`.
- Token revoke increments `ddag_token_revoked_total`.

### 8.5 Source DB Query Metrics

Metric:

```text
ddag_db_query_duration_seconds_bucket{service, connection, db_type, le}
```

Type: Histogram

Purpose:

- Measure source DB query latency per connector, DB type, and connection.
- Separate PostgreSQL, MySQL/MariaDB, Oracle, and SQL Server performance.

Grafana query:

```promql
histogram_quantile(
  0.95,
  sum by (le, service, db_type, connection) (
    rate(ddag_db_query_duration_seconds_bucket[5m])
  )
)
```

Required label values:

| Label | Example |
|---|---|
| `service` | `connector-postgres` |
| `connection` | source DB UUID |
| `db_type` | `postgres`, `mysql`, `oracle`, `sqlserver` |

Acceptance criteria:

- PostgreSQL traffic creates `db_type="postgres"` series.
- MySQL/MariaDB traffic creates `db_type="mysql"` series.
- Oracle traffic creates `db_type="oracle"` series when enabled.
- SQL Server traffic creates `db_type="sqlserver"` series.
- Dashboard legend includes `service`, `db_type`, and `connection`.

### 8.6 Connection Pool Metrics

Implemented metrics:

```text
ddag_pool_in_use_connections{service, connection}
ddag_pool_idle_connections{service, connection}
ddag_pool_max_connections{service, connection}
ddag_db_pool_active{service, connection}
ddag_db_pool_idle{service, connection}
ddag_db_pool_wait_count{service, connection}
ddag_db_pool_wait_duration_ms{service, connection}
ddag_db_pool_timeout_count{service, connection}
```

Type: Gauge

Current naming rule:

- `ddag_db_pool_active` is the v3 name for active/in-use connections.
- `ddag_db_pool_idle` is the v3 name for idle connections.
- `ddag_pool_max_connections` remains the implemented max metric.
- `ddag_db_pool_max` is not implemented and is not required for the immediate dashboard fix.

Recommended Grafana queries:

Active:

```promql
ddag_db_pool_active or ddag_pool_in_use_connections
```

Idle:

```promql
ddag_db_pool_idle or ddag_pool_idle_connections
```

Max:

```promql
ddag_pool_max_connections
```

Pool pressure:

```promql
(
  ddag_db_pool_active or ddag_pool_in_use_connections
)
/
clamp_min(ddag_pool_max_connections, 1)
```

Wait count:

```promql
ddag_db_pool_wait_count
```

Wait duration:

```promql
ddag_db_pool_wait_duration_ms
```

Timeout count:

```promql
ddag_db_pool_timeout_count
```

Implementation note:

- Pool gauges are published by `connectorpool.Registry.PublishStats()`.
- Metrics appear after a connector pool has been created by traffic or `/test` flow.
- A per-connection pool panel may legitimately be empty before any pool exists; aggregate panels should use `or vector(0)` where useful.

Acceptance criteria:

- Pool panel shows active/idle/max after connector traffic.
- Pool pressure panel stays between `0` and `1` in normal conditions.
- Pool wait/timeout panels expose saturation symptoms.

### 8.7 Connector Request, Error, and Circuit Metrics

Implemented metrics:

```text
ddag_connector_requests_total{service, connection, db_type}
ddag_connector_errors_total{service, connection, db_type}
ddag_circuit_state{service, connection, db_type}
ddag_circuit_open_total{service, connection, db_type}
ddag_circuit_half_open_total{service, connection, db_type}
```

Type: Counter + Gauge

Current limitation:

- `ddag_connector_errors_total` does not have `error_class`.
- Error class can be added later as a separate instrumentation task if dashboard needs pool timeout vs query timeout vs DB unavailable split.

Grafana queries:

```promql
sum by (service, db_type) (rate(ddag_connector_requests_total[1m])) or vector(0)
```

```promql
sum by (service, db_type) (rate(ddag_connector_errors_total[1m])) or vector(0)
```

```promql
ddag_circuit_state
```

```promql
sum by (service, db_type) (rate(ddag_circuit_open_total[5m])) or vector(0)
```

Acceptance criteria:

- Connector request panel increases when gateway dispatches queries.
- Connector error panel increases after intentionally bad DB request or unavailable source DB.
- Circuit state panel shows `0=closed`, `1=half-open`, `2=open`.

### 8.8 Security and Policy Metrics

Implemented metrics:

```text
ddag_unauthorized_total{service}
ddag_forbidden_total{service}
ddag_rate_limited_total{service, client, route}
ddag_ip_blocked_total{service, client}
```

Type: Counter

Grafana queries:

```promql
sum(rate(ddag_unauthorized_total[1m])) or vector(0)
```

```promql
sum(rate(ddag_forbidden_total[1m])) or vector(0)
```

```promql
sum(rate(ddag_rate_limited_total[1m])) or vector(0)
```

```promql
sum(rate(ddag_ip_blocked_total[1m])) or vector(0)
```

Cardinality rules:

- `client` is acceptable only while client count remains controlled.
- Do not add raw IP, JWT, request body, SQL text, or raw error message labels.

Acceptance criteria:

- Missing/invalid token increments unauthorized.
- Scope or grant failure increments forbidden.
- Rate limit test increments rate limited.
- IP whitelist denial increments IP blocked.

### 8.9 Gateway Backpressure Metrics

Implemented metrics:

```text
ddag_queued_requests_total{service, route}
ddag_queue_depth{service, route}
ddag_queue_timeout_total{service, route}
ddag_rejected_requests_total{service, route}
```

Type: Counter + Gauge

Grafana queries:

```promql
sum by (route) (rate(ddag_queued_requests_total{service="api-gateway"}[1m])) or vector(0)
```

```promql
max by (route) (ddag_queue_depth{service="api-gateway"}) or vector(0)
```

```promql
sum by (route) (rate(ddag_queue_timeout_total{service="api-gateway"}[1m])) or vector(0)
```

```promql
sum by (route) (rate(ddag_rejected_requests_total{service="api-gateway"}[1m])) or vector(0)
```

Acceptance criteria:

- Backpressure panel shows queued, timeout, and rejected rates.
- Queue depth is visible during burst/concurrency tests.

---

## 9. Dashboard Requirements

### 9.1 Current Dashboard Panels

| Panel | Current State | Required Update |
|---|---|---|
| Request rate by service | Present | Keep |
| p95 latency by service | Present | Keep |
| Error rate 5xx | Present | Add `or vector(0)` for aggregate empty state |
| Cache hit ratio | Present | Add idle-safe `or vector(0)` |
| Tokens issued vs failed | Present | Add idle-safe `or vector(0)` |
| Source DB query p95 | Present | Group by `service`, `db_type`, `connection` |
| Connection pool usage | Present | Use active/idle/max and v3/legacy fallback |
| Security events | Present | Add idle-safe `or vector(0)` |
| Connector errors | Present | Group by `service`, `db_type`; idle-safe aggregate |

### 9.2 Required New Panels

| Row | Panel | Query Source |
|---|---|---|
| Gateway | Queue accepted/timeouts/rejected | `ddag_queued_requests_total`, `ddag_queue_timeout_total`, `ddag_rejected_requests_total` |
| Gateway | Queue depth by route | `ddag_queue_depth` |
| Gateway | Singleflight active/shared | `ddag_singleflight_active`, `ddag_singleflight_shared_total` with fallback to `ddag_singleflight_shared` |
| DB | Pool pressure ratio | `ddag_db_pool_active`, `ddag_pool_max_connections` |
| DB | Pool wait/timeout | `ddag_db_pool_wait_count`, `ddag_db_pool_timeout_count` |
| Connector | Connector request rate | `ddag_connector_requests_total` |
| Connector | Circuit breaker state | `ddag_circuit_state` |
| Connector | Circuit transitions | `ddag_circuit_open_total`, `ddag_circuit_half_open_total` |
| System | RAM/swap/CPU/disk | node-exporter, optional |

### 9.3 Dashboard Design Rules

1. Use low-cardinality legends:
   - `{{service}}`
   - `{{db_type}}`
   - `{{connection}}`
   - `{{route}}` only for template route labels, not raw path variants.
2. Use `or vector(0)` for aggregate panels where an empty healthy state should read as zero.
3. Do not force zero for per-connection panels before a pool exists; empty state is acceptable there until connector traffic creates the series.
4. Keep dashboard refresh at `10s` or slower on the low-RAM VPS.

---

## 10. Prometheus Scrape Config

Current Docker Compose target config:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: ddag-services
    metrics_path: /metrics
    static_configs:
      - targets:
          - admin-backend:8080
          - auth-service:8081
          - api-gateway:8082
          - policy-engine:8083
          - cache-service:8084
          - worker:8085
          - connector-postgres:8090
          - connector-mysql:8091
          - connector-oracle:8092
          - connector-sqlserver:8093
        labels:
          platform: ddag
```

For systemd/VPS deployment, service hostnames may be replaced with `localhost` or private DNS names:

```yaml
- localhost:8080
- localhost:8081
- localhost:8082
- localhost:8083
- localhost:8084
- localhost:8085
- localhost:8090
- localhost:8091
- localhost:8092
- localhost:8093
```

Optional node-exporter:

```yaml
  - job_name: node
    static_configs:
      - targets:
          - localhost:9100
```

---

## 11. Low-RAM VPS Constraints

Because production/demo VPS only has 908MB RAM:

1. Keep scrape interval at `15s` or `30s`.
2. Keep retention short: `3d` to `7d`.
3. Avoid high-cardinality labels.
4. Do not install Loki/Tempo/ELK on this VPS.
5. Grafana and Prometheus may be stopped temporarily during Go builds if memory pressure occurs.
6. Prefer one overview dashboard with focused rows instead of many heavy dashboards.

Recommended Prometheus flags:

```text
--storage.tsdb.retention.time=7d
--storage.tsdb.retention.size=1GB
```

Fallback for tight memory:

```text
--storage.tsdb.retention.time=3d
--storage.tsdb.retention.size=512MB
```

---

## 12. Implementation Plan

### Phase 1 - Update Grafana Dashboard Queries

1. Update source DB query p95:
   - group by `service`, `db_type`, `connection`.
2. Update connection pool usage:
   - active: `ddag_db_pool_active or ddag_pool_in_use_connections`
   - idle: `ddag_db_pool_idle or ddag_pool_idle_connections`
   - max: `ddag_pool_max_connections`
3. Add pool pressure and pool wait/timeout panels.
4. Add connector request, circuit state, circuit transition panels.
5. Add queue depth/queued/timeout/rejected panels.
6. Add idle-safe `or vector(0)` to aggregate panels.
7. Add optional node-exporter panels only if node-exporter is deployed.

### Phase 2 - Register Low-Cardinality Zero Series

Add startup default labels in service initialization where useful.

Gateway:

```go
m.CacheHits.WithLabelValues("unknown").Add(0)
m.CacheMisses.WithLabelValues("unknown").Add(0)
m.QueuedRequests.WithLabelValues("unknown").Add(0)
m.QueueDepth.WithLabelValues("unknown").Set(0)
m.QueueTimeout.WithLabelValues("unknown").Add(0)
m.RejectedRequests.WithLabelValues("unknown").Add(0)
```

Connector:

```go
m.ConnectorRequests.WithLabelValues("unknown", dbType).Add(0)
m.ConnectorErr.WithLabelValues("unknown", dbType).Add(0)
m.CircuitState.WithLabelValues("unknown", dbType).Set(0)
m.CircuitOpen.WithLabelValues("unknown", dbType).Add(0)
m.CircuitHalfOpen.WithLabelValues("unknown", dbType).Add(0)
```

Do not pre-register real connection IDs until they are known. Registering every historical connection can increase cardinality.

### Phase 3 - Optional Metric Extensions

Only implement these if product/debugging need is confirmed:

1. Add `ddag_connector_errors_total{connection, db_type, error_class}` by updating the existing metric label shape and all query call sites.
2. Add `ddag_cache_bypass_total{route, reason}` if cache bypass reasons become important.
3. Add `ddag_cache_fill_duration_seconds_bucket{route}` if cache-fill latency needs a dedicated histogram.
4. Add `ddag_invalid_token_total{reason}` only if auth failure breakdown is required.
5. Add `ddag_db_pool_max` only if the team wants all v3 pool names under `ddag_db_pool_*`; otherwise continue using `ddag_pool_max_connections`.

Each optional extension requires:

- Update `internal/metrics/metrics.go`.
- Update all call sites.
- Add/adjust tests in `internal/metrics/metrics_test.go`.
- Update Grafana dashboard PromQL.
- Update this PRD.

### Phase 4 - Verification

1. Start stack.
2. Generate a token.
3. Hit published API route for PostgreSQL.
4. Hit MySQL, Oracle, and SQL Server routes where configured.
5. Repeat a cacheable request twice.
6. Run a small concurrent request burst.
7. Trigger an invalid token, forbidden scope, rate limit, and bad connector request.
8. Refresh Grafana and confirm panel states.

---

## 13. Smoke Test Commands

### 13.1 Raw Metrics

Gateway:

```bash
curl -s http://127.0.0.1:8082/metrics | grep -E 'http|cache|singleflight|queue|rate_limited|ip_blocked|unauthorized|forbidden'
```

Auth:

```bash
curl -s http://127.0.0.1:8081/metrics | grep -E 'token|http'
```

PostgreSQL connector:

```bash
curl -s http://127.0.0.1:8090/metrics | grep -E 'db_pool|pool_|db_query|connector|circuit'
```

MySQL connector:

```bash
curl -s http://127.0.0.1:8091/metrics | grep -E 'db_pool|pool_|db_query|connector|circuit'
```

Oracle connector:

```bash
curl -s http://127.0.0.1:8092/metrics | grep -E 'db_pool|pool_|db_query|connector|circuit'
```

SQL Server connector:

```bash
curl -s http://127.0.0.1:8093/metrics | grep -E 'db_pool|pool_|db_query|connector|circuit'
```

### 13.2 Prometheus Queries

HTTP traffic:

```promql
sum by (service) (rate(ddag_http_requests_total[1m]))
```

Cache hit ratio:

```promql
(
  sum(rate(ddag_cache_hits_total[5m]))
  /
  clamp_min(sum(rate(ddag_cache_hits_total[5m])) + sum(rate(ddag_cache_misses_total[5m])), 1)
) or vector(0)
```

Source DB p95:

```promql
histogram_quantile(
  0.95,
  sum by (le, service, db_type, connection) (
    rate(ddag_db_query_duration_seconds_bucket[5m])
  )
)
```

Pool usage:

```promql
ddag_db_pool_active or ddag_pool_in_use_connections
```

Pool pressure:

```promql
(
  ddag_db_pool_active or ddag_pool_in_use_connections
)
/
clamp_min(ddag_pool_max_connections, 1)
```

Connector errors:

```promql
sum by (service, db_type) (rate(ddag_connector_errors_total[1m])) or vector(0)
```

Backpressure:

```promql
sum by (route) (rate(ddag_rejected_requests_total{service="api-gateway"}[1m])) or vector(0)
```

Circuit state:

```promql
ddag_circuit_state
```

### 13.3 Load Test Diagnostics

When running `scripts/loadtest.py` or `scripts/loadtest_k6.js`, status `502` and
`503` must be treated as failures, not successful checks.

Observed demo behavior with client `loadtest-1782525997`:

| Symptom | Example Endpoint | Evidence | Diagnosis |
|---|---|---|---|
| `502 CONNECTOR_ERROR` | `/api/v1/postgres/transaksi/search?q=a&limit=5` | Gateway body: `{"code":"CONNECTOR_ERROR","message":"query failed"}` | Root failure is inside the connector query path. Inspect the published API SQL/query-builder metadata for PostgreSQL compatibility. |
| `503 CIRCUIT_BREAKER_OPEN` | `/api/v1/postgres-aws/transaksi?limit=5` | Gateway body: `{"code":"CIRCUIT_BREAKER_OPEN","message":"Database connection temporarily unavailable (circuit open)"}` | Secondary symptom after repeated connector failures. Wait for breaker recovery, then fix the query/connection cause that opened the breaker. |

Load-test scripts must therefore:

- count only `2xx` responses as successful in summary success rate,
- make k6 checks fail on non-`200` API responses,
- capture a short error-body sample for failed requests so `CONNECTOR_ERROR` and
  `CIRCUIT_BREAKER_OPEN` are visible without rerunning manual curl diagnostics.

---

## 14. Acceptance Criteria

### 14.1 Dashboard Acceptance

- [ ] Request rate panel shows services after smoke traffic.
- [ ] p95 latency panel shows service-level latency.
- [ ] 4xx/5xx panels show zero instead of `No data` during healthy no-error state.
- [ ] Cache hit ratio shows zero or percentage, not `No data`.
- [ ] Tokens issued vs failed shows issued/failed/revoked values.
- [ ] Source DB query p95 groups by service, DB type, and connection.
- [ ] Connection pool usage shows active/idle/max after connector traffic.
- [ ] Pool pressure and wait/timeout panels are visible.
- [ ] Security events shows 401/403/429/IP-blocked counters.
- [ ] Connector request/error panels group by service and DB type.
- [ ] Circuit state panel shows closed/half-open/open state values.
- [ ] Gateway queue panels show queued/depth/timeout/rejected metrics.
- [ ] Optional RAM/swap panels show data only when node-exporter is enabled.

### 14.2 System Acceptance

- [ ] No DDAG service crashes after metrics/dashboard changes.
- [ ] Prometheus scrape targets are UP.
- [ ] Prometheus memory usage remains acceptable on the 908MB VPS.
- [ ] Grafana dashboard loads within acceptable time from public HTTPS.
- [ ] No high-cardinality labels are introduced.

### 14.3 Code Acceptance for Future Metric Changes

- [ ] `go test ./internal/metrics ./internal/gatewaysvc ./internal/connector ./internal/connectorpool` passes.
- [ ] New metrics are registered in `internal/metrics/metrics_test.go`.
- [ ] All call sites use the correct variable label count.
- [ ] Grafana dashboard JSON uses only implemented metrics.
- [ ] Any newly introduced Prometheus `counter` follows naming convention with suffix `_total`.
- [ ] Renaming an existing counter requires a migration plan that preserves dashboard compatibility for at least one transition cycle (for example, query fallback old-name OR new-name).

---

## 15. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| High metric cardinality | Prometheus memory spikes | Avoid SQL text, request body, token, full IP, raw error labels |
| Dashboard query references missing metric | Panel shows `No data` | Keep PRD and dashboard aligned with `internal/metrics/metrics.go` |
| Per-connection pool not created yet | Pool panel empty before traffic | Document this as expected; use smoke traffic |
| Metric label shape changes | Existing queries break | Treat label changes as explicit migration tasks |
| Grafana/Prometheus memory pressure | VPS becomes slow | Short retention, low scrape interval, stop during builds if needed |
| Connector-specific gaps | Partial dashboard | Use shared connector service instrumentation |

---

## 16. Recommended Next Engineering Task

Implement in this order:

1. Update `deploy/grafana/dashboards/ddag-overview.json` with the PromQL changes in this PRD.
2. Add queue, pool pressure, pool wait/timeout, connector request, and circuit panels.
3. Add idle-safe `or vector(0)` to aggregate panels.
4. Add low-cardinality zero-series registration for gateway and connector metrics.
5. Run smoke traffic against token, cache, PostgreSQL, MySQL, Oracle, SQL Server, rate limit, and connector error paths.
6. Verify Grafana no longer shows `No data` for aggregate healthy-state panels.

---

## 17. Summary

DDAG v3 observability is already implemented at the service level, but the dashboard and previous PRD were behind the current repo. The main work is not adding a heavy observability stack; it is aligning Grafana queries with implemented metrics, adding missing operational panels, registering safe zero-series defaults, and keeping cardinality low for the 908MB VPS target.
