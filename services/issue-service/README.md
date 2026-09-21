# Issue Service

Go-сервис проблем многоквартирного дома: фото, заявки, подтверждения, статусы, timeline и версии заявления по шаблону. Бизнес API — gRPC `:8082`; HTTP `:8083` обслуживает только `GET /livez`, `GET /readyz`, `GET /metrics`. Авторизация MAX, справочники домов/пользователей и отправка уведомлений принадлежат gateway/другим сервисам.

## Запуск за одну команду

Нужны Go 1.26+, PowerShell 7 и запущенный Docker Desktop с Linux containers. Из этой директории:

```powershell
./scripts/local.ps1 up
```

Скрипт создаёт игнорируемый `.env.local` со случайными **локальными** паролями, поднимает PostgreSQL, Redis, MinIO, создаёт приватный тестовый bucket, собирает образ, применяет миграции и ждёт readiness. Повторный запуск сохраняет данные.

| Компонент | Адрес на машине |
|---|---|
| gRPC | localhost:18082 |
| Readiness | http://localhost:18083/readyz |
| Metrics | http://localhost:18083/metrics |
| PostgreSQL | localhost:15432, БД issue_test |
| Redis | localhost:16379 |
| MinIO API / console | localhost:19000 / localhost:19001 |

`./scripts/local.ps1 stop` останавливает только этот Compose-проект, сохраняя volumes. Возобновление: `./scripts/local.ps1 up`. При остановке сначала завершаются gRPC-запросы, затем HTTP, outbox worker, Redis и PostgreSQL pool. Лимит shutdown — 15 секунд на gRPC и HTTP, Compose даёт 40 секунд.

Этот Compose использует **тестовую** БД: интеграционные тесты пересоздают её схему. Для сохраняемой демонстрации используйте отдельную `issue_db` и production-конфигурацию.

## Архитектура и границы

`transport/grpc → usecase → domain + repository/storage interfaces`.
Адаптеры: `repository/postgres` (pgxpool), `storage/yandexs3` (AWS SDK v2), `outbox` (Redis Streams).
`internal/app` собирает зависимости и управляет жизненным циклом; `observability` содержит zap/Prometheus. Сервис не читает таблицы соседей; внешние UUID не имеют cross-service FK.

Единственный источник protobuf: `../../contracts/proto/smartquarter/issue/v1/issue.proto` в корне репозитория. Wire contract не изменён. Server stubs остаются в `internal/gen`; Gateway генерирует собственные client stubs из того же файла с Go import mapping. Общая генерация: `../max-gateway/scripts/generate.ps1`. HTTP → gRPC интеграция проверяется тестами Gateway с настоящим Issue Service.

## gRPC

Все вызовы требуют trusted metadata от gateway:

```text
x-request-id: UUID (при отсутствии/невалидности сервис генерирует свой)
x-actor-user-id: UUID
x-house-id: UUID
x-actor-role: RESIDENT | CHAIRMAN | ADMIN
```

UUID — канонический lowercase, ненулевой. Повторяющиеся actor metadata отклоняются. `house_id` запроса должен совпадать с metadata; сущности дополнительно проверяются по дому. `x-request-id` возвращается в response metadata. Эти заголовки не являются самостоятельной аутентификацией: gRPC должен быть доступен только доверенному gateway по приватной сети. TLS/mTLS на публичном входе обеспечивает инфраструктура; текущий listener plaintext.

| RPC | Правила |
|---|---|
| CreateUpload | jpeg/png, положительный размер ≤ MAX_UPLOAD_SIZE; возвращает PUT URL и обязательные headers |
| CompleteUpload | Только uploader своего дома; проверка размера, image header/signature, SHA-256, ETag; повтор READY идемпотентен |
| CreateIssue | 1–10 собственных READY attachments, категория, адрес и описание обязательны |
| GetIssue | Свой дом; attachments, confirmed_by_me, timeline; latest_statement только менеджеру |
| ListIssues | Статусы, cursor pagination; chairman_queue только CHAIRMAN/ADMIN, исключает RESOLVED |
| ConfirmIssue | Нельзя подтвердить свою заявку; одно подтверждение пользователя на заявку |
| UpdateIssueStatus | Только CHAIRMAN/ADMIN и разрешённый переход |
| GenerateStatement | Только CHAIRMAN/ADMIN; новая версия, детерминированный text/template и snapshot |
| GetStatement | Только CHAIRMAN/ADMIN; последняя сохранённая версия |
| GetAttachmentDownloadURL | ATTACHED своего дома либо собственный READY; короткий GET URL |

Категории MVP: SAFETY, CLEANLINESS, UTILITIES, INFRASTRUCTURE, OTHER. Неизвестные/нулевые enum отклоняются. Описание ≤8000 байт, адрес/место ≤1000, примечание ≤4000; page_size по умолчанию 20, максимум 100. Cursor привязан к дому и фильтру, порядок `created_at DESC, id DESC`.

