# Контракты

Папки подготовлены по «Контрактам приложения.pdf». Файлы API и схемы ещё не реализованы.

| Путь | Назначение |
| --- | --- |
| `proto/smartquarter/identity/v1/` | Будущий identity.proto, package smartquarter.identity.v1 |
| `proto/smartquarter/issue/v1/` | Будущий issue.proto, package smartquarter.issue.v1 |
| `proto/smartquarter/community/v1/` | Будущий community.proto, package smartquarter.community.v1 |
| `openapi/` | Будущий openapi.yaml: HTTPS/JSON API Gateway, /api/v1 и MAX webhook |
| `events/` | Будущие versioned schemas уведомлений, без AI-событий |

React использует HTTP Gateway. Gateway вызывает сервисы по gRPC.
Domain-структуры сервисов не являются общими контрактами.

Выбранный способ размещения generated Go-кода: `services/<name>/internal/gen/` у каждого
потребителя/производителя. Источник один — `contracts/proto/`; копии генерируются, а не редактируются.
Перед добавлением первых proto нужно зафиксировать версии buf/protoc и plugins, настроить генерацию
и проверку согласованности копий. Сейчас генератора и сгенерированного кода нет.

При реализации сохраняйте стабильные имена/номера полей protobuf (удалённые номера — reserved),
HTTP error envelope и mapping gRPC status → HTTP. Контекст actor/user/house формирует Gateway
после авторизации; данные браузера не считаются подтверждёнными правами.

События уведомлений: issue.created, issue.confirmed, issue.status_changed, statement.generated,
announcement.created; события опросов/календаря/инициатив добавляются с соответствующими функциями.
Будущий envelope: event_id, event_type, event_version, occurred_at, producer, payload.
События из БД предполагают outbox; consumer — идемпотентную обработку и ACK после успеха.

Изменения контрактов согласуются с разработчиками обеих сторон. API и proto lint/generation
появятся в CI после реализации контрактов; текущий CI проверяет только Go.
