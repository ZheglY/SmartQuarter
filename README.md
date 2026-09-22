# SmartQuarter - Умный Квартал

MAX Mini App для жителей и председателя дома. Реализованный MVP:
вход MAX → фото → проблема → подтверждение другим жителем → заявление →
ручная отправка председателем, изменение статуса и объявления дома.

## Развёртывание

Подробные команды: [гайд Ubuntu 24.04](docs/deployment/server-guide.md),
[PDF](output/pdf/SmartQuarter_Server_Deployment_Guide.pdf).
Используйте **deploy/server/compose.yaml**: React, Gateway, Identity, Issue,
Community, три PostgreSQL, Redis и Caddy HTTPS. Фотографии - внешний S3.
Старый deploy/docker-compose.yml относится к первоначальному каркасу.

```bash
cd deploy/server
cp .env.example .env
# Заполните домен, MAX, S3 и отдельные пароли БД по гайду.
chmod 600 .env
docker compose config --quiet
docker compose build
docker compose up -d
```

Identity подключён к Gateway через общий protobuf. Новый MAX-пользователь
получает сессию без доступа к дому; роли назначает оператор командой
`docker compose exec identity-service /app provision` (раздел 9 гайда).
Реальные токены, домен, S3 и регистрация webhook обязательны для испытаний в MAX.
Вход по поддельной роли или неподписанным данным не предусмотрен.

## Автоматическая приёмка

Из корня, Docker Engine и Compose v2:

```bash
docker compose -f deploy/test/compose.yaml build
docker compose -f deploy/test/compose.yaml run --rm test
# Удаляет только отдельный тестовый проект и его данные:
docker compose -f deploy/test/compose.yaml down -v
```

Стенд не публикует порты и не обращается к настоящему MAX. Тестовый Gateway
использует production router и настоящий Identity/Issue/Community по gRPC,
PostgreSQL, Redis и MinIO. MAX Bot API заменён тестовым HTTP-получателем.
Пять проходов проверяют фото, подтверждения, заявления, роли и уведомления;
дополнительно проверяются объявления, изоляция домов и отзыв доступа.
Это не заменяет проверку MAX Android/iOS и облачных credentials.

## Компоненты

| Компонент | Назначение | Внутренние порты |
| --- | --- | --- |
| max-gateway | MAX, HTTP API, сессии, gRPC-клиенты, уведомления | 8080 HTTP |
| identity-service | Пользователи, дома, memberships, роли | 50051 gRPC / 8081 HTTP |
| issue-service | Фото, проблемы, подтверждения, заявления, outbox | 8082 gRPC / 8083 HTTP |
| community-service | Объявления дома | 9090 gRPC / 8084 HTTP |
| web/miniapp | React + TypeScript, MAX Bridge | 8080 HTTP в контейнере |

gRPC и БД не публикуются в интернет. Контракты находятся в contracts/proto и
contracts/openapi. Опросы, календарь, инициативы и автоматическая отправка
заявлений во внешние ведомства не входят в текущий UI MVP.

## Проверки и CI

Go 1.26.x, Node 24. Каждый services/* - отдельный Go module:
`GOWORK=off go vet ./...` и `GOWORK=off go test ./...` из папки сервиса.
Из web/miniapp: npm ci, npm run typecheck, npm run lint, npm test,
npm run build, npx playwright install chromium webkit, npm run test:e2e.

CI включает Go, frontend, Docker builds и контейнерный application workflow.
Зависимости и action SHA зафиксированы. CD/публикация образов не настроены.
Обновление и откат на сервере описаны в гайде; merge/push выполняются отдельно.
