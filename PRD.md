# PRD — DDAG: Dynamic Database API Gateway

**Document Type:** Product Requirements Document  
**Product Name:** DDAG — Dynamic Database API Gateway  
**Version:** 1.0  
**Status:** Draft  
**Primary Goal:** Backend-as-a-Service / Auto-API Gateway untuk membuat API dinamis dari berbagai database dengan dashboard admin, OAuth2, RBAC, cache, monitoring, dan arsitektur scalable berbasis service/pod terpisah.

---

## 1. Executive Summary

DDAG adalah platform **Dynamic Database API Gateway** yang memungkinkan tim DBA, developer, dan aplikasi internal membuat API secara dinamis dari berbagai database tanpa harus membangun backend API manual untuk setiap kebutuhan data.

Masalah utama yang ingin diselesaikan adalah kebutuhan akses data antar aplikasi yang sering bergantung pada **linked server**, koneksi langsung antar database, script custom, atau service backend kecil yang sulit dikelola. DDAG menjadi layer tengah yang aman, terkontrol, scalable, dan mudah dikelola melalui dashboard admin.

Dengan DDAG, admin dapat membuat endpoint API, menentukan koneksi database, query, parameter, permission user, token, cache, rate limit, IP whitelist, dan monitoring dalam satu platform.

Target awal DDAG adalah membuat platform yang **simple dari sisi flow admin**, tetapi tetap **powerful dan high impact** untuk kebutuhan enterprise internal.

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

DDAG dibuat untuk menjawab masalah berikut:

1. Terlalu banyak akses database langsung antar aplikasi.
2. Linked server sulit dikontrol, rawan dependency, dan sulit diskalakan.
3. Pembuatan API masih manual dan berulang.
4. Tidak ada dashboard tunggal untuk mengelola API database.
5. Permission per user/client belum jelas dan sering tersebar.
6. Monitoring API, latency, error, dan query tidak terpusat.
7. Cache tidak konsisten.
8. API sulit didistribusikan ke banyak user/client dengan aman.
9. Tidak ada standard OAuth2/token flow untuk aplikasi yang mengakses data.

---

## 3. Product Vision

DDAG menjadi **platform standar internal** untuk membuat, mengamankan, mengelola, dan memonitor API berbasis database secara dinamis.

Visi produk:

> “DBA dan admin cukup membuat API dari dashboard, developer cukup konsumsi endpoint dengan token, dan semua akses data tetap aman, termonitor, ter-cache, dan scalable.”

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
10. Metadata utama menggunakan PostgreSQL 18.

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

## 6. Target Users & Personas

### 6.1 Super Admin

User dengan akses tertinggi ke seluruh sistem.

Tanggung jawab:

- Mengelola tenant/client.
- Mengelola user admin.
- Mengelola role dan permission global.
- Mengelola database connection.
- Mengelola semua endpoint API.
- Melihat audit log seluruh sistem.
- Melakukan disable endpoint/user/client bila ada issue.

### 6.2 Platform Admin

User operasional yang mengelola konfigurasi platform.

Tanggung jawab:

- Membuat dan maintain API.
- Mengatur cache rule.
- Mengatur rate limit.
- Mengatur IP whitelist.
- Melihat status connector dan health check.
- Melakukan troubleshooting endpoint.

### 6.3 DBA

User teknis yang mengelola koneksi database dan performa query.

Tanggung jawab:

- Membuat database connection.
- Test connection.
- Mengatur connection pool.
- Review query sebelum publish.
- Melihat slow query.
- Mengatur query timeout.
- Membantu tuning query/index di source database.

### 6.4 App Admin / Tenant Admin

Admin aplikasi yang mengonsumsi API.

Tanggung jawab:

- Melihat API yang diberikan ke aplikasinya.
- Melihat token/client miliknya.
- Melihat usage API miliknya.
- Melihat error log terbatas untuk aplikasinya.
- Request akses API baru.

### 6.5 Developer / API Consumer

User aplikasi yang menggunakan endpoint DDAG.

Tanggung jawab:

- Mengambil access token.
- Refresh token.
- Menggunakan API sesuai scope.
- Melihat dokumentasi endpoint yang diizinkan.
- Tidak bisa melihat query asli atau secret database.

### 6.6 Viewer / Auditor

User read-only untuk audit dan compliance.

Tanggung jawab:

- Melihat konfigurasi tanpa mengubah.
- Melihat audit log.
- Melihat usage report.
- Melihat security event.

---

## 7. Scope

### 7.1 In Scope — MVP

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
24. Metadata DB menggunakan PostgreSQL 18.

### 7.2 In Scope — Post-MVP

1. Approval workflow API.
2. Versioning API lanjutan.
3. API documentation generator.
4. Mock response.
5. SSO/OIDC integration.
6. MFA dashboard admin.
7. Advanced data masking.
8. Query cost estimator.
9. Scheduler untuk cache warming.
10. Alertmanager integration.
11. Multi-tenant isolation lanjutan.
12. GraphQL wrapper optional.
13. Support database tambahan.
14. Write API dengan approval ketat.

