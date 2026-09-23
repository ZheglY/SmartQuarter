# Регистрация домов и управление участниками

Функциональность ветки `feat/house-identity-service` дополняет существующий процесс
«фото → проблема → подтверждения → заявление → статус». Источник требований —
предоставленный пользователем документ «Промпт Макс.docx».

## Первый запуск на сервере

Используйте [Compose для Ubuntu](../../deploy/server/compose.yaml) и
[основной гайд](../deployment/server-guide.md). Старый PDF описывает базовый запуск;
этот раздел добавляет регистрацию домов и обязательную настройку администратора.

1. В `deploy/server/.env` настройте домен, MAX, S3 и отдельные пароли БД.
2. Соберите и запустите приложение. Первый вход через MAX создаёт пользователя,
   но **не выдаёт роль или дом**. Пользователь видит поиск и регистрацию домов.
3. Оператор проверяет личность будущего администратора платформы. После его первого
   входа найдите внутренний UUID по проверенному MAX user ID:

   ```bash
   cd deploy/server
   docker compose exec identity-db psql -U identity -d identity_db
   ```

   ```sql
   -- Подставьте MAX ID проверенного администратора, а не номер телефона.
   SELECT id, max_user_id, display_name FROM users WHERE max_user_id = 123456789;
   ```

4. Запишите UUID в `ADMIN_USER_IDS` в `.env`. Несколько UUID разделяются запятыми.
   Это **внутренние UUID**, не MAX ID. Пустой список означает отсутствие администраторов.
5. Примените конфигурацию: `docker compose up -d --force-recreate identity-service`.
6. Администратор открывает «Мои дома → Заявки на регистрацию домов».
   Заявитель создаёт заявку, администратор проверяет адрес и полномочия, затем одобряет.
   Заявитель становится первым председателем и нажимает «Открыть дом».
7. Другой житель находит дом или получает приглашение. Председатель одобряет
   заявку в «Мои дома → Заявки жильцов». Житель выбирает дом в «Мои заявки».

Роль `ADMIN` в `memberships` относится к конкретному дому и **не делает** пользователя
администратором платформы. Администратор платформы может рассматривать регистрации
без членства в каком-либо доме. Команда `/app provision` остаётся инструментом
доверенного оператора для миграции/восстановления, а не публичной регистрацией.

## Обновление существующей установки

Сделайте резервные копии трёх PostgreSQL по основному гайду. До применения Identity
миграции проверьте дубли адресов и председателей:

```sql
SELECT lower(trim(regexp_replace(replace(lower(city),'ё','е'),'[[:space:],.;]+',' ','g'))) AS city_key,
       trim(regexp_replace(replace(lower(address),'ё','е'),'[[:space:],.;]+',' ','g')) AS address_key,
       array_agg(id) AS house_ids
FROM houses GROUP BY 1,2 HAVING count(*) > 1;
SELECT house_id, array_agg(user_id) FROM memberships
WHERE role='CHAIRMAN' AND status='ACTIVE' GROUP BY house_id HAVING count(*) > 1;
```

Дубли исправляет оператор после проверки данных. Миграция не объединяет дома и
не удаляет участников автоматически: при нарушении уникальности она завершится ошибкой.

```bash
cd deploy/server
docker compose build identity-service community-service max-gateway miniapp
docker compose run --rm community-migrate
docker compose up -d identity-service community-service max-gateway miniapp
docker compose ps
```

Identity применяет Goose `000002_house_workflow.sql` при старте. Community-migrate
последовательно применяет идемпотентные `000001_init.up.sql` и
`000002_service_contacts.up.sql`. После обновления проверьте `/readyz`, вход, создание
заявки и доставку уведомления. Для отката сначала остановите новые записи и восстановите
согласованную резервную копию; откат схемы уничтожает новые заявки и аудит.

## Контракты и состояния

Публичный источник REST: [OpenAPI](../../contracts/openapi/openapi.yaml).
gRPC: [Identity](../../contracts/proto/smartquarter/identity/v1/identity.proto),
[Community](../../contracts/proto/smartquarter/community/v1/community.proto).
Новые RPC находятся в отдельном `HouseService`; прежний `IdentityService` совместим.

| Объект | Допустимые переходы |
|---|---|
| Регистрация дома | PENDING → APPROVED / REJECTED / CANCELLED |
| Вступление | PENDING → APPROVED / REJECTED / CANCELLED |
| Приглашение | ACTIVE → EXPIRED / REVOKED / EXHAUSTED |
| Передача председательства | PENDING → ACCEPTED / REJECTED / CANCELLED / EXPIRED |
| Участие | ACTIVE ↔ INACTIVE; удаление отдельной командой |

