# «Вовремя» — мини-приложение MAX для контроля сроков документов бизнеса

Трек «Эффективный бизнес». Полная проектная документация: [docs/ARCHITECTURE_TZ.md](docs/ARCHITECTURE_TZ.md).

## 1. Назначение

Мини-приложение MAX, в котором владелец малого бизнеса и его сотрудники ведут сроки лицензий, КЭП, фискальных накопителей, договоров и медосмотров, видят, что просрочено и что скоро истекает, и получают напоминания в MAX заранее. Продукт — мини-приложение; бот нужен только для его запуска и доставки напоминаний.

## 2. Основной сценарий

1. Пользователь открывает бота в MAX и нажимает «Открыть» — запускается мини-приложение, вход выполняется автоматически по данным MAX.
2. Создаёт организацию: название, вид деятельности, регион, ответы на вопросы о бизнесе.
3. Получает список типовых документов для своего профиля, отмечает нужные и указывает даты окончания.
4. На дашборде видит статусы сроков и ближайшие даты.
5. За выбранное число дней до окончания получает сообщение бота; кнопка открывает карточку документа; после продления указывает новый срок.
6. Приглашает сотрудников по ссылке с ролью «редактор» или «наблюдатель».

## 3. Состав и архитектура

| Компонент | Технологии | Роль |
|---|---|---|
| `frontend` (собирается в образ `edge`) | React 18, TypeScript, Vite, MAX UI | MAX Mini App — весь пользовательский интерфейс |
| `edge` | Caddy 2.10 | HTTPS, раздача мини-приложения, прокси `/api/v1` → core и `/max/webhook` → bot |
| `core` | Go 1.27 | публичный REST API `/api/v1`, предметная логика, outbox и доставка событий |
| `reminders` | Go 1.27 | план напоминаний по событиям core, расписание, передача в bot |
| `bot` | Go 1.27 | webhook MAX, очередь и отправка сообщений бота |
| `postgres` | PostgreSQL 18 | данные: схемы `core`, `reminders`, `bot` (общий экземпляр, раздельные роли) |
| `core-migrate`, `reminders-migrate`, `bot-migrate` | Go | применение миграций при запуске |

`core` доставляет изменения в `reminders` (`IngestService`) и читает план (`ReminderQueryService`);
`core` и `reminders` отправляют сообщения через `bot` (`MessagingService`). Всё межсервисное
взаимодействие — gRPC. Диаграммы: [docs/architecture/system-overview.md](docs/architecture/system-overview.md).

Мини-приложение (каталог `frontend/`) реализовано полностью: 20 экранов, работа через
публичный API core-service, интеграция с клиентским SDK MAX. Отчёт —
[docs/implementation/FRONTEND_REPORT.md](docs/implementation/FRONTEND_REPORT.md),
карта экранов — [docs/frontend/screens-map.md](docs/frontend/screens-map.md).

## 4. Запуск одной командой

```sh
docker compose up -d --build
```

Поднимаются PostgreSQL, миграции трёх схем, сервисы `core`, `reminders`, `bot` и `edge`,
который собирает мини-приложение и отдаёт его. Готовность: `docker compose ps` — все сервисы
`healthy`. Интерфейс — `http://localhost:8080/?mockUser=1001` (режим имитации MAX для локальной
проверки), публичный API — `http://localhost:8080/api/v1`.

Чтобы работал настоящий канал MAX, достаточно вставить токен бота (выдают организаторы):

```sh
cp .env.example .env
# в .env: MAX_BOT_TOKEN=<токен>, BOT_MODE=live, BOT_WEBHOOK_PUBLIC_URL=https://<домен>/max/webhook, VITE_MOCK_BRIDGE=false
# printf '%s' '<токен>' > /tmp/max_token
# CORE_MAX_WEBAPP_SECRET_HEX=$(python3 scripts/derive_webapp_secret.py --token-file /tmp/max_token)
docker compose up -d --build
```

Без токена система работает полностью в локальном режиме `BOT_MODE=stub`: сообщения
попадают в буфер `docker compose exec bot wget -qO- http://127.0.0.1:8081/debug/stub/messages`.

## 5. Требования к окружению

Docker Engine 24+ с Compose v2.20+, 2 CPU, 4 ГБ RAM, 5 ГБ диска, свободный порт 8080. Для проверки сценариев из командной строки — Python 3.10+ и curl. Сборка занимает не более 5 минут без учёта загрузки базовых образов (`scripts/measure_build.sh`).