---

## 8. High-Level Architecture

```text
+-----------------------+
| Client Application    |
+-----------+-----------+
            |
            | Bearer Token
            v
+-----------------------+
| API Gateway Service   |
| Dynamic Routing       |
+-----------+-----------+
            |
            v
+-----------------------+
| Auth Service          |
| OAuth2 / JWT          |
+-----------------------+
            |
            v
+-----------------------+
| Policy Engine Service |
| RBAC / Scope / IP     |
| Rate Limit / Quota    |
+-----------+-----------+
            |
            v
+-----------------------+
| Cache Service         |
| Redis / KeyDB         |
+-----------+-----------+
            |
            v
+-------------------------------+
| DB Connector Services         |
| PostgreSQL / MySQL / Oracle   |
| SQL Server / Others           |
+---------------+---------------+
                |
                v
+-------------------------------+
| External Source Databases     |
+-------------------------------+


+-----------------------+
| Admin Dashboard       |
+-----------+-----------+
            |
            v
+-----------------------+
| Admin Backend Service |
+-----------+-----------+
            |
            v
+-----------------------+
| Metadata DB           |
| PostgreSQL 18         |
+-----------------------+


+-----------------------+
| Monitoring Stack      |
| Prometheus / Grafana  |
+-----------------------+
```

---

## 9. Service Architecture

Prinsip utama: **setiap service memiliki pod/container sendiri**. Tidak boleh semua fitur dibundle menjadi satu image besar.

### 9.1 Service List

| Service | Tanggung Jawab | Deployment |
|---|---|---|
| admin-dashboard | UI dashboard admin | Separate pod |
| admin-backend | CRUD konfigurasi DDAG | Separate pod |
| auth-service | OAuth2, token, refresh token | Separate pod |
| api-gateway | Entry point API dinamis | Separate pod, scalable |
| policy-engine | RBAC, scope, whitelist, rate limit | Separate pod |
| cache-service | Cache rule, purge, cache metadata | Separate pod |
| connector-postgres | Query ke PostgreSQL source | Separate pod, scalable |
| connector-mysql | Query ke MySQL/MariaDB source | Separate pod, scalable |
| connector-oracle | Query ke Oracle source | Separate pod, scalable |
| connector-sqlserver | Query ke SQL Server source | Separate pod, scalable |
| worker-service | Background task, audit async, cache warming | Separate pod |
| monitoring-exporter | Custom metrics exporter bila dibutuhkan | Separate pod |

### 9.2 Scaling Strategy

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
- worker-service

---

## 10. Recommended Technology Stack

### 10.1 Backend

Primary backend:

- Go untuk API Gateway, Auth, Policy Engine, Admin Backend, dan Connector Service.
- Target runtime mengikuti versi stabil yang disetujui di environment deployment.
- Catatan requirement awal menyebut Go 2.26; versi final harus divalidasi saat implementasi agar sesuai release resmi dan compatibility library.

Alasan Go:

- Ringan.
- Concurrency bagus.
- Cocok untuk high-throughput API.
- Deployment binary sederhana.
- Cocok untuk microservice/pod kecil.

### 10.2 Metadata Database

- PostgreSQL 18 sebagai default metadata DB DDAG.

Digunakan untuk menyimpan:

- User.
- Role.
- Client.
- Token metadata.
- API definition.
- Database connection metadata.
- Policy.
- Cache rule.
- Rate limit rule.
- Audit log metadata.
- API version.

### 10.3 Cache

Pilihan utama:

- Redis.

Alternatif:

- KeyDB.
- DragonflyDB.

### 10.4 Monitoring

- Prometheus.
- Grafana.
- Alertmanager optional.
- Loki/OpenSearch optional untuk log aggregation.

### 10.5 Frontend Dashboard

Pilihan:

- Next.js.
- Vue.
- React.

Requirement frontend:

- Simple.
- Fast.
- Clean admin flow.
- Tidak overcomplicated.
- Mudah dipakai DBA/admin.

### 10.6 Deployment

- Docker/Podman image.
- Kubernetes/OpenShift compatible.
- Helm chart optional.
- ConfigMap + Secret.
- Horizontal Pod Autoscaler untuk service high traffic.

---

## 11. Functional Requirements

## 11.1 Login Page

Dashboard wajib memiliki halaman login default.

Requirement:

- Login dengan username/email dan password.
- Password disimpan dengan hashing aman.
- Session timeout.
- Account lock setelah gagal login berulang.
- Audit login success/failure.
- Logout.
- Reset password optional untuk fase berikutnya.
- MFA optional untuk fase berikutnya.

Acceptance Criteria:

- User valid bisa login.
- User invalid ditolak.
- Failed login tercatat di audit log.
- Session expired otomatis setelah durasi tertentu.

---

## 11.2 User Management

Admin dapat mengelola user dashboard.

Field minimal:

- ID.
- Name.
- Email.
- Username.
- Password hash.
- Role.
- Tenant/application owner.
- Status active/inactive.
- Last login.
- Created by.
- Created at.
- Updated at.

Fitur:

