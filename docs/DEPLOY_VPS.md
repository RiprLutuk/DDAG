# DDAG VPS Deployment Guide

This guide covers a single VPS deployment using systemd services behind Caddy.
Use it when you do not want Docker, Kubernetes, or Helm on the target server.

The recommended public layout uses two hostnames:

| Hostname | Public role | Internal target |
|---|---|---|
| `ddag.example.com` | Dashboard, admin control-plane API, OAuth token API | dashboard `127.0.0.1:3000`, admin-backend `127.0.0.1:8080`, auth-service `127.0.0.1:8081` |
| `api.ddag.example.com` | Published dynamic APIs, OpenAPI, API catalog | api-gateway `127.0.0.1:8082` |

Keep connector ports, Redis, and metadata PostgreSQL private. Do not expose
`8090-8093`, `6379`, or the metadata database to the public internet.

## Prerequisites

- Ubuntu 22.04 or 24.04 VPS.
- DNS records for `ddag.example.com` and `api.ddag.example.com`.
- Go 1.26+.
- Node.js 20+ and npm.
- Caddy 2.
- Redis reachable from the VPS. Local Redis on `127.0.0.1:6379` is fine for one
  VPS.
- PostgreSQL metadata database reachable from the VPS.
- Source databases reachable from the relevant connector service.

For a one-machine install, PostgreSQL and Redis can run on the same VPS, but
production deployments should back up PostgreSQL outside the application host.

## Firewall

Only SSH and HTTPS need to be public:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

All DDAG services should bind to `127.0.0.1`.

## User and Directories

```bash
sudo useradd --system --home /opt/ddag --shell /usr/sbin/nologin ddag
sudo mkdir -p /opt/ddag /etc/ddag /var/log/ddag
sudo chown -R ddag:ddag /opt/ddag /var/log/ddag
```

Clone and build from a normal admin user, then place the app under `/opt/ddag`:

```bash
sudo git clone https://github.com/RiprLutuk/DDAG.git /opt/ddag
sudo chown -R ddag:ddag /opt/ddag
cd /opt/ddag
sudo -u ddag make build
cd apps/dashboard
sudo -u ddag npm ci
sudo -u ddag npm run build
```

## Environment File

Create `/etc/ddag/ddag.env` and keep it readable only by root and the `ddag`
group:

```bash
sudo install -m 0640 -o root -g ddag /dev/null /etc/ddag/ddag.env
```

Example production baseline:

```env
DDAG_ENV=prod
DDAG_LOG_LEVEL=info

DDAG_DB_HOST=metadata-db.internal
DDAG_DB_PORT=5432
DDAG_DB_USER=ddag
DDAG_DB_PASSWORD=replace-with-strong-password
DDAG_DB_NAME=ddag
DDAG_DB_SSLMODE=require
DDAG_DB_MIN_CONNS=2
DDAG_DB_MAX_CONNS=20
DDAG_DB_CONNECT_TIMEOUT=5s

DDAG_REDIS_ADDR=127.0.0.1:6379
DDAG_REDIS_PASSWORD=
DDAG_REDIS_DB=0

DDAG_MASTER_KEY=replace-with-openssl-rand-base64-32
DDAG_SESSION_SECRET=replace-with-long-random-secret
DDAG_SESSION_COOKIE_SECURE=true
DDAG_DASHBOARD_ORIGINS=https://ddag.example.com
DDAG_TRUSTED_PROXIES=127.0.0.1/32
DDAG_INTERNAL_AUTH_SECRET=replace-with-long-random-shared-secret

DDAG_TOKEN_ISSUER=ddag
DDAG_TOKEN_AUDIENCE=ddag-api
DDAG_JWKS_URL=http://127.0.0.1:8081/.well-known/jwks.json

DDAG_CONNECTOR_POSTGRES_URL=http://127.0.0.1:8090
DDAG_CONNECTOR_MYSQL_URL=http://127.0.0.1:8091
DDAG_CONNECTOR_ORACLE_URL=http://127.0.0.1:8092
DDAG_CONNECTOR_SQLSERVER_URL=http://127.0.0.1:8093

DDAG_BACKPRESSURE_QUEUE_SIZE=512
DDAG_BACKPRESSURE_TIMEOUT=3s

NUXT_PUBLIC_API_BASE=https://ddag.example.com
NUXT_PUBLIC_AUTH_BASE=https://ddag.example.com
NUXT_PUBLIC_GATEWAY_BASE=https://api.ddag.example.com
```

Generate secrets on your workstation or the VPS:

```bash
openssl rand -base64 32
openssl rand -hex 48
```

Back up `DDAG_MASTER_KEY` securely. Without it, DDAG cannot decrypt stored source
database credentials.

## Migrate and Seed

Run migrations and production core seed:

```bash
cd /opt/ddag
set -a
. /etc/ddag/ddag.env
set +a
sudo -u ddag ./bin/ddag-migrate --seed
```

For a demo/staging VPS only, use `--demo` instead of `--seed`.

## systemd Services

Create one systemd unit per service. Start with this template and change
`DDAG_HTTP_ADDR` and `ExecStart`.

`/etc/systemd/system/ddag-admin-backend.service`:

