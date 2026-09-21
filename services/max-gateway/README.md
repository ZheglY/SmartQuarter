# max-gateway

Go HTTP/JSON gateway для Mini App «Умный Квартал». Ветка `feat/max-gateway-service`, база `c1aa8a6973c3e3c8a59494745d907d9ca88416f4`.

Реализованы REST → все 10 Issue RPC, Redis-сессии, MAX initData, webhook/Bot API, notification consumer и два маршрута объявлений Community. Основной E2E выполнен 5 раз с настоящим Issue Service, PostgreSQL, Redis и MinIO. В тестах Identity — отдельный test-only gRPC stub, MAX — локальный HTTP endpoint. Это **не** подтверждение реальной интеграции с Identity или платформой MAX.

## Ограничение запуска: Identity

В текущей интеграционной базе нет `identity.proto`, а Identity Service содержит HTTP-каркас. `internal/identity.Client` описывает ожидаемый Gateway port (`UpsertMaxUser`, `GetUserContext`, `GetMembership`, `ListMemberships`, `Ready`). Production использует `identity.Unavailable`: валидный MAX login возвращает 503, `/readyz` — 503. ENV `IDENTITY_GRPC_ADDR` зарезервирован и сам по себе не включает интеграцию. Пользователи и роли не создаются локально. Consumer не читает новые события, пока Identity не готов.

Другому разработчику необходимо согласовать wire contract этих методов, формат UserContext/membership и service authentication. После этого нужен Gateway gRPC adapter и проверка с настоящим Identity. Нельзя просто заменить test stub production-реализацией без согласованного контракта.

`tests/testdata/identity.go` имеет build tag `integration`, использует отдельный namespace `gateway.test.Identity` и фиксированные UUID. Его протокол — только тестовый, не предложенный wire contract Identity. Он не входит в production binary и не предоставляет публичный HTTP login bypass.

## Архитектура

```text
Mini App --REST/JSON--> HTTP middleware/handlers --gRPC--> Issue Service
    |                        |                               |
    +--signed PUT/GET--> MinIO / Yandex S3             PostgreSQL + outbox
                             |                               |
                        Redis sessions                Redis Streams
                                                             |
                      Identity recipient lookup <-- notification consumer
                                                             |
                                                       MAX Bot API
```

`internal/config` — ENV и проверка конфигурации; `identity` — доверенный порт; `state` — Redis sessions/rate/idempotency; `rpc` — metadata/deadlines/metrics; `transport/http` — middleware, REST DTO, отдельные mappers; `maxapi` — HMAC и клиент платформы; `notification` — Streams consumer; `observability` — собственный Prometheus registry и zap.

Бизнес-логика Issue, PostgreSQL-транзакции, миграции, объектное хранилище и outbox остаются в существующем Issue Service. Межсервисных SQL-запросов в production Gateway нет. SQL в E2E используется только для проверки outbox.

## REST → gRPC

Все команды требуют `Content-Type: application/json`, мутации — `Origin` из `TRUSTED_ORIGINS`. Если тело пустое, передайте `{}`. UUID дома и пользователя, роль и адрес не принимаются из тела или HTTP actor headers. Membership проверяется заново на каждом бизнес-запросе.

| REST | RPC / действие | Доступ |
|---|---|---|
| POST /api/v1/session/max | MAX validation → Identity.UpsertMaxUser/GetUserContext/GetMembership | Bootstrap |
| GET /api/v1/me | Identity.GetUserContext | Session |
| POST /api/v1/session/active-house | Identity.GetMembership | ACTIVE membership |
| POST /api/v1/session/logout | Redis session revoke | Session |
| POST /api/v1/uploads | Issue.CreateUpload | ACTIVE member |
| POST /api/v1/uploads/{id}/complete | Issue.CompleteUpload | Автор / тот же дом |
| GET /api/v1/attachments/{id}/download-url | Issue.GetAttachmentDownloadURL | Доступ к вложению |
| POST /api/v1/issues | Issue.CreateIssue | ACTIVE member |
| GET /api/v1/issues | Issue.ListIssues | ACTIVE member |
| GET /api/v1/issues/{id} | Issue.GetIssue | ACTIVE member |
| POST /api/v1/issues/{id}/confirm | Issue.ConfirmIssue | Житель, не автор |
| PATCH /api/v1/issues/{id}/status | Issue.UpdateIssueStatus | CHAIRMAN / ADMIN |
| POST /api/v1/issues/{id}/statement | Issue.GenerateStatement | CHAIRMAN / ADMIN |
| GET /api/v1/issues/{id}/statement | Issue.GetStatement | CHAIRMAN / ADMIN |
| GET /api/v1/chairman/issues | Issue.ListIssues(chairman_queue=true) | CHAIRMAN / ADMIN |
| POST /api/v1/announcements | Community.CreateAnnouncement | CHAIRMAN / ADMIN |
| GET /api/v1/announcements | Community.ListAnnouncements | ACTIVE member |
| POST /webhooks/max | MAX Update | Webhook secret |
| GET /livez, /healthz | Process liveness | Technical |
| GET /readyz | Redis + Identity + Issue gRPC + Issue readiness (DB/S3) | Technical |
| GET /metrics | Prometheus | Technical |