REST использует полные имена protobuf enum, например `JOIN_REQUEST_STATUS_PENDING`.
Повтор того же решения идемпотентен; попытка изменить окончательное решение даёт 409.
В базе действует частичный уникальный индекс: не более одного ACTIVE CHAIRMAN на дом.
Принятие передачи в одной транзакции переводит старого председателя в RESIDENT и
нового — в CHAIRMAN. Получатель должен быть активным жителем. Срок предложения — 48 часов.

Нормализация адреса приводит регистр к нижнему, `ё` к `е`, объединяет пробелы и
разделители `,.;`. Ключ включает город и адрес. Это защита от точных нормализованных
дублей, а не геокодирование или подтверждение права собственности. «ул.» и «улица»
не считаются эквивалентными автоматически; проверка адреса остаётся задачей модератора.

Приглашения содержат 256 бит случайности; в PostgreSQL хранится только SHA-256.
Срок — 1–168 часов, лимит — 1–100 использований. Повторное принятие одним пользователем
не расходует лимит снова. Приглашение создаёт PENDING-заявку, а не членство.
Открытый код возвращается при создании; список и просмотр приглашения не возвращают
его. Успешный ответ команды с Idempotency-Key может повторяться из защищённого Redis
кэша в течение 24 часов. Перед повтором привилегированного ответа права проверяются снова.

## Права и границы доверия

| Действие | Кто выполняет |
|---|---|
| Поиск, создание регистрации, собственные заявки, настройки | Пользователь с MAX-сессией, членство не требуется |
| Одобрение/отказ регистрации | Только `ADMIN_USER_IDS` |
| Вступление, приглашения, состав дома | ACTIVE CHAIRMAN / ADMIN выбранного дома |
| Создание передачи роли | Текущий ACTIVE CHAIRMAN |
| Принятие/отказ передачи | Только назначенный получатель |
| Отмена передачи | Инициатор или администратор платформы |
| Удаление/деактивация председателя | Администратор платформы с явным `platform_admin_override: true` |
| Контакты служб: чтение | ACTIVE участник выбранного дома |
| Контакты служб: изменение | ACTIVE CHAIRMAN / ADMIN выбранного дома |

`platform_admin_override` сам по себе не даёт прав: UUID должен входить в allowlist.
Обычный интерфейс не предлагает удалять председателя. Для восстановления доступа к
дому без председателя оператор использует серверную консоль. Администраторские
операции с участниками используют выбранный дом сессии; обычное переключение дома
требует активного членства.

Пользователь, дом и роль берутся из серверной сессии. HTTP-заголовки `x-actor-*`
игнорируются. Identity повторно читает участников из БД; Gateway заново проверяет
членство перед операциями Community/Issue. Внутренний gRPC доверяет только частной
сети Compose: **не публикуйте его порты наружу**. Внешнего RPC-сервера с mTLS в MVP нет.
Команды требуют JSON (включая `{}` для пустых действий), доверенный Origin и имеют
rate limit. UUID и неизвестные поля проверяются, gRPC-коды преобразуются в стабильные HTTP-ошибки.

## Контакты служб

Community владеет `service_contacts` и `service_contact_audit`. Указаны категория,
название, организация, два телефона, email, сайт, описание, признак аварийности,
порядок и активность. Телефоны сохраняются строками в формате `+79991234567`;
добавочные номера можно указать в описании. HTML запрещён; сайт — только HTTP/HTTPS.
На категорию разрешено до 10 активных контактов. PATCH заменяет все редактируемые поля.
Архивные контакты доступны менеджерам; жителям выдаются активные записи без автора
и временных меток аудита. Интерфейс содержит ссылки `tel:`.

## События и уведомления

Identity пишет бизнес-изменение, аудит и outbox в одной PostgreSQL-транзакции.
Запись включает UUID, тип, версию 1, время, producer, payload, `published_at`,
`attempts`, `last_error`. Публикация в `NOTIFICATION_STREAM` (по умолчанию
`stream:notifications`) использует `event_id`, `event_type`, `data` с полным JSON envelope.
Community публикует тот же envelope; прежний отдельный `max_notifications_stream`
больше не используется.

