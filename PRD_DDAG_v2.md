# PRD — DDAG: Dynamic Database API Gateway

**Document Type:** Product Requirements Document  
**Product Name:** DDAG — Dynamic Database API Gateway  
**Version:** 2.0 (Enterprise-Ready)  
**Status:** Updated & Hardened  
**Primary Goal:** Backend-as-a-Service / Auto-API Gateway untuk membuat API dinamis dari berbagai database dengan dashboard admin, OAuth2, RBAC, cache, monitoring, dan arsitektur scalable berbasis service/pod terpisah.

---

## 1. Executive Summary

DDAG adalah platform **Dynamic Database API Gateway** yang memungkinkan tim DBA, developer, dan aplikasi internal membuat API secara dinamis dari berbagai database tanpa harus membangun backend API manual untuk setiap kebutuhan data.

Masalah utama yang diselesaikan: **kebutuhan akses data antar aplikasi yang sering bergantung pada linked server, script custom, atau service backend repetitif.** DDAG menjadi layer perantara (*data plane*) yang aman, terkontrol, scalable, dan diaudit penuh.

Pada **Version 2.0** ini, fokus DDAG diperluas untuk menangani **high-concurrency traffic (ribuan RPS)**, mencegah database crash akibat cache stampede, dan menyediakan developer experience kelas dunia melalui auto-generated OpenAPI documentation.

### Positioning

> **"Turn approved SQL into production-ready REST APIs."**

DDAG bukan pengganti Kong (HTTP gateway) atau Hasura (GraphQL schema-exposed). DDAG fokus pada niche yang berbeda: **DBA/Platform Admin menulis dan memvalidasi query SQL, lalu DDAG mengeksposnya sebagai REST API yang aman, dilengkapi OAuth2, scope, rate limit, cache, audit, dan monitoring.**

Tagline:

- "Stop building tiny CRUD backends for every database query."
- "Database API gateway for regulated internal platforms."
- "Hasura exposes your schema. DDAG exposes approved SQL-backed REST APIs."

---

## 2. Background & Problem Statement

### 2.1 Kondisi Saat Ini

Banyak aplikasi membutuhkan akses data dari database lain seperti PostgreSQL, MySQL, Oracle, SQL Server, dan database lainnya. Pola yang sering terjadi:

- DBA harus membuat linked server atau koneksi antar database.
- Developer membuat backend kecil hanya untuk expose query tertentu.
- Akses database sering tidak punya governance yang kuat.
- Token, permission, dan scope user tidak standar.
- Monitoring API dan query tidak konsisten.
- Cache sering tidak tersedia atau dibuat manual per aplikasi.
- Audit siapa mengakses data apa sulit dilacak.
- Perubahan endpoint membutuhkan deploy aplikasi baru.

### 2.2 Masalah Utama

1. Terlalu banyak akses database langsung antar aplikasi.
2. Linked server sulit dikontrol, rawan dependency, dan sulit diskalakan.
3. Pembuatan API masih manual dan berulang.
4. Tidak ada dashboard tunggal untuk mengelola API database.
5. Permission per user/client belum jelas dan sering tersebar.
6. Monitoring API, latency, error, dan query tidak terpusat.
7. Cache tidak konsisten.
8. API sulit didistribusikan ke banyak user/client dengan aman.
9. Tidak ada standard OAuth2/token flow untuk aplikasi yang mengakses data.

### 2.3 Masalah High-Concurrency (NEW v2.0)

10. **Metadata DB bottleneck:** Setiap request ke gateway memicu query ke PostgreSQL metadata untuk lookup endpoint, scope, dan policy. Pada 1000+ RPS, metadata DB menjadi bottleneck kritis.
11. **Cache stampede:** Saat TTL cache Redis habis, ribuan request dalam milidetik yang sama menembus langsung ke source database, menyebabkan spike beban yang bisa crash.
12. **Connection pool exhaustion:** Source database yang lambat (lock contention, slow query) menyebabkan connection pool habis, goroutine menumpuk, dan service connector OOM (Out of Memory).
13. **Pagination semu:** Tanpa SQL-level pagination pushdown, connector membaca seluruh result set ke memori sebelum memotong array — berbahaya pada tabel besar (jutaan row).

---

## 3. Product Vision

DDAG menjadi **platform standar internal** untuk membuat, mengamankan, mengelola, dan memonitor API berbasis database secara dinamis.

Visi produk:

> "DBA dan admin cukup membuat API dari dashboard, developer cukup konsumsi endpoint dengan token, dan semua akses data tetap aman, termonitor, ter-cache, dan scalable — bahkan di ribuan request per detik."

---

## 4. Goals

### 4.1 Business Goals

1. Mengurangi kebutuhan linked server antar database.
2. Mempercepat pembuatan API data untuk aplikasi internal.
3. Membuat governance akses data lebih jelas.
4. Mengurangi pekerjaan backend repetitive untuk expose query sederhana.
5. Meningkatkan keamanan dan audit akses data.
6. Menyediakan monitoring terpusat untuk semua API dinamis.
7. Membuat distribusi API ke banyak aplikasi/user lebih teratur.
8. **Menjamin stabilitas platform pada traffic tinggi (1000+ RPS).** (NEW v2.0)

