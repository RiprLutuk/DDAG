## Summary

Describe what changed and why.

## Validation

- [ ] `gofmt -l .` prints nothing
- [ ] `go vet ./...`
- [ ] `go test ./... -count=1`
- [ ] Dashboard changes: `pnpm --dir apps/dashboard build`
- [ ] Deployment/config changes: relevant Compose or Caddy config was validated

## Security and compatibility

- [ ] No credentials, tokens, raw SQL errors, or sensitive data were added
- [ ] Authorization is enforced server-side for new endpoints
- [ ] Bound parameters are used for database input
- [ ] API/OpenAPI compatibility and RFC 10008 QUERY behavior were considered

## Screenshots / migration notes

Add screenshots for UI changes and deployment or migration notes when applicable.