Цепочка статусов:
`DETECTED → CONFIRMING → READY_FOR_APPEAL → HANDED_TO_CHAIRMAN → MARKED_SENT → WAITING_RESULT → RESOLVED`.
CHAIRMAN/ADMIN могут закрыть заявку из DETECTED/CONFIRMING; только ADMIN может вернуть RESOLVED → CONFIRMING. Подтверждение не меняет статус автоматически. При закрытии записывается resolved_at, при административном переоткрытии очищается.

Ошибки: InvalidArgument — входные данные; Unauthenticated — actor; PermissionDenied — дом/роль; NotFound — ресурс; AlreadyExists — повтор подтверждения; FailedPrecondition — состояние/своё подтверждение; ResourceExhausted — лимит; Unavailable — зависимости; Internal — неожиданный сбой. SQL, signed URLs и подробности AWS не попадают в ошибки и логи.

## PostgreSQL и миграции

Миграция `000001_init` создаёт `issues`, `attachments`, `confirmations`, `timeline_events`, `statement_drafts`, `outbox_events`, constraints и индексы.
PK `confirmations(issue_id,user_id)` исключает повторы; счётчик меняется атомарно. Блокировка issue сериализует статусы, подтверждения и версии заявления. Создание заявки, привязка фото, timeline и outbox коммитятся одной транзакцией.

Для самостоятельного запуска задайте переменные из `.env.example` в процессе (Go не загружает dotenv):

```powershell
$env:GOWORK = 'off'
# DATABASE_URL и остальные настройки уже заданы через окружение.
go run ./cmd/app migrate up
go run ./cmd/app
```

`go run ./cmd/app migrate down` удаляет все шесть таблиц: только для осознанного отката/пустого тестового окружения. Автомиграций при старте сервера нет. Разрешённые имена БД: issue_db или issue_test*. Для интеграционных тестов требуется строго issue_test. Production DB/роль создаются администратором отдельно.

## Фото: Yandex Object Storage / локальный MinIO

Один адаптер работает с обоими хранилищами. Production: `S3_ENDPOINT=https://storage.yandexcloud.net`, регион ru-central1, приватный bucket, ключ сервисного аккаунта через AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY. Разрешения нужны на HeadBucket, чтение, PUT и удаление объектов. Не включайте public read/write. Endpoint вне local/test требует HTTPS.