### 4.2 Product Goals

1. Admin bisa membuat endpoint API dinamis dari dashboard.
2. Semua konfigurasi API bisa dikelola tanpa code deployment.
3. Setiap aplikasi/client punya credential, token, scope, dan akses sendiri.
4. API mendukung OAuth2 bearer token dan refresh token.
5. API mendukung RBAC, IP whitelist, rate limiter, quota, dan audit log.
6. API bisa menggunakan cache per endpoint.
7. Semua endpoint punya metric Prometheus dan dashboard Grafana.
8. Connector database dibuat modular per database type.
9. Setiap service berjalan di pod/container sendiri.
10. Metadata utama menggunakan PostgreSQL.
11. **Gateway data plane tidak menyentuh metadata DB saat runtime (in-memory cache).** (NEW v2.0)
12. **Source database dilindungi circuit breaker dan singleflight.** (NEW v2.0)
13. **OpenAPI/Swagger documentation auto-generated dari metadata API.** (NEW v2.0)
14. **Pagination pushdown wajib di level SQL.** (NEW v2.0)

---

## 5. Non-Goals

DDAG tidak ditujukan untuk:

1. Menggantikan seluruh business backend aplikasi.
2. Menjadi full ETL/ELT platform seperti Airflow, NiFi, atau dbt.
3. Menjadi BI dashboard.
4. Menjadi database replication engine.
5. Memberikan akses raw SQL bebas ke user aplikasi.
6. Menjadi API gateway umum penuh seperti Kong/Traefik/Nginx API Gateway.
7. Menjadi data warehouse.
8. Menyimpan data bisnis utama milik aplikasi client.
9. Menjalankan query write/update/delete tanpa approval ketat pada fase awal.

---

## 6. Comparison (NEW v2.0)

### DDAG vs Kong

| Aspek | Kong | DDAG |
|---|---|---|
| Tipe | HTTP API Gateway (general) | Database API Gateway (specialized) |
| Upstream | HTTP services | Database engines |
| SQL awareness | Tidak | Ya (query template, binding, validation) |
| DB connection pool | Tidak | Ya (per-connection pool governance) |
| Use case | Route/aggregate HTTP APIs | Expose approved SQL as REST APIs |

**Hubungan:** Kong bisa dipasang di depan DDAG sebagai WAF/ingress. DDAG menyelesaikan problem yang berbeda.

### DDAG vs KrakenD

| Aspek | KrakenD | DDAG |
|---|---|---|
| Upstream | HTTP services | Database engines |
| Fokus | API composition/aggregation | SQL-backed endpoint generation |
| Use case | Combine multiple APIs | Expose database queries as APIs |

### DDAG vs Hasura

| Aspek | Hasura | DDAG |
|---|---|---|
| Protocol | GraphQL-first | REST-first |
| Approach | Auto-expose DB schema | Expose approved SQL templates |
| Multi-DB | Terutama PostgreSQL | PostgreSQL, MySQL, Oracle, SQL Server |
| Control | Schema-level permissions | Query-level curation by DBA |
| Audience | Developers | DBA / Platform / Internal teams |

**Diferensiasi:** "Hasura exposes your schema. DDAG exposes approved SQL-backed REST APIs."

---

## 7. Target Users & Personas

### 7.1 Super Admin

User dengan akses tertinggi ke seluruh sistem.

Tanggung jawab:

- Mengelola tenant/client.
- Mengelola user admin.
- Mengelola role dan permission global.
- Mengelola database connection.
- Mengelola semua endpoint API.
- Melihat audit log seluruh sistem.
- Melakukan disable endpoint/user/client bila ada issue.

### 7.2 Platform Admin

User operasional yang mengelola konfigurasi platform.

Tanggung jawab:

- Membuat dan maintain API.
- Mengatur cache rule.
- Mengatur rate limit.
- Mengatur IP whitelist.
- Melihat status connector dan health check.
- Melakukan troubleshooting endpoint.
- **Melihat circuit breaker state per connection.** (NEW v2.0)

### 7.3 DBA

User teknis yang mengelola koneksi database dan performa query.

Tanggung jawab:

- Membuat database connection.
- Test connection.
- Mengatur connection pool.
- Review query sebelum publish.
- Melihat slow query.
- Mengatur query timeout.
- Membantu tuning query/index di source database.
- **Melihat explain plan preview sebelum publish.** (NEW v2.0)

### 7.4 App Admin / Tenant Admin

Admin aplikasi yang mengonsumsi API.

Tanggung jawab:

- Melihat API yang diberikan ke aplikasinya.
- Melihat token/client miliknya.
- Melihat usage API miliknya.
- Melihat error log terbatas untuk aplikasinya.
- Request akses API baru.

