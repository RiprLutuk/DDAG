# DDAG — Global Launch Kit

This document contains ready-to-post launch copy, technical content drafts, and community submission templates for bringing DDAG to the international developer audience.

---

## 1. Hacker News — Show HN

### Title
```
Show HN: DDAG – Turn Postgres, MySQL, Oracle, and SQL Server into secure REST APIs
```

### Post body (Markdown)
```
Hi HN,

I built DDAG (Dynamic Database API Gateway) because I kept writing the same boilerplate backend service for every new database-backed API — auth, parameter binding, rate limiting, caching, audit logging, OpenAPI docs — over and over again.

DDAG takes a different approach: you define a route, bind typed parameters to a parameterized SQL template, set scopes and limits, publish it, and the gateway handles the rest. It supports PostgreSQL, MySQL/MariaDB, Oracle, and SQL Server out of the box with engine-specific connectors and connection pools.

A few things that make it different from existing tools:

- **Multi-database from day one.** Hasura and PostgREST are excellent for Postgres. DDAG also speaks Oracle and SQL Server natively, which matters a lot for teams with legacy enterprise databases.
- **Governance is not bolted on.** OAuth2 clients, scopes, per-client API grants, CIDR allowlists, rate limits, cache policy, audit trails, and lifecycle approvals (draft → review → published) are all part of the platform.
- **Self-hostable and lightweight.** The entire stack runs on a single VPS with under 1 GB RAM. No JVM, no heavyweight sidecars. Go binaries + Vue dashboard + Caddy.
- **Early adopter of RFC 10008.** DDAG is one of the first API gateways to implement the newly standardized HTTP QUERY method (RFC 10008, June 2026). Since OpenAPI 3.0 doesn't have a `query` operation, we built a compatibility layer: the spec emits `post` + `x-ddag-http-method: QUERY`, and the Swagger UI displays and executes it as native `QUERY`.

The dashboard is a working control plane — not a mock — with RBAC, connection management, API lifecycle, and a live Swagger UI docs page that's themeable.

Try it:
```bash
git clone https://github.com/RiprLutuk/DDAG.git
cd DDAG
./scripts/quickstart.sh
```

Live demo: https://ddag.demo.pandanteknik.com

I'd love feedback on the architecture, the QUERY compatibility approach, and what database engines you'd want supported next.
```

---

## 2. Reddit — r/selfhosted

### Title
```
DDAG — Self-hostable API gateway for Postgres, MySQL, Oracle & SQL Server (Go, < 1 GB RAM, no JVM)
```

### Post body (Markdown)
```
I've been working on DDAG, a self-hosted Dynamic Database API Gateway. It turns your databases into secure REST APIs with OAuth2, rate limiting, caching, audit logs, and auto-generated OpenAPI docs — without writing a backend service per endpoint.

**Why self-host?**
- Entire stack runs on a single VPS with < 1 GB RAM
- Go binaries, no Docker required for production (systemd + Caddy)
- Docker Compose available for local dev
- No vendor lock-in, no SaaS dependency

**Supported databases:** PostgreSQL, MySQL/MariaDB, Oracle, SQL Server

**Key features:**
- OAuth2 (RS256 JWT/JWKS), scopes, per-client grants, CIDR allowlists
- RBAC with lifecycle approvals (draft → review → published)
- Parameterized SQL with typed bound parameters (read-only by default)
- Redis-backed rate limiting and TTL caching
- Circuit breakers and backpressure
- Per-request audit logs and Prometheus metrics
- Auto-generated OpenAPI 3.0 spec + themed Swagger UI
- Early adopter of RFC 10008 (HTTP QUERY method)

Quick start:
```bash
git clone https://github.com/RiprLutuk/DDAG.git && cd DDAG && ./scripts/quickstart.sh
```

Live demo: https://ddag.demo.pandanteknik.com

What would you want to see before adopting this for your own infrastructure? Happy to answer questions about the architecture or deployment.
```

---

## 3. Reddit — r/golang

### Title
```
DDAG — A Go-based API gateway with engine-specific connectors for Postgres, MySQL, Oracle & SQL Server
```

