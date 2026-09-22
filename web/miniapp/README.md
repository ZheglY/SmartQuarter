# Умный Квартал — MAX Mini App

React-приложение для жителей и председателей. Дизайн перенесён из предоставленного `max-miniapp-design-main.zip`; рабочие данные загружаются только через REST max-gateway. Демонстрационные ответы находятся исключительно в `tests/` и не включаются в production bundle.

**Статус интеграции:** Gateway подключён к настоящим Identity, Issue и Community. Сквозной Docker-тест проверяет вход, фото, проблемы, подтверждения, заявления, уведомления и объявления. Для запуска на вашем домене и внутри MAX нужны действующие настройки бота, HTTPS и S3. Подробности: [backend-integration.md](docs/backend-integration.md).

## Запуск

Node.js 24, npm 11. Команды из `web/miniapp`:

```sh
npm ci
# Создать .env.local на основе .env.example, указать адрес Gateway и S3 origins.
npm run dev
```

Адрес Vite: `http://127.0.0.1:5173`. `/api` проксируется на `GATEWAY_PROXY_TARGET`, по умолчанию `http://127.0.0.1:18080`. Gateway по умолчанию слушает 8080 — порт 18080 является рекомендуемым локальным размещением, его надо настроить отдельно. `TRUSTED_ORIGINS` Gateway должен содержать Origin frontend. Proxy сохраняет исходный Origin.

Обычный браузер без MAX показывает экран открытия через домового бота. Фальшивый вход или переключатель роли для разработки отсутствуют. Для просмотра авторизованных экранов без MAX используйте Playwright fixtures, для настоящего входа — зарегистрированное HTTPS Mini App.

```sh
npm run typecheck
npm run lint
npm run test
npm run build
npx playwright install chromium webkit
npm run test:e2e
npm run preview -- --port 4173
```

E2E запускается на собранном `dist`, поэтому после изменения исходников сначала выполните `npm run build`. Tests не зависят от работающего backend. Playwright проверяет Chromium и WebKit с мобильными viewport. `npm run format` форматирует исходники и тесты.

## Архитектура

- `src/app`: маршруты, guards, общая оболочка, CSS.
- `src/pages`: главная, проблемы, создание и карточка, объявления, кабинет председателя, профиль, сообщество.
- `src/features/session`: MAX bootstrap, Query UserContext, смена дома, logout, обработка истёкшей сессии.
- `src/features/issues`: временные черновики форм в памяти, разделённые по user/house.
- `src/features/uploads`: проверка файлов и последовательность presign → прямой PUT → complete.
- `src/shared/api`: Fetch, Zod DTO, типизированные REST-методы, безопасные сообщения об ошибках.
- `src/shared/max`: типизированный официальный MAX Bridge.
- `src/shared/ui`: карточки, badges, диалог подтверждения, timeline, загрузка вложений, состояния страниц.
- `tests`: Vitest/RTL, HTTP-контракты, upload/security и Playwright; `scripts/probe-gateway.mjs`: отдельная проверка настоящего Gateway.

Серверные данные хранятся в TanStack Query с ключом активного дома. После mutations инвалидируются соответствующие запросы. Смена дома отменяет чтения, скрывает старый UI, отправляет `house_id`, получает новый `/me`, очищает кэш и черновики. Роль берётся только из ACTIVE membership текущего пользователя и дома; окончательное разрешение всегда проверяет backend.

Сетевые mutations не повторяются автоматически. CreateIssue, GenerateStatement и CreateAnnouncement получают `Idempotency-Key`. После timeout/5xx повторный submit использует тот же ключ и payload; форма блокирует изменение неоднозначно отправленных данных. Состояние не переживает полную перезагрузку страницы — перед новой отправкой после перезагрузки проверьте список на дубликат. Черновики и фото остаются в памяти при переходах и ошибке входа, очищаются при logout/смене дома. `initData`, cookie и подписанные URL не записываются в localStorage, sessionStorage, аналитику или console.

## Страницы

| Route | Назначение |
| --- | --- |
| `/` | Дом, последние проблемы и объявления |
| `/issues` | Проблемы, фильтр статуса, cursor pagination |
| `/issues/new` | Категория, описание, место, 1–10 фото, согласие |
| `/issues/:issueId` | Проблема, фотографии, подтверждение, история |
| `/news` | Объявления дома |
| `/community` | Ссылка на объявления; недоступные разделы |
| `/profile` | Пользователь, ACTIVE дома, роль, смена дома, logout |
| `/chairman` | Очередь нерешённых проблем, только CHAIRMAN/ADMIN |
| `/chairman/issues/:issueId` | Заявление, копирование, ручная смена статуса |
| `/chairman/announcements/new` | Создание объявления |

Переходы неизвестных маршрутов и невалидных UUID обрабатываются без произвольных запросов. Таблица экран → API и перенос старых HTML: [design-migration.md](docs/design-migration.md).

## MAX и авторизация

Официальный script `https://st.max.ru/js/max-web-app.js` подключён в `index.html`. MAX сам инициализирует `window.WebApp`. Adapter читает **сырую** `initData`, поддерживает документированные `BackButton`, `getViewportSize`, closing confirmation с проверкой доступности. `ready`, `expand`, Telegram API и выдуманные свойства темы не используются. Safe area берётся из CSS `env()`, высота — из Bridge или `100dvh`; системная клавиатура требует дополнительной проверки на реальных устройствах.