### 7.5 Developer / API Consumer

User aplikasi yang menggunakan endpoint DDAG.

Tanggung jawab:

- Mengambil access token.
- Refresh token.
- Menggunakan API sesuai scope.
- **Melihat OpenAPI/Swagger documentation endpoint yang diizinkan.** (NEW v2.0)
- Tidak bisa melihat query asli atau secret database.

### 7.6 Viewer / Auditor

User read-only untuk audit dan compliance.

Tanggung jawab:

- Melihat konfigurasi tanpa mengubah.
- Melihat audit log.
- Melihat usage report.
- Melihat security event.

---

## 8. Scope

### 8.1 In Scope — MVP (v1.0)

Fitur yang wajib masuk MVP:

1. Login page dashboard admin.
2. User management.
3. Role-Based Access Control.
4. Client/application management.
5. OAuth2 token endpoint.
6. Refresh token endpoint.
7. Database connection management.
8. Dynamic API builder.
9. API publish/unpublish.
10. API permission per client/user.
11. IP whitelist per client/API.
12. Rate limiter per client/API.
13. Cache per endpoint.
14. PostgreSQL connector.
15. MySQL connector.
16. Oracle connector.
17. SQL Server connector.
18. Request/response logging.
19. Audit log.
20. Prometheus metrics.
21. Grafana dashboard template.
22. Health check per service.
23. Deployment berbasis container/pod per service.
24. Metadata DB menggunakan PostgreSQL.

### 8.2 In Scope — v2.0 (Enterprise Hardening) (NEW)

25. **In-memory metadata cache di gateway + Redis Pub/Sub sync.**
26. **Singleflight anti-cache stampede.**
27. **Circuit breaker per database connection (Sony/gobreaker).**
28. **SQL pagination pushdown (LIMIT/OFFSET, OFFSET/FETCH).**
29. **OpenAPI/Swagger auto-generator dari metadata API.**
30. **Service-to-service authentication (internal JWT/HMAC).**
31. **Fail-fast production config validation (refuse boot on default secrets).**
32. **Trusted proxy configuration untuk X-Forwarded-For.**
33. **Rate limiter fail-mode configurable (open/closed).**
34. **Helm chart dengan deployment profiles (dev/small-prod/enterprise).**

### 8.3 In Scope — Post v2.0

35. Approval workflow API (multi-step review).
36. Versioning API lanjutan.
37. SSO/OIDC integration.
38. MFA dashboard admin.
39. Advanced data masking (sensitive column detection).
40. Query cost estimator / explain plan preview.
41. Scheduler untuk cache warming.
42. Alertmanager integration.
43. Multi-tenant isolation lanjutan (tenant_id scoping).
44. GraphQL wrapper optional.
45. Support database tambahan (MongoDB, ClickHouse, etc.).
46. Write API dengan approval ketat.
47. Plugin/extension system (custom connector, custom policy, custom audit sink).
48. Vault/KMS integration untuk secret management.
49. Audit log export ke SIEM/Loki/OpenSearch.

---

## 9. High-Level Architecture

```text
                              ┌──────────────────┐
                              │  Client App      │
                              └────────┬─────────┘
                                       │ Bearer Token
                                       ▼
                              ┌──────────────────┐
                              │  API Gateway     │ ◄── In-memory metadata
                              │  (Singleflight)  │     (Redis Pub/Sub sync)
                              └────────┬─────────┘
                                       │
                    ┌──────────────────┬┴┬──────────────────┐
                    ▼                  ▼ ▼                  ▼
           ┌────────────┐    ┌─────────────┐     ┌──────────────┐
           │ Auth       │    │ Policy      │     │ Cache        │
           │ Service    │    │ Engine      │     │ Service      │
           │ (OAuth2)   │    │ (Scope/IP/  │     │ (Redis TTL)  │
           │            │    │  Rate/CB)   │     │              │
           └────────────┘    └─────────────┘     └──────┬───────┘
                                                        │
                              ┌──────────────────────────┼──────────────────────┐
                              ▼                          ▼                      ▼
                   ┌──────────────────┐      ┌──────────────────┐   ┌──────────────────┐
                   │ connector-       │      │ connector-       │   │ connector-       │
                   │ postgres         │      │ mysql            │   │ sqlserver        │
                   │ (Pool+CB+Limit)  │      │ (Pool+CB+Limit)  │   │ (Pool+CB+Limit)  │
                   └────────┬─────────┘      └────────┬─────────┘   └────────┬─────────┘
                            ▼                         ▼                      ▼
                     [ PostgreSQL ]             [ MySQL ]              [ SQL Server ]


                              ┌──────────────────┐
                              │  Admin Dashboard │ (Nuxt 3)
                              └────────┬─────────┘
                                       ▼
                              ┌──────────────────┐
                              │  Admin Backend   │ (CRUD, RBAC, Audit)
                              └────────┬─────────┘
                                       ▼
                              ┌──────────────────┐
                              │  Metadata DB     │ (PostgreSQL)
                              └──────────────────┘

                              ┌──────────────────┐
                              │  Monitoring      │ (Prometheus + Grafana)
                              └──────────────────┘
```

