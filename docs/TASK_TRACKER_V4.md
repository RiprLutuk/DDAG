# DDAG v4 Task Tracker

**Source PRD:** [docs/PRD_V4.md](PRD_V4.md)  
**Execution mode:** bite-sized tasks, tests/build after each milestone, commit frequently.

Legend: `TODO`, `DOING`, `DONE`, `BLOCKED`

---

## M0 — Foundation

| ID | Status | Task | Files | Validation |
|---|---|---|---|---|
| V4-M0-01 | DONE | Add PRD v4 document | `docs/PRD_V4.md` | file exists |
| V4-M0-02 | DONE | Add v4 task tracker | `docs/TASK_TRACKER_V4.md` | file exists |
| V4-M0-03 | DONE | Link PRD/task tracker from README | `README.md` | link renders |
| V4-M0-04 | DONE | Add release checklist for v4 | `docs/RELEASE_V4.md` | docs build/read |

---

## M1 — Scalable Admin UX

| ID | Status | Task | Files | Validation |
|---|---|---|---|---|
| V4-M1-01 | DONE | Standardize list query parser for admin endpoints | `internal/adminsvc/helpers.go` | Go tests |
| V4-M1-02 | DONE | Add server-side list support for roles | `internal/store/identity.go`, `internal/adminsvc/*roles*`, `apps/dashboard/pages/roles.vue` | roles search/sort/page works |
| V4-M1-03 | DONE | Add server-side list support for scopes | `internal/store/clients.go`, `apps/dashboard/pages/scopes.vue` | scopes search/sort/page works |
| V4-M1-04 | DONE | Add server-side list support for rate limits | store/admin/dashboard files | rate limit table works |
| V4-M1-05 | DONE | Add server-side list support for IP whitelists | store/admin/dashboard files | whitelist table works |
| V4-M1-06 | DONE | Add server-side list support for cache rules | store/admin/dashboard files | cache table works |
| V4-M1-07 | DONE | Group sidebar navigation by Build/Access/Policy/Operations/System | `apps/dashboard/layouts/default.vue`, CSS | visual check |
| V4-M1-08 | DONE | Add shared empty/loading/error states for UiTable | `apps/dashboard/components/UiTable.vue` | visual check |

---

## M2 — Service Control Plane

| ID | Status | Task | Files | Validation |
|---|---|---|---|---|
| V4-M2-01 | DONE | Add `service_registry` migration | `migrations/*.sql` | migration applies |
| V4-M2-02 | DONE | Add service registry store methods | `internal/store/services.go` | Go tests |
| V4-M2-03 | DONE | Add admin service registry endpoints | `internal/adminsvc/handlers_services.go`, `routes.go` | API returns seeded services |
| V4-M2-04 | DONE | Add service health checker | `internal/workersvc` or `internal/adminsvc` | health sweep updates status |
| V4-M2-05 | DONE | Add dashboard Services page | `apps/dashboard/pages/services.vue` | live page shows services |
| V4-M2-06 | DONE | Add capability warnings to connections/settings | dashboard pages | warnings visible |

---

## M3 — Self-Management

| ID | Status | Task | Files | Validation |
|---|---|---|---|---|
| V4-M3-01 | DONE | Add typed settings schema metadata | migration + store | CRUD works |
| V4-M3-02 | DONE | Upgrade Settings page categories | `apps/dashboard/pages/settings.vue` | grouped UI |
| V4-M3-03 | DONE | Add maintenance jobs metadata | migration + admin runner | jobs list works |
| V4-M3-04 | DONE | Add Jobs dashboard page | `apps/dashboard/pages/jobs.vue` | trigger safe job |
| V4-M3-05 | DONE | Add metadata backup export endpoint | `internal/adminsvc/selfmanagement.go` | redacted export works |
| V4-M3-06 | DONE | Add Backups dashboard page | `apps/dashboard/pages/backups.vue` | download backup |

---

## M4 — Enterprise API Lifecycle

| ID | Status | Task | Files | Validation |
|---|---|---|---|---|
| V4-M4-01 | DONE | Add API revision tables | migrations | migration applies |
| V4-M4-02 | DONE | Store immutable published revision snapshots | store/admin gateway catalog | publish creates revision |
| V4-M4-03 | DONE | Enforce approved-before-publish gate | `internal/adminsvc/handlers_apis.go` | tests |
| V4-M4-04 | DONE | Add approval comments/audit metadata | migration + admin | audit row includes comment |
| V4-M4-05 | DONE | Add API diff endpoint | admin/store | diff JSON works |
| V4-M4-06 | DONE | Add API diff UI | `apps/dashboard/pages/apis.vue` or detail page | visual check |
| V4-M4-07 | DONE | Add export/import promotion bundle dry-run | admin/store/docs | dry-run detects issues |

---

## M5 — Observability & Compliance

| ID | Status | Task | Files | Validation |
|---|---|---|---|---|
| V4-M5-01 | DONE | Add audit advanced filters | store/admin/dashboard | filter by actor/action/entity/date |
| V4-M5-02 | DONE | Add audit CSV/JSON export | admin endpoint | file downloads |
| V4-M5-03 | DONE | Add request log advanced filters | store/admin/dashboard | filter by API/client/status/duration |
| V4-M5-04 | DONE | Add retention settings + cleanup job | settings/worker/store | cleanup job deletes old rows |
| V4-M5-05 | DONE | Add metrics catalog doc/page | docs/dashboard | page lists metrics |

---

## M6 — Release Hardening

| ID | Status | Task | Files | Validation |
|---|---|---|---|---|
| V4-M6-01 | TODO | Add/update tests for all new store queries | `internal/store/*_test.go` | `go test ./internal/store` |
| V4-M6-02 | TODO | Add admin handler tests | `internal/adminsvc/*_test.go` | `go test ./internal/adminsvc` |
| V4-M6-03 | TODO | Run full Go build | repo | `make build` or targeted builds |
| V4-M6-04 | DONE | Run dashboard production build | `apps/dashboard` | `pnpm install --frozen-lockfile && pnpm run build` |
| V4-M6-05 | TODO | Update README feature list | `README.md` | docs review |
| V4-M6-06 | TODO | Create release notes | GitHub release | release exists |
