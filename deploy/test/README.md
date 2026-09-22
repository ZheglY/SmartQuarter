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
fixture tests run separately. This stack covers the implemented MVP, not deferred
polls/calendar/initiatives or manual government submission.

On low-memory machines build each service sequentially (identity-service,
issue-service, issue-migrate, community-service, test).
