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
# в .env: MAX_BOT_TOKEN=<токен>, BOT_MODE=live, BOT_WEBHOOK_PUBLIC_URL=https://<домен>/max/webhook
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
| Платформа MAX (клиенты, Bot API `platform-api2.max.ru`) | запуск мини-приложения, доставка напоминаний | нет | проверка в MAX — на стенде по ссылке с первого слайда; локально — имитация Bridge и режим `BOT_MODE=stub` (сообщения видны в `GET /debug/stub/messages` внутри контейнера bot) |
| Let's Encrypt | сертификат HTTPS стенда | нет | только стенд |

## 10. Данные

Храним только необходимое: идентификатор и имя пользователя MAX, организации, документы (названия, номера, даты, должность ответственного), план напоминаний, очередь сообщений. Файлы не хранятся. Справочник типов документов — модельные данные, помечены в интерфейсе («сверяйте сроки с документом»). Пользователь удаляет аккаунт и свои организации в разделе «Аккаунт». Подробно: [docs/database/data-model.md](docs/database/data-model.md), [docs/architecture/security.md](docs/architecture/security.md).

## 11. Тестовые данные

[demo/demo-data.json](demo/demo-data.json) — организация «Кафе «Пример» (тестовые данные)» и 12 документов с датами относительно текущего дня.

```sh
docker compose exec core /app/core seed-demo
docker compose exec core /app/core review-token issue --login reviewer_editor --role editor --ttl 336h
docker compose exec core /app/core review-token issue --login reviewer_viewer --role viewer --ttl 336h
```

Токены используются в заголовке `Authorization: Bearer <токен>` и в [DATA-API.yaml](DATA-API.yaml) (`VV_TOKEN_EDITOR`, `VV_TOKEN_VIEWER`, запуск `python3 scripts/run_data_api_checks.py`).

## 12. Сценарий проверки

### 12.1. Через интерфейс (основной путь)

После `docker compose up -d --build` откройте `http://localhost:8080/?mockUser=1001` —
мини-приложение в режиме имитации MAX от имени тестового пользователя 1001:

1. пройдите онбординг: название, вид деятельности, регион, признаки;
2. отметьте предложенные документы и укажите сроки — документы появятся в реестре;
3. откройте карточку: статус, «осталось N дней», ближайшее напоминание;
   в блоке «Как продлить» отметьте выполненные шаги — прогресс сохраняется на устройстве;
4. «Участники» → «Пригласить» → «Скопировать ссылку»; откройте её в новой вкладке
   с другим `?mockUser=1002` — приглашение принимается, организация появляется у второго пользователя;
5. «Настройки» → «Подготовить файл календаря» → «Скачать» — отдаётся `.ics`;
6. сообщения бота в режиме `stub`:
   `docker compose exec bot wget -qO- http://127.0.0.1:8081/debug/stub/messages`.

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
  "title":"Лицензия на алкоголь","valid_until":"'"$(date -u -d '+3 days' +%F)"'","reminder_offsets_days":[3]}'

# 5. Напоминание дошло до канала MAX (режим stub)
docker compose exec bot wget -qO- http://127.0.0.1:8081/debug/stub/messages

# 6. Экспорт календаря
curl -sS -H "Authorization: Bearer $TOKEN" -X POST \
  http://localhost:8080/api/v1/organizations/7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03/exports/calendar
```

Ожидаемое: шаг 3 — `201`, роль `owner`; шаг 4 — `201`, `status: expiring`,
`reminders_state: actual`; шаг 5 — сообщение «Через 3 дня заканчивается срок: …»;
шаг 6 — ссылка вида `/api/v1/downloads/<токен>`, по ней отдаётся файл `.ics`.

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

Примеры запросов и ответов: [docs/contracts/examples.md](docs/contracts/examples.md).

## 14. Ограничения

Один сервер без резервирования (восстановление из ежедневной копии); справочник типов документов модельный, сроки нужно сверять с документами; напоминания доставляются «хотя бы один раз» (возможен редкий дубль); файлы документов не загружаются; кнопка «Сканировать QR» и тактильный отклик недоступны в desktop и web клиентах MAX; напоминания приходят только пользователям, открывшим бота; веб-версия MAX может не поддерживать скачивание файла — тогда ссылка открывается в браузере.

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
Решения: `docs/adr/README.md` (ADR-017…ADR-031).
Отчёты реализации: `docs/implementation/FIRST_SERVICE_REPORT.md` (reminders-service),
`docs/implementation/SECOND_SERVICE_REPORT.md` (bot-service),
`docs/implementation/THIRD_SERVICE_REPORT.md` (core-service),
`docs/implementation/integration-report.md` (связка сервисов, включая трёхсервисную проверку).

Состояние реализации: **все три backend-микросервиса реализованы и проверены вместе**.
Не реализован интерфейс Mini App — задание `docs/handoffs/frontend-miniapp.md`.

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
npm run dev                 # http://localhost:5173/?mockUser=1001, API проксируется на :8080
npm run build               # production-сборка, вывод размеров чанков
npm test -- --run           # модульные и компонентные тесты (63)
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

Мини-приложение умеет заполнять карточку документа по вставленному тексту (FR-21)
и подбирать вид деятельности с признаками по описанию бизнеса (FR-22). Функция
работает через core-service, браузер к GigaChat не обращается. Решение и границы
применения — [docs/adr/ADR-032-llm-assistant.md](docs/adr/ADR-032-llm-assistant.md).

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
интерфейс скрывает оба блока, остальные сценарии работают как прежде.

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
