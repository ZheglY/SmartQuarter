# Границы сервисов

```text
MAX Mini App / Bot → max-gateway → identity-service
                                → issue-service → PostgreSQL + S3
                                → community-service
issue-service ↔ Redis Streams ↔ ai-worker
Redis Streams → max-gateway/internal/notifications → MAX API
```

Это целевая схема из приложенных рекомендаций. Каркас запускает только HTTP health endpoints;
MAX, бизнес-API, БД, Redis, S3 и AI пока не подключены.

- Gateway отвечает за транспорт, авторизацию и маршрутизацию. Бизнес-правила находятся в профильных сервисах.
- Identity, Issue и Community владеют отдельными базами и миграциями. Один физический PostgreSQL допустим.
- Issue хранит реальные проблемы отдельно от сообщений о них, метаданные фотографий и историю.
- AI Worker получает задания асинхронно, возвращает результат событием; окончательные решения остаются за доменным сервисом/пользователем.
- Уведомления на MVP находятся в Gateway; отдельный notification-worker пока не нужен.
- Клиентские API доступны через Gateway. Прямые загрузки по presigned S3 URL можно добавить отдельно.
- Общих Go-библиотек пока нет. Если появятся, создайте отдельный версионируемый модуль и проверяйте потребителей с `GOWORK=off`.

Локальный Compose содержит только пять каркасов. Для интеграций добавьте PostgreSQL с отдельными
пользователями/базами, Redis Streams и S3/MinIO; вместе с ними — readiness и миграции.

