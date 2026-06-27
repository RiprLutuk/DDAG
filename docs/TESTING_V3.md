# DDAG v3 Testing Toolkit

## Token

```bash
TOKEN=$(curl -s http://localhost:8081/oauth/token \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"app-brim","client_secret":"demo-secret-brim-001","grant_type":"client_credentials"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')
```

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
