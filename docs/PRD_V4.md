# DDAG PRD v4 — Self-Managed Enterprise OSS API Platform

**Status:** Draft → execution-ready  
**Target users:** DBA, infra, backend, data-platform engineers  
**Product direction:** clean, fast, self-managed, multi-service ready, enterprise-grade features while staying open-source and simple to run.

---

## 1. Problem Statement

DDAG already provides a dynamic API gateway over relational databases with OAuth2, RBAC, audit logs, caching, rate limits, connectors, and a Vue 3 + Vite static dashboard. The next version must make DDAG easier to operate as a real long-running platform:

- many internal services must be discoverable, observable, and manageable from one control plane;
- more APIs, clients, connections, logs, and services must remain fast without UI/backend slowdown;
- operators need self-management features instead of editing config/systemd manually;
- enterprise teams need approval, governance, auditability, policy, backup, and operational safety;
- OSS users still need a clean single-node path without Kubernetes complexity.

---

## 2. Product Goals

1. **Self-managed operations** — admin can inspect services, health, config, versions, queues, pools, circuits, and maintenance state from dashboard/API.
2. **Enterprise-ready governance** — approval workflow, environment promotion, API versioning, change history, stronger RBAC, audit export.
3. **High-scale admin UX** — every list page uses server-side pagination/search/sort and consistent DataTables-like ergonomics.
4. **Clean service architecture** — formal service registry, capability model, service-to-service auth, graceful degradation, and readiness signals.
5. **Fast data plane** — protect gateway/connectors with backpressure, circuit breakers, pool visibility, cache/singleflight, and bounded logs.
6. **OSS-first** — all core features remain Apache/MIT-style OSS-friendly, deployable on VPS/systemd or containers without SaaS lock-in.

---

## 3. Non-Goals for v4

- No hosted SaaS dependency.
- No mandatory Kubernetes requirement.
- No heavy workflow engine unless simple in-process/background worker is insufficient.
- No proprietary enterprise edition split in v4.
- No exposing connector/database ports publicly.

---

## 4. Current Scan Summary

### Existing strengths

- Separate services already exist: `admin-backend`, `auth-service`, `api-gateway`, `policy-engine`, `cache-service`, `worker`, and `connector-*`.
- Core security exists: OAuth2/JWT/JWKS, dashboard session, RBAC, CSRF, audit logs, encrypted secrets, parameter binding.
- Data-plane resilience exists: Redis rate limit, cache, connector pools, circuit breakers, singleflight/backpressure foundations.
- Dashboard has admin pages for APIs, clients, connections, users, roles, scopes, policies, monitoring, logs, audit, settings.
- Docs already describe VPS/systemd, architecture, operations, and testing.

### Gaps to close

- Services are configured but not fully **self-discovered/self-managed** from metadata.
- Settings page is still too basic; no operator control center for health, config, maintenance, backups, jobs, and feature flags.
- Some admin list endpoints/pages still need consistent server-side table contract.
- API lifecycle needs stronger workflow: draft → review → approved → published → deprecated/archived, with change diff.
- No first-class multi-environment promotion path: dev/staging/prod snapshots/export/import.
- No backup/restore UX for metadata/config.
- Logs/audit need export, retention, and advanced filtering for enterprise use.
- Service scaling strategy exists in docs but needs visible service registry/capabilities.

---

## 5. Personas

| Persona | Needs |
|---|---|
| DBA / Infra Engineer | safely expose database-backed APIs, inspect pools, test connections, control limits |
| Backend Engineer | consume stable OpenAPI, OAuth2, predictable errors, versioned APIs |
| Platform Owner | self-manage services, backups, health, upgrades, governance, audit |
| Security / Compliance | RBAC, approval, audit export, secret safety, token/key rotation |
| OSS Maintainer | simple setup, clean docs, modular features, issue-friendly architecture |

---

## 6. v4 Feature Pillars

## Pillar A — Service Control Plane

### A1. Service registry

Create metadata-backed registry of DDAG services.

**Service fields:**
- `id`, `name`, `kind`, `base_url`, `enabled`, `managed_by`, `version`, `commit_sha`
- `health_url`, `ready_url`, `metrics_url`
- `capabilities` JSON: `connector:postgres`, `policy:remote`, `cache:remote`, etc.
- `last_seen_at`, `last_health_status`, `last_error`

**Acceptance criteria:**
- Admin API can list/register/update services.
- Dashboard shows services grouped by role.
- Health/ready checks are visible.
- Existing static env config remains supported.

### A2. Service health center

Dashboard page: **Operations → Services**.

Show:
- status, uptime, version, latency, last check;
- quick links to metrics/ready/health;
- degraded state when health fails;
- refresh button.

### A3. Capability-aware routing

Gateway and admin UI should understand available connector/capability state.

