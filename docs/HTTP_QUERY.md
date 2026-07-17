# HTTP QUERY compatibility

DDAG supports the HTTP `QUERY` method defined by [RFC 10008](https://www.rfc-editor.org/rfc/rfc10008.html). `QUERY` is safe and idempotent and may carry a request body. DDAG treats it as read-only; it cannot be published with `is_write=true`.

## Compatibility contract

OpenAPI 3.0 does not define a `query` Path Item operation. DDAG therefore emits a valid `post` operation with the original method preserved as an extension:

```yaml
post:
  summary: Search employees
  x-ddag-http-method: QUERY
  requestBody:
    content:
      application/json:
        schema:
          type: object
```

This representation keeps validators and SDK generators compatible. DDAG Swagger UI reads `x-ddag-http-method`, displays a `QUERY` badge, and changes the actual Execute request from `POST` to `QUERY`.

## Client behavior

Clients with arbitrary-method support should call the native method:

```bash
curl --request QUERY 'https://gateway.example/api/v1/employees/search' \
  --header 'Authorization: Bearer <access-token>' \
  --header 'Content-Type: application/json' \
  --data '{"department":"IT"}'
```

Go supports arbitrary method strings even when the standard library does not expose an `http.MethodQuery` constant:

```go
req, err := http.NewRequestWithContext(ctx, "QUERY", endpoint, body)
```

Generated clients that only understand standard OpenAPI operations will initially generate a `POST` call. Their transport adapter must inspect `x-ddag-http-method: QUERY` and replace the outgoing method with `QUERY`. Do not silently send `POST` to a native QUERY route: DDAG routes methods independently.

## Browser and edge requirements

A browser request triggers CORS preflight. The complete path must allow `QUERY`:

```http
OPTIONS /api/v1/employees/search
Access-Control-Request-Method: QUERY

Access-Control-Allow-Methods: GET, QUERY, POST, PUT, PATCH, DELETE, OPTIONS
```

Verify every intermediary: browser/client → CDN/WAF → Caddy/load balancer → DDAG gateway. A proxy that accepts an arbitrary method is not evidence that its WAF or method allowlist permits it.

## Policy, cache, and audit semantics

- `QUERY` is always read-only (`is_write=false`).
- Authentication, scopes, API grants, mandatory IP whitelisting, rate limits, and connector safety checks apply normally.
- Request logs record the actual method as `QUERY`.
- Cache behavior must use the method and normalized request body in its key; do not assume GET-style URI-only caching.
- Safe/idempotent semantics permit retries, but clients should only retry after transport failures and within their timeout budget.

## Verification

```bash
# Preflight
curl -i --request OPTIONS 'https://gateway.example/api/v1/employees/search' \
  -H 'Origin: https://app.example' \
  -H 'Access-Control-Request-Method: QUERY'

# Native request
curl -i --request QUERY 'https://gateway.example/api/v1/employees/search' \
  -H 'Authorization: Bearer <access-token>' \
  -H 'Content-Type: application/json' \
  --data '{"department":"IT"}'
```

Expected unauthenticated native requests return DDAG JSON errors, not HTML. A `404 API_NOT_FOUND` still proves the edge and gateway accepted the method when no matching route is published; successful execution additionally requires a published QUERY route, client API grant, scope, and IP whitelist.