---

## 10. Service Architecture

Prinsip utama: **setiap service memiliki pod/container sendiri**. Tidak boleh semua fitur dibundle menjadi satu image besar.

### 10.1 Service List

| Service | Port | Tanggung Jawab | Deployment |
|---|---|---|---|
| admin-dashboard | 3000 | UI dashboard admin (Nuxt 3) | Separate pod |
| admin-backend | 8080 | CRUD konfigurasi DDAG, RBAC, audit | Separate pod |
| auth-service | 8081 | OAuth2, token, refresh, revoke, JWKS | Separate pod |
| api-gateway | 8082 | Entry point API dinamis, singleflight, circuit breaker | Separate pod, scalable |
| policy-engine | 8083 | RBAC, scope, whitelist, rate limit | Separate pod |
| cache-service | 8084 | Cache rule, purge, cache metadata | Separate pod |
| worker | 8085 | Background task, audit async, cache warming, token cleanup | Separate pod |
| connector-postgres | 8090 | Query ke PostgreSQL source | Separate pod, scalable |
| connector-mysql | 8091 | Query ke MySQL/MariaDB source | Separate pod, scalable |
| connector-oracle | 8092 | Query ke Oracle source | Separate pod, scalable |
| connector-sqlserver | 8093 | Query ke SQL Server source | Separate pod, scalable |

### 10.2 Scaling Strategy

Service yang harus mudah di-scale horizontal:

- api-gateway
- auth-service
- policy-engine
- connector-postgres
- connector-mysql
- connector-oracle
- connector-sqlserver
- cache-service

Service yang tidak perlu scale besar di awal:

- admin-dashboard
- admin-backend
- worker

---

## 11. Technology Stack

### 11.1 Backend

- **Go** untuk semua backend services.
- Binary statis, tanpa CGO, cocok untuk distroless container.
- Concurrency model (goroutine) ideal untuk high-throughput API gateway.

### 11.2 Metadata Database

- **PostgreSQL** sebagai default metadata DB DDAG.
- Menyimpan: user, role, client, token metadata, API definition, database connection metadata, policy, cache rule, rate limit rule, audit log, signing keys.

### 11.3 Cache & Messaging

- **Redis** untuk cache, rate limiting, dan Pub/Sub metadata sync.
- Alternatif: KeyDB, DragonflyDB.

### 11.4 Monitoring

- **Prometheus** untuk metrics scraping.
- **Grafana** untuk dashboarding.
- Alertmanager (optional) untuk alerting.
- Loki/OpenSearch (optional) untuk log aggregation.

### 11.5 Frontend Dashboard

- **Nuxt 3** (Vue 3) — server-side rendering, clean admin flow.
- Requirement: simple, fast, clean admin flow, mudah dipakai DBA/admin.

### 11.6 Deployment

- Docker/Podman image (distroless).
- Kubernetes/OpenShift compatible.
- **Helm chart dengan deployment profiles.** (NEW v2.0)
- ConfigMap + Secret.
- Horizontal Pod Autoscaler untuk service high traffic.

---

## 12. Functional Requirements

### 12.1 Login Page

Dashboard wajib memiliki halaman login default.

Requirement:

- Login dengan username/email dan password.
- Password disimpan dengan hashing aman (bcrypt).
- Session timeout.
- Account lock setelah gagal login berulang.
- Audit login success/failure.
- Logout.
- Reset password (post-MVP).
- MFA (post-MVP).

### 12.2 User Management

Admin dapat mengelola user dashboard.

Field minimal:

- ID, Name, Email, Username, Password hash, Role, Tenant/application owner, Status active/inactive, Last login, Created by, Created at, Updated at.

Fitur: Create, Update, Disable, Reset password, Assign role, Assign tenant, View activity.

### 12.3 Role-Based Access Control

Role minimal:

```text
SUPER_ADMIN
PLATFORM_ADMIN
DBA
APP_ADMIN
DEVELOPER
VIEWER
AUDITOR
```

Permission yang perlu dikontrol:

- View dashboard, Manage user, Manage role, Manage client, Manage database connection, View database secret, Test database connection, Create API, Edit API, Approve API, Publish API, Disable API, Purge cache, View audit log, View monitoring, Manage rate limit, Manage IP whitelist.
- **View circuit breaker state.** (NEW v2.0)
- **View explain plan.** (NEW v2.0)
- **Approve write API (multi-step).** (Post v2.0)

### 12.4 Client/Application Management

Setiap aplikasi yang mengakses DDAG wajib punya client sendiri.

Field minimal:

- Client ID, Client name, Client secret (encrypted), Owner, Environment (dev/staging/prod), Status, Allowed scopes, Allowed APIs, IP whitelist, Rate limit profile, Token lifetime, Refresh token lifetime, Created by, Created at, Updated at.