Examples:
- if `connector-oracle` disabled/down, Oracle connection creation shows warning;
- if cache-service remote mode unavailable, show fallback mode;
- if policy-engine remote mode unavailable, warn before switching mode.

---

## Pillar B — Enterprise API Lifecycle

### B1. API approval workflow v4

Formalize statuses:

`DRAFT → REVIEW → APPROVED → PUBLISHED → DEPRECATED → ARCHIVED`

Rules:
- only `APPROVED` APIs can be published;
- publish creates immutable revision snapshot;
- changes after publish create a new draft revision;
- approvals write audit entries with actor/comment.

### B2. API revisions and diff

Store versioned API definitions.

Show:
- current draft vs published diff;
- SQL/template parameter diff;
- policy/cache/rate-limit diff;
- who changed what and when.

### B3. Environment promotion

Export/import deployment bundles:
- APIs + parameters + scopes + cache/rate-limit rules;
- no secrets by default;
- signed checksum;
- dry-run validation before import.

---

## Pillar C — Self-Management & Operations

### C1. Settings v4

Upgrade settings into typed categories:
- Security
- Gateway
- Connectors
- Cache
- Rate limits
- Observability
- Retention
- Feature flags

Each setting has:
- key, value, type, scope, description, default, restart requirement, updated_by.

### C2. Maintenance jobs

Add jobs page for:
- cleanup expired tokens;
- purge old request logs;
- purge audit exports;
- warm selected cache keys;
- service health sweep;
- metadata backup.

### C3. Backup/restore

Admin can export metadata backup:
- JSON or SQL dump style;
- excludes/encrypts secrets;
- includes version/migration metadata;
- restore supports dry-run.

---

## Pillar D — Scalable Admin UX

### D1. Unified table API contract

All list endpoints should support:

```text
?page=1&limit=10&search=...&sort_by=...&sort_dir=asc|desc
```

Response:

```json
{
  "items": [],
  "pagination": { "page": 1, "limit": 10, "total": 0 }
}
```

Targets:
- users
- roles
- scopes
- clients
- connections
- APIs
- cache rules
- rate limits
- IP whitelists
- request logs
- audit logs
- services
- jobs
- backups

### D2. Clean dashboard navigation

Add grouped sidebar:
- Overview
- Build: APIs, Connections
- Access: Clients, Scopes, Users, Roles
- Policy: Rate Limits, IP Whitelists, Cache
- Operations: Services, Jobs, Backups, Monitoring, Logs, Audit
- System: Settings

### D3. Performance rules

- no page loads unbounded lists;
- no table fetch without limit;
- debounce search;
- skeleton/loading state;
- all destructive actions have confirmation;
- avoid visual clutter: minimal dark enterprise theme.

---

## Pillar E — Observability, Audit, Compliance

### E1. Audit v4

- advanced filters: actor, action, entity, date range, request id;
- export CSV/JSON;
- retention policy;
- hash-chain option for tamper-evidence.

### E2. Request logs v4

- filters: API, client, status, duration range, cached, request id;
- top slow APIs panel;
- error rate panel;
- retention job.

### E3. Metrics catalog

Dashboard page that documents available metrics and service labels.

---

## Pillar F — Developer & OSS Experience

### F1. Docs v4

Add docs:
- PRD v4
- task tracker
- service registry design
- API lifecycle design
- backup/restore guide
- operator guide

### F2. Release process

- changelog generation convention;
- release checklist;
- semver guidance;
- demo domain listed in README/About.

---

## 7. v4 Milestones

| Milestone | Theme | Outcome |
|---|---|---|
| M0 | Foundation | PRD, task tracker, docs links, repo alignment |
| M1 | Admin UX scale | all tables server-side, grouped nav, cleaner dashboard |
| M2 | Service control plane | registry, health checks, services page |
| M3 | Self-management | typed settings, jobs, backup/export |
| M4 | API lifecycle | revisions, approval gates, diff, promotion bundle |
| M5 | Observability/compliance | audit/log filters, exports, retention |
| M6 | Release hardening | tests, docs, release v4 preview |

---

## 8. Success Metrics

- Admin pages can handle 100k+ rows through server-side pagination.
- Operator can identify unhealthy service in <30 seconds from dashboard.
- API publish path includes review/approval and audit trail.
- Metadata backup can be generated and dry-run restored.
- Fresh OSS user can understand architecture and run demo from docs.
- Gateway/connector hot path remains bounded under burst traffic.

---

## 9. Implementation Principles

- Security-first defaults.
- Backend is source of truth; UI never bypasses RBAC or policy.
- No unbounded lists.
- One service = one process with `/healthz`, `/readyz`, `/metrics`.
- Keep systemd/VPS deployment first-class.
- Add migrations incrementally and keep backward compatibility.
- Prefer small composable packages over large generic frameworks.
- Enterprise features must improve OSS, not hide behind proprietary locks.