Вход: `POST /api/v1/session/max {init_data}` → `{user_context, expires_at}`. Последующие запросы используют cookie с `credentials: include`; `/me` обновляется при возвращении фокуса и раз в минуту. Cookie задаёт Gateway: HttpOnly, production Secure/SameSite=None, local Lax. Frontend не читает её и не создаёт bearer token. При 401 защищённый кэш удаляется и предлагается повторный вход; при протухшей `initData` переоткройте Mini App в MAX.

Deep link использует официальный `startapp`, который MAX передаёт как подписанный `start_param`. **Соглашение этого приложения**: `issue_<UUID>`, например `https://max.ru/<bot>?startapp=issue_<UUID>`. После bootstrap открывается карточка; доступ к чужому дому определяет Gateway. Нынешний бот не формирует такие ссылки автоматически: это отдельная задача backend. Прямой HTTPS route `/issues/<UUID>` также работает при SPA fallback и наличии MAX контекста.

Документация MAX: [Bridge](https://dev.max.ru/docs/webapps/bridge), [введение](https://dev.max.ru/docs/webapps/introduction), [валидация initData](https://dev.max.ru/docs/webapps/validation). Использованные методы сверены 22.09.2026.

## Загрузка фотографий

JPEG/PNG, 1–10 файлов, каждый до 10 MiB. UI показывает локальные object URL и состояния выбора, отправки, проверки, успеха/ошибки. CreateIssue разрешён только после READY всех вложений; в него передаются только `attachment_ids`. Описание ограничено 8000 UTF-8 байт, место 1000, примечание председателя 4000; это ограничения Issue Service.

`required_headers` от Gateway передаются без изменения в **прямой** `PUT` File на allowlisted S3 Origin с `credentials: omit`, `redirect: error`, `referrerPolicy: no-referrer`. Gateway cookie и Authorization не уходят в S3. Затем вызывается `/uploads/{id}/complete`. PUT ограничен 120 секундами, REST — 20 секундами; доступна отмена. При ошибке фотография не считается READY. Незавершённые/потерянные загрузки очищает backend по TTL; frontend не имеет DELETE endpoint. Уже завершённые файлы повторно не загружаются при повторе CreateIssue в той же форме.

Ссылки чтения выдаёт `/attachments/{id}/download-url`. Они обновляются до истечения срока, при фокусе и один раз при ошибке изображения; автоматический бесконечный цикл исключён. Public object URL из метаданных не строится.

## Публичные переменные

| Variable | Значение |
| --- | --- |
| `VITE_API_BASE_URL` | Origin Gateway без `/api/v1`, по умолчанию пусто (same origin) |
| `VITE_S3_ORIGINS` | Разрешённые точные Origins хранилища через запятую; в production только HTTPS |
| `GATEWAY_PROXY_TARGET` | Только локальный Vite proxy, default `http://127.0.0.1:18080` |

Vite-переменные встраиваются во время сборки, не меняются через runtime `docker -e`. Никаких MAX bot token, S3 keys, DB/Redis credentials во frontend. Настройте точный Origin bucket, если используется virtual-hosted URL, например `https://bucket.storage.yandexcloud.net`.

## Production / Docker

```sh
npm run build
# Из корня монорепозитория:
docker build -t smartquarter-miniapp:local web/miniapp
docker run --rm -p 127.0.0.1:18091:8080 smartquarter-miniapp:local
```

Multi-stage Node → nginx. Nginx отдаёт только статику, `/healthz` и SPA fallback; неизвестные `/assets/` и `/api/` возвращают 404. Согласованный HTTPS ingress обязан направлять `/api/v1/` на настоящий Gateway, остальные пути на frontend. Пример приведён в [deployment-example.md](docs/deployment-example.md); Полный связанный стек с Caddy находится в `deploy/server/compose.yaml`. Нет отдельного Node backend. Для другого API Origin передайте build arg `VITE_API_BASE_URL`, настройте credentialed CORS и проверьте cookie в MAX webview.

## Ограничения

- Identity подключён; доступ к дому назначается оператором через `provision`. Приёмка на реальном MAX и облачном S3 выполняется после настройки credentials.
- Объявления подключены к имеющимся REST-методам, требуют `COMMUNITY_GRPC_ADDR` и работающего Community Service.
- Опросы, календарь, инициативы, управление жителями и домом, журнал администратора не имеют публичных Gateway routes. UI не имитирует их выполнение.
- Заявление — серверный текст, ручная отправка. Копирование поддерживается; `.txt` через Blob доступен в обычном браузере. В нативном MAX скачивание требует HTTPS URL и документированный `downloadFile`; такого Gateway endpoint сейчас нет.
- Нет realtime push обновления списков, офлайн-режима и постоянного хранения черновиков. Ручное обновление/возврат фокуса перечитывает данные.
- Реальный MAX Android/iOS, production S3 CORS и бизнес-сценарий с тремя аккаунтами требуют проверки с вашими credentials и устройствами. Браузерные fixtures их не заменяют.

Текущие результаты: [CI audit](../../docs/deployment/ci-audit.md). Первоначальная проверка frontend сохранена в [verification.md](docs/verification.md).

`npm run test:gateway` запускает `deploy/test` с настоящими сервисами и очищает его синтетические данные после проверки. Нужен Docker Engine и свободное место для сборки образов; отдельный Windows .exe больше не требуется.
