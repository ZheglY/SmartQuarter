# Community Service

Владеет community_db: объявления, опросы, календарь, инициативы и контакты служб.
Gateway вызывает типизированный CommunityService по gRPC; браузер не обращается к
сервису напрямую. Actor metadata приходит только из доверенной внутренней сети.

Настройки: DATABASE_URL, REDIS_URL, GRPC_ADDR (:9090), HTTP_ADDR (:8084),
NOTIFICATION_STREAM (stream:notifications). HTTP: /livez, /readyz, /metrics.

Серверные миграции: deploy/server/compose.yaml → community-migrate применяет
000001_init.up.sql и отдельную идемпотентную 000002_service_contacts.up.sql.
Существующие объявления не меняются. Контакты имеют категории, лимит 10 активных
записей на категорию и нормализованные международные телефоны. Жители получают
активные записи без автора и временных меток аудита; менеджеры могут создавать,
заменять поля и архивировать записи. Контакт, аудит и outbox записываются атомарно.

Outbox публикуется в общий stream:notifications с producer=community-service и
полным envelope версии 1. Получателей и настройки проверяет Gateway через Identity.
Публикация допускает повтор; UUID события стабилен. Старый max_notifications_stream
больше не используется.

Команды из каталога сервиса: go test ./..., go vet ./..., go build ./cmd/app.
Buf использует contracts/proto/smartquarter/community/v1/community.proto.
Полный HTTP/PostgreSQL/Redis тест: deploy/test/compose.yaml.

[Права, события, обновление и приёмка](../../docs/house-workflow/README.md).
