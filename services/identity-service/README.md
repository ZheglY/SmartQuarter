# identity-service

Пользователи, дома, memberships и роли RESIDENT/CHAIRMAN/ADMIN. Владеет identity_db.

Запуск из этой папки: `go run ./cmd/app`. HTTP-порт 8081, переопределяется через `HTTP_ADDR`.
Сейчас работает только прежний `GET /healthz`. Новые каталоги — заготовки, интеграции не подключены.

- `cmd/app` — существующая точка запуска; `internal/app` — место для будущего wiring.
- `internal/config` — окружение; `internal/observability` — будущие логи/метрики.
- `internal/transport/http` — существующий health handler.
- `internal/gen` — будущий generated protobuf-код из корневых контрактов.
- `tests/integration` — место для интеграционных тестов; unit-тесты рядом с кодом.
- `internal/domain` — сущности/инварианты; `internal/usecase` — прикладные сценарии.
- `internal/repository/postgres` — будущий адаптер pgx/pgxpool собственной БД.
- `internal/transport/grpc` — будущие бизнес-вызовы; gRPC-сервер пока не запущен.
- `internal/clients` — межсервисные клиенты; `internal/events` — outbox/events при необходимости.
- `migrations` — будущие миграции; `testdata/seed` — синтетические данные собственной БД.

Контракты — в `contracts/`, общие правила — в корневом README и `docs/architecture.md`.