## 6. Переменные окружения

Локальный запуск работает без `.env` (значения по умолчанию из `compose.yaml`). Шаблон — [.env.example](.env.example), полный словарь — [docs/operations/configuration.md](docs/operations/configuration.md). Основные: `APP_ENV` (`local`/`prod`), `PUBLIC_BASE_URL`, `EDGE_SITE_ADDRESS`, `MAX_BOT_TOKEN` (только стенд), `BOT_MODE` (`stub`/`live`), `BOT_WEBHOOK_SECRET`, `CORE_MAX_WEBAPP_SECRET_HEX`, пароли ролей БД `PG_*_PASSWORD`, `VITE_MOCK_BRIDGE`.

## 7. Порты

| Порт хоста | Сервис | Назначение |
|---|---|---|
| 8080 | edge | мини-приложение, API `/api/v1`, webhook `/max/webhook` |
| 8443 | edge | HTTPS (только при доменном имени) |
| 127.0.0.1:9091 | prometheus | метрики (профиль `monitoring`) |
| 127.0.0.1:55432 | postgres-test | тесты (профиль `test`) |

Внутренние порты сервисов наружу не публикуются.

## 8. Зависимости

Go-модули — [go.mod](go.mod) и `go.sum`; npm-пакеты — `frontend/package.json` и
`frontend/package-lock.json`. Базовые образы: `golang:1.27-alpine`, `node:22-alpine`,
`caddy:2.10-alpine`, `alpine:3.22`, `postgres:18-alpine`.

## 9. Внешние сервисы

| Сервис | Назначение | Воспроизводится в Docker | Условия проверки |
|---|---|---|---|
| Платформа MAX (клиенты, Bot API `botapi.max.ru`) | запуск мини-приложения, доставка напоминаний | нет | проверка в MAX — на стенде по ссылке с первого слайда; локально — имитация Bridge и режим `BOT_MODE=stub` (сообщения видны в `GET /debug/stub/messages` внутри контейнера bot) |
| Let's Encrypt | сертификат HTTPS стенда | нет | только стенд |
| GigaChat API (Сбер) | языковой ассистент: заполнение карточки документа по тексту (FR-21) и по фото (FR-23), подбор вида деятельности по описанию бизнеса (FR-22) | нет — внешний сервис, нужен ключ авторизации | без ключа функции выключены: `GET /api/v1/me` возвращает `assistant_enabled: false`, блоки «Быстрый ввод» и «Подобрать по описанию» скрыты, операции `…/documents/draft`, `…/documents/draft-image` и `/profile-match` отвечают 503, остальные сценарии работают; с ключом — `bash scripts/setup_gigachat_env.sh` (раздел «Языковой ассистент»); на боевом стенде ключ подключён |

Адрес Bot API задаётся `BOT_MAX_API_BASE_URL`. Стенд работает с `botapi.max.ru`: на нём зарегистрирована
подписка на webhook. В актуальной документации MAX указан `platform-api2.max.ru` — при смене адреса достаточно
переменной, сертификаты НУЦ Минцифры уже есть в образе bot. При запуске bot сверяет типы событий подписки
и оформляет её заново, если в ней нет нужных (например, `message_callback` для «Напомнить через неделю»).

## 10. Данные

Храним только необходимое: идентификатор и имя пользователя MAX, организации, документы (названия, номера, даты, должность ответственного), план напоминаний, очередь сообщений. Файлы не хранятся. Исключение — распознавание по фото (FR-23): снимок сжимается в браузере (JPEG до 1600 px), через core передаётся в хранилище GigaChat (Сбер) только на время одного запроса и сразу удаляется оттуда; в базе, журналах и на диске сервиса фото не сохраняется, в журнал попадают только операция и её исход. Текст для быстрого ввода и описание бизнеса передаются в GigaChat так же, без идентификаторов пользователя и организации. Об этом предупреждает подпись под блоком «Быстрый ввод». Справочник типов документов — модельные данные, помечены в интерфейсе («сверяйте сроки с документом»). Пользователь удаляет аккаунт и свои организации в разделе «Аккаунт». Подробно: [docs/database/data-model.md](docs/database/data-model.md), [docs/architecture/security.md](docs/architecture/security.md).

## 11. Тестовые данные