- Create user.
- Update user.
- Disable user.
- Reset password.
- Assign role.
- Assign tenant.
- View activity.

Acceptance Criteria:

- Super admin bisa membuat user baru.
- User inactive tidak bisa login.
- Perubahan role langsung memengaruhi akses dashboard.
- Semua perubahan user tercatat di audit log.

---

## 11.3 Role-Based Access Control

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

- View dashboard.
- Manage user.
- Manage role.
- Manage client.
- Manage database connection.
- View database secret.
- Test database connection.
- Create API.
- Edit API.
- Approve API.
- Publish API.
- Disable API.
- Purge cache.
- View audit log.
- View monitoring.
- Manage rate limit.
- Manage IP whitelist.

Acceptance Criteria:

- User hanya bisa mengakses menu sesuai role.
- API admin backend juga memvalidasi permission, tidak hanya frontend.
- Viewer tidak bisa melakukan perubahan data.
- DBA bisa test connection tetapi tidak otomatis bisa manage user.

---

## 11.4 Client/Application Management

Setiap aplikasi yang mengakses DDAG wajib punya client sendiri.

Field minimal:

- Client ID.
- Client name.
- Client secret hash/encrypted reference.
- Owner.
- Environment: dev/staging/prod.
- Status active/inactive.
- Allowed scopes.
- Allowed APIs.
- IP whitelist.
- Rate limit profile.
- Token lifetime.
- Refresh token lifetime.
- Created by.
- Created at.
- Updated at.

Fitur:

- Create client.
- Generate client secret.
- Rotate client secret.
- Disable client.
- Assign API access.
- Assign scope.
- Configure IP whitelist.
- Configure rate limit.
- View usage.

Acceptance Criteria:

- Client inactive tidak bisa mengambil token.
- Client hanya bisa akses API sesuai permission.
- Secret hanya tampil satu kali saat generate atau rotate.
- Rotasi secret tercatat di audit log.

---

## 11.5 OAuth2 Token Management

DDAG wajib menyediakan endpoint untuk mendapatkan token dan refresh token.

Endpoint minimal:

```http
POST /oauth/token
POST /oauth/refresh
POST /oauth/revoke
POST /oauth/introspect
```

Supported grant untuk MVP:

- client_credentials.
- refresh_token.

Contoh request token:

```http
POST /oauth/token
Content-Type: application/json

{
  "client_id": "app-brim",
  "client_secret": "secret-value",
  "grant_type": "client_credentials"
}
```

Contoh response:

```json
{
  "access_token": "jwt-access-token",
  "refresh_token": "refresh-token",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "brim.site.read brim.wo.read"
}
```

Token requirement:

- Access token berbentuk JWT atau opaque token dengan introspection.
- Token memiliki expiry.
- Refresh token dapat direvoke.
- Token membawa scope.
- Token membawa client_id.
- Signing key dapat dirotasi.
- Token failure tercatat.

Acceptance Criteria:

- Client valid bisa mendapatkan token.
- Client invalid ditolak.
- Token expired tidak bisa digunakan.
- Refresh token valid dapat menghasilkan access token baru.
- Token revoke membuat token tidak bisa dipakai lagi.

---

## 11.6 Database Connection Management

Admin/DBA dapat membuat koneksi ke source database external.

Supported database MVP:

1. PostgreSQL.
2. MySQL/MariaDB.
3. Oracle.
4. SQL Server.

Field umum:

- Connection name.
- Database type.
- Host.
- Port.
- Database name/service name/SID.
- Schema.
- Username.
- Password/secret reference.
- SSL/TLS mode.
- Min pool size.
- Max pool size.
- Connection timeout.
- Query timeout.
- Environment.
- Status.
- Tags.

Fitur:

- Create connection.
- Update connection.
- Disable connection.
- Test connection.
- Health check.
- Rotate secret.
- Mask password.
- Audit change.

Security requirement:

- Password tidak boleh disimpan plain text.
- Secret harus encrypted atau disimpan di secret manager.
- Connection string tidak boleh muncul di log.
- Hanya role tertentu yang bisa melihat/mengubah koneksi.

Acceptance Criteria:

- Admin bisa test connection sebelum menyimpan.
- Connection gagal menampilkan error aman tanpa expose password.
- Connector service hanya membaca secret melalui mekanisme aman.
- Setiap perubahan connection tercatat di audit log.

---

## 11.7 Dynamic API Builder

Admin dapat membuat endpoint API secara dinamis dari dashboard.

Field API definition:

- API ID.
- API name.
- Namespace.
- Path.
- HTTP method.
- Description.
- Database connection.
- Connector type.
- Query template.
- Parameter definition.
- Response mapping.
- Cache rule.
- Rate limit rule.
- Required scope.
- IP whitelist rule.
- Status draft/published/disabled.
- Version.
- Created by.
- Approved by.
- Published at.

Supported HTTP method MVP:

- GET.
- POST.

POST digunakan untuk query/search dengan body parameter.

Contoh endpoint:

```http
GET /api/v1/brim/sites/{site_id}
POST /api/v1/brim/workorders/search
GET /api/v1/customer/profile/{customer_id}
```

Contoh query template:

```sql
SELECT
  site_id,
  customer_name,
  status,
  created_at
FROM public.customer_site
WHERE site_id = :site_id
LIMIT 1
```

Parameter definition:

```json
[
  {
    "name": "site_id",
    "source": "path",
    "type": "string",
    "required": true,
    "max_length": 50
  }
]
```

Requirement penting:

- Query harus menggunakan parameter binding.
- Tidak boleh string concatenation langsung dari user input.
- Query harus divalidasi sebelum publish.
- Query timeout wajib ada.
- Limit row wajib ada untuk endpoint list/search.
- Pagination wajib untuk result besar.
- Query write harus disabled di MVP kecuali secara eksplisit diaktifkan melalui approval.

Acceptance Criteria:

- Admin bisa membuat API dari dashboard tanpa deploy service baru.
- Endpoint published langsung bisa dipakai client sesuai permission.
- Endpoint draft tidak bisa diakses client.
- Parameter invalid ditolak sebelum query dijalankan.
- Query tanpa binding parameter tidak boleh dipublish.

---

## 11.8 API Permission & Scope

Setiap API wajib memiliki scope.

Contoh scope:

```text
brim.site.read
brim.wo.read
customer.profile.read
inventory.stock.read
```

Rule:

- Client hanya bisa akses API jika memiliki scope yang sesuai.
- Scope dapat diberikan per client.
- Scope dapat diberikan per environment.
- Scope dapat direvoke.
- Scope change tercatat di audit log.

Acceptance Criteria:

- Request tanpa token ditolak 401.
- Request token valid tapi scope tidak sesuai ditolak 403.
- Request token valid dan scope sesuai diteruskan ke API.

---

## 11.9 IP Whitelist

DDAG wajib mendukung whitelist IP.

Level whitelist:

- Global.
- Per client.
- Per API.

Format:

- Single IP.
- CIDR.

Contoh:

```json
{
  "client_id": "app-mobile-brim",
  "ip_whitelist": [
    "10.10.10.0/24",
    "103.20.30.40"
  ]
}
```

Acceptance Criteria:

- Request dari IP tidak terdaftar ditolak.
- Request dari IP valid dilanjutkan.
- IP blocked tercatat sebagai security event.

---

## 11.10 Rate Limiter & Quota

DDAG wajib memiliki rate limiter.

Level rate limit:

- Per client.
- Per API.
- Per IP.
- Global fallback.

Metric rate limit:

- Request per second.
- Request per minute.
- Request per hour.
- Request per day.

Contoh rule:

```json
{
  "client_id": "app-brim",
  "endpoint": "GET /api/v1/brim/sites/{site_id}",
  "requests_per_minute": 300,
  "requests_per_day": 100000
}
```

Acceptance Criteria:

- Request melebihi limit mendapat HTTP 429.
- Response 429 memiliki pesan yang jelas.
- Rate limit hit tercatat di metric Prometheus.
- Admin bisa mengubah limit dari dashboard.

---

## 11.11 Cache Management

Cache harus dapat dikonfigurasi per endpoint.

Fitur:

- Enable/disable cache per API.
- TTL per API.
- Cache key strategy.
- Cache by path parameter.
- Cache by query parameter.
- Cache by request body hash.
- Cache by client_id optional.
- Manual purge cache.
- Purge per endpoint.
- Purge per cache key.
- Cache bypass untuk admin/debug.
- Cache hit/miss metric.

Contoh rule:

```json
{
  "endpoint": "GET /api/v1/brim/sites/{site_id}",
  "enabled": true,
  "ttl_seconds": 300,
  "cache_key": "client_id:path:query_params",
  "vary_by_client": true
}
```

Acceptance Criteria:

- Endpoint dengan cache aktif mengembalikan response dari cache saat key sama.
- Cache expired sesuai TTL.
- Admin bisa purge cache dari dashboard.
- Cache hit/miss muncul di Prometheus.

---

## 11.12 Database Connector Services

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
- Membuka koneksi ke source database.
- Menggunakan connection pool.
- Menjalankan prepared statement.
- Binding parameter.
- Timeout query.
- Retry terbatas untuk transient error.
- Circuit breaker.
- Normalize error.
- Return result dalam format standar.
- Mencatat query duration.
- Tidak expose raw database error ke client.

Connector tidak boleh:

- Menerima request langsung dari public internet.
- Menerima raw SQL dari client aplikasi.
- Menyimpan password dalam log.
- Menjalankan query tanpa timeout.

Acceptance Criteria:

- Setiap connector punya health endpoint.
- Connector dapat di-scale sendiri.
- Error connector tidak membuat service lain crash.
- Query lama diputus berdasarkan timeout.

---

## 11.13 Response Format Standard

Semua API DDAG harus memiliki format response standar.

Success response:

```json
{
  "success": true,
  "request_id": "req-xxxxx",
  "data": {},
  "meta": {
    "cached": false,
    "duration_ms": 120
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
    "total": 500
  },
  "meta": {
    "cached": true,
    "duration_ms": 25
  }
}
```

