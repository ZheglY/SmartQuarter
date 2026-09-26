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

- `radiator-leak.png`: Photorealistic landscape 3:2 smartphone photograph for a clearly labeled DEMO residential maintenance report. Close view of a white metal radiator in an ordinary apartment building entrance hall, a small visible drip at the pipe joint, shallow water puddle on neutral gray ceramic floor tiles, slight damp staining under the fitting. Plain beige wall, realistic everyday lighting, natural imperfections, factual neutral composition, unedited phone photo appearance. No people, no toys, no animals, no humor, no text, no watermark, no logos. Fictional generic building, not a claim about a real location.
- `stair-light.png`: Photorealistic landscape 3:2 ordinary smartphone photo for a labeled DEMO apartment building maintenance report. A stairwell landing in a generic residential building, one ceiling bulkhead light visibly unlit while weak daylight comes through a side window, neutral off-white walls, gray concrete steps and a dark metal handrail. Underexposed but clearly readable real-looking phone camera photo, factual inspection framing, no dramatic effects. No people, no animals, no toys, no humor, no text or logos, no watermark. Fictional generic building, no identifiable real location.
- `damaged-bench.png`: Photorealistic landscape 3:2 straightforward smartphone inspection photograph for a labeled DEMO residential maintenance report. Weathered wooden bench on a paved courtyard path of a generic apartment complex, one seat slat cracked with a small piece missing and a loosened screw visible. Normal overcast daylight, background shrubs and generic blurred apartment facade, natural wear and realistic wood grain. Documentary-style plain composition, no cinematic treatment. No people, no animals, no toys, no humor, no text, no logos, no watermark. Fictional generic courtyard, not evidence of a real incident.

## Уведомления

Кнопка `open_app` передаёт `payload` через MAX `initData.start_param`:
`poll_<UUID>_<houseUUID>`, `initiative_<UUID>_<houseUUID>` и аналогично для заявки,
объявления и календаря. У события в конце передаётся Unix timestamp начала,
чтобы выбрать нужный месяц с учётом часового пояса пользователя.
Маршрут принимается только из разрешённого списка; UUID проверяются. Переключение дома
разрешено только при действующем участии, API повторно проверяет права на саму запись.

Основание формата: [MAX Bot API](https://dev.max.ru/docs-api/methods/POST/messages),
[MAX Bridge](https://dev.max.ru/docs/webapps/bridge).
