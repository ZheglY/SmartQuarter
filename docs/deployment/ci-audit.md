# CI audit - 22 September 2026

Base: main `9c4e70d6f49efb00210668423e13f1de143de62f`.
Historical audit of the deployment repair. Current checks are defined in `.github/workflows/ci.yml`.

## Confirmed failure

[GitHub Actions run 35665544780](https://github.com/ZheglY/SmartQuarter/actions/runs/35665544780),
job `106550372239`, failed in `Format, vet and test Go modules`.
The first error is missing go.mod checksum entries in Community's go.sum
(multierr, x/net, genproto/googleapis/rpc, x/text, procfs).
The workflow runs with GOWORK=off, so workspace sums do not repair standalone modules.

Further local checks revealed literal unresolved merge markers in Identity main and
HTTP router, stale Identity health tests, nonconstant status.Errorf arguments in
Community, missing sums in Issue/Gateway and unformatted Issue main.

## Repairs

- Restore Identity's app.Run entrypoint and current NewRouter, with liveness,
  database readiness and unknown-route tests.
- Replace Community's dynamic status.Errorf strings with status.Error.
- Run standalone go mod tidy; only go.sum changed, no dependency version upgrades.
- Format Issue main and align Identity's Docker build with Go 1.26.5.
- Expand CI to four independent Go jobs, frontend typecheck/lint/unit/build/browser
  checks, five Docker builds, full application acceptance and an aggregate CI gate. Add read-only permissions,
  cancellation of superseded runs and job timeouts. Keep action versions SHA-pinned.

## Local verification

Passed on this Windows checkout with Go 1.26.5:

- All four services: gofmt, GOWORK=off / GOFLAGS=-mod=readonly,
  go mod tidy -diff, go mod verify, go vet ./..., go test ./....
- Identity and Community: CGO_ENABLED=0 Linux cross-builds.
- Mini App: typecheck, lint, 37 unit tests, production build.
- Playwright: all 14 scenarios passed in mobile Chromium and WebKit.
- actionlint v1.7.7: workflow validation passed.
- Server Compose: config --quiet passed with synthetic required values;
  missing required values are rejected. No real credentials used in validation.
- git diff --check: passed.

Integration verification (22 September 2026):

- Gateway uses the canonical Identity protobuf and the real Identity service.
  Redis sessions, fresh membership checks, active default-house selection,
  standard gRPC health checks and Community readiness are connected.
- The isolated Compose acceptance test passed all five Issue workflows using
  real Identity, Issue, Community, PostgreSQL, Redis and MinIO. Upload/complete,
  confirmation, statements, status changes and notification delivery passed.
  MAX Bot API is a local HTTP test receiver, not the live platform.
- Additional acceptance checks passed for users without memberships, revoked
  access on existing sessions, house isolation, chairman-only announcements,
  dependency failures and logout. The first real-service run exposed an empty
  active-house request returning 400; Gateway now correctly returns 403.
- The real Identity operator command passed house/user creation, repeated role
  updates without duplicate memberships or profile loss, revocation and invalid
  role rejection. CI runs deploy/test/check-provision.sh after acceptance.
- Identity, Issue and Community images built and started healthy. The dedicated
  acceptance image also built and passed the test as a non-root scratch image.
- Latest Identity/Gateway unit tests, vet and tidy-diff checks passed; Gateway and
  the acceptance binary also cross-compiled for Linux with CGO disabled.
- All five production Docker images built successfully. The production Gateway
  image passed /readyz=200 against real dependencies, signed synthetic MAX login,
  /me, membership denial (403), logout (204) and session invalidation (401).
  Its /app healthcheck command also passed. No live MAX requests were sent.
- Mini App image served /healthz, the root and a nested SPA route with HTTP 200.
  The server Caddyfile passed validation in the actual Caddy container.

Limitations:

- No VPS address/access was supplied, so no server deployment was performed.
- Local S3 acceptance uses MinIO; Yandex cloud credentials, external CORS/TLS and
  actual MAX Android/iOS authentication, webhook registration and delivery remain
  deployment acceptance steps.
- Repairs are uncommitted and unpublished. The existing failed GitHub run remains
  failed; a new PR run must pass after publication before calling CI green.
- This repository has CI only. No registry publication or automated CD existed;
  none was added implicitly.

The server runbook and PDF cover role provisioning, S3, migrations, HTTPS,
MAX webhook registration, acceptance, backup, update and rollback.
