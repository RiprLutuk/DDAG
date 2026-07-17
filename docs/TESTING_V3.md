# DDAG v3 Testing Toolkit

## OAuth2 token

Use the auth service directly only from the DDAG host. In the deployed VPS
profile, the public Caddy route is `https://ddag.example.com/oauth/token`;
connectors and other private service ports must not be exposed for testing.

```bash
AUTH_URL="${AUTH_URL:-http://127.0.0.1:8081}"
CLIENT_ID="${CLIENT_ID:-app-brim}"
read -rsp 'Client secret: ' CLIENT_SECRET; echo
export CLIENT_ID CLIENT_SECRET

TOKEN=$(curl --fail-with-body --silent --show-error \
  -X POST "$AUTH_URL/oauth/token" \
  -H 'Content-Type: application/json' \
  --data "$(python3 -c 'import json,os; print(json.dumps({"client_id":os.environ["CLIENT_ID"],"client_secret":os.environ["CLIENT_SECRET"],"grant_type":"client_credentials"}))" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')

[ -n "$TOKEN" ] && echo 'OAuth token acquired'
```

> Never paste a client secret or bearer token into shell history, source files,
> issue reports, or load-test output. Export `CLIENT_SECRET` from a secret store
> instead of using `read` in unattended CI.

## Python Load Test

```bash
python3 tools/loadtest/export_endpoints.py \
  --gateway-url http://localhost:8082 \
  --token "$TOKEN" \
  --out ddag-endpoints.json
```

```bash
python3 tools/loadtest/ddag_load.py \
  --base-url http://localhost:8082 \
  --token "$TOKEN" \
  --endpoints ddag-endpoints.json \
  --profile low \
  --out ddag-load-result.json
```

Profiles: `low` = 5 VU, `medium` = 15 VU, `high` = 30 VU.

## k6

```bash
ENDPOINTS="$(cat tools/loadtest/endpoints.example.json)" \
TOKEN="$TOKEN" \
BASE_URL=http://localhost:8082 \
PROFILE=medium \
k6 run tools/loadtest/k6-ddag.js
```

## Markdown Report

```bash
python3 tools/loadtest/report.py ddag-load-result.json --out ddag-load-report.md
```
