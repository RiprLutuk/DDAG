# DDAG v2.0 Task Tracker

Source PRD: `PRD_DDAG_v2.md`

## Scope

Tracker ini dipakai selama update PRD v2.0. File ini boleh dihapus setelah seluruh pekerjaan selesai dan catatan sudah dipindahkan ke changelog/release notes.

## Checklist

- [x] Review PRD v2.0 dan bandingkan dengan implementasi existing.
- [x] Tambahkan validasi konfigurasi produksi fail-fast.
- [x] Tambahkan trusted proxy handling untuk `X-Forwarded-For`.
- [x] Tambahkan rate limiter fail-mode `open|closed`.
- [x] Tambahkan singleflight anti-cache stampede pada cache miss gateway.
- [x] Tambahkan pagination pushdown (`limit` + `offset`) ke kontrak gateway-connector dan driver SQL.
- [x] Tambahkan endpoint OpenAPI/catalog dari metadata API.
- [x] Tambahkan service-to-service auth untuk connector endpoints.
- [x] Tambahkan audience/issuer/clock-skew validation untuk access token.
- [x] Evaluasi circuit breaker per connection untuk connector.
- [x] Tambahkan metadata cache penuh untuk client/API access/IP/rate-limit + Redis Pub/Sub sync.
- [x] Tambahkan Helm chart deployment profiles.
- [x] Update README/ops documentation untuk konfigurasi v2.0.
- [x] Jalankan unit test dan build checks.

## Gap Audit After Review

- [x] Tambahkan metric v2.0: `ddag_singleflight_active`, `ddag_singleflight_shared`, `ddag_metadata_sync_total`, `ddag_circuit_state`, `ddag_circuit_open_total`, `ddag_circuit_half_open_total`.
- [x] Expose circuit breaker state per connection ke admin-backend/dashboard.
- [x] Tambahkan gateway-side circuit state cache agar gateway bisa fail-fast sebelum memanggil connector; circuit breaker tetap authoritative di connector.
- [x] Tambahkan `meta.circuit_state` pada response success/list sesuai PRD response format.
- [x] Lengkapi production hardening: fatal untuk default `DDAG_SUPERADMIN_PASSWORD`, warning untuk wildcard/localhost dashboard origins, warning untuk metadata DB `sslmode=disable`, dan CSRF protection dashboard.
- [x] Lengkapi OpenAPI export `.yaml`.
- [x] Tambahkan CI pipeline v2.0: gofmt, build, vet, `golangci-lint`, `gosec`, `govulncheck`, unit test, integration test dengan service containers, `npm audit`, dashboard build, Helm render, Docker build, dan Trivy scan.
- [x] Tambahkan integration tests awal ber-tag `integration` untuk Redis rate limit, Redis cache/purge, dan PostgreSQL connector pagination pushdown.
- [x] Jalankan verifikasi lokal yang tersedia: `go test ./...`, `go test -tags=integration -count=1 ./...`, `go vet ./...`, `npm run build`, `npm audit --audit-level=moderate`, `git diff --check`.
- [x] Redis integration lokal untuk cache dan rate limiter pass; PostgreSQL connector integration test tersedia tetapi skip lokal karena tidak ada Postgres di `127.0.0.1:5432` (CI menyediakannya sebagai service container).
- [ ] Verifikasi lokal `helm template`, Docker build, Trivy, `gosec`, `govulncheck`, dan `golangci-lint` belum bisa dijalankan di environment ini karena binary/tooling lokal tidak tersedia; semuanya sudah dipasang di GitHub Actions CI.
- [ ] Integration tests yang masih butuh stack penuh/testcontainers: auth token lifecycle end-to-end, gateway full request flow, connector MySQL, metadata Pub/Sub reload terhadap metadata DB, dan circuit breaker trip/recovery lintas service.

## Notes

- MVP v1.0 sudah banyak tersedia: dashboard, auth, RBAC, connector, cache, rate limit, audit, metrics, dan Kubernetes manifests.
- Implementasi ini menyelesaikan hardening backend lokal yang bisa diuji dengan unit test, plus integration tests ber-tag untuk Redis/PostgreSQL yang berjalan ketika service target tersedia.
- `helm template` belum bisa diverifikasi lokal karena binary `helm` tidak tersedia, tetapi CI sudah menjalankan lint/render untuk seluruh profile chart.