### Post body (Markdown)
```
I built DDAG as a Go-based dynamic API gateway that publishes database-backed REST APIs with governance built in.

**Architecture highlights:**
- Single-binary Go services (admin-backend, auth-service, api-gateway, connectors)
- Engine-specific connectors with per-source connection pools and circuit breakers
- Gateway router with native support for the HTTP QUERY method (RFC 10008)
- In-process or remote policy engine and cache (Redis-backed)
- Manifest-based deployments; no config file parsing on the hot path
- Full observability: `/healthz`, `/readyz`, `/metrics` (Prometheus)

**Why Go?**
- Memory footprint: entire stack runs under 1 GB RAM
- Static binaries make deployment trivial (systemd + Caddy, no Docker needed)
- `net/http` handles arbitrary method strings, so we adopted `QUERY` before Go added an `http.MethodQuery` constant

The full Go test suite passes (`go test ./...`) and the dashboard is a Vue 3 + Vite SPA.

Repo: https://github.com/RiprLutuk/DDAG
Live demo: https://ddag.demo.pandanteknik.com

Feedback on the connector pool design, the QUERY compatibility layer, or the policy engine integration would be greatly appreciated.
```

---

## 4. Technical Blog Article — RFC 10008 adoption

### Title
```
Why we implemented the HTTP QUERY method (RFC 10008) before OpenAPI supported it
```

### Draft (Markdown)
```
# Why we implemented the HTTP QUERY method (RFC 10008) before OpenAPI supported it

On June 15, 2026, the IETF published [RFC 10008](https://www.rfc-editor.org/rfc/rfc10008.html), officially standardizing the HTTP `QUERY` method. `QUERY` is safe and idempotent — like `GET` — but unlike `GET`, it can carry a request body. This makes it the ideal method for complex search and filtering operations where query parameters alone are insufficient.

At DDAG, we decided to become an early adopter. Here's why and how.

## The problem

Consider an API that searches employees by complex criteria:

```json
{
  "department": "IT",
  "active": true,
  "skills": ["Go", "PostgreSQL"],
  "hire_date": { "after": "2023-01-01" }
}
```

With `GET`, you'd encode this as query parameters (fragile, length-limited) or make it a `POST` (semantically wrong for a safe, idempotent read).

`QUERY` solves this cleanly: the request body carries the filter, the method signals that it's a safe read, and intermediaries can cache it.

## The catch: OpenAPI doesn't know about QUERY

OpenAPI 3.0 and 3.1 define a fixed set of Path Item operations: `get`, `post`, `put`, `patch`, `delete`, `head`, `options`, `trace`. There is no `query` operation. Validators reject unknown keys. SDK generators don't handle it. Swagger UI can't render it.

So if we published `query:` in our spec, every tool in the ecosystem would break.

## Our compatibility layer

We chose a pragmatic approach that keeps the spec valid while preserving full runtime fidelity:

1. **In the OpenAPI spec**, `QUERY` operations are emitted as `post` with a custom extension:
   ```yaml
   post:
     operationId: queryEmployees
     x-ddag-http-method: QUERY
     requestBody:
       content:
         application/json:
           schema:
             type: object
   ```

2. **In Swagger UI**, our custom interceptor reads the extension, displays a `QUERY` badge instead of `POST`, and when the user clicks "Execute", the outgoing request is rewritten from `POST` to `QUERY`.

3. **In the gateway**, `QUERY` is a first-class route. The router matches it independently. CORS preflight advertises it. Audit logs record the actual method.

4. **In client generators**, the generated code produces a `POST` by default. A transport adapter inspects `x-ddag-http-method` and rewrites the outgoing method. This is documented in our [HTTP QUERY compatibility guide](../docs/HTTP_QUERY.md).

## What we learned

- **Standards move slowly; tooling moves slower.** RFC 10008 was published in June 2026. As of July 2026, Go 1.26 still doesn't have an `http.MethodQuery` constant. But `net/http` handles arbitrary method strings, so we could adopt it immediately.
- **CORS is the real bottleneck.** Even when the server and client support `QUERY`, every intermediary — CDN, WAF, load balancer, API gateway — must allow it through. We verified this end-to-end with preflight probes.
- **OpenAPI extensions are powerful.** They let us stay compatible with the entire ecosystem while pioneering a new standard. The `x-ddag-http-method` extension is a one-field bridge between today's tooling and tomorrow's standard.

## Try it

```bash
curl --request QUERY 'https://ddag.demo.pandanteknik.com/api/v1/employees/search' \
  --header 'Authorization: Bearer <token>' \
  --header 'Content-Type: application/json' \
  --data '{"department":"IT"}'
