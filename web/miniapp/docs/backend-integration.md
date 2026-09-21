# Интеграция с backend

Дата аудита: 22.09.2026. Ветка `feat/react-front`, база `50fcc75` = проверенный локальный `origin/main`. Изменения ограничены `web/miniapp/**`.

## Адрес и сессия

REST base path — `/api/v1`. По умолчанию frontend использует тот же Origin. В dev Vite `/api` → `http://127.0.0.1:18080`; в проверке настоящего Gateway использован `http://127.0.0.1:18090`. Общий HTTPS ingress пока не развернут этой задачей.

Cookies отправляются с `credentials: include`, headers `Accept`, `Content-Type`, `X-Request-Id`; для create Issue/Statement/Announcement также `Idempotency-Key`. `house_id` отправляется только в смене дома; actor/role/house для бизнес-операций выводит Gateway из сессии. Frontend не посылает `X-Actor-*`, не подписывает MAX initData и не вызывает gRPC.

## Фактические маршруты

| Метод и путь относительно `/api/v1` | Request | Response |
| --- | --- | --- |
| POST /session/max | `{init_data}` | 200 `{user_context, expires_at}` и Set-Cookie |
| GET /me | — | UserContext |
| POST /session/active-house | `{house_id}` | `{active_house_id, role}` |
| POST /session/logout | `{}` | 204 без тела |
| POST /uploads | `{filename,mime_type,size_bytes}` | 201 `{upload_id,presigned_url,expires_at,required_headers}` |
| PUT на `presigned_url` | File binary, required_headers | 2xx от S3, не Gateway |
| POST /uploads/{id}/complete | `{}` | Attachment; требуется READY |
| GET /attachments/{id}/download-url | — | `{url,expires_at}` |
| POST /issues | `{category,description,location_text,attachment_ids}` | 201 Issue |
| GET /issues | page_size, page_token, повторяемый status | `{items,next_page_token}` |
| GET /issues/{id} | — | `{issue,attachments,confirmed_by_me,timeline,latest_statement}` |
| POST /issues/{id}/confirm | `{}` | `{confirmation_count,confirmed_by_me}` |
| GET /chairman/issues | page_size, page_token, status | нерешённые IssueList |
| GET /issues/{id}/statement | — | StatementDraft либо 404 |
| POST /issues/{id}/statement | `{chairman_note}` | 201 StatementDraft |
| PATCH /issues/{id}/status | `{new_status}` | Issue |
| GET /announcements | page_size, page_token | `{items,next_page_token}` |
| POST /announcements | `{title,body}` | 201 Announcement |

`Issue.confirmations_count` — множественное число, response подтверждения использует `confirmation_count`. `resolved_at`, `attachment.issue_id`, `latest_statement` nullable. JSON snapshot и payload являются объектами. Пагинация использует opaque token сервера; frontend не вычисляет offset самостоятельно. HTTP 400/401/403/404/409/413/429/5xx представлены понятными ошибками, downstream message не показывается. `request_id` остаётся для поддержки.

## Подтверждённые блокеры и расхождения

### 1. MAX bootstrap: BLOCKED

Endpoint: `POST /api/v1/session/max` с корректной свежей сырой `init_data`.

Ожидается по PDF/OpenAPI: 200, `user_context`, `expires_at`, HttpOnly session cookie.

Фактически: **503 `{error:{code:"DEPENDENCY_UNAVAILABLE",message:"dependency unavailable",request_id:...}}`**, cookie отсутствует. В `services/max-gateway/cmd/app/main.go` по-прежнему передан `Identity: identity.Unavailable{}`. Identity protobuf/service уже есть в этой базе, но клиент Gateway не подключён. Устаревший комментарий о его отсутствии в Gateway не равен текущему состоянию монорепозитория.

Для снятия блокера backend-разработчик должен реализовать согласованный адаптер Identity, подключить его в app, передавать адрес через конфигурацию и проверить bootstrap/context/membership/readiness. Нельзя исправить это ENV-переменной frontend. Go-код в данной задаче не менялся.

Реальный Gateway собран из выбранной базы и запущен с изолированным Redis и синтетическим локальным секретом. Пять корректно подписанных входов подряд получили 503, без cookie. Это **пять воспроизведений блокера**, не пять успешных бизнес-E2E. Проверки liveness, 401 без сессии/с неверной подписью и 403 с чужим Origin прошли. Issue gRPC в этой проверке не запущен: bootstrap останавливается раньше его вызова.

### 2. Community: частично доступен контракт

В PDF описаны polls/calendar/initiatives, в Gateway опубликованы только create/list announcements. Остальные HTTP пути дают 404, даже если соответствующие gRPC существуют в Community Service. Backend должен отдельно согласовать и реализовать их маршруты. Frontend их не вызывает.