[demo/demo-data.json](demo/demo-data.json) — организация «Кафе «Пример» (тестовые данные)» и 12 документов с датами относительно текущего дня.

```sh
docker compose exec core /app/core seed-demo
docker compose exec core /app/core review-token issue --login reviewer_editor --role editor --ttl 720h
docker compose exec core /app/core review-token issue --login reviewer_viewer --role viewer --ttl 720h
```

Токены используются в заголовке `Authorization: Bearer <токен>` и в [DATA-API.yaml](DATA-API.yaml) (`VV_TOKEN_EDITOR`, `VV_TOKEN_VIEWER`, запуск `python3 scripts/run_data_api_checks.py`). Токены для первого слайда выпускайте в день сдачи: `720h` (30 суток) — наибольший допустимый срок.

## 12. Сценарий проверки

### 12.1. Через интерфейс (основной путь)

После `docker compose up -d --build` откройте `http://localhost:8080/?mockUser=1001` —
мини-приложение в режиме имитации MAX от имени тестового пользователя 1001:

1. пройдите онбординг: название, вид деятельности, регион, признаки; вместо ручного выбора
   можно описать бизнес словами и нажать «Подобрать по описанию» (FR-22, нужен ключ GigaChat);
2. отметьте предложенные документы, при необходимости добавьте свои («Свои документы» → «Добавить»)
   и укажите сроки в календаре приложения — документы появятся в реестре;
3. «+» → «Быстрый ввод»: вставьте «Лицензия № 78РПА0012345, выдана 14.03.2024, действует до 13.03.2029»
   и нажмите «Заполнить по тексту» (FR-21) или сфотографируйте документ кнопкой «Фото документа» (FR-23) —
   заполненные поля подсвечиваются, сообщение над формой перечисляет найденные реквизиты;
   документ сохраняется только кнопкой «Сохранить»;
4. «Документы» → «Подобрать типовые документы» — подбор по профилю и свои документы прямо из реестра (FR-27);
   «Импорт из Excel» — выберите `.xlsx` или `.csv`, проверьте разбор строк и нажмите «Импортировать» (FR-24);
5. откройте карточку: статус, «осталось N дней», ближайшее напоминание;
   в блоке «Как продлить» отметьте выполненные шаги — прогресс сохраняется на устройстве;
6. «Участники» → «Пригласить» → «Скопировать ссылку». В режиме имитации ссылка ведёт на
   `https://max.ru/vovremya_local_bot?startapp=inv_…`: откройте в новой вкладке
   `http://localhost:8080/?mockUser=1002&startapp=inv_…` с той же частью `startapp` — приглашение принимается,
   организация появляется у второго пользователя;
7. «Настройки» → «Время напоминаний» — время выбирается в шторке; «Подготовить файл календаря» → «Скачать» —
   отдаётся `.ics`; «Выгрузить реестр в Excel» → «Скачать реестр» — отдаётся `.xlsx` (FR-25);
8. сообщения бота в режиме `stub` — под напоминанием есть кнопка «Напомнить через неделю» (FR-26):
   `docker compose exec bot wget -qO- http://127.0.0.1:8081/debug/stub/messages`;
   нажатие кнопки в режиме `stub` имитируется запросом из раздела 12.2 (шаг 10).

Проверка в настоящем клиенте MAX — `docs/operations/max-miniapp-setup.md`.

### 12.2. Через API

Все команды выполняются из корня архива после `docker compose up -d --build`.