Успешное создание возвращает 201, logout — 204, остальные операции — 200. `status` PATCH называется `new_status`; session bootstrap использует `init_data`. Enum в REST короткий (`DETECTED`), nullable timestamps/statement — `null`; вложенные JSON snapshots — объекты. MAX ID — десятичная строка. `page_size` 1–100, default 20; `page_token` непрозрачный. `status` — повторяемый или разделённый запятыми фильтр.

Canonical [OpenAPI](../../contracts/openapi/openapi.yaml) описывает реализованные маршруты. Генерация: `python scripts/openapi.py` (Python 3, без сторонних зависимостей). Poll/calendar/initiative REST не реализованы и не объявлены работающими.

## Общий Issue protobuf

Единственный источник: `contracts/proto/smartquarter/issue/v1/issue.proto` в корне репозитория. Файл перенесён **без изменения содержимого**, включая package, RPC, номера полей, enum values и go_package. Go mapping при генерации даёт Gateway собственный `internal/gen/smartquarter/issue/v1`; Issue продолжает использовать свой `internal/gen`. Нет импорта чужого internal, нового общего Go module или изменений go.work. Community client генерируется из существующего canonical Community proto с mapping только в Gateway.

Из корня репозитория:

```powershell
./services/max-gateway/scripts/generate.ps1
```

Нужны Go и Buf. Plugins закреплены в `buf.gen.yaml`: protoc-gen-go v1.36.12, protoc-gen-go-grpc v1.6.1. Для Issue отдельно из его каталога: `buf generate ../../contracts/proto --path ../../contracts/proto/smartquarter/issue/v1/issue.proto`.

## Запуск и тесты

Нужны Go 1.26+, Docker Desktop/Linux containers, PowerShell 7. Порты тестовой инфраструктуры: PostgreSQL 15432, Redis 16379, MinIO 19000/19001, Issue 18082/18083. Gateway Compose использует 18080, чтобы не занимать обычный frontend-порт 8080.

Из корня:

```powershell
./services/max-gateway/scripts/test.ps1
```

Скрипт поднимает выделенный `smartquarter-issue-local` через существующий Issue script, создаёт тестовый bucket, применяет миграции, выполняет unit/Redis/notification/E2E тесты. Пароли генерируются существующим Issue script и лежат в игнорируемом `services/issue-service/.env.local`. E2E запускает Gateway HTTP server и test-only Identity gRPC внутри Go test; Issue работает отдельным настоящим контейнером. На этапе проверки отказа E2E кратко приостанавливает **только** `smartquarter-issue-local-postgres-1`, затем обязательно возобновляет его. Стек после тестов остаётся доступным; остановка: `./services/issue-service/scripts/local.ps1 stop`.

Проверки модуля:

```powershell
cd services/max-gateway
$env:GOWORK = 'off'
go test ./...
go vet ./...
go build ./...
docker build -t smartquarter-max-gateway:local .
```

Ручной запуск (полный пользовательский вход пока блокирован Identity): скопировать `.env.example` в `.env.local`, заполнить MAX настройки и разрешённый Origin, затем `./scripts/run.ps1`. По умолчанию native HTTP `:8080`; если занят, задать `HTTP_ADDR=:18080`. После запуска Issue test stack можно использовать `docker compose -f compose.local.yml up -d --build`: файл подключает Gateway к сети `smartquarter-issue-local_default`. В local Compose нет fake Identity/Community.

Общий `deploy/docker-compose.yml` сохранён: его независимая конфигурация Identity/Community не менялась. Для Gateway/Issue использовать описанный локальный контур.

Полный ENV с defaults в `.env.example`. Обязательны `ISSUE_GRPC_ADDR`, `ISSUE_READY_URL`, `MAX_BOT_TOKEN`, `MAX_WEBHOOK_SECRET`, `MAX_BOT_USERNAME`, `MAX_MINIAPP_URL`, непустой `TRUSTED_ORIGINS`. `COMMUNITY_GRPC_ADDR` необязателен, отсутствие даёт 503 на объявлениях. HTTP timeouts, Redis DB/password, session TTL, request/dial timeout, rate limits, stream/group и shutdown timeout задаются ENV. `APP_ENV=local|test` использует SameSite=Lax без Secure; остальные значения — Secure + SameSite=None и HTTPS Origins. Go сам `.env` не читает.

## MAX

Сверено 21.09.2026 с официальными источниками:

- [Проверка initData](https://dev.max.ru/docs/webapps/validation): HMAC-SHA256 derivation через WebAppData, URL decoding, сортировка, constant-time comparison, ограничение возраста (default 5m) и future skew 30s. Дубли полей отклоняются.
- [Webhook subscription](https://dev.max.ru/docs-api/methods/POST/subscriptions): проверяется `X-Max-Bot-Api-Secret`, не вымышленная подпись и не внутренний project envelope.
- [Отправка сообщений](https://dev.max.ru/docs-api/methods/POST/messages): `Authorization` header, `user_id` query, default `https://platform-api2.max.ru`; до 2 сообщений/сек на получателя.
- [Кнопки](https://dev.max.ru/docs-api/use-cases/sending-messages/keyboard) и [официальный SDK](https://github.com/max-messenger/max-bot-api-client-ts/pull/222/files): `open_app.web_app` содержит username бота. `MAX_MINIAPP_URL` — URL, который нужно зарегистрировать у MAX; это не значение `web_app`.

Поддержаны `bot_started`, `/start` из `message_created`, безопасный ответ на callback; неизвестные update types подтверждаются без бизнес-действий. Webhook dedup по canonical JSON hash на 7 дней; lease не даёт параллельно отправить обычный дубль. Числа при canonicalization сохраняются без float64. Клиент не перенаправляет Authorization на другой URL. HTTP 429 — до трёх попыток с ограниченным Retry-After; transport/5xx ошибки не повторяются немедленно из-за неоднозначного результата отправки.

Реальные credentials, регистрация HTTPS webhook, сертификаты платформы, запуск Mini App в MAX и поведение third-party cookies **NOT VERIFIED**. Проверка TLS включена; для требуемого платформой CA используйте доверенный CA bundle, не отключайте verification.

## Уведомления и надёжность

Consumer group читает `stream:notifications`, валидирует полный Issue envelope и обрабатывает `issue.created`, `issue.confirmed`, `issue.status_changed`, `statement.generated`. Подтверждённый получатель — автор Issue: `payload.created_by` → Identity UserContext → MAX ID, плюс актуальный membership дома. Рассылка председателям/всем жителям требует согласованного recipient API и пока не реализована. Community events не объявлены интегрированными.

После успешной доставки Redis атомарно сохраняет event dedup и XACK. После ошибки событие остаётся pending; XAUTOCLAIM через 30s восстанавливает зависшие записи. После 10 попыток или невалидного envelope — атомарный перенос в `<stream>:dead` и XACK. DLQ содержит исходное событие и причину: разбирать вручную после исправления зависимости, перепубликовывая исправленное событие. Не удаляйте поток ошибок без разбора. Dedup retention — 7 дней; очень старые redelivery и сбой между MAX success и Redis commit могут дать дубликат. Строгая exactly-once доставка не обещается.

Idempotency: ключ UUID опционален для создания Issue/statement/announcement. Redis SET NX связывает ключ с user+house и hash method/path/compact JSON; порядок полей JSON значим. Успешный ответ хранится 24h. Другой payload → 409 `IDEMPOTENCY_KEY_REUSED`. Pending или неоднозначный результат → 409 `IDEMPOTENCY_IN_PROGRESS`; lease не удаляется для автоматического повтора команды. Сначала сверить состояние ресурса, затем решать о новом ключе. После 24h, потери Redis или его eviction повтор может создать новую команду: production Redis требует persistence/no-eviction. Redis и PostgreSQL не имеют общей транзакции.

HTTP errors содержат `{error:{code,message,request_id}}`. Issue v1 передаёт только gRPC status без ErrorInfo, поэтому используются стабильные общие коды, например `RESOURCE_NOT_FOUND`, без анализа текста ошибки. HTTP `408/504` соответствуют canceled/deadline. Origin обязателен для мутаций; proxy headers не используются как доверенный IP/actor. Оригиналы фото идут напрямую в S3 и передают все `required_headers`, включая `If-None-Match: *`.

gRPC рассчитан на приватную доверенную сеть MVP; внешний production требует service identity/mTLS. Ingress должен закрывать `/metrics` и внутренние сервисы. Логи не содержат токены, initData, cookies, signed URLs, тела фото/заявлений; labels метрик ограничены маршрутами/кодами, без user/request IDs.

## Community и оставшиеся работы

Announcement transport проверен на согласованном proto с fake gRPC server. Настоящий Community runtime **NOT VERIFIED**: у текущего сервиса проверки membership/role и отображение domain errors требуют отдельной проверки командой владельцев. Gateway проверяет membership/role перед вызовом. Никакие файлы Identity/Community или их proto не менялись. Необходимо провести совместный runtime E2E после готовности Identity, согласовать service authentication, затем добавить optional Community REST при необходимости.

Подробные результаты проверок и список файлов: [IMPLEMENTATION_REPORT.md](IMPLEMENTATION_REPORT.md).