Fitur: Create, Generate/Rotate secret, Disable, Assign API access, Assign scope, Configure IP whitelist, Configure rate limit, View usage.

### 12.5 OAuth2 Token Management

Endpoint minimal:

```http
POST /oauth/token
POST /oauth/refresh
POST /oauth/revoke
POST /oauth/introspect
GET  /.well-known/jwks.json
```

Supported grant untuk MVP:

- client_credentials
- refresh_token

Token requirement:

- Access token berbentuk JWT (RS256).
- Token memiliki expiry.
- Refresh token disimpan hashed, mendukung rotation.
- Token membawa scope dan client_id.
- Signing key dapat dirotasi (kid di JWKS).
- Token failure tercatat.
- **JWT harus memiliki audience (aud) claim.** (NEW v2.0)
- **Issuer validation eksplisit di gateway.** (NEW v2.0)
- **Clock skew leeway configurable.** (NEW v2.0)

### 12.6 Database Connection Management

Supported database MVP:

1. PostgreSQL
2. MySQL/MariaDB
3. Oracle
4. SQL Server

Field umum:

- Connection name, Database type, Host, Port, Database name/service name/SID, Schema, Username, Password/secret reference, SSL/TLS mode, Min pool size, Max pool size, Connection timeout, Query timeout, Max conn lifetime, Max conn idle, Environment, Status, Tags, Config version.

Security requirement:

- Password tidak boleh disimpan plain text (envelope encryption).
- Secret harus encrypted atau disimpan di secret manager.
- Connection string tidak boleh muncul di log.
- Hanya role tertentu yang bisa melihat/mengubah koneksi.

### 12.7 Dynamic API Builder

Admin dapat membuat endpoint API secara dinamis dari dashboard.

Field API definition:

- API ID, API name, Namespace, Path, HTTP method (GET/POST), Description, Database connection, Connector type, Query template, Parameter definition, Response mapping, Cache rule, Rate limit rule, Required scope, IP whitelist rule, Status (DRAFT/REVIEW/PUBLISHED/DISABLED/ARCHIVED), Version, Default limit, Max limit, Is write flag, Created by, Approved by, Published at.

Requirement penting:

- Query harus menggunakan parameter binding.
- Tidak boleh string concatenation langsung dari user input.
- Query harus divalidasi sebelum publish (ValidateForPublish).
- Query timeout wajib ada.
- Limit row wajib ada untuk endpoint list/search.
- **Pagination pushdown wajib (SQL-level LIMIT/OFFSET).** (NEW v2.0)
- Query write harus disabled di MVP kecuali secara eksplisit diaktifkan melalui approval.
- **Multi-statement query ditolak.** (EXISTING)

### 12.8 API Permission & Scope

Setiap API wajib memiliki scope.

Contoh scope:

```text
brim.site.read
brim.wo.read
customer.profile.read
inventory.stock.read
```

Rule:

- Client hanya bisa akses API jika memiliki scope yang sesuai DAN memiliki client_api_access grant.
- Scope dapat diberikan per client.
- Scope dapat diberikan per environment.
- Scope dapat direvoke.
- Scope change tercatat di audit log.

### 12.9 IP Whitelist

Level whitelist: Global, Per client, Per API.

Format: Single IP, CIDR.

**Trusted Proxy Requirement (NEW v2.0):**

- Gateway harus memiliki config `DDAG_TRUSTED_PROXIES` (e.g. `10.0.0.0/8,172.16.0.0/12`).
- Header `X-Forwarded-For` hanya dipercaya jika request berasal dari trusted proxy.
- Jika request bukan dari trusted proxy, gunakan `RemoteAddr`.

### 12.10 Rate Limiter & Quota

Level rate limit: Per client, Per API, Per IP, Global fallback.

Metric rate limit: Request per second, per minute, per hour, per day.

**Fail Mode (NEW v2.0):**

- Configurable via `DDAG_RATE_LIMIT_FAIL_MODE=open|closed`.
- `open` (default): Jika Redis down, request tetap dilanjutkan (availability priority).
- `closed`: Jika Redis down, request ditolak 503 (security priority).
- Dapat di-override per client atau per API.

### 12.11 Cache Management

Fitur:

- Enable/disable cache per API.
- TTL per API.
- Cache key strategy (path, query params, body hash, client_id).
- Manual purge cache.
- Cache bypass untuk admin/debug.
- Cache hit/miss metric.

**Anti-Stampede (NEW v2.0):**

- Implementasi Singleflight per cache key.
- Pada cache miss, hanya 1 goroutine yang query source DB.
- Goroutine lain menunggu hasil goroutine pertama.
- Metric: `ddag_singleflight_active`, `ddag_singleflight_shared`.

### 12.12 Database Connector Services

Setiap database type memiliki connector service terpisah.

Service MVP:

```text
ddag-connector-postgres
ddag-connector-mysql
ddag-connector-oracle
ddag-connector-sqlserver
```

