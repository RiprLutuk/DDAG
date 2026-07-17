# DDAG Architecture

## Repository layout

```
cmd/                     # one main package per service (thin wrappers)
  admin-backend/ auth-service/ api-gateway/ policy-engine/ cache-service/
  worker/ connector-{postgres,mysql,oracle,sqlserver}/ migrate/
internal/
  adminsvc/              # dashboard API: auth/session, RBAC, metadata CRUD, audit
  authservice/           # OAuth2 endpoints + RS256 signing-key management + JWKS
  gatewaysvc/            # data-plane orchestration (HTTP handler)
  gateway/               # route table, param resolution/validation, connector client
  policyengine/          # standalone policy decision service
  cacheservice/          # standalone cache management service
  workersvc/             # background maintenance loop
  connector/             # shared connector service (Run(dbType))
  connectors/            # per-driver query execution + :named-param binding
  connectorpool/         # registry of per-connection pools + pool metrics
  policy/                # scope, IP whitelist, Redis rate limiter (pure logic)
  cache/                 # Redis response cache + key strategy
  auth/                  # password hashing, JWT issue/parse, JWKS, token hashing
  secret/                # envelope-encrypted secret store (AES-GCM)
  store/                 # metadata DB queries (pgx)
  models/                # row structs
  rbac/                  # permission catalog + default role→permission map
  config/ db/ httpx/ logging/ metrics/ server/   # shared plumbing
  bootstrap/             # migration runner + idempotent core/demo seed
migrations/              # embedded, versioned SQL migrations
apps/dashboard/          # Vue 3 + Vite static admin SPA
deploy/                  # Dockerfiles, k8s, prometheus, grafana
```

## Two planes

### Control plane (admins)
`dashboard → admin-backend → metadata PostgreSQL`

- Session auth: login verifies a bcrypt password, issues a signed http-only
  cookie; failed logins increment a counter and lock the account temporarily.
- Every request passes through RBAC middleware that checks the user's effective
  permissions (resolved from role→permission). Permissions are enforced server
  side, independent of what the UI shows.
- All mutations write an append-only audit row.

### Data plane (client apps)
`api-gateway → policy → cache → connector → source DB`

1. **Token verify** — the gateway validates the bearer JWT locally using public
   keys fetched from `auth-service`'s JWKS (refreshed periodically). No call to
   the auth service per request.
2. **Route match** — the gateway holds a route table built from *published* API
   definitions (refreshed on an interval). Literal path segments win over
   `{param}` segments.
3. **Policy** — scope (token must carry the API's required scope), per-client API
   access grant, IP whitelist (single IP or CIDR), and rate limiting. Rate limits
   use Redis fixed windows (per second/minute/hour/day) so they hold across
   gateway replicas. Runs in-process by default, or delegates to `policy-engine`
   when `DDAG_POLICY_MODE=remote`.
4. **Cache** — read operations may use a cache rule. The gateway builds a key
   (path + query/body, optionally varied by client) and serves a hit directly;
   misses fall through and are populated with the configured TTL. Write and
   unknown operations always bypass both cache reads and cache writes, even if
   a cache rule is misconfigured.
5. **Connector** — the gateway POSTs the query template + bound parameters +
   limit + timeout to the connector for that DB type. The connector resolves the
   connection, decrypts the secret, acquires a pooled connection, binds
   parameters, runs the query under a timeout, and returns rows. Raw driver
   errors are classified into stable DDAG error codes and never leaked.

## Connection pooling

Each source connection has its own pool, configured per connection
(`min_pool_size`, `max_pool_size`, connect/query timeouts, max conn
lifetime/idle). `connectorpool.Registry` keys pools by `connection_id` +
`config_version`; changing a connection's config bumps the version so the next
acquire rebuilds the pool and the stale one is drained. Pool gauges are exported
per connection: v3 active/idle/wait/timeout metrics use the `ddag_db_pool_*`
prefix, while configured max size remains `ddag_pool_max_connections`.

All DDAG Prometheus metrics include the logical process name as a `service`
const label, for example `service="connector-postgres"`. Connector-specific
metrics add low-cardinality variable labels such as `connection` and `db_type`.

PostgreSQL uses `pgxpool`; MySQL/Oracle/SQL Server use `database/sql` with the
pure-Go drivers (so all services build static, CGO-free images). A connectivity
"Test connection" path uses a one-shot connection rather than a pool.

## Query safety

`:named` parameters are rewritten to the driver's placeholder style by a
parser-aware binder that ignores `:` inside string literals, comments, and the
PostgreSQL `::` cast. Values are always passed as bound arguments. At publish
time, `ValidateForPublish` rejects: empty templates, multiple statements, write
statements (unless the API is explicitly a write API), undeclared/unused
parameters, and a missing row limit.

## Secrets

`secret.EnvelopeStore` encrypts each secret with a per-secret data key wrapped by
a master key (AES-GCM) and stores the ciphertext in the metadata DB. New
ciphertext uses versioned associated data bound to the secret identity, purpose,
and key version; legacy ciphertext remains readable through the compatibility
path. Source-DB passwords and JWT private keys are stored this way. The master
key comes from `DDAG_MASTER_KEY` (or a secret manager in production). Structured
logging redacts sensitive keys, so secrets never appear in logs.

## OAuth2 & signing keys

`auth-service` issues RS256 JWTs (claims: `client_id`, `scope`, `iss`, `exp`,
`jti`, …) and opaque refresh tokens (stored hashed). On first boot it generates
an RSA keypair, stores the private key envelope-encrypted, and publishes the
public key at `/.well-known/jwks.json`. Refresh rotates: using a refresh token
revokes it and issues a fresh pair. Keys are identified by `kid` so rotation is
possible without invalidating in-flight tokens.

## Metadata data model

Identity (`users`, `roles`, `permissions`, `role_permissions`, `user_roles`),
clients & scopes (`clients`, `scopes`, `client_scopes`, `client_api_access`,
`refresh_tokens`), connectivity & APIs (`database_connections`,
`api_definitions`, `api_parameters`, `cache_rules`, `rate_limit_rules`,
`ip_whitelists`, `jwt_signing_keys`, `secrets`), and logs (`audit_logs` —
append-only via trigger — and `api_request_logs`). Schema lives in
`migrations/` and is applied by `bootstrap.Migrate`.

## Standard response envelope

```json
{ "success": true, "request_id": "req-…", "data": {…}, "meta": { "cached": false, "duration_ms": 53 } }
```

List responses add `pagination`; errors return
`{ "success": false, "request_id": "…", "error": { "code": "FORBIDDEN", "message": "…" } }`.
A request ID is generated per request and propagated through logs and to
connectors.
