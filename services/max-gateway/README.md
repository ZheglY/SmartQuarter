# max-gateway

Публичный HTTP/JSON API и интеграция MAX: auth/session, маршрутизация в gRPC и уведомления.

Запуск из этой папки: `go run ./cmd/app`. HTTP-порт 8080, переопределяется через `HTTP_ADDR`.
Сейчас работает только прежний `GET /healthz`. Новые каталоги — заготовки, интеграции не подключены.

- `cmd/app` — существующая точка запуска; `internal/app` — место для будущего wiring.
- `internal/config` — окружение; `internal/observability` — будущие логи/метрики.
- `internal/transport/http` — существующий health handler.
- `internal/gen` — будущий generated protobuf-код из корневых контрактов.
- `tests/integration` — место для интеграционных тестов; unit-тесты рядом с кодом.
- `internal/transport/http/middleware`, `internal/transport/webhook` — будущий HTTP-периметр и MAX webhook.
- `internal/auth`, `internal/session` — проверка MAX initData и прикладная сессия.
- `internal/cache`, `internal/ratelimit`, `internal/idempotency` — будущие Redis-механизмы.
- `internal/clients/grpc/{identity,issue,community}` — клиенты предметных сервисов.
- `internal/clients/max` — MAX Bot API; `internal/notifications` и `internal/events` — уведомления.
- `configs` — будущие несекретные настройки. Сейчас файлы конфигурации не читаются.

Бизнес-правила и шаблон заявления принадлежат предметным сервисам.

Контракты — в `contracts/`, общие правила — в корневом README и `docs/architecture.md`.