Tanggung jawab connector:

- Menerima request query internal dari API Gateway.
- Resolve connection secret secara aman.
- Menggunakan connection pool (per connection_id + config_version).
- Menjalankan prepared statement dengan parameter binding.
- Timeout query.
- **Retry terbatas untuk transient error (1x, read-only saja).** (NEW v2.0)
- **Circuit breaker per connection_id.** (NEW v2.0)
- Normalize error (tidak expose raw DB error ke client).
- Return result dalam format standar.
- Mencatat query duration.

**Circuit Breaker Detail (NEW v2.0):**

- State: Closed → Open → Half-Open.
- Trigger: error ratio > threshold ATAU timeout ratio > threshold dalam window tertentu.
- Saat Open: langsung return HTTP 503 tanpa mencoba koneksi.
- Saat Half-Open: izinkan 1 request probe, jika sukses → Closed.
- Metric: `ddag_circuit_state{connection_id}`, `ddag_circuit_open_total`, `ddag_circuit_half_open_total`.
- Dashboard: Admin dapat melihat circuit state per connection dari UI.

**Service-to-Service Auth (NEW v2.0):**

- Connector endpoint `/query` dan `/test` dilindungi internal JWT/HMAC.
- Gateway menandatangani request internal sebelum dikirim ke connector.
- Connector memvalidasi signature sebelum mengeksekusi query.
- Tujuan: mencegah lateral movement jika pod lain di cluster compromised.

**Pagination Pushdown (NEW v2.0):**

- Connector wajib mendukung pagination pushdown.
- Gateway menyertakan parameter `ddag_limit` dan `ddag_offset` di request internal.
- Connector menginjeksi syntax pagination sesuai dialek:

| Database | Syntax |
|---|---|
| PostgreSQL | `LIMIT $n OFFSET $m` |
| MySQL | `LIMIT ?, ?` |
| SQL Server | `OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY` |
| Oracle | `OFFSET :1 ROWS FETCH NEXT :2 ROWS ONLY` |

### 12.13 Response Format Standard

Success response:

```json
{
  "success": true,
  "request_id": "req-xxxxx",
  "data": {},
  "meta": {
    "cached": false,
    "cache_hit": false,
    "duration_ms": 120,
    "circuit_state": "closed"
  }
}
```

List response:

```json
{
  "success": true,
  "request_id": "req-xxxxx",
  "data": [],
  "pagination": {
    "page": 1,
    "limit": 50,
    "offset": 0,
    "total": 1250,
    "has_next": true
  },
  "meta": {
    "cached": true,
    "cache_hit": true,
    "duration_ms": 3,
    "circuit_state": "closed"
  }
}
```

Error response:

```json
{
  "success": false,
  "request_id": "req-xxxxx",
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests"
  }
}
```

Circuit breaker error response:

```json
{
  "success": false,
  "request_id": "req-xxxxx",
  "error": {
    "code": "SERVICE_UNAVAILABLE",
    "message": "Database connection temporarily unavailable (circuit open)"
  }
}
```

### 12.14 OpenAPI / Swagger Documentation (NEW v2.0)

DDAG wajib auto-generate dokumentasi API dari metadata.

Endpoint:

```http
GET /openapi.json          — Full OpenAPI 3.0 spec
GET /docs                  — Swagger UI
GET /api-catalog           — API listing per scope/client
```

Isi OpenAPI spec per API definition:

- Path + method.
- Parameters (path, query, body) — dari `api_parameters` table.
- Required scope.
- Security scheme (OAuth2 Bearer).
- Response schema (dari response_mapping jika ada, atau generic).
- Description.
- Tags (namespace).

Fitur tambahan:

- Developer/API Consumer hanya melihat API yang scope-nya diizinkan.
- Admin melihat semua API.
- Download as `.json` atau `.yaml`.

### 12.15 Monitoring & Observability

Setiap service expose:

```http
GET /metrics     — Prometheus format
GET /healthz     — Liveness probe
GET /readyz      — Readiness probe
```

Metric wajib:

| Metric | Deskripsi |
|---|---|
| `ddag_request_total` | Total request per endpoint |
| `ddag_request_duration_seconds` | Latency histogram |
| `ddag_request_errors_total` | Error count per endpoint |
| `ddag_cache_hit_total` | Cache hit count |
| `ddag_cache_miss_total` | Cache miss count |
| `ddag_pool_in_use_connections` | Active connections per pool |
| `ddag_pool_idle_connections` | Idle connections per pool |
| `ddag_pool_max_connections` | Max connections per pool |
| `ddag_rate_limit_exceeded_total` | Rate limit hit count |
| `ddag_auth_token_issued_total` | Token issued count |
| `ddag_circuit_state` | Circuit breaker state per connection (NEW v2.0) |
| `ddag_circuit_open_total` | Circuit open events (NEW v2.0) |
| `ddag_singleflight_active` | Active singleflight requests (NEW v2.0) |
| `ddag_singleflight_shared` | Shared singleflight results (NEW v2.0) |
| `ddag_metadata_sync_total` | Metadata sync events via Pub/Sub (NEW v2.0) |

