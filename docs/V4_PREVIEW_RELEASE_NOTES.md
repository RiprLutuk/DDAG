# DDAG v4 Release Notes

## Headline features

- **Self-managed service control plane**: Service registry, health/readiness/metrics links, and real-time system status.
- **RFC 10008 HTTP `QUERY` support**: Native support for safe, idempotent, cacheable query-with-body operations across the gateway and client generators.
- **Operation-Aware Governance**: Automatic derivation of operation types (`read`, `create`, `update`, `delete`, `command`) from HTTP methods (`GET`/`QUERY`, `POST`, `PUT`/`PATCH`, `DELETE`). Non-read mutations strictly bypass read-only cache layers and automated retries to prevent double-writes and stale side effects.
- **Backward-Compatible Secret AAD v2**: AES-GCM envelope encryption now binds secret identity, purpose, and key version as Associated Data (AAD) for new mutations while retaining transparent legacy decryption for existing ciphertext.
- **HMAC Replay Protection**: Redis-backed `SET NX EX` atomic nonce verification protects internal service-to-service communication from replay attacks.
- **Dialect-Aware SQL Binder**: Cross-dialect placeholder parser safely handles `$tag$`/`$$` dollar quotes, MySQL backticks, Oracle string literals, and bracket identifiers without corrupting queries.
- **Corrupt-Cache Recovery**: Automatic eviction and database fallback when cached response payloads fail validation.
- **Enterprise API Lifecycle**: Review/approve/publish gates, immutable revision snapshots, and draft-vs-published diffing.

## Migration notes

Apply migrations in numerical order:

1. `migrations/0005_service_registry.sql`
2. `migrations/0006_self_management.sql`
3. `migrations/0007_audit_operation.sql`

The `api_request_logs` table now includes the `operation` column indexed for operation-aware audit analysis and retention policies.

## Security notes

- All write/mutation requests (`POST`, `PUT`, `PATCH`, `DELETE`) are strictly forbidden from writing to or reading from response caches.
- Secret modification updates Associated Data (`purpose`, `key_version`) without guessing generic strings, preserving existing context.
- API publish is RBAC-gated (`ApproveAPI`, `PublishAPI`) and creates immutable snapshots in `api_revisions`.
- Sensitive keys are redacted across all structured logs and metadata export artifacts.

## Deployment notes

- **Control plane**: Vue 3 + Vite static SPA served directly by Caddy alongside `admin-backend` (`:8080`) and `auth-service` (`:8081`).
- **Data plane**: `api-gateway` (`:8082`) coordinating connection pools via `connector-postgres`, `connector-mysql`, `connector-oracle`, and `connector-sqlserver`.
- **Systemd-first**: All services build static CGO-free Go binaries optimized for single-VPS hosts under 1 GB RAM. Single compose file [`docker-compose.yml`](../docker-compose.yml) available for local developer onboarding.

## Known limitations

- External hosted CI/CD pipelines require active billing/account provisioning; local regression verification via `make test` and `make vet` remains authoritative.
- MySQL, Oracle, and SQL Server connectors share core integration patterns with PostgreSQL and are available via profiles, but broader multi-engine live integration matrix testing against diverse production versions is recommended prior to mission-critical deployment.

## Verification checklist

- `go test ./internal/... -count=1`
- `go vet ./internal/...`
- `cd apps/dashboard && pnpm install --frozen-lockfile && pnpm run build`
