# DDAG Metrics Catalog v4

This document lists the operator-facing metrics and labels exposed by DDAG services and surfaced in the Monitoring dashboard.

## Service endpoints

| Service | Health | Ready | Metrics |
|---|---|---|---|
| admin-backend | `/healthz` | `/readyz` | `/metrics` |
| api-gateway | `/healthz` | `/readyz` | `/metrics` |
| auth-service | `/healthz` | `/readyz` | `/metrics` |
| policy-engine | `/healthz` | `/readyz` | `/metrics` |
| cache-service | `/healthz` | `/readyz` | `/metrics` |
| connector-* | `/healthz` | `/readyz` | `/metrics` |

## Core dashboard metrics

| Metric | Meaning | Labels / dimensions | Surface |
|---|---|---|---|
| `requests_today` | Total data-plane requests in current day window | endpoint, method | Monitoring overview |
| `avg_latency_ms` | Average request latency | endpoint, method | Monitoring overview |
| `error_rate_today` | 5xx ratio for real traffic in current day window; synthetic QA traffic is excluded | endpoint, method | Monitoring overview |
| `cache_hit_ratio` | Fraction of requests served from cache | endpoint, cache_rule | Monitoring overview |
| `top_slow` | Highest-latency APIs | api_label, path | Monitoring table |
| `top_errors` | Highest error-rate APIs | api_label, path | Monitoring table |
| `connector_pool_in_use` | Active DB connections in pool | connection_id, connection_name, db_type | Pool usage |
| `connector_pool_idle` | Idle DB connections in pool | connection_id, connection_name, db_type | Pool usage |
| `connector_pool_wait_count` | Waits caused by pool pressure | connection_id | Pool usage |
| `connector_pool_timeout_count` | Pool acquisition timeouts | connection_id | Pool usage |
| `circuit_state` | Circuit breaker state for connector | connection_id, state | Connector health |

## Audit and request-log metadata

| Field | Description |
|---|---|
| `request_id` | End-to-end correlation ID across gateway/admin logs |
| `client_id` | OAuth client / consumer identifier |
| `api_definition_id` | API definition UUID |
| `latency_ms` | End-to-end request latency |
| `source_db_duration_ms` | Time spent in source DB/connector |
| `cached` | Whether cache served the response |
| `actor_type` | user / client / system |
| `resource_type` | entity kind changed in audit trail |

## Operator usage notes

1. Use `request_id` to correlate request logs, audit entries, and upstream service logs.
2. Track `latency_ms` and `source_db_duration_ms` together to separate DB slowness from gateway overhead.
3. Watch `wait_count` and `timeout_count` before raising connector pool sizes.
4. Prefer Grafana dashboards for long-range trend analysis; use DDAG Monitoring for fast operational triage.