```ini
[Unit]
Description=DDAG admin-backend
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ddag
Group=ddag
WorkingDirectory=/opt/ddag
EnvironmentFile=/etc/ddag/ddag.env
Environment=DDAG_HTTP_ADDR=127.0.0.1:8080
ExecStart=/opt/ddag/bin/ddag-admin-backend
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Create the remaining Go service units with these values:

| Unit | `DDAG_HTTP_ADDR` | `ExecStart` |
|---|---|---|
| `ddag-auth-service.service` | `127.0.0.1:8081` | `/opt/ddag/bin/ddag-auth-service` |
| `ddag-api-gateway.service` | `127.0.0.1:8082` | `/opt/ddag/bin/ddag-api-gateway` |
| `ddag-policy-engine.service` | `127.0.0.1:8083` | `/opt/ddag/bin/ddag-policy-engine` |
| `ddag-cache-service.service` | `127.0.0.1:8084` | `/opt/ddag/bin/ddag-cache-service` |
| `ddag-worker.service` | `127.0.0.1:8085` | `/opt/ddag/bin/ddag-worker` |
| `ddag-connector-postgres.service` | `127.0.0.1:8090` | `/opt/ddag/bin/ddag-connector-postgres` |
| `ddag-connector-mysql.service` | `127.0.0.1:8091` | `/opt/ddag/bin/ddag-connector-mysql` |
| `ddag-connector-oracle.service` | `127.0.0.1:8092` | `/opt/ddag/bin/ddag-connector-oracle` |
| `ddag-connector-sqlserver.service` | `127.0.0.1:8093` | `/opt/ddag/bin/ddag-connector-sqlserver` |

Only enable connector services for database types you will actually use.

Dashboard unit:

`/etc/systemd/system/ddag-dashboard.service`:

```ini
[Unit]
Description=DDAG dashboard
After=network-online.target ddag-admin-backend.service
Wants=network-online.target

[Service]
Type=simple
User=ddag
Group=ddag
WorkingDirectory=/opt/ddag/apps/dashboard
EnvironmentFile=/etc/ddag/ddag.env
Environment=NODE_ENV=production
Environment=HOST=127.0.0.1
Environment=PORT=3000
Environment=NITRO_HOST=127.0.0.1
Environment=NITRO_PORT=3000
ExecStart=/usr/bin/node .output/server/index.mjs
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Reload and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now \
  ddag-admin-backend \
  ddag-auth-service \
  ddag-api-gateway \
  ddag-worker \
  ddag-connector-postgres \
  ddag-dashboard
```

Enable `policy-engine`, `cache-service`, and other connectors when your
configuration uses them.

## Caddy Reverse Proxy

Use separate hostnames for control plane and data plane to avoid `/api/*` route
conflicts between the dashboard backend and published dynamic APIs.

`/etc/caddy/Caddyfile`:

```caddyfile
ddag.example.com {
  encode zstd gzip

  reverse_proxy /auth/* 127.0.0.1:8080
  reverse_proxy /api/* 127.0.0.1:8080

  reverse_proxy /oauth/* 127.0.0.1:8081
  reverse_proxy /.well-known/* 127.0.0.1:8081

  reverse_proxy 127.0.0.1:3000
}

api.ddag.example.com {
  encode zstd gzip
  reverse_proxy 127.0.0.1:8082
}
```

Then reload Caddy:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## Smoke Test

Check local readiness:

```bash
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8081/readyz
curl -fsS http://127.0.0.1:8082/readyz
curl -fsS http://127.0.0.1:8090/readyz
```

Check public routing:

```bash
curl -I https://ddag.example.com
curl -fsS https://ddag.example.com/.well-known/jwks.json
curl -fsS https://api.ddag.example.com/healthz
```

After creating a client and publishing an API, run the load-test scripts in
[docs/TESTING_V3.md](TESTING_V3.md) against the public gateway base URL.

## Backup

Back up these items before every upgrade and on a schedule:

- Metadata PostgreSQL database:

  ```bash
  pg_dump "$DDAG_DB_NAME" > "ddag-$(date +%F).sql"
  ```

- `/etc/ddag/ddag.env`, stored encrypted.
- The current Git SHA or release version deployed under `/opt/ddag`.

Redis is used for cache and rate-limit counters. It can be rebuilt, though Redis
persistence can reduce disruption during restarts.

## Upgrade and Rollback

Upgrade:

```bash
cd /opt/ddag
sudo -u ddag git fetch --tags
sudo -u ddag git checkout <release-tag-or-commit>
sudo -u ddag make build
cd apps/dashboard
sudo -u ddag npm ci
sudo -u ddag npm run build
cd /opt/ddag
set -a
. /etc/ddag/ddag.env
set +a
sudo -u ddag ./bin/ddag-migrate --seed
sudo systemctl restart \
  ddag-admin-backend \
  ddag-auth-service \
  ddag-api-gateway \
  ddag-worker \
  ddag-connector-postgres \
  ddag-dashboard
```

Rollback:

```bash
cd /opt/ddag
sudo -u ddag git checkout <previous-known-good-sha>
sudo -u ddag make build
cd apps/dashboard
sudo -u ddag npm ci
sudo -u ddag npm run build
sudo systemctl restart \
  ddag-admin-backend \
  ddag-auth-service \
  ddag-api-gateway \
  ddag-worker \
  ddag-connector-postgres \
  ddag-dashboard
```

Database migrations are forward-only. Take a metadata DB backup before upgrades
so you can restore the previous database state if a rollback requires it.

## Operational Checks

- `journalctl -u ddag-api-gateway -f` for gateway errors.
- `systemctl list-units 'ddag-*'` for service health.
- `curl http://127.0.0.1:8082/metrics` for Prometheus metrics.
- Dashboard Monitoring page for circuit breaker state, connector errors, cache
  hit ratio, queue depth, and source DB pool pressure.

If `api.ddag.example.com` returns `503 DB_POOL_EXHAUSTED`, tune the source
connection pool or gateway backpressure before increasing connector replicas.
