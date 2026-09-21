# SmartQuarter — Умный Квартал

Каркас MAX Mini App для жителей и председателя дома. Целевой сценарий:
фото → проблема → подтверждения → заявление по шаблону → ручная отправка председателем.
MVP включает четыре Go-сервиса, без AI Worker.

| Компонент | Зона ответственности | Локальный HTTP-порт |
| --- | --- | --- |
| max-gateway | MAX, HTTP API, auth/session, gRPC-клиенты, уведомления | 8080 |
| identity-service | Пользователи, дома, memberships, роли | 8081 |
| issue-service | Проблемы, файлы, подтверждения, статусы, шаблоны заявлений | 8082 |
| community-service | Объявления; позже опросы, календарь и инициативы | 8084 |
| web/miniapp | Будущий React + TypeScript интерфейс в MAX | Пока не запускается |

## Структура

```text
services/                         отдельный go.mod и Dockerfile у каждого сервиса
  max-gateway/                    внешний HTTP, MAX, сессии и внутренние клиенты
  identity-service/               собственные domain/usecase, PostgreSQL и gRPC
  issue-service/                  то же + storage/s3 и statement/templates
  community-service/              собственные domain/usecase, PostgreSQL и gRPC
web/miniapp/                       app, pages, features, shared, public, tests
contracts/
  proto/smartquarter/{identity,issue,community}/v1/
  openapi/                        будущий публичный API Gateway
  events/                         будущие события уведомлений
deploy/docker-compose.yml         локальный запуск четырёх Go-каркасов
tests/e2e/                        место для сквозных проверок
docs/                            архитектура и место для материалов сдачи
go.work                          локальный workspace четырёх модулей
```

Структура адаптирована по восьми PDF: [решения и источники](docs/architecture.md).
Назначение внутренних каталогов описано в README каждого сервиса и [Mini App](web/miniapp/README.md).

## Текущее состояние

Реализованы только прежние `GET /healthz`, чтение `HTTP_ADDR`, JSON-логи через slog и graceful shutdown.
Новые каталоги содержат `.gitkeep`: gRPC, авторизация, бизнес-логика, React, proto/OpenAPI,
миграции, шаблоны заявлений и внешние интеграции ещё не реализованы.
PostgreSQL, Redis и MinIO пока не подключены к Compose. Целевые zap, `/livez`, `/readyz`, `/metrics`
из документов будут добавлены при реализации инфраструктуры. Это каркас, не готовый к сдаче MVP.

## Запуск и проверки

Нужен Go 1.26.x. Запуск одного сервиса из корня:

```sh
go run ./services/issue-usecase/cmd/app
# В другом терминале: curl http://localhost:8082/healthz
```

Параметр `HTTP_ADDR` переопределяет адрес; примеры есть в `services/<name>/.env.example`.
В PowerShell: `$env:HTTP_ADDR = ":9082"`. Файлы `.env` автоматически не загружаются.

С Docker и Compose v2:

```sh
docker compose -f deploy/docker-compose.yml up --build -d
curl http://localhost:8080/healthz
docker compose -f deploy/docker-compose.yml down
```

Повторный запуск — та же команда `up`. Наружу опубликован только Gateway на localhost:8080;
внутри сети HTTP-диагностика сервисов доступна по `<service>:8080`. gRPC пока не слушается.

Проверка модуля в Linux/macOS/Git Bash:

```sh
cd services/issue-usecase
export GOWORK=off
gofmt -l .
go vet ./...
go test ./...
```

В PowerShell вместо `export`: `$env:GOWORK = "off"`; вернуть workspace: `Remove-Item Env:GOWORK`.
Для всех сервисов с GNU Make и POSIX shell: `make check`; исправить форматирование: `make fmt`.
Корневой `go test ./...` не обходит отдельные модули.

## Совместная разработка и CI

Каждый сервис — независимый модуль `github.com/ZheglY/SmartQuarter/services/<name>`.
Обновляйте его зависимости из собственной папки, выполняйте `GOWORK=off go mod tidy`
и коммитьте `go.mod`/`go.sum`. Пока внешних зависимостей нет, `go.sum` не требуется.
Сервисы не импортируют domain-код друг друга и не читают чужие БД; контракты согласуются в PR.

Работа — в ветках через PR в `main`; реальные владельцы назначаются в `.github/CODEOWNERS`.
В GitHub Actions один check `CI`: `gofmt`, `go vet`, `go test` для всех Go-модулей с `GOWORK=off`.
Запуск на PR, push в `main` и вручную. Сборки контейнеров, публикации и развёртывания в CI нет.
Frontend/proto-проверки добавляются после появления соответствующего кода и инструментов.

Новый Go-сервис добавьте в `go.work`, Compose и CODEOWNERS; CI и Makefile находят модули автоматически.
Требования к будущим API и материалам сдачи: [контракты](contracts/README.md), [архитектура](docs/architecture.md).
