# Отчёт реализации max-gateway

Дата проверки: 21.09.2026. Ветка: `feat/max-gateway-service`. Базовый и текущий HEAD: `c1aa8a6973c3e3c8a59494745d907d9ca88416f4` (интеграционный main с Issue Service, сверено с origin/main после fetch). Новый commit не создавался; изменения находятся в рабочем дереве. Push/merge не выполнялись.

## Результат

Реализован Gateway для всех 10 Issue RPC через 11 REST-маршрутов, session endpoints, MAX HMAC validation, webhook/Bot API, Redis sessions/rate limiting/idempotency, notification consumer, announcement transport, health/metrics/shutdown, Docker и воспроизводимые тесты. Всего 22 HTTP-операции в OpenAPI. Полная таблица REST → RPC и архитектура — в [README](README.md#rest--grpc).

Issue Service не переписан: только перенос canonical proto и адаптация инструкции/конфигурации генерации. Прямое сравнение исходного proto из HEAD с новым файлом подтвердило неизменность содержимого (с нормализацией LF/CRLF). Server stubs после Buf generation совпадают с HEAD. Gateway stubs генерируются с Go mapping из того же единственного источника. RPC wire names, field numbers, enum numbers и go_package не изменены.

Identity/Community, их proto, Go-модули, миграции, Dockerfile и ENV **не изменялись**. go.work, go.work.sum и общий deploy/docker-compose.yml также не менялись. Создан отдельный `services/max-gateway/compose.local.yml`, использующий существующий Issue local stack.

## Проверки

| Проверка | Результат / границы |
|---|---|
| gofmt изменённых Go-файлов | PASS |
| Gateway `go test ./...` | PASS |
| Gateway `go vet ./...`, `go vet -tags=integration ./...` | PASS |
| Gateway `go build ./...` | PASS |
| Issue `go test ./...`, `go vet ./...`, `go build ./...` | PASS |
| Shared protobuf regeneration | PASS; закреплены Go plugins 1.36.12 / 1.6.1 |
| Docker build Gateway | PASS; `smartquarter-max-gateway:local` |
| Docker build Issue | PASS; `smartquarter-issue-local-issue-service` |
| Запуск production binary из Gateway image | PASS: `/livez` 200, `/metrics` 200, `/readyz` 503 при отсутствующем Identity; остановка SIGTERM прошла |
| OpenAPI 3.1 schema validation | PASS (`openapi-spec-validator`); сверены все 22 операции с HTTP router |
| Gateway → настоящий Issue E2E | PASS, 5/5 в итоговом запуске `scripts/test.ps1` |
| PostgreSQL и S3/MinIO | Настоящие контейнеры, signed PUT/GET и сохранённый outbox проверены |
| Redis sessions/rate/idempotency | PASS: expiry/logout, отсутствие resurrection, 20 конкурентных reservations → один победитель |
| Notification recovery | PASS: failed MAX send остаётся pending; старый consumer заменён через XAUTOCLAIM; duplicate подавлен; poison envelope перемещён в DLQ |
| MAX webhook/Bot API | PASS с локальным fake endpoint: секрет, bot_started, duplicate, Authorization и 429; live MAX NOT VERIFIED |
| Identity | BLOCKED в production: отсутствует согласованный proto; E2E использует test-only gRPC protocol |
| Community announcements | Client wire/metadata contract PASS с fake gRPC; настоящий runtime NOT VERIFIED |
| Yandex Object Storage | NOT VERIFIED в Gateway E2E; существующая Issue поддержка сохранена, тесты используют MinIO |

Последний полный E2E занял около 16 секунд и выполнил пять независимых цепочек: login с подписанным test initData → upload session → настоящий PUT → READY → create/list/get → второе лицо confirm → chairman queue → generate/get statement → status → outbox → Redis → fake MAX. Проверено 20 уведомлений по Issue и одно webhook-уведомление; повтор webhook и duplicate stream event не увеличили число отправок.

Негативные проверки: чужой дом/Issue/attachment, подмена actor headers/JSON, отсутствие/неверная/истёкшая сессия, CSRF Origin, unknown JSON fields, invalid enum/status, повтор confirm, Resident → statement/status, повтор idempotency key с иным телом, закрытое Redis-соединение, недоступный Issue gRPC, context cancel/deadline. При fault injection выделенный PostgreSQL контейнер был приостановлен; Gateway вернул 503/504, после unpause повторный запрос успешен. Request ID проверен на HTTP и в отдельном gRPC metadata contract test.

Тесты проверяют production Gateway handlers через настоящий HTTP listener, настоящий Issue binary работает отдельным контейнером. Test-only Identity и fake MAX не являются проверкой платформенной интеграции. Отдельный Docker smoke проверяет запуск бинарного Gateway без встраивания тестовых компонентов.

## Запуск и ENV

Из корня: `./services/max-gateway/scripts/test.ps1` — воспроизводимый локальный тест. Скрипт оставляет выделенный Issue stack работающим. Остановка: `./services/issue-service/scripts/local.ps1 stop`. Временный Gateway smoke-контейнер уже остановлен и удалён.

Для ручного Gateway: скопировать `.env.example` в `.env.local`, заполнить обязательные MAX settings/Origin и выполнить `./scripts/run.ps1` из его каталога. Docker: после Issue stack выполнить `docker compose -f compose.local.yml up -d --build`. Подробные команды, полный ENV и значения по умолчанию находятся в [README](README.md#запуск-и-тесты) и [.env.example](.env.example).

Обязательные параметры: `ISSUE_GRPC_ADDR`, `ISSUE_READY_URL`, `MAX_BOT_TOKEN`, `MAX_WEBHOOK_SECRET`, `MAX_BOT_USERNAME`, `MAX_MINIAPP_URL`, `TRUSTED_ORIGINS`. Остальные: APP_ENV/HTTP_ADDR/LOG_LEVEL; REDIS_ADDR/PASSWORD/DB; SESSION_TTL/COOKIE_NAME; MAX_INIT_DATA_TTL/BOT_API_BASE_URL; IDENTITY_GRPC_ADDR (зарезервирован)/COMMUNITY_GRPC_ADDR; GRPC_DIAL_TIMEOUT/REQUEST_TIMEOUT; NOTIFICATION_STREAM/CONSUMER_GROUP; четыре rate limits; HTTP_READ/WRITE/IDLE_TIMEOUT; GRACEFUL_SHUTDOWN_TIMEOUT.

## Незавершённая внешняя интеграция и ограничения

1. Команда Identity должна согласовать реальные protobuf RPC и ответы UserContext/GetMembership, включая проверенную связь внутреннего UUID с MAX ID. После этого нужен production adapter. Сейчас login и readiness честно возвращают 503, consumer ожидает готовности Identity. Fake Identity не включается ENV и не собирается в app.
2. Настоящие MAX credentials, регистрация HTTPS webhook, открытие Mini App, доверенные CA и third-party cookies требуют проверки в MAX. Официальные источники алгоритмов и HTTP-контракта приведены в [README](README.md#max).
3. Команда Community должна проверить runtime authorization/error semantics; Gateway объявления готовы на уровне согласованного wire contract. Optional polls/calendar/initiatives REST пока не реализованы; нет притворных данных.
4. Уведомления направляются только автору Issue через проверенную Identity-связь. Broadcast жителям/председателям требует отдельного recipient contract. Community events не интегрированы.
5. Exactly-once между Redis/PostgreSQL/MAX нет: idempotency reservation живёт 24h, notification dedup 7d. Неоднозначный RPC не повторяется автоматически. После ошибок требуется сверка состояния, DLQ — ручной разбор. Redis persistence/no-eviction и приватная сеть/mTLS перед production — требования эксплуатации.
6. Issue v1 не содержит структурированных ErrorInfo reason codes; HTTP ошибки общие, без разбора текста downstream errors. Protocol не расширялся ради более подробных кодов.

Полное production Definition of Done **не объявляется выполненным** из-за Identity и непроверенной живой MAX/Community интеграции. Реальная связка Gateway → Issue Service и её основной сценарий проверены.

## Полный список изменённых файлов

### Gateway

- `services/max-gateway/.dockerignore` — изменён.
- `services/max-gateway/.env.example` — изменён.
- `services/max-gateway/Dockerfile` — изменён.
- `services/max-gateway/IMPLEMENTATION_REPORT.md` — новый.
- `services/max-gateway/README.md` — изменён.
- `services/max-gateway/buf.gen.yaml` — новый.
- `services/max-gateway/cmd/app/main.go` — изменён.
- `services/max-gateway/compose.local.yml` — новый.
- `services/max-gateway/go.mod` — изменён.
- `services/max-gateway/go.sum` — новый.
- `services/max-gateway/internal/config/config.go` — изменён.
- `services/max-gateway/internal/config/config_test.go` — новый.
- `services/max-gateway/internal/gen/smartquarter/community/v1/community.pb.go` — новый.
- `services/max-gateway/internal/gen/smartquarter/community/v1/community_grpc.pb.go` — новый.
- `services/max-gateway/internal/gen/smartquarter/issue/v1/issue.pb.go` — новый.
- `services/max-gateway/internal/gen/smartquarter/issue/v1/issue_grpc.pb.go` — новый.
- `services/max-gateway/internal/identity/client.go` — новый.
- `services/max-gateway/internal/maxapi/auth.go` — новый.
- `services/max-gateway/internal/maxapi/auth_test.go` — новый.
- `services/max-gateway/internal/maxapi/client.go` — новый.
- `services/max-gateway/internal/notification/consumer.go` — новый.
- `services/max-gateway/internal/notification/consumer_test.go` — новый.
- `services/max-gateway/internal/observability/metrics.go` — новый.
- `services/max-gateway/internal/rpc/client.go` — новый.
- `services/max-gateway/internal/rpc/client_test.go` — новый.
- `services/max-gateway/internal/state/redis.go` — новый.
- `services/max-gateway/internal/state/redis_test.go` — новый.
- `services/max-gateway/internal/transport/http/community.go` — новый.
- `services/max-gateway/internal/transport/http/handler.go` — изменён.
- `services/max-gateway/internal/transport/http/handler_test.go` — изменён.
- `services/max-gateway/internal/transport/http/issues.go` — новый.
- `services/max-gateway/internal/transport/http/mappers.go` — новый.
- `services/max-gateway/internal/transport/http/middleware.go` — новый.
- `services/max-gateway/internal/transport/http/session.go` — новый.
- `services/max-gateway/internal/transport/http/webhook.go` — новый.
- `services/max-gateway/scripts/generate.ps1` — новый.
- `services/max-gateway/scripts/openapi.py` — новый.
- `services/max-gateway/scripts/run.ps1` — новый.
- `services/max-gateway/scripts/test.ps1` — новый.
- `services/max-gateway/tests/integration/e2e_test.go` — новый.
- `services/max-gateway/tests/testdata/identity.go` — новый.

### Issue Service

- `services/issue-service/README.md` — изменён.
- `services/issue-service/buf.yaml` — изменён.
- `services/issue-service/contracts/proto/smartquarter/issue/v1/issue.proto` — удалён при переносе.

### Общие контракты

- `contracts/openapi/openapi.yaml` — изменён.
- `contracts/proto/smartquarter/issue/v1/issue.proto` — новый.

