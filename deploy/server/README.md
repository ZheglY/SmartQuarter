# Server deployment

Runbook: `../../docs/deployment/server-guide.md`; PDF: `../../output/pdf/SmartQuarter_Server_Deployment_Guide.pdf`.

Target: Ubuntu 24.04 + Docker Compose + Caddy HTTPS + external Yandex Object Storage. Public ports: only 80/443. Three separate PostgreSQL volumes and a persistent Redis are private to the Compose network.

Copy `.env.example` to `.env` and supply real values. Database passwords should be independently generated hexadecimal strings; arbitrary passwords need URI escaping in DATABASE_URL. Do not commit `.env` or run `docker compose config` without `--quiet` in shared logs.

Gateway connects to the shared Identity gRPC contract and requires Identity, Issue,
Community and Redis readiness. Configure platform administrators via `ADMIN_USER_IDS`,
then assign chairmen in the Mini App admin screen; see section 9 of the runbook.
A user without active memberships can log in but cannot access house data.

`issue-migrate` uses the service's versioned migration runner. `community-migrate`
applies idempotent SQL 000001, contacts migration 000002 and community integrity 000003. Identity runs
embedded Goose migrations on start. Before upgrading, check duplicate normalized
addresses/chairmen and configure ADMIN_USER_IDS using the
[house workflow runbook](../../docs/house-workflow/README.md).

The older `../docker-compose.yml` is not the server recipe and is not included by this file.
# Рабочая конфигурация сервера

Запускайте этот стек из `deploy/server` либо явно задавайте оба пути:
`docker compose --env-file deploy/server/.env -f deploy/server/compose.yaml ...`.
Домен обслуживает проект `smartquarter-server`; корневой `.env` и
`deploy/docker-compose.yml` относятся к другой конфигурации.

Gateway использует CA-бандл Ubuntu, подключённый только для чтения через
`MAX_CA_BUNDLE` (по умолчанию `/etc/ssl/certs/ca-certificates.crt`). Установите
необходимые сертификаты MAX из [официального источника](https://www.gosuslugi.ru/landing/crt)
в доверенное хранилище хоста и выполните `sudo update-ca-certificates`.
После обновления бандла пересоздайте Gateway, чтобы он открыл обновлённый файл:
`docker compose up -d --no-deps --force-recreate max-gateway`.
TLS-проверка остаётся включённой; отсутствие файла бандла блокирует запуск.