1. CreateUpload записывает UPLOADING и выдаёт URL на ключ `houses/{house}/attachments/{id}/original`.
2. Браузер делает прямой PUT с **всеми required_headers**, включая подписанный `If-None-Match: *`. Повторная запись того же ключа отклоняется хранилищем. См. [Yandex PUT Object](https://yandex.cloud/en/docs/storage/s3/api-ref/object/upload).
3. CompleteUpload делает HEAD, затем условный GET с If-Match ETag; проверяет размер и заголовок PNG/JPEG, потоково вычисляет SHA-256 без открытой DB transaction. Затем в короткой транзакции повторно проверяет владельца, статус, параметры и срок загрузки и сохраняет результат. Параллельный успешный CompleteUpload имеет приоритет над устаревшим отказом; его файл не удаляется. Полное декодирование пикселей не выполняется.
4. Успех → READY, ошибка файла → REJECTED, истечение срока → EXPIRED. Невалидный объект удаляется best effort. Ошибка сети оставляет возможность повторить CompleteUpload.
5. CreateIssue переводит READY → ATTACHED. Ссылки не сохраняются в PostgreSQL.

Для браузера настройте bucket CORS на origin Mini App: методы PUT/GET/HEAD, allowed headers Content-Type и If-None-Match, expose ETag. Не подменяйте host/path signed URL после подписания. В локальном Compose S3_ENDPOINT=http://minio:9000 для сервера, S3_PUBLIC_ENDPOINT=http://localhost:19000 для браузера. SDK секреты не выдаёт клиенту.

## Outbox и наблюдаемость

События: `issue.created`, `issue.confirmed`, `issue.status_changed`, `statement.generated`. Worker берёт pending строки через FOR UPDATE SKIP LOCKED, XADD публикует в NOTIFICATION_STREAM и отмечает published_at. При ошибке сохраняет attempts и next_attempt_at с backoff до 256 секунд. Redis не участвует в business transaction и readiness.

Redis запись содержит поля event_id, event_type, data; data — JSON envelope с event_id, event_type, event_version=1, occurred_at, producer=issue-service, payload. Доставка **at least once**: consumer обязан дедуплицировать event_id. Между несколькими worker глобальный порядок не гарантируется. Consumer уведомлений расположен за пределами сервиса.

Zap пишет request_id, actor/house/issue ID, RPC, код, duration; без фото, signed URLs и текста заявлений. При panic отдельно пишет error с request_id, RPC, типом panic и стеком; само значение panic не логируется, клиент получает только Internal. Prometheus: grpc_requests_total, grpc_errors_total, grpc_request_duration_seconds, postgres_pool_*, outbox_pending_count, outbox_publish_errors_total, s3_operations_total, s3_operation_errors_total. Readiness проверяет PostgreSQL и HeadBucket; liveness не обращается к зависимостям.

## Конфигурация

Полный список и defaults — `.env.example`. Обязательны DATABASE_URL, S3_BUCKET и доступные SDK credentials.
Дополнительно: APP_ENV, GRPC_ADDR, HTTP_ADDR, LOG_LEVEL, S3_ENDPOINT/S3_PUBLIC_ENDPOINT/S3_REGION/S3_USE_PATH_STYLE, S3_UPLOAD_URL_TTL (10m), S3_DOWNLOAD_URL_TTL (5m), MAX_UPLOAD_SIZE (10 MiB), REDIS_ADDR/PASSWORD/DB, NOTIFICATION_STREAM, OUTBOX_BATCH_SIZE (100), OUTBOX_POLL_INTERVAL (1s), MAX_PAGE_SIZE (100), REQUEST_TIMEOUT (30s), SHUTDOWN_TIMEOUT (15s). Signed TTL: 1s–1h, размер ≤100 MiB, batch/page максимум 1000.

## Генерация и проверки

Из этой папки, независимо от соседних модулей:

```powershell
$env:GOWORK = 'off'
go test ./...
go vet ./...
go build ./...
# Buf CLI 1.72.0; версии Go plugins зафиксированы в buf.gen.yaml.
buf generate ../../contracts/proto --path ../../contracts/proto/smartquarter/issue/v1/issue.proto
buf build
./scripts/local.ps1 test
# После local.ps1 up — smoke именно собранного и запущенного контейнера:
go test -tags=container -count=1 -v ./tests/container
```

Последняя команда останавливает только локальный issue-service, поднимает зависимости, выполняет unit и integration тесты. Она **пересоздаёт схему issue_test**; после тестов приложение остаётся остановленным, `local.ps1 up` возобновляет работу. MinIO создаётся только локальным helper `cmd/devstorage`; production bucket сервис не создаёт.

Integration tests используют реальный PostgreSQL, Redis, MinIO и TCP gRPC. Проверяют up/down/up, rollback всей business transaction, ownership, курсор с совпадающими датами, конкурентные подтверждения, версии заявлений, все 10 RPC, права/дом, enum, metadata, cancel/deadline, panic recovery, signed URLs, запрет перезаписи, SKIP LOCKED, сбой Redis и дубликаты. Полный happy path повторяется пять раз.

Race detector требует C compiler. На Windows без GCC можно выполнить unit-тесты в Linux builder:

```powershell
docker build --target build -t smartquarter-issue-tests .
docker run --rm -e CGO_ENABLED=1 smartquarter-issue-tests go test -race ./...
```

Перед релизом отдельно выполните проверку настоящего Yandex bucket с тестовыми credentials:

```powershell
$env:YANDEX_SMOKE_BUCKET = 'your-dedicated-smoke-bucket'
$env:YANDEX_SMOKE_CONFIRM = 'yes'
go test -tags=yandex -run TestYandexSmoke -count=1 -v ./tests/integration
```

Тест создаёт уникальный объект в issue-service-smoke/, проверяет PUT/HEAD/GET, запрет overwrite, истечение URL и удаляет объект. Реальные Yandex credentials не входят в репозиторий; локальная MinIO-проверка не заменяет этот pre-release smoke.

## Ограничения MVP

Нет собственного MAX auth, публичного REST, AI/PDF, межсервисных справочников или consumer уведомлений. Подлинность actor гарантирует доверенная сеть/gateway. Нет глобальной идемпотентности CreateIssue/GenerateStatement; READY CompleteUpload и уникальность подтверждений защищены отдельно. Нет worker очистки заброшенных READY/UPLOADING и retention outbox/stream: настройте отдельное обслуживание, не удаляющее ATTACHED. Timeline возвращается целиком, gRPC response ограничен 4 MiB. Согласование CORS, общих proto, gateway и реальный Yandex smoke остаются интеграционными шагами за пределами локального запуска.

====

./app

./app migrate up
./app migrate down

./app healthcheck

---

# Локальное тестирование 

cd D:\Work\Projects\SmartCounty\services\issue-service

.\scripts\local.ps1 up


создаёт .env.local
↓
поднимает PostgreSQL
↓
поднимает Redis
↓
поднимает MinIO
↓
создаёт bucket issue-test
↓
собирает Docker image issue-service
↓
выполняет migrate up
↓
запускает issue-service
↓
ждёт /readyz
