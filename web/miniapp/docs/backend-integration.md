# Интеграция с backend

Обновлено после интеграции Identity 22.09.2026. Описывает текущий связанный MVP.

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

### 1. MAX bootstrap: Identity подключён

Gateway использует общий Identity protobuf и настоящий gRPC-адаптер, а не
identity.Unavailable. Вход с корректной initData создаёт HttpOnly cookie.
Новый пользователь без membership получает пустой active_house_id; UI показывает
профиль и запрос доступа к дому. Оператор назначает роли через provision-команду
Identity (docs/deployment/server-guide.md, раздел 9). Отозванная membership не
блокирует вход, но запрещает защищённые операции.

Контейнерная приёмка: deploy/test/compose.yaml. Здесь настоящий Identity, Issue,
Community, PostgreSQL, Redis и MinIO; внешний MAX - тестовый HTTP endpoint.
Проверка на реальных устройствах и публичном домене выполняется отдельно.

### 2. Community

Production Compose подключает Community; его готовность входит в /readyz Gateway.
GET/POST announcements используют проверенную роль и дом из сессии. Опросы,
календарь и инициативы не входят в опубликованный HTTP/UI MVP.

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

## Сквозная приёмка

Смотрите корневой README и deploy/test/README.md. `npm run test:gateway` запускает полный Docker-стенд с настоящим Identity и удаляет тестовые данные после прогона. Тестовый стенд
проверяет пять бизнес-проходов, объявления, роли и изоляцию домов.

Для приёмки в MAX нужны домен с HTTPS, токен и webhook вашего бота, облачный
S3 с CORS, проверенные MAX ID жителей/председателя и назначенные memberships.
Не подменяйте initData в production и не назначайте роль через frontend headers.
