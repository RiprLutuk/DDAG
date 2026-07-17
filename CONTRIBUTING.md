# Contributing to DDAG

Thanks for your interest in DDAG! Contributions of all kinds are welcome —
bug reports, docs, features, and especially **integration testing for the
MySQL / Oracle / SQL Server connectors** (they build and share the connector
codebase but have not yet been exercised end-to-end against live engines).

## Development setup

**Prerequisites:** Go 1.26+, Node 20+ with Corepack (`pnpm`), a reachable PostgreSQL instance and Redis.
All connection settings are configured via environment variables — copy
[configs/.env.example](configs/.env.example) and adjust for your setup.

```bash
make build          # build all service binaries into ./bin
make seed           # create metadata DB, migrate, seed roles + a runnable demo
make dev            # start core services in the background
make dashboard      # run the Vite dashboard on :3000 (superadmin / Admin#12345)
```

## Before you open a PR

```bash
make vet            # go vet ./...
make test           # unit tests
gofmt -l internal cmd   # must print nothing (run `gofmt -w` to fix)
```

For dashboard changes also run a production build to catch template errors:

```bash
cd apps/dashboard && pnpm install --frozen-lockfile && pnpm run build
```

Run these checks locally before opening a PR. The repository workflow is configured to repeat Go formatting, vet, tests, dashboard build, Compose validation, and quickstart syntax checks when hosted CI is available; local checks remain the source of truth when an external CI account or billing issue prevents a run.

## Guidelines

- **Match the surrounding code** — comment density, naming, and idiom. Keep
  packages small and focused; one service = one `cmd/<svc>` + `internal/<svc>`.
- **Security is not optional.** Queries must use bound parameters (never string
  concatenation); never log secrets; keep raw driver errors out of client
  responses. New endpoints must enforce authorization in the backend, not just
  the UI.
- **Add tests** for pure logic (binders, validation, policy). See
  `internal/{connectors,policy,gateway}/*_test.go` for the style.
- Keep commits focused and write a clear message describing the *why*.

## Good first issues / help wanted

- Live integration tests for `connector-mysql`, `connector-oracle`,
  `connector-sqlserver` (a docker-compose with sample source DBs would be great).
- Live browser/SDK interoperability tests for RFC 10008 `QUERY` across proxies and client generators.
- Reusable example API catalogs for common PostgreSQL, MySQL, Oracle, and SQL Server schemas.
- Additional Grafana panels / alerting rules.

## Reporting bugs

Open an issue with steps to reproduce, expected vs. actual behavior, and the
relevant structured log lines (grep by `request_id` to trace across services).
Please redact any secrets.

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE).
