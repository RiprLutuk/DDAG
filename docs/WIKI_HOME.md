# DDAG — Dynamic Database API Gateway

DDAG is a zero-trust API gateway that transforms SQL queries into production-ready REST API endpoints with zero backend boilerplate. It natively supports PostgreSQL, MySQL, Oracle, and SQL Server with dialect-aware parsing.

---

## How It Works

Unlike ORMs or GraphQL engines that force you to adopt their query languages, DDAG respects the native SQL of the target database.

### 1. Connection & Dialects

Developers register database connections via the dashboard. DDAG automatically detects the target database's dialect. MySQL uses `backticks`, SQL Server uses `[brackets]`, Oracle uses `q'[quotes]'`, and PostgreSQL uses `$$dollar_quotes$$`. No syntax conversion is forced.

### 2. API Template Registration

Developers define an endpoint (e.g., `GET /api/v1/users/:status`) and provide a SQL template:

```sql
SELECT id, username FROM users WHERE status = :status
```

The DDAG parser extracts `:status` as a required parameter, infers its data type, and automatically registers it into the OpenAPI schema. There is no string concatenation. All parameters are bound safely via the native driver's prepared statements.

### 3. Execution Pipeline (Zero-Trust)

When a client hits an endpoint, the request traverses the following pipeline:

1. **Admission Control** — Rejects oversized payloads and malicious path traversals.
2. **Authentication** — Validates Bearer Tokens (JWT) or HMAC signatures. Anti-replay protection via Redis is mandatory. If Redis is unavailable, the gateway rejects boot (fail-closed).
3. **Cache Check** — `GET` and `QUERY` methods are checked against the Redis/memory cache before hitting the database.
4. **Circuit Breaker** — If the target database is down or overloaded, the circuit breaker trips to `OPEN` and the gateway fails fast, preventing cascading failures.
5. **Dialect-Aware Binder** — JSON parameters are safely bound to SQL using native prepared statements. The parser recognizes dialect-specific escaped identifiers (`""` for PostgreSQL, ``` `` ``` for MySQL, `]]` for SQL Server).

### 4. Response Normalization

Output is converted into standard JSON regardless of the source database. Oracle's `NUMBER(1)` and MySQL's `TINYINT(1)` are both rendered as booleans. PostgreSQL's `JSONB` and SQL Server's `NVARCHAR(MAX)` are parsed as JSON objects.

Response format:

```json
{
  "success": true,
  "request_id": "req-123456",
  "row_count": 1,
  "rows": [
    { "id": 1, "username": "alice" }
  ]
}
```

---

## Core Features

**RFC 10008 QUERY Method**
Resolves the `GET` vs `POST` dilemma for complex search parameters. The `QUERY` method sends a body like POST but remains cacheable and read-only like GET. Swagger UI is customized to render this method correctly.

**Operation-Aware Governance**
The gateway automatically detects SQL side effects. Endpoints published as read-only (`GET`/`QUERY`) cannot execute `INSERT`, `UPDATE`, `DELETE`, `CALL`, `EXEC`, `GRANT`, `REVOKE`, or `TRUNCATE`, even if attempted. The validator uses dialect-aware string stripping so dangerous keywords cannot be hidden inside string literals or comments.

**Circuit Breaker**
Trips to `OPEN` after N consecutive failures. Rejects new requests during the cooldown window. Automatically resets to `CLOSED` once the database recovers. Prevents connection pool exhaustion when the database is slow.

**Connection Pooling**
Each database connection has an independent pool with configurable min/max limits. Connection and query timeouts are enforced per connection, not globally.

**Smart Dashboard**
Vue.js dashboard for managing APIs, database connections, and viewing metrics. Connection forms automatically adapt fields based on the database type (Service Name for Oracle, Schema for PostgreSQL, default ports per dialect). API Playground allows testing endpoints directly from the browser.

**Multi-Dialect Binder**
A single parser handles 4 database dialects. Escaped identifiers are handled per dialect: MySQL double backticks, SQL Server `]]`, PostgreSQL `""`, Oracle q-quotes. PostgreSQL dollar-quoting (`$$tag$...$tag$$`) is also supported. The publish-time parser and runtime binder share the same logic to prevent drift.

**Backup & Recovery**
Automated scheduled backups with AES encryption. The UI displays filenames, sizes, durations, and timestamps without exposing absolute file paths to the user.

---

## CI/CD Matrix

The GitHub Actions pipeline runs tests on 4 database engines in parallel using service containers:

| Job | Image | Tested Capabilities |
|-----|-------|------------|
| Unit test | — | Parser, binder, validator, circuit breaker, cache |
| PostgreSQL | Local container | `generate_series`, pagination, response mapping |
| MySQL 8.0 | `mysql:8.0` | Backtick escaping, `TINYINT(1)` boolean coercion, pagination |
| SQL Server 2022 | `mcr.microsoft.com/mssql/server:2022-latest` | Bracket identifiers, `BIT` type, `IDENTITY`, `OFFSET/FETCH` pagination |
| Oracle 21c XE | `gvenzl/oracle-xe:21-slim` | q-quote strings, `NUMBER(1)` boolean, DDL exception handling, pagination |

All integration tests utilize internal DDAG connectors (`BuildMySQL`, `BuildOracle`, `BuildSQLServer`) rather than manual `database/sql` implementations, ensuring pagination and response mapping are thoroughly tested.

---

## Documentation

- **[Deployment Guide](../DEPLOY_VPS.md)** — Installation via Docker Compose on a VPS.
- **[Architecture](../ARCHITECTURE.md)** — Internal pipeline, goroutine model, memory layout.
- **[Backup & Recovery](../BACKUP_RECOVERY.md)** — Backup schedules, encryption, restore procedures.
- **[Operations & Governance](../OPERATIONS.md)** — Best practices for publishing production endpoints.
- **[PRD](../PRD_V4.md)** — Complete Product Requirements Document.