| Producer | Семейства событий | Получатели |
|---|---|---|
| identity-service | house.registration.created/approved/rejected/cancelled | Заявитель, включая пользователя без членства |
| identity-service | house.created, house.chairman.assigned | Первый председатель |
| identity-service | house.join.created | Актуальные менеджеры дома |
| identity-service | house.join.approved/rejected/cancelled | Заявитель |
| identity-service | house.membership.created/activated/deactivated/removed | Участник |
| identity-service | house.invite.created/redeemed/revoked | Создатель/принявший приглашение |
| identity-service | house.chairman.transfer_requested/completed/accepted/rejected/cancelled | Инициатор/получатель |
| community-service | announcement.created, poll.created/voted, calendar.event_created, initiative.created | ACTIVE участники соответствующего дома |
| community-service | service_contact.create/update/archive | ACTIVE участники дома |
| issue-service | issue.created/confirmed/status_changed, statement.generated | Автор проблемы с актуальным доступом |

Gateway проверяет producer, тип и envelope, получает пользователей через внутренний
`ListNotificationRecipients`, проверяет актуальные настройки и доступ перед отправкой.
Общие уведомления не раскрывают содержимое объекта вне Mini App. Обновления доступа
достигают заявителя без членства, если он не выключил уведомления.

Настройки: общий переключатель, Issue, события дома, участие, доставка ботом.
Дедупликация события и каждого получателя хранится 7 суток. Ошибки повторяются через
Redis Pending/AutoClaim (30 секунд), после 10 попыток — `stream:notifications:dead`.
После сбоя между успешным MAX Send и фиксацией результата возможно повторное сообщение:
MAX не предоставляет здесь транзакцию с Redis, поэтому exactly-once не обещается.

Диагностика:

```sql
SELECT count(*), min(occurred_at) FROM identity_outbox WHERE published_at IS NULL;
SELECT event_id,event_type,attempts,last_error FROM identity_outbox
WHERE published_at IS NULL ORDER BY occurred_at LIMIT 20;
```

Проверьте Redis `XPENDING stream:notifications <группа>` и DLQ через `XRANGE`.
Не очищайте поток/дедупликацию для повтора: исправьте причину и повторно опубликуйте
нужный envelope с тем же UUID, сохранив уже доставленных получателей.
Метрики: `identity_house_operations_total`, `identity_outbox_published_total`,
`identity_outbox_publish_failures_total`, Gateway `http_requests_total`,
`grpc_client_requests_total`, `notification_events_total`, `notification_failures_total`.
ID пользователей, адреса и приглашения не используются как labels метрик.

## MAX: ручная проверка после деплоя

Бот поддерживает `/start`, `/help`, `/settings`, неизвестные команды и меню:
открыть Mini App, найти/зарегистрировать дом, мои заявки, уведомления.
Ссылки используют [официальный startapp](https://dev.max.ru/help/deeplinks).
`register_house`, `find_house`, `my_requests`, `settings`, `invite_<token>` — соглашения
нашего приложения. Параметр запуска обрабатывается после подтверждённого входа.

С двух-трёх реальных аккаунтов проверьте: заявка → одобрение администратором →
справочник служб → поиск/одобрение жильца → приглашение третьего → передача роли →
отказ старому председателю → уведомления и их отключение. Проверьте webhook secret,
HTTPS, кнопку открытия, BackButton, `tel:` и deeplink приглашения на Android/iOS.
Автоматические тесты заменяют только внешнюю доставку MAX тестовым HTTP endpoint;
они не подтверждают настройки вашего настоящего бота или домена.

## Проверки и ограничения MVP

- `go test ./...`, `go vet ./...`, `go build ./...` во всех четырёх Go-модулях.
- Для Identity дополнительно `HOUSE_TEST_DATABASE_URL` должен указывать на отдельную
  `identity_test_house`; тесты применяют миграции и проверяют гонки/переходы/права.
- `buf lint contracts/proto`; генераторы в `scripts/generate-house-*.py`,
  `scripts/generate-contact-contracts.py`; стандартные protobuf stubs генерируются Buf.
- `python services/max-gateway/scripts/openapi.py`; OpenAPI 3.1 проверяется валидатором.
- Mini App: `npm ci`, `npm run lint`, `npm test`, `npm run build`, `npm run test:e2e`.
- [Docker acceptance](../../deploy/test/README.md): пять старых Issue-workflow и новый
  House workflow с реальными Identity/Community/PostgreSQL/Redis/S3.

HTTP-списки ограничены 100 записями, участники — 500; массовое управление и постраничный
архив заявок не входят в этот MVP. Получатели уведомлений читаются страницами по 100.
Для транзакций жизненного цикла используется общий advisory lock; это осознанное
ограничение пропускной способности MVP. Нагрузочное тестирование отдельно не проводилось.
Продвинутые диалоги бота, inline-подтверждения бизнес-действий и дайджесты отложены.
Опросы/календарь/инициативы имеют существующие Community RPC и события, но их полноценные
экраны остаются за пределами этого изменения.