```sh
# 1. Готовность всех сервисов
docker compose ps
curl -fsS http://localhost:8080/api/v1/catalog -o /dev/null -w '%{http_code}\n'   # 401 без токена — норма

# 2. Демонстрационные данные и сессия пользователя MAX
docker compose exec core /app/core seed-demo
BODY=$(python3 scripts/sign_initdata.py --user-id 1001 --first-name Тест --json)
TOKEN=$(curl -sS -X POST http://localhost:8080/api/v1/sessions \
  -H 'Content-Type: application/json' -d "$BODY" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

# 3. Профиль, справочник, организация
curl -sS -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/me
curl -sS -H "Authorization: Bearer $TOKEN" -X POST http://localhost:8080/api/v1/organizations \
  -H 'Content-Type: application/json' -d '{"id":"7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03",
  "name":"Кафе на Невском","business_category_code":"food_service","region_code":"RU-SPE",
  "timezone":"Europe/Moscow","feature_codes":["has_premises","sells_alcohol"]}'

# 4. Документ со сроком и напоминанием сегодня
curl -sS -H "Authorization: Bearer $TOKEN" -X POST \
  http://localhost:8080/api/v1/organizations/7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03/documents \
  -H 'Content-Type: application/json' -d '{"id":"2f8b6d41-9c3e-4a75-b1d8-6e5f4a3c2b10",
  "title":"Лицензия на алкоголь","valid_until":"'"$(date -u -d '+10 days' +%F)"'","reminder_offsets_days":[10]}'

# 5. Напоминание дошло до канала MAX (режим stub)
docker compose exec bot wget -qO- http://127.0.0.1:8081/debug/stub/messages

# 6. Экспорт календаря
curl -sS -H "Authorization: Bearer $TOKEN" -X POST \
  http://localhost:8080/api/v1/organizations/7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03/exports/calendar

# 7. Быстрый ввод по тексту (нужен ключ GigaChat; без ключа — 503 и assistant_enabled: false)
curl -sS -H "Authorization: Bearer $TOKEN" -X POST \
  http://localhost:8080/api/v1/organizations/7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03/documents/draft \
  -H 'Content-Type: application/json' -d '{"text":"Лицензия на Алкоголь от 12.03.2022"}'

# 8. Импорт из Excel: экран импорта отправляет строки пакетами до 30 документов
curl -sS -H "Authorization: Bearer $TOKEN" -X POST \
  http://localhost:8080/api/v1/organizations/7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03/documents/batch \
  -H 'Content-Type: application/json' -d '{"items":[
  {"id":"5b0c7e21-8d4a-4f3b-9e6c-1a2b3c4d5e61","title":"Договор аренды","valid_until":"2027-06-30"},
  {"id":"5b0c7e21-8d4a-4f3b-9e6c-1a2b3c4d5e62","title":"Устав","valid_until":null}]}'

# 9. Реестр в Excel: та же временная ссылка, что у календаря, с format=xlsx
LINK=$(curl -sS -H "Authorization: Bearer $TOKEN" -X POST \
  http://localhost:8080/api/v1/organizations/7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03/exports/calendar \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["download_url"])')
curl -sS "$LINK?format=xlsx" -o reestr.xlsx && file reestr.xlsx

# 10. «Напомнить через неделю»: нажатие кнопки имитируется событием message_callback
PAYLOAD=$(docker compose exec -T bot wget -qO- http://127.0.0.1:8081/debug/stub/messages | python3 -c \
  'import sys,json;print([b["payload"] for m in json.load(sys.stdin) for b in m.get("buttons") or [] if str(b.get("payload","")).startswith("snooze:")][-1])')
curl -sS -X POST http://localhost:8080/max/webhook -H 'Content-Type: application/json' \
  -H 'X-Max-Bot-Api-Secret: devonly-webhook-secret' \
  -d '{"update_type":"message_callback","timestamp":'"$(date +%s000)"',"callback":{"callback_id":"check-1","payload":"'"$PAYLOAD"'","user":{"user_id":1001}}}'
```

Ожидаемое: шаг 3 — `201`, роль `owner`; шаг 4 — `201`, `status: expiring`,
`reminders_state: actual`; шаг 5 — сообщение «Через 10 дней заканчивается срок: …» с кнопками
«Открыть документ», «Продлил» и «Напомнить через неделю» (она есть, когда до срока больше недели); шаг 6 — ссылка вида `/api/v1/downloads/<токен>`,
по ней отдаётся файл `.ics`; шаг 7 — `200`, `valid_from: 2022-03-12`, `valid_until: null`
(дата окончания в тексте не указана и не вычисляется); текст без реквизитов — `422 DOCUMENT_NOT_RECOGNIZED`;
шаг 8 — `201`, два документа, повтор того же запроса дублей не создаёт; шаг 9 — `Microsoft Excel 2007+`;
шаг 10 — `200`; повтор записан в план reminders-service через 7 суток во время напоминаний
(`docker compose logs reminders | grep "reminder snoozed"`), повторное нажатие в тот же день второго
повтора не создаёт. Если в буфере нет напоминания, выполните шаги 4–5 заново.

Полная сквозная проверка трёх сервисов без Docker (реальные процессы, реальная PostgreSQL):

