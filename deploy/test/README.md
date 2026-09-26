# Isolated application acceptance

From repository root: `docker compose -f deploy/test/compose.yaml build`, then
`docker compose -f deploy/test/compose.yaml run --rm test`.
The exit code is the test result. Diagnostics: `docker compose -f deploy/test/compose.yaml logs`.
After a successful run, `bash deploy/test/check-provision.sh` verifies operator onboarding,
repeat updates without duplicate memberships or profile loss, and access revocation.
Cleanup: `docker compose -f deploy/test/compose.yaml down -v` removes this test project only.
Do not substitute the production Compose file in this cleanup command.

All database names end in _test. Credentials are synthetic, network ports are private,
and no real MAX messages are sent. Identity is the real service; the old test-only
protocol is retained solely for legacy isolated notification tests. Unit and browser
fixture tests run separately. This stack covers house access, issues, contacts and community participation.
Official government submission is performed outside the application.

`TestHouseWorkflowHTTP` additionally covers registration/platform approval, contacts
CRUD/archive, search/join approval, bounded invitations, chairman transfer, stale
authority denial (including cached replies), preferences, house isolation, duplicate
events and actual Identity outbox publication. The fixed administrator UUID in this
Compose is test-only. Run against a fresh stack; Issue tests assert exact fixture counts.

On low-memory machines build each service sequentially (identity-service,
issue-service, issue-migrate, community-service, minio, storage-init, test).

The S3 test server and client are built from the official MinIO sources because
the historical Quay images no longer pull anonymously. `storage/Dockerfile` pins
the source commits for MinIO `RELEASE.2025-09-07T16-13-09Z` and mc
`RELEASE.2025-08-13T08-35-41Z`; the binaries and upstream licenses are copied into
the runtime images. The production stack continues to use the configured external S3.
