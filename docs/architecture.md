# Архитектура и границы каркаса

Основа — документы проекта от 16 сентября 2026 года и исправленные схемы.
Требования к готовому MVP используются для размещения будущего кода, а не как указание
реализовать все перечисленные функции в этом изменении.

## Принятые решения

- Четыре Go-сервиса: Gateway, Identity, Issue, Community. AI Worker исключён.
- React Mini App → Gateway: HTTPS/JSON. Gateway → предметные сервисы: gRPC/protobuf.
- Gateway: проверка MAX initData, сессии, actor context, rate limit, idempotency, HTTP/gRPC mapping и MAX Bot API.
  Окончательные бизнес-права проверяет сервис-владелец операции.
- Identity: пользователи, дома и memberships. Роли MVP: RESIDENT, CHAIRMAN, ADMIN.
- Issue: загрузки, проблемы, подтверждения, статусы, timeline и StatementDraft по шаблону без AI.
  Содержимое шаблонов будет в `internal/statement/templates`, S3-адаптер — в `internal/storage/s3`.
- Community: объявления — Must Have; опросы — Should Have; календарь и инициативы — Could Have.
  Для расширений не создаются отдельные сервисы.
- PostgreSQL — источник бизнес-данных. У каждого предметного сервиса своя БД/схема,
  `repository/postgres`, `migrations` и `testdata/seed`; cross-service SQL/FK запрещены.
  Целевые инструменты: pgx/pgxpool и golang-migrate.
- Redis — временные сессии/кэш, rate limiting, idempotency и при необходимости Streams.
  Уведомления остаются в Gateway; `events` зарезервирован для outbox/consumer кода.
- MinIO/S3 — приватные файлы; метаданные хранятся в Issue. Прямой presigned upload из браузера.
- Frontend обращается к Gateway, а к S3 — только для разрешённых загрузок.
  Будущий TanStack Query кэширует server-state, MAX Bridge размещается в `shared/max`.
- `internal/observability` — место для будущих zap/Prometheus и request metadata.

## Структура предметного Go-сервиса

```text
cmd/app/                  существующая точка запуска
internal/
  app/                    место для wiring и жизненного цикла
  config/                 конфигурация
  domain/                 сущности и инварианты
  usecase/                прикладные сценарии (вместо internal/service)
  repository/postgres/    собственная БД
  transport/grpc/         будущие gRPC handlers
  transport/http/         только существующая HTTP-диагностика
  clients/                внешние/межсервисные клиенты
  gen/                    будущий сгенерированный protobuf-код
  events/                 будущие события/outbox
  observability/          логи, метрики, correlation
migrations/               будущие миграции своей БД
testdata/seed/             будущие синтетические данные сервиса
tests/integration/        будущие repository/integration tests
```

У Gateway нет собственного доменного слоя и PostgreSQL repository.
Для него выделены `auth`, `session`, `cache`, `ratelimit`, `idempotency`,
`transport/webhook`, `transport/http/middleware`, `clients/grpc`, `clients/max` и `notifications`.
`configs` предназначен для будущих несекретных настроек; текущий запуск читает только окружение.

## Сопоставление восьми документов

| Документ | Что отражено в каркасе |
| --- | --- |
| MVP и архитектура проекта.pdf, стр. 2–7 | 4 сервиса, gRPC, usecase/PostgreSQL/observability, отсутствие AI |
| Архитектура_Умный_Квартал_исправлено.pdf, стр. 2–3, 6–10 | Отдельные модули, configs Gateway, tests, миграции и владение БД |
| Умный_Квартал_сервисы_React_авторизация_кеширование_v2.pdf, стр. 2–8 | Каталоги React, auth/session/cache Gateway, разделение клиента и сервера |
| Контракты приложения.pdf, стр. 2–8 | Версионированные proto, OpenAPI, события и размещение клиентских функций |
| Постановка задачи.pdf, стр. 2–5 | Основной сценарий без AI, границы MVP и приоритеты Community |
| Формат сдачи и чек-лист приемки.pdf, стр. 2–5 | Место для seed/integration/E2E и материалов сдачи |
| Ограничения и рекомендации хакатона.pdf, стр. 2–4 | MAX bot + Mini App, четыре сервиса, явные ограничения и тестовые данные |
| ТЗ.pdf, стр. 6–10, 13, 15–20 | Воспроизводимый запуск и будущий комплект сдачи; только каркас в текущей задаче |

Уточнения между документами:
- Общие схемы сокращают путь до `contracts/proto/identity/v1`; используется более точный
  `contracts/proto/smartquarter/identity/v1` из документа контрактов, аналогично для Issue/Community.
- Формулировка «объявления и опросы в MVP» уточняется MoSCoW: обязательны объявления, опросы — Should Have.
- Чек-лист условно упоминает DATA-API.yaml, но исходное ТЗ требует его для решения с собственным API.
  Файл следует подготовить к сдаче вместе с OpenAPI; сейчас контрактов и проверяемого API ещё нет.
- Документы описывают целевой стек zap, gRPC, PostgreSQL/Redis/MinIO и health/metrics.
  Существующий технический HTTP-каркас сохранён без изменения поведения; новые пакеты и инфраструктура не подключались.

## Места для последующих материалов

`docs/submission/` — сценарий проверки в MAX, сведения о команде/доступе, источниках данных,
ограничениях и версии релиза. `tests/e2e/` — будущий сквозной smoke test.
Seed-данные будут синтетическими; рабочие токены и персональные данные не коммитятся.
SQL-миграции, proto/OpenAPI, DATA-API.yaml, frontend package/lock-файл, генерация контрактов
и инфраструктурные контейнеры добавляются при реализации соответствующих компонентов.