```sh
E2E_ADMIN_DSN="postgres://USER:PASSWORD@127.0.0.1:5432/postgres" bash test/e2e/run.sh
```

Проверка в MAX: ссылка на бота и порядок действий — на первом слайде презентации.

## 13. Ожидаемое поведение

| Действие | Результат |
|---|---|
| Запрос `POST /api/v1/sessions` с изменённой строкой `init_data` | 401, `code = LAUNCH_DATA_INVALID` |
| Документ с окончанием через 10 дней | статус «Скоро истекает», «осталось 10 дней» |
| Документ с прошедшей датой | статус «Просрочен», «просрочен на N дней» |
| Повторная отправка формы | создаётся один документ |
| Роль «наблюдатель» | нет кнопок изменения; API отвечает 403 |
| Запрос без токена | 401 `application/problem+json`, `code = UNAUTHENTICATED` |
| Остановка бота в MAX | `GET /me` возвращает `reminders_channel.state = stopped`; напоминания не отправляются |
| Остановка reminders-service | документы создаются и читаются, `reminders_state = unavailable`; после запуска события доставляются, состояние возвращается к `actual` |
| «Быстрый ввод»: «Лицензия на Алкоголь от 12.03.2022» | заполнены название и «Действует с = 12.03.2022»; сообщение над формой: даты окончания в тексте нет — выберите её в календаре; без «Сохранить» документ не создаётся |
| «Быстрый ввод»: текст без реквизитов | 422 `DOCUMENT_NOT_RECOGNIZED`, подсказка, что добавить в текст; поля не меняются |
| «Фото документа»: снимок не документа или нечитаемый | 422 `DOCUMENT_NOT_RECOGNIZED`, просьба приложить фото лицензии, договора или сертификата целиком; при сбое GigaChat — 503 и «Ассистент сейчас недоступен» |
| «Подобрать по описанию»: «кофейня на 30 мест, продаём пиво» | подставлены вид деятельности и признаки только из справочника; пользователь может их изменить |
| «Документы» → «Подобрать типовые документы» | список по профилю без уже добавленных, свои документы добавляются по названию; после сохранения — возврат в реестр |
| Импорт таблицы со строкой без названия или с неверной датой | строка показана с причиной и не загружается; остальные уходят пакетами по 30; повтор после сбоя не создаёт дублей |
| «Выгрузить реестр в Excel» | файл `.xlsx`: документ, номер, кем выдан, ответственный, даты, статус, дней до окончания, напоминания, заметки; ссылка временная (10 минут, 3 скачивания) |
| «Напомнить через неделю» под напоминанием (кнопка есть, когда до срока больше недели) | всплывающее «Напомню через неделю»; через 7 суток во время напоминаний приходит «Напоминаю ещё раз. …» с точным остатком дней; повторное нажатие в тот же день — «Уже напомню через неделю»; если повтор не успел бы до дня окончания срока — «До окончания срока меньше недели»; продление или удаление документа, исключение из организации, удаление аккаунта и отключение уведомлений повтор отменяют |
| Выбор даты и времени в формах | открывается календарь или сетка времени приложения, а не системный элемент; в тёмной теме всё читаемо |

Примеры запросов и ответов: [docs/contracts/examples.md](docs/contracts/examples.md).

## 14. Ограничения

Один сервер без резервирования (восстановление из ежедневной копии); справочник типов документов модельный, сроки нужно сверять с документами; напоминания доставляются «хотя бы один раз» (возможен редкий дубль); файлы документов не хранятся (фото для распознавания сразу удаляется из GigaChat); кнопка «Сканировать QR» и тактильный отклик недоступны в desktop и web клиентах MAX; напоминания приходят только пользователям, открывшим бота; веб-версия MAX может не поддерживать скачивание файла — тогда ссылка открывается в браузере.

Языковой ассистент — внешняя зависимость: без ключа GigaChat быстрый ввод, распознавание фото и подбор профиля выключены, при недоступности сервиса они отвечают 503, а формы заполняются вручную. Качество разбора зависит от формулировок и снимка (блики, обрезанные края, мелкий шрифт): часть полей может остаться пустой, срок окончания не вычисляется, если его нет в документе. Суточный бюджет токенов `CORE_GIGACHAT_DAILY_TOKEN_BUDGET` (по умолчанию 200 000) после исчерпания отключает ассистента до следующих суток; частота — `CORE_RATE_ASSISTANT_PER_MIN` обращений в минуту на аккаунт. Импорт читает первый лист `.xlsx` или `.csv` до 1000 строк, формат `.xls` не поддерживается; клиенты MAX без распаковки `deflate-raw` (iOS до 16.4, старые Android WebView) открывают только `.csv`, о чём сообщает экран импорта. «Напомнить через неделю» есть только в напоминаниях, до срока которых больше недели. Подробно — [docs/implementation/known-limitations.md](docs/implementation/known-limitations.md).