```

DDAG is open source: [github.com/RiprLutuk/DDAG](https://github.com/RiprLutuk/DDAG)
```

---

## 5. Awesome Lists — PR templates

### awesome-go submission

**Category:** Software Packages → API Management, or Database Tools

```markdown
- [DDAG](https://github.com/RiprLutuk/DDAG) - Dynamic Database API Gateway. Turn PostgreSQL, MySQL, Oracle, and SQL Server into secure, documented REST APIs with OAuth2, rate limiting, caching, and audit trails. Self-hostable, < 1 GB RAM.
```

### awesome-selfhosted submission

**Category:** Miscellaneous or Database Management

```markdown
- [DDAG](https://github.com/RiprLutuk/DDAG) - Dynamic Database API Gateway that turns databases into secure REST APIs with OAuth2, RBAC, rate limiting, caching, audit logs, and auto-generated OpenAPI docs. Supports PostgreSQL, MySQL, Oracle, and SQL Server. ([Demo](https://ddag.demo.pandanteknik.com), [Source Code](https://github.com/RiprLutuk/DDAG), [MIT License](https://github.com/RiprLutuk/DDAG/blob/main/LICENSE))
```

---

## 6. Product Hunt — draft

### Tagline
```
Turn any database into secure REST APIs — self-hosted, < 1 GB RAM
```

### Description
```
DDAG is a self-hosted Dynamic Database API Gateway. Define a route, bind parameters to SQL, set scopes and limits, and publish — the gateway handles OAuth2, rate limiting, caching, audit logs, and OpenAPI docs automatically.

Unlike Hasura or PostgREST, DDAG supports PostgreSQL, MySQL, Oracle, and SQL Server natively. The entire stack runs on a single VPS with under 1 GB RAM — no JVM, no heavyweight containers.

DDAG is also one of the first API gateways to implement RFC 10008, the newly standardized HTTP QUERY method.

🔒 OAuth2, RBAC, CIDR allowlists, audit trails
⚡ Redis rate limiting, TTL cache, circuit breakers
📖 Auto-generated OpenAPI + themed Swagger UI
🐳 Docker Compose for dev, systemd + Caddy for production
```

---

## 7. Social media — short posts

### Twitter/X
```
🚀 DDAG: Turn Postgres, MySQL, Oracle & SQL Server into secure REST APIs.

Self-hosted. < 1 GB RAM. No JVM. No SaaS lock-in.

OAuth2 + RBAC + rate limiting + caching + audit logs + OpenAPI docs — all built in.

One of the first gateways to support RFC 10008 HTTP QUERY.

github.com/RiprLutuk/DDAG
```

### LinkedIn
```
I'm excited to share DDAG — an open-source Dynamic Database API Gateway that turns enterprise databases (PostgreSQL, MySQL, Oracle, SQL Server) into secure, governed REST APIs.

Key differentiators:
- Multi-database support from day one (including Oracle and SQL Server)
- Governance built in: OAuth2, RBAC, rate limiting, audit trails, lifecycle approvals
- Lightweight: runs on a single VPS with < 1 GB RAM
- Early adopter of RFC 10008 (HTTP QUERY method)

If you're working with legacy databases and need a secure API layer without building a custom backend per endpoint, check it out.

GitHub: https://github.com/RiprLutuk/DDAG
Live demo: https://ddag.demo.pandanteknik.com

#OpenSource #API #Go #Database #SelfHosted #DeveloperTools
```

---

## Checklist before launching

- [ ] README.md is polished and English-first with clear hook
- [ ] Live demo is accessible and functional
- [ ] All tests pass (`go test ./...`)
- [ ] Dashboard build is fresh (`pnpm build`)
- [ ] Docker Compose quickstart works on a clean machine
- [ ] LICENSE file exists (MIT)
- [ ] CONTRIBUTING.md exists
- [ ] GitHub repository has topics: `api-gateway`, `go`, `self-hosted`, `database`, `openapi`, `oauth2`, `rest-api`, `postgres`, `mysql`, `oracle`, `sqlserver`
- [ ] GitHub repository description is set: "Dynamic Database API Gateway — turn governed SQL into secure REST APIs. Self-hosted, multi-database, < 1 GB RAM."
- [ ] Screenshots/GIFs ready for README and Product Hunt
- [ ] This launch kit has been reviewed and copy is final
