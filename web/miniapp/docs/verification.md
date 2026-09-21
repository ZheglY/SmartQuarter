# Отчёт о реализации и проверках

Дата: 22.09.2026. Рабочая ветка: `feat/react-front`.

База и текущий HEAD: `50fcc75bd60106b43834d3a15b76444b0ce05e61` (Merge pull request #21, identity-service), совпадает с проверенным локальным `origin/main`. Новый commit не создавался; push/merge не выполнялись. До реализации рабочее дерево было чистым. Все изменения находятся в `web/miniapp/**`; services, contracts, deploy, корневые файлы не изменены.

## Результат

React + strict TypeScript + Vite, React Router, TanStack Query, Fetch/Zod. Дизайн исходного HTML перенесён с сохранением палитры, Manrope, мобильной компоновки, карточек и нижней навигации. 10 рабочих маршрутов: `/`, `/issues`, `/issues/new`, `/issues/:issueId`, `/news`, `/community`, `/profile`, `/chairman`, `/chairman/issues/:issueId`, `/chairman/announcements/new`.

Клиент реализует bootstrap/session, active house, uploads/complete/download, Issue create/list/details/confirm, chairman queue/statement/status, announcements. Фото отправляются напрямую в allowlisted S3, server state хранится в Query. Роли и принадлежность дому определяет сервер; черновики не содержат постоянных credentials. Нет production mock data или локального backend-заменителя.

Точная таблица экран → REST, скрытые элементы дизайна: [design-migration.md](design-migration.md). Архитектура, команды, публичные ENV: [README](../README.md). Необходимые настройки и блокеры: [backend-integration.md](backend-integration.md). Рекомендуемый пример ingress: [deployment-example.md](deployment-example.md). Лицензии: [dependencies.md](dependencies.md).

## Проверки

| Проверка | Результат |
| --- | --- |
| `npm run typecheck` | PASS |
| `npm run lint` | PASS, 0 errors / 0 warnings |
| `npm run test` | PASS: 37 tests, 5 files |
| `npm run build` | PASS; JS около 437 kB / 134 kB gzip, CSS около 31 kB / 6 kB gzip |
| Playwright Chromium + WebKit | PASS: финальный запуск 14/14, Chromium + WebKit (41 сек.) |
| Адаптивность | 320, 360, 390, 430 px, без горизонтального overflow; screenshots осмотрены |
| `docker build -t smartquarter-miniapp:react-front web/miniapp` | PASS, multi-stage Node → nginx |
| Nginx HTTP smoke | `/healthz`, `/`, `/issues/<UUID>` → 200; `/api/v1/me`, отсутствующий asset → 404 |
| Production bundle scan | Не найдены demo-fixture markers, `MOCK_DATA`, MAX_BOT_TOKEN, S3_SECRET |
| Git scope | Изменений вне `web/miniapp/**` нет |

Unit/RTL: контракты/enum/nullable fields, роль активного дома, отсутствие ACTIVE membership, disabled submit, сохранение описания после ошибки complete, запрет преждевременного CreateIssue, отображение Attachment и Statement, XSS как текст, 401/403/error mapping. Upload: MIME/size, exact signed headers, File bytes, credentials omit, запрещённые origins/headers, failed PUT/complete, expired URL, READY. HTTP contract-тесты покрывают все бизнес-группы маршрутов.

Браузерные сценарии: create с прямым PUT/complete и 503→повтором с тем же idempotency key; другой житель confirm; resident guard; chairman queue/generate/copy/status; create announcement; house switch без старых данных; 401; отсутствие MAX без fake login; responsive navigation. Ответы контролируются `page.route` только в tests. В production коде эти fixtures отсутствуют.

Build выдаёт предупреждения Rollup о комментариях `@__PURE__` внутри Zod; сборка завершается. Npm предупреждает о lifecycle версии ESLint 9; lock-файл закрепляет работающую конфигурацию. При установке/сборке npm audit сообщил 0 известных vulnerabilities.

## Реальная интеграция — границы доказанного

Настоящий Go Gateway собран из репозитория, запущен с временным Redis на loopback и одноразовыми тестовыми credentials. Проверены 200 `/livez`, 401 `/me` без cookie, 401 неверная MAX подпись, 403 недоверенный Origin, 503 `/readyz`. Пять запросов bootstrap с корректной тестовой подписью **заблокированы**: 503 `DEPENDENCY_UNAVAILABLE`, без session cookie. Причина подтверждена кодом: app использует `identity.Unavailable{}`.

**Реальный CreateIssue → ConfirmIssue → GenerateStatement flow: BLOCKED, успешных прогонов 0/5.** Пять проверок 503 не являются прохождением бизнес-E2E. Настоящий UI PUT в S3/MinIO, публикация в Community DB, запуск внутри MAX Android/iOS: NOT RUN из-за отсутствия рабочего session bootstrap и настроенного публичного стенда. Проверки direct PUT выполнены на уровне Fetch/браузерных fixtures; CORS настоящего S3 не доказан.

Недоступные REST функции: опросы, календарь, инициативы, администрирование жителей/дома. Объявления требуют настроенного Community. MAX скачивание заявления требует HTTPS file endpoint; пока есть копирование. Deep link payload поддержан во frontend, генерацию в боте надо согласовать. Формы хранятся в памяти; после перезагрузки проверьте список перед повторной отправкой, поскольку ключ намерения в постоянное хранилище не записывается.

## Быстрые команды

```sh
cd web/miniapp
npm ci
npm run dev
npm run build
npm run test
npm run test:e2e
```

Публичные настройки: `VITE_API_BASE_URL` (Origin, без `/api/v1`), `VITE_S3_ORIGINS` (точные разрешённые Origins), `GATEWAY_PROXY_TARGET` (только dev). Secret values во frontend не требуются. В production используется HTTPS ingress с реальным Gateway; самостоятельный nginx image раздаёт только статику.

## Изменённые и добавленные файлы

- `web/miniapp/.dockerignore`
- `web/miniapp/.env.example`
- `web/miniapp/.gitignore`
- `web/miniapp/.prettierignore`
- `web/miniapp/.prettierrc.json`
- `web/miniapp/Dockerfile`
- `web/miniapp/docs/backend-integration.md`
- `web/miniapp/docs/dependencies.md`
- `web/miniapp/docs/deployment-example.md`
- `web/miniapp/docs/design-migration.md`
- `web/miniapp/docs/verification.md`
- `web/miniapp/eslint.config.js`
- `web/miniapp/index.html`
- `web/miniapp/nginx.conf`
- `web/miniapp/package-lock.json`
- `web/miniapp/package.json`
- `web/miniapp/playwright.config.ts`
- `web/miniapp/public/licenses/cookie-LICENSE.txt`
- `web/miniapp/public/licenses/fontsource-manrope-LICENSE.txt`
- `web/miniapp/public/licenses/lucide-react-LICENSE.txt`
- `web/miniapp/public/licenses/NOTICE.txt`
- `web/miniapp/public/licenses/react-dom-LICENSE.txt`
- `web/miniapp/public/licenses/react-LICENSE.txt`
- `web/miniapp/public/licenses/react-router-dom-LICENSE.md.txt`
- `web/miniapp/public/licenses/react-router-LICENSE.md.txt`
- `web/miniapp/public/licenses/scheduler-LICENSE.txt`
- `web/miniapp/public/licenses/set-cookie-parser-LICENSE.txt`
- `web/miniapp/public/licenses/tanstack-query-core-LICENSE.txt`
- `web/miniapp/public/licenses/tanstack-react-query-LICENSE.txt`
- `web/miniapp/public/licenses/zod-LICENSE.txt`
- `web/miniapp/README.md`
- `web/miniapp/scripts/probe-gateway.mjs`
- `web/miniapp/src/app/App.tsx`
- `web/miniapp/src/app/styles/app.css`
- `web/miniapp/src/app/styles/reference.css`
- `web/miniapp/src/features/issues/drafts.tsx`
- `web/miniapp/src/features/session/SessionProvider.tsx`
- `web/miniapp/src/features/uploads/upload.ts`
- `web/miniapp/src/main.tsx`
- `web/miniapp/src/pages/CreateIssuePage.tsx`
- `web/miniapp/src/pages/HomePage.tsx`
- `web/miniapp/src/pages/IssueDetailsPage.tsx`
- `web/miniapp/src/pages/IssuesPage.tsx`
- `web/miniapp/src/pages/NewsPage.tsx`
- `web/miniapp/src/pages/ProfilePage.tsx`
- `web/miniapp/src/shared/api/client.ts`
- `web/miniapp/src/shared/api/index.ts`
- `web/miniapp/src/shared/api/models.ts`
- `web/miniapp/src/shared/max/bridge.ts`
- `web/miniapp/src/shared/ui/AttachmentPreview.tsx`
- `web/miniapp/src/shared/ui/components.tsx`
- `web/miniapp/src/shared/utils/presentation.ts`
- `web/miniapp/tests/api.test.ts`
- `web/miniapp/tests/contracts.test.ts`
- `web/miniapp/tests/e2e/flows.spec.ts`
- `web/miniapp/tests/fixtures.ts`
- `web/miniapp/tests/interface.test.tsx`
- `web/miniapp/tests/pages.test.tsx`
- `web/miniapp/tests/setup.ts`
- `web/miniapp/tests/upload.test.ts`
- `web/miniapp/tsconfig.json`
- `web/miniapp/vite.config.ts`
- `web/miniapp/vitest.config.ts`