Error response:

```json
{
  "success": false,
  "request_id": "req-xxxxx",
  "error": {
    "code": "FORBIDDEN",
    "message": "You do not have permission to access this API"
  }
}
```

Acceptance Criteria:

- Semua response memiliki request_id.
- Error tidak expose query atau password.
- Response cached memiliki flag cached.

---

## 11.14 Audit Log

Semua aktivitas penting harus tercatat.

Audit event:

- Login success/failure.
- Create/update/delete user.
- Create/update/delete client.
- Generate/rotate secret.
- Create/update/publish/disable API.
- Change database connection.
- Test connection.
- Change RBAC.
- Change scope.
- Change IP whitelist.
- Change rate limit.
- Purge cache.
- Token issued/revoked.
- Unauthorized request.
- Forbidden request.

Field audit:

- Audit ID.
- Timestamp.
- Actor user/client.
- Action.
- Resource type.
- Resource ID.
- Old value hash/diff optional.
- New value hash/diff optional.
- IP address.
- User agent.
- Request ID.
- Status.

Acceptance Criteria:

- Audit log tidak bisa diedit dari dashboard.
- Audit log bisa difilter berdasarkan user, action, resource, dan tanggal.
- Security event mudah ditemukan.

---

## 11.15 Monitoring & Observability

Monitoring wajib tersedia untuk semua endpoint dinamis dan service.

Tools:

- Prometheus untuk metric.
- Grafana untuk dashboard.
- Loki/OpenSearch optional untuk centralized logs.
- Alertmanager optional.

Metric wajib:

- Total request per endpoint.
- Total request per client.
- Latency p50/p95/p99.
- Error rate.
- HTTP status distribution.
- Cache hit count.
- Cache miss count.
- Cache hit ratio.
- Database query duration.
- Database connection pool usage.
- Token request count.
- Token failure count.
- Unauthorized count.
- Forbidden count.
- Rate limited count.
- IP whitelist blocked count.
- Connector health status.
- Connector error count.
- Pod CPU usage.
- Pod memory usage.
- Goroutine count untuk service Go.
- Queue depth untuk worker bila ada.

Dashboard Grafana minimal:

1. DDAG Overview.
2. API Traffic.
3. API Latency.
4. API Error Rate.
5. Cache Performance.
6. Client Usage.
7. Connector Health.
8. Database Query Duration.
9. Security Events.
10. Pod Resource Usage.

Acceptance Criteria:

- Setiap service expose `/metrics`.
- Setiap service expose `/healthz` dan `/readyz`.
- Setiap API request menghasilkan metric.
- Admin bisa melihat top slow API.
- Admin bisa melihat top error API.

---

## 11.16 Dashboard Admin

Dashboard harus simple, jelas, dan flow-nya terarah.

Menu MVP:

1. Overview.
2. API Management.
3. Database Connections.
4. Clients / Applications.
5. Users.
6. Roles & Permissions.
7. Token & Scopes.
8. Cache Management.
9. Rate Limit.
10. IP Whitelist.
11. Logs.
12. Audit Logs.
13. Monitoring.
14. Settings.

Dashboard overview menampilkan:

- Total API active.
- Total client active.
- Total request today.
- Error rate today.
- Average latency.
- Cache hit ratio.
- Top 5 endpoint by traffic.
- Top 5 slow endpoint.
- Top 5 error endpoint.
- Connector status.

Acceptance Criteria:

- Admin bisa membuat API end-to-end dari dashboard.
- Admin bisa melihat API aktif/nonaktif.
- Admin bisa melihat health source DB.
- Admin bisa melihat usage per client.
- Dashboard tetap usable tanpa fitur yang terlalu rumit.

---

## 12. Dynamic API Lifecycle

Status API:

```text
DRAFT -> REVIEW -> PUBLISHED -> DISABLED -> ARCHIVED
```

### 12.1 Draft

API dibuat oleh admin/DBA tetapi belum bisa diakses client.

### 12.2 Review

API menunggu validasi query, security, dan permission.

### 12.3 Published

API aktif dan bisa diakses client sesuai scope.

### 12.4 Disabled

API dimatikan sementara. Request akan mendapat response 403/404 sesuai konfigurasi.

### 12.5 Archived

API tidak digunakan lagi tetapi metadata dan audit tetap disimpan.

Acceptance Criteria:

- API draft tidak bisa diakses client.
- API published bisa diakses sesuai permission.
- API disabled langsung tidak bisa diakses.
- Perubahan status tercatat di audit log.

---

## 13. Security Requirements

### 13.1 Authentication

- Dashboard menggunakan login user.
- API consumer menggunakan OAuth2 bearer token.
- Token memiliki expiry.
- Refresh token dapat direvoke.

### 13.2 Authorization

- RBAC untuk dashboard.
- Scope untuk API access.
- Permission per client.
- Permission per endpoint.

### 13.3 Network Security

- IP whitelist.
- TLS wajib untuk public/internal API.
- Connector service tidak diekspos publik.
- Admin dashboard dibatasi network/VPN bila memungkinkan.

