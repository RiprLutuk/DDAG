#!/bin/bash
# Smoke test for DDAG v3 observability metrics (PRD §14)
# Hits key endpoints to ensure metrics are registered and visible in Prometheus

set -e

BASE_URL="${1:-http://localhost:8082}"
echo "==> DDAG Metrics Smoke Test"
echo "==> Target: $BASE_URL"

# Health check
echo -n "Testing /healthz... "
curl -sf "$BASE_URL/healthz" > /dev/null && echo "✓" || echo "✗ FAILED"

# Metrics endpoint
echo -n "Testing /metrics... "
curl -sf "$BASE_URL/metrics" | grep -q "ddag_http_requests_total" && echo "✓" || echo "✗ FAILED"

# Check key metrics exist
echo ""
echo "==> Checking metric registration:"
METRICS=$(curl -sf "$BASE_URL/metrics")

for metric in \
  "ddag_http_requests_total" \
  "ddag_http_request_duration_seconds" \
  "ddag_cache_hits_total" \
  "ddag_cache_misses_total" \
  "ddag_singleflight_shared_total" \
  "ddag_queued_requests_total" \
  "ddag_queue_depth" \
  "ddag_rejected_requests_total"
do
  echo -n "  $metric... "
  echo "$METRICS" | grep -q "^$metric" && echo "✓" || echo "✗ MISSING"
done

echo ""
echo "==> Smoke test complete"
