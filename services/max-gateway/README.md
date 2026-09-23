# max-gateway

Go HTTP/JSON gateway для Mini App «Умный Квартал».

Реализованы REST → все 10 Issue RPC, Redis-сессии, MAX initData, webhook/Bot API, notification consumer и два маршрута объявлений Community. Gateway подключается к настоящим Identity, Issue и Community через общие protobuf-контракты. Стенд автоматической приёмки находится в `deploy/test`; внешний MAX API в нём заменён тестовым HTTP-получателем.

## Identity и доступ к дому

`IDENTITY_GRPC_ADDR` обязателен. `internal/identity.GRPCClient` вызывает `UpsertMaxUser`, `GetUserContext`, `GetMembership`, `ListMemberships` и gRPC Health Check. Источник контракта — `contracts/proto/smartquarter/identity/v1/identity.proto`. Consumer ожидает готовности Identity перед чтением новых событий.

Новый MAX-пользователь получает сессию с пустым списком домов и может подать заявку.
Одобрение регистрации требует `ADMIN_USER_IDS`, одобрение вступления — менеджера
выбранного дома. Привилегии проверяются повторно, в том числе перед replay ответа
Idempotency-Key. Недоступность Identity возвращает 503.
[Маршруты](../../contracts/openapi/openapi.yaml) и [runbook](../../docs/house-workflow/README.md).

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

Бизнес-логика Issue, PostgreSQL-транзакции, миграции, объектное хранилище и outbox остаются в существующем Issue Service. Межсервисных SQL-запросов в production Gateway нет. SQL в E2E используется только с тестовыми БД для подготовки пользователей, проверки отзыва доступа и outbox.

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
| GET /readyz | Redis + Identity/DB + Issue gRPC/DB/S3 + Community gRPC/DB/Redis | Technical |
| GET /metrics | Prometheus | Technical |

Успешное создание возвращает 201, logout — 204, остальные операции — 200. `status` PATCH называется `new_status`; session bootstrap использует `init_data`. Enum в REST короткий (`DETECTED`), nullable timestamps/statement — `null`; вложенные JSON snapshots — объекты. MAX ID — десятичная строка. `page_size` 1–100, default 20; `page_token` непрозрачный. `status` — повторяемый или разделённый запятыми фильтр.

Canonical [OpenAPI](../../contracts/openapi/openapi.yaml) описывает реализованные маршруты. Генерация: `python scripts/openapi.py` (Python 3, без сторонних зависимостей). Poll/calendar/initiative REST не реализованы и не объявлены работающими.

## Общий Issue protobuf

Единственный источник: `contracts/proto/smartquarter/issue/v1/issue.proto` в корне репозитория. Файл перенесён **без изменения содержимого**, включая package, RPC, номера полей, enum values и go_package. Go mapping при генерации даёт Gateway собственный `internal/gen/smartquarter/issue/v1`; Issue продолжает использовать свой `internal/gen`. Нет импорта чужого internal, нового общего Go module или изменений go.work. Клиенты Identity и Community генерируются из общих контрактов с собственным Go mapping для Gateway.

Из корня репозитория:

```powershell
./services/max-gateway/scripts/generate.ps1
```

Нужны Go и Buf. Plugins закреплены в `buf.gen.yaml`: protoc-gen-go v1.36.12, protoc-gen-go-grpc v1.6.1. Для Issue отдельно из его каталога: `buf generate ../../contracts/proto --path ../../contracts/proto/smartquarter/issue/v1/issue.proto`.

## Запуск и тесты

Нужны Go 1.26+ и Docker Engine/Compose v2. Серверный запуск с React, HTTPS и всеми сервисами: [deploy/server](../../deploy/server/README.md). Из корня репозитория:

```powershell
docker compose -f deploy/test/compose.yaml build
docker compose -f deploy/test/compose.yaml run --rm test
docker compose -f deploy/test/compose.yaml down -v
```

Это отдельный тестовый проект без опубликованных портов. Он использует настоящий Identity/Issue/Community, PostgreSQL, Redis и MinIO, синтетические MAX initData и тестовый Bot API. Проверяются пять проходов Issue workflow, отзыв membership, объявления, роли и изоляция домов. `down -v` удаляет только данные этого тестового проекта.

Проверки модуля:

```powershell
cd services/max-gateway
$env:GOWORK = 'off'
go test ./...
go vet ./...
go build ./...
```

Для native-запуска скопируйте `.env.example` в `.env.local`, заполните адреса настоящих зависимостей, MAX и Origin, затем выполните `./scripts/run.ps1`. Go сам `.env` не читает. Старый `scripts/test.ps1` оставлен для отдельной проверки Issue со stub Identity; он не заменяет полную приёмку в `deploy/test`. `compose.local.yml` подключается к старому Issue test stack и не поднимает весь продукт.

Обязательны `IDENTITY_GRPC_ADDR`, `ISSUE_GRPC_ADDR`, `ISSUE_READY_URL`, `COMMUNITY_GRPC_ADDR`, `COMMUNITY_READY_URL`, `MAX_BOT_TOKEN`, `MAX_WEBHOOK_SECRET`, `MAX_BOT_USERNAME`, `MAX_MINIAPP_URL`, непустой `TRUSTED_ORIGINS`. Остальные настройки описаны в `.env.example`. `APP_ENV=local|test` использует SameSite=Lax без Secure; production — Secure + SameSite=None и HTTPS Origins.

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

## Приёмка и внешние зависимости

Gateway проверяет membership и роль до обращения в Community. Совместный runtime-тест объявлений входит в `deploy/test`. Актуальные результаты локальных проверок и ограничения: [CI audit](../../docs/deployment/ci-audit.md). `IMPLEMENTATION_REPORT.md` описывает первоначальный этап разработки и не является текущим отчётом приёмки.

Для проверки в реальном MAX нужны зарегистрированный бот, Mini App URL, HTTPS webhook и действующие credentials. Межсервисное взаимодействие в MVP допускается только в закрытой сети Docker; публичная публикация gRPC не предусмотрена.