### 13.4 Secret Security

- Password database tidak disimpan plain text.
- Secret encryption at rest.
- Secret rotation.
- Secret tidak muncul di log.
- Client secret hanya tampil saat generate/rotate.

### 13.5 Query Security

- Prepared statement wajib.
- Parameter binding wajib.
- Query timeout wajib.
- Row limit wajib untuk list/search.
- Raw SQL dari client tidak diperbolehkan.
- Query write disabled by default.
- Error database tidak boleh expose detail sensitif.

### 13.6 Data Security

- Data masking optional untuk field sensitif.
- Response field filtering per endpoint.
- Audit akses data per client.
- Optional encryption untuk sensitive metadata.

Acceptance Criteria:

- Security rule dijalankan di backend, bukan hanya UI.
- Unauthorized/forbidden event tercatat.
- No plain secret in logs.
- Query injection attempt ditolak.

---

## 14. Non-Functional Requirements

### 14.1 Performance

Target awal:

- p95 latency untuk cached API: < 100 ms.
- p95 latency untuk non-cached simple query: < 500 ms, tergantung source DB.
- API Gateway mampu handle minimal 500 RPS di MVP environment dengan horizontal scaling.
- Query timeout default: 30 detik.
- Default response limit: 100 row.
- Maximum response limit configurable.

### 14.2 Availability

- API Gateway harus bisa di-scale minimal 2 replica untuk production.
- Auth service minimal 2 replica untuk production.
- Connector service minimal 2 replica untuk production bila critical.
- Metadata DB harus memiliki backup strategy.
- Redis/cache harus memiliki persistence/HA sesuai environment.

### 14.3 Scalability

- Horizontal scaling per service.
- Connector per database type dapat di-scale mandiri.
- API Gateway stateless.
- Auth service stateless jika menggunakan JWT.
- Rate limiter menggunakan Redis/shared storage.

### 14.4 Reliability

- Circuit breaker untuk source database bermasalah.
- Retry terbatas hanya untuk transient error.
- Timeout wajib di semua external call.
- Graceful shutdown.
- Readiness probe memastikan pod tidak menerima traffic sebelum siap.

### 14.5 Maintainability

- Config API tersimpan di metadata DB.
- Service modular.
- Log terstruktur JSON.
- Request ID wajib propagated.
- Migration database menggunakan versioned migration.
- Coding standard dan test minimal wajib.

---

## 15. Data Model — Initial Draft

### 15.1 users

```text
id
name
email
username
password_hash
status
last_login_at
created_at
updated_at
```

### 15.2 roles

```text
id
name
description
created_at
updated_at
```

### 15.3 permissions

```text
id
code
description
created_at
updated_at
```

### 15.4 user_roles

```text
user_id
role_id
```

### 15.5 clients

```text
id
client_id
client_name
client_secret_hash
owner_user_id
environment
status
access_token_ttl
refresh_token_ttl
created_at
updated_at
```

### 15.6 client_scopes

```text
client_id
scope_id
```

### 15.7 scopes

```text
id
scope_code
description
created_at
updated_at
```

### 15.8 database_connections

```text
id
name
database_type
host
port
database_name
service_name
schema_name
username
secret_ref
ssl_mode
min_pool_size
max_pool_size
connection_timeout_ms
query_timeout_ms
environment
status
created_by
created_at
updated_at
```

### 15.9 api_definitions

```text
id
name
namespace
path
method
description
database_connection_id
connector_type
query_template
status
version
required_scope
created_by
approved_by
published_at
created_at
updated_at
```

### 15.10 api_parameters

```text
id
api_definition_id
name
source
param_type
required
default_value
max_length
validation_rule
created_at
updated_at
```

### 15.11 client_api_access

```text
client_id
api_definition_id
allowed
created_at
updated_at
```

### 15.12 cache_rules

```text
id
api_definition_id
enabled
ttl_seconds
cache_key_strategy
vary_by_client
created_at
updated_at
```

### 15.13 rate_limit_rules

```text
id
client_id
api_definition_id
requests_per_second
requests_per_minute
requests_per_hour
requests_per_day
created_at
updated_at
```

### 15.14 ip_whitelists

```text
id
client_id
api_definition_id
ip_cidr
status
created_at
updated_at
```

### 15.15 audit_logs

```text
id
request_id
actor_type
actor_id
action
resource_type
resource_id
ip_address
user_agent
status
metadata_json
created_at
```

### 15.16 api_request_logs

```text
id
request_id
client_id
api_definition_id
method
path
status_code
latency_ms
cached
source_db_duration_ms
ip_address
created_at
```

---

## 16. Internal API Contracts

### 16.1 API Gateway to Policy Engine

Request:

```json
{
  "request_id": "req-123",
  "client_id": "app-brim",
  "method": "GET",
  "path": "/api/v1/brim/sites/ABC123",
  "scope": "brim.site.read",
  "ip_address": "10.10.10.5"
}
```

Response:

```json
{
  "allowed": true,
  "reason": null,
  "rate_limit": {
    "remaining": 299,
    "reset_seconds": 60
  }
}
```

### 16.2 API Gateway to Connector

Request:

```json
{
  "request_id": "req-123",
  "connection_id": "conn-postgres-brim",
  "query_template": "SELECT * FROM site WHERE site_id = :site_id",
  "parameters": {
    "site_id": "ABC123"
  },
  "timeout_ms": 30000,
  "limit": 100
}
```

Response:

```json
{
  "success": true,
  "duration_ms": 85,
  "rows": [
    {
      "site_id": "ABC123",
      "status": "ACTIVE"
    }
  ]
}
```

---

## 17. API Examples

### 17.1 Get Token

```http
POST /oauth/token
Content-Type: application/json

{
  "client_id": "app-brim",
  "client_secret": "secret",
  "grant_type": "client_credentials"
}
```

### 17.2 Refresh Token

```http
POST /oauth/refresh
Content-Type: application/json

{
  "refresh_token": "refresh-token-value",
  "grant_type": "refresh_token"
}
```

### 17.3 Consume Dynamic API

```http
GET /api/v1/brim/sites/ABC123
Authorization: Bearer jwt-access-token
```

Success:

```json
{
  "success": true,
  "request_id": "req-abc123",
  "data": {
    "site_id": "ABC123",
    "status": "ACTIVE"
  },
  "meta": {
    "cached": false,
    "duration_ms": 120
  }
}
```

---

## 18. Error Codes

| HTTP Status | Code | Description |
|---|---|---|
| 400 | BAD_REQUEST | Parameter/request tidak valid |
| 401 | UNAUTHORIZED | Token tidak ada/invalid/expired |
| 403 | FORBIDDEN | Scope/IP/RBAC tidak valid |
| 404 | API_NOT_FOUND | Endpoint tidak ditemukan atau disabled |
| 408 | QUERY_TIMEOUT | Query source database timeout |
| 409 | CONFLICT | Konflik konfigurasi/data |
| 429 | RATE_LIMITED | Rate limit terlampaui |
| 500 | INTERNAL_ERROR | Error internal DDAG |
| 502 | CONNECTOR_ERROR | Error di connector database |
| 503 | SOURCE_DB_UNAVAILABLE | Source database tidak tersedia |

---

## 19. Deployment Requirements

### 19.1 Container Image

Setiap service wajib punya image sendiri:

```text
ddag-admin-dashboard
ddag-admin-backend
ddag-auth-service
ddag-api-gateway
ddag-policy-engine
ddag-cache-service
ddag-connector-postgres
ddag-connector-mysql
ddag-connector-oracle
ddag-connector-sqlserver
ddag-worker
```

### 19.2 Kubernetes Resources

Minimal resource per service:

- Deployment.
- Service.
- ConfigMap.
- Secret.
- ServiceMonitor untuk Prometheus.
- HPA untuk service high traffic.
- Ingress untuk dashboard dan API gateway.

### 19.3 Health Endpoint

Semua service wajib expose:

```http
GET /healthz
GET /readyz
GET /metrics
```

### 19.4 Environment Separation

Environment minimal:

- dev.
- staging.
- prod.

Konfigurasi dan secret tidak boleh bercampur antar environment.

---

## 20. Admin Flow

### 20.1 Membuat Database Connection

1. Admin/DBA login dashboard.
2. Buka menu Database Connections.
3. Pilih database type.
4. Isi host, port, database/service name, schema, username, secret.
5. Test connection.
6. Simpan connection.
7. Connection masuk status active.

### 20.2 Membuat Dynamic API

1. Admin buka API Management.
2. Klik Create API.
3. Isi namespace, path, method, deskripsi.
4. Pilih database connection.
5. Isi query template.
6. Definisikan parameter.
7. Test query dengan sample parameter.
8. Atur response mapping.
9. Atur cache rule.
10. Atur required scope.
11. Atur rate limit.
12. Publish API.

### 20.3 Memberikan Akses ke Client

1. Admin buka Client Management.
2. Pilih client/application.
3. Assign API.
4. Assign scope.
5. Atur IP whitelist.
6. Atur rate limit.
7. Save.
8. Client dapat menggunakan token untuk akses API.

### 20.4 Client Mengakses API

1. Client request token ke `/oauth/token`.
2. Client menerima access token.
3. Client call endpoint DDAG dengan bearer token.
4. API Gateway validasi token.
5. Policy Engine validasi scope, IP, dan rate limit.
6. Gateway cek cache.
7. Jika cache miss, gateway call connector.
8. Connector query source database.
9. Response dikembalikan ke client.
10. Metric dan log tercatat.

---

## 21. MVP Milestones

### Phase 0 — Foundation

Deliverables:

- Repository setup.
- Base service template Go.
- Dockerfile per service.
- PostgreSQL metadata schema.
- Basic CI pipeline.
- Basic logging/request_id.

### Phase 1 — Auth & Dashboard Basic

Deliverables:

- Login page.
- User management basic.
- Role basic.
- Client management basic.
- OAuth2 token endpoint.
- Refresh token endpoint.

