# DDAG — Dynamic Database API Gateway

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![Nuxt 3](https://img.shields.io/badge/Nuxt-3-00DC82?logo=nuxtdotjs&logoColor=white)](apps/dashboard)
[![Release](https://img.shields.io/github/v/release/RiprLutuk/DDAG)](https://github.com/RiprLutuk/DDAG/releases/latest)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

Backend-as-a-Service that turns SQL queries against many database engines into
secured, observable, cached REST APIs — built and managed entirely from an admin
dashboard, with **no per-API deployment**.

> Admins build an API from the dashboard, developers consume it with an OAuth2
> token, and every access stays authenticated, authorized, rate-limited, cached,
> audited, and monitored.

This repository is a **fully working** implementation (not mocked): a Go
microservice backend, a real Nuxt 3 admin dashboard wired to live APIs, real
connection pooling per source database, OAuth2, RBAC, caching, metrics, and
container/Kubernetes deployment.

---

## Highlights

- **Dynamic API builder** — define path, method, SQL template, parameters, scope,
  cache and limits from the dashboard; publish to make it live instantly.
- **Multi-database connectors** — PostgreSQL, MySQL/MariaDB, Oracle, SQL Server,
  each a separately deployable, independently scalable pod.
- **Per-database connection pooling** — every connection has its own pool, tuned
  from config (min/max size, timeouts, lifetimes). Connections are expensive, so
  they are pooled and reused, never opened per request.
- **OAuth2** — `client_credentials` + `refresh_token` grants, RS256 JWTs with a
  rotatable signing key and JWKS, plus revoke/introspect.
- **Policy engine** — scope checks, per-client API access, IP whitelist (CIDR),
  and Redis-backed rate limiting that is consistent across replicas.
- **Caching** — per-endpoint TTL cache with vary-by-client and manual purge.
- **Enterprise hardening** — singleflight-protected cache miss fills, connector
  circuit breakers, trusted proxy IP handling, configurable rate-limit fail mode,
  service-to-service HMAC auth, JWT audience validation, CSRF-protected
  dashboard sessions, and generated OpenAPI.
- **v3 query readiness** — optional safe query-builder metadata supports
  whitelisted filters, sorting, inner/left joins, SQL preview, explain, and
  gateway backpressure for burst traffic.
- **Security by default** — parameter binding only (no string concatenation),
  read-only queries unless explicitly flagged, secrets envelope-encrypted at rest
  and redacted from logs, append-only audit log enforced in the database.
- **Observability from day one** — every service exposes `/metrics`, `/healthz`,
  `/readyz`; Prometheus scrape config and a Grafana dashboard are included,
  including singleflight, metadata-sync, circuit-breaker, pool, cache, and queue
  metrics.

---

## Architecture

```
                         ┌──────────────┐
  Client app ──Bearer──▶ │  api-gateway │ ─▶ policy (scope/ip/rate) ─▶ cache ─▶ connector-* ─▶ source DB
                         └──────┬───────┘                                              (own pool)
                                │ verifies JWT via JWKS
                         ┌──────▼───────┐
                         │ auth-service │  OAuth2 token / refresh / revoke / introspect / JWKS
                         └──────────────┘
  Admin ──cookie──▶ dashboard (Nuxt) ─▶ admin-backend ─▶ metadata PostgreSQL (users, clients, APIs, policy, audit)
                         (RBAC, CRUD)
  worker · policy-engine · cache-service   run as their own pods
  Prometheus + Grafana scrape every service
```

One service = one process = one container/pod. Source databases and the metadata
database are **external** (reached over the network); only Redis and the
monitoring stack are containerized in compose (per the product principle).

| Service | Port | Responsibility |
|---|---|---|
| `admin-backend` | 8080 | Dashboard API: login/session, RBAC, metadata CRUD, audit |
| `auth-service` | 8081 | OAuth2 tokens, refresh, revoke, introspect, JWKS |
| `api-gateway` | 8082 | Dynamic routing, token verify, policy, cache, connector calls |
| `policy-engine` | 8083 | Standalone policy decisions (optional remote mode) |
| `cache-service` | 8084 | Standalone cache rule/purge API |
| `worker` | 8085 | Background maintenance (token cleanup, cache warming hook) |
| `connector-postgres` | 8090 | PostgreSQL query execution + pool |
| `connector-mysql` | 8091 | MySQL/MariaDB |
| `connector-oracle` | 8092 | Oracle |
| `connector-sqlserver` | 8093 | SQL Server |
| `dashboard` | 3000 | Nuxt 3 admin UI |

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detail.

---

## Quick start (local)

**Prerequisites:** Go 1.26+, Node 20+, a reachable **PostgreSQL** instance (used
as the metadata store) and **Redis**. All connection details — host, port,
credentials, SSL, and pool sizes — are configured through environment variables;
copy [configs/.env.example](configs/.env.example) and adjust for your
environment.

```bash
# 1. Build all service binaries
make build

# 2. Create the metadata DB, run migrations, and seed roles + a working demo
#    (creates a real ddag_demo source database with sample data, a demo
#     connection, two published APIs, and a demo client).
make seed

# 3. Start the core services (admin, auth, gateway, connector-postgres)
make dev        # or: ./scripts/dev.sh start

# 4. Start the dashboard
make dashboard  # Nuxt dev server on http://localhost:3000
```

Open http://localhost:3000 and sign in:

- **User:** `superadmin`  **Password:** `Admin#12345`

Everything in the dashboard is live — create a connection, build an API, assign
it to a client, and call it.

### Try the data plane with curl

```bash
# Get a token for the seeded demo client
TOKEN=$(curl -s localhost:8081/oauth/token -H 'Content-Type: application/json' \
  -d '{"client_id":"app-brim","client_secret":"demo-secret-brim-001","grant_type":"client_credentials"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')

# Call a published dynamic API (gateway → policy → cache → connector → real DB)
curl localhost:8082/api/v1/brim/sites/ABC123 -H "Authorization: Bearer $TOKEN"

# Search work orders (POST body parameter)
curl -XPOST localhost:8082/api/v1/brim/workorders/search \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"status":"OPEN"}'

# Download generated OpenAPI for published APIs
curl localhost:8082/openapi.json -H "Authorization: Bearer $TOKEN"
curl localhost:8082/openapi.yaml -H "Authorization: Bearer $TOKEN"
```

---

## Run the whole stack with Docker

Metadata/source databases stay external; compose runs Redis, all DDAG services,
the dashboard, Prometheus and Grafana.

```bash
docker compose up -d --build
docker compose run --rm migrate --demo     # one-time seed
# dashboard:   http://localhost:3000
# gateway:     http://localhost:8082
# prometheus:  http://localhost:9090
# grafana:     http://localhost:3001  (admin/admin)
```

Set `DDAG_DB_HOST`, `DDAG_DB_USER`, `DDAG_DB_PASSWORD` (and a real
`DDAG_MASTER_KEY`) in a `.env` file next to `docker-compose.yml`.

---

## Kubernetes

Manifests in [deploy/k8s](deploy/k8s) (Deployments, Services, ConfigMap, Secret,
HPA for high-traffic services, Ingress, ServiceMonitor, Redis):

```bash
kubectl apply -k deploy/k8s
```

Helm profiles are also available:

```bash
helm upgrade --install ddag deploy/helm/ddag -n ddag --create-namespace
helm upgrade --install ddag deploy/helm/ddag -n ddag -f deploy/helm/ddag/values-enterprise.yaml
```

The CI workflow renders every Helm profile with `helm template` and builds the
gateway/dashboard container images before Trivy scanning.

Replace the placeholder values in `secret.yaml` (master key, session secret, DB
password) using your secret manager, and point `configmap.yaml` at your external
metadata DB. Connector services are deliberately not exposed via Ingress.

---

## Make targets

```
make build      # build all service binaries into ./bin
make test       # run unit tests
make vet        # go vet
make migrate    # create metadata DB + apply migrations
make seed       # migrate + core seed + demo data
make dev        # build, seed, start core services in the background
make dev-stop   # stop background services
make dashboard  # run the Nuxt dashboard dev server
```

---

## Security model (summary)

- API consumers authenticate with OAuth2 bearer tokens; the dashboard uses a
  signed, http-only session cookie with lockout after repeated failures and a
  double-submit CSRF token for state-changing requests.
- Authorization is enforced **in the backend**, never only in the UI: RBAC for
  the dashboard, scope + per-client API grants + IP whitelist for the data plane.
- Queries use bound parameters exclusively; write statements are rejected at
  publish time unless an API is explicitly marked as a write API.
- Source-DB secrets are envelope-encrypted at rest and never logged; structured
  logs redact sensitive keys.
- The audit log is append-only (enforced by a database trigger).
- Gateway-to-connector calls are HMAC-signed when `DDAG_INTERNAL_AUTH_SECRET` is
  configured; connectors reject unsigned internal requests in that mode.

---

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — services, data flow, internals
- [docs/OPERATIONS.md](docs/OPERATIONS.md) — deploy, configure, run, troubleshoot
- [docs/DEPLOY_VPS.md](docs/DEPLOY_VPS.md) — single-VPS systemd + Caddy runbook
- [docs/TESTING_V3.md](docs/TESTING_V3.md) — v3 load-test scripts and reports

---

## Contributing

Contributions are very welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). Areas
that especially need help: live integration testing for the MySQL / Oracle /
SQL Server connectors, an embedded Swagger UI page, and more Grafana panels.

## License

[MIT](LICENSE) © Ripr Lutuk