### 12.16 Audit Log

Audit log wajib append-only (enforced oleh database trigger).

Event yang dicatat:

- Login success/failure.
- User CRUD.
- Role change.
- Client CRUD.
- Secret rotation.
- API CRUD.
- API publish/unpublish.
- Connection CRUD.
- Cache purge.
- Rate limit change.
- IP whitelist change.
- Token revoke.
- **Circuit breaker state change.** (NEW v2.0)
- Security events (blocked IP, invalid token, scope violation).

---

## 13. High-Traffic Architecture Detail (NEW v2.0)

### 13.1 Request Hot Path (Zero Metadata DB Hit)

```text
Request masuk
    │
    ▼
Gateway: lookup route dari IN-MEMORY map
    │
    ▼
Gateway: validate JWT (public key dari in-memory JWKS)
    │
    ▼
Gateway: check scope + client access (in-memory policy)
    │
    ▼
Gateway: check IP whitelist (in-memory rules)
    │
    ▼
Gateway: check rate limit (Redis INCR — 1 network hop)
    │
    ▼
Gateway: check cache (Redis GET — 1 network hop)
    │
    ├── Cache HIT → return cached response (0 DB hit)
    │
    └── Cache MISS → Singleflight
            │
            ▼
        Gateway: check circuit breaker state (in-memory)
            │
            ├── Circuit OPEN → return 503 immediately
            │
            └── Circuit CLOSED → call connector
                    │
                    ▼
                Connector: execute query (pool + timeout + pushdown limit)
                    │
                    ▼
                Gateway: store result in Redis cache
                    │
                    ▼
                Return response
```

**Total DB metadata hits pada hot path: 0**
**Total Redis hits: 2 (rate limit + cache)**
**Total source DB hits: 0 (cache hit) atau 1 (cache miss, deduplicated by singleflight)**

### 13.2 Metadata Sync Flow

```text
Admin mengubah API definition di dashboard
    │
    ▼
Admin Backend: update PostgreSQL metadata
    │
    ▼
Admin Backend: publish event ke Redis Pub/Sub channel "ddag:metadata:sync"
    │
    ▼
Semua gateway instances: receive event
    │
    ▼
Gateway: reload metadata dari PostgreSQL (background, non-blocking)
    │
    ▼
Gateway: update in-memory maps
```

Sync interval fallback: jika Pub/Sub miss, gateway melakukan periodic full reload setiap 60 detik.

### 13.3 Circuit Breaker State Machine

```text
     ┌──────────┐
     │  CLOSED  │ ◄── Normal operation
     └────┬─────┘
          │ error ratio > threshold
          ▼
     ┌──────────┐
     │   OPEN   │ ◄── Reject all, return 503
     └────┬─────┘
          │ after timeout window
          ▼
     ┌──────────┐
     │HALF-OPEN │ ◄── Allow 1 probe request
     └────┬─────┘
          │
     ┌────┴────┐
     │         │
   success   failure
     │         │
     ▼         ▼
  CLOSED     OPEN
```

Configurable per connection:

```env
DDAG_CB_MAX_REQUESTS=1           # max requests in half-open
DDAG_CB_INTERVAL=60              # failure counting window (seconds)
DDAG_CB_TIMEOUT=30               # time in open state before half-open (seconds)
DDAG_CB_FAILURE_THRESHOLD=5      # failures to trip
DDAG_CB_FAILURE_RATIO=0.6        # failure ratio to trip
```

---

## 14. Security Requirements

### 14.1 Authentication & Authorization

- OAuth2 client_credentials + refresh_token.
- JWT RS256 dengan rotatable signing key dan JWKS.
- Dashboard: httpOnly session cookie + lockout.
- RBAC enforced server-side (bukan hanya UI).
- Scope + client API access grant + IP whitelist di data plane.

### 14.2 Query Safety

- Parameter binding only (named params rewritten per driver).
- Multi-statement ditolak.
- Write statement ditolak kecuali is_write=true.
- Undeclared/unused parameter ditolak.
- Default limit wajib > 0.

### 14.3 Secret Management

- Envelope encryption (AES-GCM) untuk DB password.
- Per-secret DEK, wrapped by master key.
- Structured log redaction untuk: password, client_secret, authorization, access_token, refresh_token, dsn, private_key, master_key.

### 14.4 Production Hardening (NEW v2.0)

Jika `DDAG_ENV=prod`, service WAJIB menolak boot jika:

| Condition | Action |
|---|---|
| Master key masih default (`AAAA...`) | Fatal error, refuse boot |
| Session secret masih `dev-insecure-session-secret-change-me` | Fatal error, refuse boot |
| Superadmin password masih `Admin#12345` | Fatal error, refuse boot |
| Cookie secure = false | Fatal error, refuse boot |
| Dashboard origins = wildcard/localhost | Warning log |
| DB sslmode = disable (metadata DB) | Warning log |

