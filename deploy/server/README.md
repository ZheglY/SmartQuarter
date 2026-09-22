# Server deployment

Runbook: `../../docs/deployment/server-guide.md`; PDF: `../../output/pdf/SmartQuarter_Server_Deployment_Guide.pdf`.

Target: Ubuntu 24.04 + Docker Compose + Caddy HTTPS + external Yandex Object Storage. Public ports: only 80/443. Three separate PostgreSQL volumes and a persistent Redis are private to the Compose network.

Copy `.env.example` to `.env` and supply real values. Database passwords should be independently generated hexadecimal strings; arbitrary passwords need URI escaping in DATABASE_URL. Do not commit `.env` or run `docker compose config` without `--quiet` in shared logs.

Gateway connects to the shared Identity gRPC contract and requires Identity, Issue,
Community and Redis readiness. Provision verified users and house memberships via
`docker compose exec identity-service /app provision`; see section 9 of the runbook.
A user without active memberships can log in but cannot access house data.

`issue-migrate` uses the service's versioned migration runner. `community-migrate` applies the current idempotent initial SQL, not a versioned migration system. Identity runs embedded Goose migrations on start. Read the update and backup sections before deploying future schema changes.

The older `../docker-compose.yml` is not the server recipe and is not included by this file.
