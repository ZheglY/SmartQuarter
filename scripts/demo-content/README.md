# Демонстрационный контент существующего дома

Набор: четыре заявки (новая, готовая к обращению, ожидание результата, решённая),
три иллюстрации, черновик заявления, три объявления, три опроса (включая завершённый),
три события, три инициативы (включая завершённую) и четыре образца контактов.
Все записи помечены как демонстрационные. Контакты нерабочие, `example.com` используется
как зарезервированный пример. Реальные пользователи, роли и дома не создаются и не меняются.

`seed.py` заменяет бизнес-контент **одного явно указанного существующего дома**.
Дочерние записи удаляются вместе с прежними заявками и опросами. Identity используется
только для чтения: авторы выбираются из действующих участников. Отпечаток всех таблиц
Identity сравнивается до и после. Скрипт требует остановленных прикладных сервисов и
проверенных дампов трёх БД. Без `--apply` выполняет SQL внутри откатываемых транзакций.

Из корня репозитория на Ubuntu, после подготовки резервных копий:

```bash
python3 scripts/demo-content/seed.py --house HOUSE_UUID \
  --manifest /absolute/path/images.json --backup-dir /absolute/path/backup
# После проверки результата добавить --apply к той же команде.
```

Манифест предварительно создаётся утилитой `services/issue-service/cmd/demo-upload`,
запущенной с окружением S3 сервиса Issue, параметрами `-house HOUSE_UUID -dir /images`.
Утилита загружает только три PNG из этого каталога и сверяет скачанные байты с исходником.
В stdout записывается JSON без секретов. Существующие объекты хранилища не удаляются:
они остаются пригодными для восстановления прежних записей из резервной копии.

Новые события outbox не создаются: заполнение примеров не рассылает сообщения жильцам.
Устаревшие события Issue/Community выбранного дома удаляются из очереди уведомлений;
сессии, настройки уведомлений и события Identity сохраняются.

## Иллюстрации

Изображения созданы встроенным инструментом imagegen специально для демонстрации.
Финальные файлы находятся в `images/`. Это вымышленные иллюстрации, не фотографии дома.
Использованные промпты:

- `uninvited-neighbor.png`: Create a polished landscape 3:2 humorous editorial 3D illustration for a Russian residential building demo issue report: a comically oversized but friendly non-threatening cockroach wearing tiny house slippers standing beside a kitchen sink in a clean modern apartment, holding a miniature suitcase as if it moved in. Warm natural morning light, tasteful sage green and cream palette, realistic materials, playful charming mood, high quality, no text, no logos, no watermark, no people. It must look clearly like a whimsical illustration, not documentary evidence. Save this project asset if supported.
- `lobby-regatta.png`: Landscape 3:2 polished humorous editorial 3D illustration for a residential community demo maintenance report: a tiny yellow toy rubber duck proudly sailing across a shallow puddle below a dripping radiator in a clean apartment building lobby, small bucket nearby, ceramic tile floor, warm morning light, sage green and cream decor, charming realistic materials, visually obvious water leak but no danger, no people, no text, no logos. Clearly whimsical illustration, not documentary evidence.
- `bench-inspection.png`: Landscape 3:2 polished humorous editorial 3D illustration for residential community demo report: a courtyard wooden bench with one visibly loose tilted seat slat, an adorable small round sparrow in a miniature yellow hard hat inspecting it beside a tiny tool box, apartment courtyard greenery blurred background, afternoon natural light, sage green cream and honey palette, realistic wood texture, warm charming restrained humor, no humans, no text, no logos. Clearly a whimsical illustration, not documentary evidence.

## Уведомления

Кнопка `open_app` передаёт `payload` через MAX `initData.start_param`:
`poll_<UUID>_<houseUUID>`, `initiative_<UUID>_<houseUUID>` и аналогично для заявки,
объявления и календаря. У события в конце передаётся Unix timestamp начала,
чтобы выбрать нужный месяц с учётом часового пояса пользователя.
Маршрут принимается только из разрешённого списка; UUID проверяются. Переключение дома
разрешено только при действующем участии, API повторно проверяет права на саму запись.

Основание формата: [MAX Bot API](https://dev.max.ru/docs-api/methods/POST/messages),
[MAX Bridge](https://dev.max.ru/docs/webapps/bridge).
