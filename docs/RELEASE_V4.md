# DDAG v4 Release Checklist

Use this checklist before cutting a v4 preview or stable release.

## 1. Code health

- [ ] `go test ./internal/... -count=1` passes
- [ ] `go vet ./internal/...` passes
- [ ] `make build` completes for all binaries
- [ ] `cd apps/dashboard && pnpm install --frozen-lockfile && pnpm run build` succeeds
- [ ] migrations apply cleanly on fresh metadata DB
- [ ] migrations apply cleanly on existing v3 DB snapshot
- [ ] `docker compose config` validates

## 2. Security & Governance

- [ ] `OperationType` isolates read vs. write cache and retry pathways
- [ ] HMAC replay protection nonce cache is verified
- [ ] Secret AAD v2 updates retain legacy fallback decryption
- [ ] Dialect-aware SQL binder correctly ignores strings/quotes/brackets
- [ ] corrupt cache fallback triggers source DB fetch
- [ ] sensitive keys and credentials are redacted from logs and exports

## 3. Operations

- [ ] each service exposes `/healthz`, `/readyz`, `/metrics`
- [ ] `api_request_logs` records `operation` classification correctly
- [ ] Services dashboard tab displays live health and memory
- [ ] Jobs page executes safe maintenance workflows
- [ ] backup export and dry-run isolated restore drills are verified

## 4. Documentation

- [ ] `README.md` is technically focused and acknowledges database support status
- [ ] `docs/ARCHITECTURE.md` includes operation-aware governance and AAD secret paths
- [ ] `docs/DEPLOY_VPS.md` specifies multi-service systemd configurations
- [ ] `docs/SECURITY.md` defines deployment boundaries
- [ ] `docker-compose.yml` local onboarding stack is current

## 5. Release notes

Include:

- headline capabilities (HTTP `QUERY`, Secret AAD, Operation Governance);
- breaking changes and backward compatibility notes;
- SQL migration order;
- security and deployment requirements;
- known test limitations and integration coverage boundaries;
- explicit verification checklist.