GET/POST `/announcements` при пустом `COMMUNITY_GRPC_ADDR` возвращают 503 `COMMUNITY_UNAVAILABLE`. После подключения нужен работающий Community с БД и metadata от Gateway. Код frontend подключён, успешная публикация в настоящую БД не подтверждена из-за блока авторизации.

### 3. Ошибки PDF детальнее runtime

PDF описывает предметные коды вроде `STATEMENT_NOT_FOUND`. Текущий Gateway отображает gRPC codes в общие HTTP codes (`NOT_FOUND`, `PERMISSION_DENIED`, конфликт и т.д.), не сохраняет все ErrorInfo reasons. Для отсутствующего заявления frontend опирается на 404. Для 409 перечитывает карточку и не выдаёт общий конфликт за конкретную бизнес-причину. Полезное будущее изменение backend — согласованная передача структурированных кодов, без чувствительных деталей.

### 4. Скачивание Statement в MAX

GET `/issues/{id}/statement` возвращает JSON с `body`, не URL файла. В MAX native `downloadFile` нужен доступный HTTPS URL; Blob/a[download] на мобильных клиентах не поддерживается как обычное браузерное скачивание. Сейчас доступно копирование текста, в обычном браузере .txt. Backend нужен отдельный согласованный authenticated/signed HTTPS download endpoint; frontend его не выдумывает.

### 5. Deep link

Frontend поддерживает `start_param=issue_<UUID>`, полученный через официальный MAX `startapp`, а также HTTPS route `/issues/<UUID>`. Это соглашение приложения, не новый метод Bridge. Текущий bot-код не формирует такой payload в уведомлениях — для end-to-end deeplink нужна согласованная генерация ссылки ботом. Чужой дом автоматически не переключается: доступ проверяется сервером.

## HTTPS, CORS и Storage

Рекомендуется общий публичный HTTPS Origin: ingress разделяет `/api/v1/` и статику, Gateway `TRUSTED_ORIGINS` содержит точный Origin. При раздельном API Origin нужны явный `Access-Control-Allow-Origin` (не `*`), `Access-Control-Allow-Credentials: true`, preflight для JSON, Idempotency-Key и X-Request-Id. `APP_ENV` production включает Secure/SameSite=None. Cookies стороннего Origin дополнительно проверить на iOS/MAX.

S3 bucket должен разрешать Origin Mini App, методы PUT/GET/HEAD и все `required_headers` подписи (включая Content-Type/x-amz-*); response ETag можно expose для диагностики, приложение его не требует. Presigned host должен быть доступен телефону: внутреннее имя Docker `minio:9000` не подходит. Backend S3 public endpoint и `VITE_S3_ORIGINS` должны совпадать. Production использует HTTPS. Локальный HTTP разрешён только между loopback frontend и allowlisted loopback storage.

Ссылки и сырой initData не должны попадать в access logs браузерной аналитики. CSP на ingress следует согласовать с реальным MAX способом встраивания: script-src для собственного bundle и `https://st.max.ru`, connect-src для API/S3, img-src для S3/blob, font-src self; frame-ancestors и webview проверить на устройствах, не ставить X-Frame-Options DENY вслепую.

## Воспроизведение проверки блокера (Windows)

Из корня монорепозитория, Go и Docker установлены; порты 16389 и 18090 свободны:

```powershell
New-Item -ItemType Directory -Force web/miniapp/test-results | Out-Null
go build -o web/miniapp/test-results/gateway-probe.exe ./services/max-gateway/cmd/app
docker run -d --rm --name smartquarter-front-probe-redis -p 127.0.0.1:16389:6379 redis:8.2-alpine
npm run test:gateway --prefix web/miniapp
docker stop smartquarter-front-probe-redis
```

Скрипт запускает именно Go Gateway, не его Node-замену, не меняет серверный код, генерирует одноразовый тестовый token в памяти и завершает созданный дочерний процесс. Сетевые Bot API вызовы направлены на несуществующий локальный адрес, production credentials не используются. После исправления Identity probe намеренно перестанет считать 503 ожидаемым; его нужно заменить успешным сценарием на тестовом стенде.

## Условия реального E2E

Нужны подключённый Identity и тестовые ACTIVE memberships двух жителей/председателя одного дома, здоровые Gateway/Redis/Issue DB/S3, Community для объявления, HTTPS Mini App в настройках MAX bot и подписанная initData трёх реальных тестовых аккаунтов. Затем **пять раз**: Resident A upload → complete → create → list/details; Resident B confirm → счётчик; Chairman queue → generate → body/version → status → обновлённая карточка; объявление. Дополнительно проверить другой дом, истёкшую сессию, S3 CORS/expiry, BackButton, клавиатуру и копирование на MAX Android/iOS.

До выполнения этих шагов реальный бизнес-E2E, настоящая S3-загрузка через UI и тест внутри MAX остаются **BLOCKED / NOT RUN**. Playwright fixtures и HTTP security probe не заменяют эту приёмку.