## 15. Остановка и повторный запуск

```sh
docker compose down          # остановить, данные сохраняются в томе pgdata
docker compose up -d         # запустить снова
docker compose down -v       # остановить и удалить все данные
```


---

# Архитектура 2.0.0: три микросервиса

Backend состоит из трёх сервисов: **core-service** (публичный API и предметные данные),
**reminders-service** (план напоминаний и расписание), **bot-service** (канал MAX).
Frontend Mini App — клиентское приложение, микросервисом не является.

Главный документ: `docs/ARCHITECTURE_TZ.md`. Согласованность: `docs/architecture/consistency.md`.
Решения: `docs/adr/README.md` (ADR-017…ADR-036).
Отчёты реализации: `docs/implementation/FIRST_SERVICE_REPORT.md` (reminders-service),
`docs/implementation/SECOND_SERVICE_REPORT.md` (bot-service),
`docs/implementation/THIRD_SERVICE_REPORT.md` (core-service),
`docs/implementation/integration-report.md` (связка сервисов, включая трёхсервисную проверку).

Состояние реализации: **три backend-микросервиса и мини-приложение реализованы и проверены вместе**
(отчёт по мини-приложению — `docs/implementation/FRONTEND_REPORT.md`).

## Совместный запуск трёх сервисов

Через Docker Compose — раздел 4 выше. Без Docker сквозная проверка поднимает настоящие
процессы трёх сервисов на одной PostgreSQL, создаёт роли и схемы, применяет миграции
и прогоняет шесть сценариев (доставка, трёхсервисный путь мини-приложения,
отказ reminders и восстановление, отказ bot и восстановление, изоляция схем):

```sh
E2E_ADMIN_DSN="postgres://USER:PASSWORD@127.0.0.1:5432/postgres" bash test/e2e/run.sh
```

## Ручной запуск core-service

```sh
export CORE_MIGRATE_DATABASE_URL="postgres://core_migrator:PASSWORD@127.0.0.1:5432/vovremya?sslmode=disable"
export CORE_DATABASE_URL="postgres://core_app:PASSWORD@127.0.0.1:5432/vovremya?sslmode=disable"
export CORE_MAX_WEBAPP_SECRET_HEX=e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663
export CORE_HTTP_ADDR="127.0.0.1:8080" CORE_ADMIN_ADDR="127.0.0.1:8081"
export CORE_BOT_GRPC_ADDR="127.0.0.1:9090" CORE_REMINDERS_GRPC_ADDR="127.0.0.1:9091"
export CORE_PUBLIC_BASE_URL="http://127.0.0.1:8080"
psql "$CORE_MIGRATE_DATABASE_URL" -c "CREATE SCHEMA IF NOT EXISTS core"
go run ./services/core/cmd/core migrate up
go run ./services/core/cmd/core serve &
curl -fsS http://127.0.0.1:8081/readyz            # готовность core-service
go run ./services/core/cmd/core seed-demo         # демонстрационные данные
go run ./services/core/cmd/core review-token issue --login demo_reviewer --role editor --ttl 168h
```

`seed-demo` и `review-token` работают на той же конфигурации; `review-token issue`
требует загруженной демонстрационной организации (иначе код выхода 2).

## Автономный запуск reminders-service

```sh
cp .env.reminders.example .env.reminders
docker compose -f compose.reminders.yaml --env-file .env.reminders up -d --build
curl -fsS http://127.0.0.1:8081/readyz          # готовность сервиса
curl -fsS http://127.0.0.1:8081/metrics | head  # метрики
curl -fsS http://127.0.0.1:8090/messages        # сообщения, принятые двойником bot
docker compose -f compose.reminders.yaml --env-file .env.reminders down -v
```

## Ручной запуск bot-service

