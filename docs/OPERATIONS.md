# DDAG Operations Guide

## Configuration

All services are configured by environment variables (12-factor). The full list
with defaults is in [configs/.env.example](../configs/.env.example). Key groups:

| Group | Vars |
|---|---|
| Metadata DB | `DDAG_DB_HOST/PORT/USER/PASSWORD/NAME/SSLMODE`, pool: `DDAG_DB_MIN_CONNS`, `DDAG_DB_MAX_CONNS`, `DDAG_DB_MAX_CONN_LIFETIME`, `DDAG_DB_MAX_CONN_IDLE`, `DDAG_DB_CONNECT_TIMEOUT` |
| Redis | `DDAG_REDIS_ADDR`, `DDAG_REDIS_PASSWORD`, `DDAG_REDIS_DB` |
| Secrets | `DDAG_MASTER_KEY` (base64 of 32 bytes — `openssl rand -base64 32`) |
| OAuth2 | `DDAG_TOKEN_ISSUER`, `DDAG_TOKEN_AUDIENCE`, `DDAG_TOKEN_CLOCK_SKEW`, `DDAG_ACCESS_TOKEN_TTL`, `DDAG_REFRESH_TOKEN_TTL`, `DDAG_JWKS_URL`, `DDAG_JWKS_REFRESH` |
| Dashboard session | `DDAG_SESSION_SECRET`, `DDAG_SESSION_TTL`, `DDAG_SESSION_COOKIE_SECURE`, `DDAG_MAX_FAILED_LOGIN`, `DDAG_LOCKOUT_WINDOW`, `DDAG_DASHBOARD_ORIGINS` |
| Gateway | `DDAG_POLICY_MODE`, `DDAG_CACHE_MODE`, `DDAG_ROUTE_REFRESH`, `DDAG_DEFAULT_LIMIT`, `DDAG_MAX_LIMIT`, `DDAG_TRUSTED_PROXIES`, `DDAG_RATE_LIMIT_FAIL_MODE`, `DDAG_INTERNAL_AUTH_SECRET`, `DDAG_CONNECTOR_*_URL` |
| Circuit breaker | `DDAG_CB_MAX_REQUESTS`, `DDAG_CB_INTERVAL`, `DDAG_CB_TIMEOUT`, `DDAG_CB_FAILURE_THRESHOLD`, `DDAG_CB_FAILURE_RATIO` |
| Per-service | `DDAG_HTTP_ADDR` (defaults to each service's conventional port) |

> **Production checklist:** set a real `DDAG_MASTER_KEY` and `DDAG_SESSION_SECRET`,
> `DDAG_SESSION_COOKIE_SECURE=true`, set `DDAG_INTERNAL_AUTH_SECRET`, configure
> `DDAG_TRUSTED_PROXIES` to your ingress/load-balancer CIDRs, restrict
> `DDAG_DASHBOARD_ORIGINS`, enable TLS at the ingress, and point `DDAG_DB_*` at a
> backed-up PostgreSQL. With `DDAG_ENV=prod`, services refuse to boot if the
> master/session secrets are still defaults or secure cookies are disabled.

## First-time setup

```bash
make build
make seed          # creates DB 'ddag', migrates, seeds roles/super-admin + demo data
```

`make seed` runs `migrate --demo`, which:
1. applies all migrations,
2. seeds the permission catalog, the seven system roles, default scopes, and the
   `superadmin` user (password from `DDAG_SUPERADMIN_PASSWORD`, default
   `Admin#12345`),
3. provisions a real `ddag_demo` source database with sample rows, registers a
   `demo-postgres` connection, two published APIs, and the `app-brim` client
   (secret from `DDAG_DEMO_CLIENT_SECRET`, default `demo-secret-brim-001`).

For production, use `make seed-core` (no demo data). `admin-backend` also
auto-applies migrations and core seed on boot when `DDAG_AUTO_MIGRATE=true`.

Re-running any seed is idempotent.

## Running

- **Local processes:** `make dev` (background) / `make dev-stop` / `make dev-logs`.
  Run additional services directly: `make run-policy`, `make run-cache`,
  `make run-worker`, `make run-connector-postgres`, etc.
- **Docker:** `docker compose up -d --build`, then
  `docker compose run --rm migrate --demo`.
- **Kubernetes:** `kubectl apply -k deploy/k8s` (edit `secret.yaml` and
  `configmap.yaml` first), or use the Helm chart:
  `helm upgrade --install ddag deploy/helm/ddag -n ddag --create-namespace`.
  Profiles are available with `-f deploy/helm/ddag/values-dev.yaml`,
  `values-production.yaml`, and `values-enterprise.yaml`.

## Scaling

Stateless and horizontally scalable: `api-gateway`, `auth-service`,
`policy-engine`, `cache-service`, and all `connector-*`. HPAs for these are in
[deploy/k8s/hpa.yaml](../deploy/k8s/hpa.yaml) (CPU target 70%, up to 8 replicas).
Rate limiting and cache are Redis-backed, so they stay correct across replicas.
`admin-backend`, `worker` run at low replica counts.

Scale connectors independently per database type based on that source's load —
e.g. more `connector-postgres` replicas without touching Oracle.

## Health & monitoring

Every service exposes:

- `GET /healthz` — liveness
- `GET /readyz` — readiness (checks DB/Redis dependencies)
- `GET /metrics` — Prometheus metrics (`ddag_*`)

v2.0 metrics added for high-concurrency operations:

| Metric | Meaning |
|---|---|
| `ddag_singleflight_active` | Active cache-fill calls protected by singleflight |
| `ddag_singleflight_shared` | Requests that reused another in-flight cache fill |
| `ddag_metadata_sync_total` | Metadata refreshes triggered by Redis Pub/Sub |
| `ddag_circuit_state` | Circuit state by connection (`0=closed`, `1=half-open`, `2=open`) |
| `ddag_circuit_open_total` | Circuit open transitions |
| `ddag_circuit_half_open_total` | Circuit half-open transitions |

Prometheus scrape config: [deploy/prometheus/prometheus.yml](../deploy/prometheus/prometheus.yml).
Grafana datasource + dashboard auto-provision from
[deploy/grafana](../deploy/grafana) (panels: request rate, p95 latency, error
rate, cache hit ratio, token issue/fail, source-DB query p95, pool usage,
security events, connector errors).

## Routine operations

- **Add a source database:** Dashboard → Database Connections → New → fill host/
  port/credentials and pool sizes → **Test Connection** → Save. The secret is
  envelope-encrypted; the connection gets a health status.
- **Publish an API:** Dashboard → API Management → Create → pick a connection,
  write a `:param` SQL template, declare parameters, set scope + limits → **Test
  Query** → Save Draft → **Publish**. Publishing runs the safety validator; list
  calls are paginated at SQL level with connector-specific `LIMIT/OFFSET` or
  `OFFSET/FETCH` syntax.
- **Grant a client:** Dashboard → Clients → New (secret shown once) → assign
  scopes + APIs + rate limit + IP whitelist.
- **Rotate a client secret:** Clients → Rotate (new secret shown once; the
  client's refresh tokens are revoked).
- **Rotate the JWT signing key:** generate a new key in `jwt_signing_keys` and
  mark it active; old keys remain in JWKS until their tokens expire, then retire.
- **Purge cache:** Dashboard → Cache → Purge (per API) or Purge All.
- **Inspect circuit breakers:** Dashboard → Monitoring shows each connection's
  circuit state. The backend endpoint is `GET /api/circuit-breakers` and
  requires `view_circuit_state`.

## Backup & DR

- Back up the **metadata PostgreSQL** regularly (it holds users, clients, API
  definitions, policy, encrypted secrets, audit). This is the source of truth.
- Store `DDAG_MASTER_KEY` in a secret manager and back it up separately —
  without it, encrypted secrets cannot be decrypted.
- Redis holds cache + rate-limit counters only; it can be rebuilt (configure
  persistence/HA per environment for smoother failover).

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `401` from gateway | Missing/expired/invalid token; check `auth-service` and JWKS reachability |
| `403 FORBIDDEN` | Token lacks the API's scope, client not granted the API, or IP not whitelisted |
| `404 API_NOT_FOUND` | API not published, or route table not yet refreshed (`DDAG_ROUTE_REFRESH`) |
| `408 QUERY_TIMEOUT` | Source query exceeded the connection's query timeout |
| `429 RATE_LIMITED` | Client/API/IP rate limit hit (see `ddag_rate_limited_total`) |
| `502 CONNECTOR_ERROR` / `503` | Source DB unreachable, circuit breaker open, or connector unavailable; check connector logs + pool gauges |
| `503` when Redis is down | `DDAG_RATE_LIMIT_FAIL_MODE=closed`; use `open` for availability-priority fail-open behavior |
| Dashboard can't log in | Check `DDAG_DASHBOARD_ORIGINS` (CORS) and that `admin-backend` migrated/seeded |
| `too many failed attempts acquiring connection` | A source connection's pool can't connect — verify host/credentials via Test Connection |

## API consumer docs

The API gateway exposes generated documentation from published metadata:

- `GET /openapi.json` — OpenAPI 3.0 JSON.
- `GET /openapi.yaml` — OpenAPI 3.0 YAML.
- `GET /docs` — Swagger UI shell.
- `GET /api-catalog` — JSON catalog of published APIs.

When a bearer token is provided, catalog generation is filtered to scopes carried
by the token. The generated spec does not expose SQL templates or database
secrets.

Logs are structured JSON with a propagated `request_id`; grep by it to trace a
request across services.

## CI / release checks

The GitHub Actions workflow runs:

- Go formatting, build, `go vet`, `golangci-lint`, `gosec`, `govulncheck`, and
  unit tests.
- Integration tests tagged `integration` with Redis and PostgreSQL service
  containers.
- Dashboard `npm audit --audit-level=moderate` and production build.
- Helm lint/render for default, dev, production, and enterprise profiles.
- Docker build for gateway/dashboard images and Trivy scans for high/critical
  vulnerabilities.
