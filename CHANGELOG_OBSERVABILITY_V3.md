# DDAG v3 Observability Fixes - 2026-06-29

## Summary
Fixed Grafana dashboard queries to align with actual metric names in `internal/metrics/metrics.go` and added zero-series registration to prevent "No data" in idle state.

## Changes Made

### 1. Grafana Dashboard pool panels (`deploy/grafana/dashboards/ddag-overview.json`)

Pool panels must use the **actual** metric names exposed by `internal/metrics/metrics.go`
(`ddag_db_pool_active`, `ddag_pool_in_use_connections`, `ddag_pool_max_connections`,
…), with the v3-name → legacy-name fallback documented in PRD §8.6. A previous
draft of this changelog shortened them to `ddag_pool_in_use` / `ddag_pool_idle` /
`ddag_pool_max`, which are **not** emitted by any service and made both pool panels
render "No data". That regression was reverted; the canonical queries are:

#### Panel: "Connection pool usage"

```promql
ddag_db_pool_active or ddag_pool_in_use_connections
ddag_db_pool_idle or ddag_pool_idle_connections
ddag_pool_max_connections
```

#### Panel: "Pool pressure ratio"

```promql
((ddag_db_pool_active or ddag_pool_in_use_connections) / clamp_min(ddag_pool_max_connections, 1))
```

### 2. Zero-Series Registration

Centralized in `internal/metrics/metrics.go` via `registerDefaultSeries(service)`,
called from `metrics.New` so every process registers its own low-cardinality
defaults at startup (PRD §8.3, §12 Phase 2). Uses the `"unknown"` placeholder so
aggregate panels read `0` instead of `No data` before real traffic, without
pre-registering real connection IDs (which would inflate cardinality).

#### Gateway (`service == "api-gateway"`)

```go
m.CacheHits.WithLabelValues("unknown").Add(0)
m.CacheMisses.WithLabelValues("unknown").Add(0)
m.QueuedRequests.WithLabelValues("unknown").Add(0)
m.QueueDepth.WithLabelValues("unknown").Set(0)
m.QueueTimeout.WithLabelValues("unknown").Add(0)
m.RejectedRequests.WithLabelValues("unknown").Add(0)
m.RequestLogsDropped.Add(0)
```

#### Connectors (`service == "connector-<dbType>"`)

```go
m.ConnectorRequests.WithLabelValues("unknown", dbType).Add(0)
m.ConnectorErr.WithLabelValues("unknown", dbType).Add(0)
m.ConnCacheHits.WithLabelValues(dbType).Add(0)
m.ConnCacheMisses.WithLabelValues(dbType).Add(0)
m.CircuitState.WithLabelValues("unknown", dbType).Set(0)
m.CircuitOpen.WithLabelValues("unknown", dbType).Add(0)
m.CircuitHalfOpen.WithLabelValues("unknown", dbType).Add(0)
```

### 3. Smoke Test Script
Created `scripts/smoke-test-metrics.sh` to verify metrics are properly exposed.

Usage:
```bash
./scripts/smoke-test-metrics.sh http://localhost:8082
```

## Testing

1. Rebuild and restart services:
```bash
make build
docker compose up -d --build
```

2. Run smoke test:
```bash
./scripts/smoke-test-metrics.sh http://localhost:8082
```

3. Check Grafana dashboard at http://localhost:3001
   - All pool panels should now show data
   - No "No data" errors on aggregate panels even before traffic

## PRD Compliance

✅ **PRD v3 §3.2** - Label convention correctly implemented  
✅ **PRD v3 §7** - All metrics properly defined  
✅ **PRD v3 §8** - Zero-series registration added  
✅ **PRD v3 §10.2** - Dashboard queries aligned with actual metric names  
✅ **PRD v3 §14** - Smoke test script for acceptance testing  
✅ **PRD v3 §16** - Implementation tasks completed

## Related Files

- `deploy/grafana/dashboards/ddag-overview.json` - Fixed PromQL queries
- `internal/gatewaysvc/service.go` - Added gateway zero-series
- `internal/connector/service.go` - Added connector zero-series
- `scripts/smoke-test-metrics.sh` - New smoke test script
- `PRD_DDAG_v3_Observability.md` - Reference PRD