```sh
export BOT_MIGRATE_DATABASE_URL="postgres://bot_migrator:PASSWORD@127.0.0.1:5432/vovremya?sslmode=disable"
export BOT_DATABASE_URL="postgres://bot_app:PASSWORD@127.0.0.1:5432/vovremya?sslmode=disable"
export BOT_MODE=stub BOT_WEBHOOK_SECRET=devonly-webhook-secret
export BOT_GRPC_ADDR="127.0.0.1:9090" BOT_HTTP_ADDR="127.0.0.1:8080" BOT_ADMIN_ADDR="127.0.0.1:8081"
go run ./services/bot/cmd/bot migrate up
go run ./services/bot/cmd/bot serve &
curl -fsS http://127.0.0.1:8081/readyz               # готовность bot-service
curl -fsS http://127.0.0.1:8081/debug/stub/messages  # сообщения канала MAX в режиме stub
```

## Мини-приложение (frontend)

```sh
cd frontend
npm ci                      # установка зависимостей по package-lock.json
npm run gen-api             # типы API из ../openapi.yaml (после изменения контракта)
npm run dev                 # http://localhost:5173/?mockUser=1001, API проксируется на :8080 (docker compose)
npm run build               # production-сборка, вывод размеров чанков
npm test -- --run           # модульные и компонентные тесты (130)
npm run lint && npm run typecheck
```

Интеграционные проверки мини-приложения против настоящих трёх сервисов (без Docker):

```sh
STACK_ADMIN_DSN="postgres://USER:PASSWORD@127.0.0.1:5432/postgres" \
  bash scripts/run_local_stack.sh -- npm --prefix frontend run test:integration
```

Браузерные сценарии (нужны загруженные браузеры Playwright и поднятый стенд):

```sh
cd frontend && npx playwright install --with-deps chromium
E2E_BASE_URL=http://localhost:8080 npm run e2e
```

## Языковой ассистент (GigaChat)

Мини-приложение умеет заполнять карточку документа по вставленному тексту (FR-21) и по
фотографии (FR-23), а также подбирать вид деятельности с признаками по описанию бизнеса
(FR-22). Функция работает через core-service, браузер к GigaChat не обращается. Решение и
границы применения — [ADR-032](docs/adr/ADR-032-llm-assistant.md), распознавание фото и
классификация ответов — [ADR-033](docs/adr/ADR-033-photo-recognition.md).

Если в тексте или на фото нет реквизитов документа (или модель отказалась обработать снимок),
сервер отвечает `422 DOCUMENT_NOT_RECOGNIZED`, и форма объясняет, что приложить или дописать;
сбой самого сервиса — `503 DEPENDENCY_UNAVAILABLE`. Даты с явными словами «от», «выдана»,
«до», «по» сервер дополнительно находит в тексте сам; срок окончания не вычисляется.

Подключение: ключ авторизации берётся в личном кабинете Studio (проект GigaChat API →
Настройки API → Получить ключ) и записывается интерактивным скриптом:

```sh
bash scripts/setup_gigachat_env.sh
docker compose up -d --build core
docker compose logs core --tail 20 | grep -i ассистент
```

Скрипт спрашивает ключ скрытым вводом, область доступа, модель и суточный бюджет
токенов, записывает их в `.env` (файл в репозиторий не попадает) и по желанию
проверяет ключ запросом токена.

Без ключа функция выключена: `GET /api/v1/me` возвращает `assistant_enabled: false`,
интерфейс скрывает блоки ассистента, остальные сценарии работают как прежде.

## Проверки

```sh
gofmt -l services internal test
go vet ./...
go build ./...
buf lint && buf generate
export CORE_TEST_DATABASE_URL="postgres://USER:PASSWORD@127.0.0.1:5432/postgres"
export REMINDERS_TEST_DATABASE_URL="$CORE_TEST_DATABASE_URL"
export BOT_TEST_DATABASE_URL="$CORE_TEST_DATABASE_URL"
go test -count=1 ./...
go test -race -count=1 ./services/... ./internal/...
python3 scripts/check_compose.py        # статическая проверка compose без Docker
E2E_ADMIN_DSN="$CORE_TEST_DATABASE_URL" bash test/e2e/run.sh
```

Интеграционные тесты создают собственные базы данных и удаляют их содержимое;
без `CORE_TEST_DATABASE_URL`, `REMINDERS_TEST_DATABASE_URL` и `BOT_TEST_DATABASE_URL`
они пропускаются.