### Phase 2 — Dynamic API Core

Deliverables:

- API definition CRUD.
- Database connection CRUD.
- PostgreSQL connector.
- API Gateway routing.
- Parameter binding.
- Standard response.

### Phase 3 — Security & Policy

Deliverables:

- Scope validation.
- RBAC dashboard/backend.
- IP whitelist.
- Rate limiter.
- Audit log.
- Token revoke.

### Phase 4 — Multi-DB Connector

Deliverables:

- MySQL connector.
- Oracle connector.
- SQL Server connector.
- Connector health check.
- Query timeout.
- Connection pool config.

### Phase 5 — Cache & Monitoring

Deliverables:

- Redis cache.
- Cache rule per API.
- Manual purge cache.
- Prometheus metrics.
- Grafana dashboard.
- Top slow endpoint.
- Top error endpoint.

### Phase 6 — Hardening for Production

Deliverables:

- HPA config.
- Kubernetes manifests/Helm.
- Secret management.
- Security test.
- Load test.
- Backup/restore metadata DB.
- Documentation.

---

## 22. Success Metrics

### 22.1 Product Metrics

- Waktu pembuatan API baru turun minimal 70% dibanding manual backend.
- Minimal 80% API read-only internal dapat dibuat melalui DDAG.
- Semua API memiliki owner/client/scope yang jelas.
- 100% request API memiliki request_id dan audit trail.

### 22.2 Technical Metrics

- p95 cached endpoint < 100 ms.
- p95 simple non-cached endpoint < 500 ms, tergantung source DB.
- Error rate < 1% untuk service DDAG di luar error source DB.
- Cache hit ratio minimal 50% untuk endpoint yang memang cacheable.
- API Gateway uptime target 99.5% untuk MVP production.

### 22.3 Security Metrics

- 0 plain text secret di log.
- 100% endpoint membutuhkan token.
- 100% endpoint memiliki scope.
- 100% config change tercatat di audit log.
- Semua request forbidden/unauthorized tercatat.

---

## 23. Risks & Mitigation

| Risk | Impact | Mitigation |
|---|---:|---|
| Query berat membebani source DB | High | Query timeout, row limit, review query, cache, monitoring |
| Salah konfigurasi scope membuka data sensitif | High | RBAC, approval flow, audit log, default deny |
| Secret bocor di log | High | Masking, structured logging, secret manager |
| Connector down | Medium | Health check, HPA, circuit breaker |
| Metadata DB down | High | Backup, HA, connection retry, disaster recovery plan |
| API terlalu bebas seperti raw SQL gateway | High | No raw SQL from client, query template approved only |
| Dashboard terlalu kompleks | Medium | MVP flow sederhana, progressive enhancement |
| Cache menampilkan data stale | Medium | TTL jelas, purge manual, cache rule per API |
| Rate limit tidak konsisten multi-pod | Medium | Shared Redis-based limiter |

---

## 24. Open Questions

1. Apakah MVP hanya read-only API atau perlu write API terbatas?
2. Apakah perlu approval flow sejak MVP atau cukup role-based publish?
3. Apakah token menggunakan JWT penuh atau opaque token dengan introspection?
4. Apakah secret akan disimpan encrypted di PostgreSQL atau memakai Vault/Kubernetes Secret?
5. Apakah dashboard hanya internal VPN atau exposed melalui public domain dengan WAF?
6. Apakah response mapping perlu transform kompleks atau cukup raw JSON row di MVP?
7. Apakah audit log disimpan penuh di PostgreSQL atau sebagian dikirim ke log storage?
8. Apakah pagination wajib untuk semua endpoint list/search sejak MVP?
9. Apakah cache invalidation manual cukup untuk MVP?
10. Apakah tenant isolation wajib hard isolation atau logical isolation cukup untuk fase awal?

---

## 25. Recommended MVP Decision

Agar aplikasi tetap simple tapi high impact, MVP sebaiknya fokus pada:

1. Read-only dynamic API.
2. PostgreSQL, MySQL, Oracle, SQL Server connector.
3. Dashboard admin sederhana.
4. OAuth2 client credentials + refresh token.
5. RBAC + scope + IP whitelist + rate limit.
6. Cache per endpoint.
7. Prometheus/Grafana dari awal.
8. Audit log lengkap.
9. Service/pod terpisah.
10. PostgreSQL 18 untuk metadata DB.

Write API, approval workflow kompleks, GraphQL, SSO, dan advanced masking dapat masuk fase berikutnya setelah core DDAG stabil.

---

## 26. Final Product Principle

DDAG harus selalu mengikuti prinsip:

```text
Simple to operate.
Secure by default.
Dynamic but controlled.
Observable from day one.
Scalable per service.
No direct database exposure.
No raw SQL from client.
Every API has owner, scope, cache rule, and audit trail.
```

## 27. Product Roadmap

untuk database internal system tidak pakai pull image tapi koneksi dari luar nanti kecuali untuk redis.
dan 1 lagi semua koneksi harus punya connection pool dan diatur di config.