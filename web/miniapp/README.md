# React Mini App

Каркас каталогов для React + TypeScript + Vite внутри MAX. Код, зависимости, package.json,
lock-файл и frontend toolchain пока не добавлены; приложение не запускается.

- `src/app` — будущие bootstrap, router и providers (сессия, TanStack Query).
- `src/pages` — страницы и сборка экранов.
- `src/features/session` — клиентское состояние сессии/активного дома.
- `src/features/uploads` — сценарий presigned upload.
- `src/features/issues` — форма, список, карточка и подтверждение проблемы.
- `src/features/chairman` — очередь, статусы и черновик заявления.
- `src/features/announcements` — объявления MVP.
- `src/features/polls` — опросы (Should Have). Календарь/инициативы добавляются позже.
- `src/shared/api` — HTTP-клиент Gateway, DTO и общие query utilities.
- `src/shared/max` — будущая обёртка MAX Bridge.
- `src/shared/ui` — общие UI-компоненты; `src/shared/config` — публичная клиентская конфигурация.
- `public` — статические файлы; `tests` — будущие frontend-тесты.

React не вызывает gRPC и не хранит доверенные роли. MAX initData проверяет Gateway;
файлы отправляются в S3 только по выданному signed URL. Server-state кэшируется отдельно
от UI-state, а окончательные права и бизнес-операции проверяет backend.
Ни bot token, ни другие серверные секреты не должны попадать во frontend-конфигурацию.