### 14.5 Network Security (NEW v2.0)

- Trusted proxy configuration wajib.
- Service-to-service auth (internal JWT/HMAC) untuk connector endpoints.
- Connector tidak boleh diexpose ke public internet.
- **CSRF protection** untuk dashboard cookie session (SameSite + optional CSRF token).

---

## 15. Deployment Profiles (NEW v2.0)

### 15.1 Dev Profile

Untuk development lokal / single node.

```text
- 1x admin-backend
- 1x auth-service
- 1x api-gateway
- 1x connector-postgres (atau sesuai kebutuhan)
- Redis (single instance)
- PostgreSQL metadata (single instance)
```

### 15.2 Small Production

Untuk deployment production kecil (1 server / small cluster).

```text
- 1x admin-backend
- 1x auth-service
- 2x api-gateway (load balanced)
- 1-2x connector per DB type
- Redis (single / sentinel)
- PostgreSQL metadata (single + backup)
```

### 15.3 Enterprise HA

Untuk deployment production enterprise (Kubernetes cluster).

```text
- 2x admin-backend (active-passive)
- 2x auth-service (active-active)
- 3-5x api-gateway (HPA)
- 2-3x connector per DB type (HPA)
- Redis Cluster (3+ nodes)
- PostgreSQL metadata (primary + read replica)
- Prometheus + Grafana + Alertmanager
- Ingress + cert-manager + external-secrets
```

### 15.4 Helm Chart Structure

```text
deploy/helm/ddag/
├── Chart.yaml
├── values.yaml
├── values-dev.yaml
├── values-production.yaml
├── values-enterprise.yaml
└── templates/
    ├── deployment-admin.yaml
    ├── deployment-auth.yaml
    ├── deployment-gateway.yaml
    ├── deployment-connector-postgres.yaml
    ├── deployment-connector-mysql.yaml
    ├── deployment-connector-sqlserver.yaml
    ├── service.yaml
    ├── configmap.yaml
    ├── secret.yaml
    ├── hpa.yaml
    ├── ingress.yaml
    ├── servicemonitor.yaml
    └── pdb.yaml
```

---

## 16. Testing Requirements (NEW v2.0)

### 16.1 Unit Tests

- Binder (SQL parameter binding per dialect).
- Validator (ValidateForPublish).
- Router (path matching).
- Policy (scope check, IP whitelist).
- Circuit breaker state transitions.
- Singleflight behavior.

### 16.2 Integration Tests

- Auth token lifecycle (issue → use → refresh → revoke).
- Gateway full request flow (token → policy → cache → connector → response).
- Connector PostgreSQL integration (with testcontainers).
- Connector MySQL integration.
- Redis rate limit integration.
- Redis cache + singleflight integration.
- Circuit breaker trip and recovery.
- Metadata Pub/Sub sync.

### 16.3 CI Pipeline

```text
gofmt → go vet → golangci-lint → gosec → govulncheck
    → go test (unit)
    → go test (integration with service containers)
    → npm audit
    → npm run build (dashboard)
    → docker build
    → trivy scan
```

---

## 17. Roadmap Summary

### Phase 1 — MVP (v1.0) ✅ DONE

- Dashboard, API builder, OAuth2, RBAC, connectors, cache, rate limit, audit, monitoring.

### Phase 2 — Enterprise Hardening (v2.0) ← CURRENT

1. In-memory metadata cache + Redis Pub/Sub sync.
2. Singleflight anti-cache stampede.
3. Circuit breaker per DB connection.
4. SQL pagination pushdown.
5. OpenAPI/Swagger auto-generator.
6. Service-to-service authentication.
7. Production config validation (fail-fast).
8. Trusted proxy configuration.
9. Rate limit fail-mode configurable.
10. Helm chart dengan deployment profiles.

### Phase 3 — Global Ecosystem

11. SSO/OIDC login.
12. MFA dashboard.
13. Vault/KMS integration.
14. Query approval workflow (multi-step review).
15. Data masking (sensitive column detection).
16. Query explain plan preview.
17. Audit export (SIEM/Loki/OpenSearch).
18. Tenant isolation (multi-tenant scoping).
19. Plugin/extension system.
20. GraphQL wrapper optional.
21. Additional database support (MongoDB, ClickHouse).

---

## 18. Success Metrics

| Metric | Target |
|---|---|
| API creation time (no deployment) | < 5 menit |
| Gateway P99 latency (cached) | < 10ms |
| Gateway P99 latency (uncached) | < 200ms |
| Throughput (single gateway pod) | > 2000 RPS |
| Cache hit ratio (steady state) | > 80% |
| Circuit breaker recovery time | < 60 detik |
| Singleflight dedup ratio (stampede) | > 95% |
| Zero metadata DB hit on hot path | 100% |
| Uptime SLA | 99.9% |

---

*Document authored by Ripr Lutuk.*  
*Version 2.0 — Updated June 2026.*
